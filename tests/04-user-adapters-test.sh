#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

render_expected_activation_router() {
  printf 'Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, if the current project contains `.oaw/policy/POLICY.md`, read that Project Policy Set and do not read or merge the User Policy Set; otherwise read `%s` as the User Policy Set. Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit ends OAW governance for that deliverable.\n' "$1"
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
  grep -F 'if the current project contains `.oaw/policy/POLICY.md`' "$router_file" >/dev/null ||
    fail "$router_description does not prefer the Project Policy Set"
  grep -F "otherwise read \`$router_policy\` as the User Policy Set" "$router_file" >/dev/null ||
    fail "$router_description does not retain the User Policy Set path"
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

target_path_for_test() {
  case "$1" in
    claude) printf '%s/.claude/CLAUDE.md\n' "$OAW_HOME" ;;
    codex) printf '%s/.codex/AGENTS.md\n' "$OAW_HOME" ;;
    gemini) printf '%s/.gemini/GEMINI.md\n' "$OAW_HOME" ;;
    opencode) printf '%s/opencode/AGENTS.md\n' "$OAW_CONFIG" ;;
    *) fail "unknown test target $1" ;;
  esac
}

native_path_for_test() {
  case "$1" in
    claude) printf '%s/.claude/skills/oaw/SKILL.md\n' "$OAW_HOME" ;;
    codex) printf '%s/.agents/skills/oaw/SKILL.md\n' "$OAW_HOME" ;;
    gemini) printf '%s/.gemini/commands/oaw.toml\n' "$OAW_HOME" ;;
    opencode) printf '%s/opencode/commands/oaw.md\n' "$OAW_CONFIG" ;;
    *) fail "unknown user native target $1" ;;
  esac
}

native_policy_path_for_test() {
  if [ "$1" = codex ]; then
    printf '%s/.agents/skills/oaw/agents/openai.yaml\n' "$OAW_HOME"
  fi
}

assert_native_entrypoint() {
  entrypoint=$1
  target_id=$2
  policy_path=$3
  [ -f "$entrypoint" ] || fail "$target_id native entrypoint is missing"
  grep -F "evidence must come from outside this dispatcher's bytes and any Host-expanded template text" "$entrypoint" >/dev/null ||
    fail "$target_id native entrypoint accepts self-originating activation evidence"
  grep -F 'quoted or discussed invocation text' "$entrypoint" >/dev/null ||
    fail "$target_id native entrypoint does not reject quoted or discussed invocations"
  grep -F 'model-led invocation or loading' "$entrypoint" >/dev/null ||
    fail "$target_id native entrypoint does not reject model-led invocation"
  grep -F 'user provenance is unavailable or ambiguous' "$entrypoint" >/dev/null ||
    fail "$target_id native entrypoint lacks ambiguous-source handling"
  grep -F 'do not activate OAW and continue as the native Host' "$entrypoint" >/dev/null ||
    fail "$target_id native entrypoint lacks its automatic-load no-op"
  grep -F 'Follow the current OAW Activation Router to select and read one Policy Set' "$entrypoint" >/dev/null ||
    fail "$target_id native entrypoint bypasses the Activation Router"
  if grep -F "$policy_path" "$entrypoint" >/dev/null; then
    fail "$target_id native entrypoint embeds a Host-preprocessed Policy path"
  fi
  if grep -F '/oaw' "$entrypoint" >/dev/null || grep -F '$oaw' "$entrypoint" >/dev/null; then
    fail "$target_id native entrypoint contains a self-authorizing invocation literal"
  fi
  grep -F 'Pass the optional Profile and task' "$entrypoint" >/dev/null ||
    fail "$target_id native entrypoint does not pass invocation input through"
  for forbidden_profile in MATT-FULL MATT-SP-HYBRID SP-FULL ECC-FULL; do
    if grep -F "$forbidden_profile" "$entrypoint" >/dev/null; then
      fail "$target_id native entrypoint hard-codes Profile $forbidden_profile"
    fi
  done
}

assert_user_host_artifacts() {
  state_file=$1
  target_id=$2
  policy_path=$3
  router_origin=$4
  router_path=$(target_path_for_test "$target_id")
  native_path=$(native_path_for_test "$target_id")

  assert_state_artifact "$state_file" "$target_id" router "$router_path" \
    managed-block "$router_origin" "$target_id Router state row is missing or invalid"
  assert_state_artifact "$state_file" "$target_id" native-entrypoint "$native_path" \
    owned-file created-file "$target_id native-entrypoint state row is missing or invalid"
  assert_native_entrypoint "$native_path" "$target_id" "$policy_path"

  native_policy=$(native_policy_path_for_test "$target_id")
  if [ -n "$native_policy" ]; then
    assert_state_artifact "$state_file" "$target_id" native-policy "$native_policy" \
      owned-file created-file "$target_id native-policy state row is missing or invalid"
    [ -f "$native_policy" ] || fail "$target_id native policy metadata is missing"
    grep -F 'allow_implicit_invocation: false' "$native_policy" >/dev/null ||
      fail "$target_id native policy permits implicit invocation"
  fi
}

