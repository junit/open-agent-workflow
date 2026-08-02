package runtime

import (
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

const (
	workflowSignalSucceeded         = "succeeded"
	workflowSignalFinding           = "finding"
	workflowSignalRemediated        = "remediated"
	workflowSignalFunctionalFailure = "functional-failure"
	workflowSignalBuildFailure      = "build-failure"
	workflowSignalDependencyFailure = "dependency-failure"
	workflowSignalTypeFailure       = "type-failure"
	workflowSignalSecurityFinding   = "security-finding"
)

func normalizeStageObservation(value *StageObservation) (*StageObservation, error) {
	if value == nil {
		return nil, runtimeError("OBSERVATION_INVALID", "Workflow Stage Observation is required", nil)
	}
	base, err := normalizeCapabilityObservation(&value.CapabilityObservation)
	if err != nil {
		return nil, err
	}
	signal := strings.TrimSpace(value.Signal)
	if !workflowObservationSignalAllowed(signal) {
		return nil, runtimeError("OBSERVATION_INVALID", "Workflow observation signal is not closed or normalized", nil)
	}
	if signal == workflowSignalSucceeded || signal == workflowSignalRemediated {
		if base.Outcome != ObservationSucceeded {
			return nil, runtimeError("OBSERVATION_INVALID", "successful Workflow signal requires a successful outcome", nil)
		}
	} else if base.Outcome != ObservationFailed {
		return nil, runtimeError("OBSERVATION_INVALID", "incident Workflow signal requires a failed outcome", nil)
	}
	boundary := strings.TrimSpace(value.StableBoundary)
	if boundary != "" {
		if validateIdentifier(boundary) != nil {
			return nil, runtimeError("OBSERVATION_INVALID", "Workflow stable boundary is invalid", nil)
		}
	}
	return &StageObservation{CapabilityObservation: *base, Signal: signal, StableBoundary: boundary}, nil
}

func workflowObservationSignalAllowed(signal string) bool {
	switch signal {
	case workflowSignalSucceeded, workflowSignalFinding, workflowSignalRemediated,
		workflowSignalFunctionalFailure, workflowSignalBuildFailure, workflowSignalDependencyFailure,
		workflowSignalTypeFailure, workflowSignalSecurityFinding:
		return true
	default:
		return false
	}
}

func normalizeStableBoundarySwitch(value *StableBoundarySwitch) (*StableBoundarySwitch, error) {
	if value == nil || validateIdentifier(strings.TrimSpace(value.Boundary)) != nil {
		return nil, runtimeError("STABLE_BOUNDARY_INVALID", "stable boundary selection is invalid", nil)
	}
	selection, err := normalizeProfileSelection(value.Selection)
	if err != nil {
		return nil, err
	}
	return &StableBoundarySwitch{Boundary: strings.TrimSpace(value.Boundary), Selection: selection}, nil
}

func (engine *Engine) continueWorkflow(current revisionRecord, frame RunFrame, normalized ContinueInput, messageDigest string) (RunReply, error) {
	switch normalized.Signal {
	case SignalDispatchPrepared:
		return engine.authorizeWorkflowDispatch(current, frame, normalized.DispatchPreparation, messageDigest)
	case SignalCapabilityObserved:
		return engine.observeWorkflowStage(current, frame, normalized.StageObservation, messageDigest)
	case SignalExecutionUncertain:
		return engine.pauseWorkflowExecution(current, frame, messageDigest, ReasonExecutionUncertain, RecoveryReconcileInvocation, "WORKFLOW_EXECUTION_UNCERTAIN")
	case SignalAdditionalCapabilityRequired, SignalRemediationRequired, SignalArchitectureRequired:
		return engine.pauseWorkflowExecution(current, frame, messageDigest, ReasonModeEscalationRequired, RecoveryStartSuccessorRun, workflowPauseEvent(normalized.Signal))
	case SignalSwitchProfile:
		return engine.switchWorkflowProfile(current, frame, normalized.StableBoundarySwitch, messageDigest)
	default:
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "unsupported Workflow transition", nil)
	}
}

