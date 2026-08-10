# OAW Provider Surface and Composable Profile Contract v4 Design

**Date:** 2026-08-10
**Status:** Approved for implementation planning
**Request mode:** WORKFLOW
**Complexity:** Complex
**Risk:** Elevated
**Selected lifecycle for this design:** `SP-FULL / CURRENT / no Add-on`
**Current workflow:** `workflow-88833e4868d3202599342e2c5c133f82`
**Current bundle:** `bundle-baae41622fe7315071afa00c957be008`

## 1. Decision Summary

OAW will replace the current Provider and Profile contracts with a hard-cut
v4 model built around a stable, Provider-neutral engineering lifecycle and
verified, composable Stage Bindings.

The design preserves all four core selections:

- `MATT-FULL`
- `SP-FULL`
- `ECC-FULL`
- `MATT-SP-HYBRID`

It also makes `USER-DEFINED` a first-class way to build additional Profiles by
combining verified installed Bindings at each lifecycle slot.

The central correction is that lifecycle slots and Provider Bindings are not a
one-to-one relationship:

- one Binding may span several slots;
- one slot may contain an ordered pipeline of several Bindings;
- a macro may call other Bindings internally;
- TDD is an implementation procedure, not a second implementation owner;
- debugging is a conditional incident route, not a mandatory mainline phase;
- neutral evidence and user-authority gates are not Provider skills; and
- Agent, Role, Instruction, Hook, Tool, and Skill surfaces are distinct.

The compiler expands macros, validates every pipeline edge, selects exactly one
outcome owner per applicable slot, and rejects missing or conflicting coverage.
It never invents a skill, attributes one Provider's skill to another Provider,
or treats installation as proof that the current Host can invoke a Binding.

There is no compatibility reader for authority-bearing v3 Provider descriptors,
v2 Profile Recipes, or Bundles compiled from them. Existing state remains as
audit history only.

## 2. Evidence Baseline

The design is based on two read-only audit reports:

- `.scratch/oaw-provider-profile-audit/upstream-skill-audit.md`
- `.scratch/oaw-provider-profile-audit/canonical-profile-stage-matrix.md`

Canonical matrix digest:

`49ec1819ab22364d763d0875d9af299ee332de3d6d39a7178a715c2b13272ccf`

The initial v4 built-ins are pinned to these immutable upstream revisions:

| Provider | Repository | Revision |
| --- | --- | --- |
| Matt | `https://github.com/mattpocock/skills` | `84fdeffd12f2ee307994d1eb6feb48173b6e0502` |
| Superpowers | `https://github.com/obra/superpowers` | `44c9b2d6e889982ac18c27d05a19fefe335194e1` |
| ECC | `https://github.com/affaan-m/ECC` | `2d46e80e0925c7be0907f18c1812311ac212a6c5` |

Confirmed facts that drive the design:

1. Matt has no `requirements` or `verification-loop` skill at the pinned
   revision. `grill-with-docs` is the requirements/domain alignment wrapper;
   it calls `grilling` and uses `domain-modeling`.
2. Matt `to-spec` synthesizes decisions already made. It explicitly does not
   perform a new interview.
3. Matt `implement` calls `tdd` and `code-review`, commits locally, and
   explicitly has no completion step or built-in remediation loop.
4. All audited Superpowers skill names exist, but SDD and review require native
   child delegation. A top-level child needs nested delegation.
5. ECC ships real Skills, Claude custom Agents, Codex Roles, Instructions, and
   Hooks. These are different Host surfaces and cannot be substituted by name.
6. ECC `e2e-runner` is not broad verification, `code-reviewer` is not
   completion, and `delivery-gate` is a Claude Stop Hook rather than a Git
   delivery procedure.
7. Current Codex Bridge v1 attests `skill` Bindings and `CURRENT` only. It does
   not attest reviewer delegation, nested delegation, Codex Roles, Claude
   Agents, or neutral Host actions.

## 3. Goals and Non-Goals

### 3.1 Goals

- Preserve the four core Profile families without preserving invalid recipes.
- Define one stable user-visible lifecycle for built-in and user-defined
  Profiles.
