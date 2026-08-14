# ADR 0010: Make Policy the Default Path and Machine Assurance Optional

## Status

Accepted

## Date

2026-08-14

## Context

OAW started as portable workflow policy. Later versions added verified Provider
instances, Profile compilation, a durable Workflow Coordinator, and a Codex
Bridge. These machine-backed capabilities can add useful evidence and
coordination, but they also introduced a false architectural dependency: normal
Policy work appeared unavailable unless machine Provider evidence existed.

The 0.1.0 refactor corrected that failure. At baseline `9d2db4b`:

- `SP-FULL`, `MATT-FULL`, `ECC-FULL`, and `MATT-SP-HYBRID` are selectable and
  Host-routable through the Codex Policy CLI without Bridge;
- the Policy catalog, route adapter, reducer, Engagement, and store have no
  transitive dependency on Core, Coordinator, Registry, discovery, integrity,
  or Bridge packages;
- cross-projection tests keep the four aliases, lifecycle responsibilities,
  gates, incidents, macro credits, and stable boundaries aligned;
- machine-backed code remains in the same `oaw` binary and is therefore
  operationally optional, not yet a separately deployed product.

The previous uncommitted ADR 0010 mixed this validated separation with
unimplemented requirements for assurance transitions, route provenance,
reconfirmation, and runtime ownership conflict checks. It also marked itself
Accepted without a completed architecture review. That draft is historical
input, not a decision record.

## Decision Drivers

- Ordinary engineering work must remain usable without Bridge installation.
- Optional assurance must add truthful guarantees without vetoing a valid
  cooperative route.
- Mandatory product complexity must be proportional to common user value.
- The Host must retain all physical execution authority.
- Policy and machine projections must share lifecycle semantics without sharing
  authority records or runtime dependencies.
- Existing machine-backed capabilities should be retained only where they have
  a concrete coordination or audit use case.
- No new compatibility layer should preserve a pre-release abstraction that
  weakens these boundaries.

## Considered Options

### Option 1: Require the machine path for every Workflow

Core, Coordinator, and a Host-native Bridge would gate all Profile use.

This provides one control path, but repeats the failure already observed with
Matt, ECC, and the Hybrid: inability to prove a machine Binding becomes
inability to perform otherwise routable work. It also makes Bridge availability
part of normal business continuity even though Bridge cannot physically contain
the Host.

### Option 2: Remove Core, Coordinator, and Bridge now

OAW would become only a Policy distributor and cooperative lifecycle engine.

This is the smallest product, but it immediately discards implemented and
tested capabilities for immutable workflow revisions, idempotency, cooperating
client leases, evidence references, and Host-native observations before their
actual demand is measured.

### Option 3: Policy-first product with an optional machine-assurance extension

The Policy path is the default complete workflow. Core, Coordinator, and Bridge
form a separately selected assurance extension for use cases that need their
specific guarantees.

This is the proposed option.

### Option 4: Split Policy and machine assurance into separate binaries now

This would make deployment optionality physical and reduce the default binary's
dependency surface. It also introduces packaging, version compatibility, and
support costs before the machine extension's demand and stable public contract
are known. Defer this option until usage evidence justifies it.

## Proposed Decision

OAW has one default path and one optional extension:

```text
Required base
  Distribution -> Policy Engine -> Agent Host
                      |
                      +-- optional explicit assurance selection
                              -> Core
                              -> optional Coordinator
                              -> optional Host Bridge
                              -> Agent Host
```

### Required base

The required base consists of installation management, the canonical Policy,
the Policy Profile catalog, Host route inspection, lifecycle reduction,
project-scoped cooperative state, and the Agent Host.

The base supports the complete `policy-cooperative` `CURRENT` lifecycle. It
does not claim verified Provider identity, Lifecycle Bundles, Capability
Grants, Resource Leases, Host Receipts, atomic revisions, idempotency, or
enforced recovery.

OAW Core is not a prerequisite for this path.

### Optional machine assurance

