package hosttest

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

// These builders intentionally stop at normalized Host records. They provide
// deterministic test facts and never invoke a Host capability or external process.
func CurrentSession(t testing.TB, hostID string, inventoryDigest string) host.SessionSnapshot {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion:       host.HostManifestSchemaV2,
		ManifestVersion:     "2.0.0",
		HostID:              hostID,
		ControlSurface:      host.SurfaceHostNative,
		Protocols:           []string{host.WorkflowProtocolV1},
		BindingKinds:        []string{"agent", "skill", "tool"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{
			host.FeatureCancellation,
			host.FeatureEnvironmentReporting,
			host.FeatureInvocationDedup,
			host.FeatureNormalizedReceipts,
			host.FeaturePause,
			host.FeatureProviderBindingInventory,
		},
	})
	if err != nil {
		t.Fatalf("hosttest manifest: %v", err)
	}

	sessionID := "session-" + hostID
	environment := currentEnvironment(t, sessionID)
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion:           host.HostSessionSchemaV2,
		HostID:                  hostID,
		IntegrationID:           "oaw-test/" + hostID,
		IntegrationVersion:      "2.0.0",
		SessionID:               sessionID,
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventoryDigest,
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
	BundleGeneration        uint64
	BundleDigest            string
	NodeID                  string
	Topology                execution.Topology
	HostSessionDigest       string
	DispatchDigest          string
	EnvironmentReportDigest string
}

func StartedReceipt(t testing.TB, identity ReceiptIdentity, handle string) host.InvocationReceipt {
	t.Helper()
	return newReceipt(t, identity, host.ReceiptStarted, handle, "", "", nil)
}

func CompletedReceipt(t testing.TB, identity ReceiptIdentity, handle string, evidence []host.EvidenceReference) host.InvocationReceipt {
	t.Helper()
	return newReceipt(t, identity, host.ReceiptCompleted, handle, "succeeded", "", evidence)
}

func FailedReceipt(t testing.TB, identity ReceiptIdentity, handle, code string) host.InvocationReceipt {
	t.Helper()
	return newReceipt(t, identity, host.ReceiptFailed, handle, "failed", code, []host.EvidenceReference{{
		Kind: "diagnostic", Reference: "evidence://hosttest/failure", Digest: strings.Repeat("f", 64),
	}})
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

func newReceipt(t testing.TB, identity ReceiptIdentity, kind host.ReceiptKind, handle, outcome, failureCode string, evidence []host.EvidenceReference) host.InvocationReceipt {
	t.Helper()
	contextFreshness := host.ContextShared
	if identity.Topology == execution.TopologySubagent {
		contextFreshness = host.ContextFresh
	} else if identity.Topology == execution.TopologyCurrent && handle != "" {
		t.Fatalf("CURRENT receipt cannot contain an invocation handle")
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion:           host.HostInvocationReceiptSchemaV2,
		Kind:                    kind,
		WorkflowID:              identity.WorkflowID,
		BundleGeneration:        identity.BundleGeneration,
		BundleDigest:            identity.BundleDigest,
		NodeID:                  identity.NodeID,
		Topology:                identity.Topology,
		HostSessionDigest:       identity.HostSessionDigest,
		DispatchDigest:          identity.DispatchDigest,
		ContextFreshness:        contextFreshness,
		EnvironmentReportDigest: identity.EnvironmentReportDigest,
		InvocationHandle:        handle,
		Outcome:                 outcome,
		FailureCode:             failureCode,
		Evidence:                append([]host.EvidenceReference{}, evidence...),
	})
	if err != nil {
		t.Fatalf("hosttest receipt: %v", err)
	}
	return receipt
}
