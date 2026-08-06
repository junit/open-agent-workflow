package host_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestValidateEnvironmentReportPinsSessionAndRequirements(t *testing.T) {
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     "session-current-1",
		Topology:      execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-session", Digest: strings.Repeat("a", 64),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := runnerManifest(t)
	manifest.ControlSurface = host.SurfaceHostNative
	manifest.SupportedTopologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV2,
		HostID:                  "codex",
		IntegrationID:           "acme/codex-runtime",
		IntegrationVersion:      "1.0.0",
		SessionID:               "session-current-1",
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: strings.Repeat("b", 64),
		EnvironmentReportDigest: report.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.ValidateEnvironmentReport(session, report); err != nil {
		t.Fatalf("ValidateEnvironmentReport() error = %v", err)
	}
	accepted := []execution.EnvironmentRequirement{{
		Surface: "skills", Required: true, AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited},
	}}
	if err := host.ValidateRequirements(accepted, report); err != nil {
		t.Fatalf("ValidateRequirements() error = %v", err)
	}
	rejected := []execution.EnvironmentRequirement{{
		Surface: "skills", Required: true, AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionHostConfigured},
	}}
	if err := host.ValidateRequirements(rejected, report); host.ErrorCode(err) != "HOST_ENVIRONMENT_REQUIREMENT_UNMET" {
		t.Fatalf("ValidateRequirements(rejected) error = %v", err)
	}

	changed, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     "session-current-2",
		Topology:      execution.TopologyCurrent,
		Observations:  report.Observations,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.ValidateEnvironmentReport(session, changed); host.ErrorCode(err) != "HOST_SESSION_CHANGED" {
		t.Fatalf("ValidateEnvironmentReport(changed) error = %v", err)
	}
}

func TestValidateEnvironmentReportPinsSubagentParent(t *testing.T) {
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion:   host.HostEnvironmentReportSchemaV2,
		SessionID:       "session-child-1",
		ParentSessionID: "session-current-1",
		Topology:        execution.TopologySubagent,
		Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-subagent", Digest: strings.Repeat("a", 64),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := runnerManifest(t)
	manifest.ControlSurface = host.SurfaceHostNative
	manifest.SupportedTopologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV2,
		HostID:                  "codex",
		IntegrationID:           "acme/codex-runtime",
		IntegrationVersion:      "1.0.0",
		SessionID:               "session-current-1",
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: strings.Repeat("b", 64),
		EnvironmentReportDigest: report.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := host.ValidateEnvironmentReport(session, report); err != nil {
		t.Fatalf("ValidateEnvironmentReport(SUBAGENT) error = %v", err)
	}
}

func TestHostManifestRejectsUnsupportedFeatureTopologyPairs(t *testing.T) {
	policy, err := host.NewManifest(policyManifestValue("codex"))
	if err != nil {
		t.Fatal(err)
	}
	if policy.ControlSurface != host.SurfacePolicy || !slices.Equal(policy.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("policy Manifest = %#v", policy)
	}
	hostNative, err := host.NewManifest(hostNativeManifestValue("codex"))
	if err != nil {
		t.Fatal(err)
	}
	if hostNative.ControlSurface != host.SurfaceHostNative || !slices.Equal(hostNative.SupportedTopologies, []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}) {
		t.Fatalf("host-native Manifest = %#v", hostNative)
	}

	for _, test := range []struct {
		name   string
		mutate func(*host.Manifest)
	}{
		{name: "policy feature", mutate: func(value *host.Manifest) {
			*value = policyManifestValue("codex")
			value.Features = []host.Feature{host.FeatureNormalizedReceipts}
		}},
		{name: "policy SUBAGENT", mutate: func(value *host.Manifest) {
			*value = policyManifestValue("codex")
			value.SupportedTopologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
		}},
		{name: "host-native without CURRENT", mutate: func(value *host.Manifest) {
			value.SupportedTopologies = []execution.Topology{execution.TopologySubagent}
		}},
		{name: "SUBAGENT without environment reporting", mutate: func(value *host.Manifest) {
			value.Features = []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := hostNativeManifestValue("codex")
			test.mutate(&value)
			if _, err := host.NewManifest(value); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
				t.Fatalf("NewManifest() error = %v", err)
			}
		})
	}
}

