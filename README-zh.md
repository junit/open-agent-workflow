# Open Agent Workflow（OAW）

[English](README.md)

Open Agent Workflow（OAW）负责协调多个 agent 工具中独立安装的 workflow provider。它为一个工程交付物指定一个明确的生命周期所有者，或一份无冲突的阶段
映射，然后把同一套治理策略安装到受支持 coding agent 的指令入口中。

OAW 是 provider-neutral 的策略分发与生命周期协调系统。OAW Core 编译生命周期契约，
可选 Workflow Coordinator 持久化 Workflow State，外部 Agent Host 执行所有 effect。
它不重新分发 workflow family，也不取代 agent 工具自身的配置。

## 为什么需要 OAW

不同工程 workflow provider 都可能自动触发规划、实现、TDD、调试、复核和完成
流程。当一个 agent 同时具备多个 provider 时，这些触发器可能争夺同一职责，造成
重复计划、冲突的测试方法，或在后续请求中静默切换工作流。

同一位开发者也可能使用多个 agent 工具。每个工具的指令文件、作用域、优先级规则
和重新加载行为都不同。手工复制治理规则会造成跨客户端 drift。OAW 维护一份 canonical
policy，并围绕它渲染轻量的 target-native 入口。

## 解决的问题

- 独立安装的 workflow provider 之间存在重叠的自动触发器。
- 在一次交付过程中静默选择 profile 或切换方法。
- 规划、实现、测试、调试、复核与完成之间的职责归属不明确。
- 用户级和仓库级 agent 设置中的工作流指令不一致。
- 更新或删除与用户指令混合的内容时缺少安全边界。
- 把 provider 检测结果误当成选择工作流的授权。

## 核心能力

- 显式激活后，在 family-specific 生命周期启动前，将顶层工程请求评估为 `DIRECT`、
  `BOUNDED` 或 `WORKFLOW`。
- 在 `policy-cooperative` Workflow Mode 中报告 Host-visible Profile candidate，并等待
  用户显式选择一条 `CURRENT` 路径；机器支撑 session 则由 OAW Core 独立计算 eligible
  Profile 与 topology。
- 只在机器支撑路径中由 OAW Core 编译选定 Lifecycle Bundle，并让它跨后续请求、
  上下文压缩、ticket 和委派 agent 保持锁定。
- 可选地由 Workflow Coordinator 记录 Workflow revision、协作式 Resource Lease、
  Receipt 和 evidence reference。
- 支持完整 family profile、预定义 Matt-Superpowers hybrid、有限 specialist add-on，
  以及用户自定义的无冲突阶段映射。
- 独立检测 Superpowers、Matt Pocock skills 和 Everything Claude Code（ECC），检测
  本身不会替用户选择。
- 为九个 agent 工具安装用户级或项目级适配器。
- 提供幂等的 `check`、`install`、`update` 和 `uninstall` 生命周期，包含目标
  选择、dry run、drift 检查和可恢复的 force。
- 保留无关用户内容，只删除 OAW 所有的构件。

## Host-scoped Provider 权限

Provider 权限遵循一条精确的身份链：

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex 与 Claude Code 是相互独立的 Host。即使它们引用同一组物理文件，OAW 也会推导出
不同的 Host Installation 身份。Provider Descriptor binding 与配置的 installation hint
都只是声明，不能创建 Host Binding Evidence。`policy` Host 可以报告 Candidate，但
Candidate 在没有 verified Provider Instance 时不能满足 Profile compilation。foreign-Host
诊断绝不会成为 pin、Registry 输入、Profile owner、Capability Grant 或 Workflow 权限。

当前 Provider Descriptor 为 `oaw.provider-descriptor/v4`，用户配置仍为 v3；旧输入会被
拒绝，不会隐式升级。
存在歧义的当前 Host candidate 只能使用下列精确身份字段固定；`location` 与 `version`
是可选的可读断言：

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

稳定的 Host-scope 诊断原因包括 `HOST_BINDING_EVIDENCE_REQUIRED`、
`PROVIDER_BINDING_UNAVAILABLE`、`PROVIDER_FOREIGN_HOST_ONLY`、
`PROVIDER_PIN_INCOMPATIBLE` 与 `HOST_PROVIDER_SCOPE_MISMATCH`。使用
`oaw providers inspect --host <host> --format json` 查看物理证据；workflow denial
保持不包含路径。

