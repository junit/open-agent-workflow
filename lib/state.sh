#!/usr/bin/env bash

# shellcheck disable=SC2034 # Parsed state is consumed by operation modules.

STATE_VERSION=
STATE_SCOPE=
STATE_POLICY_PATH=
STATE_POLICY_CHECKSUM=
STATE_TARGET_ID=
STATE_TARGET_PATH=
STATE_TARGET_MODE=
STATE_TARGET_CHECKSUM=
STATE_TARGET_ORIGIN=
STATE_TARGET_COUNT=0

state_field_is_safe() {
  [ "$(printf '%s' "$1" | tr -d '\t\r\n')" = "$1" ]
}

state_checksum_is_valid() {
  local checksum_value=$1
  local checksum_number=
  local checksum_size=

  case "$checksum_value" in
    *:*)
      checksum_number=${checksum_value%%:*}
      checksum_size=${checksum_value#*:}
      ;;
    *) return 1 ;;
  esac
  case "$checksum_number:$checksum_size" in
    *[!0-9:]*) return 1 ;;
    :*|*:) return 1 ;;
    *:*:*) return 1 ;;
    *) return 0 ;;
  esac
}

validate_target_records() {
  local records_file=$1
  local record_scope=$2
  local minimum_records=${3:-1}
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=
  local target_count=0
  local target_position=0
  local previous_position=0
  local seen_targets=,

  tab=$(printf '\t')
  [ -f "$records_file" ] || die "target records are missing" 65
  [ "$record_scope" = user ] || die "invalid target scope" 65

  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    [ -z "$extra" ] || die "invalid target record: too many fields" 65
    if ! state_field_is_safe "$target_id" || ! state_field_is_safe "$target_path" ||
      ! state_field_is_safe "$target_mode" || ! state_field_is_safe "$target_checksum" ||
      ! state_field_is_safe "$target_origin"; then
      die "target record cannot be serialized" 65
    fi
    target_supports_user "$target_id" || die "invalid target state" 65
    [ -n "$target_path" ] || die "invalid target path" 65
    case "$target_path" in
      /*) ;;
      *) die "invalid target path" 65 ;;
    esac
    [ "$target_mode" = "$(target_ownership "$target_id")" ] ||
      die "invalid target ownership" 65
    state_checksum_is_valid "$target_checksum" || die "invalid target checksum" 65
    case "$target_origin" in
      created-file|existing-file) ;;
      *) die "invalid target ownership" 65 ;;
    esac
    case "$seen_targets" in
      *",$target_id,"*) die "duplicate target state: $target_id" 65 ;;
    esac
    target_position=$(target_registry_position "$target_id")
    [ "$target_position" -gt "$previous_position" ] ||
      die "target state is not in registry order" 65
    seen_targets=$seen_targets$target_id,
    previous_position=$target_position
    target_count=$((target_count + 1))
  done <"$records_file"

  [ "$target_count" -ge "$minimum_records" ] || die "state has no target records" 65
}

write_state_file() {
  local output_file=$1
  local source_version=$2
  local scope=$3
  local policy_path=$4
  local policy_checksum=$5
  local target_records=$6
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=

  state_field_is_safe "$source_version" && [ -n "$source_version" ] ||
    die "version cannot be serialized" 65
  [ "$scope" = user ] || die "invalid state scope" 65
  state_field_is_safe "$policy_path" || die "policy path cannot be serialized" 65
  case "$policy_path" in
    /*) ;;
    *) die "invalid policy path" 65 ;;
  esac
  state_checksum_is_valid "$policy_checksum" || die "invalid policy checksum" 65
  validate_target_records "$target_records" "$scope"
  tab=$(printf '\t')

  {
    printf 'format\t1\n'
    printf 'version\t%s\n' "$source_version"
    printf 'scope\t%s\n' "$scope"
    printf 'policy\t%s\t%s\n' "$policy_path" "$policy_checksum"
    while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
      printf 'target\t%s\t%s\t%s\t%s\t%s\n' \
        "$target_id" "$target_path" "$target_mode" "$target_checksum" "$target_origin"
    done <"$target_records"
  } >"$output_file"
}

load_state_file() {
  local input_file=$1
  local normalized_targets=$2
  local tab=
  local kind=
  local first=
  local second=
  local third=
  local fourth=
  local fifth=
  local extra=
  local format_count=0
  local version_count=0
  local scope_count=0
  local policy_count=0
  local target_count=0

  tab=$(printf '\t')
  STATE_VERSION=
  STATE_SCOPE=
  STATE_POLICY_PATH=
  STATE_POLICY_CHECKSUM=
  STATE_TARGET_ID=
  STATE_TARGET_PATH=
  STATE_TARGET_MODE=
  STATE_TARGET_CHECKSUM=
  STATE_TARGET_ORIGIN=
  STATE_TARGET_COUNT=0

  [ -f "$input_file" ] || return 1
  : >"$normalized_targets"
  while IFS="$tab" read -r kind first second third fourth fifth extra; do
    [ -z "$extra" ] || die "invalid state record: too many fields" 65
    case "$kind" in
      format)
        [ "$first" = 1 ] && [ -z "$second$third$fourth$fifth" ] || die "invalid state format" 65
        format_count=$((format_count + 1))
        ;;
      version)
        [ -n "$first" ] && [ -z "$second$third$fourth$fifth" ] || die "invalid state version" 65
        STATE_VERSION=$first
        version_count=$((version_count + 1))
        ;;
      scope)
        [ "$first" = user ] && [ -z "$second$third$fourth$fifth" ] || die "invalid state scope" 65
        STATE_SCOPE=$first
        scope_count=$((scope_count + 1))
        ;;
      policy)
        [ -n "$first" ] && [ -n "$second" ] && [ -z "$third$fourth$fifth" ] || die "invalid policy state" 65
        STATE_POLICY_PATH=$first
        STATE_POLICY_CHECKSUM=$second
        policy_count=$((policy_count + 1))
        ;;
      target)
        [ -n "$first" ] && [ -n "$second" ] && [ -n "$third" ] &&
          [ -n "$fourth" ] && [ -n "$fifth" ] || die "invalid target state" 65
        printf '%s\t%s\t%s\t%s\t%s\n' "$first" "$second" "$third" "$fourth" "$fifth" >>"$normalized_targets"
        target_count=$((target_count + 1))
        ;;
      *) die "invalid state record type: $kind" 65 ;;
    esac
  done <"$input_file"

  [ "$format_count" -eq 1 ] && [ "$version_count" -eq 1 ] &&
    [ "$scope_count" -eq 1 ] && [ "$policy_count" -eq 1 ] &&
    [ "$target_count" -ge 1 ] || die "state is incomplete or duplicated" 65
  state_field_is_safe "$STATE_VERSION" || die "invalid state version" 65
  state_field_is_safe "$STATE_POLICY_PATH" || die "invalid policy state" 65
  case "$STATE_POLICY_PATH" in
    /*) ;;
    *) die "invalid policy path" 65 ;;
  esac
  state_checksum_is_valid "$STATE_POLICY_CHECKSUM" || die "invalid policy checksum" 65
  validate_target_records "$normalized_targets" "$STATE_SCOPE"
  STATE_TARGET_COUNT=$target_count

  if [ "$target_count" -eq 1 ]; then
    while IFS="$tab" read -r STATE_TARGET_ID STATE_TARGET_PATH STATE_TARGET_MODE \
      STATE_TARGET_CHECKSUM STATE_TARGET_ORIGIN extra; do
      :
    done <"$normalized_targets"
  fi
}

load_target_record() {
  local records_file=$1
  local expected_target_id=$2
  local tab=
  local target_id=
  local target_path=
  local target_mode=
  local target_checksum=
  local target_origin=
  local extra=
  local found=0

  tab=$(printf '\t')
  STATE_TARGET_ID=
  STATE_TARGET_PATH=
  STATE_TARGET_MODE=
  STATE_TARGET_CHECKSUM=
  STATE_TARGET_ORIGIN=

  while IFS="$tab" read -r target_id target_path target_mode target_checksum target_origin extra; do
    if [ "$target_id" = "$expected_target_id" ]; then
      STATE_TARGET_ID=$target_id
      STATE_TARGET_PATH=$target_path
      STATE_TARGET_MODE=$target_mode
      STATE_TARGET_CHECKSUM=$target_checksum
      STATE_TARGET_ORIGIN=$target_origin
      found=$((found + 1))
    fi
  done <"$records_file"

  [ "$found" -eq 1 ]
}

target_record_exists() {
  local records_file=$1
  local expected_target_id=$2
  awk -F '\t' -v target_id="$expected_target_id" '
    $1 == target_id { found++ }
    END { exit(found == 1 ? 0 : 1) }
  ' "$records_file"
}

merge_install_target_records() {
  local existing_records=$1
  local selected_records=$2
  local merged_records=$3
  local registry_target=

  : >"$merged_records"
  for registry_target in $(target_ids); do
    if target_record_exists "$selected_records" "$registry_target"; then
      awk -F '\t' -v target_id="$registry_target" '$1 == target_id' "$selected_records" >>"$merged_records"
    elif target_record_exists "$existing_records" "$registry_target"; then
      awk -F '\t' -v target_id="$registry_target" '$1 == target_id' "$existing_records" >>"$merged_records"
    fi
  done
  validate_target_records "$merged_records" user
}

state_file_references_policy() (
  local input_file=$1
  local policy_path=$2
  local policy_checksum=$3
  local target_records=

  target_records=$(mktemp "${TMPDIR:-/tmp}/oaw-state-records.XXXXXX") ||
    die "cannot create state validation workspace" 73
  trap 'rm -f -- "$target_records"' EXIT HUP INT TERM
  load_state_file "$input_file" "$target_records"

  [ "$STATE_POLICY_PATH" = "$policy_path" ] &&
    [ "$STATE_POLICY_CHECKSUM" = "$policy_checksum" ]
)

other_state_references_policy() {
  local installations_dir=$1
  local excluded_state=$2
  local policy_path=$3
  local policy_checksum=$4
  local candidate_state=
  local reference_status=0

  for candidate_state in "$installations_dir"/*.state; do
    [ -e "$candidate_state" ] || continue
    [ "$candidate_state" = "$excluded_state" ] && continue

    if state_file_references_policy "$candidate_state" "$policy_path" "$policy_checksum"; then
      return 0
    else
      reference_status=$?
      [ "$reference_status" -eq 1 ] || return "$reference_status"
    fi
  done

  return 1
}
