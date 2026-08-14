// Package policycatalog owns the authority-neutral built-in Profile semantics
// used by cooperative Policy execution. Provider identity, Host Bindings,
// discovery paths, integrity metadata, and machine authority are deliberately
// absent from its Interface.
package policycatalog

import (
	"errors"
	"sort"
)

type ProfileID string
type SlotID string
type StepKind string
type InvocationMode string

const (
	StepSkill      StepKind = "skill"
	StepHostAction StepKind = "host-action"
	StepUserGate   StepKind = "user-gate"
	StepHostGate   StepKind = "host-gate"

	HostVisible  InvocationMode = "host-visible"
	UserExplicit InvocationMode = "user-explicit"
)

type Step struct {
	Kind          StepKind
	Name          string
	Slot          SlotID
	Covers        []SlotID
	Completes     []SlotID
	ReviewOutcome bool
}

type IncidentRoute struct {
	Incident string
	Skill    string
	ReturnTo SlotID
}

// Requirement is a credited internal Skill contract used by a composite
// public Skill. It is routable but is not dispatched as a separate step.
type Requirement struct {
	Route string
	Slot  SlotID
}

type Profile struct {
	ID               ProfileID
	Steps            []Step
	Requirements     []Requirement
	Incidents        []IncidentRoute
	StableBoundaries []string
}

// Load returns one immutable snapshot of the built-in cooperative semantics.
func Load() ([]Profile, error) {
	profiles := builtInProfiles()
	if err := validate(profiles); err != nil {
		return nil, err
	}
	return cloneProfiles(profiles), nil
}

func builtInProfiles() []Profile {
	return []Profile{
		spFull(),
		mattFull(),
		eccFull(),
		mattSPHybrid(),
	}
}

func spFull() Profile {
	return Profile{
		ID: "SP-FULL",
		Steps: []Step{
			skill("superpowers:brainstorming", "problem-framing", "problem-framing", "solution-specification"),
			userGate("shared-understanding", "problem-framing"),
			userGate("specification-approved", "solution-specification"),
			skill("superpowers:writing-plans", "delivery-planning", "delivery-planning"),
			userGate("delivery-plan-approved", "delivery-planning"),
			skill("superpowers:using-git-worktrees", "workspace-preparation", "workspace-preparation"),
			skill("superpowers:executing-plans", "implementation", "implementation"),
			skill("superpowers:test-driven-development", "implementation-tdd", "implementation-tdd"),
			reviewSkill("superpowers:requesting-code-review", "review-remediation"),
			skill("superpowers:receiving-code-review", "review-remediation", "review-remediation"),
			skill("superpowers:verification-before-completion", "fresh-verification", "fresh-verification"),
			hostGate("fresh-evidence", "fresh-verification"),
			skill("superpowers:finishing-a-development-branch", "closeout", "closeout"),
			userGate("user-closeout", "closeout"),
		},
		Incidents: []IncidentRoute{
			incident("build-failure", "superpowers:systematic-debugging"),
			incident("dependency-failure", "superpowers:systematic-debugging"),
			incident("functional-failure", "superpowers:systematic-debugging"),
			incident("type-failure", "superpowers:systematic-debugging"),
		},
		Requirements:     []Requirement{},
		StableBoundaries: stableBoundaries(),
	}
}

func mattFull() Profile {
	return Profile{
		ID: "MATT-FULL",
		Steps: []Step{
			skill("grill-with-docs", "problem-framing", "problem-framing"),
			userGate("shared-understanding", "problem-framing"),
			skill("to-spec", "solution-specification", "solution-specification"),
			userGate("specification-approved", "solution-specification"),
			skill("to-tickets", "delivery-planning", "delivery-planning"),
			userGate("delivery-plan-approved", "delivery-planning"),
			hostAction("workspace.prepare-or-confirm", "workspace-preparation", "workspace-preparation"),
			hostGate("workspace-ready", "workspace-preparation"),
			// implement is the public Matt macro. Its internal TDD and review
			// calls receive credit here and are not dispatched a second time.
			reviewSkill("implement", "implementation", "implementation", "implementation-tdd", "review-remediation"),
			hostAction("verification.execute", "fresh-verification", "fresh-verification"),
			hostGate("fresh-evidence", "fresh-verification"),
			hostAction("closeout.execute", "closeout", "closeout"),
			userGate("user-closeout", "closeout"),
		},
		Incidents: []IncidentRoute{
			incident("functional-failure", "diagnosing-bugs"),
			incident("hard-bug", "diagnosing-bugs"),
			incident("performance-regression", "diagnosing-bugs"),
		},
		Requirements: []Requirement{
			requirement("grilling", "problem-framing"),
			requirement("domain-modeling", "problem-framing"),
			requirement("tdd", "implementation-tdd"),
			requirement("code-review", "review-remediation"),
		},
		StableBoundaries: stableBoundaries(),
	}
}

