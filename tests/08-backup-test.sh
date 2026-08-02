#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

OAW_INSTALLER=$OAW_LEGACY_INSTALLER

trap cleanup_sandbox EXIT HUP INT TERM

assert_file_equals() {
  local expected_file=$1
  local actual_file=$2
  local description=$3

  cmp -s "$expected_file" "$actual_file" || fail "$description"
}

manifest_backup_for() {
  local manifest_file=$1
  local original_path=$2

  awk -F '\t' -v original="$original_path" \
    '$1 == "artifact" && $2 == original { print $3; exit }' "$manifest_file"
}

assert_manifest_backup() {
  local manifest_file=$1
  local original_path=$2
  local expected_file=$3
  local backup_file=
  local recorded_checksum=

  backup_file=$(manifest_backup_for "$manifest_file" "$original_path")
  [ -n "$backup_file" ] || fail "manifest omitted $original_path"
  [ -f "$backup_file" ] || fail "manifest backup is missing for $original_path"
  assert_file_equals "$expected_file" "$backup_file" \
    "backup bytes differ for $original_path"
  recorded_checksum=$(awk -F '\t' -v original="$original_path" \
    '$1 == "artifact" && $2 == original { print $4; exit }' "$manifest_file")
  [ "$recorded_checksum" = "$(cksum <"$expected_file" | awk '{ print $1 ":" $2 }')" ] ||
    fail "manifest checksum differs for $original_path"
  [ "$(file_mode "$backup_file")" = 600 ] ||
    fail "backup artifact is not private: $backup_file"
}

file_mode() {
  local mode_file=$1

  if stat -f '%Lp' "$mode_file" 2>/dev/null; then
    return 0
  fi
  stat -c '%a' "$mode_file"
}

run_oaw_with_order_guard() {
  OAW_OUTPUT_FILE=$OAW_SANDBOX/output
  set +e
  HOME="$OAW_HOME" \
    XDG_CONFIG_HOME="$OAW_CONFIG" \
    XDG_STATE_HOME="$OAW_STATE" \
    OAW_ORDER_BACKUP_ROOT="$OAW_BACKUP_ROOT" \
    OAW_ORDER_FAILURE="$OAW_ORDER_FAILURE" \
    OAW_ORDER_TARGET_ONE="$OAW_TARGET_ONE" \
    OAW_ORDER_TARGET_TWO="$OAW_TARGET_TWO" \
    OAW_ORDER_STATE="$OAW_PROJECT_STATE" \
    OAW_ORDER_POLICY="$OAW_POLICY" \
    OAW_REAL_MV="$OAW_REAL_MV" \
    OAW_REAL_RM="${OAW_REAL_RM:-$(command -v rm)}" \
    PATH="$OAW_FAKE_BIN:$OAW_PATH" \
    bash "$OAW_INSTALLER" "$@" >"$OAW_OUTPUT_FILE" 2>&1
  OAW_STATUS=$?
  set -e
  OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
}

setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with force update"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target cursor,windsurf
assert_status 0 "forced update fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_TARGET_ONE=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_TARGET_TWO=$OAW_PROJECT_PHYSICAL/.devin/rules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_TARGET_ONE_BEFORE=$OAW_SANDBOX/cursor.before
OAW_TARGET_TWO_BEFORE=$OAW_SANDBOX/windsurf.before
OAW_STATE_BEFORE=$OAW_SANDBOX/state.before

printf 'drifted cursor adapter\n' >"$OAW_TARGET_ONE"
printf 'drifted windsurf adapter\n' >"$OAW_TARGET_TWO"
cp "$OAW_TARGET_ONE" "$OAW_TARGET_ONE_BEFORE"
cp "$OAW_TARGET_TWO" "$OAW_TARGET_TWO_BEFORE"
cp "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"

run_oaw update --project "$OAW_PROJECT" --target cursor,windsurf
[ "$OAW_STATUS" -ne 0 ] || fail "drifted update succeeded without force"
[ ! -e "$OAW_BACKUP_ROOT" ] || fail "rejected update created a backup"

