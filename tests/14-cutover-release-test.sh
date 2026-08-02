#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
CUTOVER_TEMP=

cleanup() {
  if [ -n "$CUTOVER_TEMP" ] && [ -d "$CUTOVER_TEMP" ]; then
    rm -rf -- "$CUTOVER_TEMP"
  fi
}

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'PASS: %s\n' "$*"
}

run_entrypoint() {
  entrypoint=$1
  output_prefix=$2
  shift 2
  set +e
  HOME="$CUTOVER_TEMP/home" \
    XDG_CONFIG_HOME="$CUTOVER_TEMP/config" \
    XDG_STATE_HOME="$CUTOVER_TEMP/state" \
    PATH="$CUTOVER_TEMP/bin:$PATH" \
    "$entrypoint" "$@" \
    >"$output_prefix.stdout" 2>"$output_prefix.stderr"
  ENTRYPOINT_STATUS=$?
  set -e
}

assert_entrypoint_help() {
  entrypoint=$1
  output_prefix=$2
  run_entrypoint "$entrypoint" "$output_prefix" --help
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "$entrypoint help exited $ENTRYPOINT_STATUS: $(cat "$output_prefix.stderr")"
  grep -F 'Usage: ./install.sh <command> [options]' "$output_prefix.stdout" >/dev/null ||
    fail "$entrypoint help omitted compatibility usage"
  [ ! -s "$output_prefix.stderr" ] ||
    fail "$entrypoint help wrote stderr: $(cat "$output_prefix.stderr")"
}

run_wrapper_contract() {
  if grep -E '^[[:space:]]*(source|\.)[[:space:]]' "$REPOSITORY/install.sh" >/dev/null; then
    fail "install.sh still loads the Bash management implementation"
  fi
  if grep -E '(^|[;&|[:space:]])(curl|wget)([[:space:]]|$)|git[[:space:]]+clone|go[[:space:]]+run|command[[:space:]]+-v[[:space:]]+oaw' \
    "$REPOSITORY/install.sh" >/dev/null; then
    fail "install.sh downloads, builds, or searches PATH for executable code"
  fi

  release_dir=$CUTOVER_TEMP/release
  mkdir -p "$release_dir" "$CUTOVER_TEMP/home" "$CUTOVER_TEMP/config" \
    "$CUTOVER_TEMP/state" "$CUTOVER_TEMP/bin"
  cp "$REPOSITORY/install.sh" "$release_dir/install.sh"
  chmod 755 "$release_dir/install.sh"
  (cd "$REPOSITORY" && go build -o "$release_dir/oaw" ./cmd/oaw)

  assert_entrypoint_help "$release_dir/oaw" "$CUTOVER_TEMP/direct-help"
  assert_entrypoint_help "$release_dir/install.sh" "$CUTOVER_TEMP/wrapper-help"
  cmp -s "$CUTOVER_TEMP/direct-help.stdout" "$CUTOVER_TEMP/wrapper-help.stdout" ||
    fail "wrapper help differs from the colocated binary"

  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/check" check --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper check exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/check.stderr")"
  grep -F 'installed claude: not-installed' "$CUTOVER_TEMP/check.stdout" >/dev/null ||
    fail "wrapper did not forward check arguments"
  [ ! -e "$CUTOVER_TEMP/state/open-agent-workflow/runtime" ] ||
    fail "wrapper check created Runtime State"

  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/install" install --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper install exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/install.stderr")"
  [ -f "$CUTOVER_TEMP/state/open-agent-workflow/installations/user.state" ] ||
    fail "wrapper install did not create Install State"
  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/update" update --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper update exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/update.stderr")"
  grep -F 'oaw: unchanged: claude' "$CUTOVER_TEMP/update.stdout" >/dev/null ||
    fail "wrapper did not forward update arguments"
  run_entrypoint "$release_dir/install.sh" "$CUTOVER_TEMP/uninstall" uninstall --target claude
  [ "$ENTRYPOINT_STATUS" -eq 0 ] ||
    fail "wrapper uninstall exited $ENTRYPOINT_STATUS: $(cat "$CUTOVER_TEMP/uninstall.stderr")"
  [ ! -e "$CUTOVER_TEMP/state/open-agent-workflow/installations/user.state" ] ||
    fail "wrapper uninstall left Install State"

  missing_dir=$CUTOVER_TEMP/missing
  mkdir -p "$missing_dir"
  cp "$REPOSITORY/install.sh" "$missing_dir/install.sh"
  chmod 755 "$missing_dir/install.sh"
  printf '%s\n' \
    '#!/usr/bin/env bash' \
    'touch "$PATH_EXECUTED_SENTINEL"' \
    >"$CUTOVER_TEMP/bin/oaw"
  chmod 755 "$CUTOVER_TEMP/bin/oaw"
  PATH_EXECUTED_SENTINEL=$CUTOVER_TEMP/path-oaw-executed
  export PATH_EXECUTED_SENTINEL
  run_entrypoint "$missing_dir/install.sh" "$CUTOVER_TEMP/missing" --help
  [ "$ENTRYPOINT_STATUS" -eq 70 ] ||
    fail "missing sibling binary exited $ENTRYPOINT_STATUS instead of 70"
  [ ! -e "$PATH_EXECUTED_SENTINEL" ] ||
    fail "wrapper executed an oaw binary from PATH"

  cp "$release_dir/oaw" "$missing_dir/oaw"
  chmod 644 "$missing_dir/oaw"
  run_entrypoint "$missing_dir/install.sh" "$CUTOVER_TEMP/non-executable" --help
  [ "$ENTRYPOINT_STATUS" -eq 70 ] ||
    fail "non-executable sibling exited $ENTRYPOINT_STATUS instead of 70"

  rm -f "$missing_dir/oaw"
  cp "$release_dir/oaw" "$missing_dir/oaw.exe"
  chmod 755 "$missing_dir/oaw.exe"
  assert_entrypoint_help "$missing_dir/install.sh" "$CUTOVER_TEMP/exe-help"

  pass "install.sh is an offline colocated-binary compatibility wrapper"
}

trap cleanup EXIT HUP INT TERM
CUTOVER_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-cutover.XXXXXX") ||
  fail "cannot create cutover test directory"

case "${1:-all}" in
  all|wrapper) run_wrapper_contract ;;
  release) fail "release archive test is not implemented" ;;
  *) fail "unknown cutover test mode: $1" ;;
esac
