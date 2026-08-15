# 架构

[English](../en/architecture.md) | [README 中文](../../README-zh.md)

Open Agent Workflow（OAW）分发一份 canonical 工程策略、编译无冲突生命周期契约，并
可选地协调持久化 Workflow State。它不拥有 Agent 执行权。Agent 工具和工程 Provider
保持独立安装与版本管理。

## 组件与边界

产品包含四个权限相互分离的模块：

1. **Distribution** 安装 user Policy 或完整的 project-scoped Canonical Policy Set，
   同时安装惰性 target-native Activation Router，并管理带 checksum 的 Install State 与 backup。
2. **OAW Core** 是无状态的。只有在显式激活且存在当前 Host-native evidence 后，
   它才分类请求、解析 verified Provider Instance、编译 Profile Recipe，并创建
   immutable Lifecycle Bundle。
3. **Workflow Coordinator** 是可选且只服务 Workflow 的。它为合作客户端记录 revision、
   idempotency、协作式 Resource Lease、evidence、cancel、switch 与 recovery。
4. **Agent Host** 位于 OAW 外部。它拥有 Agent、model call、MCP、Hook、Skill、Plugin、
   认证、工具、sandbox、approval 与所有物理 effect。

主要控制流为：

```text
顶层用户请求
    -> Activation Router
       -> 原生 Host
       -> 已激活 OAW
          -> 保证等级预检（Assurance Preflight）
             -> policy-cooperative -> Agent Host
             -> core-backed -> OAW Core -> Lifecycle Bundle -> Agent Host
             -> coordinator-backed -> OAW Core -> Workflow Coordinator -> Agent Host
```

始终生效的 frontmatter 只是 Host metadata，不是激活信号。Distribution 和 Bridge 安装都不会
启动工程生命周期。OAW Core 不保留 Workflow State。Workflow Coordinator 不执行工作。
Agent Host 也无权改写 Bundle。

## Policy 与 Machine Profile Projection

OAW 保留一份 authority-neutral 生命周期语义，并把它投影到两个相互独立的实现：

```text
共享 Profile 语义
  alias、responsibility、gate、macro credit、incident、switching
        |
        +-- Policy Projection
        |     Host-visible 与 user-explicit route
        |     neutral Host action 与 cooperative record
        |
        +-- Machine Projection
              verified Provider Instance 与 Binding
              Core compilation 与可选 coordination record
```

Policy Projection 由 Policy catalog、route inspector、lifecycle reducer、Engagement 与绑定
物理项目的 persistence module 实现。其 route input 只包含 route name 与 invocation mode。
它分别报告 `policy_selectable` 与 `host_routable`，根据 typed event 推导每个 next action，
并且绝不导入 discovery、integrity、Registry、Core、Coordinator 或 Bridge 权限。

Machine Projection 保留 Provider descriptor、source audit、完整 Binding-tree 验证、Core
compilation 与可选 Coordinator state。machine attestation 可以提高 assurance，但不能 veto
Policy Offer。因此 lockfile、distribution identity、installation path、revision、tree digest
与 Bridge state 都不能决定一个原本可路由的 Policy Profile 是否可以被选择。

四个内置 Profile 同时保留在两个 projection 中。所需 Codex route 存在时，`SP-FULL`、
`MATT-FULL`、`ECC-FULL` 与 `MATT-SP-HYBRID` 都可以在没有 Bridge 的情况下遍历协作式
`CURRENT` 生命周期。这只证明 route availability，不验证 Skill provenance、物理执行或
effect containment。

## Canonical 存储位置

OAW 遵循 XDG base-directory 约定，并保留明确默认值：

