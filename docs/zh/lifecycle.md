# 请求模式、Profile 与生命周期锁定

[English](../en/lifecycle.md) | [README 中文](../../README-zh.md)

本指南解释 Open Agent Workflow（OAW）的行为，但不会创建第二份策略。
`policy/ENGINEERING.md 是规范来源`；请阅读其中的
[canonical policy](../../policy/ENGINEERING.md)。如果本指南与该文件不同，以 policy
为准。

## 三种请求模式

OAW 会先分类每个新的顶层工程请求，再决定执行模型：

| Request Mode | 执行模型 | 是否选择生命周期 |
| --- | --- | --- |
| Direct Mode（`DIRECT`） | 主 Agent 直接完成小型、明确、可恢复的变更，并运行聚焦验证。 | 否 |
| Bounded Mode（`BOUNDED`） | 一个精确 Provider Capability 在已声明 effect、resource 和终止条件内产出一个可观察交付物。 | 否 |
| Workflow Mode（`WORKFLOW`） | 编译后的 Profile Recipe 跨多个职责和阶段进行协调。 | 是 |

Direct Mode 要求 change point 已知、需求明确、范围有界且可恢复，不存在未解决的架构或
领域决策，不改变公共契约或敏感语义，并且验证入口已知。它不会创建 Lifecycle Bundle，
也不会调用工程 Provider Capability。

Bounded Mode 就是 Atomic Skill 模式。用户或用户信任的精确规则只选择一个 Capability。
它不能认领规划、实现所有权、通用复核、remediation loop、Git completion 或任何生命周期
阶段。如果需要第二个 Capability 或更大的职责范围，就必须重新分类。

Workflow Mode 适用于需求或根因未解决、涉及领域或架构决策、多个工程职责相互作用、
公共契约、schema、dependency、migration、敏感变更、多个 ticket 或长期委派的任务。

只有 Workflow Mode 运行 Startup Gate。Complexity 与 Risk Class 仍会调整推荐和验证强度，
但不会为 Direct 或 Bounded 工作启动生命周期选择。

## Workflow Startup Gate

Workflow 生命周期开始前，OAW 会：

1. 读取 canonical policy；
2. 只执行足以完成分类的只读检查；
3. 说明 Request Mode、Complexity、Risk Class 与具体证据；
4. 展示全部可用的内置和用户自定义 Profile，标注推荐项，并列出精确的 bounded add-on；
5. 等待阻塞式用户选择，不设超时，也没有静默默认项；
6. 使用已验证 Capability 编译所选 Recipe，并记录 Lifecycle Bundle。

Provider 检测只是诊断输入，不能选择 Capability 或 Profile。所需 Capability 缺失或有歧义
时，选择必须停止，不能静默省略或替换。

## 可扩展 Provider 与 Capability 模型

Superpowers、Matt、ECC 和第三方 Provider 遵循相同契约。OAW 内置
`oaw/superpowers`、`oaw/matt` 与 `oaw/ecc` 的惰性 descriptor，但不会安装它们的 skill
内容。内置 Provider 仍会在当前 Host 上动态发现。

用户可以在配置中注册可信的第三方 Provider、声明式 discovery descriptor、Profile
Recipe、binding、pin 和 deny。可信项目配置可以推荐或收窄这些记录，但不能自行建立用户
信任或扩大权限。只有 verified Provider Instance 才能满足 Recipe 的 Capability selector。

Provider 的角色由 Recipe 决定。同一个 ECC Provider 在一个 Profile 中可以拥有完整生命
周期，在另一个 Profile 中可能只负责 build repair 或 security review。Full-family
eligibility 只取决于已验证的 Capability 覆盖，对内置和用户注册 Provider 使用同一规则。

### 检查 Provider 解析结果

`oaw catalog list providers` 展示声明的 Provider descriptor，不代表已经安装并
验证的 Provider Instance。使用以下只读命令检查指定 Host 的动态发现与验证结果：

```bash
oaw providers inspect --host codex --format text
```

当 Provider 存在歧义时，命令会列出全部 candidate 以及精确的 location-and-version
pin。OAW 不会替用户选择 candidate，也不会写入 pin。用户配置变化后必须启动新的
Run，使其捕获新的 Configuration Snapshot。

## 内置与用户自定义 Profile

以下内置 Profile alias 保持稳定：

