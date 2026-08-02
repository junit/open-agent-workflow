package runtime

import (
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func cloneWorkflowOptions(value WorkflowOptions) WorkflowOptions {
	value.Authority = admission.CloneAuthority(value.Authority)
	value.Host = host.CloneRuntimeFrame(value.Host)
	value.Executors = append([]WorkflowExecutorRegistration{}, value.Executors...)
	return value
}

func cloneWorkflowInput(value *WorkflowInput) *WorkflowInput {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func normalizeWorkflowInput(value *WorkflowInput) (WorkflowInput, error) {
	if value == nil || validateIdentifier(value.DeliverableID) != nil || !validDigest(value.InputDigest) {
		return WorkflowInput{}, runtimeError("WORKFLOW_REQUEST_INVALID", "invalid Workflow identity", nil)
	}
	return *value, nil
}

func workflowConfigurationReady(project ProjectIdentity, options WorkflowOptions) bool {
	return validDigest(options.Configuration.Digest()) && validDigest(options.Registry.Digest()) && project.ConfigurationDigest == options.Configuration.Digest()
}

func workflowAwaitingState(input WorkflowInput, options WorkflowOptions) *WorkflowState {
	return &WorkflowState{
		Input: input, ConfigurationDigest: options.Configuration.Digest(), RegistryDigest: options.Registry.Digest(),
		Bundles: []LifecycleBundle{}, ActiveGeneration: 0, ActiveNodeID: "", ActiveGrantID: "",
		Observations: []StageObservation{}, RevokedGrantIDs: []string{}, ResourceLeases: []ResourceLease{}, ProjectionLag: []ProjectionLag{},
	}
}

func (engine *Engine) selectWorkflowProfile(current revisionRecord, frame RunFrame, input ContinueInput, messageDigest string) (RunReply, error) {
	if current.Snapshot.RequestMode != classification.RequestModeWorkflow || current.Snapshot.Status != RunAwaitingSelection || current.Snapshot.Workflow == nil || input.ProfileSelection == nil {
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "Profile selection requires an awaiting Workflow run", nil)
	}
	if !workflowConfigurationReady(current.Snapshot.Project, engine.workflow) || current.Snapshot.Workflow.ConfigurationDigest != engine.workflow.Configuration.Digest() || current.Snapshot.Workflow.RegistryDigest != engine.workflow.Registry.Digest() {
		return RunReply{}, runtimeError("WORKFLOW_CONFIGURATION_REQUIRED", "active Run trusted inputs do not match Engine options", nil)
	}
	graph, err := profile.CompileProfile(engine.workflow.Configuration.Catalog(), engine.workflow.Registry, profile.CompileRequest{
		Profile: input.ProfileSelection.Profile, Bindings: input.ProfileSelection.Bindings,
	})
	if err != nil {
		return RunReply{}, runtimeError("PROFILE_SELECTION_INVALID", "selected Profile is not available", err)
	}
	hostAdmission, err := admitWorkflowHost(engine.workflow, graph.Record())
	if err != nil {
		return RunReply{}, err
	}
	nextRevision := current.Revision + 1
	bundle, err := newLifecycleBundle(bundleRequest{
		RunID: current.RunID, DeliverableID: current.Snapshot.Workflow.Input.DeliverableID,
		InputDigest: current.Snapshot.Workflow.Input.InputDigest, Generation: 1, CreatedRevision: nextRevision,
		Selection: *input.ProfileSelection, Configuration: engine.workflow.Configuration.Record(),
		RegistryDigest: engine.workflow.Registry.Digest(), Graph: graph.Record(), Host: hostAdmission,
	})
	if err != nil {
		return RunReply{}, err
	}
	snapshot := cloneSnapshot(current.Snapshot)
	snapshot.Revision = nextRevision
	snapshot.Status = RunReady
	snapshot.Workflow.Bundles = []LifecycleBundle{cloneLifecycleBundle(bundle)}
	snapshot.Workflow.ActiveGeneration = bundle.Generation
	snapshot.Workflow.ActiveNodeID = bundle.Graph.Entry
	snapshot.LifecycleBundles = []string{bundle.ID}
	snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{
		IdempotencyKey: frame.IdempotencyKey, ContentDigest: messageDigest, Revision: nextRevision,
	})
	sort.Slice(snapshot.ProcessedMessages, func(left, right int) bool {
		return snapshot.ProcessedMessages[left].IdempotencyKey < snapshot.ProcessedMessages[right].IdempotencyKey
	})
	committed, err := engine.journal.commit(revisionRecord{
		SchemaVersion: revisionSchemaV1, RunID: current.RunID, Revision: nextRevision,
		PredecessorDigest: current.Digest, MessageID: frame.MessageID, IdempotencyKey: frame.IdempotencyKey,
		MessageDigest: messageDigest, Event: "WORKFLOW_BUNDLE_CREATED", Snapshot: snapshot, Reply: workflowReadyReply(snapshot),
	})
	if err != nil {
		return RunReply{}, err
	}
	engine.projectCommittedWorkflow(committed)
	return cloneReply(committed.Reply), nil
}

func workflowSelectionReply(snapshot RunSnapshot) RunReply {
	return RunReply{
		SchemaVersion: RuntimeSchemaV1, Kind: ReplySelectionRequired, RunID: snapshot.RunID,
		Revision: snapshot.Revision, Snapshot: cloneSnapshot(snapshot),
		Diagnostics:     []Diagnostic{{Code: "SELECTION_REQUIRED", Message: "Workflow Profile selection is required."}},
		RecoveryActions: []string{},
	}
}

func workflowReadyReply(snapshot RunSnapshot) RunReply {
	return RunReply{
		SchemaVersion: RuntimeSchemaV1, Kind: ReplyModeDecided, RunID: snapshot.RunID,
		Revision: snapshot.Revision, Snapshot: cloneSnapshot(snapshot), Diagnostics: []Diagnostic{}, RecoveryActions: []string{},
	}
}

func cloneWorkflowState(value WorkflowState) WorkflowState {
	value.Bundles = append([]LifecycleBundle{}, value.Bundles...)
	for index := range value.Bundles {
		value.Bundles[index] = cloneLifecycleBundle(value.Bundles[index])
	}
	value.RevokedGrantIDs = append([]string{}, value.RevokedGrantIDs...)
	value.Observations = append([]StageObservation{}, value.Observations...)
	for index := range value.Observations {
		value.Observations[index].CapabilityObservation.EvidenceReferences = append([]EvidenceReference{}, value.Observations[index].CapabilityObservation.EvidenceReferences...)
	}
	value.ResourceLeases = append([]ResourceLease{}, value.ResourceLeases...)
	value.ProjectionLag = append([]ProjectionLag{}, value.ProjectionLag...)
	return value
}

func normalizedWorkflowSet(values []string) ([]string, error) {
	normalized := append([]string{}, values...)
	sort.Strings(normalized)
	for index, value := range normalized {
		if strings.TrimSpace(value) != value || value == "" || index > 0 && normalized[index-1] == value {
			return nil, runtimeError("WORKFLOW_REQUEST_INVALID", "invalid or duplicate set value", nil)
		}
	}
	return normalized, nil
}
