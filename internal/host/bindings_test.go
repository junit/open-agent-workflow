package host_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNewBindingInventoryPinsHostInstallationAndEvidence(t *testing.T) {
	observations := []host.BindingObservation{{
		HostID:            "codex",
		InstallationKey:   "installation-acme",
		Binding:           catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"},
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
		Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"},
		Source:  "host-filesystem", EvidenceReference: filepath.Join(t.TempDir(), "SKILL.md"), Digest: strings.Repeat("a", 64),
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
