package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const (
	builtInHostID      = "codex"
	thirdPartyHostID   = "codex-test"
	thirdPartyProvider = "acme/engineering"
	thirdPartyProfile  = "acme/current-delivery"
)

type cutoverCoreFacts struct {
	snapshot     config.Snapshot
	resolution   core.ResolutionResult
	effective    profile.EffectiveRegistry
	inventory    host.BindingInventory
	session      host.SessionSnapshot
	environment  host.EnvironmentReport
	hostEvidence profile.HostEvidence
	decision     classification.ClassificationDecision
}

type cutoverEffectiveRegistry struct {
	hostID       string
	providers    []registry.ProviderInstance
	providerByID map[string]registry.ProviderInstance
	bindings     map[string]registry.VerifiedBinding
	capabilities map[string]registry.VerifiedCapability
	digest       string
}

func (value *cutoverEffectiveRegistry) HostID() string { return value.hostID }

func (value *cutoverEffectiveRegistry) Providers() []registry.ProviderInstance {
	return append([]registry.ProviderInstance{}, value.providers...)
}

func (value *cutoverEffectiveRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	provider, found := value.providerByID[id]
	return provider, found
}

func (value *cutoverEffectiveRegistry) Binding(providerID, bindingID string) (registry.VerifiedBinding, bool) {
	binding, found := value.bindings[providerID+"\x00"+bindingID]
	binding.SupportedTopologies = append([]execution.Topology{}, binding.SupportedTopologies...)
	return binding, found
}

func (value *cutoverEffectiveRegistry) Bindings(providerID string) []registry.VerifiedBinding {
	provider, found := value.providerByID[providerID]
	if !found {
		return []registry.VerifiedBinding{}
	}
	return append([]registry.VerifiedBinding{}, provider.Bindings...)
}

func (value *cutoverEffectiveRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	capability, found := value.capabilities[providerID+"\x00"+capabilityID]
	capability.BindingIDs = append([]string{}, capability.BindingIDs...)
	return capability, found
}

func (value *cutoverEffectiveRegistry) Digest() string { return value.digest }

type currentSessionFixture struct {
	SchemaVersion string             `json:"schema_version"`
	HostID        string             `json:"host_id"`
	Topology      execution.Topology `json:"topology"`
}

func TestDirectAndBoundedNeverCreateWorkflowState(t *testing.T) {
	facts := builtInCutoverFacts(t)
	tests := []struct {
		name     string
		proposal classification.ClassificationProposal
		mode     classification.RequestMode
	}{
		{name: "direct", proposal: integrationDirectProposal(), mode: classification.RequestModeDirect},
		{name: "bounded", proposal: boundedCutoverProposal(), mode: classification.RequestModeBounded},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := core.Classify(&test.proposal, classification.ClassificationRules{})
			if err != nil {
				t.Fatal(err)
			}
			if decision.RequestMode != test.mode || decision.WorkflowComplexity != nil {
				t.Fatalf("classification = %#v, want %s without Workflow complexity", decision, test.mode)
			}
			request := baseCutoverCompilationRequest(facts, nil)
			request.Classification = decision
			request.Selection = nil
			compiled, err := core.Compile(request)
			if err == nil || !strings.Contains(err.Error(), "Lifecycle compilation requires WORKFLOW classification") {
				t.Fatalf("Core Compile(%s) error = %v", test.mode, err)
			}
			if compiled.Bundle != nil || len(compiled.EligibleProfiles) != 0 || len(compiled.EligibleAddOns) != 0 || compiled.Digest != "" {
				t.Fatalf("Core Compile(%s) produced Workflow output: %#v", test.mode, compiled)
			}

			stateRoot := filepath.Join(t.TempDir(), "workflow-state")
			engine, err := coordinator.NewEngine(coordinator.Options{
				StateRoot: stateRoot, Configuration: facts.snapshot,
				Resolutions: facts.resolution.Report, Registry: facts.resolution.Registry,
				Host: facts.hostEvidence,
			})
			if err != nil {
				t.Fatal(err)
			}
			command := unconfirmedCutoverStartCommand(facts)
			command.MessageID = "message-" + test.name
			command.IdempotencyKey = "cutover-" + test.name
			command.Start.Proposal = test.proposal
			if _, err := engine.Exchange(command); coordinator.ErrorCode(err) != "WORKFLOW_CLASSIFICATION_REQUIRED" {
				t.Fatalf("Coordinator START(%s) error = %v", test.mode, err)
			}
			assertNoCommittedCutoverState(t, stateRoot)
		})
	}
}

func TestAllBuiltInProfilesCompileForCurrentWhenCapabilitiesVerify(t *testing.T) {
	facts := builtInCutoverFacts(t)
	for _, profileID := range []string{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"} {
		t.Run(profileID, func(t *testing.T) {
			compiled, err := core.Compile(confirmedCutoverCompilationRequest(t, facts, profileID))
			if err != nil {
				t.Fatal(err)
			}
			bundle := requireCutoverBundle(t, compiled)
			if bundle.Selection.Profile != profileID || bundle.Topology != execution.TopologyCurrent ||
				bundle.Graph.Topology != execution.TopologyCurrent {
				t.Fatalf("Bundle selection = %#v", bundle.Selection)
			}
			if bundle.HostSessionDigest != facts.session.Digest || bundle.EnvironmentReportDigest != facts.environment.Digest ||
				bundle.ProviderInventoryDigest != facts.inventory.Digest {
				t.Fatalf("Bundle Host pins = %s / %s / %s", bundle.HostSessionDigest, bundle.EnvironmentReportDigest, bundle.ProviderInventoryDigest)
			}
			assertSingleResponsibilityOwners(t, facts.snapshot.Catalog(), bundle)
			assertBundleContainsNoInvocationState(t, bundle)
		})
	}
}

