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

setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "independent user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "independent project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_USER_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_USER_POLICY_BEFORE=$(file_fingerprint "$OAW_USER_POLICY")
OAW_USER_TARGET_BEFORE=$(file_fingerprint "$OAW_USER_TARGET")
OAW_USER_STATE_BEFORE=$(file_fingerprint "$OAW_USER_STATE")

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/project-update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.1-project-independent\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nPROJECT POLICY SET UPDATE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/POLICY.md"
build_checkout_installer "$OAW_UPDATE_CHECKOUT"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw update --project "$OAW_PROJECT" --target cursor
assert_status 0 "project update with an installed user scope"
grep -F 'PROJECT POLICY SET UPDATE SENTINEL' "$OAW_PROJECT_POLICY" >/dev/null ||
  fail "project update did not use the checkout Policy Set"
grep -F "$(printf 'version\t0.1.1-project-independent')" "$OAW_PROJECT_STATE" >/dev/null ||
  fail "project update did not record the project version"
[ "$(file_fingerprint "$OAW_USER_POLICY")" = "$OAW_USER_POLICY_BEFORE" ] ||
  fail "project update changed the user Policy"
[ "$(file_fingerprint "$OAW_USER_TARGET")" = "$OAW_USER_TARGET_BEFORE" ] ||
  fail "project update changed the user adapter"
[ "$(file_fingerprint "$OAW_USER_STATE")" = "$OAW_USER_STATE_BEFORE" ] ||
  fail "project update changed the user state"

run_oaw check --target codex
assert_status 0 "user check after project update"
assert_contains "installed codex: clean" "project update keeps the user scope clean"
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check after project update"
assert_contains "installed cursor: clean" "project update keeps the project scope clean"

pass "project update is independent from the user Policy lifecycle"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "reverse independent user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "reverse independent project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_USER_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_POLICY_DIR=$OAW_PROJECT_PHYSICAL/.oaw/policy
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_PROJECT_POLICY_BEFORE=$OAW_SANDBOX/project-policy.before
snapshot_tree "$OAW_PROJECT_POLICY_DIR" >"$OAW_PROJECT_POLICY_BEFORE"
OAW_PROJECT_TARGET_BEFORE=$(file_fingerprint "$OAW_PROJECT_TARGET")
OAW_PROJECT_STATE_BEFORE=$(file_fingerprint "$OAW_PROJECT_STATE")

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/user-update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.2-user-independent\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nUSER POLICY UPDATE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
build_checkout_installer "$OAW_UPDATE_CHECKOUT"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw update --target codex
assert_status 0 "user update with an installed project scope"
grep -F 'USER POLICY UPDATE SENTINEL' "$OAW_USER_POLICY" >/dev/null ||
  fail "user update did not use the checkout Policy"
grep -F "$(printf 'version\t0.1.2-user-independent')" "$OAW_USER_STATE" >/dev/null ||
  fail "user update did not record the user version"
snapshot_tree "$OAW_PROJECT_POLICY_DIR" >"$OAW_SANDBOX/project-policy.after"
cmp -s "$OAW_PROJECT_POLICY_BEFORE" "$OAW_SANDBOX/project-policy.after" ||
  fail "user update changed the project Policy Set"
[ "$(file_fingerprint "$OAW_PROJECT_TARGET")" = "$OAW_PROJECT_TARGET_BEFORE" ] ||
  fail "user update changed the project adapter"
[ "$(file_fingerprint "$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "user update changed the project state"

run_oaw check --target codex
assert_status 0 "user check after user update"
assert_contains "installed codex: clean" "user update keeps the user scope clean"
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check after user update"
assert_contains "installed cursor: clean" "user update keeps the project scope clean"

pass "user update is independent from the project Policy Set lifecycle"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_BASE_INSTALLER
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "independent uninstall user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "independent uninstall project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_USER_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_POLICY_DIR=$OAW_PROJECT_PHYSICAL/.oaw/policy
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state

run_oaw uninstall --project "$OAW_PROJECT" --target cursor
assert_status 0 "project uninstall with an installed user scope"
[ ! -e "$OAW_PROJECT_POLICY_DIR" ] || fail "project uninstall retained its Policy Set"
[ ! -e "$OAW_PROJECT_TARGET" ] || fail "project uninstall retained its owned adapter"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "project uninstall retained its state"
[ -f "$OAW_USER_POLICY" ] || fail "project uninstall removed the user Policy"
[ -f "$OAW_USER_TARGET" ] || fail "project uninstall removed the user adapter"
[ -f "$OAW_USER_STATE" ] || fail "project uninstall removed the user state"

run_oaw check --target codex
assert_status 0 "user check after project uninstall"
assert_contains "installed codex: clean" "project uninstall keeps the user scope clean"

run_oaw uninstall --target codex
assert_status 0 "user uninstall after project removal"
[ ! -e "$OAW_USER_POLICY" ] || fail "user uninstall retained the user Policy"
[ ! -e "$OAW_USER_STATE" ] || fail "user uninstall retained the user state"

pass "user and project uninstall lifecycles are independent"
