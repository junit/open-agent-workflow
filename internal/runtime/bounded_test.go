package runtime_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestStartBoundedWithVerifiedUserIntentPersistsReady(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	selector := &classification.CapabilitySelector{
		ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent,
	}
	reply, err := engine.Exchange(boundedStartFrame(fixture, selector, "", "bounded-ready"))
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}
	if reply.Kind != runtime.ReplyModeDecided || reply.Revision != 1 || reply.Snapshot.Status != runtime.RunReady || reply.Snapshot.RequestMode != classification.RequestModeBounded {
		t.Fatalf("Bounded START reply = %#v", reply)
	}
	if reply.Snapshot.Bounded == nil || !reflect.DeepEqual(reply.Snapshot.Bounded.Selector, selector) {
		t.Fatalf("Bounded state = %#v", reply.Snapshot.Bounded)
	}
	if reply.Snapshot.Bounded.ConfigurationDigest != fixture.snapshot.Digest() || reply.Snapshot.Bounded.RegistryDigest != fixture.registry.Digest() || reply.Snapshot.Bounded.CatalogDigest != fixture.snapshot.Catalog().Digest() {
		t.Fatalf("trusted input digests = %#v", reply.Snapshot.Bounded)
	}
	if len(reply.Snapshot.GrantIDs) != 0 || len(reply.Snapshot.LifecycleBundles) != 0 || len(reply.Snapshot.ResourceLeaseIDs) != 0 {
		t.Fatalf("START minted authority = %#v", reply.Snapshot)
	}
	inspected, err := engine.Exchange(inspectFrame(reply.RunID, "inspect-bounded-ready"))
	if err != nil {
		t.Fatalf("INSPECT error = %v", err)
	}
	if !reflect.DeepEqual(inspected.Snapshot, reply.Snapshot) {
		t.Fatalf("INSPECT state differs\n got: %#v\nwant: %#v", inspected.Snapshot, reply.Snapshot)
	}
}

func TestStartBoundedWithoutAdmissibleSelectorPersistsAwaiting(t *testing.T) {
	for _, test := range []struct {
		name        string
		selector    *classification.CapabilitySelector
		ruleID      string
		defaultRule bool
		diagnostic  string
	}{
		{name: "missing selector", diagnostic: "CAPABILITY_SELECTION_REQUIRED"},
		{name: "unverified provider", selector: &classification.CapabilitySelector{ProviderID: "missing/provider", CapabilityID: "review", Source: classification.SelectorUserIntent}, diagnostic: "CAPABILITY_NOT_VERIFIED"},
		{name: "mode not allowed", selector: &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "planning", Source: classification.SelectorUserIntent}, diagnostic: "CAPABILITY_MODE_NOT_ALLOWED"},
		{name: "missing trusted rule", selector: &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorTrustedRule}, ruleID: "missing", diagnostic: "CAPABILITY_NOT_VERIFIED"},
		{name: "mismatched trusted rule", selector: &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "security-review", Source: classification.SelectorTrustedRule}, ruleID: "review", defaultRule: true, diagnostic: "CAPABILITY_NOT_VERIFIED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, test.defaultRule)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			reply, err := engine.Exchange(boundedStartFrame(fixture, test.selector, test.ruleID, "awaiting-"+strings.ReplaceAll(test.name, " ", "-")))
			if err != nil {
				t.Fatalf("Exchange(START) error = %v", err)
			}
			if reply.Kind != runtime.ReplyCapabilitySelectionRequired || reply.Revision != 1 || reply.Snapshot.Status != runtime.RunAwaitingCapability || reply.Snapshot.RequestMode != classification.RequestModeBounded {
				t.Fatalf("awaiting reply = %#v", reply)
			}
			if reply.Snapshot.Bounded == nil || reply.Snapshot.Bounded.Selector != nil {
				t.Fatalf("untrusted selector entered Bounded state: %#v", reply.Snapshot.Bounded)
			}
			assertDiagnosticCodes(t, reply.Diagnostics, test.diagnostic)
			assertRevisionCount(t, stateRoot, reply.RunID, 1)
		})
	}
}

