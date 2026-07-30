#!/usr/bin/env bash

OAW_COMMAND=
OAW_TARGETS=
OAW_PROJECT=
OAW_DRY_RUN=0
OAW_FORCE=0
OAW_HELP=0

cli_error() {
  printf 'oaw: error: %s\n' "$*" >&2
  return 64
}

parse_cli() {
  target_seen=0
  project_seen=0
  dry_run_seen=0
  force_seen=0
  help_seen=0

  case "$1" in
    check|install|update|uninstall)
      OAW_COMMAND=$1
      shift
      ;;
    *)
      cli_error "unknown command '$1'"
      return 64
      ;;
  esac

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --target)
        if [ "$target_seen" -eq 1 ]; then
          cli_error "--target may be specified only once"
          return 64
        fi
        if [ "$#" -lt 2 ] || [ -z "$2" ]; then
          cli_error "--target requires a value"
          return 64
        fi
        case "$2" in
          -*)
            cli_error "--target requires a value"
            return 64
            ;;
        esac
        OAW_TARGETS=$2
        target_seen=1
        shift 2
        ;;
      --target=*)
        if [ "$target_seen" -eq 1 ]; then
          cli_error "--target may be specified only once"
          return 64
        fi
        OAW_TARGETS=${1#--target=}
        if [ -z "$OAW_TARGETS" ]; then
          cli_error "--target requires a value"
          return 64
        fi
        target_seen=1
        shift
        ;;
      --project)
        if [ "$project_seen" -eq 1 ]; then
          cli_error "--project may be specified only once"
          return 64
        fi
        if [ "$#" -lt 2 ] || [ -z "$2" ]; then
          cli_error "--project requires a value"
          return 64
        fi
        case "$2" in
          -*)
            cli_error "--project requires a value"
            return 64
            ;;
        esac
        OAW_PROJECT=$2
        project_seen=1
        shift 2
        ;;
      --project=*)
        if [ "$project_seen" -eq 1 ]; then
          cli_error "--project may be specified only once"
          return 64
        fi
        OAW_PROJECT=${1#--project=}
        if [ -z "$OAW_PROJECT" ]; then
          cli_error "--project requires a value"
          return 64
        fi
        project_seen=1
        shift
        ;;
      --dry-run)
        if [ "$dry_run_seen" -eq 1 ]; then
          cli_error "--dry-run may be specified only once"
          return 64
        fi
        OAW_DRY_RUN=1
        dry_run_seen=1
        shift
        ;;
      --force)
        if [ "$force_seen" -eq 1 ]; then
          cli_error "--force may be specified only once"
          return 64
        fi
        OAW_FORCE=1
        force_seen=1
        shift
        ;;
      -h|--help)
        if [ "$help_seen" -eq 1 ]; then
          cli_error "--help may be specified only once"
          return 64
        fi
        OAW_HELP=1
        help_seen=1
        shift
        ;;
      -*)
        cli_error "unknown option '$1'"
        return 64
        ;;
      *)
        cli_error "unexpected argument '$1'"
        return 64
        ;;
    esac
  done

  if [ "$OAW_COMMAND" = check ] && [ "$OAW_DRY_RUN" -eq 1 ]; then
    cli_error "--dry-run is not valid for check"
    return 64
  fi
  if [ "$OAW_COMMAND" = check ] && [ "$OAW_FORCE" -eq 1 ]; then
    cli_error "--force is not valid for check"
    return 64
  fi

  return 0
}
