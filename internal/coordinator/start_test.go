package coordinator

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gofrs/flock"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestCursorStartCompilesOnceInsideWorkflowLockAndCommitsExactBundle(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	command := startTestCommand(t, "start-once")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(command.IdempotencyKey)}
	engine, err := NewEngine(startTestOptions(t, stateRoot, compiler))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	result, err := engine.Exchange(command)
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}
	if compiler.classifyCalls != 1 || compiler.compileCalls != 1 {
		t.Fatalf("Core calls = classify %d, compile %d", compiler.classifyCalls, compiler.compileCalls)
	}
	if runtime.GOOS != "windows" && !compiler.compileInsideLock {
		t.Fatal("Core Compile did not observe the Workflow lock")
	}
	if result.Kind != ResultState || result.Snapshot == nil || result.Snapshot.Status != StatusReady ||
		result.Snapshot.ActiveGeneration != 1 || result.Snapshot.Cursor != firstStartTestCursor(t, compiler.bundle.Graph) {
		t.Fatalf("START Result = %#v", result)
	}
	if len(result.Snapshot.Bundles) != 1 || !reflect.DeepEqual(result.Snapshot.Bundles[0], compiler.bundle) {
		t.Fatalf("committed Bundle differs from Core Bundle\n got: %#v\nwant: %#v", result.Snapshot.Bundles, compiler.bundle)
	}
	committed, err := engine.journal.inspect(result.WorkflowID)
	if err != nil {
		t.Fatalf("inspect committed START: %v", err)
	}
	if !reflect.DeepEqual(committed.Result, result) {
		t.Fatalf("returned Result differs from committed Result\n got: %#v\nwant: %#v", result, committed.Result)
	}
	restartedCore := &startTestCore{t: t}
	restarted, err := NewEngine(startTestOptions(t, stateRoot, restartedCore))
	if err != nil {
		t.Fatalf("restart NewEngine() error = %v", err)
	}
	inspected, err := restarted.Exchange(Command{SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandInspect, WorkflowID: result.WorkflowID})
	if err != nil {
		t.Fatalf("restart INSPECT error = %v", err)
	}
	if inspected.Snapshot == nil || len(inspected.Snapshot.Bundles) != 1 || !reflect.DeepEqual(inspected.Snapshot.Bundles[0], compiler.bundle) {
		t.Fatalf("restart did not restore exact Core Bundle: %#v", inspected)
	}
}

func TestStartReplayDoesNotReclassifyOrRecompile(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	command := startTestCommand(t, "start-replay")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(command.IdempotencyKey)}
	engine, err := NewEngine(startTestOptions(t, stateRoot, compiler))
	if err != nil {
		t.Fatal(err)
	}
	first, err := engine.Exchange(command)
	if err != nil {
		t.Fatal(err)
	}
	command.MessageID = "message-retried"
	replayed, err := engine.Exchange(command)
	if err != nil {
		t.Fatalf("Exchange(replayed START) error = %v", err)
	}
	if compiler.classifyCalls != 1 || compiler.compileCalls != 1 {
		t.Fatalf("replay called Core: classify %d, compile %d", compiler.classifyCalls, compiler.compileCalls)
	}
	if !replayed.Replayed || replayed.Revision != first.Revision || replayed.RevisionDigest != first.RevisionDigest || replayed.Snapshot == nil {
		t.Fatalf("replayed Result = %#v, first = %#v", replayed, first)
	}
	replayedSnapshot, firstSnapshot := *replayed.Snapshot, *first.Snapshot
	replayedSnapshot.ProcessedMessages = clearResultPinForRevision(replayedSnapshot.ProcessedMessages, replayed.Revision)
	firstSnapshot.ProcessedMessages = clearResultPinForRevision(firstSnapshot.ProcessedMessages, first.Revision)
	if !reflect.DeepEqual(replayedSnapshot, firstSnapshot) {
		t.Fatalf("replayed snapshot changed authoritative state\n got: %#v\nwant: %#v", replayedSnapshot, firstSnapshot)
	}
}

