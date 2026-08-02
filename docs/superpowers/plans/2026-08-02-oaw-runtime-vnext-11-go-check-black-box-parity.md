# OAW Runtime vNext Go Check Black-box Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only `oaw check` shadow implementation in Go whose command output, streams, exit status, scope and target resolution, Provider diagnostics, installation-health diagnostics, and filesystem effects match the authoritative Bash `install.sh check` command.

**Architecture:** Introduce a self-contained `internal/check` compatibility domain that reads an injected environment, derives physical user/project coordinates, evaluates the legacy target and Provider contracts, parses existing TSV Install State without repairing it, and returns deterministic report lines or Bash-compatible errors. Wire that domain into the Go CLI as `oaw check`, while retaining Bash as the only authoritative management surface. A command-level harness will build the Go binary, run Bash and Go sequentially against the same isolated fixture, compare stdout/stderr/status byte-for-byte, and prove the fixture snapshot is unchanged after each implementation.

**Tech Stack:** Go 1.26 standard library, embedded repository version, POSIX `cksum` compatibility, existing Bash 3.2 installer, table-driven unit tests, shell black-box fixtures, race/coverage/fuzz/platform gates.

---

## Scope Boundary

Ticket 11 owns:

- a Go `oaw check` command with the Bash `check` option grammar;
- exact legacy user/project target defaults, registry ordering, deduplication,
  physical project-root resolution, readiness, and destination paths;
- the current three compatibility diagnostics (`superpowers`, `matt`, `ecc`),
  using the built-in Provider descriptors as probe sources but retaining the
  Bash compatibility match rules (Superpowers any supported indicator, Matt
  the complete four-skill bundle, ECC the cross-client global skill);
- read-only parsing and health evaluation of the existing TSV Install State,
  policy checksums, managed blocks, owned files, and owned-directory records;
- a same-fixture Bash/Go command-level parity harness covering empty, detected,
  clean, drifted, invalid, user, project, explicit-target, and invalid-argument
  cases;
- English/Chinese documentation that calls the Go command non-authoritative
  shadow/parity behavior.

It does not:

- change `install.sh`, `lib/commands/check.sh`, or any Bash management result;
- make Go authoritative, turn `install.sh` into a wrapper, or implement Go
  `install`, `update`, or `uninstall`;
- reuse Runtime State as Install State, repair malformed state, mutate a
  fixture, select a lifecycle Profile, or select a Runtime Host;
- broaden the compatibility output to arbitrary configured Providers; dynamic
  catalog/runtime discovery remains available through the typed Go catalog and
  is intentionally distinct from the frozen Bash management report.

## Locked Interfaces and File Map

The implementation uses these seams:

```go
// version.go
func Version() string

// internal/check/check.go
type Environment struct {
    Home, ConfigHome, StateHome, Path, TempDir string
}

type Request struct {
    Project string
    Targets string
}

type Result struct {
    Lines []string
}

type Error struct {
    Status  int
    Message string
}

func Execute(value catalog.Catalog, environment Environment, request Request) (Result, error)
func Write(result Result, output io.Writer) error
```

`internal/check/targets.go` owns the frozen target registry and path/scope
resolution. `internal/check/providers.go` owns compatibility Provider matching
from descriptor probe IDs. `internal/check/state.go` owns strict TSV decoding
and installation health. `internal/check/checksum.go` owns POSIX checksum parity.
`internal/cli/check.go` owns only Bash-compatible argument parsing and rendering;
it does not contain filesystem or health rules.

## Task 1: Lock the Go check command and target contract

**Files:**
- Create: `version.go`
- Create: `version_test.go`
- Create: `internal/check/check.go`
- Create: `internal/check/targets.go`
- Create: `internal/check/check_test.go`
- Create: `internal/check/targets_test.go`
- Create: `internal/cli/check.go`
- Create: `internal/cli/check_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/catalog_test.go`

