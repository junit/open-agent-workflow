package core

import (
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type SelectionSource string

const (
	SelectionUser           SelectionSource = "user-selection"
	SelectionHostOnlyOption SelectionSource = "host-only-option"
)

type Selection struct {
	Profile        string                   `json:"profile"`
	ProfileSource  SelectionSource          `json:"profile_source"`
	Topology       execution.Topology       `json:"topology"`
	TopologySource SelectionSource          `json:"topology_source"`
	AddOns         []string                 `json:"add_ons"`
	Bindings       []profile.ProfileBinding `json:"bindings"`
}

type CompilationRequest struct {
	DeliverableID               string
	InputDigest                 string
	Generation                  uint64
	Classification              classification.ClassificationDecision
	Configuration               config.Snapshot
	Resolutions                 registry.ResolutionReport
	Registry                    registry.Registry
	HostID                      string
	HostSessionDigest           string
	HostEnvironmentReportDigest string
	HostProviderInventoryDigest string
	HostTopologies              []execution.Topology
	EnvironmentObservations     []execution.EnvironmentObservation
	Selection                   *Selection
}

type ResolutionRequest struct {
	Configuration config.Snapshot
	HostID        string
	Discovery     discovery.Report
	Inventory     *host.BindingInventory
}

type ResolutionResult struct {
	Report   registry.ResolutionReport
	Registry registry.Registry
	Digest   string
}

type EligibilityDiagnostic struct {
	Code         string             `json:"code"`
	ProviderID   string             `json:"provider_id,omitempty"`
	CapabilityID string             `json:"capability_id,omitempty"`
	Topology     execution.Topology `json:"topology,omitempty"`
	Detail       string             `json:"detail"`
}

type ProfileEligibility struct {
	Profile              string                  `json:"profile"`
	RecipeID             string                  `json:"recipe_id"`
	Eligible             bool                    `json:"eligible"`
	EligibleTopologies   []execution.Topology    `json:"eligible_topologies"`
	Diagnostics          []EligibilityDiagnostic `json:"diagnostics"`
	Recommended          bool                    `json:"recommended"`
	RecommendationReason string                  `json:"recommendation_reason,omitempty"`
}

type AddOnEligibility struct {
	NodeID             string                  `json:"node_id"`
	ProviderID         string                  `json:"provider_id"`
	CapabilityID       string                  `json:"capability_id"`
	EligibleTopologies []execution.Topology    `json:"eligible_topologies"`
	Diagnostics        []EligibilityDiagnostic `json:"diagnostics"`
}

type LifecycleBundle struct {
	SchemaVersion           string                                `json:"schema_version"`
	ID                      string                                `json:"id"`
	DeliverableID           string                                `json:"deliverable_id"`
	InputDigest             string                                `json:"input_digest"`
	Generation              uint64                                `json:"generation"`
	Classification          classification.ClassificationDecision `json:"classification"`
	ClassificationDigest    string                                `json:"classification_digest"`
	Selection               Selection                             `json:"selection"`
	HostID                  string                                `json:"host_id"`
	HostSessionDigest       string                                `json:"host_session_digest"`
	EnvironmentReportDigest string                                `json:"environment_report_digest"`
	ProviderInventoryDigest string                                `json:"provider_inventory_digest"`
	Configuration           config.SnapshotRecord                 `json:"configuration"`
	ResolutionDigest        string                                `json:"resolution_digest"`
	RegistryDigest          string                                `json:"registry_digest"`
	ProviderInstances       []profile.GraphProviderInstance       `json:"provider_instances"`
	Graph                   profile.ExecutionGraphRecord          `json:"execution_graph"`
	Topology                execution.Topology                    `json:"topology"`
	EnvironmentRequirements []execution.EnvironmentRequirement    `json:"environment_requirements"`
	EnvironmentObservations []execution.EnvironmentObservation    `json:"environment_observations"`
	AddOns                  []string                              `json:"add_ons"`
	Digest                  string                                `json:"digest"`
}

type CompilationResult struct {
	EligibleProfiles []ProfileEligibility `json:"eligible_profiles"`
	EligibleAddOns   []AddOnEligibility   `json:"eligible_add_ons"`
	Bundle           *LifecycleBundle     `json:"bundle,omitempty"`
	Digest           string               `json:"digest"`
}

type Error struct {
	Code   string
	Detail string
}

func (err *Error) Error() string {
	if err.Detail == "" {
		return err.Code
	}
	return fmt.Sprintf("%s: %s", err.Code, err.Detail)
}

func coreError(code, format string, values ...any) error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, values...)}
}
