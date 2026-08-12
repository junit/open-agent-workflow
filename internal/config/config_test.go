package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestDecodeUserRejectsUnknownFields(t *testing.T) {
	registry := testRegistry(t)
	_, err := DecodeUser([]byte("schema_version = \"oaw.user-config/v3\"\nunknown = true\n"), registry)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_UNKNOWN_FIELD") {
		t.Fatalf("DecodeUser() error = %v", err)
	}
}

func TestDecodeUserRequiresV3WithoutMigrationOutput(t *testing.T) {
	registry := testRegistry(t)
	decoded, err := DecodeUser([]byte("schema_version = \"oaw.user-config/v3\"\n"), registry)
	if err != nil {
		t.Fatalf("DecodeUser(v3) error = %v", err)
	}
	if decoded.Record.SchemaVersion != "oaw.user-config/v3" || decoded.Digest == "" || len(decoded.CanonicalJSON) == 0 {
		t.Fatalf("DecodeUser(v3) = %#v", decoded)
	}

	retired, err := DecodeUser([]byte("schema_version = \"oaw.user-config/v2\"\n"), registry)
	if err == nil || !strings.Contains(err.Error(), "CONFIG_SCHEMA_UNSUPPORTED") {
		t.Fatalf("DecodeUser(v2) error = %v", err)
	}
	if retired.Record.SchemaVersion != "" || retired.Digest != "" || len(retired.CanonicalJSON) != 0 {
		t.Fatalf("DecodeUser(v2) produced migration output: %#v", retired)
	}
}

