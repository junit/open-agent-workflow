# 生命周期选择与锁定

[English](../en/lifecycle.md) | [README 中文](../../README-zh.md)

本指南解释 Open Agent Workflow（OAW）的生命周期行为，但不会创建第二份策略。
policy/ENGINEERING.md 是规范来源；请阅读其中的
[canonical policy](../../policy/ENGINEERING.md)。如果这里的解释与该文件不同，以
policy 为准。

## 门禁何时运行

每个可能使用 workflow skill 的新顶层工程任务都要运行启动门禁。纯解释、状态报告、
只读查询，或不会启动工程生命周期的直接命令不运行该门禁。

选择前，OAW 只允许足以分类任务的只读检查，不开始 discovery、design、规划、实现、
TDD、调试、委派、Git 工作、复核或完成。这样可以在任何 family-specific owner 认领
交付物前形成阻塞式用户选择。

门禁顺序如下：

1. 读取 canonical policy。
2. 只检查足以分类任务的上下文。
3. 说明分类和具体证据。
4. 展示全部五种 profile，标注 recommendation，并列出精确的拟议 specialist add-on。
5. 等待用户显式选择；没有超时或静默默认项。
6. 生命周期工作开始前，记录并锁定选定 bundle。

Provider 检测只是诊断输入。它可以显示所需 family 缺失，但不能选择 profile。如果已选
profile 缺少必要 capability，工作会停止，直到用户安装它或选择另一 profile。

## 普通任务与复杂任务分类

**普通任务**是一个连贯交付物，需求大体明确，依赖有限，不含架构决策，只需一个实施
计划。普通 feature 或 refactor 通常推荐 `SP-FULL`，但用户可以选择任何有效 profile。

**复杂任务**具有较强领域性、歧义、规模或风险。判断信号包括未解决需求、领域探索、
多个子系统、migration、架构决策、多个交付 ticket，或较高的安全、数据、运维和爆炸
半径风险。证据不确定时，OAW 按复杂任务分类并说明理由。复杂工作通常推荐
`MATT-SP-HYBRID`。

分类只控制 recommendation 和规划深度，不会自动完成选择。

## 生命周期 Profile

每次门禁都展示全部 profile：

| Profile | 所有权契约 |
| --- | --- |
| `SP-FULL` | Superpowers 拥有从 discovery 到 branch completion 的完整生命周期；Matt 与 ECC lifecycle owner 保持暂停。 |
| `MATT-FULL` | Matt 拥有领域决策、规格、ticket、实现、TDD、调试、复核、commit 和完成证据；Superpowers 与 ECC lifecycle owner 保持暂停。 |
| `ECC-FULL` | ECC 拥有规划、实现、测试、build repair、复核、委派、验证和完成；Superpowers 与 Matt lifecycle owner 保持暂停。 |
| `MATT-SP-HYBRID` | Matt 与 Superpowers 使用下方固定阶段映射；精确 ECC specialist 只能作为 bounded add-on 选择。 |
| `CUSTOM-LOCKED` | 用户提供完整映射，每项适用职责只有一个 owner，transition 明确，add-on 有界。 |

有歧义的 `CUSTOM-LOCKED` 映射会被拒绝，不会靠猜测修复。Recommendation 始终明确标为
推荐项，不会隐藏成默认项。

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
| Build、dependency 与 type repair | 已选 ECC resolver，或无 |
| Spec compliance 与 code-quality review | Superpowers |
| Review remediation 与 re-review | Superpowers |
| 新鲜验证与 branch completion | Superpowers |
| Specialist 检查 | 仅限显式命名的 bounded add-on |

Matt 的规格和 ticket 是需求与交付边界的 canonical 来源。Superpowers 可执行计划可以补充
路径、命令和预期结果，但不能改变需求或 ticket 边界。

预期的 RED test 仍属于 TDD。非预期功能失败会把预期状态、命令和输出转交 Matt 调试。
严格的 build、dependency 或 type failure 只交给已选 ECC resolver。

## 生命周期锁与 Bundle 继承

**生命周期锁**记录任务身份、分类、已选 profile、选择来源、stage owner、精确 add-on、
active stage、active ticket 和 canonical artifact reference。它在整个交付期间持续有效，
跨后续请求、上下文压缩和委派工作保持不变。

**bundle 继承**表示每个 dispatched agent 都收到完全相同的 profile、stage map 和 add-on。
Agent 不重新开启 family arbitration，不添加第二 lifecycle owner，也不替换不可用 capability。
对于多 ticket 工作，**ticket 继承**会继续使用同一个 locked bundle，除非用户在允许的
边界明确修改。

## Bounded Add-on

bounded add-on 是为一个已声明交付物选择的精确 specialist capability。例如，
`ECC(security-review)` 可以产出安全报告，但在 `MATT-SP-HYBRID` 下不拥有实现、通用
复核、Git 工作或完成。报告交付后，控制权回到记录的 stage owner。

安全、覆盖率、风格或必需检查等 outcome constraint 本身不是 add-on，也不会选择
workflow。Active owner 仍负责满足这些约束。

## 稳定切换

只有用户能修改 lifecycle lock。稳定切换可发生在规格批准后、已完成 ticket 之间、
完成 TDD 或调试周期后、复核后，或记录验证后。委派工作进行中、merge 未解决时或
red-green cycle 未完成时不能切换。

切换会保留有效 artifact 并记录新选择；它不会追溯改写已完成所有权，也不会静默替换
provider。

## 完整 Locked-Bundle 示例

假设一个仓库需要多 ticket 安装器、路径 containment、可恢复 force、双语文档和最终
安全评估。

### 1. 分类

OAW 把它分类为**复杂任务**，因为需求横跨多个子系统和 ticket，文件系统变更有安全
影响，而且复核必须跨阶段闭环。OAW 推荐 hybrid，并提出一个 bounded security add-on。

### 2. 阻塞选择

用户看到全部五种 profile，并显式选择：

```text
MATT-SP-HYBRID + ECC(security-review)
```

这就是 selection source。不能仅因为该 bundle 获得推荐或检测到 provider 就开始工作。

### 3. 锁记录

记录的 bundle 指定 Matt 负责需求、规格、ticket、TDD 和困难 bug 调试；Superpowers
负责每个 ticket 的计划、实现、复核、remediation、验证和完成；ECC 只负责 bounded
security-review 报告。Active stage 与 ticket 指向 canonical 规格、ticket 和可执行计划。

### 4. Ticket 继承

实施从 Ticket 01 进入 Ticket 02 时，ticket 继承会复制完全相同的 bundle。Dispatched
implementation agent 遵循已批准的 Superpowers 计划，不再次要求用户选择 family。如果
测试暴露非预期功能缺陷，证据转交 Matt 调试，但 profile 不变。

### 5. Specialist 返回

在声明的安全检查点，`ECC(security-review)` 只产出报告。确认的 finding 返回
Superpowers remediation 与 re-review loop。ECC 不 merge、不 commit，也不认领通用
生命周期所有权。

### 6. Stable Boundary 切换

一个 ticket 完成并记录验证后，用户可以要求 stable boundary 切换，例如为同一交付物
中剩余的已批准 ticket 从 hybrid 切换到 `SP-FULL`。OAW 在该边界记录新选择，并保留
已完成规格、ticket、测试和 review evidence。用户显式提出前，原 bundle 始终保持
锁定。新的无关任务则会清除旧 lock，并重新运行启动门禁。

[背景文档](background.md)解释门禁存在的原因，[对比文档](comparison.md)记录 hybrid
背后的设计证据。
