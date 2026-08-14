---
id: ECC-FULL
name: ECC Full
---

# ECC Full

This Profile operates under the [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md).

## Purpose

Use for an ECC-led engineering lifecycle resolved from readable ECC Skill
semantics rather than Provider or installation metadata.

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| Problem framing | `ecc:intent-driven-development` |
| Specification | `ecc:product-capability` |
| Delivery planning | `ecc:blueprint` |
| Workspace preparation | Host native, with `ecc:git-workflow` guidance when useful |
| Implementation and TDD | `ecc:tdd-workflow` |
| Review and remediation | Host-native review; findings return to `ecc:tdd-workflow` |
| Fresh verification | `ecc:verification-loop` |
| Closeout | Host native, with `ecc:git-workflow` guidance and user authority |

## Rules

- Skill availability is determined by readable or invokable semantics, not by
  Provider identity, cache location, revision, or route admission.
- A specialist contract, E2E, security, or review Skill is an Add-on unless
  this Profile assigns it a Responsibility.
- Unexpected build, dependency, or type failures require an exact applicable
  recovery Skill or an explicit stop; do not invent a handler.
- Review findings return to the same implementation and TDD owner and require
  fresh review and verification.
