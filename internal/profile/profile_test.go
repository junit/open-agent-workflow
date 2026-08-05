package profile_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestCompileRecipeIntersectsHostCapabilityBindingAndEnvironmentTopologies(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	providers := available.Providers()
	for providerIndex := range providers {
		for capabilityIndex := range providers[providerIndex].Capabilities {
			capability := &providers[providerIndex].Capabilities[capabilityIndex]
			if capability.ID == "implementation" {
				capability.SupportedTopologies = []execution.Topology{execution.TopologyCurrent}
				capability.HostBindings[0].Topologies = []execution.Topology{execution.TopologyCurrent}
			}
		}
	}
	implementation := verified.capabilities["acme/suite\x00implementation"]
	implementation.SupportedTopologies = []execution.Topology{execution.TopologyCurrent}
	implementation.Binding.Topologies = []execution.Topology{execution.TopologyCurrent}
	verified.capabilities["acme/suite\x00implementation"] = implementation
	recipe.EnvironmentRequirements = []execution.EnvironmentRequirement{{
		Surface: "skills", Required: true,
		AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited},
	}}

	graph, err := profile.CompileRecipe(
		catalogSource{providers: providers, recipes: available.Recipes(), aliases: available.Aliases()},
		verified,
		recipe,
		profile.CompileRequest{
			HostTopologies: dualTopologies(),
			EnvironmentObservations: []execution.EnvironmentObservation{{
				Surface: "skills", Disposition: execution.DispositionInherited,
				Source: "codex-session", Digest: strings.Repeat("a", 64),
			}},
		},
	)
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if want := []execution.Topology{execution.TopologyCurrent}; !slices.Equal(graph.EligibleTopologies(), want) {
		t.Fatalf("EligibleTopologies() = %#v, want %#v", graph.EligibleTopologies(), want)
	}
	if !slices.Equal(graphNode(graph, "implementation").SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("implementation topologies = %#v", graphNode(graph, "implementation").SupportedTopologies)
	}
	eligible := graph.EligibleTopologies()
	eligible[0] = execution.TopologySubagent
	requirements := graph.EnvironmentRequirements()
	requirements[0].AcceptedDispositions[0] = execution.DispositionRestricted
	if !slices.Equal(graph.EligibleTopologies(), []execution.Topology{execution.TopologyCurrent}) || graph.EnvironmentRequirements()[0].AcceptedDispositions[0] != execution.DispositionInherited {
		t.Fatal("ExecutionGraph exposed topology or environment storage")
	}
	_, err = profile.CompileRecipe(
		catalogSource{providers: providers, recipes: available.Recipes(), aliases: available.Aliases()},
		verified,
		recipe,
		profile.CompileRequest{
			HostTopologies: []execution.Topology{execution.TopologySubagent},
			EnvironmentObservations: []execution.EnvironmentObservation{{
				Surface: "skills", Disposition: execution.DispositionInherited,
				Source: "codex-session", Digest: strings.Repeat("a", 64),
			}},
		},
	)
	requireCompileCode(t, err, "PROFILE_TOPOLOGY_UNAVAILABLE")

	_, err = profile.CompileRecipe(
		catalogSource{providers: providers, recipes: available.Recipes(), aliases: available.Aliases()},
		verified,
		recipe,
		profile.CompileRequest{
			HostTopologies: dualTopologies(),
			EnvironmentObservations: []execution.EnvironmentObservation{{
				Surface: "skills", Disposition: execution.DispositionRestricted,
				Source: "codex-session", Digest: strings.Repeat("a", 64),
			}},
		},
	)
	requireCompileCode(t, err, "PROFILE_TOPOLOGY_UNAVAILABLE")
}

