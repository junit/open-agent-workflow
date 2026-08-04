# 故障排查

[English](../en/troubleshooting.md) | [安装器参考](installer.md) |
[安全模型](security.md)

整个诊断过程要使用同一个 binary、scope 与 target selection。Release archive 已包含
`oaw`；源码 checkout 必须先运行 `go build -o ./oaw ./cmd/oaw`。OAW 不获取 release，也不
修复 workflow provider；v0.1 management 输出是 human-readable。求助时保留完整命令与
stderr。

## 安全诊断顺序

从只读命令开始：

```bash
./oaw check
# 兼容包装器：
./install.sh check
```

Project scope 要重复准备 mutation 的精确 scope：

```bash
./oaw check --project /absolute/path --target claude
# 兼容包装器：
./install.sh check --project /absolute/path --target claude
```

`check exits 0` 后会报告 `clean`、`drift`、`invalid-state` 或 `not-installed`。请读取
`installed <target>:` 行；exit 0 只表示检查完成，不表示后续 mutation 已获授权。

然后预览现有 installation update：

```bash
./oaw update --dry-run
./oaw update --project /absolute/path --target claude --dry-run
# 兼容包装器：
./install.sh update --dry-run
./install.sh update --project /absolute/path --target claude --dry-run
```

`./install.sh update --dry-run` 执行与 update 相同的 state、ownership、source 与 path
preparation，但不写 managed content、state、backup 或目录。没有 installation state 时
`update exits 66`；这时应改为预览 `install --dry-run`，再用 `install` 创建缺失 target。

确认 changed file、理解 OAW 拥有哪些 byte、并检查 preview 前，不要添加 `--force`。
符合条件且有记录的 drift 使用一个显式 scoped command：

```bash
./oaw update --project /absolute/path --target claude --force
# 兼容包装器：
./install.sh update --project /absolute/path --target claude --force
```

上面的精确示例有意限制范围，不要把 project 或 target 换成宽泛猜测。普通 drifted
**mutation exits 65** 且不写入；只有接受 recorded ownership 与 backup 后才应 force。

## 读取 `check` 输出

| 输出 | 含义 | 下一步 |
| --- | --- | --- |
| `provider <name>: missing` | 在预期 instruction root 未检测到 **missing provider**。 | 独立安装/修复 provider，或选择不需要它的 lifecycle bundle；OAW 绝不安装 provider。 |
| `target <id>: detected` | 找到 target tool 的 instruction root。 | 这只是 readiness 信息，不能证明运行中的 agent 已加载 adapter。 |
| `installed <id>: not-installed` | 没有有效 state row 拥有该 target。 | 使用 `install --dry-run`，再 install；不能用 update 添加。 |
| `installed <id>: clean` | Recorded policy 与 target ownership 匹配磁盘。 | 行为仍 stale 时检查 provider loading，并 restart the target agent。 |
| `installed <id>: drift` | Managed byte、policy 或 recorded file 已不匹配。 | 对照 state-backed expectation 比较 destination；决定 force 前先预览 update 或 uninstall。 |
| `installed <id>: invalid-state` | State shape、binding、registry metadata 或 shared ownership 不可信。 | 不要 force；保留 state 与文件供手工诊断。 |

**missing provider** 不一定阻止 adapter file 安装，但选定 workflow capability 仍不可用。
Provider detection 不能选择 lifecycle profile，也不能替换另一个 family。

### Host-scoped Provider 诊断

Provider 权限按以下顺序建立：

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

运行 `oaw providers inspect --host codex --format text`；如果原 Run 使用了
`--project-root`，这里必须使用同一路径。Codex 与 Claude Code 即使引用共享文件，也仍是
独立 Host。current section 只包含所选 Host 的 Candidate 和 observation。Policy-only Host
可以显示 Candidate，但不能验证 Runtime Instance。foreign section 仅供诊断，绝不提供 pin
或权限。Descriptor binding 与 installation hint 是声明，不是 Host Binding Evidence。

稳定原因含义如下：

| 原因 | 含义 |
| --- | --- |
| `HOST_BINDING_EVIDENCE_REQUIRED` | 所选 Host 有 Candidate，但没有 Host-owned Binding Inventory。 |
| `PROVIDER_BINDING_UNAVAILABLE` | Inventory 存在，但没有精确匹配 Installation/Capability/Binding 的 observation。 |
| `PROVIDER_FOREIGN_HOST_ONLY` | Candidate 只存在于 foreign diagnostic Host，不能用于当前权限。 |
| `PROVIDER_PIN_INCOMPATIBLE` | 当前 Host 的 pin 不再匹配 installation 身份或 evidence。 |
| `HOST_PROVIDER_SCOPE_MISMATCH` | Registry、Instance、Bundle 或 Runtime 的 Host 身份不一致。 |

`PROVIDER_CANDIDATE_AMBIGUOUS` 要求 operator 选择一个当前 Host Candidate，并把精确建议
加入用户自己管理的配置：

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

