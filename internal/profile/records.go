package profile

import (
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const (
	ExecutionGraphSchemaV4 = "oaw.execution-graph/v4"
	HostEvidenceSchemaV1   = "oaw.profile-host-evidence/v1"
)

type AlternativeChoice struct {
	SlotID        catalog.SlotID          `json:"slot_id"`
	StepID        string                  `json:"step_id"`
	AlternativeID string                  `json:"alternative_id"`
	Selector      catalog.BindingSelector `json:"selector"`
}

type Selection struct {
	Profile      string              `json:"profile"`
	RecipeID     string              `json:"recipe_id"`
	RecipeDigest string              `json:"recipe_digest"`
	Topology     execution.Topology  `json:"topology"`
	AddOns       []string            `json:"add_ons"`
	Alternatives []AlternativeChoice `json:"alternatives"`
	Overlays     []string            `json:"overlays"`
	Digest       string              `json:"digest"`
}

type CompileRequest struct {
	Profile      string
	Topology     execution.Topology
	AddOns       []string
	Alternatives []AlternativeChoice
	Overlays     []string
	Host         HostEvidence
}

type DispatchDisposition string

const (
	DispatchByCoordinator DispatchDisposition = "dispatch"
	CreditInternalOnly    DispatchDisposition = "credited-internal"
	OmittedBySelection    DispatchDisposition = "omitted"
)

type ResolvedBinding struct {
	Cursor                     execution.GraphCursor          `json:"cursor"`
	UnitID                     string                         `json:"unit_id"`
	StepID                     string                         `json:"step_id"`
	AnchorSlotID               catalog.SlotID                 `json:"anchor_slot_id"`
	SlotIDs                    []catalog.SlotID               `json:"slot_ids"`
	ProviderID                 string                         `json:"provider_id"`
	ProviderInstanceDigest     string                         `json:"provider_instance_digest"`
	BindingID                  string                         `json:"binding_id"`
	DistributionID             string                         `json:"distribution_id"`
	DistributionRevision       string                         `json:"distribution_revision"`
	DistributionTreeDigest     string                         `json:"distribution_tree_digest"`
	Surface                    string                         `json:"surface"`
	Kind                       catalog.BindingKind            `json:"kind"`
	Reference                  string                         `json:"reference"`
	Invocation                 catalog.InvocationDisposition  `json:"invocation"`
	BindingTreeDigest          string                         `json:"binding_tree_digest"`
	InputArtifact              string                         `json:"input_artifact"`
	OutputArtifact             string                         `json:"output_artifact"`
	Responsibilities           []catalog.ResponsibilityClaim  `json:"responsibilities"`
	MaximumEffects             []string                       `json:"maximum_effects"`
	Resources                  []string                       `json:"resources"`
	SupportedTopologies        []execution.Topology           `json:"supported_topologies"`
	Delegation                 catalog.DelegationRequirements `json:"delegation"`
	RequiredFeatures           []host.FeatureID               `json:"required_features"`
	FeatureEvidenceDigests     []string                       `json:"feature_evidence_digests"`
	Disposition                DispatchDisposition            `json:"disposition"`
	MacroMode                  catalog.InternalCallMode       `json:"macro_mode,omitempty"`
	ParentUnitID               string                         `json:"parent_unit_id,omitempty"`
	RequiresExplicitInvocation bool                           `json:"requires_explicit_invocation"`
	BindingEvidenceDigest      string                         `json:"binding_evidence_digest"`
}

type CompiledOwner struct {
	Kind         catalog.OutcomeOwnerKind `json:"kind"`
	UnitID       string                   `json:"unit_id"`
	ProviderID   string                   `json:"provider_id,omitempty"`
	BindingID    string                   `json:"binding_id,omitempty"`
	HostActionID string                   `json:"host_action_id,omitempty"`
}

type CompiledHostAction struct {
	Cursor            execution.GraphCursor `json:"cursor"`
	ID                string                `json:"id"`
	InputArtifact     string                `json:"input_artifact"`
	OutputArtifact    string                `json:"output_artifact"`
	InputSchema       string                `json:"input_schema"`
	OutcomeSchema     string                `json:"outcome_schema"`
	MaximumEffects    []string              `json:"maximum_effects"`
	Resources         []string              `json:"resources"`
	ObservationDigest string                `json:"observation_digest"`
}

type CompiledGate struct {
	Cursor               execution.GraphCursor               `json:"cursor"`
	ID                   string                              `json:"id"`
	Authority            catalog.GateAuthority               `json:"authority"`
	Predicate            string                              `json:"predicate"`
	EvidenceRequirements []catalog.EvidenceRequirementRecord `json:"evidence_requirements"`
}

type GraphTransition struct {
	Signal string         `json:"signal"`
	Target catalog.SlotID `json:"target"`
}

type GraphProviderInstance struct {
	ProviderID     string `json:"provider_id"`
	HostID         string `json:"host_id"`
	InstanceDigest string `json:"instance_digest"`
}

type CompiledIncidentRoute struct {
	IncidentType    string                   `json:"incident_type"`
	HandlerSlotID   catalog.SlotID           `json:"handler_slot_id"`
	HandlerPipeline []execution.GraphCursor  `json:"handler_pipeline"`
	ReturnTo        catalog.SlotID           `json:"return_to"`
	IfUnavailable   catalog.IncidentFallback `json:"if_unavailable"`
}

type CompiledSlot struct {
	SlotID          catalog.SlotID            `json:"slot_id"`
	Applicability   catalog.SlotApplicability `json:"applicability"`
	Active          bool                      `json:"active"`
	EntryArtifact   string                    `json:"entry_artifact"`
	OutcomeArtifact string                    `json:"outcome_artifact"`
	OutcomeOwner    CompiledOwner             `json:"outcome_owner"`
	Pipeline        []ResolvedBinding         `json:"pipeline"`
	HostAction      *CompiledHostAction       `json:"host_action,omitempty"`
	Gates           []CompiledGate            `json:"gates"`
	Transitions     []GraphTransition         `json:"transitions"`
	Terminal        bool                      `json:"terminal"`
	Traversal       []execution.GraphCursor   `json:"traversal"`
}

type CompileDecision struct {
	SlotID        catalog.SlotID      `json:"slot_id,omitempty"`
	StepID        string              `json:"step_id,omitempty"`
	UnitID        string              `json:"unit_id,omitempty"`
	AddOnID       string              `json:"add_on_id,omitempty"`
	AlternativeID string              `json:"alternative_id,omitempty"`
	OverlayID     string              `json:"overlay_id,omitempty"`
	IncidentType  string              `json:"incident_type,omitempty"`
	Disposition   DispatchDisposition `json:"disposition"`
	ReasonCode    string              `json:"reason_code"`
	Detail        string              `json:"detail"`
}

type ExecutionGraphRecord struct {
	SchemaVersion           string                             `json:"schema_version"`
	HostID                  string                             `json:"host_id"`
	HostEvidenceDigest      string                             `json:"host_evidence_digest"`
	RegistryDigest          string                             `json:"registry_digest"`
	TaxonomyVersion         string                             `json:"taxonomy_version"`
	RecipeID                string                             `json:"recipe_id"`
	RecipeVersion           string                             `json:"recipe_version"`
	RecipeDigest            string                             `json:"recipe_digest"`
	Selection               Selection                          `json:"selection"`
	ProviderInstances       []GraphProviderInstance            `json:"provider_instances"`
	EntrySlotID             catalog.SlotID                     `json:"entry_slot_id"`
	Slots                   []CompiledSlot                     `json:"slots"`
	IncidentRoutes          []CompiledIncidentRoute            `json:"incident_routes"`
	StableBoundaries        []string                           `json:"stable_boundaries"`
	Topology                execution.Topology                 `json:"topology"`
	EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements"`
	Decisions               []CompileDecision                  `json:"decisions"`
	Digest                  string                             `json:"digest"`
}

type ExecutionGraph struct{ record ExecutionGraphRecord }

type CompileDiagnostic struct {
	Code          string             `json:"code"`
	SlotID        catalog.SlotID     `json:"slot_id,omitempty"`
	StepID        string             `json:"step_id,omitempty"`
	ProviderID    string             `json:"provider_id,omitempty"`
	BindingID     string             `json:"binding_id,omitempty"`
	AddOnID       string             `json:"add_on_id,omitempty"`
	AlternativeID string             `json:"alternative_id,omitempty"`
	OverlayID     string             `json:"overlay_id,omitempty"`
	IncidentType  string             `json:"incident_type,omitempty"`
	Topology      execution.Topology `json:"topology,omitempty"`
	Detail        string             `json:"detail"`
}

type CompileResult struct {
	graph       *ExecutionGraph
	diagnostics []CompileDiagnostic
	digest      string
}

type HostEvidenceRecord struct {
	SchemaVersion           string                             `json:"schema_version"`
	HostID                  string                             `json:"host_id"`
	Topology                execution.Topology                 `json:"topology"`
	FeatureObservations     []host.FeatureObservation          `json:"feature_observations"`
	ActionObservations      []host.HostActionObservation       `json:"action_observations"`
	EnvironmentObservations []execution.EnvironmentObservation `json:"environment_observations"`
	SessionDigest           string                             `json:"session_digest"`
	ManifestDigest          string                             `json:"manifest_digest"`
	InventoryDigest         string                             `json:"inventory_digest"`
	FeatureDigest           string                             `json:"feature_digest"`
	ActionDigest            string                             `json:"action_digest"`
	EnvironmentDigest       string                             `json:"environment_digest"`
	Digest                  string                             `json:"digest"`
}

type HostEvidence struct{ record HostEvidenceRecord }

type TraversalUnit struct {
	Cursor          execution.GraphCursor `json:"cursor"`
	ProviderBinding *ResolvedBinding      `json:"provider_binding,omitempty"`
	HostAction      *CompiledHostAction   `json:"host_action,omitempty"`
	Gate            *CompiledGate         `json:"gate,omitempty"`
	Terminal        bool                  `json:"terminal"`
}

type TraversalDisposition string

const (
	TraversalNext     TraversalDisposition = "next"
	TraversalTerminal TraversalDisposition = "terminal"
	TraversalStop     TraversalDisposition = "stop"
	TraversalReplan   TraversalDisposition = "replan"
)

type TraversalResult struct {
	Disposition TraversalDisposition   `json:"disposition"`
	Cursor      *execution.GraphCursor `json:"cursor,omitempty"`
}

type EffectiveRegistry interface {
	HostID() string
	Providers() []registry.ProviderInstance
	Provider(id string) (registry.ProviderInstance, bool)
	Binding(providerID, bindingID string) (registry.VerifiedBinding, bool)
	Bindings(providerID string) []registry.VerifiedBinding
	Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool)
	Digest() string
}

