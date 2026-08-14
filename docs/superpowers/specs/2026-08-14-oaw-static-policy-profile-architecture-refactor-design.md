# OAW Static Policy Profile Architecture Refactor

**Status:** Approved design, pending implementation plan

**Date:** 2026-08-14

**Decision:** ADR 0011

**Scope:** Policy product, Profile extension model, installation, default CLI,
optional machine components, legacy removal, and dogfood acceptance

## Summary

OAW is a rule-driven product. Its complete normal operating path is the static
Policy that an Agent Host reads, the selected model-readable Profile, the
independently installed Skills referenced by that Profile, and the Host's native
abilities. No OAW runtime process is required after installation.

The current repository does not fully satisfy that product model. The
policy-cooperative path still uses a fixed Go Profile catalog, exact Host route
inspection, a workflow reducer, project Engagement state, and mandatory CLI
transitions. User-defined composition is available only through the more
complex machine Profile Recipe path. This makes the primary Policy path less
extensible than the optional machine path and lets incomplete scanner evidence
block otherwise usable Skills.

This refactor makes static Markdown Profiles the only workflow semantic source.
The four built-in Profiles and user-created Profiles use the same format. The
default `oaw` binary becomes an installer and optional Profile inspection tool.
Machine Assurance and Bridge functionality move to physically separate optional
components that may add evidence but cannot redefine or veto Policy behavior.

There is no compatibility or migration requirement because OAW has not been
released.

## Problems to Correct

### Authority inversion

The current Policy CLI accepts a Profile only when a fixed scanner reports every
required route. That gives Adapter observations more authority than the model
that can inspect the actual Host context and Skill instructions.

### Fixed Policy extension surface

`internal/policycatalog` defines exactly four Profiles in Go and validates that
the catalog contains exactly those four. A user cannot add a new Policy Profile
without changing and rebuilding OAW.

### Duplicate workflow semantics

Built-in workflow meaning currently exists in Policy prose, the Go Policy
catalog, machine Profile Recipe assets, compiler code, and tests that compare
the projections. The comparison tests reduce drift but do not remove the
duplicate authorities.

### Machine-shaped normal interaction

The default path asks users to confirm Profile, topology, add-on sentinel,
complexity, risk, and Policy limitations. It then requires CLI transition
commands after ordinary work, review, gates, incidents, switches, and closeout.
That interaction is unnecessary for a model-driven rules product.

### Host implementation inside portable Policy

The current canonical Policy includes Codex CLI commands, plugin cache paths,
`.agents/skills` discovery, `/plan`, and `review-pr` behavior. This prevents the
portable semantics from remaining a clear Host-independent contract.

## Goals

- Make a static Canonical Policy Set sufficient for complete normal operation.
- Preserve OAW's explicit activation and native Host behavior when inactive.
- Use one Markdown Profile mechanism for built-in and custom engineering flows.
- Let users create, modify, select, and switch Profiles through natural language.
- Keep the user-facing model limited to Policy, Profile, Responsibility, Skill,
  and Add-on.
- Keep Skill discovery model-led and Adapter diagnostics non-authoritative.
- Remove formal Request Modes and CLI lifecycle state from the Policy product.
- Make project installation self-contained and portable across machines.
- Isolate optional Machine Assurance and Bridge code physically from the core
  CLI.
- Delete pre-release legacy abstractions instead of preserving compatibility.
- Re-prove all four built-in Profiles and one Custom Profile through real
  no-Runtime, no-Bridge dogfood.

## Non-Goals

- Defining the final wire protocol or schema for the optional Assurance
  component.
- Making OAW a physical sandbox or operating-system enforcement layer.
- Vendoring, downloading, updating, or removing third-party Skills.
- Building an interactive Profile editor outside the Agent conversation.
- Guaranteeing that a Host lists every readable Skill in its native Skill index.
- Providing global mutual exclusion for multiple Agents editing one repository.
- Preserving old Recipe, reducer, state, command, or schema compatibility.

## Architectural Invariants

### Static Policy sufficiency

After installation, removing the `oaw` binary from `PATH` must not stop an Agent
from activating OAW, loading a Profile, using Skills, and completing a
deliverable.

### Monotonic enhancement

Adding an optional component may add convenience, evidence, or coordination.
Removing that component may remove only those extra claims; it must not remove a
workflow that the Policy and Host can perform.

### One semantic source

The selected Markdown Profile defines the engineering method. Go code, Adapter
diagnostics, Assurance Overlays, and Bridge records may project or inspect that
Profile but may not maintain an independent workflow definition.

