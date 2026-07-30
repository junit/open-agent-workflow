#!/usr/bin/env bash

OAW_SCOPE=
OAW_PROJECT_ROOT=
# shellcheck disable=SC2034 # Read by command modules after this file is sourced.
OAW_SELECTED_TARGETS=

target_ids() {
  printf '%s\n' \
    claude \
    codex \
    gemini \
    opencode \
    cursor \
    windsurf \
    cline \
    roo \
    copilot
}

target_supports_user() {
  case "$1" in
    claude|codex|gemini|opencode) return 0 ;;
    *) return 1 ;;
  esac
}

target_is_known() {
  case "$1" in
    claude|codex|gemini|opencode|cursor|windsurf|cline|roo|copilot) return 0 ;;
    *) return 1 ;;
  esac
}

default_targets() {
  case "$1" in
    user)
      printf '%s\n' 'claude,codex,gemini,opencode'
      ;;
    project)
      printf '%s\n' 'claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot'
      ;;
    *)
      cli_error "unknown scope '$1'"
      return 64
      ;;
  esac
}

contains_control_characters() {
  case "$1" in
    *[[:cntrl:]]*) return 0 ;;
    *) return 1 ;;
  esac
}

normalize_targets() {
  local selection=$1
  local selection_scope=$2
  local selection_remaining=
  local selection_member=
  local normalized_selection=
  local registry_target=

  if [ -z "$selection" ]; then
    default_targets "$selection_scope"
    return $?
  fi

  case "$selection" in
    *[[:space:]]*)
      cli_error "target selection must not contain whitespace"
      return 64
      ;;
    ,*|*,|*,,*)
      cli_error "target selection contains an empty member"
      return 64
      ;;
  esac

  selection_remaining=$selection
  while :; do
    case "$selection_remaining" in
      *,*)
        selection_member=${selection_remaining%%,*}
        selection_remaining=${selection_remaining#*,}
        ;;
      *)
        selection_member=$selection_remaining
        selection_remaining=
        ;;
    esac

    if ! target_is_known "$selection_member"; then
      cli_error "unknown target '$selection_member'"
      return 64
    fi
    if [ "$selection_scope" = user ] && ! target_supports_user "$selection_member"; then
      cli_error "target '$selection_member' does not support user scope"
      return 64
    fi

    if [ -z "$selection_remaining" ]; then
      break
    fi
  done

  for registry_target in $(target_ids); do
    case ",$selection," in
      *",$registry_target,"*)
        if [ -z "$normalized_selection" ]; then
          normalized_selection=$registry_target
        else
          normalized_selection=$normalized_selection,$registry_target
        fi
        ;;
    esac
  done

  printf '%s\n' "$normalized_selection"
}

resolve_scope_and_targets() {
  OAW_SCOPE=user
  OAW_PROJECT_ROOT=

  if [ -n "$OAW_PROJECT" ]; then
    if contains_control_characters "$OAW_PROJECT"; then
      cli_error "project path contains control characters"
      return 64
    fi
    if [ ! -d "$OAW_PROJECT" ]; then
      cli_error "project directory does not exist: $OAW_PROJECT"
      return 64
    fi

    OAW_PROJECT_ROOT=$(CDPATH='' cd -P -- "$OAW_PROJECT" 2>/dev/null && pwd -P) || {
      cli_error "project directory could not be resolved: $OAW_PROJECT"
      return 64
    }
    if contains_control_characters "$OAW_PROJECT_ROOT"; then
      cli_error "resolved project path contains control characters"
      return 64
    fi
    require_absolute_root "$OAW_PROJECT_ROOT"
    OAW_SCOPE=project
  fi

  # shellcheck disable=SC2034 # Consumed by command modules after resolution.
  OAW_SELECTED_TARGETS=$(normalize_targets "$OAW_TARGETS" "$OAW_SCOPE") || return $?
}
