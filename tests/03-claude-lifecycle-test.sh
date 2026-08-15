#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

setup_sandbox
mkdir -p "$OAW_HOME/.claude"
OAW_EXPECTED_USER_CONTENT=$OAW_SANDBOX/expected-user-content
printf 'personal instruction before\n' >"$OAW_HOME/.claude/CLAUDE.md"
printf 'personal instruction before\npersonal instruction after\n' >"$OAW_EXPECTED_USER_CONTENT"

run_oaw install --target claude
assert_status 0 "fresh Claude install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_CLAUDE=$OAW_HOME/.claude/CLAUDE.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created"
[ -f "$OAW_CLAUDE" ] || fail "Claude instructions were not created"
[ -f "$OAW_INSTALL_STATE" ] || fail "user installation state was not created"

grep -F 'personal instruction' "$OAW_CLAUDE" >/dev/null ||
  fail "existing Claude instructions were not preserved"
grep -F '<!-- BEGIN OPEN AGENT WORKFLOW -->' "$OAW_CLAUDE" >/dev/null ||
  fail "Claude managed block begin marker is missing"
grep -F 'Open Agent Workflow is opt-in.' "$OAW_CLAUDE" >/dev/null ||
  fail "Claude entrypoint is missing opt-in activation"
grep -F 'behave as the native Host' "$OAW_CLAUDE" >/dev/null ||
  fail "Claude entrypoint does not preserve Native Host behavior"
grep -F 'if the current project contains `.oaw/policy/POLICY.md`' "$OAW_CLAUDE" >/dev/null ||
  fail "Claude entrypoint does not prefer the Project Policy Set"
grep -F "otherwise read \`$OAW_POLICY\` as the User Policy Set" "$OAW_CLAUDE" >/dev/null ||
  fail "Claude entrypoint does not retain the User Policy Set path"
grep -F 'ordinary Skill invocation do not activate OAW' "$OAW_CLAUDE" >/dev/null ||
  fail "Claude entrypoint incorrectly governs normal Skill routing"
if grep -F "@$OAW_POLICY" "$OAW_CLAUDE" >/dev/null; then
  fail "Claude entrypoint incorrectly imports the canonical policy"
fi
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
printf '\nLOCAL UPDATE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
build_checkout_installer "$OAW_UPDATE_CHECKOUT"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw update --target claude
assert_status 0 "local checkout update"
grep -F 'LOCAL UPDATE SENTINEL' "$OAW_POLICY" >/dev/null ||
  fail "update did not use the local checkout policy"
grep -F "$(printf 'version\t0.1.1-local')" "$OAW_INSTALL_STATE" >/dev/null ||
  fail "update did not record the local checkout version"
pass "update uses only current checkout artifacts"

printf 'personal instruction after\n' >>"$OAW_CLAUDE"

printf '0.1.2-dry-run\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nDRY RUN SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
build_checkout_installer "$OAW_UPDATE_CHECKOUT"
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

run_oaw uninstall --target claude --dry-run
assert_status 0 "uninstall dry run"
assert_contains "would-remove" "uninstall dry run reports removals"
[ -f "$OAW_POLICY" ] || fail "uninstall dry run removed canonical policy"
[ -f "$OAW_INSTALL_STATE" ] || fail "uninstall dry run removed state"
grep -F '<!-- BEGIN OPEN AGENT WORKFLOW -->' "$OAW_CLAUDE" >/dev/null ||
  fail "uninstall dry run removed the Claude block"
pass "uninstall dry run performs no managed writes"

run_oaw uninstall --target claude
assert_status 0 "clean uninstall from existing Claude file"
[ ! -e "$OAW_POLICY" ] || fail "clean uninstall retained canonical policy"
[ ! -e "$OAW_INSTALL_STATE" ] || fail "clean uninstall retained state"
cmp -s "$OAW_CLAUDE" "$OAW_EXPECTED_USER_CONTENT" ||
  fail "clean uninstall changed existing Claude content"
if grep -F '<!-- BEGIN OPEN AGENT WORKFLOW -->' "$OAW_CLAUDE" >/dev/null; then
  fail "clean uninstall retained the managed block"
fi
case "$OAW_OUTPUT" in
  *"update: $OAW_CLAUDE"*"remove: $OAW_POLICY"*"remove: $OAW_INSTALL_STATE"*) ;;
  *) fail "uninstall did not remove managed artifacts before state (output: $OAW_OUTPUT)" ;;
