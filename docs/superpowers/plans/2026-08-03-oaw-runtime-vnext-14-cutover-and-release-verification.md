# OAW Runtime vNext Ticket 14 Cutover and Release Verification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut installation management over to the public Go CLI, reduce `install.sh` to an offline compatibility wrapper, and produce environment-aware verified cross-platform release archives without importing Policy-only or Install State into Runtime State.

**Architecture:** The public `oaw` command becomes the single management implementation for `check`, `install`, `update`, and `uninstall`; the compatibility script may only locate and execute the binary shipped beside it. A local release builder cross-compiles self-contained binaries and packages each with the wrapper and governance documents. Existing Install State remains under `open-agent-workflow/installations`, while Runtime State remains under `open-agent-workflow/runtime`; release and CLI black-box tests prove that neither Policy-only artifacts nor TSV state cross that boundary. A shared Linux archive smoke runs through Docker when available and through WSL when present; unavailable platform executors return status 77 and are recorded without blocking completion.

**Tech Stack:** Go 1.26 standard library and cross-compilation, Bash 3.2 compatibility wrapper and black-box tests, `tar`, SHA-256 tooling, ShellCheck, Go race/coverage/vet/fuzz tooling, and the existing OAW conformance/eval corpus.

---

## Confirmed Test Seams

The previously approved seams remain the only behavior seams for this ticket:

- public `cli.Run` and the compiled `cmd/oaw` command;
- `install.sh` as an exact compatibility forwarding entrypoint;
- release archive names, contents, checksums, and current-platform execution;
- the disjoint Install State and Runtime State directories;
- `scripts/smoke-wsl.sh` running a Linux release archive inside an actual WSL kernel.

Tests must not inspect private mutation plans or mock internal filesystem helpers.

### Task 1: Promote Go Management to the Public CLI

**Files:**
- Rename: `internal/cli/shadow_install.go` to `internal/cli/management.go`
- Rename: `internal/cli/shadow_install_test.go` to `internal/cli/management_test.go`
- Modify: `internal/cli/run.go`
- Delete: `internal/cmd/oaw-management-shadow/main.go`

- [ ] **Step 1: Write the failing public-routing and state-separation tests**

Replace `TestPublicRunDoesNotRouteManagement` with public behavior tests that invoke `Run` for `install`, `update`, and `uninstall`. Use isolated absolute `HOME`, `XDG_CONFIG_HOME`, and `XDG_STATE_HOME` roots and assert the same statuses, stdout, stderr, files, and Install State already proven through `RunShadowManagement`.

Add a migration-boundary test that:

```go
project := filepath.Join(root, "policy-only-project")
policyOnly := filepath.Join(project, ".scratch", "existing-task", "workflow.md")
runtimeRoot := filepath.Join(state, "open-agent-workflow", "runtime")
runtimeSentinel := filepath.Join(runtimeRoot, "preexisting-runtime-sentinel")
```

creates `policyOnly`, completes a public project install, creates `runtimeSentinel`, then completes public update and uninstall. Assert the Policy-only file is unchanged, no Runtime State exists after install, and the exact Runtime sentinel bytes and tree remain unchanged after update/uninstall. The test must separately assert that TSV Install State was created and consumed under `open-agent-workflow/installations`.

Also assert these routing rules:

```text
oaw / oaw help / oaw -h / oaw --help -> installerUsage, status 0
oaw check                               -> Go check
oaw install|update|uninstall            -> Go management
oaw catalog ...                         -> catalog CLI
oaw runtime ... / oaw run ...           -> Runtime CLI
unknown top-level command               -> installer-style status 64
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
rtk go test ./internal/cli -run 'Public|Management|PolicyOnly' -count=1
```

Expected: FAIL because public `Run` still routes management verbs and legacy help through the catalog parser.

- [ ] **Step 3: Implement the authoritative management path**

Rename shadow-only identifiers to production names:

```go
type managementCommand struct { /* existing fields */ }
func runManagement(args []string, stdout, stderr io.Writer) int
func parseManagement(args []string) (managementCommand, error)
func managementEnvironment() management.Environment
func executeManagement(managementCommand, management.Environment) (management.Result, error)
```

