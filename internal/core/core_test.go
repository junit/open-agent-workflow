package core_test

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
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

func TestCoreResolveAcceptsOnlyBindingInventoryV3(t *testing.T) {
	snapshot, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: "codex", Discovery: report, Inventory: &inventory})
	if err != nil || resolved.Report.HostID() != "codex" || resolved.Registry.HostID() != "codex" || resolved.Digest == "" {
		t.Fatalf("Resolve(v3) = %#v, %v", resolved, err)
	}

	retired := inventory
	retired.SchemaVersion = "oaw.host-binding-inventory/v2"
	_, err = core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: "codex", Discovery: report, Inventory: &retired})
	requireCoreCode(t, err, "HOST_BINDING_INVENTORY_INVALID")
}

func TestCompileBundleV4ForBuiltInAliasesAndTopologies(t *testing.T) {
	aliases := []string{"ECC-FULL", "MATT-FULL", "MATT-SP-HYBRID", "SP-FULL"}
	for _, topology := range []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent} {
		t.Run(string(topology), func(t *testing.T) {
			fixture := newCoreV4Fixture(t, coreFixtureOptions{hostID: "codex", topology: topology, complete: true})
			inspection := compileCore(t, fixture.request)
			if inspection.Bundle != nil || inspection.SelectionPreview != nil {
				t.Fatalf("inspection created a selection or Bundle: %#v", inspection)
			}
			for _, alias := range aliases {
				eligibility := requireEligibility(t, inspection, alias, "")
				if !eligibility.Eligible || eligibility.Preview.Selection.ConfirmationDigest == "" {
					t.Fatalf("%s eligibility = %#v", alias, eligibility)
				}
				selection := eligibility.Preview.Selection
				selection.ProfileSource = core.SelectionUser
				selection.TopologySource = core.SelectionUser
				compiled := compileSelection(t, fixture.request, selection)
				bundle := requireBundleV4(t, compiled)
				if bundle.Selection.Profile != alias || bundle.Selection.RecipeID != eligibility.RecipeID || bundle.Topology != topology {
					t.Fatalf("%s Bundle identity = %#v", alias, bundle)
				}
				if len(bundle.Selection.AddOns) != 0 || len(bundle.AddOns) != 0 {
					t.Fatalf("%s silently selected Add-ons: %#v", alias, bundle.Selection.AddOns)
				}
			}
		})
	}
}

func TestSelectionConfirmationCoversAddOnAlternativesAndOverlay(t *testing.T) {
	fixture := newCoreV4Fixture(t, coreFixtureOptions{hostID: "claude", topology: execution.TopologyCurrent, complete: true})
	recipe := recipeFor(t, fixture.request.Configuration.Catalog(), "oaw/reliable-feature")
	selection := core.Selection{
		Profile: "MATT-SP-HYBRID", RecipeID: recipe.ID, ProfileSource: core.SelectionUser,
		Topology: execution.TopologyCurrent, TopologySource: core.SelectionUser,
		AddOns: []string{"ecc-build-repair"}, Alternatives: alternativesForHost(t, fixture.request.Configuration.Catalog(), recipe, "claude"),
		Overlays: []string{"default-inline"},
	}
	previewResult := compileSelection(t, fixture.request, selection)
	if previewResult.Bundle != nil || previewResult.SelectionPreview == nil {
		t.Fatalf("selection preview = %#v", previewResult)
	}
	preview := *previewResult.SelectionPreview
	if preview.Graph == nil || preview.Selection.ConfirmationDigest == "" || len(preview.Diagnostics) != 0 {
		t.Fatalf("selection preview = %#v", preview)
	}
	if !slices.Equal(preview.Selection.AddOns, []string{"ecc-build-repair"}) || !slices.Equal(preview.Selection.Overlays, []string{"default-inline"}) || len(preview.Selection.Alternatives) == 0 {
		t.Fatalf("canonical selection = %#v", preview.Selection)
	}

	confirmed := compileSelection(t, fixture.request, preview.Selection)
	bundle := requireBundleV4(t, confirmed)
	if bundle.Selection.ConfirmationDigest != preview.Selection.ConfirmationDigest || bundle.Graph.Selection.Digest != bundle.Selection.GraphSelectionDigest {
		t.Fatalf("Bundle confirmation pins = %#v", bundle)
	}

	changed := preview.Selection
	changed.ConfirmationDigest = strings.Repeat("f", 64)
	request := fixture.request
	request.Selection = &changed
	_, err := core.Compile(request)
	requireCoreCode(t, err, "PROFILE_SELECTION_INVALID")

	missingPin := preview.Selection
	missingPin.GraphSelectionDigest = ""
	request.Selection = &missingPin
	_, err = core.Compile(request)
	requireCoreCode(t, err, "PROFILE_SELECTION_INVALID")
}

