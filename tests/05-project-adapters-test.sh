#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

OAW_PROJECT_POLICY_REFERENCE=.oaw/policy/POLICY.md

render_expected_activation_router() {
  printf 'Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, read `%s` and apply it only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.\n' "$1"
}

write_expected_router_block() {
  expected_block=$1
  expected_policy=$2
  {
    printf '%s\n' '<!-- BEGIN OPEN AGENT WORKFLOW -->'
    render_expected_activation_router "$expected_policy"
    printf '%s\n' '<!-- END OPEN AGENT WORKFLOW -->'
  } >"$expected_block"
}

assert_lazy_router_file() {
  router_file=$1
  router_policy=$2
  router_description=$3

  grep -F 'Open Agent Workflow is opt-in.' "$router_file" >/dev/null ||
    fail "$router_description is missing opt-in activation"
  grep -F 'behave as the native Host' "$router_file" >/dev/null ||
    fail "$router_description does not preserve Native Host behavior"
  grep -F "On explicit activation, read \`$router_policy\`" "$router_file" >/dev/null ||
    fail "$router_description does not retain the canonical policy path"
  grep -F 'ordinary Skill invocation do not activate OAW' "$router_file" >/dev/null ||
    fail "$router_description incorrectly governs normal Skill routing"
  if grep -F "@$router_policy" "$router_file" >/dev/null ||
    grep -F 'For every new top-level engineering request, first read' "$router_file" >/dev/null ||
    grep -F 'Before engineering lifecycle work, read' "$router_file" >/dev/null ||
    grep -F 'classify it as DIRECT, BOUNDED, or WORKFLOW' "$router_file" >/dev/null; then
    fail "$router_description retains eager OAW activation"
  fi
}

project_target_path_for_test() {
  case "$1" in
    claude) printf '.claude/CLAUDE.md\n' ;;
    codex|opencode) printf 'AGENTS.md\n' ;;
    gemini) printf 'GEMINI.md\n' ;;
    cursor) printf '.cursor/rules/open-agent-workflow.mdc\n' ;;
    windsurf) printf '.devin/rules/open-agent-workflow.md\n' ;;
    cline) printf '.clinerules/open-agent-workflow.md\n' ;;
    roo) printf '.roo/rules/open-agent-workflow.md\n' ;;
    copilot) printf '.github/instructions/open-agent-workflow.instructions.md\n' ;;
    *) fail "unknown project test target: $1" ;;
  esac
}

setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT/.cursor/rules"
printf 'personal Cursor rule\n' >"$OAW_PROJECT/.cursor/rules/personal.mdc"
OAW_CURSOR_SIBLING_BEFORE=$(cksum <"$OAW_PROJECT/.cursor/rules/personal.mdc")

run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "fresh project Cursor install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
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
  "$(render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE")" \
  >"$OAW_EXPECTED_CURSOR"
cmp -s "$OAW_EXPECTED_CURSOR" "$OAW_CURSOR" || fail "Cursor adapter bytes are invalid"
assert_lazy_router_file "$OAW_CURSOR" "$OAW_PROJECT_POLICY_REFERENCE" "Cursor adapter"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_WINDSURF=$OAW_PROJECT_PHYSICAL/.devin/rules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_WINDSURF=$OAW_SANDBOX/expected-windsurf.md

printf '%s\n' \
  '---' \
  'trigger: always_on' \
  '---' \
  '' \
  "$(render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE")" \
  >"$OAW_EXPECTED_WINDSURF"
cmp -s "$OAW_EXPECTED_WINDSURF" "$OAW_WINDSURF" || fail "Windsurf adapter bytes are invalid"
assert_lazy_router_file "$OAW_WINDSURF" "$OAW_PROJECT_POLICY_REFERENCE" "Windsurf adapter"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_CLINE=$OAW_PROJECT_PHYSICAL/.clinerules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_CLINE=$OAW_SANDBOX/expected-cline.md

