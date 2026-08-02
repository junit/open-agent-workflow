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

setup_sandbox
OAW_OUTSIDE=$OAW_SANDBOX/outside-state
mkdir -p "$OAW_OUTSIDE"
printf 'outside state sentinel\n' >"$OAW_OUTSIDE/sentinel"
OAW_OUTSIDE_SENTINEL_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE/sentinel")
ln -s "$OAW_OUTSIDE" "$OAW_STATE/open-agent-workflow"
OAW_OUTSIDE_STATE=$OAW_OUTSIDE/installations/user.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md

run_oaw install --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "symlinked state directory allowed an outside write"
assert_contains "$OAW_STATE/open-agent-workflow" \
  "symlinked state diagnostic identifies the unsafe component"
[ ! -e "$OAW_OUTSIDE_STATE" ] || fail "symlinked state directory created an outside state"
assert_artifact_snapshot "$OAW_OUTSIDE/sentinel" "$OAW_OUTSIDE_SENTINEL_BEFORE" \
  "symlinked state install"
[ ! -e "$OAW_POLICY" ] || fail "symlinked state install created policy"
[ ! -e "$OAW_USER_TARGET" ] || fail "symlinked state install created a user target"

pass "XDG state components cannot redirect writes through symlinks"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with swapped final target"
OAW_OUTSIDE=$OAW_SANDBOX/outside-final-target
mkdir -p "$OAW_PROJECT" "$OAW_OUTSIDE"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "final symlink swap fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_OUTSIDE_TARGET=$OAW_OUTSIDE/open-agent-workflow.mdc
cp "$OAW_PROJECT_TARGET" "$OAW_OUTSIDE_TARGET"
rm -- "$OAW_PROJECT_TARGET"
ln -s "$OAW_OUTSIDE_TARGET" "$OAW_PROJECT_TARGET"
OAW_OUTSIDE_TARGET_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE_TARGET")
OAW_PROJECT_STATE_BEFORE=$(artifact_snapshot "$OAW_PROJECT_STATE")
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")

run_oaw update --project "$OAW_PROJECT" --target cursor
[ "$OAW_STATUS" -ne 0 ] || fail "update accepted a swapped final target symlink"
assert_contains "$OAW_PROJECT_TARGET" "update identifies the swapped final target symlink"
[ -L "$OAW_PROJECT_TARGET" ] || fail "update replaced the swapped final target symlink"
assert_artifact_snapshot "$OAW_OUTSIDE_TARGET" "$OAW_OUTSIDE_TARGET_BEFORE" \
  "update with swapped final target"
assert_artifact_snapshot "$OAW_PROJECT_STATE" "$OAW_PROJECT_STATE_BEFORE" \
  "update with swapped final target"
assert_artifact_snapshot "$OAW_POLICY" "$OAW_POLICY_BEFORE" \
  "update with swapped final target"

run_oaw uninstall --project "$OAW_PROJECT" --target cursor
[ "$OAW_STATUS" -ne 0 ] || fail "uninstall accepted a swapped final target symlink"
assert_contains "$OAW_PROJECT_TARGET" "uninstall identifies the swapped final target symlink"
[ -L "$OAW_PROJECT_TARGET" ] || fail "uninstall removed the swapped final target symlink"
assert_artifact_snapshot "$OAW_OUTSIDE_TARGET" "$OAW_OUTSIDE_TARGET_BEFORE" \
  "uninstall with swapped final target"
assert_artifact_snapshot "$OAW_PROJECT_STATE" "$OAW_PROJECT_STATE_BEFORE" \
  "uninstall with swapped final target"
assert_artifact_snapshot "$OAW_POLICY" "$OAW_POLICY_BEFORE" \
  "uninstall with swapped final target"

pass "installed final targets cannot be replaced by content-matching symlinks"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with swapped parent"
OAW_OUTSIDE=$OAW_SANDBOX/outside-parent
mkdir -p "$OAW_PROJECT" "$OAW_OUTSIDE/rules"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "parent symlink swap fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_ORIGINAL_CURSOR=$OAW_SANDBOX/original-cursor
mv "$OAW_PROJECT_PHYSICAL/.cursor" "$OAW_ORIGINAL_CURSOR"
cp "$OAW_ORIGINAL_CURSOR/rules/open-agent-workflow.mdc" \
  "$OAW_OUTSIDE/rules/open-agent-workflow.mdc"
