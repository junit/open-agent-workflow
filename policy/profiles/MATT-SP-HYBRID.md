---
id: MATT-SP-HYBRID
name: Matt Superpowers Hybrid
---

# Matt Superpowers Hybrid

This Profile operates under the [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md).

## Purpose

Use when Matt owns domain intent, specification, ticket edges, and TDD while
Superpowers supplies executable planning, implementation, review, verification,
and closeout.

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| Problem framing | `grill-with-docs`, including `grilling` and `domain-modeling` |
| Specification | `to-spec` |
| Delivery planning | `to-tickets`, then `superpowers:writing-plans` for executable detail |
| Workspace preparation | `superpowers:using-git-worktrees` |
| Implementation and TDD | Inline `superpowers:executing-plans` implements; Matt `tdd` is the only TDD procedure |
| Review and remediation | `superpowers:requesting-code-review`, then `superpowers:receiving-code-review` and re-review |
| Fresh verification | `superpowers:verification-before-completion` |
| Closeout | `superpowers:finishing-a-development-branch` with user authority |

## Rules

- Matt specifications and ticket dependencies are canonical for domain intent
  and delivery edges. Superpowers planning may add paths, commands, steps, and
  expected results without changing those edges.
- Superpowers TDD and standalone Matt review are not additional owners.
- Expected RED belongs to Matt `tdd`. Unexpected functional, hard-bug, or
  performance failures use `diagnosing-bugs`.
- Build, dependency, or type incidents require an explicitly selected
  specialist Add-on or an explicit stop.
- Review findings return to implementation and require fresh review and
  verification.
