# OAW Codex Host Bridge 06: Documentation, Verification, and Release Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Document the Codex Host Bridge in English and Chinese, exercise its protocol and security boundaries in black-box tests, run supported macOS and Docker Linux verification, and leave a reproducible release gate without publishing anything.

**Architecture:** Product documentation describes OAW as a policy/compiler and optional Coordinator whose physical execution remains Host-owned. Bridge documentation explicitly separates policy installation from opt-in Plugin installation, records the exact `CURRENT` topology, and explains that only a fresh Hook observation can prove a current session. Verification is layered: deterministic Go tests first, isolated Docker tests second, and one explicitly approved real macOS Codex dogfood last.

**Tech Stack:** Markdown, Bash, Go 1.26, Docker, Codex CLI 0.146.1 baseline, local-only evidence artifacts, `scripts/check-docs.sh`.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish, push, tag, or create a release from this plan.

**Depends on:** Plans 01-05 and the controlled-dogfooding design at `docs/superpowers/specs/2026-08-03-oaw-controlled-dogfooding-design.md`.

**Produces:** Bilingual operator docs, repeatable protocol/security gates, local dogfood evidence, and a release-readiness report.

## File Map

| Path | Responsibility |
| --- | --- |
| `docs/en/codex-bridge.md` | English Bridge installation, trust, operation, diagnostics, and recovery guide. |
| `docs/zh/codex-bridge.md` | Chinese translation with identical command and safety contracts. |
| `docs/en/architecture.md` | Add Bridge as Host integration without changing Core/Coordinator boundaries. |
| `docs/zh/architecture.md` | Chinese architecture projection. |
| `docs/en/installer.md` | Separate `oaw install --target codex` and `oaw bridge ...` command families. |
| `docs/zh/installer.md` | Chinese installer projection. |
| `docs/en/troubleshooting.md` | Bridge diagnostic codes and recovery table. |
| `docs/zh/troubleshooting.md` | Chinese diagnostic table. |
| `docs/en/security.md` | Hook, metadata minimization, same-user limitation, and approval boundary. |
| `docs/zh/security.md` | Chinese security projection. |
| `README.md` / `README-zh.md` | Public architecture and verification statements. |
| `scripts/check-docs.sh` | Updated release boundaries and Bridge vocabulary checks. |
| `tests/10-docs-test.sh` | Checker fixtures, document pairs, and stale-boundary regression assertions. |
| `scripts/check-codex-bridge.sh` | Local deterministic Bridge verification gate. |
| `scripts/smoke-codex-bridge.sh` | macOS real-Codex smoke with explicit `SKIP=77`. |
| `scripts/smoke-codex-bridge-docker.sh` | Linux Docker protocol/filesystem smoke. |
| `tests/18-codex-bridge-protocol-test.sh` | Black-box command, Hook, and forbidden-path tests. |
| `internal/integration/codex_bridge_blackbox_test.go` | In-memory MCP and Core/Coordinator end-to-end transcript tests. |
| `.scratch/oaw-codex-host-bridge-dogfood/` | Local-only evidence; never commit or parse as authority. |

## Task 1: Write the bilingual Bridge operator guide

**Files:**
- Create: `docs/en/codex-bridge.md`
- Create: `docs/zh/codex-bridge.md`
- Modify: `docs/en/installer.md`
- Modify: `docs/zh/installer.md`

- [ ] **Step 1: Write the English guide with the exact public contract**

The guide must contain these exact command examples and operational rules:

```text
oaw install --target codex
oaw bridge install codex
oaw bridge check codex
oaw bridge update codex
oaw bridge uninstall codex
oaw bridge serve codex
oaw bridge hook codex
```