## 快速开始

发布归档包含对应平台的预编译 `oaw` 或 `oaw.exe`。验证 `SHA256SUMS` 并解压后，
直接调用二进制：

```bash
./oaw check
./oaw install
./oaw install --project /path/to/repository
./oaw update --dry-run
./oaw uninstall
```

归档中的 `install.sh` 是兼容旧入口脚本的 Bash 3.2 包装器，以下命令会执行同目录
二进制：

```bash
./install.sh check
./install.sh install
./install.sh install --project /path/to/repository
./install.sh update --dry-run
./install.sh uninstall
```

从源码 checkout 使用时，必须先构建二进制，再调用任一入口：

```bash
go build -o ./oaw ./cmd/oaw
./oaw check
./oaw profile list
```

`check` 只报告 provider 检测、目标就绪情况和安装健康状态，不做变更。直接运行
`install` 时使用用户作用域和四个 core target；`--project` 选择一个已存在的仓库，
并默认使用全部九个 target。可以使用 `--target claude,codex`（或其他逗号分隔的
ID 集合）缩小命令范围。运行 `./oaw --help` 或 `./install.sh --help` 可查看 management
CLI。

Profile 检查是可选且只读的：

```bash
./oaw profile list
./oaw profile show MATT-FULL
./oaw profile show project:team-delivery
./oaw profile check .oaw/profiles/team-delivery.md
```

`profile list` 会报告当前二进制嵌入的 Built-in，以及当前 project 与 user config root 中的
直接 Markdown Custom Profile。Project/user 出现同 ID 时保留为两个条目，`show` 与 `check`
必须使用 `project:<id>` 或 `user:<id>`。同一 scope 的重复 ID、缺少必需 `id`/`name`
metadata，以及占用保留 Built-in ID 的 Custom Profile 都会被明确报告。Partial Profile
仍然有效；body 与 Responsibility 诊断只是 warning。这些命令不检查 Skill availability、
不选择 Profile、不创建 workflow state，也不判断模型能否使用某个 Profile。

### Core、Coordination 与 Host 边界

公开安装管理以 Go 为权威实现。

`install.sh` 是离线的同目录二进制兼容包装器。

发布归档包含预编译二进制，运行时不会下载可执行文件。

安装管理只分发 canonical Policy 和 target-native 指令入口，不执行工程工作。

协作式 Policy 路径不需要 OAW Core。只有机器支撑路径才要求无状态的 OAW Core。Workflow Coordinator 是可选的，只为 `WORKFLOW` 保存
Workflow State；Install State 与 Workflow State 相互独立，不迁移也不隐式接管。

Agent Host 拥有 Agent、model call、MCP、Hook、Skill、Plugin、认证、工具、sandbox、
approval 和全部物理 effect。OAW 绝不启动 model process。

`CURRENT` 原样使用当前会话。只有 active Host 提供原生 Subagent facility 时，
`SUBAGENT` 才可用；不存在 process fallback。Codex 默认提供 policy integration，并另有独立且经过审计的 host-native Bridge，必须显式安装并信任。当前 Codex 证明 `skill` binding 与 `CURRENT` topology；在有效 `SubagentStart` event 后，下一次 observation 还可以为精确 session/CWD 证明 `child-delegation`。Role、instruction、agent、tool、parallel/nested delegation 与 Host action 仍保持 unknown 或 unavailable。

可用的原生和 Docker smoke test 必须通过；不可用的平台检查返回 77，且不阻塞 release readiness。在 macOS 上，如果 Docker Desktop 可用，应使用 `scripts/smoke-docker.sh` 验证 Linux 归档。WSL-specific 检查是可选项，`SKIP` 必须记录，绝不能报告为 pass。

```bash
docker_arch=$(docker version --format '{{.Server.Arch}}')
bash scripts/smoke-docker.sh \
  "$PWD/dist/open-agent-workflow_0.1.0_linux_${docker_arch}.tar.gz"
```

## 显式激活

安装只会分发惰性 Activation Router，不会把日常工作自动纳入 OAW。在当前
顶层用户请求明确要求 OAW 治理某个交付物之前，Host 始终保持原生 Host 行为，
就像没有安装 OAW 一样。普通 Bug 修复、Host 自动选择 Skill，或用户直接调用普通
Skill，都继续使用 Host 原有的 routing，不会产生 OAW mode、gate、推荐或 state。

