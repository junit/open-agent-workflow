# 扩展 Adapter

[English](../en/extending-adapters.md) | [Adapter 证据](adapters.md) |
[贡献指南](../../CONTRIBUTING-zh.md)

Open Agent Workflow（OAW）adapter 是指向 canonical policy 的 thin、target-native
entrypoint。新增 adapter 是实现与证据变更，不是新的 workflow 设计。Adapter
**must not change lifecycle semantics**，也 **must not vendor a provider**。目标工具和
每个 workflow provider 都保持独立安装。

## Host Integration 是独立边界

Installation adapter 只暴露 `policy surface`。它为 `CURRENT` 分发 instruction，不保证
Workflow Coordinator、`SUBAGENT` 或 Receipt。Discovery evidence、Provider config、target
registration 与成功 installation 都不会把 adapter 自动晋级为 `host-native`。

Host-native integration 由 Agent Host 实现。它支持 secret-free session facts、topology
eligibility、Dispatch Packet 与 normalized Receipts，同时 Host 保留物理执行权限。它只
报告 fact 与 Receipt，不会给 OAW model command、credential、Hook payload。OAW 不会收到
private Plugin/MCP configuration。

Integration 可以报告 opaque policy digest，但 Host sandbox and approvals 仍具权威性。
Capability Grant 或 Resource Lease 只为合作客户端表达 logical workflow authority。该
contract 的 conformance evidence 不属于下方 adapter graduation level。

## 从 Verified Binding 组合 USER-DEFINED Recipe

`USER-DEFINED` selection 指向可信、版本化的 Profile Recipe。它可以在每个 canonical
lifecycle slot 自由组合已安装 Provider，但只能使用 Host 精确验证且兼容的 Binding。
Binding kind 为 `skill`、`agent`、`role` 与 `instruction`；Skill、Claude custom Agent、
Codex Role、Instruction、Hooks 与 tools 保持不同。Adapter 不得从其他 kind 或 Provider
brand 推断 Binding。

Recipe 声明每个适用 slot 的一个 outcome owner、alternative、overlay、typed incident
route、macro/internal-call credit、neutral Host action 与 gate、effect、resource、evidence
和 termination condition。Compiler 会拒绝 ownership 缺失或重复、未证明的 delegation
或 invocation、不兼容 topology 与 source drift。不存在 silent default；新增 Provider 或
Binding 依赖 data 与 evidence，而不是 brand-specific compiler branch。

## 从 Evidence Packet 开始

修改 registry 前先记录：

1. 工具名、拟议的小写 **target ID** 与 support level。
2. 精确的 user/project instruction path，并明确 **scope support**。
3. 工具文档化的 loader、import/reference 行为、instruction precedence，以及 reload 或
   session refresh 行为。
4. 每项 provider 声明对应的 **official primary source** URL，以及 `YYYY-MM-DD` 格式的
   retrieval date。
5. OAW 将避开的 experimental、version-sensitive 或 undocumented 行为。

二手教程可帮助发现资料，但不能作为契约证据。没有官方来源证明 instruction surface
稳定时，应把 adapter 保留为 candidate，而不是猜测 destination。

## Registry Metadata

[internal/management/targets.go](../../internal/management/targets.go) 中的
`targetRegistry` 是权威 management registry。已注册 adapter 必须一致地定义所有适用项：

| 字段或 helper | 契约 |
| --- | --- |
| `ID` 与 registry position | 添加唯一 target ID，并保持确定性的 normalization。 |
| `User` | 声明 user scope support，不静默跳过 unsupported scope。 |
| `UserSuffix` / `ProjectSuffix` | 为每个 supported scope 提供安全相对 destination。 |
| `Ownership` | 精确选择一种 ownership mode：`managed-block` 或 `owned-file`。 |
| `normalizeTargets` / `findTarget` | 从同一个 registry 解析默认值与显式 selection。 |

User-scope adapter 还需要在
[internal/management/paths.go](../../internal/management/paths.go) 中设置 allowed root
mapping。路径必须从已验证 root 向下推导；不能把未检查 CLI input 拼接到 destination。
Target ID、scope declaration、path mapping、ownership、renderer dispatch 与测试必须完全
一致。残缺的 registry entry 是 internal contract failure，不是 fallback 机会。

## 有意识地选择 Ownership

只有 provider 文档化的 surface 是 shared instruction file，且 install、update、uninstall
都必须保留无关 user byte 时，才使用 `managed-block`。Renderer 只提供 OAW fragment；
marker comment 定义 mechanical ownership，不定义 model precedence。

