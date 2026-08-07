# OAW Codex Host Bridge 04: Host Integration and Conformance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans in the current session to implement this plan task-by-task. This plan is locked to `CURRENT`; do not dispatch subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register a separate audited `oaw/codex-host` integration, prove its exact v1 features through the existing Host conformance contracts, and fail closed on incompatible Plugin, Hook, Bridge, Core, or Codex metadata capabilities.

**Architecture:** Keep `oaw/codex-policy` unchanged and add a second, explicit host-native record only after a deterministic Bridge transcript verifies `CURRENT`, Skill inventory, Environment reporting, and normalized Receipts. Version negotiation checks component protocol identifiers and probes the required stable App Server methods; a Codex version string is diagnostic evidence, never the sole capability proof.

**Tech Stack:** Go 1.26, existing Host v2 audit/conformance constructors, canonical JSON, embedded JSON assets, deterministic generation tests.

---

**Selected execution:** `CURRENT`. Do not dispatch subagents. Do not publish an intermediate phase commit.

**Depends on:** Plans 01-03.

**Produces:** Production-eligible `oaw/codex-host`, conformance evidence, and version negotiation APIs consumed by Plan 05 checks and Plan 06 dogfood.

## File Map

| Path | Responsibility |
| --- | --- |
| `internal/codexbridge/version.go` | Component versions, required metadata methods, and compatibility result. |
| `internal/codexbridge/version_test.go` | Method probes, mismatches, minimum baseline, and no-version-only promotion. |
| `internal/codexbridge/integration.go` | Reuse and verify the canonical `CodexHostManifest` constructor introduced in Plan 02. |
| `internal/codexbridge/integration_test.go` | Manifest feature, topology, and policy/native separation tests. |
| `internal/codexbridge/conformance.go` | Build a deterministic Bridge Host transcript from real service outputs. |
| `internal/codexbridge/conformance_test.go` | Verify only the declared Integration features. |
| `internal/assets/conformance/codex-host-v1.json` | Canonical secret-free conformance transcript artifact. |
| `internal/assets/audits/codex-host-v1.json` | Canonical audit references and content digests. |
| `internal/assets/generate/main.go` | Defined generator entry point used by the documented `go run` command. |
| `internal/assets/generate/codex_host.go` | Deterministically render the host-native Integration record into the built-in set. |
| `internal/assets/host-integrations.json` | Add `oaw/codex-host`; retain `oaw/codex-policy` unchanged. |
| `internal/assets/embed.go` | Embed conformance/audit evidence used by tests. |
| `internal/assets/embed_test.go` | Asset presence, digest, and separation tests. |
| `internal/host/builtin_test.go` | Built-in Integration canonicality and authority separation. |
| `internal/integration/codex_bridge_conformance_test.go` | Superpowers, Matt, ECC, user Provider, policy-only, and host-native integration tests. |

## Locked Host Integration

The new record has exactly this semantic content before canonical digests are
filled by the generator:

```go
host.Manifest{
	SchemaVersion: host.HostManifestSchemaV2,
	ManifestVersion: "1.0.0",
	HostID: "codex",
	ControlSurface: host.SurfaceHostNative,
	Protocols: []string{host.WorkflowProtocolV1},
	BindingKinds: []string{"skill"},
	SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
	Features: []host.Feature{
		host.FeatureEnvironmentReporting,
		host.FeatureNormalizedReceipts,
		host.FeatureProviderBindingInventory,
	},
}
```

It does not declare `SUBAGENT`, pause, cancellation, invocation
deduplication, Agent binding, Tool binding, Plugin inventory, sandbox control,
or approval control.

## Task 1: Add explicit version and metadata capability negotiation

**Files:**
- Create: `internal/codexbridge/version.go`
- Create: `internal/codexbridge/version_test.go`
- Modify: `internal/codexbridge/integration.go`
- Create: `internal/codexbridge/integration_test.go`
- Modify: `internal/codexbridge/appserver/client.go`
- Modify: `internal/codexbridge/appserver/client_test.go`

- [ ] **Step 1: Write failing negotiation tests**

