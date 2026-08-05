package host_test

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestNewInvocationReceiptPinsCurrentCompletion(t *testing.T) {
	evidence := []host.EvidenceReference{{
		Kind: "test-report", Reference: "evidence://workflow/test-report", Digest: strings.Repeat("e", 64),
	}}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion:           host.HostInvocationReceiptSchemaV2,
		Kind:                    host.ReceiptCompleted,
		WorkflowID:              "workflow-1",
		BundleGeneration:        2,
		BundleDigest:            strings.Repeat("a", 64),
		NodeID:                  "implementation",
		Topology:                execution.TopologyCurrent,
		HostSessionDigest:       strings.Repeat("b", 64),
		ContextFreshness:        host.ContextShared,
		EnvironmentReportDigest: strings.Repeat("c", 64),
		Outcome:                 "succeeded",
		Evidence:                evidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence[0].Reference = "changed"
	if receipt.Digest == "" || receipt.ContextFreshness != host.ContextShared || receipt.InvocationHandle != "" ||
		len(receipt.Evidence) != 1 || receipt.Evidence[0].Reference != "evidence://workflow/test-report" {
		t.Fatalf("NewInvocationReceipt() = %#v", receipt)
	}
}

func TestNewInvocationReceiptRequiresSubagentHandleAndFreshContext(t *testing.T) {
	input := host.InvocationReceipt{
		SchemaVersion:           host.HostInvocationReceiptSchemaV2,
		Kind:                    host.ReceiptCompleted,
		WorkflowID:              "workflow-1",
		BundleGeneration:        2,
		BundleDigest:            strings.Repeat("a", 64),
		NodeID:                  "implementation",
		Topology:                execution.TopologySubagent,
		HostSessionDigest:       strings.Repeat("b", 64),
		InvocationHandle:        "child-invocation-1",
		ContextFreshness:        host.ContextFresh,
		EnvironmentReportDigest: strings.Repeat("c", 64),
		Outcome:                 "succeeded",
		Evidence: []host.EvidenceReference{{
			Kind: "test-report", Reference: "evidence://workflow/test-report", Digest: strings.Repeat("e", 64),
		}},
	}
	if _, err := host.NewInvocationReceipt(input); err != nil {
		t.Fatalf("NewInvocationReceipt(SUBAGENT) error = %v", err)
	}
	missingHandle := input
	missingHandle.InvocationHandle = ""
	if _, err := host.NewInvocationReceipt(missingHandle); host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
		t.Fatalf("NewInvocationReceipt(missing handle) error = %v", err)
	}
	shared := input
	shared.ContextFreshness = host.ContextShared
	if _, err := host.NewInvocationReceipt(shared); host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
		t.Fatalf("NewInvocationReceipt(shared SUBAGENT) error = %v", err)
	}
}

func TestNewInvocationReceiptAcceptsClosedKinds(t *testing.T) {
	base := host.InvocationReceipt{
		SchemaVersion:           host.HostInvocationReceiptSchemaV2,
		WorkflowID:              "workflow-1",
		BundleGeneration:        2,
		BundleDigest:            strings.Repeat("a", 64),
		NodeID:                  "implementation",
		Topology:                execution.TopologyCurrent,
		HostSessionDigest:       strings.Repeat("b", 64),
		ContextFreshness:        host.ContextShared,
		EnvironmentReportDigest: strings.Repeat("c", 64),
	}
	for _, test := range []struct {
		name  string
		kind  host.ReceiptKind
		setup func(*host.InvocationReceipt)
	}{
		{name: "started", kind: host.ReceiptStarted},
		{name: "paused", kind: host.ReceiptPaused, setup: func(value *host.InvocationReceipt) { value.Outcome = "paused" }},
		{name: "completed", kind: host.ReceiptCompleted, setup: func(value *host.InvocationReceipt) {
			value.Outcome = "succeeded"
			value.Evidence = []host.EvidenceReference{{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("e", 64)}}
		}},
		{name: "failed", kind: host.ReceiptFailed, setup: func(value *host.InvocationReceipt) {
			value.Outcome = "failed"
			value.FailureCode = "BUILD_FAILED"
			value.Evidence = []host.EvidenceReference{{Kind: "diagnostic", Reference: "evidence://failure", Digest: strings.Repeat("f", 64)}}
		}},
		{name: "cancelled", kind: host.ReceiptCancelled, setup: func(value *host.InvocationReceipt) { value.Outcome = "cancelled" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := base
			value.Kind = test.kind
			if test.setup != nil {
				test.setup(&value)
			}
			if _, err := host.NewInvocationReceipt(value); err != nil {
				t.Fatalf("NewInvocationReceipt(%s) error = %v", test.name, err)
			}
		})
	}
	unknown := base
	unknown.Kind = "UNKNOWN"
	if _, err := host.NewInvocationReceipt(unknown); host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
		t.Fatalf("NewInvocationReceipt(unknown) error = %v", err)
	}
}

func TestNewInvocationReceiptRejectsOversizedEvidence(t *testing.T) {
	evidence := make([]host.EvidenceReference, 129)
	for index := range evidence {
		evidence[index] = host.EvidenceReference{
			Kind: "report", Reference: "evidence://report/" + strings.Repeat("x", index+1), Digest: strings.Repeat("e", 64),
		}
	}
	_, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV2, Kind: host.ReceiptCompleted,
		WorkflowID: "workflow-1", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), NodeID: "implementation",
		Topology: execution.TopologyCurrent, HostSessionDigest: strings.Repeat("b", 64), ContextFreshness: host.ContextShared,
		EnvironmentReportDigest: strings.Repeat("c", 64), Outcome: "succeeded", Evidence: evidence,
	})
	if host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
		t.Fatalf("NewInvocationReceipt(oversized evidence) error = %v", err)
	}
}
