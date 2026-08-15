# Deterministic Rollout Selection

## Capability

A release operator can supply a rollout percentage and stable subject keys to
select a reproducible cohort without a service, network call, or stored state.

## Constraints

- Percentages are decimal integers from 0 through 100 inclusive.
- Each key is assigned `FNV-1a-32(key) % 100`; a key is selected when its
  bucket is lower than the percentage.
- Selection preserves input order and repeated keys.
- Invalid input emits no selected keys and exits with usage status `2`.

## Implementation Contract

- Actor: release operator.
- Surface: `rollout <percentage> <key> [key...]`.
- Input: one percentage and at least one non-empty key.
- Output: selected keys, one per line, in input order.
- State: a key is either selected or not selected for the supplied percentage;
  the command has no persistent lifecycle state.

## Non-Goals

This tool does not assign experiments, store cohorts, manage authorization, or
make a production rollout decision safe.

## Open Questions

None for this bounded delivery.

## Handoff

Ready for direct implementation using the ECC TDD and verification rules.
