# Codex Bridge

Codex Bridge 是可选的 Machine Assurance transport。SP-FULL、MATT-FULL、ECC-FULL 和 MATT-SP-HYBRID
都不依赖它。

## 范围

Bridge 观察当前 Codex session，并向 Machine Assurance 路径报告不含 secret 的当前 Host fact。
为已选 Markdown Profile 签发或验证身份 Overlay 的是 Machine Assurance，而不是 Bridge。Bridge 不选择
Profile、不读写项目文件、不调用 Skill、不管理进度、review、verification 或 completion，也不拥有权限。

因此 Bridge 缺失或失败只会移除 evidence overlay。正常 Policy 对话继续使用可读 Skill 和 Host 原生工具。
Host 安全策略仍可独立拒绝物理调用。

## 边界

Bridge protocol 和 cache discovery 保留在本文档及其实现包中，不要复制到 `POLICY.md`、Profile 或默认
`oaw` 命令。Host observation 是 evidence input，不是 Profile 语义或物理执行权限。
