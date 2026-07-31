#!/usr/bin/env bash

# shellcheck disable=SC2034 # Read by operation orchestration after verification.
OAW_FORCE_BACKUP_REQUIRED=0
OAW_FORCED_TARGET_CHECKSUM=
OAW_FORCE_POLICY_BASELINE=

verify_mutation_policy_state() {
  local actual_policy_checksum=

  [ "$STATE_SCOPE" = "$OAW_SCOPE" ] || die "installed scope does not match" 65
  case "$OAW_SCOPE" in
    user) [ -z "$STATE_PROJECT_ROOT" ] || die "installed project root does not match" 65 ;;
    project)
      [ "$STATE_PROJECT_ROOT" = "$OAW_PROJECT_ROOT" ] ||
        die "installed project root does not match" 65
      ;;
  esac
  [ "$STATE_POLICY_PATH" = "$OAW_POLICY_DESTINATION" ] ||
    die "installed policy path does not match" 65
  [ -f "$STATE_POLICY_PATH" ] || die "managed policy is missing" 65
  actual_policy_checksum=$(checksum_file "$STATE_POLICY_PATH")
  if [ "$actual_policy_checksum" != "$STATE_POLICY_CHECKSUM" ]; then
    [ "$OAW_FORCE" -eq 1 ] || die "managed policy has drifted" 65
    OAW_FORCE_BACKUP_REQUIRED=1
    OAW_FORCE_POLICY_BASELINE=$STATE_POLICY_CHECKSUM
  fi
}

verify_forced_managed_target() {
  local target_id=$1
  local target_path=$2
  local target_checksum=$3
  local target_status=

  target_status=$(managed_block_status "$target_path")
  if [ "$target_status" != present ]; then
    render_managed_block "$target_id" "$STATE_POLICY_PATH" \
      "$OAW_OPERATION_TEMP/expected-block-$target_id" "$OAW_SCOPE"
    if [ "$(checksum_file "$OAW_OPERATION_TEMP/expected-block-$target_id")" = \
      "$target_checksum" ] && repair_managed_marker_structure "$target_path" \
      "$OAW_OPERATION_TEMP/expected-block-$target_id" \
      "$OAW_OPERATION_TEMP/forced-current-$target_id"; then
      OAW_FORCE_BACKUP_REQUIRED=1
      return 0
    fi
    create_manual_recovery_backup "$OAW_COMMAND" "$target_id" "$target_path"
  fi
  extract_managed_block "$target_path" "$OAW_OPERATION_TEMP/current-block-$target_id"
  OAW_FORCED_TARGET_CHECKSUM=$(checksum_file \
    "$OAW_OPERATION_TEMP/current-block-$target_id")
}

verify_forced_target_record() {
  local target_id=$1
  local target_path=$2
  local target_mode=$3
  local target_checksum=$4
  local expected_target_path=
  local actual_target_checksum=

  expected_target_path=$(target_destination "$target_id") || return $?
  [ "$target_path" = "$expected_target_path" ] ||
    die "installed target path does not match: $target_id at $target_path" 65
  [ "$target_mode" = "$(target_ownership "$target_id")" ] ||
    die "installed target ownership does not match: $target_id at $target_path" 65
  [ -f "$target_path" ] ||
    die "forced target has no recoverable file: $target_id at $target_path" 65
  case "$target_mode" in
    managed-block)
      OAW_FORCED_TARGET_CHECKSUM=
      verify_forced_managed_target "$target_id" "$target_path" "$target_checksum" ||
        return $?
      actual_target_checksum=$OAW_FORCED_TARGET_CHECKSUM
      ;;
    owned-file) actual_target_checksum=$(checksum_file "$target_path") ;;
    *) die "unknown target ownership mode: $target_mode" 65 ;;
  esac
  if [ "$actual_target_checksum" != "$target_checksum" ]; then
    OAW_FORCE_BACKUP_REQUIRED=1
  fi
}

verify_mutation_target_records() {
  local target_records=$1
  local selected_ids=$2
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=
  local selected_paths=$OAW_OPERATION_TEMP/force-selected-paths

  tab=$(printf '\t')
  : >"$selected_paths"
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    if target_id_is_selected "$selected_ids" "$target_id"; then
      printf '%s\n' "$target_path" >>"$selected_paths"
    fi
  done <"$target_records"
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    [ -z "$extra" ] || die "invalid mutation target record" 65
    : "$target_origin"
    if [ "$OAW_FORCE" -eq 1 ] && grep -F -x "$target_path" "$selected_paths" >/dev/null; then
      verify_forced_target_record "$target_id" "$target_path" \
        "$target_mode" "$target_checksum"
    else
      verify_state_target_record "$OAW_SCOPE" "$OAW_PROJECT_ROOT" \
        "$target_id" "$target_path" "$target_mode" "$target_checksum"
    fi
  done <"$target_records"
}
