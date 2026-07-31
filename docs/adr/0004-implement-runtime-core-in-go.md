# ADR 0004: Implement the Runtime Core in Go

## Status

Accepted

## Context

OAW v0.1 is a zero-dependency Bash 3.2 installer. Runtime vNext needs strict
TOML and JSON parsing, versioned schemas, immutable control transitions,
cross-process file locks, atomic state replacement, Host process integration,
concurrency testing, fuzzing, and platform-specific release artifacts.

Continuing to place this responsibility in Shell would make the control plane
difficult to validate and maintain. The Runtime must remain easy to distribute
on macOS, Linux, and WSL without requiring a language runtime on the user's
machine.

## Decision Drivers

- Produce standalone cross-platform binaries.
- Keep process, filesystem, JSON, and concurrency behavior straightforward.
- Support fast unit, race, fuzz, integration, and black-box tests.
- Limit dependency and contributor complexity.
- Migrate from Bash incrementally rather than rewrite installation behavior at once.

## Considered Options

### Continue in Bash

This preserves the current runtime baseline but is poorly suited to strict
structured data, control graphs, revision journals, and cross-process state.

### Go

Go provides standalone binaries, simple cross-compilation, mature filesystem
and process APIs, fast builds, built-in race and fuzz tooling, and a relatively
low contributor barrier.

### Rust

Rust provides the strongest type-level control over immutable state and error
handling, but adds compilation, cross-platform packaging, and contributor cost
that is not yet justified by OAW's size.

### Node.js and TypeScript

TypeScript provides excellent schema and integration ergonomics, but requiring
Node or relying on packaged JavaScript binaries conflicts with OAW's low-runtime
dependency and system-CLI goals.

## Decision

Implement the modular Runtime Core and eventual authoritative `oaw` CLI in Go.
Use a small dependency set for TOML, JSON Schema, and cross-platform file locks.
Do not add a web framework, ORM, dependency-injection framework, scripting
engine, executable plugin runtime, or Runtime v1 daemon.

Keep the existing Bash implementation during migration. `install.sh` becomes a
compatibility wrapper only after the Go management commands pass the existing
black-box behavior suite. Release archives contain a precompiled `oaw` binary;
the wrapper does not download a binary at execution time.

Before parity, Go code may run only as a non-authoritative shadow implementation
for diagnostics, compilation, and conformance fixtures. It does not replace or
change user-facing Bash management behavior.

## Consequences

### Positive

- Users receive a standalone binary on supported platforms.
- The control state machine and persistence code can use typed data and
  deterministic tests.
- Go's race detector, fuzzing, and cross-compilation support the required
  verification strategy.
- Bash and Go can coexist behind black-box parity tests during migration.

### Negative

- The project gains a compiled-language toolchain and release matrix.
- TOML, JSON Schema, and locking require carefully selected dependencies.
- Go does not enforce immutability; domain constructors and copy-on-return
  conventions must provide it.

### Risks

- A premature installer rewrite could regress the existing safety model.
- Go and Bash behavior may drift during the compatibility period.

The migration keeps Bash authoritative until focused and full black-box parity
is demonstrated for each management command.

## Related Decisions

- Implements the Runtime Plane accepted in ADR 0003.
- Preserves the XDG ownership model accepted in ADR 0002.
