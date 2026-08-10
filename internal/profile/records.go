package profile

import (
	"fmt"
	"slices"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const ExecutionGraphSchemaV3 = "oaw.execution-graph/v3"

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

type CompileRequest struct {
	Profile                 string                             `json:"profile"`
	Bindings                []ProfileBinding                   `json:"bindings"`
	AddOns                  []string                           `json:"add_ons"`
	HostTopologies          []execution.Topology               `json:"host_topologies"`
	EnvironmentObservations []execution.EnvironmentObservation `json:"environment_observations"`
}

type ProfileBinding struct {
	Selector            catalog.CapabilitySelector `json:"selector"`
	PreferredProviderID string                     `json:"preferred_provider_id"`
}

type GraphProviderInstance struct {
	ProviderID     string `json:"provider_id"`
	HostID         string `json:"host_id"`
	InstanceDigest string `json:"instance_digest"`
}

type GraphTransition struct {
	Signal string `json:"signal"`
	Target string `json:"target"`
}

type GraphIncidentRoute struct {
	Incident string `json:"incident"`
	Handler  string `json:"handler"`
}

type GraphNode struct {
	ID                     string                `json:"id"`
	Kind                   catalog.NodeKind      `json:"kind"`
	Responsibility         string                `json:"responsibility"`
	Phase                  string                `json:"phase,omitempty"`
	Optional               bool                  `json:"optional,omitempty"`
	ProviderID             string                `json:"provider_id"`
	ProviderInstanceDigest string                `json:"provider_instance_digest"`
	CapabilityID           string                `json:"capability_id"`
	Binding                catalog.HostBinding   `json:"binding"`
	InputSchema            string                `json:"input_schema"`
	OutcomeSchema          string                `json:"outcome_schema"`
	MaximumEffects         []string              `json:"maximum_effects"`
	Resources              []string              `json:"resources"`
	RequestModes           []catalog.RequestMode `json:"request_modes"`
	SupportedTopologies    []execution.Topology  `json:"supported_topologies"`
	DelegationAllowList    []string              `json:"delegation_allow_list"`
	Transitions            []GraphTransition     `json:"transitions"`
}

type ExecutionGraph struct {
	schemaVersion           string
	hostID                  string
	recipeID                string
	recipeVersion           string
	recipeDigest            string
	entry                   string
	bindings                []ProfileBinding
	providerInstances       []GraphProviderInstance
	nodes                   []GraphNode
	incidentRoutes          []GraphIncidentRoute
	terminalGates           []string
	stableBoundaries        []string
	eligibleTopologies      []execution.Topology
	environmentRequirements []execution.EnvironmentRequirement
	digest                  string
}

type ExecutionGraphRecord struct {
	SchemaVersion           string                             `json:"schema_version"`
	HostID                  string                             `json:"host_id"`
	RecipeID                string                             `json:"recipe_id"`
	RecipeVersion           string                             `json:"recipe_version"`
	RecipeDigest            string                             `json:"recipe_digest"`
	Entry                   string                             `json:"entry"`
	Bindings                []ProfileBinding                   `json:"bindings"`
	ProviderInstances       []GraphProviderInstance            `json:"provider_instances"`
	Nodes                   []GraphNode                        `json:"nodes"`
	IncidentRoutes          []GraphIncidentRoute               `json:"incident_routes"`
	TerminalGates           []string                           `json:"terminal_gates"`
	StableBoundaries        []string                           `json:"stable_boundaries"`
	EligibleTopologies      []execution.Topology               `json:"eligible_topologies"`
	EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements"`
	Digest                  string                             `json:"digest"`
}

func (graph ExecutionGraph) SchemaVersion() string { return graph.schemaVersion }
func (graph ExecutionGraph) HostID() string        { return graph.hostID }
func (graph ExecutionGraph) RecipeID() string      { return graph.recipeID }
func (graph ExecutionGraph) RecipeVersion() string { return graph.recipeVersion }
func (graph ExecutionGraph) RecipeDigest() string  { return graph.recipeDigest }
func (graph ExecutionGraph) Entry() string         { return graph.entry }
func (graph ExecutionGraph) Digest() string        { return graph.digest }

func (graph ExecutionGraph) Record() ExecutionGraphRecord {
	return ExecutionGraphRecord{
		SchemaVersion: graph.schemaVersion, HostID: graph.hostID, RecipeID: graph.recipeID,
		RecipeVersion: graph.recipeVersion, RecipeDigest: graph.recipeDigest,
		Entry: graph.entry, Bindings: cloneBindings(graph.bindings),
		ProviderInstances:       append([]GraphProviderInstance{}, graph.providerInstances...),
		Nodes:                   cloneGraphNodes(graph.nodes),
		IncidentRoutes:          append([]GraphIncidentRoute{}, graph.incidentRoutes...),
		TerminalGates:           append([]string{}, graph.terminalGates...),
		StableBoundaries:        append([]string{}, graph.stableBoundaries...),
		EligibleTopologies:      append([]execution.Topology{}, graph.eligibleTopologies...),
		EnvironmentRequirements: cloneRequirements(graph.environmentRequirements),
		Digest:                  graph.digest,
	}
}

func (record ExecutionGraphRecord) ContentDigest() string {
	content := record
	content.Digest = ""
	digest, _, err := canonicaljson.Digest(executionGraphRecordContent(content))
	if err != nil {
		return ""
	}
	return digest
}

func ValidateExecutionGraphRecord(record ExecutionGraphRecord) error {
	if record.SchemaVersion != ExecutionGraphSchemaV3 || record.HostID == "" || record.RecipeID == "" || record.RecipeVersion == "" || record.RecipeDigest == "" || record.Entry == "" || record.Digest == "" || record.ContentDigest() != record.Digest {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID")
	}
	if _, err := catalog.ParseLocalID(record.HostID); err != nil {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID: %w", err)
	}
	for _, provider := range record.ProviderInstances {
		if provider.HostID != record.HostID {
			return fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: graph Provider %s belongs to Host %q, not %q", provider.ProviderID, provider.HostID, record.HostID)
		}
	}
	for _, node := range record.Nodes {
		if node.Binding.Host != record.HostID {
			return fmt.Errorf("HOST_PROVIDER_SCOPE_MISMATCH: graph node %s Binding belongs to Host %q, not %q", node.ID, node.Binding.Host, record.HostID)
		}
		if _, err := execution.NormalizeTopologies(node.SupportedTopologies); err != nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID: %w", err)
		}
		if _, err := execution.NormalizeTopologies(node.Binding.Topologies); err != nil {
			return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID: %w", err)
		}
	}
	if normalized, err := execution.NormalizeTopologies(record.EligibleTopologies); err != nil || !slices.Equal(normalized, record.EligibleTopologies) {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID: invalid eligible topologies")
	}
	if _, err := execution.NormalizeRequirements(record.EnvironmentRequirements); err != nil {
		return fmt.Errorf("PROFILE_GRAPH_RECORD_INVALID: %w", err)
	}
	return nil
}

