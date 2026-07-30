#!/usr/bin/env bash

atomic_install_file() {
  local source_file=$1
  local destination_file=$2
  local destination_mode=$3
  local destination_dir=
  local temporary_file=

  destination_dir=$(dirname -- "$destination_file")
  mkdir -p "$destination_dir"
  temporary_file=$(mktemp "$destination_dir/.oaw.XXXXXX") ||
    die "cannot create temporary file in $destination_dir" 73
  cp "$source_file" "$temporary_file"
  chmod "$destination_mode" "$temporary_file"
  mv -f "$temporary_file" "$destination_file"
}

apply_replace() {
  local action_label=$1
  local source_file=$2
  local destination_file=$3
  local destination_mode=$4
  local action=create

  if [ -f "$destination_file" ] && files_equal "$destination_file" "$source_file"; then
    note "unchanged: $action_label"
    return 0
  fi
  [ -e "$destination_file" ] && action=update
  if [ "$OAW_DRY_RUN" -eq 1 ]; then
    note "would-$action: $destination_file"
    return 0
  fi
  atomic_install_file "$source_file" "$destination_file" "$destination_mode"
  note "$action: $destination_file"
}

apply_remove() {
  local destination_file=$1

  [ -e "$destination_file" ] || return 0
  if [ "$OAW_DRY_RUN" -eq 1 ]; then
    note "would-remove: $destination_file"
  else
    rm -f -- "$destination_file"
    note "remove: $destination_file"
  fi
}
