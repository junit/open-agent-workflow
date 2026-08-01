package runtime

import (
	"path/filepath"
	"reflect"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

func validateWorkflowState(record revisionRecord) error {
	snapshot := record.Snapshot
	if snapshot.RequestMode != classification.RequestModeWorkflow || snapshot.Classification.RequestMode != classification.RequestModeWorkflow || !validDigest(snapshot.ClassificationDigest) || snapshot.Workflow == nil || snapshot.Bounded != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow classification state", nil)
	}
	if snapshot.Status != RunAwaitingSelection && snapshot.Status != RunReady && snapshot.Status != RunGranted {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow status", nil)
	}
	if validateIdentifier(snapshot.RequestID) != nil || snapshot.Project.Root == "" || !filepath.IsAbs(snapshot.Project.Root) || filepath.Clean(snapshot.Project.Root) != snapshot.Project.Root || !validDigest(snapshot.ConfigurationDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Workflow identity", nil)
	}
	workflow := snapshot.Workflow
	if snapshot.ConfigurationDigest != workflow.ConfigurationDigest || !validDigest(workflow.RegistryDigest) || validateIdentifier(workflow.Input.DeliverableID) != nil || !validDigest(workflow.Input.InputDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Workflow trusted inputs", nil)
	}
	if snapshot.ProcessedMessages == nil || uint64(len(snapshot.ProcessedMessages)) != record.Revision || snapshot.Observations != nil || snapshot.GrantIDs == nil || snapshot.ResourceLeaseIDs == nil || len(snapshot.ResourceLeaseIDs) != 0 || workflow.RevokedGrantIDs == nil || workflow.ResourceLeases == nil || workflow.ProjectionLag == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow authority collections", nil)
	}
	if snapshot.Classification.WorkflowComplexity == nil || snapshot.Classification.CapabilitySelector != nil || snapshot.Classification.EvidenceRequirements == nil || snapshot.Classification.EscalationReasons == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow classification details", nil)
	}
	for index, message := range snapshot.ProcessedMessages {
		if validateIdentifier(message.IdempotencyKey) != nil || !validDigest(message.ContentDigest) || message.Revision == 0 || message.Revision > record.Revision || index > 0 && snapshot.ProcessedMessages[index-1].IdempotencyKey >= message.IdempotencyKey {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow processed message", nil)
		}
	}
	currentMessage := ProcessedMessage{}
	for _, message := range snapshot.ProcessedMessages {
		if message.Revision == record.Revision {
			currentMessage = message
			break
		}
	}
	if currentMessage.IdempotencyKey != record.IdempotencyKey || currentMessage.ContentDigest != record.MessageDigest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "current Workflow message does not match revision", nil)
	}
	if snapshot.Status == RunAwaitingSelection {
		if record.Revision != 1 || record.Event != "WORKFLOW_SELECTION_REQUIRED" || len(workflow.Bundles) != 0 || workflow.ActiveGeneration != 0 || workflow.ActiveNodeID != "" || workflow.ActiveGrantID != "" || snapshot.Grants != nil || len(snapshot.GrantIDs) != 0 || snapshot.LifecycleBundles == nil || len(snapshot.LifecycleBundles) != 0 || record.Reply.Kind != ReplySelectionRequired || len(record.Reply.Diagnostics) != 1 || record.Reply.Diagnostics[0].Code != "SELECTION_REQUIRED" || record.Reply.Reason != "" || len(record.Reply.RecoveryActions) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow selection state or reply", nil)
		}
		return nil
	}
	if len(workflow.Bundles) != 1 || workflow.ActiveGeneration != 1 || len(snapshot.LifecycleBundles) != 1 || snapshot.LifecycleBundles[0] != workflow.Bundles[0].ID {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Bundle collection", nil)
	}
	bundle := workflow.Bundles[0]
	if err := validateLifecycleBundle(bundle); err != nil || bundle.RunID != snapshot.RunID || bundle.DeliverableID != workflow.Input.DeliverableID || bundle.InputDigest != workflow.Input.InputDigest || bundle.Generation != workflow.ActiveGeneration || bundle.CreatedRevision > record.Revision || bundle.Configuration.Digest != workflow.ConfigurationDigest || bundle.RegistryDigest != workflow.RegistryDigest || workflow.ActiveNodeID != bundle.Graph.Entry {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid active Lifecycle Bundle", err)
	}
	if snapshot.Status == RunReady {
		if record.Event != "WORKFLOW_BUNDLE_CREATED" || workflow.ActiveGrantID != "" || snapshot.Grants != nil || len(snapshot.GrantIDs) != 0 || record.Reply.Kind != ReplyModeDecided || len(record.Reply.Diagnostics) != 0 || record.Reply.Reason != "" || len(record.Reply.RecoveryActions) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow ready state or reply", nil)
		}
		return nil
	}
	if record.Event != "WORKFLOW_STAGE_GRANT_ISSUED" || len(snapshot.Grants) != 1 || len(snapshot.GrantIDs) != 1 || workflow.ActiveGrantID == "" || snapshot.GrantIDs[0] != workflow.ActiveGrantID || snapshot.Grants[0].ID != workflow.ActiveGrantID || record.Reply.Kind != ReplyGrantIssued || len(record.Reply.Diagnostics) != 0 || record.Reply.Reason != "" || len(record.Reply.RecoveryActions) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Stage Grant state or reply", nil)
	}
	grant := snapshot.Grants[0]
	if err := admission.ValidateGrant(grant); err != nil || grant.RunID != snapshot.RunID || grant.RequestID != snapshot.RequestID || grant.DeliverableID != workflow.Input.DeliverableID || grant.InputDigest != workflow.Input.InputDigest || grant.IssuedRevision != record.Revision || grant.Generation != bundle.Generation || grant.BundleID != bundle.ID || grant.NodeID != workflow.ActiveNodeID || grant.GraphDigest != bundle.GraphDigest || grant.RegistryDigest != workflow.RegistryDigest || grant.CatalogDigest != bundle.Configuration.CatalogDigest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid active Workflow Stage Grant", err)
	}
	return nil
}

func validateWorkflowRevisionTransition(previous, current RunSnapshot) error {
	if previous.Workflow == nil || current.Workflow == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow revision edge", nil)
	}
	if previous.Status == RunReady && current.Status == RunGranted {
		previousState := cloneWorkflowState(*previous.Workflow)
		currentState := cloneWorkflowState(*current.Workflow)
		previousState.ActiveGrantID, currentState.ActiveGrantID = "", ""
		if !reflect.DeepEqual(previousState, currentState) || !reflect.DeepEqual(previous.LifecycleBundles, current.LifecycleBundles) || len(previous.Grants) != 0 || len(current.Grants) != 1 || len(previous.GrantIDs) != 0 || len(current.GrantIDs) != 1 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Stage Grant transition", nil)
		}
		return nil
	}
	if previous.Status != RunAwaitingSelection || current.Status != RunReady {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow revision edge", nil)
	}
	previousIdentity := *previous.Workflow
	currentIdentity := *current.Workflow
	previousIdentity.Bundles, currentIdentity.Bundles = nil, nil
	previousIdentity.ActiveGeneration, currentIdentity.ActiveGeneration = 0, 0
	previousIdentity.ActiveNodeID, currentIdentity.ActiveNodeID = "", ""
	if !reflect.DeepEqual(previousIdentity, currentIdentity) || len(previous.Workflow.Bundles) != 0 || len(current.Workflow.Bundles) != 1 || len(previous.LifecycleBundles) != 0 || len(current.LifecycleBundles) != 1 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow selection changed immutable request state", nil)
	}
	return nil
}
