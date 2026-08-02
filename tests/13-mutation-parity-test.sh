#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

OAW_MUTATION_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-mutation-parity.XXXXXX")
OAW_FIXED_SANDBOX=$OAW_MUTATION_TEMP/sandbox
OAW_BASELINE_TREE=$OAW_MUTATION_TEMP/baseline-tree
OAW_GO_MANAGEMENT=$OAW_MUTATION_TEMP/oaw-management-shadow
OAW_CHANGED_SOURCE=$OAW_MUTATION_TEMP/changed-source
OAW_CHANGED_GO=$OAW_MUTATION_TEMP/oaw-management-shadow-changed
OAW_REAL_PATH=$PATH
OAW_ACTIVE_INSTALLER=$OAW_INSTALLER
OAW_ACTIVE_GO=$OAW_GO_MANAGEMENT

cleanup_mutation_parity() {
  if [ -n "${OAW_FIXED_SANDBOX:-}" ] && [ -d "$OAW_FIXED_SANDBOX" ]; then
    rm -rf "$OAW_FIXED_SANDBOX"
  fi
  if [ -n "${OAW_MUTATION_TEMP:-}" ] && [ -d "$OAW_MUTATION_TEMP" ]; then
    rm -rf "$OAW_MUTATION_TEMP"
  fi
}

trap cleanup_mutation_parity EXIT HUP INT TERM

go build -o "$OAW_GO_MANAGEMENT" "$OAW_REPOSITORY/internal/cmd/oaw-management-shadow"

mkdir -p "$OAW_CHANGED_SOURCE"
cp -pR "$OAW_REPOSITORY/." "$OAW_CHANGED_SOURCE/"
printf '99.0.0\n' >"$OAW_CHANGED_SOURCE/VERSION"
printf '\n<!-- mutation parity changed checkout -->\n' >>"$OAW_CHANGED_SOURCE/policy/ENGINEERING.md"
(
  cd "$OAW_CHANGED_SOURCE"
  go build -o "$OAW_CHANGED_GO" ./internal/cmd/oaw-management-shadow
)

reset_mutation_fixture() {
  rm -rf "$OAW_FIXED_SANDBOX" "$OAW_BASELINE_TREE"
  mkdir -p "$OAW_FIXED_SANDBOX"
  setup_sandbox_at "$OAW_FIXED_SANDBOX"
  OAW_TMP=$OAW_FIXED_SANDBOX/tmp
  mkdir -p "$OAW_TMP"
  OAW_ACTIVE_INSTALLER=$OAW_INSTALLER
  OAW_ACTIVE_GO=$OAW_GO_MANAGEMENT
}

run_setup_command() {
  set +e
  HOME="$OAW_HOME" \
    XDG_CONFIG_HOME="$OAW_CONFIG" \
    XDG_STATE_HOME="$OAW_STATE" \
    PATH="$OAW_REAL_PATH" \
    TMPDIR="$OAW_TMP" \
    bash "$OAW_INSTALLER" "$@" \
    >"$OAW_MUTATION_TEMP/setup.stdout" 2>"$OAW_MUTATION_TEMP/setup.stderr"
  setup_status=$?
  set -e
  if [ "$setup_status" -ne 0 ]; then
    fail "authoritative mutation setup failed with $setup_status: $(cat "$OAW_MUTATION_TEMP/setup.stderr")"
  fi
}

write_target_drift() {
  printf '%s\n' \
    '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
    'target drift' \
    '<!-- END OPEN AGENT WORKFLOW -->' \
    >"$OAW_HOME/.claude/CLAUDE.md"
  chmod 640 "$OAW_HOME/.claude/CLAUDE.md"
}

