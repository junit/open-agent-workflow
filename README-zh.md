# Open Agent Workflow（OAW）

[English](README.md)

Open Agent Workflow（OAW）负责协调多个 agent 工具中独立安装的 workflow provider。它为一个工程交付物指定一个明确的生命周期所有者，或一份无冲突的阶段
映射，然后把同一套治理策略安装到受支持 coding agent 的指令入口中。

OAW 是 provider-neutral 的策略、适配器层和零依赖 Bash 安装器。它不重新分发
workflow family，也不取代 agent 工具自身的配置。

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

- 在 family-specific 生命周期启动前，将顶层工程任务分类为普通或复杂任务。
- 展示全部五种 lifecycle profile，并等待用户显式选择。
- 让选定 bundle 跨后续请求、上下文压缩、ticket 和委派 agent 保持锁定。
- 支持完整 family profile、预定义 Matt-Superpowers hybrid、有限 specialist add-on，
  以及用户自定义的无冲突阶段映射。
- 独立检测 Superpowers、Matt Pocock skills 和 Everything Claude Code（ECC），检测
  本身不会替用户选择。
- 为九个 agent 工具安装用户级或项目级适配器。
- 提供幂等的 `check`、`install`、`update` 和 `uninstall` 生命周期，包含目标
  选择、dry run、drift 检查和可恢复的 force。
- 保留无关用户内容，只删除 OAW 所有的构件。

## 快速开始

请从已经复核的本地 checkout 运行，要求 Bash 3.2 或更新版本。不需要 Node.js、
Python、`jq`、包管理器、账户、token 或网络获取。

```bash
./install.sh check
./install.sh install
./install.sh install --project /path/to/repository
./install.sh update --dry-run
./install.sh uninstall
```

`check` 只报告 provider 检测、目标就绪情况和安装健康状态，不做变更。直接运行
`install` 时使用用户作用域和四个 core target；`--project` 选择一个已存在的仓库，
并默认使用全部九个 target。可以使用 `--target claude,codex`（或其他逗号分隔的
ID 集合）缩小命令范围。运行 `./install.sh --help` 可查看完整的本地 CLI。

## 任务门禁

OAW 通过足够的只读检查，把每个顶层工程请求分类为 `DIRECT`、`BOUNDED` 或
`WORKFLOW`。Direct Mode 由主 Agent 处理小型、明确、可恢复的变更；Bounded Mode 只为
一个可观察交付物准入一个精确 Provider Capability。这两种模式都不选择生命周期。

只有 Workflow Mode 运行 Startup Gate。OAW 随后展示全部可用的内置与用户自定义
Profile、一个推荐项和所有拟议 bounded add-on，由用户显式选择。
没有超时自动选择，也没有静默默认项。
编译后的 Lifecycle Bundle 会锁定到当前交付物。只有用户能切换它，而且只能在规格批准、
已完成 ticket、调试周期、复核或验证等 stable boundary 上切换。

## 生命周期配置

| Profile | 生命周期归属 |
| --- | --- |
| `SP-FULL` | Superpowers 拥有完整生命周期。 |
| `MATT-FULL` | Matt 拥有完整生命周期。 |
| `ECC-FULL` | ECC 拥有完整的 `oaw/ecc-engineering` 生命周期。 |
| `MATT-SP-HYBRID` | Matt 和 Superpowers 按下方显式阶段分工；声明的 ECC specialist 保持为 bounded add-on。 |
| `USER-DEFINED` | 选择配置中版本化的用户自定义 Profile Recipe；它不是第五个内置 alias。 |

推荐项永远不会变成默认项。缺少所需 provider capability 时，任务门禁会停止，不会
静默省略或替代。Superpowers、Matt、ECC 和第三方 Provider 使用同一个可扩展 Provider
与 Capability 模型。委派 agent 继承完全相同的 locked bundle，不重新进行 family
arbitration。bounded add-on 只能产出声明的 specialist 交付物，不能接管生命周期。

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
运行。这样每项职责都恰好只有一个所有者。

## 支持的目标

Target ID 是稳定的 CLI 输入，必须严格按下表书写。Core adapter 同时支持用户和项目
作用域。Extension adapter 在项目作用域获得正式支持，因为它们的全局入口可能由 GUI
管理、依赖平台、仍属实验性，或稳定性较低。

| Target ID | Agent 工具 | 用户作用域 | 项目作用域 | 支持级别 |
| --- | --- | --- | --- | --- |
| `claude` | Claude Code | 是 | 是 | Core |
| `codex` | Codex CLI | 是 | 是 | Core |
| `gemini` | Gemini CLI | 是 | 是 | Core |
| `opencode` | OpenCode | 是 | 是 | Core |
| `cursor` | Cursor | 否 | 是 | Project extension |
| `windsurf` | Windsurf / Devin rules | 否 | 是 | Project extension |
| `cline` | Cline | 否 | 是 | Project extension |
| `roo` | Roo Code | 否 | 是 | Project extension |
| `copilot` | GitHub Copilot | 否 | 是 | Project extension |

用户作用域默认选择 `claude,codex,gemini,opencode`；项目作用域按注册表顺序默认选择
全部 target。不支持的 target/scope 组合或未知 ID 会在变更前失败。Provider 检测和
target readiness 都只是诊断信息，不会选择 lifecycle profile。

## 安全模型

- OAW 不安装 Superpowers、Matt Pocock skills 或 ECC。Provider 的许可、安装、配置和
  更新始终保持独立。
- 选定的本地 checkout 是可执行代码，必须经过复核并可信。OAW 不获取或执行远程代码。
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

更新只从当前 checkout 读取构件。v0.1 不包含 self-update、远程主分支获取、包管理器
更新或 provider 更新。输入相同的重复安装或更新是幂等操作。

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

## 文档

每份详细指南都有语义对等的英文和中文入口。这些指南正在本地 v0.1 文档 ticket 中完成：

| 主题 | English | 简体中文 |
| --- | --- | --- |
| 背景 | [English](docs/en/background.md) | [中文](docs/zh/background.md) |
| 对比 | [English](docs/en/comparison.md) | [中文](docs/zh/comparison.md) |
| 生命周期 | [English](docs/en/lifecycle.md) | [中文](docs/zh/lifecycle.md) |
| 架构 | [English](docs/en/architecture.md) | [中文](docs/zh/architecture.md) |
| 安装器 | [English](docs/en/installer.md) | [中文](docs/zh/installer.md) |
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

此仓库是尚未发布、仅限本地的 v0.1 candidate。它不声称已有公开远程仓库、package、
release、domain 或全局保留名称。machine-readable status 保留为 post-v0.1 扩展。
v0.1 只输出 human-readable 状态。任何远程发布都需要所有者另行批准。
