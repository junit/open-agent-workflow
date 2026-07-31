#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)

for test_script in \
  01-cli-test.sh \
  02-check-test.sh \
  03-claude-lifecycle-test.sh \
  04-core-adapters-test.sh \
  05-project-adapters-test.sh \
  05-policy-coordination-test.sh \
  06-security-test.sh; do
  bash "$TEST_DIR/$test_script"
done

printf 'PASS: all implemented installer cases passed\n'
