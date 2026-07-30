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
  render_managed_block "$target_id" "$OAW_POLICY_DESTINATION" \
    "$OAW_OPERATION_TEMP/block-$target_id" "$OAW_SCOPE"
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
  write_state_file "$OAW_OPERATION_TEMP/state" "$source_version" "$OAW_SCOPE" \
    "$OAW_PROJECT_ROOT" "$OAW_POLICY_DESTINATION" "$policy_checksum" "$target_records"
}

write_selected_target_ids() {
  local output_file=$1
  local selected_remaining=$OAW_SELECTED_TARGETS
  local selected_target=

  : >"$output_file"
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
    printf '%s\n' "$selected_target" >>"$output_file"
  done
}

add_target_action() {
  local actions_file=$1
  local action_type=$2
  local action_label=$3
  local source_file=$4
  local destination_file=$5
  local destination_mode=$6
  local existing_type=
  local existing_source=
  local existing_mode=

  if ! state_field_is_safe "$action_type" || ! state_field_is_safe "$action_label" ||
    ! state_field_is_safe "$source_file" || ! state_field_is_safe "$destination_file" ||
    ! state_field_is_safe "$destination_mode"; then
    die "target action cannot be serialized" 65
  fi
  existing_type=$(awk -F '\t' -v destination="$destination_file" \
    '$4 == destination { print $1; exit }' "$actions_file")
  if [ -n "$existing_type" ]; then
    [ "$existing_type" = "$action_type" ] ||
      die "conflicting target actions for $destination_file" 65
    if [ "$action_type" = replace ]; then
      existing_source=$(awk -F '\t' -v destination="$destination_file" \
        '$4 == destination { print $3; exit }' "$actions_file")
      existing_mode=$(awk -F '\t' -v destination="$destination_file" \
        '$4 == destination { print $5; exit }' "$actions_file")
      if [ "$existing_mode" != "$destination_mode" ] ||
        ! files_equal "$existing_source" "$source_file"; then
        die "conflicting target renders for $destination_file" 65
      fi
    fi
    return 0
  fi

  printf '%s\t%s\t%s\t%s\t%s\n' \
    "$action_type" "$action_label" "$source_file" "$destination_file" "$destination_mode" \
    >>"$actions_file"
}

apply_target_actions() {
  local actions_file=$1
  local tab=
  local action_type=
  local action_label=
  local source_file=
  local destination_file=
  local destination_mode=
  local extra=

  tab=$(printf '\t')
  while IFS="$tab" read -r action_type action_label source_file destination_file destination_mode extra; do
    [ -z "$extra" ] || die "invalid target action" 65
    case "$action_type" in
      replace)
        apply_replace "$action_label" "$source_file" "$destination_file" "$destination_mode"
        ;;
      remove)
        apply_remove "$destination_file"
        ;;
      *) die "invalid target action type: $action_type" 65 ;;
    esac
  done <"$actions_file"
}

verify_installed_policy_state() {
  local actual_policy_checksum=

  [ "$STATE_SCOPE" = "$OAW_SCOPE" ] || die "installed scope does not match" 65
  case "$OAW_SCOPE" in
    user)
      [ -z "$STATE_PROJECT_ROOT" ] || die "installed project root does not match" 65
      ;;
    project)
      [ "$STATE_PROJECT_ROOT" = "$OAW_PROJECT_ROOT" ] ||
        die "installed project root does not match" 65
      ;;
  esac
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

operation_install() {
  local selected_remaining=$OAW_SELECTED_TARGETS
  local selected_target=
  local target_path=
  local target_origin=
  local source_version=
  local target_status=
  local recorded_block_checksum=
  local selected_was_installed=0

  [ -r "$OAW_SOURCE_DIR/policy/ENGINEERING.md" ] || die "canonical policy source is not readable" 70
  init_oaw_paths
  prepare_operation_temp
  source_version=$(read_source_version)
  cp "$OAW_SOURCE_DIR/policy/ENGINEERING.md" "$OAW_OPERATION_TEMP/policy"
  : >"$OAW_OPERATION_TEMP/existing-records"
  : >"$OAW_OPERATION_TEMP/selected-records"
  : >"$OAW_OPERATION_TEMP/target-actions"

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
    add_target_action "$OAW_OPERATION_TEMP/target-actions" replace "$selected_target" \
      "$OAW_OPERATION_TEMP/target-$selected_target" "$target_path" 644
    if [ "$selected_was_installed" -eq 1 ] &&
      [ "$(checksum_file "$OAW_OPERATION_TEMP/block-$selected_target")" != "$recorded_block_checksum" ]; then
      die "installed content differs from this checkout; run update" 65
    fi
  done

  merge_install_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-records" "$OAW_OPERATION_TEMP/merged-records" "$OAW_SCOPE"
  normalize_destination_checksums "$OAW_OPERATION_TEMP/merged-records" \
    "$OAW_OPERATION_TEMP/selected-records" "$OAW_OPERATION_TEMP/final-records" "$OAW_SCOPE"
  write_operation_state "$source_version" "$OAW_OPERATION_TEMP/final-records"

  apply_replace policy "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" 600
  apply_target_actions "$OAW_OPERATION_TEMP/target-actions"
  apply_replace state "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
}

