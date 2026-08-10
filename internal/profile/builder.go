package profile

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

type BuilderCandidate struct {
	ProviderID            string              `json:"provider_id"`
	BindingID             string              `json:"binding_id"`
	Kind                  catalog.BindingKind `json:"kind"`
	Topology              execution.Topology  `json:"topology"`
	InputArtifact         string              `json:"input_artifact"`
	OutputArtifact        string              `json:"output_artifact"`
	MaximumEffects        []string            `json:"maximum_effects"`
	Resources             []string            `json:"resources"`
	RequiredFeatures      []host.FeatureID    `json:"required_features"`
	Compatible            bool                `json:"compatible"`
	Diagnostics           []CompileDiagnostic `json:"diagnostics"`
	BindingEvidenceDigest string              `json:"binding_evidence_digest"`
}

type BuilderSlot struct {
	SlotID           catalog.SlotID          `json:"slot_id"`
	EntryArtifact    string                  `json:"entry_artifact"`
	OutcomeArtifact  string                  `json:"outcome_artifact"`
	SelectedPipeline []catalog.PipelineStep  `json:"selected_pipeline"`
	SelectedOwner    catalog.OutcomeOwner    `json:"selected_owner"`
	MacroPreview     []ResolvedBinding       `json:"macro_preview"`
	HostAction       *CompiledHostAction     `json:"host_action,omitempty"`
	Gates            []CompiledGate          `json:"gates"`
	IncidentRoutes   []CompiledIncidentRoute `json:"incident_routes"`
	Candidates       []BuilderCandidate      `json:"candidates"`
	Diagnostics      []CompileDiagnostic     `json:"diagnostics"`
}

type BuilderBaseKind string

const (
	BuilderBaseCanonical BuilderBaseKind = "canonical-lifecycle"
	BuilderBaseRecipe    BuilderBaseKind = "recipe"
)

type BuilderSelectionRequest struct {
	Profile      string              `json:"profile"`
	Topology     execution.Topology  `json:"topology"`
	AddOns       []string            `json:"add_ons"`
	Alternatives []AlternativeChoice `json:"alternatives"`
	Overlays     []string            `json:"overlays"`
}

type BuilderProjection struct {
	TaxonomyVersion    string                  `json:"taxonomy_version"`
	BaseKind           BuilderBaseKind         `json:"base_kind"`
	BaseRecipeID       string                  `json:"base_recipe_id"`
	BaseDigest         string                  `json:"base_digest"`
	HostEvidenceDigest string                  `json:"host_evidence_digest"`
	RegistryDigest     string                  `json:"registry_digest"`
	Request            BuilderSelectionRequest `json:"request"`
	Selection          *Selection              `json:"selection,omitempty"`
	SelectionDigest    string                  `json:"selection_digest,omitempty"`
	Slots              []BuilderSlot           `json:"slots"`
	PreviewGraph       *ExecutionGraphRecord   `json:"preview_graph,omitempty"`
	PreviewGraphDigest string                  `json:"preview_graph_digest,omitempty"`
	Diagnostics        []CompileDiagnostic     `json:"diagnostics"`
	ConfirmationDigest string                  `json:"confirmation_digest,omitempty"`
	Digest             string                  `json:"digest"`
}

type ConfirmedRecipe struct {
	Recipe             catalog.ProfileRecipeRecord `json:"recipe"`
	RecipeDigest       string                      `json:"recipe_digest"`
	Selection          Selection                   `json:"selection"`
	RegistryDigest     string                      `json:"registry_digest"`
	ProviderInstances  []GraphProviderInstance     `json:"provider_instances"`
	HostEvidenceDigest string                      `json:"host_evidence_digest"`
	Graph              ExecutionGraphRecord        `json:"graph"`
	ConfirmationDigest string                      `json:"confirmation_digest"`
	Digest             string                      `json:"digest"`
}

