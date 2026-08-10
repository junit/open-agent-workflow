package registry

import (
	"fmt"
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const providerInstanceSchemaV4 = "oaw.provider-instance/v4"

type resolutionSource interface {
	Catalog() catalog.Catalog
	ProviderSettings(providerID, hostID string) config.ProviderSettings
	RequiredProviders() []string
	RecommendedProviders() []string
	UntrustedProviderIDs() []string
}

func Resolve(snapshot config.Snapshot, hostID string, evidence discovery.Report, inventory *host.BindingInventory) (ResolutionReport, Registry, error) {
	return resolveFromSource(snapshot, hostID, evidence, inventory)
}

func resolveFromSource(source resolutionSource, hostID string, evidence discovery.Report, inventory *host.BindingInventory) (ResolutionReport, Registry, error) {
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return ResolutionReport{}, Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: invalid Host ID %q: %w", hostID, err)
	}
	if evidence.HostID() != hostID {
		return ResolutionReport{}, Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: discovery Host %q does not match selected Host %q", evidence.HostID(), hostID)
	}
	var normalizedInventory *host.BindingInventory
	if inventory != nil {
		if inventory.HostID != hostID {
			return ResolutionReport{}, Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: inventory Host %q does not match selected Host %q", inventory.HostID, hostID)
		}
		value, err := host.ValidateBindingInventory(*inventory)
		if err != nil {
			return ResolutionReport{}, Registry{}, err
		}
		normalizedInventory = &value
	}

	descriptors := make(map[string]catalog.ProviderDescriptorRecord)
	identities := make(map[string]struct{})
	for _, provider := range source.Catalog().Providers() {
		descriptors[provider.ID] = provider
		identities[provider.ID] = struct{}{}
	}
	untrusted := make(map[string]struct{})
	for _, id := range source.UntrustedProviderIDs() {
		untrusted[id] = struct{}{}
		identities[id] = struct{}{}
	}
	for _, ids := range [][]string{source.RequiredProviders(), source.RecommendedProviders()} {
		for _, id := range ids {
			identities[id] = struct{}{}
		}
	}
	providerIDs := make([]string, 0, len(identities))
	for id := range identities {
		providerIDs = append(providerIDs, id)
	}
	sort.Strings(providerIDs)

	resolutions := make([]ProviderResolution, 0, len(providerIDs))
	instances := make([]ProviderInstance, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		settings := source.ProviderSettings(providerID, hostID)
		candidates := evidence.Candidates(providerID)
		resolution := ProviderResolution{ProviderID: providerID, Candidates: candidates}
		if settings.Disabled {
			resolution.State = ProviderDisabled
			resolution.Reason = "PROVIDER_DISABLED_BY_USER"
			resolutions = append(resolutions, resolution)
			continue
		}
		if _, found := untrusted[providerID]; found {
			resolution.State = ProviderUntrusted
			resolution.Reason = "PROVIDER_PROJECT_CONTENT_UNTRUSTED"
			resolutions = append(resolutions, resolution)
			continue
		}
		descriptor, descriptorFound := descriptors[providerID]
		if !descriptorFound || len(candidates) == 0 {
			resolution.State = ProviderNotFound
			resolution.Reason = "PROVIDER_NOT_FOUND"
			resolutions = append(resolutions, resolution)
			continue
		}
		selected := candidates
		if settings.Pin != nil {
			selected = matchingCandidates(candidates, *settings.Pin)
			if len(selected) == 0 {
				resolution.State = ProviderIncompatible
				resolution.Reason = "PROVIDER_PIN_INCOMPATIBLE"
				resolutions = append(resolutions, resolution)
				continue
			}
		}
		if len(selected) != 1 {
			resolution.State = ProviderAmbiguous
			resolution.Reason = "PROVIDER_CANDIDATE_AMBIGUOUS"
			resolutions = append(resolutions, resolution)
			continue
		}
		if normalizedInventory == nil || len(normalizedInventory.Observations) == 0 {
			resolution.State = ProviderCandidate
			resolution.Reason = "HOST_BINDING_EVIDENCE_REQUIRED"
			resolutions = append(resolutions, resolution)
			continue
		}
		bindings, capabilities, preferenceIncompatible := resolveProviderBindings(descriptor, settings, selected[0], *normalizedInventory)
		if preferenceIncompatible {
			resolution.State = ProviderIncompatible
			resolution.Reason = "BINDING_PREFERENCE_INCOMPATIBLE"
			resolutions = append(resolutions, resolution)
			continue
		}
		if len(bindings) == 0 || len(capabilities) == 0 {
			resolution.State = ProviderBindingUnavailable
			resolution.Reason = "PROVIDER_BINDING_UNAVAILABLE"
			resolutions = append(resolutions, resolution)
			continue
		}
		instance, err := buildProviderInstance(descriptor, settings, selected[0], normalizedInventory.Digest, bindings, capabilities)
		if err != nil {
			return ResolutionReport{}, Registry{}, err
		}
		resolution.State = ProviderVerified
		resolution.Reason = "PROVIDER_VERIFIED"
		resolution.Instance = &instance
		resolutions = append(resolutions, resolution)
		instances = append(instances, instance)
	}
	report, err := newResolutionReport(hostID, resolutions)
	if err != nil {
		return ResolutionReport{}, Registry{}, err
	}
	effective, err := newRegistry(hostID, instances)
	if err != nil {
		return ResolutionReport{}, Registry{}, err
	}
	return report, effective, nil
}

