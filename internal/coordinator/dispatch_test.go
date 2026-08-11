package coordinator

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestDispatchV2PrepareIssuesTopologyBoundGrantAndDispatchPacket(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "prepare-read")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	options := startTestOptions(t, stateRoot, compiler)
	options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	started, err := engine.Exchange(start)
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	prepare := prepareTestCommand(start, "prepare-read", []string{"read-project"}, []string{"project-worktree"})
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
		grant.BundleDigest != started.Snapshot.Bundles[0].Digest || grant.Cursor != started.Snapshot.Cursor || grant.Topology != execution.TopologyCurrent ||
		grant.HostSessionDigest != start.Start.HostSession.Digest || !reflect.DeepEqual(grant.Effects, []string{"read-project"}) || !reflect.DeepEqual(grant.Resources, []string{"project-worktree"}) {
		t.Fatalf("Grant pins = %#v", grant)
	}
	if packet.WorkflowID != grant.WorkflowID || packet.RequestID != grant.RequestID || packet.BundleID != grant.BundleID || packet.BundleGeneration != grant.BundleGeneration ||
		packet.BundleDigest != grant.BundleDigest || packet.Cursor != grant.Cursor || packet.TargetKind != grant.Target.TargetKind || packet.Topology != grant.Topology || packet.HostSessionDigest != grant.HostSessionDigest ||
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
	options := startTestOptions(t, stateRoot, compiler)
	options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepare := prepareTestCommand(start, "prepare-replay", []string{"read-project"}, []string{"project-worktree"})
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
	conflict := prepareTestCommand(start, "prepare-conflict", []string{"read-project"}, []string{"project-worktree"})
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
	projectRoot := t.TempDir()
	start := startTestCommand(t, "prepare-write")
	compiler := &startTestCore{
		t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey),
		mutateBundle: func(bundle *core.LifecycleBundle) {
			binding := firstStartTestBinding(t, bundle)
			binding.MaximumEffects = []string{"read-project", "write-project"}
			binding.Resources = []string{"project-worktree"}
		},
	}
	options := startTestOptions(t, stateRoot, compiler)
	options.PhysicalProjectRoot = projectRoot
	options.Authority = admissionAuthority([]string{"read-project", "write-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
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
	physicalProjectRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result.Snapshot == nil || len(result.Snapshot.ResourceLeases) != 1 || result.Snapshot.ResourceLeases[0].PhysicalRoot != physicalProjectRoot || result.Snapshot.ResourceLeases[0].GrantID != result.Snapshot.ActiveGrant.ID {
		t.Fatalf("write PREPARE lease = %#v", result.Snapshot.ResourceLeases)
	}
}

func TestPrepareRejectsEffectResourceAndBundleScopeExpansionWithoutCommit(t *testing.T) {
	for _, test := range []struct {
		name    string
		effects []string
		res     []string
	}{
		{name: "effect", effects: []string{"write-project"}, res: []string{"project-worktree"}},
		{name: "resource", effects: []string{"read-project"}, res: []string{"network"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "state")
			start := startTestCommand(t, "prepare-invalid-"+test.name)
			compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
			options := startTestOptions(t, stateRoot, compiler)
			options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
			engine, err := NewEngine(options)
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
	options := startTestOptions(t, stateRoot, compiler)
	options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Exchange(prepareTestCommand(start, "dispatch-invariants-prepare", []string{"read-project"}, []string{"project-worktree"}))
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

func TestDispatchResultRejectsSemanticallyReSignedBundleClosureTampering(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "dispatch-closure-start")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	options := startTestOptions(t, stateRoot, compiler)
	options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Exchange(prepareTestCommand(start, "dispatch-closure-prepare", []string{"read-project"}, []string{"project-worktree"}))
	if err != nil {
		t.Fatal(err)
	}

	mutations := []struct {
		name   string
		mutate func(*DispatchPacket)
	}{
		{name: "Bundle digest", mutate: func(packet *DispatchPacket) {
			packet.BundleDigest = strings.Repeat("f", 64)
			packet.Grant.BundleDigest = packet.BundleDigest
		}},
		{name: "topology", mutate: func(packet *DispatchPacket) {
			packet.Topology = execution.TopologySubagent
			packet.Grant.Topology = packet.Topology
		}},
		{name: "Host session", mutate: func(packet *DispatchPacket) {
			packet.HostSessionDigest = strings.Repeat("f", 64)
			packet.Grant.HostSessionDigest = packet.HostSessionDigest
		}},
		{name: "environment report", mutate: func(packet *DispatchPacket) {
			packet.EnvironmentReportDigest = strings.Repeat("f", 64)
		}},
		{name: "environment requirements", mutate: func(packet *DispatchPacket) {
			packet.EnvironmentRequirements = []execution.EnvironmentRequirement{{
				Surface: "skills", Required: true,
				AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited},
			}}
		}},
		{name: "Provider Binding target", mutate: func(packet *DispatchPacket) {
			packet.Grant.Target.ProviderBinding.Reference = "superpowers:writing-plans"
		}},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneDispatchResultForTamper(result)
			test.mutate(candidate.Dispatch)
			resealDispatchPacket(t, candidate.Dispatch)
			grant := admission.CloneGrant(candidate.Dispatch.Grant)
			candidate.Snapshot.ActiveGrant = &grant
			candidate.Snapshot.GrantHistory[len(candidate.Snapshot.GrantHistory)-1] = admission.CloneGrant(grant)
			if _, err := normalizeResult(candidate); ErrorCode(err) != "WORKFLOW_DISPATCH_INVALID" {
				t.Fatalf("normalizeResult(re-signed %s) error = %v", test.name, err)
			}
		})
	}
}

