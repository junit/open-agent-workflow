package host_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestBindingInventoryV3PinsObservedTopologyAndCallerStorage(t *testing.T) {
	inputTopologies := []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	observation, err := host.NewBindingObservation(host.BindingObservation{
		HostID: "codex", ProviderID: "oaw/provider", InstallationKey: "installation-provider",
		DistributionID: "distribution", BindingID: "binding-skill", Surface: "codex",
		Kind: catalog.BindingSkill, Reference: "provider:skill", Invocation: catalog.InvocationModel,
		BindingTreeDigest: "sha256:" + strings.Repeat("a", 64), Topologies: inputTopologies,
		Source: host.SourceNativeAPI, EvidenceReference: "evidence://codex/bindings/skill",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", []host.BindingObservation{observation})
	if err != nil {
		t.Fatal(err)
	}
	inputTopologies[0] = execution.TopologySubagent
	clone := host.CloneBindingInventory(inventory)
	clone.Observations[0].Topologies[0] = execution.TopologySubagent
	if !slices.Equal(inventory.Observations[0].Topologies, []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}) {
		t.Fatalf("BindingInventory shares topology storage: %#v", inventory)
	}
}

func TestBindingInventoryV3AcceptsFiveExactBindingKinds(t *testing.T) {
	kinds := allBindingKindsV3()
	observations := make([]host.BindingObservation, len(kinds))
	for index, kind := range kinds {
		observation, err := host.NewBindingObservation(host.BindingObservation{
			HostID: "codex", ProviderID: "oaw/provider", InstallationKey: "installation-provider",
			DistributionID: "distribution", BindingID: "binding-" + string(kind), Surface: "codex",
			Kind: kind, Reference: "provider:" + string(kind), Invocation: catalog.InvocationModel,
			BindingTreeDigest: "sha256:" + strings.Repeat(string(rune('a'+index)), 64),
			Topologies:        []execution.Topology{execution.TopologyCurrent}, Source: host.SourceLiveFilesystem,
			EvidenceReference: "evidence://codex/bindings/" + string(kind),
		})
		if err != nil {
			t.Fatalf("NewBindingObservation(%s) error = %v", kind, err)
		}
		observations[index] = observation
	}
	inventory, err := host.BuildBindingInventoryV3("codex", observations)
	if err != nil {
		t.Fatalf("BuildBindingInventoryV3() error = %v", err)
	}
	if inventory.SchemaVersion != host.BindingInventorySchemaV3 || inventory.Digest == "" || len(inventory.Observations) != len(kinds) {
		t.Fatalf("BindingInventory = %#v", inventory)
	}
	if normalized, err := host.ValidateBindingInventory(inventory); err != nil || normalized.Digest != inventory.Digest {
		t.Fatalf("ValidateBindingInventory() = %#v, %v", normalized, err)
	}

	changed := host.CloneBindingInventory(inventory)
	changed.Digest = ""
	changed.Observations[0].Digest = ""
	changed.Observations[0].BindingTreeDigest = "sha256:" + strings.Repeat("f", 64)
	changed, err = host.BuildBindingInventoryV3("codex", changed.Observations)
	if err != nil || changed.Digest == inventory.Digest || changed.Observations[0].Digest == inventory.Observations[0].Digest {
		t.Fatalf("Binding tree did not bind inventory digest: %#v / %#v / %v", inventory, changed, err)
	}
}

func TestBindingObservationV3AcceptsLogicalSlashInstructionReference(t *testing.T) {
	base := host.BindingObservation{
		HostID: "codex", ProviderID: "oaw/ecc", InstallationKey: "installation-ecc", DistributionID: "ecc",
		BindingID: "codex-feature-dev", Surface: "codex-plugin", Kind: catalog.BindingInstruction, Reference: "/feature-dev",
		Invocation: catalog.InvocationModel, BindingTreeDigest: "sha256:" + strings.Repeat("a", 64),
		Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/instructions/feature-dev",
	}
	if _, err := host.NewBindingObservation(base); err != nil {
		t.Fatalf("logical slash instruction was rejected: %v", err)
	}
	base.Kind = catalog.BindingSkill
	if _, err := host.NewBindingObservation(base); err == nil {
		t.Fatal("absolute-looking skill reference was accepted")
	}
	base.Kind = catalog.BindingInstruction
	base.Reference = "/commands/feature-dev"
	if _, err := host.NewBindingObservation(base); err == nil {
		t.Fatal("multi-segment absolute instruction reference was accepted")
	}
}

func TestBindingInventoryV3RejectsInvalidIdentityEvidenceAndDuplicates(t *testing.T) {
	valid := host.BindingObservation{
		HostID: "codex", ProviderID: "oaw/provider", InstallationKey: "installation-provider",
		DistributionID: "distribution", BindingID: "binding-skill", Surface: "codex",
		Kind: catalog.BindingSkill, Reference: "provider:skill", Invocation: catalog.InvocationModel,
		BindingTreeDigest: "sha256:" + strings.Repeat("a", 64), Topologies: []execution.Topology{execution.TopologyCurrent},
		Source: host.SourceLiveFilesystem, EvidenceReference: "evidence://codex/bindings/skill",
	}
	for _, test := range []struct {
		name   string
		mutate func(*host.BindingObservation)
	}{
		{"host", func(value *host.BindingObservation) { value.HostID = "Bad Host" }},
		{"provider", func(value *host.BindingObservation) { value.ProviderID = "other" }},
		{"distribution", func(value *host.BindingObservation) { value.DistributionID = "Bad" }},
		{"binding", func(value *host.BindingObservation) { value.BindingID = "Bad" }},
		{"hook", func(value *host.BindingObservation) { value.Kind = catalog.BindingKind("hook") }},
		{"surface", func(value *host.BindingObservation) { value.Surface = "/private/surface" }},
		{"reference", func(value *host.BindingObservation) { value.Reference = "/private/skill" }},
		{"invocation", func(value *host.BindingObservation) { value.Invocation = "automatic" }},
		{"static source", func(value *host.BindingObservation) { value.Source = host.SourceStaticConfig }},
		{"absolute evidence", func(value *host.BindingObservation) { value.EvidenceReference = "/private/skills/SKILL.md" }},
		{"tree digest", func(value *host.BindingObservation) { value.BindingTreeDigest = strings.Repeat("a", 64) }},
		{"empty topology", func(value *host.BindingObservation) { value.Topologies = []execution.Topology{} }},
		{"duplicate topology", func(value *host.BindingObservation) {
			value.Topologies = []execution.Topology{execution.TopologyCurrent, execution.TopologyCurrent}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if _, err := host.NewBindingObservation(input); host.ErrorCode(err) != "HOST_BINDING_OBSERVATION_INVALID" {
				t.Fatalf("NewBindingObservation() error = %v", err)
			}
		})
	}

	first, err := host.NewBindingObservation(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.BuildBindingInventoryV3("codex", []host.BindingObservation{first, first}); host.ErrorCode(err) != "HOST_BINDING_INVENTORY_INVALID" {
		t.Fatalf("duplicate inventory error = %v", err)
	}
	if _, err := host.ValidateBindingInventory(host.BindingInventory{SchemaVersion: "oaw.host-binding-inventory/v2"}); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("v2 inventory error = %v", err)
	}
}
