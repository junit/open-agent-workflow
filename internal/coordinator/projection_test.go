package coordinator

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestProjectionEmitsOneWayRedactedCommittedRecords(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "projection-workflow")
	sink := &task6ProjectionSink{}
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, sink)
	started := exchangeTask6(t, engine, start)
	prepared := exchangeTask6(t, engine, task6Prepare(start, "projection-prepare", []string{"read-project"}))
	receipt := receiptTestCommand(t, prepared, "projection-started", host.ReceiptStarted, "", "")
	receipt.ExpectedRevision = prepared.Revision
	inFlight := exchangeReceipt(t, engine, receipt)

	records := sink.recordsCopy()
	if len(records) != 3 || records[0].Revision != started.Revision || records[1].Revision != prepared.Revision || records[2].Revision != inFlight.Revision {
		t.Fatalf("projection sequence = %#v", records)
	}
	for _, record := range records {
		if record.SchemaVersion != "oaw.workflow-projection/v1" || record.WorkflowID != started.WorkflowID || record.BundleDigest == "" || record.Digest == "" {
			t.Fatalf("projection record = %#v", record)
		}
		raw, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"request_id", "deliverable_id", "host_session_digest", "provider_inventory_digest", "configuration"} {
			if strings.Contains(string(raw), forbidden) {
				t.Fatalf("projection leaked %q: %s", forbidden, raw)
			}
		}
	}
}

func TestProjectionFailureRecordsLagWithoutChangingCommittedState(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "projection-failure")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, task6FailingProjection{})
	result := exchangeTask6(t, engine, start)

	restarted := newTask6Engine(t, stateRoot, t.TempDir(), start, nil)
	inspected := exchangeTask6(t, restarted, Command{SchemaVersion: WorkflowCommandSchemaV1, Kind: CommandInspect, WorkflowID: result.WorkflowID})
	if inspected.RevisionDigest != result.RevisionDigest || inspected.Digest != result.Digest || len(inspected.Snapshot.ProjectionLag) != 0 {
		t.Fatalf("projection failure changed committed state: %#v", inspected)
	}
	lagPath := filepath.Join(stateRoot, "projection-lag", result.WorkflowID, revisionFileName(result.Revision))
	raw, err := os.ReadFile(lagPath)
	if err != nil || !strings.Contains(string(raw), "PROJECTION_WRITE_FAILED") || !strings.Contains(string(raw), result.RevisionDigest) {
		t.Fatalf("projection lag marker = %s, %v", raw, err)
	}
}

func TestProjectionPanicRecordsLagWithoutFailingCommand(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	start := startTestCommand(t, "projection-panic")
	engine := newTask6Engine(t, stateRoot, t.TempDir(), start, task6PanickingProjection{})
	result := exchangeTask6(t, engine, start)
	lagPath := filepath.Join(stateRoot, "projection-lag", result.WorkflowID, revisionFileName(result.Revision))
	if _, err := os.Stat(lagPath); err != nil {
		t.Fatalf("projection panic lag marker: %v", err)
	}
}

type task6ProjectionSink struct {
	mu      sync.Mutex
	records []ProjectionRecord
}

func (value *task6ProjectionSink) WriteProjection(record ProjectionRecord) error {
	value.mu.Lock()
	defer value.mu.Unlock()
	value.records = append(value.records, record)
	return nil
}

func (value *task6ProjectionSink) recordsCopy() []ProjectionRecord {
	value.mu.Lock()
	defer value.mu.Unlock()
	return append([]ProjectionRecord{}, value.records...)
}

type task6FailingProjection struct{}

func (task6FailingProjection) WriteProjection(ProjectionRecord) error {
	return errors.New("projection unavailable")
}

type task6PanickingProjection struct{}

func (task6PanickingProjection) WriteProjection(ProjectionRecord) error {
	panic("projection failure")
}
