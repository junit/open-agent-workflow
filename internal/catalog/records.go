package catalog

const (
	ProviderDescriptorSchemaV2 = "oaw.provider-descriptor/v2"
	ProfileRecipeSchemaV1      = "oaw.profile-recipe/v1"
	ProfileAliasSetSchemaV1    = "oaw.profile-alias-set/v1"
)

type RequestMode string

const (
	RequestModeBounded  RequestMode = "BOUNDED"
	RequestModeWorkflow RequestMode = "WORKFLOW"
)

type ExecutorTopology string

const (
	MainAgentAllowed ExecutorTopology = "main-agent-allowed"
	IsolatedRequired ExecutorTopology = "isolated-required"
)

type NodeKind string

const (
	PhaseNode           NodeKind = "phase"
	ProcedureNode       NodeKind = "procedure"
	IncidentHandlerNode NodeKind = "incident-handler"
	CheckpointNode      NodeKind = "checkpoint"
	GateNode            NodeKind = "gate"
)

type ProviderDescriptorRecord struct {
	SchemaVersion     string             `json:"schema_version" toml:"schema_version"`
	DescriptorVersion string             `json:"descriptor_version" toml:"descriptor_version"`
	ID                string             `json:"id" toml:"id"`
	DisplayName       string             `json:"display_name" toml:"display_name"`
	Discovery         []DiscoveryProbe   `json:"discovery" toml:"discovery"`
	Capabilities      []CapabilityRecord `json:"capabilities" toml:"capabilities"`
}

type DiscoveryProbe struct {
	ID            string   `json:"id" toml:"id"`
	Hosts         []string `json:"hosts" toml:"hosts"`
	Surface       string   `json:"surface" toml:"surface"`
	Distribution  string   `json:"distribution" toml:"distribution"`
	Kind          string   `json:"kind" toml:"kind"`
	Root          string   `json:"root" toml:"root"`
	CandidatePath string   `json:"candidate_path,omitempty" toml:"candidate_path"`
	EvidencePath  string   `json:"evidence_path,omitempty" toml:"evidence_path"`
	Prefix        string   `json:"prefix,omitempty" toml:"prefix"`
}

type CapabilityRecord struct {
	ID                  string           `json:"id" toml:"id"`
	InputSchema         string           `json:"input_schema" toml:"input_schema"`
	OutcomeSchema       string           `json:"outcome_schema" toml:"outcome_schema"`
	MaximumEffects      []string         `json:"maximum_effects" toml:"maximum_effects"`
	Resources           []string         `json:"resources" toml:"resources"`
	RequestModes        []RequestMode    `json:"request_modes" toml:"request_modes"`
	Responsibilities    []string         `json:"responsibilities" toml:"responsibilities"`
	ExecutorTopology    ExecutorTopology `json:"executor_topology" toml:"executor_topology"`
	DelegationAllowList []string         `json:"delegation_allow_list" toml:"delegation_allow_list"`
	HostBindings        []HostBinding    `json:"host_bindings" toml:"host_bindings"`
}

type HostBinding struct {
	Host      string `json:"host" toml:"host"`
	Kind      string `json:"kind" toml:"kind"`
	Reference string `json:"reference" toml:"reference"`
}

type ProfileRecipeRecord struct {
	SchemaVersion            string          `json:"schema_version" toml:"schema_version"`
	RecipeVersion            string          `json:"recipe_version" toml:"recipe_version"`
	ID                       string          `json:"id" toml:"id"`
	DisplayName              string          `json:"display_name" toml:"display_name"`
	RequiredResponsibilities []string        `json:"required_responsibilities" toml:"required_responsibilities"`
	Nodes                    []RecipeNode    `json:"nodes" toml:"nodes"`
	IncidentRoutes           []IncidentRoute `json:"incident_routes" toml:"incident_routes"`
	Entry                    string          `json:"entry" toml:"entry"`
	TerminalGates            []string        `json:"terminal_gates" toml:"terminal_gates"`
	StableBoundaries         []string        `json:"stable_boundaries" toml:"stable_boundaries"`
}

type RecipeNode struct {
	ID             string             `json:"id" toml:"id"`
	Kind           NodeKind           `json:"kind" toml:"kind"`
	Responsibility string             `json:"responsibility" toml:"responsibility"`
	Selector       CapabilitySelector `json:"selector" toml:"selector"`
	Phase          string             `json:"phase,omitempty" toml:"phase"`
	Optional       bool               `json:"optional,omitempty" toml:"optional"`
	Transitions    []RecipeTransition `json:"transitions" toml:"transitions"`
}

type RecipeTransition struct {
	Signal string `json:"signal" toml:"signal"`
	Target string `json:"target" toml:"target"`
}

type IncidentRoute struct {
	Incident string `json:"incident" toml:"incident"`
	Handler  string `json:"handler" toml:"handler"`
}

type CapabilitySelector struct {
	ProviderID   string `json:"provider_id" toml:"provider_id"`
	CapabilityID string `json:"capability_id" toml:"capability_id"`
}

type ProfileAliasSetRecord struct {
	SchemaVersion string               `json:"schema_version"`
	Aliases       []ProfileAliasRecord `json:"aliases"`
}

type ProfileAliasRecord struct {
	Alias    string `json:"alias"`
	RecipeID string `json:"recipe_id"`
}
