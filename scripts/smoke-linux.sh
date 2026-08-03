#!/usr/bin/env bash

set -eu

LINUX_SMOKE_TEMP=

cleanup() {
  if [ -n "$LINUX_SMOKE_TEMP" ] && [ -d "$LINUX_SMOKE_TEMP" ]; then
    rm -rf -- "$LINUX_SMOKE_TEMP"
  fi
}

fail() {
  printf 'Linux smoke: error: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT HUP INT TERM

[ "$#" -eq 1 ] || fail "usage: scripts/smoke-linux.sh <absolute-linux-release-archive>"
ARCHIVE=$1
case "$ARCHIVE" in
  /*) ;;
  *) fail "release archive path must be absolute" ;;
esac
[ -f "$ARCHIVE" ] || fail "release archive does not exist: $ARCHIVE"
[ ! -L "$ARCHIVE" ] || fail "release archive must not be a symlink"

LINUX_SMOKE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-linux-smoke.XXXXXX") ||
  fail "cannot create smoke directory"
if tar -tzf "$ARCHIVE" | grep -E '(^/|(^|/)\.\.(/|$))' >/dev/null; then
  fail "release archive contains an unsafe path"
fi
tar -xzf "$ARCHIVE" -C "$LINUX_SMOKE_TEMP"
PACKAGE=$(find "$LINUX_SMOKE_TEMP" -mindepth 1 -maxdepth 1 -type d -print -quit)
[ -n "$PACKAGE" ] || fail "release archive has no package directory"
[ ! -L "$PACKAGE/oaw" ] || fail "release binary must not be a symlink"
[ ! -L "$PACKAGE/install.sh" ] || fail "release wrapper must not be a symlink"
[ -x "$PACKAGE/oaw" ] || fail "release archive has no executable Linux oaw binary"
[ -x "$PACKAGE/install.sh" ] || fail "release archive has no executable wrapper"

"$PACKAGE/oaw" --help >"$LINUX_SMOKE_TEMP/help.stdout"
bash "$PACKAGE/install.sh" --help >"$LINUX_SMOKE_TEMP/wrapper-help.stdout"
cmp -s "$LINUX_SMOKE_TEMP/help.stdout" "$LINUX_SMOKE_TEMP/wrapper-help.stdout" ||
  fail "wrapper help differs from binary help"
"$PACKAGE/oaw" catalog validate >"$LINUX_SMOKE_TEMP/catalog.stdout"
grep -F 'catalog valid' "$LINUX_SMOKE_TEMP/catalog.stdout" >/dev/null ||
  fail "catalog validation failed"

SMOKE_HOME=$LINUX_SMOKE_TEMP/home
SMOKE_CONFIG=$LINUX_SMOKE_TEMP/config
SMOKE_STATE=$LINUX_SMOKE_TEMP/state
SMOKE_PROJECT=$LINUX_SMOKE_TEMP/policy-only-project
POLICY_ONLY=$SMOKE_PROJECT/.scratch/existing-task/workflow.md
mkdir -p "$SMOKE_HOME" "$SMOKE_CONFIG" "$SMOKE_STATE" "$(dirname -- "$POLICY_ONLY")"
printf 'profile: ECC-FULL\nstage: implementation\n' >"$POLICY_ONLY"
POLICY_ONLY_BEFORE=$(cksum <"$POLICY_ONLY")

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  bash "$PACKAGE/install.sh" install --project "$SMOKE_PROJECT" --target cursor \
  >"$LINUX_SMOKE_TEMP/install.stdout"
find "$SMOKE_STATE/open-agent-workflow/installations/projects" -type f -name '*.state' \
  -print -quit | grep . >/dev/null || fail "install did not create project Install State"
[ ! -e "$SMOKE_STATE/open-agent-workflow/runtime" ] ||
  fail "management imported Install State into Runtime State"
[ "$(cksum <"$POLICY_ONLY")" = "$POLICY_ONLY_BEFORE" ] ||
  fail "management changed the Policy-only task"

HOME="$SMOKE_HOME" XDG_CONFIG_HOME="$SMOKE_CONFIG" XDG_STATE_HOME="$SMOKE_STATE" \
  bash "$PACKAGE/install.sh" uninstall --project "$SMOKE_PROJECT" --target cursor \
  >"$LINUX_SMOKE_TEMP/uninstall.stdout"
[ "$(cksum <"$POLICY_ONLY")" = "$POLICY_ONLY_BEFORE" ] ||
  fail "uninstall changed the Policy-only task"
[ ! -e "$SMOKE_STATE/open-agent-workflow/runtime" ] ||
  fail "uninstall created Runtime State"

printf 'PASS: Linux release binary, wrapper, Install State, and Policy-only boundaries verified\n'
