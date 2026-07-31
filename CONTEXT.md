# Open Agent Workflow Domain

Open Agent Workflow (OAW) governs how independently installed engineering
capabilities are selected, composed, and admitted for one engineering
deliverable. It supports both instruction-only coordination and runtime-managed
execution without owning the providers or host permissions it coordinates.

## Requests and Execution

**Engineering Request**:
The user's engineering intent before OAW classifies or executes it.
_Avoid_: Task, prompt

**Deliverable**:
One coherent engineering outcome with explicit completion conditions. It is the
scope of lifecycle selection and locking.
_Avoid_: Ticket, task

**Request Mode**:
The execution topology selected for an Engineering Request: `DIRECT`,
`BOUNDED`, or `WORKFLOW`. It is independent of complexity and risk.
_Avoid_: Level, profile

**Direct Mode**:
A Request Mode in which the main agent performs a bounded change without
invoking an engineering Provider Capability or creating a Lifecycle Bundle.
_Avoid_: L0, none profile

**Bounded Mode**:
A Request Mode that admits one Capability for one declared deliverable with
limited effects and an explicit termination condition.
_Avoid_: Atomic skill mode, L1

**Workflow Mode**:
A Request Mode that requires explicit Profile Selection, a compiled Execution
Graph, and lifecycle-governed execution.
_Avoid_: L2, complex mode

**Workflow Complexity**:
An `ordinary` or `complex` assessment used only to shape Workflow Mode
recommendations and planning depth.
_Avoid_: Request mode, risk

**Risk Class**:
A `normal`, `elevated`, or `critical` constraint on admission and escalation.
It never selects a Provider or Profile Recipe.
_Avoid_: Complexity, profile recommendation

**Classification Decision**:
The evidence-backed Request Mode, Workflow Complexity, Risk Class, constraints,
and escalation triggers assigned to an Engineering Request.
_Avoid_: Profile recommendation, lifecycle selection

**Classification Proposal**:
A Host-produced, evidence-backed semantic description of an Engineering Request
using OAW's closed trait vocabulary. It is untrusted input to deterministic
Runtime classification.
_Avoid_: Classification decision, free-form analysis

**Classification Policy**:
A versioned set of deterministic minimum-mode, minimum-risk, protected-resource,
and evidence rules used to validate a Classification Proposal.
_Avoid_: Profile recipe, classifier prompt

**Engineering Run**:
One runtime-managed execution of a Deliverable, including its immutable
decisions, state revisions, grants, events, and evidence.
_Avoid_: Session, task

**Successor Run**:
A new Engineering Run created after an explicit Mode Escalation that references
prior evidence without inheriting the predecessor's authority or mutable state.
_Avoid_: Mode mutation, resumed run

**Executor**:
A host-native agent context with a distinct OAW authority identity that acts
under a Capability Grant. Logical Grant separation is bookkeeping, not physical
context isolation. Runtime-managed Workflow execution requires a distinct Host
execution context verified through the trusted Host integration record.
_Avoid_: Provider, lifecycle owner

## Providers and Capabilities

**Provider**:
An independently installed collection of engineering skills, agents, or tools.
Being a Provider does not by itself imply or exclude complete lifecycle
ownership. The selected Profile Recipe and verified Capability coverage
determine whether one Provider Instance is a Lifecycle Owner, Stage Owner, or
Bounded Add-on for a Deliverable.
_Avoid_: Workflow family

**Provider Descriptor**:
Inert, versioned metadata that declares a Provider's identity, discovery probes,
Capabilities, and Host Bindings. It cannot contain executable discovery logic.
_Avoid_: Plugin, installer

**Discovery Evidence**:
The normalized, non-authoritative facts produced by declarative probes about a
possible Provider installation. Evidence never enables a Provider or selects a
Profile Recipe by itself.
_Avoid_: Provider instance, capability verification

**Provider Instance**:
A discovered and verified Provider installation pinned to exact evidence,
version, configuration, and Host Binding for an Engineering Run.
_Avoid_: Provider descriptor, detected provider

**Capability**:
A named engineering ability with declared inputs, outcomes, maximum effects,
Executor topology, delegation ceiling, and Host Bindings that OAW can admit or
compose.
_Avoid_: Skill, stage

**Host Binding**:
An inert mapping from a Capability to a host-native skill, agent, or tool
reference. It never grants permission or embeds executable commands.
_Avoid_: Adapter, script

**Provider Catalog**:
The combined collection of built-in and user-registered Provider Descriptors
known to OAW.
_Avoid_: Detected providers, registry

