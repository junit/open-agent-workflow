package runtime_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestBoundedRestartInspectsEveryHandshakeState(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T, *runtime.Engine, boundedRuntimeFixture) runtime.RunReply
	}{
		{"awaiting", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture) runtime.RunReply {
			reply, err := engine.Exchange(boundedStartFrame(fixture, nil, "", "restart-awaiting"))
			if err != nil {
				t.Fatal(err)
			}
			return reply
		}},
		{"ready", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture) runtime.RunReply {
			reply, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "restart-ready"))
			if err != nil {
				t.Fatal(err)
			}
			return reply
		}},
		{"granted", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture) runtime.RunReply {
			reply, _ := grantBoundedRun(t, engine, fixture, "restart-granted")
			return reply
		}},
		{"in flight", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture) runtime.RunReply {
			reply, _ := authorizeBoundedRun(t, engine, fixture, "restart-in-flight")
			return reply
		}},
		{"finished", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture) runtime.RunReply {
			reply, grant := authorizeBoundedRun(t, engine, fixture, "restart-finished")
			finished, err := engine.Exchange(observedFrame(reply, grant, runtime.ObservationSucceeded, "restart-finished-observed"))
			if err != nil {
				t.Fatal(err)
			}
			return finished
		}},
		{"mode paused", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture) runtime.RunReply {
			reply, _ := authorizeBoundedRun(t, engine, fixture, "restart-mode-paused")
			paused, err := engine.Exchange(runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "restart-mode-pause", IdempotencyKey: "restart-mode-pause", RunID: reply.RunID, ExpectedRevision: reply.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRemediationRequired}})
			if err != nil {
				t.Fatal(err)
			}
			return paused
		}},
		{"uncertain paused", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture) runtime.RunReply {
			reply, _ := authorizeBoundedRun(t, engine, fixture, "restart-uncertain-paused")
			paused, err := engine.Exchange(runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "restart-uncertain-pause", IdempotencyKey: "restart-uncertain-pause", RunID: reply.RunID, ExpectedRevision: reply.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalExecutionUncertain}})
			if err != nil {
				t.Fatal(err)
			}
			return paused
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			committed := test.setup(t, engine, fixture)
			restarted, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
			if err != nil {
				t.Fatal(err)
			}
			inspected, err := restarted.Exchange(inspectFrame(committed.RunID, "restart-inspect"))
			if err != nil {
				t.Fatalf("INSPECT after restart error = %v", err)
			}
			if !reflect.DeepEqual(inspected.Snapshot, committed.Snapshot) || inspected.RevisionDigest != committed.RevisionDigest {
				t.Fatalf("restart state differs\n got: %#v\nwant: %#v", inspected, committed)
			}
		})
	}
}

func TestBoundedConcurrentDispatchHasOneGrantWinner(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	started, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "concurrent-start"))
	if err != nil {
		t.Fatal(err)
	}
	frames := []runtime.RunFrame{
		{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "concurrent-one", IdempotencyKey: "concurrent-one", RunID: started.RunID, ExpectedRevision: started.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}},
		{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "concurrent-two", IdempotencyKey: "concurrent-two", RunID: started.RunID, ExpectedRevision: started.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}},
	}
	results := make(chan struct {
		reply runtime.RunReply
		err   error
	}, len(frames))
	var group sync.WaitGroup
	for _, frame := range frames {
		frame := frame
		group.Add(1)
		go func() {
			defer group.Done()
			reply, exchangeErr := engine.Exchange(frame)
			results <- struct {
				reply runtime.RunReply
				err   error
			}{reply: reply, err: exchangeErr}
		}()
	}
	group.Wait()
	close(results)
	winners := 0
	conflicts := 0
	for result := range results {
		if result.err == nil {
			winners++
			if result.reply.Snapshot.Status != runtime.RunGranted || len(result.reply.Snapshot.Grants) != 1 {
				t.Fatalf("winner reply = %#v", result.reply)
			}
		} else if runtime.ErrorCode(result.err) == "RUN_REVISION_CONFLICT" {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent dispatch error = %v", result.err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("concurrent results winners=%d conflicts=%d", winners, conflicts)
	}
	assertRevisionCount(t, stateRoot, started.RunID, 2)
}

