# SP-FULL TDD Evidence

## Problem framing and specification

The small delivery was a dependency-free `slugify` CLI for release headings.
The contract is captured in `../spec.md`: Unicode letters and digits are
lowercased and retained, separator runs become one hyphen, edge separators are
removed, and invalid argument counts return usage status `2`.

## RED/GREEN record

1. `go test ./internal/slug` failed with `undefined: Slugify` before the
   transformation implementation existed.
2. The minimal `internal/slug.Slugify` implementation made the five behavior
   cases pass (`6 passed`).
3. `go test ./cmd/slugify` failed with `undefined: run` before the CLI adapter
   existed.
4. The thin argument/output adapter made the full project suite pass (`8
   passed in 2 packages`).

No second implementation or review owner was introduced. The implementation
and TDD owner remained the inline SP-FULL path.
