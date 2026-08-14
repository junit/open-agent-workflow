# Request Modes, Profiles, and Lifecycle Locking

[简体中文](../zh/lifecycle.md) | [README](../../README.md)

This guide explains Open Agent Workflow (OAW) behavior. It is not a second
policy. `policy/ENGINEERING.md is normative`; if this guide differs from the
[canonical policy](../../policy/ENGINEERING.md), the policy wins.

## Activation and Assurance

Native Host is the default. Installation, task complexity, automatic Host Skill
selection, and direct invocation of an ordinary Skill do not activate OAW. A
current top-level user instruction such as `/oaw <task>` creates one OAW
Engagement for one deliverable; related follow-ups inherit it and unrelated
deliverables remain Native Host behavior.

Every activated Engagement starts with Assurance Preflight:

```text
Native Host unless explicitly activated
    -> Assurance Preflight
    -> DIRECT / BOUNDED / WORKFLOW
    -> policy-cooperative / core-backed / coordinator-backed
```

Request Mode describes the activated work contract. Assurance Level separately
describes which claims the current Host integration can support:

| Assurance Level | Available contract |
| --- | --- |
| `policy-cooperative` | Cooperative Assessment, Host-visible candidates, Policy Workflow Plan, Progress Tracker, Execution Notes, and Conflict Warnings. It cannot claim Core or Coordinator records. |
| `core-backed` | Core classification, Host-verified Provider resolution, reason-coded eligibility, explicit selection preview, and immutable Lifecycle Bundle. |
| `coordinator-backed` | All `core-backed` claims plus durable revisions, Grants, cooperative Leases, normalized Receipts, transitions, and recovery state. |

Installed files, Provider descriptors, or Bridge installation alone do not
raise assurance. Machine-backed levels require current Host-native session
evidence.

## Three Request Modes

Request Modes exist only inside an active OAW Engagement. Native Host is not a
Request Mode.

| Request Mode | Execution contract | Lifecycle selection |
| --- | --- | --- |
| Direct Mode (`DIRECT`) | The Main Agent makes a small, clear, recoverable change and runs focused verification. | None |
| Bounded Mode (`BOUNDED`) | One exact Provider Capability produces one observable terminal deliverable. | None |
| Workflow Mode (`WORKFLOW`) | A compiled Profile Recipe coordinates multiple responsibilities and stages. | Required |

Direct Mode requires a known change point, clear requirements, bounded scope,
no unresolved architecture or domain decision, no public contract or sensitive
semantic change, and a known verification boundary. It creates no Capability,
Profile, Lifecycle Bundle, Startup Gate, or Workflow State.

Activated Bounded Mode is not normal Host Skill routing. It admits one selected
Capability with declared effects, resources, evidence, and one terminal
condition for one named observable deliverable. If the activated request names
an exact Skill or Capability, selection is `user-explicit`. Otherwise OAW shows
one Host-visible candidate and stops with `CAPABILITY_SELECTION_REQUIRED` until
the user confirms. It cannot claim planning, implementation ownership, general
review, a remediation loop, Git completion, or a lifecycle stage. A required
second Capability or broader responsibility triggers reclassification.

Workflow Mode covers unresolved requirements or root cause, domain and
architecture decisions, interacting engineering responsibilities, public
contracts, schemas, dependencies, migrations, sensitive changes, multiple
tickets, and long-lived delegation.

Only Workflow Mode runs the Startup Gate. Complexity and Risk Class tune
recommendations and verification, but they do not activate OAW. `DIRECT` and
`BOUNDED` do not create Workflow State.

## Workflow Selection Gates

At `core-backed` or `coordinator-backed`, the machine Startup Gate:

1. reads the canonical policy;
2. states Assurance Level, Request Mode, Complexity, Risk Class, and evidence;
3. resolves verified Provider Instances through OAW Core;
4. shows every eligible built-in and user-defined Profile, eligible `CURRENT`
   or native `SUBAGENT` topology, recommendations, exclusions, and proposed
   bounded add-ons;
5. waits for a blocking user choice with no timeout or silent default; and
6. has OAW Core compile and record the immutable Lifecycle Bundle.

