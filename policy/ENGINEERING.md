# Open Agent Workflow Policy

## Purpose

Open Agent Workflow (OAW) is a portable Policy Plane for engineering agents.
It classifies requests, prevents competing engineering methodologies from
claiming the same responsibility, and records one explicit lifecycle choice
when a lifecycle is required. OAW does not install or own engineering
Providers.

The optional Runtime Plane can enforce admission and transitions on a
conforming Host. On every other Host, this document remains a coordination
policy rather than a claim of technical enforcement.

## Request Classification

Classify every new top-level engineering request as exactly one Request Mode:

| Mode | Contract |
| --- | --- |
| `DIRECT` | The Main Agent performs a small, clear, bounded change without selecting a lifecycle or invoking a Provider Capability. |
| `BOUNDED` | One verified Capability performs one observable deliverable with declared effects, resources, and termination conditions. |
| `WORKFLOW` | A versioned Profile Recipe coordinates multiple engineering responsibilities through a Lifecycle Bundle. |

Use only enough read-only inspection to classify the request. Classification
evidence should name the known change point, affected contracts and resources,
verification seam, unresolved decisions, and relevant risk.

### Direct Mode

Direct Mode requires clear requirements, a known change point, bounded and
recoverable scope, no unresolved architecture or domain decision, no public
contract, schema, dependency, security, data, or deployment semantic change,
and a known focused verification command.

The Main Agent may implement Direct work without workflow artifacts or a
Lifecycle Bundle. Direct work invokes no engineering Provider Capability and
is outside OAW Runtime admission and Resource Leases. Scope expansion requires
reclassification; Direct Mode never silently absorbs lifecycle work.

### Bounded Mode

Bounded Mode admits one atomic Capability for one named deliverable. The
selection must identify one verified Provider Instance and Capability, either
from explicit user intent or an exact user-trusted rule. Detection alone never
selects it.

A Bounded Capability cannot claim a Canonical Phase, general planning,
implementation ownership, a remediation loop, Git completion, or lifecycle
ownership. The Main Agent may execute it only when its declared executor
topology permits main-agent execution. A required second Capability, an
architectural decision, a remediation loop, or wider effects requires
reclassification or escalation.

### Workflow Mode

Workflow Mode applies when requirements, root cause, domain behavior, or
architecture remain unresolved; several engineering responsibilities interact;
public contracts, schemas, dependencies, migrations, sensitive mutations,
multiple tickets, or long-lived delegation are involved; or a lower mode
escalates.

Only Workflow Mode runs the Startup Gate. Workflow Complexity and Risk Class
control recommendation, planning depth, and verification strength; they are
orthogonal to Request Mode and do not activate the Gate by themselves.

## Mandatory Startup Gate

For a request classified as `WORKFLOW`:

1. Read this policy before invoking a lifecycle Capability.
2. State the Request Mode, Complexity, Risk Class, and concrete evidence.
3. Show every eligible built-in and user-defined Profile, mark a recommendation,
   and list every proposed bounded add-on.
4. Wait for the user's explicit Profile selection. There is no timeout, silent
   default, or selection based only on Provider discovery.
5. Compile the selected Recipe against verified Provider Capabilities. Reject
   missing, ambiguous, or conflicting ownership instead of guessing.
6. Record the resulting Lifecycle Bundle before lifecycle execution begins.

Before Workflow selection, do not start discovery, design, planning,
implementation, TDD, debugging, delegation, Git work, review, or completion.
Governance, read-only Utility work, classification, explanation, and status
reporting remain allowed. Direct Mode and Bounded Mode do not run this Gate.

## Skill and Capability Classes

OAW distinguishes three operational classes:

1. Governance and Utility operations are read-only or administrative and do
   not claim engineering ownership.
2. Atomic skills are modeled as Bounded Capabilities. They have one purpose,
   declared effects, and a termination condition.
3. Workflow skills expose lifecycle Capabilities used by a Profile Recipe.
   Their ownership is controlled by the active Execution Graph.

This classification does not forbid the Main Agent from using skills. It
prevents an atomic skill from becoming an undeclared lifecycle and prevents
multiple workflow owners from controlling the same responsibility.

## Provider and Capability Model

Superpowers, Matt, ECC, and third-party Providers use the same extensible
Provider and Capability model. Built-in descriptors have these stable IDs:

```text
oaw/superpowers
oaw/matt
oaw/ecc
```

