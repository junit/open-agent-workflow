package runtime

import (
	"path/filepath"
	"reflect"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

func validateWorkflowState(record revisionRecord) error {
	snapshot := record.Snapshot
	if snapshot.RequestMode != classification.RequestModeWorkflow || snapshot.Classification.RequestMode != classification.RequestModeWorkflow || !validDigest(snapshot.ClassificationDigest) || snapshot.Workflow == nil || snapshot.Bounded != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow classification state", nil)
	}
	if snapshot.Status != RunAwaitingSelection && snapshot.Status != RunReady && snapshot.Status != RunGranted && snapshot.Status != RunInFlight && snapshot.Status != RunPaused && snapshot.Status != RunFinished {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow status", nil)
	}
	if validateIdentifier(snapshot.RequestID) != nil || snapshot.Project.Root == "" || !filepath.IsAbs(snapshot.Project.Root) || filepath.Clean(snapshot.Project.Root) != snapshot.Project.Root || !validDigest(snapshot.ConfigurationDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Workflow identity", nil)
	}
	workflow := snapshot.Workflow
	if _, err := catalog.ParseLocalID(workflow.HostID); err != nil || !validDigest(workflow.ConfigurationDigest) || !validDigest(workflow.RegistryDigest) || validateIdentifier(workflow.Input.DeliverableID) != nil || !validDigest(workflow.Input.InputDigest) || workflow.Input.ActiveTicket != "" && validateIdentifier(workflow.Input.ActiveTicket) != nil || workflow.ActiveTicket != workflow.Input.ActiveTicket {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Workflow trusted inputs", nil)
	}
	if snapshot.ProcessedMessages == nil || uint64(len(snapshot.ProcessedMessages)) != record.Revision || snapshot.Observations != nil || snapshot.GrantIDs == nil || snapshot.ResourceLeaseIDs == nil || workflow.Observations == nil || workflow.RevokedGrantIDs == nil || workflow.ResourceLeases == nil || workflow.ProjectionLag == nil || len(workflow.ProjectionLag) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow authority collections", nil)
	}
	if snapshot.Classification.WorkflowComplexity == nil || snapshot.Classification.CapabilitySelector != nil || snapshot.Classification.EvidenceRequirements == nil || snapshot.Classification.EscalationReasons == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow classification details", nil)
	}
	if err := validateWorkflowMessages(record); err != nil {
		return err
	}
	if snapshot.Status == RunAwaitingSelection {
		if record.Revision != 1 || record.Event != "WORKFLOW_SELECTION_REQUIRED" || len(workflow.Bundles) != 0 || workflow.ActiveGeneration != 0 || workflow.ActiveNodeID != "" || workflow.ActiveGrantID != "" || len(workflow.Observations) != 0 || snapshot.Grants != nil || len(snapshot.GrantIDs) != 0 || len(snapshot.ResourceLeaseIDs) != 0 || len(workflow.ResourceLeases) != 0 || snapshot.LifecycleBundles == nil || len(snapshot.LifecycleBundles) != 0 || record.Reply.Kind != ReplySelectionRequired || len(record.Reply.Diagnostics) != 1 || record.Reply.Diagnostics[0].Code != "SELECTION_REQUIRED" || record.Reply.Reason != "" || len(record.Reply.RecoveryActions) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow selection state or reply", nil)
		}
		return nil
	}
	if len(workflow.Bundles) == 0 || len(snapshot.LifecycleBundles) != len(workflow.Bundles) || workflow.ActiveGeneration == 0 || workflow.ActiveNodeID == "" {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Bundle collection", nil)
	}
	if err := validateWorkflowBundles(snapshot, record); err != nil {
		return err
	}
	if err := validateWorkflowGrants(snapshot, record); err != nil {
		return err
	}
	if err := validateWorkflowObservations(snapshot, record); err != nil {
		return err
	}
	if err := validateWorkflowResourceLeases(record); err != nil {
		return err
	}
	if err := validateWorkflowReply(record); err != nil {
		return err
	}
	return nil
}

func validateWorkflowMessages(record revisionRecord) error {
	snapshot := record.Snapshot
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
	return nil
}

