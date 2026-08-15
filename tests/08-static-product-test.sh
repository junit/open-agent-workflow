#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

trap cleanup_sandbox EXIT HUP INT TERM
setup_sandbox

for retired_package in policyflow policyroute policyengagement policyrun; do
  [ ! -e "$OAW_REPOSITORY/internal/$retired_package" ] ||
    fail "retired Policy state package remains: internal/$retired_package"
done

for retired_authority in \
  admission classification config coordinator core dogfood execution host \
  hosttest policycatalog profile registry schema; do
  [ ! -e "$OAW_REPOSITORY/internal/$retired_authority" ] ||
    fail "retired machine workflow authority remains: internal/$retired_authority"
done

for retired_asset in \
  cmd/oaw-dogfood internal/assets/generate internal/assets/host-integrations.json \
  internal/assets/profile-aliases.json internal/assets/profile-matrix.json \
  internal/assets/recipes internal/assets/schemas/v1/classification-proposal.schema.json \
  internal/assets/schemas/v1/profile-alias-set.schema.json \
  internal/assets/schemas/v1/project-config.schema.json \
  internal/assets/schemas/v3/profile-recipe.schema.json \
  internal/assets/schemas/v3/user-config.schema.json \
  internal/assets/schemas/v4/provider-descriptor.schema.json; do
  [ ! -e "$OAW_REPOSITORY/$retired_asset" ] ||
    fail "retired duplicate workflow asset remains: $retired_asset"
done

PROGRESS_NOTE="$OAW_PROJECT/.scratch/progress.md"
mkdir -p "$(dirname -- "$PROGRESS_NOTE")"
printf '%s\n' 'profile: MATT-SP-HYBRID' 'next: model-owned work' >"$PROGRESS_NOTE"
PROGRESS_NOTE_BEFORE=$(cksum <"$PROGRESS_NOTE")

run_oaw --help
assert_status 0 "static CLI help"
for command in \
  'oaw profile list' \
  'oaw profile show' \
  'oaw profile check'; do
  assert_contains "$command" "help exposes $command"
done
for forbidden in \
  'oaw profiles' 'oaw use' 'oaw status' 'oaw workflow' 'oaw providers' \
  'oaw catalog' --complexity --risk topology add-on; do
  case "$OAW_OUTPUT" in
    *"$forbidden"*) fail "static CLI help contains removed lifecycle surface: $forbidden" ;;
  esac
done
assert_read_only_roots

run_oaw profile list
assert_status 0 "advisory Profile list"
for profile in SP-FULL MATT-FULL ECC-FULL MATT-SP-HYBRID; do
  assert_contains "built-in:$profile" "$profile is available from the static Policy Set"
done
assert_read_only_roots

run_oaw profile show built-in:MATT-SP-HYBRID
assert_status 0 "show built-in Hybrid Profile"
assert_contains "id: MATT-SP-HYBRID" "Profile show preserves the selected identity"
assert_read_only_roots

run_oaw profile check built-in:MATT-SP-HYBRID
assert_status 0 "check built-in Hybrid Profile"
assert_contains "result: metadata-valid" "Profile check validates static metadata"
assert_contains "Skill availability: not evaluated" "Profile check remains advisory"
assert_read_only_roots

for removed_command in \
  profiles use status complete review approve satisfy incident switch stop uncertain \
  workflow providers policy catalog bridge runtime run; do
  run_oaw "$removed_command"
  assert_status 64 "$removed_command is absent"
  assert_contains "unknown command '$removed_command'" "$removed_command has no compatibility wrapper"
  assert_read_only_roots
done

[ ! -e "$OAW_STATE/open-agent-workflow/workflows" ] ||
  fail "static CLI created Workflow State"
[ ! -e "$OAW_STATE/open-agent-workflow/policy-engagements" ] ||
  fail "static CLI created Policy Engagement state"
[ "$(cksum <"$PROGRESS_NOTE")" = "$PROGRESS_NOTE_BEFORE" ] ||
  fail "static CLI changed the model-owned Progress Note"

pass "default oaw exposes only static inspection and leaves model-owned progress untouched"
