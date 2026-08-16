#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

OAW_PROJECT_POLICY_REFERENCE=.oaw/policy/POLICY.md

render_expected_activation_router() {
  printf 'Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, read `%s` as the Project Policy Set and do not read or merge the User Policy Set. Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit ends OAW governance for that deliverable.\n' "$1"
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
  grep -F "On explicit activation, read \`$router_policy\` as the Project Policy Set" "$router_file" >/dev/null ||
    fail "$router_description does not retain the Project Policy Set path"
  grep -F 'do not read or merge the User Policy Set' "$router_file" >/dev/null ||
    fail "$router_description does not preserve whole-set project precedence"
  grep -F 'ordinary Skill invocation do not activate OAW' "$router_file" >/dev/null ||
    fail "$router_description incorrectly governs normal Skill routing"
  if grep -F "@$router_policy" "$router_file" >/dev/null ||
    grep -F 'For every new top-level engineering request, first read' "$router_file" >/dev/null ||
    grep -F 'Before engineering lifecycle work, read' "$router_file" >/dev/null ||
    grep -F 'classify it as DIRECT, BOUNDED, or WORKFLOW' "$router_file" >/dev/null; then
    fail "$router_description retains eager OAW activation"
  fi
}

assert_state_artifacts() {
  state_file=$1
  expected_artifacts=$2
  description=$3
  grep -Fx "$(printf 'format\t2')" "$state_file" >/dev/null ||
    fail "$description (state format is not 2)"
  actual_artifacts=$(awk -F '\t' '
    $1 == "target" {
      if (artifacts == "") {
        artifacts = $2 "/" $3
      } else {
        artifacts = artifacts "," $2 "/" $3
      }
    }
    END { print artifacts }
  ' "$state_file")
  if [ "$actual_artifacts" != "$expected_artifacts" ]; then
    fail "$description (expected $expected_artifacts, got $actual_artifacts)"
  fi
}

assert_state_artifact() {
  state_file=$1
  target_id=$2
  artifact_id=$3
  expected_path=$4
  expected_mode=$5
  expected_origin=$6
  description=$7
  awk -F '\t' \
    -v target_id="$target_id" \
    -v artifact_id="$artifact_id" \
    -v expected_path="$expected_path" \
    -v expected_mode="$expected_mode" \
    -v expected_origin="$expected_origin" '
      $1 == "target" && $2 == target_id && $3 == artifact_id &&
        $4 == expected_path && $5 == expected_mode && length($6) != 0 &&
        $7 == expected_origin && NF == 7 { found++ }
      END { exit(found == 1 ? 0 : 1) }
    ' "$state_file" || fail "$description"
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

project_native_path_for_test() {
  case "$1" in
    claude) printf '.claude/skills/oaw/SKILL.md\n' ;;
    codex) printf '.agents/skills/oaw/SKILL.md\n' ;;
    gemini) printf '.gemini/commands/oaw.toml\n' ;;
    opencode) printf '.opencode/commands/oaw.md\n' ;;
    cursor) printf '.cursor/skills/oaw/SKILL.md\n' ;;
    windsurf) printf '.windsurf/workflows/oaw.md\n' ;;
    cline) printf '.cline/skills/oaw/SKILL.md\n' ;;
    roo) printf '.roo/commands/oaw.md\n' ;;
    copilot) printf '.github/skills/oaw/SKILL.md\n' ;;
    *) fail "unknown project native target: $1" ;;
  esac
}

project_native_policy_path_for_test() {
  if [ "$1" = codex ]; then
    printf '.agents/skills/oaw/agents/openai.yaml\n'
  fi
}

assert_native_entrypoint() {
  entrypoint=$1
  target_id=$2
  [ -f "$entrypoint" ] || fail "$target_id project native entrypoint is missing"
  grep -F 'current top-level user request' "$entrypoint" >/dev/null ||
    fail "$target_id project native entrypoint does not inspect top-level user intent"
  grep -F 'reliable Host metadata that the user, not the model, selected this entrypoint' "$entrypoint" >/dev/null ||
    fail "$target_id project native entrypoint does not distinguish user and model selection"
  grep -F 'Invocation or loading of this entrypoint alone is not evidence of user intent' "$entrypoint" >/dev/null ||
    fail "$target_id project native entrypoint treats physical invocation as user intent"
  grep -F 'do not activate OAW and continue as the native Host' "$entrypoint" >/dev/null ||
    fail "$target_id project native entrypoint lacks its automatic-load no-op"
  grep -F 'Follow the current OAW Activation Router to select and read one Policy Set' "$entrypoint" >/dev/null ||
    fail "$target_id project native entrypoint bypasses the Activation Router"
  if grep -F '.oaw/policy/POLICY.md' "$entrypoint" >/dev/null; then
    fail "$target_id project native entrypoint embeds a Policy path"
  fi
  grep -F 'Pass the optional Profile and task' "$entrypoint" >/dev/null ||
    fail "$target_id project native entrypoint does not pass invocation input through"
  for forbidden_profile in MATT-FULL MATT-SP-HYBRID SP-FULL ECC-FULL; do
    if grep -F "$forbidden_profile" "$entrypoint" >/dev/null; then
      fail "$target_id project native entrypoint hard-codes Profile $forbidden_profile"
    fi
  done
}

