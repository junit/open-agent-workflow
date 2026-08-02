# OAW Runtime vNext First Codex Runtime Host and `oaw run`

> For inline execution: use Matt `tdd` for every implementation slice and
> Superpowers `executing-plans` for this plan. Do not dispatch subagents.

**Goal:** Promote only the explicitly selected Codex CLI to a digest-pinned,
`runner-managed` Host Integration, expose `oaw run` and the neutral Runtime
Protocol transport through one shared `runtime.Engine.Exchange` seam, and keep
all other Hosts instruction-only without overstating Runtime guarantees.

**Architecture:** Keep the Runtime Core authoritative. Add a strict, bounded
JSON transport around `runtime.RunFrame`/`RunReply`; `oaw runtime exchange`
performs exactly one protocol exchange, while `oaw run --host codex` uses the
same exchange seam and drives the ordered `GRANT_ISSUED -> DISPATCH_PREPARED ->
DISPATCH_AUTHORIZED -> CAPABILITY_OBSERVED` handshake through a built-in Codex
runner. The runner invokes an exact `codex exec` process binding with bounded
stdout/stderr, `--json`, `--ephemeral`, and an explicit sandbox mode. Provider
output is never persisted as raw Runtime state: only closed outcomes and
digest-pinned evidence references cross the Runtime boundary.

**Selected Host:** Codex CLI, local evidence `codex-cli 0.146.0`, integration
ID `oaw/codex-runner`, level `runner-managed`, selection source
`user-explicit`, selected 2026-08-03. The final integration record is admitted
only after official audit references, a fixed Manifest, and a passing
conformance report are pinned. Claude, Gemini, OpenCode, Cursor, Windsurf,
Cline, Roo, and Copilot remain `instruction-only` in the built-in catalog.

## Scope Boundary

This ticket owns:

- the selected Host migration record and exact Codex integration asset;
- strict single-frame JSON decoding/encoding for the existing Runtime Protocol;
- the `oaw runtime exchange` machine entrypoint;
- the `oaw run --host codex` runner-managed loop and Host Driver seam;
- bounded Codex process execution, cancellation/pause bookkeeping, JSONL
  normalization, and evidence digesting;
- exact Host capability checks for runtime-aware entrypoints;
- process-level fixture tests, conformance asset verification, and migration
  evidence.

It does not:

- change Runtime state-machine semantics, Profile ownership, or TDD ownership;
- invoke a real paid/model-backed Codex run in tests or during verification;
- promote any non-Codex Host or add native-managed integrations;
- load executable third-party Provider or Host plugins;
- make Go installation management authoritative (Ticket 14 owns cutover);
- persist Codex raw output, prompts, credentials, or stderr in Runtime State.

## Locked Interfaces

The implementation must preserve these public seams:

```go
func (engine *runtime.Engine) Exchange(frame runtime.RunFrame) (runtime.RunReply, error)
func runtime.DecodeFrame(raw []byte) (runtime.RunFrame, error)
func runtime.EncodeReply(reply runtime.RunReply) ([]byte, error)
func runtime.ExchangeJSON(ctx context.Context, in io.Reader, out io.Writer, engine *runtime.Engine) error
func cli.RunWithInput(args []string, in io.Reader, stdout, stderr io.Writer) int
```

The transport accepts exactly one bounded UTF-8 JSON frame per invocation,
rejects unknown fields and trailing values, validates the Runtime schema and
canonicalizes replies before writing. Machine stdout contains only one
canonical JSON value; all diagnostics, process progress, and failure detail go
to stderr. `oaw runtime exchange` never invokes a Host.

The Codex driver uses a Host-neutral dispatch seam so the CLI does not duplicate
Runtime transitions:

```go
type Driver interface {
    Prepare(context.Context, DispatchRequest) error
    Invoke(context.Context, DispatchRequest) (DispatchResult, error)
    Cancel(context.Context, string) error
}
```

`DispatchRequest` contains only the committed Grant identity, Invocation ID,
Executor ID, Bundle digest, and exact `catalog.HostBinding`. `DispatchResult`
contains a closed `SUCCEEDED`/`FAILED` outcome and sorted evidence references;
it never carries raw Host output.

## Task 1: Freeze selection evidence and Host asset contracts

**Files:**
- Modify: `.scratch/oaw-runtime-vnext/evidence/host-selection.md`
- Modify: `.scratch/oaw-runtime-vnext/issues/09-first-runtime-host-and-oaw-run.md`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `internal/assets/host-integrations.json`
- Modify/Create: `internal/host/codex_manifest.go`, `internal/host/codex_manifest_test.go`
- Modify: `internal/config/snapshot_test.go`, `internal/integration/host_configuration_test.go`

