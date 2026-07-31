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
assert_status 0 "cross-scope user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "cross-scope project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_USER_TARGET_BEFORE=$(file_fingerprint "$OAW_USER_TARGET")

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.1-policy-coordination\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nPROJECT UPDATE POLICY COORDINATION SENTINEL\n' \
  >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw update --project "$OAW_PROJECT" --target cursor
assert_status 0 "project update with an installed user scope"
OAW_POLICY_CHECKSUM=$(cksum <"$OAW_POLICY" | awk '{ print $1 ":" $2 }')
grep -F "$(printf 'version\t0.1.1-policy-coordination')" "$OAW_PROJECT_STATE" >/dev/null ||
  fail "project update did not record the new version"
grep -F "$(printf 'version\t0.1.1-policy-coordination')" "$OAW_USER_STATE" >/dev/null ||
  fail "project update did not synchronize the user state version"
grep -F "$(printf 'policy\t%s\t%s' "$OAW_POLICY" "$OAW_POLICY_CHECKSUM")" \
  "$OAW_USER_STATE" >/dev/null || fail "project update did not synchronize the user policy checksum"
[ "$(file_fingerprint "$OAW_USER_TARGET")" = "$OAW_USER_TARGET_BEFORE" ] ||
  fail "project update rewrote the user adapter"

run_oaw check --target codex
assert_status 0 "user check after project update"
assert_contains "installed codex: clean" "project update keeps the user scope clean"
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check after project update"
assert_contains "installed cursor: clean" "project update keeps the project scope clean"

pass "project update coordinates shared policy state without rewriting user adapters"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "reverse cross-scope user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "reverse cross-scope project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_PROJECT_TARGET_BEFORE=$(file_fingerprint "$OAW_PROJECT_TARGET")

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.2-reverse-coordination\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nUSER UPDATE POLICY COORDINATION SENTINEL\n' \
  >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw update --target codex
assert_status 0 "user update with an installed project scope"
OAW_POLICY_CHECKSUM=$(cksum <"$OAW_POLICY" | awk '{ print $1 ":" $2 }')
grep -F "$(printf 'version\t0.1.2-reverse-coordination')" "$OAW_USER_STATE" >/dev/null ||
  fail "user update did not record the new version"
grep -F "$(printf 'version\t0.1.2-reverse-coordination')" "$OAW_PROJECT_STATE" >/dev/null ||
  fail "user update did not synchronize the project state version"
grep -F "$(printf 'policy\t%s\t%s' "$OAW_POLICY" "$OAW_POLICY_CHECKSUM")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "user update did not synchronize the project policy checksum"
[ "$(file_fingerprint "$OAW_PROJECT_TARGET")" = "$OAW_PROJECT_TARGET_BEFORE" ] ||
  fail "user update rewrote the project adapter"

run_oaw check --target codex
assert_status 0 "user check after user update"
assert_contains "installed codex: clean" "user update keeps the user scope clean"
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "project check after user update"
assert_contains "installed cursor: clean" "user update keeps the project scope clean"

pass "user update coordinates shared policy state without rewriting project adapters"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "new-scope coordination user fixture install"
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_USER_TARGET_BEFORE=$(file_fingerprint "$OAW_USER_TARGET")

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.3-new-scope-coordination\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nNEW PROJECT SCOPE POLICY COORDINATION SENTINEL\n' \
  >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "new project scope install coordinates the user state"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY_CHECKSUM=$(cksum <"$OAW_POLICY" | awk '{ print $1 ":" $2 }')
for OAW_COORDINATED_STATE in "$OAW_USER_STATE" "$OAW_PROJECT_STATE"; do
  grep -F "$(printf 'version\t0.1.3-new-scope-coordination')" \
    "$OAW_COORDINATED_STATE" >/dev/null || fail "new project scope did not synchronize every state version"
  grep -F "$(printf 'policy\t%s\t%s' "$OAW_POLICY" "$OAW_POLICY_CHECKSUM")" \
    "$OAW_COORDINATED_STATE" >/dev/null || fail "new project scope did not synchronize every policy checksum"
done
[ "$(file_fingerprint "$OAW_USER_TARGET")" = "$OAW_USER_TARGET_BEFORE" ] ||
  fail "new project scope install rewrote the user adapter"

run_oaw check --target codex
assert_status 0 "user check after new project scope install"
assert_contains "installed codex: clean" "new project scope keeps the user scope clean"

pass "new project scope synchronizes existing canonical policy references"

OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_POLICY_BEFORE=$(file_fingerprint "$OAW_POLICY")
OAW_USER_STATE_BEFORE=$(file_fingerprint "$OAW_USER_STATE")
OAW_PROJECT_STATE_BEFORE=$(file_fingerprint "$OAW_PROJECT_STATE")
OAW_USER_TARGET_BEFORE=$(file_fingerprint "$OAW_USER_TARGET")
OAW_PROJECT_TARGET_BEFORE=$(file_fingerprint "$OAW_PROJECT_TARGET")
printf '0.1.4-coordination-dry-run\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nCROSS SCOPE DRY RUN SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"