func (engine *Engine) authorizeWorkflowDispatch(current revisionRecord, frame RunFrame, preparation *DispatchPreparation, messageDigest string) (RunReply, error) {
	if current.Snapshot.RequestMode != classification.RequestModeWorkflow || current.Snapshot.Status != RunGranted || current.Snapshot.Workflow == nil || preparation == nil {
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "DISPATCH_PREPARED requires a granted Workflow run", nil)
	}
	grant, err := workflowActiveGrant(current.Snapshot)
	if err != nil {
		return RunReply{}, err
	}
	if preparation.GrantID != grant.ID || preparation.InvocationID != grant.InvocationID || preparation.ExecutorID != grant.Executor.ID {
		return RunReply{}, runtimeError("DISPATCH_PREPARATION_INVALID", "preparation does not identify the committed Workflow Grant", nil)
	}
	snapshot := workflowTransitionSnapshot(current.Snapshot, frame, messageDigest, current.Revision+1)
	snapshot.Status = RunInFlight
	return engine.commitWorkflowTransition(current, frame, snapshot, "WORKFLOW_DISPATCH_AUTHORIZED", messageDigest, workflowReply(snapshot, ReplyDispatchAuthorized, "", nil))
}

func (engine *Engine) observeWorkflowStage(current revisionRecord, frame RunFrame, observation *StageObservation, messageDigest string) (RunReply, error) {
	if current.Snapshot.RequestMode != classification.RequestModeWorkflow || current.Snapshot.Status != RunInFlight || current.Snapshot.Workflow == nil || observation == nil {
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "CAPABILITY_OBSERVED requires an authorized Workflow invocation", nil)
	}
	grant, err := workflowActiveGrant(current.Snapshot)
	if err != nil {
		return RunReply{}, err
	}
	if observation.GrantID != grant.ID || observation.InvocationID != grant.InvocationID || observation.ExecutorID != grant.Executor.ID {
		return RunReply{}, runtimeError("OBSERVATION_INVALID", "observation does not identify the authorized Workflow invocation", nil)
	}
	bundle, err := workflowActiveBundle(current.Snapshot)
	if err != nil {
		return RunReply{}, err
	}
	node, found := workflowGraphNode(bundle.Graph, current.Snapshot.Workflow.ActiveNodeID)
	if !found {
		return RunReply{}, runtimeError("RUN_STATE_REVISION_INVALID", "active Workflow node is missing", nil)
	}
	target, routedIncident, found := workflowObservationTarget(bundle.Graph, node, observation.Signal)
	if !found {
		return RunReply{}, runtimeError("WORKFLOW_GRAPH_SIGNAL_INVALID", "observation signal is not declared by the active graph node", nil)
	}
	if observation.StableBoundary != "" {
		if observation.Outcome != ObservationSucceeded || !containsWorkflowValue(bundle.Graph.StableBoundaries, observation.StableBoundary) {
			return RunReply{}, runtimeError("STABLE_BOUNDARY_INVALID", "observation stable boundary is not declared by the active Bundle", nil)
		}
	}
	snapshot := workflowTransitionSnapshot(current.Snapshot, frame, messageDigest, current.Revision+1)
	snapshot.Workflow.Observations = append(snapshot.Workflow.Observations, cloneStageObservation(*observation))
	if observation.StableBoundary != "" {
		snapshot.Workflow.LastStableBoundary = observation.StableBoundary
	}
	snapshot.Workflow.ActiveNodeID = target
	snapshot.Workflow.ActiveGrantID = ""
	terminal := workflowGraphTerminal(bundle.Graph, node.ID) && target == node.ID && observation.Outcome == ObservationSucceeded
	if terminal {
		snapshot.Status = RunFinished
		snapshot.ResourceLeaseIDs = []string{}
	} else {
		snapshot.Status = RunReady
		if observation.Outcome == ObservationSucceeded || observation.Signal == workflowSignalRemediated {
			snapshot.ResourceLeaseIDs = []string{}
		}
	}
	event := "WORKFLOW_STAGE_OBSERVED"
	if routedIncident {
		event = "WORKFLOW_INCIDENT_ROUTED"
	}
	if terminal {
		event = "WORKFLOW_COMPLETED"
	}
	if terminal {
		return engine.commitWorkflowTransition(current, frame, snapshot, event, messageDigest, workflowReply(snapshot, ReplyFinished, "", nil))
	}
	return engine.commitWorkflowTransition(current, frame, snapshot, event, messageDigest, workflowReply(snapshot, ReplyModeDecided, "", nil))
}

func workflowObservationTarget(graph profile.ExecutionGraphRecord, node profile.GraphNode, signal string) (string, bool, bool) {
	for _, transition := range node.Transitions {
		if transition.Signal == signal {
			return transition.Target, false, true
		}
	}
	for _, route := range graph.IncidentRoutes {
		if route.Incident == signal {
			return route.Handler, true, true
		}
	}
	if workflowGraphTerminal(graph, node.ID) && signal == workflowSignalSucceeded {
		return node.ID, false, true
	}
	return "", false, false
}

