package admission

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

var (
	runIDPattern        = regexp.MustCompile(`^run-[0-9a-f]{32}$`)
	grantIDPattern      = regexp.MustCompile(`^grant-[0-9a-f]{32}$`)
	invocationIDPattern = regexp.MustCompile(`^invocation-[0-9a-f]{32}$`)
)

// VerifyBoundedCapability checks that one selector resolves to one exact,
// verified Capability that explicitly supports Bounded mode. It does not
// authorize effects, resources, or an Executor; Grant issuance owns those
// narrower authority checks.
func VerifyBoundedCapability(selector classification.CapabilitySelector, catalogSource CatalogSource, registrySource VerifiedRegistry) error {
	if catalogSource == nil || registrySource == nil || !validDigest(catalogSource.Digest()) || !validDigest(registrySource.Digest()) {
		return admissionError("CAPABILITY_NOT_VERIFIED", "trusted Catalog or Registry is unavailable", nil)
	}
	request := GrantRequest{Selector: selector, Catalog: catalogSource, Registry: registrySource}
	_, capability, _, _, err := resolveCapability(request)
	if err != nil {
		return err
	}
	if !containsMode(capability.RequestModes, catalog.RequestModeBounded) {
		return admissionError("CAPABILITY_MODE_NOT_ALLOWED", selector.CapabilityID, nil)
	}
	return nil
}

func IssueBoundedGrant(request GrantRequest) (CapabilityGrant, error) {
	normalized, err := normalizeGrantRequest(request)
	if err != nil {
		return CapabilityGrant{}, err
	}
	providerRecord, capabilityRecord, providerInstance, verifiedCapability, err := resolveCapability(normalized)
	if err != nil {
		return CapabilityGrant{}, err
	}
	if !containsMode(capabilityRecord.RequestModes, catalog.RequestModeBounded) {
		return CapabilityGrant{}, admissionError("CAPABILITY_MODE_NOT_ALLOWED", normalized.Selector.CapabilityID, nil)
	}
	if err := validateEffects(normalized, capabilityRecord); err != nil {
		return CapabilityGrant{}, err
	}
	if err := validateResources(normalized, capabilityRecord); err != nil {
		return CapabilityGrant{}, err
	}
	executor, err := resolveExecutor(normalized.Executor, normalized.Executors, capabilityRecord.ExecutorTopology)
	if err != nil {
		return CapabilityGrant{}, err
	}
	if len(normalized.DelegationAllowList) != 0 {
		if !normalized.Authority.AllowDelegation || !subset(normalized.DelegationAllowList, capabilityRecord.DelegationAllowList) {
			return CapabilityGrant{}, admissionError("CAPABILITY_AUTHORITY_EXCEEDED", "delegation exceeds authority", nil)
		}
	}
	descriptorDigest, _, err := canonicaljson.Digest(providerRecord)
	if err != nil {
		return CapabilityGrant{}, admissionError("CAPABILITY_NOT_VERIFIED", "digest Provider Descriptor", err)
	}
	grant := CapabilityGrant{
		SchemaVersion: CapabilityGrantSchemaV1,
		RunID:         normalized.RunID, RequestID: normalized.RequestID, DeliverableID: normalized.DeliverableID,
		InputDigest: normalized.InputDigest, IssuedRevision: normalized.IssuedRevision, Generation: 0,
		ProviderID: normalized.Selector.ProviderID, ProviderInstanceDigest: providerInstance.Digest,
		DescriptorDigest: descriptorDigest, RegistryDigest: normalized.Registry.Digest(), CatalogDigest: normalized.Catalog.Digest(),
		CapabilityID: normalized.Selector.CapabilityID, Binding: verifiedCapability.Binding, Executor: executor,
		Effects: normalized.Effects, Resources: normalized.Resources, TerminationCondition: normalized.TerminationCondition,
		DelegationAllowList: normalized.DelegationAllowList,
	}
	return finalizeGrant(grant)
}

