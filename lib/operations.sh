#!/usr/bin/env bash

OAW_OPERATION_TEMP=

cleanup_operation_temp() {
  if [ -n "$OAW_OPERATION_TEMP" ] && [ -d "$OAW_OPERATION_TEMP" ]; then
    rm -rf -- "$OAW_OPERATION_TEMP"
  fi
}

read_source_version() {
  local source_version=
  IFS= read -r source_version <"$OAW_SOURCE_DIR/VERSION" || die "VERSION is invalid" 70
  [ -n "$source_version" ] || die "VERSION is invalid" 70
  printf '%s\n' "$source_version"
}

prepare_operation_temp() {
  OAW_OPERATION_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-operation.XXXXXX") ||
    die "cannot create operation workspace" 73
  trap cleanup_operation_temp EXIT HUP INT TERM
}

render_target_artifacts() {
  local target_id=$1
  local target_path=$2
  local target_origin=$3
  local source_version=$4
  local block_checksum=
  local target_mode=

  [ -n "$source_version" ] || die "VERSION is invalid" 70
  render_managed_block "$target_id" "$OAW_POLICY_DESTINATION" "$OAW_OPERATION_TEMP/block-$target_id"
  render_file_with_block "$target_path" "$OAW_OPERATION_TEMP/block-$target_id" \
    "$OAW_OPERATION_TEMP/target-$target_id"
  block_checksum=$(checksum_file "$OAW_OPERATION_TEMP/block-$target_id")
  target_mode=$(target_ownership "$target_id")
  printf '%s\t%s\t%s\t%s\t%s\n' \
    "$target_id" "$target_path" "$target_mode" "$block_checksum" "$target_origin" \
    >>"$OAW_OPERATION_TEMP/selected-records"
}

write_operation_state() {
  local source_version=$1
  local target_records=$2
  local policy_checksum=

  policy_checksum=$(checksum_file "$OAW_OPERATION_TEMP/policy")
  write_state_file "$OAW_OPERATION_TEMP/state" "$source_version" user \
    "$OAW_POLICY_DESTINATION" "$policy_checksum" "$target_records"
}

verify_installed_policy_state() {
  local actual_policy_checksum=

  [ "$STATE_SCOPE" = user ] || die "installed scope does not match user scope" 65
  [ "$STATE_POLICY_PATH" = "$OAW_POLICY_DESTINATION" ] ||
    die "installed policy path does not match" 65
  [ -f "$STATE_POLICY_PATH" ] || die "managed policy is missing" 65
  actual_policy_checksum=$(checksum_file "$STATE_POLICY_PATH")
  [ "$actual_policy_checksum" = "$STATE_POLICY_CHECKSUM" ] ||
    die "managed policy has drifted" 65
}

verify_installed_target_record() {
  local target_id=$1
  local target_path=$2
  local target_mode=$3
  local target_checksum=$4
  local expected_target_path=
  local actual_block_checksum=
  local target_status=

  expected_target_path=$(target_destination "$target_id")
  [ "$target_path" = "$expected_target_path" ] || die "installed target path does not match" 65
  [ "$target_mode" = "$(target_ownership "$target_id")" ] ||
    die "installed target ownership does not match" 65
  target_status=$(managed_block_status "$target_path")
  [ "$target_status" = present ] || die "managed target block has drifted" 65
  extract_managed_block "$target_path" "$OAW_OPERATION_TEMP/current-block-$target_id"
  actual_block_checksum=$(checksum_file "$OAW_OPERATION_TEMP/current-block-$target_id")
  [ "$actual_block_checksum" = "$target_checksum" ] ||
    die "managed target block has drifted" 65
}

verify_installed_target_records() {
  local target_records=$1
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=

  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    verify_installed_target_record "$target_id" "$target_path" "$target_mode" "$target_checksum"
  done <"$target_records"
}

verify_current_target_state() {
  local expected_target_id=$1
  local expected_target_path=$2

  load_state_file "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/existing-records" ||
    die "no installation state; run install first" 66
  [ "$STATE_TARGET_COUNT" -eq 1 ] ||
    die "Ticket 03 lifecycle operations do not yet support multi-target state" 69
  load_target_record "$OAW_OPERATION_TEMP/existing-records" "$expected_target_id" ||
    die "installed target ID does not match" 65
  [ "$STATE_TARGET_PATH" = "$expected_target_path" ] || die "installed target path does not match" 65
  verify_installed_policy_state
  verify_installed_target_record "$STATE_TARGET_ID" "$STATE_TARGET_PATH" \
    "$STATE_TARGET_MODE" "$STATE_TARGET_CHECKSUM"
}

