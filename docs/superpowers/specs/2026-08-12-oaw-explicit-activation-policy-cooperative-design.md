# OAW Explicit Activation and Policy-Cooperative Degradation Design

**Status:** Approved design, pending implementation plan  
**Date:** 2026-08-12  
**Scope:** Explicit activation, request-mode semantics after activation,
policy-only assurance, Host bootstrap rendering, migration, and documentation

## Summary

Installing Open Agent Workflow must not place every engineering request under
OAW governance. Until the user explicitly invokes OAW for a deliverable, the
Agent Host behaves as though OAW were not installed: it keeps its native
conversation flow, automatic Skill and Agent selection, tools, approvals, and
execution behavior.

Explicit activation creates one task-scoped OAW Engagement. OAW then performs
an assurance preflight and classifies the activated deliverable as `DIRECT`,
`BOUNDED`, or `WORKFLOW`. Request Mode describes the work contract. Assurance
Level separately describes which claims the installed Host integration can
support.

Instruction-only Hosts use `policy-cooperative` assurance. They may follow OAW
ownership and lifecycle guidance, but they must not claim machine guarantees
that require Host-observed evidence, OAW Core output, or Workflow Coordinator
state. A Bridge or another Host-native adapter may upgrade the same activated
deliverable to `core-backed` or `coordinator-backed` assurance.

This change is intentionally behavior-breaking for installations that relied
on OAW's previous always-on classification. The installer will replace only
OAW-owned managed content and preserve unrelated user instructions.

## Problem

The current canonical policy says to classify every new top-level engineering
request. The current Host renderers repeat that behavior and, on some Hosts,
unconditionally import the complete policy. This creates three problems:

1. Installing OAW changes ordinary Host behavior even when the user did not ask
   OAW to participate.
2. A normal Host Skill invocation can be confused with OAW `BOUNDED`, although
   the user only intended to use the Host normally.
3. The policy-only path claims access to verified Providers and the same
   Core-produced Lifecycle Bundle even though it exposes no Host binding
   inventory or public Core inspect/compile interface.

The resulting interface is both intrusive and stronger than the implementation
can honestly support.

## Goals

- Preserve native Host behavior until explicit user activation.
- Keep `DIRECT`, `BOUNDED`, and `WORKFLOW`, but evaluate them only inside an
  activated OAW Engagement.
- Make activation task-scoped and sticky for related follow-ups without
  creating a session-global OAW mode.
- Separate Request Mode from Assurance Level.
- Keep policy-only OAW useful through honest cooperative degradation.
- Define exact Bounded selection rules that do not convert ordinary Host Skill
  routing into OAW governance.
- Give policy-only Workflow work a usable selection gate, plan, tracker, and
  conservative stop conditions.
- Preserve existing Core, Coordinator, and Host-native machine terminology for
  records that actually satisfy those contracts.
- Update all Host renderers and migration tests as one behavior change.

## Non-Goals

- Changing the classification schema or Core classification algorithm.
- Making `ordinary` Workflow complexity reachable.
- Removing `DIRECT` or `BOUNDED`.
- Adding `INACTIVE`, `BYPASS`, or another Request Mode.
- Implementing trusted-rule matching for `bounded_capability_defaults`.
- Changing the ten-slot engineering-delivery taxonomy.
- Defining new Effect, Resource, Grant, Lease, approval, Evidence, uncertain
  execution, or version-negotiation protocols.
- Making policy-only state authoritative or machine enforced.
- Adding a process fallback for unavailable Host-native features.
- Automatically migrating an active legacy Markdown lifecycle lock.

## Domain Model

### Activation State

Activation State is outside Request Mode.

```text
Current request router
    |- unrelated request ----------------------> Native Host
    `- explicit activation or related follow-up
         `-> Active OAW Engagement
              |- DIRECT
              |- BOUNDED
              `- WORKFLOW

Active OAW Engagement
    |- unrelated request -> preserve Engagement; route that request to Native Host
    `- complete, cancel, or explicit exit -> close Engagement
```

`Native Host` is the default. It is not an OAW Request Mode and must not be
reported as `DIRECT`, `BOUNDED`, or `BYPASS`.

