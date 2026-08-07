# Adapter 证据

[English](../en/adapters.md) | [README 中文](../../README-zh.md)

本指南记录 Open Agent Workflow（OAW）如何把一份 canonical policy 投射到每个受支持的
agent 工具。OAW 行为由本地 [lib/targets.sh](../../lib/targets.sh) 与
[lib/render.sh](../../lib/render.sh) 中的 adapter registry 和 renderer 定义。Provider
行为只依据下方列出的一手官方来源。

所有官方来源 **Retrieved: 2026-07-30**。

## 支持级别

| Target ID | 工具 | OAW scope | Control surface |
| --- | --- | --- | --- |
| `claude` | Claude Code | user + project | `policy` |
| `codex` | Codex CLI | user + project | `policy` |
| `gemini` | Gemini CLI | user + project | `policy` |
| `opencode` | OpenCode | user + project | `policy` |
| `cursor` | Cursor | project only | `policy` |
| `windsurf` | Windsurf / Devin rules | project only | `policy` |
| `cline` | Cline | project only | `policy` |
| `roo` | Roo Code | project only | `policy` |
| `copilot` | GitHub Copilot | project only | `policy` |

User scope 默认安装 core adapter：`claude,codex,gemini,opencode`。Project scope 默认
按上表 registry order 安装全部九个 target。Extension adapter 不支持 user scope 是 OAW
的支持决策，不代表 provider 没有全局设置。

## Host Integration Surface

Adapter 安装与 Host execution 是两个不同问题。Codex 默认暴露 `oaw/codex-policy`，
并通过独立且经过审计的 `oaw/codex-host` host-native Bridge 提供显式 opt-in。Bridge
只支持 `CURRENT` 与 `skill` binding。其他内置 target 仍是 policy surface，除非它们各自
的 Host-native integration 被显式安装并验证。

Codex Bridge 不能从 `codex` target adapter 推断出来。它必须单独安装并信任，并且只有
trusted Hook observation 成功后才报告 current-session fact。`host-native` integration
是明确的 Host 能力，不能从 target name 推断。

`CURRENT` 表示 active Agent session 保持不变。`SUBAGENT` 表示 active Agent Host 通过
原生 Subagent facility 创建 child。可用性是 session-dependent；facility 缺失时返回
`SUBAGENT_UNAVAILABLE`，没有 process fallback。

host-native adapter 可以报告 secret-free session facts、Provider Binding Evidence、topology
availability、Dispatch Packet status 和 normalized Receipts。Agent Host owns physical execution authority。
OAW 绝不启动 model process，也不要求 adapter 重建 MCP、Hook、Skill、
Plugin、认证、sandbox、approval 或 private configuration。

Host 可以报告 `inherited`、`host-configured`、`restricted`、`unknown` 或 `unavailable`
environment observation。Receipt 只是 Host attested outcome 的 evidence，不声称 OAW
物理包含了 Host。

## OAW 路径

Canonical OAW policy 安装在
`$XDG_CONFIG_HOME/open-agent-workflow/ENGINEERING.md`；未设置 `XDG_CONFIG_HOME` 时使用
`~/.config/open-agent-workflow/ENGINEERING.md`。User-scope state 位于
`$XDG_STATE_HOME/open-agent-workflow/installations/user.state`，未设置 `XDG_STATE_HOME`
时使用 `~/.local/state/open-agent-workflow/installations/user.state`。Project-scope state
位于 `$XDG_STATE_HOME/open-agent-workflow/installations/projects/<project-id>.state`。

