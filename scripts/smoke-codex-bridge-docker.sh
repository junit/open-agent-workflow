#!/usr/bin/env bash

set -eu

skip() {
  printf 'SKIP: Codex Bridge Docker smoke unavailable: %s\n' "$*" >&2
  exit 77
}

fail() {
  printf 'Codex Bridge Docker smoke: error: %s\n' "$*" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || skip 'Docker CLI unavailable'
docker version >/dev/null 2>&1 || skip 'Docker daemon unavailable'
command -v go >/dev/null 2>&1 || skip 'host Go toolchain unavailable for module-cache preparation'

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null) ||
  fail 'cannot resolve repository root'
[ -d "$REPOSITORY" ] || fail 'repository root is unavailable'

DOCKER_ARCH=$(docker version --format '{{.Server.Arch}}' 2>/dev/null) ||
  skip 'Docker daemon architecture unavailable'
case "$DOCKER_ARCH" in
  amd64)
    IMAGE=golang@sha256:111d79159b2326f7e80c4a4706e1ba166acb0e2611df853955f3621828cd49e8
    ;;
  arm64)
    IMAGE=golang@sha256:787328cefd7937073af18fc4b3a725f47e011ffdde9c2908239a25cae6b2f02b
    ;;
  *) skip "unsupported Docker server architecture: $DOCKER_ARCH" ;;
esac

(cd "$REPOSITORY" && go mod download) || fail 'cannot prepare the host Go module cache'
MODULE_CACHE=$(go env GOMODCACHE) || fail 'cannot resolve the host Go module cache'
[ -d "$MODULE_CACHE" ] || fail 'host Go module cache does not exist'

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  docker pull --platform "linux/$DOCKER_ARCH" "$IMAGE" >/dev/null 2>&1 ||
    skip 'pinned Go verification image could not be prepared'
fi

set +e
docker run --rm --network none --read-only \
  --cap-drop ALL --security-opt no-new-privileges \
  --platform "linux/$DOCKER_ARCH" \
  --tmpfs /tmp:rw,exec,nosuid,nodev,size=512m \
  -e CGO_ENABLED=0 \
  -e GOCACHE=/tmp/go-cache \
  -e GOMODCACHE=/go/pkg/mod \
  -e GOPROXY=off \
  --mount "type=bind,src=$MODULE_CACHE,dst=/go/pkg/mod,readonly" \
  --mount "type=bind,src=$REPOSITORY,dst=/src,readonly" \
  -w /src \
  "$IMAGE" sh -c 'go test -mod=readonly ./cmd/oaw-bridge ./internal/assurance ./internal/bridgecli ./internal/codexbridge/... ./internal/profileinspect ./internal/integration'
DOCKER_STATUS=$?
set -e

case "$DOCKER_STATUS" in
  0) printf 'PASS: Codex Bridge Docker smoke passed on linux/%s\n' "$DOCKER_ARCH" ;;
  125)
    if ! docker version >/dev/null 2>&1; then
      skip 'Docker executor became unavailable'
    fi
    exit 125
    ;;
  *) exit "$DOCKER_STATUS" ;;
esac
