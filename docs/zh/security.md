# 安装器安全模型

[English](../en/security.md) | [安全策略](../../SECURITY-zh.md) |
[架构](architecture.md)

本指南说明本地 Open Agent Workflow（OAW）安装器的安全控制与限制。它不表示不可信
checkout、操作系统、agent 工具或 workflow provider 是安全的。

## Trust Boundary

安装器把以下值与构件视为 trust-boundary input：

- 当前 checkout，包括可执行 shell code、`VERSION` 与 `policy/ENGINEERING.md`；
- CLI target 与 project argument；
- `HOME`、`XDG_CONFIG_HOME` 与 `XDG_STATE_HOME`；
- physical project root，以及选定 destination 下的每个 component；
- 现有 policy、adapter、state、directory 与 backup artifact。

只能运行可信 checkout。OAW **does not access the network**，不会下载 release、安装
provider，也不会执行 instruction file 或 state record 中的内容。这消除了 remote-fetch
边界，但不会让本地 checkout 变成不可执行数据。

## Root、Path 与 Symlink 防御

被使用的 root 必须是绝对路径，且不含 **control characters**。Project scope 在 identity
与 containment 检查前按物理目录语义解析。Registry function 为每个 target 提供固定
relative suffix；空 component、`.`、`..`、absolute suffix 与不安全 serialization field
都会被拒绝。

OAW 验证每个 intermediate component 和 final destination。无论指向 allowed root 内外，
**symlink** 都会被拒绝。同样检查覆盖 policy、user target、project target、state、backup
与 recorded cross-scope reference。Project destination 必须满足 physical root
**containment**；其他位置的同名文件不能通过检查。

创建目录、复制 backup、每次 replace/remove 与清理目录前都会重复验证。这样可以减少
path swap 与 TOCTOU 风险，但不能阻止以 **same local account** 运行的进程在最后一次检查
后或操作返回后修改文件。

## State 是数据，不是 Shell

安装 state 作为 **inert tab-separated data** 解析，且 **never sourced or evaluated**。
Parser 只接受已知 record type 与 cardinality、安全 field、absolute recorded path、numeric
checksum pair、registry-order target row、已知 ownership mode/origin、一致的 shared
destination，以及与选定 physical project 匹配的 scope binding。

语法有效的 record 并不足以授权变更。Mutation 前，OAW 从 registry 重新推导 target
destination，验证已安装 policy 与 target byte，检查记录的 OAW-created directory，并在
保留 shared policy 前验证其他 live state。Forged、stale、malformed 或
executable-looking state 会以 65 关闭失败；`--force` 不能覆盖 invalid state schema。

State file 与 backup artifact 使用 `600` mode，operation backup directory 使用 `700`。
这些权限减少意外的跨用户泄露，但 backup 可能包含 user instruction file，仍应作为敏感
本地数据处理。

## Prepare 与 Apply

在 **prepare phase**，OAW 渲染 prospective content、解析所有相关 state、验证 drift 与
ownership、解决 shared destination，并在任何 managed write 开始前构建全部 file/directory
action。因此 preflight 中较后的 target 失败，会阻止较早 target 被写入。

Apply path 针对 allowed root 与 expected relative suffix 执行 **apply revalidation**。
Replacement 在 target 旁写临时文件，设置声明 mode，再次验证后执行 `mv`，从而提供
**atomic replacement per destination**。这 **not operation-wide atomicity**：多个
destination 不属于同一个 filesystem transaction，后续 apply 失败时 OAW 不承诺自动
rollback。

Dry-run 执行 preparation 并报告 action，但不创建 managed file、state、backup 或目录。
Dry-run 不是锁；真实命令会重新验证。

## Force 与 Backup

`--force` 只是针对仍能证明既有 ownership 的 drift 的窄恢复机制。它不会接管 untracked
owned file，不会绕过 symlink/containment failure，不会接受 malformed state，也不会在
ambiguous marker layout 之间猜测。

