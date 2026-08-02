package host_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const conformanceRawSecret = "host-token-ticket08-raw"

func TestRunConformancePassesRunnerManagedAdapter(t *testing.T) {
	manifest := runnerManifest(t)
	report, err := host.RunConformance("acme/codex-runtime", manifest, conformingAdapter{})
	if err != nil {
		t.Fatalf("RunConformance() error = %v", err)
	}
	if !report.Passed || len(report.Checks) != len(manifest.Features) || report.Digest == "" || report.TranscriptDigest == "" {
		t.Fatalf("report = %#v", report)
	}
	for _, check := range report.Checks {
		if !check.Passed || check.Evidence == "" {
			t.Fatalf("check = %#v", check)
		}
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), conformanceRawSecret) {
		t.Fatalf("Conformance Report contains raw Adapter output: %s", raw)
	}
}

type conformingAdapter struct {
	native bool
}

func (adapter conformingAdapter) CreateExecutor(request host.ExecutorFixtureRequest) (host.ExecutorFixtureReceipt, error) {
	return host.ExecutorFixtureReceipt{
		ExecutorID: request.ExecutorID, Isolated: true, BundleDigest: request.BundleDigest,
	}, nil
}

func (adapter conformingAdapter) Invoke(request host.InvocationFixtureRequest) (host.ObservationFixtureReceipt, error) {
	return host.ObservationFixtureReceipt{
		InvocationID: request.InvocationID, ExecutionID: "fixture-execution",
		Binding: request.Binding, BundleDigest: request.BundleDigest,
		Outcome:  host.FixtureSucceeded,
		Evidence: []host.NormalizedEvidence{{Reference: "evidence://host-conformance", Digest: request.EvidenceChallengeDigest}},
		Native:   adapter.native, RawOutput: conformanceRawSecret,
	}, nil
}

func (conformingAdapter) Pause(request host.PauseFixtureRequest) (host.PauseFixtureReceipt, error) {
	return host.PauseFixtureReceipt{RunID: request.RunID, Paused: true}, nil
}

func (conformingAdapter) Cancel(request host.CancelFixtureRequest) (host.CancelFixtureReceipt, error) {
	return host.CancelFixtureReceipt{InvocationID: request.InvocationID, Cancelled: true}, nil
}

func TestRunConformanceRequiresNativeInvocationForNativeManaged(t *testing.T) {
	manifest := nativeManifest(t)
	passed, err := host.RunConformance("acme/codex-native", manifest, conformingAdapter{native: true})
	if err != nil || !passed.Passed || !conformanceCheckPassed(passed, host.CheckNativeInvocation) {
		t.Fatalf("native report = %#v, %v", passed, err)
	}
	failed, err := host.RunConformance("acme/codex-native", manifest, conformingAdapter{})
	if err != nil || failed.Passed || conformanceCheckPassed(failed, host.CheckNativeInvocation) {
		t.Fatalf("non-native report = %#v, %v", failed, err)
	}
}

func TestRunConformanceChecksEveryDeclaredBindingKind(t *testing.T) {
	adapter := &mutatingAdapter{observation: func(_ int, value *host.ObservationFixtureReceipt) {
		if value.Binding.Kind == "skill" {
			value.Binding.Reference = "substituted"
		}
	}}
	report, err := host.RunConformance("acme/codex-runtime", runnerManifest(t), adapter)
	if err != nil || report.Passed || conformanceCheckPassed(report, host.CheckExactBindingInvocation) {
		t.Fatalf("Report accepted a substituted declared Binding kind: %#v, %v", report, err)
	}
}

func TestRunConformanceReportsEachBehavioralFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		check  host.CheckID
		mutate func(*mutatingAdapter)
	}{
		{"isolated Executor", host.CheckIsolatedExecutor, func(adapter *mutatingAdapter) {
			adapter.executor = func(value *host.ExecutorFixtureReceipt) { value.Isolated = false }
		}},
		{"exact Binding", host.CheckExactBindingInvocation, func(adapter *mutatingAdapter) {
			adapter.observation = func(_ int, value *host.ObservationFixtureReceipt) { value.Binding.Reference = "other" }
		}},
		{"Bundle inheritance", host.CheckBundleInheritance, func(adapter *mutatingAdapter) {
			adapter.observation = func(_ int, value *host.ObservationFixtureReceipt) { value.BundleDigest = strings.Repeat("0", 64) }
		}},
		{"Evidence return", host.CheckEvidenceReturn, func(adapter *mutatingAdapter) {
			adapter.observation = func(_ int, value *host.ObservationFixtureReceipt) { value.Evidence[0].Digest = strings.Repeat("0", 64) }
		}},
		{"normalized observation", host.CheckNormalizedObservation, func(adapter *mutatingAdapter) {
			adapter.observation = func(_ int, value *host.ObservationFixtureReceipt) { value.Outcome = "INVENTED" }
		}},
		{"invocation deduplication", host.CheckInvocationDedup, func(adapter *mutatingAdapter) {
			adapter.observation = func(call int, value *host.ObservationFixtureReceipt) {
				if call == 2 {
					value.ExecutionID = "second-execution"
				}
			}
		}},
		{"pause", host.CheckPause, func(adapter *mutatingAdapter) {
			adapter.pause = func(value *host.PauseFixtureReceipt) { value.Paused = false }
		}},
		{"cancellation", host.CheckCancellation, func(adapter *mutatingAdapter) {
			adapter.cancel = func(value *host.CancelFixtureReceipt) { value.Cancelled = false }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter := &mutatingAdapter{}
			test.mutate(adapter)
			report, err := host.RunConformance("acme/codex-runtime", runnerManifest(t), adapter)
			if err != nil || report.Passed || conformanceCheckPassed(report, test.check) {
				t.Fatalf("report = %#v, %v", report, err)
			}
		})
	}
}

