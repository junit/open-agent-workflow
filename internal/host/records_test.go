package host_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNewSessionSnapshotPinsCurrentSession(t *testing.T) {
	manifest := runnerManifest(t)
	manifest.ControlSurface = host.SurfaceHostNative
	manifest.SupportedTopologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	topologies := []execution.Topology{execution.TopologyCurrent}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV2,
		HostID:                  "codex",
		IntegrationID:           "acme/codex-runtime",
		IntegrationVersion:      "1.0.0",
		SessionID:               "session-current-1",
		SupportedTopologies:     topologies,
		ProviderInventoryDigest: strings.Repeat("a", 64),
		EnvironmentReportDigest: strings.Repeat("b", 64),
		SandboxPolicyDigest:     strings.Repeat("c", 64),
		ApprovalPolicyDigest:    strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatalf("NewSessionSnapshot() error = %v", err)
	}
	if session.Digest == "" || session.SessionID != "session-current-1" || !slices.Equal(session.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("SessionSnapshot = %#v", session)
	}

	topologies[0] = execution.TopologySubagent
	cloned := host.CloneSessionSnapshot(session)
	cloned.SupportedTopologies[0] = execution.TopologySubagent
	if session.SupportedTopologies[0] != execution.TopologyCurrent {
		t.Fatal("SessionSnapshot shares topology storage")
	}
}

func TestSessionSnapshotRejectsSubagentWithoutManifestSupport(t *testing.T) {
	manifest := runnerManifest(t)
	manifest.ControlSurface = host.SurfaceHostNative
	manifest.SupportedTopologies = []execution.Topology{execution.TopologyCurrent}
	_, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV2,
		HostID:                  "codex",
		IntegrationID:           "acme/codex-runtime",
		IntegrationVersion:      "1.0.0",
		SessionID:               "session-current-1",
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: strings.Repeat("a", 64),
		EnvironmentReportDigest: strings.Repeat("b", 64),
	})
	if host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("NewSessionSnapshot() error = %v", err)
	}
}

func TestSessionSnapshotRejectsPolicyManifest(t *testing.T) {
	manifest := runnerManifest(t)
	manifest.ControlSurface = host.SurfacePolicy
	manifest.SupportedTopologies = []execution.Topology{execution.TopologyCurrent}
	_, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV2,
		HostID:                  "codex",
		IntegrationID:           "acme/codex-runtime",
		IntegrationVersion:      "1.0.0",
		SessionID:               "session-current-1",
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: strings.Repeat("a", 64),
		EnvironmentReportDigest: strings.Repeat("b", 64),
	})
	if host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("NewSessionSnapshot(policy) error = %v", err)
	}
}

func TestEnvironmentReportUsesClosedDispositions(t *testing.T) {
	observations := []execution.EnvironmentObservation{
		{Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-session", Digest: strings.Repeat("a", 64)},
		{Surface: "mcp", Disposition: execution.DispositionHostConfigured, Source: "codex-session", Digest: strings.Repeat("b", 64)},
	}
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     "session-current-1",
		Topology:      execution.TopologyCurrent,
		Observations:  observations,
	})
	if err != nil {
		t.Fatalf("NewEnvironmentReport() error = %v", err)
	}
	if report.Digest == "" || report.ParentSessionID != "" || report.Observations[0].Surface != "mcp" {
		t.Fatalf("EnvironmentReport = %#v", report)
	}
	observations[0].Surface = "changed"
	cloned := host.CloneEnvironmentReport(report)
	cloned.Observations[0].Surface = "changed-again"
	if report.Observations[0].Surface != "mcp" {
		t.Fatal("EnvironmentReport shares observation storage")
	}

	_, err = host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     "session-current-1",
		Topology:      execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.EnvironmentDisposition("invented"), Source: "codex-session", Digest: strings.Repeat("a", 64),
		}},
	})
	if host.ErrorCode(err) != "HOST_ENVIRONMENT_REPORT_INVALID" {
		t.Fatalf("invalid disposition error = %v", err)
	}

	for _, invalid := range []string{"skills\nprivate", string([]byte{'s', 'k', 0xff})} {
		_, err = host.NewEnvironmentReport(host.EnvironmentReport{
			SchemaVersion: host.HostEnvironmentReportSchemaV2,
			SessionID:     "session-current-1",
			Topology:      execution.TopologyCurrent,
			Observations: []execution.EnvironmentObservation{{
				Surface: invalid, Disposition: execution.DispositionInherited, Source: "codex-session", Digest: strings.Repeat("a", 64),
			}},
		})
		if host.ErrorCode(err) != "HOST_ENVIRONMENT_REPORT_INVALID" {
			t.Fatalf("invalid surface %q error = %v", invalid, err)
		}
	}
}

