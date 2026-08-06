# Request Mode、Profile 与生命周期锁

[English](../en/lifecycle.md) | [README 中文](../../README-zh.md)

本指南解释 Open Agent Workflow（OAW）的行为，不是第二份 policy。
`policy/ENGINEERING.md 是规范来源`；如果本指南与
[canonical policy](../../policy/ENGINEERING.md) 不一致，以 policy 为准。

## 三种 Request Mode

OAW Core 在选择工程方法前，对每个新顶层工程请求分类：

| Request Mode | 执行契约 | 生命周期选择 |
| --- | --- | --- |
| Direct Mode（`DIRECT`） | 主 Agent 完成小型、明确、可恢复的变更，并运行 focused verification。 | 无 |
| Bounded Mode（`BOUNDED`） | 一个精确 Provider Capability 产出一个可观察 terminal deliverable。 | 无 |
| Workflow Mode（`WORKFLOW`） | 编译后的 Profile Recipe 协调多项职责与阶段。 | 必需 |

Direct Mode 要求 change point 已知、需求明确、scope 有界、无未解决 architecture 或 domain
decision、不改变 public contract 或敏感语义，并且 verification boundary 已知。它不创建
Capability、Profile、Lifecycle Bundle、Startup Gate 或 Workflow State。

Bounded Mode 是 Atomic Skill mode。用户或 user-trusted rule 选择一个精确 Capability，
并声明 effect、resource、evidence 和 terminal condition。它不能拥有 planning、
implementation、general review、remediation loop、Git completion 或生命周期阶段。需要
第二个 Capability 或更广职责时必须重新分类。

Workflow Mode 适用于未解决需求或 root cause、domain 与 architecture decision、相互作用的
工程职责、public contract、schema、dependency、migration、敏感变更、多 ticket 或长期
委派。

只有 Workflow Mode 运行 Startup Gate。Complexity 与 Risk Class 会调整推荐和验证强度，
但不会为 Direct 或 Bounded 激活生命周期选择。`DIRECT` 与 `BOUNDED` 不创建 Workflow
State。

## Workflow Startup Gate

Workflow lifecycle 工作开始前，OAW：

1. 读取 canonical policy；
2. 只执行足以分类请求的只读检查；
3. 陈述 Request Mode、Complexity、Risk Class 与具体 evidence；
4. 通过 OAW Core 解析 verified Provider Instance；
5. 展示每个可用的内置与用户自定义 Profile、可用的 `CURRENT` 或原生 `SUBAGENT`
   topology、推荐、排除原因和拟议 bounded add-on；
6. 等待没有超时或静默默认项的阻塞式用户选择；
7. 由 OAW Core 编译并记录 immutable Lifecycle Bundle。

Provider detection 只是诊断输入，绝不选择 Capability、Profile 或 topology。所需
Capability 缺失或有歧义时停止选择，绝不静默省略或替换。

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

`oaw catalog list providers` 展示声明的 Provider descriptor。使用以下命令检查所选 Host
的动态 discovery：

```bash
oaw providers inspect --host codex --format text
```

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

内置 Profile alias 保持稳定：

| 选择 | Recipe | 所有权契约 |
| --- | --- | --- |
| `SP-FULL` | `oaw/delivery` | 所需 Capability 全部验证后，Superpowers 拥有完整交付生命周期。 |
| `MATT-FULL` | `oaw/domain-engineering` | 所需 Capability 全部验证后，Matt 拥有完整 domain-engineering 生命周期。 |
| `ECC-FULL` | `oaw/ecc-engineering` | 所需 Capability 全部验证后，ECC 拥有完整工程生命周期。 |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` | Matt 与 Superpowers 使用下方固定职责映射；精确 ECC specialist 保持 bounded。 |
| `USER-DEFINED` | 配置中的 Recipe ID | 选择版本化用户 Recipe 的动作，不是第五个内置 Profile。 |

`ECC-FULL` 包含 discovery、specification 与 planning、implementation、testing、debugging
与 build repair、review、delegation、verification 和 completion。ECC 不会被缩减为
hardening；其他 Recipe 中的 specialist 角色不削弱完整 `oaw/ecc-engineering` 选项。

Recipe 必须编译出每项适用职责唯一的 owner、明确 transition 与 terminal gate、bounded
add-on，以及不超出可信权限的 effect。有歧义的 Recipe 会被拒绝，不会靠猜测修复。

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

OAW Core 是必需且无状态的。它拥有 classification、Host-scoped Provider resolution、
eligibility、Profile compilation 和 Lifecycle Bundle construction。调用方绝不自行构造
Bundle。

Workflow Coordinator 是可选且只服务 Workflow 的。它为合作客户端记录 revision、
idempotency、协作式 Resource Lease、Receipt、evidence、pause、cancel、switch 与
recovery。它不创建 Agent、执行 model、调用 Skill、使用 tool 或强制 Host sandbox。

Agent Host 拥有物理执行权限。Lifecycle Bundle、Capability Grant 或 Resource Lease 只
表达 logical workflow authority。当前九个内置 Host integration 都使用 `policy` surface
并支持 `CURRENT`，不保证 Coordinator semantics。未来的 `host-native` surface 可以交换
session fact、Dispatch Packet 与 Receipt，但所有 effect 仍由 Host 执行。

## Matt-Superpowers 阶段映射

`MATT-SP-HYBRID` 为每项职责指定一个 owner：

| 职责 | Owner |
| --- | --- |
| 需求与领域建模 | Matt |
| 产品规格与验收标准 | Matt |
| Test boundary 选择与 ticket 拆分 | Matt |
| 每个 ticket 的可执行实施计划 | Superpowers `writing-plans` |
| Workspace 与 Git setup | Superpowers |
| 实现编排与代码变更 | 一个 Superpowers executor |
| TDD 方法与 red-green loop | Matt `tdd` |
| 功能性与困难 bug 调试 | Matt `diagnosing-bugs` |
| Build、dependency 与 type repair | 已选 ECC Incident Handler，或无 |
| Spec compliance 与 code-quality review | Superpowers |
| Review remediation 与 re-review | Superpowers |
| 新鲜验证与 branch completion | Superpowers |
| Specialist 检查 | 仅限显式选择的 bounded add-on |

Matt 规格与 ticket 是需求和交付边界的 canonical 来源。Superpowers plan 可以补充路径、
命令、代码步骤与预期结果，但不能改变需求或 ticket 边界。Matt `tdd` 是本 hybrid 唯一
TDD 流程。

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
revision。Policy-only lock 同样不是权威来源，不能授予物理执行权限。

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