```go
func TestNegotiateRequiresExactBridgeAndHookProtocols(t *testing.T) {
	input := compatibleVersions()
	input.HookContextSchema = "oaw.codex-hook-context/v2"
	if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" { t.Fatalf("error=%v", err) }
}

func TestNegotiateRejectsPluginAndPublicSchemaDrift(t *testing.T) {
	for _, mutate := range []func(*VersionEvidence){
		func(value *VersionEvidence) { value.PluginVersion = "1.1.0" },
		func(value *VersionEvidence) { value.InventorySchema = "oaw.host-binding-inventory/v3" },
		func(value *VersionEvidence) { value.BundleSchema = "oaw.lifecycle-bundle/v4" },
		func(value *VersionEvidence) { value.WorkflowSchema = "oaw.workflow-command/v2" },
	} {
		input := compatibleVersions()
		mutate(&input)
		if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" { t.Fatalf("input=%#v error=%v", input, err) }
	}
}

func TestNegotiateProbesMethodsInsteadOfTrustingCodexVersion(t *testing.T) {
	input := compatibleVersions()
	input.CodexVersion = "99.0.0"
	input.MetadataMethods = []string{"hooks/list", "config/read"}
	if _, err := Negotiate(input); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" { t.Fatalf("error=%v", err) }
}

func TestNegotiateKeepsOptionalMetadataFailuresPartial(t *testing.T) {
	input := compatibleVersions()
	input.MetadataMethods = []string{"skills/list"}
	result, err := Negotiate(input)
	if err != nil || !slices.Equal(result.MissingOptionalMethods, []string{"config/read", "hooks/list"}) { t.Fatalf("result=%#v err=%v", result, err) }
}

func TestNegotiateRejectsExperimentalPluginListAsRequirement(t *testing.T) {
	input := compatibleVersions()
	input.MetadataMethods = append(input.MetadataMethods, "plugin/list")
	result, err := Negotiate(input)
	if err != nil || slices.Contains(result.RequiredMethods, "plugin/list") { t.Fatalf("result=%#v err=%v", result, err) }
}

func TestCodexHostManifestDeclaresOnlyCurrentSkillEvidence(t *testing.T) {
	manifest, err := CodexHostManifest()
	if err != nil { t.Fatal(err) }
	if manifest.ControlSurface != host.SurfaceHostNative ||
		!slices.Equal(manifest.BindingKinds, []string{"skill"}) ||
		!slices.Equal(manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func compatibleVersions() VersionEvidence {
	return VersionEvidence{
		PluginVersion: BridgeIntegrationVersion, BridgeProtocol: BridgeProtocolVersion,
		HookContextSchema: HookContextSchemaV1, IntegrationVersion: BridgeIntegrationVersion,
		HostSessionSchema: host.HostSessionSchemaV2, InventorySchema: host.BindingInventorySchemaV2,
		EnvironmentSchema: host.HostEnvironmentReportSchemaV2, BundleSchema: LifecycleBundleSchemaV3,
		WorkflowSchema: coordinator.WorkflowCommandSchemaV1, CodexVersion: "codex-cli 0.146.1",
		MetadataMethods: []string{"skills/list", "hooks/list", "config/read"},
	}
}
```

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge -run 'Negotiate'
```

Expected: FAIL because negotiation is absent.

- [ ] **Step 3: Implement the closed version record**

```go
const MinimumCodexBaseline = "0.146.1"
const LifecycleBundleSchemaV3 = "oaw.lifecycle-bundle/v3"

type VersionEvidence struct {
	PluginVersion      string
	BridgeProtocol     string
	HookContextSchema  string
	IntegrationVersion string
	HostSessionSchema  string
	InventorySchema    string
	EnvironmentSchema  string
	BundleSchema       string
	WorkflowSchema     string
	CodexVersion       string
	MetadataMethods    []string
}

type Compatibility struct {
	Compatible             bool     `json:"compatible"`
	RequiredMethods        []string `json:"required_methods"`
	MissingOptionalMethods []string `json:"missing_optional_methods"`
	CodexVersion           string   `json:"codex_version"`
}

func Negotiate(value VersionEvidence) (Compatibility, error) {
	if value.PluginVersion != BridgeIntegrationVersion || value.BridgeProtocol != BridgeProtocolVersion ||
		value.HookContextSchema != HookContextSchemaV1 || value.IntegrationVersion != BridgeIntegrationVersion {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Bridge component versions differ", nil)
	}
	if value.HostSessionSchema != host.HostSessionSchemaV2 || value.InventorySchema != host.BindingInventorySchemaV2 ||
		value.EnvironmentSchema != host.HostEnvironmentReportSchemaV2 || value.BundleSchema != LifecycleBundleSchemaV3 ||
		value.WorkflowSchema != coordinator.WorkflowCommandSchemaV1 {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "public OAW schema versions differ", nil)
	}
	if compareCodexVersion(value.CodexVersion, MinimumCodexBaseline) < 0 {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Codex version is below the verified baseline", nil)
	}
	required := []string{"skills/list"}
	for _, method := range required {
		if !slices.Contains(value.MetadataMethods, method) { return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "required Codex metadata method is unavailable", nil) }
	}
	missingOptional := make([]string, 0, 2)
	for _, method := range []string{"config/read", "hooks/list"} {
		if !slices.Contains(value.MetadataMethods, method) { missingOptional = append(missingOptional, method) }
	}
	return Compatibility{Compatible: true, RequiredMethods: required, MissingOptionalMethods: missingOptional, CodexVersion: value.CodexVersion}, nil
}

