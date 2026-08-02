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

type conformanceInvocationTranscript struct {
	Request     InvocationFixtureRequest
	First       ObservationFixtureReceipt
	FirstError  bool
	Second      ObservationFixtureReceipt
	SecondError bool
}

type conformanceInvocationResult struct {
	Transcripts        []conformanceInvocationTranscript
	CancelInvocationID string
	BindingValid       bool
	BundleValid        bool
	EvidenceValid      bool
	ObservationValid   bool
	DedupValid         bool
	NativeValid        bool
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
	invocations := runConformanceInvocations(integrationID, normalized, adapter, executorRequest)
	pauseRequest := PauseFixtureRequest{RunID: "oaw-conformance-run"}
	pauseReceipt, pauseErr := adapter.Pause(pauseRequest)
	cancelRequest := CancelFixtureRequest{InvocationID: invocations.CancelInvocationID}
	cancelReceipt, cancelErr := adapter.Cancel(cancelRequest)

	executorValid := executorErr == nil && executorReceipt.ExecutorID == executorRequest.ExecutorID && executorReceipt.Isolated
	bundleValid := executorReceipt.BundleDigest == bundleDigest && invocations.BundleValid
	pauseValid := pauseErr == nil && pauseReceipt.RunID == pauseRequest.RunID && pauseReceipt.Paused
	cancelValid := cancelErr == nil && cancelReceipt.InvocationID == cancelRequest.InvocationID && cancelReceipt.Cancelled

	results := map[CheckID]bool{
		CheckIsolatedExecutor: executorValid, CheckExactBindingInvocation: invocations.BindingValid,
		CheckPause: pauseValid, CheckBundleInheritance: bundleValid,
		CheckEvidenceReturn: invocations.EvidenceValid, CheckInvocationDedup: invocations.DedupValid,
		CheckCancellation: cancelValid, CheckNormalizedObservation: invocations.ObservationValid,
		CheckNativeInvocation: invocations.NativeValid,
	}
	transcript := struct {
		ExecutorRequest ExecutorFixtureRequest
		ExecutorReceipt ExecutorFixtureReceipt
		ExecutorError   bool
		Invocations     []conformanceInvocationTranscript
		PauseRequest    PauseFixtureRequest
		PauseReceipt    PauseFixtureReceipt
		PauseError      bool
		CancelRequest   CancelFixtureRequest
		CancelReceipt   CancelFixtureReceipt
		CancelError     bool
	}{executorRequest, executorReceipt, executorErr != nil, invocations.Transcripts, pauseRequest, pauseReceipt, pauseErr != nil, cancelRequest, cancelReceipt, cancelErr != nil}
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

func runConformanceInvocations(integrationID string, manifest Manifest, adapter ConformanceAdapter, executor ExecutorFixtureRequest) conformanceInvocationResult {
	result := conformanceInvocationResult{
		Transcripts: []conformanceInvocationTranscript{}, BindingValid: true, BundleValid: true,
		EvidenceValid: true, ObservationValid: true, DedupValid: true, NativeValid: true,
	}
	for index, kind := range manifest.BindingKinds {
		binding := catalog.HostBinding{Host: manifest.HostID, Kind: kind, Reference: "oaw-conformance-binding-" + kind}
		request := InvocationFixtureRequest{
			InvocationID: "oaw-conformance-invocation-" + kind, ExecutorID: executor.ExecutorID,
			Binding: binding, BundleDigest: executor.BundleDigest,
			EvidenceChallengeDigest: fixtureDigest("evidence", integrationID, manifest.ContentDigest(), kind),
		}
		first, firstErr := adapter.Invoke(request)
		second, secondErr := adapter.Invoke(request)
		firstSafe, secondSafe := normalizedObservation(first), normalizedObservation(second)
		if index == 0 {
			result.CancelInvocationID = request.InvocationID
		}
		result.BindingValid = result.BindingValid && firstErr == nil && secondErr == nil && firstSafe.Binding == binding && secondSafe.Binding == binding
		result.BundleValid = result.BundleValid && firstSafe.BundleDigest == executor.BundleDigest && secondSafe.BundleDigest == executor.BundleDigest
		result.EvidenceValid = result.EvidenceValid && firstErr == nil && secondErr == nil && validFixtureEvidence(firstSafe.Evidence, request.EvidenceChallengeDigest) && validFixtureEvidence(secondSafe.Evidence, request.EvidenceChallengeDigest)
		observationValid := firstErr == nil && secondErr == nil && validFixtureObservation(firstSafe, request) && validFixtureObservation(secondSafe, request)
		result.ObservationValid = result.ObservationValid && observationValid
		result.DedupValid = result.DedupValid && observationValid && firstSafe.ExecutionID == secondSafe.ExecutionID && reflect.DeepEqual(firstSafe, secondSafe)
		result.NativeValid = result.NativeValid && firstErr == nil && secondErr == nil && firstSafe.Native && secondSafe.Native
		result.Transcripts = append(result.Transcripts, conformanceInvocationTranscript{Request: request, First: firstSafe, FirstError: firstErr != nil, Second: secondSafe, SecondError: secondErr != nil})
	}
	return result
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