OAW_FAKE_BIN=$OAW_SANDBOX/fake-bin
OAW_ORDER_FAILURE=$OAW_SANDBOX/order-failure
OAW_REAL_MV=$(command -v mv)
mkdir -p "$OAW_FAKE_BIN"
{
  printf '%s\n' '#!/usr/bin/env bash'
  printf '%s\n' 'case "$*" in'
  printf '%s\n' '  *"$OAW_ORDER_TARGET_ONE"*|*"$OAW_ORDER_TARGET_TWO"*|*"$OAW_ORDER_STATE"*)'
  printf '%s\n' '    manifest=$(find "$OAW_ORDER_BACKUP_ROOT" -name manifest.tsv -type f -print -quit 2>/dev/null)'
  printf '%s\n' '    count=0'
  printf '%s\n' '    if [ -n "$manifest" ]; then'
  printf '%s\n' '      count=$(awk -F "\t" '\''$1 == "artifact" { count++ } END { print count + 0 }'\'' "$manifest")'
  printf '%s\n' '    fi'
  printf '%s\n' '    [ "$count" -eq 3 ] || : >"$OAW_ORDER_FAILURE"'
  printf '%s\n' '    ;;'
  printf '%s\n' 'esac'
  printf '%s\n' 'exec "$OAW_REAL_MV" "$@"'
} >"$OAW_FAKE_BIN/mv"
chmod 755 "$OAW_FAKE_BIN/mv"

run_oaw_with_order_guard update --project "$OAW_PROJECT" --target cursor,windsurf --force
assert_status 0 "forced update"
[ ! -e "$OAW_ORDER_FAILURE" ] || fail "forced update mutated before backup completion"

OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$OAW_BACKUP_DIR" ] || fail "forced update created no operation backup"
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
[ -f "$OAW_MANIFEST" ] || fail "forced update created no manifest"
[ "$(file_mode "$OAW_BACKUP_DIR")" = 700 ] || fail "backup directory is not private"
[ "$(file_mode "$OAW_MANIFEST")" = 600 ] || fail "backup manifest is not private"
[ "$(awk -F '\t' '$1 == "artifact" { count++ } END { print count + 0 }' "$OAW_MANIFEST")" -eq 3 ] ||
  fail "forced update manifest does not list every affected artifact"
grep -F "$(printf 'operation\tupdate')" "$OAW_MANIFEST" >/dev/null ||
  fail "forced update manifest omits the operation"
grep -F "$(printf 'scope\tproject')" "$OAW_MANIFEST" >/dev/null ||
  fail "forced update manifest omits the scope"

assert_manifest_backup "$OAW_MANIFEST" "$OAW_TARGET_ONE" "$OAW_TARGET_ONE_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_TARGET_TWO" "$OAW_TARGET_TWO_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"
grep -F "$(printf 'backup\t%s' "$OAW_BACKUP_DIR")" "$OAW_PROJECT_STATE" >/dev/null ||
  fail "forced update state omits its backup reference"
assert_contains "$OAW_BACKUP_DIR" "forced update output reports the backup path"
if grep -F 'drifted cursor adapter' "$OAW_TARGET_ONE" >/dev/null ||
  grep -F 'drifted windsurf adapter' "$OAW_TARGET_TWO" >/dev/null; then
  fail "forced update retained drifted adapter content"
fi

pass "forced update backs up every affected artifact before mutation"

run_oaw update --project "$OAW_PROJECT" --target cursor,windsurf
assert_status 0 "clean update after forced recovery"
[ "$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')" -eq 1 ] ||
  fail "clean update created another backup"
grep -F "$(printf 'backup\t%s' "$OAW_BACKUP_DIR")" "$OAW_PROJECT_STATE" >/dev/null ||
  fail "clean update discarded the prior backup reference"

