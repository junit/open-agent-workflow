#!/usr/bin/env bash

command_mutate() {
  case "$OAW_COMMAND" in
    install) operation_install ;;
    update) operation_update ;;
    uninstall) die "command not implemented: uninstall" 69 ;;
  esac
}