An `Active OAW Engagement` is bound to one deliverable. The first release
supports at most one unfinished OAW Engagement in a conversation. Parallel or
named Engagements require a later design with explicit deliverable identity.

### Assurance Level

Assurance Level is orthogonal to Request Mode.

| Level | Supported claims | Unsupported claims |
| --- | --- | --- |
| `policy-cooperative` | Cooperative assessment, Host-visible candidates, Policy Workflow Plan, Progress Tracker, execution notes, and conflict warnings. | Canonical Core classification, verified Provider Instance, eligible Profile, Lifecycle Bundle, Capability Grant, Resource Lease, Host Receipt, atomic revision, idempotency, or recovery enforcement. |
| `core-backed` | Core classification, Host-verified Provider resolution, reason-coded eligibility, trusted selection preview, and immutable Lifecycle Bundle. | Coordinator revision, Lease, idempotency, durable transition, and recovery guarantees. |
| `coordinator-backed` | Core-backed claims plus durable revisions, admitted Grants, cooperative Leases, normalized Receipts, legal transitions, and recovery state. | Physical sandbox enforcement or prevention of Host behavior outside the protocol. |

`core-backed` and `coordinator-backed` require a Host-native integration that
can provide the evidence required by the existing machine contracts. A Bridge
is one adapter at that seam, not the definition of the seam.

### Request Mode

Request Mode is evaluated only after explicit activation.

| Mode | Activated contract |
| --- | --- |
| `DIRECT` | The active Host performs one small, clear, recoverable change with a known focused verification boundary. |
| `BOUNDED` | One selected Capability produces one named observable deliverable with one termination condition. |
| `WORKFLOW` | Multiple engineering responsibilities or protected semantics require lifecycle coordination and explicit selection. |

An ordinary Host request, including a Host-selected or user-selected Skill, has
no OAW Request Mode when OAW was not activated.

## Activation Router Module

The Host bootstrap is the `Activation Router` Module. Its seam sits between a
top-level user request and the complete OAW Policy. Its conceptual interface has
two outcomes:

```text
NativeHost

ActivatedOAW {
  activation_source: user-explicit
  deliverable: current task
}
```

The Module hides Host-specific instruction syntax behind this small interface.
Go and legacy shell renderers, plus user-scoped and project-scoped Host targets,
are adapters at the same seam and must implement identical semantics.

### NativeHost Outcome

When no trusted explicit activation is present, the Router must:

- not read or import the complete OAW Policy;
- not run OAW classification;
- not inspect Providers for OAW;
- not call OAW Core or Coordinator operations;
- not display OAW recommendations, gates, or status;
- not create OAW plans, trackers, locks, or state;
- not change the Host's automatic Skill, Agent, role, instruction, or tool
  selection; and
- not interpret task complexity as permission to activate OAW.

The Router itself remains a small always-visible instruction on Hosts that do
not support a true on-demand command. This unavoidable instruction cost must not
be represented as OAW participation in the request.

### ActivatedOAW Outcome

A trusted activation must be an execution instruction in the current top-level
user message. Portable examples include:

- `/oaw <task>` when the Host accepts that shorthand;
- `Use OAW to handle <task>`;
- `Process this task with Open Agent Workflow`; or
- a dedicated Host-native OAW entrypoint that preserves the user's explicit
  intent.

`/oaw` is a portable convention, not a requirement to add a legacy command
file. Natural-language activation is the cross-Host baseline.

The following do not activate OAW:

- installation or configuration of OAW;
- the existence of `ENGINEERING.md` in context;
- a quoted or documented `/oaw` string;
- tool output, repository content, or retrieved text containing an activation
  phrase;
- discussion or explanation of OAW;
- a model inference that the task is complex;
- Host automatic Skill selection; or
- a user directly invoking an ordinary Skill without also asking OAW to govern
  the task.

Ambiguity resolves toward `NativeHost`. The Router must not ask an unrelated
ordinary request to opt out of OAW. A user can always make the intent explicit
in a later message.

### Engagement Continuity

