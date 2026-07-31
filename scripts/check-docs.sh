#!/usr/bin/env bash

set -eu

SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
REPOSITORY=$(CDPATH='' cd -P -- "$SCRIPT_DIR/.." && pwd)
CHECK_TEMP=

cleanup() {
  if [ -n "$CHECK_TEMP" ] && [ -d "$CHECK_TEMP" ]; then
    rm -rf -- "$CHECK_TEMP"
  fi
}

fail() {
  printf 'docs: error: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT HUP INT TERM

CHECK_TEMP=$(mktemp -d "${TMPDIR:-/tmp}/oaw-docs.XXXXXX") ||
  fail "cannot create temporary directory"

PAIRED_DOCUMENTS='
README.md|README-zh.md
CONTRIBUTING.md|CONTRIBUTING-zh.md
SECURITY.md|SECURITY-zh.md
docs/en/background.md|docs/zh/background.md
docs/en/comparison.md|docs/zh/comparison.md
docs/en/lifecycle.md|docs/zh/lifecycle.md
docs/en/architecture.md|docs/zh/architecture.md
docs/en/installer.md|docs/zh/installer.md
docs/en/adapters.md|docs/zh/adapters.md
docs/en/extending-adapters.md|docs/zh/extending-adapters.md
docs/en/security.md|docs/zh/security.md
docs/en/troubleshooting.md|docs/zh/troubleshooting.md
'

printf '%s\n' "$PAIRED_DOCUMENTS" >"$CHECK_TEMP/document-pairs"
while IFS='|' read -r english_file chinese_file; do
  [ -n "$english_file$chinese_file" ] || continue
  [ -f "$REPOSITORY/$english_file" ] ||
    fail "missing paired document: $english_file"
  [ -f "$REPOSITORY/$chinese_file" ] ||
    fail "missing paired document: $chinese_file"
done <"$CHECK_TEMP/document-pairs"

for command in check install update uninstall; do
  for readme in README.md README-zh.md; do
    grep -F "./install.sh $command" "$REPOSITORY/$readme" >/dev/null ||
      fail "$readme omits public command: $command"
  done
done

grep -F "experience-based" "$REPOSITORY/docs/en/comparison.md" >/dev/null ||
  fail "English comparison omits its experience-based caveat"
grep -F "基于经验" "$REPOSITORY/docs/zh/comparison.md" >/dev/null ||
  fail "Chinese comparison omits its experience-based caveat"

find "$REPOSITORY" -type f -name '*.md' \
  ! -path "$REPOSITORY/.git/*" \
  ! -path "$REPOSITORY/.serena/*" \
  ! -path "$REPOSITORY/.worktrees/*" \
  -print >"$CHECK_TEMP/markdown-files"

while IFS= read -r markdown_file; do
  # Parse destinations separately from optional titles. This covers inline and
  # reference-style links without requiring a non-standard Markdown runtime.
  awk '
    function emit_destination(raw,    character, cursor, escaped, result) {
      result = ""
      escaped = 0
      for (cursor = 1; cursor <= length(raw); cursor++) {
        character = substr(raw, cursor, 1)
        if (escaped) {
          result = result character
          escaped = 0
        } else if (character == "\\") {
          escaped = 1
        } else {
          result = result character
        }
      }
      if (escaped) {
        result = result "\\"
      }
      if (result != "") {
        print result
      }
    }

    function parse_destination(line, start, inline,    character, cursor, depth, escaped, raw) {
      cursor = start
      while (substr(line, cursor, 1) == " " || substr(line, cursor, 1) == "\t") {
        cursor++
      }

      if (substr(line, cursor, 1) == "<") {
        cursor++
        raw = ""
        escaped = 0
        while (cursor <= length(line)) {
          character = substr(line, cursor, 1)
          if (character == ">" && !escaped) {
            emit_destination(raw)
            return cursor + 1
          }
          raw = raw character
          if (character == "\\" && !escaped) {
            escaped = 1
          } else {
            escaped = 0
          }
          cursor++
        }
        return start
      }

      raw = ""
      depth = 0
      escaped = 0
      while (cursor <= length(line)) {
        character = substr(line, cursor, 1)
        if (escaped) {
          raw = raw character
          escaped = 0
        } else if (character == "\\") {
          raw = raw character
          escaped = 1
        } else if (character == "(") {
          depth++
          raw = raw character
        } else if (character == ")") {
          if (inline && depth == 0) {
            break
          }
          if (depth > 0) {
            depth--
          }
          raw = raw character
        } else if ((character == " " || character == "\t") && depth == 0) {
          break
        } else {
          raw = raw character
        }
        cursor++
      }
      emit_destination(raw)
      return cursor
    }

    function parse_inline_links(line,    marker, offset, start) {
      offset = 1
      while (offset <= length(line)) {
        marker = index(substr(line, offset), "](")
        if (!marker) {
          return
        }
        start = offset + marker + 1
        parse_destination(line, start, 1)
        offset = start + 1
      }
    }

    function normalize_reference(label) {
      gsub(/[ \t]+/, " ", label)
      sub(/^ /, "", label)
      sub(/ $/, "", label)
      return tolower(label)
    }

    function reference_opening(line, before,    cursor) {
      for (cursor = before; cursor >= 1; cursor--) {
        if (substr(line, cursor, 1) == "[" &&
            (cursor == 1 || substr(line, cursor - 1, 1) != "\\")) {
          return cursor
        }
      }
      return 0
    }

    function parse_reference_usages(line,    closing, label, marker, normalized, offset, opening) {
      offset = 1
      while (offset <= length(line)) {
        marker = index(substr(line, offset), "][")
        if (!marker) {
          return
        }
        marker = offset + marker - 1
        opening = reference_opening(line, marker - 1)
        closing = index(substr(line, marker + 2), "]")
        if (!opening || !closing) {
          offset = marker + 2
          continue
        }

        label = substr(line, marker + 2, closing - 1)
        if (label == "") {
          label = substr(line, opening + 1, marker - opening - 1)
        }
        normalized = normalize_reference(label)
        if (normalized != "") {
          reference_uses[normalized] = label
        }
        offset = marker + closing + 2
      }
    }

    function parse_reference_definition(line,    closing, content, label, leading, normalized) {
      leading = 0
      while (leading < 4 && substr(line, leading + 1, 1) == " ") {
        leading++
      }
      if (leading > 3) {
        return 0
      }
      content = substr(line, leading + 1)
      if (substr(content, 1, 1) != "[") {
        return 0
      }
      closing = index(content, "]:")
      if (closing < 2) {
        return 0
      }
      label = substr(content, 2, closing - 2)
      normalized = normalize_reference(label)
      if (normalized != "") {
        reference_definitions[normalized] = 1
      }
      parse_destination(content, closing + 2, 0)
      return 1
    }

    {
      content = $0
      leading = 0
      while (leading < 4 && substr(content, leading + 1, 1) == " ") {
        leading++
      }
      fence_probe = substr(content, leading + 1, 3)
      if (!in_fence && (fence_probe == "```" || fence_probe == "~~~")) {
        in_fence = 1
        fence_marker = fence_probe
        next
      }
      if (in_fence) {
        if (fence_probe == fence_marker) {
          in_fence = 0
        }
        next
      }

      is_definition = parse_reference_definition(content)
      if (!is_definition) {
        parse_reference_usages(content)
      }
      parse_inline_links(content)
    }

    END {
      for (reference in reference_uses) {
        if (!(reference in reference_definitions)) {
          print "__OAW_MISSING_REFERENCE__" reference_uses[reference]
        }
      }
    }
  ' "$markdown_file" >"$CHECK_TEMP/links"

  while IFS= read -r link_target; do
    case "$link_target" in
      ''|\#*|http://*|https://*|mailto:*) continue ;;
      __OAW_MISSING_REFERENCE__*)
        missing_reference=${link_target#__OAW_MISSING_REFERENCE__}
        fail "missing Markdown reference definition in ${markdown_file#"$REPOSITORY"/}: $missing_reference"
        ;;
      /*|file:*)
        fail "absolute local Markdown link in ${markdown_file#"$REPOSITORY"/}: $link_target"
        ;;
    esac
    link_target=${link_target%%#*}
    [ -n "$link_target" ] || continue
    [ -e "$(dirname -- "$markdown_file")/$link_target" ] ||
      fail "missing Markdown link target in ${markdown_file#"$REPOSITORY"/}: $link_target"
  done <"$CHECK_TEMP/links"
done <"$CHECK_TEMP/markdown-files"

printf 'PASS: bilingual documentation contracts and local links passed\n'
