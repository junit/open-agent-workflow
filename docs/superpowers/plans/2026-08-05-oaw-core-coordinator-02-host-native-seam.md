# OAW Core Coordinator Phase 02 Host-Native Seam Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace process-oriented Host authority with secret-free Host session, environment, binding inventory, and normalized receipt contracts that support `CURRENT` and optional native `SUBAGENT` execution.

**Architecture:** The active Host reports facts; OAW validates and pins them. Static integration records describe `policy` or `host-native` control surfaces, while per-session snapshots decide actual topology eligibility. Conformance validates records and deterministic transcripts, never invokes a model or creates a child Agent.

**Tech Stack:** Go 1.26, JSON Schema Draft 2020-12, canonical JSON, immutable records, deterministic fake transcripts, table-driven and fuzz tests.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. This phase is not releasable until Phase 04 removes the old Runner code.

**Hard-cut integration boundary:** This phase replaces Host records in place and does not keep old Runner consumers compiling. Run only the Host/Core leaf-package checks named below. Phase 03 replaces Runtime and Admission consumers; Phase 04 deletes the remaining Runner consumers and is the first required full-repository GREEN gate.

**Depends on:** Phase 01 Core contracts.

**Produces:** Host facts consumed by Core compilation and Workflow Coordinator dispatch.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/host/records.go` | v2 integration and conformance records. |
| `internal/host/session.go` | Host session snapshot construction and validation. |
| `internal/host/environment.go` | Environment report validation and requirement projection. |
| `internal/host/bindings.go` | Topology-aware Provider binding inventory. |
| `internal/host/receipt.go` | Normalized Host invocation receipts. |
| `internal/host/conformance.go` | Static/session transcript conformance validation only. |
| `internal/assets/schemas/v2/host-manifest.schema.json` | Static Host v2 manifest. |
| `internal/assets/schemas/v2/host-integration.schema.json` | Trusted integration record. |
| `internal/assets/schemas/v2/host-integration-set.schema.json` | Integration collection. |
| `internal/assets/schemas/v2/host-session.schema.json` | Secret-free session facts. |
| `internal/assets/schemas/v2/host-environment-report.schema.json` | Topology-specific environment observations. |
| `internal/assets/schemas/v2/host-binding-inventory.schema.json` | Observed topology-aware bindings. |
| `internal/assets/schemas/v2/host-invocation-receipt.schema.json` | Normalized invocation outcome. |
| `internal/assets/schemas/v2/host-conformance-transcript.schema.json` | Deterministic Host transcript. |
| `internal/assets/schemas/v2/host-conformance-report.schema.json` | Conformance result. |
| `internal/assets/host-integrations.json` | Nine built-in `policy` integrations; no Runner. |
| `internal/integration/host_configuration_test.go` | Built-in and user Host integration trust. |
| `internal/integration/host_conformance_test.go` | Session and transcript behavior. |

## Locked Host Records

```go
type ControlSurface string

const (
    SurfacePolicy     ControlSurface = "policy"
    SurfaceHostNative ControlSurface = "host-native"
)

type Manifest struct {
    SchemaVersion      string
    ManifestVersion    string
    HostID             string
    ControlSurface     ControlSurface
    Protocols          []string
    BindingKinds       []string
    SupportedTopologies []execution.Topology
    Features           []Feature
}

type SessionSnapshot struct {
    SchemaVersion            string
    HostID                   string
    IntegrationID            string
    IntegrationVersion       string
    SessionID                string
    SupportedTopologies      []execution.Topology
    ProviderInventoryDigest  string
    EnvironmentReportDigest  string
    SandboxPolicyDigest      string
    ApprovalPolicyDigest     string
    Digest                   string
}

