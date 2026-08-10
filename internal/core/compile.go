package core

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type profileCandidate struct {
	Profile      string
	RecipeID     string
	Recipe       catalog.ProfileRecipeRecord
	RecipeDigest string
}

func Compile(request CompilationRequest) (CompilationResult, error) {
	hostRecord, candidates, err := validateCompilationRequest(request)
	if err != nil {
		return CompilationResult{}, err
	}

	profiles, err := compileProfileEligibility(request, candidates)
	if err != nil {
		return CompilationResult{}, err
	}
	markRecommendation(profiles, request.Classification)
	addOns, err := compileAddOnEligibility(request, candidates)
	if err != nil {
		return CompilationResult{}, err
	}
	result := CompilationResult{EligibleProfiles: profiles, EligibleAddOns: addOns}

	if request.Selection != nil {
		candidate, selection, err := normalizeRequestedSelection(*request.Selection, candidates, hostRecord.Topology)
		if err != nil {
			return CompilationResult{}, err
		}
		suppliedConfirmation := selection.ConfirmationDigest
		preview, err := compileSelectionPreview(request, candidate, selection)
		if err != nil {
			return CompilationResult{}, err
		}
		result.SelectionPreview = &preview
		if suppliedConfirmation != "" {
			if preview.Graph == nil || preview.Selection.ConfirmationDigest != suppliedConfirmation {
				return CompilationResult{}, coreError("PROFILE_SELECTION_INVALID", "selection confirmation does not match the current trusted preview")
			}
			if request.Selection.RecipeDigest != preview.Selection.RecipeDigest ||
				request.Selection.GraphSelectionDigest != preview.Selection.GraphSelectionDigest {
				return CompilationResult{}, coreError("PROFILE_SELECTION_INVALID", "selection pins do not match the current trusted preview")
			}
			if request.Selection.ProfileSource != SelectionUser || request.Selection.TopologySource != SelectionUser {
				return CompilationResult{}, coreError("PROFILE_SELECTION_INVALID", "Profile and topology require explicit user selection")
			}
			bundle, err := compileBundle(request, candidate, preview, hostRecord)
			if err != nil {
				return CompilationResult{}, err
			}
			result.Bundle = &bundle
		}
	}

	result.Digest, err = compilationResultDigest(result)
	if err != nil {
		return CompilationResult{}, err
	}
	return result, nil
}

func validateCompilationRequest(request CompilationRequest) (profile.HostEvidenceRecord, []profileCandidate, error) {
	if !validIdentifier(request.DeliverableID) || !validDigest(request.InputDigest) || request.Generation == 0 || !validDigest(request.ResolutionDigest) {
		return profile.HostEvidenceRecord{}, nil, coreError("CORE_INPUT_INVALID", "invalid Deliverable or resolution identity")
	}
	if err := validateClassification(request.Classification); err != nil {
		return profile.HostEvidenceRecord{}, nil, err
	}
	if request.Classification.RequestMode != classification.RequestModeWorkflow {
		return profile.HostEvidenceRecord{}, nil, coreError("CORE_INPUT_INVALID", "Lifecycle compilation requires WORKFLOW classification")
	}
	if err := validateConfiguration(request.Configuration); err != nil {
		return profile.HostEvidenceRecord{}, nil, err
	}
	hostRecord := request.Host.Record()
	if err := profile.ValidateHostEvidenceRecord(hostRecord); err != nil {
		return profile.HostEvidenceRecord{}, nil, err
	}
	if request.Registry == nil {
		return profile.HostEvidenceRecord{}, nil, coreError("CORE_INPUT_INVALID", "Registry is required")
	}
	if err := validateRegistry(request.Configuration.Catalog(), request.Registry, hostRecord); err != nil {
		return profile.HostEvidenceRecord{}, nil, err
	}
	candidates, err := compilationCandidates(request.Configuration.Catalog())
	if err != nil {
		return profile.HostEvidenceRecord{}, nil, err
	}
	return hostRecord, candidates, nil
}

