# OAW Core Coordinator Phase 06 Conformance and Controlled Dogfooding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the hard-cut architecture through contract tests, deterministic Workflow transcripts, a read-only third-party Provider dogfood, recovery trials, 80%+ changed-package coverage, native macOS checks, Docker-representable Linux checks, and explicit skips for unavailable Host-native or WSL behavior.

**Architecture:** Automated tests exercise Core and Coordinator with secret-free fake Host facts and normalized Receipts; OAW never invokes an Agent. A controlled `CURRENT` pilot uses the active session to consume a Dispatch Packet and review the existing non-production OpenCodeReview repository read-only. Native `SUBAGENT` evidence is optional and can only be supplied by the active Host; absence returns status 77 rather than triggering a process fallback.

**Tech Stack:** Go 1.26 tests and coverage, Bash 3.2-compatible harnesses, canonical JSON fixtures, macOS native tooling, Docker Desktop Linux smoke tests.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not invoke `codex exec`, Claude CLI, or another model process. Do not push, publish, merge, tag, or modify the OpenCodeReview repository.

**Depends on:** Phases 01-05 are complete and the canonical docs gate is GREEN.

**Produces:** Local release-readiness evidence only. P3 publication remains frozen.

## Verification Matrix

| ID | Scenario | Required result |
| --- | --- | --- |
| C1 | Policy-only installation and management | Policy installs; no Workflow State exists. |
| C2 | `DIRECT` and `BOUNDED` Core decisions | No Profile, Bundle, or Coordinator state. |
| C3 | Four built-in Profiles under `CURRENT` | Each compiles through the generic Core path when bindings verify. |
| C4 | Third-party Provider and user Profile | Same descriptor, discovery, Registry, and compiler path as built-ins. |
| C5 | `CURRENT` Workflow exchange | START, PREPARE, STARTED, COMPLETED, and INSPECT close deterministically. |
| C6 | Recovery and coordination | Replay, stale revision, lease conflict, uncertain execution, pause, cancel, switch, and restart behave deterministically. |
| C7 | Native `SUBAGENT` | Validate a real Host-provided transcript or return 77/SKIP; never emulate it. |
| C8 | Old contracts | Old schemas, commands, state, topology values, and model launch paths fail closed. |
| C9 | macOS and Linux | Native suite passes; Docker-representable Linux suite passes or Docker returns 77; WSL-only behavior returns 77 off WSL. |
| C10 | Coverage | Changed packages report at least 80.0% aggregate statement coverage. |

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/integration/core_coordinator_cutover_test.go` | Acceptance-matrix vertical slices across Core, Host facts, Coordinator, and hard rejection. |
| `internal/integration/external_host_transcript_test.go` | Optional strict validation of a Host-produced native transcript. |
| `internal/integration/testdata/core-coordinator/acme-provider.json` | Third-party Provider v3 fixture. |
| `internal/integration/testdata/core-coordinator/acme-profile.json` | User Profile Recipe v2 fixture. |
| `internal/integration/testdata/core-coordinator/user-config.toml` | User config v3 registration fixture. |
| `internal/integration/testdata/core-coordinator/current-session.json` | Secret-free CURRENT Host session fixture. |
| `internal/integration/testdata/core-coordinator/old-runtime-command.json` | Explicitly rejected old transport fixture. |
| `internal/hosttest/workflow.go` | Deterministic fake Host facts and Receipt builders; no invocation behavior. |
| `internal/hosttest/workflow_test.go` | Copy safety and invalid-fixture tests. |
| `scripts/check-core-coordinator-coverage.sh` | 80% changed-package coverage enforcement. |
| `scripts/dogfood-current.sh` | Prepare/inspect/finalize a read-only CURRENT pilot without launching an Agent. |
| `scripts/smoke-host-native.sh` | Validate externally supplied native Host transcript or return 77. |
| `scripts/smoke-linux.sh` | Linux archive checks for Core, Provider inspection, Policy-only state, and Workflow CLI. |
| `scripts/smoke-docker.sh` | Sandboxed Docker launcher for representable Linux behavior. |
| `tests/14-cutover-release-test.sh` | Native/archive/Docker/WSL hard-cut assertions. |
| `tests/16-core-coordinator-conformance-test.sh` | Black-box Core/Coordinator CLI, state, and no-model-process checks. |
| `tests/run.sh` | Repository-suite registration. |

## Locked Test Fixtures

The third-party fixture uses the generic path and these stable identities:

```text
Provider: acme/engineering
Capability: delivery
Recipe: acme/current-delivery
Profile selection: acme/current-delivery
Host: codex-test
Binding: skill / acme:delivery
Topology: CURRENT
```

The Provider v3 fixture has one `WORKFLOW` Capability with responsibilities
`planning`, `implementation`, `review`, and `verification`, maximum effects
`read-project`, `write-project`, and `run-process`, resource
`project-worktree`, supported topology `CURRENT`, and a matching `codex-test`
binding. The Recipe v2 fixture has four required nodes in that order, one
owner per responsibility, terminal gate `verification`, stable boundaries
`plan-approved`, `implementation-complete`, and `verification-complete`, and
an empty `environment_requirements` list.

Use placeholder-free deterministic digests in tests by computing them through
repository constructors. Do not author a digest and then repair it during
decode.

## Task 1: Add deterministic Host facts and Receipt builders

**Files:**
- Create: `internal/hosttest/workflow.go`
- Create: `internal/hosttest/workflow_test.go`

- [ ] **Step 1: Write failing copy-safety and identity tests**

Add `TestCurrentSessionFixtureIsSecretFree`,
`TestCompletedReceiptPinsDispatchIdentity`, and
`TestHostFixturesReturnDefensiveCopies`. Explicitly search canonical fixture
bytes for `token`, `password`, `authorization`, `api_key`, raw MCP config, and
model commands.

- [ ] **Step 2: Run tests to verify RED**

```bash
rtk go test ./internal/hosttest -run 'CurrentSession|Receipt|Defensive'
```

Expected: FAIL because the builders do not exist.

- [ ] **Step 3: Implement data-only builders**

Expose these functions exactly:

```go
func CurrentSession(t testing.TB, hostID string, inventoryDigest string) host.SessionSnapshot
func CurrentEnvironment(t testing.TB, session host.SessionSnapshot) host.EnvironmentReport
type ReceiptIdentity struct {
    WorkflowID       string
    BundleGeneration uint64
    BundleDigest     string
    NodeID           string
    Topology         execution.Topology
    HostSessionDigest string
}

