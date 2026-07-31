#!/usr/bin/env bash

OAW_OPERATION_BACKUP_PATH=
OAW_BACKUP_ARTIFACT_COUNT=0
OAW_ACTIVE_BACKUP_MANIFEST=

reserve_operation_backup_path() {
  local timestamp=

  timestamp=$(date -u '+%Y%m%dT%H%M%SZ') || die "cannot create backup timestamp" 73
  OAW_OPERATION_BACKUP_PATH=$OAW_BACKUP_ROOT/$timestamp-$$
}

create_private_backup_directory() {
  local operation_suffix=

  operation_suffix=$(destination_relative_suffix \
    "$OAW_XDG_STATE_HOME" "$OAW_OPERATION_BACKUP_PATH") || return $?
  create_private_destination_directory "$OAW_XDG_STATE_HOME" \
    "$operation_suffix" "$OAW_OPERATION_BACKUP_PATH"
}

add_backup_candidate() {
  local candidates_file=$1
  local original_path=$2
  local allowed_root=$3
  local relative_suffix=$4

  state_field_is_safe "$original_path" && state_field_is_safe "$allowed_root" &&
    state_field_is_safe "$relative_suffix" || die "backup candidate cannot be serialized" 65
  awk -F '\t' -v original="$original_path" '$1 == original { found = 1 } END { exit(found ? 0 : 1) }' \
    "$candidates_file" && return 0
  printf '%s\t%s\t%s\n' "$original_path" "$allowed_root" "$relative_suffix" \
    >>"$candidates_file"
}

collect_action_backup_candidates() {
  local actions_file=$1
  local candidates_file=$2
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
    [ -z "$extra" ] || die "invalid backup action" 65
    : "$action_label" "$destination_mode"
    case "$action_type" in
      replace)
        [ -f "$destination_file" ] && ! files_equal "$destination_file" "$source_file" ||
          continue
        ;;
      remove) [ -f "$destination_file" ] || continue ;;
      *) die "invalid backup action type: $action_type" 65 ;;
    esac
    add_backup_candidate "$candidates_file" "$destination_file" \
      "$allowed_root" "$relative_suffix"
  done <"$actions_file"
}

collect_policy_backup_candidate() {
  local candidates_file=$1
  local prospective_policy=$2
  local relative_suffix=

  [ -f "$OAW_POLICY_DESTINATION" ] || return 0
  if [ "$prospective_policy" != - ]; then
    files_equal "$OAW_POLICY_DESTINATION" "$prospective_policy" && return 0
  fi
  relative_suffix=$(destination_relative_suffix \
    "$OAW_XDG_CONFIG_HOME" "$OAW_POLICY_DESTINATION") || return $?
  add_backup_candidate "$candidates_file" "$OAW_POLICY_DESTINATION" \
    "$OAW_XDG_CONFIG_HOME" "$relative_suffix"
}

write_backup_artifact() {
  local manifest_temp=$1
  local original_path=$2
  local allowed_root=$3
  local relative_suffix=$4
  local artifact_index=$5
  local verified_path=
  local artifact_name=
  local backup_path=
  local backup_suffix=
  local original_checksum=
  local backup_checksum=

  verified_path=$(revalidated_destination_path \
    "$allowed_root" "$relative_suffix" "$original_path") || return $?
  [ -f "$verified_path" ] || die "backup source is not a file: $verified_path" 65
  artifact_name=$(printf '%03d-%s' "$artifact_index" "${verified_path##*/}")
  backup_path=$OAW_OPERATION_BACKUP_PATH/$artifact_name
  original_checksum=$(checksum_file "$verified_path")
  backup_suffix=$(destination_relative_suffix "$OAW_XDG_STATE_HOME" "$backup_path") ||
    return $?
  atomic_install_file "$verified_path" "$OAW_XDG_STATE_HOME" \
    "$backup_suffix" "$backup_path" 600
  backup_checksum=$(checksum_file "$backup_path")
  [ "$backup_checksum" = "$original_checksum" ] ||
    die "backup verification failed: $verified_path" 74
  verified_path=$(revalidated_destination_path \
    "$allowed_root" "$relative_suffix" "$original_path") || return $?
  [ "$(checksum_file "$verified_path")" = "$original_checksum" ] ||
    die "backup source changed during copy: $verified_path" 65
  printf 'artifact\t%s\t%s\t%s\n' \
    "$verified_path" "$backup_path" "$original_checksum" >>"$manifest_temp"
}

write_backup_candidates() {
  local manifest_temp=$1
  local candidates_file=$2
  local tab=
  local original_path=
  local allowed_root=
  local relative_suffix=
  local extra=

  OAW_BACKUP_ARTIFACT_COUNT=0
  tab=$(printf '\t')
  while IFS="$tab" read -r original_path allowed_root relative_suffix extra; do
    [ -z "$extra" ] || die "invalid backup candidate" 65
    OAW_BACKUP_ARTIFACT_COUNT=$((OAW_BACKUP_ARTIFACT_COUNT + 1))
    write_backup_artifact "$manifest_temp" "$original_path" \
      "$allowed_root" "$relative_suffix" "$OAW_BACKUP_ARTIFACT_COUNT"
  done <"$candidates_file"
}