func validateConfiguration(snapshot config.Snapshot) error {
	record := snapshot.Record()
	if !validDigest(snapshot.Digest()) || record.Digest != snapshot.Digest() || record.ContentDigest() != record.Digest {
		return coreError("CONFIGURATION_DIGEST_INVALID", "Configuration Snapshot digest is invalid")
	}
	return nil
}

func validateClassification(value classification.ClassificationDecision) error {
	record := struct {
		SchemaVersion        string                               `json:"schema_version"`
		RequestMode          classification.RequestMode           `json:"request_mode"`
		WorkflowComplexity   *classification.WorkflowComplexity   `json:"workflow_complexity"`
		RiskClass            classification.RiskClass             `json:"risk_class"`
		EvidenceRequirements []classification.EvidenceRequirement `json:"evidence_requirements"`
		EscalationReasons    []string                             `json:"escalation_reasons"`
		CapabilitySelector   *classification.CapabilitySelector   `json:"capability_selector,omitempty"`
	}{"oaw.classification-decision/v1", value.RequestMode, value.WorkflowComplexity, value.RiskClass, value.EvidenceRequirements, value.EscalationReasons, value.CapabilitySelector}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil || !validDigest(value.Digest()) || digest != value.Digest() {
		return coreError("CLASSIFICATION_DIGEST_INVALID", "Classification Decision digest is invalid")
	}
	return nil
}

func validateRegistry(available catalog.Catalog, effective profile.EffectiveRegistry, hostRecord profile.HostEvidenceRecord) error {
	if effective.HostID() != hostRecord.HostID || !validDigest(effective.Digest()) {
		return coreError("HOST_PROVIDER_SCOPE_MISMATCH", "Registry does not match Host %q", hostRecord.HostID)
	}
	descriptors := make(map[string]catalog.ProviderDescriptorRecord)
	for _, descriptor := range available.Providers() {
		descriptors[descriptor.ID] = descriptor
	}
	providers := effective.Providers()
	sort.Slice(providers, func(left, right int) bool { return providers[left].ProviderID < providers[right].ProviderID })
	for index, provider := range providers {
		if index > 0 && providers[index-1].ProviderID == provider.ProviderID || provider.HostID != hostRecord.HostID || provider.BindingInventoryDigest != hostRecord.InventoryDigest {
			return coreError("RESOLUTION_DIGEST_INVALID", "Registry Provider %s is not pinned to current Host evidence", provider.ProviderID)
		}
		descriptor, found := descriptors[provider.ProviderID]
		descriptorDigest, _, digestErr := canonicaljson.Digest(descriptor)
		if !found || digestErr != nil || descriptorDigest != provider.DescriptorDigest || providerInstanceDigest(provider) != provider.Digest {
			return coreError("RESOLUTION_DIGEST_INVALID", "Registry Provider %s is malformed or stale", provider.ProviderID)
		}
		lookedUp, found := effective.Provider(provider.ProviderID)
		if !found || lookedUp.Digest != provider.Digest {
			return coreError("RESOLUTION_DIGEST_INVALID", "Registry Provider %s lookup disagrees with enumeration", provider.ProviderID)
		}
	}
	record := struct {
		SchemaVersion string                      `json:"schema_version"`
		HostID        string                      `json:"host_id"`
		Providers     []registry.ProviderInstance `json:"providers"`
	}{"oaw.effective-registry/v4", hostRecord.HostID, providers}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil || digest != effective.Digest() {
		return coreError("RESOLUTION_DIGEST_INVALID", "Registry digest is invalid")
	}
	return nil
}

