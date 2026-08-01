package runtime_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
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

type boundedRuntimeFixture struct {
	projectRoot string
	snapshot    config.Snapshot
	registry    registry.Registry
}

func newBoundedRuntimeFixture(t *testing.T, withDefault bool) boundedRuntimeFixture {
	t.Helper()
	projectRoot := t.TempDir()
	userRoot := t.TempDir()
	userConfig := "schema_version = \"oaw.user-config/v1\"\n"
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
	writeTestFile(t, eccPath, []byte("ecc"))
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{UserHome: home})
	if err != nil {
		t.Fatalf("discovery.Discover() error = %v", err)
	}
	inventory := &registry.BindingInventory{Host: "codex", Bindings: []catalog.HostBinding{
		{Host: "codex", Kind: "agent", Reference: "planner"},
		{Host: "codex", Kind: "agent", Reference: "code-reviewer"},
		{Host: "codex", Kind: "agent", Reference: "security-reviewer"},
	}}
	_, effective, err := registry.Resolve(snapshot, evidence, inventory)
	if err != nil {
		t.Fatalf("registry.Resolve() error = %v", err)
	}
	return boundedRuntimeFixture{projectRoot: projectRoot, snapshot: snapshot, registry: effective}
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
			Registry:      fixture.registry,
		},
	}
}

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