Document that `install --target codex` distributes only `ENGINEERING.md`,
while `bridge install codex` is an opt-in executable Plugin transaction. State
that the user must inspect the exact four Hook matchers and start a new Codex
session after install/update. Explain that the active Codex session invokes
Skills/tools; OAW only observes metadata, compiles policy, and exchanges
Coordinator records. Document the exact `PreToolUse` contract: observation
returns nested `hookSpecificOutput` with `hookEventName = PreToolUse`, `allow`,
and object-valued `updatedInput`; valid later operations return no stdout; a
session/cwd mismatch returns the nested `deny` form. List the stable diagnostic
codes from the approved design with one concrete recovery command for each.

- [ ] **Step 2: Translate the same contract into Chinese**

Keep command names, JSON field names, diagnostic codes, topology names, and
exit statuses byte-for-byte unchanged. Translate prose, not protocol tokens.
Include a visible statement that the Bridge does not create a child session or
copy Host extensions, credentials, sandbox, or approval configuration.

- [ ] **Step 3: Update installer references**

Add a dedicated `Codex Host Bridge` section to both installer documents. The
section must distinguish OAW-owned install state below the XDG state root and
OAW-owned binary/marketplace files below the XDG data root from Codex-owned
Plugin cache/config, describe rollback and drift behavior, and state that
uninstall invokes official Codex removal before deleting only clean OAW files.

- [ ] **Step 4: Run documentation RED/GREEN checks**

```bash
rtk bash scripts/check-docs.sh
```

Expected before the matching changes to other docs: FAIL with missing paired
document or release-boundary messages. After the task, it must pass.

- [ ] **Step 5: Commit the operator guide**

```bash
rtk git add docs/en/codex-bridge.md docs/zh/codex-bridge.md docs/en/installer.md docs/zh/installer.md
rtk git commit -m "docs: document Codex Host Bridge operations"
```

## Task 2: Update architecture, lifecycle, security, and README projections

**Files:**
- Modify: `README.md`
- Modify: `README-zh.md`
- Modify: `docs/en/architecture.md`
- Modify: `docs/zh/architecture.md`
- Modify: `docs/en/lifecycle.md`
- Modify: `docs/zh/lifecycle.md`
- Modify: `docs/en/security.md`
- Modify: `docs/zh/security.md`
- Modify: `docs/en/troubleshooting.md`
- Modify: `docs/zh/troubleshooting.md`
- Modify: `docs/en/adapters.md`
- Modify: `docs/zh/adapters.md`
- Modify: `scripts/check-docs.sh`
- Modify: `tests/10-docs-test.sh`

- [ ] **Step 1: Replace stale Host claims**

Replace the old statement that all built-in integrations are policy-only with
the precise split:

```text
Codex has a policy integration by default and a separate audited host-native
Bridge that must be explicitly installed and trusted. The Bridge v1 supports
CURRENT and skill bindings only; all other Host surfaces remain unknown unless
the Host reports stable evidence.
```

Update lifecycle diagrams to show `observe_current -> Core inspect -> explicit
Startup Gate -> Core compile/Coordinator START`, and show Direct/Bounded paths
without the Bridge. Update the adapters table with `oaw/codex-host` as an
optional host-native surface while retaining `oaw/codex-policy`.

- [ ] **Step 2: Add the diagnostics and security boundary text**

The troubleshooting tables must include, at minimum:

```text
HOST_BRIDGE_UNAVAILABLE
HOST_BRIDGE_CONTEXT_REQUIRED
HOST_BRIDGE_PROTOCOL_MISMATCH
HOST_EVIDENCE_HANDLE_REQUIRED
HOST_EVIDENCE_HANDLE_INVALID
HOST_EVIDENCE_EXPIRED
HOST_EVIDENCE_SESSION_MISMATCH
HOST_OBSERVATION_FAILED
HOST_OBSERVATION_PARTIAL
HOST_SESSION_CHANGED
```

Security docs must state that Hook input is the only current-session identity
source, `skills/list` is the only v1 Provider binding authority, `plugin/list`
is not a production dependency, and a same-user process can interfere with
local programs. Do not promise inherited MCP, Hook, Skill, Plugin, model,
authentication, sandbox, or approval behavior beyond observed Host facts.

