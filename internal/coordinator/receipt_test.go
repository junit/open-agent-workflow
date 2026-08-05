package coordinator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestReceiptStartedAndTerminalCompletedAdvanceFromPinnedDispatch(t *testing.T) {
	engine, start, prepared := preparedReceiptWorkflow(t, nil)
	startedCommand := receiptTestCommand(t, prepared, "receipt-started", host.ReceiptStarted, "", "")
	started, err := engine.Exchange(startedCommand)
	if err != nil {
		t.Fatalf("STARTED error = %v", err)
	}
	if started.Kind != ResultState || started.Snapshot == nil || started.Snapshot.Status != StatusInFlight ||
		started.Snapshot.ActiveGrant == nil || len(started.Snapshot.Receipts) != 1 {
		t.Fatalf("STARTED Result = %#v", started)
	}

	completedCommand := receiptTestCommand(t, prepared, "receipt-completed", host.ReceiptCompleted, "", "")
	completedCommand.ExpectedRevision = started.Revision
	completed, err := engine.Exchange(completedCommand)
	if err != nil {
		t.Fatalf("COMPLETED error = %v", err)
	}
	if completed.Snapshot == nil || completed.Snapshot.Status != StatusFinished || completed.Snapshot.ActiveGrant != nil ||
		len(completed.Snapshot.Receipts) != 2 || completed.WorkflowID != deriveWorkflowID(start.IdempotencyKey) {
		t.Fatalf("COMPLETED Result = %#v", completed)
	}
	completedCommand.MessageID = "message-retried"
	replayed, err := engine.Exchange(completedCommand)
	if err != nil || !replayed.Replayed || replayed.Revision != completed.Revision {
		t.Fatalf("COMPLETED replay = %#v, %v", replayed, err)
	}
}

func TestReceiptRejectsDispatchPinAndStatusMismatchWithoutCommit(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*host.InvocationReceipt)
	}{
		{name: "Workflow", mutate: func(value *host.InvocationReceipt) { value.WorkflowID = "workflow-00000000000000000000000000000000" }},
		{name: "Bundle generation", mutate: func(value *host.InvocationReceipt) { value.BundleGeneration++ }},
		{name: "Bundle digest", mutate: func(value *host.InvocationReceipt) { value.BundleDigest = strings.Repeat("f", 64) }},
		{name: "node", mutate: func(value *host.InvocationReceipt) { value.NodeID = "different" }},
		{name: "topology", mutate: func(value *host.InvocationReceipt) {
			value.Topology = execution.TopologySubagent
			value.ContextFreshness = host.ContextFresh
			value.InvocationHandle = "different-invocation"
		}},
		{name: "Host session", mutate: func(value *host.InvocationReceipt) { value.HostSessionDigest = strings.Repeat("f", 64) }},
		{name: "Dispatch", mutate: func(value *host.InvocationReceipt) { value.DispatchDigest = strings.Repeat("f", 64) }},
		{name: "environment report", mutate: func(value *host.InvocationReceipt) { value.EnvironmentReportDigest = strings.Repeat("f", 64) }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			engine, _, prepared := preparedReceiptWorkflow(t, nil)
			command := receiptTestCommand(t, prepared, "receipt-mismatch-"+strings.ReplaceAll(test.name, " ", "-"), host.ReceiptStarted, "", "")
			value := command.Receipt.Receipt
			value.Digest = ""
			test.mutate(&value)
			normalized, err := host.NewInvocationReceipt(value)
			if err != nil {
				t.Fatal(err)
			}
			command.Receipt.Receipt = normalized
			if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_RECEIPT_INVALID" {
				t.Fatalf("mismatched Receipt error = %v", err)
			}
			current, err := engine.journal.inspect(prepared.WorkflowID)
			if err != nil || current.Revision != prepared.Revision || current.Snapshot.Status != StatusPrepared {
				t.Fatalf("mismatched Receipt changed state: %#v, %v", current, err)
			}
		})
	}

	engine, _, prepared := preparedReceiptWorkflow(t, nil)
	completed := receiptTestCommand(t, prepared, "receipt-before-start", host.ReceiptCompleted, "", "")
	if _, err := engine.Exchange(completed); ErrorCode(err) != "WORKFLOW_RECEIPT_INVALID" {
		t.Fatalf("COMPLETED before STARTED error = %v", err)
	}
}

func TestReceiptSubagentInvocationHandleRemainsPinnedToStarted(t *testing.T) {
	engine, _, prepared := preparedSubagentReceiptWorkflow(t)
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "subagent-started", host.ReceiptStarted, "", ""))
	completed := receiptTestCommand(t, prepared, "subagent-completed", host.ReceiptCompleted, "", "")
	completed.ExpectedRevision = started.Revision
	completed.Receipt.Receipt.Digest = ""
	completed.Receipt.Receipt.InvocationHandle = "different-invocation"
	completed.Receipt.Receipt, _ = host.NewInvocationReceipt(completed.Receipt.Receipt)
	if _, err := engine.Exchange(completed); ErrorCode(err) != "WORKFLOW_RECEIPT_INVALID" {
		t.Fatalf("changed SUBAGENT handle error = %v", err)
	}
	current, err := engine.journal.inspect(prepared.WorkflowID)
	if err != nil || current.Revision != started.Revision || current.Snapshot.Status != StatusInFlight {
		t.Fatalf("changed SUBAGENT handle changed state: %#v, %v", current, err)
	}
}

