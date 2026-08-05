package coordinator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/gofrs/flock"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

const (
	maximumHeadBytes          = 4 << 10
	maximumRevisionBytes      = 4 << 20
	maximumWorkflowRevisions  = 1 << 20
	workflowRecordsDirectory  = "records"
	legacyRuntimeRunDirectory = "runs"
)

type headRecord struct {
	SchemaVersion  string `json:"schema_version"`
	WorkflowID     string `json:"workflow_id"`
	Revision       uint64 `json:"revision"`
	RevisionDigest string `json:"revision_digest"`
	Digest         string `json:"digest"`
}

type revisionRecord struct {
	SchemaVersion     string   `json:"schema_version"`
	WorkflowID        string   `json:"workflow_id"`
	Revision          uint64   `json:"revision"`
	PredecessorDigest string   `json:"predecessor_digest"`
	MessageID         string   `json:"message_id"`
	IdempotencyKey    string   `json:"idempotency_key"`
	MessageDigest     string   `json:"message_digest"`
	Event             string   `json:"event"`
	Snapshot          Snapshot `json:"snapshot"`
	Result            Result   `json:"result"`
	Digest            string   `json:"digest"`
}

type journal struct {
	stateRoot string
}

func newJournal(stateRoot string) (*journal, error) {
	if stateRoot == "" || !filepath.IsAbs(stateRoot) || filepath.Clean(stateRoot) != stateRoot {
		return nil, coordinatorError("WORKFLOW_STATE_ROOT_INVALID", "state root must be a clean absolute path", nil)
	}
	if info, err := os.Lstat(stateRoot); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, coordinatorError("WORKFLOW_STATE_ROOT_INVALID", "state root must be a non-symlinked directory", nil)
		}
		if legacyRuntimeStatePresent(stateRoot) {
			return nil, coordinatorError("WORKFLOW_STATE_UNSUPPORTED", "legacy Runtime state is not readable by the Workflow Coordinator", nil)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, coordinatorError("WORKFLOW_STATE_ROOT_INVALID", "inspect state root", err)
	}
	if err := ensurePrivateDirectory(stateRoot); err != nil {
		return nil, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "create state root", err)
	}
	if err := ensurePrivateDirectory(filepath.Join(stateRoot, workflowRecordsDirectory)); err != nil {
		return nil, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "create Workflow records root", err)
	}
	return &journal{stateRoot: stateRoot}, nil
}

func legacyRuntimeStatePresent(stateRoot string) bool {
	_, err := os.Lstat(filepath.Join(stateRoot, legacyRuntimeRunDirectory))
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

func (value *journal) withWorkflowLock(workflowID string, action func() error) error {
	if value == nil || !validWorkflowID(workflowID) || action == nil {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid Workflow lock request", nil)
	}
	workflowRoot := value.workflowRoot(workflowID)
	if err := ensurePrivateDirectory(workflowRoot); err != nil {
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "create Workflow directory", err)
	}
	lockPath := filepath.Join(workflowRoot, "LOCK")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "create Workflow lock", err)
	}
	if err := lockFile.Chmod(0o600); err != nil {
		_ = lockFile.Close()
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "protect Workflow lock", err)
	}
	if err := lockFile.Close(); err != nil {
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "close Workflow lock", err)
	}
	guard := flock.New(lockPath)
	if err := guard.Lock(); err != nil {
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "lock Workflow", err)
	}
	defer func() {
		_ = guard.Unlock()
		_ = guard.Close()
	}()
	return action()
}

