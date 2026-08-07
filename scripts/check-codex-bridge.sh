#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)

fail() {
  printf 'Codex Bridge check: error: %s\n' "$*" >&2
  exit 1
}

cd "$REPOSITORY"
go test ./internal/codexbridge/... ./internal/integration
go test -race ./internal/codexbridge/... ./internal/integration
go vet ./internal/codexbridge/... ./internal/integration
bash tests/18-codex-bridge-protocol-test.sh
bash scripts/check-docs.sh

FORBIDDEN='codex exec|thread/start|thread/resume|thread/fork|turn/start|turn/steer|plugin/list|private.*HOME|projected.*config|NATIVE_SUBAGENT|INLINE|oaw/codex-runner'
if rg -n "$FORBIDDEN" --glob '*.go' --glob '!**/*_test.go' \
  internal/codexbridge internal/cli; then
  fail 'production Go source contains a forbidden process, metadata, environment, or legacy-topology path'
fi
if rg -n "$FORBIDDEN" --glob '*.json' --glob '*.md' \
  internal/codexbridge/install/assets; then
  fail 'installed Bridge assets contain a forbidden process, metadata, environment, or legacy-topology path'
fi

printf 'PASS: deterministic Codex Bridge gate passed\n'
