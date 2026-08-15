# 故障排查

## OAW 没有激活

激活是显式且有任务范围的。可以用“Use OAW with SP-FULL to deliver this change”开头。
讨论 OAW、调用 Skill、仓库文本或任务复杂度本身都不会激活 OAW。

## Profile 或 Skill 没有被发现

发现结果是 advisory。使用 oaw profile list 或 oaw profile check 检查 Markdown 元数据，然后要求
模型直接读取指定 Skill。Codex 先使用当前 Skill 索引，再根据 Codex Adapter 回退到可读 Skill 文档。
即使生成的索引遗漏了 Matt 或 ECC，只要规则可读仍然可以使用。

不要为了让 Policy Profile selectable 而添加 Provider pin、cache path、lockfile digest 或 Bridge；这些
属于可选证据问题。

## Project 与 User Set

一个交付物只加载一套 Policy Set。project/.oaw/policy 优先于 User set，文件不会合并。Custom Profile
保留来源；两个来源同 ID 时使用 project:id 或 user:id。

## Install 或 Update 失败

使用相同的 project 和 target 参数运行 oaw check。常见原因：

- 已有 managed block 被编辑或重复；
- Policy Set 文件或 target 漂移；
- 未跟踪文件占用了 owned destination；
- scope 与 Install State 不一致。

显式处理或备份用户内容后再运行 update。Force 会备份已跟踪的漂移，但不会接管外部文件。

## Bridge 或机器证据缺失

正常路径不要求它们。Bridge 和 Machine Assurance 只增加证据，不能阻断 Profile 选择、Skill 使用、
review、verification 或 completion。Host 安全策略仍然可以拒绝物理调用；这与 Policy 选择是两件事。

## 提交问题

提供命令、scope、target 和完整诊断，不要提供凭证、token 或私有 Skill 内容。安装缺陷应在新的临时项目中
复现。
