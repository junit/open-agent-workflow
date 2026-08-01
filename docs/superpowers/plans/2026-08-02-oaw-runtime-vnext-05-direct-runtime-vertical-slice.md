# OAW Runtime vNext Ticket 05 Direct Runtime Vertical Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist one Direct Engineering Run through the Runtime `START`, `CONTINUE`, and `INSPECT` protocol seams without creating Capability admission, lifecycle, or lease authority.

**Architecture:** A new `internal/runtime` package owns closed protocol records, immutable Run snapshots, deterministic transitions, and an owner-only revision journal. `Engine.Exchange` validates one frame, classifies a normalized proposal, and commits one complete revision under a per-Run cross-process lock before returning; immutable revision files and an atomically replaced `HEAD` make either the prior or complete next revision authoritative after a crash. Direct scope expansion records a requirement to start a Successor Run while preserving the original `DIRECT` mode and creating no Bundle, Grant, or Resource Lease.

**Tech Stack:** Go 1.26, existing `classification` and `canonicaljson` packages, `github.com/gofrs/flock` for cross-platform process locks, standard filesystem primitives, table-driven public-seam tests, race tests, fault-injected journal fixtures, and existing repository verification commands.

---

## Scope Boundary

Ticket 05 owns the first authoritative Runtime and journal vertical slice. It does not:

- resolve or invoke a Provider Capability;
- issue a Capability Grant or Stage Grant;
- compile or select a Profile or create a Lifecycle Bundle;
- acquire a Resource Lease for Direct work;
- implement Bounded dispatch, Workflow graph execution, Host conformance, or projections;
- expose the machine JSON CLI transport selected by Ticket 09;
- alter Bash installation state or make Go installation commands authoritative.

Non-Direct classification is rejected with `REQUEST_MODE_NOT_IMPLEMENTED` until
Tickets 06 and 07 add their transitions. Missing classification input returns
`CLASSIFICATION_REQUIRED`; neither outcome creates Runtime State.

## Locked Public Seam

```go
type Options struct {
    StateRoot string
    Rules     classification.ClassificationRules
}

func NewEngine(options Options) (*Engine, error)
func (engine *Engine) Exchange(frame RunFrame) (RunReply, error)
```

`StateRoot` is an explicit absolute OAW Runtime namespace; the core never reads
`HOME`, `XDG_STATE_HOME`, or the current directory. Production callers will
resolve the XDG root. Tests use an isolated temporary root.

```go
type RunFrame struct {
    SchemaVersion    string
    Kind             FrameKind
    MessageID        string
    IdempotencyKey   string
    RunID            string
    ExpectedRevision uint64
    Start            *StartInput
    Continue         *ContinueInput
}

type StartInput struct {
    RequestID string
    Project   ProjectIdentity
    Proposal  *classification.ClassificationProposal
}

type ContinueInput struct {
    Signal ContinueSignal
}
```

The v1 frame kinds are `START`, `CONTINUE`, and `INSPECT`; Ticket 05 admits only
the `SCOPE_EXPANDED` Continue signal. START derives an opaque stable Run ID from
the idempotency key, normalizes and digests the Classification Proposal, and
persists the exact physical project root plus configuration digest.

Replies use the already approved kinds `MODE_DECIDED`, `PAUSED`, and
`STATE_SNAPSHOT`. A successful Direct START returns a `MODE_DECIDED` reply with
state `RELEASED`. The reply diagnostics must state both that Direct execution is
outside Capability admission and that OAW does not control subsequent Host tool
calls or apply Resource Lease guarantees.

## Locked Journal Contract

```text
<state-root>/runs/<run-id>/
    LOCK
    HEAD
    revisions/00000000000000000001.json
```

- Directories use owner-only mode `0700`; files use `0600` on supported systems.
- Each revision pins Run ID, revision number, predecessor digest, message and
  idempotency identities, message digest, event, resulting state, state digest,
  configuration digest, and emitted reply.
- A transition locks the Run, validates current HEAD/revision content and the
  expected revision, writes and syncs the immutable next revision, atomically
  replaces and syncs HEAD, then returns the stored reply.
- INSPECT reads one committed HEAD/revision pair and creates no revision.
- Replaying an idempotency key with identical normalized content returns its
  stored reply. Reusing it with different content returns
  `IDEMPOTENCY_KEY_REUSED`.
