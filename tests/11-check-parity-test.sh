#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

OAW_PARITY_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-parity.XXXXXX")
OAW_GO_CHECK=$OAW_PARITY_TEMP/oaw
OAW_REAL_PATH=$PATH

cleanup_parity() {
  cleanup_sandbox
  if [ -n "${OAW_PARITY_TEMP:-}" ] && [ -d "$OAW_PARITY_TEMP" ]; then
    rm -rf "$OAW_PARITY_TEMP"
  fi
}

trap cleanup_parity EXIT HUP INT TERM

go build -o "$OAW_GO_CHECK" "$OAW_REPOSITORY/cmd/oaw"

new_parity_fixture() {
  cleanup_sandbox
  OAW_PATH=$OAW_REAL_PATH
  setup_sandbox
  OAW_TMP=$OAW_SANDBOX/tmp
  mkdir -p "$OAW_TMP"
}

run_authoritative_setup() {
  set +e
  HOME="$OAW_HOME" \
    XDG_CONFIG_HOME="$OAW_CONFIG" \
    XDG_STATE_HOME="$OAW_STATE" \
    PATH="$OAW_REAL_PATH" \
    TMPDIR="$OAW_TMP" \
    bash "$OAW_LEGACY_INSTALLER" "$@" \
    >"$OAW_PARITY_TEMP/setup.stdout" 2>"$OAW_PARITY_TEMP/setup.stderr"
  setup_status=$?
  set -e
  if [ "$setup_status" -ne 0 ]; then
    fail "authoritative fixture setup failed with $setup_status: $(cat "$OAW_PARITY_TEMP/setup.stderr")"
  fi
}

run_parity_implementation() {
  parity_implementation=$1
  shift
  parity_stdout=$OAW_PARITY_TEMP/$parity_implementation.stdout
  parity_stderr=$OAW_PARITY_TEMP/$parity_implementation.stderr
  set +e
  case "$parity_implementation" in
    bash)
      HOME="$OAW_HOME" \
        XDG_CONFIG_HOME="$OAW_CONFIG" \
        XDG_STATE_HOME="$OAW_STATE" \
        PATH="$OAW_PATH" \
        TMPDIR="$OAW_TMP" \
        bash "$OAW_LEGACY_INSTALLER" check "$@" >"$parity_stdout" 2>"$parity_stderr"
      BASH_PARITY_STATUS=$?
      ;;
    go)
      HOME="$OAW_HOME" \
        XDG_CONFIG_HOME="$OAW_CONFIG" \
        XDG_STATE_HOME="$OAW_STATE" \
        PATH="$OAW_PATH" \
        TMPDIR="$OAW_TMP" \
        "$OAW_GO_CHECK" check "$@" >"$parity_stdout" 2>"$parity_stderr"
      GO_PARITY_STATUS=$?
      ;;
    *)
      set -e
      fail "unknown parity implementation: $parity_implementation"
      ;;
  esac
  set -e
}

assert_snapshot_unchanged() {
  expected_snapshot=$1
  actual_snapshot=$2
  description=$3
  if ! cmp -s "$expected_snapshot" "$actual_snapshot"; then
    diff -u "$expected_snapshot" "$actual_snapshot" >&2 || true
    fail "$description mutated the isolated fixture"
  fi
}

run_check_pair() {
  parity_description=$1
  shift
  snapshot_tree "$OAW_SANDBOX" >"$OAW_PARITY_TEMP/before.snapshot"
  run_parity_implementation bash "$@"
  snapshot_tree "$OAW_SANDBOX" >"$OAW_PARITY_TEMP/after-bash.snapshot"
  assert_snapshot_unchanged \
    "$OAW_PARITY_TEMP/before.snapshot" "$OAW_PARITY_TEMP/after-bash.snapshot" \
    "$parity_description Bash check"
  run_parity_implementation go "$@"
  snapshot_tree "$OAW_SANDBOX" >"$OAW_PARITY_TEMP/after-go.snapshot"
  assert_snapshot_unchanged \
    "$OAW_PARITY_TEMP/after-bash.snapshot" "$OAW_PARITY_TEMP/after-go.snapshot" \
    "$parity_description Go check"

  if [ "$BASH_PARITY_STATUS" -ne "$GO_PARITY_STATUS" ]; then
    fail "$parity_description status differs: Bash=$BASH_PARITY_STATUS Go=$GO_PARITY_STATUS"
  fi
  if ! cmp -s "$OAW_PARITY_TEMP/bash.stdout" "$OAW_PARITY_TEMP/go.stdout"; then
    diff -u "$OAW_PARITY_TEMP/bash.stdout" "$OAW_PARITY_TEMP/go.stdout" >&2 || true
    fail "$parity_description stdout differs"
  fi
  if ! cmp -s "$OAW_PARITY_TEMP/bash.stderr" "$OAW_PARITY_TEMP/go.stderr"; then
    diff -u "$OAW_PARITY_TEMP/bash.stderr" "$OAW_PARITY_TEMP/go.stderr" >&2 || true
    fail "$parity_description stderr differs"
  fi
  pass "$parity_description has exact read-only Bash/Go parity"
}

