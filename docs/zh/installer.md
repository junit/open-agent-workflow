# 安装器

安装器在 User 或 Project scope 管理一套 Canonical Policy Set。它不安装 Profile 使用的工程 Skill、
不读取这些 Skill 的内容、不调用 model，也不创建工作流执行状态。它会安装每个选中 Host target
自身使用的小型 OAW dispatcher Skill、command 或 Workflow。

## 命令

~~~text
oaw check [--project PATH] [--target IDS]
oaw install [--project PATH] [--target IDS] [--dry-run] [--force]
oaw update [--project PATH] [--target IDS] [--dry-run] [--force]
oaw uninstall [--project PATH] [--target IDS] [--dry-run] [--force]
oaw profile list
oaw profile show SOURCE:ID
oaw profile check SOURCE:ID
~~~

User installation 默认写入用户 Host 指令文件。Project installation 将自包含的 Policy Set 写入
PATH/.oaw/policy 和项目 Adapter。项目 set 优先于 User set，但不会合并。

## 各 Scope 的 Artifact

每个选中的 target 都是一个逻辑单元，包含 Activation Router 和薄原生入口。Codex 还包含禁止隐式
Skill 调用的 metadata。User install 精确支持以下四个 target：

| Host | User Router | User 原生入口 |
| --- | --- | --- |
| Claude | `~/.claude/CLAUDE.md` | `~/.claude/skills/oaw/SKILL.md` |
| Codex | `~/.codex/AGENTS.md` | `~/.agents/skills/oaw/SKILL.md`；`~/.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `~/.gemini/GEMINI.md` | `~/.gemini/commands/oaw.toml` |
| OpenCode | `$XDG_CONFIG_HOME/opencode/AGENTS.md` | `$XDG_CONFIG_HOME/opencode/commands/oaw.md` |

Project install 精确支持以下九个 target；路径均相对于 `--project PATH` 指定的项目根目录：

| Host | Project Router | Project 原生入口 |
| --- | --- | --- |
| Claude | `.claude/CLAUDE.md` | `.claude/skills/oaw/SKILL.md` |
| Codex | `AGENTS.md` | `.agents/skills/oaw/SKILL.md`；`.agents/skills/oaw/agents/openai.yaml` |
| Gemini | `GEMINI.md` | `.gemini/commands/oaw.toml` |
| OpenCode | `AGENTS.md` | `.opencode/commands/oaw.md` |
| Cursor | `.cursor/rules/open-agent-workflow.mdc` | `.cursor/skills/oaw/SKILL.md` |
| Windsurf | `.devin/rules/open-agent-workflow.md` | `.windsurf/workflows/oaw.md` |
| Cline | `.clinerules/open-agent-workflow.md` | `.cline/skills/oaw/SKILL.md` |
| Roo | `.roo/rules/open-agent-workflow.md` | `.roo/commands/oaw.md` |
| Copilot | `.github/instructions/open-agent-workflow.instructions.md` | `.github/skills/oaw/SKILL.md` |

Copilot artifact 是 Copilot CLI Agent Skill，不是 VS Code Prompt File。Codex 使用
`$oaw [PROFILE] <request>`，其他所有 target 使用 `/oaw [PROFILE] <request>`。用户仍可直接用
自然语言要求使用 OAW，不需要原生入口。Dispatcher 不包含 Policy 路径，而是使用该 Host 已安装的
Activation Router，因此 Host 模板预处理不会重新解释安装坐标。

## 所有权

Managed block 会保留 Host 指令周围内容。只有目标不存在时才创建 owned file。所有原生入口和 Codex
native policy metadata 都是 owned file，部分项目 Router 格式也是 owned file。任一 owned destination
存在外部文件都会产生冲突；普通 install 和 `--force` 都不会覆盖或接管它。

Install State 记录本次安装拥有的 Policy Set 文件、每个 target artifact 及其 checksum、scope 和目录。
只有声明的所有 artifact 都被记录，选中的 target 才算完整。写入前会准备并检查整个请求，因此单个入口
冲突不会产生半安装 target。Install State 只是更新与卸载的私有记账，不是工作流进度。

0.1.0 和 0.1.1 版本的 Install State format 1 只作为旧版升级输入读取。`check` 会把其中每个干净的
legacy target 报告为 `upgrade-required`。使用相同 release 内容执行干净的 `install`，或通过
`update` 更新到当前内容时，会为该安装已记录的所有 target 补齐原生 artifact，并原子写成 format 2。
新 owned-file destination 已有外部文件时仍然视为冲突，`--force` 也不会接管。部分 uninstall 可以为
剩余 legacy target 保留 format 1；全新安装和完成迁移的安装都使用 format 2。

install、update、uninstall 在写入前校验所有路径和源。`check` 聚合 Router 和入口的健康状态；已跟踪
owned file 缺失、被替换或被编辑都会成为 drift。`update` 从同一 release 刷新选中 target 的全部
artifact。Force 可以在修复被编辑或替换的已跟踪文件前创建私有 backup；缺失文件没有可备份的原
文件，因此 force 会拒绝，必须先从可信副本恢复完全相同的文件再重试。Force 不会接管未跟踪文件，
也不会改变另一 scope 的安装。

如果 install 或 legacy migration 在开始写入后失败，OAW 会按逆序恢复已更改文件，并且只删除本次
创建、仍为空且文件系统身份与创建时一致的目录。并发替换目录和外部内容都会被保留，不会被删除。

`uninstall` 删除干净的已跟踪 owned file、从共享指令文件移除 OAW managed block，并且只删除本次
安装拥有的空目录。外部内容会被保留。破坏性清理前，必须先解决 tracked drift，或使用同一套
force-and-backup 规则处理。

install 或 update 后应执行 [Host Adapter](adapters.md) 记录的刷新动作。Gemini 使用
`/commands reload`，Copilot CLI 使用 `/skills reload`；没有 reload command 的 Host 需要按文档新建
session、task、chat，reload workspace 或重启进程。`check` 为 clean 只验证安装字节和所有权，不代表
真实 Host runtime E2E。

## 包装器与发布

install.sh 只解析同目录的 oaw 或 oaw.exe，不搜索 PATH 中的其他可执行文件，不下载 release，也不构建代码。
Release archive 包含预编译二进制、包装器、Policy 文档和 checksum。

从源代码构建：go build -o ./oaw ./cmd/oaw。执行前使用发布的 SHA256SUMS 校验 release。