- Support N:M slot-to-Binding pipelines and macro spans.
- Prove exact Provider provenance and immutable Binding content.
- Separate outer topology from child and nested-child requirements.
- Model Host actions and neutral gates without pretending they are Provider
  skills.
- Distinguish Skill, Agent, Role, Instruction, Tool, and Hook Bindings.
- Allow users to clone a built-in template or start a new Profile and replace
  compatible slot pipelines.
- Make every compile decision and exclusion reason observable.
- Lock the user-confirmed resolved graph into an immutable Lifecycle Bundle.
- Reject all stale authority instead of silently converting it.

### 3.2 Non-Goals

- Installing or modifying Matt, Superpowers, or ECC.
- Making every installed skill eligible for every lifecycle slot.
- Inferring semantic compatibility from a skill name.
- Inferring child delegation from static Host configuration.
- Treating OAW Core or the Coordinator as a model or command executor.
- Giving neutral gates authority to produce missing engineering work.
- Making a Provider `FULL` by silently borrowing another Provider's method.
- Preserving v3 Provider descriptors, v2 Recipes, or their compiled Bundles.
- Pushing, publishing, merging, or changing user credentials/configuration as
  part of this migration.

## 4. Alternatives

### 4.1 Selected: Stable Slots with N:M Verified Pipelines

OAW exposes ten stable lifecycle slots. Each concrete Recipe supplies an
ordered Binding pipeline, optional Host action, and neutral gates for each
applicable slot. Bindings may span contiguous slots and declare internal calls.

This preserves a comprehensible engineering lifecycle while allowing Matt,
Superpowers, ECC, and future Providers to express their real workflow shapes.

### 4.2 Rejected: One Skill per Stage

A one-to-one table duplicates macro internals and creates fictional Bindings.
Examples include running `domain-modeling` again after `grill-with-docs`,
running standalone review after SDD already reviewed each task, or assigning a
nonexistent Matt `verification-loop` to final verification.

### 4.3 Rejected: Unconstrained Free-Form DAG

A completely free graph is flexible but removes stable comparison, outcome
coverage, and safety gates. Profiles would no longer be meaningfully
comparable, and the compiler could not explain which lifecycle outcome is
missing.

## 5. Canonical Lifecycle Taxonomy

The ten entries are user-visible lifecycle slots. They are not ten mandatory
skill invocations.

| # | Slot ID | User-visible name | Machine kind | Required outcome |
| --- | --- | --- | --- | --- |
| 1 | `problem-framing` | Requirements and domain alignment | `stage` | Purpose, constraints, domain terms, decisions, and success conditions are user-aligned. |
| 2 | `solution-specification` | Solution specification and test boundaries | `stage` | A reviewable solution specification and test boundaries are approved. |
| 3 | `delivery-planning` | Delivery planning, decomposition, and acceptance items | `stage` | Work is decomposed into independently verifiable units sufficient for the selected executor. |
| 4 | `workspace-preparation` | Workspace preparation | `host-action + neutral-gate` | The selected workspace is safe, initialized, and has a known baseline. |
| 5 | `implementation` | Implementation execution | `stage` | Approved changes are produced with bounded effects and progress evidence. |
| 6 | `implementation-tdd` | TDD and implementation testing | `procedure` | Expected behavior drives a witnessed RED/GREEN cycle and focused tests. |
| 7 | `incident-recovery` | Conditional debugging and repair | `incident-handler` | A typed unexpected failure is diagnosed and returns to a declared stage, replans, or stops. |
| 8 | `review-remediation` | Review and remediation | `assurance-loop` | Findings are reported, fixed or adjudicated, and re-reviewed. |
| 9 | `fresh-verification` | Fresh final verification | `host-or-provider-procedure + neutral-gate` | Claim-relevant commands run after remediation and produce fresh evidence. |
| 10 | `closeout` | Completion and delivery | `terminal-sequence + user-gate` | Acceptance is reconciled and the user-authorized delivery or preservation action is recorded. |

The machine contract uses typed ownership namespaces:

- `stage.*` for one outcome owner;
- `procedure.*` for supporting methods;
- `incident.*` for conditional recovery;
- `assurance.*` for review/remediation loops;
- `host-action.*` for Host-executed neutral procedures; and
- `gate.*` for evidence or user-authority predicates.

