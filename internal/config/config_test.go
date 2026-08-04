package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestDecodeUserRejectsUnknownFields(t *testing.T) {
	registry := testRegistry(t)
	_, err := DecodeUser([]byte("schema_version = \"oaw.user-config/v2\"\nunknown = true\n"), registry)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_UNKNOWN_FIELD") {
		t.Fatalf("DecodeUser() error = %v", err)
	}
}

func TestDecodeUserNormalizesEquivalentTOML(t *testing.T) {
	registry := testRegistry(t)
	first := []byte(`
schema_version = "oaw.user-config/v2"
denied_providers = ["zeta/suite", "acme/suite"]

[[provider_pins]]
provider_id = "zeta/suite"
host_id = "codex"
installation_key = "installation-zeta"
evidence_digest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
version = "2.0.0"

[[provider_pins]]
provider_id = "acme/suite"
host_id = "codex"
installation_key = "installation-acme"
evidence_digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
location = "/opt/acme"
`)
	second := []byte(`schema_version="oaw.user-config/v2"
denied_providers=["acme/suite","zeta/suite"]
[[provider_pins]]
provider_id="acme/suite"
host_id="codex"
installation_key="installation-acme"
evidence_digest="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
location="/opt/acme"
[[provider_pins]]
provider_id="zeta/suite"
host_id="codex"
installation_key="installation-zeta"
evidence_digest="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
version="2.0.0"
`)
	left, err := DecodeUser(first, registry)
	if err != nil {
		t.Fatalf("DecodeUser(first) error = %v", err)
	}
	right, err := DecodeUser(second, registry)
	if err != nil {
		t.Fatalf("DecodeUser(second) error = %v", err)
	}
	if left.Digest != right.Digest || !bytes.Equal(left.CanonicalJSON, right.CanonicalJSON) {
		t.Fatalf("equivalent TOML differs:\n%s\n%s", left.CanonicalJSON, right.CanonicalJSON)
	}
	if got := left.Record.DeniedProviders; len(got) != 2 || got[0] != "acme/suite" || got[1] != "zeta/suite" {
		t.Fatalf("DeniedProviders = %#v", got)
	}
}

func TestDecodeProjectRejectsAuthorityFields(t *testing.T) {
	registry := testRegistry(t)
	for _, field := range []string{"enabled_providers = [\"acme/suite\"]", "authority = \"write\"", "binding = \"agent\"", "host_integrations = []", "runtime_host = \"codex\""} {
		raw := []byte("schema_version = \"oaw.project-config/v1\"\n" + field + "\n")
		if _, err := DecodeProject(raw, registry); err == nil || !strings.Contains(err.Error(), "CONFIG_UNKNOWN_FIELD") {
			t.Fatalf("DecodeProject(%q) error = %v", field, err)
		}
	}
}

func TestDecodeProjectNormalizesCapabilityLimits(t *testing.T) {
	registry := testRegistry(t)
	raw := []byte(`
schema_version = "oaw.project-config/v1"
required_providers = ["zeta/suite", "acme/suite"]
recommended_providers = ["oaw/superpowers"]

[[capability_limits]]
provider_id = "zeta/suite"
capability_ids = ["verification", "review"]

[[capability_limits]]
provider_id = "acme/suite"
capability_ids = ["review"]
`)
	decoded, err := DecodeProject(raw, registry)
	if err != nil {
		t.Fatalf("DecodeProject() error = %v", err)
	}
	if got := decoded.Record.RequiredProviders; len(got) != 2 || got[0] != "acme/suite" || got[1] != "zeta/suite" {
		t.Fatalf("RequiredProviders = %#v", got)
	}
	limits := decoded.Record.CapabilityLimits
	if len(limits) != 2 || limits[0].ProviderID != "acme/suite" || limits[1].CapabilityIDs[0] != "review" {
		t.Fatalf("CapabilityLimits = %#v", limits)
	}
}