func TestCompileRecipeRequiresExplicitAddOns(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "optional-repair", Kind: catalog.IncidentHandlerNode, Responsibility: "optional-repair", Optional: true,
		Selector:    catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "optional-repair"},
		Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}},
	})
	recipe.IncidentRoutes = []catalog.IncidentRoute{{Incident: "build-failure", Handler: "optional-repair"}}

	without, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	if err != nil {
		t.Fatalf("CompileRecipe(without add-on) error = %v", err)
	}
	if graphNode(without, "optional-repair").ID != "" || len(without.IncidentRoutes()) != 0 {
		t.Fatalf("unselected add-on retained: %#v %#v", without.Nodes(), without.IncidentRoutes())
	}

	request := defaultCompileRequest()
	request.AddOns = []string{"optional-repair"}
	with, err := profile.CompileRecipe(available, verified, recipe, request)
	if err != nil {
		t.Fatalf("CompileRecipe(with add-on) error = %v", err)
	}
	if graphNode(with, "optional-repair").ID == "" || len(with.IncidentRoutes()) != 1 {
		t.Fatalf("selected add-on omitted: %#v %#v", with.Nodes(), with.IncidentRoutes())
	}

	for _, addOns := range [][]string{{"missing"}, {"optional-repair", "optional-repair"}, {"implementation"}} {
		request := defaultCompileRequest()
		request.AddOns = addOns
		_, err := profile.CompileRecipe(available, verified, recipe, request)
		requireCompileCode(t, err, "PROFILE_ADD_ON_INVALID")
	}
	delete(verified.capabilities, "acme/suite\x00optional-repair")
	_, err = profile.CompileRecipe(available, verified, recipe, request)
	requireCompileCode(t, err, "PROFILE_ADD_ON_INVALID")
}

func TestCompileRecipePinsVerifiedCapabilityContract(t *testing.T) {
	available, verified, recipe := compilerFixture(t)

	graph, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if graph.SchemaVersion() != profile.ExecutionGraphSchemaV3 || graph.HostID() != "codex" || graph.RecipeID() != recipe.ID || graph.RecipeVersion() != recipe.RecipeVersion {
		t.Fatalf("graph identity = %q %q %q", graph.SchemaVersion(), graph.RecipeID(), graph.RecipeVersion())
	}
	if graph.RecipeDigest() == "" || graph.Digest() == "" || graph.Entry() != "implementation" {
		t.Fatalf("graph digests/entry = %q %q %q", graph.RecipeDigest(), graph.Digest(), graph.Entry())
	}

	providers := graph.ProviderInstances()
	if len(providers) != 1 || providers[0].ProviderID != "acme/suite" || providers[0].HostID != "codex" || providers[0].InstanceDigest != "acme-instance-digest" {
		t.Fatalf("ProviderInstances() = %#v", providers)
	}
	nodes := graph.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("Nodes() = %#v", nodes)
	}
	implementation := nodes[1]
	if nodes[0].ID == "implementation" {
		implementation = nodes[0]
	}
	if implementation.ID != "implementation" || implementation.Kind != catalog.PhaseNode || implementation.Responsibility != "implementation" {
		t.Fatalf("implementation node = %#v", implementation)
	}
	if implementation.ProviderID != "acme/suite" || implementation.ProviderInstanceDigest != "acme-instance-digest" || implementation.CapabilityID != "implementation" {
		t.Fatalf("resolved node identity = %#v", implementation)
	}
	if !reflect.DeepEqual(implementation.Binding, catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:implement", Topologies: dualTopologies()}) {
		t.Fatalf("node binding = %#v", implementation.Binding)
	}
	if !equalStrings(implementation.MaximumEffects, []string{"read-project", "run-process", "write-project"}) || !equalStrings(implementation.Resources, []string{"project-worktree"}) {
		t.Fatalf("effects/resources = %#v / %#v", implementation.MaximumEffects, implementation.Resources)
	}
	if len(implementation.RequestModes) != 1 || implementation.RequestModes[0] != catalog.RequestModeWorkflow || !slices.Equal(implementation.SupportedTopologies, dualTopologies()) {
		t.Fatalf("mode/topologies = %#v / %#v", implementation.RequestModes, implementation.SupportedTopologies)
	}
	if implementation.InputSchema != "acme.input/v1" || implementation.OutcomeSchema != "acme.outcome/v1" || !equalStrings(implementation.DelegationAllowList, []string{"review"}) {
		t.Fatalf("schemas/delegation = %q / %q / %#v", implementation.InputSchema, implementation.OutcomeSchema, implementation.DelegationAllowList)
	}
	if len(implementation.Transitions) != 1 || implementation.Transitions[0] != (profile.GraphTransition{Signal: "succeeded", Target: "completion"}) {
		t.Fatalf("transitions = %#v", implementation.Transitions)
	}
	if !equalStrings(graph.TerminalGates(), []string{"completion"}) || !equalStrings(graph.StableBoundaries(), []string{"ticket-complete"}) || len(graph.IncidentRoutes()) != 0 || len(graph.Bindings()) != 0 {
		t.Fatalf("graph control metadata = %#v %#v %#v %#v", graph.TerminalGates(), graph.StableBoundaries(), graph.IncidentRoutes(), graph.Bindings())
	}

	providers[0].InstanceDigest = "changed"
	nodes[0].MaximumEffects[0] = "changed"
	nodes[0].Transitions = append(nodes[0].Transitions, profile.GraphTransition{Signal: "finding", Target: "implementation"})
	terminals := graph.TerminalGates()
	terminals[0] = "changed"
	if graph.ProviderInstances()[0].InstanceDigest != "acme-instance-digest" || graph.Nodes()[0].MaximumEffects[0] == "changed" || len(graph.Nodes()[0].Transitions) > 1 || graph.TerminalGates()[0] != "completion" {
		t.Fatal("ExecutionGraph exposed mutable storage")
	}
}