| Target ID | OAW user 路径 | OAW project 路径 | OAW ownership |
| --- | --- | --- | --- |
| `claude` | `$HOME/.claude/CLAUDE.md` | `.claude/CLAUDE.md` | Managed block |
| `codex` | `$HOME/.codex/AGENTS.md` | `AGENTS.md` | Managed block |
| `gemini` | `$HOME/.gemini/GEMINI.md` | `GEMINI.md` | Managed block |
| `opencode` | `$XDG_CONFIG_HOME/opencode/AGENTS.md` | `AGENTS.md` | Managed block |
| `cursor` | 不支持 | `.cursor/rules/open-agent-workflow.mdc` | Owned file |
| `windsurf` | 不支持 | `.devin/rules/open-agent-workflow.md` | Owned file |
| `cline` | 不支持 | `.clinerules/open-agent-workflow.md` | Owned file |
| `roo` | 不支持 | `.roo/rules/open-agent-workflow.md` | Owned file |
| `copilot` | 不支持 | `.github/instructions/open-agent-workflow.instructions.md` | Owned file |

Managed-block adapter 保留 shared destination 中无关的内容，只替换带 marker 的 OAW
block。Owned-file adapter 要求精确 destination 不存在或已由 OAW 拥有；OAW 不合并这些
文件内部的内容。这些是 OAW 在 `lib/targets.sh` 与 `lib/render.sh` 中定义的 mechanical
choice，不是 provider-level precedence rule。

下表中，**documented import** 指 provider 定义的文件导入能力；如果 provider 没有这类
documented import，**OAW bootstrap** 指要求模型读取 canonical policy 的可见指令。

## Adapter Matrix