Only one active outcome owner is allowed for each applicable slot. Supporting
procedures may be ordered N:M contributors. Neutral gates have no Provider
selector and do not participate in Provider ownership counts.

## 6. Audited Built-in Profile Matrix

Legend:

- `[S]` exact Skill
- `[A]` exact custom Agent
- `[R]` exact Host Role
- `[I]` exact Instruction surface
- `[M]` macro or internal call; do not dispatch it again as a peer
- `[H]` Host action or Host requirement
- `[N]` neutral evidence/user gate

| Slot | `MATT-FULL` | `SP-FULL` | `ECC-FULL` | `MATT-SP-HYBRID/default` |
| --- | --- | --- | --- | --- |
| 1. Requirements/domain | `[S/M] grill-with-docs`, internally `grilling + domain-modeling`; explicit human invocation | `[S/M] brainstorming`, one run spans slots 1-2 | `[S] intent-driven-development`; Claude may use `[A] architect/planner` when attested | Matt `grill-with-docs` |
| 2. Specification/test boundaries | `[S] to-spec`; synthesis only, followed by `[N]` approval | Same `brainstorming` run writes and gains approval for the design | `[S] product-capability`; conditionally `[S] contract-first` for shared contracts | Matt `to-spec` |
| 3. Planning/decomposition | `[S] to-tickets`; tracer-bullet tickets, blocking edges, and per-ticket acceptance | `[S] writing-plans`; file/command-level executable plan | Claude `[A] planner`; `[S] blueprint` only for its large multi-session trigger and supported Host | Matt `to-tickets ->` SP `writing-plans`; SP may add execution detail but not alter Matt requirements or edges |
| 4. Workspace | `[H] workspace.prepare-or-confirm + [N] workspace-ready`; no Matt workspace skill | `[S/M] using-git-worktrees`; execution macros reuse its result | `[S] git-workflow` is guidance only, plus `[H/N]` workspace readiness | SP `using-git-worktrees` |
| 5. Implementation | `[S/M] implement`, one ticket per explicit invocation; internally TDD/review and local commit | Alternative macro: `[S/M] subagent-driven-development` or `[S/M] executing-plans` | `[S] tdd-workflow` or Claude `[A] tdd-guide`; matching `orch-*` macro only when all internal dependencies verify | SP `executing-plans` is the default inline executor |
| 6. TDD | Matt `[S/M] tdd`, normally internal to `implement` | `[S] test-driven-development`, internal to the chosen execution path/plan | `[S] tdd-workflow` or Claude `[A] tdd-guide` | Matt `tdd` only; SP TDD ownership is disabled |
| 7. Incidents | `[S] diagnosing-bugs` for functional/hard-bug/performance incidents; other types stop | `[S] systematic-debugging` for typed technical failures | Regression route via `tdd-workflow`; Claude `[A] build-error-resolver` for build/type/dependency failures | Matt `diagnosing-bugs`; build/type/dependency defaults to stop because no Add-on is selected |
| 8. Review/remediation | `[S/M] code-review` reports Standards and Spec axes; findings re-enter `implement` as an explicit remediation packet, followed by a fresh standalone review | SDD internal review loop, or standalone `[S] requesting-code-review -> receiving-code-review -> re-review` | Claude `[A] code-reviewer` or Codex `[R] reviewer`; remediation must be separately bound | SP standalone request/receive/re-review loop |
| 9. Fresh verification | `[H] verification.execute + [N] fresh-evidence`; no Matt `verification-loop` | `[S] verification-before-completion + [N] fresh-evidence` | `[S] verification-loop + [N] fresh-evidence`; E2E is only an optional specialist check | SP `verification-before-completion` |
| 10. Closeout | Matt's earlier local commit plus `[H] closeout.execute + [N] acceptance/user-authority`; no Matt completion skill | `[S/M] finishing-a-development-branch + [N] user-authority` | `[S] git-workflow` guidance plus `[H/N]` closeout; `delivery-gate` is not the owner | SP `finishing-a-development-branch + [N] user-authority` |

## 7. Profile Semantics

### 7.1 Meaning of FULL

`FULL` means one Provider-led methodology owns every Provider-owned canonical
outcome and procedure selected by its Recipe, combined with mandatory neutral
Host actions and OAW/Host/User gates.