func TestCompileRecipeRejectsProviderFromAnotherHost(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	instance := verified.providers["acme/suite"]
	instance.HostID = "claude"
	verified.providers["acme/suite"] = instance
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "HOST_PROVIDER_SCOPE_MISMATCH")
}

func TestCompileRecipeAppliesExactProviderBinding(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	graph, err := profile.CompileRecipe(available, verified, recipe, profile.CompileRequest{Bindings: []profile.ProfileBinding{{
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"},
		PreferredProviderID: "vendor/suite",
	}, {
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "completion"},
		PreferredProviderID: "acme/suite",
	}}, HostTopologies: dualTopologies()})
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	implementation := graphNode(graph, "implementation")
	if implementation.ProviderID != "vendor/suite" || implementation.ProviderInstanceDigest != "vendor-instance-digest" || implementation.Binding.Reference != "vendor:implement" {
		t.Fatalf("bound implementation = %#v", implementation)
	}
	if len(graph.ProviderInstances()) != 2 || len(graph.Bindings()) != 2 {
		t.Fatalf("binding/provider pins = %#v / %#v", graph.Bindings(), graph.ProviderInstances())
	}
}

func TestCompileProfileResolvesAliasAndReportsUnknownProfile(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	aliased, err := catalog.New(available.Providers(), available.Recipes(), []catalog.ProfileAliasRecord{{Alias: "ACME-FULL", RecipeID: recipe.ID}})
	if err != nil {
		t.Fatalf("catalog.New(alias) error = %v", err)
	}
	graph, err := profile.CompileProfile(aliased, verified, profile.CompileRequest{Profile: "ACME-FULL", HostTopologies: dualTopologies()})
	if err != nil || graph.RecipeID() != recipe.ID {
		t.Fatalf("CompileProfile(alias) = %#v, %v", graph, err)
	}
	_, err = profile.CompileProfile(aliased, verified, profile.CompileRequest{Profile: "UNKNOWN", HostTopologies: dualTopologies()})
	requireCompileCode(t, err, "PROFILE_NOT_FOUND")
	if err.Error() == "" {
		t.Fatal("CompileError.Error() is empty")
	}
}

