#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

file_fingerprint() {
  local fingerprint_file=$1
  local fingerprint_stat=

  if fingerprint_stat=$(stat -f '%d:%i:%m:%z' "$fingerprint_file" 2>/dev/null); then
    :
  else
    fingerprint_stat=$(stat -c '%d:%i:%Y:%s' "$fingerprint_file")
  fi
  printf '%s:%s\n' "$(cksum <"$fingerprint_file")" "$fingerprint_stat"
}

assert_invalid_project_state() {
  local invalid_case=$1
  local invalid_description=$2
  local original_state=
  local mutated_state=
  local other_project=
  local other_physical=
  local policy_before=
  local target_before=
  local state_before=

  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_PROJECT="$OAW_SANDBOX/project with spaces"
  mkdir -p "$OAW_PROJECT"
  run_oaw install --project "$OAW_PROJECT" --target cursor
  assert_status 0 "$invalid_description fixture install"

  OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
  OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
  OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
  OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
  original_state=$OAW_SANDBOX/original.state
  mutated_state=$OAW_SANDBOX/mutated.state
  cp "$OAW_PROJECT_STATE" "$original_state"

  case "$invalid_case" in
    unknown-record)
      cp "$original_state" "$mutated_state"
      printf 'unknown\tvalue\n' >>"$mutated_state"
      ;;
    wrong-field-count)
      awk -F '\t' 'BEGIN { OFS = "\t" } $1 == "format" { print $0, "extra"; next } { print }' \
        "$original_state" >"$mutated_state"
      ;;
    duplicate-metadata)
      awk '$1 == "policy" { print; print; next } { print }' \
        "$original_state" >"$mutated_state"
      ;;
    duplicate-target)
      awk '$1 == "target" { print; print; next } { print }' \
        "$original_state" >"$mutated_state"
      ;;
    mismatched-project-root)
      other_project=$OAW_SANDBOX/other-project
      mkdir -p "$other_project"
      other_physical=$(CDPATH='' cd -P -- "$other_project" && pwd -P)
      awk -F '\t' -v project_root="$other_physical" \
        'BEGIN { OFS = "\t" } $1 == "project" { $2 = project_root } { print }' \
        "$original_state" >"$mutated_state"
      ;;
    tab-in-field)
      awk -F '\t' 'BEGIN { OFS = "\t" } $1 == "target" { print $0, "extra"; next } { print }' \
        "$original_state" >"$mutated_state"
      ;;
    newline-in-field)
      awk -F '\t' 'BEGIN { OFS = "\t" } $1 == "version" { print $1, $2; print "split-value"; next } { print }' \
        "$original_state" >"$mutated_state"
      ;;
    literal-payload)
      cp "$original_state" "$mutated_state"
      printf 'payload\t$(touch "%s/pwned")\n' "$OAW_SANDBOX" >>"$mutated_state"
      ;;
    *) fail "unknown invalid-state fixture: $invalid_case" ;;
  esac
  mv "$mutated_state" "$OAW_PROJECT_STATE"

  run_oaw check --project "$OAW_PROJECT" --target cursor
  assert_status 0 "$invalid_description check"
  assert_contains "installed cursor: invalid-state" "$invalid_description is classified as invalid state"
  policy_before=$(file_fingerprint "$OAW_POLICY")
  target_before=$(file_fingerprint "$OAW_PROJECT_TARGET")
  state_before=$(file_fingerprint "$OAW_PROJECT_STATE")
  run_oaw update --project "$OAW_PROJECT" --target cursor
  [ "$OAW_STATUS" -ne 0 ] || fail "$invalid_description update unexpectedly succeeded"
  [ "$(file_fingerprint "$OAW_POLICY")" = "$policy_before" ] ||
    fail "$invalid_description changed the canonical policy"
  [ "$(file_fingerprint "$OAW_PROJECT_TARGET")" = "$target_before" ] ||
    fail "$invalid_description changed the project target"
  [ "$(file_fingerprint "$OAW_PROJECT_STATE")" = "$state_before" ] ||
    fail "$invalid_description rewrote the invalid state"
  [ ! -e "$OAW_SANDBOX/pwned" ] || fail "$invalid_description executed a state payload"
}

setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target codex,opencode
assert_status 0 "conflicting-destination state fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_TAMPERED_STATE=$OAW_SANDBOX/conflicting-destination.state
awk -F '\t' '
  BEGIN { OFS = "\t" }
  $1 == "target" && $2 == "opencode" { $5 = "1:1" }
  { print }
' "$OAW_PROJECT_STATE" >"$OAW_TAMPERED_STATE"
mv "$OAW_TAMPERED_STATE" "$OAW_PROJECT_STATE"

run_oaw check --project "$OAW_PROJECT" --target codex
assert_status 0 "check conflicting shared-destination state"
assert_contains "installed codex: invalid-state" \
  "conflicting shared-destination checksums invalidate the complete state"

