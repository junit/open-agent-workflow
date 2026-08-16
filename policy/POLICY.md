# Open Agent Workflow Policy

## Purpose

Open Agent Workflow (OAW) is an opt-in, rule-driven engineering workflow. Its
complete operating path is this Policy, one selected Profile, the independently
installed Skills named by that Profile, and the Agent Host's native abilities.
Installation leaves readable rules in the Host or project, so delivery does not
require an OAW process or optional machine evidence.

This file defines portable semantics. Natural-language operation is defined by
the [cooperative protocol](cooperative-protocol.md). Host-specific discovery
and invocation guidance belongs in a Host Adapter such as the
[Claude Adapter](adapters/claude-policy.md) or
[Codex Adapter](adapters/codex-policy.md). The current Canonical Policy Set
includes adapters for Claude, Codex, Gemini, OpenCode, Cursor, Windsurf,
Cline, Roo, and Copilot.

## Explicit Activation and Non-Interference

Native Host behavior is the default. OAW governs a deliverable only when the
current top-level user request explicitly asks to use OAW or clearly continues
an active OAW deliverable. Installing OAW, discussing it, quoting its rules,
task complexity, automatic discovery or model-led loading of an OAW-named
Skill, and ordinary Skill selection do not activate it.

A Host-native OAW entrypoint documented by the current Host Adapter is an
explicit request only when evidence outside the entrypoint artifact identifies
user selection. That evidence is either the original top-level user input,
observed before Host template expansion, using the native form as a command
rather than quoting or discussing it, or reliable Host metadata distinguishing
a user picker selection from model-led invocation. The entrypoint's name,
description, body, argument hints, and expanded template text never supply this
evidence. Automatic matching, model-led invocation, and physical loading alone
are not proof of user intent. When the source cannot be distinguished, remain
in Native Host behavior. A direct natural-language activation request in the
original top-level user input remains the portable form and does not require a
native entrypoint.

A native entrypoint is a thin dispatcher, not an engineering method. It may
carry an explicitly named Profile and the user's task into the selected Policy
Set. It must not choose a default Profile, duplicate Profile Responsibilities,
impose lifecycle stages or approval gates, or treat automatic model invocation
as user intent. It follows the Activation Router for Policy Set selection and
must not embed a Policy path in a Host-preprocessed command or Skill template.

On activation, load one Canonical Policy Set and select one Profile for the
deliverable. Related follow-ups continue that selection until completion,
cancellation, explicit exit, or a user-requested Profile switch. Unrelated
requests remain Native Host behavior and inherit no OAW selection.

OAW activation must not alter the Host's normal Skill, Agent, tool, permission,
approval, sandbox, or model selection outside the activated deliverable.

## Product Invariants

### Static Policy sufficiency

After installation, the Policy, selected Profile, readable or invokable Skills,
and Host-native abilities are sufficient to complete normal engineering work.
Removing the `oaw` executable, Machine Assurance, or Bridge may remove only the
services those components provide; it must not disable the Policy workflow.

### Monotonic enhancement

Optional components may add convenience, machine evidence, or exact identities.
They may not redefine a Profile, reduce the model's ability to
follow readable Skill rules, or veto a workflow that is valid under this Policy.
A Host security rule may still refuse a physical action; that refusal concerns
execution permission, not Profile existence.

### One semantic source

The selected Markdown Profile defines the engineering method. Go code,
diagnostics, indexes, optional overlays, and Bridge records may inspect or
reference it but must not maintain a parallel workflow definition.

## Physical Authority Boundary

The Agent Host, operating system, repository, and user approvals control every
physical effect. OAW does not create credentials, bypass a sandbox, expand tool
permissions, authorize network mutation, or make destructive actions safe.

Selecting a Profile authorizes its declared Skills as engineering procedures
for this deliverable. It does not authorize effects that would otherwise need
Host or user approval. Following readable Skill instructions must not be
reported as a native Host invocation when no native invocation occurred.

## Canonical Policy Set

One selected Canonical Policy Set consists of:

- this portable Policy;
- the [cooperative protocol](cooperative-protocol.md);
- Built-in Profiles;
- the current Host Adapter.

A Project Policy Set takes precedence over a User Policy Set as a whole. Policy
Set files are never merged. Project and user Custom Profiles may both be
discovered, but their source remains explicit.

The Built-in Profiles are ordinary Markdown Profiles:

- [SP-FULL](profiles/SP-FULL.md)
- [MATT-FULL](profiles/MATT-FULL.md)
- [ECC-FULL](profiles/ECC-FULL.md)
- [MATT-SP-HYBRID](profiles/MATT-SP-HYBRID.md)

## Stable Responsibilities

Responsibilities are reasoning anchors and expected outcomes, not state-machine
nodes. A Skill may cover several Responsibilities, and several Skills may
cooperate within one Responsibility when the Profile makes ownership clear.

