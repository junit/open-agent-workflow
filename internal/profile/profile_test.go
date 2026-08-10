package profile_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestCompileProfileMaterializesTenSlotGraph(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found || len(result.Diagnostics()) != 0 || len(graph.Slots) != len(catalog.CanonicalSlots()) || graph.RegistryDigest != fixture.registry.Digest() || graph.HostEvidenceDigest != fixture.host.Digest() {
		t.Fatalf("CompileProfile() = %#v / %#v", graph, result.Diagnostics())
	}
	if graph.SchemaVersion != profile.ExecutionGraphSchemaV4 || graph.Selection.Profile != "TEST-FULL" || graph.Topology != execution.TopologyCurrent || graph.ContentDigest() != graph.Digest {
		t.Fatalf("graph identity = %#v", graph)
	}
	if err := profile.ValidateExecutionGraphRecord(graph); err != nil {
		t.Fatal(err)
	}
	first, err := profile.FirstActionableCursor(graph)
	if err != nil || first.SlotID != string(catalog.SlotProblemFraming) || first.Kind != execution.CursorBinding {
		t.Fatalf("FirstActionableCursor() = %#v, %v", first, err)
	}
}

func TestCompileProfileReturnsErrorForMalformedTrustedEvidence(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	request := fixture.request
	request.Host = profile.HostEvidence{}
	if _, err := profile.CompileProfile(fixture.catalog, fixture.registry, request); err == nil {
		t.Fatal("CompileProfile accepted zero Host evidence")
	}
	verified := fixture.registry.bindings["test/provider\x00implementation"]
	verified.Reference = "test:changed"
	fixture.registry.bindings["test/provider\x00implementation"] = verified
	if _, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request); err == nil || !strings.Contains(err.Error(), "PROFILE_TRUSTED_BINDING_MISMATCH") {
		t.Fatalf("CompileProfile trusted mismatch error = %v", err)
	}
}

func TestCompileRecipeUsesTheSamePipelineAsCompileProfile(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	byProfile, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	byRecipe, err := profile.CompileRecipe(fixture.catalog, fixture.registry, fixture.catalog.Recipes()[0], fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	profileGraph, _ := byProfile.Graph()
	recipeGraph, _ := byRecipe.Graph()
	if profileGraph.Digest != recipeGraph.Digest || byProfile.Digest() != byRecipe.Digest() {
		t.Fatalf("compiler paths differ: %s / %s", profileGraph.Digest, recipeGraph.Digest)
	}
}

func TestCompileRecipeDigestMatchesCatalogNormalizedRecord(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	recipe := fixture.catalog.Recipes()[0]
	_, digest, err := catalog.NormalizeAndDigestRecipe(fixture.catalog.Providers(), recipe)
	if err != nil {
		t.Fatal(err)
	}
	result, err := profile.CompileRecipe(fixture.catalog, fixture.registry, recipe, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, _ := result.Graph()
	if graph.RecipeDigest != digest || graph.Selection.RecipeDigest != digest {
		t.Fatalf("Recipe digest pins = %#v", graph)
	}
}

func TestCompileReturnsDigestPinnedUnavailableDiagnostics(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	delete(fixture.registry.bindings, "test/provider\x00implementation")
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := result.Graph(); found || result.Digest() == "" {
		t.Fatalf("unexpected graph or digest: %#v", result)
	}
	diagnostics := result.Diagnostics()
	if len(diagnostics) != 1 {
		t.Fatalf("Diagnostics() = %#v", diagnostics)
	}
	if !slices.ContainsFunc(diagnostics, func(value profile.CompileDiagnostic) bool {
		return value.Code == "PROFILE_BINDING_UNAVAILABLE" && value.BindingID == "implementation"
	}) {
		t.Fatalf("Diagnostics() = %#v", diagnostics)
	}
	diagnostics[0].Code = "changed"
	if result.Diagnostics()[0].Code == "changed" {
		t.Fatal("CompileResult exposed diagnostics storage")
	}
}

func TestCompileSelectedAlternativeUsesExactBinding(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, recipe *catalog.ProfileRecipeRecord) {
		base, _ := testBinding(provider.Bindings, "implementation")
		alternative := base
		alternative.ID = "implementation-alt"
		alternative.ContentRoot = "skills/implementation-alt"
		alternative.InstallRoot = "skills/implementation-alt"
		alternative.Reference = "test:implementation-alt"
		alternative.TreeDigest = "sha256:" + strings.Repeat("d", 64)
		alternative.Alternatives = []string{"implementation"}
		provider.Bindings = append(provider.Bindings, alternative)
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "implementation" {
				provider.Bindings[index].Alternatives = []string{"implementation-alt"}
			}
		}
		provider.Capabilities = append(provider.Capabilities, capabilityFor(alternative))
	})
	fixture.request.Alternatives = []profile.AlternativeChoice{{
		SlotID: catalog.SlotImplementation, StepID: "implementation", AlternativeID: "implementation-alt",
		Selector: catalog.BindingSelector{ProviderID: "test/provider", BindingID: "implementation-alt"},
	}}
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
	slot := requireSlot(t, graph, catalog.SlotImplementation)
	if len(slot.Pipeline) != 1 || slot.Pipeline[0].BindingID != "implementation-alt" || len(graph.Selection.Alternatives) != 1 {
		t.Fatalf("alternative slot = %#v", slot)
	}
}

