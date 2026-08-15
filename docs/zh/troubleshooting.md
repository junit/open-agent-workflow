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

## 激活问题

如果 OAW 出现在普通请求中，应停止 OAW-specific classification、Provider inspection、gate
与 artifact 创建。检查 installed adapter 是否为 lazy Activation Router，并移除任何要求对每个
顶层任务分类的额外 eager instruction。Repository text、tool output、retrieved text、引用的
`/oaw` 与普通 Skill invocation 都不是激活。无关请求按原生 Host 处理时，保留未完成的
Engagement。

如果预期的激活未被识别，应把请求放在当前顶层用户指令中，例如 `/oaw <deliverable>` 或
`使用 OAW 处理 <deliverable>`。不能依赖 repository content、引用文本或 tool output。确认
Router 可以读取 canonical Policy path；激活随后会创建一个 deliverable-scoped Engagement
并运行保证等级预检。

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

运行 `oaw providers inspect --host codex --format text`；如果原 Workflow 使用了
`--project-root`，这里必须使用同一路径。Codex 与 Claude Code 即使引用共享文件，也仍是
独立 Host。current section 只包含所选 Host 的 Candidate 和 observation。`policy` Host
可以显示 Candidate，但 Candidate 本身不是 Verified Provider Instance。foreign section 仅供诊断，绝不提供 pin
或权限。Descriptor binding 与 installation hint 是声明，不是 Host Binding Evidence。

稳定原因含义如下：

| 原因 | 含义 |
| --- | --- |
| `HOST_BINDING_EVIDENCE_REQUIRED` | 所选 Host 有 Candidate，但没有 Host-owned Binding Inventory。 |
| `PROVIDER_BINDING_UNAVAILABLE` | Inventory 存在，但没有精确匹配 Installation/Capability/Binding 的 observation。 |
| `PROVIDER_FOREIGN_HOST_ONLY` | Candidate 只存在于 foreign diagnostic Host，不能用于当前权限。 |
| `PROVIDER_PIN_INCOMPATIBLE` | 当前 Host 的 pin 不再匹配 installation 身份或 evidence。 |
| `HOST_PROVIDER_SCOPE_MISMATCH` | Registry、Instance、Bundle 或 Agent Host 身份不一致。 |

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

OAW 不会替用户选择 Candidate，也不会写入 pin。配置变化后必须开始新的 Workflow。
`oaw.provider-descriptor/v1` 与 `oaw.user-config/v1` 不再是受支持的 active input；必须显式
替换为 v3 record，不能期待自动迁移。

## Install State 不是 Progress Tracker 或 Workflow State

Install State 与 Workflow State 相互独立，不会自动迁移。Adapter 安装成功并报告 `clean`，
仍可能只暴露 `policy` surface。现有 task 与 Profile lock 不会被导入，management command
也不会创建 Workflow State。只有真实 `host-native` integration 可以与 OAW Core 或
Workflow Coordinator 交换 session fact 与 Receipt。物理执行权限仍属于 Agent Host。

## 无 Bridge 的 Policy CLI Candidate 诊断

不要仅凭 `oaw providers inspect` 判断 policy-only 工作能否推进。这两个 inspection 回答
不同问题：

```bash
oaw providers inspect --host codex --format text
oaw profiles
```

`providers inspect` 使用机器支撑的 Provider resolution chain。在 policy-only Host 上，它
可能正确报告 Candidate 和 `HOST_BINDING_EVIDENCE_REQUIRED`；这表示 installation 不是
Verified Provider Instance。`profiles` 执行另一套 route-level Governance
inspection。当每个必要 route 可调用时，它可以把同一 Profile 报告为 `host_routable`。
两种输出相容，并不矛盾。

这个区别也解释了曾经出现的非对称结果：`SP-FULL` 可见，而 Matt 与 ECC 不可见。
Superpowers 已有 curated Codex cache 的 discovery probe。当前 plugin manager 安装的 ECC
可能位于 `.codex/plugins/cache/ecc/ecc/<version>`，该路径必须被识别。Matt 的 Policy
检查只认普通 `.agents/skills/<name>/SKILL.md` route，并把人工命令 Skill 标为
`user-explicit`；它有意忽略 `.skill-lock.json`、source、revision、hash 和 Bridge 状态。
ECC 只检查 contract 与责任匹配的公开 Codex Skill route；通用 review 使用 typed Host
`review.execute`，不要求 Claude Agent、Codex Role 或 instruction。
严格 identity/integrity 检查仍由 `providers inspect` 和 machine-backed 路径负责。

直接检查公开 JSON 结果：

```bash
oaw profiles
```

