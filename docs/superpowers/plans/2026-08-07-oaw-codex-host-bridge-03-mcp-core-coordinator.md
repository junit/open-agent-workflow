# OAW Codex Host Bridge 03: MCP, Core, and Coordinator Integration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose four narrow MCP operations that create current-session evidence, inspect and compile Profiles through OAW Core, and exchange existing Coordinator records without executing any engineering Capability.

**Architecture:** A single stdio MCP server owns the metadata client and in-memory Evidence Store. `observe_current` is the only operation that creates facts; all other operations resolve its opaque handle and use the exact cached Configuration, Discovery, Registry, Host Session, Inventory, and Environment records. Core and Coordinator remain the only policy compilers/state machines, while the current Codex session remains the physical executor.

**Tech Stack:** Go 1.26, official `github.com/modelcontextprotocol/go-sdk/mcp` v1.7.0, existing Core and Coordinator APIs, strict JSON schemas, in-memory MCP integration tests.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish an intermediate phase commit.

**Depends on:** Plans 01 and 02.

**Produces:** `oaw bridge serve codex` service API and verified Core/Coordinator exchange consumed by packaging in Plan 05.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/codexbridge/service.go` | Four operation handlers and shared Host evidence validation. |
| `internal/codexbridge/inputs.go` | Closed public MCP input/output records; caller cannot supply Host facts. |
| `internal/codexbridge/mcp.go` | MCP server registration, explicit public schemas, structured results, and stdio runner. |
| `internal/codexbridge/service_test.go` | Observe, inspect, compile, changed facts, and Coordinator behavior. |
| `internal/codexbridge/mcp_test.go` | MCP initialize, tools/list, input schema, structured result, and approval-surface tests. |
| `internal/cli/provider_inputs.go` | Pass a Bridge-created Binding Inventory into existing Core resolution. |
| `internal/cli/provider_inputs_test.go` | Candidate-to-verified resolution with exact inventory. |
| `internal/cli/providers.go` | Add fail-closed strict inspection projection without inventing Host evidence. |
| `internal/cli/providers_test.go` | Strict/no-Bridge and verified-inventory diagnostics. |
| `internal/coordinator/engine.go` | No authority change; expose inspection needed for Bridge preflight through existing command protocol. |
| `internal/coordinator/start_test.go` | Core and Coordinator pin identical Host fact digests. |

## Locked MCP Tool Surface

The MCP server key is `oaw_codex_bridge`; the four generated Codex tool names
used by the Hook matcher are:

```text
mcp__oaw_codex_bridge__observe_current
mcp__oaw_codex_bridge__core_inspect
mcp__oaw_codex_bridge__core_compile
mcp__oaw_codex_bridge__workflow_exchange
```

The logical operation names remain `observe_current`, `core.inspect`,
`core.compile`, and `workflow_exchange`. The underscore conversion belongs
only to the MCP adapter and must not leak into OAW Core records.

Public MCP inputs are closed:

```go
type ObserveCurrentInput struct{}
type CoreInspectInput struct {
	HostEvidenceHandle HostEvidenceHandle                   `json:"host_evidence_handle"`
	DeliverableID      string                               `json:"deliverable_id"`
	InputDigest        string                               `json:"input_digest"`
	Proposal           classification.ClassificationProposal `json:"proposal"`
}
type CoreCompileInput struct {
	HostEvidenceHandle HostEvidenceHandle                   `json:"host_evidence_handle"`
	DeliverableID      string                               `json:"deliverable_id"`
	InputDigest        string                               `json:"input_digest"`
	Proposal           classification.ClassificationProposal `json:"proposal"`
	Selection          core.Selection                       `json:"selection"`
}
type WorkflowExchangeInput struct {
	HostEvidenceHandle HostEvidenceHandle `json:"host_evidence_handle"`
	Command            coordinator.Command `json:"command"`
}