esac

run_oaw uninstall --target claude
assert_status 0 "repeated uninstall"
assert_contains "unchanged: claude" "repeated uninstall reports unchanged"
pass "uninstall preserves an existing Claude file and is idempotent"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
mkdir -p "$OAW_HOME/.claude"
OAW_NO_NEWLINE_EXPECTED=$OAW_SANDBOX/expected-without-final-newline
OAW_NO_NEWLINE_CLAUDE=$OAW_HOME/.claude/CLAUDE.md
printf 'personal instruction without final newline' >"$OAW_NO_NEWLINE_EXPECTED"
cp "$OAW_NO_NEWLINE_EXPECTED" "$OAW_NO_NEWLINE_CLAUDE"
run_oaw install --target claude
assert_status 0 "install around user content without a final newline"
OAW_NO_NEWLINE_INSTALLED=$(cksum <"$OAW_NO_NEWLINE_CLAUDE")
run_oaw install --target claude
assert_status 0 "repeat install around user content without a final newline"
[ "$(cksum <"$OAW_NO_NEWLINE_CLAUDE")" = "$OAW_NO_NEWLINE_INSTALLED" ] ||
  fail "repeated install changed user content without a final newline"
run_oaw uninstall --target claude
assert_status 0 "uninstall around user content without a final newline"
cmp -s "$OAW_NO_NEWLINE_CLAUDE" "$OAW_NO_NEWLINE_EXPECTED" ||
  fail "uninstall changed user content without a final newline"
pass "uninstall preserves exact user bytes without a final newline"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
run_oaw install --target claude
assert_status 0 "install into an empty home"
OAW_CREATED_CLAUDE=$OAW_HOME/.claude/CLAUDE.md
[ -f "$OAW_CREATED_CLAUDE" ] || fail "install did not create Claude instructions"
run_oaw uninstall --target claude
assert_status 0 "uninstall OAW-created Claude file"
[ ! -e "$OAW_CREATED_CLAUDE" ] ||
  fail "uninstall retained an otherwise empty OAW-created file"
pass "uninstall removes an OAW-created Claude file"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
run_oaw install --target claude
assert_status 0 "install before shared policy reference test"
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_CLAUDE=$OAW_HOME/.claude/CLAUDE.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_OTHER_PROJECT="$OAW_SANDBOX/other project"
mkdir -p "$OAW_OTHER_PROJECT"
run_oaw install --project "$OAW_OTHER_PROJECT" --target claude
assert_status 0 "install canonical project policy reference"
OAW_OTHER_PROJECT=$(CDPATH='' cd -P -- "$OAW_OTHER_PROJECT" && pwd -P)
OAW_OTHER_PROJECT_ID=$(printf '%s' "$OAW_OTHER_PROJECT" | cksum | awk '{ print $1 "-" $2 }')
OAW_OTHER_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_OTHER_PROJECT_ID.state
OAW_OTHER_CLAUDE=$OAW_OTHER_PROJECT/.claude/CLAUDE.md
OAW_OTHER_POLICY=$OAW_OTHER_PROJECT/.oaw/policy/POLICY.md

run_oaw uninstall --target claude
assert_status 0 "uninstall with another policy reference"
[ ! -e "$OAW_POLICY" ] ||
  fail "uninstall retained the user-owned canonical policy"
[ ! -e "$OAW_CLAUDE" ] ||
  fail "uninstall retained the current state's created Claude file"
[ ! -e "$OAW_INSTALL_STATE" ] ||
  fail "uninstall retained current state after removing its managed target"
[ -f "$OAW_OTHER_POLICY" ] ||
  fail "uninstall removed another project's canonical Policy Set"
[ -f "$OAW_OTHER_STATE" ] ||
  fail "uninstall removed another installation state"
[ -f "$OAW_OTHER_CLAUDE" ] ||
  fail "uninstall removed another installation target"
case "$OAW_OUTPUT" in
  *"remove: $OAW_CLAUDE"*"remove: $OAW_INSTALL_STATE"*) ;;
  *) fail "uninstall did not remove managed target before current state (output: $OAW_OUTPUT)" ;;
esac
pass "uninstall keeps user and project Policy Set ownership independent"
