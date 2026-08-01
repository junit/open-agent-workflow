package runtime

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

const resourceLeaseProjectWorktree = "project-worktree"

var (
	resourceLeaseIDPattern = regexp.MustCompile(`^lease-[0-9a-f]{32}$`)
	grantIDPatternRuntime  = regexp.MustCompile(`^grant-[0-9a-f]{32}$`)
)

type resourceLeaseRequest struct {
	RunID            string
	GrantID          string
	BundleID         string
	Generation       uint64
	Resource         string
	PhysicalRoot     string
	AcquiredRevision uint64
}

func canonicalPhysicalRoot(root string) (string, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", os.ErrInvalid
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	physical = filepath.Clean(physical)
	info, err := os.Stat(physical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", os.ErrInvalid
	}
	return physical, nil
}

func newResourceLease(request resourceLeaseRequest) (ResourceLease, error) {
	physicalRoot, err := canonicalPhysicalRoot(request.PhysicalRoot)
	if err != nil {
		return ResourceLease{}, runtimeError("RESOURCE_LEASE_INVALID", "physical Worktree root is invalid", err)
	}
	if !runIDPattern.MatchString(request.RunID) || !grantIDPatternRuntime.MatchString(request.GrantID) || !bundleIDPattern.MatchString(request.BundleID) || request.Generation == 0 || request.Resource != resourceLeaseProjectWorktree || request.AcquiredRevision == 0 {
		return ResourceLease{}, runtimeError("RESOURCE_LEASE_INVALID", "invalid Resource Lease identity", nil)
	}
	value := ResourceLease{
		SchemaVersion: resourceLeaseSchemaV1, RunID: request.RunID, GrantID: request.GrantID,
		BundleID: request.BundleID, Generation: request.Generation, Resource: request.Resource,
		PhysicalRoot: physicalRoot, AcquiredRevision: request.AcquiredRevision,
	}
	seed := value
	seed.ID, seed.Digest = "", ""
	seedDigest, _, err := canonicaljson.Digest(seed)
	if err != nil {
		return ResourceLease{}, runtimeError("RESOURCE_LEASE_INVALID", "digest Resource Lease seed", err)
	}
	value.ID = deterministicRuntimeID("lease-", "lease\x00"+seedDigest)
	value.Digest, _, err = canonicaljson.Digest(value)
	if err != nil {
		return ResourceLease{}, runtimeError("RESOURCE_LEASE_INVALID", "digest Resource Lease", err)
	}
	return value, nil
}

func validateResourceLease(value ResourceLease) error {
	if value.SchemaVersion != resourceLeaseSchemaV1 || !resourceLeaseIDPattern.MatchString(value.ID) || !runIDPattern.MatchString(value.RunID) || !grantIDPatternRuntime.MatchString(value.GrantID) || !bundleIDPattern.MatchString(value.BundleID) || value.Generation == 0 || value.Resource != resourceLeaseProjectWorktree || value.AcquiredRevision == 0 || value.PhysicalRoot == "" || !filepath.IsAbs(value.PhysicalRoot) || filepath.Clean(value.PhysicalRoot) != value.PhysicalRoot {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Resource Lease identity", nil)
	}
	physicalRoot, err := canonicalPhysicalRoot(value.PhysicalRoot)
	if err != nil || physicalRoot != value.PhysicalRoot {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted Resource Lease root is not canonical", err)
	}
	stored := value.Digest
	value.Digest = ""
	digest, _, digestErr := canonicaljson.Digest(value)
	if digestErr != nil || digest != stored {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted Resource Lease digest mismatch", digestErr)
	}
	return nil
}

func workflowStageNeedsResourceLease(request *StageGrantRequest) bool {
	if request == nil {
		return false
	}
	for _, effect := range request.RequestedEffects {
		if effect == "write-project" || effect == "git-local" {
			return true
		}
	}
	return false
}

func workflowGrantNeedsResourceLease(grant admission.CapabilityGrant) bool {
	for _, effect := range grant.Effects {
		if effect == "write-project" || effect == "git-local" {
			return true
		}
	}
	return false
}

func (value *journal) acquireWorkflowResourceLease(current revisionRecord, grant admission.CapabilityGrant) (ResourceLease, error) {
	if !workflowGrantNeedsResourceLease(grant) {
		return ResourceLease{}, nil
	}
	if !containsWorkflowValue(grant.Resources, resourceLeaseProjectWorktree) {
		return ResourceLease{}, runtimeError("RESOURCE_LEASE_REQUIRED", "write-capable Workflow Grant must include project-worktree", nil)
	}
	if current.Snapshot.Workflow == nil {
		return ResourceLease{}, runtimeError("RUN_STATE_REVISION_INVALID", "Workflow state is missing for Resource Lease", nil)
	}
	physicalRoot, err := canonicalPhysicalRoot(current.Snapshot.Project.Root)
	if err != nil {
		return ResourceLease{}, runtimeError("RESOURCE_LEASE_INVALID", "resolve Workflow project root", err)
	}
	conflict, err := value.findActiveWorkflowLease(current.RunID, physicalRoot)
	if err != nil {
		return ResourceLease{}, err
	}
	if conflict != (ResourceLease{}) {
		return ResourceLease{}, runtimeError("RESOURCE_LEASE_CONFLICT", "physical project Worktree is already leased", nil)
	}
	return newResourceLease(resourceLeaseRequest{
		RunID: current.RunID, GrantID: grant.ID, BundleID: grant.BundleID, Generation: grant.Generation,
		Resource: resourceLeaseProjectWorktree, PhysicalRoot: physicalRoot, AcquiredRevision: grant.IssuedRevision,
	})
}

