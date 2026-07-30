# ADR 0001: Keep OAW Provider-Neutral

## Status

Accepted

## Context

OAW coordinates workflow families that are independently licensed, installed,
versioned, and configured. Bundling those providers would make OAW responsible
for upstream distribution, compatibility, and user configuration decisions.

## Decision

OAW owns lifecycle arbitration, adapter entrypoints, provider detection, and
install state. Workflow providers remain external dependencies. OAW may report
missing capabilities and link to provider installation documentation, but it
does not download, vendor, update, or remove providers.

## Consequences

- OAW can evolve without repackaging provider releases.
- Provider licenses and update channels remain independent.
- Some profiles will be unavailable until users install their providers.
- Detection and diagnostics must clearly separate installed, missing, and
  unsupported capabilities.

