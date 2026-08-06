#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)

fail() {
  printf 'dogfood-current: error: %s\n' "$*" >&2
  exit 1
}

usage() {
  printf '%s\n' \
    'usage: scripts/dogfood-current.sh start <absolute-repository> <absolute-evidence-dir>' \
    '       scripts/dogfood-current.sh prepare <absolute-evidence-dir>' \
    '       scripts/dogfood-current.sh inspect <absolute-evidence-dir>' \
    '       scripts/dogfood-current.sh receipt <absolute-evidence-dir> <absolute-receipt-json>'
}

run_helper() {
  (cd "$REPOSITORY" && go run ./cmd/oaw-dogfood "$@")
}

[ "$#" -gt 0 ] || {
  usage >&2
  exit 64
}

case "$1" in
  start)
    [ "$#" -eq 3 ] || {
      usage >&2
      exit 64
    }
    [ -n "${OAW_DOGFOOD_APPROVED_REPOSITORY:-}" ] || fail "OAW_DOGFOOD_APPROVED_REPOSITORY is required"
    [ -n "${OAW_HOST_SESSION_ID:-}" ] || fail "OAW_HOST_SESSION_ID is required"
    run_helper start "$2" "$3" "$OAW_DOGFOOD_APPROVED_REPOSITORY" "$OAW_HOST_SESSION_ID"
    ;;
  prepare)
    [ "$#" -eq 2 ] || {
      usage >&2
      exit 64
    }
    run_helper prepare "$2"
    ;;
  inspect)
    [ "$#" -eq 2 ] || {
      usage >&2
      exit 64
    }
    run_helper inspect "$2"
    ;;
  receipt)
    [ "$#" -eq 3 ] || {
      usage >&2
      exit 64
    }
    run_helper receipt "$2" "$3"
    ;;
  *)
    usage >&2
    exit 64
    ;;
esac
