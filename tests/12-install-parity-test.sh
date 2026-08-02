#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

OAW_PARITY_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-install-parity.XXXXXX")
OAW_FIXED_SANDBOX=$OAW_PARITY_TEMP/sandbox
OAW_BASH_TREE=$OAW_PARITY_TEMP/bash-tree
OAW_GO_INSTALL=$OAW_PARITY_TEMP/oaw-install-shadow
OAW_REAL_PATH=$PATH

cleanup_install_parity() {
  if [ -n "${OAW_FIXED_SANDBOX:-}" ] && [ -d "$OAW_FIXED_SANDBOX" ]; then
    rm -rf "$OAW_FIXED_SANDBOX"
  fi
  if [ -n "${OAW_PARITY_TEMP:-}" ] && [ -d "$OAW_PARITY_TEMP" ]; then
    rm -rf "$OAW_PARITY_TEMP"
  fi
}

trap cleanup_install_parity EXIT HUP INT TERM

go build -o "$OAW_GO_INSTALL" "$OAW_REPOSITORY/internal/cmd/oaw-management-shadow"

reset_install_fixture() {
  rm -rf "$OAW_FIXED_SANDBOX"
  mkdir -p "$OAW_FIXED_SANDBOX"
  OAW_PATH=$OAW_REAL_PATH
  setup_sandbox_at "$OAW_FIXED_SANDBOX"
  OAW_TMP=$OAW_FIXED_SANDBOX/tmp
  mkdir -p "$OAW_TMP"
}

run_setup_install() {
  set +e
  HOME="$OAW_HOME" \
    XDG_CONFIG_HOME="$OAW_CONFIG" \
    XDG_STATE_HOME="$OAW_STATE" \
    PATH="$OAW_REAL_PATH" \
    TMPDIR="$OAW_TMP" \
    bash "$OAW_INSTALLER" install "$@" \
    >"$OAW_PARITY_TEMP/setup.stdout" 2>"$OAW_PARITY_TEMP/setup.stderr"
  setup_status=$?
  set -e
  if [ "$setup_status" -ne 0 ]; then
    fail "authoritative install setup failed with $setup_status: $(cat "$OAW_PARITY_TEMP/setup.stderr")"
  fi
}

