package catalog

import "github.com/wifibaby4u/open-agent-workflow/internal/execution"

const (
	ProviderDescriptorSchemaV4 = "oaw.provider-descriptor/v4"
	ProfileRecipeSchemaV3      = "oaw.profile-recipe/v3"
	ProfileAliasSetSchemaV1    = "oaw.profile-alias-set/v1"
	TaxonomyVersionV1          = "oaw.lifecycle-taxonomy/v1"
)

type RequestMode string

const (
	RequestModeBounded  RequestMode = "BOUNDED"
	RequestModeWorkflow RequestMode = "WORKFLOW"
)

type OwnershipNamespace string

const (
	OwnershipStage      OwnershipNamespace = "stage"
	OwnershipProcedure  OwnershipNamespace = "procedure"
	OwnershipIncident   OwnershipNamespace = "incident"
	OwnershipAssurance  OwnershipNamespace = "assurance"
	OwnershipHostAction OwnershipNamespace = "host-action"
	OwnershipGate       OwnershipNamespace = "gate"
)

type BindingKind string

const (
	BindingSkill       BindingKind = "skill"
	BindingAgent       BindingKind = "agent"
	BindingRole        BindingKind = "role"
	BindingInstruction BindingKind = "instruction"
	BindingTool        BindingKind = "tool"
)

type InvocationDisposition string
type InternalCallMode string

const (
	InvocationHumanExplicit InvocationDisposition = "human-explicit"
	InvocationModel         InvocationDisposition = "model"
	InvocationHost          InvocationDisposition = "host"
	InvocationInternal      InvocationDisposition = "internal"

	InternalCreditOnly     InternalCallMode = "credit-only"
	InternalDispatchBefore InternalCallMode = "dispatch-before"
	InternalDispatchAfter  InternalCallMode = "dispatch-after"
)

type ResponsibilityClaim struct {
	Namespace    OwnershipNamespace `json:"namespace" toml:"namespace"`
	Name         string             `json:"name" toml:"name"`
	SlotID       SlotID             `json:"slot_id" toml:"slot_id"`
	OutcomeOwner bool               `json:"outcome_owner" toml:"outcome_owner"`
}

type DelegationRequirements struct {
	Child          bool `json:"child" toml:"child"`
	ParallelChild  bool `json:"parallel_child" toml:"parallel_child"`
	NestedChild    bool `json:"nested_child" toml:"nested_child"`
	NestedParallel bool `json:"nested_parallel_child" toml:"nested_parallel_child"`
}

type InternalCall struct {
	BindingID string           `json:"binding_id" toml:"binding_id"`
	Required  bool             `json:"required" toml:"required"`
	Mode      InternalCallMode `json:"mode" toml:"mode"`
	StageSpan []SlotID         `json:"stage_span" toml:"stage_span"`
}

type ProviderDescriptorRecord struct {
	SchemaVersion     string               `json:"schema_version" toml:"schema_version"`
	DescriptorVersion string               `json:"descriptor_version" toml:"descriptor_version"`
	ID                string               `json:"id" toml:"id"`
	DisplayName       string               `json:"display_name" toml:"display_name"`
	Distributions     []DistributionRecord `json:"distributions" toml:"distributions"`
	Discovery         []DiscoveryProbe     `json:"discovery" toml:"discovery"`
	Bindings          []BindingRecord      `json:"bindings" toml:"bindings"`
	Capabilities      []CapabilityRecord   `json:"capabilities" toml:"capabilities"`
}

type DistributionRecord struct {
	ID         string `json:"id" toml:"id"`
	SourceURI  string `json:"source_uri" toml:"source_uri"`
	Revision   string `json:"revision" toml:"revision"`
	TreeDigest string `json:"tree_digest" toml:"tree_digest"`
}

type DiscoveryProbe struct {
	ID             string   `json:"id" toml:"id"`
	Hosts          []string `json:"hosts" toml:"hosts"`
	Surface        string   `json:"surface" toml:"surface"`
	DistributionID string   `json:"distribution_id" toml:"distribution_id"`
	Kind           string   `json:"kind" toml:"kind"`
	Root           string   `json:"root" toml:"root"`
	CandidatePath  string   `json:"candidate_path,omitempty" toml:"candidate_path"`
	EvidencePath   string   `json:"evidence_path,omitempty" toml:"evidence_path"`
	Prefix         string   `json:"prefix,omitempty" toml:"prefix"`
}

