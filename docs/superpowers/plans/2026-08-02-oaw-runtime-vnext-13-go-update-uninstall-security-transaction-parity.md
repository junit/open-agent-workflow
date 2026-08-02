# OAW Runtime vNext Go Update, Uninstall and Security Transaction Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add non-authoritative Go shadow implementations of Bash `update` and
`uninstall` whose managed-content, Install State, forced-recovery backup,
diagnostic, dry-run, containment, and normal success/failure behavior match the
current Bash commands, with deterministic fault-injection rollback that
preserves all pre-existing user bytes.

**Architecture:** Generalize Ticket 12's immutable prepare/apply seam into a
closed mutation plan with `replace`, `remove`, and `retain` effects. Preparation
performs the complete Bash preflight, snapshots every destination, computes
state/policy/directory effects, and reserves any forced-backup identity without
writing. Apply revalidates the plan, completes and verifies a private backup
before the first forced mutation, executes root-scoped atomic effects in Bash
order, and records enough inverse state to roll back injected Go failures. A
test-only shadow driver exposes all three management verbs; `install.sh` and the
public Go CLI remain unchanged and authoritative cutover remains Ticket 14.

**Tech Stack:** Go 1.26 standard library including `os.Root`, embedded canonical
Policy, frozen TSV Install State and backup manifest formats, POSIX `cksum`
compatibility, Bash 3.2 oracle fixtures, table-driven/fuzz/fault-injection tests,
same-path replay snapshots, race/coverage/vet/platform/security gates.

---

## Scope Boundary

Ticket 13 owns:

- internal Go `update` and `uninstall` preparation/application with exact Bash
  command grammar, target selection, status codes, diagnostics, result ordering,
  dry-run behavior, state coordination, shared destinations, and owned-directory
  cleanup;
- clean drift refusal plus Bash-compatible `--force` recovery of content drift,
  uniquely identifiable missing markers, ambiguous manual-recovery cases, and
  policy drift;
- private operation backups with exact `manifest.tsv` fields, candidate order,
  checksums, modes, source revalidation, active-backup enforcement, and Install
  State references;
- deterministic internal fault points around every mutation effect, reverse
  rollback of already-applied Go effects, and proof that pre-existing user bytes
  and modes survive each injected failure;
- mutation-specific containment for unsafe roots/suffixes, project identity,
  symlink and path races, bounded reads, inert control characters, and output
  redaction of file contents and credential-like fixture values;
- an internal-only Bash/Go mutation replay harness covering normal behavior and
  a Go-only fault matrix covering rollback guarantees.

It does not:

- edit Bash management behavior, make Go authoritative, turn `install.sh` into
  a wrapper, route public `cli.Run` to management, or authorize release cutover;
- claim Bash has operation-wide rollback. Bash remains atomic per destination;
  Go's injected-failure rollback is a Ticket acceptance guard for the shadow
  implementation and does not weaken or rewrite the documented Bash contract;
- restore user files automatically after ambiguous real-world drift. Manual
  recovery cases retain a verified operation backup and exit 65 exactly as Bash;
- alter target registry metadata/rendering, merge Install State with Runtime
  State, select a Runtime Host/Profile, or change Provider discovery.

Normal, externally observable Bash behavior is the parity oracle. The Go-only
fault injector is not exposed through production CLI or environment variables.

## Locked Interfaces and File Map

Keep Ticket 12's `InstallRequest`, `PreparedInstall`, `PrepareInstall`,
`ApplyInstall`, and `Install` source compatible. Add these immutable management
interfaces in `internal/management/management.go`:

```go
type UpdateRequest struct {
    Project string
    Targets string
    DryRun  bool
    Force   bool
}

type UninstallRequest struct {
    Project string
    Targets string
    DryRun  bool
    Force   bool
}

type PreparedUpdate struct { /* private mutationPlan */ }
type PreparedUninstall struct { /* private mutationPlan */ }

func PrepareUpdate(Source, Environment, UpdateRequest) (PreparedUpdate, error)
func ApplyUpdate(PreparedUpdate) (Result, error)
func Update(Source, Environment, UpdateRequest) (Result, error)

func PrepareUninstall(Environment, UninstallRequest) (PreparedUninstall, error)
func ApplyUninstall(PreparedUninstall) (Result, error)
func Uninstall(Environment, UninstallRequest) (Result, error)
```

