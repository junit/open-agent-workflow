package coordinator

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
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
			SchemaVersion: WorkflowRevisionSchemaV1, WorkflowID: command.WorkflowID, Revision: nextRevision,
			PredecessorDigest: current.Digest, MessageID: command.MessageID, IdempotencyKey: command.IdempotencyKey,
			MessageDigest: messageDigest, Event: "WORKFLOW_RECEIPT_" + string(command.Receipt.Receipt.Kind), Snapshot: snapshot,
			Result: Result{SchemaVersion: WorkflowResultSchemaV1, Kind: ResultState, WorkflowID: command.WorkflowID, Revision: nextRevision, Diagnostics: diagnostics},
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
	}{WorkflowCommandSchemaV1, CommandReceipt, input}
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
		receipt.BundleGeneration != packet.BundleGeneration || receipt.BundleDigest != packet.BundleDigest ||
		receipt.NodeID != packet.NodeID || receipt.NodeID != snapshot.ActiveNodeID || receipt.Topology != packet.Topology ||
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
		if err := validateEvidenceClosure(packet.EvidenceRequirements, input.Receipt.Evidence); err != nil {
			return nil, err
		}
		return []Diagnostic{}, applyCompletedTransition(snapshot, graph, input.Signal)
	case host.ReceiptFailed:
		if input.Signal != "" {
			return nil, coordinatorError("WORKFLOW_RECEIPT_INVALID", "FAILED cannot declare a graph signal", nil)
		}
		return applyFailedTransition(snapshot, graph, input.Receipt.FailureCode), nil
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

func applyCompletedTransition(snapshot *Snapshot, graph profile.ExecutionGraphRecord, signal string) error {
	if containsString(graph.TerminalGates, snapshot.ActiveNodeID) {
		if signal != "" {
			return coordinatorError("WORKFLOW_SIGNAL_UNDECLARED", "terminal graph node does not accept a completion signal", nil)
		}
		snapshot.Status = StatusFinished
		snapshot.ActiveGrant = nil
		return nil
	}
	node, found := graphNode(graph, snapshot.ActiveNodeID)
	if !found {
		return coordinatorError("WORKFLOW_RECEIPT_INVALID", "active graph node is unavailable", nil)
	}
	target := ""
	for _, transition := range node.Transitions {
		if transition.Signal == signal {
			if target != "" {
				return coordinatorError("WORKFLOW_SIGNAL_UNDECLARED", "completion signal is ambiguous", nil)
			}
			target = transition.Target
		}
	}
	if target == "" {
		return coordinatorError("WORKFLOW_SIGNAL_UNDECLARED", "completion signal is not declared by the active graph node", nil)
	}
	if _, found := graphNode(graph, target); !found {
		return coordinatorError("WORKFLOW_RECEIPT_INVALID", "completion target is unavailable", nil)
	}
	snapshot.ActiveNodeID = target
	snapshot.Status = StatusReady
	snapshot.ActiveGrant = nil
	return nil
}

func applyFailedTransition(snapshot *Snapshot, graph profile.ExecutionGraphRecord, failureCode string) []Diagnostic {
	for _, route := range graph.IncidentRoutes {
		if route.Incident == failureCode {
			snapshot.ActiveNodeID = route.Handler
			snapshot.Status = StatusReady
			snapshot.ActiveGrant = nil
			return []Diagnostic{}
		}
	}
	snapshot.Status = StatusPaused
	snapshot.ActiveGrant = nil
	return []Diagnostic{{Code: "WORKFLOW_INCIDENT_UNROUTED", Detail: "Host reported an incident without a declared Bundle route"}}
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (engine *Engine) prepare(command Command) (Result, error) {
	if command.Prepare == nil {
		return Result{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "PREPARE input is required", nil)
	}
	messageDigest, err := prepareMessageDigest(*command.Prepare)
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
			return coordinatorError("WORKFLOW_REVISION_CONFLICT", "PREPARE expected revision does not match committed Workflow state", nil)
		}
		if current.Snapshot.Status != StatusReady || current.Snapshot.ActiveGrant != nil {
			return coordinatorError("WORKFLOW_PREPARE_INVALID", "PREPARE requires READY state without an active Grant", nil)
		}
		bundle, err := activeBundle(current.Snapshot)
		if err != nil {
			return err
		}
		node, found := graphNode(bundle.Graph, current.Snapshot.ActiveNodeID)
		if !found {
			return coordinatorError("WORKFLOW_PREPARE_INVALID", "active graph node is not present in the Bundle", nil)
		}
		grant, err := admission.IssueWorkflowGrant(admission.WorkflowGrantRequest{
			WorkflowID: current.WorkflowID, RequestID: current.Snapshot.RequestID, BundleID: bundle.ID,
			BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, Node: node, Topology: bundle.Topology,
			HostSessionDigest: bundle.HostSessionDigest, Effects: command.Prepare.RequestedEffects,
			Resources: command.Prepare.RequestedResources, TerminationCondition: command.Prepare.TerminationCondition,
			Authority: admission.CloneAuthority(engine.options.Authority),
		})
		if err != nil {
			code := admission.ErrorCode(err)
			if code == "" {
				code = "WORKFLOW_PREPARE_INVALID"
			}
			return coordinatorError(code, "PREPARE Grant admission failed", err)
		}
		commitPrepared := func() error {
			nextRevision := current.Revision + 1
			leases, err := engine.prepareProjectLease(current.Snapshot, grant, nextRevision)
			if err != nil {
				return err
			}
			packet, err := newDispatchPacket(current.Snapshot, bundle, node, grant, *command.Prepare)
			if err != nil {
				return err
			}
			snapshot := cloneSnapshot(current.Snapshot)
			snapshot.Revision = nextRevision
			snapshot.Status = StatusPrepared
			snapshot.ActiveGrant = &grant
			snapshot.GrantHistory = append(snapshot.GrantHistory, admission.CloneGrant(grant))
			snapshot.ResourceLeases = leases
			snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{IdempotencyKey: command.IdempotencyKey, ContentDigest: messageDigest, Revision: nextRevision})
			sort.Slice(snapshot.ProcessedMessages, func(left, right int) bool {
				return snapshot.ProcessedMessages[left].IdempotencyKey < snapshot.ProcessedMessages[right].IdempotencyKey
			})
			candidate := revisionRecord{
				SchemaVersion: WorkflowRevisionSchemaV1, WorkflowID: command.WorkflowID, Revision: nextRevision,
				PredecessorDigest: current.Digest, MessageID: command.MessageID, IdempotencyKey: command.IdempotencyKey,
				MessageDigest: messageDigest, Event: "WORKFLOW_PREPARED", Snapshot: snapshot,
				Result: Result{SchemaVersion: WorkflowResultSchemaV1, Kind: ResultDispatch, WorkflowID: command.WorkflowID, Revision: nextRevision, Dispatch: &packet, Diagnostics: []Diagnostic{}},
			}
			committed, err := engine.journal.commit(candidate)
			if err != nil {
				return err
			}
			result = committed.Result
			return nil
		}
		if grantRequiresResourceLease(grant.Effects) {
			return engine.journal.withResourceLeaseLock(commitPrepared)
		}
		return commitPrepared()
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func prepareMessageDigest(input PrepareInput) (string, error) {
	record := struct {
		SchemaVersion string       `json:"schema_version"`
		Kind          CommandKind  `json:"kind"`
		Prepare       PrepareInput `json:"prepare"`
	}{WorkflowCommandSchemaV1, CommandPrepare, input}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return "", coordinatorError("WORKFLOW_COMMAND_INVALID", "digest PREPARE input", err)
	}
	return digest, nil
}