func TestDecodeUserNormalizesEquivalentTOML(t *testing.T) {
	registry := testRegistry(t)
	first := []byte(`
schema_version = "oaw.user-config/v3"
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
	second := []byte(`schema_version="oaw.user-config/v3"
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

func TestDecodeProviderV4TOMLUsesCatalogContract(t *testing.T) {
	decoded, err := DecodeProvider(embeddedCatalogTOML(t, "providers/oaw-matt.json"), testRegistry(t))
	if err != nil {
		t.Fatalf("DecodeProvider(v4 TOML) error = %v", err)
	}
	if decoded.Record.SchemaVersion != catalog.ProviderDescriptorSchemaV4 || decoded.Record.ID != "test/provider" || decoded.Digest == "" {
		t.Fatalf("decoded Provider = %#v", decoded)
	}
}

func TestDecodeRecipeV3TOMLUsesCatalogContract(t *testing.T) {
	decoded, err := DecodeRecipe(embeddedCatalogTOML(t, "recipes/oaw-delivery.json"), testRegistry(t))
	if err != nil {
		t.Fatalf("DecodeRecipe(v3 TOML) error = %v", err)
	}
	if decoded.Record.SchemaVersion != catalog.ProfileRecipeSchemaV3 || decoded.Record.ID != "oaw/delivery" || len(decoded.Record.Slots) != len(catalog.CanonicalSlots()) || decoded.Digest == "" {
		t.Fatalf("decoded Recipe = %#v", decoded)
	}
}

func TestReferencedAuthorityHardCutRejectsV3AndV2(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  []byte
		code string
		call func([]byte) error
	}{
		{name: "Provider v3", raw: []byte(`schema_version = "oaw.provider-descriptor/v3"`), code: "UNSUPPORTED_PROVIDER_SCHEMA", call: func(raw []byte) error {
			_, err := DecodeProvider(raw, testRegistry(t))
			return err
		}},
		{name: "Provider v2", raw: []byte(`schema_version = "oaw.provider-descriptor/v2"`), code: "UNSUPPORTED_PROVIDER_SCHEMA", call: func(raw []byte) error {
			_, err := DecodeProvider(raw, testRegistry(t))
			return err
		}},
		{name: "Recipe v2", raw: []byte(`schema_version = "oaw.profile-recipe/v2"`), code: "UNSUPPORTED_RECIPE_SCHEMA", call: func(raw []byte) error {
			_, err := DecodeRecipe(raw, testRegistry(t))
			return err
		}},
		{name: "Recipe v1", raw: []byte(`schema_version = "oaw.profile-recipe/v1"`), code: "UNSUPPORTED_RECIPE_SCHEMA", call: func(raw []byte) error {
			_, err := DecodeRecipe(raw, testRegistry(t))
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(test.raw); err == nil || !strings.Contains(err.Error(), test.code) {
				t.Fatalf("decode error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestV4BindingPreferenceResolvesByBindingID(t *testing.T) {
	provider := configProviderV4Record()
	if len(provider.Capabilities) == 0 || len(provider.Capabilities[0].BindingRefs) == 0 {
		t.Fatalf("Provider has no Binding-backed Capability: %#v", provider)
	}
	bindingID := provider.Capabilities[0].BindingRefs[0]
	var binding catalog.BindingRecord
	for _, candidate := range provider.Bindings {
		if candidate.ID == bindingID {
			binding = candidate
			break
		}
	}
	if binding.ID == "" {
		t.Fatalf("Binding %q not found", bindingID)
	}
	effective, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := buildProviderSettings(effective, UserConfigRecord{BindingPreferences: []BindingPreference{{
		ProviderID: provider.ID, CapabilityID: provider.Capabilities[0].ID, HostID: binding.Host, Kind: string(binding.Kind), Reference: binding.Reference,
	}}}, emptyProjectConfig())
	if err != nil {
		t.Fatalf("buildProviderSettings() error = %v", err)
	}
	value := settingsFor(settings, provider.ID, binding.Host)
	if len(value.Preferences) != 1 || value.Preferences[0].Reference != binding.Reference {
		t.Fatalf("resolved settings = %#v", value)
	}

	missing := UserConfigRecord{BindingPreferences: []BindingPreference{{
		ProviderID: provider.ID, CapabilityID: provider.Capabilities[0].ID, HostID: binding.Host, Kind: string(binding.Kind), Reference: "missing",
	}}}
	if _, err := buildProviderSettings(effective, missing, emptyProjectConfig()); err == nil || !strings.Contains(err.Error(), "BINDING_PREFERENCE_UNDECLARED") {
		t.Fatalf("zero-match preference error = %v", err)
	}

	crossDistributionProvider := configProviderV4Record()
	secondDistribution := crossDistributionProvider.Distributions[0]
	secondDistribution.ID = "distribution-second"
	secondDistribution.SourceURI = "https://example.test/provider-second"
	secondDistribution.Revision = strings.Repeat("b", 40)
	secondDistribution.TreeDigest = "sha256:" + strings.Repeat("b", 64)
	crossDistributionProvider.Distributions = append(crossDistributionProvider.Distributions, secondDistribution)
	secondProbe := crossDistributionProvider.Discovery[0]
	secondProbe.ID = "probe-second"
	secondProbe.DistributionID = secondDistribution.ID
	secondProbe.CandidatePath = ".agents/skills-second"
	crossDistributionProvider.Discovery = append(crossDistributionProvider.Discovery, secondProbe)
	secondBinding := crossDistributionProvider.Bindings[0]
	secondBinding.ID = "binding-second"
	secondBinding.DistributionID = secondDistribution.ID
	secondBinding.TreeDigest = "sha256:" + strings.Repeat("b", 64)
	crossDistributionProvider.Bindings = append(crossDistributionProvider.Bindings, secondBinding)
	crossDistributionProvider.Capabilities[0].BindingRefs = append(crossDistributionProvider.Capabilities[0].BindingRefs, secondBinding.ID)
	crossDistributionCatalog, err := catalog.New([]catalog.ProviderDescriptorRecord{crossDistributionProvider}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildProviderSettings(crossDistributionCatalog, UserConfigRecord{BindingPreferences: []BindingPreference{{
		ProviderID: crossDistributionProvider.ID, CapabilityID: crossDistributionProvider.Capabilities[0].ID,
		HostID: secondBinding.Host, Kind: string(secondBinding.Kind), Reference: secondBinding.Reference,
	}}}, emptyProjectConfig()); err != nil {
		t.Fatalf("cross-Distribution preference error = %v", err)
	}

	ambiguousProvider := configProviderV4Record()
	duplicate := ambiguousProvider.Bindings[0]
	duplicate.ID = "binding-second"
	ambiguousProvider.Bindings = append(ambiguousProvider.Bindings, duplicate)
	ambiguousProvider.Capabilities[0].BindingRefs = append(ambiguousProvider.Capabilities[0].BindingRefs, duplicate.ID)
	ambiguousCatalog, err := catalog.New([]catalog.ProviderDescriptorRecord{ambiguousProvider}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildProviderSettings(ambiguousCatalog, UserConfigRecord{BindingPreferences: []BindingPreference{{
		ProviderID: ambiguousProvider.ID, CapabilityID: ambiguousProvider.Capabilities[0].ID, HostID: duplicate.Host, Kind: string(duplicate.Kind), Reference: duplicate.Reference,
	}}}, emptyProjectConfig()); err == nil || !strings.Contains(err.Error(), "BINDING_PREFERENCE_UNDECLARED") {
		t.Fatalf("multi-match preference error = %v", err)
	}
}

func TestUserConfigV3AcceptsAllV4BindingKinds(t *testing.T) {
	kinds := []string{"skill", "agent", "role", "instruction", "tool"}
	var raw strings.Builder
	raw.WriteString("schema_version = \"oaw.user-config/v3\"\n")
	for index, kind := range kinds {
		fmt.Fprintf(&raw, "\n[[binding_preferences]]\nprovider_id = \"acme/suite\"\ncapability_id = \"cap-%d\"\nhost_id = \"codex\"\nkind = \"%s\"\nreference = \"acme:%s\"\n", index, kind, kind)
	}
	decoded, err := DecodeUser([]byte(raw.String()), testRegistry(t))
	if err != nil {
		t.Fatalf("DecodeUser(all v4 Binding kinds) error = %v", err)
	}
	if len(decoded.Record.BindingPreferences) != len(kinds) {
		t.Fatalf("BindingPreferences = %#v", decoded.Record.BindingPreferences)
	}
	hook := strings.Replace(raw.String(), `kind = "tool"`, `kind = "hook"`, 1)
	if _, err := DecodeUser([]byte(hook), testRegistry(t)); err == nil || !strings.Contains(err.Error(), "INVALID_BINDING_PREFERENCE") {
		t.Fatalf("DecodeUser(hook) error = %v", err)
	}
}

func TestDecodeProviderTOMLUsesCatalogContract(t *testing.T) {
	decoded, err := DecodeProvider(embeddedCatalogTOML(t, "providers/oaw-matt.json"), testRegistry(t))
	if err != nil {
		t.Fatalf("DecodeProvider() error = %v", err)
	}
	if decoded.Record.ID != "test/provider" || decoded.Record.Capabilities[0].BindingRefs[0] != "binding" || decoded.Digest == "" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDecodeDescriptorJSONRejectsAmbiguousInput(t *testing.T) {
	registry := testRegistry(t)
	for _, test := range []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "duplicate field",
			raw:  `{"schema_version":"oaw.provider-descriptor/v3","schema_version":"oaw.provider-descriptor/v3"}`,
			want: "CONFIG_JSON_INVALID",
		},
		{
			name: "unknown field",
			raw:  `{"schema_version":"oaw.provider-descriptor/v3","unknown":true}`,
			want: "CONFIG_UNKNOWN_FIELD",
		},
		{
			name: "trailing value",
			raw:  `{} {}`,
			want: "CONFIG_JSON_INVALID",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeProvider([]byte(test.raw), registry); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("DecodeProvider() error = %v, want %s", err, test.want)
			}
		})
	}
}

func TestDecodeUserAcceptsIndependentHostPins(t *testing.T) {
	raw := []byte(`schema_version = "oaw.user-config/v3"

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
	if _, err := DecodeUser([]byte(`schema_version = "oaw.user-config/v1"`), testRegistry(t)); err == nil || !strings.Contains(err.Error(), "CONFIG_SCHEMA_UNSUPPORTED") {
		t.Fatalf("DecodeUser(v1) error = %v", err)
	}
}

func TestSnapshotSeparatesHostSettingsAndCopiesInstallations(t *testing.T) {
	root := t.TempDir()
	codexLocation := filepath.Join(t.TempDir(), "codex-superpowers")
	claudeLocation := filepath.Join(t.TempDir(), "claude-superpowers")
	writeUserConfig(t, root, fmt.Sprintf(`
schema_version = "oaw.user-config/v3"

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
			raw: `schema_version = "oaw.user-config/v3"
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
			raw: `schema_version = "oaw.user-config/v3"
[[provider_pins]]
provider_id = "oaw/superpowers"
host_id = "codex"
installation_key = "installation-one"
`,
			want: "INVALID_PROVIDER_PIN",
		},
		{
			name: "unsafe installation location",
			raw: `schema_version = "oaw.user-config/v3"
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
			raw: fmt.Sprintf(`schema_version = "oaw.user-config/v3"
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
			raw: `schema_version = "oaw.user-config/v3"
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
	decoded, err := DecodeRecipe(embeddedCatalogTOML(t, "recipes/oaw-delivery.json"), testRegistry(t))
	if err != nil {
		t.Fatalf("DecodeRecipe() error = %v", err)
	}
	if decoded.Record.ID != "oaw/delivery" || decoded.Record.Slots[0].Pipeline[0].Selector.ProviderID != "test/provider" || decoded.Digest == "" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDecodeUserRejectsUnsafeContentReferencePaths(t *testing.T) {
	registry := testRegistry(t)
	for _, path := range []string{"../provider.toml", "/tmp/provider.toml", `providers\provider.toml`, "providers/./provider.toml"} {
		raw := []byte("schema_version = \"oaw.user-config/v3\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"" + strings.ReplaceAll(path, `\`, `\\`) + "\"\n")
		if _, err := DecodeUser(raw, registry); err == nil || !strings.Contains(err.Error(), "CONFIG_PATH_INVALID") {
			t.Fatalf("DecodeUser(path=%q) error = %v", path, err)
		}
	}
}

func TestDecodeUserRejectsDuplicateStableIdentities(t *testing.T) {
	registry := testRegistry(t)
	raw := []byte(`
schema_version = "oaw.user-config/v3"
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
	if len(value.Providers()) != 3 || len(value.Recipes()) != 4 || len(value.Aliases()) != 4 {
		t.Fatalf("catalog counts = %d/%d/%d", len(value.Providers()), len(value.Recipes()), len(value.Aliases()))
	}
	if first.RequiredProviders() == nil || first.RecommendedProviders() == nil || first.UntrustedProviderIDs() == nil {
		t.Fatal("snapshot exposed nil collections")
	}
}

func TestSnapshotRecordIsDigestPinnedAndDefensive(t *testing.T) {
	userRoot := t.TempDir()
	writeUserConfig(t, userRoot, `schema_version = "oaw.user-config/v3"

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
schema_version = "oaw.user-config/v3"
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
	if err := os.WriteFile(filepath.Join(userRoot, "profiles", "review.toml"), []byte(testReviewRecipeTOML(t, "acme/review", "acme/suite")), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, userRoot, fmt.Sprintf(`
schema_version = "oaw.user-config/v3"
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
schema_version = "oaw.user-config/v3"
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
schema_version = "oaw.user-config/v3"
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
	recipe := testReviewRecipeTOML(t, "oaw/replacement", "oaw/superpowers")
	if err := os.WriteFile(filepath.Join(root, "profiles", "replacement.toml"), []byte(recipe), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, root, `
schema_version = "oaw.user-config/v3"
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
schema_version = "oaw.user-config/v3"
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
	writeUserConfig(t, root, "schema_version = \"oaw.user-config/v3\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.toml\"\n")
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
	return fmt.Sprintf("schema_version = \"oaw.user-config/v3\"\ndenied_providers = [%s]\n%s", strings.Join(quotedDenied, ","), projectTrustTableTOML(fingerprint))
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
schema_version = "oaw.provider-descriptor/v4"
descriptor_version = "4.0.0"
id = "acme/suite"
display_name = "Acme Suite"
[[distributions]]
id = "distribution"
source_uri = "https://example.test/acme"
revision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
tree_digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
[[discovery]]
id = "probe"
hosts = ["codex"]
surface = "codex-skills"
distribution_id = "distribution"
kind = "path-exists"
root = "user-home"
candidate_path = ".agents/skills/acme"
evidence_path = "SKILL.md"
[[bindings]]
id = "binding"
distribution_id = "distribution"
content_root = "skills/acme"
install_root = "acme"
tree_digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
host = "codex"
surface = "codex-skills"
kind = "skill"
reference = "acme:review"
invocation = "model"
input_artifact = "artifact"
output_artifact = "artifact"
maximum_effects = ["read-project"]
resources = ["project"]
supported_topologies = ["CURRENT"]
responsibilities = [{namespace = "stage", name = "problem-framing", slot_id = "problem-framing", outcome_owner = true}]
delegation = {child = false, parallel_child = false, nested_child = false, nested_parallel_child = false}
stage_span = ["problem-framing"]
internal_calls = []
alternatives = []
conflicts = []
[[capabilities]]
id = "review"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
request_modes = ["BOUNDED"]
binding_refs = ["binding"]
`

const testReviewProviderTOML = `
schema_version = "oaw.provider-descriptor/v4"
descriptor_version = "4.0.0"
id = "acme/suite"
display_name = "Acme Suite"

[[distributions]]
id = "distribution"
source_uri = "https://example.test/acme"
revision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
tree_digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

[[discovery]]
id = "acme-skill"
hosts = ["codex"]
surface = "codex-user-skills"
distribution_id = "distribution"
kind = "path-exists"
root = "user-home"
candidate_path = ".agents/skills/acme"
evidence_path = "SKILL.md"

[[bindings]]
id = "binding"
distribution_id = "distribution"
content_root = "skills/acme"
install_root = "acme"
tree_digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
host = "codex"
surface = "codex-user-skills"
kind = "skill"
reference = "acme:review"
invocation = "model"
input_artifact = "artifact"
output_artifact = "artifact"
maximum_effects = ["read-project"]
resources = ["project"]
supported_topologies = ["CURRENT"]
responsibilities = [
  {namespace = "stage", name = "problem-framing", slot_id = "problem-framing", outcome_owner = true},
  {namespace = "stage", name = "solution-specification", slot_id = "solution-specification", outcome_owner = true},
  {namespace = "stage", name = "delivery-planning", slot_id = "delivery-planning", outcome_owner = true},
  {namespace = "stage", name = "workspace-preparation", slot_id = "workspace-preparation", outcome_owner = true},
  {namespace = "stage", name = "implementation", slot_id = "implementation", outcome_owner = true},
  {namespace = "procedure", name = "implementation-tdd", slot_id = "implementation-tdd", outcome_owner = true},
  {namespace = "incident", name = "incident-recovery", slot_id = "incident-recovery", outcome_owner = true},
  {namespace = "assurance", name = "review-remediation", slot_id = "review-remediation", outcome_owner = true},
  {namespace = "assurance", name = "fresh-verification", slot_id = "fresh-verification", outcome_owner = true},
  {namespace = "stage", name = "closeout", slot_id = "closeout", outcome_owner = true}
]
delegation = {child = false, parallel_child = false, nested_child = false, nested_parallel_child = false}
stage_span = ["problem-framing", "solution-specification", "delivery-planning", "workspace-preparation", "implementation", "implementation-tdd", "incident-recovery", "review-remediation", "fresh-verification", "closeout"]
internal_calls = []
alternatives = []
conflicts = []

[[capabilities]]
id = "review"
input_schema = "oaw.capability-input/v1"
outcome_schema = "oaw.capability-outcome/v1"
request_modes = ["BOUNDED"]
binding_refs = ["binding"]
`

func testReviewRecipeTOML(t *testing.T, id, providerID string) string {
	t.Helper()
	record := configRecipeV3Record()
	record.ID = id
	record.DisplayName = "Acme Review"
	record.Family = "review"
	for slotIndex := range record.Slots {
		for stepIndex := range record.Slots[slotIndex].Pipeline {
			record.Slots[slotIndex].Pipeline[stepIndex].Selector = catalog.BindingSelector{
				ProviderID: providerID,
				BindingID:  "binding",
			}
		}
	}
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(record); err != nil {
		t.Fatal(err)
	}
	return encoded.String()
}

func testRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	registry, err := schema.New(assets.FS())
	if err != nil {
		t.Fatalf("schema.New() error = %v", err)
	}
	return registry
}

func embeddedCatalogTOML(t *testing.T, name string) []byte {
	t.Helper()
	var value any
	switch name {
	case "providers/oaw-matt.json":
		value = configProviderV4Record()
	case "recipes/oaw-delivery.json":
		value = configRecipeV3Record()
	default:
		t.Fatalf("unknown catalog test fixture %q", name)
	}
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).Encode(value); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func configProviderV4Record() catalog.ProviderDescriptorRecord {
	claims := []catalog.ResponsibilityClaim{
		{Namespace: catalog.OwnershipStage, Name: "problem-framing", SlotID: catalog.SlotProblemFraming, OutcomeOwner: true},
		{Namespace: catalog.OwnershipStage, Name: "solution-specification", SlotID: catalog.SlotSolutionSpecification, OutcomeOwner: true},
		{Namespace: catalog.OwnershipStage, Name: "delivery-planning", SlotID: catalog.SlotDeliveryPlanning, OutcomeOwner: true},
		{Namespace: catalog.OwnershipStage, Name: "implementation", SlotID: catalog.SlotImplementation, OutcomeOwner: true},
		{Namespace: catalog.OwnershipProcedure, Name: "implementation-tdd", SlotID: catalog.SlotImplementationTDD, OutcomeOwner: true},
		{Namespace: catalog.OwnershipAssurance, Name: "review-remediation", SlotID: catalog.SlotReviewRemediation, OutcomeOwner: true},
		{Namespace: catalog.OwnershipProcedure, Name: "fresh-verification", SlotID: catalog.SlotFreshVerification, OutcomeOwner: true},
	}
	return catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "test/provider", DisplayName: "Test Provider",
		Distributions: []catalog.DistributionRecord{{ID: "distribution", SourceURI: "https://example.test/provider", Revision: strings.Repeat("a", 40), TreeDigest: "sha256:" + strings.Repeat("a", 64)}},
		Discovery:     []catalog.DiscoveryProbe{{ID: "probe", Hosts: []string{"codex"}, Surface: "codex-skills", DistributionID: "distribution", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/skills", EvidencePath: "skill/SKILL.md"}},
		Bindings: []catalog.BindingRecord{{
			ID: "binding", DistributionID: "distribution", ContentRoot: "skills/skill", InstallRoot: "skill", TreeDigest: "sha256:" + strings.Repeat("a", 64),
			Host: "codex", Surface: "codex-skills", Kind: catalog.BindingSkill, Reference: "skill", Invocation: catalog.InvocationModel,
			Responsibilities: claims, InputArtifact: "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: catalog.DelegationRequirements{},
			StageSpan: configCanonicalSlotIDs(), InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{ID: "workflow", InputSchema: "artifact", OutcomeSchema: "artifact", RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"binding"}}},
	}
}

func configRecipeV3Record() catalog.ProfileRecipeRecord {
	definitions := catalog.CanonicalSlots()
	record := catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV3, TaxonomyVersion: catalog.TaxonomyVersionV1, RecipeVersion: "3.0.0", ID: "oaw/delivery", DisplayName: "Delivery", Family: "delivery",
		Slots: make([]catalog.SlotRecipe, len(definitions)), AddOns: []catalog.AddOnRecord{}, IncidentRoutes: []catalog.IncidentRoute{}, Overlays: []catalog.OverlayRecord{},
		StableBoundaries: []string{"between-slots"}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
	for index, definition := range definitions {
		transitions := []catalog.RecipeTransition{}
		if index+1 < len(definitions) {
			transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: definitions[index+1].ID}}
		}
		slot := catalog.SlotRecipe{SlotID: definition.ID, Applicability: catalog.SlotMandatory, Pipeline: []catalog.PipelineStep{}, Gates: []catalog.GateRecord{}, Transitions: transitions}
		switch definition.ID {
		case catalog.SlotWorkspacePreparation:
			slot.HostAction = &catalog.HostActionRef{ID: "workspace.prepare-or-confirm", InputArtifact: "artifact", OutputArtifact: "artifact"}
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerHostAction, HostAction: slot.HostAction.ID}
		case catalog.SlotIncidentRecovery:
			slot.Applicability = catalog.SlotConditional
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerNone}
		case catalog.SlotFreshVerification:
			slot.HostAction = &catalog.HostActionRef{ID: "verification.execute", InputArtifact: "artifact", OutputArtifact: "artifact"}
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerHostAction, HostAction: slot.HostAction.ID}
		case catalog.SlotCloseout:
			slot.HostAction = &catalog.HostActionRef{ID: "closeout.execute", InputArtifact: "artifact", OutputArtifact: "artifact"}
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerHostAction, HostAction: slot.HostAction.ID}
		default:
			step := catalog.PipelineStep{ID: "main", Selector: catalog.BindingSelector{ProviderID: "test/provider", BindingID: "binding"}, StageSpan: []catalog.SlotID{definition.ID}, RequiredInputArtifact: "artifact", ProducedOutputArtifact: "artifact"}
			slot.Pipeline = []catalog.PipelineStep{step}
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: step.ID}
		}
		record.Slots[index] = slot
	}
	return record
}

func configCanonicalSlotIDs() []catalog.SlotID {
	definitions := catalog.CanonicalSlots()
	result := make([]catalog.SlotID, len(definitions))
	for index, definition := range definitions {
		result[index] = definition.ID
	}
	return result
}

func settingsFor(values []ProviderSettings, providerID, hostID string) ProviderSettings {
	for _, value := range values {
		if value.ProviderID == providerID && value.HostID == hostID {
			return value
		}
	}
	return ProviderSettings{}
}