seed_target_for_test() {
  seed_target_id=$1
  seed_target_path=$(target_path_for_test "$seed_target_id")
  seed_expected_file=$2
  mkdir -p "$(dirname -- "$seed_target_path")"
  printf 'personal %s instruction\n' "$seed_target_id" >"$seed_expected_file"
  cp "$seed_expected_file" "$seed_target_path"
}

setup_sandbox
mkdir -p "$OAW_HOME/.codex"
printf 'personal instruction\n' >"$OAW_HOME/.codex/AGENTS.md"

run_oaw install --target codex
assert_status 0 "fresh Codex install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_CODEX=$OAW_HOME/.codex/AGENTS.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-codex-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-codex-block

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created for Codex"
[ -f "$OAW_CODEX" ] || fail "Codex instructions were not created at the user destination"
[ -f "$OAW_INSTALL_STATE" ] || fail "Codex installation state was not created"

write_expected_router_block "$OAW_EXPECTED_BLOCK" "$OAW_POLICY"

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
assert_lazy_router_file "$OAW_CODEX" "$OAW_POLICY" "Codex instructions"
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "codex/router,codex/native-entrypoint,codex/native-policy" \
  "Codex state does not contain its complete artifact set"
assert_user_host_artifacts "$OAW_INSTALL_STATE" codex "$OAW_POLICY" existing-file

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_CODEX_BEFORE=$(cksum <"$OAW_CODEX")
OAW_CODEX_NATIVE=$(native_path_for_test codex)
OAW_CODEX_NATIVE_POLICY=$(native_policy_path_for_test codex)
OAW_CODEX_NATIVE_BEFORE=$(cksum <"$OAW_CODEX_NATIVE")
OAW_CODEX_NATIVE_POLICY_BEFORE=$(cksum <"$OAW_CODEX_NATIVE_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target codex
assert_status 0 "repeated Codex install"
for OAW_CODEX_ARTIFACT in router native-entrypoint native-policy; do
  assert_contains "unchanged: codex/$OAW_CODEX_ARTIFACT" \
    "repeated Codex install reports $OAW_CODEX_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated Codex install changed canonical policy bytes"
[ "$(cksum <"$OAW_CODEX")" = "$OAW_CODEX_BEFORE" ] ||
  fail "repeated Codex install changed instruction bytes"
[ "$(cksum <"$OAW_CODEX_NATIVE")" = "$OAW_CODEX_NATIVE_BEFORE" ] ||
  fail "repeated Codex install changed native entrypoint bytes"
[ "$(cksum <"$OAW_CODEX_NATIVE_POLICY")" = "$OAW_CODEX_NATIVE_POLICY_BEFORE" ] ||
  fail "repeated Codex install changed native policy bytes"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "repeated Codex install changed state bytes"

pass "Codex installs a target-native entrypoint idempotently"

cleanup_sandbox
setup_sandbox
mkdir -p "$OAW_HOME/.gemini"
printf 'personal instruction\n' >"$OAW_HOME/.gemini/GEMINI.md"

run_oaw install --target gemini
assert_status 0 "fresh Gemini install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_GEMINI=$OAW_HOME/.gemini/GEMINI.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-gemini-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-gemini-block

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created for Gemini"
[ -f "$OAW_GEMINI" ] || fail "Gemini instructions were not created at the user destination"
[ -f "$OAW_INSTALL_STATE" ] || fail "Gemini installation state was not created"

write_expected_router_block "$OAW_EXPECTED_BLOCK" "$OAW_POLICY"

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
if grep -Fx "@$OAW_POLICY" "$OAW_GEMINI" >/dev/null; then
  fail "Gemini instructions incorrectly use a standalone Markdown import"
fi
assert_lazy_router_file "$OAW_GEMINI" "$OAW_POLICY" "Gemini instructions"
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "gemini/router,gemini/native-entrypoint" \
  "Gemini state does not contain its complete artifact set"
assert_user_host_artifacts "$OAW_INSTALL_STATE" gemini "$OAW_POLICY" existing-file

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_GEMINI_BEFORE=$(cksum <"$OAW_GEMINI")
OAW_GEMINI_NATIVE=$(native_path_for_test gemini)
OAW_GEMINI_NATIVE_BEFORE=$(cksum <"$OAW_GEMINI_NATIVE")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target gemini
assert_status 0 "repeated Gemini install"
for OAW_GEMINI_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: gemini/$OAW_GEMINI_ARTIFACT" \
    "repeated Gemini install reports $OAW_GEMINI_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated Gemini install changed canonical policy bytes"
[ "$(cksum <"$OAW_GEMINI")" = "$OAW_GEMINI_BEFORE" ] ||
  fail "repeated Gemini install changed instruction bytes"
[ "$(cksum <"$OAW_GEMINI_NATIVE")" = "$OAW_GEMINI_NATIVE_BEFORE" ] ||
  fail "repeated Gemini install changed native entrypoint bytes"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "repeated Gemini install changed state bytes"

pass "Gemini installs a target-native entrypoint idempotently"

cleanup_sandbox
setup_sandbox
mkdir -p "$OAW_CONFIG/opencode"
printf 'personal instruction\n' >"$OAW_CONFIG/opencode/AGENTS.md"

run_oaw install --target opencode
assert_status 0 "fresh OpenCode install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_OPENCODE=$OAW_CONFIG/opencode/AGENTS.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_EXPECTED_BLOCK=$OAW_SANDBOX/expected-opencode-block
OAW_ACTUAL_BLOCK=$OAW_SANDBOX/actual-opencode-block

[ -f "$OAW_POLICY" ] || fail "canonical policy was not created for OpenCode"
[ -f "$OAW_OPENCODE" ] || fail "OpenCode instructions were not created at the user destination"
[ -f "$OAW_INSTALL_STATE" ] || fail "OpenCode installation state was not created"

write_expected_router_block "$OAW_EXPECTED_BLOCK" "$OAW_POLICY"

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
assert_lazy_router_file "$OAW_OPENCODE" "$OAW_POLICY" "OpenCode instructions"
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "opencode/router,opencode/native-entrypoint" \
  "OpenCode state does not contain its complete artifact set"
assert_user_host_artifacts "$OAW_INSTALL_STATE" opencode "$OAW_POLICY" existing-file

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_OPENCODE_BEFORE=$(cksum <"$OAW_OPENCODE")
OAW_OPENCODE_NATIVE=$(native_path_for_test opencode)
OAW_OPENCODE_NATIVE_BEFORE=$(cksum <"$OAW_OPENCODE_NATIVE")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target opencode
assert_status 0 "repeated OpenCode install"
for OAW_OPENCODE_ARTIFACT in router native-entrypoint; do
  assert_contains "unchanged: opencode/$OAW_OPENCODE_ARTIFACT" \
    "repeated OpenCode install reports $OAW_OPENCODE_ARTIFACT unchanged"
done
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "repeated OpenCode install changed canonical policy bytes"
[ "$(cksum <"$OAW_OPENCODE")" = "$OAW_OPENCODE_BEFORE" ] ||
  fail "repeated OpenCode install changed instruction bytes"
[ "$(cksum <"$OAW_OPENCODE_NATIVE")" = "$OAW_OPENCODE_NATIVE_BEFORE" ] ||
  fail "repeated OpenCode install changed native entrypoint bytes"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "repeated OpenCode install changed state bytes"

pass "OpenCode installs a target-native entrypoint idempotently"

cleanup_sandbox
setup_sandbox
mkdir -p "$OAW_HOME/.claude" "$OAW_HOME/.codex" "$OAW_HOME/.gemini" "$OAW_CONFIG/opencode"
printf 'personal Claude instruction\n' >"$OAW_HOME/.claude/CLAUDE.md"
printf 'personal Codex instruction\n' >"$OAW_HOME/.codex/AGENTS.md"
printf 'personal Gemini instruction\n' >"$OAW_HOME/.gemini/GEMINI.md"
printf 'personal OpenCode instruction\n' >"$OAW_CONFIG/opencode/AGENTS.md"

OAW_GEMINI_BEFORE=$(cksum <"$OAW_HOME/.gemini/GEMINI.md")
OAW_OPENCODE_BEFORE=$(cksum <"$OAW_CONFIG/opencode/AGENTS.md")
run_oaw install --target claude,codex
assert_status 0 "fresh multi-target install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_CLAUDE=$OAW_HOME/.claude/CLAUDE.md
OAW_CODEX=$OAW_HOME/.codex/AGENTS.md
OAW_GEMINI=$OAW_HOME/.gemini/GEMINI.md
OAW_OPENCODE=$OAW_CONFIG/opencode/AGENTS.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state

assert_state_artifacts "$OAW_INSTALL_STATE" \
  "claude/router,claude/native-entrypoint,codex/router,codex/native-entrypoint,codex/native-policy" \
  "fresh multi-target artifact state is not in registry order"
assert_user_host_artifacts "$OAW_INSTALL_STATE" claude "$OAW_POLICY" existing-file
assert_user_host_artifacts "$OAW_INSTALL_STATE" codex "$OAW_POLICY" existing-file
grep -Fx 'personal Claude instruction' "$OAW_CLAUDE" >/dev/null ||
  fail "multi-target install did not preserve Claude content"
grep -Fx 'personal Codex instruction' "$OAW_CODEX" >/dev/null ||
  fail "multi-target install did not preserve Codex content"
[ "$(cksum <"$OAW_GEMINI")" = "$OAW_GEMINI_BEFORE" ] ||
  fail "multi-target install changed an unselected Gemini destination"
[ "$(cksum <"$OAW_OPENCODE")" = "$OAW_OPENCODE_BEFORE" ] ||
  fail "multi-target install changed an unselected OpenCode destination"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_CLAUDE_BEFORE=$(cksum <"$OAW_CLAUDE")
OAW_CODEX_BEFORE=$(cksum <"$OAW_CODEX")
OAW_OPENCODE_BEFORE=$(cksum <"$OAW_OPENCODE")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target gemini
assert_status 0 "add one target to an existing installation"
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "claude/router,claude/native-entrypoint,codex/router,codex/native-entrypoint,codex/native-policy,gemini/router,gemini/native-entrypoint" \
  "merged artifact state is not unique and registry ordered"
assert_user_host_artifacts "$OAW_INSTALL_STATE" gemini "$OAW_POLICY" existing-file
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "adding Gemini changed the canonical policy"
[ "$(cksum <"$OAW_CLAUDE")" = "$OAW_CLAUDE_BEFORE" ] ||
  fail "adding Gemini changed the retained Claude destination"
[ "$(cksum <"$OAW_CODEX")" = "$OAW_CODEX_BEFORE" ] ||
  fail "adding Gemini changed the retained Codex destination"
[ "$(cksum <"$OAW_OPENCODE")" = "$OAW_OPENCODE_BEFORE" ] ||
  fail "adding Gemini changed the unselected OpenCode destination"
[ "$(cksum <"$OAW_INSTALL_STATE")" != "$OAW_STATE_BEFORE" ] ||
  fail "adding Gemini did not update installation state"
grep -Fx 'personal Gemini instruction' "$OAW_GEMINI" >/dev/null ||
  fail "adding Gemini did not preserve its existing content"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_CLAUDE_BEFORE=$(cksum <"$OAW_CLAUDE")
OAW_CODEX_BEFORE=$(cksum <"$OAW_CODEX")
OAW_GEMINI_BEFORE=$(cksum <"$OAW_GEMINI")
OAW_OPENCODE_BEFORE=$(cksum <"$OAW_OPENCODE")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw install --target codex,claude,claude
assert_status 0 "repeat a deduplicated multi-target selection"
for OAW_REPEAT_ARTIFACT in claude/router claude/native-entrypoint \
  codex/router codex/native-entrypoint codex/native-policy
do
  assert_contains "unchanged: $OAW_REPEAT_ARTIFACT" \
    "deduplicated repeat does not report $OAW_REPEAT_ARTIFACT unchanged"
done
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "claude/router,claude/native-entrypoint,codex/router,codex/native-entrypoint,codex/native-policy,gemini/router,gemini/native-entrypoint" \
  "deduplicated repeat changed retained artifact state"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "deduplicated repeat changed the canonical policy"
[ "$(cksum <"$OAW_CLAUDE")" = "$OAW_CLAUDE_BEFORE" ] ||
  fail "deduplicated repeat changed Claude instructions"
[ "$(cksum <"$OAW_CODEX")" = "$OAW_CODEX_BEFORE" ] ||
  fail "deduplicated repeat changed Codex instructions"
[ "$(cksum <"$OAW_GEMINI")" = "$OAW_GEMINI_BEFORE" ] ||
  fail "deduplicated repeat changed retained Gemini instructions"
[ "$(cksum <"$OAW_OPENCODE")" = "$OAW_OPENCODE_BEFORE" ] ||
  fail "deduplicated repeat changed unselected OpenCode instructions"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "deduplicated repeat changed installation state"

pass "selected installs merge unique target state in registry order"

cleanup_sandbox
setup_sandbox
run_oaw install
assert_status 0 "default user target install"
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "claude/router,claude/native-entrypoint,codex/router,codex/native-entrypoint,codex/native-policy,gemini/router,gemini/native-entrypoint,opencode/router,opencode/native-entrypoint" \
  "default install did not select every artifact for the four user targets"
for OAW_DEFAULT_TARGET in \
  "$OAW_HOME/.claude/CLAUDE.md" \
  "$OAW_HOME/.codex/AGENTS.md" \
  "$OAW_HOME/.gemini/GEMINI.md" \
  "$OAW_CONFIG/opencode/AGENTS.md"
do
  [ -f "$OAW_DEFAULT_TARGET" ] || fail "default install omitted $OAW_DEFAULT_TARGET"
done
for OAW_DEFAULT_HOST in claude codex gemini opencode; do
  assert_user_host_artifacts "$OAW_INSTALL_STATE" "$OAW_DEFAULT_HOST" "$OAW_CONFIG/open-agent-workflow/POLICY.md" created-file
done
pass "default install selects all user targets"

cleanup_sandbox
setup_sandbox
run_oaw install --target all
assert_status 64 "all is not a target alias"
assert_contains "unknown target 'all'" "all is rejected as an unknown target"
assert_read_only_roots
pass "all remains an unknown target without mutation"

for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_MATRIX_PATH=$(target_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_NATIVE=$(native_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_NATIVE_POLICY=$(native_policy_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_EXPECTED=$OAW_SANDBOX/expected-$OAW_MATRIX_TARGET
  seed_target_for_test "$OAW_MATRIX_TARGET" "$OAW_MATRIX_EXPECTED"

  OAW_MATRIX_BEFORE=$(cksum <"$OAW_MATRIX_PATH")
  run_oaw install --target "$OAW_MATRIX_TARGET" --dry-run
  assert_status 0 "$OAW_MATRIX_TARGET install dry run"
  assert_contains "would-update: $OAW_MATRIX_PATH" \
    "$OAW_MATRIX_TARGET install dry run reports its destination"
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET install dry run changed user instructions"
  [ ! -e "$OAW_CONFIG/open-agent-workflow/POLICY.md" ] ||
    fail "$OAW_MATRIX_TARGET install dry run created the canonical policy"
  [ ! -e "$OAW_STATE/open-agent-workflow/installations/user.state" ] ||
    fail "$OAW_MATRIX_TARGET install dry run created installation state"
  [ ! -e "$OAW_MATRIX_NATIVE" ] ||
    fail "$OAW_MATRIX_TARGET install dry run created its native entrypoint"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] || [ ! -e "$OAW_MATRIX_NATIVE_POLICY" ] ||
    fail "$OAW_MATRIX_TARGET install dry run created native policy metadata"

  run_oaw install --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET install before update"
  OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
  OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
  assert_user_host_artifacts "$OAW_INSTALL_STATE" "$OAW_MATRIX_TARGET" "$OAW_POLICY" existing-file

  OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-$OAW_MATRIX_TARGET
  cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
  printf '0.1.1-%s\n' "$OAW_MATRIX_TARGET" >"$OAW_UPDATE_CHECKOUT/VERSION"
  printf '\nTASK 3 %s UPDATE SENTINEL\n' "$OAW_MATRIX_TARGET" \
    >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
  build_checkout_installer "$OAW_UPDATE_CHECKOUT"
  OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

  OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
  OAW_MATRIX_BEFORE=$(cksum <"$OAW_MATRIX_PATH")
  OAW_MATRIX_NATIVE_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE")
  OAW_MATRIX_NATIVE_POLICY_BEFORE=
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    OAW_MATRIX_NATIVE_POLICY_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE_POLICY")
  OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
  run_oaw update --target "$OAW_MATRIX_TARGET" --dry-run
  assert_status 0 "$OAW_MATRIX_TARGET update dry run"
  assert_contains "would-update" "$OAW_MATRIX_TARGET update dry run reports planned writes"
  [ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET update dry run changed the canonical policy"
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET update dry run changed user instructions"
  [ "$(cksum <"$OAW_MATRIX_NATIVE")" = "$OAW_MATRIX_NATIVE_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET update dry run changed its native entrypoint"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    [ "$(cksum <"$OAW_MATRIX_NATIVE_POLICY")" = "$OAW_MATRIX_NATIVE_POLICY_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET update dry run changed native policy metadata"
  [ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET update dry run changed installation state"

  run_oaw update --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET copied-checkout update"
  grep -F "TASK 3 $OAW_MATRIX_TARGET UPDATE SENTINEL" "$OAW_POLICY" >/dev/null ||
    fail "$OAW_MATRIX_TARGET update did not use the copied checkout policy"
  grep -F "$(printf 'version\t0.1.1-%s' "$OAW_MATRIX_TARGET")" "$OAW_INSTALL_STATE" >/dev/null ||
    fail "$OAW_MATRIX_TARGET update did not record the copied checkout version"
  grep -Fx "personal $OAW_MATRIX_TARGET instruction" "$OAW_MATRIX_PATH" >/dev/null ||
    fail "$OAW_MATRIX_TARGET update did not preserve user instructions"
  assert_lazy_router_file "$OAW_MATRIX_PATH" "$OAW_POLICY" "$OAW_MATRIX_TARGET updated instructions"
  assert_native_entrypoint "$OAW_MATRIX_NATIVE" "$OAW_MATRIX_TARGET" "$OAW_POLICY"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    grep -F 'allow_implicit_invocation: false' "$OAW_MATRIX_NATIVE_POLICY" >/dev/null ||
    fail "$OAW_MATRIX_TARGET update lost native policy metadata"

  OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
  OAW_MATRIX_BEFORE=$(cksum <"$OAW_MATRIX_PATH")
  OAW_MATRIX_NATIVE_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE")
  OAW_MATRIX_NATIVE_POLICY_BEFORE=
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    OAW_MATRIX_NATIVE_POLICY_BEFORE=$(cksum <"$OAW_MATRIX_NATIVE_POLICY")
  OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
  run_oaw uninstall --target "$OAW_MATRIX_TARGET" --dry-run
  assert_status 0 "$OAW_MATRIX_TARGET uninstall dry run"
  assert_contains "would-" "$OAW_MATRIX_TARGET uninstall dry run reports planned writes"
  [ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET uninstall dry run changed the canonical policy"
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET uninstall dry run changed user instructions"
  [ "$(cksum <"$OAW_MATRIX_NATIVE")" = "$OAW_MATRIX_NATIVE_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET uninstall dry run changed its native entrypoint"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] ||
    [ "$(cksum <"$OAW_MATRIX_NATIVE_POLICY")" = "$OAW_MATRIX_NATIVE_POLICY_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET uninstall dry run changed native policy metadata"
  [ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
    fail "$OAW_MATRIX_TARGET uninstall dry run changed installation state"

  run_oaw uninstall --target "$OAW_MATRIX_TARGET"
  assert_status 0 "$OAW_MATRIX_TARGET final uninstall"
  cmp -s "$OAW_MATRIX_PATH" "$OAW_MATRIX_EXPECTED" ||
    fail "$OAW_MATRIX_TARGET final uninstall changed user content"
  [ ! -e "$OAW_MATRIX_NATIVE" ] ||
    fail "$OAW_MATRIX_TARGET final uninstall retained its native entrypoint"
  [ -z "$OAW_MATRIX_NATIVE_POLICY" ] || [ ! -e "$OAW_MATRIX_NATIVE_POLICY" ] ||
    fail "$OAW_MATRIX_TARGET final uninstall retained native policy metadata"
  [ ! -e "$OAW_POLICY" ] || fail "$OAW_MATRIX_TARGET final uninstall retained policy"
  [ ! -e "$OAW_INSTALL_STATE" ] || fail "$OAW_MATRIX_TARGET final uninstall retained state"
done
pass "each user target completes copied-update and dry-run management operations"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  seed_target_for_test "$OAW_MATRIX_TARGET" "$OAW_SANDBOX/default-$OAW_MATRIX_TARGET"
done

for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  OAW_MATRIX_PATH=$(target_path_for_test "$OAW_MATRIX_TARGET")
  cksum <"$OAW_MATRIX_PATH" >"$OAW_SANDBOX/default-$OAW_MATRIX_TARGET-before"
done
run_oaw install --dry-run
assert_status 0 "default install dry run"
[ ! -e "$OAW_CONFIG/open-agent-workflow/POLICY.md" ] ||
  fail "default install dry run created the canonical policy"
[ ! -e "$OAW_STATE/open-agent-workflow/installations/user.state" ] ||
  fail "default install dry run created installation state"
for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  OAW_MATRIX_PATH=$(target_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_BEFORE=$(cat "$OAW_SANDBOX/default-$OAW_MATRIX_TARGET-before")
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] ||
    fail "default install dry run changed $OAW_MATRIX_TARGET instructions"
  [ ! -e "$(native_path_for_test "$OAW_MATRIX_TARGET")" ] ||
    fail "default install dry run created $OAW_MATRIX_TARGET native entrypoint"
done

run_oaw install
assert_status 0 "default install before management matrix"
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "claude/router,claude/native-entrypoint,codex/router,codex/native-entrypoint,codex/native-policy,gemini/router,gemini/native-entrypoint,opencode/router,opencode/native-entrypoint" \
  "default management fixture does not record every user artifact"
for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  assert_user_host_artifacts "$OAW_INSTALL_STATE" "$OAW_MATRIX_TARGET" "$OAW_POLICY" existing-file
done
OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-default
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.1-default\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nTASK 3 DEFAULT UPDATE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
build_checkout_installer "$OAW_UPDATE_CHECKOUT"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw update --dry-run
assert_status 0 "default update dry run"
assert_contains "would-update" "default update dry run reports planned writes"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "default update dry run changed the canonical policy"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "default update dry run changed installation state"

run_oaw update
assert_status 0 "default copied-checkout update"
grep -F 'TASK 3 DEFAULT UPDATE SENTINEL' "$OAW_POLICY" >/dev/null ||
  fail "default update did not use copied checkout policy"
grep -F "$(printf 'version\t0.1.1-default')" "$OAW_INSTALL_STATE" >/dev/null ||
  fail "default update did not record copied checkout version"
for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  OAW_MATRIX_PATH=$(target_path_for_test "$OAW_MATRIX_TARGET")
  assert_lazy_router_file "$OAW_MATRIX_PATH" "$OAW_POLICY" "$OAW_MATRIX_TARGET default updated instructions"
  assert_native_entrypoint "$(native_path_for_test "$OAW_MATRIX_TARGET")" \
    "$OAW_MATRIX_TARGET" "$OAW_POLICY"
done
OAW_CODEX_NATIVE_POLICY=$(native_policy_path_for_test codex)
grep -F 'allow_implicit_invocation: false' "$OAW_CODEX_NATIVE_POLICY" >/dev/null ||
  fail "default update lost Codex native policy metadata"

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  OAW_MATRIX_PATH=$(target_path_for_test "$OAW_MATRIX_TARGET")
  cksum <"$OAW_MATRIX_PATH" >"$OAW_SANDBOX/installed-$OAW_MATRIX_TARGET-before"
  OAW_MATRIX_NATIVE=$(native_path_for_test "$OAW_MATRIX_TARGET")
  cksum <"$OAW_MATRIX_NATIVE" >"$OAW_SANDBOX/installed-native-$OAW_MATRIX_TARGET-before"
done
cksum <"$OAW_CODEX_NATIVE_POLICY" >"$OAW_SANDBOX/installed-native-policy-codex-before"
run_oaw uninstall --dry-run
assert_status 0 "default uninstall dry run"
assert_contains "would-" "default uninstall dry run reports planned writes"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "default uninstall dry run changed the canonical policy"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "default uninstall dry run changed installation state"
for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  OAW_MATRIX_PATH=$(target_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_BEFORE=$(cat "$OAW_SANDBOX/installed-$OAW_MATRIX_TARGET-before")
  [ "$(cksum <"$OAW_MATRIX_PATH")" = "$OAW_MATRIX_BEFORE" ] ||
    fail "default uninstall dry run changed $OAW_MATRIX_TARGET instructions"
  OAW_MATRIX_NATIVE=$(native_path_for_test "$OAW_MATRIX_TARGET")
  OAW_MATRIX_NATIVE_BEFORE=$(cat "$OAW_SANDBOX/installed-native-$OAW_MATRIX_TARGET-before")
  [ "$(cksum <"$OAW_MATRIX_NATIVE")" = "$OAW_MATRIX_NATIVE_BEFORE" ] ||
    fail "default uninstall dry run changed $OAW_MATRIX_TARGET native entrypoint"
done
[ "$(cksum <"$OAW_CODEX_NATIVE_POLICY")" = \
  "$(cat "$OAW_SANDBOX/installed-native-policy-codex-before")" ] ||
  fail "default uninstall dry run changed Codex native policy metadata"

run_oaw uninstall --target codex
assert_status 0 "selected Codex uninstall from default state"
assert_state_artifacts "$OAW_INSTALL_STATE" \
  "claude/router,claude/native-entrypoint,gemini/router,gemini/native-entrypoint,opencode/router,opencode/native-entrypoint" \
  "selected uninstall did not retain remaining target rows"
[ -f "$OAW_POLICY" ] || fail "selected uninstall removed the shared policy"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "selected uninstall changed the shared policy"
cmp -s "$OAW_HOME/.codex/AGENTS.md" "$OAW_SANDBOX/default-codex" ||
  fail "selected uninstall did not preserve Codex user content"
[ ! -e "$(native_path_for_test codex)" ] ||
  fail "selected uninstall retained the Codex native entrypoint"
[ ! -e "$OAW_CODEX_NATIVE_POLICY" ] ||
  fail "selected uninstall retained Codex native policy metadata"
for OAW_RETAINED_TARGET in claude gemini opencode; do
  OAW_RETAINED_PATH=$(target_path_for_test "$OAW_RETAINED_TARGET")
  OAW_RETAINED_BEFORE=$(cat "$OAW_SANDBOX/installed-$OAW_RETAINED_TARGET-before")
  [ "$(cksum <"$OAW_RETAINED_PATH")" = "$OAW_RETAINED_BEFORE" ] ||
    fail "selected uninstall changed retained $OAW_RETAINED_TARGET content"
  grep -F '<!-- BEGIN OPEN AGENT WORKFLOW -->' "$OAW_RETAINED_PATH" >/dev/null ||
    fail "selected uninstall removed retained $OAW_RETAINED_TARGET content"
  OAW_RETAINED_NATIVE=$(native_path_for_test "$OAW_RETAINED_TARGET")
  OAW_RETAINED_NATIVE_BEFORE=$(cat "$OAW_SANDBOX/installed-native-$OAW_RETAINED_TARGET-before")
  [ "$(cksum <"$OAW_RETAINED_NATIVE")" = "$OAW_RETAINED_NATIVE_BEFORE" ] ||
    fail "selected uninstall changed retained $OAW_RETAINED_TARGET native entrypoint"
done

OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw update --target codex
assert_status 65 "update rejects a selected target absent from state"
assert_contains "selected target is not installed: codex" \
  "absent selected update reports the missing target"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "rejected absent-target update changed the canonical policy"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "rejected absent-target update changed installation state"

run_oaw uninstall
assert_status 0 "final default uninstall after selected removal"
[ ! -e "$OAW_POLICY" ] || fail "final default uninstall retained the shared policy"
[ ! -e "$OAW_INSTALL_STATE" ] || fail "final default uninstall retained installation state"
for OAW_MATRIX_TARGET in claude codex gemini opencode; do
  OAW_MATRIX_PATH=$(target_path_for_test "$OAW_MATRIX_TARGET")
  cmp -s "$OAW_MATRIX_PATH" "$OAW_SANDBOX/default-$OAW_MATRIX_TARGET" ||
    fail "final default uninstall changed $OAW_MATRIX_TARGET user content"
  [ ! -e "$(native_path_for_test "$OAW_MATRIX_TARGET")" ] ||
    fail "final default uninstall retained $OAW_MATRIX_TARGET native entrypoint"
done
pass "default management supports selected removal and final cleanup"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
run_oaw install
assert_status 0 "default install before health diagnostics"
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_INSTALL_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
for OAW_HEALTH_TARGET in claude codex gemini opencode; do
  OAW_HEALTH_PATH=$(target_path_for_test "$OAW_HEALTH_TARGET")
  cksum <"$OAW_HEALTH_PATH" >"$OAW_SANDBOX/health-$OAW_HEALTH_TARGET-before"
  OAW_HEALTH_NATIVE=$(native_path_for_test "$OAW_HEALTH_TARGET")
  cksum <"$OAW_HEALTH_NATIVE" >"$OAW_SANDBOX/health-native-$OAW_HEALTH_TARGET-before"
done
OAW_HEALTH_CODEX_POLICY=$(native_policy_path_for_test codex)
cksum <"$OAW_HEALTH_CODEX_POLICY" >"$OAW_SANDBOX/health-native-policy-codex-before"
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")

run_oaw check
assert_status 0 "check after default install"
for OAW_HEALTH_TARGET in claude codex gemini opencode; do
  assert_contains "target $OAW_HEALTH_TARGET: detected (user, project)" \
    "check keeps $OAW_HEALTH_TARGET tool readiness separate"
  assert_contains "installed $OAW_HEALTH_TARGET: clean" \
    "check reports clean $OAW_HEALTH_TARGET installation"
  OAW_HEALTH_PATH=$(target_path_for_test "$OAW_HEALTH_TARGET")
  OAW_HEALTH_BEFORE=$(cat "$OAW_SANDBOX/health-$OAW_HEALTH_TARGET-before")
  [ "$(cksum <"$OAW_HEALTH_PATH")" = "$OAW_HEALTH_BEFORE" ] ||
    fail "clean check changed $OAW_HEALTH_TARGET instructions"
  OAW_HEALTH_NATIVE=$(native_path_for_test "$OAW_HEALTH_TARGET")
  OAW_HEALTH_NATIVE_BEFORE=$(cat "$OAW_SANDBOX/health-native-$OAW_HEALTH_TARGET-before")
  [ "$(cksum <"$OAW_HEALTH_NATIVE")" = "$OAW_HEALTH_NATIVE_BEFORE" ] ||
    fail "clean check changed $OAW_HEALTH_TARGET native entrypoint"
done
[ "$(cksum <"$OAW_HEALTH_CODEX_POLICY")" = \
  "$(cat "$OAW_SANDBOX/health-native-policy-codex-before")" ] ||
  fail "clean check changed Codex native policy metadata"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "clean check changed the canonical policy"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "clean check changed installation state"

OAW_CODEX=$OAW_HOME/.codex/AGENTS.md
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { in_block = 1 }
  in_block && $0 != "<!-- BEGIN OPEN AGENT WORKFLOW -->" &&
    $0 != "<!-- END OPEN AGENT WORKFLOW -->" {
    print "drifted Codex managed instruction"
    in_block = 0
    next
  }
  { print }
' "$OAW_CODEX" >"$OAW_SANDBOX/drifted-codex"
mv "$OAW_SANDBOX/drifted-codex" "$OAW_CODEX"
OAW_CODEX_BEFORE=$(cksum <"$OAW_CODEX")
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")

run_oaw check --target codex
assert_status 0 "check with managed target drift"
assert_contains "target codex: detected (user, project)" \
  "drift does not alter Codex tool readiness"
assert_contains "installed codex: drift" "check reports Codex managed drift"
[ "$(cksum <"$OAW_CODEX")" = "$OAW_CODEX_BEFORE" ] ||
  fail "drift check changed Codex instructions"
[ "$(cksum <"$(native_path_for_test codex)")" = \
  "$(cat "$OAW_SANDBOX/health-native-codex-before")" ] ||
  fail "drift check changed Codex native entrypoint"
[ "$(cksum <"$OAW_HEALTH_CODEX_POLICY")" = \
  "$(cat "$OAW_SANDBOX/health-native-policy-codex-before")" ] ||
  fail "drift check changed Codex native policy metadata"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "drift check changed the canonical policy"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "drift check changed installation state"

printf 'invalid\tstate\trecord\n' >>"$OAW_INSTALL_STATE"
OAW_CODEX_BEFORE=$(cksum <"$OAW_CODEX")
OAW_POLICY_BEFORE=$(cksum <"$OAW_POLICY")
OAW_STATE_BEFORE=$(cksum <"$OAW_INSTALL_STATE")
run_oaw check
assert_status 0 "check with invalid installation state"
for OAW_HEALTH_TARGET in claude codex gemini opencode; do
  assert_contains "installed $OAW_HEALTH_TARGET: invalid-state" \
    "check reports invalid state for $OAW_HEALTH_TARGET"
  OAW_HEALTH_NATIVE=$(native_path_for_test "$OAW_HEALTH_TARGET")
  OAW_HEALTH_NATIVE_BEFORE=$(cat "$OAW_SANDBOX/health-native-$OAW_HEALTH_TARGET-before")
  [ "$(cksum <"$OAW_HEALTH_NATIVE")" = "$OAW_HEALTH_NATIVE_BEFORE" ] ||
    fail "invalid-state check changed $OAW_HEALTH_TARGET native entrypoint"
done
[ "$(cksum <"$OAW_HEALTH_CODEX_POLICY")" = \
  "$(cat "$OAW_SANDBOX/health-native-policy-codex-before")" ] ||
  fail "invalid-state check changed Codex native policy metadata"
[ "$(cksum <"$OAW_CODEX")" = "$OAW_CODEX_BEFORE" ] ||
  fail "invalid-state check changed Codex instructions"
[ "$(cksum <"$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "invalid-state check changed the canonical policy"
[ "$(cksum <"$OAW_INSTALL_STATE")" = "$OAW_STATE_BEFORE" ] ||
  fail "invalid-state check changed installation state"
pass "check reports installation health without mutation"