type RecipeEdit struct {
	SlotID       catalog.SlotID         `json:"slot_id"`
	Pipeline     []catalog.PipelineStep `json:"pipeline"`
	OutcomeOwner catalog.OutcomeOwner   `json:"outcome_owner"`
}

func NewRecipe(newID, version string) (catalog.ProfileRecipeRecord, error) {
	if _, err := catalog.ParseQualifiedID(newID); err != nil {
		return catalog.ProfileRecipeRecord{}, fmt.Errorf("PROFILE_RECIPE_INVALID: %w", err)
	}
	if _, err := catalog.ParseContentVersion(version); err != nil {
		return catalog.ProfileRecipeRecord{}, fmt.Errorf("PROFILE_RECIPE_INVALID: %w", err)
	}
	slots := make([]catalog.SlotRecipe, len(catalog.CanonicalSlots()))
	for index, definition := range catalog.CanonicalSlots() {
		applicability := catalog.SlotMandatory
		owner := catalog.OutcomeOwner{Kind: catalog.OwnerNone}
		if definition.ID == catalog.SlotIncidentRecovery {
			applicability = catalog.SlotConditional
		}
		slots[index] = catalog.SlotRecipe{SlotID: definition.ID, Applicability: applicability, OutcomeOwner: owner, Pipeline: []catalog.PipelineStep{}, Gates: []catalog.GateRecord{}, Transitions: []catalog.RecipeTransition{}}
	}
	return catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV3, TaxonomyVersion: catalog.TaxonomyVersionV1, RecipeVersion: version, ID: newID, DisplayName: newID, Family: "user-defined",
		Slots: slots, AddOns: []catalog.AddOnRecord{}, IncidentRoutes: []catalog.IncidentRoute{}, Overlays: []catalog.OverlayRecord{}, StableBoundaries: []string{}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}, nil
}

func CloneRecipe(source CatalogSource, base, newID, version string) (catalog.ProfileRecipeRecord, error) {
	for _, recipe := range source.Recipes() {
		if recipe.ID != base {
			continue
		}
		normalized, _, err := catalog.NormalizeAndDigestRecipe(source.Providers(), recipe)
		if err != nil {
			return catalog.ProfileRecipeRecord{}, err
		}
		normalized.ID = newID
		normalized.RecipeVersion = version
		normalized.DisplayName = newID
		_, _, err = catalog.NormalizeAndDigestRecipe(source.Providers(), normalized)
		if err != nil {
			return catalog.ProfileRecipeRecord{}, err
		}
		return normalized, nil
	}
	return catalog.ProfileRecipeRecord{}, fmt.Errorf("PROFILE_RECIPE_NOT_FOUND: %s", base)
}

func EditRecipe(recipe catalog.ProfileRecipeRecord, edits []RecipeEdit) (catalog.ProfileRecipeRecord, error) {
	value := cloneRecipeRecord(recipe)
	seen := make(map[catalog.SlotID]struct{}, len(edits))
	for _, edit := range edits {
		if _, duplicate := seen[edit.SlotID]; duplicate {
			return catalog.ProfileRecipeRecord{}, fmt.Errorf("PROFILE_RECIPE_EDIT_INVALID: duplicate slot")
		}
		seen[edit.SlotID] = struct{}{}
		found := false
		for index := range value.Slots {
			if value.Slots[index].SlotID == edit.SlotID {
				value.Slots[index].Pipeline = clonePipelineSteps(edit.Pipeline)
				value.Slots[index].OutcomeOwner = edit.OutcomeOwner
				found = true
				break
			}
		}
		if !found {
			return catalog.ProfileRecipeRecord{}, fmt.Errorf("PROFILE_RECIPE_EDIT_INVALID: unknown slot")
		}
	}
	return value, nil
}

