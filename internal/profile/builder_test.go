package profile_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestBuilderStartsFromCanonicalAndClonesRecipe(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	draft, err := profile.NewRecipe("user/custom", "3.0.0")
	if err != nil || draft.Family != "user-defined" || len(draft.Slots) != len(catalog.CanonicalSlots()) {
		t.Fatalf("NewRecipe() = %#v, %v", draft, err)
	}
	clone, err := profile.CloneRecipe(fixture.catalog, "test/delivery", "user/clone", "3.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if clone.ID != "user/clone" || clone.RecipeVersion != "3.1.0" || clone.Family != "test" {
		t.Fatalf("CloneRecipe() = %#v", clone)
	}
	original := fixture.catalog.Recipes()[0]
	clone.Slots[0].Pipeline[0].ID = "changed"
	if fixture.catalog.Recipes()[0].Slots[0].Pipeline[0].ID != original.Slots[0].Pipeline[0].ID {
		t.Fatal("CloneRecipe mutated source catalog")
	}
}

func TestBuilderClonePinsOriginalRecipeBase(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	base := fixture.catalog.Recipes()[0]
	clone, err := profile.CloneRecipe(fixture.catalog, base.ID, "user/clone", "3.1.0")
	if err != nil {
		t.Fatal(err)
	}
	_, baseDigest, err := catalog.NormalizeAndDigestRecipe(fixture.catalog.Providers(), base)
	if err != nil {
		t.Fatal(err)
	}
	request := profile.BuilderSelectionRequest{Profile: clone.ID, Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, clone, profile.BuilderBaseRecipe, base.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if projection.BaseRecipeID != base.ID || projection.BaseDigest != baseDigest {
		t.Fatalf("clone base provenance = id %q digest %q, want %q %q", projection.BaseRecipeID, projection.BaseDigest, base.ID, baseDigest)
	}
}

func TestBuilderProjectsAndConfirmsExactGraph(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	request := profile.BuilderSelectionRequest{Profile: "test/delivery", Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], profile.BuilderBaseRecipe, "test/delivery", request)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.ValidateBuilderProjection(projection); err != nil {
		t.Fatal(err)
	}
	if projection.PreviewGraph == nil || projection.ConfirmationDigest == "" || projection.Selection == nil || len(projection.Slots) != len(catalog.CanonicalSlots()) {
		t.Fatalf("projection = %#v", projection)
	}
	confirmed, err := profile.ConfirmRecipe(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], request, projection, projection.ConfirmationDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.ValidateConfirmedRecipe(confirmed); err != nil {
		t.Fatal(err)
	}
	if confirmed.Graph.Digest != projection.PreviewGraphDigest || confirmed.Selection.Digest != projection.SelectionDigest {
		t.Fatalf("confirmed pins = %#v", confirmed)
	}
}

func TestBuilderShowsUnavailableTrustedBindingAsCandidate(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	delete(fixture.registry.bindings, "test/provider\x00implementation")
	request := profile.BuilderSelectionRequest{Profile: "test/delivery", Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], profile.BuilderBaseRecipe, "test/delivery", request)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, slot := range projection.Slots {
		for _, candidate := range slot.Candidates {
			if candidate.BindingID == "implementation" {
				found = true
				if candidate.Compatible || len(candidate.Diagnostics) == 0 {
					t.Fatalf("candidate = %#v", candidate)
				}
			}
		}
	}
	if !found {
		t.Fatal("Builder omitted trusted unavailable Binding candidate")
	}
}

func TestBuilderRejectsStaleConfirmationAndCopiesProjection(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	request := profile.BuilderSelectionRequest{Profile: "test/delivery", Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], profile.BuilderBaseRecipe, "test/delivery", request)
	if err != nil {
		t.Fatal(err)
	}
	copy := profile.CloneBuilderProjection(projection)
	copy.Slots[0].Candidates[0].BindingID = "changed"
	if profile.CloneBuilderProjection(projection).Slots[0].Candidates[0].BindingID == "changed" {
		t.Fatal("CloneBuilderProjection exposed storage")
	}
	request.Topology = execution.TopologySubagent
	if _, err := profile.ConfirmRecipe(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], request, projection, projection.ConfirmationDigest); err == nil {
		t.Fatal("ConfirmRecipe accepted topology drift")
	}
}

func TestBuilderStartsFromCanonicalLifecycle(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	draft, err := profile.NewRecipe("user/canonical", "3.0.0")
	if err != nil {
		t.Fatal(err)
	}
	request := profile.BuilderSelectionRequest{Profile: draft.ID, Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, draft, profile.BuilderBaseCanonical, "", request)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.ValidateBuilderProjection(projection); err != nil {
		t.Fatal(err)
	}
	if projection.BaseRecipeID != "" || projection.Selection != nil || projection.PreviewGraph != nil || projection.ConfirmationDigest != "" || len(projection.Diagnostics) != 1 || projection.Diagnostics[0].Code != "PROFILE_DRAFT_INCOMPLETE" {
		t.Fatalf("canonical draft projection = %#v", projection)
	}
}