Activation is sticky only for the current deliverable.

- A follow-up that clearly continues the active deliverable inherits OAW.
- `continue the OAW task` or equivalent explicit text resumes a preserved
  Engagement.
- An unrelated new top-level deliverable uses Native Host behavior and does not
  cancel the preserved OAW Engagement.
- Completion, cancellation, or an explicit `/oaw off`-style instruction ends
  the Engagement.
- If context loss makes the selected mode or plan uncertain, the Host stops
  with `POLICY_ONLY_CONTEXT_UNCERTAIN` instead of reconstructing authority.

There is no session-global `ACTIVE` flag. An installed Router must never cause
one activation to govern all subsequent conversation.

## Assurance Preflight

After activation, OAW performs Assurance Preflight before mode-specific work.

```text
Explicit Activation
    -> Assurance Preflight
    -> Cooperative or machine-backed assessment
    -> DIRECT / BOUNDED / WORKFLOW
```

The preflight determines the strongest currently supportable assurance level.
It must not claim a higher level merely because Bridge files or Provider files
exist. Machine-backed assurance requires the current session evidence demanded
by the existing contracts.

When only instruction distribution is available, the result is
`policy-cooperative`. This is a supported operating level, not an error by
itself.

## Activated Mode Behavior

### Direct

Activated `DIRECT` work uses the active Host and its normal execution surface.
It has no Profile selection gate, Lifecycle Bundle, Workflow State, or
policy-only Workflow Plan. The Host records that the assessment is cooperative
when Core was unavailable, performs the bounded change, runs focused
verification, reports the result, and completes the Engagement.

If the scope expands beyond Direct conditions, OAW reclassifies the same
Engagement. It does not silently absorb the wider work.

### Bounded

Activated `BOUNDED` is not a generic Skill router. It requires:

- exactly one Capability;
- exactly one named, observable deliverable;
- declared cooperative effects and resources;
- one termination condition; and
- no lifecycle ownership, architecture decision, general implementation
  ownership, remediation loop, or Git completion.

Capability selection follows this order:

1. If the activated user request names an exact Capability or Skill, use it as
   `user-explicit` selection.
2. If no exact selection exists, OAW may show one Host-visible candidate but
   must wait for user confirmation.
3. An automatic selection is allowed only when a future exact user-trusted rule
   can prove both its match and selected Capability.

The current `bounded_capability_defaults` interface does not define a matching
predicate or carry rule identity into `CapabilitySelector`. This release must
not present those defaults as implemented trusted-rule routing.

Needing a second Capability, a remediation loop, an architectural decision, or
wider effects/resources triggers reclassification, normally to `WORKFLOW`.

### Workflow

Activated `WORKFLOW` uses one of two selection paths:

- Core-backed selection produces verified eligibility, trusted selection
  preview, and an immutable Lifecycle Bundle under the existing contracts.
- Policy-cooperative selection produces candidate diagnostics, an explicit user
  choice, a Policy Workflow Plan, and a Progress Tracker.

The policy-cooperative path must never use `eligible` for an unverified
Profile. Candidate dispositions are:

- `complete candidate`: every necessary engineering responsibility has a
  Host-visible proposed owner;
- `incomplete candidate`: at least one necessary responsibility lacks a
  Host-visible proposed owner; or
- `unavailable`: the visible Host surface cannot support the requested
  candidate.

Policy-only execution supports `CURRENT`. It cannot declare `SUBAGENT`
eligible because static instruction context is not current-session delegation
evidence.

## Cooperative Selection Gate

For policy-cooperative `WORKFLOW`, the Gate must:

1. State `policy-cooperative`, Request Mode, current complexity/risk assessment,
   and concrete evidence.
2. State that verified Provider, Lifecycle Bundle, Grant, Lease, Receipt,
   idempotency, and recovery guarantees are unavailable.
3. Perform Governance Inspection of Host-visible Provider and Capability
   metadata without invoking lifecycle work.
4. Show complete, incomplete, and unavailable Profile candidates with exact
   missing responsibilities.