setup_mutation_case() {
  mutation_case=$1
  case "$mutation_case" in
    user-update-*|user-uninstall-*)
      mutation_target=${mutation_case#user-update-}
      mutation_target=${mutation_target#user-uninstall-}
      run_setup_command install --target "$mutation_target"
      ;;
    project-update-*|project-uninstall-*)
      mutation_target=${mutation_case#project-update-}
      mutation_target=${mutation_target#project-uninstall-}
      run_setup_command install --project "$OAW_PROJECT" --target "$mutation_target"
      ;;
    partial-uninstall)
      run_setup_command install --target claude,codex
      ;;
    shared-uninstall)
      run_setup_command install --project "$OAW_PROJECT" --target codex,opencode
      ;;
    cross-scope-uninstall)
      run_setup_command install --target claude
      run_setup_command install --project "$OAW_PROJECT" --target cursor
      ;;
    missing-update|missing-uninstall)
      ;;
    dry-run-update|dry-run-uninstall|clean-force-update|clean-force-uninstall)
      run_setup_command install --target claude,codex
      ;;
    repeated-root-spelling)
      OAW_HOME=$OAW_HOME/
      OAW_CONFIG=$OAW_CONFIG/
      OAW_STATE=$OAW_STATE/
      run_setup_command install --target claude
      write_target_drift
      ;;
    target-drift|forced-target-drift|recoverable-begin|recoverable-end|manual-recovery|backup-root-symlink)
      run_setup_command install --target claude
      case "$mutation_case" in
        target-drift|forced-target-drift|backup-root-symlink)
          write_target_drift
          ;;
        recoverable-begin)
          sed '/<!-- BEGIN OPEN AGENT WORKFLOW -->/d' "$OAW_HOME/.claude/CLAUDE.md" >"$OAW_TMP/target"
          mv "$OAW_TMP/target" "$OAW_HOME/.claude/CLAUDE.md"
          chmod 644 "$OAW_HOME/.claude/CLAUDE.md"
          ;;
        recoverable-end)
          sed '/<!-- END OPEN AGENT WORKFLOW -->/d' "$OAW_HOME/.claude/CLAUDE.md" >"$OAW_TMP/target"
          mv "$OAW_TMP/target" "$OAW_HOME/.claude/CLAUDE.md"
          chmod 644 "$OAW_HOME/.claude/CLAUDE.md"
          ;;
        manual-recovery)
          printf '%s\n' \
            'personal content' \
            '<!-- END OPEN AGENT WORKFLOW -->' \
            'ambiguous content' \
            '<!-- END OPEN AGENT WORKFLOW -->' \
            >"$OAW_HOME/.claude/CLAUDE.md"
          chmod 640 "$OAW_HOME/.claude/CLAUDE.md"
          ;;
      esac
      if [ "$mutation_case" = backup-root-symlink ]; then
        outside_backup=$OAW_FIXED_SANDBOX/outside-backup
        mkdir -p "$outside_backup" "$OAW_STATE/open-agent-workflow"
        ln -s "$outside_backup" "$OAW_STATE/open-agent-workflow/backups"
      fi
      ;;
    policy-drift|forced-policy-drift)
      run_setup_command install --target claude
      printf 'policy drift\n' >>"$OAW_CONFIG/open-agent-workflow/ENGINEERING.md"
      ;;
    invalid-state)
      mkdir -p "$OAW_STATE/open-agent-workflow/installations"
      printf 'format\t2\n' >"$OAW_STATE/open-agent-workflow/installations/user.state"
      chmod 600 "$OAW_STATE/open-agent-workflow/installations/user.state"
      ;;
    scope-drift)
      run_setup_command install --target claude
      user_state=$OAW_STATE/open-agent-workflow/installations/user.state
      awk -F '\t' -v OFS='\t' '$1 == "scope" { $2 = "project" } { print }' \
        "$user_state" >"$OAW_TMP/user.state"
      mv "$OAW_TMP/user.state" "$user_state"
      chmod 600 "$user_state"
      ;;
    project-drift)
      run_setup_command install --project "$OAW_PROJECT" --target cursor
      project_physical=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
      project_identity=$(printf '%s' "$project_physical" | cksum | awk '{ print $1 "-" $2 }')
      project_state=$OAW_STATE/open-agent-workflow/installations/projects/$project_identity.state
      awk -F '\t' -v OFS='\t' -v project="$OAW_FIXED_SANDBOX/other-project" \
        '$1 == "project" { $2 = project } { print }' \
        "$project_state" >"$OAW_TMP/project.state"
      mv "$OAW_TMP/project.state" "$project_state"
      chmod 600 "$project_state"
      ;;
    directory-redirection)
      run_setup_command install --project "$OAW_PROJECT" --target cursor
      moved_cursor=$OAW_FIXED_SANDBOX/moved-cursor
      mv "$OAW_PROJECT/.cursor" "$moved_cursor"
      ln -s "$moved_cursor" "$OAW_PROJECT/.cursor"
      ;;
    later-invalid-state-record)
      run_setup_command install --target claude,codex
      user_state=$OAW_STATE/open-agent-workflow/installations/user.state
      awk -F '\t' -v OFS='\t' '
        $1 == "target" && $2 == "codex" { $4 = "/tmp/oaw-invalid-target-path" }
        { print }
      ' "$user_state" >"$OAW_TMP/user.state"
      mv "$OAW_TMP/user.state" "$user_state"
      chmod 600 "$user_state"
      ;;
    hostile-project)
      rm -rf "$OAW_PROJECT"
      OAW_PROJECT=$OAW_FIXED_SANDBOX/'-project [glob]* ; touch PARTIAL-PAYLOAD'
      mkdir -p "$OAW_PROJECT"
      run_setup_command install --project "$OAW_PROJECT" --target cursor
      ;;
    changed-checkout)
      run_setup_command install --target claude
      OAW_ACTIVE_INSTALLER=$OAW_CHANGED_SOURCE/install.sh
      OAW_ACTIVE_GO=$OAW_CHANGED_GO
      ;;
    *)
      fail "unknown mutation parity setup: $mutation_case"
      ;;
  esac
}

