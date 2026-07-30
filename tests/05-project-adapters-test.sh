#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 69 "unsupported project target preserves destination resolution failure"
assert_output_equals "oaw: error: target 'cursor' is not implemented for project scope" \
  "unsupported project target reports one precise error"
assert_read_only_roots
assert_empty_dir "$OAW_PROJECT" "unsupported project target must not mutate the project"

pass "unsupported project target fails before destination preflight"

cleanup_sandbox
setup_sandbox
OAW_PROJECT=$OAW_SANDBOX/project\ with\ spaces
mkdir -p "$OAW_PROJECT/.claude"
printf 'personal project Claude instruction\n' >"$OAW_PROJECT/.claude/CLAUDE.md"

mkdir -p "$OAW_HOME/.claude" "$OAW_HOME/.codex" "$OAW_HOME/.gemini" "$OAW_CONFIG/opencode"
printf 'user Claude sentinel\n' >"$OAW_HOME/.claude/CLAUDE.md"
printf 'user Codex sentinel\n' >"$OAW_HOME/.codex/AGENTS.md"
printf 'user Gemini sentinel\n' >"$OAW_HOME/.gemini/GEMINI.md"
printf 'user OpenCode sentinel\n' >"$OAW_CONFIG/opencode/AGENTS.md"

OAW_USER_CLAUDE_BEFORE=$(cksum <"$OAW_HOME/.claude/CLAUDE.md")
OAW_USER_CODEX_BEFORE=$(cksum <"$OAW_HOME/.codex/AGENTS.md")
OAW_USER_GEMINI_BEFORE=$(cksum <"$OAW_HOME/.gemini/GEMINI.md")
OAW_USER_OPENCODE_BEFORE=$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")

run_oaw install --project "$OAW_PROJECT" --target claude
assert_status 0 "fresh project Claude install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_CLAUDE=$OAW_PROJECT_PHYSICAL/.claude/CLAUDE.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state

[ -f "$OAW_POLICY" ] || fail "project install did not create the canonical policy"
[ -f "$OAW_PROJECT_STATE" ] || fail "project install did not create identity-scoped state"
grep -F "@$OAW_POLICY" "$OAW_PROJECT_CLAUDE" >/dev/null ||
  fail "project Claude entrypoint does not import the canonical policy"
grep -Fx 'personal project Claude instruction' "$OAW_PROJECT_CLAUDE" >/dev/null ||
  fail "project Claude install did not preserve project instructions"
grep -F "$(printf 'scope\tproject')" "$OAW_PROJECT_STATE" >/dev/null ||
  fail "project state does not record project scope"
grep -F "$(printf 'project\t%s' "$OAW_PROJECT_PHYSICAL")" "$OAW_PROJECT_STATE" >/dev/null ||
  fail "project state does not bind the physical project root"
grep -F "$(printf 'target\tclaude\t%s\tmanaged-block' "$OAW_PROJECT_CLAUDE")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Claude destination"
[ ! -e "$OAW_STATE/open-agent-workflow/installations/user.state" ] ||
  fail "project install created user-scoped state"

[ "$(cksum <"$OAW_HOME/.claude/CLAUDE.md")" = "$OAW_USER_CLAUDE_BEFORE" ] ||
  fail "project install changed user Claude instructions"
[ "$(cksum <"$OAW_HOME/.codex/AGENTS.md")" = "$OAW_USER_CODEX_BEFORE" ] ||
  fail "project install changed user Codex instructions"
[ "$(cksum <"$OAW_HOME/.gemini/GEMINI.md")" = "$OAW_USER_GEMINI_BEFORE" ] ||
  fail "project install changed user Gemini instructions"
[ "$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")" = "$OAW_USER_OPENCODE_BEFORE" ] ||
  fail "project install changed user OpenCode instructions"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_PROJECT_CLAUDE_BEFORE=$(cksum <"$OAW_PROJECT_CLAUDE")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target claude
