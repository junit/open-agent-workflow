#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH= cd -P -- "$(dirname -- "$0")" && pwd)
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM
setup_sandbox

run_oaw
assert_status 0 "no arguments print usage"
assert_contains "Usage: ./install.sh <command> [options]" "no arguments show the usage header"
assert_read_only_roots
pass "no arguments are inert"

for help_argument in help -h --help; do
  run_oaw "$help_argument"
  assert_status 0 "$help_argument exits successfully"
  assert_contains "Usage: ./install.sh <command> [options]" "$help_argument shows the usage header"
  assert_read_only_roots
done

for usage_item in check install update uninstall --target --project --dry-run --force --help; do
  assert_contains "$usage_item" "usage lists $usage_item"
done
pass "help is inert"
