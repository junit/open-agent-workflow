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

verify_current_directory_coordinate() {
  local allowed_root=$1
  local consumed_suffix=$2
  local expected_component=
  local expected_physical=
  local current_physical=

  expected_component=$(validated_destination_path "$allowed_root" "$consumed_suffix") ||
    return $?
  expected_physical=$(CDPATH='' cd -P -- "$expected_component" 2>/dev/null && pwd -P) ||
    die "destination directory changed during creation: $expected_component" 65
  current_physical=$(pwd -P) ||
    die "cannot resolve destination directory: $expected_component" 65
  [ "$current_physical" = "$expected_physical" ] ||
    die "destination directory changed during creation: $expected_component" 65
}

enter_destination_component() {
  local allowed_root=$1
  local consumed_suffix=$2
  local path_component=$3
  local component_path=$allowed_root/$consumed_suffix

  [ ! -L "./$path_component" ] ||
    die "destination path contains a symlink: $component_path" 65
  if [ -e "./$path_component" ]; then
    [ -d "./$path_component" ] ||
      die "destination path component is not a directory: $component_path" 65
  else
    mkdir "./$path_component"
  fi
  CDPATH='' cd -P -- "./$path_component" ||
    die "cannot enter destination directory: $path_component" 65
  verify_current_directory_coordinate "$allowed_root" "$consumed_suffix"
}

ensure_destination_directory() {
  local allowed_root=$1
  local relative_suffix=$2
  local expected_destination=$3
  local destination_dir=
  local directory_suffix=
  local remaining_suffix=
  local path_component=
  local consumed_suffix=

  destination_dir=$(dirname -- "$expected_destination")
  directory_suffix=${relative_suffix%/*}
  [ "$directory_suffix" != "$relative_suffix" ] || directory_suffix=

  if [ ! -d "$allowed_root" ]; then
    mkdir -p "$allowed_root"
  fi
  [ -d "$allowed_root" ] || die "allowed root is not a directory: $allowed_root" 65

  (
    CDPATH='' cd -P -- "$allowed_root" ||
      die "cannot enter allowed root: $allowed_root" 65
    remaining_suffix=$directory_suffix
    while [ -n "$remaining_suffix" ]; do
      case "$remaining_suffix" in
        */*)
          path_component=${remaining_suffix%%/*}
          remaining_suffix=${remaining_suffix#*/}
          ;;
        *)
          path_component=$remaining_suffix
          remaining_suffix=
          ;;
      esac

      consumed_suffix=$consumed_suffix$path_component
      enter_destination_component "$allowed_root" "$consumed_suffix" "$path_component" ||
        exit $?
      [ -z "$remaining_suffix" ] || consumed_suffix=$consumed_suffix/
    done
  ) || return $?

  [ -d "$destination_dir" ] || die "destination directory is missing: $destination_dir" 65
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
  ensure_destination_directory "$allowed_root" "$relative_suffix" "$expected_destination" ||
    return $?
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
