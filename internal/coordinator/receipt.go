package coordinator

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func (engine *Engine) receipt(command Command) (Result, error) {
	if command.Receipt == nil {
		return Result{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "RECEIPT input is required", nil)
	}
	messageDigest, err := receiptMessageDigest(*command.Receipt)
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
			return coordinatorError("WORKFLOW_REVISION_CONFLICT", "RECEIPT expected revision does not match committed Workflow state", nil)
		}
		packet, packetErr := engine.activeDispatch(current)
		if packetErr != nil {
			return packetErr
		}
		snapshot, diagnostics, transitionErr := transitionReceipt(current.Snapshot, packet, *command.Receipt)
		if transitionErr != nil {
			return transitionErr
		}
		nextRevision := current.Revision + 1
		snapshot.Revision = nextRevision
		if snapshot.ActiveGrant == nil {
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
			MessageDigest: messageDigest, Event: "WORKFLOW_RECEIPT_" + string(command.Receipt.Receipt.Kind), Snapshot: snapshot,
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

func receiptMessageDigest(input ReceiptInput) (string, error) {
	record := struct {
		SchemaVersion string       `json:"schema_version"`
		Kind          CommandKind  `json:"kind"`
		Receipt       ReceiptInput `json:"receipt"`
	}{WorkflowCommandSchemaV2, CommandReceipt, input}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return "", coordinatorError("WORKFLOW_COMMAND_INVALID", "digest RECEIPT input", err)
	}
	return digest, nil
}

func (engine *Engine) activeDispatch(current revisionRecord) (DispatchPacket, error) {
	if current.Snapshot.ActiveGrant == nil ||
		(current.Snapshot.Status != StatusPrepared && current.Snapshot.Status != StatusInFlight && current.Snapshot.Status != StatusPaused) {
		return DispatchPacket{}, coordinatorError("WORKFLOW_RECEIPT_INVALID", "Workflow has no active invocation for RECEIPT", nil)
	}
	for revision := current.Revision; revision > 0; revision-- {
		record, err := engine.journal.loadRevision(current.WorkflowID, revision)
		if err != nil {
			return DispatchPacket{}, err
		}
		if record.Result.Dispatch != nil && sameCanonicalValue(record.Result.Dispatch.Grant, *current.Snapshot.ActiveGrant) {
			return *record.Result.Dispatch, nil
		}
	}
	return DispatchPacket{}, coordinatorError("WORKFLOW_RECEIPT_INVALID", "active Grant has no committed Dispatch Packet", nil)
}

func transitionReceipt(snapshot Snapshot, packet DispatchPacket, input ReceiptInput) (Snapshot, []Diagnostic, error) {
	receipt := input.Receipt
	if err := validateReceiptPins(snapshot, packet, receipt); err != nil {
		return Snapshot{}, nil, err
	}
	if err := validateReceiptSequence(snapshot, receipt); err != nil {
		return Snapshot{}, nil, err
	}
	bundle, err := activeBundle(snapshot)
	if err != nil {
		return Snapshot{}, nil, coordinatorError("WORKFLOW_RECEIPT_INVALID", "active Bundle is unavailable", err)
	}
	next := cloneSnapshot(snapshot)
	next.Receipts = append(next.Receipts, host.CloneInvocationReceipt(receipt))
	diagnostics, err := applyReceiptTransition(&next, bundle.Graph, packet, input)
	if err != nil {
		return Snapshot{}, nil, err
	}
	if input.StableBoundary != "" {
		if !containsString(bundle.Graph.StableBoundaries, input.StableBoundary) {
			return Snapshot{}, nil, coordinatorError("WORKFLOW_RECEIPT_INVALID", "Receipt stable boundary is not declared by the active Bundle", nil)
		}
		next.LastStableBoundary = input.StableBoundary
	}
	return next, diagnostics, nil
}

func validateReceiptPins(snapshot Snapshot, packet DispatchPacket, receipt host.InvocationReceipt) error {
	if snapshot.ActiveGrant == nil || !sameCanonicalValue(packet.Grant, *snapshot.ActiveGrant) ||
		receipt.WorkflowID != snapshot.WorkflowID || receipt.WorkflowID != packet.WorkflowID ||
		receipt.BundleID != packet.BundleID ||
		receipt.BundleGeneration != packet.BundleGeneration || receipt.BundleDigest != packet.BundleDigest ||
		receipt.Cursor != packet.Cursor || receipt.Cursor != snapshot.Cursor || receipt.Topology != packet.Topology ||
		receipt.HostSessionDigest != packet.HostSessionDigest || receipt.DispatchDigest != packet.Digest ||
		receipt.EnvironmentReportDigest != packet.EnvironmentReportDigest {
		return coordinatorError("WORKFLOW_RECEIPT_INVALID", "Receipt does not match the active Dispatch Packet", nil)
	}
	return nil
}

