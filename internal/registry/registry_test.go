package registry_test

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
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/host/codex"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestResolveReportsEveryProviderState(t *testing.T) {
	verificationBinding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "superpowers:verification-before-completion"}
	cases := []struct {
		name       string
		userConfig string
		setupHome  func(*testing.T, string)
		inventory  string
		state      registry.ProviderState
		reason     string
	}{
		{"not found", "", nil, "none", registry.NotFound, "PROVIDER_NOT_FOUND"},
		{"candidate", "", writeSuperpowersDirect, "none", registry.CandidateState, "HOST_BINDING_EVIDENCE_REQUIRED"},
		{"verified", "", writeSuperpowersDirect, "verified", registry.Verified, "PROVIDER_VERIFIED"},
		{"ambiguous", "", writeSuperpowersDirectAndVersion, "none", registry.Ambiguous, "PROVIDER_CANDIDATE_AMBIGUOUS"},
		{"incompatible", `
schema_version = "oaw.user-config/v2"
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-incompatible"
evidence_digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
version = "9.9.9"
`, writeSuperpowersDirect, "none", registry.Incompatible, "PROVIDER_PIN_INCOMPATIBLE"},
		{"binding unavailable", "", writeSuperpowersDirect, "empty", registry.CandidateState, "HOST_BINDING_EVIDENCE_REQUIRED"},
		{"disabled", `
schema_version = "oaw.user-config/v2"
denied_providers = ["oaw/superpowers"]
`, writeSuperpowersDirect, "verified", registry.Disabled, "PROVIDER_DISABLED_BY_USER"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, evidence := builtInInputs(t, tt.userConfig, tt.setupHome)
			var inventory *host.BindingInventory
			if tt.inventory == "verified" {
				inventory = inventoryForCandidate(t, evidence.Candidates("oaw/superpowers"), verificationBinding)
			} else if tt.inventory == "empty" {
				empty, err := host.NewBindingInventory("codex", nil)
				if err != nil {
					t.Fatal(err)
				}
				inventory = &empty
			}
			report, effective, err := registry.Resolve(snapshot, "codex", evidence, inventory)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			resolution := requireResolution(t, report, "oaw/superpowers")
			if resolution.State != tt.state || resolution.Reason != tt.reason {
				t.Fatalf("resolution = %#v", resolution)
			}
			_, admitted := effective.Provider("oaw/superpowers")
			if admitted != (tt.state == registry.Verified) {
				t.Fatalf("registry admitted=%v for state %q", admitted, tt.state)
			}
		})
	}
}

func TestResolveRequiresExactHostInstallationInventory(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "---\nname: using-superpowers\n---\n")
	reviewPath := writeFile(t, home, ".codex/plugins/superpowers/skills/requesting-code-review/SKILL.md", "---\nname: requesting-code-review\n---\n")
	snapshot, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := codex.ObserveBindings(snapshot.Catalog(), discovered, codex.InventoryOptions{UserHome: home, CodexConfigRoot: filepath.Join(home, ".codex")})
	if err != nil {
		t.Fatal(err)
	}
	report, effective, err := registry.Resolve(snapshot, "codex", discovered, &inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireResolution(t, report, "oaw/superpowers")
	if report.HostID() != "codex" || effective.HostID() != "codex" || resolution.Instance == nil || resolution.Instance.HostID != "codex" || resolution.Instance.InstallationKey == "" || resolution.Instance.BindingInventoryDigest != inventory.Digest {
		t.Fatalf("Host-scoped resolution = %#v / %#v", resolution, effective)
	}
	capability, found := effective.Capability("oaw/superpowers", "review")
	if !found || capability.BindingEvidenceDigest == "" {
		t.Fatalf("verified Capability = %#v, %v", capability, found)
	}

	if _, _, err := registry.Resolve(snapshot, "claude", discovered, &inventory); err == nil || !strings.Contains(err.Error(), "HOST_PROVIDER_SCOPE_MISMATCH") {
		t.Fatalf("foreign Host Resolve() error = %v", err)
	}
	empty, err := host.NewBindingInventory("codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, current := range map[string]*host.BindingInventory{"nil": nil, "empty": &empty} {
		t.Run(name, func(t *testing.T) {
			report, _, err := registry.Resolve(snapshot, "codex", discovered, current)
			if err != nil {
				t.Fatal(err)
			}
			resolution := requireResolution(t, report, "oaw/superpowers")
			if resolution.State != registry.CandidateState || resolution.Reason != "HOST_BINDING_EVIDENCE_REQUIRED" {
				t.Fatalf("resolution = %#v", resolution)
			}
		})
	}
	candidates := discovered.Candidates("oaw/superpowers")
	wrongInstallation, err := host.NewBindingInventory("codex", []host.BindingObservation{{
		HostID: "codex", InstallationKey: "installation-wrong",
		Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "superpowers:requesting-code-review"},
		Source:  "host-filesystem", EvidenceReference: reviewPath, Digest: strings.Repeat("a", 64),
	}})
	if err != nil {
		t.Fatal(err)
	}
	report, _, err = registry.Resolve(snapshot, "codex", discovered, &wrongInstallation)
	if err != nil {
		t.Fatal(err)
	}
	if resolution = requireResolution(t, report, "oaw/superpowers"); resolution.State != registry.BindingUnavailable || resolution.Reason != "PROVIDER_BINDING_UNAVAILABLE" || resolution.Candidates[0].InstallationKey != candidates[0].InstallationKey {
		t.Fatalf("wrong Installation resolution = %#v", resolution)
	}
}

