package core

import (
	"fmt"
	"slices"
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

const lifecycleBundleSchemaV3 = "oaw.lifecycle-bundle/v3"

type profileCandidate struct {
	Profile  string
	RecipeID string
	Recipe   catalog.ProfileRecipeRecord
}

func Compile(request CompilationRequest) (CompilationResult, error) {
	hostTopologies, observations, err := validateCompilationRequest(request)
	if err != nil {
		return CompilationResult{}, err
	}
	candidates := compilationCandidates(request.Configuration.Catalog())
	profiles := compileProfileEligibility(request, candidates, hostTopologies, observations)
	markRecommendation(profiles, request.Classification)
	addOns := compileAddOnEligibility(request, candidates, hostTopologies, observations)
	result := CompilationResult{EligibleProfiles: profiles, EligibleAddOns: addOns}
	if request.Selection != nil {
		bundle, err := compileBundle(request, candidates, hostTopologies, observations)
		if err != nil {
			return CompilationResult{}, err
		}
		result.Bundle = &bundle
	}
	result.Digest, err = compilationResultDigest(result)
	if err != nil {
		return CompilationResult{}, err
	}
	return result, nil
}

func validateCompilationRequest(request CompilationRequest) ([]execution.Topology, []execution.EnvironmentObservation, error) {
	if !validIdentifier(request.DeliverableID) || !validDigest(request.InputDigest) || request.Generation == 0 {
		return nil, nil, coreError("CORE_INPUT_INVALID", "invalid Deliverable identity")
	}
	if _, err := catalog.ParseLocalID(request.HostID); err != nil {
		return nil, nil, coreError("HOST_PROVIDER_SCOPE_MISMATCH", "invalid Host %q", request.HostID)
	}
	if !validDigest(request.HostSessionDigest) || !validDigest(request.HostEnvironmentReportDigest) || !validDigest(request.HostProviderInventoryDigest) {
		return nil, nil, coreError("CORE_INPUT_INVALID", "Host snapshot digests are invalid")
	}
	if err := validateClassification(request.Classification); err != nil {
		return nil, nil, err
	}
	if request.Classification.RequestMode != classification.RequestModeWorkflow {
		return nil, nil, coreError("CORE_INPUT_INVALID", "Lifecycle compilation requires WORKFLOW classification")
	}
	if err := validateConfiguration(request.Configuration); err != nil {
		return nil, nil, err
	}
	if request.Resolutions.HostID() != request.HostID || request.Registry.HostID() != request.HostID {
		return nil, nil, coreError("HOST_PROVIDER_SCOPE_MISMATCH", "Resolution Report and Registry do not match Host %q", request.HostID)
	}
	if !validDigest(request.Resolutions.Digest()) || !validDigest(request.Registry.Digest()) {
		return nil, nil, coreError("CORE_INPUT_INVALID", "Resolution or Registry digest is invalid")
	}
	if err := validateResolutionPair(request); err != nil {
		return nil, nil, err
	}
	hostTopologies, err := execution.NormalizeTopologies(request.HostTopologies)
	if err != nil {
		return nil, nil, err
	}
	observations, err := normalizeObservations(request.EnvironmentObservations)
	if err != nil {
		return nil, nil, err
	}
	return hostTopologies, observations, nil
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

func validateResolutionPair(request CompilationRequest) error {
	verified := make(map[string]string)
	for _, resolution := range request.Resolutions.Resolutions() {
		if resolution.State != registry.Verified {
			if resolution.Instance != nil {
				return coreError("RESOLUTION_DIGEST_INVALID", "non-verified Provider %s has an Instance", resolution.ProviderID)
			}
			continue
		}
		if resolution.Instance == nil {
			return coreError("RESOLUTION_DIGEST_INVALID", "verified Provider %s has no Instance", resolution.ProviderID)
		}
		provider, found := request.Registry.Provider(resolution.ProviderID)
		if !found || provider.Digest != resolution.Instance.Digest || provider.HostID != request.HostID {
			return coreError("RESOLUTION_DIGEST_INVALID", "Resolution and Registry disagree for %s", resolution.ProviderID)
		}
		verified[resolution.ProviderID] = provider.Digest
	}
	descriptors := make(map[string]catalog.ProviderDescriptorRecord)
	for _, descriptor := range request.Configuration.Catalog().Providers() {
		descriptors[descriptor.ID] = descriptor
	}
	for _, provider := range request.Registry.Providers() {
		if verified[provider.ProviderID] != provider.Digest || provider.BindingInventoryDigest != request.HostProviderInventoryDigest {
			return coreError("RESOLUTION_DIGEST_INVALID", "Registry Provider %s is not pinned to the supplied resolution and inventory", provider.ProviderID)
		}
		descriptor, found := descriptors[provider.ProviderID]
		if !found {
			return coreError("RESOLUTION_DIGEST_INVALID", "Registry Provider %s has no configured descriptor", provider.ProviderID)
		}
		digest, _, err := canonicaljson.Digest(descriptor)
		if err != nil || digest != provider.DescriptorDigest {
			return coreError("RESOLUTION_DIGEST_INVALID", "Registry Provider %s descriptor digest is invalid", provider.ProviderID)
		}
	}
	return nil
}

func compilationCandidates(available catalog.Catalog) []profileCandidate {
	recipes := make(map[string]catalog.ProfileRecipeRecord)
	for _, recipe := range available.Recipes() {
		recipes[recipe.ID] = recipe
	}
	aliased := make(map[string]struct{})
	result := make([]profileCandidate, 0, len(recipes))
	for _, alias := range available.Aliases() {
		if recipe, found := recipes[alias.RecipeID]; found {
			result = append(result, profileCandidate{Profile: alias.Alias, RecipeID: alias.RecipeID, Recipe: recipe})
			aliased[alias.RecipeID] = struct{}{}
		}
	}
	for _, recipe := range recipes {
		if _, found := aliased[recipe.ID]; found || strings.HasPrefix(recipe.ID, "oaw/") {
			continue
		}
		result = append(result, profileCandidate{Profile: recipe.ID, RecipeID: recipe.ID, Recipe: recipe})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Profile < result[right].Profile })
	return result
}

func compileProfileEligibility(request CompilationRequest, candidates []profileCandidate, hostTopologies []execution.Topology, observations []execution.EnvironmentObservation) []ProfileEligibility {
	result := make([]ProfileEligibility, 0, len(candidates))
	for _, candidate := range candidates {
		graph, err := profile.CompileProfile(request.Configuration.Catalog(), request.Registry, profile.CompileRequest{
			Profile: candidate.Profile, HostTopologies: hostTopologies, EnvironmentObservations: observations,
		})
		eligibility := ProfileEligibility{Profile: candidate.Profile, RecipeID: candidate.RecipeID, EligibleTopologies: []execution.Topology{}, Diagnostics: []EligibilityDiagnostic{}}
		if err != nil {
			eligibility.Diagnostics = diagnosticsForCompileError(request.Resolutions, err)
		} else {
			eligibility.Eligible = true
			eligibility.EligibleTopologies = graph.EligibleTopologies()
		}
		result = append(result, eligibility)
	}
	return result
}

func compileAddOnEligibility(request CompilationRequest, candidates []profileCandidate, hostTopologies []execution.Topology, observations []execution.EnvironmentObservation) []AddOnEligibility {
	seen := make(map[string]struct{})
	result := []AddOnEligibility{}
	for _, candidate := range candidates {
		for _, node := range candidate.Recipe.Nodes {
			if !node.Optional {
				continue
			}
			key := node.ID + "\x00" + node.Selector.ProviderID + "\x00" + node.Selector.CapabilityID
			if _, found := seen[key]; found {
				continue
			}
			seen[key] = struct{}{}
			entry := AddOnEligibility{NodeID: node.ID, ProviderID: node.Selector.ProviderID, CapabilityID: node.Selector.CapabilityID, EligibleTopologies: []execution.Topology{}, Diagnostics: []EligibilityDiagnostic{}}
			graph, err := profile.CompileProfile(request.Configuration.Catalog(), request.Registry, profile.CompileRequest{
				Profile: candidate.Profile, AddOns: []string{node.ID}, HostTopologies: hostTopologies, EnvironmentObservations: observations,
			})
			if err != nil {
				entry.Diagnostics = diagnosticsForCompileError(request.Resolutions, err)
			} else {
				entry.EligibleTopologies = graph.EligibleTopologies()
				for _, graphNode := range graph.Nodes() {
					if graphNode.ID == node.ID {
						entry.ProviderID = graphNode.ProviderID
						entry.CapabilityID = graphNode.CapabilityID
						break
					}
				}
			}
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return addOnKey(result[left]) < addOnKey(result[right])
	})
	return result
}

func diagnosticsForCompileError(report registry.ResolutionReport, err error) []EligibilityDiagnostic {
	diagnostic := EligibilityDiagnostic{Code: "PROFILE_UNAVAILABLE", Detail: err.Error()}
	if compileErr, ok := err.(*profile.CompileError); ok {
		diagnostic.Code = compileErr.Code
		diagnostic.ProviderID = compileErr.ProviderID
		diagnostic.CapabilityID = compileErr.CapabilityID
		if compileErr.ProviderID != "" {
			if resolution, found := report.Resolution(compileErr.ProviderID); found && resolution.State != registry.Verified {
				diagnostic.Code = resolution.Reason
				diagnostic.Detail = fmt.Sprintf("%s: %s", resolution.Reason, err)
			}
		}
	}
	return []EligibilityDiagnostic{diagnostic}
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

func compileBundle(request CompilationRequest, candidates []profileCandidate, hostTopologies []execution.Topology, observations []execution.EnvironmentObservation) (LifecycleBundle, error) {
	selection, err := normalizeSelection(*request.Selection)
	if err != nil {
		return LifecycleBundle{}, err
	}
	if !candidatePresent(candidates, selection.Profile) {
		return LifecycleBundle{}, coreError("PROFILE_SELECTION_INVALID", "Profile %q is not selectable", selection.Profile)
	}
	graph, err := profile.CompileProfile(request.Configuration.Catalog(), request.Registry, profile.CompileRequest{
		Profile: selection.Profile, Bindings: selection.Bindings, AddOns: selection.AddOns,
		HostTopologies: hostTopologies, EnvironmentObservations: observations,
	})
	if err != nil {
		if compileErr, ok := err.(*profile.CompileError); ok && compileErr.Code == "PROFILE_ADD_ON_INVALID" {
			return LifecycleBundle{}, err
		}
		return LifecycleBundle{}, coreError("PROFILE_SELECTION_INVALID", "%v", err)
	}
	eligibleTopologies := graph.EligibleTopologies()
	if !slices.Contains(eligibleTopologies, selection.Topology) {
		return LifecycleBundle{}, coreError("PROFILE_TOPOLOGY_UNAVAILABLE", "Profile %q does not support %s", selection.Profile, selection.Topology)
	}
	if len(eligibleTopologies) == 1 && eligibleTopologies[0] == execution.TopologyCurrent {
		if selection.TopologySource != SelectionHostOnlyOption {
			return LifecycleBundle{}, coreError("PROFILE_TOPOLOGY_SOURCE_INVALID", "CURRENT is the sole Host option")
		}
	} else if selection.TopologySource != SelectionUser {
		return LifecycleBundle{}, coreError("PROFILE_TOPOLOGY_SOURCE_INVALID", "topology requires explicit user selection")
	}
	selection.Bindings = graph.Bindings()
	graphRecord := graph.Record()
	bundle := LifecycleBundle{
		SchemaVersion: lifecycleBundleSchemaV3, DeliverableID: request.DeliverableID, InputDigest: request.InputDigest, Generation: request.Generation,
		Classification: cloneClassification(request.Classification), ClassificationDigest: request.Classification.Digest(), Selection: selection,
		HostID: request.HostID, HostSessionDigest: request.HostSessionDigest, EnvironmentReportDigest: request.HostEnvironmentReportDigest,
		ProviderInventoryDigest: request.HostProviderInventoryDigest,
		Configuration:           request.Configuration.Record(), ResolutionDigest: request.Resolutions.Digest(), RegistryDigest: request.Registry.Digest(),
		ProviderInstances: append([]profile.GraphProviderInstance{}, graphRecord.ProviderInstances...), Graph: graphRecord, Topology: selection.Topology,
		EnvironmentRequirements: cloneRequirements(graphRecord.EnvironmentRequirements), EnvironmentObservations: append([]execution.EnvironmentObservation{}, observations...),
		AddOns: append([]string{}, selection.AddOns...),
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

func normalizeSelection(value Selection) (Selection, error) {
	if value.ProfileSource != SelectionUser || value.Profile == "" || strings.TrimSpace(value.Profile) != value.Profile || strings.ContainsAny(value.Profile, "\r\n\x00") {
		return Selection{}, coreError("PROFILE_SELECTION_INVALID", "Profile and source are invalid")
	}
	if _, err := execution.NormalizeTopologies([]execution.Topology{value.Topology}); err != nil {
		return Selection{}, err
	}
	value.AddOns = append([]string{}, value.AddOns...)
	sort.Strings(value.AddOns)
	for index, addOn := range value.AddOns {
		if addOn == "" || strings.TrimSpace(addOn) != addOn || index > 0 && value.AddOns[index-1] == addOn {
			return Selection{}, coreError("PROFILE_ADD_ON_INVALID", "add-on selection is invalid or duplicated")
		}
	}
	value.Bindings = append([]profile.ProfileBinding{}, value.Bindings...)
	sort.Slice(value.Bindings, func(left, right int) bool {
		return bindingKey(value.Bindings[left]) < bindingKey(value.Bindings[right])
	})
	for index, binding := range value.Bindings {
		if _, err := catalog.ParseQualifiedID(binding.Selector.ProviderID); err != nil {
			return Selection{}, coreError("PROFILE_SELECTION_INVALID", "invalid binding Provider")
		}
		if _, err := catalog.ParseLocalID(binding.Selector.CapabilityID); err != nil {
			return Selection{}, coreError("PROFILE_SELECTION_INVALID", "invalid binding Capability")
		}
		if _, err := catalog.ParseQualifiedID(binding.PreferredProviderID); err != nil {
			return Selection{}, coreError("PROFILE_SELECTION_INVALID", "invalid preferred Provider")
		}
		if index > 0 && selectorKey(value.Bindings[index-1]) == selectorKey(binding) {
			return Selection{}, coreError("PROFILE_SELECTION_INVALID", "duplicate Profile Binding")
		}
	}
	return value, nil
}

func normalizeObservations(values []execution.EnvironmentObservation) ([]execution.EnvironmentObservation, error) {
	result := append([]execution.EnvironmentObservation{}, values...)
	if err := execution.RequirementsSatisfied([]execution.EnvironmentRequirement{}, result); err != nil {
		return nil, err
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Surface < result[right].Surface })
	return result, nil
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

func candidatePresent(values []profileCandidate, wanted string) bool {
	for _, value := range values {
		if value.Profile == wanted {
			return true
		}
	}
	return false
}

func addOnKey(value AddOnEligibility) string {
	return value.NodeID + "\x00" + value.ProviderID + "\x00" + value.CapabilityID
}

func bindingKey(value profile.ProfileBinding) string {
	return selectorKey(value) + "\x00" + value.PreferredProviderID
}

func selectorKey(value profile.ProfileBinding) string {
	return value.Selector.ProviderID + "\x00" + value.Selector.CapabilityID
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
