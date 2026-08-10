#!/usr/bin/env bash

set -eu

repo_dir=$(CDPATH='' cd -P -- "$(dirname -- "$0")/.." && pwd)
manifest="$repo_dir/internal/assets/audits/provider-sources-v4.json"

test -f "$manifest"
output=$(cd "$repo_dir" && go run ./cmd/oaw-provider-audit --validate --manifest "$manifest")
case "$output" in
  *"oaw.provider-source-audit/v1"*) ;;
  *) printf 'manifest validation omitted schema id: %s\n' "$output" >&2; exit 1 ;;
esac
for expected in \
  '84fdeffd12f2ee307994d1eb6feb48173b6e0502' \
  '44c9b2d6e889982ac18c27d05a19fefe335194e1' \
  '2d46e80e0925c7be0907f18c1812311ac212a6c5' \
  '49ec1819ab22364d763d0875d9af299ee332de3d6d39a7178a715c2b13272ccf' \
  'codex-grill-with-docs' 'claude-grill-with-docs' \
  'codex-brainstorming' 'claude-brainstorming' \
  'codex-intent-driven-development' 'claude-intent-driven-development' \
  'claude-architect' 'codex-explorer' 'codex-plan'; do
  grep -F "$expected" "$manifest" >/dev/null
done
if grep -E '"provider_id"[[:space:]]*:[[:space:]]*"oaw/[^" ]+"' "$manifest" | grep -vE 'oaw/(matt|superpowers|ecc)' >/dev/null; then
  printf 'manifest contains an unexpected Provider\n' >&2
  exit 1
fi
if grep -E 'tar[[:space:]].*-[^[:space:]]*x' "$repo_dir/scripts/audit-provider-sources.sh" >/dev/null; then
  printf 'source audit wrapper must not extract network archives with tar\n' >&2
  exit 1
fi
# shellcheck disable=SC2016 # Match the wrapper's literal shell variables.
grep -F 'fetch --depth 1 origin "$revision"' "$repo_dir/scripts/audit-provider-sources.sh" >/dev/null
# shellcheck disable=SC2016 # Match the wrapper's literal shell variables.
grep -F 'checkout --detach "$revision"' "$repo_dir/scripts/audit-provider-sources.sh" >/dev/null
printf 'PASS: Provider source audit manifest is valid and pinned\n'
