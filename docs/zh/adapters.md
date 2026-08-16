# Host Adapter

Adapter 是某个 Agent Host 的安装与加载指引，不是工程方法，也不增加 Profile Responsibility。

## Adapter 契约

每个 Adapter 说明 Activation Router 与薄原生入口路径、scope 与 precedence、managed-block 或
owned-file 规则、刷新行为和原生 Skill 或 command surface。可以说明可读 cache 位置作为 fallback，
但不能要求特定 cache、lockfile、revision 或 digest 才能使用 Policy。

Canonical Policy Set 当前包含 Claude、Codex、Gemini、OpenCode、Cursor、Windsurf、Cline、Roo 和 Copilot
Adapter。它们通过各自的原生指令格式应用同一套可移植语义。要增加可安装的 Host target，应在
`policy/adapters/` 中加入 Adapter 指引，并在 `internal/management/targets.go` 中加入 destination 坐标。

## 安装坐标

User scope 支持四个具有稳定用户坐标的 Host。`$XDG_CONFIG_HOME` 使用平台解析出的配置根目录；
其常见 Unix 默认值是 `~/.config`。

| Host | User Activation Router | User 原生入口 |
| --- | --- | --- |
| Claude | `~/.claude/CLAUDE.md` | `~/.claude/skills/oaw/SKILL.md` |
| Codex | `~/.codex/AGENTS.md` | `~/.agents/skills/oaw/SKILL.md` 和 `~/.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `~/.gemini/GEMINI.md` | `~/.gemini/commands/oaw.toml` |
| OpenCode | `$XDG_CONFIG_HOME/opencode/AGENTS.md` | `$XDG_CONFIG_HOME/opencode/commands/oaw.md` |

Project scope 支持全部九个 Host，以下路径均相对于选中的项目根目录。

| Host | Project Activation Router | Project 原生入口 |
| --- | --- | --- |
| Claude | `.claude/CLAUDE.md` | `.claude/skills/oaw/SKILL.md` |
| Codex | `AGENTS.md` | `.agents/skills/oaw/SKILL.md` 和 `.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `GEMINI.md` | `.gemini/commands/oaw.toml` |
| OpenCode | `AGENTS.md` | `.opencode/commands/oaw.md` |
| Cursor | `.cursor/rules/open-agent-workflow.mdc` | `.cursor/skills/oaw/SKILL.md` |
| Windsurf | `.devin/rules/open-agent-workflow.md` | `.windsurf/workflows/oaw.md` |
| Cline | `.clinerules/open-agent-workflow.md` | `.cline/skills/oaw/SKILL.md` |
| Roo | `.roo/rules/open-agent-workflow.md` | `.roo/commands/oaw.md` |
| Copilot | `.github/instructions/open-agent-workflow.instructions.md` | `.github/skills/oaw/SKILL.md` |

Copilot target 明确是 **Copilot CLI Agent Skill**，不是 `.github/prompts/` 下的 VS Code Prompt
File；本契约不声明跨 surface 的 Prompt File 行为。

## 调用与刷新

| Host | 显式调用 | 安装或变更后的刷新方式 |
| --- | --- | --- |
| Claude | `/oaw [PROFILE] <request>` | 新建顶层 Skill 目录或修改 Router 后启动新 session。 |
| Codex | `$oaw [PROFILE] <request>` | Codex 通常自动发现 Skill 变更；看不到 `oaw` 时重启 session。 |
| Gemini | `/oaw [PROFILE] <request>` | 运行 `/commands reload`；Router 或更广的 context 变化需要新 session。 |
| OpenCode | `/oaw [PROFILE] <request>` | 重启 OpenCode。 |
| Cursor | `/oaw [PROFILE] <request>` | 启动新的 Agent chat；Skill 不可见时 reload workspace。 |
| Windsurf | `/oaw [PROFILE] <request>` | 启动新的 Cascade task 或 reload workspace。 |
| Cline | `/oaw [PROFILE] <request>` | 启动新的 Cline task 或 reload active context。 |
| Roo | `/oaw [PROFILE] <request>` | 启动新的 Roo task；必要时使用已记录的 VS Code window reload。 |
| Copilot CLI | `/oaw [PROFILE] <request>` | 运行 `/skills reload`；其他兼容 Agent surface 需新 chat 或 reload Host。 |

每个 Host 仍支持“Use OAW with SP-FULL to deliver this change”这样的自然语言激活。原生入口没有
更高优先级，也不是 Policy 正常运行的必要条件。它只把显式用户意图、可选 Profile 和请求带入选中的
Policy Set，不得选择默认 Profile、复制 Responsibility、定义生命周期阶段或施加 approval gate。

自动发现、相关性匹配或模型主动加载名为 OAW 的 Skill 都不是显式激活。Claude、Codex 等具有已记录
explicit-only 控制的 Host 会使用该控制。Cline 没有已记录的 per-Skill manual-only 字段，因此其入口
依赖 Policy self-gating：只有观察到明确用户意图后才能激活 OAW。物理调用本身不是用户意图证据；
顶层请求或可靠的 Host 元数据必须能识别用户选择。每个 Dispatcher 只跟随 Activation Router，且不包含
Policy 路径，避免 Host 模板预处理接触安装坐标。

## Target 所有权

Managed-block Router 保留周围指令。原生入口、Codex 入口 metadata，以及 Host 格式要求独立文件的
Router，都是不包含用户内容的 owned file。安装不会覆盖或接管 owned destination 上的未跟踪文件，
即使使用 `--force` 也不会。

Install State 独立跟踪每个 artifact。`check` 会把缺失或被编辑的已跟踪入口报告为 drift。`update`
从同一 release 一起刷新 Router 与原生入口。被编辑的已跟踪 artifact 可以走常规
force-and-backup 修复路径；缺失文件没有可备份的原文件，因此 force 会拒绝，必须先从可信副本恢复
完全相同的文件再重试。`uninstall` 删除干净的 OAW-owned file 及其空 owned directory，同时保留
managed file 周围内容和外部文件。Adapter 必须声明每个 destination 的所有权，让这些操作保持保守。

## 分层

可移植规则属于 POLICY.md、cooperative-protocol.md 和 Profile。Host path 与调用细节属于 Adapter。
机器身份与 attestation 属于可选 Machine Assurance。分层可以避免 Host scanner 变成 Policy 依赖。

这些坐标与管理契约本身不代表已经完成每个 Host 的真实 runtime 端到端验证。只有对应真实 Host
session 已实际运行后，才会报告 runtime dogfood。