func TestHostV2RejectsRetiredSchemasAndControlSurfaces(t *testing.T) {
	retiredManifest := policyManifestValue("codex")
	retiredManifest.SchemaVersion = "oaw.host-manifest/v1"
	if _, err := host.NewManifest(retiredManifest); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewManifest(v1) error = %v", err)
	}
	for _, surface := range []host.ControlSurface{"instruction-only", "runner-managed", "native-managed"} {
		retiredSurface := policyManifestValue("codex")
		retiredSurface.ControlSurface = surface
		if _, err := host.NewManifest(retiredSurface); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
			t.Fatalf("NewManifest(%s) error = %v", surface, err)
		}
	}
	retiredProtocol := hostNativeManifestValue("codex")
	retiredProtocol.Protocols = []string{"oaw.runtime/v1"}
	if _, err := host.NewManifest(retiredProtocol); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
		t.Fatalf("NewManifest(oaw.runtime/v1) error = %v", err)
	}
	for _, feature := range []host.Feature{
		"isolated-executor", "exact-binding-invocation", "bundle-inheritance",
		"evidence-return", "normalized-observation", "native-invocation",
	} {
		retiredFeature := hostNativeManifestValue("codex")
		retiredFeature.Features = append(retiredFeature.Features, feature)
		if _, err := host.NewManifest(retiredFeature); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
			t.Fatalf("NewManifest(%s) error = %v", feature, err)
		}
	}

	manifest, err := host.NewManifest(policyManifestValue("codex"))
	if err != nil {
		t.Fatal(err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPending})
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: "oaw.host-integration/v1", IntegrationVersion: "1.0.0", ID: "oaw/codex-policy",
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit,
	})
	if host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewIntegration(v1) error = %v", err)
	}
	legacyManifest := policyManifestValue("codex")
	legacyManifest.SchemaVersion = "oaw.host-manifest/v1"
	_, err = host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV2, IntegrationVersion: "2.0.0", ID: "oaw/codex-policy",
		Manifest: legacyManifest, ManifestDigest: legacyManifest.ContentDigest(), Audit: audit,
	})
	if host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewIntegration(legacy Manifest) error = %v", err)
	}
	if _, err := host.DecodeIntegrationSetJSON([]byte(`{"schema_version":"oaw.host-integration-set/v1","integrations":[]}`)); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("DecodeIntegrationSetJSON(v1) error = %v", err)
	}

	for _, test := range []struct {
		name string
		json string
		toml string
		code string
	}{
		{
			name: "Integration v1",
			json: `{"schema_version":"oaw.host-integration/v1"}`,
			toml: `schema_version = "oaw.host-integration/v1"`,
			code: "HOST_SCHEMA_UNSUPPORTED",
		},
		{
			name: "Manifest v1",
			json: `{"schema_version":"oaw.host-integration/v2","manifest":{"schema_version":"oaw.host-manifest/v1"}}`,
			toml: "schema_version = \"oaw.host-integration/v2\"\n[manifest]\nschema_version = \"oaw.host-manifest/v1\"",
			code: "HOST_SCHEMA_UNSUPPORTED",
		},
		{
			name: "Integration Level",
			json: `{"schema_version":"oaw.host-integration/v2","manifest":{"schema_version":"oaw.host-manifest/v2","integration_level":"runner-managed"}}`,
			toml: "schema_version = \"oaw.host-integration/v2\"\n[manifest]\nschema_version = \"oaw.host-manifest/v2\"\nintegration_level = \"runner-managed\"",
			code: "HOST_INTEGRATION_DECODE_INVALID",
		},
		{
			name: "retired control surface",
			json: `{"schema_version":"oaw.host-integration/v2","manifest":{"schema_version":"oaw.host-manifest/v2","control_surface":"runner-managed"}}`,
			toml: "schema_version = \"oaw.host-integration/v2\"\n[manifest]\nschema_version = \"oaw.host-manifest/v2\"\ncontrol_surface = \"runner-managed\"",
			code: "HOST_INTEGRATION_DECODE_INVALID",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := host.DecodeIntegrationJSON([]byte(test.json)); host.ErrorCode(err) != test.code {
				t.Fatalf("DecodeIntegrationJSON() error = %v", err)
			}
			if _, err := host.DecodeIntegrationTOML([]byte(test.toml)); host.ErrorCode(err) != test.code {
				t.Fatalf("DecodeIntegrationTOML() error = %v", err)
			}
		})
	}
}

func policyManifestValue(hostID string) host.Manifest {
	return host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: hostID,
		ControlSurface: host.SurfacePolicy, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
	}
}

func hostNativeManifestValue(hostID string) host.Manifest {
	return host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: hostID,
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []string{"skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
	}
}

