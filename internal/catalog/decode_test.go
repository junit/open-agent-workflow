package catalog

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func TestDecodeProviderV3RequiresTopologySets(t *testing.T) {
	validCapability := `{"id":"implementation","input_schema":"oaw.capability-input/v1","outcome_schema":"oaw.capability-outcome/v1","maximum_effects":["read-project","write-project"],"resources":["project-worktree"],"request_modes":["WORKFLOW"],"responsibilities":["implementation"],"supported_topologies":["CURRENT","SUBAGENT"],"delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"acme:implementation","topologies":["CURRENT","SUBAGENT"]}]}`
	provider := func(capability string) []byte {
		return []byte(fmt.Sprintf(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"3.0.0","id":"acme/suite","display_name":"Acme Suite","discovery":[{"id":"codex","hosts":["codex"],"surface":"codex-skills","distribution":"acme","kind":"path-exists","root":"user-home","candidate_path":".agents/skills/acme","evidence_path":"implementation/SKILL.md"}],"capabilities":[%s]}`, capability))
	}

	decoded, err := DecodeProvider(provider(validCapability))
	if err != nil {
		t.Fatalf("DecodeProvider(v3) error = %v", err)
	}
	want := []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	if !slices.Equal(decoded.Capabilities[0].SupportedTopologies, want) || !slices.Equal(decoded.Capabilities[0].HostBindings[0].Topologies, want) {
		t.Fatalf("decoded topology sets = %#v", decoded.Capabilities[0])
	}

	tests := []struct {
		name       string
		capability string
	}{
		{name: "singular topology", capability: strings.Replace(validCapability, `"supported_topologies":["CURRENT","SUBAGENT"]`, `"executor_topology":"isolated-required"`, 1)},
		{name: "empty capability set", capability: strings.Replace(validCapability, `"supported_topologies":["CURRENT","SUBAGENT"]`, `"supported_topologies":[]`, 1)},
		{name: "empty binding set", capability: strings.Replace(validCapability, `"topologies":["CURRENT","SUBAGENT"]`, `"topologies":[]`, 1)},
		{name: "binding outside capability", capability: strings.Replace(strings.Replace(validCapability, `"supported_topologies":["CURRENT","SUBAGENT"]`, `"supported_topologies":["CURRENT"]`, 1), `"topologies":["CURRENT","SUBAGENT"]`, `"topologies":["SUBAGENT"]`, 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertDecodeError(t, DecodeProvider, provider(test.capability), "INVALID_PROVIDER_DESCRIPTOR")
		})
	}
}

func TestDecodeProviderAndRecipeRejectReplacedVersions(t *testing.T) {
	v2Provider := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]}`)
	assertDecodeError(t, DecodeProvider, v2Provider, "UNSUPPORTED_PROVIDER_SCHEMA")

	v1Recipe := []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[]}`)
	assertDecodeError(t, DecodeRecipe, v1Recipe, "UNSUPPORTED_RECIPE_SCHEMA")

	v2Recipe := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"2.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]}`)
	if _, err := DecodeRecipe(v2Recipe); err != nil {
		t.Fatalf("DecodeRecipe(v2) error = %v", err)
	}
	missingRequirements := []byte(strings.Replace(string(v2Recipe), `,"environment_requirements":[]`, "", 1))
	assertDecodeError(t, DecodeRecipe, missingRequirements, "INVALID_PROFILE_RECIPE")
}

func TestBuiltinBindingsRemainHostScoped(t *testing.T) {
	tests := []struct {
		path  string
		hosts []string
	}{
		{path: "providers/oaw-superpowers.json", hosts: []string{"claude", "codex"}},
		{path: "providers/oaw-ecc.json", hosts: []string{"claude", "codex"}},
		{path: "providers/oaw-matt.json", hosts: []string{"codex"}},
	}
	wantTopologies := []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			raw, err := fs.ReadFile(assets.FS(), test.path)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := DecodeProvider(raw)
			if err != nil {
				t.Fatalf("DecodeProvider() error = %v", err)
			}
			for _, capability := range provider.Capabilities {
				if !slices.Equal(capability.SupportedTopologies, wantTopologies) {
					t.Fatalf("%s/%s topologies = %#v", provider.ID, capability.ID, capability.SupportedTopologies)
				}
				hosts := make([]string, 0, len(capability.HostBindings))
				for _, binding := range capability.HostBindings {
					hosts = append(hosts, binding.Host)
					if !slices.Equal(binding.Topologies, wantTopologies) {
						t.Fatalf("%s/%s %s binding topologies = %#v", provider.ID, capability.ID, binding.Host, binding.Topologies)
					}
				}
				slices.Sort(hosts)
				if !slices.Equal(hosts, test.hosts) {
					t.Fatalf("%s/%s binding hosts = %#v, want %#v", provider.ID, capability.ID, hosts, test.hosts)
				}
			}
		})
	}
}

