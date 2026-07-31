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

checksum_rendered_target_artifact() {
  local target_id=$1
  local target_mode=$2

  case "$target_mode" in
    managed-block) checksum_file "$OAW_OPERATION_TEMP/block-$target_id" ;;
    owned-file) checksum_file "$OAW_OPERATION_TEMP/target-$target_id" ;;
    *) die "unknown target ownership mode: $target_mode" 65 ;;
  esac
}

render_target_artifacts() {
  local target_id=$1
  local target_path=$2
  local target_origin=$3
  local source_version=$4
  local target_checksum=
  local target_mode=
  local current_target_path=$target_path

  [ -n "$source_version" ] || die "VERSION is invalid" 70
  target_mode=$(target_ownership "$target_id")
  case "$target_mode" in
    managed-block)
      if [ -f "$OAW_OPERATION_TEMP/forced-current-$target_id" ]; then
        current_target_path=$OAW_OPERATION_TEMP/forced-current-$target_id
      fi
      render_managed_block "$target_id" "$OAW_POLICY_DESTINATION" \
        "$OAW_OPERATION_TEMP/block-$target_id" "$OAW_SCOPE"
      render_file_with_block "$current_target_path" "$OAW_OPERATION_TEMP/block-$target_id" \
        "$OAW_OPERATION_TEMP/target-$target_id"
      ;;
    owned-file)
      render_target_content "$target_id" "$OAW_POLICY_DESTINATION" "$OAW_SCOPE" \
        >"$OAW_OPERATION_TEMP/target-$target_id"
      ;;
    *) die "unknown target ownership mode: $target_mode" 65 ;;
  esac
  target_checksum=$(checksum_rendered_target_artifact "$target_id" "$target_mode")
  printf '%s\t%s\t%s\t%s\t%s\n' \
    "$target_id" "$target_path" "$target_mode" "$target_checksum" "$target_origin" \
    >>"$OAW_OPERATION_TEMP/selected-records"
}

write_operation_state() {
  local source_version=$1
  local target_records=$2
  local directory_records=$3
  local policy_checksum=
  local backup_path=${OAW_OPERATION_BACKUP_PATH:-$STATE_BACKUP_PATH}

  policy_checksum=$(checksum_file "$OAW_OPERATION_TEMP/policy")
  write_state_file "$OAW_OPERATION_TEMP/state" "$source_version" "$OAW_SCOPE" \
    "$OAW_PROJECT_ROOT" "$OAW_POLICY_DESTINATION" "$policy_checksum" \
    "$target_records" "$backup_path" "$directory_records"
}

verify_policy_state_binding() {
  local candidate_state=$1
  local target_records=$2
  local expected_state=
  local expected_target_path=
  local project_identity=
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=

  destination_relative_suffix "$OAW_XDG_STATE_HOME" "$candidate_state" >/dev/null || return $?

  case "$STATE_SCOPE" in
    user)
      expected_state=$OAW_INSTALLATIONS_DIR/user.state
      [ "$candidate_state" = "$expected_state" ] ||
        die "installed user state path does not match" 65
      ;;
    project)
      project_identity=$(project_state_identity "$STATE_PROJECT_ROOT")
      expected_state=$OAW_INSTALLATIONS_DIR/projects/$project_identity.state
      [ "$candidate_state" = "$expected_state" ] ||
        die "installed project root does not match" 65
      ;;
    *) die "installed scope does not match" 65 ;;
  esac

  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    expected_target_path=$(
      OAW_SCOPE=$STATE_SCOPE
      OAW_PROJECT_ROOT=$STATE_PROJECT_ROOT
      target_destination "$target_id"
    )
    [ "$target_path" = "$expected_target_path" ] ||
      die "installed target path does not match" 65
  done <"$target_records"
}