type BindingRecord struct {
	ID                  string                 `json:"id" toml:"id"`
	DistributionID      string                 `json:"distribution_id" toml:"distribution_id"`
	ContentRoot         string                 `json:"content_root" toml:"content_root"`
	InstallRoot         string                 `json:"install_root" toml:"install_root"`
	TreeDigest          string                 `json:"tree_digest" toml:"tree_digest"`
	Host                string                 `json:"host" toml:"host"`
	Surface             string                 `json:"surface" toml:"surface"`
	Kind                BindingKind            `json:"kind" toml:"kind"`
	Reference           string                 `json:"reference" toml:"reference"`
	Invocation          InvocationDisposition  `json:"invocation" toml:"invocation"`
	Responsibilities    []ResponsibilityClaim  `json:"responsibilities" toml:"responsibilities"`
	InputArtifact       string                 `json:"input_artifact" toml:"input_artifact"`
	OutputArtifact      string                 `json:"output_artifact" toml:"output_artifact"`
	MaximumEffects      []string               `json:"maximum_effects" toml:"maximum_effects"`
	Resources           []string               `json:"resources" toml:"resources"`
	SupportedTopologies []execution.Topology   `json:"supported_topologies" toml:"supported_topologies"`
	Delegation          DelegationRequirements `json:"delegation" toml:"delegation"`
	StageSpan           []SlotID               `json:"stage_span" toml:"stage_span"`
	InternalCalls       []InternalCall         `json:"internal_calls" toml:"internal_calls"`
	Alternatives        []string               `json:"alternatives" toml:"alternatives"`
	Conflicts           []string               `json:"conflicts" toml:"conflicts"`
}

type CapabilityRecord struct {
	ID            string        `json:"id" toml:"id"`
	InputSchema   string        `json:"input_schema" toml:"input_schema"`
	OutcomeSchema string        `json:"outcome_schema" toml:"outcome_schema"`
	RequestModes  []RequestMode `json:"request_modes" toml:"request_modes"`
	BindingRefs   []string      `json:"binding_refs" toml:"binding_refs"`
}

type ProfileRecipeRecord struct {
	SchemaVersion           string                             `json:"schema_version" toml:"schema_version"`
	TaxonomyVersion         string                             `json:"taxonomy_version" toml:"taxonomy_version"`
	RecipeVersion           string                             `json:"recipe_version" toml:"recipe_version"`
	ID                      string                             `json:"id" toml:"id"`
	DisplayName             string                             `json:"display_name" toml:"display_name"`
	Family                  string                             `json:"family" toml:"family"`
	Template                string                             `json:"template,omitempty" toml:"template"`
	Slots                   []SlotRecipe                       `json:"slots" toml:"slots"`
	AddOns                  []AddOnRecord                      `json:"add_ons" toml:"add_ons"`
	IncidentRoutes          []IncidentRoute                    `json:"incident_routes" toml:"incident_routes"`
	Overlays                []OverlayRecord                    `json:"overlays" toml:"overlays"`
	StableBoundaries        []string                           `json:"stable_boundaries" toml:"stable_boundaries"`
	EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements" toml:"environment_requirements"`
}

type SlotRecipe struct {
	SlotID        SlotID             `json:"slot_id" toml:"slot_id"`
	Applicability SlotApplicability  `json:"applicability" toml:"applicability"`
	OutcomeOwner  OutcomeOwner       `json:"outcome_owner" toml:"outcome_owner"`
	Pipeline      []PipelineStep     `json:"pipeline" toml:"pipeline"`
	HostAction    *HostActionRef     `json:"host_action,omitempty" toml:"host_action"`
	Gates         []GateRecord       `json:"gates" toml:"gates"`
	Transitions   []RecipeTransition `json:"transitions" toml:"transitions"`
}

type BindingSelector struct {
	ProviderID string `json:"provider_id" toml:"provider_id"`
	BindingID  string `json:"binding_id" toml:"binding_id"`
}