5. Show `CURRENT` as the only policy-supported topology.
6. Show every proposed Bounded add-on and its single deliverable.
7. Wait for the user's explicit Profile candidate, topology, add-on, and
   acceptance of policy-only limitations.
8. Create the Policy Workflow Plan and Progress Tracker before lifecycle work
   begins.

`Governance Inspection` includes policy loading, configuration reading,
Provider metadata inspection, candidate calculation, explanation, and status.
It is distinct from lifecycle problem discovery, design, planning,
implementation, debugging, review, verification, or completion. The latter
remain blocked until the selection Gate closes.

If there is no complete candidate, the Gate stops. It does not wait forever on
an empty selection set and does not invent an owner.

## Policy-Only Artifacts

Policy-only artifacts use names that cannot be confused with machine records.

| Policy-only artifact or observation | Reserved machine term |
| --- | --- |
| Cooperative Assessment | Core Classification Decision |
| Host-visible Candidate | Verified Provider Instance |
| Bounded Plan | Capability Grant |
| Policy Workflow Plan | Lifecycle Bundle |
| Progress Tracker | Lifecycle Lock or Workflow State |
| Execution Note | Host Receipt |
| Conflict Warning | Resource Lease |

### Policy Workflow Plan

A Policy Workflow Plan is a human-readable cooperative contract. At minimum it
records:

```text
assurance: policy-cooperative
activation_source: user-explicit
deliverable: <human-readable scope>
mode: WORKFLOW
complexity: <cooperative assessment>
risk: <cooperative assessment>
selected_profile_candidate: <id>
selection_source: user-explicit
topology: CURRENT
responsibility_map: <ten-slot candidate mapping>
accepted_limitations: <policy-only limitations>
status: active | completed | stopped
```

The line-oriented example is explanatory, not a new machine schema. The Plan
must not fabricate Bundle IDs, generations, digests, Grants, Leases, Receipts,
or Coordinator revisions.

### Progress Tracker

A Progress Tracker is a best-effort policy record. It may track the selected
candidate, current lifecycle slot, active deliverable, completed artifacts,
known evidence, stop reason, and next cooperative action.

It is not authoritative, atomic, or guaranteed to survive context loss. The
Host may preserve it in the project's existing documentation layout, but OAW
must not claim persistence unless the Host actually wrote and recovered it.

## Stop Conditions

Policy-cooperative work fails closed on unsupported authority and uncertainty.
The initial stable reasons are:

| Reason | Required behavior |
| --- | --- |
| `CAPABILITY_SELECTION_REQUIRED` | Stop Bounded work until the user selects the candidate or a future exact trusted rule proves the selection. |
| `POLICY_ONLY_PROVIDER_UNVERIFIED` | Stop a request that requires a verified Provider or exact Binding guarantee. |
| `POLICY_ONLY_PROFILE_INCOMPLETE` | Stop Workflow selection when a necessary responsibility has no Host-visible candidate owner. |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | Stop when the requested OAW-managed topology is not `CURRENT`. |
| `POLICY_ONLY_GUARANTEE_UNAVAILABLE` | Stop when the task requires Grant, Lease, Receipt, idempotency, atomic revision, or recovery enforcement. |
| `POLICY_ONLY_CONCURRENT_MUTATION` | Stop or serialize when another task may mutate overlapping project or Git resources. |
| `POLICY_ONLY_EXECUTION_UNCERTAIN` | Do not retry an external or destructive effect whose outcome is unknown. |
| `POLICY_ONLY_CONTEXT_UNCERTAIN` | Stop and require explicit reconfirmation when prior selection or progress cannot be recovered reliably. |

Scope expansion is not itself a failure. It triggers reclassification inside
the same Engagement.

At every policy-only stop, the user may explicitly exit OAW and return the
request to Native Host behavior. OAW must not make that choice silently.

## Host Renderer Design

The canonical Router semantics must have one source within each implementation
language and remain byte-tested across Host adapters.

### Go Renderer

`internal/management/render.go` will replace all always-on classification text
with Host-specific renderings of the Activation Router contract. Shared project
targets continue to share one canonical managed block.

### Legacy Shell Renderer