assert_project_host_artifacts() {
  state_file=$1
  project_root=$2
  target_id=$3
  router_origin=$4
  router_path=$project_root/$(project_target_path_for_test "$target_id")
  native_path=$project_root/$(project_native_path_for_test "$target_id")

  case "$target_id" in
    claude|codex|gemini|opencode) router_mode=managed-block ;;
    *) router_mode=owned-file ;;
  esac
  assert_state_artifact "$state_file" "$target_id" router "$router_path" \
    "$router_mode" "$router_origin" "$target_id Router state row is missing or invalid"
  assert_state_artifact "$state_file" "$target_id" native-entrypoint "$native_path" \
    owned-file created-file "$target_id native-entrypoint state row is missing or invalid"
  assert_native_entrypoint "$native_path" "$target_id"

  native_policy_relative=$(project_native_policy_path_for_test "$target_id")
  if [ -n "$native_policy_relative" ]; then
    native_policy=$project_root/$native_policy_relative
    assert_state_artifact "$state_file" "$target_id" native-policy "$native_policy" \
      owned-file created-file "$target_id native-policy state row is missing or invalid"
    [ -f "$native_policy" ] || fail "$target_id native policy metadata is missing"
    grep -F 'allow_implicit_invocation: false' "$native_policy" >/dev/null ||
      fail "$target_id native policy permits implicit invocation"
  fi
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
OAW_CURSOR_NATIVE=$OAW_PROJECT_PHYSICAL/.cursor/skills/oaw/SKILL.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_CURSOR=$OAW_SANDBOX/expected-cursor.mdc

printf '%s\n' \
  '---' \
  'description: Open Agent Workflow activation router' \
  'globs: "**/*"' \
  'alwaysApply: true' \
  '---' \
  '' \
  "$(render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE")" \
  >"$OAW_EXPECTED_CURSOR"
cmp -s "$OAW_EXPECTED_CURSOR" "$OAW_CURSOR" || fail "Cursor adapter bytes are invalid"
assert_lazy_router_file "$OAW_CURSOR" "$OAW_PROJECT_POLICY_REFERENCE" "Cursor adapter"
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "cursor/router,cursor/native-entrypoint" \
  "project Cursor state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" cursor created-file
[ "$(cksum <"$OAW_PROJECT/.cursor/rules/personal.mdc")" = "$OAW_CURSOR_SIBLING_BEFORE" ] ||
  fail "Cursor install changed a sibling rule"

OAW_CURSOR_BEFORE=$(cksum <"$OAW_CURSOR")
OAW_CURSOR_NATIVE_BEFORE=$(cksum <"$OAW_CURSOR_NATIVE")
OAW_CURSOR_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "repeated project Cursor install"
for OAW_CURSOR_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: cursor/$OAW_CURSOR_ARTIFACT" \
    "repeated project install does not report Cursor $OAW_CURSOR_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_CURSOR")" = "$OAW_CURSOR_BEFORE" ] ||
  fail "repeated Cursor install changed adapter bytes"
[ "$(cksum <"$OAW_CURSOR_NATIVE")" = "$OAW_CURSOR_NATIVE_BEFORE" ] ||
  fail "repeated Cursor install changed native entrypoint bytes"
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
assert_contains "owned target artifact already exists" "pre-existing owned destination is explicit"
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
OAW_WINDSURF_NATIVE=$OAW_PROJECT_PHYSICAL/.windsurf/workflows/oaw.md
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
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "windsurf/router,windsurf/native-entrypoint" \
  "project Windsurf state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" windsurf created-file
[ "$(cksum <"$OAW_PROJECT/.devin/rules/personal.md")" = "$OAW_WINDSURF_SIBLING_BEFORE" ] ||
  fail "Windsurf install changed a sibling rule"

OAW_WINDSURF_BEFORE=$(cksum <"$OAW_WINDSURF")
OAW_WINDSURF_NATIVE_BEFORE=$(cksum <"$OAW_WINDSURF_NATIVE")
OAW_WINDSURF_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target windsurf
assert_status 0 "repeated project Windsurf install"
for OAW_WINDSURF_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: windsurf/$OAW_WINDSURF_ARTIFACT" \
    "repeated project install does not report Windsurf $OAW_WINDSURF_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_WINDSURF")" = "$OAW_WINDSURF_BEFORE" ] ||
  fail "repeated Windsurf install changed adapter bytes"
[ "$(cksum <"$OAW_WINDSURF_NATIVE")" = "$OAW_WINDSURF_NATIVE_BEFORE" ] ||
  fail "repeated Windsurf install changed native entrypoint bytes"
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
OAW_CLINE_NATIVE=$OAW_PROJECT_PHYSICAL/.cline/skills/oaw/SKILL.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_CLINE=$OAW_SANDBOX/expected-cline.md

render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE" >"$OAW_EXPECTED_CLINE"
cmp -s "$OAW_EXPECTED_CLINE" "$OAW_CLINE" || fail "Cline adapter bytes are invalid"
assert_lazy_router_file "$OAW_CLINE" "$OAW_PROJECT_POLICY_REFERENCE" "Cline adapter"
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "cline/router,cline/native-entrypoint" \
  "project Cline state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" cline created-file
[ "$(cksum <"$OAW_PROJECT/.clinerules/personal.md")" = "$OAW_CLINE_SIBLING_BEFORE" ] ||
  fail "Cline install changed a sibling rule"

