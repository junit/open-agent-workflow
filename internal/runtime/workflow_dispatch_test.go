package runtime_test

import (
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestWorkflowDispatchPreparedMovesOnlyTheCommittedGrantToInFlight(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, engine, fixture, "dispatch-prepared")
	granted := requestWorkflowStage(t, engine, ready, "dispatch-grant", []string{"read-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[0]

	_, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "dispatch-wrong", IdempotencyKey: "dispatch-wrong", RunID: granted.RunID, ExpectedRevision: granted.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalDispatchPrepared, DispatchPreparation: &runtime.DispatchPreparation{
			GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: "wrong-executor",
		}},
	})
	assertErrorCode(t, err, "DISPATCH_PREPARATION_INVALID")
	assertRevisionCount(t, stateRoot, granted.RunID, int(granted.Revision))

	frame := runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "dispatch-correct", IdempotencyKey: "dispatch-correct", RunID: granted.RunID, ExpectedRevision: granted.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalDispatchPrepared, DispatchPreparation: &runtime.DispatchPreparation{
			GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
		}},
	}
	prepared, err := engine.Exchange(frame)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Kind != runtime.ReplyDispatchAuthorized || prepared.Snapshot.Status != runtime.RunInFlight {
		t.Fatalf("Workflow dispatch reply = %#v", prepared)
	}
	replayed, err := engine.Exchange(frame)
	if err != nil || !reflect.DeepEqual(replayed, prepared) {
		t.Fatalf("Workflow dispatch replay = %#v, %v", replayed, err)
	}
	assertRevisionCount(t, stateRoot, granted.RunID, int(prepared.Revision))
}

func TestWorkflowContinueRevalidatesActiveHostAfterRestart(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	healthy := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, healthy, fixture, "workflow-host-restart")
	granted := requestWorkflowStage(t, healthy, ready, "workflow-host-restart-grant", []string{"read-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
	unavailable := newWorkflowEngineWithHostFrame(t, stateRoot, fixture, host.RuntimeFrame{
		IntegrationID: fixture.hostIntegration.ID, UnavailableFeatures: []host.Feature{host.FeaturePause},
	})
	_, err := unavailable.Exchange(inspectFrame(granted.RunID, "workflow-host-restart-inspect"))
	assertErrorCode(t, err, "HOST_RUNTIME_REQUIREMENTS_UNMET")

	_, err = unavailable.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-host-restart-dispatch", IdempotencyKey: "workflow-host-restart-dispatch",
		RunID: granted.RunID, ExpectedRevision: granted.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalDispatchPrepared, DispatchPreparation: &runtime.DispatchPreparation{
			GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
		}},
	})
	assertErrorCode(t, err, "HOST_RUNTIME_REQUIREMENTS_UNMET")
	assertRevisionCount(t, stateRoot, granted.RunID, int(granted.Revision))

	prepared := prepareWorkflowStage(t, healthy, granted, "workflow-host-restart-prepared", grant)
	_, err = unavailable.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-host-restart-observation", IdempotencyKey: "workflow-host-restart-observation",
		RunID: prepared.RunID, ExpectedRevision: prepared.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilityObserved, StageObservation: &runtime.StageObservation{
			CapabilityObservation: runtime.CapabilityObservation{
				GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
				Outcome: runtime.ObservationSucceeded, EvidenceReferences: []runtime.EvidenceReference{{Reference: "evidence://host-restart", Digest: strings.Repeat("a", 64)}},
			},
			Signal: "succeeded",
		}},
	})
	assertErrorCode(t, err, "HOST_RUNTIME_REQUIREMENTS_UNMET")
	assertRevisionCount(t, stateRoot, prepared.RunID, int(prepared.Revision))
}