type ProviderStateSummary struct {
	ProviderID string                 `json:"provider_id"`
	State      registry.ProviderState `json:"state"`
}
type HostSummary struct {
	SessionDigest   string                 `json:"session_digest"`
	InventoryDigest string                 `json:"inventory_digest"`
	EnvironmentDigest string               `json:"environment_digest"`
	Providers       []ProviderStateSummary `json:"providers"`
	Diagnostics     []Diagnostic           `json:"diagnostics"`
	DirectAvailable bool                   `json:"direct_available"`
}
type ObserveCurrentOutput struct {
	HostEvidenceHandle HostEvidenceHandle `json:"host_evidence_handle"`
	HostSummary        HostSummary        `json:"host_summary"`
}
type CoreInspectOutput struct {
	Classification classification.ClassificationDecision `json:"classification"`
	HostSummary    HostSummary                            `json:"host_summary"`
	Compilation    *core.CompilationResult                `json:"compilation,omitempty"`
}
```

The private `_oaw_host_context` field exists only after the Hook rewrites
`observe_current`. The public `tools/list` schema must not advertise it and
must set `additionalProperties: false` at every object boundary.

## Task 1: Route Bridge inventory through the existing Core resolution seam

**Files:**
- Modify: `internal/cli/provider_inputs.go`
- Modify: `internal/cli/provider_inputs_test.go`
- Modify: `internal/cli/providers.go`
- Modify: `internal/cli/providers_test.go`

- [ ] **Step 1: Write failing inventory and strict-inspection tests**

```go
func TestLoadProviderInputsUsesHostBindingInventory(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	inputs, err := loadProviderInputs(providerInputOptions{HostID: "codex", UserHome: fixture.Home, Inventory: &fixture.Inventory})
	if err != nil { t.Fatal(err) }
	resolution, ok := inputs.Resolutions.Resolution("acme/suite")
	if !ok || resolution.State != registry.Verified { t.Fatalf("resolution = %#v", resolution) }
}

func TestStrictProviderInspectionFailsClosedWithoutBridgeInventory(t *testing.T) {
	status, stderr := runProvidersFixture(t, []string{"inspect", "--host", "codex", "--strict"}, nil)
	if status == 0 || !strings.Contains(stderr, "HOST_BRIDGE_UNAVAILABLE") { t.Fatalf("status=%d stderr=%s", status, stderr) }
}
```

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/cli -run 'ProviderInputsUses|StrictProviderInspection'
```

Expected: FAIL because `Inventory` and `--strict` do not exist.

- [ ] **Step 3: Pass the exact inventory pointer into Core**

```go
type providerInputOptions struct {
	HostID                    string
	ProjectRoot               string
	UserConfigRoot            string
	UserHome                  string
	Inventory                 *host.BindingInventory
	IncludeForeignDiagnostics bool
}

type providerInputs struct {
	HostID           string
	Configuration    config.Snapshot
	Discovery        discovery.Report
	Inventory        *host.BindingInventory
	Resolutions      registry.ResolutionReport
	Registry         registry.Registry
	Foreign          []foreignProviderDiscovery
	UserConfigPath   string
	UserConfigExists bool
}

resolved, err := core.Resolve(core.ResolutionRequest{
	Configuration: snapshot,
	HostID: options.HostID,
	Discovery: evidence,
	Inventory: options.Inventory,
})
```

Clone and validate the inventory before passing it. `nil` remains honest
Candidate-only diagnostics. Do not load an Inventory from a path, environment
variable, stdin, or user config. Set the returned `providerInputs.Inventory`
to that validated clone so strict projections can distinguish Bridge-backed
resolution from Candidate-only inspection.

- [ ] **Step 4: Add strict projection behavior**

`--strict` means "do not accept Candidate-only output as success." A direct
CLI process has no access to the MCP process's in-memory handle, so it returns
`HOST_BRIDGE_UNAVAILABLE` with recovery `invoke core.inspect in the active
Codex session`. The shared projection function accepts a non-nil inventory
when called inside the Bridge and then displays verified Providers. This keeps
the command honest without adding IPC, an evidence file, or a daemon.

```go
if parsed.strict && inputs.Inventory == nil {
	fmt.Fprintln(stderr, "oaw: HOST_BRIDGE_UNAVAILABLE: strict inspection requires current-session Bridge evidence")
	return 69
}
```

- [ ] **Step 5: Run GREEN**

```bash
rtk gofmt -w internal/cli/provider_inputs.go internal/cli/provider_inputs_test.go internal/cli/providers.go internal/cli/providers_test.go
rtk go test ./internal/cli -run 'Provider|Strict'
rtk go test ./internal/core ./internal/registry
```