func eccFull() Profile {
	return Profile{
		ID: "ECC-FULL",
		Steps: []Step{
			skill("ecc:intent-driven-development", "problem-framing", "problem-framing"),
			userGate("shared-understanding", "problem-framing"),
			skill("ecc:product-capability", "solution-specification", "solution-specification"),
			userGate("specification-approved", "solution-specification"),
			skill("ecc:blueprint", "delivery-planning", "delivery-planning"),
			userGate("delivery-plan-approved", "delivery-planning"),
			skill("ecc:git-workflow", "workspace-preparation"),
			hostAction("workspace.prepare-or-confirm", "workspace-preparation", "workspace-preparation"),
			hostGate("workspace-ready", "workspace-preparation"),
			skill("ecc:tdd-workflow", "implementation", "implementation", "implementation-tdd"),
			reviewHostAction("review.execute", "review-remediation", "review-remediation"),
			skill("ecc:verification-loop", "fresh-verification", "fresh-verification"),
			hostGate("fresh-evidence", "fresh-verification"),
			skill("ecc:git-workflow", "closeout"),
			hostAction("closeout.execute", "closeout", "closeout"),
			userGate("user-closeout", "closeout"),
		},
		// The built-in Codex Policy profile has no public ECC Skill contract
		// for build/type/dependency recovery. Empty routes preserve the typed
		// incident contract while causing an honest stop when one occurs.
		Incidents: []IncidentRoute{
			unavailableIncident("build-failure"),
			unavailableIncident("dependency-failure"),
			unavailableIncident("type-failure"),
		},
		Requirements:     []Requirement{},
		StableBoundaries: stableBoundaries(),
	}
}

func mattSPHybrid() Profile {
	return Profile{
		ID: "MATT-SP-HYBRID",
		Steps: []Step{
			skill("grill-with-docs", "problem-framing", "problem-framing"),
			userGate("shared-understanding", "problem-framing"),
			skill("to-spec", "solution-specification", "solution-specification"),
			userGate("specification-approved", "solution-specification"),
			skill("to-tickets", "delivery-planning"),
			skill("superpowers:writing-plans", "delivery-planning", "delivery-planning"),
			userGate("delivery-plan-approved", "delivery-planning"),
			skill("superpowers:using-git-worktrees", "workspace-preparation", "workspace-preparation"),
			skill("superpowers:executing-plans", "implementation", "implementation"),
			skill("tdd", "implementation-tdd", "implementation-tdd"),
			reviewSkill("superpowers:requesting-code-review", "review-remediation"),
			skill("superpowers:receiving-code-review", "review-remediation", "review-remediation"),
			skill("superpowers:verification-before-completion", "fresh-verification", "fresh-verification"),
			hostGate("fresh-evidence", "fresh-verification"),
			skill("superpowers:finishing-a-development-branch", "closeout", "closeout"),
			userGate("user-closeout", "closeout"),
		},
		Incidents: []IncidentRoute{
			incident("functional-failure", "diagnosing-bugs"),
			incident("hard-bug", "diagnosing-bugs"),
			incident("performance-regression", "diagnosing-bugs"),
			// The built-in Policy profile has no selected ECC Incident
			// Handler Add-on. Keep these incidents typed and stop only if
			// one is reported; they are not normal Profile requirements.
			unavailableIncident("build-failure"),
			unavailableIncident("dependency-failure"),
			unavailableIncident("type-failure"),
		},
		Requirements: []Requirement{
			requirement("grilling", "problem-framing"),
			requirement("domain-modeling", "problem-framing"),
		},
		StableBoundaries: stableBoundaries(),
	}
}

func skill(name string, slot SlotID, completes ...SlotID) Step {
	return Step{
		Kind: StepSkill, Name: name, Slot: slot,
		Covers: coverage(slot, completes), Completes: append([]SlotID(nil), completes...),
	}
}

func reviewSkill(name string, slot SlotID, completes ...SlotID) Step {
	step := skill(name, slot, completes...)
	step.ReviewOutcome = true
	return step
}

func hostAction(name string, slot SlotID, completes ...SlotID) Step {
	return Step{
		Kind: StepHostAction, Name: name, Slot: slot,
		Covers: coverage(slot, completes), Completes: append([]SlotID(nil), completes...),
	}
}

