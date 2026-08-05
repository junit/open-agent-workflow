package execution

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func NormalizeTopologies(values []Topology) ([]Topology, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("EXECUTION_TOPOLOGY_INVALID: topology set is empty")
	}

	result := append([]Topology{}, values...)
	slices.Sort(result)
	for index, value := range result {
		if !validTopology(value) {
			return nil, fmt.Errorf("EXECUTION_TOPOLOGY_INVALID: unknown topology %q", value)
		}
		if index > 0 && result[index-1] == value {
			return nil, fmt.Errorf("EXECUTION_TOPOLOGY_INVALID: duplicate topology %q", value)
		}
	}
	return result, nil
}

func IntersectTopologies(sets ...[]Topology) ([]Topology, error) {
	if len(sets) == 0 {
		return nil, fmt.Errorf("EXECUTION_TOPOLOGY_INVALID: no topology sets supplied")
	}

	intersection, err := NormalizeTopologies(sets[0])
	if err != nil {
		return nil, err
	}
	for _, set := range sets[1:] {
		normalized, err := NormalizeTopologies(set)
		if err != nil {
			return nil, err
		}
		intersection = intersectSorted(intersection, normalized)
	}
	return append([]Topology{}, intersection...), nil
}

func NormalizeRequirements(values []EnvironmentRequirement) ([]EnvironmentRequirement, error) {
	result := make([]EnvironmentRequirement, len(values))
	for index, value := range values {
		if value.Surface == "" || strings.TrimSpace(value.Surface) != value.Surface {
			return nil, fmt.Errorf("ENVIRONMENT_REQUIREMENT_INVALID: invalid surface %q", value.Surface)
		}
		accepted := append([]EnvironmentDisposition{}, value.AcceptedDispositions...)
		slices.Sort(accepted)
		for dispositionIndex, disposition := range accepted {
			if !validDisposition(disposition) {
				return nil, fmt.Errorf("ENVIRONMENT_REQUIREMENT_INVALID: unknown disposition %q", disposition)
			}
			if dispositionIndex > 0 && accepted[dispositionIndex-1] == disposition {
				return nil, fmt.Errorf("ENVIRONMENT_REQUIREMENT_INVALID: duplicate disposition %q", disposition)
			}
		}
		if value.Required && len(accepted) == 0 {
			return nil, fmt.Errorf("ENVIRONMENT_REQUIREMENT_INVALID: required surface %q has no accepted disposition", value.Surface)
		}
		result[index] = EnvironmentRequirement{
			Surface:              value.Surface,
			Required:             value.Required,
			AcceptedDispositions: accepted,
		}
	}

	slices.SortFunc(result, func(left, right EnvironmentRequirement) int {
		return strings.Compare(left.Surface, right.Surface)
	})
	for index := 1; index < len(result); index++ {
		if result[index-1].Surface == result[index].Surface {
			return nil, fmt.Errorf("ENVIRONMENT_REQUIREMENT_INVALID: duplicate surface %q", result[index].Surface)
		}
	}
	return result, nil
}

func RequirementsSatisfied(requirements []EnvironmentRequirement, observations []EnvironmentObservation) error {
	normalized, err := NormalizeRequirements(requirements)
	if err != nil {
		return err
	}

	observed := make(map[string]EnvironmentDisposition, len(observations))
	for _, observation := range observations {
		if observation.Surface == "" || strings.TrimSpace(observation.Surface) != observation.Surface {
			return fmt.Errorf("ENVIRONMENT_REQUIREMENT_UNSATISFIED: invalid observed surface %q", observation.Surface)
		}
		if !validDisposition(observation.Disposition) {
			return fmt.Errorf("ENVIRONMENT_REQUIREMENT_UNSATISFIED: unknown observed disposition %q", observation.Disposition)
		}
		if observation.Source == "" || strings.TrimSpace(observation.Source) != observation.Source {
			return fmt.Errorf("ENVIRONMENT_REQUIREMENT_UNSATISFIED: invalid observation source for %q", observation.Surface)
		}
		if !digestPattern.MatchString(observation.Digest) {
			return fmt.Errorf("ENVIRONMENT_REQUIREMENT_UNSATISFIED: invalid observation digest for %q", observation.Surface)
		}
		if _, duplicate := observed[observation.Surface]; duplicate {
			return fmt.Errorf("ENVIRONMENT_REQUIREMENT_UNSATISFIED: duplicate observed surface %q", observation.Surface)
		}
		observed[observation.Surface] = observation.Disposition
	}

	for _, requirement := range normalized {
		disposition, found := observed[requirement.Surface]
		if !found {
			if requirement.Required {
				return fmt.Errorf("ENVIRONMENT_REQUIREMENT_UNSATISFIED: required surface %q is not observed", requirement.Surface)
			}
			continue
		}
		if !slices.Contains(requirement.AcceptedDispositions, disposition) {
			return fmt.Errorf("ENVIRONMENT_REQUIREMENT_UNSATISFIED: surface %q has unaccepted disposition %q", requirement.Surface, disposition)
		}
	}
	return nil
}

func intersectSorted(left, right []Topology) []Topology {
	result := make([]Topology, 0, min(len(left), len(right)))
	leftIndex, rightIndex := 0, 0
	for leftIndex < len(left) && rightIndex < len(right) {
		switch {
		case left[leftIndex] < right[rightIndex]:
			leftIndex++
		case left[leftIndex] > right[rightIndex]:
			rightIndex++
		default:
			result = append(result, left[leftIndex])
			leftIndex++
			rightIndex++
		}
	}
	return result
}

func validTopology(value Topology) bool {
	return value == TopologyCurrent || value == TopologySubagent
}

func validDisposition(value EnvironmentDisposition) bool {
	switch value {
	case DispositionInherited, DispositionHostConfigured, DispositionRestricted, DispositionUnknown, DispositionUnavailable:
		return true
	default:
		return false
	}
}
