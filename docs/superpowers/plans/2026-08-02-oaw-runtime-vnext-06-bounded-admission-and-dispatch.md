# OAW Runtime vNext Ticket 06 Bounded Admission and Dispatch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admit exactly one verified Bounded Capability for one declared deliverable, issue one immutable narrowed Grant, and persist the safe external-dispatch handshake through success, escalation, or execution uncertainty.

**Architecture:** `internal/admission` owns pure Capability/authority intersection and immutable Grant construction. User configuration supplies optional named Bounded selector defaults; Runtime accepts explicit user intent or an exact reference to one pinned user rule. The existing revision journal remains the sole transition authority. Bounded START persists either `AWAITING_CAPABILITY` or `READY`; subsequent frames issue a Grant, authorize a prepared dispatch, and record one normalized observation or a fail-closed pause. Trusted configuration, registry, authority, and Executor registrations enter through `Options`, never through Host self-attestation.

**Tech Stack:** Go 1.26, existing `catalog`, `config`, `registry`, `classification`, `canonicaljson`, and `runtime` packages, JSON Schema/TOML configuration fixtures, table-driven admission tests, race/restart journal tests, and existing repository verification commands.

---

## Scope Boundary

Ticket 06 owns:

- named user-trusted Bounded selector rules;
- exact verified Provider/Capability/Binding resolution;
- Bounded effect, resource, Executor, and parent-Grant narrowing;
- immutable Capability Grant and invocation identities;
- durable Grant issuance, Host preparation acceptance, dispatch authorization,
  normalized observation, escalation, and uncertainty transitions;
- strict Bounded state/reply/replay validation.

It does not:

- select or compile a Workflow Profile or create a Lifecycle Bundle;
- execute a Provider or wait for Host execution;
- accept Host self-attestation of isolation or conformance;
- implement the Host Manifest/Adapter conformance model from Ticket 08;
- implement cross-Run Resource Leases or Workflow Stage Grants from Ticket 07;
- implement the machine JSON CLI transport or select the first Host;
- permit a second Bounded Capability or a remediation loop in the same Run.

## Locked Trusted Inputs

Extend Runtime options without changing Direct callers:

```go
type Options struct {
    StateRoot string
    Rules     classification.ClassificationRules
    Bounded   BoundedOptions
}

type BoundedOptions struct {
    Configuration config.Snapshot
    Registry      registry.Registry
    Authority     admission.AuthorityCeiling
    Executors     []admission.ExecutorRegistration
}
```

`config.Snapshot` supplies the exact Catalog, configuration digest, and named
user-owned selector rules. `registry.Registry` supplies only verified Provider
Instances and Host Bindings. `AuthorityCeiling` and `Executors` are trusted Host
Adapter projections until Ticket 08 replaces them with pinned conforming Host
integration records. Run frames may request narrower authority or name an
Executor; they cannot widen these options.

User configuration gains only this user-owned record:

```toml
[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/ecc"
capability_id = "review"
```

Project configuration cannot define or replace these rules. A proposal selector
with `source = "trusted-rule"` must also carry the exact rule ID and match the
pinned user rule. `source = "user-intent"` must not carry a rule ID.

## Locked Bounded Request and Grant

```go
type BoundedInput struct {
    DeliverableID        string
    InputDigest          string
    RequestedEffects     []string
    RequestedResources   []string
    TerminationCondition string
    ExecutorID           string
    TrustedRuleID        string
}
```

Effects and resources are closed, sorted, unique sets. `git-local` is outside
the Bounded ceiling because Bounded work cannot own Git completion. Until
Ticket 07 supplies the cross-Run Resource Lease primitive, Ticket 06 also denies
`write-project` at Runtime dispatch with `RESOURCE_LEASE_REQUIRED`; it never
issues an unmanaged write Grant. Read-only or declared non-project-mutating
Capabilities remain admissible. A Main Agent Grant is permitted only when both
the verified Capability and trusted Executor record allow
`main-agent-allowed`.

`admission.CapabilityGrant` pins:

- schema, Grant, Run, Request, Deliverable, and invocation identities;
- issue revision and generation zero;
- Provider Instance and Descriptor digests;
- exact Capability and Host Binding;
- Executor identity/topology;
- input and termination condition;
- effective effects, resources, and delegation allow-list;
- optional parent Grant ID and immutable Grant digest.

Top-level Bounded Runtime Grants carry an empty delegation allow-list and admit
one logical invocation. `admission.DeriveChildGrant` exists as a pure validated
domain seam for later Workflow orchestration: the child Capability must appear
in the authoritative parent's allow-list, and effects, resources, and onward
delegation must be subsets of both the parent and child Capability ceilings.
The Bounded Runtime itself never dispatches that second Capability.

