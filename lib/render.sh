#!/usr/bin/env bash

OAW_BEGIN_MARKER='<!-- BEGIN OPEN AGENT WORKFLOW -->'
OAW_END_MARKER='<!-- END OPEN AGENT WORKFLOW -->'

render_claude() {
  local policy_path=$1
  printf '%s\n' \
    'Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:' \
    "@$policy_path"
}

render_codex() {
  printf 'For every new top-level engineering request, first read `%s`, classify it as DIRECT, BOUNDED, or WORKFLOW, and run its blocking selection gate only for WORKFLOW. Preserve the selected Lifecycle Bundle for Workflow work.\n' "$1"
}

render_gemini() {
  printf 'Follow the Open Agent Workflow policy before engineering lifecycle work:\n@%s\n' "$1"
}

render_opencode() {
  printf 'Before engineering lifecycle work, use the Read tool to read `%s`, then follow its blocking selection gate and lifecycle lock.\n' "$1"
}

render_project_bootstrap() {
  printf 'Before engineering lifecycle work, read `%s`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task.\n' "$1"
}

render_cursor() {
  local policy_path=$1

  printf '%s\n' \
    '---' \
    'description: Open Agent Workflow lifecycle policy' \
    'globs: "**/*"' \
    'alwaysApply: true' \
    '---' \
    ''
  render_project_bootstrap "$policy_path"
}

render_windsurf() {
  local policy_path=$1

  printf '%s\n' \
    '---' \
    'trigger: always_on' \
    '---' \
    ''
  render_project_bootstrap "$policy_path"
}

render_cline() {
  render_project_bootstrap "$1"
}

render_roo() {
  render_project_bootstrap "$1"
}

render_copilot() {
  local policy_path=$1

  printf '%s\n' \
    '---' \
    'applyTo: "**"' \
    '---' \
    ''
  render_project_bootstrap "$policy_path"
}

render_target_content() {
  local render_target=$1
  local render_policy=$2
  local render_scope=$3

  case "$render_scope:$render_target" in
    user:claude|project:claude) render_claude "$render_policy" ;;
    user:codex) render_codex "$render_policy" ;;
    project:codex|project:opencode) render_project_bootstrap "$render_policy" ;;
    user:gemini|project:gemini) render_gemini "$render_policy" ;;
    user:opencode) render_opencode "$render_policy" ;;
    project:cursor) render_cursor "$render_policy" ;;
    project:windsurf) render_windsurf "$render_policy" ;;
    project:cline) render_cline "$render_policy" ;;
    project:roo) render_roo "$render_policy" ;;
    project:copilot) render_copilot "$render_policy" ;;
    *) die "no renderer for $render_scope target '$render_target'" 69 ;;
  esac
}

render_managed_block() {
  local block_target=$1
  local block_policy=$2
  local block_output=$3
  local block_scope=$4

  {
    printf '%s\n' "$OAW_BEGIN_MARKER"
    render_target_content "$block_target" "$block_policy" "$block_scope"
    printf '%s\n' "$OAW_END_MARKER"
  } >"$block_output"
}
