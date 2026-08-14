package policyflow

import "github.com/wifibaby4u/open-agent-workflow/internal/policycatalog"

type stepKind uint8

const (
	stepSkill stepKind = iota
	stepHostAction
	stepUserGate
	stepHostGate
	stepAny        stepKind = 254
	stepExecutable stepKind = 255
)

type programStep struct {
	kind          stepKind
	name          string
	slot          LifecycleSlot
	covers        []LifecycleSlot
	completes     []LifecycleSlot
	reviewOutcome bool
}

type profileProgram struct {
	id               ProfileID
	steps            []programStep
	requires         []programRequirement
	incidents        []programIncidentRoute
	stableBoundaries []string
}

type programRequirement struct {
	route string
	slot  LifecycleSlot
}

type programIncidentRoute struct {
	incident IncidentType
	route    string
	returnTo LifecycleSlot
}

func loadBuiltInPrograms() ([]profileProgram, error) {
	profiles, err := policycatalog.Load()
	if err != nil {
		return nil, fail(FailureSemanticsInvalid, err.Error())
	}
	result := make([]profileProgram, len(profiles))
	for index, profile := range profiles {
		program := profileProgram{
			id:               ProfileID(profile.ID),
			steps:            make([]programStep, len(profile.Steps)),
			requires:         make([]programRequirement, len(profile.Requirements)),
			incidents:        make([]programIncidentRoute, len(profile.Incidents)),
			stableBoundaries: append([]string(nil), profile.StableBoundaries...),
		}
		for stepIndex, step := range profile.Steps {
			kind, ok := policyStepKind(step.Kind)
			if !ok {
				return nil, fail(FailureSemanticsInvalid, "Policy Profile contains an unsupported step kind")
			}
			program.steps[stepIndex] = programStep{
				kind: kind, name: step.Name, slot: LifecycleSlot(step.Slot),
				covers: policySlots(step.Covers), completes: policySlots(step.Completes),
				reviewOutcome: step.ReviewOutcome,
			}
		}
		for requirementIndex, requirement := range profile.Requirements {
			program.requires[requirementIndex] = programRequirement{
				route: requirement.Route,
				slot:  LifecycleSlot(requirement.Slot),
			}
		}
		for incidentIndex, incident := range profile.Incidents {
			program.incidents[incidentIndex] = programIncidentRoute{
				incident: IncidentType(incident.Incident), route: incident.Skill,
				returnTo: LifecycleSlot(incident.ReturnTo),
			}
		}
		result[index] = program
	}
	return result, nil
}

func policyStepKind(kind policycatalog.StepKind) (stepKind, bool) {
	switch kind {
	case policycatalog.StepSkill:
		return stepSkill, true
	case policycatalog.StepHostAction:
		return stepHostAction, true
	case policycatalog.StepUserGate:
		return stepUserGate, true
	case policycatalog.StepHostGate:
		return stepHostGate, true
	default:
		return 0, false
	}
}

func policySlots(values []policycatalog.SlotID) []LifecycleSlot {
	result := make([]LifecycleSlot, len(values))
	for index, value := range values {
		result[index] = LifecycleSlot(value)
	}
	return result
}
