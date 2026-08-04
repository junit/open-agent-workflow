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
    'Install State and Runtime State are disjoint; no automatic migration occurs.' \
    'Existing Policy-only tasks and profile locks remain Policy-only unless explicitly adopted at a Stable Boundary.' \
    'Only the pinned Codex runner is currently Runtime-managed.' \
    'Other installed adapters remain Policy-only and provide no Runtime admission, Capability Grant, Resource Lease, transition enforcement, or physical isolation guarantee.' \
    'Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness.' \
    >>"$fixture_root/README.md"
  printf '%s\n' \
    '公开安装管理以 Go 为权威实现。' \
    '`install.sh` 是离线的同目录二进制兼容包装器。' \
    '发布归档包含预编译二进制，运行时不会下载可执行文件。' \
    'Install State 与 Runtime State 相互独立，不会自动迁移。' \
    '现有 Policy-only task 和 profile lock 仍保持 Policy-only，除非在 Stable Boundary 显式接管。' \
    '目前只有固定版本的 Codex runner 是 Runtime-managed。' \
    '其他已安装 adapter 仍为 Policy-only，不提供 Runtime admission、Capability Grant、Resource Lease、transition enforcement 或 physical isolation 保证。' \
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
  for document_path in \
    README.md README-zh.md \
    docs/en/lifecycle.md docs/zh/lifecycle.md \
    docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
    printf '%s\n' provider_id host_id installation_key evidence_digest \
      >>"$fixture_root/$document_path"
  done
  for document_path in docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
    printf '%s\n' \
      HOST_BINDING_EVIDENCE_REQUIRED \
      PROVIDER_BINDING_UNAVAILABLE \
      PROVIDER_FOREIGN_HOST_ONLY \
      PROVIDER_PIN_INCOMPATIBLE \
      HOST_PROVIDER_SCOPE_MISMATCH \
      oaw.provider-descriptor/v1 \
      oaw.user-config/v1 \
      >>"$fixture_root/$document_path"
  done
  : >"$fixture_root/CHANGELOG.md"
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
  'Install State and Runtime State' \
  'Available native and Docker smoke tests must pass'; do
  assert_contains CONTRIBUTING.md "$contribution_contract"
done
for contribution_contract in \
  '公开 Go `oaw` 二进制' \
  '预编译的同目录二进制' \
  'Install State 与 Runtime State' \
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
  'Install State and Runtime State' \
  'Only the pinned Codex runner'; do
  assert_contains SECURITY.md "$security_contract"
done
for security_contract in \
  '公开 Go binary' \
  '预编译二进制' \
  '运行时不会下载可执行文件' \
  'Install State 与 Runtime State' \
  '只有固定版本的 Codex runner'; do
  assert_contains SECURITY-zh.md "$security_contract"
done
pass "bilingual security policies publish binary, state, and Runtime trust boundaries"

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
assert_contains CHANGELOG.md "Install State and revisioned Runtime State remain disjoint"
pass "changelog describes the local unreleased 0.1.0 candidate"

assert_executable scripts/check-docs.sh
assert_contains scripts/check-docs.sh "README.md|README-zh.md"
assert_contains scripts/check-docs.sh "docs/en/background.md|docs/zh/background.md"
assert_contains scripts/check-docs.sh "docs/en/extending-adapters.md|docs/zh/extending-adapters.md"
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
  'Install State and Runtime State are disjoint; no automatic migration occurs.' \
  'Existing Policy-only tasks and profile locks remain Policy-only unless explicitly adopted at a Stable Boundary.' \
  'Only the pinned Codex runner is currently Runtime-managed.' \
  'Other installed adapters remain Policy-only and provide no Runtime admission, Capability Grant, Resource Lease, transition enforcement, or physical isolation guarantee.' \
  'Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness.'; do
  assert_contains README.md "$release_boundary"
