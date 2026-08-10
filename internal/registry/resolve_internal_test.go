package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

type syntheticResolutionSource struct {
	catalog      catalog.Catalog
	settings     map[string]config.ProviderSettings
	required     []string
	recommended  []string
	untrustedIDs []string
}

func (source syntheticResolutionSource) Catalog() catalog.Catalog { return source.catalog }

func (source syntheticResolutionSource) ProviderSettings(providerID, hostID string) config.ProviderSettings {
	value := source.settings[providerID+"\x00"+hostID]
	value.Preferences = append([]config.BindingPreference{}, value.Preferences...)
	value.CapabilityLimit = append([]string{}, value.CapabilityLimit...)
	if value.Pin != nil {
		pin := *value.Pin
		value.Pin = &pin
	}
	return value
}

func (source syntheticResolutionSource) RequiredProviders() []string {
	return append([]string{}, source.required...)
}

func (source syntheticResolutionSource) RecommendedProviders() []string {
	return append([]string{}, source.recommended...)
}

func (source syntheticResolutionSource) UntrustedProviderIDs() []string {
	return append([]string{}, source.untrustedIDs...)
}

type registryV4Fixture struct {
	home      string
	provider  catalog.ProviderDescriptorRecord
	source    syntheticResolutionSource
	report    discovery.Report
	inventory host.BindingInventory
}

func TestRegistryV4RetainsBindingAlternatives(t *testing.T) {
	fixture := newRegistryV4Fixture(t)
	report, effective, err := resolveFromSource(fixture.source, "codex", fixture.report, &fixture.inventory)
	if err != nil {
		t.Fatalf("resolveFromSource() error = %v", err)
	}
	resolution := requireInternalResolution(t, report, fixture.provider.ID)
	if resolution.State != ProviderVerified || resolution.Instance == nil || len(resolution.Instance.Bindings) != 2 {
		t.Fatalf("resolution = %#v", resolution)
	}
	capability, found := effective.Capability(fixture.provider.ID, "workflow")
	if !found || !slices.Equal(capability.BindingIDs, []string{"alpha", "zeta"}) || capability.PreferredBindingID != "" {
		t.Fatalf("Capability() = %#v, found=%v", capability, found)
	}
	bindings := effective.Bindings(fixture.provider.ID)
	if len(bindings) != 2 || bindings[0].BindingID != "alpha" || bindings[1].BindingID != "zeta" {
		t.Fatalf("Bindings() = %#v", bindings)
	}
	for _, bindingID := range capability.BindingIDs {
		if _, found := effective.Binding(fixture.provider.ID, bindingID); !found {
			t.Fatalf("Binding(%q) not found", bindingID)
		}
	}
}