run_mutation_implementation() {
  implementation=$1
  operation=$2
  shift 2
  stdout_file=$OAW_MUTATION_TEMP/$implementation.stdout
  stderr_file=$OAW_MUTATION_TEMP/$implementation.stderr
  set +e
  case "$implementation" in
    bash)
      HOME="$OAW_HOME" \
        XDG_CONFIG_HOME="$OAW_CONFIG" \
        XDG_STATE_HOME="$OAW_STATE" \
        PATH="$OAW_REAL_PATH" \
        TMPDIR="$OAW_TMP" \
        bash "$OAW_ACTIVE_INSTALLER" "$operation" "$@" >"$stdout_file" 2>"$stderr_file"
      BASH_MUTATION_STATUS=$?
      ;;
    go)
      HOME="$OAW_HOME" \
        XDG_CONFIG_HOME="$OAW_CONFIG" \
        XDG_STATE_HOME="$OAW_STATE" \
        PATH="$OAW_REAL_PATH" \
        TMPDIR="$OAW_TMP" \
        "$OAW_ACTIVE_GO" "$operation" "$@" >"$stdout_file" 2>"$stderr_file"
      GO_MUTATION_STATUS=$?
      ;;
    *)
      set -e
      fail "unknown mutation implementation: $implementation"
      ;;
  esac
  set -e
}