func TestStartBoundedResolvesOnlyExactPinnedTrustedRule(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, true)
	for _, test := range []struct {
		name     string
		selector *classification.CapabilitySelector
	}{
		{
			name: "matching selector",
			selector: &classification.CapabilitySelector{
				ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorTrustedRule,
			},
		},
		{name: "rule reference derives selector"},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := newBoundedEngine(t, filepath.Join(t.TempDir(), "state"), fixture)
			reply, err := engine.Exchange(boundedStartFrame(fixture, test.selector, "review", "trusted-"+strings.ReplaceAll(test.name, " ", "-")))
			if err != nil {
				t.Fatalf("Exchange(START) error = %v", err)
			}
			if reply.Kind != runtime.ReplyModeDecided || reply.Snapshot.Status != runtime.RunReady || reply.Snapshot.Bounded == nil || reply.Snapshot.Bounded.Selector == nil {
				t.Fatalf("trusted-rule reply = %#v", reply)
			}
			if selector := reply.Snapshot.Bounded.Selector; selector.ProviderID != "oaw/ecc" || selector.CapabilityID != "review" || selector.Source != classification.SelectorTrustedRule {
				t.Fatalf("resolved selector = %#v", selector)
			}
		})
	}
}

func TestStartBoundedRejectsForgedRuleProvenanceAndMissingTrustedInputs(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, true)
	selector := &classification.CapabilitySelector{
		ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent,
	}
	for _, test := range []struct {
		name    string
		options runtime.Options
		frame   runtime.RunFrame
		code    string
	}{
		{
			name:    "user intent carrying rule ID",
			options: boundedOptions(filepath.Join(t.TempDir(), "state"), fixture),
			frame:   boundedStartFrame(fixture, selector, "review", "forged-user-intent"),
			code:    "BOUNDED_REQUEST_INVALID",
		},
		{
			name:    "trusted rule without ID",
			options: boundedOptions(filepath.Join(t.TempDir(), "state"), fixture),
			frame:   boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorTrustedRule}, "", "missing-rule-id"),
			code:    "BOUNDED_REQUEST_INVALID",
		},
		{
			name:    "missing pinned configuration",
			options: runtime.Options{StateRoot: filepath.Join(t.TempDir(), "state")},
			frame:   boundedStartFrame(fixture, selector, "", "missing-bounded-options"),
			code:    "BOUNDED_CONFIGURATION_REQUIRED",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, err := runtime.NewEngine(test.options)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}
			_, err = engine.Exchange(test.frame)
			assertErrorCode(t, err, test.code)
			assertRunsRootEmpty(t, test.options.StateRoot)
		})
	}
}

func TestStartBoundedRejectsMalformedRequestWithoutState(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*runtime.RunFrame)
	}{
		{"missing Bounded input", func(frame *runtime.RunFrame) { frame.Start.Bounded = nil }},
		{"deliverable ID", func(frame *runtime.RunFrame) { frame.Start.Bounded.DeliverableID = "bad\ndeliverable" }},
		{"input digest", func(frame *runtime.RunFrame) { frame.Start.Bounded.InputDigest = "bad" }},
		{"termination condition", func(frame *runtime.RunFrame) { frame.Start.Bounded.TerminationCondition = "\n" }},
		{"executor ID", func(frame *runtime.RunFrame) { frame.Start.Bounded.ExecutorID = "bad\nexecutor" }},
		{"empty effects", func(frame *runtime.RunFrame) { frame.Start.Bounded.RequestedEffects = nil }},
		{"duplicate effects", func(frame *runtime.RunFrame) {
			frame.Start.Bounded.RequestedEffects = []string{"read-project", "read-project"}
		}},
		{"empty effect member", func(frame *runtime.RunFrame) { frame.Start.Bounded.RequestedEffects = []string{""} }},
		{"empty resources", func(frame *runtime.RunFrame) { frame.Start.Bounded.RequestedResources = nil }},
		{"duplicate resources", func(frame *runtime.RunFrame) { frame.Start.Bounded.RequestedResources = []string{"project", "project"} }},
		{"trusted rule ID", func(frame *runtime.RunFrame) { frame.Start.Bounded.TrustedRuleID = "Bad Rule" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			frame := boundedStartFrame(fixture, nil, "", "malformed-"+strings.ReplaceAll(test.name, " ", "-"))
			test.mutate(&frame)
			_, err := engine.Exchange(frame)
			assertErrorCode(t, err, "BOUNDED_REQUEST_INVALID")
			assertRunsRootEmpty(t, stateRoot)
		})
	}
}

