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
    fail "$file contains forbidden text: $text"
  fi
}

assert_bridge_skill_v2() {
  skill=$OAW_REPOSITORY/internal/codexbridge/install/assets/skills/oaw-codex-bridge/SKILL.md
  for required in \
    'observe_current' \
    'core_inspect' \
    'core_compile' \
    'workflow_exchange' \
    'oaw.codex-bridge/v2' \
    'oaw.codex-hook-context/v2' \
    'oaw.host-evidence-handle/v2' \
    'installation integrity' \
    'live protocol proof' \
    'same session and working directory' \
    'Never persist'; do
    grep -F -- "$required" "$skill" >/dev/null ||
      fail "installed Bridge Skill omits required v2 instruction: $required"
  done
  for forbidden in runtime_exchange workflow_start resume_workflow codex_exec; do
    fail_if_contains "$skill" "$forbidden"
  done
}

setup_sandbox
assert_bridge_skill_v2
BRIDGE_RELEASE=$(mktemp -d "${TMPDIR:-/tmp}/oaw-bridge-protocol.XXXXXX")
cp "$OAW_REPOSITORY/install.sh" "$BRIDGE_RELEASE/install.sh"
chmod 755 "$BRIDGE_RELEASE/install.sh"
(cd "$OAW_REPOSITORY" && go build -o "$BRIDGE_RELEASE/oaw" ./cmd/oaw) ||
  fail 'cannot build Codex Bridge protocol binary'
OAW_INSTALLER=$BRIDGE_RELEASE/install.sh

run_input() {
  name=$1
  expected_status=$2
  input=$3
  shift 3
  stdout=$OAW_SANDBOX/$name.stdout
  stderr=$OAW_SANDBOX/$name.stderr
  set +e
  HOME="$OAW_HOME" XDG_CONFIG_HOME="$OAW_CONFIG" XDG_STATE_HOME="$OAW_STATE" \
    bash "$OAW_INSTALLER" "$@" <"$input" >"$stdout" 2>"$stderr"
  status=$?
  set -e
  [ "$status" -eq "$expected_status" ] ||
    fail "$name exited $status, want $expected_status: $(cat "$stderr")"
}

run_oaw catalog validate
assert_status 0 'catalog validate must not require Bridge state'
assert_read_only_roots

run_oaw bridge hook claude
assert_status 64 'Bridge Hook must reject a non-Codex Host'

run_oaw help
assert_status 0 'root help must remain available'
fail_if_contains "$OAW_OUTPUT_FILE" 'codex exec'
fail_if_contains "$OAW_OUTPUT_FILE" 'NATIVE_SUBAGENT'

cat >"$OAW_SANDBOX/observe.json" <<'EOF'
{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}
EOF
run_input observe 0 "$OAW_SANDBOX/observe.json" bridge hook codex
grep -F '"hookEventName":"PreToolUse"' "$OAW_SANDBOX/observe.stdout" >/dev/null || fail 'observation omitted PreToolUse envelope'
grep -F '"permissionDecision":"allow"' "$OAW_SANDBOX/observe.stdout" >/dev/null || fail 'observation was not allowed'
grep -F '"_oaw_host_context"' "$OAW_SANDBOX/observe.stdout" >/dev/null || fail 'observation omitted reserved Host context'
grep -F 'oaw.codex-hook-context/v2' "$OAW_SANDBOX/observe.stdout" >/dev/null || fail 'observation omitted Hook Context v2'

digest_header() {
  kind=$1
  value=$2
  if command -v shasum >/dev/null 2>&1; then
    printf 'oaw.host-evidence-handle/v2\000%s\000%s' "$kind" "$value" |
      shasum -a 256 | awk '{print $1}'
  else
    printf 'oaw.host-evidence-handle/v2\000%s\000%s' "$kind" "$value" |
      sha256sum | awk '{print $1}'
  fi
}

