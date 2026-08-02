# OAW Runtime vNext Workflow Runtime Orchestration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start a durable `WORKFLOW` Run only after an explicit Profile-selection Gate, pin one immutable Lifecycle Bundle generation, execute its compiled graph through isolated Stage Grants and observations, serialize cross-Run Worktree leases, and emit non-authoritative project projections.

**Architecture:** Keep the Ticket 06 revision journal as the only Runtime authority and add a Workflow branch beside the existing Direct and Bounded branches. `profile` and `config` expose defensive, digest-pinned records so a Bundle can be fully reconstructed from committed JSON. `admission` issues generation-bound Stage Grants from compiled graph nodes; `runtime` owns Bundle state, graph transitions, lease arbitration, stable-boundary switching, and projection output. Host isolation, executor freshness, registry, authority, and configuration enter as trusted `Options` projections; Runtime never calls a Host Binding.

**Tech Stack:** Go 1.26, existing `runtime` journal and `flock` locking, `profile` ExecutionGraph, `config.Snapshot`, `admission` immutable Grant domain, `canonicaljson`, JSON state fixtures, table-driven tests, race tests, and the repository verification commands.

---

## Scope Boundary

Ticket 07 owns:

- Workflow-only blocking Profile Selection (`AWAITING_SELECTION`); Direct and Bounded startup behavior remains unchanged.
- Immutable Bundle generations containing the exact selection, recipe version/digest, Provider Instances, Profile Bindings, selected add-ons, public Configuration Snapshot record, and Execution Graph record/digest.
- Trusted Host isolation and Executor registration inputs needed to admit Workflow stages.
- Generation- and graph-node-bound Stage Grants, fresh read-only Review Executors, and graph-driven normalized observations.
- Cross-Run physical Worktree Resource Leases with one active write-capable Stage Grant per canonical worktree.
- Stable-boundary Bundle switching, old outstanding Grant revocation, and configuration adoption only by explicit selection.
- One-way projection rendering and lag recording; projection files are never read by Runtime.

It does not:

- implement the Host Manifest, Adapter Conformance Suite, or a third-party executable Host plugin (Ticket 08);
- invoke Providers, Host Bindings, Shell commands, or model/network APIs;
- change Direct or Bounded authority semantics, including Ticket 06's absence of Resource Leases;
- infer Profile selection from project documents or existing `.scratch` artifacts;
- allow a project projection to become Runtime authority;
- add a CLI transport or choose the first production Host (Tickets 09-14).

## Locked Interfaces

The implementation extends existing public records without changing Direct or
Bounded callers:

```go
type WorkflowOptions struct {
    Configuration config.Snapshot
    Registry      registry.Registry
    Authority     admission.AuthorityCeiling
    Host          WorkflowHostDeclaration
    Executors     []WorkflowExecutorRegistration
    Projection    ProjectionOptions
}

type WorkflowHostDeclaration struct {
    PhysicalIsolation bool
}

type WorkflowExecutorRegistration struct {
    Registration admission.ExecutorRegistration
    ReadOnly     bool
    Fresh        bool
}

type WorkflowInput struct {
    DeliverableID string
    InputDigest   string
}

type ProfileSelection struct {
    Profile  string
    Bindings []profile.ProfileBinding
}

type StageGrantRequest struct {
    ExecutorID           string
    RequestedEffects     []string
    RequestedResources   []string
    TerminationCondition string
}

type StageObservation struct {
    CapabilityObservation
    Signal          string
    StableBoundary string
}
```

`StartInput.Workflow` is required for a Workflow decision. `ContinueInput`
gains `ProfileSelection`, `StageGrant`, `StageObservation`, and
`StableBoundarySwitch` payloads with separate signals. The existing Bounded
payloads remain valid and are rejected on the Workflow branch. `RunSnapshot`
keeps the compatibility `LifecycleBundles []string`, `GrantIDs []string`, and
`ResourceLeaseIDs []string` fields and adds typed `Workflow *WorkflowState`;
the typed state contains full Bundle, Grant history, lease history, active node,
active Grant, revocations, and projection lag references.

## Task 1: Expose immutable configuration and graph records