| 构件 | Canonical 路径 |
| --- | --- |
| User Policy Set | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/` |
| User Built-in Profile（受管） | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/builtin/` |
| User Custom Profile（用户拥有） | `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/profiles/`，不含 `builtin/` |
| Project Policy Set | `<project>/.oaw/policy/` |
| Project Custom Profile（用户拥有） | `<project>/.oaw/profiles/` |
| User 安装 state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/user.state` |
| Project 安装 state | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/installations/projects/<crc>-<bytes>.state` |
| 操作 backup | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/backups` |
| Workflow State | `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows` |

Install State 与 Workflow State 使用相互独立的 namespace；二者之间不迁移也不隐式接管。
`<crc>-<bytes>` 是物理 project root 路径字节的 `cksum` 结果，它隔离安装器 metadata，
同时不把这些 metadata 放入仓库。

项目 Workflow 文档是 committed Workflow State 的单向、非权威 projection，绝不会被
解析回权限来源。

当 `<project>/.oaw/policy/POLICY.md` 存在时，整套选择 Project Policy Set；否则选择
User Policy Set。Core Policy 文件绝不合并；project 与 user Custom Profile 仍可分别发现，
并保留各自 source identity。

## Distribution 数据流

Management mutation pipeline 是：

```text
embedded checkout policy -> pure renderer -> preflight/prepare -> required backup -> apply -> Install State/targets
```

Go binary 嵌入构建它的 checkout 中的 Policy、Version、registry 与 renderer behavior。
Release archive 已包含该二进制；源码 checkout 必须先构建 `./oaw`。这些箭头表示数据与
控制流，不表示 operation-wide 同步原子性。**pure renderer** 只把 prospective content
写入调用方提供的临时路径，不检查或修改最终 destination。

## Host-scoped Provider 权限

Provider 身份与权限沿以下链条流转：

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex 与 Claude Code 是独立 Host。即使共享物理 Provider 目录，由于 Host 和 surface
身份参与 digest，它们仍会产生不同 Host Installation key。Descriptor binding 和配置的
installation hint 只是声明。Host 必须报告精确 binding，并关联精确 installation，registry
才可以产生 Verified Provider Instance。

`policy` integration 可以分发指令并报告 Candidate，但 static detection 不能验证 Provider
Instance。`host-native` integration 可以报告无 secret 的 session fact 与 Host Binding
Evidence。foreign-Host diagnostics 绝不会进入 pin matching、Profile compilation 或
Lifecycle Bundle。active schema 会拒绝 `oaw.provider-descriptor/v1` 和
`oaw.user-config/v1`，不会静默升级。

Codex 的默认 Integration set 只包含 `oaw/codex-policy`。独立构建的 `oaw-bridge` 是可选
Assurance adapter，不是 host-native workflow surface。它的 `observe_profile` operation
可以把当前精确 `skill` Binding 身份附着到一份 source-qualified Markdown Profile。它不证明
topology、delegation、invocation、Host action、Receipt 或 completion，也不能通过
filesystem detection 提升 Policy surface。

v4 Provider 模型严格区分 Skill、Claude custom Agent、Codex Role、Instruction、Hooks 与
tools。Binding kind 为 `skill`、`agent`、`role`、`instruction`；完整 Binding tree evidence
必须位于精确 Distribution 与 Host Installation 下。共享祖先、相同名称、filesystem role
file 或 static multi-agent configuration 都不能创建 verified Binding 或证明 live
delegation。

## Core 编译

OAW Core 接收 request evidence、可信配置、Host session fact、verified Provider
Capability 和用户显式选择，返回 eligible Profile 与 topology、reason-coded exclusion、
recommendation 和 immutable Lifecycle Bundle。调用方绝不能自行构造 Bundle。

每个内置和用户 Provider 都使用相同 descriptor、binding 与 compiler 路径。Provider brand
不固定角色；selected Recipe 为每项职责分配唯一 owner。内置选择为：

| 选择 | Recipe |
| --- | --- |
| `SP-FULL` | `oaw/delivery` |
| `MATT-FULL` | `oaw/domain-engineering` |
| `ECC-FULL` | `oaw/ecc-engineering` |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` |
| `USER-DEFINED` | 配置中版本化的用户 Recipe |

`ECC-FULL` 仍是完整生命周期。同一个 ECC Provider 在其他 Recipe 中也可以只拥有一个
bounded specialist Capability。

Compiler hard cut 使用 Provider Descriptor `oaw.provider-descriptor/v4`、Profile Recipe
`oaw.profile-recipe/v3`、Execution Graph `oaw.execution-graph/v4`、Lifecycle Bundle
`oaw.lifecycle-bundle/v4` 与 Capability Grant `oaw.capability-grant/v3`。Workflow
command/result/snapshot/revision record 均为 v2。旧 authority record 不会被转换。

## 执行拓扑与 Host Integration

OAW 只识别两种执行拓扑：

- `CURRENT` 原样使用 active Agent session 及其环境。
- `SUBAGENT` 请求 active Agent Host 通过原生 Subagent facility 创建 child。

Subagent 不可用时，没有 process 或 container fallback。可用集合取 selected Profile、
所有 active Capability binding、integration metadata 与当前 Host session fact 的交集。
用户在 Workflow Startup Gate 中选择 topology。

Codex 默认暴露 `oaw/codex-policy`。`oaw-bridge` 需要独立构建、安装和测试；默认 `oaw`
安装器不会安装或管理它。Bridge 只读取当前 `skills/list` metadata 并返回可选 Assurance
Overlay。其他内置 integration 仍是 Policy surface，除非另一个 Host-native workflow
contract 被显式安装并验证。任何 integration 都不会向 OAW 转交 model command、
credential、private Hook payload 或 private MCP、Skill、Plugin 配置。

可选 Codex Assurance 路径为：