func TestDecodeProviderTOMLUsesCatalogContract(t *testing.T) {
	registry := testRegistry(t)
	raw := []byte(`
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
candidate_path = ".agents/skills/acme"
evidence_path = "SKILL.md"

[[capabilities]]
id = "review"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
maximum_effects = ["read-project"]
resources = ["project"]
request_modes = ["BOUNDED"]
responsibilities = ["review"]
executor_topology = "main-agent-allowed"
delegation_allow_list = []

[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:review"
`)
	decoded, err := DecodeProvider(raw, registry)
	if err != nil {
		t.Fatalf("DecodeProvider() error = %v", err)
	}
	if decoded.Record.ID != "acme/suite" || decoded.Record.Capabilities[0].ID != "review" || decoded.Digest == "" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDecodeUserAcceptsIndependentHostPins(t *testing.T) {
	raw := []byte(`schema_version = "oaw.user-config/v2"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-codex"
evidence_digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "claude"
installation_key = "installation-claude"
evidence_digest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
`)
	decoded, err := DecodeUser(raw, testRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Record.ProviderPins) != 2 || decoded.Record.ProviderPins[0].HostID != "claude" || decoded.Record.ProviderPins[1].HostID != "codex" {
		t.Fatalf("pins = %#v", decoded.Record.ProviderPins)
	}
}

func TestDecodeUserRejectsV1Schema(t *testing.T) {
	if _, err := DecodeUser([]byte(`schema_version = "oaw.user-config/v1"`), testRegistry(t)); err == nil || !strings.Contains(err.Error(), "UNSUPPORTED_USER_CONFIG_SCHEMA") {
		t.Fatalf("DecodeUser(v1) error = %v", err)
	}
}

func TestSnapshotSeparatesHostSettingsAndCopiesInstallations(t *testing.T) {
	root := t.TempDir()
	codexLocation := filepath.Join(t.TempDir(), "codex-superpowers")
	claudeLocation := filepath.Join(t.TempDir(), "claude-superpowers")
	writeUserConfig(t, root, fmt.Sprintf(`
schema_version = "oaw.user-config/v2"

[[provider_installations]]
provider_id = "oaw/superpowers"
host_id = "codex"
surface_id = "codex-plugin"
location = %q
discovery_probe_id = "codex-direct"

[[provider_installations]]
provider_id = "oaw/superpowers"
host_id = "claude"
surface_id = "claude-plugin"
location = %q
discovery_probe_id = "claude-direct"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-codex"
evidence_digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "claude"
installation_key = "installation-claude"
evidence_digest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
`, codexLocation, claudeLocation))
	snapshot, err := Load(LoadOptions{UserConfigRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	codex := snapshot.ProviderSettings("oaw/superpowers", "codex")
	claude := snapshot.ProviderSettings("oaw/superpowers", "claude")
	if codex.Pin == nil || claude.Pin == nil || codex.Pin.InstallationKey == claude.Pin.InstallationKey {
		t.Fatalf("Host settings = %#v / %#v", codex, claude)
	}
	installations := snapshot.ProviderInstallations()
	if len(installations) != 2 || snapshot.Record().SchemaVersion != configurationSnapshotSchemaV2 {
		t.Fatalf("Snapshot installations = %#v", installations)
	}
	installations[0].Location = "changed"
	if snapshot.ProviderInstallations()[0].Location == "changed" {
		t.Fatal("ProviderInstallations exposed mutable storage")
	}
}

func TestDecodeUserRejectsInvalidHostScopedProviderRecords(t *testing.T) {
	absolute := t.TempDir()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "duplicate same-host pins",
			raw: `schema_version = "oaw.user-config/v2"
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-one"
evidence_digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-two"
evidence_digest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
`,
			want: "DUPLICATE_PROVIDER_PIN",
		},
		{
			name: "missing evidence digest",
			raw: `schema_version = "oaw.user-config/v2"
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-one"
`,
			want: "INVALID_PROVIDER_PIN",
		},
		{
			name: "unsafe installation location",
			raw: `schema_version = "oaw.user-config/v2"
[[provider_installations]]
provider_id = "oaw/superpowers"
host_id = "codex"
surface_id = "codex-plugin"
location = "../superpowers"
discovery_probe_id = "codex-direct"
`,
			want: "INVALID_PROVIDER_INSTALLATION",
		},
		{
			name: "duplicate installation identity",
			raw: fmt.Sprintf(`schema_version = "oaw.user-config/v2"
[[provider_installations]]
provider_id = "oaw/superpowers"
host_id = "codex"
surface_id = "codex-plugin"
location = %q
discovery_probe_id = "codex-direct"
[[provider_installations]]
provider_id = "oaw/superpowers"
host_id = "codex"
surface_id = "codex-plugin"
location = %q
discovery_probe_id = "codex-direct"
`, absolute, absolute),
			want: "DUPLICATE_PROVIDER_INSTALLATION",
		},
		{
			name: "legacy binding host",
			raw: `schema_version = "oaw.user-config/v2"
[[binding_preferences]]
provider_id = "oaw/superpowers"
capability_id = "review"
host = "codex"
kind = "skill"
reference = "superpowers:requesting-code-review"
`,
			want: "CONFIG_UNKNOWN_FIELD",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeUser([]byte(tt.raw), testRegistry(t)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeUser() error = %v, want %s", err, tt.want)
			}
		})
	}
}

