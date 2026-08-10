package profile

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

type pendingIncidentRoute struct {
	record  catalog.IncidentRoute
	unitIDs []string
}

func CompileProfile(source CatalogSource, verified EffectiveRegistry, request CompileRequest) (CompileResult, error) {
	recipeID := request.Profile
	matches := 0
	for _, alias := range source.Aliases() {
		if alias.Alias == request.Profile {
			recipeID = alias.RecipeID
			matches++
		}
	}
	if matches > 1 {
		return CompileResult{}, fmt.Errorf("PROFILE_TRUSTED_ALIAS_INVALID: duplicate alias %q", request.Profile)
	}
	for _, recipe := range source.Recipes() {
		if recipe.ID == recipeID {
			return CompileRecipe(source, verified, recipe, request)
		}
	}
	return diagnosticCompileResult([]CompileDiagnostic{{Code: "PROFILE_NOT_FOUND", Detail: "requested Profile is not declared"}})
}

func CompileRecipe(source CatalogSource, verified EffectiveRegistry, recipe catalog.ProfileRecipeRecord, request CompileRequest) (CompileResult, error) {
	hostRecord := request.Host.Record()
	if err := ValidateHostEvidenceRecord(hostRecord); err != nil {
		return CompileResult{}, err
	}
	if verified.HostID() != hostRecord.HostID || !recordDigestPattern.MatchString(verified.Digest()) {
		return CompileResult{}, fmt.Errorf("PROFILE_TRUSTED_REGISTRY_INVALID: Host or digest mismatch")
	}
	if request.Topology != hostRecord.Topology {
		return diagnosticCompileResult([]CompileDiagnostic{{Code: "PROFILE_TOPOLOGY_UNAVAILABLE", Topology: request.Topology, Detail: "selection topology differs from Host evidence"}})
	}
	if _, err := execution.NormalizeTopologies([]execution.Topology{request.Topology}); err != nil {
		return diagnosticCompileResult([]CompileDiagnostic{{Code: "PROFILE_TOPOLOGY_UNAVAILABLE", Topology: request.Topology, Detail: "selection topology is invalid"}})
	}

	normalizedRecipe, recipeDigest, err := catalog.NormalizeAndDigestRecipe(source.Providers(), recipe)
	if err != nil {
		return CompileResult{}, fmt.Errorf("PROFILE_TRUSTED_RECIPE_INVALID: %w", err)
	}
	if err := execution.RequirementsSatisfied(normalizedRecipe.EnvironmentRequirements, hostRecord.EnvironmentObservations); err != nil {
		return diagnosticCompileResult([]CompileDiagnostic{{Code: "PROFILE_ENVIRONMENT_UNAVAILABLE", Topology: request.Topology, Detail: "Host environment does not satisfy the Recipe"}})
	}
	selection, diagnostics, err := normalizeSelection(source, normalizedRecipe, recipeDigest, request)
	if err != nil {
		return CompileResult{}, err
	}
	if len(diagnostics) != 0 {
		return diagnosticCompileResult(diagnostics)
	}
	context := newCompilerContext(source.Providers(), verified, hostRecord, request.Topology)
	slots, pendingRoutes, decisions, diagnostics, err := compileSelectedRecipe(context, normalizedRecipe, selection)
	if err != nil {
		return CompileResult{}, err
	}
	if len(diagnostics) != 0 {
		return diagnosticCompileResult(diagnostics)
	}
	assignTraversal(slots)
	routes := materializeIncidentRoutes(slots, pendingRoutes)
	if diagnostics = validateCompiledControlGraph(slots, routes); len(diagnostics) != 0 {
		return diagnosticCompileResult(diagnostics)
	}
	providers, err := graphProviders(verified, slots)
	if err != nil {
		return CompileResult{}, err
	}
	entry := firstActiveSlot(slots)
	if entry == "" {
		return CompileResult{}, fmt.Errorf("PROFILE_COMPILER_INTERNAL: no active slot")
	}
	sortCompileDecisions(decisions)
	graphRecord := ExecutionGraphRecord{
		SchemaVersion: ExecutionGraphSchemaV4, HostID: hostRecord.HostID, HostEvidenceDigest: hostRecord.Digest, RegistryDigest: verified.Digest(),
		TaxonomyVersion: normalizedRecipe.TaxonomyVersion, RecipeID: normalizedRecipe.ID, RecipeVersion: normalizedRecipe.RecipeVersion, RecipeDigest: recipeDigest,
		Selection: selection, ProviderInstances: providers, EntrySlotID: entry, Slots: slots, IncidentRoutes: routes,
		StableBoundaries: append([]string{}, normalizedRecipe.StableBoundaries...), Topology: request.Topology,
		EnvironmentRequirements: cloneRequirements(normalizedRecipe.EnvironmentRequirements), Decisions: decisions,
	}
	graphRecord.Digest = graphRecord.ContentDigest()
	if err := ValidateExecutionGraphRecord(graphRecord); err != nil {
		return CompileResult{}, err
	}
	return graphCompileResult(ExecutionGraph{record: graphRecord})
}