At `policy-cooperative`, the Cooperative Selection Gate shows Host-visible
Profile candidates and states the unavailable machine guarantees. It also
states that execution remains in the current Host session and uses no add-on
unless the user explicitly requested one with a route-level contract. It waits
only for explicit Profile selection: these fixed execution facts and the
already-declared limitations are not extra choices. It then creates a
cooperative Policy Workflow Plan and Progress Tracker before lifecycle work.
An unverified candidate is never called an eligible Profile, and static
instruction context cannot prove `SUBAGENT` availability.

Provider detection is diagnostic input. It never chooses a Capability, Profile,
or topology. Missing or ambiguous required Capabilities stop selection rather
than being silently omitted or replaced.

### No-Bridge policy-cooperative CLI path

The reference CLI provides a real Codex `CURRENT` path without installing the
Bridge. It remains `policy-cooperative`: the CLI inspects files visible to the
Host, persists a cooperative snapshot, and checks tracker transitions, while
the current Agent Host invokes Skills and performs every effect.

Start with a fresh Governance inspection:

```bash
oaw profiles
```

Each built-in Profile is reported as `policy_selectable` and independently as
`host_routable`, with `missing` naming any required routes that are not
callable. `policy_selectable` means that OAW defines the Profile semantics;
`host_routable` means that every required route currently resolves to a
Host-visible Skill, a user-explicit Skill, or a neutral Host action. Route
inventory remains internal to the Policy module. Each public Profile also
includes `incident_routes`, which reports conditional handlers as
`routable-if-triggered` or `unavailable-if-triggered`. An unavailable
conditional handler does not make the normal Profile incomplete.

For Codex, cooperative discovery recognizes current plugin layouts including
the ECC cache below `.codex/plugins/cache/ecc/ecc/<version>` and the curated
Superpowers cache below
`.codex/plugins/cache/openai-api-curated/superpowers/<version>`. Matt discovery
uses regular `.agents/skills/<name>/SKILL.md` files. Human-command Matt Skills
are `user-explicit` and yield `AwaitUserSkill`. Policy inspection does not read
Matt lockfiles, source identity, revisions, tree hashes, Provider evidence, or
Bridge state. Those checks remain available only on the machine-backed path.
ECC uses public Codex Skill routes where their contracts fit the responsibility;
it does not require a Claude Agent, Codex Role, or slash-command instruction.
Because ECC's public `review-pr` command requires a PR, generic Policy review
uses the typed neutral Host action `review.execute`. Machine-backed execution
may still use an exact verified ECC reviewer Binding.

When all required Host routes are present, `SP-FULL`, `MATT-FULL`,
`ECC-FULL`, and `MATT-SP-HYBRID` can each be selected and carried through the
policy-only `CURRENT` lifecycle without a Bridge. Availability is inspected,
not promised merely from the Provider name or installation claim.

The user then starts the project Engagement with one explicit Profile:

```bash
oaw use --profile MATT-SP-HYBRID --complexity complex --risk normal -- "Typora-like editor"
```

`use` atomically re-inspects the routes, validates the Profile, records the
complexity and risk already reported by the Cooperative Assessment, and stores
the first reducer state. It does not invent assessment defaults or invoke the
machine classifier. The current session, default absence of an add-on, Policy
limitations, project identity, and opaque references are internal facts rather
than repeated confirmation fields. The project-level Policy CLI has no
`--add-on` option or `NONE` sentinel; a future bounded add-on requires its own
explicit route-level contract. Running `oaw use -- "deliverable"` without a
Profile prints the candidates and stops for explicit Profile selection; the
assessment flags are required only when a Profile is selected.

After the Host completes the exact current work or gate, use the matching
business command:

```bash
oaw complete         # current ordinary Skill or neutral Host action
oaw review clean     # current review produced no findings
oaw review findings  # current review requires remediation and re-review
oaw approve          # current user gate
oaw satisfy          # current Host gate
oaw status
```

