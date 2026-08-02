#!/usr/bin/env bash

set -eu

WSL_TEMP=

cleanup() {
  if [ -n "$WSL_TEMP" ] && [ -d "$WSL_TEMP" ]; then
    rm -rf -- "$WSL_TEMP"
  fi
}

fail() {
  printf 'WSL smoke: error: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT HUP INT TERM

if [ ! -r /proc/sys/kernel/osrelease ] ||
  ! grep -qi microsoft /proc/sys/kernel/osrelease; then
  printf 'SKIP: WSL smoke requires an actual Microsoft WSL kernel\n' >&2
  exit 77
fi

[ "$#" -eq 1 ] || fail "usage: scripts/smoke-wsl.sh <absolute-linux-release-archive>"
ARCHIVE=$1
case "$ARCHIVE" in
  /*) ;;
  *) fail "release archive path must be absolute" ;;
esac
[ -f "$ARCHIVE" ] || fail "release archive does not exist: $ARCHIVE"

WSL_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-wsl-smoke.XXXXXX") ||
  fail "cannot create smoke directory"
if tar -tzf "$ARCHIVE" | grep -E '(^/|(^|/)\.\.(/|$))' >/dev/null; then
  fail "release archive contains an unsafe path"
fi
tar -xzf "$ARCHIVE" -C "$WSL_TEMP"
PACKAGE=$(find "$WSL_TEMP" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$PACKAGE" ] || fail "release archive has no package directory"
[ ! -L "$PACKAGE/oaw" ] || fail "release binary must not be a symlink"
[ ! -L "$PACKAGE/install.sh" ] || fail "release wrapper must not be a symlink"
[ -x "$PACKAGE/oaw" ] || fail "release archive has no executable Linux oaw binary"
[ -x "$PACKAGE/install.sh" ] || fail "release archive has no executable wrapper"

"$PACKAGE/oaw" --help >"$WSL_TEMP/help.stdout"
bash "$PACKAGE/install.sh" --help >"$WSL_TEMP/wrapper-help.stdout"
cmp -s "$WSL_TEMP/help.stdout" "$WSL_TEMP/wrapper-help.stdout" ||
  fail "wrapper help differs from binary help"
"$PACKAGE/oaw" catalog validate >"$WSL_TEMP/catalog.stdout"
grep -F 'catalog valid' "$WSL_TEMP/catalog.stdout" >/dev/null ||
  fail "catalog validation failed"

SMOKE_HOME=$WSL_TEMP/home
SMOKE_CONFIG=$WSL_TEMP/config
SMOKE_STATE=$WSL_TEMP/state
SMOKE_PROJECT=$WSL_TEMP/policy-only-project
POLICY_ONLY=$SMOKE_PROJECT/.scratch/existing-task/workflow.md
mkdir -p "$SMOKE_HOME" "$SMOKE_CONFIG" "$SMOKE_STATE" "$(dirname -- "$POLICY_ONLY")"
printf 'profile: ECC-FULL\nstage: implementation\n' >"$POLICY_ONLY"
POLICY_ONLY_BEFORE=$(cksum <"$POLICY_ONLY")

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  bash "$PACKAGE/install.sh" install --project "$SMOKE_PROJECT" --target cursor \
  >"$WSL_TEMP/install.stdout"
find "$SMOKE_STATE/open-agent-workflow/installations/projects" -type f -name '*.state' \
  -print -quit | grep . >/dev/null || fail "install did not create project Install State"
[ ! -e "$SMOKE_STATE/open-agent-workflow/runtime" ] ||
  fail "management imported Install State into Runtime State"
[ "$(cksum <"$POLICY_ONLY")" = "$POLICY_ONLY_BEFORE" ] ||
  fail "management changed the Policy-only task"

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  bash "$PACKAGE/install.sh" uninstall --project "$SMOKE_PROJECT" --target cursor \
  >"$WSL_TEMP/uninstall.stdout"
[ "$(cksum <"$POLICY_ONLY")" = "$POLICY_ONLY_BEFORE" ] ||
  fail "uninstall changed the Policy-only task"
[ ! -e "$SMOKE_STATE/open-agent-workflow/runtime" ] ||
  fail "uninstall created Runtime State"

printf 'PASS: WSL release binary, wrapper, Install State, and Policy-only boundaries verified\n'
