# 机器保证

[English](../en/machine-assurance.md) | [README 中文](../../README-zh.md)

`oaw-assurance` 是一个可选、独立构建的命令，用于把精确 Provider 与 Host Binding
身份附着到某一份已选 Markdown Policy Profile 已声明的引用上。它不选择 Profile、
不执行工程工作、不维护进度，也不决定 Profile 是否可用。

默认 `oaw` 二进制和安装器都不会导入、构建、安装、启动或管理这个组件。在可信源码
checkout 中，需要显式构建或安装：

```bash
go build -o ./oaw-assurance ./cmd/oaw-assurance

GOBIN="$HOME/.local/bin" go install ./cmd/oaw-assurance
```

默认 release archive 有意只包含 `oaw`。打包或发布 `oaw-assurance` 需要独立的 release
决策。

## 检查 Profile 引用

Assurance 与 `oaw profile` 使用同一个只读 Profile reader。built-in、project 与 user
source qualifier 遵循相同选择规则：

```bash
oaw-assurance overlay inspect --profile project:team-delivery
```

该命令返回精确 Profile content digest，以及按原 Profile 顺序排列的 opaque occurrence
reference 和对应 binding reference。它不会把 Responsibility、Rules、所有权或工作流
顺序复制到 Overlay 中。

```json
{
  "schema_version": "oaw.assurance-reference-index/v1",
  "profile": {
    "source": "project",
    "id": "team-delivery",
    "content_digest": "sha256:..."
  },
  "occurrences": [
    {
      "occurrence_ref": "profile-occurrence:sha256:...",
      "binding_reference": "test-skill"
    }
  ]
}
```

无法生成无歧义 occurrence index 的自由格式 Profile 内容仍是有效 Policy 内容；只有
可选机器 claim 不可用。

## 签发 Overlay

签发集成提供精确 Binding 身份和不含 secret 的 evidence reference。
`binding_reference` 必须等于所选 occurrence 中的引用。digest 使用小写
`sha256:<64 hex>` 格式。

```json
{
  "schema_version": "oaw.assurance-issue-request/v1",
  "issuer": "team-ci",
  "claims": [
    {
      "occurrence_ref": "profile-occurrence:sha256:...",
      "provider_id": "team/provider",
      "distribution_id": "provider",
      "distribution_revision": "0123456789abcdef0123456789abcdef01234567",
      "distribution_tree_digest": "sha256:...",
      "host_id": "codex",
      "surface": "codex-plugin",
      "binding_id": "test-skill",
      "binding_kind": "skill",
      "binding_reference": "test-skill",
      "invocation": "model",
      "binding_content_digest": "sha256:...",
      "evidence": [
        {
          "kind": "host-observation",
          "reference": "evidence://team-ci/test-skill",
          "digest": "sha256:..."
        }
      ]
    }
  ]
}
```

可从标准输入或普通输入文件签发：

```bash
oaw-assurance overlay issue \
  --profile project:team-delivery \
  --input request.json > overlay.json
```

签发时会重新解析原始 Markdown、固定全文 digest、拒绝未知或重复 occurrence、要求
binding reference 精确相等，并按 Profile occurrence 顺序规范化 claim。未知 JSON 字段
会被拒绝。结果 Overlay 不包含 Responsibility、Skill composition、顺序、Rules、Add-on、
Risk、Request Mode、topology、progress 或 completion 字段。

## 校验 Overlay

校验必须使用同一个 source-qualified Profile：

```bash
oaw-assurance overlay verify \
  --profile project:team-delivery \
  --input overlay.json
```

Profile ID、source、完整 Markdown 内容、occurrence mapping、Binding reference、claim
顺序、evidence 顺序或 artifact digest 发生变化时，校验都会失败。命令不会写 Profile、
Install State、Workflow State、lock 或 receipt。

Overlay 是 content-addressed identity claim，不是签名、invocation receipt、完成证据、
Host permission 或 sandbox。issuer 仍须为其提供的 evidence 负责。独立安装的 Host Bridge
可以提供当前 Host observation，但依赖方向保持单向：

```text
oaw-bridge -> oaw-assurance -> read-only Profile reader -> Markdown Profile
```

如果没有安装 `oaw-assurance`，或者任一操作失败，缺少的只会是机器 claim。Agent 仍可
使用已安装 Skill 和 Host-native ability 选择并遵循该 Policy Profile。