func providerInstanceDigest(instance registry.ProviderInstance) string {
	record := struct {
		SchemaVersion          string                        `json:"schema_version"`
		ProviderID             string                        `json:"provider_id"`
		HostID                 string                        `json:"host_id"`
		DescriptorDigest       string                        `json:"descriptor_digest"`
		DistributionID         string                        `json:"distribution_id"`
		DistributionRevision   string                        `json:"distribution_revision"`
		DistributionTreeDigest string                        `json:"distribution_tree_digest"`
		InstallationKey        string                        `json:"installation_key"`
		ConfigurationDigest    string                        `json:"configuration_digest"`
		BindingInventoryDigest string                        `json:"binding_inventory_digest"`
		EvidenceDigest         string                        `json:"evidence_digest"`
		Bindings               []registry.VerifiedBinding    `json:"bindings"`
		Capabilities           []registry.VerifiedCapability `json:"capabilities"`
	}{
		"oaw.provider-instance/v4", instance.ProviderID, instance.HostID, instance.DescriptorDigest,
		instance.DistributionID, instance.DistributionRevision, instance.DistributionTreeDigest, instance.InstallationKey,
		instance.ConfigurationDigest, instance.BindingInventoryDigest, instance.EvidenceDigest, instance.Bindings, instance.Capabilities,
	}
	digest, _, _ := canonicaljson.Digest(record)
	return digest
}

func compilationCandidates(available catalog.Catalog) ([]profileCandidate, error) {
	recipes := make(map[string]catalog.ProfileRecipeRecord)
	for _, recipe := range available.Recipes() {
		recipes[recipe.ID] = recipe
	}
	aliased := make(map[string]struct{})
	result := make([]profileCandidate, 0, len(recipes))
	for _, alias := range available.Aliases() {
		recipe, found := recipes[alias.RecipeID]
		if !found {
			return nil, fmt.Errorf("PROFILE_TRUSTED_ALIAS_INVALID: missing Recipe %s", alias.RecipeID)
		}
		normalized, digest, err := catalog.NormalizeAndDigestRecipe(available.Providers(), recipe)
		if err != nil {
			return nil, fmt.Errorf("PROFILE_TRUSTED_RECIPE_INVALID: %w", err)
		}
		result = append(result, profileCandidate{Profile: alias.Alias, RecipeID: alias.RecipeID, Recipe: normalized, RecipeDigest: digest})
		aliased[alias.RecipeID] = struct{}{}
	}
	for _, recipe := range recipes {
		if _, found := aliased[recipe.ID]; found || strings.HasPrefix(recipe.ID, "oaw/") {
			continue
		}
		normalized, digest, err := catalog.NormalizeAndDigestRecipe(available.Providers(), recipe)
		if err != nil {
			return nil, fmt.Errorf("PROFILE_TRUSTED_RECIPE_INVALID: %w", err)
		}
		result = append(result, profileCandidate{Profile: UserDefinedProfile, RecipeID: recipe.ID, Recipe: normalized, RecipeDigest: digest})
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Profile == result[right].Profile {
			return result[left].RecipeID < result[right].RecipeID
		}
		return result[left].Profile < result[right].Profile
	})
	return result, nil
}

func compileProfileEligibility(request CompilationRequest, candidates []profileCandidate) ([]ProfileEligibility, error) {
	result := make([]ProfileEligibility, 0, len(candidates))
	for _, candidate := range candidates {
		selection, err := defaultCandidateSelection(candidate, request.Host.Record().Topology)
		if err != nil {
			return nil, err
		}
		preview, err := compileSelectionPreview(request, candidate, selection)
		if err != nil {
			return nil, err
		}
		result = append(result, ProfileEligibility{
			Profile: candidate.Profile, RecipeID: candidate.RecipeID, Eligible: preview.Graph != nil,
			Topology: selection.Topology, Diagnostics: append([]profile.CompileDiagnostic{}, preview.Diagnostics...), Preview: preview,
		})
	}
	return result, nil
}

func compileAddOnEligibility(request CompilationRequest, candidates []profileCandidate) ([]AddOnEligibility, error) {
	result := []AddOnEligibility{}
	for _, candidate := range candidates {
		for _, addOn := range candidate.Recipe.AddOns {
			selection, err := defaultCandidateSelection(candidate, request.Host.Record().Topology)
			if err != nil {
				return nil, err
			}
			selection.AddOns = []string{addOn.ID}
			preview, err := compileSelectionPreview(request, candidate, selection)
			if err != nil {
				return nil, err
			}
			result = append(result, AddOnEligibility{
				Profile: candidate.Profile, RecipeID: candidate.RecipeID, AddOnID: addOn.ID, Kind: addOn.Kind, SlotID: addOn.SlotID,
				Eligible: preview.Graph != nil, Diagnostics: append([]profile.CompileDiagnostic{}, preview.Diagnostics...), Preview: preview,
			})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		leftKey := result[left].Profile + "\x00" + result[left].RecipeID + "\x00" + result[left].AddOnID
		rightKey := result[right].Profile + "\x00" + result[right].RecipeID + "\x00" + result[right].AddOnID
		return leftKey < rightKey
	})
	return result, nil
}

