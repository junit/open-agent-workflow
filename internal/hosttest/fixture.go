package hosttest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/config"
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
	integrationPath := filepath.Join(userRoot, "integrations", "codex-runtime.toml")
	if err := os.MkdirAll(filepath.Dir(integrationPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(integrationPath, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := "schema_version = \"oaw.user-config/v1\"\n" +
		"[[host_integrations]]\n" +
		"id = \"" + ManagedIntegrationID + "\"\n" +
		"path = \"integrations/codex-runtime.toml\"\n" +
		"replace = false\n" + extra
	if err := os.WriteFile(filepath.Join(userRoot, "config.toml"), []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	return integration
}

func ManagedIntegration(t testing.TB) host.IntegrationRecord {
	t.Helper()
	features := []host.Feature{
		host.FeatureBundleInheritance, host.FeatureCancellation, host.FeatureEvidenceReturn,
		host.FeatureExactBindingInvocation, host.FeatureInvocationDedup, host.FeatureIsolatedExecutor,
		host.FeatureNormalizedObservation, host.FeaturePause,
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV1, ManifestVersion: "1.0.0", HostID: "codex",
		IntegrationLevel: host.RunnerManaged, Protocols: []string{host.RuntimeProtocolV1},
		BindingKinds: []string{"agent", "skill", "tool"}, Features: features,
	})
	if err != nil {
		t.Fatal(err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPassed, References: []host.AuditEvidenceReference{{Reference: "https://example.test/codex/official-audit", Digest: strings.Repeat("a", 64)}}})
	if err != nil {
		t.Fatal(err)
	}
	checks := make([]host.ConformanceCheck, len(features))
	for index, feature := range features {
		checks[index] = host.ConformanceCheck{ID: host.CheckID(feature), Passed: true, Evidence: strings.Repeat(string("12345678"[index]), 64)}
	}
	report, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion: host.ConformanceReportSchemaV1, SuiteVersion: host.ConformanceSuiteV1,
		IntegrationID: ManagedIntegrationID, ManifestDigest: manifest.ContentDigest(), Checks: checks,
		TranscriptDigest: strings.Repeat("f", 64), Passed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV1, IntegrationVersion: "1.0.0", ID: ManagedIntegrationID,
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit, Conformance: &report,
	})
	if err != nil {
		t.Fatal(err)
	}
	return integration
}
