package builtin

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/policycatalog"
)

// This test is the only place where the cooperative and machine projections
// meet. Production Policy code remains independent of Provider attestation.
func TestPolicyAndMachineProjectionsShareStableProfileContract(t *testing.T) {
	policyProfiles, err := policycatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	machineMatrix := buildMatrix(t)

	wantAliases := map[string]string{
		"SP-FULL":        "oaw/delivery",
		"MATT-FULL":      "oaw/domain-engineering",
		"ECC-FULL":       "oaw/ecc-engineering",
		"MATT-SP-HYBRID": "oaw/reliable-feature",
	}
	policyByAlias := make(map[string]policycatalog.Profile, len(policyProfiles))
	for _, profile := range policyProfiles {
		policyByAlias[string(profile.ID)] = profile
	}
	machineByAlias := make(map[string]MatrixProfile, len(machineMatrix.Profiles))
	for _, profile := range machineMatrix.Profiles {
		machineByAlias[profile.Alias] = profile
	}
	if len(policyByAlias) != len(wantAliases) || len(machineByAlias) != len(wantAliases) {
		t.Fatalf("profile aliases differ: policy=%v machine=%v", sortedKeys(policyByAlias), sortedKeys(machineByAlias))
	}

	transportDiffs := []string{}
	for alias, recipeID := range wantAliases {
		t.Run(alias, func(t *testing.T) {
			policyProfile, policyFound := policyByAlias[alias]
			machineProfile, machineFound := machineByAlias[alias]
			if !policyFound || !machineFound {
				t.Fatalf("alias missing: policy=%t machine=%t", policyFound, machineFound)
			}
			if machineProfile.RecipeID != recipeID {
				t.Fatalf("machine Recipe = %q, want %q", machineProfile.RecipeID, recipeID)
			}

			machineSlots := make(map[catalog.SlotID]MatrixSlot, len(machineProfile.Slots))
			for _, slot := range machineProfile.Slots {
				machineSlots[slot.SlotID] = slot
			}
			for _, definition := range catalog.CanonicalSlots() {
				machineSlot, found := machineSlots[definition.ID]
				if !found {
					t.Errorf("machine projection is missing slot %s", definition.ID)
					continue
				}
				if definition.ID == catalog.SlotIncidentRecovery {
					if machineSlot.Applicability != catalog.SlotConditional {
						t.Errorf("incident-recovery applicability = %q", machineSlot.Applicability)
					}
					continue
				}
				if machineSlot.Applicability != catalog.SlotMandatory {
					t.Errorf("slot %s applicability = %q", definition.ID, machineSlot.Applicability)
					continue
				}

				policyFamilies := policyResponsibilityFamilies(t, policyProfile, policycatalog.SlotID(definition.ID))
				machineFamilies := machineResponsibilityFamilies(t, machineSlot)
				if policyMachineTransportDiffAllowed(alias, definition.ID, policyProfile, machineSlot, policyFamilies, machineFamilies) {
					// Policy uses a typed neutral Host review because the public ECC
					// command is PR-only. Machine execution may use an attested ECC
					// reviewer; both still own the same mandatory review outcome.
					transportDiffs = append(transportDiffs, alias+"/"+string(definition.ID))
				} else if !reflect.DeepEqual(policyFamilies, machineFamilies) {
					t.Errorf("slot %s responsibility families: policy=%v machine=%v", definition.ID, policyFamilies, machineFamilies)
				}

				policyGates := policyGateNames(policyProfile, policycatalog.SlotID(definition.ID))
				machineGates := sortedUnique(machineSlot.GateIDs)
				if !reflect.DeepEqual(policyGates, machineGates) {
					t.Errorf("slot %s gates: policy=%v machine=%v", definition.ID, policyGates, machineGates)
				}
			}
			if len(machineSlots) != len(catalog.CanonicalSlots()) {
				t.Errorf("machine slot count = %d, want %d", len(machineSlots), len(catalog.CanonicalSlots()))
			}
		})
	}
	if want := []string{"ECC-FULL/review-remediation"}; !reflect.DeepEqual(sortedUnique(transportDiffs), want) {
		t.Errorf("Policy/machine transport differences = %v, want %v", sortedUnique(transportDiffs), want)
	}
}

func TestPolicyAndMachineProjectionsShareMattCreditedMacroResponsibilities(t *testing.T) {
	policyByAlias, machineByAlias := projectionProfiles(t)
	for _, alias := range sortedKeys(policyByAlias) {
		t.Run(alias, func(t *testing.T) {
			policyCredits := policyMattCredits(policyByAlias[alias])
			machineCredits := machineMattCredits(machineByAlias[alias])
			if !reflect.DeepEqual(policyCredits, machineCredits) {
				t.Errorf("Matt credited responsibilities: policy=%v machine=%v", policyCredits, machineCredits)
			}
		})
	}
}

