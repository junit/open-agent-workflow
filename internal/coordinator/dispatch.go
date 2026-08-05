package coordinator

import (
	"path/filepath"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
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
		nextRevision := current.Revision + 1
		leases, err := prepareProjectLease(engine.options.PhysicalProjectRoot, current.Snapshot, grant, nextRevision)
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
	result.Receipts = append(result.Receipts[:0:0], value.Receipts...)
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
		Ticket: snapshot.ActiveTicket, Topology: bundle.Topology, HostSessionDigest: bundle.HostSessionDigest,
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

func prepareProjectLease(physicalRoot string, snapshot Snapshot, grant admission.CapabilityGrant, revision uint64) ([]ResourceLease, error) {
	leases := append([]ResourceLease{}, snapshot.ResourceLeases...)
	if !grantRequiresResourceLease(grant.Effects) {
		return leases, nil
	}
	if physicalRoot == "" || !filepath.IsAbs(physicalRoot) || filepath.Clean(physicalRoot) != physicalRoot {
		return nil, coordinatorError("RESOURCE_LEASE_REQUIRED", "write-capable PREPARE requires a physical project root", nil)
	}
	lease := ResourceLease{SchemaVersion: "oaw.resource-lease/v1", WorkflowID: snapshot.WorkflowID, GrantID: grant.ID, BundleID: grant.BundleID, BundleGeneration: grant.BundleGeneration, Resource: "project-worktree", PhysicalRoot: physicalRoot, AcquiredRevision: revision}
	seed := lease
	seed.ID, seed.Digest = "", ""
	digest, _, err := canonicaljson.Digest(seed)
	if err != nil {
		return nil, coordinatorError("RESOURCE_LEASE_INVALID", "digest project-worktree lease identity", err)
	}
	lease.ID = "lease-" + digest[:32]
	lease.Digest, _, err = canonicaljson.Digest(lease)
	if err != nil {
		return nil, coordinatorError("RESOURCE_LEASE_INVALID", "digest project-worktree lease", err)
	}
	leases = append(leases, lease)
	return leases, nil
}

func grantRequiresResourceLease(effects []string) bool {
	for _, effect := range effects {
		if effect == "write-project" || effect == "git-local" {
			return true
		}
	}
	return false
}
