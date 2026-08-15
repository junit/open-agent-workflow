# SP-FULL Verification Evidence

## Policy installation

The temporary `oaw` binary installed a project Policy Set and Codex router:

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

After installation, `/tmp/oaw-sp-full.1yKmHN/bin/oaw` was removed. The state
root listed only the project Install State file; no `oaw-assurance`,
`oaw-bridge`, runtime state, or post-install executable was present.

## Fresh artifact checks

Run from the project after binary removal:

```text
go test ./...
Go test: 8 passed in 2 packages

go vet ./...
exit 0

go run ./cmd/slugify "  Fresh: Verification--2026  "
fresh-verification-2026
```

The project Git worktree was clean after commit `abe6180`.

## Managed identity evidence

```text
POLICY.md  fafced6b1bd66291c4bc68acd0aec53150a9d243653198f7d5741b6ad71b4ca0
SP-FULL.md 8239cd7348ef8117eaeb9d3da9c45245bf277b2844a9ab16be3b040004c0b7de
AGENTS.md 56293f6b9a1e911a260eb9d93100458984f882faa2103557bcbc3f76890cbbd6
```
