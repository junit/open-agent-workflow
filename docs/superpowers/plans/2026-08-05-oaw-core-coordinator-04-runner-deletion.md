# OAW Core Coordinator Phase 04 Runner Deletion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete every OAW-owned model process, Codex execution profile, Runner-only Host contract, and dead compatibility path, then add a permanent repository gate that prevents execution supervision from returning.

**Architecture:** Phase 03 already moved durable Workflow behavior into Coordinator and removed public Runtime commands. This phase removes now-unreachable implementations rather than wrapping them. Provider discovery stays in generic discovery/Registry packages; no Codex-specific process or environment code survives.

**Tech Stack:** Go 1.26, repository shell tests, static source inspection, Go dependency/build tests, macOS native verification.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not invoke a real model CLI while testing.

**Hard-cut integration boundary:** This phase closes the Phase 01-04 atomic replacement batch. Delete obsolete consumers before running the first required full-repository compile and test gate. A failure must be fixed in the replacement contracts or by completing the deletion set, never by restoring a legacy type or command.

**Depends on:** Phase 03 Coordinator CLI cutover.

**Produces:** First source tree in the branch with no execution Runtime or model Runner.

## Exact Deletion Set

Delete these paths completely:

```text
internal/host/codex/
internal/host/driver.go
internal/host/driver_test.go
internal/host/entrypoint.go
internal/host/entrypoint_test.go
internal/host/codex_manifest_test.go
```

Delete any remaining definitions or tests for:

```text
RunnerManaged
NativeManaged
RuntimeFrame
RuntimeProtocolV1
WorkflowAdmission
ConformanceAdapter
ExecutorRegistration
ExecutorMainAgent
ExecutorIsolated
FeatureIsolatedExecutor
FeatureNativeInvocation
```

Do not delete generic Host binding inventory, Provider discovery, config trust,
Core compilation, Workflow Coordinator, or policy installation adapters.

## Task 1: Add an executable-boundary regression test

**Files:**
- Create: `tests/15-host-execution-boundary-test.sh`
- Modify: `tests/run.sh`

- [ ] **Step 1: Write the failing source-boundary test**

The test scans production Go, active schemas/assets, and CLI help. Use an exact
allow-list only for diagnostic error strings that describe rejection. Fail on:

```text
codex exec
claude --
oaw/codex-runner
runner-managed
native-managed
private HOME
ignore-user-config
ignore-rules
disable hooks
isolated-executor
native-invocation
```

Also fail when any production file under `internal/host` imports `os/exec` or
exposes a method named `Invoke` that starts a process.

- [ ] **Step 2: Run the test to verify RED**

```bash
rtk bash tests/15-host-execution-boundary-test.sh
```

Expected: FAIL and list the existing Codex Runner/process-profile paths.

- [ ] **Step 3: Add the test to the repository suite**

Append one direct call in `tests/run.sh`:

```bash
run_test "15-host-execution-boundary-test.sh"
```

Do not suppress exit 1 from a forbidden match.

- [ ] **Step 4: Commit the RED regression gate**

```bash
rtk git add tests/15-host-execution-boundary-test.sh tests/run.sh
rtk git commit -m "test: forbid OAW-owned model execution"
```

## Task 2: Delete the Codex Runner and process profiles

**Files:**
- Delete: `internal/host/codex/inventory.go`
- Delete: `internal/host/codex/inventory_test.go`
- Delete: `internal/host/codex/output.go`
- Delete: `internal/host/codex/output_fuzz_test.go`
- Delete: `internal/host/codex/output_test.go`
- Delete: `internal/host/codex/profile.go`
- Delete: `internal/host/codex/profile_test.go`
- Delete: `internal/host/codex/runner.go`
- Delete: `internal/host/codex/runner_test.go`
- Delete: `internal/host/driver.go`
- Delete: `internal/host/driver_test.go`
- Delete: `internal/host/entrypoint.go`
- Delete: `internal/host/entrypoint_test.go`
- Delete: `internal/host/codex_manifest_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/workflow.go`
- Modify: `internal/cli/workflow_test.go`
- Modify: `internal/integration/host_configuration_test.go`

- [ ] **Step 1: Confirm no active caller remains**

```bash
rtk rg -n 'host/codex|host\.Driver|RuntimeEntrypointAllowed|SelectedRuntimeIntegrationID|runCodexContext|runHostLoop' internal cmd
```

Expected: matches exist only in the deletion set. If a Coordinator or Core
match appears, remove that dependency before proceeding.

- [ ] **Step 2: Delete the complete process implementation**

Use `git rm` for the exact deletion set. Do not retain output parsing,
inventory probing, execution-profile creation, Skill staging, private HOME,
neutral workspace, MCP filtering, cancellation process tables, or a no-op
Driver interface.

