package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
	snapshot    config.Snapshot
	resolution  core.ResolutionResult
	inventory   host.BindingInventory
	session     host.SessionSnapshot
	environment host.EnvironmentReport
	decision    classification.ClassificationDecision
}

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
			request := cutoverCompilationRequest(facts, "SP-FULL")
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
			})
			if err != nil {
				t.Fatal(err)
			}
			command := workflowStartCommand(facts.session, facts.environment)
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
			compiled, err := core.Compile(cutoverCompilationRequest(facts, profileID))
			if err != nil {
				t.Fatal(err)
			}
			bundle := requireCutoverBundle(t, compiled)
			if bundle.Selection.Profile != profileID || bundle.Topology != execution.TopologyCurrent ||
				!slices.Contains(bundle.Graph.EligibleTopologies, execution.TopologyCurrent) {
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
	if !providerFound || !recipeFound || !resolutionFound || resolution.State != registry.Verified || resolution.Instance == nil {
		t.Fatalf("generic inputs were not loaded and resolved: provider=%t recipe=%t resolution=%#v", providerFound, recipeFound, resolution)
	}
	if provider.Capabilities[0].ID != "delivery" || recipe.Nodes[0].Selector.ProviderID != thirdPartyProvider {
		t.Fatalf("third-party descriptor or recipe changed: %#v / %#v", provider, recipe)
	}

	compiled, err := core.Compile(cutoverCompilationRequest(facts, thirdPartyProfile))
	if err != nil {
		t.Fatal(err)
	}
	bundle := requireCutoverBundle(t, compiled)
	if bundle.Selection.Profile != thirdPartyProfile || bundle.Graph.RecipeID != thirdPartyProfile || len(bundle.Graph.Nodes) != 4 {
		t.Fatalf("third-party Bundle = %#v", bundle)
	}
	for _, node := range bundle.Graph.Nodes {
		if node.ProviderID != thirdPartyProvider || node.CapabilityID != "delivery" || node.Binding.Reference != "acme:delivery" {
			t.Fatalf("third-party graph node = %#v", node)
		}
	}
	assertSingleResponsibilityOwners(t, facts.snapshot.Catalog(), bundle)
}