### Host-owned physical authority

The Host, operating system, repository, and user approvals remain authoritative
for physical effects. Policy selection and machine evidence do not expand those
permissions.

### No machine veto of Policy

Machine Assurance may refuse to issue a machine claim. A Host security policy
may refuse a physical invocation. Neither result changes whether the Policy
Profile exists; the Agent attempts a valid alternative or explains the actual
Host limitation.

## Target Architecture

```text
Agent Host
  |
  v
Activation Managed Block
  |
  v
Selected Canonical Policy Set
  |- POLICY.md
  |- cooperative-protocol.md
  |- profiles/*.md
  `- adapters/<host>-policy.md
          |
          v
  Installed Skills + Host-native abilities

Optional, separately installed:

oaw-bridge -> oaw-assurance -> selected Policy Profile
```

The dependency direction is one-way. The Policy product has no import,
installation, process, state, or availability dependency on either optional
component.

## Canonical Policy Set

### Project installation

```text
<project>/
  AGENTS.md
  .oaw/
    policy/
      POLICY.md
      cooperative-protocol.md
      profiles/
        SP-FULL.md
        MATT-FULL.md
        ECC-FULL.md
        MATT-SP-HYBRID.md
      adapters/
        codex-policy.md
    profiles/
      <custom-profile>.md
```

`oaw install --scope project` installs the complete managed Policy Set beneath
`.oaw/policy/` and writes a small Managed Block to the Host instruction surface.
The Managed Block points only to project-relative files. A project may commit
the managed Policy Set and Custom Profiles so every collaborator uses the same
rules without a global OAW installation.

`oaw update` may replace only OAW-managed content under `.oaw/policy/`. It must
never modify `.oaw/profiles/`.

### User installation

The user-scoped equivalent lives beneath:

```text
${XDG_CONFIG_HOME}/open-agent-workflow/
  POLICY.md
  cooperative-protocol.md
  profiles/
    builtin/
    <custom-profile>.md
  adapters/
```

If a project contains `.oaw/policy/POLICY.md`, the Host loads only the Project
Policy Set. Otherwise it loads the User Policy Set. Core Policy Sets are never
merged. Custom Profiles from project and user scopes may both be discovered.

### Policy file responsibilities

`POLICY.md` owns:

- explicit activation and continuation rules;
- static Policy sufficiency and monotonic enhancement;
- the physical authority boundary;
- the stable Responsibility vocabulary;
- Profile selection, Skill authorization, fallback, and conflict principles;
- Complexity and Risk guidance;
- safety, review, verification, and closeout defaults.

`cooperative-protocol.md` owns:

- natural-language Profile recommendation and selection;
- lazy Profile and Skill loading;
- Add-ons;
- model-driven progress and optional Progress Notes;
- Profile switching by completed-Responsibility difference;
- handling of uncertainty and actual Host refusal.

`adapters/codex-policy.md` owns:

- Codex instruction and Skill surfaces;
- current Codex Skill index behavior;
- readable Codex plugin and `.agents/skills` locations;
- Codex-native planning, review, and tool conventions;
- optional Observed Route diagnostics.

Codex paths, plugin names, slash commands, and cache layouts must not appear in
`POLICY.md` or built-in Profile semantics.

## Profile Model

### Stable Responsibilities

The Policy defines these stable engineering outcomes:

1. problem framing;
2. specification;
3. delivery planning;
4. workspace preparation;
5. implementation and TDD;
6. review and remediation;
7. fresh verification; and
8. closeout.

They are model reasoning anchors, not state-machine nodes. A Skill may cover
multiple Responsibilities, and one Responsibility may use several Skills.
Profiles may add method-specific steps around them.

### Minimal document contract

```markdown
---
id: team-delivery
name: Team Delivery
---

# Team Delivery

## Purpose

Use for normal product feature delivery.

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| Problem framing | `grill-with-docs` |
| Specification | `to-spec` |
| Delivery planning | `superpowers:writing-plans` |
| Workspace preparation | Host native |
| Implementation and TDD | `ecc:tdd-workflow` |
| Review and remediation | `code-review` |
| Fresh verification | `verification-loop` |
| Closeout | Host native |

## Rules