Expected: PASS; no inventory still produces Candidate state in non-strict
diagnostics, while a Bridge inventory produces a verified Instance.

- [ ] **Step 6: Commit the Core input seam**

```bash
rtk git add internal/cli/provider_inputs.go internal/cli/provider_inputs_test.go internal/cli/providers.go internal/cli/providers_test.go
rtk git commit -m "feat: resolve Providers from Bridge inventory"
```

## Task 2: Implement the four Bridge service handlers

**Files:**
- Create: `internal/codexbridge/inputs.go`
- Create: `internal/codexbridge/service.go`
- Create: `internal/codexbridge/service_test.go`

- [ ] **Step 1: Write failing operation tests**

```go
func TestObserveCurrentCreatesHandleFromInjectedContext(t *testing.T) {
	service := newTestService(t)
	result, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil || result.HostEvidenceHandle.Token == "" { t.Fatalf("result=%#v err=%v", result, err) }
	if result.HostSummary.SessionDigest == "" || result.HostSummary.InventoryDigest == "" { t.Fatalf("summary=%#v", result.HostSummary) }
}

func TestObserveCurrentPropagatesOptionalMetadataDiagnostics(t *testing.T) {
	service := newTestService(t)
	service.observer.(*fakeObserver).SetDiagnostics([]appserver.ObservationDiagnostic{{Code: "HOST_OBSERVATION_PARTIAL", Detail: "hooks/list unavailable"}})
	result, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil { t.Fatal(err) }
	if !hasDiagnostic(result.HostSummary.Diagnostics, "HOST_OBSERVATION_PARTIAL") { t.Fatalf("summary=%#v", result.HostSummary) }
}

func TestCoreCompileCannotReplaceCachedHostFacts(t *testing.T) {
	_, handle := observedTestService(t)
	input := validCompileInput(t, handle)
	raw, _ := json.Marshal(input)
	var object map[string]any
	json.Unmarshal(raw, &object)
	object["host_session"] = map[string]any{"host_id":"forged"}
	forged, _ := json.Marshal(object)
	if _, err := DecodeCoreCompileInput(forged); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" { t.Fatalf("error=%v", err) }
}

func TestWorkflowExchangeRejectsChangedPinnedFactsBeforeMutation(t *testing.T) {
	service, handle, command := startedWorkflow(t)
	service.observer.(*fakeObserver).Set(changedSkillObservation(t))
	changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil { t.Fatal(err) }
	if changed.HostEvidenceHandle.Token == handle.Token { t.Fatal("changed observation reused the old handle") }
	if _, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{HostEvidenceHandle: changed.HostEvidenceHandle, Command: command}); Code(err) != "HOST_SESSION_CHANGED" { t.Fatalf("error=%v", err) }
}
```

Add a successful inspect/compile test for a verified Superpowers fixture, a
user-defined Provider, and the honest exclusion of Matt/ECC when their exact
Skills are absent.

The `service_test.go` fixture defines `fakeObserver.Set`,
`fakeObserver.SetDiagnostics`, `newTestService`, `observedTestService`,
`startedWorkflow`, `testHookContext`, `validCompileInput`,
`changedSkillObservation`, and `hasDiagnostic`; every fixture uses `t.TempDir()`
and the fake App Server path, never a live Codex process. `SetDiagnostics` only
changes the redacted `MetadataObservation.Diagnostics` field.

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge -run 'ObserveCurrent|CoreCompile|WorkflowExchange'
```

Expected: FAIL because `Service` is absent.

- [ ] **Step 3: Implement observation and fact storage**

```go
type Observer interface {
	Observe(context.Context, string) (appserver.MetadataObservation, error)
}
type ServiceOptions struct {
	Observer Observer
	Store EvidenceStore
	StateRoot string
	ProjectRoot string
	UserConfigRoot string
	UserHome string
	BridgeVersion string
	Rules classification.ClassificationRules
	Authority admission.AuthorityCeiling
}