func TestSnapshotRejectsSemanticallyReSignedActiveGrantOutsideBundle(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "snapshot-grant-closure-start")
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey)}
	options := startTestOptions(t, stateRoot, compiler)
	options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
	engine, err := NewEngine(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepared, err := engine.Exchange(prepareTestCommand(start, "snapshot-grant-closure-prepare", []string{"read-project"}, []string{"project-worktree"}))
	if err != nil {
		t.Fatal(err)
	}
	candidate := cloneDispatchResultForTamper(prepared)
	candidate.Kind = ResultState
	candidate.Dispatch = nil
	candidate.Snapshot.ActiveGrant.BundleID = "bundle-00000000000000000000000000000000"
	resealGrant(t, candidate.Snapshot.ActiveGrant)
	candidate.Snapshot.GrantHistory[len(candidate.Snapshot.GrantHistory)-1] = admission.CloneGrant(*candidate.Snapshot.ActiveGrant)
	if _, err := normalizeResult(candidate); ErrorCode(err) != "WORKFLOW_STATE_REVISION_INVALID" {
		t.Fatalf("normalizeResult(re-signed active Grant) error = %v", err)
	}
}

func TestPrepareCommitsBundlePinnedAuthorizationHistories(t *testing.T) {
	t.Run("User Authorization", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		start := startTestCommand(t, "authorization-history-start")
		compiler := &startTestCore{
			t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey),
			mutateBundle: func(bundle *core.LifecycleBundle) {
				binding := firstStartTestBinding(t, bundle)
				binding.MaximumEffects = []string{"network-write", "read-project", "write-project"}
			},
		}
		options := startTestOptions(t, stateRoot, compiler)
		options.Authority = admissionAuthority([]string{"network-write"}, []string{"project-worktree"})
		engine, err := NewEngine(options)
		if err != nil {
			t.Fatal(err)
		}
		started, err := engine.Exchange(start)
		if err != nil {
			t.Fatal(err)
		}
		bundle := started.Snapshot.Bundles[0]
		unit, err := profile.UnitAtCursor(bundle.Graph, started.Snapshot.Cursor)
		if err != nil {
			t.Fatal(err)
		}
		capability := options.capabilityResolver(*unit.ProviderBinding)
		target, err := admission.NewProviderBindingAuthority(*unit.ProviderBinding, *capability)
		if err != nil {
			t.Fatal(err)
		}
		authorization, err := admission.NewUserAuthorization(admission.UserAuthorization{
			SchemaVersion: admission.UserAuthorizationSchemaV1, IssuerHostID: bundle.HostID, HostSessionDigest: bundle.HostSessionDigest,
			EvidenceHandleDigest: strings.Repeat("7", 64), AuthorizationNonce: "authorization-history-nonce", WorkflowID: started.WorkflowID,
			BundleID: bundle.ID, BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, Cursor: started.Snapshot.Cursor,
			Target: admission.AuthorizationTarget{TargetKind: admission.GrantProviderBinding, ProviderBinding: &target}, Decision: admission.AuthorizationAllowed,
			Effects: []string{"network-write"}, Resources: []string{"project-worktree"},
			Evidence: []host.EvidenceReference{{Kind: "approval", Reference: "evidence://authorization/history", Digest: strings.Repeat("8", 64)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		prepare := prepareTestCommand(start, "authorization-history-prepare", []string{"network-write"}, []string{"project-worktree"})
		prepare.Prepare.Authorization = &authorization
		prepared, err := engine.Exchange(prepare)
		if err != nil {
			t.Fatal(err)
		}
		if len(prepared.Snapshot.UserAuthorizations) != 1 || prepared.Snapshot.ActiveGrant.AuthorizationDigest != authorization.Digest {
			t.Fatalf("User Authorization history = %#v", prepared.Snapshot)
		}
	})

	t.Run("Invocation Attestation", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		start := startTestCommand(t, "invocation-history-start")
		compiler := &startTestCore{
			t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey),
			mutateBundle: func(bundle *core.LifecycleBundle) {
				binding := firstStartTestBinding(t, bundle)
				binding.Invocation = catalog.InvocationHumanExplicit
				binding.RequiresExplicitInvocation = true
			},
		}
		options := startTestOptions(t, stateRoot, compiler)
		options.Authority = admissionAuthority([]string{"read-project"}, []string{"project-worktree"})
		engine, err := NewEngine(options)
		if err != nil {
			t.Fatal(err)
		}
		started, err := engine.Exchange(start)
		if err != nil {
			t.Fatal(err)
		}
		bundle := started.Snapshot.Bundles[0]
		unit, err := profile.UnitAtCursor(bundle.Graph, started.Snapshot.Cursor)
		if err != nil {
			t.Fatal(err)
		}
		capability := options.capabilityResolver(*unit.ProviderBinding)
		target, err := admission.NewProviderBindingAuthority(*unit.ProviderBinding, *capability)
		if err != nil {
			t.Fatal(err)
		}
		attestation, err := admission.NewExplicitInvocationAttestation(admission.ExplicitInvocationAttestation{
			SchemaVersion: admission.ExplicitInvocationAttestationSchemaV1, IssuerHostID: bundle.HostID, HostSessionDigest: bundle.HostSessionDigest,
			EvidenceHandleDigest: strings.Repeat("7", 64), InvocationNonce: "invocation-history-nonce", WorkflowID: started.WorkflowID,
			BundleID: bundle.ID, BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, Cursor: started.Snapshot.Cursor,
			ProviderBinding: target,
			Evidence:        []host.EvidenceReference{{Kind: "invocation", Reference: "evidence://invocation/history", Digest: strings.Repeat("9", 64)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		prepare := prepareTestCommand(start, "invocation-history-prepare", []string{"read-project"}, []string{"project-worktree"})
		prepare.Prepare.InvocationAttestation = &attestation
		prepared, err := engine.Exchange(prepare)
		if err != nil {
			t.Fatal(err)
		}
		if len(prepared.Snapshot.InvocationAttestations) != 1 || prepared.Snapshot.ActiveGrant.InvocationAttestationDigest != attestation.Digest {
			t.Fatalf("Invocation Attestation history = %#v", prepared.Snapshot)
		}
	})
}

func cloneDispatchResultForTamper(value Result) Result {
	result := value
	snapshot := cloneSnapshot(*value.Snapshot)
	packet := *value.Dispatch
	packet.Grant = admission.CloneGrant(value.Dispatch.Grant)
	packet.InputReferences = append([]ArtifactReference{}, value.Dispatch.InputReferences...)
	packet.EvidenceRequirements = append([]EvidenceRequirement{}, value.Dispatch.EvidenceRequirements...)
	packet.EnvironmentRequirements = append([]execution.EnvironmentRequirement{}, value.Dispatch.EnvironmentRequirements...)
	result.Snapshot, result.Dispatch, result.Digest = &snapshot, &packet, ""
	result.Snapshot.ProcessedMessages = clearResultPinForRevision(result.Snapshot.ProcessedMessages, result.Revision)
	return result
}

func resealGrant(t testing.TB, value *admission.CapabilityGrant) {
	t.Helper()
	value.ID, value.Digest = "", ""
	identity, _, err := canonicaljson.Digest(*value)
	if err != nil {
		t.Fatal(err)
	}
	value.ID = "grant-" + identity[:32]
	value.Digest, _, err = canonicaljson.Digest(*value)
	if err != nil {
		t.Fatal(err)
	}
}

func resealDispatchPacket(t testing.TB, value *DispatchPacket) {
	t.Helper()
	resealGrant(t, &value.Grant)
	value.ID, value.Digest = "", ""
	identity, _, err := canonicaljson.Digest(*value)
	if err != nil {
		t.Fatal(err)
	}
	value.ID = "dispatch-" + identity[:32]
	value.Digest, _, err = canonicaljson.Digest(*value)
	if err != nil {
		t.Fatal(err)
	}
}

func admissionAuthority(effects, resources []string) admission.AuthorityCeiling {
	return admission.AuthorityCeiling{Effects: effects, Resources: resources, ResourceLeases: true}
}

func prepareTestCommand(start Command, key string, effects, resources []string) Command {
	return Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandPrepare, MessageID: "message-" + key, IdempotencyKey: "command-" + key,
		WorkflowID: deriveWorkflowID(start.IdempotencyKey), ExpectedRevision: 1,
		Prepare: &PrepareInput{RequestedEffects: effects, RequestedResources: resources, TerminationCondition: "complete", InputReferences: []ArtifactReference{}, EvidenceRequirements: []EvidenceRequirement{}},
	}
}
