#!/usr/bin/env bash

set -eu

repository=$(CDPATH='' cd -P -- "$(dirname -- "$0")/.." && pwd)
mode=${1:-}
target=${2:-}
if [ "$mode" != "--write" ] && [ "$mode" != "--check" ]; then
  printf 'usage: %s --write|--check MANIFEST [--matt-root DIR --superpowers-root DIR --openai-plugins-root DIR --ecc-root DIR]\n' "$0" >&2
  exit 64
fi
if [ -z "$target" ]; then
  printf 'manifest path is required\n' >&2
  exit 64
fi
shift 2

matt_root=
superpowers_root=
openai_plugins_root=
ecc_root=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --matt-root) matt_root=${2:-}; shift 2 ;;
    --superpowers-root) superpowers_root=${2:-}; shift 2 ;;
    --openai-plugins-root) openai_plugins_root=${2:-}; shift 2 ;;
    --ecc-root) ecc_root=${2:-}; shift 2 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; exit 64 ;;
  esac
done

temporary=
cleanup() {
  if [ -n "$temporary" ] && [ -d "$temporary" ]; then
    rm -rf -- "$temporary"
  fi
}
trap cleanup EXIT HUP INT TERM

if [ -n "$matt_root$superpowers_root$openai_plugins_root$ecc_root" ]; then
  if [ -z "$matt_root" ] || [ -z "$superpowers_root" ] || [ -z "$openai_plugins_root" ] || [ -z "$ecc_root" ]; then
    printf 'all four Distribution roots are required\n' >&2
    exit 64
  fi
else
  temporary=$(mktemp -d "${TMPDIR:-/tmp}/oaw-provider-audit.XXXXXX")
  checkout_pinned() {
    source_uri=$1
    revision=$2
    destination=$3
    mkdir -p "$destination"
    git -C "$destination" init -q
    git -C "$destination" remote add origin "$source_uri"
    if ! git -C "$destination" fetch --depth 1 origin "$revision"; then
      if [ "$mode" = "--check" ]; then
        exit 77
      fi
      exit 1
    fi
    git -C "$destination" -c advice.detachedHead=false checkout --detach "$revision"
  }
  matt_root="$temporary/matt"
  superpowers_root="$temporary/superpowers"
  openai_plugins_root="$temporary/openai-plugins"
  ecc_root="$temporary/ecc"
  checkout_pinned https://github.com/mattpocock/skills 84fdeffd12f2ee307994d1eb6feb48173b6e0502 "$matt_root"
  checkout_pinned https://github.com/obra/superpowers 44c9b2d6e889982ac18c27d05a19fefe335194e1 "$superpowers_root"
  checkout_pinned https://github.com/openai/plugins 11c74d6ba24d3a6d48f54a194cd00ef3beea18f9 "$openai_plugins_root"
  checkout_pinned https://github.com/affaan-m/ECC 2d46e80e0925c7be0907f18c1812311ac212a6c5 "$ecc_root"
fi

if [ "$mode" = "--write" ]; then
  (cd "$repository" && go run ./cmd/oaw-provider-audit --write --output "$target" --matt-root "$matt_root" --superpowers-root "$superpowers_root" --openai-plugins-root "$openai_plugins_root" --ecc-root "$ecc_root")
else
  (cd "$repository" && go run ./cmd/oaw-provider-audit --check --manifest "$target" --matt-root "$matt_root" --superpowers-root "$superpowers_root" --openai-plugins-root "$openai_plugins_root" --ecc-root "$ecc_root")
fi