func DeriveChildGrant(request ChildGrantRequest) (CapabilityGrant, error) {
	parent := CloneGrant(request.Parent)
	if err := ValidateGrant(parent); err != nil {
		return CapabilityGrant{}, admissionError("PARENT_GRANT_INVALID", "parent Grant failed validation", err)
	}
	childRequest := request.Request
	if childRequest.RunID != parent.RunID || childRequest.RequestID != parent.RequestID || childRequest.IssuedRevision <= parent.IssuedRevision || childRequest.Selector.ProviderID != parent.ProviderID {
		return CapabilityGrant{}, admissionError("CHILD_GRANT_NOT_ALLOWED", "child identity is outside parent Run", nil)
	}
	if !contains(parent.DelegationAllowList, childRequest.Selector.CapabilityID) || !subset(childRequest.Effects, parent.Effects) || !subset(childRequest.Resources, parent.Resources) || !subset(childRequest.DelegationAllowList, parent.DelegationAllowList) {
		return CapabilityGrant{}, admissionError("CHILD_GRANT_NOT_ALLOWED", "child authority exceeds parent", nil)
	}
	childRequest.Authority = AuthorityCeiling{
		Effects: append([]string{}, parent.Effects...), Resources: append([]string{}, parent.Resources...),
		ResourceLeases: request.Request.Authority.ResourceLeases, AllowDelegation: true,
	}
	grant, err := IssueBoundedGrant(childRequest)
	if err != nil {
		return CapabilityGrant{}, admissionError("CHILD_GRANT_NOT_ALLOWED", "child Capability admission failed", err)
	}
	grant.ParentGrantID = parent.ID
	return finalizeGrant(grant)
}

func ValidateGrant(value CapabilityGrant) error {
	if value.SchemaVersion != CapabilityGrantSchemaV1 || !grantIDPattern.MatchString(value.ID) || !invocationIDPattern.MatchString(value.InvocationID) || !runIDPattern.MatchString(value.RunID) || !validIdentifier(value.RequestID) || !validIdentifier(value.DeliverableID) || !validDigest(value.InputDigest) || value.IssuedRevision == 0 || value.Generation != 0 {
		return admissionError("PARENT_GRANT_INVALID", "invalid Grant identity", nil)
	}
	if _, err := catalog.ParseQualifiedID(value.ProviderID); err != nil || !validDigest(value.ProviderInstanceDigest) || !validDigest(value.DescriptorDigest) || !validDigest(value.RegistryDigest) || !validDigest(value.CatalogDigest) {
		return admissionError("PARENT_GRANT_INVALID", "invalid Provider identity", err)
	}
	if _, err := catalog.ParseLocalID(value.CapabilityID); err != nil || !validExecutor(value.Executor) || value.Binding.Host == "" || value.Binding.Reference == "" || value.TerminationCondition == "" {
		return admissionError("PARENT_GRANT_INVALID", "invalid Capability identity", err)
	}
	if !validSortedSet(value.Effects, knownEffect) || !validSortedSet(value.Resources, knownResource) || !validSortedLocalIDs(value.DelegationAllowList) {
		return admissionError("PARENT_GRANT_INVALID", "invalid Grant authority", nil)
	}
	stored := value.Digest
	if !validDigest(stored) {
		return admissionError("PARENT_GRANT_INVALID", "invalid Grant digest", nil)
	}
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil || digest != stored {
		return admissionError("PARENT_GRANT_INVALID", "Grant digest mismatch", err)
	}
	return nil
}