It does not mean the Provider owns the Host, user authorization, or neutral
evidence predicates. A Host action does not turn `MATT-FULL` into a hybrid.
Adding an ECC or Superpowers Provider procedure to fill a Matt engineering slot
does turn the result into a hybrid or user-defined Profile.

A FULL Profile is eligible only when all mandatory Provider Bindings, Host
actions, Host features, artifacts, and gates compile. Keeping an alias does not
make it eligible on every Host.

### 7.2 MATT-FULL

`MATT-FULL` and `oaw/domain-engineering` remain active catalog identities, but
the invalid v2 Recipe is replaced rather than supported.

The corrected Recipe is Matt-led:

```text
grill-with-docs
  -> user-alignment gate
  -> to-spec
  -> spec-approval gate
  -> to-tickets
  -> ticket-approval gate
  -> workspace.prepare-or-confirm
  -> implement(ticket)
       -> tdd
       -> code-review
       -> local commit
  -> fresh standalone code-review
  -> finding? explicit remediation packet -> implement -> re-review
  -> verification.execute
  -> fresh-evidence gate
  -> acceptance reconciliation and user closeout choice
```

Requirements:

- exact Matt Distribution provenance and Binding digests;
- explicit user invocation for `grill-with-docs`, `to-spec`, `to-tickets`, and
  each `implement` run;
- child delegation and parallel review children for `code-review`;
- nested parallel children if the top-level topology is `SUBAGENT`;
- attested Host workspace, verification, and closeout actions; and
- no build/dependency/type fallback unless the user selects an Add-on, in
  which case the resulting graph is no longer pure MATT-FULL.

### 7.3 SP-FULL

`SP-FULL` remains the complete Superpowers methodology. It has two legal
execution alternatives:

1. SDD macro: `subagent-driven-development` expands workspace, implementer,
   per-task review/remediation, final review, and branch finish.
2. Inline macro: `executing-plans` expands workspace and execution, while
   standalone Superpowers TDD, review/remediation, verification, and finish
   provide the remaining outcomes.

The compiler must not schedule SDD's internal task/final reviews or finish as
additional peer invocations. `executing-plans` also calls branch finish; that
internal call is credited once.

Eligibility requirements:

- SDD under `CURRENT`: child implementer/reviewer delegation;
- SDD under top-level `SUBAGENT`: nested implementer/reviewer delegation;
- inline path: no implementer child is required, but
  `requesting-code-review` still requires a reviewer child;
- top-level `SUBAGENT` review requires a nested reviewer; and
- missing reviewer delegation makes SP-FULL ineligible rather than silently
  converting review to self-review.

### 7.4 ECC-FULL

`ECC-FULL` remains `oaw/ecc-engineering`, with Host-specific pipeline variants
inside one Provider family:

- Skills use exact skill Bindings such as `intent-driven-development`,
  `product-capability`, `contract-first`, `tdd-workflow`,
  `verification-loop`, and `git-workflow`.
- Claude custom Agents use exact `agent` Bindings such as `architect`,
  `planner`, `tdd-guide`, `build-error-resolver`, and `code-reviewer`.
- Codex Roles use exact observed role identities. At the pinned revision the
  supplied roles are `explorer`, `reviewer`, and `docs-researcher`; Claude
  Agent names are not inferred as Codex Roles.
- Legacy commands are `instruction` Bindings only where a Host explicitly
  supports and observes that surface.
- Hooks may contribute Host evidence but never own engineering or closeout.
- `orch-*` macros compile only when every internal Agent/Instruction/Skill and
  both user gates are available on the current Host.

The current Codex Host v1 cannot compile ECC-FULL because it does not attest
the necessary Agent/Role/Instruction and remediation/closeout surfaces. The
alias remains; compilation reports exact missing Bindings.

### 7.5 MATT-SP-HYBRID

`MATT-SP-HYBRID` is a composable Profile family. OAW ships an immutable
`default` template matching the matrix above. It is not the only legal
Matt-Superpowers composition.

The default deliberately uses `executing-plans`, not SDD, so Matt can remain
the sole TDD owner and Superpowers review can run as a separate assurance
pipeline. It disables Superpowers TDD ownership and Matt general review after
the explicit Superpowers review pipeline takes over.