func (value *journal) inspect(workflowID string) (revisionRecord, error) {
	if value == nil || !validWorkflowID(workflowID) {
		return revisionRecord{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid Workflow ID", nil)
	}
	return value.loadCommitted(workflowID)
}

func (value *journal) replay(workflowID, idempotencyKey, messageDigest string) (Result, bool, error) {
	if !validWorkflowID(workflowID) || !validText(idempotencyKey, 512) || !validDigest(messageDigest) {
		return Result{}, false, coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid replay identity", nil)
	}
	current, err := value.loadCommitted(workflowID)
	if err != nil {
		return Result{}, false, err
	}
	for _, message := range current.Snapshot.ProcessedMessages {
		if message.IdempotencyKey != idempotencyKey {
			continue
		}
		if message.ContentDigest != messageDigest {
			return Result{}, false, coordinatorError("WORKFLOW_IDEMPOTENCY_KEY_REUSED", "idempotency key was already committed with different content", nil)
		}
		revision, err := value.loadRevision(workflowID, message.Revision)
		if err != nil {
			return Result{}, false, err
		}
		if revision.Result.Digest != message.ResultDigest {
			return Result{}, false, coordinatorError("WORKFLOW_STATE_DIGEST_MISMATCH", "processed message Result digest mismatch", nil)
		}
		result := revision.Result
		result.Replayed = true
		result.Digest = ""
		if result.Snapshot != nil {
			setResultProcessedMessagePin(&result, "")
		}
		result, err = normalizeResult(result)
		if err != nil {
			return Result{}, false, err
		}
		if result.Snapshot != nil && !setResultProcessedMessagePin(&result, result.Digest) {
			return Result{}, false, coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "replayed Result is missing the current processed message", nil)
		}
		return result, true, nil
	}
	return Result{}, false, nil
}

func (value *journal) loadCommitted(workflowID string) (revisionRecord, error) {
	headPath := filepath.Join(value.workflowRoot(workflowID), "HEAD")
	rawHead, err := readLimitedStateFile(headPath, maximumHeadBytes)
	if errors.Is(err, os.ErrNotExist) {
		return revisionRecord{}, coordinatorError("WORKFLOW_NOT_FOUND", workflowID, err)
	}
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_HEAD_INVALID", "read HEAD", err)
	}
	var head headRecord
	if err := decodeStrictState(rawHead, &head); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_HEAD_INVALID", "decode HEAD", err)
	}
	if err := validateHead(head, workflowID); err != nil {
		return revisionRecord{}, err
	}

	var current revisionRecord
	predecessorDigest := ""
	for revision := uint64(1); revision <= head.Revision; revision++ {
		loaded, err := value.loadRevision(workflowID, revision)
		if err != nil {
			return revisionRecord{}, err
		}
		if loaded.PredecessorDigest != predecessorDigest {
			return revisionRecord{}, coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid predecessor chain", nil)
		}
		if revision > 1 {
			if err := validateRevisionTransition(current, loaded); err != nil {
				return revisionRecord{}, err
			}
		}
		predecessorDigest = loaded.Digest
		current = loaded
	}
	if current.Digest != head.RevisionDigest {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_DIGEST_MISMATCH", "HEAD revision digest mismatch", nil)
	}
	return current, nil
}

func (value *journal) loadRevision(workflowID string, revision uint64) (revisionRecord, error) {
	path := filepath.Join(value.revisionsRoot(workflowID), revisionFileName(revision))
	raw, err := readLimitedStateFile(path, maximumRevisionBytes)
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "read immutable revision", err)
	}
	var record revisionRecord
	if err := decodeStrictState(raw, &record); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "decode immutable revision", err)
	}
	if err := restoreSnapshotDecisionDigests(&record.Snapshot); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "restore snapshot classification digest", err)
	}
	if record.Result.Snapshot != nil {
		if err := restoreSnapshotDecisionDigests(record.Result.Snapshot); err != nil {
			return revisionRecord{}, coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "restore Result classification digest", err)
		}
	}
	if err := validateRevision(record, workflowID, revision); err != nil {
		return revisionRecord{}, err
	}
	return record, nil
}