OAW_CLINE_BEFORE=$(cksum <"$OAW_CLINE")
OAW_CLINE_NATIVE_BEFORE=$(cksum <"$OAW_CLINE_NATIVE")
OAW_CLINE_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target cline
assert_status 0 "repeated project Cline install"
for OAW_CLINE_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: cline/$OAW_CLINE_ARTIFACT" \
    "repeated project install does not report Cline $OAW_CLINE_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_CLINE")" = "$OAW_CLINE_BEFORE" ] ||
  fail "repeated Cline install changed adapter bytes"
[ "$(cksum <"$OAW_CLINE_NATIVE")" = "$OAW_CLINE_NATIVE_BEFORE" ] ||
  fail "repeated Cline install changed native entrypoint bytes"
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
OAW_ROO_NATIVE=$OAW_PROJECT_PHYSICAL/.roo/commands/oaw.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_EXPECTED_ROO=$OAW_SANDBOX/expected-roo.md

render_expected_activation_router "$OAW_PROJECT_POLICY_REFERENCE" >"$OAW_EXPECTED_ROO"
cmp -s "$OAW_EXPECTED_ROO" "$OAW_ROO" || fail "Roo adapter bytes are invalid"
assert_lazy_router_file "$OAW_ROO" "$OAW_PROJECT_POLICY_REFERENCE" "Roo adapter"
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "roo/router,roo/native-entrypoint" \
  "project Roo state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" roo created-file
[ "$(cksum <"$OAW_PROJECT/.roo/rules/personal.md")" = "$OAW_ROO_SIBLING_BEFORE" ] ||
  fail "Roo install changed a sibling rule"

OAW_ROO_BEFORE=$(cksum <"$OAW_ROO")
OAW_ROO_NATIVE_BEFORE=$(cksum <"$OAW_ROO_NATIVE")
OAW_ROO_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target roo
assert_status 0 "repeated project Roo install"
for OAW_ROO_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: roo/$OAW_ROO_ARTIFACT" \
    "repeated project install does not report Roo $OAW_ROO_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_ROO")" = "$OAW_ROO_BEFORE" ] ||
  fail "repeated Roo install changed adapter bytes"
[ "$(cksum <"$OAW_ROO_NATIVE")" = "$OAW_ROO_NATIVE_BEFORE" ] ||
  fail "repeated Roo install changed native entrypoint bytes"
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
OAW_COPILOT_NATIVE=$OAW_PROJECT_PHYSICAL/.github/skills/oaw/SKILL.md
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
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "copilot/router,copilot/native-entrypoint" \
  "project Copilot state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" copilot created-file
[ "$(cksum <"$OAW_PROJECT/.github/instructions/personal.instructions.md")" = \
  "$OAW_COPILOT_SIBLING_BEFORE" ] || fail "Copilot install changed a sibling instruction"

OAW_COPILOT_BEFORE=$(cksum <"$OAW_COPILOT")
OAW_COPILOT_NATIVE_BEFORE=$(cksum <"$OAW_COPILOT_NATIVE")
OAW_COPILOT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target copilot
assert_status 0 "repeated project Copilot install"
for OAW_COPILOT_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: copilot/$OAW_COPILOT_ARTIFACT" \
    "repeated project install does not report Copilot $OAW_COPILOT_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_COPILOT")" = "$OAW_COPILOT_BEFORE" ] ||
  fail "repeated Copilot install changed adapter bytes"
[ "$(cksum <"$OAW_COPILOT_NATIVE")" = "$OAW_COPILOT_NATIVE_BEFORE" ] ||
  fail "repeated Copilot install changed native entrypoint bytes"
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
OAW_PROJECT_CLAUDE_NATIVE=$OAW_PROJECT_PHYSICAL/.claude/skills/oaw/SKILL.md
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
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "claude/router,claude/native-entrypoint" \
  "project Claude state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" claude existing-file
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
OAW_PROJECT_CLAUDE_NATIVE_BEFORE=$(cksum <"$OAW_PROJECT_CLAUDE_NATIVE")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target claude
assert_status 0 "repeated project Claude install"
for OAW_CLAUDE_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: claude/$OAW_CLAUDE_ARTIFACT" \
    "repeated project install does not report Claude $OAW_CLAUDE_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated project install changed the canonical policy"
[ "$(cksum <"$OAW_PROJECT_CLAUDE")" = "$OAW_PROJECT_CLAUDE_BEFORE" ] ||
  fail "repeated project install changed Claude instructions"
[ "$(cksum <"$OAW_PROJECT_CLAUDE_NATIVE")" = "$OAW_PROJECT_CLAUDE_NATIVE_BEFORE" ] ||
  fail "repeated project install changed Claude native entrypoint"
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
OAW_PROJECT_CODEX_NATIVE=$OAW_PROJECT_PHYSICAL/.agents/skills/oaw/SKILL.md
OAW_PROJECT_CODEX_NATIVE_POLICY=$OAW_PROJECT_PHYSICAL/.agents/skills/oaw/agents/openai.yaml
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
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "codex/router,codex/native-entrypoint,codex/native-policy" \
  "project Codex state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" codex existing-file
[ "$(cksum <"$OAW_HOME/.codex/AGENTS.md")" = "$OAW_USER_CODEX_BEFORE" ] ||
  fail "project Codex install changed user Codex instructions"
[ "$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")" = "$OAW_USER_OPENCODE_BEFORE" ] ||
  fail "project Codex install changed user OpenCode instructions"