func activeBundle(snapshot Snapshot) (core.LifecycleBundle, error) {
	for _, bundle := range snapshot.Bundles {
		if bundle.Generation == snapshot.ActiveGeneration {
			return bundle, nil
		}
	}
	return core.LifecycleBundle{}, coordinatorError("WORKFLOW_PREPARE_INVALID", "active Bundle generation is not present", nil)
}

func cloneSnapshot(value Snapshot) Snapshot {
	result := value
	result.Bundles = append([]core.LifecycleBundle{}, value.Bundles...)
	result.GrantHistory = append([]admission.CapabilityGrant{}, value.GrantHistory...)
	result.Receipts = make([]host.InvocationReceipt, len(value.Receipts))
	for index, receipt := range value.Receipts {
		result.Receipts[index] = host.CloneInvocationReceipt(receipt)
	}
	result.ResourceLeases = append([]ResourceLease{}, value.ResourceLeases...)
	result.ProcessedMessages = append([]ProcessedMessage{}, value.ProcessedMessages...)
	result.ProjectionLag = append([]ProjectionLag{}, value.ProjectionLag...)
	if value.ActiveGrant != nil {
		grant := admission.CloneGrant(*value.ActiveGrant)
		result.ActiveGrant = &grant
	}
	return result
}

func newDispatchPacket(snapshot Snapshot, bundle core.LifecycleBundle, node profile.GraphNode, grant admission.CapabilityGrant, input PrepareInput) (DispatchPacket, error) {
	packet := DispatchPacket{
		SchemaVersion: DispatchPacketSchemaV1, WorkflowID: snapshot.WorkflowID, RequestID: snapshot.RequestID,
		BundleID: bundle.ID, BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, NodeID: node.ID,
		Ticket: snapshot.ActiveTicket, Topology: bundle.Topology, HostSessionDigest: bundle.HostSessionDigest, EnvironmentReportDigest: bundle.EnvironmentReportDigest,
		Grant: admission.CloneGrant(grant), InputReferences: append([]ArtifactReference{}, input.InputReferences...),
		EvidenceRequirements: append([]EvidenceRequirement{}, input.EvidenceRequirements...), EnvironmentRequirements: append([]execution.EnvironmentRequirement{}, bundle.EnvironmentRequirements...),
	}
	sort.Slice(packet.InputReferences, func(left, right int) bool {
		return artifactReferenceKey(packet.InputReferences[left]) < artifactReferenceKey(packet.InputReferences[right])
	})
	sort.Slice(packet.EvidenceRequirements, func(left, right int) bool {
		return evidenceRequirementKey(packet.EvidenceRequirements[left]) < evidenceRequirementKey(packet.EvidenceRequirements[right])
	})
	seed := packet
	seed.ID, seed.Digest = "", ""
	digest, _, err := canonicaljson.Digest(seed)
	if err != nil {
		return DispatchPacket{}, coordinatorError("WORKFLOW_DISPATCH_INVALID", "digest Dispatch Packet identity", err)
	}
	packet.ID = "dispatch-" + digest[:32]
	packet.Digest, _, err = canonicaljson.Digest(packet)
	if err != nil {
		return DispatchPacket{}, coordinatorError("WORKFLOW_DISPATCH_INVALID", "digest Dispatch Packet", err)
	}
	return packet, nil
}

func grantRequiresResourceLease(effects []string) bool {
	for _, effect := range effects {
		if effect == "write-project" || effect == "git-local" {
			return true
		}
	}
	return false
}
