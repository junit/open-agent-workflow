#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT/.cursor/rules"
printf 'personal Cursor rule\n' >"$OAW_PROJECT/.cursor/rules/personal.mdc"
OAW_CURSOR_SIBLING_BEFORE=$(cksum <"$OAW_PROJECT/.cursor/rules/personal.mdc")

run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "fresh project Cursor install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_CURSOR=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_CURSOR=$OAW_SANDBOX/expected-cursor.mdc

printf '%s\n' \
  '---' \
  'description: Open Agent Workflow lifecycle policy' \
  'globs: "**/*"' \
  'alwaysApply: true' \
  '---' \
  '' \
  "Before engineering lifecycle work, read \`$OAW_POLICY\`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  >"$OAW_EXPECTED_CURSOR"
cmp -s "$OAW_EXPECTED_CURSOR" "$OAW_CURSOR" || fail "Cursor adapter bytes are invalid"
grep -F "$(printf 'target\tcursor\t%s\towned-file' "$OAW_CURSOR")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Cursor ownership"
[ "$(cksum <"$OAW_PROJECT/.cursor/rules/personal.mdc")" = "$OAW_CURSOR_SIBLING_BEFORE" ] ||
  fail "Cursor install changed a sibling rule"

OAW_CURSOR_BEFORE=$(cksum <"$OAW_CURSOR")
OAW_CURSOR_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "repeated project Cursor install"
assert_contains "unchanged: cursor" "repeated project install reports Cursor unchanged"
[ "$(cksum <"$OAW_CURSOR")" = "$OAW_CURSOR_BEFORE" ] ||
  fail "repeated Cursor install changed adapter bytes"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_CURSOR_STATE_BEFORE" ] ||
  fail "repeated Cursor install changed project state"

printf 'local Cursor drift\n' >"$OAW_CURSOR"
OAW_CURSOR_DRIFT=$(cksum <"$OAW_CURSOR")
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_CURSOR_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 65 "drifted Cursor destination"
assert_contains "owned target file has drifted" "owned-file drift is explicit"
[ "$(cksum <"$OAW_CURSOR")" = "$OAW_CURSOR_DRIFT" ] ||
  fail "Cursor drift was overwritten during failed install"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "Cursor drift changed canonical policy"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_CURSOR_STATE_BEFORE" ] ||
  fail "Cursor drift changed project state"

pass "project Cursor install renders an owned MDC rule"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
OAW_CURSOR="$OAW_PROJECT/.cursor/rules/open-agent-workflow.mdc"
OAW_CURSOR_SIBLING="$OAW_PROJECT/.cursor/rules/personal.mdc"
mkdir -p "$OAW_PROJECT/.cursor/rules"
printf 'pre-existing Cursor destination\n' >"$OAW_CURSOR"
printf 'personal Cursor rule\n' >"$OAW_CURSOR_SIBLING"
OAW_CURSOR_BEFORE=$(cksum <"$OAW_CURSOR")
OAW_CURSOR_SIBLING_BEFORE=$(cksum <"$OAW_CURSOR_SIBLING")

run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 65 "pre-existing Cursor destination"
assert_contains "owned target already exists" "pre-existing owned destination is explicit"
[ "$(cksum <"$OAW_CURSOR")" = "$OAW_CURSOR_BEFORE" ] ||
  fail "Cursor conflict changed the owned destination"
[ "$(cksum <"$OAW_CURSOR_SIBLING")" = "$OAW_CURSOR_SIBLING_BEFORE" ] ||
  fail "Cursor conflict changed a sibling rule"
assert_read_only_roots

pass "pre-existing owned Cursor destination fails before mutation"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT/.devin/rules"
printf 'personal Windsurf rule\n' >"$OAW_PROJECT/.devin/rules/personal.md"
OAW_WINDSURF_SIBLING_BEFORE=$(cksum <"$OAW_PROJECT/.devin/rules/personal.md")

run_oaw install --project "$OAW_PROJECT" --target windsurf
assert_status 0 "fresh project Windsurf install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_WINDSURF=$OAW_PROJECT_PHYSICAL/.devin/rules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_WINDSURF=$OAW_SANDBOX/expected-windsurf.md

printf '%s\n' \
  '---' \
  'trigger: always_on' \
  '---' \
  '' \
  "Before engineering lifecycle work, read \`$OAW_POLICY\`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  >"$OAW_EXPECTED_WINDSURF"
cmp -s "$OAW_EXPECTED_WINDSURF" "$OAW_WINDSURF" || fail "Windsurf adapter bytes are invalid"
grep -F "$(printf 'target\twindsurf\t%s\towned-file' "$OAW_WINDSURF")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Windsurf ownership"
[ "$(cksum <"$OAW_PROJECT/.devin/rules/personal.md")" = "$OAW_WINDSURF_SIBLING_BEFORE" ] ||
  fail "Windsurf install changed a sibling rule"

