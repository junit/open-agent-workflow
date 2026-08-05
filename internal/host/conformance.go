package host

import (
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func NewConformanceTranscript(input ConformanceTranscript) (ConformanceTranscript, error) {
	providedDigest := input.Digest
	input.Digest = ""
	if len(input.Receipts) > 256 || len(input.Invocations) > 256 {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "Conformance Transcript exceeds collection limits", nil)
	}
	input = CloneConformanceTranscript(input)
	if input.SchemaVersion != HostConformanceTranscriptSchemaV2 {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "unsupported Conformance Transcript schema", nil)
	}
	if err := validateStoredSessionSnapshot(input.Session); err != nil {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "invalid Host session", err)
	}
	normalizedInventory, err := NewBindingInventory(input.Inventory.HostID, input.Inventory.Observations)
	if err != nil || !reflect.DeepEqual(normalizedInventory, input.Inventory) {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "invalid Binding Inventory", err)
	}
	if input.Inventory.HostID != input.Session.HostID || input.Inventory.Digest != input.Session.ProviderInventoryDigest {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "Binding Inventory is not pinned to the Host session", nil)
	}
	if len(input.EnvironmentReports) != 1 {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "Conformance Transcript requires one Environment Report", nil)
	}
	if err := ValidateEnvironmentReport(input.Session, input.EnvironmentReports[0]); err != nil {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "invalid Environment Report", err)
	}
	for index, receipt := range input.Receipts {
		normalized, receiptErr := NewInvocationReceipt(receipt)
		if receiptErr != nil {
			return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "invalid Invocation Receipt", receiptErr)
		}
		input.Receipts[index] = normalized
	}
	sort.SliceStable(input.Invocations, func(left, right int) bool {
		return invocationRecordKey(input.Invocations[left]) < invocationRecordKey(input.Invocations[right])
	})
	if err := validateInvocationRecords(input.Invocations, input.Receipts); err != nil {
		return ConformanceTranscript{}, err
	}
	digest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "Conformance Transcript cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "Conformance Transcript digest mismatch", nil)
	}
	input.Digest = digest
	return input, nil
}

func invocationRecordKey(value InvocationRecord) string {
	return value.IdempotencyKey + "\x00" + value.DispatchDigest + "\x00" + value.ReceiptDigest
}

func ValidateConformanceTranscript(manifest Manifest, transcript ConformanceTranscript) (ConformanceReport, error) {
	normalizedManifest, err := NewManifest(manifest)
	if err != nil {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "invalid Host Manifest", err)
	}
	if normalizedManifest.ControlSurface != SurfaceHostNative {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_NOT_APPLICABLE", "policy Host has no Host-native conformance", nil)
	}
	normalizedTranscript, err := NewConformanceTranscript(transcript)
	if err != nil {
		return ConformanceReport{}, err
	}
	if normalizedTranscript.Session.HostID != normalizedManifest.HostID {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "Transcript Host does not match Manifest", nil)
	}
	for _, topology := range normalizedTranscript.Session.SupportedTopologies {
		if !slices.Contains(normalizedManifest.SupportedTopologies, topology) {
			return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "Transcript topology is not declared by Manifest", nil)
		}
	}
	if err := validateReceiptBindings(normalizedTranscript); err != nil {
		return ConformanceReport{}, err
	}

	verified := make([]Feature, 0, len(normalizedManifest.Features))
	diagnostics := make([]string, 0)
	if hasBindingInventoryEvidence(normalizedManifest, normalizedTranscript.Inventory) {
		verified = append(verified, FeatureProviderBindingInventory)
	} else {
		diagnostics = append(diagnostics, "provider-binding-inventory evidence is incomplete")
	}
	if hasEnvironmentReportingEvidence(normalizedTranscript) {
		verified = append(verified, FeatureEnvironmentReporting)
	}
	if hasNormalizedReceiptEvidence(normalizedTranscript) {
		verified = append(verified, FeatureNormalizedReceipts)
	} else {
		diagnostics = append(diagnostics, "normalized-receipts evidence is incomplete")
	}
	if hasInvocationDeduplicationEvidence(normalizedTranscript.Invocations) {
		verified = append(verified, FeatureInvocationDedup)
	}
	if hasReceiptKind(normalizedTranscript.Receipts, ReceiptPaused) {
		verified = append(verified, FeaturePause)
	}
	if hasReceiptKind(normalizedTranscript.Receipts, ReceiptCancelled) {
		verified = append(verified, FeatureCancellation)
	}

	missing := make(map[Feature]bool)
	for _, feature := range normalizedManifest.Features {
		missing[feature] = true
	}
	filtered := verified[:0]
	for _, feature := range verified {
		if missing[feature] {
			filtered = append(filtered, feature)
		}
	}
	verified = filtered
	for _, feature := range normalizedManifest.Features {
		if !slices.Contains(verified, feature) {
			diagnostics = append(diagnostics, fmt.Sprintf("feature %q lacks transcript evidence", feature))
		}
	}
	sort.Slice(verified, func(left, right int) bool { return verified[left] < verified[right] })
	return NewConformanceReport(ConformanceReport{
		SchemaVersion: HostConformanceReportSchemaV2, ManifestDigest: normalizedManifest.ContentDigest(),
		TranscriptDigest: normalizedTranscript.Digest, VerifiedFeatures: verified, Diagnostics: diagnostics,
	})
}