Private values are closed and defensively copied:

```go
type mutationOperation uint8 // update or uninstall
type mutationEffect uint8    // replace, remove, or retain

type mutationAction struct {
    effect mutationEffect
    label string
    data []byte
    destination string
    mode fs.FileMode
    allowedRoot string
    relativeSuffix string
    before installPathSnapshot
}

type directoryAction struct {
    destination string
    allowedRoot string
    relativeSuffix string
    before installPathSnapshot
    namespace bool
}

type mutationPlan struct {
    operation mutationOperation
    source Source
    environment Environment
    request mutationRequest
    resolved resolvedRequest
    coordinates coordinates
    targetActions []mutationAction
    policyAction mutationAction
    stateActions []mutationAction
    directoryActions []directoryAction
    backup backupPlan
    terminal terminalMutation
    predicted Result
}
```

`terminalMutation` represents only Bash's ambiguous-marker manual-recovery path:
it carries status/message after a prepared backup, so preparation stays write
free and apply can emit `backup`/`would-backup` before returning exit 65.

File ownership is fixed as follows:

- `mutation_values.go`: enums, immutable copies, action validation/deduplication;
- `mutation_prepare.go`: shared state/policy/target preflight and drift decisions;
- `update.go`: update rendering, record merge, cross-scope state rewrites;
- `uninstall.go`: target filtering, block removal, policy retention, directory
  partition/removal;
- `backup.go`: immutable candidates, manifest rendering, private verified apply;
- `mutation_apply.go`: action ordering, failpoint seam, inverse journal, rollback;
- `filesystem.go`: root-scoped atomic replace/remove and empty-directory effects;
- `internal/cli/shadow_install.go`: internal shadow grammar/dispatch only;
- `tests/13-mutation-parity-test.sh`: same-path Bash/Go normal parity.

## Task 1: Generalize immutable mutation actions without changing install

**Files:**
- Create: `internal/management/mutation_values.go`
- Create: `internal/management/mutation_values_test.go`
- Modify: `internal/management/install_prepare.go`
- Modify: `internal/management/install_apply.go`
- Modify: `internal/management/install_values.go`
- Modify: `internal/management/management.go`

- [x] **Step 1: Add failing action-contract and install-regression tests.**

Test construction and defensive copying for replace/remove/retain actions;
reject empty/control-character fields, absolute or traversing suffixes,
destination/root mismatch, invalid mode/effect combinations, duplicate
destinations with conflicting effects, and mutation of caller-owned byte
slices. Assert conversion of Ticket 12 install actions preserves every existing
`PreparedInstall` result and action order.

- [x] **Step 2: Run focused tests and record RED.**

```bash
rtk go test ./internal/management -run 'MutationAction|InstallActionRegression' -count=1
```

Expected: compile failure because mutation action types and constructors do not
exist. No implementation changes precede this RED run.

- [x] **Step 3: Implement the closed action domain and install adapters.**

Add validated constructors and clone helpers. Replace duplicated install-only
validation with shared immutable primitives while retaining `installAction` as
an internal compatibility adapter where it keeps Ticket 12 changes smaller.
`remove` has no source data and no mode; `retain` has no filesystem effect;
`replace` permits only 0600/0644 and copied data. Every effect retains the exact
preparation snapshot for TOCTOU checks and rollback.