OAW_PROJECT_AGENTS_BEFORE=$(cksum <"$OAW_PROJECT_AGENTS")
OAW_PROJECT_CODEX_NATIVE_BEFORE=$(cksum <"$OAW_PROJECT_CODEX_NATIVE")
OAW_PROJECT_CODEX_NATIVE_POLICY_BEFORE=$(cksum <"$OAW_PROJECT_CODEX_NATIVE_POLICY")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "repeated project Codex install"
for OAW_CODEX_ARTIFACT in router native-entrypoint native-policy; do
  assert_contains "unchanged: codex/$OAW_CODEX_ARTIFACT" \
    "repeated project install does not report Codex $OAW_CODEX_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_PROJECT_AGENTS")" = "$OAW_PROJECT_AGENTS_BEFORE" ] ||
  fail "repeated project Codex install changed AGENTS instructions"
[ "$(cksum <"$OAW_PROJECT_CODEX_NATIVE")" = "$OAW_PROJECT_CODEX_NATIVE_BEFORE" ] ||
  fail "repeated project Codex install changed native entrypoint"
[ "$(cksum <"$OAW_PROJECT_CODEX_NATIVE_POLICY")" = "$OAW_PROJECT_CODEX_NATIVE_POLICY_BEFORE" ] ||
  fail "repeated project Codex install changed native policy metadata"
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
OAW_PROJECT_GEMINI_NATIVE=$OAW_PROJECT_PHYSICAL/.gemini/commands/oaw.toml
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
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "gemini/router,gemini/native-entrypoint" \
  "project Gemini state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" gemini existing-file
[ "$(cksum <"$OAW_HOME/.gemini/GEMINI.md")" = "$OAW_USER_GEMINI_BEFORE" ] ||
  fail "project Gemini install changed user Gemini instructions"

OAW_PROJECT_GEMINI_BEFORE=$(cksum <"$OAW_PROJECT_GEMINI")
OAW_PROJECT_GEMINI_NATIVE_BEFORE=$(cksum <"$OAW_PROJECT_GEMINI_NATIVE")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target gemini
assert_status 0 "repeated project Gemini install"
for OAW_GEMINI_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: gemini/$OAW_GEMINI_ARTIFACT" \
    "repeated project install does not report Gemini $OAW_GEMINI_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_PROJECT_GEMINI")" = "$OAW_PROJECT_GEMINI_BEFORE" ] ||
  fail "repeated project Gemini install changed GEMINI instructions"
[ "$(cksum <"$OAW_PROJECT_GEMINI_NATIVE")" = "$OAW_PROJECT_GEMINI_NATIVE_BEFORE" ] ||
  fail "repeated project Gemini install changed native entrypoint"
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
OAW_PROJECT_OPENCODE_NATIVE=$OAW_PROJECT_PHYSICAL/.opencode/commands/oaw.md
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
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "opencode/router,opencode/native-entrypoint" \
  "project OpenCode state does not contain its complete artifact set"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" opencode existing-file
[ "$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")" = "$OAW_USER_OPENCODE_BEFORE" ] ||
  fail "project OpenCode install changed user OpenCode instructions"

OAW_PROJECT_AGENTS_BEFORE=$(cksum <"$OAW_PROJECT_AGENTS")
OAW_PROJECT_OPENCODE_NATIVE_BEFORE=$(cksum <"$OAW_PROJECT_OPENCODE_NATIVE")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target opencode
assert_status 0 "repeated project OpenCode install"
for OAW_OPENCODE_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: opencode/$OAW_OPENCODE_ARTIFACT" \
    "repeated project install does not report OpenCode $OAW_OPENCODE_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_PROJECT_AGENTS")" = "$OAW_PROJECT_AGENTS_BEFORE" ] ||
  fail "repeated project OpenCode install changed AGENTS instructions"
[ "$(cksum <"$OAW_PROJECT_OPENCODE_NATIVE")" = "$OAW_PROJECT_OPENCODE_NATIVE_BEFORE" ] ||
  fail "repeated project OpenCode install changed native entrypoint"
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
  assert_state_artifacts "$OAW_PROJECT_STATE" \
    "codex/router,codex/native-entrypoint,codex/native-policy,opencode/router,opencode/native-entrypoint" \
    "sequential shared install did not retain both Hosts' artifacts"
  assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" codex existing-file
  assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" opencode existing-file
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
OAW_CODEX_CHECKSUM=$(awk -F '\t' '$1 == "target" && $2 == "codex" && $3 == "router" { print $6 }' "$OAW_PROJECT_STATE")
OAW_OPENCODE_CHECKSUM=$(awk -F '\t' '$1 == "target" && $2 == "opencode" && $3 == "router" { print $6 }' "$OAW_PROJECT_STATE")
[ -n "$OAW_CODEX_CHECKSUM" ] && [ "$OAW_CODEX_CHECKSUM" = "$OAW_OPENCODE_CHECKSUM" ] ||
  fail "shared project AGENTS targets do not share one checksum"
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "codex/router,codex/native-entrypoint,codex/native-policy,opencode/router,opencode/native-entrypoint" \
  "shared project state does not retain both Hosts' artifacts"
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" codex existing-file
assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" opencode existing-file