func TestCompileMacroCreditsInternalBindingAndTraversalSkipsIt(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		parent, _ := testBinding(provider.Bindings, "implementation")
		helper := parent
		helper.ID = "implementation-helper"
		helper.ContentRoot = "skills/implementation-helper"
		helper.InstallRoot = "skills/implementation-helper"
		helper.Reference = "test:implementation-helper"
		helper.Invocation = catalog.InvocationInternal
		helper.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		helper.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipProcedure, Name: "implementation-helper", SlotID: catalog.SlotImplementation, OutcomeOwner: false}}
		provider.Bindings = append(provider.Bindings, helper)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(helper))
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "implementation" {
				provider.Bindings[index].InternalCalls = []catalog.InternalCall{{BindingID: helper.ID, Required: true, Mode: catalog.InternalCreditOnly, StageSpan: []catalog.SlotID{catalog.SlotImplementation}}}
			}
		}
	})
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
	slot := requireSlot(t, graph, catalog.SlotImplementation)
	if len(slot.Pipeline) != 2 || slot.Pipeline[1].Disposition != profile.CreditInternalOnly || len(slot.Traversal) != 2 {
		t.Fatalf("macro slot = %#v", slot)
	}
	unit, err := profile.UnitAtCursor(graph, slot.Traversal[1])
	if err != nil || unit.ProviderBinding == nil || unit.ProviderBinding.Disposition != profile.CreditInternalOnly {
		t.Fatalf("UnitAtCursor(credited) = %#v, %v", unit, err)
	}
}

func TestCompileRequiresLiveDelegationForSelectedTopology(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "implementation" {
				provider.Bindings[index].Delegation.Child = true
			}
		}
	})
	fixture.host = hostEvidence(t, false)
	fixture.request.Host = fixture.host
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := result.Graph(); found || !slices.ContainsFunc(result.Diagnostics(), func(value profile.CompileDiagnostic) bool { return value.Code == "HOST_FEATURE_UNATTESTED" }) {
		t.Fatalf("delegation result = %#v", result.Diagnostics())
	}
}

func TestHumanExplicitBindingCompilesAsPrepareRequirement(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "problem" {
				provider.Bindings[index].Invocation = catalog.InvocationHumanExplicit
			}
		}
	})
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found || !requireSlot(t, graph, catalog.SlotProblemFraming).Pipeline[0].RequiresExplicitInvocation {
		t.Fatalf("explicit invocation result = %#v / %#v", graph, result.Diagnostics())
	}
}