func TestUserDefinedUsesCompilerPathAndNeverImplicitlySelects(t *testing.T) {
	fixture := newCoreV4Fixture(t, coreFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent, complete: true, userRecipe: true})
	inspection := compileCore(t, fixture.request)
	if inspection.Bundle != nil || inspection.SelectionPreview != nil {
		t.Fatalf("USER-DEFINED inspection selected implicitly: %#v", inspection)
	}
	eligibility := requireEligibility(t, inspection, core.UserDefinedProfile, userRecipeID)
	if !eligibility.Eligible || eligibility.Preview.Selection.Profile != core.UserDefinedProfile || eligibility.Preview.Selection.RecipeID != userRecipeID {
		t.Fatalf("USER-DEFINED eligibility = %#v", eligibility)
	}
	selection := eligibility.Preview.Selection
	selection.ProfileSource = core.SelectionUser
	selection.TopologySource = core.SelectionUser
	bundle := requireBundleV4(t, compileSelection(t, fixture.request, selection))
	if bundle.Recipe.ID != userRecipeID || bundle.Graph.RecipeID != userRecipeID || bundle.Selection.Profile != core.UserDefinedProfile {
		t.Fatalf("USER-DEFINED Bundle = %#v", bundle)
	}
}

func TestSelectionConfirmationRejectsStaleProviderOrHostEvidence(t *testing.T) {
	fixture := newCoreV4Fixture(t, coreFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent, complete: true})
	inspection := compileCore(t, fixture.request)
	selection := requireEligibility(t, inspection, "SP-FULL", "").Preview.Selection
	selection.ProfileSource = core.SelectionUser
	selection.TopologySource = core.SelectionUser

	staleRegistry := fixture.request
	staleRegistry.Registry = fixture.registry.withChangedBindingEvidence()
	staleRegistry.Selection = &selection
	_, err := core.Compile(staleRegistry)
	requireCoreCode(t, err, "PROFILE_SELECTION_INVALID")

	changedHost := newCoreV4Fixture(t, coreFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent, complete: true, evidenceLabel: "changed"})
	changedHost.request.Selection = &selection
	_, err = core.Compile(changedHost.request)
	requireCoreCode(t, err, "PROFILE_SELECTION_INVALID")
}

func TestCurrentCodexReportsReasonCodedExclusions(t *testing.T) {
	fixture := newCoreV4Fixture(t, coreFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent})
	inspection := compileCore(t, fixture.request)
	if inspection.Bundle != nil || inspection.SelectionPreview != nil {
		t.Fatalf("CURRENT Codex inspection created authority: %#v", inspection)
	}
	wantCodes := map[string][]string{
		"ECC-FULL":       {"PROFILE_BINDING_UNAVAILABLE", "HOST_ACTION_UNATTESTED"},
		"MATT-FULL":      {"HOST_ACTION_UNATTESTED"},
		"MATT-SP-HYBRID": {"HOST_FEATURE_UNATTESTED"},
		"SP-FULL":        {"HOST_FEATURE_UNATTESTED"},
	}
	for alias, codes := range wantCodes {
		eligibility := requireEligibility(t, inspection, alias, "")
		if eligibility.Eligible || eligibility.Diagnostics == nil {
			t.Fatalf("%s CURRENT Codex eligibility = %#v", alias, eligibility)
		}
		for _, code := range codes {
			if !hasDiagnostic(eligibility.Diagnostics, code) {
				t.Fatalf("%s diagnostics lack %s: %#v", alias, code, eligibility.Diagnostics)
			}
		}
	}
}

