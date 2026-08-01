package runtime_test

import (
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestWorkflowReadyRunIssuesOneGenerationBoundStageGrant(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	started, err := engine.Exchange(workflowStartFrame(fixture, "stage-start"))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "stage-select", IdempotencyKey: "stage-select", RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalProfileSelected, ProfileSelection: &runtime.ProfileSelection{Profile: "MATT-SP-HYBRID", Bindings: []profile.ProfileBinding{}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	granted, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "stage-grant", IdempotencyKey: "stage-grant", RunID: started.RunID, ExpectedRevision: selected.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestStageGrant, StageGrant: &runtime.StageGrantRequest{
			ExecutorID: "executor-write", RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"},
			TerminationCondition: "requirements are recorded",
		}},
	})
	if err != nil {
		t.Fatalf("Exchange(REQUEST_STAGE_GRANT) error = %v", err)
	}
	if granted.Kind != runtime.ReplyGrantIssued || granted.Snapshot.Status != runtime.RunGranted || granted.Revision != 3 || len(granted.Snapshot.Grants) != 1 || len(granted.Snapshot.GrantIDs) != 1 {
		t.Fatalf("stage Grant reply = %#v", granted)
	}
	grant := granted.Snapshot.Grants[0]
	bundle := granted.Snapshot.Workflow.Bundles[0]
	if grant.ID != granted.Snapshot.GrantIDs[0] || grant.ID != granted.Snapshot.Workflow.ActiveGrantID || grant.Generation != 1 || grant.BundleID != bundle.ID || grant.GraphDigest != bundle.GraphDigest || grant.NodeID != "requirements" || grant.ProviderID != "oaw/matt" || grant.CapabilityID != "specification" || grant.Executor.ID != "executor-write" {
		t.Fatalf("stage Grant pins = %#v", grant)
	}

	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "stage-grant-second", IdempotencyKey: "stage-grant-second", RunID: started.RunID, ExpectedRevision: granted.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestStageGrant, StageGrant: &runtime.StageGrantRequest{
			ExecutorID: "executor-write", RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "duplicate",
		}},
	})
	assertErrorCode(t, err, "RUN_TRANSITION_INVALID")
	assertRevisionCount(t, stateRoot, started.RunID, 3)
}