run_oaw update --target codex --dry-run
assert_status 0 "cross-scope policy update dry run"
assert_contains "would-update" "cross-scope dry run reports prepared replacements"
[ "$(file_fingerprint "$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "cross-scope dry run changed the canonical policy"
[ "$(file_fingerprint "$OAW_USER_STATE")" = "$OAW_USER_STATE_BEFORE" ] ||
  fail "cross-scope dry run changed the user state"
[ "$(file_fingerprint "$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "cross-scope dry run changed the project state"
[ "$(file_fingerprint "$OAW_USER_TARGET")" = "$OAW_USER_TARGET_BEFORE" ] ||
  fail "cross-scope dry run changed the user adapter"
[ "$(file_fingerprint "$OAW_PROJECT_TARGET")" = "$OAW_PROJECT_TARGET_BEFORE" ] ||
  fail "cross-scope dry run changed the project adapter"

pass "cross-scope policy dry run preserves every managed fingerprint"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "path-reference user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "path-reference project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_OLDER_USER_STATE=$OAW_SANDBOX/older-user.state
cp "$OAW_USER_STATE" "$OAW_OLDER_USER_STATE"
OAW_USER_TARGET_BEFORE=$(file_fingerprint "$OAW_USER_TARGET")

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.5-path-reference\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nPATH REFERENCE RETENTION SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh
run_oaw update --project "$OAW_PROJECT" --target cursor
assert_status 0 "path-reference project update"
cp "$OAW_OLDER_USER_STATE" "$OAW_USER_STATE"

run_oaw uninstall --project "$OAW_PROJECT" --target cursor
assert_status 0 "project uninstall with an older valid user policy reference"
[ -f "$OAW_POLICY" ] || fail "project uninstall removed a policy still referenced by user state"
[ -f "$OAW_USER_STATE" ] || fail "project uninstall removed the remaining user state"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "project uninstall kept its final project state"
[ "$(file_fingerprint "$OAW_USER_TARGET")" = "$OAW_USER_TARGET_BEFORE" ] ||
  fail "project uninstall changed the remaining user adapter"

pass "uninstall retains the canonical policy for every valid path reference"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "clean final-reference user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "clean final-reference project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.6-final-reference\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nFINAL POLICY REFERENCE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh

run_oaw update --project "$OAW_PROJECT" --target cursor
assert_status 0 "clean final-reference project update"
run_oaw uninstall --project "$OAW_PROJECT" --target cursor
assert_status 0 "updated project final uninstall"
[ -f "$OAW_POLICY" ] || fail "updated project uninstall removed the user-referenced policy"
[ -f "$OAW_USER_STATE" ] || fail "updated project uninstall removed the user state"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "updated project uninstall kept the project state"
run_oaw check --target codex
assert_status 0 "remaining user check after project uninstall"
assert_contains "installed codex: clean" "remaining user scope stays clean after project uninstall"

run_oaw uninstall --target codex
assert_status 0 "final user reference uninstall"
[ ! -e "$OAW_POLICY" ] || fail "final user uninstall retained the canonical policy"
[ ! -e "$OAW_USER_STATE" ] || fail "final user uninstall retained the final state"

pass "canonical policy survives until the final clean scope reference"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT="$OAW_SANDBOX/project with spaces"
mkdir -p "$OAW_PROJECT"

run_oaw install --target codex
assert_status 0 "stale-reference user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "stale-reference project fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_TAMPERED_STATE=$OAW_SANDBOX/tampered-user.state
awk -F '\t' 'BEGIN { OFS = "\t" } $1 == "policy" { $3 = "1:1" } { print }' \
  "$OAW_USER_STATE" >"$OAW_TAMPERED_STATE"
mv "$OAW_TAMPERED_STATE" "$OAW_USER_STATE"
OAW_POLICY_BEFORE=$(file_fingerprint "$OAW_POLICY")
OAW_USER_TARGET_BEFORE=$(file_fingerprint "$OAW_USER_TARGET")
OAW_PROJECT_TARGET_BEFORE=$(file_fingerprint "$OAW_PROJECT_TARGET")
OAW_USER_STATE_BEFORE=$(file_fingerprint "$OAW_USER_STATE")
OAW_PROJECT_STATE_BEFORE=$(file_fingerprint "$OAW_PROJECT_STATE")

OAW_UPDATE_CHECKOUT=$OAW_SANDBOX/update-checkout
cp -R "$OAW_REPOSITORY" "$OAW_UPDATE_CHECKOUT"
printf '0.1.7-stale-reference\n' >"$OAW_UPDATE_CHECKOUT/VERSION"
printf '\nSTALE POLICY REFERENCE SENTINEL\n' >>"$OAW_UPDATE_CHECKOUT/policy/ENGINEERING.md"
OAW_INSTALLER=$OAW_UPDATE_CHECKOUT/install.sh
run_oaw update --project "$OAW_PROJECT" --target cursor
assert_status 65 "stale cross-scope policy reference"
assert_contains "managed policy has drifted" "stale reference fails with a policy diagnostic"
[ "$(file_fingerprint "$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "stale reference changed the canonical policy"
[ "$(file_fingerprint "$OAW_USER_TARGET")" = "$OAW_USER_TARGET_BEFORE" ] ||
  fail "stale reference changed the user adapter"
[ "$(file_fingerprint "$OAW_PROJECT_TARGET")" = "$OAW_PROJECT_TARGET_BEFORE" ] ||
  fail "stale reference changed the project adapter"
[ "$(file_fingerprint "$OAW_USER_STATE")" = "$OAW_USER_STATE_BEFORE" ] ||
  fail "stale reference rewrote the user state"
[ "$(file_fingerprint "$OAW_PROJECT_STATE")" = "$OAW_PROJECT_STATE_BEFORE" ] ||
  fail "stale reference rewrote the project state"

pass "stale policy references fail before every managed write"