execute_mutation_case() {
  implementation=$1
  mutation_case=$2
  case "$mutation_case" in
    user-update-*)
      run_mutation_implementation "$implementation" update --target "${mutation_case#user-update-}"
      ;;
    project-update-*)
      run_mutation_implementation "$implementation" update --project "$OAW_PROJECT" --target "${mutation_case#project-update-}"
      ;;
    user-uninstall-*)
      run_mutation_implementation "$implementation" uninstall --target "${mutation_case#user-uninstall-}"
      ;;
    project-uninstall-*)
      run_mutation_implementation "$implementation" uninstall --project "$OAW_PROJECT" --target "${mutation_case#project-uninstall-}"
      ;;
    partial-uninstall)
      run_mutation_implementation "$implementation" uninstall --target claude
      ;;
    shared-uninstall)
      run_mutation_implementation "$implementation" uninstall --project "$OAW_PROJECT" --target codex
      ;;
    cross-scope-uninstall)
      run_mutation_implementation "$implementation" uninstall --project "$OAW_PROJECT" --target cursor
      ;;
    missing-update)
      run_mutation_implementation "$implementation" update --target claude
      ;;
    missing-uninstall)
      run_mutation_implementation "$implementation" uninstall --target claude
      ;;
    dry-run-update)
      run_mutation_implementation "$implementation" update --target claude --dry-run
      ;;
    dry-run-uninstall)
      run_mutation_implementation "$implementation" uninstall --target claude --dry-run
      ;;
    clean-force-update)
      run_mutation_implementation "$implementation" update --target claude --force
      ;;
    clean-force-uninstall)
      run_mutation_implementation "$implementation" uninstall --target claude --force
      ;;
    target-drift|policy-drift|invalid-state|scope-drift|project-drift|directory-redirection|later-invalid-state-record)
      run_mutation_implementation "$implementation" update --target claude,codex
      ;;
    forced-target-drift|forced-policy-drift|repeated-root-spelling|recoverable-begin|recoverable-end|manual-recovery|backup-root-symlink)
      run_mutation_implementation "$implementation" update --target claude --force
      ;;
    hostile-project)
      run_mutation_implementation "$implementation" update --project "$OAW_PROJECT" --target cursor
      ;;
    changed-checkout)
      run_mutation_implementation "$implementation" update --target claude
      ;;
    *)
      fail "unknown mutation parity execution: $mutation_case"
      ;;
  esac
}

normalize_backup_ids() {
  sed -E 's/[0-9]{8}T[0-9]{6}Z-[0-9]+/BACKUP-ID/g'
}