func compareCodexVersion(value, minimum string) int {
	parse := func(raw string) ([3]int, bool) {
		raw = strings.TrimPrefix(strings.TrimSpace(raw), "codex-cli ")
		if strings.ContainsAny(raw, "+-") { return [3]int{}, false }
		parts := strings.Split(raw, ".")
		if len(parts) != 3 { return [3]int{}, false }
		var result [3]int
		for index, part := range parts {
			parsed, err := strconv.Atoi(part)
			if err != nil || parsed < 0 { return [3]int{}, false }
			result[index] = parsed
		}
		return result, true
	}
	left, leftOK := parse(value)
	right, rightOK := parse(minimum)
	if !leftOK || !rightOK { return -1 }
	for index := range left {
		if left[index] < right[index] { return -1 }
		if left[index] > right[index] { return 1 }
	}
	return 0
}
```

`compatibleVersions` fills every field with the constants in this block and
the active exported Host/Coordinator constants; it is defined in
`version_test.go`. Reject malformed and prerelease Codex version strings until
explicit compatibility evidence exists. A newer well-formed version remains
diagnostic evidence only: it cannot compensate for a missing required method.

- [ ] **Step 4: Probe capabilities during App Server initialization**

After initialization, make the three allowlisted calls and record a method as
available only when its request/response schema succeeds. An unsupported,
timed-out, or malformed `skills/list` maps to
`HOST_BRIDGE_PROTOCOL_MISMATCH` or `HOST_OBSERVATION_FAILED` and blocks
Provider authority. The same failure for diagnostic `hooks/list` or
`config/read` records the method in `MissingOptionalMethods`, emits
`HOST_OBSERVATION_PARTIAL`, and projects the affected dispositions as
`unknown`; it does not fabricate evidence or make the whole Bridge
incompatible.

- [ ] **Step 5: Run GREEN**

```bash
rtk gofmt -w internal/codexbridge/version.go internal/codexbridge/version_test.go internal/codexbridge/integration.go internal/codexbridge/integration_test.go internal/codexbridge/appserver/client.go internal/codexbridge/appserver/client_test.go
rtk go test ./internal/codexbridge/... -run 'Negotiate|Capability|Method'
rtk go vet ./internal/codexbridge/...
```

Expected: PASS.

- [ ] **Step 6: Commit version negotiation**

```bash
rtk git add internal/codexbridge/version.go internal/codexbridge/version_test.go internal/codexbridge/integration.go internal/codexbridge/integration_test.go internal/codexbridge/appserver/client.go internal/codexbridge/appserver/client_test.go
rtk git commit -m "feat: negotiate Codex Bridge capabilities"
```

## Task 2: Produce a real Bridge conformance transcript

**Files:**
- Create: `internal/codexbridge/conformance.go`
- Create: `internal/codexbridge/conformance_test.go`
- Create: `internal/assets/conformance/codex-host-v1.json`

- [ ] **Step 1: Write the failing conformance test**

```go
func TestCodexBridgeTranscriptVerifiesDeclaredFeatures(t *testing.T) {
	service, facts := observedConformanceService(t)
	receipt := completedCurrentReceipt(t, facts.Session, facts.Environment)
	transcript, err := BuildConformanceTranscript(facts, []host.InvocationReceipt{receipt})
	if err != nil { t.Fatal(err) }
	manifest, err := CodexHostManifest()
	if err != nil { t.Fatal(err) }
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil { t.Fatal(err) }
	want := []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}
	if !slices.Equal(report.VerifiedFeatures, want) || len(report.Diagnostics) != 0 { t.Fatalf("report=%#v", report) }
	_ = service
}
```

The fixture must pass through the actual `Service.ObserveCurrent` path with a
fake App Server transcript and real Core resolution. It may not call
`hosttest.ObserveProviderBindings`, because this phase is proving the new
Bridge binding algorithm.

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/codexbridge -run 'ConformanceTranscript'
```

