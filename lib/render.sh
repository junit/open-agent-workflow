#!/usr/bin/env bash

OAW_BEGIN_MARKER='<!-- BEGIN OPEN AGENT WORKFLOW -->'
OAW_END_MARKER='<!-- END OPEN AGENT WORKFLOW -->'

render_claude() {
  local policy_path=$1
  printf '%s\n' \
    'Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:' \
    "@$policy_path"
}

render_target_content() {
  local render_target=$1
  local render_policy=$2

  case "$render_target" in
    claude) render_claude "$render_policy" ;;
    *) die "no renderer for target '$render_target'" 69 ;;
  esac
}

render_managed_block() {
  local block_target=$1
  local block_policy=$2
  local block_output=$3

  {
    printf '%s\n' "$OAW_BEGIN_MARKER"
    render_target_content "$block_target" "$block_policy"
    printf '%s\n' "$OAW_END_MARKER"
  } >"$block_output"
}