pass "shared destinations reject inconsistent state checksums"

assert_invalid_project_state unknown-record "unknown state record"
assert_invalid_project_state wrong-field-count "wrong state field count"
assert_invalid_project_state duplicate-metadata "duplicate state metadata"
assert_invalid_project_state duplicate-target "duplicate target state"
assert_invalid_project_state mismatched-project-root "mismatched project root"
assert_invalid_project_state tab-in-field "tab-split state field"
assert_invalid_project_state newline-in-field "newline-split state field"
assert_invalid_project_state literal-payload "literal executable-looking state payload"

pass "malformed and executable-looking state remains inert"

artifact_snapshot() {
  local artifact_path=$1

  if [ -e "$artifact_path" ]; then
    printf 'present:%s\n' "$(file_fingerprint "$artifact_path")"
  else
    printf 'absent\n'
  fi
}

assert_artifact_snapshot() {
  local artifact_path=$1
  local expected_snapshot=$2
  local description=$3

  [ "$(artifact_snapshot "$artifact_path")" = "$expected_snapshot" ] ||
    fail "$description changed $artifact_path"
}

assert_project_drift_blocks_mutation() {
  local drift_case=$1
  local begin_marker='<!-- BEGIN OPEN AGENT WORKFLOW -->'
  local end_marker='<!-- END OPEN AGENT WORKFLOW -->'
  local target_id=
  local target_path=
  local project_physical=
  local project_id=
  local project_state=
  local policy_path=
  local mutated_file=
  local target_before=
  local state_before=
  local policy_before=

  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_PROJECT="$OAW_SANDBOX/project with spaces"
  mkdir -p "$OAW_PROJECT"
  case "$drift_case" in
    changed-owned-file|missing-recorded-file|state-checksum-mismatch) target_id=cursor ;;
    *) target_id=codex ;;
  esac
  run_oaw install --project "$OAW_PROJECT" --target "$target_id"
  assert_status 0 "$drift_case fixture install"

  project_physical=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  project_id=$(printf '%s' "$project_physical" | cksum | awk '{ print $1 "-" $2 }')
  case "$target_id" in
    codex) target_path=$project_physical/AGENTS.md ;;
    cursor) target_path=$project_physical/.cursor/rules/open-agent-workflow.mdc ;;
    *) fail "unknown drift target: $target_id" ;;
  esac
  project_state=$OAW_STATE/open-agent-workflow/installations/projects/$project_id.state
  policy_path=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
  mutated_file=$OAW_SANDBOX/mutated-target

  case "$drift_case" in
    changed-block-body)
      awk '
        $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { in_block = 1 }
        in_block && $0 != "<!-- BEGIN OPEN AGENT WORKFLOW -->" &&
          $0 != "<!-- END OPEN AGENT WORKFLOW -->" {
          print "drifted managed instruction"
          in_block = 0
          next
        }
        { print }
      ' "$target_path" >"$mutated_file"
      mv "$mutated_file" "$target_path"
      ;;
    changed-owned-file)
      printf 'drifted owned adapter\n' >"$target_path"
      ;;
    missing-begin-marker)
      awk -v marker="$begin_marker" '$0 != marker { print }' \
        "$target_path" >"$mutated_file"
      mv "$mutated_file" "$target_path"
      ;;
    missing-end-marker)
      awk -v marker="$end_marker" '$0 != marker { print }' \
        "$target_path" >"$mutated_file"
      mv "$mutated_file" "$target_path"
      ;;
    reversed-markers)
      awk -v begin="$begin_marker" -v end="$end_marker" '
        $0 == begin { print end; next }
        $0 == end { print begin; next }
        { print }
      ' "$target_path" >"$mutated_file"
      mv "$mutated_file" "$target_path"
      ;;
    duplicate-marker-pairs)
      cat "$target_path" "$target_path" >"$mutated_file"
      mv "$mutated_file" "$target_path"
      ;;
    nested-markers)
      awk -v begin="$begin_marker" -v end="$end_marker" '
        $0 == begin { print; print begin; next }
        $0 == end { print end; print; next }
        { print }
      ' "$target_path" >"$mutated_file"
      mv "$mutated_file" "$target_path"
      ;;
    state-checksum-mismatch)
      awk -F '\t' '
        BEGIN { OFS = "\t" }
        $1 == "target" { $5 = "1:1" }
        { print }
      ' "$project_state" >"$mutated_file"
      mv "$mutated_file" "$project_state"
      ;;
    missing-recorded-file)
      rm -- "$target_path"
      ;;
    *) fail "unknown managed drift fixture: $drift_case" ;;
  esac

  run_oaw check --project "$OAW_PROJECT" --target "$target_id"
  assert_status 0 "$drift_case check"
  assert_contains "installed $target_id: drift" "$drift_case check reports drift"

  target_before=$(artifact_snapshot "$target_path")
  state_before=$(artifact_snapshot "$project_state")
  policy_before=$(artifact_snapshot "$policy_path")

  run_oaw update --project "$OAW_PROJECT" --target "$target_id"
  [ "$OAW_STATUS" -ne 0 ] || fail "$drift_case update unexpectedly succeeded"
  assert_contains "$target_id" "$drift_case update identifies the target"
  assert_contains "$target_path" "$drift_case update identifies the destination"
  assert_artifact_snapshot "$target_path" "$target_before" "$drift_case update"
  assert_artifact_snapshot "$project_state" "$state_before" "$drift_case update"
  assert_artifact_snapshot "$policy_path" "$policy_before" "$drift_case update"

  run_oaw uninstall --project "$OAW_PROJECT" --target "$target_id"
  [ "$OAW_STATUS" -ne 0 ] || fail "$drift_case uninstall unexpectedly succeeded"
  assert_contains "$target_id" "$drift_case uninstall identifies the target"
  assert_contains "$target_path" "$drift_case uninstall identifies the destination"
  assert_artifact_snapshot "$target_path" "$target_before" "$drift_case uninstall"
  assert_artifact_snapshot "$project_state" "$state_before" "$drift_case uninstall"
  assert_artifact_snapshot "$policy_path" "$policy_before" "$drift_case uninstall"
}