func normalizeSelection(source CatalogSource, recipe catalog.ProfileRecipeRecord, recipeDigest string, request CompileRequest) (Selection, []CompileDiagnostic, error) {
	profileName := request.Profile
	if profileName == "" {
		profileName = recipe.ID
	}
	addOns, valid := sortedUniqueStrings(request.AddOns)
	if !valid {
		return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", Detail: "Add-on selection is invalid or duplicated"}}, nil
	}
	declaredAddOns := make(map[string]struct{}, len(recipe.AddOns))
	for _, addOn := range recipe.AddOns {
		declaredAddOns[addOn.ID] = struct{}{}
	}
	for _, addOn := range addOns {
		if _, found := declaredAddOns[addOn]; !found {
			return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", AddOnID: addOn, Detail: "Add-on is not declared by the Recipe"}}, nil
		}
	}
	overlaySet := make(map[string]struct{}, len(request.Overlays))
	for _, id := range request.Overlays {
		if id == "" {
			return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", Detail: "overlay selection is invalid"}}, nil
		}
		if _, duplicate := overlaySet[id]; duplicate {
			return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", OverlayID: id, Detail: "overlay selection is duplicated"}}, nil
		}
		overlaySet[id] = struct{}{}
	}
	overlays := make([]string, 0, len(overlaySet))
	for _, overlay := range recipe.Overlays {
		if _, selected := overlaySet[overlay.ID]; selected {
			overlays = append(overlays, overlay.ID)
			delete(overlaySet, overlay.ID)
		}
	}
	if len(overlaySet) != 0 {
		for id := range overlaySet {
			return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", OverlayID: id, Detail: "overlay is not declared by the Recipe"}}, nil
		}
	}

	choices := append([]AlternativeChoice{}, request.Alternatives...)
	positions := make(map[string]int)
	for slotIndex, slot := range recipe.Slots {
		for stepIndex, step := range slot.Pipeline {
			positions[string(slot.SlotID)+"\x00"+step.ID] = slotIndex*1000 + stepIndex
		}
	}
	seenChoices := make(map[string]struct{}, len(choices))
	providers := providerIndex(source.Providers())
	for _, choice := range choices {
		key := string(choice.SlotID) + "\x00" + choice.StepID
		if _, duplicate := seenChoices[key]; duplicate {
			return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", SlotID: choice.SlotID, StepID: choice.StepID, Detail: "alternative selection is duplicated"}}, nil
		}
		seenChoices[key] = struct{}{}
		step, found := recipeStep(recipe, choice.SlotID, choice.StepID)
		if !found || choice.AlternativeID != choice.Selector.BindingID || choice.Selector.ProviderID != step.Selector.ProviderID {
			return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", SlotID: choice.SlotID, StepID: choice.StepID, AlternativeID: choice.AlternativeID, Detail: "alternative identity does not match the declared step"}}, nil
		}
		provider, found := providers[step.Selector.ProviderID]
		if !found {
			return Selection{}, nil, fmt.Errorf("PROFILE_TRUSTED_RECIPE_INVALID: Provider is unavailable")
		}
		binding, found := descriptorBinding(provider, step.Selector.BindingID)
		if !found || !slices.Contains(binding.Alternatives, choice.AlternativeID) {
			return Selection{}, []CompileDiagnostic{{Code: "PROFILE_SELECTION_INVALID", SlotID: choice.SlotID, StepID: choice.StepID, AlternativeID: choice.AlternativeID, Detail: "alternative is not declared by the selected Binding"}}, nil
		}
	}
	sort.Slice(choices, func(left, right int) bool {
		leftKey := string(choices[left].SlotID) + "\x00" + choices[left].StepID
		rightKey := string(choices[right].SlotID) + "\x00" + choices[right].StepID
		return positions[leftKey] < positions[rightKey]
	})
	selection := Selection{
		Profile: profileName, RecipeID: recipe.ID, RecipeDigest: recipeDigest, Topology: request.Topology,
		AddOns: addOns, Alternatives: choices, Overlays: overlays,
	}
	selection.Digest = selectionContentDigest(selection)
	return selection, nil, nil
}