assert_status 0 "repeated project Claude install"
assert_contains "unchanged: claude" "repeated project install reports Claude unchanged"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated project install changed the canonical policy"
[ "$(cksum <"$OAW_PROJECT_CLAUDE")" = "$OAW_PROJECT_CLAUDE_BEFORE" ] ||
  fail "repeated project install changed Claude instructions"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "repeated project install changed project state"

pass "project Claude install uses identity-scoped state without user-target mutation"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT" "$OAW_HOME/.codex" "$OAW_CONFIG/opencode"
printf 'personal shared project instruction\n' >"$OAW_PROJECT/AGENTS.md"
printf 'user Codex sentinel\n' >"$OAW_HOME/.codex/AGENTS.md"
printf 'user OpenCode sentinel\n' >"$OAW_CONFIG/opencode/AGENTS.md"
OAW_USER_CODEX_BEFORE=$(cksum <"$OAW_HOME/.codex/AGENTS.md")
OAW_USER_OPENCODE_BEFORE=$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")

run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "fresh project Codex install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-project-agents-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-project-agents-block

printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  "Before engineering lifecycle work, read \`$OAW_POLICY\`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  '<!-- END OPEN AGENT WORKFLOW -->' >"$OAW_EXPECTED_BLOCK"
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_PROJECT_AGENTS" >"$OAW_ACTUAL_BLOCK"
cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "project Codex does not use the shared AGENTS bootstrap"
grep -Fx 'personal shared project instruction' "$OAW_PROJECT_AGENTS" >/dev/null ||
  fail "project Codex install did not preserve project AGENTS content"
grep -F "$(printf 'target\tcodex\t%s\tmanaged-block' "$OAW_PROJECT_AGENTS")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Codex destination"
[ "$(cksum <"$OAW_HOME/.codex/AGENTS.md")" = "$OAW_USER_CODEX_BEFORE" ] ||
  fail "project Codex install changed user Codex instructions"
[ "$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")" = "$OAW_USER_OPENCODE_BEFORE" ] ||
  fail "project Codex install changed user OpenCode instructions"

OAW_PROJECT_AGENTS_BEFORE=$(cksum <"$OAW_PROJECT_AGENTS")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "repeated project Codex install"
assert_contains "unchanged: codex" "repeated project install reports Codex unchanged"
[ "$(cksum <"$OAW_PROJECT_AGENTS")" = "$OAW_PROJECT_AGENTS_BEFORE" ] ||
  fail "repeated project Codex install changed AGENTS instructions"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "repeated project Codex install changed project state"

pass "project Codex install uses the shared AGENTS bootstrap"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT" "$OAW_HOME/.gemini"
printf 'personal project Gemini instruction\n' >"$OAW_PROJECT/GEMINI.md"
printf 'user Gemini sentinel\n' >"$OAW_HOME/.gemini/GEMINI.md"
OAW_USER_GEMINI_BEFORE=$(cksum <"$OAW_HOME/.gemini/GEMINI.md")

run_oaw install --project "$OAW_PROJECT" --target gemini
assert_status 0 "fresh project Gemini install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_GEMINI=$OAW_PROJECT_PHYSICAL/GEMINI.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-project-gemini-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-project-gemini-block

printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  'Follow the Open Agent Workflow policy before engineering lifecycle work:' \
  "@$OAW_POLICY" \
  '<!-- END OPEN AGENT WORKFLOW -->' >"$OAW_EXPECTED_BLOCK"
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_PROJECT_GEMINI" >"$OAW_ACTUAL_BLOCK"
cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "project Gemini does not use its native policy import"
grep -Fx 'personal project Gemini instruction' "$OAW_PROJECT_GEMINI" >/dev/null ||
  fail "project Gemini install did not preserve project instructions"
grep -F "$(printf 'target\tgemini\t%s\tmanaged-block' "$OAW_PROJECT_GEMINI")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Gemini destination"
[ "$(cksum <"$OAW_HOME/.gemini/GEMINI.md")" = "$OAW_USER_GEMINI_BEFORE" ] ||
  fail "project Gemini install changed user Gemini instructions"

