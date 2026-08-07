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

setup_sandbox
BRIDGE_RELEASE=$(mktemp -d "${TMPDIR:-/tmp}/oaw-bridge-release.XXXXXX")
cp "$OAW_REPOSITORY/install.sh" "$BRIDGE_RELEASE/install.sh"
chmod 755 "$BRIDGE_RELEASE/install.sh"
(cd "$OAW_REPOSITORY" && go build -o "$BRIDGE_RELEASE/oaw" ./cmd/oaw) ||
  fail 'cannot build Codex Bridge test binary'
OAW_INSTALLER=$BRIDGE_RELEASE/install.sh
OAW_DATA=$OAW_SANDBOX/data
OAW_BIN=$OAW_SANDBOX/bin
OAW_CODEX_LOG=$OAW_SANDBOX/codex.argv
mkdir -p "$OAW_DATA" "$OAW_BIN"

cat >"$OAW_BIN/codex" <<'EOF'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >>"$OAW_CODEX_LOG"
case "$*" in
  'plugin marketplace list --json')
    printf '%s\n' '{"marketplaces":[]}'
    ;;
  'plugin list --json')
    printf '%s\n' '{"installed":[]}'
    ;;
  *)
    printf 'unexpected fake Codex command: %s\n' "$*" >&2
    exit 99
    ;;
esac
EOF
chmod 755 "$OAW_BIN/codex"

run_bridge() {
  name=$1
  expected_status=$2
  input=$3
  shift 3
  output=$OAW_SANDBOX/$name.stdout
  error_output=$OAW_SANDBOX/$name.stderr
  set +e
  (
    cd "$OAW_PROJECT"
    HOME="$OAW_HOME" \
      XDG_CONFIG_HOME="$OAW_CONFIG" \
      XDG_STATE_HOME="$OAW_STATE" \
      XDG_DATA_HOME="$OAW_DATA" \
      OAW_CODEX_LOG="$OAW_CODEX_LOG" \
      PATH="$OAW_BIN:$OAW_PATH" \
      bash "$OAW_INSTALLER" "$@" <"$input" >"$output" 2>"$error_output"
  )
  status=$?
  set -e
  if [ "$status" -ne "$expected_status" ]; then
    fail "$name exited $status, want $expected_status: $(cat "$error_output")"
  fi
}

run_bridge help 0 /dev/null bridge --help
grep -F 'oaw bridge serve codex' "$OAW_SANDBOX/help.stdout" >/dev/null ||
  fail 'Bridge help omits serve codex'
grep -F 'oaw bridge install codex' "$OAW_SANDBOX/help.stdout" >/dev/null ||
  fail 'Bridge help omits install codex'

run_bridge unknown-host 64 /dev/null bridge serve claude
run_bridge removed-runtime 64 /dev/null runtime
run_bridge removed-run 64 /dev/null run

run_bridge check 0 /dev/null bridge check codex --format json
grep -F '"current_session_loaded":false' "$OAW_SANDBOX/check.stdout" >/dev/null ||
  fail 'Bridge check inferred a loaded current session'
grep -F '"code":"BRIDGE_INSTALL_NOT_INSTALLED"' "$OAW_SANDBOX/check.stdout" >/dev/null ||
  fail 'Bridge check omitted not-installed diagnosis'

expected_codex_commands='plugin marketplace list --json
plugin list --json'
actual_codex_commands=$(cat "$OAW_CODEX_LOG")
[ "$actual_codex_commands" = "$expected_codex_commands" ] ||
  fail "Bridge check used unexpected Codex argv: $actual_codex_commands"

: >"$OAW_CODEX_LOG"
run_bridge dry-run 0 /dev/null bridge install codex --dry-run --format json
grep -F '"operation":"install"' "$OAW_SANDBOX/dry-run.stdout" >/dev/null ||
  fail 'Bridge dry-run omitted install operation'
grep -F '"changed":false' "$OAW_SANDBOX/dry-run.stdout" >/dev/null ||
  fail 'Bridge dry-run reported a mutation'
[ ! -s "$OAW_CODEX_LOG" ] || fail 'Bridge dry-run invoked Codex'

printf '%s\n' '{"hook_event_name":"wrong"}' >"$OAW_SANDBOX/malformed-hook.json"
run_bridge malformed-hook 0 "$OAW_SANDBOX/malformed-hook.json" bridge hook codex
grep -F '"permissionDecision":"deny"' "$OAW_SANDBOX/malformed-hook.stdout" >/dev/null ||
  fail 'malformed matched Hook did not fail closed'
if grep -F 'updatedInput' "$OAW_SANDBOX/malformed-hook.stdout" >/dev/null; then
  fail 'denied Hook output contains updatedInput'
fi

assert_empty_dir "$OAW_HOME" 'Bridge management must not write HOME'
assert_empty_dir "$OAW_CONFIG" 'Bridge management must not write XDG_CONFIG_HOME'
assert_empty_dir "$OAW_STATE" 'read-only and dry-run Bridge commands must not write XDG_STATE_HOME'
assert_empty_dir "$OAW_DATA" 'read-only and dry-run Bridge commands must not write XDG_DATA_HOME'

printf 'PASS: Codex Bridge management uses isolated roots and official argv only\n'