- An orphan revision written without a HEAD update is non-authoritative.
- Corrupt HEAD, revision digest, predecessor chain, Run identity, or state digest
  fails closed with a stable `RUN_STATE_*` code.

## Direct State Contract

`RunSnapshot` pins the Run/Request/Project identity, revision, Request Mode,
status, normalized Classification record/digest, configuration digest, processed
message references, and authority collections. For Direct all lifecycle bundle,
Grant, and Resource Lease collections are non-nil and empty.

`SCOPE_EXPANDED` appends a committed revision whose state remains `DIRECT` and
`RELEASED`; its `PAUSED` reply carries `MODE_ESCALATION_REQUIRED` and the single
recovery action `START_SUCCESSOR_RUN`. It never mutates the original mode or
silently creates a Workflow/Bounded authority object.

Stable Ticket 05 failures include `RUNTIME_FRAME_INVALID`,
`RUNTIME_SCHEMA_UNSUPPORTED`, `CLASSIFICATION_REQUIRED`,
`REQUEST_MODE_NOT_IMPLEMENTED`, `PROJECT_IDENTITY_INVALID`, `RUN_NOT_FOUND`,
`RUN_REVISION_CONFLICT`, `IDEMPOTENCY_KEY_REUSED`, `RUN_STATE_HEAD_INVALID`,
`RUN_STATE_REVISION_INVALID`, `RUN_STATE_DIGEST_MISMATCH`, and
`RUN_STATE_WRITE_FAILED`.

## File Structure

| Path | Responsibility |
| --- | --- |
| `internal/runtime/records.go` | Closed protocol enums/DTOs, snapshots, diagnostics, replay records, errors, and defensive copies. |
| `internal/runtime/engine.go` | Frame validation, project/proposal normalization, classification, Direct transitions, idempotent replay, and reply construction. |
| `internal/runtime/journal.go` | Cross-process Run lock, immutable revisions, strict load validation, atomic HEAD replacement, permission handling, and directory sync. |
| `internal/runtime/runtime_test.go` | Public `Exchange` behavior for START, INSPECT, scope expansion, replay, and fail-closed validation. |
| `internal/integration/direct_runtime_test.go` | Durable restart, concurrency, orphan revision, corruption, permissions, and classifier/config integration. |
| `go.mod`, `go.sum` | Pin the cross-platform file-lock dependency. |

### Task 1: Commit a Direct START before replying