setup_install_case() {
  install_case=$1
  case "$install_case" in
    existing-newline)
      mkdir -p "$OAW_HOME/.claude"
      printf 'user content\n' >"$OAW_HOME/.claude/CLAUDE.md"
      ;;
    existing-no-newline)
      mkdir -p "$OAW_HOME/.claude"
      printf 'user content' >"$OAW_HOME/.claude/CLAUDE.md"
      ;;
    repeated-user|additive-user|backup-reference|policy-drift|target-drift|checkout-mismatch)
      run_setup_install --target claude
      case "$install_case" in
        backup-reference)
          backup_path=$OAW_STATE/open-agent-workflow/backups/existing
          mkdir -p "$backup_path"
          printf 'sentinel backup\n' >"$backup_path/manifest.tsv"
          user_state=$OAW_STATE/open-agent-workflow/installations/user.state
          awk -F '\t' -v OFS='\t' -v backup="$backup_path" '
            $1 == "directory" && !inserted { print "backup", backup; inserted = 1 }
            { print }
            END { if (!inserted) print "backup", backup }
          ' "$user_state" >"$OAW_TMP/user.state"
          mv "$OAW_TMP/user.state" "$user_state"
          chmod 600 "$user_state"
          ;;
        policy-drift)
          printf 'policy drift\n' >>"$OAW_CONFIG/open-agent-workflow/ENGINEERING.md"
          ;;
        target-drift)
          printf '%s\n' \
            '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
            'target drift' \
            '<!-- END OPEN AGENT WORKFLOW -->' \
            >"$OAW_HOME/.claude/CLAUDE.md"
          ;;
        checkout-mismatch)
          user_state=$OAW_STATE/open-agent-workflow/installations/user.state
          awk -F '\t' -v OFS='\t' '
            $1 == "version" { $2 = "9.9.9" }
            { print }
          ' "$user_state" >"$OAW_TMP/user.state"
          mv "$OAW_TMP/user.state" "$user_state"
          chmod 600 "$user_state"
          ;;
      esac
      ;;
    cross-scope)
      run_setup_install --target claude
      ;;
    project-link-space)
      rm -rf "$OAW_PROJECT"
      real_project=$OAW_FIXED_SANDBOX/'real project'
      OAW_PROJECT=$OAW_FIXED_SANDBOX/'project link'
      mkdir -p "$real_project"
      ln -s "$real_project" "$OAW_PROJECT"
      ;;
    hostile-project)
      rm -rf "$OAW_PROJECT"
      OAW_PROJECT=$OAW_FIXED_SANDBOX/'-project [glob]* ; touch PARTIAL-PAYLOAD'
      mkdir -p "$OAW_PROJECT"
      ;;
    owned-collision|force-owned-collision)
      mkdir -p "$OAW_PROJECT/.cursor/rules"
      printf 'foreign owned file\n' >"$OAW_PROJECT/.cursor/rules/open-agent-workflow.mdc"
      ;;
    untracked-markers)
      mkdir -p "$OAW_HOME/.claude"
      printf '%s\n' \
        '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
        'foreign block' \
        '<!-- END OPEN AGENT WORKFLOW -->' \
        >"$OAW_HOME/.claude/CLAUDE.md"
      ;;
    partial-markers)
      mkdir -p "$OAW_HOME/.claude"
      printf '%s\n' '<!-- BEGIN OPEN AGENT WORKFLOW -->' 'partial block' \
        >"$OAW_HOME/.claude/CLAUDE.md"
      ;;
    invalid-state)
      mkdir -p "$OAW_STATE/open-agent-workflow/installations"
      printf 'format\t2\n' >"$OAW_STATE/open-agent-workflow/installations/user.state"
      chmod 600 "$OAW_STATE/open-agent-workflow/installations/user.state"
      ;;
    symlink-component)
      outside=$OAW_FIXED_SANDBOX/outside
      mkdir -p "$outside"
      ln -s "$outside" "$OAW_HOME/.claude"
      ;;
    later-target)
      mkdir -p "$OAW_PROJECT/.claude"
      printf 'pre-existing instructions\n' >"$OAW_PROJECT/.claude/CLAUDE.md"
      printf 'not a directory\n' >"$OAW_PROJECT/.cursor"
      ;;
  esac
}

run_install_implementation() {
  implementation=$1
  shift
  stdout_file=$OAW_PARITY_TEMP/$implementation.stdout
  stderr_file=$OAW_PARITY_TEMP/$implementation.stderr
  set +e
  case "$implementation" in
    bash)
      HOME="$OAW_HOME" \
        XDG_CONFIG_HOME="$OAW_CONFIG" \
        XDG_STATE_HOME="$OAW_STATE" \
        PATH="$OAW_PATH" \
        TMPDIR="$OAW_TMP" \
        bash "$OAW_INSTALLER" install "$@" >"$stdout_file" 2>"$stderr_file"
      BASH_INSTALL_STATUS=$?
      ;;
    go)
      HOME="$OAW_HOME" \
        XDG_CONFIG_HOME="$OAW_CONFIG" \
        XDG_STATE_HOME="$OAW_STATE" \
        PATH="$OAW_PATH" \
        TMPDIR="$OAW_TMP" \
        "$OAW_GO_INSTALL" install "$@" >"$stdout_file" 2>"$stderr_file"
      GO_INSTALL_STATUS=$?
      ;;
    *)
      set -e
      fail "unknown install parity implementation: $implementation"
      ;;
  esac
  set -e
}

