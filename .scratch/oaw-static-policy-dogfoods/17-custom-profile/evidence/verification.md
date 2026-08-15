# Custom Profile Verification Evidence

## Project Policy installation

The temporary `oaw` executable installed the project Policy Set and Codex
Activation Router. The Custom Profile was then created under
`project/.oaw/profiles/`; the only user-scope file was an isolated same-ID
fixture used to prove source qualification.

The isolated diagnostic before execution reported all three Providers missing
while the project Codex installation was clean. It did not block Profile
creation, selection, or either task.

## Fresh artifact checks after binary removal

`/tmp/oaw-custom-profile.MHQOP4/bin/oaw` was removed before these commands:

```text
go test ./...
Go test: 10 passed in 2 packages

go vet ./...
exit 0

go test -race ./...
Go test: 10 passed in 2 packages

go run ./cmd/manifestcheck RELEASE-MANIFEST
valid release manifest: 1.2.3

go run ./cmd/manifestcheck --require owner RELEASE-MANIFEST
valid release manifest: 1.2.3
```

The isolated state root contained only the project Install State record. The
installer bin directory was empty; no `oaw-assurance`, `oaw-bridge`, runtime
state, or post-install OAW executable existed. The project Git worktree was
clean at `f98f202`.

## Managed identity evidence

```text
POLICY.md fafced6b1bd66291c4bc68acd0aec53150a9d243653198f7d5741b6ad71b4ca0
release-check.md 9688e3f490c2123367089e72f047c2e62121b5f5f56db802fc3edd356aa0a2f3
AGENTS.md 56293f6b9a1e911a260eb9d93100458984f882faa2103557bcbc3f76890cbbd6
user fixture d82ad4728bfc9372a1352faa2c4befa9d9dbb853727b36fd315bcec6546bdcee
```
