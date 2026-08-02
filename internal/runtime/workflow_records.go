package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const (
	lifecycleBundleSchemaV1 = "oaw.lifecycle-bundle/v1"
	resourceLeaseSchemaV1   = "oaw.resource-lease/v1"
)

var bundleIDPattern = regexp.MustCompile(`^bundle-[0-9a-f]{32}$`)

type WorkflowOptions struct {
	Configuration config.Snapshot
	Registry      registry.Registry
	Authority     admission.AuthorityCeiling
	Host          WorkflowHostDeclaration
	Executors     []WorkflowExecutorRegistration
	Projection    ProjectionOptions
}

type WorkflowHostDeclaration struct {
	PhysicalIsolation bool `json:"physical_isolation"`
}

type WorkflowExecutorRegistration struct {
	Registration admission.ExecutorRegistration `json:"registration"`
	ReadOnly     bool                           `json:"read_only"`
	Fresh        bool                           `json:"fresh"`
}

type ProjectionOptions struct {
	Root string `json:"root"`
}

type WorkflowInput struct {
	DeliverableID string `json:"deliverable_id"`
	InputDigest   string `json:"input_digest"`
}

type ProfileSelection struct {
	Profile  string                   `json:"profile"`
	Bindings []profile.ProfileBinding `json:"bindings"`
}

type StageGrantRequest struct {
	ExecutorID           string   `json:"executor_id"`
	RequestedEffects     []string `json:"requested_effects"`
	RequestedResources   []string `json:"requested_resources"`
	TerminationCondition string   `json:"termination_condition"`
}

type StageObservation struct {
	CapabilityObservation
	Signal         string `json:"signal"`
	StableBoundary string `json:"stable_boundary,omitempty"`
}

type StableBoundarySwitch struct {
	Boundary  string           `json:"boundary"`
	Selection ProfileSelection `json:"selection"`
}

type LifecycleBundle struct {
	SchemaVersion     string                          `json:"schema_version"`
	ID                string                          `json:"id"`
	RunID             string                          `json:"run_id"`
	DeliverableID     string                          `json:"deliverable_id"`
	InputDigest       string                          `json:"input_digest"`
	Generation        uint64                          `json:"generation"`
	CreatedRevision   uint64                          `json:"created_revision"`
	Selection         ProfileSelection                `json:"selection"`
	RecipeID          string                          `json:"recipe_id"`
	RecipeVersion     string                          `json:"recipe_version"`
	RecipeDigest      string                          `json:"recipe_digest"`
	RegistryDigest    string                          `json:"registry_digest"`
	ProviderInstances []profile.GraphProviderInstance `json:"provider_instances"`
	Bindings          []profile.ProfileBinding        `json:"bindings"`
	AddOns            []string                        `json:"add_ons"`
	Configuration     config.SnapshotRecord           `json:"configuration"`
	Graph             profile.ExecutionGraphRecord    `json:"execution_graph"`
	GraphDigest       string                          `json:"graph_digest"`
	Digest            string                          `json:"digest"`
}

type WorkflowState struct {
	Input               WorkflowInput      `json:"input"`
	ConfigurationDigest string             `json:"configuration_digest"`
	RegistryDigest      string             `json:"registry_digest"`
	Bundles             []LifecycleBundle  `json:"bundles"`
	ActiveGeneration    uint64             `json:"active_generation"`
	ActiveNodeID        string             `json:"active_node_id"`
	ActiveGrantID       string             `json:"active_grant_id,omitempty"`
	Observations        []StageObservation `json:"observations"`
	RevokedGrantIDs     []string           `json:"revoked_grant_ids"`
	ResourceLeases      []ResourceLease    `json:"resource_leases"`
	LastStableBoundary  string             `json:"last_stable_boundary,omitempty"`
	ProjectionLag       []ProjectionLag    `json:"projection_lag"`
}

type ResourceLease struct {
	SchemaVersion    string `json:"schema_version"`
	ID               string `json:"id"`
	RunID            string `json:"run_id"`
	GrantID          string `json:"grant_id"`
	BundleID         string `json:"bundle_id"`
	Generation       uint64 `json:"generation"`
	Resource         string `json:"resource"`
	PhysicalRoot     string `json:"physical_root"`
	AcquiredRevision uint64 `json:"acquired_revision"`
	Digest           string `json:"digest"`
}

type ProjectionLag struct {
	Revision uint64 `json:"revision"`
	Digest   string `json:"digest"`
	Reason   string `json:"reason"`
}

