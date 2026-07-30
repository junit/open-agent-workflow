#!/usr/bin/env bash

command_mutate() {
  case "$OAW_COMMAND" in
    install) operation_install ;;
    update) operation_update ;;
    uninstall) operation_uninstall ;;
  esac
}
