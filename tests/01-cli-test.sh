#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
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
for profile_command in 'oaw profile list' 'oaw profile show' 'oaw profile check'; do
  assert_contains "$profile_command" "top-level help lists $profile_command"
done
pass "help is inert"

for removed_command in \
  profiles use status complete review approve satisfy incident switch stop uncertain \
  workflow providers policy catalog bridge runtime run; do
  run_oaw "$removed_command"
  assert_status 64 "$removed_command command is removed"
  assert_contains "unknown command '$removed_command'" "$removed_command is absent from the command surface"
  assert_read_only_roots
done
pass "machine-shaped commands are absent without state"

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
assert_status 0 "install accepts mutating options"
assert_contains "would-create" "install dry run reports planned creation"
assert_read_only_roots

run_oaw update --target=codex
assert_status 66 "the equals target form parses"
assert_contains "no installation state; run install first" "supported update without state is explicit"
assert_read_only_roots

run_oaw uninstall --project "$OAW_PROJECT"
assert_status 0 "a project value containing no shell expansion parses"
assert_contains "unchanged: copilot" "empty project uninstall is idempotent"
assert_read_only_roots
assert_empty_dir "$OAW_PROJECT" "empty project uninstall must not mutate the project"
pass "mutating command lines parse without mutation"

run_oaw check --help
assert_status 0 "command-scoped help exits successfully"
assert_contains "Usage: ./install.sh <command> [options]" "command-scoped help shows usage"
assert_read_only_roots
pass "command-scoped help is inert"

run_oaw check
assert_status 0 "check accepts the user-scope defaults"
assert_contains "scope: user" "check reports user scope"
assert_contains "targets: claude,codex,gemini,opencode" "user defaults are the four user targets"
assert_read_only_roots

run_oaw check --target opencode,claude,codex,claude,gemini
assert_status 0 "check accepts an explicit user target selection"
assert_contains "targets: claude,codex,gemini,opencode" "user targets are deduplicated in registry order"
assert_read_only_roots
pass "user target selection is deterministic"

OAW_PROJECT_WITH_SPACES=$OAW_SANDBOX/real\ project
OAW_PROJECT_LINK=$OAW_SANDBOX/project\ link
mkdir -p "$OAW_PROJECT_WITH_SPACES"
ln -s "$OAW_PROJECT_WITH_SPACES" "$OAW_PROJECT_LINK"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT_WITH_SPACES" && pwd -P)

run_oaw check --project="$OAW_PROJECT_LINK"
assert_status 0 "check accepts a project path containing spaces"
assert_contains "scope: project ($OAW_PROJECT_PHYSICAL)" "project scope uses the physical path"
assert_contains "targets: claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot" "project defaults include all targets in registry order"
assert_read_only_roots

run_oaw check --project "$OAW_PROJECT_WITH_SPACES" --target=copilot,roo,cline,windsurf,cursor,opencode,gemini,codex,claude,copilot
assert_status 0 "check accepts every registered project target"
assert_contains "targets: claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot" "explicit project targets normalize in registry order"
assert_read_only_roots
pass "project target selection is deterministic"

for invalid_targets in \
  ',claude' \
  'claude,' \
  'claude,,codex'
do
  assert_cli_error "target selection contains an empty member" "empty target member '$invalid_targets'" check --target="$invalid_targets"
done

assert_cli_error "target selection must not contain whitespace" "space in target selection" check --target='claude, codex'
assert_cli_error "target selection must not contain whitespace" "tab in target selection" check --target="claude	codex"
assert_cli_error "unknown target 'vscode'" "unknown target" check --target=claude,vscode

for extension_target in cursor windsurf cline roo copilot; do
  assert_cli_error "target '$extension_target' does not support user scope" "user-scope $extension_target" check --target="$extension_target"
done
pass "invalid target selections fail before mutation"

assert_cli_error "project directory does not exist" "missing project directory" check --project "$OAW_SANDBOX/missing project"

OAW_NOT_A_DIRECTORY=$OAW_SANDBOX/not-a-directory
: >"$OAW_NOT_A_DIRECTORY"
assert_cli_error "project directory does not exist" "project path is not a directory" check --project "$OAW_NOT_A_DIRECTORY"

OAW_CONTROL_PROJECT=$(printf '%s\nbad' "$OAW_PROJECT")
assert_cli_error "project path contains control characters" "control character in project path" check --project "$OAW_CONTROL_PROJECT"
pass "invalid project scopes fail before mutation"
