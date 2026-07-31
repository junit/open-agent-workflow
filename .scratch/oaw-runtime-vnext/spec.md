# OAW Runtime vNext Specification

**Status:** Approved design, awaiting written-spec review
**Date:** 2026-08-01
**Lifecycle:** `MATT-SP-HYBRID`
**Canonical glossary:** `CONTEXT.md`

## 1. Summary

OAW vNext preserves the portable provider-neutral Policy Plane and adds an
optional Runtime Plane. The Runtime classifies engineering requests, resolves
trusted Provider Capabilities, compiles user-selected Profile Recipes, admits
exact Capability invocations, persists authoritative run state, validates
control transitions, coordinates isolated Executors, and records evidence.

OAW does not load Provider code, install engineering-skill Providers, replace a
Host sandbox, or claim that instruction-only integrations provide mechanical
enforcement.

The reference implementation is a modular Go CLI. `oaw run` is the reference
Runtime entrypoint, and native Host integrations use the same Runtime Protocol.
Existing Bash installation behavior remains authoritative until Go reaches
black-box parity command by command.

## 2. Goals

- Give small, bounded requests a low-friction execution path.
- Run a blocking Profile-selection Gate only for lifecycle work.
- Separate Request Mode, Workflow Complexity, and Risk Class.
- Treat Superpowers, Matt, and ECC as built-in Providers in an extensible
  Capability registry, not as a closed enumeration.
- Let users register inert third-party Provider Descriptors and create custom
  Capability-backed Profiles.
- Discover built-in Provider installations dynamically and deterministically.
- Execute Runtime-managed Workflows in a context isolated from the Main Agent.
- Enforce one owner per applicable responsibility and one active mutating
  Executor per physical Worktree.
- Preserve exact Provider instances, configuration, Profile, control graph,
  Grants, transitions, and evidence for each Engineering Run.
- Recover safely from process crashes and uncertain external invocations.
- Retain Policy-only compatibility for Hosts without Runtime integration.

## 3. Non-Goals

- Installing, updating, licensing, or removing workflow Providers.
- Executing arbitrary code from Provider Descriptors or project configuration.
- Intercepting every filesystem, Shell, Git, browser, network, or credential
  operation performed by a Host.
- Preventing a non-cooperating Host from bypassing OAW.
- Shipping an executable Provider-plugin system in Runtime v1.
- Requiring a daemon, local socket, web service, or model API in Runtime v1.
- Loading remote Descriptor, Recipe, or Provider content.
- Inferring Runtime State from existing `.scratch` documents.
- Selecting the first Runtime Host without a fresh official-capability audit and
  Adapter conformance proof.

## 4. Product Architecture

```text
                         OAW Application Core
                    /                            \
         Management Interface              Runtime Protocol
                 |                                |
       install/check/catalog           oaw run / Host Adapter
                 |                                |
           Policy Plane                     Runtime Plane
```

The Policy Plane is the canonical instruction surface and portable fallback.
The Runtime Plane is optional and provides authoritative state and Capability
admission. Both use the same domain language, built-in Catalog, Profile
semantics, and safety rules.

The Management Interface handles installation, catalog diagnostics, Profile
validation, and Project Trust. Each mutating Runtime Protocol exchange drives
one durable control transition; read-only inspection creates no transition.
Installation and execution are distinct external seams over shared internal
configuration, registry, trust, and schema modules.

## 5. Request Model

Every Engineering Request receives three orthogonal decisions:

```yaml
request_mode: DIRECT | BOUNDED | WORKFLOW
workflow_complexity: ordinary | complex | null
risk_class: normal | elevated | critical
```

### 5.1 Direct Mode

Direct Mode requires clear requirements, a known change point, bounded and
recoverable scope, no architecture or domain decision, no public contract,
schema, dependency, security, data, or deployment semantic change, and a known
focused verification command. Direct execution creates no Lifecycle Bundle and
invokes no engineering Provider Capability.