assert_project_drift_blocks_mutation changed-block-body

pass "managed block body drift blocks mutation with a target/path diagnostic"

assert_project_drift_blocks_mutation changed-owned-file
assert_project_drift_blocks_mutation missing-begin-marker
assert_project_drift_blocks_mutation missing-end-marker
assert_project_drift_blocks_mutation reversed-markers
assert_project_drift_blocks_mutation duplicate-marker-pairs
assert_project_drift_blocks_mutation nested-markers
assert_project_drift_blocks_mutation state-checksum-mismatch
assert_project_drift_blocks_mutation missing-recorded-file

pass "recorded marker, owned-file, checksum, and missing-file drift blocks mutation"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_UNTRACKED_TARGET=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_UNTRACKED_STATE_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_UNTRACKED_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_UNTRACKED_STATE_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
printf '%s\nuntracked body\n%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' '<!-- END OPEN AGENT WORKFLOW -->' \
  >"$OAW_UNTRACKED_TARGET"

run_oaw check --project "$OAW_PROJECT" --target codex
assert_status 0 "untracked marker check"
assert_contains "installed codex: drift" "untracked markers are classified as drift"
OAW_UNTRACKED_BEFORE=$(artifact_snapshot "$OAW_UNTRACKED_TARGET")

run_oaw update --project "$OAW_PROJECT" --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "untracked marker update unexpectedly succeeded"
assert_contains "codex" "untracked marker update identifies the target"
assert_contains "$OAW_UNTRACKED_TARGET" "untracked marker update identifies the destination"
assert_artifact_snapshot "$OAW_UNTRACKED_TARGET" "$OAW_UNTRACKED_BEFORE" \
  "untracked marker update"
[ ! -e "$OAW_UNTRACKED_STATE" ] || fail "untracked marker update created installation state"
[ ! -e "$OAW_POLICY" ] || fail "untracked marker update created the canonical policy"

run_oaw uninstall --project "$OAW_PROJECT" --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "untracked marker uninstall unexpectedly succeeded"
assert_contains "codex" "untracked marker uninstall identifies the target"
assert_contains "$OAW_UNTRACKED_TARGET" "untracked marker uninstall identifies the destination"
assert_artifact_snapshot "$OAW_UNTRACKED_TARGET" "$OAW_UNTRACKED_BEFORE" \
  "untracked marker uninstall"
[ ! -e "$OAW_UNTRACKED_STATE" ] || fail "untracked marker uninstall created installation state"
[ ! -e "$OAW_POLICY" ] || fail "untracked marker uninstall created the canonical policy"

pass "untracked OAW markers block mutation without creating policy or state"

setup_cross_scope_candidate_fixture() {
  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_PROJECT="$OAW_SANDBOX/project with spaces"
  mkdir -p "$OAW_PROJECT"
  run_oaw install --target codex
  assert_status 0 "cross-scope candidate user fixture install"
  run_oaw install --project "$OAW_PROJECT" --target cursor
  assert_status 0 "cross-scope candidate project fixture install"

  OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
  OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
  OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
  OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
  OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
  OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
}

assert_cross_scope_candidate_unchanged() {
  local description=$1
  local policy_before=$2
  local user_target_before=$3
  local user_state_before=$4
  local project_target_before=$5
  local project_state_before=$6

  assert_artifact_snapshot "$OAW_POLICY" "$policy_before" "$description"
  assert_artifact_snapshot "$OAW_USER_TARGET" "$user_target_before" "$description"
  assert_artifact_snapshot "$OAW_USER_STATE" "$user_state_before" "$description"
  assert_artifact_snapshot "$OAW_PROJECT_TARGET" "$project_target_before" "$description"
  assert_artifact_snapshot "$OAW_PROJECT_STATE" "$project_state_before" "$description"
}

