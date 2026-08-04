package profile

import (
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

func validateOwnership(nodes []GraphNode, required []string) error {
	owners := make(map[string]int, len(nodes))
	for _, node := range nodes {
		if node.Responsibility != "" {
			owners[node.Responsibility]++
		}
	}
	for responsibility, count := range owners {
		if count > 1 {
			return compileError("PROFILE_OWNER_DUPLICATE", "responsibility %s has %d owners", responsibility, count)
		}
	}
	for _, responsibility := range required {
		if owners[responsibility] == 0 {
			return compileError("PROFILE_OWNER_MISSING", "responsibility %s has no owner", responsibility)
		}
	}
	return nil
}

func validateCapabilityLimits(capability catalog.CapabilityRecord) error {
	for _, effect := range capability.MaximumEffects {
		if !knownEffect(effect) {
			return compileError("PROFILE_EFFECT_UNSUPPORTED", "%s declares unsupported effect %s", capability.ID, effect)
		}
	}
	for _, resource := range capability.Resources {
		if !knownResource(resource) {
			return compileError("PROFILE_EFFECT_UNSUPPORTED", "%s declares unsupported resource %s", capability.ID, resource)
		}
	}
	return nil
}

func knownEffect(value string) bool {
	switch value {
	case "read-project", "write-project", "run-process", "git-local", "network-read":
		return true
	default:
		return false
	}
}

func knownResource(value string) bool {
	switch value {
	case "project", "project-worktree", "git-repository":
		return true
	default:
		return false
	}
}

func requestModePresent(values []catalog.RequestMode, wanted catalog.RequestMode) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func stringPresent(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func validateExecutionGraph(graph ExecutionGraph, omitted map[string]bool) error {
	if _, err := catalog.ParseLocalID(graph.hostID); err != nil {
		return compileError("HOST_PROVIDER_SCOPE_MISMATCH", "Execution Graph has invalid Host %q", graph.hostID)
	}
	for _, provider := range graph.providerInstances {
		if provider.HostID != graph.hostID {
			return compileError("HOST_PROVIDER_SCOPE_MISMATCH", "Graph Provider %s belongs to Host %q, not %q", provider.ProviderID, provider.HostID, graph.hostID)
		}
	}
	nodes := make(map[string]GraphNode, len(graph.nodes))
	for _, node := range graph.nodes {
		if node.Binding.Host != graph.hostID {
			return compileError("HOST_PROVIDER_SCOPE_MISMATCH", "Graph node %s Binding belongs to Host %q, not %q", node.ID, node.Binding.Host, graph.hostID)
		}
		nodes[node.ID] = node
	}
	entry, found := nodes[graph.entry]
	if !found || entry.Kind == catalog.ProcedureNode {
		return compileError("PROFILE_NODE_MISSING", "entry node %s is not a retained control node", graph.entry)
	}
	if len(graph.terminalGates) == 0 {
		return compileError("PROFILE_TERMINAL_INVALID", "recipe has no terminal gate")
	}
	terminals := make(map[string]struct{}, len(graph.terminalGates))
	for _, gateID := range graph.terminalGates {
		gate, exists := nodes[gateID]
		if !exists {
			return compileError("PROFILE_NODE_MISSING", "terminal gate %s is not retained", gateID)
		}
		if gate.Kind != catalog.GateNode || len(gate.Transitions) != 0 {
			return compileError("PROFILE_TERMINAL_INVALID", "terminal gate %s is not a transitionless gate", gateID)
		}
		terminals[gateID] = struct{}{}
	}
	for _, node := range graph.nodes {
		if node.Kind == catalog.ProcedureNode {
			phase, exists := nodes[node.Phase]
			if !exists || phase.Kind != catalog.PhaseNode {
				return compileError("PROFILE_NODE_MISSING", "procedure %s has no retained phase %s", node.ID, node.Phase)
			}
			if len(node.Transitions) != 0 {
				return compileError("PROFILE_TERMINAL_INVALID", "procedure %s has graph transitions", node.ID)
			}
			continue
		}
		if node.Phase != "" {
			return compileError("PROFILE_TERMINAL_INVALID", "control node %s has a procedure phase", node.ID)
		}
		for _, transition := range node.Transitions {
			if omitted[transition.Target] {
				return compileError("PROFILE_NODE_MISSING", "transition %s/%s targets omitted node %s", node.ID, transition.Signal, transition.Target)
			}
			if _, exists := nodes[transition.Target]; !exists {
				return compileError("PROFILE_NODE_MISSING", "transition %s/%s targets missing node %s", node.ID, transition.Signal, transition.Target)
			}
		}
	}
	for _, route := range graph.incidentRoutes {
		handler, exists := nodes[route.Handler]
		if !exists || handler.Kind != catalog.IncidentHandlerNode {
			return compileError("PROFILE_NODE_MISSING", "incident route %s targets invalid handler %s", route.Incident, route.Handler)
		}
	}
	adjacency := controlAdjacency(graph.nodes)
	forward := reachableFrom(graph.entry, adjacency)
	for _, route := range graph.incidentRoutes {
		forward = mergeReachable(forward, reachableFrom(route.Handler, adjacency))
	}
	reverse := reverseAdjacency(adjacency)
	canTerminate := reachableFromMany(graph.terminalGates, reverse)
	for nodeID := range forward {
		if _, exists := canTerminate[nodeID]; exists {
			continue
		}
		if hasCycle(nodeID, adjacency) {
			return compileError("PROFILE_LOOP_NOT_CLOSED", "control cycle at %s has no terminal exit", nodeID)
		}
		return compileError("PROFILE_GRAPH_UNREACHABLE", "control node %s cannot reach a terminal gate", nodeID)
	}
	for _, node := range graph.nodes {
		if node.Kind != catalog.ProcedureNode {
			if _, exists := forward[node.ID]; !exists {
				return compileError("PROFILE_GRAPH_UNREACHABLE", "control node %s is unreachable", node.ID)
			}
		}
	}
	return nil
}

func controlAdjacency(nodes []GraphNode) map[string][]string {
	result := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		if node.Kind == catalog.ProcedureNode {
			continue
		}
		for _, transition := range node.Transitions {
			result[node.ID] = append(result[node.ID], transition.Target)
		}
		if _, exists := result[node.ID]; !exists {
			result[node.ID] = []string{}
		}
	}
	return result
}

func reverseAdjacency(adjacency map[string][]string) map[string][]string {
	result := make(map[string][]string, len(adjacency))
	for source, targets := range adjacency {
		if _, exists := result[source]; !exists {
			result[source] = []string{}
		}
		for _, target := range targets {
			result[target] = append(result[target], source)
		}
	}
	return result
}

func reachableFrom(start string, adjacency map[string][]string) map[string]struct{} {
	return reachableFromMany([]string{start}, adjacency)
}

func reachableFromMany(starts []string, adjacency map[string][]string) map[string]struct{} {
	seen := make(map[string]struct{}, len(starts))
	queue := append([]string{}, starts...)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := seen[current]; exists {
			continue
		}
		seen[current] = struct{}{}
		queue = append(queue, adjacency[current]...)
	}
	return seen
}

func mergeReachable(left, right map[string]struct{}) map[string]struct{} {
	for value := range right {
		left[value] = struct{}{}
	}
	return left
}

func hasCycle(start string, adjacency map[string][]string) bool {
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(string) bool
	visit = func(current string) bool {
		if visiting[current] {
			return true
		}
		if visited[current] {
			return false
		}
		visiting[current] = true
		for _, target := range adjacency[current] {
			if visit(target) {
				return true
			}
		}
		delete(visiting, current)
		visited[current] = true
		return false
	}
	return visit(start)
}
