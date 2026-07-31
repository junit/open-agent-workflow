#!/usr/bin/env bash

# shellcheck disable=SC2034 # Exports consumed by separately sourced modules.

project_state_identity() {
  local project_root=$1
  printf '%s' "$project_root" | cksum | awk '{ print $1 "-" $2 }'
}

validate_consumed_root() {
  local root_label=$1
  local root_value=$2

  require_absolute_root "$root_value"
  if contains_control_characters "$root_value"; then
    die "$root_label contains control characters" 64
  fi
}

validate_relative_suffix() {
  local relative_suffix=$1
  local remaining_suffix=
  local path_component=

  [ -n "$relative_suffix" ] || die "destination suffix is empty" 65
  case "$relative_suffix" in
    /*) die "destination suffix must be relative: $relative_suffix" 65 ;;
  esac
  if contains_control_characters "$relative_suffix"; then
    die "destination suffix contains control characters" 65
  fi

  remaining_suffix=$relative_suffix
  while :; do
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
    case "$path_component" in
      ''|.|..) die "destination suffix contains an unsafe component: $relative_suffix" 65 ;;
    esac
    [ -n "$remaining_suffix" ] || break
  done
}

validated_destination_path() {
  local allowed_root=$1
  local relative_suffix=$2
  local remaining_suffix=
  local path_component=
  local candidate_path=$allowed_root

  require_absolute_root "$allowed_root"
  if contains_control_characters "$allowed_root"; then
    die "root contains control characters" 64
  fi
  validate_relative_suffix "$relative_suffix"

  remaining_suffix=$relative_suffix
  while :; do
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
    candidate_path=$candidate_path/$path_component
    [ ! -L "$candidate_path" ] ||
      die "destination path contains a symlink: $candidate_path" 65
    if [ -n "$remaining_suffix" ] && [ -e "$candidate_path" ] && [ ! -d "$candidate_path" ]; then
      die "destination path component is not a directory: $candidate_path" 65
    fi
    [ -n "$remaining_suffix" ] || break
  done

  printf '%s\n' "$candidate_path"
}

destination_relative_suffix() {
  local allowed_root=$1
  local expected_destination=$2
  local relative_suffix=
  local rebuilt_destination=

  case "$expected_destination" in
    "$allowed_root"/*) relative_suffix=${expected_destination#"$allowed_root"/} ;;
    *) die "destination escapes its allowed root: $expected_destination" 65 ;;
  esac
  rebuilt_destination=$(validated_destination_path "$allowed_root" "$relative_suffix") || return $?
  [ "$rebuilt_destination" = "$expected_destination" ] ||
    die "destination does not match its allowed root: $expected_destination" 65
  printf '%s\n' "$relative_suffix"
}

target_allowed_root() {
  local target_id=$1

  case "$OAW_SCOPE:$target_id" in
    user:claude|user:codex|user:gemini) printf '%s\n' "$HOME" ;;
    user:opencode) printf '%s\n' "$OAW_XDG_CONFIG_HOME" ;;
    project:*) printf '%s\n' "$OAW_PROJECT_ROOT" ;;
    *) die "target '$target_id' is not implemented for $OAW_SCOPE scope" 69 ;;
  esac
}

target_relative_suffix() {
  local target_id=$1

  case "$OAW_SCOPE:$target_id" in
    user:claude) printf '.claude/CLAUDE.md\n' ;;
    user:codex) printf '.codex/AGENTS.md\n' ;;
    user:gemini) printf '.gemini/GEMINI.md\n' ;;
    user:opencode) printf 'opencode/AGENTS.md\n' ;;
    project:*) target_project_relative_path "$target_id" ;;
    *) die "target '$target_id' is not implemented for $OAW_SCOPE scope" 69 ;;
  esac
}

init_oaw_paths() {
  local project_identity=

  OAW_XDG_CONFIG_HOME=${XDG_CONFIG_HOME:-"$HOME/.config"}
  OAW_XDG_STATE_HOME=${XDG_STATE_HOME:-"$HOME/.local/state"}

  validate_consumed_root HOME "$HOME"
  validate_consumed_root XDG_CONFIG_HOME "$OAW_XDG_CONFIG_HOME"
  validate_consumed_root XDG_STATE_HOME "$OAW_XDG_STATE_HOME"

  OAW_CONFIG_DIR=$(validated_destination_path "$OAW_XDG_CONFIG_HOME" open-agent-workflow)
  OAW_POLICY_DESTINATION=$(
    validated_destination_path "$OAW_XDG_CONFIG_HOME" open-agent-workflow/ENGINEERING.md
  )
  OAW_STATE_DIR=$(validated_destination_path "$OAW_XDG_STATE_HOME" open-agent-workflow)
  OAW_INSTALLATIONS_DIR=$(
    validated_destination_path "$OAW_XDG_STATE_HOME" open-agent-workflow/installations
  )
  OAW_BACKUP_ROOT=$(
    validated_destination_path "$OAW_XDG_STATE_HOME" open-agent-workflow/backups
  )
  case "$OAW_SCOPE" in
    user)
      OAW_STATE_FILE=$(
        validated_destination_path "$OAW_XDG_STATE_HOME" \
          open-agent-workflow/installations/user.state
      )
      ;;
    project)
      project_identity=$(project_state_identity "$OAW_PROJECT_ROOT")
      OAW_STATE_FILE=$(
        validated_destination_path "$OAW_XDG_STATE_HOME" \
          "open-agent-workflow/installations/projects/$project_identity.state"
      )
      ;;
    *) die "unknown operation scope: $OAW_SCOPE" 64 ;;
  esac
}

target_destination() {
  local allowed_root=
  local relative_suffix=
  local target_id=$1

  allowed_root=$(target_allowed_root "$target_id") || return $?
  relative_suffix=$(target_relative_suffix "$target_id") || return $?
  validated_destination_path "$allowed_root" "$relative_suffix"
}