- [ ] **Step 1: Write failing version, parser, scope, and target tests.**

  Assert the embedded version equals trimmed `VERSION`; `oaw check` accepts only
  one `--target` and one `--project`, rejects `--dry-run`, `--force`, unknown
  options, unexpected operands, missing/empty values, whitespace, empty target
  members, unknown targets, and extension targets at user scope with the exact
  Bash message/status 64. Assert defaults, duplicate normalization, registry
  order, paths containing spaces, project symlink resolution, nonexistent/file/
  control-character project paths, and command-scoped help. Invalid arguments
  must fail before catalog loading or filesystem inspection.

- [ ] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./ ./internal/check ./internal/cli -run 'Version|Check|Scope|Target'
  ```

  Expected: the root version package, `internal/check`, and Go `check` command do
  not yet exist.

- [ ] **Step 3: Embed the release version and implement immutable target resolution.**

  Use root `//go:embed VERSION`, return a trimmed non-empty version, and define
  the target registry as copied values rather than caller-mutable global slices:

  ```go
  type target struct {
      ID, UserSuffix, ProjectSuffix, Ownership string
      User bool
  }

  var targetRegistry = [...]target{
      {ID: "claude", UserSuffix: ".claude/CLAUDE.md", ProjectSuffix: ".claude/CLAUDE.md", Ownership: "managed-block", User: true},
      {ID: "codex", UserSuffix: ".codex/AGENTS.md", ProjectSuffix: "AGENTS.md", Ownership: "managed-block", User: true},
      {ID: "gemini", UserSuffix: ".gemini/GEMINI.md", ProjectSuffix: "GEMINI.md", Ownership: "managed-block", User: true},
      {ID: "opencode", UserSuffix: "opencode/AGENTS.md", ProjectSuffix: "AGENTS.md", Ownership: "managed-block", User: true},
      {ID: "cursor", ProjectSuffix: ".cursor/rules/open-agent-workflow.mdc", Ownership: "owned-file"},
      {ID: "windsurf", ProjectSuffix: ".devin/rules/open-agent-workflow.md", Ownership: "owned-file"},
      {ID: "cline", ProjectSuffix: ".clinerules/open-agent-workflow.md", Ownership: "owned-file"},
      {ID: "roo", ProjectSuffix: ".roo/rules/open-agent-workflow.md", Ownership: "owned-file"},
      {ID: "copilot", ProjectSuffix: ".github/instructions/open-agent-workflow.instructions.md", Ownership: "owned-file"},
  }
  ```

  Resolve a project with `filepath.Abs`, `filepath.EvalSymlinks`, and `os.Stat`;
  reject controls and non-directories; keep user defaults to four core targets
  and project defaults to all nine.

- [ ] **Step 4: Add the CLI compatibility parser and renderer.**

  Route only first argument `check` to the new parser. Preserve the existing
  catalog parser and catalog error vocabulary. Construct `check.Environment`
  from the process environment, load only the built-in catalog after parsing,
  call `check.Execute`, render `*check.Error` as `oaw: error: <message>`, and use
  the exact Bash global usage for `oaw check --help`. Do not append Go catalog
  usage to check failures.

- [ ] **Step 5: Run focused GREEN tests and commit.**

  ```bash
  rtk gofmt -w version.go version_test.go internal/check internal/cli
  rtk go test ./ ./internal/check ./internal/cli
  rtk git add version.go version_test.go internal/check internal/cli
  rtk git commit -m "feat: define Go check compatibility command"
  ```

## Task 2: Match Provider and target-readiness diagnostics

**Files:**
- Modify: `internal/assets/providers/oaw-matt.json`
- Modify: `internal/assets/embed_test.go`
- Create: `internal/check/providers.go`
- Create: `internal/check/providers_test.go`
- Create: `internal/check/readiness.go`
- Create: `internal/check/readiness_test.go`
- Modify: `internal/check/check.go`
- Modify: `internal/check/check_test.go`

