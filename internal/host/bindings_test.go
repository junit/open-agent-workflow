package host_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNewBindingInventoryPinsObservedTopologySubset(t *testing.T) {
	for _, observed := range [][]execution.Topology{
		{execution.TopologyCurrent},
		{execution.TopologySubagent},
		{execution.TopologyCurrent, execution.TopologySubagent},
	} {
		t.Run(string(observed[0]), func(t *testing.T) {
			declared := []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
			inputTopologies := append([]execution.Topology{}, observed...)
			inventory, err := host.NewBindingInventory("codex", []host.BindingObservation{{
				HostID:          "codex",
				InstallationKey: "installation-acme",
				Binding: catalog.HostBinding{
					Host: "codex", Kind: "skill", Reference: "acme:review", Topologies: declared,
				},
				Topologies:        inputTopologies,
				Source:            "native-probe",
				EvidenceReference: "evidence://binding/acme-review",
				Digest:            strings.Repeat("a", 64),
			}})
			if err != nil {
				t.Fatalf("NewBindingInventory(%v) error = %v", observed, err)
			}
			if inventory.SchemaVersion != host.BindingInventorySchemaV2 {
				t.Fatalf("SchemaVersion = %q", inventory.SchemaVersion)
			}
			if !slices.Equal(inventory.Observations[0].Topologies, observed) {
				t.Fatalf("observed topologies = %#v, want %#v", inventory.Observations[0].Topologies, observed)
			}
			declared[0] = execution.TopologySubagent
			inputTopologies[0] = execution.Topology("changed")
			cloned := host.CloneBindingInventory(inventory)
			cloned.Observations[0].Binding.Topologies[0] = execution.TopologySubagent
			cloned.Observations[0].Topologies[0] = execution.Topology("changed-again")
			if !slices.Equal(inventory.Observations[0].Binding.Topologies, []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}) || !slices.Equal(inventory.Observations[0].Topologies, observed) {
				t.Fatalf("BindingInventory shares topology storage: %#v", inventory)
			}
		})
	}
}

func TestNewBindingInventoryPinsHostInstallationAndEvidence(t *testing.T) {
	observations := []host.BindingObservation{{
		HostID:            "codex",
		InstallationKey:   "installation-acme",
		Binding:           catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review", Topologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}},
		Topologies:        []execution.Topology{execution.TopologyCurrent},
		Source:            "host-filesystem",
		EvidenceReference: filepath.Join(t.TempDir(), "review", "SKILL.md"),
		Digest:            strings.Repeat("a", 64),
	}}
	first, err := host.NewBindingInventory("codex", observations)
	if err != nil {
		t.Fatal(err)
	}
	second, err := host.NewBindingInventory("codex", append([]host.BindingObservation{}, observations...))
	if err != nil {
		t.Fatal(err)
	}
	if first.HostID != "codex" || first.Digest == "" || first.Digest != second.Digest || len(first.Observations) != 1 {
		t.Fatalf("inventories = %#v / %#v", first, second)
	}
	observations[0].Binding.Reference = "changed"
	if first.Observations[0].Binding.Reference != "acme:review" {
		t.Fatal("BindingInventory shares caller storage")
	}
}

func TestNewBindingInventoryRejectsInvalidObservations(t *testing.T) {
	valid := host.BindingObservation{
		HostID: "codex", InstallationKey: "installation-acme",
		Binding:    catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review", Topologies: []execution.Topology{execution.TopologyCurrent}},
		Topologies: []execution.Topology{execution.TopologyCurrent},
		Source:     "host-filesystem", EvidenceReference: filepath.Join(t.TempDir(), "SKILL.md"), Digest: strings.Repeat("a", 64),
	}
	tests := []struct {
		name        string
		hostID      string
		observation host.BindingObservation
	}{
		{"empty host", "", valid},
		{"invalid host", "Bad Host", valid},
		{"binding host mismatch", "codex", withObservation(valid, func(value *host.BindingObservation) { value.Binding.Host = "claude" })},
		{"empty installation", "codex", withObservation(valid, func(value *host.BindingObservation) { value.InstallationKey = "" })},
		{"unsupported source", "codex", withObservation(valid, func(value *host.BindingObservation) { value.Source = "descriptor" })},
		{"relative evidence", "codex", withObservation(valid, func(value *host.BindingObservation) { value.EvidenceReference = "SKILL.md" })},
		{"invalid digest", "codex", withObservation(valid, func(value *host.BindingObservation) { value.Digest = "bad" })},
		{"empty topology", "codex", withObservation(valid, func(value *host.BindingObservation) { value.Topologies = nil })},
		{"duplicate topology", "codex", withObservation(valid, func(value *host.BindingObservation) {
			value.Topologies = []execution.Topology{execution.TopologyCurrent, execution.TopologyCurrent}
		})},
		{"topology outside Binding", "codex", withObservation(valid, func(value *host.BindingObservation) {
			value.Topologies = []execution.Topology{execution.TopologySubagent}
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := host.NewBindingInventory(tt.hostID, []host.BindingObservation{tt.observation}); err == nil {
				t.Fatal("NewBindingInventory() unexpectedly succeeded")
			}
		})
	}
	if _, err := host.NewBindingInventory("codex", []host.BindingObservation{valid, valid}); err == nil {
		t.Fatal("duplicate observation unexpectedly succeeded")
	}
}

func withObservation(value host.BindingObservation, mutate func(*host.BindingObservation)) host.BindingObservation {
	mutate(&value)
	return value
}
