package runtime_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestWorkflowProjectionEmitsRedactedCommittedRevisions(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	sink := &recordingProjectionSink{stateRoot: stateRoot}
	engine := newWorkflowEngineWithProjection(t, stateRoot, fixture, oawruntime.ProjectionOptions{Sink: sink})

	started, err := engine.Exchange(workflowStartFrame(fixture, "projection-start"))
	if err != nil {
		t.Fatal(err)
	}
	ready := selectWorkflowProfile(t, engine, started, "projection-select", "MATT-SP-HYBRID")
	granted := requestWorkflowStage(t, engine, ready, "projection-grant", []string{"read-project"}, []string{"project-worktree"})
	grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
	prepared := prepareWorkflowStage(t, engine, granted, "projection-prepared", grant)
	observed := observeWorkflowStage(t, engine, prepared, grant, "projection-observed", oawruntime.ObservationSucceeded, "succeeded", "specification-approved")
	switched := switchWorkflowProfile(t, engine, observed, "projection-switched", "specification-approved", "SP-FULL")

	records := sink.Records()
	if len(records) != int(switched.Revision) {
		t.Fatalf("projection count = %d, want %d", len(records), switched.Revision)
	}
	for index, record := range records {
		if record.RunID != started.RunID || record.Revision != uint64(index+1) || len(record.RevisionDigest) != 64 || len(record.StateDigest) != 64 || len(record.Digest) != 64 {
			t.Fatalf("projection %d = %#v", index, record)
		}
		raw, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		assertProjectionRedacted(t, raw)
	}
	last := records[len(records)-1]
	if records[0].Status != oawruntime.RunAwaitingSelection || records[0].BundleID != "" || records[1].Status != oawruntime.RunReady || records[1].BundleID == "" || last.ActiveNodeID != "requirements" || last.Generation != 2 || last.BundleID != switched.Snapshot.Workflow.Bundles[1].ID {
		t.Fatalf("projection state sequence = %#v", records)
	}
	bundle := switched.Snapshot.Workflow.Bundles[1]
	if last.HostIntegrationID != bundle.HostIntegrationID || last.HostIntegrationDigest != bundle.HostIntegrationDigest || last.HostManifestDigest != bundle.HostManifestDigest || last.HostAuditDigest != bundle.HostAuditDigest || last.HostConformanceDigest != bundle.HostConformanceDigest {
		t.Fatalf("projection Host pins = %#v, want %#v", last, bundle)
	}
}

func TestWorkflowProjectionFailureRecordsLagWithoutChangingCommittedReply(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngineWithProjection(t, stateRoot, fixture, oawruntime.ProjectionOptions{Sink: failingProjectionSink{}})

	started, err := engine.Exchange(workflowStartFrame(fixture, "projection-failure"))
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := engine.Exchange(inspectFrame(started.RunID, "projection-failure-inspect"))
	if err != nil {
		t.Fatal(err)
	}
	if inspected.RevisionDigest != started.RevisionDigest || !reflect.DeepEqual(inspected.Snapshot, started.Snapshot) || len(started.Snapshot.Workflow.ProjectionLag) != 0 {
		t.Fatalf("projection failure changed committed reply = %#v", started)
	}
	lagPath := filepath.Join(stateRoot, "projection-lag", started.RunID, "00000000000000000001.json")
	raw, err := os.ReadFile(lagPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "PROJECTION_WRITE_FAILED") || !strings.Contains(string(raw), started.RevisionDigest) || strings.Contains(string(raw), "credential=projection-secret") {
		t.Fatalf("projection lag marker = %s", raw)
	}
	assertFileMode(t, lagPath, 0o600)
}

func TestWorkflowProjectionPanicRecordsLagWithoutChangingCommittedReply(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngineWithProjection(t, stateRoot, fixture, oawruntime.ProjectionOptions{Sink: panickingProjectionSink{}})

	started, err := engine.Exchange(workflowStartFrame(fixture, "projection-panic"))
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := engine.Exchange(inspectFrame(started.RunID, "projection-panic-inspect"))
	if err != nil {
		t.Fatal(err)
	}
	if inspected.RevisionDigest != started.RevisionDigest || !reflect.DeepEqual(inspected.Snapshot, started.Snapshot) {
		t.Fatalf("projection panic changed committed reply = %#v", started)
	}
	lagPath := filepath.Join(stateRoot, "projection-lag", started.RunID, "00000000000000000001.json")
	if _, err := os.Stat(lagPath); err != nil {
		t.Fatalf("projection panic lag marker: %v", err)
	}
}

