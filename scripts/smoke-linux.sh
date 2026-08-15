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
SMOKE_PROJECT=$LINUX_SMOKE_TEMP/policy-project
SMOKE_TRAPS=$LINUX_SMOKE_TEMP/model-traps
MODEL_SENTINEL=$LINUX_SMOKE_TEMP/model-executed
POLICY_NOTE=$SMOKE_PROJECT/.scratch/existing-task/progress.md
mkdir -p "$SMOKE_HOME" "$SMOKE_CONFIG" "$SMOKE_STATE" "$SMOKE_TRAPS" "$(dirname -- "$POLICY_NOTE")"
printf 'profile: ECC-FULL\nstage: implementation\n' >"$POLICY_NOTE"
POLICY_NOTE_BEFORE=$(cksum <"$POLICY_NOTE")

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
  shift 2
  set +e
  HOME="$SMOKE_HOME" \
    XDG_CONFIG_HOME="$SMOKE_CONFIG" \
    XDG_STATE_HOME="$SMOKE_STATE" \
    PATH="$SMOKE_TRAPS:$PATH" \
    OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
    "$PACKAGE/oaw" "$@" \
    >"$LINUX_SMOKE_TEMP/$name.stdout" 2>"$LINUX_SMOKE_TEMP/$name.stderr"
  status=$?
  set -e
  [ "$status" -eq "$expected_status" ] ||
    fail "$name exited $status, want $expected_status: $(cat "$LINUX_SMOKE_TEMP/$name.stderr")"
}

assert_no_machine_state() {
  [ ! -e "$SMOKE_STATE/open-agent-workflow/workflows" ] || fail "static command created Workflow State"
  [ ! -e "$SMOKE_STATE/open-agent-workflow/policy-engagements" ] || fail "static command created Policy Engagement state"
}

run_oaw help 0 --help
HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  PATH="$SMOKE_TRAPS:$PATH" OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
  bash "$PACKAGE/install.sh" --help >"$LINUX_SMOKE_TEMP/wrapper-help.stdout"
cmp -s "$LINUX_SMOKE_TEMP/help.stdout" "$LINUX_SMOKE_TEMP/wrapper-help.stdout" ||
  fail "wrapper help differs from binary help"
grep -F 'oaw profile list' "$LINUX_SMOKE_TEMP/help.stdout" >/dev/null ||
  fail "help omits advisory Profile inspection"

run_oaw profile-list 0 profile list
grep -F 'built-in:SP-FULL' "$LINUX_SMOKE_TEMP/profile-list.stdout" >/dev/null ||
  fail "Profile list omits SP-FULL"
run_oaw profile-show 0 profile show built-in:SP-FULL
grep -F 'id: SP-FULL' "$LINUX_SMOKE_TEMP/profile-show.stdout" >/dev/null ||
  fail "Profile show omits the selected identity"
run_oaw profile-check 0 profile check built-in:SP-FULL
grep -F 'result: metadata-valid' "$LINUX_SMOKE_TEMP/profile-check.stdout" >/dev/null ||
  fail "Profile check did not validate the static Profile"
assert_no_machine_state

for removed_command in \
  profiles use status complete review approve satisfy incident switch stop uncertain \
  workflow providers policy catalog bridge runtime run; do
  run_oaw "removed-$removed_command" 64 "$removed_command"
  grep -F "unknown command '$removed_command'" "$LINUX_SMOKE_TEMP/removed-$removed_command.stderr" >/dev/null ||
    fail "$removed_command retained a removed-command handler"
done
assert_no_machine_state

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  PATH="$SMOKE_TRAPS:$PATH" OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
  bash "$PACKAGE/install.sh" install --project "$SMOKE_PROJECT" --target cursor \
  >"$LINUX_SMOKE_TEMP/install.stdout"
find "$SMOKE_STATE/open-agent-workflow/installations/projects" -type f -name '*.state' \
  -print -quit | grep . >/dev/null || fail "install did not create project Install State"
assert_no_machine_state
[ "$(cksum <"$POLICY_NOTE")" = "$POLICY_NOTE_BEFORE" ] ||
  fail "management changed the model-owned Progress Note"

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  PATH="$SMOKE_TRAPS:$PATH" OAW_MODEL_SENTINEL="$MODEL_SENTINEL" \
  bash "$PACKAGE/install.sh" uninstall --project "$SMOKE_PROJECT" --target cursor \
  >"$LINUX_SMOKE_TEMP/uninstall.stdout"
[ "$(cksum <"$POLICY_NOTE")" = "$POLICY_NOTE_BEFORE" ] ||
  fail "uninstall changed the model-owned Progress Note"
assert_no_machine_state
[ ! -e "$MODEL_SENTINEL" ] || fail "OAW launched a model process: $(cat "$MODEL_SENTINEL")"

printf 'PASS: Linux release static CLI, Profile inspection, installation, no-state, and no-model boundaries verified\n'
