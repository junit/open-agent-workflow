package host

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

type FixtureOutcome string

const FixtureSucceeded FixtureOutcome = "SUCCEEDED"

type NormalizedEvidence struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

type ExecutorFixtureRequest struct {
	ExecutorID   string `json:"executor_id"`
	BundleDigest string `json:"bundle_digest"`
}

type ExecutorFixtureReceipt struct {
	ExecutorID   string `json:"executor_id"`
	Isolated     bool   `json:"isolated"`
	BundleDigest string `json:"bundle_digest"`
}

type InvocationFixtureRequest struct {
	InvocationID            string              `json:"invocation_id"`
	ExecutorID              string              `json:"executor_id"`
	Binding                 catalog.HostBinding `json:"binding"`
	BundleDigest            string              `json:"bundle_digest"`
	EvidenceChallengeDigest string              `json:"evidence_challenge_digest"`
}

type ObservationFixtureReceipt struct {
	InvocationID string               `json:"invocation_id"`
	ExecutionID  string               `json:"execution_id"`
	Binding      catalog.HostBinding  `json:"binding"`
	BundleDigest string               `json:"bundle_digest"`
	Outcome      FixtureOutcome       `json:"outcome"`
	Evidence     []NormalizedEvidence `json:"evidence"`
	Native       bool                 `json:"native"`
	RawOutput    string               `json:"-"`
}

type PauseFixtureRequest struct {
	RunID string `json:"run_id"`
}

type PauseFixtureReceipt struct {
	RunID  string `json:"run_id"`
	Paused bool   `json:"paused"`
}

type CancelFixtureRequest struct {
	InvocationID string `json:"invocation_id"`
}

type CancelFixtureReceipt struct {
	InvocationID string `json:"invocation_id"`
	Cancelled    bool   `json:"cancelled"`
}

type ConformanceAdapter interface {
	CreateExecutor(ExecutorFixtureRequest) (ExecutorFixtureReceipt, error)
	Invoke(InvocationFixtureRequest) (ObservationFixtureReceipt, error)
	Pause(PauseFixtureRequest) (PauseFixtureReceipt, error)
	Cancel(CancelFixtureRequest) (CancelFixtureReceipt, error)
}