func (value *journal) commit(candidate revisionRecord) (revisionRecord, error) {
	record, err := cloneRevisionRecord(candidate)
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "clone revision candidate", err)
	}
	if err := validateRevisionCandidate(record); err != nil {
		return revisionRecord{}, err
	}
	current, loadErr := value.loadCommitted(record.WorkflowID)
	switch {
	case record.Revision == 1 && loadErr == nil:
		return revisionRecord{}, coordinatorError("WORKFLOW_REVISION_CONFLICT", "Workflow already has a committed revision", nil)
	case record.Revision == 1 && ErrorCode(loadErr) != "WORKFLOW_NOT_FOUND":
		return revisionRecord{}, loadErr
	case record.Revision > 1 && loadErr != nil:
		return revisionRecord{}, loadErr
	case record.Revision > 1 && (current.Revision+1 != record.Revision || current.Digest != record.PredecessorDigest):
		return revisionRecord{}, coordinatorError("WORKFLOW_REVISION_CONFLICT", "expected revision does not match committed Workflow state", nil)
	}

	if record.Result.Kind != ResultRejected {
		record.Result.Snapshot, err = snapshotPointer(record.Snapshot)
		if err != nil {
			return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "clone Result snapshot", err)
		}
	} else {
		record.Result.Snapshot = nil
	}
	record.Result.RevisionDigest = ""
	record.Result.Digest = ""
	record.Digest = ""
	record.Digest, _, err = canonicaljson.Digest(revisionDigestProjection(record))
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "digest Workflow revision", err)
	}
	record.Result.RevisionDigest = record.Digest
	record.Result, err = normalizeResult(record.Result)
	if err != nil {
		return revisionRecord{}, err
	}
	if err := setProcessedResultDigest(&record, record.Result.Digest); err != nil {
		return revisionRecord{}, err
	}
	if err := validateRevision(record, record.WorkflowID, record.Revision); err != nil {
		return revisionRecord{}, err
	}
	rawRevision, err := canonicaljson.Marshal(record)
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "encode Workflow revision", err)
	}
	committed, err := value.writeImmutableRevision(record, rawRevision)
	if err != nil {
		return revisionRecord{}, err
	}
	head := headRecord{SchemaVersion: WorkflowHeadSchemaV1, WorkflowID: committed.WorkflowID, Revision: committed.Revision, RevisionDigest: committed.Digest}
	head.Digest, _, err = canonicaljson.Digest(headDigestProjection(head))
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "digest Workflow HEAD", err)
	}
	rawHead, err := canonicaljson.Marshal(head)
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "encode Workflow HEAD", err)
	}
	if err := atomicWriteStateFile(filepath.Join(value.workflowRoot(record.WorkflowID), "HEAD"), rawHead, 0o600); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "replace Workflow HEAD", err)
	}
	return value.loadRevision(record.WorkflowID, record.Revision)
}

func validateRevisionCandidate(record revisionRecord) error {
	if record.SchemaVersion != WorkflowRevisionSchemaV1 || !validWorkflowID(record.WorkflowID) || record.Revision == 0 || record.Revision > maximumWorkflowRevisions ||
		!validText(record.MessageID, 512) || !validText(record.IdempotencyKey, 512) || !validDigest(record.MessageDigest) || !validText(record.Event, 512) ||
		record.Digest != "" || record.Result.Digest != "" || record.Result.RevisionDigest != "" {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid revision candidate identity", nil)
	}
	if record.Revision == 1 && record.PredecessorDigest != "" || record.Revision > 1 && !validDigest(record.PredecessorDigest) {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid revision candidate predecessor", nil)
	}
	if err := validateSnapshot(record.Snapshot, record.WorkflowID, record.Revision, false); err != nil {
		return err
	}
	if record.Result.SchemaVersion != WorkflowResultSchemaV1 || record.Result.WorkflowID != record.WorkflowID || record.Result.Revision != record.Revision || record.Result.Replayed || record.Result.Snapshot != nil {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid revision candidate Result", nil)
	}
	if record.Result.Kind != ResultState && record.Result.Kind != ResultDispatch && record.Result.Kind != ResultRejected {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid revision candidate Result kind", nil)
	}
	current, found := processedMessageForRevision(record.Snapshot.ProcessedMessages, record.Revision)
	if !found || current.IdempotencyKey != record.IdempotencyKey || current.ContentDigest != record.MessageDigest || current.ResultDigest != "" {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "current processed message does not match revision candidate", nil)
	}
	return nil
}