func TestPolicyAndMachineProjectionsShareTypedIncidentSets(t *testing.T) {
	policyByAlias, machineByAlias := projectionProfiles(t)
	for _, alias := range sortedKeys(policyByAlias) {
		t.Run(alias, func(t *testing.T) {
			policyIncidents := policyIncidentTypes(policyByAlias[alias])
			machineIncidents := machineIncidentTypes(t, machineByAlias[alias])
			if !reflect.DeepEqual(policyIncidents, machineIncidents) {
				t.Errorf("typed incidents: policy=%v machine=%v", policyIncidents, machineIncidents)
			}
		})
	}
}

func TestPolicyAndMachineProjectionsShareStableBoundariesExceptMachineTickets(t *testing.T) {
	policyByAlias, machineByAlias := projectionProfiles(t)
	machineCatalog := loadCatalog(t)
	machineRecipes := make(map[string]catalog.ProfileRecipeRecord, len(machineCatalog.Recipes()))
	for _, recipe := range machineCatalog.Recipes() {
		machineRecipes[recipe.ID] = recipe
	}

	for _, alias := range sortedKeys(policyByAlias) {
		t.Run(alias, func(t *testing.T) {
			policyBoundaries := sortedStrings(policyByAlias[alias].StableBoundaries)
			if containsString(policyBoundaries, "ticket-complete") {
				t.Fatal("Policy declares ticket-complete without a Policy ticket model")
			}

			machineProfile := machineByAlias[alias]
			machineRecipe, found := machineRecipes[machineProfile.RecipeID]
			if !found {
				t.Fatalf("machine Recipe %q is missing", machineProfile.RecipeID)
			}
			machineWithoutTickets := []string{}
			ticketBoundaries := 0
			for _, boundary := range machineRecipe.StableBoundaries {
				if boundary == "ticket-complete" {
					ticketBoundaries++
					continue
				}
				machineWithoutTickets = append(machineWithoutTickets, boundary)
			}
			if ticketBoundaries != 1 {
				t.Errorf("machine ticket-complete boundary count = %d, want 1", ticketBoundaries)
			}
			machineWithoutTickets = sortedStrings(machineWithoutTickets)
			if !reflect.DeepEqual(policyBoundaries, machineWithoutTickets) {
				t.Errorf("stable boundaries excluding machine ticket model: policy=%v machine=%v", policyBoundaries, machineWithoutTickets)
			}
		})
	}
}

func policyMachineTransportDiffAllowed(alias string, slot catalog.SlotID, policyProfile policycatalog.Profile, machineSlot MatrixSlot, policy, machine []string) bool {
	if alias != "ECC-FULL" || slot != catalog.SlotReviewRemediation ||
		!reflect.DeepEqual(policy, []string{"host"}) || !reflect.DeepEqual(machine, []string{"ecc"}) {
		return false
	}
	policyReviewActions := []string{}
	for _, step := range policyProfile.Steps {
		if step.Slot == policycatalog.SlotID(slot) && step.Kind == policycatalog.StepHostAction && step.ReviewOutcome {
			policyReviewActions = append(policyReviewActions, step.Name)
		}
	}
	if !reflect.DeepEqual(policyReviewActions, []string{"review.execute"}) || machineSlot.HostActionID != "" || machineSlot.OutcomeOwner != "oaw/ecc/codex-reviewer" {
		return false
	}
	for _, binding := range machineSlot.Pipeline {
		if !binding.Paused && binding.ProviderID == "oaw/ecc" && binding.BindingID == "codex-reviewer" {
			return true
		}
	}
	return false
}

func projectionProfiles(t *testing.T) (map[string]policycatalog.Profile, map[string]MatrixProfile) {
	t.Helper()
	policyProfiles, err := policycatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	policyByAlias := make(map[string]policycatalog.Profile, len(policyProfiles))
	for _, profile := range policyProfiles {
		policyByAlias[string(profile.ID)] = profile
	}
	machineProfiles := buildMatrix(t).Profiles
	machineByAlias := make(map[string]MatrixProfile, len(machineProfiles))
	for _, profile := range machineProfiles {
		machineByAlias[profile.Alias] = profile
	}
	if !reflect.DeepEqual(sortedKeys(policyByAlias), sortedKeys(machineByAlias)) {
		t.Fatalf("profile aliases differ: policy=%v machine=%v", sortedKeys(policyByAlias), sortedKeys(machineByAlias))
	}
	return policyByAlias, machineByAlias
}