type EnvironmentReport struct {
    SchemaVersion  string
    SessionID      string
    ParentSessionID string
    Topology       execution.Topology
    Observations   []execution.EnvironmentObservation
    Digest         string
}
```

For `CURRENT`, `SessionID` equals the active session and `ParentSessionID` is
empty. For `SUBAGENT`, `SessionID` is the child and `ParentSessionID` is the
active parent. This is an observation report, not a dump of Host configuration.

Use this evidence reference everywhere Receipts and Dispatch requirements need
an artifact pointer:

```go
type EvidenceReference struct {
    Kind      string `json:"kind"`
    Reference string `json:"reference"`
    Digest    string `json:"digest"`
}
```

`CURRENT` is required for every `host-native` Manifest and session.
`SUBAGENT` is optional. A `policy` integration has no protocol or conformance
claim and cannot produce a Runtime/Coordinator guarantee.

Use these closed features only:

```text
provider-binding-inventory
normalized-receipts
invocation-deduplication
pause
cancellation
environment-reporting
```

Delete the semantic meaning of `isolated-executor`, `native-invocation`, and
`exact-binding-invocation`; exact bindings are validated directly from the
inventory and Dispatch identity.

## Task 1: Add Host session and environment records

**Files:**
- Modify: `internal/host/records.go`
- Create: `internal/host/session.go`
- Create: `internal/host/environment.go`
- Modify: `internal/host/records_test.go`
- Modify: `internal/host/validation_test.go`
- Create: `internal/assets/schemas/v2/host-session.schema.json`
- Create: `internal/assets/schemas/v2/host-environment-report.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`

- [ ] **Step 1: Write failing immutable session tests**

Add `TestNewSessionSnapshotPinsCurrentSession`,
`TestSessionSnapshotRejectsSubagentWithoutManifestSupport`, and
`TestEnvironmentReportUsesClosedDispositions`. Mutate returned topology and
observation slices and prove subsequent clones are unchanged.

- [ ] **Step 2: Run the focused tests to verify RED**

```bash
rtk go test ./internal/host -run 'Session|EnvironmentReport|Environment'
```

- [ ] **Step 3: Implement constructors and stable diagnostics**

```go
func NewSessionSnapshot(manifest Manifest, input SessionSnapshot) (SessionSnapshot, error)
func NewEnvironmentReport(input EnvironmentReport) (EnvironmentReport, error)
func ValidateEnvironmentReport(session SessionSnapshot, report EnvironmentReport) error
func ValidateRequirements(requirements []execution.EnvironmentRequirement, report EnvironmentReport) error
```

Use `HOST_SESSION_INVALID`, `HOST_SESSION_CHANGED`,
`HOST_ENVIRONMENT_REPORT_INVALID`, and `HOST_ENVIRONMENT_REQUIREMENT_UNMET`.
Require valid canonical digests and never store credentials or raw extension
configuration.

- [ ] **Step 4: Run GREEN and copy-safety checks**

```bash
rtk gofmt -w internal/host
rtk go test ./internal/host -run 'Session|EnvironmentReport|Environment'
rtk go test -race ./internal/host -run 'Session|EnvironmentReport|Environment'
```

- [ ] **Step 5: Commit Host session records**

```bash
rtk git add internal/host
rtk git commit -m "feat: validate host session capabilities"
```

## Task 2: Make binding inventory topology-aware

**Files:**
- Modify: `internal/host/bindings.go`
- Modify: `internal/host/bindings_test.go`
- Modify: `internal/registry/resolve.go`
- Modify: `internal/registry/registry_test.go`
- Modify: `internal/core/resolve.go`
- Modify: `internal/core/core_test.go`
- Modify: `internal/cli/provider_inputs.go`
- Modify: `internal/cli/provider_inputs_test.go`
- Modify: `internal/cli/providers.go`
- Modify: `internal/cli/providers_test.go`
- Create: `internal/assets/schemas/v2/host-binding-inventory.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`

- [ ] **Step 1: Write failing binding topology tests**

Prove one physical Provider can expose a binding for `CURRENT`, `SUBAGENT`, or
both, and that the Registry pins only the observed subset. A parent-visible
binding without accepted child evidence must not verify for `SUBAGENT`. A
Codex observation must never satisfy a Claude binding or appear in Claude's
current-Host section; the reverse is equally invalid.

- [ ] **Step 2: Run focused tests to verify RED**

```bash
rtk go test ./internal/host ./internal/registry ./internal/cli -run 'Binding.*Topology|Provider.*Topology'
```

- [ ] **Step 3: Extend observations without inferring child visibility**

```go
type BindingObservation struct {
    HostID            string
    InstallationKey   string
    Binding           catalog.HostBinding
    Topologies        []execution.Topology
    Source            string
    EvidenceReference string
    Digest            string
}
```

Require every observed topology to appear in the descriptor binding. Update
`providers inspect` JSON/text output to display the observed topology set but
keep detection diagnostic and selection-neutral. Route the normalized
Configuration, discovery report, and inventory through `core.Resolve`; CLI and
Host integrations must not call `registry.Resolve` directly.

- [ ] **Step 4: Run Registry and CLI GREEN checks**

```bash
rtk gofmt -w internal/host internal/registry internal/core internal/cli
rtk go test ./internal/host ./internal/registry ./internal/core ./internal/cli
```

- [ ] **Step 5: Commit topology-aware inventory**

```bash
rtk git add internal/host internal/registry internal/core internal/cli
rtk git commit -m "feat: verify host binding topologies"
```

## Task 3: Replace Host manifests and built-in integrations

**Files:**
- Modify: `internal/host/records.go`
- Modify: `internal/host/decode.go`
- Modify: `internal/host/validate.go`
- Modify: `internal/host/builtin.go`
- Modify: `internal/host/builtin_test.go`
- Modify: `internal/host/validation_test.go`
- Delete: `internal/assets/schemas/v1/host-manifest.schema.json`
- Delete: `internal/assets/schemas/v1/host-integration.schema.json`
- Delete: `internal/assets/schemas/v1/host-integration-set.schema.json`
- Create: `internal/assets/schemas/v2/host-manifest.schema.json`
- Create: `internal/assets/schemas/v2/host-integration.schema.json`
- Create: `internal/assets/schemas/v2/host-integration-set.schema.json`
- Modify: `internal/assets/host-integrations.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Modify: `internal/config/snapshot_test.go`
- Modify: `internal/integration/host_configuration_test.go`

