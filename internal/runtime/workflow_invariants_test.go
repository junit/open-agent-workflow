package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestWorkflowStateValidationRejectsSemanticTampering(t *testing.T) {
	states := internalWorkflowLifecycleRecords(t)
	tests := []struct {
		name   string
		base   revisionRecord
		mutate func(*revisionRecord)
	}{
		{"classification", states.ready, func(value *revisionRecord) { value.Snapshot.RequestMode = classification.RequestModeDirect }},
		{"status", states.ready, func(value *revisionRecord) { value.Snapshot.Status = "TAMPERED" }},
		{"identity", states.ready, func(value *revisionRecord) { value.Snapshot.RequestID = "bad\nrequest" }},
		{"trusted inputs", states.ready, func(value *revisionRecord) { value.Snapshot.Workflow.RegistryDigest = "bad" }},
		{"active ticket alias", states.ready, func(value *revisionRecord) {
			value.Snapshot.Workflow.ActiveTicket = value.Snapshot.Workflow.Input.DeliverableID
		}},
		{"authority collections", states.ready, func(value *revisionRecord) { value.Snapshot.ProcessedMessages = nil }},
		{"projection authority leakage", states.ready, func(value *revisionRecord) {
			value.Snapshot.Workflow.ProjectionLag = []ProjectionLag{{Revision: 2, Digest: strings.Repeat("1", 64), Reason: projectionFailureReason}}
		}},
		{"classification details", states.ready, func(value *revisionRecord) { value.Snapshot.Classification.WorkflowComplexity = nil }},
		{"message", states.ready, func(value *revisionRecord) { value.Snapshot.ProcessedMessages[0].IdempotencyKey = "bad\nkey" }},
		{"current message", states.ready, func(value *revisionRecord) { value.IdempotencyKey = "different" }},
		{"selection state", states.awaiting, func(value *revisionRecord) { value.Snapshot.Workflow.ActiveNodeID = "requirements" }},
		{"Bundle collection", states.ready, func(value *revisionRecord) { value.Snapshot.Workflow.Bundles = nil }},
		{"Bundle content", states.ready, func(value *revisionRecord) { value.Snapshot.Workflow.Bundles[0].Digest = strings.Repeat("0", 64) }},
		{"Bundle Host pin", states.ready, func(value *revisionRecord) {
			value.Snapshot.Workflow.Bundles[0].HostIntegrationDigest = strings.Repeat("0", 64)
		}},
		{"Bundle Host record", states.ready, func(value *revisionRecord) {
			bundle := &value.Snapshot.Workflow.Bundles[0]
			for index := range bundle.Configuration.HostIntegrations {
				if bundle.Configuration.HostIntegrations[index].ID == bundle.HostIntegrationID {
					bundle.Configuration.HostIntegrations[index].Audit.Digest = strings.Repeat("0", 64)
				}
			}
		}},
		{"active Bundle", states.ready, func(value *revisionRecord) { value.Snapshot.Workflow.ActiveGeneration = 2 }},
		{"active node", states.ready, func(value *revisionRecord) { value.Snapshot.Workflow.ActiveNodeID = "missing" }},
		{"stable boundary", states.ready, func(value *revisionRecord) { value.Snapshot.Workflow.LastStableBoundary = "missing" }},
		{"Grant compatibility", states.granted, func(value *revisionRecord) { value.Snapshot.GrantIDs = nil }},
		{"Grant content", states.granted, func(value *revisionRecord) { value.Snapshot.Grants[0].Digest = strings.Repeat("0", 64) }},
		{"Grant ID", states.granted, func(value *revisionRecord) { value.Snapshot.GrantIDs[0] = "grant-other" }},
		{"duplicate Grant", states.granted, func(value *revisionRecord) {
			value.Snapshot.Grants = append(value.Snapshot.Grants, value.Snapshot.Grants[0])
			value.Snapshot.GrantIDs = append(value.Snapshot.GrantIDs, value.Snapshot.GrantIDs[0])
		}},
		{"revoked Grant", states.switched, func(value *revisionRecord) { value.Snapshot.Workflow.RevokedGrantIDs = []string{"grant-missing"} }},
		{"duplicate revocation", states.switched, func(value *revisionRecord) {
			value.Snapshot.Workflow.RevokedGrantIDs = append(value.Snapshot.Workflow.RevokedGrantIDs, value.Snapshot.Workflow.RevokedGrantIDs[0])
		}},
		{"active Grant missing", states.granted, func(value *revisionRecord) { value.Snapshot.Workflow.ActiveGrantID = "grant-missing" }},
		{"active Grant outside node", states.granted, func(value *revisionRecord) { value.Snapshot.Workflow.ActiveNodeID = "domain-modeling" }},
		{"inactive active Grant", states.ready, func(value *revisionRecord) { value.Snapshot.Workflow.ActiveGrantID = "grant-stale" }},
		{"observation validation", states.observed, func(value *revisionRecord) { value.Snapshot.Workflow.Observations[0].RawOutput = "raw" }},
		{"lease validation", states.incident, func(value *revisionRecord) {
			value.Snapshot.Workflow.ResourceLeases[0].Digest = strings.Repeat("0", 64)
		}},
		{"reply collections", states.ready, func(value *revisionRecord) { value.Reply.Diagnostics = nil }},
		{"ready reply", states.ready, func(value *revisionRecord) { value.Reply.Kind = ReplyPaused }},
		{"Grant reply", states.granted, func(value *revisionRecord) { value.Event = "WRONG" }},
		{"dispatch reply", states.inflight, func(value *revisionRecord) { value.Reply.Kind = ReplyModeDecided }},
		{"paused reply", states.paused, func(value *revisionRecord) { value.Reply.RecoveryActions = nil }},
		{"uncertainty reply", states.paused, func(value *revisionRecord) { value.Event = "WRONG" }},
		{"finished reply", states.finished, func(value *revisionRecord) { value.Event = "WRONG" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneInternalWorkflowRecord(test.base)
			test.mutate(&candidate)
			assertInternalErrorCode(t, validateWorkflowState(candidate), "RUN_STATE_REVISION_INVALID")
		})
	}
}