type Service struct {
	observer      Observer
	store         EvidenceStore
	stateRoot     string
	projectRoot   string
	userConfigRoot string
	userHome      string
	rules         classification.ClassificationRules
	authority     admission.AuthorityCeiling
	bridgeVersion string
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Observer == nil || options.Store == nil { return nil, NewError("HOST_BRIDGE_UNAVAILABLE", "Observer and EvidenceStore are required", nil) }
	return &Service{observer: options.Observer, store: options.Store, stateRoot: options.StateRoot, projectRoot: options.ProjectRoot, userConfigRoot: options.UserConfigRoot, userHome: options.UserHome, rules: cloneClassificationRules(options.Rules), authority: admission.CloneAuthority(options.Authority), bridgeVersion: options.BridgeVersion}, nil
}

func cloneClassificationRules(value classification.ClassificationRules) classification.ClassificationRules {
	value.User.ProtectedResources = append([]classification.Resource{}, value.User.ProtectedResources...)
	value.User.RequiredEvidence = append([]classification.EvidenceKind{}, value.User.RequiredEvidence...)
	value.Project.ProtectedResources = append([]classification.Resource{}, value.Project.ProtectedResources...)
	value.Project.RequiredEvidence = append([]classification.EvidenceKind{}, value.Project.RequiredEvidence...)
	return value
}

func (s *Service) ObserveCurrent(ctx context.Context, _ ObserveCurrentInput, hostContext HookContext) (ObserveCurrentOutput, error) {
	metadata, err := s.observer.Observe(ctx, hostContext.CWD)
	if err != nil { return ObserveCurrentOutput{}, bridgeErrorFromAppServer(err) }
	snapshot, discovery, err := s.loadInputs(hostContext.CWD)
	if err != nil { return ObserveCurrentOutput{}, err }
	inventory, diagnostics, err := BuildBindingInventory(snapshot.Catalog(), discovery, metadata, hostContext.CWD)
	if err != nil { return ObserveCurrentOutput{}, err }
	diagnostics = append(diagnostics, projectObservationDiagnostics(metadata.Diagnostics)...)
	resolved, err := core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: "codex", Discovery: discovery, Inventory: &inventory})
	if err != nil { return ObserveCurrentOutput{}, err }
	facts, err := AssembleFacts(hostContext, metadata, snapshot, discovery, inventory, resolved)
	if err != nil { return ObserveCurrentOutput{}, err }
	handle, err := s.store.Put(hostContext, facts)
	return ObserveCurrentOutput{HostEvidenceHandle: handle, HostSummary: secretFreeSummary(facts, diagnostics)}, err
}

func projectObservationDiagnostics(values []appserver.ObservationDiagnostic) []Diagnostic {
	result := make([]Diagnostic, 0, len(values))
	for _, value := range values {
		result = append(result, NewDiagnostic(value.Code, "observation", value.Detail, true))
	}
	return result
}

func bridgeErrorFromAppServer(err error) error {
	if err == nil { return nil }
	code := appserver.Code(err)
	if code == "" { return NewError("HOST_OBSERVATION_FAILED", "Codex metadata observation failed", err) }
	return NewError(code, "Codex metadata observation failed", err)
}
```

`secretFreeSummary` exposes short digests, Provider IDs/states, Profile-impact
diagnostics, Direct availability, and recovery actions. It does not return
absolute Skill paths, the token in a log field, raw config, or Hook commands.

```go
func secretFreeSummary(facts Facts, diagnostics []Diagnostic) HostSummary {
	providers := make([]ProviderStateSummary, 0)
	for _, resolution := range facts.Resolutions.Resolutions() {
		providers = append(providers, ProviderStateSummary{ProviderID: resolution.ProviderID, State: resolution.State})
	}
	projected := make([]Diagnostic, len(diagnostics))
	for index, diagnostic := range diagnostics {
		projected[index] = diagnostic
		projected[index].AffectedProviders = append([]string{}, diagnostic.AffectedProviders...)
		projected[index].AffectedProfiles = append([]string{}, diagnostic.AffectedProfiles...)
		projected[index].EvidenceDigest = facts.Session.Digest[:16]
	}
	return HostSummary{
		SessionDigest: facts.Session.Digest[:16], InventoryDigest: facts.Inventory.Digest[:16], EnvironmentDigest: facts.Environment.Digest[:16],
		Providers: providers, Diagnostics: projected, DirectAvailable: true,
	}
}
```

- [ ] **Step 4: Implement Core inspect and compile from cached facts**

```go
func (s *Service) getFacts(handle HostEvidenceHandle) (Facts, error) {
	return s.store.Get(handle)
}