func compileSelectedRecipe(context compilerContext, recipe catalog.ProfileRecipeRecord, selection Selection) ([]CompiledSlot, []pendingIncidentRoute, []CompileDecision, []CompileDiagnostic, error) {
	selectedAddOns := make(map[string]struct{}, len(selection.AddOns))
	for _, id := range selection.AddOns {
		selectedAddOns[id] = struct{}{}
	}
	choices := make(map[string]AlternativeChoice, len(selection.Alternatives))
	for _, choice := range selection.Alternatives {
		choices[string(choice.SlotID)+"\x00"+choice.StepID] = choice
	}
	selectedOverlays := make(map[string]struct{}, len(selection.Overlays))
	for _, id := range selection.Overlays {
		selectedOverlays[id] = struct{}{}
	}
	decisions := []CompileDecision{}
	diagnostics := []CompileDiagnostic{}
	context.decisions = &decisions
	for _, overlay := range recipe.Overlays {
		if _, selected := selectedOverlays[overlay.ID]; !selected {
			continue
		}
		type overlayMatch struct {
			slot catalog.SlotID
			step catalog.PipelineStep
		}
		matches := []overlayMatch{}
		for _, slot := range recipe.Slots {
			for _, step := range slot.Pipeline {
				provider := context.descriptors[step.Selector.ProviderID]
				binding, found := descriptorBinding(provider, step.Selector.BindingID)
				if found && slices.Contains(binding.Alternatives, overlay.SelectedAlternative) {
					matches = append(matches, overlayMatch{slot: slot.SlotID, step: step})
				}
			}
		}
		if len(matches) != 1 {
			diagnostics = append(diagnostics, CompileDiagnostic{Code: "OVERLAY_INVALID", OverlayID: overlay.ID, Detail: "overlay alternative does not identify exactly one declared step"})
			continue
		}
		match := matches[0]
		key := string(match.slot) + "\x00" + match.step.ID
		choice := AlternativeChoice{SlotID: match.slot, StepID: match.step.ID, AlternativeID: overlay.SelectedAlternative, Selector: catalog.BindingSelector{ProviderID: match.step.Selector.ProviderID, BindingID: overlay.SelectedAlternative}}
		if explicit, found := choices[key]; found && explicit.Selector != choice.Selector {
			diagnostics = append(diagnostics, CompileDiagnostic{Code: "PROFILE_SELECTION_INVALID", SlotID: match.slot, StepID: match.step.ID, OverlayID: overlay.ID, Detail: "overlay alternative conflicts with the explicit selection"})
			continue
		}
		choices[key] = choice
		decisions = append(decisions, CompileDecision{SlotID: match.slot, StepID: match.step.ID, AlternativeID: overlay.SelectedAlternative, OverlayID: overlay.ID, Disposition: DispatchByCoordinator, ReasonCode: "OVERLAY_ALTERNATIVE_SELECTED", Detail: "overlay selected the declared alternative"})
	}

	// Expand every selected mainline and Add-on step before materialization. A
	// multi-slot unit is retained once at its declared anchor; later Recipe
	// references credit the same Unit ID instead of copying the Binding.
	slots := make([]CompiledSlot, len(recipe.Slots))
	pipelineByAnchor := make(map[catalog.SlotID][]ResolvedBinding, len(recipe.Slots))
	stepUnits := make(map[string]string)
	seenUnits := make(map[string]struct{})
	multiUnits := make(map[string][]ResolvedBinding)
	stepsBySlot := make(map[catalog.SlotID][]catalog.PipelineStep, len(recipe.Slots))
	appendUnits := func(values []ResolvedBinding) {
		for _, unit := range values {
			if _, duplicate := seenUnits[unit.UnitID]; duplicate {
				continue
			}
			seenUnits[unit.UnitID] = struct{}{}
			pipelineByAnchor[unit.AnchorSlotID] = append(pipelineByAnchor[unit.AnchorSlotID], cloneResolvedBinding(unit))
		}
	}
	for _, slotRecipe := range recipe.Slots {
		steps := append([]catalog.PipelineStep{}, slotRecipe.Pipeline...)
		for stepIndex := range steps {
			if choice, found := choices[string(slotRecipe.SlotID)+"\x00"+steps[stepIndex].ID]; found {
				steps[stepIndex].Selector = choice.Selector
				decisions = append(decisions, CompileDecision{SlotID: slotRecipe.SlotID, StepID: steps[stepIndex].ID, AlternativeID: choice.AlternativeID, Disposition: DispatchByCoordinator, ReasonCode: "ALTERNATIVE_SELECTED", Detail: "explicit alternative selected"})
			}
		}
		for _, addOn := range recipe.AddOns {
			if addOn.SlotID != slotRecipe.SlotID {
				continue
			}
			if _, selected := selectedAddOns[addOn.ID]; !selected {
				decisions = append(decisions, CompileDecision{SlotID: slotRecipe.SlotID, AddOnID: addOn.ID, Disposition: OmittedBySelection, ReasonCode: "ADD_ON_NOT_SELECTED", Detail: "optional Add-on omitted"})
				continue
			}
			if addOn.Kind == catalog.AddOnIncidentHandler {
				decisions = append(decisions, CompileDecision{SlotID: slotRecipe.SlotID, AddOnID: addOn.ID, Disposition: DispatchByCoordinator, ReasonCode: "ADD_ON_SELECTED", Detail: "explicit incident-handler Add-on selected"})
				continue
			}
			provider := context.descriptors[addOn.Selector.ProviderID]
			binding, found := descriptorBinding(provider, addOn.Selector.BindingID)
			if !found {
				diagnostics = append(diagnostics, CompileDiagnostic{Code: "PROFILE_ADD_ON_INVALID", SlotID: slotRecipe.SlotID, AddOnID: addOn.ID, ProviderID: addOn.Selector.ProviderID, BindingID: addOn.Selector.BindingID, Detail: "selected Add-on Binding is unavailable"})
				continue
			}
			steps = append(steps, catalog.PipelineStep{
				ID: "add-on-" + addOn.ID, Selector: addOn.Selector, StageSpan: append([]catalog.SlotID{}, binding.StageSpan...),
				RequiredInputArtifact: binding.InputArtifact, ProducedOutputArtifact: binding.OutputArtifact,
			})
			decisions = append(decisions, CompileDecision{SlotID: slotRecipe.SlotID, AddOnID: addOn.ID, Disposition: DispatchByCoordinator, ReasonCode: "ADD_ON_SELECTED", Detail: "explicit Add-on selected"})
		}
		stepsBySlot[slotRecipe.SlotID] = clonePipelineSteps(steps)
		for _, step := range steps {
			stepKey := string(slotRecipe.SlotID) + "\x00" + step.ID
			multiKey := selectorIdentity(step.Selector) + "\x00" + stageSpanIdentity(step.StageSpan)
			if len(step.StageSpan) > 1 {
				if prior, found := multiUnits[multiKey]; found {
					if ownerUnitID, found := outcomeUnitID(prior, slotRecipe.SlotID); found {
						stepUnits[stepKey] = ownerUnitID
					}
					for _, unit := range prior {
						decisions = append(decisions, CompileDecision{SlotID: slotRecipe.SlotID, StepID: step.ID, UnitID: unit.UnitID, Disposition: CreditInternalOnly, ReasonCode: "MULTI_SLOT_CREDIT", Detail: "multi-slot Binding is anchored in an earlier slot"})
					}
					continue
				}
			}
			units, values, err := context.resolveStep(slotRecipe.SlotID, step)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			for index := range values {
				if values[index].SlotID == "" {
					values[index].SlotID = slotRecipe.SlotID
				}
			}
			diagnostics = append(diagnostics, values...)
			if len(units) == 0 {
				continue
			}
			for _, unit := range units {
				if unit.Disposition == CreditInternalOnly {
					decisions = append(decisions, CompileDecision{SlotID: unit.AnchorSlotID, StepID: unit.StepID, UnitID: unit.UnitID, Disposition: CreditInternalOnly, ReasonCode: "MACRO_INTERNAL_CREDIT", Detail: "internal Binding is credited without Coordinator dispatch"})
				}
			}
			if ownerUnitID, found := outcomeUnitID(units, slotRecipe.SlotID); found {
				stepUnits[stepKey] = ownerUnitID
			}
			if len(step.StageSpan) > 1 {
				multiUnits[multiKey] = cloneResolvedBindings(units)
				for _, unit := range units {
					for _, creditedSlot := range unit.SlotIDs {
						if creditedSlot != unit.AnchorSlotID {
							decisions = append(decisions, CompileDecision{SlotID: creditedSlot, StepID: step.ID, UnitID: unit.UnitID, Disposition: CreditInternalOnly, ReasonCode: "MULTI_SLOT_CREDIT", Detail: "multi-slot Binding is credited at this slot"})
						}
					}
				}
			}
			appendUnits(units)
		}
	}

	// Apply overlay pauses only after all macros have been expanded. The same
	// anchored unit can therefore be paused from any credited slot.
	for _, overlay := range recipe.Overlays {
		if _, selected := selectedOverlays[overlay.ID]; !selected {
			continue
		}
		for anchor, pipeline := range pipelineByAnchor {
			for unitIndex := range pipeline {
				selector := catalog.BindingSelector{ProviderID: pipeline[unitIndex].ProviderID, BindingID: pipeline[unitIndex].BindingID}
				if slices.ContainsFunc(overlay.PausedBindings, func(value catalog.BindingSelector) bool { return value == selector }) {
					pipeline[unitIndex].Disposition = OmittedBySelection
					decisions = append(decisions, CompileDecision{SlotID: anchor, StepID: pipeline[unitIndex].StepID, UnitID: pipeline[unitIndex].UnitID, OverlayID: overlay.ID, Disposition: OmittedBySelection, ReasonCode: "OVERLAY_PAUSED_BINDING", Detail: "overlay paused the Binding"})
				}
			}
			pipelineByAnchor[anchor] = pipeline
		}
	}

	pendingRoutes := []pendingIncidentRoute{}
	for _, route := range recipe.IncidentRoutes {
		incidentAddOn, gatedByAddOn := incidentAddOnForRoute(recipe.AddOns, route)
		if gatedByAddOn {
			if _, selected := selectedAddOns[incidentAddOn.ID]; !selected {
				pendingRoutes = append(pendingRoutes, pendingIncidentRoute{record: route})
				decisions = append(decisions, CompileDecision{SlotID: catalog.SlotIncidentRecovery, AddOnID: incidentAddOn.ID, IncidentType: route.IncidentType, Disposition: OmittedBySelection, ReasonCode: "INCIDENT_ADD_ON_NOT_SELECTED", Detail: "incident-handler Add-on was not selected"})
				continue
			}
		}
		provider := context.descriptors[route.Handler.ProviderID]
		binding, found := descriptorBinding(provider, route.Handler.BindingID)
		if !found {
			pendingRoutes = append(pendingRoutes, pendingIncidentRoute{record: route})
			decisions = append(decisions, CompileDecision{SlotID: catalog.SlotIncidentRecovery, IncidentType: route.IncidentType, Disposition: OmittedBySelection, ReasonCode: "INCIDENT_HANDLER_UNAVAILABLE", Detail: "incident handler Binding is unavailable"})
			continue
		}
		step := catalog.PipelineStep{ID: "incident-" + sanitizeUnitID(route.IncidentType), Selector: route.Handler, StageSpan: append([]catalog.SlotID{}, binding.StageSpan...), RequiredInputArtifact: binding.InputArtifact, ProducedOutputArtifact: binding.OutputArtifact}
		units, values, err := context.resolveStep(catalog.SlotIncidentRecovery, step)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if len(values) != 0 {
			for index := range values {
				values[index].IncidentType = route.IncidentType
			}
			if gatedByAddOn {
				for index := range values {
					values[index].AddOnID = incidentAddOn.ID
				}
				diagnostics = append(diagnostics, values...)
			} else if route.IfUnavailable != catalog.IncidentStop && route.IfUnavailable != catalog.IncidentReplan {
				diagnostics = append(diagnostics, values...)
			}
			detail := "incident handler Binding is unavailable"
			if values[0].Code != "" {
				detail = values[0].Code + ": " + values[0].Detail
			}
			decisions = append(decisions, CompileDecision{SlotID: catalog.SlotIncidentRecovery, IncidentType: route.IncidentType, Disposition: OmittedBySelection, ReasonCode: "INCIDENT_HANDLER_UNAVAILABLE", Detail: detail})
			pendingRoutes = append(pendingRoutes, pendingIncidentRoute{record: route})
			continue
		}
		unitIDs := make([]string, 0, len(units))
		for _, unit := range units {
			unitIDs = append(unitIDs, unit.UnitID)
		}
		appendUnits(units)
		pendingRoutes = append(pendingRoutes, pendingIncidentRoute{record: route, unitIDs: unitIDs})
	}

	for slotIndex, slotRecipe := range recipe.Slots {
		steps := stepsBySlot[slotRecipe.SlotID]
		pipeline := cloneResolvedBindings(pipelineByAnchor[slotRecipe.SlotID])
		compiled := CompiledSlot{SlotID: slotRecipe.SlotID, Applicability: slotRecipe.Applicability,
			Active:   slotRecipe.Applicability == catalog.SlotMandatory || len(pipeline) != 0 || slotRecipe.HostAction != nil,
			Pipeline: pipeline, Gates: compileGates(slotRecipe.Gates), Transitions: compileTransitions(slotRecipe.Transitions)}
		if len(steps) != 0 {
			compiled.EntryArtifact = steps[0].RequiredInputArtifact
			compiled.OutcomeArtifact = steps[len(steps)-1].ProducedOutputArtifact
			for stepIndex := 1; stepIndex < len(steps); stepIndex++ {
				if steps[stepIndex-1].ProducedOutputArtifact != steps[stepIndex].RequiredInputArtifact {
					diagnostics = append(diagnostics, CompileDiagnostic{Code: "PIPELINE_ARTIFACT_INCOMPATIBLE", SlotID: slotRecipe.SlotID, StepID: steps[stepIndex].ID, Detail: "pipeline edge is incompatible"})
				}
			}
		}
		if len(pipeline) != 0 {
			compiled.EntryArtifact = pipeline[0].InputArtifact
			compiled.OutcomeArtifact = pipeline[len(pipeline)-1].OutputArtifact
		}
		if slotRecipe.HostAction != nil {
			action, values := compileHostAction(context.host, *slotRecipe.HostAction, slotRecipe.SlotID, context.topology)
			diagnostics = append(diagnostics, values...)
			if len(values) == 0 {
				compiled.HostAction = &action
				compiled.EntryArtifact = slotRecipe.HostAction.InputArtifact
				compiled.OutcomeArtifact = slotRecipe.HostAction.OutputArtifact
			}
		}
		if !compiled.Active {
			decisions = append(decisions, CompileDecision{SlotID: slotRecipe.SlotID, Disposition: OmittedBySelection, ReasonCode: "CONDITIONAL_SLOT_INACTIVE", Detail: "conditional slot has no selected or available pipeline"})
		}
		slots[slotIndex] = compiled
	}
	for slotIndex, slotRecipe := range recipe.Slots {
		owner, values := compileOwner(slotRecipe, slots[slotIndex], stepUnits, pipelineByAnchor)
		if !hasSlotDiagnostic(diagnostics, slotRecipe.SlotID) {
			diagnostics = append(diagnostics, values...)
		}
		slots[slotIndex].OutcomeOwner = owner
		slots[slotIndex].Terminal = slots[slotIndex].Active && slots[slotIndex].SlotID != catalog.SlotIncidentRecovery && len(slots[slotIndex].Transitions) == 0
	}
	return slots, pendingRoutes, decisions, diagnostics, nil
}

