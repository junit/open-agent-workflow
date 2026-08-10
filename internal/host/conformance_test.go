package host_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestConformanceV4PinsHostV3FactsAndReceiptV3(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureProviderBindingInventory, host.FeatureNormalizedReceipts})
	inventory, environment, session := currentHostFacts(t, manifest)
	receipt := validReceiptV3(host.ReceiptCompleted)
	receipt.HostSessionDigest = session.Digest
	receipt.EnvironmentReportDigest = environment.Digest
	receipt, err := host.NewInvocationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV4, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{receipt}, Invocations: []host.InvocationRecord{},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil || report.SchemaVersion != host.HostConformanceReportSchemaV4 || report.TranscriptDigest != transcript.Digest ||
		len(report.Diagnostics) != 0 || !slices.Equal(report.VerifiedFeatures, manifest.Features) {
		t.Fatalf("ValidateConformanceTranscript() = %#v, %v", report, err)
	}
	inventory.Observations[0].Topologies[0] = execution.TopologySubagent
	environment.Observations[0].Surface = "changed"
	receipt.Outputs[0].Reference = "changed"
	if transcript.Inventory.Observations[0].Topologies[0] != execution.TopologyCurrent || transcript.EnvironmentReports[0].Observations[0].Surface != "skills" ||
		transcript.Receipts[0].Outputs[0].Reference == "changed" {
		t.Fatal("Conformance Transcript shares caller storage")
	}
}

func TestConformanceV4RequiresPinnedSubagentEnvironment(t *testing.T) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		DelegationFeatures:  []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := host.NewBindingObservation(host.BindingObservation{
		HostID: "codex", ProviderID: "oaw/provider", InstallationKey: "installation-codex", DistributionID: "distribution", BindingID: "binding-skill",
		Surface: "codex", Kind: catalog.BindingSkill, Reference: "provider:skill", Invocation: catalog.InvocationModel,
		BindingTreeDigest: "sha256:" + strings.Repeat("a", 64), Topologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Source: host.SourceNativeAPI, EvidenceReference: "evidence://codex/bindings/skill",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", []host.BindingObservation{binding})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-child", ParentSessionID: "session-current", Topology: execution.TopologySubagent,
		Observations: []execution.EnvironmentObservation{{Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-subagent", Digest: strings.Repeat("e", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "3.0.0", SessionID: "session-current",
		ManifestDigest: manifest.Digest, SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: inventory.Digest, FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := validReceiptV3(host.ReceiptCompleted)
	receipt.Topology = execution.TopologySubagent
	receipt.HostSessionDigest = session.Digest
	receipt.InvocationHandle = "child-invocation-1"
	receipt.ContextFreshness = host.ContextFresh
	receipt.EnvironmentReportDigest = environment.Digest
	receipt, err = host.NewInvocationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	newTranscript := func(value host.InvocationReceipt) host.ConformanceTranscript {
		t.Helper()
		transcript, transcriptErr := host.NewConformanceTranscript(host.ConformanceTranscript{
			SchemaVersion: host.HostConformanceTranscriptSchemaV4, Session: session, Inventory: inventory,
			EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{value}, Invocations: []host.InvocationRecord{},
		})
		if transcriptErr != nil {
			t.Fatal(transcriptErr)
		}
		return transcript
	}
	if report, err := host.ValidateConformanceTranscript(manifest, newTranscript(receipt)); err != nil || len(report.Diagnostics) != 0 {
		t.Fatalf("ValidateConformanceTranscript(SUBAGENT) = %#v, %v", report, err)
	}
	unpinned := receipt
	unpinned.Digest = ""
	unpinned.EnvironmentReportDigest = strings.Repeat("9", 64)
	unpinned, err = host.NewInvocationReceipt(unpinned)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.ValidateConformanceTranscript(manifest, newTranscript(unpinned)); host.ErrorCode(err) != "HOST_CONFORMANCE_INVALID" {
		t.Fatalf("unpinned receipt error = %v", err)
	}
}

func TestConformanceV4VerifiesControlReceiptsAndDeduplication(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{
		host.FeatureCancellation, host.FeatureInvocationDedup, host.FeatureNormalizedReceipts, host.FeaturePause, host.FeatureProviderBindingInventory,
	})
	inventory, environment, session := currentHostFacts(t, manifest)
	newReceipt := func(kind host.ReceiptKind) host.InvocationReceipt {
		t.Helper()
		value := validReceiptV3(kind)
		value.HostSessionDigest = session.Digest
		value.EnvironmentReportDigest = environment.Digest
		result, err := host.NewInvocationReceipt(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	completed := newReceipt(host.ReceiptCompleted)
	paused := newReceipt(host.ReceiptPaused)
	cancelled := newReceipt(host.ReceiptCancelled)
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV4, Session: session, Inventory: inventory, EnvironmentReports: []host.EnvironmentReport{environment},
		Receipts: []host.InvocationReceipt{paused, cancelled, completed}, Invocations: []host.InvocationRecord{
			{IdempotencyKey: "dedup-key", DispatchDigest: strings.Repeat("d", 64), ReceiptDigest: completed.Digest},
			{IdempotencyKey: "a-key", DispatchDigest: strings.Repeat("d", 64), ReceiptDigest: completed.Digest},
			{IdempotencyKey: "dedup-key", DispatchDigest: strings.Repeat("d", 64), ReceiptDigest: completed.Digest},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil || len(report.Diagnostics) != 0 || !slices.Equal(report.VerifiedFeatures, manifest.Features) {
		t.Fatalf("ValidateConformanceTranscript() = %#v, %v", report, err)
	}
}

func TestConformanceV4RejectsOversizedAndOldAuthority(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory})
	inventory, environment, session := currentHostFacts(t, manifest)
	base := host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV4, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{}, Invocations: []host.InvocationRecord{},
	}
	tooMany := base
	tooMany.Invocations = make([]host.InvocationRecord, 257)
	if _, err := host.NewConformanceTranscript(tooMany); host.ErrorCode(err) != "HOST_CONFORMANCE_TRANSCRIPT_INVALID" {
		t.Fatalf("oversized transcript error = %v", err)
	}
	old := base
	old.SchemaVersion = "oaw.host-conformance-transcript/v3"
	if _, err := host.NewConformanceTranscript(old); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewConformanceTranscript(v3) error = %v", err)
	}
	report := host.ConformanceReport{SchemaVersion: "oaw.host-conformance-report/v3"}
	if _, err := host.NewConformanceReport(report); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewConformanceReport(v3) error = %v", err)
	}
}

func hostNativeManifest(t *testing.T, features []host.Feature) host.Manifest {
	t.Helper()
	value, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: features, DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func currentHostFacts(t *testing.T, manifest host.Manifest) (host.BindingInventory, host.EnvironmentReport, host.SessionSnapshot) {
	t.Helper()
	binding, err := host.NewBindingObservation(host.BindingObservation{
		HostID: "codex", ProviderID: "oaw/provider", InstallationKey: "installation-codex", DistributionID: "distribution", BindingID: "binding-skill",
		Surface: "codex", Kind: catalog.BindingSkill, Reference: "provider:skill", Invocation: catalog.InvocationModel,
		BindingTreeDigest: "sha256:" + strings.Repeat("d", 64), Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/bindings/skill",
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", []host.BindingObservation{binding})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-current", Topology: execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{{Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex", Digest: strings.Repeat("e", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "3.0.0", SessionID: "session-current", ManifestDigest: manifest.Digest,
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, ProviderInventoryDigest: inventory.Digest, FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{}, EnvironmentReportDigest: report.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return inventory, report, session
}
