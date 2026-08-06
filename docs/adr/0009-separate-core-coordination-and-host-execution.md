# ADR 0009: Separate OAW Core, Workflow Coordination, and Host Execution

## Status

Accepted

## Context

ADR 0003 introduced an optional Capability-admission Runtime so OAW could add
durable state, idempotency, Resource Leases, and evidence closure to its
portable Markdown policy. The implementation later combined those valid state
responsibilities with an invalid execution responsibility: `oaw run --host
codex` launched `codex exec`, constructed a private HOME, staged selected
Skills, and filtered Host configuration.

ADR 0007 inverted control back to the active Host and selected `INLINE` and
`NATIVE_SUBAGENT`. ADR 0008 then clarified that the Host, not OAW, owns the
child's environment. Further review showed that continuing to describe OAW as
an execution Runtime remains misleading even after removing the Runner. OAW
can validate its own state transitions, but it cannot invoke or contain the
Host's actual tools and cannot prevent a Host from bypassing the protocol.

The product still has useful deterministic behavior beyond Markdown policy:
dynamic Provider resolution, Profile compilation, durable revisions,
idempotency, cooperative leases, and evidence indexing. Those capabilities
must be retained without implying that OAW executes Agents.

## Decision Drivers

- Make OAW's product identity and enforcement claims match its real authority.
- Preserve portable Policy-only operation.
- Keep deterministic Provider and Profile compilation in a required Core.
- Retain durable coordination for long-lived and concurrent Workflows.
- Keep Agent execution and capability environments inside the selected Host.
- Use short, user-friendly topology names with precise contracts.
- Remove pre-release legacy contracts instead of maintaining compatibility.

## Considered Options

### Option 1: Keep the Runtime name and remove only the Codex Runner

This retains the least code and documentation churn, but callers can still
reasonably interpret Runtime Grants and dispatch as execution control. The
module would continue mixing stateless policy compilation with durable state.

### Option 2: Remove all Runtime and state behavior

This leaves a clean Policy and compiler product. It is sufficient for a single
cooperative Agent session, but loses atomic revisions, idempotency, concurrent
mutation coordination, evidence indexing, pause, and recovery.

### Option 3: Required OAW Core plus optional Workflow Coordinator

OAW Core owns policy decisions and Lifecycle Bundle compilation. An optional
Coordinator owns only durable Workflow State. The active Host owns execution
through the current session or a native Subagent.

This option is selected.

## Decision

OAW is divided into these ownership areas:

1. **OAW Core** is required and stateless. It owns request classification,
   Host-scoped Provider resolution, Profile eligibility, recommendations,
   explicit selection validation, and Lifecycle Bundle compilation.
2. **Workflow Coordinator** is optional. It owns immutable Workflow revisions,
   idempotency, Resource Leases, evidence references, pause, cancellation,
   recovery, and legal lifecycle transitions for cooperating clients.
3. **Agent Host** owns the current Agent, native Subagents, model invocation,
   authentication, tools, extensions, sandboxing, approvals, and all physical
   effects.

The Coordinator depends on Core. A Workflow `START` carries request evidence,
immutable Host snapshots, and the explicit user selection; the Coordinator
invokes Core and atomically commits the returned Lifecycle Bundle. Clients
cannot submit or replace a caller-authored Bundle.

The execution topology enum is replaced with:

- `CURRENT`: the active Agent session executes;
- `SUBAGENT`: the active Host creates a child through its native Subagent
  facility.

`SUBAGENT` is Host-native by contract. OAW never launches a model process as a
fallback. `INLINE`, `NATIVE_SUBAGENT`, `main-agent-allowed`,
`isolated-required`, `runner-managed`, and `native-managed` are removed without
aliases.

`DIRECT` and `BOUNDED` do not create Coordinator state. Only `WORKFLOW` uses a
durable Lifecycle Bundle and transition graph. Policy-only Hosts may execute
the same lifecycle cooperatively but cannot claim Coordinator guarantees.

The public `oaw run --host codex` command is deleted. The optional state
transport becomes `oaw workflow exchange`; `oaw runtime exchange` is deleted
without an alias. The `internal/runtime` package is removed, and retained
Workflow state implementation moves to `internal/coordinator`.

All affected schemas receive new versions. Old development state is reset
explicitly; OAW supplies no migration, dual reader, or compatibility shim.

## Consequences

### Positive

- OAW's interface states exactly what it controls.
- The selected Host environment remains intact under `CURRENT`.
- Context separation uses the Host's real Subagent facility.
- Policy-only operation remains lightweight and portable.
- Long-lived Workflows retain deterministic state and recovery when the
  Coordinator is used.
- Provider discovery and Profile compilation remain independent of execution
  topology and Provider brand.
- The public terminology is shorter and easier to explain.

### Negative

- OAW cannot provide context separation on a Host without native Subagents.
- Policy-only execution remains cooperative rather than machine-enforced.
- Host integrations must actively call Core and Coordinator interfaces to gain
  deterministic guarantees.
- The hard cutover requires replacement of schemas, tests, documentation, and
  pre-release state.
- Bounded operations no longer receive durable Runtime dispatch tracking.

### Risks

- A Host may bypass the Coordinator or report inaccurate session facts.
  Mitigation: state only cooperating-client guarantees, validate reports, and
  treat missing evidence as unknown.
- Users may interpret Resource Leases as filesystem locks. Mitigation: describe
  them as Workflow coordination, not physical containment.
- Removing Bounded persistence may expose duplicated Host invocation.
  Mitigation: Bounded execution remains Host-owned; workflows requiring durable
  recovery must classify as `WORKFLOW`.
- Historical documentation may be mistaken for current behavior. Mitigation:
  mark superseded ADRs and rewrite every canonical product document in the
  cutover.

## Implementation Notes

- Preserve existing classification, discovery, Registry, Profile compiler,
  canonical JSON, journal integrity, revision, lease, and evidence algorithms
  where they fit the new ownership model.
- Delete the Codex Runner, private execution profile, process output parser,
  pinned Runtime entrypoint, model-process Host Driver, and Runner-only
  conformance assets.
- Replace Host integration levels with `policy` and `host-native` surfaces.
- Replace singular executor requirements with topology sets and Host binding
  topology sets.
- Keep historical ADRs and completed plans as history; do not parse or expose
  them as active contracts.

## Related Decisions

- Supersedes ADR 0003's execution Runtime model while retaining optional
  durable coordination.
- Keeps ADR 0004's decision to implement the OAW binary and state logic in Go.
- Supersedes ADR 0007's topology names while retaining Host-native control
  inversion and no-process-fallback behavior.
- Incorporates ADR 0008's Host-owned environment semantics.
- The complete design is
  `docs/superpowers/specs/2026-08-05-oaw-core-coordinator-hard-cutover-design.md`.
