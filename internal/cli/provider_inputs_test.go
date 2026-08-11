package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestProviderInputsRejectsStaleHostInventorySchema(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	stale := host.CloneBindingInventory(fixture.Inventory)
	stale.SchemaVersion = "oaw.host-binding-inventory/v2"
	if _, err := loadProviderInputs(providerInputOptions{HostID: "codex", UserHome: fixture.Home, Inventory: &stale}); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadProviderInputsUsesHostBindingInventory(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	configRoot := filepath.Join(t.TempDir(), "open-agent-workflow")
	providerRoot := filepath.Join(configRoot, "providers")
	if err := os.MkdirAll(providerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	descriptor, err := json.Marshal(fixture.Catalog.Providers()[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providerRoot, "acme.json"), descriptor, 0o600); err != nil {
		t.Fatal(err)
	}
	configContents := "schema_version = \"oaw.user-config/v3\"\n" +
		"[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.json\"\n"
	if err := os.WriteFile(filepath.Join(configRoot, "config.toml"), []byte(configContents), 0o600); err != nil {
		t.Fatal(err)
	}

	inputs, err := loadProviderInputs(providerInputOptions{
		HostID: "codex", UserConfigRoot: configRoot, UserHome: fixture.Home, Inventory: &fixture.Inventory,
	})
	if err != nil {
		t.Fatal(err)
	}
	resolution, ok := inputs.Resolutions.Resolution("acme/suite")
	if !ok || resolution.State != registry.ProviderVerified {
		t.Fatalf("resolution = %#v, found=%v", resolution, ok)
	}
	if inputs.Inventory == nil || inputs.Inventory == &fixture.Inventory || len(inputs.Inventory.Observations) == 0 {
		t.Fatalf("inventory was not defensively cloned: %#v", inputs.Inventory)
	}
	fixture.Inventory.Observations[0].Topologies = []execution.Topology{execution.TopologySubagent}
	if got := inputs.Inventory.Observations[0].Topologies; len(got) != 1 || got[0] != execution.TopologyCurrent {
		t.Fatalf("cloned inventory changed with caller input: %#v", got)
	}
}

func TestLoadProviderInputsSeparatesSelectedHostAuthorityFromForeignDiagnostics(t *testing.T) {
	userHome := t.TempDir()
	writeProviderInputMarker(t, userHome, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "codex-superpowers")
	writeProviderInputMarker(t, userHome, ".claude/plugins/superpowers/skills/using-superpowers/SKILL.md", "claude-superpowers")
	base := providerInputOptions{
		HostID: "codex", ProjectRoot: t.TempDir(), UserConfigRoot: filepath.Join(t.TempDir(), "open-agent-workflow"), UserHome: userHome,
	}

	currentOnly, err := loadProviderInputs(base)
	if err != nil {
		t.Fatalf("loadProviderInputs(current) error = %v", err)
	}
	withForeignOptions := base
	withForeignOptions.IncludeForeignDiagnostics = true
	withForeign, err := loadProviderInputs(withForeignOptions)
	if err != nil {
		t.Fatalf("loadProviderInputs(foreign) error = %v", err)
	}
	if withForeign.HostID != "codex" || withForeign.Discovery.HostID() != "codex" || withForeign.Registry.HostID() != "codex" || len(withForeign.Foreign) != 1 || withForeign.Foreign[0].HostID != "claude" {
		t.Fatalf("provider inputs = %#v", withForeign)
	}
	for _, candidate := range withForeign.Discovery.Candidates("oaw/superpowers") {
		if candidate.HostID != "codex" || strings.Contains(candidate.DiagnosticLocation, ".claude") {
			t.Fatalf("current Candidate = %#v", candidate)
		}
	}
	foreignCandidates := withForeign.Foreign[0].Discovery.Candidates("oaw/superpowers")
	if len(foreignCandidates) != 1 || foreignCandidates[0].HostID != "claude" || !strings.Contains(foreignCandidates[0].DiagnosticLocation, ".claude") {
		t.Fatalf("foreign Candidates = %#v", foreignCandidates)
	}
	if len(currentOnly.Foreign) != 0 || currentOnly.Resolutions.Digest() != withForeign.Resolutions.Digest() || currentOnly.Registry.Digest() != withForeign.Registry.Digest() {
		t.Fatalf("foreign diagnostics changed authority: current=%#v foreign=%#v", currentOnly, withForeign)
	}

	claudeOptions := base
	claudeOptions.HostID = "claude"
	claude, err := loadProviderInputs(claudeOptions)
	if err != nil {
		t.Fatalf("loadProviderInputs(claude) error = %v", err)
	}
	if claude.HostID != "claude" || claude.Discovery.HostID() != "claude" || claude.Registry.HostID() != "claude" {
		t.Fatalf("policy-only inputs = %#v", claude)
	}
	if _, found := claude.Registry.Provider("oaw/superpowers"); found {
		t.Fatalf("policy-only Host produced verified Registry: %#v", claude.Registry)
	}
	resolution, found := claude.Resolutions.Resolution("oaw/superpowers")
	if !found || resolution.State != registry.ProviderCandidate || resolution.Reason != "HOST_BINDING_EVIDENCE_REQUIRED" || len(resolution.Candidates) != 1 {
		t.Fatalf("policy-only resolution = %#v, found=%v", resolution, found)
	}
}

func TestLoadProviderInputsScopesConfiguredInstallationsToTheirHost(t *testing.T) {
	userHome := t.TempDir()
	configRoot := filepath.Join(t.TempDir(), "open-agent-workflow")
	codexInstallation := filepath.Join(t.TempDir(), "codex-superpowers")
	claudeInstallation := filepath.Join(t.TempDir(), "claude-superpowers")
	writeProviderInputMarker(t, codexInstallation, "skills/using-superpowers/SKILL.md", "codex-superpowers")
	writeProviderInputMarker(t, claudeInstallation, "skills/using-superpowers/SKILL.md", "claude-superpowers")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	configContents := "schema_version = \"oaw.user-config/v3\"\n\n" +
		"[[provider_installations]]\nprovider_id = \"oaw/superpowers\"\nhost_id = \"codex\"\nsurface_id = \"codex-plugin\"\nlocation = \"" + codexInstallation + "\"\ndiscovery_probe_id = \"sp-codex-direct\"\n\n" +
		"[[provider_installations]]\nprovider_id = \"oaw/superpowers\"\nhost_id = \"claude\"\nsurface_id = \"claude-plugin\"\nlocation = \"" + claudeInstallation + "\"\ndiscovery_probe_id = \"sp-claude-direct\"\n"
	if err := os.WriteFile(filepath.Join(configRoot, "config.toml"), []byte(configContents), 0o600); err != nil {
		t.Fatal(err)
	}
	inputs, err := loadProviderInputs(providerInputOptions{
		HostID: "codex", ProjectRoot: t.TempDir(), UserConfigRoot: configRoot, UserHome: userHome, IncludeForeignDiagnostics: true,
	})
	if err != nil {
		t.Fatalf("loadProviderInputs() error = %v", err)
	}
	physicalCodex, err := filepath.EvalSymlinks(codexInstallation)
	if err != nil {
		t.Fatal(err)
	}
	physicalClaude, err := filepath.EvalSymlinks(claudeInstallation)
	if err != nil {
		t.Fatal(err)
	}
	current := inputs.Discovery.Candidates("oaw/superpowers")
	if len(current) != 1 || current[0].HostID != "codex" || current[0].DiagnosticLocation != physicalCodex {
		t.Fatalf("selected Host candidates = %#v", current)
	}
	if len(inputs.Foreign) != 1 || inputs.Foreign[0].HostID != "claude" {
		t.Fatalf("foreign reports = %#v", inputs.Foreign)
	}
	foreign := inputs.Foreign[0].Discovery.Candidates("oaw/superpowers")
	if len(foreign) != 1 || foreign[0].HostID != "claude" || foreign[0].DiagnosticLocation != physicalClaude {
		t.Fatalf("foreign Host candidates = %#v", foreign)
	}
}

func TestLoadProviderInputsIsDeterministicAndReadOnly(t *testing.T) {
	userHome := t.TempDir()
	userConfigRoot := filepath.Join(t.TempDir(), "open-agent-workflow")
	projectRoot := t.TempDir()
	for _, version := range []string{"6.0.3", "6.1.1"} {
		indicator := filepath.Join(userHome, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", version, "skills", "using-superpowers", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(indicator), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(indicator, []byte(version), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	options := providerInputOptions{
		HostID:         "codex",
		ProjectRoot:    projectRoot,
		UserConfigRoot: userConfigRoot,
		UserHome:       userHome,
	}
	first, err := loadProviderInputs(options)
	if err != nil {
		t.Fatalf("loadProviderInputs() error = %v", err)
	}
	second, err := loadProviderInputs(options)
	if err != nil {
		t.Fatalf("loadProviderInputs(second) error = %v", err)
	}
	if first.Configuration.Digest() != second.Configuration.Digest() ||
		first.Discovery.Digest() != second.Discovery.Digest() ||
		first.Resolutions.Digest() != second.Resolutions.Digest() ||
		first.Registry.Digest() != second.Registry.Digest() {
		t.Fatal("Provider input assembly is not deterministic")
	}
	resolution, found := first.Resolutions.Resolution("oaw/superpowers")
	if !found || resolution.State != registry.ProviderAmbiguous || len(resolution.Candidates) != 2 {
		t.Fatalf("resolution = %#v, found=%v", resolution, found)
	}
	wantConfigPath := filepath.Join(userConfigRoot, "config.toml")
	if first.UserConfigPath != wantConfigPath || first.UserConfigExists {
		t.Fatalf("user config metadata = %q, %v", first.UserConfigPath, first.UserConfigExists)
	}
	if _, err := os.Stat(wantConfigPath); !os.IsNotExist(err) {
		t.Fatalf("Provider assembly created user config: %v", err)
	}
}

func writeProviderInputMarker(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
