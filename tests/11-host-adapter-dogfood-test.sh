#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

target_path_for_dogfood() {
  case "$1" in
    claude) printf '%s/.claude/CLAUDE.md\n' "$2" ;;
    codex|opencode) printf '%s/AGENTS.md\n' "$2" ;;
    gemini) printf '%s/GEMINI.md\n' "$2" ;;
    cursor) printf '%s/.cursor/rules/open-agent-workflow.mdc\n' "$2" ;;
    windsurf) printf '%s/.devin/rules/open-agent-workflow.md\n' "$2" ;;
    cline) printf '%s/.clinerules/open-agent-workflow.md\n' "$2" ;;
    roo) printf '%s/.roo/rules/open-agent-workflow.md\n' "$2" ;;
    copilot) printf '%s/.github/instructions/open-agent-workflow.instructions.md\n' "$2" ;;
    *) fail "unknown dogfood target: $1" ;;
  esac
}

for OAW_DOGFOOD_HOST in \
  claude codex gemini opencode cursor windsurf cline roo copilot; do
  cleanup_sandbox
  setup_sandbox

  OAW_DOGFOOD_RELEASE=$OAW_SANDBOX/release
  mkdir -p "$OAW_DOGFOOD_RELEASE"
  cp "$OAW_BASE_INSTALLER" "$OAW_DOGFOOD_RELEASE/install.sh"
  cp "$(dirname -- "$OAW_BASE_INSTALLER")/oaw" "$OAW_DOGFOOD_RELEASE/oaw"
  chmod 755 "$OAW_DOGFOOD_RELEASE/install.sh" "$OAW_DOGFOOD_RELEASE/oaw"
  OAW_INSTALLER=$OAW_DOGFOOD_RELEASE/install.sh

  run_oaw install --project "$OAW_PROJECT" --target "$OAW_DOGFOOD_HOST"
  assert_status 0 "dogfood install for $OAW_DOGFOOD_HOST"

  OAW_DOGFOOD_PROJECT=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  OAW_DOGFOOD_POLICY=$OAW_DOGFOOD_PROJECT/.oaw/policy
  OAW_DOGFOOD_ROUTER=$(target_path_for_dogfood "$OAW_DOGFOOD_HOST" "$OAW_DOGFOOD_PROJECT")
  OAW_DOGFOOD_ADAPTER=$OAW_DOGFOOD_POLICY/adapters/$OAW_DOGFOOD_HOST-policy.md

  [ -f "$OAW_DOGFOOD_ROUTER" ] || fail "$OAW_DOGFOOD_HOST Router was not installed"
  [ -f "$OAW_DOGFOOD_POLICY/POLICY.md" ] || fail "$OAW_DOGFOOD_HOST Policy was not installed"
  [ -f "$OAW_DOGFOOD_POLICY/cooperative-protocol.md" ] ||
    fail "$OAW_DOGFOOD_HOST protocol was not installed"
  [ -f "$OAW_DOGFOOD_ADAPTER" ] || fail "$OAW_DOGFOOD_HOST Adapter was not installed"

  rm -f -- "$OAW_DOGFOOD_RELEASE/oaw"
  [ ! -e "$OAW_DOGFOOD_RELEASE/oaw" ] || fail "dogfood binary remains for $OAW_DOGFOOD_HOST"

  grep -F 'Open Agent Workflow is opt-in.' "$OAW_DOGFOOD_ROUTER" >/dev/null ||
    fail "$OAW_DOGFOOD_HOST Router is not readable after binary removal"
  grep -F 'Open Agent Workflow Policy' "$OAW_DOGFOOD_POLICY/POLICY.md" >/dev/null ||
    fail "$OAW_DOGFOOD_HOST Policy is not readable after binary removal"
  grep -F 'Skill Discovery' "$OAW_DOGFOOD_ADAPTER" >/dev/null ||
    fail "$OAW_DOGFOOD_HOST Adapter is not readable after binary removal"
  grep -F 'MATT-FULL' "$OAW_DOGFOOD_POLICY/profiles/MATT-FULL.md" >/dev/null ||
    fail "$OAW_DOGFOOD_HOST Profile is not readable after binary removal"

  pass "$OAW_DOGFOOD_HOST static Policy Adapter dogfood survives binary removal"
done
