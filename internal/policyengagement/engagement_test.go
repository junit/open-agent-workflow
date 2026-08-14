package policyengagement

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyrun"
)

func TestProfilesExposeBridgeFreePolicyAndHostProjections(t *testing.T) {
	root := engagementFixture(t, true)
	t.Chdir(filepath.Join(root, "project"))

	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	result, err := module.Execute(Command{Action: ActionProfiles})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"}
	if len(result.Profiles) != len(want) {
		t.Fatalf("profiles = %#v", result.Profiles)
	}
	for index, profile := range result.Profiles {
		if profile.Name != want[index] || !profile.PolicySelectable || !profile.HostRoutable || len(profile.Missing) != 0 {
			t.Fatalf("profile[%d] = %#v", index, profile)
		}
	}
	ecc := findProfile(t, result.Profiles, string(policyflow.ProfileECCFull))
	for _, incident := range ecc.IncidentRoutes {
		if incident.Incident == string(policyflow.IncidentBuildFailure) &&
			(incident.Status != "unavailable-if-triggered" || incident.Skill != "") {
			t.Fatalf("ECC unavailable incident route = %#v", incident)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "state", "open-agent-workflow", "policy-engagements")); !os.IsNotExist(err) {
		t.Fatalf("read-only profile inspection created Engagement state: %v", err)
	}
}

