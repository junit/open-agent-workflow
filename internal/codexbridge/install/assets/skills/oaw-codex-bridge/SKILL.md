---
name: oaw-codex-bridge
description: Coordinate Open Agent Workflow through the current Codex Host session.
---

# OAW Codex Host Bridge

Use `observe_current` before relying on current-session Host evidence. Treat its
returned handle and inventory digest as the authority for subsequent Bridge
calls in the same session and working directory.

Use `core_inspect` and `core_compile` for read-only Core projections. Use
`workflow_exchange` only for an OAW workflow already selected by the user.

The presence of this Skill or Plugin is not evidence that the current session
loaded the Bridge. Only a successful `observe_current` response establishes
that fact.
