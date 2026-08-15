# 安装器参考

[English](../en/installer.md) | [README 中文](../../README-zh.md) |
[架构](architecture.md)

公开安装管理以 Go 为权威实现。发布归档已经包含预编译 `oaw` 或 `oaw.exe`；验证
checksum 后直接调用二进制：

```text
./oaw check
./oaw install
./oaw update
./oaw uninstall
```

从源码 checkout 使用时，先构建嵌入预期 policy 与 version 的二进制：

```text
go build -o ./oaw ./cmd/oaw
./oaw check
```

`install.sh` 是离线的同目录二进制兼容包装器。它只执行自身目录中的 `oaw` 或
`oaw.exe`，不搜索 `PATH`、不构建二进制、不获取 release，也不下载可执行代码。
兼容命令为：

```text
./install.sh check
./install.sh install
./install.sh update
./install.sh uninstall
```

发布归档包含预编译二进制，运行时不会下载可执行文件。命令输出 human-readable 信息；
machine-readable management status 不属于 v0.1 契约。任一入口不带参数运行，或使用
`help`、`-h`、`--help`，都会显示 help 并以 0 退出。`./install.sh install --help` 这样的
command-scoped help 也会以 0 退出且不执行 mutation。同目录二进制缺失或不可执行时，
包装器以 70 退出。

## 安装不会激活 OAW

安装不会激活 OAW。`install`、`update` 和 Bridge 安装只分发 Policy 与惰性 Activation Router，或管理
Host-native integration 文件。它们不会对当前对话分类、为 OAW Engagement 检查 Provider、创建 Workflow，也不改变原生 Host Skill routing。
之后仍必须由当前顶层用户指令为一个交付物显式激活 OAW。

`update` 会把有效的 OAW-owned eager managed instruction 替换为惰性 Activation Router，同时保留 non-OAW byte。它不会将活跃的 legacy policy-only Markdown lifecycle lock 转换为 Progress Tracker。请在旧契约下完成该工作，或在当前 Policy 下显式重新激活并重新选择交付物。

## 语法与选项

