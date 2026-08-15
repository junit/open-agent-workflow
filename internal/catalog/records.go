package catalog

const ProviderDescriptorSchemaV5 = "oaw.provider-descriptor/v5"

type BindingKind string

const (
	BindingSkill       BindingKind = "skill"
	BindingAgent       BindingKind = "agent"
	BindingRole        BindingKind = "role"
	BindingInstruction BindingKind = "instruction"
	BindingTool        BindingKind = "tool"
)

type InvocationDisposition string

const (
	InvocationHumanExplicit InvocationDisposition = "human-explicit"
	InvocationModel         InvocationDisposition = "model"
	InvocationHost          InvocationDisposition = "host"
	InvocationInternal      InvocationDisposition = "internal"
)

// ProviderDescriptorRecord contains only identity and discovery evidence for
// exact Profile Binding claims. Workflow semantics belong to Markdown Profiles.
type ProviderDescriptorRecord struct {
	SchemaVersion     string               `json:"schema_version"`
	DescriptorVersion string               `json:"descriptor_version"`
	ID                string               `json:"id"`
	DisplayName       string               `json:"display_name"`
	Distributions     []DistributionRecord `json:"distributions"`
	Discovery         []DiscoveryProbe     `json:"discovery"`
	Bindings          []BindingRecord      `json:"bindings"`
}

type DistributionRecord struct {
	ID         string `json:"id"`
	SourceURI  string `json:"source_uri"`
	Revision   string `json:"revision"`
	TreeDigest string `json:"tree_digest"`
}

type DiscoveryProbe struct {
	ID             string   `json:"id"`
	Hosts          []string `json:"hosts"`
	Surface        string   `json:"surface"`
	DistributionID string   `json:"distribution_id"`
	Kind           string   `json:"kind"`
	Root           string   `json:"root"`
	CandidatePath  string   `json:"candidate_path,omitempty"`
	EvidencePath   string   `json:"evidence_path,omitempty"`
	Prefix         string   `json:"prefix,omitempty"`
}

type BindingRecord struct {
	ID             string                `json:"id"`
	DistributionID string                `json:"distribution_id"`
	ContentRoot    string                `json:"content_root"`
	InstallRoot    string                `json:"install_root"`
	TreeDigest     string                `json:"tree_digest"`
	Host           string                `json:"host"`
	Surface        string                `json:"surface"`
	Kind           BindingKind           `json:"kind"`
	Reference      string                `json:"reference"`
	Invocation     InvocationDisposition `json:"invocation"`
}