- [ ] **Step 3: Update `scripts/check-docs.sh` release boundaries**

Replace the exact old README assertion beginning `All nine built-in
integrations currently expose the policy surface` in both
`scripts/check-docs.sh` and the checker fixtures/assertions in
`tests/10-docs-test.sh`. Add these paired requirements:

```bash
README.md|Codex has a policy integration by default and a separate audited host-native Bridge
README-zh.md|Codex 默认提供 policy integration，并另有独立且经过审计的 host-native Bridge
docs/en/architecture.md|oaw/codex-host
docs/zh/architecture.md|oaw/codex-host
docs/en/lifecycle.md|observe_current
docs/zh/lifecycle.md|observe_current
docs/en/troubleshooting.md|HOST_BRIDGE_PROTOCOL_MISMATCH
docs/zh/troubleshooting.md|HOST_BRIDGE_PROTOCOL_MISMATCH
```

Keep the existing paired-document, local-link, stale-vocabulary, schema-field,
and no-absolute-link checks. Add `docs/en/codex-bridge.md` and
`docs/zh/codex-bridge.md` to the paired-document list and reject claims that
OAW starts a model/child process or guarantees Host extension inheritance.
Update `make_checker_fixture` in `tests/10-docs-test.sh` with the same new
document pair and release-boundary literals so the checker is tested against
the new contract instead of the removed nine-policy assertion.

- [ ] **Step 4: Run GREEN documentation checks**

```bash
rtk bash scripts/check-docs.sh
rtk bash tests/10-docs-test.sh
rtk git diff --check
```

Expected: PASS with both language projections present and no stale execution
vocabulary.

- [ ] **Step 5: Commit documentation projections**

```bash
rtk git add README.md README-zh.md docs/en/architecture.md docs/zh/architecture.md docs/en/lifecycle.md docs/zh/lifecycle.md docs/en/security.md docs/zh/security.md docs/en/troubleshooting.md docs/zh/troubleshooting.md docs/en/adapters.md docs/zh/adapters.md scripts/check-docs.sh tests/10-docs-test.sh
rtk git commit -m "docs: align architecture with Codex Host Bridge"
```

## Task 3: Add deterministic in-memory and black-box protocol coverage

**Files:**
- Create: `internal/integration/codex_bridge_blackbox_test.go`
- Create: `tests/18-codex-bridge-protocol-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write failing integration assertions**

```go
func TestCodexBridgeCurrentWorkflowTranscript(t *testing.T) {
	service := newCodexBridgeFixture(t)
	observed := callObserveThroughMCP(t, service)
	inspection := callInspectThroughMCP(t, service, observed.HostEvidenceHandle)
	if inspection.Compilation == nil || len(inspection.Compilation.EligibleProfiles) == 0 || inspection.HostSummary.SessionDigest == "" { t.Fatalf("inspection=%#v", inspection) }
	compiled := callCompileThroughMCP(t, service, observed.HostEvidenceHandle, explicitCurrentSelection(t, inspection))
	started := callWorkflowStartThroughMCP(t, service, observed.HostEvidenceHandle, compiled)
	if started.Snapshot == nil || len(started.Snapshot.Bundles) != 1 || started.Snapshot.Bundles[0].HostSessionDigest[:16] != inspection.HostSummary.SessionDigest { t.Fatalf("started=%#v", started) }
}

func TestDirectPathDoesNotRequireBridge(t *testing.T) {
	if status := runWithBridgeDisabled(t, []string{"catalog", "validate"}); status != 0 { t.Fatalf("status=%d", status) }
}
```

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/integration -run 'CodexBridge|DirectPath'
```

Expected: FAIL until the integration fixture and MCP calls are wired.

- [ ] **Step 3: Add shell boundary checks**

