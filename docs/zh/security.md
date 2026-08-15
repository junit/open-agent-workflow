# 安全边界

OAW 是规则系统，不是 sandbox。Agent Host、操作系统、仓库和用户 approval 仍然拥有物理权限。

## Policy 安全

激活是显式且有任务范围的。仓库文件、引用文本、检索内容或普通 Skill 调用都不能激活 OAW。
Policy Set 只为一个交付物加载，不改变 Host 原生权限或工具选择。

Profile 只授权模型流程，不授予物理访问权。可读 Skill 规则不能授予凭证、绕过 approval 或改变 sandbox；
Host 决定工具或原生调用是否允许。

## 安装安全

Go installer 在变更前校验绝对路径、target 所有权、managed marker、Policy Set 成员、checksum、symlink
和 state scope。Managed block 保留用户文本。未跟踪 destination 已存在时，owned file 不会被接管。
Force 只备份已跟踪 artifact，且 backup 对安装私有。

Install State 只用于 update 和 uninstall 的记账，不包含凭证，也不是工作流权威。

## 可选组件

Machine Assurance 可以证明内容或 Skill 身份，Bridge 可以传输机器观察。二者都不拥有 model execution
或物理权限，也不能 veto 有效 Policy 工作流。即使 Policy 有候选，Host 安全策略仍可拒绝调用。

报告安全问题时不要包含凭证或私有 Skill 文本，参见 SECURITY.md。