func validateReceiptSequence(snapshot Snapshot, receipt host.InvocationReceipt) error {
	for _, existing := range snapshot.Receipts {
		if existing.Digest == receipt.Digest {
			return coordinatorError("WORKFLOW_RECEIPT_INVALID", "Receipt was already committed under another message", nil)
		}
	}
	switch receipt.Kind {
	case host.ReceiptStarted:
		if snapshot.Status != StatusPrepared {
			return coordinatorError("WORKFLOW_RECEIPT_INVALID", "STARTED requires PREPARED state", nil)
		}
	case host.ReceiptCompleted, host.ReceiptFailed, host.ReceiptPaused:
		if snapshot.Status != StatusInFlight {
			return coordinatorError("WORKFLOW_RECEIPT_INVALID", "Host outcome requires IN_FLIGHT state", nil)
		}
	case host.ReceiptCancelled:
		if snapshot.Status != StatusInFlight && snapshot.Status != StatusPaused {
			return coordinatorError("WORKFLOW_RECEIPT_INVALID", "CANCELLED requires an active invocation", nil)
		}
	default:
		return coordinatorError("WORKFLOW_RECEIPT_INVALID", "unsupported Host Receipt kind", nil)
	}
	if receipt.Kind != host.ReceiptStarted && receipt.Topology == execution.TopologySubagent {
		startedHandle := ""
		for _, existing := range snapshot.Receipts {
			if existing.Kind == host.ReceiptStarted && existing.DispatchDigest == receipt.DispatchDigest {
				startedHandle = existing.InvocationHandle
				break
			}
		}
		if startedHandle == "" || receipt.InvocationHandle != startedHandle {
			return coordinatorError("WORKFLOW_RECEIPT_INVALID", "SUBAGENT Receipt changed the STARTED invocation handle", nil)
		}
	}
	return nil
}

func applyReceiptTransition(snapshot *Snapshot, graph profile.ExecutionGraphRecord, packet DispatchPacket, input ReceiptInput) ([]Diagnostic, error) {
	switch input.Receipt.Kind {
	case host.ReceiptStarted:
		if input.Signal != "" || input.StableBoundary != "" {
			return nil, coordinatorError("WORKFLOW_RECEIPT_INVALID", "STARTED cannot declare a signal or stable boundary", nil)
		}
		snapshot.Status = StatusInFlight
	case host.ReceiptCompleted:
		if err := validateOutputClosure(packet, input.Receipt.Outputs); err != nil {
			return nil, err
		}
		if err := validateEvidenceClosure(packet.EvidenceRequirements, input.Receipt.Evidence); err != nil {
			return nil, err
		}
		return applyCompletedTransition(snapshot, graph, input.Signal)
	case host.ReceiptFailed:
		if input.Signal != "" {
			return nil, coordinatorError("WORKFLOW_RECEIPT_INVALID", "FAILED cannot declare a graph signal", nil)
		}
		return applyFailedTransition(snapshot, graph, input.Receipt.FailureCode)
	case host.ReceiptPaused:
		if input.Signal != "" {
			return nil, coordinatorError("WORKFLOW_RECEIPT_INVALID", "PAUSED cannot declare a graph signal", nil)
		}
		snapshot.Status = StatusPaused
	case host.ReceiptCancelled:
		if input.Signal != "" || input.StableBoundary != "" {
			return nil, coordinatorError("WORKFLOW_RECEIPT_INVALID", "CANCELLED cannot declare a signal or stable boundary", nil)
		}
		snapshot.Status = StatusCancelled
		snapshot.ActiveGrant = nil
	}
	return []Diagnostic{}, nil
}

func applyCompletedTransition(snapshot *Snapshot, graph profile.ExecutionGraphRecord, signal string) ([]Diagnostic, error) {
	transition, err := profile.NextActionableCursor(graph, snapshot.Cursor, signal, "")
	if err != nil {
		return nil, coordinatorError("WORKFLOW_SIGNAL_UNDECLARED", "completion transition is not declared", err)
	}
	return applyTraversalResult(snapshot, transition)
}

