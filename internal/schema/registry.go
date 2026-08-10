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
	ProviderDescriptorV4            = "https://open-agent-workflow.dev/schemas/v4/provider-descriptor.schema.json"
	ExecutionGraphV4                = "https://open-agent-workflow.dev/schemas/v4/execution-graph.schema.json"
	ProfileRecipeV3                 = "https://open-agent-workflow.dev/schemas/v3/profile-recipe.schema.json"
	ProfileAliasSetV1               = "https://open-agent-workflow.dev/schemas/v1/profile-alias-set.schema.json"
	UserConfigV3                    = "https://open-agent-workflow.dev/schemas/v3/user-config.schema.json"
	ProjectConfigV1                 = "https://open-agent-workflow.dev/schemas/v1/project-config.schema.json"
	ClassificationProposalV1        = "https://open-agent-workflow.dev/schemas/v1/classification-proposal.schema.json"
	HostManifestV3                  = "https://open-agent-workflow.dev/schemas/v3/host-manifest.schema.json"
	HostIntegrationV3               = "https://open-agent-workflow.dev/schemas/v3/host-integration.schema.json"
	HostIntegrationSetV3            = "https://open-agent-workflow.dev/schemas/v3/host-integration-set.schema.json"
	HostSessionV3                   = "https://open-agent-workflow.dev/schemas/v3/host-session.schema.json"
	HostEnvironmentReportV2         = "https://open-agent-workflow.dev/schemas/v2/host-environment-report.schema.json"
	HostBindingInventoryV3          = "https://open-agent-workflow.dev/schemas/v3/host-binding-inventory.schema.json"
	HostInvocationReceiptV3         = "https://open-agent-workflow.dev/schemas/v3/host-invocation-receipt.schema.json"
	HostConformanceTranscriptV4     = "https://open-agent-workflow.dev/schemas/v4/host-conformance-transcript.schema.json"
	HostConformanceReportV4         = "https://open-agent-workflow.dev/schemas/v4/host-conformance-report.schema.json"
	CapabilityGrantV3               = "https://open-agent-workflow.dev/schemas/v3/capability-grant.schema.json"
	UserAuthorizationV1             = "https://open-agent-workflow.dev/schemas/v1/user-authorization.schema.json"
	ExplicitInvocationAttestationV1 = "https://open-agent-workflow.dev/schemas/v1/explicit-invocation-attestation.schema.json"
	DispatchPacketV1                = "https://open-agent-workflow.dev/schemas/v1/dispatch-packet.schema.json"
	WorkflowCommandV1               = "https://open-agent-workflow.dev/schemas/v1/workflow-command.schema.json"
	WorkflowResultV1                = "https://open-agent-workflow.dev/schemas/v1/workflow-result.schema.json"
	WorkflowSnapshotV1              = "https://open-agent-workflow.dev/schemas/v1/workflow-snapshot.schema.json"
	WorkflowRevisionV1              = "https://open-agent-workflow.dev/schemas/v1/workflow-revision.schema.json"
	WorkflowHeadV1                  = "https://open-agent-workflow.dev/schemas/v1/workflow-head.schema.json"
)

type Registry struct {
	schemas map[string]*jsonschema.Schema
}

func New(files fs.FS) (*Registry, error) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	dependencies := []struct{ path, id string }{
		{"schemas/v2/capability-grant.schema.json", "https://open-agent-workflow.dev/schemas/v2/capability-grant.schema.json"},
		{"schemas/v2/host-invocation-receipt.schema.json", "https://open-agent-workflow.dev/schemas/v2/host-invocation-receipt.schema.json"},
	}
	resources := []struct{ path, id string }{
		{"schemas/v4/provider-descriptor.schema.json", ProviderDescriptorV4},
		{"schemas/v4/execution-graph.schema.json", ExecutionGraphV4},
		{"schemas/v3/user-config.schema.json", UserConfigV3},
		{"schemas/v3/profile-recipe.schema.json", ProfileRecipeV3},
		{"schemas/v1/profile-alias-set.schema.json", ProfileAliasSetV1},
		{"schemas/v1/project-config.schema.json", ProjectConfigV1},
		{"schemas/v1/classification-proposal.schema.json", ClassificationProposalV1},
		{"schemas/v3/host-manifest.schema.json", HostManifestV3},
		{"schemas/v3/host-integration.schema.json", HostIntegrationV3},
		{"schemas/v3/host-integration-set.schema.json", HostIntegrationSetV3},
		{"schemas/v3/host-session.schema.json", HostSessionV3},
		{"schemas/v2/host-environment-report.schema.json", HostEnvironmentReportV2},
		{"schemas/v3/host-binding-inventory.schema.json", HostBindingInventoryV3},
		{"schemas/v3/capability-grant.schema.json", CapabilityGrantV3},
		{"schemas/v1/user-authorization.schema.json", UserAuthorizationV1},
		{"schemas/v1/explicit-invocation-attestation.schema.json", ExplicitInvocationAttestationV1},
		{"schemas/v3/host-invocation-receipt.schema.json", HostInvocationReceiptV3},
		{"schemas/v4/host-conformance-transcript.schema.json", HostConformanceTranscriptV4},
		{"schemas/v4/host-conformance-report.schema.json", HostConformanceReportV4},
		{"schemas/v1/dispatch-packet.schema.json", DispatchPacketV1},
		{"schemas/v1/workflow-command.schema.json", WorkflowCommandV1},
		{"schemas/v1/workflow-result.schema.json", WorkflowResultV1},
		{"schemas/v1/workflow-snapshot.schema.json", WorkflowSnapshotV1},
		{"schemas/v1/workflow-revision.schema.json", WorkflowRevisionV1},
		{"schemas/v1/workflow-head.schema.json", WorkflowHeadV1},
	}
	for _, resource := range append(dependencies, resources...) {
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