func (s *Service) CoreInspect(ctx context.Context, input CoreInspectInput) (CoreInspectOutput, error) {
	facts, err := s.getFacts(input.HostEvidenceHandle); if err != nil { return CoreInspectOutput{}, err }
	decision, err := core.Classify(&input.Proposal, s.rules); if err != nil { return CoreInspectOutput{}, err }
	output := CoreInspectOutput{Classification: decision, HostSummary: secretFreeSummary(facts, nil)}
	if decision.RequestMode != classification.RequestModeWorkflow { return output, nil }
	result, err := core.Compile(compilationRequest(input.DeliverableID, input.InputDigest, 1, decision, facts, nil))
	if err != nil { return CoreInspectOutput{}, err }
	output.Compilation = &result
	return output, nil
}

func (s *Service) CoreCompile(ctx context.Context, input CoreCompileInput) (core.CompilationResult, error) {
	facts, err := s.getFacts(input.HostEvidenceHandle); if err != nil { return core.CompilationResult{}, err }
	decision, err := core.Classify(&input.Proposal, s.rules); if err != nil { return core.CompilationResult{}, err }
	return core.Compile(compilationRequest(input.DeliverableID, input.InputDigest, 1, decision, facts, &input.Selection))
}

func compilationRequest(deliverableID, inputDigest string, generation uint64, decision classification.ClassificationDecision, facts Facts, selection *core.Selection) core.CompilationRequest {
	return core.CompilationRequest{
		DeliverableID: deliverableID, InputDigest: inputDigest, Generation: generation,
		Classification: decision, Configuration: facts.Configuration,
		Resolutions: facts.Resolutions, Registry: facts.Registry, HostID: facts.Session.HostID,
		HostSessionDigest: facts.Session.Digest, HostEnvironmentReportDigest: facts.Environment.Digest,
		HostProviderInventoryDigest: facts.Inventory.Digest,
		HostTopologies: append([]execution.Topology{}, facts.Session.SupportedTopologies...),
		EnvironmentObservations: append([]execution.EnvironmentObservation{}, facts.Environment.Observations...),
		Selection: selection,
	}
}
```

`inputs.go` also owns strict decoding for the public MCP records. It uses
`json.Decoder.DisallowUnknownFields`, requires one JSON value followed only by
EOF, and translates unknown fields into `HOST_BRIDGE_PROTOCOL_MISMATCH`:

```go
func DecodeCoreCompileInput(raw []byte) (CoreCompileInput, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var input CoreCompileInput
	if err := decoder.Decode(&input); err != nil { return CoreCompileInput{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "unknown or malformed public field", err) }
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF { return CoreCompileInput{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "trailing JSON value", err) }
	return input, nil
}
```

`NewService` defensively clones `ServiceOptions.Rules` and `Authority`; callers
provide the same configured rules and authority ceiling used by the CLI and
Coordinator. `compilationRequest` copies only cached Host facts. Reject non-
WORKFLOW classification for compile; inspect reports DIRECT/BOUNDED without
calling the lifecycle compiler or creating a Bundle.

- [ ] **Step 5: Implement Coordinator exchange and fact-change preflight**

```go
func (s *Service) WorkflowExchange(ctx context.Context, input WorkflowExchangeInput) (coordinator.Result, error) {
	facts, err := s.getFacts(input.HostEvidenceHandle); if err != nil { return coordinator.Result{}, err }
	if err := validateCommandHostFacts(input.Command, facts); err != nil { return coordinator.Result{}, err }
	engine, err := coordinator.NewEngine(coordinator.Options{StateRoot: s.stateRoot, PhysicalProjectRoot: facts.Configuration.Record().ProjectRoot, Rules: s.rules, Configuration: facts.Configuration, Resolutions: facts.Resolutions, Registry: facts.Registry, Authority: s.authority})
	if err != nil { return coordinator.Result{}, err }
	if requiresPinnedPreflight(input.Command.Kind) {
		current, err := engine.Exchange(coordinator.Command{SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: input.Command.WorkflowID})
		if err != nil { return coordinator.Result{}, err }
		if err := compareActiveBundleFacts(current, facts); err != nil { return coordinator.Result{}, err }
	}
	return engine.Exchange(input.Command)
}
```

The helper contracts used above are part of `service.go` and are intentionally
small:

```go
func validateCommandHostFacts(command coordinator.Command, facts Facts) error {
	switch command.Kind {
	case coordinator.CommandStart:
		if command.Start == nil { return NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "START payload is missing", nil) }
		if !reflect.DeepEqual(command.Start.HostSession, facts.Session) || !reflect.DeepEqual(command.Start.Environment, facts.Environment) { return NewError("HOST_SESSION_CHANGED", "START facts differ from observation", nil) }
	case coordinator.CommandSwitch:
		if command.Switch == nil { return NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "SWITCH payload is missing", nil) }
		if !reflect.DeepEqual(command.Switch.HostSession, facts.Session) || !reflect.DeepEqual(command.Switch.Environment, facts.Environment) { return NewError("HOST_SESSION_CHANGED", "SWITCH facts differ from observation", nil) }
	}
	return nil
}

