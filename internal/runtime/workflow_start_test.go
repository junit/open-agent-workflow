package runtime_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestWorkflowStartRequiresExplicitProfileSelection(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)

	started, err := engine.Exchange(workflowStartFrame(fixture, "workflow-selection-start"))
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}
	if started.Kind != runtime.ReplySelectionRequired || started.Snapshot.Status != runtime.RunAwaitingSelection || started.Snapshot.RequestMode != classification.RequestModeWorkflow || started.Revision != 1 {
		t.Fatalf("Workflow START reply = %#v", started)
	}
	if started.Snapshot.Workflow == nil || len(started.Snapshot.Workflow.Bundles) != 0 || len(started.Snapshot.LifecycleBundles) != 0 || len(started.Snapshot.GrantIDs) != 0 || len(started.Snapshot.ResourceLeaseIDs) != 0 {
		t.Fatalf("Workflow START created authority before selection: %#v", started.Snapshot)
	}
	if len(started.Diagnostics) != 1 || started.Diagnostics[0].Code != "SELECTION_REQUIRED" {
		t.Fatalf("selection diagnostics = %#v", started.Diagnostics)
	}

	selected, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "profile-selected", IdempotencyKey: "profile-selected",
		RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalProfileSelected, ProfileSelection: &runtime.ProfileSelection{Profile: "MATT-SP-HYBRID", Bindings: []profile.ProfileBinding{}}},
	})
	if err != nil {
		t.Fatalf("Exchange(PROFILE_SELECTED) error = %v", err)
	}
	if selected.Kind != runtime.ReplyModeDecided || selected.Snapshot.Status != runtime.RunReady || selected.Revision != 2 {
		t.Fatalf("selected reply = %#v", selected)
	}
	workflow := selected.Snapshot.Workflow
	if workflow == nil || workflow.ActiveGeneration != 1 || workflow.ActiveNodeID != "requirements" || len(workflow.Bundles) != 1 || len(selected.Snapshot.LifecycleBundles) != 1 {
		t.Fatalf("selected Workflow state = %#v", workflow)
	}
	bundle := workflow.Bundles[0]
	if selected.Snapshot.LifecycleBundles[0] != bundle.ID || bundle.Selection.Profile != "MATT-SP-HYBRID" || bundle.RecipeID != "oaw/reliable-feature" || bundle.RecipeVersion != "1.0.0" || bundle.Configuration.Digest != fixture.snapshot.Digest() || bundle.RegistryDigest != fixture.registry.Digest() || bundle.GraphDigest == "" || len(bundle.ProviderInstances) != 2 || len(bundle.AddOns) != 0 {
		t.Fatalf("Lifecycle Bundle pins = %#v", bundle)
	}
	if bundle.HostIntegrationID != fixture.hostIntegration.ID || bundle.HostIntegrationDigest != fixture.hostIntegration.Digest || bundle.HostManifestDigest != fixture.hostIntegration.ManifestDigest || bundle.HostAuditDigest != fixture.hostIntegration.Audit.Digest || bundle.HostConformanceDigest != fixture.hostIntegration.Conformance.Digest {
		t.Fatalf("Lifecycle Bundle Host pins = %#v", bundle)
	}
}

func TestWorkflowProfileSelectionRejectsInstructionOnlyHost(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, false)
	started, err := engine.Exchange(workflowStartFrame(fixture, "workflow-no-isolation-start"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "profile-no-isolation", IdempotencyKey: "profile-no-isolation",
		RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalProfileSelected, ProfileSelection: &runtime.ProfileSelection{Profile: "MATT-SP-HYBRID", Bindings: []profile.ProfileBinding{}}},
	})
	assertErrorCode(t, err, "HOST_INTEGRATION_NOT_ADMITTED")
	assertRevisionCount(t, stateRoot, started.RunID, 1)
}

func TestWorkflowProfileSelectionRejectsUnavailableHostFeature(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngineWithHostFrame(t, stateRoot, fixture, host.RuntimeFrame{
		IntegrationID: fixture.hostIntegration.ID, UnavailableFeatures: []host.Feature{host.FeaturePause},
	})
	started, err := engine.Exchange(workflowStartFrame(fixture, "workflow-host-feature-start"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: "workflow-host-feature-select", IdempotencyKey: "workflow-host-feature-select",
		RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalProfileSelected, ProfileSelection: &runtime.ProfileSelection{Profile: "MATT-SP-HYBRID"}},
	})
	assertErrorCode(t, err, "HOST_RUNTIME_REQUIREMENTS_UNMET")
	if runtime.ErrorCode(errors.Unwrap(err)) != "HOST_ISOLATION_UNAVAILABLE" {
		t.Fatalf("Host compatibility diagnostic = %v", errors.Unwrap(err))
	}
	assertRevisionCount(t, stateRoot, started.RunID, 1)
}