Expected: FAIL until the builder exists.

- [ ] **Step 3: Build and canonicalize the transcript**

```go
func BuildConformanceTranscript(facts Facts, receipts []host.InvocationReceipt) (host.ConformanceTranscript, error) {
	return host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV2,
		Session: facts.Session,
		Inventory: facts.Inventory,
		EnvironmentReports: []host.EnvironmentReport{facts.Environment},
		Receipts: append([]host.InvocationReceipt{}, receipts...),
		Invocations: []host.InvocationRecord{},
	})
}
```

Create one canonical completed `CURRENT` Receipt with `ContextShared`, exact
Session/Environment/Dispatch digests, and a secret-free evidence reference.
Serialize the normalized transcript into
`internal/assets/conformance/codex-host-v1.json`; a test must rebuild it and
compare bytes so stale digests fail CI. The test uses the explicit helpers
`observedConformanceService`, `completedCurrentReceipt`, and
`canonicalTranscriptBytes`; each is defined in `conformance_test.go` and uses
the fake App Server transcript plus `host.NewInvocationReceipt`, never an
undefined fixture or a live Codex process.

- [ ] **Step 4: Run GREEN and record the transcript digest**

```bash
rtk gofmt -w internal/codexbridge/conformance.go internal/codexbridge/conformance_test.go
rtk go test ./internal/codexbridge -run 'Conformance'
rtk git diff --check internal/assets/conformance/codex-host-v1.json
```

Expected: PASS; the transcript contains no absolute user path, handle,
credential, raw Hook command, or transcript text.

- [ ] **Step 5: Commit conformance evidence**

```bash
rtk git add internal/codexbridge/conformance.go internal/codexbridge/conformance_test.go internal/assets/conformance/codex-host-v1.json
rtk git commit -m "test: prove Codex Bridge Host conformance"
```

## Task 3: Add the audited built-in Host Integration

**Files:**
- Create: `internal/assets/audits/codex-host-v1.json`
- Create: `internal/assets/generate/main.go`
- Create: `internal/assets/generate/codex_host.go`
- Modify: `internal/assets/host-integrations.json`
- Modify: `internal/assets/embed.go`
- Modify: `internal/assets/embed_test.go`
- Modify: `internal/host/builtin_test.go`

- [ ] **Step 1: Write failing separation and digest tests**

```go
func TestBuiltinCodexPolicyAndHostRemainSeparate(t *testing.T) {
	integrations, err := host.LoadBuiltinIntegrations(FS())
	if err != nil { t.Fatal(err) }
	policy := integrationByID(t, integrations, "oaw/codex-policy")
	native := integrationByID(t, integrations, "oaw/codex-host")
	if policy.Manifest.ControlSurface != host.SurfacePolicy || native.Manifest.ControlSurface != host.SurfaceHostNative { t.Fatalf("policy=%#v native=%#v", policy, native) }
	if !slices.Equal(native.Manifest.BindingKinds, []string{"skill"}) || !slices.Equal(native.Manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) { t.Fatalf("native manifest=%#v", native.Manifest) }
}
```

- [ ] **Step 2: Run RED**

```bash
rtk go test ./internal/assets ./internal/host -run 'CodexPolicyAndHost|Builtin'
```

Expected: FAIL because `oaw/codex-host` is absent.

- [ ] **Step 3: Add canonical audit evidence and deterministic generator**

The audit asset contains a passed status and references to the approved design,
security tests, Hook matcher tests, and conformance transcript, each with its
SHA-256 content digest. The generator must:

```go
func buildCodexHostIntegration(transcript host.ConformanceTranscript, audit host.AuditEvidence) (host.IntegrationRecord, error) {
	manifest, err := codexbridge.CodexHostManifest()
	if err != nil { return host.IntegrationRecord{}, err }
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil { return host.IntegrationRecord{}, err }
	return host.NewIntegration(host.IntegrationRecord{SchemaVersion: host.HostIntegrationSchemaV2, IntegrationVersion: codexbridge.BridgeIntegrationVersion, ID: codexbridge.BridgeIntegrationID, Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit, Conformance: &report})
}
```

`internal/assets/generate/main.go` is a real `package main` entry point:

```go
func main() {
	root, err := os.Getwd()
	if err != nil { log.Fatal(err) }
	if err := generateCodexHost(root); err != nil { log.Fatal(err) }
}
```

