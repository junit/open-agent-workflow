package integration_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestCodexBridgeUsesOneBindingAlgorithmForEveryProvider(t *testing.T) {
	cases := []struct {
		provider string
		skill    string
	}{
		{provider: "oaw/superpowers", skill: "superpowers:writing-plans"},
		{provider: "oaw/matt", skill: "tdd"},
		{provider: "oaw/ecc", skill: "tdd-workflow"},
		{provider: "acme/custom", skill: "acme:delivery"},
	}
	for _, test := range cases {
		t.Run(test.provider, func(t *testing.T) {
			assertExactSkillBinding(t, test.provider, test.skill)
		})
	}
}

func TestCodexBridgeECCFullFixtureRequiresEverySkill(t *testing.T) {
	assertECCFullSkillCoverage(t)
}

func TestCodexPolicyInstallationNeverClaimsHostNative(t *testing.T) {
	snapshot := loadPolicyAndNativeCodexSnapshot(t)
	if integrationCanSupplyInventory(snapshot, "oaw/codex-policy") {
		t.Fatal("policy Integration supplied Host authority")
	}
	if !integrationCanSupplyInventory(snapshot, "oaw/codex-host") {
		t.Fatal("audited Host-native Integration did not supply inventory authority")
	}
}

func assertExactSkillBinding(t *testing.T, providerID, skill string) {
	t.Helper()
	home := t.TempDir()
	providerRoot := filepath.Join(home, ".codex", "plugins", "provider")
	markerPath := filepath.Join(providerRoot, "provider.marker")
	skillPath := filepath.Join(providerRoot, "skills", "observed", "SKILL.md")
	writeBridgeFixtureFile(t, markerPath, providerID)
	writeBridgeFixtureFile(t, skillPath, "---\nname: "+skill+"\n---\n")
	descriptor := singleSkillProvider(providerID, skill)
	available, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(available, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory, diagnostics, err := codexbridge.BuildBindingInventory(available, report, appserver.MetadataObservation{
		Skills: appserver.SkillsEntry{CWD: home, Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{{
			Name: skill, Enabled: true, Path: skillPath, Scope: "user",
		}}},
	}, home)
	if err != nil || len(diagnostics) != 0 || len(inventory.Observations) != 1 {
		t.Fatalf("inventory = %#v, diagnostics = %#v, error = %v", inventory, diagnostics, err)
	}
	observation := inventory.Observations[0]
	candidates := report.Candidates(providerID)
	if len(candidates) != 1 || observation.InstallationKey != candidates[0].InstallationKey ||
		observation.Binding.Reference != skill || observation.Binding.Kind != "skill" || observation.Source != "native-probe" {
		t.Fatalf("candidates = %#v, observation = %#v", candidates, observation)
	}
}

func singleSkillProvider(providerID, skill string) catalog.ProviderDescriptorRecord {
	return catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV3, DescriptorVersion: "3.0.0",
		ID: providerID, DisplayName: providerID,
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex-skill", Hosts: []string{"codex"}, Surface: "codex-plugin", Distribution: "fixture",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/plugins/provider", EvidencePath: "provider.marker",
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "delivery", InputSchema: "oaw.capability-input/v1", OutcomeSchema: "oaw.capability-outcome/v1",
			MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{"implementation"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, DelegationAllowList: []string{},
			HostBindings: []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: skill, Topologies: []execution.Topology{execution.TopologyCurrent}}},
		}},
	}
}

