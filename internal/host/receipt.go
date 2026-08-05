package host

import (
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func NewInvocationReceipt(input InvocationReceipt) (InvocationReceipt, error) {
	providedDigest := input.Digest
	input.Digest = ""
	input.Evidence = append([]EvidenceReference{}, input.Evidence...)
	sort.Slice(input.Evidence, func(left, right int) bool {
		return evidenceReferenceKey(input.Evidence[left]) < evidenceReferenceKey(input.Evidence[right])
	})
	if err := validateInvocationReceipt(input); err != nil {
		return InvocationReceipt{}, err
	}
	digest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return InvocationReceipt{}, hostError("HOST_INVOCATION_RECEIPT_INVALID", "invocation receipt cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return InvocationReceipt{}, hostError("HOST_INVOCATION_RECEIPT_INVALID", "invocation receipt digest mismatch", nil)
	}
	input.Digest = digest
	return input, nil
}

func CloneInvocationReceipt(value InvocationReceipt) InvocationReceipt {
	value.Evidence = append([]EvidenceReference{}, value.Evidence...)
	return value
}

func validateInvocationReceipt(value InvocationReceipt) error {
	if value.SchemaVersion != HostInvocationReceiptSchemaV2 ||
		!validHostText(value.WorkflowID, 512) || value.BundleGeneration == 0 || !digestPattern.MatchString(value.BundleDigest) ||
		!validHostText(value.NodeID, 512) || !digestPattern.MatchString(value.HostSessionDigest) ||
		!digestPattern.MatchString(value.EnvironmentReportDigest) {
		return hostError("HOST_INVOCATION_RECEIPT_INVALID", "invalid invocation receipt identity", nil)
	}
	if len(value.Evidence) > 128 {
		return hostError("HOST_INVOCATION_RECEIPT_INVALID", "invocation receipt has too many evidence references", nil)
	}
	if _, err := execution.NormalizeTopologies([]execution.Topology{value.Topology}); err != nil {
		return hostError("HOST_INVOCATION_RECEIPT_INVALID", "invalid invocation receipt topology", err)
	}
	switch value.Topology {
	case execution.TopologyCurrent:
		if value.ContextFreshness != ContextShared || value.InvocationHandle != "" {
			return hostError("HOST_INVOCATION_RECEIPT_INVALID", "invalid CURRENT receipt context", nil)
		}
	case execution.TopologySubagent:
		if value.ContextFreshness != ContextFresh || !validHostText(value.InvocationHandle, 1024) {
			return hostError("HOST_INVOCATION_RECEIPT_INVALID", "invalid SUBAGENT receipt context", nil)
		}
	}
	if !validReceiptPayload(value) {
		return hostError("HOST_INVOCATION_RECEIPT_INVALID", "invalid receipt payload", nil)
	}
	for index, reference := range value.Evidence {
		if !validHostText(reference.Kind, 128) || !validHostText(reference.Reference, 2048) || !digestPattern.MatchString(reference.Digest) ||
			index > 0 && evidenceReferenceKey(value.Evidence[index-1]) == evidenceReferenceKey(reference) {
			return hostError("HOST_INVOCATION_RECEIPT_INVALID", "invalid or duplicate evidence reference", nil)
		}
	}
	return nil
}

func validReceiptPayload(value InvocationReceipt) bool {
	switch value.Kind {
	case ReceiptStarted:
		return value.Outcome == "" && value.FailureCode == "" && len(value.Evidence) == 0
	case ReceiptPaused:
		return value.Outcome == "paused" && value.FailureCode == "" && len(value.Evidence) == 0
	case ReceiptCompleted:
		return validHostText(value.Outcome, 512) && value.FailureCode == "" && len(value.Evidence) > 0
	case ReceiptFailed:
		return value.Outcome == "failed" && validHostText(value.FailureCode, 128) && len(value.Evidence) > 0
	case ReceiptCancelled:
		return value.Outcome == "cancelled" && value.FailureCode == "" && len(value.Evidence) == 0
	default:
		return false
	}
}

func evidenceReferenceKey(value EvidenceReference) string {
	return strings.Join([]string{value.Kind, value.Reference, value.Digest}, "\x00")
}