`lib/render.sh` remains a compatibility implementation and must emit the same
semantic contract as the Go renderer. Shell and Go parity tests remain
mandatory until the shell management path is retired.

### Host-Specific Rules

- Claude and Gemini renderers must not unconditionally `@`-import the complete
  Policy. Their Router instructs the Host to read the Policy only after trusted
  activation.
- Codex, OpenCode, Cline, and Roo managed instructions use the same lazy Router
  semantics.
- Cursor, Windsurf, and Copilot may retain always-applied metadata required by
  their Host, but the body is only the lazy Router.
- The Router must name the canonical Policy path so an activated request can
  load the exact installed policy.

No renderer needs to install a slash-command compatibility file in this
release.

### Canonical Router Body

Every Host adapter must preserve the following normative meaning, with only the
Policy path and Host-required wrapper syntax varying:

```text
Open Agent Workflow is opt-in. Unless the current top-level user request
explicitly asks to use OAW, or clearly continues an active OAW task, behave as
the native Host: do not read the OAW Policy, classify the request, inspect OAW
Providers, mention OAW, create OAW state, or change normal Skill, Agent, role,
instruction, or tool selection. Installing OAW, discussing or quoting OAW,
task complexity, and ordinary Skill invocation do not activate OAW. On explicit
activation, read <POLICY_PATH> and apply it only to that deliverable. Related
follow-ups inherit activation; unrelated requests remain native. Completion,
cancellation, or explicit exit closes the OAW Engagement.
```

The implementation may wrap this body for Host metadata or line-length needs,
but it must not weaken any negative condition. Go and shell renderers must emit
the same body for equivalent targets.

## Policy Changes

The canonical Policy will be reorganized in this order:

1. Purpose and physical authority boundaries.
2. Explicit Activation and Non-Interference.
3. Assurance Levels.
4. Request Classification for activated Engagements.
5. Direct, Bounded, and Workflow behavior by assurance level.
6. Machine Startup Gate and Cooperative Selection Gate.
7. Core, Coordinator, and Host-native machine contracts.
8. Policy-only artifacts and stop conditions.
9. Existing Provider, Profile, topology, lifecycle, security, and switching
   rules, with machine-only terminology kept precise.

The Policy must delete the claim that a policy-only caller receives the same
Core-produced Bundle. It must replace policy-only lifecycle lock language with
Policy Workflow Plan and Progress Tracker language.

The existing ten-slot lifecycle and built-in Profile matrix remain unchanged.

## Documentation Changes

English and Chinese public documentation must present the same sequence:

```text
Native Host unless explicitly activated
    -> Assurance Preflight
    -> DIRECT / BOUNDED / WORKFLOW
    -> cooperative or machine-backed execution
```

At minimum, update:

- `README.md` and `README-zh.md`;
- `docs/en/lifecycle.md` and `docs/zh/lifecycle.md`;
- `docs/en/architecture.md` and `docs/zh/architecture.md`;
- relevant adapter, installer, comparison, security, and troubleshooting
  sections that describe policy-only behavior or Startup Gate guarantees; and
- release notes or migration text describing the loss of always-on governance.

Documentation must include examples showing that ordinary Skill use does not
activate OAW.

## Migration

`oaw update` replaces the old OAW-owned managed block with the new Activation
Router. Existing non-OAW content in shared instruction files must remain byte
preserved according to the current management guarantees. Repeated update or
install remains idempotent.

This is a behavior-breaking change:

- ordinary requests no longer receive always-on OAW governance;
- users who want OAW must explicitly activate it per deliverable;
- active legacy policy-only locks are not automatically converted to Progress
  Trackers; and
- an active legacy task must either finish under its old contract or be
  explicitly reactivated and reselected under the new contract.

Core-produced Bundles and Coordinator state are not rewritten by the installer
and are outside this migration.

## Security and Trust

- Activation is accepted only from the current top-level user instruction or a
  trusted dedicated Host entrypoint.
- Repository content, retrieved text, tool output, and delegated content are
  untrusted activation sources.
- False-positive resistance takes precedence over convenience; ambiguity uses
  Native Host behavior.