prepare_policy_state_actions() {
  local source_version=$1
  local new_policy_checksum=$2
  local excluded_state=$3
  local actions_file=$4
  local installed_policy_checksum=
  local candidate_state=
  local candidate_records=
  local candidate_output=
  local candidate_index=0

  if [ -f "$OAW_POLICY_DESTINATION" ]; then
    installed_policy_checksum=$(checksum_file "$OAW_POLICY_DESTINATION")
  fi

  for candidate_state in \
    "$OAW_INSTALLATIONS_DIR"/*.state \
    "$OAW_INSTALLATIONS_DIR"/projects/*.state; do
    [ -e "$candidate_state" ] || continue
    [ "$candidate_state" = "$excluded_state" ] && continue
    candidate_index=$((candidate_index + 1))
    candidate_records="$OAW_OPERATION_TEMP/policy-records-$candidate_index"
    candidate_output="$OAW_OPERATION_TEMP/policy-state-$candidate_index"
    load_state_file "$candidate_state" "$candidate_records" \
      "$candidate_records.directories"
    [ "$STATE_POLICY_PATH" = "$OAW_POLICY_DESTINATION" ] || continue
    verify_policy_state_binding "$candidate_state" "$candidate_records"
    verify_state_target_records "$candidate_records" "$STATE_SCOPE" "$STATE_PROJECT_ROOT"
    verify_owned_directory_records "$candidate_records.directories" \
      "$candidate_records" "$STATE_SCOPE" "$STATE_PROJECT_ROOT"
    if [ -n "$OAW_FORCE_POLICY_BASELINE" ]; then
      installed_policy_checksum=$OAW_FORCE_POLICY_BASELINE
    fi
    [ -n "$installed_policy_checksum" ] &&
      [ "$STATE_POLICY_CHECKSUM" = "$installed_policy_checksum" ] ||
      die "managed policy has drifted" 65
    write_state_file "$candidate_output" "$source_version" "$STATE_SCOPE" \
      "$STATE_PROJECT_ROOT" "$STATE_POLICY_PATH" "$new_policy_checksum" \
      "$candidate_records" "${OAW_OPERATION_BACKUP_PATH:-$STATE_BACKUP_PATH}" \
      "$candidate_records.directories"
    add_target_action "$actions_file" replace "state-reference-$candidate_index" \
      "$candidate_output" "$candidate_state" 600
  done
}

other_live_state_references_policy() {
  local installations_dir=$1
  local excluded_state=$2
  local policy_path=$3
  local candidate_state=
  local candidate_records=
  local candidate_index=0
  local found_reference=0

  for candidate_state in \
    "$installations_dir"/*.state \
    "$installations_dir"/projects/*.state; do
    [ -e "$candidate_state" ] || continue
    [ "$candidate_state" = "$excluded_state" ] && continue
    candidate_index=$((candidate_index + 1))
    candidate_records="$OAW_OPERATION_TEMP/retention-records-$candidate_index"
    load_state_file "$candidate_state" "$candidate_records" \
      "$candidate_records.directories"
    [ "$STATE_POLICY_PATH" = "$policy_path" ] || continue
    verify_policy_state_binding "$candidate_state" "$candidate_records"
    verify_state_target_records "$candidate_records" "$STATE_SCOPE" "$STATE_PROJECT_ROOT"
    verify_owned_directory_records "$candidate_records.directories" \
      "$candidate_records" "$STATE_SCOPE" "$STATE_PROJECT_ROOT"
    found_reference=1
  done

  [ "$found_reference" -eq 1 ]
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

verify_untracked_selected_markers() {
  local selected_ids=$1
  local selected_target=
  local target_mode=
  local target_path=
  local target_status=

  while IFS= read -r selected_target; do
    target_mode=$(target_ownership "$selected_target")
    [ "$target_mode" = managed-block ] || continue
    target_path=$(target_destination "$selected_target")
    target_status=$(managed_block_status "$target_path")
    [ "$target_status" = absent ] ||
      die "untracked OAW markers already exist: $selected_target at $target_path" 65
  done <"$selected_ids"
}

prepare_action_coordinates() {
  local action_label=$1
  local destination_file=$2
  local rebuilt_destination=

  case "$action_label" in
    state|state-reference-*)
      OAW_ACTION_ALLOWED_ROOT=$OAW_XDG_STATE_HOME
      OAW_ACTION_RELATIVE_SUFFIX=$(
        destination_relative_suffix "$OAW_ACTION_ALLOWED_ROOT" "$destination_file"
      ) || return $?
      ;;
    *)
      target_is_known "$action_label" || die "unknown target action label: $action_label" 65
      OAW_ACTION_ALLOWED_ROOT=$(target_allowed_root "$action_label") || return $?
      OAW_ACTION_RELATIVE_SUFFIX=$(target_relative_suffix "$action_label") || return $?
      rebuilt_destination=$(
        validated_destination_path "$OAW_ACTION_ALLOWED_ROOT" "$OAW_ACTION_RELATIVE_SUFFIX"
      ) || return $?
      [ "$rebuilt_destination" = "$destination_file" ] ||
        die "target action destination does not match registry: $destination_file" 65
      ;;
  esac
}

apply_scoped_replace() {
  local action_label=$1
  local source_file=$2
  local allowed_root=$3
  local destination_file=$4
  local destination_mode=$5
  local relative_suffix=

  relative_suffix=$(destination_relative_suffix "$allowed_root" "$destination_file") ||
    return $?
  apply_replace "$action_label" "$source_file" "$allowed_root" \
    "$relative_suffix" "$destination_file" "$destination_mode"
}

apply_scoped_remove() {
  local allowed_root=$1
  local destination_file=$2
  local relative_suffix=

  relative_suffix=$(destination_relative_suffix "$allowed_root" "$destination_file") ||
    return $?
  apply_remove "$allowed_root" "$relative_suffix" "$destination_file"
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
  local existing_root=
  local existing_suffix=

  OAW_ACTION_ALLOWED_ROOT=
  OAW_ACTION_RELATIVE_SUFFIX=
  prepare_action_coordinates "$action_label" "$destination_file" || return $?

  if ! state_field_is_safe "$action_type" || ! state_field_is_safe "$action_label" ||
    ! state_field_is_safe "$source_file" || ! state_field_is_safe "$destination_file" ||
    ! state_field_is_safe "$destination_mode" ||
    ! state_field_is_safe "$OAW_ACTION_ALLOWED_ROOT" ||
    ! state_field_is_safe "$OAW_ACTION_RELATIVE_SUFFIX"; then
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
      existing_root=$(awk -F '\t' -v destination="$destination_file" \
        '$4 == destination { print $6; exit }' "$actions_file")
      existing_suffix=$(awk -F '\t' -v destination="$destination_file" \
        '$4 == destination { print $7; exit }' "$actions_file")
      if [ "$existing_mode" != "$destination_mode" ] ||
        [ "$existing_root" != "$OAW_ACTION_ALLOWED_ROOT" ] ||
        [ "$existing_suffix" != "$OAW_ACTION_RELATIVE_SUFFIX" ] ||
        ! files_equal "$existing_source" "$source_file"; then
        die "conflicting target renders for $destination_file" 65
      fi
    fi
    return 0
  fi

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$action_type" "$action_label" "$source_file" "$destination_file" \
    "$destination_mode" "$OAW_ACTION_ALLOWED_ROOT" "$OAW_ACTION_RELATIVE_SUFFIX" \
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
  local allowed_root=
  local relative_suffix=
  local extra=

  tab=$(printf '\t')
  while IFS="$tab" read -r action_type action_label source_file destination_file destination_mode \
    allowed_root relative_suffix extra; do
    [ -z "$extra" ] || die "invalid target action" 65
    case "$action_type" in
      replace)
        apply_replace "$action_label" "$source_file" "$allowed_root" \
          "$relative_suffix" "$destination_file" "$destination_mode"
        ;;
      remove)
        apply_remove "$allowed_root" "$relative_suffix" "$destination_file"
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

verify_state_target_record() {
  local record_scope=$1
  local record_project_root=$2
  local target_id=$3
  local target_path=$4
  local target_mode=$5
  local target_checksum=$6
  local expected_target_path=
  local actual_target_checksum=
  local target_status=

  expected_target_path=$(
    OAW_SCOPE=$record_scope
    OAW_PROJECT_ROOT=$record_project_root
    target_destination "$target_id"
  )
  [ "$target_path" = "$expected_target_path" ] ||
    die "installed target path does not match: $target_id at $target_path" 65
  [ "$target_mode" = "$(target_ownership "$target_id")" ] ||
    die "installed target ownership does not match: $target_id at $target_path" 65
  case "$target_mode" in
    managed-block)
      target_status=$(managed_block_status "$target_path")
      [ "$target_status" = present ] ||
        die "managed target block has drifted: $target_id at $target_path" 65
      extract_managed_block "$target_path" "$OAW_OPERATION_TEMP/current-block-$target_id"
      actual_target_checksum=$(checksum_file "$OAW_OPERATION_TEMP/current-block-$target_id")
      [ "$actual_target_checksum" = "$target_checksum" ] ||
        die "managed target block has drifted: $target_id at $target_path" 65
      ;;
    owned-file)
      [ -f "$target_path" ] ||
        die "owned target file has drifted: $target_id at $target_path" 65
      actual_target_checksum=$(checksum_file "$target_path")
      [ "$actual_target_checksum" = "$target_checksum" ] ||
        die "owned target file has drifted: $target_id at $target_path" 65
      ;;
    *) die "unknown target ownership mode: $target_mode" 65 ;;
  esac
}

verify_state_target_records() {
  local target_records=$1
  local record_scope=$2
  local record_project_root=$3
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=

  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    verify_state_target_record "$record_scope" "$record_project_root" \
      "$target_id" "$target_path" "$target_mode" "$target_checksum"
  done <"$target_records"
}

verify_installed_target_records() {
  local target_records=$1

  verify_state_target_records "$target_records" "$OAW_SCOPE" "$OAW_PROJECT_ROOT"
}

operation_install() {
  local selected_remaining=$OAW_SELECTED_TARGETS
  local selected_target=
  local target_path=
  local target_origin=
  local target_mode=
  local source_version=
  local target_status=
  local recorded_target_checksum=
  local rendered_target_checksum=
  local shared_destination_checksum=
  local selected_joins_shared=0
  local selected_was_installed=0
  local new_policy_checksum=

  [ -r "$OAW_SOURCE_DIR/policy/ENGINEERING.md" ] || die "canonical policy source is not readable" 70
  init_oaw_paths
  prepare_operation_temp
  source_version=$(read_source_version)
  cp "$OAW_SOURCE_DIR/policy/ENGINEERING.md" "$OAW_OPERATION_TEMP/policy"
  : >"$OAW_OPERATION_TEMP/existing-records"
  : >"$OAW_OPERATION_TEMP/selected-records"
  : >"$OAW_OPERATION_TEMP/state-actions"
  : >"$OAW_OPERATION_TEMP/target-actions"
  : >"$OAW_OPERATION_TEMP/directory-actions"
  : >"$OAW_OPERATION_TEMP/existing-directories"

  if [ -f "$OAW_STATE_FILE" ]; then
    load_state_file "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/existing-records" \
      "$OAW_OPERATION_TEMP/existing-directories"
    verify_installed_policy_state
    verify_installed_target_records "$OAW_OPERATION_TEMP/existing-records"
    verify_owned_directory_records "$OAW_OPERATION_TEMP/existing-directories" \
      "$OAW_OPERATION_TEMP/existing-records" "$OAW_SCOPE" "$OAW_PROJECT_ROOT"
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
    target_mode=$(target_ownership "$selected_target")
    target_origin=existing-file
    shared_destination_checksum=
    selected_joins_shared=0
    selected_was_installed=0
    recorded_target_checksum=
    if target_record_exists "$OAW_OPERATION_TEMP/existing-records" "$selected_target"; then
      load_target_record "$OAW_OPERATION_TEMP/existing-records" "$selected_target"
      [ "$STATE_TARGET_PATH" = "$target_path" ] || die "installed target path does not match" 65
      target_origin=$STATE_TARGET_ORIGIN
      recorded_target_checksum=$STATE_TARGET_CHECKSUM
      selected_was_installed=1
    else
      case "$target_mode" in
        managed-block)
          target_status=$(managed_block_status "$target_path")
          if target_path_is_referenced "$OAW_OPERATION_TEMP/existing-records" "$target_path"; then
            [ "$target_status" = present ] || die "managed target block has drifted" 65
            target_origin=$(destination_origin_from_records \
              "$OAW_OPERATION_TEMP/existing-records" "$target_path") ||
              die "conflicting target origins for $target_path" 65
            shared_destination_checksum=$(destination_checksum_from_records \
              "$OAW_OPERATION_TEMP/existing-records" "$target_path") ||
              die "conflicting target checksums for $target_path" 65
            selected_joins_shared=1
          else
            [ "$target_status" = absent ] ||
              die "untracked OAW markers already exist: $target_path" 65
            [ -e "$target_path" ] || target_origin=created-file
          fi
          ;;
        owned-file)
          [ ! -e "$target_path" ] || die "owned target already exists: $target_path" 65
          target_origin=created-file
          ;;
        *) die "unknown target ownership mode: $target_mode" 65 ;;
      esac
    fi

    render_target_artifacts "$selected_target" "$target_path" "$target_origin" "$source_version"
    add_target_action "$OAW_OPERATION_TEMP/target-actions" replace "$selected_target" \
      "$OAW_OPERATION_TEMP/target-$selected_target" "$target_path" 644
    rendered_target_checksum=$(checksum_rendered_target_artifact "$selected_target" "$target_mode")
    if [ "$selected_joins_shared" -eq 1 ] &&
      [ "$rendered_target_checksum" != "$shared_destination_checksum" ]; then
      die "conflicting target renders for $target_path" 65
    fi
    if [ "$selected_was_installed" -eq 1 ] &&
      [ "$rendered_target_checksum" != "$recorded_target_checksum" ]; then
      die "installed content differs from this checkout; run update" 65
    fi
  done

  merge_install_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-records" "$OAW_OPERATION_TEMP/merged-records" "$OAW_SCOPE"
  normalize_destination_checksums "$OAW_OPERATION_TEMP/merged-records" \
    "$OAW_OPERATION_TEMP/selected-records" "$OAW_OPERATION_TEMP/final-records" "$OAW_SCOPE"
  prepare_owned_directories "$OAW_OPERATION_TEMP/existing-directories" \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/final-records" \
    "$OAW_OPERATION_TEMP/final-directories" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
  write_operation_state "$source_version" "$OAW_OPERATION_TEMP/final-records" \
    "$OAW_OPERATION_TEMP/final-directories"
  new_policy_checksum=$(checksum_file "$OAW_OPERATION_TEMP/policy")
  add_target_action "$OAW_OPERATION_TEMP/state-actions" replace state \
    "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
  prepare_policy_state_actions "$source_version" "$new_policy_checksum" \
    "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/state-actions"

  verify_prepared_operation replace "$OAW_OPERATION_TEMP/policy" \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions" \
    "$OAW_OPERATION_TEMP/directory-actions" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
  apply_operation install replace "$OAW_OPERATION_TEMP/policy" \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions" \
    "$OAW_OPERATION_TEMP/directory-actions" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
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
  local new_policy_checksum=

  [ -r "$OAW_SOURCE_DIR/policy/ENGINEERING.md" ] ||
    die "canonical policy source is not readable" 70
  init_oaw_paths
  prepare_operation_temp
  source_version=$(read_source_version)
  cp "$OAW_SOURCE_DIR/policy/ENGINEERING.md" "$OAW_OPERATION_TEMP/policy"
  : >"$OAW_OPERATION_TEMP/existing-records"
  : >"$OAW_OPERATION_TEMP/selected-records"
  : >"$OAW_OPERATION_TEMP/state-actions"
  : >"$OAW_OPERATION_TEMP/target-actions"
  : >"$OAW_OPERATION_TEMP/directory-actions"
  : >"$OAW_OPERATION_TEMP/existing-directories"
  write_selected_target_ids "$OAW_OPERATION_TEMP/selected-ids"

  if [ ! -f "$OAW_STATE_FILE" ]; then
    verify_untracked_selected_markers "$OAW_OPERATION_TEMP/selected-ids"
    die "no installation state; run install first" 66
  fi
  load_state_file "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/existing-directories"
  verify_mutation_policy_state
  verify_mutation_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-ids"
  verify_owned_directory_records "$OAW_OPERATION_TEMP/existing-directories" \
    "$OAW_OPERATION_TEMP/existing-records" "$OAW_SCOPE" "$OAW_PROJECT_ROOT"
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
  prepare_owned_directories "$OAW_OPERATION_TEMP/existing-directories" \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/final-records" \
    "$OAW_OPERATION_TEMP/final-directories" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
  if [ "$OAW_FORCE_BACKUP_REQUIRED" -eq 1 ]; then
    reserve_operation_backup_path
  fi
  write_operation_state "$source_version" "$OAW_OPERATION_TEMP/final-records" \
    "$OAW_OPERATION_TEMP/final-directories"
  new_policy_checksum=$(checksum_file "$OAW_OPERATION_TEMP/policy")
  add_target_action "$OAW_OPERATION_TEMP/state-actions" replace state \
    "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
  prepare_policy_state_actions "$source_version" "$new_policy_checksum" \
    "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/state-actions"
  if [ "$OAW_FORCE_BACKUP_REQUIRED" -eq 1 ]; then
    perform_operation_backup update "$OAW_OPERATION_TEMP/policy" \
      "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions"
  fi

  verify_prepared_operation replace "$OAW_OPERATION_TEMP/policy" \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions" \
    "$OAW_OPERATION_TEMP/directory-actions" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
  apply_operation update replace "$OAW_OPERATION_TEMP/policy" \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions" \
    "$OAW_OPERATION_TEMP/directory-actions" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
}

operation_uninstall() {
  local tab=
  local selected_target=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=
  local reference_status=0
  local retain_policy=0
  local policy_action=retain

  init_oaw_paths
  prepare_operation_temp
  : >"$OAW_OPERATION_TEMP/target-actions"
  : >"$OAW_OPERATION_TEMP/state-actions"
  : >"$OAW_OPERATION_TEMP/directory-actions"
  : >"$OAW_OPERATION_TEMP/existing-directories"
  : >"$OAW_OPERATION_TEMP/planned-owned-directories"
  write_selected_target_ids "$OAW_OPERATION_TEMP/selected-ids"

  if [ ! -f "$OAW_STATE_FILE" ]; then
    verify_untracked_selected_markers "$OAW_OPERATION_TEMP/selected-ids"
    while IFS= read -r selected_target; do
      note "unchanged: $selected_target"
    done <"$OAW_OPERATION_TEMP/selected-ids"
    return 0
  fi

  load_state_file "$OAW_STATE_FILE" "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/existing-directories"
  verify_mutation_policy_state
  verify_mutation_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-ids"
  verify_owned_directory_records "$OAW_OPERATION_TEMP/existing-directories" \
    "$OAW_OPERATION_TEMP/existing-records" "$OAW_SCOPE" "$OAW_PROJECT_ROOT"
  filter_target_records "$OAW_OPERATION_TEMP/existing-records" \
    "$OAW_OPERATION_TEMP/selected-ids" "$OAW_OPERATION_TEMP/remaining-records" \
    "$OAW_OPERATION_TEMP/removed-records" "$OAW_SCOPE"
  partition_owned_directories "$OAW_OPERATION_TEMP/existing-directories" \
    "$OAW_OPERATION_TEMP/remaining-records" "$OAW_OPERATION_TEMP/remaining-directories" \
    "$OAW_OPERATION_TEMP/removed-directories"
  prepare_directory_removals "$OAW_OPERATION_TEMP/removed-directories" \
    "$OAW_OPERATION_TEMP/existing-records" "$OAW_OPERATION_TEMP/directory-actions"

  while IFS= read -r selected_target; do
    if ! target_record_exists "$OAW_OPERATION_TEMP/existing-records" "$selected_target"; then
      note "unchanged: $selected_target"
    fi
  done <"$OAW_OPERATION_TEMP/selected-ids"

  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    if ! target_path_is_referenced "$OAW_OPERATION_TEMP/remaining-records" "$target_path"; then
      case "$target_mode" in
        managed-block)
          if [ -f "$OAW_OPERATION_TEMP/forced-current-$target_id" ]; then
            render_file_without_block "$OAW_OPERATION_TEMP/forced-current-$target_id" \
              "$OAW_OPERATION_TEMP/uninstall-target-$target_id"
          else
            render_file_without_block "$target_path" \
              "$OAW_OPERATION_TEMP/uninstall-target-$target_id"
          fi
          if [ "$target_origin" = created-file ] &&
            [ ! -s "$OAW_OPERATION_TEMP/uninstall-target-$target_id" ]; then
            add_target_action "$OAW_OPERATION_TEMP/target-actions" remove "$target_id" - "$target_path" -
          else
            add_target_action "$OAW_OPERATION_TEMP/target-actions" replace "$target_id" \
              "$OAW_OPERATION_TEMP/uninstall-target-$target_id" "$target_path" 644
          fi
          ;;
        owned-file)
          [ "$target_origin" = created-file ] || die "invalid owned target origin" 65
          add_target_action "$OAW_OPERATION_TEMP/target-actions" remove "$target_id" - "$target_path" -
          ;;
        *) die "unknown target ownership mode: $target_mode" 65 ;;
      esac
    fi
  done <"$OAW_OPERATION_TEMP/removed-records"

  if [ -s "$OAW_OPERATION_TEMP/remaining-records" ]; then
    if [ "$OAW_FORCE_BACKUP_REQUIRED" -eq 1 ]; then
      reserve_operation_backup_path
    fi
    write_state_file "$OAW_OPERATION_TEMP/state" "$STATE_VERSION" "$OAW_SCOPE" \
      "$OAW_PROJECT_ROOT" "$STATE_POLICY_PATH" "$STATE_POLICY_CHECKSUM" \
      "$OAW_OPERATION_TEMP/remaining-records" \
      "${OAW_OPERATION_BACKUP_PATH:-$STATE_BACKUP_PATH}" \
      "$OAW_OPERATION_TEMP/remaining-directories"
    add_target_action "$OAW_OPERATION_TEMP/state-actions" replace state \
      "$OAW_OPERATION_TEMP/state" "$OAW_STATE_FILE" 600
  else
    if other_live_state_references_policy "$OAW_INSTALLATIONS_DIR" "$OAW_STATE_FILE" \
      "$OAW_POLICY_DESTINATION"; then
      retain_policy=1
    else
      reference_status=$?
      [ "$reference_status" -eq 1 ] || exit "$reference_status"
    fi
    if [ "$OAW_FORCE_BACKUP_REQUIRED" -eq 1 ]; then
      reserve_operation_backup_path
    fi
    add_target_action "$OAW_OPERATION_TEMP/state-actions" remove state - \
      "$OAW_STATE_FILE" -
  fi

  if [ "$OAW_FORCE_BACKUP_REQUIRED" -eq 1 ]; then
    if [ "$retain_policy" -eq 0 ] && [ ! -s "$OAW_OPERATION_TEMP/remaining-records" ]; then
      perform_operation_backup uninstall - "$OAW_OPERATION_TEMP/target-actions" \
        "$OAW_OPERATION_TEMP/state-actions"
    else
      perform_operation_backup uninstall "$OAW_POLICY_DESTINATION" \
        "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions"
    fi
  fi

  if [ ! -s "$OAW_OPERATION_TEMP/remaining-records" ] && [ "$retain_policy" -eq 0 ]; then
    policy_action=remove
  fi
  verify_prepared_operation "$policy_action" - \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions" \
    "$OAW_OPERATION_TEMP/directory-actions" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
  apply_operation uninstall "$policy_action" - \
    "$OAW_OPERATION_TEMP/target-actions" "$OAW_OPERATION_TEMP/state-actions" \
    "$OAW_OPERATION_TEMP/directory-actions" \
    "$OAW_OPERATION_TEMP/planned-owned-directories"
}
