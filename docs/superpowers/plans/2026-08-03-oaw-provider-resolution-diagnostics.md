# OAW Provider Resolution Diagnostics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only dynamic Provider inspection command and preserve concrete Provider resolution reasons through Bounded and Workflow Runtime failures.

**Architecture:** One CLI assembly path loads configuration, performs discovery, and produces both the diagnostic Resolution Report and authority-bearing Effective Registry. Runtime receives the report as diagnostic-only input, while registry verification and Profile compilation remain the only authority paths. The public `oaw providers inspect` command renders deterministic text or JSON and exact Provider pin suggestions without writing configuration.

**Tech Stack:** Go 1.26, BurntSushi TOML, canonical JSON, existing OAW configuration/discovery/registry/runtime packages, shell release verification, Docker Desktop on macOS.

**Design spec:** `docs/superpowers/specs/2026-08-03-oaw-provider-resolution-diagnostics-design.md`

**Execution decision:** Inline Execution. Do not dispatch subagents. Implement in a new isolated worktree and preserve all unrelated changes in the main checkout.

---

## File Map

| File | Responsibility |
| --- | --- |
| `internal/profile/records.go` | Carry the resolved Provider and Capability identity on typed missing-Capability compiler errors. |
| `internal/profile/compile.go` | Emit selector-aware errors only at unverified Provider/Capability boundaries. |
| `internal/profile/profile_test.go` | Verify Profile Binding resolution is reflected in typed compiler errors. |
| `internal/registry/registry_test.go` | Prove ambiguity stays fail-closed even with authoritative bindings and version ordering. |
| `internal/runtime/records.go` | Carry immutable Provider Resolution Reports in Bounded Runtime options. |
| `internal/runtime/workflow_records.go` | Carry immutable Provider Resolution Reports in Workflow Runtime options. |
| `internal/runtime/provider_diagnostics.go` | Translate non-verified Provider resolutions into path-free Runtime diagnostics. |
| `internal/runtime/bounded.go` | Preserve specific Provider reasons during Bounded Capability selection. |
| `internal/runtime/workflow_start.go` | Preserve specific Provider reasons during initial Workflow Profile selection. |
| `internal/runtime/workflow_dispatch.go` | Preserve the same reasons during stable-boundary Profile switching. |
| `internal/runtime/bounded_test.go` | Test Bounded Provider-state mapping and non-masking behavior through `Engine.Exchange`. |
| `internal/runtime/workflow_start_test.go` | Test selected Workflow Profile diagnostics and unrelated ambiguity behavior. |
| `internal/cli/provider_inputs.go` | Assemble Configuration, Discovery, Resolution, and Registry inputs once for CLI consumers. |
| `internal/cli/provider_inputs_test.go` | Prove shared assembly is deterministic and read-only. |
| `internal/cli/providers.go` | Parse and execute `oaw providers inspect`; render text and JSON contracts. |
| `internal/cli/providers_test.go` | Test the public inspect command, pin rendering, sorting, statuses, and immutability. |
| `internal/cli/run.go` | Route the new command. |
| `internal/cli/run_runtime.go` | Reuse shared Provider inputs and pass Resolution Reports to Runtime. |
| `internal/cli/run_runtime_test.go` | Test precise runner diagnostics and zero dispatch for ambiguity. |
| `README.md`, `README-zh.md` | Document the new read-only command and recovery sequence. |
| `docs/en/lifecycle.md`, `docs/zh/lifecycle.md` | Document diagnostic-only Resolution Reports and new-Run semantics. |
| `docs/en/troubleshooting.md`, `docs/zh/troubleshooting.md` | Document ambiguous candidate inspection and exact pin recovery. |

## Task 0: Create the Isolated Phase 16 Worktree

**Files:**
- Worktree: `.worktrees/phase16-provider-resolution-diagnostics`
- Branch: `feat/provider-resolution-diagnostics`

- [ ] **Step 1: Confirm the current checkout and worktree directory**

```bash
rtk git status --short --branch
rtk git check-ignore -v .worktrees
rtk git worktree list --porcelain
```

Expected: the main checkout remains at the approved Phase 16 design commit,
`.worktrees/` is ignored by `.gitignore`, and no worktree already owns the new
branch or path.

- [ ] **Step 2: Create and enter the isolated worktree**

```bash
rtk git worktree add .worktrees/phase16-provider-resolution-diagnostics -b feat/provider-resolution-diagnostics
```

All remaining commands run from the absolute worktree path
`/Users/wifibaby4u/LLM/open-agent-workflow/.worktrees/phase16-provider-resolution-diagnostics`.