func (graph ExecutionGraph) Bindings() []ProfileBinding {
	return cloneBindings(graph.bindings)
}

func (graph ExecutionGraph) ProviderInstances() []GraphProviderInstance {
	return append([]GraphProviderInstance{}, graph.providerInstances...)
}

func (graph ExecutionGraph) Nodes() []GraphNode {
	return cloneGraphNodes(graph.nodes)
}

func (graph ExecutionGraph) IncidentRoutes() []GraphIncidentRoute {
	return append([]GraphIncidentRoute{}, graph.incidentRoutes...)
}

func (graph ExecutionGraph) TerminalGates() []string {
	return append([]string{}, graph.terminalGates...)
}

func (graph ExecutionGraph) StableBoundaries() []string {
	return append([]string{}, graph.stableBoundaries...)
}

func (graph ExecutionGraph) EligibleTopologies() []execution.Topology {
	return append([]execution.Topology{}, graph.eligibleTopologies...)
}

func (graph ExecutionGraph) EnvironmentRequirements() []execution.EnvironmentRequirement {
	return cloneRequirements(graph.environmentRequirements)
}

type CompileError struct {
	Code         string
	Detail       string
	ProviderID   string
	CapabilityID string
}

func (err *CompileError) Error() string {
	if err.Detail == "" {
		return err.Code
	}
	return fmt.Sprintf("%s: %s", err.Code, err.Detail)
}

func compileError(code, format string, values ...any) error {
	return &CompileError{Code: code, Detail: fmt.Sprintf(format, values...)}
}

func compileCapabilityError(providerID, capabilityID, format string, values ...any) error {
	return &CompileError{
		Code:         "PROFILE_CAPABILITY_MISSING",
		Detail:       fmt.Sprintf(format, values...),
		ProviderID:   providerID,
		CapabilityID: capabilityID,
	}
}

func cloneBindings(values []ProfileBinding) []ProfileBinding {
	return append([]ProfileBinding{}, values...)
}

func cloneGraphNodes(values []GraphNode) []GraphNode {
	result := make([]GraphNode, len(values))
	for i, value := range values {
		result[i] = value
		result[i].MaximumEffects = append([]string{}, value.MaximumEffects...)
		result[i].Resources = append([]string{}, value.Resources...)
		result[i].RequestModes = append([]catalog.RequestMode{}, value.RequestModes...)
		result[i].Binding.Topologies = append([]execution.Topology{}, value.Binding.Topologies...)
		result[i].SupportedTopologies = append([]execution.Topology{}, value.SupportedTopologies...)
		result[i].DelegationAllowList = append([]string{}, value.DelegationAllowList...)
		result[i].Transitions = append([]GraphTransition{}, value.Transitions...)
	}
	return result
}

func executionGraphRecordContent(record ExecutionGraphRecord) any {
	return struct {
		SchemaVersion           string                             `json:"schema_version"`
		HostID                  string                             `json:"host_id"`
		RecipeID                string                             `json:"recipe_id"`
		RecipeVersion           string                             `json:"recipe_version"`
		RecipeDigest            string                             `json:"recipe_digest"`
		Entry                   string                             `json:"entry"`
		Bindings                []ProfileBinding                   `json:"bindings"`
		ProviderInstances       []GraphProviderInstance            `json:"provider_instances"`
		Nodes                   []GraphNode                        `json:"nodes"`
		IncidentRoutes          []GraphIncidentRoute               `json:"incident_routes"`
		TerminalGates           []string                           `json:"terminal_gates"`
		StableBoundaries        []string                           `json:"stable_boundaries"`
		EligibleTopologies      []execution.Topology               `json:"eligible_topologies"`
		EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements"`
	}{
		record.SchemaVersion, record.HostID, record.RecipeID, record.RecipeVersion, record.RecipeDigest,
		record.Entry, record.Bindings, record.ProviderInstances, record.Nodes,
		record.IncidentRoutes, record.TerminalGates, record.StableBoundaries,
		record.EligibleTopologies, record.EnvironmentRequirements,
	}
}

func cloneRequirements(values []execution.EnvironmentRequirement) []execution.EnvironmentRequirement {
	result := make([]execution.EnvironmentRequirement, len(values))
	for index, value := range values {
		result[index] = value
		result[index].AcceptedDispositions = append([]execution.EnvironmentDisposition{}, value.AcceptedDispositions...)
	}
	return result
}
