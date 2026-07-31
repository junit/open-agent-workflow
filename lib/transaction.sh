#!/usr/bin/env bash

directory_is_oaw_namespace() {
  case "$1" in
    "$OAW_CONFIG_DIR"|"$OAW_STATE_DIR"|"$OAW_INSTALLATIONS_DIR"|"$OAW_INSTALLATIONS_DIR/projects")
      return 0
      ;;
    *) return 1 ;;
  esac
}

oaw_namespace_allowed_root() {
  case "$1" in
    "$OAW_CONFIG_DIR") printf '%s\n' "$OAW_XDG_CONFIG_HOME" ;;
    "$OAW_STATE_DIR"|"$OAW_INSTALLATIONS_DIR"|"$OAW_INSTALLATIONS_DIR/projects")
      printf '%s\n' "$OAW_XDG_STATE_HOME"
      ;;
    *) return 1 ;;
  esac
}

owned_directory_allowed_root() {
  local directory_path=$1
  local target_records=$2
  local record_scope=$3
  local record_project_root=$4
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=
  local allowed_root=

  if directory_is_oaw_namespace "$directory_path"; then
    allowed_root=$(oaw_namespace_allowed_root "$directory_path") || return $?
    destination_relative_suffix "$allowed_root" "$directory_path" >/dev/null ||
      return $?
    printf '%s\n' "$allowed_root"
    return 0
  fi

  tab=$(printf '\t')
  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    : "$target_mode" "$target_checksum" "$extra"
    [ "$target_origin" = created-file ] || continue
    allowed_root=$(
      OAW_SCOPE=$record_scope
      OAW_PROJECT_ROOT=$record_project_root
      target_allowed_root "$target_id"
    ) || return $?
    case "$directory_path:$target_path" in
      "$allowed_root"/*:"$directory_path"/*)
        destination_relative_suffix "$allowed_root" "$directory_path" >/dev/null ||
          return $?
        printf '%s\n' "$allowed_root"
        return 0
        ;;
    esac
  done <"$target_records"
  return 1
}

verify_owned_directory_records() {
  local directory_records=$1
  local target_records=$2
  local record_scope=$3
  local record_project_root=$4
  local directory_path=

  validate_directory_records "$directory_records"
  while IFS= read -r directory_path; do
    owned_directory_allowed_root "$directory_path" "$target_records" \
      "$record_scope" "$record_project_root" >/dev/null ||
      die "owned directory does not match an installed target: $directory_path" 65
  done <"$directory_records"
}

add_owned_directory() {
  local directory_records=$1
  local directory_path=$2

  state_field_is_safe "$directory_path" || die "owned directory cannot be serialized" 65
  awk -v directory="$directory_path" \
    '$0 == directory { found = 1 } END { exit(found ? 0 : 1) }' \
    "$directory_records" && return 0
  printf '%s\n' "$directory_path" >>"$directory_records"
}

owned_directory_is_listed() {
  local directory_records=$1
  local directory_path=$2

  [ -f "$directory_records" ] || return 1
  awk -v directory="$directory_path" \
    '$0 == directory { found = 1 } END { exit(found ? 0 : 1) }' "$directory_records"
}

collect_action_owned_directories() {
  local actions_file=$1
  local directory_records=$2
  local planned_directories=$3
  local tab=
  local action_type=
  local action_label=
  local source_file=
  local destination_file=
  local destination_mode=
  local allowed_root=
  local relative_suffix=
  local extra=
  local directory_suffix=
  local remaining_suffix=
  local path_component=
  local consumed_suffix=
  local directory_path=

  tab=$(printf '\t')
  while IFS="$tab" read -r action_type action_label source_file destination_file \
    destination_mode allowed_root relative_suffix extra; do
    [ -z "$extra" ] || die "invalid target action while preparing directories" 65
    [ "$action_type" = replace ] || continue
    target_is_known "$action_label" || continue
    : "$source_file" "$destination_file" "$destination_mode"
    directory_suffix=${relative_suffix%/*}
    [ "$directory_suffix" != "$relative_suffix" ] || continue
    remaining_suffix=$directory_suffix
    consumed_suffix=
    while [ -n "$remaining_suffix" ]; do
      case "$remaining_suffix" in
        */*)
          path_component=${remaining_suffix%%/*}
          remaining_suffix=${remaining_suffix#*/}
          ;;
        *) path_component=$remaining_suffix; remaining_suffix= ;;
      esac
      if [ -z "$consumed_suffix" ]; then
        consumed_suffix=$path_component
      else
        consumed_suffix=$consumed_suffix/$path_component
      fi
      directory_path=$(validated_destination_path "$allowed_root" "$consumed_suffix") ||
        return $?
      if [ ! -e "$directory_path" ]; then
        add_owned_directory "$directory_records" "$directory_path"
        add_owned_directory "$planned_directories" "$directory_path"
      fi
    done
  done <"$actions_file"
}