func validateRevision(record revisionRecord, workflowID string, revision uint64) error {
	if record.SchemaVersion != WorkflowRevisionSchemaV1 || record.WorkflowID != workflowID || record.Revision != revision ||
		!validText(record.MessageID, 512) || !validText(record.IdempotencyKey, 512) || !validDigest(record.MessageDigest) || !validText(record.Event, 512) || !validDigest(record.Digest) {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid revision identity", nil)
	}
	if revision == 1 && record.PredecessorDigest != "" || revision > 1 && !validDigest(record.PredecessorDigest) {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid revision predecessor", nil)
	}
	if err := validateSnapshot(record.Snapshot, workflowID, revision, true); err != nil {
		return err
	}
	if record.Result.WorkflowID != workflowID || record.Result.Revision != revision || record.Result.RevisionDigest != record.Digest || record.Result.Replayed {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "Result identity does not match revision", nil)
	}
	normalizedResult, err := normalizeResult(record.Result)
	if err != nil {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid persisted Result", err)
	}
	if !sameCanonicalValue(normalizedResult, record.Result) {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "persisted Result is not canonical", nil)
	}
	if record.Result.Kind == ResultRejected {
		if record.Result.Snapshot != nil || record.Result.Dispatch != nil {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "REJECTED Result contains persisted payload", nil)
		}
	} else if record.Result.Snapshot == nil || !sameCanonicalValue(*record.Result.Snapshot, record.Snapshot) {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "Result snapshot does not match revision snapshot", nil)
	}
	current, found := processedMessageForRevision(record.Snapshot.ProcessedMessages, revision)
	if !found || current.IdempotencyKey != record.IdempotencyKey || current.ContentDigest != record.MessageDigest || current.ResultDigest != record.Result.Digest {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "processed message does not match revision Result", nil)
	}
	digest, _, err := canonicaljson.Digest(revisionDigestProjection(record))
	if err != nil || digest != record.Digest {
		return coordinatorError("WORKFLOW_STATE_DIGEST_MISMATCH", "Workflow revision digest mismatch", err)
	}
	return nil
}

func validateSnapshot(snapshot Snapshot, workflowID string, revision uint64, persisted bool) error {
	if snapshot.SchemaVersion != WorkflowSnapshotSchemaV1 || snapshot.WorkflowID != workflowID || snapshot.Revision != revision ||
		!validText(snapshot.RequestID, 512) || !validText(snapshot.DeliverableID, 512) || snapshot.Classification.RequestMode != classification.RequestModeWorkflow ||
		snapshot.Classification.WorkflowComplexity == nil || *snapshot.Classification.WorkflowComplexity != classification.ComplexityComplex ||
		!validRiskClass(snapshot.Classification.RiskClass) || snapshot.Classification.EvidenceRequirements == nil || snapshot.Classification.EscalationReasons == nil ||
		snapshot.Classification.CapabilitySelector != nil || !validSnapshotStatus(snapshot.Status) || snapshot.Bundles == nil || snapshot.GrantHistory == nil ||
		snapshot.Receipts == nil || snapshot.ResourceLeases == nil || snapshot.ProcessedMessages == nil || snapshot.ProjectionLag == nil || uint64(len(snapshot.ProcessedMessages)) != revision {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid Workflow snapshot identity or collections", nil)
	}
	for index, message := range snapshot.ProcessedMessages {
		if !validText(message.IdempotencyKey, 512) || !validDigest(message.ContentDigest) || message.Revision == 0 || message.Revision > revision ||
			persisted && !validDigest(message.ResultDigest) || !persisted && message.ResultDigest != "" && !validDigest(message.ResultDigest) ||
			index > 0 && snapshot.ProcessedMessages[index-1].IdempotencyKey >= message.IdempotencyKey {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid processed message collection", nil)
		}
	}
	return nil
}

func validateRevisionTransition(previous, current revisionRecord) error {
	if current.WorkflowID != previous.WorkflowID || current.Revision != previous.Revision+1 || current.Snapshot.RequestID != previous.Snapshot.RequestID ||
		current.Snapshot.DeliverableID != previous.Snapshot.DeliverableID || len(current.Snapshot.ProcessedMessages) != len(previous.Snapshot.ProcessedMessages)+1 {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid Workflow revision transition", nil)
	}
	currentMessages := make(map[string]ProcessedMessage, len(current.Snapshot.ProcessedMessages))
	for _, message := range current.Snapshot.ProcessedMessages {
		currentMessages[message.IdempotencyKey] = message
	}
	for _, message := range previous.Snapshot.ProcessedMessages {
		if currentMessages[message.IdempotencyKey] != message {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "processed message history was rewritten", nil)
		}
	}
	return nil
}