func TestRunConformanceRedactsAdapterErrors(t *testing.T) {
	adapter := &mutatingAdapter{invokeError: errors.New(conformanceRawSecret)}
	report, err := host.RunConformance("acme/codex-runtime", runnerManifest(t), adapter)
	if err != nil || report.Passed {
		t.Fatalf("report = %#v, %v", report, err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), conformanceRawSecret) {
		t.Fatalf("Report leaked Adapter error: %s", raw)
	}
}

func TestRunConformanceRejectsInstructionOnlyAndInvalidInputs(t *testing.T) {
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV1, ManifestVersion: "1.0.0", HostID: "codex",
		IntegrationLevel: host.InstructionOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.RunConformance("oaw/codex-instruction", manifest, conformingAdapter{}); host.ErrorCode(err) != "HOST_CONFORMANCE_NOT_APPLICABLE" {
		t.Fatalf("instruction-only error = %v", err)
	}
	if _, err := host.RunConformance("Bad", runnerManifest(t), conformingAdapter{}); host.ErrorCode(err) != "HOST_CONFORMANCE_INVALID" {
		t.Fatalf("invalid ID error = %v", err)
	}
	if _, err := host.RunConformance("acme/codex-runtime", runnerManifest(t), nil); host.ErrorCode(err) != "HOST_CONFORMANCE_INVALID" {
		t.Fatalf("nil Adapter error = %v", err)
	}
	invalid := runnerManifest(t)
	invalid.SchemaVersion = "bad"
	if _, err := host.RunConformance("acme/codex-runtime", invalid, conformingAdapter{}); host.ErrorCode(err) != "HOST_CONFORMANCE_INVALID" {
		t.Fatalf("invalid Manifest error = %v", err)
	}
}

type mutatingAdapter struct {
	native      bool
	calls       int
	executor    func(*host.ExecutorFixtureReceipt)
	observation func(int, *host.ObservationFixtureReceipt)
	pause       func(*host.PauseFixtureReceipt)
	cancel      func(*host.CancelFixtureReceipt)
	invokeError error
}

func (adapter *mutatingAdapter) CreateExecutor(request host.ExecutorFixtureRequest) (host.ExecutorFixtureReceipt, error) {
	value, err := (conformingAdapter{native: adapter.native}).CreateExecutor(request)
	if adapter.executor != nil {
		adapter.executor(&value)
	}
	return value, err
}

func (adapter *mutatingAdapter) Invoke(request host.InvocationFixtureRequest) (host.ObservationFixtureReceipt, error) {
	adapter.calls++
	value, err := (conformingAdapter{native: adapter.native}).Invoke(request)
	if adapter.observation != nil {
		adapter.observation(adapter.calls, &value)
	}
	if adapter.invokeError != nil {
		return value, adapter.invokeError
	}
	return value, err
}

func (adapter *mutatingAdapter) Pause(request host.PauseFixtureRequest) (host.PauseFixtureReceipt, error) {
	value, err := (conformingAdapter{}).Pause(request)
	if adapter.pause != nil {
		adapter.pause(&value)
	}
	return value, err
}

func (adapter *mutatingAdapter) Cancel(request host.CancelFixtureRequest) (host.CancelFixtureReceipt, error) {
	value, err := (conformingAdapter{}).Cancel(request)
	if adapter.cancel != nil {
		adapter.cancel(&value)
	}
	return value, err
}

func nativeManifest(t *testing.T) host.Manifest {
	t.Helper()
	value := runnerManifest(t)
	value.IntegrationLevel = host.NativeManaged
	value.Features = append(value.Features, host.FeatureNativeInvocation)
	manifest, err := host.NewManifest(value)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func conformanceCheckPassed(report host.ConformanceReport, id host.CheckID) bool {
	for _, check := range report.Checks {
		if check.ID == id {
			return check.Passed
		}
	}
	return false
}