func TestExecutionGraphAndTraversalAccessorsAreImmutable(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, _ := result.Graph()
	digest := graph.Digest
	graph.Slots[0].Pipeline[0].MaximumEffects[0] = "changed"
	graph.Selection.AddOns = append(graph.Selection.AddOns, "changed")
	fresh, _ := result.Graph()
	if fresh.Digest != digest || fresh.Slots[0].Pipeline[0].MaximumEffects[0] == "changed" || len(fresh.Selection.AddOns) != 0 {
		t.Fatal("CompileResult exposed graph storage")
	}
	first, _ := profile.FirstActionableCursor(fresh)
	unit, err := profile.UnitAtCursor(fresh, first)
	if err != nil || unit.ProviderBinding == nil {
		t.Fatalf("UnitAtCursor() = %#v, %v", unit, err)
	}
	unit.ProviderBinding.MaximumEffects[0] = "changed"
	freshUnit, _ := profile.UnitAtCursor(fresh, first)
	if freshUnit.ProviderBinding.MaximumEffects[0] == "changed" {
		t.Fatal("TraversalUnit exposed graph storage")
	}
}

type profileFixture struct {
	catalog  catalog.Catalog
	registry *fakeRegistry
	host     profile.HostEvidence
	request  profile.CompileRequest
}

type fakeRegistry struct {
	hostID       string
	digest       string
	providers    map[string]registry.ProviderInstance
	bindings     map[string]registry.VerifiedBinding
	capabilities map[string]registry.VerifiedCapability
}

func (value *fakeRegistry) HostID() string { return value.hostID }
func (value *fakeRegistry) Digest() string { return value.digest }

func (value *fakeRegistry) Providers() []registry.ProviderInstance {
	result := make([]registry.ProviderInstance, 0, len(value.providers))
	for _, provider := range value.providers {
		result = append(result, provider)
	}
	return result
}

func (value *fakeRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	provider, found := value.providers[id]
	return provider, found
}

func (value *fakeRegistry) Binding(providerID, bindingID string) (registry.VerifiedBinding, bool) {
	binding, found := value.bindings[providerID+"\x00"+bindingID]
	return binding, found
}

func (value *fakeRegistry) Bindings(providerID string) []registry.VerifiedBinding {
	result := []registry.VerifiedBinding{}
	for key, binding := range value.bindings {
		if strings.HasPrefix(key, providerID+"\x00") {
			result = append(result, binding)
		}
	}
	return result
}

func (value *fakeRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	capability, found := value.capabilities[providerID+"\x00"+capabilityID]
	return capability, found
}

func newProfileFixture(t testing.TB, mutate func(*catalog.ProviderDescriptorRecord, *catalog.ProfileRecipeRecord)) profileFixture {
	t.Helper()
	provider := baseProvider()
	recipe := baseRecipe()
	if mutate != nil {
		mutate(&provider, &recipe)
	}
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, []catalog.ProfileRecipeRecord{recipe}, []catalog.ProfileAliasRecord{{Alias: "TEST-FULL", RecipeID: recipe.ID}})
	if err != nil {
		t.Fatalf("catalog.New() error = %v", err)
	}
	provider = value.Providers()[0]
	verified := registryFor(provider)
	evidence := hostEvidence(t, true)
	return profileFixture{
		catalog: value, registry: verified, host: evidence,
		request: profile.CompileRequest{Profile: "TEST-FULL", Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}, Host: evidence},
	}
}

