# MATT-FULL Verification Evidence

## Policy installation and isolation

The temporary `oaw` binary installed the project Policy Set and Codex router.
`oaw check` reported the three providers as missing in the intentionally empty
isolated HOME, while the project install remained clean. That advisory result
did not block the readable Profile or Skill path.

The binary `/tmp/oaw-matt-full.mxJ4P7/bin/oaw` was removed before artifact
verification. The project state root contained only project Install State; no
Assurance, Bridge, runtime state, or post-install executable was present.

## Fresh artifact checks

Run from the project after binary removal and commit `ce3863d`:

```text
go test ./...
Go test: 4 passed in 2 packages

go vet ./...
exit 0

go run ./cmd/checklist RELEASE.md
2/3 complete
```

The missing-file path also returned the declared non-zero error:

```text
read checklist "missing.md": open missing.md: no such file or directory
exit status 1
```

The project Git worktree was clean after verification.

## Managed identity evidence

```text
POLICY.md fafced6b1bd66291c4bc68acd0aec53150a9d243653198f7d5741b6ad71b4ca0
MATT-FULL.md 0cbeca519785e8b99eb1ceb83677a44f928c3ebfd3c9959ae31a3756bc3e1aa8
AGENTS.md 56293f6b9a1e911a260eb9d93100458984f882faa2103557bcbc3f76890cbbd6
HOST_SKILLS_INDEX.txt 25d5af90e9d6338c963be19041542a3613084c611bc2bc728b99d03815a128b9
grill-with-docs/SKILL.md 610d091047bcfb9db0f75c057d15538481a721111579fc5ec7f83ad9131a2165
tdd/SKILL.md 5363bb2775679fe9311fbb67947f95359169c6e7f1fac77c0f25e190bca6cf2f
```
