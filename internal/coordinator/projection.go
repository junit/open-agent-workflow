package coordinator

import (
	"errors"
	"path/filepath"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	workflowProjectionSchemaV1 = "oaw.workflow-projection/v1"
	projectionFailureReason    = "PROJECTION_WRITE_FAILED"
)

func (engine *Engine) projectResult(result Result) {
	if engine == nil || engine.projection == nil || result.Snapshot == nil {
		return
	}
	record, err := newProjectionRecord(result)
	if err == nil {
		err = writeProjectionSafely(engine.projection, record)
	}
	if err != nil {
		_ = engine.recordProjectionLag(result)
	}
}

func newProjectionRecord(result Result) (ProjectionRecord, error) {
	if result.Snapshot == nil || result.Revision == 0 || !validDigest(result.RevisionDigest) {
		return ProjectionRecord{}, coordinatorError("PROJECTION_INVALID", "committed Workflow Result is required", nil)
	}
	bundle, err := activeBundle(*result.Snapshot)
	if err != nil {
		return ProjectionRecord{}, coordinatorError("PROJECTION_INVALID", "active Bundle is unavailable", err)
	}
	evidence := projectionEvidence(result.Snapshot.Receipts)
	record := ProjectionRecord{
		SchemaVersion: workflowProjectionSchemaV1, WorkflowID: result.WorkflowID, Revision: result.Revision,
		BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, NodeID: result.Snapshot.ActiveNodeID,
		Ticket: result.Snapshot.ActiveTicket, Topology: bundle.Topology, Evidence: evidence,
	}
	record.Digest, _, err = canonicaljson.Digest(record)
	if err != nil {
		return ProjectionRecord{}, coordinatorError("PROJECTION_INVALID", "digest Workflow projection", err)
	}
	return record, nil
}

func projectionEvidence(receipts []host.InvocationReceipt) []host.EvidenceReference {
	seen := make(map[string]host.EvidenceReference)
	for _, receipt := range receipts {
		for _, reference := range receipt.Evidence {
			key := reference.Kind + "\x00" + reference.Reference + "\x00" + reference.Digest
			seen[key] = reference
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]host.EvidenceReference, 0, len(keys))
	for _, key := range keys {
		result = append(result, seen[key])
	}
	return result
}

func writeProjectionSafely(sink ProjectionSink, record ProjectionRecord) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("Projection sink panicked")
		}
	}()
	return sink.WriteProjection(record)
}

func (engine *Engine) recordProjectionLag(result Result) error {
	lag := ProjectionLag{Revision: result.Revision, Digest: result.RevisionDigest, Reason: projectionFailureReason}
	raw, err := canonicaljson.Marshal(lag)
	if err != nil {
		return err
	}
	root := filepath.Join(engine.journal.stateRoot, "projection-lag", result.WorkflowID)
	if err := ensurePrivateDirectory(root); err != nil {
		return err
	}
	return atomicWriteStateFile(filepath.Join(root, revisionFileName(result.Revision)), raw, 0o600)
}
