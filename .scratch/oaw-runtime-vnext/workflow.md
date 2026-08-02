# OAW Runtime vNext Policy-Plane Design Tracker

This is a manual pre-Runtime tracker for the currently selected design
lifecycle. It is not Runtime State, is not parsed by the future Runtime, and
will remain only a project projection or legacy provenance record. The selected
Policy profile maps to the vNext `oaw/reliable-feature` compatibility Recipe.

```yaml
task: oaw-agent-engineering-runtime-design
classification: complex-architecture-workflow
profile: MATT-SP-HYBRID
selection_source: user-selected
bundle: MATT-SP-HYBRID
vnext_recipe: oaw/reliable-feature
add_ons: []
current_stage: completed
active_ticket: 12-go-install-rendering-and-state-parity
domain_glossary: CONTEXT.md
spec: .scratch/oaw-runtime-vnext/spec.md
architecture_decisions:
  - docs/adr/0003-add-optional-capability-admission-runtime.md
  - docs/adr/0004-implement-runtime-core-in-go.md
implementation_plans:
  - docs/superpowers/plans/2026-08-01-oaw-runtime-vnext-01-contracts-and-builtin-catalog.md
  - docs/superpowers/plans/2026-08-01-oaw-runtime-vnext-02-configuration-trust-and-provider-discovery.md
  - docs/superpowers/plans/2026-08-01-oaw-runtime-vnext-03-deterministic-request-classifier.md
  - docs/superpowers/plans/2026-08-01-oaw-runtime-vnext-04-profile-recipe-compiler.md
  - docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-05-direct-runtime-vertical-slice.md
  - docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-06-bounded-admission-and-dispatch.md
  - docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-07-workflow-runtime-orchestration.md
  - docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-08-host-conformance-and-capability-audit.md
  - docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-11-go-check-black-box-parity.md
  - docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-12-go-install-rendering-state-parity.md
review_evidence: .scratch/oaw-runtime-vnext/evidence/review.md
verification_evidence: .scratch/oaw-runtime-vnext/evidence/verification.md
requirements_owner: matt
domain_owner: matt
specification_owner: matt
planning_owner: superpowers
implementation_owner: superpowers
tdd_owner: matt
functional_debug_owner: matt
review_owner: superpowers
verification_owner: superpowers
completion_owner: superpowers
```

## Approved Product Decisions

- Preserve a portable Policy Plane and add an optional Runtime Plane.
- Enforce Capability admission, dispatch, and control transitions without
  replacing Host filesystem, process, Git, network, or credential permissions.
- Use one Runtime Core with `oaw run` as the reference entrypoint and Host
  Adapters as alternate clients of the same Runtime Protocol.
- Classify requests as `DIRECT`, `BOUNDED`, or `WORKFLOW`; keep Workflow
  Complexity and Risk Class orthogonal.
- Automatically classify Direct and Bounded request modes without a blocking
  Profile choice; Bounded Capability selection remains explicit or governed by
  an exact user-trusted rule. Run the Startup Gate only for Workflow Mode.
- Model Providers as extensible Capability sources and Profiles as versioned,
  Capability-backed recipes.
- Ship built-in Provider Descriptors for Superpowers, Matt, and ECC without
  treating those Providers as a closed registry.
- Preserve `ECC-FULL` as a complete ECC-owned lifecycle mapped to the built-in
  `oaw/ecc-engineering` Recipe. ECC may instead act as a bounded specialist when
  a different selected Recipe assigns it only that role.
- Determine full-family eligibility from verified lifecycle Capability coverage,
  using the same rule for built-in and user-registered Providers.
- Use dual-track Provider registration: built-in dynamic discovery and
  user-trusted third-party descriptors.
- Merge built-in, user, and trusted-project configuration without allowing the
  project layer to grant trust or expand authority.
- Keep Provider Descriptors inert and declarative; do not load executable
  Provider plugins in Runtime v1.
- Compile Profile Recipes into a canonical control graph with typed Procedures,
  Incident Handlers, Checkpoints, loops, and terminal gates.
- Store authoritative Runtime State under XDG and generate project workflow
  documents only as downstream projections.
- Resolve Host guarantees from pinned built-in or user-trusted integration
  records; per-run Host frames may narrow but never self-enable features.
- Scope Resource Lease guarantees to Runtime-admitted Capability invocations;
  Direct work remains outside Runtime admission.
- Persist Grant issuance and Host dispatch preparation before authorizing any
  external Capability invocation.
- Use a unified modular Go Runtime and migrate the Bash installer only after
  black-box behavior parity.

## Stage Status

The canonical written specification was approved by the user on 2026-08-01.
Independent written-spec findings were remediated, and fresh documentation plus
full Bash regression verification passed. Matt ticket decomposition was approved
by the user on 2026-08-01. Ticket 01 completed in the isolated worktree with
fresh Go race, coverage, documentation, and full Bash verification. After
reviewing delegated Task 1 execution, the user directed subsequent work to
continue inline without additional subagents. Tickets 01 through 07 are
complete on the implementation branches. Ticket 08 passed final inline review
and the full verification matrix on 2026-08-02 at implementation fixed point
`60ea7ee`. Ticket 11 passed final inline review and the full Bash/Go parity,
race, coverage, fuzz, cross-platform build, and vulnerability matrix on
2026-08-02 at implementation fixed point `07f1552`. Bash remains the
authoritative management interface, while `oaw check` remains a
non-authoritative shadow path. Ticket 12 passed final two-axis inline review
and the complete rendering, state, mutating parity, race, coverage, fuzz,
cross-platform build, and vulnerability matrix on 2026-08-02 at implementation
fixed point `6f15694`. Its internal Go install driver remains parity-only;
public `oaw install` is not enabled. Built-in Host records remain
instruction-only, and no first production Runtime Host was selected.

Every unfinished published ticket carries Matt's `ready-for-agent` triage
status, which means the ticket is specified for agent execution; completed
tickets are marked `completed`. Scheduling still honors `Blocked by` edges;
completion of Ticket 12 unblocks Ticket 13 without changing its independent
scheduling or selection gates.
