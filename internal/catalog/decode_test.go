package catalog

import (
	"fmt"
	"strings"
	"testing"
)

func TestDecodeProviderAcceptsMinimalRecord(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]}`)
	got, err := DecodeProvider(raw)
	if err != nil {
		t.Fatalf("DecodeProvider() error = %v", err)
	}
	if got.ID != "oaw/test" || got.DescriptorVersion != "2.0.0" {
		t.Fatalf("DecodeProvider() = %#v", got)
	}
}

func TestDecodeRecipeAcceptsMinimalRecord(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[]}`)
	got, err := DecodeRecipe(raw)
	if err != nil {
		t.Fatalf("DecodeRecipe() error = %v", err)
	}
	if got.ID != "oaw/test" || got.RecipeVersion != "1.0.0" {
		t.Fatalf("DecodeRecipe() = %#v", got)
	}
}

func TestDecodeAliasSetAcceptsMinimalRecord(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"oaw/test"}]}`)
	got, err := DecodeAliasSet(raw)
	if err != nil {
		t.Fatalf("DecodeAliasSet() error = %v", err)
	}
	if len(got.Aliases) != 1 || got.Aliases[0].Alias != "SP-FULL" {
		t.Fatalf("DecodeAliasSet() = %#v", got)
	}
}

func TestDecodeProviderRejectsUnknownField(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[],"extra":true}`)
	assertDecodeError(t, DecodeProvider, raw, "INVALID_PROVIDER_DESCRIPTOR")
}

func TestDecodeRecipeRejectsTrailingJSON(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[]} {}`)
	assertDecodeError(t, DecodeRecipe, raw, "INVALID_PROFILE_RECIPE")
}

func TestDecodeRecipeRejectsUnsupportedSchema(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[]}`)
	assertDecodeError(t, DecodeRecipe, raw, "UNSUPPORTED_RECIPE_SCHEMA")
}

func TestDecodeProviderRejectsInvalidEnum(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":"test","evidence_path":"SKILL.md"}],"capabilities":[{"id":"implementation","input_schema":"oaw.capability-input/v1","outcome_schema":"oaw.capability-outcome/v1","maximum_effects":[],"resources":[],"request_modes":["INVALID"],"responsibilities":[],"executor_topology":"isolated-required","delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"test"}]}]}`)
	assertDecodeError(t, DecodeProvider, raw, "INVALID_PROVIDER_DESCRIPTOR")
}

func TestDecodeRecipeRejectsDuplicateSetMember(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":["implementation","implementation"],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[]}`)
	assertDecodeError(t, DecodeRecipe, raw, "DUPLICATE_RECIPE_RESPONSIBILITY")
}

func TestDecodeAliasSetRejectsInvalidID(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"bad"}]}`)
	assertDecodeError(t, DecodeAliasSet, raw, "INVALID_PROFILE_ALIAS_SET")
}

func TestDecodersRejectMissingRequiredFields(t *testing.T) {
	assertDecodeError(t, DecodeProvider, []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[]}`), "INVALID_PROVIDER_DESCRIPTOR")
	assertDecodeError(t, DecodeRecipe, []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[]}`), "INVALID_PROFILE_RECIPE")
	assertDecodeError(t, DecodeAliasSet, []byte(`{"schema_version":"oaw.profile-alias-set/v1","aliases":[]}`), "INVALID_PROFILE_ALIAS_SET")
}

func TestDecodeProviderReturnsIndependentSlices(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":".agents/skills/test","evidence_path":"SKILL.md"}],"capabilities":[]}`)
	first, err := DecodeProvider(raw)
	if err != nil {
		t.Fatalf("first DecodeProvider() error = %v", err)
	}
	second, err := DecodeProvider(raw)
	if err != nil {
		t.Fatalf("second DecodeProvider() error = %v", err)
	}
	first.Discovery[0].Hosts[0] = "changed"
	if second.Discovery[0].Hosts[0] == "changed" {
		t.Fatal("DecodeProvider() reused mutable slices")
	}
}

func TestDecodeProviderRejectsProbeShapeAndUnsafePath(t *testing.T) {
	shape := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":"ok","evidence_path":"SKILL.md","prefix":"extra"}],"capabilities":[]}`)
	assertDecodeError(t, DecodeProvider, shape, "DISCOVERY_PROBE_SHAPE_INVALID")
	unsafe := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":".agents/../secret","evidence_path":"SKILL.md"}],"capabilities":[]}`)
	assertDecodeError(t, DecodeProvider, unsafe, "DISCOVERY_PATH_INVALID")
}