- [ ] **Step 3: Verify the clean baseline**

```bash
rtk git status --short --branch
rtk go test ./... -count=1
```

Expected: the worktree is clean and the complete baseline suite passes. If the
baseline fails, stop implementation and diagnose the pre-existing failure
before writing a Phase 16 test.

## Task 1: Preserve Resolved Selector Identity in Profile Errors

**Files:**
- Modify: `internal/profile/records.go`
- Modify: `internal/profile/compile.go`
- Modify: `internal/profile/profile_test.go`

- [ ] **Step 1: Write the failing compiler metadata test**

Add a test that removes the bound Provider Instance, compiles a Recipe whose
implementation selector is rebound from `acme/suite` to `vendor/suite`, and
asserts the typed error carries the resolved identity:

```go
func TestCompileMissingCapabilityCarriesResolvedSelector(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	delete(verified.providers, "vendor/suite")
	delete(verified.capabilities, "vendor/suite\x00implementation")

	_, err := profile.CompileRecipe(available, verified, recipe, []profile.ProfileBinding{{
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"},
		PreferredProviderID: "vendor/suite",
	}})
	var compileErr *profile.CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if compileErr.Code != "PROFILE_CAPABILITY_MISSING" || compileErr.ProviderID != "vendor/suite" || compileErr.CapabilityID != "implementation" {
		t.Fatalf("CompileError = %#v", compileErr)
	}
}
```

Also extend `TestCompileRecipeRejectsUnverifiedBindingAndMissingDescriptor` to
assert an undeclared verified binding does not expose selector metadata. That
case is a contract mismatch, not Provider discovery failure.

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
rtk go test ./internal/profile -run 'TestCompileMissingCapabilityCarriesResolvedSelector|TestCompileRecipeRejectsUnverifiedBindingAndMissingDescriptor' -count=1
```

Expected: FAIL because `CompileError` has no `ProviderID` or `CapabilityID`.

- [ ] **Step 3: Add selector-aware compile errors**

Extend the type and add a dedicated constructor:

```go
type CompileError struct {
	Code         string
	Detail       string
	ProviderID   string
	CapabilityID string
}

func compileCapabilityError(providerID, capabilityID, format string, values ...any) error {
	return &CompileError{
		Code:         "PROFILE_CAPABILITY_MISSING",
		Detail:       fmt.Sprintf(format, values...),
		ProviderID:   providerID,
		CapabilityID: capabilityID,
	}
}
```

Use `compileCapabilityError` only when `verified.Provider(providerID)` or
`verified.Capability(providerID, capabilityID)` is absent. Keep `compileError`
for identity mismatches, undeclared descriptors/bindings, unsupported modes,
and all graph validation failures.

- [ ] **Step 4: Run profile tests and verify GREEN**

Run:

```bash
rtk go test ./internal/profile -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the compiler slice**

```bash
rtk git add internal/profile/records.go internal/profile/compile.go internal/profile/profile_test.go
rtk git commit -m "feat: identify unresolved profile providers"
```

## Task 2: Return Concrete Provider Reasons in Bounded Runtime

**Files:**
- Modify: `internal/registry/registry_test.go`
- Create: `internal/runtime/provider_diagnostics.go`
- Modify: `internal/runtime/records.go`
- Modify: `internal/runtime/bounded.go`
- Modify: `internal/runtime/bounded_test.go`

- [ ] **Step 1: Strengthen the registry ambiguity contract**

Add a registry test with versions `6.0.3`, `6.1.1`, and `11c74d6b`, plus an
authoritative Codex binding inventory. Assert the state remains ambiguous, all
three candidates remain sorted in the report, and no Provider Instance enters
the Effective Registry. This proves neither newest-version ordering nor Host
binding availability selects authority.

Run:

```bash
rtk go test ./internal/registry -run TestResolveDoesNotSelectByVersionOrHostAffinity -count=1
```

Expected before adding the test: no matching tests. Expected after adding the
test: PASS against the existing fail-closed resolver.

- [ ] **Step 2: Extend the Bounded fixture with Resolution Reports**

Change the fixture and existing constructor to retain the report already
returned by `registry.Resolve`:

```go
type boundedRuntimeFixture struct {
	projectRoot string
	snapshot    config.Snapshot
	resolutions registry.ResolutionReport
	registry    registry.Registry
}
```

In `newBoundedRuntimeFixture`, replace the discarded report with:

```go
resolutions, effective, err := registry.Resolve(snapshot, evidence, inventory)
```