assert_parity_output_contains() {
  expected_text=$1
  description=$2
  if ! grep -F "$expected_text" "$OAW_PARITY_TEMP/go.stdout" >/dev/null; then
    fail "$description missing '$expected_text' in $(cat "$OAW_PARITY_TEMP/go.stdout")"
  fi
}

make_parity_indicator() {
  indicator_path=$1
  mkdir -p "$(dirname -- "$indicator_path")"
  : >"$indicator_path"
}

new_parity_fixture
run_check_pair "empty user defaults"
assert_parity_output_contains "installed claude: not-installed" "empty user defaults"

run_check_pair "explicit target normalization" --target opencode,claude,codex,claude,gemini

for matt_skill in to-spec to-tickets tdd; do
  make_parity_indicator "$OAW_HOME/.agents/skills/$matt_skill/SKILL.md"
done
run_check_pair "partial Matt bundle" --target claude
assert_parity_output_contains "provider matt: missing" "partial Matt bundle"

new_parity_fixture
make_parity_indicator "$OAW_HOME/.codex/plugins/cache/openai-api-curated/superpowers/.hidden/skills/using-superpowers/SKILL.md"
run_check_pair "hidden Provider version directory" --target claude
assert_parity_output_contains "provider superpowers: missing" "hidden Provider version directory"

new_parity_fixture
for matt_skill in to-spec to-tickets tdd diagnosing-bugs; do
  make_parity_indicator "$OAW_HOME/.agents/skills/$matt_skill/SKILL.md"
done
make_parity_indicator "$OAW_HOME/.agents/skills/everything-claude-code/SKILL.md"
make_parity_indicator "$OAW_HOME/.codex/plugins/cache/openai-api-curated/superpowers/test-build/skills/using-superpowers/SKILL.md"
run_check_pair "complete built-in Provider diagnostics" --target claude
assert_parity_output_contains "provider superpowers: detected" "Provider diagnostics"
assert_parity_output_contains "provider matt: detected" "Provider diagnostics"
assert_parity_output_contains "provider ecc: detected" "Provider diagnostics"

new_parity_fixture
real_project=$OAW_SANDBOX/real\ project
project_link=$OAW_SANDBOX/project\ link
mkdir -p "$real_project"
ln -s "$real_project" "$project_link"
run_check_pair "physical project defaults" --project "$project_link"
assert_parity_output_contains "targets: claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot" "project defaults"

new_parity_fixture
run_authoritative_setup install --target claude
run_check_pair "clean user installation" --target claude
assert_parity_output_contains "installed claude: clean" "clean user installation"

user_state=$OAW_STATE/open-agent-workflow/installations/user.state
awk -F '\t' -v OFS='\t\t' '{$1 = $1; print}' \
  "$user_state" >"$OAW_TMP/repeated-tabs.state"
mv "$OAW_TMP/repeated-tabs.state" "$user_state"
run_check_pair "repeated TSV separators" --target claude
assert_parity_output_contains "installed claude: clean" "repeated TSV separators"

new_parity_fixture
run_authoritative_setup install --target claude
user_state=$OAW_STATE/open-agent-workflow/installations/user.state
state_without_newline=$(cat "$user_state")
printf '%s' "$state_without_newline" >"$user_state"
run_check_pair "unterminated final state record" --target claude
assert_parity_output_contains "installed claude: invalid-state" "unterminated final state record"

new_parity_fixture
run_authoritative_setup install --target claude
printf '%s\n' 'policy drift' >>"$OAW_CONFIG/open-agent-workflow/ENGINEERING.md"
run_check_pair "policy drift" --target claude
assert_parity_output_contains "installed claude: drift" "policy drift"

new_parity_fixture
run_authoritative_setup install --target claude
printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  'target drift' \
  '<!-- END OPEN AGENT WORKFLOW -->' \
  >"$OAW_HOME/.claude/CLAUDE.md"
