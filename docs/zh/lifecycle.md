# Request Mode、Profile 与生命周期锁

[English](../en/lifecycle.md) | [README 中文](../../README-zh.md)

本指南解释 Open Agent Workflow（OAW）的行为，不是第二份 policy。
`policy/ENGINEERING.md 是规范来源`；如果本指南与
[canonical policy](../../policy/ENGINEERING.md) 不一致，以 policy 为准。

## 激活与 Assurance

原生 Host 是默认状态。安装 OAW、任务复杂度、Host 自动选择 Skill，或直接调用一个
普通 Skill 都不会激活 OAW。只有当前顶层用户指令，例如 `/oaw <task>`，才会为一个
交付物建立 OAW Engagement。相关 follow-up 继承该 Engagement；无关交付物继续使用原生 Host。

每个已激活 Engagement 都先运行保证等级预检（Assurance Preflight）：

```text
未显式激活 -> 原生 Host
    -> 保证等级预检
    -> DIRECT / BOUNDED / WORKFLOW
    -> policy-cooperative / core-backed / coordinator-backed
```

Request Mode 描述已激活工作的契约；Assurance Level 单独描述当前 Host integration 能够
支持哪些声明：

| Assurance Level | 可用契约 |
| --- | --- |
| `policy-cooperative` | Cooperative Assessment、Host-visible candidate、协作式 Policy Workflow Plan、Progress Tracker、Execution Note 和 Conflict Warning；不能声称 Core 或 Coordinator record。 |
| `core-backed` | Core classification、Host-verified Provider resolution、reason-coded eligibility、显式选择 preview 与 immutable Lifecycle Bundle。 |
| `coordinator-backed` | 全部 `core-backed` 声明，加上 durable revision、Grant、协作式 Lease、normalized Receipt、transition 与 recovery state。 |

已安装文件、Provider descriptor 或 Bridge 安装本身都不会提升 assurance。机器支撑等级必须有
当前 Host-native session evidence。

## 三种 Request Mode

Request Mode 只存在于活跃 OAW Engagement 内。原生 Host 不是 Request Mode。

| Request Mode | 执行契约 | 生命周期选择 |
| --- | --- | --- |
| Direct Mode（`DIRECT`） | 主 Agent 完成小型、明确、可恢复的变更，并运行 focused verification。 | 无 |
| Bounded Mode（`BOUNDED`） | 一个精确 Provider Capability 产出一个可观察 terminal deliverable。 | 无 |
| Workflow Mode（`WORKFLOW`） | 编译后的 Profile Recipe 协调多项职责与阶段。 | 必需 |

Direct Mode 要求 change point 已知、需求明确、scope 有界、无未解决 architecture 或 domain
decision、不改变 public contract 或敏感语义，并且 verification boundary 已知。它不创建
Capability、Profile、Lifecycle Bundle、Startup Gate 或 Workflow State。

激活后的 Bounded Mode 不是原生 Host Skill routing。它只为一个命名、可观察交付物准入
一个已选 Capability，并声明 effect、resource、evidence 和一个 terminal condition。如果激活
请求明确命名 Skill 或 Capability，则为 `user-explicit`；否则 OAW 只展示一个 Host-visible
candidate，并以 `CAPABILITY_SELECTION_REQUIRED` 停止，直到用户确认。它不能拥有 planning、
implementation、general review、remediation loop、Git completion 或生命周期阶段。需要
第二个 Capability 或更广职责时必须重新分类。

Workflow Mode 适用于未解决需求或 root cause、domain 与 architecture decision、相互作用的
工程职责、public contract、schema、dependency、migration、敏感变更、多 ticket 或长期
委派。

只有 Workflow Mode 运行 Startup Gate。Complexity 与 Risk Class 会调整推荐和验证强度，
但不会激活 OAW。`DIRECT` 与 `BOUNDED` 不创建 Workflow State。

## Workflow 选择 Gate

在 `core-backed` 或 `coordinator-backed` 下，machine Startup Gate：

1. 读取 canonical policy；
2. 陈述 Assurance Level、Request Mode、Complexity、Risk Class 与具体 evidence；
3. 通过 OAW Core 解析 verified Provider Instance；
4. 展示每个可用的内置与用户自定义 Profile、可用的 `CURRENT` 或原生 `SUBAGENT`
   topology、推荐、排除原因和拟议 bounded add-on；
5. 等待没有超时或静默默认项的阻塞式用户选择；
6. 由 OAW Core 编译并记录 immutable Lifecycle Bundle。