func TestCapabilitySelectedRejectsMalformedSelectorFrames(t *testing.T) {
	for _, test := range []struct {
		name     string
		selector *classification.CapabilitySelector
		ruleID   string
	}{
		{name: "provider ID", selector: &classification.CapabilitySelector{ProviderID: "bad", CapabilityID: "review", Source: classification.SelectorUserIntent}},
		{name: "capability ID", selector: &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "Bad Capability", Source: classification.SelectorUserIntent}},
		{name: "selector source", selector: &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: "host-claim"}},
		{name: "trusted rule ID", ruleID: "Bad Rule"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := newBoundedEngine(t, stateRoot, fixture)
			started, err := engine.Exchange(boundedStartFrame(fixture, nil, "", "malformed-selection-start"))
			if err != nil {
				t.Fatalf("START error = %v", err)
			}
			_, err = engine.Exchange(runtime.RunFrame{
				SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "malformed-selection", IdempotencyKey: "malformed-selection", RunID: started.RunID, ExpectedRevision: 1,
				Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilitySelected, CapabilitySelector: test.selector, TrustedRuleID: test.ruleID},
			})
			assertErrorCode(t, err, "BOUNDED_REQUEST_INVALID")
			assertRevisionCount(t, stateRoot, started.RunID, 1)
		})
	}
}

func TestCapabilitySelectedMovesAwaitingBoundedRunToReadyWithoutReclassification(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	started, err := engine.Exchange(boundedStartFrame(fixture, nil, "", "select-later-start"))
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}
	selector := &classification.CapabilitySelector{
		ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent,
	}
	frame := runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "select-later", IdempotencyKey: "select-later", RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilitySelected, CapabilitySelector: selector},
	}
	selected, err := engine.Exchange(frame)
	if err != nil {
		t.Fatalf("Exchange(CAPABILITY_SELECTED) error = %v", err)
	}
	if selected.Kind != runtime.ReplyModeDecided || selected.Revision != 2 || selected.Snapshot.Status != runtime.RunReady || selected.Snapshot.Bounded == nil || !reflect.DeepEqual(selected.Snapshot.Bounded.Selector, selector) {
		t.Fatalf("selected reply = %#v", selected)
	}
	if selected.Snapshot.ClassificationDigest != started.Snapshot.ClassificationDigest || selected.Snapshot.Classification.CapabilitySelector != nil {
		t.Fatal("CAPABILITY_SELECTED reclassified the Run")
	}

	replay := frame
	replay.MessageID = "select-later-replay"
	replayed, err := engine.Exchange(replay)
	if err != nil {
		t.Fatalf("replay error = %v", err)
	}
	if !reflect.DeepEqual(replayed, selected) {
		t.Fatalf("replay differs\n got: %#v\nwant: %#v", replayed, selected)
	}
	assertRevisionCount(t, stateRoot, started.RunID, 2)

	restarted := newBoundedEngine(t, stateRoot, fixture)
	inspected, err := restarted.Exchange(inspectFrame(started.RunID, "inspect-after-restart"))
	if err != nil {
		t.Fatalf("INSPECT after restart error = %v", err)
	}
	if !reflect.DeepEqual(inspected.Snapshot, selected.Snapshot) {
		t.Fatalf("restart state differs\n got: %#v\nwant: %#v", inspected.Snapshot, selected.Snapshot)
	}
}

func TestBoundedSelectionRejectsInvalidTransitionsWithoutMutation(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	started, err := engine.Exchange(boundedStartFrame(fixture, nil, "", "invalid-selection-start"))
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	invalidFrames := []struct {
		name  string
		frame runtime.RunFrame
		code  string
	}{
		{
			name: "empty selection",
			frame: runtime.RunFrame{
				SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "empty-selection", IdempotencyKey: "empty-selection", RunID: started.RunID, ExpectedRevision: 1,
				Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilitySelected},
			},
			code: "BOUNDED_REQUEST_INVALID",
		},
		{
			name: "unverified selection",
			frame: runtime.RunFrame{
				SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "unverified-selection", IdempotencyKey: "unverified-selection", RunID: started.RunID, ExpectedRevision: 1,
				Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilitySelected, CapabilitySelector: &classification.CapabilitySelector{ProviderID: "missing/provider", CapabilityID: "review", Source: classification.SelectorUserIntent}},
			},
			code: "CAPABILITY_NOT_VERIFIED",
		},
	}
	for _, test := range invalidFrames {
		t.Run(test.name, func(t *testing.T) {
			_, err := engine.Exchange(test.frame)
			assertErrorCode(t, err, test.code)
			assertRevisionCount(t, stateRoot, started.RunID, 1)
		})
	}

	ready, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "valid-selection", IdempotencyKey: "valid-selection", RunID: started.RunID, ExpectedRevision: 1,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilitySelected, CapabilitySelector: &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}},
	})
	if err != nil {
		t.Fatalf("valid selection error = %v", err)
	}
	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "second-selection", IdempotencyKey: "second-selection", RunID: started.RunID, ExpectedRevision: ready.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalCapabilitySelected, CapabilitySelector: &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}},
	})
	assertErrorCode(t, err, "RUN_TRANSITION_INVALID")
	assertRevisionCount(t, stateRoot, started.RunID, 2)
}