func validateWorkflowBundles(snapshot RunSnapshot, record revisionRecord) error {
	workflow := snapshot.Workflow
	for index, bundle := range workflow.Bundles {
		if err := validateLifecycleBundle(bundle); err != nil || bundle.HostID != workflow.HostID || bundle.RunID != snapshot.RunID || bundle.DeliverableID != workflow.Input.DeliverableID || bundle.InputDigest != workflow.Input.InputDigest || bundle.Generation != uint64(index+1) || bundle.CreatedRevision > record.Revision || snapshot.LifecycleBundles[index] != bundle.ID {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Workflow Bundle", err)
		}
		if index > 0 && workflow.Bundles[index-1].Generation >= bundle.Generation {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Bundle generations are not monotonic", nil)
		}
	}
	active, err := workflowActiveBundle(snapshot)
	if err != nil {
		return err
	}
	if active.ID != workflow.Bundles[len(workflow.Bundles)-1].ID || active.Generation != workflow.ActiveGeneration || workflow.ActiveNodeID == "" {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Bundle does not match the latest generation", nil)
	}
	if active.HostID != workflow.HostID || active.Configuration.Digest != workflow.ConfigurationDigest || active.RegistryDigest != workflow.RegistryDigest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active trusted inputs do not match the active Bundle", nil)
	}
	if _, found := workflowGraphNode(active.Graph, workflow.ActiveNodeID); !found {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active node is missing from the active Bundle", nil)
	}
	if workflow.LastStableBoundary != "" && !containsWorkflowValue(active.Graph.StableBoundaries, workflow.LastStableBoundary) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow stable boundary is not declared by the active Bundle", nil)
	}
	return nil
}

func validateWorkflowGrants(snapshot RunSnapshot, record revisionRecord) error {
	workflow := snapshot.Workflow
	if len(snapshot.GrantIDs) != len(snapshot.Grants) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Grant compatibility history does not match typed history", nil)
	}
	seen := make(map[string]struct{}, len(snapshot.Grants))
	for index, grant := range snapshot.Grants {
		if err := admission.ValidateGrant(grant); err != nil || grant.RunID != snapshot.RunID || grant.RequestID != snapshot.RequestID || grant.IssuedRevision > record.Revision {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Workflow Grant", err)
		}
		if snapshot.GrantIDs[index] != grant.ID {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Grant compatibility ID mismatch", nil)
		}
		if _, exists := seen[grant.ID]; exists {
			return runtimeError("RUN_STATE_REVISION_INVALID", "duplicate persisted Workflow Grant", nil)
		}
		seen[grant.ID] = struct{}{}
		bundle, found := workflowBundleByID(workflow.Bundles, grant.BundleID)
		if !found || grant.Generation != bundle.Generation || grant.GraphDigest != bundle.GraphDigest || grant.NodeID == "" {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Grant is not bound to a persisted Bundle", nil)
		}
		node, found := workflowGraphNode(bundle.Graph, grant.NodeID)
		if !found || grant.ProviderID != node.ProviderID || grant.ProviderInstanceDigest != node.ProviderInstanceDigest || grant.CapabilityID != node.CapabilityID || grant.Binding != node.Binding || grant.Binding.Host != bundle.HostID {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Grant node is missing from its Bundle", nil)
		}
	}
	revoked := make(map[string]struct{}, len(workflow.RevokedGrantIDs))
	for _, grantID := range workflow.RevokedGrantIDs {
		grant, found := grantByID(snapshot.Grants, grantID)
		if !found || grant.Generation >= workflow.ActiveGeneration {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Grant revocation history", nil)
		}
		if _, duplicate := revoked[grantID]; duplicate {
			return runtimeError("RUN_STATE_REVISION_INVALID", "duplicate Workflow Grant revocation history", nil)
		}
		revoked[grantID] = struct{}{}
	}
	activeRequired := snapshot.Status == RunGranted || snapshot.Status == RunInFlight || snapshot.Status == RunPaused
	if activeRequired {
		active, found := grantByID(snapshot.Grants, workflow.ActiveGrantID)
		if !found || len(snapshot.GrantIDs) == 0 || snapshot.GrantIDs[len(snapshot.GrantIDs)-1] != workflow.ActiveGrantID {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Grant is not the latest Grant", nil)
		}
		if _, revoked := revoked[workflow.ActiveGrantID]; revoked {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Grant is revoked", nil)
		}
		bundle, found := workflowBundleByID(workflow.Bundles, active.BundleID)
		if !found || bundle.Generation != workflow.ActiveGeneration || active.Generation != workflow.ActiveGeneration || active.NodeID != workflow.ActiveNodeID {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Grant is outside the active graph node", nil)
		}
	} else if workflow.ActiveGrantID != "" {
		return runtimeError("RUN_STATE_REVISION_INVALID", "inactive Workflow state retains an active Grant", nil)
	}
	return nil
}