**Files:**
- Modify: `internal/config/snapshot.go`, `internal/config/snapshot_test.go` (or the existing config test file)
- Modify: `internal/profile/records.go`, `internal/profile/compile.go`, `internal/profile/profile_test.go`
- Create: `internal/runtime/workflow_records.go`, `internal/runtime/workflow_records_test.go`

- [x] **Step 1: Write failing record tests.**

  Test `config.Snapshot.Record()` for a non-empty loaded snapshot, verify its
  digest equals `Snapshot.Digest()`, mutate returned catalog/settings/defaults,
  and verify a second call is unchanged. Test `ExecutionGraph.Record()` for
  recipe, bindings, Provider Instances, nodes, routes, terminal gates, stable
  boundaries, and digest. Mutating any returned nested slice must not change the
  graph or its digest. Test Bundle normalization rejects empty profile IDs,
  duplicate bindings/add-ons, invalid digests, generation zero, and missing
  graph/recipe identity.

- [x] **Step 2: Run the focused tests and verify RED.**

  Run:

  ```bash
  rtk go test ./internal/config ./internal/profile ./internal/runtime -run 'SnapshotRecord|GraphRecord|Bundle'
  ```

  Expected: compilation failures for the new record accessors and Workflow
  records.

- [x] **Step 3: Implement immutable records.**

  Add a public `config.SnapshotRecord` that is exactly the canonical record
  already used by `Snapshot.setDigest`, plus `Snapshot.Record()` returning
  defensive copies and `SnapshotRecord.Digest()`. Add public
  `profile.ExecutionGraphRecord`, `ExecutionGraph.Record()`, and a validating
  constructor that recomputes the graph digest from the existing canonical
  graph record. Do not expose mutable private graph fields. Add Workflow
  records, clone helpers, canonical digest finalization, sorted-set helpers,
  and stable validation errors in `internal/runtime/workflow_records.go`.

- [x] **Step 4: Run focused tests and compatibility tests.**

  Run:

  ```bash
  rtk gofmt -w internal/config/snapshot.go internal/profile/records.go internal/profile/compile.go internal/runtime/workflow_records.go
  rtk go test ./internal/config ./internal/profile ./internal/runtime -run 'SnapshotRecord|GraphRecord|Bundle'
  rtk go test ./internal/config ./internal/profile ./internal/runtime
  ```

  Expected: all new and existing tests pass; existing Direct/Bounded snapshot
  JSON still contains empty compatibility arrays.

- [x] **Step 5: Commit.**

  ```bash
  rtk git add internal/config internal/profile internal/runtime/workflow_records.go
  rtk git commit -m "feat: add immutable workflow bundle records"
  ```

## Task 2: Add Workflow startup Gate and Bundle generation

**Files:**
- Modify: `internal/runtime/records.go`, `internal/runtime/engine.go`, `internal/runtime/journal.go`, `internal/runtime/transitions.go`
- Create: `internal/runtime/workflow_start.go`, `internal/runtime/workflow_start_test.go`
- Modify: `internal/runtime/runtime_test.go`, `internal/runtime/invariants_test.go`

- [x] **Step 1: Write failing startup tests.**

  Add table-driven tests proving Direct START still returns `RELEASED` without
  a Bundle, Bounded START still returns `READY`/`AWAITING_CAPABILITY` without a
  Bundle, and Workflow START returns revision 1 `AWAITING_SELECTION` with
  `SELECTION_REQUIRED` and no compiled graph. Add a `PROFILE_SELECTED` test
  using the built-in `oaw/reliable-feature` fixture and complete trusted
  registry: revision 2 is `READY`, `Workflow.ActiveNodeID` equals the graph
  entry, generation is 1, exactly one Bundle is pinned, and its selection,
  recipe digest, Provider Instance digests, bindings, configuration digest,
  add-ons, and graph digest are persisted. Add tests for missing Workflow
  input, untrusted/missing Configuration or Registry, missing
  `PhysicalIsolation`, unknown profile, and unverified capability; all fail
  without a revision write and use stable diagnostics.