render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE" >"$OAW_EXPECTED_CLINE"
cmp -s "$OAW_EXPECTED_CLINE" "$OAW_CLINE" || fail "Cline adapter bytes are invalid"
assert_lazy_router_file "$OAW_CLINE" "$OAW_PROJECT_POLICY_REFERENCE" "Cline adapter"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_ROO=$OAW_PROJECT_PHYSICAL/.roo/rules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_ROO=$OAW_SANDBOX/expected-roo.md

render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE" >"$OAW_EXPECTED_ROO"
cmp -s "$OAW_EXPECTED_ROO" "$OAW_ROO" || fail "Roo adapter bytes are invalid"
assert_lazy_router_file "$OAW_ROO" "$OAW_PROJECT_POLICY_REFERENCE" "Roo adapter"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_COPILOT=$OAW_PROJECT_PHYSICAL/.github/instructions/open-agent-workflow.instructions.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_COPILOT=$OAW_SANDBOX/expected-copilot.instructions.md

printf '%s\n' \
  '---' \
  'applyTo: "**"' \
  '---' \
  '' \
  "$(render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE")" \
  >"$OAW_EXPECTED_COPILOT"
cmp -s "$OAW_EXPECTED_COPILOT" "$OAW_COPILOT" || fail "Copilot adapter bytes are invalid"
assert_lazy_router_file "$OAW_COPILOT" "$OAW_PROJECT_POLICY_REFERENCE" "Copilot adapter"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_PROJECT_CLAUDE=$OAW_PROJECT_PHYSICAL/.claude/CLAUDE.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state

[ -f "$OAW_POLICY" ] || fail "project install did not create the canonical policy"
[ -f "$OAW_PROJECT_STATE" ] || fail "project install did not create identity-scoped state"
if grep -F "@$OAW_PROJECT_POLICY_REFERENCE" "$OAW_PROJECT_CLAUDE" >/dev/null; then
  fail "project Claude entrypoint incorrectly imports the canonical policy"
fi
assert_lazy_router_file "$OAW_PROJECT_CLAUDE" "$OAW_PROJECT_POLICY_REFERENCE" "project Claude instructions"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-project-agents-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-project-agents-block

write_expected_router_block "$OAW_EXPECTED_BLOCK" "$OAW_PROJECT_POLICY_REFERENCE"
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_PROJECT_AGENTS" >"$OAW_ACTUAL_BLOCK"
cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "project Codex does not use the shared AGENTS activation router"
assert_lazy_router_file "$OAW_PROJECT_AGENTS" "$OAW_PROJECT_POLICY_REFERENCE" "project Codex instructions"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_PROJECT_GEMINI=$OAW_PROJECT_PHYSICAL/GEMINI.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-project-gemini-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-project-gemini-block

write_expected_router_block "$OAW_EXPECTED_BLOCK" "$OAW_PROJECT_POLICY_REFERENCE"
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_PROJECT_GEMINI" >"$OAW_ACTUAL_BLOCK"
cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "project Gemini does not use the activation router"
if grep -Fx "@$OAW_PROJECT_POLICY_REFERENCE" "$OAW_PROJECT_GEMINI" >/dev/null; then
  fail "project Gemini instructions incorrectly use a standalone Markdown import"
fi
assert_lazy_router_file "$OAW_PROJECT_GEMINI" "$OAW_PROJECT_POLICY_REFERENCE" "project Gemini instructions"
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-project-opencode-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-project-opencode-block

write_expected_router_block "$OAW_EXPECTED_BLOCK" "$OAW_PROJECT_POLICY_REFERENCE"
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_PROJECT_AGENTS" >"$OAW_ACTUAL_BLOCK"
cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "project OpenCode does not use the shared AGENTS activation router"
assert_lazy_router_file "$OAW_PROJECT_AGENTS" "$OAW_PROJECT_POLICY_REFERENCE" "project OpenCode instructions"
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

