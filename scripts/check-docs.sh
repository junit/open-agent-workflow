#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)
CHECK_TEMP=

cleanup() {
  if [ -n "$CHECK_TEMP" ] && [ -d "$CHECK_TEMP" ]; then
    rm -rf -- "$CHECK_TEMP"
  fi
}

fail() {
  printf 'docs: error: %s\n' "$*" >&2
  exit 1
}

require_literal() {
  document=$1
  expected=$2

  grep -F -- "$expected" "$REPOSITORY/$document" >/dev/null ||
    fail "$document omits required release boundary: $expected"
}

reject_literal() {
  document=$1
  forbidden=$2

  if grep -F -- "$forbidden" "$REPOSITORY/$document" >/dev/null; then
    fail "$document contains stale release boundary: $forbidden"
  fi
}

trap cleanup EXIT HUP INT TERM

CHECK_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-docs.XXXXXX") ||
  fail "cannot create temporary directory"

[ -f "$REPOSITORY/VERSION" ] || fail "VERSION is missing"
FIXED_VERSION=$(sed -n '1{s/\r$//;p;}' "$REPOSITORY/VERSION")
[ -n "$FIXED_VERSION" ] || fail "VERSION is empty"
printf '%s\n' "$FIXED_VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' ||
  fail "VERSION is not a fixed semantic version: $FIXED_VERSION"
require_literal CHANGELOG.md "## [$FIXED_VERSION]"
require_literal README.md "source baseline is fixed at v$FIXED_VERSION"
require_literal README-zh.md "固定为 v$FIXED_VERSION"

PAIRED_DOCUMENTS='
README.md|README-zh.md
CONTRIBUTING.md|CONTRIBUTING-zh.md
SECURITY.md|SECURITY-zh.md
docs/en/background.md|docs/zh/background.md
docs/en/comparison.md|docs/zh/comparison.md
docs/en/lifecycle.md|docs/zh/lifecycle.md
docs/en/architecture.md|docs/zh/architecture.md
docs/en/installer.md|docs/zh/installer.md
docs/en/machine-assurance.md|docs/zh/machine-assurance.md
docs/en/codex-bridge.md|docs/zh/codex-bridge.md
docs/en/adapters.md|docs/zh/adapters.md
docs/en/extending-adapters.md|docs/zh/extending-adapters.md
docs/en/security.md|docs/zh/security.md
docs/en/troubleshooting.md|docs/zh/troubleshooting.md
'

printf '%s\n' "$PAIRED_DOCUMENTS" >"$CHECK_TEMP/document-pairs"
while IFS='|' read -r english_file chinese_file; do
  [ -n "$english_file$chinese_file" ] || continue
  [ -f "$REPOSITORY/$english_file" ] ||
    fail "missing paired document: $english_file"
  [ -f "$REPOSITORY/$chinese_file" ] ||
    fail "missing paired document: $chinese_file"
done <"$CHECK_TEMP/document-pairs"

for command in check install update uninstall; do
  for readme in README.md README-zh.md; do
    grep -F "./install.sh $command" "$REPOSITORY/$readme" >/dev/null ||
      fail "$readme omits public command: $command"
  done
done

grep -F "experience-based" "$REPOSITORY/docs/en/comparison.md" >/dev/null ||
  fail "English comparison omits its experience-based caveat"
grep -F "基于经验" "$REPOSITORY/docs/zh/comparison.md" >/dev/null ||
  fail "Chinese comparison omits its experience-based caveat"

cat >"$CHECK_TEMP/release-boundaries" <<'EOF'
README.md|Public installation management is Go-authoritative.
README.md|`install.sh` is an offline sibling-binary compatibility wrapper.
README.md|Release archives contain precompiled binaries and perform no runtime executable download.
README.md|Installation management distributes the canonical Policy and target-native instruction entrypoints; it does not execute engineering work.
README.md|The cooperative Policy path does not require OAW Core. On machine-backed paths, OAW Core is required and stateless. The Workflow Coordinator is optional and stores only Workflow State for `WORKFLOW`; Install State and Workflow State are disjoint, with no migration or implicit adoption.
README.md|The Agent Host owns Agents, model calls, MCP, Hooks, Skills, Plugins, authentication, tools, sandbox, approvals, and every physical effect. OAW never starts a model process.
README.md|Codex has a policy integration by default and a separate audited host-native Bridge
README.md|Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness.
README-zh.md|公开安装管理以 Go 为权威实现。
README-zh.md|`install.sh` 是离线的同目录二进制兼容包装器。
README-zh.md|发布归档包含预编译二进制，运行时不会下载可执行文件。
README-zh.md|安装管理只分发 canonical Policy 和 target-native 指令入口，不执行工程工作。
README-zh.md|协作式 Policy 路径不需要 OAW Core。只有机器支撑路径才要求无状态的 OAW Core。Workflow Coordinator 是可选的，只为 `WORKFLOW` 保存
README-zh.md|Agent Host 拥有 Agent、model call、MCP、Hook、Skill、Plugin、认证、工具、sandbox、
README-zh.md|Codex 默认提供 policy integration，并另有独立且经过审计的 host-native Bridge
README-zh.md|可用的原生和 Docker smoke test 必须通过；不可用的平台检查返回 77，且不阻塞 release readiness。
EOF
while IFS='|' read -r boundary_document boundary_text; do
  require_literal "$boundary_document" "$boundary_text"