func TestReceiptCompletedIncidentPausedAndCancelledTransitions(t *testing.T) {
	t.Run("declared graph edge", func(t *testing.T) {
		engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
		started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "edge-started", host.ReceiptStarted, "", ""))
		completed := receiptTestCommand(t, prepared, "edge-completed", host.ReceiptCompleted, "succeeded", "")
		completed.ExpectedRevision = started.Revision
		result := exchangeReceipt(t, engine, completed)
		if result.Snapshot.Status != StatusReady || result.Snapshot.ActiveNodeID != "completion" || result.Snapshot.ActiveGrant != nil {
			t.Fatalf("edge Result = %#v", result)
		}
	})

	t.Run("declared incident", func(t *testing.T) {
		engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
		started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "incident-started", host.ReceiptStarted, "", ""))
		failed := receiptTestCommand(t, prepared, "incident-failed", host.ReceiptFailed, "", "build-failure")
		failed.ExpectedRevision = started.Revision
		result := exchangeReceipt(t, engine, failed)
		if result.Snapshot.Status != StatusReady || result.Snapshot.ActiveNodeID != "build-repair" || result.Snapshot.ActiveGrant != nil {
			t.Fatalf("incident Result = %#v", result)
		}
	})

	t.Run("unrouted incident pauses", func(t *testing.T) {
		engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
		started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "unrouted-started", host.ReceiptStarted, "", ""))
		failed := receiptTestCommand(t, prepared, "unrouted-failed", host.ReceiptFailed, "", "unknown-failure")
		failed.ExpectedRevision = started.Revision
		result := exchangeReceipt(t, engine, failed)
		if result.Snapshot.Status != StatusPaused || result.Snapshot.ActiveGrant != nil || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "WORKFLOW_INCIDENT_UNROUTED" {
			t.Fatalf("unrouted Result = %#v", result)
		}
	})

	for _, test := range []struct {
		name       string
		kind       host.ReceiptKind
		wantStatus Status
		active     bool
	}{
		{name: "paused", kind: host.ReceiptPaused, wantStatus: StatusPaused, active: true},
		{name: "cancelled", kind: host.ReceiptCancelled, wantStatus: StatusCancelled, active: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, _, prepared := preparedReceiptWorkflow(t, nil)
			started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, test.name+"-started", host.ReceiptStarted, "", ""))
			command := receiptTestCommand(t, prepared, test.name+"-terminal", test.kind, "", "")
			command.ExpectedRevision = started.Revision
			result := exchangeReceipt(t, engine, command)
			if result.Snapshot.Status != test.wantStatus || (result.Snapshot.ActiveGrant != nil) != test.active || len(result.Snapshot.Receipts) != 2 {
				t.Fatalf("%s Result = %#v", test.name, result)
			}
		})
	}
}

func TestReceiptCompletedRequiresDeclaredEvidenceAndSignal(t *testing.T) {
	engine, _, prepared := preparedReceiptWorkflow(t, receiptGraphBundle)
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "evidence-started", host.ReceiptStarted, "", ""))
	missing := receiptTestCommand(t, prepared, "evidence-missing", host.ReceiptCompleted, "succeeded", "")
	missing.ExpectedRevision = started.Revision
	missing.Receipt.Receipt.Evidence[0].Kind = "different"
	missing.Receipt.Receipt.Digest = ""
	missing.Receipt.Receipt, _ = host.NewInvocationReceipt(missing.Receipt.Receipt)
	if _, err := engine.Exchange(missing); ErrorCode(err) != "WORKFLOW_EVIDENCE_INCOMPLETE" {
		t.Fatalf("missing evidence error = %v", err)
	}
	undeclared := receiptTestCommand(t, prepared, "signal-undeclared", host.ReceiptCompleted, "invented", "")
	undeclared.ExpectedRevision = started.Revision
	if _, err := engine.Exchange(undeclared); ErrorCode(err) != "WORKFLOW_SIGNAL_UNDECLARED" {
		t.Fatalf("undeclared signal error = %v", err)
	}
}

func preparedReceiptWorkflow(t *testing.T, mutate func(*core.LifecycleBundle)) (*Engine, Command, Result) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "receipt-workflow-"+strings.ReplaceAll(t.Name(), "/", "-"))
	compiler := &startTestCore{t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey), mutateBundle: mutate}
	engine, err := NewEngine(Options{StateRoot: stateRoot, Core: compiler, Authority: admissionAuthority([]string{"read-project"}, []string{"project"})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepare := prepareTestCommand(start, "receipt-prepare-"+strings.ReplaceAll(t.Name(), "/", "-"), []string{"read-project"}, []string{"project"})
	prepare.Prepare.EvidenceRequirements = []EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "completion report"}}
	prepared, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatal(err)
	}
	return engine, start, prepared
}

