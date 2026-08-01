package runtime_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestDispatchPreparedAuthorizesCommittedInvocationAndReplays(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	granted, grant := grantBoundedRun(t, engine, fixture, "authorize")
	frame := dispatchPreparedFrame(granted, grant, "authorize-prepared")

	authorized, err := engine.Exchange(frame)
	if err != nil {
		t.Fatalf("DISPATCH_PREPARED error = %v", err)
	}
	if authorized.Kind != runtime.ReplyDispatchAuthorized || authorized.Revision != 3 || authorized.Snapshot.Status != runtime.RunInFlight || len(authorized.Snapshot.Grants) != 1 || len(authorized.Snapshot.Observations) != 0 {
		t.Fatalf("dispatch authorization reply = %#v", authorized)
	}
	inspected, err := engine.Exchange(inspectFrame(granted.RunID, "authorize-inspect"))
	if err != nil {
		t.Fatalf("INSPECT error = %v", err)
	}
	if !reflect.DeepEqual(inspected.Snapshot, authorized.Snapshot) {
		t.Fatalf("authorization was not committed before reply\n got: %#v\nwant: %#v", inspected.Snapshot, authorized.Snapshot)
	}
	replayed, err := engine.Exchange(frame)
	if err != nil {
		t.Fatalf("DISPATCH_PREPARED replay error = %v", err)
	}
	if !reflect.DeepEqual(replayed, authorized) {
		t.Fatalf("authorization replay differs\n got: %#v\nwant: %#v", replayed, authorized)
	}
	assertRevisionCount(t, stateRoot, granted.RunID, 3)
}

func TestDispatchPreparedRejectsWrongOrDuplicateInvocationWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*runtime.RunFrame)
		code   string
	}{
		{"missing preparation", func(frame *runtime.RunFrame) { frame.Continue.DispatchPreparation = nil }, "DISPATCH_PREPARATION_INVALID"},
		{"wrong Grant", func(frame *runtime.RunFrame) { frame.Continue.DispatchPreparation.GrantID = "grant-wrong" }, "DISPATCH_PREPARATION_INVALID"},
		{"wrong invocation", func(frame *runtime.RunFrame) { frame.Continue.DispatchPreparation.InvocationID = "invocation-wrong" }, "DISPATCH_PREPARATION_INVALID"},
		{"wrong Executor", func(frame *runtime.RunFrame) { frame.Continue.DispatchPreparation.ExecutorID = "executor-wrong" }, "DISPATCH_PREPARATION_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			granted, grant := grantBoundedRun(t, engine, fixture, "reject-"+strings.ReplaceAll(test.name, " ", "-"))
			frame := dispatchPreparedFrame(granted, grant, "reject-prepared")
			test.mutate(&frame)
			_, err := engine.Exchange(frame)
			assertErrorCode(t, err, test.code)
			assertRevisionCount(t, stateRoot, granted.RunID, 2)
		})
	}

	t.Run("second distinct preparation", func(t *testing.T) {
		fixture := newBoundedRuntimeFixture(t, false)
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine := newBoundedEngine(t, stateRoot, fixture)
		granted, grant := grantBoundedRun(t, engine, fixture, "duplicate-preparation")
		authorized, err := engine.Exchange(dispatchPreparedFrame(granted, grant, "prepared-once"))
		if err != nil {
			t.Fatal(err)
		}
		frame := dispatchPreparedFrame(granted, grant, "prepared-twice")
		frame.ExpectedRevision = authorized.Revision
		_, err = engine.Exchange(frame)
		assertErrorCode(t, err, "RUN_TRANSITION_INVALID")
		assertRevisionCount(t, stateRoot, granted.RunID, 3)
	})
}