func TestDecodeProviderAcceptsMinimalRecord(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]}`)
	got, err := DecodeProvider(raw)
	if err != nil {
		t.Fatalf("DecodeProvider() error = %v", err)
	}
	if got.ID != "oaw/test" || got.DescriptorVersion != "2.0.0" {
		t.Fatalf("DecodeProvider() = %#v", got)
	}
}

func TestDecodeRecipeAcceptsMinimalRecord(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]}`)
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
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[],"extra":true}`)
	assertDecodeError(t, DecodeProvider, raw, "INVALID_PROVIDER_DESCRIPTOR")
}

func TestDecodeRecipeRejectsTrailingJSON(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]} {}`)
	assertDecodeError(t, DecodeRecipe, raw, "INVALID_PROFILE_RECIPE")
}

func TestDecodeRecipeRejectsUnsupportedSchema(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v1","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[]}`)
	assertDecodeError(t, DecodeRecipe, raw, "UNSUPPORTED_RECIPE_SCHEMA")
}

func TestDecodeProviderRejectsInvalidEnum(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":"test","evidence_path":"SKILL.md"}],"capabilities":[{"id":"implementation","input_schema":"oaw.capability-input/v1","outcome_schema":"oaw.capability-outcome/v1","maximum_effects":[],"resources":[],"request_modes":["INVALID"],"responsibilities":[],"supported_topologies":["CURRENT","SUBAGENT"],"delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"test","topologies":["CURRENT","SUBAGENT"]}]}]}`)
	assertDecodeError(t, DecodeProvider, raw, "INVALID_PROVIDER_DESCRIPTOR")
}

func TestDecodeRecipeRejectsDuplicateSetMember(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":["implementation","implementation"],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]}`)
	assertDecodeError(t, DecodeRecipe, raw, "DUPLICATE_RECIPE_RESPONSIBILITY")
}

func TestDecodeAliasSetRejectsInvalidID(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-alias-set/v1","aliases":[{"alias":"SP-FULL","recipe_id":"bad"}]}`)
	assertDecodeError(t, DecodeAliasSet, raw, "INVALID_PROFILE_ALIAS_SET")
}

func TestDecodersRejectMissingRequiredFields(t *testing.T) {
	assertDecodeError(t, DecodeProvider, []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[]}`), "INVALID_PROVIDER_DESCRIPTOR")
	assertDecodeError(t, DecodeRecipe, []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[]}`), "INVALID_PROFILE_RECIPE")
	assertDecodeError(t, DecodeAliasSet, []byte(`{"schema_version":"oaw.profile-alias-set/v1","aliases":[]}`), "INVALID_PROFILE_ALIAS_SET")
}

func TestDecodeProviderReturnsIndependentSlices(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":".agents/skills/test","evidence_path":"SKILL.md"}],"capabilities":[]}`)
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
	shape := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":"ok","evidence_path":"SKILL.md","prefix":"extra"}],"capabilities":[]}`)
	assertDecodeError(t, DecodeProvider, shape, "DISCOVERY_PROBE_SHAPE_INVALID")
	unsafe := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":".agents/../secret","evidence_path":"SKILL.md"}],"capabilities":[]}`)
	assertDecodeError(t, DecodeProvider, unsafe, "DISCOVERY_PATH_INVALID")
}

