package policyrun

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

func TestStorePersistsReducerStateAndSerializesMutations(t *testing.T) {
	root := filepath.Join(t.TempDir(), "policy")
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	run := testRun(t)
	if _, err := store.Start(run); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errorsSeen := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := store.Mutate(run.ID, func(current Run) (Run, error) {
				module := policyflow.New()
				progress, err := module.Restore(current.Inventory, current.State)
				if err != nil {
					return Run{}, err
				}
				next, ok := progress.Next.(policyflow.InvokeSkill)
				if !ok {
					return Run{}, errors.New("stale reference")
				}
				progress, err = module.Apply(current.Inventory, policyflow.SkillCompleted{WorkRef: next.WorkRef})
				if err != nil {
					return Run{}, err
				}
				current.State, err = module.Export(progress)
				return current, err
			})
			errorsSeen <- err
		}()
	}
	close(start)
	group.Wait()
	close(errorsSeen)
	succeeded := 0
	for err := range errorsSeen {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful mutations = %d", succeeded)
	}
}

func TestStoreRejectsLegacyStateAsContextUncertain(t *testing.T) {
	root := filepath.Join(t.TempDir(), "policy")
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"id":"run-1","plan":{"schema_version":"oaw.policy-workflow-plan/v1"}}`)
	if err := os.WriteFile(filepath.Join(root, "run-1.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load("run-1")
	if err == nil || !strings.Contains(err.Error(), "POLICY_ONLY_CONTEXT_UNCERTAIN") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreRejectsPreviousEngagementSchemaWithoutMigration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "policy")
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"schema_version":"oaw.policy-engagement/v4","id":"run-1"}`)
	if err := os.WriteFile(filepath.Join(root, "run-1.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load("run-1")
	if err == nil || !strings.Contains(err.Error(), "POLICY_ONLY_CONTEXT_UNCERTAIN") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreRejectsPreviousReducerSchemaWithoutMigration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "policy")
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	run := testRun(t)
	run.State.SchemaVersion = "oaw.policy-flow-state/v3"
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, run.ID+".json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Load(run.ID)
	if err == nil || !strings.Contains(err.Error(), "POLICY_RUN_INVALID") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreRejectsWhitespaceFormattedPreviousSchemaAsContextUncertain(t *testing.T) {
	root := filepath.Join(t.TempDir(), "policy")
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte("{\n  \"schema_version\" : \"oaw.policy-engagement/v5\",\n  \"id\" : \"run-1\"\n}\n")
	if err := os.WriteFile(filepath.Join(root, "run-1.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load("run-1")
	if err == nil || !strings.Contains(err.Error(), "POLICY_ONLY_CONTEXT_UNCERTAIN") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreRejectsSnapshotThatCannotRestoreReducerState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "policy")
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	run := testRun(t)
	run.State.StepIndex++
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, run.ID+".json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Load(run.ID)
	if err == nil || !strings.Contains(err.Error(), "POLICY_RUN_INVALID") || !strings.Contains(err.Error(), "saved Policy state does not match replayed lifecycle history") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreRejectsResponsibilityMapThatDoesNotMatchReducerState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "policy")
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	run := testRun(t)
	run.Plan.ResponsibilityMap["problem-framing"][0].Route = "forged-route"
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, run.ID+".json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = store.Load(run.ID)
	if err == nil || !strings.Contains(err.Error(), "Policy responsibility map does not match reducer state") {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreJSONContainsNoMachineAuthorityTerms(t *testing.T) {
	raw, err := json.Marshal(testRun(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{"provider_id", "binding_id", "bundle", "grant", "lease", "receipt", "machine_revision"} {
		if strings.Contains(strings.ToLower(string(raw)), term) {
			t.Fatalf("state contains %q: %s", term, raw)
		}
	}
}

func testRun(t *testing.T) Run {
	t.Helper()
	module := policyflow.New()
	inventory := testInventory(t, module)
	offer, err := module.Offer(inventory)
	if err != nil {
		t.Fatal(err)
	}
	progress, err := module.Start(inventory, policyflow.Selection{OfferRef: offer.Ref, Profile: policyflow.ProfileSPFull})
	if err != nil {
		t.Fatal(err)
	}
	state, err := module.Export(progress)
	if err != nil {
		t.Fatal(err)
	}
	responsibilities, err := ProjectResponsibilities(policyflow.ProfileSPFull, offer)
	if err != nil {
		t.Fatal(err)
	}
	return Run{
		SchemaVersion: RunSchemaV6, ID: "run-1", ProjectRoot: t.TempDir(),
		Plan: Plan{
			Assurance: AssurancePolicyCooperative, ActivationSource: ActivationUserExplicit,
			Deliverable: "scope", Mode: ModeWorkflow, Complexity: "complex", Risk: "normal",
			SelectedProfileCandidate: string(policyflow.ProfileSPFull), SelectionSource: SelectionUserExplicit,
			Topology: TopologyCurrent, ResponsibilityMap: responsibilities,
			AcceptedLimitations: []string{"policy-only"}, AddOn: AddOnNone, Status: StatusActive,
		},
		Inventory: inventory, State: state,
	}
}

func testInventory(t *testing.T, module *policyflow.Module) policyflow.RouteInventory {
	t.Helper()
	offer, err := module.Offer(nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]policyflow.Route{}
	for _, profile := range offer.Profiles {
		for _, route := range profile.Routes {
			mode := policyflow.HostVisible
			if route.Kind == policyflow.RouteHostAction {
				mode = policyflow.HostControlled
			}
			seen[route.Name] = policyflow.Route{Name: route.Name, Mode: mode}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make(policyflow.RouteInventory, 0, len(names))
	for _, name := range names {
		result = append(result, seen[name])
	}
	return result
}
