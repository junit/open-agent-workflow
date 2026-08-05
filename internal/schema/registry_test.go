package schema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
)

func TestNewCompilesEmbeddedSchemas(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if registry == nil {
		t.Fatal("New() returned nil registry")
	}
}

func TestRegistryValidatesClosedHostSchemas(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte(`{"schema_version":"oaw.host-manifest/v1","manifest_version":"1.0.0","host_id":"codex","integration_level":"instruction-only","protocols":[],"binding_kinds":[],"features":[]}`)
	if err := registry.Validate(HostManifestV1, manifest); err != nil {
		t.Fatalf("Validate(Manifest) error = %v", err)
	}
	digest := strings.Repeat("a", 64)
	integration := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-integration/v1","integration_version":"1.0.0","id":"oaw/codex-instruction","manifest":%s,"manifest_digest":"%s","audit":{"status":"pending","references":[],"digest":"%s"},"digest":"%s"}`, manifest, digest, digest, digest))
	if err := registry.Validate(HostIntegrationV1, integration); err != nil {
		t.Fatalf("Validate(Integration) error = %v", err)
	}
	invalid := []byte(strings.Replace(string(integration), `"features":[]`, `"features":["isolated-executor"]`, 1))
	if err := registry.Validate(HostIntegrationV1, invalid); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(instruction-only feature) error = %v", err)
	}
}

func TestRegistryValidatesKnownSchemaAndRejectsUnknown(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	valid := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"3.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV3, valid); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}
	if err := registry.Validate("oaw.capability-input/v1", valid); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(unknown) error = %v", err)
	}
}

func TestRegistryUsesProviderV3AndRecipeV2Only(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	provider := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"3.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV3, provider); err != nil {
		t.Fatalf("Validate(provider v3) error = %v", err)
	}
	recipe := []byte(`{"schema_version":"oaw.profile-recipe/v2","recipe_version":"2.0.0","id":"oaw/test","display_name":"Test","required_responsibilities":[],"nodes":[],"incident_routes":[],"entry":"start","terminal_gates":[],"stable_boundaries":[],"environment_requirements":[]}`)
	if err := registry.Validate(ProfileRecipeV2, recipe); err != nil {
		t.Fatalf("Validate(recipe v2) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/provider-descriptor.schema.json", provider); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Provider v2) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v1/profile-recipe.schema.json", recipe); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Recipe v1) error = %v", err)
	}
}

func TestRegistryValidatesHostScopedProviderDescriptorV3(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"3.0.0","id":"acme/suite","display_name":"Acme Suite","discovery":[{"id":"codex","hosts":["codex"],"surface":"codex-skills","distribution":"acme","kind":"path-exists","root":"user-home","candidate_path":".agents/skills/acme","evidence_path":"review/SKILL.md"}],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV3, raw); err != nil {
		t.Fatalf("Validate(v3 descriptor) error = %v", err)
	}
}

func TestRegistryRejectsV2ProviderDescriptorFromActiveV3Schema(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v2","descriptor_version":"2.0.0","id":"acme/suite","display_name":"Acme Suite","discovery":[],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV3, raw); err == nil {
		t.Fatal("v2 descriptor unexpectedly validated against v3")
	}
}

func TestRegistryUsesUserConfigV3Only(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"schema_version":"oaw.user-config/v3","denied_providers":[],"provider_descriptors":[],"profile_recipes":[],"host_integrations":[],"provider_installations":[],"provider_pins":[],"binding_preferences":[],"bounded_capability_defaults":[],"project_trust":[]}`)
	if err := registry.Validate(UserConfigV3, raw); err != nil {
		t.Fatalf("Validate(v3 user config) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/user-config.schema.json", raw); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired v2 user config) error = %v", err)
	}
}

func TestRegistryRejectsTrailingJSON(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"3.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]} {}`)
	if err := registry.Validate(ProviderDescriptorV3, raw); err == nil || !strings.Contains(err.Error(), "SCHEMA_INPUT_INVALID") {
		t.Fatalf("Validate(trailing) error = %v", err)
	}
}

func TestRegistryRejectsSchemaViolations(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	unknownField := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"3.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[],"extra":true}`)
	if err := registry.Validate(ProviderDescriptorV3, unknownField); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(unknown field) error = %v", err)
	}
	unsafePath := []byte(`{"schema_version":"oaw.provider-descriptor/v3","descriptor_version":"3.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"p","hosts":["codex"],"surface":"codex-skills","distribution":"test","kind":"path-exists","root":"user-home","candidate_path":".agents/../secret","evidence_path":"SKILL.md"}],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV3, unsafePath); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(unsafe path) error = %v", err)
	}
}

func TestRegistryValidatesNormalizedConfigurationSchemas(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	user := []byte(`{"schema_version":"oaw.user-config/v3","denied_providers":[],"provider_descriptors":[],"profile_recipes":[],"host_integrations":[],"provider_installations":[],"provider_pins":[],"binding_preferences":[],"bounded_capability_defaults":[],"project_trust":[]}`)
	project := []byte(`{"schema_version":"oaw.project-config/v1","required_providers":[],"recommended_providers":[],"provider_descriptors":[],"profile_recipes":[],"capability_limits":[]}`)
	if err := registry.Validate(UserConfigV3, user); err != nil {
		t.Fatalf("Validate(user) error = %v", err)
	}
	if err := registry.Validate(ProjectConfigV1, project); err != nil {
		t.Fatalf("Validate(project) error = %v", err)
	}
}

func TestProjectConfigSchemaRejectsAuthorityFields(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	raw := []byte(`{"schema_version":"oaw.project-config/v1","required_providers":[],"recommended_providers":[],"provider_descriptors":[],"profile_recipes":[],"capability_limits":[],"enabled_providers":["acme/suite"]}`)
	if err := registry.Validate(ProjectConfigV1, raw); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(project authority) error = %v", err)
	}
}

func TestRegistryValidatesClassificationProposalSchema(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	valid := []byte(`{"schema_version":"oaw.classification-proposal/v1","traits":[],"resources":[],"evidence":[]}`)
	if err := registry.Validate(ClassificationProposalV1, valid); err != nil {
		t.Fatalf("Validate(classification) error = %v", err)
	}
	invalid := []byte(`{"schema_version":"oaw.classification-proposal/v1","traits":[],"resources":[],"evidence":[],"extra":true}`)
	if err := registry.Validate(ClassificationProposalV1, invalid); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(classification extra) error = %v", err)
	}
}