func TestResolveReportsUntrustedAndUserDenyWins(t *testing.T) {
	projectRoot := untrustedProject(t)
	for _, tt := range []struct {
		name       string
		userConfig string
		state      registry.ProviderState
		reason     string
	}{
		{"untrusted", "", registry.Untrusted, "PROVIDER_PROJECT_CONTENT_UNTRUSTED"},
		{"denied", "schema_version = \"oaw.user-config/v2\"\ndenied_providers = [\"acme/suite\"]\n", registry.Disabled, "PROVIDER_DISABLED_BY_USER"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			userRoot := writeUserConfig(t, tt.userConfig)
			snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
			if err != nil {
				t.Fatal(err)
			}
			evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			report, effective, err := registry.Resolve(snapshot, "codex", evidence, nil)
			if err != nil {
				t.Fatal(err)
			}
			resolution := requireResolution(t, report, "acme/suite")
			if resolution.State != tt.state || resolution.Reason != tt.reason {
				t.Fatalf("resolution = %#v", resolution)
			}
			if _, found := effective.Provider("acme/suite"); found {
				t.Fatal("unverified provider entered registry")
			}
		})
	}
}

func TestResolveUsesExactPinToDisambiguate(t *testing.T) {
	home := t.TempDir()
	writeSuperpowersDirectAndVersion(t, home)
	baseSnapshot, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	baseEvidence, err := discovery.Discover(baseSnapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	var pinned discovery.Candidate
	for _, candidate := range baseEvidence.Candidates("oaw/superpowers") {
		if candidate.Version == "1.2.3" {
			pinned = candidate
			break
		}
	}
	if pinned.InstallationKey == "" {
		t.Fatal("versioned candidate was not discovered")
	}
	versionLocation, err := filepath.EvalSymlinks(filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "1.2.3"))
	if err != nil {
		t.Fatal(err)
	}
	userConfig := fmt.Sprintf(`
schema_version = "oaw.user-config/v2"
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = %q
evidence_digest = %q
location = %q
version = "1.2.3"
`, pinned.InstallationKey, pinned.EvidenceDigest, versionLocation)
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: writeUserConfig(t, userConfig)})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := inventoryForCandidate(t, []discovery.Candidate{pinned}, catalog.HostBinding{
		Host: "codex", Kind: "skill", Reference: "superpowers:verification-before-completion",
	})
	report, effective, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireResolution(t, report, "oaw/superpowers")
	if resolution.State != registry.Verified || resolution.Instance == nil || resolution.Instance.Version != "1.2.3" {
		t.Fatalf("resolution = %#v", resolution)
	}
	if resolution.Instance.Location != versionLocation {
		t.Fatalf("instance location = %q, want %q", resolution.Instance.Location, versionLocation)
	}
	if provider, found := effective.Provider("oaw/superpowers"); !found || provider.Digest != resolution.Instance.Digest {
		t.Fatalf("registry provider = %#v, %v", provider, found)
	}
}

