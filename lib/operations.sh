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

render_claude_artifacts() {
  local target_path=$1
  local target_origin=$2
  local source_version=$3
  local policy_checksum=
  local block_checksum=

  cp "$OAW_SOURCE_DIR/policy/ENGINEERING.md" "$OAW_OPERATION_TEMP/policy"
  render_managed_block claude "$OAW_POLICY_DESTINATION" "$OAW_OPERATION_TEMP/block"
  render_file_with_block "$target_path" "$OAW_OPERATION_TEMP/block" "$OAW_OPERATION_TEMP/target"
  policy_checksum=$(checksum_file "$OAW_OPERATION_TEMP/policy")
  block_checksum=$(checksum_file "$OAW_OPERATION_TEMP/block")
  write_state_file "$OAW_OPERATION_TEMP/state" "$source_version" user \
    "$OAW_POLICY_DESTINATION" "$policy_checksum" claude "$target_path" \
    "$block_checksum" "$target_origin"
}

verify_current_claude_state() {
  local expected_target=$1
  local actual_policy_checksum=
  local actual_block_checksum=
  local target_status=

  load_state_file "$OAW_STATE_FILE" || die "no installation state; run install first" 66
  [ "$STATE_SCOPE" = user ] || die "installed scope does not match user scope" 65
  [ "$STATE_POLICY_PATH" = "$OAW_POLICY_DESTINATION" ] || die "installed policy path does not match" 65
  [ "$STATE_TARGET_PATH" = "$expected_target" ] || die "installed target path does not match" 65
  [ -f "$STATE_POLICY_PATH" ] || die "managed policy is missing" 65
  actual_policy_checksum=$(checksum_file "$STATE_POLICY_PATH")
  [ "$actual_policy_checksum" = "$STATE_POLICY_CHECKSUM" ] || die "managed policy has drifted" 65

  target_status=$(managed_block_status "$expected_target")
  [ "$target_status" = present ] || die "managed target block has drifted" 65
  extract_managed_block "$expected_target" "$OAW_OPERATION_TEMP/current-block"
  actual_block_checksum=$(checksum_file "$OAW_OPERATION_TEMP/current-block")
  [ "$actual_block_checksum" = "$STATE_TARGET_CHECKSUM" ] || die "managed target block has drifted" 65
}

operation_install() {
  local target_path=
  local target_origin=existing-file
  local source_version=
  local target_status=

  [ "$OAW_SELECTED_TARGETS" = claude ] || die "Ticket 02 install supports only target 'claude'" 69
  [ -r "$OAW_SOURCE_DIR/policy/ENGINEERING.md" ] || die "canonical policy source is not readable" 70
  init_oaw_paths
  prepare_operation_temp
  target_path=$(target_destination claude)
  source_version=$(read_source_version)

  if [ -f "$OAW_STATE_FILE" ]; then
    verify_current_claude_state "$target_path"
    target_origin=$STATE_TARGET_ORIGIN
    render_claude_artifacts "$target_path" "$target_origin" "$source_version"
    if ! files_equal "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" ||
      [ "$(checksum_file "$OAW_OPERATION_TEMP/block")" != "$STATE_TARGET_CHECKSUM" ] ||
      [ "$source_version" != "$STATE_VERSION" ]; then
      die "installed content differs from this checkout; run update" 65
    fi
  else
    target_status=$(managed_block_status "$target_path")
    [ "$target_status" = absent ] || die "untracked OAW markers already exist: $target_path" 65
    [ -e "$target_path" ] || target_origin=created-file
    render_claude_artifacts "$target_path" "$target_origin" "$source_version"
  fi

  apply_replace policy "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" 600
  apply_replace claude "$OAW_OPERATION_TEMP/target" "$target_path" 644
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

  verify_current_claude_state "$target_path"
  render_claude_artifacts "$target_path" "$STATE_TARGET_ORIGIN" "$source_version"

  apply_replace policy "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" 600
  apply_replace claude "$OAW_OPERATION_TEMP/target" "$target_path" 644
  apply_replace state "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
}
