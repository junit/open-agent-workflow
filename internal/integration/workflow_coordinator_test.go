package integration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestWorkflowCoordinatorVerticalSliceNeverInvokesHostBinding(t *testing.T) {
	home := t.TempDir()
	providerRoot := filepath.Join(home, ".agents", "acme")
	marker := filepath.Join(t.TempDir(), "host-binding-invoked")
	writeWorkflowFile(t, providerRoot, "SKILL.md", "acme")
	skillRoot := filepath.Join(providerRoot, "skills", "zeta-review")
	writeWorkflowFile(t, skillRoot, "SKILL.md", "#!/bin/sh\ntouch \""+marker+"\"\nexit 97\n")
	providerDocument := testProviderDocument(t, "acme/suite", digestIntegrationTree(t, skillRoot), digestIntegrationTree(t, providerRoot))
	snapshot, _, _, projectRoot := buildTrustedFixture(t, providerDocument)
	stateRoot := filepath.Join(t.TempDir(), "workflows")
	integration := workflowTestHostIntegration(t)

	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := workflowBindingInventory(t, snapshot.Catalog(), discovered, "acme/suite")
	resolutions, effective, err := registry.Resolve(snapshot, "codex", discovered, &inventory)
	if err != nil {
		t.Fatal(err)
	}
	assertVerifiedProvider(t, resolutions, "acme/suite")

	environment, session := workflowHostFacts(t, integration, inventory)
	hostEvidence, err := profile.NewHostEvidence(integration.Manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: stateRoot, PhysicalProjectRoot: projectRoot,
		Configuration: snapshot, Resolutions: resolutions, Registry: effective,
		Host: hostEvidence,
		Authority: admission.AuthorityCeiling{
			Effects:         []string{"read-project"},
			Resources:       []string{"project"},
			ResourceLeases:  true,
			AllowDelegation: false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	started := exchangeWorkflow(t, engine, workflowStartCommand(t, snapshot, resolutions, effective, hostEvidence, session, environment))
	if started.Kind != coordinator.ResultState || started.Snapshot == nil || started.Snapshot.Status != coordinator.StatusReady {
		t.Fatalf("START Result = %#v", started)
	}
	prepared := exchangeWorkflow(t, engine, workflowPrepareCommand(started))
	if prepared.Kind != coordinator.ResultDispatch || prepared.Dispatch == nil || prepared.Snapshot.Status != coordinator.StatusPrepared {
		t.Fatalf("PREPARE Result = %#v", prepared)
	}
	inFlight := exchangeWorkflow(t, engine, workflowReceiptCommand(t, prepared, host.ReceiptStarted, prepared.Revision))
	if inFlight.Snapshot == nil || inFlight.Snapshot.Status != coordinator.StatusInFlight {
		t.Fatalf("STARTED Receipt Result = %#v", inFlight)
	}
	completed := exchangeWorkflow(t, engine, workflowReceiptCommand(t, prepared, host.ReceiptCompleted, inFlight.Revision))
	if completed.Snapshot == nil || completed.Snapshot.Status != coordinator.StatusReady || completed.Snapshot.ActiveGrant != nil ||
		completed.Snapshot.Cursor == started.Snapshot.Cursor || len(completed.Snapshot.Receipts) != 2 {
		t.Fatalf("COMPLETED Receipt Result = %#v", completed)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Coordinator invoked a Host binding or process: %v", err)
	}
}

func workflowTestHostIntegration(t *testing.T) host.IntegrationRecord {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "1.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1},
		BindingKinds:        []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		DelegationFeatures:  []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: "1.0.0", ID: "test/codex-host",
		Manifest: manifest, ManifestDigest: manifest.Digest,
	}
}

func workflowBindingInventory(t *testing.T, available catalog.Catalog, discovered discovery.Report, providerID string) host.BindingInventory {
	t.Helper()
	inventory := integrationInventory(t, available, discovered, map[string][]string{providerID: {"codex-zeta-review"}})
	return *inventory
}

func workflowHostFacts(t *testing.T, integration host.IntegrationRecord, inventory host.BindingInventory) (host.EnvironmentReport, host.SessionSnapshot) {
	t.Helper()
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     "session-current",
		Topology:      execution.TopologyCurrent,
		Observations:  []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(integration.Manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: integration.ID,
		IntegrationVersion: integration.IntegrationVersion, SessionID: "session-current",
		ManifestDigest:          integration.Manifest.Digest,
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: environment.Digest,
		FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return environment, session
}

func workflowStartCommand(
	t *testing.T,
	snapshot config.Snapshot,
	resolutions registry.ResolutionReport,
	effective registry.Registry,
	evidence profile.HostEvidence,
	session host.SessionSnapshot,
	environment host.EnvironmentReport,
) coordinator.Command {
	t.Helper()
	proposal := classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1, Traits: []classification.TraitObservation{},
		Resources: []classification.Resource{}, Evidence: []classification.ProposalEvidence{},
	}
	decision, err := core.Classify(&proposal, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := core.Compile(core.CompilationRequest{
		DeliverableID: "deliverable-integration", InputDigest: strings.Repeat("a", 64), Generation: 1,
		Classification: decision, Configuration: snapshot, ResolutionDigest: resolutions.Digest(), Registry: effective, Host: evidence,
		Selection: &core.Selection{
			Profile: core.UserDefinedProfile, RecipeID: "acme/review", ProfileSource: core.SelectionUser,
			Topology: execution.TopologyCurrent, TopologySource: core.SelectionUser,
			AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.SelectionPreview == nil || preview.SelectionPreview.Selection.ConfirmationDigest == "" {
		t.Fatalf("selection preview = %#v", preview.SelectionPreview)
	}
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: "integration-workflow-start",
		Start: &coordinator.StartInput{
			RequestID: "request-integration", DeliverableID: "deliverable-integration", InputDigest: strings.Repeat("a", 64), ActiveTicket: "ticket-1",
			Proposal: proposal, Selection: preview.SelectionPreview.Selection,
			HostSession: session, Environment: environment,
		},
	}
}

func workflowPrepareCommand(started coordinator.Result) coordinator.Command {
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
		MessageID: "message-prepare", IdempotencyKey: "integration-workflow-prepare",
		WorkflowID: started.WorkflowID, ExpectedRevision: started.Revision,
		Prepare: &coordinator.PrepareInput{
			RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"},
			TerminationCondition: "complete the active workflow node", InputReferences: []coordinator.ArtifactReference{},
			EvidenceRequirements: []coordinator.EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "completion report"}},
		},
	}
}

func workflowReceiptCommand(t *testing.T, prepared coordinator.Result, kind host.ReceiptKind, revision uint64) coordinator.Command {
	t.Helper()
	packet := prepared.Dispatch
	receipt := host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: kind, WorkflowID: packet.WorkflowID,
		BundleID: packet.BundleID, BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest, Cursor: packet.Cursor,
		Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest, DispatchDigest: packet.Digest,
		ContextFreshness: host.ContextShared, EnvironmentReportDigest: packet.EnvironmentReportDigest,
		Outputs: []host.OutputReference{}, Evidence: []host.EvidenceReference{},
	}
	signal := ""
	if kind == host.ReceiptCompleted {
		receipt.Outcome = "succeeded"
		receipt.Outputs = cutoverDispatchOutputs(t, *packet, "evidence://integration/output")
		receipt.Evidence = []host.EvidenceReference{{Kind: "report", Reference: "evidence://integration/report", Digest: strings.Repeat("e", 64)}}
		signal = cutoverCompletionSignal(t, prepared.Snapshot)
	}
	normalized, err := host.NewInvocationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandReceipt,
		MessageID: "message-receipt-" + strings.ToLower(string(kind)), IdempotencyKey: "integration-receipt-" + strings.ToLower(string(kind)),
		WorkflowID: prepared.WorkflowID, ExpectedRevision: revision,
		Receipt: &coordinator.ReceiptInput{Receipt: normalized, Signal: signal},
	}
}

func exchangeWorkflow(t *testing.T, engine *coordinator.Engine, command coordinator.Command) coordinator.Result {
	t.Helper()
	result, err := engine.Exchange(command)
	if err != nil {
		t.Fatalf("%v (cause: %v)", err, errors.Unwrap(err))
	}
	return result
}

func assertVerifiedProvider(t *testing.T, report registry.ResolutionReport, providerID string) {
	t.Helper()
	resolution, found := report.Resolution(providerID)
	if !found || resolution.State != registry.ProviderVerified {
		t.Fatalf("Provider %s resolution = %#v", providerID, resolution)
	}
}

func writeWorkflowFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