**Files:**
- Create: `internal/runtime/records.go`
- Create: `internal/runtime/engine.go`
- Create: `internal/runtime/journal.go`
- Create: `internal/runtime/runtime_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write the failing Direct START tracer test**

Add `TestStartDirectRunCommitsReleasedSnapshotBeforeReply`. Construct a complete
clear Direct proposal, an existing temporary project root, and a lowercase
configuration digest. Call `Exchange(START)` and assert:

```go
reply.Kind == runtime.ModeDecided
reply.Snapshot.RequestMode == classification.RequestModeDirect
reply.Snapshot.Status == runtime.RunReleased
reply.Revision == 1
len(reply.Snapshot.LifecycleBundles) == 0
len(reply.Snapshot.GrantIDs) == 0
len(reply.Snapshot.ResourceLeaseIDs) == 0
```

Also assert both required Direct diagnostics, non-empty Run/decision/revision
digests, physical Project identity, and that `HEAD` plus revision 1 exist before
`Exchange` returns.

- [ ] **Step 2: Run the tracer to verify RED**

```bash
rtk go test ./internal/runtime -run TestStartDirectRunCommitsReleasedSnapshotBeforeReply
```

Expected: FAIL because `internal/runtime` does not exist.

- [ ] **Step 3: Add the closed records and minimum engine**

Implement the locked records and `NewEngine`. Validate frame shape, IDs, an
absolute State Root, an existing physical project directory, and a lowercase
64-character configuration digest. Marshal then call
`classification.DecodeProposal` so a typed proposal is subject to the same
normalization/limits as JSON input, classify it, and admit only `DIRECT` in this
ticket.

- [ ] **Step 4: Add owner-only revision 1 persistence**

Pin `github.com/gofrs/flock@v0.13.0`. Under the per-Run lock, create revision 1
as compact Canonical JSON, sync it, atomically replace/sync HEAD, then return the
reply stored in that committed revision. The Run ID is `run-` plus 32 lowercase
SHA-256 hex characters derived from the idempotency key; validate it before it
is used as a path component.

- [ ] **Step 5: Run GREEN, race, and vet checks**

```bash
rtk gofmt -w internal/runtime
rtk go test ./internal/runtime -run TestStartDirectRunCommitsReleasedSnapshotBeforeReply
rtk go test -race ./internal/runtime
rtk go vet ./internal/runtime
```

Expected: PASS.

- [ ] **Step 6: Commit the vertical tracer**

```bash
rtk git add go.mod go.sum internal/runtime
rtk git commit -m "feat: persist direct runtime start"
```

### Task 2: Add read-only INSPECT and strict journal loading

**Files:**
- Extend: `internal/runtime/records.go`
- Extend: `internal/runtime/engine.go`
- Extend: `internal/runtime/journal.go`
- Extend: `internal/runtime/runtime_test.go`

- [ ] **Step 1: Write failing INSPECT and restart tests**

Add `TestInspectReturnsCommittedSnapshotWithoutRevision` and
`TestNewEngineReadsCommittedDirectRunAfterRestart`. START a Run, count revision
files, create a fresh Engine over the same State Root, then INSPECT. Assert the
snapshot/reply equals committed revision 1 and the revision-file count and HEAD
bytes remain unchanged.

- [ ] **Step 2: Run the tests to verify RED**

```bash
rtk go test ./internal/runtime -run 'Inspect|Restart'
```

- [ ] **Step 3: Implement strict committed-state loading**

Read and strictly decode HEAD, validate its schema/Run ID/revision/digest, read
the exact immutable revision, recompute its Canonical JSON digest, then validate
revision identity, number, state digest, configuration digest, and predecessor
shape. INSPECT returns `STATE_SNAPSHOT` and never takes a write path.

- [ ] **Step 4: Prove defensive reply copies and stable missing/corrupt errors**

Mutate every returned collection and nested classification selector, INSPECT
again, and assert committed state did not change. Add table cases for missing
Run, malformed HEAD, missing revision, digest mismatch, and state mismatch with
their stable codes.

- [ ] **Step 5: Run focused GREEN and commit**

```bash
rtk gofmt -w internal/runtime
rtk go test -race ./internal/runtime -run 'Inspect|Restart|Corrupt|Missing'
rtk git add internal/runtime
rtk git commit -m "feat: inspect committed runtime state"
```

### Task 3: Record Direct scope expansion without upgrading in place

**Files:**
- Extend: `internal/runtime/engine.go`
- Extend: `internal/runtime/journal.go`
- Extend: `internal/runtime/runtime_test.go`

- [ ] **Step 1: Write the failing scope-expansion test**

Add `TestDirectScopeExpansionRequiresSuccessorRun`. Continue a Direct Run with
the exact current revision and `SCOPE_EXPANDED`; assert revision 2 commits, reply
kind is `PAUSED`, reason is `MODE_ESCALATION_REQUIRED`, recovery contains only
`START_SUCCESSOR_RUN`, and state remains `DIRECT`/`RELEASED` with no Bundle,
Grant, or Lease.

- [ ] **Step 2: Run the test to verify RED**

```bash
rtk go test ./internal/runtime -run TestDirectScopeExpansionRequiresSuccessorRun
```

- [ ] **Step 3: Implement the immutable Continue transition**

Under the Run lock, load/validate current HEAD, require the exact expected
revision, clone the snapshot, append one processed-message reference, set the
new revision number, build the PAUSED reply, and atomically commit revision 2.
Do not re-run classification or mutate Request Mode.

- [ ] **Step 4: Add revision-conflict and signal validation tests**

Wrong expected revisions return `RUN_REVISION_CONFLICT` without writing.
Unknown Continue signals and invalid START/CONTINUE/INSPECT payload shapes
return `RUNTIME_FRAME_INVALID` without creating or changing Run State.

- [ ] **Step 5: Run focused GREEN and commit**

```bash
rtk gofmt -w internal/runtime
rtk go test -race ./internal/runtime -run 'ScopeExpansion|Revision|Frame'
rtk git add internal/runtime
rtk git commit -m "feat: record direct mode escalation"
```

### Task 4: Prove replay, crash, concurrency, and authority invariants

**Files:**
- Extend: `internal/runtime/runtime_test.go`
- Create: `internal/integration/direct_runtime_test.go`

- [ ] **Step 1: Write failing idempotency tests**

Replay START and CONTINUE with the same key/content and assert the stored reply
is returned byte-equivalently without a new revision. Reuse either key with a
different normalized proposal or signal and assert `IDEMPOTENCY_KEY_REUSED`
without mutation.

- [ ] **Step 2: Implement processed-message replay**

Store sorted `ProcessedMessage` references containing key, normalized content
digest, and revision number. Check replay before expected-revision validation;
load the referenced immutable revision and return its stored reply. Never store
maps or recursive reply copies in Runtime State.

- [ ] **Step 3: Add concurrency and crash fixtures**

Start the same idempotent Run concurrently from multiple Engine instances and
assert one revision and identical replies. Continue concurrently with distinct
keys at one expected revision and assert exactly one revision 2 commit while the
others receive revision conflicts. Write a valid orphan revision file without
updating HEAD and prove INSPECT still returns revision 1.

- [ ] **Step 4: Add permission and no-authority integration assertions**

On systems exposing Unix mode bits, assert Runtime directories are `0700` and
files are `0600`. Across START, replay, expansion, restart, and INSPECT assert
all lifecycle bundle, Grant, and Resource Lease collections stay empty and the
required Host-control disclaimer remains present only in the Direct release
reply.

- [ ] **Step 5: Run focused race/coverage and commit**

```bash
rtk gofmt -w internal/runtime internal/integration
rtk go test -race ./internal/runtime ./internal/integration
rtk go test -cover ./internal/runtime
rtk git add internal/runtime internal/integration/direct_runtime_test.go
rtk git commit -m "test: harden direct runtime journal"
```

Require at least 90% `internal/runtime` statement coverage.

### Task 5: Review and complete Ticket 05 verification

**Files:**
- Modify only Ticket 05 code/tests if review finds a defect.
- Modify: `.scratch/oaw-runtime-vnext/issues/05-direct-runtime-vertical-slice.md`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`

