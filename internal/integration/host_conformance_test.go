package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

const (
	ticket08Credential = "credential=ticket08-provider-secret"
	ticket08AuditRaw   = "ticket08 raw official audit text"
	ticket08AdapterRaw = "ticket08 raw adapter output"
)

func TestTicket08ConformanceToRestartPinsRunnerAndNativeHosts(t *testing.T) {
	for _, level := range []host.IntegrationLevel{host.RunnerManaged, host.NativeManaged} {
		t.Run(string(level), func(t *testing.T) {
			adapter := &ticket08Adapter{native: level == host.NativeManaged}
			integration := ticket08ConformingIntegration(t, "acme/codex-"+string(level), "1.0.0", level, "codex", []string{"agent", "skill", "tool"}, adapter)
			wantCalls := 4 + 2*len(integration.Manifest.BindingKinds)
			if adapter.calls != wantCalls {
				t.Fatalf("conformance Adapter calls = %d, want %d", adapter.calls, wantCalls)
			}
			projectRoot := t.TempDir()
			fixture := ticket08RuntimeFixture(t, projectRoot, integration)
			stateRoot := filepath.Join(t.TempDir(), "state")
			projectionRoot := ticket07PhysicalDirectory(t, filepath.Join(t.TempDir(), "projection"))
			engine := ticket08Engine(t, stateRoot, fixture, host.RuntimeFrame{HostID: integration.Manifest.HostID, IntegrationID: integration.ID}, oawruntime.ProjectionOptions{Root: projectionRoot})
			selected := startAndSelectTicket07Workflow(t, engine, fixture, "ticket08-"+string(level))
			granted, err := requestTicket07WriteStage(engine, selected, "ticket08-grant")
			if err != nil {
				t.Fatal(err)
			}
			grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
			prepared := ticket08Dispatch(t, engine, granted, grant, "ticket08-dispatch")
			observed := ticket08Observe(t, engine, prepared, grant, "ticket08-observe", "")
			restarted := ticket08Engine(t, stateRoot, fixture, host.RuntimeFrame{HostID: integration.Manifest.HostID, IntegrationID: integration.ID}, oawruntime.ProjectionOptions{Root: projectionRoot})
			inspected, err := restarted.Exchange(oawruntime.RunFrame{
				SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameInspect,
				MessageID: "ticket08-inspect", IdempotencyKey: "ticket08-inspect", RunID: observed.RunID,
			})
			if err != nil || !reflect.DeepEqual(inspected.Snapshot, observed.Snapshot) {
				t.Fatalf("restarted INSPECT = %#v, %v", inspected, err)
			}
			bundle := inspected.Snapshot.Workflow.Bundles[0]
			if bundle.HostIntegrationID != integration.ID || bundle.HostIntegrationDigest != integration.Digest || bundle.HostManifestDigest != integration.ManifestDigest || bundle.HostAuditDigest != integration.Audit.Digest || bundle.HostConformanceDigest != integration.Conformance.Digest {
				t.Fatalf("Bundle Host pins = %#v", bundle)
			}
			if adapter.calls != wantCalls {
				t.Fatalf("Runtime invoked the conformance Adapter: calls=%d", adapter.calls)
			}
			assertTicket08Redacted(t, integration.Conformance, inspected, stateRoot, projectionRoot)
			assertTicket08OwnerOnly(t, stateRoot, projectionRoot)
		})
	}
}