The Runtime returns a classification record and releases execution to the Host.
It does not claim to control subsequent Host tool calls. Scope expansion relies
on Policy Plane instructions and the Main Agent stopping to request escalation.
Because Direct work is outside Capability admission, OAW Resource Leases do not
govern it. Hosts may provide their own Direct-execution concurrency control, but
must not describe that as an OAW Runtime guarantee.

### 5.2 Bounded Mode

Bounded Mode admits one verified Capability for one observable deliverable with
declared effects, resource scope, and termination conditions. It can be
read-only or permit limited writes. It cannot claim a Canonical Phase, general
review, remediation loop, Git completion, or lifecycle ownership.

Automatic Request Mode classification never selects a Capability or Provider.
A top-level Bounded request must carry an exact Capability selector attributable
to explicit user intent or match one user-trusted default rule. Runtime resolves
that selector to exactly one verified Provider Instance. A missing or ambiguous
selector returns `CAPABILITY_SELECTION_REQUIRED`; this bounded choice is not the
Workflow Startup Gate.

A Bounded Grant may name the Main Agent as Executor only when the Capability
declares `executor_topology = "main-agent-allowed"`, its effects fit Bounded
limits, and it acquires no lifecycle responsibility. Otherwise the Host must
create the required separate Executor or Runtime denies the invocation and
offers escalation.

Any required second Capability, remediation loop, architectural decision, or
scope expansion pauses with `MODE_ESCALATION_REQUIRED`.

### 5.3 Workflow Mode

Workflow Mode applies when requirements, root cause, domain behavior, or
architecture remain unresolved; multiple engineering responsibilities interact;
public contracts, schemas, dependencies, migrations, sensitive mutations,
multiple tickets, or long-lived delegation are involved; or a lower mode
escalates.

Only Workflow Mode runs the Startup Gate. Workflow Complexity controls
recommendation and planning depth, not Gate activation.

### 5.4 Mode Escalation

Request Mode is immutable for one Engineering Run. Explicit escalation creates
a Successor Run that references predecessor evidence without inheriting its
authority. Workflow Mode never silently downgrades.

## 6. Two-Layer Classification

Go Runtime does not parse arbitrary natural language with keyword rules or call
a model API. A Host Semantic Classifier submits an untrusted, evidence-backed
Classification Proposal using a closed trait vocabulary. The deterministic
Runtime Policy Classifier validates the proposal and applies built-in, user, and
trusted-project minimum-mode, minimum-risk, protected-resource, and evidence
rules.

Unknown or missing critical traits fail upward to Workflow. User and project
rules may only raise mode, risk, or evidence requirements. Project rules cannot
weaken built-in or user rules.

If a Host cannot produce a valid Proposal, the fallback is Workflow/complex
with reason `CLASSIFICATION_UNAVAILABLE`. It never receives automatic Direct.

The Runtime Protocol accepts a Proposal with `START`, or returns
`CLASSIFICATION_REQUIRED`. Pre-classification work is limited to Governance and
read-only Utility operations; it cannot invoke Provider lifecycle Capabilities.

## 7. Provider and Capability Model

OAW ships inert Provider Descriptors for:

```text
oaw/superpowers
oaw/matt
oaw/ecc
```

These are built-in integration records, not bundled Provider content. Third
parties use qualified IDs such as `acme/engineering-suite`.

A Provider Descriptor declares identity, schema and descriptor versions,
declarative discovery probes, Capability declarations, maximum effects, and
Host Bindings. It cannot contain commands, scripts, executable templates,
environment interpolation, or arbitrary expressions.

Each Capability declaration includes input and outcome schemas, maximum effects
and resources, permitted Request Modes, lifecycle responsibilities,
`executor_topology` (`main-agent-allowed` or `isolated-required`), a closed
delegation allow-list, and compatible Host Bindings. Admission validates these
fields before issuing a Grant.

Discovery produces evidence. Verification produces a Provider Instance pinned
to exact descriptor, location, version, configuration, binding, and evidence
digests. Detection never selects a Provider or Profile.

