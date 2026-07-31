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

  grep -F "$expected_text" "$REPOSITORY/$relative_path" >/dev/null ||
    fail "$relative_path is missing required text: $expected_text"
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
pass "changelog describes the local unreleased 0.1.0 candidate"

assert_executable scripts/check-docs.sh
assert_contains scripts/check-docs.sh "README.md|README-zh.md"
assert_contains scripts/check-docs.sh "docs/en/background.md|docs/zh/background.md"
assert_contains scripts/check-docs.sh "docs/en/extending-adapters.md|docs/zh/extending-adapters.md"
assert_contains scripts/check-docs.sh "for command in check install update uninstall"
assert_contains scripts/check-docs.sh "experience-based"
assert_contains scripts/check-docs.sh "基于经验"
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
    CUSTOM-LOCKED; do
    assert_contains "$readme_file" "$profile_id"
  done
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
assert_contains README.md "Updates read artifacts only from the current checkout."
assert_contains README.md "Drift fails closed before mutation."
assert_contains README.md '`--force` backs up every affected artifact before mutation.'
assert_contains README.md "experience-based design inputs"
assert_contains README.md "Machine-readable status is reserved for a post-v0.1 extension."
assert_contains README.md "v0.1 output is human-readable only."
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
assert_contains README-zh.md "更新只从当前 checkout 读取构件。"
assert_contains README-zh.md "检测到 drift 时，会在变更前关闭失败。"
assert_contains README-zh.md '`--force` 会在变更前先备份所有受影响构件。'
assert_contains README-zh.md "基于经验的设计输入"
assert_contains README-zh.md "machine-readable status 保留为 post-v0.1 扩展。"
assert_contains README-zh.md "v0.1 只输出 human-readable 状态。"
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
    CUSTOM-LOCKED; do
    assert_contains "$lifecycle_file" "$profile_id"
  done
  assert_contains "$lifecycle_file" 'MATT-SP-HYBRID + ECC(security-review)'
done
for lifecycle_contract in \
  'ordinary task' \
  'complex task' \
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
  '普通任务' \
  '复杂任务' \
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
pass "lifecycle documents explain classification, locking, inheritance, add-ons, and switching"

printf 'PASS: governance documentation contracts passed\n'
