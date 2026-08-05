package host_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNewConformanceTranscriptPinsCurrentHostFacts(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureProviderBindingInventory, host.FeatureNormalizedReceipts})
	inventory, report, session := currentHostFacts(t, manifest)
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion:      host.HostConformanceTranscriptSchemaV2,
		Session:            session,
		Inventory:          inventory,
		EnvironmentReports: []host.EnvironmentReport{report},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory.Observations[0].Topologies[0] = execution.TopologySubagent
	report.Observations[0].Surface = "changed"
	if transcript.Digest == "" || transcript.Session.Digest != session.Digest ||
		transcript.Inventory.Observations[0].Topologies[0] != execution.TopologyCurrent ||
		transcript.EnvironmentReports[0].Observations[0].Surface != "skills" {
		t.Fatalf("NewConformanceTranscript() = %#v", transcript)
	}
}

func TestValidateConformanceTranscriptVerifiesNormalizedCurrentReceipt(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureProviderBindingInventory, host.FeatureNormalizedReceipts})
	inventory, report, session := currentHostFacts(t, manifest)
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: report.Digest, DispatchDigest: strings.Repeat("8", 64), Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("f", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV2, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{report}, Receipts: []host.InvocationReceipt{receipt},
	})
	if err != nil {
		t.Fatal(err)
	}
	conformance, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil {
		t.Fatal(err)
	}
	if conformance.ManifestDigest != manifest.ContentDigest() || conformance.TranscriptDigest != transcript.Digest ||
		len(conformance.Diagnostics) != 0 || !slices.Equal(conformance.VerifiedFeatures, manifest.Features) {
		t.Fatalf("ValidateConformanceTranscript() = %#v", conformance)
	}
}

func TestValidateConformanceTranscriptRequiresPinnedSubagentEnvironment(t *testing.T) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []string{"skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.NewBindingInventory("codex", []host.BindingObservation{{
		HostID: "codex", InstallationKey: "installation-codex", Binding: catalog.HostBinding{
			Host: "codex", Kind: "skill", Reference: "acme:implementation",
			Topologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		}, Topologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}, Source: "native-probe",
		EvidenceReference: "evidence://codex/implementation", Digest: strings.Repeat("d", 64),
	}})
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
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "2.0.0",
		SessionID: "session-current", SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologySubagent, HostSessionDigest: session.Digest, InvocationHandle: "child-invocation-1",
		ContextFreshness: host.ContextFresh, EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("8", 64), Outcome: "succeeded",
		Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("f", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	newTranscript := func(value host.InvocationReceipt) host.ConformanceTranscript {
		t.Helper()
		transcript, transcriptErr := host.NewConformanceTranscript(host.ConformanceTranscript{
			SchemaVersion: host.HostConformanceTranscriptSchemaV2, Session: session, Inventory: inventory,
			EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{value},
		})
		if transcriptErr != nil {
			t.Fatal(transcriptErr)
		}
		return transcript
	}
	conformance, err := host.ValidateConformanceTranscript(manifest, newTranscript(receipt))
	if err != nil || !slices.Equal(conformance.VerifiedFeatures, manifest.Features) || len(conformance.Diagnostics) != 0 {
		t.Fatalf("ValidateConformanceTranscript(SUBAGENT) = %#v, %v", conformance, err)
	}
	unpinned := receipt
	unpinned.Digest = ""
	unpinned.EnvironmentReportDigest = strings.Repeat("9", 64)
	unpinned, err = host.NewInvocationReceipt(unpinned)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.ValidateConformanceTranscript(manifest, newTranscript(unpinned)); host.ErrorCode(err) != "HOST_CONFORMANCE_INVALID" {
		t.Fatalf("ValidateConformanceTranscript(unpinned SUBAGENT) error = %v", err)
	}
}

