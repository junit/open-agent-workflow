# ADR 0001: Use Static Policy As The Product Core

## Status

Accepted

## Date

2026-08-16

## Context

Agent Hosts already provide model execution, Skills, tools, credentials,
sandboxing, approvals, and physical effects. OAW needs to apply installed
engineering methods consistently without replacing those Host capabilities or
requiring a second execution engine.

The product must remain usable when an installer binary, scanner, optional
integration, or machine evidence service is absent after installation. Users
must also be able to compose new methods from their installed Skills without
changing Go code.

## Decision Drivers

- Rules must remain readable by the model and the user.
- Missing machine metadata must not block readable Skills.
- Built-in and user-created methods need one extension contract.
- Host-specific loading details must not enter portable engineering semantics.
- The common path must have no background process or workflow state database.

## Decision

OAW's complete product core is one selected Canonical Policy Set:

```text
POLICY.md
cooperative-protocol.md
profiles/*.md
adapters/<host>-policy.md
```

The Policy defines activation, Responsibilities, defaults, safety, review,
verification, and physical authority boundaries. A selected Markdown Profile
assigns Skills and Host-native actions to Responsibilities. The model reads the
assigned Skill when its Responsibility becomes current and performs the work
with normal Host tools.

A Project Policy Set under `.oaw/policy/` takes precedence over a User Policy
Set. The two sets are never merged. Built-in and Custom Profiles use the same
Markdown contract; omitted Custom Profile Responsibilities use Policy Defaults.

The default `oaw` executable owns only installation management and advisory
Profile inspection. It does not select a Profile, invoke a Skill, classify a
request into machine modes, manage delivery progress, or execute a model.

## Consequences

### Positive

- Installed rules continue working without an OAW process.
- Skill availability is decided from the current Host context, not scanner
  admission.
- Users can add or modify a method with Markdown.
- The Host remains the single physical execution authority.
- The default executable has a narrow, testable responsibility.

### Negative

- Progress discipline depends on model instruction following and user review.
- Skill indexes may be incomplete, so the model sometimes reads Skill files
  directly.
- Host adapters must be maintained for each instruction format.

## Required Invariants

1. Deleting the post-install `oaw` executable cannot invalidate an installed
   Policy workflow.
2. Go code and diagnostics cannot maintain a second Profile definition.
3. Profile selection remains natural-language first.
4. Installation State cannot become delivery progress state.
5. Physical permissions always remain with the Host and user.
