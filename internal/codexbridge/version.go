package codexbridge

import (
	"slices"
	"strconv"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	MinimumCodexBaseline    = "0.146.1"
	LifecycleBundleSchemaV3 = "oaw.lifecycle-bundle/v3"
)

type VersionEvidence struct {
	PluginVersion      string
	BridgeProtocol     string
	HookContextSchema  string
	IntegrationVersion string
	HostSessionSchema  string
	InventorySchema    string
	EnvironmentSchema  string
	BundleSchema       string
	WorkflowSchema     string
	CodexVersion       string
	MetadataMethods    []string
}

type Compatibility struct {
	Compatible             bool     `json:"compatible"`
	RequiredMethods        []string `json:"required_methods"`
	MissingOptionalMethods []string `json:"missing_optional_methods"`
	CodexVersion           string   `json:"codex_version"`
}

func Negotiate(value VersionEvidence) (Compatibility, error) {
	if value.PluginVersion != BridgeIntegrationVersion ||
		value.BridgeProtocol != BridgeProtocolVersion ||
		value.HookContextSchema != HookContextSchemaV1 ||
		value.IntegrationVersion != BridgeIntegrationVersion {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Bridge component versions differ", nil)
	}
	if value.HostSessionSchema != host.HostSessionSchemaV2 ||
		value.InventorySchema != host.BindingInventorySchemaV2 ||
		value.EnvironmentSchema != host.HostEnvironmentReportSchemaV2 ||
		value.BundleSchema != LifecycleBundleSchemaV3 ||
		value.WorkflowSchema != coordinator.WorkflowCommandSchemaV1 {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "public OAW schema versions differ", nil)
	}
	if compareCodexVersion(value.CodexVersion, MinimumCodexBaseline) < 0 {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Codex version is below the verified baseline", nil)
	}

	required := []string{"skills/list"}
	for _, method := range required {
		if !slices.Contains(value.MetadataMethods, method) {
			return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "required Codex metadata method is unavailable", nil)
		}
	}
	missingOptional := make([]string, 0, 2)
	for _, method := range []string{"config/read", "hooks/list"} {
		if !slices.Contains(value.MetadataMethods, method) {
			missingOptional = append(missingOptional, method)
		}
	}
	return Compatibility{
		Compatible:             true,
		RequiredMethods:        required,
		MissingOptionalMethods: missingOptional,
		CodexVersion:           value.CodexVersion,
	}, nil
}

func currentVersionEvidence(codexVersion string, metadataMethods []string) VersionEvidence {
	return VersionEvidence{
		PluginVersion:      BridgeIntegrationVersion,
		BridgeProtocol:     BridgeProtocolVersion,
		HookContextSchema:  HookContextSchemaV1,
		IntegrationVersion: BridgeIntegrationVersion,
		HostSessionSchema:  host.HostSessionSchemaV2,
		InventorySchema:    host.BindingInventorySchemaV2,
		EnvironmentSchema:  host.HostEnvironmentReportSchemaV2,
		BundleSchema:       LifecycleBundleSchemaV3,
		WorkflowSchema:     coordinator.WorkflowCommandSchemaV1,
		CodexVersion:       codexVersion,
		MetadataMethods:    append([]string{}, metadataMethods...),
	}
}

func compareCodexVersion(value, minimum string) int {
	parse := func(raw string) ([3]int, bool) {
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "codex-cli ")
		raw = strings.TrimPrefix(raw, "codex-cli/")
		if strings.ContainsAny(raw, "+-") {
			return [3]int{}, false
		}
		parts := strings.Split(raw, ".")
		if len(parts) != 3 {
			return [3]int{}, false
		}
		var result [3]int
		for index, part := range parts {
			parsed, err := strconv.Atoi(part)
			if err != nil || parsed < 0 {
				return [3]int{}, false
			}
			result[index] = parsed
		}
		return result, true
	}
	left, leftOK := parse(value)
	right, rightOK := parse(minimum)
	if !leftOK || !rightOK {
		return -1
	}
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}