- [ ] **Step 1: Write failing Provider and readiness tests.**

  Cover every Superpowers direct/versioned/marketplace indicator as OR probes,
  incomplete and complete Matt bundles as four-probe AND, ECC global skill as
  the sole compatibility indicator, unrelated Provider evidence, regular files,
  symlinked indicator files, missing roots, core-tool executable lookup, core
  configuration directories, and all five adapter-only project targets. Assert
  stable output order: version, scope, targets, three Providers, selected target
  readiness, selected installation health.

- [ ] **Step 2: Run the focused tests and verify RED.**

  ```bash
  rtk go test ./internal/assets ./internal/check -run 'Provider|Readiness|Report'
  ```

- [ ] **Step 3: Complete Matt descriptor evidence and implement compatibility matching.**

  Add `matt-tdd` and `matt-diagnosing-bugs` probes to the built-in descriptor so
  the descriptor expresses every path consumed by compatibility diagnostics.
  Resolve probes by stable ID; fail closed if a required built-in ID disappears.
  Superpowers succeeds when any of its six current descriptor probes is a
  regular file, Matt only when all four are regular files, and ECC only when
  `ecc-global-skill` is a regular file. The compatibility matcher must not infer
  lifecycle selection or enumerate user-configured Providers.

- [ ] **Step 4: Implement readiness and deterministic report assembly.**

  Mirror `command -v` with `exec.LookPath` under the injected PATH. For core
  targets accept either the executable or the exact config directory; render
  `detected|missing (user, project)`. Render extension targets as
  `adapter-only (project)` without probing commands or creating directories.

- [ ] **Step 5: Run focused GREEN, descriptor regression tests, and commit.**

  ```bash
  rtk gofmt -w internal/check
  rtk go test ./internal/assets ./internal/builtin ./internal/discovery ./internal/check ./internal/cli
  rtk git add internal/assets/providers/oaw-matt.json internal/assets/embed_test.go internal/check
  rtk git commit -m "feat: mirror check readiness diagnostics"
  ```

## Task 3: Parse legacy Install State and evaluate health without writes

**Files:**
- Create: `internal/check/checksum.go`
- Create: `internal/check/checksum_test.go`
- Create: `internal/check/managed.go`
- Create: `internal/check/managed_test.go`
- Create: `internal/check/state.go`
- Create: `internal/check/state_test.go`
- Create: `internal/check/health.go`
- Create: `internal/check/health_test.go`
- Modify: `internal/check/check.go`

- [ ] **Step 1: Write failing checksum, marker, state, and health tests.**

  Compare Go checksums against fixed POSIX `cksum` vectors, including empty,
  text, binary, and project-root bytes without a newline. Cover absent/present/
  duplicate/reversed/partial managed markers and extraction normalization.
  Table-test valid user/project state plus every structural invariant consumed
  by Bash: exact record arity/counts, scope/project binding, absolute/safe policy
  and target paths, numeric checksums, known unique registry-ordered targets,
  ownership/origin values, shared-destination consistency, unique absolute owned
  directories, and owned-directory-to-target/namespace containment.

  Health tests must cover no state, tracked/untracked targets, clean policy and
  targets, missing or changed policy, changed managed block, changed owned file,
  target destination mismatch, invalid state, policy/target symlinks, target
  path component symlinks, shared Codex/OpenCode project destinations, and state
  scope/root mismatch. Diagnostic drift and invalid state still exit zero;
  unsafe path resolution returns the same Bash status/message.