## Locked Runtime Transitions

Bounded frame signals:

```text
CAPABILITY_SELECTED
REQUEST_DISPATCH
DISPATCH_PREPARED
CAPABILITY_OBSERVED
EXECUTION_UNCERTAIN
SCOPE_EXPANDED
ADDITIONAL_CAPABILITY_REQUIRED
REMEDIATION_REQUIRED
ARCHITECTURE_REQUIRED
```

Transition contract:

```text
START without admissible selector
  -> AWAITING_CAPABILITY / CAPABILITY_SELECTION_REQUIRED

START with admissible selector, or CAPABILITY_SELECTED
  -> READY / MODE_DECIDED

REQUEST_DISPATCH
  -> GRANTED / GRANT_ISSUED

DISPATCH_PREPARED with exact Grant + invocation
  -> IN_FLIGHT / DISPATCH_AUTHORIZED

CAPABILITY_OBSERVED succeeded
  -> FINISHED / FINISHED

CAPABILITY_OBSERVED failed, or scope/additional/remediation/architecture signal
  -> PAUSED / MODE_ESCALATION_REQUIRED / START_SUCCESSOR_RUN

EXECUTION_UNCERTAIN after authorization
  -> PAUSED / EXECUTION_UNCERTAIN / RECONCILE_INVOCATION
```

Every mutating frame commits exactly one revision before replying. The Runtime
never invokes the Host Binding. A preparation for the wrong Grant, invocation,
or Executor fails without mutation. Replays return the original stored reply.
No signal can change `BOUNDED` to another Request Mode in place.

## Stable Ticket 06 Codes

Add and test:

```text
BOUNDED_CONFIGURATION_REQUIRED
BOUNDED_REQUEST_INVALID
CAPABILITY_SELECTION_REQUIRED
CAPABILITY_NOT_VERIFIED
CAPABILITY_MODE_NOT_ALLOWED
CAPABILITY_AUTHORITY_EXCEEDED
CAPABILITY_EFFECT_NOT_ALLOWED
CAPABILITY_RESOURCE_NOT_ALLOWED
RESOURCE_LEASE_REQUIRED
EXECUTOR_NOT_REGISTERED
EXECUTOR_TOPOLOGY_DENIED
PARENT_GRANT_INVALID
CHILD_GRANT_NOT_ALLOWED
RUN_TRANSITION_INVALID
DISPATCH_PREPARATION_INVALID
OBSERVATION_INVALID
```

Existing `MODE_ESCALATION_REQUIRED`, `EXECUTION_UNCERTAIN`, Runtime frame,
journal, conflict, corruption, and idempotency codes remain stable.

## Planned Files

| Path | Responsibility |
| --- | --- |
| `internal/admission/records.go` | Authority, Executor, request, Grant, observation records and defensive copies. |
| `internal/admission/admit.go` | Catalog/Registry resolution, effective-authority intersection, top-level Grant issuance, child narrowing. |
| `internal/admission/admission_test.go` | Pure admission and Grant invariants at >=90% coverage. |
| `internal/config/records.go`, `decode.go`, `snapshot.go` | User-trusted named Bounded selector defaults and immutable accessors/digest. |
| `internal/assets/schemas/v1/user-config.schema.json` | Strict user configuration contract for named defaults. |
| `internal/runtime/records.go` | Bounded frames, statuses, replies, snapshot state, Grants, observations. |
| `internal/runtime/engine.go` | Bounded START/selection/admission/dispatch transitions. |
| `internal/runtime/journal.go` | Mode-dispatched semantic validation for Direct and Bounded revisions. |
| `internal/runtime/bounded_test.go` | Public Runtime Bounded transition and failure tests. |
| `internal/integration/bounded_runtime_test.go` | Config/Registry integration, restart, replay, concurrency, and crash tests. |

### Task 1: Add user-trusted Bounded defaults and pure admission

- [x] Add a failing config test for one normalized named default, duplicate IDs,
  invalid Provider/Capability IDs, project-layer rejection, defensive copies,
  and configuration digest changes.
- [x] Run the focused config tests and confirm RED.
- [x] Extend the user schema, records, decoding normalization, snapshot digest,
  and accessor. Keep project configuration unable to grant a default.
- [x] Add failing admission tests for verified exact resolution, unavailable
  Capability, Bounded mode mismatch, unknown effects/resources, authority
  excess, Main Agent topology denial, and unregistered Executor.
- [x] Implement immutable admission records, deterministic identities, sorted
  set intersection, Catalog/Registry digest checks, and exact Binding pinning.
- [x] Add child-Grant tests proving Capability allow-list and strict
  effects/resources/delegation narrowing.
