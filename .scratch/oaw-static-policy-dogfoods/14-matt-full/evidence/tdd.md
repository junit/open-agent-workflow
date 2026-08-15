# MATT-FULL TDD Evidence

## Domain framing

`grill-with-docs` and `domain-modeling` established three terms: a Checklist
contains Items, an Item is complete only for `[x]` or `[X]`, and surrounding
Markdown is context. `to-spec` captured the external CLI contract in
`../spec.md`; `to-tickets` split parser behavior from the CLI adapter in
`../tickets.md`.

## RED/GREEN record

1. `go test ./internal/checklist` failed with `undefined: Summarize` before the
   Checklist parser existed.
2. The parser implementation made both domain tests pass (`2 passed`).
3. `go test ./cmd/checklist` failed with `undefined: run` before the CLI
   adapter existed.
4. The adapter implementation made the full project suite pass (`4 passed in
   2 packages`).

Matt `tdd` was the only TDD procedure. The implementation remained a single
owner and did not use provider or index metadata.