func TestWorkflowObservationAndLeaseValidationRejectsTampering(t *testing.T) {
	states := internalWorkflowLifecycleRecords(t)
	for _, test := range []struct {
		name   string
		mutate func(*RunSnapshot)
	}{
		{"observation normalization", func(value *RunSnapshot) { value.Workflow.Observations[0].RawOutput = "raw" }},
		{"observation Grant identity", func(value *RunSnapshot) { value.Workflow.Observations[0].ExecutorID = "other" }},
		{"duplicate observation", func(value *RunSnapshot) {
			value.Workflow.Observations = append(value.Workflow.Observations, value.Workflow.Observations[0])
		}},
		{"observation Bundle", func(value *RunSnapshot) { value.Grants[0].BundleID = "bundle-missing" }},
		{"observation node", func(value *RunSnapshot) { value.Grants[0].NodeID = "node-missing" }},
		{"observation signal", func(value *RunSnapshot) { value.Workflow.Observations[0].Signal = workflowSignalSecurityFinding }},
		{"observation boundary", func(value *RunSnapshot) { value.Workflow.Observations[0].StableBoundary = "missing" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := cloneSnapshot(states.observed.Snapshot)
			test.mutate(&snapshot)
			assertInternalErrorCode(t, validateWorkflowObservations(snapshot, states.observed), "RUN_STATE_REVISION_INVALID")
		})
	}

	validLease := states.incident.Snapshot.Workflow.ResourceLeases[0]
	otherLease := states.paused.Snapshot.Workflow.ResourceLeases[0]
	for _, test := range []struct {
		name   string
		mutate func(*revisionRecord)
	}{
		{"missing Workflow", func(value *revisionRecord) { value.Snapshot.Workflow = nil }},
		{"too many active", func(value *revisionRecord) {
			value.Snapshot.ResourceLeaseIDs = append(value.Snapshot.ResourceLeaseIDs, value.Snapshot.ResourceLeaseIDs[0])
		}},
		{"invalid lease", func(value *revisionRecord) {
			value.Snapshot.Workflow.ResourceLeases[0].Digest = strings.Repeat("0", 64)
		}},
		{"lease exceeds Run", func(value *revisionRecord) { value.Snapshot.Workflow.ResourceLeases = []ResourceLease{otherLease} }},
		{"duplicate lease", func(value *revisionRecord) {
			value.Snapshot.Workflow.ResourceLeases = append(value.Snapshot.Workflow.ResourceLeases, validLease)
		}},
		{"lease Grant missing", func(value *revisionRecord) { value.Snapshot.Grants = nil }},
		{"lease Bundle missing", func(value *revisionRecord) { value.Snapshot.Workflow.Bundles = nil }},
		{"duplicate active lease", func(value *revisionRecord) { value.Snapshot.ResourceLeaseIDs = []string{validLease.ID, validLease.ID} }},
		{"active lease missing", func(value *revisionRecord) {
			value.Snapshot.ResourceLeaseIDs = []string{"lease-0123456789abcdef0123456789abcdef"}
		}},
		{"active Grant no longer writes", func(value *revisionRecord) { value.Snapshot.Grants[0].Effects = []string{"read-project"} }},
		{"active Bundle generation", func(value *revisionRecord) { value.Snapshot.Workflow.ActiveGeneration = 2 }},
		{"non-resumable status", func(value *revisionRecord) { value.Snapshot.Status = RunFinished }},
		{"ready active Grant", func(value *revisionRecord) { value.Snapshot.Workflow.ActiveGrantID = value.Snapshot.Grants[0].ID }},
		{"ready successful observation", func(value *revisionRecord) { value.Snapshot.Workflow.Observations[0].Outcome = ObservationSucceeded }},
	} {
		t.Run("lease "+test.name, func(t *testing.T) {
			candidate := cloneInternalWorkflowRecord(states.incident)
			test.mutate(&candidate)
			assertInternalErrorCode(t, validateWorkflowResourceLeases(candidate), "RUN_STATE_REVISION_INVALID")
		})
	}
}