func TestBoundedRecordsAreDefensiveAndStartReplayDoesNotNeedCurrentAdmission(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	frame := boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "bounded-copy")
	started, err := engine.Exchange(frame)
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	expected, err := engine.Exchange(frame)
	if err != nil {
		t.Fatalf("initial replay error = %v", err)
	}
	started.Snapshot.Bounded.Input.RequestedEffects[0] = "mutated"
	started.Snapshot.Bounded.Input.RequestedResources[0] = "mutated"
	started.Snapshot.Bounded.Selector.CapabilityID = "mutated"
	inspected, err := engine.Exchange(inspectFrame(started.RunID, "bounded-copy-inspect"))
	if err != nil {
		t.Fatalf("INSPECT error = %v", err)
	}
	if inspected.Snapshot.Bounded.Input.RequestedEffects[0] != "read-project" || inspected.Snapshot.Bounded.Input.RequestedResources[0] != "project" || inspected.Snapshot.Bounded.Selector.CapabilityID != "review" {
		t.Fatalf("Bounded state exposed mutable aliases: %#v", inspected.Snapshot.Bounded)
	}

	restarted, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine(replay without current admission) error = %v", err)
	}
	replayed, err := restarted.Exchange(frame)
	if err != nil {
		t.Fatalf("START replay without current admission error = %v", err)
	}
	if !reflect.DeepEqual(replayed, expected) {
		t.Fatalf("replayed committed START differs\n got: %#v\nwant: %#v", replayed, expected)
	}
}

func TestRequestDispatchIssuesOneImmutableBoundedGrant(t *testing.T) {
	fixture := newBoundedRuntimeFixture(t, false)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newBoundedEngine(t, stateRoot, fixture)
	started, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "grant-start"))
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	dispatch := runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "grant-dispatch", IdempotencyKey: "grant-dispatch", RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch},
	}
	granted, err := engine.Exchange(dispatch)
	if err != nil {
		t.Fatalf("REQUEST_DISPATCH error = %v", err)
	}
	if granted.Kind != runtime.ReplyGrantIssued || granted.Revision != 2 || granted.Snapshot.Status != runtime.RunGranted || len(granted.Snapshot.Grants) != 1 || len(granted.Snapshot.GrantIDs) != 1 {
		t.Fatalf("Grant reply = %#v", granted)
	}
	grant := granted.Snapshot.Grants[0]
	if grant.ID != granted.Snapshot.GrantIDs[0] || grant.IssuedRevision != 2 || grant.InvocationID == "" || grant.ProviderID != "oaw/ecc" || grant.CapabilityID != "review" || grant.Executor.ID != "executor-review" || !equalStrings(grant.Effects, []string{"read-project"}) || !equalStrings(grant.Resources, []string{"project"}) {
		t.Fatalf("Grant = %#v", grant)
	}
	if err := admission.ValidateGrant(grant); err != nil {
		t.Fatalf("ValidateGrant() error = %v", err)
	}
	expected, err := engine.Exchange(dispatch)
	if err != nil {
		t.Fatalf("initial REQUEST_DISPATCH replay error = %v", err)
	}
	grant.Effects[0] = "mutated"
	granted.Snapshot.Grants[0].Effects[0] = "mutated-again"
	inspected, err := engine.Exchange(inspectFrame(started.RunID, "grant-inspect"))
	if err != nil {
		t.Fatalf("INSPECT error = %v", err)
	}
	if inspected.Snapshot.Grants[0].Effects[0] != "read-project" {
		t.Fatal("Grant copy mutation reached committed state")
	}
	replay, err := engine.Exchange(dispatch)
	if err != nil {
		t.Fatalf("REQUEST_DISPATCH replay error = %v", err)
	}
	if !reflect.DeepEqual(replay, expected) {
		t.Fatalf("Grant replay differs\n got: %#v\nwant: %#v", replay, expected)
	}
	assertRevisionCount(t, stateRoot, started.RunID, 2)

	restarted := newBoundedEngine(t, stateRoot, fixture)
	restartedState, err := restarted.Exchange(inspectFrame(started.RunID, "grant-inspect-restart"))
	if err != nil {
		t.Fatalf("INSPECT after restart error = %v", err)
	}
	if !reflect.DeepEqual(restartedState.Snapshot, inspected.Snapshot) {
		t.Fatalf("Grant state after restart differs\n got: %#v\nwant: %#v", restartedState.Snapshot, inspected.Snapshot)
	}
}