- [ ] **Step 1: Review the complete diff**

Inspect `git diff main...HEAD` for reply-before-commit windows, mutable snapshots,
path traversal, unsafe modes, missing sync/rename steps, non-Direct authority,
silent mode changes, replay ambiguity, race windows, raw environment reads,
Provider execution, or Ticket 06/07 scope expansion. Record findings and
remediation in the existing review evidence file.

- [ ] **Step 2: Run fresh Go quality gates**

```bash
rtk go vet ./...
rtk go test -race ./...
rtk go test -coverprofile=/tmp/oaw-ticket-05-coverage.out ./...
rtk go tool cover -func=/tmp/oaw-ticket-05-coverage.out
rtk go test -coverprofile=/tmp/oaw-ticket-05-runtime.out ./internal/runtime
rtk go tool cover -func=/tmp/oaw-ticket-05-runtime.out
```

Require repository coverage at least 80% and `internal/runtime` at least 90%.

- [ ] **Step 3: Run compatibility, fuzz, vulnerability, and build gates**

```bash
rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk bash tests/run.sh
rtk go test ./internal/classification -run '^$' -fuzz FuzzDecodeProposalFailsClosed -fuzztime 2s
rtk govulncheck ./...
rtk env GOOS=linux GOARCH=amd64 go build ./cmd/oaw
rtk env GOOS=windows GOARCH=amd64 go build ./cmd/oaw
```

Record unavailable optional tools or platform limitations explicitly.

- [ ] **Step 4: Record evidence and complete the ticket**

Check each acceptance criterion only after matching implementation/test evidence
exists. Set Ticket 05 complete and move the tracker to the next unblocked Ticket
06. Preserve all `.serena/` directories and keep them out of commits.

- [ ] **Step 5: Commit evidence and merge automatically**

```bash
rtk git add .scratch/oaw-runtime-vnext docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-05-direct-runtime-vertical-slice.md
rtk git commit -m "docs: record ticket 05 verification"
```

After a final clean `rtk go test ./...`, merge the named Ticket 05 branch into
`main`, re-run the full test suite on the merged result, then remove only the
Ticket 05 worktree and branch. User authorization for this automatic local merge
is recorded in the active thread goal.