func TestExecutionGraphRecordIsDigestPinnedAndDefensive(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	graph, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	record := graph.Record()
	if record.ContentDigest() != graph.Digest() || record.RecipeDigest == "" || len(record.Nodes) == 0 {
		t.Fatalf("graph record = %#v", record)
	}
	if err := profile.ValidateExecutionGraphRecord(record); err != nil {
		t.Fatalf("ValidateExecutionGraphRecord() error = %v", err)
	}
	v2Record := record
	v2Record.SchemaVersion = "oaw.execution-graph/v2"
	if err := profile.ValidateExecutionGraphRecord(v2Record); err == nil {
		t.Fatal("ValidateExecutionGraphRecord() accepted a v2 record")
	}
	implementationIndex := -1
	for index := range record.Nodes {
		if record.Nodes[index].ID == "implementation" {
			implementationIndex = index
			break
		}
	}
	if implementationIndex < 0 {
		t.Fatal("implementation node missing from graph record")
	}
	record.Nodes[implementationIndex].Transitions[0].Target = "mutated"
	record.Nodes[implementationIndex].SupportedTopologies[0] = execution.TopologySubagent
	record.Nodes[implementationIndex].Binding.Topologies[0] = execution.TopologySubagent
	record.EligibleTopologies[0] = execution.TopologySubagent
	record.Bindings = append(record.Bindings, profile.ProfileBinding{})
	if err := profile.ValidateExecutionGraphRecord(record); err == nil {
		t.Fatal("ValidateExecutionGraphRecord() accepted tampered record")
	}
	second := graph.Record()
	if graphNodeFromRecord(second, "implementation").Transitions[0].Target == "mutated" || graphNodeFromRecord(second, "implementation").SupportedTopologies[0] != execution.TopologyCurrent || graphNodeFromRecord(second, "implementation").Binding.Topologies[0] != execution.TopologyCurrent || second.EligibleTopologies[0] != execution.TopologyCurrent || len(second.Bindings) != len(graph.Bindings()) {
		t.Fatalf("ExecutionGraph.Record() leaked mutable state: %#v", second)
	}
}

func graphNodeFromRecord(record profile.ExecutionGraphRecord, id string) profile.GraphNode {
	for _, node := range record.Nodes {
		if node.ID == id {
			return node
		}
	}
	return profile.GraphNode{}
}

func TestCompileRecipeRejectsUnsupportedEffectAtCompilerBoundary(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	providers := available.Providers()
	providers[0].Capabilities[0].MaximumEffects = append(providers[0].Capabilities[0].MaximumEffects, "delete-project")
	unsafeCatalog := catalogSource{providers: providers, recipes: available.Recipes(), aliases: available.Aliases()}
	_, err := profile.CompileRecipe(unsafeCatalog, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_EFFECT_UNSUPPORTED")

	providers[0].Capabilities[0].MaximumEffects = []string{"read-project"}
	providers[0].Capabilities[0].Resources = []string{"secret-store"}
	_, err = profile.CompileRecipe(catalogSource{providers: providers, recipes: available.Recipes(), aliases: available.Aliases()}, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_EFFECT_UNSUPPORTED")
}

func TestCompileRecipeRejectsUnverifiedBindingAndMissingDescriptor(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	verified.capabilities["acme/suite\x00implementation"] = registry.VerifiedCapability{ID: "implementation", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "not-declared"}}
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")
	var compileErr *profile.CompileError
	if !errors.As(err, &compileErr) || compileErr.ProviderID != "" || compileErr.CapabilityID != "" {
		t.Fatalf("binding contract error exposed Provider resolution metadata: %#v", compileErr)
	}

	_, err = profile.CompileRecipe(catalogSource{providers: []catalog.ProviderDescriptorRecord{}, recipes: available.Recipes(), aliases: available.Aliases()}, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")
}

func TestCompileMissingCapabilityCarriesResolvedSelector(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	delete(verified.providers, "vendor/suite")
	delete(verified.capabilities, "vendor/suite\x00implementation")

	_, err := profile.CompileRecipe(available, verified, recipe, profile.CompileRequest{Bindings: []profile.ProfileBinding{{
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"},
		PreferredProviderID: "vendor/suite",
	}}, HostTopologies: dualTopologies()})
	var compileErr *profile.CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if compileErr.Code != "PROFILE_CAPABILITY_MISSING" || compileErr.ProviderID != "vendor/suite" || compileErr.CapabilityID != "implementation" {
		t.Fatalf("CompileError = %#v", compileErr)
	}
}

func TestCompileRecipeNormalizesMultipleTransitionsAndIncidentRoutes(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes[0].Transitions = []catalog.RecipeTransition{
		{Signal: "finding", Target: "repair"},
		{Signal: "succeeded", Target: "completion"},
	}
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "repair", Kind: catalog.IncidentHandlerNode, Responsibility: "optional-repair",
		Selector:    catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "optional-repair"},
		Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}},
	})
	recipe.IncidentRoutes = []catalog.IncidentRoute{{Incident: "build-failure", Handler: "repair"}, {Incident: "functional-failure", Handler: "repair"}}
	graph, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if len(graph.Nodes()) != 3 || len(graph.IncidentRoutes()) != 2 || len(graphNode(graph, "implementation").Transitions) != 2 {
		t.Fatalf("normalized control graph = %#v %#v", graph.Nodes(), graph.IncidentRoutes())
	}
}

