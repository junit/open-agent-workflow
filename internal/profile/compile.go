package profile

import (
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

func CompileProfile(available CatalogSource, verified EffectiveRegistry, request CompileRequest) (ExecutionGraph, error) {
	recipeID := request.Profile
	for _, alias := range available.Aliases() {
		if alias.Alias == request.Profile {
			recipeID = alias.RecipeID
			break
		}
	}
	for _, recipe := range available.Recipes() {
		if recipe.ID == recipeID {
			return CompileRecipe(available, verified, recipe, request.Bindings)
		}
	}
	return ExecutionGraph{}, compileError("PROFILE_NOT_FOUND", "profile %q is not declared", request.Profile)
}

func CompileRecipe(available CatalogSource, verified EffectiveRegistry, recipe catalog.ProfileRecipeRecord, bindings []ProfileBinding) (ExecutionGraph, error) {
	bindingIndex, normalizedBindings, err := indexBindings(bindings)
	if err != nil {
		return ExecutionGraph{}, err
	}
	if err := validateBindingSelectors(normalizedBindings, recipe.Nodes); err != nil {
		return ExecutionGraph{}, err
	}
	normalizedRecipe := normalizeRecipe(recipe)
	recipeDigest, _, err := canonicaljson.Digest(normalizedRecipe)
	if err != nil {
		return ExecutionGraph{}, err
	}

	descriptors := make(map[string]catalog.ProviderDescriptorRecord)
	for _, descriptor := range available.Providers() {
		descriptors[descriptor.ID] = descriptor
	}

	nodes := make([]GraphNode, 0, len(recipe.Nodes))
	providers := make(map[string]GraphProviderInstance)
	omitted := make(map[string]bool)
	for _, node := range recipe.Nodes {
		providerID := node.Selector.ProviderID
		if binding, found := bindingIndex[selectorKey(node.Selector)]; found {
			providerID = binding.PreferredProviderID
		}
		instance, found := verified.Provider(providerID)
		if !found {
			if node.Optional {
				omitted[node.ID] = true
				continue
			}
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "%s/%s is not verified", providerID, node.Selector.CapabilityID)
		}
		if instance.ProviderID != providerID {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "verified provider identity %s does not match %s", instance.ProviderID, providerID)
		}
		verifiedCapability, found := verified.Capability(providerID, node.Selector.CapabilityID)
		if !found {
			if node.Optional {
				omitted[node.ID] = true
				continue
			}
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "%s/%s is not verified", providerID, node.Selector.CapabilityID)
		}
		if verifiedCapability.ID != node.Selector.CapabilityID {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "verified capability identity %s does not match %s", verifiedCapability.ID, node.Selector.CapabilityID)
		}
		descriptor, found := descriptors[providerID]
		if !found {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "provider descriptor %s is not available", providerID)
		}
		capability, found := descriptorCapability(descriptor, node.Selector.CapabilityID)
		if !found {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "capability descriptor %s/%s is not available", providerID, node.Selector.CapabilityID)
		}
		if !bindingDeclared(capability.HostBindings, verifiedCapability.Binding) {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "verified binding for %s/%s is not declared", providerID, node.Selector.CapabilityID)
		}
		if node.Responsibility != "" && !stringPresent(capability.Responsibilities, node.Responsibility) {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "%s/%s does not own %s", providerID, node.Selector.CapabilityID, node.Responsibility)
		}
		if !requestModePresent(capability.RequestModes, catalog.RequestModeWorkflow) {
			return ExecutionGraph{}, compileError("PROFILE_REQUEST_MODE_UNSUPPORTED", "%s/%s does not allow WORKFLOW", providerID, node.Selector.CapabilityID)
		}
		if err := validateCapabilityLimits(capability); err != nil {
			return ExecutionGraph{}, err
		}
		transitions := make([]GraphTransition, len(node.Transitions))
		for i, transition := range node.Transitions {
			transitions[i] = GraphTransition{Signal: transition.Signal, Target: transition.Target}
		}
		sort.Slice(transitions, func(i, j int) bool { return transitionKey(transitions[i]) < transitionKey(transitions[j]) })
		graphNode := GraphNode{
			ID: node.ID, Kind: node.Kind, Responsibility: node.Responsibility, Phase: node.Phase, Optional: node.Optional,
			ProviderID: providerID, ProviderInstanceDigest: instance.Digest, CapabilityID: capability.ID,
			Binding: verifiedCapability.Binding, InputSchema: capability.InputSchema, OutcomeSchema: capability.OutcomeSchema,
			MaximumEffects: sortedStrings(capability.MaximumEffects), Resources: sortedStrings(capability.Resources),
			RequestModes: sortedRequestModes(capability.RequestModes), ExecutorTopology: capability.ExecutorTopology,
			DelegationAllowList: sortedStrings(capability.DelegationAllowList), Transitions: transitions,
		}
		nodes = append(nodes, graphNode)
		providers[providerID] = GraphProviderInstance{ProviderID: providerID, InstanceDigest: instance.Digest}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	if err := validateOwnership(nodes, recipe.RequiredResponsibilities); err != nil {
		return ExecutionGraph{}, err
	}

	providerInstances := make([]GraphProviderInstance, 0, len(providers))
	for _, provider := range providers {
		providerInstances = append(providerInstances, provider)
	}
	sort.Slice(providerInstances, func(i, j int) bool { return providerInstances[i].ProviderID < providerInstances[j].ProviderID })

	incidentRoutes := make([]GraphIncidentRoute, len(recipe.IncidentRoutes))
	incidentRoutes = incidentRoutes[:0]
	for _, route := range recipe.IncidentRoutes {
		if omitted[route.Handler] {
			continue
		}
		incidentRoutes = append(incidentRoutes, GraphIncidentRoute{Incident: route.Incident, Handler: route.Handler})
	}
	sort.Slice(incidentRoutes, func(i, j int) bool { return incidentRouteKey(incidentRoutes[i]) < incidentRouteKey(incidentRoutes[j]) })

	graph := ExecutionGraph{
		schemaVersion: ExecutionGraphSchemaV1, recipeID: recipe.ID, recipeVersion: recipe.RecipeVersion,
		recipeDigest: recipeDigest, entry: recipe.Entry, bindings: normalizedBindings,
		providerInstances: providerInstances, nodes: nodes, incidentRoutes: incidentRoutes,
		terminalGates: sortedStrings(recipe.TerminalGates), stableBoundaries: sortedStrings(recipe.StableBoundaries),
	}
	if err := validateExecutionGraph(graph, omitted); err != nil {
		return ExecutionGraph{}, err
	}
	digestRecord := executionGraphDigestRecord(graph)
	graph.digest, _, err = canonicaljson.Digest(digestRecord)
	if err != nil {
		return ExecutionGraph{}, err
	}
	return graph, nil
}