func TestRequestDispatchFailsClosedWithoutMutation(t *testing.T) {
	for _, test := range []struct {
		name       string
		start      func(boundedRuntimeFixture) runtime.RunFrame
		options    func(string, boundedRuntimeFixture) runtime.Options
		startFirst bool
		code       string
	}{
		{
			name: "awaiting capability",
			start: func(fixture boundedRuntimeFixture) runtime.RunFrame {
				return boundedStartFrame(fixture, nil, "", "dispatch-awaiting")
			},
			options:    boundedOptions,
			startFirst: true,
			code:       "RUN_TRANSITION_INVALID",
		},
		{
			name: "authority exceeded",
			start: func(fixture boundedRuntimeFixture) runtime.RunFrame {
				return boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "dispatch-authority")
			},
			options: func(stateRoot string, fixture boundedRuntimeFixture) runtime.Options {
				options := boundedOptions(stateRoot, fixture)
				options.Bounded.Authority.Effects = []string{}
				return options
			},
			startFirst: true,
			code:       "CAPABILITY_AUTHORITY_EXCEEDED",
		},
		{
			name: "Executor missing",
			start: func(fixture boundedRuntimeFixture) runtime.RunFrame {
				return boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "dispatch-executor")
			},
			options: func(stateRoot string, fixture boundedRuntimeFixture) runtime.Options {
				options := boundedOptions(stateRoot, fixture)
				options.Bounded.Executors = nil
				return options
			},
			startFirst: true,
			code:       "EXECUTOR_NOT_REGISTERED",
		},
		{
			name: "Main Agent topology",
			start: func(fixture boundedRuntimeFixture) runtime.RunFrame {
				frame := boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "dispatch-main")
				frame.Start.Bounded.ExecutorID = "main-agent"
				return frame
			},
			options: func(stateRoot string, fixture boundedRuntimeFixture) runtime.Options {
				options := boundedOptions(stateRoot, fixture)
				options.Bounded.Executors = append(options.Bounded.Executors, admission.ExecutorRegistration{ID: "main-agent", Kind: admission.ExecutorMainAgent})
				return options
			},
			startFirst: true,
			code:       "EXECUTOR_TOPOLOGY_DENIED",
		},
		{
			name: "Git completion effect",
			start: func(fixture boundedRuntimeFixture) runtime.RunFrame {
				frame := boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "dispatch-git")
				frame.Start.Bounded.RequestedEffects = []string{"git-local"}
				return frame
			},
			options:    boundedOptions,
			startFirst: true,
			code:       "CAPABILITY_EFFECT_NOT_ALLOWED",
		},
		{
			name: "project write requires Resource Lease",
			start: func(fixture boundedRuntimeFixture) runtime.RunFrame {
				frame := boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "architecture", Source: classification.SelectorUserIntent}, "", "dispatch-write")
				frame.Start.Bounded.RequestedEffects = []string{"write-project"}
				frame.Start.Bounded.RequestedResources = []string{"project-worktree"}
				return frame
			},
			options: func(stateRoot string, fixture boundedRuntimeFixture) runtime.Options {
				options := boundedOptions(stateRoot, fixture)
				options.Bounded.Authority.Effects = append(options.Bounded.Authority.Effects, "write-project")
				options.Bounded.Authority.Resources = append(options.Bounded.Authority.Resources, "project-worktree")
				options.Bounded.Authority.ResourceLeases = true
				return options
			},
			startFirst: true,
			code:       "RESOURCE_LEASE_REQUIRED",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newBoundedRuntimeFixture(t, false)
			stateRoot := filepath.Join(t.TempDir(), "state")
			options := test.options(stateRoot, fixture)
			engine, err := runtime.NewEngine(options)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}
			started, err := engine.Exchange(test.start(fixture))
			if err != nil {
				t.Fatalf("START error = %v", err)
			}
			if test.startFirst && started.Snapshot.Status == runtime.RunAwaitingCapability {
				_, err = engine.Exchange(runtime.RunFrame{
					SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "dispatch-awaiting-request", IdempotencyKey: "dispatch-awaiting-request", RunID: started.RunID, ExpectedRevision: 1,
					Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch},
				})
			} else {
				_, err = engine.Exchange(runtime.RunFrame{
					SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "dispatch-failure", IdempotencyKey: "dispatch-failure", RunID: started.RunID, ExpectedRevision: 1,
					Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch},
				})
			}
			assertErrorCode(t, err, test.code)
			assertRevisionCount(t, stateRoot, started.RunID, 1)
		})
	}

	t.Run("second dispatch", func(t *testing.T) {
		fixture := newBoundedRuntimeFixture(t, false)
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine := newBoundedEngine(t, stateRoot, fixture)
		started, err := engine.Exchange(boundedStartFrame(fixture, &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}, "", "dispatch-second"))
		if err != nil {
			t.Fatal(err)
		}
		frame := runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "dispatch-once", IdempotencyKey: "dispatch-once", RunID: started.RunID, ExpectedRevision: 1, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}}
		granted, err := engine.Exchange(frame)
		if err != nil {
			t.Fatal(err)
		}
		_, err = engine.Exchange(runtime.RunFrame{SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue, MessageID: "dispatch-twice", IdempotencyKey: "dispatch-twice", RunID: started.RunID, ExpectedRevision: granted.Revision, Continue: &runtime.ContinueInput{Signal: runtime.SignalRequestDispatch}})
		assertErrorCode(t, err, "RUN_TRANSITION_INVALID")
		assertRevisionCount(t, stateRoot, started.RunID, 2)
	})
}

