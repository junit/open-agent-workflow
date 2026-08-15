# SP-FULL Dogfood Specification

## Deliverable

Create a dependency-free Go command named `slugify` that converts one release
heading into a stable URL slug. It accepts exactly one argument, writes one
slug and a newline to stdout, and exits with usage status `2` for any other
argument count.

## Behavior

- Unicode letters and digits are retained and letters are lowercased.
- Every run of other characters becomes one ASCII hyphen.
- Leading and trailing separators are removed.
- Empty or separator-only input produces an empty slug.

## Design

`internal/slug.Slugify` owns transformation semantics. `cmd/slugify` owns only
argument validation and output. The implementation uses the Go standard
library and carries no configuration, persistence, network, or optional OAW
component.

## Verification

Tests cover mixed separators, edge trimming, Unicode, empty input, and the CLI
contract. A fresh isolated copy receives only a project Policy installation;
the temporary `oaw` executable is removed before the artifact tests and CLI
verification run.
