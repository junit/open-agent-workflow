package host_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestHostV3SessionPinsManifestAndCurrentFacts(t *testing.T) {
	manifest := runnerManifest(t)
	topologies := []execution.Topology{execution.TopologyCurrent}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "acme/codex-runtime",
		IntegrationVersion: "3.0.0", SessionID: "session-current-1", ManifestDigest: manifest.Digest,
		SupportedTopologies: topologies, ProviderInventoryDigest: strings.Repeat("a", 64),
		FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{},
		EnvironmentReportDigest: strings.Repeat("b", 64), SandboxPolicyDigest: strings.Repeat("c", 64), ApprovalPolicyDigest: strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatalf("NewSessionSnapshot() error = %v", err)
	}
	if session.Digest == "" || session.ManifestDigest != manifest.Digest || session.FeatureDigest == "" || session.HostActionDigest == "" ||
		!slices.Equal(session.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("SessionSnapshot = %#v", session)
	}
	topologies[0] = execution.TopologySubagent
	clone := host.CloneSessionSnapshot(session)
	clone.SupportedTopologies[0] = execution.TopologySubagent
	if session.SupportedTopologies[0] != execution.TopologyCurrent {
		t.Fatal("SessionSnapshot shares topology storage")
	}

	tampered := session
	tampered.Digest = ""
	tampered.ManifestDigest = strings.Repeat("f", 64)
	if _, err := host.NewSessionSnapshot(manifest, tampered); host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("tampered Manifest digest error = %v", err)
	}
}

func TestHostV3SessionRejectsUnsupportedTopologyAndPolicyManifest(t *testing.T) {
	manifest := runnerManifest(t)
	manifest.Digest = ""
	manifest.SupportedTopologies = []execution.Topology{execution.TopologyCurrent}
	manifest, err := host.NewManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	input := host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "acme/codex-runtime", IntegrationVersion: "3.0.0",
		SessionID: "session-current-1", ManifestDigest: manifest.Digest,
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: strings.Repeat("a", 64), FeatureObservations: []host.FeatureObservation{},
		HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: strings.Repeat("b", 64),
	}
	if _, err := host.NewSessionSnapshot(manifest, input); host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("unsupported SUBAGENT error = %v", err)
	}
	policy, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfacePolicy,
		Protocols: []string{}, BindingKinds: []catalog.BindingKind{}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{}, DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	input.ManifestDigest = policy.Digest
	input.SupportedTopologies = []execution.Topology{execution.TopologyCurrent}
	if _, err := host.NewSessionSnapshot(policy, input); host.ErrorCode(err) != "HOST_SESSION_INVALID" {
		t.Fatalf("policy Manifest error = %v", err)
	}
}

func TestEnvironmentReportV2BridgeUsesClosedDispositions(t *testing.T) {
	observations := []execution.EnvironmentObservation{
		{Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-session", Digest: strings.Repeat("a", 64)},
		{Surface: "mcp", Disposition: execution.DispositionHostConfigured, Source: "codex-session", Digest: strings.Repeat("b", 64)},
	}
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-current-1", Topology: execution.TopologyCurrent, Observations: observations,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Digest == "" || report.Observations[0].Surface != "mcp" {
		t.Fatalf("EnvironmentReport = %#v", report)
	}
	observations[0].Surface = "changed"
	clone := host.CloneEnvironmentReport(report)
	clone.Observations[0].Surface = "changed-again"
	if report.Observations[0].Surface != "mcp" {
		t.Fatal("EnvironmentReport shares observation storage")
	}
	invalid := report
	invalid.Digest = ""
	invalid.Observations[0].Disposition = "invented"
	if _, err := host.NewEnvironmentReport(invalid); host.ErrorCode(err) != "HOST_ENVIRONMENT_REPORT_INVALID" {
		t.Fatalf("invalid disposition error = %v", err)
	}
}

