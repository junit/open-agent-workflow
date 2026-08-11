package coordinator

import (
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

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
		unit, unitErr := profile.UnitAtCursor(bundle.Graph, current.Snapshot.Cursor)
		if unitErr != nil {
			return coordinatorError("WORKFLOW_PREPARE_INVALID", "active graph cursor is not present in the Bundle", unitErr)
		}
		if unit.Gate != nil {
			gateResult, gateErr := engine.prepareGate(current, command, bundle, unit, messageDigest)
			if gateErr == nil {
				result = gateResult
			}
			return gateErr
		}
		if unit.ProviderBinding == nil && unit.HostAction == nil {
			return coordinatorError("WORKFLOW_PREPARE_INVALID", "active graph cursor is not dispatchable", nil)
		}
		var binding *profile.ResolvedBinding
		var action *profile.CompiledHostAction
		var capability *catalog.CapabilityRecord
		if unit.ProviderBinding != nil {
			binding = unit.ProviderBinding
			capability = engine.capabilityForBinding(*binding)
		}
		if unit.HostAction != nil {
			action = unit.HostAction
		}
		if binding != nil && capability == nil {
			return coordinatorError("WORKFLOW_PREPARE_INVALID", "active Binding capability is not present in Registry", nil)
		}
		grant, err := admission.IssueWorkflowGrant(admission.WorkflowGrantRequest{
			WorkflowID: current.WorkflowID, RequestID: current.Snapshot.RequestID, BundleID: bundle.ID,
			BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, Cursor: current.Snapshot.Cursor, ProviderBinding: binding,
			Capability: capability, HostAction: action, Topology: bundle.Topology, HostID: bundle.HostID,
			HostSessionDigest: bundle.HostSessionDigest, Effects: command.Prepare.RequestedEffects,
			Resources: command.Prepare.RequestedResources, TerminationCondition: command.Prepare.TerminationCondition,
			Authorization: command.Prepare.Authorization, InvocationAttestation: command.Prepare.InvocationAttestation,
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
			packet, err := newDispatchPacket(current.Snapshot, bundle, current.Snapshot.Cursor, grant, *command.Prepare)
			if err != nil {
				return err
			}
			snapshot := cloneSnapshot(current.Snapshot)
			snapshot.Revision = nextRevision
			snapshot.Status = StatusPrepared
			snapshot.ActiveGrant = &grant
			snapshot.GrantHistory = append(snapshot.GrantHistory, admission.CloneGrant(grant))
			if command.Prepare.Authorization != nil {
				snapshot.UserAuthorizations = append(snapshot.UserAuthorizations, admission.CloneUserAuthorization(*command.Prepare.Authorization))
			}
			if command.Prepare.InvocationAttestation != nil {
				snapshot.InvocationAttestations = append(snapshot.InvocationAttestations, admission.CloneExplicitInvocationAttestation(*command.Prepare.InvocationAttestation))
			}
			snapshot.ResourceLeases = leases
			snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{IdempotencyKey: command.IdempotencyKey, ContentDigest: messageDigest, Revision: nextRevision})
			sort.Slice(snapshot.ProcessedMessages, func(left, right int) bool {
				return snapshot.ProcessedMessages[left].IdempotencyKey < snapshot.ProcessedMessages[right].IdempotencyKey
			})
			candidate := revisionRecord{
				SchemaVersion: WorkflowRevisionSchemaV2, WorkflowID: command.WorkflowID, Revision: nextRevision,
				PredecessorDigest: current.Digest, MessageID: command.MessageID, IdempotencyKey: command.IdempotencyKey,
				MessageDigest: messageDigest, Event: "WORKFLOW_PREPARED", Snapshot: snapshot,
				Result: Result{SchemaVersion: WorkflowResultSchemaV2, Kind: ResultDispatch, WorkflowID: command.WorkflowID, Revision: nextRevision, Dispatch: &packet, Diagnostics: []Diagnostic{}},
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
	}{WorkflowCommandSchemaV2, CommandPrepare, input}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return "", coordinatorError("WORKFLOW_COMMAND_INVALID", "digest PREPARE input", err)
	}
	return digest, nil
}

func activeBundle(snapshot Snapshot) (core.LifecycleBundle, error) {
	bundle, err := bundleForGeneration(snapshot, snapshot.ActiveGeneration)
	if err != nil {
		return core.LifecycleBundle{}, coordinatorError("WORKFLOW_PREPARE_INVALID", "active Bundle generation is not present exactly once", err)
	}
	return bundle, nil
}

