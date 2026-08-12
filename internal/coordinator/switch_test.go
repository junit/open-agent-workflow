package coordinator

import (
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestSwitchRecompilesNextBundleGenerationInsideWorkflowLock(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "switch-workflow")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	compiler := engine.core.(*startTestCore)
	baseMutation := compiler.mutateBundle
	compiler.mutateBundle = func(bundle *core.LifecycleBundle) {
		baseMutation(bundle)
		receiptGraphBundle(bundle)
	}
	exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "switch-prepare", []string{"read-project"}))
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "switch-started", host.ReceiptStarted, "", ""))
	completed := receiptTestCommand(t, prepared, "switch-completed", host.ReceiptCompleted, "succeeded", "")
	completed.ExpectedRevision = started.Revision
	completed.Receipt.StableBoundary = "problem-framing-complete"
	ready := exchangeReceipt(t, engine, completed)

	command := Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandSwitch, MessageID: "message-switch", IdempotencyKey: "switch-command",
		WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision,
		Switch: &SwitchInput{Boundary: "problem-framing-complete", Selection: start.Start.Selection, HostSession: start.Start.HostSession, Environment: start.Start.Environment},
	}
	command.Switch.Selection.Profile = "MATT-FULL"
	graphSelection := startTestGraphSelection()
	graphSelection.Profile = "MATT-FULL"
	graphSelection.Digest = ""
	command.Switch.Selection.GraphSelectionDigest = startTestDigest(graphSelection)
	switched := exchangeTask6(t, engine, command)
	if switched.Snapshot.ActiveGeneration != 2 || len(switched.Snapshot.Bundles) != 2 || switched.Snapshot.Cursor != firstStartTestCursor(t, switched.Snapshot.Bundles[1].Graph) ||
		switched.Snapshot.LastStableBoundary != "" || switched.Snapshot.ActiveGrant != nil || compiler.compileCalls != 2 || !compiler.compileInsideLock ||
		switched.Snapshot.Bundles[1].Selection.Profile != "MATT-FULL" {
		t.Fatalf("SWITCH result = %#v, compiler = %#v", switched, compiler)
	}
	command.MessageID = "message-switch-replay"
	replayed := exchangeTask6(t, engine, command)
	if !replayed.Replayed || replayed.Revision != switched.Revision {
		t.Fatalf("SWITCH replay = %#v", replayed)
	}
	oldReceipt := receiptTestCommand(t, prepared, "switch-old-grant", host.ReceiptStarted, "", "")
	oldReceipt.ExpectedRevision = switched.Revision
	if _, err := engine.Exchange(oldReceipt); ErrorCode(err) != "WORKFLOW_RECEIPT_INVALID" {
		t.Fatalf("old Grant Receipt error = %v", err)
	}
}

func TestSwitchRejectsCompilerRewrittenTrustedSelection(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "switch-rewritten-selection")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	compiler := engine.core.(*startTestCore)
	baseMutation := compiler.mutateBundle
	compiler.mutateBundle = func(bundle *core.LifecycleBundle) {
		baseMutation(bundle)
		receiptGraphBundle(bundle)
	}
	exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "switch-rewritten-prepare", []string{"read-project"}))
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "switch-rewritten-started", host.ReceiptStarted, "", ""))
	completed := receiptTestCommand(t, prepared, "switch-rewritten-completed", host.ReceiptCompleted, "succeeded", "")
	completed.ExpectedRevision = started.Revision
	completed.Receipt.StableBoundary = "problem-framing-complete"
	ready := exchangeReceipt(t, engine, completed)

	compiler.mutateCompilation = func(request *core.CompilationRequest, bundle *core.LifecycleBundle) {
		request.Selection.Profile = "SP-FULL"
		bundle.Graph.Selection.Profile = request.Selection.Profile
		bundle.Graph.Selection.Digest = ""
		bundle.Graph.Selection.Digest = startTestDigest(bundle.Graph.Selection)
		request.Selection.GraphSelectionDigest = bundle.Graph.Selection.Digest
		bundle.Selection = *request.Selection
	}
	command := Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandSwitch, MessageID: "message-switch-rewritten", IdempotencyKey: "switch-rewritten",
		WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision,
		Switch: &SwitchInput{Boundary: "problem-framing-complete", Selection: start.Start.Selection, HostSession: start.Start.HostSession, Environment: start.Start.Environment},
	}
	command.Switch.Selection.Profile = "MATT-FULL"
	graphSelection := startTestGraphSelection()
	graphSelection.Profile = command.Switch.Selection.Profile
	graphSelection.Digest = ""
	command.Switch.Selection.GraphSelectionDigest = startTestDigest(graphSelection)

	if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_CORE_RESULT_INVALID" {
		t.Fatalf("rewritten SWITCH error = %v", err)
	}
	if compiler.compileCalls != 2 {
		t.Fatalf("Core compile calls = %d", compiler.compileCalls)
	}
	current, err := engine.journal.inspect(ready.WorkflowID)
	if err != nil {
		t.Fatalf("inspect rejected SWITCH: %v", err)
	}
	if current.Revision != ready.Revision || current.Snapshot.ActiveGeneration != ready.Snapshot.ActiveGeneration {
		t.Fatalf("rejected SWITCH changed Workflow state: %#v", current)
	}
}

