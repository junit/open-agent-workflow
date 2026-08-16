#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
# shellcheck source=tests/test-helper.sh
. "$TEST_DIR/test-helper.sh"

OAW_NATIVE_SUITE_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/oaw-native-dogfood.XXXXXX")

cleanup_native_suite() {
  if [ -n "${OAW_NATIVE_SUITE_ROOT:-}" ] && [ -d "$OAW_NATIVE_SUITE_ROOT" ]; then
    rm -rf "$OAW_NATIVE_SUITE_ROOT"
  fi
}

trap cleanup_native_suite EXIT HUP INT TERM

OAW_NATIVE_RELEASE=$OAW_NATIVE_SUITE_ROOT/release
mkdir -p "$OAW_NATIVE_RELEASE"
cp "$OAW_BASE_INSTALLER" "$OAW_NATIVE_RELEASE/install.sh"
(
  cd "$OAW_REPOSITORY"
  go build -o "$OAW_NATIVE_RELEASE/oaw" ./cmd/oaw
)
chmod 755 "$OAW_NATIVE_RELEASE/install.sh" "$OAW_NATIVE_RELEASE/oaw"
OAW_INSTALLER=$OAW_NATIVE_RELEASE/install.sh

router_path() {
  host=$1
  root=$2
  artifact_scope=$3
  if [ "$artifact_scope" = user ]; then
    case "$host" in
      claude) printf '%s/.claude/CLAUDE.md\n' "$root" ;;
      codex) printf '%s/.codex/AGENTS.md\n' "$root" ;;
      gemini) printf '%s/.gemini/GEMINI.md\n' "$root" ;;
      opencode) printf '%s/opencode/AGENTS.md\n' "$root" ;;
      *) fail "unknown user Router host: $host" ;;
    esac
    return
  fi
  case "$host" in
    claude) printf '%s/.claude/CLAUDE.md\n' "$root" ;;
    codex|opencode) printf '%s/AGENTS.md\n' "$root" ;;
    gemini) printf '%s/GEMINI.md\n' "$root" ;;
    cursor) printf '%s/.cursor/rules/open-agent-workflow.mdc\n' "$2" ;;
    windsurf) printf '%s/.devin/rules/open-agent-workflow.md\n' "$2" ;;
    cline) printf '%s/.clinerules/open-agent-workflow.md\n' "$2" ;;
    roo) printf '%s/.roo/rules/open-agent-workflow.md\n' "$2" ;;
    copilot) printf '%s/.github/instructions/open-agent-workflow.instructions.md\n' "$2" ;;
    *) fail "unknown router host: $1" ;;
  esac
}

native_path() {
  host=$1
  root=$2
  artifact_scope=$3
  if [ "$artifact_scope" = user ]; then
    case "$host" in
      claude) printf '%s/.claude/skills/oaw/SKILL.md\n' "$root" ;;
      codex) printf '%s/.agents/skills/oaw/SKILL.md\n' "$root" ;;
      gemini) printf '%s/.gemini/commands/oaw.toml\n' "$root" ;;
      opencode) printf '%s/opencode/commands/oaw.md\n' "$root" ;;
      *) fail "unknown user native host: $host" ;;
    esac
    return
  fi
  case "$host" in
    claude) printf '%s/.claude/skills/oaw/SKILL.md\n' "$root" ;;
    codex) printf '%s/.agents/skills/oaw/SKILL.md\n' "$root" ;;
    gemini) printf '%s/.gemini/commands/oaw.toml\n' "$root" ;;
    opencode) printf '%s/.opencode/commands/oaw.md\n' "$root" ;;
    cursor) printf '%s/.cursor/skills/oaw/SKILL.md\n' "$2" ;;
    windsurf) printf '%s/.windsurf/workflows/oaw.md\n' "$2" ;;
    cline) printf '%s/.cline/skills/oaw/SKILL.md\n' "$2" ;;
    roo) printf '%s/.roo/commands/oaw.md\n' "$2" ;;
    copilot) printf '%s/.github/skills/oaw/SKILL.md\n' "$2" ;;
    *) fail "unknown native host: $1" ;;
  esac
}

native_policy_path() {
  if [ "$1" = codex ]; then
    printf '%s/.agents/skills/oaw/agents/openai.yaml\n' "$2"
  fi
}

assert_router() {
  router=$1
  [ -f "$router" ] || fail "missing Activation Router: $router"
  grep -F 'Open Agent Workflow is opt-in.' "$router" >/dev/null ||
    fail "Router is not opt-in: $router"
  grep -F 'behave as the native Host' "$router" >/dev/null ||
    fail "Router does not preserve native Host behavior: $router"
  grep -F 'ordinary Skill invocation do not activate OAW' "$router" >/dev/null ||
    fail "Router changes ordinary Skill activation: $router"
}

