# MATT-SP-HYBRID Verification Evidence

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

The isolated diagnostic reported all three Providers missing but the project
Codex installation clean. That advisory result did not block Hybrid selection.

## Fresh artifact checks after binary removal

`/tmp/oaw-matt-sp-hybrid.VIPAA4/bin/oaw` was removed before these commands:

```text
go test ./...
Go test: 7 passed in 2 packages

go vet ./...
exit 0

go tool cover -func=coverage.out
total: 89.6% (statements)

go run ./cmd/windowcheck 09:00-10:00 10:00-11:00
valid maintenance plan: 2 windows

go run ./cmd/windowcheck 09:00-10:00 09:30-11:00
overlap: "09:00-10:00" and "09:30-11:00"
exit status 1

go run ./cmd/windowcheck 9:00-10:00
invalid window "9:00-10:00": use HH:MM-HH:MM
exit status 1
```

The isolated state root contained only the project Install State record. The
installer bin directory was empty; no `oaw-assurance`, `oaw-bridge`, workflow
runtime state, or post-install OAW executable was present. The project Git
worktree was clean at `61de956`.

## Managed identity evidence

```text
POLICY.md fafced6b1bd66291c4bc68acd0aec53150a9d243653198f7d5741b6ad71b4ca0
MATT-SP-HYBRID.md 39aec271c5f003ac6e25be190c1465610f9d2de7b6d1f1157ff9e14192230966
AGENTS.md 56293f6b9a1e911a260eb9d93100458984f882faa2103557bcbc3f76890cbbd6
```
