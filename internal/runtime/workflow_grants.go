package runtime

import (
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func normalizeStageGrantRequest(value *StageGrantRequest) (*StageGrantRequest, error) {
	if value == nil || strings.TrimSpace(value.TerminationCondition) == "" || validateIdentifier(strings.TrimSpace(value.TerminationCondition)) != nil {
		return nil, runtimeError("WORKFLOW_GRANT_INVALID", "Stage Grant request is incomplete", nil)
	}
	effects, err := normalizedWorkflowSet(value.RequestedEffects)
	if err != nil || len(effects) == 0 {
		return nil, runtimeError("WORKFLOW_GRANT_INVALID", "Stage effects must be a unique non-empty set", err)
	}
	resources, err := normalizedWorkflowSet(value.RequestedResources)
	if err != nil || len(resources) == 0 {
		return nil, runtimeError("WORKFLOW_GRANT_INVALID", "Stage resources must be a unique non-empty set", err)
	}
	result := *value
	result.ExecutorID = strings.TrimSpace(result.ExecutorID)
	result.RequestedEffects = effects
	result.RequestedResources = resources
	result.TerminationCondition = strings.TrimSpace(result.TerminationCondition)
	return &result, nil
}

func (engine *Engine) issueWorkflowStage(current revisionRecord, frame RunFrame, request *StageGrantRequest, messageDigest string) (RunReply, error) {
	snapshot := current.Snapshot
	if snapshot.RequestMode != classification.RequestModeWorkflow || snapshot.Status != RunReady || snapshot.Workflow == nil || snapshot.Workflow.ActiveGrantID != "" || len(snapshot.Workflow.Bundles) == 0 {
		return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "Stage Grant requires a ready Workflow run without an active Grant", nil)
	}
	if !workflowConfigurationReady(snapshot.Project, engine.workflow) || snapshot.Workflow.ConfigurationDigest != engine.workflow.Configuration.Digest() || snapshot.Workflow.RegistryDigest != engine.workflow.Registry.Digest() || !engine.workflow.Host.PhysicalIsolation {
		return RunReply{}, runtimeError("HOST_ISOLATION_UNAVAILABLE", "Workflow trusted isolation or configuration is unavailable", nil)
	}
	bundle := snapshot.Workflow.Bundles[len(snapshot.Workflow.Bundles)-1]
	node, found := workflowGraphNode(bundle.Graph, snapshot.Workflow.ActiveNodeID)
	if !found {
		return RunReply{}, runtimeError("RUN_STATE_REVISION_INVALID", "active Workflow node is missing", nil)
	}
	executor, err := selectWorkflowExecutor(node, request.ExecutorID, engine.workflow.Executors, snapshot.Grants)
	if err != nil {
		return RunReply{}, err
	}
	registrations := make([]admission.ExecutorRegistration, len(engine.workflow.Executors))
	for index, value := range engine.workflow.Executors {
		registrations[index] = value.Registration
	}
	nextRevision := current.Revision + 1
	grant, err := admission.IssueWorkflowStageGrant(admission.WorkflowStageGrantRequest{
		Grant: admission.GrantRequest{
			RunID: snapshot.RunID, RequestID: snapshot.RequestID, DeliverableID: snapshot.Workflow.Input.DeliverableID,
			InputDigest: snapshot.Workflow.Input.InputDigest, IssuedRevision: nextRevision,
			Selector: classification.CapabilitySelector{ProviderID: node.ProviderID, CapabilityID: node.CapabilityID, Source: classification.SelectorUserIntent},
			Effects:  request.RequestedEffects, Resources: request.RequestedResources, TerminationCondition: request.TerminationCondition,
			Executor: executor, DelegationAllowList: node.DelegationAllowList,
			Catalog: engine.workflow.Configuration.Catalog(), Registry: engine.workflow.Registry,
			Authority: engine.workflow.Authority, Executors: registrations,
		},
		BundleID: bundle.ID, NodeID: node.ID, GraphDigest: bundle.GraphDigest,
		Generation: bundle.Generation, Node: node,
	})
	if err != nil {
		return RunReply{}, runtimeError(admission.ErrorCode(err), "Workflow Stage Grant admission failed", err)
	}
	lease, err := engine.journal.acquireWorkflowResourceLease(current, grant)
	if err != nil {
		return RunReply{}, err
	}
	next := cloneSnapshot(snapshot)
	next.Revision = nextRevision
	next.Status = RunGranted
	next.Grants = append(next.Grants, admission.CloneGrant(grant))
	next.GrantIDs = append(next.GrantIDs, grant.ID)
	next.Workflow.ActiveGrantID = grant.ID
	if lease.ID != "" {
		if _, found := workflowResourceLease(next.Workflow.ResourceLeases, lease.ID); !found {
			next.Workflow.ResourceLeases = append(next.Workflow.ResourceLeases, lease)
		}
		if !containsWorkflowValue(next.ResourceLeaseIDs, lease.ID) {
			next.ResourceLeaseIDs = append(next.ResourceLeaseIDs, lease.ID)
		}
	}
	next.ProcessedMessages = append(next.ProcessedMessages, ProcessedMessage{
		IdempotencyKey: frame.IdempotencyKey, ContentDigest: messageDigest, Revision: nextRevision,
	})
	sort.Slice(next.ProcessedMessages, func(left, right int) bool {
		return next.ProcessedMessages[left].IdempotencyKey < next.ProcessedMessages[right].IdempotencyKey
	})
	committed, err := engine.journal.commit(revisionRecord{
		SchemaVersion: revisionSchemaV1, RunID: snapshot.RunID, Revision: nextRevision,
		PredecessorDigest: current.Digest, MessageID: frame.MessageID, IdempotencyKey: frame.IdempotencyKey,
		MessageDigest: messageDigest, Event: "WORKFLOW_STAGE_GRANT_ISSUED", Snapshot: next, Reply: workflowGrantReply(next),
	})
	if err != nil {
		return RunReply{}, err
	}
	return cloneReply(committed.Reply), nil
}

