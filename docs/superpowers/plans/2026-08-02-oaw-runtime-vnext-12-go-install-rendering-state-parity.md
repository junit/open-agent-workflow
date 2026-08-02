# OAW Runtime vNext Go Install, Rendering and State Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a non-authoritative Go shadow implementation of Bash `install`
whose argument behavior, rendering, output, file modes, Install State, directory
ownership, idempotency, cross-scope policy coordination, dry-run effects, and
backup-reference behavior match the current Bash command.

**Architecture:** Extract Ticket 11's frozen management compatibility domain
behind `internal/management`, keeping `internal/check` as a source-compatible
facade. Build install as an immutable prepare/apply pipeline: preparation reads
and validates all coordinates and state, produces deterministic in-memory
artifacts and actions, and performs no writes; apply revalidates the complete
plan, renders dry-run notes or performs scoped atomic replacements in Bash
order. An internal-only shadow driver is built by the parity harness; the public
`oaw` CLI and `install.sh` remain unchanged.

**Tech Stack:** Go 1.26 standard library, embedded `VERSION` and canonical
Policy, existing POSIX checksum implementation, frozen TSV Install State,
Bash 3.2 oracle fixtures, table-driven unit tests, same-path replay snapshots,
race/coverage/fuzz/platform gates.

---

## Scope Boundary

Ticket 12 owns:

- an internal Go install shadow driver with the Bash `install` option grammar;
- exact user/project target defaults, physical project identity, destination
  ownership, renderer bytes, managed-block placement, and shared destinations;
- strict reads plus exact writes of legacy TSV Install State, including target
  registry order, checksums, origins, owned directories, and cross-scope policy
  references;
- immutable preparation, no-write dry runs, scoped atomic replacement, modes,
  action ordering, repeated installs, and all-or-nothing preflight failures;
- preservation of foreign content outside OAW-owned blocks/files and sibling
  paths;
- Bash install's backup contract: install never creates a new operation backup,
  even with `--force`; existing valid `backup` records survive state extension
  and cross-scope state-reference rewrites; rejected installs create no backup;
- a same-physical-path Bash/Go replay harness comparing stdout, stderr, status,
  file types, modes, symlinks, bytes, state records, and backup-tree effects.

It does not:

- edit `install.sh` or `lib/`, route public `oaw install`, make Go authoritative,
  turn Bash into a wrapper, or document a management cutover;
- implement Go `update` or `uninstall`, repair drift, create forced recovery
  backups, restore backups, or port Ticket 13's complete security transaction
  and fault-injection matrix;
- reuse Runtime State as Install State, alter Provider discovery, change target
  paths/renderers, select a Profile, or download Policy/provider content.

The current Bash result is the oracle. Any desired Bash behavior change is a
separate decision and must not be folded into parity remediation.

## Locked Interfaces and File Map

The management module exposes only immutable compatibility values:

```go
// policy.go
func CanonicalPolicy() []byte // returns a copy of embedded policy/ENGINEERING.md

// internal/management/management.go
type Environment struct {
    Home, ConfigHome, StateHome, Path string
}

type Source struct { /* private copied version and policy */ }
func NewSource(version string, policy []byte) (Source, error)

type InstallRequest struct {
    Project string
    Targets string
    DryRun  bool
    Force   bool
}

type Result struct {
    Lines    []string
    Trailing string
}

type PreparedInstall struct { /* private immutable coordinates/artifacts/actions */ }

func PrepareInstall(Source, Environment, InstallRequest) (PreparedInstall, error)
func ApplyInstall(PreparedInstall) (Result, error)
func Install(Source, Environment, InstallRequest) (Result, error)
func WriteResult(Result, io.Writer) error
```

`internal/check` remains a facade with aliases for its current `Environment`,
`Request`, `Result`, and `Error` plus delegating `Execute` and `Write`; Ticket 11
callers and tests must not change behavior. `internal/management/render.go` owns
pure adapter/block/file rendering. `install_prepare.go` owns state merging and
actions. `install_apply.go` owns revalidation and filesystem effects.
`install_state.go` owns exact serialization. `filesystem.go` owns scoped atomic
replacement. `internal/cli/shadow_install.go` owns only Bash-compatible parsing
and output; `internal/cmd/oaw-install-shadow` is test-only and is not a release
entrypoint.