- [x] **Step 4: Run install regression, race, and commit.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management ./internal/cli -count=1
rtk go test -race ./internal/management ./internal/cli
rtk bash tests/12-install-parity-test.sh
rtk git add internal/management
rtk git commit -m "refactor: generalize management mutation actions"
```

## Task 2: Prepare clean update and exact cross-scope state effects

**Files:**
- Create: `internal/management/mutation_prepare.go`
- Create: `internal/management/mutation_prepare_test.go`
- Create: `internal/management/update.go`
- Create: `internal/management/update_test.go`
- Modify: `internal/management/management.go`
- Modify: `internal/management/install_prepare.go`

- [x] **Step 1: Write failing zero-write update preparation tests.**

Cover user/project default and selected targets, all target IDs, shared project
`AGENTS.md`, local-checkout version/Policy changes, clean unchanged updates,
state backup-reference preservation, physical project links/spaces, missing
state, selected target not installed, malformed/foreign state, mismatched scope,
project root or policy path, missing policy, policy/target/directory drift,
cross-scope state synchronization, and a later invalid record after a valid
selected target. Snapshot HOME/config/state/project before and after every
`PrepareUpdate` and require byte/mode/tree equality.

- [x] **Step 2: Run update preparation tests and record RED.**

```bash
rtk go test ./internal/management -run 'PrepareUpdate|UpdateStateCoordination' -count=1
```

Expected: missing `UpdateRequest` and `PrepareUpdate` symbols.

- [x] **Step 3: Implement shared mutation preflight.**

Resolve the request and coordinates using Ticket 12 rules. If state is absent,
scan selected managed destinations for untracked markers before returning 66
`no installation state; run install first`. Strictly load current and candidate
states, bind scope/project/policy/state paths, validate owned directories, and
validate every installed record before producing any action. Preserve registry
order and Bash diagnostics/statuses.

- [x] **Step 4: Render update targets and state effects.**

Render selected installed targets from the supplied current checkout, normalize
all records sharing a selected destination to the new checksum, preserve
origins/directories/prior backup reference, replace Policy with mode 0600, and
rewrite current plus every live same-Policy state reference in deterministic
Bash glob order. Other-scope adapters are validated but not rewritten. All
states receive the new version and Policy checksum.

- [x] **Step 5: Predict exact clean and dry-run output.**

Use Bash order: selected target actions, Policy, current state, state references.
Emit `oaw: unchanged: <label>`, `oaw: would-create: <path>`, or
`oaw: would-update: <path>` with no writes. A clean update creates no backup and
retains any existing backup reference.

- [x] **Step 6: Run focused GREEN, coverage, and commit.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management -run 'PrepareUpdate|UpdateStateCoordination|UpdateDryRun' -count=1
rtk go test -race ./internal/management
rtk go test ./internal/management -coverprofile=/tmp/oaw-ticket13-management.cover -count=1
rtk go tool cover -func=/tmp/oaw-ticket13-management.cover
rtk git add internal/management
rtk git commit -m "feat: prepare Go update operations"
```

## Task 3: Prepare uninstall, policy retention, and owned-directory cleanup

**Files:**
- Create: `internal/management/uninstall.go`
- Create: `internal/management/uninstall_test.go`
- Modify: `internal/management/mutation_prepare.go`
- Modify: `internal/management/mutation_values.go`
- Modify: `internal/management/managed.go`
- Modify: `internal/management/filesystem.go`

- [x] **Step 1: Write failing zero-write uninstall preparation tests.**

Cover missing-state idempotency, selected absent targets, partial uninstall,
final uninstall, managed blocks in created/existing files with and without final
newline, owned files, shared destinations, retained cross-scope Policy, state
backup references, non-empty user directories, namespace directory cleanup,
hostile path names, malformed state and all clean drift refusal cases. Snapshot
every root and require no preparation changes.

- [x] **Step 2: Run uninstall preparation tests and record RED.**

```bash
rtk go test ./internal/management -run 'PrepareUninstall|UninstallPlan|PolicyRetention|DirectoryRemoval' -count=1
```

Expected: missing `UninstallRequest` and `PrepareUninstall` symbols.

- [x] **Step 3: Implement target filtering and managed-content removal.**

For installed selected IDs, remove their records and only act on a physical
destination after its last record is removed. For managed blocks, remove exactly
the owned marker span while preserving all foreign bytes/newline behavior; delete
a created file only when its rendered remainder is empty. Remove owned files only
when state origin is `created-file`. Missing selected IDs emit
`oaw: unchanged: <id>` before filesystem action output.

- [x] **Step 4: Implement state/Policy retention and directory plans.**

When targets remain, replace state with retained records/directories/version,
Policy checksum, and prior backup reference. When none remain, remove current
state and remove Policy only when no other strictly validated live state
references it. Partition owned directories against remaining records, sort
removals deepest-first then bytewise, classify target versus OAW namespace
directories, and predict `would-remove-directory` only when planned removals
make the directory empty; otherwise emit `unchanged-directory`.