Review findings return to implementation and require fresh review and
verification.
```

Only `id` and `name` are required metadata. Scope comes from the file location.
The Markdown body is the normative rule content. Policy Profiles do not require
a schema version, content digest, Provider, Binding, topology, Risk, or route
contract.

### Partial Profiles and defaults

A Custom Profile may name only the Responsibilities the user wants to customize.
The Policy supplies model-native defaults for all omitted Responsibilities.
Review, verification, and safety behavior do not disappear through omission.
An explicit `not applicable` statement is evaluated against the current
deliverable rather than accepted as a machine exemption.

Agent-created Profiles should render the complete table for readability, even
though hand-written partial Profiles remain valid.

### Built-in and custom Profiles

The four built-in Profiles are ordinary managed Markdown files. They do not
have a parallel Go representation.

Custom Profile locations are:

- project: `.oaw/profiles/<id>.md`;
- user: `${XDG_CONFIG_HOME}/open-agent-workflow/profiles/<id>.md`.

Project scope is the default when the user creates a Profile inside a project.
User scope requires an explicit request for a global or reusable Profile.

Built-in IDs are reserved. User and project Custom Profiles with the same ID
are both retained and displayed with a source qualifier. There is no implicit
override, merge, or Profile inheritance. To customize an existing Profile, the
Agent creates a new complete Profile with a new ID.

### Lazy loading

OAW activation initially reads only the selected Canonical Policy Set and the
minimal identity and Purpose metadata needed to identify candidate Profiles.
The complete Profile is loaded only after selection. Referenced Skill rules are
loaded when the current Responsibility needs them.

An optional CLI index may accelerate discovery but is never authoritative.

## Selection and Skill Use

### Natural-language operation

Examples:

```text
Use MATT-FULL for this deliverable.
Use team-delivery and add e2e-testing.
Create a project Profile using grill-with-docs, writing-plans, tdd-workflow,
code-review, and verification-loop.
```

An explicitly named Profile starts immediately. The Agent does not repeat
Profile, topology, add-on sentinel, Complexity, Risk, or Policy limitation
confirmation fields.

When no Profile is named, the Agent makes and states a reasonable choice. It
asks the user only when materially different Profile semantics create genuine
ambiguity.

### Profile authorization

Selecting a Profile is an explicit request to use every Skill declared by that
Profile for the current deliverable. The Agent does not pause for a second
per-Skill confirmation merely because a Skill is marked user-explicit or not
automatically invoked by the Host.

If the Host physically requires a user action, the Agent must still request that
action. A readable Skill followed as rules must not be reported as a native Host
invocation when no such invocation occurred.

### Model-led Skill resolution

For each declared Skill, the Agent evaluates, in order:

1. the current Host-provided Skill index;
2. readable Skill instructions at Adapter-documented locations;
3. the Profile's declared alternatives;
4. semantically equivalent installed Skills;
5. the Policy Default or Host-native behavior.

Absence from a Host index or Adapter observation is not proof of absence. A
Skill is actually unavailable only when the Agent cannot access its rules and
the Host cannot invoke it.

A declared alternative may be used directly and reported. An undeclared
substitution that preserves method and Responsibility ownership may proceed
with an explanation. A substitution that changes TDD, review, verification,
security, or Responsibility ownership requires user confirmation. A Profile
may state that one exact Skill is mandatory, in which case it is not silently
replaced.

Required route contracts, Provider revisions, lockfiles, digests, and cache
paths are forbidden as Policy eligibility conditions. An Adapter may expose an
optional semantic hint, but it remains advisory.

### Add-ons

An Add-on is a task-scoped specialist Skill named in natural language or
recommended by the Agent. There is no `NONE` sentinel and no startup Add-on
form. An Add-on does not acquire a core Responsibility unless the user changes
the Profile. Persistent method changes belong in a new or edited Profile.

### Profile switching

The user may switch Profile at any time. The Agent summarizes completed
Responsibilities and artifacts, maps them to the new Profile, reuses compatible
outcomes, and performs only the missing or strengthened work. A switch that
would reduce an important TDD, review, verification, or safety guarantee
requires explicit confirmation.

No stable-boundary state machine or `oaw switch` command decides whether the
change is permitted.

## Complexity, Risk, and Progress

The Policy product has no formal `DIRECT`, `BOUNDED`, or `WORKFLOW` Request
Mode. The Agent handles a normal OAW task directly, uses an explicitly requested
Skill, or follows a selected Profile.

Complexity adjusts decomposition, planning detail, and coordination. Risk
adjusts approval, negative testing, review, security attention, and fresh
verification. Neither is a CLI field, Profile selector, or execution gate.

The Agent and Host-native planning tools own live progress. For long or
cross-session work the Agent may write a Markdown Progress Note containing:

- deliverable;
- selected Profile and source;
- task-scoped Add-ons;
- completed Responsibilities and artifacts;
- current Responsibility;
- review or verification evidence;
- next step and unresolved decisions.

A missing, stale, or damaged Progress Note cannot block work. The Agent
reconstructs only factual progress from current artifacts and asks when a
material decision is uncertain.

## Default CLI

The default binary exposes only installation management and optional Profile
inspection:

```text
oaw install
oaw update
oaw check
oaw uninstall

