package coordinator

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestPrepareIssuesTopologyBoundGrantAndDispatchPacket(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "prepare-read")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	engine, err := NewEngine(Options{
		StateRoot: stateRoot, Core: compiler,
		Authority: admissionAuthority([]string{"read-project"}, []string{"project"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	started, err := engine.Exchange(start)
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	prepare := prepareTestCommand(start, "prepare-read", []string{"read-project"}, []string{"project"})
	result, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatalf("PREPARE error = %v", err)
	}
	if result.Kind != ResultDispatch || result.Snapshot == nil || result.Dispatch == nil || result.Snapshot.Status != StatusPrepared ||
		result.Revision != started.Revision+1 || result.Snapshot.ActiveGrant == nil || len(result.Snapshot.GrantHistory) != 1 {
		t.Fatalf("PREPARE result = %#v", result)
	}
	grant := *result.Snapshot.ActiveGrant
	packet := *result.Dispatch
	if grant.WorkflowID != started.WorkflowID || grant.BundleID != started.Snapshot.Bundles[0].ID || grant.BundleGeneration != 1 ||
		grant.BundleDigest != started.Snapshot.Bundles[0].Digest || grant.NodeID != started.Snapshot.ActiveNodeID || grant.Topology != execution.TopologyCurrent ||
		grant.HostSessionDigest != start.Start.HostSession.Digest || !reflect.DeepEqual(grant.Effects, []string{"read-project"}) || !reflect.DeepEqual(grant.Resources, []string{"project"}) {
		t.Fatalf("Grant pins = %#v", grant)
	}
	if packet.WorkflowID != grant.WorkflowID || packet.RequestID != grant.RequestID || packet.BundleID != grant.BundleID || packet.BundleGeneration != grant.BundleGeneration ||
		packet.BundleDigest != grant.BundleDigest || packet.NodeID != grant.NodeID || packet.Topology != grant.Topology || packet.HostSessionDigest != grant.HostSessionDigest ||
		packet.EnvironmentReportDigest != start.Start.Environment.Digest || !reflect.DeepEqual(packet.Grant, grant) || packet.Digest == "" {
		t.Fatalf("Dispatch Packet pins = %#v", packet)
	}
	raw, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"executor", "executor_id", "process_command"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("Dispatch Packet contains retired field %q: %s", forbidden, raw)
		}
	}
}

func TestPrepareReplayAndSingleActiveGrant(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "prepare-replay")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	engine, err := NewEngine(Options{StateRoot: stateRoot, Core: compiler, Authority: admissionAuthority([]string{"read-project"}, []string{"project"})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepare := prepareTestCommand(start, "prepare-replay", []string{"read-project"}, []string{"project"})
	first, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatal(err)
	}
	prepare.MessageID = "prepare-retried"
	replayed, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatalf("PREPARE replay error = %v", err)
	}
	if !replayed.Replayed || replayed.Revision != first.Revision || compiler.compileCalls != 1 {
		t.Fatalf("PREPARE replay = %#v, compile calls = %d", replayed, compiler.compileCalls)
	}
	conflict := prepareTestCommand(start, "prepare-conflict", []string{"read-project"}, []string{"project"})
	conflict.ExpectedRevision = first.Revision
	if _, err := engine.Exchange(conflict); ErrorCode(err) != "WORKFLOW_PREPARE_INVALID" {
		t.Fatalf("second active PREPARE error = %v", err)
	}
	current, err := engine.journal.inspect(first.WorkflowID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Revision != first.Revision || current.Snapshot.ActiveGrant == nil {
		t.Fatalf("second PREPARE changed state: %#v", current)
	}
}

