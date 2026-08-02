# OAW Runtime vNext Written-Spec Review

**Date:** 2026-08-01
**Scope:** `CONTEXT.md`, the Runtime vNext specification and design tracker,
ADR 0003, and ADR 0004
**Review owner:** Superpowers
**Result:** Ready for user written-spec review; implementation planning has not
started

## Review Method

Two independent read-only reviews checked the approved design against the
request model, Provider extensibility requirements, Runtime authority boundary,
migration constraints, glossary consistency, and ADR consequences. Subsequent
user written-spec review corrected the `ECC-FULL` capability assumption. A final
self-review checked the resulting schema vocabulary and control-flow ordering.

No Critical findings were reported. All Important findings were corrected
before this evidence was recorded.

## Findings and Dispositions

| Finding | Disposition |
| --- | --- |
| Executor logical identity conflicted with required Workflow context isolation. | The glossary now separates authority bookkeeping from physical context isolation, and the spec requires a trusted conforming Host integration. |
| Read-only `INSPECT` appeared to create a Run Revision. | Only mutating `START` and `CONTINUE` exchanges commit transitions; `INSPECT` reads a committed snapshot. |
| Bounded Mode did not define Capability selection or Main Agent execution. | Mode classification no longer implies Capability selection; exact user intent or a user-trusted rule is required, ambiguity pauses, and `executor_topology` controls Main Agent eligibility. |
| Host frames could self-attest isolation, deduplication, and cancellation features. | `START` references a pinned built-in or user-trusted Host integration record; per-run declarations may only narrow its Manifest. |
| Resource Lease wording overclaimed protection for Direct work. | Lease guarantees now cover only Runtime-admitted write-capable Capability invocations; Direct is explicitly outside that guarantee. |
| External dispatch crash windows were underspecified. | The protocol now persists Grant issuance, requires durable Host preparation, commits `DISPATCH_AUTHORIZED`, and treats post-authorization ambiguity as `EXECUTION_UNCERTAIN`. |
| Child Capability narrowing relied on an undefined Capability hierarchy. | Parent Grants now contain a closed delegation allow-list; child effects, resources, and onward delegation must also narrow. |
| Pre-parity Go migration language could imply early authority. | Go is explicitly non-authoritative shadow code until command-level Bash parity. |
| The manual tracker could be mistaken for future authoritative Runtime State. | It is now labeled a Policy-Plane design tracker and declared non-authoritative, one-way provenance. |
| The draft incorrectly inferred that ECC's specialist strengths prevented it from owning a complete lifecycle. | Restored `ECC-FULL` as an explicit alias to `oaw/ecc-engineering`; kept `oaw/hardening` as a separate composed Recipe. Eligibility now depends on verified lifecycle Capability coverage, as it does for every Provider. |
| The first real Host and conformance timing were unclear. | Host promotion is deferred to the Host-audit migration phase after official-capability audit and conformance. |

## Final Assessment

The corrected specification consistently models OAW as a dual-plane engineering
runtime: Policy remains portable; Runtime admits Capabilities and transitions
without claiming Host sandbox authority. Provider discovery is extensible and
declarative, Profile Recipes are user-configurable control graphs, Workflow
selection is explicit, and built-in Superpowers/Matt/ECC support is ordinary
catalog data plus OAW Recipes. ECC can own `oaw/ecc-engineering` as a complete
lifecycle or provide bounded Capabilities to another Recipe; neither role is
inferred from its comparative strengths.

The design is ready for user review. It is not yet an executable implementation
plan and authorizes no Go or Policy vNext implementation work.

## Ticket 03 Implementation Review

**Date:** 2026-08-01

**Fixed point:** `5104a01`

**Scope:** Deterministic Request Classifier

**Result:** Passed after scoped corrections

The implementation diff was reviewed separately against repository standards
and Ticket 03's issue, Runtime specification, and executable plan. The review
found no unresolved Critical or Important issues.

Corrections made before the final result:

- Bounded selector evidence no longer raises an otherwise valid Bounded request
  to Workflow; it remains an admission requirement.
- User and project policy layers are composed before application, making mode,
  risk, evidence, reasons, and decision digests order independent and monotonic.
- Typed proposals now enforce the same collection and reference limits as the
  JSON Schema, and raw proposal bytes must be valid UTF-8.
- Classification and validation were split into focused functions; unused
  Request Mode aliases and duplicated selector handling were removed.
- Exhaustive policy invariants and a fails-closed fuzz seam were added alongside
  the critical-release corpus.

The final diff does not parse request prose, call a model or network API, select
a Provider or Capability, issue authority, mutate Runtime State, or add a CLI
surface. The only selector retained in a decision is explicit Bounded intent;
Workflow decisions carry no Capability selector.

## Ticket 04 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `ebc04a8`

**Scope:** Profile Recipe Compiler and deterministic Execution Graphs

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed

The diff was checked against Ticket 04, specification section 9, and the
catalog/registry contracts. The compiler accepts a read-only `CatalogSource`
and `EffectiveRegistry`, resolves aliases and custom Recipes through one path,
and never discovers or executes Providers. Nodes are admitted only when the
queried Provider Instance and Capability identities match and the verified Host
Binding is declared by the selected descriptor. Provider brand names do not
participate in eligibility.

The graph is defensively immutable and pins normalized Recipe/Provider digests,
Capability contract limits, Host Binding, executor topology, Procedures,
Incident Routes, terminal gates, and stable boundaries. Binding duplicates,
unknown selectors, missing verification, identity mismatch, duplicate/missing
owners, unsupported effects/resources, unsupported Request Modes, missing
targets, unreachable controls, invalid Procedures/terminals, and unclosed loops
fail with stable compiler codes. Optional missing handlers are removed together
with their routes; required nodes are never silently omitted.