func workflowGraphTerminal(graph profile.ExecutionGraphRecord, nodeID string) bool {
	return containsWorkflowValue(graph.TerminalGates, nodeID)
}

func workflowActiveGrant(snapshot RunSnapshot) (admission.CapabilityGrant, error) {
	if snapshot.Workflow == nil || snapshot.Workflow.ActiveGrantID == "" {
		return admission.CapabilityGrant{}, runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Grant is missing", nil)
	}
	for _, grant := range snapshot.Grants {
		if grant.ID == snapshot.Workflow.ActiveGrantID {
			return admission.CloneGrant(grant), nil
		}
	}
	return admission.CapabilityGrant{}, runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Grant is not in Grant history", nil)
}

func workflowActiveBundle(snapshot RunSnapshot) (LifecycleBundle, error) {
	if snapshot.Workflow == nil || snapshot.Workflow.ActiveGeneration == 0 {
		return LifecycleBundle{}, runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Bundle is missing", nil)
	}
	for _, bundle := range snapshot.Workflow.Bundles {
		if bundle.Generation == snapshot.Workflow.ActiveGeneration {
			return cloneLifecycleBundle(bundle), nil
		}
	}
	return LifecycleBundle{}, runtimeError("RUN_STATE_REVISION_INVALID", "Workflow active Bundle generation is missing", nil)
}

func workflowTransitionSnapshot(current RunSnapshot, frame RunFrame, messageDigest string, nextRevision uint64) RunSnapshot {
	snapshot := cloneSnapshot(current)
	snapshot.Revision = nextRevision
	snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{
		IdempotencyKey: frame.IdempotencyKey, ContentDigest: messageDigest, Revision: nextRevision,
	})
	sort.Slice(snapshot.ProcessedMessages, func(left, right int) bool {
		return snapshot.ProcessedMessages[left].IdempotencyKey < snapshot.ProcessedMessages[right].IdempotencyKey
	})
	return snapshot
}

func (engine *Engine) pauseWorkflowExecution(current revisionRecord, frame RunFrame, messageDigest, reason, recovery, event string) (RunReply, error) {
	if current.Snapshot.RequestMode != classification.RequestModeWorkflow || current.Snapshot.Status != RunInFlight || current.Snapshot.Workflow == nil {
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "Workflow pause requires an in-flight invocation", nil)
	}
	if _, err := workflowActiveGrant(current.Snapshot); err != nil {
		return RunReply{}, err
	}
	snapshot := workflowTransitionSnapshot(current.Snapshot, frame, messageDigest, current.Revision+1)
	snapshot.Status = RunPaused
	return engine.commitWorkflowTransition(current, frame, snapshot, event, messageDigest, workflowReply(snapshot, ReplyPaused, reason, []string{recovery}))
}

func workflowPauseEvent(signal ContinueSignal) string {
	switch signal {
	case SignalAdditionalCapabilityRequired:
		return "WORKFLOW_ADDITIONAL_CAPABILITY_REQUIRED"
	case SignalRemediationRequired:
		return "WORKFLOW_REMEDIATION_REQUIRED"
	case SignalArchitectureRequired:
		return "WORKFLOW_ARCHITECTURE_REQUIRED"
	default:
		return "WORKFLOW_PAUSED"
	}
}

func (engine *Engine) commitWorkflowTransition(current revisionRecord, frame RunFrame, snapshot RunSnapshot, event, messageDigest string, candidateReply RunReply) (RunReply, error) {
	committed, err := engine.journal.commit(revisionRecord{
		SchemaVersion: revisionSchemaV1, RunID: frame.RunID, Revision: snapshot.Revision,
		PredecessorDigest: current.Digest, MessageID: frame.MessageID, IdempotencyKey: frame.IdempotencyKey,
		MessageDigest: messageDigest, Event: event,
		Snapshot: snapshot, Reply: candidateReply,
	})
	if err != nil {
		return RunReply{}, err
	}
	engine.projectCommittedWorkflow(committed)
	return cloneReply(committed.Reply), nil
}

func workflowReply(snapshot RunSnapshot, kind ReplyKind, reason string, recovery []string) RunReply {
	return RunReply{SchemaVersion: RuntimeSchemaV1, Kind: kind, RunID: snapshot.RunID, Revision: snapshot.Revision,
		Snapshot: cloneSnapshot(snapshot), Diagnostics: []Diagnostic{}, Reason: reason, RecoveryActions: append([]string{}, recovery...)}
}

