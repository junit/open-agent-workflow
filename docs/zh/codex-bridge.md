# Codex Assurance Bridge

[English](../en/codex-bridge.md) | [README 中文](../../README-zh.md)

`oaw-bridge` 是一个可选、独立构建的 Codex 集成。它观察当前 Codex Skill Binding，
并让独立 Assurance 模块为一份 source-qualified Markdown Profile 签发 Assurance
Overlay。

默认 `oaw` 可执行文件和安装器不会导入、构建、安装、启动、更新、检查或卸载
Bridge。Bridge 缺失、被撤销、失败或不完整时，缺少的只会是可选机器 claim；Agent
仍可通过正常的规则驱动 Policy 路径选择并遵循 Markdown Profile。

安装 Bridge 不会激活 OAW。只有活跃 OAW Engagement 或显式审计请求需要当前
Binding evidence 时才使用它。

## 构建与命令

在可信源码 checkout 中构建独立可执行文件：

```bash
go build -o ./oaw-bridge ./cmd/oaw-bridge
```

完整命令面为：

```text
oaw-bridge serve codex
oaw-bridge hook codex
oaw-bridge install codex [--dry-run] [--format text|json]
oaw-bridge update codex [--dry-run] [--format text|json]
oaw-bridge check codex [--format text|json]
oaw-bridge uninstall codex [--format text|json]
```

`oaw bridge ...` 被有意禁用。默认 CLI 只会提示显式 Bridge 用户改用
`oaw-bridge`，自身不执行任何 Bridge 管理。

## 安装生命周期

变更前先预览：

```bash
./oaw-bridge install codex --dry-run --format json
./oaw-bridge install codex --format json
./oaw-bridge check codex --format json
```

安装只拥有以下目录：

```text
${XDG_DATA_HOME:-$HOME/.local/share}/open-agent-workflow/codex-bridge/
${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/codex-bridge/
```

Data payload 包含 `bin/oaw-bridge`、名为 `oaw-local` 的 local marketplace，以及一个
名为 `oaw-codex-assurance` 的 Plugin：

```text
marketplace/.agents/plugins/marketplace.json
marketplace/plugins/oaw-codex-assurance/.codex-plugin/plugin.json
marketplace/plugins/oaw-codex-assurance/.mcp.json
marketplace/plugins/oaw-codex-assurance/hooks/hooks.json
marketplace/plugins/oaw-codex-assurance/skills/oaw-codex-bridge/SKILL.md
```

Install 与 update 使用精确 Codex Plugin 命令，先 staging 再发布，并记录 owned file
digest 与 mode。Dry-run 不修改文件，也不调用 Codex。Uninstall 只删除记录过且 clean
的文件；drifted 文件会被保留并报告。

`check` 只证明安装完整性。文本输出包含：

```text
proof_scope: installation-integrity
live_protocol_proof: false
current_session_loaded: false
```

`current_session_loaded: false` 表示 management command 没有检查 live Agent session。
Install、update 或 uninstall 后可能需要新建 session，让 Codex 重新加载 Plugin 配置。

## Bridge v3 协议

Bridge v3 只暴露一个 MCP operation：

```text
observe_profile
```

Public input 只包含一个 source-qualified selector，例如
`project:team-delivery`。精确 PreToolUse matcher 为：

```text
mcp__oaw_codex_bridge__observe_profile
```

Hook 只接受该 matcher，保留 selector，并使用 `oaw.codex-hook-context/v3` 注入 private
`_oaw_host_context`。MCP service 只接受 `oaw.codex-bridge/v3`。调用方注入 reserved
context、其他 Hook event、其他 tool、malformed input 或 working directory 不匹配时都会
fail closed。

Service 通过 `internal/profileinspect` 读取同一个 source-qualified Profile，观察当前
Codex `skills/list` metadata，匹配精确 Provider installation 与 Binding content，然后调用
`internal/assurance`。结果包含一个 `oaw.assurance-overlay/v1` artifact，它绑定完整
Markdown content digest 及其声明的 Binding occurrence。

依赖方向保持单向：

```text
oaw-bridge -> assurance -> profileinspect -> Markdown Profile
```

Bridge 不拥有 Profile Responsibility、Rules、顺序、选择、Request Mode、Risk、topology、
progress、review、verification 或 completion。它不调用 OAW Core 或 Workflow
Coordinator，也不生成 Lifecycle Bundle、Capability Grant、Resource Lease、Dispatch
Packet、Receipt 或 Workflow State。

## 失败与恢复

| 原因 | 恢复方式 |
| --- | --- |
| `HOST_BRIDGE_UNAVAILABLE` | 检查独立安装与 Codex Plugin 状态；Overlay 可选时继续使用无机器保证路径。 |
| `HOST_BRIDGE_CONTEXT_REQUIRED` | 检查精确 PreToolUse matcher，并新建已加载 Plugin 的 session；不要手写 reserved context。 |
| `HOST_BRIDGE_PROTOCOL_MISMATCH` | 更新独立组件并重新加载 Codex；不要翻译 v1 或 v2 record。 |
| `HOST_OBSERVATION_FAILED` | 修复当前 `skills/list` 访问，或在没有可选 claim 的情况下继续。 |
| `HOST_OBSERVATION_PARTIAL` | 只把受影响 Binding claim 视为 unavailable；不要推断缺失 evidence。 |
| `PROFILE_NOT_FOUND` 或 `PROFILE_AMBIGUOUS` | 使用存在的 source-qualified selector，例如 `project:<id>` 或 `user:<id>`。 |
| `ASSURANCE_BINDING_UNAVAILABLE` | 安装或启用精确声明的 Skill Binding，或者在没有可选 Overlay 的情况下使用 Profile。 |

不得编辑 Overlay 或 installation state 来绕过 diagnostic。修复底层安装后，应重新观察
当前 Profile 与 Host。

## 安全边界

Bridge 不是 sandbox、process supervisor、workflow runtime、permission grant，也不证明
Skill 已被正确调用。它只启动用于读取 `skills/list` metadata 的官方本地 Codex App
Server，绝不启动 Agent 或 model process。Codex 与 Agent Host 始终保留全部物理执行
权限、sandbox、approval、authentication 与 tool 决策。

Hook context 是合作式 Host input，不是密码学签名。Overlay 是 content-addressed identity
claim，不是 completion evidence。Bridge state 与 diagnostic 不得包含 credential、
transcript、prompt、private configuration 或 raw Host output。

运行确定性检查：

```bash
bash scripts/check-codex-bridge.sh
bash tests/17-codex-bridge-management-test.sh
bash tests/18-codex-bridge-protocol-test.sh
```
