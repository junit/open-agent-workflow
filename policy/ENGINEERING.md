# Open Agent Workflow Policy

## Purpose and Authority Boundaries

Open Agent Workflow (OAW) is a portable Policy Plane for engineering agents.
It classifies requests, resolves independently installed engineering
Providers, prevents competing methodologies from claiming the same
responsibility, and compiles one explicit lifecycle contract when a lifecycle
is required. OAW does not install or own engineering Providers.

OAW has three distinct ownership boundaries:

1. **OAW Core** is required and stateless. It owns classification, Provider
   and Capability resolution, Profile compilation, and Lifecycle Bundle
   construction.
2. **Workflow Coordinator** is optional and Workflow-only. It owns durable
   revisions, idempotency, cooperative Resource Leases, evidence references,
   and legal lifecycle transitions for cooperating clients.
3. **Agent Host** is external to OAW. The Agent Host owns physical execution
   authority, including Agents, model calls, MCP, Hooks, Skills, Plugins,
   authentication, tools, sandbox, and approvals.

OAW never starts a model process. It never emulates a child Agent, reconstructs
the Host environment, or converts logical workflow authority into a claim of
physical containment.

## Request Classification

Classify every new top-level engineering request as exactly one Request Mode:

| Mode | Contract |
| --- | --- |
| `DIRECT` | The Main Agent performs a small, clear, bounded change without selecting a lifecycle or invoking a Provider Capability. |
| `BOUNDED` | One verified Capability performs one observable deliverable with declared effects, resources, and termination conditions. |
| `WORKFLOW` | A versioned Profile Recipe coordinates multiple engineering responsibilities through a Lifecycle Bundle. |

Use only enough read-only inspection to classify the request. Classification
evidence should name the known change point, affected contracts and resources,
verification boundary, unresolved decisions, and relevant risk. Workflow
Complexity and Risk Class control recommendation, planning depth, and
verification strength; they are orthogonal to Request Mode.

Scope expansion requires reclassification. A lower mode never silently
absorbs work that requires a broader contract.

## Direct Mode

Direct Mode requires clear requirements, a known change point, bounded and
recoverable scope, no unresolved architecture or domain decision, no public
contract, schema, dependency, security, data, or deployment semantic change,
and a known focused verification command.

The Main Agent may implement Direct work in the active session. `DIRECT` has
no Capability, Profile, Lifecycle Bundle, Startup Gate, lifecycle lock,
Resource Lease, or Workflow State.

## Bounded Mode

Bounded Mode admits one atomic Capability for one named deliverable. The
selection identifies exactly one verified Provider Instance and Capability,
either from explicit user intent or an exact user-trusted rule. Detection alone
never selects a Capability.

A Bounded Capability has declared effects, resources, evidence, and one
terminal condition. It cannot claim a Canonical Phase, general planning,
implementation ownership, a remediation loop, Git completion, or lifecycle
ownership. A required second Capability, architectural decision, remediation
loop, or wider effect requires reclassification or escalation.

`BOUNDED` has no Profile, Lifecycle Bundle, Startup Gate, lifecycle lock,
Resource Lease, or Workflow State. Its topology must be supported by both the
Capability and current Host session. An unavailable requested topology is
reported; OAW does not create a process fallback.

## Workflow Mode

Workflow Mode applies when requirements, root cause, domain behavior, or
architecture remain unresolved; several engineering responsibilities
interact; public contracts, schemas, dependencies, migrations, sensitive
mutations, multiple tickets, or long-lived delegation are involved; or a
lower mode escalates.

Only Workflow Mode runs the Startup Gate. Workflow execution begins only after
the user explicitly selects a Profile, eligible execution topology, and any
bounded add-ons, and OAW Core compiles the resulting Lifecycle Bundle.

DIRECT and BOUNDED do not create Workflow State. Only `WORKFLOW` may use the
optional Workflow Coordinator and its durable transition protocol.

## Mandatory Startup Gate

For a request classified as `WORKFLOW`:

1. Read this policy before invoking a lifecycle Capability.
2. State the Request Mode, Complexity, Risk Class, and concrete evidence.
3. Resolve verified Provider Instances and compile every eligible built-in and
   user-defined Profile choice.
