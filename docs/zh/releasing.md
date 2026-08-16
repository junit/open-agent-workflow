# 发布操作手册

本手册用于按照仓库当前的发布契约发布默认 `oaw` 产品。所有命令都应在 release commit 的干净
checkout 根目录中执行。

## 发布边界

`scripts/build-release.sh` 只构建默认的 `cmd/oaw` 可执行文件。一次正式发布包含六个平台归档和
一份 checksum manifest：

```text
open-agent-workflow_<version>_darwin_amd64.tar.gz
open-agent-workflow_<version>_darwin_arm64.tar.gz
open-agent-workflow_<version>_linux_amd64.tar.gz
open-agent-workflow_<version>_linux_arm64.tar.gz
open-agent-workflow_<version>_windows_amd64.tar.gz
open-agent-workflow_<version>_windows_arm64.tar.gz
SHA256SUMS
```

每个归档包含默认二进制、离线 `install.sh` 包装器、版本与许可证文件、changelog 和用户 README。
Canonical Policy Set 嵌入在二进制中，由 `oaw install` 写出到安装位置。

可选的 `oaw-assurance` 和 `oaw-bridge` 是单独构建的组件，不包含在这组正式发布资产中。不要临时把它们
加入某次发布；必须先明确扩展发布契约、测试、文档和 checksum。

## 1. 准备版本

1. 选择一个语义版本，例如 `0.1.2`。
2. 将不带 `v` 前缀的准确版本号作为 `VERSION` 的唯一一行。
3. 把 release note 从 `Unreleased` 移入 `CHANGELOG.md` 中带日期的
   `## [<version>] - YYYY-MM-DD` 小节。
4. 创建 tag 前，将所有发布改动提交并推送到 `main`。

为后续命令设置可复用坐标：

```bash
VERSION=$(sed -n '1{s/\r$//;p;}' VERSION)
TAG="v${VERSION}"
REPO="junit/open-agent-workflow"
OUTPUT="dist/${TAG}"
```

确认 `VERSION` 是普通正式版本，且 changelog 存在匹配小节：

```bash
printf '%s\n' "$VERSION" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$'
grep -F "## [$VERSION]" CHANGELOG.md
```

## 2. 检查仓库与凭证

release commit 必须位于干净的 `main`、已经存在于远端，而且相同 tag 和 GitHub Release 尚不存在。

```bash
git status --short
test "$(git branch --show-current)" = main
git fetch origin main --tags
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"
test -z "$(git tag --list "$TAG")"
gh auth status
git ls-remote origin HEAD
gh release view "$TAG" --repo "$REPO"
```

`git status --short` 必须没有输出。最后一条命令预期提示 Release 不存在。如果 Git 和 GitHub CLI 使用
不同凭证，或者 SSH remote 无法认证，应先停止并修复选定的 Git transport。不能在 push 权限不确定时
创建本地 tag。

## 3. 执行发布门禁

针对准确的 release commit 执行完整验证矩阵：

```bash
go test ./... -count=1
go test -mod=readonly ./... -count=1
go test -race ./... -count=1
go vet ./...
bash -n install.sh tests/*.sh scripts/*.sh
bash scripts/check-docs.sh
bash tests/run.sh
```

`tests/run.sh` 覆盖安装所有权、所有已注册 Host target、静态产品边界、Profile inspection、release
archive、可用时的 Docker smoke，以及可选 Codex Bridge 隔离。任何必需检查失败时都不能创建 tag；如果
Docker 等环境型 smoke 无法运行，应明确记录 skip。

## 4. 构建并校验资产

使用新的输出目录构建。release builder 会拒绝覆盖已有 archive 或 checksum 文件。

```bash
bash scripts/build-release.sh "$OUTPUT"
ls -lh "$OUTPUT"
```

manifest 使用相对文件名，因此应进入输出目录进行校验：

```bash
(cd "$OUTPUT" && shasum -a 256 -c SHA256SUMS)
```

提供 GNU coreutils 的系统可以改用：

```bash
(cd "$OUTPUT" && sha256sum -c SHA256SUMS)
```

六个 archive 必须全部报告 `OK`，目录中必须恰好包含六个预期 archive 和 `SHA256SUMS`。需要重新构建时
应使用新的空目录，不能静默复用部分产物。

## 5. 创建并推送不可变 Tag

记录准确 commit、创建 annotated tag、检查 tag，然后只推送该 tag：

```bash
COMMIT=$(git rev-parse HEAD)
git tag -a "$TAG" "$COMMIT" -m "Release $TAG"
git show --no-patch "$TAG"
test "$(git rev-list -n 1 "$TAG")" = "$COMMIT"
git push origin "refs/tags/$TAG"
```

已经发布的 release tag 绝不能移动或 force-push。

## 6. 创建 GitHub Draft Release

根据匹配的 changelog 小节在 tracked source tree 之外准备简明 release notes，然后创建 Draft Release：

```bash
gh release create "$TAG" \
  --repo "$REPO" \
  --verify-tag \
  --draft \
  --title "Open Agent Workflow $TAG" \
  --notes-file /absolute/path/to/release-notes.md \
  "$OUTPUT"/*.tar.gz \
  "$OUTPUT"/SHA256SUMS
```

Draft 阶段可以在远端校验资产，避免用户看到未完成或损坏的上传。

## 7. 校验远端资产并发布

检查 Draft metadata，并把上传的资产下载到新的临时目录：

```bash
gh release view "$TAG" --repo "$REPO" \
  --json tagName,name,isDraft,isPrerelease,targetCommitish,assets,url
VERIFY_DIR=$(mktemp -d "${TMPDIR:-/tmp}/oaw-release-verify.XXXXXX")
gh release download "$TAG" --repo "$REPO" --dir "$VERIFY_DIR"
(cd "$VERIFY_DIR" && shasum -a 256 -c SHA256SUMS)
```

正式发布前必须确认：

- tag 解析到预期 commit；
- `isDraft` 为 `true`，`isPrerelease` 为 `false`；
- 恰好存在六个 archive 和一个 `SHA256SUMS` 资产；
- 所有下载的 archive 都匹配上传的 checksum manifest；
- 名称与 release notes 使用和 `VERSION` 相同的版本。

发布已经验证的 Draft，并检查最终状态：

```bash
gh release edit "$TAG" --repo "$REPO" --draft=false
gh release view "$TAG" --repo "$REPO" \
  --json tagName,name,isDraft,isPrerelease,publishedAt,url,assets
git ls-remote origin "refs/tags/$TAG" "refs/tags/$TAG^{}"
git status --short
```

最终 Release 不能是 Draft 或 Prerelease，dereferenced tag 必须匹配 release commit，所有资产都必须完成
上传，并且工作树仍然干净。

## 故障处理

- tag 推送前，先修复问题，在新目录重新构建并重跑完整门禁。误建的纯本地 tag 可以删除后重建。
- tag 正确但 Draft 资产损坏时，保持 Draft 状态，从相同 commit 重建，有意识地替换 Draft 资产，然后重新
  执行远端下载校验。
- 已推送的 tag 指向错误 commit 时，不要移动 tag；应修复源码、增加 patch version 并发布新 tag。
- 正式发布后，不要替换 archive 或重写 tag。发布修正 patch release，保持用户可依赖的不可变证据链。
