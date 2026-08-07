package appserver

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

const maximumMetadataEntries = 4096

func ProjectConfig(config map[string]any) (ConfigProjection, error) {
	projection := unknownConfigProjection()
	if config == nil {
		return projection, nil
	}
	if configuredValue(config, "sandbox_mode") {
		projection.SandboxDisposition = string(execution.DispositionHostConfigured)
	}
	if configuredValue(config, "mcp_servers") {
		projection.MCPDisposition = string(execution.DispositionHostConfigured)
	}
	if configuredNestedValue(config, "features", "hooks") {
		projection.HookDisposition = string(execution.DispositionHostConfigured)
	}
	if configuredValue(config, "approval_policy") {
		projection.ApprovalDisposition = string(execution.DispositionHostConfigured)
	}
	return projection, nil
}

func unknownConfigProjection() ConfigProjection {
	unknown := string(execution.DispositionUnknown)
	return ConfigProjection{
		SandboxDisposition:  unknown,
		MCPDisposition:      unknown,
		HookDisposition:     unknown,
		ApprovalDisposition: unknown,
	}
}

func configuredValue(values map[string]any, key string) bool {
	value, found := values[key]
	return found && value != nil
}

func configuredNestedValue(values map[string]any, parent, key string) bool {
	value, found := values[parent]
	if !found || value == nil {
		return false
	}
	nested, ok := value.(map[string]any)
	return ok && configuredValue(nested, key)
}

func normalizeMetadataErrors(values []metadataErrorWire) ([]MetadataError, error) {
	if len(values) > maximumMetadataEntries {
		return nil, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "metadata error list is oversized", nil)
	}
	result := make([]MetadataError, 0, len(values))
	for _, value := range values {
		if value.Message == nil || value.Path == nil {
			return nil, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "metadata error projection is incomplete", nil)
		}
		result = append(result, MetadataError{Message: "metadata source reported an error"})
	}
	return result, nil
}

func normalizeWarnings(values []string) ([]string, error) {
	if len(values) > maximumMetadataEntries {
		return nil, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "metadata warning list is oversized", nil)
	}
	result := make([]string, 0, len(values))
	for range values {
		result = append(result, "metadata source reported a warning")
	}
	return result, nil
}

func optionalMetadataFailure(surface string) []MetadataError {
	return []MetadataError{{Message: fmt.Sprintf("%s unavailable", surface)}}
}

func validMetadataText(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximum &&
		!hasControl(value)
}

func validOptionalMetadataText(value string, maximum int) bool {
	return value == "" || validMetadataText(value, maximum)
}

func hasControl(value string) bool {
	return !utf8.ValidString(value) || stringsIndexControl(value)
}

func stringsIndexControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}