done <"$CHECK_TEMP/release-boundaries"

cat >"$CHECK_TEMP/bridge-boundaries" <<'EOF'
docs/en/architecture.md|oaw/codex-host
docs/zh/architecture.md|oaw/codex-host
docs/en/lifecycle.md|observe_current
docs/zh/lifecycle.md|observe_current
docs/en/troubleshooting.md|HOST_BRIDGE_PROTOCOL_MISMATCH
docs/zh/troubleshooting.md|HOST_BRIDGE_PROTOCOL_MISMATCH
EOF
while IFS='|' read -r boundary_document boundary_text; do
  require_literal "$boundary_document" "$boundary_text"
done <"$CHECK_TEMP/bridge-boundaries"

cat >"$CHECK_TEMP/current-user-documents" <<'EOF'
README.md
README-zh.md
CHANGELOG.md
CONTRIBUTING.md
CONTRIBUTING-zh.md
SECURITY.md
SECURITY-zh.md
docs/en/installer.md
docs/zh/installer.md
docs/en/architecture.md
docs/zh/architecture.md
docs/en/troubleshooting.md
docs/zh/troubleshooting.md
docs/en/extending-adapters.md
docs/zh/extending-adapters.md
EOF
cat >"$CHECK_TEMP/stale-release-boundaries" <<'EOF'
Bash remains authoritative
Bash 仍是权威
public `oaw install` is not enabled
public `oaw install` 尚未启用
zero-dependency Bash installer
actual WSL smoke pass is required before publishing a release
release readiness remains blocked until
发布前必须在真实 WSL 环境通过 smoke test
仍不具备 release readiness
EOF
while IFS= read -r current_document; do
  while IFS= read -r stale_boundary; do
    reject_literal "$current_document" "$stale_boundary"
  done <"$CHECK_TEMP/stale-release-boundaries"
done <"$CHECK_TEMP/current-user-documents"