func TestTicket08WorkflowAdmissionFailuresLeaveHeadUnchanged(t *testing.T) {
	for _, test := range []struct {
		name        string
		hostID      string
		kinds       []string
		unavailable []host.Feature
		code        string
	}{
		{name: "wrong Binding Host", hostID: "claude", kinds: []string{"agent", "skill", "tool"}, code: "HOST_BINDING_UNSUPPORTED"},
		{name: "wrong Binding kind", hostID: "codex", kinds: []string{"tool"}, code: "HOST_BINDING_UNSUPPORTED"},
		{name: "per-run narrowing", hostID: "codex", kinds: []string{"agent", "skill", "tool"}, unavailable: []host.Feature{host.FeaturePause}, code: "HOST_RUNTIME_REQUIREMENTS_UNMET"},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter := &ticket08Adapter{}
			integration := ticket08ConformingIntegration(t, "acme/denied-host", "1.0.0", host.RunnerManaged, test.hostID, test.kinds, adapter)
			fixture := ticket08RuntimeFixture(t, t.TempDir(), integration)
			stateRoot := filepath.Join(t.TempDir(), "state")
			engine := ticket08Engine(t, stateRoot, fixture, host.RuntimeFrame{HostID: integration.Manifest.HostID, IntegrationID: integration.ID, UnavailableFeatures: test.unavailable}, oawruntime.ProjectionOptions{})
			started := ticket08StartWorkflow(t, engine, fixture, "ticket08-denied")
			headBefore := ticket08Head(t, stateRoot, started.RunID)
			_, err := engine.Exchange(oawruntime.RunFrame{
				SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
				MessageID: "ticket08-denied-select", IdempotencyKey: "ticket08-denied-select",
				RunID: started.RunID, ExpectedRevision: started.Revision,
				Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalProfileSelected, ProfileSelection: &oawruntime.ProfileSelection{Profile: "MATT-SP-HYBRID"}},
			})
			if oawruntime.ErrorCode(err) != test.code {
				t.Fatalf("selection error = %v, want %s", err, test.code)
			}
			if headAfter := ticket08Head(t, stateRoot, started.RunID); headAfter != headBefore {
				t.Fatalf("denied selection changed HEAD: %s != %s", headAfter, headBefore)
			}
		})
	}
}

func TestTicket08StableSwitchAdoptsCurrentHostAndRejectsStaleEngine(t *testing.T) {
	firstAdapter, secondAdapter := &ticket08Adapter{}, &ticket08Adapter{}
	first := ticket08ConformingIntegration(t, "acme/codex-generation", "1.0.0", host.RunnerManaged, "codex", []string{"agent", "skill", "tool"}, firstAdapter)
	second := ticket08ConformingIntegration(t, first.ID, "1.0.1", host.RunnerManaged, "codex", []string{"agent", "skill", "tool"}, secondAdapter)
	projectRoot := t.TempDir()
	firstFixture := ticket08RuntimeFixture(t, projectRoot, first)
	secondFixture := ticket08RuntimeFixture(t, projectRoot, second)
	stateRoot := filepath.Join(t.TempDir(), "state")
	firstEngine := ticket08Engine(t, stateRoot, firstFixture, host.RuntimeFrame{HostID: first.Manifest.HostID, IntegrationID: first.ID}, oawruntime.ProjectionOptions{})
	selected := startAndSelectTicket07Workflow(t, firstEngine, firstFixture, "ticket08-generation")
	granted, err := requestTicket07WriteStage(firstEngine, selected, "ticket08-generation-grant")
	if err != nil {
		t.Fatal(err)
	}
	grant := granted.Snapshot.Grants[len(granted.Snapshot.Grants)-1]
	prepared := ticket08Dispatch(t, firstEngine, granted, grant, "ticket08-generation-dispatch")
	observed := ticket08Observe(t, firstEngine, prepared, grant, "ticket08-generation-observe", "specification-approved")

	secondEngine := ticket08Engine(t, stateRoot, secondFixture, host.RuntimeFrame{HostID: second.Manifest.HostID, IntegrationID: second.ID}, oawruntime.ProjectionOptions{})
	headBefore := ticket08Head(t, stateRoot, observed.RunID)
	_, err = secondEngine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameInspect,
		MessageID: "ticket08-generation-new-inspect", IdempotencyKey: "ticket08-generation-new-inspect", RunID: observed.RunID,
	})
	if oawruntime.ErrorCode(err) != "HOST_INTEGRATION_CHANGED" || ticket08Head(t, stateRoot, observed.RunID) != headBefore {
		t.Fatalf("changed Host INSPECT = %v", err)
	}
	switched, err := secondEngine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: "ticket08-generation-switch", IdempotencyKey: "ticket08-generation-switch",
		RunID: observed.RunID, ExpectedRevision: observed.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalSwitchProfile, StableBoundarySwitch: &oawruntime.StableBoundarySwitch{
			Boundary: "specification-approved", Selection: oawruntime.ProfileSelection{Profile: "SP-FULL"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(switched.Snapshot.Workflow.Bundles) != 2 || !reflect.DeepEqual(switched.Snapshot.Workflow.Bundles[0], observed.Snapshot.Workflow.Bundles[0]) || switched.Snapshot.Workflow.Bundles[1].HostIntegrationDigest != second.Digest {
		t.Fatalf("Host generation switch = %#v", switched.Snapshot.Workflow.Bundles)
	}
	headBefore = ticket08Head(t, stateRoot, switched.RunID)
	_, err = firstEngine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameInspect,
		MessageID: "ticket08-generation-stale-inspect", IdempotencyKey: "ticket08-generation-stale-inspect", RunID: switched.RunID,
	})
	if oawruntime.ErrorCode(err) != "HOST_INTEGRATION_CHANGED" || ticket08Head(t, stateRoot, switched.RunID) != headBefore {
		t.Fatalf("stale Host INSPECT = %v", err)
	}
	wantFirst := 4 + 2*len(first.Manifest.BindingKinds)
	wantSecond := 4 + 2*len(second.Manifest.BindingKinds)
	if firstAdapter.calls != wantFirst || secondAdapter.calls != wantSecond {
		t.Fatalf("Runtime invoked Adapters: first=%d second=%d", firstAdapter.calls, secondAdapter.calls)
	}
}