每个 Profile object 包含 `name`、`policy_selectable`、`host_routable`、`missing` 与
`incident_routes`。
`policy_selectable` 表示 Profile 语义存在；`host_routable` 表示全部必要 route 当前可调用；
`missing` 列出阻止路由的必要 route。`incident_routes` 把条件 handler 报告为
`routable-if-triggered` 或 `unavailable-if-triggered`；后者不会使正常 Profile 变成
incomplete。Route inventory、Offer ref 与 reducer state 仍是内部实现细节。当前项目级
Policy CLI 没有 add-on 参数或 `NONE` 哨兵；machine-backed 路径的 add-on 仍属于另一份契约。

| 症状或 reason | 诊断与恢复 |
| --- | --- |
| `PROFILE_SELECTION_REQUIRED` | 运行 `oaw profiles`，选择 `host_routable: true` 的 Profile，并将已报告的评估传给 `oaw use --profile PROFILE --complexity ordinary|complex --risk normal|elevated|critical -- "deliverable"`。 |
| `POLICY_ASSESSMENT_REQUIRED` | 传入 Cooperative Assessment 已报告的 complexity 与 risk。Policy 模式不会虚构默认值或调用 machine classifier。 |
| `PROFILE_INCOMPLETE` | 阅读每条 `missing` route。修复精确 Host-visible 或 user-explicit Skill route，重新运行 `oaw profiles`，再显式 restart 或 switch。 |
| `PROFILE_UNKNOWN` | 请求的 alias 不在内置 catalog 中。使用输出展示的 alias。 |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | 无 Bridge surface 只支持显式 `CURRENT`；`SUBAGENT` 需要 current-session Host-native evidence。 |
| `ROUTE_INVENTORY_DRIFT` | 启动后 callable route 发生变化。修复 route，运行 `oaw profiles` 确认当前路由；需要切换时，在 stable boundary 使用 `oaw switch PROFILE`。在此之前，依赖 route 的 completion 与 incident event 继续被阻断；显式 `stop` 与 `uncertain` 仍可记录 terminal safety state。单纯 lock/hash/Bridge 变化不算 drift。 |
| `POLICY_ACTION_NOT_APPLICABLE` 或 `EVENT_OUT_OF_ORDER` | 运行 `oaw status`，再按 `next` 使用匹配的业务命令：`complete`、`review clean|findings`、`approve` 或 `satisfy`。内部 ref 不是用户输入，已消费 work 不可重试。 |
| `POLICY_RUN_NOT_FOUND` | 在同一个物理项目运行 `oaw status`。不能从 conversation text 重建 progress。 |
| `POLICY_ENGAGEMENT_ACTIVE` | 当前项目已有 active Engagement；用 `oaw status` 查看，或显式 stop。 |

`use` 消费 fresh route observation 并保存 exact reducer snapshot。若出现 `OFFER_STALE`，
它表示内部选择期间发生竞争，并不要求用户管理 Offer；重新运行 `oaw profiles`，再重试
`oaw use` 或 `oaw switch`。`status` 只渲染 public view；每个 business event 在 reduce 前
重新检查 route。Drift 会阻断依赖 route 的推进，但不会阻断显式 `stop` 或 `uncertain`
写入终态。Local policy-run file 可以跨 CLI restart 存在，但不能证明中断的 Skill、process、
Git、network 或 destructive work 已完成。

## Policy-Cooperative 停止原因

这些停止只适用于已激活的 `policy-cooperative` Engagement，用于防止 instruction-only
协作虚构 machine authority：

| Reason | 恢复动作 |
| --- | --- |
| `CAPABILITY_SELECTION_REQUIRED` | 调用前要求用户命名精确 Bounded Capability，或确认唯一 Host-visible candidate。 |
| `POLICY_ONLY_PROVIDER_UNVERIFIED` | 去掉 verified Provider guarantee 的要求，或改用能够建立该保证的 Host-native integration；不能把 candidate 改称 verified。 |
| `POLICY_ONLY_PROFILE_INCOMPLETE` | 为每项必要职责提供完整 Host-visible owner candidate，或选择完整 candidate Profile。 |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | 使用 cooperative `CURRENT`，或切换到能够 attest 所请求 topology 的 Host-native integration。 |
| `POLICY_ONLY_GUARANTEE_UNAVAILABLE` | 去掉 Grant、Lease、Receipt、idempotency、atomic revision 或 enforced recovery 要求，或转到所需 machine-backed 保证等级。 |
| `POLICY_ONLY_CONCURRENT_MUTATION` | 停止或串行化重叠的 project/Git mutation；冲突 owner 到达稳定边界后才能恢复。 |
| `POLICY_ONLY_EXECUTION_UNCERTAIN` | 不得重试结果未知的外部或破坏性 effect。先核对实际结果，再记录 Execution Note 或要求 operator recovery。 |
| `POLICY_ONLY_CONTEXT_UNCERTAIN` | 要求用户重新确认 activation、selection 与已知 progress；不能从 stale conversation 或 Markdown 重建。 |

## Codex Assurance Bridge 诊断