type boundedRuntimeFixture struct {
	projectRoot string
	snapshot    config.Snapshot
	resolutions registry.ResolutionReport
	registry    registry.Registry
}

func newBoundedRuntimeFixture(t *testing.T, withDefault bool) boundedRuntimeFixture {
	t.Helper()
	projectRoot := t.TempDir()
	userRoot := t.TempDir()
	userConfig := "schema_version = \"oaw.user-config/v2\"\n"
	if withDefault {
		userConfig += `
[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/ecc"
capability_id = "review"
`
	}
	writeTestFile(t, filepath.Join(userRoot, "config.toml"), []byte(userConfig))
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	home := t.TempDir()
	eccPath := filepath.Join(home, ".agents/skills/everything-claude-code/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(eccPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(ECC fixture) error = %v", err)
	}
	writeTestFile(t, eccPath, []byte("---\nname: everything-claude-code\n---\n"))
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("discovery.Discover() error = %v", err)
	}
	inventory := hosttest.ObserveProviderBindings(t, snapshot.Catalog(), evidence, home, "oaw/ecc")
	resolutions, effective, err := registry.Resolve(snapshot, "codex", evidence, &inventory)
	if err != nil {
		t.Fatalf("registry.Resolve() error = %v", err)
	}
	return boundedRuntimeFixture{projectRoot: projectRoot, snapshot: snapshot, resolutions: resolutions, registry: effective}
}

func newAmbiguousBoundedRuntimeFixture(t *testing.T) (boundedRuntimeFixture, string) {
	t.Helper()
	projectRoot := t.TempDir()
	userRoot := t.TempDir()
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	home := t.TempDir()
	for _, relative := range []string{
		".codex/plugins/cache/openai-api-curated/superpowers/6.1.1/skills/using-superpowers/SKILL.md",
		".codex/plugins/cache/openai-api-curated/superpowers/11c74d6b/skills/using-superpowers/SKILL.md",
	} {
		path := filepath.Join(home, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
		}
		writeTestFile(t, path, []byte("ambiguous superpowers"))
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("discovery.Discover() error = %v", err)
	}
	resolutions, effective, err := registry.Resolve(snapshot, "codex", evidence, nil)
	if err != nil {
		t.Fatalf("registry.Resolve() error = %v", err)
	}
	return boundedRuntimeFixture{
		projectRoot: projectRoot,
		snapshot:    snapshot,
		resolutions: resolutions,
		registry:    effective,
	}, home
}