- [ ] **Step 1: Write failing v2-only integration tests**

Assert all nine built-ins use IDs ending in `-policy`, `ControlSurface ==
policy`, support only `CURRENT`, and claim no workflow protocol or conformance.
Assert `oaw/codex-runner` is absent. Assert every v1 Host schema and old control
surface returns `HOST_SCHEMA_UNSUPPORTED`.

- [ ] **Step 2: Run Host asset tests to verify RED**

```bash
rtk go test ./internal/host ./internal/schema ./internal/config ./internal/integration -run 'Builtin|Manifest|Integration|HostSchema'
```

- [ ] **Step 3: Replace the active Host schema family**

Use schema IDs `oaw.host-manifest/v2`, `oaw.host-integration/v2`, and
`oaw.host-integration-set/v2`. A `host-native` record requires
`oaw.workflow/v1`, `CURRENT`, Provider inventory, and normalized receipts. It
may add `SUBAGENT` and environment reporting. A `policy` record has empty
protocols/features and `CURRENT` only.

- [ ] **Step 4: Regenerate canonical built-in digests**

Use the existing constructors in a test-only generator or focused Go test,
then author the resulting canonical digests into `host-integrations.json`.
Never repair a digest while decoding.

- [ ] **Step 5: Run GREEN and full schema checks**

```bash
rtk gofmt -w internal/host internal/schema internal/config internal/integration
rtk go test ./internal/host ./internal/schema ./internal/config ./internal/integration
```

- [ ] **Step 6: Commit Host v2**

```bash
rtk git add internal/host internal/schema internal/config internal/integration internal/assets
rtk git commit -m "feat: replace host integration contracts"
```

## Task 4: Replace invocation conformance with receipt conformance

**Files:**
- Modify: `internal/host/conformance.go`
- Modify: `internal/host/conformance_test.go`
- Modify: `internal/host/conformance_fuzz_test.go`
- Create: `internal/host/receipt.go`
- Create: `internal/host/receipt_test.go`
- Create: `internal/assets/schemas/v2/host-invocation-receipt.schema.json`
- Create: `internal/assets/schemas/v2/host-conformance-transcript.schema.json`
- Create: `internal/assets/schemas/v2/host-conformance-report.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Modify: `internal/integration/host_conformance_test.go`
- Modify: `internal/hosttest/fixture.go`

- [ ] **Step 1: Write failing receipt and transcript tests**

Use this closed Receipt interface:

```go
type ReceiptKind string

