package profile

import (
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
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
			return CompileRecipe(available, verified, recipe, request)
		}
	}
	return ExecutionGraph{}, compileError("PROFILE_NOT_FOUND", "profile %q is not declared", request.Profile)
}

func CompileRecipe(available CatalogSource, verified EffectiveRegistry, recipe catalog.ProfileRecipeRecord, request CompileRequest) (ExecutionGraph, error) {
	hostID := verified.HostID()
	if _, err := catalog.ParseLocalID(hostID); err != nil {
		return ExecutionGraph{}, compileError("HOST_PROVIDER_SCOPE_MISMATCH", "verified Registry has invalid Host %q", hostID)
	}
	hostTopologies, err := execution.NormalizeTopologies(request.HostTopologies)
	if err != nil {
		return ExecutionGraph{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%v", err)
	}
	requirements, err := execution.NormalizeRequirements(recipe.EnvironmentRequirements)
	if err != nil {
		return ExecutionGraph{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%v", err)
	}
	if err := execution.RequirementsSatisfied(requirements, request.EnvironmentObservations); err != nil {
		return ExecutionGraph{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%v", err)
	}
	addOns, err := indexAddOns(request.AddOns, recipe.Nodes)
	if err != nil {
		return ExecutionGraph{}, err
	}
	bindingIndex, normalizedBindings, err := indexBindings(request.Bindings)
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
	eligibleTopologies := append([]execution.Topology{}, hostTopologies...)
	for _, node := range recipe.Nodes {
		selectedAddOn := addOns[node.ID]
		if node.Optional && !selectedAddOn {
			omitted[node.ID] = true
			continue
		}
		providerID := node.Selector.ProviderID
		if binding, found := bindingIndex[selectorKey(node.Selector)]; found {
			providerID = binding.PreferredProviderID
		}
		instance, found := verified.Provider(providerID)
		if !found {
			if selectedAddOn {
				return ExecutionGraph{}, compileError("PROFILE_ADD_ON_INVALID", "add-on %q Provider %s is not verified", node.ID, providerID)
			}
			return ExecutionGraph{}, compileCapabilityError(providerID, node.Selector.CapabilityID, "%s/%s is not verified", providerID, node.Selector.CapabilityID)
		}
		if instance.ProviderID != providerID {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "verified provider identity %s does not match %s", instance.ProviderID, providerID)
		}
		if instance.HostID != hostID {
			return ExecutionGraph{}, compileError("HOST_PROVIDER_SCOPE_MISMATCH", "verified Provider %s belongs to Host %q, not %q", providerID, instance.HostID, hostID)
		}
		verifiedCapability, found := verified.Capability(providerID, node.Selector.CapabilityID)
		if !found {
			if selectedAddOn {
				return ExecutionGraph{}, compileError("PROFILE_ADD_ON_INVALID", "add-on %q Capability %s/%s is not verified", node.ID, providerID, node.Selector.CapabilityID)
			}
			return ExecutionGraph{}, compileCapabilityError(providerID, node.Selector.CapabilityID, "%s/%s is not verified", providerID, node.Selector.CapabilityID)
		}
		if verifiedCapability.ID != node.Selector.CapabilityID {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "verified capability identity %s does not match %s", verifiedCapability.ID, node.Selector.CapabilityID)
		}
		if verifiedCapability.Binding.Host != hostID {
			return ExecutionGraph{}, compileError("HOST_PROVIDER_SCOPE_MISMATCH", "verified Binding for %s/%s belongs to Host %q, not %q", providerID, node.Selector.CapabilityID, verifiedCapability.Binding.Host, hostID)
		}
		descriptor, found := descriptors[providerID]
		if !found {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "provider descriptor %s is not available", providerID)
		}
		capability, found := descriptorCapability(descriptor, node.Selector.CapabilityID)
		if !found {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "capability descriptor %s/%s is not available", providerID, node.Selector.CapabilityID)
		}
		declaredBinding, found := declaredBinding(capability.HostBindings, verifiedCapability.Binding)
		if !found {
			return ExecutionGraph{}, compileError("PROFILE_CAPABILITY_MISSING", "verified binding for %s/%s is not declared", providerID, node.Selector.CapabilityID)
		}
		nodeTopologies, pinnedBinding, err := compileNodeTopologies(capability, declaredBinding, verifiedCapability)
		if err != nil {
			return ExecutionGraph{}, err
		}
		eligibleTopologies, err = execution.IntersectTopologies(eligibleTopologies, nodeTopologies)
		if err != nil || len(eligibleTopologies) == 0 {
			return ExecutionGraph{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "node %q leaves no eligible topology", node.ID)
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
			Binding: pinnedBinding, InputSchema: capability.InputSchema, OutcomeSchema: capability.OutcomeSchema,
			MaximumEffects: sortedStrings(capability.MaximumEffects), Resources: sortedStrings(capability.Resources),
			RequestModes: sortedRequestModes(capability.RequestModes), SupportedTopologies: nodeTopologies,
			DelegationAllowList: sortedStrings(capability.DelegationAllowList), Transitions: transitions,
		}
		nodes = append(nodes, graphNode)
		providers[providerID] = GraphProviderInstance{ProviderID: providerID, HostID: instance.HostID, InstanceDigest: instance.Digest}
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
		schemaVersion: ExecutionGraphSchemaV3, hostID: hostID, recipeID: recipe.ID, recipeVersion: recipe.RecipeVersion,
		recipeDigest: recipeDigest, entry: recipe.Entry, bindings: normalizedBindings,
		providerInstances: providerInstances, nodes: nodes, incidentRoutes: incidentRoutes,
		terminalGates: sortedStrings(recipe.TerminalGates), stableBoundaries: sortedStrings(recipe.StableBoundaries),
		eligibleTopologies: eligibleTopologies, environmentRequirements: requirements,
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

func indexAddOns(values []string, nodes []catalog.RecipeNode) (map[string]bool, error) {
	available := make(map[string]catalog.RecipeNode, len(nodes))
	for _, node := range nodes {
		available[node.ID] = node
	}
	selected := make(map[string]bool, len(values))
	for _, id := range values {
		if selected[id] {
			return nil, compileError("PROFILE_ADD_ON_INVALID", "add-on %q is duplicated", id)
		}
		node, found := available[id]
		if !found {
			return nil, compileError("PROFILE_ADD_ON_INVALID", "add-on %q is not declared", id)
		}
		if !node.Optional {
			return nil, compileError("PROFILE_ADD_ON_INVALID", "node %q is required and cannot be selected as an add-on", id)
		}
		selected[id] = true
	}
	return selected, nil
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

func declaredBinding(values []catalog.HostBinding, wanted catalog.HostBinding) (catalog.HostBinding, bool) {
	for _, value := range values {
		if value.Host == wanted.Host && value.Kind == wanted.Kind && value.Reference == wanted.Reference {
			return value, true
		}
	}
	return catalog.HostBinding{}, false
}

func compileNodeTopologies(capability catalog.CapabilityRecord, declared catalog.HostBinding, verified registry.VerifiedCapability) ([]execution.Topology, catalog.HostBinding, error) {
	capabilityTopologies, err := execution.NormalizeTopologies(capability.SupportedTopologies)
	if err != nil {
		return nil, catalog.HostBinding{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%s has invalid Capability topologies: %v", capability.ID, err)
	}
	bindingTopologies, err := execution.NormalizeTopologies(declared.Topologies)
	if err != nil {
		return nil, catalog.HostBinding{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%s has invalid binding topologies: %v", capability.ID, err)
	}
	verifiedTopologies, err := execution.NormalizeTopologies(verified.SupportedTopologies)
	if err != nil {
		return nil, catalog.HostBinding{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%s has invalid verified topologies: %v", capability.ID, err)
	}
	verifiedBindingTopologies, err := execution.NormalizeTopologies(verified.Binding.Topologies)
	if err != nil {
		return nil, catalog.HostBinding{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%s has invalid verified binding topologies: %v", capability.ID, err)
	}
	if !slices.Equal(bindingTopologies, verifiedTopologies) || !slices.Equal(bindingTopologies, verifiedBindingTopologies) {
		return nil, catalog.HostBinding{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%s verified binding topology evidence does not match its descriptor", capability.ID)
	}
	eligible, err := execution.IntersectTopologies(capabilityTopologies, bindingTopologies, verifiedTopologies, verifiedBindingTopologies)
	if err != nil || len(eligible) == 0 {
		return nil, catalog.HostBinding{}, compileError("PROFILE_TOPOLOGY_UNAVAILABLE", "%s has no eligible binding topology", capability.ID)
	}
	declared.Topologies = bindingTopologies
	return eligible, cloneHostBinding(declared), nil
}

func cloneHostBinding(value catalog.HostBinding) catalog.HostBinding {
	value.Topologies = append([]execution.Topology{}, value.Topologies...)
	return value
}

func normalizeRecipe(recipe catalog.ProfileRecipeRecord) catalog.ProfileRecipeRecord {
	recipe.RequiredResponsibilities = sortedStrings(recipe.RequiredResponsibilities)
	recipe.IncidentRoutes = append([]catalog.IncidentRoute{}, recipe.IncidentRoutes...)
	sort.Slice(recipe.IncidentRoutes, func(i, j int) bool {
		return recipe.IncidentRoutes[i].Incident+"\x00"+recipe.IncidentRoutes[i].Handler < recipe.IncidentRoutes[j].Incident+"\x00"+recipe.IncidentRoutes[j].Handler
	})
	recipe.TerminalGates = sortedStrings(recipe.TerminalGates)
	recipe.StableBoundaries = sortedStrings(recipe.StableBoundaries)
	requirements, _ := execution.NormalizeRequirements(recipe.EnvironmentRequirements)
	recipe.EnvironmentRequirements = requirements
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