func TestDecodeRecipeRejectsInvalidProcedure(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[{"id":"proc","kind":"procedure","responsibility":"tdd","selector":{"provider_id":"oaw/test","capability_id":"tdd"},"phase":"phase","transitions":[{"signal":"succeeded","target":"next"}]}],"incident_routes":[],"entry":"proc","terminal_gates":[],"stable_boundaries":[]}`)
	assertDecodeError(t, DecodeRecipe, raw, "INVALID_PROFILE_RECIPE")
}

func TestDecodeProviderValidatesProbeKindsAndCapabilityContract(t *testing.T) {
	base := `{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":%s,"capabilities":%s}`
	probes := []string{
		`[{"id":"direct","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"project-root","candidate_path":"a","evidence_path":"SKILL.md"}]`,
		`[{"id":"version","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"one-level-version-path-exists","root":"xdg-config-home","prefix":"cache/tool","evidence_path":"SKILL.md"}]`,
		`[{"id":"bad","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"unknown","root":"user-home","candidate_path":"a","evidence_path":"SKILL.md"}]`,
		`[{"id":"badroot","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"unknown","candidate_path":"a","evidence_path":"SKILL.md"}]`,
	}
	for _, probes := range probes[:2] {
		if _, err := DecodeProvider([]byte(fmt.Sprintf(base, probes, "[]"))); err != nil {
			t.Fatalf("valid probe rejected: %v", err)
		}
	}
	assertDecodeError(t, DecodeProvider, []byte(fmt.Sprintf(base, probes[2], "[]")), "DISCOVERY_PROBE_SHAPE_INVALID")
	assertDecodeError(t, DecodeProvider, []byte(fmt.Sprintf(base, probes[3], "[]")), "INVALID_PROVIDER_DESCRIPTOR")
	capability := `[{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":["read-project"],"resources":["project"],"request_modes":["WORKFLOW"],"responsibilities":["implementation"],"executor_topology":"isolated-required","delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl"}]}]`
	if _, err := DecodeProvider([]byte(fmt.Sprintf(base, probes[0], capability))); err != nil {
		t.Fatalf("valid capability rejected: %v", err)
	}
}

func TestDecodeProviderRejectsDuplicateMembersAndInvalidBindings(t *testing.T) {
	base := `{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":"test","evidence_path":"SKILL.md"}],"capabilities":[%s]}`
	duplicates := []string{
		`{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":["read-project","read-project"],"resources":[],"request_modes":["WORKFLOW"],"responsibilities":[],"executor_topology":"isolated-required","delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl"}]}`,
		`{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":[],"resources":[],"request_modes":["WORKFLOW","WORKFLOW"],"responsibilities":[],"executor_topology":"isolated-required","delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl"}]}`,
		`{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":[],"resources":[],"request_modes":["WORKFLOW"],"responsibilities":[],"executor_topology":"isolated-required","delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl"},{"host":"codex","kind":"skill","reference":"impl"}]}`,
	}
	for _, value := range duplicates {
		assertDecodeError(t, DecodeProvider, []byte(fmt.Sprintf(base, value)), "DUPLICATE")
	}
	invalid := `{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":[],"resources":[],"request_modes":["WORKFLOW"],"responsibilities":[],"executor_topology":"isolated-required","delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"invalid","reference":"impl"}]}`
	assertDecodeError(t, DecodeProvider, []byte(fmt.Sprintf(base, invalid)), "INVALID_PROVIDER_DESCRIPTOR")
}

func TestDecodeRecipeValidatesNodesRoutesAndCopiesNestedSlices(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":["implementation"],"nodes":[{"id":"phase","kind":"phase","responsibility":"implementation","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[{"signal":"succeeded","target":"gate"}]},{"id":"gate","kind":"gate","responsibility":"completion","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}],"incident_routes":[{"incident":"build-failure","handler":"gate"}],"entry":"phase","terminal_gates":["gate"],"stable_boundaries":["complete"]}`)
	record, err := DecodeRecipe(raw)
	if err != nil {
		t.Fatalf("valid recipe rejected: %v", err)
	}
	record.Nodes[0].Transitions[0].Target = "changed"
	second, err := DecodeRecipe(raw)
	if err != nil || second.Nodes[0].Transitions[0].Target != "gate" {
		t.Fatalf("nested transition copy failed: %#v, %v", second, err)
	}
}

func TestDecodeRecipeRejectsInvalidNodeAndRouteValues(t *testing.T) {
	base := `{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[%s],"incident_routes":[],"entry":"x","terminal_gates":[],"stable_boundaries":[]}`
	values := []string{
		`{"id":"bad","kind":"unknown","responsibility":"x","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}`,
		`{"id":"bad","kind":"phase","responsibility":"","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}`,
		`{"id":"bad","kind":"procedure","responsibility":"x","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}`,
		`{"id":"bad","kind":"phase","responsibility":"x","selector":{"provider_id":"bad","capability_id":"impl"},"transitions":[]}`,
	}
	for _, value := range values {
		assertDecodeError(t, DecodeRecipe, []byte(fmt.Sprintf(base, value)), "INVALID_PROFILE_RECIPE")
	}
	route := `{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[{"incident":"unknown","handler":"x"}],"entry":"x","terminal_gates":[],"stable_boundaries":[]}`
	assertDecodeError(t, DecodeRecipe, []byte(route), "INVALID_PROFILE_RECIPE")
}

func assertDecodeError[T any](t *testing.T, decode func([]byte) (T, error), raw []byte, code string) {
	t.Helper()
	_, err := decode(raw)
	if err == nil {
		t.Fatalf("expected error containing %q", code)
	}
	if !strings.Contains(err.Error(), code) {
		t.Fatalf("error %q does not contain %q", err, code)
	}
}