Provider states are `not-found`, `candidate`, `verified`, `ambiguous`,
`incompatible`, `binding-unavailable`, `disabled`, and `untrusted`. Only a
verified instance enters the Effective Registry.

## 8. Configuration and Trust

Human-authored configuration and descriptors use TOML. Runtime normalizes them
to Canonical JSON for validation, hashing, state, and protocol use.

```text
Built-in Catalog
      +
User Config, Trust, Pins, Descriptors, and Profiles
      +
Trusted Project Requirements, Descriptors, and Profiles
      =
Effective Configuration Snapshot
```

Configuration merges immutable whole records. Duplicate replacement must be
explicit; user deny always wins. The built-in `oaw/*` namespace is reserved.

Project Trust pins a physical project root, canonical project-config digest,
and referenced project Descriptor and Recipe digests. A content change revokes
trust. Project configuration can request, recommend, or narrow but cannot enable
a user-denied Provider, grant authority, or introduce executable probes.

An active Lifecycle Bundle uses an immutable Configuration Snapshot and is not
changed by later disk configuration. A stable-boundary switch may adopt current
configuration only through explicit user choice.

## 9. Profiles and Execution Graphs

A Profile Recipe is a declarative, versioned control recipe. It binds Canonical
Phases, Procedures, typed Incident Handlers, Checkpoints, terminal gates, and
stable boundaries to Capability selectors. A Profile Binding maps selectors to
preferred Provider Instances without copying the graph.

Compilation resolves exact Provider Instances, validates every applicable
responsibility has exactly one owner, ensures effects remain within all limits,
checks incident and loop closure, and emits a deterministic canonical Execution
Graph digest. The graph may contain remediation and recovery cycles; it is not
required to be a DAG.

The Lifecycle Bundle pins user selection, exact Recipe version and digest,
Provider Instances, Profile Bindings, add-ons, Configuration Snapshot, and
Execution Graph digest. It is immutable. Stable switching creates a new Bundle
generation and revokes outstanding Grants from the prior generation.

### 9.1 Built-in Recipes

- `oaw/delivery`: Superpowers-backed standard feature and delivery lifecycle.
- `oaw/domain-engineering`: Matt-backed complete domain engineering lifecycle,
  available only when a full Matt Capability set is verified.
- `oaw/reliable-feature`: Matt owns requirements, domain, specification,
  ticketing, TDD, and functional debugging; Superpowers owns executable plans,
  workspace, implementation, review, remediation, verification, and completion;
  ECC build/type repair is an optional typed Incident Handler.
- `oaw/hardening`: a composed hardening lifecycle using specification,
  orchestration, ECC specialist handlers/checkpoints, remediation, review, and
  verification. It is not an ECC-only full lifecycle.

Compatibility aliases map `SP-FULL`, `MATT-FULL`, and `MATT-SP-HYBRID` to the
first three Recipes. `ECC-FULL` is deprecated without a silent alias.
`CUSTOM-LOCKED` becomes a UI action for selecting a user-defined Profile, not a
Profile itself.

## 10. Runtime Protocol

The Runtime Core exposes one deep transition interface:

```text
exchange(RunFrame) -> RunReply
```

`START` supplies request identity, optional Classification Proposal, project
identity, a trusted Host Adapter registration reference and pinned Host Manifest
digest, an optional exact Capability selector with user-authority provenance,
user authority ceiling, and optional parent Grant. A per-run Host availability
declaration may narrow the pinned Manifest but cannot add features.
`CONTINUE` supplies a run ID, expected revision, and a closed signal such as a
user decision, dispatch preparation, normalized Capability report, child
request, cancellation, or reconciliation. `INSPECT` provides a read-only state
query.

Replies are `CLASSIFICATION_REQUIRED`, `MODE_DECIDED`,
`CAPABILITY_SELECTION_REQUIRED`, `SELECTION_REQUIRED`, `GRANT_ISSUED`,
`DISPATCH_AUTHORIZED`, `PAUSED`, `FINISHED`, `DENIED`, or `STATE_SNAPSHOT`.

