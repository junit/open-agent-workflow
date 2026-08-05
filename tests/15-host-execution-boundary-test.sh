#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
BOUNDARY_TEMP=
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
  binary=$BOUNDARY_TEMP/oaw
  help_output=$BOUNDARY_TEMP/help

  if ! (cd "$REPOSITORY" && GOCACHE="$BOUNDARY_TEMP/go-cache" go build -o "$binary" ./cmd/oaw); then
    fail_match 'could not build oaw for the CLI help boundary' 'go build ./cmd/oaw failed'
    return
  fi
  if ! "$binary" --help >"$help_output" 2>&1; then
    fail_match 'oaw --help failed during the CLI boundary check' "$(cat "$help_output")"
    return
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
    'native-invocation'; do
    if grep -F -- "$forbidden_text" "$help_output" >/dev/null; then
      fail_match "CLI help contains forbidden execution text '$forbidden_text'" \
        "$(grep -nF -- "$forbidden_text" "$help_output")"
    fi
  done
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

if [ "$BOUNDARY_FAILURES" -ne 0 ]; then
  printf 'FAIL: found %s host execution boundary violation(s)\n' "$BOUNDARY_FAILURES" >&2
  exit 1
fi

printf 'PASS: OAW owns no model execution boundary\n'