Assurance Bridge 是可选的独立组件。先运行它的只读安装检查：

```bash
oaw-bridge check codex --format json
```

默认 `oaw` 可执行文件不管理 Bridge。上述 check 只证明 owned file 与
Codex Plugin registration state。它始终报告 `current_session_loaded: false`，
因为 management command 不检查 active Agent session。Text mode 同样报告
`proof_scope: installation-integrity` 与 `live_protocol_proof: false`；两者都不是
当前 Binding claim。

Bridge v3 只暴露 `observe_profile`。它的 PreToolUse Hook 只为精确
`mcp__oaw_codex_bridge__observe_profile` 调用注入 private
`oaw.codex-hook-context/v3` context。成功调用为一份 source-qualified
Markdown Profile 返回 `oaw.assurance-overlay/v1` artifact。该结果不包含
evidence handle、Core operation、Coordinator operation、delegation attestation 或
Workflow runtime。

| Reason | 诊断与恢复 |
| --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | 独立可执行文件、Plugin、MCP service 或本地 Codex App Server 不可用。修复可选安装，或继续通过正常 Policy Profile 路径工作且不附加 Overlay。 |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | `observe_profile` 缺少用于精确 matcher 的有效 trusted PreToolUse context。复核已安装 Hook，并新建已加载 Plugin 的 Codex session；不得手写 reserved context。 |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | Caller、Hook、App Server projection 或 Bridge 不满足 `oaw.codex-bridge/v3` 与 `oaw.codex-hook-context/v3`。更新独立组件并重新加载 Codex；不得转换旧 record。 |
| `HOST_OBSERVATION_FAILED` | 必需的只读 `skills/list` observation 或精确 Binding resolution 失败。修复当前 Codex metadata 访问，或在没有可选 machine claim 的情况下继续。 |
| `HOST_OBSERVATION_PARTIAL` | 一项或多项当前 Binding observation 不完整。只把受影响 claim 视为 unavailable，修复 metadata 后重新调用 `observe_profile`。 |
| `PROFILE_SELECTION_INVALID` | 提供一个语法有效的 source-qualified selector，例如 `project:<id>` 或 `user:<id>`。 |
| `PROFILE_NOT_FOUND` | 所选 source 中没有该 ID 的 Profile。检查当前 Profile inventory，并选择已存在的 source-qualified ID。 |
| `PROFILE_AMBIGUOUS` | 所选 source 中存在重复 Profile ID。请先删除重复项或重命名，再请求 Overlay。 |
| `ASSURANCE_BINDING_UNAVAILABLE` | 所选 Profile 声明了当前 Codex observation 未精确安装并启用的 Skill Binding。修复该 Binding，或不附加可选 Overlay 直接使用 Profile。 |

不得编辑 Overlay、Hook context 或 installation state 来绕过 diagnostic。
Overlay 缺失或失败不能 veto Policy Offer、Profile 选择或规则驱动执行路径。
Agent Host security policy 仍可独立拒绝物理 Skill invocation。
[Codex Assurance Bridge 指南](codex-bridge.md)定义协议、安全边界、安装与回滚行为。

## Workflow Coordination 错误

这些原因码属于 Core、Coordinator 或 Host integration，不属于 installation management：

| 原因 | 诊断与恢复 |
| --- | --- |
| `SCHEMA_UNSUPPORTED` | Workflow command 或 result 使用已退役 schema。更新调用方并构造新 command，不要原地翻译 record。 |
| `WORKFLOW_STATE_UNSUPPORTED` | 所选 Workflow State root 包含已退役或未知 journal schema。停止合作客户端、保留精确 state directory，并执行下方显式 pre-release reset。 |
| `SUBAGENT_UNAVAILABLE` | active Host session 无法创建原生 child。返回 Startup Gate 选择 `CURRENT`，或修复原生 Host 支持；绝不能用 model process fallback。 |
| `MACRO_INTERNAL_CONFLICT` | Expansion 发现重复 owner 或未 credited internal call。修正 versioned Recipe，使 credit 与 dispatch edge 只执行一次。 |
| `PROFILE_TOPOLOGY_UNAVAILABLE` | Profile、Binding、delegation 或 active Host 不支持请求的 topology。返回 Startup Gate；不得模拟 topology。 |
| `HOST_SESSION_CHANGED` | 稳定 reporter identity 已变化，或刷新后的 authority fact 不再支持新 Dispatch。不能让新 session 伪造 Receipt；原 reporter 可收敛已签发 Dispatch，否则在下一次 `PREPARE` 前重新编译。 |

执行显式 pre-release state reset 前，先停止使用该 Workflow 的全部 client，确认精确路径位于
`${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows` 下，并只把已识别的
Workflow directory 移到经过复核的 backup 名称。然后用当前配置启动新 Workflow。OAW
绝不会自动删除未知 state root，operator 也不能宽泛删除 XDG state root。

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