func TestRegistryV4RejectsEvidenceMismatches(t *testing.T) {
	t.Run("Provider", func(t *testing.T) {
		fixture := newRegistryV4Fixture(t)
		inventory := rebuildInventory(t, fixture.inventory, func(values []host.BindingObservation) {
			for index := range values {
				values[index].ProviderID = "other/provider"
			}
		})
		assertBindingUnavailable(t, fixture.source, fixture.report, inventory, fixture.provider.ID)
	})

	t.Run("Host", func(t *testing.T) {
		fixture := newRegistryV4Fixture(t)
		if _, _, err := resolveFromSource(fixture.source, "other", fixture.report, &fixture.inventory); err == nil || !strings.Contains(err.Error(), "HOST_PROVIDER_SCOPE_MISMATCH") {
			t.Fatalf("Host mismatch error = %v", err)
		}
	})

	t.Run("surface", func(t *testing.T) {
		fixture := newRegistryV4Fixture(t)
		provider := cloneFixtureProvider(t, fixture.provider)
		provider.Discovery[0].Surface = "other-skills"
		for index := range provider.Bindings {
			provider.Bindings[index].Surface = "other-skills"
		}
		report := discoverFixture(t, provider, fixture.home)
		assertBindingUnavailable(t, fixture.source, report, &fixture.inventory, fixture.provider.ID)
	})

	for _, test := range []struct {
		name   string
		mutate func(*catalog.ProviderDescriptorRecord)
	}{
		{name: "revision", mutate: func(provider *catalog.ProviderDescriptorRecord) {
			provider.Distributions[0].Revision = strings.Repeat("d", 40)
		}},
		{name: "Distribution tree", mutate: func(provider *catalog.ProviderDescriptorRecord) {
			provider.Distributions[0].TreeDigest = "sha256:" + strings.Repeat("d", 64)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRegistryV4Fixture(t)
			provider := cloneFixtureProvider(t, fixture.provider)
			test.mutate(&provider)
			source := sourceWithProvider(t, fixture.source, provider)
			assertBindingUnavailable(t, source, fixture.report, &fixture.inventory, fixture.provider.ID)
		})
	}

	for _, test := range []struct {
		name   string
		mutate func(*host.BindingObservation)
	}{
		{name: "Binding tree", mutate: func(value *host.BindingObservation) { value.BindingTreeDigest = "sha256:" + strings.Repeat("d", 64) }},
		{name: "kind", mutate: func(value *host.BindingObservation) { value.Kind = catalog.BindingTool }},
		{name: "reference", mutate: func(value *host.BindingObservation) { value.Reference = "different" }},
		{name: "invocation", mutate: func(value *host.BindingObservation) { value.Invocation = catalog.InvocationHost }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRegistryV4Fixture(t)
			inventory := rebuildInventory(t, fixture.inventory, func(values []host.BindingObservation) {
				for index := range values {
					test.mutate(&values[index])
				}
			})
			assertBindingUnavailable(t, fixture.source, fixture.report, inventory, fixture.provider.ID)
		})
	}

	t.Run("duplicate Host observation", func(t *testing.T) {
		fixture := newRegistryV4Fixture(t)
		invalid := host.CloneBindingInventory(fixture.inventory)
		invalid.Observations = append(invalid.Observations, invalid.Observations[0])
		invalid.Digest = ""
		if _, _, err := resolveFromSource(fixture.source, "codex", fixture.report, &invalid); err == nil || !strings.Contains(err.Error(), "HOST_BINDING_INVENTORY_INVALID") {
			t.Fatalf("duplicate observation error = %v", err)
		}
	})

	t.Run("provenance", func(t *testing.T) {
		fixture := newRegistryV4Fixture(t)
		provider := cloneFixtureProvider(t, fixture.provider)
		provider.Bindings[0].TreeDigest = "sha256:" + strings.Repeat("e", 64)
		report := discoverFixture(t, provider, fixture.home)
		assertBindingUnavailable(t, fixture.source, report, &fixture.inventory, fixture.provider.ID)
	})
}