func validateHead(record headRecord, workflowID string) error {
	if record.SchemaVersion != WorkflowHeadSchemaV1 || record.WorkflowID != workflowID || record.Revision == 0 || record.Revision > maximumWorkflowRevisions ||
		!validDigest(record.RevisionDigest) || !validDigest(record.Digest) {
		return coordinatorError("WORKFLOW_STATE_HEAD_INVALID", "invalid HEAD identity", nil)
	}
	digest, _, err := canonicaljson.Digest(headDigestProjection(record))
	if err != nil || digest != record.Digest {
		return coordinatorError("WORKFLOW_STATE_DIGEST_MISMATCH", "Workflow HEAD digest mismatch", err)
	}
	return nil
}

func (value *journal) writeImmutableRevision(record revisionRecord, raw []byte) (revisionRecord, error) {
	revisionsRoot := value.revisionsRoot(record.WorkflowID)
	if err := ensurePrivateDirectory(revisionsRoot); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "create revisions directory", err)
	}
	path := filepath.Join(revisionsRoot, revisionFileName(record.Revision))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return reuseMatchingOrphan(path, record, revisionsRoot)
		}
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "create immutable revision", err)
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "protect immutable revision", err)
	}
	if _, err := file.Write(raw); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "write immutable revision", err)
	}
	if err := file.Sync(); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "sync immutable revision", err)
	}
	if err := file.Close(); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "close immutable revision", err)
	}
	if err := syncStateDirectory(revisionsRoot); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "sync revisions directory", err)
	}
	remove = false
	return record, nil
}

func reuseMatchingOrphan(path string, expected revisionRecord, directory string) (revisionRecord, error) {
	raw, err := readLimitedStateFile(path, maximumRevisionBytes)
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "read orphan revision", err)
	}
	var existing revisionRecord
	if err := decodeStrictState(raw, &existing); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "decode orphan revision", err)
	}
	if err := validateRevision(existing, expected.WorkflowID, expected.Revision); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "validate orphan revision", err)
	}
	if !sameLogicalRevision(existing, expected) {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "conflicting orphan revision", nil)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "open matching orphan revision", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "protect matching orphan revision", err)
	}
	if err := file.Sync(); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "sync matching orphan revision", err)
	}
	if err := syncStateDirectory(directory); err != nil {
		return revisionRecord{}, coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "sync matching orphan directory", err)
	}
	return existing, nil
}

func sameLogicalRevision(left, right revisionRecord) bool {
	left.MessageID, right.MessageID = "", ""
	left.Digest, right.Digest = "", ""
	left.Result.RevisionDigest, right.Result.RevisionDigest = "", ""
	left.Result.Digest, right.Result.Digest = "", ""
	return sameCanonicalValue(revisionDigestProjection(left), revisionDigestProjection(right))
}

func revisionDigestProjection(value revisionRecord) revisionRecord {
	value.Digest = ""
	value.Result.RevisionDigest = ""
	value.Result.Digest = ""
	value.Snapshot.ProcessedMessages = clearResultPinForRevision(value.Snapshot.ProcessedMessages, value.Revision)
	if value.Result.Snapshot != nil {
		snapshot := *value.Result.Snapshot
		snapshot.ProcessedMessages = clearResultPinForRevision(snapshot.ProcessedMessages, value.Revision)
		value.Result.Snapshot = &snapshot
	}
	return value
}

func headDigestProjection(value headRecord) headRecord {
	value.Digest = ""
	return value
}

func setProcessedResultDigest(record *revisionRecord, digest string) error {
	updated := false
	for index := range record.Snapshot.ProcessedMessages {
		if record.Snapshot.ProcessedMessages[index].Revision == record.Revision {
			record.Snapshot.ProcessedMessages[index].ResultDigest = digest
			updated = true
		}
	}
	if !updated {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "current processed message is unavailable", nil)
	}
	if record.Result.Snapshot != nil {
		record.Result.Snapshot.ProcessedMessages = append([]ProcessedMessage{}, record.Snapshot.ProcessedMessages...)
	}
	return nil
}

func processedMessageForRevision(values []ProcessedMessage, revision uint64) (ProcessedMessage, bool) {
	for _, message := range values {
		if message.Revision == revision {
			return message, true
		}
	}
	return ProcessedMessage{}, false
}