func BuildProjection(source CatalogSource, verified EffectiveRegistry, evidence HostEvidence, recipe catalog.ProfileRecipeRecord, baseKind BuilderBaseKind, base string, request BuilderSelectionRequest) (BuilderProjection, error) {
	if baseKind != BuilderBaseCanonical && baseKind != BuilderBaseRecipe {
		return BuilderProjection{}, fmt.Errorf("PROFILE_BUILDER_INVALID: unknown base kind")
	}
	if err := ValidateHostEvidenceRecord(evidence.Record()); err != nil {
		return BuilderProjection{}, err
	}
	normalizedRecipe, recipeDigest, normalizeErr := catalog.NormalizeAndDigestRecipe(source.Providers(), recipe)
	incomplete := normalizeErr != nil
	if incomplete {
		if !validBuilderDraftShape(recipe) {
			return BuilderProjection{}, normalizeErr
		}
		normalizedRecipe = cloneRecipeRecord(recipe)
		var err error
		recipeDigest, _, err = canonicaljson.Digest(normalizedRecipe)
		if err != nil {
			return BuilderProjection{}, err
		}
	}
	var err error
	baseDigest := ""
	baseRecipeID := ""
	if baseKind == BuilderBaseCanonical {
		baseDigest, _, err = canonicaljson.Digest(struct {
			TaxonomyVersion string                   `json:"taxonomy_version"`
			Slots           []catalog.SlotDefinition `json:"slots"`
		}{catalog.TaxonomyVersionV1, catalog.CanonicalSlots()})
		if err != nil {
			return BuilderProjection{}, err
		}
	} else {
		for _, candidate := range source.Recipes() {
			if candidate.ID != base {
				continue
			}
			_, baseDigest, err = catalog.NormalizeAndDigestRecipe(source.Providers(), candidate)
			if err != nil {
				return BuilderProjection{}, err
			}
			baseRecipeID = candidate.ID
			break
		}
		if baseRecipeID == "" {
			return BuilderProjection{}, fmt.Errorf("PROFILE_BUILDER_INVALID: base Recipe mismatch")
		}
	}
	if baseKind == BuilderBaseCanonical && base != "" {
		return BuilderProjection{}, fmt.Errorf("PROFILE_BUILDER_INVALID: canonical base must not name a Recipe")
	}
	request = normalizeBuilderRequest(request)
	slots, slotsErr := builderSlots(normalizedRecipe, source.Providers(), verified, evidence.Record(), request.Topology)
	if slotsErr != nil {
		return BuilderProjection{}, slotsErr
	}
	projection := BuilderProjection{
		TaxonomyVersion: catalog.TaxonomyVersionV1, BaseKind: baseKind, BaseRecipeID: baseRecipeID, BaseDigest: baseDigest,
		HostEvidenceDigest: evidence.Digest(), RegistryDigest: verified.Digest(), Request: request, Slots: slots, Diagnostics: []CompileDiagnostic{},
	}
	if incomplete {
		projection.Diagnostics = []CompileDiagnostic{{Code: "PROFILE_DRAFT_INCOMPLETE", Detail: "Recipe draft is not complete enough to compile"}}
		projection.Digest = builderProjectionDigest(projection)
		return projection, nil
	}
	compileRequest := CompileRequest{Profile: request.Profile, Topology: request.Topology, AddOns: request.AddOns, Alternatives: request.Alternatives, Overlays: request.Overlays, Host: evidence}
	selection, selectionDiagnostics, selectionErr := normalizeSelection(source, normalizedRecipe, recipeDigest, compileRequest)
	if selectionErr != nil {
		return BuilderProjection{}, selectionErr
	}
	if len(selectionDiagnostics) == 0 {
		selectionCopy := cloneSelection(selection)
		projection.Selection = &selectionCopy
		projection.SelectionDigest = selection.Digest
	}
	result, compileErr := CompileRecipe(source, verified, normalizedRecipe, compileRequest)
	if compileErr != nil {
		return BuilderProjection{}, compileErr
	}
	projection.Diagnostics = result.Diagnostics()
	for _, diagnostic := range projection.Diagnostics {
		for index := range projection.Slots {
			if projection.Slots[index].SlotID == diagnostic.SlotID {
				projection.Slots[index].Diagnostics = append(projection.Slots[index].Diagnostics, diagnostic)
			}
		}
	}
	if graph, found := result.Graph(); found {
		selection := graph.Selection
		projection.Selection = &selection
		projection.SelectionDigest = selection.Digest
		projection.PreviewGraph = &graph
		projection.PreviewGraphDigest = graph.Digest
		projectCompiledSlots(projection.Slots, graph)
	}
	if projection.Selection != nil && len(projection.Diagnostics) == 0 {
		projection.ConfirmationDigest = confirmationDigest(recipeDigest, request, projection)
	}
	projection.Digest = builderProjectionDigest(projection)
	return projection, nil
}