func TestThirdPartyProviderAndUserProfileUseGenericCompiler(t *testing.T) {
	facts := thirdPartyCutoverFacts(t)
	provider, providerFound := catalogProvider(facts.snapshot.Catalog(), thirdPartyProvider)
	recipe, recipeFound := catalogRecipe(facts.snapshot.Catalog(), thirdPartyProfile)
	resolution, resolutionFound := facts.resolution.Report.Resolution(thirdPartyProvider)
	if !providerFound || !recipeFound || !resolutionFound || resolution.State != registry.ProviderVerified || resolution.Instance == nil {
		t.Fatalf("generic inputs were not loaded and resolved: provider=%t recipe=%t resolution=%#v", providerFound, recipeFound, resolution)
	}
	if provider.Capabilities[0].ID != "delivery" || len(recipe.Slots) != len(catalog.CanonicalSlots()) ||
		len(recipe.Slots[0].Pipeline) != 1 || recipe.Slots[0].Pipeline[0].Selector.ProviderID != thirdPartyProvider {
		t.Fatalf("third-party descriptor or recipe changed: %#v / %#v", provider, recipe)
	}

	compiled, err := core.Compile(confirmedCutoverCompilationRequest(t, facts, thirdPartyProfile))
	if err != nil {
		t.Fatal(err)
	}
	bundle := requireCutoverBundle(t, compiled)
	if bundle.Selection.Profile != core.UserDefinedProfile || bundle.Selection.RecipeID != thirdPartyProfile ||
		bundle.Graph.RecipeID != thirdPartyProfile || len(bundle.Graph.Slots) != len(catalog.CanonicalSlots()) {
		t.Fatalf("third-party Bundle = %#v", bundle)
	}
	for _, slot := range bundle.Graph.Slots {
		for _, unit := range slot.Pipeline {
			if unit.ProviderID != thirdPartyProvider || unit.BindingID != "codex-test-delivery" || unit.Reference != "acme:delivery" {
				t.Fatalf("third-party graph unit = %#v", unit)
			}
		}
	}
	assertSingleResponsibilityOwners(t, facts.snapshot.Catalog(), bundle)
}

func TestCurrentWorkflowClosesThroughNormalizedReceipts(t *testing.T) {
	facts := thirdPartyCutoverFacts(t)
	stateRoot := filepath.Join(t.TempDir(), "workflow-state")
	projectRoot := t.TempDir()
	engine := newCutoverEngine(t, facts, stateRoot, projectRoot)

	startCommand := cutoverStartCommand(t, facts)
	current := exchangeCutoverReplay(t, engine, startCommand)
	if current.Kind != coordinator.ResultState || current.Snapshot == nil || current.Snapshot.Status != coordinator.StatusReady || current.Revision != 1 {
		t.Fatalf("START Result = %#v", current)
	}
	changedStart := cloneCutoverCommand(t, startCommand)
	changedStart.Start.ActiveTicket = "ticket-changed"
	assertCutoverIdempotencyReuse(t, engine, current.WorkflowID, current.Revision, changedStart)

	dispatched := 0
	for current.Snapshot.Status != coordinator.StatusFinished {
		if current.Snapshot.Cursor.Kind == execution.CursorGate {
			bundle := current.Snapshot.Bundles[len(current.Snapshot.Bundles)-1]
			unit, err := profile.UnitAtCursor(bundle.Graph, current.Snapshot.Cursor)
			if err != nil || unit.Gate == nil {
				t.Fatalf("active gate = %#v, %v", unit, err)
			}
			attestation := coordinator.GateAttestation{
				SchemaVersion: coordinator.GateAttestationSchemaV1, WorkflowID: current.WorkflowID, BundleID: bundle.ID,
				BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, Cursor: current.Snapshot.Cursor,
				GateID: unit.Gate.ID, Authority: unit.Gate.Authority, Decision: coordinator.GateSatisfied,
				Evidence: []host.EvidenceReference{{Kind: "approval", Reference: "evidence://cutover/closeout", Digest: strings.Repeat("d", 64)}},
			}
			attestation.Digest = cutoverGateAttestationDigest(t, attestation)
			current = exchangeCutoverReplay(t, engine, coordinator.Command{
				SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
				MessageID: "message-gate-" + unit.Gate.ID, IdempotencyKey: "gate-" + unit.Gate.ID,
				WorkflowID: current.WorkflowID, ExpectedRevision: current.Revision,
				Prepare: &coordinator.PrepareInput{
					RequestedEffects: []string{}, RequestedResources: []string{}, InputReferences: []coordinator.ArtifactReference{},
					EvidenceRequirements: []coordinator.EvidenceRequirement{}, GateAttestation: &attestation,
				},
			})
			continue
		}
		cursor := current.Snapshot.Cursor
		unitID := strings.Join([]string{cursor.SlotID, string(cursor.Kind), cursor.UnitID}, "-")
		prepare := coordinator.Command{
			SchemaVersion:    coordinator.WorkflowCommandSchemaV2,
			Kind:             coordinator.CommandPrepare,
			MessageID:        "message-prepare-" + unitID,
			IdempotencyKey:   "prepare-" + unitID,
			WorkflowID:       current.WorkflowID,
			ExpectedRevision: current.Revision,
			Prepare: &coordinator.PrepareInput{
				RequestedEffects:     []string{"read-project"},
				RequestedResources:   []string{"project-worktree"},
				TerminationCondition: "complete " + unitID,
				InputReferences:      []coordinator.ArtifactReference{},
				EvidenceRequirements: []coordinator.EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "node completion report"}},
			},
		}
		prepared := exchangeCutoverReplay(t, engine, prepare)
		if prepared.Kind != coordinator.ResultDispatch || prepared.Dispatch == nil || prepared.Snapshot.Status != coordinator.StatusPrepared {
			t.Fatalf("PREPARE(%s) Result = %#v", unitID, prepared)
		}
		dispatched++
		changedPrepare := cloneCutoverCommand(t, prepare)
		changedPrepare.Prepare.TerminationCondition += " with changed content"
		assertCutoverIdempotencyReuse(t, engine, prepared.WorkflowID, prepared.Revision, changedPrepare)

		identity := cutoverReceiptIdentity(*prepared.Dispatch)
		startedCommand := coordinator.Command{
			SchemaVersion:    coordinator.WorkflowCommandSchemaV2,
			Kind:             coordinator.CommandReceipt,
			MessageID:        "message-started-" + unitID,
			IdempotencyKey:   "started-" + unitID,
			WorkflowID:       prepared.WorkflowID,
			ExpectedRevision: prepared.Revision,
			Receipt:          &coordinator.ReceiptInput{Receipt: hosttest.StartedReceipt(t, identity, "")},
		}
		inFlight := exchangeCutoverReplay(t, engine, startedCommand)
		if inFlight.Snapshot == nil || inFlight.Snapshot.Status != coordinator.StatusInFlight {
			t.Fatalf("STARTED(%s) Result = %#v", unitID, inFlight)
		}
		changedStarted := cloneCutoverCommand(t, startedCommand)
		changedStarted.Receipt.StableBoundary = "changed-content"
		assertCutoverIdempotencyReuse(t, engine, inFlight.WorkflowID, inFlight.Revision, changedStarted)

		evidence := []host.EvidenceReference{{
			Kind: "report", Reference: "evidence://cutover/" + unitID, Digest: strings.Repeat("e", 64),
		}}
		outputs := cutoverDispatchOutputs(t, *prepared.Dispatch, "evidence://cutover/output/"+unitID)
		completedCommand := coordinator.Command{
			SchemaVersion:    coordinator.WorkflowCommandSchemaV2,
			Kind:             coordinator.CommandReceipt,
			MessageID:        "message-completed-" + unitID,
			IdempotencyKey:   "completed-" + unitID,
			WorkflowID:       inFlight.WorkflowID,
			ExpectedRevision: inFlight.Revision,
			Receipt: &coordinator.ReceiptInput{
				Receipt: hosttest.CompletedReceipt(t, identity, "", outputs, evidence),
				Signal:  cutoverCompletionSignal(t, current.Snapshot),
			},
		}
		current = exchangeCutoverReplay(t, engine, completedCommand)
		changedCompleted := cloneCutoverCommand(t, completedCommand)
		changedCompleted.Receipt.StableBoundary = "changed-content"
		assertCutoverIdempotencyReuse(t, engine, current.WorkflowID, current.Revision, changedCompleted)
	}

	if current.Snapshot.ActiveGrant != nil || dispatched == 0 || len(current.Snapshot.Receipts) != dispatched*2 ||
		current.Snapshot.Cursor.Kind != execution.CursorGate {
		t.Fatalf("terminal Workflow Result = %#v", current)
	}
	beforeInspect := readCutoverState(t, stateRoot)
	inspected := exchangeWorkflow(t, engine, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2,
		Kind:          coordinator.CommandInspect,
		WorkflowID:    current.WorkflowID,
	})
	afterInspect := readCutoverState(t, stateRoot)
	if !bytes.Equal(stableCutoverResultBytes(t, current), stableCutoverResultBytes(t, inspected)) ||
		!reflect.DeepEqual(beforeInspect, afterInspect) {
		t.Fatalf("INSPECT changed terminal state: current=%#v inspected=%#v", current, inspected)
	}
}