In `RunWithInput`, reserve `catalog`, `runtime`, and `run` for their existing handlers, route `check` to `runCheck`, and route every other top-level form to `runManagement`. `runManagement` must preserve legacy global help/no-argument behavior before parsing. Do not add an Install State migration or any call from management into `internal/runtime`.

Delete the test-only management shadow command after all test callers use `cmd/oaw`.

- [ ] **Step 4: Run formatting and focused tests for GREEN**

Run:

```bash
rtk gofmt -w internal/cli
rtk go test ./internal/cli ./internal/management -count=1
```

Expected: PASS, including the state-separation test.

- [ ] **Step 5: Commit the public cutover slice**

```bash
rtk git add internal/cli internal/cmd/oaw-management-shadow
rtk git commit -m "feat: route management through public oaw cli"
```

### Task 2: Replace the Bash Implementation with a Colocated-Binary Wrapper

**Files:**
- Modify: `install.sh`
- Modify: `tests/run.sh`
- Modify: `tests/test-helper.sh`
- Modify: `tests/03-claude-lifecycle-test.sh`
- Modify: `tests/04-core-adapters-test.sh`
- Modify: `tests/05-policy-coordination-test.sh`
- Modify: `tests/05-project-adapters-test.sh`
- Modify: `tests/06-security-test.sh`
- Modify: `tests/09-transaction-test.sh`
- Modify: `tests/11-check-parity-test.sh`
- Modify: `tests/12-install-parity-test.sh`
- Modify: `tests/13-mutation-parity-test.sh`
- Create: `tests/14-cutover-release-test.sh`

- [ ] **Step 1: Add failing wrapper contract tests**

Create `tests/14-cutover-release-test.sh` with an isolated release fixture containing a copied `install.sh` and a compiled `oaw`. Assert:

- wrapper and direct binary results match for global help, `check`, dry-run install, install, update, and uninstall;
- the wrapper source contains no `source`/`.` imports from `lib`, `curl`, `wget`, `git clone`, `go run`, or `PATH` lookup;
- a missing or non-executable sibling binary fails without changing HOME/XDG/project roots;
- a hostile `PATH` containing a fake `oaw` is never executed;
- the wrapper supports sibling `oaw` and sibling `oaw.exe` names only.

Use known literal output/status expectations from the existing CLI tests rather than computing expected management results with the Go implementation.

- [ ] **Step 2: Run the wrapper test and verify RED**

Run:

```bash
rtk bash tests/14-cutover-release-test.sh wrapper
```

Expected: FAIL because `install.sh` still sources and executes the Bash management implementation.

- [ ] **Step 3: Implement the compatibility wrapper**

Replace `install.sh` with Bash 3.2-compatible logic equivalent to:

```bash
#!/usr/bin/env bash
set -eu
OAW_RELEASE_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
OAW_EXECUTABLE=$OAW_RELEASE_DIR/oaw
if [ ! -f "$OAW_EXECUTABLE" ]; then
  OAW_EXECUTABLE=$OAW_RELEASE_DIR/oaw.exe
fi
if [ ! -f "$OAW_EXECUTABLE" ] || [ ! -x "$OAW_EXECUTABLE" ]; then
  printf 'oaw: error: precompiled sibling binary is missing or not executable\n' >&2
  exit 70
fi
exec "$OAW_EXECUTABLE" "$@"
```

Do not add a `PATH` fallback, compiler invocation, network operation, self-update, or environment-variable binary override.

- [ ] **Step 4: Adapt the black-box harness without weakening changed-checkout coverage**

In `tests/run.sh`, create a temporary release fixture, compile `cmd/oaw` into it, copy the wrapper beside it, export that wrapper as `OAW_INSTALLER`, and remove the fixture on exit. Include `14-cutover-release-test.sh` in the suite.

In `tests/test-helper.sh`, preserve the initial installer as `OAW_BASE_INSTALLER` and provide:

```bash
build_checkout_installer() {
  checkout=$1
  (cd "$checkout" && go build -o "$checkout/oaw" ./cmd/oaw)
}
```