func outcomeUnitID(units []ResolvedBinding, slotID catalog.SlotID) (string, bool) {
	result := ""
	for _, unit := range units {
		if unit.Disposition == OmittedBySelection || !slices.ContainsFunc(unit.Responsibilities, func(claim catalog.ResponsibilityClaim) bool {
			return claim.SlotID == slotID && claim.OutcomeOwner
		}) {
			continue
		}
		if result != "" {
			return "", false
		}
		result = unit.UnitID
	}
	return result, result != ""
}

func incidentAddOnForRoute(addOns []catalog.AddOnRecord, route catalog.IncidentRoute) (catalog.AddOnRecord, bool) {
	for _, addOn := range addOns {
		if addOn.Kind == catalog.AddOnIncidentHandler && addOn.Selector == route.Handler && slices.Contains(addOn.IncidentTypes, route.IncidentType) {
			return addOn, true
		}
	}
	return catalog.AddOnRecord{}, false
}

func hasSlotDiagnostic(values []CompileDiagnostic, slotID catalog.SlotID) bool {
	for _, value := range values {
		if value.SlotID == slotID {
			return true
		}
	}
	return false
}

func stageSpanIdentity(values []catalog.SlotID) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = string(value)
	}
	return strings.Join(parts, ",")
}

func compileHostAction(evidence HostEvidenceRecord, reference catalog.HostActionRef, slotID catalog.SlotID, topology execution.Topology) (CompiledHostAction, []CompileDiagnostic) {
	observation, found := hostAction(evidence, reference.ID)
	if !found || observation.State != host.AvailabilityAvailable || !liveSource(observation.Source) {
		return CompiledHostAction{}, []CompileDiagnostic{{Code: "HOST_ACTION_UNATTESTED", SlotID: slotID, Topology: topology, Detail: "required Host action is not live and available"}}
	}
	return CompiledHostAction{
		ID: reference.ID, InputArtifact: reference.InputArtifact, OutputArtifact: reference.OutputArtifact,
		InputSchema: observation.Action.InputSchema, OutcomeSchema: observation.Action.OutcomeSchema,
		MaximumEffects: append([]string{}, observation.Action.MaximumEffects...), Resources: append([]string{}, observation.Action.Resources...),
		ObservationDigest: observation.Digest,
	}, nil
}

