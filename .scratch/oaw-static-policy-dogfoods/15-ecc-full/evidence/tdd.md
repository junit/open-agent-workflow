# ECC-FULL TDD Evidence

## User journey

As a release operator, I want a stable percentage-based cohort from subject
keys so that repeated dry-run decisions select the same subjects without
persisted state.

## RED/GREEN record

1. Core behavior tests were executed before implementation and failed to build
   with `undefined: Bucket` and `undefined: Select`. RED checkpoint:
   `79b6af8`.
2. The minimal FNV-1a bucket and selection implementation made five core tests
   pass. GREEN checkpoint: `66f0b68`.
3. CLI tests were executed before the adapter existed and failed to build with
   `undefined: run`. RED checkpoint: `78f6090`.
4. The minimal argument, error, and output adapter made the full suite pass.
   GREEN checkpoint: `d52e470`.

The final suite contains nine tests across two packages. It covers the stable
hash value, input ordering, duplicate keys, zero and 100 percent boundaries,
invalid percentages, empty keys, CLI usage, parse errors, successful output,
and the no-partial-stdout error guarantee.

`go test -coverprofile=... ./...` followed by `go tool cover -func=...`
reported 96.4% statement coverage. No tests were skipped or disabled.