Each command is accepted only for its matching current work. Review-producing
work requires an explicit `clean` or `findings` outcome; ordinary `complete`
cannot silently claim a clean review. Findings return to implementation and
force a fresh review. The reducer selects every next route, neutral action,
gate, incident return, and stable switch. Completion occurs only when it
returns `Done`. Incident, switch, stop, and uncertain execution remain typed
actions:

```bash
oaw incident build-failure --reason "compiler failed"
oaw switch SP-FULL
oaw stop --reason "user stopped"
oaw uncertain --reason "Host result unknown"
```

Policy Engagement files live under
`${XDG_STATE_HOME:-$HOME/.local/state}/open-agent-workflow/policy-engagements`.
The CLI derives an internal identity from the physical current project; users
do not select a run ID or state root. Local locking and replacement support
reload after a process restart, but they do not create Coordinator revision,
idempotency, Lease, Receipt, or recovery guarantees.

`use` stores an exact reducer snapshot and `status` renders a public view that
omits the internal ID, route inventory, reducer snapshot, and opaque
references. Every event re-inspects routes; material changes reject
route-dependent completion and incident events with
`ROUTE_INVENTORY_DRIFT`. After the routes are repaired, `oaw switch PROFILE`
performs its own fresh inspection at a stable boundary; users never obtain or
submit an Offer reference. `stop` and `uncertain` remain available so drift
cannot prevent a terminal safety record. Lockfile, revision, digest, path, or
Bridge changes that preserve the same route inventory do not affect the active
Engagement.

Policy-cooperative execution uses these stable stop reasons:

| Reason | Recovery |
| --- | --- |
| `CAPABILITY_SELECTION_REQUIRED` | Select the Bounded candidate explicitly. |
| `POLICY_ONLY_PROVIDER_UNVERIFIED` | Use a machine-backed integration or remove the verified-Provider requirement. |
| `POLICY_ONLY_PROFILE_INCOMPLETE` | Supply a visible owner for every required responsibility or choose another complete candidate. |
| `POLICY_ONLY_TOPOLOGY_UNAVAILABLE` | Select `CURRENT` or exit OAW. |
| `POLICY_ONLY_GUARANTEE_UNAVAILABLE` | Remove the Grant, Lease, Receipt, idempotency, atomic revision, or recovery requirement, or use machine-backed assurance. |
| `POLICY_ONLY_CONCURRENT_MUTATION` | Stop or serialize overlapping project or Git mutation. |
| `POLICY_ONLY_EXECUTION_UNCERTAIN` | Reconcile an uncertain external or destructive effect; do not retry blindly. |
| `POLICY_ONLY_CONTEXT_UNCERTAIN` | Reconfirm selection and progress instead of reconstructing authority. |

## Provider and Capability Model

Superpowers, Matt, ECC, and third-party Providers follow the same descriptor,
binding, verification, and compiler contract. OAW ships inert descriptors for
`oaw/superpowers`, `oaw/matt`, and `oaw/ecc`; it does not install Provider
skills. Built-in Provider discovery remains dynamic on the current Host.

Provider verification follows this exact chain:

```text
Provider Family
  -> Distribution
  -> Host Installation
  -> Host Binding Evidence
  -> Verified Provider Instance
```

Codex and Claude Code are independent Hosts, so shared physical files still
produce separate Host Installation identities. Descriptor bindings and
installation hints are declarations only. Foreign diagnostics never enter
pins, Profile compilation, or a Lifecycle Bundle.

Users can register trusted third-party Providers, discovery descriptors,
Profile Recipes, bindings, pins, and denials. Trusted project configuration may
recommend or narrow those records but cannot create user trust or expand
authority. Only a verified Provider Instance can satisfy a Recipe Capability
selector.

A Provider's role is Recipe-specific. One Profile may use ECC for a complete
lifecycle; another may admit the same Provider only for build repair or a
security review. Full-family eligibility is based on verified Capability
coverage, using the same rule for built-in and user-registered Providers.

### Inspecting Provider Resolution

`oaw catalog list providers` shows declared Provider descriptors. Inspect
dynamic discovery for a selected Host with:

```bash
oaw providers inspect --host codex --format text
```

An ambiguous current-Host Provider lists every Candidate and an exact pin:

```toml
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-<sha256>"
evidence_digest = "<sha256>"
# location = "/exact/physical/path"
# version = "6.1.1"
```

OAW never chooses a Candidate or writes the pin. Active contracts reject
`oaw.provider-descriptor/v1` and `oaw.user-config/v1` rather than migrating
them. Start a new Workflow after changing user configuration so OAW Core sees
the new Configuration Snapshot.

## Built-in and User-Defined Profiles

The four aliases remain active catalog entries. A current-Host exclusion does
not remove an alias and there is no old-schema fallback.

| Selection | Recipe | Contract |
| --- | --- | --- |
| `MATT-FULL` | `oaw/domain-engineering` | Matt-led lifecycle with explicit neutral Host/user controls for Matt's exact gaps. |
| `SP-FULL` | `oaw/delivery` | Complete inline Superpowers delivery path. |
| `ECC-FULL` | `oaw/ecc-engineering` | ECC-led path over exact Host-surface alternatives. |
| `MATT-SP-HYBRID` | `oaw/reliable-feature` | Preserved built-in Matt/Superpowers composition. |
| `USER-DEFINED` | Configured Recipe ID | Selection action for a trusted, versioned custom Recipe, not a fifth built-in alias. |

`FULL` means a Provider-led lifecycle plus Provider-neutral Host actions and
user/Host gates. It never means Provider ownership of the Agent Host. A Recipe
compiles only when every applicable slot has exactly one outcome owner and all
Bindings, topology, delegation, invocation, actions, authority, effects,
resources, transitions, and terminal gates verify.

### Canonical ten-slot lifecycle

| # | Slot ID | Outcome and neutral control |
| --- | --- | --- |
| 1 | `problem-framing` | Purpose, constraints, domain terms, decisions, success conditions; `shared-understanding` gate. |
| 2 | `solution-specification` | Reviewable specification and test boundaries; `specification-approved` gate. |
| 3 | `delivery-planning` | Verifiable units and executable plan; `delivery-plan-approved` gate. |
| 4 | `workspace-preparation` | Safe workspace and known baseline; `workspace.prepare-or-confirm` action and `workspace-ready` gate when Host-owned. |
| 5 | `implementation` | Approved bounded changes. |
| 6 | `implementation-tdd` | Witnessed expected RED/GREEN cycle. |
| 7 | `incident-recovery` | Conditional typed recovery, replan, or stop. |
| 8 | `review-remediation` | Findings adjudicated, fixed, and re-reviewed. |
| 9 | `fresh-verification` | Fresh claim-relevant output; `verification.execute` action when Host-owned and `fresh-evidence` gate. |
| 10 | `closeout` | Accepted and user-authorized delivery/preservation; `closeout.execute` action when Host-owned and `user-closeout` gate. |

Host actions and gates contain no Provider selector. A macro's `credit-only`
calls count work already performed by its enclosing unit. `dispatch-before`
and `dispatch-after` calls run once at their declared edge. Duplicate or
uncredited ownership fails with `MACRO_INTERNAL_CONFLICT`.

### Exact built-in matrix