done
for release_boundary in \
  '公开安装管理以 Go 为权威实现。' \
  '`install.sh` 是离线的同目录二进制兼容包装器。' \
  '发布归档包含预编译二进制，运行时不会下载可执行文件。' \
  'Install State 与 Runtime State 相互独立，不会自动迁移。' \
  '现有 Policy-only task 和 profile lock 仍保持 Policy-only，除非在 Stable Boundary 显式接管。' \
  '目前只有固定版本的 Codex runner 是 Runtime-managed。' \
  '其他已安装 adapter 仍为 Policy-only，不提供 Runtime admission、Capability Grant、Resource Lease、transition enforcement 或 physical isolation 保证。' \
  '可用的原生和 Docker smoke test 必须通过；不可用的平台检查返回 77，且不阻塞 release readiness。'; do
  assert_contains README-zh.md "$release_boundary"
done
pass "bilingual README entrypoints publish the Go cutover and Runtime boundaries"

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
  '## Task Gate' \
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
  '## 任务门禁' \
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
pass "lifecycle documents explain Runtime vNext modes, extensible Profiles, locking, inheritance, add-ons, and switching"

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
  'Runtime admission' \
  'Resource Leases' \
  'physical isolation'; do
  assert_contains policy/ENGINEERING.md "$policy_contract"
done
assert_not_contains policy/ENGINEERING.md 'CUSTOM-LOCKED'
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
for runtime_boundary in \
  '${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/runtime' \
  'Only the pinned Codex runner is currently Runtime-managed' \
  'Install State and Runtime State are disjoint; no automatic migration occurs' \
  'Existing Policy-only tasks and profile locks remain Policy-only unless' \
  'Capability Grant' \
  'Resource Lease' \
  'mutation journal' \
  'automatic recovery from a process or machine crash'; do
  assert_contains docs/en/architecture.md "$runtime_boundary"
done
for runtime_boundary in \
  '${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/runtime' \
  '目前只有固定版本的 Codex runner 是 Runtime-managed' \
  'Install State 与 Runtime State 相互独立，不会自动迁移' \
  '现有 Policy-only task 和 profile' \
  'Capability Grant' \
  'Resource Lease' \
  'mutation journal' \
  'process 或 machine crash'; do
  assert_contains docs/zh/architecture.md "$runtime_boundary"
done
pass "architecture documents match management, Runtime, rendering, transaction, ownership, and state contracts"

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
  'Install State and Runtime State are disjoint; no automatic migration occurs' \
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
  'Install State 与 Runtime State 相互独立，不会自动迁移' \
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
for runtime_containment_contract in \
  '--sandbox read-only' \
  '--sandbox workspace-write' \
  'danger-full-access' \
  'EXECUTION_UNCERTAIN' \
  'RECONCILE_INVOCATION' \
  'SIGKILL'; do
  assert_contains docs/en/adapters.md "$runtime_containment_contract"
  assert_contains docs/zh/adapters.md "$runtime_containment_contract"
  assert_contains docs/en/security.md "$runtime_containment_contract"
  assert_contains docs/zh/security.md "$runtime_containment_contract"
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
    'isolated `HOME`' \
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
  'Runtime admission' \
  'Capability Grant' \
  'Resource Lease' \
  'physical isolation' \
  'Only the pinned Codex runner is currently Runtime-managed'; do
  assert_contains docs/en/extending-adapters.md "$extension_contract"
done
for extension_contract in \
  'public `oaw` CLI' \
  'Runtime admission' \
  'Capability Grant' \
  'Resource Lease' \
  'physical' \
  '目前只有固定版本的 Codex runner 是 Runtime-managed'; do
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
    'Runtime State' \
    'precompiled sibling binary is missing or not executable'; do
    assert_contains "$troubleshooting_file" "$troubleshooting_contract"
  done
done
pass "troubleshooting documents provide exact diagnosis, recovery, provider, reload, and uninstall guidance"

printf 'PASS: governance documentation contracts passed\n'