func TestCompileBundleV4IsDeterministicAndDefensive(t *testing.T) {
	fixture := newCoreV4Fixture(t, coreFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent, complete: true})
	inspection := compileCore(t, fixture.request)
	selection := requireEligibility(t, inspection, "MATT-SP-HYBRID", "").Preview.Selection
	selection.ProfileSource = core.SelectionUser
	selection.TopologySource = core.SelectionUser
	first := compileSelection(t, fixture.request, selection)
	want, err := canonicaljson.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	first.EligibleProfiles[0].Diagnostics = append(first.EligibleProfiles[0].Diagnostics, profile.CompileDiagnostic{Code: "changed"})
	first.Bundle.Graph.Slots[0].Pipeline[0].MaximumEffects[0] = "changed"
	first.Bundle.Selection.AddOns = append(first.Bundle.Selection.AddOns, "changed")
	second := compileSelection(t, fixture.request, selection)
	got, err := canonicaljson.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("Compile changed after returned mutation\nwant=%s\ngot=%s", want, got)
	}
}

func TestCompileRejectsMalformedCoreV4Inputs(t *testing.T) {
	fixture := newCoreV4Fixture(t, coreFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent, complete: true})
	inspection := compileCore(t, fixture.request)
	selection := requireEligibility(t, inspection, "SP-FULL", "").Preview.Selection
	selection.ProfileSource = core.SelectionUser
	selection.TopologySource = core.SelectionUser

	tests := []struct {
		name   string
		code   string
		mutate func(*core.CompilationRequest)
	}{
		{name: "input digest", code: "CORE_INPUT_INVALID", mutate: func(value *core.CompilationRequest) { value.InputDigest = "invalid" }},
		{name: "resolution digest", code: "CORE_INPUT_INVALID", mutate: func(value *core.CompilationRequest) { value.ResolutionDigest = "invalid" }},
		{name: "unknown Recipe", code: "PROFILE_SELECTION_INVALID", mutate: func(value *core.CompilationRequest) { value.Selection.RecipeID = "acme/missing" }},
		{name: "topology drift", code: "PROFILE_TOPOLOGY_UNAVAILABLE", mutate: func(value *core.CompilationRequest) { value.Selection.Topology = execution.TopologySubagent }},
		{name: "selection source", code: "PROFILE_SELECTION_INVALID", mutate: func(value *core.CompilationRequest) { value.Selection.ProfileSource = core.SelectionHostOnlyOption }},
		{name: "duplicate Add-on", code: "PROFILE_SELECTION_INVALID", mutate: func(value *core.CompilationRequest) { value.Selection.AddOns = []string{"duplicate", "duplicate"} }},
		{name: "unknown overlay", code: "PROFILE_SELECTION_INVALID", mutate: func(value *core.CompilationRequest) { value.Selection.Overlays = []string{"missing"} }},
		{name: "Registry Host", code: "HOST_PROVIDER_SCOPE_MISMATCH", mutate: func(value *core.CompilationRequest) {
			changed := *fixture.registry
			changed.hostID = "claude"
			value.Registry = &changed
		}},
		{name: "Registry digest", code: "RESOLUTION_DIGEST_INVALID", mutate: func(value *core.CompilationRequest) {
			changed := *fixture.registry
			changed.digest = strings.Repeat("e", 64)
			value.Registry = &changed
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := fixture.request
			selected := selection
			request.Selection = &selected
			test.mutate(&request)
			_, err := core.Compile(request)
			requireCoreCode(t, err, test.code)
		})
	}
}

