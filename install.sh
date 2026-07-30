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

case "${1:-}" in
  ''|help|-h|--help)
    usage
    exit 0
    ;;
  *)
    usage
    exit 0
    ;;
esac