`/oaw <task>` 或“使用 OAW 处理 <task>”会为一个交付物建立 task-scoped
`OAW Engagement`。相关 follow-up 继承该 Engagement；无关交付物仍使用原生 Host。随后
OAW 先执行保证等级预检（Assurance Preflight），再对已激活任务分类：

```text
未显式激活 -> 原生 Host
    -> 保证等级预检
    -> DIRECT / BOUNDED / WORKFLOW
    -> 协作式或机器支撑的执行
```

Assurance Level 与 Request Mode 相互独立。仅有指令分发能力的 Host 使用
`policy-cooperative`；当前 Host-native integration 可能支持 `core-backed` 或
`coordinator-backed`。激活后的 `DIRECT` 处理一个小型、可恢复变更；
`BOUNDED` 处理一个由用户选择的 Capability 和一个命名交付物，它不是原生
Host Skill routing；`WORKFLOW` 运行选择 gate。没有超时自动选择，也没有静默默认项。
机器支撑的选择可以编译 Lifecycle Bundle；`policy-cooperative` 使用显式
Profile candidate、`CURRENT`、协作式 Policy Workflow Plan 和 Progress Tracker，不冒充机器保证。

## 无 Bridge Policy Workflow

参考 Codex Policy CLI 无需安装 Bridge 即可提供完整的协作式 `CURRENT` 路径：

```bash
oaw profiles
oaw use --profile MATT-SP-HYBRID \
  --complexity ordinary --risk normal -- "deliverable"
oaw status
```

`profiles` 报告 `policy_selectable`、`host_routable`、精确 `missing` route 与条件式
incident 可用性。`use` 要求 active Host 已经给出的 cooperative complexity 与 risk
assessment；它建立 Policy Workflow Plan 和 Progress Tracker，随后由 reducer 推导每个
next Skill、Host action、gate、review outcome、incident return、switch boundary 与 terminal
state。caller 不提供 slot、work reference 或自由文本 next action。

Codex route inspection 会识别 `.agents/skills` 下的 Matt Skills、
`.codex/plugins/cache/ecc/ecc/<version>` 下的 ECC Skills，以及
`.codex/plugins/cache/openai-api-curated/superpowers/<version>` 下的 curated
Superpowers Skills。当这些 route 存在时，`SP-FULL`、`MATT-FULL`、`ECC-FULL` 与
`MATT-SP-HYBRID` 都可以在不安装 Bridge 的情况下被选择和路由。缺失的条件式 incident
handler 会单独报告，并且只在该 incident 实际发生时停止。

这条路径仍是 `policy-cooperative`。它不声称拥有 verified Provider Instance、Lifecycle
Bundle、Capability Grant、Resource Lease、Host Receipt、atomic revision、idempotency 或
enforced recovery。Bridge 是可选的机器保证 integration，不是日常 Policy 执行的前置条件。

## 生命周期配置

| Profile | 生命周期契约 |
| --- | --- |
| `MATT-FULL` | Matt 主导 `oaw/domain-engineering`；精确的 workspace、广义验证和 closeout 缺口由 neutral Host action 补足。 |
| `SP-FULL` | inline Superpowers `oaw/delivery`，使用真实的 planning、TDD、debugging、review、verification 与 finish skill。 |
| `ECC-FULL` | ECC 主导 `oaw/ecc-engineering`；只有 Host 精确观察到的 Skill、Agent、Role 或 Instruction alternative 才可编译。 |
| `MATT-SP-HYBRID` | 保留的 Matt/Superpowers 组合；ECC 只可作为显式选择的 typed Add-on。 |
| `USER-DEFINED` | 选择配置中版本化的用户自定义 Profile Recipe；它不是第五个内置 alias。 |

推荐项永远不会变成默认项。缺少所需 provider capability 时，任务门禁会停止，不会
静默省略或替代。Superpowers、Matt、ECC 和第三方 Provider 使用同一个可扩展 Provider
与 Capability 模型。Host-native Subagent 继承完全相同的 locked bundle，不重新进行
family arbitration。bounded add-on 只能产出声明的 specialist 交付物，不能接管生命周期。
`DIRECT` 和 `BOUNDED` 不会创建 Workflow State。