| Target | 官方加载行为 | OAW rendering 选择 | Precedence 与 reload 注意事项 | 一手来源 |
| --- | --- | --- | --- | --- |
| Claude Code | Claude Code 加载 `CLAUDE.md` memory file，并文档化 `@path` import。它支持 `~/.claude/CLAUDE.md` user memory，以及来自 `CLAUDE.md` 或 `.claude/CLAUDE.md` 的 project memory。Import 可以递归。 | OAW 写入含 `@<canonical-policy-path>` 的 managed block，让 Claude 直接导入 canonical policy。Ownership marker 对安装器仍有用，但 HTML 注释会从注入的上下文中剥离。 | Claude 在 session 启动时加载 memory。官方文档说明从宽到窄的 memory loading 与 `@` import 限制；变更未反映时应重启或刷新 Claude Code session。 | <https://code.claude.com/docs/en/memory> |
| Codex CLI | Codex 文档化 `AGENTS.md` discovery，包括 `$HOME/.codex/AGENTS.md` user instruction，以及来自 `AGENTS.md` 的 project instruction。它还记录 fallback filename；更近的 instruction 在 combined prompt 中更晚出现，从而覆盖更宽泛的 instruction。 | OAW 写入要求 Codex 读取 canonical policy 的 model-visible bootstrap。Codex 没有文档化的 Markdown import，因此 OAW 不依赖 import marker。 | Codex 为新的 run 或 TUI session 重建 instruction。变更后的 `AGENTS.md` 应在下一次 Codex 调用中生效；OAW 不声称正在进行的 exchange 内可以 live hot reload。 | <https://developers.openai.com/codex/guides/agents-md> |
| Gemini CLI | Gemini CLI 使用分层 `GEMINI.md` context file。它文档化 global `~/.gemini/GEMINI.md`、project 与 subdirectory context、`@file.md` import、nested import、循环检测和 `/memory refresh`。 | OAW 写入含 `@<canonical-policy-path>` 的 managed block，使用 Gemini 文档化的 memory import 行为。 | Gemini context 是分层的；更具体的 context 可以补充或覆盖更宽泛的 guidance。使用 `/memory show` 检查已加载 context，编辑后使用 `/memory refresh`。 | <https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/configuration.md>, <https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/memport.md> |
| OpenCode | OpenCode 文档化 project root 与 `~/.config/opencode/AGENTS.md` 中的 `AGENTS.md` instruction file。没有 OpenCode file 时，它还文档化 Claude fallback file。Rules 页面说明 OpenCode 不会自动解析 `AGENTS.md` 中的文件引用。 | OAW 写入要求 OpenCode 读取 canonical policy 的 model-visible bootstrap。OpenCode 没有文档化的 Markdown import，因此 OAW 不依赖 import marker。 | OpenCode 按文档化 precedence 搜索 local 与 global instruction。它还支持 `opencode.json` instruction entry，包括 glob 与 URL；OAW v0.1 有意使用 `AGENTS.md` bootstrap，而不修改 provider config。 | <https://opencode.ai/docs/rules/> |
| Cursor | Cursor Project Rules 是 `.cursor/rules` 下的 `.mdc` 文件。Frontmatter 控制 `description`、`globs` 和 `alwaysApply`；`alwaysApply: true` 表示始终包含该 rule。Cursor 也文档化 project-root 与 nested `AGENTS.md`，但 Project Rules 要求 `.mdc`。 | OAW 创建一个 owned `.mdc` file，使用 `alwaysApply: true`、`globs: "**/*"`，以及要求 agent 读取 canonical policy 的 bootstrap。 | Cursor 按 Team、Project、User 的文档化 precedence 合并 rule。Nested `AGENTS.md` 支持是官方功能，但 OAW 不使用它，因为 `.mdc` rule surface 是带显式 frontmatter 的稳定 project-rule target。 | <https://cursor.com/docs/rules> |
| Windsurf / Devin rules | Devin Desktop / Cascade 文档化 `.devin/rules/*.md` workspace rule，以 `.windsurf/rules/*.md` 为 fallback，并仍读取 legacy `.windsurfrules`。`.devin/` 是优先 surface 且 precedence 更高。Rule frontmatter 可设置 `trigger: always_on`。 | OAW 创建一个带 `trigger: always_on` 与 bootstrap text 的 owned `.devin/rules/open-agent-workflow.md` 文件。 | Workspace rule 仅作用于 project。由于当前 surface 优先使用 `.devin/rules`，OAW 不写入 `.windsurf/rules` 或 legacy `.windsurfrules`。如果目标应用已缓存 workspace rule，应重启或刷新。 | <https://docs.devin.ai/desktop/cascade/memories> |
| Cline | Cline 的主要 project rule 格式是 `.clinerules/`，会处理其中的 `.md` 与 `.txt` 文件。它还检测包括 `AGENTS.md` 在内的多个兼容文件。发生冲突时，workspace rule 优先于 global rule。 | OAW 创建一个含 bootstrap text 的 owned `.clinerules/open-agent-workflow.md` 文件。 | Cline 支持带 YAML `paths` frontmatter 的 conditional rule；没有 frontmatter 表示 rule 始终 active。OAW 使用 always-active 形式，因为 lifecycle gate 要在 project 各处的工程生命周期工作前运行。 | <https://docs.cline.bot/customization/cline-rules> |
| Roo Code | Roo Code 首选 `.roo/rules/` workspace rule directory，以 `.roorules` 为 fallback；首选的 mode-specific rule 使用 `.roo/rules-<modeSlug>/`。Roo 也文档化 workspace-root `AGENTS.md` / `AGENT.md`，但 `.roo/rules/` 是首选 workspace rule surface。 | OAW 创建一个含 bootstrap text 的 owned `.roo/rules/open-agent-workflow.md` 文件。 | Roo 先加载 global rule，再加载 workspace rule；冲突时 workspace rule 优先。Rule directory 会以字母顺序递归读取，symlink 有文档化 depth limit。OAW 写普通文件，不使用 Roo 的 `AGENTS.md` fallback。 | <https://docs.roocode.com/features/custom-instructions> |
| GitHub Copilot | GitHub Copilot repository custom instruction 支持 repository-wide `.github/copilot-instructions.md`，以及带 `applyTo` frontmatter 的 path-specific `.github/instructions/<name>.instructions.md`。两者都匹配时会同时使用。GitHub 与 VS Code 也文档化 `AGENTS.md`，VS Code 把 nested `AGENTS.md` 支持标为 experimental。 | OAW 创建一个使用 `applyTo: "**"` 与 bootstrap text 的 owned `.github/instructions/open-agent-workflow.instructions.md` 文件。OAW 不使用 Copilot `AGENTS.md` 行为，因为实验性的嵌套 `AGENTS.md` 行为未被采用。 | GitHub 文档化 personal、repository 与 organization instruction priority。OAW 使用 path-specific repository instruction，从而不修改 user 或 organization setting，并避开 experimental nested `AGENTS.md` 行为。 | <https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions>, <https://code.visualstudio.com/docs/agent-customization/custom-instructions> |

