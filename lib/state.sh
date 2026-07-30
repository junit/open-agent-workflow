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

state_field_is_safe() {
  [ "$(printf '%s' "$1" | tr -d '\t\r\n')" = "$1" ]
}

write_state_file() {
  local output_file=$1
  local source_version=$2
  local scope=$3
  local policy_path=$4
  local policy_checksum=$5
  local target_id=$6
  local target_path=$7
  local target_mode=$8
  local target_checksum=$9
  local target_origin=${10}

  state_field_is_safe "$policy_path" || die "policy path cannot be serialized" 65
  state_field_is_safe "$target_path" || die "target path cannot be serialized" 65
  target_supports_user "$target_id" || die "invalid target state" 65
  [ "$target_mode" = "$(target_ownership "$target_id")" ] || die "invalid target ownership" 65

  {
    printf 'format\t1\n'
    printf 'version\t%s\n' "$source_version"
    printf 'scope\t%s\n' "$scope"
    printf 'policy\t%s\t%s\n' "$policy_path" "$policy_checksum"
    printf 'target\t%s\t%s\t%s\t%s\t%s\n' \
      "$target_id" "$target_path" "$target_mode" "$target_checksum" "$target_origin"
  } >"$output_file"
}

load_state_file() {
  local input_file=$1
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

  [ -f "$input_file" ] || return 1
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
        target_supports_user "$first" && [ -n "$second" ] &&
          [ "$third" = "$(target_ownership "$first")" ] &&
          [ -n "$fourth" ] && [ -n "$fifth" ] || die "invalid target state" 65
        case "$fifth" in
          created-file|existing-file) ;;
          *) die "invalid target ownership" 65 ;;
        esac
        STATE_TARGET_ID=$first
        STATE_TARGET_PATH=$second
        STATE_TARGET_MODE=$third
        STATE_TARGET_CHECKSUM=$fourth
        STATE_TARGET_ORIGIN=$fifth
        target_count=$((target_count + 1))
        ;;
      *) die "invalid state record type: $kind" 65 ;;
    esac
  done <"$input_file"

  [ "$format_count" -eq 1 ] && [ "$version_count" -eq 1 ] &&
    [ "$scope_count" -eq 1 ] && [ "$policy_count" -eq 1 ] &&
    [ "$target_count" -eq 1 ] || die "state is incomplete or duplicated" 65
}

state_file_references_policy() (
  local input_file=$1
  local policy_path=$2
  local policy_checksum=$3

  load_state_file "$input_file"

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
