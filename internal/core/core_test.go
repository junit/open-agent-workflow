package core_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestClassifyMatchesDeterministicClassifier(t *testing.T) {
	want, err := classification.Classify(nil, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := core.Classify(nil, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest() != want.Digest() || got.RequestMode != want.RequestMode || got.RiskClass != want.RiskClass {
		t.Fatalf("Classify() = %#v digest=%s, want %#v digest=%s", got, got.Digest(), want, want.Digest())
	}
}

func TestResolvePinsHostScopedInputs(t *testing.T) {
	fixture := newCoreFixture(t, false)
	if fixture.resolution.Report.HostID() != "codex" || fixture.resolution.Registry.HostID() != "codex" || fixture.resolution.Digest == "" {
		t.Fatalf("Resolve() = %#v", fixture.resolution)
	}
	if _, found := fixture.resolution.Registry.Provider("oaw/superpowers"); !found {
		t.Fatal("verified Superpowers Provider is absent")
	}
	_, err := core.Resolve(core.ResolutionRequest{
		Configuration: fixture.request.Configuration,
		HostID:        "claude",
		Discovery:     fixture.discovery,
		Inventory:     &fixture.inventory,
	})
	requireCoreCode(t, err, "HOST_PROVIDER_SCOPE_MISMATCH")

	foreignInventory := fixture.inventory
	foreignInventory.HostID = "claude"
	_, err = core.Resolve(core.ResolutionRequest{
		Configuration: fixture.request.Configuration,
		HostID:        "codex",
		Discovery:     fixture.discovery,
		Inventory:     &foreignInventory,
	})
	requireCoreCode(t, err, "HOST_PROVIDER_SCOPE_MISMATCH")

	retiredInventory := fixture.inventory
	retiredInventory.SchemaVersion = "oaw.host-binding-inventory/v1"
	_, err = core.Resolve(core.ResolutionRequest{
		Configuration: fixture.request.Configuration,
		HostID:        "codex",
		Discovery:     fixture.discovery,
		Inventory:     &retiredInventory,
	})
	requireCoreCode(t, err, "HOST_BINDING_INVENTORY_INVALID")
}

func TestCompileReportsBuiltInAndUserDefinedEligibilityWithoutSelection(t *testing.T) {
	fixture := newCoreFixture(t, false)
	result, err := core.Compile(fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Bundle != nil || result.Digest == "" {
		t.Fatalf("Compile(no selection) = %#v", result)
	}
	profiles := make([]string, len(result.EligibleProfiles))
	recommended := ""
	for index, eligibility := range result.EligibleProfiles {
		profiles[index] = eligibility.Profile
		if !eligibility.Eligible || !slices.Equal(eligibility.EligibleTopologies, dualTopologies()) {
			t.Fatalf("Profile eligibility = %#v", eligibility)
		}
		if eligibility.Recommended {
			if recommended != "" {
				t.Fatalf("multiple recommendations: %q and %q", recommended, eligibility.Profile)
			}
			recommended = eligibility.Profile
		}
	}
	wantProfiles := []string{"ECC-FULL", "MATT-FULL", "MATT-SP-HYBRID", "SP-FULL", "acme/reliable-delivery"}
	if !slices.Equal(profiles, wantProfiles) || recommended != "MATT-SP-HYBRID" {
		t.Fatalf("profiles/recommendation = %#v / %q", profiles, recommended)
	}
	if len(result.EligibleAddOns) != 3 {
		t.Fatalf("EligibleAddOns = %#v", result.EligibleAddOns)
	}
}

func TestCompileCreatesImmutableLifecycleBundle(t *testing.T) {
	fixture := newCoreFixture(t, false)
	request := fixture.request
	request.EnvironmentObservations = []execution.EnvironmentObservation{{
		Surface: "skills", Disposition: execution.DispositionInherited,
		Source: "codex-session", Digest: strings.Repeat("b", 64),
	}}
	request.Selection = &core.Selection{
		Profile: "MATT-SP-HYBRID", ProfileSource: core.SelectionUser,
		Topology: execution.TopologyCurrent, TopologySource: core.SelectionUser,
		AddOns: []string{"build-repair"},
		Bindings: []profile.ProfileBinding{
			{Selector: catalog.CapabilitySelector{ProviderID: "oaw/superpowers", CapabilityID: "implementation"}, PreferredProviderID: "oaw/superpowers"},
			{Selector: catalog.CapabilitySelector{ProviderID: "oaw/matt", CapabilityID: "tdd"}, PreferredProviderID: "oaw/matt"},
		},
	}

	first, err := core.Compile(request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bundle == nil || first.Bundle.SchemaVersion != "oaw.lifecycle-bundle/v3" || first.Bundle.Topology != execution.TopologyCurrent || first.Bundle.Digest == "" || first.Bundle.ID == "" {
		t.Fatalf("Bundle = %#v", first.Bundle)
	}
	if !slices.Equal(first.Bundle.AddOns, []string{"build-repair"}) || first.Bundle.Graph.Digest == "" || first.Bundle.Configuration.Digest == "" {
		t.Fatalf("Bundle pins = %#v", first.Bundle)
	}
	wantBytes, err := canonicaljson.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	wantResultDigest := first.Digest
	wantBundleDigest := first.Bundle.Digest

	first.EligibleProfiles[0].EligibleTopologies[0] = execution.TopologySubagent
	first.EligibleAddOns[0].EligibleTopologies[0] = execution.TopologySubagent
	first.Bundle.Selection.AddOns[0] = "changed"
	first.Bundle.Selection.Bindings[0].PreferredProviderID = "changed"
	first.Bundle.ProviderInstances[0].InstanceDigest = "changed"
	first.Bundle.Graph.Nodes[0].Binding.Topologies[0] = execution.TopologySubagent
	first.Bundle.Graph.Nodes[0].SupportedTopologies[0] = execution.TopologySubagent
	first.Bundle.Graph.Nodes[0].MaximumEffects[0] = "changed"
	first.Bundle.Configuration.Settings[0].ProviderID = "changed"
	first.Bundle.EnvironmentObservations[0].Surface = "changed"
	first.Bundle.Classification.EscalationReasons[0] = "changed"
	*first.Bundle.Classification.WorkflowComplexity = classification.ComplexityOrdinary
	first.Bundle.Classification.EvidenceRequirements[0].Reason = "changed"
	first.Bundle.Graph.Bindings[0].PreferredProviderID = "changed"
	first.Bundle.Graph.ProviderInstances[0].InstanceDigest = "changed"
	first.Bundle.Graph.Nodes[0].Resources[0] = "changed"
	first.Bundle.Graph.Nodes[0].RequestModes[0] = catalog.RequestMode("changed")
	for index := range first.Bundle.Graph.Nodes {
		if len(first.Bundle.Graph.Nodes[index].DelegationAllowList) > 0 {
			first.Bundle.Graph.Nodes[index].DelegationAllowList[0] = "changed"
			break
		}
	}
	if len(first.Bundle.Graph.Nodes[0].Transitions) > 0 {
		first.Bundle.Graph.Nodes[0].Transitions[0].Target = "changed"
	}
	first.Bundle.Graph.IncidentRoutes[0].Handler = "changed"
	first.Bundle.Graph.TerminalGates[0] = "changed"
	first.Bundle.Graph.StableBoundaries[0] = "changed"
	first.Bundle.Graph.EligibleTopologies[0] = execution.TopologySubagent
	if len(first.Bundle.Graph.EnvironmentRequirements) > 0 && len(first.Bundle.Graph.EnvironmentRequirements[0].AcceptedDispositions) > 0 {
		first.Bundle.Graph.EnvironmentRequirements[0].AcceptedDispositions[0] = execution.DispositionUnavailable
	}
	if first.Bundle.Configuration.Settings[0].Pin != nil {
		first.Bundle.Configuration.Settings[0].Pin.Location = "changed"
	}
	if len(first.Bundle.Configuration.Settings[0].Preferences) > 0 {
		first.Bundle.Configuration.Settings[0].Preferences[0].Reference = "changed"
	}
	if len(first.Bundle.Configuration.Settings[0].CapabilityLimit) > 0 {
		first.Bundle.Configuration.Settings[0].CapabilityLimit[0] = "changed"
	}
	if len(first.Bundle.Configuration.ProviderInstallations) > 0 {
		first.Bundle.Configuration.ProviderInstallations[0].Location = "changed"
	}
	if len(first.Bundle.Configuration.BoundedCapabilityDefaults) > 0 {
		first.Bundle.Configuration.BoundedCapabilityDefaults[0].ID = "changed"
	}
	if len(first.Bundle.Configuration.RequiredProviders) > 0 {
		first.Bundle.Configuration.RequiredProviders[0] = "changed"
	}
	if len(first.Bundle.Configuration.RecommendedProviders) > 0 {
		first.Bundle.Configuration.RecommendedProviders[0] = "changed"
	}
	if len(first.Bundle.Configuration.HostIntegrations) > 0 {
		first.Bundle.Configuration.HostIntegrations[0].ID = "changed"
	}

	second, err := core.Compile(request)
	if err != nil {
		t.Fatal(err)
	}
	gotBytes, err := canonicaljson.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBytes) != string(wantBytes) || second.Digest != wantResultDigest || second.Bundle == nil || second.Bundle.Digest != wantBundleDigest {
		t.Fatalf("Compile() changed after returned-value mutation\nfirst=%s\nsecond=%s", wantBytes, gotBytes)
	}
}

func TestCompileEnforcesSelectionTopologyAddOnAndDigestContracts(t *testing.T) {
	fixture := newCoreFixture(t, false)

	t.Run("invalid profile", func(t *testing.T) {
		request := fixture.request
		request.Selection = workflowSelection("UNKNOWN", execution.TopologyCurrent, core.SelectionUser)
		_, err := core.Compile(request)
		requireCoreCode(t, err, "PROFILE_SELECTION_INVALID")
	})

	t.Run("retired topology", func(t *testing.T) {
		request := fixture.request
		request.Selection = workflowSelection("SP-FULL", execution.Topology("INLINE"), core.SelectionUser)
		_, err := core.Compile(request)
		requireCoreCode(t, err, "EXECUTION_TOPOLOGY_INVALID")
	})

	t.Run("add-on outside profile", func(t *testing.T) {
		request := fixture.request
		request.Selection = workflowSelection("SP-FULL", execution.TopologyCurrent, core.SelectionUser)
		request.Selection.AddOns = []string{"build-repair"}
		_, err := core.Compile(request)
		requireCoreCode(t, err, "PROFILE_ADD_ON_INVALID")
	})

	t.Run("current only provenance", func(t *testing.T) {
		request := fixture.request
		request.HostTopologies = []execution.Topology{execution.TopologyCurrent}
		request.Selection = workflowSelection("SP-FULL", execution.TopologyCurrent, core.SelectionHostOnlyOption)
		result, err := core.Compile(request)
		if err != nil || result.Bundle == nil {
			t.Fatalf("Compile(current only) = %#v, %v", result, err)
		}
		request.Selection = workflowSelection("SP-FULL", execution.TopologyCurrent, core.SelectionUser)
		_, err = core.Compile(request)
		requireCoreCode(t, err, "PROFILE_TOPOLOGY_SOURCE_INVALID")
	})

	t.Run("mutated classification", func(t *testing.T) {
		request := fixture.request
		request.Classification.RiskClass = classification.RiskNormal
		_, err := core.Compile(request)
		requireCoreCode(t, err, "CLASSIFICATION_DIGEST_INVALID")
	})
}

func TestCompileRejectsInvalidCoreInputsAndNormalizesBindings(t *testing.T) {
	fixture := newCoreFixture(t, false)

	t.Run("invalid input digest", func(t *testing.T) {
		request := fixture.request
		request.InputDigest = "not-a-digest"
		_, err := core.Compile(request)
		requireCoreCode(t, err, "CORE_INPUT_INVALID")
	})

	t.Run("empty Host topology set", func(t *testing.T) {
		request := fixture.request
		request.HostTopologies = []execution.Topology{}
		_, err := core.Compile(request)
		requireCoreCode(t, err, "EXECUTION_TOPOLOGY_INVALID")
	})

	t.Run("invalid environment observation", func(t *testing.T) {
		request := fixture.request
		request.EnvironmentObservations = []execution.EnvironmentObservation{{
			Surface: "skills", Disposition: execution.DispositionInherited, Source: "", Digest: strings.Repeat("a", 64),
		}}
		_, err := core.Compile(request)
		requireCoreCode(t, err, "ENVIRONMENT_REQUIREMENT_UNSATISFIED")
	})

	t.Run("explicit bindings", func(t *testing.T) {
		request := fixture.request
		request.Selection = workflowSelection("SP-FULL", execution.TopologyCurrent, core.SelectionUser)
		request.Selection.Bindings = []profile.ProfileBinding{
			{Selector: catalog.CapabilitySelector{ProviderID: "oaw/superpowers", CapabilityID: "review"}, PreferredProviderID: "oaw/superpowers"},
			{Selector: catalog.CapabilitySelector{ProviderID: "oaw/superpowers", CapabilityID: "implementation"}, PreferredProviderID: "oaw/superpowers"},
		}
		result, err := core.Compile(request)
		if err != nil || result.Bundle == nil {
			t.Fatalf("Compile(explicit bindings) = %#v, %v", result, err)
		}
		bindings := result.Bundle.Selection.Bindings
		if len(bindings) != 2 || bindings[0].Selector.CapabilityID != "implementation" || bindings[1].Selector.CapabilityID != "review" {
			t.Fatalf("normalized bindings = %#v", bindings)
		}
	})
}

func TestCompileReportsAmbiguousProviderWithoutSubstitution(t *testing.T) {
	fixture := newCoreFixture(t, true)
	result, err := core.Compile(fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	sp := profileEligibility(result.EligibleProfiles, "SP-FULL")
	if sp.Eligible || !hasDiagnostic(sp.Diagnostics, "PROVIDER_CANDIDATE_AMBIGUOUS") {
		t.Fatalf("SP-FULL eligibility = %#v", sp)
	}
	request := fixture.request
	request.Selection = workflowSelection("SP-FULL", execution.TopologyCurrent, core.SelectionUser)
	_, err = core.Compile(request)
	requireCoreCode(t, err, "PROFILE_SELECTION_INVALID")
}

func TestCompileIsDeterministicForEquivalentInputOrder(t *testing.T) {
	fixture := newCoreFixture(t, false)
	firstRequest := fixture.request
	firstRequest.EnvironmentObservations = []execution.EnvironmentObservation{
		{Surface: "mcp", Disposition: execution.DispositionHostConfigured, Source: "codex-session", Digest: strings.Repeat("c", 64)},
		{Surface: "skills", Disposition: execution.DispositionInherited, Source: "codex-session", Digest: strings.Repeat("d", 64)},
	}
	firstRequest.Selection = workflowSelection("MATT-SP-HYBRID", execution.TopologySubagent, core.SelectionUser)
	firstRequest.Selection.AddOns = []string{"type-repair", "build-repair"}
	first, err := core.Compile(firstRequest)
	if err != nil {
		t.Fatal(err)
	}

	secondRequest := fixture.request
	secondRequest.HostTopologies = []execution.Topology{execution.TopologySubagent, execution.TopologyCurrent}
	secondRequest.EnvironmentObservations = []execution.EnvironmentObservation{firstRequest.EnvironmentObservations[1], firstRequest.EnvironmentObservations[0]}
	secondRequest.Selection = workflowSelection("MATT-SP-HYBRID", execution.TopologySubagent, core.SelectionUser)
	secondRequest.Selection.AddOns = []string{"build-repair", "type-repair"}
	second, err := core.Compile(secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || first.Bundle == nil || second.Bundle == nil || first.Bundle.Digest != second.Bundle.Digest {
		t.Fatalf("equivalent digests differ: %s/%s != %s/%s", first.Digest, first.Bundle.Digest, second.Digest, second.Bundle.Digest)
	}
}

type coreFixture struct {
	request    core.CompilationRequest
	resolution core.ResolutionResult
	discovery  discovery.Report
	inventory  host.BindingInventory
}

func newCoreFixture(t *testing.T, ambiguousSuperpowers bool) coreFixture {
	t.Helper()
	userRoot := t.TempDir()
	writeCoreFile(t, userRoot, "config.toml", `schema_version = "oaw.user-config/v3"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
[[profile_recipes]]
id = "acme/reliable-delivery"
path = "recipes/reliable-delivery.toml"
`)
	writeCoreFile(t, userRoot, "providers/acme.toml", customProviderTOML)
	writeCoreFile(t, userRoot, "recipes/reliable-delivery.toml", customRecipeTOML)
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot})
	if err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	writeCoreFile(t, home, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "superpowers")
	if ambiguousSuperpowers {
		writeCoreFile(t, home, ".codex/plugins/cache/openai-api-curated/superpowers/1.0.0/skills/using-superpowers/SKILL.md", "superpowers-version")
	}
	for _, relative := range []string{"to-spec/SKILL.md", "to-tickets/SKILL.md", "tdd/SKILL.md", "diagnosing-bugs/SKILL.md"} {
		writeCoreFile(t, home, filepath.Join(".agents/skills", relative), relative)
	}
	writeCoreFile(t, home, ".agents/skills/everything-claude-code/SKILL.md", "ecc")
	writeCoreFile(t, home, ".agents/acme/SKILL.md", "acme")
	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := inventoryForCatalog(t, snapshot.Catalog(), discovered)
	resolved, err := core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: "codex", Discovery: discovered, Inventory: &inventory})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := core.Classify(nil, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	return coreFixture{
		resolution: resolved, discovery: discovered, inventory: inventory,
		request: core.CompilationRequest{
			DeliverableID: "core-contracts", InputDigest: strings.Repeat("1", 64), Generation: 1,
			Classification: decision, Configuration: snapshot, Resolutions: resolved.Report, Registry: resolved.Registry,
			HostID: "codex", HostSessionDigest: strings.Repeat("2", 64), HostEnvironmentReportDigest: strings.Repeat("3", 64), HostProviderInventoryDigest: inventory.Digest,
			HostTopologies: dualTopologies(), EnvironmentObservations: []execution.EnvironmentObservation{},
		},
	}
}

func inventoryForCatalog(t *testing.T, available catalog.Catalog, discovered discovery.Report) host.BindingInventory {
	t.Helper()
	seen := make(map[string]struct{})
	observations := []host.BindingObservation{}
	for _, provider := range available.Providers() {
		for _, candidate := range discovered.Candidates(provider.ID) {
			for _, capability := range provider.Capabilities {
				for _, binding := range capability.HostBindings {
					if binding.Host != "codex" {
						continue
					}
					key := candidate.InstallationKey + "\x00" + binding.Host + "\x00" + binding.Kind + "\x00" + binding.Reference
					if _, found := seen[key]; found {
						continue
					}
					seen[key] = struct{}{}
					observations = append(observations, host.BindingObservation{
						HostID: "codex", InstallationKey: candidate.InstallationKey, Binding: binding,
						Topologies: append([]execution.Topology{}, binding.Topologies...),
						Source:     "host-filesystem", EvidenceReference: filepath.Join(candidate.Location, fmt.Sprintf("inventory-%d", len(observations))),
						Digest: strings.Repeat("a", 64),
					})
				}
			}
		}
	}
	inventory, err := host.NewBindingInventory("codex", observations)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func workflowSelection(profileName string, topology execution.Topology, source core.SelectionSource) *core.Selection {
	return &core.Selection{Profile: profileName, ProfileSource: core.SelectionUser, Topology: topology, TopologySource: source, AddOns: []string{}, Bindings: []profile.ProfileBinding{}}
}

func profileEligibility(values []core.ProfileEligibility, profile string) core.ProfileEligibility {
	for _, value := range values {
		if value.Profile == profile {
			return value
		}
	}
	return core.ProfileEligibility{}
}

func hasDiagnostic(values []core.EligibilityDiagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}

func requireCoreCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !strings.HasPrefix(err.Error(), code) {
		t.Fatalf("error = %v, want prefix %s", err, code)
	}
}

func writeCoreFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func dualTopologies() []execution.Topology {
	return []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
}

const customProviderTOML = `
schema_version = "oaw.provider-descriptor/v3"
descriptor_version = "3.0.0"
id = "acme/suite"
display_name = "Acme Suite"

[[discovery]]
id = "acme-skill"
hosts = ["codex"]
surface = "codex-user-skills"
distribution = "acme"
kind = "path-exists"
root = "user-home"
candidate_path = ".agents/acme"
evidence_path = "SKILL.md"

[[capabilities]]
id = "review"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
maximum_effects = ["read-project"]
resources = ["project"]
request_modes = ["WORKFLOW"]
responsibilities = ["review"]
supported_topologies = ["CURRENT", "SUBAGENT"]
delegation_allow_list = []
[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:review"
topologies = ["CURRENT", "SUBAGENT"]

[[capabilities]]
id = "completion"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
maximum_effects = ["read-project"]
resources = ["project"]
request_modes = ["WORKFLOW"]
responsibilities = ["completion"]
supported_topologies = ["CURRENT", "SUBAGENT"]
delegation_allow_list = []
[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:completion"
topologies = ["CURRENT", "SUBAGENT"]
`

const customRecipeTOML = `
schema_version = "oaw.profile-recipe/v2"
recipe_version = "2.0.0"
id = "acme/reliable-delivery"
display_name = "Acme Reliable Delivery"
required_responsibilities = ["review", "completion"]
incident_routes = []
entry = "review"
terminal_gates = ["completion"]
stable_boundaries = ["review-complete"]
environment_requirements = []

[[nodes]]
id = "review"
kind = "phase"
responsibility = "review"
transitions = [{signal = "succeeded", target = "completion"}]
[nodes.selector]
provider_id = "acme/suite"
capability_id = "review"

[[nodes]]
id = "completion"
kind = "gate"
responsibility = "completion"
transitions = []
[nodes.selector]
provider_id = "acme/suite"
capability_id = "completion"
`