func (value *journal) findActiveWorkflowLease(excludeRunID, physicalRoot string) (ResourceLease, error) {
	entries, err := os.ReadDir(filepath.Join(value.stateRoot, "runs"))
	if err != nil {
		return ResourceLease{}, runtimeError("RUN_STATE_READ_FAILED", "scan committed Runs for Resource Leases", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !runIDPattern.MatchString(entry.Name()) || entry.Name() == excludeRunID {
			continue
		}
		record, loadErr := value.loadCommitted(entry.Name())
		if ErrorCode(loadErr) == "RUN_NOT_FOUND" {
			continue
		}
		if loadErr != nil {
			return ResourceLease{}, runtimeError("RUN_STATE_REVISION_INVALID", "validate committed Run during Resource Lease scan", loadErr)
		}
		if record.Snapshot.RequestMode != classification.RequestModeWorkflow || record.Snapshot.Workflow == nil {
			continue
		}
		for _, leaseID := range record.Snapshot.ResourceLeaseIDs {
			lease, found := workflowResourceLease(record.Snapshot.Workflow.ResourceLeases, leaseID)
			if !found {
				return ResourceLease{}, runtimeError("RUN_STATE_REVISION_INVALID", "active Resource Lease is missing from history", nil)
			}
			if lease.Resource == resourceLeaseProjectWorktree && lease.PhysicalRoot == physicalRoot {
				return lease, nil
			}
		}
	}
	return ResourceLease{}, nil
}

func workflowResourceLease(values []ResourceLease, id string) (ResourceLease, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return ResourceLease{}, false
}

func containsWorkflowValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validateWorkflowResourceLeases(record revisionRecord) error {
	snapshot := record.Snapshot
	workflow := snapshot.Workflow
	if workflow == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow state is missing for Resource Leases", nil)
	}
	if len(snapshot.ResourceLeaseIDs) > 1 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Workflow has more than one active Resource Lease", nil)
	}
	leases := make(map[string]ResourceLease, len(workflow.ResourceLeases))
	for _, lease := range workflow.ResourceLeases {
		if err := validateResourceLease(lease); err != nil {
			return err
		}
		if lease.RunID != snapshot.RunID || lease.PhysicalRoot != snapshot.Project.Root || lease.AcquiredRevision > record.Revision {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Resource Lease exceeds its Workflow Run", nil)
		}
		if _, exists := leases[lease.ID]; exists {
			return runtimeError("RUN_STATE_REVISION_INVALID", "duplicate Workflow Resource Lease", nil)
		}
		leases[lease.ID] = lease
	}
	grants := make(map[string]admission.CapabilityGrant, len(snapshot.Grants))
	for _, grant := range snapshot.Grants {
		grants[grant.ID] = grant
	}
	for _, lease := range workflow.ResourceLeases {
		grant, exists := grants[lease.GrantID]
		if !exists || !workflowGrantNeedsResourceLease(grant) || !containsWorkflowValue(grant.Resources, resourceLeaseProjectWorktree) || grant.BundleID != lease.BundleID || grant.Generation != lease.Generation || grant.IssuedRevision != lease.AcquiredRevision {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Resource Lease is not bound to a write Grant", nil)
		}
		bundle, exists := workflowBundleByID(workflow.Bundles, lease.BundleID)
		if !exists || bundle.Generation != lease.Generation {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Resource Lease is not bound to a Lifecycle Bundle", nil)
		}
	}
	active := make(map[string]struct{}, len(snapshot.ResourceLeaseIDs))
	for _, leaseID := range snapshot.ResourceLeaseIDs {
		if _, exists := active[leaseID]; exists {
			return runtimeError("RUN_STATE_REVISION_INVALID", "duplicate active Workflow Resource Lease", nil)
		}
		active[leaseID] = struct{}{}
		lease, exists := leases[leaseID]
		if !exists {
			return runtimeError("RUN_STATE_REVISION_INVALID", "active Resource Lease is missing from history", nil)
		}
		grant, exists := grants[lease.GrantID]
		if !exists || !workflowGrantNeedsResourceLease(grant) || !containsWorkflowValue(grant.Resources, resourceLeaseProjectWorktree) || grant.BundleID != lease.BundleID || grant.Generation != lease.Generation || grant.IssuedRevision != lease.AcquiredRevision {
			return runtimeError("RUN_STATE_REVISION_INVALID", "active Resource Lease is not bound to a write Grant", nil)
		}
		if workflow.ActiveGrantID != grant.ID || snapshot.Status != RunGranted && snapshot.Status != RunInFlight && snapshot.Status != RunPaused {
			return runtimeError("RUN_STATE_REVISION_INVALID", "active Resource Lease has no active Workflow Grant", nil)
		}
	}
	if snapshot.Status == RunReady && len(snapshot.ResourceLeaseIDs) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "ready Workflow Run retains an active Resource Lease", nil)
	}
	return nil
}

func workflowBundleByID(values []LifecycleBundle, id string) (LifecycleBundle, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return LifecycleBundle{}, false
}