operation_update() {
  local source_version=
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=

  [ "$OAW_SCOPE" = user ] || die "Ticket 03 update supports only user scope" 69
  [ -r "$OAW_SOURCE_DIR/policy/ENGINEERING.md" ] ||
    die "canonical policy source is not readable" 70
  init_oaw_paths
  prepare_operation_temp
  source_version=$(read_source_version)
  cp "$OAW_SOURCE_DIR/policy/ENGINEERING.md" "$OAW_OPERATION_TEMP/policy"
  : >"$OAW_OPERATION_TEMP/existing-records"
  : >"$OAW_OPERATION_TEMP/selected-records"
  : >"$OAW_OPERATION_TEMP/target-actions"
  write_selected_target_ids "$OAW_OPERATION_TEMP/selected-ids"

  load_state_file "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/existing-records" ||
    die "no installation state; run install first" 66
  verify_installed_policy_state
  verify_installed_target_records "$OAW_OPERATION_TEMP/existing-records"
  select_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-ids" "$OAW_OPERATION_TEMP/selected-installed-records" \
    "$OAW_SCOPE"

  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    render_target_artifacts "$target_id" "$target_path" "$target_origin" "$source_version"
    add_target_action "$OAW_OPERATION_TEMP/target-actions" replace "$target_id" \
      "$OAW_OPERATION_TEMP/target-$target_id" "$target_path" 644
  done <"$OAW_OPERATION_TEMP/selected-installed-records"

  merge_install_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-records" "$OAW_OPERATION_TEMP/merged-records" "$OAW_SCOPE"
  normalize_destination_checksums "$OAW_OPERATION_TEMP/merged-records" \
    "$OAW_OPERATION_TEMP/selected-records" "$OAW_OPERATION_TEMP/final-records" "$OAW_SCOPE"
  write_operation_state "$source_version" "$OAW_OPERATION_TEMP/final-records"

  apply_replace policy "$OAW_OPERATION_TEMP/policy" "$OAW_POLICY_DESTINATION" 600
  apply_target_actions "$OAW_OPERATION_TEMP/target-actions"
  apply_replace state "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
}

operation_uninstall() {
  local tab=
  local selected_target=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_status=
  local target_origin=
  local extra=
  local reference_status=0
  local retain_policy=0

  [ "$OAW_SCOPE" = user ] || die "Ticket 03 uninstall supports only user scope" 69
  init_oaw_paths
  prepare_operation_temp
  : >"$OAW_OPERATION_TEMP/target-actions"
  write_selected_target_ids "$OAW_OPERATION_TEMP/selected-ids"

  if [ ! -f "$OAW_STATE_FILE" ]; then
    while IFS= read -r selected_target; do
      target_path=$(target_destination "$selected_target")
      target_status=$(managed_block_status "$target_path")
      [ "$target_status" = absent ] ||
        die "untracked OAW markers already exist: $target_path" 65
    done <"$OAW_OPERATION_TEMP/selected-ids"
    while IFS= read -r selected_target; do
      note "unchanged: $selected_target"
    done <"$OAW_OPERATION_TEMP/selected-ids"
    return 0
  fi

  load_state_file "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/existing-records"
  verify_installed_policy_state
  verify_installed_target_records "$OAW_OPERATION_TEMP/existing-records"
  filter_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-ids" "$OAW_OPERATION_TEMP/remaining-records" \
    "$OAW_OPERATION_TEMP/removed-records" "$OAW_SCOPE"

  while IFS= read -r selected_target; do
    if ! target_record_exists "$OAW_OPERATION_TEMP/existing-records" "$selected_target"; then
      note "unchanged: $selected_target"
    fi
  done <"$OAW_OPERATION_TEMP/selected-ids"

  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    if ! target_path_is_referenced "$OAW_OPERATION_TEMP/remaining-records" "$target_path"; then
      render_file_without_block "$target_path" "$OAW_OPERATION_TEMP/uninstall-target-$target_id"
      if [ "$target_origin" = created-file ] &&
        [ ! -s "$OAW_OPERATION_TEMP/uninstall-target-$target_id" ]; then
        add_target_action "$OAW_OPERATION_TEMP/target-actions" remove "$target_id" - "$target_path" -
      else
        add_target_action "$OAW_OPERATION_TEMP/target-actions" replace "$target_id" \
          "$OAW_OPERATION_TEMP/uninstall-target-$target_id" "$target_path" 644
      fi
    fi
  done <"$OAW_OPERATION_TEMP/removed-records"

  if [ -s "$OAW_OPERATION_TEMP/remaining-records" ]; then
    write_state_file "$OAW_OPERATION_TEMP/state" "$STATE_VERSION" "$OAW_SCOPE" \
      "$OAW_PROJECT_ROOT" "$STATE_POLICY_PATH" "$STATE_POLICY_CHECKSUM" \
      "$OAW_OPERATION_TEMP/remaining-records"
  else
    if other_state_references_policy "$OAW_INSTALLATIONS_DIR" "$OAW_STATE_FILE" \
      "$OAW_POLICY_DESTINATION" "$STATE_POLICY_CHECKSUM"; then
      retain_policy=1
    else
      reference_status=$?
      [ "$reference_status" -eq 1 ] || exit "$reference_status"
    fi
  fi

  apply_target_actions "$OAW_OPERATION_TEMP/target-actions"
  if [ -s "$OAW_OPERATION_TEMP/remaining-records" ]; then
    apply_replace state "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
  else
    if [ "$retain_policy" -eq 0 ]; then
      apply_remove "$OAW_POLICY_DESTINATION"
    fi
    apply_remove "$OAW_STATE_FILE"
  fi
}
