#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)

fail() {
  printf 'docs: error: %s\n' "$*" >&2
  exit 1
}

require_file() {
  [ -f "$REPOSITORY/$1" ] || fail "missing required file: $1"
}

require_text() {
  file=$1
  text=$2
  grep -F -- "$text" "$REPOSITORY/$file" >/dev/null ||
    fail "$file omits required text: $text"
}

reject_text() {
  file=$1
  text=$2
  if grep -F -- "$text" "$REPOSITORY/$file" >/dev/null; then
    fail "$file contains retired current-product text: $text"
  fi
}

VERSION=$(sed -n '1{s/\r$//;p;}' "$REPOSITORY/VERSION")
printf '%s\n' "$VERSION" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' >/dev/null ||
  fail "VERSION is not a release version: $VERSION"
require_text CHANGELOG.md "## [$VERSION]"
require_text README.md 'The source version is recorded in `VERSION`.'
require_text README-zh.md '源码版本记录在 `VERSION` 中。'

for pair in \
  "README.md README-zh.md" \
  "CONTRIBUTING.md CONTRIBUTING-zh.md" \
  "SECURITY.md SECURITY-zh.md" \
  "docs/en/architecture.md docs/zh/architecture.md" \
  "docs/en/background.md docs/zh/background.md" \
  "docs/en/comparison.md docs/zh/comparison.md" \
  "docs/en/lifecycle.md docs/zh/lifecycle.md" \
  "docs/en/installer.md docs/zh/installer.md" \
  "docs/en/adapters.md docs/zh/adapters.md" \
  "docs/en/extending-adapters.md docs/zh/extending-adapters.md" \
  "docs/en/troubleshooting.md docs/zh/troubleshooting.md" \
  "docs/en/security.md docs/zh/security.md" \
  "docs/en/codex-bridge.md docs/zh/codex-bridge.md" \
  "docs/en/machine-assurance.md docs/zh/machine-assurance.md" \
  "docs/en/releasing.md docs/zh/releasing.md"; do
  set -- $pair
  require_file "$1"
  require_file "$2"
done

for path in \
  docs/adr/README.md \
  docs/adr/0001-static-policy-product.md \
  docs/adr/0002-optional-machine-evidence.md \
  policy/POLICY.md \
  policy/cooperative-protocol.md \
  policy/adapters/claude-policy.md \
  policy/adapters/codex-policy.md \
  policy/adapters/gemini-policy.md \
  policy/adapters/opencode-policy.md \
  policy/adapters/cursor-policy.md \
  policy/adapters/windsurf-policy.md \
  policy/adapters/cline-policy.md \
  policy/adapters/roo-policy.md \
  policy/adapters/copilot-policy.md \
  policy/profiles/SP-FULL.md \
  policy/profiles/MATT-FULL.md \
  policy/profiles/ECC-FULL.md \
  policy/profiles/MATT-SP-HYBRID.md; do
  require_file "$path"
done

for retired in \
  policy/ENGINEERING.md \
  internal/assets/audits \
  internal/assets/schemas \
  internal/check \
  internal/provideraudit \
  cmd/oaw-provider-audit \
  scripts/audit-provider-sources.sh \
  scripts/check-core-coordinator-coverage.sh \
  scripts/dogfood-current.sh \
  scripts/smoke-host-native.sh \
  tests/19-provider-source-audit-test.sh \
  docs/superpowers \
  docs/adr/0001-provider-neutral-arbitration-layer.md \
  docs/adr/0002-xdg-canonical-rule-source.md \
  docs/adr/0003-add-optional-capability-admission-runtime.md \
  docs/adr/0004-implement-runtime-core-in-go.md \
  docs/adr/0005-codex-read-only-mcp-containment.md \
  docs/adr/0006-separate-codex-discovery-and-execution-profiles.md \
  docs/adr/0007-use-host-native-execution-topologies.md \
  docs/adr/0008-treat-subagent-environment-as-host-owned.md \
  docs/adr/0009-separate-core-coordination-and-host-execution.md \
  docs/adr/0010-policy-first-with-optional-machine-assurance.md \
  docs/adr/0011-static-policy-profiles-as-the-product-core.md \
  docs/adr/0012-retain-only-profile-binding-machine-assurance.md; do
  [ ! -e "$REPOSITORY/$retired" ] || fail "retired asset remains: $retired"
done

if command -v git >/dev/null 2>&1 &&
  git -C "$REPOSITORY" rev-parse --is-inside-work-tree >/dev/null 2>&1 &&
  [ -n "$(git -C "$REPOSITORY" ls-files -- .scratch)" ]; then
  fail "tracked scratch data remains"
fi

for file in \
  README.md README-zh.md CONTRIBUTING.md CONTRIBUTING-zh.md \
  SECURITY.md SECURITY-zh.md \
  docs/en/*.md docs/zh/*.md; do
  for retired_text in \
    "policy/ENGINEERING.md" \
    "Profile Recipe" \
    "USER-DEFINED" \
    "Request Mode" \
    "oaw workflow" \
    "oaw run" \
    "oaw runtime" \
    "Policy Plane" \
    "Observed Route" \
    "engineering manual"; do
    reject_text "$file" "$retired_text"
  done
done

for file in \
  README.md README-zh.md \
  CONTRIBUTING.md CONTRIBUTING-zh.md \
  docs/en/architecture.md docs/zh/architecture.md \
  docs/en/lifecycle.md docs/zh/lifecycle.md \
  docs/en/extending-adapters.md docs/zh/extending-adapters.md; do
  while IFS= read -r link; do
		target=${link%%#*}
    case "$target" in
      ""|http://*|https://*|mailto:*) continue ;;
    esac
    base=$(dirname -- "$REPOSITORY/$file")
    [ -e "$base/$target" ] || fail "$file has unresolved local link: $target"
  done < <(grep -oE '\]\([^)]+' "$REPOSITORY/$file" | sed 's/^](//')
done

for dependency in \
  github.com/BurntSushi/toml \
  github.com/gofrs/flock \
  github.com/santhosh-tekuri/jsonschema/v6; do
  if grep -F -- "$dependency" "$REPOSITORY/go.mod" >/dev/null; then
    fail "retired direct dependency remains: $dependency"
  fi
done

require_text docs/en/architecture.md "static"
require_text docs/zh/architecture.md "静态"
require_text docs/en/lifecycle.md "Switch this deliverable to SP-FULL"
require_text docs/zh/lifecycle.md "切换只改变剩余"
require_text docs/en/lifecycle.md "Use the Policy verification"
require_text docs/zh/lifecycle.md "fallback"
require_text docs/en/extending-adapters.md "Custom"
require_text docs/zh/extending-adapters.md "Custom"
require_text docs/en/codex-bridge.md "optional"
require_text docs/zh/codex-bridge.md "可选"
require_text docs/en/adapters.md "OpenCode, Cursor, Windsurf, Cline, Roo, and Copilot"
require_text docs/zh/adapters.md "Claude、Codex、Gemini、OpenCode、Cursor、Windsurf、Cline、Roo 和 Copilot"
require_text docs/en/releasing.md 'six platform archives'
require_text docs/zh/releasing.md '六个平台归档'
require_text docs/en/releasing.md 'oaw-bridge'
require_text docs/zh/releasing.md 'oaw-bridge'

printf 'PASS: documentation and static Policy contracts passed\n'
