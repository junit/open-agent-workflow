#!/usr/bin/env bash

set -eu

fail() {
  printf 'Docker smoke: error: %s\n' "$*" >&2
  exit 1
}

skip() {
  printf 'SKIP: Docker Linux release smoke unavailable: %s\n' "$*" >&2
  exit 77
}

[ "$#" -eq 1 ] || fail "usage: scripts/smoke-docker.sh <absolute-linux-release-archive>"
ARCHIVE=$1
case "$ARCHIVE" in
  /*) ;;
  *) fail "release archive path must be absolute" ;;
esac
[ -f "$ARCHIVE" ] || fail "release archive does not exist: $ARCHIVE"
[ ! -L "$ARCHIVE" ] || fail "release archive must not be a symlink"

command -v docker >/dev/null 2>&1 || skip "Docker CLI not found"
set +e
DOCKER_ARCH=$(docker version --format '{{.Server.Arch}}' 2>/dev/null)
DOCKER_STATUS=$?
set -e
[ "$DOCKER_STATUS" -eq 0 ] || skip "Docker daemon not reachable"

case "$DOCKER_ARCH" in
  amd64)
    SMOKE_IMAGE=bash@sha256:534a5f1d11652aadaa9f08838f6637ac11a46a8b4b736a4cbf09c5945e38516f
    ;;
  arm64)
    SMOKE_IMAGE=bash@sha256:26b3d1c3d49066239fc1c44002f316c1893ca83f714c9fd9636e100d3e11224d
    ;;
  *) skip "unsupported Docker server architecture: $DOCKER_ARCH" ;;
esac
case "$ARCHIVE" in
  *_linux_${DOCKER_ARCH}.tar.gz) ;;
  *) fail "release archive does not match Docker server architecture: $DOCKER_ARCH" ;;
esac

if ! docker image inspect "$SMOKE_IMAGE" >/dev/null 2>&1; then
  docker pull --platform "linux/$DOCKER_ARCH" "$SMOKE_IMAGE" >/dev/null 2>&1 ||
    skip "pinned verification image could not be prepared"
fi

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
set +e
docker run --rm --network none --read-only \
  --tmpfs /tmp:rw,exec,nosuid,nodev \
  --cap-drop ALL --security-opt no-new-privileges \
  --platform "linux/$DOCKER_ARCH" \
  --mount "type=bind,src=$ARCHIVE,dst=/release.tar.gz,readonly" \
  --mount "type=bind,src=$SCRIPT_DIR/smoke-linux.sh,dst=/smoke-linux.sh,readonly" \
  "$SMOKE_IMAGE" bash /smoke-linux.sh /release.tar.gz
DOCKER_STATUS=$?
set -e

case "$DOCKER_STATUS" in
  0) printf 'PASS: Docker Linux release smoke verified on linux/%s\n' "$DOCKER_ARCH" ;;
  125)
    if ! docker version >/dev/null 2>&1; then
      skip "Docker executor became unavailable"
    fi
    exit 125
    ;;
  *) exit "$DOCKER_STATUS" ;;
esac
