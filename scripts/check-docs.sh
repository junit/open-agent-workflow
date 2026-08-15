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
[ "$VERSION" = "0.1.0" ] || fail "unexpected fixed source version: $VERSION"
require_text CHANGELOG.md "## [0.1.0]"
require_text README.md "source baseline is fixed at v0.1.0"
require_text README-zh.md "源码基线固定为 v0.1.0"

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
  "docs/en/machine-assurance.md docs/zh/machine-assurance.md"; do
  set -- $pair
  require_file "$1"
  require_file "$2"
done

for path in \
  policy/POLICY.md \
  policy/cooperative-protocol.md \
  policy/adapters/codex-policy.md \
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
  internal/provideraudit \
  cmd/oaw-provider-audit \
  scripts/audit-provider-sources.sh \
  scripts/check-core-coordinator-coverage.sh \
  scripts/dogfood-current.sh \
  scripts/smoke-host-native.sh \
  tests/19-provider-source-audit-test.sh; do
  [ ! -e "$REPOSITORY/$retired" ] || fail "retired asset remains: $retired"
done

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
    "engineering manual"; do
    reject_text "$file" "$retired_text"
  done
done

for file in \
  README.md README-zh.md \
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

printf 'PASS: documentation and static Policy contracts passed\n'
