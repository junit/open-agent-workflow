package execution_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func TestNormalizeTopologiesAcceptsOnlyCurrentAndSubagent(t *testing.T) {
	t.Parallel()

	input := []execution.Topology{execution.TopologySubagent, execution.TopologyCurrent}
	got, err := execution.NormalizeTopologies(input)
	if err != nil {
		t.Fatalf("NormalizeTopologies() error = %v", err)
	}
	want := []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeTopologies() = %#v, want %#v", got, want)
	}

	input[0] = execution.TopologyCurrent
	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeTopologies() shares caller storage: %#v", got)
	}

	for _, values := range [][]execution.Topology{
		nil,
		{},
		{execution.TopologyCurrent, execution.TopologyCurrent},
		{"INLINE"},
		{"NATIVE_SUBAGENT"},
		{"main-agent-allowed"},
		{"isolated-required"},
		{"REMOTE"},
	} {
		values := values
		t.Run(strings.Join(topologyStrings(values), ","), func(t *testing.T) {
			t.Parallel()
			if _, err := execution.NormalizeTopologies(values); err == nil || !strings.HasPrefix(err.Error(), "EXECUTION_TOPOLOGY_INVALID:") {
				t.Fatalf("NormalizeTopologies(%#v) error = %v", values, err)
			}
		})
	}
}

func TestIntersectTopologiesIsDeterministic(t *testing.T) {
	t.Parallel()

	first := []execution.Topology{execution.TopologySubagent, execution.TopologyCurrent}
	second := []execution.Topology{execution.TopologyCurrent}
	third := []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	got, err := execution.IntersectTopologies(first, second, third)
	if err != nil {
		t.Fatalf("IntersectTopologies() error = %v", err)
	}
	want := []execution.Topology{execution.TopologyCurrent}
	if !slices.Equal(got, want) {
		t.Fatalf("IntersectTopologies() = %#v, want %#v", got, want)
	}

	first[0] = execution.TopologyCurrent
	got[0] = execution.TopologySubagent
	fresh, err := execution.IntersectTopologies(third, second, []execution.Topology{execution.TopologySubagent, execution.TopologyCurrent})
	if err != nil {
		t.Fatalf("IntersectTopologies() replay error = %v", err)
	}
	if !slices.Equal(fresh, want) {
		t.Fatalf("IntersectTopologies() is not deterministic or defensive: %#v", fresh)
	}

	empty, err := execution.IntersectTopologies(
		[]execution.Topology{execution.TopologyCurrent},
		[]execution.Topology{execution.TopologySubagent},
	)
	if err != nil {
		t.Fatalf("empty IntersectTopologies() error = %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty IntersectTopologies() = %#v, want non-nil empty slice", empty)
	}

	for _, sets := range [][][]execution.Topology{
		nil,
		{},
		{{}},
		{{execution.TopologyCurrent, execution.TopologyCurrent}},
		{{execution.TopologyCurrent}, {"INLINE"}},
	} {
		if _, err := execution.IntersectTopologies(sets...); err == nil || !strings.HasPrefix(err.Error(), "EXECUTION_TOPOLOGY_INVALID:") {
			t.Fatalf("IntersectTopologies(%#v) error = %v", sets, err)
		}
	}
}

func TestExecutionRequirementsRejectDuplicateSurfaces(t *testing.T) {
	t.Parallel()

	requirements := []execution.EnvironmentRequirement{
		{
			Surface:              "skills",
			Required:             true,
			AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited, execution.DispositionHostConfigured},
		},
		{
			Surface:              "mcp",
			Required:             false,
			AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionHostConfigured},
		},
	}
	normalized, err := execution.NormalizeRequirements(requirements)
	if err != nil {
		t.Fatalf("NormalizeRequirements() error = %v", err)
	}
	if got, want := []string{normalized[0].Surface, normalized[1].Surface}, []string{"mcp", "skills"}; !slices.Equal(got, want) {
		t.Fatalf("normalized surfaces = %#v, want %#v", got, want)
	}
	requirements[0].AcceptedDispositions[0] = execution.DispositionUnavailable
	if normalized[1].AcceptedDispositions[0] != execution.DispositionHostConfigured {
		t.Fatalf("NormalizeRequirements() shares nested caller storage: %#v", normalized)
	}

	observations := []execution.EnvironmentObservation{
		{Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-session", Digest: strings.Repeat("a", 64)},
		{Surface: "mcp", Disposition: execution.DispositionHostConfigured, Source: "codex-session", Digest: strings.Repeat("b", 64)},
	}
	if err := execution.RequirementsSatisfied(normalized, observations); err != nil {
		t.Fatalf("RequirementsSatisfied() error = %v", err)
	}

	invalidRequirements := []struct {
		name   string
		values []execution.EnvironmentRequirement
	}{
		{name: "empty surface", values: []execution.EnvironmentRequirement{{Required: true, AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited}}}},
		{name: "duplicate surface", values: []execution.EnvironmentRequirement{{Surface: "skills"}, {Surface: "skills"}}},
		{name: "required without disposition", values: []execution.EnvironmentRequirement{{Surface: "skills", Required: true}}},
		{name: "unknown disposition", values: []execution.EnvironmentRequirement{{Surface: "skills", AcceptedDispositions: []execution.EnvironmentDisposition{"copied"}}}},
		{name: "duplicate disposition", values: []execution.EnvironmentRequirement{{Surface: "skills", AcceptedDispositions: []execution.EnvironmentDisposition{execution.DispositionInherited, execution.DispositionInherited}}}},
	}
	for _, test := range invalidRequirements {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := execution.NormalizeRequirements(test.values); err == nil || !strings.HasPrefix(err.Error(), "ENVIRONMENT_REQUIREMENT_INVALID:") {
				t.Fatalf("NormalizeRequirements(%#v) error = %v", test.values, err)
			}
		})
	}

	unsatisfied := []struct {
		name         string
		observations []execution.EnvironmentObservation
	}{
		{name: "missing required", observations: []execution.EnvironmentObservation{{Surface: "mcp", Disposition: execution.DispositionHostConfigured, Source: "codex-session", Digest: strings.Repeat("b", 64)}}},
		{name: "unaccepted required", observations: []execution.EnvironmentObservation{{Surface: "skills", Disposition: execution.DispositionRestricted, Source: "codex-session", Digest: strings.Repeat("a", 64)}}},
		{name: "duplicate surface", observations: append(observations, observations[0])},
		{name: "unknown disposition", observations: []execution.EnvironmentObservation{{Surface: "skills", Disposition: "copied", Source: "codex-session", Digest: strings.Repeat("a", 64)}}},
	}
	for _, test := range unsatisfied {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := execution.RequirementsSatisfied(normalized, test.observations); err == nil || !strings.HasPrefix(err.Error(), "ENVIRONMENT_REQUIREMENT_UNSATISFIED:") {
				t.Fatalf("RequirementsSatisfied(%#v) error = %v", test.observations, err)
			}
		})
	}
}

func topologyStrings(values []execution.Topology) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}
