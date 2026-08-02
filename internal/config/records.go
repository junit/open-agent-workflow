package config

const (
	UserConfigSchemaV1    = "oaw.user-config/v1"
	ProjectConfigSchemaV1 = "oaw.project-config/v1"
)

type ContentReference struct {
	ID      string `json:"id" toml:"id"`
	Path    string `json:"path" toml:"path"`
	Replace bool   `json:"replace" toml:"replace"`
}

type ProviderPin struct {
	ID       string `json:"id" toml:"id"`
	Location string `json:"location" toml:"location"`
	Version  string `json:"version" toml:"version"`
}

type BindingPreference struct {
	ProviderID   string `json:"provider_id" toml:"provider_id"`
	CapabilityID string `json:"capability_id" toml:"capability_id"`
	Host         string `json:"host" toml:"host"`
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
