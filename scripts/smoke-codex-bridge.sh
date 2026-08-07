#!/usr/bin/env bash

set -eu

if [ "$(uname -s)" != "Darwin" ]; then
  printf 'SKIP: Codex Bridge macOS smoke requires Darwin\n' >&2
  exit 77
fi
command -v codex >/dev/null 2>&1 || {
  printf 'SKIP: codex CLI unavailable\n' >&2
  exit 77
}

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)
SMOKE_TEMP=

cleanup() {
  if [ -n "$SMOKE_TEMP" ] && [ -d "$SMOKE_TEMP" ]; then
    rm -rf -- "$SMOKE_TEMP"
  fi
}

fail() {
  printf 'Codex Bridge macOS smoke: error: %s\n' "$*" >&2
  exit 1
}

require_json_help() {
  label=$1
  shift
  help_output=$SMOKE_TEMP/$label.help
  codex "$@" --help >"$help_output" 2>&1 || fail "$label help is unavailable"
  grep -F -- '--json' "$help_output" >/dev/null || fail "$label help does not advertise --json"
}

trap cleanup EXIT HUP INT TERM
SMOKE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-codex-bridge-smoke.XXXXXX") ||
  fail 'cannot create temporary directory'

codex --version
codex plugin marketplace list --json >"$SMOKE_TEMP/marketplaces.json" ||
  fail 'Codex marketplace list failed'
require_json_help plugin-add plugin add
require_json_help plugin-remove plugin remove
require_json_help plugin-list plugin list
require_json_help marketplace-add plugin marketplace add
require_json_help marketplace-list plugin marketplace list
require_json_help marketplace-remove plugin marketplace remove

(cd "$REPOSITORY" && go run ./cmd/oaw bridge check codex --format json) \
  >"$SMOKE_TEMP/bridge-check.json" || fail 'Bridge management check failed'
grep -F '"current_session_loaded":false' "$SMOKE_TEMP/bridge-check.json" >/dev/null ||
  fail 'Bridge management check inferred a loaded current session'

(cd "$REPOSITORY" && bash tests/18-codex-bridge-protocol-test.sh) ||
  fail 'fake-transcript protocol smoke failed'

printf 'PASS: Codex Bridge macOS read-only smoke passed\n'
