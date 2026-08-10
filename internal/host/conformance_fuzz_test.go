package host_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func FuzzInvocationReceiptFailsClosed(f *testing.F) {
	f.Add("COMPLETED", "CURRENT", "shared", "", "succeeded", "", "report", "evidence://result", uint64(1), true)
	f.Add("FAILED", "SUBAGENT", "fresh", "child-1", "failed", "BUILD_FAILED", "diagnostic", "evidence://failure", uint64(2), true)
	f.Add("INVENTED", "CURRENT", "fresh", "child-1", "raw", "secret", "", "\n", uint64(0), false)

	f.Fuzz(func(t *testing.T, kind, topology, freshness, handle, outcome, failureCode, evidenceKind, evidenceReference string, generation uint64, withEvidence bool) {
		if len(kind)+len(topology)+len(freshness)+len(handle)+len(outcome)+len(failureCode)+len(evidenceKind)+len(evidenceReference) > 16<<10 {
			t.Skip()
		}
		evidence := []host.EvidenceReference(nil)
		if withEvidence {
			evidence = []host.EvidenceReference{{
				Kind: evidenceKind, Reference: evidenceReference, Digest: strings.Repeat("e", 64),
			}}
		}
		input := host.InvocationReceipt{
			SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: host.ReceiptKind(kind),
			WorkflowID: "workflow-0123456789abcdef0123456789abcdef", BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleGeneration: generation, BundleDigest: strings.Repeat("a", 64),
			Cursor:   execution.GraphCursor{SlotID: "implementation", Kind: execution.CursorBinding, UnitID: "implementation-main", Ordinal: 1},
			Topology: execution.Topology(topology), HostSessionDigest: strings.Repeat("b", 64), InvocationHandle: handle,
			ContextFreshness: freshness, EnvironmentReportDigest: strings.Repeat("c", 64), DispatchDigest: strings.Repeat("d", 64), Outcome: outcome,
			FailureCode: failureCode, Outputs: []host.OutputReference{}, Evidence: evidence,
		}
		if input.Kind == host.ReceiptCompleted {
			input.Outputs = validOutputs()
		}
		first, firstErr := host.NewInvocationReceipt(input)
		second, secondErr := host.NewInvocationReceipt(input)
		if hostErrorText(firstErr) != hostErrorText(secondErr) || !reflect.DeepEqual(first, second) {
			t.Fatalf("receipt construction is nondeterministic: %#v/%v %#v/%v", first, firstErr, second, secondErr)
		}
		if firstErr != nil {
			if host.ErrorCode(firstErr) != "HOST_INVOCATION_RECEIPT_INVALID" {
				t.Fatalf("receipt failed open with unstable error: %v", firstErr)
			}
			return
		}
		if first.Digest == "" {
			t.Fatal("accepted receipt lacks a digest")
		}
		switch first.Topology {
		case execution.TopologyCurrent:
			if first.ContextFreshness != host.ContextShared || first.InvocationHandle != "" {
				t.Fatalf("accepted invalid CURRENT receipt: %#v", first)
			}
		case execution.TopologySubagent:
			if first.ContextFreshness != host.ContextFresh || first.InvocationHandle == "" {
				t.Fatalf("accepted invalid SUBAGENT receipt: %#v", first)
			}
		default:
			t.Fatalf("accepted unknown topology %q", first.Topology)
		}
		roundTrip, err := host.NewInvocationReceipt(first)
		if err != nil || !reflect.DeepEqual(roundTrip, first) {
			t.Fatalf("accepted receipt is not canonical: %#v, %v", roundTrip, err)
		}
		if len(evidence) > 0 {
			evidence[0].Reference = "changed"
			if first.Evidence[0].Reference == "changed" {
				t.Fatal("accepted receipt aliases caller evidence")
			}
		}
	})
}

func FuzzConformanceTranscriptFailsClosed(f *testing.F) {
	f.Add("dedup-key", strings.Repeat("8", 64), "", false)
	f.Add("\n", "bad", strings.Repeat("0", 64), true)

	f.Fuzz(func(t *testing.T, idempotencyKey, dispatchDigest, receiptDigest string, duplicate bool) {
		if len(idempotencyKey)+len(dispatchDigest)+len(receiptDigest) > 16<<10 {
			t.Skip()
		}
		manifest := hostNativeManifest(t, []host.Feature{
			host.FeatureInvocationDedup, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory,
		})
		inventory, environment, session := currentHostFacts(t, manifest)
		receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
			SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: host.ReceiptCompleted,
			WorkflowID: "workflow-0123456789abcdef0123456789abcdef", BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64),
			Cursor:   execution.GraphCursor{SlotID: "implementation", Kind: execution.CursorBinding, UnitID: "implementation-main", Ordinal: 1},
			Topology: execution.TopologyCurrent, HostSessionDigest: session.Digest, ContextFreshness: host.ContextShared,
			EnvironmentReportDigest: environment.Digest, DispatchDigest: strings.Repeat("8", 64), Outcome: "succeeded",
			Outputs:  validOutputs(),
			Evidence: []host.EvidenceReference{{Kind: "report", Reference: "evidence://result", Digest: strings.Repeat("e", 64)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if receiptDigest == "" {
			receiptDigest = receipt.Digest
		}
		invocations := []host.InvocationRecord{{
			IdempotencyKey: idempotencyKey, DispatchDigest: dispatchDigest, ReceiptDigest: receiptDigest,
		}}
		if duplicate {
			invocations = append(invocations, invocations[0])
		}
		input := host.ConformanceTranscript{
			SchemaVersion: host.HostConformanceTranscriptSchemaV4, Session: session, Inventory: inventory,
			EnvironmentReports: []host.EnvironmentReport{environment}, Receipts: []host.InvocationReceipt{receipt},
			Invocations: invocations,
		}
		first, firstErr := host.NewConformanceTranscript(input)
		second, secondErr := host.NewConformanceTranscript(input)
		if hostErrorText(firstErr) != hostErrorText(secondErr) || !reflect.DeepEqual(first, second) {
			t.Fatalf("transcript construction is nondeterministic: %#v/%v %#v/%v", first, firstErr, second, secondErr)
		}
		if firstErr != nil {
			if host.ErrorCode(firstErr) != "HOST_CONFORMANCE_TRANSCRIPT_INVALID" {
				t.Fatalf("transcript failed open with unstable error: %v", firstErr)
			}
			return
		}
		roundTrip, err := host.NewConformanceTranscript(first)
		if err != nil || !reflect.DeepEqual(roundTrip, first) {
			t.Fatalf("accepted transcript is not canonical: %#v, %v", roundTrip, err)
		}
		report, err := host.ValidateConformanceTranscript(manifest, first)
		if err != nil || report.TranscriptDigest != first.Digest {
			t.Fatalf("accepted transcript cannot be validated: %#v, %v", report, err)
		}
		assertTranscriptFieldBoundary(t, first)
	})
}

func assertTranscriptFieldBoundary(t *testing.T, transcript host.ConformanceTranscript) {
	t.Helper()
	raw, err := json.Marshal(transcript)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"raw_output", "command", "credentials", "process"} {
		if _, exists := value[forbidden]; exists {
			t.Fatalf("transcript exposes forbidden field %q", forbidden)
		}
	}
}

func hostErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