Provider authority follows this exact identity chain:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

A Verified Provider Instance means exactly one Host Installation plus
Host-owned Binding Evidence for that same installation. Codex and Claude Code
are independent Hosts; even shared physical files produce separate Host
Installation identities. Descriptor bindings and configured installation
hints are declarations only and never create Binding Evidence. A Policy-only
Host may expose Candidates but cannot verify a Runtime Instance. Foreign-Host
diagnostics never become pins, Registry input, Profile ownership, admission,
or Runtime authority.

Built-in descriptors define declarative discovery probes and Capability
contracts; discovery still occurs dynamically on the current Host. Users may
register trusted third-party descriptors, Profile Recipes, bindings, pins, and
denials through configuration. Trusted project configuration may recommend or
narrow records but cannot create trust or expand authority.

The active Provider Descriptor and user configuration schemas are v2-only.
Reject `oaw.provider-descriptor/v1` and `oaw.user-config/v1` instead of
upgrading them. A Provider pin is Host-scoped by `provider_id`, `host_id`,
`installation_key`, and `evidence_digest`; optional `location` and `version`
are readable assertions and must also match when present. Stable scope reasons
include `HOST_BINDING_EVIDENCE_REQUIRED`, `PROVIDER_BINDING_UNAVAILABLE`,
`PROVIDER_FOREIGN_HOST_ONLY`, `PROVIDER_PIN_INCOMPATIBLE`, and
`HOST_PROVIDER_SCOPE_MISMATCH`.

A Provider may offer both complete lifecycle and specialist Capabilities. Its
role comes from the selected Recipe, not its brand. Full-family eligibility
requires verified Capability coverage for every responsibility in that Recipe,
using the same rule for built-in and third-party Providers.

Provider detection is diagnostic. Only a verified Provider Instance enters
Profile compilation. If a required Capability is missing or ambiguous, stop
and ask the user to install it, repair its trusted registration, or select a
different Profile. Never silently omit or substitute a required owner.

## Profile Selection

OAW ships these built-in Recipes and compatibility aliases:

| Selection | Recipe and ownership |
| --- | --- |
| `SP-FULL` | `oaw/delivery`; a verified Superpowers Provider owns the complete delivery lifecycle. |
| `MATT-FULL` | `oaw/domain-engineering`; a verified Matt Provider owns the complete domain-engineering lifecycle. |
| `ECC-FULL` | `oaw/ecc-engineering`; a verified ECC Provider owns the complete engineering lifecycle. |
| `MATT-SP-HYBRID` | `oaw/reliable-feature`; ownership follows the explicit map below. |
| `USER-DEFINED` | A selection action that chooses a configured, versioned user-defined Profile Recipe. It is not a built-in Profile or alias. |

`ECC-FULL` is a complete lifecycle, not merely a hardening add-on. It covers
discovery, specification and planning, implementation, testing, debugging and
build repair, review, delegation, verification, and completion when the full
required ECC Capability set verifies. In another Recipe, ECC may instead own
only a bounded specialist Capability or typed Incident Handler.

A user-defined Recipe is eligible only when compilation resolves exactly one
owner for every applicable responsibility, explicit transitions and terminal
gates, bounded add-ons, and effects within configured authority. A
recommendation never becomes a default, and the user's valid selection wins.

## Workflow Execution Isolation

Runtime-managed Workflow Capabilities that declare `isolated-required` execute
in an isolated Executor context. The Main Agent or Host coordinates user
communication, classification, Profile selection, and Bundle identity; the
admitted Executor performs its assigned discovery, planning, implementation,
testing, or verification and returns normalized evidence references.

Every dispatched Executor inherits the exact Lifecycle Bundle generation,
active graph node, admitted Capability, allowed effects and resources,
termination condition, and evidence requirements. It does not reopen Profile
arbitration. The active graph and Grant define allowed actions; work outside
that admission is blocked or requires a transition.

Policy-only Hosts provide instruction-level coordination only. They do not
provide OAW Runtime admission, Grants, Resource Leases, transition enforcement,
or physical isolation. On such Hosts, lifecycle locks and allowed-action rules
are best-effort policy obligations, and OAW must not describe them as Runtime
guarantees.

## Lifecycle Lock

Record the task and deliverable identity, classification, selected Profile and
selection source, Bundle generation and digest, stage owners, exact add-ons,
active stage, active ticket, allowed and blocked actions, and canonical
artifact or evidence references.