- [x] **Step 2: Run the focused tests and verify RED.**

  ```bash
  rtk go test ./internal/runtime -run 'WorkflowStart|ProfileSelected|SelectionRequired'
  ```

  Expected: the current `REQUEST_MODE_NOT_IMPLEMENTED` behavior or missing
  symbols causes failures; Direct and Bounded baseline tests remain green.

- [x] **Step 3: Implement the Workflow START path.**

  Extend `Options` and `Engine` with cloned Workflow options. Route only
  `classification.RequestModeWorkflow` to `workflowStart`; do not run the
  blocking Gate for Direct or Bounded. Normalize and persist `WorkflowInput`
  and project identity at START, create `RunAwaitingSelection`, and emit a
  committed `WORKFLOW_SELECTION_REQUIRED` reply. Add `SignalProfileSelected`
  and normalize `ProfileSelection` as user input. Under the existing run lock,
  replay before revision checks, require the awaiting state, verify the pinned
  configuration/registry and trusted host declaration, compile the selected
  Profile with `profile.CompileProfile`, derive add-ons from the compiled
  optional nodes, create generation 1 Bundle, set the entry node and `READY`,
  and commit `WORKFLOW_BUNDLE_CREATED` before returning `MODE_DECIDED`.

- [x] **Step 4: Extend journal validation and revision edges.**

  Add `validateWorkflowState`, `validateWorkflowReply`, and
  `validateWorkflowRevisionTransition`. Enforce immutable Run/classification/
  configuration identity, exactly one appended processed message, selection
  reply shape, full Bundle digest, generation 1 on first creation, entry node
  existence, graph digest consistency, and no Grants/Leases before a stage is
  requested. Extend `validateRevision` dispatch without altering Direct or
  Bounded validators.

- [x] **Step 5: Run focused and full tests.**

  ```bash
  rtk gofmt -w internal/runtime
  rtk go test ./internal/runtime -run 'Workflow|Direct|Bounded'
  rtk go test ./...
  ```

  Expected: Workflow Gate tests pass and all pre-Ticket-07 tests remain green.

- [x] **Step 6: Commit.**

  ```bash
  rtk git add internal/runtime
  rtk git commit -m "feat: add workflow profile selection gate"
  ```

## Task 3: Admit isolated Stage Grants and review Executors

**Files:**
- Modify: `internal/admission/records.go`, `internal/admission/admit.go`, `internal/admission/admission_test.go`
- Modify: `internal/runtime/records.go`, `internal/runtime/engine.go`, `internal/runtime/journal.go`, `internal/runtime/transitions.go`
- Create: `internal/runtime/workflow_grants.go`, `internal/runtime/workflow_grants_test.go`

- [x] **Step 1: Write failing admission tests.**

  Add `IssueWorkflowStageGrant` cases for exact compiled-node Provider and
  Binding pinning, generation and Bundle/Node/Graph identity, effects/resources
  narrowed by user authority and node ceilings, isolated Executor enforcement,
  Main Agent rejection, unknown effects/resources, stale graph digest, and
  mutable input copies. Add Runtime tests from `READY` proving a stage request
  commits one generation-bound Grant and returns `GRANT_ISSUED`. A request for
  `review` without an Executor ID must select a trusted `Fresh && ReadOnly`
  Executor not previously used; a non-review stage must require a registered
  isolated Executor. Missing physical isolation returns
  `HOST_ISOLATION_UNAVAILABLE`; no revision or Grant is created.