func grantByID(values []admission.CapabilityGrant, id string) (admission.CapabilityGrant, bool) {
	for _, grant := range values {
		if grant.ID == id {
			return grant, true
		}
	}
	return admission.CapabilityGrant{}, false
}

func validateWorkflowObservations(snapshot RunSnapshot, record revisionRecord) error {
	workflow := snapshot.Workflow
	grants := make(map[string]admission.CapabilityGrant, len(snapshot.Grants))
	for _, grant := range snapshot.Grants {
		grants[grant.ID] = grant
	}
	seen := make(map[string]struct{}, len(workflow.Observations))
	for _, observation := range workflow.Observations {
		normalized, err := normalizeStageObservation(&observation)
		if err != nil || !equalStageObservation(*normalized, observation) {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Workflow Stage Observation", err)
		}
		grant, found := grants[observation.GrantID]
		if !found || observation.GrantID == "" || grant.InvocationID != observation.InvocationID || grant.Executor.ID != observation.ExecutorID {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation exceeds its Grant", nil)
		}
		if _, exists := seen[observation.GrantID]; exists {
			return runtimeError("RUN_STATE_REVISION_INVALID", "duplicate Workflow observation for Grant", nil)
		}
		seen[observation.GrantID] = struct{}{}
		bundle, found := workflowBundleByID(workflow.Bundles, grant.BundleID)
		if !found {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation Bundle is missing", nil)
		}
		node, found := workflowGraphNode(bundle.Graph, grant.NodeID)
		if !found {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation node is missing", nil)
		}
		if _, _, found := workflowObservationTarget(bundle.Graph, node, observation.Signal); !found {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation signal is not declared by its graph", nil)
		}
		if observation.StableBoundary != "" && (observation.Outcome != ObservationSucceeded || !containsWorkflowValue(bundle.Graph.StableBoundaries, observation.StableBoundary)) {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation stable boundary is invalid", nil)
		}
	}
	return nil
}

func equalStageObservation(left, right StageObservation) bool {
	if left.GrantID != right.GrantID || left.InvocationID != right.InvocationID || left.ExecutorID != right.ExecutorID || left.Outcome != right.Outcome || left.RawOutput != right.RawOutput || left.Signal != right.Signal || left.StableBoundary != right.StableBoundary || len(left.EvidenceReferences) != len(right.EvidenceReferences) {
		return false
	}
	for index := range left.EvidenceReferences {
		if left.EvidenceReferences[index] != right.EvidenceReferences[index] {
			return false
		}
	}
	return true
}

func validateWorkflowReply(record revisionRecord) error {
	reply := record.Reply
	if reply.Diagnostics == nil || reply.RecoveryActions == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow reply collections", nil)
	}
	snapshot := record.Snapshot
	switch snapshot.Status {
	case RunReady:
		if reply.Kind != ReplyModeDecided || reply.Reason != "" || len(reply.Diagnostics) != 0 || len(reply.RecoveryActions) != 0 || record.Event != "WORKFLOW_BUNDLE_CREATED" && record.Event != "WORKFLOW_STAGE_OBSERVED" && record.Event != "WORKFLOW_INCIDENT_ROUTED" && record.Event != "WORKFLOW_BUNDLE_SWITCHED" {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow ready reply", nil)
		}
		if snapshot.Workflow.ActiveGrantID != "" {
			return runtimeError("RUN_STATE_REVISION_INVALID", "ready Workflow reply retains an active Grant", nil)
		}
	case RunGranted:
		if record.Event != "WORKFLOW_STAGE_GRANT_ISSUED" || reply.Kind != ReplyGrantIssued || reply.Reason != "" || len(reply.Diagnostics) != 0 || len(reply.RecoveryActions) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Grant reply", nil)
		}
	case RunInFlight:
		if record.Event != "WORKFLOW_DISPATCH_AUTHORIZED" || reply.Kind != ReplyDispatchAuthorized || reply.Reason != "" || len(reply.Diagnostics) != 0 || len(reply.RecoveryActions) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow dispatch reply", nil)
		}
	case RunPaused:
		if reply.Kind != ReplyPaused || len(reply.Diagnostics) != 0 || len(reply.RecoveryActions) != 1 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow paused reply", nil)
		}
		if reply.Reason == ReasonExecutionUncertain {
			if record.Event != "WORKFLOW_EXECUTION_UNCERTAIN" || reply.RecoveryActions[0] != RecoveryReconcileInvocation {
				return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow uncertainty reply", nil)
			}
		} else if reply.Reason != ReasonModeEscalationRequired || reply.RecoveryActions[0] != RecoveryStartSuccessorRun {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow escalation reply", nil)
		}
	case RunFinished:
		if record.Event != "WORKFLOW_COMPLETED" || reply.Kind != ReplyFinished || reply.Reason != "" || len(reply.Diagnostics) != 0 || len(reply.RecoveryActions) != 0 || snapshot.Workflow.ActiveGrantID != "" || len(snapshot.ResourceLeaseIDs) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow finished reply", nil)
		}
	default:
		return runtimeError("RUN_STATE_REVISION_INVALID", "unsupported Workflow reply status", nil)
	}
	return nil
}

