package admission

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

var (
	workflowIDPattern = regexp.MustCompile(`^workflow-[0-9a-f]{32}$`)
	bundleIDPattern   = regexp.MustCompile(`^bundle-[0-9a-f]{32}$`)
	grantIDPattern    = regexp.MustCompile(`^grant-[0-9a-f]{32}$`)
)

func IssueWorkflowGrant(request WorkflowGrantRequest) (CapabilityGrant, error) {
	normalized, err := normalizeWorkflowGrantRequest(request)
	if err != nil {
		return CapabilityGrant{}, err
	}
	grant := CapabilityGrant{
		SchemaVersion: CapabilityGrantSchemaV2, WorkflowID: normalized.WorkflowID, RequestID: normalized.RequestID,
		BundleID: normalized.BundleID, BundleGeneration: normalized.BundleGeneration, BundleDigest: normalized.BundleDigest,
		NodeID: normalized.Node.ID, Topology: normalized.Topology, HostSessionDigest: normalized.HostSessionDigest,
		ProviderID: normalized.Node.ProviderID, ProviderInstanceDigest: normalized.Node.ProviderInstanceDigest,
		CapabilityID: normalized.Node.CapabilityID, Binding: cloneBinding(normalized.Node.Binding),
		Effects: normalized.Effects, Resources: normalized.Resources, TerminationCondition: normalized.TerminationCondition,
	}
	seed := grant
	seed.ID = ""
	seed.Digest = ""
	digest, _, err := canonicaljson.Digest(seed)
	if err != nil {
		return CapabilityGrant{}, admissionError("CAPABILITY_GRANT_INVALID", "digest Grant identity", err)
	}
	grant.ID = "grant-" + digest[:32]
	grant.Digest, _, err = canonicaljson.Digest(grant)
	if err != nil {
		return CapabilityGrant{}, admissionError("CAPABILITY_GRANT_INVALID", "digest Grant", err)
	}
	return grant, nil
}

func ValidateGrant(value CapabilityGrant) error {
	if value.SchemaVersion != CapabilityGrantSchemaV2 || !grantIDPattern.MatchString(value.ID) || !validWorkflowID(value.WorkflowID) ||
		!validText(value.RequestID, 512) || !bundleIDPattern.MatchString(value.BundleID) || value.BundleGeneration == 0 || !validDigest(value.BundleDigest) ||
		!validLocalID(value.NodeID) || value.Topology != execution.TopologyCurrent && value.Topology != execution.TopologySubagent || !validDigest(value.HostSessionDigest) ||
		!validQualifiedID(value.ProviderID) || !validDigest(value.ProviderInstanceDigest) || !validLocalID(value.CapabilityID) ||
		!containsTopology(value.Binding.Topologies, value.Topology) || !validText(value.TerminationCondition, 2048) || !validSortedSet(value.Effects, knownEffect) ||
		!validSortedSet(value.Resources, knownResource) || !validDigest(value.Digest) {
		return admissionError("CAPABILITY_GRANT_INVALID", "invalid Grant identity or authority", nil)
	}
	if err := validateBinding(value.Binding); err != nil {
		return err
	}
	if err := validateGrantDigest(value); err != nil {
		return err
	}
	return nil
}

func normalizeWorkflowGrantRequest(value WorkflowGrantRequest) (WorkflowGrantRequest, error) {
	if !validWorkflowID(value.WorkflowID) || !validText(value.RequestID, 512) || !bundleIDPattern.MatchString(value.BundleID) || value.BundleGeneration == 0 ||
		!validDigest(value.BundleDigest) || !validLocalID(value.Node.ID) || !validText(value.Node.Responsibility, 512) ||
		!validQualifiedID(value.Node.ProviderID) || !validDigest(value.Node.ProviderInstanceDigest) || !validLocalID(value.Node.CapabilityID) ||
		!validDigest(value.HostSessionDigest) ||
		(value.Topology != execution.TopologyCurrent && value.Topology != execution.TopologySubagent) || !validText(value.TerminationCondition, 2048) {
		return WorkflowGrantRequest{}, admissionError("WORKFLOW_GRANT_INVALID", "invalid Grant request identity", nil)
	}
	if err := validateNodeBinding(value.Node); err != nil {
		return WorkflowGrantRequest{}, err
	}
	if !containsTopology(value.Node.SupportedTopologies, value.Topology) || !containsTopology(value.Node.Binding.Topologies, value.Topology) {
		return WorkflowGrantRequest{}, admissionError("CAPABILITY_TOPOLOGY_DENIED", "selected topology is not supported by the active graph node", nil)
	}
	var err error
	value.Effects, err = normalizeSet(value.Effects, knownEffect)
	if err != nil {
		return WorkflowGrantRequest{}, admissionError("WORKFLOW_GRANT_INVALID", "invalid requested effects", err)
	}
	value.Resources, err = normalizeSet(value.Resources, knownResource)
	if err != nil {
		return WorkflowGrantRequest{}, admissionError("WORKFLOW_GRANT_INVALID", "invalid requested resources", err)
	}
	if len(value.Effects) == 0 || len(value.Resources) == 0 || !subset(value.Effects, value.Node.MaximumEffects) || !subset(value.Resources, value.Node.Resources) {
		return WorkflowGrantRequest{}, admissionError("CAPABILITY_AUTHORITY_EXCEEDED", "requested authority exceeds active graph node", nil)
	}
	authorityEffects, err := normalizeSet(value.Authority.Effects, knownEffect)
	if err != nil {
		return WorkflowGrantRequest{}, admissionError("WORKFLOW_GRANT_INVALID", "invalid Engine effect ceiling", err)
	}
	authorityResources, err := normalizeSet(value.Authority.Resources, knownResource)
	if err != nil {
		return WorkflowGrantRequest{}, admissionError("WORKFLOW_GRANT_INVALID", "invalid Engine resource ceiling", err)
	}
	if !subset(value.Effects, authorityEffects) || !subset(value.Resources, authorityResources) {
		return WorkflowGrantRequest{}, admissionError("CAPABILITY_AUTHORITY_EXCEEDED", "requested authority exceeds Engine ceiling", nil)
	}
	if requiresResourceLease(value.Effects) {
		if !value.Authority.ResourceLeases {
			return WorkflowGrantRequest{}, admissionError("RESOURCE_LEASE_REQUIRED", "write-capable Grant requires Resource Lease authority", nil)
		}
		if !contains(value.Resources, "project-worktree") {
			return WorkflowGrantRequest{}, admissionError("RESOURCE_LEASE_REQUIRED", "write-capable Grant must include project-worktree", nil)
		}
	}
	return value, nil
}