If a user selects SDD in a variant, the compiler must transfer SDD's internal
TDD and review responsibilities to Superpowers or reject the overlap. It may
not claim Matt TDD ownership while mandatory SDD internals still own the same
run.

Changing the default template does not mutate it. The Profile Builder saves a
new versioned concrete Recipe with family/template provenance. The Bundle pins
that exact Recipe and resolved graph.

The default has no ECC Add-on. A build/type/dependency failure stops and
escalates. An ECC handler appears only after an explicit user selection.

## 8. USER-DEFINED and Profile Builder Contract

Users may clone any built-in template or start from the canonical lifecycle.
For every slot they may choose zero or more ordered Bindings among candidates
that are installed, provenance-verified, semantically compatible, and callable
on the active Host.

An installed Binding that lacks a trusted descriptor can be used only after a
trusted user or project descriptor supplies its immutable source, semantic
contract, effects, and Host requirements. A name and directory path alone are
not enough.

Profile Builder behavior:

1. Observe the current Host and verified Provider Instances.
2. Show each canonical slot and its entry/outcome artifact contract.
3. List eligible Bindings by kind and Provider.
4. Gray out incompatible Bindings with reason-coded diagnostics.
5. Allow ordered pipelines and an explicit outcome-owner selection.
6. Expand macro/internal-call previews before confirmation.
7. Show paused/conflicting capabilities and required Host features.
8. Show incident routes, missing fallbacks, effects, and external actions.
9. Require explicit Recipe/topology/Add-on/overlay confirmation.
10. Save a versioned Recipe and compile an immutable digest-pinned Bundle.

Freedom is constrained by correctness. The builder cannot save or compile a
Profile with an uncovered mandatory outcome, multiple active owners, an
unsupported macro suppression, an incompatible artifact edge, or an
unattested Host feature.

## 9. Profile Recipe v3

`oaw.profile-recipe/v3` replaces the flat responsibility list and one-selector
node model.

Conceptual shape:

```text
ProfileRecipeV3
  schema_version
  taxonomy_version
  recipe_version
  id
  family
  template?
  slots[]
    slot_id
    applicability = mandatory | conditional
    outcome_owner = provider-binding | host-action | none
    pipeline[]
      binding_selector
      stage_span[]
      required_input_artifact
      produced_output_artifact
    gates[]
      authority = oaw-core | host | user
      predicate
      evidence_requirements[]
    transitions[]
  incident_routes[]
    incident_type
    handler
    return_to
    if_unavailable = stop | replan
  overlays[]
    precedence[]
    paused_bindings[]
    selected_alternative
    rationale
  stable_boundaries[]
  environment_requirements[]
```

Required invariants:

- Every mandatory engineering stage has exactly one Provider Binding outcome
  owner. Only taxonomy-declared control slots may use a Host action or an
  evidence-only neutral outcome; a gate can never cover missing engineering
  work.
- `outcome_owner = none` is legal only for a conditional slot that is omitted
  by the compiled Recipe or for a taxonomy-declared evidence-only control
  slot.
- A slot pipeline is ordered; producer output must satisfy consumer input.
- A macro's mandatory internal calls are part of compilation.
- An internal call credited to a slot cannot also be scheduled as a peer.
- An overlay may select a documented alternative or disable an optional call.
  It cannot contradict a mandatory upstream instruction.
- Procedures attach to a stage and do not emit independent mainline
  transitions.
- Incident handlers are reachable only through typed incident routes.
- Neutral gates have no Provider selector, project mutation, or execution
  authority.
- Host actions have declared effects and are dispatched only by the Host; OAW
  Core and Coordinator never execute them.

## 10. Provider Descriptor v4

`oaw.provider-descriptor/v4` binds semantic claims to immutable Distributions
and exact Host surfaces.

Each descriptor records:

- Provider family and Distribution IDs;
- immutable repository revision or content-addressed source;
- complete Binding tree digest;
- Host and surface;
- Binding kind: `skill`, `agent`, `role`, `instruction`, or `tool`;
- exact reference and invocation disposition;
- semantic stage/procedure/incident/assurance responsibilities;
- input/output artifact schemas;
- maximum effects and resources;
- supported outer topologies;
- child, parallel-child, and nested-child requirements;
- macro `stage_span`, internal calls, handoffs, alternatives, and conflicts;
  and
