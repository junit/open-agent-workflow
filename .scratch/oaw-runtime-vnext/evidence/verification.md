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

## Ticket 13 Fresh Verification

**Date:** 2026-08-02

**Result:** Passed

**Implementation fixed point:** `9fb61d8`

**Scope:** Go update/uninstall parity, forced recovery, verified backups,
fault-injected rollback, security containment, Bash authority preservation,
documentation, and repository compatibility

| Check | Result |
| --- | --- |
| Go formatting and `rtk git diff --check` | Exit 0 |
| `rtk go test ./... -count=1` | Exit 0: 1419 tests passed in 21 packages |
| `rtk go test -race ./...` | Exit 0: 1419 tests passed in 21 packages |
| `rtk go vet ./...` | Exit 0 |
| Repository Go statement coverage | `88.3%` (`7132/8080 = 88.267327%`), required minimum `80%` |
| `internal/management` statement coverage | `90.1%` (`2460/2731 = 90.076895%`), required minimum `90%` |
| Focused Go security/containment corpus | Exit 0: 50 tests passed |
| Bash syntax and ShellCheck | Exit 0 |
| Bilingual docs contract and link checker | Exit 0: `tests/10-docs-test.sh` and `scripts/check-docs.sh` passed |
| `rtk bash tests/run.sh` | Exit 0: complete installer, documentation, check, install-parity, and mutation-parity suite passed |
| Same-path Bash/Go install parity | Exit 0: 45 cases passed |
| Same-path Bash/Go update/uninstall parity | Exit 0: 51 cases passed |
| Bash security and containment corpora | Exit 0: 17 security plus 5 containment cases passed |
| Six mutation fuzz targets | Exit 0: each ran independently for 10 seconds |
| Linux and Windows `cmd/oaw` builds | Exit 0; outputs directed to `/tmp` |
| Versioned official `govulncheck` | Exit 0: 0 reachable and 0 imported-package vulnerabilities; 2 unreachable required-module findings |
| Bash/public authority diff | Exit 0: `install.sh`, `lib`, `internal/cli/run.go`, and `cmd/oaw` unchanged from `main` |
| Ticket diff secret-pattern scan | Exit 0: no credential or private-key patterns found |

The six fuzz targets were `FuzzMutationAction`, `FuzzManagedRemoval`,
`FuzzBackupManifest`, `FuzzMutationRollbackOrder`, `FuzzMutationPathSuffix`, and
`FuzzMutationStateFields`. The retained NUL corpus under
`internal/management/testdata/fuzz/FuzzBackupManifest` also passed.

The 51 mutation parity fixtures compare status, stdout, stderr, path type,
mode, symlink target, exact regular-file bytes, Install State, directories, and
normalized operation-backup trees after replay at the same physical path. They
cover every user/project target, shared and cross-scope state, clean and dry-run
operations, partial/final uninstall, drift refusal, forced target/policy
recovery, missing-marker repair, manual recovery, malformed state, redirected
directories/backups, repeated-separator HOME/XDG root spelling, hostile project
names, changed checkouts, and later preflight failures.

The Go-only fault matrix covers every before/after backup, target, directory,
Policy, state, and namespace-directory point. Rollback restores pre-existing
bytes and exact modes, refuses concurrent changes, removes operation-created
artifacts, preserves a completed verified backup, and reports rollback failure
without file-content leakage. Review remediation additionally proves
existing-only roots, same-inode bounded reads, backup-source identity, final
source revalidation before manifest activation, and parent/final identity for
directory removal.

`govulncheck` reported no vulnerable symbol or imported package. It noted two
findings only in required modules whose affected code is not called; no
dependency was changed solely for the scanner. WSL release smoke remains a
Ticket 14 cutover gate and is not claimed by this internal shadow/parity ticket.

Implementation commits are `8e9c79b`, `0f07857`, `d9254a9`, `ef329ce`,
`0b818c1`, `341d528`, `367b598`, review remediation `0fda0c0`, and final
coverage/containment evidence `d4f4159`; canonical-equivalent path remediation
is `9fb61d8`, executable planning is `43b56ce`, and bilingual boundary
documentation is `b4c98f7`. Bash remains authoritative,
public Go management routing remains closed, and Ticket 14 alone owns cutover.
All `.serena/` directories, unrelated branches, and unrelated worktrees remain
preserved.

## Pre-Ticket 09 Tracker Consistency Audit

**Date:** 2026-08-02

**Result:** Passed

The current `main` implementation was rechecked before normalizing the stale
issue projections for Tickets 01 through 06. The isolated-worktree baseline
passed 1419 tests in 21 packages. Ticket 01 catalog, schema, canonical JSON,
built-in asset, and CLI surfaces passed 162 focused tests in six packages;
Ticket 02 configuration, trust, discovery, registry, and integration surfaces
passed 40 focused tests in four packages; and Ticket 03 classifier, critical
eval, Direct, Bounded, Workflow, and integration surfaces passed 52 focused
tests in two packages. Tickets 04 through 08 already carried checked acceptance
items and dedicated review/verification evidence.