## Task 1: Extract the shared management compatibility domain

**Files:**
- Create: `internal/management/management.go`
- Move: `internal/check/{checksum,health,managed,paths,providers,readiness,state,targets}.go`
- Move: internal-package tests for checksum, managed blocks, paths, Providers,
  and state into `internal/management/`
- Modify: `internal/check/check.go`
- Modify: external `internal/check/*_test.go` imports only if required

- [ ] **Step 1: Add a failing facade-compatibility test.**

Add `internal/check/facade_test.go` that calls both the existing `check.Execute`
and the proposed `management.Check` over the same catalog and fixture, compares
`Result` and typed errors with `reflect.DeepEqual`, writes both results, and
asserts byte equality. Include success, invalid target, invalid state, and the
partial-output symlink case.

- [ ] **Step 2: Run the facade test and verify RED.**

```bash
rtk go test ./internal/check -run 'FacadeCompatibility' -count=1
```

Expected: compile failure because `internal/management` and `Check` do not yet
exist; no implementation file changes before this RED evidence.

- [ ] **Step 3: Move the compatibility domain and add the facade.**

Move the Ticket 11 implementation without changing algorithms. Rename the
entrypoint to `management.Check`, keep the check-specific request name, and use
aliases/delegation in `internal/check/check.go`:

```go
type Environment = management.Environment
type Request = management.CheckRequest
type Result = management.Result
type Error = management.Error

func Execute(value catalog.Catalog, environment Environment, request Request) (Result, error) {
    return management.Check(value, environment, request)
}

func Write(result Result, output io.Writer) error {
    return management.WriteResult(result, output)
}
```

Do not introduce exported mutable slices or maps. Move internal tests next to
the implementation; keep the black-box-facing tests on the facade.

- [ ] **Step 4: Run Ticket 11 and repository regression gates.**

```bash
rtk gofmt -w internal/check internal/management
rtk go test ./internal/check ./internal/management ./internal/cli -count=1
rtk go test -race ./internal/check ./internal/management ./internal/cli
rtk bash tests/11-check-parity-test.sh
```

Expected: all existing check bytes, streams, statuses, and snapshots remain
unchanged.

- [ ] **Step 5: Commit the extraction.**

```bash
rtk git add internal/check internal/management
rtk git commit -m "refactor: extract management compatibility domain"
```

## Task 2: Lock pure Policy, adapter, managed-file, and state rendering

**Files:**
- Create: `policy.go`
- Create: `policy_test.go`
- Create: `internal/management/render.go`
- Create: `internal/management/render_test.go`
- Create: `internal/management/install_state.go`
- Create: `internal/management/install_state_test.go`

- [ ] **Step 1: Write failing source-copy and renderer tests.**

Assert `CanonicalPolicy()` equals `policy/ENGINEERING.md`, is non-empty, and
returns defensive copies. For every user/project target, assert the exact Bash
renderer bytes. Cover shared project `AGENTS.md`, front matter, backticks,
absolute policy paths with spaces, and all terminal newlines.

For managed files cover these exact Bash rules:

```text
missing/empty file                         -> block only
existing content with final newline        -> existing bytes, then block
existing content without final newline     -> block, then exact existing bytes
one valid block                            -> replace block, preserve outside bytes
zero markers                               -> installable
duplicate/reversed/partial markers         -> reject as untracked drift
```

- [ ] **Step 2: Run renderer tests and verify RED.**

```bash
rtk go test ./ ./internal/management -run 'CanonicalPolicy|Render|ManagedFile|SerializeInstallState' -count=1
```

Expected: missing Policy and renderer symbols.

- [ ] **Step 3: Embed Policy and implement pure renderers.**

Use root `//go:embed policy/ENGINEERING.md`; return `bytes.Clone`. Implement
one closed `(scope,target)` renderer switch matching `lib/render.sh`, then wrap
managed content with the existing markers. No renderer reads or writes disk.

```go
func renderTarget(id targetID, scope scope, policyPath string) ([]byte, error)
func renderManagedBlock(id targetID, scope scope, policyPath string) ([]byte, error)
func renderManagedFile(current, block []byte) ([]byte, error)
```

- [ ] **Step 4: Implement exact Install State serialization.**