func ConfirmRecipe(source CatalogSource, verified EffectiveRegistry, evidence HostEvidence, recipe catalog.ProfileRecipeRecord, request BuilderSelectionRequest, projection BuilderProjection, expectedConfirmationDigest string) (ConfirmedRecipe, error) {
	if err := ValidateBuilderProjection(projection); err != nil || projection.ConfirmationDigest == "" || expectedConfirmationDigest == "" || projection.ConfirmationDigest != expectedConfirmationDigest {
		return ConfirmedRecipe{}, fmt.Errorf("PROFILE_SELECTION_INVALID: confirmation digest mismatch")
	}
	current, err := BuildProjection(source, verified, evidence, recipe, projection.BaseKind, projection.BaseRecipeID, request)
	if err != nil {
		return ConfirmedRecipe{}, err
	}
	if current.Digest != projection.Digest || current.ConfirmationDigest != expectedConfirmationDigest {
		return ConfirmedRecipe{}, fmt.Errorf("PROFILE_SELECTION_INVALID: trusted inputs or preview changed")
	}
	graph := current.PreviewGraph
	if graph == nil {
		return ConfirmedRecipe{}, fmt.Errorf("PROFILE_SELECTION_INVALID: preview has no valid graph")
	}
	normalized, recipeDigest, err := catalog.NormalizeAndDigestRecipe(source.Providers(), recipe)
	if err != nil {
		return ConfirmedRecipe{}, err
	}
	selection := *current.Selection
	confirmed := ConfirmedRecipe{Recipe: normalized, RecipeDigest: recipeDigest, Selection: selection, RegistryDigest: verified.Digest(), ProviderInstances: append([]GraphProviderInstance{}, graph.ProviderInstances...), HostEvidenceDigest: evidence.Digest(), Graph: *graph, ConfirmationDigest: expectedConfirmationDigest}
	confirmed.Digest = confirmedDigest(confirmed)
	return confirmed, nil
}

func CloneBuilderProjection(value BuilderProjection) BuilderProjection {
	value.Request = normalizeBuilderRequest(value.Request)
	if value.Selection != nil {
		selection := cloneSelection(*value.Selection)
		value.Selection = &selection
	}
	value.Slots = cloneBuilderSlots(value.Slots)
	if value.PreviewGraph != nil {
		graph := cloneExecutionGraphRecord(*value.PreviewGraph)
		value.PreviewGraph = &graph
	}
	value.Diagnostics = append([]CompileDiagnostic{}, value.Diagnostics...)
	return value
}