type bundleRequest struct {
	RunID           string
	DeliverableID   string
	InputDigest     string
	Generation      uint64
	CreatedRevision uint64
	Selection       ProfileSelection
	Configuration   config.SnapshotRecord
	RegistryDigest  string
	Graph           profile.ExecutionGraphRecord
}

func newLifecycleBundle(request bundleRequest) (LifecycleBundle, error) {
	selection, err := normalizeProfileSelection(request.Selection)
	if err != nil {
		return LifecycleBundle{}, err
	}
	if !runIDPattern.MatchString(request.RunID) || validateIdentifier(request.DeliverableID) != nil || !validDigest(request.InputDigest) || request.Generation == 0 || request.CreatedRevision == 0 || !validDigest(request.RegistryDigest) {
		return LifecycleBundle{}, runtimeError("WORKFLOW_BUNDLE_INVALID", "invalid Bundle identity", nil)
	}
	if request.Configuration.Digest == "" || request.Configuration.ContentDigest() != request.Configuration.Digest {
		return LifecycleBundle{}, runtimeError("WORKFLOW_BUNDLE_INVALID", "invalid Configuration Snapshot digest", nil)
	}
	if err := profile.ValidateExecutionGraphRecord(request.Graph); err != nil {
		return LifecycleBundle{}, runtimeError("WORKFLOW_BUNDLE_INVALID", "invalid Execution Graph", err)
	}
	addOns := make([]string, 0)
	for _, node := range request.Graph.Nodes {
		if node.Optional {
			addOns = append(addOns, node.ID)
		}
	}
	sort.Strings(addOns)
	value := LifecycleBundle{
		SchemaVersion: lifecycleBundleSchemaV1, RunID: request.RunID, DeliverableID: request.DeliverableID,
		InputDigest: request.InputDigest, Generation: request.Generation, CreatedRevision: request.CreatedRevision,
		Selection: selection, RecipeID: request.Graph.RecipeID, RecipeVersion: request.Graph.RecipeVersion,
		RecipeDigest: request.Graph.RecipeDigest, ProviderInstances: append([]profile.GraphProviderInstance{}, request.Graph.ProviderInstances...),
		RegistryDigest: request.RegistryDigest,
		Bindings:       append([]profile.ProfileBinding{}, request.Graph.Bindings...), AddOns: addOns,
		Configuration: cloneSnapshotRecord(request.Configuration), Graph: cloneGraphRecord(request.Graph), GraphDigest: request.Graph.Digest,
	}
	seed := value
	seed.ID, seed.Digest = "", ""
	seedDigest, _, digestErr := canonicaljson.Digest(seed)
	if digestErr != nil {
		return LifecycleBundle{}, runtimeError("WORKFLOW_BUNDLE_INVALID", "digest Bundle seed", digestErr)
	}
	value.ID = deterministicRuntimeID("bundle-", "bundle\x00"+seedDigest)
	value.Digest, _, digestErr = canonicaljson.Digest(value)
	if digestErr != nil {
		return LifecycleBundle{}, runtimeError("WORKFLOW_BUNDLE_INVALID", "digest Bundle", digestErr)
	}
	return cloneLifecycleBundle(value), nil
}

func validateLifecycleBundle(value LifecycleBundle) error {
	if value.SchemaVersion != lifecycleBundleSchemaV1 || !bundleIDPattern.MatchString(value.ID) || !runIDPattern.MatchString(value.RunID) || validateIdentifier(value.DeliverableID) != nil || !validDigest(value.InputDigest) || value.Generation == 0 || value.CreatedRevision == 0 || !validDigest(value.RegistryDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Lifecycle Bundle identity", nil)
	}
	if value.Configuration.Digest == "" || value.Configuration.ContentDigest() != value.Configuration.Digest || profile.ValidateExecutionGraphRecord(value.Graph) != nil || value.GraphDigest != value.Graph.Digest || value.RecipeID != value.Graph.RecipeID || value.RecipeVersion != value.Graph.RecipeVersion || value.RecipeDigest != value.Graph.RecipeDigest || !equalGraphProviders(value.ProviderInstances, value.Graph.ProviderInstances) || !equalBindings(value.Bindings, value.Graph.Bindings) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted Lifecycle Bundle content mismatch", nil)
	}
	selection, err := normalizeProfileSelection(value.Selection)
	if err != nil || !equalProfileSelection(selection, value.Selection) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted Lifecycle Bundle selection is not normalized", err)
	}
	stored := value.Digest
	copyValue := cloneLifecycleBundle(value)
	copyValue.Digest = ""
	if digest, _, digestErr := canonicaljson.Digest(copyValue); digestErr != nil || digest != stored {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted Lifecycle Bundle digest mismatch", digestErr)
	}
	return nil
}