OAW_PROJECT_GEMINI_BEFORE=$(cksum <"$OAW_PROJECT_GEMINI")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target gemini
assert_status 0 "repeated project Gemini install"
assert_contains "unchanged: gemini" "repeated project install reports Gemini unchanged"
[ "$(cksum <"$OAW_PROJECT_GEMINI")" = "$OAW_PROJECT_GEMINI_BEFORE" ] ||
  fail "repeated project Gemini install changed GEMINI instructions"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "repeated project Gemini install changed project state"

pass "project Gemini install uses its native policy import"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT" "$OAW_CONFIG/opencode"
printf 'personal shared project instruction\n' >"$OAW_PROJECT/AGENTS.md"
printf 'user OpenCode sentinel\n' >"$OAW_CONFIG/opencode/AGENTS.md"
OAW_USER_OPENCODE_BEFORE=$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")

run_oaw install --project "$OAW_PROJECT" --target opencode
assert_status 0 "fresh project OpenCode install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-project-opencode-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-project-opencode-block

printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  "Before engineering lifecycle work, read \`$OAW_POLICY\`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  '<!-- END OPEN AGENT WORKFLOW -->' >"$OAW_EXPECTED_BLOCK"
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_PROJECT_AGENTS" >"$OAW_ACTUAL_BLOCK"
cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "project OpenCode does not use the shared AGENTS bootstrap"
grep -Fx 'personal shared project instruction' "$OAW_PROJECT_AGENTS" >/dev/null ||
  fail "project OpenCode install did not preserve project AGENTS content"
grep -F "$(printf 'target\topencode\t%s\tmanaged-block' "$OAW_PROJECT_AGENTS")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record OpenCode destination"
[ "$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")" = "$OAW_USER_OPENCODE_BEFORE" ] ||
  fail "project OpenCode install changed user OpenCode instructions"

OAW_PROJECT_AGENTS_BEFORE=$(cksum <"$OAW_PROJECT_AGENTS")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target opencode
assert_status 0 "repeated project OpenCode install"
assert_contains "unchanged: opencode" "repeated project install reports OpenCode unchanged"
[ "$(cksum <"$OAW_PROJECT_AGENTS")" = "$OAW_PROJECT_AGENTS_BEFORE" ] ||
  fail "repeated project OpenCode install changed AGENTS instructions"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "repeated project OpenCode install changed project state"

pass "project OpenCode install uses the shared AGENTS bootstrap"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
OAW_OTHER_PROJECT="$OAW_SANDBOX/other project"
mkdir -p "$OAW_PROJECT/.claude" "$OAW_OTHER_PROJECT"
printf 'personal project Claude instruction\n' >"$OAW_PROJECT/.claude/CLAUDE.md"

run_oaw install --project "$OAW_PROJECT" --target claude
assert_status 0 "project root mismatch fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_OTHER_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_OTHER_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_CLAUDE=$OAW_PROJECT_PHYSICAL/.claude/CLAUDE.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_TAMPERED_STATE=$OAW_SANDBOX/tampered-project.state

awk -F '\t' -v project_root="$OAW_OTHER_PROJECT_PHYSICAL" '
  BEGIN { OFS = "\t" }
  $1 == "project" { $2 = project_root }
  { print }
' "$OAW_PROJECT_STATE" >"$OAW_TAMPERED_STATE"
mv "$OAW_TAMPERED_STATE" "$OAW_PROJECT_STATE"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_PROJECT_CLAUDE_BEFORE=$(cksum <"$OAW_PROJECT_CLAUDE")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target claude
assert_status 65 "stored project root mismatch"
assert_contains "installed project root does not match" "stored project root mismatch is explicit"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "project root mismatch changed the canonical policy"
[ "$(cksum <"$OAW_PROJECT_CLAUDE")" = "$OAW_PROJECT_CLAUDE_BEFORE" ] ||
  fail "project root mismatch changed project instructions"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "project root mismatch rewrote installation state"
assert_empty_dir "$OAW_OTHER_PROJECT" "project root mismatch must not mutate the stored root"

pass "stored project root mismatch fails before mutation"
