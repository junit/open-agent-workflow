#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)

if [ "$#" -ne 0 ]; then
  printf '%s\n' 'usage: scripts/smoke-host-native.sh' >&2
  exit 64
fi

if [ -z "${OAW_HOST_NATIVE_TRANSCRIPT:-}" ] || [ ! -r "$OAW_HOST_NATIVE_TRANSCRIPT" ] || [ ! -f "$OAW_HOST_NATIVE_TRANSCRIPT" ]; then
  printf '%s\n' 'SKIP: Host-native SUBAGENT transcript unavailable' >&2
  exit 77
fi

cd "$REPOSITORY"
exec go test ./internal/integration -run '^TestExternalHostNativeTranscript$' -count=1
