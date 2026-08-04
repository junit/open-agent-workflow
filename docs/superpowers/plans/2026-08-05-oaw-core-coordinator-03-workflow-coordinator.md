# OAW Core Coordinator Phase 03 Workflow Coordinator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `internal/runtime` with a Workflow-only Coordinator that atomically compiles selected Bundles through OAW Core, persists deterministic revisions and leases, emits Host-neutral Dispatch Packets, and advances only from normalized Host Receipts.

**Architecture:** `Coordinator.Exchange` is the only durable state seam. The Coordinator owns no classification algorithm, Provider compiler, Host Driver, Agent, or tool invocation; it calls Core inside the locked `START`/`SWITCH` transition and treats the Host as an external protocol participant. Direct and Bounded work never enter this package.

**Tech Stack:** Go 1.26, canonical JSON, cross-process file locks, atomic file replacement, JSON Schema Draft 2020-12, immutable revision journal, table-driven, race, recovery, and fuzz tests.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. This phase performs the public state-command hard cut but remains unreleasable until Phase 04 deletes dead Runner source.

**Hard-cut integration boundary:** This phase completes the replacement logic for Core, Host, Admission, Coordinator, CLI, and integration tests. Obsolete root Driver and Codex Runner files may still prevent their package dependency closure from compiling until Phase 04 deletes them; do not repair them. Full GREEN and `go test ./...` are deliberately deferred to Phase 04.

**Depends on:** Phase 01 Core and Phase 02 Host-native records.