func TestCurrentWorkflowClosesThroughNormalizedReceipts(t *testing.T) {
	facts := thirdPartyCutoverFacts(t)
	stateRoot := filepath.Join(t.TempDir(), "workflow-state")
	projectRoot := t.TempDir()
	engine := newCutoverEngine(t, facts, stateRoot, projectRoot)

	startCommand := cutoverStartCommand(facts)
	current := exchangeCutoverReplay(t, engine, startCommand)
	if current.Kind != coordinator.ResultState || current.Snapshot == nil || current.Snapshot.Status != coordinator.StatusReady || current.Revision != 1 {
		t.Fatalf("START Result = %#v", current)
	}
	changedStart := cloneCutoverCommand(t, startCommand)
	changedStart.Start.ActiveTicket = "ticket-changed"
	assertCutoverIdempotencyReuse(t, engine, current.WorkflowID, current.Revision, changedStart)

	for current.Snapshot.Status != coordinator.StatusFinished {
		nodeID := current.Snapshot.ActiveNodeID
		prepare := coordinator.Command{
			SchemaVersion:    coordinator.WorkflowCommandSchemaV1,
			Kind:             coordinator.CommandPrepare,
			MessageID:        "message-prepare-" + nodeID,
			IdempotencyKey:   "prepare-" + nodeID,
			WorkflowID:       current.WorkflowID,
			ExpectedRevision: current.Revision,
			Prepare: &coordinator.PrepareInput{
				RequestedEffects:     []string{"read-project"},
				RequestedResources:   []string{"project-worktree"},
				TerminationCondition: "complete " + nodeID,
				InputReferences:      []coordinator.ArtifactReference{},
				EvidenceRequirements: []coordinator.EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "node completion report"}},
			},
		}
		prepared := exchangeCutoverReplay(t, engine, prepare)
		if prepared.Kind != coordinator.ResultDispatch || prepared.Dispatch == nil || prepared.Snapshot.Status != coordinator.StatusPrepared {
			t.Fatalf("PREPARE(%s) Result = %#v", nodeID, prepared)
		}
		changedPrepare := cloneCutoverCommand(t, prepare)
		changedPrepare.Prepare.TerminationCondition += " with changed content"
		assertCutoverIdempotencyReuse(t, engine, prepared.WorkflowID, prepared.Revision, changedPrepare)

		identity := cutoverReceiptIdentity(*prepared.Dispatch)
		startedCommand := coordinator.Command{
			SchemaVersion:    coordinator.WorkflowCommandSchemaV1,
			Kind:             coordinator.CommandReceipt,
			MessageID:        "message-started-" + nodeID,
			IdempotencyKey:   "started-" + nodeID,
			WorkflowID:       prepared.WorkflowID,
			ExpectedRevision: prepared.Revision,
			Receipt:          &coordinator.ReceiptInput{Receipt: hosttest.StartedReceipt(t, identity, "")},
		}
		inFlight := exchangeCutoverReplay(t, engine, startedCommand)
		if inFlight.Snapshot == nil || inFlight.Snapshot.Status != coordinator.StatusInFlight {
			t.Fatalf("STARTED(%s) Result = %#v", nodeID, inFlight)
		}
		changedStarted := cloneCutoverCommand(t, startedCommand)
		changedStarted.Receipt.StableBoundary = "changed-content"
		assertCutoverIdempotencyReuse(t, engine, inFlight.WorkflowID, inFlight.Revision, changedStarted)

		evidence := []host.EvidenceReference{{
			Kind: "report", Reference: "evidence://cutover/" + nodeID, Digest: strings.Repeat("e", 64),
		}}
		completedCommand := coordinator.Command{
			SchemaVersion:    coordinator.WorkflowCommandSchemaV1,
			Kind:             coordinator.CommandReceipt,
			MessageID:        "message-completed-" + nodeID,
			IdempotencyKey:   "completed-" + nodeID,
			WorkflowID:       inFlight.WorkflowID,
			ExpectedRevision: inFlight.Revision,
			Receipt: &coordinator.ReceiptInput{
				Receipt: hosttest.CompletedReceipt(t, identity, "", evidence),
				Signal:  cutoverCompletionSignal(current.Snapshot, nodeID),
			},
		}
		current = exchangeCutoverReplay(t, engine, completedCommand)
		changedCompleted := cloneCutoverCommand(t, completedCommand)
		changedCompleted.Receipt.StableBoundary = "changed-content"
		assertCutoverIdempotencyReuse(t, engine, current.WorkflowID, current.Revision, changedCompleted)
	}

	if current.Revision != 13 || current.Snapshot.ActiveGrant != nil || len(current.Snapshot.Receipts) != 8 ||
		current.Snapshot.ActiveNodeID != "verification" {
		t.Fatalf("terminal Workflow Result = %#v", current)
	}
	beforeInspect := readCutoverState(t, stateRoot)
	inspected := exchangeWorkflow(t, engine, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1,
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

	start := cutoverStartCommand(facts)
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
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: prepared.WorkflowID,
	})
	if preparedInspection.RevisionDigest != prepared.RevisionDigest || preparedInspection.Snapshot.Status != coordinator.StatusPrepared ||
		preparedInspection.Dispatch == nil || preparedInspection.Snapshot.ActiveGrant == nil {
		t.Fatalf("recovered PREPARED Result = %#v", preparedInspection)
	}

	identity := cutoverReceiptIdentity(*preparedInspection.Dispatch)
	started := exchangeWorkflow(t, afterPrepare, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandReceipt,
		MessageID: "message-recovery-started", IdempotencyKey: "recovery-started", WorkflowID: prepared.WorkflowID,
		ExpectedRevision: preparedInspection.Revision,
		Receipt:          &coordinator.ReceiptInput{Receipt: hosttest.StartedReceipt(t, identity, "")},
	})
	afterStarted := newCutoverEngine(t, facts, stateRoot, projectRoot)
	startedInspection := exchangeWorkflow(t, afterStarted, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: started.WorkflowID,
	})
	if startedInspection.RevisionDigest != started.RevisionDigest || startedInspection.Snapshot.Status != coordinator.StatusInFlight ||
		startedInspection.Snapshot.ActiveGrant == nil || len(startedInspection.Snapshot.Receipts) != 1 {
		t.Fatalf("recovered IN_FLIGHT Result = %#v", startedInspection)
	}

	pending := exchangeWorkflow(t, afterStarted, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandCancel,
		MessageID: "message-recovery-pending", IdempotencyKey: "recovery-pending", WorkflowID: started.WorkflowID,
		ExpectedRevision: startedInspection.Revision,
		Cancel:           &coordinator.CancelInput{Reason: "uncertain host termination", InvocationTerminal: false},
	})
	if pending.Snapshot.Status != coordinator.StatusPaused || pending.Snapshot.ActiveGrant == nil || pending.Snapshot.ResourceLeases[0].ReleasedRevision != 0 {
		t.Fatalf("pending uncertainty Result = %#v", pending)
	}
	afterPending := newCutoverEngine(t, facts, stateRoot, projectRoot)
	recoveredPending := exchangeWorkflow(t, afterPending, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: pending.WorkflowID,
	})
	if recoveredPending.RevisionDigest != pending.RevisionDigest || recoveredPending.Snapshot.Status != coordinator.StatusPaused || recoveredPending.Snapshot.ActiveGrant == nil {
		t.Fatalf("recovered uncertainty Result = %#v", recoveredPending)
	}
	confirmed := exchangeWorkflow(t, afterPending, coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandCancel,
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
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandCancel,
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
	home := t.TempDir()
	writeFile(t, home, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "superpowers")
	for _, skill := range []string{"to-spec", "to-tickets", "tdd", "diagnosing-bugs"} {
		writeFile(t, home, ".agents/skills/"+skill+"/SKILL.md", skill)
	}
	writeFile(t, home, ".agents/skills/everything-claude-code/SKILL.md", "ecc")
	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: builtInHostID, UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := cutoverBindingInventory(t, snapshot.Catalog(), discovered, "oaw/superpowers", "oaw/matt", "oaw/ecc")
	return resolveCutoverFacts(t, snapshot, builtInHostID, discovered, inventory)
}

