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

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_CLAUDE_BEFORE=$(cksum <"$OAW_CLAUDE")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
sleep 1
run_oaw install --target claude
assert_status 0 "repeated Claude install"
assert_contains "unchanged: claude" "repeated install reports unchanged target"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated install changed canonical policy bytes"
[ "$(cksum <"$OAW_CLAUDE")" = "$OAW_CLAUDE_BEFORE" ] ||
  fail "repeated install changed Claude instruction bytes"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "repeated install changed state bytes"
pass "repeated Claude install is idempotent"

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.1-local\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nLOCAL UPDATE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw update --target claude
assert_status 0 "local checkout update"
grep -F 'LOCAL UPDATE SENTINEL' "$OAW_POLICY" >/dev/null ||
  fail "update did not use the local checkout policy"
grep -F "$(printf 'version\t0.1.1-local')" "$OAW_INSTALL_STATE" >/dev/null ||
  fail "update did not record the local checkout version"
pass "update uses only current checkout artifacts"

printf '0.1.2-dry-run\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nDRY RUN SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw update --target claude --dry-run
assert_status 0 "update dry run"
assert_contains "would-update" "dry run reports planned update"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "dry run changed canonical policy"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "dry run changed installation state"
if grep -F 'DRY RUN SENTINEL' "$OAW_POLICY" >/dev/null; then
  fail "dry run wrote checkout content"
fi
pass "update dry run performs no managed writes"