func validateReceiptBindings(transcript ConformanceTranscript) error {
	report := transcript.EnvironmentReports[0]
	for _, receipt := range transcript.Receipts {
		if receipt.HostSessionDigest != transcript.Session.Digest || receipt.EnvironmentReportDigest != report.Digest ||
			receipt.Topology != report.Topology || !slices.Contains(transcript.Session.SupportedTopologies, receipt.Topology) {
			return hostError("HOST_CONFORMANCE_INVALID", "Invocation Receipt is not pinned to the Host session environment", nil)
		}
	}
	return nil
}

func hasBindingInventoryEvidence(manifest Manifest, inventory BindingInventory) bool {
	for _, kind := range manifest.BindingKinds {
		found := false
		for _, observation := range inventory.Observations {
			if observation.Binding.Kind == kind && slices.Contains(observation.Topologies, execution.TopologyCurrent) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(manifest.BindingKinds) > 0
}

func hasEnvironmentReportingEvidence(transcript ConformanceTranscript) bool {
	return len(transcript.EnvironmentReports) == 1 && transcript.EnvironmentReports[0].Digest == transcript.Session.EnvironmentReportDigest
}

func hasNormalizedReceiptEvidence(transcript ConformanceTranscript) bool {
	for _, receipt := range transcript.Receipts {
		if receipt.HostSessionDigest == transcript.Session.Digest && receipt.EnvironmentReportDigest == transcript.Session.EnvironmentReportDigest {
			return true
		}
	}
	return false
}

func hasReceiptKind(receipts []InvocationReceipt, kind ReceiptKind) bool {
	for _, receipt := range receipts {
		if receipt.Kind == kind {
			return true
		}
	}
	return false
}

func hasInvocationDeduplicationEvidence(records []InvocationRecord) bool {
	counts := make(map[string]int)
	for _, record := range records {
		counts[record.IdempotencyKey]++
	}
	for _, count := range counts {
		if count > 1 {
			return true
		}
	}
	return false
}

func validateInvocationRecords(records []InvocationRecord, receipts []InvocationReceipt) error {
	receiptDigests := make(map[string]bool, len(receipts))
	for _, receipt := range receipts {
		receiptDigests[receipt.Digest] = true
	}
	for index, record := range records {
		if !validHostText(record.IdempotencyKey, 512) || !digestPattern.MatchString(record.DispatchDigest) || !receiptDigests[record.ReceiptDigest] ||
			index > 0 && record.IdempotencyKey < records[index-1].IdempotencyKey {
			return hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "invalid or non-canonical Invocation Record", nil)
		}
		if index > 0 && record.IdempotencyKey == records[index-1].IdempotencyKey &&
			(record.DispatchDigest != records[index-1].DispatchDigest || record.ReceiptDigest != records[index-1].ReceiptDigest) {
			return hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "idempotency key maps to conflicting receipts", nil)
		}
	}
	return nil
}

func CloneConformanceTranscript(value ConformanceTranscript) ConformanceTranscript {
	value.Session = CloneSessionSnapshot(value.Session)
	value.Inventory = CloneBindingInventory(value.Inventory)
	reports := make([]EnvironmentReport, len(value.EnvironmentReports))
	for index, report := range value.EnvironmentReports {
		reports[index] = CloneEnvironmentReport(report)
	}
	value.EnvironmentReports = reports
	receipts := make([]InvocationReceipt, len(value.Receipts))
	for index, receipt := range value.Receipts {
		receipts[index] = CloneInvocationReceipt(receipt)
	}
	value.Receipts = receipts
	value.Invocations = append([]InvocationRecord{}, value.Invocations...)
	return value
}