assert_thin_entrypoint() {
  entrypoint=$1
  host=$2
  policy_reference=$3

  [ -f "$entrypoint" ] || fail "missing native entrypoint for $host: $entrypoint"
  grep -F "evidence must come from outside this dispatcher's bytes and any Host-expanded template text" "$entrypoint" >/dev/null ||
    fail "$host native entrypoint accepts self-originating activation evidence"
  grep -F 'quoted or discussed invocation text' "$entrypoint" >/dev/null ||
    fail "$host native entrypoint does not reject quoted or discussed invocations"
  grep -F 'model-led invocation or loading' "$entrypoint" >/dev/null ||
    fail "$host native entrypoint does not reject model-led invocation"
  grep -F 'user provenance is unavailable or ambiguous' "$entrypoint" >/dev/null ||
    fail "$host native entrypoint lacks ambiguous-source handling"
  grep -F 'do not activate OAW and continue as the native Host' "$entrypoint" >/dev/null ||
    fail "$host native entrypoint lacks the automatic-load no-op behavior"
  grep -F 'Follow the current OAW Activation Router to select and read one Policy Set' "$entrypoint" >/dev/null ||
    fail "$host native entrypoint bypasses the Activation Router"
  grep -F 'Pass the optional Profile and task' "$entrypoint" >/dev/null ||
    fail "$host native entrypoint does not pass Profile and task through"
  if grep -F '.oaw/policy/POLICY.md' "$entrypoint" >/dev/null ||
    grep -F "$policy_reference" "$entrypoint" >/dev/null; then
    fail "$host native entrypoint embeds a Host-preprocessed Policy path"
  fi
  if grep -F '/oaw' "$entrypoint" >/dev/null || grep -F '$oaw' "$entrypoint" >/dev/null; then
    fail "$host native entrypoint contains a self-authorizing invocation literal"
  fi

  case "$host" in
    claude|opencode)
      grep -F '$ARGUMENTS' "$entrypoint" >/dev/null ||
        fail "$host native entrypoint omits its documented argument expansion"
      ;;
    gemini)
      grep -F '{{args}}' "$entrypoint" >/dev/null ||
        fail "$host native entrypoint omits Gemini argument expansion"
      ;;
    *)
      grep -F 'remainder of this user request' "$entrypoint" >/dev/null ||
        fail "$host native entrypoint omits remainder-of-request forwarding"
      ;;
  esac

  if [ "$host" = copilot ]; then
    grep -F 'disable-model-invocation: true' "$entrypoint" >/dev/null ||
      fail "Copilot native entrypoint does not disable model invocation"
    grep -F 'argument-hint: "[PROFILE] <task>"' "$entrypoint" >/dev/null ||
      fail "Copilot native entrypoint omits its argument hint"
  fi

  for forbidden in MATT-FULL MATT-SP-HYBRID SP-FULL ECC-FULL; do
    if grep -F "$forbidden" "$entrypoint" >/dev/null; then
      fail "$host native entrypoint hard-codes Profile $forbidden"
    fi
  done
  if grep -E 'Spec[[:space:][:punct:]]*.?[[:space:][:punct:]]*Plan[[:space:][:punct:]]*.?[[:space:][:punct:]]*TDD|must wait for (user )?approval' "$entrypoint" >/dev/null; then
    fail "$host native entrypoint duplicates a lifecycle or approval gate"
  fi
  if grep -E 'oaw (install|check|update|uninstall|bridge|runtime)' "$entrypoint" >/dev/null; then
    fail "$host native entrypoint depends on the OAW binary"
  fi
}

assert_codex_native_policy() {
  policy=$1
  [ -f "$policy" ] || fail "missing Codex native policy metadata: $policy"
  grep -F 'allow_implicit_invocation' "$policy" >/dev/null ||
    fail "Codex native policy does not disable implicit invocation"
}

assert_policy_set() {
  policy_root=$1
  profile_root=$policy_root/profiles
  if [ -d "$profile_root/builtin" ]; then
    profile_root=$profile_root/builtin
  fi
  [ -f "$policy_root/POLICY.md" ] || fail "missing Canonical Policy Set: $policy_root/POLICY.md"
  [ -f "$policy_root/cooperative-protocol.md" ] || fail "missing cooperative protocol"
  for host in claude codex gemini opencode cursor windsurf cline roo copilot; do
    [ -f "$policy_root/adapters/$host-policy.md" ] ||
      fail "missing $host Policy Adapter"
  done
  for profile in MATT-FULL MATT-SP-HYBRID SP-FULL ECC-FULL; do
    [ -f "$profile_root/$profile.md" ] || fail "missing built-in Profile $profile"
    grep -F "$profile" "$profile_root/$profile.md" >/dev/null ||
      fail "built-in Profile $profile is unreadable"
  done
}

assert_project_artifacts() {
  project=$1
  policy_root=$project/.oaw/policy
  assert_policy_set "$policy_root"
  for host in claude codex gemini opencode cursor windsurf cline roo copilot; do
    assert_router "$(router_path "$host" "$project" project)"
    assert_thin_entrypoint "$(native_path "$host" "$project" project)" "$host" '.oaw/policy/POLICY.md'
    native_policy=$(native_policy_path "$host" "$project")
    if [ -n "$native_policy" ]; then
      assert_codex_native_policy "$native_policy"
    fi
  done
}

