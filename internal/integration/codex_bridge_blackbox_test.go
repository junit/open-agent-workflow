package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/cli"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

func TestCodexBridgeCurrentWorkflowTranscript(t *testing.T) {
	fixture := newCodexBridgeFixture(t)
	observed := callObserveThroughMCP(t, fixture)
	inspection := callInspectThroughMCP(t, fixture, observed.HostEvidenceHandle)
	if inspection.Compilation == nil || len(inspection.Compilation.EligibleProfiles) == 0 || inspection.HostSummary.SessionDigest == "" {
		t.Fatalf("inspection=%#v", inspection)
	}
	compiled := callCompileThroughMCP(t, fixture, observed.HostEvidenceHandle, explicitCurrentSelection(t, inspection))
	started := callWorkflowStartThroughMCP(t, fixture, observed.HostEvidenceHandle, compiled)
	if started.Snapshot == nil || len(started.Snapshot.Bundles) != 1 ||
		started.Snapshot.Bundles[0].HostSessionDigest[:16] != inspection.HostSummary.SessionDigest {
		t.Fatalf("started=%#v", started)
	}
}

func TestDirectPathDoesNotRequireBridge(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := cli.RunWithInput([]string{"catalog", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "HOST_BRIDGE") {
		t.Fatalf("direct path unexpectedly consulted Bridge: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

type codexBridgeFixture struct {
	client        *mcp.ClientSession
	hostContext   codexbridge.HookContext
	proposal      classification.ClassificationProposal
	deliverableID string
	inputDigest   string
}

type codexBridgeObserver struct {
	metadata appserver.MetadataObservation
}

func (observer codexBridgeObserver) Observe(_ context.Context, cwd string) (appserver.MetadataObservation, error) {
	value := observer.metadata
	value.Skills.CWD = cwd
	value.Skills.Errors = append([]appserver.MetadataError{}, value.Skills.Errors...)
	value.Skills.Skills = append([]appserver.SkillMetadata{}, value.Skills.Skills...)
	value.Hooks.CWD = cwd
	value.Hooks.Errors = append([]appserver.MetadataError{}, value.Hooks.Errors...)
	value.Hooks.Warnings = append([]string{}, value.Hooks.Warnings...)
	value.Hooks.Hooks = append([]appserver.HookMetadata{}, value.Hooks.Hooks...)
	value.Methods = append([]string{}, value.Methods...)
	value.Diagnostics = append([]appserver.ObservationDiagnostic{}, value.Diagnostics...)
	return value, nil
}

func newCodexBridgeFixture(t *testing.T) codexBridgeFixture {
	t.Helper()
	projectRoot := t.TempDir()
	userHome := t.TempDir()
	userConfigRoot := t.TempDir()
	metadata := appserver.MetadataObservation{
		Skills: appserver.SkillsEntry{Errors: []appserver.MetadataError{}, Skills: installUserDefinedSkillFixture(t, userHome, userConfigRoot)},
		Hooks:  appserver.HooksEntry{Errors: []appserver.MetadataError{}, Warnings: []string{}, Hooks: []appserver.HookMetadata{}},
		Config: appserver.ConfigProjection{
			CWDObserved: true, SandboxDisposition: "host-configured", MCPDisposition: "host-configured",
			HookDisposition: "host-configured", ApprovalDisposition: "host-configured",
		},
		Methods: []string{"config/read", "hooks/list", "skills/list"}, CodexVersion: "codex-cli/0.146.1",
	}
	hostContext := codexbridge.HookContext{
		SchemaVersion: codexbridge.HookContextSchemaV2, BridgeProtocolVersion: codexbridge.BridgeProtocolVersion,
		SessionID: "bridge-blackbox-session", TurnID: "turn-1", ToolUseID: "tool-1", CWD: projectRoot,
		Model: "gpt-test", PermissionMode: "workspace-write",
	}
	observer := codexBridgeObserver{metadata: metadata}
	service, err := codexbridge.NewService(codexbridge.ServiceOptions{
		Observer: observer, Store: codexbridge.NewEvidenceStore(codexbridge.CacheOptions{MaximumEntries: 8}),
		StateRoot: t.TempDir(), ProjectRoot: projectRoot, UserConfigRoot: userConfigRoot,
		UserHome: userHome, BridgeVersion: "1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	return codexBridgeFixture{
		client: connectCodexBridgeMCP(t, service), hostContext: hostContext,
		proposal:      classification.ClassificationProposal{SchemaVersion: classification.ProposalSchemaV1},
		deliverableID: "codex-bridge-blackbox", inputDigest: canonicaljson.DigestBytes([]byte("codex-bridge-blackbox-input")),
	}
}

func installUserDefinedSkillFixture(t *testing.T, userHome, userConfigRoot string) []appserver.SkillMetadata {
	t.Helper()
	installRoot := filepath.Join(userHome, ".codex", "plugins", "acme")
	skillRoot := filepath.Join(installRoot, "skills", "delivery")
	skillPath := filepath.Join(skillRoot, "SKILL.md")
	writeCodexBridgeFixture(t, skillPath, "---\nname: acme:delivery\n---\n")
	bindingTree, err := integrity.DigestTree(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	distributionTree, err := integrity.DigestTree(installRoot)
	if err != nil {
		t.Fatal(err)
	}

	definitions := catalog.CanonicalSlots()
	claims := make([]catalog.ResponsibilityClaim, 0, len(definitions))
	stageSpan := make([]catalog.SlotID, 0, len(definitions))
	for _, definition := range definitions {
		claims = append(claims, catalog.ResponsibilityClaim{
			Namespace: catalog.OwnershipStage, Name: string(definition.ID), SlotID: definition.ID, OutcomeOwner: true,
		})
		stageSpan = append(stageSpan, definition.ID)
	}
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "acme/suite", DisplayName: "Acme Suite",
		Distributions: []catalog.DistributionRecord{{
			ID: "acme", SourceURI: "https://example.test/acme/suite", Revision: strings.Repeat("a", 40), TreeDigest: distributionTree.RootDigest,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "acme", Kind: "path-exists",
			Root: "user-home", CandidatePath: ".codex/plugins/acme", EvidencePath: "skills/delivery/SKILL.md",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-delivery", DistributionID: "acme", ContentRoot: "skills/delivery", InstallRoot: "skills/delivery", TreeDigest: bindingTree.RootDigest,
			Host: "codex", Surface: "codex-plugin", Kind: catalog.BindingSkill, Reference: "acme:delivery", Invocation: catalog.InvocationModel,
			Responsibilities: claims, InputArtifact: "oaw.workflow-artifact/v1", OutputArtifact: "oaw.workflow-artifact/v1",
			MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project-worktree"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: catalog.DelegationRequirements{}, StageSpan: stageSpan,
			InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "delivery", InputSchema: "oaw.capability-input/v1", OutcomeSchema: "oaw.capability-outcome/v1",
			RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"codex-delivery"},
		}},
	}
	available, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	var recipe catalog.ProfileRecipeRecord
	for _, candidate := range available.Recipes() {
		if candidate.ID == "oaw/delivery" {
			recipe = candidate
			break
		}
	}
	if recipe.ID == "" {
		t.Fatal("built-in delivery Recipe missing")
	}
	recipe.ID = "acme/current-delivery"
	recipe.DisplayName = "Acme Current Delivery"
	recipe.Family = "user-defined"
	recipe.Template = ""
	recipe.AddOns = []catalog.AddOnRecord{}
	recipe.IncidentRoutes = []catalog.IncidentRoute{}
	recipe.Overlays = []catalog.OverlayRecord{}
	for index := range recipe.Slots {
		slotID := recipe.Slots[index].SlotID
		stepID := "acme-" + string(slotID)
		recipe.Slots[index].Pipeline = []catalog.PipelineStep{{
			ID: stepID, Selector: catalog.BindingSelector{ProviderID: descriptor.ID, BindingID: "codex-delivery"}, StageSpan: []catalog.SlotID{slotID},
			RequiredInputArtifact: "oaw.workflow-artifact/v1", ProducedOutputArtifact: "oaw.workflow-artifact/v1",
		}}
		recipe.Slots[index].OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: stepID}
		recipe.Slots[index].HostAction = nil
	}

	providerPath := filepath.Join(userConfigRoot, "providers", "acme.json")
	recipePath := filepath.Join(userConfigRoot, "recipes", "acme.json")
	descriptorJSON, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	recipeJSON, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	writeCodexBridgeFixture(t, providerPath, string(descriptorJSON))
	writeCodexBridgeFixture(t, recipePath, string(recipeJSON))
	writeCodexBridgeFixture(t, filepath.Join(userConfigRoot, "config.toml"),
		"schema_version = \"oaw.user-config/v3\"\n"+
			"[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.json\"\n"+
			"[[profile_recipes]]\nid = \"acme/current-delivery\"\npath = \"recipes/acme.json\"\n")
	return []appserver.SkillMetadata{{Name: "acme:delivery", Enabled: true, Path: skillPath, Scope: "user"}}
}

func writeCodexBridgeFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func connectCodexBridgeMCP(t *testing.T, service *codexbridge.Service) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- codexbridge.NewMCPServer(service, "test").Run(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "bridge-blackbox-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		<-done
	})
	return session
}

func callObserveThroughMCP(t *testing.T, fixture codexBridgeFixture) codexbridge.ObserveCurrentOutput {
	t.Helper()
	return callCodexBridgeTool[codexbridge.ObserveCurrentOutput](t, fixture.client, "observe_current", map[string]any{
		"_oaw_host_context": fixture.hostContext,
	})
}

func callInspectThroughMCP(t *testing.T, fixture codexBridgeFixture, handle codexbridge.HostEvidenceHandle) codexbridge.CoreInspectOutput {
	t.Helper()
	return callCodexBridgeTool[codexbridge.CoreInspectOutput](t, fixture.client, "core_inspect", codexbridge.CoreInspectInput{
		HostEvidenceHandle: handle, DeliverableID: fixture.deliverableID,
		InputDigest: fixture.inputDigest, Proposal: fixture.proposal,
	})
}

func callCompileThroughMCP(
	t *testing.T,
	fixture codexBridgeFixture,
	handle codexbridge.HostEvidenceHandle,
	selection core.Selection,
) core.LifecycleBundle {
	t.Helper()
	result := callCodexBridgeTool[core.LifecycleBundle](t, fixture.client, "core_compile", codexbridge.CoreCompileInput{
		HostEvidenceHandle: handle, DeliverableID: fixture.deliverableID,
		InputDigest: fixture.inputDigest, Proposal: fixture.proposal, Selection: selection,
	})
	if result.SchemaVersion != core.LifecycleBundleSchemaV4 {
		t.Fatalf("compiled=%#v", result)
	}
	return result
}

func explicitCurrentSelection(t *testing.T, inspection codexbridge.CoreInspectOutput) core.Selection {
	t.Helper()
	for _, candidate := range inspection.Compilation.EligibleProfiles {
		if candidate.Profile == core.UserDefinedProfile && candidate.Eligible && candidate.Topology == execution.TopologyCurrent {
			selection := candidate.Preview.Selection
			selection.ProfileSource = core.SelectionUser
			selection.TopologySource = core.SelectionUser
			return selection
		}
	}
	t.Fatalf("USER-DEFINED CURRENT unavailable: %#v", inspection.Compilation.EligibleProfiles)
	return core.Selection{}
}

func callWorkflowStartThroughMCP(
	t *testing.T,
	fixture codexBridgeFixture,
	handle codexbridge.HostEvidenceHandle,
	compiled core.LifecycleBundle,
) coordinator.Result {
	t.Helper()
	command := codexbridge.WorkflowCommandInput{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: "codex-bridge-blackbox-start",
		Start: &codexbridge.WorkflowStartInput{
			RequestID: "request-codex-bridge-blackbox", DeliverableID: fixture.deliverableID,
			InputDigest: fixture.inputDigest, Proposal: fixture.proposal, Selection: compiled.Selection,
		},
	}
	result := callCodexBridgeTool[coordinator.Result](t, fixture.client, "workflow_exchange", codexbridge.WorkflowExchangeInput{
		HostEvidenceHandle: handle, Command: command,
	})
	if result.Snapshot == nil || len(result.Snapshot.Bundles) != 1 || result.Snapshot.Bundles[0].Digest != compiled.Digest {
		t.Fatalf("compiled=%#v started=%#v", compiled, result)
	}
	return result
}

func callCodexBridgeTool[T any](t *testing.T, client *mcp.ClientSession, name string, arguments any) T {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil || result.IsError {
		t.Fatalf("%s result=%#v err=%v", name, result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output T
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	return output
}