func compileGates(values []catalog.GateRecord) []CompiledGate {
	result := make([]CompiledGate, len(values))
	for index, gate := range values {
		result[index] = CompiledGate{ID: gate.ID, Authority: gate.Authority, Predicate: gate.Predicate, EvidenceRequirements: append([]catalog.EvidenceRequirementRecord{}, gate.EvidenceRequirements...)}
	}
	return result
}

func compileTransitions(values []catalog.RecipeTransition) []GraphTransition {
	result := make([]GraphTransition, len(values))
	for index, value := range values {
		result[index] = GraphTransition{Signal: value.Signal, Target: value.Target}
	}
	return result
}

func compileOwner(recipe catalog.SlotRecipe, slot CompiledSlot, stepUnits map[string]string, pipelines map[catalog.SlotID][]ResolvedBinding) (CompiledOwner, []CompileDiagnostic) {
	switch recipe.OutcomeOwner.Kind {
	case catalog.OwnerNone:
		return CompiledOwner{Kind: catalog.OwnerNone}, nil
	case catalog.OwnerHostAction:
		if slot.HostAction == nil || slot.HostAction.ID != recipe.OutcomeOwner.HostAction {
			return CompiledOwner{}, []CompileDiagnostic{{Code: "OUTCOME_OWNER_MISSING", SlotID: recipe.SlotID, Detail: "Host action owner is unavailable"}}
		}
		return CompiledOwner{Kind: catalog.OwnerHostAction, UnitID: slot.HostAction.ID, HostActionID: slot.HostAction.ID}, nil
	case catalog.OwnerProviderBinding:
		unitID := stepUnits[string(recipe.SlotID)+"\x00"+recipe.OutcomeOwner.StepID]
		matches := []ResolvedBinding{}
		for _, pipeline := range pipelines {
			for _, unit := range pipeline {
				if unit.UnitID == unitID && unit.Disposition != OmittedBySelection {
					matches = append(matches, unit)
				}
			}
		}
		if len(matches) != 1 {
			return CompiledOwner{}, []CompileDiagnostic{{Code: "OUTCOME_OWNER_MISSING", SlotID: recipe.SlotID, StepID: recipe.OutcomeOwner.StepID, Detail: "exactly one active Provider outcome owner is required"}}
		}
		owner := matches[0]
		return CompiledOwner{Kind: catalog.OwnerProviderBinding, UnitID: owner.UnitID, ProviderID: owner.ProviderID, BindingID: owner.BindingID}, nil
	default:
		return CompiledOwner{}, []CompileDiagnostic{{Code: "OUTCOME_OWNER_MISSING", SlotID: recipe.SlotID, Detail: "outcome owner kind is invalid"}}
	}
}

