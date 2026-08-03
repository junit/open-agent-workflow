#!/usr/bin/env bash

set -eu

if [ ! -r /proc/sys/kernel/osrelease ] ||
  ! grep -qi microsoft /proc/sys/kernel/osrelease; then
  printf 'SKIP: WSL smoke requires an actual Microsoft WSL kernel\n' >&2
  exit 77
fi

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
exec bash "$SCRIPT_DIR/smoke-linux.sh" "$@"