- [x] Run focused race, vet, and >=90% admission coverage checks.
- [x] Commit as `feat: add bounded capability admission`.

### Task 2: Persist Bounded classification and selector resolution

- [x] Add a failing `START` test with a complete Bounded proposal, declared
  deliverable, and explicit user-intent selector; require revision 1 `READY`
  and `MODE_DECIDED` with no Grant yet.
- [x] Add failing missing/unverified/ambiguous selector tests; require committed
  `AWAITING_CAPABILITY` and `CAPABILITY_SELECTION_REQUIRED`, not Workflow Gate.
- [x] Add trusted-rule tests proving exact rule ID/selector matching and rejection
  of Host-claimed rules absent from the pinned user configuration.
- [x] Extend frames/snapshots, clone methods, content digests, START routing, and
  strict Bounded state validation. Preserve the Direct byte/state contract.
- [x] Add `CAPABILITY_SELECTED` to move an awaiting Run to READY without
  reclassifying or changing Request Mode.
- [x] Run focused race/restart/replay tests and commit as
  `feat: persist bounded runtime admission`.

### Task 3: Issue one immutable Bounded Grant

- [x] Add a failing `REQUEST_DISPATCH` test from READY; require revision 2,
  status `GRANTED`, reply `GRANT_ISSUED`, and one exact immutable Grant plus
  invocation ID committed before reply.
- [x] Prove requested effects/resources are intersections of trusted authority,
  descriptor maxima, verified Capability, Request Mode, and Executor topology.
- [x] Prove a second Grant request, parent/child widening, lifecycle ownership,
  project write without a Resource Lease, Git completion effect, or changed
  Grant content fails without mutation or pauses for successor-Run escalation
  as applicable.
- [x] Implement deterministic Grant/invocation IDs and snapshot Grant copies;
  keep Lifecycle Bundles, Stage Grants, and Resource Leases empty in Ticket 06.
- [x] Run focused race/coverage tests and commit as
  `feat: issue bounded capability grants`.

### Task 4: Persist dispatch authorization, observations, and uncertainty

- [ ] Add a failing `DISPATCH_PREPARED` test that names the exact committed Grant,
  invocation, and Executor and returns `DISPATCH_AUTHORIZED` only after the
  `IN_FLIGHT` revision commits.
- [ ] Reject wrong or stale preparation, observation before authorization,
  duplicate distinct invocation attempts, and raw/unknown outcomes.
- [ ] Add successful normalized observation tests ending `FINISHED` with
  digest-pinned Evidence References and no further Grant.
- [ ] Add failed observation and explicit scope/additional/remediation/
  architecture signals that pause with `MODE_ESCALATION_REQUIRED` and only
  `START_SUCCESSOR_RUN`.
- [ ] Add post-authorization uncertainty that pauses with
  `EXECUTION_UNCERTAIN`; never mint or authorize a retry invocation.
- [ ] Prove replay before revision checks across every handshake frame.
- [ ] Commit as `feat: persist bounded dispatch handshake`.

### Task 5: Harden Bounded recovery and concurrency

- [ ] Add restart/INSPECT tests at AWAITING_CAPABILITY, READY, GRANTED,
  IN_FLIGHT, FINISHED, and both PAUSED reasons.
- [ ] Add concurrent dispatch requests proving one Grant/invocation wins and
  all other distinct messages receive revision conflicts.
- [ ] Add matching/conflicting orphan tests at Grant issuance and dispatch
  authorization, semantic-tamper fixtures for Grants/observations/status, and
  permission assertions.
- [ ] Assert Runtime never invokes a Binding and all Ticket 07/08/09 authority
  fields remain absent.
- [ ] Run `go test -race ./internal/admission ./internal/runtime ./internal/integration`,
  enforce >=90% admission/runtime coverage, and commit as
  `test: harden bounded runtime dispatch`.

### Task 6: Review, verify, and merge Ticket 06

- [ ] Review `main...HEAD` for selector provenance forgery, unverified Provider
  admission, mutable Grants, authority widening, Main Agent topology bypass,
  reply-before-commit windows, Host invocation, blind retry, Workflow leakage,
  and Direct regression.
- [ ] Record findings/remediation in the existing review evidence.
- [ ] Run fresh `go vet`, repository race tests, repository/admission/runtime
  coverage, Bash syntax, ShellCheck, full Bash tests, classifier fuzz,
  `govulncheck`, and Linux/Windows builds.
- [ ] Mark Ticket 06 complete and move the tracker to Ticket 07 only after every
  acceptance criterion has matching evidence.
- [ ] Commit verification evidence, run a final clean `go test ./...`, merge the
  named branch into `main`, re-run merged tests, remove only the Ticket 06
  worktree/branch, and continue to Ticket 07.
