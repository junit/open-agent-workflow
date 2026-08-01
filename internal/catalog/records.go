package catalog

const (
	ProviderDescriptorSchemaV1 = "oaw.provider-descriptor/v1"
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
	SchemaVersion     string             `json:"schema_version"`
	DescriptorVersion string             `json:"descriptor_version"`
	ID                string             `json:"id"`
	DisplayName       string             `json:"display_name"`
	Discovery         []DiscoveryProbe   `json:"discovery"`
	Capabilities      []CapabilityRecord `json:"capabilities"`
}

type DiscoveryProbe struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Root   string   `json:"root"`
	Path   string   `json:"path,omitempty"`
	Prefix string   `json:"prefix,omitempty"`
	Suffix string   `json:"suffix,omitempty"`
	Paths  []string `json:"paths,omitempty"`
}

type CapabilityRecord struct {
	ID                  string           `json:"id"`
	InputSchema         string           `json:"input_schema"`
	OutcomeSchema       string           `json:"outcome_schema"`
	MaximumEffects      []string         `json:"maximum_effects"`
	Resources           []string         `json:"resources"`
	RequestModes        []RequestMode    `json:"request_modes"`
	Responsibilities    []string         `json:"responsibilities"`
	ExecutorTopology    ExecutorTopology `json:"executor_topology"`
	DelegationAllowList []string         `json:"delegation_allow_list"`
	HostBindings        []HostBinding    `json:"host_bindings"`
}

type HostBinding struct {
	Host      string `json:"host"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
}

type ProfileRecipeRecord struct {
	SchemaVersion            string          `json:"schema_version"`
	RecipeVersion            string          `json:"recipe_version"`
	ID                       string          `json:"id"`
	DisplayName              string          `json:"display_name"`
	RequiredResponsibilities []string        `json:"required_responsibilities"`
	Nodes                    []RecipeNode    `json:"nodes"`
	IncidentRoutes           []IncidentRoute `json:"incident_routes"`
	Entry                    string          `json:"entry"`
	TerminalGates            []string        `json:"terminal_gates"`
	StableBoundaries         []string        `json:"stable_boundaries"`
}

type RecipeNode struct {
	ID             string             `json:"id"`
	Kind           NodeKind           `json:"kind"`
	Responsibility string             `json:"responsibility"`
	Selector       CapabilitySelector `json:"selector"`
	Phase          string             `json:"phase,omitempty"`
	Optional       bool               `json:"optional,omitempty"`
	Transitions    []RecipeTransition `json:"transitions"`
}

type RecipeTransition struct {
	Signal string `json:"signal"`
	Target string `json:"target"`
}

type IncidentRoute struct {
	Incident string `json:"incident"`
	Handler  string `json:"handler"`
}

type CapabilitySelector struct {
	ProviderID   string `json:"provider_id"`
	CapabilityID string `json:"capability_id"`
}

type ProfileAliasSetRecord struct {
	SchemaVersion string               `json:"schema_version"`
	Aliases       []ProfileAliasRecord `json:"aliases"`
}

type ProfileAliasRecord struct {
	Alias    string `json:"alias"`
	RecipeID string `json:"recipe_id"`
}