func TestAuditEvidenceRejectsInvalidRecords(t *testing.T) {
	valid := host.AuditEvidence{
		Status:     host.AuditPassed,
		References: []host.AuditEvidenceReference{{Reference: "https://example.test/docs", Digest: strings.Repeat("a", 64)}},
	}
	for _, test := range []struct {
		name   string
		mutate func(*host.AuditEvidence)
	}{
		{"status", func(value *host.AuditEvidence) { value.Status = "invented" }},
		{"whitespace", func(value *host.AuditEvidence) { value.References[0].Reference = " https://example.test" }},
		{"control", func(value *host.AuditEvidence) { value.References[0].Reference = "https://example.test\nsecret" }},
		{"long", func(value *host.AuditEvidence) { value.References[0].Reference = strings.Repeat("x", 2049) }},
		{"digest", func(value *host.AuditEvidence) { value.References[0].Digest = "bad" }},
		{"duplicate", func(value *host.AuditEvidence) {
			value.References = append(value.References, host.AuditEvidenceReference{Reference: value.References[0].Reference, Digest: strings.Repeat("b", 64)})
		}},
		{"record digest", func(value *host.AuditEvidence) { value.Digest = strings.Repeat("0", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneAuditEvidence(valid)
			test.mutate(&value)
			if _, err := host.NewAuditEvidence(value); host.ErrorCode(err) != "HOST_AUDIT_INVALID" {
				t.Fatalf("NewAuditEvidence() error = %v", err)
			}
		})
	}
}

func TestConformanceReportRejectsInvalidRecords(t *testing.T) {
	base := host.CloneConformanceReport(*hostNativeIntegration(t).Conformance)
	for _, test := range []struct {
		name   string
		mutate func(*host.ConformanceReport)
		code   string
	}{
		{"schema", func(value *host.ConformanceReport) { value.SchemaVersion = "oaw.host-conformance-report/v1" }, "HOST_SCHEMA_UNSUPPORTED"},
		{"Manifest digest", func(value *host.ConformanceReport) { value.ManifestDigest = "bad" }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"transcript", func(value *host.ConformanceReport) { value.TranscriptDigest = "bad" }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"unknown feature", func(value *host.ConformanceReport) { value.VerifiedFeatures[0] = "invented" }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"duplicate feature", func(value *host.ConformanceReport) {
			value.VerifiedFeatures = append(value.VerifiedFeatures, value.VerifiedFeatures[0])
		}, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"invalid diagnostic", func(value *host.ConformanceReport) { value.Diagnostics = []string{"bad\nsecret"} }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"duplicate diagnostic", func(value *host.ConformanceReport) { value.Diagnostics = []string{"missing", "missing"} }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"too many diagnostics", func(value *host.ConformanceReport) {
			value.Diagnostics = make([]string, 33)
			for index := range value.Diagnostics {
				value.Diagnostics[index] = fmt.Sprintf("missing evidence %02d", index)
			}
		}, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"record digest", func(value *host.ConformanceReport) { value.Digest = strings.Repeat("0", 64) }, "HOST_CONFORMANCE_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneConformanceReport(base)
			test.mutate(&value)
			if _, err := host.NewConformanceReport(value); host.ErrorCode(err) != test.code {
				t.Fatalf("NewConformanceReport() error = %v", err)
			}
		})
	}
}