在 `policy-cooperative` 下，Cooperative Selection Gate 展示 Host-visible Profile candidate，
声明不可用的机器保证，并说明执行留在当前 Host session；除非用户明确请求且存在
route-level contract，否则不使用 add-on。Gate 只等待显式 Profile 选择；这些固定执行事实和
已声明限制不是额外选择项。随后在 lifecycle 工作前建立协作式 Policy Workflow Plan 与
Progress Tracker。未验证 candidate 不得称为 eligible Profile；静态指令上下文也不能证明
`SUBAGENT` 可用。

Provider detection 只是诊断输入，绝不选择 Capability、Profile 或 topology。所需
Capability 缺失或有歧义时停止选择，绝不静默省略或替换。

### 无 Bridge 的静态 Policy 路径

默认 `oaw` 二进制没有 workflow transition、route admission、Provider catalog 或
Policy-run 命令。通过自然语言激活 OAW，并在同一个请求中指定 Profile，例如：
“使用 OAW 和 MATT-SP-HYBRID 完成交付。”未指定 Profile 时，Agent 会陈述合理选择并
继续；只有方法确实存在重大歧义时才提问。

Profile 选择不要求 topology、Add-on 哨兵、Complexity、Risk、Policy 限制、Provider
证据，也不需要逐个确认 Skill。`profile list`、`profile show` 与 `profile check` 是只读
Markdown inspection 命令，不选择或执行 Profile。旧的 `policy_selectable`、
`host_routable` 与 `incident_routes` observation 只是 advisory 信息，不能阻塞 model-led
Skill resolution。

Codex 先使用原生 Skill index，再按照 Codex Adapter 记录的位置延迟读取当前需要的指令。
当 index 或可选 Bridge 没有报告某个可读 Skill 时，该 Skill 仍然可用。模型通过正常会话
与规划状态维护进度；可选 Markdown Progress Note 只是连续性辅助，不是 authority 或 gate。

Machine Assurance 与 Bridge 可以增加精确证据，但缺失或失败只会移除这些机器 claim，
不会移除 Profile 选择、Skill 使用、复核、fresh verification 或 closeout。

旧 cooperative integration 仍可能报告以下停止原因。它们不是默认 CLI 命令，也不是静态
Profile 准入 gate：

| Reason | 恢复方式 |
| --- | --- |
| `CAPABILITY_SELECTION_REQUIRED` | 澄清需要的 specialist Skill。 |
| `POLICY_ONLY_PROVIDER_UNVERIFIED` | 去掉机器 identity claim，或使用可选 Machine Assurance。 |
| `POLICY_ONLY_PROFILE_INCOMPLETE` | 诊断实际不可读或不可调用的 Skill；scanner 输出本身不足以判断。 |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | 使用 Host 原生能力继续，或解释真实 Host 限制。 |
| `POLICY_ONLY_GUARANTEE_UNAVAILABLE` | 去掉机器保证要求，或使用拥有该保证的 component。 |
| `POLICY_ONLY_CONCURRENT_MUTATION` | 通过 Host 或仓库工作流串行化真实重叠的物理工作。 |
| `POLICY_ONLY_EXECUTION_UNCERTAIN` | 重试前先核对不确定的外部或破坏性 effect。 |
| `POLICY_ONLY_CONTEXT_UNCERTAIN` | 重新确认继续所需的重要决策或先前结果。 |

## Provider 与 Capability 模型

Superpowers、Matt、ECC 与第三方 Provider 使用同一 descriptor、binding、verification 和
compiler contract。OAW 为 `oaw/superpowers`、`oaw/matt` 与 `oaw/ecc` 提供 inert
descriptor，但不安装 Provider skill。内置 Provider 仍在当前 Host 上动态发现。

Provider 验证遵循精确链条：

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex 与 Claude Code 是独立 Host，因此共享物理文件仍会产生不同 Host Installation
identity。Descriptor binding 与 installation hint 只是声明。foreign diagnostic 绝不会
进入 pin、Profile compilation 或 Lifecycle Bundle。

用户可以注册可信第三方 Provider、discovery descriptor、Profile Recipe、binding、pin 和
deny。可信 project config 可以推荐或收窄记录，但不能创建用户信任或扩大权限。只有
verified Provider Instance 能满足 Recipe Capability selector。

Provider 的角色由 Recipe 决定。一个 Profile 可以让 ECC 拥有完整生命周期，另一个
Profile 可以只准入它的 build repair 或 security review。Full-family eligibility 对内置和
用户 Provider 使用相同 verified Capability coverage 规则。

