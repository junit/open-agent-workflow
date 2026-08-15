# MATT-SP-HYBRID TDD Evidence

## Matt seam agreement

The Matt specification selected the public `maintenance.Evaluate` seam for
domain behavior. Tests observe valid summaries and domain errors without
mocking private collaborators. The command adapter has focused output tests;
the compiled command is also exercised by fresh CLI verification.

## RED/GREEN record

1. The Boundary Touch and unordered-plan test was run before `Evaluate` existed
   and failed with `undefined: Evaluate`. RED checkpoint: `502b18e`.
2. The minimal parser and valid summary made the first test pass. GREEN
   checkpoint: `c1222f9`.
3. Overlap, malformed, backwards, and empty-plan tests were added. The suite
   had three passes and one failure because Overlap was not yet detected. RED
   checkpoint: `8eec288`.
4. Ordering conflict detection and strict clock validation made all four domain
   tests pass. GREEN checkpoint: `3e33a8c`.
5. CLI tests were run before the adapter existed and failed with
   `undefined: run`. RED checkpoint: `a0c13bb`.
6. The thin Superpowers execution adapter made the full suite pass. GREEN
   checkpoint: `61de956`.

The final suite contains seven tests across two packages. It covers Boundary
Touch, unordered input, Overlap naming, malformed and backwards windows, empty
plans, valid CLI output, CLI domain errors, and usage handling.

`go test -coverprofile=... ./...` followed by `go tool cover -func=...`
reported 89.6% statement coverage. No tests were skipped or disabled.
