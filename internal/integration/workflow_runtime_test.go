package integration

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

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
	integrationHostCredential = "credential=ticket07-host-secret"
	integrationRawOutput      = "ticket07-raw-provider-output"
	integrationSinkCredential = "credential=ticket07-projection-secret"
)

func TestTicket07ReliableFeaturePinsECCAddOnsOnlyWhenVerified(t *testing.T) {
	tests := []struct {
		name          string
		installECC    bool
		wantAddOns    []string
		wantProviders int
	}{
		{name: "ECC absent", installECC: false, wantAddOns: []string{}, wantProviders: 2},
		{name: "ECC verified", installECC: true, wantAddOns: []string{"build-repair", "dependency-repair", "type-repair"}, wantProviders: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTicket07IntegrationFixture(t, test.installECC)
			engine := newTicket07Engine(t, filepath.Join(t.TempDir(), "state"), fixture, oawruntime.ProjectionOptions{})
			selected := startAndSelectTicket07Workflow(t, engine, fixture, "ecc-add-ons")
			bundle := selected.Snapshot.Workflow.Bundles[0]
			if !reflect.DeepEqual(bundle.AddOns, test.wantAddOns) || len(bundle.ProviderInstances) != test.wantProviders {
				t.Fatalf("Bundle add-ons/providers = %#v/%#v", bundle.AddOns, bundle.ProviderInstances)
			}
			_, eccPinned := ticket07ProviderPinned(bundle, "oaw/ecc")
			if eccPinned != test.installECC {
				t.Fatalf("ECC pinned = %t, want %t", eccPinned, test.installECC)
			}
		})
	}
}

func TestTicket07TwoEnginesShareAuthorityWithoutInvokingHostBindings(t *testing.T) {
	fixture := newTicket07IntegrationFixture(t, true)
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectionRoot := ticket07PhysicalDirectory(t, filepath.Join(t.TempDir(), "projections"))
	engines := []*oawruntime.Engine{
		newTicket07Engine(t, stateRoot, fixture, oawruntime.ProjectionOptions{Root: projectionRoot}),
		newTicket07Engine(t, stateRoot, fixture, oawruntime.ProjectionOptions{Sink: ticket07FailingSink{}}),
	}
	ready := []oawruntime.RunReply{
		startAndSelectTicket07Workflow(t, engines[0], fixture, "shared-engine-a"),
		startAndSelectTicket07Workflow(t, engines[1], fixture, "shared-engine-b"),
	}

	replies := make([]oawruntime.RunReply, len(engines))
	errs := make([]error, len(engines))
	var wait sync.WaitGroup
	wait.Add(len(engines))
	for index := range engines {
		go func(index int) {
			defer wait.Done()
			replies[index], errs[index] = requestTicket07WriteStage(engines[index], ready[index], "shared-grant")
		}(index)
	}
	wait.Wait()

	winner := -1
	conflicts := 0
	for index, err := range errs {
		switch {
		case err == nil:
			winner = index
		case oawruntime.ErrorCode(err) == "RESOURCE_LEASE_CONFLICT":
			conflicts++
		default:
			t.Fatalf("Engine %d stage request error = %v", index, err)
		}
	}
	if winner < 0 || conflicts != 1 {
		t.Fatalf("shared lease results: winner=%d conflicts=%d errors=%v", winner, conflicts, errs)
	}

	grant := replies[winner].Snapshot.Grants[len(replies[winner].Snapshot.Grants)-1]
	prepared, err := engines[winner].Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: "shared-dispatch", IdempotencyKey: "shared-dispatch", RunID: replies[winner].RunID, ExpectedRevision: replies[winner].Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalDispatchPrepared, DispatchPreparation: &oawruntime.DispatchPreparation{
			GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = engines[winner].Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: "shared-raw-output", IdempotencyKey: "shared-raw-output", RunID: prepared.RunID, ExpectedRevision: prepared.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalCapabilityObserved, StageObservation: &oawruntime.StageObservation{
			CapabilityObservation: oawruntime.CapabilityObservation{
				GrantID: grant.ID, InvocationID: grant.InvocationID, ExecutorID: grant.Executor.ID,
				Outcome: oawruntime.ObservationSucceeded, EvidenceReferences: []oawruntime.EvidenceReference{{Reference: "evidence://ticket07", Digest: strings.Repeat("e", 64)}}, RawOutput: integrationRawOutput,
			},
			Signal: "succeeded",
		}},
	})
	if oawruntime.ErrorCode(err) != "OBSERVATION_INVALID" {
		t.Fatalf("raw output error = %v", err)
	}

	observer := engines[(winner+1)%len(engines)]
	inspected, err := observer.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameInspect,
		MessageID: "shared-inspect", IdempotencyKey: "shared-inspect", RunID: prepared.RunID,
	})
	if err != nil || inspected.RevisionDigest != prepared.RevisionDigest || !reflect.DeepEqual(inspected.Snapshot, prepared.Snapshot) {
		t.Fatalf("cross-Engine INSPECT = %#v, %v", inspected, err)
	}
	if len(inspected.Snapshot.Workflow.ProjectionLag) != 0 {
		t.Fatalf("projection failure entered authoritative state: %#v", inspected.Snapshot.Workflow.ProjectionLag)
	}
	if _, err := os.Stat(fixture.hostInvocationMarker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Runtime invoked a Host Binding: %v", err)
	}

	assertTicket07Paths(t, stateRoot, projectionRoot, ready[0].RunID, ready[1].RunID)
	assertTicket07TreeRedacted(t, stateRoot, projectionRoot)
}