func preparedSubagentReceiptWorkflow(t *testing.T) (*Engine, Command, Result) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "receipt-subagent-"+strings.ReplaceAll(t.Name(), "/", "-"))
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []string{"skill"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features: []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-child", ParentSessionID: "session-current",
		Topology: execution.TopologySubagent, Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "2.0.0",
		SessionID: "session-current", SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: strings.Repeat("a", 64), EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	start.Start.Selection.Topology = execution.TopologySubagent
	start.Start.HostSession, start.Start.Environment = session, environment
	compiler := &startTestCore{
		t: t, stateRoot: stateRoot, workflowID: deriveWorkflowID(start.IdempotencyKey),
		mutateBundle: func(bundle *core.LifecycleBundle) {
			bundle.Graph.EligibleTopologies = []execution.Topology{execution.TopologySubagent}
			bundle.Graph.Nodes[0].SupportedTopologies = []execution.Topology{execution.TopologySubagent}
			bundle.Graph.Nodes[0].Binding.Topologies = []execution.Topology{execution.TopologySubagent}
		},
	}
	engine, err := NewEngine(Options{StateRoot: stateRoot, Core: compiler, Authority: admissionAuthority([]string{"read-project"}, []string{"project"})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(start); err != nil {
		t.Fatal(err)
	}
	prepare := prepareTestCommand(start, "receipt-subagent-prepare-"+strings.ReplaceAll(t.Name(), "/", "-"), []string{"read-project"}, []string{"project"})
	prepare.Prepare.EvidenceRequirements = []EvidenceRequirement{{Kind: "report", Minimum: 1, Description: "completion report"}}
	prepared, err := engine.Exchange(prepare)
	if err != nil {
		t.Fatal(err)
	}
	return engine, start, prepared
}

func receiptTestCommand(t *testing.T, prepared Result, key string, kind host.ReceiptKind, signal, failureCode string) Command {
	t.Helper()
	packet := prepared.Dispatch
	receipt := host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: kind, WorkflowID: packet.WorkflowID,
		BundleGeneration: packet.BundleGeneration, BundleDigest: packet.BundleDigest, NodeID: packet.NodeID,
		Topology: packet.Topology, HostSessionDigest: packet.HostSessionDigest, DispatchDigest: packet.Digest,
		ContextFreshness: host.ContextShared, EnvironmentReportDigest: packet.EnvironmentReportDigest,
	}
	if packet.Topology == execution.TopologySubagent {
		receipt.ContextFreshness = host.ContextFresh
		receipt.InvocationHandle = "child-invocation-1"
	}
	switch kind {
	case host.ReceiptPaused:
		receipt.Outcome = "paused"
	case host.ReceiptCompleted:
		receipt.Outcome = "succeeded"
		receipt.Evidence = []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("e", 64)}}
	case host.ReceiptFailed:
		receipt.Outcome = "failed"
		receipt.FailureCode = failureCode
		receipt.Evidence = []host.EvidenceReference{{Kind: "diagnostic", Reference: "evidence://failure", Digest: strings.Repeat("f", 64)}}
	case host.ReceiptCancelled:
		receipt.Outcome = "cancelled"
	}
	normalized, err := host.NewInvocationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return Command{
		SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandReceipt, MessageID: "message-" + key, IdempotencyKey: "command-" + key,
		WorkflowID: prepared.WorkflowID, ExpectedRevision: prepared.Revision,
		Receipt: &ReceiptInput{Receipt: normalized, Signal: signal},
	}
}

func exchangeReceipt(t *testing.T, engine *Engine, command Command) Result {
	t.Helper()
	result, err := engine.Exchange(command)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func receiptGraphBundle(bundle *core.LifecycleBundle) {
	entry := bundle.Graph.Nodes[0]
	entry.Transitions = []profile.GraphTransition{{Signal: "succeeded", Target: "completion"}}
	completion := entry
	completion.ID, completion.Responsibility, completion.Phase = "completion", "completion", "completion"
	completion.Transitions = []profile.GraphTransition{}
	repair := entry
	repair.ID, repair.Responsibility, repair.Phase = "build-repair", "build repair", "debugging"
	repair.Transitions = []profile.GraphTransition{}
	bundle.Graph.Nodes = []profile.GraphNode{entry, completion, repair}
	bundle.Graph.IncidentRoutes = []profile.GraphIncidentRoute{{Incident: "build-failure", Handler: "build-repair"}}
	bundle.Graph.TerminalGates = []string{"completion"}
	bundle.Graph.StableBoundaries = []string{"completion"}
}