func TestRecoveryAcrossEnginesPreservesLeasesAndUncertainty(t *testing.T) {
	facts := thirdPartyCutoverFacts(t)
	stateRoot := filepath.Join(t.TempDir(), "workflow-state")
	projectRoot := t.TempDir()
	first := newCutoverEngine(t, facts, stateRoot, projectRoot)
	second := newCutoverEngine(t, facts, stateRoot, projectRoot)

	start := cutoverStartCommand(t, facts)
	ready := exchangeWorkflow(t, first, start)
	prepared := exchangeWorkflow(t, first, cutoverPrepareCommand(ready, "recovery-first-prepare", "write-project"))
	if prepared.Snapshot.Status != coordinator.StatusPrepared || len(prepared.Snapshot.ResourceLeases) != 1 {
		t.Fatalf("first PREPARED Result = %#v", prepared)
	}

	secondStart := cloneCutoverCommand(t, start)
	secondStart.MessageID = "message-recovery-second-start"
	secondStart.IdempotencyKey = "recovery-second-start"
	secondStart.Start.RequestID = "request-recovery-second"
	secondStart.Start.DeliverableID = "deliverable-recovery-second"
	secondReady := exchangeWorkflow(t, second, secondStart)
	secondPrepare := cutoverPrepareCommand(secondReady, "recovery-second-prepare", "write-project")
	if _, err := second.Exchange(secondPrepare); coordinator.ErrorCode(err) != "RESOURCE_LEASE_CONFLICT" {
		t.Fatalf("second Workflow lease conflict = %v", err)
	}

	afterPrepare := newCutoverEngine(t, facts, stateRoot, projectRoot)
	preparedInspection := exchangeWorkflow(t, afterPrepare, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandInspect, WorkflowID: prepared.WorkflowID,
	})
	if preparedInspection.RevisionDigest != prepared.RevisionDigest || preparedInspection.Snapshot.Status != coordinator.StatusPrepared ||
		preparedInspection.Dispatch == nil || preparedInspection.Snapshot.ActiveGrant == nil {
		t.Fatalf("recovered PREPARED Result = %#v", preparedInspection)
	}

	identity := cutoverReceiptIdentity(*preparedInspection.Dispatch)
	started := exchangeWorkflow(t, afterPrepare, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandReceipt,
		MessageID: "message-recovery-started", IdempotencyKey: "recovery-started", WorkflowID: prepared.WorkflowID,
		ExpectedRevision: preparedInspection.Revision,
		Receipt:          &coordinator.ReceiptInput{Receipt: hosttest.StartedReceipt(t, identity, "")},
	})
	afterStarted := newCutoverEngine(t, facts, stateRoot, projectRoot)
	startedInspection := exchangeWorkflow(t, afterStarted, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandInspect, WorkflowID: started.WorkflowID,
	})
	if startedInspection.RevisionDigest != started.RevisionDigest || startedInspection.Snapshot.Status != coordinator.StatusInFlight ||
		startedInspection.Snapshot.ActiveGrant == nil || len(startedInspection.Snapshot.Receipts) != 1 {
		t.Fatalf("recovered IN_FLIGHT Result = %#v", startedInspection)
	}

	pending := exchangeWorkflow(t, afterStarted, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandCancel,
		MessageID: "message-recovery-pending", IdempotencyKey: "recovery-pending", WorkflowID: started.WorkflowID,
		ExpectedRevision: startedInspection.Revision,
		Cancel:           &coordinator.CancelInput{Reason: "uncertain host termination", InvocationTerminal: false},
	})
	if pending.Snapshot.Status != coordinator.StatusPaused || pending.Snapshot.ActiveGrant == nil || pending.Snapshot.ResourceLeases[0].ReleasedRevision != 0 {
		t.Fatalf("pending uncertainty Result = %#v", pending)
	}
	afterPending := newCutoverEngine(t, facts, stateRoot, projectRoot)
	recoveredPending := exchangeWorkflow(t, afterPending, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandInspect, WorkflowID: pending.WorkflowID,
	})
	if recoveredPending.RevisionDigest != pending.RevisionDigest || recoveredPending.Snapshot.Status != coordinator.StatusPaused || recoveredPending.Snapshot.ActiveGrant == nil {
		t.Fatalf("recovered uncertainty Result = %#v", recoveredPending)
	}
	confirmed := exchangeWorkflow(t, afterPending, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandCancel,
		MessageID: "message-recovery-confirmed", IdempotencyKey: "recovery-confirmed", WorkflowID: pending.WorkflowID,
		ExpectedRevision: recoveredPending.Revision,
		Cancel:           &coordinator.CancelInput{Reason: "host termination confirmed", InvocationTerminal: true},
	})
	if confirmed.Snapshot.Status != coordinator.StatusCancelled || confirmed.Snapshot.ActiveGrant != nil || confirmed.Snapshot.ResourceLeases[0].ReleasedRevision != confirmed.Revision {
		t.Fatalf("confirmed uncertainty Result = %#v", confirmed)
	}

	secondPrepared := exchangeWorkflow(t, second, secondPrepare)
	if secondPrepared.Snapshot.Status != coordinator.StatusPrepared || len(secondPrepared.Snapshot.ResourceLeases) != 1 {
		t.Fatalf("second Workflow after release = %#v", secondPrepared)
	}
	secondCancelled := exchangeWorkflow(t, second, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandCancel,
		MessageID: "message-recovery-second-cancel", IdempotencyKey: "recovery-second-cancel", WorkflowID: secondReady.WorkflowID,
		ExpectedRevision: secondPrepared.Revision,
		Cancel:           &coordinator.CancelInput{Reason: "release test Workflow", InvocationTerminal: true},
	})
	if secondCancelled.Snapshot.Status != coordinator.StatusCancelled || secondCancelled.Snapshot.ResourceLeases[0].ReleasedRevision != secondCancelled.Revision {
		t.Fatalf("second Workflow cancellation = %#v", secondCancelled)
	}
}