func TestNewManifestNormalizesAndDefendsCollections(t *testing.T) {
	features := []host.Feature{
		host.FeatureCancellation,
		host.FeatureEnvironmentReporting,
		host.FeatureInvocationDedup,
		host.FeatureNormalizedReceipts,
		host.FeaturePause,
		host.FeatureProviderBindingInventory,
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1},
		BindingKinds:        []string{"tool", "agent", "skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}, Features: features,
	})
	if err != nil {
		t.Fatalf("NewManifest() error = %v", err)
	}
	wantKinds := []string{"agent", "skill", "tool"}
	if !slices.Equal(manifest.BindingKinds, wantKinds) {
		t.Fatalf("BindingKinds = %#v, want %#v", manifest.BindingKinds, wantKinds)
	}
	if manifest.ContentDigest() == "" {
		t.Fatal("ContentDigest() is empty")
	}

	features[0] = host.Feature("invented")
	manifest.BindingKinds[0] = "changed"
	fresh := host.CloneManifest(manifest)
	if slices.Contains(fresh.Features, host.Feature("invented")) || fresh.BindingKinds[0] != "changed" {
		t.Fatalf("CloneManifest() did not isolate caller mutation: %#v", fresh)
	}
	fresh.BindingKinds[0] = "mutated-again"
	if manifest.BindingKinds[0] != "changed" {
		t.Fatalf("CloneManifest() exposed source storage: %#v", manifest.BindingKinds)
	}
	if manifest.ContentDigest() == fresh.ContentDigest() {
		t.Fatal("ContentDigest() ignored changed clone content")
	}
}

func TestNewAuditEvidencePinsCanonicalReferences(t *testing.T) {
	references := []host.AuditEvidenceReference{
		{Reference: "https://example.test/host/release-notes", Digest: strings.Repeat("b", 64)},
		{Reference: "https://example.test/host/docs", Digest: strings.Repeat("a", 64)},
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPassed, References: references})
	if err != nil {
		t.Fatalf("NewAuditEvidence() error = %v", err)
	}
	if audit.Digest == "" || audit.References[0].Digest != strings.Repeat("a", 64) {
		t.Fatalf("audit = %#v", audit)
	}
	references[0].Reference = "changed"
	if audit.References[1].Reference != "https://example.test/host/release-notes" {
		t.Fatalf("audit shares caller storage: %#v", audit.References)
	}
	fresh := host.CloneAuditEvidence(audit)
	fresh.References[0].Reference = "mutated"
	if audit.References[0].Reference == "mutated" {
		t.Fatal("CloneAuditEvidence() exposed source storage")
	}

	if _, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPassed}); host.ErrorCode(err) != "HOST_AUDIT_INVALID" {
		t.Fatalf("empty passed audit error = %v", err)
	}
	if _, err := host.NewAuditEvidence(host.AuditEvidence{
		Status:     host.AuditPending,
		References: []host.AuditEvidenceReference{{Reference: "https://example.test", Digest: strings.Repeat("c", 64)}},
	}); host.ErrorCode(err) != "HOST_AUDIT_INVALID" {
		t.Fatalf("pending evidence error = %v", err)
	}
}

func TestNewIntegrationPinsManifestAuditAndConformance(t *testing.T) {
	manifest := runnerManifest(t)
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status: host.AuditPassed,
		References: []host.AuditEvidenceReference{{
			Reference: "https://example.test/codex/official-audit",
			Digest:    strings.Repeat("a", 64),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	features := append([]host.Feature{}, manifest.Features...)
	report, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion:    host.HostConformanceReportSchemaV2,
		ManifestDigest:   manifest.ContentDigest(),
		TranscriptDigest: strings.Repeat("f", 64),
		VerifiedFeatures: features,
	})
	if err != nil {
		t.Fatalf("NewConformanceReport() error = %v", err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion:      host.HostIntegrationSchemaV2,
		IntegrationVersion: "2.0.0",
		ID:                 "acme/codex-host",
		Manifest:           manifest,
		ManifestDigest:     manifest.ContentDigest(),
		Audit:              audit,
		Conformance:        &report,
	})
	if err != nil {
		t.Fatalf("NewIntegration() error = %v", err)
	}
	if integration.Digest == "" || integration.Conformance == nil || integration.Conformance.Digest == "" {
		t.Fatalf("integration = %#v", integration)
	}
	features[0] = host.Feature("changed")
	report.VerifiedFeatures[0] = host.Feature("changed")
	if integration.Conformance.VerifiedFeatures[0] == host.Feature("changed") {
		t.Fatal("Integration shares caller conformance storage")
	}
	fresh := host.CloneIntegration(integration)
	fresh.Conformance.VerifiedFeatures[0] = host.Feature("changed")
	if integration.Conformance.VerifiedFeatures[0] == host.Feature("changed") {
		t.Fatal("CloneIntegration() exposed source storage")
	}

	tampered := integration
	tampered.ManifestDigest = strings.Repeat("0", 64)
	if _, err := host.NewIntegration(tampered); host.ErrorCode(err) != "HOST_INTEGRATION_INVALID" {
		t.Fatalf("tampered Manifest digest error = %v", err)
	}
	missing := integration
	missing.Conformance = nil
	missing.Digest = ""
	if _, err := host.NewIntegration(missing); host.ErrorCode(err) != "HOST_INTEGRATION_INVALID" {
		t.Fatalf("missing Conformance error = %v", err)
	}
}

