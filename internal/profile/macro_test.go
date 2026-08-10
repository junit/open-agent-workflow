package profile_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestSDDDispatchesBeforeCreditsAndDispatchesAfterOnce(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		parent, _ := testBinding(provider.Bindings, "implementation")
		for index, spec := range []struct {
			id   string
			mode catalog.InternalCallMode
		}{
			{"before", catalog.InternalDispatchBefore}, {"credited", catalog.InternalCreditOnly}, {"after", catalog.InternalDispatchAfter},
		} {
			child := parent
			child.ID = spec.id
			child.ContentRoot = "skills/" + spec.id
			child.InstallRoot = "skills/" + spec.id
			child.Reference = "test:" + spec.id
			child.TreeDigest = "sha256:" + strings.Repeat(string(rune('a'+index)), 64)
			child.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipProcedure, Name: spec.id, SlotID: catalog.SlotImplementation, OutcomeOwner: false}}
			provider.Bindings = append(provider.Bindings, child)
			provider.Capabilities = append(provider.Capabilities, capabilityFor(child))
		}
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "implementation" {
				provider.Bindings[index].InternalCalls = []catalog.InternalCall{
					{BindingID: "before", Required: true, Mode: catalog.InternalDispatchBefore, StageSpan: []catalog.SlotID{catalog.SlotImplementation}},
					{BindingID: "credited", Required: true, Mode: catalog.InternalCreditOnly, StageSpan: []catalog.SlotID{catalog.SlotImplementation}},
					{BindingID: "after", Required: true, Mode: catalog.InternalDispatchAfter, StageSpan: []catalog.SlotID{catalog.SlotImplementation}},
				}
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
	pipeline := requireSlot(t, graph, catalog.SlotImplementation).Pipeline
	if len(pipeline) != 4 || pipeline[0].BindingID != "before" || pipeline[1].BindingID != "implementation" || pipeline[2].BindingID != "credited" || pipeline[3].BindingID != "after" {
		t.Fatalf("expanded macro order = %#v", pipeline)
	}
	credited := 0
	for _, unit := range pipeline {
		if unit.BindingID == "credited" && unit.Disposition == profile.CreditInternalOnly {
			credited++
		}
	}
	if credited != 1 {
		t.Fatalf("credited units = %d", credited)
	}
}

func TestMacroRejectsCycleMandatoryPausePeerDuplicateAndOwnerConflict(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		parent, _ := testBinding(provider.Bindings, "implementation")
		child := parent
		child.ID = "cycle"
		child.ContentRoot = "skills/cycle"
		child.InstallRoot = "skills/cycle"
		child.Reference = "test:cycle"
		child.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		child.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipProcedure, Name: child.ID, SlotID: catalog.SlotImplementation, OutcomeOwner: false}}
		child.InternalCalls = []catalog.InternalCall{{BindingID: "implementation", Required: true, Mode: catalog.InternalCreditOnly, StageSpan: []catalog.SlotID{catalog.SlotImplementation}}}
		provider.Bindings = append(provider.Bindings, child)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(child))
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "implementation" {
				provider.Bindings[index].InternalCalls = []catalog.InternalCall{{BindingID: child.ID, Required: true, Mode: catalog.InternalCreditOnly, StageSpan: []catalog.SlotID{catalog.SlotImplementation}}}
			}
		}
	})
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := result.Graph(); found || len(result.Diagnostics()) == 0 || result.Diagnostics()[0].Code != "MACRO_INTERNAL_CONFLICT" {
		t.Fatalf("cycle compilation = graph, diagnostics %#v", result.Diagnostics())
	}
}

func TestCompileOptionalInternalUnavailablePersistsOmittedDecision(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		parent, _ := testBinding(provider.Bindings, "implementation")
		optional := parent
		optional.ID = "optional"
		optional.ContentRoot = "skills/optional"
		optional.InstallRoot = "skills/optional"
		optional.Reference = "test:optional"
		optional.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		optional.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipProcedure, Name: optional.ID, SlotID: catalog.SlotImplementation, OutcomeOwner: false}}
		provider.Bindings = append(provider.Bindings, optional)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(optional))
		for index := range provider.Bindings {
			if provider.Bindings[index].ID == "implementation" {
				provider.Bindings[index].InternalCalls = []catalog.InternalCall{{BindingID: optional.ID, Required: false, Mode: catalog.InternalCreditOnly, StageSpan: []catalog.SlotID{catalog.SlotImplementation}}}
			}
		}
	})
	delete(fixture.registry.bindings, "test/provider\x00optional")
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
	for _, decision := range graph.Decisions {
		if decision.ReasonCode == "OPTIONAL_INTERNAL_UNAVAILABLE" && decision.Disposition == profile.OmittedBySelection {
			return
		}
	}
	t.Fatalf("optional Binding omission was not persisted: %#v", graph.Decisions)
}