func TestResolveSeesOnlySelectedHostCandidates(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".claude/plugins/cache/claude-plugins-official/superpowers/6.0.3/skills/using-superpowers/SKILL.md", "claude-6.0.3")
	writeFile(t, home, ".claude/plugins/cache/claude-plugins-official/superpowers/6.1.1/skills/using-superpowers/SKILL.md", "claude-6.1.1")
	writeFile(t, home, ".codex/plugins/cache/openai-api-curated/superpowers/11c74d6b/skills/using-superpowers/SKILL.md", "codex-11c74d6b")

	snapshot, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := inventoryForCandidate(t, evidence.Candidates("oaw/superpowers"), catalog.HostBinding{
		Host: "codex", Kind: "skill", Reference: "superpowers:requesting-code-review",
	})
	report, effective, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireResolution(t, report, "oaw/superpowers")
	if resolution.State != registry.Verified || resolution.Reason != "PROVIDER_VERIFIED" {
		t.Fatalf("resolution = %#v", resolution)
	}
	versions := make([]string, len(resolution.Candidates))
	for index, candidate := range resolution.Candidates {
		versions[index] = candidate.Version
	}
	if got := fmt.Sprint(versions); got != "[11c74d6b]" {
		t.Fatalf("candidate versions = %s", got)
	}
	if _, found := effective.Provider("oaw/superpowers"); !found {
		t.Fatal("selected-Host Provider did not enter the Effective Registry")
	}
}

func TestResolveAppliesPreferencesLimitsAndPartialCapabilities(t *testing.T) {
	snapshot := customSnapshot(t, true, true)
	home := t.TempDir()
	writeFile(t, home, ".agents/acme/SKILL.md", "acme")
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := inventoryForCandidate(t, evidence.Candidates("acme/suite"),
		catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:alpha-review"},
		catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:zeta-review"},
		catalog.HostBinding{Host: "codex", Kind: "tool", Reference: "acme:verify"},
	)
	firstReport, firstRegistry, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	secondReport, secondRegistry, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if firstReport.Digest() == "" || firstReport.Digest() != secondReport.Digest() || firstRegistry.Digest() != secondRegistry.Digest() {
		t.Fatalf("digests report=%q/%q registry=%q/%q", firstReport.Digest(), secondReport.Digest(), firstRegistry.Digest(), secondRegistry.Digest())
	}
	resolution := requireResolution(t, firstReport, "acme/suite")
	if resolution.State != registry.Verified || resolution.Instance == nil || len(resolution.Instance.Capabilities) != 1 {
		t.Fatalf("resolution = %#v", resolution)
	}
	capability := resolution.Instance.Capabilities[0]
	if capability.ID != "review" || capability.Binding.Reference != "acme:zeta-review" {
		t.Fatalf("verified capability = %#v", capability)
	}
	if got, found := firstRegistry.Capability("acme/suite", "review"); !found || got.Binding.Reference != "acme:zeta-review" {
		t.Fatalf("Capability() = %#v, %v", got, found)
	}
	if _, found := firstRegistry.Capability("acme/suite", "verification"); found {
		t.Fatal("project capability limit was widened")
	}

	resolution.Instance.Capabilities[0].Binding.Reference = "changed"
	resolution.Candidates[0].Evidence[0].Path = "changed"
	provider, _ := firstRegistry.Provider("acme/suite")
	provider.Capabilities[0].Binding.Reference = "changed"
	freshResolution := requireResolution(t, firstReport, "acme/suite")
	freshProvider, _ := firstRegistry.Provider("acme/suite")
	if freshResolution.Instance.Capabilities[0].Binding.Reference != "acme:zeta-review" || freshResolution.Candidates[0].Evidence[0].Path == "changed" || freshProvider.Capabilities[0].Binding.Reference != "acme:zeta-review" {
		t.Fatal("report or registry exposed mutable storage")
	}
}

func TestResolveUsesDeterministicBindingFallbackAndVerifiedSubset(t *testing.T) {
	snapshot := customSnapshot(t, false, false)
	home := t.TempDir()
	writeFile(t, home, ".agents/acme/SKILL.md", "acme")
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := inventoryForCandidate(t, evidence.Candidates("acme/suite"),
		catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:zeta-review"},
		catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:alpha-review"},
	)
	report, effective, err := registry.Resolve(snapshot, "codex", evidence, inventory)
	if err != nil {
		t.Fatal(err)
	}
	resolution := requireResolution(t, report, "acme/suite")
	if resolution.State != registry.Verified || len(resolution.Instance.Capabilities) != 1 || resolution.Instance.Capabilities[0].Binding.Reference != "acme:alpha-review" {
		t.Fatalf("resolution = %#v", resolution)
	}
	providers := effective.Providers()
	if len(providers) != 1 || providers[0].ProviderID != "acme/suite" || effective.Digest() == "" {
		t.Fatalf("Providers() = %#v digest=%q", providers, effective.Digest())
	}
	if _, found := effective.Capability("acme/suite", "verification"); found {
		t.Fatal("unavailable capability entered registry")
	}
}