- [x] **Step 5: Run focused GREEN, existing Bash lifecycle tests, and commit.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management -run 'PrepareUninstall|UninstallPlan|PolicyRetention|DirectoryRemoval' -count=1
rtk go test -race ./internal/management
rtk bash tests/03-claude-lifecycle-test.sh
rtk bash tests/05-project-adapters-test.sh
rtk bash tests/09-transaction-test.sh
rtk git add internal/management
rtk git commit -m "feat: prepare Go uninstall operations"
```

## Task 4: Match forced drift recovery and verified backup manifests

**Files:**
- Create: `internal/management/backup.go`
- Create: `internal/management/backup_test.go`
- Create: `internal/management/force.go`
- Create: `internal/management/force_test.go`
- Modify: `internal/management/mutation_prepare.go`
- Modify: `internal/management/update.go`
- Modify: `internal/management/uninstall.go`
- Modify: `internal/management/managed.go`

- [x] **Step 1: Write failing force-decision and pure-manifest tests.**

Cover policy drift, managed-block body drift, owned-file drift, shared selected
destinations, missing begin/end marker repairs, ambiguous/duplicate/reversed
markers, missing/non-regular target, unselected drift, clean `--force`, and
force dry-run. Assert exact backup candidate order and deduplication, manifest
bytes, operation/scope rows, original/backup/checksum fields, basename numbering,
state backup reference, private modes, and no credential/file-content leakage.

- [x] **Step 2: Run force/backup tests and record RED.**

```bash
rtk go test ./internal/management -run 'Force|BackupPlan|BackupManifest|ManualRecovery' -count=1
```

Expected: force verification and backup plan symbols are undefined.

- [x] **Step 3: Implement Bash-compatible force verification.**

Without `Force`, return exit 65 on any drift and create no backup. With `Force`,
validate selected physical destinations, apply force to every record sharing a
selected path, retain the current managed file in memory, and mark backup
required when checksums differ. Repair only a single missing marker when the
adjacent fragment exactly equals the recorded checkout block. For ambiguous
marker ownership, produce a terminal manual-recovery plan containing target,
state, and Policy candidates, then return exit 65 only after apply completes the
backup (or reports `would-backup` in dry-run).

- [x] **Step 4: Implement immutable backup reservation and rendering.**

Reserve `<StateHome>/open-agent-workflow/backups/<UTC YYYYMMDDTHHMMSSZ>-<pid>`
through an injected private clock/PID seam used only by same-package tests.
Candidate order is Policy when changed, then target actions, then state actions,
deduplicated by original path. Collect regular files whose replace bytes differ
or whose action removes them. Render inert TSV; never source/evaluate values.

- [x] **Step 5: Implement private verified backup apply.**

For real apply, create the operation directory at 0700, artifacts and manifest
at 0600 using root-scoped atomic writes. Revalidate source path identity and
checksum before copy, verify each backup checksum, atomically publish the
manifest only after every artifact completes, revalidate every source again,
then activate the manifest. No mutation action may start until all candidates
are present. Dry-run creates no directory and emits only
`oaw: would-backup: <path>`.

- [x] **Step 6: Run GREEN, Bash backup oracle, race, and commit.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management -run 'Force|BackupPlan|BackupManifest|ManualRecovery' -count=1
rtk go test -race ./internal/management
rtk bash tests/08-backup-test.sh
rtk git add internal/management
rtk git commit -m "feat: add Go forced recovery backups"
```

## Task 5: Apply root-scoped mutations with deterministic rollback

**Files:**
- Create: `internal/management/mutation_apply.go`
- Create: `internal/management/mutation_apply_test.go`
- Create: `internal/management/mutation_fault_test.go`
- Modify: `internal/management/filesystem.go`
- Modify: `internal/management/filesystem_test.go`
- Modify: `internal/management/update.go`
- Modify: `internal/management/uninstall.go`

- [x] **Step 1: Write failing apply-order and fault-matrix tests.**

Define private typed failpoints before and after backup, each target effect,
target-directory removal, Policy effect, each state effect, and namespace
directory removal. For every failpoint, snapshot bytes, modes, symlinks and
directory ownership before apply; assert the returned failure is stable, all
pre-existing non-backup roots equal the snapshot, no temporary files remain,
and a completed forced backup remains valid. Also test rollback failure reporting
without exposing file bytes.