- [ ] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/check -run 'Checksum|Managed|State|Health'
  ```

- [ ] **Step 3: Implement POSIX checksum and managed-block readers.**

  Implement the POSIX CRC algorithm locally—do not invoke `cksum` or write a
  temporary file. Complement the CRC after folding the input length and render
  `<unsigned-decimal>:<byte-count>`. Read bytes only, classify markers by exact
  line equality, and checksum an extracted managed block with one newline after
  every extracted line to match Bash `awk` output.

- [ ] **Step 4: Implement strict, bounded read-only TSV state parsing.**

  Parse records into immutable values and validate before exposing metadata.
  Never repair, rewrite, chmod, mkdir, remove, or rename anything. Preserve Bash
  health behavior by collapsing every state decode/owned-directory verification
  failure to `invalid-state`, while path initialization and unsafe destination
  failures remain command errors. Derive project state identity with the same
  POSIX checksum over physical project-root bytes.

- [ ] **Step 5: Implement installation-health evaluation.**

  Use these exact terminal values:

  ```text
  clean
  drift
  invalid-state
  not-installed
  ```

  A valid state with policy drift makes tracked targets drift. An untracked
  managed block or owned file is drift. A selected target absent from valid
  state falls back to untracked health. A malformed state makes every selected
  installed line `invalid-state` without changing exit zero.

- [ ] **Step 6: Run focused GREEN, race, coverage, and commit.**

  ```bash
  rtk gofmt -w internal/check
  rtk go test -race ./internal/check ./internal/cli
  rtk go test ./internal/check -coverprofile=/tmp/oaw-check-ticket11.cover -count=1
  rtk go tool cover -func=/tmp/oaw-check-ticket11.cover
  rtk git add internal/check internal/cli
  rtk git commit -m "feat: evaluate legacy install health in Go"
  ```

  Expected: at least 90% statement coverage for `internal/check`.

## Task 4: Prove same-fixture Bash/Go black-box parity

**Files:**
- Modify: `tests/test-helper.sh`
- Create: `tests/11-check-parity-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Add a failing command-level parity harness.**

  Build `cmd/oaw` once into a private temporary directory. Add helpers that:

  ```sh
  snapshot_fixture >"$snapshot_file"
  run_check_implementation bash "$@"
  run_check_implementation go "$@"
  assert_same_streams_and_status
  assert_fixture_matches_snapshot
  ```

  Capture stdout and stderr separately outside HOME/XDG/project, preserve each
  status under `set +e`, and compare both implementations byte-for-byte. The
  snapshot must include type, relative path, mode, symlink target, and content
  checksum for HOME, XDG config/state, and project. Run Bash first and snapshot
  again before Go so either implementation's mutation is attributed correctly.

- [ ] **Step 2: Add the complete fixture matrix and verify RED.**

  Include empty user defaults; explicit deduplicated targets; physical project
  path with spaces; all project adapters; each Provider missing/detected and
  partial Matt; executable/config-root readiness; clean user managed block;
  clean project owned file; policy drift; managed-block drift; owned-file drift;
  invalid state; valid state without a selected target; untracked managed block;
  untracked owned file; shared destination; scope/root mismatch; nonexistent
  project; target selection errors; duplicate/missing options; forbidden
  `--dry-run`/`--force`; unknown option; unexpected argument; and check help.

  ```bash
  rtk bash tests/11-check-parity-test.sh
  ```

  Expected: the harness reports the first exact stream/status or mutation
  difference and exits nonzero before promotion.

- [ ] **Step 3: Complete parity remediation without editing Bash behavior.**

  Fix only Go compatibility code or the parity harness. Treat any requested Bash
  behavior change as a new migration decision outside Ticket 11. Add a focused
  Go regression test for every mismatch found by the black-box harness.

- [ ] **Step 4: Run the Bash suite and commit.**

  ```bash
  rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk bash tests/run.sh
  rtk git diff --exit-code main -- install.sh lib
  rtk git add tests/test-helper.sh tests/11-check-parity-test.sh tests/run.sh internal/check internal/cli
  rtk git commit -m "test: enforce Bash Go check parity"
  ```

  Expected: all legacy Bash cases and the new parity matrix pass; the diff check
  proves the authoritative Bash implementation is unchanged.

## Task 5: Document shadow status and close Ticket 11