execute_install_case() {
  implementation=$1
  install_case=$2
  case "$install_case" in
    fresh-user-default) run_install_implementation "$implementation" ;;
    user-extension) run_install_implementation "$implementation" --target cursor ;;
    user-*) run_install_implementation "$implementation" --target "${install_case#user-}" ;;
    existing-newline|existing-no-newline|repeated-user|policy-drift|target-drift|checkout-mismatch)
      run_install_implementation "$implementation" --target claude
      ;;
    additive-user|backup-reference)
      run_install_implementation "$implementation" --target codex
      ;;
    project-target-*)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target "${install_case#project-target-}"
      ;;
    project-shared)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target opencode,codex
      ;;
    project-link-space)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target claude,cursor
      ;;
    cross-scope)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target cursor
      ;;
    dry-run-user)
      run_install_implementation "$implementation" --target claude,codex --dry-run
      ;;
    owned-collision)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target cursor
      ;;
    force-owned-collision)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target cursor --force
      ;;
    untracked-markers|partial-markers|invalid-state|symlink-component)
      run_install_implementation "$implementation" --target claude
      ;;
    unknown-target)
      run_install_implementation "$implementation" --target vscode
      ;;
    missing-project)
      run_install_implementation "$implementation" --project "$OAW_FIXED_SANDBOX/missing project" --target claude
      ;;
    missing-target-value)
      run_install_implementation "$implementation" --target
      ;;
    duplicate-target)
      run_install_implementation "$implementation" --target claude --target=codex
      ;;
    duplicate-project)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --project="$OAW_PROJECT"
      ;;
    duplicate-dry-run)
      run_install_implementation "$implementation" --dry-run --dry-run
      ;;
    duplicate-force)
      run_install_implementation "$implementation" --force --force
      ;;
    unknown-option)
      run_install_implementation "$implementation" --bogus
      ;;
    unexpected-operand)
      run_install_implementation "$implementation" operand
      ;;
    help)
      run_install_implementation "$implementation" --help
      ;;
    hostile-project)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target cursor
      ;;
    later-target)
      run_install_implementation "$implementation" --project "$OAW_PROJECT" --target claude,cursor
      ;;
    *) fail "unknown install parity case: $install_case" ;;
  esac
}

snapshot_optional_tree() {
  optional_root=$1
  if [ -e "$optional_root" ]; then
    snapshot_tree "$optional_root"
  else
    printf 'absent\n'
  fi
}

case_requires_no_write() {
  case "$1" in
    repeated-user|dry-run-user|owned-collision|force-owned-collision|untracked-markers|partial-markers|invalid-state|policy-drift|target-drift|checkout-mismatch|unknown-target|user-extension|missing-project|missing-target-value|duplicate-target|duplicate-project|duplicate-dry-run|duplicate-force|unknown-option|unexpected-operand|help|symlink-component|later-target)
      return 0
      ;;
    *) return 1 ;;
  esac
}

save_bash_tree() {
  rm -rf "$OAW_BASH_TREE"
  mkdir -p "$OAW_BASH_TREE"
  cp -pR "$OAW_FIXED_SANDBOX/." "$OAW_BASH_TREE"
}