for OAW_SHARED_ORDER in codex:opencode opencode:codex; do
  cleanup_sandbox
  setup_sandbox
  OAW_PROJECT="$OAW_SANDBOX/project with spaces"
  mkdir -p "$OAW_PROJECT"
  printf 'personal sequential shared instruction\n' >"$OAW_PROJECT/AGENTS.md"
  OAW_FIRST_SHARED=${OAW_SHARED_ORDER%%:*}
  OAW_SECOND_SHARED=${OAW_SHARED_ORDER#*:}
  run_oaw install --project "$OAW_PROJECT" --target "$OAW_FIRST_SHARED"
  assert_status 0 "sequential shared $OAW_FIRST_SHARED fixture install"
  run_oaw install --project "$OAW_PROJECT" --target "$OAW_SECOND_SHARED"
  assert_status 0 "sequential $OAW_SECOND_SHARED join"
  OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
  OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
  OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
  grep -Fx 'personal sequential shared instruction' "$OAW_PROJECT_AGENTS" >/dev/null ||
    fail "sequential shared install changed project instructions"
  [ "$(awk '$0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { count++ } END { print count + 0 }' \
    "$OAW_PROJECT_AGENTS")" -eq 1 ] || fail "sequential shared install duplicated the OAW block"
  [ "$(awk -F '\t' '$1 == "target" { count++ } END { print count + 0 }' "$OAW_PROJECT_STATE")" -eq 2 ] ||
    fail "sequential shared install did not retain both target rows"
done

pass "later project target joins an installed shared AGENTS destination"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"
printf 'personal shared project instruction\n' >"$OAW_PROJECT/AGENTS.md"

run_oaw install --project "$OAW_PROJECT" --target codex,opencode
assert_status 0 "shared project AGENTS install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-shared-agents-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-shared-agents-block

write_expected_router_block "$OAW_EXPECTED_BLOCK" "$OAW_PROJECT_POLICY_REFERENCE"
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { copying = 1 }
  copying { print }
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" && copying { exit }
' "$OAW_PROJECT_AGENTS" >"$OAW_ACTUAL_BLOCK"
cmp -s "$OAW_EXPECTED_BLOCK" "$OAW_ACTUAL_BLOCK" ||
  fail "shared project AGENTS block is not canonical"
assert_lazy_router_file "$OAW_PROJECT_AGENTS" "$OAW_PROJECT_POLICY_REFERENCE" "shared project instructions"
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
printf 'personal independently scoped project instruction\n' >"$OAW_PROJECT/AGENTS.md"

run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "independent project fixture install"
run_oaw install --target codex
assert_status 0 "independent user fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_PROJECT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state

run_oaw uninstall --target codex
assert_status 0 "independent user uninstall"
[ -f "$OAW_POLICY" ] || fail "user uninstall removed project-owned policy"
[ -f "$OAW_PROJECT_STATE" ] || fail "user uninstall removed project state"
[ -f "$OAW_PROJECT_AGENTS" ] || fail "user uninstall changed project adapter"
[ ! -e "$OAW_USER_STATE" ] || fail "user uninstall kept user state"

run_oaw uninstall --project "$OAW_PROJECT" --target codex
assert_status 0 "independent project uninstall"
[ ! -e "$OAW_POLICY" ] || fail "project uninstall kept project-owned policy"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "project uninstall kept project state"

pass "user and project policy ownership remain independent"

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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
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

for OAW_MATRIX_TARGET in claude codex gemini opencode cursor windsurf cline roo copilot; do
  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_PROJECT="$OAW_SANDBOX/project with spaces"
  OAW_MATRIX_RELATIVE=$(project_target_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_PATH=$OAW_PROJECT/$OAW_MATRIX_RELATIVE
  mkdir -p "$(dirname -- "$OAW_MATRIX_PATH")"
  case "$OAW_MATRIX_TARGET" in
    claude|codex|gemini|opencode)
      printf 'personal %s project instruction\n' "$OAW_MATRIX_TARGET" >"$OAW_MATRIX_PATH"
      OAW_MATRIX_SIBLING=
      ;;
    *)
      OAW_MATRIX_SIBLING=$(dirname -- "$OAW_MATRIX_PATH")/personal-$OAW_MATRIX_TARGET.md
      printf 'personal %s sibling\n' "$OAW_MATRIX_TARGET" >"$OAW_MATRIX_SIBLING"
      ;;
  esac
  OAW_MATRIX_SIBLING_BEFORE=
  [ -z "$OAW_MATRIX_SIBLING" ] || OAW_MATRIX_SIBLING_BEFORE=$(cksum <"$OAW_MATRIX_SIBLING")

  run_oaw install --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET project matrix install"
  OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
  OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
  OAW_MATRIX_PATH=$OAW_PROJECT_PHYSICAL/$OAW_MATRIX_RELATIVE
  OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
  OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-$OAW_MATRIX_TARGET
  cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
  printf '0.1.1-project-%s\n' "$OAW_MATRIX_TARGET" >"$OAW_UPDATE_CHECKOUT/VERSION"
  printf '\nTASK 4 MATRIX %s UPDATE SENTINEL\n' "$OAW_MATRIX_TARGET" \
    >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
  build_checkout_installer "$OAW_UPDATE_CHECKOUT"
  OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

  OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
  OAW_MATRIX_BEFORE=$(cksum <"$OAW_MATRIX_PATH")
  OAW_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
  run_oaw update --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET" --dry-run
  assert_status 0 "$OAW_MATRIX_TARGET project update dry run"
  [ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "$OAW_MATRIX_TARGET update dry run changed policy"
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] || fail "$OAW_MATRIX_TARGET update dry run changed target"
  [ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "$OAW_MATRIX_TARGET update dry run changed state"

  run_oaw update --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET project copied-checkout update"
  grep -F "TASK 4 MATRIX $OAW_MATRIX_TARGET UPDATE SENTINEL" "$OAW_POLICY" >/dev/null || fail "$OAW_MATRIX_TARGET update ignored checkout"
  grep -F "$(printf 'version\t0.1.1-project-%s' "$OAW_MATRIX_TARGET")" "$OAW_PROJECT_STATE" >/dev/null || fail "$OAW_MATRIX_TARGET update did not record version"
  [ -z "$OAW_MATRIX_SIBLING" ] || [ "$(cksum <"$OAW_MATRIX_SIBLING")" = "$OAW_MATRIX_SIBLING_BEFORE" ] || fail "$OAW_MATRIX_TARGET update changed sibling"
  assert_lazy_router_file "$OAW_MATRIX_PATH" "$OAW_PROJECT_POLICY_REFERENCE" "$OAW_MATRIX_TARGET updated project instructions"

  OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
  OAW_MATRIX_BEFORE=$(cksum <"$OAW_MATRIX_PATH")
  OAW_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
  run_oaw uninstall --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET" --dry-run
  assert_status 0 "$OAW_MATRIX_TARGET project uninstall dry run"
  [ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall dry run changed policy"
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall dry run changed target"
  [ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall dry run changed state"

  run_oaw uninstall --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET project final uninstall"
  case "$OAW_MATRIX_TARGET" in
    claude|codex|gemini|opencode) grep -Fx "personal $OAW_MATRIX_TARGET project instruction" "$OAW_MATRIX_PATH" >/dev/null || fail "$OAW_MATRIX_TARGET uninstall changed project content" ;;
    *) [ ! -e "$OAW_MATRIX_PATH" ] || fail "$OAW_MATRIX_TARGET uninstall kept owned target" ;;
  esac
  [ -z "$OAW_MATRIX_SIBLING" ] || [ "$(cksum <"$OAW_MATRIX_SIBLING")" = "$OAW_MATRIX_SIBLING_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall changed sibling"
done
pass "all nine project targets complete copied-update and dry-run lifecycle"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target opencode
assert_status 0 "partial shared project install before default set"
run_oaw install --project "$OAW_PROJECT"
assert_status 0 "default nine-target project install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
[ "$(awk -F '\t' '$1 == "target" { count++ } END { print count + 0 }' "$OAW_PROJECT_STATE")" -eq 9 ] || fail "default project install did not record nine targets"
OAW_DEFAULT_TARGETS=$(awk -F '\t' '$1 == "target" { if (targets == "") targets = $2; else targets = targets "," $2 } END { print targets }' "$OAW_PROJECT_STATE")
[ "$OAW_DEFAULT_TARGETS" = "claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot" ] || fail "default project target order is not deterministic"
run_oaw install --project "$OAW_PROJECT"
assert_status 0 "repeated default nine-target project install"
assert_contains "unchanged: copilot" "repeated default project install reports unchanged targets"
OAW_DEFAULT_AGENTS=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_DEFAULT_CURSOR=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-default
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.1-project-default\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nTASK 4 DEFAULT UPDATE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
build_checkout_installer "$OAW_UPDATE_CHECKOUT"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
OAW_AGENTS_BEFORE=$(cksum <"$OAW_DEFAULT_AGENTS")
OAW_CURSOR_BEFORE=$(cksum <"$OAW_DEFAULT_CURSOR")
run_oaw update --project "$OAW_PROJECT" --dry-run
assert_status 0 "default project update dry run"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "default update dry run changed policy"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "default update dry run changed state"
[ "$(cksum <"$OAW_DEFAULT_AGENTS")" = "$OAW_AGENTS_BEFORE" ] || fail "default update dry run changed AGENTS"
[ "$(cksum <"$OAW_DEFAULT_CURSOR")" = "$OAW_CURSOR_BEFORE" ] || fail "default update dry run changed Cursor"
run_oaw update --project "$OAW_PROJECT"
assert_status 0 "default project copied-checkout update"
grep -F 'TASK 4 DEFAULT UPDATE SENTINEL' "$OAW_POLICY" >/dev/null || fail "default update ignored checkout"
grep -F "$(printf 'version\t0.1.1-project-default')" "$OAW_PROJECT_STATE" >/dev/null || fail "default update did not record version"
for OAW_MATRIX_TARGET in claude codex gemini opencode cursor windsurf cline roo copilot; do
  OAW_MATRIX_RELATIVE=$(project_target_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_PATH=$OAW_PROJECT_PHYSICAL/$OAW_MATRIX_RELATIVE
  assert_lazy_router_file "$OAW_MATRIX_PATH" "$OAW_PROJECT_POLICY_REFERENCE" "$OAW_MATRIX_TARGET default updated project instructions"
done
run_oaw uninstall --project "$OAW_PROJECT" --target cursor
assert_status 0 "default project selected uninstall"
[ ! -e "$OAW_DEFAULT_CURSOR" ] || fail "selected uninstall kept Cursor"
[ -f "$OAW_POLICY" ] || fail "selected uninstall removed referenced policy"
[ "$(awk -F '\t' '$1 == "target" { count++ } END { print count + 0 }' "$OAW_PROJECT_STATE")" -eq 8 ] || fail "selected uninstall did not retain eight targets"
run_oaw uninstall --project "$OAW_PROJECT"
assert_status 0 "default project final uninstall"
[ ! -e "$OAW_POLICY" ] || fail "default final uninstall kept policy"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "default final uninstall kept state"
pass "default project set completes update and selected uninstall lifecycle"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT/.cursor/rules"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_CURSOR=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check reports clean adapter"
assert_contains "installed cursor: clean" "project check reports clean state"
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
printf 'drifted Cursor check\n' >"$OAW_CURSOR"
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check reports drift"
assert_contains "installed cursor: drift" "project check reports owned-file drift"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "project check changed policy"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "project check changed state"
OAW_OTHER_PROJECT="$OAW_SANDBOX/other project"
mkdir -p "$OAW_OTHER_PROJECT"
OAW_TAMPERED_STATE=$OAW_SANDBOX/check-tampered.state
OAW_OTHER_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_OTHER_PROJECT" && pwd -P)
awk -F '\t' -v project_root="$OAW_OTHER_PHYSICAL" 'BEGIN { OFS = "\t" } $1 == "project" { $2 = project_root } { print }' \
  "$OAW_PROJECT_STATE" >"$OAW_TAMPERED_STATE"
mv "$OAW_TAMPERED_STATE" "$OAW_PROJECT_STATE"
OAW_TAMPERED_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check reports invalid root state"
assert_contains "installed cursor: invalid-state" "project check rejects stored root mismatch"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_TAMPERED_STATE_BEFORE" ] || fail "project check rewrote invalid state"
pass "project check reports clean, drift, and invalid-state without writes"
