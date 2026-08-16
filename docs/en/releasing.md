# Release Operations Manual

This manual publishes the default `oaw` product from the repository's current
release contract. Run every command from a clean checkout of the release commit.

## Release Boundary

`scripts/build-release.sh` builds only the default `cmd/oaw` executable. A
formal release contains six platform archives and one checksum manifest:

```text
open-agent-workflow_<version>_darwin_amd64.tar.gz
open-agent-workflow_<version>_darwin_arm64.tar.gz
open-agent-workflow_<version>_linux_amd64.tar.gz
open-agent-workflow_<version>_linux_arm64.tar.gz
open-agent-workflow_<version>_windows_amd64.tar.gz
open-agent-workflow_<version>_windows_arm64.tar.gz
SHA256SUMS
```

Each archive contains the default binary, the offline `install.sh` wrapper,
version and license files, changelog, and user README files. The Canonical
Policy Set is embedded in the binary and is materialized by `oaw install`.

The optional `oaw-assurance` and `oaw-bridge` executables are separately built
components. They are not included in this release asset set. Do not add them to
a release ad hoc; first extend the release contract, tests, documentation, and
checksums explicitly.

## 1. Prepare The Version

1. Choose a semantic version such as `0.1.2`.
2. Write that exact version, without a `v` prefix, as the single line in
   `VERSION`.
3. Move the release notes from `Unreleased` into a dated
   `## [<version>] - YYYY-MM-DD` section in `CHANGELOG.md`.
4. Commit and push all release changes to `main` before creating a tag.

Set reusable coordinates for the remaining commands:

```bash
VERSION=$(sed -n '1{s/\r$//;p;}' VERSION)
TAG="v${VERSION}"
REPO="junit/open-agent-workflow"
OUTPUT="dist/${TAG}"
```

Confirm that `VERSION` is a plain release version and that the changelog has a
matching section:

```bash
printf '%s\n' "$VERSION" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$'
grep -F "## [$VERSION]" CHANGELOG.md
```

## 2. Verify Repository And Credentials

The release commit must be clean, on `main`, and already present on the remote.
The tag and GitHub Release must not already exist.

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

An empty `git status --short` is required. The last command is expected to say
that the Release does not exist. If Git and GitHub CLI use different
credentials, or an SSH remote cannot authenticate, stop and repair the chosen
Git transport before proceeding. Do not create a local tag while push access
is uncertain.

## 3. Run The Release Gate

Run the complete matrix against the exact release commit:

```bash
go test ./... -count=1
go test -mod=readonly ./... -count=1
go test -race ./... -count=1
go vet ./...
bash -n install.sh tests/*.sh scripts/*.sh
bash scripts/check-docs.sh
bash tests/run.sh
```

`tests/run.sh` covers installation ownership, all registered Host targets,
static product boundaries, Profile inspection, release archives, Docker smoke
where available, and isolation of the optional Codex Bridge. Do not tag a
commit when a required check fails. Record an explicit skip if an environmental
smoke check, such as Docker, cannot run.

## 4. Build And Verify Assets

Build into a new output directory. The release builder refuses to overwrite an
existing archive or checksum file.

```bash
bash scripts/build-release.sh "$OUTPUT"
ls -lh "$OUTPUT"
```

Verify the manifest from inside the output directory because its entries use
relative filenames:

```bash
(cd "$OUTPUT" && shasum -a 256 -c SHA256SUMS)
```

On systems that provide GNU coreutils instead:

```bash
(cd "$OUTPUT" && sha256sum -c SHA256SUMS)
```

All six archives must report `OK`. Confirm that the directory contains exactly
the six expected archives and `SHA256SUMS`. Use a new empty output directory if
a rebuild is required; do not silently reuse partial artifacts.

## 5. Create And Push The Immutable Tag

Capture the exact commit, create an annotated tag, inspect it, and push only
that tag:

```bash
COMMIT=$(git rev-parse HEAD)
git tag -a "$TAG" "$COMMIT" -m "Release $TAG"
git show --no-patch "$TAG"
test "$(git rev-list -n 1 "$TAG")" = "$COMMIT"
git push origin "refs/tags/$TAG"
```

Never move or force-push a published release tag.

## 6. Create A Draft GitHub Release

Prepare concise release notes from the matching changelog section in a file
outside the tracked source tree, then create a Draft Release:

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

The Draft step keeps incomplete or corrupt uploads away from users while the
remote assets are checked.

## 7. Verify Remote Assets And Publish

Inspect the Draft metadata and download the uploaded assets into a fresh
temporary directory:

```bash
gh release view "$TAG" --repo "$REPO" \
  --json tagName,name,isDraft,isPrerelease,targetCommitish,assets,url
VERIFY_DIR=$(mktemp -d "${TMPDIR:-/tmp}/oaw-release-verify.XXXXXX")
gh release download "$TAG" --repo "$REPO" --dir "$VERIFY_DIR"
(cd "$VERIFY_DIR" && shasum -a 256 -c SHA256SUMS)
```

Verify all of the following before publication:

- the tag resolves to the intended commit;
- `isDraft` is `true` and `isPrerelease` is `false`;
- exactly six archives and one `SHA256SUMS` asset are present;
- every downloaded archive matches the uploaded checksum manifest;
- names and release notes use the same version as `VERSION`.

Publish the verified Draft and confirm its final state:

```bash
gh release edit "$TAG" --repo "$REPO" --draft=false
gh release view "$TAG" --repo "$REPO" \
  --json tagName,name,isDraft,isPrerelease,publishedAt,url,assets
git ls-remote origin "refs/tags/$TAG" "refs/tags/$TAG^{}"
git status --short
```

The final Release must not be a Draft or Prerelease, the dereferenced tag must
match the release commit, every asset must be uploaded, and the working tree
must remain clean.

## Failure Handling

- Before a tag is pushed, fix the issue, rebuild in a new directory, and rerun
  the complete gate. A mistaken local-only tag may be deleted and recreated.
- If the tag is correct but a Draft asset is corrupt, keep the Release as a
  Draft, rebuild from the same commit, replace the Draft asset deliberately,
  and repeat the remote download verification.
- If a pushed tag points to the wrong commit, do not move it. Correct the
  source, increment the patch version, and publish a new tag.
- After publication, do not replace archives or rewrite the tag. Publish a
  corrective patch release so users retain an immutable evidence chain.