Serialize validated immutable state in Bash order: `format`, `version`,
`scope`, optional `project`, `policy`, optional `backup`, ordered `directory`
rows, then registry-ordered `target` rows. Reject unsafe fields, duplicates,
bad shared destinations, invalid origins, and non-absolute coordinates before
returning bytes. Reparse rendered bytes with the existing strict parser in
tests and compare every value.

- [ ] **Step 5: Run focused GREEN and commit.**

```bash
rtk gofmt -w policy.go policy_test.go internal/management
rtk go test ./ ./internal/management -run 'CanonicalPolicy|Render|ManagedFile|SerializeInstallState' -count=1
rtk git add policy.go policy_test.go internal/management
rtk git commit -m "feat: render install artifacts in Go"
```

## Task 3: Prepare immutable install actions and state merges without writes

**Files:**
- Create: `internal/management/install_prepare.go`
- Create: `internal/management/install_prepare_test.go`
- Modify: `internal/management/management.go`
- Modify: shared target/path/state files only for extracted helpers

- [ ] **Step 1: Write failing preparation tests with complete snapshot guards.**

Build table fixtures for fresh/repeated/additive user installs; every project
target; physical project symlinks and spaces; project `codex+opencode` shared
destination; pre-existing managed user content with/without newline; existing
owned-file refusal; untracked/malformed markers; invalid/mismatched/drifted
state; checkout Policy/version mismatch; registry-order normalization; and a
later invalid target after a valid first target. Snapshot all roots before and
after `PrepareInstall` and require byte equality.

- [ ] **Step 2: Run preparation tests and verify RED.**

```bash
rtk go test ./internal/management -run 'PrepareInstall' -count=1
```

Expected: `PrepareInstall` is undefined.

- [ ] **Step 3: Implement immutable source/request resolution and action values.**

Validate and defensively copy source bytes. Reuse frozen scope/target/path rules.
Represent actions as private values containing type, label, source bytes,
destination, mode, allowed root, and relative suffix; reject conflicting
actions for one destination unless rendered bytes and coordinates are equal.
Return copied slices from test-only accessors.

- [ ] **Step 4: Implement installed-state merge and cross-scope coordination.**

Match `operation_install` exactly:

- validate an existing current-scope state and live Policy/targets/directories;
- reject checkout/version drift with `installed content differs from this checkout; run update`;
- preserve installed origins and backup reference;
- refuse owned-file collisions and untracked markers, including with `Force`;
- merge targets in registry order and normalize checksums for shared paths;
- inherit only valid namespace directory ownership from other live states;
- record only directories absent during preparation;
- prepare current state plus every valid same-Policy cross-scope state reference
  in deterministic Bash glob order, preserving each state's backup reference.

Do not create a backup action. Assert `Force=false` and `Force=true` produce the
same install plan for clean input and the same refusal for conflicts.

- [ ] **Step 5: Implement deterministic dry-run/action prediction.**

Compute Bash action order without writes: selected targets, target-directory
effects as applicable, Policy, then state/state-reference actions. Use exactly
`unchanged: <label>`, `would-create: <path>`, and `would-update: <path>` with one
newline per result line. Dry-run still performs full validation and preparation.

