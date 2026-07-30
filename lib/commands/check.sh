#!/usr/bin/env bash

command_check() {
  local oaw_version=
  local provider_name=
  local check_target=

  if [ ! -r "$OAW_SOURCE_DIR/VERSION" ]; then
    die "VERSION is not readable" 70
  fi

  IFS= read -r oaw_version <"$OAW_SOURCE_DIR/VERSION" || die "VERSION is invalid" 70
  if [ -z "$oaw_version" ]; then
    die "VERSION is invalid" 70
  fi

  printf 'version: %s\n' "$oaw_version"
  if [ "$OAW_SCOPE" = project ]; then
    printf 'scope: project (%s)\n' "$OAW_PROJECT_ROOT"
  else
    printf 'scope: user\n'
  fi
  printf 'targets: %s\n' "$OAW_SELECTED_TARGETS"

  for provider_name in superpowers matt ecc; do
    if detect_provider "$provider_name"; then
      printf 'provider %s: detected\n' "$provider_name"
    else
      printf 'provider %s: missing\n' "$provider_name"
    fi
  done

  for check_target in $(target_ids); do
    if selected_target "$check_target"; then
      printf 'target %s: ' "$check_target"
      target_readiness "$check_target"
    fi
  done
}