func TestCapabilityObservedFinishesOrEscalatesWithPinnedEvidence(t *testing.T) {
	t.Run("succeeded", func(t *testing.T) {
		fixture := newBoundedRuntimeFixture(t, false)
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine := newBoundedEngine(t, stateRoot, fixture)
		authorized, grant := authorizeBoundedRun(t, engine, fixture, "observed-success")
		frame := observedFrame(authorized, grant, runtime.ObservationSucceeded, "observed-success-result")

		finished, err := engine.Exchange(frame)
		if err != nil {
			t.Fatalf("CAPABILITY_OBSERVED error = %v", err)
		}
		if finished.Kind != runtime.ReplyFinished || finished.Revision != 4 || finished.Snapshot.Status != runtime.RunFinished || len(finished.Snapshot.Grants) != 1 || len(finished.Snapshot.Observations) != 1 {
			t.Fatalf("finished reply = %#v", finished)
		}
		observation := finished.Snapshot.Observations[0]
		if observation.GrantID != grant.ID || observation.InvocationID != grant.InvocationID || observation.ExecutorID != grant.Executor.ID || observation.Outcome != runtime.ObservationSucceeded || len(observation.EvidenceReferences) != 1 || observation.EvidenceReferences[0].Digest != strings.Repeat("2", 64) {
			t.Fatalf("persisted observation = %#v", observation)
		}
		replayed, err := engine.Exchange(frame)
		if err != nil {
			t.Fatalf("CAPABILITY_OBSERVED replay error = %v", err)
		}
		if !reflect.DeepEqual(replayed, finished) {
			t.Fatalf("observation replay differs\n got: %#v\nwant: %#v", replayed, finished)
		}
		finished.Snapshot.Observations[0].EvidenceReferences[0].Reference = "mutated"
		inspected, err := engine.Exchange(inspectFrame(authorized.RunID, "observed-success-inspect"))
		if err != nil || inspected.Snapshot.Observations[0].EvidenceReferences[0].Reference != "evidence://observed-success-result" {
			t.Fatalf("observation mutation reached committed state: %#v, %v", inspected, err)
		}
		assertRevisionCount(t, stateRoot, authorized.RunID, 4)
	})

	t.Run("failed", func(t *testing.T) {
		fixture := newBoundedRuntimeFixture(t, false)
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine := newBoundedEngine(t, stateRoot, fixture)
		authorized, grant := authorizeBoundedRun(t, engine, fixture, "observed-failure")
		paused, err := engine.Exchange(observedFrame(authorized, grant, runtime.ObservationFailed, "observed-failure-result"))
		if err != nil {
			t.Fatalf("failed CAPABILITY_OBSERVED error = %v", err)
		}
		if paused.Kind != runtime.ReplyPaused || paused.Snapshot.Status != runtime.RunPaused || paused.Reason != runtime.ReasonModeEscalationRequired || !equalStrings(paused.RecoveryActions, []string{runtime.RecoveryStartSuccessorRun}) || len(paused.Snapshot.Observations) != 1 {
			t.Fatalf("failed observation reply = %#v", paused)
		}
	})
}

func TestCapabilityObservedRejectsPrematureMismatchedAndRawOutcomes(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	granted, grant := grantBoundedRun(t, engine, fixture, "premature-observation")
	_, err := engine.Exchange(observedFrame(granted, grant, runtime.ObservationSucceeded, "premature-result"))
	assertErrorCode(t, err, "RUN_TRANSITION_INVALID")
	assertRevisionCount(t, stateRoot, granted.RunID, 2)

	for _, test := range []struct {
		name   string
		mutate func(*runtime.CapabilityObservation)
	}{
		{"wrong Grant", func(value *runtime.CapabilityObservation) { value.GrantID = "grant-wrong" }},
		{"wrong invocation", func(value *runtime.CapabilityObservation) { value.InvocationID = "invocation-wrong" }},
		{"wrong Executor", func(value *runtime.CapabilityObservation) { value.ExecutorID = "executor-wrong" }},
		{"unknown outcome", func(value *runtime.CapabilityObservation) { value.Outcome = "UNKNOWN" }},
		{"raw output", func(value *runtime.CapabilityObservation) { value.RawOutput = "untrusted provider output" }},
		{"whitespace raw output", func(value *runtime.CapabilityObservation) { value.RawOutput = " " }},
		{"missing evidence", func(value *runtime.CapabilityObservation) { value.EvidenceReferences = nil }},
		{"duplicate evidence", func(value *runtime.CapabilityObservation) {
			value.EvidenceReferences = append(value.EvidenceReferences, value.EvidenceReferences[0])
		}},
		{"empty evidence reference", func(value *runtime.CapabilityObservation) { value.EvidenceReferences[0].Reference = "" }},
		{"invalid evidence digest", func(value *runtime.CapabilityObservation) { value.EvidenceReferences[0].Digest = "bad" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			authorized, grant := authorizeBoundedRun(t, engine, fixture, "invalid-observation-"+strings.ReplaceAll(test.name, " ", "-"))
			frame := observedFrame(authorized, grant, runtime.ObservationSucceeded, "invalid-observation")
			test.mutate(frame.Continue.Observation)
			_, err := engine.Exchange(frame)
			assertErrorCode(t, err, "OBSERVATION_INVALID")
			assertRevisionCount(t, stateRoot, authorized.RunID, 3)
		})
	}
}

