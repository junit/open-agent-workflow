#!/usr/bin/env bash

command_mutate() {
  case "$OAW_COMMAND" in
    install) operation_install ;;
    update|uninstall) die "command not implemented: $OAW_COMMAND" 69 ;;
  esac
}