func requiresPinnedPreflight(kind coordinator.CommandKind) bool {
	return kind == coordinator.CommandInspect || kind == coordinator.CommandPrepare || kind == coordinator.CommandReceipt || kind == coordinator.CommandSwitch || kind == coordinator.CommandCancel
}

func compareActiveBundleFacts(result coordinator.Result, facts Facts) error {
	if result.Snapshot == nil || len(result.Snapshot.Bundles) == 0 { return nil }
	found := false
	for _, bundle := range result.Snapshot.Bundles {
		if bundle.Generation != result.Snapshot.ActiveGeneration { continue }
		found = true
		if bundle.HostSessionDigest != facts.FactDigests.Session || bundle.EnvironmentReportDigest != facts.FactDigests.Environment || bundle.ProviderInventoryDigest != facts.FactDigests.Inventory || bundle.Configuration.Digest != facts.FactDigests.Configuration || bundle.ResolutionDigest != facts.FactDigests.Resolution || bundle.RegistryDigest != facts.FactDigests.Registry {
			return NewError("HOST_SESSION_CHANGED", "active Bundle facts differ from current observation", nil)
		}
	}
	if !found { return NewError("HOST_SESSION_CHANGED", "active Bundle generation is missing", nil) }
	return nil
}
```

For `START` and `SWITCH`, command Session/Environment must be byte-for-byte
canonical equivalents of cached facts. For `PREPARE`, `RECEIPT`, and `CANCEL`,
inspect the current revision and compare the active Bundle's Session,
Inventory, Environment, Configuration, Resolution, and Registry digests before
the mutating exchange. `INSPECT` remains read-only and may report
`HOST_SESSION_CHANGED`. Never journal the evidence handle.

- [ ] **Step 6: Run service GREEN and Coordinator tests**

```bash
rtk gofmt -w internal/codexbridge/inputs.go internal/codexbridge/service.go internal/codexbridge/service_test.go
rtk go test ./internal/codexbridge -run 'Observe|Core|Workflow|Facts'
rtk go test ./internal/core ./internal/coordinator ./internal/registry
rtk go test -race ./internal/codexbridge ./internal/coordinator
```

Expected: PASS; Core compilation and Coordinator START pin identical fact
digests, and changed facts fail before a new revision is committed.

- [ ] **Step 7: Commit service integration**

```bash
rtk git add internal/codexbridge/inputs.go internal/codexbridge/service.go internal/codexbridge/service_test.go internal/coordinator/start_test.go
rtk git commit -m "feat: connect Codex Bridge to Core and Coordinator"
```

## Task 3: Register the stdio MCP server with closed schemas

**Files:**
- Create: `internal/codexbridge/mcp.go`
- Create: `internal/codexbridge/mcp_test.go`

- [ ] **Step 1: Write failing in-memory MCP tests**

```go
func TestMCPListsExactlyFourClosedTools(t *testing.T) {
	client := connectInMemoryMCP(t, newTestService(t))
	tools := collectTools(t, client)
	if got := toolNames(tools); !slices.Equal(got, []string{"core_compile", "core_inspect", "observe_current", "workflow_exchange"}) { t.Fatalf("tools=%v", got) }
	for _, tool := range tools {
		if tool.InputSchema["additionalProperties"] != false { t.Fatalf("open schema for %s: %#v", tool.Name, tool.InputSchema) }
	}
}