func TestBoundedEscalationAndExecutionUncertaintyPauseWithoutRetry(t *testing.T) {
	for _, signal := range []runtime.ContinueSignal{
		runtime.SignalScopeExpanded,
		runtime.SignalAdditionalCapabilityRequired,
		runtime.SignalRemediationRequired,
		runtime.SignalArchitectureRequired,
	} {
		t.Run(string(signal), func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			authorized, _ := authorizeBoundedRun(t, engine, fixture, "escalate-"+string(signal))
			frame := runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "escalation", IdempotencyKey: "escalation", RunID: authorized.RunID, ExpectedRevision: authorized.Revision, Continue: &runtime.ContinueInput{Signal: signal}}
			paused, err := engine.Exchange(frame)
			if err != nil {
				t.Fatalf("%s error = %v", signal, err)
			}
			if paused.Kind != runtime.ReplyPaused || paused.Snapshot.Status != runtime.RunPaused || paused.Reason != runtime.ReasonModeEscalationRequired || !equalStrings(paused.RecoveryActions, []string{runtime.RecoveryStartSuccessorRun}) || len(paused.Snapshot.Grants) != 1 || len(paused.Snapshot.Observations) != 0 {
				t.Fatalf("escalation reply = %#v", paused)
			}
			replayed, err := engine.Exchange(frame)
			if err != nil || !reflect.DeepEqual(replayed, paused) {
				t.Fatalf("escalation replay = %#v, %v", replayed, err)
			}
		})
	}

	t.Run("execution uncertain", func(t *testing.T) {
		fixture := newBoundedRuntimeFixture(t, false)
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine := newBoundedEngine(t, stateRoot, fixture)
		authorized, grant := authorizeBoundedRun(t, engine, fixture, "uncertain")
		frame := runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "uncertain", IdempotencyKey: "uncertain", RunID: authorized.RunID, ExpectedRevision: authorized.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalExecutionUncertain}}
		paused, err := engine.Exchange(frame)
		if err != nil {
			t.Fatalf("EXECUTION_UNCERTAIN error = %v", err)
		}
		if paused.Kind != runtime.ReplyPaused || paused.Snapshot.Status != runtime.RunPaused || paused.Reason != runtime.ReasonExecutionUncertain || !equalStrings(paused.RecoveryActions, []string{runtime.RecoveryReconcileInvocation}) || len(paused.Snapshot.Grants) != 1 || paused.Snapshot.Grants[0].InvocationID != grant.InvocationID {
			t.Fatalf("uncertain reply = %#v", paused)
		}
		replayed, err := engine.Exchange(frame)
		if err != nil || !reflect.DeepEqual(replayed, paused) {
			t.Fatalf("uncertainty replay = %#v, %v", replayed, err)
		}
		retry := dispatchPreparedFrame(authorized, grant, "uncertain-retry")
		retry.ExpectedRevision = paused.Revision
		_, err = engine.Exchange(retry)
		assertErrorCode(t, err, "RUN_TRANSITION_INVALID")
		assertRevisionCount(t, stateRoot, authorized.RunID, 4)
	})
}

func grantBoundedRun(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture, key string) (runtime.RunReply, admission.CapabilityGrant) {
	t.Helper()
	started, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", key+"-start"))
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	granted, err := engine.Exchange(runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: key + "-grant", IdempotencyKey: key + "-grant", RunID: started.RunID, ExpectedRevision: started.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}})
	if err != nil {
		t.Fatalf("REQUEST_DISPATCH error = %v", err)
	}
	return granted, granted.Snapshot.Grants[0]
}

func authorizeBoundedRun(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture, key string) (runtime.RunReply, admission.CapabilityGrant) {
	t.Helper()
	granted, grant := grantBoundedRun(t, engine, fixture, key)
	authorized, err := engine.Exchange(dispatchPreparedFrame(granted, grant, key+"-prepared"))
	if err != nil {
		t.Fatalf("DISPATCH_PREPARED error = %v", err)
	}
	return authorized, grant
}

func dispatchPreparedFrame(granted runtime.RunReply, grant admission.CapabilityGrant, key string) runtime.RunFrame {
	return runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: key, IdempotencyKey: key, RunID: granted.RunID, ExpectedRevision: granted.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalDispatchPrepared, DispatchPreparation: &runtime.DispatchPreparation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID}}}
}

func observedFrame(current runtime.RunReply, grant admission.CapabilityGrant, outcome runtime.ObservationOutcome, key string) runtime.RunFrame {
	return runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilityObserved, Observation: &runtime.CapabilityObservation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID, Outcome: outcome, EvidenceReferences: []runtime.EvidenceReference{{Reference: "evidence://" + key, Digest: strings.Repeat("2", 64)}}}}}
}
