package core

import (
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const (
	LifecycleBundleSchemaV4 = "oaw.lifecycle-bundle/v4"
	UserDefinedProfile      = "USER-DEFINED"
)

type SelectionSource string

const (
	SelectionUser           SelectionSource = "user-selection"
	SelectionHostOnlyOption SelectionSource = "host-only-option"
)

type Selection struct {
	Profile              string                      `json:"profile"`
	RecipeID             string                      `json:"recipe_id"`
	RecipeDigest         string                      `json:"recipe_digest"`
	ProfileSource        SelectionSource             `json:"profile_source"`
	Topology             execution.Topology          `json:"topology"`
	TopologySource       SelectionSource             `json:"topology_source"`
	AddOns               []string                    `json:"add_ons"`
	Alternatives         []profile.AlternativeChoice `json:"alternatives"`
	Overlays             []string                    `json:"overlays"`
	GraphSelectionDigest string                      `json:"graph_selection_digest"`
	ConfirmationDigest   string                      `json:"confirmation_digest,omitempty"`
}

type CompilationRequest struct {
	DeliverableID    string
	InputDigest      string
	Generation       uint64
	Classification   classification.ClassificationDecision
	Configuration    config.Snapshot
	ResolutionDigest string
	Registry         profile.EffectiveRegistry
	Host             profile.HostEvidence
	Selection        *Selection
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

type SelectionPreview struct {
	Selection         Selection                       `json:"selection"`
	Recipe            catalog.ProfileRecipeRecord     `json:"recipe"`
	ProviderInstances []profile.GraphProviderInstance `json:"provider_instances"`
	Graph             *profile.ExecutionGraphRecord   `json:"execution_graph,omitempty"`
	Diagnostics       []profile.CompileDiagnostic     `json:"diagnostics"`
	Digest            string                          `json:"digest"`
}

type ProfileEligibility struct {
	Profile              string                      `json:"profile"`
	RecipeID             string                      `json:"recipe_id"`
	Eligible             bool                        `json:"eligible"`
	Topology             execution.Topology          `json:"topology"`
	Diagnostics          []profile.CompileDiagnostic `json:"diagnostics"`
	Preview              SelectionPreview            `json:"preview"`
	Recommended          bool                        `json:"recommended"`
	RecommendationReason string                      `json:"recommendation_reason,omitempty"`
}

type AddOnEligibility struct {
	Profile     string                      `json:"profile"`
	RecipeID    string                      `json:"recipe_id"`
	AddOnID     string                      `json:"add_on_id"`
	Kind        catalog.AddOnKind           `json:"kind"`
	SlotID      catalog.SlotID              `json:"slot_id"`
	Eligible    bool                        `json:"eligible"`
	Diagnostics []profile.CompileDiagnostic `json:"diagnostics"`
	Preview     SelectionPreview            `json:"preview"`
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
	Recipe                  catalog.ProfileRecipeRecord           `json:"recipe"`
	RecipeDigest            string                                `json:"recipe_digest"`
	HostID                  string                                `json:"host_id"`
	HostSessionDigest       string                                `json:"host_session_digest"`
	HostManifestDigest      string                                `json:"host_manifest_digest"`
	EnvironmentReportDigest string                                `json:"environment_report_digest"`
	ProviderInventoryDigest string                                `json:"provider_inventory_digest"`
	HostFeatureDigest       string                                `json:"host_feature_digest"`
	HostActionDigest        string                                `json:"host_action_digest"`
	HostEvidenceDigest      string                                `json:"host_evidence_digest"`
	Configuration           config.SnapshotRecord                 `json:"configuration"`
	ResolutionDigest        string                                `json:"resolution_digest"`
	RegistryDigest          string                                `json:"registry_digest"`
	ProviderInstances       []profile.GraphProviderInstance       `json:"provider_instances"`
	Graph                   profile.ExecutionGraphRecord          `json:"execution_graph"`
	Topology                execution.Topology                    `json:"topology"`
	EnvironmentRequirements []execution.EnvironmentRequirement    `json:"environment_requirements"`
	AddOns                  []string                              `json:"add_ons"`
	Digest                  string                                `json:"digest"`
}

type CompilationResult struct {
	EligibleProfiles []ProfileEligibility `json:"eligible_profiles"`
	EligibleAddOns   []AddOnEligibility   `json:"eligible_add_ons"`
	SelectionPreview *SelectionPreview    `json:"selection_preview,omitempty"`
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
