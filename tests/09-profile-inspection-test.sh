#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM
setup_sandbox

write_profile() {
  profile_path=$1
  profile_id=$2
  profile_name=$3
  profile_body=${4:-}
  mkdir -p "$(dirname -- "$profile_path")"
  {
    printf '%s\n' '---' "id: $profile_id" "name: $profile_name" '---'
    printf '%s' "$profile_body"
  } >"$profile_path"
}

run_profile_oaw() {
  OAW_OUTPUT_FILE=$OAW_SANDBOX/output
  set +e
  (
    cd "$OAW_PROJECT"
    HOME="$OAW_HOME" \
      XDG_CONFIG_HOME="$OAW_CONFIG" \
      XDG_STATE_HOME="$OAW_STATE" \
      PATH="$OAW_PATH" \
      bash "$OAW_INSTALLER" "$@"
  ) >"$OAW_OUTPUT_FILE" 2>&1
  OAW_STATUS=$?
  set -e
  OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
}

OAW_PROJECT_PROFILE=$OAW_PROJECT/.oaw/profiles/project-shared.md
OAW_USER_PROFILE=$OAW_CONFIG/open-agent-workflow/profiles/user-shared.md
OAW_PARTIAL_PROFILE=$OAW_PROJECT/.oaw/profiles/partial.md
write_profile "$OAW_PROJECT_PROFILE" shared 'Project Shared' '
PROJECT PROFILE BODY
'
write_profile "$OAW_USER_PROFILE" shared 'User Shared' '
USER PROFILE BODY
'
write_profile "$OAW_PARTIAL_PROFILE" partial Partial '
## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| Implementation and TDD | `tdd` |
| Unknown | `other` |
'

snapshot_tree "$OAW_PROJECT" >"$OAW_SANDBOX/project.before"
snapshot_tree "$OAW_CONFIG" >"$OAW_SANDBOX/config.before"

run_profile_oaw profile list
assert_status 0 "Profile list with a cross-scope identity"
for expected in \
  'Profile inspection is advisory' \
  'built-in:SP-FULL' \
  'project:shared' \
  'user:shared' \
  'PROFILE_ID_CROSS_SCOPE'; do
  assert_contains "$expected" "Profile list reports $expected"
done

run_profile_oaw profile show shared
assert_status 65 "unqualified cross-scope Profile show"
assert_contains 'source qualifier' "ambiguous Profile show explains source qualification"

run_profile_oaw profile show project:shared
assert_status 0 "qualified project Profile show"
assert_contains 'source: project' "qualified Profile show reports source"
assert_contains 'PROJECT PROFILE BODY' "qualified Profile show returns normative Markdown"

run_profile_oaw profile check partial
assert_status 0 "partial Profile advisory check"
assert_contains 'result: metadata-valid' "partial Profile keeps valid identity metadata"
assert_contains 'responsibilities: 1/8' "partial Profile reports declared Responsibilities"
assert_contains 'Policy defaults cover omitted Responsibilities' "partial Profile preserves Policy defaults"
assert_contains 'Skill availability: not evaluated' "Profile check does not decide Skill usability"
assert_contains 'PROFILE_BODY_WARNING' "Profile check reports body diagnostics as warnings"

snapshot_tree "$OAW_PROJECT" >"$OAW_SANDBOX/project.after"
snapshot_tree "$OAW_CONFIG" >"$OAW_SANDBOX/config.after"
cmp -s "$OAW_SANDBOX/project.before" "$OAW_SANDBOX/project.after" ||
  fail "Profile inspection changed the project tree"
cmp -s "$OAW_SANDBOX/config.before" "$OAW_SANDBOX/config.after" ||
  fail "Profile inspection changed the user Profile tree"
assert_empty_dir "$OAW_STATE" "Profile inspection must not create state"

write_profile "$OAW_PROJECT/.oaw/profiles/duplicate-one.md" duplicate One
write_profile "$OAW_PROJECT/.oaw/profiles/duplicate-two.md" duplicate Two
write_profile "$OAW_USER_PROFILE" SP-FULL Reserved
OAW_MALFORMED_PROFILE=$OAW_CONFIG/open-agent-workflow/profiles/malformed.md
printf '%s\n' '---' 'name: Missing ID' '---' >"$OAW_MALFORMED_PROFILE"

run_profile_oaw profile list
assert_status 0 "Profile list with structural diagnostics"
for diagnostic in PROFILE_ID_DUPLICATE PROFILE_METADATA_INVALID PROFILE_ID_RESERVED; do
  assert_contains "$diagnostic" "Profile list reports $diagnostic"
done

run_profile_oaw profile check "$OAW_MALFORMED_PROFILE"
assert_status 65 "malformed Profile metadata check"
assert_contains 'PROFILE_METADATA_INVALID' "malformed Profile reports its required metadata error"

run_profile_oaw profile check user:SP-FULL
assert_status 65 "reserved built-in Profile ID check"
assert_contains 'PROFILE_SELECTION_INVALID' "reserved custom Profile cannot shadow a built-in"
assert_empty_dir "$OAW_STATE" "invalid Profile diagnostics must not create state"

pass "advisory Profile inspection covers built-in, project, and user sources"