setup_cross_scope_candidate_fixture
printf 'drifted candidate adapter\n' >"$OAW_PROJECT_TARGET"
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "drifted candidate check"
assert_contains "installed cursor: drift" "drifted candidate check reports drift"
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")
OAW_USER_TARGET_BEFORE=$(artifact_snapshot "$OAW_USER_TARGET")
OAW_USER_STATE_BEFORE=$(artifact_snapshot "$OAW_USER_STATE")
OAW_PROJECT_TARGET_BEFORE=$(artifact_snapshot "$OAW_PROJECT_TARGET")
OAW_PROJECT_STATE_BEFORE=$(artifact_snapshot "$OAW_PROJECT_STATE")

run_oaw update --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "cross-scope update accepted a drifted candidate state"
assert_contains "cursor" "cross-scope update identifies the drifted candidate target"
assert_contains "$OAW_PROJECT_TARGET" \
  "cross-scope update identifies the drifted candidate destination"
assert_cross_scope_candidate_unchanged "cross-scope update with drifted candidate" \
  "$OAW_POLICY_BEFORE" "$OAW_USER_TARGET_BEFORE" "$OAW_USER_STATE_BEFORE" \
  "$OAW_PROJECT_TARGET_BEFORE" "$OAW_PROJECT_STATE_BEFORE"

pass "cross-scope policy synchronization rejects a drifted live candidate"

setup_cross_scope_candidate_fixture
printf 'drifted retention candidate\n' >"$OAW_PROJECT_TARGET"
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")
OAW_USER_TARGET_BEFORE=$(artifact_snapshot "$OAW_USER_TARGET")
OAW_USER_STATE_BEFORE=$(artifact_snapshot "$OAW_USER_STATE")
OAW_PROJECT_TARGET_BEFORE=$(artifact_snapshot "$OAW_PROJECT_TARGET")
OAW_PROJECT_STATE_BEFORE=$(artifact_snapshot "$OAW_PROJECT_STATE")

run_oaw uninstall --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "final uninstall accepted a drifted policy-retention candidate"
assert_contains "cursor" "final uninstall identifies the drifted retention target"
assert_contains "$OAW_PROJECT_TARGET" \
  "final uninstall identifies the drifted retention destination"
assert_cross_scope_candidate_unchanged "final uninstall with drifted retention candidate" \
  "$OAW_POLICY_BEFORE" "$OAW_USER_TARGET_BEFORE" "$OAW_USER_STATE_BEFORE" \
  "$OAW_PROJECT_TARGET_BEFORE" "$OAW_PROJECT_STATE_BEFORE"

pass "final uninstall rejects a drifted policy-retention candidate"

setup_forged_candidate_fixture() {
  local policy_checksum=
  local source_version=

  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_PROJECT="$OAW_SANDBOX/forged project"
  mkdir -p "$OAW_PROJECT"
  run_oaw install --target codex
  assert_status 0 "forged candidate user fixture install"

  OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
  OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
  OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
  OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
  OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
  OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
  policy_checksum=$(cksum <"$OAW_POLICY" | awk '{ print $1 ":" $2 }')
  IFS= read -r source_version <"$OAW_REPOSITORY/VERSION"
  mkdir -p "$(dirname -- "$OAW_PROJECT_STATE")"
  {
    printf 'format\t1\n'
    printf 'version\t%s\n' "$source_version"
    printf 'scope\tproject\n'
    printf 'project\t%s\n' "$OAW_PROJECT_PHYSICAL"
    printf 'policy\t%s\t%s\n' "$OAW_POLICY" "$policy_checksum"
    printf 'target\tcursor\t%s\towned-file\t1:1\tcreated-file\n' "$OAW_PROJECT_TARGET"
  } >"$OAW_PROJECT_STATE"
}

assert_forged_candidate_unchanged() {
  local description=$1
  local policy_before=$2
  local user_target_before=$3
  local user_state_before=$4
  local project_state_before=$5

  assert_artifact_snapshot "$OAW_POLICY" "$policy_before" "$description"
  assert_artifact_snapshot "$OAW_USER_TARGET" "$user_target_before" "$description"
  assert_artifact_snapshot "$OAW_USER_STATE" "$user_state_before" "$description"
  assert_artifact_snapshot "$OAW_PROJECT_TARGET" absent "$description"
  assert_artifact_snapshot "$OAW_PROJECT_STATE" "$project_state_before" "$description"
}

setup_forged_candidate_fixture
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "forged candidate check"
assert_contains "installed cursor: drift" "forged non-live candidate is classified as drift"
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")
OAW_USER_TARGET_BEFORE=$(artifact_snapshot "$OAW_USER_TARGET")
OAW_USER_STATE_BEFORE=$(artifact_snapshot "$OAW_USER_STATE")
OAW_PROJECT_STATE_BEFORE=$(artifact_snapshot "$OAW_PROJECT_STATE")