| Slot | `MATT-FULL` | `SP-FULL` | `ECC-FULL` | `MATT-SP-HYBRID` |
| --- | --- | --- | --- | --- |
| `problem-framing` | `grill-with-docs` (`grilling` + `domain-modeling` credited once) | `superpowers:brainstorming` | Codex `intent-driven-development` skill or exact Claude `architect` Agent | Matt `grill-with-docs` with credited internal calls |
| `solution-specification` | `to-spec` | enclosing brainstorming outcome | `product-capability`; conditional `contract-first` does not become a peer owner | Matt `to-spec` |
| `delivery-planning` | `to-tickets` | `superpowers:writing-plans`, once after brainstorming | observed Codex `/plan` Instruction or `blueprint`; Claude `planner` Agent or `blueprint` | Matt `to-tickets`, then SP `superpowers:writing-plans` for file/command detail |
| `workspace-preparation` | Host `workspace.prepare-or-confirm` | `superpowers:using-git-worktrees` | Host action; `git-workflow` is guidance | SP `superpowers:using-git-worktrees` |
| `implementation` | `implement` macro | inline `superpowers:executing-plans` | `tdd-workflow` or exact Claude `tdd-guide` alternative | inline SP `superpowers:executing-plans`; SDD paused |
| `implementation-tdd` | `implement` credits `tdd` once | `superpowers:test-driven-development` | same selected implementation/TDD unit | Matt `tdd`; SP TDD paused |
| `incident-recovery` | `diagnosing-bugs` for functional/hard/performance incidents; otherwise stop | `superpowers:systematic-debugging` typed route | only a verified typed route; Claude `build-error-resolver` may handle build/type/dependency | Matt `diagnosing-bugs`; build/type/dependency stops unless an ECC handler Add-on was selected |
| `review-remediation` | `implement` credits `code-review`; remediation re-enters bounded `implement` and re-reviews | `superpowers:requesting-code-review` -> `superpowers:receiving-code-review` -> re-review | Policy: typed Host `review.execute`, with findings returning to `tdd-workflow`; machine-backed: exact Codex `reviewer` Role or Claude `code-reviewer` Agent | SP request/receive/re-review; Matt review and SDD review paused |
| `fresh-verification` | Host `verification.execute` | `superpowers:verification-before-completion` | `verification-loop`; E2E surfaces remain specialist-only | SP `superpowers:verification-before-completion` |
| `closeout` | Host `closeout.execute` | `superpowers:finishing-a-development-branch` | Host action with `git-workflow` guidance | SP `superpowers:finishing-a-development-branch` |

Matt ships `grill-with-docs`, not a made-up requirements or verification Skill.
It does not supply workspace creation, broad fresh verification, or completion.
ECC Skills, Claude custom Agents, Codex Roles, Instructions, Hooks, and tools
remain different surfaces. A Claude Agent name cannot prove a Codex Role;
static multi-agent configuration cannot prove live delegation; `e2e-runner`,
`e2e-testing`, reviewers, and delivery Hooks do not acquire broader ownership.

A `USER-DEFINED` Recipe can combine installed, trusted, Host-verified compatible
Bindings freely per slot. It must pin its version and sources, declare exact
outcome owners, alternatives, overlays, incident routes, internal calls,
actions, gates, effects, resources, and termination conditions, and fail closed
on every gap. A user-defined SDD variant may select
`superpowers:subagent-driven-development` only when the active Host proves the
required child or nested-child delegation.

## Execution Topologies

The Startup Gate uses two topology names:

| Topology | Contract |
| --- | --- |
| `CURRENT` | Execute in the current Agent session with the active Agent Host environment unchanged. |
| `SUBAGENT` | Ask the active Agent Host to create a child through its native Subagent facility. |

Topology eligibility is the intersection of the selected Profile, Capability
bindings, Host integration, and active Host session. Missing native Subagent
support makes `SUBAGENT` unavailable; OAW never substitutes a shell, model CLI,
container, or remote process. When both are eligible, the user chooses. When
only one is eligible, OAW records the reason.

Host environment observations use `inherited`, `host-configured`, `restricted`,
`unknown`, or `unavailable`. OAW does not reconstruct or guarantee unreported
MCP, Hook, Skill, Plugin, model, authentication, sandbox, approval, or tool
behavior.

## OAW Core, Coordinator, and Agent Host

The cooperative Policy path does not require OAW Core. For machine-backed
assurance, OAW Core is required and stateless. It owns classification,
Host-scoped Provider resolution, eligibility, Profile compilation, and
Lifecycle Bundle construction. Callers never author a Bundle.

The Workflow Coordinator is optional and Workflow-only. It records revisions,
idempotency, cooperative Resource Leases, Receipts, evidence, pause,
cancellation, switching, and recovery for cooperating clients. It does not
create Agents, execute models, invoke Skills, use tools, or enforce the Host
sandbox.

The Agent Host owns physical execution authority. A Lifecycle Bundle,
Capability Grant, or Resource Lease expresses logical workflow authority only.
Codex has a policy integration by default and a separate audited host-native
Bridge at `oaw/codex-host` that must be explicitly installed and trusted. The
Bridge supports `CURRENT` and `skill` bindings only; all other Host surfaces
remain unknown unless the Host reports stable evidence.