func validateWorkflowRevisionTransition(previous, current RunSnapshot) error {
	if previous.Workflow == nil || current.Workflow == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow revision edge", nil)
	}
	switch previous.Status {
	case RunAwaitingSelection:
		return validateWorkflowSelectionTransition(previous, current)
	case RunReady:
		if current.Status == RunGranted {
			return validateWorkflowGrantTransition(previous, current)
		}
		if current.Status == RunReady && current.Workflow.ActiveGeneration > previous.Workflow.ActiveGeneration {
			return validateWorkflowBundleSwitchTransition(previous, current)
		}
	case RunGranted:
		if current.Status == RunInFlight {
			if !reflect.DeepEqual(previous.Grants, current.Grants) || !reflect.DeepEqual(previous.GrantIDs, current.GrantIDs) || !reflect.DeepEqual(previous.Workflow, current.Workflow) || !reflect.DeepEqual(previous.ResourceLeaseIDs, current.ResourceLeaseIDs) || !reflect.DeepEqual(previous.LifecycleBundles, current.LifecycleBundles) {
				return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow dispatch transition", nil)
			}
			return nil
		}
	case RunInFlight:
		return validateWorkflowObservationTransition(previous, current)
	}
	return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow revision edge", nil)
}

func validateWorkflowSelectionTransition(previous, current RunSnapshot) error {
	if current.Status != RunReady {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow selection transition", nil)
	}
	previousIdentity := *previous.Workflow
	currentIdentity := *current.Workflow
	previousIdentity.Bundles, currentIdentity.Bundles = nil, nil
	previousIdentity.ActiveGeneration, currentIdentity.ActiveGeneration = 0, 0
	previousIdentity.ActiveNodeID, currentIdentity.ActiveNodeID = "", ""
	if !reflect.DeepEqual(previousIdentity, currentIdentity) || len(previous.Workflow.Bundles) != 0 || len(current.Workflow.Bundles) != 1 || current.Workflow.Bundles[0].CreatedRevision != current.Revision || len(previous.LifecycleBundles) != 0 || len(current.LifecycleBundles) != 1 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow selection changed immutable request state", nil)
	}
	return nil
}

func validateWorkflowGrantTransition(previous, current RunSnapshot) error {
	if len(previous.Grants) != len(previous.GrantIDs) || len(current.Grants) != len(current.GrantIDs) || len(current.Grants) != len(previous.Grants)+1 || !appendOnlyWorkflowGrants(previous.Grants, current.Grants) || !appendOnlyStrings(previous.GrantIDs, current.GrantIDs) || !appendOnlyWorkflowLeases(previous.Workflow.ResourceLeases, current.Workflow.ResourceLeases) || !workflowStageLeaseTransitionIDs(previous, current) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Stage Grant transition", nil)
	}
	previousState := cloneWorkflowState(*previous.Workflow)
	currentState := cloneWorkflowState(*current.Workflow)
	previousState.ActiveGrantID, currentState.ActiveGrantID = "", ""
	previousState.ResourceLeases, currentState.ResourceLeases = nil, nil
	if !reflect.DeepEqual(previousState, currentState) || !reflect.DeepEqual(previous.LifecycleBundles, current.LifecycleBundles) || len(current.GrantIDs) == 0 || current.Workflow.ActiveGrantID != current.GrantIDs[len(current.GrantIDs)-1] {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Stage Grant changed immutable state", nil)
	}
	grant := current.Grants[len(current.Grants)-1]
	bundle, found := workflowBundleByID(current.Workflow.Bundles, grant.BundleID)
	if !found || grant.IssuedRevision != current.Revision || grant.Generation != current.Workflow.ActiveGeneration || bundle.Generation != current.Workflow.ActiveGeneration || grant.NodeID != current.Workflow.ActiveNodeID {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Stage Grant is outside the active graph node", nil)
	}
	return nil
}