OAW_PROJECT_AGENTS_BEFORE=$(cksum <"$OAW_PROJECT_AGENTS")
OAW_CODEX_NATIVE=$OAW_PROJECT_PHYSICAL/.agents/skills/oaw/SKILL.md
OAW_CODEX_NATIVE_POLICY=$OAW_PROJECT_PHYSICAL/.agents/skills/oaw/agents/openai.yaml
OAW_OPENCODE_NATIVE=$OAW_PROJECT_PHYSICAL/.opencode/commands/oaw.md
OAW_OPENCODE_NATIVE_BEFORE=$(cksum <"$OAW_OPENCODE_NATIVE")
run_oaw uninstall --project "$OAW_PROJECT" --target codex
assert_status 0 "selected shared AGENTS uninstall"
[ "$(cksum <"$OAW_PROJECT_AGENTS")" = "$OAW_PROJECT_AGENTS_BEFORE" ] ||
  fail "selected shared AGENTS uninstall changed the block"
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "opencode/router,opencode/native-entrypoint" \
  "selected shared AGENTS uninstall removed the wrong artifact rows"
[ ! -e "$OAW_CODEX_NATIVE" ] || fail "selected shared uninstall kept Codex native entrypoint"
[ ! -e "$OAW_CODEX_NATIVE_POLICY" ] || fail "selected shared uninstall kept Codex native policy"
[ "$(cksum <"$OAW_OPENCODE_NATIVE")" = "$OAW_OPENCODE_NATIVE_BEFORE" ] ||
  fail "selected shared uninstall changed OpenCode native entrypoint"

run_oaw uninstall --project "$OAW_PROJECT" --target opencode
assert_status 0 "final shared AGENTS uninstall"
grep -Fx 'personal shared project instruction' "$OAW_PROJECT_AGENTS" >/dev/null ||
  fail "final shared AGENTS uninstall removed project instructions"
if grep -F '<!-- BEGIN OPEN AGENT WORKFLOW -->' "$OAW_PROJECT_AGENTS" >/dev/null 2>&1; then
  fail "final shared AGENTS uninstall left the OAW block"
fi
[ ! -e "$OAW_OPENCODE_NATIVE" ] || fail "final shared uninstall kept OpenCode native entrypoint"
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
OAW_PROJECT_CODEX_NATIVE=$OAW_PROJECT_PHYSICAL/.agents/skills/oaw/SKILL.md
OAW_PROJECT_CODEX_NATIVE_POLICY=$OAW_PROJECT_PHYSICAL/.agents/skills/oaw/agents/openai.yaml
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_USER_CODEX_NATIVE=$OAW_HOME/.agents/skills/oaw/SKILL.md
OAW_USER_CODEX_NATIVE_POLICY=$OAW_HOME/.agents/skills/oaw/agents/openai.yaml
OAW_PROJECT_CODEX_NATIVE_BEFORE=$(cksum <"$OAW_PROJECT_CODEX_NATIVE")
OAW_PROJECT_CODEX_NATIVE_POLICY_BEFORE=$(cksum <"$OAW_PROJECT_CODEX_NATIVE_POLICY")

run_oaw uninstall --target codex
assert_status 0 "independent user uninstall"
[ -f "$OAW_POLICY" ] || fail "user uninstall removed project-owned policy"
[ -f "$OAW_PROJECT_STATE" ] || fail "user uninstall removed project state"
[ -f "$OAW_PROJECT_AGENTS" ] || fail "user uninstall changed project adapter"
[ "$(cksum <"$OAW_PROJECT_CODEX_NATIVE")" = "$OAW_PROJECT_CODEX_NATIVE_BEFORE" ] ||
  fail "user uninstall changed project Codex native entrypoint"
[ "$(cksum <"$OAW_PROJECT_CODEX_NATIVE_POLICY")" = "$OAW_PROJECT_CODEX_NATIVE_POLICY_BEFORE" ] ||
  fail "user uninstall changed project Codex native policy"
[ ! -e "$OAW_USER_CODEX_NATIVE" ] || fail "user uninstall kept user Codex native entrypoint"
[ ! -e "$OAW_USER_CODEX_NATIVE_POLICY" ] || fail "user uninstall kept user Codex native policy"
[ ! -e "$OAW_USER_STATE" ] || fail "user uninstall kept user state"

run_oaw uninstall --project "$OAW_PROJECT" --target codex
assert_status 0 "independent project uninstall"
[ ! -e "$OAW_POLICY" ] || fail "project uninstall kept project-owned policy"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "project uninstall kept project state"
[ ! -e "$OAW_PROJECT_CODEX_NATIVE" ] || fail "project uninstall kept Codex native entrypoint"
[ ! -e "$OAW_PROJECT_CODEX_NATIVE_POLICY" ] || fail "project uninstall kept Codex native policy"

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
OAW_PROJECT_CLAUDE_NATIVE=$OAW_PROJECT_PHYSICAL/.claude/skills/oaw/SKILL.md
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
OAW_PROJECT_CLAUDE_NATIVE_BEFORE=$(cksum <"$OAW_PROJECT_CLAUDE_NATIVE")
OAW_PROJECT_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw install --project "$OAW_PROJECT" --target claude
assert_status 65 "stored project root mismatch"
assert_contains "installed project root does not match" "stored project root mismatch is explicit"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "project root mismatch changed the canonical policy"
[ "$(cksum <"$OAW_PROJECT_CLAUDE")" = "$OAW_PROJECT_CLAUDE_BEFORE" ] ||
  fail "project root mismatch changed project instructions"
