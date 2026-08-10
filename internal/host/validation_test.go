package host_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestHostV3EnvironmentValidationPinsSessionAndRequirements(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory})
	_, report, session := currentHostFacts(t, manifest)
	if err := host.ValidateEnvironmentReport(session, report); err != nil {
		t.Fatalf("ValidateEnvironmentReport() error = %v", err)
	}
	if err := host.ValidateRequirements([]execution.EnvironmentRequirement{{Surface: "skills", Required: true, AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited}}}, report); err != nil {
		t.Fatalf("ValidateRequirements() error = %v", err)
	}
	changed := host.CloneEnvironmentReport(report)
	changed.Digest = ""
	changed.Observations[0].Digest = strings.Repeat("f", 64)
	changed, err := host.NewEnvironmentReport(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.ValidateEnvironmentReport(session, changed); host.ErrorCode(err) != "HOST_SESSION_CHANGED" {
		t.Fatalf("changed environment error = %v", err)
	}
	if err := host.ValidateRequirements([]execution.EnvironmentRequirement{{Surface: "mcp", Required: true, AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited}}}, report); host.ErrorCode(err) != "HOST_ENVIRONMENT_REQUIREMENT_UNMET" {
		t.Fatalf("unmet requirement error = %v", err)
	}
}

func TestHostV3ManifestRejectsInvalidSurfaceFeatureActionAndTopologyPairs(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*host.Manifest)
	}{
		{"version", func(value *host.Manifest) { value.ManifestVersion = "latest" }},
		{"host", func(value *host.Manifest) { value.HostID = "Bad Host" }},
		{"protocol", func(value *host.Manifest) { value.Protocols = []string{"oaw.runtime/v1"} }},
		{"binding kind", func(value *host.Manifest) { value.BindingKinds = []catalog.BindingKind{"hook"} }},
		{"duplicate binding kind", func(value *host.Manifest) {
			value.BindingKinds = []catalog.BindingKind{catalog.BindingSkill, catalog.BindingSkill}
		}},
		{"unknown control feature", func(value *host.Manifest) { value.Features = append(value.Features, "invented") }},
		{"duplicate control feature", func(value *host.Manifest) { value.Features = append(value.Features, value.Features[0]) }},
		{"unknown delegation feature", func(value *host.Manifest) { value.DelegationFeatures = append(value.DelegationFeatures, "invented") }},
		{"duplicate delegation feature", func(value *host.Manifest) {
			value.DelegationFeatures = append(value.DelegationFeatures, value.DelegationFeatures[0])
		}},
		{"action drift", func(value *host.Manifest) { value.HostActions[0].MaximumEffects[0] = "delete-project" }},
		{"duplicate action", func(value *host.Manifest) { value.HostActions = append(value.HostActions, value.HostActions[0]) }},
		{"without CURRENT", func(value *host.Manifest) {
			value.SupportedTopologies = []execution.Topology{execution.TopologySubagent}
		}},
		{"SUBAGENT without environment", func(value *host.Manifest) {
			value.Features = []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}
		}},
		{"retired surface", func(value *host.Manifest) { value.ControlSurface = "runner-managed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := host.CloneManifest(runnerManifest(t))
			value.Digest = ""
			test.mutate(&value)
			if _, err := host.NewManifest(value); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
				t.Fatalf("NewManifest() error = %v", err)
			}
		})
	}
	policy := policyManifestValue("codex")
	policy.Protocols = []string{host.WorkflowProtocolV1}
	if _, err := host.NewManifest(policy); host.ErrorCode(err) != "HOST_MANIFEST_INVALID" {
		t.Fatalf("policy authority error = %v", err)
	}
}

func TestHostV3HardCutRejectsV2AuthoritySchemas(t *testing.T) {
	manifest := mustHostManifestV3(t)
	tests := []struct {
		name string
		run  func() error
	}{
		{"Manifest", func() error {
			_, err := host.NewManifest(host.Manifest{SchemaVersion: "oaw.host-manifest/v2"})
			return err
		}},
		{"Session", func() error {
			_, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{SchemaVersion: "oaw.host-session/v2"})
			return err
		}},
		{"Inventory", func() error {
			_, err := host.ValidateBindingInventory(host.BindingInventory{SchemaVersion: "oaw.host-binding-inventory/v2"})
			return err
		}},
		{"Integration", func() error {
			_, err := host.NewIntegration(host.IntegrationRecord{SchemaVersion: "oaw.host-integration/v2"})
			return err
		}},
		{"Integration Set", func() error {
			_, err := host.DecodeIntegrationSetJSON([]byte(`{"schema_version":"oaw.host-integration-set/v2","integrations":[]}`))
			return err
		}},
		{"Conformance Transcript", func() error {
			_, err := host.NewConformanceTranscript(host.ConformanceTranscript{SchemaVersion: "oaw.host-conformance-transcript/v2"})
			return err
		}},
		{"Conformance Report", func() error {
			_, err := host.NewConformanceReport(host.ConformanceReport{SchemaVersion: "oaw.host-conformance-report/v2"})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
				t.Fatalf("error = %v, want HOST_SCHEMA_UNSUPPORTED", err)
			}
		})
	}
	for _, decode := range []func([]byte) error{
		func(raw []byte) error { _, err := host.DecodeIntegrationJSON(raw); return err },
		func(raw []byte) error { _, err := host.DecodeIntegrationTOML(raw); return err },
	} {
		if err := decode([]byte(`schema_version = "oaw.host-integration/v2"`)); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" && host.ErrorCode(err) != "HOST_INTEGRATION_DECODE_INVALID" {
			t.Fatalf("retired Integration decode error = %v", err)
		}
	}
}