func cloneRevisionRecord(value revisionRecord) (revisionRecord, error) {
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		return revisionRecord{}, err
	}
	var cloned revisionRecord
	if err := decodeStrictState(raw, &cloned); err != nil {
		return revisionRecord{}, err
	}
	restoreSnapshotPrivateFields(&cloned.Snapshot, value.Snapshot)
	if value.Result.Snapshot != nil && cloned.Result.Snapshot != nil {
		restoreSnapshotPrivateFields(cloned.Result.Snapshot, *value.Result.Snapshot)
	}
	return cloned, nil
}

func snapshotPointer(value Snapshot) (*Snapshot, error) {
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		return nil, err
	}
	var cloned Snapshot
	if err := decodeStrictState(raw, &cloned); err != nil {
		return nil, err
	}
	restoreSnapshotPrivateFields(&cloned, value)
	return &cloned, nil
}

func restoreSnapshotPrivateFields(destination *Snapshot, source Snapshot) {
	if destination == nil {
		return
	}
	destination.Classification = cloneClassificationDecision(source.Classification)
	for index := range destination.Bundles {
		if index < len(source.Bundles) {
			destination.Bundles[index].Classification = cloneClassificationDecision(source.Bundles[index].Classification)
		}
	}
}

func cloneClassificationDecision(value classification.ClassificationDecision) classification.ClassificationDecision {
	value.EvidenceRequirements = append([]classification.EvidenceRequirement{}, value.EvidenceRequirements...)
	value.EscalationReasons = append([]string{}, value.EscalationReasons...)
	if value.WorkflowComplexity != nil {
		complexity := *value.WorkflowComplexity
		value.WorkflowComplexity = &complexity
	}
	if value.CapabilitySelector != nil {
		selector := *value.CapabilitySelector
		value.CapabilitySelector = &selector
	}
	return value
}

func restoreSnapshotDecisionDigests(value *Snapshot) error {
	if value == nil {
		return nil
	}
	decision, err := classification.RecomputeDecisionDigest(value.Classification)
	if err != nil {
		return err
	}
	value.Classification = decision
	for index := range value.Bundles {
		decision, err := classification.RecomputeDecisionDigest(value.Bundles[index].Classification)
		if err != nil {
			return err
		}
		if decision.Digest() != value.Bundles[index].ClassificationDigest {
			return coordinatorError("WORKFLOW_STATE_DIGEST_MISMATCH", "Bundle classification digest mismatch", nil)
		}
		value.Bundles[index].Classification = decision
	}
	return nil
}

func sameCanonicalValue(left, right any) bool {
	leftRaw, leftErr := canonicaljson.Marshal(left)
	rightRaw, rightErr := canonicaljson.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}

func validWorkflowID(value string) bool {
	if !strings.HasPrefix(value, "workflow-") || !validText(value, 512) {
		return false
	}
	_, err := catalog.ParseLocalID(value)
	return err == nil
}

func validSnapshotStatus(value Status) bool {
	switch value {
	case StatusReady, StatusPrepared, StatusInFlight, StatusPaused, StatusFinished, StatusCancelled:
		return true
	default:
		return false
	}
}

func validRiskClass(value classification.RiskClass) bool {
	switch value {
	case classification.RiskNormal, classification.RiskElevated, classification.RiskCritical:
		return true
	default:
		return false
	}
}

func (value *journal) workflowRoot(workflowID string) string {
	return filepath.Join(value.stateRoot, workflowRecordsDirectory, workflowID)
}

func (value *journal) revisionsRoot(workflowID string) string {
	return filepath.Join(value.workflowRoot(workflowID), "revisions")
}

func revisionFileName(revision uint64) string {
	return fmt.Sprintf("%020d.json", revision)
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("path is not a private directory")
	}
	return os.Chmod(path, 0o700)
}

func atomicWriteStateFile(path string, content []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".HEAD-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	remove := true
	defer func() {
		_ = temporary.Close()
		if remove {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := replaceStateFile(temporaryPath, path); err != nil {
		return err
	}
	remove = false
	return syncStateDirectory(directory)
}

func syncStateDirectory(path string) error {
	if goruntime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func readLimitedStateFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, fmt.Errorf("state file is not regular or exceeds %d bytes", maximum)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("state file exceeds %d bytes", maximum)
	}
	return raw, nil
}

func decodeStrictState(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