符合条件的 forced update/uninstall 在任何 mutation 前，都会收集所有受影响的现有 policy、
target 与 state artifact。OAW 创建 operation-scoped backup，以 `600` mode 复制每个
artifact，对比 source/backup checksum，写入 `manifest.tsv`，并在 apply 前重新检查 source
byte。每个 `artifact` row 记录 original absolute path、backup path 与 checksum。Apply 还会
确认待变更 destination 存在于 active manifest 中，并仍匹配 mutation 前 checksum。

Marker ownership 有歧义时，OAW 会在可能的情况下创建 recovery backup，再以 65 退出并
要求 **manual recovery**，不会自行选择删除哪些 user byte。用户通过读取 `manifest.tsv`
手工恢复；manifest 是数据，绝不能执行或 source。

## 精确 Uninstall Ownership

Uninstall 只删除干净且有记录的 managed block 或 owned file。它保留周围 user byte；没有
符合条件的 forced operation 时，不删除 drifted artifact。只有 state 记录为 OAW 实际创建、
仍解析在 allowed root 下，并且 planned file removal 后为空的目录才可删除。Prepare 后才
出现的目录绝不会被认领为 OAW-owned。

## Host-scoped Provider Trust

Runtime Provider 权限遵循以下精确链条：

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex 与 Claude Code 是独立 trust domain。共享文件会获得不同的 Host Installation 身份，
不能在 Host 之间转移权限。Descriptor binding、discovery marker、配置的 installation hint
和 pin 都只是声明或选择约束，不能伪造 Host-owned Binding Evidence。Policy-only Host 可以
报告 Candidate，但不能创建 verified Runtime Instance。

foreign diagnostics 不会进入 pin generation、Registry resolution、Profile compilation、
admission、Bundle 或 Runtime State。Active decoder 会拒绝
`oaw.provider-descriptor/v1` 与 `oaw.user-config/v1`。Fail-closed scope condition 使用
`HOST_BINDING_EVIDENCE_REQUIRED`、`PROVIDER_BINDING_UNAVAILABLE`、
`PROVIDER_FOREIGN_HOST_ONLY`、`PROVIDER_PIN_INCOMPATIBLE` 或
`HOST_PROVIDER_SCOPE_MISMATCH`；Runtime 只暴露稳定原因和不包含路径的明确 inspection
入口提示。

## Runtime Dispatch Containment

Codex Host Driver 会收到不可变的 Grant effect 与 resource set。只读 Grant 在
`--sandbox read-only` 下运行；只有包含 `write-project` 或 `git-local` 的 Grant 才能使用
`--sandbox workspace-write`。`danger-full-access` 不是 OAW Runtime dispatch mode。
这样可以防止逻辑只读的 Capability 仅因为 Host 支持写入就获得 project-write 权限。

CLI 会把可优雅处理的 interrupt 与 termination cancellation 传入活动 Host invocation。
请求 Host cancel 后，Runtime 记录 `EXECUTION_UNCERTAIN` / `PAUSED` 并要求
`RECONCILE_INVOCATION`，绝不会伪造成功 observation。任何进程都无法在不可捕获的
`SIGKILL` 后持久化最终转换，因此 deadline controller 必须在 hard kill 前预留优雅取消
时间。

## 范围之外

安装器不能防御：

- 选定 checkout 中的恶意 shell code；
- 操作系统或 **same local account** compromise；
- 验证后由 unrelated software 修改 allowed root；
- provider loader 忽略 instruction、使用 undocumented precedence 或保留 stale session；
- 模型不遵守已安装 policy；
- 手工恢复到错误路径，或从未验证 backup 恢复。

测试应使用隔离 root；每次 forced dry-run 都要检查，并保留 stderr 与报告的 backup path。
Ownership 不清楚时应停止。疑似漏洞应按[安全策略](../../SECURITY-zh.md)的私密流程报告，
不要在公开 issue 中放入 exploit detail 或本地配置。