func TestBridgeStateDoesNotChangePolicyProfiles(t *testing.T) {
	root := engagementFixture(t, true)
	t.Chdir(filepath.Join(root, "project"))
	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	withoutBridge, err := module.Execute(Command{Action: ActionProfiles})
	if err != nil {
		t.Fatal(err)
	}
	bridgeState := filepath.Join(root, "state", "open-agent-workflow", "codex-bridge")
	if err := os.MkdirAll(bridgeState, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bridgeState, "install.json"), []byte(`{"schema_version":"oaw.codex-bridge-install/v1","bridge":"present"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	withBridge, err := module.Execute(Command{Action: ActionProfiles})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withoutBridge.Profiles, withBridge.Profiles) {
		t.Fatalf("Bridge changed Policy profiles: without=%#v with=%#v", withoutBridge.Profiles, withBridge.Profiles)
	}
}

func TestProfilesKeepPolicySelectableWhenAHostRouteIsMissing(t *testing.T) {
	root := engagementFixture(t, false)
	t.Chdir(filepath.Join(root, "project"))

	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	result, err := module.Execute(Command{Action: ActionProfiles})
	if err != nil {
		t.Fatal(err)
	}
	matt := findProfile(t, result.Profiles, string(policyflow.ProfileMattFull))
	if !matt.PolicySelectable || matt.HostRoutable || !slices.Contains(matt.Missing, "implement") {
		t.Fatalf("MATT-FULL = %#v", matt)
	}
}

func TestUseRequiresReportedCooperativeAssessment(t *testing.T) {
	root := engagementFixture(t, true)
	t.Chdir(filepath.Join(root, "project"))
	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	_, err = module.Execute(Command{Action: ActionUse, Profile: policyflow.ProfileSPFull, Intent: "unassessed work"})
	if err == nil || !strings.Contains(err.Error(), "POLICY_ASSESSMENT_REQUIRED") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecutePersistsBusinessProgressWithoutPublicReducerReferences(t *testing.T) {
	root := engagementFixture(t, true)
	t.Chdir(filepath.Join(root, "project"))

	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	started, err := module.Execute(Command{
		Action: ActionUse, Profile: policyflow.ProfileSPFull, Intent: "Typora-like editor", Complexity: "complex", Risk: "normal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.Status == nil || started.Status.Profile != string(policyflow.ProfileSPFull) || started.Status.Next != "invoke-user-skill" {
		t.Fatalf("started = %#v", started)
	}

	advanced, err := module.Execute(Command{Action: ActionComplete})
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Status == nil || advanced.Status.State != "active" || advanced.Status.Next == "invoke-user-skill" {
		t.Fatalf("advanced = %#v", advanced)
	}

	reloaded, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	status, err := reloaded.Execute(Command{Action: ActionStatus})
	if err != nil {
		t.Fatal(err)
	}
	if status.Status == nil || *status.Status != *advanced.Status {
		t.Fatalf("reloaded status = %#v, want %#v", status.Status, advanced.Status)
	}
}

func TestEveryBridgeFreeProfilePersistsPlanFromPolicyOffer(t *testing.T) {
	for _, profile := range []policyflow.ProfileID{
		policyflow.ProfileSPFull,
		policyflow.ProfileMattFull,
		policyflow.ProfileECCFull,
		policyflow.ProfileMattSPHybrid,
	} {
		t.Run(string(profile), func(t *testing.T) {
			root := engagementFixture(t, true)
			t.Chdir(filepath.Join(root, "project"))

			module, err := NewCurrent()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := module.Execute(Command{Action: ActionUse, Profile: profile, Intent: "bridge-free work", Complexity: "complex", Risk: "normal"}); err != nil {
				t.Fatal(err)
			}

			raw, err := os.ReadFile(filepath.Join(root, "state", "open-agent-workflow", "policy-engagements", module.id+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var run policyrun.Run
			if err := json.Unmarshal(raw, &run); err != nil {
				t.Fatal(err)
			}
			if run.SchemaVersion != policyrun.RunSchemaV6 || run.Plan.SelectedProfileCandidate != string(profile) || len(run.Plan.ResponsibilityMap) != 10 {
				t.Fatalf("persisted Plan = %#v", run.Plan)
			}
			for slot, responsibilities := range run.Plan.ResponsibilityMap {
				if len(responsibilities) == 0 {
					t.Errorf("slot %s has no projected responsibility", slot)
				}
			}
		})
	}
}

func TestMattPlanPreservesCreditedInternalResponsibilities(t *testing.T) {
	root := engagementFixture(t, true)
	t.Chdir(filepath.Join(root, "project"))
	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.Execute(Command{Action: ActionUse, Profile: policyflow.ProfileMattFull, Intent: "bridge-free Matt work", Complexity: "complex", Risk: "normal"}); err != nil {
		t.Fatal(err)
	}
	run, err := module.store.Load(module.id)
	if err != nil {
		t.Fatal(err)
	}
	assertResponsibility(t, run.Plan.ResponsibilityMap["implementation-tdd"], "tdd", "credited-skill")
	assertResponsibility(t, run.Plan.ResponsibilityMap["review-remediation"], "code-review", "credited-skill")
	assertResponsibility(t, run.Plan.ResponsibilityMap["problem-framing"], "shared-understanding", "user-gate")
	assertResponsibility(t, run.Plan.ResponsibilityMap["fresh-verification"], "fresh-evidence", "host-gate")
}

func TestProfileSwitchReprojectsAndReloadsResponsibilityMap(t *testing.T) {
	root := engagementFixture(t, true)
	t.Chdir(filepath.Join(root, "project"))
	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.Execute(Command{Action: ActionUse, Profile: policyflow.ProfileSPFull, Intent: "switch profiles", Complexity: "complex", Risk: "normal"}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []Action{ActionComplete, ActionApprove, ActionApprove} {
		if _, err := module.Execute(Command{Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := module.Execute(Command{Action: ActionSwitch, Profile: policyflow.ProfileMattFull}); err != nil {
		t.Fatal(err)
	}

	run, err := module.store.Load(module.id)
	if err != nil {
		t.Fatal(err)
	}
	if run.Plan.SelectedProfileCandidate != string(policyflow.ProfileMattFull) {
		t.Fatalf("selected profile = %q", run.Plan.SelectedProfileCandidate)
	}
	assertResponsibility(t, run.Plan.ResponsibilityMap["problem-framing"], "grill-with-docs", "skill")
	for _, responsibility := range run.Plan.ResponsibilityMap["problem-framing"] {
		if responsibility.Route == "superpowers:brainstorming" {
			t.Fatalf("responsibility map retained SP route: %#v", run.Plan.ResponsibilityMap)
		}
	}

	reloaded, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	status, err := reloaded.Execute(Command{Action: ActionStatus})
	if err != nil {
		t.Fatal(err)
	}
	if status.Status == nil || status.Status.Profile != string(policyflow.ProfileMattFull) {
		t.Fatalf("reloaded status = %#v", status.Status)
	}
}

func TestRouteDriftStillPersistsExplicitStop(t *testing.T) {
	root := engagementFixture(t, true)
	t.Chdir(filepath.Join(root, "project"))
	module, err := NewCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.Execute(Command{Action: ActionUse, Profile: policyflow.ProfileSPFull, Intent: "drifting work", Complexity: "complex", Risk: "normal"}); err != nil {
		t.Fatal(err)
	}
	brainstorming := filepath.Join(root, "home", ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "v1", "skills", "brainstorming", "SKILL.md")
	if err := os.Remove(brainstorming); err != nil {
		t.Fatal(err)
	}
	stopped, err := module.Execute(Command{Action: ActionStop, Reason: "route disappeared"})
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status == nil || stopped.Status.State != "stopped" {
		t.Fatalf("stopped = %#v", stopped.Status)
	}
	run, err := module.store.Load(module.id)
	if err != nil {
		t.Fatal(err)
	}
	for _, responsibility := range run.Plan.ResponsibilityMap["problem-framing"] {
		if responsibility.Route == "superpowers:brainstorming" && responsibility.Mode == "unavailable" {
			return
		}
	}
	t.Fatalf("drifted route is not recorded as unavailable: %#v", run.Plan.ResponsibilityMap["problem-framing"])
}

func assertResponsibility(t *testing.T, values []policyrun.Responsibility, route, kind string) {
	t.Helper()
	for _, value := range values {
		if value.Route == route && value.Kind == kind {
			return
		}
	}
	t.Errorf("responsibility %s/%s is missing from %#v", route, kind, values)
}

func engagementFixture(t *testing.T, includeMattImplement bool) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	project := filepath.Join(root, "project")
	state := filepath.Join(root, "state")
	for _, directory := range []string{home, project, state} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", state)
	writeEngagementFixture(t, filepath.Join(home, ".codex", "config.toml"), "[plugins.\"superpowers@openai-api-curated\"]\nenabled = true\n[plugins.\"ecc@ecc\"]\nenabled = true\n")

	matt := []string{"grill-with-docs", "grilling", "domain-modeling", "to-spec", "to-tickets", "tdd", "diagnosing-bugs", "code-review"}
	if includeMattImplement {
		matt = append(matt, "implement")
	}
	for _, skill := range matt {
		writeEngagementFixture(t, filepath.Join(home, ".agents", "skills", skill, "SKILL.md"), "fixture")
	}
	for _, skill := range []string{"brainstorming", "writing-plans", "using-git-worktrees", "executing-plans", "test-driven-development", "systematic-debugging", "requesting-code-review", "receiving-code-review", "verification-before-completion", "finishing-a-development-branch"} {
		writeEngagementFixture(t, filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "v1", "skills", skill, "SKILL.md"), "fixture")
	}
	for _, skill := range []string{"intent-driven-development", "product-capability", "blueprint", "git-workflow", "tdd-workflow", "verification-loop"} {
		writeEngagementFixture(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills", skill, "SKILL.md"), "fixture")
	}
	return root
}

func findProfile(t *testing.T, profiles []Profile, name string) Profile {
	t.Helper()
	for _, profile := range profiles {
		if profile.Name == name {
			return profile
		}
	}
	t.Fatalf("profile %q is missing from %#v", name, profiles)
	return Profile{}
}

func writeEngagementFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