Every mutating `START` or `CONTINUE` exchange computes and durably commits one
transition before emitting its reply. `INSPECT` reads a committed snapshot and
does not create a Run Revision. Exchange never waits for Provider execution.
Host Adapters perform admitted native invocations and return Normalized
Observations. Provider output cannot choose the next graph node.

Runtime v1 transport is one Canonical JSON frame on stdin and one Canonical JSON
reply on stdout. Diagnostics use stderr. Messages include IDs, idempotency keys,
expected revisions, size limits, and strict schema versions.

## 11. State Machine and Grants

```text
RECEIVED -> CLASSIFIED
  DIRECT   -> RELEASED
  BOUNDED  -> AWAITING_CAPABILITY -> READY
  WORKFLOW -> AWAITING_SELECTION -> READY

READY -> GRANTED -> PREPARED -> IN_FLIGHT -> READY | PAUSED | FINISHED
```

Terminal states are `RELEASED`, `FINISHED`, `CANCELLED`, and `DENIED`. `PAUSED`
always contains a stable reason code and permitted recovery actions.

A Capability Grant is immutable, non-transferable, revision-bound, generation-
bound, Executor-bound, and scoped to one logical invocation. A Workflow node
receives a Stage Grant. Child Grants must narrow Capability, effects, resources,
and delegation compared with the parent. Specifically, the child Capability
must appear in the parent's immutable delegation allow-list; its effects,
resource scope, and onward delegation cannot exceed the parent Grant.

Bounded execution may bind a Grant to the Main Agent only under the declaration
and effect constraints in section 5.2. That identity is explicit in the Grant;
it does not turn the Main Agent into a lifecycle owner or permit delegation.

Effective authority is the intersection of user authority, Host integration
ceiling, Request Mode ceiling, Provider Descriptor maximum, Profile-node
maximum, parent Grant, and current graph state. Unknown effects fail closed.
Capability effects do not replace operating-system or Host permissions.

Workflow-managed execution requires a Host execution context physically
separated from the Main Agent. The pinned Host integration record must declare
that isolation and pass the corresponding conformance fixture; a run frame
cannot self-attest it. The Main Agent retains only user communication and
neutral control state. A physical Worktree has at most one active write-capable
Stage Grant; Review uses a fresh read-only Executor by default. Hosts unable to
provide required isolation receive `HOST_ISOLATION_UNAVAILABLE` and may offer
an explicit Policy-only fallback.

Expected RED stays within the TDD Procedure. Functional, build, type,
dependency, and security observations route to distinct handlers or checkpoints
defined by the graph.

## 12. Authoritative State and Recovery

Runtime State lives under the OAW XDG state namespace. Each Engineering Run has
an immutable identity, an atomically replaced `HEAD`, a process lock, and an
immutable sequence of Run Revisions. A Revision contains its predecessor and
content digests, accepted message, event, resulting state, Grant changes,
Evidence References, Configuration digest, and emitted reply.

Each transition acquires the lock, validates HEAD and expected revision,
computes a complete new Revision, writes and syncs it, atomically replaces and
syncs HEAD, releases the lock, and returns the committed reply. A crash leaves
either the prior Revision or the complete new Revision authoritative.

Idempotent message replay returns the stored reply. Reusing an idempotency key
with different content is denied.

The Runtime and an external Host invocation cannot share a transaction. Dispatch
therefore uses an ordered handshake. Runtime first mints the invocation ID,
persists the Grant, and emits `GRANT_ISSUED`. The Host durably records its intent
without invoking and returns `DISPATCH_PREPARED`. Runtime then commits
`IN_FLIGHT` and emits `DISPATCH_AUTHORIZED`; only then may the Host invoke the
Binding with that invocation ID.