func TestBoundedDispatchRequiresPinnedTrustedConfigurationAfterRestart(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	started, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "configuration-drift"))
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatal(err)
	}
	_, err = restarted.Exchange(runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "configuration-drift-dispatch", IdempotencyKey: "configuration-drift-dispatch", RunID: started.RunID, ExpectedRevision: started.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}})
	assertErrorCode(t, err, "BOUNDED_CONFIGURATION_REQUIRED")
	inspected, inspectErr := restarted.Exchange(inspectFrame(started.RunID, "configuration-drift-inspect"))
	if inspectErr != nil || inspected.Revision != 1 || inspected.Snapshot.Status != runtime.RunReady {
		t.Fatalf("configuration drift changed committed state: %#v, %v", inspected, inspectErr)
	}
}

func TestBoundedGrantAndAuthorizationRecoverMatchingOrphans(t *testing.T) {
	t.Run("Grant issuance", func(t *testing.T) {
		fixture := newBoundedRuntimeFixture(t, false)
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine := newBoundedEngine(t, stateRoot, fixture)
		started, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "orphan-grant-start"))
		if err != nil {
			t.Fatal(err)
		}
		headPath := filepath.Join(stateRoot, "runs", started.RunID, "HEAD")
		headBefore, err := os.ReadFile(headPath)
		if err != nil {
			t.Fatal(err)
		}
		dispatch := runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "orphan-grant", IdempotencyKey: "orphan-grant", RunID: started.RunID, ExpectedRevision: started.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}}
		first, err := engine.Exchange(dispatch)
		if err != nil {
			t.Fatal(err)
		}
		revisionPath := filepath.Join(stateRoot, "runs", started.RunID, "revisions", "00000000000000000002.json")
		rawRevision, err := os.ReadFile(revisionPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(headPath, headBefore, 0o600); err != nil {
			t.Fatal(err)
		}
		promoted, err := engine.Exchange(dispatch)
		if err != nil || !reflect.DeepEqual(promoted, first) {
			t.Fatalf("matching Grant orphan promotion = %#v, %v", promoted, err)
		}
		if len(rawRevision) == 0 {
			t.Fatal("Grant orphan revision is empty")
		}
	})

	t.Run("dispatch authorization", func(t *testing.T) {
		fixture := newBoundedRuntimeFixture(t, false)
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine := newBoundedEngine(t, stateRoot, fixture)
		granted, grant := grantBoundedRun(t, engine, fixture, "orphan-auth")
		headPath := filepath.Join(stateRoot, "runs", granted.RunID, "HEAD")
		headBefore, err := os.ReadFile(headPath)
		if err != nil {
			t.Fatal(err)
		}
		prepared := dispatchPreparedFrame(granted, grant, "orphan-auth-prepared")
		first, err := engine.Exchange(prepared)
		if err != nil {
			t.Fatal(err)
		}
		revisionPath := filepath.Join(stateRoot, "runs", granted.RunID, "revisions", "00000000000000000003.json")
		rawRevision, err := os.ReadFile(revisionPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(headPath, headBefore, 0o600); err != nil {
			t.Fatal(err)
		}
		promoted, err := engine.Exchange(prepared)
		if err != nil || !reflect.DeepEqual(promoted, first) || len(rawRevision) == 0 {
			t.Fatalf("matching authorization orphan promotion = %#v, %v", promoted, err)
		}
	})
}

