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
  OAW_INSTALLER=$OAW_REPOSITORY/install.sh
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