func matchingCandidates(values []discovery.Candidate, pin config.ProviderPin) []discovery.Candidate {
	result := make([]discovery.Candidate, 0, len(values))
	for _, candidate := range values {
		if candidate.ProviderID != pin.ProviderID || candidate.HostID != pin.HostID || candidate.InstallationKey != pin.InstallationKey || candidate.EvidenceDigest != pin.EvidenceDigest {
			continue
		}
		if pin.Location != "" && candidate.DiagnosticLocation != pin.Location {
			continue
		}
		if pin.Version != "" && candidate.ObservedRevision != pin.Version {
			continue
		}
		result = append(result, candidate)
	}
	return result
}

func resolveProviderBindings(descriptor catalog.ProviderDescriptorRecord, settings config.ProviderSettings, candidate discovery.Candidate, inventory host.BindingInventory) ([]VerifiedBinding, []VerifiedCapability, bool) {
	distribution, found := distributionByID(descriptor.Distributions, candidate.DistributionID)
	if !found || !candidateMatchesDistribution(descriptor.ID, candidate, distribution) {
		return nil, nil, false
	}
	roots := make(map[string]discovery.BindingRootEvidence, len(candidate.BindingRoots))
	for _, root := range candidate.BindingRoots {
		if _, exists := roots[root.BindingID]; exists {
			return nil, nil, false
		}
		roots[root.BindingID] = root
	}
	observations := make(map[string][]host.BindingObservation)
	for _, observation := range inventory.Observations {
		key := observationIdentityKey(observation)
		observations[key] = append(observations[key], observation)
	}

	verifiedByID := make(map[string]VerifiedBinding)
	for _, binding := range descriptor.Bindings {
		if binding.DistributionID != distribution.ID || binding.Host != candidate.HostID || binding.Surface != candidate.Surface {
			continue
		}
		root, found := roots[binding.ID]
		if !found || root.BindingID != binding.ID || root.ContentRoot != binding.ContentRoot || root.InstallRoot != binding.InstallRoot || root.Tree.RootDigest != binding.TreeDigest {
			continue
		}
		key := bindingObservationKey(candidate.HostID, descriptor.ID, candidate.InstallationKey, distribution.ID, binding.ID)
		matches := observations[key]
		if len(matches) != 1 || !observationMatchesBinding(matches[0], binding, candidate) {
			continue
		}
		observation := matches[0]
		verifiedByID[binding.ID] = VerifiedBinding{
			BindingID: binding.ID, DistributionID: distribution.ID, DistributionRevision: distribution.Revision, DistributionTreeDigest: distribution.TreeDigest,
			Surface: binding.Surface, Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
			SupportedTopologies: append([]execution.Topology{}, observation.Topologies...), Delegation: binding.Delegation, Provenance: candidate.Provenance,
			BindingEvidenceDigest: observation.Digest,
		}
	}

	allowed := make(map[string]struct{}, len(settings.CapabilityLimit))
	for _, id := range settings.CapabilityLimit {
		allowed[id] = struct{}{}
	}
	preferences := make(map[string][]config.BindingPreference)
	for _, preference := range settings.Preferences {
		preferences[preference.CapabilityID] = append(preferences[preference.CapabilityID], preference)
	}
	capabilities := make([]VerifiedCapability, 0, len(descriptor.Capabilities))
	usedBindings := make(map[string]VerifiedBinding)
	for _, capability := range descriptor.Capabilities {
		if len(allowed) != 0 {
			if _, found := allowed[capability.ID]; !found {
				continue
			}
		}
		bindingIDs := make([]string, 0, len(capability.BindingRefs))
		for _, bindingID := range capability.BindingRefs {
			if binding, found := verifiedByID[bindingID]; found {
				bindingIDs = append(bindingIDs, bindingID)
				usedBindings[bindingID] = binding
			}
		}
		if len(bindingIDs) == 0 {
			continue
		}
		sort.Strings(bindingIDs)
		preferredBindingID := ""
		if values := preferences[capability.ID]; len(values) != 0 {
			if len(values) != 1 || values[0].ProviderID != descriptor.ID || values[0].CapabilityID != capability.ID || values[0].HostID != candidate.HostID {
				return nil, nil, true
			}
			preference := values[0]
			matches := make([]string, 0, len(bindingIDs))
			for _, bindingID := range bindingIDs {
				binding := verifiedByID[bindingID]
				if binding.Kind == catalog.BindingKind(preference.Kind) && binding.Reference == preference.Reference {
					matches = append(matches, bindingID)
				}
			}
			if len(matches) != 1 {
				return nil, nil, true
			}
			preferredBindingID = matches[0]
		}
		capabilities = append(capabilities, VerifiedCapability{ID: capability.ID, BindingIDs: bindingIDs, PreferredBindingID: preferredBindingID})
	}
	sort.Slice(capabilities, func(left, right int) bool { return capabilities[left].ID < capabilities[right].ID })
	bindings := make([]VerifiedBinding, 0, len(usedBindings))
	for _, binding := range usedBindings {
		bindings = append(bindings, binding)
	}
	sort.Slice(bindings, func(left, right int) bool { return bindings[left].BindingID < bindings[right].BindingID })
	return bindings, capabilities, false
}

