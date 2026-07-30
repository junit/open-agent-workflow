#!/usr/bin/env bash

checksum_file() {
  local checksum_input=$1
  cksum <"$checksum_input" | awk '{ print $1 ":" $2 }'
}

files_equal() {
  [ -f "$1" ] && cmp -s "$1" "$2"
}