The lock persists across follow-ups, context compaction, and delegated work on
the same deliverable. Ticket inheritance keeps the same Bundle unless the user
switches at an allowed stable boundary. A new unrelated request is classified
again and does not inherit authority from the old Run.

The Runtime State and Lifecycle Bundle are authoritative on a Runtime-managed
Host. A Markdown lock is only a Policy-only projection and cannot grant
authority.

## Matt-Superpowers Hybrid

`MATT-SP-HYBRID` assigns exactly one owner to each responsibility:

| Responsibility | Owner |
| --- | --- |
| Requirements and domain modeling | Matt |
| Product specification and acceptance criteria | Matt |
| Test-seam selection and ticket decomposition | Matt |
| Per-ticket executable implementation plan | Superpowers `writing-plans` |
| Workspace and Git setup | Superpowers |
| Implementation orchestration and code changes | One Superpowers executor |
| TDD method and red-green loop | Matt `tdd` |
| Functional and hard-bug debugging | Matt `diagnosing-bugs` |
| Build, dependency, and type repair | Selected ECC Incident Handler, or none |
| Spec compliance and code-quality review | Superpowers |
| Review remediation and re-review | Superpowers |
| Fresh verification and branch completion | Superpowers |
| Specialist checks | Only explicitly selected bounded add-ons |

Matt specifications and tickets are canonical for requirements and delivery
edges. Superpowers plans may add exact paths, commands, code steps, and expected
results but may not change those requirements or ticket boundaries. Repair a
requirement gap in the Matt source before continuing.

Use Matt `tdd` as the only TDD procedure in this hybrid; pause Superpowers TDD.
Use one Superpowers implementation executor; do not add a second Matt or ECC
implementation owner. Keep Superpowers review and completion active; pause Matt
and ECC general review.

An expected RED test belongs to TDD. For an unexpected functional failure,
record the intended state, command, and output and transfer control to Matt
debugging. A strictly build, dependency, or type failure may route only to the
selected ECC Incident Handler.

## Bounded Add-ons and Security

An add-on from a non-owning Provider is authorized only for its named
deliverable. It cannot take over discovery, planning, implementation, TDD,
debugging, general review, delegation, Git, or completion. End it when the
deliverable is complete and return digest-pinned evidence to the active owner.

Security, coverage, style, and required checks are outcome constraints, not
Profile selections. Security review may be supplied by a bounded ECC or
third-party Capability, external scanners, or the lifecycle owner's checklist.
Security-sensitive work can also require security acceptance criteria and
negative tests without changing lifecycle ownership.

## Artifacts and Project Projections

Use the project's existing documentation layout. A local Policy-only tracker
may use:

```text
CONTEXT.md
.scratch/<feature>/workflow.md
.scratch/<feature>/spec.md
.scratch/<feature>/issues/<NN>-<slug>.md
.scratch/<feature>/evidence/review.md
.scratch/<feature>/evidence/verification.md
docs/superpowers/plans/YYYY-MM-DD-<feature>-<NN>-<ticket>.md
```

Runtime-managed project workflow documents are one-way, non-authoritative
projections of committed Runtime State. They may show selected Profile, Bundle
generation, stage, active ticket, digest-pinned evidence references, and lag
status. They exclude credentials, full Grants, sensitive Evidence content, and
raw Provider output. They are never parsed back as authority or control input.

Do not create duplicate design specifications or implementation plans for the
same owner and ticket.

## Stable Switching

Only the user may switch a selected Workflow Profile. Switch at an approved
specification, between completed tickets, after a completed TDD or debugging
cycle, after review, or after recorded verification. Do not switch during an
active Capability invocation, delegated work, unresolved merge, or incomplete
red-green cycle.

A switch compiles a new immutable Bundle generation, revokes outstanding
Grants from the old generation, and preserves valid artifacts and evidence. It
does not rewrite completed ownership or silently replace a Provider.

## Neutral Safety Rules

- Observe fresh verification output before claiming completion.
- Preserve unrelated user changes.
- Do not perform destructive Git or filesystem operations without approval.
- Diagnose root causes before bug fixes.
- Treat network tools as read-only unless the user authorizes mutation.
- Validate inputs at system boundaries and never embed credentials in policy,
  Runtime State, project projections, logs, or evidence references.
- Detection reports capabilities; it never selects a Capability or Profile.