pass "clean update preserves recovery state without creating a backup"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with force uninstall"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target cursor,windsurf
assert_status 0 "forced uninstall fixture install"

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_TARGET_ONE=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_TARGET_TWO=$OAW_PROJECT_PHYSICAL/.devin/rules/open-agent-workflow.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_TARGET_ONE_BEFORE=$OAW_SANDBOX/uninstall-cursor.before
OAW_TARGET_TWO_BEFORE=$OAW_SANDBOX/uninstall-windsurf.before
OAW_STATE_BEFORE=$OAW_SANDBOX/uninstall-state.before
OAW_POLICY_BEFORE=$OAW_SANDBOX/uninstall-policy.before
printf 'drifted cursor before uninstall\n' >"$OAW_TARGET_ONE"
printf 'drifted windsurf before uninstall\n' >"$OAW_TARGET_TWO"
cp "$OAW_TARGET_ONE" "$OAW_TARGET_ONE_BEFORE"
cp "$OAW_TARGET_TWO" "$OAW_TARGET_TWO_BEFORE"
cp "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"
cp "$OAW_POLICY" "$OAW_POLICY_BEFORE"

run_oaw uninstall --project "$OAW_PROJECT" --target cursor,windsurf
[ "$OAW_STATUS" -ne 0 ] || fail "drifted uninstall succeeded without force"
[ ! -e "$OAW_BACKUP_ROOT" ] || fail "rejected uninstall created a backup"

OAW_FAKE_BIN=$OAW_SANDBOX/uninstall-fake-bin
OAW_ORDER_FAILURE=$OAW_SANDBOX/uninstall-order-failure
OAW_REAL_MV=$(command -v mv)
mkdir -p "$OAW_FAKE_BIN"
{
  printf '%s\n' '#!/usr/bin/env bash'
  printf '%s\n' 'case "$*" in'
  printf '%s\n' '  *"$OAW_ORDER_TARGET_ONE"*|*"$OAW_ORDER_TARGET_TWO"*|*"$OAW_ORDER_STATE"*|*"$OAW_ORDER_POLICY"*)'
  printf '%s\n' '    manifest=$(find "$OAW_ORDER_BACKUP_ROOT" -name manifest.tsv -type f -print -quit 2>/dev/null)'
  printf '%s\n' '    count=0'
  printf '%s\n' '    if [ -n "$manifest" ]; then'
  printf '%s\n' '      count=$(awk -F "\t" '\''$1 == "artifact" { count++ } END { print count + 0 }'\'' "$manifest")'
  printf '%s\n' '    fi'
  printf '%s\n' '    [ "$count" -eq 4 ] || : >"$OAW_ORDER_FAILURE"'
  printf '%s\n' '    ;;'
  printf '%s\n' 'esac'
  printf '%s\n' 'exec "$OAW_REAL_MV" "$@"'
} >"$OAW_FAKE_BIN/mv"
chmod 755 "$OAW_FAKE_BIN/mv"
OAW_REAL_RM=$(command -v rm)
{
  printf '%s\n' '#!/usr/bin/env bash'
  printf '%s\n' 'case "$*" in'
  printf '%s\n' '  *"$OAW_ORDER_TARGET_ONE"*|*"$OAW_ORDER_TARGET_TWO"*|*"$OAW_ORDER_STATE"*|*"$OAW_ORDER_POLICY"*)'
  printf '%s\n' '    manifest=$(find "$OAW_ORDER_BACKUP_ROOT" -name manifest.tsv -type f -print -quit 2>/dev/null)'
  printf '%s\n' '    count=0'
  printf '%s\n' '    if [ -n "$manifest" ]; then'
  printf '%s\n' '      count=$(awk -F "\t" '\''$1 == "artifact" { count++ } END { print count + 0 }'\'' "$manifest")'
  printf '%s\n' '    fi'
  printf '%s\n' '    [ "$count" -eq 4 ] || : >"$OAW_ORDER_FAILURE"'
  printf '%s\n' '    ;;'
  printf '%s\n' 'esac'
  printf '%s\n' 'exec "$OAW_REAL_RM" "$@"'
} >"$OAW_FAKE_BIN/rm"
chmod 755 "$OAW_FAKE_BIN/rm"

