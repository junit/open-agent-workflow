#!/usr/bin/env bash

set -eu

OAW_RELEASE_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
OAW_EXECUTABLE=$OAW_RELEASE_DIR/oaw

if [ ! -f "$OAW_EXECUTABLE" ]; then
  OAW_EXECUTABLE=$OAW_RELEASE_DIR/oaw.exe
fi

if [ ! -f "$OAW_EXECUTABLE" ] || [ ! -x "$OAW_EXECUTABLE" ]; then
  printf 'oaw: error: precompiled sibling binary is missing or not executable\n' >&2
  exit 70
fi

exec "$OAW_EXECUTABLE" "$@"