No Critical or Important findings remain. The only intentional non-blocking
verification limitation is that `govulncheck` is not installed in the current
environment.

## Ticket 05 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `7a7d10b`

**Scope:** Direct Runtime protocol, durable Run journal, and Direct escalation

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped corrections

The complete diff was checked against Ticket 05, specification sections 5,
10-12, 14-15, and 18, and the executable implementation plan. No unresolved
Critical or Important issues remain.

Corrections made during review:

- START replay now consults committed state before applying current
  classification rules, so a rule change cannot replace an idempotently stored
  reply.
- Matching orphan revisions can be promoted after a crash before `HEAD`, while
  valid but logically conflicting or malformed orphans fail closed.
- Loaded Direct state now validates classification, project identity,
  processed-message ordering and revision ownership, empty authority
  collections, event/reply shape, size limits, and every pinned digest.
- Windows `HEAD` replacement uses `MoveFileEx` with replace and write-through
  flags; Unix uses atomic rename and directory sync.
- The commit path strictly reloads the immutable revision after advancing
  `HEAD`, making persisted bytes the only reply source.

The implementation creates no Lifecycle Bundle, Capability Grant, Stage Grant,
Resource Lease, Provider invocation, Profile selection, Host dispatch, or CLI
transport. Scope expansion preserves `DIRECT`/`RELEASED` and returns only the
successor-Run recovery action. Direct release diagnostics explicitly disclaim
Capability admission, Host tool-call control, and Resource Lease guarantees.

## Ticket 06 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `9ecdac8`

**Scope:** Bounded admission, immutable Grant issuance, dispatch handshake,
observations, recovery, and journal hardening

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

The review checked selector provenance, verified Provider and Binding
resolution, authority/effect/resource narrowing, Main Agent topology, Resource
Lease deferral, immutable Grant identities, reply-before-return ordering,
replay and restart behavior, Host invocation boundaries, observation
normalization, blind retry prevention, future authority-field leakage, and
Direct compatibility.

One Important finding was corrected before completion: single-revision
semantic validation allowed a later re-signed revision to rewrite an earlier
Grant or processed-message history. `validateRevisionTransition` now enforces
immutable Run identity, append-only message history, legal Bounded state edges,
and immutable Grants/observations while loading the committed chain.

No unresolved Critical or Important findings remain. Runtime never invokes a
Host Binding; it persists `DISPATCH_AUTHORIZED` before any external Host may
act. Ticket 07 Resource Leases and Ticket 08 Host Manifest/Adapter fields remain
outside the Runtime state surface.

## Ticket 07 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `8665458`

**Scope:** Workflow Profile selection, immutable Lifecycle Bundles, isolated
Stage Grants, Worktree leases, graph observations, stable-boundary switching,
one-way projections, restart recovery, and Direct/Bounded compatibility

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

The review checked Workflow-only Gate enforcement, Bundle and graph pinning,
Provider invocation boundaries, Main Agent and Host-isolation rejection,
generation-bound Grants, review Executor freshness, cross-Run lease
serialization, switch timing, projection authority, journal recovery,
permissions, and credential/raw-output persistence.

Three findings were corrected before completion:

- Projection lag references were removed from authoritative `WorkflowState`;
  failures now exist only as immutable owner-only sidecars, and Runtime never
  reads them as authority.
- Built-in Superpowers, Matt, and ECC completion capabilities now declare both
  `git-repository` and `project-worktree` resources when allowing `git-local`,
  closing the Grant resource contract.
- Explicit stable-boundary switching can now adopt the current trusted
  Configuration and Registry in the new Bundle generation. Run and Project
  identity and every old Bundle remain immutable; old Engines cannot issue new
  Grants after the switch, while an Engine holding the current trusted inputs
  can continue.

No unresolved Critical, High, or Important findings remain. Runtime invokes no
Host Binding, projection sink failures cannot change a committed reply, and
projection deletion or tampering cannot influence inspection, admission,
recovery, or Bundle switching.

## Ticket 08 Implementation Review

**Date:** 2026-08-02

**Fixed point:** `60ea7ee`

**Scope:** Trusted Host Integration records, capability audit evidence,
deterministic Adapter conformance, Configuration pinning, Workflow admission,
Lifecycle Bundle Host identities, stable switching, projection summaries, and
security/recovery integration coverage

**Review owner:** Superpowers (inline main-agent review; no subagents)

**Result:** Passed after scoped remediation

The complete `main...HEAD` diff was checked for self-attested Features,
project-granted Host trust, forged Manifest/Audit/Report digests,
instruction-only promotion, incomplete Feature/check matrices, Runtime Adapter
invocation, Binding substitution, stale Integration reuse, unsafe Bundle
switching, mutable record leakage, transcript/raw-output leakage, unbounded
inputs, unstable reason codes, Direct/Bounded regressions, and premature first
Host selection.

One Important finding was corrected before completion: a Manifest could declare
multiple Binding kinds while the conformance harness exercised only the first
sorted kind. The harness now performs two deterministic invocations for every
declared kind, verifies exact Binding delivery, evidence normalization, Bundle
inheritance, deduplication, and native receipts across the complete declared
surface, and hashes the redacted multi-Binding transcript. A RED regression
test proved that substituting only a non-first Binding kind was previously
missed and is now rejected.

No unresolved Critical, High, or Important findings remain. User configuration
is the only external Host trust source; project configuration cannot register
one. Runtime consumes immutable records and Reports but never holds or invokes
a `ConformanceAdapter`. Old Bundle generations remain byte-for-byte immutable,
and a Host change is admitted only in a new stable-boundary generation. The
nine built-in integrations remain instruction-only, so Ticket 08 selects no
first production Runtime Host.