type ticket07IntegrationFixture struct {
	projectRoot          string
	snapshot             config.Snapshot
	registry             registry.Registry
	hostIntegration      host.IntegrationRecord
	hostInvocationMarker string
}

func newTicket07IntegrationFixture(t *testing.T, installECC bool) ticket07IntegrationFixture {
	t.Helper()
	projectRoot := t.TempDir()
	snapshot, hostIntegration := hosttest.LoadManagedSnapshot(t, projectRoot)
	home := t.TempDir()
	marker := filepath.Join(t.TempDir(), "host-binding-invoked")
	evidence := "#!/bin/sh\ntouch \"" + marker + "\"\nexit 97\n# " + integrationHostCredential + "\n"
	paths := map[string]string{
		".codex/plugins/superpowers/skills/using-superpowers/SKILL.md": "using-superpowers",
		".agents/skills/to-spec/SKILL.md":                              "to-spec",
		".agents/skills/to-tickets/SKILL.md":                           "to-tickets",
	}
	if installECC {
		paths[".agents/skills/everything-claude-code/SKILL.md"] = "everything-claude-code"
	}
	for relative, name := range paths {
		writeTicket07File(t, filepath.Join(home, filepath.FromSlash(relative)), "---\nname: "+name+"\n---\n"+evidence)
	}
	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	providerIDs := []string{"oaw/superpowers", "oaw/matt"}
	if installECC {
		providerIDs = append(providerIDs, "oaw/ecc")
	}
	inventory := hosttest.ObserveProviderBindings(t, snapshot.Catalog(), discovered, home, providerIDs...)
	report, effective, err := registry.Resolve(snapshot, "codex", discovered, &inventory)
	if err != nil {
		t.Fatal(err)
	}
	for _, providerID := range []string{"oaw/superpowers", "oaw/matt"} {
		resolution, found := report.Resolution(providerID)
		if !found || resolution.State != registry.Verified {
			t.Fatalf("%s resolution = %#v", providerID, resolution)
		}
	}
	ecc, found := report.Resolution("oaw/ecc")
	if !found || installECC && ecc.State != registry.Verified || !installECC && ecc.State != registry.NotFound {
		t.Fatalf("ECC resolution = %#v", ecc)
	}
	return ticket07IntegrationFixture{projectRoot: projectRoot, snapshot: snapshot, registry: effective, hostIntegration: hostIntegration, hostInvocationMarker: marker}
}