- [x] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/admission ./internal/runtime -run 'WorkflowGrant|StageGrant|ReviewExecutor|Isolation'
  ```

- [x] **Step 3: Implement the admission seam.**

  Add `StageGrantRequest` and `IssueWorkflowStageGrant` as a pure function that
  validates a compiled `profile.GraphNode` against the pinned Catalog/Registry,
  calls the existing immutable Grant finalizer with `RequestModeWorkflow`, and
  sets `BundleID`, `NodeID`, `GraphDigest`, and generation. Keep bounded Grant
  generation zero and its old limits. Extend Grant validation to distinguish
  bounded and workflow identity fields; never weaken existing `ValidateGrant`
  checks.

- [x] **Step 4: Implement Runtime stage issuance and Executor choice.**

  Add `SignalRequestStageGrant` (with `StageGrantRequest` payload) and route it
  only for Workflow `READY` snapshots. Resolve the active graph node, choose a
  fresh read-only Executor when the node responsibility is `review` and the
  request omits an ID, otherwise require an exact isolated registration. Issue
  the Stage Grant, append immutable Grant history, set `ActiveGrantID`, and
  commit `WORKFLOW_STAGE_GRANT_ISSUED`. Reject a second active Grant, a node or
  generation mismatch, and any write request without the future lease check.

- [x] **Step 5: Validate grant-state transitions and run tests.**

  Add strict `GRANTED` state/reply validation, append-only Grant history,
  immutable old Bundles, and active Grant identity checks. Run:

  ```bash
  rtk gofmt -w internal/admission internal/runtime
  rtk go test ./internal/admission ./internal/runtime -run 'Workflow|Grant|ReviewExecutor'
  rtk go test ./...
  ```

- [x] **Step 6: Commit.**

  ```bash
  rtk git add internal/admission internal/runtime
  rtk git commit -m "feat: issue isolated workflow stage grants"
  ```

## Task 4: Add cross-Run physical Worktree Resource Leases

**Files:**
- Create: `internal/runtime/workflow_leases.go`, `internal/runtime/workflow_leases_test.go`
- Modify: `internal/runtime/journal.go`, `internal/runtime/engine.go`, `internal/runtime/records.go`, `internal/runtime/transitions.go`
- Modify: `internal/runtime/bounded_test.go`, `internal/runtime/runtime_test.go`

- [x] **Step 1: Write failing lease tests.**

  Start two Workflow Runs in the same state root and canonical project root.
  Prove a write-capable stage Grant in Run A commits one lease, while Run B's
  concurrent write-capable request returns `RESOURCE_LEASE_CONFLICT` with no
  revision. Prove read-only stages in B can proceed concurrently. Use a symlink
  alias for the same physical worktree and prove it conflicts. Prove a
  completed observation releases the lease, a failed/uncertain stage retains
  it, and Direct/Bounded Runs never create or inspect OAW leases.

- [x] **Step 2: Run the focused tests and verify RED.**

  ```bash
  rtk go test ./internal/runtime -run 'Lease|ConcurrentWorkflow|Direct.*Lease|Bounded.*Lease'
  ```

- [x] **Step 3: Implement the lease primitive.**

  Add `ResourceLease` with immutable ID, Run/Grant/Bundle generation, resource
  kind, canonical physical worktree, acquisition revision, and digest. Add a
  state-root global `resource-leases/LOCK` using the existing `flock` library;
  acquire it before any Workflow stage mutation that can write. While holding
  it, load committed snapshots for all Runs, validate their chains, and reject
  any active write-capable lease for the same physical path. Store acquired
  leases in the Run snapshot and expose only active IDs in the compatibility
  `ResourceLeaseIDs` field. A completion transition removes the ID from active
  state but retains immutable history. Do not add lease logic to Direct or
  Bounded branches.

- [x] **Step 4: Integrate lease ordering safely.**

  Use one lock ordering (`resource lock` then `run lock`) for Workflow stage
  issuance and completion/switch transitions. Never hold a run lock while
  acquiring a resource lock. Ensure a failed admission or revision conflict
  leaves HEAD and lease state unchanged. Normalize worktree roots with the
  same physical-root helper used by project identity and reject non-absolute
  paths.

- [x] **Step 5: Run race and recovery tests.**

  ```bash
  rtk gofmt -w internal/runtime
  rtk go test -race ./internal/runtime -run 'Lease|Workflow'
  rtk go test ./...
  ```

- [x] **Step 6: Commit.**

  ```bash
  rtk git add internal/runtime
  rtk git commit -m "feat: enforce workflow worktree leases"
  ```

## Task 5: Authorize dispatch, drive graph observations, and switch Bundles

**Files:**
- Modify: `internal/runtime/records.go`, `internal/runtime/engine.go`, `internal/runtime/journal.go`, `internal/runtime/transitions.go`
- Create: `internal/runtime/workflow_dispatch.go`, `internal/runtime/workflow_dispatch_test.go`
- Modify: `internal/runtime/workflow_records.go`, `internal/runtime/workflow_grants.go`

- [x] **Step 1: Write failing dispatch and graph tests.**

  Prove exact `DISPATCH_PREPARED` moves Workflow `GRANTED` to `IN_FLIGHT` only
  for the committed Grant/invocation/Executor, and wrong/stale preparations do
  not mutate. A normalized observation with signal `succeeded` advances to the
  declared transition target and returns `READY`; terminal-gate success returns
  `FINISHED`. A declared incident signal routes through `IncidentRoutes`; an
  unknown signal is rejected. `RawOutput` is always rejected, evidence is
  digest-pinned, and replay returns the stored reply before revision checks.
  Prove graph nodes cannot be skipped, Provider output cannot invent a target,
  and no new Grant is minted until the next explicit stage request.

- [x] **Step 2: Write failing stable-boundary switch tests.**

  Mark a declared stable boundary on a successful observation, then switch to
  another verified Profile. Require generation 2, a new Bundle ID and digest,
  current configuration digest, entry node from the new graph, and old active
  Grant IDs in `RevokedGrantIDs`. Reject unknown boundaries, switching during
  `IN_FLIGHT`, and a switch without explicit user selection. A tampered old
  Bundle or projection must never influence the new Bundle.

- [x] **Step 3: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/runtime -run 'WorkflowDispatch|GraphTransition|StableBoundary|BundleSwitch'
  ```