verify_completed_backup_sources() {
  local candidates_file=$1
  local manifest_path=$2
  local tab=
  local original_path=
  local allowed_root=
  local relative_suffix=
  local extra=
  local verified_path=
  local recorded_checksum=

  tab=$(printf '\t')
  while IFS="$tab" read -r original_path allowed_root relative_suffix extra; do
    [ -z "$extra" ] || die "invalid backup candidate" 65
    verified_path=$(revalidated_destination_path \
      "$allowed_root" "$relative_suffix" "$original_path") || return $?
    recorded_checksum=$(awk -F '\t' -v original="$verified_path" \
      '$1 == "artifact" && $2 == original { print $4; exit }' "$manifest_path")
    [ -n "$recorded_checksum" ] &&
      [ "$(checksum_file "$verified_path")" = "$recorded_checksum" ] ||
      die "backup source changed before mutation: $verified_path" 65
  done <"$candidates_file"
}

verify_active_backup_destination() {
  local destination_file=$1
  local recorded_checksum=

  [ -n "$OAW_ACTIVE_BACKUP_MANIFEST" ] || return 0
  recorded_checksum=$(awk -F '\t' -v original="$destination_file" \
    '$1 == "artifact" && $2 == original { print $4; exit }' \
    "$OAW_ACTIVE_BACKUP_MANIFEST")
  [ -n "$recorded_checksum" ] ||
    die "mutation destination is missing from backup: $destination_file" 65
  [ -f "$destination_file" ] &&
    [ "$(checksum_file "$destination_file")" = "$recorded_checksum" ] ||
    die "backup source changed before mutation: $destination_file" 65
}

create_operation_backup() {
  local operation_name=$1
  local scope=$2
  local candidates_file=$3
  local manifest_path=$OAW_OPERATION_BACKUP_PATH/manifest.tsv
  local manifest_suffix=
  local manifest_temp=
  local previous_umask=
  local directory_status=0

  if [ "$OAW_DRY_RUN" -eq 1 ]; then
    note "would-backup: $OAW_OPERATION_BACKUP_PATH"
    return 0
  fi
  manifest_suffix=$(destination_relative_suffix "$OAW_XDG_STATE_HOME" "$manifest_path") ||
    return $?
  previous_umask=$(umask)
  umask 077
  create_private_backup_directory || {
    directory_status=$?
    umask "$previous_umask"
    return "$directory_status"
  }
  umask "$previous_umask"
  manifest_temp=$(mktemp "$OAW_OPERATION_BACKUP_PATH/.manifest.XXXXXX") ||
    die "cannot create backup manifest" 73
  chmod 600 "$manifest_temp"
  {
    printf 'format\t1\n'
    printf 'operation\t%s\n' "$operation_name"
    printf 'scope\t%s\n' "$scope"
  } >"$manifest_temp"

  write_backup_candidates "$manifest_temp" "$candidates_file"
  [ "$OAW_BACKUP_ARTIFACT_COUNT" -gt 0 ] ||
    die "forced mutation has no recoverable artifacts" 65
  atomic_install_file "$manifest_temp" "$OAW_XDG_STATE_HOME" \
    "$manifest_suffix" "$manifest_path" 600
  rm -f -- "$manifest_temp"
  verify_completed_backup_sources "$candidates_file" "$manifest_path"
  OAW_ACTIVE_BACKUP_MANIFEST=$manifest_path
  note "backup: $OAW_OPERATION_BACKUP_PATH"
}

perform_operation_backup() {
  local operation_name=$1
  local prospective_policy=$2
  local target_actions=$3
  local state_actions=$4
  local candidates_file=$OAW_OPERATION_TEMP/backup-candidates

  : >"$candidates_file"
  collect_policy_backup_candidate "$candidates_file" "$prospective_policy"
  collect_action_backup_candidates "$target_actions" "$candidates_file"
  collect_action_backup_candidates "$state_actions" "$candidates_file"
  create_operation_backup "$operation_name" "$OAW_SCOPE" "$candidates_file"
}

create_manual_recovery_backup() {
  local operation_name=$1
  local target_id=$2
  local target_path=$3
  local candidates_file=$OAW_OPERATION_TEMP/manual-backup-candidates
  local target_root=
  local target_suffix=
  local state_suffix=
  local policy_suffix=

  reserve_operation_backup_path
  : >"$candidates_file"
  target_root=$(target_allowed_root "$target_id") || return $?
  target_suffix=$(target_relative_suffix "$target_id") || return $?
  add_backup_candidate "$candidates_file" "$target_path" "$target_root" "$target_suffix"
  state_suffix=$(destination_relative_suffix "$OAW_XDG_STATE_HOME" "$OAW_STATE_FILE") ||
    return $?
  add_backup_candidate "$candidates_file" "$OAW_STATE_FILE" \
    "$OAW_XDG_STATE_HOME" "$state_suffix"
  if [ -f "$OAW_POLICY_DESTINATION" ]; then
    policy_suffix=$(destination_relative_suffix \
      "$OAW_XDG_CONFIG_HOME" "$OAW_POLICY_DESTINATION") || return $?
    add_backup_candidate "$candidates_file" "$OAW_POLICY_DESTINATION" \
      "$OAW_XDG_CONFIG_HOME" "$policy_suffix"
  fi
  create_operation_backup "$operation_name" "$OAW_SCOPE" "$candidates_file"
  die "manual recovery required; backup: $OAW_OPERATION_BACKUP_PATH" 65
}