run_oaw_with_order_guard uninstall --project "$OAW_PROJECT" --target cursor,windsurf --force
assert_status 0 "forced uninstall"
[ ! -e "$OAW_ORDER_FAILURE" ] || fail "forced uninstall mutated before backup completion"
OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$OAW_BACKUP_DIR" ] || fail "forced uninstall created no operation backup"
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
[ "$(awk -F '\t' '$1 == "artifact" { count++ } END { print count + 0 }' "$OAW_MANIFEST")" -eq 4 ] ||
  fail "forced uninstall manifest does not list every affected artifact"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_TARGET_ONE" "$OAW_TARGET_ONE_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_TARGET_TWO" "$OAW_TARGET_TWO_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_POLICY" "$OAW_POLICY_BEFORE"
assert_contains "$OAW_BACKUP_DIR" "forced uninstall output reports the backup path"
[ ! -e "$OAW_TARGET_ONE" ] || fail "forced uninstall retained cursor"
[ ! -e "$OAW_TARGET_TWO" ] || fail "forced uninstall retained windsurf"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "forced uninstall retained state"
[ ! -e "$OAW_POLICY" ] || fail "forced uninstall retained policy"

pass "forced uninstall backs up every affected artifact before removal"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with recoverable markers"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "recoverable marker fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_MARKER_TARGET=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_CORRUPT_BEFORE=$OAW_SANDBOX/corrupt.before
awk '$0 != "<!-- BEGIN OPEN AGENT WORKFLOW -->" { print }' \
  "$OAW_MARKER_TARGET" >"$OAW_CORRUPT_BEFORE"
cp "$OAW_CORRUPT_BEFORE" "$OAW_MARKER_TARGET"

run_oaw update --project "$OAW_PROJECT" --target codex --force
assert_status 0 "forced recoverable marker update"
OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$OAW_BACKUP_DIR" ] || fail "recoverable marker update created no backup"
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
assert_manifest_backup "$OAW_MANIFEST" "$OAW_MARKER_TARGET" "$OAW_CORRUPT_BEFORE"
[ "$(grep -c '^<!-- BEGIN OPEN AGENT WORKFLOW -->$' "$OAW_MARKER_TARGET")" -eq 1 ] ||
  fail "forced marker update did not restore one begin marker"
[ "$(grep -c '^<!-- END OPEN AGENT WORKFLOW -->$' "$OAW_MARKER_TARGET")" -eq 1 ] ||
  fail "forced marker update did not retain one end marker"

pass "force repairs uniquely identifiable marker corruption after backup"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with missing end marker"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "missing end-marker fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_MARKER_TARGET=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_CORRUPT_BEFORE=$OAW_SANDBOX/missing-end.before
awk '$0 != "<!-- END OPEN AGENT WORKFLOW -->" { print }' \
  "$OAW_MARKER_TARGET" >"$OAW_CORRUPT_BEFORE"
cp "$OAW_CORRUPT_BEFORE" "$OAW_MARKER_TARGET"
run_oaw update --project "$OAW_PROJECT" --target codex --force
assert_status 0 "forced missing end-marker update"
OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
assert_manifest_backup "$OAW_MANIFEST" "$OAW_MARKER_TARGET" "$OAW_CORRUPT_BEFORE"
[ "$(grep -c '^<!-- BEGIN OPEN AGENT WORKFLOW -->$' "$OAW_MARKER_TARGET")" -eq 1 ] ||
  fail "forced missing end-marker update changed the begin marker"
[ "$(grep -c '^<!-- END OPEN AGENT WORKFLOW -->$' "$OAW_MARKER_TARGET")" -eq 1 ] ||
  fail "forced missing end-marker update did not restore one end marker"

pass "force repairs a uniquely identifiable missing end marker after backup"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with ambiguous markers"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "ambiguous marker fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_MARKER_TARGET=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_CORRUPT_BEFORE=$OAW_SANDBOX/ambiguous.before
OAW_STATE_BEFORE=$OAW_SANDBOX/ambiguous-state.before
awk '$0 != "<!-- BEGIN OPEN AGENT WORKFLOW -->" { print }' \
  "$OAW_MARKER_TARGET" >"$OAW_CORRUPT_BEFORE"
