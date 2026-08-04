package host_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestAdmitWorkflowReturnsPinnedEffectiveHost(t *testing.T) {
	integration := runnerIntegration(t)
	bindings := []catalog.HostBinding{
		{Host: "codex", Kind: "skill", Reference: "superpowers:writing-plans"},
		{Host: "codex", Kind: "agent", Reference: "matt:tdd"},
	}
	admitted, err := host.AdmitWorkflow([]host.IntegrationRecord{integration}, host.RuntimeFrame{HostID: "codex", IntegrationID: integration.ID}, bindings)
	if err != nil {
		t.Fatalf("AdmitWorkflow() error = %v", err)
	}
	if admitted.IntegrationID != integration.ID || admitted.IntegrationDigest != integration.Digest || admitted.ManifestDigest != integration.ManifestDigest || admitted.AuditDigest != integration.Audit.Digest || admitted.ConformanceDigest != integration.Conformance.Digest || !slices.Equal(admitted.EffectiveFeatures, integration.Manifest.Features) {
		t.Fatalf("admission = %#v", admitted)
	}
	admitted.EffectiveFeatures[0] = host.FeatureNativeInvocation
	fresh, err := host.AdmitWorkflow([]host.IntegrationRecord{integration}, host.RuntimeFrame{HostID: "codex", IntegrationID: integration.ID}, bindings)
	if err != nil || fresh.EffectiveFeatures[0] == host.FeatureNativeInvocation {
		t.Fatalf("AdmitWorkflow() exposed mutable storage: %#v, %v", fresh, err)
	}
}

func TestAdmitWorkflowAcceptsConformingNativeIntegration(t *testing.T) {
	integration := nativeIntegration(t)
	admitted, err := host.AdmitWorkflow(
		[]host.IntegrationRecord{integration},
		host.RuntimeFrame{HostID: "codex", IntegrationID: integration.ID},
		[]catalog.HostBinding{{Host: "codex", Kind: "tool", Reference: "native.invoke"}},
	)
	if err != nil {
		t.Fatalf("AdmitWorkflow() error = %v", err)
	}
	if !slices.Contains(admitted.EffectiveFeatures, host.FeatureNativeInvocation) || admitted.ConformanceDigest != integration.Conformance.Digest {
		t.Fatalf("native admission = %#v", admitted)
	}
}

