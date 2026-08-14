# ADR 0011: Make Static Policy Profiles the Product Core

## Status

Accepted

## Date

2026-08-14

## Context

ADR 0010 made Policy the default path and prevented machine evidence from
vetoing a cooperative workflow. Its required base still included a fixed Go
Profile catalog, Host route inspection, a lifecycle reducer, project-scoped
Engagement state, and workflow commands in the default `oaw` binary. That
design preserved no-Bridge operation, but it made the rule-driven path depend
on machine-shaped metadata and fixed the Policy product to four built-in
Profiles. The resulting scanner false negatives reproduced the original Matt
and ECC availability problem, while custom Profile composition remained
available only through the more complex machine Recipe system.

OAW has not been released, so no compatibility obligation justifies retaining
these pre-release abstractions.

## Decision

OAW's complete product core is a static, model-readable Canonical Policy Set:

```text
POLICY.md
cooperative-protocol.md
profiles/*.md
adapters/<host>-policy.md
```

After installation, the Policy, selected Profile, independently installed
Skills, and Host-native abilities are sufficient for normal engineering work.
No OAW runtime process, route scanner, reducer, state database, machine
Provider evidence, Assurance component, or Bridge is required.

Built-in and user-created Profiles use the same Markdown format. A Profile has
minimal identity metadata and maps a stable set of engineering
Responsibilities to freely composed Skills or Host-native actions. Custom
Profiles may define only the Responsibilities they need to customize; Policy
Defaults cover the rest. Users create, select, modify, and switch Profiles
through natural language. Selecting a Profile authorizes its declared Skills
for the deliverable.

The default Policy product has no formal `DIRECT`, `BOUNDED`, or `WORKFLOW`
Request Mode, no startup form, and no CLI-driven lifecycle state machine.
Complexity and Risk are model judgments that scale planning, review, approval,
and verification. Add-ons are task-scoped specialist Skills. Progress is
model-owned, with an optional Markdown Progress Note for continuity.

Host Adapter inspection is advisory. An Observed Route may help the model find
a Skill, but it cannot decide Profile availability. The model checks Host
listings, readable Skill instructions, user intent, and available native
abilities. Missing machine metadata never blocks Policy execution.

Project installation is self-contained under `.oaw/policy/`; a Project Policy
Set takes precedence over a User Policy Set without merging. Project Custom
Profiles live under `.oaw/profiles/`. Built-in Profile IDs are reserved, and
same-ID custom Profiles from different scopes require explicit source
selection rather than implicit override.

The default `oaw` CLI is limited to installation management and optional
Profile discovery or checking. Machine Assurance and Bridge functionality move
to physically separate optional components. Machine Assurance may attach an
Assurance Overlay to a Policy Profile, but it may not maintain a second
workflow definition or veto the Policy Profile. Bridge depends on Machine
Assurance; the Policy product depends on neither.

The old `USER-DEFINED` Recipe configuration, Profile Recipe builder, Policy
route gate, reducer, Engagement database, formal Request Mode classifier, and
their compatibility surfaces are removed without migration or dual readers.

## Consequences

- The model-readable rules are the product authority instead of a Go workflow
  projection.
- OAW remains fully usable when every optional executable component is absent.
- Built-in and custom engineering methods share one extension mechanism.
- User interaction becomes natural-language first and no longer requires
  topology, add-on sentinel, risk, or limitation confirmations.
- The default binary and vocabulary become substantially smaller.
- Machine Assurance can remain strict because failure to issue a machine claim
  does not affect Policy operation.
- The pre-release machine Recipe and cooperative reducer implementations must
  be deleted or replaced rather than wrapped in compatibility layers.
- Five fresh dogfoods are required: the four built-in Profiles and one
  user-created Custom Profile, all in isolated projects without Runtime,
  Assurance, or Bridge.

## Related Decisions

- Supersedes ADR 0010's required Policy catalog, route inspection, reducer,
  Engagement state, and single-binary extension shape while retaining its
  Policy-first and machine non-veto principles.
- Refines ADR 0002 by replacing one XDG-only policy artifact with one selected
  Canonical Policy Set that may be project-contained or user-scoped.
- Refines ADR 0009 by making Core and Coordinator optional machine components
  rather than required Policy dependencies while retaining Host-owned physical
  execution.
- The complete design is
  [OAW Static Policy Profile Architecture Refactor](../superpowers/specs/2026-08-14-oaw-static-policy-profile-architecture-refactor-design.md).