func TestWorkflowStartIgnoresUnrelatedAmbiguousProvider(t *testing.T) {
	fixture := newWorkflowRuntimeFixtureWithAmbiguousSuperpowers(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)

	started, err := engine.Exchange(workflowStartFrame(fixture, "workflow-unrelated-provider-start"))
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}
	if started.Kind != runtime.ReplySelectionRequired || started.Snapshot.Status != runtime.RunAwaitingSelection || started.Revision != 1 {
		t.Fatalf("Workflow START reply = %#v", started)
	}

	selected, err := exchangeWorkflowSelection(engine, started, "workflow-unrelated-provider-select", runtime.ProfileSelection{Profile: "MATT-FULL"})
	if err != nil {
		t.Fatalf("Exchange(PROFILE_SELECTED) error = %v", err)
	}
	if selected.Kind != runtime.ReplyModeDecided || selected.Snapshot.Status != runtime.RunReady || selected.Revision != 2 {
		t.Fatalf("selected reply = %#v", selected)
	}
}

func TestWorkflowProfileSelectionReportsProviderResolution(t *testing.T) {
	fixture := newWorkflowRuntimeFixtureWithAmbiguousSuperpowers(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	started, err := engine.Exchange(workflowStartFrame(fixture, "workflow-provider-resolution-start"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = exchangeWorkflowSelection(engine, started, "workflow-provider-resolution-select", runtime.ProfileSelection{Profile: "SP-FULL"})
	assertErrorCode(t, err, "PROVIDER_CANDIDATE_AMBIGUOUS")
	if strings.Contains(err.Error(), fixture.home) || strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("Runtime diagnostic leaked discovery paths: %q", err)
	}
	assertRevisionCount(t, stateRoot, started.RunID, 1)
}

func TestWorkflowProfileSelectionReportsBoundProviderResolution(t *testing.T) {
	fixture := newWorkflowRuntimeFixtureWithAmbiguousSuperpowers(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	started, err := engine.Exchange(workflowStartFrame(fixture, "workflow-bound-provider-start"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = exchangeWorkflowSelection(engine, started, "workflow-bound-provider-select", runtime.ProfileSelection{
		Profile: "MATT-FULL",
		Bindings: []profile.ProfileBinding{{
			Selector:            catalog.CapabilitySelector{ProviderID: "oaw/matt", CapabilityID: "implementation"},
			PreferredProviderID: "oaw/superpowers",
		}},
	})
	assertErrorCode(t, err, "PROVIDER_CANDIDATE_AMBIGUOUS")
	assertRevisionCount(t, stateRoot, started.RunID, 1)
}

func TestWorkflowProfileSelectionPreservesVerifiedContractError(t *testing.T) {
	fixture := newWorkflowRuntimeFixture(t)
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine := newWorkflowEngine(t, stateRoot, fixture, true)
	started, err := engine.Exchange(workflowStartFrame(fixture, "workflow-contract-error-start"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = exchangeWorkflowSelection(engine, started, "workflow-contract-error-select", runtime.ProfileSelection{
		Profile: "MATT-FULL",
		Bindings: []profile.ProfileBinding{{
			Selector:            catalog.CapabilitySelector{ProviderID: "oaw/matt", CapabilityID: "implementation"},
			PreferredProviderID: "oaw/superpowers",
		}},
	})
	assertErrorCode(t, err, "PROFILE_SELECTION_INVALID")
	var compileErr *profile.CompileError
	if !errors.As(err, &compileErr) || compileErr.ProviderID != "" || compileErr.CapabilityID != "" {
		t.Fatalf("contract error = %#v", compileErr)
	}
	assertRevisionCount(t, stateRoot, started.RunID, 1)
}

type workflowRuntimeFixture struct {
	projectRoot     string
	home            string
	snapshot        config.Snapshot
	resolutions     registry.ResolutionReport
	registry        registry.Registry
	bindings        []catalog.HostBinding
	hostIntegration host.IntegrationRecord
}

func newWorkflowRuntimeFixture(t *testing.T) workflowRuntimeFixture {
	return newWorkflowRuntimeFixtureWithCandidates(t, false)
}

func newWorkflowRuntimeFixtureWithAmbiguousSuperpowers(t *testing.T) workflowRuntimeFixture {
	return newWorkflowRuntimeFixtureWithCandidates(t, true)
}

func newWorkflowRuntimeFixtureWithCandidates(t *testing.T, ambiguousSuperpowers bool) workflowRuntimeFixture {
	t.Helper()
	projectRoot := t.TempDir()
	snapshot, hostIntegration := hosttest.LoadManagedSnapshot(t, projectRoot)
	home := t.TempDir()
	providers := map[string]string{
		".codex/plugins/superpowers/skills/using-superpowers/SKILL.md": "superpowers",
		".agents/skills/to-spec/SKILL.md":                              "matt-spec",
		".agents/skills/to-tickets/SKILL.md":                           "matt-tickets",
	}
	if ambiguousSuperpowers {
		providers[".claude/plugins/cache/claude-plugins-official/superpowers/6.1.1/skills/using-superpowers/SKILL.md"] = "superpowers-6.1.1"
	}
	for path, content := range providers {
		fullPath := filepath.Join(home, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(fullPath), err)
		}
		writeTestFile(t, fullPath, []byte(content))
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{UserHome: home})
	if err != nil {
		t.Fatalf("discovery.Discover() error = %v", err)
	}
	bindings := make([]catalog.HostBinding, 0)
	for _, provider := range snapshot.Catalog().Providers() {
		if provider.ID != "oaw/superpowers" && provider.ID != "oaw/matt" {
			continue
		}
		for _, capability := range provider.Capabilities {
			bindings = append(bindings, capability.HostBindings...)
		}
	}
	resolutions, effective, err := registry.Resolve(snapshot, evidence, &registry.BindingInventory{Host: "codex", Bindings: bindings})
	if err != nil {
		t.Fatalf("registry.Resolve() error = %v", err)
	}
	return workflowRuntimeFixture{projectRoot: projectRoot, home: home, snapshot: snapshot, resolutions: resolutions, registry: effective, bindings: append([]catalog.HostBinding{}, bindings...), hostIntegration: hostIntegration}
}

func newWorkflowEngine(t *testing.T, stateRoot string, fixture workflowRuntimeFixture, isolated bool) *runtime.Engine {
	t.Helper()
	frame := host.RuntimeFrame{IntegrationID: "oaw/codex-instruction"}
	if isolated {
		frame = host.RuntimeFrame{IntegrationID: fixture.hostIntegration.ID}
	}
	return newWorkflowEngineWithHostFrame(t, stateRoot, fixture, frame)
}

func newWorkflowEngineWithHostFrame(t *testing.T, stateRoot string, fixture workflowRuntimeFixture, frame host.RuntimeFrame) *runtime.Engine {
	t.Helper()
	engine, err := runtime.NewEngine(runtime.Options{
		StateRoot: stateRoot,
		Workflow: runtime.WorkflowOptions{
			Configuration: fixture.snapshot, Resolutions: fixture.resolutions, Registry: fixture.registry,
			Authority: admission.AuthorityCeiling{
				Effects:   []string{"git-local", "read-project", "run-process", "write-project"},
				Resources: []string{"git-repository", "project", "project-worktree"}, ResourceLeases: true, AllowDelegation: true,
			},
			Host: frame,
			Executors: []runtime.WorkflowExecutorRegistration{
				{Registration: admission.ExecutorRegistration{ID: "executor-write", Kind: admission.ExecutorIsolated}},
				{Registration: admission.ExecutorRegistration{ID: "executor-review", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("runtime.NewEngine() error = %v", err)
	}
	return engine
}

func exchangeWorkflowSelection(engine *runtime.Engine, current runtime.RunReply, key string, selection runtime.ProfileSelection) (runtime.RunReply, error) {
	return engine.Exchange(runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameContinue,
		MessageID: key, IdempotencyKey: key,
		RunID: current.RunID, ExpectedRevision: current.Revision,
		Continue: &runtime.ContinueInput{Signal: runtime.SignalProfileSelected, ProfileSelection: &selection},
	})
}

func workflowStartFrame(fixture workflowRuntimeFixture, key string) runtime.RunFrame {
	proposal := directProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitArchitectureDecision || proposal.Traits[index].Trait == classification.TraitDomainUncertainty {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	return runtime.RunFrame{
		SchemaVersion: runtime.RuntimeSchemaV1, Kind: runtime.FrameStart,
		MessageID: key, IdempotencyKey: key,
		Start: &runtime.StartInput{
			RequestID: "request-workflow", Project: runtime.ProjectIdentity{Root: fixture.projectRoot, ConfigurationDigest: fixture.snapshot.Digest()},
			Proposal: proposal, Workflow: &runtime.WorkflowInput{DeliverableID: "deliverable-workflow", InputDigest: strings.Repeat("1", 64)},
		},
	}
}