func TestOldRuntimeContractsFailClosed(t *testing.T) {
	t.Run("schema and model launch path", func(t *testing.T) {
		raw := readCutoverFixture(t, "old-runtime-command.json")
		if _, err := coordinator.DecodeCommand(raw); err == nil {
			t.Fatal("old Runtime command was accepted")
		}
	})

	t.Run("old commands", func(t *testing.T) {
		for _, kind := range []string{"RUN", "PROFILE_SELECTED", "DISPATCHED"} {
			raw := []byte(`{"schema_version":"oaw.workflow-command/v1","kind":"` + kind + `","message_id":"old","idempotency_key":"old","workflow_id":"","expected_revision":0}`)
			if _, err := coordinator.DecodeCommand(raw); err == nil {
				t.Fatalf("old command %s was accepted", kind)
			}
		}
	})

	t.Run("old topology values", func(t *testing.T) {
		for _, topology := range []execution.Topology{"INLINE", "NATIVE_SUBAGENT"} {
			if _, err := execution.NormalizeTopologies([]execution.Topology{topology}); err == nil {
				t.Fatalf("old topology %s was accepted", topology)
			}
		}
	})

	t.Run("old state", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		if err := os.MkdirAll(filepath.Join(stateRoot, "runs"), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := coordinator.NewEngine(coordinator.Options{StateRoot: stateRoot}); coordinator.ErrorCode(err) != "WORKFLOW_STATE_UNSUPPORTED" {
			t.Fatalf("NewEngine(old state) error = %v", err)
		}
	})

	t.Run("model launch fields", func(t *testing.T) {
		raw := []byte(`{"schema_version":"oaw.workflow-command/v1","kind":"INSPECT","message_id":"","idempotency_key":"","workflow_id":"workflow-old","expected_revision":0,"model_command":"codex exec"}`)
		if _, err := coordinator.DecodeCommand(raw); coordinator.ErrorCode(err) != "WORKFLOW_COMMAND_DECODE_INVALID" {
			t.Fatalf("DecodeCommand(model launch field) error = %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join("..", "runtime")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired internal/runtime surface still exists: %v", err)
	}
}

func builtInCutoverFacts(t *testing.T) cutoverCoreFacts {
	t.Helper()
	snapshot, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	effective, inventory := completeBuiltInCutoverRegistry(t, snapshot.Catalog(), builtInHostID)
	session, environment, hostEvidence := cutoverHostFacts(t, builtInHostID, inventory, true)
	decision, err := core.Classify(workflowCutoverProposal(), classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	return cutoverCoreFacts{
		snapshot: snapshot, effective: effective, inventory: inventory,
		session: session, environment: environment, hostEvidence: hostEvidence, decision: decision,
	}
}

func completeBuiltInCutoverRegistry(t *testing.T, available catalog.Catalog, hostID string) (*cutoverEffectiveRegistry, host.BindingInventory) {
	t.Helper()
	observations := make([]host.BindingObservation, 0)
	for _, provider := range available.Providers() {
		for _, binding := range provider.Bindings {
			if binding.Host != hostID {
				continue
			}
			observation, err := host.NewBindingObservation(host.BindingObservation{
				HostID: hostID, ProviderID: provider.ID, InstallationKey: "installation-" + strings.ReplaceAll(provider.ID, "/", "-"),
				DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface,
				Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
				Topologies: append([]execution.Topology{}, binding.SupportedTopologies...), Source: host.SourceNativeAPI,
				EvidenceReference: "evidence://cutover/bindings/" + strings.ReplaceAll(provider.ID, "/", "-") + "/" + binding.ID,
			})
			if err != nil {
				t.Fatal(err)
			}
			observations = append(observations, observation)
		}
	}
	inventory, err := host.BuildBindingInventoryV3(hostID, observations)
	if err != nil {
		t.Fatal(err)
	}
	effective := &cutoverEffectiveRegistry{
		hostID: hostID, providerByID: map[string]registry.ProviderInstance{},
		bindings: map[string]registry.VerifiedBinding{}, capabilities: map[string]registry.VerifiedCapability{},
	}
	for _, provider := range available.Providers() {
		verifiedBindings := make([]registry.VerifiedBinding, 0)
		for _, binding := range provider.Bindings {
			if binding.Host != hostID {
				continue
			}
			var distribution catalog.DistributionRecord
			for _, candidate := range provider.Distributions {
				if candidate.ID == binding.DistributionID {
					distribution = candidate
					break
				}
			}
			var observation host.BindingObservation
			for _, candidate := range inventory.Observations {
				if candidate.ProviderID == provider.ID && candidate.BindingID == binding.ID {
					observation = candidate
					break
				}
			}
			verified := registry.VerifiedBinding{
				BindingID: binding.ID, DistributionID: binding.DistributionID, DistributionRevision: distribution.Revision,
				DistributionTreeDigest: distribution.TreeDigest, Surface: binding.Surface, Kind: binding.Kind,
				Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
				SupportedTopologies: append([]execution.Topology{}, binding.SupportedTopologies...), Delegation: binding.Delegation,
				Provenance: discovery.ProvenanceDistributionAttested, BindingEvidenceDigest: observation.Digest,
			}
			verifiedBindings = append(verifiedBindings, verified)
			effective.bindings[provider.ID+"\x00"+binding.ID] = verified
		}
		if len(verifiedBindings) == 0 {
			continue
		}
		verifiedCapabilities := make([]registry.VerifiedCapability, 0)
		for _, capability := range provider.Capabilities {
			bindingIDs := make([]string, 0)
			for _, bindingID := range capability.BindingRefs {
				if _, found := effective.bindings[provider.ID+"\x00"+bindingID]; found {
					bindingIDs = append(bindingIDs, bindingID)
				}
			}
			if len(bindingIDs) == 0 {
				continue
			}
			sort.Strings(bindingIDs)
			verified := registry.VerifiedCapability{ID: capability.ID, BindingIDs: bindingIDs}
			verifiedCapabilities = append(verifiedCapabilities, verified)
			effective.capabilities[provider.ID+"\x00"+capability.ID] = verified
		}
		descriptorDigest, _, err := canonicaljson.Digest(provider)
		if err != nil {
			t.Fatal(err)
		}
		distribution := provider.Distributions[0]
		instance := registry.ProviderInstance{
			ProviderID: provider.ID, HostID: hostID, DescriptorDigest: descriptorDigest,
			DistributionID: distribution.ID, DistributionRevision: distribution.Revision, DistributionTreeDigest: distribution.TreeDigest,
			InstallationKey:     "installation-" + strings.ReplaceAll(provider.ID, "/", "-"),
			ConfigurationDigest: canonicaljson.DigestBytes([]byte(provider.ID + "/configuration")), BindingInventoryDigest: inventory.Digest,
			EvidenceDigest: canonicaljson.DigestBytes([]byte(provider.ID + "/evidence")), Bindings: verifiedBindings, Capabilities: verifiedCapabilities,
		}
		instance.Digest = cutoverProviderInstanceDigest(instance)
		effective.providers = append(effective.providers, instance)
		effective.providerByID[provider.ID] = instance
	}
	sort.Slice(effective.providers, func(left, right int) bool {
		return effective.providers[left].ProviderID < effective.providers[right].ProviderID
	})
	effective.digest, _, err = canonicaljson.Digest(struct {
		SchemaVersion string                      `json:"schema_version"`
		HostID        string                      `json:"host_id"`
		Providers     []registry.ProviderInstance `json:"providers"`
	}{"oaw.effective-registry/v4", hostID, effective.providers})
	if err != nil {
		t.Fatal(err)
	}
	return effective, inventory
}

func cutoverProviderInstanceDigest(instance registry.ProviderInstance) string {
	record := struct {
		SchemaVersion          string                        `json:"schema_version"`
		ProviderID             string                        `json:"provider_id"`
		HostID                 string                        `json:"host_id"`
		DescriptorDigest       string                        `json:"descriptor_digest"`
		DistributionID         string                        `json:"distribution_id"`
		DistributionRevision   string                        `json:"distribution_revision"`
		DistributionTreeDigest string                        `json:"distribution_tree_digest"`
		InstallationKey        string                        `json:"installation_key"`
		ConfigurationDigest    string                        `json:"configuration_digest"`
		BindingInventoryDigest string                        `json:"binding_inventory_digest"`
		EvidenceDigest         string                        `json:"evidence_digest"`
		Bindings               []registry.VerifiedBinding    `json:"bindings"`
		Capabilities           []registry.VerifiedCapability `json:"capabilities"`
	}{
		"oaw.provider-instance/v4", instance.ProviderID, instance.HostID, instance.DescriptorDigest,
		instance.DistributionID, instance.DistributionRevision, instance.DistributionTreeDigest, instance.InstallationKey,
		instance.ConfigurationDigest, instance.BindingInventoryDigest, instance.EvidenceDigest, instance.Bindings, instance.Capabilities,
	}
	digest, _, _ := canonicaljson.Digest(record)
	return digest
}

func cutoverHostFacts(t *testing.T, hostID string, inventory host.BindingInventory, complete bool) (host.SessionSnapshot, host.EnvironmentReport, profile.HostEvidence) {
	t.Helper()
	kindSet := make(map[catalog.BindingKind]struct{})
	for _, observation := range inventory.Observations {
		kindSet[observation.Kind] = struct{}{}
	}
	kinds := make([]catalog.BindingKind, 0, len(kindSet))
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}
	features := []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}
	topologies := []execution.Topology{execution.TopologyCurrent}
	delegation := []host.FeatureID{}
	actions := []host.HostActionContract{}
	if complete {
		topologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
		delegation = []host.FeatureID{
			host.FeatureChildDelegation, host.FeatureNestedChildDelegation,
			host.FeatureNestedParallelDelegation, host.FeatureParallelChildDelegation,
		}
		actions = cutoverHostActions()
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: hostID,
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: kinds,
		SupportedTopologies: topologies, Features: features, DelegationFeatures: delegation, HostActions: actions,
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-" + hostID,
		Topology: execution.TopologyCurrent, Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	featureObservations := make([]host.FeatureObservation, 0, len(delegation))
	for _, feature := range delegation {
		observation, err := host.NewFeatureObservation(host.FeatureObservation{
			Feature: feature, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://cutover/features/" + string(feature),
		})
		if err != nil {
			t.Fatal(err)
		}
		featureObservations = append(featureObservations, observation)
	}
	actionObservations := make([]host.HostActionObservation, 0, len(actions))
	for _, action := range actions {
		observation, err := host.NewHostActionObservation(host.HostActionObservation{
			Action: action, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://cutover/actions/" + action.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		actionObservations = append(actionObservations, observation)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: hostID, IntegrationID: "test/" + hostID,
		IntegrationVersion: "3.0.0", SessionID: "session-" + hostID, ManifestDigest: manifest.Digest,
		SupportedTopologies: topologies, ProviderInventoryDigest: inventory.Digest,
		FeatureObservations: featureObservations, HostActionObservations: actionObservations, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	return session, environment, evidence
}

func cutoverHostActions() []host.HostActionContract {
	return []host.HostActionContract{
		{ID: "workspace.prepare-or-confirm", InputSchema: "oaw.host-action.workspace-input/v1", OutcomeSchema: "oaw.host-action.workspace-outcome/v1", MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project-worktree"}},
		{ID: "verification.execute", InputSchema: "oaw.host-action.verification-input/v1", OutcomeSchema: "oaw.host-action.verification-outcome/v1", MaximumEffects: []string{"read-project", "run-process"}, Resources: []string{"project"}},
		{ID: "closeout.execute", InputSchema: "oaw.host-action.closeout-input/v1", OutcomeSchema: "oaw.host-action.closeout-outcome/v1", MaximumEffects: []string{"git-local", "network-mutation", "read-project", "run-process"}, Resources: []string{"git-repository", "network", "project-worktree"}},
	}
}

func thirdPartyCutoverFacts(t *testing.T) cutoverCoreFacts {
	t.Helper()
	fixture := loadCurrentSessionFixture(t)
	if fixture.HostID != thirdPartyHostID || fixture.Topology != execution.TopologyCurrent {
		t.Fatalf("CURRENT session fixture = %#v", fixture)
	}
	home := t.TempDir()
	providerRoot := filepath.Join(home, ".agents", "acme-engineering")
	writeFile(t, providerRoot, "SKILL.md", "acme engineering")
	skillRoot := filepath.Join(providerRoot, "skills", "delivery")
	writeFile(t, skillRoot, "SKILL.md", "---\nname: acme:delivery\n---\n")
	var descriptor catalog.ProviderDescriptorRecord
	if err := json.Unmarshal(readCutoverFixture(t, "acme-provider.json"), &descriptor); err != nil {
		t.Fatal(err)
	}
	descriptor.Distributions[0].TreeDigest = digestIntegrationTree(t, providerRoot)
	descriptor.Bindings[0].TreeDigest = digestIntegrationTree(t, skillRoot)
	descriptorRaw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	userRoot := t.TempDir()
	writeFile(t, userRoot, "acme-provider.json", string(descriptorRaw))
	writeFile(t, userRoot, "acme-profile.json", string(readCutoverFixture(t, "acme-profile.json")))
	writeFile(t, userRoot, "config.toml", string(readCutoverFixture(t, "user-config.toml")))
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot})
	if err != nil {
		t.Fatal(err)
	}
	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: fixture.HostID, UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := cutoverBindingInventory(t, snapshot.Catalog(), discovered, thirdPartyProvider)
	return resolveCutoverFacts(t, snapshot, fixture.HostID, discovered, inventory)
}

func cutoverBindingInventory(t *testing.T, available catalog.Catalog, discovered discovery.Report, providerIDs ...string) host.BindingInventory {
	t.Helper()
	observations := make([]host.BindingObservation, 0)
	for _, providerID := range providerIDs {
		candidates := discovered.Candidates(providerID)
		if len(candidates) != 1 {
			t.Fatalf("Provider %s candidates = %d, want one", providerID, len(candidates))
		}
		for _, provider := range available.Providers() {
			if provider.ID != providerID {
				continue
			}
			for _, binding := range provider.Bindings {
				if binding.Host != discovered.HostID() {
					continue
				}
				observations = append(observations, host.BindingObservation{
					HostID: discovered.HostID(), ProviderID: provider.ID, InstallationKey: candidates[0].InstallationKey,
					DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface,
					Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
					Topologies: append([]execution.Topology{}, binding.SupportedTopologies...), Source: host.SourceLiveFilesystem,
					EvidenceReference: "evidence://cutover/" + strings.ReplaceAll(provider.ID, "/", "-") + "/" + binding.ID,
				})
			}
		}
	}
	inventory, err := host.BuildBindingInventoryV3(discovered.HostID(), observations)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func resolveCutoverFacts(t *testing.T, snapshot config.Snapshot, hostID string, discovered discovery.Report, inventory host.BindingInventory) cutoverCoreFacts {
	t.Helper()
	resolved, err := core.Resolve(core.ResolutionRequest{
		Configuration: snapshot,
		HostID:        hostID,
		Discovery:     discovered,
		Inventory:     &inventory,
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := core.Classify(workflowCutoverProposal(), classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	session, environment, hostEvidence := cutoverHostFacts(t, hostID, inventory, false)
	return cutoverCoreFacts{
		snapshot: snapshot, resolution: resolved, effective: resolved.Registry, inventory: inventory,
		session: session, environment: environment, hostEvidence: hostEvidence, decision: decision,
	}
}

func newCutoverEngine(t *testing.T, facts cutoverCoreFacts, stateRoot, projectRoot string) *coordinator.Engine {
	t.Helper()
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: stateRoot, PhysicalProjectRoot: projectRoot,
		Configuration: facts.snapshot, Resolutions: facts.resolution.Report, Registry: facts.resolution.Registry,
		Host: facts.hostEvidence,
		Authority: admission.AuthorityCeiling{
			Effects: []string{"read-project", "run-process", "write-project"}, Resources: []string{"project-worktree"},
			ResourceLeases: true, AllowDelegation: false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func cutoverPrepareCommand(ready coordinator.Result, key, effect string) coordinator.Command {
	unitID := ready.Snapshot.Cursor.UnitID
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandPrepare,
		MessageID: "message-" + key, IdempotencyKey: key, WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision,
		Prepare: &coordinator.PrepareInput{
			RequestedEffects: []string{effect}, RequestedResources: []string{"project-worktree"},
			TerminationCondition: "complete " + unitID, InputReferences: []coordinator.ArtifactReference{},
			EvidenceRequirements: []coordinator.EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "recovery report"}},
		},
	}
}

func baseCutoverCompilationRequest(facts cutoverCoreFacts, selection *core.Selection) core.CompilationRequest {
	resolutionDigest := facts.resolution.Report.Digest()
	if resolutionDigest == "" {
		resolutionDigest = canonicaljson.DigestBytes([]byte("complete-built-in-cutover-resolution"))
	}
	return core.CompilationRequest{
		DeliverableID: "deliverable-cutover", InputDigest: strings.Repeat("a", 64), Generation: 1,
		Classification: facts.decision, Configuration: facts.snapshot,
		ResolutionDigest: resolutionDigest, Registry: facts.effective, Host: facts.hostEvidence, Selection: selection,
	}
}

func confirmedCutoverCompilationRequest(t *testing.T, facts cutoverCoreFacts, profileID string) core.CompilationRequest {
	t.Helper()
	inspected, err := core.Compile(baseCutoverCompilationRequest(facts, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range inspected.EligibleProfiles {
		matches := candidate.Profile == profileID
		if profileID == thirdPartyProfile {
			matches = candidate.Profile == core.UserDefinedProfile && candidate.RecipeID == profileID
		}
		if matches && candidate.Eligible && candidate.Topology == execution.TopologyCurrent {
			selection := candidate.Preview.Selection
			selection.ProfileSource = core.SelectionUser
			selection.TopologySource = core.SelectionUser
			return baseCutoverCompilationRequest(facts, &selection)
		}
	}
	t.Fatalf("Profile %s CURRENT is unavailable: %#v", profileID, inspected.EligibleProfiles)
	return core.CompilationRequest{}
}

func cutoverStartCommand(t *testing.T, facts cutoverCoreFacts) coordinator.Command {
	t.Helper()
	selection := confirmedCutoverCompilationRequest(t, facts, thirdPartyProfile).Selection
	return coordinator.Command{
		SchemaVersion:  coordinator.WorkflowCommandSchemaV2,
		Kind:           coordinator.CommandStart,
		MessageID:      "message-start-cutover",
		IdempotencyKey: "start-current-cutover",
		Start: &coordinator.StartInput{
			RequestID: "request-cutover", DeliverableID: "deliverable-cutover", InputDigest: strings.Repeat("a", 64),
			ActiveTicket: "ticket-cutover", Proposal: *workflowCutoverProposal(),
			Selection:   *selection,
			HostSession: facts.session, Environment: facts.environment,
		},
	}
}

func unconfirmedCutoverStartCommand(facts cutoverCoreFacts) coordinator.Command {
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandStart,
		MessageID: "message-unconfirmed-cutover", IdempotencyKey: "unconfirmed-cutover",
		Start: &coordinator.StartInput{
			RequestID: "request-cutover", DeliverableID: "deliverable-cutover", InputDigest: strings.Repeat("a", 64),
			Proposal: *workflowCutoverProposal(),
			Selection: core.Selection{
				Profile: "SP-FULL", ProfileSource: core.SelectionUser,
				Topology: execution.TopologyCurrent, TopologySource: core.SelectionUser,
				AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{},
			},
			HostSession: facts.session, Environment: facts.environment,
		},
	}
}

func boundedCutoverProposal() classification.ClassificationProposal {
	proposal := integrationDirectProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitBoundedCapabilityRequest {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	proposal.CapabilitySelector = &classification.CapabilitySelector{
		ProviderID: "oaw/superpowers", CapabilityID: "review", Source: classification.SelectorUserIntent,
	}
	return proposal
}

func workflowCutoverProposal() *classification.ClassificationProposal {
	proposal := integrationDirectProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitMultipleResponsibilities {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	return &proposal
}

func requireCutoverBundle(t *testing.T, compiled core.CompilationResult) core.LifecycleBundle {
	t.Helper()
	if compiled.Bundle == nil || compiled.Bundle.Digest == "" || compiled.Digest == "" {
		t.Fatalf("Compilation Result has no Bundle: %#v", compiled)
	}
	return *compiled.Bundle
}

func assertSingleResponsibilityOwners(t *testing.T, available catalog.Catalog, bundle core.LifecycleBundle) {
	t.Helper()
	recipe, found := catalogRecipe(available, bundle.Graph.RecipeID)
	if !found {
		t.Fatalf("Bundle recipe %s is not configured", bundle.Graph.RecipeID)
	}
	if len(recipe.Slots) != len(catalog.CanonicalSlots()) || len(bundle.Graph.Slots) != len(recipe.Slots) {
		t.Fatalf("Recipe/Graph slot counts = %d/%d", len(recipe.Slots), len(bundle.Graph.Slots))
	}
	for index, slot := range bundle.Graph.Slots {
		if slot.SlotID != recipe.Slots[index].SlotID {
			t.Fatalf("slot %d identity = %s/%s", index, recipe.Slots[index].SlotID, slot.SlotID)
		}
		if slot.Active && slot.SlotID != catalog.SlotIncidentRecovery && slot.OutcomeOwner.Kind == catalog.OwnerNone {
			t.Fatalf("active slot %s has no outcome owner in %s", slot.SlotID, bundle.Selection.Profile)
		}
		if slot.OutcomeOwner.Kind == catalog.OwnerProviderBinding && (slot.OutcomeOwner.ProviderID == "" || slot.OutcomeOwner.BindingID == "" || slot.OutcomeOwner.UnitID == "") {
			t.Fatalf("Provider-owned slot %s has incomplete owner %#v", slot.SlotID, slot.OutcomeOwner)
		}
		if slot.OutcomeOwner.Kind == catalog.OwnerHostAction && (slot.OutcomeOwner.HostActionID == "" || slot.OutcomeOwner.UnitID == "") {
			t.Fatalf("Host-owned slot %s has incomplete owner %#v", slot.SlotID, slot.OutcomeOwner)
		}
	}
}

func assertBundleContainsNoInvocationState(t *testing.T, bundle core.LifecycleBundle) {
	t.Helper()
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"model_command", "process_command", "environment_variables", "authorization", "api_key", "password", "token"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("Bundle contains private invocation field %q", forbidden)
		}
	}
}

func catalogProvider(available catalog.Catalog, id string) (catalog.ProviderDescriptorRecord, bool) {
	for _, provider := range available.Providers() {
		if provider.ID == id {
			return provider, true
		}
	}
	return catalog.ProviderDescriptorRecord{}, false
}

func catalogRecipe(available catalog.Catalog, id string) (catalog.ProfileRecipeRecord, bool) {
	for _, recipe := range available.Recipes() {
		if recipe.ID == id {
			return recipe, true
		}
	}
	return catalog.ProfileRecipeRecord{}, false
}

func cutoverReceiptIdentity(packet coordinator.DispatchPacket) hosttest.ReceiptIdentity {
	return hosttest.ReceiptIdentity{
		WorkflowID: packet.WorkflowID, BundleID: packet.BundleID, BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest,
		Cursor: packet.Cursor, Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest,
		DispatchDigest: packet.Digest, EnvironmentReportDigest: packet.EnvironmentReportDigest,
	}
}

func cutoverDispatchOutputs(t *testing.T, packet coordinator.DispatchPacket, reference string) []host.OutputReference {
	t.Helper()
	artifactID, schema := "", ""
	switch packet.Grant.Target.TargetKind {
	case admission.GrantProviderBinding:
		if packet.Grant.Target.ProviderBinding == nil {
			t.Fatal("Dispatch provider target is missing")
		}
		artifactID = packet.Grant.Target.ProviderBinding.OutputArtifact
		schema = packet.Grant.Target.ProviderBinding.OutcomeSchema
	case admission.GrantHostAction:
		if packet.Grant.Target.HostAction == nil {
			t.Fatal("Dispatch Host action target is missing")
		}
		artifactID = packet.Grant.Target.HostAction.OutputArtifact
		schema = packet.Grant.Target.HostAction.OutcomeSchema
	default:
		t.Fatalf("unsupported Dispatch target %s", packet.Grant.Target.TargetKind)
	}
	return []host.OutputReference{{ArtifactID: artifactID, Schema: schema, Reference: reference, Digest: strings.Repeat("f", 64)}}
}

func cutoverCompletionSignal(t *testing.T, snapshot *coordinator.Snapshot) string {
	t.Helper()
	graph := snapshot.Bundles[len(snapshot.Bundles)-1].Graph
	if _, err := profile.NextActionableCursor(graph, snapshot.Cursor, "", ""); err == nil {
		return ""
	}
	if _, err := profile.NextActionableCursor(graph, snapshot.Cursor, "succeeded", ""); err != nil {
		t.Fatalf("active cursor has no completion transition: %#v: %v", snapshot.Cursor, err)
	}
	return "succeeded"
}

func cutoverGateAttestationDigest(t *testing.T, value coordinator.GateAttestation) string {
	t.Helper()
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func exchangeCutoverReplay(t *testing.T, engine *coordinator.Engine, command coordinator.Command) coordinator.Result {
	t.Helper()
	first := exchangeWorkflow(t, engine, command)
	replayed := exchangeWorkflow(t, engine, command)
	if !replayed.Replayed || first.Replayed || first.Revision != replayed.Revision || first.RevisionDigest != replayed.RevisionDigest ||
		!bytes.Equal(stableCutoverResultBytes(t, first), stableCutoverResultBytes(t, replayed)) {
		t.Fatalf("idempotent replay changed authoritative Result:\nfirst: %#v\nreplay: %#v", first, replayed)
	}
	return first
}

func assertCutoverIdempotencyReuse(t *testing.T, engine *coordinator.Engine, workflowID string, revision uint64, changed coordinator.Command) {
	t.Helper()
	if _, err := engine.Exchange(changed); coordinator.ErrorCode(err) != "WORKFLOW_IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("changed idempotency content error = %v", err)
	}
	inspected := exchangeWorkflow(t, engine, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV2, Kind: coordinator.CommandInspect, WorkflowID: workflowID,
	})
	if inspected.Revision != revision {
		t.Fatalf("idempotency rejection changed revision: got %d, want %d", inspected.Revision, revision)
	}
}

func stableCutoverResultBytes(t *testing.T, value coordinator.Result) []byte {
	t.Helper()
	value.Replayed = false
	value.Digest = ""
	if value.Snapshot != nil {
		snapshot := *value.Snapshot
		snapshot.ProcessedMessages = append([]coordinator.ProcessedMessage{}, snapshot.ProcessedMessages...)
		for index := range snapshot.ProcessedMessages {
			if snapshot.ProcessedMessages[index].Revision == value.Revision {
				snapshot.ProcessedMessages[index].ResultDigest = ""
			}
		}
		value.Snapshot = &snapshot
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func cloneCutoverCommand(t *testing.T, value coordinator.Command) coordinator.Command {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result coordinator.Command
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func readCutoverState(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[relative] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertNoCommittedCutoverState(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "LOCK" {
			return nil
		}
		t.Fatalf("non-Workflow request committed state file %s", path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func loadCurrentSessionFixture(t *testing.T) currentSessionFixture {
	t.Helper()
	raw := readCutoverFixture(t, "current-session.json")
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"token", "password", "authorization", "api_key", "mcp", "model_command"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("CURRENT session fixture contains %q", forbidden)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var fixture currentSessionFixture
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("CURRENT session fixture has trailing JSON: %v", err)
	}
	if fixture.SchemaVersion != "oaw.test-current-session/v1" {
		t.Fatalf("CURRENT session fixture schema = %q", fixture.SchemaVersion)
	}
	return fixture
}

func readCutoverFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "core-coordinator", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