run_check_pair "managed target drift" --target claude
assert_parity_output_contains "installed claude: drift" "managed target drift"

new_parity_fixture
run_authoritative_setup install --target claude
printf 'format\t2\n' >"$OAW_STATE/open-agent-workflow/installations/user.state"
run_check_pair "invalid Install State" --target claude
assert_parity_output_contains "installed claude: invalid-state" "invalid state"

new_parity_fixture
run_authoritative_setup install --target claude
run_check_pair "valid state without selected target" --target codex
assert_parity_output_contains "installed codex: not-installed" "untracked selection"

new_parity_fixture
mkdir -p "$OAW_HOME/.codex"
printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  'untracked' \
  '<!-- END OPEN AGENT WORKFLOW -->' \
  >"$OAW_HOME/.codex/AGENTS.md"
run_check_pair "untracked managed block" --target codex
assert_parity_output_contains "installed codex: drift" "untracked managed block"

new_parity_fixture
mkdir -p "$OAW_PROJECT/.cursor/rules"
printf '%s\n' 'untracked' >"$OAW_PROJECT/.cursor/rules/open-agent-workflow.mdc"
run_check_pair "untracked project owned file" --project "$OAW_PROJECT" --target cursor
assert_parity_output_contains "installed cursor: drift" "untracked project owned file"

new_parity_fixture
run_authoritative_setup install --project "$OAW_PROJECT" --target cursor
run_check_pair "clean project owned file" --project "$OAW_PROJECT" --target cursor
assert_parity_output_contains "installed cursor: clean" "clean project owned file"

printf '%s\n' 'owned drift' >"$OAW_PROJECT/.cursor/rules/open-agent-workflow.mdc"
run_check_pair "tracked project owned-file drift" --project "$OAW_PROJECT" --target cursor
assert_parity_output_contains "installed cursor: drift" "tracked project owned-file drift"

new_parity_fixture
run_authoritative_setup install --project "$OAW_PROJECT" --target codex,opencode
run_check_pair "shared project destination" --project "$OAW_PROJECT" --target opencode,codex
assert_parity_output_contains "installed codex: clean" "shared project destination"
assert_parity_output_contains "installed opencode: clean" "shared project destination"

new_parity_fixture
run_authoritative_setup install --target claude
user_state=$OAW_STATE/open-agent-workflow/installations/user.state
awk '
  $1 == "scope" { print "scope\tproject"; next }
  { print }
' "$user_state" >"$OAW_TMP/mismatched.state"
mv "$OAW_TMP/mismatched.state" "$user_state"
run_check_pair "Install State scope mismatch" --target claude
assert_parity_output_contains "installed claude: invalid-state" "Install State scope mismatch"

new_parity_fixture
run_authoritative_setup install --target claude
user_state=$OAW_STATE/open-agent-workflow/installations/user.state
awk -F '\t' -v OFS='\t' -v wrong="$OAW_HOME/wrong/CLAUDE.md" '
  $1 == "target" { $3 = wrong }
  { print }
' "$user_state" >"$OAW_TMP/mismatched.state"
mv "$OAW_TMP/mismatched.state" "$user_state"
run_check_pair "Install State target mismatch" --target claude
assert_parity_output_contains "installed claude: invalid-state" "Install State target mismatch"

new_parity_fixture
for parity_case in \
  'missing-target' \
  'empty-target' \
  'duplicate-target' \
  'missing-project' \
  'dry-run' \
  'force' \
  'unknown-option' \
  'unexpected-argument' \
  'help'
do
  case "$parity_case" in
    missing-target) run_check_pair "$parity_case" --target ;;
    empty-target) run_check_pair "$parity_case" --target= ;;
    duplicate-target) run_check_pair "$parity_case" --target claude --target=codex ;;
    missing-project) run_check_pair "$parity_case" --project ;;
    dry-run) run_check_pair "$parity_case" --dry-run ;;
    force) run_check_pair "$parity_case" --force ;;
    unknown-option) run_check_pair "$parity_case" --bogus ;;
    unexpected-argument) run_check_pair "$parity_case" operand ;;
    help) run_check_pair "$parity_case" --help ;;
  esac
done

run_check_pair "missing project scope" --project "$OAW_SANDBOX/missing project"
run_check_pair "user extension target rejection" --target cursor

new_parity_fixture
mkdir -p "$OAW_SANDBOX/outside"
ln -s "$OAW_SANDBOX/outside" "$OAW_HOME/.claude"
run_check_pair "symlinked target coordinate" --target claude

pass "Go check shadow path matches every implemented Bash parity fixture"
