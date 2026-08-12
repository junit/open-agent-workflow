#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
DOCS_TEST_TEMP=

cleanup() {
  if [ -n "$DOCS_TEST_TEMP" ] && [ -d "$DOCS_TEST_TEMP" ]; then
    rm -rf -- "$DOCS_TEST_TEMP"
  fi
}

trap cleanup EXIT HUP INT TERM

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'PASS: %s\n' "$*"
}

assert_file() {
  local relative_path=$1

  [ -f "$REPOSITORY/$relative_path" ] ||
    fail "required governance file is missing: $relative_path"
}

assert_executable() {
  local relative_path=$1

  [ -x "$REPOSITORY/$relative_path" ] ||
    fail "required script is not executable: $relative_path"
}

assert_contains() {
  local relative_path=$1
  local expected_text=$2

  grep -F -- "$expected_text" "$REPOSITORY/$relative_path" >/dev/null ||
    fail "$relative_path is missing required text: $expected_text"
}

assert_not_contains() {
  local relative_path=$1
  local forbidden_text=$2

  if grep -F -- "$forbidden_text" "$REPOSITORY/$relative_path" >/dev/null; then
    fail "$relative_path contains forbidden stale text: $forbidden_text"
  fi
}

make_checker_fixture() {
  local fixture_root=$1
  local english_file
  local chinese_file
  local document_path

  mkdir -p "$fixture_root/scripts"
  cp "$REPOSITORY/scripts/check-docs.sh" "$fixture_root/scripts/check-docs.sh"

  while IFS='|' read -r english_file chinese_file; do
    [ -n "$english_file$chinese_file" ] || continue
    for document_path in "$english_file" "$chinese_file"; do
      mkdir -p "$(dirname -- "$fixture_root/$document_path")"
      : >"$fixture_root/$document_path"
    done
  done <<'EOF'
README.md|README-zh.md
CONTRIBUTING.md|CONTRIBUTING-zh.md
SECURITY.md|SECURITY-zh.md
docs/en/background.md|docs/zh/background.md
docs/en/comparison.md|docs/zh/comparison.md
docs/en/lifecycle.md|docs/zh/lifecycle.md
docs/en/architecture.md|docs/zh/architecture.md
docs/en/installer.md|docs/zh/installer.md
docs/en/codex-bridge.md|docs/zh/codex-bridge.md
docs/en/adapters.md|docs/zh/adapters.md
docs/en/extending-adapters.md|docs/zh/extending-adapters.md
docs/en/security.md|docs/zh/security.md
docs/en/troubleshooting.md|docs/zh/troubleshooting.md
EOF

  for document_path in README.md README-zh.md; do
    printf '%s\n' \
      './install.sh check' \
      './install.sh install' \
      './install.sh update' \
      './install.sh uninstall' >>"$fixture_root/$document_path"
  done
  printf '%s\n' \
    'Public installation management is Go-authoritative.' \
    '`install.sh` is an offline sibling-binary compatibility wrapper.' \
    'Release archives contain precompiled binaries and perform no runtime executable download.' \
    'Installation management distributes the canonical Policy and target-native instruction entrypoints; it does not execute engineering work.' \
    'OAW Core is required and stateless. The Workflow Coordinator is optional and stores only Workflow State for `WORKFLOW`; Install State and Workflow State are disjoint, with no migration or implicit adoption.' \
    'The Agent Host owns Agents, model calls, MCP, Hooks, Skills, Plugins, authentication, tools, sandbox, approvals, and every physical effect. OAW never starts a model process.' \
    'Codex has a policy integration by default and a separate audited host-native Bridge' \
    'Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness.' \
    >>"$fixture_root/README.md"
  printf '%s\n' \
    '公开安装管理以 Go 为权威实现。' \
    '`install.sh` 是离线的同目录二进制兼容包装器。' \
    '发布归档包含预编译二进制，运行时不会下载可执行文件。' \
    '安装管理只分发 canonical Policy 和 target-native 指令入口，不执行工程工作。' \
    'OAW Core 是必需且无状态的。Workflow Coordinator 是可选的，只为 `WORKFLOW` 保存' \
    'Agent Host 拥有 Agent、model call、MCP、Hook、Skill、Plugin、认证、工具、sandbox、' \
    'Codex 默认提供 policy integration，并另有独立且经过审计的 host-native Bridge' \
    '可用的原生和 Docker smoke test 必须通过；不可用的平台检查返回 77，且不阻塞 release readiness。' \
    >>"$fixture_root/README-zh.md"
  mkdir -p "$fixture_root/policy"
  : >"$fixture_root/policy/ENGINEERING.md"
  for document_path in \
    README.md README-zh.md \
    docs/en/architecture.md docs/zh/architecture.md \
    docs/en/lifecycle.md docs/zh/lifecycle.md \
    docs/en/troubleshooting.md docs/zh/troubleshooting.md \
    docs/en/security.md docs/zh/security.md \
    policy/ENGINEERING.md; do
    printf '%s\n' \
      'Provider Family' \
      'Distribution' \
      'Host Installation' \
      'Host Binding Evidence' \
      'Verified Provider Instance' \
      >>"$fixture_root/$document_path"
  done
  printf '%s\n' 'oaw/codex-host' \
    >>"$fixture_root/docs/en/architecture.md"
  printf '%s\n' 'oaw/codex-host' \
    >>"$fixture_root/docs/zh/architecture.md"
  printf '%s\n' 'observe_current' \
    >>"$fixture_root/docs/en/lifecycle.md"
  printf '%s\n' 'observe_current' \
    >>"$fixture_root/docs/zh/lifecycle.md"
  for document_path in \
    README.md README-zh.md \
    docs/en/lifecycle.md docs/zh/lifecycle.md \
    docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
    printf '%s\n' provider_id host_id installation_key evidence_digest \
      >>"$fixture_root/$document_path"
  done
  for document_path in \
    docs/en/architecture.md docs/en/lifecycle.md docs/en/security.md \
    docs/zh/architecture.md docs/zh/lifecycle.md docs/zh/security.md; do
    printf '%s\n' \
      'OAW Core' \
      'Workflow Coordinator' \
      'Agent Host' \
      'CURRENT' \
      'SUBAGENT' \
      'logical workflow authority' \
      >>"$fixture_root/$document_path"
    case "$document_path" in
      docs/en/*)
        printf '%s\n' 'physical execution authority' >>"$fixture_root/$document_path"
        ;;
      docs/zh/*)
        printf '%s\n' 'Agent Host 拥有物理执行权限' >>"$fixture_root/$document_path"
        ;;
    esac
  done
  for document_path in docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
    printf '%s\n' \
      HOST_BINDING_EVIDENCE_REQUIRED \
      PROVIDER_BINDING_UNAVAILABLE \
      PROVIDER_FOREIGN_HOST_ONLY \
      PROVIDER_PIN_INCOMPATIBLE \
      HOST_PROVIDER_SCOPE_MISMATCH \
      HOST_BRIDGE_UNAVAILABLE \
      HOST_BRIDGE_CONTEXT_REQUIRED \
      HOST_BRIDGE_PROTOCOL_MISMATCH \
      HOST_EVIDENCE_HANDLE_REQUIRED \
      HOST_EVIDENCE_HANDLE_INVALID \
      HOST_EVIDENCE_EXPIRED \
      HOST_EVIDENCE_SESSION_MISMATCH \
      HOST_OBSERVATION_FAILED \
      HOST_OBSERVATION_PARTIAL \
      HOST_SESSION_CHANGED \
      PROVIDER_BINDING_CONTENT_MISMATCH \
      BINDING_EXPLICIT_INVOCATION_REQUIRED \
      HOST_FEATURE_UNATTESTED \
      HOST_ACTION_UNAVAILABLE \
      MACRO_INTERNAL_CONFLICT \
      PROFILE_TOPOLOGY_UNAVAILABLE \
      WORKFLOW_STATE_UNSUPPORTED \
      oaw.provider-descriptor/v1 \
      oaw.user-config/v1 \
      >>"$fixture_root/$document_path"
  done
  for document_path in \
    policy/ENGINEERING.md README.md README-zh.md \
    docs/en/lifecycle.md docs/zh/lifecycle.md; do
    printf '%s\n' \
      problem-framing \
      solution-specification \
      delivery-planning \
      workspace-preparation \
      implementation \
      implementation-tdd \
      incident-recovery \
      review-remediation \
      fresh-verification \
      closeout \
      MATT-FULL \
      SP-FULL \
      ECC-FULL \
      MATT-SP-HYBRID \
      USER-DEFINED \
      >>"$fixture_root/$document_path"
  done
  for document_path in docs/en/comparison.md docs/zh/comparison.md; do
    printf '%s\n' \
      84fdeffd12f2ee307994d1eb6feb48173b6e0502 \
      44c9b2d6e889982ac18c27d05a19fefe335194e1 \
      2d46e80e0925c7be0907f18c1812311ac212a6c5 \
      >>"$fixture_root/$document_path"
  done
  for document_path in docs/en/codex-bridge.md docs/zh/codex-bridge.md; do
    printf '%s\n' \
      oaw.provider-descriptor/v4 \
      oaw.profile-recipe/v3 \
      oaw.host-manifest/v3 \
      oaw.host-session/v3 \
      oaw.host-binding-inventory/v3 \
      oaw.host-environment-report/v2 \
      oaw.host-invocation-receipt/v3 \
      oaw.host-conformance-transcript/v4 \
      oaw.host-conformance-report/v4 \
      oaw.execution-graph/v4 \
      oaw.lifecycle-bundle/v4 \
      oaw.capability-grant/v3 \
      oaw.dispatch-packet/v2 \
      oaw.workflow-command/v2 \
      oaw.workflow-result/v2 \
      oaw.workflow-snapshot/v2 \
      oaw.workflow-revision/v2 \
      oaw.codex-bridge/v2 \
      oaw.codex-hook-context/v2 \
      oaw.host-evidence-handle/v2 \
      2.0.0 \
      'proof_scope: installation-integrity' \
      'live_protocol_proof: false' \
      >>"$fixture_root/$document_path"
  done
  printf '%s\n' \
    'Native Host is the default. It is not an OAW Request Mode.' \
    'Request Mode is evaluated only after explicit activation.' \
    'Assurance Level is orthogonal to Request Mode.' \
    policy-cooperative \
    core-backed \
    coordinator-backed \
    'Activated `BOUNDED` is not a generic Skill router.' \
    'The current `bounded_capability_defaults` interface does not define a matching predicate' \
    'Policy-only execution supports `CURRENT`. It cannot declare `SUBAGENT` eligible' \
    'Policy Workflow Plan' \
    'Progress Tracker' \
    CAPABILITY_SELECTION_REQUIRED \
    POLICY_ONLY_PROVIDER_UNVERIFIED \
    POLICY_ONLY_PROFILE_INCOMPLETE \
    POLICY_ONLY_TOPOLOGY_UNAVAILABLE \
    POLICY_ONLY_GUARANTEE_UNAVAILABLE \
    POLICY_ONLY_CONCURRENT_MUTATION \
    POLICY_ONLY_EXECUTION_UNCERTAIN \
    POLICY_ONLY_CONTEXT_UNCERTAIN \
    >>"$fixture_root/policy/ENGINEERING.md"
  printf '%s\n' \
    '## Explicit Activation' \
    'Native Host' \
    'OAW Engagement' \
    'Assurance Preflight' \
    policy-cooperative core-backed coordinator-backed \
    >>"$fixture_root/README.md"
  printf '%s\n' \
    '## 显式激活' \
    '原生 Host' \
    'OAW Engagement' \
    '保证等级预检' \
    policy-cooperative core-backed coordinator-backed \
    >>"$fixture_root/README-zh.md"
  printf '%s\n' 'explicit activation' 'Native Host' 'OAW Engagement' \
    >>"$fixture_root/docs/en/background.md"
  printf '%s\n' '显式激活' '原生 Host' 'OAW Engagement' \
    >>"$fixture_root/docs/zh/background.md"
  printf '%s\n' 'Assurance Preflight' 'Policy Workflow Plan' 'Progress Tracker' \
    policy-cooperative core-backed coordinator-backed \
    >>"$fixture_root/docs/en/lifecycle.md"
  printf '%s\n' '保证等级预检' '协作式 Policy Workflow Plan' 'Progress Tracker' \
    policy-cooperative core-backed coordinator-backed \
    >>"$fixture_root/docs/zh/lifecycle.md"
  printf '%s\n' 'Activation Router' 'Assurance Preflight' policy-cooperative \
    >>"$fixture_root/docs/en/architecture.md"
  printf '%s\n' 'Activation Router' '保证等级预检' policy-cooperative \
    >>"$fixture_root/docs/zh/architecture.md"
  printf '%s\n' 'Activation Router' 'Claude and Gemini do not emit `@`' \
    >>"$fixture_root/docs/en/adapters.md"
  printf '%s\n' 'Activation Router' 'Claude 和 Gemini 不会输出 `@`' \
    >>"$fixture_root/docs/zh/adapters.md"
  printf '%s\n' 'Activation Router' 'eager Policy import' \
    >>"$fixture_root/docs/en/extending-adapters.md"
  printf '%s\n' 'Activation Router' '急切导入 Policy' \
    >>"$fixture_root/docs/zh/extending-adapters.md"
  printf '%s\n' 'Installation does not activate OAW' 'Activation Router' \
    >>"$fixture_root/docs/en/installer.md"
  printf '%s\n' '安装不会激活 OAW' 'Activation Router' \
    >>"$fixture_root/docs/zh/installer.md"
  printf '%s\n' 'after explicit activation' 'Normal Host Skill routing' \
    >>"$fixture_root/docs/en/comparison.md"
  printf '%s\n' '显式激活后' '原生 Host Skill routing' \
    >>"$fixture_root/docs/zh/comparison.md"
  printf '%s\n' 'Bridge installation does not activate OAW' 'active OAW Engagement' \
    >>"$fixture_root/docs/en/codex-bridge.md"
  printf '%s\n' '安装 Bridge 不会激活 OAW' '活跃 OAW Engagement' \
    >>"$fixture_root/docs/zh/codex-bridge.md"
  for document_path in docs/en/security.md SECURITY.md; do
    printf '%s\n' 'current top-level user instruction' 'Policy Workflow Plan cannot grant' \
      >>"$fixture_root/$document_path"
  done
  for document_path in docs/zh/security.md SECURITY-zh.md; do
    printf '%s\n' '当前顶层用户指令' 'Policy Workflow Plan 不能授予' \
      >>"$fixture_root/$document_path"
  done
  printf '%s\n' 'Install State is not a Progress Tracker or Workflow State' \
    >>"$fixture_root/docs/en/troubleshooting.md"
  printf '%s\n' 'Install State 不是 Progress Tracker 或 Workflow State' \
    >>"$fixture_root/docs/zh/troubleshooting.md"
  for document_path in \
    docs/en/lifecycle.md docs/zh/lifecycle.md \
    docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
    printf '%s\n' \
      CAPABILITY_SELECTION_REQUIRED \
      POLICY_ONLY_PROVIDER_UNVERIFIED \
      POLICY_ONLY_PROFILE_INCOMPLETE \
      POLICY_ONLY_TOPOLOGY_UNAVAILABLE \
      POLICY_ONLY_GUARANTEE_UNAVAILABLE \
      POLICY_ONLY_CONCURRENT_MUTATION \
      POLICY_ONLY_EXECUTION_UNCERTAIN \
      POLICY_ONLY_CONTEXT_UNCERTAIN \
      >>"$fixture_root/$document_path"
  done
  for document_path in \
    policy/ENGINEERING.md docs/en/lifecycle.md docs/zh/lifecycle.md; do
    printf '%s\n' \
      workspace.prepare-or-confirm \
      verification.execute \
      closeout.execute \
      shared-understanding \
      specification-approved \
      delivery-plan-approved \
      workspace-ready \
      fresh-evidence \
      user-closeout \
      credit-only \
      dispatch-before \
      dispatch-after \
      MACRO_INTERNAL_CONFLICT \
      >>"$fixture_root/$document_path"
  done
  for document_path in \
    policy/ENGINEERING.md \
    docs/en/adapters.md docs/zh/adapters.md \
    docs/en/extending-adapters.md docs/zh/extending-adapters.md; do
    printf '%s\n' skill agent role instruction Hooks tools \
      >>"$fixture_root/$document_path"
  done
  printf '%s\n' 'Current Codex proves only `skill` bindings and `CURRENT` topology.' \
    >>"$fixture_root/README.md"
  printf '%s\n' '当前 Codex 只证明 `skill` binding 与 `CURRENT` topology。' \
    >>"$fixture_root/README-zh.md"
  printf '%s\n' 'Current Codex proves only `skill` bindings and `CURRENT` topology.' \
    >>"$fixture_root/docs/en/architecture.md"
  printf '%s\n' '当前 Codex 只证明 `skill` binding 与 `CURRENT` topology。' \
    >>"$fixture_root/docs/zh/architecture.md"
  mkdir -p "$fixture_root/internal/assets/providers"
  printf '%s\n' '{"id":"oaw/matt"}' \
    >"$fixture_root/internal/assets/providers/oaw-matt.json"
  : >"$fixture_root/CHANGELOG.md"
  printf '%s\n' \
    'OAW is now explicitly activated per deliverable' \
    'lazy Activation Router' \
    'policy-only Markdown lifecycle locks are not converted' \
    >>"$fixture_root/CHANGELOG.md"
  mkdir -p "$fixture_root/docs/adr" "$fixture_root/docs/superpowers/specs"
  printf '%s\n' 'Superseded by ADR 0009' \
    >"$fixture_root/docs/adr/0003-add-optional-capability-admission-runtime.md"
  printf '%s\n' 'Superseded by ADR 0009; Host-native control is retained' \
    >"$fixture_root/docs/adr/0007-use-host-native-execution-topologies.md"
  printf '%s\n' 'Superseded by ADR 0009; Host-owned environment semantics are retained' \
    >"$fixture_root/docs/adr/0008-treat-subagent-environment-as-host-owned.md"
  printf '%s\n' 'Accepted' \
    >"$fixture_root/docs/adr/0009-separate-core-coordination-and-host-execution.md"
  printf '%s\n' 'Superseded by the 2026-08-05 OAW Core and Workflow Coordinator hard-cutover design' \
    >"$fixture_root/docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md"
  printf '%s\n' 'Approved for implementation' \
    >"$fixture_root/docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md"
  printf '%s\n' 'experience-based' >>"$fixture_root/docs/en/comparison.md"
  printf '%s\n' '基于经验' >>"$fixture_root/docs/zh/comparison.md"
}

for governance_file in \
  LICENSE \
  CONTRIBUTING.md \
  CONTRIBUTING-zh.md \
  SECURITY.md \
  SECURITY-zh.md \
  CODE_OF_CONDUCT.md \
  CHANGELOG.md \
  scripts/check-docs.sh; do
  assert_file "$governance_file"
done
for readme_file in README.md README-zh.md; do
  assert_file "$readme_file"
done

assert_contains LICENSE "Apache License"
assert_contains LICENSE "Version 2.0, January 2004"
assert_contains LICENSE "Copyright 2026 Open Agent Workflow contributors"
for section in 1 2 3 4 5 6 7 8 9; do
  grep -E "^[[:space:]]+$section\\." "$REPOSITORY/LICENSE" >/dev/null ||
    fail "LICENSE is missing Apache-2.0 section $section"
done

for contribution_file in CONTRIBUTING.md CONTRIBUTING-zh.md; do
  assert_contains "$contribution_file" "Bash 3.2"
  assert_contains "$contribution_file" "black-box"
  assert_contains "$contribution_file" "tests/10-docs-test.sh"
  assert_contains "$contribution_file" "adapter evidence"
  assert_contains "$contribution_file" "retrieval date"
  assert_contains "$contribution_file" "provider"
  assert_contains "$contribution_file" "English/Chinese"
  assert_contains "$contribution_file" "remote publication"
done
pass "bilingual contribution contracts define the delivery and adapter evidence seam"
for contribution_contract in \
  'public Go `oaw` binary' \
  'precompiled sibling binary' \
    'Install State and Workflow State' \
  'Available native and Docker smoke tests must pass'; do
  assert_contains CONTRIBUTING.md "$contribution_contract"
done
for contribution_contract in \
  '公开 Go `oaw` 二进制' \
  '预编译的同目录二进制' \
    'Install State 与 Workflow State' \
  '可用的原生和 Docker smoke test 必须通过'; do
  assert_contains CONTRIBUTING-zh.md "$contribution_contract"
done
pass "bilingual contribution contracts distinguish Go authority, wrapper compatibility, state, and environment-aware release gates"

for security_file in SECURITY.md SECURITY-zh.md; do
  assert_contains "$security_file" "private"
  assert_contains "$security_file" "supported version"
  assert_contains "$security_file" "no guaranteed response SLA"
  assert_contains "$security_file" "installer trust boundary"
  assert_contains "$security_file" "HOME"
  assert_contains "$security_file" "XDG_CONFIG_HOME"
  assert_contains "$security_file" "XDG_STATE_HOME"
  if grep -E '[[:alnum:]._%+-]+@[[:alnum:].-]+\.[[:alpha:]]{2,}' \
    "$REPOSITORY/$security_file" >/dev/null; then
    fail "$security_file invents a live security email address"
  fi
done
pass "bilingual security policies use a private reporting contract without a fake address"
for security_contract in \
  'public Go binary' \
  'precompiled binaries' \
  'runtime executable download' \
  'Install State and Workflow State'; do
  assert_contains SECURITY.md "$security_contract"
done
for security_contract in \
  '公开 Go binary' \
  '预编译二进制' \
  '运行时不会下载可执行文件' \
  'Install State 与 Workflow State'; do
  assert_contains SECURITY-zh.md "$security_contract"
done
pass "bilingual security policies publish binary, state, and Core/Coordinator/Host trust boundaries"

for security_boundary_document in \
  SECURITY.md SECURITY-zh.md \
  docs/en/security.md docs/zh/security.md; do
  for security_boundary in \
    'logical workflow authority' \
    'Host sandbox and approvals' \
    'secret-free' \
    'opaque digest' \
    'cooperating clients' \
    'OAW never starts a model CLI'; do
    assert_contains "$security_boundary_document" "$security_boundary"
  done
  for forbidden_authority_claim in \
    'Grant contains Host tools' \
    'guarantees MCP inheritance' \
    'guarantees Hook inheritance' \
    'guarantees Skill inheritance' \
    'guarantees Plugin inheritance'; do
    assert_not_contains "$security_boundary_document" "$forbidden_authority_claim"
  done
done
pass "security documents separate logical workflow authority from Host physical authority"

assert_contains CODE_OF_CONDUCT.md "Contributor Covenant"
assert_contains CODE_OF_CONDUCT.md "version 2.1"
assert_contains CODE_OF_CONDUCT.md "maintainer-designated private channel"
if grep -F "[INSERT CONTACT METHOD]" "$REPOSITORY/CODE_OF_CONDUCT.md" >/dev/null; then
  fail "CODE_OF_CONDUCT.md retains the Contributor Covenant contact placeholder"
fi
pass "code of conduct records Contributor Covenant 2.1 and neutral enforcement"

assert_contains CHANGELOG.md "## [Unreleased]"
assert_contains CHANGELOG.md "### 0.1.0"
assert_contains CHANGELOG.md "local candidate"
assert_contains CHANGELOG.md "not published"
assert_contains CHANGELOG.md "canonical policy"
assert_contains CHANGELOG.md "forced"
assert_contains CHANGELOG.md "bilingual documentation"
assert_contains CHANGELOG.md "Go-authoritative management CLI"
assert_contains CHANGELOG.md "precompiled Darwin, Linux, and Windows archives"
assert_contains CHANGELOG.md "Docker or optional WSL execution"
assert_contains CHANGELOG.md "Install State and revisioned Workflow State remain disjoint"
assert_contains CHANGELOG.md "OAW Core"
assert_contains CHANGELOG.md "optional Workflow Coordinator"
assert_contains CHANGELOG.md '`CURRENT`/`SUBAGENT`'
assert_contains CHANGELOG.md "Host session reports"
assert_contains CHANGELOG.md "Provider descriptor v3"
assert_contains CHANGELOG.md "Profile Recipe v2"
assert_contains CHANGELOG.md "user configuration v3"
assert_contains CHANGELOG.md "Host integration v2"
assert_contains CHANGELOG.md "Capability Grant v2"
assert_contains CHANGELOG.md "Workflow State v1"
assert_contains CHANGELOG.md "oaw run --host codex"
assert_contains CHANGELOG.md "oaw runtime exchange"
assert_contains CHANGELOG.md "Codex Runner"
assert_contains CHANGELOG.md "private HOME/Skill staging"
assert_contains CHANGELOG.md "Core and Coordinator state is secret-free"
pass "changelog describes the local unreleased 0.1.0 candidate"

assert_executable scripts/check-docs.sh
assert_contains scripts/check-docs.sh "README.md|README-zh.md"
assert_contains scripts/check-docs.sh "docs/en/background.md|docs/zh/background.md"
assert_contains scripts/check-docs.sh "docs/en/extending-adapters.md|docs/zh/extending-adapters.md"
assert_contains scripts/check-docs.sh "docs/en/codex-bridge.md|docs/zh/codex-bridge.md"
assert_contains scripts/check-docs.sh "for command in check install update uninstall"
assert_contains scripts/check-docs.sh "experience-based"
assert_contains scripts/check-docs.sh "基于经验"
assert_contains scripts/check-docs.sh "Public installation management is Go-authoritative."
assert_contains scripts/check-docs.sh "公开安装管理以 Go 为权威实现。"
assert_contains scripts/check-docs.sh "Bash remains authoritative"
assert_contains scripts/check-docs.sh "Bash 仍是权威"
assert_contains scripts/check-docs.sh "host-scope-documents"
assert_contains scripts/check-docs.sh "Host Installation"
assert_contains scripts/check-docs.sh "Verified Provider Instance"
assert_contains scripts/check-docs.sh "PROVIDER_FOREIGN_HOST_ONLY"
assert_contains scripts/check-docs.sh "HOST_BRIDGE_PROTOCOL_MISMATCH"
assert_contains scripts/check-docs.sh "forbidden positive authority claim"
assert_contains scripts/check-docs.sh "provider-surface-matrix-documents"
assert_contains scripts/check-docs.sh "oaw.provider-descriptor/v4"
assert_contains scripts/check-docs.sh "oaw.codex-bridge/v2"
assert_contains scripts/check-docs.sh "84fdeffd12f2ee307994d1eb6feb48173b6e0502"
assert_contains scripts/check-docs.sh "44c9b2d6e889982ac18c27d05a19fefe335194e1"
assert_contains scripts/check-docs.sh "2d46e80e0925c7be0907f18c1812311ac212a6c5"
assert_contains scripts/check-docs.sh "forbidden provider claim"
if grep -E '(^|[;&|[:space:]])(curl|wget)([[:space:]]|$)' \
  "$REPOSITORY/scripts/check-docs.sh" >/dev/null; then
  fail "documentation checker contains a network client command"
fi
pass "documentation checker declares bilingual, CLI, caveat, and offline contracts"

DOCS_TEST_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-docs-test.XXXXXX") ||
  fail "cannot create documentation test fixture"
make_checker_fixture "$DOCS_TEST_TEMP/repository"
: >"$DOCS_TEST_TEMP/repository/docs/en/target.md"
printf '%s\n' \
  '[inline](target.md "inline title")' \
  '[reference][target]' \
  '[target]: target.md "reference title"' \
  >"$DOCS_TEST_TEMP/repository/docs/en/link-fixture.md"
if ! checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  fail "documentation checker rejects valid links with titles: $checker_output"
fi
pass "documentation checker accepts inline and reference links with titles"

printf '%s\n' '[missing]: no-such.md' \
  >"$DOCS_TEST_TEMP/repository/docs/en/link-fixture.md"
if checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  fail "documentation checker accepts a broken reference-style local link"
fi
case "$checker_output" in
  *'missing Markdown link target'*'no-such.md'*) ;;
  *) fail "documentation checker gives no broken reference-link diagnostic" ;;
esac
pass "documentation checker rejects broken reference-style local links"

printf '%s\n' '[missing reference][not-defined]' \
  >"$DOCS_TEST_TEMP/repository/docs/en/link-fixture.md"
if checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  fail "documentation checker accepts a missing reference definition"
fi
case "$checker_output" in
  *'missing Markdown reference definition'*'not-defined'*) ;;
  *) fail "documentation checker gives no missing reference-definition diagnostic" ;;
esac
pass "documentation checker rejects missing reference definitions"

: >"$DOCS_TEST_TEMP/repository/docs/en/link-fixture.md"

FORBIDDEN_EXECUTION_LITERAL='Runtime Plane'
CURRENT_FIXTURE="$DOCS_TEST_TEMP/repository/docs/en/architecture.md"
CURRENT_FIXTURE_BACKUP="$DOCS_TEST_TEMP/architecture.md.before-forbidden"
cp "$CURRENT_FIXTURE" "$CURRENT_FIXTURE_BACKUP"
printf '%s\n' "$FORBIDDEN_EXECUTION_LITERAL" >>"$CURRENT_FIXTURE"
if checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  fail "documentation checker accepts a forbidden literal in a current source"
fi
printf '%s\n' "$checker_output" |
  grep -E "docs/en/architecture\\.md:[0-9]+:Runtime Plane" >/dev/null ||
  fail "documentation checker omits file, line, and literal for a current-source violation: $checker_output"
cp "$CURRENT_FIXTURE_BACKUP" "$CURRENT_FIXTURE"

HISTORICAL_FIXTURE="$DOCS_TEST_TEMP/repository/docs/adr/0003-add-optional-capability-admission-runtime.md"
HISTORICAL_FIXTURE_BACKUP="$DOCS_TEST_TEMP/0003.before-forbidden"
cp "$HISTORICAL_FIXTURE" "$HISTORICAL_FIXTURE_BACKUP"
printf '%s\n' "$FORBIDDEN_EXECUTION_LITERAL" >>"$HISTORICAL_FIXTURE"
if checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  checker_status=0
else
  checker_status=$?
fi
[ "$checker_status" -eq 0 ] ||
  fail "documentation checker rejects a forbidden literal in a historical ADR: $checker_output"
cp "$HISTORICAL_FIXTURE_BACKUP" "$HISTORICAL_FIXTURE"
pass "documentation checker rejects stale current claims while allowing historical ADR text"

AUTHORITY_FIXTURE="$DOCS_TEST_TEMP/repository/docs/en/architecture.md"
AUTHORITY_FIXTURE_BACKUP="$DOCS_TEST_TEMP/architecture.md.before-authority"
cp "$AUTHORITY_FIXTURE" "$AUTHORITY_FIXTURE_BACKUP"
printf '%s\n' 'OAW starts a model process' >>"$AUTHORITY_FIXTURE"
if checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  fail "documentation checker accepts a positive OAW execution-authority claim"
fi
case "$checker_output" in
  *'forbidden positive authority claim'*'OAW starts a model process'*) ;;
  *) fail "documentation checker gives no positive authority diagnostic" ;;
esac
cp "$AUTHORITY_FIXTURE_BACKUP" "$AUTHORITY_FIXTURE"
printf '%s\n' 'OAW never starts a model process.' >>"$AUTHORITY_FIXTURE"
if ! checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  fail "documentation checker rejects a valid negative authority statement: $checker_output"
fi
cp "$AUTHORITY_FIXTURE_BACKUP" "$AUTHORITY_FIXTURE"
pass "documentation checker rejects positive execution claims and permits negative boundaries"

PROVIDER_CLAIM_FIXTURE="$DOCS_TEST_TEMP/repository/docs/en/lifecycle.md"
PROVIDER_CLAIM_FIXTURE_BACKUP="$DOCS_TEST_TEMP/lifecycle.md.before-provider-claim"
cp "$PROVIDER_CLAIM_FIXTURE" "$PROVIDER_CLAIM_FIXTURE_BACKUP"
printf '%s\n' 'ECC `e2e-runner` owns broad final verification' \
  >>"$PROVIDER_CLAIM_FIXTURE"
if checker_output=$(bash "$DOCS_TEST_TEMP/repository/scripts/check-docs.sh" 2>&1); then
  fail "documentation checker accepts a fictional Provider ownership claim"
fi
case "$checker_output" in
  *'forbidden provider claim'*'ECC `e2e-runner` owns broad final verification'*) ;;
  *) fail "documentation checker gives no fictional Provider-claim diagnostic" ;;
esac
cp "$PROVIDER_CLAIM_FIXTURE_BACKUP" "$PROVIDER_CLAIM_FIXTURE"
pass "documentation checker rejects fictional Provider ownership claims"

for readme_file in README.md README-zh.md; do
  for command_example in \
    './install.sh check' \
    './install.sh install' \
    './install.sh install --project /path/to/repository' \
    './install.sh update --dry-run' \
    './install.sh uninstall'; do
    assert_contains "$readme_file" "$command_example"
  done
  for profile_id in \
    SP-FULL \
    MATT-FULL \
    ECC-FULL \
    MATT-SP-HYBRID \
    USER-DEFINED; do
    assert_contains "$readme_file" "$profile_id"
  done
  assert_not_contains "$readme_file" 'CUSTOM-LOCKED'
  for target_id in \
    claude \
    codex \
    gemini \
    opencode \
    cursor \
    windsurf \
    cline \
    roo \
    copilot; do
    assert_contains "$readme_file" "\`$target_id\`"
  done
  for score_row in \
    '4.8 / 5.0 / 3.8' \
    '5.0 / 4.2 / 3.7' \
    '4.8 / 4.9 / 4.1' \
    '4.7 / 5.0 / 2.8' \
    '5.0 / 4.8 / 4.4' \
    '5.0 / 3.6 / 4.0'; do
    assert_contains "$readme_file" "$score_row"
  done
  for detail_document in \
    background \
    comparison \
    lifecycle \
    architecture \
    installer \
    codex-bridge \
    adapters \
    extending-adapters \
    security \
    troubleshooting; do
    assert_contains "$readme_file" "docs/en/$detail_document.md"
    assert_contains "$readme_file" "docs/zh/$detail_document.md"
  done
done
pass "both README entrypoints preserve commands, profiles, targets, scores, and document pairs"

for release_boundary in \
  'Public installation management is Go-authoritative.' \
  '`install.sh` is an offline sibling-binary compatibility wrapper.' \
  'Release archives contain precompiled binaries and perform no runtime executable download.' \
  'Installation management distributes the canonical Policy and target-native instruction entrypoints; it does not execute engineering work.' \
  'OAW Core is required and stateless. The Workflow Coordinator is optional and stores only Workflow State for `WORKFLOW`; Install State and Workflow State are disjoint, with no migration or implicit adoption.' \
  'The Agent Host owns Agents, model calls, MCP, Hooks, Skills, Plugins, authentication, tools, sandbox, approvals, and every physical effect. OAW never starts a model process.' \
  'Codex has a policy integration by default and a separate audited host-native Bridge' \
  'Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness.'; do
  assert_contains README.md "$release_boundary"
done
for release_boundary in \
  '公开安装管理以 Go 为权威实现。' \
  '`install.sh` 是离线的同目录二进制兼容包装器。' \
  '发布归档包含预编译二进制，运行时不会下载可执行文件。' \
  '安装管理只分发 canonical Policy 和 target-native 指令入口，不执行工程工作。' \
  'OAW Core 是必需且无状态的。Workflow Coordinator 是可选的，只为 `WORKFLOW` 保存' \
  'Agent Host 拥有 Agent、model call、MCP、Hook、Skill、Plugin、认证、工具、sandbox、' \
  'Codex 默认提供 policy integration，并另有独立且经过审计的 host-native Bridge' \
  '可用的原生和 Docker smoke test 必须通过；不可用的平台检查返回 77，且不阻塞 release readiness。'; do
  assert_contains README-zh.md "$release_boundary"
done
pass "bilingual README entrypoints publish Core, Coordinator, and Host boundaries"

for current_document in \
  README.md README-zh.md CHANGELOG.md CONTRIBUTING.md CONTRIBUTING-zh.md \
  SECURITY.md SECURITY-zh.md docs/en/installer.md docs/zh/installer.md \
  docs/en/architecture.md docs/zh/architecture.md \
  docs/en/troubleshooting.md docs/zh/troubleshooting.md \
  docs/en/extending-adapters.md docs/zh/extending-adapters.md; do
  assert_not_contains "$current_document" 'Bash remains authoritative'
  assert_not_contains "$current_document" 'Bash 仍是权威'
  assert_not_contains "$current_document" 'public `oaw install` is not enabled'
  assert_not_contains "$current_document" 'public `oaw install` 尚未启用'
  assert_not_contains "$current_document" 'zero-dependency Bash installer'
  assert_not_contains "$current_document" 'actual WSL smoke pass is required before publishing a release'
  assert_not_contains "$current_document" 'release readiness remains blocked until'
  assert_not_contains "$current_document" '发布前必须在真实 WSL 环境通过 smoke test'
  assert_not_contains "$current_document" '仍不具备 release readiness'
done
pass "current user-facing documents reject stale pre-cutover authority claims"

for english_heading in \
  '## Why OAW' \
  '## Problems It Solves' \
  '## Capabilities' \
  '## Quick Start' \
  '## Explicit Activation' \
  '## Lifecycle Profiles' \
  '## Matt-Superpowers Hybrid' \
  '## Supported Targets' \
  '## Safety Model' \
  '## Update and Uninstall' \
  '## Provider Prerequisites' \
  '## Documentation' \
  '## Contributing' \
  '## License' \
  '## Project Status'; do
  assert_contains README.md "$english_heading"
done
assert_contains README.md "[简体中文](README-zh.md)"
assert_contains README.md "arbitrates independently installed workflow providers across agent tools"
assert_contains README.md "There is no timeout or silent default."
assert_contains README.md "OAW does not install Superpowers, Matt Pocock skills, or ECC."
assert_contains README.md "Updates use the Policy, Version, registry, and rendering behavior embedded"
assert_contains README.md 'Rebuild `./oaw` after changing a source checkout'
assert_contains README.md "Drift fails closed before mutation."
assert_contains README.md '`--force` backs up every affected artifact before mutation.'
assert_contains README.md "experience-based design inputs"
assert_contains README.md "Machine-readable management status"
assert_contains README.md "v0.1 management output is human-readable"
pass "English README covers the complete entrypoint and safety contract"

for chinese_heading in \
  '## 为什么需要 OAW' \
  '## 解决的问题' \
  '## 核心能力' \
  '## 快速开始' \
  '## 显式激活' \
  '## 生命周期配置' \
  '## Matt-Superpowers 混合配置' \
  '## 支持的目标' \
  '## 安全模型' \
  '## 更新与卸载' \
  '## Provider 前置条件' \
  '## 文档' \
  '## 贡献' \
  '## 许可证' \
  '## 项目状态'; do
  assert_contains README-zh.md "$chinese_heading"
done
assert_contains README-zh.md "[English](README.md)"
assert_contains README-zh.md "协调多个 agent 工具中独立安装的 workflow provider"
assert_contains README-zh.md "没有超时自动选择，也没有静默默认项。"
assert_contains README-zh.md "OAW 不安装 Superpowers、Matt Pocock skills 或 ECC。"
assert_contains README-zh.md "更新使用运行中 binary 嵌入的 Policy、Version、registry 与 rendering behavior。"
assert_contains README-zh.md 'checkout 后必须重新构建 `./oaw`'
assert_contains README-zh.md "检测到 drift 时，会在变更前关闭失败。"
assert_contains README-zh.md '`--force` 会在变更前先备份所有受影响构件。'
assert_contains README-zh.md "基于经验的设计输入"
assert_contains README-zh.md "machine-readable management status 保留为 post-v0.1 扩展"
assert_contains README-zh.md "v0.1 management 只输出"
assert_contains README-zh.md "human-readable 状态"
pass "Chinese README covers the equivalent entrypoint and safety contract"

for rationale_document in \
  docs/en/background.md \
  docs/zh/background.md \
  docs/en/comparison.md \
  docs/zh/comparison.md \
  docs/en/lifecycle.md \
  docs/zh/lifecycle.md; do
  assert_file "$rationale_document"
done

for background_contract in \
  'overlapping automatic triggers' \
  'cross-client drift' \
  'providers remain independently installed' \
  'does not install, update, or remove providers'; do
  assert_contains docs/en/background.md "$background_contract"
done
assert_contains docs/en/background.md '[简体中文](../zh/background.md)'
for background_contract in \
  '重叠的自动触发' \
  '跨客户端 drift' \
  'provider 保持独立安装' \
  '不安装、更新或删除 provider'; do
  assert_contains docs/zh/background.md "$background_contract"
done
assert_contains docs/zh/background.md '[English](../en/background.md)'
pass "background documents explain trigger conflicts, client drift, and provider independence"

for comparison_file in docs/en/comparison.md docs/zh/comparison.md; do
  for comparison_score in \
    '4.8 | 5.0 | 3.8' \
    '5.0 | 4.2 | 3.7' \
    '4.8 | 4.9 | 4.1' \
    '4.7 | 5.0 | 2.8' \
    '5.0 | 4.8 | 4.4' \
    '5.0 | 3.6 | 4.0'; do
    assert_contains "$comparison_file" "$comparison_score"
  done
done
for comparison_criterion in \
  'procedure completeness' \
  'correctness discipline' \
  'ambiguity handling' \
  'review closure' \
  'verification strength' \
  'operational overhead'; do
  assert_contains docs/en/comparison.md "$comparison_criterion"
done
assert_contains docs/en/comparison.md 'experience-based judgments'
assert_contains docs/en/comparison.md 'version-sensitive'
assert_contains docs/en/comparison.md 'not a universal benchmark'
assert_contains docs/en/comparison.md '| Planning | 4.8 | 5.0 | 3.8 | Matt for complex work |'
assert_contains docs/en/comparison.md '| Implementation | 5.0 | 4.2 | 3.7 | Superpowers |'
assert_contains docs/en/comparison.md '| TDD | 4.8 | 4.9 | 4.1 | Matt |'
assert_contains docs/en/comparison.md '| Debugging | 4.7 | 5.0 | 2.8 | Matt |'
assert_contains docs/en/comparison.md '| Review | 5.0 | 4.8 | 4.4 | Superpowers |'
assert_contains docs/en/comparison.md '| Completion | 5.0 | 3.6 | 4.0 | Superpowers |'
assert_contains docs/en/comparison.md '[简体中文](../zh/comparison.md)'

for comparison_criterion in \
  '流程完整性' \
  '正确性纪律' \
  '歧义处理' \
  '复核闭环' \
  '验证强度' \
  '运维开销'; do
  assert_contains docs/zh/comparison.md "$comparison_criterion"
done
assert_contains docs/zh/comparison.md '基于经验的判断'
assert_contains docs/zh/comparison.md '会随版本变化'
assert_contains docs/zh/comparison.md '不是通用 benchmark'
assert_contains docs/zh/comparison.md '| 规划 | 4.8 | 5.0 | 3.8 | 复杂任务由 Matt 负责 |'
assert_contains docs/zh/comparison.md '| 实现 | 5.0 | 4.2 | 3.7 | Superpowers |'
assert_contains docs/zh/comparison.md '| TDD | 4.8 | 4.9 | 4.1 | Matt |'
assert_contains docs/zh/comparison.md '| 调试 | 4.7 | 5.0 | 2.8 | Matt |'
assert_contains docs/zh/comparison.md '| 复核 | 5.0 | 4.8 | 4.4 | Superpowers |'
assert_contains docs/zh/comparison.md '| 完成 | 5.0 | 3.6 | 4.0 | Superpowers |'
assert_contains docs/zh/comparison.md '[English](../en/comparison.md)'
pass "comparison documents preserve approved scores, criteria, caveats, and stage owners"

for lifecycle_file in docs/en/lifecycle.md docs/zh/lifecycle.md; do
  for profile_id in \
    SP-FULL \
    MATT-FULL \
    ECC-FULL \
    MATT-SP-HYBRID \
    USER-DEFINED; do
    assert_contains "$lifecycle_file" "$profile_id"
  done
  assert_contains "$lifecycle_file" 'oaw/ecc-engineering'
  assert_contains "$lifecycle_file" 'MATT-SP-HYBRID + ECC(security-review)'
  assert_not_contains "$lifecycle_file" 'CUSTOM-LOCKED'
done
for lifecycle_contract in \
  'Direct Mode' \
  'Bounded Mode' \
  'Workflow Mode' \
  'Only Workflow Mode runs the Startup Gate' \
  'Provider and Capability' \
  'third-party Providers' \
  'blocking user choice' \
  'lifecycle lock' \
  'bundle inheritance' \
  'bounded add-ons' \
  'stable switching' \
  'ticket inheritance' \
  'stable-boundary switch' \
  'policy/ENGINEERING.md is normative'; do
  assert_contains docs/en/lifecycle.md "$lifecycle_contract"
done
assert_contains docs/en/lifecycle.md '[简体中文](../zh/lifecycle.md)'
for lifecycle_contract in \
  'Direct Mode' \
  'Bounded Mode' \
  'Workflow Mode' \
  '只有 Workflow Mode 运行 Startup Gate' \
  'Provider 与 Capability' \
  '第三方 Provider' \
  '阻塞式用户选择' \
  '生命周期锁' \
  'bundle 继承' \
  'bounded add-on' \
  '稳定切换' \
  'ticket 继承' \
  'stable boundary 切换' \
  'policy/ENGINEERING.md 是规范来源'; do
  assert_contains docs/zh/lifecycle.md "$lifecycle_contract"
done
assert_contains docs/zh/lifecycle.md '[English](../en/lifecycle.md)'
pass "lifecycle documents explain Request Modes, extensible Profiles, topology, locking, add-ons, and switching"

for matrix_document in \
  policy/ENGINEERING.md README.md README-zh.md \
  docs/en/lifecycle.md docs/zh/lifecycle.md; do
  for canonical_slot in \
    problem-framing solution-specification delivery-planning \
    workspace-preparation implementation implementation-tdd \
    incident-recovery review-remediation fresh-verification closeout; do
    assert_contains "$matrix_document" "$canonical_slot"
  done
done
for comparison_document in docs/en/comparison.md docs/zh/comparison.md; do
  for upstream_revision in \
    84fdeffd12f2ee307994d1eb6feb48173b6e0502 \
    44c9b2d6e889982ac18c27d05a19fefe335194e1 \
    2d46e80e0925c7be0907f18c1812311ac212a6c5; do
    assert_contains "$comparison_document" "$upstream_revision"
  done
done
assert_not_contains internal/assets/providers/oaw-matt.json '"reference":"requirements"'
assert_not_contains internal/assets/providers/oaw-matt.json '"reference":"verification-loop"'
assert_contains README.md 'Current Codex proves only `skill` bindings and `CURRENT` topology.'
assert_contains README-zh.md '当前 Codex 只证明 `skill` binding 与 `CURRENT` topology。'
pass "provider surface v4 docs pin real sources, canonical slots, and conservative Codex facts"

for boundary_document in \
  docs/en/architecture.md \
  docs/zh/architecture.md \
  docs/en/lifecycle.md \
  docs/zh/lifecycle.md \
  docs/en/security.md \
  docs/zh/security.md; do
  for boundary_contract in \
    'OAW Core' \
    'Workflow Coordinator' \
    'Agent Host' \
    'CURRENT' \
    'SUBAGENT'; do
    assert_contains "$boundary_document" "$boundary_contract"
  done
  assert_contains "$boundary_document" 'logical workflow authority'
done
for lifecycle_boundary_document in \
  docs/en/architecture.md docs/zh/architecture.md \
  docs/en/lifecycle.md docs/zh/lifecycle.md; do
  for lifecycle_boundary_contract in \
    'policy' \
    'host-native' \
    'Workflow State' \
    'SP-FULL' \
    'MATT-FULL' \
    'ECC-FULL' \
    'MATT-SP-HYBRID' \
    'USER-DEFINED'; do
    assert_contains "$lifecycle_boundary_document" "$lifecycle_boundary_contract"
  done
done
for english_boundary_document in \
  docs/en/architecture.md docs/en/lifecycle.md docs/en/security.md; do
  assert_contains "$english_boundary_document" 'physical execution authority'
done
for chinese_boundary_document in \
  docs/zh/architecture.md docs/zh/lifecycle.md docs/zh/security.md; do
  assert_contains "$chinese_boundary_document" 'Agent Host 拥有物理执行权限'
done
assert_contains docs/en/architecture.md 'Top-level user request'
assert_contains docs/en/architecture.md '-> Activation Router'
assert_contains docs/en/architecture.md '-> Native Host'
assert_contains docs/en/architecture.md '-> coordinator-backed -> OAW Core -> Workflow Coordinator -> Agent Host'
assert_contains docs/zh/architecture.md '顶层用户请求'
assert_contains docs/zh/architecture.md '-> Activation Router'
assert_contains docs/zh/architecture.md '-> 原生 Host'
assert_contains docs/zh/architecture.md '-> coordinator-backed -> OAW Core -> Workflow Coordinator -> Agent Host'
pass "bilingual architecture and lifecycle documents define Core, Coordinator, Host, topology, and Profile boundaries"

for policy_contract in \
  'DIRECT' \
  'BOUNDED' \
  'WORKFLOW' \
  'Only Workflow Mode runs the Startup Gate' \
  'oaw/superpowers' \
  'oaw/matt' \
  'oaw/ecc' \
  'third-party Providers' \
  'oaw/ecc-engineering' \
  'USER-DEFINED' \
  'OAW Core' \
  'Workflow Coordinator' \
  'Agent Host' \
  'CURRENT' \
  'SUBAGENT' \
  'Resource Leases' \
  'physical execution authority'; do
  assert_contains policy/ENGINEERING.md "$policy_contract"
done
assert_not_contains policy/ENGINEERING.md 'CUSTOM-LOCKED'
for activation_policy_contract in \
  'Native Host is the default. It is not an OAW Request Mode.' \
  'Request Mode is evaluated only after explicit activation.' \
  'Assurance Level is orthogonal to Request Mode.' \
  policy-cooperative \
  core-backed \
  coordinator-backed \
  'Activated `BOUNDED` is not a generic Skill router.' \
  'The current `bounded_capability_defaults` interface does not define a matching predicate' \
  'Policy-only execution supports `CURRENT`. It cannot declare `SUBAGENT` eligible' \
  'Policy Workflow Plan' \
  'Progress Tracker' \
  CAPABILITY_SELECTION_REQUIRED \
  POLICY_ONLY_PROVIDER_UNVERIFIED \
  POLICY_ONLY_PROFILE_INCOMPLETE \
  POLICY_ONLY_TOPOLOGY_UNAVAILABLE \
  POLICY_ONLY_GUARANTEE_UNAVAILABLE \
  POLICY_ONLY_CONCURRENT_MUTATION \
  POLICY_ONLY_EXECUTION_UNCERTAIN \
  POLICY_ONLY_CONTEXT_UNCERTAIN; do
  assert_contains policy/ENGINEERING.md "$activation_policy_contract"
done
for stale_activation_policy_contract in \
  'Classify every new top-level engineering request as exactly one Request Mode:' \
  'In policy-only use, the caller receives the same Core-produced Bundle' \
  'Policy-only Hosts may coordinate the same ownership model with a local lock'; do
  assert_not_contains policy/ENGINEERING.md "$stale_activation_policy_contract"
done
assert_not_contains docs/en/comparison.md 'CUSTOM-LOCKED'
assert_not_contains docs/zh/comparison.md 'CUSTOM-LOCKED'
pass "canonical policy distinguishes Request Modes, extensible Providers, user-defined Profiles, and Policy-only Host limits"

for operating_document in \
  docs/en/architecture.md \
  docs/zh/architecture.md \
  docs/en/installer.md \
  docs/zh/installer.md \
  docs/en/adapters.md \
  docs/zh/adapters.md; do
  assert_file "$operating_document"
done

for architecture_file in docs/en/architecture.md docs/zh/architecture.md; do
  for architecture_contract in \
    '${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/ENGINEERING.md' \
    '${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/user.state' \
    '${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/projects/<crc>-<bytes>.state' \
    '${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/backups' \
    'embedded checkout policy -> pure renderer -> preflight/prepare -> required backup -> apply -> Install State/targets' \
    'pure renderer' \
    'prepare phase' \
    'apply phase' \
    'atomic replacement per destination' \
    'managed-block' \
    'owned-file' \
    '`format`' \
    '`version`' \
    '`scope`' \
    '`project`' \
    '`policy`' \
    '`backup`' \
    '`directory`' \
    '`target`' \
    'operation-scoped backup'; do
    assert_contains "$architecture_file" "$architecture_contract"
  done
done
assert_contains docs/en/architecture.md 'Marker comments do not establish model precedence'
assert_contains docs/zh/architecture.md 'marker 注释不建立模型优先级'
for workflow_boundary in \
  '${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows' \
  'Install State and Workflow State use disjoint namespaces' \
  'Project Workflow documents are one-way, non-authoritative projections' \
  'Capability Grant' \
  'Resource Lease' \
  'The Agent Host owns physical execution authority' \
  'mutation journal' \
  'automatic recovery from a process or machine crash'; do
  assert_contains docs/en/architecture.md "$workflow_boundary"
done
for workflow_boundary in \
  '${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows' \
  'Install State 与 Workflow State 使用相互独立的 namespace' \
  '项目 Workflow 文档是 committed Workflow State 的单向、非权威 projection' \
  'Capability Grant' \
  'Resource Lease' \
  'Agent Host 拥有物理执行权限' \
  'mutation journal' \
  'process 或 machine crash'; do
  assert_contains docs/zh/architecture.md "$workflow_boundary"
done
pass "architecture documents match management, Core, Coordinator, Host, rendering, transaction, ownership, and state contracts"

for installer_file in docs/en/installer.md docs/zh/installer.md; do
  for command_contract in \
    './oaw check' \
    './oaw install' \
    './oaw update' \
    './oaw uninstall' \
    './install.sh check' \
    './install.sh install' \
    './install.sh update' \
    './install.sh uninstall' \
    '--target <ids>' \
    '--target=<ids>' \
    '--project <path>' \
    '--project=<path>' \
    '--dry-run' \
    '--force' \
    '-h' \
    '--help' \
    'claude,codex,gemini,opencode' \
    'claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot' \
    'registry order' \
    '<crc>-<bytes>.state' \
    'current checkout' \
    '0, 64, 65, 66, 69, 70, 73, and 74'; do
    assert_contains "$installer_file" "$command_contract"
  done
done
for installer_contract in \
  'writes no managed content, state, backups, or directories' \
  'drift exits 65' \
  'removes only clean OAW ownership' \
  'only OAW-created empty directories' \
  'state is a successful no-op' \
  'does not add or remove' \
  '`66` | `update` was requested without installation state'; do
  assert_contains docs/en/installer.md "$installer_contract"
done
for installer_contract in \
  '不写入 managed content、state、backup 或目录' \
  'drift 以 65 退出' \
  '只删除干净的 OAW ownership' \
  '只清理 OAW 创建的空目录' \
  '没有 state 的 `uninstall`' \
  '不添加或删除 target' \
  '`66` | 没有安装 state 时请求 `update`'; do
  assert_contains docs/zh/installer.md "$installer_contract"
done
for management_contract in \
  'Public installation management is Go-authoritative' \
  '`install.sh` is an offline sibling-binary compatibility wrapper' \
  'Release archives contain precompiled binaries' \
  'go build -o ./oaw ./cmd/oaw' \
  'same public production binary' \
  'independent test oracle' \
  'A normal `install` creates no operation backup' \
  'preserves any existing valid `backup` reference' \
  'mutation journal' \
  'rollback failure' \
    'Install State and Workflow State are disjoint; no automatic migration occurs' \
  'Stable Boundary'; do
  assert_contains docs/en/installer.md "$management_contract"
done
for management_contract in \
  '公开安装管理以 Go 为权威实现' \
  '`install.sh` 是离线的同目录二进制兼容包装器' \
  '发布归档包含预编译二进制' \
  'go build -o ./oaw ./cmd/oaw' \
  '同一个公开 production binary' \
  '独立测试 oracle' \
  '普通 `install` 不创建 operation backup' \
  '保留已有且有效的 `backup` 引用' \
  'mutation journal' \
  'Rollback failure' \
    'Install State 与 Workflow State 相互独立，不会自动迁移' \
  'Stable Boundary'; do
  assert_contains docs/zh/installer.md "$management_contract"
done
pass "installer documents cover public Go management, wrapper, source/release, state, rollback, commands, and exits"

DOCS_RUNTIME_HOME=$DOCS_TEST_TEMP/runtime/home
DOCS_RUNTIME_CONFIG=$DOCS_TEST_TEMP/runtime/config
DOCS_RUNTIME_STATE=$DOCS_TEST_TEMP/runtime/state
DOCS_RUNTIME_RELEASE=$DOCS_TEST_TEMP/runtime/release
DOCS_RUNTIME_INSTALLER=$DOCS_RUNTIME_RELEASE/install.sh
mkdir -p "$DOCS_RUNTIME_HOME" "$DOCS_RUNTIME_CONFIG" "$DOCS_RUNTIME_STATE" \
  "$DOCS_RUNTIME_RELEASE"
cp "$REPOSITORY/install.sh" "$DOCS_RUNTIME_INSTALLER"
chmod 755 "$DOCS_RUNTIME_INSTALLER"
(cd "$REPOSITORY" && go build -o "$DOCS_RUNTIME_RELEASE/oaw" ./cmd/oaw)

if installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
  XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
  bash "$DOCS_RUNTIME_INSTALLER" 2>&1); then
  installer_status=0
else
  installer_status=$?
fi
[ "$installer_status" -eq 0 ] || fail "no-argument help returned $installer_status"
case "$installer_output" in
  *'Usage: ./install.sh <command> [options]'*) ;;
  *) fail "no-argument help omitted usage text" ;;
esac

for help_form in help short-help long-help command-help; do
  case "$help_form" in
    help) set -- help ;;
    short-help) set -- -h ;;
    long-help) set -- --help ;;
    command-help) set -- install --help ;;
  esac
  if installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
    XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
    bash "$DOCS_RUNTIME_INSTALLER" "$@" 2>&1); then
    installer_status=0
  else
    installer_status=$?
  fi
  [ "$installer_status" -eq 0 ] ||
    fail "$help_form returned $installer_status: $installer_output"
  case "$installer_output" in
    *'Usage: ./install.sh <command> [options]'*) ;;
    *) fail "$help_form omitted usage text" ;;
  esac
done

if installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
  XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
  bash "$DOCS_RUNTIME_INSTALLER" uninstall --target claude 2>&1); then
  installer_status=0
else
  installer_status=$?
fi
[ "$installer_status" -eq 0 ] ||
  fail "uninstall without state returned $installer_status: $installer_output"

if installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
  XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
  bash "$DOCS_RUNTIME_INSTALLER" update --target claude 2>&1); then
  installer_status=0
else
  installer_status=$?
fi
[ "$installer_status" -eq 66 ] ||
  fail "update without state returned $installer_status instead of 66: $installer_output"

if ! installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
  XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
  bash "$DOCS_RUNTIME_INSTALLER" install --target claude 2>&1); then
  fail "runtime fixture install failed: $installer_output"
fi

if installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
  XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
  bash "$DOCS_RUNTIME_INSTALLER" update --target codex 2>&1); then
  installer_status=0
else
  installer_status=$?
fi
[ "$installer_status" -eq 65 ] ||
  fail "update added an uninstalled target or returned $installer_status: $installer_output"

awk '
  $0 == "<!-- END OPEN AGENT WORKFLOW -->" { print "managed drift" }
  { print }
' "$DOCS_RUNTIME_HOME/.claude/CLAUDE.md" >"$DOCS_RUNTIME_HOME/.claude/CLAUDE.md.drift"
mv "$DOCS_RUNTIME_HOME/.claude/CLAUDE.md.drift" \
  "$DOCS_RUNTIME_HOME/.claude/CLAUDE.md"
if installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
  XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
  bash "$DOCS_RUNTIME_INSTALLER" check --target claude 2>&1); then
  installer_status=0
else
  installer_status=$?
fi
[ "$installer_status" -eq 0 ] ||
  fail "check returned $installer_status while reporting drift: $installer_output"
case "$installer_output" in
  *'installed claude: drift'*) ;;
  *) fail "check did not report target drift: $installer_output" ;;
esac

printf '%s\n' 'invalid state fixture' \
  >"$DOCS_RUNTIME_STATE/open-agent-workflow/installations/user.state"
if installer_output=$(env HOME="$DOCS_RUNTIME_HOME" \
  XDG_CONFIG_HOME="$DOCS_RUNTIME_CONFIG" XDG_STATE_HOME="$DOCS_RUNTIME_STATE" \
  bash "$DOCS_RUNTIME_INSTALLER" check --target claude 2>&1); then
  installer_status=0
else
  installer_status=$?
fi
[ "$installer_status" -eq 0 ] ||
  fail "check returned $installer_status while reporting invalid state: $installer_output"
case "$installer_output" in
  *'installed claude: invalid-state'*) ;;
  *) fail "check did not report invalid state: $installer_output" ;;
esac
pass "black-box CLI behavior matches documented help, state, target-set, and check exits"

for adapter_file in docs/en/adapters.md docs/zh/adapters.md; do
  for adapter_path in \
    '$HOME/.claude/CLAUDE.md' \
    '$HOME/.codex/AGENTS.md' \
    '$HOME/.gemini/GEMINI.md' \
    '$XDG_CONFIG_HOME/opencode/AGENTS.md' \
    '.claude/CLAUDE.md' \
    'AGENTS.md' \
    'GEMINI.md' \
    '.cursor/rules/open-agent-workflow.mdc' \
    '.devin/rules/open-agent-workflow.md' \
    '.clinerules/open-agent-workflow.md' \
    '.roo/rules/open-agent-workflow.md' \
    '.github/instructions/open-agent-workflow.instructions.md'; do
    assert_contains "$adapter_file" "$adapter_path"
  done
  for adapter_contract in \
    'user + project' \
    'project only' \
    'documented import' \
    'OAW bootstrap' \
    'precedence' \
    'reload' \
    'Retrieved: 2026-07-30'; do
    assert_contains "$adapter_file" "$adapter_contract"
  done
done
for adapter_contract in \
  'HTML comments are stripped from injected context' \
  'no documented Markdown import' \
  'requires `.mdc`' \
  'prefers `.devin/rules`' \
  'experimental nested `AGENTS.md` behavior is not used'; do
  assert_contains docs/en/adapters.md "$adapter_contract"
done
for adapter_contract in \
  'HTML 注释会从注入的上下文中剥离' \
  '没有文档化的 Markdown import' \
  '要求 `.mdc`' \
  '优先使用 `.devin/rules`' \
  '实验性的嵌套 `AGENTS.md` 行为未被采用'; do
  assert_contains docs/zh/adapters.md "$adapter_contract"
done
for host_surface_contract in \
  'policy' \
  'host-native' \
  'CURRENT' \
  'SUBAGENT' \
  'session-dependent' \
  'session facts' \
  'Receipts' \
  'Agent Host owns physical execution authority'; do
  assert_contains docs/en/adapters.md "$host_surface_contract"
  assert_contains docs/zh/adapters.md "$host_surface_contract"
done
for host_surface_contract in \
  'reports facts and Receipts' \
  'never gives OAW a model command' \
  'credential' \
  'Hook payload' \
  'private Plugin/MCP configuration'; do
  assert_contains docs/en/extending-adapters.md "$host_surface_contract"
done
for host_surface_contract in \
  '报告 fact 与 Receipt' \
  '不会给 OAW model command' \
  'credential' \
  'Hook payload' \
  'private Plugin/MCP configuration'; do
  assert_contains docs/zh/extending-adapters.md "$host_surface_contract"
done
for host_surface_contract in \
  'Host sandbox and approvals' \
  'logical workflow authority' \
  'secret-free'; do
  assert_contains docs/en/extending-adapters.md "$host_surface_contract"
done
pass "adapter documents preserve paths, scopes, source evidence, loading behavior, and caveats"

for operations_document in \
  docs/en/extending-adapters.md \
  docs/zh/extending-adapters.md \
  docs/en/security.md \
  docs/zh/security.md \
  docs/en/troubleshooting.md \
  docs/zh/troubleshooting.md; do
  assert_file "$operations_document"
done

for extension_file in docs/en/extending-adapters.md docs/zh/extending-adapters.md; do
  for extension_contract in \
    'target ID' \
    'scope support' \
    '`managed-block`' \
    '`owned-file`' \
    'pure renderer' \
    'shared destination' \
    'Host-native' \
    'session facts' \
    'Receipts' \
    'official primary source' \
    'retrieval date' \
    'precedence' \
    'reload' \
    'hostile path' \
    'symlink' \
    'inert state' \
    'candidate -> project extension -> core' \
    'must not change lifecycle semantics' \
    'must not vendor a provider'; do
    assert_contains "$extension_file" "$extension_contract"
  done
done
for extension_contract in \
  'public `oaw` CLI' \
  'policy surface' \
  'host-native' \
  'reports facts and Receipts' \
  'never gives OAW a model command' \
  'credential' \
  'Hook payload' \
  'private Plugin/MCP configuration'; do
  assert_contains docs/en/extending-adapters.md "$extension_contract"
done
for extension_contract in \
  'public `oaw` CLI' \
  'policy surface' \
  'host-native' \
  '报告 fact 与 Receipt' \
  '不会给 OAW model command' \
  'credential' \
  'Hook payload' \
  'private Plugin/MCP configuration'; do
  assert_contains docs/zh/extending-adapters.md "$extension_contract"
done
pass "extension documents define evidence, metadata, rendering, collision, fixtures, security, and graduation"

for security_file in docs/en/security.md docs/zh/security.md; do
  for security_contract in \
    '`HOME`' \
    '`XDG_CONFIG_HOME`' \
    '`XDG_STATE_HOME`' \
    'physical project root' \
    'control characters' \
    'symlink' \
    'containment' \
    'inert tab-separated data' \
    'never sourced or evaluated' \
    'prepare phase' \
    'apply revalidation' \
    'atomic replacement per destination' \
    'not operation-wide atomicity' \
    '`--force`' \
    '`manifest.tsv`' \
    'manual recovery' \
    'same local account' \
    'does not access the network'; do
    assert_contains "$security_file" "$security_contract"
  done
done
pass "security documents define trust boundaries, path defenses, inert state, transactions, and recovery limits"

for troubleshooting_file in docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
  for troubleshooting_contract in \
    './install.sh check' \
    './install.sh update --dry-run' \
    './install.sh update --project /absolute/path --target claude --force' \
    '`manifest.tsv`' \
    'check exits 0' \
    'update exits 66' \
    'mutation exits 65' \
    'missing provider' \
    'restart the target agent' \
    'restore backups manually' \
    'uninstall refusal' \
    'rollback failed' \
    'Install State' \
    'Workflow State' \
    'SCHEMA_UNSUPPORTED' \
    'WORKFLOW_STATE_UNSUPPORTED' \
    'SUBAGENT_UNAVAILABLE' \
    'HOST_SESSION_CHANGED' \
    'precompiled sibling binary is missing or not executable'; do
    assert_contains "$troubleshooting_file" "$troubleshooting_contract"
  done
done
pass "troubleshooting documents provide exact diagnosis, recovery, provider, reload, and uninstall guidance"

printf 'PASS: governance documentation contracts passed\n'
