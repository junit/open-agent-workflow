# 架构

OAW 只有一个产品权威：静态、模型可读的 Canonical Policy Set。Go 二进制负责安装和检查，
不决定也不执行工程方法。

## 产品核心

Policy Set 包含：

~~~text
POLICY.md                  可移植规则和边界
cooperative-protocol.md    模型驱动的激活与进度过程
profiles/*.md              内置和用户组合的方法
adapters/<host>-policy.md  Host 特定的加载与发现指引
~~~

Policy 定义激活、Profile 选择、Responsibilities、默认行为、Skill 解析、安全边界和物理权限边界。
Profile 是 Markdown，不是编译后的图。Go 诊断和可选证据可以检查它，但不能替代它。

## 所有权

Agent Host 拥有 model call、Skill、Agent、工具、凭证、Plugin、MCP、Hook、sandbox、approval 以及所有物理效果。
OAW 不启动 model process，也不会把逻辑规则变成 sandbox。

安装管理只拥有 OAW 管理的文件和私有 Install State。项目 Policy Set 自包含，并优先于 User Set；
两者不会合并。Custom Profile 仍归用户或项目所有。

Machine Assurance 与 Bridge 是可选组件，可以增加精确内容、Skill 身份、证据或协同，但不能选择
Profile、调用 Skill、改变 Profile 语义或 veto 有效的 Policy 工作流。Offer eligibility 与物理执行
权限始终分离。

## 正常流程

1. 用户明确要求 OAW 处理一个交付物。
2. 模型加载一套 Policy Set 并选择 Profile，只有真实的 Profile 歧义才提问。
3. 某项 Responsibility 成为当前工作时，模型读取对应 Skill。
4. 模型使用 Host 原生工具完成工作，进度保存在对话中，也可以写入可选的 Markdown Progress Note。
5. 模型按照选中的 Profile 和 Policy 默认行为完成 review、verification 与 closeout。

安装后移除二进制不会移除项目或 Host 指令中的规则。移除可选机器组件只会移除机器证据声明。

## 扩展边界

Host Adapter 可以说明路径、索引、原生调用名称和重新加载行为，但不能增加第二个 Profile 目录、
route gate、状态机或 Host 特定的工程所有权。参见扩展 Adapter 和已接受的静态核心 ADR。