func TestTicket08ConfigurationRejectsInvalidOrProjectGrantedHostTrust(t *testing.T) {
	valid := ticket08ConformingIntegration(t, "acme/config-host", "1.0.0", host.RunnerManaged, "codex", []string{"agent", "skill", "tool"}, &ticket08Adapter{})
	invalid := map[string]func(host.IntegrationRecord) host.IntegrationRecord{
		"missing Feature": func(value host.IntegrationRecord) host.IntegrationRecord {
			value.Manifest.Features = value.Manifest.Features[1:]
			return value
		},
		"failed audit": func(value host.IntegrationRecord) host.IntegrationRecord {
			value.Audit.Status = host.AuditPending
			return value
		},
		"missing Report": func(value host.IntegrationRecord) host.IntegrationRecord { value.Conformance = nil; return value },
		"stale Report": func(value host.IntegrationRecord) host.IntegrationRecord {
			value.Conformance.SuiteVersion = "oaw.host-conformance/v0"
			return value
		},
		"failed Report": func(value host.IntegrationRecord) host.IntegrationRecord {
			value.Conformance.Passed = false
			return value
		},
	}
	for name, mutate := range invalid {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			ticket08WriteUserIntegration(t, root, mutate(host.CloneIntegration(valid)))
			_, err := config.Load(config.LoadOptions{UserConfigRoot: root, ProjectRoot: t.TempDir()})
			if err == nil || strings.Contains(err.Error(), ticket08AuditRaw) || strings.Contains(err.Error(), ticket08AdapterRaw) {
				t.Fatalf("config.Load() error = %v", err)
			}
		})
	}
	projectRoot := t.TempDir()
	writeTicket07File(t, filepath.Join(projectRoot, ".oaw", "config.toml"), "schema_version = \"oaw.project-config/v1\"\nhost_integrations = []\n")
	if _, err := config.Load(config.LoadOptions{ProjectRoot: projectRoot}); err == nil {
		t.Fatal("project configuration granted Host trust")
	}
}

type ticket08Adapter struct {
	native bool
	calls  int
}

func (adapter *ticket08Adapter) CreateExecutor(request host.ExecutorFixtureRequest) (host.ExecutorFixtureReceipt, error) {
	adapter.calls++
	return host.ExecutorFixtureReceipt{ExecutorID: request.ExecutorID, Isolated: true, BundleDigest: request.BundleDigest}, nil
}

func (adapter *ticket08Adapter) Invoke(request host.InvocationFixtureRequest) (host.ObservationFixtureReceipt, error) {
	adapter.calls++
	return host.ObservationFixtureReceipt{
		InvocationID: request.InvocationID, ExecutionID: "ticket08-execution", Binding: request.Binding,
		BundleDigest: request.BundleDigest, Outcome: host.FixtureSucceeded,
		Evidence: []host.NormalizedEvidence{{Reference: "evidence://host-conformance", Digest: request.EvidenceChallengeDigest}},
		Native:   adapter.native, RawOutput: ticket08AdapterRaw,
	}, nil
}

