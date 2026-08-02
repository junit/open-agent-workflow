# 安装器参考

[English](../en/installer.md) | [README 中文](../../README-zh.md) |
[架构](architecture.md)

请从提供预期 policy 与 renderer 的 checkout 中运行安装器。它有四个命令：

```text
./install.sh check
./install.sh install
./install.sh update
./install.sh uninstall
```

命令输出 human-readable 信息；machine-readable status 不属于 v0.1 契约。运行
`./install.sh`、`./install.sh help`、`./install.sh -h` 或 `./install.sh --help` 会显示 help
并以 0 退出。`./install.sh install --help` 这样的 command-scoped help 也会以 0 退出，
且不执行 mutation。

## 语法与选项

```text
./install.sh <check|install|update|uninstall> [options]

--target <ids>       逗号分隔的 target ID
--target=<ids>        等价的 inline 形式
--project <path>     对一个物理 project root 操作
--project=<path>      等价的 inline 形式
--dry-run             准备并报告，但不做持久写入
--force               备份后恢复符合条件、已有记录的 drift
-h, --help            显示帮助
```

`--target` 接受逗号分隔的 ID。空项与未知 ID 都是 usage error。重复项会折叠，而且无论输入
顺序如何，选定 ID 都会按 **registry order** 规范化。

没有 `--project` 时命令使用 user scope。提供 `--project` 时，OAW 把路径解析为物理 root，
并使用名为 `<crc>-<bytes>.state` 的隔离 project state。Project target 必须始终包含在该
root 内。

`check` 是只读操作，会拒绝 `--dry-run` 与 `--force`。三个 mutation command 接受
`--dry-run`。`--force` 可以在 `update` 或 `uninstall` 时恢复符合条件、已有记录的 drift；
它绝不会把 untracked file 接管为 OAW ownership。

## 默认 Target

省略 `--target` 时，默认值取决于 scope：

| Scope | 默认 target ID |
| --- | --- |
| User | `claude,codex,gemini,opencode` |
| Project | `claude,codex,gemini,opencode,cursor,windsurf,cline,roo,copilot` |

只有 Claude、Codex、Gemini 和 OpenCode 存在 user destination。在 user scope 请求
project-only target 会被拒绝，而不是静默跳过。[Adapter matrix](adapters.md)列出每个
destination 与 ownership mode。

## 命令行为

### `check`

`check` 验证 checkout source、选定 scope、安装 state、policy 和 target ownership，
但不写入。它报告安装是否 absent、clean、drifted、invalid 或存在其他不安全状态。
`check` 报告这些 human-readable status 后以 0 退出，包括 drift 或 invalid state。该 status
不会授权 mutation；后续 mutation 仍会验证 ownership，问题未解决或不能安全 force 时以
65 退出。

#### Go shadow/parity 路径

编译后的 Go CLI 也提供 `oaw check`。它是只读 Bash 命令的非权威
**shadow/parity** 实现：报告相同的 scope、规范化 target、内置 Provider 兼容诊断、
target readiness、Install State health、输出流与退出状态。

Bash 仍是权威的安装管理入口。Parity gate 会让 `./install.sh check` 与 `oaw check`
读取 same isolated fixture，逐字节比较 stdout、stderr 与退出状态，并验证两条命令都未
改变 fixture。drift、scope、target、Provider、状态、输出流或文件系统只要存在差异，
该 gate 就会失败。

通过 check parity 不会切换管理权威。用户与自动化仍使用 `install.sh` 执行 mutation。
Go 不提供权威的 `install`、`update` 或 `uninstall`。未来若要 cutover，必须另行作出明确
迁移决策，并提供 command-level parity 证据。

### `install`

`install` 从当前 checkout 渲染 policy adapter，准备完整操作，并在应用 target 后创建该
scope 的 state record。Owned-file destination 中已有的 foreign content 或冲突的 managed
ownership 即使使用 `--force` 也会被拒绝。

#### Go install shadow/parity 边界

内部 Go install driver 仅用于 parity 验证。Parity harness 会构建这个 test-only command，
让 Bash 与 Go 在同一个物理 sandbox 路径上重放，并比较 status、stdout、stderr、file type、
mode、symlink target、精确 bytes、Install State 与 backup-tree effect。

`install.sh` 仍是权威入口。public `oaw install` 尚未启用，内部 driver 不是 release
entrypoint，通过 parity 也不会授权 management cutover。

普通 `install` 不创建 operation backup，即使提供 `--force` 也是如此。扩展或协调有效
Install State 时会保留已有且有效的 `backup` 引用；被拒绝的 install 不会改变 state 或
backup tree。Ticket 13 负责 Go `update`、`uninstall` 与 forced-backup parity。