- [ ] **Step 6: Run focused GREEN, race, coverage, and commit.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management -run 'PrepareInstall|InstallPlan|DryRun|StateMerge|BackupReference' -count=1
rtk go test -race ./internal/management
rtk go test ./internal/management -coverprofile=/tmp/oaw-ticket12-management.cover -count=1
rtk go tool cover -func=/tmp/oaw-ticket12-management.cover
rtk git add internal/management
rtk git commit -m "feat: prepare Go install operations"
```

Expected: management coverage remains at least 90% and preparation mutates no
fixture.

## Task 4: Apply scoped atomic install actions and preserve ownership

**Files:**
- Create: `internal/management/filesystem.go`
- Create: `internal/management/filesystem_test.go`
- Create: `internal/management/install_apply.go`
- Create: `internal/management/install_apply_test.go`

- [ ] **Step 1: Write failing apply and fault-boundary tests.**

Cover modes `0600` for Policy/state and `0644` for targets; existing user bytes;
created vs pre-existing directories; sibling preservation; idempotent mtime and
bytes for unchanged files; target/Policy/state output order; dry-run zero writes;
preflight rejection before the first write; symlink and non-directory
components; destination appearance after preparation; source/state change after
preparation; and write failure reporting after any already-completed Bash-order
action. Assert no backup directory is created for any install case.

- [ ] **Step 2: Run apply tests and verify RED.**

```bash
rtk go test ./internal/management -run 'ApplyInstall|AtomicReplace|OwnedDirector' -count=1
```

Expected: apply/filesystem symbols are missing.

- [ ] **Step 3: Implement scoped directory creation and atomic replacement.**

Validate every component with `Lstat`, never follow a symlink, create missing
components one at a time, and record only planned directories actually created.
Write a same-directory private temporary file, set the final mode, revalidate
the destination coordinate, rename atomically, and sync the file and directory
where supported. Clean temporary files on failure. Never use a shell or expand
state-derived text.

- [ ] **Step 4: Implement final-plan revalidation and Bash-order apply.**

Before any mutation, revalidate all roots, paths, existing state/live bytes,
planned-directory absence, action uniqueness, and immutable rendered checksums.
For dry-run, emit predicted lines only. For real apply, replace targets in
registry order, then Policy, then current and cross-scope states. Report
`create: <path>`, `update: <path>`, or `unchanged: <label>` exactly. Verify that
created directories equal the prepared ownership set before returning.

- [ ] **Step 5: Run GREEN, full regressions, and commit.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management -count=1
rtk go test -race ./internal/management
rtk bash tests/11-check-parity-test.sh
rtk git diff --exit-code main -- install.sh lib
rtk git add internal/management
rtk git commit -m "feat: apply Go install operations"
```

## Task 5: Add the internal shadow CLI and mutating black-box parity harness

**Files:**
- Create: `internal/cli/shadow_install.go`
- Create: `internal/cli/shadow_install_test.go`
- Create: `internal/cmd/oaw-install-shadow/main.go`
- Create: `tests/12-install-parity-test.sh`
- Modify: `tests/test-helper.sh`
- Modify: `tests/run.sh`
- Do not modify: `internal/cli/run.go` public routing

- [ ] **Step 1: Write failing parser and shadow-authority tests.**

Assert the shadow parser accepts Bash install's one `--target`, one `--project`,
one `--dry-run`, one `--force`, and one help flag; rejects missing/empty values,
duplicates, unknown options, and operands with exact status 64 messages. Assert
the normal `cli.Run([]string{"install"}, ...)` still follows the existing public
catalog error path and never calls management install.

- [ ] **Step 2: Implement the internal driver without public routing.**

`RunShadowInstall(args, stdout, stderr)` accepts the full argument vector,
requires first argument `install`, constructs `Source` from embedded
Version/Policy and environment roots, calls management install, writes partial
or complete stdout before typed stderr, and maps statuses exactly. The internal
command's main passes `os.Args[1:]` only to this function. Do not add `install` handling to
`internal/cli.Run`, public help, README usage, or release packaging.

- [ ] **Step 3: Build a same-path replay harness and verify RED.**

Build the internal driver once. For each case, allocate one fixed sandbox path
and define a deterministic setup function. Invoke setup, snapshot, run Bash and
save stdout/stderr/status/final snapshot; remove and recreate that same sandbox
path, invoke the same setup function again, then run Go. Compare every
stream, status, path type, mode, symlink target, and checksum. Also compare exact
state and backup-tree bytes. A mismatch must fail before promotion.

```bash
rtk bash tests/12-install-parity-test.sh
```

Expected before remediation: the first Bash/Go stream or tree mismatch fails.

- [ ] **Step 4: Complete the parity matrix.**

Include fresh/default/idempotent/additive user installs; all four user
renderers; existing content with and without newline; all nine project targets;
shared project AGENTS; physical paths with spaces; cross-scope policy/state
coordination; dry-run; registry normalization; exact state checksums/modes;
owned-directory records; prior backup-reference preservation; no new backups;
owned-file collision; untracked/malformed markers; invalid/drifted/mismatched
state; checkout mismatch; `--force` conflicts; target/scope/parser failures;
symlink components; hostile inert path text; and later-target preflight failure.

- [ ] **Step 5: Run the legacy suite and commit.**

```bash
rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk bash tests/run.sh
rtk git diff --exit-code main -- install.sh lib
rtk git add internal/cli/shadow_install.go internal/cli/shadow_install_test.go internal/cmd/oaw-install-shadow tests
rtk git commit -m "test: enforce Bash Go install parity"
```