func (adapter *ticket08Adapter) ObserveProviderBindings(request host.BindingInventoryFixtureRequest) (host.BindingInventory, error) {
	adapter.calls++
	return host.NewBindingInventory(request.HostID, []host.BindingObservation{{
		HostID: request.HostID, InstallationKey: request.InstallationKey, Binding: request.Binding,
		Source: "native-probe", EvidenceReference: "evidence://host-conformance/provider-binding", Digest: request.EvidenceChallengeDigest,
	}})
}

func (adapter *ticket08Adapter) Pause(request host.PauseFixtureRequest) (host.PauseFixtureReceipt, error) {
	adapter.calls++
	return host.PauseFixtureReceipt{RunID: request.RunID, Paused: true}, nil
}

func (adapter *ticket08Adapter) Cancel(request host.CancelFixtureRequest) (host.CancelFixtureReceipt, error) {
	adapter.calls++
	return host.CancelFixtureReceipt{InvocationID: request.InvocationID, Cancelled: true}, nil
}

func ticket08ConformingIntegration(t *testing.T, id, version string, level host.IntegrationLevel, hostID string, kinds []string, adapter *ticket08Adapter) host.IntegrationRecord {
	t.Helper()
	features := []host.Feature{host.FeatureBundleInheritance, host.FeatureCancellation, host.FeatureEvidenceReturn, host.FeatureExactBindingInvocation, host.FeatureInvocationDedup, host.FeatureIsolatedExecutor, host.FeatureNormalizedObservation, host.FeaturePause, host.FeatureProviderBindingInventory}
	if level == host.NativeManaged {
		features = append(features, host.FeatureNativeInvocation)
	}
	manifest, err := host.NewManifest(host.Manifest{SchemaVersion: host.HostManifestSchemaV1, ManifestVersion: "1.0.0", HostID: hostID, IntegrationLevel: level, Protocols: []string{host.RuntimeProtocolV1}, BindingKinds: kinds, Features: features})
	if err != nil {
		t.Fatal(err)
	}
	report, err := host.RunConformance(id, manifest, adapter)
	if err != nil || !report.Passed {
		t.Fatalf("RunConformance() = %#v, %v", report, err)
	}
	auditDigest := sha256.Sum256([]byte(ticket08AuditRaw + version))
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPassed, References: []host.AuditEvidenceReference{{Reference: "audit://official/" + version, Digest: hex.EncodeToString(auditDigest[:])}}})
	if err != nil {
		t.Fatal(err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{SchemaVersion: host.HostIntegrationSchemaV1, IntegrationVersion: version, ID: id, Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit, Conformance: &report})
	if err != nil {
		t.Fatal(err)
	}
	return integration
}