### `update`

`update` 要求现有且有效的安装记录。更新只从 **current checkout** 读取 policy、version、
registry metadata 与 renderer code；不会执行网络获取或隐藏的 release selection。
`--target` 只限制刷新哪些已安装 target；`update` 不添加或删除 target。选择尚未安装的
target 会以 65 退出。请用 `install` 添加、用 `uninstall` 删除。

### `uninstall`

存在有效安装 state 时，`uninstall` **只删除干净的 OAW ownership**：从 host file 删除
干净的 managed block，并删除干净的 OAW-owned file。它**只清理 OAW 创建的空目录**，
且这些目录必须记录在 state 中。它不会删除周围用户内容、非空目录、provider installation，
也不会删除 ownership 已 drift 的文件。没有 state 的 `uninstall` 会先确认选定
managed-block destination 不含 untracked OAW marker，然后作为成功 no-op 返回。

如果 managed destination 与记录的 checksum 或预期 OAW fragment 不同，**drift 以 65 退出**，
正常 mutation 不会开始。`--force` 也不会静默抹掉历史：apply 前，每个受影响构件都会进入
经过验证的 backup。

#### Go update/uninstall shadow/parity 边界

内部 Go update/uninstall shadow driver 仅用于 parity 测试，不是 release entrypoint。
普通 update/uninstall 行为与 Bash 匹配：在同一物理 fixture 上比较 status、stdout、stderr、
tree、mode、symlink、精确文件 bytes、Install State 与 backup effect。

`install.sh` 仍是权威入口。public `oaw update` 与 `oaw uninstall` 尚未启用。
parity 通过不会授予 management authority。只有 Ticket 14 负责 management cutover。

对于 forced operation，Go shadow 会在第一次 forced mutation 之前完成经过验证的 operation backup。
在确定性的 fault-injection matrix 中，注入的 Go failure 会恢复每个预先存在的 destination，
但只处理本次操作实际改变的对象，并按 effect 逆序执行，同时保持已完成 backup 有效。
这只是 Go shadow 的 acceptance guard：Bash 不承诺整个操作 rollback；它的公开契约仍是每个
destination 的 atomic replacement。

## Dry Run

对于 `install`、`update` 和 `uninstall`，`--dry-run` 会执行参数解析、路径推导、rendering、
state validation、drift detection 和 operation preparation，但**不写入 managed content、state、backup 或目录**。
报告会说明真实运行准备尝试的 action。

Dry run 不是 reservation。之后的真实调用会重新验证；并发文件系统变更可能使真实调用失败。

## State 与 Backup

User 与 project installation 绝不共用 state file。不同物理 project root 也分别获得不同的
state file。已安装 policy 位于 XDG config root，state 与 operation backup 位于 XDG state
root；精确路径与 record schema 见[架构指南](architecture.md)。

普通 `install` 不创建 operation backup。Clean `update` 与 `uninstall` 不一定创建 backup；
forced `update` 或 `uninstall` 会在任何 prepared destination 改变前创建经过验证的
operation-scoped backup。安装器提供 atomic replacement per destination，但不承诺整个
操作 rollback。

## 退出码

完整 v0.1 exit set 是 **0, 64, 65, 66, 69, 70, 73, and 74**：

| Code | 含义 |
| --- | --- |
| `0` | 成功、显示 help，或成功的 no-op/check 结果。 |
| `64` | Command、option、scope、root 或 target selection 无效。 |
| `65` | Drift、invalid state、containment/symlink failure 或 unsafe ownership。 |
| `66` | 没有安装 state 时请求 `update`。 |
| `69` | Unsupported/internal target 或 renderer-contract failure。 |
| `70` | `VERSION`、checkout policy 等必需本地 source 不可读或无效。 |
| `73` | Temporary workspace 或 filesystem creation 失败。 |
| `74` | Backup 创建或验证发生 I/O failure。 |

脚本应把任何未文档化的非零结果视为失败，并保留 stderr 供诊断。Exit code 只标识 failure
class，不是 machine-readable state schema。

## 示例

```bash
# 检查默认 user installation。
./install.sh check

# 预览 project installation，不创建文件或 state。
./install.sh install --project /path/to/repository --dry-run

# 安装两个 user target；输出会按 registry order 规范化。
./install.sh install --target=opencode,claude

# 从当前 checkout 更新现有 project installation。
./install.sh update --project=/path/to/repository

# 强制删除前把 drifted artifact 保存到 backup。
./install.sh uninstall --project /path/to/repository --force
```