ln -s "$OAW_OUTSIDE" "$OAW_PROJECT_PHYSICAL/.cursor"
OAW_OUTSIDE_TARGET=$OAW_OUTSIDE/rules/open-agent-workflow.mdc
OAW_OUTSIDE_TARGET_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE_TARGET")
OAW_PROJECT_STATE_BEFORE=$(artifact_snapshot "$OAW_PROJECT_STATE")
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")

run_oaw update --project "$OAW_PROJECT" --target cursor
[ "$OAW_STATUS" -ne 0 ] || fail "update accepted a swapped target parent symlink"
assert_contains "$OAW_PROJECT_PHYSICAL/.cursor" \
  "update identifies the swapped target parent symlink"
[ -L "$OAW_PROJECT_PHYSICAL/.cursor" ] || fail "update replaced the swapped parent symlink"
assert_artifact_snapshot "$OAW_OUTSIDE_TARGET" "$OAW_OUTSIDE_TARGET_BEFORE" \
  "update with swapped target parent"
assert_artifact_snapshot "$OAW_PROJECT_STATE" "$OAW_PROJECT_STATE_BEFORE" \
  "update with swapped target parent"
assert_artifact_snapshot "$OAW_POLICY" "$OAW_POLICY_BEFORE" \
  "update with swapped target parent"

run_oaw uninstall --project "$OAW_PROJECT" --target cursor
[ "$OAW_STATUS" -ne 0 ] || fail "uninstall accepted a swapped target parent symlink"
assert_contains "$OAW_PROJECT_PHYSICAL/.cursor" \
  "uninstall identifies the swapped target parent symlink"
[ -L "$OAW_PROJECT_PHYSICAL/.cursor" ] || fail "uninstall removed the swapped parent symlink"
assert_artifact_snapshot "$OAW_OUTSIDE_TARGET" "$OAW_OUTSIDE_TARGET_BEFORE" \
  "uninstall with swapped target parent"
assert_artifact_snapshot "$OAW_PROJECT_STATE" "$OAW_PROJECT_STATE_BEFORE" \
  "uninstall with swapped target parent"
assert_artifact_snapshot "$OAW_POLICY" "$OAW_POLICY_BEFORE" \
  "uninstall with swapped target parent"

pass "installed target parents cannot be replaced by content-matching symlinks"

run_oaw_with_mkdir_race() {
  OAW_OUTPUT_FILE=$OAW_SANDBOX/output
  set +e
  HOME="$OAW_HOME" \
    XDG_CONFIG_HOME="$OAW_CONFIG" \
    XDG_STATE_HOME="$OAW_STATE" \
    OAW_RACE_PROJECT="$OAW_PROJECT_PHYSICAL" \
    OAW_RACE_OUTSIDE="$OAW_OUTSIDE" \
    OAW_REAL_MKDIR="$OAW_REAL_MKDIR" \
    PATH="$OAW_PATH" \
    bash "$OAW_LEGACY_INSTALLER" install --project "$OAW_PROJECT" --target cursor \
    >"$OAW_OUTPUT_FILE" 2>&1
  OAW_STATUS=$?
  set -e
  OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
}

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with apply race"
OAW_OUTSIDE=$OAW_SANDBOX/outside-apply-race
OAW_FAKE_BIN=$OAW_SANDBOX/fake-bin
mkdir -p "$OAW_PROJECT" "$OAW_OUTSIDE" "$OAW_FAKE_BIN"
printf 'apply race sentinel\n' >"$OAW_OUTSIDE/sentinel"
OAW_OUTSIDE_SENTINEL_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE/sentinel")
OAW_REAL_MKDIR=$(command -v mkdir)
{
  printf '%s\n' '#!/usr/bin/env bash'
  printf '%s\n' 'case " $* " in'
  printf '%s\n' '  *"$OAW_RACE_PROJECT/.cursor/rules"*|*" ./rules "*)'
  printf '%s\n' '    if [ ! -L "$OAW_RACE_PROJECT/.cursor" ]; then'
  printf '%s\n' '      if [ -d "$OAW_RACE_PROJECT/.cursor" ]; then'
  printf '%s\n' '        mv "$OAW_RACE_PROJECT/.cursor" "$OAW_RACE_PROJECT/.cursor-original"'
  printf '%s\n' '      fi'
  printf '%s\n' '      ln -s "$OAW_RACE_OUTSIDE" "$OAW_RACE_PROJECT/.cursor"'
  printf '%s\n' '    fi'
  printf '%s\n' '    ;;'
  printf '%s\n' 'esac'
  printf '%s\n' 'exec "$OAW_REAL_MKDIR" "$@"'
} >"$OAW_FAKE_BIN/mkdir"
chmod 755 "$OAW_FAKE_BIN/mkdir"
OAW_PATH=$OAW_FAKE_BIN:$PATH
OAW_OUTSIDE_TARGET=$OAW_OUTSIDE/rules/open-agent-workflow.mdc
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md

