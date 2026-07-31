#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

scope_snapshot() {
  local snapshot_root=$1
  local snapshot_path=
  local relative_path=

  find "$snapshot_root" -mindepth 1 -print | LC_ALL=C sort |
    while IFS= read -r snapshot_path; do
      relative_path=${snapshot_path#"$snapshot_root"/}
      if [ -L "$snapshot_path" ]; then
        printf 'link\t%s\t%s\n' "$relative_path" "$(readlink "$snapshot_path")"
      elif [ -d "$snapshot_path" ]; then
        printf 'directory\t%s\n' "$relative_path"
      elif [ -f "$snapshot_path" ]; then
        printf 'file\t%s\t%s\n' "$relative_path" "$(cksum <"$snapshot_path")"
      else
        printf 'other\t%s\n' "$relative_path"
      fi
    done
}

all_scope_snapshot() {
  printf 'HOME\n'
  scope_snapshot "$OAW_HOME"
  printf 'CONFIG\n'
  scope_snapshot "$OAW_CONFIG"
  printf 'STATE\n'
  scope_snapshot "$OAW_STATE"
  printf 'PROJECT\n'
  scope_snapshot "$OAW_PROJECT"
}

setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'-project [glob]* ; touch PARTIAL-PAYLOAD'
mkdir -p "$OAW_PROJECT/.claude"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
printf 'pre-existing Claude instructions\n' >"$OAW_PROJECT/.claude/CLAUDE.md"
printf 'later target parent sentinel\n' >"$OAW_PROJECT/.cursor"
OAW_SCOPE_BEFORE=$(all_scope_snapshot)

run_oaw install --project "$OAW_PROJECT" --target claude,cursor
[ "$OAW_STATUS" -ne 0 ] || fail "later invalid target parent unexpectedly installed"
assert_contains "$OAW_PROJECT_PHYSICAL/.cursor" \
  "later invalid target diagnostic identifies the parent"
[ "$(all_scope_snapshot)" = "$OAW_SCOPE_BEFORE" ] ||
  fail "later invalid target partially changed an earlier target or scope metadata"
[ ! -e "$OAW_SANDBOX/PARTIAL-PAYLOAD" ] ||
  fail "hostile project name was interpreted as a shell command"

pass "later invalid targets leave the complete scope byte-identical"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'fresh dry run project'
mkdir -p "$OAW_PROJECT"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_SCOPE_BEFORE=$(all_scope_snapshot)
run_oaw install --project "$OAW_PROJECT" --target cursor --dry-run
assert_status 0 "fresh project install dry run"
assert_contains "would-create: $OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc" \
  "fresh dry run previews its target"
[ "$(all_scope_snapshot)" = "$OAW_SCOPE_BEFORE" ] ||
  fail "fresh install dry run created planned directories or files"

pass "fresh dry run previews planned directories without requiring creation"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'-project with spaces [*] ; exact-uninstall'
mkdir -p "$OAW_PROJECT/.roo/rules"

run_oaw install --project "$OAW_PROJECT" --target cursor,roo
assert_status 0 "hostile-name project install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
[ -f "$OAW_PROJECT/.cursor/rules/open-agent-workflow.mdc" ] ||
  fail "hostile-name install missed the Cursor target"
[ -f "$OAW_PROJECT/.roo/rules/open-agent-workflow.md" ] ||
  fail "hostile-name install missed the Roo target"
grep -F "$(printf 'directory\t%s' "$OAW_PROJECT_PHYSICAL/.cursor")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "state omitted the OAW-created Cursor directory"
grep -F "$(printf 'directory\t%s' "$OAW_PROJECT_PHYSICAL/.cursor/rules")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "state omitted the OAW-created Cursor rules directory"
if grep -F "$(printf 'directory\t%s' "$OAW_PROJECT_PHYSICAL/.roo")" \
  "$OAW_PROJECT_STATE" >/dev/null; then
  fail "state claimed ownership of a pre-existing Roo directory"
fi

OAW_SCOPE_BEFORE=$(all_scope_snapshot)
run_oaw uninstall --project "$OAW_PROJECT" --target cursor,roo --dry-run
assert_status 0 "hostile-name project uninstall dry run"
assert_contains "would-remove-directory: $OAW_PROJECT_PHYSICAL/.cursor/rules" \
  "dry run previews the nested OAW-created directory removal"
assert_contains "would-remove-directory: $OAW_PROJECT_PHYSICAL/.cursor" \
  "dry run previews the OAW-created directory removal"
[ "$(all_scope_snapshot)" = "$OAW_SCOPE_BEFORE" ] ||
  fail "uninstall dry run changed the hostile-name scope"

run_oaw uninstall --project "$OAW_PROJECT" --target cursor,roo
assert_status 0 "hostile-name project uninstall"
[ ! -e "$OAW_PROJECT/.cursor" ] ||
  fail "uninstall retained directories created for the Cursor target"
[ -d "$OAW_PROJECT/.roo/rules" ] ||
  fail "uninstall removed a pre-existing Roo directory"
[ ! -e "$OAW_PROJECT/.roo/rules/open-agent-workflow.md" ] ||
  fail "uninstall retained the Roo target"
[ ! -e "$OAW_CONFIG/open-agent-workflow" ] ||
  fail "final clean uninstall retained the empty OAW config namespace"
[ ! -e "$OAW_STATE/open-agent-workflow" ] ||
  fail "final clean uninstall retained the empty OAW state namespace"
[ ! -e "$OAW_SANDBOX/exact-uninstall" ] ||
  fail "hostile project name created a payload file"

pass "exact uninstall prunes only OAW-created empty target directories"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'project with inert $(touch OWNERSHIP-PAYLOAD) text'
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "owned-directory state fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_UNRELATED_DIRECTORY=$OAW_PROJECT_PHYSICAL/'$(touch OWNERSHIP-PAYLOAD)'
OAW_TAMPERED_STATE=$OAW_SANDBOX/tampered-directory.state
awk -v record="$(printf 'directory\t%s' "$OAW_UNRELATED_DIRECTORY")" '
  $1 == "target" && !inserted { print record; inserted = 1 }
  { print }
' "$OAW_PROJECT_STATE" >"$OAW_TAMPERED_STATE"
mv "$OAW_TAMPERED_STATE" "$OAW_PROJECT_STATE"
OAW_SCOPE_BEFORE=$(all_scope_snapshot)

run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "check forged directory ownership"
assert_contains "installed cursor: invalid-state" \
  "check rejects a directory record unrelated to its target"
run_oaw update --project "$OAW_PROJECT" --target cursor
[ "$OAW_STATUS" -ne 0 ] || fail "update accepted forged directory ownership"
assert_contains "owned directory does not match" \
  "update identifies forged directory ownership"
[ "$(all_scope_snapshot)" = "$OAW_SCOPE_BEFORE" ] ||
  fail "forged directory ownership changed the managed scope"
[ ! -e "$OAW_SANDBOX/OWNERSHIP-PAYLOAD" ] ||
  fail "directory state executed a shell-looking payload"

pass "owned-directory state is inert and registry-bound"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'directory ownership race project'
OAW_FAKE_BIN=$OAW_SANDBOX/directory-race-bin
OAW_RACE_MARKER=$OAW_SANDBOX/directory-race-triggered
mkdir -p "$OAW_PROJECT" "$OAW_FAKE_BIN"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_CURSOR=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_REAL_SORT=$(command -v sort)
OAW_REAL_MKDIR=$(command -v mkdir)
{
  printf '%s\n' '#!/usr/bin/env bash'
  printf '%s\n' 'case "$*" in'
  printf '%s\n' '  *final-directories*)'
  printf '%s\n' '    if [ ! -e "$OAW_RACE_MARKER" ]; then'
  printf '%s\n' '      "$OAW_REAL_MKDIR" -p "$OAW_RACE_PROJECT/.cursor/rules"'
  printf '%s\n' '      : >"$OAW_RACE_MARKER"'
  printf '%s\n' '    fi'
  printf '%s\n' '    ;;'
  printf '%s\n' 'esac'
  printf '%s\n' 'exec "$OAW_REAL_SORT" "$@"'
} >"$OAW_FAKE_BIN/sort"
chmod 755 "$OAW_FAKE_BIN/sort"
OAW_OUTPUT_FILE=$OAW_SANDBOX/output
set +e
HOME="$OAW_HOME" \
  XDG_CONFIG_HOME="$OAW_CONFIG" \
  XDG_STATE_HOME="$OAW_STATE" \
  OAW_RACE_PROJECT="$OAW_PROJECT_PHYSICAL" \
  OAW_RACE_MARKER="$OAW_RACE_MARKER" \
  OAW_REAL_SORT="$OAW_REAL_SORT" \
  OAW_REAL_MKDIR="$OAW_REAL_MKDIR" \
  PATH="$OAW_FAKE_BIN:$OAW_PATH" \
  bash "$OAW_INSTALLER" install --project "$OAW_PROJECT" --target cursor \
  >"$OAW_OUTPUT_FILE" 2>&1
OAW_STATUS=$?
set -e
OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
[ -e "$OAW_RACE_MARKER" ] || fail "directory ownership race hook did not execute"
[ "$OAW_STATUS" -ne 0 ] || fail "install claimed a directory created after preparation"
assert_contains "owned directory appeared before creation" \
  "directory ownership race reports its refusal"
[ -d "$OAW_PROJECT_PHYSICAL/.cursor/rules" ] ||
  fail "directory ownership race removed the user-created directory"
[ ! -e "$OAW_CURSOR" ] || fail "directory ownership race created a target"
[ ! -e "$OAW_POLICY" ] || fail "directory ownership race created policy"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "directory ownership race created state"

pass "directories appearing after preparation are never claimed as OAW-owned"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'directory removal race project'
OAW_OUTSIDE=$OAW_SANDBOX/directory-removal-outside
OAW_FAKE_BIN=$OAW_SANDBOX/directory-removal-bin
mkdir -p "$OAW_PROJECT" "$OAW_OUTSIDE/rules" "$OAW_FAKE_BIN"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "directory removal race fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_PROJECT_STATE_BEFORE=$OAW_SANDBOX/directory-removal-state.before
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_POLICY_BEFORE=$OAW_SANDBOX/directory-removal-policy.before
cp "$OAW_PROJECT_STATE" "$OAW_PROJECT_STATE_BEFORE"
cp "$OAW_POLICY" "$OAW_POLICY_BEFORE"
OAW_REAL_RMDIR=$(command -v rmdir)
OAW_REAL_MV=$(command -v mv)
{
  printf '%s\n' '#!/usr/bin/env bash'
  printf '%s\n' 'case "$*" in'
  printf '%s\n' '  *"$OAW_RACE_PROJECT/.cursor/rules"*|*" ./rules"*)'
  printf '%s\n' '    if [ ! -L "$OAW_RACE_PROJECT/.cursor" ]; then'
  printf '%s\n' '      "$OAW_REAL_MV" "$OAW_RACE_PROJECT/.cursor" "$OAW_RACE_PROJECT/.cursor-original"'
  printf '%s\n' '      ln -s "$OAW_RACE_OUTSIDE" "$OAW_RACE_PROJECT/.cursor"'
  printf '%s\n' '    fi'
  printf '%s\n' '    ;;'
  printf '%s\n' 'esac'
  printf '%s\n' 'exec "$OAW_REAL_RMDIR" "$@"'
} >"$OAW_FAKE_BIN/rmdir"
chmod 755 "$OAW_FAKE_BIN/rmdir"
OAW_OUTPUT_FILE=$OAW_SANDBOX/output
set +e
HOME="$OAW_HOME" \
  XDG_CONFIG_HOME="$OAW_CONFIG" \
  XDG_STATE_HOME="$OAW_STATE" \
  OAW_RACE_PROJECT="$OAW_PROJECT_PHYSICAL" \
  OAW_RACE_OUTSIDE="$OAW_OUTSIDE" \
  OAW_REAL_RMDIR="$OAW_REAL_RMDIR" \
  OAW_REAL_MV="$OAW_REAL_MV" \
  PATH="$OAW_FAKE_BIN:$OAW_PATH" \
  bash "$OAW_INSTALLER" uninstall --project "$OAW_PROJECT" --target cursor \
  >"$OAW_OUTPUT_FILE" 2>&1
OAW_STATUS=$?
set -e
OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
[ "$OAW_STATUS" -ne 0 ] || fail "directory removal parent swap unexpectedly succeeded"
assert_contains "$OAW_PROJECT_PHYSICAL/.cursor" \
  "directory removal race identifies the swapped parent"
[ -d "$OAW_OUTSIDE/rules" ] ||
  fail "directory removal parent swap deleted an outside directory"
[ -L "$OAW_PROJECT_PHYSICAL/.cursor" ] ||
  fail "directory removal race hook did not swap the project parent"
cmp -s "$OAW_PROJECT_STATE_BEFORE" "$OAW_PROJECT_STATE" ||
  fail "directory removal race changed project state"
cmp -s "$OAW_POLICY_BEFORE" "$OAW_POLICY" ||
  fail "directory removal race changed canonical policy"

pass "directory removal cannot follow a swapped parent outside scope"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'multi target drift project'
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target claude,cursor
assert_status 0 "multi-target drift fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_CURSOR=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
printf 'later Cursor drift\n' >"$OAW_CURSOR"
OAW_SCOPE_BEFORE=$(all_scope_snapshot)

run_oaw update --project "$OAW_PROJECT" --target claude,cursor
[ "$OAW_STATUS" -ne 0 ] || fail "multi-target update accepted later drift"
assert_contains "$OAW_CURSOR" "multi-target update identifies the later drift"
[ "$(all_scope_snapshot)" = "$OAW_SCOPE_BEFORE" ] ||
  fail "multi-target update changed an earlier clean target before later drift"
run_oaw uninstall --project "$OAW_PROJECT" --target claude,cursor
[ "$OAW_STATUS" -ne 0 ] || fail "multi-target uninstall accepted later drift"
assert_contains "$OAW_CURSOR" "multi-target uninstall identifies the later drift"
[ "$(all_scope_snapshot)" = "$OAW_SCOPE_BEFORE" ] ||
  fail "multi-target uninstall changed an earlier clean target before later drift"

pass "later drift blocks multi-target update and uninstall before every write"

cleanup_sandbox
setup_sandbox
OAW_INSTALLER=$OAW_REPOSITORY/install.sh
OAW_PROJECT=$OAW_SANDBOX/'partial directory ownership project'
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target cursor,roo
assert_status 0 "partial directory ownership fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state

run_oaw uninstall --project "$OAW_PROJECT" --target cursor
assert_status 0 "selected Cursor directory uninstall"
[ ! -e "$OAW_PROJECT_PHYSICAL/.cursor" ] ||
  fail "selected uninstall retained the Cursor directory"
[ -f "$OAW_PROJECT_PHYSICAL/.roo/rules/open-agent-workflow.md" ] ||
  fail "selected uninstall removed the remaining Roo target"
if grep -F "$(printf 'directory\t%s' "$OAW_PROJECT_PHYSICAL/.cursor")" \
  "$OAW_PROJECT_STATE" >/dev/null; then
  fail "selected uninstall retained removed Cursor directory ownership"
fi
grep -F "$(printf 'directory\t%s' "$OAW_PROJECT_PHYSICAL/.roo/rules")" \
  "$OAW_PROJECT_STATE" >/dev/null || fail "selected uninstall lost remaining Roo ownership"

printf 'personal Roo rule\n' >"$OAW_PROJECT_PHYSICAL/.roo/rules/personal.md"
OAW_PERSONAL_ROO_BEFORE=$(cksum <"$OAW_PROJECT_PHYSICAL/.roo/rules/personal.md")
run_oaw uninstall --project "$OAW_PROJECT" --target roo
assert_status 0 "final nonempty Roo directory uninstall"
[ ! -e "$OAW_PROJECT_PHYSICAL/.roo/rules/open-agent-workflow.md" ] ||
  fail "final uninstall retained the Roo target"
[ -d "$OAW_PROJECT_PHYSICAL/.roo/rules" ] ||
  fail "final uninstall removed a nonempty OAW-created directory"
[ "$(cksum <"$OAW_PROJECT_PHYSICAL/.roo/rules/personal.md")" = "$OAW_PERSONAL_ROO_BEFORE" ] ||
  fail "final uninstall changed a user file in an OAW-created directory"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "final uninstall retained project state"

pass "partial uninstall transfers no directory ownership and preserves nonempty directories"

printf 'PASS: transactional preparation and exact uninstall cases passed\n'