func TestSwitchRejectsCompilerRewrittenTrustedClassification(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "switch-rewritten-classification")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	compiler := engine.core.(*startTestCore)
	baseMutation := compiler.mutateBundle
	compiler.mutateBundle = func(bundle *core.LifecycleBundle) {
		baseMutation(bundle)
		receiptGraphBundle(bundle)
	}
	exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "switch-classification-prepare", []string{"read-project"}))
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "switch-classification-started", host.ReceiptStarted, "", ""))
	completed := receiptTestCommand(t, prepared, "switch-classification-completed", host.ReceiptCompleted, "succeeded", "")
	completed.ExpectedRevision = started.Revision
	completed.Receipt.StableBoundary = "problem-framing-complete"
	ready := exchangeReceipt(t, engine, completed)

	compiler.mutateCompilation = func(request *core.CompilationRequest, bundle *core.LifecycleBundle) {
		request.Classification.EscalationReasons[0] = "compiler-rewritten"
		bundle.Classification = request.Classification
	}
	command := Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandSwitch, MessageID: "message-switch-classification", IdempotencyKey: "switch-classification",
		WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision,
		Switch: &SwitchInput{Boundary: "problem-framing-complete", Selection: start.Start.Selection, HostSession: start.Start.HostSession, Environment: start.Start.Environment},
	}
	if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_CORE_RESULT_INVALID" {
		t.Fatalf("rewritten SWITCH classification error = %v", err)
	}
	current, err := engine.journal.inspect(ready.WorkflowID)
	if err != nil {
		t.Fatalf("inspect rejected SWITCH classification: %v", err)
	}
	if current.Revision != ready.Revision || current.Snapshot.ActiveGeneration != ready.Snapshot.ActiveGeneration {
		t.Fatalf("rejected SWITCH classification changed Workflow state: %#v", current)
	}
}

func TestSwitchRejectsUncommittedBoundaryAndActiveInvocation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "switch-rejected")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	ready := exchangeTask6(t, engine, start)
	command := Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandSwitch, MessageID: "message-switch-invalid", IdempotencyKey: "switch-invalid",
		WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision,
		Switch: &SwitchInput{Boundary: "discovery", Selection: start.Start.Selection, HostSession: start.Start.HostSession, Environment: start.Start.Environment},
	}
	if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_STABLE_BOUNDARY_INVALID" {
		t.Fatalf("uncommitted boundary error = %v", err)
	}
	prepared := exchangeTask6(t, engine, task6Prepare(start, "switch-active-prepare", []string{"read-project"}))
	command.ExpectedRevision = prepared.Revision
	command.IdempotencyKey = "switch-active"
	if _, err := engine.Exchange(command); ErrorCode(err) != "WORKFLOW_SWITCH_INVALID" {
		t.Fatalf("active SWITCH error = %v", err)
	}
}
