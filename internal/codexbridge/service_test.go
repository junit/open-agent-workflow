package codexbridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestObserveCurrentCreatesHandleFromInjectedContext(t *testing.T) {
	service := newTestService(t)
	result, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if result.HostEvidenceHandle.Token == "" || result.HostSummary.SessionDigest == "" || result.HostSummary.InventoryDigest == "" || result.HostSummary.EnvironmentDigest == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestObserveCurrentPropagatesOptionalMetadataDiagnostics(t *testing.T) {
	service := newTestService(t)
	service.observer.(*fakeObserver).SetDiagnostics([]appserver.ObservationDiagnostic{{
		Code: "HOST_OBSERVATION_PARTIAL", Detail: "hooks/list unavailable",
	}})
	result, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(result.HostSummary.Diagnostics, "HOST_OBSERVATION_PARTIAL") {
		t.Fatalf("summary = %#v", result.HostSummary)
	}
}

func TestCoreCompileCannotReplaceCachedHostFacts(t *testing.T) {
	raw, err := json.Marshal(CoreCompileInput{})
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["host_session"] = map[string]any{"host_id": "forged"}
	forged, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCoreCompileInput(forged); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
}

func TestCoreInspectAndCompileUseVerifiedCurrentFacts(t *testing.T) {
	service := newTestService(t)
	installUserProvider(t, service)
	installProviderSkills(t, service, "oaw/superpowers", ".codex/plugins/superpowers", "skills/using-superpowers/SKILL.md")
	installProviderSkills(t, service, "acme/suite", ".codex/plugins/acme", "marker.txt")

	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	for _, providerID := range []string{"oaw/superpowers", "acme/suite"} {
		if state := providerSummaryState(t, observed.HostSummary, providerID); state != registry.Verified {
			t.Fatalf("Provider %s state = %s", providerID, state)
		}
	}
	input := CoreInspectInput{
		HostEvidenceHandle: observed.HostEvidenceHandle, DeliverableID: "bridge-service-test",
		InputDigest: testDigest("input"), Proposal: workflowProposal(),
	}
	inspected, err := service.CoreInspect(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Classification.RequestMode != classification.RequestModeWorkflow || inspected.Compilation == nil {
		t.Fatalf("inspection = %#v", inspected)
	}
	if !profileEligibility(t, inspected.Compilation, "SP-FULL").Eligible {
		t.Fatalf("SP-FULL = %#v", profileEligibility(t, inspected.Compilation, "SP-FULL"))
	}
	for _, profile := range []string{"MATT-FULL", "ECC-FULL"} {
		if profileEligibility(t, inspected.Compilation, profile).Eligible {
			t.Fatalf("%s unexpectedly eligible", profile)
		}
	}
	compiled, err := service.CoreCompile(context.Background(), CoreCompileInput{
		HostEvidenceHandle: observed.HostEvidenceHandle, DeliverableID: input.DeliverableID,
		InputDigest: input.InputDigest, Proposal: input.Proposal,
		Selection: core.Selection{
			Profile: "SP-FULL", ProfileSource: core.SelectionUser, Topology: execution.TopologyCurrent,
			TopologySource: core.SelectionHostOnlyOption, AddOns: []string{}, Bindings: []profile.ProfileBinding{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Bundle == nil || compiled.Bundle.Selection.Profile != "SP-FULL" || compiled.Bundle.ProviderInventoryDigest == "" {
		t.Fatalf("compilation = %#v", compiled)
	}
}

func TestWorkflowExchangeRejectsChangedPinnedFactsBeforeMutation(t *testing.T) {
	service, handle, cancel := startedWorkflow(t)
	service.observer.(*fakeObserver).SetSandboxDisposition("unknown")
	changed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	if changed.HostEvidenceHandle.Token == handle.Token {
		t.Fatal("changed observation reused the old handle")
	}
	if _, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: changed.HostEvidenceHandle, Command: cancel,
	}); Code(err) != "HOST_SESSION_CHANGED" {
		t.Fatalf("error = %v", err)
	}
	inspected, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{
		HostEvidenceHandle: handle,
		Command: coordinator.Command{
			SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: cancel.WorkflowID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Revision != 1 {
		t.Fatalf("rejected changed facts committed revision %d", inspected.Revision)
	}
}

type fakeObserver struct {
	mu          sync.Mutex
	metadata    appserver.MetadataObservation
	diagnostics []appserver.ObservationDiagnostic
}

func (observer *fakeObserver) SetDiagnostics(values []appserver.ObservationDiagnostic) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.diagnostics = append([]appserver.ObservationDiagnostic{}, values...)
}

func (observer *fakeObserver) AddSkill(value appserver.SkillMetadata) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.metadata.Skills.Skills = append(observer.metadata.Skills.Skills, value)
}

func (observer *fakeObserver) SetSandboxDisposition(value string) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.metadata.Config.SandboxDisposition = value
}

func (observer *fakeObserver) Observe(_ context.Context, cwd string) (appserver.MetadataObservation, error) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	value := observer.metadata
	value.Skills.CWD = cwd
	value.Hooks.CWD = cwd
	value.Diagnostics = append([]appserver.ObservationDiagnostic{}, observer.diagnostics...)
	return value, nil
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	projectRoot := t.TempDir()
	observer := &fakeObserver{metadata: appserver.MetadataObservation{
		Skills: appserver.SkillsEntry{Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{}},
		Hooks:  appserver.HooksEntry{Errors: []appserver.MetadataError{}, Warnings: []string{}, Hooks: []appserver.HookMetadata{}},
		Config: appserver.ConfigProjection{
			CWDObserved: true, SandboxDisposition: "host-configured", MCPDisposition: "host-configured",
			HookDisposition: "host-configured", ApprovalDisposition: "host-configured",
		},
		Methods: []string{"config/read", "hooks/list", "skills/list"}, CodexVersion: "codex-cli/test",
	}}
	service, err := NewService(ServiceOptions{
		Observer: observer, Store: NewEvidenceStore(CacheOptions{MaximumEntries: 8}), StateRoot: t.TempDir(),
		ProjectRoot: projectRoot, UserConfigRoot: t.TempDir(), UserHome: t.TempDir(), BridgeVersion: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func testHookContext(sessionID, cwd string) HookContext {
	return HookContext{
		SchemaVersion: HookContextSchemaV1, BridgeProtocolVersion: BridgeProtocolVersion,
		SessionID: sessionID, TurnID: "turn-1", ToolUseID: "tool-1", CWD: cwd,
		Model: "gpt-test", PermissionMode: "workspace-write",
	}
}

func installUserProvider(t *testing.T, service *Service) {
	t.Helper()
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV3, DescriptorVersion: "3.0.0", ID: "acme/suite", DisplayName: "Acme Suite",
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", Distribution: "acme", Kind: "path-exists",
			Root: "user-home", CandidatePath: ".codex/plugins/acme", EvidencePath: "marker.txt",
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "review", InputSchema: "in", OutcomeSchema: "out", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			RequestModes: []catalog.RequestMode{catalog.RequestModeBounded, catalog.RequestModeWorkflow}, Responsibilities: []string{"review"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, DelegationAllowList: []string{},
			HostBindings: []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "acme:review", Topologies: []execution.Topology{execution.TopologyCurrent}}},
		}},
	}
	providerRoot := filepath.Join(service.userConfigRoot, "providers")
	if err := os.MkdirAll(providerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providerRoot, "acme.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	contents := "schema_version = \"oaw.user-config/v3\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.json\"\n"
	if err := os.WriteFile(filepath.Join(service.userConfigRoot, "config.toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func installProviderSkills(t *testing.T, service *Service, providerID, relativeRoot, evidencePath string) {
	t.Helper()
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: service.userConfigRoot, ProjectRoot: service.projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	var descriptor catalog.ProviderDescriptorRecord
	for _, candidate := range snapshot.Catalog().Providers() {
		if candidate.ID == providerID {
			descriptor = candidate
			break
		}
	}
	if descriptor.ID == "" {
		t.Fatalf("Provider %s not configured", providerID)
	}
	root := filepath.Join(service.userHome, filepath.FromSlash(relativeRoot))
	writeServiceFixtureFile(t, filepath.Join(root, filepath.FromSlash(evidencePath)), "provider-evidence")
	seen := make(map[string]struct{})
	for _, capability := range descriptor.Capabilities {
		for _, binding := range capability.HostBindings {
			if binding.Host != "codex" || binding.Kind != "skill" {
				continue
			}
			if _, found := seen[binding.Reference]; found {
				continue
			}
			seen[binding.Reference] = struct{}{}
			name := binding.Reference
			path := filepath.Join(root, "observed-skills", name, "SKILL.md")
			writeServiceFixtureFile(t, path, "---\nname: "+name+"\n---\n")
			service.observer.(*fakeObserver).AddSkill(appserver.SkillMetadata{Name: name, Enabled: true, Path: path, Scope: "user"})
		}
	}
}

func writeServiceFixtureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func providerSummaryState(t *testing.T, summary HostSummary, providerID string) registry.ProviderState {
	t.Helper()
	for _, provider := range summary.Providers {
		if provider.ProviderID == providerID {
			return provider.State
		}
	}
	t.Fatalf("Provider %s missing from %#v", providerID, summary.Providers)
	return ""
}

func profileEligibility(t *testing.T, result *core.CompilationResult, profile string) core.ProfileEligibility {
	t.Helper()
	for _, value := range result.EligibleProfiles {
		if value.Profile == profile {
			return value
		}
	}
	t.Fatalf("Profile %s missing from %#v", profile, result.EligibleProfiles)
	return core.ProfileEligibility{}
}

func workflowProposal() classification.ClassificationProposal {
	return classification.ClassificationProposal{SchemaVersion: classification.ProposalSchemaV1}
}

func testDigest(value string) string {
	return canonicaljson.DigestBytes([]byte(value))
}

func startedWorkflow(t *testing.T) (*Service, HostEvidenceHandle, coordinator.Command) {
	t.Helper()
	service := newTestService(t)
	installProviderSkills(t, service, "oaw/superpowers", ".codex/plugins/superpowers", "skills/using-superpowers/SKILL.md")
	observed, err := service.ObserveCurrent(context.Background(), ObserveCurrentInput{}, testHookContext("session-1", service.projectRoot))
	if err != nil {
		t.Fatal(err)
	}
	facts, err := service.getFacts(observed.HostEvidenceHandle)
	if err != nil {
		t.Fatal(err)
	}
	start := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: "bridge-service-start",
		Start: &coordinator.StartInput{
			RequestID: "request-1", DeliverableID: "bridge-service-workflow", InputDigest: testDigest("workflow-input"),
			Proposal: workflowProposal(), Selection: core.Selection{
				Profile: "SP-FULL", ProfileSource: core.SelectionUser, Topology: execution.TopologyCurrent,
				TopologySource: core.SelectionHostOnlyOption, AddOns: []string{}, Bindings: []profile.ProfileBinding{},
			},
			HostSession: facts.Session, Environment: facts.Environment,
		},
	}
	started, err := service.WorkflowExchange(context.Background(), WorkflowExchangeInput{HostEvidenceHandle: observed.HostEvidenceHandle, Command: start})
	if err != nil {
		t.Fatal(err)
	}
	return service, observed.HostEvidenceHandle, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandCancel,
		MessageID: "message-cancel", IdempotencyKey: "bridge-service-cancel", WorkflowID: started.WorkflowID,
		ExpectedRevision: started.Revision, Cancel: &coordinator.CancelInput{Reason: "test cancellation", InvocationTerminal: true},
	}
}
