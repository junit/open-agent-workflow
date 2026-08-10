package catalog

import (
	"errors"
	"fmt"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func validateProviderReferences(provider *ProviderDescriptorRecord) error {
	distributions := make(map[string]struct{}, len(provider.Distributions))
	for _, distribution := range provider.Distributions {
		distributions[distribution.ID] = struct{}{}
	}
	discoverySurfaces := make(map[string]struct{})
	for _, probe := range provider.Discovery {
		if _, exists := distributions[probe.DistributionID]; !exists {
			return errors.New("PROVIDER_DISTRIBUTION_NOT_FOUND: discovery probe Distribution is missing")
		}
		for _, host := range probe.Hosts {
			discoverySurfaces[host+"\x00"+probe.Surface+"\x00"+probe.DistributionID] = struct{}{}
		}
	}

	bindings := make(map[string]BindingRecord, len(provider.Bindings))
	for _, binding := range provider.Bindings {
		if _, exists := distributions[binding.DistributionID]; !exists {
			return errors.New("PROVIDER_DISTRIBUTION_NOT_FOUND: Binding Distribution is missing")
		}
		if _, exists := discoverySurfaces[binding.Host+"\x00"+binding.Surface+"\x00"+binding.DistributionID]; !exists {
			return errors.New("INVALID_PROVIDER_DESCRIPTOR: Binding has no matching discovery surface")
		}
		bindings[binding.ID] = binding
	}
	for _, binding := range provider.Bindings {
		for _, target := range append(cloneSlice(binding.Alternatives), binding.Conflicts...) {
			if _, exists := bindings[target]; !exists {
				return errors.New("PROVIDER_BINDING_NOT_FOUND: Binding alternative or conflict is missing")
			}
		}
		for _, call := range binding.InternalCalls {
			target, exists := bindings[call.BindingID]
			if !exists {
				return errors.New("PROVIDER_BINDING_NOT_FOUND: internal-call Binding is missing")
			}
			if (call.Mode == InternalDispatchBefore || call.Mode == InternalDispatchAfter) && target.Invocation == InvocationInternal {
				return errors.New("INTERNAL_CALL_NOT_HOST_CALLABLE: dispatching an internal Binding is forbidden")
			}
		}
	}
	for _, capability := range provider.Capabilities {
		for _, bindingID := range capability.BindingRefs {
			if _, exists := bindings[bindingID]; !exists {
				return errors.New("PROVIDER_BINDING_NOT_FOUND: Capability Binding is missing")
			}
		}
	}
	return nil
}

func validateCatalog(catalog *Catalog) error {
	providerIndex := make(map[string]ProviderDescriptorRecord, len(catalog.providers))
	for index := range catalog.providers {
		provider := &catalog.providers[index]
		if err := validateProviderRecord(provider); err != nil {
			return err
		}
		normalizeProvider(provider)
		if _, exists := providerIndex[provider.ID]; exists {
			return errors.New("DUPLICATE_PROVIDER_ID: duplicate provider id")
		}
		providerIndex[provider.ID] = cloneProvider(*provider)
	}

	recipeIndex := make(map[string]ProfileRecipeRecord, len(catalog.recipes))
	for index := range catalog.recipes {
		recipe := &catalog.recipes[index]
		if _, exists := recipeIndex[recipe.ID]; exists {
			return errors.New("DUPLICATE_RECIPE_ID: duplicate recipe id")
		}
		normalized, _, err := normalizeAndDigestRecipe(providerIndex, *recipe)
		if err != nil {
			return err
		}
		*recipe = normalized
		recipeIndex[recipe.ID] = cloneRecipe(*recipe)
	}
	if err := validateAliases(catalog.aliases, recipeIndex); err != nil {
		return err
	}
	return nil
}

func validateAliases(values []ProfileAliasRecord, recipes map[string]ProfileRecipeRecord) error {
	if values == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	for _, alias := range values {
		if _, err := ParseAlias(alias.Alias); err != nil {
			return fmt.Errorf("INVALID_PROFILE_ALIAS_SET: %w", err)
		}
		if _, err := ParseQualifiedID(alias.RecipeID); err != nil {
			return fmt.Errorf("INVALID_PROFILE_ALIAS_SET: %w", err)
		}
		if _, exists := seen[alias.Alias]; exists {
			return errors.New("DUPLICATE_PROFILE_ALIAS: duplicate alias")
		}
		seen[alias.Alias] = struct{}{}
		if recipes != nil {
			if _, exists := recipes[alias.RecipeID]; !exists {
				return errors.New("ALIAS_RECIPE_NOT_FOUND: alias target is not declared")
			}
		}
	}
	return nil
}

func validateRecipeGraph(recipe *ProfileRecipeRecord, providers map[string]ProviderDescriptorRecord) error {
	activeSteps := make(map[string]struct{})
	stepBindings := make(map[string]BindingRecord)
	for slotIndex := range recipe.Slots {
		slot := &recipe.Slots[slotIndex]
		var previousOutput string
		for stepIndex := range slot.Pipeline {
			step := &slot.Pipeline[stepIndex]
			provider, exists := providers[step.Selector.ProviderID]
			if !exists {
				return errors.New("PROVIDER_BINDING_NOT_FOUND: pipeline Provider is missing")
			}
			binding, exists := bindingByID(provider.Bindings, step.Selector.BindingID)
			if !exists {
				return errors.New("PROVIDER_BINDING_NOT_FOUND: pipeline Binding is missing")
			}
			if !spanContains(binding.StageSpan, step.StageSpan) {
				return errors.New("STAGE_SPAN_INVALID: pipeline span is outside Binding span")
			}
			if !containsSlot(step.StageSpan, slot.SlotID) {
				return errors.New("STAGE_SPAN_INVALID: pipeline span does not cover its slot")
			}
			if step.RequiredInputArtifact != binding.InputArtifact || step.ProducedOutputArtifact != binding.OutputArtifact {
				return errors.New("PIPELINE_ARTIFACT_INCOMPATIBLE: step artifacts do not match Binding contract")
			}
			if stepIndex > 0 && previousOutput != step.RequiredInputArtifact {
				return errors.New("PIPELINE_ARTIFACT_INCOMPATIBLE: pipeline edge is incompatible")
			}
			previousOutput = step.ProducedOutputArtifact
			key := step.Selector.ProviderID + "\x00" + step.Selector.BindingID
			activeSteps[key] = struct{}{}
			stepBindings[string(slot.SlotID)+"\x00"+step.ID] = binding
		}
	}

	for slotIndex := range recipe.Slots {
		slot := &recipe.Slots[slotIndex]
		ownerCount := 0
		ownerFound := false
		for stepIndex := range slot.Pipeline {
			step := &slot.Pipeline[stepIndex]
			provider := providers[step.Selector.ProviderID]
			binding := stepBindings[string(slot.SlotID)+"\x00"+step.ID]
			claims, err := expandedOutcomeClaimCount(provider, binding, slot.SlotID, map[string]bool{})
			if err != nil {
				return err
			}
			ownerCount += claims
			if claims == 1 && slot.OutcomeOwner.Kind == OwnerProviderBinding && slot.OutcomeOwner.StepID == step.ID {
				ownerFound = true
			}
			for _, call := range binding.InternalCalls {
				if _, peer := activeSteps[provider.ID+"\x00"+call.BindingID]; peer {
					return errors.New("MACRO_INTERNAL_CONFLICT: internal Binding is also scheduled as a peer")
				}
			}
		}
		if slot.OutcomeOwner.Kind == OwnerProviderBinding {
			if ownerCount > 1 {
				return errors.New("OUTCOME_OWNER_AMBIGUOUS: multiple active Bindings claim the slot outcome")
			}
			if ownerCount == 0 || !ownerFound {
				return errors.New("OUTCOME_OWNER_MISSING: designated Provider step has no matching outcome claim")
			}
		}
	}

	for _, route := range recipe.IncidentRoutes {
		provider, exists := providers[route.Handler.ProviderID]
		if !exists {
			return errors.New("INCIDENT_HANDLER_UNAVAILABLE: incident Provider is missing")
		}
		binding, exists := bindingByID(provider.Bindings, route.Handler.BindingID)
		if !exists || !hasNamespaceClaim(binding.Responsibilities, OwnershipIncident) {
			return errors.New("INCIDENT_HANDLER_UNAVAILABLE: incident handler Binding is unavailable")
		}
	}
	for _, addOn := range recipe.AddOns {
		provider, exists := providers[addOn.Selector.ProviderID]
		if !exists {
			return errors.New("INCIDENT_HANDLER_UNAVAILABLE: Add-on Provider is missing")
		}
		if _, exists := bindingByID(provider.Bindings, addOn.Selector.BindingID); !exists {
			return errors.New("INCIDENT_HANDLER_UNAVAILABLE: Add-on Binding is unavailable")
		}
	}
	for _, overlay := range recipe.Overlays {
		alternativeCount := 0
		for active := range activeSteps {
			providerID, bindingID, ok := splitSelectorKey(active)
			if !ok {
				continue
			}
			binding, exists := bindingByID(providers[providerID].Bindings, bindingID)
			if exists && contains(binding.Alternatives, overlay.SelectedAlternative) {
				alternativeCount++
			}
		}
		if alternativeCount != 1 {
			return errors.New("OVERLAY_INVALID: selected alternative is not declared exactly once")
		}
		for _, paused := range overlay.PausedBindings {
			provider, exists := providers[paused.ProviderID]
			if !exists {
				return errors.New("OVERLAY_INVALID: paused Provider is unavailable")
			}
			if _, exists := bindingByID(provider.Bindings, paused.BindingID); !exists {
				return errors.New("OVERLAY_INVALID: paused Binding is unavailable")
			}
			for _, parent := range provider.Bindings {
				if _, active := activeSteps[paused.ProviderID+"\x00"+parent.ID]; !active {
					continue
				}
				for _, call := range parent.InternalCalls {
					if call.Required && call.BindingID == paused.BindingID {
						return errors.New("OVERLAY_INVALID: mandatory internal call cannot be paused")
					}
				}
			}
		}
	}
	return nil
}

func expandedOutcomeClaimCount(provider ProviderDescriptorRecord, binding BindingRecord, slot SlotID, stack map[string]bool) (int, error) {
	if stack[binding.ID] {
		// Macro expansion owns cycle diagnostics; ownership validation only
		// counts the acyclic prefix available at this Catalog boundary.
		return 0, nil
	}
	stack[binding.ID] = true
	defer delete(stack, binding.ID)

	count := 0
	if hasOutcomeClaim(binding.Responsibilities, slot) {
		count++
	}
	for _, call := range binding.InternalCalls {
		child, exists := bindingByID(provider.Bindings, call.BindingID)
		if !exists {
			return 0, errors.New("PROVIDER_BINDING_NOT_FOUND: internal-call Binding is missing")
		}
		childCount, err := expandedOutcomeClaimCount(provider, child, slot, stack)
		if err != nil {
			return 0, err
		}
		count += childCount
	}
	return count, nil
}

func splitSelectorKey(value string) (string, string, bool) {
	for index := 0; index < len(value); index++ {
		if value[index] == 0 {
			return value[:index], value[index+1:], true
		}
	}
	return "", "", false
}

func bindingByID(bindings []BindingRecord, id string) (BindingRecord, bool) {
	for _, binding := range bindings {
		if binding.ID == id {
			return binding, true
		}
	}
	return BindingRecord{}, false
}

func containsSlot(values []SlotID, wanted SlotID) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func hasOutcomeClaim(values []ResponsibilityClaim, slot SlotID) bool {
	for _, value := range values {
		if value.SlotID == slot && value.OutcomeOwner {
			return true
		}
	}
	return false
}

func hasNamespaceClaim(values []ResponsibilityClaim, namespace OwnershipNamespace) bool {
	for _, value := range values {
		if value.Namespace == namespace {
			return true
		}
	}
	return false
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func normalizeProvider(provider *ProviderDescriptorRecord) {
	sort.Slice(provider.Distributions, func(i, j int) bool { return provider.Distributions[i].ID < provider.Distributions[j].ID })
	sort.Slice(provider.Discovery, func(i, j int) bool { return provider.Discovery[i].ID < provider.Discovery[j].ID })
	for index := range provider.Discovery {
		sort.Strings(provider.Discovery[index].Hosts)
	}
	sort.Slice(provider.Bindings, func(i, j int) bool { return provider.Bindings[i].ID < provider.Bindings[j].ID })
	for index := range provider.Bindings {
		binding := &provider.Bindings[index]
		sort.Strings(binding.MaximumEffects)
		sort.Strings(binding.Resources)
		sort.Strings(binding.Alternatives)
		sort.Strings(binding.Conflicts)
		sort.Slice(binding.Responsibilities, func(i, j int) bool {
			return responsibilityLess(binding.Responsibilities[i], binding.Responsibilities[j])
		})
		for callIndex := range binding.InternalCalls {
			binding.InternalCalls[callIndex].StageSpan = cloneSlice(binding.InternalCalls[callIndex].StageSpan)
		}
	}
	sort.Slice(provider.Capabilities, func(i, j int) bool { return provider.Capabilities[i].ID < provider.Capabilities[j].ID })
	for index := range provider.Capabilities {
		sort.Slice(provider.Capabilities[index].RequestModes, func(i, j int) bool {
			return provider.Capabilities[index].RequestModes[i] < provider.Capabilities[index].RequestModes[j]
		})
		sort.Strings(provider.Capabilities[index].BindingRefs)
	}
}

func normalizeRecipe(recipe *ProfileRecipeRecord) error {
	for index := range recipe.Slots {
		for gateIndex := range recipe.Slots[index].Gates {
			sort.Slice(recipe.Slots[index].Gates[gateIndex].EvidenceRequirements, func(i, j int) bool {
				return recipe.Slots[index].Gates[gateIndex].EvidenceRequirements[i].Kind < recipe.Slots[index].Gates[gateIndex].EvidenceRequirements[j].Kind
			})
		}
	}
	for index := range recipe.AddOns {
		addOn := &recipe.AddOns[index]
		sort.Strings(addOn.IncidentTypes)
		sort.Slice(addOn.EvidenceRequirements, func(i, j int) bool { return addOn.EvidenceRequirements[i].Kind < addOn.EvidenceRequirements[j].Kind })
	}
	sort.Slice(recipe.IncidentRoutes, func(i, j int) bool {
		return recipe.IncidentRoutes[i].IncidentType < recipe.IncidentRoutes[j].IncidentType
	})
	for index := range recipe.Overlays {
		sort.Slice(recipe.Overlays[index].PausedBindings, func(i, j int) bool {
			left := recipe.Overlays[index].PausedBindings[i]
			right := recipe.Overlays[index].PausedBindings[j]
			if left.ProviderID != right.ProviderID {
				return left.ProviderID < right.ProviderID
			}
			return left.BindingID < right.BindingID
		})
	}
	sort.Strings(recipe.StableBoundaries)
	requirements, err := execution.NormalizeRequirements(recipe.EnvironmentRequirements)
	if err != nil {
		return fmt.Errorf("INVALID_PROFILE_RECIPE: %w", err)
	}
	recipe.EnvironmentRequirements = requirements
	return nil
}

func normalizeAliases(values []ProfileAliasRecord) {
	sort.Slice(values, func(i, j int) bool { return values[i].Alias < values[j].Alias })
}

func responsibilityLess(left, right ResponsibilityClaim) bool {
	if left.Namespace != right.Namespace {
		return left.Namespace < right.Namespace
	}
	if left.Name != right.Name {
		return left.Name < right.Name
	}
	if left.SlotID != right.SlotID {
		return left.SlotID < right.SlotID
	}
	return !left.OutcomeOwner && right.OutcomeOwner
}

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	result := make([]T, len(values))
	copy(result, values)
	return result
}