run_oaw update --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "cross-scope update synchronized a forged non-live candidate"
assert_contains "cursor" "forged update identifies the missing candidate target"
assert_contains "$OAW_PROJECT_TARGET" "forged update identifies the missing candidate destination"
assert_forged_candidate_unchanged "cross-scope update with forged candidate" \
  "$OAW_POLICY_BEFORE" "$OAW_USER_TARGET_BEFORE" "$OAW_USER_STATE_BEFORE" \
  "$OAW_PROJECT_STATE_BEFORE"

pass "cross-scope update never synchronizes a forged non-live candidate"

setup_forged_candidate_fixture
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")
OAW_USER_TARGET_BEFORE=$(artifact_snapshot "$OAW_USER_TARGET")
OAW_USER_STATE_BEFORE=$(artifact_snapshot "$OAW_USER_STATE")
OAW_PROJECT_STATE_BEFORE=$(artifact_snapshot "$OAW_PROJECT_STATE")

run_oaw uninstall --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "final uninstall retained policy for a forged non-live candidate"
assert_contains "cursor" "forged retention identifies the missing candidate target"
assert_contains "$OAW_PROJECT_TARGET" \
  "forged retention identifies the missing candidate destination"
assert_forged_candidate_unchanged "final uninstall with forged candidate" \
  "$OAW_POLICY_BEFORE" "$OAW_USER_TARGET_BEFORE" "$OAW_USER_STATE_BEFORE" \
  "$OAW_PROJECT_STATE_BEFORE"

pass "forged non-live candidate cannot retain policy through a successful uninstall"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_PROJECT_ONE="$OAW_SANDBOX/project one"
OAW_PROJECT_TWO="$OAW_SANDBOX/project two"
mkdir -p "$OAW_PROJECT_ONE" "$OAW_PROJECT_TWO"
run_oaw install --target codex
assert_status 0 "multi-candidate user fixture install"
run_oaw install --project "$OAW_PROJECT_ONE" --target cursor
assert_status 0 "multi-candidate first project install"
run_oaw install --project "$OAW_PROJECT_TWO" --target cursor
assert_status 0 "multi-candidate second project install"

