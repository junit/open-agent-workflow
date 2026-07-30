#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

setup_sandbox
mkdir -p "$OAW_HOME/.claude"
printf 'personal instruction\n' >"$OAW_HOME/.claude/CLAUDE.md"

run_oaw install --target claude
assert_status 0 "fresh Claude install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_CLAUDE=$OAW_HOME/.claude/CLAUDE.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created"
[ -f "$OAW_CLAUDE" ] || fail "Claude instructions were not created"
[ -f "$OAW_INSTALL_STATE" ] || fail "user installation state was not created"

grep -F 'personal instruction' "$OAW_CLAUDE" >/dev/null ||
  fail "existing Claude instructions were not preserved"
grep -F '<!-- BEGIN OPEN AGENT WORKFLOW -->' "$OAW_CLAUDE" >/dev/null ||
  fail "Claude managed block begin marker is missing"
grep -F "@$OAW_POLICY" "$OAW_CLAUDE" >/dev/null ||
  fail "Claude entrypoint does not import the canonical policy"
grep -F 'format' "$OAW_INSTALL_STATE" >/dev/null ||
  fail "installation state has no format record"

pass "fresh Claude install creates policy, entrypoint, and inert state"