func defaultCandidateSelection(candidate profileCandidate, topology execution.Topology) (Selection, error) {
	overlays, err := selectedRecipeOverlays(candidate.Recipe, nil)
	if err != nil {
		return Selection{}, err
	}
	return Selection{
		Profile: candidate.Profile, RecipeID: candidate.RecipeID, RecipeDigest: candidate.RecipeDigest,
		Topology: topology, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: overlays,
	}, nil
}

func normalizeRequestedSelection(value Selection, candidates []profileCandidate, topology execution.Topology) (profileCandidate, Selection, error) {
	if value.Profile != UserDefinedProfile && value.Profile == "" || value.RecipeID == "" || strings.TrimSpace(value.Profile) != value.Profile || strings.TrimSpace(value.RecipeID) != value.RecipeID {
		return profileCandidate{}, Selection{}, coreError("PROFILE_SELECTION_INVALID", "Profile and Recipe identity are required")
	}
	if value.ProfileSource != "" && value.ProfileSource != SelectionUser || value.TopologySource != "" && value.TopologySource != SelectionUser {
		return profileCandidate{}, Selection{}, coreError("PROFILE_SELECTION_INVALID", "invalid selection source")
	}
	if value.Topology != topology {
		return profileCandidate{}, Selection{}, coreError("PROFILE_TOPOLOGY_UNAVAILABLE", "selection topology differs from current Host evidence")
	}
	if _, err := execution.NormalizeTopologies([]execution.Topology{value.Topology}); err != nil {
		return profileCandidate{}, Selection{}, coreError("PROFILE_TOPOLOGY_UNAVAILABLE", "selection topology is invalid")
	}
	var candidate profileCandidate
	found := false
	for _, available := range candidates {
		if available.Profile == value.Profile && available.RecipeID == value.RecipeID {
			candidate = available
			found = true
			break
		}
	}
	if !found {
		return profileCandidate{}, Selection{}, coreError("PROFILE_SELECTION_INVALID", "Profile %q and Recipe %q are not selectable", value.Profile, value.RecipeID)
	}
	addOns, valid := sortedUniqueStrings(value.AddOns)
	if !valid {
		return profileCandidate{}, Selection{}, coreError("PROFILE_SELECTION_INVALID", "Add-on selection is invalid or duplicated")
	}
	overlays, err := selectedRecipeOverlays(candidate.Recipe, value.Overlays)
	if err != nil {
		return profileCandidate{}, Selection{}, err
	}
	value.AddOns = addOns
	value.Overlays = overlays
	value.Alternatives = append([]profile.AlternativeChoice{}, value.Alternatives...)
	return candidate, value, nil
}

