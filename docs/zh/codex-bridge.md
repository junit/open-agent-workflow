# Codex Bridge

Codex Bridge 是可选的 Machine Assurance transport。SP-FULL、MATT-FULL、ECC-FULL 和 MATT-SP-HYBRID
都不依赖它。

## 范围

Bridge 可以观察当前 Codex session，为已经选中的 Markdown Profile 附加精确内容或 Skill 身份。
它不选择 Profile、不读写项目文件、不调用 Skill、不管理进度、review、verification 或 completion，也
不拥有权限。

因此 Bridge 缺失或失败只会移除 evidence overlay。正常 Policy 对话继续使用可读 Skill 和 Host 原生工具。
Host 安全策略仍可独立拒绝物理调用。

## 边界

Bridge protocol 和 cache discovery 保留在本文档及其实现包中，不要复制到 POLICY.md 或 Profile。Route name
不是 Provider provenance；可选 contract identifier 只描述 Adapter 面向的语义接口。