func TestDecodeRecipeRejectsInvalidProcedure(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[{"id":"proc","kind":"procedure","responsibility":"tdd","selector":{"provider_id":"oaw/test","capability_id":"tdd"},"phase":"phase","transitions":[{"signal":"succeeded","target":"next"}]}],"incident_routes":[],"entry":"proc","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]}`)
	assertDecodeError(t, DecodeRecipe, raw, "INVALID_PROFILE_RECIPE")
}

func TestDecodeProviderValidatesProbeKindsAndCapabilityContract(t *testing.T) {
	base := `{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":%s,"capabilities":%s}`
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
	capability := `[{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":["read-project"],"resources":["project"],"request_modes":["WORKFLOW"],"responsibilities":["implementation"],"supported_topologies":["CURRENT","SUBAGENT"],"delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl","topologies":["CURRENT","SUBAGENT"]}]}]`
	if _, err := DecodeProvider([]byte(fmt.Sprintf(base, probes[0], capability))); err != nil {
		t.Fatalf("valid capability rejected: %v", err)
	}
}

func TestDecodeProviderRejectsDuplicateMembersAndInvalidBindings(t *testing.T) {
	base := `{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"2.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":"test","evidence_path":"SKILL.md"}],"capabilities":[%s]}`
	duplicates := []string{
		`{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":["read-project","read-project"],"resources":[],"request_modes":["WORKFLOW"],"responsibilities":[],"supported_topologies":["CURRENT","SUBAGENT"],"delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl","topologies":["CURRENT","SUBAGENT"]}]}`,
		`{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":[],"resources":[],"request_modes":["WORKFLOW","WORKFLOW"],"responsibilities":[],"supported_topologies":["CURRENT","SUBAGENT"],"delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl","topologies":["CURRENT","SUBAGENT"]}]}`,
		`{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":[],"resources":[],"request_modes":["WORKFLOW"],"responsibilities":[],"supported_topologies":["CURRENT","SUBAGENT"],"delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"skill","reference":"impl","topologies":["CURRENT","SUBAGENT"]},{"host":"codex","kind":"skill","reference":"impl","topologies":["CURRENT","SUBAGENT"]}]}`,
	}
	for _, value := range duplicates {
		assertDecodeError(t, DecodeProvider, []byte(fmt.Sprintf(base, value)), "DUPLICATE")
	}
	invalid := `{"id":"impl","input_schema":"in","outcome_schema":"out","maximum_effects":[],"resources":[],"request_modes":["WORKFLOW"],"responsibilities":[],"supported_topologies":["CURRENT","SUBAGENT"],"delegation_allow_list":[],"host_bindings":[{"host":"codex","kind":"invalid","reference":"impl","topologies":["CURRENT","SUBAGENT"]}]}`
	assertDecodeError(t, DecodeProvider, []byte(fmt.Sprintf(base, invalid)), "INVALID_PROVIDER_DESCRIPTOR")
}

func TestDecodeRecipeValidatesNodesRoutesAndCopiesNestedSlices(t *testing.T) {
	raw := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":["implementation"],"nodes":[{"id":"phase","kind":"phase","responsibility":"implementation","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[{"signal":"succeeded","target":"gate"}]},{"id":"gate","kind":"gate","responsibility":"completion","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}],"incident_routes":[{"incident":"build-failure","handler":"gate"}],"entry":"phase","terminal_gates":["gate"],"stable_boundaries":["complete"],"environment_requirements":[]}`)
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
	base := `{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[%s],"incident_routes":[],"entry":"x","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]}`
	values := []string{
		`{"id":"bad","kind":"unknown","responsibility":"x","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}`,
		`{"id":"bad","kind":"phase","responsibility":"","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}`,
		`{"id":"bad","kind":"procedure","responsibility":"x","selector":{"provider_id":"oaw/test","capability_id":"impl"},"transitions":[]}`,
		`{"id":"bad","kind":"phase","responsibility":"x","selector":{"provider_id":"bad","capability_id":"impl"},"transitions":[]}`,
	}
	for _, value := range values {
		assertDecodeError(t, DecodeRecipe, []byte(fmt.Sprintf(base, value)), "INVALID_PROFILE_RECIPE")
	}
	route := `{"schema_version":"oaw.profile-recipe/v2","recipe_version":"1.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[{"incident":"unknown","handler":"x"}],"entry":"x","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]}`
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
