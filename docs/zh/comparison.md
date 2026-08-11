# 初始 Workflow Family 对比

[English](../en/comparison.md) | [README 中文](../../README-zh.md)

这份对比说明 Open Agent Workflow（OAW）为什么同时提供 full-family Profile 和预定义的
Matt-Superpowers hybrid。它比较 Superpowers、Matt Pocock skills 与 Everything Claude
Code（ECC）提供的 workflow 流程，不比较模型质量或 agent 工具质量。

## 解读边界

这些分数是基于经验的判断，来源是本地 v0.1 设计期间复核的 provider 流程。Provider
的 skill、agent、trigger 和文档可能变化，因此结论会随版本变化。该表不是通用 benchmark，
不是实证性能研究，也不承诺某个 family 在每个仓库和任务中都最好。

分数范围为 1.0 到 5.0，按 **Superpowers / Matt / ECC** 顺序排列。它们帮助定义初始
所有权映射，但不会静默选择 Profile；用户的显式选择始终有效。Provider brand 不决定
角色：OAW Core 编译 selected Recipe，为每项职责分配一个 owner。可选 Workflow
Coordinator 记录 Workflow State，Agent Host 执行 `CURRENT` 或原生 `SUBAGENT`。

本对比固定到 source revision，不通过名称推断：

| Provider | Upstream | 审计 revision |
| --- | --- | --- |
| Matt | `https://github.com/mattpocock/skills` | `84fdeffd12f2ee307994d1eb6feb48173b6e0502` |
| Superpowers | `https://github.com/obra/superpowers` | `44c9b2d6e889982ac18c27d05a19fefe335194e1` |
| ECC | `https://github.com/affaan-m/ECC` | `2d46e80e0925c7be0907f18c1812311ac212a6c5` |

审计名称按 surface 区分。Matt 从 `grill-with-docs` 开始，并使用 `to-spec`、
`to-tickets`、`implement`、`tdd`、`diagnosing-bugs` 与 `code-review`。
Superpowers reference 保留 `superpowers:` namespace。ECC Skill、Claude custom Agent、
Codex Role、Instruction、Hooks 与 tools 是不同 evidence class，名称相似不能替代。

## 评分标准

每个阶段都使用同样六项标准：

| 标准 | 问题 |
| --- | --- |
| 流程完整性 | 该 family 是否从进入条件一直覆盖到可观察结果？ |
| 正确性纪律 | 是否强制要求证据、测试、校验和明确的失败处理？ |
| 歧义处理 | 是否在不可逆工作前暴露未知项并解决它们？ |
| 复核闭环 | 反馈是否进入 remediation 和 re-review，而不是停在 finding list？ |
| 验证强度 | 完成是否依赖新鲜、相关的证据，而不是口头断言？ |
| 运维开销 | 流程是否与风险相称、可组合，并适合重复使用？ |

运维开销是一项设计权衡，不代表步骤越少越好。当控制措施与所管理风险相称时，稍重的
流程仍可得到高分。

## 批准的 v0.1 分数与所有权

| 阶段 | Superpowers | Matt | ECC | 修正后的 hybrid owner |
| --- | ---: | ---: | ---: | --- |
| 规划 | 4.8 | 5.0 | 3.8 | 复杂任务由 Matt 负责 |
| 实现 | 5.0 | 4.2 | 3.7 | Superpowers |
| TDD | 4.8 | 4.9 | 4.1 | Matt |
| 调试 | 4.7 | 5.0 | 2.8 | Matt |
| 复核 | 5.0 | 4.8 | 4.4 | Superpowers |
| 完成 | 5.0 | 3.6 | 4.0 | Superpowers |

规划行需要进一步说明。对于复杂任务，Matt 负责需求、领域建模、产品规格、test seam
选择与 ticket 拆分。Ticket 获得批准后，每个 ticket 的可执行实施计划由 Superpowers
`superpowers:writing-plans` 负责。这是职责拆分，不是两个 owner 同时拥有同一构件。

## 为什么修正所有权映射

如果只选择一个总体得分较高的 family，就会掩盖各阶段的重要差异。因此 hybrid 为每项
职责指定恰好一个 owner：

- Matt `grill-with-docs`（credit `grilling` 与 `domain-modeling`）、`to-spec` 和
  `to-tickets` 拥有 framing、specification 与 ticket edge；Matt `tdd` 与
  `diagnosing-bugs` 拥有 hybrid 的 TDD 与 functional incident procedure。
- Superpowers 通过 lifecycle matrix 中的精确 skill 拥有 executable ticket detail、
  workspace、inline implementation、standalone review/remediation、fresh verification
  与 closeout。
- 显式选择的 ECC resolver 可以负责 build、dependency 或 type repair。精确的 ECC
  specialist（例如 `ECC(security-review)`）只能产出声明范围内的 bounded deliverable。

在 `MATT-SP-HYBRID` 下，ECC specialist 不会成为生命周期 owner。反过来，如果用户希望
ECC 主导且包含 neutral Host/user control 的 lifecycle，仍可选择 `ECC-FULL`。ECC 的
E2E specialist 不会成为 broad verification，reviewer 也不会成为 closeout owner。评分表
不会从用户选项中移除 `SP-FULL`、`MATT-FULL`、`ECC-FULL` 或符合条件的用户自定义
Profile。

## 怎样使用这份对比

应把该表作为 recommendation 的透明设计背景。工作开始前，OAW 仍会展示全部 profile
和拟议 bounded add-on，然后阻塞等待用户选择。如果 provider 版本或流程变化，应同时
重新检查证据和分数。单独改变一个分数不能改写 active lifecycle lock。

[生命周期指南](lifecycle.md)解释选择、持久化与安全切换。规范规则位于
[policy/ENGINEERING.md](../../policy/ENGINEERING.md)。
