#!/usr/bin/env bash

OAW_CHECK_TEMP=
CHECK_STATE_STATUS=not-installed
CHECK_POLICY_STATUS=not-installed
CHECK_STATE_SCOPE=
CHECK_STATE_PROJECT_ROOT=
CHECK_POLICY_PATH=
CHECK_POLICY_CHECKSUM=

cleanup_check_temp() {
  if [ -n "$OAW_CHECK_TEMP" ] && [ -d "$OAW_CHECK_TEMP" ]; then
    rm -rf -- "$OAW_CHECK_TEMP"
  fi
}

prepare_installation_health() {
  local tab=
  local extra=
  local actual_policy_checksum=

  init_oaw_paths
  CHECK_STATE_STATUS=not-installed
  CHECK_POLICY_STATUS=not-installed
  CHECK_STATE_SCOPE=
  CHECK_STATE_PROJECT_ROOT=
  CHECK_POLICY_PATH=
  CHECK_POLICY_CHECKSUM=

  [ -f "$OAW_STATE_FILE" ] || return 0
  OAW_CHECK_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-check.XXXXXX") ||
    die "cannot create check workspace" 73
  trap cleanup_check_temp EXIT HUP INT TERM

  if (
    load_state_file "$OAW_STATE_FILE" "$OAW_CHECK_TEMP/target-records" || exit $?
    printf '%s\t%s\t%s\n' \
      "$STATE_SCOPE" "$STATE_POLICY_PATH" "$STATE_POLICY_CHECKSUM" \
      >"$OAW_CHECK_TEMP/metadata"
    printf '%s\n' "$STATE_PROJECT_ROOT" >"$OAW_CHECK_TEMP/project-root"
  ) 2>/dev/null; then
    tab=$(printf '\t')
    IFS="$tab" read -r CHECK_STATE_SCOPE CHECK_POLICY_PATH CHECK_POLICY_CHECKSUM extra \
      <"$OAW_CHECK_TEMP/metadata" || {
        CHECK_STATE_STATUS=invalid-state
        return 0
      }
    IFS= read -r CHECK_STATE_PROJECT_ROOT <"$OAW_CHECK_TEMP/project-root" || {
      CHECK_STATE_STATUS=invalid-state
      return 0
    }
    if [ -n "$extra" ] || [ "$CHECK_POLICY_PATH" != "$OAW_POLICY_DESTINATION" ]; then
      CHECK_STATE_STATUS=invalid-state
      return 0
    fi
    case "$OAW_SCOPE" in
      user)
        [ "$CHECK_STATE_SCOPE" = user ] && [ -z "$CHECK_STATE_PROJECT_ROOT" ] ||
          CHECK_STATE_STATUS=invalid-state
        ;;
      project)
        [ "$CHECK_STATE_SCOPE" = project ] &&
          [ "$CHECK_STATE_PROJECT_ROOT" = "$OAW_PROJECT_ROOT" ] ||
          CHECK_STATE_STATUS=invalid-state
        ;;
    esac
    [ "$CHECK_STATE_STATUS" != invalid-state ] || return 0
  else
    CHECK_STATE_STATUS=invalid-state
    return 0
  fi

  CHECK_STATE_STATUS=valid
  CHECK_POLICY_STATUS=drift
  if [ -f "$CHECK_POLICY_PATH" ]; then
    actual_policy_checksum=$(checksum_file "$CHECK_POLICY_PATH")
    if [ "$actual_policy_checksum" = "$CHECK_POLICY_CHECKSUM" ]; then
      CHECK_POLICY_STATUS=clean
    fi
  fi
}

untracked_target_health() {
  local target_id=$1
  local target_path=
  local target_mode=
  local target_status=

  target_path=$(target_destination "$target_id")
  target_mode=$(target_ownership "$target_id")
  case "$target_mode" in
    managed-block)
      target_status=$(managed_block_status "$target_path")
      case "$target_status" in
        absent) printf 'not-installed\n' ;;
        present|drift) printf 'drift\n' ;;
        *) printf 'invalid-state\n' ;;
      esac
      ;;
    owned-file)
      if [ -e "$target_path" ]; then
        printf 'drift\n'
      else
        printf 'not-installed\n'
      fi
      ;;
    *) printf 'invalid-state\n' ;;
  esac
}

installed_target_health() {
  local target_id=$1
  local expected_target_path=
  local target_status=
  local actual_target_checksum=

  case "$CHECK_STATE_STATUS" in
    not-installed)
      untracked_target_health "$target_id"
      return 0
      ;;
    invalid-state)
      printf 'invalid-state\n'
      return 0
      ;;
  esac

  if ! load_target_record "$OAW_CHECK_TEMP/target-records" "$target_id"; then
    untracked_target_health "$target_id"
    return 0
  fi

  expected_target_path=$(target_destination "$target_id")
  if [ "$STATE_TARGET_PATH" != "$expected_target_path" ]; then
    printf 'invalid-state\n'
    return 0
  fi
  if [ "$CHECK_POLICY_STATUS" != clean ]; then
    printf 'drift\n'
    return 0
  fi

  case "$STATE_TARGET_MODE" in
    managed-block)
      target_status=$(managed_block_status "$STATE_TARGET_PATH")
      if [ "$target_status" != present ]; then
        printf 'drift\n'
        return 0
      fi
      extract_managed_block "$STATE_TARGET_PATH" "$OAW_CHECK_TEMP/block-$target_id"
      actual_target_checksum=$(checksum_file "$OAW_CHECK_TEMP/block-$target_id")
      ;;
    owned-file)
      if [ ! -f "$STATE_TARGET_PATH" ]; then
        printf 'drift\n'
        return 0
      fi
      actual_target_checksum=$(checksum_file "$STATE_TARGET_PATH")
      ;;
    *)
      printf 'invalid-state\n'
      return 0
      ;;
  esac
  [ "$actual_target_checksum" = "$STATE_TARGET_CHECKSUM" ] &&
    printf 'clean\n' || printf 'drift\n'
}

command_check() {
  local oaw_version=
  local provider_name=
  local check_target=

  if [ ! -r "$OAW_SOURCE_DIR/VERSION" ]; then
    die "VERSION is not readable" 70
  fi

  IFS= read -r oaw_version <"$OAW_SOURCE_DIR/VERSION" || die "VERSION is invalid" 70
  if [ -z "$oaw_version" ]; then
    die "VERSION is invalid" 70
  fi

  printf 'version: %s\n' "$oaw_version"
  if [ "$OAW_SCOPE" = project ]; then
    printf 'scope: project (%s)\n' "$OAW_PROJECT_ROOT"
  else
    printf 'scope: user\n'
  fi
  printf 'targets: %s\n' "$OAW_SELECTED_TARGETS"

  for provider_name in superpowers matt ecc; do
    if detect_provider "$provider_name"; then
      printf 'provider %s: detected\n' "$provider_name"
    else
      printf 'provider %s: missing\n' "$provider_name"
    fi
  done

  for check_target in $(target_ids); do
    if selected_target "$check_target"; then
      printf 'target %s: ' "$check_target"
      target_readiness "$check_target"
    fi
  done

  prepare_installation_health
  for check_target in $(target_ids); do
    selected_target "$check_target" || continue
    if [ "$OAW_SCOPE" = user ] && ! target_supports_user "$check_target"; then
      continue
    fi
    printf 'installed %s: ' "$check_target"
    installed_target_health "$check_target"
  done
}