```bash
#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "$0")/test-helper.sh"
assert_exit 0 run_oaw catalog validate
assert_exit 64 run_oaw bridge hook claude
assert_not_contains "$(run_oaw help)" "codex exec"
assert_not_contains "$(run_oaw help)" "NATIVE_SUBAGENT"
```

Use a temporary stdin fixture for `bridge hook codex`; assert an observation
input with `hook_event_name=PreToolUse` receives
`hookSpecificOutput.hookEventName=PreToolUse`,
`hookSpecificOutput.permissionDecision=allow`, and
`hookSpecificOutput.updatedInput._oaw_host_context`, while a valid input for
each of the other three tool names produces zero stdout bytes. Assert a missing
or wrong `hook_event_name`, an edited/expired handle, a foreign session/cwd, a
disabled Skill, an orphan Skill, and a foreign Host Candidate all fail closed;
the three later-operation context failures must return
`hookSpecificOutput.permissionDecision=deny` with no `updatedInput`.

- [ ] **Step 4: Run integration and shell GREEN**

```bash
rtk gofmt -w internal/integration/codex_bridge_blackbox_test.go
rtk go test ./internal/integration -run 'CodexBridge|DirectPath'
rtk bash tests/18-codex-bridge-protocol-test.sh
rtk git diff --check
```

Expected: PASS.

- [ ] **Step 5: Commit protocol coverage**

```bash
rtk git add internal/integration/codex_bridge_blackbox_test.go tests/18-codex-bridge-protocol-test.sh tests/run.sh
rtk git commit -m "test: cover Codex Bridge protocol boundaries"
```

## Task 4: Add macOS and Docker verification scripts

**Files:**
- Create: `scripts/check-codex-bridge.sh`
- Create: `scripts/smoke-codex-bridge.sh`
- Create: `scripts/smoke-codex-bridge-docker.sh`

- [ ] **Step 1: Add the deterministic local gate**

```bash
#!/usr/bin/env bash
set -eu
go test ./internal/codexbridge/... ./internal/integration
go test -race ./internal/codexbridge/... ./internal/integration
go vet ./internal/codexbridge/... ./internal/integration
bash tests/18-codex-bridge-protocol-test.sh
bash scripts/check-docs.sh
```

The committed script uses ordinary shell commands when run by users. `rtk`
appears only in the separate commands the implementing Agent invokes from this
plan.
Add scans that production source contains no `plugin/list`, model/thread/turn
launch, private projected environment, or legacy topology alias.

- [ ] **Step 2: Add the macOS real-Codex smoke with explicit skips**

```bash
if [ "$(uname -s)" != "Darwin" ]; then
  printf 'SKIP: Codex Bridge macOS smoke requires Darwin\n' >&2
  exit 77
fi
command -v codex >/dev/null 2>&1 || { printf 'SKIP: codex CLI unavailable\n' >&2; exit 77; }
codex --version
```

The script checks `codex plugin marketplace list --json`, verifies that local
Codex help advertises `--json` for `plugin add`, `plugin remove`, `plugin list`,
and marketplace add/list/remove, starts the Bridge check command, and runs the
fake-transcript protocol smoke. It does not install into a user's real Codex
configuration automatically. The real session steps in Task 5 require an
explicit operator checkpoint.

- [ ] **Step 3: Add Docker Linux smoke**

The Docker script detects a reachable Docker daemon, selects a pinned
`bash`/Go verification image by `linux/amd64` or `linux/arm64`, mounts the
repository and the host-prepared Go module cache read-only, supplies writable
`tmpfs` storage for `/tmp` and `GOCACHE`, disables network and capabilities,
runs the deterministic Bridge tests with `GOPROXY=off` and `-mod=readonly`, and
returns 77 when Docker is unavailable or the architecture is unsupported.
`REPOSITORY` is set from `git rev-parse --show-toplevel`; `MODULE_CACHE` is set
from `go env GOMODCACHE` after `go mod download`; `IMAGE` is selected from a
checked-in architecture-to-digest map, and an unknown architecture returns 77
before Docker is invoked. The temporary filesystem must remain executable
because Go test binaries are launched from it.
It must not attempt to run a real Codex CLI inside Linux.

