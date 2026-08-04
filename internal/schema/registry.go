package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	ProviderDescriptorV2     = "https://open-agent-workflow.dev/schemas/v2/provider-descriptor.schema.json"
	ProfileRecipeV1          = "https://open-agent-workflow.dev/schemas/v1/profile-recipe.schema.json"
	ProfileAliasSetV1        = "https://open-agent-workflow.dev/schemas/v1/profile-alias-set.schema.json"
	UserConfigV2             = "https://open-agent-workflow.dev/schemas/v2/user-config.schema.json"
	ProjectConfigV1          = "https://open-agent-workflow.dev/schemas/v1/project-config.schema.json"
	ClassificationProposalV1 = "https://open-agent-workflow.dev/schemas/v1/classification-proposal.schema.json"
	HostManifestV1           = "https://open-agent-workflow.dev/schemas/v1/host-manifest.schema.json"
	HostIntegrationV1        = "https://open-agent-workflow.dev/schemas/v1/host-integration.schema.json"
	HostIntegrationSetV1     = "https://open-agent-workflow.dev/schemas/v1/host-integration-set.schema.json"
	RuntimeFrameV1           = "https://open-agent-workflow.dev/schemas/v1/runtime-frame.schema.json"
	RuntimeReplyV1           = "https://open-agent-workflow.dev/schemas/v1/runtime-reply.schema.json"
)

type Registry struct {
	schemas map[string]*jsonschema.Schema
}

func New(files fs.FS) (*Registry, error) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	resources := []struct{ path, id string }{
		{"schemas/v2/provider-descriptor.schema.json", ProviderDescriptorV2},
		{"schemas/v2/user-config.schema.json", UserConfigV2},
		{"schemas/v1/profile-recipe.schema.json", ProfileRecipeV1},
		{"schemas/v1/profile-alias-set.schema.json", ProfileAliasSetV1},
		{"schemas/v1/project-config.schema.json", ProjectConfigV1},
		{"schemas/v1/classification-proposal.schema.json", ClassificationProposalV1},
		{"schemas/v1/host-manifest.schema.json", HostManifestV1},
		{"schemas/v1/host-integration.schema.json", HostIntegrationV1},
		{"schemas/v1/host-integration-set.schema.json", HostIntegrationSetV1},
		{"schemas/v1/runtime-frame.schema.json", RuntimeFrameV1},
		{"schemas/v1/runtime-reply.schema.json", RuntimeReplyV1},
	}
	for _, resource := range resources {
		data, err := fs.ReadFile(files, resource.path)
		if err != nil {
			return nil, fmt.Errorf("SCHEMA_READ_FAILED: %s: %w", resource.path, err)
		}
		var document any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&document); err != nil {
			return nil, fmt.Errorf("SCHEMA_DECODE_FAILED: %s: %w", resource.path, err)
		}
		if err := compiler.AddResource(resource.id, document); err != nil {
			return nil, fmt.Errorf("SCHEMA_REGISTER_FAILED: %s: %w", resource.id, err)
		}
	}
	compiled := make(map[string]*jsonschema.Schema, len(resources))
	for _, resource := range resources {
		value, err := compiler.Compile(resource.id)
		if err != nil {
			return nil, fmt.Errorf("SCHEMA_COMPILE_FAILED: %s: %w", resource.id, err)
		}
		compiled[resource.id] = value
	}
	return &Registry{schemas: compiled}, nil
}

func (registry *Registry) Validate(schemaID string, raw []byte) error {
	schema, ok := registry.schemas[schemaID]
	if !ok {
		return fmt.Errorf("UNKNOWN_SCHEMA: %s", schemaID)
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("SCHEMA_INPUT_INVALID: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("SCHEMA_INPUT_INVALID: trailing JSON value")
		}
		return fmt.Errorf("SCHEMA_INPUT_INVALID: %w", err)
	}
	if err := schema.Validate(value); err != nil {
		return fmt.Errorf("SCHEMA_VALIDATION_FAILED: %w", err)
	}
	return nil
}