func graphProviders(verified EffectiveRegistry, slots []CompiledSlot) ([]GraphProviderInstance, error) {
	used := make(map[string]string)
	for _, slot := range slots {
		for _, unit := range slot.Pipeline {
			if current, found := used[unit.ProviderID]; found && current != unit.ProviderInstanceDigest {
				return nil, fmt.Errorf("PROFILE_TRUSTED_REGISTRY_INVALID: Provider Instance digest changed")
			}
			provider, found := verified.Provider(unit.ProviderID)
			if !found || provider.Digest != unit.ProviderInstanceDigest {
				return nil, fmt.Errorf("PROFILE_TRUSTED_REGISTRY_INVALID: Provider Instance is unavailable")
			}
			used[unit.ProviderID] = unit.ProviderInstanceDigest
		}
	}
	result := make([]GraphProviderInstance, 0, len(used))
	for providerID, digest := range used {
		result = append(result, GraphProviderInstance{ProviderID: providerID, HostID: verified.HostID(), InstanceDigest: digest})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ProviderID < result[right].ProviderID })
	return result, nil
}

func diagnosticCompileResult(values []CompileDiagnostic) (CompileResult, error) {
	diagnostics := append([]CompileDiagnostic{}, values...)
	sortCompileDiagnostics(diagnostics)
	digest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string              `json:"schema_version"`
		Diagnostics   []CompileDiagnostic `json:"diagnostics"`
	}{"oaw.profile-compile-result/v1", diagnostics})
	if err != nil {
		return CompileResult{}, err
	}
	return CompileResult{diagnostics: diagnostics, digest: digest}, nil
}