**Effective Registry**:
The trusted, enabled, and verified Provider Instances and Capabilities available
to a particular Host and project context.
_Avoid_: Provider catalog

**Workflow Family**:
A coherent, versioned engineering methodology that covers every applicable
canonical lifecycle responsibility. A Provider qualifies only when such a
complete recipe is explicitly defined and its required Capabilities are
verified. The same Provider may still expose specialist Capabilities for use by
other Recipes.
_Avoid_: Provider

**Bounded Add-on**:
A Capability bound to one declared specialist deliverable inside a Workflow
without acquiring lifecycle or general stage ownership.
_Avoid_: Specialist provider, lifecycle owner

## Profiles and Control

**Canonical Phase**:
One stable part of OAW's normal lifecycle control flow, such as specification,
planning, execution, review, verification, or completion.
_Avoid_: Skill, arbitrary recipe node

**Procedure**:
A method used inside a Canonical Phase, such as TDD during execution. It does
not become a separate phase merely because a Provider implements it.
_Avoid_: Stage

**Incident Handler**:
A typed recovery path activated by a declared failure class, such as functional
debugging or build repair.
_Avoid_: Lifecycle phase, general fallback

**Checkpoint**:
A named, bounded validation point added to an Execution Graph without replacing
canonical completion or safety requirements.
_Avoid_: Phase, add-on

**Profile Recipe**:
A versioned declarative recipe that binds canonical responsibilities,
Procedures, Incident Handlers, and Checkpoints to Capability selectors.
_Avoid_: Provider, stage list

**Profile Binding**:
A declarative mapping from a Profile Recipe's Capability selectors to preferred
Provider Instances. It changes ownership resolution without copying or changing
the control graph.
_Avoid_: Profile recipe, host binding

**Profile Selection**:
The user's explicit choice of a Profile Recipe and declared add-ons for one
Deliverable.
_Avoid_: Recommendation, automatic selection

**Execution Graph**:
The validated control graph compiled from a Profile Recipe and exact Provider
Instances. It may contain typed recovery and remediation cycles.
_Avoid_: Plan, DAG

**Lifecycle Bundle**:
An immutable generation that pins Profile Selection, Provider Instances,
configuration, add-ons, and the compiled Execution Graph for a Deliverable.
_Avoid_: Mutable workflow record, profile

**Lifecycle Lock**:
The commitment that an active Deliverable follows its current Lifecycle Bundle
until the user switches it at a Stable Boundary.
_Avoid_: File lock, install lock

**Lifecycle Owner**:
The Provider Instance assigned every applicable canonical responsibility by a
complete single-provider Profile Recipe.
_Avoid_: Provider, executor

**Stage Owner**:
The single Provider Instance assigned one canonical responsibility by a Profile
Recipe.
_Avoid_: Executor, add-on

**Stable Boundary**:
A validated transition at which a user may create a new Lifecycle Bundle
generation without invalidating completed work or in-flight execution.
_Avoid_: Pause, arbitrary switch point

## Runtime Authority and State

**Capability Grant**:
An immutable, revision-bound authorization for one Executor to invoke one
Capability within declared effects and resource scope.
_Avoid_: Host permission, role

**Stage Grant**:
A Capability Grant tied to the active node and generation of an Execution Graph.
_Avoid_: Lifecycle lock, provider permission

**Child Grant**:
A Capability Grant derived from a parent Grant. Its Capability must appear in
the parent's immutable delegation allow-list, and its effects, resource scope,
and onward delegation must be strictly equal to or narrower than the parent's
limits.
_Avoid_: Delegated permission, independent grant

**Normalized Observation**:
A typed report produced by a Host Adapter from Provider execution results. It is
transition input, never authority to select the next control-graph node.
_Avoid_: Provider decision, raw output

**Effective Authority**:
The intersection of user authorization, Host permissions, OAW mode limits,
Provider limits, Profile limits, and current control state.
_Avoid_: OAW permission

**Runtime State**:
The authoritative, revisioned record of an Engineering Run's decisions,
Lifecycle Bundle, active graph node, grants, events, and evidence references.
_Avoid_: Install state, project workflow file

**Run Revision**:
One immutable, ordered control transition containing the resulting Runtime
State, accepted event, emitted decision, and Grant changes.
_Avoid_: File version, lifecycle generation

**Resource Lease**:
An exclusive, revision-bound claim that prevents concurrent Runtime-admitted
Capability invocations from mutating the same physical Worktree or Git
repository resource. It does not govern Host work released through Direct Mode.
_Avoid_: Capability grant, timeout lock