- [ ] **Step 3: Remove CLI assembly imports and fixtures**

The CLI must assemble only management, catalog, Provider inspection, and
Workflow Coordinator commands. It must not accept `--host codex`, a model
command path, sandbox mode, Codex HOME, or execution root.

- [ ] **Step 4: Run focused compile and boundary checks**

```bash
rtk gofmt -w internal/cli internal/integration
rtk go test ./internal/host ./internal/cli ./internal/integration
rtk bash tests/15-host-execution-boundary-test.sh
```

Expected: PASS.

- [ ] **Step 5: Commit Runner deletion**

```bash
rtk git add -u internal/host
rtk git add internal/cli internal/integration
rtk git commit -m "refactor: delete codex execution runner"
```

## Task 3: Remove obsolete Host and Grant vocabulary

**Files:**
- Modify: `internal/host/records.go`
- Modify: `internal/host/validate.go`
- Modify: `internal/host/records_test.go`
- Modify: `internal/host/validation_test.go`
- Modify: `internal/hosttest/fixture.go`
- Modify: `internal/admission/records.go`
- Modify: `internal/admission/admit.go`
- Modify: `internal/admission/admission_test.go`
- Modify: `internal/config/snapshot.go`
- Modify: `internal/config/snapshot_test.go`
- Modify: `internal/integration/config_discovery_test.go`
- Modify: `internal/integration/host_conformance_test.go`

- [ ] **Step 1: Write absence and replacement tests**

Compile tests using only `ControlSurface`, `SessionSnapshot`, execution
topologies, topology-aware binding inventory, v2 Grants, Dispatch Packets, and
Receipts. Add schema rejection cases for every deleted old field/value.

- [ ] **Step 2: Run focused tests to expose stale symbols**

```bash
rtk go test ./internal/host ./internal/hosttest ./internal/admission ./internal/config ./internal/integration
```

Expected: compile failures identify every stale old symbol.

- [ ] **Step 3: Delete obsolete records and branches**

Do not rename old concepts into aliases. Remove their constructors, clone
helpers, validators, error cases, fixture fields, and test data. Keep only
`policy` and `host-native` control surfaces and `CURRENT`/`SUBAGENT` topology.

- [ ] **Step 4: Run GREEN and static scans**

```bash
rtk gofmt -w internal/host internal/hosttest internal/admission internal/config internal/integration
rtk go test ./internal/host ./internal/hosttest ./internal/admission ./internal/config ./internal/integration
rtk rg -n 'RunnerManaged|NativeManaged|RuntimeFrame|WorkflowAdmission|ExecutorRegistration|ExecutorMainAgent|ExecutorIsolated' internal
```

Expected: tests pass; final `rg` exits 1.

- [ ] **Step 5: Commit vocabulary deletion**

```bash
rtk git add internal/host internal/hosttest internal/admission internal/config internal/integration
rtk git commit -m "refactor: remove legacy execution authority"
```

## Task 4: Prove the binary has no model-launch path

**Files:**
- Modify: `tests/01-cli-test.sh`
- Modify: `tests/06-security-test.sh`
- Modify: `tests/14-cutover-release-test.sh`
- Modify: `tests/15-host-execution-boundary-test.sh`

- [ ] **Step 1: Add CLI black-box assertions**

Assert these commands fail without state or subprocess effects:

```bash
"$OAW_BIN" run --host codex
"$OAW_BIN" runtime exchange
```

Assert help lists `workflow exchange` and does not list `run`, `runtime`, a
model Host, sandbox, execution root, or private HOME option.

- [ ] **Step 2: Add a PATH trap fixture**

Place test executables named `codex`, `claude`, `gemini`, and `opencode` in a
temporary PATH. Each writes a sentinel and exits 99. Run every public OAW
command exercised by the CLI suite and assert no sentinel exists.

- [ ] **Step 3: Run complete native verification**

```bash
rtk go test -race ./internal/core ./internal/coordinator ./internal/host ./internal/profile ./internal/registry
rtk go test ./...
rtk bash tests/run.sh
rtk bash scripts/check-docs.sh
rtk git diff --check
```

Expected: PASS. No real model command is invoked.

- [ ] **Step 4: Commit the hard-cut boundary gates**

```bash
rtk git add tests
rtk git commit -m "test: prove host-owned agent execution"
```

## Phase 04 Completion Gate

- [ ] The exact deletion set is absent from the filesystem and Git index.
- [ ] No production Go package imports `os/exec` for Agent/model execution.
- [ ] Public CLI has no model-launch command or option.
- [ ] PATH trap proves OAW never starts a model CLI.
- [ ] No compatibility alias, legacy schema reader, or no-op Driver remains.
- [ ] The branch is now eligible for documentation cutover and full local
      release-readiness testing, but P3 publication remains frozen.
