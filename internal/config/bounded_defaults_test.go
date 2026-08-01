package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/schema"
)

func TestDecodeUserNormalizesNamedBoundedCapabilityDefaults(t *testing.T) {
	registry := testSchemaRegistry(t)
	decoded, err := config.DecodeUser([]byte(`
schema_version = "oaw.user-config/v1"

[[bounded_capability_defaults]]
id = "zeta-review"
provider_id = "oaw/ecc"
capability_id = "review"

[[bounded_capability_defaults]]
id = "alpha-review"
provider_id = "acme/suite"
capability_id = "review"
`), registry)
	if err != nil {
		t.Fatalf("DecodeUser() error = %v", err)
	}
	defaults := decoded.Record.BoundedCapabilityDefaults
	if len(defaults) != 2 || defaults[0].ID != "alpha-review" || defaults[1].ID != "zeta-review" {
		t.Fatalf("normalized defaults = %#v", defaults)
	}
	if decoded.Digest == "" || len(decoded.CanonicalJSON) == 0 {
		t.Fatalf("decoded digest/canonical JSON missing: %#v", decoded)
	}
}

func TestDecodeUserRejectsInvalidOrDuplicateBoundedDefaults(t *testing.T) {
	registry := testSchemaRegistry(t)
	for _, test := range []struct {
		name   string
		values string
		code   string
	}{
		{
			name: "duplicate rule ID",
			values: `
[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/ecc"
capability_id = "review"
[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/superpowers"
capability_id = "review"
`,
			code: "DUPLICATE_BOUNDED_CAPABILITY_DEFAULT",
		},
		{
			name: "invalid rule ID",
			values: `
[[bounded_capability_defaults]]
id = "Bad Rule"
provider_id = "oaw/ecc"
capability_id = "review"
`,
			code: "INVALID_BOUNDED_CAPABILITY_DEFAULT",
		},
		{
			name: "invalid provider ID",
			values: `
[[bounded_capability_defaults]]
id = "review"
provider_id = "bad"
capability_id = "review"
`,
			code: "INVALID_BOUNDED_CAPABILITY_DEFAULT",
		},
		{
			name: "invalid capability ID",
			values: `
[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/ecc"
capability_id = "Bad Capability"
`,
			code: "INVALID_BOUNDED_CAPABILITY_DEFAULT",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := config.DecodeUser([]byte("schema_version = \"oaw.user-config/v1\"\n"+test.values), registry)
			if err == nil || !containsConfigCode(err.Error(), test.code) {
				t.Fatalf("DecodeUser() error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestProjectCannotDeclareBoundedCapabilityDefaults(t *testing.T) {
	registry := testSchemaRegistry(t)
	_, err := config.DecodeProject([]byte(`
schema_version = "oaw.project-config/v1"
[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/ecc"
capability_id = "review"
`), registry)
	if err == nil || !containsConfigCode(err.Error(), "CONFIG_UNKNOWN_FIELD") {
		t.Fatalf("DecodeProject() error = %v", err)
	}
}

func TestSnapshotPinsAndDefensivelyCopiesBoundedDefaults(t *testing.T) {
	empty, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatalf("Load(empty) error = %v", err)
	}
	userRoot := t.TempDir()
	writeConfigFile(t, filepath.Join(userRoot, "config.toml"), `
schema_version = "oaw.user-config/v1"
[[bounded_capability_defaults]]
id = "review"
provider_id = "oaw/ecc"
capability_id = "review"
`)
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: userRoot})
	if err != nil {
		t.Fatalf("Load(user defaults) error = %v", err)
	}
	if snapshot.Digest() == empty.Digest() {
		t.Fatal("bounded default did not change configuration snapshot digest")
	}
	first := snapshot.BoundedCapabilityDefaults()
	if len(first) != 1 || first[0].ProviderID != "oaw/ecc" {
		t.Fatalf("defaults = %#v", first)
	}
	first[0].ProviderID = "changed/provider"
	second := snapshot.BoundedCapabilityDefaults()
	if second[0].ProviderID != "oaw/ecc" {
		t.Fatal("snapshot exposed mutable bounded defaults")
	}
}

func testSchemaRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	registry, err := schema.New(assets.FS())
	if err != nil {
		t.Fatalf("schema.New() error = %v", err)
	}
	return registry
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func containsConfigCode(value, code string) bool {
	for index := 0; index+len(code) <= len(value); index++ {
		if value[index:index+len(code)] == code {
			return true
		}
	}
	return false
}