func ticket08RuntimeFixture(t *testing.T, projectRoot string, integration host.IntegrationRecord) ticket07IntegrationFixture {
	t.Helper()
	userRoot := t.TempDir()
	ticket08WriteUserIntegration(t, userRoot, integration)
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	for relative, name := range map[string]string{".codex/plugins/superpowers/skills/using-superpowers/SKILL.md": "using-superpowers", ".agents/skills/to-spec/SKILL.md": "to-spec", ".agents/skills/to-tickets/SKILL.md": "to-tickets"} {
		writeTicket07File(t, filepath.Join(home, filepath.FromSlash(relative)), "---\nname: "+name+"\n---\n"+ticket08Credential)
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := hosttest.ObserveProviderBindings(t, snapshot.Catalog(), evidence, home, "oaw/superpowers", "oaw/matt")
	_, effective, err := registry.Resolve(snapshot, "codex", evidence, &inventory)
	if err != nil {
		t.Fatal(err)
	}
	return ticket07IntegrationFixture{projectRoot: projectRoot, snapshot: snapshot, registry: effective, hostIntegration: integration, hostInvocationMarker: filepath.Join(t.TempDir(), "runtime-invoked-host")}
}

func ticket08WriteUserIntegration(t *testing.T, root string, integration host.IntegrationRecord) {
	t.Helper()
	var raw bytes.Buffer
	if err := toml.NewEncoder(&raw).Encode(integration); err != nil {
		t.Fatal(err)
	}
	writeTicket07File(t, filepath.Join(root, "integrations", "host.toml"), raw.String())
	writeTicket07File(t, filepath.Join(root, "config.toml"), "schema_version = \"oaw.user-config/v2\"\n[[host_integrations]]\nid = \""+integration.ID+"\"\npath = \"integrations/host.toml\"\nreplace = false\n")
}

func ticket08Engine(t *testing.T, stateRoot string, fixture ticket07IntegrationFixture, frame host.RuntimeFrame, projection oawruntime.ProjectionOptions) *oawruntime.Engine {
	t.Helper()
	engine, err := oawruntime.NewEngine(oawruntime.Options{StateRoot: stateRoot, Workflow: oawruntime.WorkflowOptions{
		Configuration: fixture.snapshot, Registry: fixture.registry, Host: frame, Projection: projection,
		Authority: admission.AuthorityCeiling{Effects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project", "project-worktree"}, ResourceLeases: true, AllowDelegation: true},
		Executors: []oawruntime.WorkflowExecutorRegistration{{Registration: admission.ExecutorRegistration{ID: "executor-write", Kind: admission.ExecutorIsolated}}, {Registration: admission.ExecutorRegistration{ID: "executor-review", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func ticket08StartWorkflow(t *testing.T, engine *oawruntime.Engine, fixture ticket07IntegrationFixture, key string) oawruntime.RunReply {
	t.Helper()
	proposal := integrationDirectProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitArchitectureDecision || proposal.Traits[index].Trait == classification.TraitDomainUncertainty {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	reply, err := engine.Exchange(oawruntime.RunFrame{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameStart, MessageID: key, IdempotencyKey: key, Start: &oawruntime.StartInput{RequestID: key, Project: oawruntime.ProjectIdentity{Root: fixture.projectRoot, ConfigurationDigest: fixture.snapshot.Digest()}, Proposal: &proposal, Workflow: &oawruntime.WorkflowInput{DeliverableID: key, InputDigest: strings.Repeat("d", 64)}}})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func ticket08Dispatch(t *testing.T, engine *oawruntime.Engine, current oawruntime.RunReply, grant admission.CapabilityGrant, key string) oawruntime.RunReply {
	t.Helper()
	reply, err := engine.Exchange(oawruntime.RunFrame{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue, MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision, Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalDispatchPrepared, DispatchPreparation: &oawruntime.DispatchPreparation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func ticket08Observe(t *testing.T, engine *oawruntime.Engine, current oawruntime.RunReply, grant admission.CapabilityGrant, key, boundary string) oawruntime.RunReply {
	t.Helper()
	reply, err := engine.Exchange(oawruntime.RunFrame{SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue, MessageID: key, IdempotencyKey: key, RunID: current.RunID, ExpectedRevision: current.Revision, Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalCapabilityObserved, StageObservation: &oawruntime.StageObservation{CapabilityObservation: oawruntime.CapabilityObservation{GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID, Outcome: oawruntime.ObservationSucceeded, EvidenceReferences: []oawruntime.EvidenceReference{{Reference: "evidence://ticket08", Digest: strings.Repeat("e", 64)}}}, Signal: "succeeded", StableBoundary: boundary}}})
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func ticket08Head(t *testing.T, stateRoot, runID string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(stateRoot, "runs", runID, "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertTicket08Redacted(t *testing.T, report *host.ConformanceReport, reply oawruntime.RunReply, roots ...string) {
	t.Helper()
	values := []any{report, reply}
	for _, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		assertTicket08BytesRedacted(t, raw)
	}
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, readErr := os.ReadFile(path)
			if readErr == nil {
				assertTicket08BytesRedacted(t, raw)
			}
			return readErr
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func assertTicket08BytesRedacted(t *testing.T, raw []byte) {
	t.Helper()
	for _, forbidden := range []string{ticket08Credential, ticket08AuditRaw, ticket08AdapterRaw} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("sensitive fixture input leaked: %q", forbidden)
		}
	}
}

func assertTicket08OwnerOnly(t *testing.T, roots ...string) {
	t.Helper()
	if os.PathSeparator == '\\' {
		return
	}
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			want := os.FileMode(0o600)
			if entry.IsDir() {
				want = 0o700
			}
			if info.Mode().Perm() != want {
				t.Fatalf("mode(%s) = %o, want %o", path, info.Mode().Perm(), want)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