func TestBuilderListsTrustedCandidatesAndMarksExactVerifiedCompatibility(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		base, _ := testBinding(provider.Bindings, "implementation")
		candidate := base
		candidate.ID = "implementation-candidate"
		candidate.ContentRoot = "skills/implementation-candidate"
		candidate.InstallRoot = "skills/implementation-candidate"
		candidate.Reference = "test:implementation-candidate"
		candidate.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		candidate.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: candidate.ID, SlotID: catalog.SlotImplementation, OutcomeOwner: true}}
		provider.Bindings = append(provider.Bindings, candidate)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(candidate))
	})
	request := profile.BuilderSelectionRequest{Profile: "test/delivery", Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], profile.BuilderBaseRecipe, "test/delivery", request)
	if err != nil {
		t.Fatal(err)
	}
	slot := projection.Slots[4]
	if len(slot.Candidates) != 2 {
		t.Fatalf("implementation candidates = %#v", slot.Candidates)
	}
	for _, candidate := range slot.Candidates {
		if !candidate.Compatible || len(candidate.Diagnostics) != 0 || candidate.BindingEvidenceDigest == "" || len(candidate.MaximumEffects) == 0 || len(candidate.Resources) == 0 {
			t.Fatalf("candidate = %#v", candidate)
		}
	}
}

func TestBuilderEditsOrderedPipelineAndOwnerImmutably(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	original := fixture.catalog.Recipes()[0]
	step := original.Slots[4].Pipeline[0]
	edited, err := profile.EditRecipe(original, []profile.RecipeEdit{{SlotID: catalog.SlotImplementation, Pipeline: []catalog.PipelineStep{step}, OutcomeOwner: catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: step.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	edited.Slots[4].Pipeline[0].StageSpan[0] = catalog.SlotCloseout
	if original.Slots[4].Pipeline[0].StageSpan[0] != catalog.SlotImplementation {
		t.Fatal("EditRecipe mutated the source Recipe")
	}
	if _, err := profile.EditRecipe(original, []profile.RecipeEdit{{SlotID: catalog.SlotImplementation}, {SlotID: catalog.SlotImplementation}}); err == nil {
		t.Fatal("EditRecipe accepted duplicate slot edits")
	}
}

func TestBuilderRejectsMissingOwnerAsIncompleteDraft(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	recipe := fixture.catalog.Recipes()[0]
	recipe.Slots[4].OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerNone}
	request := profile.BuilderSelectionRequest{Profile: recipe.ID, Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, recipe, profile.BuilderBaseRecipe, recipe.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Selection != nil || len(projection.Diagnostics) != 1 || projection.Diagnostics[0].Code != "PROFILE_DRAFT_INCOMPLETE" {
		t.Fatalf("incomplete owner projection = %#v", projection)
	}
}

func TestBuilderRequiresExactReturnedConfirmationDigest(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	request := profile.BuilderSelectionRequest{Profile: "test/delivery", Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], profile.BuilderBaseRecipe, "test/delivery", request)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"", strings.Repeat("f", 64)} {
		if _, err := profile.ConfirmRecipe(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], request, projection, expected); err == nil {
			t.Fatalf("ConfirmRecipe accepted confirmation digest %q", expected)
		}
	}
}

func TestBuilderProjectionAndConfirmedRecipeAreDeeplyImmutable(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	request := profile.BuilderSelectionRequest{Profile: "test/delivery", Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}}
	projection, err := profile.BuildProjection(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], profile.BuilderBaseRecipe, "test/delivery", request)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := profile.ConfirmRecipe(fixture.catalog, fixture.registry, fixture.host, fixture.catalog.Recipes()[0], request, projection, projection.ConfirmationDigest)
	if err != nil {
		t.Fatal(err)
	}
	copy := profile.CloneConfirmedRecipe(confirmed)
	copy.Recipe.Slots[9].Gates[0].EvidenceRequirements[0].Description = "changed"
	copy.Graph.Slots[3].HostAction.MaximumEffects[0] = "changed"
	copy.ProviderInstances[0].ProviderID = "changed/provider"
	second := profile.CloneConfirmedRecipe(confirmed)
	if second.Recipe.Slots[9].Gates[0].EvidenceRequirements[0].Description == "changed" || second.Graph.Slots[3].HostAction.MaximumEffects[0] == "changed" || second.ProviderInstances[0].ProviderID == "changed/provider" {
		t.Fatal("CloneConfirmedRecipe exposed nested storage")
	}
	if err := profile.ValidateConfirmedRecipe(second); err != nil {
		t.Fatal(err)
	}
}