- [ ] **Step 1: Write failing asset tests.**

  Assert the built-in Codex record has ID `oaw/codex-runner`, Host ID `codex`,
  `runner-managed` level, exactly `oaw.runtime/v1`, binding kinds `agent`,
  `skill`, and `tool`, all required non-native Runtime Features, passed audit
  evidence linked to the official Codex sources, and a passing conformance
  report whose Manifest/Integration identities match. Assert every other
  built-in record remains `instruction-only` with no Runtime claims. Assert
  equivalent asset decoding is deterministic and digest-pinned.

- [ ] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/host ./internal/config ./internal/integration -run 'Codex|Builtin.*Host|HostIntegration'
  ```

- [ ] **Step 3: Add the deterministic Codex Manifest and audit references.**

  Use the selected integration ID and the official URLs already recorded in
  `host-selection.md`. Generate the authored Manifest, Audit, Conformance, and
  Integration digests with the existing canonical JSON constructors; never
  repair a wrong digest while decoding. Keep the asset's other eight records
  byte-for-byte instruction-only.

- [ ] **Step 4: Verify Configuration trust and Host admission.**

  Load the built-in snapshot and prove only `codex` can satisfy Runtime
  admission. Prove an instruction-only Host, a stale Codex digest, a missing
  Feature, or an unavailable exact Binding remains denied without a journal
  revision. Keep project configuration unable to promote a Host.

- [ ] **Step 5: Commit.**

  ```bash
  rtk git add .scratch/oaw-runtime-vnext internal/assets/host-integrations.json internal/host internal/config internal/integration
  rtk git commit -m "feat: select codex as first runtime host"
  ```

## Task 2: Add strict Runtime Protocol transport

**Files:**
- Create: `internal/runtime/transport.go`
- Create: `internal/runtime/transport_test.go`
- Create/Modify: `internal/assets/schemas/v1/runtime-frame.schema.json`, `internal/assets/schemas/v1/runtime-reply.schema.json`, `internal/assets/embed.go`, `internal/schema/registry.go`, and their tests
- Modify: `internal/cli/run.go`, `internal/cli/run_test.go`, `cmd/oaw/main.go`

- [ ] **Step 1: Write failing transport seam tests.**

  Test canonical decoding of START/CONTINUE/INSPECT, unknown-field rejection,
  duplicate/trailing JSON rejection, invalid UTF-8, missing IDs, oversized
  frames, and schema-version failures. Test replies are canonical JSON with no
  diagnostic bytes mixed into stdout. Test `runtime exchange` performs one
  `Engine.Exchange` call and returns the committed reply, while an Engine error
  produces a stable reason-coded machine denial and human detail only on
  stderr. Test no Host Driver is touched by the neutral command.

- [ ] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/runtime ./internal/schema ./internal/cli -run 'Transport|RuntimeExchange|CanonicalFrame'
  ```

- [ ] **Step 3: Implement bounded strict transport.**

  Add a 1 MiB maximum, UTF-8 check, `json.Decoder.DisallowUnknownFields`,
  exactly-one-value enforcement, Runtime frame validation through the existing
  Engine seam, and `canonicaljson.Marshal` for replies. Use a closed
  `DENIED` error envelope for transport failures so stdout remains JSON-only;
  never print stack traces, raw Host output, or process diagnostics there.

- [ ] **Step 4: Route `oaw runtime exchange`.**

  Extend argument parsing without changing catalog/check behavior. Add
  `RunWithInput` for deterministic tests and keep `Run` as the os.Stdin/os.Stdout
  wrapper. Build a read-only Runtime Engine from explicit state/config options
  and reject unsupported Host selection; do not infer or promote a Host from
  local installation or discovery alone.

- [ ] **Step 5: Run focused and compatibility tests.**

  ```bash
  rtk gofmt -w internal/runtime internal/schema internal/cli cmd/oaw
  rtk go test ./internal/runtime ./internal/schema ./internal/cli
  rtk go test ./...
  ```

- [ ] **Step 6: Commit.**

  ```bash
  rtk git add internal/runtime internal/schema internal/cli cmd/oaw internal/assets
  rtk git commit -m "feat: expose canonical runtime protocol transport"
  ```

## Task 3: Implement the Codex runner and bounded process normalization

**Files:**
- Create: `internal/host/driver.go`, `internal/host/driver_test.go`
- Create: `internal/host/codex/runner.go`, `internal/host/codex/runner_test.go`
- Create: `internal/host/codex/output.go`, `internal/host/codex/output_test.go`
- Create: `internal/host/codex/fixture_test.go`