func TestBoundedConflictingOrphansFailClosedAtGrantAndAuthorization(t *testing.T) {
	tests := []struct {
		name string
		step func(*testing.T, *runtime.Engine, boundedRuntimeFixture, string)
	}{
		{"Grant issuance", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture, stateRoot string) {
			started, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "conflict-grant-start"))
			if err != nil {
				t.Fatal(err)
			}
			writeConflictingOrphan(t, stateRoot, started.RunID, 2)
			_, err = engine.Exchange(runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "conflict-grant", IdempotencyKey: "conflict-grant", RunID: started.RunID, ExpectedRevision: started.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}})
			assertErrorCode(t, err, "RUN_STATE_WRITE_FAILED")
			inspected, inspectErr := engine.Exchange(inspectFrame(started.RunID, "conflict-grant-inspect"))
			if inspectErr != nil || inspected.Revision != 1 || inspected.Snapshot.Status != runtime.RunReady {
				t.Fatalf("Grant conflict changed committed state: %#v, %v", inspected, inspectErr)
			}
		}},
		{"authorization", func(t *testing.T, engine *runtime.Engine, fixture boundedRuntimeFixture, stateRoot string) {
			granted, grant := grantBoundedRun(t, engine, fixture, "conflict-auth")
			writeConflictingOrphan(t, stateRoot, granted.RunID, 3)
			_, err := engine.Exchange(dispatchPreparedFrame(granted, grant, "conflict-auth-prepared"))
			assertErrorCode(t, err, "RUN_STATE_WRITE_FAILED")
			inspected, inspectErr := engine.Exchange(inspectFrame(granted.RunID, "conflict-auth-inspect"))
			if inspectErr != nil || inspected.Revision != 2 || inspected.Snapshot.Status != runtime.RunGranted {
				t.Fatalf("authorization conflict changed committed state: %#v, %v", inspected, inspectErr)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			test.step(t, engine, fixture, stateRoot)
		})
	}
}

func TestBoundedStatePermissionsAndFutureAuthorityFieldsRemainClosed(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	granted, _ := grantBoundedRun(t, engine, fixture, "permissions")
	runRoot := filepath.Join(stateRoot, "runs", granted.RunID)
	for _, test := range []struct {
		name string
		path string
		mode os.FileMode
	}{
		{"run directory", runRoot, 0o700},
		{"lock", filepath.Join(runRoot, "LOCK"), 0o600},
		{"revisions directory", filepath.Join(runRoot, "revisions"), 0o700},
		{"head", filepath.Join(runRoot, "HEAD"), 0o600},
		{"revision", filepath.Join(runRoot, "revisions", "00000000000000000002.json"), 0o600},
	} {
		info, err := os.Stat(test.path)
		if err != nil {
			t.Fatalf("stat %s: %v", test.name, err)
		}
		if info.Mode().Perm() != test.mode {
			t.Fatalf("%s mode = %o, want %o", test.name, info.Mode().Perm(), test.mode)
		}
	}
	raw, err := json.Marshal(granted.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"stage_grant", "host_manifest", "profile_recipe"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("future authority field %q leaked into Bounded snapshot: %s", forbidden, raw)
		}
	}
	if !strings.Contains(string(raw), "resource_lease_ids") {
		t.Fatal("Ticket 06 compatibility ResourceLeaseIDs field disappeared")
	}
	if len(granted.Snapshot.ResourceLeaseIDs) != 0 || len(granted.Snapshot.LifecycleBundles) != 0 {
		t.Fatalf("future authority was issued: %#v", granted.Snapshot)
	}
}

func writeConflictingOrphan(t *testing.T, stateRoot, runID string, revision uint64) {
	t.Helper()
	var name string
	if revision == 2 {
		name = "00000000000000000002.json"
	} else if revision == 3 {
		name = "00000000000000000003.json"
	} else {
		t.Fatalf("unsupported orphan revision %d", revision)
	}
	path := filepath.Join(stateRoot, "runs", runID, "revisions", name)
	if err := os.WriteFile(path, []byte("conflicting orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
}
