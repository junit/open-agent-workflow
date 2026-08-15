# 模型驱动生命周期

生命周期是由 Policy 和选中 Profile 描述的对话契约，不是 CLI 状态机，也不要求 OAW 进程。

## 激活

只有当前顶层请求明确要求使用 OAW 后，OAW 才会激活。激活后加载一套 Policy Set，并为交付物选择
一个 Profile。相关 follow-up 继承选择；无关交付物按 Host 原生方式处理。方法确实需要变化时，
用户可以明确切换 Profile。

## Responsibilities

Profile 将 framing、planning、implementation、review、verification、closeout 等稳定的
Responsibility 映射到 Skill 或 Host 原生操作。模型拥有顺序，并可在新证据改变计划时重新处理某项
Responsibility。Profile 未声明的 Responsibility 使用 Policy 默认行为。

Complexity 与 Risk 是模型判断。Complexity 改变拆解和计划深度，Risk 改变 review、approval、
negative test 和 verification 强度。二者都不选择 Profile、不激活 OAW、不授予权限，也不创建机器记录。

## Skill 解析

每个声明的 Skill 使用 Host 原生调用面或可读 Skill 文档。Host 索引只是 advisory，可能不完整。
Skill 不可用时，使用 Profile 或 Policy 明确的替代方案，或说明限制并询问用户。不要臆造 Provider、
route、结果或 recovery owner。

## 进度与完成

对话是主要进度记录。项目可以保留可选 Markdown Progress Note，记录选中的 Profile、完成的
Responsibility、证据和下一步。它只是连续性辅助，不能成为权威控制状态或 gate。

完成意味着命名交付物已实现、按合适强度 review、fresh verification，并报告剩余风险。Git commit、
release 和 deployment 都是用户授权的动作，不是隐藏的生命周期阶段。

## 可选机器路径

可选 Machine Assurance 可以证明 Profile 内容或精确 Skill 身份，可选 coordinator 可以记录合作客户端
协同。这些组件只增加证据；缺失、失败或不可用都不能阻断正常 Policy 生命周期。
