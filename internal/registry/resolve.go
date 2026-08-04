package registry

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
)

const (
	providerBindingsSchemaV1 = "oaw.provider-bindings/v1"
	providerInstanceSchemaV1 = "oaw.provider-instance/v1"
)

func Resolve(snapshot config.Snapshot, evidence discovery.Report, inventory *BindingInventory) (ResolutionReport, Registry, error) {
	descriptors := make(map[string]catalog.ProviderDescriptorRecord)
	identities := make(map[string]struct{})
	for _, provider := range snapshot.Catalog().Providers() {
		descriptors[provider.ID] = provider
		identities[provider.ID] = struct{}{}
	}
	untrusted := make(map[string]struct{})
	for _, id := range snapshot.UntrustedProviderIDs() {
		untrusted[id] = struct{}{}
		identities[id] = struct{}{}
	}
	for _, ids := range [][]string{snapshot.RequiredProviders(), snapshot.RecommendedProviders()} {
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
		settings := snapshot.ProviderSettings(providerID, evidence.HostID())
		candidates := evidence.Candidates(providerID)
		resolution := ProviderResolution{ProviderID: providerID, Instance: nil, Candidates: candidates}
		if settings.Disabled {
			resolution.State = Disabled
			resolution.Reason = "PROVIDER_DISABLED_BY_USER"
			resolutions = append(resolutions, resolution)
			continue
		}
		if _, found := untrusted[providerID]; found {
			resolution.State = Untrusted
			resolution.Reason = "PROVIDER_PROJECT_CONTENT_UNTRUSTED"
			resolutions = append(resolutions, resolution)
			continue
		}
		descriptor, descriptorFound := descriptors[providerID]
		if !descriptorFound || len(candidates) == 0 {
			resolution.State = NotFound
			resolution.Reason = "PROVIDER_NOT_FOUND"
			resolutions = append(resolutions, resolution)
			continue
		}
		selected := candidates
		if settings.Pin != nil {
			selected = matchingCandidates(candidates, *settings.Pin)
			if len(selected) == 0 {
				resolution.State = Incompatible
				resolution.Reason = "PROVIDER_PIN_INCOMPATIBLE"
				resolutions = append(resolutions, resolution)
				continue
			}
		}
		if len(selected) != 1 {
			resolution.State = Ambiguous
			resolution.Reason = "PROVIDER_CANDIDATE_AMBIGUOUS"
			resolutions = append(resolutions, resolution)
			continue
		}
		if inventory == nil {
			resolution.State = CandidateState
			resolution.Reason = "PROVIDER_DISCOVERED_UNVERIFIED"
			resolutions = append(resolutions, resolution)
			continue
		}
		capabilities := resolveCapabilities(descriptor, settings, *inventory)
		if len(capabilities) == 0 {
			resolution.State = BindingUnavailable
			resolution.Reason = "PROVIDER_BINDING_UNAVAILABLE"
			resolutions = append(resolutions, resolution)
			continue
		}
		instance, err := buildProviderInstance(descriptor, settings, selected[0], capabilities)
		if err != nil {
			return ResolutionReport{}, Registry{}, err
		}
		resolution.State = Verified
		resolution.Reason = "PROVIDER_VERIFIED"
		resolution.Instance = &instance
		resolutions = append(resolutions, resolution)
		instances = append(instances, instance)
	}
	report, err := newResolutionReport(resolutions)
	if err != nil {
		return ResolutionReport{}, Registry{}, err
	}
	effective, err := newRegistry(instances)
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
		if pin.Location != "" && candidate.Location != pin.Location {
			continue
		}
		if pin.Version != "" && candidate.Version != pin.Version {
			continue
		}
		result = append(result, candidate)
	}
	return result
}

func resolveCapabilities(descriptor catalog.ProviderDescriptorRecord, settings config.ProviderSettings, inventory BindingInventory) []VerifiedCapability {
	allowed := make(map[string]struct{}, len(settings.CapabilityLimit))
	for _, id := range settings.CapabilityLimit {
		allowed[id] = struct{}{}
	}
	available := make(map[string]struct{}, len(inventory.Bindings))
	for _, binding := range inventory.Bindings {
		if binding.Host == inventory.Host {
			available[bindingKey(binding)] = struct{}{}
		}
	}
	preferences := make(map[string]config.BindingPreference, len(settings.Preferences))
	for _, preference := range settings.Preferences {
		preferences[preference.CapabilityID] = preference
	}
	result := make([]VerifiedCapability, 0, len(descriptor.Capabilities))
	for _, capability := range descriptor.Capabilities {
		if len(allowed) != 0 {
			if _, found := allowed[capability.ID]; !found {
				continue
			}
		}
		bindings := make([]catalog.HostBinding, 0, len(capability.HostBindings))
		for _, binding := range capability.HostBindings {
			if binding.Host != inventory.Host {
				continue
			}
			if _, found := available[bindingKey(binding)]; found {
				bindings = append(bindings, binding)
			}
		}
		if len(bindings) == 0 {
			continue
		}
		sort.Slice(bindings, func(i, j int) bool { return bindingKey(bindings[i]) < bindingKey(bindings[j]) })
		selected := bindings[0]
		if preference, found := preferences[capability.ID]; found {
			preferred := catalog.HostBinding{Host: preference.HostID, Kind: preference.Kind, Reference: preference.Reference}
			for _, binding := range bindings {
				if binding == preferred {
					selected = binding
					break
				}
			}
		}
		result = append(result, VerifiedCapability{ID: capability.ID, Binding: selected})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func buildProviderInstance(descriptor catalog.ProviderDescriptorRecord, settings config.ProviderSettings, candidate discovery.Candidate, capabilities []VerifiedCapability) (ProviderInstance, error) {
	descriptorDigest, _, err := canonicaljson.Digest(descriptor)
	if err != nil {
		return ProviderInstance{}, err
	}
	bindingRecord := struct {
		SchemaVersion string               `json:"schema_version"`
		Capabilities  []VerifiedCapability `json:"capabilities"`
	}{providerBindingsSchemaV1, capabilities}
	bindingDigest, _, err := canonicaljson.Digest(bindingRecord)
	if err != nil {
		return ProviderInstance{}, err
	}
	instance := ProviderInstance{
		ProviderID: descriptor.ID, DescriptorDigest: descriptorDigest, Location: candidate.Location,
		Version: candidate.Version, ConfigurationDigest: settings.Digest, BindingDigest: bindingDigest,
		EvidenceDigest: candidate.EvidenceDigest, Capabilities: append([]VerifiedCapability{}, capabilities...),
	}
	record := struct {
		SchemaVersion       string               `json:"schema_version"`
		ProviderID          string               `json:"provider_id"`
		DescriptorDigest    string               `json:"descriptor_digest"`
		Location            string               `json:"location"`
		Version             string               `json:"version"`
		ConfigurationDigest string               `json:"configuration_digest"`
		BindingDigest       string               `json:"binding_digest"`
		EvidenceDigest      string               `json:"evidence_digest"`
		Capabilities        []VerifiedCapability `json:"capabilities"`
	}{
		providerInstanceSchemaV1, instance.ProviderID, instance.DescriptorDigest, instance.Location,
		instance.Version, instance.ConfigurationDigest, instance.BindingDigest, instance.EvidenceDigest,
		instance.Capabilities,
	}
	instance.Digest, _, err = canonicaljson.Digest(record)
	return instance, err
}

func bindingKey(value catalog.HostBinding) string {
	return value.Host + "\x00" + value.Kind + "\x00" + value.Reference
}