func assertECCFullSkillCoverage(t *testing.T) {
	t.Helper()
	available, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	var descriptor catalog.ProviderDescriptorRecord
	for _, value := range available.Providers() {
		if value.ID == "oaw/ecc" {
			descriptor = value
			break
		}
	}
	var recipe catalog.ProfileRecipeRecord
	for _, value := range available.Recipes() {
		if value.ID == "oaw/ecc-engineering" {
			recipe = value
			break
		}
	}
	if descriptor.ID == "" || recipe.ID == "" {
		t.Fatal("built-in ECC Provider or recipe is missing")
	}
	// Bridge v1 cannot verify the built-in Agent bindings. Clone the full
	// lifecycle with Skill bindings to test neutrality without promoting them.
	descriptor.ID = "fixture/ecc"
	descriptor.DisplayName = "ECC Skill Fixture"
	descriptor.Discovery = []catalog.DiscoveryProbe{{
		ID: "codex-skills", Hosts: []string{"codex"}, Surface: "codex-user-skills", Distribution: "ecc-fixture",
		Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/skills/ecc-fixture", EvidencePath: "SKILL.md",
	}}
	for index := range descriptor.Capabilities {
		capability := &descriptor.Capabilities[index]
		reference := "ecc:" + capability.ID
		if capability.ID == "tdd" {
			reference = "tdd-workflow"
		}
		capability.HostBindings = []catalog.HostBinding{{
			Host: "codex", Kind: "skill", Reference: reference, Topologies: []execution.Topology{execution.TopologyCurrent},
		}}
		capability.SupportedTopologies = []execution.Topology{execution.TopologyCurrent}
	}
	recipe.ID = "fixture/ecc-full"
	recipe.DisplayName = "ECC Full Skill Fixture"
	for index := range recipe.Nodes {
		recipe.Nodes[index].Selector.ProviderID = descriptor.ID
	}
	fixtureCatalog, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor}, []catalog.ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	providerRoot := filepath.Join(home, ".agents", "skills", "ecc-fixture")
	writeBridgeFixtureFile(t, filepath.Join(providerRoot, "SKILL.md"), "---\nname: ecc-fixture\n---\n")
	metadata := appserver.MetadataObservation{Skills: appserver.SkillsEntry{CWD: home, Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{}}}
	for _, capability := range descriptor.Capabilities {
		reference := capability.HostBindings[0].Reference
		path := filepath.Join(providerRoot, "capabilities", capability.ID, "SKILL.md")
		writeBridgeFixtureFile(t, path, "---\nname: "+reference+"\n---\n")
		metadata.Skills.Skills = append(metadata.Skills.Skills, appserver.SkillMetadata{Name: reference, Enabled: true, Path: path, Scope: "user"})
	}
	report, err := discovery.Discover(fixtureCatalog, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	fullInventory, diagnostics, err := codexbridge.BuildBindingInventory(fixtureCatalog, report, metadata, home)
	if err != nil || len(diagnostics) != 0 || len(fullInventory.Observations) != len(descriptor.Capabilities) {
		t.Fatalf("inventory = %#v, diagnostics = %#v, error = %v", fullInventory, diagnostics, err)
	}
	fullRegistry := bridgeFixtureRegistry(descriptor, fullInventory)
	request := profile.CompileRequest{Profile: recipe.ID, HostTopologies: []execution.Topology{execution.TopologyCurrent}, EnvironmentObservations: []execution.EnvironmentObservation{}}
	if _, err := profile.CompileProfile(fixtureCatalog, fullRegistry, request); err != nil {
		t.Fatalf("complete ECC Skill fixture did not compile: %v", err)
	}

	missing := metadata
	missing.Skills.Skills = slices.DeleteFunc(append([]appserver.SkillMetadata{}, metadata.Skills.Skills...), func(value appserver.SkillMetadata) bool {
		return value.Name == "tdd-workflow"
	})
	partialInventory, diagnostics, err := codexbridge.BuildBindingInventory(fixtureCatalog, report, missing, home)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("partial inventory = %#v, diagnostics = %#v, error = %v", partialInventory, diagnostics, err)
	}
	_, err = profile.CompileProfile(fixtureCatalog, bridgeFixtureRegistry(descriptor, partialInventory), request)
	requireCode(t, err, "PROFILE_CAPABILITY_MISSING")
}

type bridgeRegistryFixture struct {
	provider     registry.ProviderInstance
	capabilities map[string]registry.VerifiedCapability
}

func (value bridgeRegistryFixture) HostID() string { return "codex" }

func (value bridgeRegistryFixture) Provider(id string) (registry.ProviderInstance, bool) {
	return value.provider, id == value.provider.ProviderID
}

func (value bridgeRegistryFixture) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	valueCapability, found := value.capabilities[providerID+"\x00"+capabilityID]
	return valueCapability, found
}

func bridgeFixtureRegistry(descriptor catalog.ProviderDescriptorRecord, inventory host.BindingInventory) bridgeRegistryFixture {
	observed := make(map[string]host.BindingObservation, len(inventory.Observations))
	for _, observation := range inventory.Observations {
		observed[observation.Binding.Reference] = observation
	}
	result := bridgeRegistryFixture{
		provider:     registry.ProviderInstance{ProviderID: descriptor.ID, HostID: "codex"},
		capabilities: make(map[string]registry.VerifiedCapability),
	}
	for _, capability := range descriptor.Capabilities {
		binding := capability.HostBindings[0]
		observation, found := observed[binding.Reference]
		if !found {
			continue
		}
		verified := registry.VerifiedCapability{
			ID: capability.ID, Binding: observation.Binding,
			SupportedTopologies: append([]execution.Topology{}, observation.Topologies...), BindingEvidenceDigest: observation.Digest,
		}
		result.provider.Capabilities = append(result.provider.Capabilities, verified)
		result.capabilities[descriptor.ID+"\x00"+capability.ID] = verified
	}
	result.provider.Digest = canonicaljson.DigestBytes([]byte(descriptor.ID + "\x00" + inventory.Digest))
	return result
}

func loadPolicyAndNativeCodexSnapshot(t *testing.T) config.Snapshot {
	t.Helper()
	snapshot, err := config.Load(config.LoadOptions{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func integrationCanSupplyInventory(snapshot config.Snapshot, id string) bool {
	for _, integration := range snapshot.HostIntegrations() {
		if integration.ID != id {
			continue
		}
		return integration.Manifest.ControlSurface == host.SurfaceHostNative &&
			slices.Contains(integration.Manifest.BindingKinds, "skill") &&
			slices.Contains(integration.Manifest.Features, host.FeatureProviderBindingInventory) &&
			integration.Audit.Status == host.AuditPassed && integration.Conformance != nil
	}
	return false
}

func writeBridgeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