type coreV4Fixture struct {
	request  core.CompilationRequest
	registry *testRegistry
}

type coreFixtureOptions struct {
	hostID        string
	topology      execution.Topology
	complete      bool
	userRecipe    bool
	evidenceLabel string
}

const userRecipeID = "acme/reliable-delivery"

func newCoreV4Fixture(t testing.TB, options coreFixtureOptions) coreV4Fixture {
	t.Helper()
	if options.hostID == "" {
		options.hostID = "codex"
	}
	if options.topology == "" {
		options.topology = execution.TopologyCurrent
	}
	snapshot := coreSnapshot(t, options.userRecipe)
	observations := bindingObservations(t, snapshot.Catalog(), options)
	inventory, err := host.BuildBindingInventoryV3(options.hostID, observations)
	if err != nil {
		t.Fatal(err)
	}
	manifest := hostManifest(t, observations, options)
	environment := hostEnvironment(t, options)
	session := hostSession(t, manifest, inventory, environment, options)
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	effective := newTestRegistry(t, snapshot.Catalog(), inventory, observations, options.hostID)
	decision, err := core.Classify(nil, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	return coreV4Fixture{
		registry: effective,
		request: core.CompilationRequest{
			DeliverableID: "provider-surface-v4", InputDigest: strings.Repeat("1", 64), Generation: 1,
			Classification: decision, Configuration: snapshot, ResolutionDigest: strings.Repeat("2", 64),
			Registry: effective, Host: evidence,
		},
	}
}

func coreSnapshot(t testing.TB, withUserRecipe bool) config.Snapshot {
	t.Helper()
	if !withUserRecipe {
		snapshot, err := config.Load(config.LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	available, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	recipe := recipeFor(t, available, "oaw/delivery")
	recipe.ID = userRecipeID
	recipe.DisplayName = "Acme Reliable Delivery"
	recipe.Family = "acme"
	recipe.Template = "custom"
	recipe.RecipeVersion = "3.0.1"
	raw, err := canonicaljson.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeTestFile(t, root, "config.toml", []byte("schema_version = \"oaw.user-config/v3\"\n[[profile_recipes]]\nid = \""+userRecipeID+"\"\npath = \"recipes/reliable-delivery.json\"\n"))
	writeTestFile(t, root, "recipes/reliable-delivery.json", raw)
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func bindingObservations(t testing.TB, available catalog.Catalog, options coreFixtureOptions) []host.BindingObservation {
	t.Helper()
	result := []host.BindingObservation{}
	for _, provider := range available.Providers() {
		for _, binding := range provider.Bindings {
			if binding.Host != options.hostID || !options.complete && binding.Kind != catalog.BindingSkill {
				continue
			}
			topologies := []execution.Topology{execution.TopologyCurrent}
			if options.complete {
				topologies = append([]execution.Topology{}, binding.SupportedTopologies...)
			}
			observation, err := host.NewBindingObservation(host.BindingObservation{
				HostID: options.hostID, ProviderID: provider.ID, InstallationKey: installationKey(provider.ID),
				DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface, Kind: binding.Kind,
				Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
				Topologies: topologies, Source: host.SourceNativeAPI,
				EvidenceReference: "evidence://core/" + evidenceLabel(options) + "/bindings/" + strings.ReplaceAll(provider.ID, "/", "-") + "/" + binding.ID,
			})
			if err != nil {
				t.Fatalf("binding observation %s/%s: %v", provider.ID, binding.ID, err)
			}
			result = append(result, observation)
		}
	}
	return result
}

func hostManifest(t testing.TB, observations []host.BindingObservation, options coreFixtureOptions) host.Manifest {
	t.Helper()
	kindSet := map[catalog.BindingKind]struct{}{}
	for _, observation := range observations {
		kindSet[observation.Kind] = struct{}{}
	}
	kinds := make([]catalog.BindingKind, 0, len(kindSet))
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}
	topologies := []execution.Topology{execution.TopologyCurrent}
	features := []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}
	delegation := []host.FeatureID{}
	actions := []host.HostActionContract{}
	if options.complete {
		topologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
		features = append(features, host.FeatureEnvironmentReporting)
		delegation = []host.FeatureID{host.FeatureChildDelegation, host.FeatureNestedChildDelegation, host.FeatureNestedParallelDelegation, host.FeatureParallelChildDelegation}
		actions = coreHostActions()
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: options.hostID,
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: kinds,
		SupportedTopologies: topologies, Features: features, DelegationFeatures: delegation, HostActions: actions,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func hostEnvironment(t testing.TB, options coreFixtureOptions) host.EnvironmentReport {
	t.Helper()
	sessionID := "session-" + options.hostID + "-" + evidenceLabel(options)
	report := host.EnvironmentReport{SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: sessionID, Topology: options.topology, Observations: []execution.EnvironmentObservation{}}
	if options.topology == execution.TopologySubagent {
		report.ParentSessionID = "session-" + options.hostID + "-parent-" + evidenceLabel(options)
	}
	value, err := host.NewEnvironmentReport(report)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func hostSession(t testing.TB, manifest host.Manifest, inventory host.BindingInventory, environment host.EnvironmentReport, options coreFixtureOptions) host.SessionSnapshot {
	t.Helper()
	features := make([]host.FeatureObservation, len(manifest.DelegationFeatures))
	for index, feature := range manifest.DelegationFeatures {
		observation, err := host.NewFeatureObservation(host.FeatureObservation{
			Feature: feature, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://core/" + evidenceLabel(options) + "/features/" + string(feature),
		})
		if err != nil {
			t.Fatal(err)
		}
		features[index] = observation
	}
	actions := make([]host.HostActionObservation, len(manifest.HostActions))
	for index, action := range manifest.HostActions {
		observation, err := host.NewHostActionObservation(host.HostActionObservation{
			Action: action, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://core/" + evidenceLabel(options) + "/actions/" + action.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		actions[index] = observation
	}
	sessionID := environment.SessionID
	if options.topology == execution.TopologySubagent {
		sessionID = environment.ParentSessionID
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: options.hostID, IntegrationID: "test/core-host", IntegrationVersion: "3.0.0",
		SessionID: sessionID, ManifestDigest: manifest.Digest, SupportedTopologies: append([]execution.Topology{}, manifest.SupportedTopologies...),
		ProviderInventoryDigest: inventory.Digest, FeatureObservations: features, HostActionObservations: actions, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

type testRegistry struct {
	hostID       string
	providers    []registry.ProviderInstance
	providerByID map[string]registry.ProviderInstance
	bindings     map[string]registry.VerifiedBinding
	capabilities map[string]registry.VerifiedCapability
	digest       string
}

func newTestRegistry(t testing.TB, available catalog.Catalog, inventory host.BindingInventory, observations []host.BindingObservation, hostID string) *testRegistry {
	t.Helper()
	observed := map[string]host.BindingObservation{}
	for _, observation := range observations {
		observed[observation.ProviderID+"\x00"+observation.BindingID] = observation
	}
	result := &testRegistry{hostID: hostID, providerByID: map[string]registry.ProviderInstance{}, bindings: map[string]registry.VerifiedBinding{}, capabilities: map[string]registry.VerifiedCapability{}}
	for _, provider := range available.Providers() {
		verifiedBindings := []registry.VerifiedBinding{}
		for _, binding := range provider.Bindings {
			observation, found := observed[provider.ID+"\x00"+binding.ID]
			if !found {
				continue
			}
			verified := registry.VerifiedBinding{
				BindingID: binding.ID, DistributionID: binding.DistributionID, DistributionRevision: provider.Distributions[0].Revision,
				DistributionTreeDigest: provider.Distributions[0].TreeDigest, Surface: binding.Surface, Kind: binding.Kind,
				Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
				SupportedTopologies: append([]execution.Topology{}, observation.Topologies...), Delegation: binding.Delegation,
				Provenance: discovery.ProvenanceDistributionAttested, BindingEvidenceDigest: observation.Digest,
			}
			verifiedBindings = append(verifiedBindings, verified)
			result.bindings[provider.ID+"\x00"+binding.ID] = verified
		}
		if len(verifiedBindings) == 0 {
			continue
		}
		verifiedCapabilities := []registry.VerifiedCapability{}
		for _, capability := range provider.Capabilities {
			bindingIDs := []string{}
			for _, bindingID := range capability.BindingRefs {
				if _, found := result.bindings[provider.ID+"\x00"+bindingID]; found {
					bindingIDs = append(bindingIDs, bindingID)
				}
			}
			if len(bindingIDs) == 0 {
				continue
			}
			value := registry.VerifiedCapability{ID: capability.ID, BindingIDs: bindingIDs}
			verifiedCapabilities = append(verifiedCapabilities, value)
			result.capabilities[provider.ID+"\x00"+capability.ID] = value
		}
		descriptorDigest, _, err := canonicaljson.Digest(provider)
		if err != nil {
			t.Fatal(err)
		}
		instance := registry.ProviderInstance{
			ProviderID: provider.ID, HostID: hostID, DescriptorDigest: descriptorDigest,
			DistributionID: provider.Distributions[0].ID, DistributionRevision: provider.Distributions[0].Revision,
			DistributionTreeDigest: provider.Distributions[0].TreeDigest, InstallationKey: installationKey(provider.ID),
			ConfigurationDigest: canonicaljson.DigestBytes([]byte(provider.ID + "/configuration")), BindingInventoryDigest: inventory.Digest,
			EvidenceDigest: canonicaljson.DigestBytes([]byte(provider.ID + "/evidence/" + inventory.Digest)),
			Bindings:       verifiedBindings, Capabilities: verifiedCapabilities,
		}
		instance.Digest = providerInstanceDigest(instance)
		result.providers = append(result.providers, instance)
		result.providerByID[provider.ID] = instance
	}
	sort.Slice(result.providers, func(left, right int) bool {
		return result.providers[left].ProviderID < result.providers[right].ProviderID
	})
	result.digest, _, _ = canonicaljson.Digest(struct {
		SchemaVersion string                      `json:"schema_version"`
		HostID        string                      `json:"host_id"`
		Providers     []registry.ProviderInstance `json:"providers"`
	}{"oaw.effective-registry/v4", hostID, result.providers})
	return result
}

func (value *testRegistry) HostID() string { return value.hostID }
func (value *testRegistry) Digest() string { return value.digest }

func (value *testRegistry) Providers() []registry.ProviderInstance {
	return append([]registry.ProviderInstance{}, value.providers...)
}

func (value *testRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	provider, found := value.providerByID[id]
	return provider, found
}

func (value *testRegistry) Binding(providerID, bindingID string) (registry.VerifiedBinding, bool) {
	binding, found := value.bindings[providerID+"\x00"+bindingID]
	binding.SupportedTopologies = append([]execution.Topology{}, binding.SupportedTopologies...)
	return binding, found
}

func (value *testRegistry) Bindings(providerID string) []registry.VerifiedBinding {
	result := []registry.VerifiedBinding{}
	for _, provider := range value.providers {
		if provider.ProviderID == providerID {
			result = append(result, provider.Bindings...)
			break
		}
	}
	return result
}

func (value *testRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	capability, found := value.capabilities[providerID+"\x00"+capabilityID]
	capability.BindingIDs = append([]string{}, capability.BindingIDs...)
	return capability, found
}

func (value *testRegistry) withChangedBindingEvidence() *testRegistry {
	copy := *value
	copy.providers = append([]registry.ProviderInstance{}, value.providers...)
	copy.providerByID = make(map[string]registry.ProviderInstance, len(value.providerByID))
	for id, provider := range value.providerByID {
		copy.providerByID[id] = provider
	}
	copy.bindings = make(map[string]registry.VerifiedBinding, len(value.bindings))
	for id, binding := range value.bindings {
		copy.bindings[id] = binding
	}
	provider := copy.providers[0]
	provider.Bindings = append([]registry.VerifiedBinding{}, provider.Bindings...)
	provider.Bindings[0].BindingEvidenceDigest = strings.Repeat("e", 64)
	provider.Digest = providerInstanceDigest(provider)
	copy.providers[0] = provider
	copy.providerByID[provider.ProviderID] = provider
	copy.bindings[provider.ProviderID+"\x00"+provider.Bindings[0].BindingID] = provider.Bindings[0]
	copy.digest, _, _ = canonicaljson.Digest(struct {
		SchemaVersion string                      `json:"schema_version"`
		HostID        string                      `json:"host_id"`
		Providers     []registry.ProviderInstance `json:"providers"`
	}{"oaw.effective-registry/v4", copy.hostID, copy.providers})
	return &copy
}

func providerInstanceDigest(instance registry.ProviderInstance) string {
	digest, _, _ := canonicaljson.Digest(struct {
		SchemaVersion          string                        `json:"schema_version"`
		ProviderID             string                        `json:"provider_id"`
		HostID                 string                        `json:"host_id"`
		DescriptorDigest       string                        `json:"descriptor_digest"`
		DistributionID         string                        `json:"distribution_id"`
		DistributionRevision   string                        `json:"distribution_revision"`
		DistributionTreeDigest string                        `json:"distribution_tree_digest"`
		InstallationKey        string                        `json:"installation_key"`
		ConfigurationDigest    string                        `json:"configuration_digest"`
		BindingInventoryDigest string                        `json:"binding_inventory_digest"`
		EvidenceDigest         string                        `json:"evidence_digest"`
		Bindings               []registry.VerifiedBinding    `json:"bindings"`
		Capabilities           []registry.VerifiedCapability `json:"capabilities"`
	}{
		"oaw.provider-instance/v4", instance.ProviderID, instance.HostID, instance.DescriptorDigest,
		instance.DistributionID, instance.DistributionRevision, instance.DistributionTreeDigest, instance.InstallationKey,
		instance.ConfigurationDigest, instance.BindingInventoryDigest, instance.EvidenceDigest, instance.Bindings, instance.Capabilities,
	})
	return digest
}

func compileCore(t testing.TB, request core.CompilationRequest) core.CompilationResult {
	t.Helper()
	result, err := core.Compile(request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func compileSelection(t testing.TB, base core.CompilationRequest, selection core.Selection) core.CompilationResult {
	t.Helper()
	request := base
	request.Selection = &selection
	return compileCore(t, request)
}

func requireEligibility(t testing.TB, result core.CompilationResult, profileID, recipeID string) core.ProfileEligibility {
	t.Helper()
	for _, value := range result.EligibleProfiles {
		if value.Profile == profileID && (recipeID == "" || value.RecipeID == recipeID) {
			return value
		}
	}
	t.Fatalf("eligibility %s/%s missing from %#v", profileID, recipeID, result.EligibleProfiles)
	return core.ProfileEligibility{}
}

func requireBundleV4(t testing.TB, result core.CompilationResult) core.LifecycleBundle {
	t.Helper()
	if result.Bundle == nil {
		t.Fatalf("Compile returned no Bundle: %#v", result)
	}
	bundle := *result.Bundle
	if bundle.SchemaVersion != core.LifecycleBundleSchemaV4 || bundle.Graph.SchemaVersion != profile.ExecutionGraphSchemaV4 || bundle.Digest == "" || bundle.ID == "" {
		t.Fatalf("Bundle v4 = %#v", bundle)
	}
	if bundle.Graph.Digest != bundle.Graph.ContentDigest() || bundle.RegistryDigest != bundle.Graph.RegistryDigest || bundle.HostEvidenceDigest != bundle.Graph.HostEvidenceDigest {
		t.Fatalf("Bundle graph pins = %#v", bundle)
	}
	return bundle
}

func alternativesForHost(t testing.TB, available catalog.Catalog, recipe catalog.ProfileRecipeRecord, hostID string) []profile.AlternativeChoice {
	t.Helper()
	providers := map[string]catalog.ProviderDescriptorRecord{}
	for _, provider := range available.Providers() {
		providers[provider.ID] = provider
	}
	choices := []profile.AlternativeChoice{}
	for _, slot := range recipe.Slots {
		for _, step := range slot.Pipeline {
			provider := providers[step.Selector.ProviderID]
			binding := descriptorBinding(t, provider, step.Selector.BindingID)
			if binding.Host == hostID {
				continue
			}
			alternativeID := ""
			for _, candidateID := range binding.Alternatives {
				if descriptorBinding(t, provider, candidateID).Host == hostID {
					alternativeID = candidateID
					break
				}
			}
			if alternativeID == "" {
				t.Fatalf("%s/%s has no %s alternative", provider.ID, binding.ID, hostID)
			}
			choices = append(choices, profile.AlternativeChoice{
				SlotID: slot.SlotID, StepID: step.ID, AlternativeID: alternativeID,
				Selector: catalog.BindingSelector{ProviderID: provider.ID, BindingID: alternativeID},
			})
		}
	}
	return choices
}

func recipeFor(t testing.TB, available catalog.Catalog, identity string) catalog.ProfileRecipeRecord {
	t.Helper()
	for _, recipe := range available.Recipes() {
		if recipe.ID == identity {
			return recipe
		}
	}
	t.Fatalf("recipe %s not found", identity)
	return catalog.ProfileRecipeRecord{}
}

func descriptorBinding(t testing.TB, provider catalog.ProviderDescriptorRecord, bindingID string) catalog.BindingRecord {
	t.Helper()
	for _, binding := range provider.Bindings {
		if binding.ID == bindingID {
			return binding
		}
	}
	t.Fatalf("binding %s/%s not found", provider.ID, bindingID)
	return catalog.BindingRecord{}
}

func hasDiagnostic(values []profile.CompileDiagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}

func coreHostActions() []host.HostActionContract {
	return []host.HostActionContract{
		{ID: "workspace.prepare-or-confirm", InputSchema: "oaw.host-action.workspace-input/v1", OutcomeSchema: "oaw.host-action.workspace-outcome/v1", MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project-worktree"}},
		{ID: "verification.execute", InputSchema: "oaw.host-action.verification-input/v1", OutcomeSchema: "oaw.host-action.verification-outcome/v1", MaximumEffects: []string{"read-project", "run-process"}, Resources: []string{"project"}},
		{ID: "closeout.execute", InputSchema: "oaw.host-action.closeout-input/v1", OutcomeSchema: "oaw.host-action.closeout-outcome/v1", MaximumEffects: []string{"git-local", "network-mutation", "read-project", "run-process"}, Resources: []string{"git-repository", "network", "project-worktree"}},
	}
}

func writeTestFile(t testing.TB, root, relative string, content []byte) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}

func installationKey(providerID string) string {
	return "installation-" + strings.ReplaceAll(providerID, "/", "-")
}

func evidenceLabel(options coreFixtureOptions) string {
	if options.evidenceLabel == "" {
		return "default"
	}
	return options.evidenceLabel
}

func requireCoreCode(t testing.TB, err error, code string) {
	t.Helper()
	coreErr, ok := err.(*core.Error)
	if !ok || coreErr.Code != code {
		t.Fatalf("error = %T %v, want Core code %s", err, err, code)
	}
}
