package hosttest

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

// These builders intentionally stop at normalized Host records. They provide
// deterministic test facts and never invoke a Host capability or external process.
func CurrentSession(t testing.TB, hostID string, inventoryDigest string) host.SessionSnapshot {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion:       host.HostManifestSchemaV3,
		ManifestVersion:     "3.0.0",
		HostID:              hostID,
		ControlSurface:      host.SurfaceHostNative,
		Protocols:           []string{host.WorkflowProtocolV1},
		BindingKinds:        []catalog.BindingKind{catalog.BindingAgent, catalog.BindingSkill, catalog.BindingTool},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{
			host.FeatureCancellation,
			host.FeatureEnvironmentReporting,
			host.FeatureInvocationDedup,
			host.FeatureNormalizedReceipts,
			host.FeaturePause,
			host.FeatureProviderBindingInventory,
		},
		DelegationFeatures: []host.FeatureID{},
		HostActions:        []host.HostActionContract{},
	})
	if err != nil {
		t.Fatalf("hosttest manifest: %v", err)
	}

	sessionID := "session-" + hostID
	environment := currentEnvironment(t, sessionID)
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV3,
		HostID:                  hostID,
		IntegrationID:           "oaw-test/" + hostID,
		IntegrationVersion:      "3.0.0",
		SessionID:               sessionID,
		ManifestDigest:          manifest.Digest,
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventoryDigest,
		FeatureObservations:     []host.FeatureObservation{},
		HostActionObservations:  []host.HostActionObservation{},
		EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatalf("hosttest session: %v", err)
	}
	return session
}

func CurrentEnvironment(t testing.TB, session host.SessionSnapshot) host.EnvironmentReport {
	t.Helper()
	return currentEnvironment(t, session.SessionID)
}

type ReceiptIdentity struct {
	WorkflowID              string
	BundleID                string
	BundleGeneration        uint64
	BundleDigest            string
	Cursor                  execution.GraphCursor
	Topology                execution.Topology
	HostSessionDigest       string
	DispatchDigest          string
	EnvironmentReportDigest string
}

func StartedReceipt(t testing.TB, identity ReceiptIdentity, handle string) host.InvocationReceipt {
	t.Helper()
	return newReceipt(t, identity, host.ReceiptStarted, handle, "", "", nil, nil)
}

func CompletedReceipt(t testing.TB, identity ReceiptIdentity, handle string, outputs []host.OutputReference, evidence []host.EvidenceReference) host.InvocationReceipt {
	t.Helper()
	return newReceipt(t, identity, host.ReceiptCompleted, handle, "succeeded", "", outputs, evidence)
}

func FailedReceipt(t testing.TB, identity ReceiptIdentity, handle, code string) host.InvocationReceipt {
	t.Helper()
	return newReceipt(t, identity, host.ReceiptFailed, handle, "failed", code, nil, []host.EvidenceReference{{
		Kind: "diagnostic", Reference: "evidence://hosttest/failure", Digest: strings.Repeat("f", 64),
	}})
}

func CancelledReceipt(t testing.TB, identity ReceiptIdentity, handle string) host.InvocationReceipt {
	t.Helper()
	return newReceipt(t, identity, host.ReceiptCancelled, handle, "cancelled", "", nil, nil)
}

func currentEnvironment(t testing.TB, sessionID string) host.EnvironmentReport {
	t.Helper()
	report, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2,
		SessionID:     sessionID,
		Topology:      execution.TopologyCurrent,
		Observations:  []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatalf("hosttest environment: %v", err)
	}
	return report
}

func newReceipt(t testing.TB, identity ReceiptIdentity, kind host.ReceiptKind, handle, outcome, failureCode string, outputs []host.OutputReference, evidence []host.EvidenceReference) host.InvocationReceipt {
	t.Helper()
	contextFreshness := host.ContextShared
	if identity.Topology == execution.TopologySubagent {
		contextFreshness = host.ContextFresh
	} else if identity.Topology == execution.TopologyCurrent && handle != "" {
		t.Fatalf("CURRENT receipt cannot contain an invocation handle")
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion:           host.HostInvocationReceiptSchemaV3,
		Kind:                    kind,
		WorkflowID:              identity.WorkflowID,
		BundleID:                identity.BundleID,
		BundleGeneration:        identity.BundleGeneration,
		BundleDigest:            identity.BundleDigest,
		Cursor:                  identity.Cursor,
		Topology:                identity.Topology,
		HostSessionDigest:       identity.HostSessionDigest,
		DispatchDigest:          identity.DispatchDigest,
		ContextFreshness:        contextFreshness,
		EnvironmentReportDigest: identity.EnvironmentReportDigest,
		InvocationHandle:        handle,
		Outcome:                 outcome,
		FailureCode:             failureCode,
		Outputs:                 append([]host.OutputReference{}, outputs...),
		Evidence:                append([]host.EvidenceReference{}, evidence...),
	})
	if err != nil {
		t.Fatalf("hosttest receipt: %v", err)
	}
	return receipt
}
