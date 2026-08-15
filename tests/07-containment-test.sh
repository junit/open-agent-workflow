#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

file_checksum() {
  cksum <"$1"
}

file_fingerprint() {
  fingerprint_file=$1
  fingerprint_stat=

  if fingerprint_stat=$(stat -f '%d:%i:%m:%z' "$fingerprint_file" 2>/dev/null); then
    :
  else
    fingerprint_stat=$(stat -c '%d:%i:%Y:%s' "$fingerprint_file")
  fi
  printf '%s:%s\n' "$(cksum <"$fingerprint_file")" "$fingerprint_stat"
}

artifact_snapshot() {
  artifact_path=$1

  if [ -e "$artifact_path" ]; then
    printf 'present:%s\n' "$(file_fingerprint "$artifact_path")"
  else
    printf 'absent\n'
  fi
}

assert_artifact_snapshot() {
  artifact_path=$1
  expected_snapshot=$2
  description=$3

  [ "$(artifact_snapshot "$artifact_path")" = "$expected_snapshot" ] ||
    fail "$description changed $artifact_path"
}

setup_sandbox

run_oaw check --target claude
assert_status 0 "empty management check"
assert_contains "installed claude: not-installed" "empty management check reports status"
assert_read_only_roots

run_oaw install --target claude,codex
assert_status 0 "multi-target user install"

OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
OAW_STATE_FILE=$OAW_STATE/open-agent-workflow/installations/user.state
OAW_CLAUDE=$OAW_HOME/.claude/CLAUDE.md
OAW_CODEX=$OAW_HOME/.codex/AGENTS.md

for artifact in "$OAW_POLICY" "$OAW_STATE_FILE" "$OAW_CLAUDE" "$OAW_CODEX"; do
  [ -f "$artifact" ] || fail "multi-target install omitted $artifact"
done

run_oaw check --target claude,codex
assert_status 0 "clean multi-target check"
assert_contains "installed claude: clean" "Claude installation is clean"
assert_contains "installed codex: clean" "Codex installation is clean"

printf '%s\n' \
  '<!-- BEGIN OPEN AGENT WORKFLOW -->' \
  'local Codex drift' \
  '<!-- END OPEN AGENT WORKFLOW -->' \
  >"$OAW_CODEX"

OAW_POLICY_BEFORE=$(file_checksum "$OAW_POLICY")
OAW_STATE_BEFORE=$(file_checksum "$OAW_STATE_FILE")
OAW_CLAUDE_BEFORE=$(file_checksum "$OAW_CLAUDE")
OAW_CODEX_BEFORE=$(file_checksum "$OAW_CODEX")

run_oaw update --target claude,codex
[ "$OAW_STATUS" -ne 0 ] || fail "multi-target update accepted Codex drift"
assert_contains "$OAW_CODEX" "drift rejection identifies the Codex target"
[ "$(file_checksum "$OAW_POLICY")" = "$OAW_POLICY_BEFORE" ] ||
  fail "drift rejection changed canonical policy"
[ "$(file_checksum "$OAW_STATE_FILE")" = "$OAW_STATE_BEFORE" ] ||
  fail "drift rejection changed installation state"
[ "$(file_checksum "$OAW_CLAUDE")" = "$OAW_CLAUDE_BEFORE" ] ||
  fail "drift rejection changed an earlier clean target"
[ "$(file_checksum "$OAW_CODEX")" = "$OAW_CODEX_BEFORE" ] ||
  fail "drift rejection overwrote the drifted target"

run_oaw update --target codex --force
assert_status 0 "forced Codex recovery"
assert_contains "oaw: backup:" "forced recovery reports its backup"
grep -F 'Open Agent Workflow is opt-in.' "$OAW_CODEX" >/dev/null ||
  fail "forced recovery did not restore the Codex managed block"
OAW_BACKUP_MANIFEST=$(find "$OAW_STATE/open-agent-workflow/backups" \
  -type f -name manifest.tsv -print -quit)
[ -n "$OAW_BACKUP_MANIFEST" ] || fail "forced recovery omitted its backup manifest"
grep -F "$OAW_CODEX" "$OAW_BACKUP_MANIFEST" >/dev/null ||
  fail "backup manifest omitted the recovered Codex target"

run_oaw uninstall --target claude
assert_status 0 "partial user uninstall"
[ ! -e "$OAW_CLAUDE" ] || fail "partial uninstall retained the Claude target"
[ -f "$OAW_CODEX" ] || fail "partial uninstall removed the remaining Codex target"
[ -f "$OAW_POLICY" ] || fail "partial uninstall removed the shared policy"
[ -f "$OAW_STATE_FILE" ] || fail "partial uninstall removed live installation state"

run_oaw uninstall --target codex
assert_status 0 "final user uninstall"
[ ! -e "$OAW_CODEX" ] || fail "final uninstall retained the Codex target"
[ ! -e "$OAW_POLICY" ] || fail "final uninstall retained the canonical policy"
[ ! -e "$OAW_STATE_FILE" ] || fail "final uninstall retained installation state"

pass "Go management CLI preserves atomic drift, backup, and partial-uninstall behavior"

cleanup_sandbox
setup_sandbox

OAW_OUTSIDE=$OAW_SANDBOX/outside-state
mkdir -p "$OAW_OUTSIDE"
printf 'outside sentinel\n' >"$OAW_OUTSIDE/sentinel"
ln -s "$OAW_OUTSIDE" "$OAW_STATE/open-agent-workflow"

run_oaw install --target claude
[ "$OAW_STATUS" -ne 0 ] || fail "symlinked state root redirected an install"
[ "$(find "$OAW_OUTSIDE" -mindepth 1 -maxdepth 1 -print | wc -l | tr -d ' ')" -eq 1 ] ||
  fail "symlinked state root created an outside artifact"
grep -F 'outside sentinel' "$OAW_OUTSIDE/sentinel" >/dev/null ||
  fail "symlinked state root changed the outside sentinel"
[ ! -e "$OAW_CONFIG/open-agent-workflow/POLICY.md" ] ||
  fail "rejected symlinked state install created canonical policy"
[ ! -e "$OAW_HOME/.claude/CLAUDE.md" ] ||
  fail "rejected symlinked state install created a target"

pass "Go management CLI rejects state-root symlink containment escapes"

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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
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
OAW_POLICY=$OAW_PROJECT_PHYSICAL/.oaw/policy/POLICY.md
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
OAW_POLICY=$OAW_CONFIG/open-agent-workflow/POLICY.md
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
assert_status 0 "scope-independent user update"
assert_contains "unchanged: codex" "scope-independent update keeps the user adapter unchanged"
[ -L "$OAW_PROJECT_STATE" ] || fail "scope-independent update replaced the unrelated project state symlink"
assert_artifact_snapshot "$OAW_OUTSIDE_STATE" "$OAW_OUTSIDE_STATE_BEFORE" \
  "scope-independent update with unrelated project state"
assert_artifact_snapshot "$OAW_PROJECT_TARGET" "$OAW_PROJECT_TARGET_BEFORE" \
  "scope-independent update with unrelated project state"
assert_artifact_snapshot "$OAW_USER_STATE" "$OAW_USER_STATE_BEFORE" \
  "scope-independent update with unrelated project state"
assert_artifact_snapshot "$OAW_USER_TARGET" "$OAW_USER_TARGET_BEFORE" \
  "scope-independent update with unrelated project state"
assert_artifact_snapshot "$OAW_POLICY" "$OAW_POLICY_BEFORE" \
  "scope-independent update with unrelated project state"

pass "unrelated scope state cannot veto a managed Policy Set update"