session_digest=$(digest_header session session-a)
cwd_digest=$(digest_header cwd /repo)
cat >"$OAW_SANDBOX/handle.json" <<EOF
{"version":"oaw.host-evidence-handle/v2","session_digest":"$session_digest","cwd_digest":"$cwd_digest","token":"opaque-test-token"}
EOF

for tool_name in core_inspect core_compile workflow_exchange; do
  cat >"$OAW_SANDBOX/$tool_name.json" <<EOF
{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__$tool_name","tool_input":{"host_evidence_handle":$(cat "$OAW_SANDBOX/handle.json")}}
EOF
  run_input "$tool_name" 0 "$OAW_SANDBOX/$tool_name.json" bridge hook codex
  [ ! -s "$OAW_SANDBOX/$tool_name.stdout" ] || fail "$tool_name emitted stdout for a valid handle"
done

cat >"$OAW_SANDBOX/wrong-event.json" <<'EOF'
{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PostToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__observe_current","tool_input":{}}
EOF
run_input wrong-event 0 "$OAW_SANDBOX/wrong-event.json" bridge hook codex
grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/wrong-event.stdout" >/dev/null || fail 'wrong Hook event did not fail closed'
fail_if_contains "$OAW_SANDBOX/wrong-event.stdout" 'updatedInput'

cat >"$OAW_SANDBOX/edited-handle.json" <<EOF
{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__core_inspect","tool_input":{"host_evidence_handle":{"version":"oaw.host-evidence-handle/v2","session_digest":"$(printf '%064d' 0)","cwd_digest":"$cwd_digest","token":"opaque-test-token"}}}
EOF
run_input edited-handle 0 "$OAW_SANDBOX/edited-handle.json" bridge hook codex
grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/edited-handle.stdout" >/dev/null || fail 'edited handle did not fail closed'
fail_if_contains "$OAW_SANDBOX/edited-handle.stdout" 'updatedInput'

cat >"$OAW_SANDBOX/v1-handle.json" <<EOF
{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__core_inspect","tool_input":{"host_evidence_handle":{"version":"oaw.host-evidence-handle/v1","session_digest":"$session_digest","cwd_digest":"$cwd_digest","token":"opaque-test-token"}}}
EOF
run_input v1-handle 0 "$OAW_SANDBOX/v1-handle.json" bridge hook codex
grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/v1-handle.stdout" >/dev/null || fail 'v1 handle did not fail closed'
fail_if_contains "$OAW_SANDBOX/v1-handle.stdout" 'updatedInput'

cat >"$OAW_SANDBOX/foreign-cwd.json" <<EOF
{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/foreign-repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__core_inspect","tool_input":{"host_evidence_handle":$(cat "$OAW_SANDBOX/handle.json")}}
EOF
run_input foreign-cwd 0 "$OAW_SANDBOX/foreign-cwd.json" bridge hook codex
grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/foreign-cwd.stdout" >/dev/null || fail 'foreign CWD did not fail closed'
fail_if_contains "$OAW_SANDBOX/foreign-cwd.stdout" 'updatedInput'

cat >"$OAW_SANDBOX/missing-handle.json" <<'EOF'
{"session_id":"session-a","turn_id":"turn-a","tool_use_id":"tool-a","cwd":"/repo","hook_event_name":"PreToolUse","model":"gpt-test","permission_mode":"default","tool_name":"mcp__oaw_codex_bridge__core_inspect","tool_input":{}}
EOF
run_input missing-handle 0 "$OAW_SANDBOX/missing-handle.json" bridge hook codex
grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/missing-handle.stdout" >/dev/null || fail 'missing handle did not fail closed'
fail_if_contains "$OAW_SANDBOX/missing-handle.stdout" 'updatedInput'

(cd "$OAW_REPOSITORY" && go test ./internal/codexbridge -run 'BridgeV2|VersionEvidence|HandleV2|BindingTree|HostFactsV3|CoreInspectV4|CoreCompileV4|WorkflowV2|ReceiptV3|ConformanceV4') >/dev/null

printf 'PASS: Codex Bridge MCP/Hook boundaries and fail-closed protocol cases passed\n'