func ValidateBuilderProjection(value BuilderProjection) error {
	if value.TaxonomyVersion != catalog.TaxonomyVersionV1 || (value.BaseKind != BuilderBaseCanonical && value.BaseKind != BuilderBaseRecipe) || value.Slots == nil || value.Diagnostics == nil || !recordDigestPattern.MatchString(value.HostEvidenceDigest) || !recordDigestPattern.MatchString(value.RegistryDigest) || !recordDigestPattern.MatchString(value.BaseDigest) || !recordDigestPattern.MatchString(value.Digest) || builderProjectionDigest(value) != value.Digest {
		return fmt.Errorf("PROFILE_BUILDER_INVALID")
	}
	if value.Selection != nil && (!recordDigestPattern.MatchString(value.SelectionDigest) || value.Selection.Digest != value.SelectionDigest) {
		return fmt.Errorf("PROFILE_BUILDER_INVALID")
	}
	if value.BaseKind == BuilderBaseCanonical && value.BaseRecipeID != "" || value.BaseKind == BuilderBaseRecipe && value.BaseRecipeID == "" || !reflect.DeepEqual(value.Request, normalizeBuilderRequest(value.Request)) {
		return fmt.Errorf("PROFILE_BUILDER_INVALID")
	}
	if len(value.Slots) != len(catalog.CanonicalSlots()) {
		return fmt.Errorf("PROFILE_BUILDER_INVALID")
	}
	for index, slot := range value.Slots {
		if slot.SlotID != catalog.CanonicalSlots()[index].ID || slot.SelectedPipeline == nil || slot.MacroPreview == nil || slot.Gates == nil || slot.IncidentRoutes == nil || slot.Candidates == nil || slot.Diagnostics == nil {
			return fmt.Errorf("PROFILE_BUILDER_INVALID")
		}
	}
	if value.Selection == nil {
		if value.SelectionDigest != "" || value.PreviewGraph != nil || value.PreviewGraphDigest != "" || value.ConfirmationDigest != "" {
			return fmt.Errorf("PROFILE_BUILDER_INVALID")
		}
		return nil
	}
	if value.PreviewGraph == nil {
		if value.PreviewGraphDigest != "" || value.ConfirmationDigest != "" || len(value.Diagnostics) == 0 {
			return fmt.Errorf("PROFILE_BUILDER_INVALID")
		}
		return nil
	}
	if value.PreviewGraphDigest != value.PreviewGraph.Digest || value.PreviewGraph.Selection.Digest != value.SelectionDigest ||
		value.PreviewGraph.HostEvidenceDigest != value.HostEvidenceDigest || value.PreviewGraph.RegistryDigest != value.RegistryDigest || ValidateExecutionGraphRecord(*value.PreviewGraph) != nil {
		return fmt.Errorf("PROFILE_BUILDER_INVALID")
	}
	if len(value.Diagnostics) == 0 && value.ConfirmationDigest == "" || len(value.Diagnostics) != 0 && value.ConfirmationDigest != "" {
		return fmt.Errorf("PROFILE_BUILDER_INVALID")
	}
	return nil
}

func CloneConfirmedRecipe(value ConfirmedRecipe) ConfirmedRecipe {
	value.Recipe = cloneRecipeRecord(value.Recipe)
	value.Selection = cloneSelection(value.Selection)
	value.ProviderInstances = append([]GraphProviderInstance{}, value.ProviderInstances...)
	value.Graph = cloneExecutionGraphRecord(value.Graph)
	return value
}

func ValidateConfirmedRecipe(value ConfirmedRecipe) error {
	recipeDigest, _, digestErr := canonicaljson.Digest(value.Recipe)
	if digestErr != nil || recipeDigest != value.RecipeDigest || !recordDigestPattern.MatchString(value.RecipeDigest) || !recordDigestPattern.MatchString(value.RegistryDigest) || !recordDigestPattern.MatchString(value.HostEvidenceDigest) || !recordDigestPattern.MatchString(value.ConfirmationDigest) || !recordDigestPattern.MatchString(value.Digest) ||
		value.Selection.RecipeID != value.Recipe.ID || value.Selection.RecipeDigest != value.RecipeDigest || value.Selection.Digest != value.Graph.Selection.Digest ||
		value.Graph.RecipeDigest != value.RecipeDigest || value.Graph.RegistryDigest != value.RegistryDigest || value.Graph.HostEvidenceDigest != value.HostEvidenceDigest ||
		!reflect.DeepEqual(value.ProviderInstances, value.Graph.ProviderInstances) || ValidateExecutionGraphRecord(value.Graph) != nil || confirmedDigest(value) != value.Digest {
		return fmt.Errorf("PROFILE_CONFIRMED_RECIPE_INVALID")
	}
	return nil
}

