#!/usr/bin/env bash

# shellcheck disable=SC2034 # Exports consumed by separately sourced modules.

project_state_identity() {
  local project_root=$1
  printf '%s' "$project_root" | cksum | awk '{ print $1 "-" $2 }'
}

init_oaw_paths() {
  local project_identity=

  OAW_XDG_CONFIG_HOME=${XDG_CONFIG_HOME:-"$HOME/.config"}
  OAW_XDG_STATE_HOME=${XDG_STATE_HOME:-"$HOME/.local/state"}

  require_absolute_root "$HOME"
  require_absolute_root "$OAW_XDG_CONFIG_HOME"
  require_absolute_root "$OAW_XDG_STATE_HOME"

  OAW_CONFIG_DIR=$OAW_XDG_CONFIG_HOME/open-agent-workflow
  OAW_POLICY_DESTINATION=$OAW_CONFIG_DIR/ENGINEERING.md
  OAW_STATE_DIR=$OAW_XDG_STATE_HOME/open-agent-workflow
  OAW_INSTALLATIONS_DIR=$OAW_STATE_DIR/installations
  case "$OAW_SCOPE" in
    user)
      OAW_STATE_FILE=$OAW_INSTALLATIONS_DIR/user.state
      ;;
    project)
      project_identity=$(project_state_identity "$OAW_PROJECT_ROOT")
      OAW_STATE_FILE=$OAW_INSTALLATIONS_DIR/projects/$project_identity.state
      ;;
    *) die "unknown operation scope: $OAW_SCOPE" 64 ;;
  esac
}

target_destination() {
  local project_relative_path=
  local target_id=$1

  case "$OAW_SCOPE:$target_id" in
    user:claude) printf '%s/.claude/CLAUDE.md\n' "$HOME" ;;
    user:codex) printf '%s/.codex/AGENTS.md\n' "$HOME" ;;
    user:gemini) printf '%s/.gemini/GEMINI.md\n' "$HOME" ;;
    user:opencode) printf '%s/opencode/AGENTS.md\n' "$OAW_XDG_CONFIG_HOME" ;;
    project:*)
      project_relative_path=$(target_project_relative_path "$target_id") || return $?
      printf '%s/%s\n' "$OAW_PROJECT_ROOT" "$project_relative_path"
      ;;
    *) die "target '$target_id' is not implemented for $OAW_SCOPE scope" 69 ;;
  esac
}
