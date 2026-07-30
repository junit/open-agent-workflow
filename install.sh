#!/usr/bin/env bash

set -eu

OAW_SOURCE_DIR=$(CDPATH= cd -P -- "$(dirname -- "$0")" && pwd)

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

case "$OAW_COMMAND" in
  check|install|update|uninstall)
    die "command not implemented: $OAW_COMMAND" 69
    ;;
esac
