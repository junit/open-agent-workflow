# Model-Led Lifecycle

The lifecycle is a conversation contract described by the Policy and selected
Profile. It is not a CLI state machine and it does not require an OAW process.

## Activation

OAW is inactive until the current top-level request explicitly asks for it.
After activation, load one Policy Set and select one Profile for the
deliverable. Related follow-ups keep the selection; an unrelated deliverable
is native Host work. A user may explicitly switch Profiles when the method
must change.

## Responsibilities

Profiles map stable Responsibilities such as framing, planning,
implementation, review, verification, and closeout to Skills or Host-native
actions. The model owns the order and can revisit a Responsibility when new
evidence changes the plan. A Profile may omit Responsibilities and use the
Policy default.

Complexity and Risk are model judgments. Complexity changes decomposition and
planning depth. Risk changes review, approval, negative testing, and
verification strength. Neither value selects a Profile, activates OAW, grants
permissions, or creates a machine record.

## Skill Resolution

For each declared Skill, use the current Host's native invocation or a
readable Skill document. The Host index is advisory and may be incomplete.
When a Skill is unavailable, use an alternative explicitly named by the
Profile or the Policy default, or state the limitation and ask the user.
Do not invent a provider, route, result, or recovery owner.

## Natural-Language Examples

Profile work stays in the conversation rather than moving into a command or a
machine gate:

```text
Use OAW to create a project Profile named release-readiness from the Skills I
already have, then use it for this release checklist.

Use OAW with project:release-readiness to deliver this change.

Switch this deliverable to SP-FULL at the next stable boundary and preserve
the completed specification and tests.

The `ecc:delivery-gate` Skill is unavailable. Use the Policy verification
default, state that fallback, and continue only with the same Responsibility.
```

When a project and user Profile share an ID, select the source explicitly with
`project:<id>` or `user:<id>`. A switch changes the method for the remaining
work; it does not rewrite completed ownership or silently merge the two
sources. A fallback preserves the Responsibility and names the replacement
method before it is used.

## Progress and Completion

Conversation is the primary progress record. A project may keep an optional
Markdown Progress Note containing the selected Profile, completed
Responsibilities, evidence, and next step. It is continuity aid, never
authoritative control state and never a gate.

Completion means the named deliverable is implemented, reviewed at the
appropriate strength, freshly verified, and reported with remaining risks.
Git commits, releases, and deployment are user-authorized actions, not hidden
lifecycle stages.

## Optional Machine Path

An optional Machine Assurance component may attest Profile content or exact
Skill identity. An optional coordinator may record cooperating-client
coordination. These components add evidence only; their absence, failure, or
unavailability never blocks the normal Policy lifecycle.
