package coordinator

import (
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestCancelPausesUncertainInvocationThenReleasesOnTerminalConfirmation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	start := startTestCommand(t, "cancel-inflight")
	engine := newTask6Engine(t, stateRoot, projectRoot, start, nil)
	exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "cancel-prepare", []string{"write-project"}))
	inFlight := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "cancel-started", host.ReceiptStarted, "", ""))

	pendingCommand := Command{
		SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandCancel, MessageID: "message-cancel-pending", IdempotencyKey: "cancel-pending",
		WorkflowID: inFlight.WorkflowID, ExpectedRevision: inFlight.Revision,
		Cancel: &CancelInput{Reason: "user requested", InvocationTerminal: false},
	}
	pending := exchangeTask6(t, engine, pendingCommand)
	if pending.Snapshot.Status != StatusPaused || pending.Snapshot.ActiveGrant == nil || pending.Snapshot.ResourceLeases[0].ReleasedRevision != 0 ||
		len(pending.Diagnostics) != 1 || pending.Diagnostics[0].Code != "WORKFLOW_CANCELLATION_PENDING" {
		t.Fatalf("pending CANCEL = %#v", pending)
	}
	restarted := newTask6Engine(t, stateRoot, projectRoot, start, nil)
	recovered := exchangeTask6(t, restarted, Command{SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandInspect, WorkflowID: pending.WorkflowID})
	if recovered.RevisionDigest != pending.RevisionDigest || recovered.Snapshot.Status != StatusPaused || recovered.Snapshot.ActiveGrant == nil {
		t.Fatalf("recovered uncertain CANCEL = %#v", recovered)
	}
	confirmedCommand := Command{
		SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandCancel, MessageID: "message-cancel-confirmed", IdempotencyKey: "cancel-confirmed",
		WorkflowID: pending.WorkflowID, ExpectedRevision: pending.Revision,
		Cancel: &CancelInput{Reason: "Host confirmed terminal", InvocationTerminal: true},
	}
	confirmed := exchangeTask6(t, restarted, confirmedCommand)
	if confirmed.Snapshot.Status != StatusCancelled || confirmed.Snapshot.ActiveGrant != nil || confirmed.Snapshot.ResourceLeases[0].ReleasedRevision != confirmed.Revision {
		t.Fatalf("confirmed CANCEL = %#v", confirmed)
	}
}

func TestCancelReadyWorkflowIsTerminalAndRestartSafe(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "cancel-ready")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	ready := exchangeTask6(t, engine, start)
	command := Command{
		SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandCancel, MessageID: "message-cancel-ready", IdempotencyKey: "cancel-ready-command",
		WorkflowID: ready.WorkflowID, ExpectedRevision: ready.Revision, Cancel: &CancelInput{Reason: "no longer needed"},
	}
	cancelled := exchangeTask6(t, engine, command)
	if cancelled.Snapshot.Status != StatusCancelled {
		t.Fatalf("ready CANCEL = %#v", cancelled)
	}
	restarted := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	inspected := exchangeTask6(t, restarted, Command{SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandInspect, WorkflowID: cancelled.WorkflowID})
	if inspected.RevisionDigest != cancelled.RevisionDigest || inspected.Snapshot.Status != StatusCancelled {
		t.Fatalf("restarted CANCEL = %#v", inspected)
	}
}