func normalizeProfileSelection(value ProfileSelection) (ProfileSelection, error) {
	value.Profile = strings.TrimSpace(value.Profile)
	if value.Profile == "" || strings.ContainsAny(value.Profile, "\r\n\x00") {
		return ProfileSelection{}, runtimeError("PROFILE_SELECTION_INVALID", "profile is required", nil)
	}
	value.Bindings = append([]profile.ProfileBinding{}, value.Bindings...)
	sort.Slice(value.Bindings, func(left, right int) bool {
		l, r := value.Bindings[left], value.Bindings[right]
		return l.Selector.ProviderID+"\x00"+l.Selector.CapabilityID+"\x00"+l.PreferredProviderID < r.Selector.ProviderID+"\x00"+r.Selector.CapabilityID+"\x00"+r.PreferredProviderID
	})
	for index, binding := range value.Bindings {
		if index > 0 && value.Bindings[index-1] == binding {
			return ProfileSelection{}, runtimeError("PROFILE_SELECTION_INVALID", "duplicate Profile Binding", nil)
		}
		if binding.Selector.ProviderID == "" || binding.Selector.CapabilityID == "" || binding.PreferredProviderID == "" {
			return ProfileSelection{}, runtimeError("PROFILE_SELECTION_INVALID", "incomplete Profile Binding", nil)
		}
	}
	return value, nil
}

func cloneLifecycleBundle(value LifecycleBundle) LifecycleBundle {
	value.Selection.Bindings = append([]profile.ProfileBinding{}, value.Selection.Bindings...)
	value.ProviderInstances = append([]profile.GraphProviderInstance{}, value.ProviderInstances...)
	value.Bindings = append([]profile.ProfileBinding{}, value.Bindings...)
	value.AddOns = append([]string{}, value.AddOns...)
	value.Configuration = cloneSnapshotRecord(value.Configuration)
	value.Graph = cloneGraphRecord(value.Graph)
	return value
}

func cloneSnapshotRecord(value config.SnapshotRecord) config.SnapshotRecord {
	value.Settings = append([]config.ProviderSettings{}, value.Settings...)
	for index := range value.Settings {
		value.Settings[index].Preferences = append([]config.BindingPreference{}, value.Settings[index].Preferences...)
		value.Settings[index].CapabilityLimit = append([]string{}, value.Settings[index].CapabilityLimit...)
		if value.Settings[index].Pin != nil {
			pin := *value.Settings[index].Pin
			value.Settings[index].Pin = &pin
		}
	}
	value.BoundedCapabilityDefaults = append([]config.BoundedCapabilityDefault{}, value.BoundedCapabilityDefaults...)
	value.RequiredProviders = append([]string{}, value.RequiredProviders...)
	value.RecommendedProviders = append([]string{}, value.RecommendedProviders...)
	value.UntrustedProviderIDs = append([]string{}, value.UntrustedProviderIDs...)
	return value
}

func cloneGraphRecord(value profile.ExecutionGraphRecord) profile.ExecutionGraphRecord {
	value.Bindings = append([]profile.ProfileBinding{}, value.Bindings...)
	value.ProviderInstances = append([]profile.GraphProviderInstance{}, value.ProviderInstances...)
	value.Nodes = append([]profile.GraphNode{}, value.Nodes...)
	for index := range value.Nodes {
		value.Nodes[index].MaximumEffects = append([]string{}, value.Nodes[index].MaximumEffects...)
		value.Nodes[index].Resources = append([]string{}, value.Nodes[index].Resources...)
		value.Nodes[index].RequestModes = append([]catalog.RequestMode{}, value.Nodes[index].RequestModes...)
		value.Nodes[index].DelegationAllowList = append([]string{}, value.Nodes[index].DelegationAllowList...)
		value.Nodes[index].Transitions = append([]profile.GraphTransition{}, value.Nodes[index].Transitions...)
	}
	value.IncidentRoutes = append([]profile.GraphIncidentRoute{}, value.IncidentRoutes...)
	value.TerminalGates = append([]string{}, value.TerminalGates...)
	value.StableBoundaries = append([]string{}, value.StableBoundaries...)
	return value
}

func equalGraphProviders(left, right []profile.GraphProviderInstance) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalBindings(left, right []profile.ProfileBinding) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalProfileSelection(left, right ProfileSelection) bool {
	return left.Profile == right.Profile && equalBindings(left.Bindings, right.Bindings)
}

func deterministicRuntimeID(prefix, seed string) string {
	digest := sha256.Sum256([]byte(seed))
	return prefix + hex.EncodeToString(digest[:16])
}