func compileSelectionPreview(request CompilationRequest, candidate profileCandidate, selection Selection) (SelectionPreview, error) {
	addOns, valid := sortedUniqueStrings(selection.AddOns)
	if !valid {
		return SelectionPreview{}, coreError("PROFILE_SELECTION_INVALID", "Add-on selection is invalid or duplicated")
	}
	overlays, err := selectedRecipeOverlays(candidate.Recipe, selection.Overlays)
	if err != nil {
		return SelectionPreview{}, err
	}
	compileRequest := profile.CompileRequest{
		Profile: candidate.Profile, Topology: selection.Topology, AddOns: addOns,
		Alternatives: append([]profile.AlternativeChoice{}, selection.Alternatives...), Overlays: overlays, Host: request.Host,
	}
	var compiled profile.CompileResult
	if candidate.Profile == UserDefinedProfile {
		compiled, err = profile.CompileRecipe(request.Configuration.Catalog(), request.Registry, candidate.Recipe, compileRequest)
	} else {
		compiled, err = profile.CompileProfile(request.Configuration.Catalog(), request.Registry, compileRequest)
	}
	if err != nil {
		return SelectionPreview{}, err
	}
	preview := SelectionPreview{
		Selection: Selection{
			Profile: candidate.Profile, RecipeID: candidate.RecipeID, RecipeDigest: candidate.RecipeDigest,
			ProfileSource: selection.ProfileSource, Topology: selection.Topology, TopologySource: selection.TopologySource,
			AddOns: addOns, Alternatives: append([]profile.AlternativeChoice{}, selection.Alternatives...), Overlays: overlays,
		},
		Recipe: candidate.Recipe, ProviderInstances: []profile.GraphProviderInstance{}, Diagnostics: compiled.Diagnostics(),
	}
	if graph, found := compiled.Graph(); found {
		preview.Selection.RecipeDigest = graph.RecipeDigest
		preview.Selection.AddOns = append([]string{}, graph.Selection.AddOns...)
		preview.Selection.Alternatives = append([]profile.AlternativeChoice{}, graph.Selection.Alternatives...)
		preview.Selection.Overlays = append([]string{}, graph.Selection.Overlays...)
		preview.Selection.GraphSelectionDigest = graph.Selection.Digest
		preview.ProviderInstances = append([]profile.GraphProviderInstance{}, graph.ProviderInstances...)
		preview.Graph = &graph
		preview.Selection.ConfirmationDigest = selectionConfirmationDigest(request.Registry.Digest(), request.Host.Digest(), preview)
	}
	preview.Digest = selectionPreviewDigest(preview)
	return preview, nil
}

func selectedRecipeOverlays(recipe catalog.ProfileRecipeRecord, requested []string) ([]string, error) {
	roots := append([]string{}, requested...)
	if len(roots) == 0 && recipe.Template == "default" {
		for _, overlay := range recipe.Overlays {
			roots = append(roots, overlay.ID)
		}
	}
	declared := make(map[string]catalog.OverlayRecord, len(recipe.Overlays))
	for _, overlay := range recipe.Overlays {
		declared[overlay.ID] = overlay
	}
	selected := make(map[string]struct{})
	for _, root := range roots {
		overlay, found := declared[root]
		if !found {
			return nil, coreError("PROFILE_SELECTION_INVALID", "overlay %q is not declared by Recipe %q", root, recipe.ID)
		}
		for _, id := range overlay.Precedence {
			if _, found := declared[id]; !found {
				return nil, fmt.Errorf("PROFILE_TRUSTED_RECIPE_INVALID: overlay %q has unknown precedence %q", root, id)
			}
			selected[id] = struct{}{}
		}
		selected[root] = struct{}{}
	}
	result := make([]string, 0, len(selected))
	for _, overlay := range recipe.Overlays {
		if _, found := selected[overlay.ID]; found {
			result = append(result, overlay.ID)
		}
	}
	return result, nil
}

func compileBundle(request CompilationRequest, candidate profileCandidate, preview SelectionPreview, hostRecord profile.HostEvidenceRecord) (LifecycleBundle, error) {
	graph := *preview.Graph
	selection := preview.Selection
	selection.ProfileSource = request.Selection.ProfileSource
	selection.TopologySource = request.Selection.TopologySource
	bundle := LifecycleBundle{
		SchemaVersion: LifecycleBundleSchemaV4, DeliverableID: request.DeliverableID, InputDigest: request.InputDigest, Generation: request.Generation,
		Classification: cloneClassification(request.Classification), ClassificationDigest: request.Classification.Digest(),
		Selection: selection, Recipe: candidate.Recipe, RecipeDigest: candidate.RecipeDigest,
		HostID: hostRecord.HostID, HostSessionDigest: hostRecord.SessionDigest, HostManifestDigest: hostRecord.ManifestDigest,
		EnvironmentReportDigest: hostRecord.EnvironmentDigest, ProviderInventoryDigest: hostRecord.InventoryDigest,
		HostFeatureDigest: hostRecord.FeatureDigest, HostActionDigest: hostRecord.ActionDigest, HostEvidenceDigest: hostRecord.Digest,
		Configuration: request.Configuration.Record(), ResolutionDigest: request.ResolutionDigest, RegistryDigest: request.Registry.Digest(),
		ProviderInstances: append([]profile.GraphProviderInstance{}, graph.ProviderInstances...), Graph: graph, Topology: graph.Topology,
		EnvironmentRequirements: cloneRequirements(graph.EnvironmentRequirements), AddOns: append([]string{}, selection.AddOns...),
	}
	seedDigest, _, err := canonicaljson.Digest(bundle)
	if err != nil {
		return LifecycleBundle{}, err
	}
	bundle.ID = "bundle-" + seedDigest[:32]
	bundle.Digest, _, err = canonicaljson.Digest(bundle)
	if err != nil {
		return LifecycleBundle{}, err
	}
	return bundle, nil
}