func TestCompileErrorWithoutDetailHasStableText(t *testing.T) {
	if (&profile.CompileError{Code: "PROFILE_TEST"}).Error() != "PROFILE_TEST" {
		t.Fatal("CompileError without detail changed its stable text")
	}
}

func TestCompileRecipeRejectsDuplicateBinding(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	_, err := profile.CompileRecipe(available, verified, recipe, profile.CompileRequest{Bindings: []profile.ProfileBinding{
		{Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"}, PreferredProviderID: "vendor/suite"},
		{Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"}, PreferredProviderID: "acme/suite"},
	}, HostTopologies: dualTopologies()})
	requireCompileCode(t, err, "PROFILE_SELECTOR_AMBIGUOUS")
}

func TestCompileRecipeRejectsBindingForUnknownSelector(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	_, err := profile.CompileRecipe(available, verified, recipe, profile.CompileRequest{Bindings: []profile.ProfileBinding{{
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "missing"},
		PreferredProviderID: "vendor/suite",
	}}, HostTopologies: dualTopologies()})
	requireCompileCode(t, err, "PROFILE_SELECTOR_NOT_FOUND")
}

func TestCompileRecipeRejectsMissingVerifiedCapability(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	delete(verified.capabilities, "acme/suite\x00implementation")
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")
}

func TestCompileRecipeRejectsMismatchedVerifiedIdentity(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	provider := verified.providers["acme/suite"]
	provider.ProviderID = "vendor/suite"
	verified.providers["acme/suite"] = provider
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")

	available, verified, recipe = compilerFixture(t)
	capability := verified.capabilities["acme/suite\x00implementation"]
	capability.ID = "other"
	verified.capabilities["acme/suite\x00implementation"] = capability
	_, err = profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")
}

func TestCompileRecipeRequiresExactlyOneOwner(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.RequiredResponsibilities = []string{"missing"}
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_OWNER_MISSING")

	recipe.RequiredResponsibilities = []string{"implementation", "completion"}
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "duplicate", Kind: catalog.PhaseNode, Responsibility: "implementation",
		Selector:    catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"},
		Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}},
	})
	_, err = profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_OWNER_DUPLICATE")
}

func TestCompileRecipeRequiresWorkflowCapability(t *testing.T) {
	available, verified, recipe := compilerFixtureWithImplementationMode(t, catalog.RequestModeBounded)
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_REQUEST_MODE_UNSUPPORTED")
}

func TestCompileRecipeRejectsMissingControlTarget(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes[0].Transitions[0].Target = "missing"
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_NODE_MISSING")
}

func TestCompileRecipeRejectsInvalidProcedureAndTerminal(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "procedure", Kind: catalog.ProcedureNode, Responsibility: "optional-repair",
		Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "optional-repair"}, Phase: "missing", Transitions: []catalog.RecipeTransition{},
	})
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_NODE_MISSING")

	_, _, recipe = compilerFixture(t)
	recipe.Nodes[1].Transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: "implementation"}}
	_, err = profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_TERMINAL_INVALID")
}