OAW_PROJECT_ONE_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT_ONE" && pwd -P)
OAW_PROJECT_TWO_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT_TWO" && pwd -P)
OAW_PROJECT_ONE_ID=$(printf '%s' "$OAW_PROJECT_ONE_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_TWO_ID=$(printf '%s' "$OAW_PROJECT_TWO_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_ONE_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ONE_ID.state
OAW_PROJECT_TWO_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_TWO_ID.state
OAW_PROJECT_ONE_TARGET=$OAW_PROJECT_ONE_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_TWO_TARGET=$OAW_PROJECT_TWO_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
if [[ "$OAW_PROJECT_ONE_STATE" < "$OAW_PROJECT_TWO_STATE" ]]; then
  OAW_EARLY_TARGET=$OAW_PROJECT_ONE_TARGET
  OAW_EARLY_STATE=$OAW_PROJECT_ONE_STATE
  OAW_LATE_TARGET=$OAW_PROJECT_TWO_TARGET
  OAW_LATE_STATE=$OAW_PROJECT_TWO_STATE
else
  OAW_EARLY_TARGET=$OAW_PROJECT_TWO_TARGET
  OAW_EARLY_STATE=$OAW_PROJECT_TWO_STATE
  OAW_LATE_TARGET=$OAW_PROJECT_ONE_TARGET
  OAW_LATE_STATE=$OAW_PROJECT_ONE_STATE
fi
printf 'drifted later retention candidate\n' >"$OAW_LATE_TARGET"
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")
OAW_USER_TARGET_BEFORE=$(artifact_snapshot "$OAW_USER_TARGET")
OAW_USER_STATE_BEFORE=$(artifact_snapshot "$OAW_USER_STATE")
OAW_EARLY_TARGET_BEFORE=$(artifact_snapshot "$OAW_EARLY_TARGET")
OAW_EARLY_STATE_BEFORE=$(artifact_snapshot "$OAW_EARLY_STATE")
OAW_LATE_TARGET_BEFORE=$(artifact_snapshot "$OAW_LATE_TARGET")
OAW_LATE_STATE_BEFORE=$(artifact_snapshot "$OAW_LATE_STATE")

run_oaw uninstall --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "retention preflight stopped before a later drifted candidate"
assert_contains "cursor" "retention preflight identifies the later drifted target"
assert_contains "$OAW_LATE_TARGET" "retention preflight identifies the later drifted destination"
assert_artifact_snapshot "$OAW_POLICY" "$OAW_POLICY_BEFORE" "multi-candidate uninstall"
assert_artifact_snapshot "$OAW_USER_TARGET" "$OAW_USER_TARGET_BEFORE" "multi-candidate uninstall"
assert_artifact_snapshot "$OAW_USER_STATE" "$OAW_USER_STATE_BEFORE" "multi-candidate uninstall"
assert_artifact_snapshot "$OAW_EARLY_TARGET" "$OAW_EARLY_TARGET_BEFORE" \
  "multi-candidate uninstall"
assert_artifact_snapshot "$OAW_EARLY_STATE" "$OAW_EARLY_STATE_BEFORE" \
  "multi-candidate uninstall"
assert_artifact_snapshot "$OAW_LATE_TARGET" "$OAW_LATE_TARGET_BEFORE" \
  "multi-candidate uninstall"
assert_artifact_snapshot "$OAW_LATE_STATE" "$OAW_LATE_STATE_BEFORE" \
  "multi-candidate uninstall"

pass "retention preflight validates every matching candidate before mutation"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_PROJECT="$OAW_SANDBOX/project with symlink"
OAW_OUTSIDE=$OAW_SANDBOX/outside
mkdir -p "$OAW_PROJECT" "$OAW_OUTSIDE/rules"
printf 'outside sentinel\n' >"$OAW_OUTSIDE/sentinel"
OAW_OUTSIDE_SENTINEL_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE/sentinel")
ln -s "$OAW_OUTSIDE" "$OAW_PROJECT/.cursor"
OAW_OUTSIDE_TARGET=$OAW_OUTSIDE/rules/open-agent-workflow.mdc
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md

run_oaw install --project "$OAW_PROJECT" --target cursor
[ "$OAW_STATUS" -ne 0 ] || fail "symlinked project component allowed an outside write"
assert_contains "$OAW_PROJECT_PHYSICAL/.cursor" \
  "symlinked project component diagnostic identifies the unsafe component"
[ ! -e "$OAW_OUTSIDE_TARGET" ] || fail "symlinked project component created an outside adapter"
assert_artifact_snapshot "$OAW_OUTSIDE/sentinel" "$OAW_OUTSIDE_SENTINEL_BEFORE" \
  "symlinked project component install"
[ ! -e "$OAW_POLICY" ] || fail "symlinked project component install created policy before preflight"
[ ! -e "$OAW_PROJECT_STATE" ] ||
  fail "symlinked project component install created state before preflight"

pass "project intermediate symlinks cannot redirect adapter writes"

assert_project_final_symlink_rejected() {
  local target_id=$1
  local relative_target=$2
  local link_scope=$3
  local project_physical=
  local project_id=
  local project_target=
  local link_destination=
  local sentinel_before=
  local project_state=
  local policy_path=

  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_PROJECT="$OAW_SANDBOX/project with final symlink"
  mkdir -p "$OAW_PROJECT/$(dirname -- "$relative_target")" "$OAW_SANDBOX/outside"
  project_physical=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
  project_target=$project_physical/$relative_target
  case "$link_scope" in
    inside) link_destination=$project_physical/link-sentinel ;;
    outside) link_destination=$OAW_SANDBOX/outside/link-sentinel ;;
    *) fail "unknown final symlink scope: $link_scope" ;;
  esac
  printf 'final symlink sentinel\n' >"$link_destination"
  sentinel_before=$(artifact_snapshot "$link_destination")
  ln -s "$link_destination" "$project_target"
  project_id=$(printf '%s' "$project_physical" | cksum | awk '{ print $1 "-" $2 }')
  project_state=$OAW_STATE/open-agent-workflow/installations/projects/$project_id.state
  policy_path=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md

  run_oaw install --project "$OAW_PROJECT" --target "$target_id"
  [ "$OAW_STATUS" -ne 0 ] || fail "$target_id accepted a final $link_scope symlink"
  assert_contains "$project_target" "$target_id final symlink diagnostic identifies the target"
  [ -L "$project_target" ] || fail "$target_id final symlink was replaced"
  assert_artifact_snapshot "$link_destination" "$sentinel_before" \
    "$target_id final symlink install"
  [ ! -e "$policy_path" ] || fail "$target_id final symlink install created policy"
  [ ! -e "$project_state" ] || fail "$target_id final symlink install created state"
}

assert_project_final_symlink_rejected cursor .cursor/rules/open-agent-workflow.mdc outside
assert_project_final_symlink_rejected codex AGENTS.md outside
assert_project_final_symlink_rejected cursor .cursor/rules/open-agent-workflow.mdc inside

pass "project final symlinks are rejected inside and outside scope"