func markRecommendation(values []ProfileEligibility, decision classification.ClassificationDecision) {
	preferred := []string{"SP-FULL", "MATT-SP-HYBRID", "MATT-FULL", "ECC-FULL"}
	reason := "ORDINARY_WORKFLOW_DELIVERY"
	if decision.WorkflowComplexity != nil && *decision.WorkflowComplexity == classification.ComplexityComplex {
		preferred = []string{"MATT-SP-HYBRID", "MATT-FULL", "SP-FULL", "ECC-FULL"}
		reason = "COMPLEX_WORKFLOW_RELIABILITY"
	}
	for _, profileID := range preferred {
		for index := range values {
			if values[index].Profile == profileID && values[index].Eligible {
				values[index].Recommended = true
				values[index].RecommendationReason = reason
				return
			}
		}
	}
	for index := range values {
		if values[index].Eligible {
			values[index].Recommended = true
			values[index].RecommendationReason = "ONLY_ELIGIBLE_PROFILE"
			return
		}
	}
}

func selectionConfirmationDigest(registryDigest, hostEvidenceDigest string, preview SelectionPreview) string {
	selection := preview.Selection
	selection.ProfileSource = ""
	selection.TopologySource = ""
	selection.ConfirmationDigest = ""
	value := struct {
		Selection          Selection                       `json:"selection"`
		Recipe             catalog.ProfileRecipeRecord     `json:"recipe"`
		RegistryDigest     string                          `json:"registry_digest"`
		HostEvidenceDigest string                          `json:"host_evidence_digest"`
		ProviderInstances  []profile.GraphProviderInstance `json:"provider_instances"`
		Graph              *profile.ExecutionGraphRecord   `json:"execution_graph"`
	}{selection, preview.Recipe, registryDigest, hostEvidenceDigest, preview.ProviderInstances, preview.Graph}
	digest, _, _ := canonicaljson.Digest(value)
	return digest
}

func selectionPreviewDigest(value SelectionPreview) string {
	value.Digest = ""
	digest, _, _ := canonicaljson.Digest(value)
	return digest
}

func compilationResultDigest(value CompilationResult) (string, error) {
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	return digest, err
}

func cloneClassification(value classification.ClassificationDecision) classification.ClassificationDecision {
	if value.WorkflowComplexity != nil {
		complexity := *value.WorkflowComplexity
		value.WorkflowComplexity = &complexity
	}
	value.EvidenceRequirements = append([]classification.EvidenceRequirement{}, value.EvidenceRequirements...)
	value.EscalationReasons = append([]string{}, value.EscalationReasons...)
	if value.CapabilitySelector != nil {
		selector := *value.CapabilitySelector
		value.CapabilitySelector = &selector
	}
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

func sortedUniqueStrings(values []string) ([]string, bool) {
	result := append([]string{}, values...)
	sort.Strings(result)
	for index := range result {
		if strings.TrimSpace(result[index]) != result[index] || result[index] == "" || index > 0 && result[index-1] == result[index] {
			return nil, false
		}
	}
	return result, true
}

func validIdentifier(value string) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 512 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
