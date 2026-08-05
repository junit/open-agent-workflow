package coordinator

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func (engine *Engine) switchWorkflow(command Command) (Result, error) {
	if command.Switch == nil {
		return Result{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "SWITCH input is required", nil)
	}
	messageDigest, err := switchMessageDigest(*command.Switch)
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
			return coordinatorError("WORKFLOW_REVISION_CONFLICT", "SWITCH expected revision does not match committed Workflow state", nil)
		}
		if current.Snapshot.Status != StatusReady || current.Snapshot.ActiveGrant != nil {
			return coordinatorError("WORKFLOW_SWITCH_INVALID", "SWITCH requires READY state without an active Grant", nil)
		}
		bundle, bundleErr := activeBundle(current.Snapshot)
		if bundleErr != nil {
			return coordinatorError("WORKFLOW_SWITCH_INVALID", "active Bundle is unavailable", bundleErr)
		}
		if current.Snapshot.LastStableBoundary != command.Switch.Boundary || !containsString(bundle.Graph.StableBoundaries, command.Switch.Boundary) {
			return coordinatorError("WORKFLOW_STABLE_BOUNDARY_INVALID", "SWITCH requires the last committed stable boundary declared by the active Bundle", nil)
		}
		request := compilationRequestFromSwitch(engine.options, current.Snapshot, bundle, *command.Switch)
		compiled, compileErr := engine.core.Compile(request)
		if compileErr != nil {
			return coordinatorError("WORKFLOW_CORE_COMPILE_FAILED", "Core recompilation failed", compileErr)
		}
		nextBundle, verifyErr := verifyStartCompilation(request, compiled)
		if verifyErr != nil {
			return verifyErr
		}
		nextRevision := current.Revision + 1
		snapshot := cloneSnapshot(current.Snapshot)
		snapshot.Revision = nextRevision
		snapshot.Status = StatusReady
		snapshot.Bundles = append(snapshot.Bundles, nextBundle)
		snapshot.ActiveGeneration = nextBundle.Generation
		snapshot.ActiveNodeID = nextBundle.Graph.Entry
		snapshot.ActiveGrant = nil
		snapshot.LastStableBoundary = ""
		if releaseErr := releaseResourceLeases(&snapshot, nextRevision); releaseErr != nil {
			return releaseErr
		}
		snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{
			IdempotencyKey: command.IdempotencyKey, ContentDigest: messageDigest, Revision: nextRevision,
		})
		sort.Slice(snapshot.ProcessedMessages, func(left, right int) bool {
			return snapshot.ProcessedMessages[left].IdempotencyKey < snapshot.ProcessedMessages[right].IdempotencyKey
		})
		candidate := revisionRecord{
			SchemaVersion: WorkflowRevisionSchemaV1, WorkflowID: command.WorkflowID, Revision: nextRevision,
			PredecessorDigest: current.Digest, MessageID: command.MessageID, IdempotencyKey: command.IdempotencyKey,
			MessageDigest: messageDigest, Event: "WORKFLOW_BUNDLE_SWITCHED", Snapshot: snapshot,
			Result: Result{SchemaVersion: WorkflowResultSchemaV1, Kind: ResultState, WorkflowID: command.WorkflowID, Revision: nextRevision, Diagnostics: []Diagnostic{}},
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

func compilationRequestFromSwitch(options Options, snapshot Snapshot, bundle core.LifecycleBundle, input SwitchInput) core.CompilationRequest {
	selection := input.Selection
	return core.CompilationRequest{
		DeliverableID: snapshot.DeliverableID, InputDigest: bundle.InputDigest, Generation: snapshot.ActiveGeneration + 1,
		Classification: snapshot.Classification, Configuration: options.Configuration, Resolutions: options.Resolutions, Registry: options.Registry,
		HostID: input.HostSession.HostID, HostSessionDigest: input.HostSession.Digest, HostEnvironmentReportDigest: input.Environment.Digest,
		HostProviderInventoryDigest: input.HostSession.ProviderInventoryDigest,
		HostTopologies:              append([]execution.Topology{}, input.HostSession.SupportedTopologies...),
		EnvironmentObservations:     append([]execution.EnvironmentObservation{}, input.Environment.Observations...), Selection: &selection,
	}
}

func switchMessageDigest(input SwitchInput) (string, error) {
	record := struct {
		SchemaVersion string      `json:"schema_version"`
		Kind          CommandKind `json:"kind"`
		Switch        SwitchInput `json:"switch"`
	}{WorkflowCommandSchemaV1, CommandSwitch, input}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return "", coordinatorError("WORKFLOW_COMMAND_INVALID", "digest SWITCH input", err)
	}
	return digest, nil
}