run_oaw_with_mkdir_race
[ "$OAW_STATUS" -ne 0 ] ||
  fail "apply-time parent swap redirected a target write (output: $OAW_OUTPUT; outside: $(artifact_snapshot "$OAW_OUTSIDE_TARGET"))"
assert_contains "$OAW_PROJECT_PHYSICAL/.cursor" \
  "apply-time race diagnostic identifies the swapped component"
[ ! -e "$OAW_OUTSIDE_TARGET" ] || fail "apply-time parent swap created an outside target"
[ ! -e "$OAW_OUTSIDE/rules" ] || fail "apply-time parent swap created an outside directory"
assert_artifact_snapshot "$OAW_OUTSIDE/sentinel" "$OAW_OUTSIDE_SENTINEL_BEFORE" \
  "apply-time parent swap"
[ ! -e "$OAW_POLICY" ] || fail "apply-time parent swap created canonical policy"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "apply-time parent swap created installation state"

pass "apply revalidation blocks a parent symlink introduced after preparation"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with symlinked candidate state"
mkdir -p "$OAW_PROJECT"
run_oaw install --target codex
assert_status 0 "candidate state symlink user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "candidate state symlink project fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_PROJECT_TARGET=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_USER_TARGET=$OAW_HOME/.codex/AGENTS.md
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_OUTSIDE_STATE=$OAW_SANDBOX/outside-candidate.state
cp "$OAW_PROJECT_STATE" "$OAW_OUTSIDE_STATE"
rm -- "$OAW_PROJECT_STATE"
ln -s "$OAW_OUTSIDE_STATE" "$OAW_PROJECT_STATE"
OAW_OUTSIDE_STATE_BEFORE=$(artifact_snapshot "$OAW_OUTSIDE_STATE")
OAW_PROJECT_TARGET_BEFORE=$(artifact_snapshot "$OAW_PROJECT_TARGET")
OAW_USER_STATE_BEFORE=$(artifact_snapshot "$OAW_USER_STATE")
OAW_USER_TARGET_BEFORE=$(artifact_snapshot "$OAW_USER_TARGET")
OAW_POLICY_BEFORE=$(artifact_snapshot "$OAW_POLICY")

run_oaw update --target codex
[ "$OAW_STATUS" -ne 0 ] || fail "cross-scope update accepted a symlinked candidate state"
assert_contains "$OAW_PROJECT_STATE" "cross-scope update identifies the symlinked candidate state"
[ -L "$OAW_PROJECT_STATE" ] || fail "cross-scope update replaced the candidate state symlink"
assert_artifact_snapshot "$OAW_OUTSIDE_STATE" "$OAW_OUTSIDE_STATE_BEFORE" \
  "cross-scope update with symlinked candidate state"
assert_artifact_snapshot "$OAW_PROJECT_TARGET" "$OAW_PROJECT_TARGET_BEFORE" \
  "cross-scope update with symlinked candidate state"
assert_artifact_snapshot "$OAW_USER_STATE" "$OAW_USER_STATE_BEFORE" \
  "cross-scope update with symlinked candidate state"
assert_artifact_snapshot "$OAW_USER_TARGET" "$OAW_USER_TARGET_BEFORE" \
  "cross-scope update with symlinked candidate state"
assert_artifact_snapshot "$OAW_POLICY" "$OAW_POLICY_BEFORE" \
  "cross-scope update with symlinked candidate state"

pass "cross-scope candidate state symlinks fail before mutation"