oaw profile list
oaw profile show <id>
oaw profile check <id-or-path>
```

Natural language is the primary create, edit, select, use, and switch interface.
The CLI may report malformed frontmatter, duplicate IDs in one scope, missing
files, or advisory Responsibility and Skill warnings. Those diagnostics do not
decide whether the model can understand and use a Profile.

The following Policy commands are removed:

```text
oaw profiles
oaw use
oaw status
oaw complete
oaw review
oaw approve
oaw satisfy
oaw switch
oaw incident
oaw uncertain
oaw stop
```

Their reducer, Engagement store, project lock, typed transitions, opaque work
references, and route-drift gate are removed with them.

## Optional Components

### `oaw-assurance`

Machine Assurance is a separately built and installed component. It may create
an Assurance Overlay containing:

- Profile ID and content digest;
- exact Provider and Binding identities;
- Host invocation evidence;
- effects, resources, and approval requirements;
- verification evidence and Receipts.

An Overlay references one Policy Profile and may not change its
Responsibilities, Skill composition, order, or rules. If a free-form Custom
Profile cannot be projected into a machine-checkable Overlay, the Profile still
works normally; only the machine claim is unavailable.

### `oaw-bridge`

Bridge is a separately installed Host integration for machine evidence or
special coordination. It depends on `oaw-assurance`. It is not installed by the
default `oaw` installer and is not a sandbox, process supervisor, or Policy
dependency.

### Coordinator

Strict transition state, idempotency, evidence journals, or cooperating-client
serialization belong only in the optional machine component. No machine lock or
Lease is projected into Policy semantics.

## Current-to-Target Code Map

| Current area | Target action |
| --- | --- |
| `policy/ENGINEERING.md` | Replace with the Canonical Policy Set and rename the entry to `POLICY.md`. |
| `internal/policycatalog` | Remove the fixed four-Profile authority; replace only the optional `profile list/show/check` reading needs. |
| `internal/policyroute` | Remove from Profile admission; retain no more than non-authoritative Adapter diagnostics if they provide user value. |
| `internal/policyflow` | Delete from the Policy product. |
| `internal/policyengagement` and `internal/policyrun` | Delete project Engagement state, locks, and reducer persistence. |
| `internal/cli/policy.go` | Replace workflow transition commands with the small `profile` command group. |
| `internal/classification` and Policy classification flags | Remove from the Policy product; optional machine classification must live behind `oaw-assurance` if retained. |
| `internal/assets/recipes`, Profile Recipe schemas, `internal/profile` Recipe builder, and `USER-DEFINED` config | Delete rather than migrate; design any future Overlay from the new Profile source. |
| `internal/config`, `internal/core`, `internal/discovery`, `internal/registry`, and `internal/integrity` | Split out the portions still justified by Machine Assurance; no default CLI dependency may remain. |
| `internal/coordinator` | Move behind the optional machine component or remove unneeded behavior after usage review. |
| `internal/codexbridge` | Move to the separately built `oaw-bridge` component. |
| `internal/management` | Preserve safe installation ownership, but install a complete project or user Policy Set instead of one global policy artifact. |
| cross-projection tests | Replace with tests that built-in Markdown Profiles are valid and optional Overlays reference, rather than redefine, them. |

The implementation plan must confirm imports before deletion, but it must not
retain a package merely to provide compatibility with the pre-release design.

## Implementation Sequence

### Phase 1: Canonical artifacts

- Add `POLICY.md`, `cooperative-protocol.md`, the four built-in Profile files,
  and `adapters/codex-policy.md`.
- Move portable rules, cooperative behavior, Profile semantics, and Codex
  details into their respective owners.
- Add static Profile format and cross-file reference checks.

### Phase 2: Self-contained installation

- Teach project installation to copy the complete set into `.oaw/policy/`.
- Teach user installation to own the complete XDG set.
- Implement project-over-user Policy selection without merging.
- Preserve all Custom Profile files during update and uninstall.
- Update Managed Blocks to load the selected set lazily.

### Phase 3: Minimal Profile CLI

- Implement `profile list`, `show`, and advisory `check` over built-in, user,
  and project Profile locations.
- Detect same-scope duplicate IDs and display cross-scope conflicts explicitly.
- Keep CLI output out of the Agent's authority path.

### Phase 4: Policy hard cut

- Remove formal Request Mode handling from Policy and default CLI.
- Remove route admission, reducer, Engagement state, locks, and transition
  commands.
- Remove fixed Go Profile semantics and old Policy CLI tests.
- Add dependency guards proving the default binary has no Assurance or Bridge
  imports.

### Phase 5: Optional component extraction

- Create separate command and package boundaries for `oaw-assurance` and
  `oaw-bridge`.
- Remove old Recipe-based custom configuration and define only the minimum
  Overlay interface needed by retained machine use cases.
- Delete optional machine functionality that has no named use case after the
  split.

### Phase 6: Documentation and dogfood

- Rewrite active English and Chinese product documentation for the new user
  model.
- Remove active references to `ENGINEERING.md`, Policy Offer, Host-routable
  candidates, Engagement state, formal Request Modes, and Policy lifecycle CLI
  transitions.
- Execute the five isolated dogfoods and record evidence.
- Fix the version only after all acceptance gates pass.

No intermediate phase is a releasable compatibility state. The repository may
temporarily contain old and new code during implementation, but active Policy
documentation and default commands cut over together.

## Verification Strategy

### Static and unit checks

- Profile frontmatter and identity validation;
- built-in Profile uniqueness and Responsibility coverage;
- partial Custom Profile defaulting;
- user/project discovery and conflict reporting;
- project-over-user Policy Set selection;
- managed installation ownership and Custom Profile preservation;
- no import path from the default CLI into Assurance, Coordinator, or Bridge;
- forbidden active references to old Policy commands and terminology.

### Installation black boxes

Each fixture uses isolated `HOME`, `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, and
project directories. Cover project and user install, update, check, uninstall,
surrounding Host instruction content, managed drift, hostile paths, symlink
replacement, and Custom Profile preservation.