func reviewHostAction(name string, slot SlotID, completes ...SlotID) Step {
	step := hostAction(name, slot, completes...)
	step.ReviewOutcome = true
	return step
}

func coverage(slot SlotID, completes []SlotID) []SlotID {
	result := []SlotID{slot}
	for _, completed := range completes {
		if completed != slot {
			result = append(result, completed)
		}
	}
	return result
}

func userGate(name string, slot SlotID) Step { return Step{Kind: StepUserGate, Name: name, Slot: slot} }
func hostGate(name string, slot SlotID) Step { return Step{Kind: StepHostGate, Name: name, Slot: slot} }
func incident(kind, route string) IncidentRoute {
	return IncidentRoute{Incident: kind, Skill: route, ReturnTo: "implementation"}
}

func unavailableIncident(kind string) IncidentRoute {
	return IncidentRoute{Incident: kind, ReturnTo: "implementation"}
}

func requirement(route string, slot SlotID) Requirement {
	return Requirement{Route: route, Slot: slot}
}

func stableBoundaries() []string {
	return []string{"specification-approved", "tdd-cycle-complete", "debugging-cycle-complete", "review-complete", "verification-complete"}
}

func validate(profiles []Profile) error {
	wantIDs := []ProfileID{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"}
	if len(profiles) != len(wantIDs) {
		return errors.New("POLICY_CATALOG_INVALID: expected four built-in Profiles")
	}
	for index, profile := range profiles {
		if profile.ID != wantIDs[index] || len(profile.Steps) == 0 {
			return errors.New("POLICY_CATALOG_INVALID: built-in Profile order or content is invalid")
		}
		completed := map[SlotID]bool{}
		hasReviewOutcome := false
		for _, step := range profile.Steps {
			if step.Name == "" || step.Slot == "" || !validStepKind(step.Kind) {
				return errors.New("POLICY_CATALOG_INVALID: Profile step is invalid")
			}
			for _, slot := range step.Completes {
				completed[slot] = true
			}
			hasReviewOutcome = hasReviewOutcome || step.ReviewOutcome
		}
		if !hasReviewOutcome {
			return errors.New("POLICY_CATALOG_INVALID: Profile has no typed review outcome")
		}
		for _, slot := range requiredSlots() {
			if !completed[slot] {
				return errors.New("POLICY_CATALOG_INVALID: mandatory lifecycle slot is not completed")
			}
		}
		for _, route := range profile.Incidents {
			if route.Incident == "" || route.ReturnTo == "" {
				return errors.New("POLICY_CATALOG_INVALID: incident route is invalid")
			}
		}
		seenRequirements := map[Requirement]bool{}
		for _, requirement := range profile.Requirements {
			if requirement.Route == "" || !validRequiredSlot(requirement.Slot) || seenRequirements[requirement] {
				return errors.New("POLICY_CATALOG_INVALID: Profile requirement is invalid")
			}
			seenRequirements[requirement] = true
		}
	}
	return nil
}

func validStepKind(kind StepKind) bool {
	return kind == StepSkill || kind == StepHostAction || kind == StepUserGate || kind == StepHostGate
}

func requiredSlots() []SlotID {
	return []SlotID{"problem-framing", "solution-specification", "delivery-planning", "workspace-preparation", "implementation", "implementation-tdd", "review-remediation", "fresh-verification", "closeout"}
}

func validRequiredSlot(wanted SlotID) bool {
	for _, slot := range requiredSlots() {
		if slot == wanted {
			return true
		}
	}
	return false
}

func cloneProfiles(values []Profile) []Profile {
	result := make([]Profile, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Steps = make([]Step, len(value.Steps))
		for stepIndex, step := range value.Steps {
			result[index].Steps[stepIndex] = step
			result[index].Steps[stepIndex].Covers = append([]SlotID(nil), step.Covers...)
			result[index].Steps[stepIndex].Completes = append([]SlotID(nil), step.Completes...)
		}
		result[index].Incidents = append([]IncidentRoute(nil), value.Incidents...)
		result[index].Requirements = append([]Requirement(nil), value.Requirements...)
		result[index].StableBoundaries = append([]string(nil), value.StableBoundaries...)
	}
	sort.SliceStable(result, func(left, right int) bool {
		return profileOrder(result[left].ID) < profileOrder(result[right].ID)
	})
	return result
}

func profileOrder(id ProfileID) int {
	switch id {
	case "SP-FULL":
		return 0
	case "MATT-FULL":
		return 1
	case "ECC-FULL":
		return 2
	case "MATT-SP-HYBRID":
		return 3
	default:
		return 4
	}
}