func normalizeBuilderRequest(value BuilderSelectionRequest) BuilderSelectionRequest {
	value.AddOns = append([]string{}, value.AddOns...)
	sort.Strings(value.AddOns)
	value.Overlays = append([]string{}, value.Overlays...)
	sort.Strings(value.Overlays)
	value.Alternatives = append([]AlternativeChoice{}, value.Alternatives...)
	sort.Slice(value.Alternatives, func(left, right int) bool {
		return string(value.Alternatives[left].SlotID)+"\x00"+value.Alternatives[left].StepID < string(value.Alternatives[right].SlotID)+"\x00"+value.Alternatives[right].StepID
	})
	return value
}

func builderSlots(recipe catalog.ProfileRecipeRecord, descriptors []catalog.ProviderDescriptorRecord, verified EffectiveRegistry, evidence HostEvidenceRecord, topology execution.Topology) ([]BuilderSlot, error) {
	context := newCompilerContext(descriptors, verified, evidence, topology)
	result := make([]BuilderSlot, len(recipe.Slots))
	for index, slot := range recipe.Slots {
		value := BuilderSlot{SlotID: slot.SlotID, SelectedPipeline: clonePipelineSteps(slot.Pipeline), SelectedOwner: slot.OutcomeOwner, MacroPreview: []ResolvedBinding{}, Gates: compileGates(slot.Gates), IncidentRoutes: []CompiledIncidentRoute{}, Candidates: []BuilderCandidate{}, Diagnostics: []CompileDiagnostic{}}
		if len(slot.Pipeline) != 0 {
			value.EntryArtifact = slot.Pipeline[0].RequiredInputArtifact
			value.OutcomeArtifact = slot.Pipeline[len(slot.Pipeline)-1].ProducedOutputArtifact
		}
		if slot.HostAction != nil {
			action := CompiledHostAction{ID: slot.HostAction.ID, InputArtifact: slot.HostAction.InputArtifact, OutputArtifact: slot.HostAction.OutputArtifact}
			value.HostAction = &action
		}
		for _, provider := range descriptors {
			for _, binding := range provider.Bindings {
				if !slotSpanContains(binding.StageSpan, []catalog.SlotID{slot.SlotID}) {
					continue
				}
				candidate := BuilderCandidate{ProviderID: provider.ID, BindingID: binding.ID, Kind: binding.Kind, Topology: topology,
					InputArtifact: binding.InputArtifact, OutputArtifact: binding.OutputArtifact,
					MaximumEffects: append([]string{}, binding.MaximumEffects...), Resources: append([]string{}, binding.Resources...),
					RequiredFeatures: delegationFeatures(binding.Delegation, topology), Compatible: false, Diagnostics: []CompileDiagnostic{}, BindingEvidenceDigest: ""}
				if observed, found := verified.Binding(provider.ID, binding.ID); found {
					candidate.BindingEvidenceDigest = observed.BindingEvidenceDigest
				}
				_, _, diagnostics, resolveErr := context.resolveExactBinding(catalog.BindingSelector{ProviderID: provider.ID, BindingID: binding.ID}, binding.ID, string(slot.SlotID)+"/candidate-"+binding.ID, "", binding.StageSpan, "")
				if resolveErr != nil {
					return nil, resolveErr
				}
				if len(diagnostics) == 0 {
					candidate.Compatible = true
				} else {
					for index := range diagnostics {
						if diagnostics[index].SlotID == "" {
							diagnostics[index].SlotID = slot.SlotID
						}
					}
					candidate.Diagnostics = append(candidate.Diagnostics, diagnostics...)
				}
				value.Candidates = append(value.Candidates, candidate)
			}
		}
		sort.Slice(value.Candidates, func(left, right int) bool {
			leftKey := value.Candidates[left].ProviderID + "\x00" + value.Candidates[left].BindingID + "\x00" + string(value.Candidates[left].Kind)
			rightKey := value.Candidates[right].ProviderID + "\x00" + value.Candidates[right].BindingID + "\x00" + string(value.Candidates[right].Kind)
			return leftKey < rightKey
		})
		result[index] = value
	}
	return result, nil
}