func TestStartRejectsNonWorkflowWithoutWorkflowState(t *testing.T) {
	for _, mode := range []classification.RequestMode{classification.RequestModeDirect, classification.RequestModeBounded} {
		t.Run(string(mode), func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "state")
			command := startTestCommand(t, "start-"+strings.ToLower(string(mode)))
			compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(command.IdempotencyKey), decision: classificationDecision(t, mode)}
			engine, err := NewEngine(startTestOptions(t, stateRoot, compiler))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_CLASSIFICATION_REQUIRED" {
				t.Fatalf("Exchange(START) error = %v", err)
			}
			if compiler.classifyCalls != 1 || compiler.compileCalls != 0 {
				t.Fatalf("Core calls = classify %d, compile %d", compiler.classifyCalls, compiler.compileCalls)
			}
			assertNoCommittedWorkflow(t, engine.journal, compiler.workflowID)
		})
	}
}

func TestStartRejectsMismatchedCoreBundleWithoutWorkflowState(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(*core.LifecycleBundle)
		corruptBundle bool
		corruptResult bool
	}{
		{name: "selection", mutate: func(value *core.LifecycleBundle) { value.Selection.Profile = "MATT-FULL" }},
		{name: "host", mutate: func(value *core.LifecycleBundle) { value.HostID = "claude-code" }},
		{name: "host session", mutate: func(value *core.LifecycleBundle) { value.HostSessionDigest = strings.Repeat("f", 64) }},
		{name: "environment report", mutate: func(value *core.LifecycleBundle) { value.EnvironmentReportDigest = strings.Repeat("f", 64) }},
		{name: "provider inventory", mutate: func(value *core.LifecycleBundle) { value.ProviderInventoryDigest = strings.Repeat("f", 64) }},
		{name: "topology", mutate: func(value *core.LifecycleBundle) { value.Topology = execution.TopologySubagent }},
		{name: "deliverable", mutate: func(value *core.LifecycleBundle) { value.DeliverableID = "different" }},
		{name: "input", mutate: func(value *core.LifecycleBundle) { value.InputDigest = strings.Repeat("f", 64) }},
		{name: "generation", mutate: func(value *core.LifecycleBundle) { value.Generation = 2 }},
		{name: "graph entry", mutate: func(value *core.LifecycleBundle) { value.Graph.EntrySlotID = "missing" }},
		{name: "Bundle digest", corruptBundle: true},
		{name: "Compilation Result digest", corruptResult: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "state")
			command := startTestCommand(t, "start-mismatch-"+strings.ReplaceAll(test.name, " ", "-"))
			compiler := &startTestCore{
				t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(command.IdempotencyKey), mutateBundle: test.mutate,
				corruptBundleDigest: test.corruptBundle, corruptResultDigest: test.corruptResult,
			}
			engine, err := NewEngine(startTestOptions(t, stateRoot, compiler))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_CORE_RESULT_INVALID" {
				t.Fatalf("Exchange(START) error = %v", err)
			}
			if compiler.compileCalls != 1 {
				t.Fatalf("Core compile calls = %d", compiler.compileCalls)
			}
			assertNoCommittedWorkflow(t, engine.journal, compiler.workflowID)
		})
	}
}

func TestCallerAuthoredBundleIsUnknownStartField(t *testing.T) {
	raw := mustMarshalCommand(t, startTestCommand(t, "start-authored-bundle"))
	forged := strings.Replace(string(raw), `"request_id":`, `"bundle":{},"request_id":`, 1)
	if _, err := DecodeCommand([]byte(forged)); ErrorCode(err) != "WORKFLOW_COMMAND_DECODE_INVALID" {
		t.Fatalf("DecodeCommand(caller Bundle) error = %v", err)
	}
}

type startTestCore struct {
	t                   *testing.T
	stateRoot           string
	workflowID          string
	decision            classification.ClassificationDecision
	classifyCalls       int
	compileCalls        int
	compileInsideLock   bool
	mutateBundle        func(*core.LifecycleBundle)
	corruptBundleDigest bool
	corruptResultDigest bool
	bundle              core.LifecycleBundle
}

func (value *startTestCore) Classify(_ *classification.ClassificationProposal, _ classification.ClassificationRules) (classification.ClassificationDecision, error) {
	value.classifyCalls++
	if value.decision.RequestMode != "" {
		return value.decision, nil
	}
	return classificationDecision(value.t, classification.RequestModeWorkflow), nil
}