Every copied checkout whose `VERSION` or `policy/ENGINEERING.md` is changed must call this helper after the edit and before selecting its wrapper. Every reset must use `OAW_BASE_INSTALLER`, not the source checkout's wrapper. This preserves tests proving that changed embedded sources, rather than stale release bytes, drive update behavior.

Update Tickets 11-13 parity scripts to execute the public `cmd/oaw` binary instead of the deleted shadow driver. Their post-cutover purpose is compatibility forwarding and historical behavior regression, not independent management authority.

- [ ] **Step 5: Run wrapper and complete black-box tests for GREEN**

Run:

```bash
rtk bash -n install.sh tests/*.sh scripts/*.sh
rtk shellcheck -S warning -x install.sh tests/*.sh scripts/*.sh
rtk bash tests/14-cutover-release-test.sh wrapper
rtk bash tests/run.sh
```

Expected: PASS. Every prior installer behavior case continues to pass through the wrapper/public Go binary.

- [ ] **Step 6: Commit the wrapper cutover slice**

```bash
rtk git add install.sh tests
rtk git commit -m "feat: make installer a binary compatibility wrapper"
```

### Task 3: Build Offline Cross-Platform Release Archives

**Files:**
- Modify: `.gitignore`
- Create: `scripts/build-release.sh`
- Create: `scripts/smoke-wsl.sh`
- Modify: `tests/14-cutover-release-test.sh`

- [ ] **Step 1: Add failing release archive tests**

Extend `tests/14-cutover-release-test.sh` with a `release` mode that invokes `scripts/build-release.sh` into a fresh temporary directory and asserts exactly these archives plus `SHA256SUMS`:

```text
open-agent-workflow_0.1.0_darwin_amd64.tar.gz
open-agent-workflow_0.1.0_darwin_arm64.tar.gz
open-agent-workflow_0.1.0_linux_amd64.tar.gz
open-agent-workflow_0.1.0_linux_arm64.tar.gz
open-agent-workflow_0.1.0_windows_amd64.tar.gz
open-agent-workflow_0.1.0_windows_arm64.tar.gz
```

For every archive assert one top-level directory containing `CHANGELOG.md`, `LICENSE`, `README.md`, `README-zh.md`, `VERSION`, `install.sh`, and exactly one precompiled `oaw` or `oaw.exe`. Verify all checksums. Extract the current `GOOS/GOARCH` archive and prove both the binary and wrapper run global help, `catalog validate`, and isolated `check` without creating Runtime State.

- [ ] **Step 2: Run the release test and verify RED**

Run:

```bash
rtk bash tests/14-cutover-release-test.sh release
```

Expected: FAIL because no release builder exists.

- [ ] **Step 3: Implement the release builder**

Add `/dist/` to `.gitignore`. Implement `scripts/build-release.sh [output-directory]` with these properties:

- reads and validates the repository `VERSION`;
- fails if a planned archive or checksum file already exists instead of deleting caller data;
- uses a private `mktemp -d` staging directory removed by a trap;
- builds `cmd/oaw` with `CGO_ENABLED=0`, `-trimpath`, and the six exact `GOOS/GOARCH` targets;
- copies only the named release documents and wrapper;
- writes `.exe` only for Windows and preserves executable modes;
- creates `.tar.gz` archives without fetching any source or executable code;
- writes sorted SHA-256 records using available local `shasum -a 256` or `sha256sum`.

The script must not clone, download, install tools, execute an artifact from the network, or package `lib/` as a runtime dependency.

- [ ] **Step 4: Implement an actual-WSL smoke gate**

Add `scripts/smoke-wsl.sh <linux-release-archive>` that returns `77` with a truthful `SKIP` diagnostic unless `/proc/sys/kernel/osrelease` identifies Microsoft WSL. Inside WSL it must:

- reject a non-absolute or missing archive path;
- extract into a private temporary directory;
- run the Linux binary and wrapper help;
- run `catalog validate` and an isolated `check`;
- assert Policy-only fixture files and Install State are not imported into the Runtime State directory;
- print one `PASS` line and return zero only after all checks succeed.

No non-WSL environment may be reported as a passing WSL run.