| 选择 | Recipe | 所有权契约 |
| --- | --- | --- |
| `SP-FULL` | `oaw/delivery` | 所需 Capability 全部验证后，Superpowers 拥有完整交付生命周期。 |
| `MATT-FULL` | `oaw/domain-engineering` | 所需 Capability 全部验证后，Matt 拥有完整领域工程生命周期。 |
| `ECC-FULL` | `oaw/ecc-engineering` | 所需 Capability 全部验证后，ECC 拥有完整工程生命周期。 |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` | Matt 与 Superpowers 使用下方固定职责映射；精确 ECC specialist 保持有界。 |
| `USER-DEFINED` | 配置中的 Recipe ID | 这是选择版本化用户自定义 Recipe 的动作，不是第五个内置 Profile。 |

`ECC-FULL` 覆盖 discovery、specification 与 planning、implementation、testing、debugging
与 build repair、review、delegation、verification 和 completion。因此 ECC 不会被缩减为
hardening add-on；它在其他 Recipe 中的 specialist 角色不会削弱完整的
`oaw/ecc-engineering` 选项。

用户自定义 Recipe 必须能编译出每项适用职责唯一的 owner、明确 transition 与 terminal
gate、有界 add-on，以及不超过可信权限的 effect。有歧义的 Recipe 会被拒绝，不会靠猜测
修复。

## Matt-Superpowers 阶段映射

`MATT-SP-HYBRID` 为每项职责指定一个 owner：

| 职责 | Owner |
| --- | --- |
| 需求与领域建模 | Matt |
| 产品规格与验收标准 | Matt |
| Test seam 选择与 ticket 拆分 | Matt |
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

Matt 的规格和 ticket 是需求与交付边界的 canonical 来源。Superpowers 计划可以补充路径、
命令、代码步骤和预期结果，但不能改变需求或 ticket 边界。

Matt `tdd` 是本 hybrid 唯一的 TDD 方法。预期 RED test 仍在该循环内。非预期功能失败会
把预期状态、命令和输出转交 Matt 调试。严格的 build、dependency 或 type failure 只能
路由给已选 ECC Incident Handler。

## 隔离与 Host 保证

在 Runtime-managed Host 上，标记为 `isolated-required` 的 Workflow Capability 会在独立
Executor context 中运行。bundle 继承会把精确 Profile、Bundle generation、active graph
node、已准入 Capability、允许的 effect/resource、终止条件和 evidence requirement 交给每个
Executor。Executor 不能重新开启 family arbitration，也不能增加第二个 owner。

在 Policy-only Host 上，同一生命周期锁和 allowed-action map 只是指令级协调。OAW 不能在
这里声称具备 Runtime admission、Grant、Resource Lease、transition enforcement 或物理
隔离。Host 或 Agent 可以额外提供隔离，但那不是 OAW Runtime 保证。

## 生命周期锁、继承与 Add-on

生命周期锁记录任务与交付物身份、分类、所选 Profile、选择来源、Bundle generation/digest、
stage owner、精确 add-on、active stage、active ticket、允许和禁止的 action，以及 canonical
artifact 或 evidence reference。

该锁会跨后续请求、上下文压缩与委派持续有效。bundle 继承把它交给每个 dispatched
Executor；多 ticket 工作通过 ticket 继承继续使用相同 Bundle，直到用户在 stable boundary
切换。

bounded add-on 只授权一个精确 specialist 交付物。例如 `ECC(security-review)` 可以返回
digest-pinned 报告，但不会拥有实现、通用复核、Git 工作或 completion。安全和覆盖率要求
是约束，不是生命周期选择。

## Runtime 项目 Projection

Runtime-managed 项目 workflow 文件是 committed Runtime State 的人类可读下游视图。
Projection 包含 selected Profile、Bundle generation、stage、active ticket、digest-pinned
evidence reference 和 lag status；不包含 credential、完整 Grant、敏感 evidence 内容或原始
Provider output。

可选 active ticket 是独立的交付追踪引用，不会从 Workflow Deliverable ID 推导，也不会把
Deliverable ID 当作 ticket alias。

Projection 永远不会被解析回 authority。写入失败只记录 lag，不会回滚已提交 Runtime
revision。

## 稳定切换

只有用户能修改已选 Workflow Profile。稳定切换可以发生在规格批准后、已完成 ticket
之间、完成 TDD 或调试周期后、复核后，或记录验证后。active Capability invocation、委派
工作、未解决 merge 或不完整 red-green cycle 期间不能执行 stable boundary 切换。

切换会编译新的 Bundle generation、撤销旧 generation 的未完成 Grant，并保留有效 artifact
与 evidence。它不会追溯改写已完成所有权，也不会静默替换 Provider。

## 完整 Workflow 示例

一个仓库需要多 ticket installer、路径 containment、可恢复 force、双语文档和最终安全
评估。公共行为、文件系统风险、多个 ticket 与 remediation loop 使它被分类为 Workflow
Mode。OAW 推荐 hybrid，并提出一个 bounded add-on。

用户明确作出以下阻塞式选择：

```text
MATT-SP-HYBRID + ECC(security-review)
```

编译后的 Bundle 指定 Matt 负责需求、规格、ticket、TDD 和功能调试；Superpowers 负责
可执行计划、实现、复核、remediation、验证与 completion；ECC 只负责安全报告。每个
Executor 继承 Bundle 与 active ticket。ECC 报告返回 Superpowers remediation，不接管生命
周期。

一个 ticket 验证完成后，用户可以执行 stable boundary 切换，改用另一个可用的内置或用户
自定义 Profile。显式选择发生前，原 Bundle 始终保持锁定。

[背景文档](background.md)解释设计动机，[对比文档](comparison.md)记录初始 hybrid 的
experience-based 输入。