OAW_WINDSURF_BEFORE=$(cksum <"$OAW_WINDSURF")
OAW_WINDSURF_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target windsurf
assert_status 0 "repeated project Windsurf install"
assert_contains "unchanged: windsurf" "repeated project install reports Windsurf unchanged"
[ "$(cksum <"$OAW_WINDSURF")" = "$OAW_WINDSURF_BEFORE" ] ||
  fail "repeated Windsurf install changed adapter bytes"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_WINDSURF_STATE_BEFORE" ] ||
  fail "repeated Windsurf install changed project state"

pass "project Windsurf install renders an always-on rule"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT/.clinerules"
printf 'personal Cline rule\n' >"$OAW_PROJECT/.clinerules/personal.md"
OAW_CLINE_SIBLING_BEFORE=$(cksum <"$OAW_PROJECT/.clinerules/personal.md")

run_oaw install --project "$OAW_PROJECT" --target cline
assert_status 0 "fresh project Cline install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_CLINE=$OAW_PROJECT_PHYSICAL/.clinerules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_CLINE=$OAW_SANDBOX/expected-cline.md

printf '%s\n' \
  "Before engineering lifecycle work, read \`$OAW_POLICY\`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  >"$OAW_EXPECTED_CLINE"
cmp -s "$OAW_EXPECTED_CLINE" "$OAW_CLINE" || fail "Cline adapter bytes are invalid"
grep -F "$(printf 'target\tcline\t%s\towned-file' "$OAW_CLINE")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Cline ownership"
[ "$(cksum <"$OAW_PROJECT/.clinerules/personal.md")" = "$OAW_CLINE_SIBLING_BEFORE" ] ||
  fail "Cline install changed a sibling rule"

OAW_CLINE_BEFORE=$(cksum <"$OAW_CLINE")
OAW_CLINE_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target cline
assert_status 0 "repeated project Cline install"
assert_contains "unchanged: cline" "repeated project install reports Cline unchanged"
[ "$(cksum <"$OAW_CLINE")" = "$OAW_CLINE_BEFORE" ] ||
  fail "repeated Cline install changed adapter bytes"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_CLINE_STATE_BEFORE" ] ||
  fail "repeated Cline install changed project state"

pass "project Cline install renders an owned rule"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT/.roo/rules"
printf 'personal Roo rule\n' >"$OAW_PROJECT/.roo/rules/personal.md"
OAW_ROO_SIBLING_BEFORE=$(cksum <"$OAW_PROJECT/.roo/rules/personal.md")

run_oaw install --project "$OAW_PROJECT" --target roo
assert_status 0 "fresh project Roo install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_ROO=$OAW_PROJECT_PHYSICAL/.roo/rules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_ROO=$OAW_SANDBOX/expected-roo.md

printf '%s\n' \
  "Before engineering lifecycle work, read \`$OAW_POLICY\`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  >"$OAW_EXPECTED_ROO"
cmp -s "$OAW_EXPECTED_ROO" "$OAW_ROO" || fail "Roo adapter bytes are invalid"
grep -F "$(printf 'target\troo\t%s\towned-file' "$OAW_ROO")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Roo ownership"
[ "$(cksum <"$OAW_PROJECT/.roo/rules/personal.md")" = "$OAW_ROO_SIBLING_BEFORE" ] ||
  fail "Roo install changed a sibling rule"

OAW_ROO_BEFORE=$(cksum <"$OAW_ROO")
OAW_ROO_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target roo
assert_status 0 "repeated project Roo install"
assert_contains "unchanged: roo" "repeated project install reports Roo unchanged"
[ "$(cksum <"$OAW_ROO")" = "$OAW_ROO_BEFORE" ] ||
  fail "repeated Roo install changed adapter bytes"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_ROO_STATE_BEFORE" ] ||
  fail "repeated Roo install changed project state"

pass "project Roo install renders an owned rule"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT/.github/instructions"
printf 'personal Copilot instruction\n' \
  >"$OAW_PROJECT/.github/instructions/personal.instructions.md"
OAW_COPILOT_SIBLING_BEFORE=$(cksum \
  <"$OAW_PROJECT/.github/instructions/personal.instructions.md")

run_oaw install --project "$OAW_PROJECT" --target copilot
assert_status 0 "fresh project Copilot install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_COPILOT=$OAW_PROJECT_PHYSICAL/.github/instructions/open-agent-workflow.instructions.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_COPILOT=$OAW_SANDBOX/expected-copilot.instructions.md

printf '%s\n' \
  '---' \
  'applyTo: "**"' \
  '---' \
  '' \
  "Before engineering lifecycle work, read \`$OAW_POLICY\`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task." \
  >"$OAW_EXPECTED_COPILOT"