### 检查 Provider 解析

默认 `oaw` 二进制刻意不提供 Provider catalog 或动态 resolution 命令。它的 Profile
inspection 命令只读取 Markdown identity 与结构。model-led Policy 执行从 Host 原生
surface 和可读指令解析 Skill；精确 Provider 与 Binding claim 属于独立构建的 Machine
Assurance component。

当前 Host Provider 有歧义时，会列出全部 Candidate 和精确 pin：

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

OAW 不选择 Candidate，也不写 pin。Active contract 会拒绝
`oaw.provider-descriptor/v1` 与 `oaw.user-config/v1`，不会迁移。用户配置变化后启动新
Workflow，使 OAW Core 读取新的 Configuration Snapshot。

## 内置与用户自定义 Profile

四个 alias 始终保留为 active catalog entry。当前 Host 排除某个 alias 不会删除它，也不
存在旧 schema fallback。

| 选择 | Recipe | 契约 |
| --- | --- | --- |
| `MATT-FULL` | `oaw/domain-engineering` | Matt 主导，并用 neutral Host/user control 填补 Matt 的精确缺口。 |
| `SP-FULL` | `oaw/delivery` | 完整的 inline Superpowers delivery path。 |
| `ECC-FULL` | `oaw/ecc-engineering` | ECC 主导，只使用精确的 Host-surface alternative。 |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` | 保留的内置 Matt/Superpowers 组合。 |
| `USER-DEFINED` | 配置中的 Recipe ID | 选择可信版本化 custom Recipe 的动作，不是第五个内置 alias。 |

`FULL` 指 Provider 主导的生命周期加 Provider-neutral Host action 与 user/Host gate，绝不
表示 Provider 拥有 Agent Host。每个适用 slot 必须有一个 outcome owner，且 Binding、
topology、delegation、invocation、action、authority、effect、resource、transition 与
terminal gate 全部验证后 Recipe 才能编译。

### Canonical 十 slot 生命周期

| # | Slot ID | Outcome 与 neutral control |
| --- | --- | --- |
| 1 | `problem-framing` | 目的、约束、领域术语、决策与成功条件；`shared-understanding` gate。 |
| 2 | `solution-specification` | 可复核规格与 test boundary；`specification-approved` gate。 |
| 3 | `delivery-planning` | 可独立验证的单元与执行计划；`delivery-plan-approved` gate。 |
| 4 | `workspace-preparation` | 安全 workspace 与已知 baseline；Host-owned 时使用 `workspace.prepare-or-confirm` action 与 `workspace-ready` gate。 |
| 5 | `implementation` | 获批的有界变更。 |
| 6 | `implementation-tdd` | 观测到预期 RED/GREEN cycle。 |
| 7 | `incident-recovery` | 条件式 typed recovery、replan 或 stop。 |
| 8 | `review-remediation` | finding 已裁定、修复并重新复核。 |
| 9 | `fresh-verification` | 与声明相关的新鲜输出；Host-owned 时用 `verification.execute` action 与 `fresh-evidence` gate。 |
| 10 | `closeout` | 已接受且用户授权的交付/保留结果；Host-owned 时用 `closeout.execute` action 与 `user-closeout` gate。 |

Host action 与 gate 不包含 Provider selector。Macro 的 `credit-only` call 只记录 enclosing
unit 已执行的工作；`dispatch-before` 与 `dispatch-after` 只在声明边界运行一次。重复或未
credited 的所有权以 `MACRO_INTERNAL_CONFLICT` 失败。

### 精确内置矩阵

| Slot | `MATT-FULL` | `SP-FULL` | `ECC-FULL` | `MATT-SP-HYBRID` |
| --- | --- | --- | --- | --- |
| `problem-framing` | `grill-with-docs`（`grilling` + `domain-modeling` 只 credited 一次） | `superpowers:brainstorming` | Codex `intent-driven-development` skill 或精确 Claude `architect` Agent | Matt `grill-with-docs` 与 credited internal call |
| `solution-specification` | `to-spec` | enclosing brainstorming outcome | `product-capability`；条件式 `contract-first` 不是 peer owner | Matt `to-spec` |
| `delivery-planning` | `to-tickets` | brainstorming 后只调用一次 `superpowers:writing-plans` | observed Codex `/plan` Instruction 或 `blueprint`；Claude `planner` Agent 或 `blueprint` | Matt `to-tickets` 后由 SP `superpowers:writing-plans` 增加文件/命令细节 |
| `workspace-preparation` | Host `workspace.prepare-or-confirm` | `superpowers:using-git-worktrees` | Host action；`git-workflow` 只是 guidance | SP `superpowers:using-git-worktrees` |
| `implementation` | `implement` macro | inline `superpowers:executing-plans` | `tdd-workflow` 或精确 Claude `tdd-guide` alternative | inline SP `superpowers:executing-plans`；SDD paused |
| `implementation-tdd` | `implement` 只 credit `tdd` 一次 | `superpowers:test-driven-development` | 与 implementation 相同的所选 implementation/TDD unit | Matt `tdd`；SP TDD paused |
| `incident-recovery` | `diagnosing-bugs` 仅处理 functional/hard/performance incident；其他类型 stop | `superpowers:systematic-debugging` typed route | 只有已验证 typed route；Claude `build-error-resolver` 可处理 build/type/dependency | Matt `diagnosing-bugs`；build/type/dependency 在未选择 ECC handler Add-on 时 stop |
| `review-remediation` | `implement` credit `code-review`；remediation 重新进入有界 `implement` 并 re-review | `superpowers:requesting-code-review` -> `superpowers:receiving-code-review` -> re-review | Policy：typed Host `review.execute`，findings 返回 `tdd-workflow`；machine-backed：精确 Codex `reviewer` Role 或 Claude `code-reviewer` Agent | SP request/receive/re-review；Matt review 与 SDD review paused |
| `fresh-verification` | Host `verification.execute` | `superpowers:verification-before-completion` | `verification-loop`；E2E surface 仅为 specialist | SP `superpowers:verification-before-completion` |
| `closeout` | Host `closeout.execute` | `superpowers:finishing-a-development-branch` | Host action 加 `git-workflow` guidance | SP `superpowers:finishing-a-development-branch` |

Matt 提供的是 `grill-with-docs`，不是虚构的 requirements 或 verification Skill；它不提供
workspace creation、广义 fresh verification 或 completion。ECC Skill、Claude custom
Agent、Codex Role、Instruction、Hooks 与 tools 是不同 surface。Claude Agent 名称不能
证明 Codex Role；static multi-agent configuration 不能证明 live delegation；
`e2e-runner`、`e2e-testing`、reviewer 与 delivery Hook 不会获得更广所有权。

`USER-DEFINED` Recipe 可按 slot 自由组合已安装、可信、Host-verified 且兼容的 Binding。
它必须 pin 版本和来源，声明精确 outcome owner、alternative、overlay、incident route、
internal call、action、gate、effect、resource 与 termination condition，并在任何缺口上
fail closed。用户自定义 SDD variant 只有在 active Host 证明 child 或 nested-child
delegation 后才能选择 `superpowers:subagent-driven-development`。

## 执行拓扑

Startup Gate 使用两个 topology 名称：

| Topology | 契约 |
| --- | --- |
| `CURRENT` | 在当前 Agent session 中执行，active Agent Host 环境保持不变。 |
| `SUBAGENT` | 请求 active Agent Host 通过原生 Subagent facility 创建 child。 |

Topology eligibility 是 selected Profile、Capability binding、Host integration 与 active
Host session 的交集。缺少原生 Subagent 支持时 `SUBAGENT` 不可用；OAW 不会替换成 shell、
model CLI、container 或 remote process。两者都可用时由用户选择；只有一个可用时记录原因。

Host environment observation 使用 `inherited`、`host-configured`、`restricted`、`unknown`
或 `unavailable`。OAW 不重建或保证未报告的 MCP、Hook、Skill、Plugin、model、认证、
sandbox、approval 或 tool 行为。

## OAW Core、Coordinator 与 Agent Host

协作式 Policy 路径不需要 OAW Core。对于机器支撑 assurance，OAW Core 是必需且无状态的。
它拥有 classification、Host-scoped Provider resolution、eligibility、Profile compilation
和 Lifecycle Bundle construction。调用方绝不自行构造 Bundle。

Workflow Coordinator 是可选且只服务 Workflow 的。它为合作客户端记录 revision、
idempotency、协作式 Resource Lease、Receipt、evidence、pause、cancel、switch 与
recovery。它不创建 Agent、执行 model、调用 Skill、使用 tool 或强制 Host sandbox。

Agent Host 拥有物理执行权限。Lifecycle Bundle、Capability Grant 或 Resource Lease 只
表达 logical workflow authority。Codex 默认提供 Policy integration。独立构建的
`oaw-bridge` 是可选 Machine Assurance，不是 host-native Workflow 路径。它可以为当前
精确 Skill Binding 签发 Assurance Overlay，但不选择 Profile，也不改变生命周期所有权。

可选 evidence 路径与执行相互独立：

```text
Markdown Profile -> 规则驱动 Policy 生命周期
       |
       +-> observe_profile -> 精确 Skill Binding 的 Assurance Overlay
