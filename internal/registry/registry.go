package registry

import (
	"fmt"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const effectiveRegistrySchemaV4 = "oaw.effective-registry/v4"

type Registry struct {
	hostID        string
	providers     []ProviderInstance
	providerIndex map[string]int
	bindingIndex  map[string]map[string]int
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

func (registry Registry) Binding(providerID, bindingID string) (VerifiedBinding, bool) {
	providerBindings, found := registry.bindingIndex[providerID]
	if !found {
		return VerifiedBinding{}, false
	}
	index, found := providerBindings[bindingID]
	if !found {
		return VerifiedBinding{}, false
	}
	value := registry.providers[registry.providerIndex[providerID]].Bindings[index]
	value.SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
	return value, true
}

func (registry Registry) Bindings(providerID string) []VerifiedBinding {
	index, found := registry.providerIndex[providerID]
	if !found {
		return []VerifiedBinding{}
	}
	return cloneVerifiedBindings(registry.providers[index].Bindings)
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
	value := capabilities[index]
	value.BindingIDs = append([]string{}, value.BindingIDs...)
	return value, true
}

func (registry Registry) Digest() string { return registry.digest }

func newRegistry(hostID string, values []ProviderInstance) (Registry, error) {
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: invalid Host ID %q: %w", hostID, err)
	}
	providers := cloneProviderInstances(values)
	sort.Slice(providers, func(i, j int) bool { return providers[i].ProviderID < providers[j].ProviderID })
	providerIndex := make(map[string]int, len(providers))
	bindingIndex := make(map[string]map[string]int, len(providers))
	for providerIndexValue := range providers {
		provider := &providers[providerIndexValue]
		if provider.HostID != hostID {
			return Registry{}, fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: Provider %s belongs to Host %q, not %q", provider.ProviderID, provider.HostID, hostID)
		}
		if _, found := providerIndex[provider.ProviderID]; found {
			return Registry{}, fmt.Errorf("DUPLICATE_PROVIDER_INSTANCE: %s", provider.ProviderID)
		}
		providerIndex[provider.ProviderID] = providerIndexValue
		sort.Slice(provider.Bindings, func(left, right int) bool {
			return provider.Bindings[left].BindingID < provider.Bindings[right].BindingID
		})
		sort.Slice(provider.Capabilities, func(left, right int) bool { return provider.Capabilities[left].ID < provider.Capabilities[right].ID })
		for capabilityIndex := range provider.Capabilities {
			sort.Strings(provider.Capabilities[capabilityIndex].BindingIDs)
		}
		bindings := make(map[string]int, len(provider.Bindings))
		for bindingIndexValue, binding := range provider.Bindings {
			if _, found := bindings[binding.BindingID]; found {
				return Registry{}, fmt.Errorf("DUPLICATE_PROVIDER_BINDING: %s/%s", provider.ProviderID, binding.BindingID)
			}
			bindings[binding.BindingID] = bindingIndexValue
		}
		bindingIndex[provider.ProviderID] = bindings
	}
	record := struct {
		SchemaVersion string             `json:"schema_version"`
		HostID        string             `json:"host_id"`
		Providers     []ProviderInstance `json:"providers"`
	}{effectiveRegistrySchemaV4, hostID, providers}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return Registry{}, err
	}
	return Registry{hostID: hostID, providers: providers, providerIndex: providerIndex, bindingIndex: bindingIndex, digest: digest}, nil
}

func cloneProviderInstances(values []ProviderInstance) []ProviderInstance {
	result := make([]ProviderInstance, len(values))
	for index, value := range values {
		result[index] = cloneProviderInstance(value)
	}
	return result
}