func TestHostV3AuditConformanceAndIntegrationRejectInvalidRecords(t *testing.T) {
	validAudit := host.AuditEvidence{Status: host.AuditPassed, References: []host.AuditEvidenceReference{{Reference: "evidence://audit/docs", Digest: strings.Repeat("a", 64)}}}
	for _, test := range []struct {
		name   string
		mutate func(*host.AuditEvidence)
	}{
		{"status", func(value *host.AuditEvidence) { value.Status = "invented" }},
		{"absolute", func(value *host.AuditEvidence) { value.References[0].Reference = "/private/audit" }},
		{"digest", func(value *host.AuditEvidence) { value.References[0].Digest = "bad" }},
		{"duplicate", func(value *host.AuditEvidence) { value.References = append(value.References, value.References[0]) }},
	} {
		t.Run("audit "+test.name, func(t *testing.T) {
			value := validAudit
			value.References = append([]host.AuditEvidenceReference{}, validAudit.References...)
			test.mutate(&value)
			if _, err := host.NewAuditEvidence(value); host.ErrorCode(err) != "HOST_AUDIT_INVALID" {
				t.Fatalf("NewAuditEvidence() error = %v", err)
			}
		})
	}

	reportBase := host.CloneConformanceReport(*hostNativeIntegration(t).Conformance)
	for _, test := range []struct {
		name   string
		mutate func(*host.ConformanceReport)
		code   string
	}{
		{"manifest digest", func(value *host.ConformanceReport) { value.ManifestDigest = "bad" }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"control", func(value *host.ConformanceReport) { value.VerifiedFeatures[0] = "invented" }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"delegation", func(value *host.ConformanceReport) { value.VerifiedDelegationFeatures[0] = "invented" }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"action", func(value *host.ConformanceReport) { value.VerifiedHostActionIDs[0] = "invented" }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"diagnostic", func(value *host.ConformanceReport) { value.Diagnostics = []string{"bad\nsecret"} }, "HOST_CONFORMANCE_REPORT_INVALID"},
		{"digest", func(value *host.ConformanceReport) { value.Digest = strings.Repeat("0", 64) }, "HOST_CONFORMANCE_INVALID"},
	} {
		t.Run("report "+test.name, func(t *testing.T) {
			value := host.CloneConformanceReport(reportBase)
			test.mutate(&value)
			if _, err := host.NewConformanceReport(value); host.ErrorCode(err) != test.code {
				t.Fatalf("NewConformanceReport() error = %v", err)
			}
		})
	}

	integrationBase := hostNativeIntegration(t)
	for _, test := range []struct {
		name   string
		mutate func(*host.IntegrationRecord)
	}{
		{"version", func(value *host.IntegrationRecord) { value.IntegrationVersion = "latest" }},
		{"ID", func(value *host.IntegrationRecord) { value.ID = "Bad" }},
		{"manifest digest", func(value *host.IntegrationRecord) { value.ManifestDigest = strings.Repeat("0", 64) }},
		{"audit", func(value *host.IntegrationRecord) { value.Audit.Status = "invented" }},
		{"missing report", func(value *host.IntegrationRecord) { value.Conformance = nil }},
		{"record digest", func(value *host.IntegrationRecord) { value.Digest = strings.Repeat("0", 64) }},
	} {
		t.Run("integration "+test.name, func(t *testing.T) {
			value := host.CloneIntegration(integrationBase)
			test.mutate(&value)
			if _, err := host.NewIntegration(value); host.ErrorCode(err) != "HOST_INTEGRATION_INVALID" {
				t.Fatalf("NewIntegration() error = %v", err)
			}
		})
	}
}

func TestIntegrationV3DecodeRejectsSizeSyntaxAndDigestFailures(t *testing.T) {
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

func TestIntegrationV3StoredRecordRequiresCanonicalBytes(t *testing.T) {
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

func policyManifestValue(hostID string) host.Manifest {
	return host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: hostID, ControlSurface: host.SurfacePolicy,
		Protocols: []string{}, BindingKinds: []catalog.BindingKind{}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{}, DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func diagnosticSet(size int) []string {
	result := make([]string, size)
	for index := range result {
		result[index] = fmt.Sprintf("diagnostic %02d", index)
	}
	return result
}
