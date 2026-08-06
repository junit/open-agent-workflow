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

require_literal() {
  document=$1
  expected=$2

  grep -F -- "$expected" "$REPOSITORY/$document" >/dev/null ||
    fail "$document omits required release boundary: $expected"
}

reject_literal() {
  document=$1
  forbidden=$2

  if grep -F -- "$forbidden" "$REPOSITORY/$document" >/dev/null; then
    fail "$document contains stale release boundary: $forbidden"
  fi
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

cat >"$CHECK_TEMP/release-boundaries" <<'EOF'
README.md|Public installation management is Go-authoritative.
README.md|`install.sh` is an offline sibling-binary compatibility wrapper.
README.md|Release archives contain precompiled binaries and perform no runtime executable download.
README.md|Installation management distributes the canonical Policy and target-native instruction entrypoints; it does not execute engineering work.
README.md|OAW Core is required and stateless. The Workflow Coordinator is optional and stores only Workflow State for `WORKFLOW`; Install State and Workflow State are disjoint, with no migration or implicit adoption.
README.md|The Agent Host owns Agents, model calls, MCP, Hooks, Skills, Plugins, authentication, tools, sandbox, approvals, and every physical effect. OAW never starts a model process.
README.md|`CURRENT` uses the active session unchanged. `SUBAGENT` is eligible only when the active Host provides a native Subagent facility; there is no process fallback. All nine built-in integrations currently expose the `policy` surface. A future `host-native` integration may report session facts and Receipts without transferring execution authority to OAW.
README.md|Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness.
README-zh.md|公开安装管理以 Go 为权威实现。
README-zh.md|`install.sh` 是离线的同目录二进制兼容包装器。
README-zh.md|发布归档包含预编译二进制，运行时不会下载可执行文件。
README-zh.md|安装管理只分发 canonical Policy 和 target-native 指令入口，不执行工程工作。
README-zh.md|OAW Core 是必需且无状态的。Workflow Coordinator 是可选的，只为 `WORKFLOW` 保存
README-zh.md|Agent Host 拥有 Agent、model call、MCP、Hook、Skill、Plugin、认证、工具、sandbox、
README-zh.md|`CURRENT` 原样使用当前会话。只有 active Host 提供原生 Subagent facility 时，
README-zh.md|可用的原生和 Docker smoke test 必须通过；不可用的平台检查返回 77，且不阻塞 release readiness。
EOF
while IFS='|' read -r boundary_document boundary_text; do
  require_literal "$boundary_document" "$boundary_text"
done <"$CHECK_TEMP/release-boundaries"

cat >"$CHECK_TEMP/current-user-documents" <<'EOF'
README.md
README-zh.md
CHANGELOG.md
CONTRIBUTING.md
CONTRIBUTING-zh.md
SECURITY.md
SECURITY-zh.md
docs/en/installer.md
docs/zh/installer.md
docs/en/architecture.md
docs/zh/architecture.md
docs/en/troubleshooting.md
docs/zh/troubleshooting.md
docs/en/extending-adapters.md
docs/zh/extending-adapters.md
EOF
cat >"$CHECK_TEMP/stale-release-boundaries" <<'EOF'
Bash remains authoritative
Bash 仍是权威
public `oaw install` is not enabled
public `oaw install` 尚未启用
zero-dependency Bash installer
actual WSL smoke pass is required before publishing a release
release readiness remains blocked until
发布前必须在真实 WSL 环境通过 smoke test
仍不具备 release readiness
EOF
while IFS= read -r current_document; do
  while IFS= read -r stale_boundary; do
    reject_literal "$current_document" "$stale_boundary"
  done <"$CHECK_TEMP/stale-release-boundaries"
done <"$CHECK_TEMP/current-user-documents"

cat >"$CHECK_TEMP/host-scope-documents" <<'EOF'
README.md
README-zh.md
docs/en/architecture.md
docs/zh/architecture.md
docs/en/lifecycle.md
docs/zh/lifecycle.md
docs/en/troubleshooting.md
docs/zh/troubleshooting.md
docs/en/security.md
docs/zh/security.md
policy/ENGINEERING.md
EOF
while IFS= read -r host_scope_document; do
  for authority_term in \
    'Provider Family' \
    'Distribution' \
    'Host Installation' \
    'Host Binding Evidence' \
    'Verified Provider Instance'; do
    require_literal "$host_scope_document" "$authority_term"
  done
done <"$CHECK_TEMP/host-scope-documents"

for lifecycle_document in \
  README.md README-zh.md \
  docs/en/lifecycle.md docs/zh/lifecycle.md \
  docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
  for pin_field in provider_id host_id installation_key evidence_digest; do
    require_literal "$lifecycle_document" "$pin_field"
  done
done

for diagnostic_document in docs/en/troubleshooting.md docs/zh/troubleshooting.md; do
  for stable_reason in \
    HOST_BINDING_EVIDENCE_REQUIRED \
    PROVIDER_BINDING_UNAVAILABLE \
    PROVIDER_FOREIGN_HOST_ONLY \
    PROVIDER_PIN_INCOMPATIBLE \
    HOST_PROVIDER_SCOPE_MISMATCH; do
    require_literal "$diagnostic_document" "$stable_reason"
  done
  require_literal "$diagnostic_document" 'oaw.provider-descriptor/v1'
  require_literal "$diagnostic_document" 'oaw.user-config/v1'
done

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