func TestWorkflowObservationAdvancesThePinnedGraphAndReleasesSuccessfulLease(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, engine, fixture, "workflow-observation")
	granted := requestWorkflowStage(t, engine, ready, "workflow-observation-grant", []string{"write-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[0]
	prepared := prepareWorkflowStage(t, engine, granted, "workflow-observation-prepared", grant)

	observed, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-observation-result", IdempotencyKey: "workflow-observation-result", RunID: prepared.RunID, ExpectedRevision: prepared.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilityObserved, StageObservation: &runtime.StageObservation{
			CapabilityObservation: runtime.CapabilityObservation{
				GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
				Outcome: runtime.ObservationSucceeded, EvidenceReferences: []runtime.EvidenceReference{{Reference: "evidence://workflow-observation", Digest: strings.Repeat("1", 64)}},
			},
			Signal: "succeeded",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if observed.Kind != runtime.ReplyModeDecided || observed.Snapshot.Status != runtime.RunReady || observed.Snapshot.Workflow.ActiveNodeID != "domain-modeling" || observed.Snapshot.Workflow.ActiveGrantID != "" || len(observed.Snapshot.ResourceLeaseIDs) != 0 || len(observed.Snapshot.Workflow.Observations) != 1 {
		t.Fatalf("workflow observation reply = %#v", observed)
	}
	assertRevisionCount(t, stateRoot, prepared.RunID, int(observed.Revision))
}

func TestWorkflowObservationRejectsRawOutputAndUnknownSignalsWithoutRevision(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, engine, fixture, "workflow-observation-invalid")
	granted := requestWorkflowStage(t, engine, ready, "workflow-observation-invalid-grant", []string{"read-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[0]
	prepared := prepareWorkflowStage(t, engine, granted, "workflow-observation-invalid-prepared", grant)
	base := runtime.StageObservation{
		CapabilityObservation: runtime.CapabilityObservation{
			GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
			Outcome: runtime.ObservationSucceeded, EvidenceReferences: []runtime.EvidenceReference{{Reference: "evidence://invalid", Digest: strings.Repeat("2", 64)}},
		},
		Signal: "succeeded",
	}
	base.RawOutput = "provider output must not enter Runtime state"
	_, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-raw-output", IdempotencyKey: "workflow-raw-output", RunID: prepared.RunID, ExpectedRevision: prepared.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilityObserved, StageObservation: &base},
	})
	assertErrorCode(t, err, "OBSERVATION_INVALID")
	base.RawOutput = ""
	base.Signal = "provider-invented-target"
	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-unknown-signal", IdempotencyKey: "workflow-unknown-signal", RunID: prepared.RunID, ExpectedRevision: prepared.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilityObserved, StageObservation: &base},
	})
	assertErrorCode(t, err, "OBSERVATION_INVALID")
	assertRevisionCount(t, stateRoot, prepared.RunID, int(prepared.Revision))
}

func TestWorkflowIncidentRetainsAndReusesLeaseUntilSuccessfulRecovery(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, engine, fixture, "workflow-incident")
	granted := requestWorkflowStage(t, engine, ready, "workflow-incident-grant", []string{"write-project"}, []string{"project-worktree"})
	leaseID := granted.Snapshot.ResourceLeaseIDs[0]
	grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
	prepared := prepareWorkflowStage(t, engine, granted, "workflow-incident-prepared", grant)

	routed := observeWorkflowStage(t, engine, prepared, grant, "workflow-incident-result", runtime.ObservationFailed, "functional-failure", "")
	if routed.Snapshot.Status != runtime.RunReady || routed.Snapshot.Workflow.ActiveNodeID != "functional-debugging" || len(routed.Snapshot.ResourceLeaseIDs) != 1 || routed.Snapshot.ResourceLeaseIDs[0] != leaseID {
		t.Fatalf("Workflow incident state = %#v", routed.Snapshot)
	}

	recoveryGranted := requestWorkflowStage(t, engine, routed, "workflow-recovery-grant", []string{"write-project"}, []string{"project-worktree"})
	if len(recoveryGranted.Snapshot.ResourceLeaseIDs) != 1 || recoveryGranted.Snapshot.ResourceLeaseIDs[0] != leaseID || len(recoveryGranted.Snapshot.Workflow.ResourceLeases) != 1 {
		t.Fatalf("Workflow recovery minted a second lease = %#v", recoveryGranted.Snapshot.Workflow.ResourceLeases)
	}
	recoveryGrant := recoveryGranted.Snapshot.Grants[len(recoveryGranted.Snapshot.Grants)-1]
	recoveryPrepared := prepareWorkflowStage(t, engine, recoveryGranted, "workflow-recovery-prepared", recoveryGrant)
	recovered := observeWorkflowStage(t, engine, recoveryPrepared, recoveryGrant, "workflow-recovery-result", runtime.ObservationSucceeded, "succeeded", "")
	if recovered.Snapshot.Status != runtime.RunReady || recovered.Snapshot.Workflow.ActiveNodeID != "remediation" || len(recovered.Snapshot.ResourceLeaseIDs) != 0 {
		t.Fatalf("Workflow recovery state = %#v", recovered.Snapshot)
	}
}