func applyFailedTransition(snapshot *Snapshot, graph profile.ExecutionGraphRecord, failureCode string) ([]Diagnostic, error) {
	transition, err := profile.NextActionableCursor(graph, snapshot.Cursor, "", failureCode)
	if err != nil {
		snapshot.Status = StatusPaused
		snapshot.ActiveGrant = nil
		return []Diagnostic{{Code: "WORKFLOW_INCIDENT_UNROUTED", Detail: "Host reported an incident without a declared Bundle route"}}, nil
	}
	return applyTraversalResult(snapshot, transition)
}

func applyTraversalResult(snapshot *Snapshot, transition profile.TraversalResult) ([]Diagnostic, error) {
	snapshot.ActiveGrant = nil
	switch transition.Disposition {
	case profile.TraversalNext:
		if transition.Cursor == nil {
			return nil, coordinatorError("WORKFLOW_TRAVERSAL_INVALID", "next traversal result omitted cursor", nil)
		}
		snapshot.Cursor = *transition.Cursor
		snapshot.Status = StatusReady
	case profile.TraversalTerminal:
		if transition.Cursor != nil {
			return nil, coordinatorError("WORKFLOW_TRAVERSAL_INVALID", "terminal traversal result carried a cursor", nil)
		}
		snapshot.Status = StatusFinished
	case profile.TraversalStop:
		if transition.Cursor != nil {
			return nil, coordinatorError("WORKFLOW_TRAVERSAL_INVALID", "stop traversal result carried a cursor", nil)
		}
		snapshot.Status = StatusPaused
		return []Diagnostic{{Code: "WORKFLOW_TRAVERSAL_STOPPED", Detail: "profile traversal requested stop"}}, nil
	case profile.TraversalReplan:
		if transition.Cursor != nil {
			return nil, coordinatorError("WORKFLOW_TRAVERSAL_INVALID", "replan traversal result carried a cursor", nil)
		}
		snapshot.Status = StatusPaused
		return []Diagnostic{{Code: "WORKFLOW_TRAVERSAL_REPLAN_REQUIRED", Detail: "profile traversal requested replan"}}, nil
	default:
		return nil, coordinatorError("WORKFLOW_TRAVERSAL_INVALID", "unknown traversal disposition", nil)
	}
	return []Diagnostic{}, nil
}

func validateEvidenceClosure(requirements []EvidenceRequirement, evidence []host.EvidenceReference) error {
	counts := make(map[string]uint64, len(evidence))
	for _, reference := range evidence {
		counts[reference.Kind]++
	}
	for _, requirement := range requirements {
		if counts[requirement.Kind] < requirement.Minimum {
			return coordinatorError("WORKFLOW_EVIDENCE_INCOMPLETE", "COMPLETED Receipt does not satisfy Dispatch Packet evidence requirements", nil)
		}
	}
	return nil
}

func validateOutputClosure(packet DispatchPacket, outputs []host.OutputReference) error {
	if len(outputs) != 1 {
		return coordinatorError("WORKFLOW_OUTPUT_INVALID", "COMPLETED Receipt must contain exactly one output for the active Dispatch Packet", nil)
	}
	output := outputs[0]
	var expectedArtifact, expectedSchema string
	switch packet.Grant.Target.TargetKind {
	case admission.GrantProviderBinding:
		if packet.Grant.Target.ProviderBinding == nil {
			return coordinatorError("WORKFLOW_OUTPUT_INVALID", "Dispatch Packet provider target is missing", nil)
		}
		expectedArtifact = packet.Grant.Target.ProviderBinding.OutputArtifact
		expectedSchema = packet.Grant.Target.ProviderBinding.OutcomeSchema
	case admission.GrantHostAction:
		if packet.Grant.Target.HostAction == nil {
			return coordinatorError("WORKFLOW_OUTPUT_INVALID", "Dispatch Packet Host action target is missing", nil)
		}
		expectedArtifact = packet.Grant.Target.HostAction.OutputArtifact
		expectedSchema = packet.Grant.Target.HostAction.OutcomeSchema
	default:
		return coordinatorError("WORKFLOW_OUTPUT_INVALID", "Dispatch Packet target kind is unsupported", nil)
	}
	if output.ArtifactID != expectedArtifact || output.Schema != expectedSchema || !validText(output.Reference, 2048) || !validDigest(output.Digest) {
		return coordinatorError("WORKFLOW_OUTPUT_INVALID", "COMPLETED Receipt output does not match the Dispatch Packet target", nil)
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