func TestCompileRecipeRejectsUnreachableAndUnclosedLoop(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "dead", Kind: catalog.CheckpointNode, Responsibility: "",
		Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "review"}, Transitions: []catalog.RecipeTransition{},
	})
	_, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_GRAPH_UNREACHABLE")

	_, _, recipe = compilerFixture(t)
	recipe.Nodes[0].Transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: "implementation"}}
	_, err = profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	requireCompileCode(t, err, "PROFILE_LOOP_NOT_CLOSED")
}

func TestCompileRecipeOmitsOptionalIncidentHandlerAndRoute(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "optional-repair", Kind: catalog.IncidentHandlerNode, Responsibility: "optional-repair", Optional: true,
		Selector:    catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "optional-repair"},
		Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}},
	})
	recipe.IncidentRoutes = []catalog.IncidentRoute{{Incident: "build-failure", Handler: "optional-repair"}}
	delete(verified.capabilities, "acme/suite\x00optional-repair")
	graph, err := profile.CompileRecipe(available, verified, recipe, defaultCompileRequest())
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if graphNode(graph, "optional-repair").ID != "" || len(graph.IncidentRoutes()) != 0 {
		t.Fatalf("optional handler was retained: nodes=%#v routes=%#v", graph.Nodes(), graph.IncidentRoutes())
	}
}

type fakeRegistry struct {
	hostID       string
	providers    map[string]registry.ProviderInstance
	capabilities map[string]registry.VerifiedCapability
}

func (value fakeRegistry) HostID() string { return value.hostID }

type catalogSource struct {
	providers []catalog.ProviderDescriptorRecord
	recipes   []catalog.ProfileRecipeRecord
	aliases   []catalog.ProfileAliasRecord
}

func (value catalogSource) Providers() []catalog.ProviderDescriptorRecord { return value.providers }
func (value catalogSource) Recipes() []catalog.ProfileRecipeRecord        { return value.recipes }
func (value catalogSource) Aliases() []catalog.ProfileAliasRecord         { return value.aliases }

func (value fakeRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	provider, found := value.providers[id]
	return provider, found
}

func (value fakeRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	capability, found := value.capabilities[providerID+"\x00"+capabilityID]
	return capability, found
}

func compilerFixture(t *testing.T) (catalog.Catalog, fakeRegistry, catalog.ProfileRecipeRecord) {
	return compilerFixtureWithImplementationMode(t, catalog.RequestModeWorkflow)
}

