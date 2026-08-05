package coordinator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

const lifecycleBundleSchemaV3 = "oaw.lifecycle-bundle/v3"

func (engine *Engine) start(command Command) (Result, error) {
	if command.Start == nil {
		return Result{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "START input is required", nil)
	}
	messageDigest, err := startMessageDigest(*command.Start)
	if err != nil {
		return Result{}, err
	}
	workflowID := deriveWorkflowID(command.IdempotencyKey)
	var result Result
	err = engine.journal.withWorkflowLock(workflowID, func() error {
		replayed, found, replayErr := engine.journal.replay(workflowID, command.IdempotencyKey, messageDigest)
		if replayErr == nil && found {
			result = replayed
			return nil
		}
		if replayErr != nil && ErrorCode(replayErr) != "WORKFLOW_NOT_FOUND" {
			return replayErr
		}
		if replayErr == nil && !found {
			return coordinatorError("WORKFLOW_IDEMPOTENCY_KEY_REUSED", "derived Workflow already contains another message", nil)
		}

		decision, classifyErr := engine.core.Classify(&command.Start.Proposal, engine.options.Rules)
		if classifyErr != nil {
			return coordinatorError("WORKFLOW_CLASSIFICATION_FAILED", "Core classification failed", classifyErr)
		}
		if decision.RequestMode != classification.RequestModeWorkflow {
			return coordinatorError("WORKFLOW_CLASSIFICATION_REQUIRED", fmt.Sprintf("START classification is %s; only WORKFLOW enters the Coordinator", decision.RequestMode), nil)
		}
		request := compilationRequestFromStart(engine.options, decision, *command.Start)
		compiled, compileErr := engine.core.Compile(request)
		if compileErr != nil {
			return coordinatorError("WORKFLOW_CORE_COMPILE_FAILED", "Core compilation failed", compileErr)
		}
		bundle, verifyErr := verifyStartCompilation(request, compiled)
		if verifyErr != nil {
			return verifyErr
		}
		snapshot := Snapshot{
			SchemaVersion: WorkflowSnapshotSchemaV1, WorkflowID: workflowID, RequestID: command.Start.RequestID,
			DeliverableID: command.Start.DeliverableID, Revision: 1, Status: StatusReady, Classification: decision,
			Bundles: []core.LifecycleBundle{bundle}, ActiveGeneration: bundle.Generation, ActiveNodeID: bundle.Graph.Entry,
			ActiveTicket: command.Start.ActiveTicket, GrantHistory: []admission.CapabilityGrant{}, Receipts: []host.InvocationReceipt{},
			ResourceLeases: []ResourceLease{}, LastStableBoundary: "", ProcessedMessages: []ProcessedMessage{{
				IdempotencyKey: command.IdempotencyKey, ContentDigest: messageDigest, Revision: 1,
			}}, ProjectionLag: []ProjectionLag{},
		}
		candidate := revisionRecord{
			SchemaVersion: WorkflowRevisionSchemaV1, WorkflowID: workflowID, Revision: 1, MessageID: command.MessageID,
			IdempotencyKey: command.IdempotencyKey, MessageDigest: messageDigest, Event: "WORKFLOW_STARTED", Snapshot: snapshot,
			Result: Result{SchemaVersion: WorkflowResultSchemaV1, Kind: ResultState, WorkflowID: workflowID, Revision: 1, Diagnostics: []Diagnostic{}},
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

func compilationRequestFromStart(options Options, decision classification.ClassificationDecision, input StartInput) core.CompilationRequest {
	selection := input.Selection
	return core.CompilationRequest{
		DeliverableID: input.DeliverableID, InputDigest: input.InputDigest, Generation: 1, Classification: decision,
		Configuration: options.Configuration, Resolutions: options.Resolutions, Registry: options.Registry,
		HostID: input.HostSession.HostID, HostSessionDigest: input.HostSession.Digest, HostEnvironmentReportDigest: input.Environment.Digest,
		HostProviderInventoryDigest: input.HostSession.ProviderInventoryDigest,
		HostTopologies:              append([]execution.Topology{}, input.HostSession.SupportedTopologies...), EnvironmentObservations: append([]execution.EnvironmentObservation{}, input.Environment.Observations...),
		Selection: &selection,
	}
}

func startMessageDigest(input StartInput) (string, error) {
	record := struct {
		SchemaVersion string      `json:"schema_version"`
		Kind          CommandKind `json:"kind"`
		Start         StartInput  `json:"start"`
	}{WorkflowCommandSchemaV1, CommandStart, input}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return "", coordinatorError("WORKFLOW_COMMAND_INVALID", "digest START input", err)
	}
	return digest, nil
}

func deriveWorkflowID(idempotencyKey string) string {
	digest := sha256.Sum256([]byte(idempotencyKey))
	return "workflow-" + hex.EncodeToString(digest[:16])
}

func verifyStartCompilation(request core.CompilationRequest, result core.CompilationResult) (core.LifecycleBundle, error) {
	if !validDigest(result.Digest) {
		return core.LifecycleBundle{}, coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Compilation Result digest is invalid", nil)
	}
	resultDigest := result
	resultDigest.Digest = ""
	digest, _, err := canonicaljson.Digest(resultDigest)
	if err != nil || digest != result.Digest {
		return core.LifecycleBundle{}, coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Compilation Result digest mismatch", err)
	}
	if result.Bundle == nil {
		return core.LifecycleBundle{}, coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core did not return exactly one Bundle", nil)
	}
	bundle, err := cloneStartBundle(*result.Bundle)
	if err != nil {
		return core.LifecycleBundle{}, coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "clone Core Bundle", err)
	}
	if err := validateStartBundle(request, bundle); err != nil {
		return core.LifecycleBundle{}, err
	}
	return bundle, nil
}

func validateStartBundle(request core.CompilationRequest, bundle core.LifecycleBundle) error {
	if bundle.SchemaVersion != lifecycleBundleSchemaV3 || !validText(bundle.ID, 512) || bundle.DeliverableID != request.DeliverableID ||
		bundle.InputDigest != request.InputDigest || bundle.Generation != request.Generation || bundle.ClassificationDigest != request.Classification.Digest() ||
		bundle.HostID != request.HostID || bundle.HostSessionDigest != request.HostSessionDigest || bundle.EnvironmentReportDigest != request.HostEnvironmentReportDigest ||
		bundle.ProviderInventoryDigest != request.HostProviderInventoryDigest ||
		bundle.Topology != request.Selection.Topology || bundle.ResolutionDigest != request.Resolutions.Digest() || bundle.RegistryDigest != request.Registry.Digest() {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle does not match trusted START inputs", nil)
	}
	if !sameCanonicalValue(bundle.Selection, *request.Selection) || !sameCanonicalValue(bundle.Configuration, request.Configuration.Record()) ||
		!sameCanonicalValue(bundle.Classification, request.Classification) || bundle.Classification.Digest() != request.Classification.Digest() ||
		!sameCanonicalValue(bundle.EnvironmentObservations, request.EnvironmentObservations) || !sameCanonicalValue(bundle.AddOns, request.Selection.AddOns) {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle selection or trusted facts differ", nil)
	}
	if err := profile.ValidateExecutionGraphRecord(bundle.Graph); err != nil {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle graph is invalid", err)
	}
	entry, found := graphNode(bundle.Graph, bundle.Graph.Entry)
	if bundle.Graph.HostID != request.HostID || bundle.Graph.Entry == "" || !slices.Contains(bundle.Graph.EligibleTopologies, request.Selection.Topology) || !found ||
		!slices.Contains(entry.SupportedTopologies, request.Selection.Topology) || !slices.Contains(entry.Binding.Topologies, request.Selection.Topology) {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle graph topology or entry is invalid", nil)
	}
	if !sameCanonicalValue(bundle.ProviderInstances, bundle.Graph.ProviderInstances) || !sameCanonicalValue(bundle.EnvironmentRequirements, bundle.Graph.EnvironmentRequirements) {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle graph projections differ", nil)
	}
	if !validDigest(bundle.Digest) {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle digest is invalid", nil)
	}
	unsigned := bundle
	unsigned.Digest = ""
	digest, _, err := canonicaljson.Digest(unsigned)
	if err != nil || digest != bundle.Digest {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle digest mismatch", err)
	}
	return nil
}

func graphNode(graph profile.ExecutionGraphRecord, id string) (profile.GraphNode, bool) {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return profile.GraphNode{}, false
}

func cloneStartBundle(value core.LifecycleBundle) (core.LifecycleBundle, error) {
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		return core.LifecycleBundle{}, err
	}
	var cloned core.LifecycleBundle
	if err := decodeStrictState(raw, &cloned); err != nil {
		return core.LifecycleBundle{}, err
	}
	cloned.Classification = cloneClassificationDecision(value.Classification)
	return cloned, nil
}