```bash
command -v docker >/dev/null 2>&1 || { printf 'SKIP: Docker CLI unavailable\n' >&2; exit 77; }
docker version >/dev/null 2>&1 || { printf 'SKIP: Docker daemon unavailable\n' >&2; exit 77; }
go mod download
MODULE_CACHE=$(go env GOMODCACHE)
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges --tmpfs /tmp:rw,exec,nosuid,nodev,size=512m -e GOCACHE=/tmp/go-cache -e GOMODCACHE=/go/pkg/mod -e GOPROXY=off -v "$MODULE_CACHE:/go/pkg/mod:ro" -v "$REPOSITORY:/src:ro" -w /src "$IMAGE" sh -c 'go test -mod=readonly ./internal/codexbridge/... ./internal/integration'
```

- [ ] **Step 4: Run platform scripts**

```bash
rtk bash scripts/check-codex-bridge.sh
rtk bash scripts/smoke-codex-bridge.sh
rtk bash scripts/smoke-codex-bridge-docker.sh
```

Expected: local gate passes; macOS smoke passes or returns 77 only when its
prerequisites are absent; Docker smoke passes or returns 77 only when Docker
cannot run. A 77 skip is recorded and does not block supported environments.

- [ ] **Step 5: Commit platform gates**

```bash
rtk git add scripts/check-codex-bridge.sh scripts/smoke-codex-bridge.sh scripts/smoke-codex-bridge-docker.sh
rtk git commit -m "test: add Codex Bridge platform gates"
```

## Task 5: Execute controlled macOS dogfood in the approved repository

**Files:**
- Create: `.scratch/oaw-codex-host-bridge-dogfood/README.md`
- Create: `.scratch/oaw-codex-host-bridge-dogfood/preflight.json`
- Create: `.scratch/oaw-codex-host-bridge-dogfood/observation.json`
- Create: `.scratch/oaw-codex-host-bridge-dogfood/inspection.json`
- Create: `.scratch/oaw-codex-host-bridge-dogfood/verification.json`
- Create: `.scratch/oaw-codex-host-bridge-dogfood/cleanup.json`

- [ ] **Step 1: Capture the clean pilot preflight**

Run against `/Users/wifibaby4u/LLM/open-code-review` without changing its
remote or publishing artifacts:

```bash
rtk git -C /Users/wifibaby4u/LLM/open-code-review status --short --branch
rtk git -C /Users/wifibaby4u/LLM/open-code-review rev-parse HEAD
rtk codex --version
rtk go run ./cmd/oaw bridge check codex --format json
```

Record the exact baseline, current branch, Codex version, Bridge version, and
whether the Plugin is already enabled. Abort the live step if the repository is
not clean; leave only a local diagnostic artifact.

- [ ] **Step 2: Install and review the opt-in Plugin**

Build the current binary, run `oaw bridge install codex`, inspect the rendered
manifest pointers (`skills`, `mcpServers`, and `hooks`), `hooks/hooks.json`, and
MCP command path, then use Codex's `/hooks` view to review/trust the exact Hook.
Start a new Codex session before observing. Do not use a bypass-trust flag.
Capture Hook hash and install state digests, never raw Hook commands or
credentials.

- [ ] **Step 3: Prove current-session observation and Provider resolution**

In the new Codex session, call `observe_current`, then `core.inspect`; record
only the secret-free outputs. Confirm at least one actually enabled Skill maps
to exactly one discovered installation and that absent Matt/ECC Skills remain
Candidate/ineligible. Confirm `CURRENT` is the only topology and Direct Mode
still works with Bridge disabled.

- [ ] **Step 4: Run one controlled Workflow**