**Execution Uncertainty**:
A paused condition in which an external Capability invocation may have started
but no trustworthy completion observation exists. It must be reconciled rather
than blindly replayed.
_Avoid_: Failure, timeout

**Evidence Reference**:
A typed, digest-pinned reference to an observed artifact or command result used
to justify a control transition without copying its full content by default.
_Avoid_: Raw log, project path

**Configuration Snapshot**:
The immutable effective configuration and trust inputs captured for an
Engineering Run. Later configuration changes never alter an active generation.
_Avoid_: Live configuration, lifecycle bundle

**Project Trust**:
A user-owned decision that permits exact project configuration content at a
physical project identity to participate in effective configuration.
_Avoid_: Project configuration, repository permission

**Project Projection**:
A human-readable downstream view of Runtime State stored with project artifacts.
It is never parsed back as an authority or control-state source.
_Avoid_: Lifecycle lock, runtime state

**Mode Escalation**:
A paused decision that requires a new Request Mode when current scope, risk, or
effects exceed the active mode's ceiling. Modes never change silently.
_Avoid_: Automatic profile selection

## Planes and Host Integration

**Policy Plane**:
The portable instruction layer that defines OAW governance and supports Hosts
that cannot participate in runtime admission.
_Avoid_: Runtime, policy-only mode

**Runtime Plane**:
The optional execution layer that manages Engineering Runs, Provider
resolution, Capability admission, control transitions, and authoritative state.
_Avoid_: Sandbox, provider executor

**Policy-only Mode**:
A Host integration mode that supplies OAW instructions without claiming runtime
admission, isolation, or transition enforcement.
_Avoid_: Runtime-managed mode

**Runtime-managed Mode**:
A Host integration mode that drives execution through the Runtime Protocol and
honors emitted Capability Grants and control decisions.
_Avoid_: Policy-only mode

**Host**:
An agent environment, such as Codex or Claude Code, that loads instructions and
may expose native skills, agents, tools, or runtime integration points.
_Avoid_: Provider, target

**Integration Level**:
A Host Adapter's declared OAW participation level: `instruction-only`,
`runner-managed`, or `native-managed`. It states available guarantees rather
than general Host quality.
_Avoid_: Support status, request mode

**Host Manifest**:
A versioned declaration of a Host Adapter's supported Runtime Protocols,
binding kinds, Executor isolation, invocation, deduplication, cancellation, and
observation features. Runtime admits it through a built-in or user-trusted Host
integration record and pins its digest; a per-run Host frame may only narrow
the admitted features.
_Avoid_: Provider descriptor, host configuration

**Runtime Protocol**:
The host-neutral transition interface shared by the reference runner and every
runtime-capable Host Adapter.
_Avoid_: CLI output, provider protocol

**Host Adapter**:
A translation between the Runtime Protocol and one Host's native interaction,
Capability invocation, and Executor mechanisms.
_Avoid_: Instruction adapter, provider binding

**Instruction Adapter**:
A translation that makes the canonical OAW policy visible through one Host's
instruction surface without claiming runtime enforcement.
_Avoid_: Host adapter, provider binding

## Installation and Ownership

**Core Instruction Adapter**:
An Instruction Adapter whose user-level instruction surface is stable enough
for OAW to support as a default global installation target.
_Avoid_: Core adapter

**Extension Instruction Adapter**:
An Instruction Adapter supported primarily at project scope because its global
instruction surface is less stable or platform-specific.
_Avoid_: Extension adapter

**Target**:
One supported Host selected for an installation lifecycle operation.
_Avoid_: Host, provider

**Scope**:
The installation extent for an Instruction Adapter: user scope across projects
or project scope for one repository.
_Avoid_: Capability resource scope

**Canonical Rule Source**:
The single OAW-owned policy artifact from which Instruction Adapters derive
their governance behavior.
_Avoid_: Adapter copy, runtime state

**Thin Entrypoint**:
A small Host-native instruction that directs an agent to the Canonical Rule
Source without duplicating its policy.
_Avoid_: Policy copy

**Managed Block**:
A mechanically delimited OAW-owned section inside a user-owned instruction file.
It controls installer ownership, not model precedence.
_Avoid_: Policy priority marker

**Owned File**:
An adapter-specific instruction file whose complete contents are managed by OAW.
_Avoid_: Managed block

**Install State**:
The OAW-owned record of installed policy version, Targets, Scopes, destinations,
checksums, and recoverable backups.
_Avoid_: Runtime state, lifecycle lock

**Drift**:
A mismatch between recorded OAW-owned installation content and current content
on disk that blocks mutation until explicitly and recoverably resolved.
_Avoid_: Runtime state divergence