and pass `Resolutions: fixture.resolutions` from `boundedOptions`.

- [ ] **Step 3: Write the failing Bounded ambiguity test**

Create a fixture with two Superpowers version directories, an authoritative
Codex binding inventory, no Provider pin, and an explicit
`oaw/superpowers/review` selector. Assert through `Engine.Exchange`:

```go
if reply.Kind != runtime.ReplyCapabilitySelectionRequired || reply.Snapshot.Status != runtime.RunAwaitingCapability {
	t.Fatalf("reply = %#v", reply)
}
if len(reply.Diagnostics) != 1 || reply.Diagnostics[0].Code != "PROVIDER_CANDIDATE_AMBIGUOUS" {
	t.Fatalf("diagnostics = %#v", reply.Diagnostics)
}
if strings.Contains(reply.Diagnostics[0].Message, home) || strings.Contains(reply.Diagnostics[0].Message, "SKILL.md") {
	t.Fatalf("Runtime diagnostic leaked discovery paths: %q", reply.Diagnostics[0].Message)
}
```

Add table cases for not-found, pin-incompatible, binding-unavailable, disabled,
and untrusted resolutions. Keep an existing verified-but-missing Capability
case expecting `CAPABILITY_NOT_VERIFIED`.

- [ ] **Step 4: Run the Bounded tests and verify RED**

```bash
rtk go test ./internal/runtime -run 'TestStartBounded.*Provider|TestStartBoundedWithoutAdmissibleSelectorPersistsAwaiting' -count=1
```

Expected: FAIL because Runtime options do not carry the report and the result is
still `CAPABILITY_NOT_VERIFIED`.

- [ ] **Step 5: Add the path-free diagnostic translator**

Add `Resolutions registry.ResolutionReport` to `BoundedOptions`. Implement one
pure translator in `provider_diagnostics.go`:

```go
func providerResolutionDiagnostic(report registry.ResolutionReport, providerID string) (Diagnostic, bool) {
	resolution, found := report.Resolution(providerID)
	if !found || resolution.State == registry.Verified {
		return Diagnostic{}, false
	}
	return Diagnostic{
		Code: resolution.Reason,
		Message: fmt.Sprintf(
			"Provider %s is %s with %d candidate(s). Run oaw providers inspect --host <host>, update the user-owned Provider pin, then start a new Run.",
			providerID, resolution.State, len(resolution.Candidates),
		),
	}, true
}
```

The translator must use only `ProviderID`, state, reason, and candidate count.
It must not render `Location`, Evidence, or version fields.

- [ ] **Step 6: Preserve a complete selection diagnostic**

Replace the string-only result from `resolveBoundedSelector` with an unexported
diagnostic value:

```go
type boundedSelectionDiagnostic struct {
	Code    string
	Message string
}
```

After `admission.VerifyBoundedCapability` returns
`CAPABILITY_NOT_VERIFIED`, query `options.Resolutions` with the normalized
selector Provider ID. Use the concrete Provider diagnostic when present;
otherwise keep the current generic diagnostic. Do not apply Provider mapping to
trusted-rule mismatches that fail before Capability verification.

Change `boundedReply` to accept the complete diagnostic and persist its message
without changing reply kind, status, revision, or recovery actions.

- [ ] **Step 7: Run focused and full Runtime tests**