func selectWorkflowExecutor(node profile.GraphNode, requested string, available []WorkflowExecutorRegistration, prior []admission.CapabilityGrant) (admission.ExecutorRegistration, error) {
	used := make(map[string]struct{}, len(prior))
	for _, grant := range prior {
		used[grant.Executor.ID] = struct{}{}
	}
	if requested != "" {
		for _, candidate := range available {
			if candidate.Registration.ID == requested && candidate.Registration.Kind == admission.ExecutorIsolated {
				if node.Responsibility == "review" && (!candidate.ReadOnly || !candidate.Fresh) {
					return admission.ExecutorRegistration{}, runtimeError("REVIEW_EXECUTOR_REQUIRED", "Review requires a fresh read-only Executor", nil)
				}
				if node.Responsibility == "review" {
					if _, found := used[requested]; found {
						return admission.ExecutorRegistration{}, runtimeError("REVIEW_EXECUTOR_REQUIRED", "Review Executor was already used", nil)
					}
				}
				return candidate.Registration, nil
			}
		}
		return admission.ExecutorRegistration{}, runtimeError("EXECUTOR_NOT_REGISTERED", requested, nil)
	}
	if node.Responsibility != "review" {
		return admission.ExecutorRegistration{}, runtimeError("EXECUTOR_NOT_REGISTERED", "Stage Executor is required", nil)
	}
	for _, candidate := range available {
		if candidate.Registration.Kind != admission.ExecutorIsolated || !candidate.ReadOnly || !candidate.Fresh {
			continue
		}
		if _, found := used[candidate.Registration.ID]; !found {
			return candidate.Registration, nil
		}
	}
	return admission.ExecutorRegistration{}, runtimeError("REVIEW_EXECUTOR_REQUIRED", "fresh read-only Review Executor is unavailable", nil)
}

func workflowGraphNode(graph profile.ExecutionGraphRecord, id string) (profile.GraphNode, bool) {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return profile.GraphNode{}, false
}

func workflowGrantReply(snapshot RunSnapshot) RunReply {
	return RunReply{
		SchemaVersion: RuntimeSchemaV1, Kind: ReplyGrantIssued, RunID: snapshot.RunID,
		Revision: snapshot.Revision, Snapshot: cloneSnapshot(snapshot), Diagnostics: []Diagnostic{}, RecoveryActions: []string{},
	}
}

func workflowNodeAllowsWrites(node profile.GraphNode) bool {
	for _, effect := range node.MaximumEffects {
		if effect == "write-project" || effect == "git-local" {
			return true
		}
	}
	return false
}

func workflowNodeReadOnly(node profile.GraphNode) bool {
	return !workflowNodeAllowsWrites(node) && node.ExecutorTopology == catalog.IsolatedRequired
}