func StartedReceipt(t testing.TB, identity ReceiptIdentity, handle string) host.InvocationReceipt
func CompletedReceipt(t testing.TB, identity ReceiptIdentity, handle string, evidence []host.EvidenceReference) host.InvocationReceipt
func FailedReceipt(t testing.TB, identity ReceiptIdentity, handle, code string) host.InvocationReceipt
```

Integration tests copy every identity from the Dispatch Packet into
`ReceiptIdentity`. Builders call production constructors, pin those values,
use `context_freshness = "shared"`, and return new records. They contain no
function that starts a process, Agent, tool, filesystem mutation, or network
request.

- [ ] **Step 4: Run GREEN and race checks**

```bash
rtk gofmt -w internal/hosttest
rtk go test ./internal/hosttest
rtk go test -race ./internal/hosttest
```

- [ ] **Step 5: Commit fake-Host test support**

```bash
rtk git add internal/hosttest/workflow.go internal/hosttest/workflow_test.go
rtk git commit -m "test: add data-only host workflow fixtures"
```

## Task 2: Prove the Core and Coordinator acceptance matrix

**Files:**
- Create: `internal/integration/core_coordinator_cutover_test.go`
- Create: `internal/integration/testdata/core-coordinator/acme-provider.json`
- Create: `internal/integration/testdata/core-coordinator/acme-profile.json`
- Create: `internal/integration/testdata/core-coordinator/user-config.toml`
- Create: `internal/integration/testdata/core-coordinator/current-session.json`
- Create: `internal/integration/testdata/core-coordinator/old-runtime-command.json`

- [ ] **Step 1: Write failing table-driven acceptance tests**

Add these exact tests:

```go
func TestDirectAndBoundedNeverCreateWorkflowState(t *testing.T)
func TestAllBuiltInProfilesCompileForCurrentWhenCapabilitiesVerify(t *testing.T)
func TestThirdPartyProviderAndUserProfileUseGenericCompiler(t *testing.T)
func TestCurrentWorkflowClosesThroughNormalizedReceipts(t *testing.T)
func TestOldRuntimeContractsFailClosed(t *testing.T)
```

For built-ins, iterate `SP-FULL`, `MATT-FULL`, `ECC-FULL`, and
`MATT-SP-HYBRID`; assert the selected Bundle contains `CURRENT`, one owner for
every required responsibility, the exact Host session/inventory digests, and
no process command or private environment field.

- [ ] **Step 2: Run the integration tests to verify RED**

```bash
rtk go test ./internal/integration -run 'DirectAndBounded|BuiltInProfiles|ThirdParty|CurrentWorkflow|OldRuntime'
```

- [ ] **Step 3: Add the third-party fixtures through strict constructors**

Register these paths in `user-config.toml`:

```toml
schema_version = "oaw.user-config/v3"

