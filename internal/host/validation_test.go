package host_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNewManifestRejectsInvalidRecords(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*host.Manifest)
	}{
		{"schema", func(value *host.Manifest) { value.SchemaVersion = "oaw.host-manifest/v2" }},
		{"version", func(value *host.Manifest) { value.ManifestVersion = "latest" }},
		{"Host ID", func(value *host.Manifest) { value.HostID = "Bad Host" }},
		{"level", func(value *host.Manifest) { value.IntegrationLevel = "invented" }},
		{"duplicate protocol", func(value *host.Manifest) { value.Protocols = append(value.Protocols, host.RuntimeProtocolV1) }},
		{"no protocol", func(value *host.Manifest) { value.Protocols = nil }},
		{"no binding", func(value *host.Manifest) { value.BindingKinds = nil }},
		{"bad binding", func(value *host.Manifest) { value.BindingKinds = append(value.BindingKinds, "command") }},
		{"duplicate binding", func(value *host.Manifest) { value.BindingKinds = append(value.BindingKinds, "skill") }},
		{"missing feature", func(value *host.Manifest) { value.Features = value.Features[1:] }},
		{"unknown feature", func(value *host.Manifest) { value.Features = append(value.Features, "invented") }},
		{"duplicate feature", func(value *host.Manifest) { value.Features = append(value.Features, host.FeaturePause) }},
		{"runner native feature", func(value *host.Manifest) { value.Features = append(value.Features, host.FeatureNativeInvocation) }},
		{"native missing feature", func(value *host.Manifest) { value.IntegrationLevel = host.NativeManaged }},
		{"instruction capabilities", func(value *host.Manifest) { value.IntegrationLevel = host.InstructionOnly }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := runnerManifest(t)
			test.mutate(&value)
			if _, err := host.NewManifest(value); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
				t.Fatalf("NewManifest() error = %v", err)
			}
		})
	}
}