type PipelineStep struct {
	ID                     string          `json:"id" toml:"id"`
	Selector               BindingSelector `json:"binding_selector" toml:"binding_selector"`
	StageSpan              []SlotID        `json:"stage_span" toml:"stage_span"`
	RequiredInputArtifact  string          `json:"required_input_artifact" toml:"required_input_artifact"`
	ProducedOutputArtifact string          `json:"produced_output_artifact" toml:"produced_output_artifact"`
}

type OutcomeOwner struct {
	Kind       OutcomeOwnerKind `json:"kind" toml:"kind"`
	StepID     string           `json:"step_id,omitempty" toml:"step_id"`
	HostAction string           `json:"host_action,omitempty" toml:"host_action"`
}

type HostActionRef struct {
	ID             string `json:"id" toml:"id"`
	InputArtifact  string `json:"input_artifact" toml:"input_artifact"`
	OutputArtifact string `json:"output_artifact" toml:"output_artifact"`
}

type EvidenceRequirementRecord struct {
	Kind        string `json:"kind" toml:"kind"`
	Minimum     uint64 `json:"minimum" toml:"minimum"`
	Description string `json:"description" toml:"description"`
}

type GateRecord struct {
	ID                   string                      `json:"id" toml:"id"`
	Authority            GateAuthority               `json:"authority" toml:"authority"`
	Predicate            string                      `json:"predicate" toml:"predicate"`
	EvidenceRequirements []EvidenceRequirementRecord `json:"evidence_requirements" toml:"evidence_requirements"`
}

type RecipeTransition struct {
	Signal string `json:"signal" toml:"signal"`
	Target SlotID `json:"target" toml:"target"`
}

type IncidentRoute struct {
	IncidentType  string           `json:"incident_type" toml:"incident_type"`
	Handler       BindingSelector  `json:"handler" toml:"handler"`
	ReturnTo      SlotID           `json:"return_to" toml:"return_to"`
	IfUnavailable IncidentFallback `json:"if_unavailable" toml:"if_unavailable"`
}

type OverlayRecord struct {
	ID                  string            `json:"id" toml:"id"`
	Precedence          []string          `json:"precedence" toml:"precedence"`
	PausedBindings      []BindingSelector `json:"paused_bindings" toml:"paused_bindings"`
	SelectedAlternative string            `json:"selected_alternative" toml:"selected_alternative"`
	Rationale           string            `json:"rationale" toml:"rationale"`
}

type AddOnRecord struct {
	ID                   string                      `json:"id" toml:"id"`
	Kind                 AddOnKind                   `json:"kind" toml:"kind"`
	Selector             BindingSelector             `json:"binding_selector" toml:"binding_selector"`
	SlotID               SlotID                      `json:"slot_id" toml:"slot_id"`
	IncidentTypes        []string                    `json:"incident_types" toml:"incident_types"`
	EvidenceRequirements []EvidenceRequirementRecord `json:"evidence_requirements" toml:"evidence_requirements"`
}

type SlotApplicability string
type OutcomeOwnerKind string
type GateAuthority string
type IncidentFallback string
type AddOnKind string

const (
	SlotMandatory   SlotApplicability = "mandatory"
	SlotConditional SlotApplicability = "conditional"

	OwnerProviderBinding OutcomeOwnerKind = "provider-binding"
	OwnerHostAction      OutcomeOwnerKind = "host-action"
	OwnerNone            OutcomeOwnerKind = "none"

	GateOAWCore GateAuthority = "oaw-core"
	GateHost    GateAuthority = "host"
	GateUser    GateAuthority = "user"

	IncidentStop   IncidentFallback = "stop"
	IncidentReplan IncidentFallback = "replan"

	AddOnIncidentHandler AddOnKind = "incident-handler"
	AddOnSpecialistCheck AddOnKind = "specialist-check"
)

type ProfileAliasSetRecord struct {
	SchemaVersion string               `json:"schema_version"`
	Aliases       []ProfileAliasRecord `json:"aliases"`
}

type ProfileAliasRecord struct {
	Alias    string `json:"alias"`
	RecipeID string `json:"recipe_id"`
}