func TestPrepareWriteRequiresAndCommitsProjectLease(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := filepath.Join(t.TempDir(), "project")
	start := startTestCommand(t, "prepare-write")
	compiler := &startTestCore{
		t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey),
		mutateBundle: func(bundle *core.LifecycleBundle) {
			bundle.Graph.Nodes[0].MaximumEffects = []string{"read-project", "write-project"}
			bundle.Graph.Nodes[0].Resources = []string{"project-worktree"}
			bundle.ProviderInstances = append([]profile.GraphProviderInstance{}, bundle.Graph.ProviderInstances...)
		},
	}
	engine, err := NewEngine(Options{
		StateRoot: stateRoot, PhysicalProjectRoot: projectRoot, Core: compiler,
		Authority: admissionAuthority([]string{"read-project", "write-project"}, []string{"project-worktree"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepare := prepareTestCommand(start, "prepare-write", []string{"write-project"}, []string{"project-worktree"})
	result, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatalf("write PREPARE error = %v", err)
	}
	if result.Snapshot == nil || len(result.Snapshot.ResourceLeases) != 1 || result.Snapshot.ResourceLeases[0].PhysicalRoot != projectRoot || result.Snapshot.ResourceLeases[0].GrantID != result.Snapshot.ActiveGrant.ID {
		t.Fatalf("write PREPARE lease = %#v", result.Snapshot.ResourceLeases)
	}
}

func TestPrepareRejectsEffectResourceAndBundleScopeExpansionWithoutCommit(t *testing.T) {
	for _, test := range []struct {
		name    string
		effects []string
		res     []string
	}{
		{name: "effect", effects: []string{"write-project"}, res: []string{"project"}},
		{name: "resource", effects: []string{"read-project"}, res: []string{"project-worktree"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "state")
			start := startTestCommand(t, "prepare-invalid-"+test.name)
			compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
			engine, err := NewEngine(Options{StateRoot: stateRoot, Core: compiler, Authority: admissionAuthority([]string{"read-project"}, []string{"project"})})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Exchange(start); err != nil {
				t.Fatal(err)
			}
			prepare := prepareTestCommand(start, "prepare-invalid-"+test.name, test.effects, test.res)
			if _, err := engine.Exchange(prepare); ErrorCode(err) != "CAPABILITY_AUTHORITY_EXCEEDED" {
				t.Fatalf("invalid PREPARE error = %v", err)
			}
			current, err := engine.journal.inspect(deriveWorkflowID(start.IdempotencyKey))
			if err != nil {
				t.Fatal(err)
			}
			if current.Revision != 1 || current.Snapshot.ActiveGrant != nil {
				t.Fatalf("invalid PREPARE changed state: %#v", current)
			}
		})
	}
}

func TestDispatchPacketAndSnapshotValidationRejectsTampering(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "dispatch-invariants-start")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	engine, err := NewEngine(Options{StateRoot: stateRoot, Core: compiler, Authority: admissionAuthority([]string{"read-project"}, []string{"project"})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Exchange(prepareTestCommand(start, "dispatch-invariants-prepare", []string{"read-project"}, []string{"project"}))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*Result)
	}{
		{name: "packet ID", mutate: func(value *Result) { value.Dispatch.ID = "dispatch-00000000000000000000000000000000" }},
		{name: "packet digest", mutate: func(value *Result) { value.Dispatch.Digest = strings.Repeat("0", 64) }},
		{name: "packet Bundle", mutate: func(value *Result) { value.Dispatch.BundleGeneration++ }},
		{name: "packet Grant", mutate: func(value *Result) { value.Dispatch.Grant.TerminationCondition = "changed" }},
		{name: "active Grant", mutate: func(value *Result) { value.Snapshot.ActiveGrant.TerminationCondition = "changed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := result
			snapshot := cloneSnapshot(*result.Snapshot)
			packet := *result.Dispatch
			packet.Grant = admission.CloneGrant(result.Dispatch.Grant)
			packet.InputReferences = append([]ArtifactReference{}, result.Dispatch.InputReferences...)
			packet.EvidenceRequirements = append([]EvidenceRequirement{}, result.Dispatch.EvidenceRequirements...)
			packet.EnvironmentRequirements = append([]execution.EnvironmentRequirement{}, result.Dispatch.EnvironmentRequirements...)
			candidate.Snapshot, candidate.Dispatch, candidate.Digest = &snapshot, &packet, ""
			candidate.Snapshot.ProcessedMessages = clearResultPinForRevision(candidate.Snapshot.ProcessedMessages, candidate.Revision)
			test.mutate(&candidate)
			if _, err := normalizeResult(candidate); ErrorCode(err) != "WORKFLOW_DISPATCH_INVALID" {
				t.Fatalf("normalizeResult(tampered %s) error = %v", test.name, err)
			}
		})
	}
}

func admissionAuthority(effects, resources []string) admission.AuthorityCeiling {
	return admission.AuthorityCeiling{Effects: effects, Resources: resources, ResourceLeases: true}
}

func prepareTestCommand(start Command, key string, effects, resources []string) Command {
	return Command{
		SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandPrepare, MessageID: "message-" + key, IdempotencyKey: "command-" + key,
		WorkflowID: deriveWorkflowID(start.IdempotencyKey), ExpectedRevision: 1,
		Prepare: &PrepareInput{RequestedEffects: effects, RequestedResources: resources, TerminationCondition: "complete", InputReferences: []ArtifactReference{}, EvidenceRequirements: []EvidenceRequirement{}},
	}
}