func baseProvider() catalog.ProviderDescriptorRecord {
	specs := []struct {
		id     string
		slot   catalog.SlotID
		input  string
		output string
	}{
		{"problem", catalog.SlotProblemFraming, "request", "framed"},
		{"specification", catalog.SlotSolutionSpecification, "framed", "spec"},
		{"planning", catalog.SlotDeliveryPlanning, "spec", "plan"},
		{"implementation", catalog.SlotImplementation, "workspace", "implementation"},
		{"tdd", catalog.SlotImplementationTDD, "implementation", "tested"},
		{"review", catalog.SlotReviewRemediation, "tested", "reviewed"},
	}
	bindings := make([]catalog.BindingRecord, len(specs))
	capabilities := make([]catalog.CapabilityRecord, len(specs))
	for index, spec := range specs {
		binding := catalog.BindingRecord{
			ID: spec.id, DistributionID: "distribution", ContentRoot: "skills/" + spec.id, InstallRoot: "skills/" + spec.id,
			TreeDigest: "sha256:" + strings.Repeat(string(rune('a'+index)), 64), Host: "codex", Surface: "test-skills", Kind: catalog.BindingSkill,
			Reference: "test:" + spec.id, Invocation: catalog.InvocationModel,
			Responsibilities: []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: spec.id, SlotID: spec.slot, OutcomeOwner: true}},
			InputArtifact:    spec.input, OutputArtifact: spec.output, MaximumEffects: []string{"read-project", "write-project"}, Resources: []string{"project-worktree"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}, Delegation: catalog.DelegationRequirements{},
			StageSpan: []catalog.SlotID{spec.slot}, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}
		bindings[index] = binding
		capabilities[index] = capabilityFor(binding)
	}
	return catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "test/provider", DisplayName: "Test Provider",
		Distributions: []catalog.DistributionRecord{{ID: "distribution", SourceURI: "https://example.test/provider", Revision: strings.Repeat("a", 40), TreeDigest: "sha256:" + strings.Repeat("f", 64)}},
		Discovery:     []catalog.DiscoveryProbe{{ID: "codex", Hosts: []string{"codex"}, Surface: "test-skills", DistributionID: "distribution", Kind: "path-exists", Root: "user-home", CandidatePath: ".test/provider", EvidencePath: "probe"}},
		Bindings:      bindings, Capabilities: capabilities,
	}
}

func capabilityFor(binding catalog.BindingRecord) catalog.CapabilityRecord {
	return catalog.CapabilityRecord{ID: "cap-" + binding.ID, InputSchema: binding.InputArtifact, OutcomeSchema: binding.OutputArtifact, RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{binding.ID}}
}