- [ ] **Step 1: Write failing driver and process tests.**

  Use a fixture executable, never a real Codex/model invocation. Prove the
  driver passes the exact Binding reference, Invocation ID, Executor ID, and
  Bundle digest; uses `codex exec --json --ephemeral` plus an explicit sandbox;
  bounds stdout/stderr and event count; preserves cancellation; deduplicates a
  repeated Invocation ID; and rejects a wrong Host, unsupported Binding kind,
  forged IDs, malformed JSONL, unknown outcome, or oversized output. Assert
  stdout remains untrusted and only a digest reference is returned.

- [ ] **Step 2: Run focused tests and verify RED.**

  ```bash
  rtk go test ./internal/host ./internal/host/codex -run 'Driver|Codex|Output|Fixture'
  ```

- [ ] **Step 3: Implement the Host-neutral Driver seam.**

  Add immutable dispatch request/result records and strict validation. `Prepare`
  records intent only. `Invoke` is the only method allowed to start a process;
  the driver owns a bounded per-invocation process table and supports
  cancellation. Duplicate invocation IDs return the first normalized result.

- [ ] **Step 4: Implement the Codex process adapter.**

  Resolve the executable from an explicit option or `PATH`; do not execute
  shell strings. Invoke the exact process with argument slices and a bounded
  writer. Use `--json`, `--ephemeral`, `--sandbox workspace-write`, and a
  deterministic prompt envelope containing the Binding reference and OAW
  Invocation ID. Parse JSONL into a closed final outcome; hash canonical
  normalized event evidence, and send stderr diagnostics only to the caller's
  stderr. Never return raw output to Runtime.

- [ ] **Step 5: Add pause/cancel and conformance fixture behavior.**

  Pause marks an active invocation as paused without replaying it; cancel
  terminates the context and returns a stable cancellation result. Exercise
  duplicate delivery and cancellation under `-race`. Reuse the same fixture
  identities as `host.RunConformance` so the built-in report cannot drift from
  the driver contract.

- [ ] **Step 6: Run focused tests and commit.**

  ```bash
  rtk gofmt -w internal/host internal/host/codex
  rtk go test ./internal/host ./internal/host/codex -count=1
  rtk go test -race ./internal/host ./internal/host/codex
  rtk git add internal/host
  rtk git commit -m "feat: add bounded codex runtime driver"
  ```

## Task 4: Drive `oaw run` through the same Runtime Protocol

**Files:**
- Modify: `internal/cli/run.go`
- Create: `internal/cli/run_runtime.go`, `internal/cli/run_runtime_test.go`
- Modify: `internal/runtime/transport.go`
- Create: `internal/integration/codex_run_test.go`

- [ ] **Step 1: Write failing runner-loop tests.**

  Feed a deterministic START/selection/stage-grant exchange to
  `RunWithInput`. Assert every emitted frame is canonical JSON and every
  transition is produced by `Engine.Exchange`. For `GRANT_ISSUED`, assert the
  loop first calls Driver.Prepare, commits `DISPATCH_PREPARED`, observes
  `DISPATCH_AUTHORIZED`, invokes Codex exactly once, and commits a normalized
  `CAPABILITY_OBSERVED` result. Assert replayed invocation IDs do not start a
  second process. Assert a failed process yields the Runtime's typed paused or
  failed reply, not an uncommitted success.

- [ ] **Step 2: Run tests and verify RED.**

  ```bash
  rtk go test ./internal/cli ./internal/integration -run 'Run|Codex|Dispatch|Protocol'
  ```

- [ ] **Step 3: Implement `oaw run --host codex`.**

  Parse only explicit `--host=codex`/`--host codex` and bounded state/config
  options. Refuse absent, instruction-only, native-managed, or non-Codex Host
  IDs with stable `HOST_RUNTIME_UNSUPPORTED` diagnostics. Feed every external
  frame through the shared transport; do not copy Runtime transition logic into
  the CLI. The runner may auto-continue only for the committed dispatch
  handshake, and must stop for user Profile selection, escalation, uncertainty,
  or cancellation.

- [ ] **Step 4: Add end-to-end process-level coverage.**

  Use a fixture executable in an isolated PATH. Verify exact argument delivery,
  bounded output, stderr separation, Runtime revision ordering, owner-only state
  files, no raw output/secret persistence, and no invocation for instruction-
  only Hosts. Test Codex unavailable, malformed output, cancellation, duplicate
  replay, and unsupported Host selection.

- [ ] **Step 5: Run focused and full Go tests.**

  ```bash
  rtk gofmt -w internal/cli internal/runtime internal/integration
  rtk go test ./internal/cli ./internal/runtime ./internal/integration
  rtk go test ./...
  ```

