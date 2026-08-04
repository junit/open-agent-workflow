package codex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host/codex"
)

func TestObserveBindingsRequiresPhysicalCodexEvidence(t *testing.T) {
	fixture := newCodexInventoryFixture(t, "skill", "acme:review")
	writeInventoryFile(t, fixture.Home, ".codex/plugins/acme/skills/review/SKILL.md", "---\nname: review\n---\n")
	inventory, err := codex.ObserveBindings(
		fixture.Catalog,
		fixture.Discovery,
		codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot},
	)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.HostID != "codex" || len(inventory.Observations) != 1 {
		t.Fatalf("inventory = %#v", inventory)
	}
	observation := inventory.Observations[0]
	if observation.InstallationKey != fixture.InstallationKey ||
		observation.Binding != (catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:review"}) ||
		observation.Source != "host-filesystem" ||
		observation.Digest == "" {
		t.Fatalf("observation = %#v", observation)
	}
	second, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot})
	if err != nil || second.Digest != inventory.Digest {
		t.Fatalf("repeated inventory = %#v, %v", second, err)
	}
}

func TestObserveBindingsDoesNotTrustDescriptorDeclarations(t *testing.T) {
	fixture := newCodexInventoryFixture(t, "skill", "acme:review")
	inventory, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot})
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Observations) != 0 {
		t.Fatalf("inventory = %#v", inventory)
	}
	writeInventoryFile(t, fixture.Home, ".codex/plugins/other/skills/review/SKILL.md", "---\nname: review\n---\n")
	inventory, err = codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot})
	if err != nil || len(inventory.Observations) != 0 {
		t.Fatalf("outside Candidate inventory = %#v, %v", inventory, err)
	}
}

func TestObserveBindingsRejectsUnsafeSkillEvidence(t *testing.T) {
	t.Run("symlink escape", func(t *testing.T) {
		fixture := newCodexInventoryFixture(t, "skill", "acme:review")
		outside := writeInventoryFile(t, t.TempDir(), "review/SKILL.md", "---\nname: review\n---\n")
		link := filepath.Join(fixture.Home, ".codex", "plugins", "acme", "skills", "review")
		if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Dir(outside), link); err != nil {
			t.Fatal(err)
		}
		if _, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot}); err == nil || !strings.Contains(err.Error(), "HOST_BINDING_EVIDENCE_INVALID") {
			t.Fatalf("ObserveBindings() error = %v", err)
		}
	})

	t.Run("oversized", func(t *testing.T) {
		fixture := newCodexInventoryFixture(t, "skill", "acme:review")
		writeInventoryFile(t, fixture.Home, ".codex/plugins/acme/skills/review/SKILL.md", "---\nname: review\n---\n")
		if _, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot, MaximumEvidenceBytes: 8}); err == nil || !strings.Contains(err.Error(), "HOST_BINDING_EVIDENCE_TOO_LARGE") {
			t.Fatalf("ObserveBindings() error = %v", err)
		}
	})
}

func TestObserveBindingsRequiresRegisteredContainedAgent(t *testing.T) {
	t.Run("registered agent", func(t *testing.T) {
		fixture := newCodexInventoryFixture(t, "agent", "planner")
		agentPath := writeInventoryFile(t, fixture.Home, ".codex/plugins/acme/agents/planner.toml", "model = \"gpt-5\"\n")
		writeInventoryFile(t, fixture.CodexRoot, "config.toml", "[agents.planner]\nconfig_file = "+quotedTOML(agentPath)+"\n")
		inventory, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot})
		if err != nil || len(inventory.Observations) != 1 {
			t.Fatalf("inventory = %#v, %v", inventory, err)
		}
	})

	t.Run("outside candidate", func(t *testing.T) {
		fixture := newCodexInventoryFixture(t, "agent", "planner")
		agentPath := writeInventoryFile(t, t.TempDir(), "planner.toml", "model = \"gpt-5\"\n")
		writeInventoryFile(t, fixture.CodexRoot, "config.toml", "[agents.planner]\nconfig_file = "+quotedTOML(agentPath)+"\n")
		if _, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot}); err == nil || !strings.Contains(err.Error(), "HOST_BINDING_EVIDENCE_INVALID") {
			t.Fatalf("ObserveBindings() error = %v", err)
		}
	})

	t.Run("malformed registry", func(t *testing.T) {
		fixture := newCodexInventoryFixture(t, "agent", "planner")
		writeInventoryFile(t, fixture.CodexRoot, "config.toml", "[agents.planner]\nconfig_file = 42\n")
		if _, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot}); err == nil || !strings.Contains(err.Error(), "HOST_BINDING_REGISTRY_INVALID") {
			t.Fatalf("ObserveBindings() error = %v", err)
		}
	})
}

func TestObserveBindingsDoesNotInventToolEvidence(t *testing.T) {
	fixture := newCodexInventoryFixture(t, "tool", "acme:verify")
	inventory, err := codex.ObserveBindings(fixture.Catalog, fixture.Discovery, codex.InventoryOptions{UserHome: fixture.Home, CodexConfigRoot: fixture.CodexRoot})
	if err != nil || len(inventory.Observations) != 0 {
		t.Fatalf("inventory = %#v, %v", inventory, err)
	}
}

type codexInventoryFixture struct {
	Home            string
	CodexRoot       string
	Catalog         catalog.Catalog
	Discovery       discovery.Report
	InstallationKey string
}

func newCodexInventoryFixture(t *testing.T, kind, reference string) codexInventoryFixture {
	t.Helper()
	home := t.TempDir()
	codexRoot := filepath.Join(home, ".codex")
	writeInventoryFile(t, home, ".codex/plugins/acme/marker.txt", "acme")
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV2, DescriptorVersion: "2.0.0", ID: "acme/suite", DisplayName: "Acme Suite",
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", Distribution: "acme",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/plugins/acme", EvidencePath: "marker.txt",
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "review", InputSchema: "in", OutcomeSchema: "out", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			RequestModes: []catalog.RequestMode{catalog.RequestModeBounded}, Responsibilities: []string{"review"}, ExecutorTopology: catalog.IsolatedRequired,
			DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{{Host: "codex", Kind: kind, Reference: reference}},
		}},
	}
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor}, []catalog.ProfileRecipeRecord{}, []catalog.ProfileAliasRecord{})
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	candidates := report.Candidates("acme/suite")
	if len(candidates) != 1 {
		t.Fatalf("Candidates() = %#v", candidates)
	}
	return codexInventoryFixture{Home: home, CodexRoot: codexRoot, Catalog: value, Discovery: report, InstallationKey: candidates[0].InstallationKey}
}

func writeInventoryFile(t *testing.T, root, relative, content string) string {
	t.Helper()
	path := relative
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(relative))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func quotedTOML(value string) string {
	return `"` + strings.ReplaceAll(value, `\`, `\\`) + `"`
}