A crash before `DISPATCH_AUTHORIZED` is safe to resume without external replay.
After authorization, a missing trustworthy observation pauses the Run as
`EXECUTION_UNCERTAIN`, even when the Host may have crashed before the actual
invocation. Automatic re-dispatch is permitted only when the Binding and Host
deduplicate the same invocation ID, reconciliation confirms the prior outcome
is absent or safely repeatable, and user authority permits retry. Otherwise
explicit reconciliation is required.

Cross-Run Resource Leases protect Runtime-admitted write-capable Capability
invocations that target physical Worktree writes or Git-repository mutation.
Read-only Runs may execute concurrently. Runtime v1 does not attempt fine-grained
parallel file writes.

Project workflow documents are one-way projections of committed Revisions.
Projection failure records lag and never rolls back state. Projections contain
no credentials, full Grants, or sensitive Evidence and are never parsed back as
authority.

## 13. Host Integration

Integration Levels are:

- `instruction-only`: Policy-only, no Runtime guarantee.
- `runner-managed`: `oaw run` uses a built-in Host Driver.
- `native-managed`: a Host Hook, Plugin, or MCP client drives Runtime Protocol.

A Host Manifest declares supported protocols, binding kinds, fresh and child
Executor behavior, native Capability invocation, invocation deduplication,
cancellation, normalized observations, and artifact return. Runtime resolves
the Manifest through a built-in or user-trusted Host Adapter registration whose
identity, conformance status, and Manifest digest are pinned in the
Configuration Snapshot. A Host frame can report temporary unavailability only;
it cannot self-enable a feature or raise its Integration Level.

Runtime-managed Workflow requires isolated Executor creation, exact Binding
invocation, pause behavior, Bundle inheritance, and Evidence return. Missing
features deny Runtime execution; fallback requires explicit user selection.

Runtime v1 loads no third-party executable Host plugins. Third-party Hosts run
independent protocol clients and pass an Adapter Conformance Suite. The first
runner-managed Host is selected during the migration Host-audit phase only after
current official integration behavior is verified and a complete conformance
fixture passes.

## 14. Security Model

Project configuration, Provider files, Host frames, Provider output, Evidence
paths, and projection destinations are untrusted. Descriptor probes use
enumerated safe roots and Go filesystem APIs, never Shell execution. Project
paths are resolved physically, symlink redirection is rejected, and exact
configuration digests gate trust.

Protocol input has strict schema, UTF-8, field, depth, collection, and byte
limits. Runtime validates revision chains, HEAD, Run identity, active node,
Grants, and Resource Leases on load. XDG Runtime directories and files use
owner-only permissions where the platform supports them.

Remote mutation, credentials, paid actions, destructive writes, publishing,
push, and merge require explicit User Authority. A Recipe or project cannot
grant them. Provider raw output is normalized through a closed outcome
vocabulary and terminal control characters are escaped in diagnostics.

Risk Class may require security acceptance criteria, negative tests, named
checkpoints, remediation, re-review, and final evidence. These constraints do
not select ECC or any other Provider.

OAW does not claim to stop a bypassing Host, replace a sandbox, verify Provider
internals, or undo completed external side effects. Policy-only Mode never
claims Runtime admission or physical isolation.

## 15. Go Implementation

The repository gains a Go module with domain-oriented internal packages for
management, configuration, trust, registry, classification, recipe compilation,
workflow transitions, admission, run state, projections, Host integration, and
installation. Runtime Protocol types and schemas form the only public stable Go
surface.

Protocol DTOs are strictly decoded and converted into validated domain values.
Domain values have private fields, constructor validation, copied collections,
and transition methods that return new state rather than mutating prior state.

Policy, Schemas, built-in Descriptors, Recipes, and aliases are repository
sources embedded into release binaries. Runtime does not search the current
working directory for built-in assets.

Runtime v1 uses a small dependency set for TOML, JSON Schema, and cross-platform
file locking. It uses no web framework, ORM, dependency-injection framework,
scripting engine, executable plugin runtime, or daemon.

## 16. CLI

