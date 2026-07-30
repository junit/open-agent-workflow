#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

setup_sandbox
mkdir -p "$OAW_HOME/.codex"
printf 'personal instruction\n' >"$OAW_HOME/.codex/AGENTS.md"

run_oaw install --target codex
assert_status 0 "fresh Codex install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_CODEX=$OAW_HOME/.codex/AGENTS.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-codex-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-codex-block

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created for Codex"
[ -f "$OAW_CODEX" ] || fail "Codex instructions were not created at the user destination"
[ -f "$OAW_INSTALL_STATE" ] || fail "Codex installation state was not created"

printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  "For every new top-level engineering task that may use workflow skills, first read \`$OAW_POLICY\`, run its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  '<!-- END OPEN AGENT WORKFLOW -->' >"$OAW_EXPECTED_BLOCK"

awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_CODEX" >"$OAW_ACTUAL_BLOCK"

cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "Codex managed block does not contain the exact target-native instruction"
[ "$(awk '$0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { count++ } END { print count + 0 }' "$OAW_CODEX")" -eq 1 ] ||
  fail "Codex instructions do not contain exactly one managed block"
grep -Fx 'personal instruction' "$OAW_CODEX" >/dev/null ||
  fail "existing Codex instructions were not preserved"
if grep -Fx "@$OAW_POLICY" "$OAW_CODEX" >/dev/null; then
  fail "Codex instructions incorrectly use a standalone Markdown import"
fi
awk -F '\t' -v expected_path="$OAW_CODEX" '
  $1 == "target" && $2 == "codex" && $3 == expected_path &&
    $4 == "managed-block" && $6 == "existing-file" { found++ }
  END { exit(found == 1 ? 0 : 1) }
' "$OAW_INSTALL_STATE" || fail "Codex state target row is missing or invalid"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_CODEX_BEFORE=$(cksum <"$OAW_CODEX")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target codex
assert_status 0 "repeated Codex install"
assert_contains "unchanged: codex" "repeated Codex install reports an unchanged target"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated Codex install changed canonical policy bytes"
[ "$(cksum <"$OAW_CODEX")" = "$OAW_CODEX_BEFORE" ] ||
  fail "repeated Codex install changed instruction bytes"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "repeated Codex install changed state bytes"

pass "Codex installs a target-native entrypoint idempotently"

cleanup_sandbox
setup_sandbox
mkdir -p "$OAW_HOME/.gemini"
printf 'personal instruction\n' >"$OAW_HOME/.gemini/GEMINI.md"

run_oaw install --target gemini
assert_status 0 "fresh Gemini install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_GEMINI=$OAW_HOME/.gemini/GEMINI.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-gemini-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-gemini-block

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created for Gemini"
[ -f "$OAW_GEMINI" ] || fail "Gemini instructions were not created at the user destination"
[ -f "$OAW_INSTALL_STATE" ] || fail "Gemini installation state was not created"

printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  'Follow the Open Agent Workflow policy before engineering lifecycle work:' \
  "@$OAW_POLICY" \
  '<!-- END OPEN AGENT WORKFLOW -->' >"$OAW_EXPECTED_BLOCK"

awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_GEMINI" >"$OAW_ACTUAL_BLOCK"

cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "Gemini managed block does not contain the exact target-native instruction"
[ "$(awk '$0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { count++ } END { print count + 0 }' "$OAW_GEMINI")" -eq 1 ] ||
  fail "Gemini instructions do not contain exactly one managed block"
grep -Fx 'personal instruction' "$OAW_GEMINI" >/dev/null ||
  fail "existing Gemini instructions were not preserved"
grep -Fx "@$OAW_POLICY" "$OAW_GEMINI" >/dev/null ||
  fail "Gemini instructions do not import the canonical policy"
awk -F '\t' -v expected_path="$OAW_GEMINI" '
  $1 == "target" && $2 == "gemini" && $3 == expected_path &&
    $4 == "managed-block" && $6 == "existing-file" { found++ }
  END { exit(found == 1 ? 0 : 1) }
' "$OAW_INSTALL_STATE" || fail "Gemini state target row is missing or invalid"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_GEMINI_BEFORE=$(cksum <"$OAW_GEMINI")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target gemini
assert_status 0 "repeated Gemini install"
assert_contains "unchanged: gemini" "repeated Gemini install reports an unchanged target"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated Gemini install changed canonical policy bytes"
[ "$(cksum <"$OAW_GEMINI")" = "$OAW_GEMINI_BEFORE" ] ||
  fail "repeated Gemini install changed instruction bytes"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "repeated Gemini install changed state bytes"

pass "Gemini installs a target-native entrypoint idempotently"

cleanup_sandbox
setup_sandbox
mkdir -p "$OAW_CONFIG/opencode"
printf 'personal instruction\n' >"$OAW_CONFIG/opencode/AGENTS.md"

run_oaw install --target opencode
assert_status 0 "fresh OpenCode install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_OPENCODE=$OAW_CONFIG/opencode/AGENTS.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-opencode-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-opencode-block

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created for OpenCode"
[ -f "$OAW_OPENCODE" ] || fail "OpenCode instructions were not created at the user destination"
[ -f "$OAW_INSTALL_STATE" ] || fail "OpenCode installation state was not created"

printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  "Before engineering lifecycle work, use the Read tool to read \`$OAW_POLICY\`, then follow its blocking selection gate and lifecycle lock." \
  '<!-- END OPEN AGENT WORKFLOW -->' >"$OAW_EXPECTED_BLOCK"

awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_OPENCODE" >"$OAW_ACTUAL_BLOCK"

cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "OpenCode managed block does not contain the exact target-native instruction"
[ "$(awk '$0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { count++ } END { print count + 0 }' "$OAW_OPENCODE")" -eq 1 ] ||
  fail "OpenCode instructions do not contain exactly one managed block"
grep -Fx 'personal instruction' "$OAW_OPENCODE" >/dev/null ||
  fail "existing OpenCode instructions were not preserved"
if grep -Fx "@$OAW_POLICY" "$OAW_OPENCODE" >/dev/null; then
  fail "OpenCode instructions incorrectly use a standalone Markdown import"
fi
awk -F '\t' -v expected_path="$OAW_OPENCODE" '
  $1 == "target" && $2 == "opencode" && $3 == expected_path &&
    $4 == "managed-block" && $6 == "existing-file" { found++ }
  END { exit(found == 1 ? 0 : 1) }
' "$OAW_INSTALL_STATE" || fail "OpenCode state target row is missing or invalid"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_OPENCODE_BEFORE=$(cksum <"$OAW_OPENCODE")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target opencode
assert_status 0 "repeated OpenCode install"
assert_contains "unchanged: opencode" "repeated OpenCode install reports an unchanged target"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated OpenCode install changed canonical policy bytes"
[ "$(cksum <"$OAW_OPENCODE")" = "$OAW_OPENCODE_BEFORE" ] ||
  fail "repeated OpenCode install changed instruction bytes"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "repeated OpenCode install changed state bytes"

pass "OpenCode installs a target-native entrypoint idempotently"

cleanup_sandbox
setup_sandbox
run_oaw install --target claude,codex
assert_status 69 "multi-target install before state merging is implemented"
assert_contains "Ticket 03 install does not yet support multiple targets" \
  "multi-target install reports the temporary Task 1 boundary"
assert_read_only_roots

pass "Task 1 rejects multi-target installs before mutation"
