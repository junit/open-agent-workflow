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

require_literal() {
  path=$1
  literal=$2
  grep -F -- "$literal" "$REPOSITORY/$path" >/dev/null ||
    fail "$path does not expose required cutover constant $literal"
}

trap cleanup EXIT HUP INT TERM

require_literal internal/profile/records.go 'ExecutionGraphSchemaV4 = "oaw.execution-graph/v4"'
require_literal internal/core/records.go 'LifecycleBundleSchemaV4 = "oaw.lifecycle-bundle/v4"'
require_literal internal/coordinator/records.go 'WorkflowCommandSchemaV2  = "oaw.workflow-command/v2"'
require_literal internal/coordinator/records.go 'WorkflowRevisionSchemaV2 = "oaw.workflow-revision/v2"'
require_literal internal/coordinator/records.go 'DispatchPacketSchemaV2   = "oaw.dispatch-packet/v2"'

LEGACY_CORE_COORDINATOR='oaw\.execution-graph/v3|oaw\.lifecycle-bundle/v3|oaw\.capability-grant/v2|oaw\.dispatch-packet/v1|oaw\.workflow-(command|result|snapshot|revision)/v1'
if (cd "$REPOSITORY" && rg -n "$LEGACY_CORE_COORDINATOR" \
  internal/core internal/profile internal/coordinator --glob '!**/*_test.go'); then
  fail 'Core or Coordinator production code contains superseded authority'
fi

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

printf 'coverage: Bundle/Graph v4 and Workflow v2 aggregate %.1f%%\n' "$COVERAGE_TOTAL"
