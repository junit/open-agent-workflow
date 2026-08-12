# Codex Host Bridge

[English](../en/codex-bridge.md) | [安装器](installer.md) | [架构](architecture.md) | [安全](security.md)

Codex Host Bridge 是 Open Agent Workflow (OAW) 的显式、经过审计的 Codex Plugin 集成。它与 policy adapter 分离：policy adapter 分发 `ENGINEERING.md`；Bridge 增加 opt-in MCP server 与四个范围严格受限的 `PreToolUse` Hook。

Bridge 是 Host integration，不是第二个 Runtime。当前 Codex session 调用 Skill 与 tool 并执行所有物理 effect。OAW 只观察无 secret 的 Host metadata、编译 policy 并交换 Coordinator record。OAW 不启动 child session，不执行 lifecycle Capability，不调用 model，也不重建 Codex environment。

安装 Bridge 不会激活 OAW。Filesystem evidence、已注册 Plugin 与 `observe_current` 可用性都
只是 readiness fact。只有当前顶层用户请求创建活跃 OAW Engagement 后，才能调用 Bridge
与 Core operation。保证等级预检随后可以使用 Bridge 建立 `core-backed` 或
`coordinator-backed` 支持；在此之前，普通请求与普通 Skill routing 都保持原生 Host 行为。

## 命令表面

Policy installation 与 Bridge installation 是两项不同的操作：

```text
oaw install --target codex
oaw bridge install codex
oaw bridge check codex
oaw bridge update codex
oaw bridge uninstall codex
oaw bridge serve codex
oaw bridge hook codex
```

`oaw install --target codex` 是 policy-only 路径，不安装 executable Plugin，也不声明 Host-native evidence。`oaw bridge install codex` 是显式的 executable Plugin transaction，要求 operator 检查并信任精确的 Hook definition。

Management command 使用当前工作目录作为 physical project root，并使用当前用户的 `codex` executable。`check`、`install`、`update` 与 `uninstall` 使用 `--format json` 可得到 machine-readable management projection；`install` 与 `update` 还接受 `--dry-run`。

`bridge serve codex` 由 Plugin 的 MCP configuration 启动。`bridge hook codex` 由 Plugin Hook 调用，并从 stdin 接收一个 JSON Hook event。两者都不是通用 shell 或 model launcher。

## 安装与信任

安装前先检查 policy-only 状态与当前 checkout：

```bash
oaw install --target codex
oaw bridge check codex --format json
```

确认当前 Codex Host 可以信任其报告的 fact 后，再执行 opt-in 安装：

```bash
oaw bridge install codex --format json
oaw bridge check codex --format json
```

`bridge check` 只属于 installation management。Text output 会明确报告：

```text
proof_scope: installation-integrity
live_protocol_proof: false
```

它只证明 managed file 与 Codex registration，不证明 active session 已加载的 protocol。
只有一次新的 `observe_current` call 会协商 live protocol evidence。如果 `check` 报告
drift、version mismatch 或需要新 session，必须在 Workflow START 前停止；update 与替换
session 是独立 operator action。

该 transaction 渲染名为 `oaw-local` 的 local marketplace、名为 `oaw-codex-host` 的 Plugin，以及 running OAW binary 的 checksum-pinned copy。信任 Plugin 前检查以下五个 rendered file：

```text
.agents/plugins/marketplace.json
plugins/oaw-codex-host/.codex-plugin/plugin.json
plugins/oaw-codex-host/.mcp.json
plugins/oaw-codex-host/hooks/hooks.json
plugins/oaw-codex-host/skills/oaw-codex-bridge/SKILL.md
```

Plugin manifest 必须精确指向 `./skills/`、`./.mcp.json` 与 `./hooks/hooks.json`。MCP map 必须以 direct argument vector 调用 OAW binary 的 `bridge serve codex`。Hook file 必须恰好包含下列四个 generated tool 各一个 `PreToolUse` matcher：

```text
mcp__oaw_codex_bridge__observe_current
mcp__oaw_codex_bridge__core_inspect
mcp__oaw_codex_bridge__core_compile
mcp__oaw_codex_bridge__workflow_exchange
```

打开 Codex 的 `/hooks` view，检查精确的四个 matcher 与 command path。不要使用 trust-bypass flag。安装后启动新的 Codex session；旧 session 不能被声称已经加载新的 Plugin。

## 当前 Session 契约

Bridge v2 在当前 Codex integration 中只支持 `CURRENT` topology。当前 Codex session 是
physical executor。没有 `SUBAGENT` implementation、shell fallback、container fallback、
隔离用户主目录、projected configuration，也没有复制 MCP、Hook、Skill、Plugin、model、
authentication、sandbox 或 approval environment 的行为。当前 observation 只证明 `skill`
Binding。Role、instruction、agent、tool、delegation 与 Host-action fact 只有在 Codex 报告
稳定、allowlisted evidence 后才可用，否则保持 unknown 或 unavailable。

Canonical live negotiation tuple 为：