operation_install() {
  local selected_remaining=$OAW_SELECTED_TARGETS
  local selected_target=
  local target_path=
  local target_origin=
  local source_version=
  local target_status=
  local recorded_block_checksum=
  local selected_was_installed=0
  local tab=
  local target_id=
  local target_mode=
  local target_checksum=
  local extra=

  [ -r "$OAW_SOURCE_DIR/policy/ENGINEERING.md" ] || die "canonical policy source is not readable" 70
  init_oaw_paths
  prepare_operation_temp
  source_version=$(read_source_version)
  cp "$OAW_SOURCE_DIR/policy/ENGINEERING.md" "$OAW_OPERATION_TEMP/policy"
  : >"$OAW_OPERATION_TEMP/existing-records"
  : >"$OAW_OPERATION_TEMP/selected-records"

  if [ -f "$OAW_STATE_FILE" ]; then
    load_state_file "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/existing-records"
    verify_installed_policy_state
    verify_installed_target_records "$OAW_OPERATION_TEMP/existing-records"
    if ! files_equal "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" ||
      [ "$(checksum_file "$OAW_OPERATION_TEMP/policy")" != "$STATE_POLICY_CHECKSUM" ] ||
      [ "$source_version" != "$STATE_VERSION" ]; then
      die "installed content differs from this checkout; run update" 65
    fi
  fi

  while [ -n "$selected_remaining" ]; do
    case "$selected_remaining" in
      *,*)
        selected_target=${selected_remaining%%,*}
        selected_remaining=${selected_remaining#*,}
        ;;
      *)
        selected_target=$selected_remaining
        selected_remaining=
        ;;
    esac

    target_path=$(target_destination "$selected_target")
    target_origin=existing-file
    selected_was_installed=0
    recorded_block_checksum=
    if target_record_exists "$OAW_OPERATION_TEMP/existing-records" "$selected_target"; then
      load_target_record "$OAW_OPERATION_TEMP/existing-records" "$selected_target"
      [ "$STATE_TARGET_PATH" = "$target_path" ] || die "installed target path does not match" 65
      target_origin=$STATE_TARGET_ORIGIN
      recorded_block_checksum=$STATE_TARGET_CHECKSUM
      selected_was_installed=1
    else
      target_status=$(managed_block_status "$target_path")
      [ "$target_status" = absent ] || die "untracked OAW markers already exist: $target_path" 65
      [ -e "$target_path" ] || target_origin=created-file
    fi

    render_target_artifacts "$selected_target" "$target_path" "$target_origin" "$source_version"
    if [ "$selected_was_installed" -eq 1 ] &&
      [ "$(checksum_file "$OAW_OPERATION_TEMP/block-$selected_target")" != "$recorded_block_checksum" ]; then
      die "installed content differs from this checkout; run update" 65
    fi
  done

  merge_install_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-records" "$OAW_OPERATION_TEMP/merged-records"
  write_operation_state "$source_version" "$OAW_OPERATION_TEMP/merged-records"

  apply_replace policy "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" 600
  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    [ -z "$extra" ] || die "invalid selected target record" 65
    apply_replace "$target_id" "$OAW_OPERATION_TEMP/target-$target_id" "$target_path" 644
  done <"$OAW_OPERATION_TEMP/selected-records"
  apply_replace state "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
}

operation_update() {
  local target_path=
  local source_version=

  [ "$OAW_SELECTED_TARGETS" = claude ] ||
    die "Ticket 02 update supports only target 'claude'" 69
  [ -r "$OAW_SOURCE_DIR/policy/ENGINEERING.md" ] ||
    die "canonical policy source is not readable" 70
  init_oaw_paths
  prepare_operation_temp
  target_path=$(target_destination claude)
  source_version=$(read_source_version)

  verify_current_target_state claude "$target_path"
  cp "$OAW_SOURCE_DIR/policy/ENGINEERING.md" "$OAW_OPERATION_TEMP/policy"
  : >"$OAW_OPERATION_TEMP/selected-records"
  render_target_artifacts claude "$target_path" "$STATE_TARGET_ORIGIN" "$source_version"
  write_operation_state "$source_version" "$OAW_OPERATION_TEMP/selected-records"

  apply_replace policy "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" 600
  apply_replace claude "$OAW_OPERATION_TEMP/target-claude" "$target_path" 644
  apply_replace state "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
}

operation_uninstall() {
  local target_path=
  local target_status=
  local target_origin=
  local policy_checksum=
  local reference_status=0
  local retain_policy=0

  [ "$OAW_SCOPE" = user ] && [ "$OAW_SELECTED_TARGETS" = claude ] ||
    die "Ticket 02 uninstall supports only target 'claude'" 69
  init_oaw_paths
  target_path=$(target_destination claude)

  if [ ! -f "$OAW_STATE_FILE" ]; then
    target_status=$(managed_block_status "$target_path")
    [ "$target_status" = absent ] ||
      die "untracked OAW markers already exist: $target_path" 65
    note "unchanged: claude"
    return 0
  fi

  prepare_operation_temp
  verify_current_target_state claude "$target_path"
  target_origin=$STATE_TARGET_ORIGIN
  policy_checksum=$STATE_POLICY_CHECKSUM
  render_file_without_block "$target_path" "$OAW_OPERATION_TEMP/target"

  if other_state_references_policy "$OAW_INSTALLATIONS_DIR" "$OAW_STATE_FILE" \
    "$OAW_POLICY_DESTINATION" "$policy_checksum"; then
    retain_policy=1
  else
    reference_status=$?
    [ "$reference_status" -eq 1 ] || exit "$reference_status"
  fi

  if [ "$target_origin" = created-file ] && [ ! -s "$OAW_OPERATION_TEMP/target" ]; then
    apply_remove "$target_path"
  else
    apply_replace claude "$OAW_OPERATION_TEMP/target" "$target_path" 644
  fi
  if [ "$retain_policy" -eq 0 ]; then
    apply_remove "$OAW_POLICY_DESTINATION"
  fi
  apply_remove "$OAW_STATE_FILE"
}
