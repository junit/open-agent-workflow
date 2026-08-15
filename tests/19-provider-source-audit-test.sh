#!/usr/bin/env bash

set -eu

repo_dir=$(CDPATH='' cd -P -- "$(dirname -- "$0")/.." && pwd)
manifest="$repo_dir/internal/assets/audits/provider-sources-v5.json"

test -f "$manifest"
output=$(cd "$repo_dir" && go run ./cmd/oaw-provider-audit --validate --manifest "$manifest")
case "$output" in
  *"oaw.provider-source-audit/v2"*) ;;
  *) printf 'manifest validation omitted schema id: %s\n' "$output" >&2; exit 1 ;;
esac
for expected in \
  '84fdeffd12f2ee307994d1eb6feb48173b6e0502' \
  '44c9b2d6e889982ac18c27d05a19fefe335194e1' \
  '11c74d6ba24d3a6d48f54a194cd00ef3beea18f9' \
  '2d46e80e0925c7be0907f18c1812311ac212a6c5' \
  '"distribution_id":"superpowers-codex"' \
  '"distribution_root":"plugins/superpowers"' \
  'codex-grill-with-docs' 'claude-grill-with-docs' \
  'codex-brainstorming' 'codex-upstream-brainstorming' 'claude-brainstorming' \
  'superpowers:brainstorming' \
  'codex-intent-driven-development' 'claude-intent-driven-development' \
  'ecc:intent-driven-development' '"install_root":"skills/grill-with-docs"' \
  'claude-architect' 'codex-explorer' 'codex-plan'; do
  grep -F "$expected" "$manifest" >/dev/null
done
for stem in \
  brainstorming writing-plans using-git-worktrees subagent-driven-development \
  executing-plans test-driven-development systematic-debugging requesting-code-review \
  receiving-code-review verification-before-completion finishing-a-development-branch; do
  grep -F "codex-$stem" "$manifest" >/dev/null
  grep -F "codex-upstream-$stem" "$manifest" >/dev/null
  grep -F "claude-$stem" "$manifest" >/dev/null
  grep -F "superpowers:$stem" "$manifest" >/dev/null
done
provider_entries=$(grep -oE '"provider_id"[[:space:]]*:[[:space:]]*"[^" ]+"' "$manifest" | wc -l | awk '{print $1}')
provider_ids=$(grep -oE '"provider_id"[[:space:]]*:[[:space:]]*"[^" ]+"' "$manifest" | sort -u | wc -l | awk '{print $1}')
distribution_entries=$(grep -oE '"distribution_id"[[:space:]]*:[[:space:]]*"[^" ]+"' "$manifest" | wc -l | awk '{print $1}')
if [ "$provider_entries" -ne 4 ] || [ "$provider_ids" -ne 3 ] || [ "$distribution_entries" -ne 4 ]; then
  printf 'manifest must contain exactly three Providers across four Distributions\n' >&2
  exit 1
fi
if grep -E '"provider_id"[[:space:]]*:[[:space:]]*"oaw/[^" ]+"' "$manifest" | grep -vE 'oaw/(matt|superpowers|ecc)' >/dev/null; then
  printf 'manifest contains an unexpected Provider\n' >&2
  exit 1
fi
if grep -E '"distribution_id"[[:space:]]*:[[:space:]]*"[^" ]+"' "$manifest" | grep -vE 'matt-skills|superpowers(-codex)?|ecc' >/dev/null; then
  printf 'manifest contains an unexpected Distribution\n' >&2
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
grep -F 'https://github.com/openai/plugins 11c74d6ba24d3a6d48f54a194cd00ef3beea18f9' "$repo_dir/scripts/audit-provider-sources.sh" >/dev/null
grep -F -- '--openai-plugins-root "$openai_plugins_root"' "$repo_dir/scripts/audit-provider-sources.sh" >/dev/null
printf 'PASS: Provider source audit manifest is valid and pinned\n'