[ "$(cksum <"$OAW_PROJECT_CLAUDE_NATIVE")" = "$OAW_PROJECT_CLAUDE_NATIVE_BEFORE" ] ||
  fail "project root mismatch changed Claude native entrypoint"
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
  OAW_MATRIX_NATIVE_RELATIVE=$(project_native_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_NATIVE_POLICY_RELATIVE=$(project_native_policy_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_PATH=$OAW_PROJECT/$OAW_MATRIX_RELATIVE
  OAW_MATRIX_NATIVE=$OAW_PROJECT/$OAW_MATRIX_NATIVE_RELATIVE
  mkdir -p "$(dirname -- "$OAW_MATRIX_PATH")"
  case "$OAW_MATRIX_TARGET" in
    claude|codex|gemini|opencode)
      printf 'personal %s project instruction\n' "$OAW_MATRIX_TARGET" >"$OAW_MATRIX_PATH"
      OAW_MATRIX_SIBLING=
      OAW_MATRIX_ROUTER_ORIGIN=existing-file
      ;;
    *)
      OAW_MATRIX_SIBLING=$(dirname -- "$OAW_MATRIX_PATH")/personal-$OAW_MATRIX_TARGET.md
      printf 'personal %s sibling\n' "$OAW_MATRIX_TARGET" >"$OAW_MATRIX_SIBLING"
      OAW_MATRIX_ROUTER_ORIGIN=created-file
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
  OAW_MATRIX_NATIVE=$OAW_PROJECT_PHYSICAL/$OAW_MATRIX_NATIVE_RELATIVE
  OAW_MATRIX_NATIVE_POLICY=
  [ -z "$OAW_MATRIX_NATIVE_POLICY_RELATIVE" ] ||
    OAW_MATRIX_NATIVE_POLICY=$OAW_PROJECT_PHYSICAL/$OAW_MATRIX_NATIVE_POLICY_RELATIVE
  OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
  OAW_EXPECTED_ARTIFACTS="$OAW_MATRIX_TARGET/router,$OAW_MATRIX_TARGET/native-entrypoint"
  [ "$OAW_MATRIX_TARGET" != codex ] ||
    OAW_EXPECTED_ARTIFACTS="$OAW_EXPECTED_ARTIFACTS,codex/native-policy"
  assert_state_artifacts "$OAW_PROJECT_STATE" "$OAW_EXPECTED_ARTIFACTS" \
    "$OAW_MATRIX_TARGET project matrix state does not record every artifact"
  assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" \
    "$OAW_MATRIX_TARGET" "$OAW_MATRIX_ROUTER_ORIGIN"
  OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-$OAW_MATRIX_TARGET
  cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
  printf '0.1.1-project-%s\n' "$OAW_MATRIX_TARGET" >"$OAW_UPDATE_CHECKOUT/VERSION"
  printf '\nTASK 4 MATRIX %s UPDATE SENTINEL\n' "$OAW_MATRIX_TARGET" \
    >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
  build_checkout_installer "$OAW_UPDATE_CHECKOUT"
  OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

  OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
  OAW_MATRIX_BEFORE=$(cksum <"$OAW_MATRIX_PATH")
  OAW_MATRIX_NATIVE_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE")
  OAW_MATRIX_NATIVE_POLICY_BEFORE=
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    OAW_MATRIX_NATIVE_POLICY_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE_POLICY")
  OAW_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
  run_oaw update --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET" --dry-run
  assert_status 0 "$OAW_MATRIX_TARGET project update dry run"
  [ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "$OAW_MATRIX_TARGET update dry run changed policy"
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] || fail "$OAW_MATRIX_TARGET update dry run changed target"
  [ "$(cksum <"$OAW_MATRIX_NATIVE")" = "$OAW_MATRIX_NATIVE_BEFORE" ] || fail "$OAW_MATRIX_TARGET update dry run changed native entrypoint"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    [ "$(cksum <"$OAW_MATRIX_NATIVE_POLICY")" = "$OAW_MATRIX_NATIVE_POLICY_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET update dry run changed native policy metadata"
  [ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "$OAW_MATRIX_TARGET update dry run changed state"

  run_oaw update --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET project copied-checkout update"
  grep -F "TASK 4 MATRIX $OAW_MATRIX_TARGET UPDATE SENTINEL" "$OAW_POLICY" >/dev/null || fail "$OAW_MATRIX_TARGET update ignored checkout"
  grep -F "$(printf 'version\t0.1.1-project-%s' "$OAW_MATRIX_TARGET")" "$OAW_PROJECT_STATE" >/dev/null || fail "$OAW_MATRIX_TARGET update did not record version"
  [ -z "$OAW_MATRIX_SIBLING" ] || [ "$(cksum <"$OAW_MATRIX_SIBLING")" = "$OAW_MATRIX_SIBLING_BEFORE" ] || fail "$OAW_MATRIX_TARGET update changed sibling"
  assert_lazy_router_file "$OAW_MATRIX_PATH" "$OAW_PROJECT_POLICY_REFERENCE" "$OAW_MATRIX_TARGET updated project instructions"
  assert_native_entrypoint "$OAW_MATRIX_NATIVE" "$OAW_MATRIX_TARGET"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    grep -F 'allow_implicit_invocation: false' "$OAW_MATRIX_NATIVE_POLICY" >/dev/null ||
    fail "$OAW_MATRIX_TARGET update lost native policy metadata"

  OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
  OAW_MATRIX_BEFORE=$(cksum <"$OAW_MATRIX_PATH")
  OAW_MATRIX_NATIVE_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE")
  OAW_MATRIX_NATIVE_POLICY_BEFORE=
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    OAW_MATRIX_NATIVE_POLICY_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE_POLICY")
  OAW_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
  run_oaw uninstall --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET" --dry-run
  assert_status 0 "$OAW_MATRIX_TARGET project uninstall dry run"
  [ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall dry run changed policy"
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall dry run changed target"
  [ "$(cksum <"$OAW_MATRIX_NATIVE")" = "$OAW_MATRIX_NATIVE_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall dry run changed native entrypoint"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    [ "$(cksum <"$OAW_MATRIX_NATIVE_POLICY")" = "$OAW_MATRIX_NATIVE_POLICY_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET uninstall dry run changed native policy metadata"
  [ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall dry run changed state"

  run_oaw uninstall --project "$OAW_PROJECT" --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET project final uninstall"
  case "$OAW_MATRIX_TARGET" in
    claude|codex|gemini|opencode) grep -Fx "personal $OAW_MATRIX_TARGET project instruction" "$OAW_MATRIX_PATH" >/dev/null || fail "$OAW_MATRIX_TARGET uninstall changed project content" ;;
    *) [ ! -e "$OAW_MATRIX_PATH" ] || fail "$OAW_MATRIX_TARGET uninstall kept owned target" ;;
  esac
  [ ! -e "$OAW_MATRIX_NATIVE" ] || fail "$OAW_MATRIX_TARGET uninstall kept native entrypoint"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] || [ ! -e "$OAW_MATRIX_NATIVE_POLICY" ] ||
    fail "$OAW_MATRIX_TARGET uninstall kept native policy metadata"
  [ -z "$OAW_MATRIX_SIBLING" ] || [ "$(cksum <"$OAW_MATRIX_SIBLING")" = "$OAW_MATRIX_SIBLING_BEFORE" ] || fail "$OAW_MATRIX_TARGET uninstall changed sibling"
done
pass "all nine project targets complete copied-update and dry-run management"

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
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "claude/router,claude/native-entrypoint,codex/router,codex/native-entrypoint,codex/native-policy,gemini/router,gemini/native-entrypoint,opencode/router,opencode/native-entrypoint,cursor/router,cursor/native-entrypoint,windsurf/router,windsurf/native-entrypoint,cline/router,cline/native-entrypoint,roo/router,roo/native-entrypoint,copilot/router,copilot/native-entrypoint" \
  "default project artifact order is not deterministic"
for OAW_DEFAULT_TARGET in claude codex gemini opencode cursor windsurf cline roo copilot; do
  assert_project_host_artifacts "$OAW_PROJECT_STATE" "$OAW_PROJECT_PHYSICAL" \
    "$OAW_DEFAULT_TARGET" created-file
done
run_oaw install --project "$OAW_PROJECT"
assert_status 0 "repeated default nine-target project install"
for OAW_DEFAULT_ARTIFACT in \
  claude/router claude/native-entrypoint \
  codex/router codex/native-entrypoint codex/native-policy \
  gemini/router gemini/native-entrypoint \
  opencode/native-entrypoint \
  cursor/router cursor/native-entrypoint \
  windsurf/router windsurf/native-entrypoint \
  cline/router cline/native-entrypoint \
  roo/router roo/native-entrypoint \
  copilot/router copilot/native-entrypoint
do
  assert_contains "unchanged: $OAW_DEFAULT_ARTIFACT" \
    "repeated default install does not report $OAW_DEFAULT_ARTIFACT unchanged"
done
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
for OAW_DEFAULT_TARGET in claude codex gemini opencode cursor windsurf cline roo copilot; do
  OAW_DEFAULT_NATIVE=$OAW_PROJECT_PHYSICAL/$(project_native_path_for_test "$OAW_DEFAULT_TARGET")
  cksum <"$OAW_DEFAULT_NATIVE" >"$OAW_SANDBOX/default-native-$OAW_DEFAULT_TARGET-before"
done
OAW_DEFAULT_CODEX_POLICY=$OAW_PROJECT_PHYSICAL/$(project_native_policy_path_for_test codex)
cksum <"$OAW_DEFAULT_CODEX_POLICY" >"$OAW_SANDBOX/default-native-policy-codex-before"
run_oaw update --project "$OAW_PROJECT" --dry-run
assert_status 0 "default project update dry run"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "default update dry run changed policy"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "default update dry run changed state"
[ "$(cksum <"$OAW_DEFAULT_AGENTS")" = "$OAW_AGENTS_BEFORE" ] || fail "default update dry run changed AGENTS"
[ "$(cksum <"$OAW_DEFAULT_CURSOR")" = "$OAW_CURSOR_BEFORE" ] || fail "default update dry run changed Cursor"
for OAW_DEFAULT_TARGET in claude codex gemini opencode cursor windsurf cline roo copilot; do
  OAW_DEFAULT_NATIVE=$OAW_PROJECT_PHYSICAL/$(project_native_path_for_test "$OAW_DEFAULT_TARGET")
  [ "$(cksum <"$OAW_DEFAULT_NATIVE")" = \
    "$(cat "$OAW_SANDBOX/default-native-$OAW_DEFAULT_TARGET-before")" ] ||
    fail "default update dry run changed $OAW_DEFAULT_TARGET native entrypoint"
done
[ "$(cksum <"$OAW_DEFAULT_CODEX_POLICY")" = \
  "$(cat "$OAW_SANDBOX/default-native-policy-codex-before")" ] ||
  fail "default update dry run changed Codex native policy metadata"
run_oaw update --project "$OAW_PROJECT"
assert_status 0 "default project copied-checkout update"
grep -F 'TASK 4 DEFAULT UPDATE SENTINEL' "$OAW_POLICY" >/dev/null || fail "default update ignored checkout"
grep -F "$(printf 'version\t0.1.1-project-default')" "$OAW_PROJECT_STATE" >/dev/null || fail "default update did not record version"
for OAW_MATRIX_TARGET in claude codex gemini opencode cursor windsurf cline roo copilot; do
  OAW_MATRIX_RELATIVE=$(project_target_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_PATH=$OAW_PROJECT_PHYSICAL/$OAW_MATRIX_RELATIVE
  assert_lazy_router_file "$OAW_MATRIX_PATH" "$OAW_PROJECT_POLICY_REFERENCE" "$OAW_MATRIX_TARGET default updated project instructions"
  assert_native_entrypoint \
    "$OAW_PROJECT_PHYSICAL/$(project_native_path_for_test "$OAW_MATRIX_TARGET")" \
    "$OAW_MATRIX_TARGET"
done
grep -F 'allow_implicit_invocation: false' "$OAW_DEFAULT_CODEX_POLICY" >/dev/null ||
  fail "default update lost Codex native policy metadata"
run_oaw uninstall --project "$OAW_PROJECT" --target cursor
assert_status 0 "default project selected uninstall"
[ ! -e "$OAW_DEFAULT_CURSOR" ] || fail "selected uninstall kept Cursor"
[ ! -e "$OAW_PROJECT_PHYSICAL/.cursor/skills/oaw/SKILL.md" ] ||
  fail "selected uninstall kept Cursor native entrypoint"
[ -f "$OAW_POLICY" ] || fail "selected uninstall removed referenced policy"
assert_state_artifacts "$OAW_PROJECT_STATE" \
  "claude/router,claude/native-entrypoint,codex/router,codex/native-entrypoint,codex/native-policy,gemini/router,gemini/native-entrypoint,opencode/router,opencode/native-entrypoint,windsurf/router,windsurf/native-entrypoint,cline/router,cline/native-entrypoint,roo/router,roo/native-entrypoint,copilot/router,copilot/native-entrypoint" \
  "selected uninstall did not retain every non-Cursor artifact"
run_oaw uninstall --project "$OAW_PROJECT"
assert_status 0 "default project final uninstall"
[ ! -e "$OAW_POLICY" ] || fail "default final uninstall kept policy"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "default final uninstall kept state"
for OAW_DEFAULT_TARGET in claude codex gemini opencode cursor windsurf cline roo copilot; do
  [ ! -e "$OAW_PROJECT_PHYSICAL/$(project_native_path_for_test "$OAW_DEFAULT_TARGET")" ] ||
    fail "default final uninstall kept $OAW_DEFAULT_TARGET native entrypoint"
done
[ ! -e "$OAW_DEFAULT_CODEX_POLICY" ] || fail "default final uninstall kept Codex native policy"
pass "default project set completes update and selected uninstall management"

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
OAW_CURSOR_NATIVE=$OAW_PROJECT_PHYSICAL/.cursor/skills/oaw/SKILL.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_CURSOR_BEFORE=$(cksum <"$OAW_CURSOR")
OAW_CURSOR_NATIVE_BEFORE=$(cksum <"$OAW_CURSOR_NATIVE")
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_PROJECT_STATE")
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check reports clean adapter"
assert_contains "installed cursor: clean" "project check reports clean state"
[ "$(cksum <"$OAW_CURSOR")" = "$OAW_CURSOR_BEFORE" ] || fail "clean project check changed Cursor Router"
[ "$(cksum <"$OAW_CURSOR_NATIVE")" = "$OAW_CURSOR_NATIVE_BEFORE" ] || fail "clean project check changed Cursor native entrypoint"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "clean project check changed policy"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_STATE_BEFORE" ] || fail "clean project check changed state"
printf 'drifted Cursor check\n' >"$OAW_CURSOR"
OAW_CURSOR_DRIFT_BEFORE=$(cksum <"$OAW_CURSOR")
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check reports drift"
assert_contains "installed cursor: drift" "project check reports owned-file drift"
[ "$(cksum <"$OAW_CURSOR")" = "$OAW_CURSOR_DRIFT_BEFORE" ] || fail "project check changed drifted Cursor Router"
[ "$(cksum <"$OAW_CURSOR_NATIVE")" = "$OAW_CURSOR_NATIVE_BEFORE" ] || fail "project check changed Cursor native entrypoint"
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
[ "$(cksum <"$OAW_CURSOR")" = "$OAW_CURSOR_DRIFT_BEFORE" ] || fail "invalid-state check changed drifted Cursor Router"
[ "$(cksum <"$OAW_CURSOR_NATIVE")" = "$OAW_CURSOR_NATIVE_BEFORE" ] || fail "invalid-state check changed Cursor native entrypoint"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] || fail "invalid-state check changed policy"
[ "$(cksum <"$OAW_PROJECT_STATE")" = "$OAW_TAMPERED_STATE_BEFORE" ] || fail "project check rewrote invalid state"
pass "project check reports clean, drift, and invalid-state without writes"
