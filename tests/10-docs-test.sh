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

printf 'PASS: governance documentation contracts passed\n'