```text
./oaw <check|install|update|uninstall> [options]
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

Project scope 会把完整 Canonical Policy Set 安装到 `<project>/.oaw/policy/`，并向选定的
Host target 写入 project-relative Activation Router。Project Custom Profile 位于
`<project>/.oaw/profiles/`，属于用户 ownership，不会被这些命令管理。User scope 会把完整
Policy Set 安装到 `${XDG_CONFIG_HOME:-$HOME/.config}/open-agent-workflow/`；受管 Built-in
Profile 位于 `profiles/builtin/`，用户拥有的 Custom Profile 保留在 `profiles/*.md`，这些命令
绝不管理它们。

Activation Router 在 `.oaw/policy/POLICY.md` 存在时整套选择 Project Policy Set，否则选择
User Policy Set；两者的 core 文件绝不合并。Project 与 user Custom Profile 保持可发现，
并带有明确 source identity。

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

`check` 验证 embedded management source、选定 scope、Install State、policy 和 target ownership，
但不写入。它报告安装是否 absent、clean、drifted、invalid 或存在其他不安全状态。
`check` 报告这些 human-readable status 后以 0 退出，包括 drift 或 invalid state。该 status
不会授权 mutation；后续 mutation 仍会验证 ownership，问题未解决或不能安全 force 时以
65 退出。

#### 公开 Go management 边界

`oaw check` 与三个 mutation command 使用同一个公开 production binary。兼容包装器通过
`exec` 到达这一实现；release 中不存在第二套 Bash management 实现。Cutover 前的 Bash
行为只作为仓库中的独立测试 oracle 保留。

### `install`

`install` 从运行中 binary 的 embedded source 渲染 policy adapter，准备完整操作，并在
应用 target 后创建该 scope 的 state record。Owned-file destination 中已有的 foreign
content 或冲突的 managed ownership 即使使用 `--force` 也会被拒绝。

普通 `install` 不创建 operation backup，即使提供 `--force` 也是如此。
扩展或协调有效 Install State 时会保留已有且有效的 `backup` 引用；被拒绝的 install 不会改变 state 或 backup tree。

在 project scope 中，如果 `.oaw/policy/` 已存在且非空，会被视为 untracked ownership，
install 会拒绝覆盖。对已有且已记录的 Policy Set，应使用 `update` 路径。

在 user scope 中，可以预先存在包含 Custom Profile 的 `profiles/` 目录，install 会保留它。
已有的 `POLICY.md`、`cooperative-protocol.md`、`adapters/` 或 `profiles/builtin/` 等受管
destination 仍属于 untracked ownership conflict，install 绝不会接管。

### `update`

`update` 要求现有且有效的安装记录。二进制嵌入从 **current checkout** 构建的 policy、
version、registry metadata 与 renderer behavior；修改源码 checkout 后必须重新构建
`./oaw`，而 release archive 已经包含 release Policy 与 Version。不会执行网络获取或隐藏
的 release selection。
`--target` 只限制刷新哪些已安装 target；`update` 不添加或删除 target。选择尚未安装的
target 会以 65 退出。请用 `install` 添加、用 `uninstall` 删除。

### `uninstall`

存在有效安装 state 时，`uninstall` **只删除干净的 OAW ownership**：从 host file 删除
干净的 managed block，并删除干净的 OAW-owned file。它**只清理 OAW 创建的空目录**，
且这些目录必须记录在 state 中。它会删除干净的受管 Policy Set 文件与目录，但保留 project
`.oaw/profiles/` 与 user `profiles/*.md` Custom Profile。它不会删除周围用户内容、非空目录、provider installation，
也不会删除 ownership 已 drift 的文件。没有 state 的 `uninstall` 会先确认选定
managed-block destination 不含 untracked OAW marker，然后作为成功 no-op 返回。

如果 managed destination 与记录的 checksum 或预期 OAW fragment 不同，**drift 以 65 退出**，
正常 mutation 不会开始。`--force` 也不会静默抹掉历史：apply 前，每个受影响构件都会进入
经过验证的 backup。

对于 `update` 与 `uninstall`，Go mutation journal 会在每个 effect 应用后记录 inverse。
apply failure 会尝试按逆序恢复已改变 destination 和已删除的 owned directory。Rollback
failure 以状态 74 报告，并要求 manual recovery。Forced operation 还会在第一次 mutation
之前完成 verified operation backup，因此即使 automatic rollback 无法完成，backup 仍是
可审计的恢复来源。

## Dry Run

对于 `install`、`update` 和 `uninstall`，`--dry-run` 会执行参数解析、路径推导、rendering、
state validation、drift detection 和 operation preparation，但**不写入 managed content、state、backup 或目录**。
报告会说明真实运行准备尝试的 action。

Dry run 不是 reservation。之后的真实调用会重新验证；并发文件系统变更可能使真实调用失败。

## State 与 Backup

User 与 project installation 绝不共用 state file。不同物理 project root 也分别获得不同的
state file。Project Policy Set 位于 `<project>/.oaw/policy/`；User Policy Set 位于 XDG
`open-agent-workflow/` config root。State 与 operation backup 位于 XDG state root；精确路径
与 record schema 见[架构指南](architecture.md)。

Install State 与 Workflow State 相互独立，不会自动迁移。Management command 不会创建
Workflow State、协作式 Policy Workflow Plan 或 Progress Tracker，也不会导入现有 policy-only task 或 Profile lock。
在 Stable Boundary 启动协调或执行切换是显式 Workflow action，不是 `install`、`update` 或 `uninstall` 的副作用。

普通 `install` 不创建 operation backup。Clean `update` 与 `uninstall` 不一定创建 backup；
forced `update` 或 `uninstall` 会在任何 prepared destination 改变前创建经过验证的
operation-scoped backup。每个 destination 使用 atomic replacement；后续 apply failure
会触发 Go mutation journal 的 best-effort whole-operation rollback。Rollback failure 会
明确报告，并保留 verified backup 供 manual recovery。

## Codex Host Bridge

Codex 有两套相互独立的 OAW installation surface：

```text
oaw install --target codex
oaw bridge install codex
```

第一条命令按选定 scope 安装 Policy Set 与 policy adapter。Project scope 的文件自包含于
`.oaw/policy/`，user scope 的文件自包含于 XDG `open-agent-workflow/` root；它不安装 executable Plugin，
也不声明 current-session Host evidence。第二条命令是经过审计的 Codex Host Bridge
的显式 opt-in transaction。两者都不会为任何请求激活 OAW。它的 management surface 是：

```text
oaw bridge check codex
oaw bridge update codex
oaw bridge uninstall codex
oaw bridge serve codex
oaw bridge hook codex
```

Bridge 在 `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/codex-bridge`
下拥有 install state，在
`${XDG_DATA_HOME:-$HOME/.local/share}/open-agent-workflow/codex-bridge` 下拥有 binary
与 local marketplace。Codex 拥有 Plugin cache、enablement configuration、approval
与其他 Host state。OAW 只通过固定 argument vector 调用官方 Codex Plugin command，
不编辑 Codex config 或 cache，不创建隔离用户主目录，也不投影 Host configuration。

使用 Bridge 前，在 Codex `/hooks` 中检查并信任 rendered `hooks/hooks.json` 的四个
精确 `PreToolUse` matcher 与 `SubagentStart` matcher。安装或 update 后启动新的 Codex
session。只有该新 session 中成功的 `observe_current` 才能证明 current-session evidence；
`bridge check` 永远
报告 `current_session_loaded: false`。

仅启动新 session 不会证明 `child-delegation`。当用户已显式请求 Profile/topology（例如
`SP-FULL / CURRENT`），且唯一阻断是 reviewer child requirement 时，Startup Gate 可在该
session 中仅启动一次零项目副作用的原生 child capability probe。child 只报告已启动并立即
终止；随后重新调用 `observe_current`，再重复 `core_inspect`。

Bridge install 与 update 是 transactional。它们渲染 running binary 的
digest-pinned copy，使用 OAW-owned local marketplace；official Codex registration
失败时回滚 OAW-owned file。Drift、symlink、unrecorded payload file 与不匹配的 state
会保留并报告。Uninstall 先调用 official Plugin 与 marketplace removal，然后只删除
clean 的 recorded OAW file 与 state，保留无关 Codex config、user file 与 drifted
content。

Bridge 只支持 `CURRENT`。当前 Codex session 调用 Skill 与 tool；OAW 观察
allowlisted metadata、编译 policy 并交换 Coordinator record。它不创建 child session、
不调用 model，也不复制或重建 MCP、Hook、Skill、Plugin、authentication、sandbox
或 approval configuration，除非这些是 Host 报告的 fact。完整 Hook 与 recovery
契约见 [Codex Host Bridge 指南](codex-bridge.md)。

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

# 从运行中的 binary 更新现有 project installation。
./install.sh update --project=/path/to/repository

# 强制删除前把 drifted artifact 保存到 backup。
./install.sh uninstall --project /path/to/repository --force
```