const (
    ReceiptStarted   ReceiptKind = "STARTED"
    ReceiptPaused    ReceiptKind = "PAUSED"
    ReceiptCompleted ReceiptKind = "COMPLETED"
    ReceiptFailed    ReceiptKind = "FAILED"
    ReceiptCancelled ReceiptKind = "CANCELLED"
)

type InvocationReceipt struct {
    SchemaVersion          string
    Kind                   ReceiptKind
    WorkflowID             string
    BundleGeneration       uint64
    BundleDigest           string
    NodeID                 string
    Topology               execution.Topology
    HostSessionDigest      string
    InvocationHandle       string
    ContextFreshness       string
    EnvironmentReportDigest string
    Outcome                string
    FailureCode            string
    Evidence               []EvidenceReference
    Digest                 string
}

type InvocationRecord struct {
    IdempotencyKey string `json:"idempotency_key"`
    DispatchDigest string `json:"dispatch_digest"`
    ReceiptDigest string `json:"receipt_digest"`
}

type ConformanceTranscript struct {
    SchemaVersion      string               `json:"schema_version"`
    Session            SessionSnapshot      `json:"session"`
    Inventory          BindingInventory     `json:"inventory"`
    EnvironmentReports []EnvironmentReport  `json:"environment_reports"`
    Receipts           []InvocationReceipt  `json:"receipts"`
    Invocations        []InvocationRecord   `json:"invocations"`
    Digest             string               `json:"digest"`
}

type ConformanceReport struct {
    SchemaVersion     string   `json:"schema_version"`
    ManifestDigest    string   `json:"manifest_digest"`
    TranscriptDigest  string   `json:"transcript_digest"`
    VerifiedFeatures  []Feature `json:"verified_features"`
    Diagnostics       []string `json:"diagnostics"`
    Digest            string   `json:"digest"`
}
```

Assert `CURRENT` requires `context_freshness = "shared"`; `SUBAGENT` requires
a child invocation handle and accepted environment report. Raw output and
credentials are not fields.

- [ ] **Step 2: Run focused tests to verify RED**

```bash
rtk go test ./internal/host ./internal/integration -run 'Receipt|Conformance|Transcript'
```

- [ ] **Step 3: Replace callable Adapter conformance**

Delete `ConformanceAdapter`, `CreateExecutor`, and `Invoke` from conformance.
Implement:

```go
func ValidateConformanceTranscript(manifest Manifest, transcript ConformanceTranscript) (ConformanceReport, error)
```

The transcript contains only deterministic session, inventory, receipt,
deduplication, pause, and cancellation records. Validation never calls an
Adapter and cannot start a process or Agent.

- [ ] **Step 4: Run fuzz, race, and Phase 02 convergence checks**

```bash
rtk gofmt -w internal/host internal/hosttest internal/integration
rtk go test ./internal/host ./internal/hosttest ./internal/integration
rtk go test -race ./internal/host ./internal/integration
rtk go test ./internal/schema ./internal/assets
rtk git diff --check
```

Expected after Phase 04 closes the batch: all Host and integration commands
PASS. At this checkpoint, schema/assets must pass; the other commands may name
only old Driver, Runtime, Admission, or Runner symbols owned by Phases 03-04.
Do not repair those consumers and do not publish.

- [ ] **Step 5: Commit receipt conformance**

```bash
rtk git add internal/host internal/hosttest internal/integration
rtk git commit -m "feat: validate host-native receipts"
```

## Phase 02 Completion Gate

- [ ] No active Host asset declares `runner-managed`, `native-managed`,
      `isolated-executor`, or `native-invocation`.
- [ ] Built-in integrations are all honest `policy` surfaces; no Host is
      promoted to `host-native` without a real integration.
- [ ] Host conformance validates data and transcripts but invokes nothing.
- [ ] A parent binding alone never proves `SUBAGENT` binding availability.
- [ ] Host records contain no secrets, command lines, HOME paths, raw model
      output, or private extension configuration.
- [ ] Production callers outside `internal/core` do not call
      `classification.Classify` or `registry.Resolve` directly.