func TestInstructionOnlyManifestAndIntegrationRemainPolicyOnly(t *testing.T) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV1, ManifestVersion: "1.0.0", HostID: "codex",
		IntegrationLevel: host.InstructionOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPending})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV1, IntegrationVersion: "1.0.0", ID: "oaw/codex-instruction",
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit,
	})
	if err != nil || integration.Conformance != nil || integration.Digest == "" {
		t.Fatalf("NewIntegration() = %#v, %v", integration, err)
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
	base := host.CloneConformanceReport(*runnerIntegration(t).Conformance)
	for _, test := range []struct {
		name   string
		mutate func(*host.ConformanceReport)
	}{
		{"schema", func(value *host.ConformanceReport) { value.SchemaVersion = "oaw.host-conformance-report/v2" }},
		{"suite", func(value *host.ConformanceReport) { value.SuiteVersion = "oaw.host-conformance/v2" }},
		{"Integration ID", func(value *host.ConformanceReport) { value.IntegrationID = "Bad" }},
		{"Manifest digest", func(value *host.ConformanceReport) { value.ManifestDigest = "bad" }},
		{"transcript", func(value *host.ConformanceReport) { value.TranscriptDigest = "bad" }},
		{"empty checks", func(value *host.ConformanceReport) { value.Checks = nil }},
		{"unknown check", func(value *host.ConformanceReport) { value.Checks[0].ID = "invented" }},
		{"check digest", func(value *host.ConformanceReport) { value.Checks[0].Evidence = "bad" }},
		{"duplicate check", func(value *host.ConformanceReport) { value.Checks = append(value.Checks, value.Checks[0]) }},
		{"result", func(value *host.ConformanceReport) { value.Checks[0].Passed = false }},
		{"record digest", func(value *host.ConformanceReport) { value.Digest = strings.Repeat("0", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneConformanceReport(base)
			test.mutate(&value)
			if _, err := host.NewConformanceReport(value); host.ErrorCode(err) != "HOST_CONFORMANCE_INVALID" {
				t.Fatalf("NewConformanceReport() error = %v", err)
			}
		})
	}
}

func TestIntegrationRejectsInvalidRecords(t *testing.T) {
	base := runnerIntegration(t)
	for _, test := range []struct {
		name   string
		mutate func(*host.IntegrationRecord)
	}{
		{"schema", func(value *host.IntegrationRecord) { value.SchemaVersion = "oaw.host-integration/v2" }},
		{"version", func(value *host.IntegrationRecord) { value.IntegrationVersion = "latest" }},
		{"ID", func(value *host.IntegrationRecord) { value.ID = "Bad" }},
		{"Manifest", func(value *host.IntegrationRecord) { value.Manifest.SchemaVersion = "bad" }},
		{"Manifest digest", func(value *host.IntegrationRecord) { value.ManifestDigest = strings.Repeat("0", 64) }},
		{"audit", func(value *host.IntegrationRecord) { value.Audit.Status = "invented" }},
		{"report", func(value *host.IntegrationRecord) { value.Conformance.Digest = strings.Repeat("0", 64) }},
		{"pending", func(value *host.IntegrationRecord) { value.Audit = pendingAudit(t) }},
		{"missing report", func(value *host.IntegrationRecord) { value.Conformance = nil }},
		{"failed report", func(value *host.IntegrationRecord) { value.Conformance = failedReport(t, *value.Conformance) }},
		{"report ID", func(value *host.IntegrationRecord) {
			value.Conformance = rebuiltReport(t, *value.Conformance, func(report *host.ConformanceReport) { report.IntegrationID = "acme/other" })
		}},
		{"report Manifest", func(value *host.IntegrationRecord) {
			value.Conformance = rebuiltReport(t, *value.Conformance, func(report *host.ConformanceReport) { report.ManifestDigest = strings.Repeat("0", 64) })
		}},
		{"report checks", func(value *host.IntegrationRecord) {
			value.Conformance = rebuiltReport(t, *value.Conformance, func(report *host.ConformanceReport) { report.Checks = report.Checks[1:] })
		}},
		{"record digest", func(value *host.IntegrationRecord) { value.Digest = strings.Repeat("0", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneIntegration(base)
			test.mutate(&value)
			if _, err := host.NewIntegration(value); host.ErrorCode(err) != "HOST_INTEGRATION_INVALID" {
				t.Fatalf("NewIntegration() error = %v", err)
			}
		})
	}
}

func TestDecodeIntegrationRejectsSizeSyntaxAndDigestFailures(t *testing.T) {
	integration := runnerIntegration(t)
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

func TestHostErrorAndRuntimeFrameCopies(t *testing.T) {
	cause := errors.New("cause")
	err := &host.Error{Code: "HOST_TEST", Detail: "detail", Cause: cause}
	if err.Error() != "HOST_TEST: detail" || !errors.Is(err, cause) || host.ErrorCode(err) != "HOST_TEST" || host.ErrorCode(errors.New("other")) != "" {
		t.Fatalf("error contract = %q, %v", err, errors.Unwrap(err))
	}
	if (&host.Error{Code: "HOST_TEST"}).Error() != "HOST_TEST" {
		t.Fatal("code-only error changed")
	}
	frame := host.RuntimeFrame{IntegrationID: "acme/codex-runtime", UnavailableFeatures: []host.Feature{host.FeaturePause}}
	cloned := host.CloneRuntimeFrame(frame)
	cloned.UnavailableFeatures[0] = host.FeatureCancellation
	if frame.UnavailableFeatures[0] != host.FeaturePause {
		t.Fatal("CloneRuntimeFrame() exposed source storage")
	}
}

func TestValidateIntegrationRecordRequiresCanonicalStoredBytes(t *testing.T) {
	valid := runnerIntegration(t)
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

func failedReport(t *testing.T, value host.ConformanceReport) *host.ConformanceReport {
	t.Helper()
	value = host.CloneConformanceReport(value)
	value.Digest = ""
	value.Checks[0].Passed = false
	value.Passed = false
	report, err := host.NewConformanceReport(value)
	if err != nil {
		t.Fatal(err)
	}
	return &report
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