assert_user_artifacts() {
  policy_root=$OAW_CONFIG/open-agent-workflow
  assert_policy_set "$policy_root"
  for host in claude codex gemini opencode; do
    artifact_root=$OAW_HOME
    if [ "$host" = opencode ]; then
      artifact_root=$OAW_CONFIG
    fi
    assert_router "$(router_path "$host" "$artifact_root" user)"
    assert_thin_entrypoint "$(native_path "$host" "$artifact_root" user)" "$host" "$policy_root/POLICY.md"
    native_policy=$(native_policy_path "$host" "$artifact_root")
    if [ -n "$native_policy" ]; then
      assert_codex_native_policy "$native_policy"
    fi
  done
}

snapshot_install_roots() {
  output=$1
  : >"$output"
  for root in "$OAW_HOME" "$OAW_CONFIG" "$OAW_STATE" "$OAW_PROJECT"; do
    printf 'ROOT %s\n' "$root" >>"$output"
    snapshot_tree "$root" >>"$output"
  done
}

# A project install must materialize every registered Host's Router and native
# entrypoint in one transaction, with no binary dependency in the result.
OAW_PROJECT_CASE=$OAW_NATIVE_SUITE_ROOT/project-install
setup_sandbox_at "$OAW_PROJECT_CASE"
run_oaw install --project "$OAW_PROJECT"
assert_status 0 "nine-Host project installation"
assert_project_artifacts "$OAW_PROJECT"

# The user scope keeps its four user-capable Hosts and uses the same static
# dispatcher contract, with project Policy precedence and a user fallback.
OAW_USER_CASE=$OAW_NATIVE_SUITE_ROOT/user-install
setup_sandbox_at "$OAW_USER_CASE"
run_oaw install
assert_status 0 "four-Host user installation"
assert_user_artifacts

# Native-file collisions are fail-closed: no Policy, Router, sibling target,
# state, or owned directory may be written before the conflict is reported.
OAW_PROJECT_COLLISION_CASE=$OAW_NATIVE_SUITE_ROOT/project-collision
setup_sandbox_at "$OAW_PROJECT_COLLISION_CASE"
mkdir -p "$OAW_PROJECT/.cursor/skills/oaw"
printf 'user-owned native entrypoint\n' >"$OAW_PROJECT/.cursor/skills/oaw/SKILL.md"
OAW_COLLISION_BEFORE=$OAW_NATIVE_SUITE_ROOT/project-collision.before
OAW_COLLISION_AFTER=$OAW_NATIVE_SUITE_ROOT/project-collision.after
snapshot_install_roots "$OAW_COLLISION_BEFORE"
run_oaw install --project "$OAW_PROJECT"
assert_status 65 "project native-entrypoint collision"
assert_contains 'owned target artifact already exists' "project native-entrypoint collision is explicit"
snapshot_install_roots "$OAW_COLLISION_AFTER"
cmp -s "$OAW_COLLISION_BEFORE" "$OAW_COLLISION_AFTER" ||
  fail "project native-entrypoint collision partially wrote installation artifacts"
pass "project native-entrypoint collision is atomic"

OAW_USER_COLLISION_CASE=$OAW_NATIVE_SUITE_ROOT/user-collision
setup_sandbox_at "$OAW_USER_COLLISION_CASE"
mkdir -p "$OAW_HOME/.agents/skills/oaw"
printf 'user-owned native entrypoint\n' >"$OAW_HOME/.agents/skills/oaw/SKILL.md"
OAW_COLLISION_BEFORE=$OAW_NATIVE_SUITE_ROOT/user-collision.before
OAW_COLLISION_AFTER=$OAW_NATIVE_SUITE_ROOT/user-collision.after
snapshot_install_roots "$OAW_COLLISION_BEFORE"
run_oaw install
assert_status 65 "user native-entrypoint collision"
assert_contains 'owned target artifact already exists' "user native-entrypoint collision is explicit"
snapshot_install_roots "$OAW_COLLISION_AFTER"
cmp -s "$OAW_COLLISION_BEFORE" "$OAW_COLLISION_AFTER" ||
  fail "user native-entrypoint collision partially wrote installation artifacts"
pass "user native-entrypoint collision is atomic"

# The static Policy Sets and all native dispatchers remain usable after the
# release binary is removed, proving that the native path is self-contained.
rm -f -- "$OAW_NATIVE_RELEASE/oaw"
[ ! -e "$OAW_NATIVE_RELEASE/oaw" ] || fail "dogfood release binary remains"
assert_project_artifacts "$OAW_PROJECT_CASE/project"
OAW_USER_CASE_PROJECT=$OAW_USER_CASE/project
assert_user_artifacts_at_user_case() {
  OAW_HOME=$OAW_USER_CASE/home
  OAW_CONFIG=$OAW_USER_CASE/config
  OAW_STATE=$OAW_USER_CASE/state
  OAW_PROJECT=$OAW_USER_CASE_PROJECT
  assert_user_artifacts
}
assert_user_artifacts_at_user_case

pass "nine-Host native entrypoints and Policy Sets survive binary removal"