```text
Markdown Profile -> Policy 选择与规则驱动执行
       |
       +-> observe_profile -> 当前 Skill Binding evidence
                           -> 绑定同一 Profile 的 Assurance Overlay
```

Bridge 失败只会移除下方可选分支，绝不会改变 Policy Profile 或控制规则驱动分支。

Agent Host 拥有物理执行权限。Lifecycle Bundle、Capability Grant 与 Resource Lease 为
合作客户端表达 logical workflow authority。Grant 可以比 Host sandbox and approvals
更窄，但不能物理阻止 Host 在协议外执行 action。

## Workflow 协调

可选 Workflow Coordinator 只接受 `WORKFLOW`。`DIRECT` 与 `BOUNDED` 不创建 Workflow
State。它提交 immutable revision、Bundle generation 与 digest、current graph node、
ticket、stable boundary、logical Capability Grant、Resource Lease、Receipt 和
digest-pinned evidence reference。

Resource Lease 协调声明了冲突 project resource 的合作 Workflow。它不会锁定操作系统、
文件系统、Git 或其他 process。在 `policy-cooperative` 下，Host 可以建立 Policy Workflow
Plan、Progress Tracker、Execution Note 和 Conflict Warning。这些术语不能创建 verified Provider
Instance、eligible Profile、Lifecycle Bundle、Grant、Lease、Receipt、atomic revision、idempotency、
transition enforcement 或 recovery guarantee。

## Management Transaction

在 **prepare phase**，OAW 解析 source 与 destination，渲染 prospective file，检查
containment 与 symlink 规则，解析旧 state，检测 drift，并在写入任何 managed destination
前验证所有 planned action。Target selection 与 shared-destination collision 也在这里解决。

如果 forced mutation 会替换 drifted 或 foreign content，OAW 会先创建并验证覆盖每个受
影响构件的 **operation-scoped backup**，然后才应用 prepared action。Backup reference
可以记录到 Install State。

在 **apply phase**，路径会在使用前再次验证。每个文件先在 destination 旁写为临时文件，
再移动替换，因此提供 **atomic replacement per destination**。每个 effect 完成后，Go
manager 会在 mutation journal 中记录 inverse。发生已报告 apply failure 时，会尝试逆序
rollback；rollback failure 会显式返回。OAW 不承诺跨 destination 同步原子替换，也不
承诺从 process 或 machine crash 自动恢复。

## 所有权模式

每个 target 声明一种 ownership mode：

- `managed-block` 在保留周围用户内容的同时，插入 marker 分隔的 Activation Router。
  Claude、Codex、Gemini 和 OpenCode 使用此模式。
- `owned-file` 为 OAW 保留 adapter 专用的 Activation Router 文件。Cursor、Windsurf、
  Cline、Roo Code 和 Copilot 使用此模式。它们始终生效的 metadata 不会激活 OAW。

Marker 是安装器 ownership boundary；**marker 注释不建立模型优先级**，不会覆盖工具文档
规定的 instruction hierarchy，也不能强制 active Agent session reload。

Drift 表示已记录 OAW ownership 不再匹配当前文件。没有 `--force` 时变更会关闭失败。
Uninstall 删除干净的 managed block 或 owned file，只清理 state 能证明由 OAW 创建的
空目录。

## Install State Schema

Install State 是 tab-separated inert text，绝不会被 shell source 或 evaluate。它不是
Workflow State，也不能授予 workflow 权限。

| Record | 数量 | 含义 |
| --- | --- | --- |
| `format` | 一个 | State serialization format。 |
| `version` | 一个 | 已安装 OAW version。 |
| `scope` | 一个 | `user` 或 `project`。 |
| `project` | 仅 project | 物理 project root。 |
| `policy` | 一个 | 已安装主 Policy 的路径与 checksum metadata。 |
| `policy-file` | 零个或多个 | 每个 project Policy Set 文件的路径与 checksum metadata。 |
| `backup` | 可选 | 与最近相关操作关联的 backup manifest。 |
| `directory` | 零个或多个 | 由 OAW 创建、因此可能由它删除的目录。 |
| `target` | 一个或多个 | Target ID、绝对路径、ownership mode、checksum 与 origin。 |

Update 或 uninstall 前，OAW 会验证 record format、scope、project identity、target registry
membership、路径、ownership mode 与 checksum。Malformed 或不安全 state 会被拒绝。

## 信任模型

`HOME`、`XDG_CONFIG_HOME`、`XDG_STATE_HOME`、project root、checkout artifact、现有
target file 与 state 都是 trust-boundary input。OAW 要求绝对且安全的 root，以物理路径
解析 project scope，拒绝不安全 symlink 或 containment 路径，并在 apply 期间重新检查
destination。[安装器指南](installer.md)记录命令与失败；[适配器指南](adapters.md)记录每个
client-facing surface。