func normalizeGrantRequest(value GrantRequest) (GrantRequest, error) {
	if !runIDPattern.MatchString(value.RunID) || !validIdentifier(value.RequestID) || !validIdentifier(value.DeliverableID) || !validDigest(value.InputDigest) || value.IssuedRevision == 0 || strings.TrimSpace(value.TerminationCondition) == "" || value.Catalog == nil || value.Registry == nil || !validDigest(value.Catalog.Digest()) || !validDigest(value.Registry.Digest()) {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "invalid request identity or trusted inputs", nil)
	}
	if _, err := catalog.ParseQualifiedID(value.Selector.ProviderID); err != nil {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "invalid Provider selector", err)
	}
	if _, err := catalog.ParseLocalID(value.Selector.CapabilityID); err != nil {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "invalid Capability selector", err)
	}
	if value.Selector.Source != classification.SelectorUserIntent && value.Selector.Source != classification.SelectorTrustedRule {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "invalid selector source", nil)
	}
	effects, err := normalizeSet(value.Effects)
	if err != nil || len(effects) == 0 {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "effects must be a unique non-empty set", err)
	}
	resources, err := normalizeSet(value.Resources)
	if err != nil || len(resources) == 0 {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "resources must be a unique non-empty set", err)
	}
	delegation, err := normalizeSet(value.DelegationAllowList)
	if err != nil {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "delegation must be unique", err)
	}
	authorityEffects, err := normalizeSet(value.Authority.Effects)
	if err != nil {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "authority effects must be unique", err)
	}
	authorityResources, err := normalizeSet(value.Authority.Resources)
	if err != nil {
		return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "authority resources must be unique", err)
	}
	executors := CloneExecutors(value.Executors)
	sort.Slice(executors, func(i, j int) bool { return executors[i].ID < executors[j].ID })
	for index, executor := range executors {
		if !validExecutor(executor) || index > 0 && executors[index-1].ID == executor.ID {
			return GrantRequest{}, admissionError("BOUNDED_REQUEST_INVALID", "invalid or duplicate Executor registration", nil)
		}
	}
	value.Effects = effects
	value.Resources = resources
	value.DelegationAllowList = delegation
	value.Authority = AuthorityCeiling{Effects: authorityEffects, Resources: authorityResources, ResourceLeases: value.Authority.ResourceLeases, AllowDelegation: value.Authority.AllowDelegation}
	value.Executors = executors
	return value, nil
}

func resolveCapability(request GrantRequest) (catalog.ProviderDescriptorRecord, catalog.CapabilityRecord, registry.ProviderInstance, registry.VerifiedCapability, error) {
	var providerRecord catalog.ProviderDescriptorRecord
	providerMatches := 0
	for _, candidate := range request.Catalog.Providers() {
		if candidate.ID == request.Selector.ProviderID {
			providerRecord = candidate
			providerMatches++
		}
	}
	providerInstance, providerFound := request.Registry.Provider(request.Selector.ProviderID)
	verifiedCapability, capabilityVerified := request.Registry.Capability(request.Selector.ProviderID, request.Selector.CapabilityID)
	if providerMatches != 1 || !providerFound || !capabilityVerified || providerInstance.ProviderID != request.Selector.ProviderID || verifiedCapability.ID != request.Selector.CapabilityID || !validDigest(providerInstance.Digest) {
		return catalog.ProviderDescriptorRecord{}, catalog.CapabilityRecord{}, registry.ProviderInstance{}, registry.VerifiedCapability{}, admissionError("CAPABILITY_NOT_VERIFIED", "Provider or Capability is not uniquely verified", nil)
	}
	descriptorDigest, _, err := canonicaljson.Digest(providerRecord)
	if err != nil || descriptorDigest != providerInstance.DescriptorDigest {
		return catalog.ProviderDescriptorRecord{}, catalog.CapabilityRecord{}, registry.ProviderInstance{}, registry.VerifiedCapability{}, admissionError("CAPABILITY_NOT_VERIFIED", "Provider Descriptor digest mismatch", err)
	}
	var capabilityRecord catalog.CapabilityRecord
	capabilityMatches := 0
	for _, candidate := range providerRecord.Capabilities {
		if candidate.ID == request.Selector.CapabilityID {
			capabilityRecord = candidate
			capabilityMatches++
		}
	}
	if capabilityMatches != 1 || !containsBinding(capabilityRecord.HostBindings, verifiedCapability.Binding) {
		return catalog.ProviderDescriptorRecord{}, catalog.CapabilityRecord{}, registry.ProviderInstance{}, registry.VerifiedCapability{}, admissionError("CAPABILITY_NOT_VERIFIED", "Capability contract or Binding mismatch", nil)
	}
	return providerRecord, capabilityRecord, providerInstance, verifiedCapability, nil
}