func TestIntegrationRejectsInvalidRecords(t *testing.T) {
	base := hostNativeIntegration(t)
	for _, test := range []struct {
		name   string
		mutate func(*host.IntegrationRecord)
		code   string
	}{
		{"schema", func(value *host.IntegrationRecord) { value.SchemaVersion = "oaw.host-integration/v1" }, "HOST_SCHEMA_UNSUPPORTED"},
		{"version", func(value *host.IntegrationRecord) { value.IntegrationVersion = "latest" }, "HOST_INTEGRATION_INVALID"},
		{"ID", func(value *host.IntegrationRecord) { value.ID = "Bad" }, "HOST_INTEGRATION_INVALID"},
		{"Manifest", func(value *host.IntegrationRecord) { value.Manifest.SchemaVersion = "oaw.host-manifest/v1" }, "HOST_SCHEMA_UNSUPPORTED"},
		{"Manifest digest", func(value *host.IntegrationRecord) { value.ManifestDigest = strings.Repeat("0", 64) }, "HOST_INTEGRATION_INVALID"},
		{"audit", func(value *host.IntegrationRecord) { value.Audit.Status = "invented" }, "HOST_INTEGRATION_INVALID"},
		{"report schema", func(value *host.IntegrationRecord) {
			value.Conformance.SchemaVersion = "oaw.host-conformance-report/v1"
		}, "HOST_SCHEMA_UNSUPPORTED"},
		{"report", func(value *host.IntegrationRecord) { value.Conformance.Digest = strings.Repeat("0", 64) }, "HOST_INTEGRATION_INVALID"},
		{"pending", func(value *host.IntegrationRecord) { value.Audit = pendingAudit(t) }, "HOST_INTEGRATION_INVALID"},
		{"missing report", func(value *host.IntegrationRecord) { value.Conformance = nil }, "HOST_INTEGRATION_INVALID"},
		{"diagnostic report", func(value *host.IntegrationRecord) {
			value.Conformance = rebuiltReport(t, *value.Conformance, func(report *host.ConformanceReport) { report.Diagnostics = []string{"missing evidence"} })
		}, "HOST_INTEGRATION_INVALID"},
		{"report Manifest", func(value *host.IntegrationRecord) {
			value.Conformance = rebuiltReport(t, *value.Conformance, func(report *host.ConformanceReport) { report.ManifestDigest = strings.Repeat("0", 64) })
		}, "HOST_INTEGRATION_INVALID"},
		{"report features", func(value *host.IntegrationRecord) {
			value.Conformance = rebuiltReport(t, *value.Conformance, func(report *host.ConformanceReport) { report.VerifiedFeatures = report.VerifiedFeatures[1:] })
		}, "HOST_INTEGRATION_INVALID"},
		{"record digest", func(value *host.IntegrationRecord) { value.Digest = strings.Repeat("0", 64) }, "HOST_INTEGRATION_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneIntegration(base)
			test.mutate(&value)
			if _, err := host.NewIntegration(value); host.ErrorCode(err) != test.code {
				t.Fatalf("NewIntegration() error = %v", err)
			}
		})
	}
}

func TestDecodeIntegrationRejectsSizeSyntaxAndDigestFailures(t *testing.T) {
	integration := hostNativeIntegration(t)
	oversized := []byte(strings.Repeat("x", (1<<20)+1))
	for name, decode := range map[string]func() error{
		"oversized JSON": func() error { _, err := host.DecodeIntegrationJSON(oversized); return err },
		"oversized TOML": func() error { _, err := host.DecodeIntegrationTOML(oversized); return err },
		"syntax JSON":    func() error { _, err := host.DecodeIntegrationJSON([]byte(`{"id":`)); return err },
		"syntax TOML":    func() error { _, err := host.DecodeIntegrationTOML([]byte(`id = [`)); return err },
		"digest JSON": func() error {
			raw := []byte(strings.Replace(mustJSON(t, integration), integration.Digest, strings.Repeat("0", 64), 1))
			_, err := host.DecodeIntegrationJSON(raw)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := decode(); host.ErrorCode(err) != "HOST_INTEGRATION_DECODE_INVALID" {
				t.Fatalf("decode error = %v", err)
			}
		})
	}
}

func TestHostErrorContract(t *testing.T) {
	cause := errors.New("cause")
	err := &host.Error{Code: "HOST_TEST", Detail: "detail", Cause: cause}
	if err.Error() != "HOST_TEST: detail" || !errors.Is(err, cause) || host.ErrorCode(err) != "HOST_TEST" || host.ErrorCode(errors.New("other")) != "" {
		t.Fatalf("error contract = %q, %v", err, errors.Unwrap(err))
	}
	if (&host.Error{Code: "HOST_TEST"}).Error() != "HOST_TEST" {
		t.Fatal("code-only error changed")
	}
}

func TestValidateIntegrationRecordRequiresCanonicalStoredBytes(t *testing.T) {
	valid := hostNativeIntegration(t)
	if err := host.ValidateIntegrationRecord(valid); err != nil {
		t.Fatalf("ValidateIntegrationRecord(valid) error = %v", err)
	}
	noncanonical := host.CloneIntegration(valid)
	noncanonical.Manifest.Features[0], noncanonical.Manifest.Features[1] = noncanonical.Manifest.Features[1], noncanonical.Manifest.Features[0]
	if err := host.ValidateIntegrationRecord(noncanonical); host.ErrorCode(err) != "HOST_INTEGRATION_INVALID" {
		t.Fatalf("ValidateIntegrationRecord(noncanonical) error = %v", err)
	}
}

func pendingAudit(t *testing.T) host.AuditEvidence {
	t.Helper()
	value, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPending})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func rebuiltReport(t *testing.T, value host.ConformanceReport, mutate func(*host.ConformanceReport)) *host.ConformanceReport {
	t.Helper()
	value = host.CloneConformanceReport(value)
	value.Digest = ""
	mutate(&value)
	report, err := host.NewConformanceReport(value)
	if err != nil {
		t.Fatal(err)
	}
	return &report
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