func compilerFixtureWithImplementationMode(t *testing.T, implementationMode catalog.RequestMode) (catalog.Catalog, fakeRegistry, catalog.ProfileRecipeRecord) {
	t.Helper()
	implementationBinding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:implement", Topologies: dualTopologies()}
	completionBinding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:complete", Topologies: dualTopologies()}
	provider := catalog.ProviderDescriptorRecord{
		SchemaVersion:     catalog.ProviderDescriptorSchemaV3,
		DescriptorVersion: "3.0.0",
		ID:                "acme/suite",
		DisplayName:       "Acme Suite",
		Discovery:         []catalog.DiscoveryProbe{{ID: "codex", Hosts: []string{"codex"}, Surface: "codex-skills", Distribution: "acme", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/skills/acme", EvidencePath: "SKILL.md"}},
		Capabilities: []catalog.CapabilityRecord{
			{
				ID: "implementation", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"write-project", "read-project", "run-process"}, Resources: []string{"project-worktree"},
				RequestModes: []catalog.RequestMode{implementationMode}, Responsibilities: []string{"implementation"},
				SupportedTopologies: dualTopologies(), DelegationAllowList: []string{"review"}, HostBindings: []catalog.HostBinding{implementationBinding},
			},
			{
				ID: "review", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
				RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{},
				SupportedTopologies: dualTopologies(), DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "acme:review", Topologies: dualTopologies()}},
			},
			{
				ID: "completion", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"read-project"}, Resources: []string{"git-repository"},
				RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{"completion"},
				SupportedTopologies: dualTopologies(), DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{completionBinding},
			},
			{
				ID: "optional-repair", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"read-project", "write-project"}, Resources: []string{"project-worktree"},
				RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{"optional-repair"},
				SupportedTopologies: dualTopologies(), DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "acme:optional-repair", Topologies: dualTopologies()}},
			},
		},
	}
	alternate := provider
	alternate.ID = "vendor/suite"
	alternate.DisplayName = "Vendor Suite"
	alternate.Capabilities = append([]catalog.CapabilityRecord{}, provider.Capabilities...)
	alternate.Capabilities[0].HostBindings = []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "vendor:implement", Topologies: dualTopologies()}}
	recipe := catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV2, RecipeVersion: "2.0.0", ID: "acme/delivery", DisplayName: "Acme Delivery",
		RequiredResponsibilities: []string{"implementation", "completion"},
		Nodes: []catalog.RecipeNode{
			{ID: "implementation", Kind: catalog.PhaseNode, Responsibility: "implementation", Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"}, Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}}},
			{ID: "completion", Kind: catalog.GateNode, Responsibility: "completion", Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "completion"}, Transitions: []catalog.RecipeTransition{}},
		},
		IncidentRoutes: []catalog.IncidentRoute{}, Entry: "implementation", TerminalGates: []string{"completion"}, StableBoundaries: []string{"ticket-complete"}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
	available, err := catalog.New([]catalog.ProviderDescriptorRecord{provider, alternate}, []catalog.ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatalf("catalog.New() error = %v", err)
	}
	instance := registry.ProviderInstance{ProviderID: "acme/suite", HostID: "codex", Digest: "acme-instance-digest", Capabilities: []registry.VerifiedCapability{
		{ID: "implementation", Binding: implementationBinding, SupportedTopologies: dualTopologies()},
		{ID: "review", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review", Topologies: dualTopologies()}, SupportedTopologies: dualTopologies()},
		{ID: "completion", Binding: completionBinding, SupportedTopologies: dualTopologies()},
		{ID: "optional-repair", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:optional-repair", Topologies: dualTopologies()}, SupportedTopologies: dualTopologies()},
	}}
	alternateInstance := registry.ProviderInstance{ProviderID: "vendor/suite", HostID: "codex", Digest: "vendor-instance-digest", Capabilities: []registry.VerifiedCapability{
		{ID: "implementation", Binding: alternate.Capabilities[0].HostBindings[0], SupportedTopologies: dualTopologies()},
	}}
	verified := fakeRegistry{
		hostID:    "codex",
		providers: map[string]registry.ProviderInstance{"acme/suite": instance, "vendor/suite": alternateInstance},
		capabilities: map[string]registry.VerifiedCapability{
			"acme/suite\x00implementation":   {ID: "implementation", Binding: implementationBinding, SupportedTopologies: dualTopologies()},
			"acme/suite\x00review":           {ID: "review", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review", Topologies: dualTopologies()}, SupportedTopologies: dualTopologies()},
			"acme/suite\x00completion":       {ID: "completion", Binding: completionBinding, SupportedTopologies: dualTopologies()},
			"vendor/suite\x00implementation": {ID: "implementation", Binding: alternate.Capabilities[0].HostBindings[0], SupportedTopologies: dualTopologies()},
			"acme/suite\x00optional-repair":  {ID: "optional-repair", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:optional-repair", Topologies: dualTopologies()}, SupportedTopologies: dualTopologies()},
		},
	}
	return available, verified, recipe
}

func graphNode(graph profile.ExecutionGraph, id string) profile.GraphNode {
	for _, node := range graph.Nodes() {
		if node.ID == id {
			return node
		}
	}
	return profile.GraphNode{}
}

func requireCompileCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("CompileRecipe() error = nil, want %s", code)
	}
	var compileErr *profile.CompileError
	if !errors.As(err, &compileErr) || compileErr.Code != code {
		t.Fatalf("CompileRecipe() error = %v, want code %s", err, code)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func dualTopologies() []execution.Topology {
	return []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
}

func defaultCompileRequest() profile.CompileRequest {
	return profile.CompileRequest{HostTopologies: dualTopologies()}
}
