#!/usr/bin/env bash

OAW_BEGIN_MARKER='<!-- BEGIN OPEN AGENT WORKFLOW -->'
OAW_END_MARKER='<!-- END OPEN AGENT WORKFLOW -->'

render_activation_router() {
  printf 'Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, read `%s` and apply it only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.\n' "$1"
}

render_claude() {
  render_activation_router "$1"
}

render_codex() {
  render_activation_router "$1"
}

render_gemini() {
  render_activation_router "$1"
}

render_opencode() {
  render_activation_router "$1"
}

render_project_bootstrap() {
  render_activation_router "$1"
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