- [x] **Step 2: Run apply/fault tests and record RED.**

```bash
rtk go test ./internal/management -run 'ApplyUpdate|ApplyUninstall|MutationFault|Rollback' -count=1
```

Expected: apply entrypoints and failpoint executor are undefined.

- [x] **Step 3: Add root-scoped replace/remove/directory primitives.**

Use `os.Root` for every destination. Rebuild and revalidate lexical coordinates,
opened-root identity, each component, parent identity, final path type, and the
complete prepared snapshot immediately before effects. `replace` uses a sibling
temporary file, sync, chmod and rename. `remove` rejects symlinks/non-regular
paths and removes only the exact final name. Empty-directory removal reopens and
revalidates the exact directory and never follows links.

- [x] **Step 4: Implement ordered apply and inverse journal.**

Update order is targets, target directories, Policy, states. Uninstall order is
targets, non-namespace directories, optional Policy remove, state, namespace
directories. Before the first mutation, ensure forced backup completion and
revalidate its manifest against every changed pre-existing destination. After
each successful effect append a copied inverse snapshot. On an injected or real
later apply error, replay inverses in reverse using containment primitives,
restore original bytes/modes, remove files created by the operation, and remove
only operation-created empty directories. Return the original error when
rollback succeeds; return a stable combined error when rollback fails.

- [x] **Step 5: Implement public wrappers and dry-run no-write apply.**

`ApplyUpdate`/`ApplyUninstall` clone and validate their private plans, revalidate
all snapshots before any write, and return copied result lines. `Update` and
`Uninstall` compose prepare/apply. Dry-run performs complete revalidation and
returns predicted output without backup or filesystem creation. The test-only
executor accepts failpoints by direct Go value; no env/CLI failpoint exists.

- [x] **Step 6: Run GREEN, containment regression, coverage, and commit.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management -run 'ApplyUpdate|ApplyUninstall|MutationFault|Rollback|ScopedMutation' -count=1
rtk go test -race ./internal/management
rtk bash tests/07-containment-test.sh
rtk bash tests/09-transaction-test.sh
rtk go test ./internal/management -coverprofile=/tmp/oaw-ticket13-management.cover -count=1
rtk go tool cover -func=/tmp/oaw-ticket13-management.cover
rtk git add internal/management
rtk git commit -m "feat: apply Go mutations with rollback"
```

Expected: `internal/management` statement coverage remains at least 90 percent.

## Task 6: Expose only the internal shadow grammar and prove Bash/Go parity

**Files:**
- Modify: `internal/cli/shadow_install.go`
- Modify: `internal/cli/shadow_install_test.go`
- Rename: `internal/cmd/oaw-install-shadow` to `internal/cmd/oaw-management-shadow`
- Create: `tests/13-mutation-parity-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Add failing shadow parser and public-authority tests.**

Require `install`, `update`, and `uninstall` to accept the exact shared Bash
options, duplicate/missing-value errors, help bytes, stdout/stderr routing, and
management statuses. Assert `cli.Run` still rejects all three management verbs
without changing HOME/config/state/project.

- [ ] **Step 2: Run CLI tests and record RED.**

```bash
rtk go test ./internal/cli -run 'ShadowManagement|PublicRunDoesNotRouteManagement' -count=1
```

Expected: update/uninstall are rejected by the install-only parser.

- [ ] **Step 3: Generalize the internal driver and keep public routing closed.**

Rename parser/command values to management-neutral names, dispatch the three
private verbs to `management.Install`, `management.Update`, or
`management.Uninstall`, and preserve exact `oaw: error: ...` handling. Keep the
internal command under `internal/cmd`; do not add it to release packaging or the
public command switch.

- [ ] **Step 4: Add same-physical-path mutation parity replay.**

Build the internal shadow binary once. For each case, create an identical Bash
install fixture, save it, run Bash mutation, restore the byte/mode/symlink tree,
run Go mutation, then compare normalized status, stdout, stderr, file types,
modes, symlinks, regular-file bytes, state rows, directory ownership and backup
tree. Normalize only operation backup timestamp/PID path segments while
requiring all manifest references to be internally consistent.

