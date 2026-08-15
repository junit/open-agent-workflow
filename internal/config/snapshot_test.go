package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestLoadPinsUserTrustedHostIntegration(t *testing.T) {
	userRoot := t.TempDir()
	integration := configTestHostIntegration(t)
	writeConfigTestHostIntegration(t, userRoot, "integrations/codex.toml", integration)
	if err := os.WriteFile(filepath.Join(userRoot, "config.toml"), []byte(`
schema_version = "oaw.user-config/v3"

[[host_integrations]]
id = "acme/codex-policy"
path = "integrations/codex.toml"
replace = false
`), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Load(LoadOptions{UserConfigRoot: userRoot})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	loaded, found := snapshot.HostIntegration("acme/codex-policy")
	if !found || loaded.Digest != integration.Digest {
		t.Fatalf("HostIntegration() = %#v, %t", loaded, found)
	}
	if got := len(snapshot.HostIntegrations()); got != 10 {
		t.Fatalf("HostIntegrations() count = %d, want 10", got)
	}
	if _, found := snapshot.HostIntegration("oaw/codex-host"); found {
		t.Fatal("retired default Bridge Host Integration is still present")
	}
	loaded.Manifest.SupportedTopologies[0] = execution.TopologySubagent
	fresh, found := snapshot.HostIntegration("acme/codex-policy")
	if !found || fresh.Manifest.SupportedTopologies[0] != execution.TopologyCurrent {
		t.Fatal("HostIntegration() exposed Snapshot storage")
	}
	for _, record := range snapshot.Record().HostIntegrations {
		if record.Digest == "" {
			t.Fatalf("Snapshot record does not pin Host Integration %q", record.ID)
		}
	}
	if snapshot.Digest() == "" {
		t.Fatalf("Snapshot record does not pin Host Integrations: %#v", snapshot.Record())
	}
}

func TestLoadRejectsUntrustedHostIntegrationInputs(t *testing.T) {
	t.Run("reserved built-in replacement", func(t *testing.T) {
		root := t.TempDir()
		builtins, err := host.LoadBuiltinIntegrations(assets.FS())
		if err != nil {
			t.Fatal(err)
		}
		writeConfigTestHostIntegration(t, root, "integrations/codex.toml", configTestIntegrationByID(t, builtins, "oaw/codex-policy"))
		writeConfigTestUserFile(t, root, `
schema_version = "oaw.user-config/v3"
[[host_integrations]]
id = "oaw/codex-policy"
path = "integrations/codex.toml"
replace = true
`)
		if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "RESERVED_HOST_INTEGRATION_NAMESPACE") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("reference ID mismatch", func(t *testing.T) {
		root := t.TempDir()
		writeConfigTestHostIntegration(t, root, "integrations/codex.toml", configTestHostIntegration(t))
		writeConfigTestUserFile(t, root, `
schema_version = "oaw.user-config/v3"
[[host_integrations]]
id = "acme/other-policy"
path = "integrations/codex.toml"
replace = false
`)
		if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "CONTENT_REFERENCE_ID_MISMATCH") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("digest tampering", func(t *testing.T) {
		root := t.TempDir()
		integration := configTestHostIntegration(t)
		integration.Digest = strings.Repeat("0", 64)
		writeConfigTestHostIntegration(t, root, "integrations/codex.toml", integration)
		writeConfigTestUserFile(t, root, `
schema_version = "oaw.user-config/v3"
[[host_integrations]]
id = "acme/codex-policy"
path = "integrations/codex.toml"
replace = false
`)
		if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "USER_HOST_INTEGRATION_INVALID") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("duplicate reference", func(t *testing.T) {
		root := t.TempDir()
		writeConfigTestUserFile(t, root, `
schema_version = "oaw.user-config/v3"
[[host_integrations]]
id = "acme/codex-policy"
path = "integrations/one.toml"
replace = false
[[host_integrations]]
id = "acme/codex-policy"
path = "integrations/two.toml"
replace = false
`)
		if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "DUPLICATE_HOST_INTEGRATION_REFERENCE") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("unsafe path", func(t *testing.T) {
		root := t.TempDir()
		writeConfigTestUserFile(t, root, `
schema_version = "oaw.user-config/v3"
[[host_integrations]]
id = "acme/codex-policy"
path = "../codex.toml"
replace = false
`)
		if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "CONFIG_PATH_INVALID") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("symlink escape", func(t *testing.T) {
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "codex.toml")
		writeConfigTestHostIntegration(t, filepath.Dir(outside), filepath.Base(outside), configTestHostIntegration(t))
		inside := filepath.Join(root, "integrations")
		if err := os.MkdirAll(inside, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(inside, "codex.toml")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		writeConfigTestUserFile(t, root, `
schema_version = "oaw.user-config/v3"
[[host_integrations]]
id = "acme/codex-policy"
path = "integrations/codex.toml"
replace = false
`)
		if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "CONFIG_PATH_ESCAPE") {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func configTestIntegrationByID(t *testing.T, values []host.IntegrationRecord, id string) host.IntegrationRecord {
	t.Helper()
	for _, value := range values {
		if value.ID == id {
			return value
		}
	}
	t.Fatalf("Host Integration %q not found", id)
	return host.IntegrationRecord{}
}

func TestEquivalentHostIntegrationOrderProducesSameSnapshot(t *testing.T) {
	first := configTestHostIntegration(t)
	second := configTestHostIntegrationWithID(t, first, "acme/second-policy")
	load := func(order []host.IntegrationRecord) Snapshot {
		t.Helper()
		root := t.TempDir()
		for _, integration := range []host.IntegrationRecord{first, second} {
			writeConfigTestHostIntegration(t, root, "integrations/"+strings.TrimPrefix(integration.ID, "acme/")+".toml", integration)
		}
		var configText strings.Builder
		configText.WriteString("schema_version = \"oaw.user-config/v3\"\n")
		for _, integration := range order {
			configText.WriteString("[[host_integrations]]\n")
			configText.WriteString("id = \"" + integration.ID + "\"\n")
			configText.WriteString("path = \"integrations/" + strings.TrimPrefix(integration.ID, "acme/") + ".toml\"\n")
			configText.WriteString("replace = false\n")
		}
		writeConfigTestUserFile(t, root, configText.String())
		snapshot, err := Load(LoadOptions{UserConfigRoot: root})
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	left := load([]host.IntegrationRecord{first, second})
	right := load([]host.IntegrationRecord{second, first})
	if left.Digest() != right.Digest() {
		t.Fatalf("equivalent Snapshot digests differ: %s != %s", left.Digest(), right.Digest())
	}
}

func configTestHostIntegration(t *testing.T) host.IntegrationRecord {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex",
		ControlSurface: host.SurfacePolicy, Protocols: []string{}, BindingKinds: []catalog.BindingKind{},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: []host.Feature{},
		DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status: host.AuditPending,
	})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: "3.0.0", ID: "acme/codex-policy",
		Manifest: manifest, ManifestDigest: manifest.Digest, Audit: audit,
	})
	if err != nil {
		t.Fatal(err)
	}
	return integration
}

func writeConfigTestHostIntegration(t *testing.T, root, relative string, integration host.IntegrationRecord) {
	t.Helper()
	var raw bytes.Buffer
	if err := toml.NewEncoder(&raw).Encode(integration); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func configTestHostIntegrationWithID(t *testing.T, value host.IntegrationRecord, id string) host.IntegrationRecord {
	t.Helper()
	value = host.CloneIntegration(value)
	value.ID = id
	value.Digest = ""
	integration, err := host.NewIntegration(value)
	if err != nil {
		t.Fatal(err)
	}
	return integration
}

func writeConfigTestUserFile(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
