package registry

import (
	"fmt"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

const effectiveRegistrySchemaV3 = "oaw.effective-registry/v3"

type Registry struct {
	hostID        string
	providers     []ProviderInstance
	providerIndex map[string]int
	digest        string
}

func (registry Registry) HostID() string { return registry.hostID }

func (registry Registry) Providers() []ProviderInstance {
	return cloneProviderInstances(registry.providers)
}

func (registry Registry) Provider(id string) (ProviderInstance, bool) {
	index, found := registry.providerIndex[id]
	if !found {
		return ProviderInstance{}, false
	}
	return cloneProviderInstance(registry.providers[index]), true
}

func (registry Registry) Capability(providerID, capabilityID string) (VerifiedCapability, bool) {
	providerIndex, found := registry.providerIndex[providerID]
	if !found {
		return VerifiedCapability{}, false
	}
	capabilities := registry.providers[providerIndex].Capabilities
	index := sort.Search(len(capabilities), func(i int) bool { return capabilities[i].ID >= capabilityID })
	if index == len(capabilities) || capabilities[index].ID != capabilityID {
		return VerifiedCapability{}, false
	}
	return cloneVerifiedCapability(capabilities[index]), true
}

func (registry Registry) Digest() string { return registry.digest }

func newRegistry(hostID string, values []ProviderInstance) (Registry, error) {
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: invalid Host ID %q: %w", hostID, err)
	}
	providers := cloneProviderInstances(values)
	for i := range providers {
		sort.Slice(providers[i].Capabilities, func(left, right int) bool {
			return providers[i].Capabilities[left].ID < providers[i].Capabilities[right].ID
		})
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].ProviderID < providers[j].ProviderID })
	index := make(map[string]int, len(providers))
	for i, provider := range providers {
		if provider.HostID != hostID {
			return Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: Provider %s belongs to Host %q, not %q", provider.ProviderID, provider.HostID, hostID)
		}
		if _, found := index[provider.ProviderID]; found {
			return Registry{}, fmt.Errorf("DUPLICATE_PROVIDER_INSTANCE: %s", provider.ProviderID)
		}
		index[provider.ProviderID] = i
	}
	record := struct {
		SchemaVersion string             `json:"schema_version"`
		HostID        string             `json:"host_id"`
		Providers     []ProviderInstance `json:"providers"`
	}{effectiveRegistrySchemaV3, hostID, providers}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return Registry{}, err
	}
	return Registry{hostID: hostID, providers: providers, providerIndex: index, digest: digest}, nil
}

func cloneProviderInstances(values []ProviderInstance) []ProviderInstance {
	result := make([]ProviderInstance, len(values))
	for i, value := range values {
		result[i] = cloneProviderInstance(value)
	}
	return result
}