func requireResolution(t *testing.T, report registry.ResolutionReport, providerID string) registry.ProviderResolution {
	t.Helper()
	resolution, found := report.Resolution(providerID)
	if !found {
		t.Fatalf("Resolution(%q) not found in %#v", providerID, report.Resolutions())
	}
	return resolution
}

func builtInInputs(t *testing.T, userConfig string, setupHome func(*testing.T, string)) (config.Snapshot, discovery.Report) {
	t.Helper()
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: writeUserConfig(t, userConfig)})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if setupHome != nil {
		setupHome(t, home)
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, evidence
}

func writeSuperpowersDirect(t *testing.T, home string) {
	t.Helper()
	writeFile(t, home, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "direct")
}

func writeSuperpowersDirectAndVersion(t *testing.T, home string) {
	t.Helper()
	writeSuperpowersDirect(t, home)
	writeFile(t, home, ".codex/plugins/cache/openai-api-curated/superpowers/1.2.3/skills/using-superpowers/SKILL.md", "version")
}

func inventoryForCandidate(t *testing.T, candidates []discovery.Candidate, bindings ...catalog.HostBinding) *host.BindingInventory {
	t.Helper()
	if len(candidates) != 1 {
		t.Fatalf("inventoryForCandidate requires one Candidate, got %d", len(candidates))
	}
	candidate := candidates[0]
	observations := make([]host.BindingObservation, 0, len(bindings))
	for index, binding := range bindings {
		observations = append(observations, host.BindingObservation{
			HostID: "codex", InstallationKey: candidate.InstallationKey, Binding: binding,
			Source: "host-filesystem", EvidenceReference: filepath.Join(candidate.Location, fmt.Sprintf("evidence-%d", index)), Digest: strings.Repeat(string(rune('a'+index)), 64),
		})
	}
	inventory, err := host.NewBindingInventory("codex", observations)
	if err != nil {
		t.Fatal(err)
	}
	return &inventory
}

func untrustedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, ".oaw/providers/acme.toml", testProviderTOML)
	writeFile(t, root, ".oaw/config.toml", `
schema_version = "oaw.project-config/v1"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
`)
	return root
}

func customSnapshot(t *testing.T, preference, capabilityLimit bool) config.Snapshot {
	t.Helper()
	projectRoot := ""
	trust := ""
	if capabilityLimit {
		projectRoot = t.TempDir()
		writeFile(t, projectRoot, ".oaw/config.toml", `
schema_version = "oaw.project-config/v1"
[[capability_limits]]
provider_id = "acme/suite"
capability_ids = ["review"]
`)
		fingerprint, err := config.InspectProject(projectRoot, testSchemaRegistry(t))
		if err != nil {
			t.Fatal(err)
		}
		trust = fmt.Sprintf(`
[[project_trust]]
root = %q
config_digest = %q
descriptor_digests = []
recipe_digests = []
`, fingerprint.Root, fingerprint.ConfigDigest)
	}
	preferenceTOML := ""
	if preference {
		preferenceTOML = `
[[binding_preferences]]
provider_id = "acme/suite"
capability_id = "review"
host_id = "codex"
kind = "skill"
reference = "acme:zeta-review"
`
	}
	userRoot := t.TempDir()
	writeFile(t, userRoot, "providers/acme.toml", testProviderTOML)
	writeFile(t, userRoot, "config.toml", fmt.Sprintf(`
schema_version = "oaw.user-config/v2"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
%s
%s
`, preferenceTOML, trust))
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func writeUserConfig(t *testing.T, content string) string {
	t.Helper()
	if content == "" {
		return ""
	}
	root := t.TempDir()
	writeFile(t, root, "config.toml", content)
	return root
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
schema_version = "oaw.provider-descriptor/v2"
descriptor_version = "2.0.0"
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
executor_topology = "main-agent-allowed"
delegation_allow_list = []

[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:zeta-review"

[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:alpha-review"

[[capabilities]]
id = "verification"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
maximum_effects = ["read-project"]
resources = ["project"]
request_modes = ["BOUNDED", "WORKFLOW"]
responsibilities = ["verification"]
executor_topology = "main-agent-allowed"
delegation_allow_list = []

[[capabilities.host_bindings]]
host = "codex"
kind = "tool"
reference = "acme:verify"
`
