#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)
COVERAGE_TEMP=

cleanup() {
  if [ -n "$COVERAGE_TEMP" ] && [ -d "$COVERAGE_TEMP" ]; then
    rm -rf -- "$COVERAGE_TEMP"
  fi
}

fail() {
  printf 'coverage: error: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT HUP INT TERM

COVERAGE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-core-coverage.XXXXXX") ||
  fail "cannot create temporary directory"
COVERAGE_PROFILE=$COVERAGE_TEMP/coverage.out
COVERAGE_REPORT=$COVERAGE_TEMP/functions.txt

PACKAGES='./internal/admission ./internal/core ./internal/coordinator ./internal/execution ./internal/host ./internal/profile ./internal/registry'
COVER_PACKAGES='./internal/admission,./internal/core,./internal/coordinator,./internal/execution,./internal/host,./internal/profile,./internal/registry'

(cd "$REPOSITORY" && go test -covermode=atomic -coverpkg="$COVER_PACKAGES" -coverprofile="$COVERAGE_PROFILE" $PACKAGES) ||
  fail "target package tests failed"
(cd "$REPOSITORY" && go tool cover -func="$COVERAGE_PROFILE") >"$COVERAGE_REPORT" ||
  fail "cannot produce function coverage report"

COVERAGE_TOTAL=$(awk '$1 == "total:" { value = $3; sub(/%$/, "", value); print value }' "$COVERAGE_REPORT")
[ -n "$COVERAGE_TOTAL" ] || fail "coverage report has no total row"

if ! awk -v total="$COVERAGE_TOTAL" 'BEGIN { exit !(total + 0 >= 80.0) }'; then
  cat "$COVERAGE_REPORT" >&2
  fail "aggregate statement coverage is ${COVERAGE_TOTAL}%, want at least 80.0%"
fi

printf 'coverage: core coordinator aggregate %.1f%%\n' "$COVERAGE_TOTAL"
