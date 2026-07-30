#!/usr/bin/env bash

marker_count() {
  local marker_file=$1
  local marker_text=$2
  awk -v marker="$marker_text" '$0 == marker { count++ } END { print count + 0 }' "$marker_file"
}

marker_line() {
  local marker_file=$1
  local marker_text=$2
  awk -v marker="$marker_text" '$0 == marker { print NR; exit }' "$marker_file"
}

managed_block_status() {
  local managed_file=$1
  local begin_count=0
  local end_count=0
  local begin_line=0
  local end_line=0

  if [ ! -e "$managed_file" ]; then
    printf 'absent\n'
    return 0
  fi
  if [ ! -f "$managed_file" ]; then
    printf 'drift\n'
    return 0
  fi

  begin_count=$(marker_count "$managed_file" "$OAW_BEGIN_MARKER")
  end_count=$(marker_count "$managed_file" "$OAW_END_MARKER")
  if [ "$begin_count" -eq 0 ] && [ "$end_count" -eq 0 ]; then
    printf 'absent\n'
    return 0
  fi
  if [ "$begin_count" -ne 1 ] || [ "$end_count" -ne 1 ]; then
    printf 'drift\n'
    return 0
  fi

  begin_line=$(marker_line "$managed_file" "$OAW_BEGIN_MARKER")
  end_line=$(marker_line "$managed_file" "$OAW_END_MARKER")
  if [ "$begin_line" -ge "$end_line" ]; then
    printf 'drift\n'
  else
    printf 'present\n'
  fi
}

file_has_trailing_newline() {
  local newline_file=$1
  local final_byte=

  [ -s "$newline_file" ] || return 0
  final_byte=$(tail -c 1 "$newline_file" | od -An -t u1 | tr -d '[:space:]')
  [ "$final_byte" = 10 ]
}

render_file_with_block() {
  local current_file=$1
  local block_file=$2
  local rendered_file=$3
  local current_status=
  local trailing_newline=0

  current_status=$(managed_block_status "$current_file")
  case "$current_status" in
    absent)
      : >"$rendered_file"
      if [ -s "$current_file" ] && ! file_has_trailing_newline "$current_file"; then
        cat "$block_file" >>"$rendered_file"
        cat "$current_file" >>"$rendered_file"
      else
        if [ -f "$current_file" ]; then
          cp "$current_file" "$rendered_file"
        fi
        cat "$block_file" >>"$rendered_file"
      fi
      ;;
    present)
      if file_has_trailing_newline "$current_file"; then
        trailing_newline=1
      fi
      awk -v begin="$OAW_BEGIN_MARKER" '$0 == begin { exit } { print }' "$current_file" >"$rendered_file"
      cat "$block_file" >>"$rendered_file"
      awk -v end="$OAW_END_MARKER" -v trailing_newline="$trailing_newline" '
        $0 == end { found = 1; next }
        found {
          if (have_previous) {
            print previous
          }
          previous = $0
          have_previous = 1
        }
        END {
          if (have_previous) {
            printf "%s", previous
            if (trailing_newline) {
              printf "\n"
            }
          }
        }
      ' "$current_file" >>"$rendered_file"
      ;;
    drift)
      die "managed markers are invalid: $current_file" 65
      ;;
  esac
}

extract_managed_block() {
  local managed_file=$1
  local extracted_file=$2

  awk -v begin="$OAW_BEGIN_MARKER" -v end="$OAW_END_MARKER" '
    $0 == begin { copying = 1 }
    copying { print }
    $0 == end && copying { exit }
  ' "$managed_file" >"$extracted_file"
}

render_file_without_block() {
  local managed_file=$1
  local rendered_file=$2
  local trailing_newline=0

  if file_has_trailing_newline "$managed_file"; then
    trailing_newline=1
  fi

  awk -v begin="$OAW_BEGIN_MARKER" -v end="$OAW_END_MARKER" \
    -v trailing_newline="$trailing_newline" '
    $0 == begin { skipping = 1; next }
    $0 == end && skipping { skipping = 0; next }
    !skipping {
      if (have_previous) {
        print previous
      }
      previous = $0
      have_previous = 1
    }
    END {
      if (have_previous) {
        printf "%s", previous
        if (trailing_newline) {
          printf "\n"
        }
      }
    }
  ' "$managed_file" >"$rendered_file"
}