func validateBindingSelectors(bindings []ProfileBinding, nodes []catalog.RecipeNode) error {
	selectors := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		selectors[selectorKey(node.Selector)] = struct{}{}
	}
	for _, binding := range bindings {
		if _, found := selectors[selectorKey(binding.Selector)]; !found {
			return compileError("PROFILE_SELECTOR_NOT_FOUND", "selector %s/%s is not declared by the recipe", binding.Selector.ProviderID, binding.Selector.CapabilityID)
		}
	}
	return nil
}

func indexBindings(values []ProfileBinding) (map[string]ProfileBinding, []ProfileBinding, error) {
	index := make(map[string]ProfileBinding, len(values))
	normalized := cloneBindings(values)
	for _, binding := range normalized {
		key := selectorKey(binding.Selector)
		if _, found := index[key]; found {
			return nil, nil, compileError("PROFILE_SELECTOR_AMBIGUOUS", "selector %s/%s has multiple bindings", binding.Selector.ProviderID, binding.Selector.CapabilityID)
		}
		index[key] = binding
	}
	sort.Slice(normalized, func(i, j int) bool {
		return bindingKey(normalized[i]) < bindingKey(normalized[j])
	})
	return index, normalized, nil
}

func selectorKey(value catalog.CapabilitySelector) string {
	return value.ProviderID + "\x00" + value.CapabilityID
}

func bindingKey(value ProfileBinding) string {
	return selectorKey(value.Selector) + "\x00" + value.PreferredProviderID
}

func descriptorCapability(descriptor catalog.ProviderDescriptorRecord, id string) (catalog.CapabilityRecord, bool) {
	for _, capability := range descriptor.Capabilities {
		if capability.ID == id {
			return capability, true
		}
	}
	return catalog.CapabilityRecord{}, false
}

func bindingDeclared(values []catalog.HostBinding, wanted catalog.HostBinding) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func normalizeRecipe(recipe catalog.ProfileRecipeRecord) catalog.ProfileRecipeRecord {
	recipe.RequiredResponsibilities = sortedStrings(recipe.RequiredResponsibilities)
	recipe.IncidentRoutes = append([]catalog.IncidentRoute{}, recipe.IncidentRoutes...)
	sort.Slice(recipe.IncidentRoutes, func(i, j int) bool {
		return recipe.IncidentRoutes[i].Incident+"\x00"+recipe.IncidentRoutes[i].Handler < recipe.IncidentRoutes[j].Incident+"\x00"+recipe.IncidentRoutes[j].Handler
	})
	recipe.TerminalGates = sortedStrings(recipe.TerminalGates)
	recipe.StableBoundaries = sortedStrings(recipe.StableBoundaries)
	recipe.Nodes = append([]catalog.RecipeNode{}, recipe.Nodes...)
	for i := range recipe.Nodes {
		recipe.Nodes[i].Transitions = append([]catalog.RecipeTransition{}, recipe.Nodes[i].Transitions...)
		sort.Slice(recipe.Nodes[i].Transitions, func(left, right int) bool {
			leftTransition := recipe.Nodes[i].Transitions[left]
			rightTransition := recipe.Nodes[i].Transitions[right]
			return leftTransition.Signal+"\x00"+leftTransition.Target < rightTransition.Signal+"\x00"+rightTransition.Target
		})
	}
	sort.Slice(recipe.Nodes, func(i, j int) bool { return recipe.Nodes[i].ID < recipe.Nodes[j].ID })
	return recipe
}

func sortedStrings(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	return result
}

func sortedRequestModes(values []catalog.RequestMode) []catalog.RequestMode {
	result := append([]catalog.RequestMode{}, values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func transitionKey(value GraphTransition) string {
	return value.Signal + "\x00" + value.Target
}

func incidentRouteKey(value GraphIncidentRoute) string {
	return value.Incident + "\x00" + value.Handler
}

func executionGraphDigestRecord(graph ExecutionGraph) any {
	return executionGraphRecordContent(graph.Record())
}
