# 设计对比

OAW 将三个经常被混在一起的问题分开：

| 问题 | OAW 所属 |
| --- | --- |
| 工程方法与默认行为 | 可读 Markdown Policy 和 Profile |
| Skill 流程 | 独立安装的 Skill |
| 物理执行与权限 | Agent Host |

安装器不是 workflow runner，只分发 Policy Set 和各 Host 的原生 activation router。Profile inspection
只是 advisory，不能选择方法，也不能证明 Skill 内容。

可选机器证据刻意设计为 additive。它可以让声明更精确，但证据缺失不会移除 Profile，也不会阻止
模型遵循可读规则。

这个设计以项目 dogfood 经验为依据，优先保持规则面小而易懂，不代表对其他 Agent 产品作普遍结论。
