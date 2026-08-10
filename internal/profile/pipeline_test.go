package profile_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestCompileOrderedMultiBindingPipeline(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, recipe *catalog.ProfileRecipeRecord) {
		owner, _ := testBinding(provider.Bindings, "implementation")
		prep := owner
		prep.ID = "implementation-prep"
		prep.ContentRoot = "skills/implementation-prep"
		prep.InstallRoot = "skills/implementation-prep"
		prep.Reference = "test:implementation-prep"
		prep.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		prep.OutputArtifact = "workspace-ready"
		prep.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipProcedure, Name: prep.ID, SlotID: catalog.SlotImplementation, OutcomeOwner: false}}
		provider.Bindings = append(provider.Bindings, prep)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(prep))
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "implementation" {
				provider.Bindings[index].InputArtifact = "workspace-ready"
			}
		}
		for index := range recipe.Slots {
			if recipe.Slots[index].SlotID == catalog.SlotImplementation {
				recipe.Slots[index].Pipeline[0].RequiredInputArtifact = "workspace-ready"
				recipe.Slots[index].Pipeline = append([]catalog.PipelineStep{{
					ID: "implementation-prep", Selector: catalog.BindingSelector{ProviderID: provider.ID, BindingID: prep.ID},
					StageSpan: []catalog.SlotID{catalog.SlotImplementation}, RequiredInputArtifact: "workspace", ProducedOutputArtifact: "workspace-ready",
				}}, recipe.Slots[index].Pipeline...)
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
	if len(slot.Pipeline) != 2 || slot.Pipeline[0].BindingID != "implementation-prep" || slot.Pipeline[1].BindingID != "implementation" || slot.OutcomeOwner.UnitID != slot.Pipeline[1].UnitID {
		t.Fatalf("implementation pipeline = %#v", slot)
	}
}

func TestTraversalAnchorsMultiSlotUnitOnce(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, recipe *catalog.ProfileRecipeRecord) {
		for index := range provider.Bindings {
			if provider.Bindings[index].ID != "problem" {
				continue
			}
			provider.Bindings[index].InputArtifact = "spec"
			provider.Bindings[index].OutputArtifact = "spec"
			provider.Bindings[index].StageSpan = []catalog.SlotID{catalog.SlotProblemFraming, catalog.SlotSolutionSpecification}
			provider.Bindings[index].Responsibilities = append(provider.Bindings[index].Responsibilities, catalog.ResponsibilityClaim{Namespace: catalog.OwnershipStage, Name: "solution", SlotID: catalog.SlotSolutionSpecification, OutcomeOwner: true})
		}
		for index := range recipe.Slots {
			slot := &recipe.Slots[index]
			switch slot.SlotID {
			case catalog.SlotProblemFraming:
				slot.Pipeline[0].StageSpan = []catalog.SlotID{catalog.SlotProblemFraming, catalog.SlotSolutionSpecification}
				slot.Pipeline[0].RequiredInputArtifact = "spec"
				slot.Pipeline[0].ProducedOutputArtifact = "spec"
			case catalog.SlotSolutionSpecification:
				slot.Pipeline = []catalog.PipelineStep{{ID: "problem-continuation", Selector: catalog.BindingSelector{ProviderID: provider.ID, BindingID: "problem"}, StageSpan: []catalog.SlotID{catalog.SlotProblemFraming, catalog.SlotSolutionSpecification}, RequiredInputArtifact: "spec", ProducedOutputArtifact: "spec"}}
				slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: "problem-continuation"}
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
	framing := requireSlot(t, graph, catalog.SlotProblemFraming)
	specification := requireSlot(t, graph, catalog.SlotSolutionSpecification)
	if len(framing.Pipeline) != 1 || len(specification.Pipeline) != 0 || len(framing.Pipeline[0].SlotIDs) != 2 || specification.OutcomeOwner.UnitID != framing.Pipeline[0].UnitID {
		t.Fatalf("multi-slot materialization = framing %#v, specification %#v", framing, specification)
	}
	next, err := profile.NextActionableCursor(graph, framing.Pipeline[0].Cursor, "succeeded", "")
	if err != nil || next.Cursor == nil || next.Cursor.SlotID != string(catalog.SlotDeliveryPlanning) {
		t.Fatalf("NextActionableCursor() = %#v, %v", next, err)
	}
}

func TestUnselectedUnavailableAddOnDoesNotFailExactCompilation(t *testing.T) {
	fixture := addOnFixture(t)
	delete(fixture.registry.bindings, "test/provider\x00specialist")
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := result.Graph(); !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
}

func TestSelectedUnavailableAddOnReturnsStableDiagnostic(t *testing.T) {
	fixture := addOnFixture(t)
	delete(fixture.registry.bindings, "test/provider\x00specialist")
	fixture.request.AddOns = []string{"specialist-check"}
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := result.Graph(); found || len(result.Diagnostics()) == 0 || result.Diagnostics()[0].Code != "PROFILE_BINDING_UNAVAILABLE" {
		t.Fatalf("CompileProfile() = graph, diagnostics %#v", result.Diagnostics())
	}
}

func TestIncidentHandlerAddOnRequiresExplicitSelection(t *testing.T) {
	fixture := incidentAddOnFixture(t)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found || len(graph.IncidentRoutes) != 1 || len(graph.IncidentRoutes[0].HandlerPipeline) != 0 {
		t.Fatalf("unselected incident Add-on graph = %#v, diagnostics %#v", graph, result.Diagnostics())
	}
	fixture.request.AddOns = []string{"build-repair"}
	result, err = profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found = result.Graph()
	if !found || len(graph.IncidentRoutes[0].HandlerPipeline) == 0 {
		t.Fatalf("selected incident Add-on graph = %#v, diagnostics %#v", graph, result.Diagnostics())
	}
	delete(fixture.registry.bindings, "test/provider\x00build-repair")
	result, err = profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := result.Graph(); found || len(result.Diagnostics()) == 0 || result.Diagnostics()[0].AddOnID != "build-repair" {
		t.Fatalf("unavailable selected incident Add-on diagnostics = %#v", result.Diagnostics())
	}
}

func addOnFixture(t *testing.T) profileFixture {
	t.Helper()
	return newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, recipe *catalog.ProfileRecipeRecord) {
		base, _ := testBinding(provider.Bindings, "review")
		specialist := base
		specialist.ID = "specialist"
		specialist.ContentRoot = "skills/specialist"
		specialist.InstallRoot = "skills/specialist"
		specialist.Reference = "test:specialist"
		specialist.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		specialist.InputArtifact = "reviewed"
		specialist.OutputArtifact = "reviewed"
		specialist.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipAssurance, Name: specialist.ID, SlotID: catalog.SlotReviewRemediation, OutcomeOwner: false}}
		provider.Bindings = append(provider.Bindings, specialist)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(specialist))
		recipe.AddOns = []catalog.AddOnRecord{{ID: "specialist-check", Kind: catalog.AddOnSpecialistCheck, Selector: catalog.BindingSelector{ProviderID: provider.ID, BindingID: specialist.ID}, SlotID: catalog.SlotReviewRemediation, IncidentTypes: []string{}, EvidenceRequirements: []catalog.EvidenceRequirementRecord{}}}
	})
}