func validateEffects(request GrantRequest, capability catalog.CapabilityRecord) error {
	for _, effect := range request.Effects {
		if !knownEffect(effect) || !contains(capability.MaximumEffects, effect) {
			return admissionError("CAPABILITY_EFFECT_NOT_ALLOWED", effect, nil)
		}
		if !contains(request.Authority.Effects, effect) {
			return admissionError("CAPABILITY_AUTHORITY_EXCEEDED", effect, nil)
		}
		if effect == "git-local" {
			return admissionError("CAPABILITY_EFFECT_NOT_ALLOWED", "Bounded Git completion is forbidden", nil)
		}
		if effect == "write-project" && !request.Authority.ResourceLeases {
			return admissionError("RESOURCE_LEASE_REQUIRED", effect, nil)
		}
	}
	return nil
}

func validateResources(request GrantRequest, capability catalog.CapabilityRecord) error {
	for _, resource := range request.Resources {
		if !knownResource(resource) || !contains(capability.Resources, resource) {
			return admissionError("CAPABILITY_RESOURCE_NOT_ALLOWED", resource, nil)
		}
		if !contains(request.Authority.Resources, resource) {
			return admissionError("CAPABILITY_AUTHORITY_EXCEEDED", resource, nil)
		}
	}
	return nil
}

func resolveExecutor(requested ExecutorRegistration, registered []ExecutorRegistration, topology catalog.ExecutorTopology) (ExecutorRegistration, error) {
	for _, executor := range registered {
		if executor.ID != requested.ID {
			continue
		}
		if executor != requested {
			return ExecutorRegistration{}, admissionError("EXECUTOR_NOT_REGISTERED", requested.ID, nil)
		}
		if executor.Kind == ExecutorMainAgent && topology != catalog.MainAgentAllowed {
			return ExecutorRegistration{}, admissionError("EXECUTOR_TOPOLOGY_DENIED", requested.ID, nil)
		}
		return executor, nil
	}
	return ExecutorRegistration{}, admissionError("EXECUTOR_NOT_REGISTERED", requested.ID, nil)
}

func finalizeGrant(value CapabilityGrant) (CapabilityGrant, error) {
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	value.DelegationAllowList = append([]string{}, value.DelegationAllowList...)
	value.ID = ""
	value.InvocationID = ""
	value.Digest = ""
	seed, _, err := canonicaljson.Digest(value)
	if err != nil {
		return CapabilityGrant{}, admissionError("BOUNDED_REQUEST_INVALID", "digest Grant seed", err)
	}
	value.ID = deterministicID("grant-", "grant\x00"+seed)
	value.InvocationID = deterministicID("invocation-", "invocation\x00"+seed)
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return CapabilityGrant{}, admissionError("BOUNDED_REQUEST_INVALID", "digest Grant", err)
	}
	value.Digest = digest
	return CloneGrant(value), nil
}

func deterministicID(prefix, seed string) string {
	digest := sha256.Sum256([]byte(seed))
	return prefix + hex.EncodeToString(digest[:16])
}

func normalizeSet(values []string) ([]string, error) {
	normalized := append([]string{}, values...)
	sort.Strings(normalized)
	for index := 1; index < len(normalized); index++ {
		if normalized[index-1] == normalized[index] {
			return nil, fmt.Errorf("duplicate value %q", normalized[index])
		}
	}
	return normalized, nil
}

func subset(values, ceiling []string) bool {
	for _, value := range values {
		if !contains(ceiling, value) {
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

func containsMode(values []catalog.RequestMode, wanted catalog.RequestMode) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsBinding(values []catalog.HostBinding, wanted catalog.HostBinding) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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
	case "project", "project-worktree", "git-repository":
		return true
	default:
		return false
	}
}

func validSortedSet(values []string, validate func(string) bool) bool {
	if values == nil || len(values) == 0 {
		return false
	}
	for index, value := range values {
		if !validate(value) || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func validSortedLocalIDs(values []string) bool {
	if values == nil {
		return false
	}
	for index, value := range values {
		if _, err := catalog.ParseLocalID(value); err != nil || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func validExecutor(value ExecutorRegistration) bool {
	return validIdentifier(value.ID) && (value.Kind == ExecutorMainAgent || value.Kind == ExecutorIsolated)
}

func validIdentifier(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 256 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