func cloneSnapshot(value Snapshot) Snapshot {
	result := value
	result.Bundles = append([]core.LifecycleBundle{}, value.Bundles...)
	result.GrantHistory = append([]admission.CapabilityGrant{}, value.GrantHistory...)
	result.UserAuthorizations = make([]admission.UserAuthorization, len(value.UserAuthorizations))
	for index, authorization := range value.UserAuthorizations {
		result.UserAuthorizations[index] = admission.CloneUserAuthorization(authorization)
	}
	result.InvocationAttestations = make([]admission.ExplicitInvocationAttestation, len(value.InvocationAttestations))
	for index, attestation := range value.InvocationAttestations {
		result.InvocationAttestations[index] = admission.CloneExplicitInvocationAttestation(attestation)
	}
	result.GateAttestations = make([]GateAttestation, len(value.GateAttestations))
	for index, attestation := range value.GateAttestations {
		result.GateAttestations[index] = cloneGateAttestation(attestation)
	}
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

func newDispatchPacket(snapshot Snapshot, bundle core.LifecycleBundle, cursor execution.GraphCursor, grant admission.CapabilityGrant, input PrepareInput) (DispatchPacket, error) {
	packet := DispatchPacket{
		SchemaVersion: DispatchPacketSchemaV2, WorkflowID: snapshot.WorkflowID, RequestID: snapshot.RequestID,
		BundleID: bundle.ID, BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest, Cursor: cursor, TargetKind: grant.Target.TargetKind,
		Ticket: snapshot.ActiveTicket, Topology: bundle.Topology, HostSessionDigest: bundle.HostSessionDigest, EnvironmentReportDigest: bundle.EnvironmentReportDigest,
		Grant: admission.CloneGrant(grant), InputReferences: append([]ArtifactReference{}, input.InputReferences...),
		EvidenceRequirements: append([]EvidenceRequirement{}, input.EvidenceRequirements...), EnvironmentRequirements: append([]execution.EnvironmentRequirement{}, bundle.EnvironmentRequirements...),
		Authorization: cloneOptionalAuthorization(input.Authorization), InvocationAttestation: cloneOptionalInvocation(input.InvocationAttestation),
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

func cloneOptionalAuthorization(value *admission.UserAuthorization) *admission.UserAuthorization {
	if value == nil {
		return nil
	}
	clone := admission.CloneUserAuthorization(*value)
	return &clone
}

func cloneOptionalInvocation(value *admission.ExplicitInvocationAttestation) *admission.ExplicitInvocationAttestation {
	if value == nil {
		return nil
	}
	clone := admission.CloneExplicitInvocationAttestation(*value)
	return &clone
}

func (engine *Engine) capabilityForBinding(binding profile.ResolvedBinding) *catalog.CapabilityRecord {
	if engine.options.capabilityResolver != nil {
		return engine.options.capabilityResolver(binding)
	}
	verifiedBinding, found := engine.options.Registry.Binding(binding.ProviderID, binding.BindingID)
	if !found || verifiedBinding.DistributionID != binding.DistributionID || verifiedBinding.DistributionRevision != binding.DistributionRevision ||
		verifiedBinding.DistributionTreeDigest != binding.DistributionTreeDigest || verifiedBinding.Surface != binding.Surface || verifiedBinding.Kind != binding.Kind ||
		verifiedBinding.Reference != binding.Reference || verifiedBinding.Invocation != binding.Invocation || verifiedBinding.BindingTreeDigest != binding.BindingTreeDigest ||
		verifiedBinding.BindingEvidenceDigest != binding.BindingEvidenceDigest {
		return nil
	}
	var matched *catalog.CapabilityRecord
	for _, provider := range engine.options.Configuration.Catalog().Providers() {
		if provider.ID != binding.ProviderID {
			continue
		}
		for _, capability := range provider.Capabilities {
			verified, capabilityFound := engine.options.Registry.Capability(binding.ProviderID, capability.ID)
			if !capabilityFound || !slices.Contains(verified.BindingIDs, binding.BindingID) || !slices.Contains(capability.BindingRefs, binding.BindingID) ||
				!slices.Contains(capability.RequestModes, catalog.RequestModeWorkflow) {
				continue
			}
			if matched != nil && (matched.InputSchema != capability.InputSchema || matched.OutcomeSchema != capability.OutcomeSchema) {
				return nil
			}
			value := capability
			matched = &value
		}
	}
	return matched
}

func (engine *Engine) prepareGate(current revisionRecord, command Command, bundle core.LifecycleBundle, unit profile.TraversalUnit, messageDigest string) (Result, error) {
	attestation := command.Prepare.GateAttestation
	if attestation == nil || attestation.WorkflowID != current.WorkflowID || attestation.BundleID != bundle.ID ||
		attestation.BundleGeneration != bundle.Generation || attestation.BundleDigest != bundle.Digest || attestation.Cursor != current.Snapshot.Cursor ||
		attestation.GateID != unit.Gate.ID || attestation.Authority != unit.Gate.Authority {
		return Result{}, coordinatorError("GATE_ATTESTATION_INVALID", "PREPARE gate attestation does not exactly match active gate", nil)
	}
	if attestation.Decision != GateSatisfied && attestation.Decision != GateRejected {
		return Result{}, coordinatorError("GATE_ATTESTATION_INVALID", "PREPARE gate attestation has an unsupported decision", nil)
	}
	if err := validateGateEvidenceClosure(unit.Gate.EvidenceRequirements, attestation.Evidence); err != nil {
		return Result{}, err
	}
	snapshot := cloneSnapshot(current.Snapshot)
	snapshot.Revision = current.Revision + 1
	snapshot.GateAttestations = append(snapshot.GateAttestations, cloneGateAttestation(*attestation))
	snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{IdempotencyKey: command.IdempotencyKey, ContentDigest: messageDigest, Revision: snapshot.Revision})
	sort.Slice(snapshot.ProcessedMessages, func(left, right int) bool {
		return snapshot.ProcessedMessages[left].IdempotencyKey < snapshot.ProcessedMessages[right].IdempotencyKey
	})
	diagnostics := []Diagnostic{}
	if attestation.Decision == GateRejected {
		snapshot.Status = StatusPaused
		diagnostics = append(diagnostics, Diagnostic{Code: "WORKFLOW_GATE_REJECTED", Detail: "gate attestation rejected the active gate"})
	} else {
		signal := "succeeded"
		for _, slot := range bundle.Graph.Slots {
			if string(slot.SlotID) == current.Snapshot.Cursor.SlotID && slot.Terminal {
				signal = ""
				break
			}
		}
		next, err := profile.NextActionableCursor(bundle.Graph, current.Snapshot.Cursor, signal, "")
		if err != nil {
			return Result{}, coordinatorError("WORKFLOW_PREPARE_INVALID", "gate transition is invalid", err)
		}
		if next.Disposition == profile.TraversalNext && next.Cursor != nil && next.Cursor.Kind == execution.CursorTerminal {
			next, err = profile.NextActionableCursor(bundle.Graph, *next.Cursor, "", "")
			if err != nil {
				return Result{}, coordinatorError("WORKFLOW_PREPARE_INVALID", "terminal gate transition is invalid", err)
			}
		}
		transitionDiagnostics, transitionErr := applyTraversalResult(&snapshot, next)
		if transitionErr != nil {
			return Result{}, transitionErr
		}
		diagnostics = append(diagnostics, transitionDiagnostics...)
	}
	candidate := revisionRecord{SchemaVersion: WorkflowRevisionSchemaV2, WorkflowID: command.WorkflowID, Revision: snapshot.Revision,
		PredecessorDigest: current.Digest, MessageID: command.MessageID, IdempotencyKey: command.IdempotencyKey, MessageDigest: messageDigest,
		Event: "WORKFLOW_GATE_ATTESTED", Snapshot: snapshot,
		Result: Result{SchemaVersion: WorkflowResultSchemaV2, Kind: ResultState, WorkflowID: command.WorkflowID, Revision: snapshot.Revision, Diagnostics: diagnostics}}
	committed, err := engine.journal.commit(candidate)
	if err != nil {
		return Result{}, err
	}
	return committed.Result, nil
}

func validateGateEvidenceClosure(requirements []catalog.EvidenceRequirementRecord, evidence []host.EvidenceReference) error {
	counts := make(map[string]uint64, len(evidence))
	for _, reference := range evidence {
		counts[reference.Kind]++
	}
	for _, requirement := range requirements {
		if counts[requirement.Kind] < requirement.Minimum {
			return coordinatorError("GATE_EVIDENCE_INCOMPLETE", "Gate Attestation does not satisfy the active gate evidence requirements", nil)
		}
	}
	return nil
}

func grantRequiresResourceLease(effects []string) bool {
	for _, effect := range effects {
		if effect == "write-project" || effect == "git-local" {
			return true
		}
	}
	return false
}