func baseRecipe() catalog.ProfileRecipeRecord {
	definitions := catalog.CanonicalSlots()
	slots := make([]catalog.SlotRecipe, len(definitions))
	steps := map[catalog.SlotID]catalog.PipelineStep{
		catalog.SlotProblemFraming:        pipelineStep("problem", catalog.SlotProblemFraming, "request", "framed"),
		catalog.SlotSolutionSpecification: pipelineStep("specification", catalog.SlotSolutionSpecification, "framed", "spec"),
		catalog.SlotDeliveryPlanning:      pipelineStep("planning", catalog.SlotDeliveryPlanning, "spec", "plan"),
		catalog.SlotImplementation:        pipelineStep("implementation", catalog.SlotImplementation, "workspace", "implementation"),
		catalog.SlotImplementationTDD:     pipelineStep("tdd", catalog.SlotImplementationTDD, "implementation", "tested"),
		catalog.SlotReviewRemediation:     pipelineStep("review", catalog.SlotReviewRemediation, "tested", "reviewed"),
	}
	next := map[catalog.SlotID]catalog.SlotID{
		catalog.SlotProblemFraming: catalog.SlotSolutionSpecification, catalog.SlotSolutionSpecification: catalog.SlotDeliveryPlanning,
		catalog.SlotDeliveryPlanning: catalog.SlotWorkspacePreparation, catalog.SlotWorkspacePreparation: catalog.SlotImplementation,
		catalog.SlotImplementation: catalog.SlotImplementationTDD, catalog.SlotImplementationTDD: catalog.SlotReviewRemediation,
		catalog.SlotReviewRemediation: catalog.SlotFreshVerification, catalog.SlotFreshVerification: catalog.SlotCloseout,
	}
	for index, definition := range definitions {
		slot := catalog.SlotRecipe{SlotID: definition.ID, Applicability: catalog.SlotMandatory, Pipeline: []catalog.PipelineStep{}, Gates: []catalog.GateRecord{}, Transitions: []catalog.RecipeTransition{}}
		if step, found := steps[definition.ID]; found {
			slot.Pipeline = []catalog.PipelineStep{step}
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: step.ID}
		}
		switch definition.ID {
		case catalog.SlotWorkspacePreparation:
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerHostAction, HostAction: "workspace.prepare-or-confirm"}
			slot.HostAction = &catalog.HostActionRef{ID: "workspace.prepare-or-confirm", InputArtifact: "plan", OutputArtifact: "workspace"}
		case catalog.SlotIncidentRecovery:
			slot.Applicability = catalog.SlotConditional
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerNone}
		case catalog.SlotFreshVerification:
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerHostAction, HostAction: "verification.execute"}
			slot.HostAction = &catalog.HostActionRef{ID: "verification.execute", InputArtifact: "reviewed", OutputArtifact: "verified"}
		case catalog.SlotCloseout:
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerHostAction, HostAction: "closeout.execute"}
			slot.HostAction = &catalog.HostActionRef{ID: "closeout.execute", InputArtifact: "verified", OutputArtifact: "closed"}
			slot.Gates = []catalog.GateRecord{{ID: "user-closeout", Authority: catalog.GateUser, Predicate: "user-authorized", EvidenceRequirements: []catalog.EvidenceRequirementRecord{{Kind: "user-decision", Minimum: 1, Description: "User selects the closeout action."}}}}
		}
		if target, found := next[definition.ID]; found {
			slot.Transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: target}}
		}
		slots[index] = slot
	}
	return catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV3, TaxonomyVersion: catalog.TaxonomyVersionV1, RecipeVersion: "3.0.0", ID: "test/delivery", DisplayName: "Test Delivery", Family: "test",
		Slots: slots, AddOns: []catalog.AddOnRecord{}, IncidentRoutes: []catalog.IncidentRoute{}, Overlays: []catalog.OverlayRecord{}, StableBoundaries: []string{"ticket-complete"}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
}

func pipelineStep(id string, slot catalog.SlotID, input, output string) catalog.PipelineStep {
	return catalog.PipelineStep{ID: id, Selector: catalog.BindingSelector{ProviderID: "test/provider", BindingID: id}, StageSpan: []catalog.SlotID{slot}, RequiredInputArtifact: input, ProducedOutputArtifact: output}
}

func registryFor(provider catalog.ProviderDescriptorRecord) *fakeRegistry {
	bindings := make(map[string]registry.VerifiedBinding, len(provider.Bindings))
	capabilities := make(map[string]registry.VerifiedCapability, len(provider.Capabilities))
	verifiedBindings := make([]registry.VerifiedBinding, len(provider.Bindings))
	for index, binding := range provider.Bindings {
		verified := registry.VerifiedBinding{
			BindingID: binding.ID, DistributionID: binding.DistributionID, DistributionRevision: provider.Distributions[0].Revision, DistributionTreeDigest: provider.Distributions[0].TreeDigest,
			Surface: binding.Surface, Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
			SupportedTopologies: append([]execution.Topology{}, binding.SupportedTopologies...), Delegation: binding.Delegation,
			BindingEvidenceDigest: strings.Repeat(string(rune('1'+index%8)), 64),
		}
		bindings[provider.ID+"\x00"+binding.ID] = verified
		verifiedBindings[index] = verified
	}
	verifiedCapabilities := make([]registry.VerifiedCapability, len(provider.Capabilities))
	for index, capability := range provider.Capabilities {
		value := registry.VerifiedCapability{ID: capability.ID, BindingIDs: append([]string{}, capability.BindingRefs...)}
		capabilities[provider.ID+"\x00"+capability.ID] = value
		verifiedCapabilities[index] = value
	}
	instance := registry.ProviderInstance{
		ProviderID: provider.ID, HostID: "codex", DistributionID: provider.Distributions[0].ID, DistributionRevision: provider.Distributions[0].Revision,
		DistributionTreeDigest: provider.Distributions[0].TreeDigest, Bindings: verifiedBindings, Capabilities: verifiedCapabilities, Digest: strings.Repeat("a", 64),
	}
	return &fakeRegistry{hostID: "codex", digest: strings.Repeat("b", 64), providers: map[string]registry.ProviderInstance{provider.ID: instance}, bindings: bindings, capabilities: capabilities}
}