CHECK_VIOLATIONS="$CHECK_TEMP/forbidden-execution-vocabulary-violations"
: >"$CHECK_VIOLATIONS"
cat >"$CHECK_TEMP/forbidden-execution-vocabulary" <<'EOF'
Runtime Plane
Runtime-managed
oaw runtime exchange
oaw run --host codex
oaw/codex-runner
runner-managed
native-managed
INLINE
NATIVE_SUBAGENT
main-agent-allowed
isolated-required
private HOME
Codex Runner
EOF
for current_document_path in \
  "$REPOSITORY/policy/ENGINEERING.md" \
  "$REPOSITORY/README.md" \
  "$REPOSITORY/README-zh.md" \
  "$REPOSITORY/SECURITY.md" \
  "$REPOSITORY/SECURITY-zh.md" \
  "$REPOSITORY"/docs/en/*.md \
  "$REPOSITORY"/docs/zh/*.md; do
  [ -f "$current_document_path" ] ||
    fail "missing current documentation source: ${current_document_path#"$REPOSITORY"/}"
  current_document=${current_document_path#"$REPOSITORY"/}
  while IFS= read -r forbidden_literal; do
    [ -n "$forbidden_literal" ] || continue
    if grep -nF -- "$forbidden_literal" "$current_document_path" \
      >"$CHECK_TEMP/forbidden-execution-vocabulary-matches"; then
      while IFS=: read -r line_number _; do
        printf '%s:%s:%s\n' "$current_document" "$line_number" \
          "$forbidden_literal" >>"$CHECK_VIOLATIONS"
      done <"$CHECK_TEMP/forbidden-execution-vocabulary-matches"
    fi
  done <"$CHECK_TEMP/forbidden-execution-vocabulary"
done
if [ -s "$CHECK_VIOLATIONS" ]; then
  while IFS= read -r violation; do
    printf 'docs: error: stale execution vocabulary: %s\n' "$violation" >&2
  done <"$CHECK_VIOLATIONS"
  exit 1
fi

cat >"$CHECK_TEMP/forbidden-authority-claims" <<'EOF'
OAW starts a model process
OAW launches a model process
OAW starts a child process
OAW creates a child process
OAW creates a child session
The Bridge creates a child session
OAW guarantees Host extension inheritance
OAW guarantees MCP inheritance
OAW guarantees Hook inheritance
OAW guarantees Skill inheritance
OAW guarantees Plugin inheritance
OAW 启动 model process
OAW 创建 child process
OAW 创建 child session
OAW 保证 Host extension inheritance
OAW 保证 MCP inheritance
OAW 保证 Hook inheritance
OAW 保证 Skill inheritance
OAW 保证 Plugin inheritance
EOF
for current_document_path in \
  "$REPOSITORY/policy/ENGINEERING.md" \
  "$REPOSITORY/README.md" \
  "$REPOSITORY/README-zh.md" \
  "$REPOSITORY/SECURITY.md" \
  "$REPOSITORY/SECURITY-zh.md" \
  "$REPOSITORY"/docs/en/*.md \
  "$REPOSITORY"/docs/zh/*.md; do
  current_document=${current_document_path#"$REPOSITORY"/}
  while IFS= read -r forbidden_claim; do
    [ -n "$forbidden_claim" ] || continue
    if grep -nF -- "$forbidden_claim" "$current_document_path" \
      >"$CHECK_TEMP/forbidden-authority-claim-matches"; then
      while IFS=: read -r line_number _; do
        printf 'docs: error: forbidden positive authority claim: %s:%s:%s\n' \
          "$current_document" "$line_number" "$forbidden_claim" >&2
      done <"$CHECK_TEMP/forbidden-authority-claim-matches"
      exit 1
    fi
  done <"$CHECK_TEMP/forbidden-authority-claims"
done

cat >"$CHECK_TEMP/core-boundary-documents" <<'EOF'
docs/en/architecture.md
docs/zh/architecture.md
docs/en/lifecycle.md
docs/zh/lifecycle.md
docs/en/security.md
docs/zh/security.md
EOF
while IFS= read -r boundary_document; do
  for boundary_literal in \
    'OAW Core' \
    'Workflow Coordinator' \
    'Agent Host' \
    'CURRENT' \
    'SUBAGENT' \
    'logical workflow authority'; do
    require_literal "$boundary_document" "$boundary_literal"
  done
  case "$boundary_document" in
    docs/en/*)
      require_literal "$boundary_document" 'physical execution authority'
      ;;
    docs/zh/*)
      require_literal "$boundary_document" 'Agent Host 拥有物理执行权限'
      ;;
  esac
done <"$CHECK_TEMP/core-boundary-documents"

cat >"$CHECK_TEMP/host-scope-documents" <<'EOF'
README.md
README-zh.md
docs/en/architecture.md
docs/zh/architecture.md
docs/en/lifecycle.md
docs/zh/lifecycle.md
docs/en/troubleshooting.md
docs/zh/troubleshooting.md
docs/en/security.md
docs/zh/security.md
policy/ENGINEERING.md
EOF
while IFS= read -r host_scope_document; do
  for authority_term in \
    'Provider Family' \
    'Distribution' \
    'Host Installation' \
    'Host Binding Evidence' \
    'Verified Provider Instance'; do
    require_literal "$host_scope_document" "$authority_term"
  done
done <"$CHECK_TEMP/host-scope-documents"

for lifecycle_document in \
  README.md README-zh.md \
  docs/en/lifecycle.md docs/zh/lifecycle.md \
  docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
  for pin_field in provider_id host_id installation_key evidence_digest; do
    require_literal "$lifecycle_document" "$pin_field"
  done
done

for diagnostic_document in docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
  for stable_reason in \
    HOST_BINDING_EVIDENCE_REQUIRED \
    PROVIDER_BINDING_UNAVAILABLE \
    PROVIDER_FOREIGN_HOST_ONLY \
    PROVIDER_PIN_INCOMPATIBLE \
    HOST_PROVIDER_SCOPE_MISMATCH; do
    require_literal "$diagnostic_document" "$stable_reason"
  done
  require_literal "$diagnostic_document" 'oaw.provider-descriptor/v1'
  require_literal "$diagnostic_document" 'oaw.user-config/v1'
  for bridge_reason in \
    HOST_BRIDGE_UNAVAILABLE \
    HOST_BRIDGE_CONTEXT_REQUIRED \
    HOST_BRIDGE_PROTOCOL_MISMATCH \
    HOST_EVIDENCE_HANDLE_REQUIRED \
    HOST_EVIDENCE_HANDLE_INVALID \
    HOST_EVIDENCE_EXPIRED \
    HOST_EVIDENCE_SESSION_MISMATCH \
    HOST_OBSERVATION_FAILED \
    HOST_OBSERVATION_PARTIAL \
    HOST_SESSION_CHANGED; do
    require_literal "$diagnostic_document" "$bridge_reason"
  done
done

cat >"$CHECK_TEMP/provider-surface-matrix-documents" <<'EOF'
policy/ENGINEERING.md
README.md
README-zh.md
docs/en/lifecycle.md
docs/zh/lifecycle.md
EOF
while IFS= read -r matrix_document; do
  for slot_id in \
    problem-framing \
    solution-specification \
    delivery-planning \
    workspace-preparation \
    implementation \
    implementation-tdd \
    incident-recovery \
    review-remediation \
    fresh-verification \
    closeout; do
    require_literal "$matrix_document" "$slot_id"
  done
  for selection_alias in \
    MATT-FULL \
    SP-FULL \
    ECC-FULL \
    MATT-SP-HYBRID \
    USER-DEFINED; do
    require_literal "$matrix_document" "$selection_alias"
  done
done <"$CHECK_TEMP/provider-surface-matrix-documents"

for comparison_document in docs/en/comparison.md docs/zh/comparison.md; do
  for upstream_revision in \
    84fdeffd12f2ee307994d1eb6feb48173b6e0502 \
    44c9b2d6e889982ac18c27d05a19fefe335194e1 \
    11c74d6ba24d3a6d48f54a194cd00ef3beea18f9 \
    2d46e80e0925c7be0907f18c1812311ac212a6c5; do
    require_literal "$comparison_document" "$upstream_revision"
  done
  require_literal "$comparison_document" 'superpowers-codex'
done

cat >"$CHECK_TEMP/provider-surface-version-tuple" <<'EOF'
oaw.provider-descriptor/v4
oaw.profile-recipe/v3
oaw.host-manifest/v3
oaw.host-session/v3
oaw.host-binding-inventory/v3
oaw.host-environment-report/v2
oaw.host-invocation-receipt/v3
oaw.host-conformance-transcript/v4
oaw.host-conformance-report/v4
oaw.execution-graph/v4
oaw.lifecycle-bundle/v4
oaw.capability-grant/v3
oaw.dispatch-packet/v2
oaw.workflow-command/v2
oaw.workflow-result/v2
oaw.workflow-snapshot/v2
oaw.workflow-revision/v2
oaw.codex-bridge/v2
oaw.codex-hook-context/v2
oaw.host-evidence-handle/v2
EOF
for bridge_document in docs/en/codex-bridge.md docs/zh/codex-bridge.md; do
  while IFS= read -r contract_version; do
    require_literal "$bridge_document" "$contract_version"
  done <"$CHECK_TEMP/provider-surface-version-tuple"
  require_literal "$bridge_document" '2.0.0'
  require_literal "$bridge_document" 'proof_scope: installation-integrity'
  require_literal "$bridge_document" 'live_protocol_proof: false'
  require_literal "$bridge_document" 'SubagentStart'
  require_literal "$bridge_document" 'child-delegation'
  require_literal "$bridge_document" 'agents.enabled'
done

for lifecycle_document in policy/ENGINEERING.md docs/en/lifecycle.md docs/zh/lifecycle.md; do
  for host_action in \
    workspace.prepare-or-confirm \
    verification.execute \
    closeout.execute; do
    require_literal "$lifecycle_document" "$host_action"
  done
  for neutral_gate in \
    shared-understanding \
    specification-approved \
    delivery-plan-approved \
    workspace-ready \
    fresh-evidence \
    user-closeout; do
    require_literal "$lifecycle_document" "$neutral_gate"
  done
  for macro_contract in credit-only dispatch-before dispatch-after MACRO_INTERNAL_CONFLICT; do
    require_literal "$lifecycle_document" "$macro_contract"
  done
done

for binding_document in \
  policy/ENGINEERING.md \
  docs/en/adapters.md docs/zh/adapters.md \
  docs/en/extending-adapters.md docs/zh/extending-adapters.md; do
  for binding_kind in skill agent role instruction; do
    require_literal "$binding_document" "$binding_kind"
  done
  require_literal "$binding_document" 'Hooks'
  require_literal "$binding_document" 'tools'
done

require_literal README.md 'after a valid `SubagentStart` event, the next observation may additionally prove `child-delegation`'
require_literal README-zh.md '在有效 `SubagentStart` event 后，下一次 observation 还可以为精确 session/CWD 证明 `child-delegation`'
require_literal docs/en/architecture.md '`SubagentStart` event can additionally prove `child-delegation` for the exact'
require_literal docs/zh/architecture.md '有效 `SubagentStart` event 可以为精确 session/CWD 额外证明 `child-delegation`'
require_literal policy/ENGINEERING.md 'Startup Gate Host capability probe'
require_literal policy/ENGINEERING.md 'explicitly requested a Profile and topology'
require_literal policy/ENGINEERING.md 'Governance observation'

cat >"$CHECK_TEMP/activation-policy-contract" <<'EOF'
Native Host is the default. It is not an OAW Request Mode.
Request Mode is evaluated only after explicit activation.
Assurance Level is orthogonal to Request Mode.
policy-cooperative
core-backed
coordinator-backed
Activated `BOUNDED` is not a generic Skill router.
The current `bounded_capability_defaults` interface does not define a matching predicate
Policy-only execution supports `CURRENT`. It cannot declare `SUBAGENT` eligible
Policy Workflow Plan
Progress Tracker
CAPABILITY_SELECTION_REQUIRED
POLICY_ONLY_PROVIDER_UNVERIFIED
POLICY_ONLY_PROFILE_INCOMPLETE
POLICY_ONLY_TOPOLOGY_UNAVAILABLE
POLICY_ONLY_GUARANTEE_UNAVAILABLE
POLICY_ONLY_CONCURRENT_MUTATION
POLICY_ONLY_EXECUTION_UNCERTAIN
POLICY_ONLY_CONTEXT_UNCERTAIN
EOF
while IFS= read -r activation_contract; do
  require_literal policy/ENGINEERING.md "$activation_contract"
done <"$CHECK_TEMP/activation-policy-contract"

cat >"$CHECK_TEMP/stale-activation-policy-contract" <<'EOF'
Classify every new top-level engineering request as exactly one Request Mode:
In policy-only use, the caller receives the same Core-produced Bundle
Policy-only Hosts may coordinate the same ownership model with a local lock
EOF
while IFS= read -r stale_activation_contract; do
  reject_literal policy/ENGINEERING.md "$stale_activation_contract"
done <"$CHECK_TEMP/stale-activation-policy-contract"

cat >"$CHECK_TEMP/activation-document-contract" <<'EOF'
README.md|## Explicit Activation
README.md|Native Host
README.md|OAW Engagement
README.md|Assurance Preflight
README.md|policy-cooperative
README.md|core-backed
README.md|coordinator-backed
README-zh.md|## 显式激活
README-zh.md|原生 Host
README-zh.md|OAW Engagement
README-zh.md|保证等级预检
README-zh.md|policy-cooperative
README-zh.md|core-backed
README-zh.md|coordinator-backed
docs/en/background.md|explicit activation
docs/en/background.md|Native Host
docs/en/background.md|OAW Engagement
docs/zh/background.md|显式激活
docs/zh/background.md|原生 Host
docs/zh/background.md|OAW Engagement
docs/en/lifecycle.md|Assurance Preflight
docs/en/lifecycle.md|Policy Workflow Plan
docs/en/lifecycle.md|Progress Tracker
docs/zh/lifecycle.md|保证等级预检
docs/zh/lifecycle.md|协作式 Policy Workflow Plan
docs/zh/lifecycle.md|Progress Tracker
docs/en/architecture.md|Activation Router
docs/en/architecture.md|Assurance Preflight
docs/en/architecture.md|policy-cooperative
docs/zh/architecture.md|Activation Router
docs/zh/architecture.md|保证等级预检
docs/zh/architecture.md|policy-cooperative
docs/en/adapters.md|Activation Router
docs/en/adapters.md|Claude and Gemini do not emit `@`
docs/zh/adapters.md|Activation Router
docs/zh/adapters.md|Claude 和 Gemini 不会输出 `@`
docs/en/extending-adapters.md|Activation Router
docs/en/extending-adapters.md|eager Policy import
docs/zh/extending-adapters.md|Activation Router
docs/zh/extending-adapters.md|急切导入 Policy
docs/en/installer.md|Installation does not activate OAW
docs/en/installer.md|Activation Router
docs/zh/installer.md|安装不会激活 OAW
docs/zh/installer.md|Activation Router
docs/en/comparison.md|after explicit activation
docs/en/comparison.md|Normal Host Skill routing
docs/zh/comparison.md|显式激活后
docs/zh/comparison.md|原生 Host Skill routing
docs/en/codex-bridge.md|Bridge installation does not activate OAW
docs/en/codex-bridge.md|active OAW Engagement
docs/zh/codex-bridge.md|安装 Bridge 不会激活 OAW
docs/zh/codex-bridge.md|活跃 OAW Engagement
docs/en/security.md|current top-level user instruction
docs/en/security.md|Policy Workflow Plan cannot grant
docs/zh/security.md|当前顶层用户指令
docs/zh/security.md|Policy Workflow Plan 不能授予
SECURITY.md|current top-level user instruction
SECURITY.md|Policy Workflow Plan cannot grant
SECURITY-zh.md|当前顶层用户指令
SECURITY-zh.md|Policy Workflow Plan 不能授予
docs/en/troubleshooting.md|Install State is not a Progress Tracker or Workflow State
docs/zh/troubleshooting.md|Install State 不是 Progress Tracker 或 Workflow State
CHANGELOG.md|OAW is now explicitly activated per deliverable
CHANGELOG.md|lazy Activation Router
CHANGELOG.md|policy-only Markdown lifecycle locks are not converted
EOF
while IFS='|' read -r activation_document activation_literal; do
  require_literal "$activation_document" "$activation_literal"
done <"$CHECK_TEMP/activation-document-contract"

cat >"$CHECK_TEMP/policy-projection-contract" <<'EOF'
README.md|## No-Bridge Policy Workflow
README.md|policy_selectable
README.md|host_routable
README-zh.md|## 无 Bridge Policy Workflow
README-zh.md|policy_selectable
README-zh.md|host_routable
docs/en/architecture.md|## Policy and Machine Profile Projections
docs/en/architecture.md|Machine attestation may increase assurance, but it cannot veto a Policy Offer.
docs/zh/architecture.md|## Policy 与 Machine Profile Projection
docs/zh/architecture.md|machine attestation 可以提高 assurance，但不能 veto
docs/en/lifecycle.md|policy_selectable
docs/en/lifecycle.md|host_routable
docs/en/lifecycle.md|incident_routes
docs/en/troubleshooting.md|incident_routes
docs/zh/lifecycle.md|policy_selectable
docs/zh/lifecycle.md|host_routable
docs/zh/lifecycle.md|incident_routes
docs/zh/troubleshooting.md|incident_routes
EOF
while IFS='|' read -r projection_document projection_literal; do
  require_literal "$projection_document" "$projection_literal"
done <"$CHECK_TEMP/policy-projection-contract"

for activation_lifecycle_document in docs/en/lifecycle.md docs/zh/lifecycle.md; do
  for assurance_level in policy-cooperative core-backed coordinator-backed; do
    require_literal "$activation_lifecycle_document" "$assurance_level"
  done
done

for cooperative_stop_document in \
  docs/en/lifecycle.md docs/zh/lifecycle.md \
  docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
  for cooperative_stop_reason in \
    CAPABILITY_SELECTION_REQUIRED \
    POLICY_ONLY_PROVIDER_UNVERIFIED \
    POLICY_ONLY_PROFILE_INCOMPLETE \
    POLICY_ONLY_TOPOLOGY_UNAVAILABLE \
    POLICY_ONLY_GUARANTEE_UNAVAILABLE \
    POLICY_ONLY_CONCURRENT_MUTATION \
    POLICY_ONLY_EXECUTION_UNCERTAIN \
    POLICY_ONLY_CONTEXT_UNCERTAIN; do
    require_literal "$cooperative_stop_document" "$cooperative_stop_reason"
  done
done

cat >"$CHECK_TEMP/stale-activation-document-contract" <<'EOF'
OAW performs enough read-only inspection to classify each top-level engineering
OAW Core classifies each new top-level engineering request
OAW 通过足够的只读检查，把每个顶层工程请求分类为
OAW Core 在选择工程方法前，对每个新顶层工程请求分类
Bounded Mode is the Atomic Skill mode.
Policy-only use follows the same lifecycle ownership rules
A policy-only lock
OAW writes a managed block containing `@<canonical-policy-path>`
Claude and Gemini use documented Markdown import behavior.
the lifecycle gate applies before engineering lifecycle work anywhere in the project
EOF
for activation_document in \
  README.md README-zh.md SECURITY.md SECURITY-zh.md CHANGELOG.md \
  "$REPOSITORY"/docs/en/*.md "$REPOSITORY"/docs/zh/*.md; do
  case "$activation_document" in
    "$REPOSITORY"/*) activation_document=${activation_document#"$REPOSITORY"/} ;;
  esac
  while IFS= read -r stale_activation_literal; do
    reject_literal "$activation_document" "$stale_activation_literal"
  done <"$CHECK_TEMP/stale-activation-document-contract"
done

for troubleshooting_document in docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
  for provider_surface_reason in \
    PROVIDER_BINDING_CONTENT_MISMATCH \
    BINDING_EXPLICIT_INVOCATION_REQUIRED \
    HOST_FEATURE_UNATTESTED \
    HOST_ACTION_UNAVAILABLE \
    MACRO_INTERNAL_CONFLICT \
    PROFILE_TOPOLOGY_UNAVAILABLE \
    WORKFLOW_STATE_UNSUPPORTED; do
    require_literal "$troubleshooting_document" "$provider_surface_reason"
  done
  require_literal "$troubleshooting_document" 'bounded native child probe'
  require_literal "$troubleshooting_document" 'Startup Gate'
  require_literal "$troubleshooting_document" 'observe_current'
done

require_literal internal/assets/providers/oaw-matt.json '"id":"oaw/matt"'
reject_literal internal/assets/providers/oaw-matt.json '"reference":"requirements"'
reject_literal internal/assets/providers/oaw-matt.json '"reference":"verification-loop"'

cat >"$CHECK_TEMP/forbidden-provider-claims" <<'EOF'
Matt `requirements` skill
Matt requirements skill
Matt `verification-loop` skill
Matt verification-loop skill
Matt owns workspace creation
Matt owns broad final verification
Matt owns remediation
Matt owns completion
ECC `e2e-runner` owns broad final verification
ECC `e2e-testing` owns broad final verification
ECC `code-reviewer` owns closeout
ECC `code-reviewer` owns completion
Claude custom Agent is a verified Codex Role
static multi-agent configuration proves live delegation
compatibility reader
compatibility decoder
remove MATT-FULL
remove MATT-SP-HYBRID
Matt `requirements` 技能
Matt requirements 技能
Matt `verification-loop` 技能
Matt verification-loop 技能
Matt 拥有 workspace creation
Matt 拥有 broad final verification
Matt 拥有 remediation
Matt 拥有 completion
ECC `e2e-runner` 拥有 broad final verification
ECC `e2e-testing` 拥有 broad final verification
ECC `code-reviewer` 拥有 closeout
ECC `code-reviewer` 拥有 completion
Claude custom Agent 是 verified Codex Role
static multi-agent configuration 证明 live delegation
兼容 reader
兼容 decoder
删除 MATT-FULL
删除 MATT-SP-HYBRID
EOF
for current_document_path in \
  "$REPOSITORY/policy/ENGINEERING.md" \
  "$REPOSITORY/README.md" \
  "$REPOSITORY/README-zh.md" \
  "$REPOSITORY"/docs/en/*.md \
  "$REPOSITORY"/docs/zh/*.md; do
  current_document=${current_document_path#"$REPOSITORY"/}
  while IFS= read -r forbidden_claim; do
    [ -n "$forbidden_claim" ] || continue
    if grep -nF -- "$forbidden_claim" "$current_document_path" \
      >"$CHECK_TEMP/forbidden-provider-claim-matches"; then
      while IFS=: read -r line_number _; do
        printf 'docs: error: forbidden provider claim: %s:%s:%s\n' \
          "$current_document" "$line_number" "$forbidden_claim" >&2
      done <"$CHECK_TEMP/forbidden-provider-claim-matches"
      exit 1
    fi
  done <"$CHECK_TEMP/forbidden-provider-claims"
done

require_literal docs/adr/0003-add-optional-capability-admission-runtime.md \
  'Superseded by ADR 0009'
require_literal docs/adr/0007-use-host-native-execution-topologies.md \
  'Superseded by ADR 0009; Host-native control is retained'
require_literal docs/adr/0008-treat-subagent-environment-as-host-owned.md \
  'Superseded by ADR 0009; Host-owned environment semantics are retained'
require_literal docs/adr/0009-separate-core-coordination-and-host-execution.md \
  'Accepted'
require_literal docs/superpowers/specs/2026-08-04-oaw-host-native-execution-topology-design.md \
  'Superseded by the 2026-08-05 OAW Core and Workflow Coordinator hard-cutover design'
require_literal docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md \
  'Approved for implementation'

git -C "$REPOSITORY" rev-parse --is-inside-work-tree >/dev/null 2>&1 ||
  fail "documentation checks require a Git worktree"
git -C "$REPOSITORY" ls-files -- '*.md' >"$CHECK_TEMP/tracked-markdown-files"
: >"$CHECK_TEMP/markdown-files"
while IFS= read -r tracked_markdown_file; do
  [ -n "$tracked_markdown_file" ] || continue
  printf '%s/%s\n' "$REPOSITORY" "$tracked_markdown_file" \
    >>"$CHECK_TEMP/markdown-files"
done <"$CHECK_TEMP/tracked-markdown-files"

while IFS= read -r markdown_file; do
  # Parse destinations separately from optional titles. This covers inline and
  # reference-style links without requiring a non-standard Markdown runtime.
  awk '
    function emit_destination(raw,    character, cursor, escaped, result) {
      result = ""
      escaped = 0
      for (cursor = 1; cursor <= length(raw); cursor++) {
        character = substr(raw, cursor, 1)
        if (escaped) {
          result = result character
          escaped = 0
        } else if (character == "\\") {
          escaped = 1
        } else {
          result = result character
        }
      }
      if (escaped) {
        result = result "\\"
      }
      if (result != "") {
        print result
      }
    }

    function parse_destination(line, start, inline,    character, cursor, depth, escaped, raw) {
      cursor = start
      while (substr(line, cursor, 1) == " " || substr(line, cursor, 1) == "\t") {
        cursor++
      }

      if (substr(line, cursor, 1) == "<") {
        cursor++
        raw = ""
        escaped = 0
        while (cursor <= length(line)) {
          character = substr(line, cursor, 1)
          if (character == ">" && !escaped) {
            emit_destination(raw)
            return cursor + 1
          }
          raw = raw character
          if (character == "\\" && !escaped) {
            escaped = 1
          } else {
            escaped = 0
          }
          cursor++
        }
        return start
      }

      raw = ""
      depth = 0
      escaped = 0
      while (cursor <= length(line)) {
        character = substr(line, cursor, 1)
        if (escaped) {
          raw = raw character
          escaped = 0
        } else if (character == "\\") {
          raw = raw character
          escaped = 1
        } else if (character == "(") {
          depth++
          raw = raw character
        } else if (character == ")") {
          if (inline && depth == 0) {
            break
          }
          if (depth > 0) {
            depth--
          }
          raw = raw character
        } else if ((character == " " || character == "\t") && depth == 0) {
          break
        } else {
          raw = raw character
        }
        cursor++
      }
      emit_destination(raw)
      return cursor
    }

    function parse_inline_links(line,    marker, offset, start) {
      offset = 1
      while (offset <= length(line)) {
        marker = index(substr(line, offset), "](")
        if (!marker) {
          return
        }
        start = offset + marker + 1
        parse_destination(line, start, 1)
        offset = start + 1
      }
    }

    function normalize_reference(label) {
      gsub(/[ \t]+/, " ", label)
      sub(/^ /, "", label)
      sub(/ $/, "", label)
      return tolower(label)
    }

    function reference_opening(line, before,    cursor) {
      for (cursor = before; cursor >= 1; cursor--) {
        if (substr(line, cursor, 1) == "[" &&
            (cursor == 1 || substr(line, cursor - 1, 1) != "\\")) {
          return cursor
        }
      }
      return 0
    }

    function parse_reference_usages(line,    closing, label, marker, normalized, offset, opening) {
      offset = 1
      while (offset <= length(line)) {
        marker = index(substr(line, offset), "][")
        if (!marker) {
          return
        }
        marker = offset + marker - 1
        opening = reference_opening(line, marker - 1)
        closing = index(substr(line, marker + 2), "]")
        if (!opening || !closing) {
          offset = marker + 2
          continue
        }

        label = substr(line, marker + 2, closing - 1)
        if (label == "") {
          label = substr(line, opening + 1, marker - opening - 1)
        }
        normalized = normalize_reference(label)
        if (normalized != "") {
          reference_uses[normalized] = label
        }
        offset = marker + closing + 2
      }
    }

    function parse_reference_definition(line,    closing, content, label, leading, normalized) {
      leading = 0
      while (leading < 4 && substr(line, leading + 1, 1) == " ") {
        leading++
      }
      if (leading > 3) {
        return 0
      }
      content = substr(line, leading + 1)
      if (substr(content, 1, 1) != "[") {
        return 0
      }
      closing = index(content, "]:")
      if (closing < 2) {
        return 0
      }
      label = substr(content, 2, closing - 2)
      normalized = normalize_reference(label)
      if (normalized != "") {
        reference_definitions[normalized] = 1
      }
      parse_destination(content, closing + 2, 0)
      return 1
    }

    {
      content = $0
      leading = 0
      while (leading < 4 && substr(content, leading + 1, 1) == " ") {
        leading++
      }
      fence_probe = substr(content, leading + 1, 3)
      if (!in_fence && (fence_probe == "```" || fence_probe == "~~~")) {
        in_fence = 1
        fence_marker = fence_probe
        next
      }
      if (in_fence) {
        if (fence_probe == fence_marker) {
          in_fence = 0
        }
        next
      }

      is_definition = parse_reference_definition(content)
      if (!is_definition) {
        parse_reference_usages(content)
      }
      parse_inline_links(content)
    }

    END {
      for (reference in reference_uses) {
        if (!(reference in reference_definitions)) {
          print "__OAW_MISSING_REFERENCE__" reference_uses[reference]
        }
      }
    }
  ' "$markdown_file" >"$CHECK_TEMP/links"

  while IFS= read -r link_target; do
    case "$link_target" in
      ''|\#*|http://*|https://*|mailto:*) continue ;;
      __OAW_MISSING_REFERENCE__*)
        missing_reference=${link_target#__OAW_MISSING_REFERENCE__}
        fail "missing Markdown reference definition in ${markdown_file#"$REPOSITORY"/}: $missing_reference"
        ;;
      /*|file:*)
        fail "absolute local Markdown link in ${markdown_file#"$REPOSITORY"/}: $link_target"
        ;;
    esac
    link_target=${link_target%%#*}
    [ -n "$link_target" ] || continue
    [ -e "$(dirname -- "$markdown_file")/$link_target" ] ||
      fail "missing Markdown link target in ${markdown_file#"$REPOSITORY"/}: $link_target"
  done <"$CHECK_TEMP/links"
done <"$CHECK_TEMP/markdown-files"

printf 'PASS: bilingual documentation contracts and local links passed\n'
