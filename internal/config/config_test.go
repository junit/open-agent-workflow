package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestDecodeUserRejectsUnknownFields(t *testing.T) {
	registry := testRegistry(t)
	_, err := DecodeUser([]byte("schema_version = \"oaw.user-config/v1\"\nunknown = true\n"), registry)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_UNKNOWN_FIELD") {
		t.Fatalf("DecodeUser() error = %v", err)
	}
}

func TestDecodeUserNormalizesEquivalentTOML(t *testing.T) {
	registry := testRegistry(t)
	first := []byte(`
schema_version = "oaw.user-config/v1"
denied_providers = ["zeta/suite", "acme/suite"]

[[provider_pins]]
id = "zeta/suite"
version = "2.0.0"

[[provider_pins]]
id = "acme/suite"
location = "/opt/acme"
`)
	second := []byte(`schema_version="oaw.user-config/v1"
denied_providers=["acme/suite","zeta/suite"]
[[provider_pins]]
id="acme/suite"
location="/opt/acme"
[[provider_pins]]
id="zeta/suite"
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
	for _, field := range []string{"enabled_providers = [\"acme/suite\"]", "authority = \"write\"", "binding = \"agent\""} {
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
schema_version = "oaw.provider-descriptor/v1"
descriptor_version = "1.0.0"
id = "acme/suite"
display_name = "Acme Suite"

[[discovery]]
id = "acme-skill"
kind = "path-exists"
root = "user-home"
path = ".agents/skills/acme/SKILL.md"

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
		raw := []byte("schema_version = \"oaw.user-config/v1\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"" + strings.ReplaceAll(path, `\`, `\\`) + "\"\n")
		if _, err := DecodeUser(raw, registry); err == nil || !strings.Contains(err.Error(), "CONFIG_PATH_INVALID") {
			t.Fatalf("DecodeUser(path=%q) error = %v", path, err)
		}
	}
}

func TestDecodeUserRejectsDuplicateStableIdentities(t *testing.T) {
	registry := testRegistry(t)
	raw := []byte(`
schema_version = "oaw.user-config/v1"
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

const testProviderTOML = `
schema_version = "oaw.provider-descriptor/v1"
descriptor_version = "1.0.0"
id = "acme/suite"
display_name = "Acme Suite"
discovery = []
capabilities = []
`

func testRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	registry, err := schema.New(assets.FS())
	if err != nil {
		t.Fatalf("schema.New() error = %v", err)
	}
	return registry
}