专用 OAW adapter path 使用 `owned-file`。Install 必须拒绝 pre-existing foreign file，即使
提供 `--force` 也一样。Update 与 uninstall 只有在 inert state 证明 OAW ownership 时才能
操作。不要把 owned file 放在预期由用户手工编辑的 provider path。

文档必须说明为何选定 surface 比 provider alternative 更稳定，以及它属于 user-wide
还是 project-local。

## 保持 Rendering 纯净

在 [internal/management/render.go](../../internal/management/render.go) 添加 **pure renderer**，
并通过 `renderTarget` 路由新的 scope/target 组合。它只能使用已验证 input，
并返回 prospective byte。它不得读取、创建、chmod、rename 或删除最终 destination；这些
effect 归 transaction code 所有。

对 output byte 做精确断言，包括 frontmatter、import syntax、quoting、final newline 与
canonical policy 绝对路径。只有 provider 定义 import 时才使用 documented import；否则
使用 model-visible bootstrap text，并明确这是 OAW choice。

## 解决 Shared Destination

两个 target 只有在同一 scope 使用相同 ownership mode，并生成 byte-identical OAW
fragment 时，才能指向同一个 **shared destination**。Codex 与 OpenCode 在 project-root
`AGENTS.md` 展示了这条规则：两个 state row 引用一个 managed block 和一个 checksum。

应覆盖两种安装顺序、joint update、selected uninstall、final uninstall、周围已有内容与
不一致 recorded checksum 的 fixture。如果 renderer 无法保持相同，就选择不同的文档化
路径或拒绝该组合。绝不能依赖 registry order 让一个 target 静默覆盖另一个。

## Black-Box Fixture

测试应通过 public `oaw` CLI，不要直接调用 renderer 或 state helper。另行证明
`install.sh` 会把相同 argument 与 status 转发给同目录 binary，并且没有 `PATH`、build 或
download fallback。每个 fixture 都使用 **isolated `HOME`**、`XDG_CONFIG_HOME`、
`XDG_STATE_HOME` 与 project root。支持的组合至少覆盖以下 observable flow：

- `check`、首次 install、重复 install、copied-checkout update、dry-run、selected
  uninstall 与 final uninstall；
- 默认选择和显式 `--target`，包括 registry-order output；
- 精确 destination byte、ownership mode、state row、origin 与 checksum；
- pre-existing user content、foreign owned file、clean ownership、missing content、drift
  与符合条件的 forced backup；
- provider-specific frontmatter、import/reference 行为、precedence caveat 与 reload 指南。

Core user adapter 放在 `tests/04-core-adapters-test.sh`；project adapter 与 shared-path 行为
放在 `tests/05-project-adapters-test.sh`。Expected byte 应使用独立 literal，不能在测试中
重现 renderer logic。

## Security Case

每个新 destination 都要扩展 black-box security matrix。至少覆盖 absolute 或
control-character root、包含空格和 shell metacharacter 的 **hostile path**、parent traversal
尝试、intermediate/final **symlink** replacement、project containment、foreign owned
content、适用时的 marker corruption，以及 forged 或 executable-looking **inert state**。

如果 adapter 引入新 parent tree，还要测试 apply-time path swap 与 directory ownership。
失败必须发生在任何 outside file 或 untracked destination 改变前。`--force` 不能绕过 state
schema、containment、symlink 或 ambiguous-ownership 检查。

## 文档与复核

在两种语言的 adapter matrix 中更新精确路径、support level、ownership、官方 URL、
retrieval date、loading behavior、precedence 与 reload caveat。运行离线文档检查、完整
Go 与 black-box suite，并保持包装器检查兼容 Bash 3.2。最终 diff 要检查 unrelated file、
hardcoded credential、unsafe expansion 与中英文 semantic drift。

## Graduation Level

演进路径是 **candidate -> project extension -> core**：

| Level | 准入条件 |
| --- | --- |
| Candidate | Evidence 与 fixture 正在复核；path 或 loader 仍是 speculative 时，不作为默认项，也不注册成 supported adapter。 |
| Project extension | 稳定的官方 project surface、pure rendering、project lifecycle fixture、adversarial path test 与双语 evidence 均通过；显式批准后才可加入 project default。 |
| Core | 稳定且文档化的 user/project surface、完整 lifecycle/security coverage、清楚的 precedence/reload 行为与维护承诺，足以支持 user default。 |

Graduation 是经过复核的 compatibility decision，不是 test-count threshold。Provider 变化后
可以降级 adapter，直到最新官方证据和 fixture 重新建立可信度。