func TestAdmitWorkflowFailsClosed(t *testing.T) {
	managed := runnerIntegration(t)
	instruction := instructionIntegration(t)
	validBinding := []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "fixture"}}
	for _, test := range []struct {
		name     string
		records  []host.IntegrationRecord
		frame    host.RuntimeFrame
		bindings []catalog.HostBinding
		code     string
	}{
		{"missing frame", []host.IntegrationRecord{managed}, host.RuntimeFrame{}, validBinding, "HOST_INTEGRATION_REQUIRED"},
		{"unknown Integration", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: "acme/missing"}, validBinding, "HOST_INTEGRATION_NOT_ADMITTED"},
		{"instruction-only", []host.IntegrationRecord{instruction}, host.RuntimeFrame{HostID: "codex", IntegrationID: instruction.ID}, validBinding, "HOST_INTEGRATION_NOT_ADMITTED"},
		{"wrong Runtime Host", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "claude", IntegrationID: managed.ID}, validBinding, "HOST_PROVIDER_SCOPE_MISMATCH"},
		{"required Feature unavailable", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID, UnavailableFeatures: []host.Feature{host.FeaturePause}}, validBinding, "HOST_RUNTIME_REQUIREMENTS_UNMET"},
		{"unknown unavailable Feature", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID, UnavailableFeatures: []host.Feature{"invented"}}, validBinding, "HOST_RUNTIME_REQUIREMENTS_UNMET"},
		{"duplicate unavailable Feature", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID, UnavailableFeatures: []host.Feature{host.FeaturePause, host.FeaturePause}}, validBinding, "HOST_RUNTIME_REQUIREMENTS_UNMET"},
		{"wrong Binding Host", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID}, []catalog.HostBinding{{Host: "claude", Kind: "skill", Reference: "fixture"}}, "HOST_BINDING_UNSUPPORTED"},
		{"unsupported Binding kind", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID}, []catalog.HostBinding{{Host: "codex", Kind: "command", Reference: "fixture"}}, "HOST_BINDING_UNSUPPORTED"},
		{"invalid Binding", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID}, []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: ""}}, "HOST_BINDING_UNSUPPORTED"},
		{"no Binding", []host.IntegrationRecord{managed}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID}, nil, "HOST_BINDING_UNSUPPORTED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := host.AdmitWorkflow(test.records, test.frame, test.bindings)
			if host.ErrorCode(err) != test.code {
				t.Fatalf("AdmitWorkflow() error = %v, want %s", err, test.code)
			}
		})
	}

	tampered := host.CloneIntegration(managed)
	tampered.Digest = strings.Repeat("0", 64)
	if _, err := host.AdmitWorkflow([]host.IntegrationRecord{tampered}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID}, validBinding); host.ErrorCode(err) != "HOST_INTEGRATION_NOT_ADMITTED" {
		t.Fatalf("tampered Integration error = %v", err)
	}
	pendingAudit := host.CloneIntegration(managed)
	pendingAudit.Audit.Status = host.AuditPending
	missingReport := host.CloneIntegration(managed)
	missingReport.Conformance = nil
	staleReport := host.CloneIntegration(managed)
	staleReport.Conformance.SuiteVersion = "oaw.host-conformance/v0"
	failedReport := host.CloneIntegration(managed)
	failedReport.Conformance.Checks[0].Passed = false
	failedReport.Conformance.Passed = false
	missingProtocol := host.CloneIntegration(managed)
	missingProtocol.Manifest.Protocols = nil
	missingInventoryFeature := host.CloneIntegration(managed)
	for index, feature := range missingInventoryFeature.Manifest.Features {
		if feature == host.FeatureProviderBindingInventory {
			missingInventoryFeature.Manifest.Features = append(missingInventoryFeature.Manifest.Features[:index], missingInventoryFeature.Manifest.Features[index+1:]...)
			break
		}
	}
	for name, record := range map[string]host.IntegrationRecord{
		"pending audit": pendingAudit, "missing Report": missingReport, "stale Report": staleReport,
		"failed Report": failedReport, "missing Runtime Protocol": missingProtocol, "missing Provider Binding Inventory Feature": missingInventoryFeature,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := host.AdmitWorkflow([]host.IntegrationRecord{record}, host.RuntimeFrame{HostID: "codex", IntegrationID: managed.ID}, validBinding); host.ErrorCode(err) != "HOST_INTEGRATION_NOT_ADMITTED" {
				t.Fatalf("AdmitWorkflow() error = %v", err)
			}
		})
	}
}

func nativeIntegration(t *testing.T) host.IntegrationRecord {
	t.Helper()
	base := runnerIntegration(t)
	manifestValue := host.CloneManifest(base.Manifest)
	manifestValue.IntegrationLevel = host.NativeManaged
	manifestValue.Features = append(manifestValue.Features, host.FeatureNativeInvocation)
	manifest, err := host.NewManifest(manifestValue)
	if err != nil {
		t.Fatal(err)
	}
	checks := make([]host.ConformanceCheck, len(manifest.Features))
	for index, feature := range manifest.Features {
		checks[index] = host.ConformanceCheck{ID: host.CheckID(feature), Passed: true, Evidence: strings.Repeat(string("123456789a"[index]), 64)}
	}
	report, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion: host.ConformanceReportSchemaV1, SuiteVersion: host.ConformanceSuiteV1,
		IntegrationID: base.ID, ManifestDigest: manifest.ContentDigest(), Checks: checks,
		TranscriptDigest: strings.Repeat("f", 64), Passed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV1, IntegrationVersion: "1.0.0", ID: base.ID,
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: base.Audit, Conformance: &report,
	})
	if err != nil {
		t.Fatal(err)
	}
	return integration
}

func instructionIntegration(t *testing.T) host.IntegrationRecord {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV1, ManifestVersion: "1.0.0", HostID: "codex", IntegrationLevel: host.InstructionOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPending})
	if err != nil {
		t.Fatal(err)
	}
	record, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV1, IntegrationVersion: "1.0.0", ID: "oaw/codex-instruction",
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit,
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}
