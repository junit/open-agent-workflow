package hosttest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const ManagedIntegrationID = "acme/codex-runtime"

func LoadManagedSnapshot(t testing.TB, projectRoot string) (config.Snapshot, host.IntegrationRecord) {
	t.Helper()
	userRoot := t.TempDir()
	integration := WriteManagedConfiguration(t, userRoot, "")
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, integration
}

func WriteManagedConfiguration(t testing.TB, userRoot, extra string) host.IntegrationRecord {
	t.Helper()
	integration := ManagedIntegration(t)
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(integration); err != nil {
		t.Fatal(err)
	}
	integrationPath := filepath.Join(userRoot, "integrations", "codex-host.toml")
	if err := os.MkdirAll(filepath.Dir(integrationPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(integrationPath, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := "schema_version = \"oaw.user-config/v3\"\n" +
		"[[host_integrations]]\n" +
		"id = \"" + ManagedIntegrationID + "\"\n" +
		"path = \"integrations/codex-host.toml\"\n" +
		"replace = false\n" + extra
	if err := os.WriteFile(filepath.Join(userRoot, "config.toml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	return integration
}

func ManagedIntegration(t testing.TB) host.IntegrationRecord {
	t.Helper()
	features := []host.Feature{
		host.FeatureCancellation, host.FeatureInvocationDedup, host.FeatureNormalizedReceipts,
		host.FeaturePause, host.FeatureProviderBindingInventory,
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1},
		BindingKinds: []string{"agent", "skill", "tool"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: features,
	})
	if err != nil {
		t.Fatal(err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPassed, References: []host.AuditEvidenceReference{{Reference: "https://example.test/codex/official-audit", Digest: strings.Repeat("a", 64)}}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion: host.HostConformanceReportSchemaV2, ManifestDigest: manifest.ContentDigest(),
		TranscriptDigest: strings.Repeat("f", 64), VerifiedFeatures: manifest.Features,
	})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV2, IntegrationVersion: "2.0.0", ID: ManagedIntegrationID,
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit, Conformance: &report,
	})
	if err != nil {
		t.Fatal(err)
	}
	return integration
}