func newTicket07Engine(t *testing.T, stateRoot string, fixture ticket07IntegrationFixture, projection oawruntime.ProjectionOptions) *oawruntime.Engine {
	t.Helper()
	engine, err := oawruntime.NewEngine(oawruntime.Options{
		StateRoot: stateRoot,
		Workflow: oawruntime.WorkflowOptions{
			Configuration: fixture.snapshot, Registry: fixture.registry,
			Authority: admission.AuthorityCeiling{
				Effects:   []string{"git-local", "read-project", "run-process", "write-project"},
				Resources: []string{"git-repository", "project", "project-worktree"}, ResourceLeases: true, AllowDelegation: true,
			},
			Host: host.RuntimeFrame{HostID: "codex", IntegrationID: fixture.hostIntegration.ID},
			Executors: []oawruntime.WorkflowExecutorRegistration{
				{Registration: admission.ExecutorRegistration{ID: "executor-write", Kind: admission.ExecutorIsolated}},
				{Registration: admission.ExecutorRegistration{ID: "executor-review", Kind: admission.ExecutorIsolated}, ReadOnly: true, Fresh: true},
			},
			Projection: projection,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func startAndSelectTicket07Workflow(t *testing.T, engine *oawruntime.Engine, fixture ticket07IntegrationFixture, key string) oawruntime.RunReply {
	t.Helper()
	proposal := integrationDirectProposal()
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == classification.TraitArchitectureDecision || proposal.Traits[index].Trait == classification.TraitDomainUncertainty {
			proposal.Traits[index].Value = classification.TraitTrue
		}
	}
	started, err := engine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameStart,
		MessageID: key + "-start", IdempotencyKey: key + "-start",
		Start: &oawruntime.StartInput{
			RequestID: key + "-request", Project: oawruntime.ProjectIdentity{Root: fixture.projectRoot, ConfigurationDigest: fixture.snapshot.Digest()},
			Proposal: &proposal, Workflow: &oawruntime.WorkflowInput{DeliverableID: key + "-deliverable", InputDigest: strings.Repeat("d", 64)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := engine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: key + "-select", IdempotencyKey: key + "-select", RunID: started.RunID, ExpectedRevision: started.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalProfileSelected, ProfileSelection: &oawruntime.ProfileSelection{Profile: "MATT-SP-HYBRID"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return selected
}

func requestTicket07WriteStage(engine *oawruntime.Engine, ready oawruntime.RunReply, key string) (oawruntime.RunReply, error) {
	return engine.Exchange(oawruntime.RunFrame{
		SchemaVersion: oawruntime.RuntimeSchemaV1, Kind: oawruntime.FrameContinue,
		MessageID: key + "-" + ready.RunID, IdempotencyKey: key + "-" + ready.RunID, RunID: ready.RunID, ExpectedRevision: ready.Revision,
		Continue: &oawruntime.ContinueInput{Signal: oawruntime.SignalRequestStageGrant, StageGrant: &oawruntime.StageGrantRequest{
			ExecutorID: "executor-write", RequestedEffects: []string{"write-project"}, RequestedResources: []string{"project-worktree"}, TerminationCondition: "ticket 07 integration stage complete",
		}},
	})
}

func ticket07ProviderPinned(bundle oawruntime.LifecycleBundle, providerID string) (string, bool) {
	for _, provider := range bundle.ProviderInstances {
		if provider.ProviderID == providerID {
			return provider.InstanceDigest, true
		}
	}
	return "", false
}

func assertTicket07Paths(t *testing.T, stateRoot, projectionRoot, projectedRunID, laggedRunID string) {
	t.Helper()
	paths := []string{
		filepath.Join(stateRoot, "resource-leases", "LOCK"),
		filepath.Join(stateRoot, "projection-lag", laggedRunID, "00000000000000000001.json"),
		filepath.Join(projectionRoot, projectedRunID, "workflow.json"),
		filepath.Join(projectionRoot, projectedRunID, "workflow.md"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("required Ticket 07 path %s: %v", path, err)
		}
	}
	if goruntime.GOOS == "windows" {
		return
	}
	for _, root := range []string{stateRoot, projectionRoot} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			want := os.FileMode(0o600)
			if entry.IsDir() {
				want = 0o700
			}
			if info.Mode().Perm() != want {
				t.Fatalf("mode(%s) = %o, want %o", path, info.Mode().Perm(), want)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func assertTicket07TreeRedacted(t *testing.T, roots ...string) {
	t.Helper()
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{integrationHostCredential, integrationRawOutput, integrationSinkCredential} {
				if strings.Contains(string(raw), forbidden) {
					t.Fatalf("%s contains sensitive value %q", path, forbidden)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func writeTicket07File(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func ticket07PhysicalDirectory(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(physical)
}

type ticket07FailingSink struct{}

func (ticket07FailingSink) WriteProjection(oawruntime.WorkflowProjection) error {
	return errors.New(integrationSinkCredential)
}
