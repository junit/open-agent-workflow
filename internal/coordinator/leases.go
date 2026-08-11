package coordinator

import (
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const resourceLeaseProjectWorktree = "project-worktree"

func (value *journal) withResourceLeaseLock(action func() error) error {
	if value == nil || action == nil {
		return coordinatorError("RESOURCE_LEASE_INVALID", "invalid Resource Lease lock request", nil)
	}
	path := filepath.Join(value.stateRoot, "RESOURCE_LEASES.LOCK")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "create Resource Lease lock", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "protect Resource Lease lock", err)
	}
	if err := file.Close(); err != nil {
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "close Resource Lease lock", err)
	}
	guard := flock.New(path)
	if err := guard.Lock(); err != nil {
		return coordinatorError("WORKFLOW_STATE_WRITE_FAILED", "lock Resource Leases", err)
	}
	defer func() {
		_ = guard.Unlock()
		_ = guard.Close()
	}()
	return action()
}

func canonicalPhysicalRoot(root string) (string, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", coordinatorError("RESOURCE_LEASE_INVALID", "physical project root must be a clean absolute path", nil)
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", coordinatorError("RESOURCE_LEASE_INVALID", "resolve physical project root", err)
	}
	info, err := os.Stat(physical)
	if err != nil || !info.IsDir() {
		return "", coordinatorError("RESOURCE_LEASE_INVALID", "physical project root must be an existing directory", err)
	}
	return filepath.Clean(physical), nil
}

func (engine *Engine) prepareProjectLease(snapshot Snapshot, grant admission.CapabilityGrant, revision uint64) ([]ResourceLease, error) {
	leases := append([]ResourceLease{}, snapshot.ResourceLeases...)
	if !grantRequiresResourceLease(grant.Effects) {
		return leases, nil
	}
	physicalRoot, err := canonicalPhysicalRoot(engine.options.PhysicalProjectRoot)
	if err != nil {
		return nil, err
	}
	conflict, err := engine.journal.findActiveResourceLease(snapshot.WorkflowID, physicalRoot)
	if err != nil {
		return nil, err
	}
	if conflict != (ResourceLease{}) {
		return nil, coordinatorError("RESOURCE_LEASE_CONFLICT", "physical project Worktree is already leased", nil)
	}
	lease := ResourceLease{
		SchemaVersion: "oaw.resource-lease/v1", WorkflowID: snapshot.WorkflowID, GrantID: grant.ID,
		BundleID: grant.BundleID, BundleGeneration: grant.BundleGeneration, Cursor: grant.Cursor, Resource: resourceLeaseProjectWorktree,
		PhysicalRoot: physicalRoot, AcquiredRevision: revision,
	}
	seed := lease
	seed.ID, seed.Digest = "", ""
	identityDigest, _, err := canonicaljson.Digest(seed)
	if err != nil {
		return nil, coordinatorError("RESOURCE_LEASE_INVALID", "digest Resource Lease identity", err)
	}
	lease.ID = "lease-" + identityDigest[:32]
	if err := sealResourceLease(&lease); err != nil {
		return nil, err
	}
	return append(leases, lease), nil
}

func (value *journal) findActiveResourceLease(excludeWorkflowID, physicalRoot string) (ResourceLease, error) {
	entries, err := os.ReadDir(filepath.Join(value.stateRoot, workflowRecordsDirectory))
	if err != nil {
		return ResourceLease{}, coordinatorError("WORKFLOW_STATE_READ_FAILED", "scan Workflows for Resource Leases", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == excludeWorkflowID || !validWorkflowID(entry.Name()) {
			continue
		}
		record, loadErr := value.loadCommitted(entry.Name())
		if ErrorCode(loadErr) == "WORKFLOW_NOT_FOUND" {
			continue
		}
		if loadErr != nil {
			return ResourceLease{}, coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "validate Workflow during Resource Lease scan", loadErr)
		}
		for _, lease := range record.Snapshot.ResourceLeases {
			if lease.ReleasedRevision == 0 && lease.Resource == resourceLeaseProjectWorktree && lease.PhysicalRoot == physicalRoot {
				return lease, nil
			}
		}
	}
	return ResourceLease{}, nil
}

