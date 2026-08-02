# OAW Runtime vNext Written-Spec Verification

**Date:** 2026-08-01
**Result:** Passed
**Scope:** Design artifacts only; no Runtime implementation exists yet

## Fresh Evidence

| Check | Result |
| --- | --- |
| `rtk bash scripts/check-docs.sh` | Exit 0: `PASS: bilingual documentation contracts and local links passed` |
| `rtk bash tests/run.sh` | Exit 0: every existing installer test script passed; terminal output was `PASS: all implemented installer cases passed` |
| Standard unfinished-work marker scan across the design artifacts | No matches (`rg` exit 1 with empty output, as expected) |
| Superseded-contract wording scan across the design artifacts | No matches (`rg` exit 1 with empty output, as expected) |
| Required-contract scan | Found `executor_topology`, `CAPABILITY_SELECTION_REQUIRED`, `DISPATCH_PREPARED`, `DISPATCH_AUTHORIZED`, explicit `ECC-FULL` to `oaw/ecc-engineering` mapping, and non-authoritative shadow migration wording |
| File-size check | Largest design artifact is `spec.md` at 558 lines; every file remains below the 800-line repository limit |
| `rtk git diff --check` | Exit 0 with no whitespace errors |

The full Bash suite exercised CLI parsing, Provider detection, user and project
Adapter lifecycles, cross-scope coordination, drift handling, path containment,
backup and transaction behavior, and documentation contracts. The new files are
documentation-only and were included in the local-link scan.

## Worktree Scope

The verified design changes are limited to `CONTEXT.md`, the Runtime vNext
tracker/spec/evidence directory, and ADRs 0003 and 0004. The untracked
`.serena/` directory is unrelated user state and was not read, changed, or added.

## Ticket 03 Fresh Verification

**Date:** 2026-08-01

**Result:** Passed

**Scope:** Deterministic Request Classifier

| Check | Result |
| --- | --- |
| Go formatting and `git diff --check` | Exit 0 |
| `GOCACHE=/tmp/oaw-build-ticket03 rtk go vet ./...` | Exit 0 |
| `GOCACHE=/tmp/oaw-build-ticket03 rtk go test -race ./...` | Exit 0: 253 tests in 12 packages |
| Repository Go statement coverage | `84.2%`, required minimum `80%` |
| `internal/classification` statement coverage | `92.8%`, target minimum `90%` |
| Classifier fuzz seam | Exit 0 after a fresh 2-second run |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0 |
| Ticket 03 forbidden-surface and secret scans | Exit 0; no matches |
| `govulncheck -show verbose ./...` | Exit 0: 0 reachable symbol or package vulnerabilities |

`govulncheck` reports `GO-2026-5970` in required module
`golang.org/x/text@v0.14.0`, fixed in `v0.39.0`, but confirms that no package or
symbol from the vulnerable path is imported or reachable. Dependency upgrade is
outside Ticket 03 and remains a repository-level follow-up risk.

The verified Ticket 03 paths are limited to the lifecycle tracker and plan, the
classification proposal schema and registry entry, `internal/classification`,
and one classification integration test. The untracked `.serena/` directory
remains excluded.

## Ticket 04 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed with one unavailable optional tool

**Scope:** Profile Recipe Compiler, built-in and custom Recipe integration, and
repository compatibility

| Check | Result |
| --- | --- |
| `rtk go vet ./...` | Exit 0 |
| `rtk go test -race ./...` | Exit 0: 284 tests in 13 packages |
| Repository Go statement coverage | `85.1%`, required minimum `80%` |
| `internal/profile` statement coverage | `91.3%`, target minimum `90%` |
| `rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh` | Exit 0 |
| `rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh` | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all existing installer cases passed |
| `rtk go test ./internal/profile ./internal/integration` | Exit 0: 30 focused tests |
| `rtk go test -race ./internal/profile ./internal/integration` | Exit 0 |
| `rtk go test ./internal/classification -run '^$' -fuzz FuzzDecodeProposalFailsClosed -fuzztime 2s` | Exit 0 |
| `rtk govulncheck ./...` | Not run: command unavailable in environment |