The issue edits only align completed status spelling and checked acceptance
projections with the already merged, currently passing implementation. They do
not alter scope, dependency edges, Runtime authority, Host selection, or the
Ticket 09 requirement for explicit user selection of the first production
Runtime Host.

## Ticket 09 Fresh Verification

**Date:** 2026-08-03

**Result:** Passed

**Implementation fixed point:** `0cb396d`

**Scope:** Codex Host asset admission, strict Runtime transport, `oaw run`
dispatch and replay recovery, bounded Codex process normalization, capability
gating, project configuration reload, bilingual documentation, and preserved
Bash management authority.

| Check | Result |
| --- | --- |
| `rtk git diff --check` | Exit 0 |
| `rtk go test ./... -count=1` | Exit 0: 1463 tests passed in 22 packages |
| `rtk go test -race ./...` | Exit 0: 1463 tests passed in 22 packages |
| `rtk go vet ./...` | Exit 0 |
| Repository Go statement coverage | `87.2%`, required minimum `80%` |
| Runtime transport fuzz target | Exit 0: `FuzzDecodeFrameFailsClosed`, 2 seconds |
| Codex JSONL fuzz target | Exit 0: `FuzzNormalizeJSONLFailsClosed`, 2 seconds |
| Bash syntax and ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: complete installer and documentation suite passed |
| Bilingual docs and link checks | Exit 0: `tests/10-docs-test.sh` and `scripts/check-docs.sh` passed |
| Linux and Windows `cmd/oaw` builds | Exit 0 |
| Versioned official `govulncheck` | Exit 0: 0 reachable vulnerabilities; 2 unreachable module findings |

The public CLI fixture test runs an isolated HOME, XDG configuration root,
project root, PATH fixture, and Runtime state root. It executes a deterministic
Codex fixture only, verifies the exact Runtime handshake and stderr separation,
replays the same dispatch through a newly constructed CLI Runner, and confirms
the fixture process runs exactly once. A second recovery fixture proves an
authorized invocation without an observation transitions to
`EXECUTION_UNCERTAIN` without invoking the Host. Transport fuzzing found and
closed the previously missing frame-kind validation before this fixed point.

The selected Codex Manifest, audit, Conformance, and Integration digests are
recorded in the Ticket 09 issue and Host selection evidence. All other built-in
Hosts remain `instruction-only`; no project configuration or discovery result
can promote them. `install.sh` and existing management renderers are unchanged,
and no real paid/model-backed `codex exec` was invoked. `.serena/` and unrelated
worktrees remain preserved.

## Ticket 10 Fresh Verification

**Date:** 2026-08-03

**Result:** Passed

**Implementation fixed point:** `8fdd78a`

**Scope:** Policy vNext classification and ownership, extensible Profiles,
Policy-only Host limits, independent Active Ticket state, redacted Runtime
project projections, Codex bootstrap parity, and bilingual lifecycle guidance.

| Check | Result |
| --- | --- |
| `rtk git diff --check` | Exit 0 |
| `rtk go test ./... -count=1` | Exit 0: 1465 tests passed in 22 packages |
| `rtk go test -race ./... -count=1` | Exit 0: 1465 tests passed in 22 packages |
| `rtk go vet ./...` | Exit 0 |
| Repository Go statement coverage | `87.2%`, required minimum `80%` |
| Runtime transport fuzz target | Exit 0: `FuzzDecodeFrameFailsClosed`, 2 seconds |
| Codex JSONL fuzz target | Exit 0: `FuzzNormalizeJSONLFailsClosed`, 2 seconds |
| Bash syntax and warning-level ShellCheck | Exit 0 |
| `rtk bash tests/run.sh` | Exit 0: all installer, security, docs, and Bash/Go parity cases passed |
| Bilingual docs and link checks | Exit 0: `tests/10-docs-test.sh` and `scripts/check-docs.sh` passed |
| Linux and Windows `cmd/oaw` builds | Exit 0 |
| Versioned official `govulncheck` | Exit 0: 0 reachable vulnerabilities; 2 unreachable required-module findings |

Projection-focused tests validate selected/unselected templates, Profile
switch generations, active stage, independent Active Ticket, sorted and
deduplicated evidence references, explicit current/lagging status, owner-only
file modes, symlink rejection, restart non-authority, and failure/panic lag
recording. Redaction checks reject complete Grant fields, invocation/executor
identity, raw output, credentials, and provider output in JSON and Markdown.

The complete Bash oracle passed after the policy and renderer changes, and the
Go shadow path remained byte-equivalent. No real paid/model-backed `codex exec`
was invoked. Bash remains authoritative until Ticket 14; `.serena/`, the
preserved stash, and unrelated branches/worktrees remain untouched.