func TestMCPObserveAcceptsHookInjectedPrivateContextOnly(t *testing.T) {
	client := connectInMemoryMCP(t, newTestService(t))
	_, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "observe_current", Arguments: map[string]any{}})
	if Code(err) != "HOST_BRIDGE_CONTEXT_REQUIRED" { t.Fatalf("error=%v", err) }
}
```

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge -run 'MCP'
```

Expected: FAIL because the server is absent.

- [ ] **Step 3: Register tools through the official SDK**

```go
func NewMCPServer(service *Service, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "oaw-codex-bridge", Version: version}, nil)
	mcp.AddTool(server, closedTool("observe_current", "Observe the current Codex Host facts", observeSchema), service.observeTool)
	mcp.AddTool(server, closedTool("core_inspect", "Inspect verified Providers and Profile eligibility", inspectSchema), service.inspectTool)
	mcp.AddTool(server, closedTool("core_compile", "Compile one explicit Lifecycle Bundle", compileSchema), service.compileTool)
	mcp.AddTool(server, closedTool("workflow_exchange", "Exchange one Coordinator command", workflowSchema), service.workflowTool)
	return server
}

func ServeStdio(ctx context.Context, service *Service, version string) error {
	return NewMCPServer(service, version).Run(ctx, &mcp.StdioTransport{})
}
```

Each handler extracts `_oaw_host_context` from raw arguments before decoding
the closed public object. Only `observe_current` accepts that reserved value.
Later operations accept only the handle and public operation fields. Return
structured JSON plus one concise text summary; error results carry stable code,
layer, affected Provider/Profile IDs when known, Direct availability, recovery
action, and secret-free evidence digest. Use `ProjectDiagnostic` for every
error result; when the handle resolves, fill `EvidenceDigest` from
`Facts.FactDigests.Session` and otherwise leave it empty. Never copy an error
cause, absolute path, handle token, or raw Host metadata into the result.

- [ ] **Step 4: Verify schemas and operation effects**

Mark `observe_current`, `core_inspect`, and `core_compile` read-only through MCP
annotations. `workflow_exchange` must not be marked read-only because some
commands mutate Coordinator state. Do not request sampling, elicitation,
roots, model calls, or arbitrary server-side tools.

- [ ] **Step 5: Run MCP GREEN and full phase gates**

```bash
rtk gofmt -w internal/codexbridge/mcp.go internal/codexbridge/mcp_test.go
rtk go test ./internal/codexbridge -run 'MCP|Service'
rtk go test ./internal/cli ./internal/core ./internal/coordinator ./internal/registry
rtk go test -race ./internal/codexbridge ./internal/coordinator
rtk go vet ./internal/codexbridge/... ./internal/cli ./internal/core ./internal/coordinator
rtk git diff --check
```

Expected: PASS. No handler starts a Skill, Agent, Thread, turn, model process,
or shell command.

- [ ] **Step 6: Commit the MCP server**

```bash
rtk git add internal/codexbridge/mcp.go internal/codexbridge/mcp_test.go
rtk git commit -m "feat: expose Codex Host Bridge MCP operations"
```

## Task 4: Self-review authority boundaries

- [ ] **Step 1: Scan production code for forbidden execution paths**

```bash
rtk rg -n 'codex exec|thread/start|thread/resume|thread/fork|turn/start|turn/steer|plugin/list|private.*HOME|NATIVE_SUBAGENT|INLINE' internal/codexbridge internal/cli internal/coordinator
```

Expected: no production match. Negative tests and design documents are outside
this scan.

- [ ] **Step 2: Verify handle non-persistence**

```bash
rtk rg -n 'HostEvidenceHandle|host_evidence_handle' internal/coordinator internal/assets/schemas
```

Expected: no Coordinator record or public OAW schema stores the Bridge handle.