The focused corpus compiled all built-in Recipes, `SP-FULL`, `MATT-FULL`,
`ECC-FULL`, `MATT-SP-HYBRID`, and a user-defined `acme/*` Recipe. It proved
verified-coverage failure, optional ECC handler omission, exact bindings,
stable failure codes, closed loops, and equivalent-input digest equality.

The Ticket 04 implementation is committed at `ebc04a8`; the executable plan
and contract correction are committed at `88ca4e6` and `dd4c50d`. Untracked
`.serena/` state was preserved and not staged.

## Ticket 05 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed

**Scope:** Direct Runtime vertical slice, durable journal recovery, repository
compatibility, and cross-platform compilation

| Check | Result |
| --- | --- |
| `rtk go vet ./...` | Exit 0 |
| `rtk go test -race ./...` | Exit 0: 390 tests in 14 packages |
| Repository Go statement coverage | `86.1%`, required minimum `80%` |
| `internal/runtime` statement coverage | `90.3%`, target minimum `90%` |
| Focused runtime/integration race tests | Exit 0: 127 tests in 2 packages |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all implemented installer cases passed |
| Classification proposal fuzz seam | Exit 0 after a fresh 2-second run |
| Linux and Windows `cmd/oaw` builds | Exit 0 with outputs directed to `/tmp` |
| Linux and Windows runtime test cross-compilation | Exit 0 |
| Secret-pattern scan on Ticket 05 code and dependency files | No matches |
| `rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | Exit 0: 0 reachable vulnerabilities |
| `rtk git diff --check` | Exit 0 |

The Runtime corpus covers durable reply-before-return ordering, restart and
read-only inspection, defensive copies, idempotent START/CONTINUE replay,
revision conflicts, cross-Engine lock contention, matching and conflicting
orphan recovery, corrupt and oversized state, strict predecessor chains,
owner-only permissions, semantic state tampering, Direct authority emptiness,
scope escalation, and Linux/Windows replacement compilation.

The global `govulncheck` command was initially unavailable. The official tool
was therefore run through a temporary versioned `go run`; it reported no called
symbol or imported-package vulnerabilities. It noted two vulnerabilities in
required modules whose affected code is not called. No repository dependency
was changed solely for the scanner.

All `.serena/` directories remain preserved and excluded from commits.

## Ticket 06 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed

**Scope:** Bounded admission, Grant issuance, dispatch authorization,
normalized observations, escalation/uncertainty, recovery, and journal chain
hardening

| Check | Result |
| --- | --- |
| `rtk go test ./... -count=1` | Exit 0: 611 tests passed in 15 packages |
| `rtk go test -race ./...` | Exit 0: 611 tests passed in 15 packages |
| `rtk go vet ./...` | Exit 0 |
| `internal/runtime` statement coverage | `90.2%` |
| `internal/admission` statement coverage | `98.3%` |
| Focused admission/runtime/integration race tests | Exit 0: 329 tests passed |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all implemented installer cases passed |
| Classification proposal fuzz seam | Exit 0 after a fresh 2-second run |
| Linux and Windows `cmd/oaw` builds | Exit 0 |
| Versioned official `govulncheck` | Exit 0: 0 reachable vulnerabilities; 2 unreachable module findings |
| `rtk git diff --check main...HEAD` | Exit 0 |

The Runtime corpus covers all Bounded handshake states, replay-before-revision
checks, restart inspection, concurrent single-Grant admission, matching and
conflicting orphan revisions, immutable Grant/observation state, permission
modes, Resource Lease deferral, and the absence of future Stage/Host authority
fields. Direct Runtime tests remain passing.

## Ticket 07 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed

**Scope:** Workflow Runtime orchestration, projection/recovery behavior,
repository compatibility, security checks, and cross-platform compilation

| Check | Result |
| --- | --- |
| `rtk git diff --cached --check` | Exit 0 |
| `rtk go test ./... -count=1` | Exit 0: 751 tests passed in 15 packages |
| `rtk go test -race ./...` | Exit 0: 751 tests passed in 15 packages |
| `rtk go vet ./...` | Exit 0 |
| `internal/runtime` statement coverage | `90.1%`, required minimum `90%` |
| Repository Go statement coverage | `87.9%`, required minimum `80%` |
| Final focused Workflow invariant tests | Exit 0: 62 tests passed |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all implemented installer cases passed |
| Classification proposal fuzz seam | Exit 0 after a fresh 2-second run |
| Linux and Windows `cmd/oaw` builds | Exit 0 |
| Versioned official `govulncheck` | Exit 0: 0 reachable vulnerabilities; 2 unreachable module findings |

The Workflow corpus verifies the blocking selection Gate, immutable Bundle
generations, fresh review Executors, physical Worktree lease conflicts across
two Engines, graph-controlled observations, stable-boundary switching,
configuration adoption only through an explicit new Bundle, replay and restart
recovery, and fail-closed journal history validation. Integration tests use the
real Configuration load, Provider discovery, and Registry resolution path.

Projection JSON and Markdown are owner-only one-way outputs. Sink errors and
panics cannot change a committed reply; lag sidecars are non-authoritative;
symlink destinations are rejected; deleting or tampering with projection files
does not affect inspection, Grant issuance, recovery, or switching. State scans
found no raw Provider output, credentials, or discovery evidence, and Runtime
never invoked a Host Binding. All `.serena/` directories remain preserved and
excluded from commits.

## Ticket 08 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed

**Scope:** Host records and schemas, built-in and user-trusted Configuration,
Adapter conformance, Workflow admission and Bundle generations, recovery,
security boundaries, and repository compatibility

| Check | Result |
| --- | --- |
| `rtk git diff --check` | Exit 0 |
| `rtk go test ./... -count=1` | Exit 0: 889 tests passed in 17 packages |
| `rtk go test -race ./...` | Exit 0: 889 tests passed in 17 packages |
| `rtk go vet ./...` | Exit 0 |
| `internal/host` statement coverage | `94.8%`, required minimum `90%` |
| `internal/runtime` statement coverage | `90.0%`, required minimum `90%` |
| Repository Go statement coverage | `87.7%`, required minimum `80%` |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all implemented installer and documentation cases passed |
| Classification proposal fuzz seam | Exit 0 after a fresh 2-second run |
| Host conformance receipt fuzz seam | Exit 0 after a fresh 2-second run |
| Linux and Windows `cmd/oaw` builds | Exit 0 |
| Versioned official `govulncheck` | Exit 0: 0 reachable vulnerabilities; 2 unreachable module findings |

Runner-managed and native-managed end-to-end fixtures executed
`RunConformance`, loaded a real user Configuration, discovered and resolved
Providers, selected a Workflow Profile, issued a Stage Grant, authorized
dispatch, returned a normalized observation, restarted, and inspected the same
pinned Host identities. Conformance invoked every declared Binding kind twice;
Runtime did not add an Adapter call. Instruction-only fallback, missing
Features, invalid audit/report evidence, wrong Binding Host/kind, per-run
narrowing, stale Engines, changed Integration generations, historical Bundle
tampering, and projection deletion/tampering all failed closed.

Implementation commits are `b67ecca`, `ae4b37a`, `3ccc5d9`, `c40e4f6`,
`3096294`, and review remediation `60ea7ee`; executable planning is `330ec3f`.
Sensitive fixture credentials, raw audit text, raw Adapter output, and Adapter
errors were absent from Conformance Reports, Runtime state, projections, and
errors. Owner-only permissions remained intact. The nine built-in Host
Integrations are still instruction-only; no first production Runtime Host was
selected. All `.serena/` directories and unrelated worktrees remain preserved.

## Ticket 11 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed

**Scope:** Go `check` compatibility behavior, Bash authority preservation,
read-only same-fixture parity, repository compatibility, security checks, and
cross-platform compilation

| Check | Result |
| --- | --- |
| `rtk git diff --check` | Exit 0 |
| `rtk go test ./... -count=1` | Exit 0: 979 tests passed in 19 packages |
| `rtk go test -race ./...` | Exit 0: 979 tests passed in 19 packages |
| `rtk go vet ./...` | Exit 0 |
| `internal/check` statement coverage | `90.2%`, required minimum `90%` |
| Repository Go statement coverage | `87.5%`, required minimum `80%` |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all implemented installer, documentation, and parity cases passed |
| Same-fixture Bash/Go parity matrix | Exit 0: 32 stream/status/snapshot cases passed |
| Classification proposal fuzz seam | Exit 0 after a fresh 2-second run |
| Host conformance receipt fuzz seam | Exit 0 after a fresh 2-second run |
| Linux and Windows `cmd/oaw` builds | Exit 0 |
| Versioned official `govulncheck` | Exit 0: 0 reachable vulnerabilities; 2 unreachable module findings |
| `rtk git diff --exit-code 9e0daca -- install.sh lib` | Exit 0: authoritative Bash implementation unchanged |

The parity corpus compares stdout, stderr, status, and sandbox snapshots after
running Bash and Go sequentially against the same fixture. It covers user and
project defaults, target normalization and errors, all built-in Provider
diagnostics, hidden Provider versions, physical project roots, clean and
drifted managed/owned targets, invalid and shell-compatible TSV state, shared
destinations, scope/path mismatches, help, and symlinked target coordinates.

Implementation commits are `f99b872`, `f5e894a`, `284a2e6`, `5552950`, and
review remediation `07f1552`; executable planning is `2ed87b3`, and bilingual
shadow documentation is `d9c740b`. Bash remains authoritative and no Go
management cutover occurred. All `.serena/` directories, unrelated branches,
and unrelated worktrees remain preserved.

## Ticket 12 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed

**Scope:** Go `install` rendering, state, prepare/apply and filesystem safety;
Bash authority preservation; same-path mutating parity; repository
compatibility; documentation; security checks; and cross-platform compilation

| Check | Result |
| --- | --- |
| Go formatting and `rtk git diff --check` | Exit 0 |
| `rtk go test ./... -count=1` | Exit 0: 1138 tests passed in 21 packages |
| `rtk go test -race ./...` | Exit 0: 1138 tests passed in 21 packages |
| `rtk go vet ./...` | Exit 0 |
| `internal/management` statement coverage | `90.0%`, required minimum `90%` |
| Repository Go statement coverage | `87.7%`, required minimum `80%` |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all implemented installer, documentation, check-parity, and install-parity cases passed |
| Same-path Bash/Go install parity matrix | Exit 0: 45 stream/status/tree/byte/backup cases passed |
| Classification proposal fuzz seam | Exit 0 after a fresh 2-second run |
| Host conformance receipt fuzz seam | Exit 0 after a fresh 2-second run |
| Linux and Windows `cmd/oaw` builds | Exit 0 |
| Versioned official `govulncheck` | Exit 0: 0 reachable vulnerabilities; 2 unreachable module findings |
| `rtk git diff --exit-code main -- install.sh lib internal/cli/run.go` | Exit 0: Bash authority and public routing unchanged |

The parity corpus replays deterministic setup at the same physical sandbox
path for Bash and Go. It compares status, stdout, stderr, path type, mode,
symlink target, checksum, every regular-file byte, Install State, backup-tree
bytes, and zero-write refusal/dry-run snapshots. The 45 cases cover fresh,
repeated, additive, and default user installs; all four user and nine project
targets; managed-file newline placement; shared destinations; physical paths;
cross-scope coordination; dry-run; backup references; drift, collision, parser,
scope, symlink, hostile-path, and later-target preflight failures.

Implementation commits are `a19baa1`, `bb9c950`, `3349c42`, `41b616e`, and
`d51750d`; executable planning is `c8cbbdc`, and review/documentation
remediation is `6f15694`. Bash remains authoritative, the internal Go install
driver remains parity-only, and no Go management cutover occurred. All
`.serena/` directories, unrelated branches, and unrelated worktrees remain
preserved.