func graphCompileResult(graph ExecutionGraph) (CompileResult, error) {
	record := graph.Record()
	digest, _, err := canonicaljson.Digest(struct {
		SchemaVersion string               `json:"schema_version"`
		Graph         ExecutionGraphRecord `json:"graph"`
	}{"oaw.profile-compile-result/v1", record})
	if err != nil {
		return CompileResult{}, err
	}
	copy := ExecutionGraph{record: record}
	return CompileResult{graph: &copy, diagnostics: []CompileDiagnostic{}, digest: digest}, nil
}

func selectionContentDigest(value Selection) string {
	value = cloneSelection(value)
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return ""
	}
	return digest
}

func sortCompileDiagnostics(values []CompileDiagnostic) {
	sort.SliceStable(values, func(left, right int) bool { return diagnosticKey(values[left]) < diagnosticKey(values[right]) })
}

func diagnosticKey(value CompileDiagnostic) string {
	return strings.Join([]string{value.Code, string(value.SlotID), value.StepID, value.ProviderID, value.BindingID, value.AddOnID, value.AlternativeID, value.OverlayID, value.IncidentType, string(value.Topology), value.Detail}, "\x00")
}

func sortCompileDecisions(values []CompileDecision) {
	positions := make(map[catalog.SlotID]int)
	for index, slot := range catalog.CanonicalSlots() {
		positions[slot.ID] = index
	}
	sort.SliceStable(values, func(left, right int) bool {
		leftKey := fmt.Sprintf("%02d\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", positions[values[left].SlotID], values[left].StepID, values[left].UnitID, values[left].AddOnID, values[left].AlternativeID, values[left].OverlayID, values[left].IncidentType, values[left].Disposition, values[left].ReasonCode, values[left].Detail)
		rightKey := fmt.Sprintf("%02d\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s", positions[values[right].SlotID], values[right].StepID, values[right].UnitID, values[right].AddOnID, values[right].AlternativeID, values[right].OverlayID, values[right].IncidentType, values[right].Disposition, values[right].ReasonCode, values[right].Detail)
		return leftKey < rightKey
	})
}

func providerIndex(values []catalog.ProviderDescriptorRecord) map[string]catalog.ProviderDescriptorRecord {
	result := make(map[string]catalog.ProviderDescriptorRecord, len(values))
	for _, provider := range values {
		result[provider.ID] = provider
	}
	return result
}

func recipeStep(recipe catalog.ProfileRecipeRecord, slotID catalog.SlotID, stepID string) (catalog.PipelineStep, bool) {
	for _, slot := range recipe.Slots {
		if slot.SlotID != slotID {
			continue
		}
		for _, step := range slot.Pipeline {
			if step.ID == stepID {
				return step, true
			}
		}
	}
	return catalog.PipelineStep{}, false
}

func firstActiveSlot(slots []CompiledSlot) catalog.SlotID {
	for _, slot := range slots {
		if slot.Active {
			return slot.SlotID
		}
	}
	return ""
}

func sanitizeUnitID(value string) string {
	value = strings.Map(func(char rune) rune {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' {
			return char
		}
		return '-'
	}, value)
	return strings.Trim(value, "-")
}
