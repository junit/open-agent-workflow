# Open Agent Workflow Domain

Open Agent Workflow (OAW) is a rule-driven engineering workflow product. Its
static Policy and model-readable Profiles form a complete operating path;
optional machine components may add evidence or coordination without reducing
what the Policy path can do.

## Policy Product

**Policy**:
The portable rules that define OAW's safety boundaries, engineering defaults,
Profile behavior, and model decision principles.
_Avoid_: Policy Plane, engineering manual, Runtime policy

**Static Policy Sufficiency**:
The invariant that an installed Policy, selected Profile, available Skills, and
the Host's native abilities are enough to complete normal engineering work
without an OAW runtime process.
_Avoid_: Policy-only degradation, fallback mode

**Monotonic Enhancement**:
The rule that an optional OAW component may add convenience, evidence, or
coordination but may not make a Policy-valid workflow unavailable.
_Avoid_: Enhanced eligibility, machine veto

**Canonical Policy Set**:
One selected version of the Policy, cooperative protocol, Built-in Profiles,
and current Host Adapter guidance. A project set and a user set are never
merged.
_Avoid_: Canonical Rule Source, single policy artifact

**Activation Router**:
The small Host-facing instruction that keeps OAW opt-in and loads the selected
Canonical Policy Set only for an explicitly activated deliverable.
_Avoid_: Always-on policy, Policy importer

**Deliverable**:
One coherent engineering outcome that the user asks OAW to help produce.
_Avoid_: Engagement, Engineering Run, ticket

**Profile**:
A named, model-readable engineering method that maps Responsibilities to
Skills, Host-native actions, and additional rules.
_Avoid_: Profile Recipe, Execution Graph, workflow family

**Built-in Profile**:
A Profile shipped and maintained as part of the Canonical Policy Set.
_Avoid_: Hard-coded profile, built-in recipe

**Custom Profile**:
A user- or project-owned Markdown Profile that may combine currently available
Skills without Provider metadata or machine evidence.
_Avoid_: USER-DEFINED Recipe, custom bundle

**Responsibility**:
One stable engineering outcome area, such as problem framing, planning,
implementation, review, verification, or closeout. It guides completeness
without acting as a machine state.
_Avoid_: Lifecycle Slot, graph node, mandatory stage

**Policy Default**:
The model-native behavior supplied by the Policy when a Profile does not name a
Skill or special rule for a Responsibility.
_Avoid_: Silent fallback, inherited profile

**Skill**:
A model-readable procedure used to perform one or more Responsibilities. A
Skill remains independently installed and owns its procedure only within the
scope assigned by the selected Profile.
_Avoid_: Capability Binding, Provider identity, lifecycle owner

**Skill Availability**:
The model's current ability to read a Skill's rules or use the Host's native
invocation surface. A scanner observation alone neither proves nor disproves
availability.
_Avoid_: Host-routable proof, verified Binding

**Profile Selection**:
The user's explicit Profile choice or the model's stated choice when no genuine
ambiguity requires a question. Selection authorizes the Profile's declared
Skills for the current deliverable.
_Avoid_: Startup Gate, Policy Offer, selection receipt

**Add-on**:
A task-scoped specialist Skill added through natural language without taking
ownership of the Profile's core Responsibilities.
_Avoid_: Bounded Capability, NONE selection, lifecycle overlay

**Progress Note**:
An optional Markdown summary of the selected Profile, completed
Responsibilities, current work, evidence, and next step. It is a continuity aid,
not authoritative control state.
_Avoid_: Runtime State, Engagement database, lifecycle lock

**Complexity**:
The model's qualitative judgment about how much decomposition, planning, and
coordination a deliverable needs.
_Avoid_: Request Mode, Profile selector, complexity gate

**Risk**:
The model's qualitative judgment about the consequence of error and the needed
strength of review, approval, and verification.
_Avoid_: Request Mode, automatic Profile selection, admission class

## Host and Installation

**Host**:
An agent environment, such as Codex or Claude Code, that loads instructions and
owns model execution, tools, credentials, sandboxing, approvals, and physical
effects.
_Avoid_: Provider, OAW executor

**Host Adapter**:
Host-specific Policy guidance and installation behavior that exposes the
Canonical Policy Set through one Host without changing Profile semantics.
_Avoid_: Provider Binding, Host runtime, workflow implementation

**Installation Scope**:
The location that owns one installed Canonical Policy Set: a user environment
or one project.
_Avoid_: Capability scope, mutation scope

**Project Policy Set**:
A project-contained Canonical Policy Set that governs that project and can be
versioned with it. It takes precedence over a User Policy Set without merging.
_Avoid_: Project override fragment, global policy import

**User Policy Set**:
A user-owned Canonical Policy Set used only when the current project has no
Project Policy Set.
_Avoid_: Machine-wide mandatory policy, project fallback merge

**Managed Block**:
A mechanically delimited OAW-owned section in a Host instruction file that
points to the selected Canonical Policy Set. It defines installer ownership,
not model authority.
_Avoid_: Policy priority marker, embedded policy copy

**Observed Route**:
A non-authoritative Host Adapter diagnostic indicating that a Skill or action
may be available through a known Host surface.
_Avoid_: Skill Availability, Profile eligibility, route proof

**Physical Authority Boundary**:
The boundary at which the Host, operating system, repository, and user approval
controls remain authoritative regardless of OAW Policy or machine claims.
_Avoid_: OAW sandbox, logical permission enforcement

## Optional Machine Assurance

**Machine Assurance**:
An optional component that adds machine-verifiable claims about Profile
content, Provider identity, Bindings, execution evidence, or coordination.
_Avoid_: Required runtime, stronger Policy mode

**Assurance Overlay**:
Machine metadata attached to one Policy Profile that pins content and exact
Bindings without changing the Profile's Responsibilities, Skill composition,
or rules.
_Avoid_: Profile Recipe, alternate workflow definition

**Provider**:
An independently installed collection of Skills, agents, or tools that may be
identified precisely by Machine Assurance.
_Avoid_: Profile, workflow owner

**Binding**:
A machine claim that maps one Profile Skill or Host action to an exact
Provider- and Host-specific invocation surface.
_Avoid_: Skill, Host Adapter, user authorization

**Machine Run**:
One optional machine-tracked execution of a selected Profile and Assurance
Overlay.
_Avoid_: Deliverable, Policy session

**Lifecycle Bundle**:
An optional immutable machine artifact that combines a Policy Profile
reference, Assurance Overlay, exact Host facts, and execution requirements for
one Machine Run.
_Avoid_: Profile, Policy plan

**Receipt**:
A machine-recorded observation about an invocation or result. It may support an
assurance claim but never transfers physical authority from the Host.
_Avoid_: Completion proof, Host permission

**Coordinator**:
An optional machine component that serializes cooperating Machine Runs and
records their evidence and transitions.
_Avoid_: Policy progress tracker, project lock

**Bridge**:
An optional Host integration for machine evidence or coordination. It is not a
sandbox, process supervisor, or prerequisite for Policy execution.
_Avoid_: Policy Adapter, physical enforcement boundary