- [ ] **Step 6: Commit.**

  ```bash
  rtk git add internal/cli internal/runtime internal/integration
  rtk git commit -m "feat: drive codex through runtime protocol"
  ```

## Task 5: Gate runtime-aware entrypoints and close migration evidence

**Files:**
- Create: `internal/host/entrypoint.go`, `internal/host/entrypoint_test.go`
- Modify: `internal/management/render.go`, `internal/management/render_test.go`
- Modify: `.scratch/oaw-runtime-vnext/issues/09-first-runtime-host-and-oaw-run.md`
- Modify: `.scratch/oaw-runtime-vnext/workflow.md`
- Modify: `.scratch/oaw-runtime-vnext/evidence/review.md`, `.scratch/oaw-runtime-vnext/evidence/verification.md`
- Modify: `docs/en/adapters.md`, `docs/zh/adapters.md`, `docs/en/architecture.md`, `docs/zh/architecture.md`

- [ ] **Step 1: Write failing capability-gate tests.**

  Assert a runtime-aware entrypoint is eligible only for the exact selected
  `oaw/codex-runner` record with a passing audit/conformance report and all
  required Features. Assert all other built-in Hosts remain Policy-only and
  that existing Bash installer output is byte-for-byte unchanged unless an
  explicit runtime capability record is supplied. Project configuration cannot
  grant this eligibility.

- [ ] **Step 2: Implement the narrow gate.**

  Add a pure `RuntimeEntrypointAllowed` check used by the runner/entrypoint
  projection. Do not mutate existing install authority or add runtime claims to
  unsupported target renderers. Document that Ticket 14 owns any management
  cutover; a policy adapter remains valid when Codex is absent.

- [ ] **Step 3: Perform two-axis review.**

  Review Standards: ownership, scope, no subagent use, TDD owner, API stability,
  docs parity, and no accidental install cutover. Review Spec: protocol
  canonicalization, stdout/stderr separation, exact Host selection, Host
  isolation/admission, grant ordering, deduplication, cancellation, raw-output
  exclusion, unsupported Host fallback, and migration evidence. Remediate every
  Critical/High finding before closure.

- [ ] **Step 4: Run the complete verification matrix.**

  ```bash
  rtk git diff --check
  rtk go test ./... -count=1
  rtk go test -race ./...
  rtk go vet ./...
  rtk go test ./... -coverprofile=/tmp/oaw-runtime-ticket09.cover
  rtk go tool cover -func=/tmp/oaw-runtime-ticket09.cover
  rtk bash -n install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk shellcheck -S warning -x install.sh lib/*.sh lib/commands/*.sh tests/*.sh scripts/*.sh
  rtk bash tests/run.sh
  rtk GOOS=linux GOARCH=amd64 go build -o /tmp/oaw-ticket09-linux ./cmd/oaw
  rtk GOOS=windows GOARCH=amd64 go build -o /tmp/oaw-ticket09-windows.exe ./cmd/oaw
  rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...
  ```

  Add bounded transport/driver fuzz tests and run their focused fuzz targets
  for at least two seconds. Do not run a real `codex exec` without explicit
  approval; all conformance and process tests use fixtures.

- [ ] **Step 5: Close Ticket 09 and commit evidence.**

  Mark all acceptance checks complete, record the selected Host, exact
  integration digest, conformance transcript digest, implementation commits,
  and verification output. Set the workflow tracker to the next dependency
  (`10-policy-vnext-and-runtime-projections`) while preserving MATT-SP-HYBRID
  ownership. Keep every non-Codex built-in Host `instruction-only`.

  ```bash
  rtk git add .scratch/oaw-runtime-vnext docs internal
  rtk git commit -m "docs: close first codex runtime host ticket"
  ```

## Plan Self-Review

- `oaw runtime exchange` and `oaw run` share the existing Runtime Protocol and
  never introduce a second state machine.
- Runtime state is authoritative; Host output is untrusted and reduced to
  closed observations/evidence digests before `Exchange` receives it.
- Codex is the only selected/potentially runtime-managed Host. Unsupported
  Hosts remain instruction-only and are never promoted by discovery, local
  installation, or project configuration.
- The runner prepares before authorization, invokes only after
  `DISPATCH_AUTHORIZED`, and deduplicates the committed Invocation ID.
- Existing Bash management remains authoritative until Ticket 14; this ticket
  cannot silently change install behavior or import Policy-only tasks.
- The plan's tests use public CLI/process seams and the already-approved Runtime
  exchange seam; no tests reach private implementation details.
