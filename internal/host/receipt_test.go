package host_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestReceiptV3PinsCursorTypedOutputsAndEvidence(t *testing.T) {
	input := validReceiptV3(host.ReceiptCompleted)
	receipt, err := host.NewInvocationReceipt(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Outputs[0].Reference = "changed"
	input.Evidence[0].Reference = "changed"
	if receipt.SchemaVersion != host.HostInvocationReceiptSchemaV3 || receipt.Cursor.Kind != execution.CursorBinding ||
		len(receipt.Outputs) != 1 || receipt.Outputs[0].ArtifactID != "workflow-output" || receipt.Outputs[0].Schema != "oaw.workflow-output/v1" ||
		receipt.Outputs[0].Reference != "artifact://workflow/output/1" || receipt.Evidence[0].Reference != "evidence://workflow/test-report" || receipt.Digest == "" {
		t.Fatalf("NewInvocationReceipt() = %#v", receipt)
	}
	cloned := host.CloneInvocationReceipt(receipt)
	cloned.Outputs[0].Reference = "changed"
	cloned.Evidence[0].Reference = "changed"
	if receipt.Outputs[0].Reference == "changed" || receipt.Evidence[0].Reference == "changed" {
		t.Fatal("CloneInvocationReceipt aliases nested storage")
	}
	roundTrip, err := host.NewInvocationReceipt(receipt)
	if err != nil || !reflect.DeepEqual(roundTrip, receipt) {
		t.Fatalf("NewInvocationReceipt(canonical) = %#v, %v", roundTrip, err)
	}
}

func TestReceiptV3RequiresSubagentHandleAndFreshContext(t *testing.T) {
	input := validReceiptV3(host.ReceiptCompleted)
	input.Topology = execution.TopologySubagent
	input.InvocationHandle = "child-invocation-1"
	input.ContextFreshness = host.ContextFresh
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

func TestReceiptV3EnforcesClosedKindOutputRules(t *testing.T) {
	for _, test := range []struct {
		kind host.ReceiptKind
		want bool
	}{
		{kind: host.ReceiptStarted, want: true},
		{kind: host.ReceiptPaused, want: true},
		{kind: host.ReceiptCompleted, want: true},
		{kind: host.ReceiptFailed, want: true},
		{kind: host.ReceiptCancelled, want: true},
		{kind: "INVENTED", want: false},
	} {
		t.Run(string(test.kind), func(t *testing.T) {
			_, err := host.NewInvocationReceipt(validReceiptV3(test.kind))
			if (err == nil) != test.want {
				t.Fatalf("NewInvocationReceipt(%s) error = %v", test.kind, err)
			}
		})
	}

	for _, kind := range []host.ReceiptKind{host.ReceiptStarted, host.ReceiptPaused, host.ReceiptFailed, host.ReceiptCancelled} {
		value := validReceiptV3(kind)
		value.Outputs = validOutputs()
		if _, err := host.NewInvocationReceipt(value); host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
			t.Fatalf("NewInvocationReceipt(%s with Outputs) error = %v", kind, err)
		}
	}
	completed := validReceiptV3(host.ReceiptCompleted)
	completed.Outputs = nil
	if _, err := host.NewInvocationReceipt(completed); host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
		t.Fatalf("NewInvocationReceipt(COMPLETED without Outputs) error = %v", err)
	}
	completed = validReceiptV3(host.ReceiptCompleted)
	completed.Evidence = nil
	if _, err := host.NewInvocationReceipt(completed); host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
		t.Fatalf("NewInvocationReceipt(COMPLETED without evidence) error = %v", err)
	}
}

func TestReceiptV3RejectsMalformedCursorAndOutput(t *testing.T) {
	for _, mutate := range []func(*host.InvocationReceipt){
		func(value *host.InvocationReceipt) { value.WorkflowID = "workflow-1" },
		func(value *host.InvocationReceipt) { value.Cursor.Ordinal = 0 },
		func(value *host.InvocationReceipt) { value.Cursor.Kind = execution.CursorGate },
		func(value *host.InvocationReceipt) { value.Outputs[0].ArtifactID = "" },
		func(value *host.InvocationReceipt) { value.Outputs[0].Schema = "" },
		func(value *host.InvocationReceipt) { value.Outputs[0].Reference = "" },
		func(value *host.InvocationReceipt) { value.Outputs[0].Digest = "bad" },
		func(value *host.InvocationReceipt) { value.Outputs = append(value.Outputs, value.Outputs[0]) },
		func(value *host.InvocationReceipt) {
			rewritten := value.Evidence[0]
			rewritten.Digest = strings.Repeat("0", 64)
			value.Evidence = append(value.Evidence, rewritten)
		},
	} {
		value := validReceiptV3(host.ReceiptCompleted)
		mutate(&value)
		if _, err := host.NewInvocationReceipt(value); host.ErrorCode(err) != "HOST_INVOCATION_RECEIPT_INVALID" {
			t.Fatalf("NewInvocationReceipt(%#v) error = %v", value, err)
		}
	}
}

func TestOldAuthorityReceiptV2IsRejected(t *testing.T) {
	value := validReceiptV3(host.ReceiptStarted)
	value.SchemaVersion = "oaw.host-invocation-receipt/v2"
	if _, err := host.NewInvocationReceipt(value); host.ErrorCode(err) != "HOST_SCHEMA_UNSUPPORTED" {
		t.Fatalf("NewInvocationReceipt(v2) error = %v", err)
	}
}

func validReceiptV3(kind host.ReceiptKind) host.InvocationReceipt {
	value := host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: kind, WorkflowID: "workflow-0123456789abcdef0123456789abcdef",
		BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleGeneration: 2, BundleDigest: strings.Repeat("a", 64),
		Cursor:   execution.GraphCursor{SlotID: "implementation", Kind: execution.CursorBinding, UnitID: "implementation-main", Ordinal: 1},
		Topology: execution.TopologyCurrent, HostSessionDigest: strings.Repeat("b", 64), DispatchDigest: strings.Repeat("d", 64),
		ContextFreshness: host.ContextShared, EnvironmentReportDigest: strings.Repeat("c", 64), Outputs: []host.OutputReference{}, Evidence: []host.EvidenceReference{},
	}
	switch kind {
	case host.ReceiptPaused:
		value.Outcome = "paused"
	case host.ReceiptCompleted:
		value.Outcome = "succeeded"
		value.Outputs = validOutputs()
		value.Evidence = []host.EvidenceReference{{Kind: "test-report", Reference: "evidence://workflow/test-report", Digest: strings.Repeat("e", 64)}}
	case host.ReceiptFailed:
		value.Outcome = "failed"
		value.FailureCode = "BUILD_FAILED"
		value.Evidence = []host.EvidenceReference{{Kind: "diagnostic", Reference: "evidence://workflow/failure", Digest: strings.Repeat("f", 64)}}
	case host.ReceiptCancelled:
		value.Outcome = "cancelled"
	}
	return value
}

func validOutputs() []host.OutputReference {
	return []host.OutputReference{{
		ArtifactID: "workflow-output", Schema: "oaw.workflow-output/v1", Reference: "artifact://workflow/output/1", Digest: strings.Repeat("7", 64),
	}}
}
