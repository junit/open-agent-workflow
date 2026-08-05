package coordinator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

func TestJournalRejectsCorruptHeadAndPredecessorChain(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	j, err := newJournal(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	first := commitRevision(t, j, testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1"))
	second := commitRevision(t, j, nextTestRevision(t, first, "message-2", "message-content-2", "key-2"))
	runRoot := j.workflowRoot(testWorkflowID)

	t.Run("invalid HEAD", func(t *testing.T) {
		original, err := os.ReadFile(filepath.Join(runRoot, "HEAD"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runRoot, "HEAD"), []byte(`{"broken":`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := j.inspect(testWorkflowID); ErrorCode(err) != "WORKFLOW_STATE_HEAD_INVALID" {
			t.Fatalf("inspect(corrupt HEAD) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(runRoot, "HEAD"), original, 0o600); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("predecessor mismatch", func(t *testing.T) {
		path := filepath.Join(j.revisionsRoot(testWorkflowID), revisionFileName(2))
		record := second
		record.PredecessorDigest = strings.Repeat("0", 64)
		sealTestRevision(&record)
		writeCanonicalTestFile(t, path, record)
		head := headRecord{SchemaVersion: WorkflowHeadSchemaV1, WorkflowID: testWorkflowID, Revision: 2, RevisionDigest: record.Digest}
		sealTestHead(&head)
		writeCanonicalTestFile(t, filepath.Join(runRoot, "HEAD"), head)
		if _, err := j.inspect(testWorkflowID); ErrorCode(err) != "WORKFLOW_STATE_REVISION_INVALID" {
			t.Fatalf("inspect(predecessor mismatch) error = %v", err)
		}
	})
}

func sealTestRevision(record *revisionRecord) {
	record.Digest = ""
	record.Result.RevisionDigest = ""
	record.Result.Digest = ""
	record.Digest, _, _ = canonicaljson.Digest(revisionDigestProjection(*record))
	record.Result.RevisionDigest = record.Digest
	record.Result.Digest, _, _ = canonicaljson.Digest(resultDigestProjection(record.Result))
	_ = setProcessedResultDigest(record, record.Result.Digest)
}

func sealTestHead(record *headRecord) {
	record.Digest = ""
	record.Digest, _, _ = canonicaljson.Digest(*record)
}

func writeCanonicalTestFile(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