prepare_owned_directories() {
  local existing_directories=$1
  local target_actions=$2
  local final_target_records=$3
  local output_directories=$4
  local planned_directories=$5

  if [ -f "$existing_directories" ]; then
    cp "$existing_directories" "$output_directories"
  else
    : >"$output_directories"
  fi
  : >"$planned_directories"
  collect_inherited_namespace_directories "$output_directories"
  collect_namespace_owned_directories "$output_directories" "$planned_directories"
  collect_action_owned_directories "$target_actions" "$output_directories" \
    "$planned_directories"
  verify_owned_directory_records "$output_directories" "$final_target_records" \
    "$OAW_SCOPE" "$OAW_PROJECT_ROOT"
}

collect_inherited_namespace_directories() {
  local output_directories=$1
  local candidate_state=
  local candidate_index=0
  local candidate_records=
  local candidate_directories=
  local inherited_directories=$OAW_OPERATION_TEMP/inherited-namespace-directories
  local directory_path=

  : >"$inherited_directories"
  for candidate_state in \
    "$OAW_INSTALLATIONS_DIR"/*.state \
    "$OAW_INSTALLATIONS_DIR"/projects/*.state; do
    [ -e "$candidate_state" ] || continue
    [ "$candidate_state" = "$OAW_STATE_FILE" ] && continue
    candidate_index=$((candidate_index + 1))
    candidate_records=$OAW_OPERATION_TEMP/inherited-records-$candidate_index
    candidate_directories=$candidate_records.directories
    (
      load_state_file "$candidate_state" "$candidate_records" "$candidate_directories"
      [ "$STATE_POLICY_PATH" = "$OAW_POLICY_DESTINATION" ] || exit 0
      verify_policy_state_binding "$candidate_state" "$candidate_records"
      verify_state_target_records "$candidate_records" "$STATE_SCOPE" "$STATE_PROJECT_ROOT"
      verify_owned_directory_records "$candidate_directories" "$candidate_records" \
        "$STATE_SCOPE" "$STATE_PROJECT_ROOT"
      while IFS= read -r directory_path; do
        if directory_is_oaw_namespace "$directory_path"; then
          printf '%s\n' "$directory_path"
        fi
      done <"$candidate_directories"
    ) >>"$inherited_directories" || return $?
  done
  while IFS= read -r directory_path; do
    add_owned_directory "$output_directories" "$directory_path"
  done <"$inherited_directories"
}

collect_namespace_owned_directories() {
  local output_directories=$1
  local planned_directories=$2
  local namespace_path=
  local namespace_paths=$OAW_OPERATION_TEMP/namespace-paths

  {
    printf '%s\n' "$OAW_CONFIG_DIR" "$OAW_STATE_DIR" "$OAW_INSTALLATIONS_DIR"
    [ "$OAW_SCOPE" != project ] || printf '%s\n' "$OAW_INSTALLATIONS_DIR/projects"
  } >"$namespace_paths"
  while IFS= read -r namespace_path; do
    if [ ! -e "$namespace_path" ]; then
      add_owned_directory "$output_directories" "$namespace_path"
      add_owned_directory "$planned_directories" "$namespace_path"
    fi
  done <"$namespace_paths"
}

verify_planned_owned_directories_absent() {
  local planned_directories=$1
  local directory_path=

  while IFS= read -r directory_path; do
    [ ! -e "$directory_path" ] && [ ! -L "$directory_path" ] ||
      die "owned directory appeared before creation: $directory_path" 65
  done <"$planned_directories"
}

verify_created_owned_directories() {
  local planned_directories=$1
  local created_directories=$2
  local directory_path=

  while IFS= read -r directory_path; do
    owned_directory_is_listed "$created_directories" "$directory_path" ||
      die "planned owned directory was not created: $directory_path" 65
  done <"$planned_directories"
  while IFS= read -r directory_path; do
    owned_directory_is_listed "$planned_directories" "$directory_path" ||
      die "unplanned owned directory was created: $directory_path" 65
  done <"$created_directories"
}

partition_owned_directories() {
  local existing_directories=$1
  local remaining_target_records=$2
  local retained_directories=$3
  local removed_directories=$4
  local directory_path=

  : >"$retained_directories"
  : >"$removed_directories"
  while IFS= read -r directory_path; do
    if directory_is_oaw_namespace "$directory_path"; then
      if [ -s "$remaining_target_records" ]; then
        printf '%s\n' "$directory_path" >>"$retained_directories"
      else
        printf '%s\n' "$directory_path" >>"$removed_directories"
      fi
    elif owned_directory_allowed_root "$directory_path" "$remaining_target_records" \
      "$OAW_SCOPE" "$OAW_PROJECT_ROOT" >/dev/null; then
      printf '%s\n' "$directory_path" >>"$retained_directories"
    else
      printf '%s\n' "$directory_path" >>"$removed_directories"
    fi
  done <"$existing_directories"
}

prepare_directory_removals() {
  local removed_directories=$1
  local installed_target_records=$2
  local directory_actions=$3
  local directory_path=
  local allowed_root=
  local relative_suffix=
  local sorted_directories=$OAW_OPERATION_TEMP/sorted-removed-directories

  awk '{ print length($0) "\t" $0 }' "$removed_directories" |
    LC_ALL=C sort -t "$(printf '\t')" -k1,1nr -k2,2 >"$sorted_directories"
  : >"$directory_actions"
  while IFS="$(printf '\t')" read -r _ directory_path; do
    [ -n "$directory_path" ] || continue
    allowed_root=$(owned_directory_allowed_root "$directory_path" \
      "$installed_target_records" "$OAW_SCOPE" "$OAW_PROJECT_ROOT") ||
      die "cannot bind owned directory removal: $directory_path" 65
    relative_suffix=$(destination_relative_suffix "$allowed_root" "$directory_path") ||
      return $?
    [ ! -e "$directory_path" ] || [ -d "$directory_path" ] ||
      die "owned directory changed before removal: $directory_path" 65
    printf '%s\t%s\t%s\n' "$allowed_root" "$relative_suffix" "$directory_path" \
      >>"$directory_actions"
  done <"$sorted_directories"
}

verify_prepared_file_actions() {
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
  while IFS="$tab" read -r action_type action_label source_file destination_file \
    destination_mode allowed_root relative_suffix extra; do
    [ -z "$extra" ] || die "invalid prepared file action" 65
    revalidated_destination_path "$allowed_root" "$relative_suffix" \
      "$destination_file" >/dev/null || return $?
    case "$action_type" in
      replace)
        [ -f "$source_file" ] || die "prepared action source is missing: $action_label" 65
        case "$destination_mode" in
          600|644) ;;
          *) die "invalid prepared destination mode: $destination_mode" 65 ;;
        esac
        ;;
      remove)
        [ "$source_file:$destination_mode" = -:- ] ||
          die "invalid prepared remove action: $action_label" 65
        ;;
      *) die "invalid prepared action type: $action_type" 65 ;;
    esac
  done <"$actions_file"
}

verify_prepared_directory_actions() {
  local directory_actions=$1
  local tab=
  local allowed_root=
  local relative_suffix=
  local directory_path=
  local extra=

  tab=$(printf '\t')
  while IFS="$tab" read -r allowed_root relative_suffix directory_path extra; do
    [ -z "$extra" ] || die "invalid prepared directory action" 65
    revalidated_destination_path "$allowed_root" "$relative_suffix" \
      "$directory_path" >/dev/null || return $?
    [ ! -e "$directory_path" ] || [ -d "$directory_path" ] ||
      die "owned directory changed before removal: $directory_path" 65
  done <"$directory_actions"
}

verify_prepared_operation() {
  local policy_action=$1
  local policy_source=$2
  local target_actions=$3
  local state_actions=$4
  local directory_actions=$5
  local planned_directories=$6
  local policy_suffix=

  case "$policy_action" in
    replace) [ -f "$policy_source" ] || die "prepared policy source is missing" 65 ;;
    remove|retain) [ "$policy_source" = - ] || die "invalid prepared policy action" 65 ;;
    *) die "invalid prepared policy action: $policy_action" 65 ;;
  esac
  policy_suffix=$(destination_relative_suffix \
    "$OAW_XDG_CONFIG_HOME" "$OAW_POLICY_DESTINATION") || return $?
  revalidated_destination_path "$OAW_XDG_CONFIG_HOME" "$policy_suffix" \
    "$OAW_POLICY_DESTINATION" >/dev/null || return $?
  verify_prepared_file_actions "$target_actions"
  verify_prepared_file_actions "$state_actions"
  verify_prepared_directory_actions "$directory_actions"
  verify_planned_owned_directories_absent "$planned_directories"
}

apply_operation() {
  local operation_name=$1
  local policy_action=$2
  local policy_source=$3
  local target_actions=$4
  local state_actions=$5
  local directory_actions=$6
  local planned_directories=$7
  local created_directories=$OAW_OPERATION_TEMP/created-owned-directories

  # shellcheck disable=SC2034 # Read by filesystem helpers sourced into this process.
  OAW_PLANNED_OWNED_DIRECTORIES=$planned_directories
  # shellcheck disable=SC2034 # Read by filesystem helpers sourced into this process.
  OAW_CREATED_OWNED_DIRECTORIES=$created_directories
  : >"$created_directories"

  case "$operation_name" in
    install|update)
      apply_target_actions "$target_actions"
      apply_directory_removals "$directory_actions" "$target_actions"
      apply_scoped_replace policy "$policy_source" \
        "$OAW_XDG_CONFIG_HOME" "$OAW_POLICY_DESTINATION" 600
      apply_target_actions "$state_actions"
      if [ "$OAW_DRY_RUN" -eq 0 ]; then
        verify_created_owned_directories "$planned_directories" "$created_directories"
      fi
      ;;
    uninstall)
      apply_target_actions "$target_actions"
      apply_directory_removals "$directory_actions" "$target_actions" \
        "$state_actions" "$policy_action" target
      [ "$policy_action" != remove ] ||
        apply_scoped_remove "$OAW_XDG_CONFIG_HOME" "$OAW_POLICY_DESTINATION"
      apply_target_actions "$state_actions"
      apply_directory_removals "$directory_actions" "$target_actions" \
        "$state_actions" "$policy_action" namespace
      ;;
    *) die "unknown prepared operation: $operation_name" 65 ;;
  esac
}

path_is_planned_removed() {
  local planned_removals=$1
  local candidate_path=$2

  awk -v candidate="$candidate_path" \
    '$0 == candidate { found = 1 } END { exit(found ? 0 : 1) }' "$planned_removals"
}

directory_will_be_empty() {
  local directory_path=$1
  local planned_removals=$2
  local child_path=

  for child_path in \
    "$directory_path"/* \
    "$directory_path"/.[!.]* \
    "$directory_path"/..?*; do
    [ -e "$child_path" ] || [ -L "$child_path" ] || continue
    path_is_planned_removed "$planned_removals" "$child_path" || return 1
  done
  return 0
}

prepare_dry_run_removals() {
  local target_actions=$1
  local state_actions=$2
  local policy_action=$3
  local planned_removals=$4

  awk -F '\t' '$1 == "remove" { print $4 }' \
    "$target_actions" "$state_actions" >"$planned_removals"
  [ "$policy_action" != remove ] || printf '%s\n' "$OAW_POLICY_DESTINATION" >>"$planned_removals"
}

apply_directory_removals() {
  local directory_actions=$1
  local target_actions=$2
  local state_actions=${3:--}
  local policy_action=${4:-retain}
  local removal_class=${5:-target}
  local tab=
  local allowed_root=
  local relative_suffix=
  local directory_path=
  local extra=
  local verified_directory=
  local planned_removals=$OAW_OPERATION_TEMP/planned-removals
  local removal_status=0

  tab=$(printf '\t')
  if [ "$state_actions" = - ]; then
    : >"$OAW_OPERATION_TEMP/no-state-actions"
    state_actions=$OAW_OPERATION_TEMP/no-state-actions
  fi
  prepare_dry_run_removals "$target_actions" "$state_actions" \
    "$policy_action" "$planned_removals"
  while IFS="$tab" read -r allowed_root relative_suffix directory_path extra; do
    [ -z "$extra" ] || die "invalid directory removal action" 65
    case "$removal_class" in
      target) directory_is_oaw_namespace "$directory_path" && continue ;;
      namespace) directory_is_oaw_namespace "$directory_path" || continue ;;
      *) die "invalid directory removal class: $removal_class" 65 ;;
    esac
    verified_directory=$(revalidated_destination_path \
      "$allowed_root" "$relative_suffix" "$directory_path") || return $?
    [ -e "$verified_directory" ] || continue
    [ -d "$verified_directory" ] ||
      die "owned directory changed before removal: $verified_directory" 65
    if [ "$OAW_DRY_RUN" -eq 1 ]; then
      if directory_will_be_empty "$verified_directory" "$planned_removals"; then
        note "would-remove-directory: $verified_directory"
        printf '%s\n' "$verified_directory" >>"$planned_removals"
      else
        note "unchanged-directory: $verified_directory"
      fi
      continue
    fi
    if [ -n "$(find "$verified_directory" -mindepth 1 -print -quit)" ]; then
      note "unchanged-directory: $verified_directory"
      continue
    fi
    remove_empty_destination_directory "$allowed_root" "$relative_suffix" \
      "$directory_path" || {
      removal_status=$?
      [ "$removal_status" -eq 3 ] && continue
      return "$removal_status"
    }
    note "remove-directory: $verified_directory"
  done <"$directory_actions"
}
