#!/usr/bin/env bash

# shellcheck disable=SC2034 # Exports consumed by separately sourced modules.

init_oaw_paths() {
  OAW_XDG_CONFIG_HOME=${XDG_CONFIG_HOME:-"$HOME/.config"}
  OAW_XDG_STATE_HOME=${XDG_STATE_HOME:-"$HOME/.local/state"}

  require_absolute_root "$HOME"
  require_absolute_root "$OAW_XDG_CONFIG_HOME"
  require_absolute_root "$OAW_XDG_STATE_HOME"

  OAW_CONFIG_DIR=$OAW_XDG_CONFIG_HOME/open-agent-workflow
  OAW_POLICY_DESTINATION=$OAW_CONFIG_DIR/ENGINEERING.md
  OAW_STATE_DIR=$OAW_XDG_STATE_HOME/open-agent-workflow
  OAW_INSTALLATIONS_DIR=$OAW_STATE_DIR/installations
  OAW_STATE_FILE=$OAW_INSTALLATIONS_DIR/user.state
}

target_destination() {
  case "$OAW_SCOPE:$1" in
    user:claude) printf '%s/.claude/CLAUDE.md\n' "$HOME" ;;
    user:codex) printf '%s/.codex/AGENTS.md\n' "$HOME" ;;
    user:gemini) printf '%s/.gemini/GEMINI.md\n' "$HOME" ;;
    user:opencode) printf '%s/opencode/AGENTS.md\n' "$OAW_XDG_CONFIG_HOME" ;;
    *) die "target '$1' is not implemented for $OAW_SCOPE scope" 69 ;;
  esac
}