```text
Bridge integration 2.0.0
oaw.codex-bridge/v2
oaw.codex-hook-context/v2
oaw.host-evidence-handle/v2
oaw.provider-descriptor/v4
oaw.profile-recipe/v3
oaw.host-manifest/v3
oaw.host-session/v3
oaw.host-binding-inventory/v3
oaw.host-environment-report/v2
oaw.host-invocation-receipt/v3
oaw.host-conformance-transcript/v4
oaw.host-conformance-report/v4
oaw.execution-graph/v4
oaw.lifecycle-bundle/v4
oaw.capability-grant/v3
oaw.dispatch-packet/v2
oaw.workflow-command/v2
oaw.workflow-result/v2
oaw.workflow-snapshot/v2
oaw.workflow-revision/v2
```

VersionEvidence digest 独立覆盖完整 normalized tuple。任何缺失、重复、非 canonical、
stale 或不匹配 field 都会在使用 Core/Coordinator authority 前返回
`HOST_BRIDGE_PROTOCOL_MISMATCH`。

只有 `observe_current` 能建立 current-session identity。它的 Hook 严格只读，在 Codex 创建 public tool input 后注入 reserved context。语义上的输出是官方 nested envelope：

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "updatedInput": {
      "_oaw_host_context": {
        "schema_version": "oaw.codex-hook-context/v2",
        "bridge_protocol_version": "oaw.codex-bridge/v2",
        "session_id": "<opaque Host value>",
        "turn_id": "<opaque Host value>",
        "tool_use_id": "<opaque Host value>",
        "cwd": "/absolute/project/root",
        "model": "<diagnostic metadata>",
        "permission_mode": "<diagnostic metadata>"
      }
    }
  }
}
```

实际输出会保留 public tool input，并增加 reserved `_oaw_host_context`；示例省略无关的 public field。Caller 不能自行填写或替换该 reserved field。

另外三个 Hook matcher 会验证 `host_evidence_handle`、exact session 与 exact working directory。合法的后续 operation 输出零字节 stdout，因此正常 Codex MCP approval policy 仍然生效。缺少、编辑过期、foreign 或 malformed handle 返回 nested deny form：

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny"
  }
}
```

每次 invocation 必须有 `hook_event_name == PreToolUse`。错误 event、缺少 context 或 foreign session 都 fail closed。只有只读 observation rewrite 可以得到自动 `allow` decision。

## Observation 与 Workflow

在新的、受信任的 session 中按以下顺序使用 Bridge operation：

```text
observe_current
core_inspect
core_compile
workflow_exchange
```

`observe_current` 返回短的 Host summary 与 opaque `HostEvidenceHandle`。Handle 只是 cache reference，不是 credential、Lifecycle Bundle field 或 durable evidence artifact。不要把它复制进 Workflow State、log、ticket 或 report。

`core_inspect` 解析 Host-scoped Provider Candidate 与 exact enabled Skill Binding。
`skills/list` 是 required；`hooks/list` 与 `config/read` 是 optional，这三个 method 构成完整
metadata allowlist。Skill 只有在 exact enabled name/path 位于一个精确 discovered Provider
installation 下，且完整 Binding tree 匹配 pinned Distribution digest 时才会 verify。
Disabled、missing、orphan、ambiguous、same-name foreign、shared-ancestor、symlinked 或
drifting tree 都不会创建 authority。

`core_compile` 要求用户显式 Profile、`CURRENT` 与 Add-on selection 对应的精确 confirmation
digest。Inspection 没有 implicit selection。`WORKFLOW` 仍必须经过 Startup Gate；Bridge
observation 只提供 Host evidence。`workflow_exchange` 只接受 Workflow Command v2，并只
返回包含 Bundle/Graph v4 的 Workflow Result/Snapshot/Revision v2。

Public caller 不能提供 user authorization、explicit invocation attestation 或 gate
attestation。Bridge 从当前 v2 handle hydrate 这些 Host-owned fact，否则 fail closed。每个
非 START command 前都会检查 active Bundle 的 session、environment、inventory、feature、
action、configuration、resolution 与 registry digest。Drift 会在读取或签发 executable
authority 前返回 `HOST_SESSION_CHANGED`。

Bridge 不可用时 Direct Mode 仍可用。Bridge failure 不得把小范围 bounded change 变成 Host-native claim。

## Ownership 与 Rollback

OAW 只拥有：

- `${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/codex-bridge` 下的 Bridge install state；
- `${XDG_DATA_HOME:-$HOME/.local/share}/open-agent-workflow/codex-bridge` 下的 managed binary 与 local marketplace；
- 该 local marketplace 内 rendered Plugin source file。

Codex 拥有 Plugin cache、enablement configuration、approval 与其他 Host state。OAW 不直接修改 Codex config 或 cache，只通过固定 argument vector 调用官方 `codex plugin marketplace` 与 `codex plugin` command surface。

Install 与 update 是 transactional。失败时回滚 OAW-owned file，并对失败 transaction 已完成的 registration 使用官方 Codex removal。Drift 会被保留，不会被覆盖。Update 要求 OAW payload clean 且精确 recorded，并且 Codex registration 匹配。Uninstall 先执行 official Plugin 与 marketplace removal，然后只删除 clean 的 recorded OAW file 与 state。无关 file、user edit、symlink 和 unrecorded payload entry 都会保留并报告。