- [ ] **Step 5: Run archive, cross-build, and local WSL-detection tests for GREEN**

Run:

```bash
rtk bash tests/14-cutover-release-test.sh release
rtk bash scripts/smoke-wsl.sh /absolute/path/to/linux-archive
```

Expected: release PASS. On actual WSL, smoke PASS; outside WSL, status 77 with `SKIP`, which must be recorded as an unavailable external platform gate rather than a pass.

- [ ] **Step 6: Commit the packaging slice**

```bash
rtk git add .gitignore scripts/build-release.sh scripts/smoke-wsl.sh tests/14-cutover-release-test.sh
rtk git commit -m "feat: build offline cross-platform release archives"
```

### Task 4: Publish Truthful Cutover and Runtime Boundaries

**Files:**
- Modify: `README.md`
- Modify: `README-zh.md`
- Modify: `CHANGELOG.md`
- Modify: `CONTRIBUTING.md`
- Modify: `CONTRIBUTING-zh.md`
- Modify: `SECURITY.md`
- Modify: `SECURITY-zh.md`
- Modify: `docs/en/installer.md`
- Modify: `docs/zh/installer.md`
- Modify: `docs/en/architecture.md`
- Modify: `docs/zh/architecture.md`
- Modify: `docs/en/troubleshooting.md`
- Modify: `docs/zh/troubleshooting.md`
- Modify: `docs/en/extending-adapters.md`
- Modify: `docs/zh/extending-adapters.md`
- Modify: `scripts/check-docs.sh`
- Modify: `tests/10-docs-test.sh`

- [ ] **Step 1: Write failing documentation-contract assertions**

In `tests/10-docs-test.sh` and `scripts/check-docs.sh`, require English and Chinese documentation to state all of these literal boundaries:

- public management is Go-authoritative;
- `install.sh` is an offline colocated-binary compatibility wrapper;
- release archives contain precompiled binaries and perform no runtime executable download;
- Install State and Runtime State are disjoint and no automatic migration occurs;
- existing Policy-only tasks/profile locks remain Policy-only unless explicitly adopted at a Stable Boundary;
- only the pinned Codex runner is currently Runtime-managed;
- other installed adapters remain Policy-only and provide no Runtime admission, Grant, lease, transition-enforcement, or physical-isolation guarantee;
- an actual WSL smoke pass is required before publishing a release.

Also reject the stale statements `Bash remains authoritative`, `public oaw install is not enabled`, and `zero-dependency Bash installer` from current user-facing docs.

- [ ] **Step 2: Run the docs tests and verify RED**

Run:

```bash
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
```

Expected: FAIL on stale pre-cutover authority language.

- [ ] **Step 3: Update bilingual docs and release notes**

Document source-checkout and archive usage separately. A source checkout must first build `./oaw`; a release archive already contains the correct binary. Keep `./install.sh` examples as compatibility examples and add direct `./oaw` examples as the primary interface.

Update the 0.1.0 local candidate release notes without claiming remote publication. Explicitly distinguish the selected Codex `runner-managed` path from all Policy-only targets and state that management cutover does not promote any Host integration.

Update security and contributor contracts from Bash-authoritative behavior to the Go binary plus minimal Bash wrapper, while retaining Bash 3.2 and exact black-box wrapper compatibility requirements.

- [ ] **Step 4: Run documentation checks for GREEN**

Run:

```bash
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
```

Expected: PASS with link and bilingual contract checks intact.

- [ ] **Step 5: Commit documentation and release notes**

```bash
rtk git add README.md README-zh.md CHANGELOG.md CONTRIBUTING.md CONTRIBUTING-zh.md SECURITY.md SECURITY-zh.md docs scripts/check-docs.sh tests/10-docs-test.sh
rtk git commit -m "docs: record go management cutover boundaries"
```

### Task 5: Review, Verify, and Record Ticket 14 Completion Evidence