func releaseResourceLeases(snapshot *Snapshot, revision uint64) error {
	if snapshot == nil {
		return coordinatorError("RESOURCE_LEASE_INVALID", "Workflow snapshot is required", nil)
	}
	for index := range snapshot.ResourceLeases {
		if snapshot.ResourceLeases[index].ReleasedRevision != 0 {
			continue
		}
		snapshot.ResourceLeases[index].ReleasedRevision = revision
		if err := sealResourceLease(&snapshot.ResourceLeases[index]); err != nil {
			return err
		}
	}
	return nil
}

func sealResourceLease(value *ResourceLease) error {
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(*value)
	if err != nil {
		return coordinatorError("RESOURCE_LEASE_INVALID", "digest Resource Lease", err)
	}
	value.Digest = digest
	return nil
}

func validateResourceLeases(snapshot Snapshot, revision uint64) error {
	grants := make(map[string]admission.CapabilityGrant, len(snapshot.GrantHistory))
	for _, grant := range snapshot.GrantHistory {
		grants[grant.ID] = grant
	}
	activeCount := 0
	seen := make(map[string]struct{}, len(snapshot.ResourceLeases))
	for _, lease := range snapshot.ResourceLeases {
		if err := validateResourceLease(lease, snapshot.WorkflowID, revision); err != nil {
			return err
		}
		if _, found := seen[lease.ID]; found {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "duplicate Resource Lease history", nil)
		}
		seen[lease.ID] = struct{}{}
		grant, found := grants[lease.GrantID]
		if !found || grant.BundleID != lease.BundleID || grant.BundleGeneration != lease.BundleGeneration || grant.Cursor != lease.Cursor || !grantRequiresResourceLease(grant.Effects) {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "Resource Lease is not bound to a write Grant", nil)
		}
		if lease.ReleasedRevision == 0 {
			activeCount++
			if snapshot.ActiveGrant == nil || snapshot.ActiveGrant.ID != lease.GrantID || snapshot.ActiveGrant.Cursor != lease.Cursor {
				return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "active Resource Lease is not bound to the active Grant", nil)
			}
		}
	}
	if activeCount > 1 || activeCount == 1 && snapshot.Status != StatusPrepared && snapshot.Status != StatusInFlight && snapshot.Status != StatusPaused {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "active Resource Lease has no resumable Workflow invocation", nil)
	}
	if snapshot.ActiveGrant != nil && grantRequiresResourceLease(snapshot.ActiveGrant.Effects) && activeCount != 1 {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "write-capable active Grant has no Resource Lease", nil)
	}
	return nil
}

func validateResourceLease(value ResourceLease, workflowID string, revision uint64) error {
	if value.SchemaVersion != "oaw.resource-lease/v1" || !validStableID("lease-", value.ID) || value.WorkflowID != workflowID ||
		!validStableID("grant-", value.GrantID) || !validText(value.BundleID, 512) || value.BundleGeneration == 0 ||
		execution.ValidateGraphCursor(value.Cursor) != nil || value.Cursor.Kind == execution.CursorGate || value.Cursor.Kind == execution.CursorTerminal || value.Resource != resourceLeaseProjectWorktree || value.AcquiredRevision == 0 || value.AcquiredRevision > revision ||
		value.ReleasedRevision != 0 && (value.ReleasedRevision <= value.AcquiredRevision || value.ReleasedRevision > revision) || !validDigest(value.Digest) {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid Resource Lease identity", nil)
	}
	if !filepath.IsAbs(value.PhysicalRoot) || filepath.Clean(value.PhysicalRoot) != value.PhysicalRoot {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "Resource Lease physical root is not canonical", nil)
	}
	if value.ReleasedRevision == 0 {
		physical, err := canonicalPhysicalRoot(value.PhysicalRoot)
		if err != nil || physical != value.PhysicalRoot {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "active Resource Lease physical root is unavailable", err)
		}
	}
	identity := value
	identity.ID, identity.Digest, identity.ReleasedRevision = "", "", 0
	identityDigest, _, err := canonicaljson.Digest(identity)
	if err != nil || value.ID != "lease-"+identityDigest[:32] {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "Resource Lease ID does not match identity", err)
	}
	unsigned := value
	unsigned.Digest = ""
	digest, _, err := canonicaljson.Digest(unsigned)
	if err != nil || digest != value.Digest {
		return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "Resource Lease digest mismatch", err)
	}
	return nil
}
