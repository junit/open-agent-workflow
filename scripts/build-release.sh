#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)
RELEASE_TEMP=

cleanup() {
  if [ -n "$RELEASE_TEMP" ] && [ -d "$RELEASE_TEMP" ]; then
    rm -rf -- "$RELEASE_TEMP"
  fi
}

fail() {
  printf 'release: error: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT HUP INT TERM

[ "$#" -le 1 ] || fail "usage: scripts/build-release.sh [output-directory]"
OUTPUT=${1:-$REPOSITORY/dist}
case "$OUTPUT" in
  /*) ;;
  *) OUTPUT=$PWD/$OUTPUT ;;
esac
[ ! -L "$OUTPUT" ] || fail "output directory must not be a symlink: $OUTPUT"
mkdir -p "$OUTPUT" || fail "cannot create output directory: $OUTPUT"
OUTPUT=$(CDPATH='' cd -P -- "$OUTPUT" && pwd)

[ -f "$REPOSITORY/VERSION" ] || fail "VERSION is missing"
[ "$(wc -l <"$REPOSITORY/VERSION" | tr -d ' ')" -eq 1 ] ||
  fail "VERSION must contain exactly one line"
VERSION=$(sed -n '1{s/\r$//;p;}' "$REPOSITORY/VERSION")
printf '%s\n' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.]+)?$' ||
  fail "VERSION is invalid"

TARGETS='darwin amd64
darwin arm64
linux amd64
linux arm64
windows amd64
windows arm64'

printf '%s\n' "$TARGETS" | while IFS=' ' read -r target_os target_arch; do
  package=open-agent-workflow_${VERSION}_${target_os}_${target_arch}
  [ ! -e "$OUTPUT/$package.tar.gz" ] ||
    fail "release artifact already exists: $OUTPUT/$package.tar.gz"
done
[ ! -e "$OUTPUT/SHA256SUMS" ] ||
  fail "release artifact already exists: $OUTPUT/SHA256SUMS"

command -v go >/dev/null 2>&1 || fail "go is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"
if command -v shasum >/dev/null 2>&1; then
  SHA_TOOL=shasum
elif command -v sha256sum >/dev/null 2>&1; then
  SHA_TOOL=sha256sum
else
  fail "shasum or sha256sum is required"
fi

RELEASE_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-release.XXXXXX") ||
  fail "cannot create staging directory"
STAGED_OUTPUT=$RELEASE_TEMP/output
mkdir -p "$STAGED_OUTPUT"

printf '%s\n' "$TARGETS" | while IFS=' ' read -r target_os target_arch; do
  package=open-agent-workflow_${VERSION}_${target_os}_${target_arch}
  package_root=$RELEASE_TEMP/$package
  binary_name=oaw
  if [ "$target_os" = windows ]; then
    binary_name=oaw.exe
  fi
  mkdir -p "$package_root"
  (
    cd "$REPOSITORY"
    CGO_ENABLED=0 GOOS=$target_os GOARCH=$target_arch \
      go build -trimpath -ldflags '-s -w' -o "$package_root/$binary_name" ./cmd/oaw
  )
  for release_file in CHANGELOG.md LICENSE README.md README-zh.md VERSION install.sh; do
    cp "$REPOSITORY/$release_file" "$package_root/$release_file"
  done
  chmod 755 "$package_root/$binary_name" "$package_root/install.sh"
  chmod 644 "$package_root/CHANGELOG.md" "$package_root/LICENSE" \
    "$package_root/README.md" "$package_root/README-zh.md" "$package_root/VERSION"
  COPYFILE_DISABLE=1 tar -C "$RELEASE_TEMP" -czf "$STAGED_OUTPUT/$package.tar.gz" "$package"
done

: >"$STAGED_OUTPUT/SHA256SUMS"
printf '%s\n' "$TARGETS" | while IFS=' ' read -r target_os target_arch; do
  package=open-agent-workflow_${VERSION}_${target_os}_${target_arch}
  archive=$STAGED_OUTPUT/$package.tar.gz
  if [ "$SHA_TOOL" = shasum ]; then
    digest=$(shasum -a 256 "$archive" | awk '{ print $1 }')
  else
    digest=$(sha256sum "$archive" | awk '{ print $1 }')
  fi
  printf '%s  %s\n' "$digest" "$package.tar.gz" >>"$STAGED_OUTPUT/SHA256SUMS"
done

printf '%s\n' "$TARGETS" | while IFS=' ' read -r target_os target_arch; do
  package=open-agent-workflow_${VERSION}_${target_os}_${target_arch}
  mv "$STAGED_OUTPUT/$package.tar.gz" "$OUTPUT/$package.tar.gz"
  printf 'release: created: %s\n' "$OUTPUT/$package.tar.gz"
done
mv "$STAGED_OUTPUT/SHA256SUMS" "$OUTPUT/SHA256SUMS"
printf 'release: created: %s\n' "$OUTPUT/SHA256SUMS"