func validateGrantDigest(value CapabilityGrant) error {
	seed := value
	seed.ID = ""
	seed.Digest = ""
	digest, _, err := canonicaljson.Digest(seed)
	if err != nil || value.ID != "grant-"+digest[:32] {
		return admissionError("CAPABILITY_GRANT_INVALID", "Grant ID does not match content", err)
	}
	unsigned := value
	unsigned.Digest = ""
	digest, _, err = canonicaljson.Digest(unsigned)
	if err != nil || digest != value.Digest {
		return admissionError("CAPABILITY_GRANT_INVALID", "Grant digest does not match content", err)
	}
	return nil
}

func requiresResourceLease(effects []string) bool {
	return contains(effects, "write-project") || contains(effects, "git-local")
}

func normalizeSet(values []string, known func(string) bool) ([]string, error) {
	result := append([]string{}, values...)
	for _, value := range result {
		if !validText(value, 128) || !known(value) {
			return nil, admissionError("WORKFLOW_GRANT_INVALID", "unknown authority value", nil)
		}
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index-1] == result[index] {
			return nil, admissionError("WORKFLOW_GRANT_INVALID", "duplicate authority value", nil)
		}
	}
	return result, nil
}

func validSortedSet(values []string, known func(string) bool) bool {
	if len(values) == 0 {
		return false
	}
	for index, value := range values {
		if !validText(value, 128) || !known(value) || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func validateNodeBinding(node profile.GraphNode) error {
	if !validText(node.Binding.Host, 512) || !validText(node.Binding.Kind, 128) || !validText(node.Binding.Reference, 2048) {
		return admissionError("WORKFLOW_GRANT_INVALID", "invalid graph node Binding", nil)
	}
	if values, err := execution.NormalizeTopologies(node.SupportedTopologies); err != nil || !slices.Equal(values, node.SupportedTopologies) {
		return admissionError("WORKFLOW_GRANT_INVALID", "graph node topologies are not canonical", err)
	}
	if values, err := execution.NormalizeTopologies(node.Binding.Topologies); err != nil || !slices.Equal(values, node.Binding.Topologies) {
		return admissionError("WORKFLOW_GRANT_INVALID", "graph Binding topologies are not canonical", err)
	}
	return nil
}

func validateBinding(value catalog.HostBinding) error {
	if !validText(value.Host, 512) || !validText(value.Kind, 128) || !validText(value.Reference, 2048) {
		return admissionError("CAPABILITY_GRANT_INVALID", "invalid Grant Binding", nil)
	}
	if values, err := execution.NormalizeTopologies(value.Topologies); err != nil || !slices.Equal(values, value.Topologies) {
		return admissionError("CAPABILITY_GRANT_INVALID", "Grant Binding topologies are not canonical", err)
	}
	return nil
}

func cloneBinding(value catalog.HostBinding) catalog.HostBinding {
	value.Topologies = append([]execution.Topology{}, value.Topologies...)
	return value
}

func subset(values, allowed []string) bool {
	for _, value := range values {
		if !contains(allowed, value) {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsTopology(values []execution.Topology, wanted execution.Topology) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func validWorkflowID(value string) bool {
	return workflowIDPattern.MatchString(value)
}

func validLocalID(value string) bool {
	_, err := catalog.ParseLocalID(value)
	return err == nil
}

func validQualifiedID(value string) bool {
	_, err := catalog.ParseQualifiedID(value)
	return err == nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validText(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum && strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func knownEffect(value string) bool {
	switch value {
	case "read-project", "write-project", "run-process", "git-local", "network-read":
		return true
	default:
		return false
	}
}

func knownResource(value string) bool {
	switch value {
	case "project", "project-worktree", "git-repository", "network", "credentials", "security", "data", "deployment":
		return true
	default:
		return false
	}
}
