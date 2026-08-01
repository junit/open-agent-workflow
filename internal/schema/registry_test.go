package schema

import (
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

func TestRegistryValidatesKnownSchemaAndRejectsUnknown(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	valid := []byte(`{"schema_version":"oaw.provider-descriptor/v1","descriptor_version":"1.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV1, valid); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}
	if err := registry.Validate("oaw.capability-input/v1", valid); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(unknown) error = %v", err)
	}
}

func TestRegistryRejectsTrailingJSON(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	raw := []byte(`{"schema_version":"oaw.provider-descriptor/v1","descriptor_version":"1.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[]} {}`)
	if err := registry.Validate(ProviderDescriptorV1, raw); err == nil || !strings.Contains(err.Error(), "SCHEMA_INPUT_INVALID") {
		t.Fatalf("Validate(trailing) error = %v", err)
	}
}

func TestRegistryRejectsSchemaViolations(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	unknownField := []byte(`{"schema_version":"oaw.provider-descriptor/v1","descriptor_version":"1.0.0","id":"oaw/test","display_name":"Test","discovery":[],"capabilities":[],"extra":true}`)
	if err := registry.Validate(ProviderDescriptorV1, unknownField); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(unknown field) error = %v", err)
	}
	unsafePath := []byte(`{"schema_version":"oaw.provider-descriptor/v1","descriptor_version":"1.0.0","id":"oaw/test","display_name":"Test","discovery":[{"id":"p","kind":"path-exists","root":"user-home","path":".agents/../secret"}],"capabilities":[]}`)
	if err := registry.Validate(ProviderDescriptorV1, unsafePath); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(unsafe path) error = %v", err)
	}
}

func TestRegistryValidatesNormalizedConfigurationSchemas(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	user := []byte(`{"schema_version":"oaw.user-config/v1","denied_providers":[],"provider_descriptors":[],"profile_recipes":[],"provider_pins":[],"binding_preferences":[],"bounded_capability_defaults":[],"project_trust":[]}`)
	project := []byte(`{"schema_version":"oaw.project-config/v1","required_providers":[],"recommended_providers":[],"provider_descriptors":[],"profile_recipes":[],"capability_limits":[]}`)
	if err := registry.Validate(UserConfigV1, user); err != nil {
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
