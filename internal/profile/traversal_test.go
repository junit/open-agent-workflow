package profile_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestTraversalAssignsStablePerSlotOrdinals(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, _ := result.Graph()
	for _, slot := range graph.Slots {
		for index, cursor := range slot.Traversal {
			if cursor.Ordinal != uint64(index+1) || cursor.SlotID != string(slot.SlotID) {
				t.Fatalf("slot %s traversal = %#v", slot.SlotID, slot.Traversal)
			}
		}
	}
}

func TestTraversalSkipsCreditedAndOmittedBindings(t *testing.T) {
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, _ *catalog.ProfileRecipeRecord) {
		parent, _ := testBinding(provider.Bindings, "implementation")
		for index, suffix := range []string{"before", "credited", "after"} {
			child := parent
			child.ID = suffix
			child.ContentRoot = "skills/" + suffix
			child.InstallRoot = "skills/" + suffix
			child.Reference = "test:" + suffix
			child.TreeDigest = "sha256:" + strings.Repeat(string(rune('d'+index)), 64)
			child.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipProcedure, Name: suffix, SlotID: catalog.SlotImplementation, OutcomeOwner: false}}
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
	slot := requireSlot(t, graph, catalog.SlotImplementation)
	if len(slot.Pipeline) != 4 || len(slot.Traversal) != 4 || slot.Pipeline[2].Disposition != profile.CreditInternalOnly {
		t.Fatalf("macro traversal = %#v", slot)
	}
	next, err := profile.NextActionableCursor(graph, slot.Pipeline[1].Cursor, "", "")
	if err != nil || next.Cursor == nil || *next.Cursor != slot.Pipeline[3].Cursor {
		t.Fatalf("NextActionableCursor() = %#v, %v", next, err)
	}
}

func TestTraversalFollowsExactSignalIncidentReturnAndTerminal(t *testing.T) {
	fixture := incidentFixture(t, true)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
	first, err := profile.FirstActionableCursor(graph)
	if err != nil {
		t.Fatal(err)
	}
	incident, err := profile.NextActionableCursor(graph, first, "", "build-failure")
	if err != nil || incident.Cursor == nil || incident.Cursor.SlotID != string(catalog.SlotIncidentRecovery) {
		t.Fatalf("incident entry = %#v, %v", incident, err)
	}
	returned, err := profile.NextActionableCursor(graph, *incident.Cursor, "succeeded", "")
	if err != nil || returned.Cursor == nil || returned.Cursor.SlotID != string(catalog.SlotImplementation) {
		t.Fatalf("incident return = %#v, %v", returned, err)
	}
	closeout := requireSlot(t, graph, catalog.SlotCloseout)
	afterAction, err := profile.NextActionableCursor(graph, closeout.HostAction.Cursor, "", "")
	if err != nil || afterAction.Cursor == nil || afterAction.Cursor.Kind != execution.CursorGate {
		t.Fatalf("closeout action next = %#v, %v", afterAction, err)
	}
	afterGate, err := profile.NextActionableCursor(graph, *afterAction.Cursor, "", "")
	if err != nil || afterGate.Cursor == nil || afterGate.Cursor.Kind != execution.CursorTerminal {
		t.Fatalf("closeout gate next = %#v, %v", afterGate, err)
	}
	terminal, err := profile.NextActionableCursor(graph, *afterGate.Cursor, "", "")
	if err != nil || terminal.Disposition != profile.TraversalTerminal || terminal.Cursor != nil {
		t.Fatalf("terminal = %#v, %v", terminal, err)
	}
}

func TestTraversalReturnsDistinctStopAndReplanResults(t *testing.T) {
	fixture := incidentFixture(t, false)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
	first, _ := profile.FirstActionableCursor(graph)
	stop, err := profile.NextActionableCursor(graph, first, "", "build-failure")
	if err != nil || stop.Disposition != profile.TraversalStop || stop.Cursor != nil {
		t.Fatalf("stop = %#v, %v", stop, err)
	}
	replan, err := profile.NextActionableCursor(graph, first, "", "test-failure")
	if err != nil || replan.Disposition != profile.TraversalReplan || replan.Cursor != nil {
		t.Fatalf("replan = %#v, %v", replan, err)
	}
}

func TestTraversalRejectsCursorForAnotherGraphUnit(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	result, _ := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	graph, _ := result.Graph()
	first, _ := profile.FirstActionableCursor(graph)
	other, err := execution.NewGraphCursor(first.SlotID, first.Kind, "another-unit", first.Ordinal)
	if err != nil {
		t.Fatal(err)
	}
	if err := profile.ValidateGraphCursor(graph, other); err == nil {
		t.Fatal("ValidateGraphCursor accepted another unit")
	}
	if _, err := profile.NextActionableCursor(graph, first, "succeeded", "build-failure"); err == nil {
		t.Fatal("NextActionableCursor accepted signal and incident together")
	}
}

func incidentFixture(t *testing.T, available bool) profileFixture {
	t.Helper()
	fixture := newProfileFixture(t, func(provider *catalog.ProviderDescriptorRecord, recipe *catalog.ProfileRecipeRecord) {
		base, _ := testBinding(provider.Bindings, "implementation")
		handler := base
		handler.ID = "incident-handler"
		handler.ContentRoot = "skills/incident-handler"
		handler.InstallRoot = "skills/incident-handler"
		handler.Reference = "test:incident-handler"
		handler.TreeDigest = "sha256:" + strings.Repeat("e", 64)
		handler.InputArtifact = "incident"
		handler.OutputArtifact = "workspace"
		handler.StageSpan = []catalog.SlotID{catalog.SlotIncidentRecovery}
		handler.Responsibilities = []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipIncident, Name: "build-failure", SlotID: catalog.SlotIncidentRecovery, OutcomeOwner: false}}
		provider.Bindings = append(provider.Bindings, handler)
		provider.Capabilities = append(provider.Capabilities, capabilityFor(handler))
		recipe.IncidentRoutes = []catalog.IncidentRoute{
			{IncidentType: "build-failure", Handler: catalog.BindingSelector{ProviderID: provider.ID, BindingID: handler.ID}, ReturnTo: catalog.SlotImplementation, IfUnavailable: catalog.IncidentStop},
			{IncidentType: "test-failure", Handler: catalog.BindingSelector{ProviderID: provider.ID, BindingID: handler.ID}, ReturnTo: catalog.SlotImplementationTDD, IfUnavailable: catalog.IncidentReplan},
		}
	})
	if !available {
		delete(fixture.registry.bindings, "test/provider\x00incident-handler")
	}
	return fixture
}