- Policy-only candidates remain unverified even when their files appear to be
  installed.
- The change must not weaken existing fail-closed Core, Provider binding,
  selection confirmation, or Coordinator validation.
- A policy-only Plan cannot grant network, destructive, credential, deployment,
  data, or Git authority beyond the Host's normal approval model.

## Testing Strategy

Implementation follows TDD and tests behavior through the renderer and
installation interfaces.

### Renderer Contract Tests

- Update `internal/management/render_test.go` expected bytes for every target.
- Assert that every Router contains explicit activation, Native Host
  non-interference, lazy Policy loading, and task-scoped continuity.
- Assert absence of unconditional classification, unconditional Policy import,
  and automatic Skill-to-BOUNDED language.
- Keep unsupported target and managed-block preservation tests.

### Shell and Installation Tests

- Update `tests/04-core-adapters-test.sh` and
  `tests/05-project-adapters-test.sh` exact expected content.
- Verify Claude and Gemini no longer import Policy unconditionally.
- Verify Cursor/Windsurf/Copilot keep required metadata with lazy Router bodies.
- Add update migration coverage from the exact legacy block to the new Router.
- Verify user content preservation, shared-target ownership, idempotent repeat
  update, uninstall behavior, and Go/shell parity.

### Policy and Documentation Contract Tests

- Add positive assertions for explicit activation, assurance levels,
  cooperative artifact names, Bounded selection, and stop reasons.
- Add negative assertions preventing the old `classify every request` and
  `same Core-produced Bundle in policy-only` claims.
- Update `scripts/check-docs.sh` and `tests/10-docs-test.sh` literals and
  bilingual parity checks.

### Regression Verification

Run, at minimum:

```text
go test ./internal/management ./internal/cli ./internal/check
go test -race ./internal/management ./internal/cli ./internal/check
tests/04-core-adapters-test.sh
tests/05-project-adapters-test.sh
tests/08-backup-test.sh
tests/09-transaction-test.sh
tests/10-docs-test.sh
tests/11-check-parity-test.sh
tests/12-install-parity-test.sh
tests/13-mutation-parity-test.sh
tests/run.sh
go test -race ./...
scripts/check-docs.sh
```

Existing Core, Coordinator, Bridge, Provider, Profile, and conformance suites
must continue to pass without schema changes.

## Acceptance Criteria

1. An ordinary request produces no OAW classification, prompt, gate, artifact,
   or state and preserves native Host Skill selection.
2. Direct invocation of an ordinary Skill without OAW activation remains Native
   Host behavior.
3. Explicit OAW activation creates one task-scoped Engagement and runs Assurance
   Preflight before Request Mode handling.
4. Related follow-ups inherit activation; unrelated new deliverables do not.
5. Policy-only Direct work completes without Profile selection.
6. Policy-only Bounded work requires user selection when no exact Capability
   was named.
7. Policy-only Workflow work uses candidate terminology, `CURRENT`, explicit
   limitation acceptance, Policy Workflow Plan, and Progress Tracker.
8. Policy-only output never claims verified Provider, eligible Profile,
   Lifecycle Bundle, Grant, Lease, Receipt, atomic state, or recovery guarantee.
9. Host-native integrations retain their existing Core and Coordinator paths.
10. Update replaces only OAW-owned legacy content, preserves user content, and
    remains idempotent.
11. English and Chinese documentation describe the same activation and
    assurance model.
12. No Classification, Provider, Profile, Bundle, Coordinator, or Host Receipt
    schema changes are introduced by this release.

## Deferred Work

The following require separate versioned designs and implementation plans:

- deterministic natural-language-to-Trait proposal construction;
- trusted Bounded rule predicates and rule identity;
- Provider revocation and just-in-time binding revalidation;
- normative Effect/Resource identity and conflict algebra;
- Lease expiry, renewal, fencing, and crash reconciliation;
- principal- and artifact-bound approval;
- Evidence subject, producer, freshness, validity, and privacy lifecycle;
- explicit uncertain-execution reconciliation; and
- protocol version negotiation, Workflow draining, migration, and rollback.