通用有序生命周期是 `problem-framing` -> `solution-specification` ->
`delivery-planning` -> `workspace-preparation` -> `implementation` ->
`implementation-tdd` -> 条件式 `incident-recovery` -> `review-remediation` ->
`fresh-verification` -> `closeout`。每个 Recipe 必须为每个适用 slot 解析一个 outcome
owner，并包含 neutral Host action 与 gate。`FULL` 绝不把 Host 所有权交给 Provider。

## Matt-Superpowers 混合配置

初始三 family 分数是基于经验的设计输入，不是通用 benchmark。它们是会随版本变化的
判断，依据包括流程完整性、正确性纪律、歧义处理、复核闭环、验证强度和运维开销。
下表按 **Superpowers / Matt / ECC** 顺序列出；详细对比会记录其局限。

| 阶段 | 分数（SP / Matt / ECC） | `MATT-SP-HYBRID` 所有者 |
| --- | --- | --- |
| 规划 | 4.8 / 5.0 / 3.8 | Matt 负责需求、领域建模、规格和复杂任务拆分；Superpowers 负责每个 ticket 的可执行计划 |
| 实现 | 5.0 / 4.2 / 3.7 | Superpowers |
| TDD | 4.8 / 4.9 / 4.1 | Matt `tdd` |
| 调试 | 4.7 / 5.0 / 2.8 | Matt `diagnosing-bugs` |
| 复核 | 5.0 / 4.8 / 4.4 | Superpowers |
| 完成 | 5.0 / 3.6 / 4.0 | Superpowers |

workspace 和 Git setup 归 Superpowers。build、dependency 或 type repair 只归显式选择的
ECC resolver；也可以不选择 ECC resolver。specialist 检查只以精确的 bounded add-on
运行。这样每项职责都恰好只有一个所有者。精确序列为 Matt `grill-with-docs` ->
`to-spec` -> `to-tickets`，Superpowers `superpowers:writing-plans` ->
`superpowers:using-git-worktrees` -> inline `superpowers:executing-plans`，Matt `tdd`
与 `diagnosing-bugs`，随后是 Superpowers review/remediation、
`superpowers:verification-before-completion` 与
`superpowers:finishing-a-development-branch`。

## 支持的目标

Target ID 是稳定的 CLI 输入，必须严格按下表书写。Core adapter 同时支持用户和项目
作用域。Extension adapter 在项目作用域获得正式支持，因为它们的全局入口可能由 GUI
管理、依赖平台、仍属实验性，或稳定性较低。

| Target ID | Agent 工具 | 用户作用域 | 项目作用域 | Control surface |
| --- | --- | --- | --- | --- |
| `claude` | Claude Code | 是 | 是 | `policy` |
| `codex` | Codex CLI | 是 | 是 | `policy` |
| `gemini` | Gemini CLI | 是 | 是 | `policy` |
| `opencode` | OpenCode | 是 | 是 | `policy` |
| `cursor` | Cursor | 否 | 是 | `policy` |
| `windsurf` | Windsurf / Devin rules | 否 | 是 | `policy` |
| `cline` | Cline | 否 | 是 | `policy` |
| `roo` | Roo Code | 否 | 是 | `policy` |
| `copilot` | GitHub Copilot | 否 | 是 | `policy` |

用户作用域默认选择 `claude,codex,gemini,opencode`；项目作用域按注册表顺序默认选择
全部 target。不支持的 target/scope 组合或未知 ID 会在变更前失败。Provider 检测和
target readiness 都只是诊断信息，不会选择 lifecycle profile。

## 安全模型

- OAW 不安装 Superpowers、Matt Pocock skills 或 ECC。Provider 的许可、安装、配置和
  更新始终保持独立。
- Agent Host 保留物理权限。OAW Grant 与 Resource Lease 只协调合作客户端，不会替代
  Host sandbox 和 approval。
- 选定的本地 checkout 或已解压 release binary 都是可执行代码，必须经过复核并可信。
  Management 在运行时不会下载可执行文件。
- 第一次 managed write 前会准备并验证全部目标路径。
- 已有指令文件只加入一个带校验和的 OAW block；extension 的 owned file 单独管理。
  marker comment 是机械所有权边界，不是模型优先级控制。
- 路径被限制在选定的用户根目录或项目根目录中，并拒绝 symlink 重定向；state 作为惰性
  数据解析。
- 检测到 drift 时，会在变更前关闭失败。
- `--force` 会在变更前先备份所有受影响构件。它不会绕过无效 state 或路径 containment
  检查。
