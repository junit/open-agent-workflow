#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM

write_skill() {
  skill_path=$1
  mkdir -p "$(dirname -- "$skill_path")"
  printf '%s\n' 'fixture' >"$skill_path"
}

setup_sandbox

mkdir -p "$OAW_HOME/.codex"
printf '%s\n' \
  '[plugins."superpowers@openai-api-curated"]' \
  'enabled = true' \
  '[plugins."ecc@ecc"]' \
  'enabled = true' \
  >"$OAW_HOME/.codex/config.toml"

for skill in \
  grill-with-docs grilling domain-modeling to-spec to-tickets implement tdd \
  diagnosing-bugs code-review; do
  write_skill "$OAW_HOME/.agents/skills/$skill/SKILL.md"
done

for skill in \
  brainstorming writing-plans using-git-worktrees executing-plans \
  test-driven-development systematic-debugging requesting-code-review \
  receiving-code-review verification-before-completion \
  finishing-a-development-branch; do
  write_skill "$OAW_HOME/.codex/plugins/cache/openai-api-curated/superpowers/v1/skills/$skill/SKILL.md"
done

for skill in \
  intent-driven-development product-capability blueprint git-workflow \
  tdd-workflow verification-loop; do
  write_skill "$OAW_HOME/.codex/plugins/cache/ecc/ecc/v1/skills/$skill/SKILL.md"
done

cd "$OAW_PROJECT"

run_oaw profiles
assert_status 0 "no-Bridge Profile inspection"
for profile in SP-FULL MATT-FULL ECC-FULL MATT-SP-HYBRID; do
  assert_contains "\"name\":\"$profile\"" "$profile is present"
done
case "$OAW_OUTPUT" in
  *'"host_routable":false'*) fail "a built-in Profile is not Host-routable: $OAW_OUTPUT" ;;
esac
assert_contains '"incident_routes":' "conditional incident diagnostics are public"
[ ! -e "$OAW_STATE/open-agent-workflow/policy-engagements" ] ||
  fail "read-only Profile inspection created Policy state"

run_oaw use --profile MATT-SP-HYBRID \
  --complexity ordinary --risk normal -- "numbered policy workflow"
assert_status 0 "start no-Bridge Hybrid Policy workflow"
assert_contains '"profile":"MATT-SP-HYBRID"' "selected Profile is recorded"
assert_contains '"state":"active"' "Policy workflow starts active"

run_oaw status
assert_status 0 "read active Policy status"
assert_contains '"profile":"MATT-SP-HYBRID"' "status preserves the selected Profile"

run_oaw complete
assert_status 0 "complete first Policy work item"

run_oaw stop --reason "numbered black-box complete"
assert_status 0 "stop Policy workflow explicitly"
assert_contains '"state":"stopped"' "Policy workflow records explicit stop"

POLICY_STATE_ROOT=$OAW_STATE/open-agent-workflow/policy-engagements
[ -n "$(find "$POLICY_STATE_ROOT" -type f -print -quit)" ] ||
  fail "Policy workflow did not persist project-scoped state"
[ -z "$(find "$OAW_CONFIG" -mindepth 1 -print -quit)" ] ||
  fail "Policy workflow mutated XDG_CONFIG_HOME"

cd "$TEST_DIR"
pass "no-Bridge Policy CLI routes all built-in Profiles and persists typed workflow progress"