func appendOnlyWorkflowGrants(previous, current []admission.CapabilityGrant) bool {
	if len(current) != len(previous)+1 {
		return false
	}
	for index := range previous {
		if !reflect.DeepEqual(previous[index], current[index]) {
			return false
		}
	}
	return true
}

func appendOnlyStrings(previous, current []string) bool {
	if len(current) != len(previous)+1 {
		return false
	}
	for index := range previous {
		if previous[index] != current[index] {
			return false
		}
	}
	return true
}

func appendOnlyWorkflowLeases(previous, current []ResourceLease) bool {
	if len(current) != len(previous) && len(current) != len(previous)+1 {
		return false
	}
	for index := range previous {
		if !reflect.DeepEqual(previous[index], current[index]) {
			return false
		}
	}
	return true
}

func workflowStageLeaseTransitionIDs(previous, current RunSnapshot) bool {
	if len(previous.ResourceLeaseIDs) == len(current.ResourceLeaseIDs) {
		return reflect.DeepEqual(previous.ResourceLeaseIDs, current.ResourceLeaseIDs)
	}
	return len(current.ResourceLeaseIDs) == len(previous.ResourceLeaseIDs)+1 && appendOnlyStrings(previous.ResourceLeaseIDs, current.ResourceLeaseIDs)
}

func validateWorkflowObservationTransition(previous, current RunSnapshot) error {
	if current.Status != RunReady && current.Status != RunFinished && current.Status != RunPaused {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow observation transition", nil)
	}
	if !reflect.DeepEqual(previous.Grants, current.Grants) || !reflect.DeepEqual(previous.GrantIDs, current.GrantIDs) || !reflect.DeepEqual(previous.LifecycleBundles, current.LifecycleBundles) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation changed immutable history", nil)
	}
	if current.Status == RunPaused && len(previous.Workflow.Observations) == len(current.Workflow.Observations) {
		if !reflect.DeepEqual(previous.Workflow, current.Workflow) || !reflect.DeepEqual(previous.ResourceLeaseIDs, current.ResourceLeaseIDs) {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow uncertainty changed invocation identity", nil)
		}
		return nil
	}
	if current.Status == RunPaused || len(current.Workflow.Observations) != len(previous.Workflow.Observations)+1 || !appendOnlyStageObservations(previous.Workflow.Observations, current.Workflow.Observations) || previous.Workflow.ActiveGrantID == "" || current.Workflow.ActiveGrantID != "" {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation did not append exactly one result", nil)
	}
	previousState := cloneWorkflowState(*previous.Workflow)
	currentState := cloneWorkflowState(*current.Workflow)
	previousState.ActiveNodeID, currentState.ActiveNodeID = "", ""
	previousState.ActiveGrantID, currentState.ActiveGrantID = "", ""
	previousState.Observations, currentState.Observations = nil, nil
	previousState.LastStableBoundary, currentState.LastStableBoundary = "", ""
	if !reflect.DeepEqual(previousState, currentState) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation changed immutable orchestration state", nil)
	}
	observation := current.Workflow.Observations[len(current.Workflow.Observations)-1]
	if observation.GrantID != previous.Workflow.ActiveGrantID {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation does not complete the active Grant", nil)
	}
	grant, found := grantByID(previous.Grants, observation.GrantID)
	if !found || grant.Generation != previous.Workflow.ActiveGeneration || grant.NodeID != previous.Workflow.ActiveNodeID {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation is outside the active graph node", nil)
	}
	bundle, found := workflowBundleByID(previous.Workflow.Bundles, grant.BundleID)
	if !found {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation Bundle is missing", nil)
	}
	node, found := workflowGraphNode(bundle.Graph, grant.NodeID)
	if !found {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation node is missing", nil)
	}
	target, _, found := workflowObservationTarget(bundle.Graph, node, observation.Signal)
	if !found || current.Workflow.ActiveNodeID != target {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation skipped the declared graph target", nil)
	}
	terminal := workflowGraphTerminal(bundle.Graph, node.ID) && target == node.ID && observation.Outcome == ObservationSucceeded
	expectedStatus := RunReady
	if terminal {
		expectedStatus = RunFinished
	}
	if current.Status != expectedStatus {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation produced the wrong terminal state", nil)
	}
	expectedLeaseIDs := append([]string{}, previous.ResourceLeaseIDs...)
	if observation.Outcome == ObservationSucceeded || observation.Signal == workflowSignalRemediated {
		expectedLeaseIDs = []string{}
	}
	if !reflect.DeepEqual(expectedLeaseIDs, current.ResourceLeaseIDs) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation changed Resource Lease ownership incorrectly", nil)
	}
	expectedBoundary := previous.Workflow.LastStableBoundary
	if observation.StableBoundary != "" {
		expectedBoundary = observation.StableBoundary
	}
	if current.Workflow.LastStableBoundary != expectedBoundary {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow observation changed the stable boundary incorrectly", nil)
	}
	return nil
}

