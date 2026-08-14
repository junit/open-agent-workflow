---
id: SP-FULL
name: Superpowers Full
---

# Superpowers Full

This Profile operates under the [OAW Policy](../POLICY.md) and
[cooperative protocol](../cooperative-protocol.md).

## Purpose

Use for complete feature delivery through the inline Superpowers method.

## Responsibilities

| Responsibility | Skill or action |
| --- | --- |
| Problem framing | `superpowers:brainstorming` |
| Specification | `superpowers:brainstorming` |
| Delivery planning | `superpowers:writing-plans` |
| Workspace preparation | `superpowers:using-git-worktrees` |
| Implementation and TDD | `superpowers:executing-plans` with `superpowers:test-driven-development` |
| Review and remediation | `superpowers:requesting-code-review`, then `superpowers:receiving-code-review` and re-review |
| Fresh verification | `superpowers:verification-before-completion` |
| Closeout | `superpowers:finishing-a-development-branch` with user authority |

## Rules

- Brainstorming owns both aligned problem framing and the approved specification.
- Expected test failures belong to TDD; unexpected technical failures use
  `superpowers:systematic-debugging` before implementation continues.
- Review findings return to the inline implementation owner and require fresh
  review and verification.
- Do not add a second implementation or review owner through an Add-on.
