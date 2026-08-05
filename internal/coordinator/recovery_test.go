package coordinator

import (
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestRecoveryRestoresPausedInvocationAndActiveLease(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	start := startTestCommand(t, "recovery-paused")
	engine := newTask6Engine(t, stateRoot, projectRoot, start, nil)
	exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "recovery-prepare", []string{"write-project"}))
	started := exchangeReceipt(t, engine, receiptTestCommand(t, prepared, "recovery-started", host.ReceiptStarted, "", ""))
	pausedCommand := receiptTestCommand(t, prepared, "recovery-paused-receipt", host.ReceiptPaused, "", "")
	pausedCommand.ExpectedRevision = started.Revision
	paused := exchangeReceipt(t, engine, pausedCommand)

	restarted := newTask6Engine(t, stateRoot, projectRoot, start, nil)
	inspected := exchangeTask6(t, restarted, Command{SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandInspect, WorkflowID: paused.WorkflowID})
	if inspected.RevisionDigest != paused.RevisionDigest || inspected.Snapshot.Status != StatusPaused || inspected.Snapshot.ActiveGrant == nil ||
		len(inspected.Snapshot.Receipts) != 2 || len(inspected.Snapshot.ResourceLeases) != 1 || inspected.Snapshot.ResourceLeases[0].ReleasedRevision != 0 {
		t.Fatalf("recovered Workflow = %#v", inspected)
	}
}