`generateCodexHost(root)` is implemented in `codex_host.go`; it reads the
canonical transcript and audit assets under `root`, calls
`buildCodexHostIntegration`, replaces only an exact `oaw/codex-host` record in
the decoded Integration Set, sorts by ID, and writes canonical JSON with a
trailing newline. It never changes `oaw/codex-policy`.

Decode the existing Integration Set, replace only an existing exact
`oaw/codex-host` record or append it, sort by ID, and write canonical indented
JSON. Never replace or upgrade `oaw/codex-policy`.

- [ ] **Step 4: Generate and verify the built-in asset**

```bash
rtk go run ./internal/assets/generate
rtk go test ./internal/assets ./internal/host -run 'Codex|Builtin|Integration'
rtk git diff --check internal/assets/host-integrations.json internal/assets/audits/codex-host-v1.json
```

Expected: PASS; the generated record has passed Audit, a conformant report,
and exactly three verified features.

- [ ] **Step 5: Commit the Host Integration**

```bash
rtk git add internal/assets/audits internal/assets/generate internal/assets/host-integrations.json internal/assets/embed.go internal/assets/embed_test.go internal/host/builtin_test.go
rtk git commit -m "feat: register audited Codex Host integration"
```

## Task 4: Prove Provider neutrality and Integration authority separation

**Files:**
- Create: `internal/integration/codex_bridge_conformance_test.go`
- Modify: `internal/integration/host_configuration_test.go`
- Modify: `internal/integration/host_conformance_test.go`

- [ ] **Step 1: Add failing Provider-family matrix tests**

```go
func TestCodexBridgeUsesOneBindingAlgorithmForEveryProvider(t *testing.T) {
	cases := []struct{ provider, skill string }{
		{"oaw/superpowers", "superpowers:writing-plans"},
		{"oaw/matt", "tdd"},
		{"oaw/ecc", "tdd-workflow"},
		{"acme/custom", "acme:delivery"},
	}
	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) { assertExactSkillBinding(t, tc.provider, tc.skill) })
	}
}
```

Use the exact Binding reference declared by each fixture Descriptor. Include
an ECC-FULL fixture whose complete Capability set verifies only when every
required ECC Skill is enabled; missing one Skill leaves the Profile
ineligible without demoting ECC to an add-on.

- [ ] **Step 2: Add policy/native non-promotion tests**

```go
func TestCodexPolicyInstallationNeverClaimsHostNative(t *testing.T) {
	snapshot := loadPolicyOnlyCodexSnapshot(t)
	if integrationCanSupplyInventory(snapshot, "oaw/codex-policy") { t.Fatal("policy integration supplied Host authority") }
}
```

- [ ] **Step 3: Run RED then implement fixture helpers**

```bash
rtk go test ./internal/integration -run 'CodexBridge|PolicyInstallation|Provider'
```

Expected before helpers: FAIL. Add only test fixture builders and production
selection checks required to keep policy and host-native records separate;
do not add Provider-specific production branches.

- [ ] **Step 4: Run full conformance GREEN**

```bash
rtk gofmt -w internal/integration/codex_bridge_conformance_test.go internal/integration/host_configuration_test.go internal/integration/host_conformance_test.go
rtk go test ./internal/host ./internal/assets ./internal/integration
rtk go test -race ./internal/codexbridge ./internal/host ./internal/integration
rtk git diff --check
```

Expected: PASS. Superpowers, Matt, ECC, and user Providers use the same
Descriptor, Candidate, path, Skill, and Inventory path.

- [ ] **Step 5: Commit integration conformance**

```bash
rtk git add internal/integration/codex_bridge_conformance_test.go internal/integration/host_configuration_test.go internal/integration/host_conformance_test.go
rtk git commit -m "test: verify Codex Host integration authority"
```

## Task 5: Self-review production claims

- [ ] **Step 1: Inspect the new Integration projection**

```bash
rtk go run ./cmd/oaw catalog validate --format json
rtk go run ./cmd/oaw catalog list providers --format json
rtk rg -n 'oaw/codex-host|oaw/codex-policy|SUBAGENT|"agent"|"tool"' internal/assets/host-integrations.json
```

Expected: catalog validation passes; `oaw/codex-host` is host-native and
CURRENT/skill-only; `oaw/codex-policy` remains policy-only.

- [ ] **Step 2: Run the phase gate**

```bash
rtk go test ./internal/codexbridge/... ./internal/host ./internal/assets ./internal/integration
rtk go test -race ./internal/codexbridge/... ./internal/host ./internal/integration
rtk git diff --check
```

Expected: PASS.