func (value *startTestCore) Compile(request core.CompilationRequest) (core.CompilationResult, error) {
	value.compileCalls++
	lock := flock.New(filepath.Join(value.stateRoot, workflowRecordsDirectory, value.workflowID, "LOCK"))
	locked, err := lock.TryLock()
	if err != nil {
		value.t.Fatalf("TryLock() error = %v", err)
	}
	if locked {
		_ = lock.Unlock()
		_ = lock.Close()
	} else {
		value.compileInsideLock = true
	}
	bundle := compiledStartTestBundle(request)
	if value.mutateBundle != nil {
		value.mutateBundle(&bundle)
		bundle.Graph.Digest = bundle.Graph.ContentDigest()
		sealStartTestBundle(&bundle)
	}
	if value.corruptBundleDigest {
		bundle.Digest = strings.Repeat("f", 64)
	}
	value.bundle = bundle
	result := core.CompilationResult{EligibleProfiles: []core.ProfileEligibility{}, EligibleAddOns: []core.AddOnEligibility{}, Bundle: &bundle}
	result.Digest = startTestDigest(result)
	if value.corruptResultDigest {
		result.Digest = strings.Repeat("f", 64)
	}
	return result, nil
}

func startTestCommand(t testing.TB, idempotencyKey string) Command {
	t.Helper()
	session, environment := startTestHostFacts(t)
	graphSelection := startTestGraphSelection()
	return Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandStart, MessageID: "message-start", IdempotencyKey: idempotencyKey,
		Start: &StartInput{
			RequestID: "request-1", DeliverableID: "deliverable-1", InputDigest: strings.Repeat("a", 64), ActiveTicket: "ticket-1",
			Proposal: classification.ClassificationProposal{SchemaVersion: classification.ProposalSchemaV1, Traits: []classification.TraitObservation{}, Resources: []classification.Resource{}, Evidence: []classification.ProposalEvidence{}},
			Selection: core.Selection{
				Profile: "SP-FULL", RecipeID: graphSelection.RecipeID, RecipeDigest: graphSelection.RecipeDigest,
				ProfileSource: core.SelectionUser, Topology: execution.TopologyCurrent, TopologySource: core.SelectionHostOnlyOption,
				AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}, GraphSelectionDigest: graphSelection.Digest,
			},
			HostSession: session, Environment: environment,
		},
	}
}

func compiledStartTestBundle(request core.CompilationRequest) core.LifecycleBundle {
	graphSelection := startTestGraphSelection()
	graphSelection.Profile = request.Selection.Profile
	graphSelection.Topology = request.Selection.Topology
	graphSelection.AddOns = append([]string{}, request.Selection.AddOns...)
	graphSelection.Alternatives = append([]profile.AlternativeChoice{}, request.Selection.Alternatives...)
	graphSelection.Overlays = append([]string{}, request.Selection.Overlays...)
	graphSelection.Digest = ""
	graphSelection.Digest = startTestDigest(graphSelection)
	providerInstanceDigest := strings.Repeat("b", 64)
	slots := startTestSlots(providerInstanceDigest, request.Selection.Topology)
	hostRecord := request.Host.Record()
	graph := profile.ExecutionGraphRecord{
		SchemaVersion: profile.ExecutionGraphSchemaV4, HostID: hostRecord.HostID, HostEvidenceDigest: hostRecord.Digest,
		RegistryDigest: request.Registry.Digest(), TaxonomyVersion: catalog.TaxonomyVersionV1,
		RecipeID: graphSelection.RecipeID, RecipeVersion: "3.0.0", RecipeDigest: graphSelection.RecipeDigest, Selection: graphSelection,
		ProviderInstances: []profile.GraphProviderInstance{{ProviderID: "oaw/superpowers", HostID: hostRecord.HostID, InstanceDigest: providerInstanceDigest}},
		EntrySlotID:       catalog.SlotProblemFraming, Slots: slots, IncidentRoutes: []profile.CompiledIncidentRoute{},
		StableBoundaries: []string{"closeout", "implementation", "review-remediation"}, Topology: request.Selection.Topology,
		EnvironmentRequirements: []execution.EnvironmentRequirement{}, Decisions: []profile.CompileDecision{},
	}
	graph.Digest = graph.ContentDigest()
	recipe := startTestRecipe()
	bundle := core.LifecycleBundle{
		SchemaVersion: core.LifecycleBundleSchemaV4, ID: "bundle-0123456789abcdef0123456789abcdef", DeliverableID: request.DeliverableID, InputDigest: request.InputDigest,
		Generation: request.Generation, Classification: request.Classification, ClassificationDigest: request.Classification.Digest(), Selection: *request.Selection,
		Recipe: recipe, RecipeDigest: graphSelection.RecipeDigest,
		HostID: hostRecord.HostID, HostSessionDigest: hostRecord.SessionDigest, HostManifestDigest: hostRecord.ManifestDigest,
		EnvironmentReportDigest: hostRecord.EnvironmentDigest, ProviderInventoryDigest: hostRecord.InventoryDigest,
		HostFeatureDigest: hostRecord.FeatureDigest, HostActionDigest: hostRecord.ActionDigest, HostEvidenceDigest: hostRecord.Digest,
		Configuration: request.Configuration.Record(), ResolutionDigest: request.ResolutionDigest, RegistryDigest: request.Registry.Digest(),
		ProviderInstances: graph.ProviderInstances, Graph: graph, Topology: request.Selection.Topology,
		EnvironmentRequirements: []execution.EnvironmentRequirement{}, AddOns: append([]string{}, request.Selection.AddOns...),
	}
	sealStartTestBundle(&bundle)
	return bundle
}