run_oaw_with_roots() {
  local custom_home=$1
  local custom_config=$2
  local custom_state=$3
  shift 3

  OAW_OUTPUT_FILE=$OAW_SANDBOX/output
  set +e
  (
    cd "$OAW_SANDBOX"
    HOME="$custom_home" \
      XDG_CONFIG_HOME="$custom_config" \
      XDG_STATE_HOME="$custom_state" \
      PATH="$OAW_PATH" \
      bash "$OAW_INSTALLER" "$@"
  ) >"$OAW_OUTPUT_FILE" 2>&1
  OAW_STATUS=$?
  set -e
  OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
}

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_CONTROL_HOME=$(printf '%s\nbad' "$OAW_HOME")
run_oaw_with_roots "$OAW_CONTROL_HOME" "$OAW_CONFIG" "$OAW_STATE" \
  install --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "control-character HOME was accepted"
assert_contains "control characters" "control-character HOME diagnostic is explicit"
assert_empty_dir "$OAW_CONFIG" "control-character HOME must not create policy"
assert_empty_dir "$OAW_STATE" "control-character HOME must not create state"
[ ! -e "$OAW_CONTROL_HOME/.codex/AGENTS.md" ] ||
  fail "control-character HOME created a Codex target"

pass "control characters in consumed roots fail before mutation"

for OAW_ROOT_CASE in relative-home relative-config relative-state control-config control-state; do
  cleanup_sandbox
  setup_sandbox
  OAW_INSTALLER=$OAW_BASE_INSTALLER
  OAW_CUSTOM_HOME=$OAW_HOME
  OAW_CUSTOM_CONFIG=$OAW_CONFIG
  OAW_CUSTOM_STATE=$OAW_STATE
  OAW_ROOT_ERROR='root must be an absolute path'
  case "$OAW_ROOT_CASE" in
    relative-home) OAW_CUSTOM_HOME=relative-home ;;
    relative-config) OAW_CUSTOM_CONFIG=relative-config ;;
    relative-state) OAW_CUSTOM_STATE=relative-state ;;
    control-config)
      OAW_CUSTOM_CONFIG=$(printf '%s\tbad' "$OAW_CONFIG")
      OAW_ROOT_ERROR='XDG_CONFIG_HOME contains control characters'
      ;;
    control-state)
      OAW_CUSTOM_STATE=$(printf '%s\rbad' "$OAW_STATE")
      OAW_ROOT_ERROR='XDG_STATE_HOME contains control characters'
      ;;
    *) fail "unknown hostile root fixture: $OAW_ROOT_CASE" ;;
  esac

  run_oaw_with_roots "$OAW_CUSTOM_HOME" "$OAW_CUSTOM_CONFIG" "$OAW_CUSTOM_STATE" \
    install --target codex
  [ "$OAW_STATUS" -ne 0 ] || fail "$OAW_ROOT_CASE was accepted"
  assert_contains "$OAW_ROOT_ERROR" "$OAW_ROOT_CASE diagnostic"
  assert_empty_dir "$OAW_CONFIG" "$OAW_ROOT_CASE must not change the normal config root"
  assert_empty_dir "$OAW_STATE" "$OAW_ROOT_CASE must not change the normal state root"
  [ ! -e "$OAW_HOME/.codex/AGENTS.md" ] || fail "$OAW_ROOT_CASE created a Codex target"
  [ ! -e "$OAW_SANDBOX/relative-home" ] || fail "$OAW_ROOT_CASE created a relative HOME"
  [ ! -e "$OAW_SANDBOX/relative-config" ] || fail "$OAW_ROOT_CASE created a relative config root"
  [ ! -e "$OAW_SANDBOX/relative-state" ] || fail "$OAW_ROOT_CASE created a relative state root"
done

pass "relative and control-character HOME/XDG roots fail before mutation"

run_oaw_with_codex_home() {
  local codex_home=$1
  shift

  OAW_OUTPUT_FILE=$OAW_SANDBOX/output
  set +e
  HOME="$OAW_HOME" \
    XDG_CONFIG_HOME="$OAW_CONFIG" \
    XDG_STATE_HOME="$OAW_STATE" \
    CODEX_HOME="$codex_home" \
    PATH="$OAW_PATH" \
    bash "$OAW_INSTALLER" "$@" >"$OAW_OUTPUT_FILE" 2>&1
  OAW_STATUS=$?
  set -e
  OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
}

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_IGNORED_CODEX_HOME=$OAW_SANDBOX/outside-codex-home
mkdir -p "$OAW_IGNORED_CODEX_HOME"
printf 'CODEX_HOME sentinel\n' >"$OAW_IGNORED_CODEX_HOME/sentinel"
OAW_CODEX_HOME_SENTINEL_BEFORE=$(artifact_snapshot "$OAW_IGNORED_CODEX_HOME/sentinel")
run_oaw_with_codex_home "$OAW_IGNORED_CODEX_HOME" install --target codex
assert_status 0 "user Codex install ignores CODEX_HOME"
[ -f "$OAW_HOME/.codex/AGENTS.md" ] || fail "Codex user target did not use HOME"
assert_artifact_snapshot "$OAW_IGNORED_CODEX_HOME/sentinel" \
  "$OAW_CODEX_HOME_SENTINEL_BEFORE" "user Codex install"
