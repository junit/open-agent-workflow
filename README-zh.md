# Open Agent Workflow

[English](README.md)

Open Agent Workflow（OAW）是面向 Agent Host 的规则驱动工程工作流。它安装一套可读的
Policy Set，由模型选择 Profile，并使用 Host 当前已经可以读取或调用的 Skill。

正常产品由 Markdown Policy、Profile、选中的 Skill 和 Host 原生能力组成。完成工程工作不需要
OAW runtime、route scanner、reducer、Bridge 或状态数据库。可选的机器组件只能增加证据或协同，
不能让规则有效的工作流失效。

## 快速开始

可以使用发布二进制，或在当前 checkout 构建：

```bash
go build -o ./oaw ./cmd/oaw
./oaw check
./oaw install
```

随附的 `install.sh` 是离线的同目录二进制包装器，不下载或构建可执行代码：

```bash
./install.sh check
./install.sh install --project /path/to/repository
./install.sh update --dry-run
./install.sh uninstall
```

安装管理以 Go 为权威实现。 `check` 只读；`install`、`update`、`uninstall` 管理 Policy Set
文件、各 Host 的指令入口和私有 Install State，不执行工程工作。

## Canonical Policy Set

安装源是一套不可拆分的文件：

```text
POLICY.md
cooperative-protocol.md
profiles/
  SP-FULL.md
  MATT-FULL.md
  ECC-FULL.md
  MATT-SP-HYBRID.md
adapters/
  codex-policy.md
```

项目中的 `.oaw/policy/` 优先于 User Policy Set；两者不会合并。Host 指令文件只包含指向选中
Policy Set 的 managed activation router。

## 规则驱动使用

OAW 是 opt-in 的。当前顶层请求必须明确要求 Host 为某个交付物使用 OAW；否则 Host 按原生方式
工作，OAW 不检查或改变 Skill、Agent、角色、工具或权限选择。

激活后尽量在同一请求中选择 Profile：

```text
Use OAW with MATT-SP-HYBRID to deliver the editor.
```

模型读取选中的 Profile，并在某项 Responsibility 成为当前工作时读取对应 Skill。Host Skill
索引只是优化，不是可用性证明。只要规则可读或 Host 提供原生调用面，模型就可以在没有 Bridge
或 Provider attestation 的情况下使用它。

四个内置 Profile 都是普通 Markdown，可用以下命令查看：

```bash
./oaw profile list
./oaw profile show built-in:MATT-SP-HYBRID
./oaw profile check built-in:SP-FULL
```

这些命令只提供 advisory、只读检查，不选择 Profile、不读取 Skill 内容、不创建进度状态，也不执行工作流。

## 自定义 Profile

用户可以从当前已安装的 Skill 组合新方法，不需要修改 Go 代码或注册 Provider。在以下位置创建 Markdown 文件：

```text
.oaw/profiles/team-delivery.md
$XDG_CONFIG_HOME/open-agent-workflow/profiles/team-delivery.md
```

最小元数据示例：

```markdown
---
id: team-delivery
name: Team Delivery
---

# Team Delivery

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| planning | `ecc:blueprint` |
| implementation | `matt:implementation` |
| verification | `ecc:verification` |
| closeout | Host-native closeout |
```

Custom Profile 未声明的 Responsibility 使用 Policy 默认行为。项目和用户 Profile 同 ID 时仍是两个
来源，需使用 `project:team-delivery` 或 `user:team-delivery` 明确选择。内置 ID 保留，不能被覆盖。

## 可选 Assurance

Machine Assurance 和单独构建的 `oaw-bridge` 是可选证据组件。它们可以为 Profile 附加精确内容或 Skill
身份，也可以协调合作客户端，但不选择 Profile、不调用 Skill、不拥有物理权限，也不能 veto Policy 路径。
移除它们只会移除机器支撑声明。

Agent Host 拥有 model call、Agent、Skill、Plugin、MCP、Hook、凭证、工具、sandbox、approval 以及所有物理效果。
OAW 不启动 model process，也不模拟 Host。

## 开发

源码基线固定为 v0.1.0。在 checkout 中运行：

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
bash scripts/check-docs.sh
bash tests/run.sh
```

参见 [CONTEXT.md](CONTEXT.md)、[架构 ADR](docs/adr/0011-static-policy-profiles-as-the-product-core.md)
和 [Policy Set](policy/POLICY.md)。
