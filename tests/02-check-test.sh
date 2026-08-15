#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

OAW_TEST_BASH=$(command -v bash)
OAW_TEST_DIRNAME=$(command -v dirname)
OAW_TEST_CKSUM=$(command -v cksum)
OAW_TEST_AWK=$(command -v awk)

new_fixture() {
  cleanup_sandbox
  OAW_PATH=$PATH
  setup_sandbox
  mkdir -p "$OAW_SANDBOX/system-bin"
  ln -s "$OAW_TEST_BASH" "$OAW_SANDBOX/system-bin/bash"
  ln -s "$OAW_TEST_DIRNAME" "$OAW_SANDBOX/system-bin/dirname"
  ln -s "$OAW_TEST_CKSUM" "$OAW_SANDBOX/system-bin/cksum"
  ln -s "$OAW_TEST_AWK" "$OAW_SANDBOX/system-bin/awk"
  OAW_PATH=$OAW_SANDBOX/system-bin
}

make_fake_executable() {
  executable_name=$1
  mkdir -p "$OAW_SANDBOX/bin"
  printf '%s\n' '#!/bin/sh' 'exit 0' >"$OAW_SANDBOX/bin/$executable_name"
  chmod +x "$OAW_SANDBOX/bin/$executable_name"
  # shellcheck disable=SC2034 # Read by run_oaw from test-helper.sh.
  OAW_PATH=$OAW_SANDBOX/bin:$OAW_SANDBOX/system-bin
}

new_fixture
OAW_SOURCE_VERSION=$(tr -d '\r\n' <"$OAW_REPOSITORY/VERSION")
run_oaw check
assert_status 0 "an empty fixture is diagnostic only"
assert_output_equals "$(printf '%s\n' \
  "version: $OAW_SOURCE_VERSION" \
  'scope: user' \
  'targets: claude,codex,gemini,opencode' \
  'target claude: missing (user, project)' \
  'target codex: missing (user, project)' \
  'target gemini: missing (user, project)' \
  'target opencode: missing (user, project)' \
  'installed claude: not-installed' \
  'installed codex: not-installed' \
  'installed gemini: not-installed' \
  'installed opencode: not-installed')" "empty detection output is stable"
assert_read_only_roots
pass "empty readiness is reported without mutation"

mkdir -p "$OAW_HOME/.claude"
run_oaw check --target claude
assert_status 0 "a core instruction root is accepted as tool evidence"
assert_contains "target claude: detected (user, project)" "Claude is detected from its instruction root"
assert_empty_xdg_roots
pass "Host instruction roots are detected"

new_fixture

for host_tool in claude codex gemini opencode; do
  make_fake_executable "$host_tool"
done

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
run_oaw check --project "$OAW_PROJECT"
assert_status 0 "all detected Host targets remain diagnostic"
assert_output_equals "$(printf '%s\n' \
  "version: $OAW_SOURCE_VERSION" \
  "scope: project ($OAW_PROJECT_PHYSICAL)" \
  'targets: claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot' \
  'target claude: detected (user, project)' \
  'target codex: detected (user, project)' \
  'target gemini: detected (user, project)' \
  'target opencode: detected (user, project)' \
  'target cursor: adapter-only (project)' \
  'target windsurf: adapter-only (project)' \
  'target cline: adapter-only (project)' \
  'target roo: adapter-only (project)' \
  'target copilot: adapter-only (project)' \
  'installed claude: not-installed' \
  'installed codex: not-installed' \
  'installed gemini: not-installed' \
  'installed opencode: not-installed' \
  'installed cursor: not-installed' \
  'installed windsurf: not-installed' \
  'installed cline: not-installed' \
  'installed roo: not-installed' \
  'installed copilot: not-installed')" "all target readiness output is stable"
assert_empty_xdg_roots
pass "all target readiness indicators are reported without mutation"