[ ! -e "$OAW_IGNORED_CODEX_HOME/AGENTS.md" ] || fail "user install wrote through CODEX_HOME"

OAW_PROJECT="$OAW_SANDBOX/project through parent/segment/../real project"
mkdir -p "$OAW_SANDBOX/project through parent/segment" \
  "$OAW_SANDBOX/project through parent/real project"
run_oaw_with_codex_home "$OAW_IGNORED_CODEX_HOME" install --project "$OAW_PROJECT" --target codex
assert_status 0 "project path through parent traversal resolves physically"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
[ -f "$OAW_PROJECT_PHYSICAL/AGENTS.md" ] || fail "project Codex target missed the physical root"
assert_artifact_snapshot "$OAW_IGNORED_CODEX_HOME/sentinel" \
  "$OAW_CODEX_HOME_SENTINEL_BEFORE" "project Codex install"
[ ! -e "$OAW_IGNORED_CODEX_HOME/AGENTS.md" ] || fail "project install wrote through CODEX_HOME"

pass "CODEX_HOME is ignored and parent traversal resolves to the physical project root"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_OUTSIDE=$OAW_SANDBOX/outside-user-target
mkdir -p "$OAW_OUTSIDE"
printf 'outside user sentinel\n' >"$OAW_OUTSIDE/sentinel"
OAW_OUTSIDE_SENTINEL_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE/sentinel")
ln -s "$OAW_OUTSIDE" "$OAW_HOME/.codex"
OAW_OUTSIDE_TARGET=$OAW_OUTSIDE/AGENTS.md
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state

run_oaw install --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "symlinked HOME target component allowed an outside write"
assert_contains "$OAW_HOME/.codex" \
  "symlinked HOME target diagnostic identifies the unsafe component"
[ ! -e "$OAW_OUTSIDE_TARGET" ] || fail "symlinked HOME target created an outside adapter"
assert_artifact_snapshot "$OAW_OUTSIDE/sentinel" "$OAW_OUTSIDE_SENTINEL_BEFORE" \
  "symlinked HOME target install"
[ ! -e "$OAW_POLICY" ] || fail "symlinked HOME target install created policy"
[ ! -e "$OAW_USER_STATE" ] || fail "symlinked HOME target install created state"

pass "user target components cannot redirect writes through symlinks"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_OUTSIDE=$OAW_SANDBOX/outside-policy
mkdir -p "$OAW_OUTSIDE"
printf 'outside policy sentinel\n' >"$OAW_OUTSIDE/sentinel"
OAW_OUTSIDE_SENTINEL_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE/sentinel")
ln -s "$OAW_OUTSIDE" "$OAW_CONFIG/open-agent-workflow"
OAW_OUTSIDE_POLICY=$OAW_OUTSIDE/ENGINEERING.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state

run_oaw install --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "symlinked policy directory allowed an outside write"
assert_contains "$OAW_CONFIG/open-agent-workflow" \
  "symlinked policy diagnostic identifies the unsafe component"
[ ! -e "$OAW_OUTSIDE_POLICY" ] || fail "symlinked policy directory created an outside policy"
assert_artifact_snapshot "$OAW_OUTSIDE/sentinel" "$OAW_OUTSIDE_SENTINEL_BEFORE" \
  "symlinked policy install"
[ ! -e "$OAW_USER_TARGET" ] || fail "symlinked policy install created a user target"
[ ! -e "$OAW_USER_STATE" ] || fail "symlinked policy install created state"

pass "XDG policy components cannot redirect writes through symlinks"

cleanup_sandbox
unset OAW_PATH
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_MODEL_BIN=$OAW_SANDBOX/model-bin
OAW_MODEL_SENTINEL=$OAW_SANDBOX/model-executed
mkdir -p "$OAW_MODEL_BIN"
export OAW_MODEL_SENTINEL
for model_command in codex claude gemini opencode; do
  {
    printf '%s\n' '#!/usr/bin/env bash'
    printf '%s\n' 'printf "%s\n" "$0" >>"$OAW_MODEL_SENTINEL"'
    printf '%s\n' 'exit 99'
  } >"$OAW_MODEL_BIN/$model_command"
  chmod 755 "$OAW_MODEL_BIN/$model_command"
done
OAW_PATH=$OAW_MODEL_BIN:$PATH

run_oaw check --target claude,codex,gemini,opencode
assert_status 0 "management check remains diagnostic with model commands on PATH"
run_oaw install --target claude,codex,gemini,opencode --dry-run
assert_status 0 "management dry run does not execute model commands"
[ ! -e "$OAW_MODEL_SENTINEL" ] ||
  fail "management commands executed a model CLI: $(cat "$OAW_MODEL_SENTINEL")"

pass "management commands never execute detected model CLIs"
