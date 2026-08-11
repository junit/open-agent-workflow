package coordinator

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

func (engine *Engine) cancel(command Command) (Result, error) {
	if command.Cancel == nil {
		return Result{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "CANCEL input is required", nil)
	}
	messageDigest, err := cancelMessageDigest(*command.Cancel)
	if err != nil {
		return Result{}, err
	}
	var result Result
	err = engine.journal.withWorkflowLock(command.WorkflowID, func() error {
		replayed, found, replayErr := engine.journal.replay(command.WorkflowID, command.IdempotencyKey, messageDigest)
		if replayErr == nil && found {
			result = replayed
			return nil
		}
		if replayErr != nil && ErrorCode(replayErr) != "WORKFLOW_NOT_FOUND" {
			return replayErr
		}
		current, inspectErr := engine.journal.inspect(command.WorkflowID)
		if inspectErr != nil {
			return inspectErr
		}
		if current.Revision != command.ExpectedRevision {
			return coordinatorError("WORKFLOW_REVISION_CONFLICT", "CANCEL expected revision does not match committed Workflow state", nil)
		}
		if current.Snapshot.Status == StatusFinished || current.Snapshot.Status == StatusCancelled {
			return coordinatorError("WORKFLOW_CANCEL_INVALID", "terminal Workflow cannot be cancelled", nil)
		}
		nextRevision := current.Revision + 1
		snapshot := cloneSnapshot(current.Snapshot)
		snapshot.Revision = nextRevision
		diagnostics := []Diagnostic{}
		event := "WORKFLOW_CANCELLED"
		invocationUncertain := snapshot.ActiveGrant != nil &&
			(snapshot.Status == StatusInFlight || snapshot.Status == StatusPaused) && !command.Cancel.InvocationTerminal
		if invocationUncertain {
			snapshot.Status = StatusPaused
			diagnostics = []Diagnostic{{Code: "WORKFLOW_CANCELLATION_PENDING", Detail: "Host invocation termination is not confirmed"}}
			event = "WORKFLOW_CANCELLATION_PENDING"
		} else {
			snapshot.Status = StatusCancelled
			snapshot.ActiveGrant = nil
			if releaseErr := releaseResourceLeases(&snapshot, nextRevision); releaseErr != nil {
				return releaseErr
			}
		}
		snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{
			IdempotencyKey: command.IdempotencyKey, ContentDigest: messageDigest, Revision: nextRevision,
		})
		sort.Slice(snapshot.ProcessedMessages, func(left, right int) bool {
			return snapshot.ProcessedMessages[left].IdempotencyKey < snapshot.ProcessedMessages[right].IdempotencyKey
		})
		candidate := revisionRecord{
			SchemaVersion: WorkflowRevisionSchemaV2, WorkflowID: command.WorkflowID, Revision: nextRevision,
			PredecessorDigest: current.Digest, MessageID: command.MessageID, IdempotencyKey: command.IdempotencyKey,
			MessageDigest: messageDigest, Event: event, Snapshot: snapshot,
			Result: Result{SchemaVersion: WorkflowResultSchemaV2, Kind: ResultState, WorkflowID: command.WorkflowID, Revision: nextRevision, Diagnostics: diagnostics},
		}
		committed, commitErr := engine.journal.commit(candidate)
		if commitErr != nil {
			return commitErr
		}
		result = committed.Result
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func cancelMessageDigest(input CancelInput) (string, error) {
	record := struct {
		SchemaVersion string      `json:"schema_version"`
		Kind          CommandKind `json:"kind"`
		Cancel        CancelInput `json:"cancel"`
	}{WorkflowCommandSchemaV2, CommandCancel, input}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return "", coordinatorError("WORKFLOW_COMMAND_INVALID", "digest CANCEL input", err)
	}
	return digest, nil
}
