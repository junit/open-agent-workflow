package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

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
	if !found || resolution.State != registry.Ambiguous || len(resolution.Candidates) != 2 {
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
