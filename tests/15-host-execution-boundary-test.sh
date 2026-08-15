#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
BOUNDARY_TEMP=
BOUNDARY_BINARY=
BOUNDARY_FAILURES=0

cleanup() {
  if [ -n "$BOUNDARY_TEMP" ] && [ -d "$BOUNDARY_TEMP" ]; then
    rm -rf -- "$BOUNDARY_TEMP"
  fi
}

fail_match() {
  description=$1
  matches=$2
  printf 'FAIL: %s\n%s\n' "$description" "$matches" >&2
  BOUNDARY_FAILURES=$((BOUNDARY_FAILURES + 1))
}

scan_fixed_text() {
  forbidden_text=$1
  matches=

  set +e
  matches=$(grep -RInF --include='*.go' --exclude='*_test.go' -- \
    "$forbidden_text" "$REPOSITORY/internal" "$REPOSITORY/cmd")
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    fail_match "production Go contains forbidden execution text '$forbidden_text'" "$matches"
  elif [ "$status" -ne 1 ]; then
    fail_match "production Go scan failed for '$forbidden_text'" "$matches"
  fi

  set +e
  matches=$(grep -RInF -- "$forbidden_text" "$REPOSITORY/internal/assets")
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    fail_match "active assets contain forbidden execution text '$forbidden_text'" "$matches"
  elif [ "$status" -ne 1 ]; then
    fail_match "active asset scan failed for '$forbidden_text'" "$matches"
  fi
}

scan_host_process_boundary() {
  matches=

  if [ ! -e "$REPOSITORY/internal/host" ]; then
    return
  fi

  set +e
  matches=$(grep -RInF --include='*.go' --exclude='*_test.go' -- \
    '"os/exec"' "$REPOSITORY/internal/host")
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    fail_match 'production Host code imports os/exec' "$matches"
  elif [ "$status" -ne 1 ]; then
    fail_match 'Host os/exec scan failed' "$matches"
  fi

  set +e
  matches=$(grep -REn --include='*.go' --exclude='*_test.go' -- \
    '^[[:space:]]*func[[:space:]]+\([^)]*\)[[:space:]]+Invoke[[:space:]]*\(' \
    "$REPOSITORY/internal/host")
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    fail_match 'production Host code exposes an Invoke process method' "$matches"
  elif [ "$status" -ne 1 ]; then
    fail_match 'Host Invoke scan failed' "$matches"
  fi
}

scan_cli_help() {
  BOUNDARY_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-execution-boundary.XXXXXX")
  BOUNDARY_TEMP=$(CDPATH='' cd -P -- "$BOUNDARY_TEMP" && pwd)
  BOUNDARY_BINARY=$BOUNDARY_TEMP/oaw
  help_output=$BOUNDARY_TEMP/help

  if ! (cd "$REPOSITORY" && GOCACHE="$BOUNDARY_TEMP/go-cache" go build -o "$BOUNDARY_BINARY" ./cmd/oaw); then
    fail_match 'could not build oaw for the CLI help boundary' 'go build ./cmd/oaw failed'
    return
  fi
  if ! "$BOUNDARY_BINARY" --help >"$help_output" 2>&1; then
    fail_match 'oaw --help failed during the CLI boundary check' "$(cat "$help_output")"
    return
  fi

  if ! grep -F -- 'oaw profile list' "$help_output" >/dev/null; then
    fail_match 'CLI help omits advisory Profile inspection' "$(cat "$help_output")"
  fi

  for forbidden_text in \
    'codex exec' \
    'claude --' \
    'oaw/codex-runner' \
    'runner-managed' \
    'native-managed' \
    'private HOME' \
    'ignore-user-config' \
    'ignore-rules' \
    'disable hooks' \
    'isolated-executor' \
    'native-invocation' \
    'oaw profiles' \
    'oaw use' \
    'oaw status' \
    'oaw workflow' \
    'oaw providers' \
    'oaw policy' \
    'oaw catalog' \
    'oaw bridge' \
    'oaw run' \
    'oaw runtime' \
    '--host codex' \
    '--sandbox' \
    'execution-root'; do
    if grep -F -- "$forbidden_text" "$help_output" >/dev/null; then
      fail_match "CLI help contains forbidden execution text '$forbidden_text'" \
        "$(grep -nF -- "$forbidden_text" "$help_output")"
    fi
  done
}

