package coordinator_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestDecodeCommandAcceptsClosedCommandKinds(t *testing.T) {
	session, environment := validHostFacts(t)
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptStarted,
		WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		command coordinator.Command
	}{
		{name: "start", command: validStartCommand(t)},
		{name: "inspect", command: coordinator.Command{SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandInspect, WorkflowID: "workflow-1"}},
		{name: "prepare", command: coordinator.Command{SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandPrepare, MessageID: "message-1", IdempotencyKey: "prepare-1", WorkflowID: "workflow-1", ExpectedRevision: 1, Prepare: &coordinator.PrepareInput{RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"}, TerminationCondition: "complete", InputReferences: []coordinator.ArtifactReference{}, EvidenceRequirements: []coordinator.EvidenceRequirement{}}}},
		{name: "receipt", command: coordinator.Command{SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandReceipt, MessageID: "message-2", IdempotencyKey: "receipt-1", WorkflowID: "workflow-1", ExpectedRevision: 2, Receipt: &coordinator.ReceiptInput{Receipt: receipt}}},
		{name: "switch", command: coordinator.Command{SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandSwitch, MessageID: "message-3", IdempotencyKey: "switch-1", WorkflowID: "workflow-1", ExpectedRevision: 3, Switch: &coordinator.SwitchInput{Boundary: "spec-complete", Selection: validSelection(), HostSession: session, Environment: environment}}},
		{name: "cancel", command: coordinator.Command{SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandCancel, MessageID: "message-4", IdempotencyKey: "cancel-1", WorkflowID: "workflow-1", ExpectedRevision: 4, Cancel: &coordinator.CancelInput{Reason: "user-requested", InvocationTerminal: true}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := mustMarshal(t, test.command)
			command, err := coordinator.DecodeCommand(raw)
			if err != nil {
				t.Fatalf("DecodeCommand() error = %v", err)
			}
			if command.SchemaVersion != coordinator.WorkflowCommandSchemaV1 || command.Kind == "" {
				t.Fatalf("DecodeCommand() = %#v", command)
			}
		})
	}
}

func TestDecodeCommandRejectsInvalidUnionAndTransportInput(t *testing.T) {
	valid := string(mustMarshal(t, validStartCommand(t)))
	wrongPayload := coordinator.Command{
		SchemaVersion:  coordinator.WorkflowCommandSchemaV1,
		Kind:           coordinator.CommandStart,
		MessageID:      "message-start",
		IdempotencyKey: "start-1",
		Prepare: &coordinator.PrepareInput{
			RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"},
			TerminationCondition: "complete", InputReferences: []coordinator.ArtifactReference{},
			EvidenceRequirements: []coordinator.EvidenceRequirement{},
		},
	}
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{name: "unknown field", raw: []byte(strings.Replace(valid, `"start":`, `"unknown":true,"start":`, 1))},
		{name: "mixed payload", raw: []byte(strings.Replace(valid, `"start":`, `"prepare":{},"start":`, 1))},
		{name: "wrong single payload", raw: mustMarshal(t, wrongPayload)},
		{name: "missing idempotency", raw: []byte(strings.Replace(valid, `"idempotency_key":"start-1",`, "", 1))},
		{name: "stale revision", raw: []byte(strings.Replace(valid, `"expected_revision":0`, `"expected_revision":1`, 1))},
		{name: "trailing JSON", raw: append([]byte(valid), []byte(` {}`)...)},
		{name: "invalid UTF-8", raw: []byte{0xff, 0xfe}},
		{name: "old runtime frame", raw: []byte(`{"schema_version":"oaw.runtime/v1","kind":"START"}`)},
		{name: "oversized", raw: []byte(strings.Repeat("x", coordinator.MaximumProtocolFrameBytes+1))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := coordinator.DecodeCommand(test.raw); err == nil {
				t.Fatal("DecodeCommand() unexpectedly accepted invalid input")
			}
		})
	}
}

func TestWorkflowRecordsRejectCorruptContentDigests(t *testing.T) {
	valid := string(mustMarshal(t, validStartCommand(t)))
	for _, test := range []struct {
		name string
		raw  string
	}{
		{name: "invalid input digest", raw: strings.Replace(valid, `"input_digest":"`+strings.Repeat("a", 64)+`"`, `"input_digest":"bad"`, 1)},
		{name: "forged Host session fact", raw: strings.Replace(valid, `"provider_inventory_digest":"`+strings.Repeat("a", 64)+`"`, `"provider_inventory_digest":"`+strings.Repeat("b", 64)+`"`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := coordinator.DecodeCommand([]byte(test.raw)); err == nil {
				t.Fatal("DecodeCommand() accepted a forged digest")
			}
		})
	}
}

