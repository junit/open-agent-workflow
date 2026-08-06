package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestTicket02VerticalSliceProducesImmutableEffectiveRegistry(t *testing.T) {
	snapshot, userRoot, providerPath, projectRoot := buildTrustedFixture(t)
	home := t.TempDir()
	writeFile(t, home, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "superpowers")
	writeFile(t, home, ".agents/skills/to-spec/SKILL.md", "matt")
	writeFile(t, home, ".agents/acme/SKILL.md", "acme")
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := integrationInventory(t, evidence, map[string][]catalog.HostBinding{
		"oaw/superpowers": {{Host: "codex", Kind: "skill", Reference: "superpowers:verification-before-completion"}},
		"acme/suite":      {{Host: "codex", Kind: "skill", Reference: "acme:zeta-review"}},
	})
	report, effective, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ProjectStatus() != config.ProjectTrusted || snapshot.ProjectReason() != "PROJECT_TRUST_VERIFIED" {
		t.Fatalf("project trust = %q, %q", snapshot.ProjectStatus(), snapshot.ProjectReason())
	}
	acme := requireResolution(t, report, "acme/suite")
	if acme.State != registry.Verified || acme.Instance == nil || len(acme.Instance.Capabilities) != 1 || acme.Instance.Capabilities[0].ID != "review" {
		t.Fatalf("acme resolution = %#v", acme)
	}
	if acme.Instance.Capabilities[0].Binding.Reference != "acme:zeta-review" {
		t.Fatalf("acme binding = %#v", acme.Instance.Capabilities[0].Binding)
	}
	if matt := requireResolution(t, report, "oaw/matt"); matt.State != registry.Disabled {
		t.Fatalf("matt resolution = %#v", matt)
	}
	if superpowers := requireResolution(t, report, "oaw/superpowers"); superpowers.State != registry.Verified || len(superpowers.Instance.Capabilities) != 1 {
		t.Fatalf("superpowers resolution = %#v", superpowers)
	}
	if _, found := effective.Provider("oaw/matt"); found {
		t.Fatal("denied Matt entered Effective Registry")
	}
	if len(snapshot.Catalog().Recipes()) != 6 || len(snapshot.Catalog().Aliases()) != 4 {
		t.Fatalf("catalog counts = %d recipes / %d aliases", len(snapshot.Catalog().Recipes()), len(snapshot.Catalog().Aliases()))
	}
	oldSnapshotDigest, oldRegistryDigest := snapshot.Digest(), effective.Digest()
	changed := strings.Replace(testProviderTOML, `display_name = "Acme Suite"`, `display_name = "Acme Suite Reloaded"`, 1)
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
	projectProvider := strings.Replace(testProviderTOML, `id = "acme/suite"`, `id = "acme/project"`, 1)
	writeFile(t, projectRoot, ".oaw/providers/acme.toml", projectProvider)
	writeFile(t, projectRoot, ".oaw/config.toml", `
schema_version = "oaw.project-config/v1"
[[provider_descriptors]]
id = "acme/project"
path = "providers/acme.toml"
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
	writeFile(t, projectRoot, ".oaw/providers/acme.toml", strings.Replace(projectProvider, `display_name = "Acme Suite"`, `display_name = "Acme Drifted"`, 1))
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
	if resolution := requireResolution(t, report, "acme/project"); resolution.State != registry.Untrusted {
		t.Fatalf("resolution = %#v", resolution)
	}
}

func TestTicket02NegativeDiscoveryAndResolutionStates(t *testing.T) {
	t.Run("escaping symlink", func(t *testing.T) {
		snapshot, _, _, _ := buildTrustedFixture(t)
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
		if resolution := requireResolution(t, report, "oaw/superpowers"); resolution.State != registry.Ambiguous {
			t.Fatalf("resolution = %#v", resolution)
		}
	})

	t.Run("inventory lacks declared bindings", func(t *testing.T) {
		snapshot, _, _, _ := buildTrustedFixture(t)
		home := t.TempDir()
		writeFile(t, home, ".agents/acme/SKILL.md", "acme")
		evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
		if err != nil {
			t.Fatal(err)
		}
		empty, err := host.NewBindingInventory("codex", nil)
		if err != nil {
			t.Fatal(err)
		}
		report, _, err := registry.Resolve(snapshot, "codex", evidence, &empty)
		if err != nil {
			t.Fatal(err)
		}
		if resolution := requireResolution(t, report, "acme/suite"); resolution.State != registry.CandidateState {
			t.Fatalf("resolution = %#v", resolution)
		}
	})
}

func buildTrustedFixture(t *testing.T) (config.Snapshot, string, string, string) {
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
	providerPath := writeFile(t, userRoot, "providers/acme.toml", testProviderTOML)
	writeFile(t, userRoot, "profiles/review.toml", testRecipeTOML)
	writeFile(t, userRoot, "config.toml", fmt.Sprintf(`
schema_version = "oaw.user-config/v3"
denied_providers = ["oaw/matt"]
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
[[profile_recipes]]
id = "acme/review"
path = "profiles/review.toml"
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

func integrationInventory(t *testing.T, evidence discovery.Report, bindings map[string][]catalog.HostBinding) *host.BindingInventory {
	t.Helper()
	observations := make([]host.BindingObservation, 0)
	for providerID, values := range bindings {
		candidates := evidence.Candidates(providerID)
		if len(candidates) != 1 {
			t.Fatalf("provider %s candidates = %d, want one", providerID, len(candidates))
		}
		for index, binding := range values {
			if len(binding.Topologies) == 0 {
				binding.Topologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
			}
			observations = append(observations, host.BindingObservation{
				HostID: "codex", InstallationKey: candidates[0].InstallationKey, Binding: binding,
				Topologies: []execution.Topology{execution.TopologyCurrent}, Source: "host-filesystem",
				EvidenceReference: filepath.Join(candidates[0].Location, fmt.Sprintf("evidence-%d", index)), Digest: strings.Repeat("a", 64),
			})
		}
	}
	inventory, err := host.NewBindingInventory("codex", observations)
	if err != nil {
		t.Fatal(err)
	}
	return &inventory
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

const testProviderTOML = `
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
request_modes = ["BOUNDED", "WORKFLOW"]
responsibilities = ["review"]
supported_topologies = ["CURRENT", "SUBAGENT"]
delegation_allow_list = []

[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:zeta-review"
topologies = ["CURRENT", "SUBAGENT"]

[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:alpha-review"
topologies = ["CURRENT", "SUBAGENT"]

[[capabilities]]
id = "verification"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
maximum_effects = ["read-project"]
resources = ["project"]
request_modes = ["BOUNDED", "WORKFLOW"]
responsibilities = ["verification"]
supported_topologies = ["CURRENT", "SUBAGENT"]
delegation_allow_list = []

[[capabilities.host_bindings]]
host = "codex"
kind = "tool"
reference = "acme:verify"
topologies = ["CURRENT", "SUBAGENT"]
`

const testRecipeTOML = `
schema_version = "oaw.profile-recipe/v2"
recipe_version = "2.0.0"
id = "acme/review"
display_name = "Acme Review"
required_responsibilities = ["review"]
incident_routes = []
entry = "review"
terminal_gates = ["review"]
stable_boundaries = ["complete"]
environment_requirements = []

[[nodes]]
id = "review"
kind = "gate"
responsibility = "review"
transitions = []

[nodes.selector]
provider_id = "acme/suite"
capability_id = "review"
`
