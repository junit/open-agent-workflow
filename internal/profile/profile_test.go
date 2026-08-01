package profile_test

import (
	"errors"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestCompileRecipePinsVerifiedCapabilityContract(t *testing.T) {
	available, verified, recipe := compilerFixture(t)

	graph, err := profile.CompileRecipe(available, verified, recipe, nil)
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if graph.SchemaVersion() != profile.ExecutionGraphSchemaV1 || graph.RecipeID() != recipe.ID || graph.RecipeVersion() != recipe.RecipeVersion {
		t.Fatalf("graph identity = %q %q %q", graph.SchemaVersion(), graph.RecipeID(), graph.RecipeVersion())
	}
	if graph.RecipeDigest() == "" || graph.Digest() == "" || graph.Entry() != "implementation" {
		t.Fatalf("graph digests/entry = %q %q %q", graph.RecipeDigest(), graph.Digest(), graph.Entry())
	}

	providers := graph.ProviderInstances()
	if len(providers) != 1 || providers[0].ProviderID != "acme/suite" || providers[0].InstanceDigest != "acme-instance-digest" {
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
	if implementation.Binding != (catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:implement"}) {
		t.Fatalf("node binding = %#v", implementation.Binding)
	}
	if !equalStrings(implementation.MaximumEffects, []string{"read-project", "run-process", "write-project"}) || !equalStrings(implementation.Resources, []string{"project-worktree"}) {
		t.Fatalf("effects/resources = %#v / %#v", implementation.MaximumEffects, implementation.Resources)
	}
	if len(implementation.RequestModes) != 1 || implementation.RequestModes[0] != catalog.RequestModeWorkflow || implementation.ExecutorTopology != catalog.IsolatedRequired {
		t.Fatalf("mode/topology = %#v / %q", implementation.RequestModes, implementation.ExecutorTopology)
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

func TestCompileRecipeAppliesExactProviderBinding(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	graph, err := profile.CompileRecipe(available, verified, recipe, []profile.ProfileBinding{{
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"},
		PreferredProviderID: "vendor/suite",
	}, {
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "completion"},
		PreferredProviderID: "acme/suite",
	}})
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
	graph, err := profile.CompileProfile(aliased, verified, profile.CompileRequest{Profile: "ACME-FULL"})
	if err != nil || graph.RecipeID() != recipe.ID {
		t.Fatalf("CompileProfile(alias) = %#v, %v", graph, err)
	}
	_, err = profile.CompileProfile(aliased, verified, profile.CompileRequest{Profile: "UNKNOWN"})
	requireCompileCode(t, err, "PROFILE_NOT_FOUND")
	if err.Error() == "" {
		t.Fatal("CompileError.Error() is empty")
	}
}

func TestCompileRecipeRejectsUnsupportedEffectAtCompilerBoundary(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	providers := available.Providers()
	providers[0].Capabilities[0].MaximumEffects = append(providers[0].Capabilities[0].MaximumEffects, "delete-project")
	unsafeCatalog := catalogSource{providers: providers, recipes: available.Recipes(), aliases: available.Aliases()}
	_, err := profile.CompileRecipe(unsafeCatalog, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_EFFECT_UNSUPPORTED")

	providers[0].Capabilities[0].MaximumEffects = []string{"read-project"}
	providers[0].Capabilities[0].Resources = []string{"secret-store"}
	_, err = profile.CompileRecipe(catalogSource{providers: providers, recipes: available.Recipes(), aliases: available.Aliases()}, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_EFFECT_UNSUPPORTED")
}

func TestCompileRecipeRejectsUnverifiedBindingAndMissingDescriptor(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	verified.capabilities["acme/suite\x00implementation"] = registry.VerifiedCapability{ID: "implementation", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "not-declared"}}
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")

	_, err = profile.CompileRecipe(catalogSource{providers: []catalog.ProviderDescriptorRecord{}, recipes: available.Recipes(), aliases: available.Aliases()}, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")
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
	graph, err := profile.CompileRecipe(available, verified, recipe, nil)
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
	_, err := profile.CompileRecipe(available, verified, recipe, []profile.ProfileBinding{
		{Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"}, PreferredProviderID: "vendor/suite"},
		{Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"}, PreferredProviderID: "acme/suite"},
	})
	requireCompileCode(t, err, "PROFILE_SELECTOR_AMBIGUOUS")
}

func TestCompileRecipeRejectsBindingForUnknownSelector(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	_, err := profile.CompileRecipe(available, verified, recipe, []profile.ProfileBinding{{
		Selector:            catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "missing"},
		PreferredProviderID: "vendor/suite",
	}})
	requireCompileCode(t, err, "PROFILE_SELECTOR_NOT_FOUND")
}

func TestCompileRecipeRejectsMissingVerifiedCapability(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	delete(verified.capabilities, "acme/suite\x00implementation")
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")
}

func TestCompileRecipeRejectsMismatchedVerifiedIdentity(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	provider := verified.providers["acme/suite"]
	provider.ProviderID = "vendor/suite"
	verified.providers["acme/suite"] = provider
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")

	available, verified, recipe = compilerFixture(t)
	capability := verified.capabilities["acme/suite\x00implementation"]
	capability.ID = "other"
	verified.capabilities["acme/suite\x00implementation"] = capability
	_, err = profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_CAPABILITY_MISSING")
}

func TestCompileRecipeRequiresExactlyOneOwner(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.RequiredResponsibilities = []string{"missing"}
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_OWNER_MISSING")

	recipe.RequiredResponsibilities = []string{"implementation", "completion"}
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "duplicate", Kind: catalog.PhaseNode, Responsibility: "implementation",
		Selector:    catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"},
		Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}},
	})
	_, err = profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_OWNER_DUPLICATE")
}

func TestCompileRecipeRequiresWorkflowCapability(t *testing.T) {
	available, verified, recipe := compilerFixtureWithImplementationMode(t, catalog.RequestModeBounded)
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_REQUEST_MODE_UNSUPPORTED")
}

func TestCompileRecipeRejectsMissingControlTarget(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes[0].Transitions[0].Target = "missing"
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_NODE_MISSING")
}

func TestCompileRecipeRejectsInvalidProcedureAndTerminal(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "procedure", Kind: catalog.ProcedureNode, Responsibility: "optional-repair",
		Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "optional-repair"}, Phase: "missing", Transitions: []catalog.RecipeTransition{},
	})
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_NODE_MISSING")

	_, _, recipe = compilerFixture(t)
	recipe.Nodes[1].Transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: "implementation"}}
	_, err = profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_TERMINAL_INVALID")
}