The matrix includes every target/scope, clean update, checkout Policy/version
change, partial/final/shared/cross-scope uninstall, missing-state behavior,
dry-run, invalid state/scope/project/policy/target/directory drift, clean force,
recoverable/ambiguous markers, forced target/policy drift, manual recovery,
backup source races, hostile names, and later preflight failures.

- [ ] **Step 5: Run exact parity and commit.**

```bash
rtk gofmt -w internal/cli internal/cmd/oaw-management-shadow
rtk go test ./internal/cli ./internal/management -count=1
rtk bash tests/12-install-parity-test.sh
rtk bash tests/13-mutation-parity-test.sh
rtk git add internal/cli internal/cmd tests
rtk git commit -m "test: prove Go mutation parity"
```

## Task 7: Complete security containment, fuzzing, and output-leakage proof

**Files:**
- Create: `internal/management/mutation_security_test.go`
- Create: `internal/management/mutation_fuzz_test.go`
- Modify: `internal/management/paths_test.go`
- Modify: `internal/management/filesystem_test.go`
- Modify: `tests/13-mutation-parity-test.sh`

- [ ] **Step 1: Add hostile boundary and leakage tests.**

Cover relative/control-character HOME/XDG roots, unsafe suffix components,
project control characters/nonexistence/file/symlink/physical alias and root
mismatch, target/policy/state/backup/directory symlink redirection, swapped final
paths and parents between prepare/apply/backup, non-regular files, oversized
artifacts, malicious TSV fields, duplicate paths/actions/candidates, and
same-path backup races. Put credential-shaped sentinels in file bodies and
injected I/O errors; assert they never appear in Result, Error, stdout, stderr,
manifest metadata beyond exact allowed paths, or test snapshots intended as
diagnostics.

- [ ] **Step 2: Add fuzz invariants.**

Fuzz managed block removal/repair, mutation action constructors, backup manifest
rendering, rollback journal order, path suffix validation, and hostile state
fields. Require no panic, no path escape, deterministic output, copied input
ownership, valid UTF-8 independence, and round-trip preservation of all bytes
outside an exactly owned block.

- [ ] **Step 3: Run security, fuzz, race, and static gates.**

```bash
rtk gofmt -w internal/management
rtk go test ./internal/management -run 'Security|Containment|Credential|Path|Symlink|ProjectRoot' -count=1
rtk go test -race ./internal/management ./internal/cli
rtk go test ./internal/management -run '^$' -fuzz 'FuzzMutation|FuzzManagedRemoval|FuzzBackupManifest' -fuzztime=10s
rtk bash tests/06-security-test.sh
rtk bash tests/07-containment-test.sh
rtk go vet ./...
rtk govulncheck ./...
rtk git diff --check
```

- [ ] **Step 4: Commit security evidence tests.**

```bash
rtk git add internal/management tests/13-mutation-parity-test.sh
rtk git commit -m "test: harden Go management transactions"
```

## Task 8: Document the shadow boundary, review, verify, and close Ticket 13