func TestWorkflowGraphReachesTerminalGateWithoutInventingTargets(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	engine := newWorkflowEngine(t, filepath.Join(t.TempDir(), "state"), fixture, true)
	current := startAndSelectWorkflow(t, engine, fixture, "workflow-terminal")

	for step := 0; step < 32 && current.Snapshot.Status != runtime.RunFinished; step++ {
		node := activeWorkflowGraphNode(t, current)
		signal := "succeeded"
		for _, transition := range node.Transitions {
			if transition.Signal == "succeeded" {
				signal = transition.Signal
				break
			}
			if signal == "succeeded" {
				signal = transition.Signal
			}
		}
		executorID := "executor-write"
		if node.Responsibility == "review" {
			executorID = ""
		}
		granted := requestWorkflowNodeStage(t, engine, current, "workflow-terminal", step, executorID, preferredWorkflowValue(node.MaximumEffects, "read-project"), preferredWorkflowValue(node.Resources, "project"))
		grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
		prepared := prepareWorkflowStage(t, engine, granted, "workflow-terminal-prepared-"+strconv.Itoa(step), grant)
		current = observeWorkflowStage(t, engine, prepared, grant, "workflow-terminal-observed-"+strconv.Itoa(step), runtime.ObservationSucceeded, signal, "")
	}
	if current.Snapshot.Status != runtime.RunFinished || current.Kind != runtime.ReplyFinished || current.Snapshot.Workflow.ActiveNodeID != "completion" {
		t.Fatalf("Workflow terminal state = %#v", current)
	}
}

