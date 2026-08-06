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
  03-claude-lifecycle-test.sh \
  04-core-adapters-test.sh \
  05-project-adapters-test.sh \
  05-policy-coordination-test.sh \
  06-security-test.sh \
  07-containment-test.sh \
  08-backup-test.sh \
  09-transaction-test.sh \
  10-docs-test.sh \
  11-check-parity-test.sh \
  12-install-parity-test.sh \
  13-mutation-parity-test.sh \
  14-cutover-release-test.sh \
  15-host-execution-boundary-test.sh \
  16-core-coordinator-conformance-test.sh; do
  bash "$TEST_DIR/$test_script"
done

printf 'PASS: all implemented installer cases passed\n'