func TestWorkflowRevisionEdgesRejectHistoryRewrites(t *testing.T) {
	states := internalWorkflowLifecycleRecords(t)
	for _, pair := range [][2]RunSnapshot{
		{states.awaiting.Snapshot, states.ready.Snapshot},
		{states.ready.Snapshot, states.granted.Snapshot},
		{states.granted.Snapshot, states.inflight.Snapshot},
		{states.inflight.Snapshot, states.observed.Snapshot},
		{states.observed.Snapshot, states.switched.Snapshot},
		{states.pausedPrevious.Snapshot, states.paused.Snapshot},
	} {
		if err := validateWorkflowRevisionTransition(pair[0], pair[1]); err != nil {
			t.Fatalf("valid Workflow revision edge rejected: %v", err)
		}
	}

	for _, test := range []struct {
		name     string
		previous RunSnapshot
		current  RunSnapshot
		mutate   func(*RunSnapshot, *RunSnapshot)
	}{
		{"missing Workflow", states.ready.Snapshot, states.granted.Snapshot, func(previous, _ *RunSnapshot) { previous.Workflow = nil }},
		{"unsupported edge", states.ready.Snapshot, states.ready.Snapshot, func(_, _ *RunSnapshot) {}},
		{"selection status", states.awaiting.Snapshot, states.ready.Snapshot, func(_, current *RunSnapshot) { current.Status = RunGranted }},
		{"selection identity", states.awaiting.Snapshot, states.ready.Snapshot, func(_, current *RunSnapshot) { current.Workflow.Input.DeliverableID = "changed" }},
		{"Grant append", states.ready.Snapshot, states.granted.Snapshot, func(_, current *RunSnapshot) { current.GrantIDs = nil }},
		{"Grant state", states.ready.Snapshot, states.granted.Snapshot, func(_, current *RunSnapshot) { current.Workflow.LastStableBoundary = "changed" }},
		{"Grant graph node", states.ready.Snapshot, states.granted.Snapshot, func(_, current *RunSnapshot) { current.Grants[len(current.Grants)-1].NodeID = "missing" }},
		{"dispatch history", states.granted.Snapshot, states.inflight.Snapshot, func(_, current *RunSnapshot) { current.GrantIDs = nil }},
		{"observation status", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.Status = RunGranted }},
		{"observation history", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.GrantIDs = nil }},
		{"uncertainty identity", states.pausedPrevious.Snapshot, states.paused.Snapshot, func(_, current *RunSnapshot) { current.Workflow.ActiveNodeID = "changed" }},
		{"observation append", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.Workflow.Observations = nil }},
		{"observation orchestration", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.Workflow.RegistryDigest = strings.Repeat("0", 64) }},
		{"observation active Grant", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.Workflow.Observations[0].GrantID = "grant-missing" }},
		{"observation graph identity", states.inflight.Snapshot, states.observed.Snapshot, func(previous, _ *RunSnapshot) { previous.Workflow.ActiveNodeID = "domain-modeling" }},
		{"observation Bundle", states.inflight.Snapshot, states.observed.Snapshot, func(previous, _ *RunSnapshot) { previous.Workflow.Bundles = nil }},
		{"observation node", states.inflight.Snapshot, states.observed.Snapshot, func(previous, _ *RunSnapshot) { previous.Grants[0].NodeID = "missing" }},
		{"observation target", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.Workflow.ActiveNodeID = "implementation" }},
		{"observation terminal state", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.Status = RunFinished }},
		{"observation lease", states.inflight.Snapshot, states.observed.Snapshot, func(previous, current *RunSnapshot) {
			current.ResourceLeaseIDs = append([]string{}, previous.ResourceLeaseIDs...)
		}},
		{"observation boundary", states.inflight.Snapshot, states.observed.Snapshot, func(_, current *RunSnapshot) { current.Workflow.LastStableBoundary = "other" }},
		{"switch shape", states.observed.Snapshot, states.switched.Snapshot, func(_, current *RunSnapshot) { current.Workflow.LastStableBoundary = "retained" }},
		{"switch revocations", states.observed.Snapshot, states.switched.Snapshot, func(_, current *RunSnapshot) { current.Workflow.RevokedGrantIDs = nil }},
		{"switch immutable history", states.observed.Snapshot, states.switched.Snapshot, func(_, current *RunSnapshot) { current.Workflow.Input.DeliverableID = "changed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			previous := cloneSnapshot(test.previous)
			current := cloneSnapshot(test.current)
			test.mutate(&previous, &current)
			assertInternalErrorCode(t, validateWorkflowRevisionTransition(previous, current), "RUN_STATE_REVISION_INVALID")
		})
	}

	grant := states.granted.Snapshot.Grants[0]
	lease := states.granted.Snapshot.Workflow.ResourceLeases[0]
	observation := states.observed.Snapshot.Workflow.Observations[0]
	bundle := states.ready.Snapshot.Workflow.Bundles[0]
	if appendOnlyWorkflowGrants([]admission.CapabilityGrant{grant}, nil) || appendOnlyWorkflowGrants([]admission.CapabilityGrant{grant}, []admission.CapabilityGrant{grant, {ID: "other"}}) == false {
		t.Fatal("Workflow Grant append-only helper is inconsistent")
	}
	if appendOnlyStrings([]string{"one"}, nil) || appendOnlyStrings([]string{"one"}, []string{"changed", "two"}) || !appendOnlyStrings([]string{"one"}, []string{"one", "two"}) {
		t.Fatal("string append-only helper is inconsistent")
	}
	if appendOnlyWorkflowLeases([]ResourceLease{lease}, nil) || appendOnlyWorkflowLeases([]ResourceLease{lease}, []ResourceLease{{ID: "changed"}}) || !appendOnlyWorkflowLeases([]ResourceLease{lease}, []ResourceLease{lease}) {
		t.Fatal("Resource Lease append-only helper is inconsistent")
	}
	if appendOnlyStageObservations([]StageObservation{observation}, nil) || appendOnlyStageObservations([]StageObservation{observation}, []StageObservation{{Signal: "changed"}}) || !appendOnlyStageObservations([]StageObservation{observation}, []StageObservation{observation}) {
		t.Fatal("Stage Observation append-only helper is inconsistent")
	}
	if appendOnlyLifecycleBundles([]LifecycleBundle{bundle}, nil) || appendOnlyLifecycleBundles([]LifecycleBundle{bundle}, []LifecycleBundle{bundle, {ID: "other"}}) == false {
		t.Fatal("Lifecycle Bundle append-only helper is inconsistent")
	}
}