**Files:**
- Modify: `.scratch/oaw-runtime-vnext/issues/14-cutover-and-release-verification.md`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`

- [ ] **Step 1: Run inline two-axis review**

Review `main...HEAD` once for repository standards/security and separately for Ticket 14/spec sections 15-19. Inspect wrapper trust, archive contents, management/runtime state separation, error streams/status, changed-checkout semantics, cross-platform paths, release truthfulness, and any file/function size regression. Record every finding and remediation; do not mark the issue complete with unresolved Critical, High, Important, Standards, or Spec findings.

- [ ] **Step 2: Run fresh full verification at one fixed point**

Run:

```bash
rtk gofmt -w internal/cli
rtk git diff --check main...HEAD
rtk go test ./... -count=1
rtk go test -race ./... -count=1
rtk go test ./... -coverprofile=/tmp/oaw-ticket-14-coverage.out -count=1
rtk go tool cover -func=/tmp/oaw-ticket-14-coverage.out
rtk go vet ./...
rtk bash -n install.sh tests/*.sh scripts/*.sh
rtk shellcheck -S warning -x install.sh tests/*.sh scripts/*.sh
rtk bash tests/run.sh
rtk bash scripts/check-docs.sh
rtk go test ./internal/classification -run 'Eval|Critical' -count=1
rtk go test ./internal/host ./internal/integration -run 'Conformance|Ticket08' -count=1
rtk go test ./internal/runtime -run '^$' -fuzz '^FuzzDecodeFrameFailsClosed$' -fuzztime=2s
rtk go test ./internal/host/codex -run '^$' -fuzz '^FuzzNormalizeJSONLFailsClosed$' -fuzztime=2s
rtk go vet ./...
```

Build all six release targets into a temporary output directory and verify archives/checksums with `tests/14-cutover-release-test.sh release`. Run a locally available official Go vulnerability scanner without invoking any model-backed command. Do not run real `codex exec`.

Run `scripts/smoke-wsl.sh` inside actual WSL. If this machine has no WSL kernel, record the status-77 external gate honestly and do not claim Ticket 14 release readiness until a real WSL pass is supplied.

- [ ] **Step 3: Record evidence and close only achieved acceptance items**

Append fixed-point commands and exact results to review/verification evidence. Set checked issue items only for gates with fresh evidence. Mark the ticket `completed` and tracker stage complete only if every release gate, including actual WSL, passed. Otherwise leave it `in-progress` with the one exact external gate named.

- [ ] **Step 4: Commit evidence at the verified fixed point**

```bash
rtk git add .scratch/oaw-runtime-vnext
rtk git commit -m "docs: record ticket 14 cutover verification"
```

## Approved Environment-Aware Platform Amendment (2026-08-03)

This amendment supersedes Task 3 Step 5, Task 4's mandatory-WSL wording, and
Task 5's actual-WSL completion condition. The user approved Docker as the only
non-macOS execution mechanism available during development and explicitly made
unavailable platform checks non-blocking. A recorded status-77 `SKIP` is not a
pass, but it no longer prevents Ticket 14 completion.

### Task 6: Share Linux Smoke and Add the Docker Executor

**Files:**
- Create: `scripts/smoke-linux.sh`
- Create: `scripts/smoke-docker.sh`
- Modify: `scripts/smoke-wsl.sh`
- Modify: `tests/14-cutover-release-test.sh`

- [ ] **Step 1: Add the failing Docker platform contract**

Add a `run_docker_contract` mode to `tests/14-cutover-release-test.sh`. It must
build all release archives into the existing private test directory, resolve
the Docker server architecture as `amd64` or `arm64`, and invoke:

```bash
bash "$REPOSITORY/scripts/smoke-docker.sh" \
  "$release_output/open-agent-workflow_${version}_linux_${docker_arch}.tar.gz"
```

Accept status 0 only with `PASS: Docker Linux release` output. Accept status 77
only with `SKIP:` output. Any other status fails the contract. Extend `all` to
run this contract after wrapper and archive verification, and add `docker` as
an explicit mode.

- [ ] **Step 2: Run the Docker contract and verify RED**

Run:

```bash
rtk bash tests/14-cutover-release-test.sh docker
```

Expected: FAIL because `scripts/smoke-docker.sh` does not exist.

- [ ] **Step 3: Extract the common Linux release smoke**

Move every assertion after WSL detection from `scripts/smoke-wsl.sh` into
`scripts/smoke-linux.sh <absolute-linux-release-archive>`. Keep the exact
archive path validation, safe extraction, executable checks, binary/wrapper
help parity, `catalog validate`, isolated install/uninstall, Install State
creation, Runtime State absence, and Policy-only checksum preservation. End
only after all assertions pass:

```bash
printf 'PASS: Linux release binary, wrapper, Install State, and Policy-only boundaries verified\n'
```

The common script performs no platform detection and no network operation.

- [ ] **Step 4: Reduce WSL smoke to detection plus delegation**

Keep the Microsoft-kernel check in `scripts/smoke-wsl.sh`, then resolve its own
physical script directory and delegate without changing the archive argument:

```bash
SCRIPT_DIR=$(CDPATH='' cd -P -- "$(dirname -- "$0")" && pwd)
exec bash "$SCRIPT_DIR/smoke-linux.sh" "$@"
```

Outside WSL it must still emit the existing `SKIP` diagnostic and return 77.

- [ ] **Step 5: Implement the Docker smoke executor**

Create `scripts/smoke-docker.sh <absolute-linux-release-archive>` with these
fixed behaviors:

```text
amd64 image: bash@sha256:534a5f1d11652aadaa9f08838f6637ac11a46a8b4b736a4cbf09c5945e38516f
arm64 image: bash@sha256:26b3d1c3d49066239fc1c44002f316c1893ca83f714c9fd9636e100d3e11224d
available server architectures: amd64, arm64
unavailable CLI/daemon/architecture/image: SKIP on stderr, status 77
common smoke assertion failure: status 1
successful common smoke: Docker-prefixed PASS, status 0
```

Validate the absolute regular archive before Docker use. Reuse a local image;
if absent, attempt one explicit image pull and convert pull unavailability to
status 77. Run the selected native Linux architecture with the archive and
common script mounted read-only, network disabled, all capabilities dropped,
`no-new-privileges`, a read-only root filesystem, and an executable private
`/tmp` tmpfs:

```bash
docker run --rm --network none --read-only \
  --tmpfs /tmp:rw,exec,nosuid,nodev \
  --cap-drop ALL --security-opt no-new-privileges \
  --platform "linux/$docker_arch" \
  --mount "type=bind,src=$ARCHIVE,dst=/input/release.tar.gz,readonly" \
  --mount "type=bind,src=$SCRIPT_DIR/smoke-linux.sh,dst=/input/smoke-linux.sh,readonly" \
  "$SMOKE_IMAGE" bash /input/smoke-linux.sh /input/release.tar.gz
```

After Docker status 125, recheck daemon availability. Map to 77 only when that
recheck fails; if the daemon remains reachable, preserve status 125 so invalid
arguments, mounts, or entrypoints remain blocking. Status 1 from the common
smoke also remains a blocking failure and must not be relabeled.

- [ ] **Step 6: Run focused GREEN verification**

Run:

```bash
rtk bash -n scripts/smoke-linux.sh scripts/smoke-docker.sh scripts/smoke-wsl.sh tests/14-cutover-release-test.sh
rtk shellcheck -S warning -x scripts/smoke-linux.sh scripts/smoke-docker.sh scripts/smoke-wsl.sh tests/14-cutover-release-test.sh
rtk bash tests/14-cutover-release-test.sh release
rtk bash tests/14-cutover-release-test.sh docker
```

Expected: release PASS; Docker PASS on the current available Docker Desktop
Linux/arm64 server. A genuinely unavailable Docker executor may instead return
the tested non-blocking status-77 `SKIP`.

- [ ] **Step 7: Commit the platform smoke slice**

```bash
rtk git add scripts/smoke-linux.sh scripts/smoke-docker.sh scripts/smoke-wsl.sh tests/14-cutover-release-test.sh
rtk git commit -m "test: verify linux releases through docker"
```

### Task 7: Publish Environment-Aware Release Verification

**Files:**
- Modify: `README.md`
- Modify: `README-zh.md`
- Modify: `CONTRIBUTING.md`
- Modify: `CONTRIBUTING-zh.md`
- Modify: `CHANGELOG.md`
- Modify: `scripts/check-docs.sh`
- Modify: `tests/10-docs-test.sh`

- [ ] **Step 1: Write failing bilingual documentation contracts**

Replace the mandatory-WSL literals with these exact release boundaries in both
documentation test surfaces:

```text
Available native and Docker smoke tests must pass; unavailable platform checks return 77 and do not block release readiness.
可用的原生和 Docker smoke test 必须通过；不可用的平台检查返回 77，且不阻塞 release readiness。
```

Reject the stale literals `actual WSL smoke pass is required before publishing
a release`, `release readiness remains blocked until`, and their Chinese
equivalents from current user-facing release documents.

- [ ] **Step 2: Run documentation tests and verify RED**

Run:

```bash
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
```

Expected: FAIL because README and contribution contracts still require WSL.

- [ ] **Step 3: Update bilingual guidance and release notes**

Document the Docker command, status-0 PASS, status-77 non-blocking SKIP, and the
rule that a skip is never reported as a pass. State that native archive
execution remains required on the current host, Linux execution uses Docker
when available, Windows/WSL execution may be skipped when unavailable, and all
six cross-builds/checksums remain mandatory. Describe Docker image preparation
as verification infrastructure, not release runtime behavior.

- [ ] **Step 4: Run documentation tests for GREEN**

Run:

```bash
rtk bash tests/10-docs-test.sh
rtk bash scripts/check-docs.sh
```

Expected: PASS with bilingual and link contracts intact.

- [ ] **Step 5: Commit documentation**

```bash
rtk git add README.md README-zh.md CONTRIBUTING.md CONTRIBUTING-zh.md CHANGELOG.md scripts/check-docs.sh tests/10-docs-test.sh
rtk git commit -m "docs: make unavailable platform smoke non-blocking"
```

### Task 8: Reverify and Close Ticket 14

**Files:**
- Modify: `.scratch/oaw-runtime-vnext/issues/14-cutover-and-release-verification.md`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`

- [ ] **Step 1: Run inline two-axis amendment review**

Review the amendment for repository/security standards and separately against
specification sections 18-19. Confirm Docker input validation, image pinning,
no-network execution, least privilege, exact status mapping, common assertion
ownership, truthful skip reporting, bilingual consistency, and preservation of
offline release behavior. Remediate every Critical, High, Important,
Standards, or Spec finding before completion.

- [ ] **Step 2: Run fresh fixed-point verification**

Run the complete Task 5 verification matrix, then additionally run:

```bash
rtk bash tests/14-cutover-release-test.sh docker
rtk bash scripts/smoke-wsl.sh "$PWD/dist/open-agent-workflow_0.1.0_linux_$(go env GOARCH).tar.gz"
```

Docker must return status 0 on the currently available Docker Desktop server.
The macOS WSL probe may return status 77 and remains truthful, non-blocking
evidence. Rebuild `dist/` only in a fresh output directory because the release
builder refuses overwrite.

- [ ] **Step 3: Record completion evidence**

Update Ticket 14 acceptance language to require cross-platform builds plus
environment-aware smoke. Check every acceptance item backed by fresh evidence,
set the issue status to `completed`, set the workflow stage to completed, and
record Docker PASS plus WSL SKIP separately. Do not claim native Windows or WSL
execution.

- [ ] **Step 4: Commit completion evidence**

```bash
rtk git add .scratch/oaw-runtime-vnext
rtk git commit -m "docs: complete ticket 14 environment verification"
```

## Plan Self-Review

- Spec coverage: Tasks 1-4 cover migration step 8, state non-import, CLI authority, archives, and the original release boundaries; Tasks 6-8 implement the approved environment-aware Docker/optional-platform amendment and close evidence.
- Placeholder scan: no TBD, TODO, deferred implementation instruction, or unnamed test remains.
- Type consistency: production identifiers consistently use `managementCommand`, `runManagement`, `parseManagement`, `managementEnvironment`, and `executeManagement`; Install State and Runtime State remain distinct names and paths.
- Ownership consistency: Matt `tdd` owns every RED/GREEN loop; Superpowers owns this plan, implementation orchestration, review, verification, and completion. Superpowers TDD remains paused and no subagent is dispatched.