func runnerManifest(t *testing.T) host.Manifest {
	t.Helper()
	value, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1},
		BindingKinds:        []string{"agent", "skill", "tool"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features: []host.Feature{
			host.FeatureCancellation,
			host.FeatureEnvironmentReporting,
			host.FeatureInvocationDedup,
			host.FeatureNormalizedReceipts,
			host.FeaturePause,
			host.FeatureProviderBindingInventory,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestDecodeIntegrationJSONAndTOMLAreStrict(t *testing.T) {
	integration := runnerIntegration(t)
	jsonRaw, err := json.Marshal(integration)
	if err != nil {
		t.Fatal(err)
	}
	decodedJSON, err := host.DecodeIntegrationJSON(jsonRaw)
	if err != nil || decodedJSON.Digest != integration.Digest {
		t.Fatalf("DecodeIntegrationJSON() = %#v, %v", decodedJSON, err)
	}

	var tomlRaw bytes.Buffer
	if err := toml.NewEncoder(&tomlRaw).Encode(integration); err != nil {
		t.Fatal(err)
	}
	decodedTOML, err := host.DecodeIntegrationTOML(tomlRaw.Bytes())
	if err != nil || decodedTOML.Digest != integration.Digest {
		t.Fatalf("DecodeIntegrationTOML() = %#v, %v", decodedTOML, err)
	}

	unknownJSON := append([]byte(`{"unknown":true,`), jsonRaw[1:]...)
	for name, raw := range map[string][]byte{
		"unknown JSON":  unknownJSON,
		"trailing JSON": append(append([]byte{}, jsonRaw...), []byte(` {}`)...),
		"unknown TOML":  append(append([]byte{}, tomlRaw.Bytes()...), []byte("\nunknown = true\n")...),
		"invalid UTF-8": {0xff, 0xfe},
	} {
		t.Run(name, func(t *testing.T) {
			var decodeErr error
			if strings.Contains(name, "TOML") {
				_, decodeErr = host.DecodeIntegrationTOML(raw)
			} else {
				_, decodeErr = host.DecodeIntegrationJSON(raw)
			}
			if host.ErrorCode(decodeErr) != "HOST_INTEGRATION_DECODE_INVALID" {
				t.Fatalf("decode error = %v", decodeErr)
			}
		})
	}
}

func TestDecodeIntegrationRequiresEveryAuthoredDigest(t *testing.T) {
	base := runnerIntegration(t)
	for _, test := range []struct {
		name   string
		mutate func(*host.IntegrationRecord)
	}{
		{"Integration", func(value *host.IntegrationRecord) { value.Digest = "" }},
		{"Manifest", func(value *host.IntegrationRecord) { value.ManifestDigest = "" }},
		{"audit", func(value *host.IntegrationRecord) { value.Audit.Digest = "" }},
		{"Conformance", func(value *host.IntegrationRecord) { value.Conformance.Digest = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneIntegration(base)
			test.mutate(&value)
			jsonRaw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := host.DecodeIntegrationJSON(jsonRaw); host.ErrorCode(err) != "HOST_INTEGRATION_DECODE_INVALID" {
				t.Fatalf("DecodeIntegrationJSON() error = %v", err)
			}
			var tomlRaw bytes.Buffer
			if err := toml.NewEncoder(&tomlRaw).Encode(value); err != nil {
				t.Fatal(err)
			}
			if _, err := host.DecodeIntegrationTOML(tomlRaw.Bytes()); host.ErrorCode(err) != "HOST_INTEGRATION_DECODE_INVALID" {
				t.Fatalf("DecodeIntegrationTOML() error = %v", err)
			}
		})
	}
}

func runnerIntegration(t *testing.T) host.IntegrationRecord {
	t.Helper()
	manifest := runnerManifest(t)
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status: host.AuditPassed,
		References: []host.AuditEvidenceReference{{
			Reference: "https://example.test/codex/official-audit",
			Digest:    strings.Repeat("a", 64),
		}},
	})
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
		SchemaVersion: host.HostIntegrationSchemaV2, IntegrationVersion: "2.0.0", ID: "acme/codex-host",
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit, Conformance: &report,
	})
	if err != nil {
		t.Fatal(err)
	}
	return integration
}