cmp -s "$OAW_EXPECTED_COPILOT" "$OAW_COPILOT" || fail "Copilot adapter bytes are invalid"
grep -F "$(printf 'target\tcopilot\t%s\towned-file' "$OAW_COPILOT")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "project state does not record Copilot ownership"
[ "$(cksum <"$OAW_PROJECT/.github/instructions/personal.instructions.md")" = \
  "$OAW_COPILOT_SIBLING_BEFORE" ] || fail "Copilot install changed a sibling instruction"

OAW_COPILOT_BEFORE=$(cksum <"$OAW_COPILOT")
OAW_COPILOT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target copilot
assert_status 0 "repeated project Copilot install"
assert_contains "unchanged: copilot" "repeated project install reports Copilot unchanged"
[ "$(cksum <"$OAW_COPILOT")" = "$OAW_COPILOT_BEFORE" ] ||
  fail "repeated Copilot install changed adapter bytes"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_COPILOT_STATE_BEFORE" ] ||
  fail "repeated Copilot install changed project state"

pass "project Copilot install renders a path-specific instruction"

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
mkdir -p "$OAW_PROJECT"
printf 'personal shared project instruction\n' >"$OAW_PROJECT/AGENTS.md"

run_oaw install --project "$OAW_PROJECT" --target codex,opencode
assert_status 0 "shared project AGENTS install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-shared-agents-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-shared-agents-block

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
  fail "shared project AGENTS block is not canonical"
[ "$(awk '$0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { count++ } END { print count + 0 }' \
  "$OAW_PROJECT_AGENTS")" -eq 1 ] || fail "shared project AGENTS has duplicate OAW blocks"
OAW_CODEX_CHECKSUM=$(awk -F '\t' '$1 == "target" && $2 == "codex" { print $4 }' "$OAW_PROJECT_STATE")
OAW_OPENCODE_CHECKSUM=$(awk -F '\t' '$1 == "target" && $2 == "opencode" { print $4 }' "$OAW_PROJECT_STATE")
[ -n "$OAW_CODEX_CHECKSUM" ] && [ "$OAW_CODEX_CHECKSUM" = "$OAW_OPENCODE_CHECKSUM" ] ||
  fail "shared project AGENTS targets do not share one checksum"
[ "$(awk -F '\t' '$1 == "target" { count++ } END { print count + 0 }' "$OAW_PROJECT_STATE")" -eq 2 ] ||
  fail "shared project state does not retain both target rows"

OAW_PROJECT_AGENTS_BEFORE=$(cksum <"$OAW_PROJECT_AGENTS")
run_oaw uninstall --project "$OAW_PROJECT" --target codex
assert_status 0 "selected shared AGENTS uninstall"
[ "$(cksum <"$OAW_PROJECT_AGENTS")" = "$OAW_PROJECT_AGENTS_BEFORE" ] ||
  fail "selected shared AGENTS uninstall changed the block"
[ "$(awk -F '\t' '$1 == "target" { print $2 }' "$OAW_PROJECT_STATE")" = opencode ] ||
  fail "selected shared AGENTS uninstall removed the wrong state row"

run_oaw uninstall --project "$OAW_PROJECT" --target opencode
assert_status 0 "final shared AGENTS uninstall"
grep -Fx 'personal shared project instruction' "$OAW_PROJECT_AGENTS" >/dev/null ||
  fail "final shared AGENTS uninstall removed project instructions"
if grep -F '<!-- BEGIN OPEN AGENT WORKFLOW -->' "$OAW_PROJECT_AGENTS" >/dev/null 2>&1; then
  fail "final shared AGENTS uninstall left the OAW block"
fi
[ ! -e "$OAW_PROJECT_STATE" ] || fail "final shared AGENTS uninstall kept project state"
[ ! -e "$OAW_POLICY" ] || fail "final shared AGENTS uninstall kept an unreferenced policy"

pass "shared project AGENTS ownership survives selected uninstall"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"
printf 'personal cross-scope project instruction\n' >"$OAW_PROJECT/AGENTS.md"

run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "cross-scope project fixture install"
run_oaw install --target codex
assert_status 0 "cross-scope user fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state

run_oaw uninstall --target codex
assert_status 0 "cross-scope user uninstall"
[ -f "$OAW_POLICY" ] || fail "user uninstall removed policy referenced by project state"
[ -f "$OAW_PROJECT_STATE" ] || fail "user uninstall removed project state"
[ -f "$OAW_PROJECT_AGENTS" ] || fail "user uninstall changed project adapter"
[ ! -e "$OAW_USER_STATE" ] || fail "user uninstall kept user state"

run_oaw uninstall --project "$OAW_PROJECT" --target codex
assert_status 0 "cross-scope project uninstall"
[ ! -e "$OAW_POLICY" ] || fail "final cross-scope uninstall kept policy"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "final cross-scope uninstall kept project state"

pass "canonical policy survives until the final cross-scope reference"

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
