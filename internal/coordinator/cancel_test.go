package coordinator

import (
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestCancelWithActiveGrantAlwaysPausesUntilDispatchBoundReceipt(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	start := startTestCommand(t, "cancel-inflight")
	engine := newTask6Engine(t, stateRoot, projectRoot, start, nil)
	exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "cancel-prepare", []string{"write-project"}))
	inFlight := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "cancel-started", host.ReceiptStarted, "", ""))

	pendingCommand := Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandCancel, MessageID: "message-cancel-pending", IdempotencyKey: "cancel-pending",
		WorkflowID: inFlight.WorkflowID, ExpectedRevision: inFlight.Revision,
		Cancel: &CancelInput{Reason: "user requested", InvocationTerminal: false},
	}
	pending := exchangeTask6(t, engine, pendingCommand)
	if pending.Snapshot.Status != StatusPaused || pending.Snapshot.ActiveGrant == nil || pending.Snapshot.ResourceLeases[0].ReleasedRevision != 0 ||
		len(pending.Diagnostics) != 1 || pending.Diagnostics[0].Code != "WORKFLOW_CANCELLATION_PENDING" {
		t.Fatalf("pending CANCEL = %#v", pending)
	}
	restarted := newTask6Engine(t, stateRoot, projectRoot, start, nil)
	recovered := exchangeTask6(t, restarted, Command{SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandInspect, WorkflowID: pending.WorkflowID})
	if recovered.RevisionDigest != pending.RevisionDigest || recovered.Snapshot.Status != StatusPaused || recovered.Snapshot.ActiveGrant == nil {
		t.Fatalf("recovered uncertain CANCEL = %#v", recovered)
	}
	callerClaimedTerminal := exchangeTask6(t, restarted, Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandCancel, MessageID: "message-cancel-confirmed", IdempotencyKey: "cancel-confirmed",
		WorkflowID: pending.WorkflowID, ExpectedRevision: pending.Revision,
		Cancel: &CancelInput{Reason: "caller claims Host termination", InvocationTerminal: true},
	})
	if callerClaimedTerminal.Snapshot.Status != StatusPaused || callerClaimedTerminal.Snapshot.ActiveGrant == nil ||
		callerClaimedTerminal.Snapshot.ResourceLeases[0].ReleasedRevision != 0 {
		t.Fatalf("caller terminal claim released authority = %#v", callerClaimedTerminal)
	}
	cancelled := receiptTestCommand(t, prepared, "cancel-receipt", host.ReceiptCancelled, "", "")
	cancelled.ExpectedRevision = callerClaimedTerminal.Revision
	confirmed := exchangeReceipt(t, restarted, cancelled)
	if confirmed.Snapshot.Status != StatusCancelled || confirmed.Snapshot.ActiveGrant != nil || confirmed.Snapshot.ResourceLeases[0].ReleasedRevision != confirmed.Revision {
		t.Fatalf("Dispatch-bound CANCELLED receipt = %#v", confirmed)
	}
}

func TestCancelPreparedInvocationRetainsDispatchForTerminalReceipt(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "cancel-prepared")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "cancel-prepared-dispatch", []string{"write-project"}))
	pending := exchangeTask6(t, engine, Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandCancel, MessageID: "message-cancel-prepared", IdempotencyKey: "cancel-prepared-command",
		WorkflowID: prepared.WorkflowID, ExpectedRevision: prepared.Revision,
		Cancel: &CancelInput{Reason: "cancel before STARTED", InvocationTerminal: true},
	})
	if pending.Snapshot.Status != StatusPaused || pending.Snapshot.ActiveGrant == nil || pending.Snapshot.ActiveDispatchDigest == "" {
		t.Fatalf("prepared CANCEL did not preserve active dispatch = %#v", pending)
	}
	cancelled := receiptTestCommand(t, prepared, "cancel-prepared-receipt", host.ReceiptCancelled, "", "")
	cancelled.ExpectedRevision = pending.Revision
	confirmed := exchangeReceipt(t, engine, cancelled)
	if confirmed.Snapshot.Status != StatusCancelled || confirmed.Snapshot.ActiveGrant != nil || confirmed.Snapshot.ActiveDispatchDigest != "" {
		t.Fatalf("prepared cancellation receipt = %#v", confirmed)
	}
}

func TestCancelReadyWorkflowIsTerminalAndRestartSafe(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "cancel-ready")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	ready := exchangeTask6(t, engine, start)
	command := Command{
		SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandCancel, MessageID: "message-cancel-ready", IdempotencyKey: "cancel-ready-command",
		WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision, Cancel: &CancelInput{Reason: "no longer needed"},
	}
	cancelled := exchangeTask6(t, engine, command)
	if cancelled.Snapshot.Status != StatusCancelled {
		t.Fatalf("ready CANCEL = %#v", cancelled)
	}
	restarted := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	inspected := exchangeTask6(t, restarted, Command{SchemaVersion: WorkflowCommandSchemaV2, Kind: CommandInspect, WorkflowID: cancelled.WorkflowID})
	if inspected.RevisionDigest != cancelled.RevisionDigest || inspected.Snapshot.Status != StatusCancelled {
		t.Fatalf("restarted CANCEL = %#v", inspected)
	}
}
