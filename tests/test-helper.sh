#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
OAW_REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
OAW_INSTALLER=${OAW_INSTALLER:-"$OAW_REPOSITORY/install.sh"}

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'PASS: %s\n' "$*"
}

assert_status() {
  expected_status=$1
  description=$2
  if [ "$OAW_STATUS" -ne "$expected_status" ]; then
    fail "$description (expected status $expected_status, got $OAW_STATUS; output: $OAW_OUTPUT)"
  fi
}

assert_contains() {
  expected_text=$1
  description=$2
  case "$OAW_OUTPUT" in
    *"$expected_text"*) ;;
    *) fail "$description (missing '$expected_text'; output: $OAW_OUTPUT)" ;;
  esac
}

assert_output_equals() {
  expected_output=$1
  description=$2
  if [ "$OAW_OUTPUT" != "$expected_output" ]; then
    fail "$description (expected: $expected_output; got: $OAW_OUTPUT)"
  fi
}

assert_empty_dir() {
  directory=$1
  description=$2
  if [ -n "$(find "$directory" -mindepth 1 -print -quit)" ]; then
    fail "$description ($directory is not empty)"
  fi
}

setup_sandbox() {
  OAW_SANDBOX=$(mktemp -d "${TMPDIR:-/tmp}/oaw-test.XXXXXX")
  OAW_HOME=$OAW_SANDBOX/home
  OAW_CONFIG=$OAW_SANDBOX/config
  OAW_STATE=$OAW_SANDBOX/state
  OAW_PROJECT=$OAW_SANDBOX/project
  OAW_PATH=${OAW_PATH:-$PATH}
  mkdir -p "$OAW_HOME" "$OAW_CONFIG" "$OAW_STATE" "$OAW_PROJECT"
}

cleanup_sandbox() {
  if [ -n "${OAW_SANDBOX:-}" ] && [ -d "$OAW_SANDBOX" ]; then
    rm -rf "$OAW_SANDBOX"
  fi
}

run_oaw() {
  OAW_OUTPUT_FILE=$OAW_SANDBOX/output
  set +e
  HOME="$OAW_HOME" \
    XDG_CONFIG_HOME="$OAW_CONFIG" \
    XDG_STATE_HOME="$OAW_STATE" \
    PATH="$OAW_PATH" \
    bash "$OAW_INSTALLER" "$@" >"$OAW_OUTPUT_FILE" 2>&1
  OAW_STATUS=$?
  set -e
  OAW_OUTPUT=$(cat "$OAW_OUTPUT_FILE")
}

assert_read_only_roots() {
  assert_empty_dir "$OAW_HOME" "HOME must remain unchanged"
  assert_empty_dir "$OAW_CONFIG" "XDG_CONFIG_HOME must remain unchanged"
  assert_empty_dir "$OAW_STATE" "XDG_STATE_HOME must remain unchanged"
}

assert_empty_xdg_roots() {
  assert_empty_dir "$OAW_CONFIG" "XDG_CONFIG_HOME must remain unchanged"
  assert_empty_dir "$OAW_STATE" "XDG_STATE_HOME must remain unchanged"
}
