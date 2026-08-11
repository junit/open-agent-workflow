package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestTicket02VerticalSliceProducesImmutableEffectiveRegistry(t *testing.T) {
	home := t.TempDir()
	providerRoot := filepath.Join(home, ".agents", "acme")
	writeFile(t, providerRoot, "SKILL.md", "acme")
	skillRoot := filepath.Join(providerRoot, "skills", "zeta-review")
	writeFile(t, skillRoot, "SKILL.md", "---\nname: acme:zeta-review\n---\n")
	providerDocument := testProviderDocument(t, "acme/suite", digestIntegrationTree(t, skillRoot), digestIntegrationTree(t, providerRoot))
	snapshot, userRoot, providerPath, projectRoot := buildTrustedFixture(t, providerDocument)
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := integrationInventory(t, snapshot.Catalog(), evidence, map[string][]string{
		"acme/suite": {"codex-zeta-review"},
	})
	report, effective, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ProjectStatus() != config.ProjectTrusted || snapshot.ProjectReason() != "PROJECT_TRUST_VERIFIED" {
		t.Fatalf("project trust = %q, %q", snapshot.ProjectStatus(), snapshot.ProjectReason())
	}
	acme := requireResolution(t, report, "acme/suite")
	if acme.State != registry.ProviderVerified || acme.Instance == nil || len(acme.Instance.Capabilities) != 1 || acme.Instance.Capabilities[0].ID != "review" {
		t.Fatalf("acme resolution = %#v", acme)
	}
	if len(acme.Instance.Bindings) != 1 || acme.Instance.Bindings[0].Reference != "acme:zeta-review" ||
		len(acme.Instance.Capabilities[0].BindingIDs) != 1 || acme.Instance.Capabilities[0].BindingIDs[0] != "codex-zeta-review" {
		t.Fatalf("acme bindings = %#v / %#v", acme.Instance.Bindings, acme.Instance.Capabilities)
	}
	if matt := requireResolution(t, report, "oaw/matt"); matt.State != registry.ProviderDisabled {
		t.Fatalf("matt resolution = %#v", matt)
	}
	if superpowers := requireResolution(t, report, "oaw/superpowers"); superpowers.State != registry.ProviderNotFound {
		t.Fatalf("superpowers resolution = %#v", superpowers)
	}
	if _, found := effective.Provider("oaw/matt"); found {
		t.Fatal("denied Matt entered Effective Registry")
	}
	if len(snapshot.Catalog().Recipes()) != 5 || len(snapshot.Catalog().Aliases()) != 4 {
		t.Fatalf("catalog counts = %d recipes / %d aliases", len(snapshot.Catalog().Recipes()), len(snapshot.Catalog().Aliases()))
	}
	oldSnapshotDigest, oldRegistryDigest := snapshot.Digest(), effective.Digest()
	changed := strings.Replace(providerDocument, `"display_name":"Acme Suite"`, `"display_name":"Acme Suite Reloaded"`, 1)
	if err := os.WriteFile(providerPath, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if snapshot.Digest() != oldSnapshotDigest || effective.Digest() != oldRegistryDigest {
		t.Fatal("existing snapshot or registry changed after source mutation")
	}
	reloaded, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Digest() == oldSnapshotDigest {
		t.Fatal("reloaded snapshot did not change after descriptor mutation")
	}
}

func TestTicket02NegativeProjectDescriptorDriftBecomesUntrusted(t *testing.T) {
	projectRoot := t.TempDir()
	projectProvider := testProviderDocument(t, "acme/project", fixtureTreeDigest("a"), fixtureTreeDigest("b"))
	writeFile(t, projectRoot, ".oaw/providers/acme.json", projectProvider)
	writeFile(t, projectRoot, ".oaw/config.toml", `
schema_version = "oaw.project-config/v1"
[[provider_descriptors]]
id = "acme/project"
path = "providers/acme.json"
`)
	fingerprint, err := config.InspectProject(projectRoot, testSchemaRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	userRoot := t.TempDir()
	writeFile(t, userRoot, "config.toml", "schema_version = \"oaw.user-config/v3\"\n"+projectTrustConfig(fingerprint))
	trusted, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if trusted.ProjectStatus() != config.ProjectTrusted {
		t.Fatalf("initial project status = %q", trusted.ProjectStatus())
	}
	writeFile(t, projectRoot, ".oaw/providers/acme.json", strings.Replace(projectProvider, `"display_name":"Acme Suite"`, `"display_name":"Acme Drifted"`, 1))
	drifted, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if drifted.ProjectStatus() != config.ProjectUntrusted || drifted.ProjectReason() != "PROJECT_DESCRIPTOR_DIGEST_MISMATCH" {
		t.Fatalf("drifted project = %q, %q", drifted.ProjectStatus(), drifted.ProjectReason())
	}
	evidence, err := discovery.Discover(drifted.Catalog(), discovery.Options{HostID: "codex", UserHome: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	report, _, err := registry.Resolve(drifted, "codex", evidence, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolution := requireResolution(t, report, "acme/project"); resolution.State != registry.ProviderUntrusted {
		t.Fatalf("resolution = %#v", resolution)
	}
}

func TestTicket02NegativeDiscoveryAndResolutionStates(t *testing.T) {
	t.Run("escaping symlink", func(t *testing.T) {
		snapshot, _, _, _ := buildTrustedFixture(t, testProviderDocument(t, "acme/suite", fixtureTreeDigest("a"), fixtureTreeDigest("b")))
		home := t.TempDir()
		outside := writeFile(t, t.TempDir(), "SKILL.md", "outside")
		writeSymlink(t, home, ".agents/acme/SKILL.md", outside)
		if _, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_PATH_ESCAPE") {
			t.Fatalf("Discover() error = %v", err)
		}
	})

	t.Run("two unpinned versions", func(t *testing.T) {
		snapshot, err := config.Load(config.LoadOptions{})
		if err != nil {
			t.Fatal(err)
		}
		home := t.TempDir()
		writeFile(t, home, ".codex/plugins/cache/openai-api-curated/superpowers/1.0.0/skills/using-superpowers/SKILL.md", "one")
		writeFile(t, home, ".codex/plugins/cache/openai-api-curated/superpowers/2.0.0/skills/using-superpowers/SKILL.md", "two")
		evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
		if err != nil {
			t.Fatal(err)
		}
		report, _, err := registry.Resolve(snapshot, "codex", evidence, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resolution := requireResolution(t, report, "oaw/superpowers"); resolution.State != registry.ProviderAmbiguous {
			t.Fatalf("resolution = %#v", resolution)
		}
	})

	t.Run("inventory lacks declared bindings", func(t *testing.T) {
		snapshot, _, _, _ := buildTrustedFixture(t, testProviderDocument(t, "acme/suite", fixtureTreeDigest("a"), fixtureTreeDigest("b")))
		home := t.TempDir()
		writeFile(t, home, ".agents/acme/SKILL.md", "acme")
		evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
		if err != nil {
			t.Fatal(err)
		}
		empty, err := host.BuildBindingInventoryV3("codex", nil)
		if err != nil {
			t.Fatal(err)
		}
		report, _, err := registry.Resolve(snapshot, "codex", evidence, &empty)
		if err != nil {
			t.Fatal(err)
		}
		if resolution := requireResolution(t, report, "acme/suite"); resolution.State != registry.ProviderCandidate {
			t.Fatalf("resolution = %#v", resolution)
		}
	})
}

func buildTrustedFixture(t *testing.T, providerDocument string) (config.Snapshot, string, string, string) {
	t.Helper()
	projectRoot := t.TempDir()
	writeFile(t, projectRoot, ".oaw/config.toml", `
schema_version = "oaw.project-config/v1"
required_providers = ["acme/suite"]
recommended_providers = ["oaw/superpowers"]
[[capability_limits]]
provider_id = "acme/suite"
capability_ids = ["review"]
`)
	fingerprint, err := config.InspectProject(projectRoot, testSchemaRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	userRoot := t.TempDir()
	providerPath := writeFile(t, userRoot, "providers/acme.json", providerDocument)
	writeFile(t, userRoot, "profiles/review.json", testRecipeDocument(t))
	writeFile(t, userRoot, "config.toml", fmt.Sprintf(`
schema_version = "oaw.user-config/v3"
denied_providers = ["oaw/matt"]
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.json"
[[profile_recipes]]
id = "acme/review"
path = "profiles/review.json"
%s
`, projectTrustConfig(fingerprint)))
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, userRoot, providerPath, projectRoot
}

func projectTrustConfig(fingerprint config.ProjectFingerprint) string {
	return fmt.Sprintf(`[[project_trust]]
root = %q
config_digest = %q
descriptor_digests = [%s]
recipe_digests = [%s]
`, fingerprint.Root, fingerprint.ConfigDigest, quotedStrings(fingerprint.DescriptorDigests), quotedStrings(fingerprint.RecipeDigests))
}

func quotedStrings(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return strings.Join(quoted, ",")
}

func requireResolution(t *testing.T, report registry.ResolutionReport, providerID string) registry.ProviderResolution {
	t.Helper()
	resolution, found := report.Resolution(providerID)
	if !found {
		t.Fatalf("Resolution(%q) not found", providerID)
	}
	return resolution
}

func integrationInventory(t *testing.T, available catalog.Catalog, evidence discovery.Report, bindingIDs map[string][]string) *host.BindingInventory {
	t.Helper()
	providers := make(map[string]catalog.ProviderDescriptorRecord)
	for _, provider := range available.Providers() {
		providers[provider.ID] = provider
	}
	observations := make([]host.BindingObservation, 0)
	for providerID, values := range bindingIDs {
		candidates := evidence.Candidates(providerID)
		if len(candidates) != 1 {
			t.Fatalf("provider %s candidates = %d, want one", providerID, len(candidates))
		}
		provider, found := providers[providerID]
		if !found {
			t.Fatalf("provider %s missing from Catalog", providerID)
		}
		for _, bindingID := range values {
			var binding catalog.BindingRecord
			for _, candidate := range provider.Bindings {
				if candidate.ID == bindingID {
					binding = candidate
					break
				}
			}
			if binding.ID == "" {
				t.Fatalf("binding %s/%s missing from Catalog", providerID, bindingID)
			}
			observations = append(observations, host.BindingObservation{
				HostID: "codex", ProviderID: providerID, InstallationKey: candidates[0].InstallationKey,
				DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface,
				Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
				Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceLiveFilesystem,
				EvidenceReference: "evidence://integration/" + strings.ReplaceAll(providerID, "/", "-") + "/" + binding.ID,
			})
		}
	}
	inventory, err := host.BuildBindingInventoryV3("codex", observations)
	if err != nil {
		t.Fatal(err)
	}
	return &inventory
}

func digestIntegrationTree(t *testing.T, path string) string {
	t.Helper()
	tree, err := integrity.DigestTree(path)
	if err != nil {
		t.Fatal(err)
	}
	return tree.RootDigest
}

func writeSymlink(t *testing.T, root, relative, target string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, root, relative, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func testSchemaRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	value, err := schema.New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fixtureTreeDigest(character string) string {
	return "sha256:" + strings.Repeat(character, 64)
}

func testProviderDocument(t *testing.T, providerID, bindingTree, distributionTree string) string {
	t.Helper()
	definitions := catalog.CanonicalSlots()
	claims := make([]catalog.ResponsibilityClaim, 0, len(definitions))
	stageSpan := make([]catalog.SlotID, 0, len(definitions))
	for _, definition := range definitions {
		claims = append(claims, catalog.ResponsibilityClaim{
			Namespace: catalog.OwnershipStage, Name: string(definition.ID), SlotID: definition.ID, OutcomeOwner: true,
		})
		stageSpan = append(stageSpan, definition.ID)
	}
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: providerID, DisplayName: "Acme Suite",
		Distributions: []catalog.DistributionRecord{{
			ID: "acme", SourceURI: "https://example.test/acme/suite", Revision: strings.Repeat("a", 40), TreeDigest: distributionTree,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "acme-skill", Hosts: []string{"codex"}, Surface: "codex-user-skills", DistributionID: "acme",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/acme", EvidencePath: "SKILL.md",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-zeta-review", DistributionID: "acme", ContentRoot: "skills/zeta-review", InstallRoot: "skills/zeta-review", TreeDigest: bindingTree,
			Host: "codex", Surface: "codex-user-skills", Kind: catalog.BindingSkill, Reference: "acme:zeta-review", Invocation: catalog.InvocationModel,
			Responsibilities: claims, InputArtifact: "oaw.workflow-artifact/v1", OutputArtifact: "oaw.workflow-artifact/v1",
			MaximumEffects: []string{"read-project"}, Resources: []string{"project"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
			Delegation: catalog.DelegationRequirements{}, StageSpan: stageSpan, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "review", InputSchema: "oaw.capability-input/v1", OutcomeSchema: "oaw.capability-outcome/v1",
			RequestModes: []catalog.RequestMode{catalog.RequestModeBounded, catalog.RequestModeWorkflow}, BindingRefs: []string{"codex-zeta-review"},
		}},
	}
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func testRecipeDocument(t *testing.T) string {
	t.Helper()
	available, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	var recipe catalog.ProfileRecipeRecord
	for _, candidate := range available.Recipes() {
		if candidate.ID == "oaw/delivery" {
			recipe = candidate
			break
		}
	}
	if recipe.ID == "" {
		t.Fatal("built-in delivery Recipe missing")
	}
	recipe.ID = "acme/review"
	recipe.DisplayName = "Acme Review"
	recipe.Family = "user-defined"
	recipe.Template = ""
	recipe.AddOns = []catalog.AddOnRecord{}
	recipe.IncidentRoutes = []catalog.IncidentRoute{}
	recipe.Overlays = []catalog.OverlayRecord{}
	for index := range recipe.Slots {
		slotID := recipe.Slots[index].SlotID
		stepID := "acme-" + string(slotID)
		recipe.Slots[index].Pipeline = []catalog.PipelineStep{{
			ID: stepID, Selector: catalog.BindingSelector{ProviderID: "acme/suite", BindingID: "codex-zeta-review"}, StageSpan: []catalog.SlotID{slotID},
			RequiredInputArtifact: "oaw.workflow-artifact/v1", ProducedOutputArtifact: "oaw.workflow-artifact/v1",
		}}
		recipe.Slots[index].OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: stepID}
		recipe.Slots[index].HostAction = nil
	}
	raw, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
