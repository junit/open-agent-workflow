# ADR 0002: Use an OAW-Owned XDG Rule Source

## Status

Accepted

## Context

The initial private setup stored its shared engineering policy in a generic
agent-skills namespace. An open-source installer should not assume ownership of
a provider or ecosystem directory, and duplicated target policies would drift.

## Decision

OAW stores its canonical policy in its own XDG configuration namespace and its
mutable state and backups in its own XDG state namespace. Target adapters
install thin entrypoints that import or direct the agent to that canonical
policy using documented target behavior.

## Consequences

- OAW has a clear ownership boundary and can uninstall cleanly.
- All targets derive from one policy artifact.
- Adapter entrypoints may contain an install-time absolute path where the
  target requires one.
- Existing private installations using another canonical path require an
  explicit migration rather than silent adoption.