func TestWorkflowStableBoundarySwitchCreatesANewBundleGeneration(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, engine, fixture, "workflow-switch")
	granted := requestWorkflowStage(t, engine, ready, "workflow-switch-grant", []string{"read-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[0]
	prepared := prepareWorkflowStage(t, engine, granted, "workflow-switch-prepared", grant)
	observed, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-switch-observation", IdempotencyKey: "workflow-switch-observation", RunID: prepared.RunID, ExpectedRevision: prepared.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilityObserved, StageObservation: &runtime.StageObservation{
			CapabilityObservation: runtime.CapabilityObservation{
				GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
				Outcome: runtime.ObservationSucceeded, EvidenceReferences: []runtime.EvidenceReference{{Reference: "evidence://switch", Digest: strings.Repeat("3", 64)}},
			},
			Signal: "succeeded", StableBoundary: "specification-approved",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	switched, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-switch-profile", IdempotencyKey: "workflow-switch-profile", RunID: observed.RunID, ExpectedRevision: observed.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalSwitchProfile, StableBoundarySwitch: &runtime.StableBoundarySwitch{
			Boundary: "specification-approved", Selection: runtime.ProfileSelection{Profile: "SP-FULL"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if switched.Snapshot.Workflow.ActiveGeneration != 2 || switched.Snapshot.Workflow.ActiveNodeID != "requirements" || len(switched.Snapshot.Workflow.Bundles) != 2 || switched.Snapshot.Workflow.Bundles[0].ID == switched.Snapshot.Workflow.Bundles[1].ID || switched.Snapshot.Workflow.Bundles[0].Digest == switched.Snapshot.Workflow.Bundles[1].Digest || len(switched.Snapshot.Workflow.RevokedGrantIDs) != 1 || switched.Snapshot.Workflow.RevokedGrantIDs[0] != grant.ID {
		t.Fatalf("switched Workflow state = %#v", switched.Snapshot.Workflow)
	}
	newBundle := switched.Snapshot.Workflow.Bundles[1]
	if newBundle.HostIntegrationID != fixture.hostIntegration.ID || newBundle.HostIntegrationDigest != fixture.hostIntegration.Digest || newBundle.HostManifestDigest != fixture.hostIntegration.ManifestDigest || newBundle.HostAuditDigest != fixture.hostIntegration.Audit.Digest || newBundle.HostConformanceDigest != fixture.hostIntegration.Conformance.Digest {
		t.Fatalf("switched Bundle Host pins = %#v", newBundle)
	}
}

func TestWorkflowStableBoundarySwitchExplicitlyAdoptsCurrentConfiguration(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, engine, fixture, "workflow-config-switch")
	granted := requestWorkflowStage(t, engine, ready, "workflow-config-switch-grant", []string{"read-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[0]
	prepared := prepareWorkflowStage(t, engine, granted, "workflow-config-switch-prepared", grant)
	observed := observeWorkflowStage(t, engine, prepared, grant, "workflow-config-switch-observed", runtime.ObservationSucceeded, "succeeded", "specification-approved")

	userRoot := t.TempDir()
	hosttest.WriteManagedConfiguration(t, userRoot, `
[[bounded_capability_defaults]]
id = "review-default"
provider_id = "oaw/superpowers"
capability_id = "review"
`)
	updatedSnapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: fixture.projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if updatedSnapshot.Digest() == fixture.snapshot.Digest() {
		t.Fatal("updated Configuration Snapshot retained the old digest")
	}
	evidence, err := discovery.Discover(updatedSnapshot.Catalog(), discovery.Options{UserHome: fixture.home})
	if err != nil {
		t.Fatal(err)
	}
	_, updatedRegistry, err := registry.Resolve(updatedSnapshot, evidence, &registry.BindingInventory{Host: "codex", Bindings: fixture.bindings})
	if err != nil {
		t.Fatal(err)
	}
	updatedFixture := fixture
	updatedFixture.snapshot = updatedSnapshot
	updatedFixture.registry = updatedRegistry
	updatedEngine := newWorkflowEngine(t, stateRoot, updatedFixture, true)
	switched := switchWorkflowProfile(t, updatedEngine, observed, "workflow-config-switch-profile", "specification-approved", "SP-FULL")
	workflow := switched.Snapshot.Workflow
	if len(workflow.Bundles) != 2 || workflow.Bundles[0].Configuration.Digest != fixture.snapshot.Digest() || workflow.Bundles[1].Configuration.Digest != updatedSnapshot.Digest() || workflow.ConfigurationDigest != updatedSnapshot.Digest() || workflow.RegistryDigest != updatedRegistry.Digest() {
		t.Fatalf("explicit Configuration adoption = %#v", workflow)
	}
	if switched.Snapshot.ConfigurationDigest != fixture.snapshot.Digest() || switched.Snapshot.Project.ConfigurationDigest != fixture.snapshot.Digest() {
		t.Fatalf("Bundle switch changed immutable Run identity = %#v", switched.Snapshot.Project)
	}

	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-old-config-grant", IdempotencyKey: "workflow-old-config-grant", RunID: switched.RunID, ExpectedRevision: switched.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestStageGrant, StageGrant: &runtime.StageGrantRequest{
			ExecutorID: "executor-write", RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "old configuration must not execute",
		}},
	})
	assertErrorCode(t, err, "WORKFLOW_CONFIGURATION_REQUIRED")
	_ = requestWorkflowStage(t, updatedEngine, switched, "workflow-new-config-grant", []string{"read-project"}, []string{"project-worktree"})
}