cat "$OAW_CORRUPT_BEFORE" "$OAW_CORRUPT_BEFORE" >"$OAW_MARKER_TARGET"
cp "$OAW_MARKER_TARGET" "$OAW_CORRUPT_BEFORE"
cp "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"

run_oaw update --project "$OAW_PROJECT" --target codex --force
[ "$OAW_STATUS" -ne 0 ] || fail "ambiguous marker update unexpectedly succeeded"
assert_contains "manual recovery" "ambiguous marker update explains manual recovery"
OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$OAW_BACKUP_DIR" ] || fail "ambiguous marker update retained no backup"
assert_contains "$OAW_BACKUP_DIR" "ambiguous marker update reports its recovery path"
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
assert_manifest_backup "$OAW_MANIFEST" "$OAW_MARKER_TARGET" "$OAW_CORRUPT_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"
assert_file_equals "$OAW_CORRUPT_BEFORE" "$OAW_MARKER_TARGET" \
  "ambiguous marker update changed the target"
assert_file_equals "$OAW_STATE_BEFORE" "$OAW_PROJECT_STATE" \
  "ambiguous marker update changed state"

pass "ambiguous marker ownership retains backup and requires manual recovery"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with recoverable uninstall"
mkdir -p "$OAW_PROJECT"
printf 'personal project instruction\n' >"$OAW_PROJECT/AGENTS.md"
run_oaw install --project "$OAW_PROJECT" --target codex
assert_status 0 "recoverable marker uninstall fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_MARKER_TARGET=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_CORRUPT_BEFORE=$OAW_SANDBOX/uninstall-corrupt.before
awk '$0 != "<!-- BEGIN OPEN AGENT WORKFLOW -->" { print }' \
  "$OAW_MARKER_TARGET" >"$OAW_CORRUPT_BEFORE"
cp "$OAW_CORRUPT_BEFORE" "$OAW_MARKER_TARGET"

run_oaw uninstall --project "$OAW_PROJECT" --target codex --force
assert_status 0 "forced recoverable marker uninstall"
OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$OAW_BACKUP_DIR" ] || fail "recoverable marker uninstall created no backup"
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
assert_manifest_backup "$OAW_MANIFEST" "$OAW_MARKER_TARGET" "$OAW_CORRUPT_BEFORE"
[ "$(cat "$OAW_MARKER_TARGET")" = 'personal project instruction' ] ||
  fail "forced marker uninstall changed user content"
[ ! -e "$OAW_PROJECT_STATE" ] || fail "forced marker uninstall retained state"
[ ! -e "$OAW_POLICY" ] || fail "forced marker uninstall retained policy"

pass "force removes uniquely identifiable corrupt block while preserving user content"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with shared force"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target codex,opencode
assert_status 0 "shared force fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_SHARED_TARGET=$OAW_PROJECT_PHYSICAL/AGENTS.md
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_SHARED_DRIFT=$OAW_SANDBOX/shared-drift
awk '
  $0 == "<!-- BEGIN OPEN AGENT WORKFLOW -->" { in_block = 1 }
  in_block && $0 != "<!-- BEGIN OPEN AGENT WORKFLOW -->" &&
    $0 != "<!-- END OPEN AGENT WORKFLOW -->" {
    print "shared destination drift"
    in_block = 0
    next
  }
  { print }
' "$OAW_SHARED_TARGET" >"$OAW_SHARED_DRIFT"
mv "$OAW_SHARED_DRIFT" "$OAW_SHARED_TARGET"
run_oaw update --project "$OAW_PROJECT" --target codex --force
assert_status 0 "forced shared-destination update"
[ "$(awk -F '\t' '$1 == "target" && $3 ~ /AGENTS.md$/ { print $5 }' \
  "$OAW_PROJECT_STATE" | sort -u | wc -l | tr -d ' ')" -eq 1 ] ||
  fail "forced shared update left conflicting state checksums"