func thirdPartyCutoverFacts(t *testing.T) cutoverCoreFacts {
	t.Helper()
	fixture := loadCurrentSessionFixture(t)
	if fixture.HostID != thirdPartyHostID || fixture.Topology != execution.TopologyCurrent {
		t.Fatalf("CURRENT session fixture = %#v", fixture)
	}
	userRoot := t.TempDir()
	for _, name := range []string{"acme-provider.json", "acme-profile.json"} {
		writeFile(t, userRoot, name, string(readCutoverFixture(t, name)))
	}
	writeFile(t, userRoot, "config.toml", string(readCutoverFixture(t, "user-config.toml")))
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	writeFile(t, home, ".agents/acme-engineering/SKILL.md", "acme engineering")
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
		inventory := workflowBindingInventory(t, available, discovered, providerID)
		observations = append(observations, inventory.Observations...)
	}
	inventory, err := host.NewBindingInventory(discovered.HostID(), observations)
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
	session := hosttest.CurrentSession(t, hostID, inventory.Digest)
	environment := hosttest.CurrentEnvironment(t, session)
	return cutoverCoreFacts{
		snapshot: snapshot, resolution: resolved, inventory: inventory,
		session: session, environment: environment, decision: decision,
	}
}

func newCutoverEngine(t *testing.T, facts cutoverCoreFacts, stateRoot, projectRoot string) *coordinator.Engine {
	t.Helper()
	engine, err := coordinator.NewEngine(coordinator.Options{
		StateRoot: stateRoot, PhysicalProjectRoot: projectRoot,
		Configuration: facts.snapshot, Resolutions: facts.resolution.Report, Registry: facts.resolution.Registry,
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
	nodeID := ready.Snapshot.ActiveNodeID
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandPrepare,
		MessageID: "message-" + key, IdempotencyKey: key, WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision,
		Prepare: &coordinator.PrepareInput{
			RequestedEffects: []string{effect}, RequestedResources: []string{"project-worktree"},
			TerminationCondition: "complete " + nodeID, InputReferences: []coordinator.ArtifactReference{},
			EvidenceRequirements: []coordinator.EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "recovery report"}},
		},
	}
}