func RunConformance(integrationID string, manifest Manifest, adapter ConformanceAdapter) (ConformanceReport, error) {
	normalized, err := NewManifest(manifest)
	if err != nil {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "invalid Manifest", err)
	}
	if normalized.IntegrationLevel == InstructionOnly {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_NOT_APPLICABLE", "instruction-only Integration has no Runtime conformance", nil)
	}
	if _, err := catalog.ParseQualifiedID(integrationID); err != nil || adapter == nil {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "invalid Integration or Adapter", err)
	}

	bundleDigest := fixtureDigest("bundle", integrationID, normalized.ContentDigest())
	executorRequest := ExecutorFixtureRequest{ExecutorID: "oaw-conformance-executor", BundleDigest: bundleDigest}
	executorReceipt, executorErr := adapter.CreateExecutor(executorRequest)
	binding := catalog.HostBinding{Host: normalized.HostID, Kind: normalized.BindingKinds[0], Reference: "oaw-conformance-binding"}
	invocationRequest := InvocationFixtureRequest{
		InvocationID: "oaw-conformance-invocation", ExecutorID: executorRequest.ExecutorID,
		Binding: binding, BundleDigest: bundleDigest,
		EvidenceChallengeDigest: fixtureDigest("evidence", integrationID, normalized.ContentDigest()),
	}
	first, firstErr := adapter.Invoke(invocationRequest)
	second, secondErr := adapter.Invoke(invocationRequest)
	pauseRequest := PauseFixtureRequest{RunID: "oaw-conformance-run"}
	pauseReceipt, pauseErr := adapter.Pause(pauseRequest)
	cancelRequest := CancelFixtureRequest{InvocationID: invocationRequest.InvocationID}
	cancelReceipt, cancelErr := adapter.Cancel(cancelRequest)

	firstSafe := normalizedObservation(first)
	secondSafe := normalizedObservation(second)
	executorValid := executorErr == nil && executorReceipt.ExecutorID == executorRequest.ExecutorID && executorReceipt.Isolated
	bindingValid := firstErr == nil && secondErr == nil && firstSafe.Binding == binding && secondSafe.Binding == binding
	bundleValid := executorReceipt.BundleDigest == bundleDigest && firstSafe.BundleDigest == bundleDigest && secondSafe.BundleDigest == bundleDigest
	evidenceValid := firstErr == nil && secondErr == nil && validFixtureEvidence(firstSafe.Evidence, invocationRequest.EvidenceChallengeDigest) && validFixtureEvidence(secondSafe.Evidence, invocationRequest.EvidenceChallengeDigest)
	observationValid := firstErr == nil && secondErr == nil && validFixtureObservation(firstSafe, invocationRequest) && validFixtureObservation(secondSafe, invocationRequest)
	dedupValid := observationValid && firstSafe.ExecutionID == secondSafe.ExecutionID && reflect.DeepEqual(firstSafe, secondSafe)
	pauseValid := pauseErr == nil && pauseReceipt.RunID == pauseRequest.RunID && pauseReceipt.Paused
	cancelValid := cancelErr == nil && cancelReceipt.InvocationID == cancelRequest.InvocationID && cancelReceipt.Cancelled
	nativeValid := firstErr == nil && secondErr == nil && firstSafe.Native && secondSafe.Native

	results := map[CheckID]bool{
		CheckIsolatedExecutor: executorValid, CheckExactBindingInvocation: bindingValid,
		CheckPause: pauseValid, CheckBundleInheritance: bundleValid,
		CheckEvidenceReturn: evidenceValid, CheckInvocationDedup: dedupValid,
		CheckCancellation: cancelValid, CheckNormalizedObservation: observationValid,
		CheckNativeInvocation: nativeValid,
	}
	transcript := struct {
		ExecutorRequest   ExecutorFixtureRequest
		ExecutorReceipt   ExecutorFixtureReceipt
		ExecutorError     bool
		InvocationRequest InvocationFixtureRequest
		First             ObservationFixtureReceipt
		FirstError        bool
		Second            ObservationFixtureReceipt
		SecondError       bool
		PauseRequest      PauseFixtureRequest
		PauseReceipt      PauseFixtureReceipt
		PauseError        bool
		CancelRequest     CancelFixtureRequest
		CancelReceipt     CancelFixtureReceipt
		CancelError       bool
	}{executorRequest, executorReceipt, executorErr != nil, invocationRequest, firstSafe, firstErr != nil, secondSafe, secondErr != nil, pauseRequest, pauseReceipt, pauseErr != nil, cancelRequest, cancelReceipt, cancelErr != nil}
	transcriptDigest, _, err := canonicaljson.Digest(transcript)
	if err != nil {
		return ConformanceReport{}, hostError("HOST_CONFORMANCE_INVALID", "fixture transcript cannot be canonicalized", err)
	}
	checks := make([]ConformanceCheck, 0, len(normalized.Features))
	allPassed := true
	for _, feature := range normalized.Features {
		id := CheckID(feature)
		passed := results[id]
		allPassed = allPassed && passed
		checks = append(checks, ConformanceCheck{ID: id, Passed: passed, Evidence: fixtureCheckDigest(id, passed, transcriptDigest)})
	}
	return NewConformanceReport(ConformanceReport{
		SchemaVersion: ConformanceReportSchemaV1, SuiteVersion: ConformanceSuiteV1,
		IntegrationID: integrationID, ManifestDigest: normalized.ContentDigest(),
		Checks: checks, TranscriptDigest: transcriptDigest, Passed: allPassed,
	})
}

func normalizedObservation(value ObservationFixtureReceipt) ObservationFixtureReceipt {
	value.Evidence = append([]NormalizedEvidence{}, value.Evidence...)
	value.RawOutput = ""
	return value
}

func validFixtureEvidence(values []NormalizedEvidence, challenge string) bool {
	return len(values) == 1 && values[0].Reference == "evidence://host-conformance" && values[0].Digest == challenge
}

func validFixtureObservation(value ObservationFixtureReceipt, request InvocationFixtureRequest) bool {
	return value.InvocationID == request.InvocationID && strings.TrimSpace(value.ExecutionID) == value.ExecutionID && value.ExecutionID != "" && value.Outcome == FixtureSucceeded
}

func fixtureCheckDigest(id CheckID, passed bool, transcriptDigest string) string {
	digest, _, _ := canonicaljson.Digest(struct {
		ID               CheckID `json:"id"`
		Passed           bool    `json:"passed"`
		TranscriptDigest string  `json:"transcript_digest"`
	}{id, passed, transcriptDigest})
	return digest
}

func fixtureDigest(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}