pass "force applies consistently to every record sharing a selected destination"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with force dry run"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "force dry-run fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_TARGET_ONE=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_TARGET_ONE_BEFORE=$OAW_SANDBOX/dry-target.before
OAW_STATE_BEFORE=$OAW_SANDBOX/dry-state.before
printf 'dry-run drift\n' >"$OAW_TARGET_ONE"
cp "$OAW_TARGET_ONE" "$OAW_TARGET_ONE_BEFORE"
cp "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"
run_oaw update --project "$OAW_PROJECT" --target cursor --force --dry-run
assert_status 0 "forced update dry run"
assert_contains "would-backup" "forced dry run previews its backup"
[ ! -e "$OAW_BACKUP_ROOT" ] || fail "forced dry run created a backup"
assert_file_equals "$OAW_TARGET_ONE_BEFORE" "$OAW_TARGET_ONE" \
  "forced dry run changed target"
assert_file_equals "$OAW_STATE_BEFORE" "$OAW_PROJECT_STATE" \
  "forced dry run changed state"

pass "forced dry run previews recovery without mutation"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with invalid force state"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "invalid force-state fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_TARGET_ONE=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_INVALID_STATE=$OAW_SANDBOX/invalid-force.state
{
  awk '$1 == "policy" { print; print "backup\t/first"; print "backup\t/second"; next } { print }' \
    "$OAW_PROJECT_STATE"
} >"$OAW_INVALID_STATE"
mv "$OAW_INVALID_STATE" "$OAW_PROJECT_STATE"
printf 'invalid state target drift\n' >"$OAW_TARGET_ONE"
run_oaw update --project "$OAW_PROJECT" --target cursor --force
[ "$OAW_STATUS" -ne 0 ] || fail "force accepted duplicate backup state"
[ ! -e "$OAW_BACKUP_ROOT" ] || fail "invalid state force created a backup"

pass "force never overrides invalid state schema"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with shared policy force"
mkdir -p "$OAW_PROJECT"
run_oaw install --target codex
assert_status 0 "shared policy force user fixture install"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "shared policy force project fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_USER_STATE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/ENGINEERING.md
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_POLICY_BEFORE=$OAW_SANDBOX/shared-policy.before
OAW_USER_STATE_BEFORE=$OAW_SANDBOX/shared-user-state.before
OAW_PROJECT_STATE_BEFORE=$OAW_SANDBOX/shared-project-state.before
printf 'drifted shared canonical policy\n' >"$OAW_POLICY"
cp "$OAW_POLICY" "$OAW_POLICY_BEFORE"
cp "$OAW_USER_STATE" "$OAW_USER_STATE_BEFORE"
cp "$OAW_PROJECT_STATE" "$OAW_PROJECT_STATE_BEFORE"
run_oaw update --project "$OAW_PROJECT" --target cursor --force
assert_status 0 "forced shared policy update"
OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
assert_manifest_backup "$OAW_MANIFEST" "$OAW_POLICY" "$OAW_POLICY_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_USER_STATE" "$OAW_USER_STATE_BEFORE"
assert_manifest_backup "$OAW_MANIFEST" "$OAW_PROJECT_STATE" "$OAW_PROJECT_STATE_BEFORE"
run_oaw check --target codex
assert_status 0 "forced shared policy user check"
assert_contains "installed codex: clean" "forced shared policy leaves user state clean"
run_oaw check --project "$OAW_PROJECT" --target cursor
assert_status 0 "forced shared policy project check"
assert_contains "installed cursor: clean" "forced shared policy leaves project state clean"

pass "force backs up and synchronizes every state sharing a drifted policy"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with force symlink"
OAW_OUTSIDE=$OAW_SANDBOX/outside-force-target
mkdir -p "$OAW_PROJECT" "$OAW_OUTSIDE"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "force symlink fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_TARGET_ONE=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_OUTSIDE_TARGET=$OAW_OUTSIDE/sentinel
printf 'outside force sentinel\n' >"$OAW_OUTSIDE_TARGET"
OAW_OUTSIDE_BEFORE=$(cksum <"$OAW_OUTSIDE_TARGET")
rm -- "$OAW_TARGET_ONE"
ln -s "$OAW_OUTSIDE_TARGET" "$OAW_TARGET_ONE"
run_oaw update --project "$OAW_PROJECT" --target cursor --force
[ "$OAW_STATUS" -ne 0 ] || fail "force accepted a target symlink"
[ -L "$OAW_TARGET_ONE" ] || fail "force replaced a target symlink"
[ "$(cksum <"$OAW_OUTSIDE_TARGET")" = "$OAW_OUTSIDE_BEFORE" ] ||
  fail "force changed an outside symlink target"
