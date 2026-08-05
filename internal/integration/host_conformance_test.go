package integration_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestHostConformancePinsReceiptTranscriptIntoTrustedIntegration(t *testing.T) {
	manifest, transcript := conformanceIntegrationFixture(t, nil)
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status: host.AuditPassed, References: []host.AuditEvidenceReference{{
			Reference: "audit://acme/codex-host", Digest: strings.Repeat("7", 64),
		}},
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
	if err := host.ValidateIntegrationRecord(integration); err != nil || integration.Conformance.TranscriptDigest != transcript.Digest {
		t.Fatalf("Host-native Integration = %#v, %v", integration, err)
	}
}

func TestHostConformanceRejectsMissingDeclaredFeatureEvidence(t *testing.T) {
	manifest, transcript := conformanceIntegrationFixture(t, []host.Feature{host.FeaturePause})
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) == 0 {
		t.Fatalf("Conformance Report omitted missing pause evidence: %#v", report)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{
		Status: host.AuditPassed, References: []host.AuditEvidenceReference{{
			Reference: "audit://acme/codex-host", Digest: strings.Repeat("7", 64),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV2, IntegrationVersion: "2.0.0", ID: "acme/codex-host",
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit, Conformance: &report,
	})
	if host.ErrorCode(err) != "HOST_INTEGRATION_INVALID" {
		t.Fatalf("NewIntegration(missing pause evidence) error = %v", err)
	}
}

func conformanceIntegrationFixture(t *testing.T, extraFeatures []host.Feature) (host.Manifest, host.ConformanceTranscript) {
	t.Helper()
	features := append([]host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}, extraFeatures...)
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []string{"skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: features,
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.NewBindingInventory("codex", []host.BindingObservation{{
		HostID: "codex", InstallationKey: "installation-acme", Binding: catalog.HostBinding{
			Host: "codex", Kind: "skill", Reference: "acme:implementation", Topologies: []execution.Topology{execution.TopologyCurrent},
		}, Topologies: []execution.Topology{execution.TopologyCurrent}, Source: "native-probe",
		EvidenceReference: "evidence://acme/implementation", Digest: strings.Repeat("1", 64),
	}})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-current", Topology: execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex", Digest: strings.Repeat("2", 64),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "2.0.0",
		SessionID: "session-current", SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("3", 64), NodeID: "implementation",
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: environment.Digest, Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://result", Digest: strings.Repeat("4", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV2, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{receipt},
	})
	if err != nil {
		t.Fatal(err)
	}
	return manifest, transcript
}