func TestWorkflowStableBoundarySwitchRejectsInvalidTimingAndSelectionWithoutRevision(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	ready := startAndSelectWorkflow(t, engine, fixture, "workflow-switch-rejected")

	_, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-switch-unknown", IdempotencyKey: "workflow-switch-unknown", RunID: ready.RunID, ExpectedRevision: ready.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalSwitchProfile, StableBoundarySwitch: &runtime.StableBoundarySwitch{
			Boundary: "unknown-boundary", Selection: runtime.ProfileSelection{Profile: "SP-FULL"},
		}},
	})
	assertErrorCode(t, err, "STABLE_BOUNDARY_INVALID")
	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-switch-missing", IdempotencyKey: "workflow-switch-missing", RunID: ready.RunID, ExpectedRevision: ready.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalSwitchProfile, StableBoundarySwitch: &runtime.StableBoundarySwitch{
			Boundary: "specification-approved", Selection: runtime.ProfileSelection{},
		}},
	})
	assertErrorCode(t, err, "PROFILE_SELECTION_INVALID")
	assertRevisionCount(t, stateRoot, ready.RunID, int(ready.Revision))

	granted := requestWorkflowStage(t, engine, ready, "workflow-switch-active-grant", []string{"read-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
	prepared := prepareWorkflowStage(t, engine, granted, "workflow-switch-active-prepared", grant)
	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-switch-active", IdempotencyKey: "workflow-switch-active", RunID: prepared.RunID, ExpectedRevision: prepared.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalSwitchProfile, StableBoundarySwitch: &runtime.StableBoundarySwitch{
			Boundary: "specification-approved", Selection: runtime.ProfileSelection{Profile: "SP-FULL"},
		}},
	})
	assertErrorCode(t, err, "RUN_TRANSITION_INVALID")
	assertRevisionCount(t, stateRoot, ready.RunID, int(prepared.Revision))
}

func prepareWorkflowStage(t *testing.T, engine *runtime.Engine, granted runtime.RunReply, key string, grant admission.CapabilityGrant) runtime.RunReply {
	t.Helper()
	prepared, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: key, IdempotencyKey: key, RunID: granted.RunID, ExpectedRevision: granted.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalDispatchPrepared, DispatchPreparation: &runtime.DispatchPreparation{
			GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func observeWorkflowStage(t *testing.T, engine *runtime.Engine, current runtime.RunReply, grant admission.CapabilityGrant, key string, outcome runtime.ObservationOutcome, signal, boundary string) runtime.RunReply {
	t.Helper()
	observed, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilityObserved, StageObservation: &runtime.StageObservation{
			CapabilityObservation: runtime.CapabilityObservation{
				GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
				Outcome: outcome, EvidenceReferences: []runtime.EvidenceReference{{Reference: "evidence://" + key, Digest: strings.Repeat("4", 64)}},
			},
			Signal: signal, StableBoundary: boundary,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return observed
}

func activeWorkflowGraphNode(t *testing.T, current runtime.RunReply) profile.GraphNode {
	t.Helper()
	bundle := current.Snapshot.Workflow.Bundles[len(current.Snapshot.Workflow.Bundles)-1]
	for _, node := range bundle.Graph.Nodes {
		if node.ID == current.Snapshot.Workflow.ActiveNodeID {
			return node
		}
	}
	t.Fatalf("active Workflow node %q is missing", current.Snapshot.Workflow.ActiveNodeID)
	return profile.GraphNode{}
}

func requestWorkflowNodeStage(t *testing.T, engine *runtime.Engine, ready runtime.RunReply, prefix string, step int, executorID, effect, resource string) runtime.RunReply {
	t.Helper()
	key := prefix + "-grant-" + strconv.Itoa(step)
	granted, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: key, IdempotencyKey: key, RunID: ready.RunID, ExpectedRevision: ready.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestStageGrant, StageGrant: &runtime.StageGrantRequest{
			ExecutorID: executorID, RequestedEffects: []string{effect}, RequestedResources: []string{resource}, TerminationCondition: "stage-complete",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return granted
}

func preferredWorkflowValue(values []string, preferred string) string {
	for _, value := range values {
		if value == preferred {
			return value
		}
	}
	return values[0]
}
