# 故障排查

## OAW 没有激活

激活是显式且有任务范围的。可以用“Use OAW with SP-FULL to deliver this change”开头。
也可以在 Codex 中使用 `$oaw SP-FULL deliver this change`，或在其他八个支持的 Host 中使用
`/oaw SP-FULL deliver this change`。自然语言激活始终有效，不依赖原生入口。

讨论 OAW、仓库文本、任务复杂度、普通 Skill 使用，或自动/模型主动加载名为 OAW 的 Skill 都不会
激活 OAW。只有顶层请求或可靠 Host 元数据能证明由用户选择的原生 OAW 入口，才等价于显式请求；物理
调用本身不是证明。Dispatcher 不选择默认 Profile、不定义生命周期阶段，也不嵌入 Policy 路径，而是
跟随 Activation Router。

## 原生入口缺失

先用相同 scope 和 target 运行 `oaw check`，确认结果为 clean。User scope 只为 Claude、Codex、
Gemini 和 OpenCode 安装入口；Cursor、Windsurf、Cline、Roo 和 Copilot 入口需要 Project install。
精确路径见 [Host Adapter](adapters.md)。

然后刷新 Host：

- Claude：新建顶层 Skill 目录或修改 Router 后启动新 session。
- Codex：未自动发现 `$oaw` 时重启 session。
- Gemini：运行 `/commands reload`；Router 变更需新 session。
- OpenCode：重启 OpenCode。
- Cursor：启动新的 Agent chat；必要时 reload workspace。
- Windsurf：启动新的 Cascade task 或 reload workspace。
- Cline：启动新的 task 或 reload active context。
- Roo：启动新的 task；必要时使用已记录的 VS Code window reload。
- Copilot CLI：运行 `/skills reload`；其他 Agent surface 需新 chat 或 reload Host。

Copilot target 是位于 `.github/skills/oaw/SKILL.md` 的 Copilot CLI Agent Skill，不是
`.github/prompts/` Prompt File。

如果 Cline 在用户没有输入 `/oaw` 时发现或选择了 `oaw`，OAW 必须保持未激活。Cline 没有已记录
的 per-Skill manual-only 控制，因此 dispatcher 依赖 Policy self-gating 并检查明确用户意图。

## Profile 或 Skill 没有被发现

发现结果是 advisory。使用 `oaw profile list` 或 `oaw profile check` 检查 Markdown 元数据，然后要求
模型直接读取指定 Skill。九份 Host Adapter 都会先使用各自的原生 Skill surface，再回退到该 Host
原生、cross-agent、extension 或 Plugin 位置中的可读 Skill 文档。即使生成的索引或 Plugin listing
遗漏了 Matt、ECC 或 Superpowers，只要规则可读仍然可以使用。
Profile 中的限定名是语义引用；Host 可以用 basename 或不同的原生命名空间暴露同一个过程。

不要为了让 Policy Profile selectable 而添加 Provider pin、cache path、lockfile digest 或 Bridge；这些
属于可选证据问题。

## Project 与 User Set

一个交付物只加载一套 Policy Set。project/.oaw/policy 优先于 User set，文件不会合并。Custom Profile
保留来源；两个来源同 ID 时使用 project:id 或 user:id。

## Install 或 Update 失败

使用相同的 project 和 target 参数运行 oaw check。常见原因：

- 已有 managed block 被编辑或重复；
- Policy Set 文件或 target 漂移；
- 未跟踪文件占用了 owned destination；
- scope 与 Install State 不一致。

显式处理或备份用户内容后再运行 update。Force 会备份已跟踪的漂移，但不会接管外部文件。

`upgrade-required` 表示该安装仍使用 OAW 0.1.0 或 0.1.1 写入的 format 1 state。请对同一 scope
运行 `oaw update`。迁移会为该安装已经拥有的所有 target 补齐原生入口，再写成 format 2；如果入口
路径已被其他文件占用，则不会接管。

原生入口模板不包含 Policy 路径。Claude、OpenCode 和 Gemini 可以预处理参数、文件引用或命令语法，
但这些预处理无法改写或执行安装坐标片段；非模板的 Activation Router 是唯一 Policy Set 选择入口。

原生入口和 Codex 的 `agents/openai.yaml` 都是 owned file。对应路径上已有的未跟踪文件是所有权
冲突，不是升级候选，`--force` 也不会覆盖。安装后，缺失或被编辑的 Router、入口或 Codex metadata
都是 tracked drift。`update` 刷新选中 target 的所有 artifact。Force 可以备份并修复被编辑的已跟踪
文件；缺失文件没有可恢复的原文件，因此 force 会拒绝，必须先从可信副本恢复完全相同的文件再重试。
`uninstall` 删除干净的 OAW-owned file 和 managed block，但保留外部文件及周围 Host 指令。

## Bridge 或机器证据缺失

正常路径不要求它们。Bridge 和 Machine Assurance 只增加证据，不能阻断 Profile 选择、Skill 使用、
review、verification 或 completion。Host 安全策略仍然可以拒绝物理调用；这与 Policy 选择是两件事。

## 提交问题

提供命令、scope、target 和完整诊断，不要提供凭证、token 或私有 Skill 内容。安装缺陷应在新的临时项目中
复现。clean install 或 `check` 只证明静态字节和所有权；没有实际运行对应真实 Host session 时，不应
报告为 live Host runtime E2E。