func TestFilesystemWorkflowProjectionIsOneWayAndOwnerOnly(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectionRoot := canonicalTestPath(t, filepath.Join(t.TempDir(), "projections"))
	engine := newWorkflowEngineWithProjection(t, stateRoot, fixture, oawruntime.ProjectionOptions{Root: projectionRoot})
	started, err := engine.Exchange(workflowStartFrame(fixture, "filesystem-projection"))
	if err != nil {
		t.Fatal(err)
	}
	ready := selectWorkflowProfile(t, engine, started, "filesystem-projection-select", "MATT-SP-HYBRID")

	runProjectionRoot := filepath.Join(projectionRoot, ready.RunID)
	jsonPath := filepath.Join(runProjectionRoot, "workflow.json")
	markdownPath := filepath.Join(runProjectionRoot, "workflow.md")
	for _, path := range []string{jsonPath, markdownPath} {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		assertProjectionRedacted(t, raw)
		assertFileMode(t, path, 0o600)
	}
	assertFileMode(t, projectionRoot, 0o700)
	assertFileMode(t, runProjectionRoot, 0o700)

	malicious := []byte(`{"active_node_id":"attacker-target","host_integration_id":"attacker/runtime","host_integration_digest":"0000000000000000000000000000000000000000000000000000000000000000","profile":"ECC-FULL","grant":"credential"}`)
	if err := os.WriteFile(jsonPath, malicious, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(markdownPath); err != nil {
		t.Fatal(err)
	}
	restarted := newWorkflowEngineWithProjection(t, stateRoot, fixture, oawruntime.ProjectionOptions{Root: projectionRoot})
	inspected, err := restarted.Exchange(inspectFrame(ready.RunID, "filesystem-projection-inspect"))
	if err != nil || !reflect.DeepEqual(inspected.Snapshot, ready.Snapshot) {
		t.Fatalf("projection influenced INSPECT = %#v, %v", inspected, err)
	}
	granted := requestWorkflowStage(t, restarted, ready, "filesystem-projection-grant", []string{"read-project"}, []string{"project-worktree"})
	if granted.Snapshot.Workflow.ActiveNodeID != "requirements" || len(granted.Snapshot.Grants) != 1 {
		t.Fatalf("projection influenced Stage Grant = %#v", granted.Snapshot)
	}
}

func TestFilesystemWorkflowProjectionRejectsSymlinkDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional Windows privileges")
	}
	fixture := newWorkflowRuntimeFixture(t)
	outside := canonicalTestPath(t, filepath.Join(t.TempDir(), "outside"))
	root := filepath.Join(t.TempDir(), "projection-link")
	if err := os.Symlink(outside, root); err != nil {
		t.Fatal(err)
	}
	_, err := oawruntime.NewEngine(oawruntime.Options{
		StateRoot: filepath.Join(t.TempDir(), "state"),
		Workflow: oawruntime.WorkflowOptions{
			Configuration: fixture.snapshot, Registry: fixture.registry,
			Projection: oawruntime.ProjectionOptions{Root: root},
		},
	})
	if oawruntime.ErrorCode(err) != "PROJECTION_DESTINATION_INVALID" {
		t.Fatalf("symlink projection destination error = %v", err)
	}
}

func TestWorkflowRestartPreservesEveryOrchestrationState(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	started, err := engine.Exchange(workflowStartFrame(fixture, "restart-workflow"))
	if err != nil {
		t.Fatal(err)
	}
	assertWorkflowRestart(t, stateRoot, fixture, started)
	ready := selectWorkflowProfile(t, engine, started, "restart-workflow-select", "MATT-SP-HYBRID")
	assertWorkflowRestart(t, stateRoot, fixture, ready)
	granted := requestWorkflowStage(t, engine, ready, "restart-workflow-grant", []string{"write-project"}, []string{"project-worktree"})
	assertWorkflowRestart(t, stateRoot, fixture, granted)
	grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
	prepared := prepareWorkflowStage(t, engine, granted, "restart-workflow-prepared", grant)
	assertWorkflowRestart(t, stateRoot, fixture, prepared)
	observed := observeWorkflowStage(t, engine, prepared, grant, "restart-workflow-observed", oawruntime.ObservationSucceeded, "succeeded", "specification-approved")
	assertWorkflowRestart(t, stateRoot, fixture, observed)
	switched := switchWorkflowProfile(t, engine, observed, "restart-workflow-switch", "specification-approved", "SP-FULL")
	assertWorkflowRestart(t, stateRoot, fixture, switched)

	pausedStart, err := engine.Exchange(workflowStartFrame(fixture, "restart-paused"))
	if err != nil {
		t.Fatal(err)
	}
	pausedReady := selectWorkflowProfile(t, engine, pausedStart, "restart-paused-select", "MATT-SP-HYBRID")
	pausedGrant := requestWorkflowStage(t, engine, pausedReady, "restart-paused-grant", []string{"write-project"}, []string{"project-worktree"})
	pausedCapability := pausedGrant.Snapshot.Grants[len(pausedGrant.Snapshot.Grants)-1]
	pausedPrepared := prepareWorkflowStage(t, engine, pausedGrant, "restart-paused-prepared", pausedCapability)
	paused, err := engine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: "restart-paused-uncertain", IdempotencyKey: "restart-paused-uncertain", RunID: pausedPrepared.RunID, ExpectedRevision: pausedPrepared.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalExecutionUncertain},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertWorkflowRestart(t, stateRoot, fixture, paused)
}