func candidateMatchesDistribution(providerID string, candidate discovery.Candidate, distribution catalog.DistributionRecord) bool {
	if candidate.ProviderID != providerID || candidate.HostID == "" || candidate.Surface == "" || candidate.DistributionID != distribution.ID || candidate.InstallationKey == "" || candidate.EvidenceDigest == "" {
		return false
	}
	switch candidate.Provenance {
	case discovery.ProvenanceDistributionAttested:
		return candidate.ObservedRevision == distribution.Revision && candidate.DistributionTreeDigest == distribution.TreeDigest
	case discovery.ProvenanceContentEquivalent:
		return candidate.ObservedRevision == "" && candidate.DistributionTreeDigest == ""
	default:
		return false
	}
}

func observationMatchesBinding(observation host.BindingObservation, binding catalog.BindingRecord, candidate discovery.Candidate) bool {
	if observation.HostID != candidate.HostID || observation.ProviderID != candidate.ProviderID || observation.InstallationKey != candidate.InstallationKey ||
		observation.DistributionID != binding.DistributionID || observation.BindingID != binding.ID || observation.Surface != binding.Surface || observation.Kind != binding.Kind ||
		observation.Reference != binding.Reference || observation.Invocation != binding.Invocation || observation.BindingTreeDigest != binding.TreeDigest {
		return false
	}
	for _, topology := range observation.Topologies {
		if !slices.Contains(binding.SupportedTopologies, topology) {
			return false
		}
	}
	return len(observation.Topologies) != 0
}

func buildProviderInstance(descriptor catalog.ProviderDescriptorRecord, settings config.ProviderSettings, candidate discovery.Candidate, inventoryDigest string, bindings []VerifiedBinding, capabilities []VerifiedCapability) (ProviderInstance, error) {
	descriptorDigest, _, err := canonicaljson.Digest(descriptor)
	if err != nil {
		return ProviderInstance{}, err
	}
	distribution, found := distributionByID(descriptor.Distributions, candidate.DistributionID)
	if !found {
		return ProviderInstance{}, fmt.Errorf("PROVIDER_DISTRIBUTION_NOT_FOUND: %s/%s", descriptor.ID, candidate.DistributionID)
	}
	instance := ProviderInstance{
		ProviderID: descriptor.ID, HostID: candidate.HostID, DescriptorDigest: descriptorDigest,
		DistributionID: distribution.ID, DistributionRevision: distribution.Revision, DistributionTreeDigest: distribution.TreeDigest,
		InstallationKey: candidate.InstallationKey, ConfigurationDigest: settings.Digest, BindingInventoryDigest: inventoryDigest, EvidenceDigest: candidate.EvidenceDigest,
		Bindings: cloneVerifiedBindings(bindings), Capabilities: cloneVerifiedCapabilities(capabilities),
	}
	record := struct {
		SchemaVersion          string               `json:"schema_version"`
		ProviderID             string               `json:"provider_id"`
		HostID                 string               `json:"host_id"`
		DescriptorDigest       string               `json:"descriptor_digest"`
		DistributionID         string               `json:"distribution_id"`
		DistributionRevision   string               `json:"distribution_revision"`
		DistributionTreeDigest string               `json:"distribution_tree_digest"`
		InstallationKey        string               `json:"installation_key"`
		ConfigurationDigest    string               `json:"configuration_digest"`
		BindingInventoryDigest string               `json:"binding_inventory_digest"`
		EvidenceDigest         string               `json:"evidence_digest"`
		Bindings               []VerifiedBinding    `json:"bindings"`
		Capabilities           []VerifiedCapability `json:"capabilities"`
	}{providerInstanceSchemaV4, instance.ProviderID, instance.HostID, instance.DescriptorDigest,
		instance.DistributionID, instance.DistributionRevision, instance.DistributionTreeDigest, instance.InstallationKey,
		instance.ConfigurationDigest, instance.BindingInventoryDigest, instance.EvidenceDigest, instance.Bindings, instance.Capabilities}
	instance.Digest, _, err = canonicaljson.Digest(record)
	return instance, err
}

func distributionByID(values []catalog.DistributionRecord, id string) (catalog.DistributionRecord, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return catalog.DistributionRecord{}, false
}

func observationIdentityKey(value host.BindingObservation) string {
	return bindingObservationKey(value.HostID, value.ProviderID, value.InstallationKey, value.DistributionID, value.BindingID)
}

func bindingObservationKey(hostID, providerID, installationKey, distributionID, bindingID string) string {
	return hostID + "\x00" + providerID + "\x00" + installationKey + "\x00" + distributionID + "\x00" + bindingID
}
