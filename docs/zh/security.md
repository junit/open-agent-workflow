# 安装器安全模型

[English](../en/security.md) | [安全策略](../../SECURITY-zh.md) |
[架构](architecture.md)

本指南说明本地 Open Agent Workflow（OAW）installer 及 policy/coordinator protocol 的控制
与限制。它不声称不可信 checkout、操作系统、Agent Host 或 Provider 是安全的。

## 信任边界

Installer 把以下值与构件视为 trust-boundary input：

- 当前 checkout，包括 executable shell code、`VERSION` 和 `policy/ENGINEERING.md`；
- CLI target 与 project 参数；
- `HOME`、`XDG_CONFIG_HOME` 与 `XDG_STATE_HOME`；
- physical project root 以及 selected destination 下的每个 component；
- 现有 policy、adapter、state、directory 与 backup artifact。

只运行你信任的 checkout。OAW **does not access the network**，不下载 release、不安装
Provider，也不执行 instruction file 或 state record 中的内容。去掉 remote-fetch 边界并不
代表本地 checkout 不是可执行代码。

## Root、Path 与 Symlink 防御

输入 root 必须是绝对路径，且不含 **control characters**。Project scope 在 identity 与
containment 检查前按 physical-directory semantics 解析。Registry function 为每个 target
提供固定 relative suffix；empty component、`.`、`..`、absolute suffix 与不安全
serialization field 都会被拒绝。

OAW 验证每个 intermediate component 与 final destination。无论 **symlink** 指向允许 root
内部还是外部，都拒绝使用。相同检查覆盖 policy、user target、project target、state、
backup 和 recorded cross-scope reference。Project destination 必须满足 physical root
**containment**，其他位置的同名文件不能替代。

创建目录、复制 backup、每次 replacement/removal 和 prune directory 前都会重复验证。
这会降低 path-swap 与 TOCTOU 风险，但无法阻止以 **same local account** 运行的其他
process 在最后一次检查后修改文件。

## State 是数据，不是 Shell

Install State 按 **inert tab-separated data** 解析，且 **never sourced or evaluated**。
Coordinator 的 Workflow State 使用独立 schema 与 namespace；两种 state 都不是可执行输入。

Parser 只接受已知 record type/cardinality、安全 field、absolute recorded path、numeric
checksum pair、registry-order target row、已知 ownership mode/origin、一致的 shared
destination，以及与 selected physical project 匹配的 scope binding。Forged、stale、
malformed 或 executable-looking state 以 exit 65 关闭失败。`--force` 不能绕过 invalid state
schema。

State file 与 backup artifact 使用 mode `600`，operation backup directory 使用 `700`。
这些 permission 减少意外交叉用户泄露，但 backup 可能含用户 instruction file，仍应视为
敏感本地数据。

## Prepare 与 Apply

在 **prepare phase**，OAW 渲染 prospective content、解析相关 state、验证 drift 与
ownership、解析 shared destination，并在 managed write 开始前构建全部 file/directory
action。后续 target 失败会阻止 preflight 中较早 target 被写入。

Apply path 对 allowed root 与 expected relative suffix 执行 **apply revalidation**。
Replacement 在 target 旁写 temporary file，设置 mode，再次验证后执行 `mv`，提供
**atomic replacement per destination**。这属于 **not operation-wide atomicity**：多个
destination 不是一个 filesystem transaction，后续 apply failure 不保证自动 rollback。

Dry-run 执行准备与报告，但不创建 managed file、state、backup 或 directory。Dry-run
不是 lock；真实 command 会重复验证。

## Force 与 Backup

`--force` 只能恢复 prior ownership 仍可建立的 drift。它不 adopt untracked owned file、
不绕过 symlink 或 containment failure、不接受 malformed state，也不猜测 ambiguous marker。

符合条件的 forced update 或 uninstall 变更前，OAW 收集全部受影响 policy、target 与 state
artifact，创建 operation-scoped backup，以 mode `600` 复制，比较 checksum，写入
`manifest.tsv`，并在 apply 前重新检查 source bytes。每个 `artifact` row 记录 original
absolute path、backup path 与 checksum。

Marker ownership 有歧义时，OAW 会尽可能创建 recovery backup，然后以 65 退出并要求
**manual recovery**。它不会选择删除哪些用户 byte。用户通过读取 `manifest.tsv` 手工恢复；
manifest 是数据，绝不能执行或 source。