**Files:**
- Modify: `docs/en/installer.md`
- Modify: `docs/zh/installer.md`
- Modify: `tests/10-docs-test.sh`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/issues/11-go-check-black-box-parity.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/verification.md`
- Modify: `docs/superpowers/plans/2026-08-02-oaw-runtime-vnext-11-go-check-black-box-parity.md`

- [ ] **Step 1: Add failing bilingual documentation assertions.**

  Require both installer guides to name `oaw check`, shadow/parity mode, Bash as
  authoritative for all management mutations, the no-cutover guarantee, and the
  same-fixture parity gate. Assert neither guide claims Go `install`, `update`,
  or `uninstall` is authoritative.

- [ ] **Step 2: Document the migration boundary and run docs tests.**

  Explain that `oaw check` is available for conformance and diagnostics only;
  users continue to use `install.sh` for management, and a later explicit ticket
  is required for cutover. Keep English and Chinese meaning aligned.

  ```bash
  rtk bash tests/10-docs-test.sh
  rtk bash tests/run.sh
  ```

- [ ] **Step 3: Perform inline Superpowers review and remediation.**

  Review `main...HEAD` for Bash behavior drift, accidental writes, TOCTOU path
  escapes, symlink following differences, unbounded reads, malformed TSV
  acceptance, checksum incompatibility, command/stream/status mismatch,
  environment leakage, Provider over-detection, mutable returned state, shared
  destination errors, platform-specific path assumptions, and premature Go
  authority. Fix every Critical/High/Important issue and rerun focused parity.

- [ ] **Step 4: Run the complete verification matrix.**

  ```bash
  rtk gofmt -w .
  rtk git diff --check
  rtk go test ./... -count=1
  rtk go test -race ./...
  rtk go vet ./...
  rtk go test ./internal/check -coverprofile=/tmp/oaw-check-ticket11.cover -count=1
  rtk go tool cover -func=/tmp/oaw-check-ticket11.cover
  rtk go test ./... -coverprofile=/tmp/oaw-ticket11-all.cover -count=1
  rtk go tool cover -func=/tmp/oaw-ticket11-all.cover
  rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk bash tests/run.sh
  rtk go test ./internal/classification -run '^$' -fuzz FuzzDecodeProposalFailsClosed -fuzztime 2s
  rtk go test ./internal/host -run '^$' -fuzz FuzzConformanceReceiptFailsClosed -fuzztime 2s
  rtk env GOOS=linux GOARCH=amd64 go build -o /tmp/oaw-ticket11-linux ./cmd/oaw
  rtk env GOOS=windows GOARCH=amd64 go build -o /tmp/oaw-ticket11-windows.exe ./cmd/oaw
  rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...
  rtk git diff --exit-code main -- install.sh lib
  ```

  Expected: all checks pass; `internal/check` coverage is at least 90%, total Go
  coverage stays above 80%, and there are no reachable vulnerabilities.

- [ ] **Step 5: Close artifacts, commit, and fast-forward `main`.**

  Mark Ticket 11 complete, check every acceptance item, set tracker stage and
  active ticket, record exact commits/results in review and verification
  evidence, and preserve `.serena/` plus all unrelated worktrees. Commit closure:

  ```bash
  rtk git add docs .scratch/oaw-runtime-vnext
  rtk git commit -m "docs: close Go check parity ticket"
  ```

  After a fresh final `rtk go test ./... -count=1`, fast-forward `main` to the
  Ticket 11 branch without deleting the branch or worktree.

## Plan Self-Review

- Ticket acceptance item 1 is covered by Task 4's `main -- install.sh lib` diff
  gate and the full legacy Bash suite.
- Acceptance item 2 is covered by Tasks 1-4: stdout, stderr, status, target and
  scope resolution, Provider diagnostics, health, and read-only snapshots.
- Acceptance item 3 is covered by explicit drift, scope, target, and Provider
  fixtures whose mismatch makes the parity script fail.
- Acceptance item 4 is covered by the sequential same-fixture harness with a
  snapshot before/after each implementation.
- Acceptance item 5 is covered by bilingual docs and negative authority tests.
- The plan keeps Bash authoritative, Install State separate from Runtime State,
  and Ticket 09/10/14 Host work blocked and untouched.
- Placeholder scan found no deferred implementation steps; identifiers and
  file ownership are consistent across all tasks.