Use `/Users/wifibaby4u/LLM/open-code-review` on an isolated branch or worktree,
select a previously approved Profile explicitly at the Startup Gate, compile
with `CURRENT`, exchange `START`, perform only the bounded approved pilot
deliverable, submit a normalized Receipt, and capture verification evidence.
Do not commit or push pilot changes. Stop on any Host fact change, unexpected
approval behavior, or missing evidence.

- [ ] **Step 5: Uninstall and capture cleanup**

After the pilot, run `oaw bridge uninstall codex`, verify official Plugin and
marketplace removal, verify only clean OAW-owned files disappeared, and start a
fresh session to confirm the policy-only surface does not claim host-native
evidence. Preserve all artifacts under `.scratch` and do not add them to Git.

Before the final commit, prove the dogfood directory remains untracked and is
absent from the index:

```bash
if rtk git ls-files --error-unmatch .scratch/oaw-codex-host-bridge-dogfood >/dev/null 2>&1; then exit 1; fi
if rtk git diff --cached --name-only | rtk rg -q '^\.scratch/oaw-codex-host-bridge-dogfood/'; then exit 1; fi
```

- [ ] **Step 6: Review dogfood evidence**

```bash
rtk jq -e '.session_id_digest and .inventory_digest' .scratch/oaw-codex-host-bridge-dogfood/observation.json
rtk jq -e '.profile and .topology == "CURRENT"' .scratch/oaw-codex-host-bridge-dogfood/inspection.json
rtk jq -e '.status == "completed"' .scratch/oaw-codex-host-bridge-dogfood/verification.json
```

Expected: all checks pass; no file contains a HostEvidenceHandle token,
credential, raw App Server config, or full absolute Skill path.

## Task 6: Run the final release-readiness gate without publishing

- [ ] **Step 1: Run the complete supported test set**

```bash
rtk go test ./...
rtk go test -race ./...
rtk bash tests/run.sh
rtk bash scripts/check-docs.sh
rtk bash scripts/check-codex-bridge.sh
```

Expected: PASS. Any unavailable platform command is recorded as `SKIP`/77,
not hidden as success.

- [ ] **Step 2: Verify hard-cutover exclusions**

```bash
rtk rg -n 'codex exec|thread/start|thread/resume|thread/fork|turn/start|turn/steer|plugin/list|private.*HOME|projected.*config|NATIVE_SUBAGENT|INLINE|oaw/codex-runner' --glob '*.go' --glob '!**/*_test.go' internal/codexbridge internal/cli
rtk rg -n 'codex exec|thread/start|thread/resume|thread/fork|turn/start|turn/steer|plugin/list|private.*HOME|projected.*config|NATIVE_SUBAGENT|INLINE|oaw/codex-runner' --glob '*.json' --glob '*.md' internal/codexbridge/install/assets
```

Expected: production files have no forbidden path. Approved negative tests,
design documents, and the verification script's own forbidden-pattern list
are outside these production-only scan roots.

- [ ] **Step 3: Inspect status and produce a local report**

```bash
rtk git status --short --branch
rtk git diff --check
rtk git log -n 12 --oneline
```

Write `.scratch/oaw-codex-host-bridge-dogfood/release-readiness.md` with test
commands, PASS/SKIP results, Docker architecture, macOS Codex version, and
unresolved follow-up tickets. Do not commit the report, push, tag, merge, or
create a release in this phase.

- [ ] **Step 4: Verify the tracked implementation is already committed**

```bash
rtk git diff --name-only
rtk git diff --cached --name-only
```

Expected: no Bridge source, documentation, script, or test remains uncommitted;
the only dogfood and release-readiness artifacts are untracked under
`.scratch`. Do not create an empty completion commit. If verification required
a tracked correction, stage only its explicit paths, commit it with the
appropriate conventional type, rerun the affected gates, and confirm the index
contains no `.scratch`, user files, generated local binary, credentials, or
unrelated changes. This plan deliberately does not run `pre-commit-review`.
