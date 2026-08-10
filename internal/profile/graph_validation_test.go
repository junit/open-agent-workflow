package profile_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestGraphV4PinsRegistryHostSelectionProviderDecisionsAndTopology(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
	if graph.SchemaVersion != profile.ExecutionGraphSchemaV4 || graph.HostID != fixture.host.Record().HostID || graph.HostEvidenceDigest != fixture.host.Digest() ||
		graph.RegistryDigest != fixture.registry.Digest() || graph.Selection.Digest == "" || graph.Selection.Topology != execution.TopologyCurrent || graph.Topology != execution.TopologyCurrent ||
		len(graph.ProviderInstances) != 1 || graph.ProviderInstances[0].ProviderID != "test/provider" || graph.ProviderInstances[0].InstanceDigest == "" || graph.Decisions == nil {
		t.Fatalf("graph pins = %#v", graph)
	}
}

func TestGraphV4RequiresTenCanonicalSlotsAndOneEntry(t *testing.T) {
	graph := compiledGraph(t)
	if len(graph.Slots) != len(catalog.CanonicalSlots()) || graph.EntrySlotID != catalog.SlotProblemFraming {
		t.Fatalf("graph slots = %#v", graph.Slots)
	}
	changed := graph
	changed.Slots = append([]profile.CompiledSlot{}, graph.Slots[:len(graph.Slots)-1]...)
	changed.Digest = changed.ContentDigest()
	if err := profile.ValidateExecutionGraphRecord(changed); err == nil {
		t.Fatal("ValidateExecutionGraphRecord accepted missing canonical slot")
	}
	changed = graph
	changed.EntrySlotID = catalog.SlotImplementation
	changed.Digest = changed.ContentDigest()
	if err := profile.ValidateExecutionGraphRecord(changed); err == nil {
		t.Fatal("ValidateExecutionGraphRecord accepted a non-entry slot")
	}
}

func TestGraphV4RejectsCursorUnitTraversalOrDigestDrift(t *testing.T) {
	graph := compiledGraph(t)
	tests := []struct {
		name   string
		mutate func(*profile.ExecutionGraphRecord)
	}{
		{"unit cursor", func(value *profile.ExecutionGraphRecord) { value.Slots[0].Pipeline[0].Cursor.Ordinal++ }},
		{"traversal", func(value *profile.ExecutionGraphRecord) { value.Slots[0].Traversal[0].UnitID = "another-unit" }},
		{"owner", func(value *profile.ExecutionGraphRecord) { value.Slots[0].OutcomeOwner.UnitID = "another-unit" }},
		{"terminal", func(value *profile.ExecutionGraphRecord) { value.Slots[len(value.Slots)-1].Terminal = false }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := graph
			changed.Slots = cloneSlotsForTest(graph.Slots)
			test.mutate(&changed)
			changed.Digest = changed.ContentDigest()
			if err := profile.ValidateExecutionGraphRecord(changed); err == nil || !strings.Contains(err.Error(), "PROFILE_GRAPH_RECORD_INVALID") {
				t.Fatalf("ValidateExecutionGraphRecord() error = %v", err)
			}
		})
	}
	changed := graph
	changed.RegistryDigest = strings.Repeat("f", 64)
	if err := profile.ValidateExecutionGraphRecord(changed); err == nil {
		t.Fatal("ValidateExecutionGraphRecord accepted digest drift")
	}
}

func TestGraphRecordAndCompileResultAreDeeplyImmutable(t *testing.T) {
	fixture := newProfileFixture(t, nil)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := result.Graph()
	first.Selection.AddOns = append(first.Selection.AddOns, "changed")
	first.Slots[0].Pipeline[0].Responsibilities[0].Name = "changed"
	first.Slots[3].HostAction.MaximumEffects[0] = "changed"
	first.Slots[9].Gates[0].EvidenceRequirements[0].Description = "changed"
	second, _ := result.Graph()
	if len(second.Selection.AddOns) != 0 || second.Slots[0].Pipeline[0].Responsibilities[0].Name == "changed" ||
		second.Slots[3].HostAction.MaximumEffects[0] == "changed" || second.Slots[9].Gates[0].EvidenceRequirements[0].Description == "changed" {
		t.Fatal("CompileResult.Graph exposed internal storage")
	}
}

func TestSchemaRegistryActivatesOnlyExecutionGraphV4(t *testing.T) {
	registry, err := schema.New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	graph := compiledGraph(t)
	raw, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Validate(schema.ExecutionGraphV4, raw); err != nil {
		t.Fatalf("Validate(ExecutionGraphV4) error = %v", err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["unknown"] = true
	withUnknown, _ := json.Marshal(object)
	if err := registry.Validate(schema.ExecutionGraphV4, withUnknown); err == nil {
		t.Fatal("Execution Graph v4 schema accepted an unknown field")
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v3/execution-graph.schema.json", raw); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("retired Graph v3 error = %v", err)
	}
	v3 := []byte(strings.Replace(string(raw), `"schema_version":"oaw.execution-graph/v4"`, `"schema_version":"oaw.execution-graph/v3"`, 1))
	if err := registry.Validate(schema.ExecutionGraphV4, v3); err == nil {
		t.Fatal("Execution Graph v4 schema accepted Graph v3 wire data")
	}
}

func compiledGraph(t testing.TB) profile.ExecutionGraphRecord {
	t.Helper()
	fixture := newProfileFixture(t, nil)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("Diagnostics() = %#v", result.Diagnostics())
	}
	return graph
}

func cloneSlotsForTest(values []profile.CompiledSlot) []profile.CompiledSlot {
	result := make([]profile.CompiledSlot, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Pipeline = append([]profile.ResolvedBinding{}, value.Pipeline...)
		result[index].Traversal = append([]execution.GraphCursor{}, value.Traversal...)
	}
	return result
}