## Shared Destination 注意事项

Codex 与 OpenCode 在 OAW project scope 都使用 project-root `AGENTS.md`。因此 OAW 把
`AGENTS.md` 视为 shared managed-block destination，渲染一个连贯的 OAW block，而不是
两个独立 block。这样既保留无关 project instruction，也避免重复 lifecycle gate text。

## 官方行为与 OAW 选择

- 官方 provider 行为定义工具从哪里查找 instruction、是否支持 import，以及自身的
  precedence 如何工作。
- OAW choice 定义使用哪个受支持 provider surface、destination 是 managed block 还是
  owned file，以及如何让模型看到 canonical policy path。
- Claude 与 Gemini 使用文档化的 Markdown import 行为。
- Codex 与 OpenCode 的 instruction Markdown file 没有文档化的自动 Markdown import；
  OAW 为它们使用 model-visible bootstrap text。
- Cursor 要求 `.mdc` 作为 Project Rules。尽管 Cursor 文档化 `AGENTS.md`，OAW 使用
  `.cursor/rules/open-agent-workflow.mdc`。
- Windsurf / Devin rules 优先使用 `.devin/rules`，因此 OAW 使用这个路径，不使用
  `.windsurf/rules` 或 `.windsurfrules`。
- Cline 使用 `.clinerules`，Roo Code 使用 `.roo/rules`。
- GitHub Copilot path-specific instruction 使用带 `applyTo` frontmatter 的
  `.github/instructions`。VS Code 中实验性的嵌套 `AGENTS.md` 行为未被采用。

## 来源与不确定性

已验证的一手官方来源：

- Claude Code memory：<https://code.claude.com/docs/en/memory>
- Codex CLI `AGENTS.md`：<https://developers.openai.com/codex/guides/agents-md>
- Gemini CLI configuration：<https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/configuration.md>
- Gemini CLI memory import processor：<https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/reference/memport.md>
- OpenCode rules：<https://opencode.ai/docs/rules/>
- Cursor rules：<https://cursor.com/docs/rules>
- Devin Desktop / Cascade memories 与 rules：<https://docs.devin.ai/desktop/cascade/memories>
- Cline rules：<https://docs.cline.bot/customization/cline-rules>
- Roo Code custom instructions：<https://docs.roocode.com/features/custom-instructions>
- GitHub Copilot repository instructions：<https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions>
- VS Code custom instructions：<https://code.visualstudio.com/docs/agent-customization/custom-instructions>

已知不确定性：

- Provider 文档变化频繁；上方 retrieval date 记录 OAW v0.1 文档采用的 evidence snapshot。
- OpenCode 文档化 `opencode.json` 的 `instructions`，其中包括 glob 与 remote URL。本指南中
  “没有文档化的 Markdown import”只限于 `AGENTS.md` 内部的自动 reference parsing。
- Cursor 与 Copilot 都文档化 `AGENTS.md` 行为。OAW v0.1 有意不在这些 project-extension
  adapter 中使用这些 surface。
- 各 provider 没有统一文档化 hot-reload 行为。如果 provider 未记录 rule file 的 live
  reload，应假定 OAW 改变 adapter file 后需要新 session 或应用刷新。
