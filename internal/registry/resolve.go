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

const providerInstanceSchemaV3 = "oaw.provider-instance/v3"

type verifiedBindingCandidate struct {
	binding     catalog.HostBinding
	observation host.BindingObservation
}

func Resolve(snapshot config.Snapshot, hostID string, evidence discovery.Report, inventory *host.BindingInventory) (ResolutionReport, Registry, error) {
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return ResolutionReport{}, Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: invalid Host ID %q: %w", hostID, err)
	}
	if evidence.HostID() != hostID {
		return ResolutionReport{}, Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: discovery Host %q does not match selected Host %q", evidence.HostID(), hostID)
	}
	if inventory != nil && inventory.HostID != hostID {
		return ResolutionReport{}, Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: inventory Host %q does not match selected Host %q", inventory.HostID, hostID)
	}

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
		settings := snapshot.ProviderSettings(providerID, hostID)
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
		if inventory == nil || len(inventory.Observations) == 0 {
			resolution.State = CandidateState
			resolution.Reason = "HOST_BINDING_EVIDENCE_REQUIRED"
			resolutions = append(resolutions, resolution)
			continue
		}
		capabilities := resolveCapabilities(descriptor, settings, selected[0], *inventory)
		if len(capabilities) == 0 {
			resolution.State = BindingUnavailable
			resolution.Reason = "PROVIDER_BINDING_UNAVAILABLE"
			resolutions = append(resolutions, resolution)
			continue
		}
		instance, err := buildProviderInstance(descriptor, settings, selected[0], inventory.Digest, capabilities)
		if err != nil {
			return ResolutionReport{}, Registry{}, err
		}
		resolution.State = Verified
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

func resolveCapabilities(descriptor catalog.ProviderDescriptorRecord, settings config.ProviderSettings, candidate discovery.Candidate, inventory host.BindingInventory) []VerifiedCapability {
	allowed := make(map[string]struct{}, len(settings.CapabilityLimit))
	for _, id := range settings.CapabilityLimit {
		allowed[id] = struct{}{}
	}
	observed := make(map[string]host.BindingObservation, len(inventory.Observations))
	for _, observation := range inventory.Observations {
		if observation.HostID != inventory.HostID || observation.HostID != candidate.HostID || observation.InstallationKey != candidate.InstallationKey {
			continue
		}
		observed[bindingKey(observation.Binding)] = observation
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
		bindings := make([]verifiedBindingCandidate, 0, len(capability.HostBindings))
		for _, binding := range capability.HostBindings {
			if binding.Host != candidate.HostID {
				continue
			}
			if observation, found := observed[bindingKey(binding)]; found && bindingEqual(observation.Binding, binding) {
				bindings = append(bindings, verifiedBindingCandidate{binding: binding, observation: observation})
			}
		}
		if len(bindings) == 0 {
			continue
		}
		sort.Slice(bindings, func(i, j int) bool {
			return bindingKey(bindings[i].binding) < bindingKey(bindings[j].binding)
		})
		selected := bindings[0]
		if preference, found := preferences[capability.ID]; found {
			for _, candidate := range bindings {
				if bindingIdentityEqual(candidate.binding, preference.HostID, preference.Kind, preference.Reference) {
					selected = candidate
					break
				}
			}
		}
		result = append(result, VerifiedCapability{
			ID: capability.ID, Binding: cloneBinding(selected.binding),
			SupportedTopologies:   append([]execution.Topology{}, selected.binding.Topologies...),
			BindingEvidenceDigest: selected.observation.Digest,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func buildProviderInstance(descriptor catalog.ProviderDescriptorRecord, settings config.ProviderSettings, candidate discovery.Candidate, inventoryDigest string, capabilities []VerifiedCapability) (ProviderInstance, error) {
	descriptorDigest, _, err := canonicaljson.Digest(descriptor)
	if err != nil {
		return ProviderInstance{}, err
	}
	instance := ProviderInstance{
		ProviderID: descriptor.ID, HostID: candidate.HostID, DescriptorDigest: descriptorDigest,
		DistributionKey: candidate.DistributionKey, InstallationKey: candidate.InstallationKey,
		Location: candidate.Location, Version: candidate.Version, ConfigurationDigest: settings.Digest,
		BindingInventoryDigest: inventoryDigest, EvidenceDigest: candidate.EvidenceDigest,
		Capabilities: cloneVerifiedCapabilities(capabilities),
	}
	record := struct {
		SchemaVersion          string               `json:"schema_version"`
		ProviderID             string               `json:"provider_id"`
		HostID                 string               `json:"host_id"`
		DescriptorDigest       string               `json:"descriptor_digest"`
		DistributionKey        string               `json:"distribution_key"`
		InstallationKey        string               `json:"installation_key"`
		Location               string               `json:"location"`
		Version                string               `json:"version"`
		ConfigurationDigest    string               `json:"configuration_digest"`
		BindingInventoryDigest string               `json:"binding_inventory_digest"`
		EvidenceDigest         string               `json:"evidence_digest"`
		Capabilities           []VerifiedCapability `json:"capabilities"`
	}{providerInstanceSchemaV3, instance.ProviderID, instance.HostID, instance.DescriptorDigest,
		instance.DistributionKey, instance.InstallationKey, instance.Location, instance.Version,
		instance.ConfigurationDigest, instance.BindingInventoryDigest, instance.EvidenceDigest, instance.Capabilities}
	instance.Digest, _, err = canonicaljson.Digest(record)
	return instance, err
}

func bindingKey(value catalog.HostBinding) string {
	return value.Host + "\x00" + value.Kind + "\x00" + value.Reference
}

func bindingEqual(left, right catalog.HostBinding) bool {
	return bindingIdentityEqual(left, right.Host, right.Kind, right.Reference) && slices.Equal(left.Topologies, right.Topologies)
}

func bindingIdentityEqual(value catalog.HostBinding, hostID, kind, reference string) bool {
	return value.Host == hostID && value.Kind == kind && value.Reference == reference
}

func cloneBinding(value catalog.HostBinding) catalog.HostBinding {
	value.Topologies = append([]execution.Topology{}, value.Topologies...)
	return value
}

func cloneVerifiedCapabilities(values []VerifiedCapability) []VerifiedCapability {
	result := make([]VerifiedCapability, len(values))
	for index, value := range values {
		result[index] = cloneVerifiedCapability(value)
	}
	return result
}