func appendOnlyStageObservations(previous, current []StageObservation) bool {
	if len(current) < len(previous) {
		return false
	}
	for index := range previous {
		if !equalStageObservation(previous[index], current[index]) {
			return false
		}
	}
	return true
}

func validateWorkflowBundleSwitchTransition(previous, current RunSnapshot) error {
	if previous.Workflow.LastStableBoundary == "" || len(current.Workflow.Bundles) != len(previous.Workflow.Bundles)+1 || len(current.LifecycleBundles) != len(previous.LifecycleBundles)+1 || !appendOnlyLifecycleBundles(previous.Workflow.Bundles, current.Workflow.Bundles) || !appendOnlyStrings(previous.LifecycleBundles, current.LifecycleBundles) || current.Workflow.ActiveGeneration != previous.Workflow.ActiveGeneration+1 || current.Workflow.ActiveGrantID != "" || len(current.ResourceLeaseIDs) != 0 || current.Workflow.LastStableBoundary != "" || !reflect.DeepEqual(previous.Grants, current.Grants) || !reflect.DeepEqual(previous.GrantIDs, current.GrantIDs) || current.Workflow.ActiveNodeID != current.Workflow.Bundles[len(current.Workflow.Bundles)-1].Graph.Entry || current.Workflow.Bundles[len(current.Workflow.Bundles)-1].CreatedRevision != current.Revision {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Workflow Bundle switch transition", nil)
	}
	expectedRevocations := appendRevokedWorkflowGrants(previous.Workflow.RevokedGrantIDs, previous.Grants, previous.Workflow.ActiveGeneration)
	if !reflect.DeepEqual(expectedRevocations, current.Workflow.RevokedGrantIDs) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Bundle switch did not revoke the previous generation", nil)
	}
	previousState := cloneWorkflowState(*previous.Workflow)
	currentState := cloneWorkflowState(*current.Workflow)
	previousState.Bundles, currentState.Bundles = nil, nil
	previousState.ActiveGeneration, currentState.ActiveGeneration = 0, 0
	previousState.ActiveNodeID, currentState.ActiveNodeID = "", ""
	previousState.ActiveGrantID, currentState.ActiveGrantID = "", ""
	previousState.RevokedGrantIDs, currentState.RevokedGrantIDs = nil, nil
	previousState.LastStableBoundary, currentState.LastStableBoundary = "", ""
	previousState.ConfigurationDigest, currentState.ConfigurationDigest = "", ""
	previousState.RegistryDigest, currentState.RegistryDigest = "", ""
	if !reflect.DeepEqual(previousState, currentState) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow Bundle switch changed immutable history", nil)
	}
	return nil
}

func appendOnlyLifecycleBundles(previous, current []LifecycleBundle) bool {
	if len(current) != len(previous)+1 {
		return false
	}
	for index := range previous {
		if !reflect.DeepEqual(previous[index], current[index]) {
			return false
		}
	}
	return true
}