compare_regular_file_bytes() {
  find "$OAW_BASH_TREE" -type f -print | LC_ALL=C sort | while IFS= read -r bash_file; do
    relative=${bash_file#"$OAW_BASH_TREE"/}
    go_file=$OAW_FIXED_SANDBOX/$relative
    if ! cmp -s "$bash_file" "$go_file"; then
      fail "$install_description bytes differ for $relative"
    fi
  done
}

run_install_pair() {
  install_description=$1
  install_case=$2

  reset_install_fixture
  setup_install_case "$install_case"
  snapshot_tree "$OAW_FIXED_SANDBOX" >"$OAW_PARITY_TEMP/bash-before.snapshot"
  snapshot_optional_tree "$OAW_STATE/open-agent-workflow/backups" \
    >"$OAW_PARITY_TEMP/bash-backup-before.snapshot"
  execute_install_case bash "$install_case"
  snapshot_tree "$OAW_FIXED_SANDBOX" >"$OAW_PARITY_TEMP/bash.snapshot"
  snapshot_optional_tree "$OAW_STATE/open-agent-workflow/backups" \
    >"$OAW_PARITY_TEMP/bash-backup-after.snapshot"
  if ! cmp -s "$OAW_PARITY_TEMP/bash-backup-before.snapshot" \
    "$OAW_PARITY_TEMP/bash-backup-after.snapshot"; then
    fail "$install_description Bash install changed the backup tree"
  fi
  if case_requires_no_write "$install_case" &&
    ! cmp -s "$OAW_PARITY_TEMP/bash-before.snapshot" "$OAW_PARITY_TEMP/bash.snapshot"; then
    fail "$install_description Bash rejection or dry-run changed the fixture"
  fi
  save_bash_tree

  reset_install_fixture
  setup_install_case "$install_case"
  snapshot_tree "$OAW_FIXED_SANDBOX" >"$OAW_PARITY_TEMP/go-before.snapshot"
  snapshot_optional_tree "$OAW_STATE/open-agent-workflow/backups" \
    >"$OAW_PARITY_TEMP/go-backup-before.snapshot"
  execute_install_case go "$install_case"
  snapshot_tree "$OAW_FIXED_SANDBOX" >"$OAW_PARITY_TEMP/go.snapshot"
  snapshot_optional_tree "$OAW_STATE/open-agent-workflow/backups" \
    >"$OAW_PARITY_TEMP/go-backup-after.snapshot"
  if ! cmp -s "$OAW_PARITY_TEMP/go-backup-before.snapshot" \
    "$OAW_PARITY_TEMP/go-backup-after.snapshot"; then
    fail "$install_description Go install changed the backup tree"
  fi
  if case_requires_no_write "$install_case" &&
    ! cmp -s "$OAW_PARITY_TEMP/go-before.snapshot" "$OAW_PARITY_TEMP/go.snapshot"; then
    fail "$install_description Go rejection or dry-run changed the fixture"
  fi

  if [ "$BASH_INSTALL_STATUS" -ne "$GO_INSTALL_STATUS" ]; then
    diff -u "$OAW_PARITY_TEMP/bash.stdout" "$OAW_PARITY_TEMP/go.stdout" >&2 || true
    diff -u "$OAW_PARITY_TEMP/bash.stderr" "$OAW_PARITY_TEMP/go.stderr" >&2 || true
    fail "$install_description status differs: Bash=$BASH_INSTALL_STATUS Go=$GO_INSTALL_STATUS"
  fi
  if ! cmp -s "$OAW_PARITY_TEMP/bash.stdout" "$OAW_PARITY_TEMP/go.stdout"; then
    diff -u "$OAW_PARITY_TEMP/bash.stdout" "$OAW_PARITY_TEMP/go.stdout" >&2 || true
    fail "$install_description stdout differs"
  fi
  if ! cmp -s "$OAW_PARITY_TEMP/bash.stderr" "$OAW_PARITY_TEMP/go.stderr"; then
    diff -u "$OAW_PARITY_TEMP/bash.stderr" "$OAW_PARITY_TEMP/go.stderr" >&2 || true
    fail "$install_description stderr differs"
  fi
  if ! cmp -s "$OAW_PARITY_TEMP/bash.snapshot" "$OAW_PARITY_TEMP/go.snapshot"; then
    diff -u "$OAW_PARITY_TEMP/bash.snapshot" "$OAW_PARITY_TEMP/go.snapshot" >&2 || true
    fail "$install_description final trees differ"
  fi
  compare_regular_file_bytes
  if [ -e "$OAW_FIXED_SANDBOX/PARTIAL-PAYLOAD" ]; then
    fail "$install_description interpreted hostile path text"
  fi
  pass "$install_description has exact mutating Bash/Go parity"
}

run_install_pair "fresh default user install" fresh-user-default
for user_target in claude codex gemini opencode; do
  run_install_pair "fresh user $user_target install" "user-$user_target"
done
run_install_pair "managed content with final newline" existing-newline
run_install_pair "managed content without final newline" existing-no-newline
run_install_pair "repeated user install" repeated-user
run_install_pair "additive user install" additive-user

for project_target in claude codex gemini opencode cursor windsurf cline roo copilot; do
  run_install_pair "project $project_target install" "project-target-$project_target"
done
run_install_pair "shared project destination" project-shared
run_install_pair "physical project path with spaces" project-link-space
run_install_pair "cross-scope policy and state coordination" cross-scope
run_install_pair "fresh install dry-run" dry-run-user
run_install_pair "prior backup reference preservation" backup-reference

for rejection_case in \
  owned-collision \
  force-owned-collision \
  untracked-markers \
  partial-markers \
  invalid-state \
  policy-drift \
  target-drift \
  checkout-mismatch \
  unknown-target \
  user-extension \
  missing-project \
  missing-target-value \
  duplicate-target \
  duplicate-project \
  duplicate-dry-run \
  duplicate-force \
  unknown-option \
  unexpected-operand \
  help \
  symlink-component \
  later-target; do
  run_install_pair "$rejection_case" "$rejection_case"
done

run_install_pair "hostile inert project path" hostile-project

pass "Go install shadow path matches every implemented Bash parity fixture"
