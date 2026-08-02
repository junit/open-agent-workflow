# 架构

[English](../en/architecture.md) | [README 中文](../../README-zh.md)

Open Agent Workflow（OAW）把一份 canonical 工程策略安装到受支持 agent 工具的指令
入口中。它不安装这些工具或它们的 workflow provider。安装器只拥有 policy、已记录的
adapter 输出，以及确实由它创建的目录。

## 组件与边界

仓库包含六个协作层次：

1. 当前 checkout 提供 `VERSION`、`policy/ENGINEERING.md`、target registry 和 pure
   renderer 函数。
2. 公开 Go CLI 解析 management command 并选择 user 或 project scope；`install.sh` 只执行
   预编译的同目录二进制。
3. 路径与 state 代码推导 canonical destination，并把现有安装记录作为 inert data 验证。
4. Transaction 代码准备每项变更、创建所有必需 backup，再应用已准备文件。
5. Adapter target 通过各工具自己的指令机制让已安装 policy 可见。
6. 可选的 Runtime Plane 提供 canonical Runtime Protocol、`oaw runtime exchange`，
   以及已选择的 `oaw run --host codex` Host driver。

Policy 是规范性的 workflow 来源。Adapter 文件是传输层，不是独立的 policy 副本。
Agent 工具、Superpowers、Matt Pocock skills 和 ECC 都保持独立安装与版本管理。

## Canonical 存储位置

OAW 遵循 XDG base-directory 约定，并明确保留默认值：