For a Codex Host-native Workflow, the evidence path is:

```text
observe_current -> Core inspect -> explicit Startup Gate
                -> Core compile / Coordinator START
                -> current Codex session executes Skills and tools
```

Other built-in integrations remain policy surfaces unless their own Host-native
integration is explicitly installed and verified. None of these logical
records transfer physical execution authority from the Agent Host.

## Matt-Superpowers Composition Notes

The matrix above is the authoritative `MATT-SP-HYBRID` projection. Matt
specifications and tickets remain canonical for domain intent and delivery
edges; the Superpowers plan adds executable detail without changing them.
Matt `tdd` remains the only TDD procedure, one inline Superpowers executor owns
implementation, and standalone Superpowers procedures own review/remediation,
fresh verification, and closeout. An ECC build/type handler is absent unless
the user explicitly selected that exact bounded Add-on.

## Lifecycle Lock, Inheritance, and Add-ons

The lifecycle lock records task and deliverable identity, classification,
selected Profile and topology, selection sources, Bundle generation and
digest, stage owners, exact add-ons, active stage and ticket, allowed and
blocked actions, and canonical artifact or evidence references.

The lock persists across follow-ups, context compaction, and delegated work.
Exact bundle inheritance gives a Host-native Subagent the active Bundle
generation, graph node, admitted Capability, effects, resources, terminal
condition, and evidence requirements. The child cannot reopen Profile
arbitration or add a second owner. For multi-ticket work, ticket inheritance
keeps the same Bundle until the user switches at a stable boundary.

bounded add-ons authorize one exact specialist deliverable. For example,
`ECC(security-review)` may return a digest-pinned report, but it does not own
implementation, general review, Git, or completion. Security and coverage
requirements are constraints, not lifecycle selections.

## Workflow State and Projections

Coordinator-backed project Workflow files are human-readable, one-way views of
committed Workflow State. A projection may contain selected Profile and
topology, Bundle generation, stage, active ticket, digest-pinned evidence
references, and lag status. It excludes credentials, full Grants, sensitive
evidence, and raw Provider output.

Projection files are never parsed back as authority. A projection write failure
records lag and does not roll back the committed revision. Policy-cooperative
work instead uses a human-readable Policy Workflow Plan and best-effort Progress
Tracker. Neither is a Lifecycle Bundle, Lifecycle Lock, or Workflow State, and
neither can grant physical execution authority.

## Stable Switching

The stable switching rules below govern every Workflow Profile and topology
change.

Only the user can change a selected Workflow Profile or topology. stable
switching is allowed at an approved specification, between completed tickets,
after a completed TDD or debugging cycle, after review, or after recorded
verification. A stable-boundary switch is not allowed during an active
Capability invocation, delegated work, unresolved merge, or incomplete
red-green cycle.

A switch compiles a new Bundle generation, revokes outstanding Grants from the
old generation, and preserves valid artifacts and evidence. It never rewrites
completed ownership, silently substitutes a Provider, or emulates an
unavailable topology.

## Complete Workflow Example

A repository needs a multi-ticket installer with path containment, recoverable
force, bilingual docs, and a final security assessment. The public behavior,
filesystem risk, several tickets, and remediation loop classify it as Workflow
Mode. OAW recommends the hybrid, `CURRENT`, and one bounded add-on.

The user makes this explicit selection:

```text
MATT-SP-HYBRID + ECC(security-review)
```

OAW Core compiles the Bundle. Matt owns requirements, specification, tickets,
TDD, and functional debugging; Superpowers owns executable plans,
implementation, review, remediation, verification, and completion; ECC owns
only the security report. The Agent Host executes the work and returns
Receipts. The Workflow Coordinator is optional.

After a ticket is verified, the user may make a stable-boundary switch to
another eligible Profile or topology. Until that explicit choice, the original
Bundle remains locked.

The [background](background.md) explains the motivation, and the
[comparison](comparison.md) records the experience-based inputs behind the
initial hybrid.