OAW 不会替用户选择 Candidate，也不会写入 pin。配置变化后必须开始新的 Run。
`oaw.provider-descriptor/v1` 与 `oaw.user-config/v1` 不再是受支持的 active input；必须显式
替换为 v2 record，不能期待自动迁移。

## Management State 不是 Runtime State

Install State 与 Runtime State 相互独立，不会自动迁移。Adapter 安装成功并报告 `clean`，
仍可能正确地保持 Policy-only。现有 task 与 profile lock 不会被导入，management command
也不会创建 Engineering Run。目前只有固定版本的 Codex runner 是 Runtime-managed；其他
已安装 adapter 都不提供 Runtime admission、Grant、lease、transition enforcement 或
physical isolation 保证。符合条件的 Policy-only task 只能在 Stable Boundary 显式接管。

## 文件 Clean 但 Agent 行为陈旧

Agent 工具的 precedence 与 reload 行为不同。先在 [adapter matrix](adapters.md) 确认精确
路径与 loader，再检查其他 user、project、nested、team 或 organization instruction 是否
具有更高的文档化 precedence。

Provider 未文档化 live rule reload 时，应 **restart the target agent** 或应用。存在官方
refresh command 时，应使用它并检查 loaded context。OAW marker comment 不会强制 reload，
也不建立 model precedence。

对 bootstrap adapter，确认运行中的 agent 能读取 canonical policy 的绝对路径。对
documented-import adapter，在可用时使用 provider 的 context inspection。OAW check 为 clean
不能证明 provider 或模型遵守了 instruction。

## Drift 与 Invalid State

Drift 通常表示 user、tool 或 checkout 改变了 recorded OAW byte。编辑前保留当前文件，
使用相同 scope/target 运行 dry-run，并检查是否提出 `would-update`、`would-remove` 或
`would-backup`。

`--force` 只能修复与 valid state 绑定的 recoverable drift，不能：

- 接管 untracked owned file；
- 修复 malformed 或 forged state；
- 跟随或替换 symlink；
- 逃逸 project/XDG containment root；
- 在 duplicate、nested 或其他 ambiguous marker 之间选择。

Ambiguous marker case 可能创建 backup，随后以 `manual recovery required` 停止。这是拒绝，
不是 partial success。编辑前比较 original、expected OAW fragment 与 backup。

## 检查并恢复 Backup

成功的 forced mutation 输出 `backup: <directory>`；forced dry-run 输出
`would-backup: <directory>`，但不创建它。在报告的目录中把 `manifest.tsv` 作为文本打开。
Header 记录 format、operation 与 scope；每个 `artifact` row 记录：

```text
artifact<TAB>original-absolute-path<TAB>backup-path<TAB>checksum
```

确认每个 original path 属于预期 scope、每个 backup file 存在且 checksum 匹配 manifest。
绝不能 source 或执行 `manifest.tsv`。恢复前停止受影响 agent/tool，再逐个审核 artifact，
从列出的 backup path 向 original path **restore backups manually**。保留 mode，之后重新运行
`check`。

Go manager 会在已报告的 apply operation 失败时尝试逆序 rollback。Replacement 仍只对
单个 destination 原子化，并非跨所有 destination 同时原子；process 或 machine crash 也不在
该 automatic rollback path 内。stderr 以状态 74 报告 `rollback failed` 时，用 manifest 与
命令输出识别需要 manual restore 的 artifact；不要把整个 backup directory 覆盖到 `HOME`、
XDG root 或 project。

## Update 问题

- `no installation state; run install first`：update exits 66。先运行 scoped install dry-run，
  再 install target。
- `selected target is not installed`：update 不能新增 target，请使用 install。
- `installed content differs from this checkout`：运行中的 binary 嵌入了不同 source
  version 或 policy。源码使用场景应从准备信任的 checkout 重新构建 `./oaw`；release 用户
  应使用已验证 archive 中的 binary。
- `VERSION`、policy 或 `precompiled sibling binary is missing or not executable` 触发
  exit 70：重新构建源码 binary，或从已验证 release archive 恢复 binary。包装器绝不搜索
  `PATH`、构建或获取替代项。
- Path、containment、control-character 或 symlink diagnostic：修正 root 或 filesystem
  layout；`--force` 不能覆盖。

## Uninstall 拒绝

**uninstall refusal** 会保护无法证明 ownership 的内容。常见原因包括 drift、invalid state、
changed/missing recorded target、symlink swap、不一致的 shared-destination checksum 或
ambiguous marker。先运行 `check`，再运行对应 scoped `uninstall --dry-run`。State 有效且
drift 是有意变更时，审核显式 scoped forced uninstall；否则进行 manual recovery。

没有 state 的 uninstall 是 guarded successful no-op，但 untracked OAW marker 仍以 65
退出。非空的 OAW-created directory 会保留并报告 unchanged，而不是递归删除。Clean
uninstall 绝不删除周围 user instruction 或独立安装的 provider。

证据仍不清楚时应停止 mutation，保留 checkout version、完整输出、state file、destination
byte 与所有 backup path，并按[安全策略](../../SECURITY-zh.md)进行私密报告。
