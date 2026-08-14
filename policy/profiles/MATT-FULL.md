---
id: MATT-FULL
name: Matt Full
---

# Matt Full

This Profile operates under the [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md).

## Purpose

Use for a Matt-led lifecycle with natural-language tickets and model-native
workspace, verification, and closeout behavior.

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| Problem framing | `grill-with-docs`, including `grilling` and `domain-modeling` |
| Specification | `to-spec` |
| Delivery planning | `to-tickets` |
| Workspace preparation | Host native |
| Implementation and TDD | `implement`, using `tdd` exactly once |
| Review and remediation | `implement`, using `code-review`; findings start a bounded remediation pass and fresh review |
| Fresh verification | Host-native fresh verification |
| Closeout | Host native with user authority |

## Rules

- `grill-with-docs` is the problem-framing entrypoint; do not invent a separate
  requirements owner.
- `implement` coordinates implementation, TDD, and review but does not claim
  workspace preparation, broad verification, or closeout.
- Unexpected functional, hard-bug, or performance failures use
  `diagnosing-bugs`; other unresolved incidents stop for an explicit decision.
- Review findings require remediation, fresh review, and fresh verification.
