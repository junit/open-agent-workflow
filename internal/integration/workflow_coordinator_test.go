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
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestWorkflowCoordinatorVerticalSliceNeverInvokesHostBinding(t *testing.T) {
	projectRoot := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "workflows")
	snapshot, integration := hosttest.LoadManagedSnapshot(t, projectRoot)
	home := t.TempDir()
	marker := filepath.Join(t.TempDir(), "host-binding-invoked")
	writeWorkflowFile(t, home, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "using-superpowers")
	writeWorkflowFile(t, home, ".codex/plugins/superpowers/skills/brainstorming/SKILL.md", "#!/bin/sh\ntouch \""+marker+"\"\nexit 97\n")

	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := workflowBindingInventory(t, snapshot.Catalog(), discovered, "oaw/superpowers")
	resolutions, effective, err := registry.Resolve(snapshot, "codex", discovered, &inventory)
	if err != nil {
		t.Fatal(err)
	}
	assertVerifiedProvider(t, resolutions, "oaw/superpowers")

	environment, session := workflowHostFacts(t, integration, inventory)
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: stateRoot, PhysicalProjectRoot: projectRoot,
		Configuration: snapshot, Resolutions: resolutions, Registry: effective,
		Authority: admission.AuthorityCeiling{
			Effects:         []string{"git-local", "network-read", "read-project", "run-process", "write-project"},
			Resources:       []string{"git-repository", "project", "project-worktree"},
			ResourceLeases:  true,
			AllowDelegation: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	started := exchangeWorkflow(t, engine, workflowStartCommand(session, environment))
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
		completed.Snapshot.ActiveNodeID == started.Snapshot.ActiveNodeID || len(completed.Snapshot.Receipts) != 2 {
		t.Fatalf("COMPLETED Receipt Result = %#v", completed)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Coordinator invoked a Host binding or process: %v", err)
	}
}

func workflowBindingInventory(t *testing.T, available catalog.Catalog, discovered discovery.Report, providerID string) host.BindingInventory {
	t.Helper()
	candidates := discovered.Candidates(providerID)
	if len(candidates) != 1 {
		t.Fatalf("Provider %s candidates = %d, want one", providerID, len(candidates))
	}
	observations := make([]host.BindingObservation, 0)
	seen := make(map[string]struct{})
	for _, provider := range available.Providers() {
		if provider.ID != providerID {
			continue
		}
		for _, capability := range provider.Capabilities {
			for _, binding := range capability.HostBindings {
				key := binding.Host + "\x00" + binding.Kind + "\x00" + binding.Reference
				if binding.Host != "codex" || !workflowBindingSupportsCurrent(binding) {
					continue
				}
				if _, found := seen[key]; found {
					continue
				}
				seen[key] = struct{}{}
				observations = append(observations, host.BindingObservation{
					HostID: "codex", InstallationKey: candidates[0].InstallationKey, Binding: binding,
					Topologies: append([]execution.Topology{}, binding.Topologies...), Source: "host-filesystem",
					EvidenceReference: filepath.Join(candidates[0].Location, "binding-evidence", binding.Kind+"-"+strings.ReplaceAll(binding.Reference, ":", "-")),
					Digest:            strings.Repeat("b", 64),
				})
			}
		}
	}
	inventory, err := host.NewBindingInventory("codex", observations)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func workflowBindingSupportsCurrent(binding catalog.HostBinding) bool {
	for _, topology := range binding.Topologies {
		if topology == execution.TopologyCurrent {
			return true
		}
	}
	return false
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
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: integration.ID,
		IntegrationVersion: integration.IntegrationVersion, SessionID: "session-current",
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return environment, session
}

func workflowStartCommand(session host.SessionSnapshot, environment host.EnvironmentReport) coordinator.Command {
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: "integration-workflow-start",
		Start: &coordinator.StartInput{
			RequestID: "request-integration", DeliverableID: "deliverable-integration", InputDigest: strings.Repeat("a", 64), ActiveTicket: "ticket-1",
			Proposal: classification.ClassificationProposal{
				SchemaVersion: classification.ProposalSchemaV1, Traits: []classification.TraitObservation{},
				Resources: []classification.Resource{}, Evidence: []classification.ProposalEvidence{},
			},
			Selection: core.Selection{
				Profile: "SP-FULL", ProfileSource: core.SelectionUser, Topology: execution.TopologyCurrent,
				TopologySource: core.SelectionHostOnlyOption, AddOns: []string{}, Bindings: []profile.ProfileBinding{},
			},
			HostSession: session, Environment: environment,
		},
	}
}

func workflowPrepareCommand(started coordinator.Result) coordinator.Command {
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandPrepare,
		MessageID: "message-prepare", IdempotencyKey: "integration-workflow-prepare",
		WorkflowID: started.WorkflowID, ExpectedRevision: started.Revision,
		Prepare: &coordinator.PrepareInput{
			RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"},
			TerminationCondition: "complete the active workflow node", InputReferences: []coordinator.ArtifactReference{},
			EvidenceRequirements: []coordinator.EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "completion report"}},
		},
	}
}

func workflowReceiptCommand(t *testing.T, prepared coordinator.Result, kind host.ReceiptKind, revision uint64) coordinator.Command {
	t.Helper()
	packet := prepared.Dispatch
	receipt := host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: kind, WorkflowID: packet.WorkflowID,
		BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest, NodeID: packet.NodeID,
		Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest, DispatchDigest: packet.Digest,
		ContextFreshness: host.ContextShared, EnvironmentReportDigest: packet.EnvironmentReportDigest,
	}
	signal := ""
	if kind == host.ReceiptCompleted {
		receipt.Outcome = "succeeded"
		receipt.Evidence = []host.EvidenceReference{{Kind: "report", Reference: "evidence://integration/report", Digest: strings.Repeat("e", 64)}}
		signal = "succeeded"
	}
	normalized, err := host.NewInvocationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandReceipt,
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
	if !found || resolution.State != registry.Verified {
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