type internalWorkflowRecords struct {
	awaiting       revisionRecord
	ready          revisionRecord
	granted        revisionRecord
	inflight       revisionRecord
	observed       revisionRecord
	switched       revisionRecord
	incident       revisionRecord
	pausedPrevious revisionRecord
	paused         revisionRecord
	finished       revisionRecord
}

func internalWorkflowLifecycleRecords(t *testing.T) internalWorkflowRecords {
	t.Helper()
	engine, fixture := newInternalWorkflowEngine(t)
	awaiting := startInternalWorkflow(t, engine, fixture, "a-main")
	ready := selectInternalWorkflow(t, engine, awaiting, "b-main-select")
	granted := grantInternalWorkflow(t, engine, ready, "c-main-grant")
	inflight := prepareInternalWorkflow(t, engine, granted, "d-main-dispatch")
	observed := observeInternalWorkflow(t, engine, inflight, "e-main-observe", ObservationSucceeded, workflowSignalSucceeded, "specification-approved")
	switchedReply, err := engine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: "f-main-switch", IdempotencyKey: "f-main-switch",
		RunID: observed.RunID, ExpectedRevision: observed.Revision,
		Continue: &ContinueInput{Signal: SignalSwitchProfile, StableBoundarySwitch: &StableBoundarySwitch{Boundary: "specification-approved", Selection: ProfileSelection{Profile: "SP-FULL"}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	incidentEngine, incidentFixture := newInternalWorkflowEngine(t)
	incidentStart := startInternalWorkflow(t, incidentEngine, incidentFixture, "g-incident")
	incidentReady := selectInternalWorkflow(t, incidentEngine, incidentStart, "h-incident-select")
	incidentGranted := grantInternalWorkflow(t, incidentEngine, incidentReady, "i-incident-grant")
	incidentFlight := prepareInternalWorkflow(t, incidentEngine, incidentGranted, "j-incident-dispatch")
	incident := observeInternalWorkflow(t, incidentEngine, incidentFlight, "k-incident-observe", ObservationFailed, workflowSignalFunctionalFailure, "")

	pausedEngine, pausedFixture := newInternalWorkflowEngine(t)
	pausedStart := startInternalWorkflow(t, pausedEngine, pausedFixture, "l-paused")
	pausedReady := selectInternalWorkflow(t, pausedEngine, pausedStart, "m-paused-select")
	pausedGranted := grantInternalWorkflow(t, pausedEngine, pausedReady, "n-paused-grant")
	pausedFlight := prepareInternalWorkflow(t, pausedEngine, pausedGranted, "o-paused-dispatch")
	pausedReply, err := pausedEngine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: "p-paused-uncertain", IdempotencyKey: "p-paused-uncertain",
		RunID: pausedFlight.RunID, ExpectedRevision: pausedFlight.Revision, Continue: &ContinueInput{Signal: SignalExecutionUncertain},
	})
	if err != nil {
		t.Fatal(err)
	}

	finishedEngine, finishedFixture := newInternalWorkflowEngine(t)
	finished := finishInternalWorkflow(t, finishedEngine, finishedFixture, "q-finished")
	return internalWorkflowRecords{
		awaiting: internalWorkflowRecord(t, engine, awaiting), ready: internalWorkflowRecord(t, engine, ready),
		granted: internalWorkflowRecord(t, engine, granted), inflight: internalWorkflowRecord(t, engine, inflight),
		observed: internalWorkflowRecord(t, engine, observed), switched: internalWorkflowRecord(t, engine, switchedReply),
		incident: internalWorkflowRecord(t, incidentEngine, incident), pausedPrevious: internalWorkflowRecord(t, pausedEngine, pausedFlight),
		paused: internalWorkflowRecord(t, pausedEngine, pausedReply), finished: internalWorkflowRecord(t, finishedEngine, finished),
	}
}