Core is required only after a caller explicitly selects a machine-backed
assurance level. Core owns verified Provider resolution and Bundle compilation.
The Coordinator remains optional and owns durable logical Workflow State for
cooperating clients. A Host Bridge remains an optional Host integration that
can report current-session facts and carry protocol records.

The Bridge is not a sandbox, hypervisor, process supervisor, or operating-system
enforcement boundary. Its value is limited to evidence, admission, and logical
coordination supported by a cooperating Host.

### Non-veto rule

Machine evidence may admit or deny machine-backed claims. It must not change
whether a Policy Profile is defined or whether its observed Host routes are
cooperatively routable.

A machine-path failure returns a machine-path diagnostic. It does not silently
downgrade, upgrade, or terminate an independent Policy Engagement.

### Engagement boundaries

The 0.1.x product does not support in-place transitions between assurance
levels. If users need to change assurance, they finish or stop the current
Engagement at an understood boundary and explicitly start a new one. A future
transition protocol requires its own use case and decision.

### Route drift and provenance

Policy events continue to re-inspect route inventory and reject route-dependent
progress after material route drift. Recovery uses the current explicit
restart, stop, uncertain, or stable switch operations. No additional generic
"reconfirm" state is required without evidence that those operations are
insufficient.

Policy routing needs route name, invocation mode, missing routes, and
conditional incident availability. Provider provenance and integrity remain
machine-assurance concerns. Public cooperative output must not imply provenance
that the Host did not attest.

### Ownership validation

Built-in ownership is validated by the Policy catalog and cross-projection
contract tests. A new runtime conflict subsystem is not required for the fixed
built-in Profiles. User-defined Profile ownership requires a separate contract
before it becomes part of the Policy CLI.

### Complexity budget

No new Bridge, Coordinator, or machine-schema feature should be added without a
named user workflow that needs its specific guarantee and cannot be served by
Policy plus Git/CI/Host controls. Maintenance cost, protocol churn, and support
surface are part of that decision.

## Consequences

### Positive

- No-Bridge business continuity is an architectural invariant.
- Strict machine evidence can remain strict without creating Policy false
  negatives.
- The common path has a smaller conceptual model.
- Existing machine capabilities remain available while their value is measured.
- Unimplemented requirements from the old ADR no longer masquerade as accepted
  release obligations.

### Negative

- The source tree and release binary still carry two substantial execution
  projections.
- Shared lifecycle semantics require explicit cross-projection tests.
- Users must understand that Policy progress is cooperative, not enforced.
- Machine assurance remains expensive to maintain even when most users do not
  select it.

### Risks and Mitigations

- **Semantic drift:** keep the existing cross-projection contract tests.
- **Machine claims leak into Policy:** keep transitive dependency guards and
  reserved-authority serialization tests.
- **Optional features grow without users:** require a named use case and
  verification budget for each extension.
- **One binary obscures optionality:** measure binary and maintenance cost;
  reconsider a split only with usage evidence and a stable extension contract.

## Acceptance Criteria

This ADR may move to Accepted only after maintainers agree that:

1. Policy is the default complete path, not a degraded fallback.
2. Core is required only for machine-backed assurance.
3. Coordinator and Bridge remain optional and cannot veto Policy routing.
4. No in-place assurance transition is promised in 0.1.x.
5. Bridge is described as cooperative evidence and coordination, not physical
   enforcement.
6. Canonical English and Chinese documentation uses the same boundaries.

## Deferred Questions

- Should the machine extension remain in the default binary after 0.1.x?
- Which real user workflow justifies Coordinator-backed operation?
- Which real user workflow justifies installing Bridge rather than relying on
  Host, Git, and CI controls?
- Is a second Host-native Bridge worth its maintenance cost?
- What adoption or maintenance threshold should trigger extension removal?

## Related Decisions

- Refines ADR 0001's Provider-neutral arbitration boundary.
- Preserves ADR 0009's separation of Core, coordination, and Host execution,
  while narrowing "Core is required" to machine-backed assurance.
- Does not supersede ADR 0009 unless accepted.
