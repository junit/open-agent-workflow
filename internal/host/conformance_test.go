package host_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestConformanceV3PinsCurrentHostFactsAndReceiptV2(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureProviderBindingInventory, host.FeatureNormalizedReceipts})
	inventory, environment, session := currentHostFacts(t, manifest)
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("8", 64), Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("f", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV3, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{receipt},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil || report.SchemaVersion != host.HostConformanceReportSchemaV3 || report.TranscriptDigest != transcript.Digest ||
		len(report.Diagnostics) != 0 || !slices.Equal(report.VerifiedFeatures, manifest.Features) {
		t.Fatalf("ValidateConformanceTranscript() = %#v, %v", report, err)
	}
	inventory.Observations[0].Topologies[0] = execution.TopologySubagent
	environment.Observations[0].Surface = "changed"
	if transcript.Inventory.Observations[0].Topologies[0] != execution.TopologyCurrent || transcript.EnvironmentReports[0].Observations[0].Surface != "skills" {
		t.Fatal("Conformance Transcript shares caller storage")
	}
}

func TestConformanceV3RequiresPinnedSubagentEnvironment(t *testing.T) {
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
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-child", ParentSessionID: "session-current",
		Topology: execution.TopologySubagent, Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-subagent", Digest: strings.Repeat("e", 64),
		}},
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
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted, WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologySubagent, HostSessionDigest: session.Digest, InvocationHandle: "child-invocation-1", ContextFreshness: host.ContextFresh,
		EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("8", 64), Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("f", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	newTranscript := func(value host.InvocationReceipt) host.ConformanceTranscript {
		t.Helper()
		transcript, transcriptErr := host.NewConformanceTranscript(host.ConformanceTranscript{
			SchemaVersion: host.HostConformanceTranscriptSchemaV3, Session: session, Inventory: inventory,
			EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{value},
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

func TestConformanceV3VerifiesControlReceiptsAndDeduplication(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{
		host.FeatureCancellation, host.FeatureInvocationDedup, host.FeatureNormalizedReceipts,
		host.FeaturePause, host.FeatureProviderBindingInventory,
	})
	inventory, environment, session := currentHostFacts(t, manifest)
	newReceipt := func(kind host.ReceiptKind, outcome string, evidence []host.EvidenceReference) host.InvocationReceipt {
		t.Helper()
		value, err := host.NewInvocationReceipt(host.InvocationReceipt{
			SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: kind, WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
			Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared, EnvironmentReportDigest: environment.Digest,
			DispatchDigest: strings.Repeat("8", 64), Outcome: outcome, Evidence: evidence,
		})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	completed := newReceipt(host.ReceiptCompleted, "succeeded", []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("f", 64)}})
	paused := newReceipt(host.ReceiptPaused, "paused", nil)
	cancelled := newReceipt(host.ReceiptCancelled, "cancelled", nil)
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV3, Session: session, Inventory: inventory, EnvironmentReports: []host.EnvironmentReport{environment},
		Receipts: []host.InvocationReceipt{paused, cancelled, completed}, Invocations: []host.InvocationRecord{
			{IdempotencyKey: "dedup-key", DispatchDigest: strings.Repeat("8", 64), ReceiptDigest: completed.Digest},
			{IdempotencyKey: "a-key", DispatchDigest: strings.Repeat("8", 64), ReceiptDigest: completed.Digest},
			{IdempotencyKey: "dedup-key", DispatchDigest: strings.Repeat("8", 64), ReceiptDigest: completed.Digest},
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

func TestConformanceV3RejectsOversizedCollections(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory})
	inventory, environment, session := currentHostFacts(t, manifest)
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted, WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared, EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("8", 64), Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("f", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	base := host.ConformanceTranscript{SchemaVersion: host.HostConformanceTranscriptSchemaV3, Session: session, Inventory: inventory, EnvironmentReports: []host.EnvironmentReport{environment}}
	tooManyReceipts := base
	tooManyReceipts.Receipts = make([]host.InvocationReceipt, 257)
	for index := range tooManyReceipts.Receipts {
		tooManyReceipts.Receipts[index] = receipt
	}
	if _, err := host.NewConformanceTranscript(tooManyReceipts); host.ErrorCode(err) != "HOST_CONFORMANCE_TRANSCRIPT_INVALID" {
		t.Fatalf("oversized receipts error = %v", err)
	}
	tooManyInvocations := base
	tooManyInvocations.Invocations = make([]host.InvocationRecord, 257)
	for index := range tooManyInvocations.Invocations {
		tooManyInvocations.Invocations[index] = host.InvocationRecord{IdempotencyKey: "key", DispatchDigest: strings.Repeat("8", 64), ReceiptDigest: receipt.Digest}
	}
	tooManyInvocations.Receipts = []host.InvocationReceipt{receipt}
	if _, err := host.NewConformanceTranscript(tooManyInvocations); host.ErrorCode(err) != "HOST_CONFORMANCE_TRANSCRIPT_INVALID" {
		t.Fatalf("oversized invocations error = %v", err)
	}
}

func hostNativeManifest(t *testing.T, features []host.Feature) host.Manifest {
	t.Helper()
	value, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex", ControlSurface: host.SurfaceHostNative,
		Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: features, DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
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
