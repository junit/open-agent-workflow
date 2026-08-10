package host

import (
	"fmt"
	"slices"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func NewConformanceTranscript(input ConformanceTranscript) (ConformanceTranscript, error) {
	if input.SchemaVersion != HostConformanceTranscriptSchemaV3 {
		return ConformanceTranscript{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Conformance Transcript schema", nil)
	}
	providedDigest := input.Digest
	input = CloneConformanceTranscript(input)
	input.Digest = ""
	if len(input.Receipts) > 256 || len(input.Invocations) > 256 {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "Conformance Transcript exceeds collection limits", nil)
	}
	if err := validateStoredSessionSnapshot(input.Session); err != nil {
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "invalid Host session", err)
	}
	normalizedInventory, err := ValidateBindingInventory(input.Inventory)
	if err != nil {
		if ErrorCode(err) == "HOST_SCHEMA_UNSUPPORTED" {
			return ConformanceTranscript{}, err
		}
		return ConformanceTranscript{}, hostError("HOST_CONFORMANCE_TRANSCRIPT_INVALID", "invalid Binding Inventory", err)
	}
	input.Inventory = normalizedInventory
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
		if ErrorCode(err) == "HOST_SCHEMA_UNSUPPORTED" {
			return ConformanceReport{}, err
		}
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "invalid Host Manifest", err)
	}
	if normalizedManifest.ControlSurface != SurfaceHostNative {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_NOT_APPLICABLE", "policy Host has no Host-native conformance", nil)
	}
	normalizedTranscript, err := NewConformanceTranscript(transcript)
	if err != nil {
		return ConformanceReport{}, err
	}
	if normalizedTranscript.Session.HostID != normalizedManifest.HostID || normalizedTranscript.Session.ManifestDigest != normalizedManifest.Digest {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "Transcript Host or Manifest digest does not match", nil)
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
	if hasBindingInventoryEvidence(normalizedManifest, normalizedTranscript.Inventory) {
		verified = append(verified, FeatureProviderBindingInventory)
	}
	if hasEnvironmentReportingEvidence(normalizedTranscript) {
		verified = append(verified, FeatureEnvironmentReporting)
	}
	if hasNormalizedReceiptEvidence(normalizedTranscript) {
		verified = append(verified, FeatureNormalizedReceipts)
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
	verified = retainDeclaredFeatures(normalizedManifest.Features, verified)

	verifiedDelegation := make([]FeatureID, 0, len(normalizedManifest.DelegationFeatures))
	for _, observation := range normalizedTranscript.Session.FeatureObservations {
		if observation.State == AvailabilityAvailable && isLiveObservationSource(observation.Source) && slices.Contains(normalizedManifest.DelegationFeatures, observation.Feature) {
			verifiedDelegation = append(verifiedDelegation, observation.Feature)
		}
	}
	sort.Slice(verifiedDelegation, func(left, right int) bool { return verifiedDelegation[left] < verifiedDelegation[right] })

	verifiedActions := make([]string, 0, len(normalizedManifest.HostActions))
	for _, observation := range normalizedTranscript.Session.HostActionObservations {
		if observation.State == AvailabilityAvailable && isLiveObservationSource(observation.Source) {
			if declared, ok := hostActionByID(normalizedManifest.HostActions, observation.Action.ID); ok && hostActionContractEqual(declared, observation.Action) {
				verifiedActions = append(verifiedActions, observation.Action.ID)
			}
		}
	}
	sort.Strings(verifiedActions)

	diagnostics := make([]string, 0)
	for _, feature := range normalizedManifest.Features {
		if !slices.Contains(verified, feature) {
			diagnostics = append(diagnostics, fmt.Sprintf("feature %q lacks transcript evidence", feature))
		}
	}
	for _, feature := range normalizedManifest.DelegationFeatures {
		if !slices.Contains(verifiedDelegation, feature) {
			diagnostics = append(diagnostics, fmt.Sprintf("delegation feature %q lacks live available evidence", feature))
		}
	}
	for _, action := range normalizedManifest.HostActions {
		if !slices.Contains(verifiedActions, action.ID) {
			diagnostics = append(diagnostics, fmt.Sprintf("Host action %q lacks live available evidence", action.ID))
		}
	}
	return NewConformanceReport(ConformanceReport{
		SchemaVersion: HostConformanceReportSchemaV3, ManifestDigest: normalizedManifest.Digest,
		TranscriptDigest: normalizedTranscript.Digest, VerifiedFeatures: verified,
		VerifiedDelegationFeatures: verifiedDelegation, VerifiedHostActionIDs: verifiedActions, Diagnostics: diagnostics,
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
			if observation.Kind == kind && slices.Contains(observation.Topologies, execution.TopologyCurrent) {
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

func retainDeclaredFeatures(declared, observed []Feature) []Feature {
	result := make([]Feature, 0, len(observed))
	for _, feature := range observed {
		if slices.Contains(declared, feature) {
			result = append(result, feature)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}

func isLiveObservationSource(source ObservationSource) bool {
	return source == SourceNativeAPI || source == SourceLiveHostIndex || source == SourceLiveFilesystem
}

func hostActionContractEqual(left, right HostActionContract) bool {
	return left.ID == right.ID && left.InputSchema == right.InputSchema && left.OutcomeSchema == right.OutcomeSchema &&
		slices.Equal(left.MaximumEffects, right.MaximumEffects) && slices.Equal(left.Resources, right.Resources)
}