type internalWorkflowFixture struct {
	projectRoot string
	snapshot    config.Snapshot
	registry    registry.Registry
}

func newInternalWorkflowEngine(t *testing.T) (*Engine, internalWorkflowFixture) {
	t.Helper()
	projectRoot := t.TempDir()
	snapshot, hostIntegration := hosttest.LoadManagedSnapshot(t, projectRoot)
	home := t.TempDir()
	for _, relative := range []string{
		".codex/plugins/superpowers/skills/using-superpowers/SKILL.md",
		".agents/skills/to-spec/SKILL.md",
		".agents/skills/to-tickets/SKILL.md",
	} {
		path := filepath.Join(home, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("workflow invariant fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	bindings := make([]catalog.HostBinding, 0)
	for _, provider := range snapshot.Catalog().Providers() {
		if provider.ID != "oaw/superpowers" && provider.ID != "oaw/matt" {
			continue
		}
		for _, capability := range provider.Capabilities {
			bindings = append(bindings, capability.HostBindings...)
		}
	}
	_, effective, err := registry.Resolve(snapshot, evidence, &registry.BindingInventory{Host: "codex", Bindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	fixture := internalWorkflowFixture{projectRoot: projectRoot, snapshot: snapshot, registry: effective}
	engine, err := NewEngine(Options{
		StateRoot: filepath.Join(t.TempDir(), "state"),
		Workflow: WorkflowOptions{
			Configuration: snapshot, Registry: effective,
			Authority: admission.AuthorityCeiling{Effects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project", "project-worktree"}, ResourceLeases: true, AllowDelegation: true},
			Host:      host.RuntimeFrame{IntegrationID: hostIntegration.ID},
			Executors: []WorkflowExecutorRegistration{
				{Registration: admission.ExecutorRegistration{ID: "executor-write", Kind: admission.ExecutorIsolated}},
				{Registration: admission.ExecutorRegistration{ID: "executor-review-1", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
				{Registration: admission.ExecutorRegistration{ID: "executor-review-2", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine, fixture
}

func startInternalWorkflow(t *testing.T, engine *Engine, fixture internalWorkflowFixture, key string) RunReply {
	t.Helper()
	proposal := internalDirectProposal()
	setInternalTrait(&proposal, classification.TraitArchitectureDecision, classification.TraitTrue)
	reply, err := engine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameStart, MessageID: key + "-start", IdempotencyKey: key + "-start",
		Start: &StartInput{RequestID: key + "-request", Project: ProjectIdentity{Root: fixture.projectRoot, ConfigurationDigest: fixture.snapshot.Digest()}, Proposal: &proposal, Workflow: &WorkflowInput{DeliverableID: key + "-deliverable", InputDigest: strings.Repeat("a", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func selectInternalWorkflow(t *testing.T, engine *Engine, current RunReply, key string) RunReply {
	t.Helper()
	reply, err := engine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: key, IdempotencyKey: key,
		RunID: current.RunID, ExpectedRevision: current.Revision, Continue: &ContinueInput{Signal: SignalProfileSelected, ProfileSelection: &ProfileSelection{Profile: "MATT-SP-HYBRID"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func grantInternalWorkflow(t *testing.T, engine *Engine, current RunReply, key string) RunReply {
	return grantInternalWorkflowWithPreference(t, engine, current, key, true)
}

func grantInternalWorkflowWithPreference(t *testing.T, engine *Engine, current RunReply, key string, preferWrite bool) RunReply {
	t.Helper()
	node, found := workflowGraphNode(current.Snapshot.Workflow.Bundles[len(current.Snapshot.Workflow.Bundles)-1].Graph, current.Snapshot.Workflow.ActiveNodeID)
	if !found {
		t.Fatal("active Workflow node missing")
	}
	executorID := "executor-write"
	if node.Responsibility == "review" {
		executorID = ""
	}
	effect := node.MaximumEffects[0]
	if !preferWrite && containsWorkflowValue(node.MaximumEffects, "read-project") {
		effect = "read-project"
	} else if containsWorkflowValue(node.MaximumEffects, "write-project") {
		effect = "write-project"
	}
	resource := node.Resources[0]
	if effect == "write-project" {
		resource = resourceLeaseProjectWorktree
	}
	reply, err := engine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &ContinueInput{Signal: SignalRequestStageGrant, StageGrant: &StageGrantRequest{ExecutorID: executorID, RequestedEffects: []string{effect}, RequestedResources: []string{resource}, TerminationCondition: "complete invariant stage"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func prepareInternalWorkflow(t *testing.T, engine *Engine, current RunReply, key string) RunReply {
	t.Helper()
	grant := current.Snapshot.Grants[len(current.Snapshot.Grants)-1]
	reply, err := engine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &ContinueInput{Signal: SignalDispatchPrepared, DispatchPreparation: &DispatchPreparation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func observeInternalWorkflow(t *testing.T, engine *Engine, current RunReply, key string, outcome ObservationOutcome, signal, boundary string) RunReply {
	t.Helper()
	grant := current.Snapshot.Grants[len(current.Snapshot.Grants)-1]
	reply, err := engine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &ContinueInput{Signal: SignalCapabilityObserved, StageObservation: &StageObservation{CapabilityObservation: CapabilityObservation{
			GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID, Outcome: outcome,
			EvidenceReferences: []EvidenceReference{{Reference: "evidence://invariant", Digest: strings.Repeat("b", 64)}},
		}, Signal: signal, StableBoundary: boundary}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func finishInternalWorkflow(t *testing.T, engine *Engine, fixture internalWorkflowFixture, key string) RunReply {
	t.Helper()
	current := selectInternalWorkflow(t, engine, startInternalWorkflow(t, engine, fixture, key), key+"-select")
	for step := 0; step < 24 && current.Snapshot.Status != RunFinished; step++ {
		prefix := key + "-" + string(rune('a'+step))
		granted := grantInternalWorkflowWithPreference(t, engine, current, prefix+"-grant", false)
		prepared := prepareInternalWorkflow(t, engine, granted, prefix+"-dispatch")
		node, _ := workflowGraphNode(current.Snapshot.Workflow.Bundles[len(current.Snapshot.Workflow.Bundles)-1].Graph, current.Snapshot.Workflow.ActiveNodeID)
		signal := workflowSignalSucceeded
		if len(node.Transitions) > 0 {
			signal = node.Transitions[0].Signal
			for _, transition := range node.Transitions {
				if transition.Signal == workflowSignalSucceeded || transition.Signal == workflowSignalRemediated {
					signal = transition.Signal
					break
				}
			}
		}
		current = observeInternalWorkflow(t, engine, prepared, prefix+"-observe", ObservationSucceeded, signal, "")
	}
	if current.Snapshot.Status != RunFinished {
		t.Fatalf("Workflow did not finish: %#v", current.Snapshot.Workflow)
	}
	return current
}

func internalWorkflowRecord(t *testing.T, engine *Engine, reply RunReply) revisionRecord {
	t.Helper()
	record, err := engine.journal.loadRevision(reply.RunID, reply.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func cloneInternalWorkflowRecord(value revisionRecord) revisionRecord {
	value.Snapshot = cloneSnapshot(value.Snapshot)
	value.Reply = cloneReply(value.Reply)
	return value
}