func TestDecodeRecipeTOMLUsesCatalogContract(t *testing.T) {
	registry := testRegistry(t)
	raw := []byte(`
schema_version = "oaw.profile-recipe/v1"
recipe_version = "1.0.0"
id = "acme/review"
display_name = "Acme Review"
required_responsibilities = ["review"]
incident_routes = []
entry = "review"
terminal_gates = ["review"]
stable_boundaries = ["complete"]

[[nodes]]
id = "review"
kind = "gate"
responsibility = "review"
transitions = []

[nodes.selector]
provider_id = "acme/suite"
capability_id = "review"
`)
	decoded, err := DecodeRecipe(raw, registry)
	if err != nil {
		t.Fatalf("DecodeRecipe() error = %v", err)
	}
	if decoded.Record.ID != "acme/review" || decoded.Record.Nodes[0].Selector.ProviderID != "acme/suite" || decoded.Digest == "" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDecodeUserRejectsUnsafeContentReferencePaths(t *testing.T) {
	registry := testRegistry(t)
	for _, path := range []string{"../provider.toml", "/tmp/provider.toml", `providers\provider.toml`, "providers/./provider.toml"} {
		raw := []byte("schema_version = \"oaw.user-config/v2\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"" + strings.ReplaceAll(path, `\`, `\\`) + "\"\n")
		if _, err := DecodeUser(raw, registry); err == nil || !strings.Contains(err.Error(), "CONFIG_PATH_INVALID") {
			t.Fatalf("DecodeUser(path=%q) error = %v", path, err)
		}
	}
}

func TestDecodeUserRejectsDuplicateStableIdentities(t *testing.T) {
	registry := testRegistry(t)
	raw := []byte(`
schema_version = "oaw.user-config/v2"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/one.toml"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/two.toml"
`)
	if _, err := DecodeUser(raw, registry); err == nil || !strings.Contains(err.Error(), "DUPLICATE_PROVIDER_REFERENCE") {
		t.Fatalf("DecodeUser() error = %v", err)
	}
}

func TestInspectProjectProducesCanonicalFingerprint(t *testing.T) {
	registry := testRegistry(t)
	root := t.TempDir()
	writeProjectConfig(t, root, `
schema_version = "oaw.project-config/v1"
recommended_providers = ["zeta/suite", "oaw/superpowers"]
`)
	first, err := InspectProject(root, registry)
	if err != nil {
		t.Fatalf("InspectProject(first) error = %v", err)
	}
	writeProjectConfig(t, root, `schema_version="oaw.project-config/v1"
recommended_providers=["oaw/superpowers","zeta/suite"]
`)
	second, err := InspectProject(root, registry)
	if err != nil {
		t.Fatalf("InspectProject(second) error = %v", err)
	}
	if first.Root == "" || first.Root != second.Root || first.ConfigDigest != second.ConfigDigest {
		t.Fatalf("fingerprints differ: %#v / %#v", first, second)
	}
}

func TestInspectProjectRejectsSymlinkEscape(t *testing.T) {
	registry := testRegistry(t)
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "provider.toml")
	if err := os.WriteFile(outside, []byte(testProviderTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	providers := filepath.Join(root, ".oaw", "providers")
	if err := os.MkdirAll(providers, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(providers, "acme.toml")); err != nil {
		t.Fatal(err)
	}
	writeProjectConfig(t, root, `
schema_version = "oaw.project-config/v1"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
`)
	if _, err := InspectProject(root, registry); err == nil || !strings.Contains(err.Error(), "CONFIG_PATH_ESCAPE") {
		t.Fatalf("InspectProject() error = %v", err)
	}
}

func TestEvaluateProjectTrustRequiresExactDigests(t *testing.T) {
	registry := testRegistry(t)
	root := t.TempDir()
	writeProjectConfig(t, root, "schema_version = \"oaw.project-config/v1\"\n")
	fingerprint, err := InspectProject(root, registry)
	if err != nil {
		t.Fatal(err)
	}
	trusted := ProjectTrust{
		Root:              fingerprint.Root,
		ConfigDigest:      fingerprint.ConfigDigest,
		DescriptorDigests: append([]string(nil), fingerprint.DescriptorDigests...),
		RecipeDigests:     append([]string(nil), fingerprint.RecipeDigests...),
	}
	if status, reason := EvaluateProjectTrust([]ProjectTrust{trusted}, fingerprint); status != ProjectTrusted || reason != "PROJECT_TRUST_VERIFIED" {
		t.Fatalf("EvaluateProjectTrust() = %q, %q", status, reason)
	}
	trusted.ConfigDigest = strings.Repeat("0", 64)
	if status, reason := EvaluateProjectTrust([]ProjectTrust{trusted}, fingerprint); status != ProjectUntrusted || reason != "PROJECT_CONFIG_DIGEST_MISMATCH" {
		t.Fatalf("mismatch = %q, %q", status, reason)
	}
}

func TestEvaluateProjectTrustReportsEachMismatch(t *testing.T) {
	fingerprint := ProjectFingerprint{
		Root:              "/physical/project",
		ConfigDigest:      strings.Repeat("1", 64),
		DescriptorDigests: []string{strings.Repeat("2", 64)},
		RecipeDigests:     []string{strings.Repeat("3", 64)},
	}
	valid := ProjectTrust{
		Root:              fingerprint.Root,
		ConfigDigest:      fingerprint.ConfigDigest,
		DescriptorDigests: append([]string{}, fingerprint.DescriptorDigests...),
		RecipeDigests:     append([]string{}, fingerprint.RecipeDigests...),
	}
	tests := []struct {
		name   string
		mutate func(*ProjectTrust)
		reason string
	}{
		{"root", func(value *ProjectTrust) { value.Root = "/other/project" }, "PROJECT_ROOT_MISMATCH"},
		{"config", func(value *ProjectTrust) { value.ConfigDigest = strings.Repeat("0", 64) }, "PROJECT_CONFIG_DIGEST_MISMATCH"},
		{"descriptor", func(value *ProjectTrust) { value.DescriptorDigests = []string{} }, "PROJECT_DESCRIPTOR_DIGEST_MISMATCH"},
		{"recipe", func(value *ProjectTrust) { value.RecipeDigests = []string{} }, "PROJECT_RECIPE_DIGEST_MISMATCH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			tt.mutate(&candidate)
			if status, reason := EvaluateProjectTrust([]ProjectTrust{candidate}, fingerprint); status != ProjectUntrusted || reason != tt.reason {
				t.Fatalf("EvaluateProjectTrust() = %q, %q", status, reason)
			}
		})
	}
}

func TestLoadBuildsBuiltInOnlySnapshotWithoutFiles(t *testing.T) {
	first, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	second, err := Load(LoadOptions{})
	if err != nil {
		t.Fatalf("Load(second) error = %v", err)
	}
	if first.Digest() == "" || first.Digest() != second.Digest() {
		t.Fatalf("snapshot digests = %q / %q", first.Digest(), second.Digest())
	}
	if first.ProjectStatus() != ProjectAbsent || first.ProjectReason() != "PROJECT_CONFIG_ABSENT" {
		t.Fatalf("project = %q, %q", first.ProjectStatus(), first.ProjectReason())
	}
	value := first.Catalog()
	if len(value.Providers()) != 3 || len(value.Recipes()) != 5 || len(value.Aliases()) != 4 {
		t.Fatalf("catalog counts = %d/%d/%d", len(value.Providers()), len(value.Recipes()), len(value.Aliases()))
	}
	if first.RequiredProviders() == nil || first.RecommendedProviders() == nil || first.UntrustedProviderIDs() == nil {
		t.Fatal("snapshot exposed nil collections")
	}
}

func TestSnapshotRecordIsDigestPinnedAndDefensive(t *testing.T) {
	userRoot := t.TempDir()
	writeUserConfig(t, userRoot, `schema_version = "oaw.user-config/v2"

[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/ecc"
capability_id = "review"
`)
	snapshot, err := Load(LoadOptions{UserConfigRoot: userRoot})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	record := snapshot.Record()
	if record.ContentDigest() != snapshot.Digest() || record.CatalogDigest == "" || len(record.BoundedCapabilityDefaults) != 1 {
		t.Fatalf("snapshot record = %#v", record)
	}
	record.BoundedCapabilityDefaults[0].ID = "mutated"
	record.Settings[0].ProviderID = "mutated"
	second := snapshot.Record()
	if second.BoundedCapabilityDefaults[0].ID != "review" || second.Settings[0].ProviderID == "mutated" {
		t.Fatalf("Snapshot.Record() leaked mutable state: %#v", second)
	}
}

func TestLoadMergesUserRecordsAndAppliesDeny(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "providers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "providers", "acme.toml"), []byte(testProviderTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, root, `
schema_version = "oaw.user-config/v2"
denied_providers = ["oaw/matt", "acme/suite"]
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"

[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-superpowers"
evidence_digest = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
location = "/opt/superpowers"
version = "1.2.3"

[[binding_preferences]]
provider_id = "oaw/superpowers"
capability_id = "review"
host_id = "codex"
kind = "skill"
reference = "superpowers:requesting-code-review"
`)
	snapshot, err := Load(LoadOptions{UserConfigRoot: root})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := len(snapshot.Catalog().Providers()); got != 4 {
		t.Fatalf("provider count = %d, want 4", got)
	}
	if !snapshot.ProviderSettings("oaw/matt", "codex").Disabled || !snapshot.ProviderSettings("acme/suite", "codex").Disabled {
		t.Fatal("user deny did not disable providers")
	}
	settings := snapshot.ProviderSettings("oaw/superpowers", "codex")
	settings.Pin.Location = "/changed"
	settings.Preferences[0].Reference = "changed"
	fresh := snapshot.ProviderSettings("oaw/superpowers", "codex")
	if fresh.Pin.Location != "/opt/superpowers" || fresh.Preferences[0].Reference != "superpowers:requesting-code-review" {
		t.Fatalf("ProviderSettings exposed mutable storage: %#v", fresh)
	}
}

func TestLoadMergesUserAndTrustedProjectWholeRecords(t *testing.T) {
	projectRoot := t.TempDir()
	projectProviders := filepath.Join(projectRoot, ".oaw", "providers")
	if err := os.MkdirAll(projectProviders, 0o700); err != nil {
		t.Fatal(err)
	}
	projectProvider := strings.Replace(testReviewProviderTOML, `display_name = "Acme Suite"`, `display_name = "Acme Suite Project"`, 1)
	if err := os.WriteFile(filepath.Join(projectProviders, "acme.toml"), []byte(projectProvider), 0o600); err != nil {
		t.Fatal(err)
	}
	writeProjectConfig(t, projectRoot, `
schema_version = "oaw.project-config/v1"
required_providers = ["acme/suite"]
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
replace = true
`)
	fingerprint, err := InspectProject(projectRoot, testRegistry(t))
	if err != nil {
		t.Fatal(err)
	}

	userRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userRoot, "providers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(userRoot, "profiles"), 0o700); err != nil {
		t.Fatal(err)
	}
	userProvider := strings.Replace(testReviewProviderTOML, `display_name = "Acme Suite"`, `display_name = "Acme Suite User"`, 1)
	if err := os.WriteFile(filepath.Join(userRoot, "providers", "acme.toml"), []byte(userProvider), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userRoot, "profiles", "review.toml"), []byte(testReviewRecipeTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, userRoot, fmt.Sprintf(`
schema_version = "oaw.user-config/v2"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
[[profile_recipes]]
id = "acme/review"
path = "profiles/review.toml"
%s
`, projectTrustTableTOML(fingerprint)))

	first, err := Load(LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	second, err := Load(LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Load(second) error = %v", err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("snapshot digests = %q / %q", first.Digest(), second.Digest())
	}

	value := first.Catalog()
	providerName := ""
	for _, provider := range value.Providers() {
		if provider.ID == "acme/suite" {
			providerName = provider.DisplayName
		}
	}
	foundRecipe := false
	for _, recipe := range value.Recipes() {
		foundRecipe = foundRecipe || recipe.ID == "acme/review"
	}
	if providerName != "Acme Suite Project" || !foundRecipe || len(value.Aliases()) != 4 {
		t.Fatalf("merged catalog provider=%q recipe=%v aliases=%d", providerName, foundRecipe, len(value.Aliases()))
	}
}

func TestLoadRejectsImplicitProjectReplacement(t *testing.T) {
	projectRoot := projectWithProvider(t, false)
	fingerprint, err := InspectProject(projectRoot, testRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	userRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userRoot, "providers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userRoot, "providers", "acme.toml"), []byte(testProviderTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, userRoot, fmt.Sprintf(`
schema_version = "oaw.user-config/v2"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
%s
`, projectTrustTableTOML(fingerprint)))
	if _, err := Load(LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot}); err == nil || !strings.Contains(err.Error(), "DUPLICATE_PROVIDER_REPLACEMENT_REQUIRED") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsReservedUserProviderNamespace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "providers"), 0o700); err != nil {
		t.Fatal(err)
	}
	provider := strings.Replace(testProviderTOML, `id = "acme/suite"`, `id = "oaw/replacement"`, 1)
	if err := os.WriteFile(filepath.Join(root, "providers", "replacement.toml"), []byte(provider), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, root, `
schema_version = "oaw.user-config/v2"
[[provider_descriptors]]
id = "oaw/replacement"
path = "providers/replacement.toml"
`)
	if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "RESERVED_PROVIDER_NAMESPACE") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsReservedUserRecipeNamespace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "profiles"), 0o700); err != nil {
		t.Fatal(err)
	}
	recipe := strings.Replace(testReviewRecipeTOML, `id = "acme/review"`, `id = "oaw/replacement"`, 1)
	recipe = strings.Replace(recipe, `provider_id = "acme/suite"`, `provider_id = "oaw/superpowers"`, 1)
	if err := os.WriteFile(filepath.Join(root, "profiles", "replacement.toml"), []byte(recipe), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, root, `
schema_version = "oaw.user-config/v2"
[[profile_recipes]]
id = "oaw/replacement"
path = "profiles/replacement.toml"
`)
	if _, err := Load(LoadOptions{UserConfigRoot: root}); err == nil || !strings.Contains(err.Error(), "RESERVED_RECIPE_NAMESPACE") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadExcludesUntrustedProjectRecords(t *testing.T) {
	projectRoot := projectWithProvider(t, false)
	snapshot, err := Load(LoadOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if snapshot.ProjectStatus() != ProjectUntrusted || snapshot.ProjectReason() != "PROJECT_TRUST_MISSING" {
		t.Fatalf("project = %q, %q", snapshot.ProjectStatus(), snapshot.ProjectReason())
	}
	if got := len(snapshot.Catalog().Providers()); got != 3 {
		t.Fatalf("provider count = %d, want 3", got)
	}
	if got := snapshot.UntrustedProviderIDs(); len(got) != 1 || got[0] != "acme/suite" {
		t.Fatalf("UntrustedProviderIDs = %#v", got)
	}
}

func TestLoadDoesNotShadowTrustedProviderWithUntrustedReplacement(t *testing.T) {
	userRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(userRoot, "providers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userRoot, "providers", "acme.toml"), []byte(testProviderTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, userRoot, `
schema_version = "oaw.user-config/v2"
[[provider_descriptors]]
id = "acme/suite"
path = "providers/acme.toml"
`)
	projectRoot := projectWithProvider(t, false)
	snapshot, err := Load(LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := snapshot.UntrustedProviderIDs(); len(got) != 0 {
		t.Fatalf("UntrustedProviderIDs() = %#v", got)
	}
	if got := len(snapshot.Catalog().Providers()); got != 4 {
		t.Fatalf("provider count = %d, want 4", got)
	}
}

func TestLoadMergesExactlyTrustedProjectAndUserDenyWins(t *testing.T) {
	projectRoot := projectWithProvider(t, true)
	registry := testRegistry(t)
	fingerprint, err := InspectProject(projectRoot, registry)
	if err != nil {
		t.Fatal(err)
	}
	userRoot := t.TempDir()
	writeUserConfig(t, userRoot, projectTrustTOML(fingerprint, []string{"acme/suite"}))
	snapshot, err := Load(LoadOptions{UserConfigRoot: userRoot, ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if snapshot.ProjectStatus() != ProjectTrusted || snapshot.ProjectReason() != "PROJECT_TRUST_VERIFIED" {
		t.Fatalf("project = %q, %q", snapshot.ProjectStatus(), snapshot.ProjectReason())
	}
	if got := len(snapshot.Catalog().Providers()); got != 4 {
		t.Fatalf("provider count = %d, want 4", got)
	}
	if !snapshot.ProviderSettings("acme/suite", "codex").Disabled {
		t.Fatal("user deny did not override trusted project")
	}
	if got := snapshot.RequiredProviders(); len(got) != 1 || got[0] != "acme/suite" {
		t.Fatalf("RequiredProviders = %#v", got)
	}
	mutated := snapshot.RequiredProviders()
	mutated[0] = "changed/provider"
	if snapshot.RequiredProviders()[0] != "acme/suite" {
		t.Fatal("RequiredProviders exposed mutable storage")
	}
}

func TestSnapshotIsImmutableAcrossSourceChanges(t *testing.T) {
	root := t.TempDir()
	providers := filepath.Join(root, "providers")
	if err := os.MkdirAll(providers, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(providers, "acme.toml")
	if err := os.WriteFile(path, []byte(testProviderTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, root, "schema_version = \"oaw.user-config/v2\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.toml\"\n")
	first, err := Load(LoadOptions{UserConfigRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(testProviderTOML, `display_name = "Acme Suite"`, `display_name = "Acme Suite v2"`, 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Load(LoadOptions{UserConfigRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == second.Digest() || first.Catalog().Digest() == second.Catalog().Digest() {
		t.Fatal("source change did not produce a new snapshot")
	}
	for _, provider := range first.Catalog().Providers() {
		if provider.ID == "acme/suite" && provider.DisplayName != "Acme Suite" {
			t.Fatalf("existing snapshot changed: %#v", provider)
		}
	}
}

func writeProjectConfig(t *testing.T, root, content string) {
	t.Helper()
	directory := filepath.Join(root, ".oaw")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeUserConfig(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func projectWithProvider(t *testing.T, required bool) string {
	t.Helper()
	root := t.TempDir()
	providers := filepath.Join(root, ".oaw", "providers")
	if err := os.MkdirAll(providers, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(providers, "acme.toml"), []byte(testProviderTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	requirement := ""
	if required {
		requirement = "required_providers = [\"acme/suite\"]\n"
	}
	writeProjectConfig(t, root, "schema_version = \"oaw.project-config/v1\"\n"+requirement+"[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.toml\"\n")
	return root
}

func projectTrustTOML(fingerprint ProjectFingerprint, denied []string) string {
	quotedDenied := make([]string, len(denied))
	for i, value := range denied {
		quotedDenied[i] = fmt.Sprintf("%q", value)
	}
	return fmt.Sprintf("schema_version = \"oaw.user-config/v2\"\ndenied_providers = [%s]\n%s", strings.Join(quotedDenied, ","), projectTrustTableTOML(fingerprint))
}

func projectTrustTableTOML(fingerprint ProjectFingerprint) string {
	quotedDescriptors := make([]string, len(fingerprint.DescriptorDigests))
	for i, value := range fingerprint.DescriptorDigests {
		quotedDescriptors[i] = fmt.Sprintf("%q", value)
	}
	quotedRecipes := make([]string, len(fingerprint.RecipeDigests))
	for i, value := range fingerprint.RecipeDigests {
		quotedRecipes[i] = fmt.Sprintf("%q", value)
	}
	return fmt.Sprintf("[[project_trust]]\nroot = %q\nconfig_digest = %q\ndescriptor_digests = [%s]\nrecipe_digests = [%s]\n", fingerprint.Root, fingerprint.ConfigDigest, strings.Join(quotedDescriptors, ","), strings.Join(quotedRecipes, ","))
}

const testProviderTOML = `
schema_version = "oaw.provider-descriptor/v2"
descriptor_version = "2.0.0"
id = "acme/suite"
display_name = "Acme Suite"
discovery = []
capabilities = []
`

const testReviewProviderTOML = `
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
candidate_path = ".agents/skills/acme"
evidence_path = "SKILL.md"

[[capabilities]]
id = "review"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
maximum_effects = ["read-project"]
resources = ["project"]
request_modes = ["BOUNDED"]
responsibilities = ["review"]
executor_topology = "main-agent-allowed"
delegation_allow_list = []

[[capabilities.host_bindings]]
host = "codex"
kind = "skill"
reference = "acme:review"
`

const testReviewRecipeTOML = `
schema_version = "oaw.profile-recipe/v1"
recipe_version = "1.0.0"
id = "acme/review"
display_name = "Acme Review"
required_responsibilities = ["review"]
incident_routes = []
entry = "review"
terminal_gates = ["review"]
stable_boundaries = ["complete"]

[[nodes]]
id = "review"
kind = "gate"
responsibility = "review"
transitions = []

[nodes.selector]
provider_id = "acme/suite"
capability_id = "review"
`

func testRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	registry, err := schema.New(assets.FS())
	if err != nil {
		t.Fatalf("schema.New() error = %v", err)
	}
	return registry
}