| 构件 | Canonical 路径 |
| --- | --- |
| 已安装 policy | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/ENGINEERING.md` |
| User 安装 state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/user.state` |
| Project 安装 state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/projects/<crc>-<bytes>.state` |
| 操作 backup | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/backups` |
| Runtime State | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/runtime` |

`<crc>-<bytes>` 是物理 project root 路径字节的 `cksum` 结果。这样每个已解析 project
root 都有隔离的 state record，同时不会把安装器元数据放进仓库。State 路径只是元数据
位置，不会扩大 project 安装可以拥有的仓库文件范围。

## 数据流

变更 pipeline 是：

```text
embedded checkout policy -> pure renderer -> preflight/prepare -> required backup -> apply -> Install State/targets
```

Go binary 嵌入用于构建它的 checkout 中的 Policy、Version、registry 与 renderer behavior。
Release archive 已包含该二进制；源码 checkout 必须先构建 `./oaw`。这些箭头表达数据与
控制流，不表示所有 destination 同时具备全局原子性。Renderer 接收已验证的值，只把
prospective content 写入调用方提供的临时路径。作为 **pure renderer**，它不会检查或变更
最终 destination。

## Runtime Plane

Runtime Plane 是可选层，不会替代 Policy Plane。Canonical `oaw.runtime/v1` transport
通过 `oaw runtime exchange` 提供；`oaw run --host codex` 使用同一个
`runtime.Engine.Exchange` seam，并在有界 Codex process 外围驱动有序的
`GRANT_ISSUED`、`DISPATCH_PREPARED`、`DISPATCH_AUTHORIZED` 与
`CAPABILITY_OBSERVED` handshake。目前只有固定版本的 Codex runner 是 Runtime-managed。
只有 pin 的 `runner-managed` integration `oaw/codex-runner` 可以通过 Runtime admission。
其他已安装 adapter 仍为 Policy-only，不提供 Runtime admission、Capability Grant、
Resource Lease、transition enforcement 或 physical isolation 保证。Discovery、
installation 与 project configuration 都不会将其晋级。

Install State 与 Runtime State 相互独立，不会自动迁移。现有 Policy-only task 和 profile
lock 仍保持 Policy-only，除非在 Stable Boundary 显式接管。Management 只读写 TSV
installation namespace；Runtime 只读写 revisioned Run namespace。

Host output 不可信。Codex driver 限制 process output，把 diagnostics 保持在 stderr，
把 JSONL 归一化为封闭的 outcome，并且只向 Runtime state 返回 digest-pinned evidence
reference。恢复运行时可以显式提供 project root，使可信的 project Configuration
Snapshot 继续参与 admission。

在 **prepare phase**，OAW 解析 source 与 destination，渲染 prospective file，检查
containment 与 symlink 规则，解析旧 state，检测 drift，并在写入任何 managed destination
前验证所有 planned action。Target 选择与 shared-destination collision 也在这里解决。例如
Codex 与 OpenCode 共用一个 project `AGENTS.md` managed block，不会竞争生成两个文件。

如果 forced mutation 会替换 drifted 或 foreign content，OAW 会先创建并验证覆盖每个受
影响构件的 **operation-scoped backup**，然后才应用任何 prepared action。对应 backup
reference 可以写入 state。

在 **apply phase**，路径会在使用前再次验证。每个文件先在 destination 旁写为临时文件，
再通过移动替换，因此提供 **atomic replacement per destination**。每个 effect 完成后，
Go manager 都会在 mutation journal 中记录 inverse。发生已报告的 apply failure 时，会尝试
逆序 rollback；rollback failure 会显式返回错误。OAW 不承诺跨 destination 的同步原子替换，
也不承诺从 process 或 machine crash 自动恢复。对于 forced operation，已验证 backup 仍是
可审计的恢复边界。

## 所有权模式

每个 target 声明以下一种 ownership mode：

- `managed-block` 在保留周围用户内容的同时，插入一个由 marker 分隔的 OAW block。
  Claude、Codex、Gemini 和 OpenCode 使用此模式。
- `owned-file` 为 OAW 保留一个 adapter 专用文件。Cursor、Windsurf、Cline、Roo Code
  和 Copilot 使用此模式。

Marker 是安装器的 ownership boundary；**marker 注释不建立模型优先级**，不会覆盖工具
文档规定的 instruction hierarchy，也不能强制正在运行的 agent session reload。发现、
优先级、合并以及 cache 或 session 行为仍由每个工具负责。

Drift 表示已记录的 OAW ownership 不再匹配当前文件。没有 `--force` 时，变更会关闭失败。
卸载时，干净的 managed block 从所在文件移除，干净的 owned file 则完整删除。只有 state
能够证明由 OAW 创建、并且当前为空的目录才可能被清理。

## State Schema

下表只描述 Install State。它由 tab-separated inert text 构成，绝不会被 shell source 或
evaluate。Runtime State 在独立 namespace 中使用 immutable Run revision、`HEAD`、Grant、
Resource Lease 与 evidence reference；它绝不会把 TSV Install State 解析成 Runtime
authority。

| Record | 数量 | 含义 |
| --- | --- | --- |
| `format` | 一个 | State serialization format。 |
| `version` | 一个 | 已安装 OAW version。 |
| `scope` | 一个 | `user` 或 `project`。 |
| `project` | 仅 project | 物理 project root。 |
| `policy` | 一个 | 已安装 policy 路径与 checksum metadata。 |
| `backup` | 可选 | 与最近相关操作关联的 backup manifest。 |
| `directory` | 零个或多个 | 由 OAW 创建、因此可能由它删除的目录。 |
| `target` | 一个或多个 | Target ID、绝对路径、ownership mode、checksum 与 origin。 |

Update 或 uninstall 前，OAW 会验证 record format、scope、project identity、target registry
membership、路径、ownership mode 和 checksum。Malformed 或不安全的 state 会被拒绝，
不会宽松解释。

## 信任模型

`HOME`、`XDG_CONFIG_HOME`、`XDG_STATE_HOME`、project root、checkout artifact、现有
target file 与 state 都是 trust-boundary input。OAW 要求绝对且安全的 root，以物理路径
解析 project scope，拒绝不安全的 symlink 或 containment 路径，并在 apply 期间重新检查
destination。[安装器指南](installer.md)记录命令与失败行为；[适配器指南](adapters.md)
记录每个 client-facing surface。