func TestReceiptRejectsSecretBearingOrUnknownFields(t *testing.T) {
	session, environment := validHostFacts(t)
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptStarted,
		WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	command := coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandReceipt,
		MessageID: "message-receipt", IdempotencyKey: "receipt-1", WorkflowID: "workflow-1", ExpectedRevision: 2,
		Receipt: &coordinator.ReceiptInput{Receipt: receipt},
	}
	valid := string(mustMarshal(t, command))
	marker := `"receipt":{"receipt":{`
	for _, field := range []string{`"credentials":"secret",`, `"raw_output":"secret",`, `"unknown":true,`} {
		t.Run(field, func(t *testing.T) {
			raw := strings.Replace(valid, marker, marker+field, 1)
			if raw == valid {
				t.Fatal("Receipt insertion marker was not found")
			}
			if _, err := coordinator.DecodeCommand([]byte(raw)); coordinator.ErrorCode(err) != "WORKFLOW_COMMAND_DECODE_INVALID" {
				t.Fatalf("DecodeCommand(secret-bearing or unknown Receipt field) error = %v", err)
			}
		})
	}
}

func TestDecodeCommandRejectsRuntimeSchemaExplicitly(t *testing.T) {
	for _, raw := range []string{
		`{"schema_version":"oaw.runtime/v1","kind":"START","message_id":"message-1","idempotency_key":"start-1","workflow_id":"","expected_revision":0,"start":{}}`,
		`{"schema_version":"oaw.runtime/v1","kind":"RECEIPT","message_id":"message-1","idempotency_key":"receipt-1","workflow_id":"workflow-1","expected_revision":1,"receipt":{"receipt":{}}}`,
	} {
		_, err := coordinator.DecodeCommand([]byte(raw))
		if coordinator.ErrorCode(err) != "SCHEMA_UNSUPPORTED" {
			t.Fatalf("ErrorCode() = %q, error = %v", coordinator.ErrorCode(err), err)
		}
	}
}

func TestEncodeResultProducesCanonicalClosedUnion(t *testing.T) {
	encoded, err := coordinator.EncodeResult(coordinator.Result{
		SchemaVersion: coordinator.WorkflowResultSchemaV1,
		Kind:          coordinator.ResultRejected,
		Diagnostics:   []coordinator.Diagnostic{{Code: "WORKFLOW_DENIED", Detail: "selection required"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schema_version"] != coordinator.WorkflowResultSchemaV1 || decoded["kind"] != "REJECTED" || len(encoded) == 0 {
		t.Fatalf("encoded Result = %s", encoded)
	}
	if _, err := coordinator.EncodeResult(coordinator.Result{
		SchemaVersion: coordinator.WorkflowResultSchemaV1,
		Kind:          coordinator.ResultRejected,
		Diagnostics:   []coordinator.Diagnostic{{Code: "WORKFLOW_DENIED", Detail: "selection required"}},
		Digest:        strings.Repeat("0", 64),
	}); err == nil {
		t.Fatal("EncodeResult() accepted forged digest")
	}
}

func validStartCommand(t testing.TB) coordinator.Command {
	t.Helper()
	session, environment := validHostFacts(t)
	return coordinator.Command{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: "start-1",
		Start: &coordinator.StartInput{
			RequestID: "request-1", DeliverableID: "deliverable-1", InputDigest: strings.Repeat("a", 64), ActiveTicket: "ticket-1",
			Proposal:  classification.ClassificationProposal{SchemaVersion: classification.ProposalSchemaV1, Traits: []classification.TraitObservation{}, Resources: []classification.Resource{}, Evidence: []classification.ProposalEvidence{}},
			Selection: validSelection(), HostSession: session, Environment: environment,
		},
	}
}

func validSelection() core.Selection {
	return core.Selection{
		Profile: "SP-FULL", ProfileSource: core.SelectionUser, Topology: execution.TopologyCurrent,
		TopologySource: core.SelectionHostOnlyOption, AddOns: []string{}, Bindings: nil,
	}
}

func validHostFacts(t testing.TB) (host.SessionSnapshot, host.EnvironmentReport) {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []string{"skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-1", Topology: execution.TopologyCurrent,
		Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV2, HostID: "codex", IntegrationID: "acme/codex-host", IntegrationVersion: "2.0.0",
		SessionID: "session-1", SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: strings.Repeat("a", 64), EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return session, environment
}

func mustMarshal(t testing.TB, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
