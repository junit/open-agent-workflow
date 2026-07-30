#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

OAW_TEST_BASH=$(command -v bash)
OAW_TEST_DIRNAME=$(command -v dirname)

new_fixture() {
  cleanup_sandbox
  OAW_PATH=$PATH
  setup_sandbox
  mkdir -p "$OAW_SANDBOX/system-bin"
  ln -s "$OAW_TEST_BASH" "$OAW_SANDBOX/system-bin/bash"
  ln -s "$OAW_TEST_DIRNAME" "$OAW_SANDBOX/system-bin/dirname"
  OAW_PATH=$OAW_SANDBOX/system-bin
}

make_indicator() {
  indicator_path=$1
  mkdir -p "$(dirname -- "$indicator_path")"
  : >"$indicator_path"
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
run_oaw check
assert_status 0 "an empty fixture is diagnostic only"
assert_output_equals "$(printf '%s\n' \
  'version: 0.1.0' \
  'scope: user' \
  'targets: claude,codex,gemini,opencode' \
  'provider superpowers: missing' \
  'provider matt: missing' \
  'provider ecc: missing' \
  'target claude: missing (user, project)' \
  'target codex: missing (user, project)' \
  'target gemini: missing (user, project)' \
  'target opencode: missing (user, project)')" "empty detection output is stable"
assert_read_only_roots
pass "empty readiness is reported without mutation"

mkdir -p "$OAW_HOME/.claude"
run_oaw check --target claude
assert_status 0 "a core instruction root is accepted as tool evidence"
assert_contains "target claude: detected (user, project)" "Claude is detected from its instruction root"
assert_empty_xdg_roots
pass "core instruction roots are detected"

new_fixture
for matt_skill in to-spec to-tickets tdd; do
  make_indicator "$OAW_HOME/.agents/skills/$matt_skill/SKILL.md"
done

run_oaw check --target claude
assert_status 0 "an incomplete Matt bundle does not fail check"
assert_contains "provider matt: missing" "Matt requires the complete capability bundle"
assert_contains "provider superpowers: missing" "Matt files do not imply Superpowers"
assert_contains "provider ecc: missing" "Matt files do not imply ECC"
assert_empty_xdg_roots

make_indicator "$OAW_HOME/.agents/skills/diagnosing-bugs/SKILL.md"
run_oaw check --target claude
assert_status 0 "the complete Matt bundle is diagnostic only"
assert_contains "provider matt: detected" "Matt is detected only with all four skills"
assert_contains "provider superpowers: missing" "the Matt-only fixture leaves Superpowers missing"
assert_contains "provider ecc: missing" "the Matt-only fixture leaves ECC missing"
assert_contains "target claude: missing (user, project)" "provider presence does not imply tool presence"
assert_empty_xdg_roots
pass "Matt capability detection is exact"

new_fixture
for matt_skill in to-spec to-tickets tdd diagnosing-bugs; do
  make_indicator "$OAW_HOME/.agents/skills/$matt_skill/SKILL.md"
done
make_indicator "$OAW_HOME/.codex/plugins/cache/openai-api-curated/superpowers/test-build/skills/using-superpowers/SKILL.md"
make_indicator "$OAW_HOME/.agents/skills/everything-claude-code/SKILL.md"

for core_tool in claude codex gemini opencode; do
  make_fake_executable "$core_tool"
done

OAW_PROJECT_PHYSICAL=$(CDPATH='' cd -P -- "$OAW_PROJECT" && pwd -P)
run_oaw check --project "$OAW_PROJECT"
assert_status 0 "all detected providers and tools remain diagnostic"
assert_output_equals "$(printf '%s\n' \
  'version: 0.1.0' \
  "scope: project ($OAW_PROJECT_PHYSICAL)" \
  'targets: claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot' \
  'provider superpowers: detected' \
  'provider matt: detected' \
  'provider ecc: detected' \
  'target claude: detected (user, project)' \
  'target codex: detected (user, project)' \
  'target gemini: detected (user, project)' \
  'target opencode: detected (user, project)' \
  'target cursor: adapter-only (project)' \
  'target windsurf: adapter-only (project)' \
  'target cline: adapter-only (project)' \
  'target roo: adapter-only (project)' \
  'target copilot: adapter-only (project)')" "all-provider detection output is stable"
assert_empty_xdg_roots
pass "all readiness indicators are reported without mutation"
