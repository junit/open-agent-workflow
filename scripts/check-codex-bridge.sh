#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)
BRIDGE_COVERAGE_TEMP=

cleanup() {
  if [ -n "$BRIDGE_COVERAGE_TEMP" ] && [ -d "$BRIDGE_COVERAGE_TEMP" ]; then
    rm -rf -- "$BRIDGE_COVERAGE_TEMP"
  fi
}

fail() {
  printf 'Codex Bridge check: error: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT HUP INT TERM

cd "$REPOSITORY"
go test ./internal/codexbridge/... ./internal/cli ./internal/dogfood ./internal/hosttest ./internal/integration ./internal/assets ./internal/assets/generate ./internal/builtin
go test -race ./internal/codexbridge/... ./internal/cli ./internal/integration

BRIDGE_COVERAGE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-bridge-coverage.XXXXXX") ||
  fail 'cannot create Bridge coverage directory'
BRIDGE_COVERAGE_PROFILE=$BRIDGE_COVERAGE_TEMP/coverage.out
BRIDGE_COVERAGE_REPORT=$BRIDGE_COVERAGE_TEMP/functions.txt
go test -covermode=atomic -coverpkg='./internal/codexbridge/...' \
  -coverprofile="$BRIDGE_COVERAGE_PROFILE" ./internal/codexbridge/... ./internal/integration ||
  fail 'Bridge coverage tests failed'
go tool cover -func="$BRIDGE_COVERAGE_PROFILE" >"$BRIDGE_COVERAGE_REPORT" ||
  fail 'cannot produce Bridge coverage report'
BRIDGE_COVERAGE_TOTAL=$(awk '$1 == "total:" { value = $3; sub(/%$/, "", value); print value }' "$BRIDGE_COVERAGE_REPORT")
[ -n "$BRIDGE_COVERAGE_TOTAL" ] || fail 'Bridge coverage report has no total row'
if ! awk -v total="$BRIDGE_COVERAGE_TOTAL" 'BEGIN { exit !(total + 0 >= 80.0) }'; then
  cat "$BRIDGE_COVERAGE_REPORT" >&2
  fail "aggregate statement coverage is ${BRIDGE_COVERAGE_TOTAL}%, want at least 80.0%"
fi

go vet ./internal/codexbridge/... ./internal/cli ./internal/dogfood ./internal/hosttest ./internal/integration ./internal/assets ./internal/assets/generate ./internal/builtin
bash tests/18-codex-bridge-protocol-test.sh

FORBIDDEN='codex exec|thread/start|thread/resume|thread/fork|turn/start|turn/steer|plugin/list|private.*HOME|projected.*config|NATIVE_SUBAGENT|INLINE|oaw/codex-runner'
if rg -n "$FORBIDDEN" --glob '*.go' --glob '!**/*_test.go' \
  internal/codexbridge internal/cli; then
  fail 'production Go source contains a forbidden process, metadata, environment, or legacy-topology path'
fi
if rg -n "$FORBIDDEN" --glob '*.json' --glob '*.md' \
  internal/codexbridge/install/assets; then
  fail 'installed Bridge assets contain a forbidden process, metadata, environment, or legacy-topology path'
fi

LEGACY_AUTHORITY='oaw\.provider-descriptor/v3|oaw\.profile-recipe/v2|oaw\.provider-instance/v3|oaw\.provider-resolution-report/v3|oaw\.effective-registry/v3|oaw\.host-manifest/v2|oaw\.host-session/v2|oaw\.host-binding-inventory/v2|oaw\.host-invocation-receipt/v2|oaw\.host-conformance-(transcript|report)/v[23]|oaw\.execution-graph/v3|oaw\.lifecycle-bundle/v3|oaw\.capability-grant/v2|oaw\.dispatch-packet/v1|oaw\.workflow-(command|result|snapshot|revision)/v1|oaw\.codex-bridge/v1|oaw\.codex-hook-context/v1|oaw\.host-evidence-handle/v1'
if rg -n "$LEGACY_AUTHORITY" cmd internal \
  --glob '!**/*_test.go' \
  --glob '!**/testdata/**' \
  --glob '!internal/assets/audits/**'; then
  fail 'production authority surface contains a superseded schema identifier'
fi

printf 'coverage: Codex Bridge aggregate %.1f%%\n' "$BRIDGE_COVERAGE_TOTAL"
printf 'PASS: deterministic Codex Bridge gate passed\n'