func TestRegistryV4AppliesExactPreference(t *testing.T) {
	fixture := newRegistryV4Fixture(t)
	preference := config.BindingPreference{ProviderID: fixture.provider.ID, CapabilityID: "workflow", HostID: "codex", Kind: "skill", Reference: "zeta"}
	source := sourceWithPreference(fixture.source, preference)
	report, effective, err := resolveFromSource(source, "codex", fixture.report, &fixture.inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireInternalResolution(t, report, fixture.provider.ID)
	capability, found := effective.Capability(fixture.provider.ID, "workflow")
	if resolution.State != ProviderVerified || !found || capability.PreferredBindingID != "zeta" || !slices.Equal(capability.BindingIDs, []string{"alpha", "zeta"}) {
		t.Fatalf("preferred resolution = %#v / %#v", resolution, capability)
	}

	t.Run("zero match", func(t *testing.T) {
		fixture := newRegistryV4Fixture(t)
		source := sourceWithPreference(fixture.source, config.BindingPreference{ProviderID: fixture.provider.ID, CapabilityID: "workflow", HostID: "codex", Kind: "skill", Reference: "missing"})
		assertPreferenceIncompatible(t, source, fixture)
	})

	t.Run("multiple matches", func(t *testing.T) {
		fixture := newRegistryV4Fixture(t)
		provider := cloneFixtureProvider(t, fixture.provider)
		provider.Bindings[1].Reference = provider.Bindings[0].Reference
		source := sourceWithProvider(t, fixture.source, provider)
		source = sourceWithPreference(source, config.BindingPreference{ProviderID: provider.ID, CapabilityID: "workflow", HostID: "codex", Kind: "skill", Reference: provider.Bindings[0].Reference})
		inventory := rebuildInventory(t, fixture.inventory, func(values []host.BindingObservation) {
			values[1].Reference = values[0].Reference
		})
		fixture.source = source
		fixture.inventory = *inventory
		assertPreferenceIncompatible(t, source, fixture)
	})
}

func TestRegistryV4DefensiveCopies(t *testing.T) {
	fixture := newRegistryV4Fixture(t)
	report, effective, err := resolveFromSource(fixture.source, "codex", fixture.report, &fixture.inventory)
	if err != nil {
		t.Fatal(err)
	}
	digest := effective.Digest()
	resolution := requireInternalResolution(t, report, fixture.provider.ID)
	resolution.Instance.Bindings[0].SupportedTopologies[0] = execution.TopologySubagent
	resolution.Instance.Capabilities[0].BindingIDs[0] = "changed"
	resolution.Candidates[0].BindingRoots[0].Tree.Entries[0].Path = "changed"

	providers := effective.Providers()
	providers[0].Bindings[0].SupportedTopologies[0] = execution.TopologySubagent
	providers[0].Capabilities[0].BindingIDs[0] = "changed"
	bindings := effective.Bindings(fixture.provider.ID)
	bindings[0].SupportedTopologies[0] = execution.TopologySubagent
	binding, _ := effective.Binding(fixture.provider.ID, "alpha")
	binding.SupportedTopologies[0] = execution.TopologySubagent
	capability, _ := effective.Capability(fixture.provider.ID, "workflow")
	capability.BindingIDs[0] = "changed"

	freshResolution := requireInternalResolution(t, report, fixture.provider.ID)
	freshProvider, _ := effective.Provider(fixture.provider.ID)
	freshBinding, _ := effective.Binding(fixture.provider.ID, "alpha")
	freshCapability, _ := effective.Capability(fixture.provider.ID, "workflow")
	if freshResolution.Instance.Bindings[0].SupportedTopologies[0] != execution.TopologyCurrent || freshResolution.Instance.Capabilities[0].BindingIDs[0] != "alpha" ||
		freshResolution.Candidates[0].BindingRoots[0].Tree.Entries[0].Path == "changed" || freshProvider.Bindings[0].SupportedTopologies[0] != execution.TopologyCurrent ||
		freshProvider.Capabilities[0].BindingIDs[0] != "alpha" || freshBinding.SupportedTopologies[0] != execution.TopologyCurrent || freshCapability.BindingIDs[0] != "alpha" || effective.Digest() != digest {
		t.Fatal("Registry v4 exposed mutable nested storage")
	}
}

func TestRegistryV4ReportsEveryProviderState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *registryV4Fixture, **host.BindingInventory)
		state  ProviderState
		reason string
	}{
		{
			name: "not found",
			mutate: func(t *testing.T, fixture *registryV4Fixture, _ **host.BindingInventory) {
				t.Helper()
				empty, err := catalog.New(nil, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				fixture.source.catalog = empty
				fixture.source.required = []string{fixture.provider.ID}
				fixture.report, err = discovery.Discover(empty, discovery.Options{HostID: "codex", UserHome: fixture.home})
				if err != nil {
					t.Fatal(err)
				}
			},
			state: ProviderNotFound, reason: "PROVIDER_NOT_FOUND",
		},
		{
			name: "candidate",
			mutate: func(_ *testing.T, _ *registryV4Fixture, inventory **host.BindingInventory) {
				*inventory = nil
			},
			state: ProviderCandidate, reason: "HOST_BINDING_EVIDENCE_REQUIRED",
		},
		{
			name:   "verified",
			mutate: func(_ *testing.T, _ *registryV4Fixture, _ **host.BindingInventory) {},
			state:  ProviderVerified, reason: "PROVIDER_VERIFIED",
		},
		{
			name: "ambiguous",
			mutate: func(t *testing.T, fixture *registryV4Fixture, _ **host.BindingInventory) {
				t.Helper()
				secondRoot := filepath.Join(fixture.home, "provider-second")
				writeFixtureFile(t, secondRoot, "probe.txt", "probe")
				writeFixtureFile(t, secondRoot, "alpha/SKILL.md", "alpha")
				writeFixtureFile(t, secondRoot, "zeta/SKILL.md", "zeta")
				writeDistributionManifest(t, secondRoot, fixture.provider.Distributions[0])
				var err error
				fixture.report, err = discovery.Discover(fixture.source.catalog, discovery.Options{
					HostID: "codex", UserHome: fixture.home,
					Installations: []discovery.InstallationHint{{
						ProviderID: fixture.provider.ID, HostID: "codex", SurfaceID: "codex-skills", Location: secondRoot, DiscoveryProbeID: "probe",
					}},
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			state: ProviderAmbiguous, reason: "PROVIDER_CANDIDATE_AMBIGUOUS",
		},
		{
			name: "incompatible pin",
			mutate: func(_ *testing.T, fixture *registryV4Fixture, _ **host.BindingInventory) {
				settings := fixture.source.ProviderSettings(fixture.provider.ID, "codex")
				settings.Pin = &config.ProviderPin{
					ProviderID: fixture.provider.ID, HostID: "codex", InstallationKey: "installation-missing", EvidenceDigest: strings.Repeat("f", 64),
				}
				fixture.source.settings[fixture.provider.ID+"\x00codex"] = settings
			},
			state: ProviderIncompatible, reason: "PROVIDER_PIN_INCOMPATIBLE",
		},
		{
			name: "binding unavailable",
			mutate: func(t *testing.T, fixture *registryV4Fixture, inventory **host.BindingInventory) {
				*inventory = rebuildInventory(t, fixture.inventory, func(values []host.BindingObservation) {
					for index := range values {
						values[index].InstallationKey = "installation-wrong"
					}
				})
			},
			state: ProviderBindingUnavailable, reason: "PROVIDER_BINDING_UNAVAILABLE",
		},
		{
			name: "disabled",
			mutate: func(_ *testing.T, fixture *registryV4Fixture, _ **host.BindingInventory) {
				settings := fixture.source.ProviderSettings(fixture.provider.ID, "codex")
				settings.Disabled = true
				fixture.source.settings[fixture.provider.ID+"\x00codex"] = settings
			},
			state: ProviderDisabled, reason: "PROVIDER_DISABLED_BY_USER",
		},
		{
			name: "untrusted",
			mutate: func(_ *testing.T, fixture *registryV4Fixture, _ **host.BindingInventory) {
				fixture.source.untrustedIDs = []string{fixture.provider.ID}
			},
			state: ProviderUntrusted, reason: "PROVIDER_PROJECT_CONTENT_UNTRUSTED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRegistryV4Fixture(t)
			inventory := &fixture.inventory
			test.mutate(t, &fixture, &inventory)
			report, effective, err := resolveFromSource(fixture.source, "codex", fixture.report, inventory)
			if err != nil {
				t.Fatal(err)
			}
			resolution := requireInternalResolution(t, report, fixture.provider.ID)
			if resolution.State != test.state || resolution.Reason != test.reason {
				t.Fatalf("resolution = %#v", resolution)
			}
			_, admitted := effective.Provider(fixture.provider.ID)
			if admitted != (test.state == ProviderVerified) {
				t.Fatalf("Registry admitted=%v for state %q", admitted, test.state)
			}
		})
	}
}

func TestRegistryV4UsesExactPinAndObservedTopologies(t *testing.T) {
	fixture := newRegistryV4Fixture(t)
	candidate := fixture.report.Candidates(fixture.provider.ID)[0]
	settings := fixture.source.ProviderSettings(fixture.provider.ID, "codex")
	settings.Pin = &config.ProviderPin{
		ProviderID: fixture.provider.ID, HostID: candidate.HostID, InstallationKey: candidate.InstallationKey,
		EvidenceDigest: candidate.EvidenceDigest, Location: candidate.DiagnosticLocation, Version: candidate.ObservedRevision,
	}
	fixture.source.settings[fixture.provider.ID+"\x00codex"] = settings

	report, effective, err := resolveFromSource(fixture.source, "codex", fixture.report, &fixture.inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireInternalResolution(t, report, fixture.provider.ID)
	if resolution.State != ProviderVerified || resolution.Instance == nil || resolution.Instance.DistributionRevision != candidate.ObservedRevision {
		t.Fatalf("resolution = %#v", resolution)
	}
	for _, binding := range effective.Bindings(fixture.provider.ID) {
		if !slices.Equal(binding.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
			t.Fatalf("Binding %q widened observed topologies: %#v", binding.BindingID, binding.SupportedTopologies)
		}
	}

	settings.Pin.Version = strings.Repeat("f", 40)
	fixture.source.settings[fixture.provider.ID+"\x00codex"] = settings
	report, _, err = resolveFromSource(fixture.source, "codex", fixture.report, &fixture.inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution = requireInternalResolution(t, report, fixture.provider.ID)
	if resolution.State != ProviderIncompatible || resolution.Reason != "PROVIDER_PIN_INCOMPATIBLE" {
		t.Fatalf("mismatched revision pin resolution = %#v", resolution)
	}
}

func newRegistryV4Fixture(t *testing.T) registryV4Fixture {
	t.Helper()
	home := t.TempDir()
	root := filepath.Join(home, "provider")
	writeFixtureFile(t, root, "probe.txt", "probe")
	writeFixtureFile(t, root, "alpha/SKILL.md", "alpha")
	writeFixtureFile(t, root, "zeta/SKILL.md", "zeta")
	alphaTree, err := integrity.DigestTree(filepath.Join(root, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	zetaTree, err := integrity.DigestTree(filepath.Join(root, "zeta"))
	if err != nil {
		t.Fatal(err)
	}
	revision := strings.Repeat("a", 40)
	distributionTree := "sha256:" + strings.Repeat("b", 64)
	provider := registryV4Provider(alphaTree.RootDigest, zetaTree.RootDigest, revision, distributionTree)
	writeDistributionManifest(t, root, provider.Distributions[0])
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	report := discoverFixture(t, provider, home)
	candidates := report.Candidates(provider.ID)
	if len(candidates) != 1 {
		t.Fatalf("Candidates() = %#v", candidates)
	}
	inventory := fixtureInventory(t, provider, candidates[0])
	settings := config.ProviderSettings{ProviderID: provider.ID, HostID: "codex", Preferences: []config.BindingPreference{}, CapabilityLimit: []string{}, Digest: strings.Repeat("c", 64)}
	return registryV4Fixture{
		home: home, provider: provider, report: report, inventory: inventory,
		source: syntheticResolutionSource{catalog: value, settings: map[string]config.ProviderSettings{provider.ID + "\x00codex": settings}},
	}
}

func registryV4Provider(alphaDigest, zetaDigest, revision, distributionTree string) catalog.ProviderDescriptorRecord {
	bindings := []catalog.BindingRecord{
		registryV4Binding("alpha", alphaDigest),
		registryV4Binding("zeta", zetaDigest),
	}
	bindings[0].Alternatives = []string{"zeta"}
	bindings[1].Alternatives = []string{"alpha"}
	return catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "test/provider", DisplayName: "Test Provider",
		Distributions: []catalog.DistributionRecord{{ID: "distribution", SourceURI: "https://example.test/provider", Revision: revision, TreeDigest: distributionTree}},
		Discovery:     []catalog.DiscoveryProbe{{ID: "probe", Hosts: []string{"codex"}, Surface: "codex-skills", DistributionID: "distribution", Kind: "path-exists", Root: "user-home", CandidatePath: "provider", EvidencePath: "probe.txt"}},
		Bindings:      bindings,
		Capabilities:  []catalog.CapabilityRecord{{ID: "workflow", InputSchema: "artifact", OutcomeSchema: "artifact", RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"alpha", "zeta"}}},
	}
}

func registryV4Binding(id, digest string) catalog.BindingRecord {
	return catalog.BindingRecord{
		ID: id, DistributionID: "distribution", ContentRoot: "skills/" + id, InstallRoot: id, TreeDigest: digest,
		Host: "codex", Surface: "codex-skills", Kind: catalog.BindingSkill, Reference: id, Invocation: catalog.InvocationModel,
		Responsibilities: []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: "implementation", SlotID: catalog.SlotImplementation, OutcomeOwner: true}},
		InputArtifact:    "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}, Delegation: catalog.DelegationRequirements{},
		StageSpan: []catalog.SlotID{catalog.SlotImplementation}, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
	}
}

func fixtureInventory(t *testing.T, provider catalog.ProviderDescriptorRecord, candidate discovery.Candidate) host.BindingInventory {
	t.Helper()
	values := make([]host.BindingObservation, len(provider.Bindings))
	for index, binding := range provider.Bindings {
		values[index] = host.BindingObservation{
			HostID: candidate.HostID, ProviderID: provider.ID, InstallationKey: candidate.InstallationKey, DistributionID: binding.DistributionID,
			BindingID: binding.ID, Surface: binding.Surface, Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation,
			BindingTreeDigest: binding.TreeDigest, Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://registry/" + binding.ID,
		}
	}
	inventory, err := host.BuildBindingInventoryV3(candidate.HostID, values)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func rebuildInventory(t *testing.T, inventory host.BindingInventory, mutate func([]host.BindingObservation)) *host.BindingInventory {
	t.Helper()
	values := host.CloneBindingInventory(inventory).Observations
	mutate(values)
	for index := range values {
		values[index].Digest = ""
	}
	rebuilt, err := host.BuildBindingInventoryV3(inventory.HostID, values)
	if err != nil {
		t.Fatal(err)
	}
	return &rebuilt
}

func sourceWithProvider(t *testing.T, source syntheticResolutionSource, provider catalog.ProviderDescriptorRecord) syntheticResolutionSource {
	t.Helper()
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	source.catalog = value
	return source
}

func sourceWithPreference(source syntheticResolutionSource, preference config.BindingPreference) syntheticResolutionSource {
	settings := source.ProviderSettings(preference.ProviderID, preference.HostID)
	settings.Preferences = []config.BindingPreference{preference}
	settings.Digest = strings.Repeat("d", 64)
	source.settings = map[string]config.ProviderSettings{preference.ProviderID + "\x00" + preference.HostID: settings}
	return source
}

func cloneFixtureProvider(t *testing.T, provider catalog.ProviderDescriptorRecord) catalog.ProviderDescriptorRecord {
	t.Helper()
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return value.Providers()[0]
}

func discoverFixture(t *testing.T, provider catalog.ProviderDescriptorRecord, home string) discovery.Report {
	t.Helper()
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func assertBindingUnavailable(t *testing.T, source syntheticResolutionSource, report discovery.Report, inventory *host.BindingInventory, providerID string) {
	t.Helper()
	result, _, err := resolveFromSource(source, "codex", report, inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireInternalResolution(t, result, providerID)
	if resolution.State != ProviderBindingUnavailable || resolution.Reason != "PROVIDER_BINDING_UNAVAILABLE" || resolution.Instance != nil {
		t.Fatalf("resolution = %#v", resolution)
	}
}

func assertPreferenceIncompatible(t *testing.T, source syntheticResolutionSource, fixture registryV4Fixture) {
	t.Helper()
	report, effective, err := resolveFromSource(source, "codex", fixture.report, &fixture.inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireInternalResolution(t, report, fixture.provider.ID)
	if resolution.State != ProviderIncompatible || resolution.Reason != "BINDING_PREFERENCE_INCOMPATIBLE" || resolution.Instance != nil {
		t.Fatalf("resolution = %#v", resolution)
	}
	if _, found := effective.Provider(fixture.provider.ID); found {
		t.Fatal("incompatible preference entered Registry")
	}
}

func requireInternalResolution(t *testing.T, report ResolutionReport, providerID string) ProviderResolution {
	t.Helper()
	value, found := report.Resolution(providerID)
	if !found {
		t.Fatalf("Resolution(%q) not found in %#v", providerID, report.Resolutions())
	}
	return value
}

func writeFixtureFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDistributionManifest(t *testing.T, root string, distribution catalog.DistributionRecord) {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"distribution_id": distribution.ID, "revision": distribution.Revision, "tree_digest": distribution.TreeDigest})
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, root, ".oaw-distribution.json", string(raw))
}