func projectCompiledSlots(slots []BuilderSlot, graph ExecutionGraphRecord) {
	for index := range slots {
		for _, compiled := range graph.Slots {
			if compiled.SlotID != slots[index].SlotID {
				continue
			}
			slots[index].MacroPreview = cloneResolvedBindings(compiled.Pipeline)
			slots[index].HostAction = nil
			if compiled.HostAction != nil {
				action := cloneCompiledHostAction(*compiled.HostAction)
				slots[index].HostAction = &action
			}
			slots[index].Gates = cloneCompiledGates(compiled.Gates)
			if compiled.SlotID == catalog.SlotIncidentRecovery {
				slots[index].IncidentRoutes = make([]CompiledIncidentRoute, len(graph.IncidentRoutes))
				for routeIndex, route := range graph.IncidentRoutes {
					slots[index].IncidentRoutes[routeIndex] = route
					slots[index].IncidentRoutes[routeIndex].HandlerPipeline = append([]execution.GraphCursor{}, route.HandlerPipeline...)
				}
			}
			break
		}
	}
}

func delegationFeatures(requirements catalog.DelegationRequirements, topology execution.Topology) []host.FeatureID {
	features := make([]host.FeatureID, 0, 2)
	if topology == execution.TopologyCurrent {
		if requirements.Child {
			features = append(features, host.FeatureChildDelegation)
		}
		if requirements.ParallelChild {
			features = append(features, host.FeatureParallelChildDelegation)
		}
	} else {
		if requirements.NestedChild {
			features = append(features, host.FeatureNestedChildDelegation)
		}
		if requirements.NestedParallel {
			features = append(features, host.FeatureNestedParallelDelegation)
		}
	}
	return features
}

func cloneRecipeRecord(value catalog.ProfileRecipeRecord) catalog.ProfileRecipeRecord {
	value.Slots = cloneRecipeSlots(value.Slots)
	addOns := value.AddOns
	value.AddOns = make([]catalog.AddOnRecord, len(addOns))
	for index, addOn := range addOns {
		value.AddOns[index] = addOn
		value.AddOns[index].IncidentTypes = append([]string{}, addOn.IncidentTypes...)
		value.AddOns[index].EvidenceRequirements = append([]catalog.EvidenceRequirementRecord{}, addOn.EvidenceRequirements...)
	}
	value.IncidentRoutes = append([]catalog.IncidentRoute{}, value.IncidentRoutes...)
	overlays := value.Overlays
	value.Overlays = make([]catalog.OverlayRecord, len(overlays))
	for index, overlay := range overlays {
		value.Overlays[index] = overlay
		value.Overlays[index].Precedence = append([]string{}, overlay.Precedence...)
		value.Overlays[index].PausedBindings = append([]catalog.BindingSelector{}, overlay.PausedBindings...)
	}
	value.StableBoundaries = append([]string{}, value.StableBoundaries...)
	value.EnvironmentRequirements = cloneRequirements(value.EnvironmentRequirements)
	return value
}

func cloneRecipeSlots(values []catalog.SlotRecipe) []catalog.SlotRecipe {
	result := make([]catalog.SlotRecipe, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Pipeline = clonePipelineSteps(value.Pipeline)
		if value.HostAction != nil {
			action := *value.HostAction
			result[index].HostAction = &action
		}
		result[index].Gates = make([]catalog.GateRecord, len(value.Gates))
		for gateIndex, gate := range value.Gates {
			result[index].Gates[gateIndex] = gate
			result[index].Gates[gateIndex].EvidenceRequirements = append([]catalog.EvidenceRequirementRecord{}, gate.EvidenceRequirements...)
		}
		result[index].Transitions = append([]catalog.RecipeTransition{}, value.Transitions...)
	}
	return result
}