func hostEvidence(t testing.TB, availableFeatures bool) profile.HostEvidence {
	t.Helper()
	manifest, session, inventory, environment := hostEvidenceRecords(t, availableFeatures)
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func hostEvidenceRecords(t testing.TB, availableFeatures bool) (host.Manifest, host.SessionSnapshot, host.BindingInventory, host.EnvironmentReport) {
	t.Helper()
	actions := []host.HostActionContract{
		{ID: "workspace.prepare-or-confirm", InputSchema: "oaw.host-action.workspace-input/v1", OutcomeSchema: "oaw.host-action.workspace-outcome/v1", MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project-worktree"}},
		{ID: "verification.execute", InputSchema: "oaw.host-action.verification-input/v1", OutcomeSchema: "oaw.host-action.verification-outcome/v1", MaximumEffects: []string{"read-project", "run-process"}, Resources: []string{"project"}},
		{ID: "closeout.execute", InputSchema: "oaw.host-action.closeout-input/v1", OutcomeSchema: "oaw.host-action.closeout-outcome/v1", MaximumEffects: []string{"git-local", "network-mutation", "read-project", "run-process"}, Resources: []string{"git-repository", "network", "project-worktree"}},
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features:           []host.Feature{host.FeatureProviderBindingInventory, host.FeatureNormalizedReceipts, host.FeatureEnvironmentReporting},
		DelegationFeatures: []host.FeatureID{host.FeatureChildDelegation, host.FeatureParallelChildDelegation, host.FeatureNestedChildDelegation, host.FeatureNestedParallelDelegation}, HostActions: actions,
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-current", Topology: execution.TopologyCurrent, Observations: []execution.EnvironmentObservation{}})
	if err != nil {
		t.Fatal(err)
	}
	featureState := host.AvailabilityUnavailable
	if availableFeatures {
		featureState = host.AvailabilityAvailable
	}
	features := make([]host.FeatureObservation, len(manifest.DelegationFeatures))
	for index, feature := range manifest.DelegationFeatures {
		features[index], err = host.NewFeatureObservation(host.FeatureObservation{Feature: feature, State: featureState, Source: host.SourceNativeAPI, EvidenceReference: "evidence://test/features/" + string(feature)})
		if err != nil {
			t.Fatal(err)
		}
	}
	observedActions := make([]host.HostActionObservation, len(manifest.HostActions))
	for index, action := range manifest.HostActions {
		observedActions[index], err = host.NewHostActionObservation(host.HostActionObservation{Action: action, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI, EvidenceReference: "evidence://test/actions/" + action.ID})
		if err != nil {
			t.Fatal(err)
		}
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "test/host", IntegrationVersion: "3.0.0", SessionID: "session-current", ManifestDigest: manifest.Digest,
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}, ProviderInventoryDigest: inventory.Digest,
		FeatureObservations: features, HostActionObservations: observedActions, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manifest, session, inventory, environment
}

func requireSlot(t *testing.T, graph profile.ExecutionGraphRecord, id catalog.SlotID) profile.CompiledSlot {
	t.Helper()
	for _, slot := range graph.Slots {
		if slot.SlotID == id {
			return slot
		}
	}
	t.Fatalf("slot %q not found", id)
	return profile.CompiledSlot{}
}

func testBinding(values []catalog.BindingRecord, id string) (catalog.BindingRecord, bool) {
	for _, value := range values {
		if value.ID == id {
			return value, true
		}
	}
	return catalog.BindingRecord{}, false
}
