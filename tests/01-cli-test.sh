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

assert_cli_error() {
  expected_message=$1
  description=$2
  shift 2

  run_oaw "$@"
  assert_status 64 "$description exits with a command-line usage error"
  assert_contains "oaw: error:" "$description uses the error prefix"
  assert_contains "$expected_message" "$description explains the invalid input"
  assert_read_only_roots
}

for parser_case in \
  unknown-command \
  unknown-check-option \
  missing-target \
  empty-target \
  duplicate-target \
  missing-project \
  empty-project \
  duplicate-project \
  check-dry-run \
  check-force \
  duplicate-dry-run \
  duplicate-force
do
  case "$parser_case" in
    unknown-command)
      assert_cli_error "unknown command 'unknown'" "$parser_case" unknown
      ;;
    unknown-check-option)
      assert_cli_error "unknown option '--bogus'" "$parser_case" check --bogus
      ;;
    missing-target)
      assert_cli_error "--target requires a value" "$parser_case" check --target
      ;;
    empty-target)
      assert_cli_error "--target requires a value" "$parser_case" check --target=
      ;;
    duplicate-target)
      assert_cli_error "--target may be specified only once" "$parser_case" install --target claude --target=codex
      ;;
    missing-project)
      assert_cli_error "--project requires a value" "$parser_case" check --project
      ;;
    empty-project)
      assert_cli_error "--project requires a value" "$parser_case" check --project=
      ;;
    duplicate-project)
      assert_cli_error "--project may be specified only once" "$parser_case" install --project "$OAW_PROJECT" --project="$OAW_PROJECT"
      ;;
    check-dry-run)
      assert_cli_error "--dry-run is not valid for check" "$parser_case" check --dry-run
      ;;
    check-force)
      assert_cli_error "--force is not valid for check" "$parser_case" check --force
      ;;
    duplicate-dry-run)
      assert_cli_error "--dry-run may be specified only once" "$parser_case" install --dry-run --dry-run
      ;;
    duplicate-force)
      assert_cli_error "--force may be specified only once" "$parser_case" install --force --force
      ;;
  esac
done
pass "invalid command lines fail before mutation"

run_oaw install --target claude --dry-run --force
assert_status 69 "install accepts mutating options until its implementation lands"
assert_contains "command not implemented: install" "install reports its temporary status"
assert_read_only_roots

run_oaw update --target=codex
assert_status 69 "the equals target form parses"
assert_contains "command not implemented: update" "update reports its temporary status"
assert_read_only_roots

run_oaw uninstall --project "$OAW_PROJECT"
assert_status 69 "a project value containing no shell expansion parses"
assert_contains "command not implemented: uninstall" "uninstall reports its temporary status"
assert_read_only_roots
pass "mutating command lines parse without mutation"

run_oaw check --help
assert_status 0 "command-scoped help exits successfully"
assert_contains "Usage: ./install.sh <command> [options]" "command-scoped help shows usage"
assert_read_only_roots
pass "command-scoped help is inert"
