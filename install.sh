#!/usr/bin/env bash

set -eu

OAW_SOURCE_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)

usage() {
  printf '%s\n' \
    'Usage: ./install.sh <command> [options]' \
    '' \
    'Commands:' \
    '  check       Report provider and target readiness' \
    '  install     Install selected workflow adapters' \
    '  update      Update selected workflow adapters' \
    '  uninstall   Remove selected workflow adapters' \
    '' \
    'Options:' \
    '  --target <ids>   Select comma-separated targets' \
    '  --project <path> Use project scope at an existing path' \
    '  --dry-run        Preview a mutating command' \
    '  --force          Override recoverable drift checks' \
    '  -h, --help       Show this help'
}

. "$OAW_SOURCE_DIR/lib/common.sh"
. "$OAW_SOURCE_DIR/lib/cli.sh"
. "$OAW_SOURCE_DIR/lib/targets.sh"
. "$OAW_SOURCE_DIR/lib/detect.sh"
. "$OAW_SOURCE_DIR/lib/paths.sh"
. "$OAW_SOURCE_DIR/lib/checksum.sh"
. "$OAW_SOURCE_DIR/lib/render.sh"
. "$OAW_SOURCE_DIR/lib/managed.sh"
. "$OAW_SOURCE_DIR/lib/state.sh"
. "$OAW_SOURCE_DIR/lib/filesystem.sh"
. "$OAW_SOURCE_DIR/lib/operations.sh"
. "$OAW_SOURCE_DIR/lib/commands/check.sh"
. "$OAW_SOURCE_DIR/lib/commands/mutate.sh"

if [ "$#" -eq 0 ]; then
  usage
  exit 0
fi

case "$1" in
  help|-h|--help)
    if [ "$#" -eq 1 ]; then
      usage
      exit 0
    fi
    ;;
esac

parse_cli "$@"

if [ "$OAW_HELP" -eq 1 ]; then
  usage
  exit 0
fi

resolve_scope_and_targets

case "$OAW_COMMAND" in
  check)
    command_check
    ;;
  install|update|uninstall)
    command_mutate
    ;;
esac