```

Bridge 缺失或失败只会移除 Overlay。其他内置 integration 仍是 Policy surface，除非其自身
Host-native integration 被显式安装并验证。这些 logical record 都不会把物理执行权限从
Agent Host 转交出去。

## Matt-Superpowers 组合说明

上方矩阵是 `MATT-SP-HYBRID` 的权威 projection。Matt specification 与 ticket 仍是
domain intent 和 delivery edge 的 canonical 来源；Superpowers plan 增加 executable
detail，但不改变它们。Matt `tdd` 是唯一 TDD procedure，一个 inline Superpowers
executor 拥有 implementation，standalone Superpowers procedure 拥有
review/remediation、fresh verification 与 closeout。只有用户显式选择精确 bounded
Add-on 后才存在 ECC build/type handler。

## 生命周期锁、继承与 Add-on

生命周期锁记录 task 与 deliverable identity、classification、所选 Profile 和 topology、
selection source、Bundle generation 与 digest、stage owner、精确 add-on、active stage 与
ticket、允许和禁止 action，以及 canonical artifact 或 evidence reference。

该锁跨后续请求、context compaction 和委派工作持续有效。精确 bundle 继承把 active
Bundle generation、graph node、admitted Capability、effect、resource、terminal condition
和 evidence requirement 交给 Host-native Subagent。Child 不能重新进行 Profile
arbitration，也不能增加第二个 owner。多 ticket 工作使用 ticket 继承保持相同 Bundle，
直到用户在 stable boundary 切换。

bounded add-on 只授权一个精确 specialist deliverable。例如 `ECC(security-review)` 可以
返回 digest-pinned report，但不拥有 implementation、general review、Git 或 completion。
Security 与 coverage requirement 是约束，不是生命周期选择。

## Workflow State 与 Projection

Coordinator-backed 项目 Workflow file 是 committed Workflow State 的单向人类可读视图。
Projection 可包含 selected Profile 与 topology、Bundle generation、stage、active ticket、
digest-pinned evidence reference 和 lag status；排除 credential、完整 Grant、敏感 evidence
和 raw Provider output。

Projection 绝不会解析回 authority。Projection write failure 只记录 lag，不回滚 committed
revision。Policy-cooperative 工作改用人类可读的协作式 Policy Workflow Plan 和尽力维护的
Progress Tracker。它们都不是 Lifecycle Bundle、Lifecycle Lock 或 Workflow State，也不能授予
物理执行权限。

## 稳定切换

只有用户能修改 selected Workflow Profile 或 topology。稳定切换可以发生在 approved
specification、已完成 ticket 之间、完成 TDD 或 debugging cycle 后、review 后，或记录
verification 后。active Capability invocation、委派工作、unresolved merge 或 incomplete
red-green cycle 期间不能执行 stable boundary 切换。

切换会编译新 Bundle generation、撤销旧 generation 的 outstanding Grant，并保留有效
artifact 与 evidence。它不会改写已完成所有权、静默替换 Provider 或模拟不可用 topology。

## 完整 Workflow 示例

一个仓库需要 multi-ticket installer、path containment、recoverable force、双语文档与最终
security assessment。public behavior、filesystem risk、多 ticket 与 remediation loop 使它
成为 Workflow Mode。OAW 推荐 hybrid、`CURRENT` 和一个 bounded add-on。

用户显式选择：

```text
MATT-SP-HYBRID + ECC(security-review)
```

OAW Core 编译 Bundle。Matt 拥有 requirements、specification、ticket、TDD 和 functional
debugging；Superpowers 拥有 executable plan、implementation、review、remediation、
verification 和 completion；ECC 只拥有 security report。Agent Host 执行工作并返回
Receipt；Workflow Coordinator 是可选的。

一个 ticket 验证完成后，用户可以 stable boundary 切换到另一个可用 Profile 或 topology。
显式选择前，原 Bundle 保持锁定。

[背景文档](background.md)解释动机，[对比文档](comparison.md)记录初始 hybrid 的
experience-based 输入。