func newWorkflowEngineWithProjection(t *testing.T, stateRoot string, fixture workflowRuntimeFixture, projection oawruntime.ProjectionOptions) *oawruntime.Engine {
	t.Helper()
	engine, err := oawruntime.NewEngine(oawruntime.Options{
		StateRoot: stateRoot,
		Workflow: oawruntime.WorkflowOptions{
			Configuration: fixture.snapshot, Registry: fixture.registry,
			Authority: admission.AuthorityCeiling{
				Effects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project", "project-worktree"}, ResourceLeases: true, AllowDelegation: true,
			},
			Host: host.RuntimeFrame{IntegrationID: fixture.hostIntegration.ID},
			Executors: []oawruntime.WorkflowExecutorRegistration{
				{Registration: admission.ExecutorRegistration{ID: "executor-write", Kind: admission.ExecutorIsolated}},
				{Registration: admission.ExecutorRegistration{ID: "executor-review", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
			},
			Projection: projection,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

type recordingProjectionSink struct {
	mu        sync.Mutex
	stateRoot string
	records   []oawruntime.WorkflowProjection
}

func (sink *recordingProjectionSink) WriteProjection(value oawruntime.WorkflowProjection) error {
	head, err := os.ReadFile(filepath.Join(sink.stateRoot, "runs", value.RunID, "HEAD"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(head), value.RevisionDigest) {
		return errors.New("projection ran before HEAD commit")
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.records = append(sink.records, value)
	return nil
}

func (sink *recordingProjectionSink) Records() []oawruntime.WorkflowProjection {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]oawruntime.WorkflowProjection{}, sink.records...)
}

type failingProjectionSink struct{}

func (failingProjectionSink) WriteProjection(oawruntime.WorkflowProjection) error {
	return errors.New("credential=projection-secret")
}

type panickingProjectionSink struct{}

func (panickingProjectionSink) WriteProjection(oawruntime.WorkflowProjection) error {
	panic("credential=projection-panic-secret")
}

func assertProjectionRedacted(t *testing.T, raw []byte) {
	t.Helper()
	for _, forbidden := range []string{"grant_id", "invocation_id", "executor_id", "evidence", "raw_output", "credential", "provider output"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("projection contains %q: %s", forbidden, raw)
		}
	}
}

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(physical)
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want {
		t.Fatalf("mode(%s) = %o, want %o", path, info.Mode().Perm(), want)
	}
}

func assertWorkflowRestart(t *testing.T, stateRoot string, fixture workflowRuntimeFixture, want oawruntime.RunReply) {
	t.Helper()
	restarted := newWorkflowEngine(t, stateRoot, fixture, true)
	inspected, err := restarted.Exchange(inspectFrame(want.RunID, "restart-inspect"))
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Revision != want.Revision || inspected.RevisionDigest != want.RevisionDigest || !reflect.DeepEqual(inspected.Snapshot, want.Snapshot) {
		t.Fatalf("restarted Workflow = %#v, want %#v", inspected, want)
	}
}

func selectWorkflowProfile(t *testing.T, engine *oawruntime.Engine, current oawruntime.RunReply, key, selected string) oawruntime.RunReply {
	t.Helper()
	reply, err := engine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalProfileSelected, ProfileSelection: &oawruntime.ProfileSelection{Profile: selected}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func switchWorkflowProfile(t *testing.T, engine *oawruntime.Engine, current oawruntime.RunReply, key, boundary, selected string) oawruntime.RunReply {
	t.Helper()
	reply, err := engine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalSwitchProfile, StableBoundarySwitch: &oawruntime.StableBoundarySwitch{
			Boundary: boundary, Selection: oawruntime.ProfileSelection{Profile: selected},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}