**Produces:** `oaw workflow exchange`; removes `oaw runtime exchange` and all Runtime State readers.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/coordinator/records.go` | Commands, results, state, Bundle references, and stable errors. |
| `internal/coordinator/engine.go` | One `Exchange` dispatcher and immutable transition entrypoint. |
| `internal/coordinator/start.go` | Core-backed START and SWITCH compilation. |
| `internal/coordinator/dispatch.go` | Grant, Dispatch Packet, and Receipt transitions. |
| `internal/coordinator/journal.go` | Revision locks, idempotency, atomic commit, and recovery reads. |
| `internal/coordinator/leases.go` | Cross-Workflow cooperative Resource Leases. |
| `internal/coordinator/projection.go` | One-way project Workflow projections. |
| `internal/coordinator/transport.go` | Strict one-command JSON transport. |
| `internal/coordinator/*_test.go` | State, invariant, replay, recovery, race, and fuzz tests. |
| `internal/cli/workflow.go` | `oaw workflow exchange` assembly and diagnostics. |
| `internal/assets/schemas/v1/workflow-command.schema.json` | Closed command union. |
| `internal/assets/schemas/v1/workflow-result.schema.json` | Closed result and denial union. |
| `internal/assets/schemas/v1/workflow-snapshot.schema.json` | Durable Workflow snapshot record. |
| `internal/assets/schemas/v1/workflow-revision.schema.json` | Immutable revision envelope. |
| `internal/assets/schemas/v1/workflow-head.schema.json` | Atomic HEAD pointer. |
| `internal/integration/workflow_coordinator_test.go` | Core-to-Coordinator-to-Receipt vertical slices. |

## Locked Coordinator Interface

```go
type CommandKind string

const (
    CommandStart   CommandKind = "START"
    CommandInspect CommandKind = "INSPECT"
    CommandPrepare CommandKind = "PREPARE"
    CommandReceipt CommandKind = "RECEIPT"
    CommandSwitch  CommandKind = "SWITCH"
    CommandCancel  CommandKind = "CANCEL"
)

type Command struct {
    SchemaVersion    string
    Kind             CommandKind
    MessageID        string
    IdempotencyKey   string
    WorkflowID       string
    ExpectedRevision uint64
    Start            *StartInput
    Prepare          *PrepareInput
    Receipt          *ReceiptInput
    Switch           *SwitchInput
    Cancel           *CancelInput
}

type Engine struct {
    core       CoreCompiler
    options    Options
    journal    *journal
    projection ProjectionSink
}

func NewEngine(options Options) (*Engine, error)
func (engine *Engine) Exchange(command Command) (Result, error)

type Options struct {
    StateRoot          string
    PhysicalProjectRoot string
    Rules              classification.ClassificationRules
    Configuration      config.Snapshot
    Resolutions        registry.ResolutionReport
    Registry           registry.Registry
    Authority          admission.AuthorityCeiling
    Core               CoreCompiler
    Projection         ProjectionSink
}

type ResultKind string

const (
    ResultState    ResultKind = "STATE"
    ResultDispatch ResultKind = "DISPATCH"
    ResultRejected ResultKind = "REJECTED"
)

type Diagnostic struct {
    Code   string `json:"code"`
    Detail string `json:"detail"`
}

type Result struct {
    SchemaVersion  string          `json:"schema_version"`
    Kind           ResultKind      `json:"kind"`
    WorkflowID     string          `json:"workflow_id"`
    Revision       uint64          `json:"revision"`
    RevisionDigest string          `json:"revision_digest"`
    Snapshot       *Snapshot       `json:"snapshot,omitempty"`
    Dispatch       *DispatchPacket `json:"dispatch,omitempty"`
    Diagnostics    []Diagnostic    `json:"diagnostics"`
    Replayed       bool            `json:"replayed"`
    Digest         string          `json:"digest"`
}
```

The six command variants are mutually exclusive. `INSPECT` has no payload and
never writes. All mutating commands require message identity, idempotency, and
the exact expected revision; `START` derives the Workflow ID from its
idempotency key and has expected revision zero.

## Locked State Model

```go
type Status string

const (
    StatusReady     Status = "READY"
    StatusPrepared  Status = "PREPARED"
    StatusInFlight  Status = "IN_FLIGHT"
    StatusPaused    Status = "PAUSED"
    StatusFinished  Status = "FINISHED"
    StatusCancelled Status = "CANCELLED"
)

type Snapshot struct {
    SchemaVersion     string
    WorkflowID        string
    RequestID         string
    DeliverableID     string
    Revision          uint64
    Status            Status
    Classification    classification.ClassificationDecision
    Bundles           []core.LifecycleBundle
    ActiveGeneration  uint64
    ActiveNodeID      string
    ActiveTicket      string
    ActiveGrant       *admission.CapabilityGrant
    GrantHistory      []admission.CapabilityGrant
    Receipts          []host.InvocationReceipt
    ResourceLeases    []ResourceLease
    LastStableBoundary string
    ProcessedMessages []ProcessedMessage
    ProjectionLag     []ProjectionLag
}

type ProcessedMessage struct {
    IdempotencyKey string `json:"idempotency_key"`
    ContentDigest  string `json:"content_digest"`
    Revision       uint64 `json:"revision"`
    ResultDigest   string `json:"result_digest"`
}

type ResourceLease struct {
    SchemaVersion    string `json:"schema_version"`
    ID               string `json:"id"`
    WorkflowID       string `json:"workflow_id"`
    GrantID          string `json:"grant_id"`
    BundleID         string `json:"bundle_id"`
    BundleGeneration uint64 `json:"bundle_generation"`
    Resource         string `json:"resource"`
    PhysicalRoot     string `json:"physical_root"`
    AcquiredRevision uint64 `json:"acquired_revision"`
    ReleasedRevision uint64 `json:"released_revision,omitempty"`
    Digest           string `json:"digest"`
}

type ProjectionLag struct {
    Revision uint64 `json:"revision"`
    Digest   string `json:"digest"`
    Reason   string `json:"reason"`
}
```

Use schema family `oaw.workflow-command/v1`, `oaw.workflow-result/v1`,
`oaw.workflow-snapshot/v1`, `oaw.workflow-revision/v1`, and
`oaw.workflow-head/v1`. Do not decode any `oaw.runtime/*` document.
The default `Options.StateRoot` is
`${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/workflows`; callers
may override it only with an absolute, non-symlinked test or integration root.

## Task 1: Define strict Workflow commands, state, and transport

**Files:**
- Create: `internal/coordinator/records.go`
- Create: `internal/coordinator/transport.go`
- Create: `internal/coordinator/records_test.go`
- Create: `internal/coordinator/transport_test.go`
- Create: `internal/coordinator/transport_fuzz_test.go`
- Create: `internal/assets/schemas/v1/workflow-command.schema.json`
- Create: `internal/assets/schemas/v1/workflow-result.schema.json`
- Create: `internal/assets/schemas/v1/workflow-snapshot.schema.json`
- Create: `internal/assets/schemas/v1/workflow-revision.schema.json`
- Create: `internal/assets/schemas/v1/workflow-head.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`

- [ ] **Step 1: Write failing closed-union tests**

Test every valid command shape plus unknown fields, mixed payloads, missing
idempotency, stale revisions, invalid UTF-8, trailing JSON, oversized input,
old Runtime frames, and forged digests. Assert old documents return
`SCHEMA_UNSUPPORTED`, never a best-effort decode.

- [ ] **Step 2: Run tests to verify RED**

```bash
rtk go test ./internal/coordinator ./internal/schema -run 'Command|Result|Transport|Schema'
```

Expected: FAIL because Coordinator and schemas do not exist.

- [ ] **Step 3: Implement records and strict transport**

```go
func DecodeCommand(raw []byte) (Command, error)
func EncodeResult(result Result) ([]byte, error)
func ExchangeJSON(in io.Reader, out io.Writer, engine *Engine) error
```

Limit input to 1 MiB, use `DisallowUnknownFields`, accept exactly one JSON
value, and write exactly one canonical JSON result. Human diagnostics go to the
CLI's stderr, not transport stdout.

- [ ] **Step 4: Run GREEN and fuzz seed checks**

```bash
rtk gofmt -w internal/coordinator internal/schema internal/assets
rtk go test ./internal/coordinator ./internal/schema ./internal/assets -run 'Command|Result|Transport|Schema'
rtk go test ./internal/coordinator -run Fuzz -fuzztime=1x
```

- [ ] **Step 5: Commit the Coordinator protocol**

```bash
rtk git add internal/coordinator internal/schema internal/assets
rtk git commit -m "feat: define workflow coordinator protocol"
```

## Task 2: Move the revision journal without legacy readers

**Files:**
- Create: `internal/coordinator/journal.go`
- Create: `internal/coordinator/replace_other.go`
- Create: `internal/coordinator/replace_windows.go`
- Create: `internal/coordinator/journal_test.go`
- Create: `internal/coordinator/invariants_test.go`

- [ ] **Step 1: Write failing commit, replay, and recovery tests**

Cover first commit, next revision, stale revision, identical replay, conflicting
idempotency content, torn temporary files, invalid `HEAD`, predecessor mismatch,
cross-process run locking, and an old Runtime state directory. Old state must
return `WORKFLOW_STATE_UNSUPPORTED` and remain untouched.

- [ ] **Step 2: Run journal tests to verify RED**

```bash
rtk go test ./internal/coordinator -run 'Journal|Revision|Replay|Recovery|Head'
```

- [ ] **Step 3: Port only the generic journal algorithms**

Copy the proven lock, canonical digest, atomic replacement, and append-only
validation algorithms from `internal/runtime/journal.go`; rewrite their record
types and error names for Workflow State. Do not import `internal/runtime`,
read its directory, or add a state translator.

- [ ] **Step 4: Run race and recovery checks**

```bash
rtk gofmt -w internal/coordinator
rtk go test ./internal/coordinator -run 'Journal|Revision|Replay|Recovery|Head'
rtk go test -race ./internal/coordinator -run 'Journal|Revision|Replay|Lock'
```

- [ ] **Step 5: Commit the Workflow journal**

```bash
rtk git add internal/coordinator
rtk git commit -m "feat: persist workflow revisions"
```

## Task 3: Compile and commit START inside the lock

**Files:**
- Create: `internal/coordinator/engine.go`
- Create: `internal/coordinator/start.go`
- Create: `internal/coordinator/start_test.go`
- Modify: `internal/coordinator/invariants_test.go`
- Modify: `internal/core/records.go`
- Modify: `internal/core/compile.go`
- Modify: `internal/core/core_test.go`

- [ ] **Step 1: Write failing Core-backed START tests**

Prove `START` accepts only a `WORKFLOW` classification proposal, invokes Core
once under the Workflow lock, commits exactly Core's Bundle generation 1, and
returns `READY` at the graph entry. Prove a caller-supplied Bundle field is an
unknown-field schema failure. Direct/Bounded proposals write no state.

- [ ] **Step 2: Run START tests to verify RED**

```bash
rtk go test ./internal/coordinator ./internal/core -run 'Start|CompileInsideLock|CallerAuthoredBundle'
```

- [ ] **Step 3: Implement START inputs and Core dependency**

```go
type StartInput struct {
    RequestID       string
    DeliverableID   string
    InputDigest     string
    ActiveTicket    string
    Proposal        classification.ClassificationProposal
    Selection       core.Selection
    HostSession     host.SessionSnapshot
    Environment     host.EnvironmentReport
}

type CoreCompiler interface {
    Classify(*classification.ClassificationProposal, classification.ClassificationRules) (classification.ClassificationDecision, error)
    Compile(core.CompilationRequest) (core.CompilationResult, error)
}
```

Call `Classify(&input.Proposal, engine.options.Rules)`, then construct
`CompilationRequest` only from trusted Engine options plus normalized START
facts. Require the result to contain one Bundle matching the selection,
Host session digest, topology, and generation before committing revision 1.

- [ ] **Step 4: Run START, Core, and invariant checks**

```bash
rtk gofmt -w internal/coordinator internal/core
rtk go test ./internal/coordinator ./internal/core -run 'Start|Bundle|Invariant|Classification'
```

- [ ] **Step 5: Commit atomic Workflow creation**

```bash
rtk git add internal/coordinator internal/core
rtk git commit -m "feat: start compiled workflows atomically"
```

## Task 4: Issue topology-bound Grants and Dispatch Packets

**Files:**
- Modify: `internal/admission/records.go`
- Modify: `internal/admission/admit.go`
- Modify: `internal/admission/admission_test.go`
- Create: `internal/assets/schemas/v2/capability-grant.schema.json`
- Create: `internal/assets/schemas/v1/dispatch-packet.schema.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/schema/registry.go`
- Modify: `internal/schema/registry_test.go`
- Create: `internal/coordinator/dispatch.go`
- Create: `internal/coordinator/dispatch_test.go`
- Modify: `internal/coordinator/invariants_test.go`

- [ ] **Step 1: Write failing PREPARE and Grant tests**

Prove requested effects/resources are subsets of the active graph node, one
active Grant exists, write-capable work requires a lease, and Packet identity
pins Workflow, Bundle, node, topology, Host session, Provider binding,
termination, input references, and evidence requirements. Assert there is no
Executor ID/kind or process command.

- [ ] **Step 2: Run PREPARE tests to verify RED**

```bash
rtk go test ./internal/admission ./internal/coordinator -run 'Prepare|Grant|DispatchPacket'
```

- [ ] **Step 3: Replace executor-based Grant fields**

```go
type CapabilityGrant struct {
    SchemaVersion          string             `json:"schema_version"`
    ID                     string             `json:"id"`
    WorkflowID             string             `json:"workflow_id"`
    RequestID              string             `json:"request_id"`
    BundleID               string             `json:"bundle_id"`
    BundleGeneration       uint64             `json:"bundle_generation"`
    BundleDigest           string             `json:"bundle_digest"`
    NodeID                 string             `json:"node_id"`
    Topology               execution.Topology `json:"topology"`
    HostSessionDigest      string             `json:"host_session_digest"`
    ProviderID             string             `json:"provider_id"`
    ProviderInstanceDigest string             `json:"provider_instance_digest"`
    CapabilityID           string             `json:"capability_id"`
    Binding                catalog.HostBinding `json:"binding"`
    Effects                []string           `json:"effects"`
    Resources              []string           `json:"resources"`
    TerminationCondition   string             `json:"termination_condition"`
    Digest                 string             `json:"digest"`
}
```

Remove `ExecutorRegistration`, `ExecutorKind`, and all isolated/main-agent
selection. Set `CapabilityGrantSchemaV2 = "oaw.capability-grant/v2"`.

- [ ] **Step 4: Implement `PREPARE`**

```go
type PrepareInput struct {
    RequestedEffects     []string
    RequestedResources   []string
    TerminationCondition string
    InputReferences      []ArtifactReference
    EvidenceRequirements []EvidenceRequirement
}

type ArtifactReference struct {
    Kind      string `json:"kind"`
    Reference string `json:"reference"`
    Digest    string `json:"digest"`
}

type EvidenceRequirement struct {
    Kind        string `json:"kind"`
    Minimum     uint64 `json:"minimum"`
    Description string `json:"description"`
}

type DispatchPacket struct {
    SchemaVersion           string                             `json:"schema_version"`
    ID                      string                             `json:"id"`
    WorkflowID              string                             `json:"workflow_id"`
    RequestID               string                             `json:"request_id"`
    BundleID                string                             `json:"bundle_id"`
    BundleGeneration        uint64                             `json:"bundle_generation"`
    BundleDigest            string                             `json:"bundle_digest"`
    NodeID                  string                             `json:"node_id"`
    Ticket                  string                             `json:"ticket,omitempty"`
    Topology                execution.Topology                 `json:"topology"`
    HostSessionDigest       string                             `json:"host_session_digest"`
    Grant                   admission.CapabilityGrant          `json:"grant"`
    InputReferences         []ArtifactReference                `json:"input_references"`
    EvidenceRequirements    []EvidenceRequirement              `json:"evidence_requirements"`
    EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements"`
    Digest                  string                             `json:"digest"`
}
```

Commit `PREPARED`, active Grant, Packet digest, and lease before returning the
Packet. The Host may prepare the selected context but must not perform admitted
effects until its `STARTED` Receipt is accepted.

- [ ] **Step 5: Run GREEN and authority checks**

```bash
rtk gofmt -w internal/admission internal/coordinator
rtk go test ./internal/admission ./internal/coordinator -run 'Prepare|Grant|DispatchPacket|Authority'
```

- [ ] **Step 6: Commit logical dispatch authority**

```bash
rtk git add internal/admission internal/coordinator internal/assets internal/schema
rtk git commit -m "feat: issue host-neutral dispatch packets"
```

## Task 5: Advance exclusively through Host Receipts

**Files:**
- Modify: `internal/coordinator/dispatch.go`
- Modify: `internal/coordinator/dispatch_test.go`
- Modify: `internal/coordinator/invariants_test.go`
- Modify: `internal/host/receipt.go`
- Modify: `internal/host/receipt_test.go`

- [ ] **Step 1: Write failing Receipt transition tests**

Cover `STARTED`, `COMPLETED`, `FAILED`, `PAUSED`, and `CANCELLED`; wrong Bundle,
node, topology, session, invocation handle, environment report, evidence, or
status; duplicate Receipt replay; terminal completion; declared incident route;
and undeclared signal. No transition may invoke Host code.

- [ ] **Step 2: Run Receipt tests to verify RED**

```bash
rtk go test ./internal/host ./internal/coordinator -run 'Receipt|Started|Completed|Incident|Paused|Cancelled'
```

- [ ] **Step 3: Implement Receipt payload and transitions**

```go
type ReceiptInput struct {
    Receipt        host.InvocationReceipt `json:"receipt"`
    Signal         string                 `json:"signal"`
    StableBoundary string                 `json:"stable_boundary,omitempty"`
}
```

`STARTED` changes `PREPARED` to `IN_FLIGHT` only after environment validation.
`COMPLETED` requires evidence closure and follows the declared graph edge.
`FAILED` follows one declared Incident Route or pauses with a stable reason.
`PAUSED` and `CANCELLED` preserve the normalized Receipt and release no lease
unless the transition contract says the invocation is terminal.

- [ ] **Step 4: Run state-machine GREEN and race checks**

```bash
rtk gofmt -w internal/host internal/coordinator
rtk go test ./internal/host ./internal/coordinator
rtk go test -race ./internal/coordinator -run 'Receipt|Exchange|Concurrent'
```

- [ ] **Step 5: Commit Receipt-driven transitions**

```bash
rtk git add internal/host internal/coordinator
rtk git commit -m "feat: advance workflows from host receipts"
```

## Task 6: Port leases, projections, switching, cancellation, and recovery

**Files:**
- Create: `internal/coordinator/leases.go`
- Create: `internal/coordinator/leases_test.go`
- Create: `internal/coordinator/projection.go`
- Create: `internal/coordinator/projection_test.go`
- Create: `internal/coordinator/switch.go`
- Create: `internal/coordinator/switch_test.go`
- Create: `internal/coordinator/cancel.go`
- Create: `internal/coordinator/cancel_test.go`
- Create: `internal/coordinator/recovery_test.go`

- [ ] **Step 1: Write failing lifecycle coordination tests**

Cover cross-Workflow physical-root lease conflict, read-only work without a
lease, terminal release, uncertain execution retention, explicit cancellation,
stable-boundary Profile/topology switch, Core recompile inside the lock,
generation increment, old Grant revocation, projection lag, and restart.

- [ ] **Step 2: Run focused tests to verify RED**

```bash
rtk go test ./internal/coordinator -run 'Lease|Projection|Switch|Cancel|Recovery'
```

- [ ] **Step 3: Port only Workflow coordination algorithms**

Use these exact command and projection records:

```go
type SwitchInput struct {
    Boundary    string                 `json:"boundary"`
    Selection   core.Selection         `json:"selection"`
    HostSession host.SessionSnapshot   `json:"host_session"`
    Environment host.EnvironmentReport `json:"environment"`
}

type CancelInput struct {
    Reason             string `json:"reason"`
    InvocationTerminal bool   `json:"invocation_terminal"`
}

type ProjectionRecord struct {
    SchemaVersion    string                   `json:"schema_version"`
    WorkflowID      string                   `json:"workflow_id"`
    Revision        uint64                   `json:"revision"`
    BundleGeneration uint64                  `json:"bundle_generation"`
    BundleDigest    string                   `json:"bundle_digest"`
    NodeID          string                   `json:"node_id"`
    Ticket          string                   `json:"ticket,omitempty"`
    Topology        execution.Topology       `json:"topology"`
    Evidence        []host.EvidenceReference `json:"evidence"`
    Digest          string                   `json:"digest"`
}

type ProjectionSink interface {
    WriteProjection(ProjectionRecord) error
}
```

Move and rename the proven algorithms from `internal/runtime/workflow_leases.go`,
`projection.go`, and stable switching. Reject Direct/Bounded inputs. Project
only non-sensitive Bundle generation, node, ticket, topology, and evidence
references; never read projections back as state.

- [ ] **Step 4: Run GREEN, race, and restart checks**

```bash
rtk gofmt -w internal/coordinator
rtk go test ./internal/coordinator -run 'Lease|Projection|Switch|Cancel|Recovery'
rtk go test -race ./internal/coordinator
```

- [ ] **Step 5: Commit lifecycle coordination**

```bash
rtk git add internal/coordinator
rtk git commit -m "feat: coordinate durable workflow lifecycle"
```

## Task 7: Cut the CLI and remove `internal/runtime`

**Files:**
- Create: `internal/cli/workflow.go`
- Create: `internal/cli/workflow_test.go`
- Modify: `internal/cli/run.go`
- Delete: `internal/cli/run_runtime.go`
- Delete: `internal/cli/run_runtime_test.go`
- Delete: `internal/cli/run_host_test.go`
- Delete: `internal/assets/schemas/v1/runtime-frame.schema.json`
- Delete: `internal/assets/schemas/v1/runtime-reply.schema.json`
- Delete: `internal/runtime/bounded.go`
- Delete: `internal/runtime/bounded_dispatch_invariants_test.go`
- Delete: `internal/runtime/bounded_dispatch_test.go`
- Delete: `internal/runtime/bounded_recovery_test.go`
- Delete: `internal/runtime/bounded_test.go`
- Delete: `internal/runtime/engine.go`
- Delete: `internal/runtime/invariants_test.go`
- Delete: `internal/runtime/journal.go`
- Delete: `internal/runtime/projection.go`
- Delete: `internal/runtime/projection_test.go`
- Delete: `internal/runtime/provider_diagnostics.go`
- Delete: `internal/runtime/records.go`
- Delete: `internal/runtime/replace_other.go`
- Delete: `internal/runtime/replace_windows.go`
- Delete: `internal/runtime/runtime_test.go`
- Delete: `internal/runtime/testdata/fuzz/FuzzDecodeFrameFailsClosed/e0350e6ff2f95a40`
- Delete: `internal/runtime/transitions.go`
- Delete: `internal/runtime/transport.go`
- Delete: `internal/runtime/transport_fuzz_test.go`
- Delete: `internal/runtime/transport_test.go`
- Delete: `internal/runtime/workflow_dispatch.go`
- Delete: `internal/runtime/workflow_dispatch_test.go`
- Delete: `internal/runtime/workflow_grants.go`
- Delete: `internal/runtime/workflow_grants_test.go`
- Delete: `internal/runtime/workflow_helpers_test.go`
- Delete: `internal/runtime/workflow_host.go`
- Delete: `internal/runtime/workflow_invariants_test.go`
- Delete: `internal/runtime/workflow_leases.go`
- Delete: `internal/runtime/workflow_leases_test.go`
- Delete: `internal/runtime/workflow_records.go`
- Delete: `internal/runtime/workflow_records_test.go`
- Delete: `internal/runtime/workflow_start.go`
- Delete: `internal/runtime/workflow_start_test.go`
- Delete: `internal/runtime/workflow_validation.go`
- Delete: `internal/integration/direct_runtime_test.go`
- Delete: `internal/integration/workflow_runtime_test.go`
- Create: `internal/integration/workflow_coordinator_test.go`
- Modify: `cmd/oaw/main.go`

- [ ] **Step 1: Write failing CLI hard-cut tests**

Assert `oaw workflow exchange` accepts one command and emits one result;
`oaw runtime exchange` and `oaw run --host codex` return `INVALID_ARGUMENT`;
no state is created for old commands; and stdout remains canonical JSON for the
new command.

- [ ] **Step 2: Run CLI tests to verify RED**

```bash
rtk go test ./internal/cli ./internal/integration -run 'WorkflowExchange|RuntimeRemoved|RunRemoved'
```

- [ ] **Step 3: Route only the new state command**

```go
case "workflow":
    return runWorkflowExchange(args[1:], stdin, stdout, stderr)
```

Update usage to list `oaw workflow exchange [--state-root path]
[--project-root path]`. Do not add aliases for `runtime` or `run`.

- [ ] **Step 4: Delete Runtime package and tests atomically**

Move no Direct/Bounded state code. Confirm every retained Workflow algorithm
already exists and is tested under `internal/coordinator`, then delete the old
directory, schemas, CLI files, and integration tests in the same commit.

- [ ] **Step 5: Run Phase 03 replacement-package verification**

```bash
rtk gofmt -w internal/coordinator internal/cli cmd/oaw
rtk go test ./internal/coordinator ./internal/cli ./internal/integration
rtk go test -race ./internal/coordinator
rtk bash scripts/check-docs.sh
rtk rg -n 'internal/runtime|oaw\.runtime/|oaw runtime exchange' internal cmd
```

Expected after Phase 04 deletes the remaining Driver/Runner files: all tests
pass. At this checkpoint, only those Phase 04 files may block compilation;
final `rg` exits 1 with no Runtime source match.

- [ ] **Step 6: Commit the state-plane hard cut**

```bash
rtk git add internal/coordinator internal/cli internal/integration internal/assets internal/schema cmd/oaw
rtk git add -u internal/runtime
rtk git commit -m "refactor: replace runtime with workflow coordinator"
```

## Phase 03 Completion Gate

- [ ] `internal/runtime` and all Runtime schemas are absent.
- [ ] Direct and Bounded requests have no Coordinator state path.
- [ ] Coordinator creates Bundles only by invoking Core inside a locked
      transition.
- [ ] Dispatch Packets contain no command, model, HOME, credential, or private
      Host configuration.
- [ ] Coordinator source imports no Codex/Claude runner and calls no Host
      Adapter.
- [ ] `oaw workflow exchange` is the only durable state command.
