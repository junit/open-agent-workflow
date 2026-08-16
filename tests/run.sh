#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$TEST_DIR/.." && pwd)
OAW_TEST_RELEASE=

cleanup() {
  if [ -n "$OAW_TEST_RELEASE" ] && [ -d "$OAW_TEST_RELEASE" ]; then
    rm -rf -- "$OAW_TEST_RELEASE"
  fi
}

trap cleanup EXIT HUP INT TERM

OAW_TEST_RELEASE=$(mktemp -d "${TMPDIR:-/tmp}/oaw-test-release.XXXXXX")
cp "$REPOSITORY/install.sh" "$OAW_TEST_RELEASE/install.sh"
chmod 755 "$OAW_TEST_RELEASE/install.sh"
(cd "$REPOSITORY" && go build -o "$OAW_TEST_RELEASE/oaw" ./cmd/oaw)
OAW_INSTALLER=$OAW_TEST_RELEASE/install.sh
export OAW_INSTALLER

for test_script in \
  01-cli-test.sh \
  02-check-test.sh \
  03-claude-installation-test.sh \
  04-user-adapters-test.sh \
  05-project-adapters-test.sh \
  05-policy-scope-test.sh \
  06-security-test.sh \
  07-containment-test.sh \
  08-static-product-test.sh \
  09-profile-inspection-test.sh \
  10-docs-test.sh \
  11-host-adapter-dogfood-test.sh \
  14-release-contract-test.sh \
  15-host-execution-boundary-test.sh \
  17-codex-bridge-management-test.sh \
  18-codex-bridge-protocol-test.sh \
  19-native-entrypoint-dogfood-test.sh; do
  bash "$TEST_DIR/$test_script"
done

printf 'PASS: all implemented installer cases passed\n'