4. Show eligible execution topologies, mark Profile and topology
   recommendations, explain exclusions, and list every proposed bounded
   add-on.
5. Wait for the user's explicit Profile and topology selection. There is no
   timeout, silent default, or selection based only on Provider discovery.
6. Compile the selected Recipe against verified Provider Capabilities. Reject
   missing, ambiguous, or conflicting ownership instead of guessing.
7. Record the resulting Lifecycle Bundle before lifecycle execution begins.

Before Workflow selection, do not start discovery, design, planning,
implementation, TDD, debugging, delegation, Git work, review, or completion.
Governance, read-only Utility work, classification, explanation, and status
reporting remain allowed. Direct Mode and Bounded Mode do not run this Gate.

## OAW Core

OAW Core is the required stateless decision and compilation module. It:

- classifies request evidence;
- resolves Host-scoped Provider Instances from trusted descriptors and
  Host-observed bindings;
- computes eligible Profiles, bounded add-ons, and execution topologies;
- returns reason-coded exclusions and non-binding recommendations;
- validates the user's explicit selection; and
- constructs the immutable Lifecycle Bundle.

Callers never author, patch, or infer a Lifecycle Bundle. In coordinated use,
the Workflow Coordinator invokes Core inside the initial state transition and
commits the exact result. In policy-only use, the caller receives the same
Core-produced Bundle without durable coordination guarantees.

Core accepts secret-free request, configuration, Provider, and Host facts. It
does not retain conversational history, credentials, or private Host extension
configuration.

## Provider, Capability, and Profile Model

OAW distinguishes Governance and Utility operations, atomic Bounded
Capabilities, and Workflow Capabilities. Superpowers, Matt, ECC, and
third-party Providers all use the same descriptor, discovery, verification,
binding, Recipe, and compiler path. The built-in Provider IDs are
`oaw/superpowers`, `oaw/matt`, and `oaw/ecc`.

Provider authority is resolved through one Host-scoped chain:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

OAW ships declarative records, not Provider code. The active contracts are
Provider Descriptor `oaw.provider-descriptor/v4`, Profile Recipe
`oaw.profile-recipe/v3`, Execution Graph `oaw.execution-graph/v4`, and
Lifecycle Bundle `oaw.lifecycle-bundle/v4`. There is no old-schema authority
fallback. A missing or stale record fails closed.

Skills, Claude custom Agents, Codex Roles, Instructions, Hooks, and tools are
distinct Host surfaces. Binding kinds are `skill`, `agent`, `role`, and
`instruction`; Hooks and tools are evidence or execution surfaces, not
interchangeable Bindings. A name on one surface never proves another surface.
Only an exact, trusted, complete-tree, Host-observed Binding can compile.

Provider role comes from the selected Recipe, never from the Provider brand.
All four built-in aliases stay active even when the current Host cannot compile
one:

| Selection | Recipe | Contract |
| --- | --- | --- |
| `MATT-FULL` | `oaw/domain-engineering` | Matt-led lifecycle plus neutral Host actions and user/Host gates. |
| `SP-FULL` | `oaw/delivery` | Complete inline Superpowers delivery path. |
| `ECC-FULL` | `oaw/ecc-engineering` | ECC-led lifecycle using exact Host-surface alternatives. |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` | The preserved Matt/Superpowers composition below. |
| `USER-DEFINED` | configured Recipe ID | Selection action for a trusted, versioned custom Recipe; not a fifth built-in alias. |

`FULL` means the Provider-led lifecycle plus neutral Host/user controls. It
does not mean that a Provider owns the Host. Eligibility is computed from
verified Bindings, topology, delegation, invocation, Host actions, user
authority, effects, resources, transitions, and gates. An ineligible alias is
reported with exact diagnostics; it is never deleted or silently replaced.

## Canonical Lifecycle Slots

Every Workflow Recipe uses these ten ordered slots. A pipeline may contain
multiple procedures while exactly one expanded unit owns the outcome.

| # | Slot | Required outcome and control |
| --- | --- | --- |
| 1 | `problem-framing` | Aligned purpose, constraints, domain terms, decisions, and success conditions; `shared-understanding` user gate. |
| 2 | `solution-specification` | Reviewable specification and test boundaries; `specification-approved` user gate. |
| 3 | `delivery-planning` | Independently verifiable units and executable plan; `delivery-plan-approved` user gate. |
| 4 | `workspace-preparation` | Safe workspace and known baseline; `workspace.prepare-or-confirm` Host action and `workspace-ready` Host gate. |
| 5 | `implementation` | Approved bounded changes from the selected implementation owner. |
| 6 | `implementation-tdd` | Witnessed expected RED/GREEN cycle, directly or through an audited macro call. |
| 7 | `incident-recovery` | Conditional typed recovery, replan, or explicit stop; never an unconditional fake stage. |
| 8 | `review-remediation` | Findings adjudicated, remediated, and re-reviewed. |
| 9 | `fresh-verification` | Fresh claim-relevant evidence; `verification.execute` when Host-owned and `fresh-evidence` Host gate. |
| 10 | `closeout` | Accepted, user-authorized delivery or preservation result; `closeout.execute` when Host-owned and `user-closeout` user gate. |

Gate records and Host actions are Provider-neutral and contain no Provider
selector. Macro expansion is also explicit: a `credit-only` internal call
records work already performed by its enclosing unit; `dispatch-before` and
`dispatch-after` run once at the declared edge. An uncredited duplicate owner
or internal call returns `MACRO_INTERNAL_CONFLICT`.

## Built-in Profile Matrix

The table is a readable projection of the pinned machine-readable Recipes.
Parenthesized calls are credited, paused, conditional, or Host-owned exactly as
shown; they are not extra outcome owners.

| Slot | `MATT-FULL` | `SP-FULL` | `ECC-FULL` | `MATT-SP-HYBRID` |
| --- | --- | --- | --- | --- |
| `problem-framing` | Matt `grill-with-docs` (`credit-only`: `grilling`, `domain-modeling`) | `superpowers:brainstorming` | Codex skill `intent-driven-development` or exact Claude Agent `architect` | Matt `grill-with-docs` (`grilling`, `domain-modeling` credited) |
| `solution-specification` | Matt `to-spec` | enclosing `superpowers:brainstorming` outcome | skill `product-capability`; conditional `contract-first` is not a second owner | Matt `to-spec` |
| `delivery-planning` | Matt `to-tickets` | `superpowers:writing-plans`, called once `dispatch-after` brainstorming | observed Codex `/plan` instruction or `blueprint`; Claude Agent `planner` or `blueprint` | Matt `to-tickets` owns ticket edges, then `superpowers:writing-plans` adds executable detail |
| `workspace-preparation` | Host `workspace.prepare-or-confirm`; Matt has no workspace Binding | `superpowers:using-git-worktrees` | Host action, with `git-workflow` guidance only | `superpowers:using-git-worktrees` |
| `implementation` | Matt `implement` macro | inline `superpowers:executing-plans` | `tdd-workflow`, or exact Claude Agent `tdd-guide` alternative | inline `superpowers:executing-plans`; SDD is paused |
| `implementation-tdd` | Matt `implement` credits Matt `tdd` once | `superpowers:test-driven-development` | same selected implementation/TDD unit; no duplicate peer | Matt `tdd`; Superpowers TDD is paused |
| `incident-recovery` | Matt `diagnosing-bugs` only for functional, hard-bug, or performance incidents; other types stop | `superpowers:systematic-debugging` for typed technical incidents | exact typed route only; Claude `build-error-resolver` may handle build/type/dependency when verified | Matt `diagnosing-bugs` for functional incidents; build/type/dependency stops unless an ECC handler was explicitly selected as an Add-on |
| `review-remediation` | `implement` credits Matt `code-review`; remediation is a new bounded `implement` pass followed by fresh internal review | `superpowers:requesting-code-review` then `superpowers:receiving-code-review` and re-review | exact Codex Role `reviewer` or Claude Agent `code-reviewer`, plus a separately verified remediation procedure | Superpowers request/receive/re-review; Matt review and SDD internal review are paused |
| `fresh-verification` | Host `verification.execute`; Matt has no broad verification Binding | `superpowers:verification-before-completion` | skill `verification-loop`; `e2e-runner` and `e2e-testing` remain specialist checks | `superpowers:verification-before-completion` |
| `closeout` | Host `closeout.execute`; Matt has no completion Binding | `superpowers:finishing-a-development-branch` with user authority | Host action with `git-workflow` guidance; reviewer and delivery Hook do not own it | `superpowers:finishing-a-development-branch` with user authority |

Matt's shipped engineering entrypoint is `grill-with-docs`, not a fictional
requirements Skill. Its `implement` macro does not claim workspace creation,
broad verification, or closeout. Superpowers `subagent-driven-development`
remains available to a versioned `USER-DEFINED` Recipe when live child or
nested-child delegation is proved; the built-in `SP-FULL` deliberately uses
the inline path because its standalone review pipeline has a different owner
shape.

ECC Skill, Agent, Role, Instruction, Hook, and tool surfaces never substitute
for one another. In particular, a Claude Agent filename does not establish a
Codex Role, static multi-agent configuration is at most `host-configured`, and
specialist E2E or review surfaces do not expand into broad verification or
completion.

A `USER-DEFINED` Recipe may freely combine installed, trusted, Host-verified,
compatible Bindings per slot. It must pin its version and sources, select one
outcome owner for every applicable slot, declare all macro/internal-call
relationships, incident routes, alternatives, Host actions, neutral gates,
effects, resources, and terminal conditions, and fail closed on every gap.
There is no Provider-specific compiler branch or silent default.

## Execution Topologies

OAW recognizes exactly two execution topologies:

| Topology | Meaning |
| --- | --- |
| `CURRENT` | The active Agent session performs the work with its current Host environment unchanged. |
| `SUBAGENT` | The active Agent Host creates a child through its native Subagent facility. |

`CURRENT` creates no Agent, model process, projected configuration, alternate
workspace, or replacement environment. The active Host's authentication,
tools, extensions, sandbox, approvals, and context remain in force.

`SUBAGENT` is Host-native by definition. OAW supplies only a Dispatch Packet
and referenced artifacts; the Host chooses and creates the child, controls its
environment and authority, and returns a normalized Receipt. If the current
Host session cannot create a native Subagent, `SUBAGENT` is ineligible. There
is no shell, CLI, container, remote-job, or clean-environment fallback.

Eligibility is the intersection of the selected Profile, every active
Capability binding, the Host integration, and current Host session. When both
topologies are eligible for Workflow work, the user selects one. When only one
is eligible, OAW records the reason and never invents another path.

Host environment observations use only these dispositions: `inherited`,
`host-configured`, `restricted`, `unknown`, or `unavailable`. OAW neither
reconstructs nor guarantees unreported MCP, Hook, Skill, Plugin, model,
authentication, sandbox, approval, or tool behavior.

## Workflow Coordinator

The Workflow Coordinator is optional durable state for `WORKFLOW` only. For
cooperating clients it owns:

- immutable Workflow revisions and the current revision pointer;
- idempotency-key replay and conflict rejection;
- active Lifecycle Bundle generation and digest;
- current graph node, ticket, stable boundary, and logical Capability Grant;
- cooperative Resource Leases for conflicting project mutations;
- normalized Receipt and evidence references;
- pause, cancellation, uncertain execution, switching, and recovery state;
- validation of legal state transitions.

The Coordinator does not classify requests, discover Providers, compile
Profiles, create Agents, invoke Skills, call tools, mutate files, perform Git
operations, execute models, enforce a sandbox, store credentials, or prevent
Host actions outside the protocol. A Capability Grant or Resource Lease may
narrow what a cooperating client should do, but it is not an operating-system
security boundary.

Policy-only Hosts may coordinate the same ownership model with a local lock,
but they make no claim of atomic revisions, leases, idempotency, transition
enforcement, or physical containment.

## Agent Host Integration

Host integrations expose one of two control surfaces:

| Surface | Contract |
| --- | --- |
| `policy` | Instruction distribution only. The Host supports `CURRENT`; coordination obligations are policy-level and no Coordinator guarantee is implied. |
| `host-native` | The Host reports session facts, calls OAW Core or the Workflow Coordinator, executes Dispatch Packets, and returns Receipts. |

A `host-native` integration must support `CURRENT`. `SUBAGENT` support is
optional and session-dependent. Static metadata cannot prove that a child is
available in the active session.

The Host may report a secret-free session identity, supported topologies,
Provider binding inventory digest, Receipt and evidence behavior, environment
observations, and opaque sandbox or approval policy digests. It never gives
OAW a model command, credential, private Hook payload, or private MCP, Skill,
or Plugin configuration.

The Agent Host owns physical execution authority. Dispatch Packets, Grants,
Resource Leases, and Lifecycle Bundles are logical coordination records. The
Host validates them before execution, performs every effect, and reports the
outcome without transferring physical authority to OAW.

## Lifecycle Bundle and Lock

The Lifecycle Bundle is the immutable Core output for a selected Workflow. It
records request and deliverable identity, classification, selected Profile and
selection source, topology and selection source, Bundle generation and digest,
stage owners, exact add-ons, active graph and terminal gates, allowed effects
and resources, and evidence requirements.

The lifecycle lock projects the active Bundle and progress: current stage,
active ticket, allowed and blocked actions, outstanding lease or Grant
references, and canonical artifact or evidence references. It cannot expand
Bundle authority.

The lock persists across follow-ups, context compaction, and delegated work on
the same deliverable. A dispatched child receives the exact Bundle generation,
active graph node, admitted Capability, effects, resources, termination
condition, and evidence requirements. It does not reopen Profile arbitration.
A new unrelated request is classified again and inherits no authority from the
old Workflow.

Coordinator Workflow State is authoritative for cooperating coordinated
clients. A Markdown lock is a non-authoritative policy projection and cannot
grant physical authority.

## Matt-Superpowers Hybrid Operational Notes

`MATT-SP-HYBRID` is the fixed built-in composition in the matrix above, not a
compatibility alias. Matt specifications and tickets are canonical for domain
intent and delivery edges. Superpowers planning may add exact paths, commands,
code steps, and expected results without changing those edges.

Matt `tdd` is the only active TDD procedure. One inline Superpowers executor
owns implementation; standalone Superpowers review/remediation, fresh
verification, and closeout remain active. An expected RED belongs to TDD. An
unexpected functional, hard-bug, or performance failure routes to Matt
`diagnosing-bugs`; a build, dependency, or type incident stops unless the user
selected an exact ECC Incident Handler Add-on at the Startup Gate.

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

## Artifacts and Projections

Use the project's existing documentation layout. A local policy-only tracker
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

Coordinator-backed project Workflow documents are one-way,
non-authoritative projections of committed Workflow State. They may show the
selected Profile, topology, Bundle generation, stage, active ticket,
digest-pinned evidence references, and projection lag status. They exclude
credentials, full Grants, sensitive evidence content, and raw Provider output.
They are never parsed back as authority or control input.

Do not create duplicate design specifications or implementation plans for the
same owner and ticket.

## Stable Switching

Only the user may switch a selected Workflow Profile or topology. Switch at an
approved specification, between completed tickets, after a completed TDD or
debugging cycle, after review, or after recorded verification. Do not switch
during an active Capability invocation, delegated work, unresolved merge, or
incomplete red-green cycle.

A switch compiles a new immutable Bundle generation, revokes outstanding
Grants from the old generation, and preserves valid artifacts and evidence. It
does not rewrite completed ownership, silently replace a Provider, or emulate
an unavailable topology.

## Neutral Safety Rules

- Observe fresh verification output before claiming completion.
- Preserve unrelated user changes.
- Do not perform destructive Git or filesystem operations without approval.
- Diagnose root causes before bug fixes.
- Treat network tools as read-only unless the user authorizes mutation.
- Validate inputs at system boundaries and never embed credentials in policy,
  Workflow State, project projections, logs, Receipts, or evidence references.
- Detection reports capabilities; it never selects a Capability or Profile.
- A Host Receipt reports an outcome; it does not prove authority beyond the
  facts the Host can attest.