- Capability-to-Binding references.

Content digests use `sha256:<64 lowercase hex>`. A tree digest covers a sorted
canonical list of relative paths, executable modes, sizes, and content hashes.
Symlinks and escaping paths are rejected.

Accepted provenance dispositions:

- `distribution-attested`: a Provider-specific installation root or manifest
  proves the Distribution and the tree digest matches.
- `content-equivalent`: a flattened installation lacks origin metadata but the
  complete Binding tree exactly matches the pinned Distribution member.

Content equivalence proves behavior, not historical origin. Same-name foreign
content does not satisfy either disposition.

Invocation dispositions include `human-explicit`, `model`, `host`, and
`internal`. Profile selection alone does not silently replace Matt's
human-explicit invocation rule. The graph pauses with
`BINDING_EXPLICIT_INVOCATION_REQUIRED` until the Host records the user's exact
invocation for that Binding/run.

## 11. Host Evidence and Actions

Outer topology and internal delegation remain orthogonal:

- `CURRENT`: active session owns the top-level Capability.
- `SUBAGENT`: Host creates a child for the top-level Capability.
- `child-delegation`: current executor can create a child.
- `parallel-child-delegation`: current executor can create the parallel
  children required by Matt review.
- `nested-child-delegation`: a top-level child can create another child.
- `nested-parallel-child-delegation`: a top-level child can create parallel
  review children.

Static config may report `host-configured` or `unknown`; only live Host-native
evidence may report `available`.

Host Manifest v3 and Host Binding Inventory v3 add:

- Distribution/Binding identity and content evidence;
- exact `role` and `instruction` observations;
- live feature observations for child/nested/parallel delegation;
- supported neutral Host actions with input/outcome/effect contracts; and
- evidence digests pinned by the session and Bundle.

Initial neutral Host actions are:

| Action | Effects | Contract |
| --- | --- | --- |
| `workspace.prepare-or-confirm` | read project; optional local workspace/Git mutation after consent | Return path, branch/isolation state, setup command results, and baseline evidence. |
| `verification.execute` | read project; run process | Run the Bundle-declared complete verification set and return command, output digest, exit status, and freshness data. |
| `closeout.execute` | read project; optional local Git; network mutation only after exact user authorization | Reconcile acceptance, present allowed actions, execute the selected action or preserve work, and return terminal evidence. |

Host actions are not fallback Provider skills. They perform control-plane work
that belongs to the Host and user boundary.

## 12. Compilation

Compilation proceeds in this order:

1. Resolve trusted descriptors and immutable Distributions.
2. Match Host-observed Binding identity and complete content digest.
3. Resolve each Recipe pipeline and expand macro/internal calls.
4. Validate slot semantics and exactly one outcome owner.
5. Validate artifact schema compatibility on every ordered edge.
6. Apply selected alternatives and conflict overlays.
7. Validate effects, resources, and user-config limits.
8. Compile each outer topology independently.
9. Validate child/parallel/nested delegation and Host actions.
10. Validate incident routes, neutral gates, and terminal coverage.
11. Require exact user choices where ambiguity remains.
12. Emit the immutable execution graph and Lifecycle Bundle.

No lexical sort, recommendation, Provider brand, or prior Bundle selects a
Binding.

Required diagnostics include:

- `PROVIDER_PROVENANCE_MISMATCH`
- `PROVIDER_BINDING_CONTENT_MISMATCH`
- `PROVIDER_BINDING_UNAVAILABLE`
- `BINDING_KIND_UNSUPPORTED`
- `BINDING_EXPLICIT_INVOCATION_REQUIRED`
- `HOST_FEATURE_UNATTESTED`
- `HOST_ACTION_UNAVAILABLE`
- `PIPELINE_ARTIFACT_INCOMPATIBLE`
- `MACRO_INTERNAL_CONFLICT`
- `OUTCOME_OWNER_MISSING`
- `OUTCOME_OWNER_AMBIGUOUS`
- `INCIDENT_HANDLER_UNAVAILABLE`
- `CAPABILITY_BINDING_AMBIGUOUS`
- `PROFILE_TOPOLOGY_UNAVAILABLE`
- `PROVIDER_PIN_INCOMPATIBLE`
- `BINDING_PREFERENCE_INCOMPATIBLE`
- `BUNDLE_PROVIDER_CONTRACT_UNSUPPORTED`
- `WORKFLOW_STATE_UNSUPPORTED`