func cloneStageObservation(value StageObservation) StageObservation {
	value.CapabilityObservation.EvidenceReferences = append([]EvidenceReference{}, value.CapabilityObservation.EvidenceReferences...)
	return value
}

func (engine *Engine) switchWorkflowProfile(current revisionRecord, frame RunFrame, request *StableBoundarySwitch, messageDigest string) (RunReply, error) {
	if current.Snapshot.RequestMode != classification.RequestModeWorkflow || current.Snapshot.Workflow == nil || request == nil {
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "Profile switching requires a Workflow run", nil)
	}
	if current.Snapshot.Status != RunReady || current.Snapshot.Workflow.ActiveGrantID != "" {
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "Profile switching is not allowed during an active invocation", nil)
	}
	if current.Snapshot.Workflow.LastStableBoundary != request.Boundary {
		return RunReply{}, runtimeError("STABLE_BOUNDARY_INVALID", "Profile switching requires the last committed stable boundary", nil)
	}
	bundle, err := workflowActiveBundle(current.Snapshot)
	if err != nil {
		return RunReply{}, err
	}
	if !containsWorkflowValue(bundle.Graph.StableBoundaries, request.Boundary) {
		return RunReply{}, runtimeError("STABLE_BOUNDARY_INVALID", "stable boundary is not declared by the active Bundle", nil)
	}
	if !validDigest(engine.workflow.Configuration.Digest()) || !validDigest(engine.workflow.Registry.Digest()) || !engine.workflow.Host.PhysicalIsolation {
		return RunReply{}, runtimeError("HOST_ISOLATION_UNAVAILABLE", "Workflow trusted isolation or configuration is unavailable", nil)
	}
	graph, err := profile.CompileProfile(engine.workflow.Configuration.Catalog(), engine.workflow.Registry, profile.CompileRequest{Profile: request.Selection.Profile, Bindings: request.Selection.Bindings})
	if err != nil {
		return RunReply{}, runtimeError("PROFILE_SELECTION_INVALID", "selected Profile is not available", err)
	}
	nextRevision := current.Revision + 1
	newBundle, err := newLifecycleBundle(bundleRequest{
		RunID: current.RunID, DeliverableID: current.Snapshot.Workflow.Input.DeliverableID, InputDigest: current.Snapshot.Workflow.Input.InputDigest,
		Generation: current.Snapshot.Workflow.ActiveGeneration + 1, CreatedRevision: nextRevision, Selection: request.Selection,
		Configuration: engine.workflow.Configuration.Record(), RegistryDigest: engine.workflow.Registry.Digest(), Graph: graph.Record(),
	})
	if err != nil {
		return RunReply{}, err
	}
	snapshot := workflowTransitionSnapshot(current.Snapshot, frame, messageDigest, nextRevision)
	snapshot.Status = RunReady
	snapshot.Workflow.Bundles = append(snapshot.Workflow.Bundles, cloneLifecycleBundle(newBundle))
	snapshot.Workflow.ConfigurationDigest = newBundle.Configuration.Digest
	snapshot.Workflow.RegistryDigest = newBundle.RegistryDigest
	snapshot.Workflow.ActiveGeneration = newBundle.Generation
	snapshot.Workflow.ActiveNodeID = newBundle.Graph.Entry
	snapshot.Workflow.ActiveGrantID = ""
	snapshot.Workflow.LastStableBoundary = ""
	snapshot.LifecycleBundles = append(snapshot.LifecycleBundles, newBundle.ID)
	snapshot.ResourceLeaseIDs = []string{}
	snapshot.Workflow.RevokedGrantIDs = appendRevokedWorkflowGrants(snapshot.Workflow.RevokedGrantIDs, snapshot.Grants, current.Snapshot.Workflow.ActiveGeneration)
	return engine.commitWorkflowTransition(current, frame, snapshot, "WORKFLOW_BUNDLE_SWITCHED", messageDigest, workflowReply(snapshot, ReplyModeDecided, "", nil))
}

func appendRevokedWorkflowGrants(existing []string, grants []admission.CapabilityGrant, generation uint64) []string {
	seen := make(map[string]struct{}, len(existing)+len(grants))
	result := append([]string{}, existing...)
	for _, value := range result {
		seen[value] = struct{}{}
	}
	for _, grant := range grants {
		if grant.Generation != generation {
			continue
		}
		if _, found := seen[grant.ID]; found {
			continue
		}
		seen[grant.ID] = struct{}{}
		result = append(result, grant.ID)
	}
	return result
}