func TestHostV3ManifestNormalizesAndDefendsAllCollections(t *testing.T) {
	manifest := runnerManifest(t)
	if manifest.Digest == "" || manifest.ContentDigest() != manifest.Digest || !slices.Equal(manifest.BindingKinds, allBindingKindsV3()) {
		t.Fatalf("Manifest = %#v", manifest)
	}
	clone := host.CloneManifest(manifest)
	clone.BindingKinds[0] = "changed"
	clone.Features[0] = "changed"
	clone.DelegationFeatures[0] = "changed"
	clone.HostActions[0].MaximumEffects[0] = "changed"
	if manifest.BindingKinds[0] == "changed" || manifest.Features[0] == "changed" || manifest.DelegationFeatures[0] == "changed" || manifest.HostActions[0].MaximumEffects[0] == "changed" {
		t.Fatal("CloneManifest() shares nested storage")
	}
}

func TestConformanceV4IntegrationRecordIsCanonical(t *testing.T) {
	integration := hostNativeIntegration(t)
	if integration.Digest == "" || integration.Conformance == nil || integration.Conformance.Digest == "" {
		t.Fatalf("Integration = %#v", integration)
	}
	clone := host.CloneIntegration(integration)
	clone.Conformance.VerifiedFeatures[0] = "changed"
	clone.Manifest.HostActions[0].Resources[0] = "changed"
	if integration.Conformance.VerifiedFeatures[0] == "changed" || integration.Manifest.HostActions[0].Resources[0] == "changed" {
		t.Fatal("CloneIntegration() shares nested storage")
	}
	tampered := integration
	tampered.ManifestDigest = strings.Repeat("0", 64)
	if _, err := host.NewIntegration(tampered); host.ErrorCode(err) != "HOST_INTEGRATION_INVALID" {
		t.Fatalf("tampered Manifest digest error = %v", err)
	}
}

func TestIntegrationV3JSONAndTOMLDecodersAreStrict(t *testing.T) {
	integration := hostNativeIntegration(t)
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
		"unknown JSON": unknownJSON, "trailing JSON": append(append([]byte{}, jsonRaw...), []byte(` {}`)...),
		"unknown TOML": append(append([]byte{}, tomlRaw.Bytes()...), []byte("\nunknown = true\n")...), "invalid UTF-8": {0xff, 0xfe},
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

func TestIntegrationV3DecoderRequiresAuthoredDigests(t *testing.T) {
	base := hostNativeIntegration(t)
	for _, test := range []struct {
		name   string
		mutate func(*host.IntegrationRecord)
	}{
		{"Integration", func(value *host.IntegrationRecord) { value.Digest = "" }},
		{"Manifest record", func(value *host.IntegrationRecord) { value.Manifest.Digest = "" }},
		{"Manifest reference", func(value *host.IntegrationRecord) { value.ManifestDigest = "" }},
		{"audit", func(value *host.IntegrationRecord) { value.Audit.Digest = "" }},
		{"Conformance", func(value *host.IntegrationRecord) { value.Conformance.Digest = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneIntegration(base)
			test.mutate(&value)
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := host.DecodeIntegrationJSON(raw); host.ErrorCode(err) != "HOST_INTEGRATION_DECODE_INVALID" {
				t.Fatalf("DecodeIntegrationJSON() error = %v", err)
			}
		})
	}
}

func runnerManifest(t *testing.T) host.Manifest {
	t.Helper()
	value, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: allBindingKindsV3(),
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features:            allControlFeaturesV3(), DelegationFeatures: allDelegationFeaturesV3(), HostActions: allHostActionsV3(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func hostNativeIntegration(t *testing.T) host.IntegrationRecord {
	t.Helper()
	manifest := runnerManifest(t)
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status:     host.AuditPassed,
		References: []host.AuditEvidenceReference{{Reference: "evidence://codex/audit/host-v3", Digest: strings.Repeat("a", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion: host.HostConformanceReportSchemaV4, ManifestDigest: manifest.Digest,
		HostSessionDigest: strings.Repeat("e", 64), BindingInventoryDigest: strings.Repeat("d", 64), TranscriptDigest: strings.Repeat("f", 64),
		VerifiedFeatures: manifest.Features, VerifiedDelegationFeatures: manifest.DelegationFeatures,
		VerifiedHostActionIDs: []string{"closeout.execute", "verification.execute", "workspace.prepare-or-confirm"}, Diagnostics: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: "3.0.0", ID: "acme/codex-host",
		Manifest: manifest, ManifestDigest: manifest.Digest, Audit: audit, Conformance: &report,
	})
	if err != nil {
		t.Fatal(err)
	}
	return integration
}
