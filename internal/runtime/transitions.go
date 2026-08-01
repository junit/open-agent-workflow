package runtime

import (
	"reflect"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

func validateRevisionTransition(previous, current revisionRecord) error {
	previousSnapshot := previous.Snapshot
	currentSnapshot := current.Snapshot
	if previous.RunID != current.RunID || current.Revision != previous.Revision+1 || previousSnapshot.SchemaVersion != currentSnapshot.SchemaVersion || previousSnapshot.RunID != currentSnapshot.RunID || previousSnapshot.RequestID != currentSnapshot.RequestID || previousSnapshot.Project != currentSnapshot.Project || previousSnapshot.RequestMode != currentSnapshot.RequestMode || previousSnapshot.ClassificationDigest != currentSnapshot.ClassificationDigest || previousSnapshot.ConfigurationDigest != currentSnapshot.ConfigurationDigest || !reflect.DeepEqual(previousSnapshot.Classification, currentSnapshot.Classification) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "immutable Run identity changed across revisions", nil)
	}
	if !processedMessagesExtend(previousSnapshot.ProcessedMessages, currentSnapshot.ProcessedMessages) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "processed message history changed across revisions", nil)
	}
	switch currentSnapshot.RequestMode {
	case classification.RequestModeDirect:
		if previousSnapshot.Status != RunReleased || currentSnapshot.Status != RunReleased || previousSnapshot.Bounded != nil || currentSnapshot.Bounded != nil || previousSnapshot.Grants != nil || currentSnapshot.Grants != nil || previousSnapshot.Observations != nil || currentSnapshot.Observations != nil {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Direct revision transition", nil)
		}
		return nil
	case classification.RequestModeBounded:
		return validateBoundedRevisionTransition(previousSnapshot, currentSnapshot)
	case classification.RequestModeWorkflow:
		return validateWorkflowRevisionTransition(previousSnapshot, currentSnapshot)
	default:
		return runtimeError("RUN_STATE_REVISION_INVALID", "unsupported revision transition mode", nil)
	}
}

func processedMessagesExtend(previous, current []ProcessedMessage) bool {
	if len(current) != len(previous)+1 {
		return false
	}
	currentByKey := make(map[string]ProcessedMessage, len(current))
	for _, message := range current {
		currentByKey[message.IdempotencyKey] = message
	}
	for _, message := range previous {
		if currentByKey[message.IdempotencyKey] != message {
			return false
		}
	}
	return true
}

func validateBoundedRevisionTransition(previous, current RunSnapshot) error {
	if previous.Bounded == nil || current.Bounded == nil || !equalBoundedTransitionState(previous, current) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Bounded request state changed across revisions", nil)
	}
	switch previous.Status {
	case RunAwaitingCapability:
		if current.Status != RunReady || len(previous.Grants) != 0 || len(current.Grants) != 0 || len(previous.Observations) != 0 || len(current.Observations) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded Capability selection transition", nil)
		}
	case RunReady:
		if current.Status != RunGranted || len(previous.Grants) != 0 || len(current.Grants) != 1 || len(previous.Observations) != 0 || len(current.Observations) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded Grant transition", nil)
		}
	case RunGranted:
		if current.Status != RunInFlight || !reflect.DeepEqual(previous.Grants, current.Grants) || !reflect.DeepEqual(previous.GrantIDs, current.GrantIDs) || len(previous.Observations) != 0 || len(current.Observations) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded dispatch transition", nil)
		}
	case RunInFlight:
		if current.Status != RunFinished && current.Status != RunPaused || !reflect.DeepEqual(previous.Grants, current.Grants) || !reflect.DeepEqual(previous.GrantIDs, current.GrantIDs) || len(previous.Observations) != 0 || len(current.Observations) > 1 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded completion transition", nil)
		}
	default:
		return runtimeError("RUN_STATE_REVISION_INVALID", "Bounded terminal state cannot transition", nil)
	}
	return nil
}

func equalBoundedTransitionState(previous, current RunSnapshot) bool {
	previousState := *previous.Bounded
	currentState := *current.Bounded
	if previous.Status == RunAwaitingCapability && current.Status == RunReady {
		previousState.Selector = nil
		currentState.Selector = nil
		previousState.Input.TrustedRuleID = ""
		currentState.Input.TrustedRuleID = ""
	}
	return reflect.DeepEqual(previousState, currentState)
}