**Files:**
- Modify: `docs/en/installer.md`
- Modify: `docs/zh/installer.md`
- Modify: `tests/10-docs-test.sh`
- Modify: `.scratch/oaw-runtime-vnext/issues/13-go-update-uninstall-and-security-transaction-parity.md`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`
- Modify: this plan

- [ ] **Step 1: Add failing bilingual boundary assertions.**

Require both installer guides to state that internal Go update/uninstall are
parity-only, Bash `install.sh` remains authoritative, public Go management is
not routed, normal behavior matches Bash, forced mutations finish verified
backup before writes, injected Go failures restore pre-existing content, and
Ticket 14 alone owns cutover. Preserve the existing truthful statement that
Bash does not promise operation-wide rollback.

- [ ] **Step 2: Document and run docs tests.**

```bash
rtk bash tests/10-docs-test.sh
rtk git add docs tests/10-docs-test.sh
rtk git commit -m "docs: define Go mutation parity boundary"
```

- [ ] **Step 3: Perform an inline correctness/security/spec review.**

Review the complete Ticket branch against the issue, this plan, Runtime vNext
migration sections 17-18, and the Bash oracle. Check authority leakage, mutable
plans, writeful preparation/dry-run, drift/status/diagnostic mismatch,
manual-recovery ordering, backup incompleteness, rollback gaps, user-byte loss,
shared-destination errors, cross-scope state corruption, directory over-removal,
symlink/TOCTOU escape, unbounded reads, temp leaks, credential leakage, output
order, platform assumptions, and Ticket 14 scope creep. Fix every Critical,
High, Important, Standards, or Spec finding and rerun focused tests.

- [ ] **Step 4: Run the complete verification matrix.**

```bash
rtk gofmt -w .
rtk git diff --check
rtk go test ./... -count=1
rtk go test -race ./...
rtk go vet ./...
rtk go test ./... -coverprofile=/tmp/oaw-ticket13-all.cover -count=1
rtk go tool cover -func=/tmp/oaw-ticket13-all.cover
rtk go test ./internal/management -coverprofile=/tmp/oaw-ticket13-management.cover -count=1
rtk go tool cover -func=/tmp/oaw-ticket13-management.cover
rtk bash tests/run.sh
rtk bash tests/12-install-parity-test.sh
rtk bash tests/13-mutation-parity-test.sh
rtk go test ./internal/management -run '^$' -fuzz 'FuzzMutation|FuzzManagedRemoval|FuzzBackupManifest' -fuzztime=10s
rtk env GOOS=linux GOARCH=amd64 go build ./cmd/oaw
rtk env GOOS=windows GOARCH=amd64 go build ./cmd/oaw
rtk shellcheck install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
rtk govulncheck ./...
```

Required: all tests/race/vet/security/platform checks pass; repository Go
statement coverage remains at least 80 percent; `internal/management` remains at
least 90 percent; all Bash management tests remain green; public management is
still not routed.

- [ ] **Step 5: Close the ticket and evidence at the fixed point.**

Mark all five Ticket 13 acceptance items complete; set issue status to
`completed`; append exact test counts, coverage, parity-case counts, fuzz,
platform, ShellCheck and vulnerability results; record the inline review with
zero unresolved required findings; update workflow dependency status; check all
plan boxes; commit closure.

```bash
rtk git add .scratch docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-13-go-update-uninstall-security-transaction-parity.md
rtk git commit -m "docs: close runtime vnext ticket 13"
```

- [ ] **Step 6: Verify branch completion and fast-forward merge.**

From the Ticket worktree, record the clean fixed point and rerun the focused
parity gate. From the primary worktree, require `main` at the Ticket 12 fixed
point, then fast-forward only:

```bash
rtk git status --short --branch
rtk git log -1 --oneline
rtk bash tests/13-mutation-parity-test.sh
rtk git -C /Users/wifibaby4u/LLM/open-agent-workflow merge --ff-only feat/oaw-runtime-vnext-ticket-13
rtk git -C /Users/wifibaby4u/LLM/open-agent-workflow status --short --branch
```

Preserve the Ticket branch/worktree and every unrelated user artifact.

## Plan Self-Review

- Acceptance item 1 maps to Tasks 2-6: exact update/uninstall rendering, managed
  and owned destinations, state/Policy coordination, backup references, dry-run,
  output/status, and same-path Bash/Go replay.
- Acceptance item 2 maps to Task 4 force matrices and Task 6 parity fixtures for
  clean refusal, recoverable repairs, manual recovery, shared paths, status 65,
  and exact diagnostics.
- Acceptance item 3 maps to Task 5's typed failpoint at every effect boundary,
  reverse inverse journal, full tree byte/mode snapshots, temp cleanup, and
  retained verified forced backups.
- Acceptance item 4 maps to Task 7 path/project/state/backup containment,
  symlink and TOCTOU races, control fields, bounded reads, fuzz invariants, and
  credential-shaped output assertions.
- Acceptance item 5 is enforced by zero-write preparation/dry-run, full preflight,
  Bash same-path replay, inverse rollback, unchanged Bash/public authority, and
  Ticket 14 exclusion.
- Types remain consistent across all tasks: install stays source compatible;
  update/uninstall wrap one immutable private mutation plan; force/manual backup
  is a plan state rather than preparation side effect; only apply writes.
- Placeholder review found no deferred implementation marker, optional behavior,
  or unspecified ownership. Clock/PID and fault injection are private test seams;
  no production environment or CLI control is introduced.