func TestValidateConformanceTranscriptVerifiesCanonicalControlReceipts(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{
		host.FeatureCancellation, host.FeatureInvocationDedup, host.FeatureNormalizedReceipts,
		host.FeaturePause, host.FeatureProviderBindingInventory,
	})
	inventory, environment, session := currentHostFacts(t, manifest)
	newReceipt := func(kind host.ReceiptKind, outcome string, evidence []host.EvidenceReference) host.InvocationReceipt {
		t.Helper()
		value := host.InvocationReceipt{
			SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: kind,
			WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
			Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared,
			EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("8", 64), Outcome: outcome, Evidence: evidence,
		}
		receipt, err := host.NewInvocationReceipt(value)
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	paused := newReceipt(host.ReceiptPaused, "paused", nil)
	cancelled := newReceipt(host.ReceiptCancelled, "cancelled", nil)
	completed := newReceipt(host.ReceiptCompleted, "succeeded", []host.EvidenceReference{{
		Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("f", 64),
	}})
	dispatchDigest := strings.Repeat("8", 64)
	transcript, err := host.NewConformanceTranscript(host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV2, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment},
		Receipts:           []host.InvocationReceipt{paused, cancelled, completed},
		Invocations: []host.InvocationRecord{
			{IdempotencyKey: "dedup-key", DispatchDigest: dispatchDigest, ReceiptDigest: completed.Digest},
			{IdempotencyKey: "a-key", DispatchDigest: dispatchDigest, ReceiptDigest: completed.Digest},
			{IdempotencyKey: "dedup-key", DispatchDigest: dispatchDigest, ReceiptDigest: completed.Digest},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if transcript.Invocations[0].IdempotencyKey != "a-key" {
		t.Fatalf("Invocation Records are not canonical: %#v", transcript.Invocations)
	}
	conformance, err := host.ValidateConformanceTranscript(manifest, transcript)
	if err != nil || len(conformance.Diagnostics) != 0 || !slices.Equal(conformance.VerifiedFeatures, manifest.Features) {
		t.Fatalf("ValidateConformanceTranscript(control receipts) = %#v, %v", conformance, err)
	}
}

func TestNewConformanceTranscriptRejectsOversizedCollections(t *testing.T) {
	manifest := hostNativeManifest(t, []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory})
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
	base := host.ConformanceTranscript{
		SchemaVersion: host.HostConformanceTranscriptSchemaV2, Session: session, Inventory: inventory,
		EnvironmentReports: []host.EnvironmentReport{environment},
	}
	tooManyReceipts := base
	tooManyReceipts.Receipts = make([]host.InvocationReceipt, 257)
	for index := range tooManyReceipts.Receipts {
		tooManyReceipts.Receipts[index] = receipt
	}
	if _, err := host.NewConformanceTranscript(tooManyReceipts); host.ErrorCode(err) != "HOST_CONFORMANCE_TRANSCRIPT_INVALID" {
		t.Fatalf("NewConformanceTranscript(oversized receipts) error = %v", err)
	}
	tooManyInvocations := base
	tooManyInvocations.Invocations = make([]host.InvocationRecord, 257)
	for index := range tooManyInvocations.Invocations {
		tooManyInvocations.Invocations[index] = host.InvocationRecord{
			IdempotencyKey: "key", DispatchDigest: strings.Repeat("8", 64), ReceiptDigest: receipt.Digest,
		}
	}
	tooManyInvocations.Receipts = []host.InvocationReceipt{receipt}
	if _, err := host.NewConformanceTranscript(tooManyInvocations); host.ErrorCode(err) != "HOST_CONFORMANCE_TRANSCRIPT_INVALID" {
		t.Fatalf("NewConformanceTranscript(oversized invocations) error = %v", err)
	}
}

func hostNativeManifest(t *testing.T, features []host.Feature) host.Manifest {
	t.Helper()
	value, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1},
		BindingKinds: []string{"skill"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: features,
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func currentHostFacts(t *testing.T, manifest host.Manifest) (host.BindingInventory, host.EnvironmentReport, host.SessionSnapshot) {
	t.Helper()
	inventory, err := host.NewBindingInventory("codex", []host.BindingObservation{{
		HostID: "codex", InstallationKey: "installation-codex", Binding: catalog.HostBinding{
			Host: "codex", Kind: "skill", Reference: "acme:implementation", Topologies: []execution.Topology{execution.TopologyCurrent},
		}, Topologies: []execution.Topology{execution.TopologyCurrent}, Source: "native-probe",
		EvidenceReference: "evidence://codex/implementation", Digest: strings.Repeat("d", 64),
	}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-current",
		Topology: execution.TopologyCurrent, Observations: []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex", Digest: strings.Repeat("e", 64),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "2.0.0",
		SessionID: "session-current", SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, EnvironmentReportDigest: report.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return inventory, report, session
}