snapshot_normalized_tree() {
  snapshot_root=$1
  find "$snapshot_root" -print | LC_ALL=C sort | while IFS= read -r snapshot_path; do
    snapshot_relative=${snapshot_path#"$snapshot_root"}
    [ -n "$snapshot_relative" ] || snapshot_relative=/
    normalized_relative=$(printf '%s\n' "$snapshot_relative" | normalize_backup_ids)
    snapshot_mode=$(portable_mode "$snapshot_path")
    if [ -L "$snapshot_path" ]; then
      normalized_link=$(readlink "$snapshot_path" | normalize_backup_ids)
      printf 'link\t%s\t%s\t%s\n' "$normalized_relative" "$snapshot_mode" "$normalized_link"
    elif [ -d "$snapshot_path" ]; then
      printf 'directory\t%s\t%s\n' "$normalized_relative" "$snapshot_mode"
    elif [ -f "$snapshot_path" ]; then
      normalized_checksum=$(normalize_backup_ids <"$snapshot_path" | cksum | awk '{ print $1 ":" $2 }')
      printf 'file\t%s\t%s\t%s\n' "$normalized_relative" "$snapshot_mode" "$normalized_checksum"
    else
      printf 'other\t%s\t%s\n' "$normalized_relative" "$snapshot_mode"
    fi
  done
}

save_mutation_baseline() {
  rm -rf "$OAW_BASELINE_TREE"
  mkdir -p "$OAW_BASELINE_TREE"
  cp -pR "$OAW_FIXED_SANDBOX/." "$OAW_BASELINE_TREE/"
}

restore_mutation_baseline() {
  rm -rf "$OAW_FIXED_SANDBOX"
  mkdir -p "$OAW_FIXED_SANDBOX"
  cp -pR "$OAW_BASELINE_TREE/." "$OAW_FIXED_SANDBOX/"
}

run_mutation_pair() {
  mutation_description=$1
  mutation_case=$2

  reset_mutation_fixture
  setup_mutation_case "$mutation_case"
  save_mutation_baseline

  execute_mutation_case bash "$mutation_case"
  normalize_backup_ids <"$OAW_MUTATION_TEMP/bash.stdout" >"$OAW_MUTATION_TEMP/bash.normal.stdout"
  normalize_backup_ids <"$OAW_MUTATION_TEMP/bash.stderr" >"$OAW_MUTATION_TEMP/bash.normal.stderr"
  snapshot_normalized_tree "$OAW_FIXED_SANDBOX" >"$OAW_MUTATION_TEMP/bash.snapshot"

  restore_mutation_baseline
  execute_mutation_case go "$mutation_case"
  normalize_backup_ids <"$OAW_MUTATION_TEMP/go.stdout" >"$OAW_MUTATION_TEMP/go.normal.stdout"
  normalize_backup_ids <"$OAW_MUTATION_TEMP/go.stderr" >"$OAW_MUTATION_TEMP/go.normal.stderr"
  snapshot_normalized_tree "$OAW_FIXED_SANDBOX" >"$OAW_MUTATION_TEMP/go.snapshot"

  if [ "$BASH_MUTATION_STATUS" -ne "$GO_MUTATION_STATUS" ]; then
    diff -u "$OAW_MUTATION_TEMP/bash.normal.stdout" "$OAW_MUTATION_TEMP/go.normal.stdout" >&2 || true
    diff -u "$OAW_MUTATION_TEMP/bash.normal.stderr" "$OAW_MUTATION_TEMP/go.normal.stderr" >&2 || true
    fail "$mutation_description status differs: Bash=$BASH_MUTATION_STATUS Go=$GO_MUTATION_STATUS"
  fi
  if ! cmp -s "$OAW_MUTATION_TEMP/bash.normal.stdout" "$OAW_MUTATION_TEMP/go.normal.stdout"; then
    diff -u "$OAW_MUTATION_TEMP/bash.normal.stdout" "$OAW_MUTATION_TEMP/go.normal.stdout" >&2 || true
    fail "$mutation_description stdout differs"
  fi
  if ! cmp -s "$OAW_MUTATION_TEMP/bash.normal.stderr" "$OAW_MUTATION_TEMP/go.normal.stderr"; then
    diff -u "$OAW_MUTATION_TEMP/bash.normal.stderr" "$OAW_MUTATION_TEMP/go.normal.stderr" >&2 || true
    fail "$mutation_description stderr differs"
  fi
  if ! cmp -s "$OAW_MUTATION_TEMP/bash.snapshot" "$OAW_MUTATION_TEMP/go.snapshot"; then
    diff -u "$OAW_MUTATION_TEMP/bash.snapshot" "$OAW_MUTATION_TEMP/go.snapshot" >&2 || true
    fail "$mutation_description final trees differ"
  fi
  if [ -e "$OAW_FIXED_SANDBOX/PARTIAL-PAYLOAD" ]; then
    fail "$mutation_description interpreted hostile path text"
  fi
  pass "$mutation_description has same-path Bash/Go parity"
}

for user_target in claude codex gemini opencode; do
  run_mutation_pair "clean user $user_target update" "user-update-$user_target"
  run_mutation_pair "final user $user_target uninstall" "user-uninstall-$user_target"
done

for project_target in claude codex gemini opencode cursor windsurf cline roo copilot; do
  run_mutation_pair "clean project $project_target update" "project-update-$project_target"
  run_mutation_pair "final project $project_target uninstall" "project-uninstall-$project_target"
done

for mutation_case in \
  partial-uninstall \
  shared-uninstall \
  cross-scope-uninstall \
  missing-update \
  missing-uninstall \
  dry-run-update \
  dry-run-uninstall \
  clean-force-update \
  clean-force-uninstall \
  target-drift \
  policy-drift \
  forced-target-drift \
  repeated-root-spelling \
  forced-policy-drift \
  recoverable-begin \
  recoverable-end \
  manual-recovery \
  invalid-state \
  scope-drift \
  project-drift \
  directory-redirection \
  later-invalid-state-record \
  backup-root-symlink \
  hostile-project \
  changed-checkout; do
  run_mutation_pair "$mutation_case" "$mutation_case"
done

pass "Go update and uninstall shadow paths match the Bash oracle"