```bash
rtk go test ./internal/runtime -run 'Bounded|ProviderResolution' -count=1
rtk go test ./internal/runtime -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit the Bounded Runtime slice**

```bash
rtk git add internal/registry/registry_test.go internal/runtime/provider_diagnostics.go internal/runtime/records.go internal/runtime/bounded.go internal/runtime/bounded_test.go
rtk git commit -m "feat: explain bounded provider resolution failures"
```

## Task 3: Return Concrete Provider Reasons in Workflow Runtime

**Files:**
- Modify: `internal/runtime/workflow_records.go`
- Modify: `internal/runtime/workflow_start.go`
- Modify: `internal/runtime/workflow_dispatch.go`
- Modify: `internal/runtime/workflow_start_test.go`
- Modify: `internal/runtime/workflow_invariants_test.go`

- [ ] **Step 1: Retain Resolution Reports in Workflow fixtures**

Add `resolutions registry.ResolutionReport` to `internalWorkflowFixture`. Capture
the report returned by `registry.Resolve` and pass it through:

```go
Workflow: WorkflowOptions{
	Configuration: snapshot,
	Resolutions:   resolutions,
	Registry:      effective,
	Authority: admission.AuthorityCeiling{
		Effects: []string{"git-local", "read-project", "run-process", "write-project"},
		Resources: []string{"git-repository", "project", "project-worktree"},
		ResourceLeases: true,
		AllowDelegation: true,
	},
	Host: host.RuntimeFrame{IntegrationID: hostIntegration.ID},
	Executors: []WorkflowExecutorRegistration{
		{Registration: admission.ExecutorRegistration{ID: "executor-write", Kind: admission.ExecutorIsolated}},
		{Registration: admission.ExecutorRegistration{ID: "executor-review-1", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
		{Registration: admission.ExecutorRegistration{ID: "executor-review-2", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
	},
},
```

- [ ] **Step 2: Write failing Workflow selection tests**

Add a test whose Workflow START reaches `SELECTION_REQUIRED`, then selects a
Profile whose required Provider has an ambiguous Resolution Report. Assert the
selection call returns an error with Runtime code
`PROVIDER_CANDIDATE_AMBIGUOUS` and no second revision is committed.

Add three controls:

```go
// Initial START succeeds even when an unrelated Provider is ambiguous.
// A Profile that does not require the ambiguous Provider compiles normally.
// A verified Provider with an invalid Capability contract stays PROFILE_SELECTION_INVALID.
```

Add a Profile Binding case where `acme/suite` is rebound to `vendor/suite` and
assert the diagnostic comes from `vendor/suite`.

- [ ] **Step 3: Run the focused Workflow tests and verify RED**

```bash
rtk go test ./internal/runtime -run 'TestWorkflow.*ProviderResolution|TestWorkflowStart' -count=1
```

Expected: FAIL because `WorkflowOptions` has no Resolution Report and compiler
errors are still wrapped as `PROFILE_SELECTION_INVALID`.

- [ ] **Step 4: Map typed compile errors after compiler authority**

Add `Resolutions registry.ResolutionReport` to `WorkflowOptions`. Implement:

```go
func workflowCompileDiagnostic(report registry.ResolutionReport, err error) (Diagnostic, bool) {
	var compileErr *profile.CompileError
	if !errors.As(err, &compileErr) || compileErr.ProviderID == "" || compileErr.CapabilityID == "" {
		return Diagnostic{}, false
	}
	return providerResolutionDiagnostic(report, compileErr.ProviderID)
}
```

In both `selectWorkflowProfile` and `switchWorkflowProfile`, call
`profile.CompileProfile` first. Only when it returns a selector-aware error may
Runtime replace `PROFILE_SELECTION_INVALID` with the concrete Provider reason:

```go
if diagnostic, found := workflowCompileDiagnostic(engine.workflow.Resolutions, err); found {
	return RunReply{}, runtimeError(diagnostic.Code, diagnostic.Message, err)
}
return RunReply{}, runtimeError("PROFILE_SELECTION_INVALID", "selected Profile is not available", err)
```

- [ ] **Step 5: Run Workflow and complete Runtime tests**

```bash
rtk go test ./internal/runtime -run 'Workflow|ProviderResolution' -count=1
rtk go test ./internal/runtime -count=1
```

Expected: PASS with existing Runtime State invariants unchanged.

- [ ] **Step 6: Commit the Workflow Runtime slice**

```bash
rtk git add internal/runtime/workflow_records.go internal/runtime/workflow_start.go internal/runtime/workflow_dispatch.go internal/runtime/workflow_start_test.go internal/runtime/workflow_invariants_test.go
rtk git commit -m "feat: explain workflow provider resolution failures"
```

## Task 4: Share Provider Input Assembly Between CLI Consumers

**Files:**
- Create: `internal/cli/provider_inputs.go`
- Create: `internal/cli/provider_inputs_test.go`
- Modify: `internal/cli/run_runtime.go`
- Modify: `internal/cli/run_runtime_test.go`

- [ ] **Step 1: Write a failing deterministic assembly test**

Create temporary HOME and XDG configuration roots, write two Superpowers
version indicators, and call a package-private assembly seam twice. Assert:

```go
if first.Configuration.Digest() != second.Configuration.Digest() ||
	first.Discovery.Digest() != second.Discovery.Digest() ||
	first.Resolutions.Digest() != second.Resolutions.Digest() ||
	first.Registry.Digest() != second.Registry.Digest() {
	t.Fatal("Provider input assembly is not deterministic")
}
resolution, found := first.Resolutions.Resolution("oaw/superpowers")
if !found || resolution.State != registry.Ambiguous || len(resolution.Candidates) != 2 {
	t.Fatalf("resolution = %#v, found=%v", resolution, found)
}
```

Record the absent user config path and assert assembly does not create it.

- [ ] **Step 2: Run the new test and verify RED**

```bash
rtk go test ./internal/cli -run TestLoadProviderInputsIsDeterministicAndReadOnly -count=1
```

Expected: FAIL because the shared seam does not exist.

- [ ] **Step 3: Implement the shared assembly record**

Create focused records:

```go
type providerInputOptions struct {
	HostID        string
	ProjectRoot   string
	UserConfigRoot string
	UserHome      string
}

type providerInputs struct {
	Configuration  config.Snapshot
	Discovery      discovery.Report
	Resolutions    registry.ResolutionReport
	Registry       registry.Registry
	UserConfigPath string
	UserConfigExists bool
}
```

`loadProviderInputs` must:

1. validate the selected Runtime-capable Host using built-in/trusted Host
   integrations;
2. load the effective Configuration Snapshot;
3. run declarative discovery under the explicit physical user home;
4. resolve with `catalogHostBindings(snapshot.Catalog(), options.HostID)`;
5. inspect but never create `<UserConfigRoot>/config.toml`;
6. return stable wrapped errors without partial output.

- [ ] **Step 4: Refactor `newCLIEngine` to use the shared result**

Replace the inline `config.Load`, `os.UserHomeDir`, `discovery.Discover`, and
`registry.Resolve` block with `loadProviderInputs`. Pass both diagnostic and
authority inputs:

```go
Bounded: oawruntime.BoundedOptions{
	Configuration: inputs.Configuration,
	Resolutions:   inputs.Resolutions,
	Registry:      inputs.Registry,
	Authority:      authority,
	Executors: []admission.ExecutorRegistration{
		{ID: "oaw-codex-write", Kind: admission.ExecutorIsolated},
		{ID: "oaw-codex-review", Kind: admission.ExecutorIsolated},
	},
},
Workflow: oawruntime.WorkflowOptions{
	Configuration: inputs.Configuration,
	Resolutions:   inputs.Resolutions,
	Registry:      inputs.Registry,
	Authority:      authority,
	Host:           host.RuntimeFrame{IntegrationID: host.SelectedRuntimeIntegrationID},
	Executors:      executors,
},
```

Direct START continues to bypass Provider assembly exactly as before.

- [ ] **Step 5: Verify runner behavior and no dispatch**

Add a `RunWithInput` test with two discovered Superpowers candidates and an
explicit Bounded selector. Assert the reply diagnostic is
`PROVIDER_CANDIDATE_AMBIGUOUS`, the Codex fixture counter is absent, and stdout
decodes as canonical Runtime JSON.

Run:

```bash
rtk go test ./internal/cli -run 'TestLoadProviderInputs|TestRunCodex.*Ambiguous|TestRunCodexFixtureDispatch' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the shared assembly slice**

```bash
rtk git add internal/cli/provider_inputs.go internal/cli/provider_inputs_test.go internal/cli/run_runtime.go internal/cli/run_runtime_test.go
rtk git commit -m "refactor: share provider resolution inputs"
```

## Task 5: Add the Read-only Provider Inspection Command

**Files:**
- Create: `internal/cli/providers.go`
- Create: `internal/cli/providers_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/catalog.go`

- [ ] **Step 1: Write failing argument and routing tests**

Cover these accepted forms:

```text
oaw providers inspect --host codex
oaw providers inspect --host=codex --format=text
oaw providers inspect --host codex --project-root /absolute/project --format json
```

Cover missing/duplicate Host, relative project root, duplicate format, unknown
format, unknown action, and unexpected operands. Assert status `64` and no
Provider assembly for invalid arguments.

Add one unsupported Host case expecting status `69`, one invalid user config
case expecting status `65`, and one discovery containment failure expecting
status `65`. Each failure must leave stdout empty and put the stable reason on
stderr.

The parser returns this closed record:

```go
type providerCommand struct {
	hostID      string
	projectRoot string
	format      string
}
```

- [ ] **Step 2: Write failing text and JSON contract tests**

Use three Superpowers version candidates including a path containing a quote.
For text, assert deterministic Provider/candidate order and parse every emitted
pin fragment with `toml.Decode` into:

```go
var document config.UserConfigRecord
```

For JSON, decode and assert:

```go
if output.SchemaVersion != "oaw.provider-inspection/v1" ||
	output.Host != "codex" ||
	output.ConfigurationDigest == "" ||
	output.DiscoveryDigest == "" ||
	output.ResolutionDigest == "" ||
	output.RegistryDigest == "" {
	t.Fatalf("output = %#v", output)
}
```

Assert each ambiguous candidate has exactly one structured Provider pin with
matching ID, canonical location, and version. Assert no indicator content or
`SKILL.md` evidence path appears.

- [ ] **Step 3: Write the failing immutability test**

Create an existing `config.toml`, record bytes, `FileMode`, and `ModTime`, run
both output formats, then assert all three values are unchanged. Repeat with an
absent config file and assert it remains absent.

- [ ] **Step 4: Run the inspect tests and verify RED**

```bash
rtk go test ./internal/cli -run 'TestProvidersInspect|TestParseProviders' -count=1
```

Expected: FAIL because the command is not routed.

- [ ] **Step 5: Implement the command parser and output records**

Define:

```go
const providerInspectionSchemaV1 = "oaw.provider-inspection/v1"

type providerInspectionOutput struct {
	SchemaVersion       string                       `json:"schema_version"`
	Host                string                       `json:"host"`
	UserConfigPath      string                       `json:"user_config_path"`
	UserConfigExists    bool                         `json:"user_config_exists"`
	ConfigurationDigest string                       `json:"configuration_digest"`
	CatalogDigest       string                       `json:"catalog_digest"`
	DiscoveryDigest     string                       `json:"discovery_digest"`
	ResolutionDigest    string                       `json:"resolution_digest"`
	RegistryDigest      string                       `json:"registry_digest"`
	Providers           []providerInspectionProvider `json:"providers"`
}

type providerInspectionProvider struct {
	ProviderID string                        `json:"provider_id"`
	State      registry.ProviderState        `json:"state"`
	Reason     string                        `json:"reason"`
	Instance   *providerInspectionInstance   `json:"instance,omitempty"`
	Candidates []providerInspectionCandidate `json:"candidates"`
}

type providerInspectionInstance struct {
	Location            string `json:"location"`
	Version             string `json:"version"`
	DescriptorDigest    string `json:"descriptor_digest"`
	ConfigurationDigest string `json:"configuration_digest"`
	BindingDigest       string `json:"binding_digest"`
	EvidenceDigest      string `json:"evidence_digest"`
	Digest              string `json:"digest"`
}

type providerInspectionCandidate struct {
	Version        string              `json:"version"`
	Location       string              `json:"location"`
	EvidenceDigest string              `json:"evidence_digest"`
	ProviderPin    *config.ProviderPin `json:"provider_pin,omitempty"`
}
```

Add dedicated provider and verified-instance projections rather than exposing
registry internals directly. Clone all slices and pointers before rendering.

The text renderer uses this stable field order:

```text
configuration path=<path> exists=<true|false>
provider <qualified-id> state=<state> reason=<reason>
candidate version=<version> location=<canonical-path> evidence_digest=<digest>
```

For ambiguous candidates, render the TOML suggestion immediately after its
candidate line. Separate Provider sections and pin suggestions with exactly one
blank line so repeated output is byte-for-byte deterministic.

- [ ] **Step 6: Render deterministic, TOML-safe suggestions**

Build one `config.ProviderPin` per ambiguous candidate. Use
`toml.NewEncoder(&buffer).Encode(...)` with a typed wrapper:

```go
type providerPinDocument struct {
	SchemaVersion string               `toml:"schema_version,omitempty"`
	ProviderPins  []config.ProviderPin `toml:"provider_pins"`
}
```

Include `SchemaVersion` only for the complete absent-config example. Never
construct TOML by concatenating candidate paths.

- [ ] **Step 7: Route and document command help**

Add `case "providers": return runProviders(...)` to `RunWithInput`. Extend the
public command usage without changing management command compatibility:

```text
oaw providers inspect --host host [--project-root path] [--format text|json]
```

- [ ] **Step 8: Run CLI and full Go tests**

```bash
rtk go test ./internal/cli -count=1
rtk go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit the operator surface**

```bash
rtk git add internal/cli/providers.go internal/cli/providers_test.go internal/cli/run.go internal/cli/catalog.go
rtk git commit -m "feat: inspect dynamic provider resolution"
```

## Task 6: Document Recovery and Run Complete Verification

**Files:**
- Modify: `README.md`
- Modify: `README-zh.md`
- Modify: `docs/en/lifecycle.md`
- Modify: `docs/zh/lifecycle.md`
- Modify: `docs/en/troubleshooting.md`
- Modify: `docs/zh/troubleshooting.md`

- [ ] **Step 1: Add English and Chinese operator documentation**

Document the exact inspect command, the distinction between Catalog and dynamic
Provider resolution, all stable Provider states, read-only behavior, exact pin
example, and the requirement to begin a new Run after a Configuration Snapshot
changes.

The troubleshooting sequence must be executable:

```bash
rtk go run ./cmd/oaw providers inspect --host codex --format text
rtk go run ./cmd/oaw providers inspect --host codex --format json
```

State explicitly that OAW never selects a candidate or writes the pin.

Add this English recovery contract under the Provider section in
`docs/en/lifecycle.md` and link to it from `README.md`:

````markdown
### Inspecting Provider resolution

`oaw catalog list providers` shows declared Provider descriptors. It does not
show installed Provider Instances. Inspect dynamic discovery and verification
for the selected Host with:

```bash
oaw providers inspect --host codex --format text
```

The command is read-only. For an ambiguous Provider it lists every candidate
and an exact location-and-version pin. OAW never chooses a candidate or writes
the pin. After changing user configuration, begin a new Run so it captures the
new Configuration Snapshot.
````

Add the corresponding Chinese contract under the Provider section in
`docs/zh/lifecycle.md` and link to it from `README-zh.md`:

````markdown
### 检查 Provider 解析结果

`oaw catalog list providers` 展示声明的 Provider descriptor，不代表已经安装并
验证的 Provider Instance。使用以下只读命令检查指定 Host 的动态发现与验证结果：

```bash
oaw providers inspect --host codex --format text
```

当 Provider 存在歧义时，命令会列出全部 candidate 以及精确的 location-and-version
pin。OAW 不会替用户选择 candidate，也不会写入 pin。用户配置变化后必须启动新的
Run，使其捕获新的 Configuration Snapshot。
````

Add this English block to `docs/en/troubleshooting.md`:

````markdown
### Provider candidate ambiguity

Run `oaw providers inspect --host codex --format text`. A completed inspection
may report `PROVIDER_NOT_FOUND`, `PROVIDER_DISCOVERED_UNVERIFIED`,
`PROVIDER_CANDIDATE_AMBIGUOUS`, `PROVIDER_PIN_INCOMPATIBLE`,
`PROVIDER_BINDING_UNAVAILABLE`, `PROVIDER_DISABLED_BY_USER`, or
`PROVIDER_PROJECT_CONTENT_UNTRUSTED`.

For `PROVIDER_CANDIDATE_AMBIGUOUS`:

1. Inspect with the same `--project-root`, if the Run used one.
2. Select one candidate explicitly.
3. Add its exact `id`, `location`, and `version` suggestion under
   `[[provider_pins]]` in the user-owned configuration.
4. Begin a new Run. An existing Run cannot absorb the new Configuration
   Snapshot.

OAW does not choose a candidate and does not write the pin.
````

Add this Chinese block to `docs/zh/troubleshooting.md`:

````markdown
### Provider candidate 歧义

运行 `oaw providers inspect --host codex --format text`。检查成功完成时可能报告
`PROVIDER_NOT_FOUND`、`PROVIDER_DISCOVERED_UNVERIFIED`、
`PROVIDER_CANDIDATE_AMBIGUOUS`、`PROVIDER_PIN_INCOMPATIBLE`、
`PROVIDER_BINDING_UNAVAILABLE`、`PROVIDER_DISABLED_BY_USER` 或
`PROVIDER_PROJECT_CONTENT_UNTRUSTED`。

处理 `PROVIDER_CANDIDATE_AMBIGUOUS`：

1. 如果原 Run 使用了 `--project-root`，使用相同路径执行检查。
2. 明确选择一个 candidate。
3. 将建议中的精确 `id`、`location` 和 `version` 作为 `[[provider_pins]]`
   写入用户自己管理的配置。
4. 启动新的 Run；现有 Run 不能吸收新的 Configuration Snapshot。

OAW 不会替用户选择 candidate，也不会写入 pin。
````

- [ ] **Step 2: Run documentation and formatting checks**

```bash
rtk gofmt -w internal/profile internal/runtime internal/cli
rtk bash scripts/check-docs.sh
rtk git diff --check
```

Expected: PASS with no generated or unrelated changes.

- [ ] **Step 3: Run the complete native verification suite**

```bash
rtk go test ./... -count=1
rtk go test -race ./internal/registry ./internal/profile ./internal/runtime ./internal/cli -count=1
rtk go test ./internal/registry ./internal/profile ./internal/runtime ./internal/cli -coverprofile=/tmp/oaw-phase16.cover -count=1
rtk go tool cover -func=/tmp/oaw-phase16.cover
```

Expected: all tests PASS and total affected-package coverage is at least 80
percent. Coverage below 80 percent is blocking and must not be reported as a
successful verification result.

- [ ] **Step 4: Run the actual-home read-only inspection smoke**

Before and after the command, hash the real user configuration when it exists.
Then run:

```bash
rtk go run ./cmd/oaw providers inspect --host codex --format json
```

Expected: the three installed Superpowers candidates appear with
`PROVIDER_CANDIDATE_AMBIGUOUS`; no Provider, Host process, or model is invoked;
the user configuration is absent or byte-for-byte unchanged.

- [ ] **Step 5: Verify an exact pin through an isolated temporary config**

Create a temporary XDG config root with the inspect-selected exact location and
version, leaving the real user configuration untouched. Run the public inspect
command under that root and assert `oaw/superpowers` is `verified` with one
Provider Instance.

Then run the existing deterministic Codex runner fixture tests to prove the
verified result reaches admission without a live model call:

```bash
rtk go test ./internal/cli -run 'TestRunCodex.*Ambiguous|TestRunCodexFixtureDispatch' -count=1
```

- [ ] **Step 6: Build release archives and run Docker Linux verification**

```bash
PHASE16_RELEASE_ROOT=$(rtk mktemp -d /tmp/oaw-phase16-release.XXXXXX)
rtk bash scripts/build-release.sh "$PHASE16_RELEASE_ROOT"
rtk bash scripts/smoke-docker.sh "$PHASE16_RELEASE_ROOT/open-agent-workflow_0.1.0_linux_arm64.tar.gz"
```

Expected: Docker Linux/arm64 PASS. If Docker itself is unavailable, record its
status `77` as `SKIP`; any reachable Docker test failure remains blocking.

- [ ] **Step 7: Record the WSL platform result**

```bash
rtk bash scripts/smoke-wsl.sh "$PHASE16_RELEASE_ROOT/open-agent-workflow_0.1.0_linux_arm64.tar.gz"
```

Expected on macOS: status `77`, recorded as `SKIP` rather than PASS or failure.

- [ ] **Step 8: Verify Phase 15 evidence and perform focused security review**

```bash
rtk shasum -a 256 /Users/wifibaby4u/LLM/.oaw-pilots/2026-08-03-controlled-dogfooding/evidence/report.md
rtk shasum -a 256 /Users/wifibaby4u/LLM/.oaw-pilots/2026-08-03-controlled-dogfooding/evidence/workflow-final.md
rtk git diff --check
rtk git status --short
```

Expected Phase 15 hashes:

```text
0a5a1a5e9a0e92d578cd043cc1b5148e67a9b19520e984142606fb9f8251816c  report.md
fcfc812e000db419a9106a3294cc13c4740a704eb8499c04c97e3d80dcf2240e  workflow-final.md
```

Review the complete Phase 16 diff for path leakage, unsafe TOML construction,
configuration writes, discovery precedence, hidden Provider selection, and
Host invocation before verified admission. Fix every CRITICAL or HIGH finding
and repeat the affected tests.

- [ ] **Step 9: Commit documentation and verification-ready state**

```bash
rtk git add README.md README-zh.md docs/en/lifecycle.md docs/zh/lifecycle.md docs/en/troubleshooting.md docs/zh/troubleshooting.md
rtk git commit -m "docs: explain provider ambiguity recovery"
```

## Task 7: Final Review and Handoff

**Files:**
- Review: all Phase 16 commits and changed files

- [ ] **Step 1: Review the complete branch diff**

```bash
rtk git log --oneline main..HEAD
rtk git diff --stat main...HEAD
rtk git diff --check main...HEAD
rtk git status --short --branch
```

Expected: six scoped commits, no uncommitted implementation changes, and no
Phase 15 evidence changes.

- [ ] **Step 2: Re-run the highest-signal acceptance tests**

```bash
rtk go test ./internal/profile ./internal/registry ./internal/runtime ./internal/cli -count=1
rtk go test -race ./internal/runtime ./internal/cli -count=1
rtk bash scripts/check-docs.sh
```

Expected: PASS.

- [ ] **Step 3: Record completion evidence without external mutation**

Report the branch, worktree path, commit list, test counts, coverage total,
Docker/WSL statuses, actual-home inspect result, exact Phase 15 hashes, and any
residual risk. Do not push, merge, tag, release, or edit the real user Provider
configuration without a separate explicit user instruction.