func startTestGraphSelection() profile.Selection {
	value := profile.Selection{
		Profile: "SP-FULL", RecipeID: "test/delivery", RecipeDigest: strings.Repeat("c", 64), Topology: execution.TopologyCurrent,
		AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{},
	}
	value.Digest = startTestDigest(value)
	return value
}

func startTestRecipe() catalog.ProfileRecipeRecord {
	return catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV3, TaxonomyVersion: catalog.TaxonomyVersionV1, RecipeVersion: "3.0.0",
		ID: "test/delivery", DisplayName: "Coordinator test delivery", Family: "test", Slots: []catalog.SlotRecipe{},
		AddOns: []catalog.AddOnRecord{}, IncidentRoutes: []catalog.IncidentRoute{}, Overlays: []catalog.OverlayRecord{},
		StableBoundaries: []string{"closeout", "implementation", "review-remediation"}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
}

func startTestSlots(providerDigest string, topology execution.Topology) []profile.CompiledSlot {
	definitions := catalog.CanonicalSlots()
	result := make([]profile.CompiledSlot, len(definitions))
	for index, definition := range definitions {
		unitID := "unit-" + string(definition.ID)
		cursor, _ := execution.NewGraphCursor(string(definition.ID), execution.CursorBinding, unitID, 1)
		binding := profile.ResolvedBinding{
			Cursor: cursor, UnitID: unitID, StepID: "step-" + string(definition.ID), AnchorSlotID: definition.ID,
			SlotIDs: []catalog.SlotID{definition.ID}, ProviderID: "oaw/superpowers", ProviderInstanceDigest: providerDigest,
			BindingID: "codex-brainstorming", DistributionID: "superpowers", DistributionRevision: strings.Repeat("1", 40),
			DistributionTreeDigest: "sha256:" + strings.Repeat("2", 64), Surface: "codex-plugin", Kind: catalog.BindingSkill,
			Reference: "superpowers:brainstorming", Invocation: catalog.InvocationModel, BindingTreeDigest: "sha256:" + strings.Repeat("3", 64),
			InputArtifact: "oaw.workflow-artifact/v1", OutputArtifact: "oaw.workflow-artifact/v1",
			Responsibilities: []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: string(definition.ID), SlotID: definition.ID, OutcomeOwner: true}},
			MaximumEffects:   []string{"read-project", "write-project"}, Resources: []string{"project-worktree"}, SupportedTopologies: []execution.Topology{topology},
			RequiredFeatures: []host.FeatureID{}, FeatureEvidenceDigests: []string{}, Disposition: profile.DispatchByCoordinator,
			BindingEvidenceDigest: strings.Repeat("4", 64),
		}
		owner := profile.CompiledOwner{Kind: catalog.OwnerProviderBinding, UnitID: unitID, ProviderID: binding.ProviderID, BindingID: binding.BindingID}
		if definition.ID == catalog.SlotIncidentRecovery {
			owner = profile.CompiledOwner{Kind: catalog.OwnerNone}
		}
		slot := profile.CompiledSlot{
			SlotID: definition.ID, Applicability: catalog.SlotMandatory, Active: true,
			EntryArtifact: "oaw.workflow-artifact/v1", OutcomeArtifact: "oaw.workflow-artifact/v1", OutcomeOwner: owner,
			Pipeline: []profile.ResolvedBinding{binding}, Gates: []profile.CompiledGate{}, Transitions: []profile.GraphTransition{}, Traversal: []execution.GraphCursor{cursor},
		}
		if definition.ID == catalog.SlotCloseout {
			gateCursor, _ := execution.NewGraphCursor(string(definition.ID), execution.CursorGate, "user-closeout", 2)
			slot.Gates = []profile.CompiledGate{{Cursor: gateCursor, ID: "user-closeout", Authority: catalog.GateUser, Predicate: "user-authorized", EvidenceRequirements: []catalog.EvidenceRequirementRecord{}}}
			terminal, _ := execution.NewGraphCursor(string(definition.ID), execution.CursorTerminal, "terminal:"+string(definition.ID), 3)
			slot.Terminal = true
			slot.Traversal = append(slot.Traversal, gateCursor, terminal)
		} else if definition.ID != catalog.SlotIncidentRecovery {
			next := nextStartTestSlot(definition.ID)
			slot.Transitions = []profile.GraphTransition{{Signal: "succeeded", Target: next}}
		}
		result[index] = slot
	}
	return result
}