- `--dry-run` 只预览计划变更，不写入 managed content、state、backup 或 target
  directory。

报告契约和 installer trust boundary 请参阅 [SECURITY.md](SECURITY.md) 或
[SECURITY-zh.md](SECURITY-zh.md)。

## 更新与卸载

更新使用运行中 binary 嵌入的 Policy、Version、registry 与 rendering behavior。修改源码
checkout 后必须重新构建 `./oaw`；release archive 已包含预期 binary。v0.1 不包含
self-update、远程主分支获取、包管理器更新或 provider 更新。输入相同的重复安装或更新
是幂等操作。

默认情况下，OAW managed content 的变化会被报告为 drift，并在任何写入前阻止整个
操作。先检查诊断并运行 dry run。只有明确希望替换或删除 drift 时才使用 `--force`；
系统会先创建完整的 operation-scoped backup。

卸载会删除 managed block 和 OAW-owned file，保留无关用户字节，并且只清理 OAW
实际创建且仍为空的目录。canonical policy 会保留到最后一个有效安装引用被删除。目标
选择支持部分卸载。

## Provider 前置条件

在选择需要某个 workflow provider 的 profile 前，请通过该 provider 自己的可信渠道进行
安装和维护。OAW 的 `check` 命令会分别报告 Superpowers、Matt 和 ECC；所需 indicator
不完整时，该 provider 会显示为 `missing`。OAW 不下载、
vendor、patch、更新、删除、许可或静默替换 provider 内容。Agent 工具本身也需要单独
安装。

`oaw catalog list providers` 只列出声明的 descriptor。要在不修改配置的情况下检查已安装
的 Provider candidate 与 Host 验证结果，请运行
`oaw providers inspect --host codex --format text`。歧义结果会列出全部 candidate 以及精确
的 location-and-version `[[provider_pins]]` 片段；OAW 不会选择 candidate，也不会写入 pin。
写入 pin 后必须启动新的 Workflow，使其捕获新的 Configuration Snapshot。恢复步骤见[生命周期指南](docs/zh/lifecycle.md)
和[故障排查指南](docs/zh/troubleshooting.md)。

## 文档

每份详细指南都有语义对等的英文和中文入口。这些指南正在本地 v0.1 文档 ticket 中完成：

| 主题 | English | 简体中文 |
| --- | --- | --- |
| 背景 | [English](docs/en/background.md) | [中文](docs/zh/background.md) |
| 对比 | [English](docs/en/comparison.md) | [中文](docs/zh/comparison.md) |
| 生命周期 | [English](docs/en/lifecycle.md) | [中文](docs/zh/lifecycle.md) |
| 架构 | [English](docs/en/architecture.md) | [中文](docs/zh/architecture.md) |
| 安装器 | [English](docs/en/installer.md) | [中文](docs/zh/installer.md) |
| Codex Host Bridge | [English](docs/en/codex-bridge.md) | [中文](docs/zh/codex-bridge.md) |
| 适配器 | [English](docs/en/adapters.md) | [中文](docs/zh/adapters.md) |
| 扩展适配器 | [English](docs/en/extending-adapters.md) | [中文](docs/zh/extending-adapters.md) |
| 安全模型 | [English](docs/en/security.md) | [中文](docs/zh/security.md) |
| 故障排除 | [English](docs/en/troubleshooting.md) | [中文](docs/zh/troubleshooting.md) |

规范性工作流位于 [policy/ENGINEERING.md](policy/ENGINEERING.md)。详细指南只解释该策略和
实现，不会取代它。

## 贡献

请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 或
[CONTRIBUTING-zh.md](CONTRIBUTING-zh.md)。贡献必须保持 Bash 3.2 兼容、black-box CLI
测试 seam、English/Chinese 对等、adapter evidence、provider independence 和禁止远程
发布的边界。社区行为约定见 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。

## 许可证

OAW 使用 [Apache License 2.0](LICENSE)。Workflow provider 和 agent 工具仍受它们各自
许可证约束。

## 项目状态

源码基线已于 2026-08-14 固定为 v0.1.0。可以在本地构建跨平台归档，release readiness
按当前可用的原生/Docker 验证矩阵判定。当前仓库状态不是已发布的远程 release，本次
变更也不会创建 tag、package、domain 或全局保留名称。任何远程发布与 tag 创建都需要
所有者另行批准。
