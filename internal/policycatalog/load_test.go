package policycatalog

import "testing"

func TestLoadReturnsAuthorityNeutralBuiltInSemantics(t *testing.T) {
	profiles, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []ProfileID{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"}
	if len(profiles) != len(want) {
		t.Fatalf("profiles = %d", len(profiles))
	}
	for index, profile := range profiles {
		if profile.ID != want[index] || len(profile.Steps) == 0 {
			t.Fatalf("profile[%d] = %#v", index, profile)
		}
	}
}

func TestLoadReturnsIndependentSnapshots(t *testing.T) {
	first, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	first[0].Steps[0].Name = "changed"
	first[0].Steps[0].Covers[0] = "changed"
	first[1].Requirements[0].Route = "changed"
	second, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Steps[0].Name == "changed" || second[0].Steps[0].Covers[0] == "changed" ||
		second[1].Requirements[0].Route == "changed" {
		t.Fatal("Load returned shared mutable Profile semantics")
	}
}

func TestMattRequirementsNameTheirCreditedSlots(t *testing.T) {
	profiles, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []Requirement{
		{Route: "grilling", Slot: "problem-framing"},
		{Route: "domain-modeling", Slot: "problem-framing"},
		{Route: "tdd", Slot: "implementation-tdd"},
		{Route: "code-review", Slot: "review-remediation"},
	}
	if got := profiles[1].Requirements; !equalRequirements(got, want) {
		t.Fatalf("MATT-FULL requirements = %#v, want %#v", got, want)
	}
}

func equalRequirements(left, right []Requirement) bool {
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

func TestPolicyCatalogDeclaresOnlyImplementedStableBoundaries(t *testing.T) {
	profiles, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		for _, boundary := range profile.StableBoundaries {
			if boundary == "ticket-complete" {
				t.Fatalf("%s declares ticket-complete without a Policy ticket model", profile.ID)
			}
		}
	}
}

func TestEveryExecutableStepCoversItsOwnSlot(t *testing.T) {
	profiles, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		for _, step := range profile.Steps {
			if step.Kind == StepUserGate || step.Kind == StepHostGate {
				continue
			}
			covered := false
			for _, slot := range step.Covers {
				covered = covered || slot == step.Slot
			}
			if !covered {
				t.Errorf("%s step %s does not cover its slot %s", profile.ID, step.Name, step.Slot)
			}
		}
	}
}

func TestECCGenericReviewUsesTypedHostActionWithoutPRDependency(t *testing.T) {
	profiles, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		if profile.ID != "ECC-FULL" {
			continue
		}
		for _, step := range profile.Steps {
			if step.Slot != "review-remediation" {
				continue
			}
			if step.Kind != StepHostAction || step.Name != "review.execute" || !step.ReviewOutcome {
				t.Fatalf("ECC review step = %#v", step)
			}
			return
		}
		t.Fatal("ECC review step is missing")
	}
	t.Fatal("ECC-FULL is missing")
}

func TestHybridDeclaresUnavailableTypedTechnicalIncidents(t *testing.T) {
	profiles, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []IncidentRoute{
		{Incident: "functional-failure", Skill: "diagnosing-bugs", ReturnTo: "implementation"},
		{Incident: "hard-bug", Skill: "diagnosing-bugs", ReturnTo: "implementation"},
		{Incident: "performance-regression", Skill: "diagnosing-bugs", ReturnTo: "implementation"},
		{Incident: "build-failure", ReturnTo: "implementation"},
		{Incident: "dependency-failure", ReturnTo: "implementation"},
		{Incident: "type-failure", ReturnTo: "implementation"},
	}
	for _, profile := range profiles {
		if profile.ID != "MATT-SP-HYBRID" {
			continue
		}
		if len(profile.Incidents) != len(want) {
			t.Fatalf("Hybrid incidents = %#v, want %#v", profile.Incidents, want)
		}
		for index := range want {
			if profile.Incidents[index] != want[index] {
				t.Fatalf("Hybrid incident[%d] = %#v, want %#v", index, profile.Incidents[index], want[index])
			}
		}
		return
	}
	t.Fatal("MATT-SP-HYBRID is missing")
}
