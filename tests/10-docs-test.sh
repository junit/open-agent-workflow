#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

require_file() {
  [ -f "$REPOSITORY/$1" ] || fail "missing documentation contract: $1"
}

bash "$REPOSITORY/scripts/check-docs.sh"

for path in \
  docs/adr/README.md \
  docs/adr/0001-static-policy-product.md \
  docs/adr/0002-optional-machine-evidence.md \
  policy/POLICY.md policy/cooperative-protocol.md \
  policy/adapters/codex-policy.md \
  policy/profiles/SP-FULL.md policy/profiles/MATT-FULL.md \
  policy/profiles/ECC-FULL.md policy/profiles/MATT-SP-HYBRID.md; do
  require_file "$path"
done

[ ! -e "$REPOSITORY/policy/ENGINEERING.md" ] ||
  fail "legacy single Policy source remains"
[ ! -d "$REPOSITORY/internal/assets/schemas" ] ||
  fail "legacy machine schema tree remains"
[ ! -d "$REPOSITORY/internal/assets/audits" ] ||
  fail "legacy Provider source-audit fixtures remain"
[ ! -d "$REPOSITORY/internal/provideraudit" ] ||
  fail "legacy Provider source-audit package remains"
[ ! -d "$REPOSITORY/internal/check" ] ||
  fail "legacy check facade remains"
[ ! -d "$REPOSITORY/docs/superpowers" ] ||
  fail "obsolete implementation plans remain"
[ -z "$(git -C "$REPOSITORY" ls-files -- .scratch)" ] ||
  fail "tracked scratch data remains"

for document in README.md README-zh.md docs/en/*.md docs/zh/*.md; do
  if grep -E '(^|[^[:alnum:]_])oaw (workflow|run|runtime)([^[:alnum:]_-]|$)' \
    "$REPOSITORY/$document" >/dev/null; then
    fail "current documentation exposes retired runtime command: $document"
  fi
done

printf 'PASS: current documentation contracts passed\n'