type CatalogSource interface {
	Providers() []catalog.ProviderDescriptorRecord
	Recipes() []catalog.ProfileRecipeRecord
	Aliases() []catalog.ProfileAliasRecord
}

func (graph ExecutionGraph) Record() ExecutionGraphRecord {
	return cloneExecutionGraphRecord(graph.record)
}

func (record ExecutionGraphRecord) ContentDigest() string {
	value := cloneExecutionGraphRecord(record)
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func (result CompileResult) Graph() (ExecutionGraphRecord, bool) {
	if result.graph == nil {
		return ExecutionGraphRecord{}, false
	}
	return result.graph.Record(), true
}

func (result CompileResult) Diagnostics() []CompileDiagnostic {
	return append([]CompileDiagnostic{}, result.diagnostics...)
}

func (result CompileResult) Digest() string { return result.digest }

func (evidence HostEvidence) Record() HostEvidenceRecord {
	return cloneHostEvidenceRecord(evidence.record)
}
func (evidence HostEvidence) Digest() string { return evidence.record.Digest }

func (record HostEvidenceRecord) ContentDigest() string {
	value := cloneHostEvidenceRecord(record)
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func cloneSelection(value Selection) Selection {
	value.AddOns = append([]string{}, value.AddOns...)
	value.Alternatives = append([]AlternativeChoice{}, value.Alternatives...)
	value.Overlays = append([]string{}, value.Overlays...)
	return value
}

func cloneResolvedBinding(value ResolvedBinding) ResolvedBinding {
	value.SlotIDs = append([]catalog.SlotID{}, value.SlotIDs...)
	value.Responsibilities = append([]catalog.ResponsibilityClaim{}, value.Responsibilities...)
	value.MaximumEffects = append([]string{}, value.MaximumEffects...)
	value.Resources = append([]string{}, value.Resources...)
	value.SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
	value.RequiredFeatures = append([]host.FeatureID{}, value.RequiredFeatures...)
	value.FeatureEvidenceDigests = append([]string{}, value.FeatureEvidenceDigests...)
	return value
}

func cloneResolvedBindings(values []ResolvedBinding) []ResolvedBinding {
	result := make([]ResolvedBinding, len(values))
	for index, value := range values {
		result[index] = cloneResolvedBinding(value)
	}
	return result
}

func cloneCompiledHostAction(value CompiledHostAction) CompiledHostAction {
	value.MaximumEffects = append([]string{}, value.MaximumEffects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}

func cloneCompiledGate(value CompiledGate) CompiledGate {
	value.EvidenceRequirements = append([]catalog.EvidenceRequirementRecord{}, value.EvidenceRequirements...)
	return value
}

func cloneCompiledSlot(value CompiledSlot) CompiledSlot {
	value.Pipeline = cloneResolvedBindings(value.Pipeline)
	if value.HostAction != nil {
		action := cloneCompiledHostAction(*value.HostAction)
		value.HostAction = &action
	}
	gates := value.Gates
	value.Gates = make([]CompiledGate, len(gates))
	for index, gate := range gates {
		value.Gates[index] = cloneCompiledGate(gate)
	}
	value.Transitions = append([]GraphTransition{}, value.Transitions...)
	value.Traversal = append([]execution.GraphCursor{}, value.Traversal...)
	return value
}

func cloneExecutionGraphRecord(value ExecutionGraphRecord) ExecutionGraphRecord {
	value.Selection = cloneSelection(value.Selection)
	value.ProviderInstances = append([]GraphProviderInstance{}, value.ProviderInstances...)
	slots := value.Slots
	value.Slots = make([]CompiledSlot, len(slots))
	for index, slot := range slots {
		value.Slots[index] = cloneCompiledSlot(slot)
	}
	routes := value.IncidentRoutes
	value.IncidentRoutes = make([]CompiledIncidentRoute, len(routes))
	for index, route := range routes {
		value.IncidentRoutes[index] = route
		value.IncidentRoutes[index].HandlerPipeline = append([]execution.GraphCursor{}, route.HandlerPipeline...)
	}
	value.StableBoundaries = append([]string{}, value.StableBoundaries...)
	value.EnvironmentRequirements = cloneRequirements(value.EnvironmentRequirements)
	value.Decisions = append([]CompileDecision{}, value.Decisions...)
	return value
}

func cloneHostEvidenceRecord(value HostEvidenceRecord) HostEvidenceRecord {
	value.FeatureObservations = append([]host.FeatureObservation{}, value.FeatureObservations...)
	actions := value.ActionObservations
	value.ActionObservations = make([]host.HostActionObservation, len(actions))
	for index, observation := range actions {
		value.ActionObservations[index] = observation
		value.ActionObservations[index].Action.MaximumEffects = append([]string{}, observation.Action.MaximumEffects...)
		value.ActionObservations[index].Action.Resources = append([]string{}, observation.Action.Resources...)
	}
	value.EnvironmentObservations = append([]execution.EnvironmentObservation{}, value.EnvironmentObservations...)
	return value
}

func cloneRequirements(values []execution.EnvironmentRequirement) []execution.EnvironmentRequirement {
	result := make([]execution.EnvironmentRequirement, len(values))
	for index, value := range values {
		result[index] = value
		result[index].AcceptedDispositions = append([]execution.EnvironmentDisposition{}, value.AcceptedDispositions...)
	}
	return result
}

func clonePipelineSteps(values []catalog.PipelineStep) []catalog.PipelineStep {
	result := make([]catalog.PipelineStep, len(values))
	for index, value := range values {
		result[index] = value
		result[index].StageSpan = append([]catalog.SlotID{}, value.StageSpan...)
	}
	return result
}