- [x] **Step 4: Implement the dispatch handshake and graph driver.**

  Add Workflow-specific preparation validation while reusing the Ticket 06
  ordered handshake. On authorized observation, append an immutable
  `StageObservation`, resolve only a matching graph transition or incident
  route, release a lease only after a terminal/success transition is committed,
  clear `ActiveGrantID`, and set the next node. Preserve a lease for failed or
  uncertain observations. Extend normalized observations with a closed graph
  signal and optional declared stable-boundary ID; validate it against the
  pinned graph. Add `SignalSwitchProfile` that is legal only at a pinned stable
  boundary and outside `IN_FLIGHT`, revokes all outstanding Grants from the old
  generation, clears active leases, compiles the explicitly selected new graph,
  and commits the new Bundle generation atomically in the new revision.

- [x] **Step 5: Extend chain/state invariants and verify.**

  Enforce append-only observations and Grants, legal `READY -> GRANTED ->
  IN_FLIGHT -> READY/FINISHED` edges, generation-bound Grant identity,
  revocation monotonicity, stable-boundary membership, and Bundle generation
  monotonicity. Run:

  ```bash
  rtk gofmt -w internal/runtime
  rtk go test ./internal/runtime -run 'Workflow'
  rtk go test -race ./internal/runtime
  rtk go test ./...
  ```

- [x] **Step 6: Commit.**

  ```bash
  rtk git add internal/runtime
  rtk git commit -m "feat: drive workflow graph observations"
  ```

## Task 6: Add one-way projections, recovery coverage, review, and release evidence

**Files:**
- Create: `internal/runtime/projection.go`, `internal/runtime/projection_test.go`
- Modify: `internal/runtime/engine.go`, `internal/runtime/journal.go`, `internal/runtime/workflow_records.go`
- Create/Modify: `internal/integration/workflow_runtime_test.go`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`, `.scratch/oaw-runtime-vnext/issues/07-workflow-runtime-orchestration.md`, `.scratch/oaw-runtime-vnext/evidence/review.md`, `.scratch/oaw-runtime-vnext/evidence/verification.md`

- [x] **Step 1: Write failing projection and restart tests.**

  Inject a projection sink and prove every committed Workflow revision emits a
  summary containing Run/Bundle/node/status/revision digests but no full Grant,
  credential, or raw Provider output. Make the sink fail and prove the Run
  revision still commits while a lag marker is recorded under Runtime state.
  Restart the Engine at selection, READY, GRANTED, IN_FLIGHT, READY-after-
  observation, PAUSED/uncertain, and generation-switched states; `INSPECT`
  must reproduce the persisted snapshot and never read projection files.
  Tamper/delete a projection and prove Runtime behavior is unchanged.

- [x] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/runtime ./internal/integration -run 'Projection|Workflow.*Restart|Workflow.*Recovery'
  ```