变更前先检查：

```bash
oaw bridge check codex --format json
oaw bridge update codex --dry-run --format json
oaw bridge uninstall codex --format json
```

安装或 update 后启动新的 Codex session。Uninstall 后也启动新的 session，确认只剩 policy integration；management check 永远不声称当前 session 已加载 Bridge。

## 诊断与恢复

Host integration diagnostic 与 Provider、Profile decision 分离。以下是稳定的 v2 reason。每项 recovery 都是显式动作，不会静默选择 Profile 或改变 Host authority。

| Code | 含义 | Recovery |
| --- | --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | 本 session 中 Plugin 或 MCP Bridge 不可用。 | 运行 `oaw bridge check codex --format json`，安装或启用 Plugin 后启动新 session。 |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | Trusted Hook 没有注入有效 context。 | 打开 Codex `/hooks`，检查并信任四个精确 matcher，然后启动新 session。 |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | Plugin、Hook、Bridge、Core 或 Host protocol version 不兼容。 | 运行 `oaw bridge update codex`，重新检查 Hook 并启动新 session。 |
| `HOST_EVIDENCE_HANDLE_REQUIRED` | 后续 operation 缺少当前 handle。 | 在 active session 调用 `observe_current`，再重试 operation。 |
| `HOST_EVIDENCE_HANDLE_INVALID` | Handle malformed、edited、unknown、evicted 或来自重启后的 Bridge。 | 调用 `observe_current`，只使用新返回的 handle。 |
| `HOST_EVIDENCE_EXPIRED` | Handle 超过 bounded TTL。 | 再次调用 `observe_current`，然后执行 `core_inspect` 或 `core_compile`。 |
| `HOST_EVIDENCE_SESSION_MISMATCH` | Handle 属于另一 Codex session 或 cwd。 | 停止 stale exchange，在当前 session 调用 `observe_current` 并重新编译。 |
| `HOST_OBSERVATION_FAILED` | 必需的 stable metadata observation 失败。 | 运行 `oaw bridge check codex --format json`，修复本地 Codex/App Server capability 后重试。 |
| `HOST_OBSERVATION_PARTIAL` | 可选 environment metadata 不完整。 | 再次运行 `core_inspect`，所有不可用 field 保持显式 `unknown`。 |
| `HOST_SESSION_CHANGED` | active Bundle pin 的 fact 发生变化。 | 暂停 Workflow，调用 `observe_current`，再次经过 Startup Gate 后再编译。 |

其他层继续使用自己的 reason，包括 `HOST_BINDING_EVIDENCE_REQUIRED`、`HOST_BINDING_INVENTORY_INVALID`、`PROVIDER_CANDIDATE_AMBIGUOUS`、`PROVIDER_PIN_INCOMPATIBLE`、`PROVIDER_BINDING_UNAVAILABLE` 与 `PROFILE_TOPOLOGY_UNAVAILABLE`。

Management diagnostic 包括 `BRIDGE_INSTALL_NOT_INSTALLED`、`BRIDGE_INSTALL_DRIFT`、`BRIDGE_INSTALL_STATE_INVALID`、`BRIDGE_INSTALL_AUTHORITY_MISMATCH` 与 `BRIDGE_INSTALL_ROLLBACK_INCOMPLETE`。先运行 `oaw bridge check codex --format json`。不要对 unsafe state、symlink containment 或 unrecorded file 使用 force override；保留原始 file 与 state 进行人工恢复。

## 安全边界

Hook 是唯一 current-session identity source。Skill instruction、prompt、filesystem
candidate 或 user-authored input 都不能替代。`skills/list` 是 required v2 Skill
observation authority；`plugin/list` 不是 production dependency。Raw Hook command、
credential、MCP environment value、header、token、arbitrary Plugin setting 与 App
Server configuration 都不会保存。

Bridge 使用当前用户的正常 environment 查询 metadata；它不创建隔离用户主目录，也不创建 projected environment。以同一用户权限运行的恶意 process 可以干扰本地 program。这是 cooperative Host integration，不是 operating-system authentication 或 isolation boundary。

不要把 handle、absolute Skill path、raw Hook command 或 credential 发布到 ticket、
Workflow State、evidence、log 或 screenshot。Handle 绑定一个 live Bridge process、session
与 CWD；绝不能跨 restart、session 或 working directory 持久化或复用。Local dogfood
record 放在 `.scratch/oaw-codex-host-bridge-dogfood/` 且保持 untracked。

## 卸载

完成 controlled pilot 或不再信任 Bridge 时：

```bash
oaw bridge uninstall codex --format json
oaw bridge check codex --format json
```

Uninstall 先调用官方 Codex Plugin removal，再删除 OAW-owned local marketplace 与 clean payload。Drift 或 unrecorded file 会保留。启动新的 Codex session，确认 policy-only surface 不再声称 current-session evidence。没有旧 Runner 或 projected Host environment 的 compatibility fallback。