Project fixtures must prove that the installed project continues to operate
after the `oaw` executable is unavailable.

### Required dogfoods

| Dogfood | Required proof |
| --- | --- |
| `SP-FULL` | Natural-language selection and a real small delivery using the managed Profile and Superpowers rules. |
| `MATT-FULL` | Matt Skills work when their files are readable even if the Host Skill index omits one. |
| `ECC-FULL` | ECC Skills are selected by model semantics without Provider or cache-path admission. |
| `MATT-SP-HYBRID` | The mixed ownership described by the Profile is followed without Bridge or route contracts. |
| Custom Profile | Natural-language creation, project discovery, execution, one Skill fallback decision, and reuse in a fresh task. |

Every dogfood uses a separate project, project-only installation, no Assurance,
no Bridge, no OAW workflow state, and no runtime dependency after installation.
Evidence must include the delivered artifact, relevant tests, review result,
fresh verification, and a check that the old startup confirmation form did not
appear.

## Security Boundaries

- Project Profiles have the same trust boundary as project instructions and
  source files; OAW adds no digest trust gate to the Policy path.
- OAW activation and Profile selection do not bypass Host sandboxing or user
  approvals.
- A Profile cannot override the core Policy's safety and authority rules.
- A Skill controls its procedure only within its assigned Responsibility.
- External sending, credentials, destructive actions, or permission expansion
  still require the Host and user authority that would be required without OAW.
- Optional machine evidence may strengthen a claim but cannot compel physical
  execution.

## Definition of Done

The refactor is complete only when:

1. `POLICY.md` and the split Canonical Policy Set are the only active Policy
   authority.
2. All four built-in Profiles are Markdown files using the Custom Profile
   contract.
3. A user can create and use a project Custom Profile through natural language
   without editing machine configuration.
4. The default CLI contains no workflow reducer or machine-assurance dependency.
5. Policy operation succeeds with the `oaw` executable absent after install.
6. Old Request Modes, `USER-DEFINED` Recipes, Policy route gates, Engagement
   state, and lifecycle transition commands are absent from active code and
   documentation.
7. `oaw-assurance` and `oaw-bridge` are physically optional and one-way
   dependent on Policy Profile semantics.
8. All five real dogfoods pass in isolated no-Bridge projects.
9. English and Chinese active documentation describe the same product.

## Deferred Optional-Component Detail

The exact Assurance Overlay schema, transport, and retained Coordinator feature
set require a separate design inside the optional component. That decision may
reduce or remove machine functionality, but it cannot delay or change the
static Policy product defined here.