func newBoundedEngine(t *testing.T, stateRoot string, fixture boundedRuntimeFixture) *runtime.Engine {
	t.Helper()
	engine, err := runtime.NewEngine(boundedOptions(stateRoot, fixture))
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	return engine
}

func boundedOptions(stateRoot string, fixture boundedRuntimeFixture) runtime.Options {
	return runtime.Options{
		StateRoot: stateRoot,
		Bounded: runtime.BoundedOptions{
			Configuration: fixture.snapshot,
			Resolutions:   fixture.resolutions,
			Registry:      fixture.registry,
			Authority: admission.AuthorityCeiling{
				Effects: []string{"read-project"}, Resources: []string{"project"},
			},
			Executors: []admission.ExecutorRegistration{{ID: "executor-review", Kind: admission.ExecutorIsolated}},
		},
	}
}

func TestStartBoundedReportsAmbiguousProviderResolution(t *testing.T) {
	fixture, home := newAmbiguousBoundedRuntimeFixture(t)
	selector := &classification.CapabilitySelector{
		ProviderID: "oaw/superpowers", CapabilityID: "review", Source: classification.SelectorUserIntent,
	}
	engine := newBoundedEngine(t, filepath.Join(t.TempDir(), "state"), fixture)
	reply, err := engine.Exchange(boundedStartFrame(fixture, selector, "", "ambiguous-provider"))
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}
	if reply.Kind != runtime.ReplyCapabilitySelectionRequired || reply.Snapshot.Status != runtime.RunAwaitingCapability {
		t.Fatalf("reply = %#v", reply)
	}
	if len(reply.Diagnostics) != 1 || reply.Diagnostics[0].Code != "PROVIDER_CANDIDATE_AMBIGUOUS" {
		t.Fatalf("diagnostics = %#v", reply.Diagnostics)
	}
	if strings.Contains(reply.Diagnostics[0].Message, home) || strings.Contains(reply.Diagnostics[0].Message, "SKILL.md") {
		t.Fatalf("Runtime diagnostic leaked discovery paths: %q", reply.Diagnostics[0].Message)
	}
}

func TestStartBoundedReportsConcreteProviderStates(t *testing.T) {
	reviewBinding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "superpowers:requesting-code-review"}
	tests := []struct {
		name       string
		userConfig string
		versions   []string
		bindings   []catalog.HostBinding
		inventory  bool
		want       string
	}{
		{name: "not found", inventory: true, bindings: []catalog.HostBinding{reviewBinding}, want: "PROVIDER_NOT_FOUND"},
		{name: "discovered unverified", versions: []string{"6.1.1"}, want: "HOST_BINDING_EVIDENCE_REQUIRED"},
		{name: "pin incompatible", userConfig: "schema_version = \"oaw.user-config/v2\"\n[[provider_pins]]\nprovider_id = \"oaw/superpowers\"\nhost_id = \"codex\"\ninstallation_key = \"installation-incompatible\"\nevidence_digest = \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"\nversion = \"9.9.9\"\n", versions: []string{"6.1.1"}, inventory: true, bindings: []catalog.HostBinding{reviewBinding}, want: "PROVIDER_PIN_INCOMPATIBLE"},
		{name: "binding unavailable", versions: []string{"6.1.1"}, inventory: true, want: "HOST_BINDING_EVIDENCE_REQUIRED"},
		{name: "disabled", userConfig: "schema_version = \"oaw.user-config/v2\"\ndenied_providers = [\"oaw/superpowers\"]\n", versions: []string{"6.1.1"}, inventory: true, bindings: []catalog.HostBinding{reviewBinding}, want: "PROVIDER_DISABLED_BY_USER"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSuperpowersBoundedStateFixture(t, test.userConfig, test.versions, test.bindings, test.inventory)
			selector := &classification.CapabilitySelector{ProviderID: "oaw/superpowers", CapabilityID: "review", Source: classification.SelectorUserIntent}
			engine := newBoundedEngine(t, filepath.Join(t.TempDir(), "state"), fixture)
			reply, err := engine.Exchange(boundedStartFrame(fixture, selector, "", "provider-state-"+strings.ReplaceAll(test.name, " ", "-")))
			if err != nil {
				t.Fatalf("Exchange(START) error = %v", err)
			}
			assertDiagnosticCodes(t, reply.Diagnostics, test.want)
		})
	}

	t.Run("untrusted", func(t *testing.T) {
		fixture := newUntrustedBoundedStateFixture(t)
		selector := &classification.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "review", Source: classification.SelectorUserIntent}
		engine := newBoundedEngine(t, filepath.Join(t.TempDir(), "state"), fixture)
		reply, err := engine.Exchange(boundedStartFrame(fixture, selector, "", "provider-state-untrusted"))
		if err != nil {
			t.Fatalf("Exchange(START) error = %v", err)
		}
		assertDiagnosticCodes(t, reply.Diagnostics, "PROVIDER_PROJECT_CONTENT_UNTRUSTED")
	})
}

