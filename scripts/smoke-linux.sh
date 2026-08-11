#!/usr/bin/env bash

set -eu

LINUX_SMOKE_TEMP=

cleanup() {
  if [ -n "$LINUX_SMOKE_TEMP" ] && [ -d "$LINUX_SMOKE_TEMP" ]; then
    rm -rf -- "$LINUX_SMOKE_TEMP"
  fi
}

fail() {
  printf 'Linux smoke: error: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT HUP INT TERM

[ "$#" -eq 1 ] || fail "usage: scripts/smoke-linux.sh <absolute-linux-release-archive>"
ARCHIVE=$1
case "$ARCHIVE" in
  /*) ;;
  *) fail "release archive path must be absolute" ;;
esac
[ -f "$ARCHIVE" ] || fail "release archive does not exist: $ARCHIVE"
[ ! -L "$ARCHIVE" ] || fail "release archive must not be a symlink"

LINUX_SMOKE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-linux-smoke.XXXXXX") ||
  fail "cannot create smoke directory"
if tar -tzf "$ARCHIVE" | grep -E '(^/|(^|/)\.\.(/|$))' >/dev/null; then
  fail "release archive contains an unsafe path"
fi
if tar -tzf "$ARCHIVE" | grep -Ei '(^|/)[^/]*runner[^/]*(/|$)' >/dev/null; then
  fail "release archive contains a Runner asset"
fi
tar -xzf "$ARCHIVE" -C "$LINUX_SMOKE_TEMP"
PACKAGE=$(find "$LINUX_SMOKE_TEMP" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$PACKAGE" ] || fail "release archive has no package directory"
[ ! -L "$PACKAGE/oaw" ] || fail "release binary must not be a symlink"
[ ! -L "$PACKAGE/install.sh" ] || fail "release wrapper must not be a symlink"
[ -x "$PACKAGE/oaw" ] || fail "release archive has no executable Linux oaw binary"
[ -x "$PACKAGE/install.sh" ] || fail "release archive has no executable wrapper"

SMOKE_HOME=$LINUX_SMOKE_TEMP/home
SMOKE_CONFIG=$LINUX_SMOKE_TEMP/config
SMOKE_STATE=$LINUX_SMOKE_TEMP/state
SMOKE_PROJECT=$LINUX_SMOKE_TEMP/policy-only-project
SMOKE_TRAPS=$LINUX_SMOKE_TEMP/model-traps
MODEL_SENTINEL=$LINUX_SMOKE_TEMP/model-executed
WORKFLOW_STATE=$SMOKE_STATE/open-agent-workflow/workflows
POLICY_ONLY=$SMOKE_PROJECT/.scratch/existing-task/workflow.md
mkdir -p "$SMOKE_HOME" "$SMOKE_CONFIG" "$SMOKE_STATE" "$SMOKE_TRAPS" "$(dirname -- "$POLICY_ONLY")"
printf 'profile: ECC-FULL\nstage: implementation\n' >"$POLICY_ONLY"
POLICY_ONLY_BEFORE=$(cksum <"$POLICY_ONLY")

for model_command in codex claude gemini opencode; do
  {
    printf '%s\n' '#!/usr/bin/env bash'
    printf '%s\n' 'printf "%s\n" "$0" >>"$OAW_MODEL_SENTINEL"'
    printf '%s\n' 'exit 99'
  } >"$SMOKE_TRAPS/$model_command"
  chmod 755 "$SMOKE_TRAPS/$model_command"
done

run_oaw() {
  name=$1
  expected_status=$2
  input=$3
  shift 3
  set +e
  HOME="$SMOKE_HOME" \
    XDG_CONFIG_HOME="$SMOKE_CONFIG" \
    XDG_STATE_HOME="$SMOKE_STATE" \
    PATH="$SMOKE_TRAPS:$PATH" \
    OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
    "$PACKAGE/oaw" "$@" \
    <"$input" >"$LINUX_SMOKE_TEMP/$name.stdout" 2>"$LINUX_SMOKE_TEMP/$name.stderr"
  status=$?
  set -e
  [ "$status" -eq "$expected_status" ] ||
    fail "$name exited $status, want $expected_status: $(cat "$LINUX_SMOKE_TEMP/$name.stderr")"
}

assert_no_workflow_state() {
  [ ! -e "$WORKFLOW_STATE" ] || fail "non-Workflow command created Workflow State"
}

run_oaw help 0 /dev/null --help
HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  PATH="$SMOKE_TRAPS:$PATH" OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
  bash "$PACKAGE/install.sh" --help >"$LINUX_SMOKE_TEMP/wrapper-help.stdout"
cmp -s "$LINUX_SMOKE_TEMP/help.stdout" "$LINUX_SMOKE_TEMP/wrapper-help.stdout" ||
  fail "wrapper help differs from binary help"
run_oaw catalog 0 /dev/null catalog validate
grep -F 'catalog valid' "$LINUX_SMOKE_TEMP/catalog.stdout" >/dev/null ||
  fail "catalog validation failed"

CODEX_PROVIDER=$SMOKE_HOME/.codex/plugins/superpowers
CLAUDE_PROVIDER=$SMOKE_HOME/.claude/plugins/superpowers
mkdir -p \
  "$CODEX_PROVIDER/skills/using-superpowers" \
  "$CLAUDE_PROVIDER/skills/using-superpowers"
printf '%s\n' 'codex-smoke-marker' \
  >"$CODEX_PROVIDER/skills/using-superpowers/SKILL.md"
printf '%s\n' 'claude-smoke-marker' \
  >"$CLAUDE_PROVIDER/skills/using-superpowers/SKILL.md"

run_oaw provider-inspection 0 /dev/null providers inspect --host codex --format json
PROVIDER_INSPECTION=$(cat "$LINUX_SMOKE_TEMP/provider-inspection.stdout")
case "$PROVIDER_INSPECTION" in
  *'"current_host":'*'"foreign_hosts":'*) ;;
  *) fail "Provider inspection omits current or foreign Host section" ;;
esac
CURRENT_HOST=${PROVIDER_INSPECTION%%\"foreign_hosts\":*}
FOREIGN_HOSTS=${PROVIDER_INSPECTION#*\"foreign_hosts\":}
case "$CURRENT_HOST" in
  *'.claude/'*) fail "current Host inspection contains a foreign Claude path" ;;
esac
case "$CURRENT_HOST" in
  *'.codex/'*) ;;
  *) fail "current Host inspection omits the Codex Candidate" ;;
esac
case "$FOREIGN_HOSTS" in
  *'.claude/'*) ;;
  *) fail "foreign diagnostics omit the Claude Candidate" ;;
esac
case "$FOREIGN_HOSTS" in
  *'provider_pin'*) fail "foreign diagnostics rendered a Provider pin" ;;
esac
[ ! -e "$SMOKE_CONFIG/open-agent-workflow/config.toml" ] ||
  fail "Provider inspection created user configuration"
assert_no_workflow_state

run_oaw removed-runtime 64 /dev/null runtime exchange
run_oaw removed-run 64 /dev/null run --host codex
assert_no_workflow_state

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  PATH="$SMOKE_TRAPS:$PATH" OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
  bash "$PACKAGE/install.sh" install --project "$SMOKE_PROJECT" --target cursor \
  >"$LINUX_SMOKE_TEMP/install.stdout"
find "$SMOKE_STATE/open-agent-workflow/installations/projects" -type f -name '*.state' \
  -print -quit | grep . >/dev/null || fail "install did not create project Install State"
assert_no_workflow_state
[ "$(cksum <"$POLICY_ONLY")" = "$POLICY_ONLY_BEFORE" ] ||
  fail "management changed the Policy-only task"

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  PATH="$SMOKE_TRAPS:$PATH" OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
  bash "$PACKAGE/install.sh" uninstall --project "$SMOKE_PROJECT" --target cursor \
  >"$LINUX_SMOKE_TEMP/uninstall.stdout"
[ "$(cksum <"$POLICY_ONLY")" = "$POLICY_ONLY_BEFORE" ] ||
  fail "uninstall changed the Policy-only task"
assert_no_workflow_state

# Generated through the production Go constructors; rejection is expected
# because the release smoke does not install a trusted Host-native integration.
cat >"$LINUX_SMOKE_TEMP/current-start.json" <<'EOF'
{"schema_version":"oaw.workflow-command/v2","kind":"START","message_id":"message-linux-smoke-start","idempotency_key":"linux-smoke-workflow-start","workflow_id":"","expected_revision":0,"start":{"request_id":"request-linux-smoke","deliverable_id":"deliverable-linux-smoke","input_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","active_ticket":"","proposal":{"schema_version":"oaw.classification-proposal/v1","traits":[],"resources":[],"evidence":[]},"selection":{"profile":"SP-FULL","recipe_id":"oaw/delivery","recipe_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","profile_source":"user-selection","topology":"CURRENT","topology_source":"host-only-option","add_ons":[],"alternatives":[],"overlays":[],"graph_selection_digest":"8f6d96ac58137765cf1cc74d837a2e2fd2bdab823164ab2448efee13dd1935fc"},"host_session":{"schema_version":"oaw.host-session/v3","host_id":"codex","integration_id":"local/linux-smoke-host","integration_version":"3.0.0","session_id":"session-current-linux-smoke","manifest_digest":"f25389abd2f9d7dfdedf44ed8aa9eb213f0978535141471233ac9274661d4c06","supported_topologies":["CURRENT"],"provider_inventory_digest":"bbc455401ac6db9879b3ff93b01281dc46b1790abab14fe2e660964d56317733","feature_observations":[],"feature_digest":"43679eb8ccc2e04ea6ac0a441d63af1357f6bdf33c94ab018adb2ec6500570c5","host_action_observations":[],"host_action_digest":"ffb602f33b29bea9a34ee4efb2156a1d3898377354b3095a4629c94752b5c23a","environment_report_digest":"15ebf31786fdfb78f7c959bf97e8990aa83890967f60f922b284dc64c8bb3445","sandbox_policy_digest":"","approval_policy_digest":"","digest":"afb61608661bd3ec2fe9be98e7b344c7460bfd42063f332647859602d72af034"},"environment":{"schema_version":"oaw.host-environment-report/v2","session_id":"session-current-linux-smoke","parent_session_id":"","topology":"CURRENT","observations":[],"digest":"15ebf31786fdfb78f7c959bf97e8990aa83890967f60f922b284dc64c8bb3445"}}}
EOF
run_oaw workflow-start 65 "$LINUX_SMOKE_TEMP/current-start.json" workflow exchange
grep -F '"kind":"REJECTED"' "$LINUX_SMOKE_TEMP/workflow-start.stdout" >/dev/null ||
  fail "CURRENT Workflow exchange did not return a canonical Result"
[ -d "$WORKFLOW_STATE/records" ] || fail "CURRENT Workflow exchange did not create Workflow State"
[ -z "$(find "$WORKFLOW_STATE/records" -mindepth 1 -print -quit)" ] ||
  fail "rejected CURRENT Workflow exchange persisted a Workflow record"
[ "$(cksum <"$POLICY_ONLY")" = "$POLICY_ONLY_BEFORE" ] ||
  fail "Workflow exchange changed the Policy-only task"
[ ! -e "$MODEL_SENTINEL" ] || fail "OAW launched a model process: $(cat "$MODEL_SENTINEL")"

printf 'PASS: Linux release Core, Coordinator, Provider inspection, no-Runner archive, and no-model boundary verified\n'