[ ! -e "$OAW_BACKUP_ROOT" ] || fail "force symlink rejection created a backup"

pass "force never overrides symlink containment"

cleanup_sandbox
setup_sandbox
OAW_PROJECT="$OAW_SANDBOX/project with backup race"
mkdir -p "$OAW_PROJECT"
run_oaw install --project "$OAW_PROJECT" --target cursor
assert_status 0 "backup race fixture install"
OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
OAW_PROJECT_ID=$(printf '%s' "$OAW_PROJECT_PHYSICAL" | cksum | awk '{ print $1 "-" $2 }')
OAW_TARGET_ONE=$OAW_PROJECT_PHYSICAL/.cursor/rules/open-agent-workflow.mdc
OAW_PROJECT_STATE=$OAW_STATE/open-agent-workflow/installations/projects/$OAW_PROJECT_ID.state
OAW_BACKUP_ROOT=$OAW_STATE/open-agent-workflow/backups
OAW_TARGET_ONE_BEFORE=$OAW_SANDBOX/race-target.before
OAW_STATE_BEFORE=$OAW_SANDBOX/race-state.before
OAW_FAKE_BIN=$OAW_SANDBOX/race-fake-bin
OAW_REAL_MV=$(command -v mv)
printf 'initial drift before backup race\n' >"$OAW_TARGET_ONE"
cp "$OAW_TARGET_ONE" "$OAW_TARGET_ONE_BEFORE"
cp "$OAW_PROJECT_STATE" "$OAW_STATE_BEFORE"
mkdir -p "$OAW_FAKE_BIN"
{
  printf '%s\n' '#!/usr/bin/env bash'
  printf '%s\n' 'case "$*" in'
  printf '%s\n' '  *"/manifest.tsv"*) printf "%s\n" "raced after backup" >"$OAW_RACE_TARGET" ;;'
  printf '%s\n' 'esac'
  printf '%s\n' 'exec "$OAW_REAL_MV" "$@"'
} >"$OAW_FAKE_BIN/mv"
chmod 755 "$OAW_FAKE_BIN/mv"
OAW_OUTPUT_FILE=$OAW_SANDBOX/output
set +e
HOME="$OAW_HOME" \
  XDG_CONFIG_HOME="$OAW_CONFIG" \
  XDG_STATE_HOME="$OAW_STATE" \
  OAW_RACE_TARGET="$OAW_TARGET_ONE" \
  OAW_REAL_MV="$OAW_REAL_MV" \
  PATH="$OAW_FAKE_BIN:$OAW_PATH" \
  bash "$OAW_INSTALLER" update --project "$OAW_PROJECT" --target cursor --force \
  >"$OAW_OUTPUT_FILE" 2>&1
OAW_STATUS=$?
set -e
OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
[ "$OAW_STATUS" -ne 0 ] || fail "backup source race unexpectedly succeeded"
assert_contains "backup source changed before mutation" \
  "backup source race reports its refusal"
OAW_BACKUP_DIR=$(find "$OAW_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -print -quit)
OAW_MANIFEST=$OAW_BACKUP_DIR/manifest.tsv
assert_manifest_backup "$OAW_MANIFEST" "$OAW_TARGET_ONE" "$OAW_TARGET_ONE_BEFORE"
assert_file_equals "$OAW_STATE_BEFORE" "$OAW_PROJECT_STATE" \
  "backup source race applied state changes"
grep -F 'raced after backup' "$OAW_TARGET_ONE" >/dev/null ||
  fail "backup source race hook did not execute"

pass "source changes after backup completion block the apply phase"