func cloneBuilderSlots(values []BuilderSlot) []BuilderSlot {
	result := make([]BuilderSlot, len(values))
	for index, value := range values {
		result[index] = value
		result[index].SelectedPipeline = clonePipelineSteps(value.SelectedPipeline)
		result[index].MacroPreview = cloneResolvedBindings(value.MacroPreview)
		result[index].Gates = cloneCompiledGates(value.Gates)
		result[index].IncidentRoutes = make([]CompiledIncidentRoute, len(value.IncidentRoutes))
		for routeIndex, route := range value.IncidentRoutes {
			result[index].IncidentRoutes[routeIndex] = route
			result[index].IncidentRoutes[routeIndex].HandlerPipeline = append([]execution.GraphCursor{}, route.HandlerPipeline...)
		}
		if value.HostAction != nil {
			action := cloneCompiledHostAction(*value.HostAction)
			result[index].HostAction = &action
		}
		result[index].Candidates = make([]BuilderCandidate, len(value.Candidates))
		for candidateIndex, candidate := range value.Candidates {
			result[index].Candidates[candidateIndex] = candidate
			result[index].Candidates[candidateIndex].MaximumEffects = append([]string{}, candidate.MaximumEffects...)
			result[index].Candidates[candidateIndex].Resources = append([]string{}, candidate.Resources...)
			result[index].Candidates[candidateIndex].RequiredFeatures = append([]host.FeatureID{}, candidate.RequiredFeatures...)
			result[index].Candidates[candidateIndex].Diagnostics = append([]CompileDiagnostic{}, candidate.Diagnostics...)
		}
		result[index].Diagnostics = append([]CompileDiagnostic{}, value.Diagnostics...)
	}
	return result
}

func cloneCompiledGates(values []CompiledGate) []CompiledGate {
	result := make([]CompiledGate, len(values))
	for index, value := range values {
		result[index] = cloneCompiledGate(value)
	}
	return result
}

func builderProjectionDigest(value BuilderProjection) string {
	value = CloneBuilderProjection(value)
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func confirmationDigest(recipeDigest string, request BuilderSelectionRequest, projection BuilderProjection) string {
	content := CloneBuilderProjection(projection)
	content.ConfirmationDigest = ""
	content.Digest = ""
	value := struct {
		RecipeDigest       string                  `json:"recipe_digest"`
		Request            BuilderSelectionRequest `json:"request"`
		Selection          *Selection              `json:"selection"`
		SelectionDigest    string                  `json:"selection_digest"`
		HostEvidenceDigest string                  `json:"host_evidence_digest"`
		RegistryDigest     string                  `json:"registry_digest"`
		PreviewGraphDigest string                  `json:"preview_graph_digest"`
		Projection         BuilderProjection       `json:"projection"`
	}{recipeDigest, request, projection.Selection, projection.SelectionDigest, projection.HostEvidenceDigest, projection.RegistryDigest, projection.PreviewGraphDigest, content}
	digest, _, _ := canonicaljson.Digest(value)
	return digest
}

func confirmedDigest(value ConfirmedRecipe) string {
	value.Digest = ""
	digest, _, _ := canonicaljson.Digest(value)
	return digest
}

func validBuilderDraftShape(recipe catalog.ProfileRecipeRecord) bool {
	if recipe.SchemaVersion != catalog.ProfileRecipeSchemaV3 || recipe.TaxonomyVersion != catalog.TaxonomyVersionV1 || len(recipe.Slots) != len(catalog.CanonicalSlots()) || recipe.AddOns == nil || recipe.IncidentRoutes == nil || recipe.Overlays == nil || recipe.StableBoundaries == nil || recipe.EnvironmentRequirements == nil {
		return false
	}
	for index, definition := range catalog.CanonicalSlots() {
		slot := recipe.Slots[index]
		if slot.SlotID != definition.ID || slot.Pipeline == nil || slot.Gates == nil || slot.Transitions == nil {
			return false
		}
	}
	return true
}
