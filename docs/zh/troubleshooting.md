# 故障排查

[English](../en/troubleshooting.md) | [安装器参考](installer.md) |
[安全模型](security.md)

整个诊断过程要使用同一个 checkout、scope 与 target selection。OAW 不获取 release，也不
修复 workflow provider；v0.1 输出是 human-readable。求助时保留完整命令与 stderr。

## 安全诊断顺序

从只读命令开始：

```bash
./install.sh check
```

Project scope 要重复准备 mutation 的精确 scope：

```bash
./install.sh check --project /absolute/path --target claude
```

`check exits 0` 后会报告 `clean`、`drift`、`invalid-state` 或 `not-installed`。请读取
`installed <target>:` 行；exit 0 只表示检查完成，不表示后续 mutation 已获授权。

然后预览现有 installation update：

```bash
./install.sh update --dry-run
./install.sh update --project /absolute/path --target claude --dry-run
```

`./install.sh update --dry-run` 执行与 update 相同的 state、ownership、source 与 path
preparation，但不写 managed content、state、backup 或目录。没有 installation state 时
`update exits 66`；这时应改为预览 `install --dry-run`，再用 `install` 创建缺失 target。

确认 changed file、理解 OAW 拥有哪些 byte、并检查 preview 前，不要添加 `--force`。
符合条件且有记录的 drift 使用一个显式 scoped command：

```bash
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

系统承诺 **no operation-wide rollback**。Replacement 只对单个 destination 原子化，因此
后续 apply 失败时较早的 destination 可能已经更新。用 manifest 与命令输出识别要手工恢复
的 artifact；不要把整个 backup directory 覆盖到 `HOME`、XDG root 或 project。

## Update 问题

- `no installation state; run install first`：update exits 66。先运行 scoped install dry-run，
  再 install target。
- `selected target is not installed`：update 不能新增 target，请使用 install。
- `installed content differs from this checkout`：install 发现不同 source version 或 policy。
  从准备信任的 checkout 运行 update。
- `VERSION` 或 policy 触发 exit 70：当前 checkout 不完整、不可读或无效。OAW 只从该
  checkout 更新，绝不获取替代项。
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