func nextStartTestSlot(current catalog.SlotID) catalog.SlotID {
	switch current {
	case catalog.SlotProblemFraming:
		return catalog.SlotSolutionSpecification
	case catalog.SlotSolutionSpecification:
		return catalog.SlotDeliveryPlanning
	case catalog.SlotDeliveryPlanning:
		return catalog.SlotWorkspacePreparation
	case catalog.SlotWorkspacePreparation:
		return catalog.SlotImplementation
	case catalog.SlotImplementation:
		return catalog.SlotImplementationTDD
	case catalog.SlotImplementationTDD:
		return catalog.SlotReviewRemediation
	case catalog.SlotReviewRemediation:
		return catalog.SlotFreshVerification
	default:
		return catalog.SlotCloseout
	}
}

func firstStartTestCursor(t testing.TB, graph profile.ExecutionGraphRecord) execution.GraphCursor {
	t.Helper()
	cursor, err := profile.FirstActionableCursor(graph)
	if err != nil {
		t.Fatal(err)
	}
	return cursor
}

func firstStartTestBinding(t testing.TB, bundle *core.LifecycleBundle) *profile.ResolvedBinding {
	t.Helper()
	cursor := firstStartTestCursor(t, bundle.Graph)
	for slotIndex := range bundle.Graph.Slots {
		for bindingIndex := range bundle.Graph.Slots[slotIndex].Pipeline {
			binding := &bundle.Graph.Slots[slotIndex].Pipeline[bindingIndex]
			if binding.Cursor == cursor {
				return binding
			}
		}
	}
	t.Fatalf("first actionable cursor %#v has no Provider Binding", cursor)
	return nil
}

func sealStartTestBundle(bundle *core.LifecycleBundle) {
	bundle.Digest = ""
	bundle.Digest = startTestDigest(*bundle)
}

