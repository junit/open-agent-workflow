#!/usr/bin/env bash

die() {
  local oaw_die_message=$1
  local oaw_die_status=${2:-1}
  printf 'oaw: error: %s\n' "$oaw_die_message" >&2
  exit "$oaw_die_status"
}

note() {
  printf 'oaw: %s\n' "$*"
}

warn() {
  printf 'oaw: warning: %s\n' "$*" >&2
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_absolute_root() {
  case "$1" in
    /*) return 0 ;;
    *) die "root must be an absolute path: $1" 64 ;;
  esac
}
