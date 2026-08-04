# Adapter 证据

[English](../en/adapters.md) | [README 中文](../../README-zh.md)

本指南记录 Open Agent Workflow（OAW）如何把一份 canonical policy 投射到每个受支持的
agent 工具。OAW 行为由本地 [lib/targets.sh](../../lib/targets.sh) 与
[lib/render.sh](../../lib/render.sh) 中的 adapter registry 和 renderer 定义。Provider
行为只依据下方列出的一手官方来源。

所有官方来源 **Retrieved: 2026-07-30**。

## 支持级别

| Target ID | 工具 | OAW scope | OAW 级别 |
| --- | --- | --- | --- |
| `claude` | Claude Code | user + project | Core |
| `codex` | Codex CLI | user + project | Core |
| `gemini` | Gemini CLI | user + project | Core |
| `opencode` | OpenCode | user + project | Core |
| `cursor` | Cursor | project only | Project extension |
| `windsurf` | Windsurf / Devin rules | project only | Project extension |
| `cline` | Cline | project only | Project extension |
| `roo` | Roo Code | project only | Project extension |
| `copilot` | GitHub Copilot | project only | Project extension |

User scope 默认安装 core adapter：`claude,codex,gemini,opencode`。Project scope 默认
按上表 registry order 安装全部九个 target。Extension adapter 不支持 user scope 是 OAW
的支持决策，不代表 provider 没有全局设置。

## Runtime Host 支持

Adapter 安装与 Runtime Host 能力是两个不同问题。当前唯一的
`runner-managed` Host 是用户明确选择的 Codex CLI integration
`oaw/codex-runner`。`oaw run --host codex` 会在启动任何 Host process 之前检查其
已 pin 的 Manifest、audit evidence、Conformance report 与 Integration record。
Claude、Gemini、OpenCode、Cursor、Windsurf、Cline、Roo 和 Copilot 仍是
`instruction-only`；它们的 policy adapter 不表示 Runtime Protocol、隔离、dispatch
或 evidence 保证。

`oaw run` 使用共享 Runtime Protocol。恢复 `CONTINUE` 或 `INSPECT` frame 时可以传入
`--project-root /absolute/project/path`，显式加载 project configuration；START frame
中的 project identity 优先，二者不一致会被拒绝。现有 Bash installer 仍是权威实现，
不会为 Policy-only adapter 安装 Runtime claim。

每次 Codex dispatch 都会收到已提交 Grant 的 effects 与 resources。不含
`write-project` 或 `git-local` 的 Grant 会被强制放入 Codex
`--sandbox read-only`；包含任一写 effect 的 Grant 使用
`--sandbox workspace-write`。Runner 永远不会选择 `danger-full-access`。Codex sandbox
mode 本身不能约束 MCP 子进程，因此 `oaw run` 将 Host discovery 与 execution 分开。
Discovery 读取选中的真实 Codex installation，并构造 Host-scoped Registry 与 Binding
Inventory。`Prepare` 会重新校验 Grant 中的 Provider Instance、Capability、Host
Installation、精确 Binding、inventory digest 与 physical evidence digest。随后每次
invocation 都在 Runtime state root 下获得私有 `0700` HOME 与中性 workspace；只有已验证的
skill 会映射进去，user config、project rule、hook、无关 plugin 与 MCP server 都不会加载。
`codex exec` 使用 `--ignore-user-config`、`--ignore-rules` 与 `--disable hooks`；原始
`CODEX_HOME` 只用于认证，physical project 通过 `--add-dir` 暴露。Evidence 发生变化时会在
模型启动前 fail closed。Agent 与 tool Binding 当前返回
`CODEX_BINDING_KIND_UNSUPPORTED`，因为隔离 profile 尚不能精确复现它们的 Host registration
语义。

收到 interrupt
或 termination signal 时，`oaw` 会先请求 Host cancel，再提交
`EXECUTION_UNCERTAIN` / `PAUSED`，并给出 `RECONCILE_INVOCATION` recovery。无法捕获的
`SIGKILL` 不能执行最后的状态转换，因此设置 deadline 的调用方必须先优雅取消，之后才能
使用 hard-kill fallback。调用方发现 stale authorized invocation 时，应重放同一个幂等
dispatch frame；Runtime 会在不再次调用 Codex 的情况下提交 uncertain pause。

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
