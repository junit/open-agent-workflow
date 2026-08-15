#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
. "$TEST_DIR/test-helper.sh"

BRIDGE_RELEASE=

cleanup() {
  cleanup_sandbox
  if [ -n "$BRIDGE_RELEASE" ] && [ -d "$BRIDGE_RELEASE" ]; then
    rm -rf -- "$BRIDGE_RELEASE"
  fi
}
trap cleanup EXIT HUP INT TERM

fail_if_contains() {
  file=$1
  text=$2
  if grep -F -- "$text" "$file" >/dev/null; then
    fail "$file contains retired Bridge authority: $text"
  fi
}

assert_bridge_skill_v3() {
  skill=$OAW_REPOSITORY/internal/codexbridge/install/assets/skills/oaw-codex-bridge/SKILL.md
  for required in \
    'observe_profile' \
    'Assurance Overlay' \
    'does not select or run the Profile' \
    'current Codex `skills/list` metadata' \
    'revoked, failed, or incomplete Bridge' \
    'Markdown Profile remains usable'; do
    grep -F -- "$required" "$skill" >/dev/null ||
      fail "installed Bridge Skill omits required v3 instruction: $required"
  done
  for forbidden in \
    observe_current core_inspect core_compile workflow_exchange \
    HostEvidenceHandle SubagentStart 'Resource Lease' 'Lifecycle Bundle'; do
    fail_if_contains "$skill" "$forbidden"
  done
}

setup_sandbox
assert_bridge_skill_v3
BRIDGE_RELEASE=$(mktemp -d "${TMPDIR:-/tmp}/oaw-bridge-protocol.XXXXXX")
(cd "$OAW_REPOSITORY" && go build -o "$BRIDGE_RELEASE/oaw" ./cmd/oaw) ||
  fail 'cannot build default oaw binary'
(cd "$OAW_REPOSITORY" && go build -o "$BRIDGE_RELEASE/oaw-bridge" ./cmd/oaw-bridge) ||
  fail 'cannot build standalone oaw-bridge binary'

run_bridge_hook() {
  name=$1
  input=$2
  stdout=$OAW_SANDBOX/$name.stdout
  stderr=$OAW_SANDBOX/$name.stderr
  set +e
  HOME="$OAW_HOME" XDG_CONFIG_HOME="$OAW_CONFIG" XDG_STATE_HOME="$OAW_STATE" \
    "$BRIDGE_RELEASE/oaw-bridge" hook codex <"$input" >"$stdout" 2>"$stderr"
  status=$?
  set -e
  [ "$status" -eq 0 ] || fail "$name exited $status: $(cat "$stderr")"
}

set +e
HOME="$OAW_HOME" XDG_CONFIG_HOME="$OAW_CONFIG" XDG_STATE_HOME="$OAW_STATE" \
  "$BRIDGE_RELEASE/oaw" profile check built-in:SP-FULL >"$OAW_SANDBOX/profile.stdout" 2>"$OAW_SANDBOX/profile.stderr"
profile_status=$?
set -e
[ "$profile_status" -eq 0 ] || fail 'default oaw Profile inspection depends on Bridge state'
grep -F 'result: metadata-valid' "$OAW_SANDBOX/profile.stdout" >/dev/null ||
  fail 'default oaw Profile inspection omitted its advisory result'
assert_read_only_roots

cat >"$OAW_SANDBOX/observe-profile.json" <<EOF
{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"$OAW_PROJECT","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_profile","tool_input":{"profile":"project:team"}}
EOF
run_bridge_hook observe-profile "$OAW_SANDBOX/observe-profile.json"
grep -F '"hookEventName":"PreToolUse"' "$OAW_SANDBOX/observe-profile.stdout" >/dev/null ||
  fail 'observe_profile omitted PreToolUse envelope'
grep -F '"permissionDecision":"allow"' "$OAW_SANDBOX/observe-profile.stdout" >/dev/null ||
  fail 'observe_profile was not allowed'
grep -F '"profile":"project:team"' "$OAW_SANDBOX/observe-profile.stdout" >/dev/null ||
  fail 'observe_profile selector was not preserved'
grep -F '"_oaw_host_context"' "$OAW_SANDBOX/observe-profile.stdout" >/dev/null ||
  fail 'observe_profile omitted reserved Host context'
grep -F 'oaw.codex-bridge/v3' "$OAW_SANDBOX/observe-profile.stdout" >/dev/null ||
  fail 'observe_profile omitted Bridge protocol v3'
grep -F 'oaw.codex-hook-context/v3' "$OAW_SANDBOX/observe-profile.stdout" >/dev/null ||
  fail 'observe_profile omitted Hook Context v3'

for retired in observe_current core_inspect core_compile workflow_exchange; do
  cat >"$OAW_SANDBOX/$retired.json" <<EOF
{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"$OAW_PROJECT","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__$retired","tool_input":{}}
EOF
  run_bridge_hook "$retired" "$OAW_SANDBOX/$retired.json"
  grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/$retired.stdout" >/dev/null ||
    fail "$retired did not fail closed"
  fail_if_contains "$OAW_SANDBOX/$retired.stdout" 'updatedInput'
done

cat >"$OAW_SANDBOX/injected-context.json" <<EOF
{"session_id":"session-a","transcript_path":null,"turn_id":"turn-a","tool_use_id":"tool-a","cwd":"$OAW_PROJECT","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_profile","tool_input":{"profile":"project:team","_oaw_host_context":{}}}
EOF
run_bridge_hook injected-context "$OAW_SANDBOX/injected-context.json"
grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/injected-context.stdout" >/dev/null ||
  fail 'caller-supplied reserved Host context did not fail closed'
fail_if_contains "$OAW_SANDBOX/injected-context.stdout" 'updatedInput'

[ ! -e "$OAW_STATE/open-agent-workflow/codex-bridge/features" ] ||
  fail 'Bridge v3 Hook retained SubagentStart feature state'

(cd "$OAW_REPOSITORY" && go test \
  ./internal/codexbridge/... ./internal/bridgecli ./internal/integration \
  -run 'ObserveProfile|StandaloneCodexBridge|StandaloneBridge|DefaultCodexPolicy') >/dev/null ||
  fail 'Bridge v3 MCP and Overlay integration tests failed'

printf 'PASS: Codex Assurance Bridge exposes only observe_profile and v3 Hook context\n'
