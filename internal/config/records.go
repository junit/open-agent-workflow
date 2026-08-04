package config

const (
	UserConfigSchemaV2    = "oaw.user-config/v2"
	ProjectConfigSchemaV1 = "oaw.project-config/v1"
)

type ContentReference struct {
	ID      string `json:"id" toml:"id"`
	Path    string `json:"path" toml:"path"`
	Replace bool   `json:"replace" toml:"replace"`
}

type ProviderPin struct {
	ProviderID      string `json:"provider_id" toml:"provider_id"`
	HostID          string `json:"host_id" toml:"host_id"`
	InstallationKey string `json:"installation_key" toml:"installation_key"`
	EvidenceDigest  string `json:"evidence_digest" toml:"evidence_digest"`
	Location        string `json:"location,omitempty" toml:"location"`
	Version         string `json:"version,omitempty" toml:"version"`
}

type ProviderInstallation struct {
	ProviderID       string `json:"provider_id" toml:"provider_id"`
	HostID           string `json:"host_id" toml:"host_id"`
	SurfaceID        string `json:"surface_id" toml:"surface_id"`
	Location         string `json:"location" toml:"location"`
	DiscoveryProbeID string `json:"discovery_probe_id" toml:"discovery_probe_id"`
}

type BindingPreference struct {
	ProviderID   string `json:"provider_id" toml:"provider_id"`
	CapabilityID string `json:"capability_id" toml:"capability_id"`
	HostID       string `json:"host_id" toml:"host_id"`
	Kind         string `json:"kind" toml:"kind"`
	Reference    string `json:"reference" toml:"reference"`
}

type BoundedCapabilityDefault struct {
	ID           string `json:"id" toml:"id"`
	ProviderID   string `json:"provider_id" toml:"provider_id"`
	CapabilityID string `json:"capability_id" toml:"capability_id"`
}

type ProjectTrust struct {
	Root              string   `json:"root" toml:"root"`
	ConfigDigest      string   `json:"config_digest" toml:"config_digest"`
	DescriptorDigests []string `json:"descriptor_digests" toml:"descriptor_digests"`
	RecipeDigests     []string `json:"recipe_digests" toml:"recipe_digests"`
}

type UserConfigRecord struct {
	SchemaVersion             string                     `json:"schema_version" toml:"schema_version"`
	DeniedProviders           []string                   `json:"denied_providers" toml:"denied_providers"`
	ProviderDescriptors       []ContentReference         `json:"provider_descriptors" toml:"provider_descriptors"`
	ProfileRecipes            []ContentReference         `json:"profile_recipes" toml:"profile_recipes"`
	HostIntegrations          []ContentReference         `json:"host_integrations" toml:"host_integrations"`
	ProviderInstallations     []ProviderInstallation     `json:"provider_installations" toml:"provider_installations"`
	ProviderPins              []ProviderPin              `json:"provider_pins" toml:"provider_pins"`
	BindingPreferences        []BindingPreference        `json:"binding_preferences" toml:"binding_preferences"`
	BoundedCapabilityDefaults []BoundedCapabilityDefault `json:"bounded_capability_defaults" toml:"bounded_capability_defaults"`
	ProjectTrust              []ProjectTrust             `json:"project_trust" toml:"project_trust"`
}

type CapabilityLimit struct {
	ProviderID    string   `json:"provider_id" toml:"provider_id"`
	CapabilityIDs []string `json:"capability_ids" toml:"capability_ids"`
}

type ProjectConfigRecord struct {
	SchemaVersion        string             `json:"schema_version" toml:"schema_version"`
	RequiredProviders    []string           `json:"required_providers" toml:"required_providers"`
	RecommendedProviders []string           `json:"recommended_providers" toml:"recommended_providers"`
	ProviderDescriptors  []ContentReference `json:"provider_descriptors" toml:"provider_descriptors"`
	ProfileRecipes       []ContentReference `json:"profile_recipes" toml:"profile_recipes"`
	CapabilityLimits     []CapabilityLimit  `json:"capability_limits" toml:"capability_limits"`
}

type Decoded[T any] struct {
	Record        T
	CanonicalJSON []byte
	Digest        string
}