assert_removed_commands_are_inert() {
  state_root=$BOUNDARY_TEMP/removed-state
  mkdir -p "$state_root"
  for command_name in \
    profiles use status complete review approve satisfy incident switch stop uncertain \
    workflow providers policy catalog bridge runtime run; do
    set +e
    HOME="$BOUNDARY_TEMP/removed-home" XDG_STATE_HOME="$state_root" \
      "$BOUNDARY_BINARY" "$command_name" >/dev/null 2>&1
    status=$?
    set -e
    if [ "$status" -ne 64 ]; then
      fail_match "removed $command_name command exited $status instead of 64" "$command_name"
    fi
  done
  if [ -n "$(find "$state_root" -mindepth 1 -print -quit)" ]; then
    fail_match 'removed execution commands created state' "$(find "$state_root" -mindepth 1 -print)"
  fi
}

run_trapped_command() {
  expected_status=$1
  shift
  set +e
  HOME="$BOUNDARY_TEMP/trap-home" \
    XDG_CONFIG_HOME="$BOUNDARY_TEMP/trap-config" \
    XDG_STATE_HOME="$BOUNDARY_TEMP/trap-state" \
    PATH="$BOUNDARY_TEMP/trap-bin:$PATH" \
    OAW_MODEL_SENTINEL="$BOUNDARY_TEMP/model-executed" \
    "$BOUNDARY_BINARY" "$@" >/dev/null 2>&1
  status=$?
  set -e
  if [ "$status" -ne "$expected_status" ]; then
    fail_match "trapped command '$*' exited $status instead of $expected_status" "$*"
  fi
}

assert_public_commands_do_not_launch_models() {
  mkdir -p "$BOUNDARY_TEMP/trap-bin" "$BOUNDARY_TEMP/trap-home" \
    "$BOUNDARY_TEMP/trap-config" "$BOUNDARY_TEMP/trap-state"
  for model_command in codex claude gemini opencode; do
    {
      printf '%s\n' '#!/usr/bin/env bash'
      printf '%s\n' 'printf "%s\n" "$0" >>"$OAW_MODEL_SENTINEL"'
      printf '%s\n' 'exit 99'
    } >"$BOUNDARY_TEMP/trap-bin/$model_command"
    chmod 755 "$BOUNDARY_TEMP/trap-bin/$model_command"
  done

  run_trapped_command 0 --help
  run_trapped_command 0 check --target codex
  run_trapped_command 0 install --target codex --dry-run
  run_trapped_command 66 update --target codex --dry-run
  run_trapped_command 0 uninstall --target codex --dry-run
  run_trapped_command 0 profile list
  run_trapped_command 0 profile show built-in:SP-FULL
  run_trapped_command 0 profile check built-in:SP-FULL
  run_trapped_command 64 profiles
  run_trapped_command 64 use
  run_trapped_command 64 status
  run_trapped_command 64 workflow
  run_trapped_command 64 providers
  run_trapped_command 64 policy
  run_trapped_command 64 catalog
  run_trapped_command 64 bridge
  run_trapped_command 64 run --host codex
  run_trapped_command 64 runtime exchange

  if [ -e "$BOUNDARY_TEMP/model-executed" ]; then
    fail_match 'a public OAW command executed a model CLI' "$(cat "$BOUNDARY_TEMP/model-executed")"
  fi
}

trap cleanup EXIT HUP INT TERM

for forbidden_text in \
  'codex exec' \
  'claude --' \
  'oaw/codex-runner' \
  'runner-managed' \
  'native-managed' \
  'private HOME' \
  'ignore-user-config' \
  'ignore-rules' \
  'disable hooks' \
  'isolated-executor' \
  'native-invocation'; do
  scan_fixed_text "$forbidden_text"
done

scan_host_process_boundary
scan_cli_help
if [ -n "$BOUNDARY_BINARY" ]; then
  assert_removed_commands_are_inert
  assert_public_commands_do_not_launch_models
fi

if [ "$BOUNDARY_FAILURES" -ne 0 ]; then
  printf 'FAIL: found %s host execution boundary violation(s)\n' "$BOUNDARY_FAILURES" >&2
  exit 1
fi

printf 'PASS: OAW owns no model execution boundary\n'