func TestCompileRecipeRejectsUnreachableAndUnclosedLoop(t *testing.T) {
	available, verified, recipe := compilerFixture(t)
	recipe.Nodes = append(recipe.Nodes, catalog.RecipeNode{
		ID: "dead", Kind: catalog.CheckpointNode, Responsibility: "",
		Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "review"}, Transitions: []catalog.RecipeTransition{},
	})
	_, err := profile.CompileRecipe(available, verified, recipe, nil)
	requireCompileCode(t, err, "PROFILE_GRAPH_UNREACHABLE")

	_, _, recipe = compilerFixture(t)
	recipe.Nodes[0].Transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: "implementation"}}
	_, err = profile.CompileRecipe(available, verified, recipe, nil)
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
	graph, err := profile.CompileRecipe(available, verified, recipe, nil)
	if err != nil {
		t.Fatalf("CompileRecipe() error = %v", err)
	}
	if graphNode(graph, "optional-repair").ID != "" || len(graph.IncidentRoutes()) != 0 {
		t.Fatalf("optional handler was retained: nodes=%#v routes=%#v", graph.Nodes(), graph.IncidentRoutes())
	}
}

type fakeRegistry struct {
	providers    map[string]registry.ProviderInstance
	capabilities map[string]registry.VerifiedCapability
}

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
	implementationBinding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:implement"}
	completionBinding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:complete"}
	provider := catalog.ProviderDescriptorRecord{
		SchemaVersion:     catalog.ProviderDescriptorSchemaV1,
		DescriptorVersion: "1.0.0",
		ID:                "acme/suite",
		DisplayName:       "Acme Suite",
		Discovery:         []catalog.DiscoveryProbe{},
		Capabilities: []catalog.CapabilityRecord{
			{
				ID: "implementation", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"write-project", "read-project", "run-process"}, Resources: []string{"project-worktree"},
				RequestModes: []catalog.RequestMode{implementationMode}, Responsibilities: []string{"implementation"},
				ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{"review"}, HostBindings: []catalog.HostBinding{implementationBinding},
			},
			{
				ID: "review", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
				RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{},
				ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "acme:review"}},
			},
			{
				ID: "completion", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"read-project"}, Resources: []string{"git-repository"},
				RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{"completion"},
				ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{completionBinding},
			},
			{
				ID: "optional-repair", InputSchema: "acme.input/v1", OutcomeSchema: "acme.outcome/v1",
				MaximumEffects: []string{"read-project", "write-project"}, Resources: []string{"project-worktree"},
				RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{"optional-repair"},
				ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "acme:optional-repair"}},
			},
		},
	}
	alternate := provider
	alternate.ID = "vendor/suite"
	alternate.DisplayName = "Vendor Suite"
	alternate.Capabilities = append([]catalog.CapabilityRecord{}, provider.Capabilities...)
	alternate.Capabilities[0].HostBindings = []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "vendor:implement"}}
	recipe := catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV1, RecipeVersion: "1.0.0", ID: "acme/delivery", DisplayName: "Acme Delivery",
		RequiredResponsibilities: []string{"implementation", "completion"},
		Nodes: []catalog.RecipeNode{
			{ID: "implementation", Kind: catalog.PhaseNode, Responsibility: "implementation", Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "implementation"}, Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "completion"}}},
			{ID: "completion", Kind: catalog.GateNode, Responsibility: "completion", Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "completion"}, Transitions: []catalog.RecipeTransition{}},
		},
		IncidentRoutes: []catalog.IncidentRoute{}, Entry: "implementation", TerminalGates: []string{"completion"}, StableBoundaries: []string{"ticket-complete"},
	}
	available, err := catalog.New([]catalog.ProviderDescriptorRecord{provider, alternate}, []catalog.ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatalf("catalog.New() error = %v", err)
	}
	instance := registry.ProviderInstance{ProviderID: "acme/suite", Digest: "acme-instance-digest", Capabilities: []registry.VerifiedCapability{
		{ID: "implementation", Binding: implementationBinding},
		{ID: "review", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"}},
		{ID: "completion", Binding: completionBinding},
		{ID: "optional-repair", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:optional-repair"}},
	}}
	alternateInstance := registry.ProviderInstance{ProviderID: "vendor/suite", Digest: "vendor-instance-digest", Capabilities: []registry.VerifiedCapability{
		{ID: "implementation", Binding: alternate.Capabilities[0].HostBindings[0]},
	}}
	verified := fakeRegistry{
		providers: map[string]registry.ProviderInstance{"acme/suite": instance, "vendor/suite": alternateInstance},
		capabilities: map[string]registry.VerifiedCapability{
			"acme/suite\x00implementation":   {ID: "implementation", Binding: implementationBinding},
			"acme/suite\x00review":           {ID: "review", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"}},
			"acme/suite\x00completion":       {ID: "completion", Binding: completionBinding},
			"vendor/suite\x00implementation": {ID: "implementation", Binding: alternate.Capabilities[0].HostBindings[0]},
			"acme/suite\x00optional-repair":  {ID: "optional-repair", Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:optional-repair"}},
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