func classificationDecision(t testing.TB, mode classification.RequestMode) classification.ClassificationDecision {
	t.Helper()
	if mode == classification.RequestModeWorkflow {
		decision, err := classification.Classify(nil, classification.ClassificationRules{})
		if err != nil {
			t.Fatal(err)
		}
		return decision
	}
	traits := []classification.TraitObservation{
		{Trait: classification.TraitScopeClear, Value: classification.TraitTrue},
		{Trait: classification.TraitChangePointKnown, Value: classification.TraitTrue},
		{Trait: classification.TraitRecoverable, Value: classification.TraitTrue},
		{Trait: classification.TraitFocusedVerificationKnown, Value: classification.TraitTrue},
		{Trait: classification.TraitBoundedCapabilityRequest, Value: classification.TraitFalse},
		{Trait: classification.TraitArchitectureDecision, Value: classification.TraitFalse},
		{Trait: classification.TraitPublicContractChange, Value: classification.TraitFalse},
		{Trait: classification.TraitSchemaChange, Value: classification.TraitFalse},
		{Trait: classification.TraitDependencyChange, Value: classification.TraitFalse},
		{Trait: classification.TraitSecuritySensitive, Value: classification.TraitFalse},
		{Trait: classification.TraitDataSensitive, Value: classification.TraitFalse},
		{Trait: classification.TraitDeploymentChange, Value: classification.TraitFalse},
		{Trait: classification.TraitDomainUncertainty, Value: classification.TraitFalse},
		{Trait: classification.TraitRootCauseUncertain, Value: classification.TraitFalse},
		{Trait: classification.TraitMultipleResponsibilities, Value: classification.TraitFalse},
		{Trait: classification.TraitMultipleTickets, Value: classification.TraitFalse},
		{Trait: classification.TraitLongLivedDelegation, Value: classification.TraitFalse},
		{Trait: classification.TraitDestructiveMutation, Value: classification.TraitFalse},
		{Trait: classification.TraitCriticalRelease, Value: classification.TraitFalse},
	}
	proposal := classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1, Traits: traits, Resources: []classification.Resource{classification.ResourceProject},
		Evidence: []classification.ProposalEvidence{
			{Kind: classification.EvidenceScope, Reference: "scope", Digest: strings.Repeat("1", 64)},
			{Kind: classification.EvidenceChangePoint, Reference: "change", Digest: strings.Repeat("2", 64)},
			{Kind: classification.EvidenceVerification, Reference: "verify", Digest: strings.Repeat("3", 64)},
		},
	}
	if mode == classification.RequestModeBounded {
		for index := range proposal.Traits {
			if proposal.Traits[index].Trait == classification.TraitBoundedCapabilityRequest {
				proposal.Traits[index].Value = classification.TraitTrue
			}
		}
		proposal.CapabilitySelector = &classification.CapabilitySelector{ProviderID: "oaw/superpowers", CapabilityID: "delivery", Source: classification.SelectorUserIntent}
	}
	decision, err := classification.Classify(&proposal, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != mode {
		t.Fatalf("classification mode = %s, want %s", decision.RequestMode, mode)
	}
	return decision
}

func startTestHostFacts(t testing.TB) (host.SessionSnapshot, host.EnvironmentReport) {
	t.Helper()
	_, session, _, environment, _ := startTestHostEvidence(t)
	return session, environment
}

func startTestHostEvidence(t testing.TB) (host.Manifest, host.SessionSnapshot, host.BindingInventory, host.EnvironmentReport, profile.HostEvidence) {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}, DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-1", Topology: execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "3.0.0",
		SessionID: "session-1", ManifestDigest: manifest.Digest, SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, ProviderInventoryDigest: inventory.Digest,
		FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, session, inventory, environment, evidence
}

func startTestOptions(t testing.TB, stateRoot string, compiler CoreCompiler) Options {
	t.Helper()
	_, _, inventory, _, evidence := startTestHostEvidence(t)
	emptyCatalog, err := catalog.New(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(emptyCatalog, discovery.Options{HostID: "codex", UserHome: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var snapshot config.Snapshot
	resolutions, effective, err := registry.Resolve(snapshot, "codex", report, &inventory)
	if err != nil {
		t.Fatal(err)
	}
	return Options{
		StateRoot: stateRoot, Core: compiler, Configuration: snapshot, Resolutions: resolutions, Registry: effective, Host: evidence,
		capabilityResolver: func(binding profile.ResolvedBinding) *catalog.CapabilityRecord {
			return &catalog.CapabilityRecord{
				ID: "cap-" + binding.BindingID, InputSchema: binding.InputArtifact, OutcomeSchema: binding.OutputArtifact,
				RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{binding.BindingID},
			}
		},
	}
}

func assertNoCommittedWorkflow(t testing.TB, journal *journal, workflowID string) {
	t.Helper()
	if _, err := journal.inspect(workflowID); ErrorCode(err) != "WORKFLOW_NOT_FOUND" {
		t.Fatalf("rejected START state error = %v", err)
	}
}

func mustMarshalCommand(t testing.TB, value Command) []byte {
	t.Helper()
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func startTestDigest(value any) string {
	digest, _, _ := canonicaljson.Digest(value)
	return digest
}