func newSuperpowersBoundedStateFixture(t *testing.T, userConfig string, versions []string, bindings []catalog.HostBinding, inventorySet bool) boundedRuntimeFixture {
	t.Helper()
	projectRoot := t.TempDir()
	userRoot := t.TempDir()
	if userConfig != "" {
		writeTestFile(t, filepath.Join(userRoot, "config.toml"), []byte(userConfig))
	}
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	for _, version := range versions {
		path := filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", version, "skills", "using-superpowers", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, path, []byte(version))
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	var inventory *host.BindingInventory
	if inventorySet && len(evidence.Candidates("oaw/superpowers")) == 1 {
		candidate := evidence.Candidates("oaw/superpowers")[0]
		observations := make([]host.BindingObservation, 0, len(bindings))
		for index, binding := range bindings {
			observations = append(observations, host.BindingObservation{HostID: "codex", InstallationKey: candidate.InstallationKey, Binding: binding, Source: "host-filesystem", EvidenceReference: filepath.Join(candidate.Location, "evidence", fmt.Sprintf("%d", index)), Digest: strings.Repeat("a", 64)})
		}
		value, inventoryErr := host.NewBindingInventory("codex", observations)
		if inventoryErr != nil {
			t.Fatal(inventoryErr)
		}
		inventory = &value
	}
	resolutions, effective, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	return boundedRuntimeFixture{projectRoot: projectRoot, snapshot: snapshot, resolutions: resolutions, registry: effective}
}

func newUntrustedBoundedStateFixture(t *testing.T) boundedRuntimeFixture {
	t.Helper()
	projectRoot := t.TempDir()
	providerPath := filepath.Join(projectRoot, ".oaw", "providers", "acme.toml")
	if err := os.MkdirAll(filepath.Dir(providerPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, providerPath, []byte(testUntrustedProviderTOML))
	writeTestFile(t, filepath.Join(projectRoot, ".oaw", "config.toml"), []byte("schema_version = \"oaw.project-config/v1\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.toml\"\n"))
	userRoot := t.TempDir()
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	resolutions, effective, err := registry.Resolve(snapshot, "codex", evidence, nil)
	if err != nil {
		t.Fatal(err)
	}
	return boundedRuntimeFixture{projectRoot: projectRoot, snapshot: snapshot, resolutions: resolutions, registry: effective}
}

const testUntrustedProviderTOML = `
schema_version = "oaw.provider-descriptor/v2"
descriptor_version = "2.0.0"
id = "acme/suite"
display_name = "Acme Suite"

[[discovery]]
id = "acme"
hosts = ["codex"]
surface = "codex-user-skills"
distribution = "acme"
kind = "path-exists"
root = "user-home"
candidate_path = ".agents/acme"
evidence_path = "SKILL.md"

[[capabilities]]
id = "review"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
maximum_effects = ["read-project"]
resources = ["project"]
request_modes = ["BOUNDED"]
responsibilities = ["review"]
executor_topology = "isolated-required"
delegation_allow_list = []

[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:review"
`

func boundedStartFrame(fixture boundedRuntimeFixture, selector *classification.CapabilitySelector, ruleID, key string) runtime.RunFrame {
	proposal := directProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitBoundedCapabilityRequest {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	proposal.CapabilitySelector = selector
	return runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameStart,
		MessageID: key, IdempotencyKey: key,
		Start: &runtime.StartInput{
			RequestID: "request-bounded",
			Project:   runtime.ProjectIdentity{Root: fixture.projectRoot, ConfigurationDigest: fixture.snapshot.Digest()},
			Proposal:  proposal,
			Bounded: &runtime.BoundedInput{
				DeliverableID: "deliverable-review", InputDigest: strings.Repeat("1", 64),
				RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"},
				TerminationCondition: "one normalized review report", ExecutorID: "executor-review", TrustedRuleID: ruleID,
			},
		},
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
