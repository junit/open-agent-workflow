# 扩展 OAW

OAW 有两个扩展点：Markdown Custom Profile 和 Host Adapter。Custom Profile 不需要 Bridge、Provider
注册或修改 Go 代码；增加一个可安装的 Host target 还需要一小段 installer target 定义。

## Host Adapter

Adapter 说明某个 Host 如何加载 Canonical Policy Set：User 与 Project 指令路径、managed-block 或
owned-file 安装行为、Skill 索引、原生调用面、reload 时机和可读 fallback 路径。

Adapter discovery hint 不能判断 Profile 是否可用、证明 Skill 内容、选择方法或拥有物理执行权限。
Host 特定路径放在 `policy/adapters/`，不要放进 `POLICY.md`。只有需要 OAW 管理该 Host 的指令文件时，
才在 `internal/management/targets.go` 中增加安装坐标。

## Custom Profile

项目 Profile 放在 `.oaw/profiles/<id>.md`，User Profile 放在
`$XDG_CONFIG_HOME/open-agent-workflow/profiles/<id>.md`。使用 `id`、`name` front matter 和
Responsibilities 表格，按 Host 可见名称引用 Skill；不需要 Skill 时写中性的 Host-native action。

Custom Profile 有意是 partial 的，Policy 会为未声明的 Responsibility 提供默认行为。项目 Profile 只有
在明确从项目来源选择时才生效；同 ID 永不静默合并。

示例：

~~~markdown
---
id: release-readiness
name: Release Readiness
---

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| planning | ecc:blueprint |
| implementation | Policy default |
| review | ecc:security-review |
| verification | ecc:delivery-gate |
| closeout | Host-native closeout |
~~~

使用 `oaw profile check` 检查元数据和 warning。它不读取或验证 Skill 内容，也不选择 Profile。

## 设计规则

Profile 语义必须可移植。不要在 Profile 中放 Codex cache path、CLI 命令语法、Provider revision 或
machine digest；这些细节应放在 Host Adapter 或可选 Machine Assurance 组件。