func cutoverCompilationRequest(facts cutoverCoreFacts, profileID string) core.CompilationRequest {
	selection := core.Selection{
		Profile: profileID, ProfileSource: core.SelectionUser,
		Topology: execution.TopologyCurrent, TopologySource: core.SelectionHostOnlyOption,
		AddOns: []string{}, Bindings: []profile.ProfileBinding{},
	}
	return core.CompilationRequest{
		DeliverableID: "deliverable-cutover", InputDigest: strings.Repeat("a", 64), Generation: 1,
		Classification: facts.decision, Configuration: facts.snapshot,
		Resolutions: facts.resolution.Report, Registry: facts.resolution.Registry,
		HostID: facts.session.HostID, HostSessionDigest: facts.session.Digest,
		HostEnvironmentReportDigest: facts.environment.Digest, HostProviderInventoryDigest: facts.inventory.Digest,
		HostTopologies:          append([]execution.Topology{}, facts.session.SupportedTopologies...),
		EnvironmentObservations: append([]execution.EnvironmentObservation{}, facts.environment.Observations...),
		Selection:               &selection,
	}
}

func cutoverStartCommand(facts cutoverCoreFacts) coordinator.Command {
	return coordinator.Command{
		SchemaVersion:  coordinator.WorkflowCommandSchemaV1,
		Kind:           coordinator.CommandStart,
		MessageID:      "message-start-cutover",
		IdempotencyKey: "start-current-cutover",
		Start: &coordinator.StartInput{
			RequestID: "request-cutover", DeliverableID: "deliverable-cutover", InputDigest: strings.Repeat("a", 64),
			ActiveTicket: "ticket-cutover", Proposal: *workflowCutoverProposal(),
			Selection: core.Selection{
				Profile: thirdPartyProfile, ProfileSource: core.SelectionUser,
				Topology: execution.TopologyCurrent, TopologySource: core.SelectionHostOnlyOption,
				AddOns: []string{}, Bindings: []profile.ProfileBinding{},
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
	for _, responsibility := range recipe.RequiredResponsibilities {
		owners := 0
		for _, node := range bundle.Graph.Nodes {
			if node.Responsibility == responsibility {
				owners++
			}
		}
		if owners != 1 {
			t.Fatalf("responsibility %s has %d owners in %s", responsibility, owners, bundle.Selection.Profile)
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
		WorkflowID: packet.WorkflowID, BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest,
		NodeID: packet.NodeID, Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest,
		DispatchDigest: packet.Digest, EnvironmentReportDigest: packet.EnvironmentReportDigest,
	}
}

func cutoverCompletionSignal(snapshot *coordinator.Snapshot, nodeID string) string {
	for _, gate := range snapshot.Bundles[len(snapshot.Bundles)-1].Graph.TerminalGates {
		if gate == nodeID {
			return ""
		}
	}
	return "succeeded"
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
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: workflowID,
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
