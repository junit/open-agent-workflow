# ECC-FULL Verification Evidence

## Project Policy installation

The temporary `oaw` executable installed only the project Policy Set and Codex
Activation Router:

```text
oaw: create: project/.oaw/policy/POLICY.md
oaw: create: project/.oaw/policy/adapters/codex-policy.md
oaw: create: project/.oaw/policy/cooperative-protocol.md
oaw: create: project/.oaw/policy/profiles/ECC-FULL.md
oaw: create: project/.oaw/policy/profiles/MATT-FULL.md
oaw: create: project/.oaw/policy/profiles/MATT-SP-HYBRID.md
oaw: create: project/.oaw/policy/profiles/SP-FULL.md
oaw: create: project/AGENTS.md
```

The isolated diagnostic then reported:

```text
version: 0.1.0
scope: project
provider superpowers: missing
provider matt: missing
provider ecc: missing
target codex: detected (user, project)
installed codex: clean
```

Those advisory Provider results did not block ECC-FULL selection or execution.

## Fresh artifact checks after binary removal

`/tmp/oaw-ecc-full.5w98kd/bin/oaw` was removed before these commands ran:

```text
go test ./...
Go test: 9 passed in 2 packages

go vet ./...
exit 0

go run ./cmd/rollout 50 a b hello
a
hello

go run ./cmd/rollout 100 a b hello
a
b
hello

go run ./cmd/rollout 101 a
percentage must be between 0 and 100
exit status 2
```

The isolated state root contained only the project Install State record. The
installer bin directory was empty; there was no `oaw-assurance`, `oaw-bridge`,
workflow runtime state, or post-install OAW executable. The project Git
worktree was clean at `d52e470`.

## Managed identity evidence

```text
POLICY.md fafced6b1bd66291c4bc68acd0aec53150a9d243653198f7d5741b6ad71b4ca0
ECC-FULL.md b40be430fa7b314fedc2469c76a9a93e9e14a03b4f3d4db7c702492b0043fa9c
AGENTS.md 56293f6b9a1e911a260eb9d93100458984f882faa2103557bcbc3f76890cbbd6
```
