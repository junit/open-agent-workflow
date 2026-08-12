package coordinator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func (engine *Engine) start(command Command) (Result, error) {
	if command.Start == nil {
		return Result{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "START input is required", nil)
	}
	trustedHost := engine.options.Host.Record()
	if command.Start.HostSession.Digest != trustedHost.SessionDigest || command.Start.Environment.Digest != trustedHost.EnvironmentDigest {
		return Result{}, coordinatorError("WORKFLOW_HOST_EVIDENCE_MISMATCH", "START Host session or environment does not match trusted Coordinator Host evidence", nil)
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
		decision = cloneClassificationDecision(decision)
		if decision.RequestMode != classification.RequestModeWorkflow {
			return coordinatorError("WORKFLOW_CLASSIFICATION_REQUIRED", fmt.Sprintf("START classification is %s; only WORKFLOW enters the Coordinator", decision.RequestMode), nil)
		}
		request := compilationRequestFromStart(engine.options, decision, *command.Start)
		compileRequest := cloneCompilationRequest(request)
		compiled, compileErr := engine.core.Compile(compileRequest)
		if compileErr != nil {
			return coordinatorError("WORKFLOW_CORE_COMPILE_FAILED", "Core compilation failed", compileErr)
		}
		bundle, verifyErr := verifyStartCompilation(request, compiled)
		if verifyErr != nil {
			return verifyErr
		}
		cursor, cursorErr := profile.FirstActionableCursor(bundle.Graph)
		if cursorErr != nil {
			return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle has no actionable cursor", cursorErr)
		}
		snapshot := Snapshot{
			SchemaVersion: WorkflowSnapshotSchemaV2, WorkflowID: workflowID, RequestID: command.Start.RequestID,
			DeliverableID: command.Start.DeliverableID, Revision: 1, Status: StatusReady, Classification: decision,
			Bundles: []core.LifecycleBundle{bundle}, ActiveGeneration: bundle.Generation, Cursor: cursor,
			ActiveTicket: command.Start.ActiveTicket, GrantHistory: []admission.CapabilityGrant{}, UserAuthorizations: []admission.UserAuthorization{},
			InvocationAttestations: []admission.ExplicitInvocationAttestation{}, GateAttestations: []GateAttestation{}, Receipts: []host.InvocationReceipt{},
			ResourceLeases: []ResourceLease{}, LastStableBoundary: "", ProcessedMessages: []ProcessedMessage{{
				IdempotencyKey: command.IdempotencyKey, ContentDigest: messageDigest, Revision: 1,
			}}, ProjectionLag: []ProjectionLag{},
		}
		candidate := revisionRecord{
			SchemaVersion: WorkflowRevisionSchemaV2, WorkflowID: workflowID, Revision: 1, MessageID: command.MessageID,
			IdempotencyKey: command.IdempotencyKey, MessageDigest: messageDigest, Event: "WORKFLOW_STARTED", Snapshot: snapshot,
			Result: Result{SchemaVersion: WorkflowResultSchemaV2, Kind: ResultState, WorkflowID: workflowID, Revision: 1, Diagnostics: []Diagnostic{}},
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
	selection := cloneSelection(input.Selection)
	return core.CompilationRequest{
		DeliverableID: input.DeliverableID, InputDigest: input.InputDigest, Generation: 1, Classification: decision,
		Configuration: options.Configuration, ResolutionDigest: options.Resolutions.Digest(), Registry: options.Registry, Host: options.Host,
		Selection: &selection,
	}
}

func cloneCompilationRequest(request core.CompilationRequest) core.CompilationRequest {
	cloned := request
	cloned.Classification = cloneClassificationDecision(request.Classification)
	selection := cloneSelection(*request.Selection)
	cloned.Selection = &selection
	return cloned
}

func cloneSelection(selection core.Selection) core.Selection {
	selection.AddOns = append([]string{}, selection.AddOns...)
	selection.Alternatives = append([]profile.AlternativeChoice{}, selection.Alternatives...)
	selection.Overlays = append([]string{}, selection.Overlays...)
	return selection
}

func startMessageDigest(input StartInput) (string, error) {
	record := struct {
		SchemaVersion string      `json:"schema_version"`
		Kind          CommandKind `json:"kind"`
		Start         StartInput  `json:"start"`
	}{WorkflowCommandSchemaV2, CommandStart, input}
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
	trustedHost := request.Host.Record()
	if bundle.SchemaVersion != core.LifecycleBundleSchemaV4 || !validStableID("bundle-", bundle.ID) || bundle.DeliverableID != request.DeliverableID ||
		bundle.InputDigest != request.InputDigest || bundle.Generation != request.Generation || bundle.ClassificationDigest != request.Classification.Digest() ||
		bundle.HostID != trustedHost.HostID || bundle.HostSessionDigest != trustedHost.SessionDigest ||
		bundle.ReporterIdentityDigest != trustedHost.ReporterIdentityDigest || bundle.HostManifestDigest != trustedHost.ManifestDigest ||
		bundle.EnvironmentReportDigest != trustedHost.EnvironmentDigest || bundle.ProviderInventoryDigest != trustedHost.InventoryDigest ||
		bundle.HostFeatureDigest != trustedHost.FeatureDigest || bundle.HostActionDigest != trustedHost.ActionDigest ||
		bundle.Topology != request.Selection.Topology || bundle.ResolutionDigest != request.ResolutionDigest || bundle.RegistryDigest != request.Registry.Digest() {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle does not match trusted START inputs", nil)
	}
	if !sameCanonicalValue(bundle.Selection, *request.Selection) || !sameCanonicalValue(bundle.Configuration, request.Configuration.Record()) ||
		!sameCanonicalValue(bundle.Classification, request.Classification) || bundle.Classification.Digest() != request.Classification.Digest() ||
		!sameCanonicalValue(bundle.AddOns, request.Selection.AddOns) || bundle.HostEvidenceDigest != trustedHost.Digest {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle selection or trusted facts differ", nil)
	}
	recipeDigest, _, err := canonicaljson.Digest(bundle.Recipe)
	if err != nil || recipeDigest != bundle.RecipeDigest {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle Recipe digest mismatch", err)
	}
	if err := profile.ValidateExecutionGraphRecord(bundle.Graph); err != nil {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle graph is invalid", err)
	}
	graphSelection := profile.Selection{
		Profile: request.Selection.Profile, RecipeID: request.Selection.RecipeID, RecipeDigest: request.Selection.RecipeDigest,
		Topology: request.Selection.Topology, AddOns: request.Selection.AddOns, Alternatives: request.Selection.Alternatives,
		Overlays: request.Selection.Overlays, Digest: request.Selection.GraphSelectionDigest,
	}
	if bundle.Graph.HostID != trustedHost.HostID || bundle.Graph.HostEvidenceDigest != trustedHost.Digest ||
		bundle.Graph.RegistryDigest != request.Registry.Digest() || bundle.Graph.RecipeID != bundle.Recipe.ID ||
		bundle.Graph.RecipeDigest != bundle.RecipeDigest || bundle.Graph.Selection.Digest != request.Selection.GraphSelectionDigest ||
		!sameCanonicalValue(bundle.Graph.Selection, graphSelection) || bundle.Graph.EntrySlotID == "" || bundle.Graph.Topology != request.Selection.Topology {
		return coordinatorError("WORKFLOW_CORE_RESULT_INVALID", "Core Bundle graph authority or topology is invalid", nil)
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