1. **Problem framing**: align purpose, constraints, domain terms, decisions, and
   success conditions.
2. **Specification**: produce reviewable behavior, boundaries, and acceptance
   criteria.
3. **Delivery planning**: decompose work into independently verifiable units.
4. **Workspace preparation**: establish a safe workspace and known baseline.
5. **Implementation and TDD**: make bounded changes with an appropriate
   red-green loop for behavior changes.
6. **Review and remediation**: inspect the result against requirements and
   standards, address findings, and review again.
7. **Fresh verification**: observe current, claim-relevant checks after the last
   change.
8. **Closeout**: report the delivered result, evidence, limitations, and any
   user-authorized repository action.

## Profile Contract

A Profile is a Markdown document with YAML frontmatter containing only two
required identity fields:

```yaml
---
id: team-delivery
name: Team Delivery
---
```

Its Markdown body is normative. A `Responsibilities` table maps stable
Responsibilities to Skills or Host-native actions, and a `Rules` section may
state ordering, mandatory Skills, alternatives, incident handling, or stronger
review and verification requirements.

Built-in and Custom Profiles use this same contract. A Custom Profile may name
only the Responsibilities it changes; Policy Defaults cover every omission.
Agent-created Profiles should render the complete table for readability.

Built-in IDs are reserved. Project and user Custom Profiles with the same ID are
both retained and shown with source qualifiers. There is no implicit override,
merge, or inheritance. To customize a Profile, create a new Profile with a new
ID and write its complete intended method.

## Profile Selection and Skill Authorization

An explicitly named Profile starts immediately. When the user does not name
one, the Agent states a reasonable choice and proceeds unless materially
different methods create genuine ambiguity.

Selecting a Profile authorizes every Skill it declares for this deliverable.
Do not request a second per-Skill confirmation solely because a Host marks a
Skill as manual or user-invoked. Ask only when the Host physically requires a
user action or when a proposed substitution changes important method semantics.

Profile selection does not depend on machine provenance, diagnostic output,
Machine Assurance, or Bridge availability. A missing index entry is not proof
that a Skill is absent.

## Policy Defaults

Omitted Responsibilities receive these model-native defaults:

| Responsibility | Default |
| --- | --- |
| Problem framing | Restate the intended outcome and resolve material ambiguity. |
| Specification | Define observable behavior and acceptance boundaries before implementation. |
| Delivery planning | Track the smallest verifiable steps needed for the current scope. |
| Workspace preparation | Inspect the current repository and preserve unrelated work. |
| Implementation and TDD | Use focused test-first changes when behavior is added or corrected. |
| Review and remediation | Review requirements and repository standards; remediate findings and re-review. |
| Fresh verification | Run fresh, relevant checks after the last change before claiming success. |
| Closeout | Summarize changes, evidence, residual risk, and only authorized Git actions. |

Omission never removes safety, review, or fresh verification. A statement that
a Responsibility is not applicable is evaluated against the actual deliverable;
it is not a machine exemption.

## Responsibility and Method Conflicts

The selected Profile owns the method. A task-scoped Add-on may contribute a
specialist result but does not acquire a core Responsibility. When two declared
procedures appear to own the same outcome, interpret explicit Profile rules
first. If ownership remains materially ambiguous, ask the user before changing
method.

An alternative or substitution may proceed with an explanation when it
preserves the same Responsibility owner and method. A change to TDD, review,
verification, security, or Responsibility ownership requires user confirmation.
An exact Skill marked mandatory by the Profile is never silently replaced.

## Complexity and Risk

Complexity and Risk are qualitative model judgments. They do not activate OAW,
select a Profile, create CLI state, or grant permissions.

- **Complexity** increases decomposition, written planning, and continuity
  detail as interactions and uncertainty grow.
- **Risk** increases approvals, negative testing, security attention, independent
  review, rollback preparation, and verification strength as consequences grow.

A small but high-risk action may need stronger safeguards without becoming a
large workflow. A complex but low-risk refactor may need detailed planning
without additional authority.

## Safety and Quality Rules

- Preserve unrelated user changes and inspect the worktree before editing.
- Diagnose root causes before fixing unexpected failures.
- Validate inputs at trust boundaries and never place credentials in Policy,
  Profiles, progress notes, logs, or evidence.
- Treat external mutation, deployment, publication, data changes, destructive
  filesystem operations, and permission expansion as requiring the same Host
  and user authority they require without OAW.
- Use tests that observe public behavior rather than implementation details.
- Review after implementation; findings return to implementation and require
  fresh review and fresh verification.
- Observe current verification output before claiming completion.
- Report actual limitations and uncertainty instead of inventing Skill use,
  evidence, or machine assurance.

The [cooperative protocol](cooperative-protocol.md) defines how these rules are
applied through natural-language interaction.