## 13. Contract Cutover

The authority-bearing contract set advances together:

| Contract | Target version |
| --- | --- |
| Provider Descriptor | `oaw.provider-descriptor/v4` |
| Profile Recipe | `oaw.profile-recipe/v3` |
| Host Manifest | `oaw.host-manifest/v3` |
| Host Binding Inventory | `oaw.host-binding-inventory/v3` |
| Provider Instance / Resolution / Registry | v4 |
| Execution Graph | `oaw.execution-graph/v4` |
| Lifecycle Bundle | `oaw.lifecycle-bundle/v4` |
| Capability Grant | v3 |
| Dispatch Packet | v2 |
| Coordinator Snapshot/Result/Revision | v2 where they embed changed authority records |

Profile Alias Set v1 remains valid because all four aliases remain and its
meaning does not change. User Config v3 remains readable because deny, pin,
preference, installation, and trusted-recipe references are not the source of
the defect. References that identify a removed v3/v2 authority object fail
explicitly.

Hard-cut rules:

- Provider descriptor v3 is rejected.
- Profile Recipe v2 is rejected.
- Old execution graphs, Bundles, Grants, and Coordinator state cannot dispatch
  under the new binary.
- No dual reader, automatic conversion, silent fallback, or alias remapping is
  provided.
- Old state remains on disk as audit history and is not deleted automatically.

## 14. Bootstrap and Current Workflow

The current Workflow was compiled under the defective catalog. Its
`brainstorming` Binding is real, so this written specification is the last
artifact it may produce. It must not authorize implementation.

After the user approves this written specification:

1. Record the valid design receipt and reach the specification-approved stable
   boundary.
2. Cancel the old Coordinator Workflow without deleting its audit history.
3. Use the user's already selected `SP-FULL / CURRENT / no Add-on` only as an
   explicit, local bootstrap methodology for this contract migration.
4. Invoke the audited Superpowers `writing-plans` skill to create the
   implementation plan.
5. Execute in an isolated workspace with strict TDD, independent review, and
   fresh verification.
6. Do not install Providers, mutate user configuration, push, publish, merge,
   or perform destructive operations.
7. After v4 is locally verified, re-observe the Host through the corrected
   surface and submit a new Coordinator START.
8. The new START must compile a v4 Bundle or fail closed with exact reasons.

This bootstrap is limited to implementing the v4 contract that restores valid
OAW authority. It is not a general compatibility mode.

## 15. Implementation Components

The implementation changes these ownership boundaries:

1. Catalog schemas and Go records for Descriptor v4, Recipe v3, Host v3, and
   the downstream authority records.
2. Distribution-scoped discovery and complete Binding tree integrity.
3. Registry support for multiple verified Binding alternatives.
4. N:M pipeline, macro, overlay, Host-action, and neutral-gate compilation.
5. Four corrected built-in descriptors and Recipes.
6. USER-DEFINED Recipe loading, validation, and Profile Builder projections.
7. Coordinator/admission/dispatch hard rejection of stale authority.
8. Codex Bridge observation of only facts its stable Host APIs can prove.
9. Policy, README, English/Chinese lifecycle and adapter documentation.
10. Generated or parity-checked Profile tables and upstream drift audit.

Implementation must preserve unrelated user changes and untracked artifacts.

## 16. Verification Strategy

### 16.1 Schema and Integrity

- v4/v3 target schemas decode and reject unknown fields.
- Provider descriptor v3 and Recipe v2 fail closed.
- invalid repository revisions, digests, paths, and symlink escapes fail.
- changing a referenced asset changes the complete Binding tree digest.
- same-name/different-content Bindings do not match.
- shared directory ancestry never establishes Provider provenance.

### 16.2 Stage Pipelines and Macros

