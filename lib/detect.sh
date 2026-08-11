#!/usr/bin/env bash

detect_provider_matt() {
  local matt_indicator=

  for matt_indicator in \
    "$HOME"/.agents/.skill-lock.json \
    "$HOME"/.claude/plugins/cache/claude-plugins-official/mattpocock-skills/*/.claude-plugin/plugin.json
  do
    if [ -f "$matt_indicator" ]; then
      return 0
    fi
  done

  return 1
}

detect_provider_superpowers() {
  local superpowers_indicator=

  # Bounded indicators: Claude official/legacy plugin caches or the Codex
  # curated plugin cache. The single wildcard is the installed version ID.
  for superpowers_indicator in \
    "$HOME"/.claude/plugins/superpowers/skills/using-superpowers/SKILL.md \
    "$HOME"/.claude/plugins/cache/claude-plugins-official/superpowers/*/skills/using-superpowers/SKILL.md \
    "$HOME"/.claude/plugins/cache/superpowers-marketplace/superpowers/*/skills/using-superpowers/SKILL.md \
    "$HOME"/.codex/plugins/superpowers/skills/using-superpowers/SKILL.md \
    "$HOME"/.codex/plugins/cache/openai-api-curated/superpowers/*/skills/using-superpowers/SKILL.md
  do
    if [ -f "$superpowers_indicator" ]; then
      return 0
    fi
  done

  # The legacy Claude marketplace checkout is unversioned.
  [ -f "$HOME/.claude/plugins/marketplaces/superpowers-marketplace/skills/using-superpowers/SKILL.md" ]
}

detect_provider_ecc() {
  local ecc_indicator=

  for ecc_indicator in \
    "$HOME"/.claude/plugins/marketplaces/everything-claude-code/plugins/ecc/.codex-plugin/plugin.json \
    "$HOME"/.claude/plugins/cache/everything-claude-code/ecc/*/.codex-plugin/plugin.json \
    "$HOME"/.codex/plugins/ecc/.codex-plugin/plugin.json \
    "$HOME"/.codex/plugins/cache/everything-claude-code/ecc/*/.codex-plugin/plugin.json
  do
    if [ -f "$ecc_indicator" ]; then
      return 0
    fi
  done

  return 1
}

detect_provider() {
  case "$1" in
    superpowers) detect_provider_superpowers ;;
    matt) detect_provider_matt ;;
    ecc) detect_provider_ecc ;;
    *) return 1 ;;
  esac
}

detect_core_target() {
  local target_to_detect=$1
  local target_config_root=${XDG_CONFIG_HOME:-"$HOME/.config"}

  case "$target_to_detect" in
    claude)
      command_exists claude || [ -d "$HOME/.claude" ]
      ;;
    codex)
      command_exists codex || [ -d "$HOME/.codex" ]
      ;;
    gemini)
      command_exists gemini || [ -d "$HOME/.gemini" ]
      ;;
    opencode)
      command_exists opencode || [ -d "$target_config_root/opencode" ]
      ;;
    *)
      return 1
      ;;
  esac
}

selected_target() {
  case ",$OAW_SELECTED_TARGETS," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

target_readiness() {
  local readiness_target=$1

  if target_supports_user "$readiness_target"; then
    if detect_core_target "$readiness_target"; then
      printf 'detected (user, project)\n'
    else
      printf 'missing (user, project)\n'
    fi
  else
    printf 'adapter-only (project)\n'
  fi
}