## 精确 Uninstall Ownership

Uninstall 只删除干净的 recorded managed block 或 owned file。它保留周围用户 byte，不在
缺少 eligible forced operation 时删除 drifted artifact。只有 state 证明由 OAW 创建、仍在
allowed root 下、且 planned file removal 后为空的目录才能删除。

## Core、Coordinator 与 Host 安全边界

Provider authority 遵循以下精确链条：

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

OAW Core 接收 secret-free fact 并编译 Lifecycle Bundle。Workflow Coordinator 只记录
secret-free Workflow State、cooperating clients、logical workflow authority 与 opaque
digest reference。它不能保存 API key、token、raw Provider output、private Hook payload，
或完整 MCP/Plugin configuration。

Agent Host 拥有物理执行权限。Host sandbox and approvals、model route、authentication、
tool、MCP、Hook、Skill 与 Plugin 都由 Host 拥有。Capability Grant 或 Resource Lease 可以
比 Host sandbox and approvals 更窄，但不能物理阻止 Host 执行协议外 action。

`CURRENT` 使用当前 Host session，环境保持不变。只有该 Host session 暴露原生 child-agent
facility 时，`SUBAGENT` 才可用。

OAW never starts a model CLI。`policy` integration 只分发 instruction。`host-native`
integration 可以报告 session fact 与 Receipt，但 OAW 绝不保证 MCP、Hook、Skill 或 Plugin
inheritance 到 `SUBAGENT`；active Host 报告各 surface 为 `inherited`、`host-configured`、
`restricted`、`unknown` 或 `unavailable`。

Host session 变化会使 stale Dispatch Packet 失效。继续前需要 fresh Host report 与 Bundle
eligibility check。OAW 不重建缺失 child environment，也不静默 fallback 到新 process。

## Codex Host Bridge 边界

Codex 默认提供 policy integration，并另有独立且经过审计的 host-native Bridge，必须显式
安装并信任。Bridge v1 只支持 `CURRENT` 与 `skill` binding。它不创建 child session，
也不保证继承 MCP、Hook、Skill、Plugin、model、authentication、sandbox 或 approval
behavior，除非 Host 提供稳定 observation。

Trusted `PreToolUse` Hook input 是唯一的 current-session identity source。Agent 不能自行
填写或替换 reserved `_oaw_host_context`。只有严格只读的 `observe_current` rewrite 可以
得到自动 `allow`；后续 Core 与 Coordinator operation 保留正常 Host approval behavior，
session 或 working-directory 不匹配时 fail closed。

`skills/list` 是 v1 唯一的 Provider binding authority。`hooks/list` 与 allowlisted
`config/read` projection 只是 diagnostic environment observation。`plugin/list` 不是
production dependency。Filesystem detection、Descriptor declaration、user configuration、
prompt 与 Skill self-report 都不能创建 Host Binding Evidence。

Bridge 在 bounded process memory 中保存 opaque session-bound handle，只返回 secret-free
summary。它不保存 raw Hook command、credential、MCP environment value、header、token、
arbitrary Plugin setting 或完整 App Server configuration。Handle 不能进入 Workflow State、
evidence artifact、log、ticket 或 screenshot。

这是 cooperation boundary，不是 operating-system isolation。具有相同用户权限的 process
可以干扰本地 program、file 或 process I/O。OAW 可以验证 protocol record，但不能认证或
contain 每个 same-user process。

## 范围之外

Installer 与 policy protocol 无法防御：

- selected checkout 中的恶意 shell code；
- 操作系统或 **same local account** compromise；
- unrelated software 在 validation 后修改 allowed root；
- Provider loader 忽略 instruction 或使用 undocumented precedence；
- model 不遵守 installed policy；
- manual restoration 使用错误 path 或未经验证 backup。

测试应使用 isolated root，检查每次 forced dry-run，保留 stderr 与 backup path，ownership
不明确时停止。疑似漏洞通过[安全策略](../../SECURITY-zh.md)中的 private 流程报告，不要把
exploit detail 或本地配置放进公开 issue。

## Canonical Security Terms

双语契约有意保留下列精确术语：

```text
logical workflow authority
Host sandbox and approvals
secret-free
opaque digest
cooperating clients
OAW never starts a model CLI
```
