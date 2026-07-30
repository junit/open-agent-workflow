#!/usr/bin/env bash

set -eu

TEST_DIR=$(CDPATH= cd -P -- "$(dirname -- "$0")" && pwd)

bash "$TEST_DIR/01-cli-test.sh"