func policyMattCredits(profile policycatalog.Profile) []string {
	credits := make([]string, 0, len(profile.Requirements))
	for _, requirement := range profile.Requirements {
		credits = append(credits, requirement.Route+"@"+string(requirement.Slot))
	}
	return sortedStrings(credits)
}

func machineMattCredits(profile MatrixProfile) []string {
	credits := []string{}
	for _, slot := range profile.Slots {
		for _, binding := range slot.Pipeline {
			if binding.Paused || binding.ProviderID != "oaw/matt" || binding.MacroMode != catalog.InternalCreditOnly {
				continue
			}
			for _, creditedSlot := range binding.StageSpan {
				credits = append(credits, binding.Reference+"@"+string(creditedSlot))
			}
		}
	}
	return sortedUnique(credits)
}

func policyIncidentTypes(profile policycatalog.Profile) []string {
	incidents := make([]string, 0, len(profile.Incidents))
	for _, route := range profile.Incidents {
		incidents = append(incidents, route.Incident)
	}
	return sortedStrings(incidents)
}

func machineIncidentTypes(t *testing.T, profile MatrixProfile) []string {
	t.Helper()
	for _, slot := range profile.Slots {
		if slot.SlotID == catalog.SlotIncidentRecovery {
			return sortedStrings(slot.IncidentTypes)
		}
	}
	t.Fatal("machine projection has no incident-recovery slot")
	return nil
}

func policyResponsibilityFamilies(t *testing.T, profile policycatalog.Profile, slot policycatalog.SlotID) []string {
	t.Helper()
	families := []string{}
	for _, step := range profile.Steps {
		if step.Slot != slot && !containsPolicySlot(step.Covers, slot) {
			continue
		}
		switch step.Kind {
		case policycatalog.StepHostAction:
			families = append(families, "host")
		case policycatalog.StepSkill:
			family, ok := policySkillFamily(step.Name)
			if !ok {
				t.Errorf("unknown Policy Skill family for %q", step.Name)
				continue
			}
			families = append(families, family)
		}
	}
	return sortedUnique(families)
}

func machineResponsibilityFamilies(t *testing.T, slot MatrixSlot) []string {
	t.Helper()
	families := []string{}
	if strings.HasPrefix(slot.OutcomeOwner, "host-action:") {
		families = append(families, "host")
	} else if slot.OutcomeOwner != "none" {
		family, ok := machineProviderFamily(slot.OutcomeOwner)
		if !ok {
			t.Errorf("unknown machine outcome-owner family for %q", slot.OutcomeOwner)
		} else {
			families = append(families, family)
		}
	}
	for _, binding := range slot.Pipeline {
		if binding.Paused {
			continue
		}
		family, ok := machineProviderFamily(binding.ProviderID)
		if !ok {
			t.Errorf("unknown machine Provider family for %q", binding.ProviderID)
			continue
		}
		families = append(families, family)
	}
	return sortedUnique(families)
}

func policyGateNames(profile policycatalog.Profile, slot policycatalog.SlotID) []string {
	values := []string{}
	for _, step := range profile.Steps {
		if step.Slot == slot && (step.Kind == policycatalog.StepUserGate || step.Kind == policycatalog.StepHostGate) {
			values = append(values, step.Name)
		}
	}
	return sortedUnique(values)
}

func policySkillFamily(name string) (string, bool) {
	if strings.HasPrefix(name, "superpowers:") {
		return "superpowers", true
	}
	if strings.HasPrefix(name, "ecc:") {
		return "ecc", true
	}
	switch name {
	case "grill-with-docs", "to-spec", "to-tickets", "implement", "tdd", "diagnosing-bugs":
		return "matt", true
	default:
		return "", false
	}
}

func machineProviderFamily(value string) (string, bool) {
	switch {
	case strings.HasPrefix(value, "oaw/superpowers/") || value == "oaw/superpowers":
		return "superpowers", true
	case strings.HasPrefix(value, "oaw/matt/") || value == "oaw/matt":
		return "matt", true
	case strings.HasPrefix(value, "oaw/ecc/") || value == "oaw/ecc":
		return "ecc", true
	default:
		return "", false
	}
}

func containsPolicySlot(values []policycatalog.SlotID, want policycatalog.SlotID) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sortedUnique(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedStrings(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	sort.Strings(result)
	return result
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sortedKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