- `grill-with-docs` credits `grilling + domain-modeling` once.
- Matt `to-spec -> to-tickets` preserves ordered artifacts and separate
  explicit invocations.
- SDD credits workspace, implementation, review, and finish internals once.
- the SP inline alternative requires standalone review and verification.
- the default Hybrid pins Matt TDD, excludes SP TDD and SDD, and retains SP
  standalone review.
- selecting SDD while retaining Matt TDD fails with
  `MACRO_INTERNAL_CONFLICT`.
- incompatible producer/consumer artifacts fail.

### 16.3 Profile Matrix

- all four aliases remain active.
- MATT-FULL never resolves nonexistent Matt skills and requires Host actions.
- MATT-FULL fails without explicit invocation or parallel review evidence.
- SP-FULL fails without the reviewer delegation needed by its selected path.
- SP-FULL top-level SUBAGENT fails without nested delegation.
- ECC Claude Agent names never satisfy Codex Role selectors.
- ECC `e2e-runner` cannot satisfy broad verification.
- ECC `code-reviewer` cannot satisfy closeout.
- current Codex Host v1 reports exact ECC-FULL and Hybrid gaps.

### 16.4 USER-DEFINED

- built-in templates clone without mutating the original.
- ordered multi-Binding pipelines compile when artifacts and effects match.
- missing owner, duplicate owner, unsupported suppression, and unknown Binding
  fail with stable diagnostics.
- untrusted installed skills remain unavailable until a trusted descriptor is
  supplied.
- the confirmed Recipe, topology, Add-ons, overlays, Provider Instances, Host
  evidence, graph, and Bundle are digest-pinned.

### 16.5 Coordinator and Repository Gates

- old state cannot resume or dispatch.
- new START produces only a v4 Bundle.
- evidence drift rejects admission/dispatch.
- `go test ./...` and race-relevant tests pass.
- changed executable logic reaches at least 80 percent coverage.
- policy and bilingual documentation match the catalog matrix.
- fresh final verification is observed before completion.
- no credentials, Provider installation, user-config mutation, push,
  publication, merge, or unrelated file changes occur.

## 17. Acceptance Criteria

The migration is complete only when:

1. the ten canonical slots and typed ownership model are machine validated;
2. Profile Recipes support ordered N:M pipelines and contiguous macro spans;
3. Provider descriptors bind immutable Distribution evidence to exact Host
   surfaces and semantic contracts;
4. Host actions and neutral gates are first-class and cannot masquerade as
   Provider skills;
5. every applicable outcome has exactly one active owner;
6. macro internals are expanded, validated, and never double-dispatched;
7. all four built-in aliases remain active with corrected Recipes;
8. MATT-FULL uses `grill-with-docs`, not a fictional `requirements` skill;
9. MATT-FULL contains no Matt `verification-loop` or completion claim;
10. SP-FULL enforces child/nested-child requirements for its chosen path;
11. ECC distinguishes Skill, Claude Agent, Codex Role, Instruction, and Hook;
12. MATT-SP-HYBRID/default uses Matt TDD and the compatible SP inline path;
13. USER-DEFINED Profiles can combine verified compatible installed Bindings
    per slot and fail closed on every incompatible combination;
14. v3 Provider descriptors, v2 Recipes, and old compiled authority cannot
    execute;
15. current Host re-observation after implementation produces truthful
    eligibility and a new Coordinator START result;
16. tests, coverage, security review, documentation parity, and fresh final
    verification pass; and
17. no unrelated user work or external resource is modified.

## 18. Residual Risks and Explicit Decisions

- Matt's upstream human-typed invocation rule is unresolved by Provider
  metadata. This design chooses the conservative behavior: pause for an exact
  invocation receipt each time.
- Repository presence does not prove a live ECC Codex Binding. Only Host
  inventory evidence can make it eligible.
- Neutral gates judge evidence; they do not run commands. Commands are produced
  only by an attested Provider procedure or Host action.
- Current Codex Bridge v1 cannot prove the delegation and Host-action facts
  required by several target Profiles. The migration must improve observation
  or truthfully leave those Profiles ineligible.
- A user can freely compose only verified compatible Bindings. OAW guarantees
  fail-closed composition, not that every arbitrary installed skill can satisfy
  every slot.
