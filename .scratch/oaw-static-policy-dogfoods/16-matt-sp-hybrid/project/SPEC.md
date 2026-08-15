# Maintenance Window Check Specification

## Problem Statement

Release operators need a quick, deterministic check that a same-day
Maintenance Plan does not schedule two Maintenance Windows at the same time.

## Solution

Provide `windowcheck`, a dependency-free command that validates textual
`HH:MM-HH:MM` windows. It accepts adjacent windows, rejects Overlap, and
reports malformed or backwards windows without writing state.

## User Stories

1. As a release operator, I want to validate a plan from the command line, so
   that I can catch scheduling mistakes before publication.
2. As a release operator, I want Boundary Touch to be accepted, so that
   back-to-back maintenance work is not reported as a conflict.
3. As a release operator, I want Overlap to name both windows, so that I can
   repair the plan without inspecting hidden state.
4. As a release operator, I want malformed clock values rejected, so that a
   typo cannot be treated as a valid maintenance period.
5. As a release operator, I want input order to be irrelevant to conflict
   detection, so that I can validate plans as written by different tools.
6. As a release operator, I want validation to avoid files and network calls,
   so that it remains safe in an isolated project.

## Implementation Decisions

- A Maintenance Window is a same-day half-open interval. Overnight windows and
  time zones are outside this capability.
- Clock text is exactly two digits for hours and minutes, with hours 00-23 and
  minutes 00-59. The end must be after the start.
- The domain evaluator parses and orders windows, then compares adjacent
  ordered windows. The command adapter owns only argument count and output.
- The evaluator reports a stable summary for a valid plan and a named pair for
  the first Overlap. Boundary Touch is valid.
- No persistence, randomness, configuration, network, authorization, or OAW
  runtime is part of the capability.

## Testing Decisions

- The highest useful seam is the public maintenance evaluator: tests observe
  valid summaries and domain errors without mocking internal collaborators.
- Tests cover valid ordering, Boundary Touch, Overlap, malformed clocks,
  backwards windows, empty input, and CLI argument/output behavior.
- Independent literal examples are used instead of recomputing expected values
  through the implementation.

## Out of Scope

Dates, time zones, overnight windows, calendar storage, notifications,
automatic rescheduling, and production deployment decisions.

## Further Notes

Matt owns this domain language and specification. Superpowers may add file-level
execution detail but may not change these terms or blocking edges.
