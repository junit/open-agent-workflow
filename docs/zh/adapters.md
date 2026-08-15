# Host Adapter

Adapter 是某个 Agent Host 的安装与加载指引，不是工程方法，也不增加 Profile Responsibility。

## Adapter 契约

每个 Adapter 说明指令文件路径、scope 与 precedence、managed-block 或 owned-file 规则、reload 行为和
原生 Skill 调用面。可以说明可读 cache 位置作为 fallback，但不能要求特定 cache、lockfile、revision
或 digest 才能使用 Policy。

当前内置 Policy 指引是 Codex Adapter。其他 Host 通过自己的原生指令格式使用同一 Policy 语义。要增加
可安装的 Host target，应在 `policy/adapters/` 中加入 Adapter 指引，并在
`internal/management/targets.go` 中加入 destination 坐标。

## Target 所有权

Managed-block target 保留周围指令。Owned-file target 是 OAW 创建且不含用户内容的文件。Adapter 必须说明
每个 destination 的模型所有权，让 update 与 uninstall 保守执行。

## 分层

可移植规则属于 POLICY.md、cooperative-protocol.md 和 Profile。Host path 与调用细节属于 Adapter。
机器身份与 attestation 属于可选 Machine Assurance。分层可以避免 Host scanner 变成 Policy 依赖。