[[provider_descriptors]]
id = "acme/engineering"
path = "acme-provider.json"

[[profile_recipes]]
id = "acme/current-delivery"
path = "acme-profile.json"
```

The test computes project trust fingerprints through `config` APIs and creates
one matching binding observation. Do not special-case `acme/*` in production
code.

- [ ] **Step 4: Implement complete CURRENT exchange assertions**

Use a temporary Workflow State root and drive:

```text
START@0 -> READY@1
PREPARE@1 -> PREPARED@2 + Dispatch Packet
RECEIPT(STARTED)@2 -> IN_FLIGHT@3
RECEIPT(COMPLETED)@3 -> next node or FINISHED@4
INSPECT@4 -> same revision and digest, no write
```

Continue across all nodes until the terminal gate closes. Replay each mutating
command once with the same idempotency key and assert identical result bytes;
then reuse the key with different content and assert rejection without a new
revision.

- [ ] **Step 5: Run GREEN, shuffle, and race checks**

```bash
rtk gofmt -w internal/integration
rtk go test ./internal/integration -run 'DirectAndBounded|BuiltInProfiles|ThirdParty|CurrentWorkflow|OldRuntime' -count=20
rtk go test -race ./internal/integration -run 'CurrentWorkflow|ThirdParty'
```

Expected: PASS with identical Bundle and revision digests across repetitions.

- [ ] **Step 6: Commit the vertical-slice conformance tests**

```bash
rtk git add internal/integration/core_coordinator_cutover_test.go internal/integration/testdata/core-coordinator
rtk git commit -m "test: cover core coordinator vertical slices"
```

## Task 3: Stress recovery, leases, switching, and uncertainty

**Files:**
- Modify: `internal/integration/core_coordinator_cutover_test.go`
- Modify: `internal/coordinator/recovery_test.go`
- Modify: `internal/coordinator/leases_test.go`
- Modify: `internal/coordinator/switch_test.go`

- [ ] **Step 1: Add failing recovery scenarios**

Add tests for two Engines opening the same state root, concurrent stale
revision writes, conflicting physical-root leases, restart after PREPARE,
restart after STARTED, uncertain execution reconciliation, explicit pause and
cancel, and a Profile/topology switch at a declared stable boundary. Assert a
switch increments Bundle generation and revokes the old Grant.

- [ ] **Step 2: Run tests to verify RED**

```bash
rtk go test ./internal/coordinator ./internal/integration -run 'Recovery|Concurrent|Lease|Uncertain|Pause|Cancel|Switch'
```

- [ ] **Step 3: Fix only production defects exposed by the trials**

Changes are allowed only in the owning Core/Coordinator/Host package and must
preserve the locked schemas. Do not weaken a test, add sleeps, introduce a
legacy reader, or automatically retry an uncertain mutation.

- [ ] **Step 4: Run deterministic and race GREEN checks**

```bash
rtk go test ./internal/coordinator ./internal/integration -run 'Recovery|Concurrent|Lease|Uncertain|Pause|Cancel|Switch' -count=20
rtk go test -race ./internal/coordinator ./internal/integration
```

- [ ] **Step 5: Commit recovery conformance**

```bash
rtk git add internal/core internal/coordinator internal/host internal/integration
rtk git commit -m "test: verify workflow coordination recovery"
```

## Task 4: Enforce 80% changed-package coverage

**Files:**
- Create: `scripts/check-core-coordinator-coverage.sh`
- Modify: `internal/admission/admission_test.go`
- Modify: `internal/core/core_test.go`
- Modify: `internal/coordinator/records_test.go`
- Modify: `internal/coordinator/journal_test.go`
- Modify: `internal/coordinator/dispatch_test.go`
- Modify: `internal/execution/execution_test.go`
- Modify: `internal/host/receipt_test.go`
- Modify: `internal/host/validation_test.go`
- Modify: `internal/profile/profile_test.go`
- Modify: `internal/registry/registry_test.go`

- [ ] **Step 1: Write a failing coverage gate**

The script creates its profile under `${TMPDIR:-/tmp}`, removes it on exit, and
runs these packages as one coverage domain:

```text
./internal/admission
./internal/core
./internal/coordinator
./internal/execution
./internal/host
./internal/profile
./internal/registry
```

Use `-covermode=atomic` and `-coverpkg` with the same comma-separated package
list. Parse only the `total:` row from `go tool cover -func`; fail below
`80.0`, print the full function report on failure, and print the measured total
on success.

- [ ] **Step 2: Run the gate to establish the baseline**

```bash
rtk bash scripts/check-core-coordinator-coverage.sh
```

Expected: FAIL if aggregate coverage is below 80.0%.

- [ ] **Step 3: Add focused tests for uncovered behavior**

Add these table-driven tests, using the function report to confirm each reaches
its owning branch:

```text
TestGrantRejectsTopologyAndSessionMismatch
TestCoreCompilationResultDeepCopiesNestedRecords
TestCoreResolveRejectsCrossHostFacts
TestWorkflowRecordsRejectCorruptContentDigests
TestJournalRecoveryRejectsBrokenPredecessor
TestReceiptTransitionRejectsIncompleteEvidence
TestExecutionRequirementsRejectDuplicateSurfaces
TestReceiptRejectsSecretBearingOrUnknownFields
TestHostManifestRejectsUnsupportedFeatureTopologyPairs
TestProfileIntersectionRejectsEmptyTopology
TestRegistryNeverWidensObservedBindingTopologies
```

Do not exclude files, count generated schemas as coverage, or lower the
threshold.

- [ ] **Step 4: Run GREEN and repository tests**

```bash
rtk bash scripts/check-core-coordinator-coverage.sh
rtk go test ./...
rtk go test -race ./internal/core ./internal/coordinator ./internal/execution ./internal/host ./internal/profile ./internal/registry
```

- [ ] **Step 5: Commit the coverage gate and tests**

```bash
rtk git add scripts/check-core-coordinator-coverage.sh internal/admission/admission_test.go internal/core/core_test.go internal/coordinator/records_test.go internal/coordinator/journal_test.go internal/coordinator/dispatch_test.go internal/execution/execution_test.go internal/host/receipt_test.go internal/host/validation_test.go internal/profile/profile_test.go internal/registry/registry_test.go
rtk git commit -m "test: enforce core coordinator coverage"
```

## Task 5: Add the no-process black-box conformance suite

**Files:**
- Create: `tests/16-core-coordinator-conformance-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write the black-box test with PATH traps**

Build `oaw` into a temporary release root. Put executables named `codex`,
`claude`, `gemini`, and `opencode` first on PATH; each writes one sentinel and
exits 99. Test:

```text
oaw --help
oaw catalog validate
oaw providers inspect --host codex --format json
oaw workflow exchange
rejected: oaw runtime exchange
rejected: oaw run --host codex
```

Use canonical START/PREPARE/Receipt JSON generated by a small Go test helper,
not shell string substitution. Assert only Workflow commands create files
under `.../open-agent-workflow/workflows`, management and Provider inspection
do not, and no PATH sentinel exists.

- [ ] **Step 2: Run the suite to verify failures are meaningful**

```bash
rtk bash tests/16-core-coordinator-conformance-test.sh
```

Expected before completion: FAIL on a missing fixture or incorrect command
contract, never because a model trap executed.

- [ ] **Step 3: Register the suite**

Append this exact entry after test 15 in `tests/run.sh`:

```bash
16-core-coordinator-conformance-test.sh
```

- [ ] **Step 4: Run GREEN and commit**

```bash
rtk bash tests/16-core-coordinator-conformance-test.sh
rtk bash tests/run.sh
rtk git add tests/16-core-coordinator-conformance-test.sh tests/run.sh
rtk git commit -m "test: exercise core coordinator black box"
```

## Task 6: Run the controlled CURRENT dogfood against OpenCodeReview

**Files:**
- Create: `scripts/dogfood-current.sh`
- Local evidence only: `.scratch/oaw-core-coordinator/evidence/current-dogfood/`
- Read only: `/Users/wifibaby4u/LLM/open-code-review`

- [ ] **Step 1: Implement a non-executing pilot harness**

The script accepts exactly:

```text
scripts/dogfood-current.sh start <absolute-repository> <absolute-evidence-dir>
scripts/dogfood-current.sh prepare <absolute-evidence-dir>
scripts/dogfood-current.sh inspect <absolute-evidence-dir>
scripts/dogfood-current.sh receipt <absolute-evidence-dir> <absolute-receipt-json>
```

`start` requires `OAW_DOGFOOD_APPROVED_REPOSITORY` to equal the canonical
repository path, refuses a `.oaw-production` marker, requires a clean Git
repository, records commit and status digests, verifies
`skills/open-code-review/SKILL.md`, creates isolated
temporary OAW config/state roots, registers that exact Skill as a trusted
third-party read-only `review` Capability, and registers a user Recipe named
`local/ocr-readonly-workflow`. The Recipe contains the three ordered
responsibilities `review-scope`, `code-review`, and `verification`, all owned
by that Capability, and selects `CURRENT`. `start` also requires an opaque
`OAW_HOST_SESSION_ID` supplied by the active Agent Host and creates a local,
user-trusted development integration that reports `CURRENT` only. This record
is pilot input, not a built-in integration and not evidence for `SUBAGENT`.
`start` commits the Bundle but does not invoke the Skill. `prepare` issues the current node's Dispatch Packet,
`receipt` validates and submits one Host-authored normalized Receipt, and
`inspect` prints committed state and verifies all evidence digests. None of the
subcommands modifies the target repository or starts an Agent.

- [ ] **Step 2: Unit-test refusal boundaries**

Add shell cases for relative paths, missing/mismatched approved-repository
authority, `.oaw-production`, dirty repository,
symlinked evidence root, missing Skill, malformed Receipt, wrong session or
Bundle digest, and a repository fingerprint change between prepare and
receipt. Every refusal leaves both repositories unchanged.

- [ ] **Step 3: Prepare the approved read-only pilot**

```bash
rtk env OAW_HOST_SESSION_ID=current-pilot-20260805 OAW_DOGFOOD_APPROVED_REPOSITORY=/Users/wifibaby4u/LLM/open-code-review bash scripts/dogfood-current.sh start /Users/wifibaby4u/LLM/open-code-review /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood
rtk bash scripts/dogfood-current.sh prepare /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood
```

Expected: a `CURRENT` Dispatch Packet for a read-only review and no diff in
OpenCodeReview.

- [ ] **Step 4: Execute the Packet in the active current session**

For each Packet, the active Agent reads the pinned OpenCodeReview commit and
produces only the named scope, review, or verification evidence file. OAW does
not launch or configure that Agent. Record findings, verification command
output, repository-before/after status, and SHA-256 digests; do not edit,
commit, push, or publish in OpenCodeReview.

- [ ] **Step 5: Submit the normalized Receipt and inspect terminal state**

```bash
rtk bash scripts/dogfood-current.sh receipt /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood/receipt-scope.json
rtk bash scripts/dogfood-current.sh prepare /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood
rtk bash scripts/dogfood-current.sh receipt /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood/receipt-review.json
rtk bash scripts/dogfood-current.sh prepare /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood
rtk bash scripts/dogfood-current.sh receipt /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood/receipt-verification.json
rtk bash scripts/dogfood-current.sh inspect /Users/wifibaby4u/LLM/open-agent-workflow/.scratch/oaw-core-coordinator/evidence/current-dogfood
rtk git -C /Users/wifibaby4u/LLM/open-code-review status --short --branch
```

Expected: evidence closes the node, state is terminal, and OpenCodeReview still
matches its original clean commit.

- [ ] **Step 6: Commit only the reusable harness**

```bash
rtk git add scripts/dogfood-current.sh
rtk git commit -m "test: add current-session dogfood harness"
```

Do not commit `.scratch` evidence; report its digest and path in the final local
verification summary.

## Task 7: Make native SUBAGENT verification optional and truthful

**Files:**
- Create: `internal/integration/external_host_transcript_test.go`
- Create: `scripts/smoke-host-native.sh`
- Modify: `tests/14-cutover-release-test.sh`

- [ ] **Step 1: Add strict external transcript validation**

`TestExternalHostNativeTranscript` reads the path from
`OAW_HOST_NATIVE_TRANSCRIPT`, strictly decodes one Host v2 conformance
transcript, requires `SUBAGENT`, separate parent/child session IDs, accepted
environment dispositions, exact binding visibility, normalized Receipts, and
no secret-bearing fields. If the variable is empty, the Go test calls
`t.Skip`; malformed supplied evidence fails.

- [ ] **Step 2: Implement the status-77 wrapper**

`scripts/smoke-host-native.sh` takes zero arguments. If
`OAW_HOST_NATIVE_TRANSCRIPT` is unset or unreadable, print:

```text
SKIP: Host-native SUBAGENT transcript unavailable
```

to stderr and exit 77. Otherwise run only the external transcript test. The
script contains no model command and no fallback.

- [ ] **Step 3: Run under the selected CURRENT topology**

```bash
rtk bash scripts/smoke-host-native.sh
```

Expected for this implementation session: exit 77/SKIP unless the active Host
has independently supplied a transcript. A skip is acceptable evidence; it is
never rewritten as PASS.

- [ ] **Step 4: Add skip-contract tests and commit**

```bash
rtk go test ./internal/integration -run ExternalHostNativeTranscript
rtk bash tests/14-cutover-release-test.sh
rtk git add internal/integration/external_host_transcript_test.go scripts/smoke-host-native.sh tests/14-cutover-release-test.sh
rtk git commit -m "test: validate optional host-native transcripts"
```

## Task 8: Verify macOS natively and Linux only through Docker

**Files:**
- Modify: `scripts/smoke-linux.sh`
- Modify: `scripts/smoke-docker.sh`
- Modify: `tests/14-cutover-release-test.sh`

- [ ] **Step 1: Replace old Runtime smoke assertions**

Linux smoke must prove Provider inspection and management create no Workflow
State, old commands fail, `workflow exchange` works with a CURRENT fixture,
the archive contains no Runner asset, and the PATH model traps remain silent.
Keep archive path traversal, read-only mount, no-network, dropped capability,
checksum, and Policy-only tracker checks.

- [ ] **Step 2: Run the native macOS gate**

```bash
rtk go test ./...
rtk go test -race ./internal/core ./internal/coordinator ./internal/execution ./internal/host ./internal/profile ./internal/registry
rtk bash scripts/check-core-coordinator-coverage.sh
rtk bash tests/run.sh
rtk bash scripts/check-docs.sh
rtk git diff --check
```

Expected: PASS.

- [ ] **Step 3: Build local release archives without publication**

```bash
rtk bash scripts/build-release.sh /tmp/oaw-core-coordinator-dist
```

Expected: six local archives and `SHA256SUMS`; no tag, release, upload, or push.

- [ ] **Step 4: Run the Docker-representable Linux gate**

Select the Docker server architecture and run the matching Linux archive:

```bash
rtk bash scripts/smoke-docker.sh /tmp/oaw-core-coordinator-dist/open-agent-workflow_0.1.0_linux_arm64.tar.gz
```

Use `_amd64` instead when Docker reports `amd64`. Expected: PASS when Docker is
available. Missing CLI/daemon/image support returns 77/SKIP and does not block
macOS results; any assertion failure returns nonzero other than 77 and blocks.

- [ ] **Step 5: Record WSL as unavailable on macOS**

```bash
rtk bash scripts/smoke-wsl.sh /tmp/oaw-core-coordinator-dist/open-agent-workflow_0.1.0_linux_arm64.tar.gz
```

Expected on macOS: exit 77 with `SKIP: WSL smoke requires an actual Microsoft
WSL kernel`. Do not emulate WSL outside Docker.

- [ ] **Step 6: Commit cross-platform smoke updates**

```bash
rtk git add scripts/smoke-linux.sh scripts/smoke-docker.sh tests/14-cutover-release-test.sh
rtk git commit -m "test: verify hard cutover on macos and docker linux"
```

## Task 9: Produce the final local verification packet

**Files:**
- Local evidence only: `.scratch/oaw-core-coordinator/evidence/verification.md`
- Local evidence only: `.scratch/oaw-core-coordinator/evidence/test-results.json`

- [ ] **Step 1: Capture fresh commands and outcomes**

Record command, start/end time, exit status, commit, stdout/stderr digest, and
PASS/FAIL/SKIP for every matrix row C1-C10. A skipped row contains status 77 and
the exact reason. Do not include credentials, raw Host config, or complete
private environment reports.

- [ ] **Step 2: Run the permanent absence scans**

```bash
rtk rg -n 'codex exec|oaw/codex-runner|runner-managed|native-managed|main-agent-allowed|isolated-required|NATIVE_SUBAGENT|INLINE' cmd internal policy README.md README-zh.md docs/en docs/zh SECURITY.md SECURITY-zh.md --glob '*.go' --glob '!**/*_test.go' --glob '*.md'
rtk rg -n 'internal/runtime|oaw\.runtime/|oaw runtime exchange|oaw run --host' cmd internal policy README.md README-zh.md docs/en docs/zh SECURITY.md SECURITY-zh.md --glob '*.go' --glob '!**/*_test.go' --glob '*.md'
```

Expected: both scans exit 1 with no match. `CHANGELOG.md` and historical docs
are intentionally excluded because they record removals.

- [ ] **Step 3: Verify repository and dogfood target integrity**

```bash
rtk git status --short --branch
rtk git diff --check
rtk git -C /Users/wifibaby4u/LLM/open-code-review status --short --branch
```

Expected: only intended OAW implementation/evidence state exists; the
OpenCodeReview repository remains clean and unchanged.

- [ ] **Step 4: Record the local promotion decision**

Set one result in `verification.md`:

```text
READY_FOR_RELEASE_ENGINEERING
BLOCKED_BY_PRODUCT_DEFECT
BLOCKED_BY_MISSING_REQUIRED_ENVIRONMENT
```

`SKIP` for optional native `SUBAGENT`, Docker unavailability, or WSL on macOS
does not block when the reason and status 77 are recorded. A failed available
test, coverage below 80%, stale execution code, or OpenCodeReview mutation does
block.

- [ ] **Step 5: Keep evidence local and stop before publication**

Do not stage `.scratch/oaw-core-coordinator/evidence`, change `VERSION`, create
a tag, push, publish, or open a release. Release engineering is a separately
authorized next phase.

## Phase 06 Completion Gate

- [ ] C1-C10 each has fresh PASS or permitted status-77 SKIP evidence.
- [ ] `DIRECT` and `BOUNDED` create no Workflow State.
- [ ] All four built-in Profiles and one user Profile compile through Core under `CURRENT`.
- [ ] A third-party Provider resolves through the same Host-scoped generic path.
- [ ] Coordinator replay, leases, recovery, uncertainty, cancellation, and switching are deterministic.
- [ ] The optional Host-native check never launches or emulates a Subagent.
- [ ] Changed-package aggregate statement coverage is at least 80.0%.
- [ ] macOS native gates pass; Docker Linux passes when available or records 77; WSL records 77 on macOS.
- [ ] OAW contains no model-launch path, old schema/state reader, command alias, or removed topology value.
- [ ] OpenCodeReview remains at its original clean commit.
- [ ] No external publication action has occurred.
