# ECC-FULL Dogfood Specification

## Capability

A release operator can supply a rollout percentage and stable subject keys to
select a reproducible cohort without a service, network call, or stored state.

## Contract

- `rollout <percentage> <key> [key...]` accepts a decimal percentage from 0
  through 100 and at least one non-empty key.
- Each key is assigned `FNV-1a-32(key) % 100` and is selected when its bucket
  is lower than the percentage.
- Selected keys are printed one per line in input order; repeated keys remain
  repeated.
- Invalid input emits no selected keys and returns usage status `2`.

## Boundaries

The command is dependency-free and has no configuration, persistence, network,
authorization, experiment assignment, or OAW runtime dependency. It does not
make a production rollout decision safe.

## Acceptance Criteria

- AC-001: identical keys always produce the same documented bucket.
- AC-002: zero percent selects no keys and 100 percent selects every key.
- AC-003: intermediate selection preserves input order.
- AC-004: invalid percentages and empty keys produce no partial stdout.
- AC-005: the project remains usable after the installer executable is removed.