## Task 6: Document shadow boundaries, review, verify, and close Ticket 12

**Files:**
- Modify: `docs/en/installer.md`
- Modify: `docs/zh/installer.md`
- Modify: `tests/10-docs-test.sh`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/issues/12-go-install-rendering-and-state-parity.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`
- Modify: this plan

- [ ] **Step 1: Add failing bilingual authority assertions.**

Require both guides to say the internal Go install driver is parity-only,
`install.sh` remains authoritative, public `oaw install` is not enabled, normal
install creates no backup, existing backup references are preserved, and
Ticket 13 owns update/uninstall/forced-backup parity. Reject cutover wording.

- [ ] **Step 2: Document the boundary and run docs tests.**

```bash
rtk bash tests/10-docs-test.sh
rtk bash tests/12-install-parity-test.sh
```

- [ ] **Step 3: Perform inline two-axis review and remediation.**

Review `main...HEAD` separately for repository standards and Ticket 12 spec.
Check Bash drift, public authority leakage, duplicate management rules, mutable
source/plan state, writes during prepare/dry-run, partial preflight mutation,
foreign-content loss, renderer/state/checksum mismatch, shared destinations,
cross-scope corruption, backup creation/reference loss, symlink following,
TOCTOU gaps, unbounded reads, unsafe modes, temp-file leaks, output/status/order
differences, platform assumptions, and Ticket 13 scope creep. Fix every
Critical/High/Important issue and rerun focused parity.

- [ ] **Step 4: Run the complete verification matrix.**

```bash
rtk gofmt -w .
rtk git diff --check
rtk go test ./... -count=1
rtk go test -race ./...
rtk go vet ./...
rtk go test ./internal/management -coverprofile=/tmp/oaw-ticket12-management.cover -count=1
rtk go tool cover -func=/tmp/oaw-ticket12-management.cover
rtk go test ./... -coverprofile=/tmp/oaw-ticket12-all.cover -count=1
rtk go tool cover -func=/tmp/oaw-ticket12-all.cover
rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk bash tests/run.sh
rtk go test ./internal/classification -run '^$' -fuzz FuzzDecodeProposalFailsClosed -fuzztime 2s
rtk go test ./internal/host -run '^$' -fuzz FuzzConformanceReceiptFailsClosed -fuzztime 2s
rtk env GOOS=linux GOARCH=amd64 go build -o /tmp/oaw-ticket12-linux ./cmd/oaw
rtk env GOOS=windows GOARCH=amd64 go build -o /tmp/oaw-ticket12-windows.exe ./cmd/oaw
rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...
rtk git diff --exit-code main -- install.sh lib
```

Expected: management coverage is at least 90%, total Go coverage stays above
80%, every parity/legacy case passes, and there are no reachable vulnerabilities.

- [ ] **Step 5: Close artifacts, commit, and fast-forward `main`.**

Mark Ticket 12 completed, record the exact review/verification fixed point,
check every acceptance item, and preserve `.serena/`, branches, and unrelated
worktrees. Commit closure with:

```bash
rtk git add docs .scratch/oaw-runtime-vnext
rtk git commit -m "docs: close Go install parity ticket"
```

After a fresh `rtk go test ./... -count=1`, fast-forward `main` without deleting
the Ticket branch or worktree.

## Plan Self-Review

- Acceptance item 1 is protected by the `main -- install.sh lib` diff gate, the
  hidden internal driver, and explicit no-public-route tests.
- Acceptance item 2 maps to Tasks 2-5: exact render bytes, checksums, modes,
  state rows, directories, dry-run notes, streams, and snapshots.
- Acceptance item 3 maps to managed-file byte fixtures, sibling sentinels,
  same-path tree comparison, preflight snapshots, and ownership checks.
- Acceptance item 4 uses the actual Bash install contract: no new operation
  backup, preserved valid backup references, and no backup on refusal for every
  scope/target. Ticket 13 retains forced backup creation and verification.
- Acceptance item 5 is enforced by immutable preparation, restored same-path
  parity fixtures, and unchanged authoritative Bash sources.
- The types and ownership stay consistent across Tasks 1-6; the public Runtime
  and CLI surfaces do not acquire mutation authority.
- Placeholder scan found no deferred implementation markers or vague edge-case
  steps; Ticket 13 and Ticket 14 work is explicitly excluded.