- [x] **Step 3: Implement the one-way projection sink.**

  Define a small `ProjectionSink` interface and a filesystem implementation
  that atomically writes a redacted JSON/Markdown projection from a committed
  revision. Record projection failure/lag in a separate owner-only state file;
  never include the failure in the already persisted reply and never parse
  projection content on load, inspect, selection, dispatch, or switching.
  Invoke projection only after journal commit and return the persisted reply
  regardless of projection success.

- [x] **Step 4: Add integration and security coverage.**

  Build a real `config.Load`/`discovery.Discover`/`registry.Resolve` fixture for
  `oaw/reliable-feature`, verify optional ECC add-ons are pinned only when
  verified, exercise two concurrent Engines against one state root, assert
  owner-only permissions for runs/leases/lag/projection files, and scan state
  bytes for forbidden raw output and credentials. Confirm Runtime never calls
  Host Bindings by using bindings that would fail if invoked.

- [x] **Step 5: Run the complete verification matrix.**

  ```bash
  rtk gofmt -w internal/runtime internal/integration
  rtk git diff --check
  rtk go test ./... -count=1
  rtk go test -race ./...
  rtk go vet ./...
  rtk go test ./internal/runtime -coverprofile=/tmp/oaw-runtime-ticket07.cover
  rtk go tool cover -func=/tmp/oaw-runtime-ticket07.cover
  rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk bash tests/run.sh
  rtk go test ./internal/classification -run '^$' -fuzz FuzzDecodeProposalFailsClosed -fuzztime 2s
  rtk GOOS=linux GOARCH=amd64 go build -o /tmp/oaw-ticket07-linux ./cmd/oaw
  rtk GOOS=windows GOARCH=amd64 go build -o /tmp/oaw-ticket07-windows.exe ./cmd/oaw
  rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...
  ```

  Expected: all tests/race/vet/build/security checks pass, runtime coverage is
  at least 90%, repository coverage remains at least 80%, and no Direct or
  Bounded regression appears.

- [x] **Step 6: Perform Superpowers review and remediation.**

  Review `main...HEAD` for Gate leakage into Direct/Bounded, selection
  bypasses, unpinned Bundle fields, graph digest forgery, Provider invocation,
  Main Agent/isolation bypass, concurrent lease races, stale Grant reuse,
  review Executor reuse, unsafe switch timing, projection authority leakage,
  reply-before-commit windows, crash/orphan recovery, permissions, and secret
  exposure. Fix all Critical/High findings, re-run focused tests and the full
  matrix, and record findings plus evidence in the existing ticket evidence.

- [x] **Step 7: Update tracker and close the ticket.**

  Change the tracker to `current_stage: completed`, add Ticket 07's plan and
  evidence paths, mark every issue acceptance checkbox complete, record commit
  hashes and exact verification output, and preserve `.serena/` untouched.

- [x] **Step 8: Commit documentation closure.**

  ```bash
  rtk git add .scratch/oaw-runtime-vnext docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-07-workflow-runtime-orchestration.md
  rtk git commit -m "docs: close workflow runtime orchestration ticket"
  ```

## Plan Self-Review

- Spec section 5.3 is covered by Task 2's Workflow-only Gate and Task 5's
  successor/switch rules; Direct and Bounded remain explicitly untouched.
- Spec sections 9 and 11 are covered by Tasks 1-3 and 5: full Bundle pins,
  graph nodes/transitions, generation-bound Grants, isolated Executors, and
  review freshness.
- Spec section 12 is covered by Tasks 4-6: global lease lock, journal-only
  authority, ordered dispatch, crash/restart validation, and uncertainty-safe
  retention.
- Spec section 13's Host Manifest/conformance remains outside this ticket; the
  trusted isolation declaration is deliberately a Ticket 08 seam.
- Spec section 14's projection rule is covered by Task 6; no projection parser
  or authority fallback is introduced.
- There are no placeholder implementation steps: every task names exact files,
  state transitions, test commands, expected outcomes, and commit boundaries.