Human-facing commands include installation management, Provider discovery and
verification, Profile listing and validation, Project Trust, classification,
Run start and recovery, and state inspection. Machine-facing
`oaw runtime exchange` emits protocol JSON only.

Human diagnostics go to stderr or formatted command output; machine stdout is
stable. All diagnostics expose stable reason codes and support structured JSON
where applicable. Runtime sends no telemetry by default and redacts sensitive
debug fields.

## 17. Migration

1. Freeze domain language, Schemas, Policy vNext, Protocol, and conformance
   fixtures without changing installation behavior.
2. Add a non-authoritative, read-only Go Core in shadow mode that reproduces
   current Provider diagnostics from descriptors and matches Bash black-box
   fixtures. No user-facing or authoritative Go management behavior ships
   before command-level parity passes.
3. Implement deterministic classification and Recipe compilation in shadow
   mode without project mutation.
4. Implement the Revision Journal, Grants, Resource Leases, protocol, and a
   deterministic Fake Host.
5. Audit and enable one conforming Runtime Host; keep all others Policy-only.
6. Install Runtime-aware or Policy-only Thin Entrypoints based on exact Host
   capability.
7. Port installer validation, rendering, backup, transaction, and inert install
   state handling to Go under Bash/Go parity tests.
8. Make `install.sh` a compatibility wrapper after command parity. Release
   archives include a precompiled binary and do not download at execution time.

Existing TSV Install State remains separate from Runtime State. Existing
Policy-only tasks are not imported. Canonical policy paths remain stable during
migration, and existing profile choices are never silently rewritten.

An active Policy-only `ECC-FULL` lock may finish under the legacy policy or
switch explicitly at a Stable Boundary. Its tracker remains legacy provenance
and is never compiled into Runtime State. New Runtime selection of `ECC-FULL`
returns `LEGACY_PROFILE_UNSUPPORTED` and presents `oaw/hardening` plus eligible
user-defined Profiles; no alias or automatic conversion is applied.

## 18. Verification Strategy

Implementation follows TDD. Tests cover Schema contracts, pure domain rules,
property and fuzz invariants, fault-injected state integration, Host Adapter
conformance, isolated CLI black-box behavior, deterministic Runtime E2E, and a
versioned Semantic Classifier eval corpus.

Repository-wide Go statement coverage must remain at least 80 percent. Critical
classification, recipe, workflow, admission, and run-state packages target at
least 90 percent and require invariant, fuzz, and fault-injection tests in
addition to coverage.

The classifier eval release corpus permits no critical scenario to be released
as Direct or Bounded. Hosts that cannot meet proposal schema and risk-recall
gates fall back to Workflow classification.

Required verification includes Go unit and race tests, coverage, vet,
static analysis, vulnerability scanning, existing Bash black-box tests,
cross-platform release builds, and a WSL smoke test.

## 19. Acceptance Criteria

- Direct, Bounded, and Workflow are distinct Request Modes with explicit
  admission and escalation rules.
- Only Workflow Mode blocks for Profile Selection.
- Built-in and user Providers use the same Descriptor, Instance, Capability,
  and Binding model.
- Project configuration cannot grant trust or widen authority.
- Profile Recipes compile deterministically and reject missing, duplicate, or
  ambiguous ownership.
- Every active Runtime-managed invocation has an immutable valid Grant.
- Workflow execution is isolated from the Main Agent or explicitly denied.
- Runtime transitions survive crash injection as either the prior or complete
  next Revision.
- External execution uncertainty never triggers blind replay.
- Concurrent Runtime-admitted write-capable Capability invocations cannot own
  the same Worktree resource; Direct work is explicitly outside this guarantee.
- Project projections never become control input.
- Integration Levels state guarantees truthfully.
- Policy-only Hosts remain supported without enforcement claims.
- Go does not replace Bash management behavior before black-box parity.
- All required unit, integration, E2E, conformance, eval, security, coverage,
  and platform gates pass before the corresponding migration phase completes.