func incidentAddOnFixture(t *testing.T) profileFixture {
	t.Helper()
	return newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, recipe *catalog.ProfileRecipeRecord) {
		base, _ := testBinding(provider.Bindings, "implementation")
		handler := base
		handler.ID = "build-repair"
		handler.ContentRoot = "skills/build-repair"
		handler.InstallRoot = "skills/build-repair"
		handler.Reference = "test:build-repair"
		handler.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		handler.InputArtifact = "incident"
		handler.OutputArtifact = "workspace"
		handler.StageSpan = []catalog.SlotID{catalog.SlotIncidentRecovery}
		handler.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipIncident, Name: "build-failure", SlotID: catalog.SlotIncidentRecovery, OutcomeOwner: false}}
		provider.Bindings = append(provider.Bindings, handler)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(handler))
		recipe.AddOns = []catalog.AddOnRecord{{ID: "build-repair", Kind: catalog.AddOnIncidentHandler, Selector: catalog.BindingSelector{ProviderID: provider.ID, BindingID: handler.ID}, SlotID: catalog.SlotIncidentRecovery, IncidentTypes: []string{"build-failure"}, EvidenceRequirements: []catalog.EvidenceRequirementRecord{}}}
		recipe.IncidentRoutes = []catalog.IncidentRoute{{IncidentType: "build-failure", Handler: catalog.BindingSelector{ProviderID: provider.ID, BindingID: handler.ID}, ReturnTo: catalog.SlotImplementation, IfUnavailable: catalog.IncidentStop}}
	})
}
