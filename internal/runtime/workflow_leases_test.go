package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestWorkflowWriteStageGrantHoldsOneCrossRunPhysicalWorktreeLease(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)

	first := startAndSelectWorkflow(t, engine, fixture, "lease-first")
	second := startAndSelectWorkflow(t, engine, fixture, "lease-second")
	firstGrant := requestWorkflowStage(t, engine, first, "lease-first-grant", []string{"write-project"}, []string{"project-worktree"})
	if len(firstGrant.Snapshot.ResourceLeaseIDs) != 1 || len(firstGrant.Snapshot.Workflow.ResourceLeases) != 1 {
		t.Fatalf("first lease = %#v", firstGrant.Snapshot)
	}
	if _, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "lease-second-grant", IdempotencyKey: "lease-second-grant", RunID: second.RunID, ExpectedRevision: second.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestStageGrant, StageGrant: &runtime.StageGrantRequest{
			ExecutorID: "executor-write", RequestedEffects: []string{"write-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "second write",
		}},
	}); runtime.ErrorCode(err) != "RESOURCE_LEASE_CONFLICT" {
		t.Fatalf("second write lease error = %v", err)
	}
	inspected, err := engine.Exchange(runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameInspect, MessageID: "lease-inspect", IdempotencyKey: "lease-inspect", RunID: first.RunID})
	if err != nil || inspected.Revision != firstGrant.Revision || inspected.Snapshot.ResourceLeaseIDs[0] != firstGrant.Snapshot.ResourceLeaseIDs[0] {
		t.Fatalf("first lease changed after conflict: %#v, %v", inspected, err)
	}
}

func TestWorkflowReadOnlyStageGrantDoesNotNeedPhysicalWorktreeLease(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)

	first := startAndSelectWorkflow(t, engine, fixture, "lease-read-first")
	second := startAndSelectWorkflow(t, engine, fixture, "lease-read-second")
	_ = requestWorkflowStage(t, engine, first, "lease-read-first-grant", []string{"write-project"}, []string{"project-worktree"})
	readOnly := requestWorkflowStage(t, engine, second, "lease-read-second-grant", []string{"read-project"}, []string{"project-worktree"})
	if len(readOnly.Snapshot.ResourceLeaseIDs) != 0 || len(readOnly.Snapshot.Workflow.ResourceLeases) != 0 {
		t.Fatalf("read-only stage acquired a Resource Lease: %#v", readOnly.Snapshot)
	}
}

func TestWorkflowWriteStageGrantConflictsThroughPhysicalWorktreeAlias(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	alias := filepath.Join(t.TempDir(), "project-alias")
	if err := os.Symlink(fixture.projectRoot, alias); err != nil {
		t.Fatal(err)
	}

	first := startAndSelectWorkflow(t, engine, fixture, "lease-alias-first")
	secondStart := workflowStartFrame(fixture, "lease-alias-second-start")
	secondStart.Start.Project.Root = alias
	started, err := engine.Exchange(secondStart)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "lease-alias-second-select", IdempotencyKey: "lease-alias-second-select", RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalProfileSelected, ProfileSelection: &runtime.ProfileSelection{Profile: "MATT-SP-HYBRID"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = requestWorkflowStage(t, engine, first, "lease-alias-first-grant", []string{"write-project"}, []string{"project-worktree"})
	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "lease-alias-second-grant", IdempotencyKey: "lease-alias-second-grant", RunID: selected.RunID, ExpectedRevision: selected.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestStageGrant, StageGrant: &runtime.StageGrantRequest{
			ExecutorID: "executor-write", RequestedEffects: []string{"write-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "alias write",
		}},
	})
	assertErrorCode(t, err, "RESOURCE_LEASE_CONFLICT")
}

func startAndSelectWorkflow(t *testing.T, engine *runtime.Engine, fixture workflowRuntimeFixture, key string) runtime.RunReply {
	t.Helper()
	started, err := engine.Exchange(workflowStartFrame(fixture, key+"-start"))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: key + "-select", IdempotencyKey: key + "-select", RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalProfileSelected, ProfileSelection: &runtime.ProfileSelection{Profile: "MATT-SP-HYBRID"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

func requestWorkflowStage(t *testing.T, engine *runtime.Engine, ready runtime.RunReply, key string, effects, resources []string) runtime.RunReply {
	t.Helper()
	granted, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: key, IdempotencyKey: key, RunID: ready.RunID, ExpectedRevision: ready.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestStageGrant, StageGrant: &runtime.StageGrantRequest{
			ExecutorID: "executor-write", RequestedEffects: effects, RequestedResources: resources, TerminationCondition: "stage complete",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return granted
}
