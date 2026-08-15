# Machine Assurance

Machine Assurance 是面向需要机器可验证证据的部署的可选组件。它是静态 Policy 产品的 overlay，不是第二套
工作流定义。

它可以验证 Profile 字节、Skill 身份、Host observation 或合作客户端事件。记录必须不含 secret，并明确
自己的 scope。证据可以提高声明置信度，但不能决定 Policy Profile 是否存在，也不能 veto 有效的 Policy
工作流。

Agent Host 仍负责物理执行与安全策略。Assurance 失败只报告证据缺失；除非用户明确要求 assurance-only
交付物，模型可以继续正常 Policy 路径。

Schema、digest、lease 和 receipt 都应留在可选组件中。可移植 Policy 与 Profile 必须不依赖它们且保持可读。
