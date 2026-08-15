# Custom Profile TDD Evidence

## RED/GREEN record

1. Default manifest behavior tests ran before the validator existed and failed
   with `undefined: Validate`. RED checkpoint: `5a345c4`.
2. The minimal parser and default `version`/`commit` checks made four tests pass.
   GREEN checkpoint: `d5121a1`.
3. CLI tests ran before the adapter existed and failed with `undefined: run`.
   RED checkpoint: `f83bc88`.
4. The file-reading and output adapter made the first full suite pass (`7
   passed`). GREEN checkpoint: `51beee3`.
5. In the fresh reuse task, `ValidateRequired` tests failed with an undefined
   function and the `--require` CLI test returned usage status `2`. RED
   checkpoint: `0e32361`.
6. The smallest generic required-field API and repeatable flag parser made the
   final suite pass (`10 passed`). GREEN checkpoint: `f98f202`.

The final tests cover valid fields, missing defaults, duplicate fields,
malformed lines, file errors, usage, additional Required Fields, and unchanged
default output. `go test -coverprofile=... ./...` followed by
`go tool cover -func=...` reported 82.6% statement coverage. `go test -race
./...` also passed. No tests were skipped or disabled.
