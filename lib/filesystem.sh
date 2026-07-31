#!/usr/bin/env bash

revalidated_destination_path() {
  local allowed_root=$1
  local relative_suffix=$2
  local expected_destination=$3
  local rebuilt_destination=

  rebuilt_destination=$(validated_destination_path "$allowed_root" "$relative_suffix") || return $?
  [ "$rebuilt_destination" = "$expected_destination" ] ||
    die "destination changed after preparation: $expected_destination" 65
  printf '%s\n' "$rebuilt_destination"
}

atomic_install_file() {
  local source_file=$1
  local allowed_root=$2
  local relative_suffix=$3
  local expected_destination=$4
  local destination_mode=$5
  local destination_file=
  local destination_dir=
  local temporary_file=
  local validation_status=0

  destination_file=$(
    revalidated_destination_path "$allowed_root" "$relative_suffix" "$expected_destination"
  ) || return $?
  destination_dir=$(dirname -- "$destination_file")
  mkdir -p "$destination_dir"
  destination_file=$(
    revalidated_destination_path "$allowed_root" "$relative_suffix" "$expected_destination"
  ) || return $?
  destination_dir=$(dirname -- "$destination_file")
  temporary_file=$(mktemp "$destination_dir/.oaw.XXXXXX") ||
    die "cannot create temporary file in $destination_dir" 73
  cp "$source_file" "$temporary_file"
  chmod "$destination_mode" "$temporary_file"
  destination_file=$(
    revalidated_destination_path "$allowed_root" "$relative_suffix" "$expected_destination"
  ) || {
    validation_status=$?
    rm -f -- "$temporary_file"
    return "$validation_status"
  }
  mv -f "$temporary_file" "$destination_file"
}

apply_replace() {
  local action_label=$1
  local source_file=$2
  local allowed_root=$3
  local relative_suffix=$4
  local expected_destination=$5
  local destination_mode=$6
  local destination_file=
  local action=create

  destination_file=$(
    revalidated_destination_path "$allowed_root" "$relative_suffix" "$expected_destination"
  ) || return $?
  if [ -f "$destination_file" ] && files_equal "$destination_file" "$source_file"; then
    note "unchanged: $action_label"
    return 0
  fi
  [ -e "$destination_file" ] && action=update
  if [ "$OAW_DRY_RUN" -eq 1 ]; then
    note "would-$action: $destination_file"
    return 0
  fi
  atomic_install_file "$source_file" "$allowed_root" "$relative_suffix" \
    "$expected_destination" "$destination_mode"
  note "$action: $destination_file"
}

apply_remove() {
  local allowed_root=$1
  local relative_suffix=$2
  local expected_destination=$3
  local destination_file=

  destination_file=$(
    revalidated_destination_path "$allowed_root" "$relative_suffix" "$expected_destination"
  ) || return $?
  [ -e "$destination_file" ] || return 0
  if [ "$OAW_DRY_RUN" -eq 1 ]; then
    note "would-remove: $destination_file"
  else
    destination_file=$(
      revalidated_destination_path "$allowed_root" "$relative_suffix" "$expected_destination"
    ) || return $?
    rm -f -- "$destination_file"
    note "remove: $destination_file"
  fi
}
