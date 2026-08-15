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
go test ./cmd/oaw-bridge ./internal/assurance ./internal/bridgecli ./internal/codexbridge/... ./internal/profileinspect ./internal/cli ./internal/integration ./internal/assets ./internal/assets/generate
go test -race ./internal/assurance ./internal/bridgecli ./internal/codexbridge/... ./internal/profileinspect ./internal/cli ./internal/integration

BRIDGE_COVERAGE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-bridge-coverage.XXXXXX") ||
  fail 'cannot create Bridge coverage directory'
BRIDGE_COVERAGE_PROFILE=$BRIDGE_COVERAGE_TEMP/coverage.out
BRIDGE_COVERAGE_REPORT=$BRIDGE_COVERAGE_TEMP/functions.txt
go test -covermode=atomic -coverpkg='./internal/codexbridge/...' \
  -coverprofile="$BRIDGE_COVERAGE_PROFILE" ./internal/assurance ./internal/bridgecli ./internal/codexbridge/... ./internal/profileinspect ./internal/integration ||
  fail 'Bridge coverage tests failed'
go tool cover -func="$BRIDGE_COVERAGE_PROFILE" >"$BRIDGE_COVERAGE_REPORT" ||
  fail 'cannot produce Bridge coverage report'
BRIDGE_COVERAGE_TOTAL=$(awk '$1 == "total:" { value = $3; sub(/%$/, "", value); print value }' "$BRIDGE_COVERAGE_REPORT")
[ -n "$BRIDGE_COVERAGE_TOTAL" ] || fail 'Bridge coverage report has no total row'
if ! awk -v total="$BRIDGE_COVERAGE_TOTAL" 'BEGIN { exit !(total + 0 >= 80.0) }'; then
  cat "$BRIDGE_COVERAGE_REPORT" >&2
  fail "aggregate statement coverage is ${BRIDGE_COVERAGE_TOTAL}%, want at least 80.0%"
fi

go vet ./cmd/oaw-bridge ./internal/assurance ./internal/bridgecli ./internal/codexbridge/... ./internal/profileinspect ./internal/cli ./internal/integration ./internal/assets ./internal/assets/generate
bash tests/17-codex-bridge-management-test.sh
bash tests/18-codex-bridge-protocol-test.sh

FORBIDDEN='codex exec|thread/start|thread/resume|thread/fork|turn/start|turn/steer|hooks/list|config/read|private.*HOME|projected.*config|NATIVE_SUBAGENT|INLINE|oaw/codex-runner'
if rg -n "$FORBIDDEN" --glob '*.go' --glob '!**/*_test.go' \
	cmd/oaw-bridge internal/bridgecli internal/codexbridge; then
  fail 'production Go source contains a forbidden process, metadata, environment, or legacy-topology path'
fi
if rg -n "$FORBIDDEN" --glob '*.json' --glob '*.md' \
  internal/codexbridge/install/assets; then
  fail 'installed Bridge assets contain a forbidden process, metadata, environment, or legacy-topology path'
fi

RETIRED_BRIDGE='observe_current|core_inspect|core_compile|workflow_exchange|HostEvidenceHandle|SubagentStart|oaw\.codex-bridge/v[12]|oaw\.codex-hook-context/v[12]|oaw\.host-evidence-handle/v[12]'
if rg -n "$RETIRED_BRIDGE" cmd/oaw-bridge internal/bridgecli internal/codexbridge \
  --glob '!**/*_test.go' \
  --glob '!**/testdata/**'; then
  fail 'standalone Bridge production surface contains retired workflow authority'
fi

printf 'coverage: Codex Bridge aggregate %.1f%%\n' "$BRIDGE_COVERAGE_TOTAL"
printf 'PASS: deterministic Codex Bridge gate passed\n'
