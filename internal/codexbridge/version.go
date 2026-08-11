package codexbridge

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

var (
	codexVersionToken  = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?`)
	pluginVersionToken = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
)

const MinimumCodexBaseline = "0.146.1"

type VersionEvidence struct {
	PluginVersion                   string   `json:"plugin_version"`
	BridgeProtocol                  string   `json:"bridge_protocol"`
	HookContextSchema               string   `json:"hook_context_schema"`
	IntegrationVersion              string   `json:"integration_version"`
	CodexVersion                    string   `json:"codex_version"`
	MetadataMethods                 []string `json:"metadata_methods"`
	ProviderDescriptorSchema        string   `json:"provider_descriptor_schema"`
	ProfileRecipeSchema             string   `json:"profile_recipe_schema"`
	HostManifestSchema              string   `json:"host_manifest_schema"`
	HostSessionSchema               string   `json:"host_session_schema"`
	HostBindingInventorySchema      string   `json:"host_binding_inventory_schema"`
	HostEnvironmentReportSchema     string   `json:"host_environment_report_schema"`
	HostInvocationReceiptSchema     string   `json:"host_invocation_receipt_schema"`
	HostConformanceTranscriptSchema string   `json:"host_conformance_transcript_schema"`
	HostConformanceReportSchema     string   `json:"host_conformance_report_schema"`
	ExecutionGraphSchema            string   `json:"execution_graph_schema"`
	LifecycleBundleSchema           string   `json:"lifecycle_bundle_schema"`
	CapabilityGrantSchema           string   `json:"capability_grant_schema"`
	DispatchPacketSchema            string   `json:"dispatch_packet_schema"`
	WorkflowCommandSchema           string   `json:"workflow_command_schema"`
	WorkflowResultSchema            string   `json:"workflow_result_schema"`
	WorkflowSnapshotSchema          string   `json:"workflow_snapshot_schema"`
	WorkflowRevisionSchema          string   `json:"workflow_revision_schema"`
	Digest                          string   `json:"digest"`
}

type Compatibility struct {
	Compatible             bool     `json:"compatible"`
	RequiredMethods        []string `json:"required_methods"`
	MissingOptionalMethods []string `json:"missing_optional_methods"`
	CodexVersion           string   `json:"codex_version"`
}

func Negotiate(value VersionEvidence) (Compatibility, error) {
	input := cloneVersionEvidence(value)
	digest, err := digestVersionEvidence(input)
	if err != nil || input.Digest == "" || input.Digest != digest {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "VersionEvidence digest is invalid", err)
	}
	if !pluginVersionToken.MatchString(input.PluginVersion) || input.BridgeProtocol != BridgeProtocolVersion ||
		input.HookContextSchema != HookContextSchemaV2 || input.IntegrationVersion != BridgeIntegrationVersion ||
		input.ProviderDescriptorSchema != catalog.ProviderDescriptorSchemaV4 || input.ProfileRecipeSchema != catalog.ProfileRecipeSchemaV3 ||
		input.HostManifestSchema != host.HostManifestSchemaV3 || input.HostSessionSchema != host.HostSessionSchemaV3 ||
		input.HostBindingInventorySchema != host.BindingInventorySchemaV3 || input.HostEnvironmentReportSchema != host.HostEnvironmentReportSchemaV2 ||
		input.HostInvocationReceiptSchema != host.HostInvocationReceiptSchemaV3 ||
		input.HostConformanceTranscriptSchema != host.HostConformanceTranscriptSchemaV4 || input.HostConformanceReportSchema != host.HostConformanceReportSchemaV4 ||
		input.ExecutionGraphSchema != profile.ExecutionGraphSchemaV4 || input.LifecycleBundleSchema != core.LifecycleBundleSchemaV4 ||
		input.CapabilityGrantSchema != admission.CapabilityGrantSchemaV3 || input.DispatchPacketSchema != coordinator.DispatchPacketSchemaV2 ||
		input.WorkflowCommandSchema != coordinator.WorkflowCommandSchemaV2 || input.WorkflowResultSchema != coordinator.WorkflowResultSchemaV2 ||
		input.WorkflowSnapshotSchema != coordinator.WorkflowSnapshotSchemaV2 || input.WorkflowRevisionSchema != coordinator.WorkflowRevisionSchemaV2 {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Bridge authority tuple differs", nil)
	}
	if compareCodexVersion(input.CodexVersion, MinimumCodexBaseline) < 0 {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Codex version is below the verified baseline", nil)
	}
	if !validVersionMethodSet(input.MetadataMethods) {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Codex metadata method evidence is not canonical", nil)
	}
	required := []string{"skills/list"}
	if !slices.Contains(input.MetadataMethods, required[0]) {
		return Compatibility{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "required Codex metadata method is unavailable", nil)
	}
	missingOptional := make([]string, 0, 2)
	for _, method := range []string{"config/read", "hooks/list"} {
		if !slices.Contains(input.MetadataMethods, method) {
			missingOptional = append(missingOptional, method)
		}
	}
	return Compatibility{Compatible: true, RequiredMethods: required, MissingOptionalMethods: missingOptional, CodexVersion: input.CodexVersion}, nil
}

func currentVersionEvidence(pluginVersion, codexVersion string, metadataMethods []string) (VersionEvidence, error) {
	methods := append([]string{}, metadataMethods...)
	sort.Strings(methods)
	if !validVersionMethodSet(methods) {
		return VersionEvidence{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Codex metadata method evidence is invalid", nil)
	}
	value := VersionEvidence{
		PluginVersion: pluginVersion, BridgeProtocol: BridgeProtocolVersion, HookContextSchema: HookContextSchemaV2,
		IntegrationVersion: BridgeIntegrationVersion, CodexVersion: codexVersion, MetadataMethods: methods,
		ProviderDescriptorSchema: catalog.ProviderDescriptorSchemaV4, ProfileRecipeSchema: catalog.ProfileRecipeSchemaV3,
		HostManifestSchema: host.HostManifestSchemaV3, HostSessionSchema: host.HostSessionSchemaV3,
		HostBindingInventorySchema: host.BindingInventorySchemaV3, HostEnvironmentReportSchema: host.HostEnvironmentReportSchemaV2,
		HostInvocationReceiptSchema:     host.HostInvocationReceiptSchemaV3,
		HostConformanceTranscriptSchema: host.HostConformanceTranscriptSchemaV4, HostConformanceReportSchema: host.HostConformanceReportSchemaV4,
		ExecutionGraphSchema: profile.ExecutionGraphSchemaV4, LifecycleBundleSchema: core.LifecycleBundleSchemaV4,
		CapabilityGrantSchema: admission.CapabilityGrantSchemaV3, DispatchPacketSchema: coordinator.DispatchPacketSchemaV2,
		WorkflowCommandSchema: coordinator.WorkflowCommandSchemaV2, WorkflowResultSchema: coordinator.WorkflowResultSchemaV2,
		WorkflowSnapshotSchema: coordinator.WorkflowSnapshotSchemaV2, WorkflowRevisionSchema: coordinator.WorkflowRevisionSchemaV2,
	}
	digest, err := digestVersionEvidence(value)
	if err != nil {
		return VersionEvidence{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "VersionEvidence cannot be canonicalized", err)
	}
	value.Digest = digest
	return value, nil
}

func digestVersionEvidence(value VersionEvidence) (string, error) {
	value = cloneVersionEvidence(value)
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(value)
	return digest, err
}

func cloneVersionEvidence(value VersionEvidence) VersionEvidence {
	value.MetadataMethods = append([]string{}, value.MetadataMethods...)
	return value
}

func validVersionMethodSet(methods []string) bool {
	if !slices.IsSorted(methods) {
		return false
	}
	for index, method := range methods {
		if method != "config/read" && method != "hooks/list" && method != "skills/list" || index > 0 && methods[index-1] == method {
			return false
		}
	}
	return true
}

func compareCodexVersion(value, minimum string) int {
	left, leftOK := parseCodexVersion(value)
	right, rightOK := parseCodexVersion(minimum)
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

func parseCodexVersion(raw string) ([3]int, bool) {
	raw = strings.TrimSpace(raw)
	for _, prefix := range []string{"codex-cli ", "codex-cli/"} {
		if strings.HasPrefix(raw, prefix) {
			raw = strings.TrimPrefix(raw, prefix)
			break
		}
	}
	if parsed, ok := parseVersionTriplet(raw); ok {
		return parsed, true
	}
	match := codexVersionToken.FindStringIndex(raw)
	if match == nil || !strings.Contains(strings.ToLower(raw[:match[0]]), "codex") || strings.ContainsAny(raw[match[0]:match[1]], "+-") {
		return [3]int{}, false
	}
	return parseVersionTriplet(raw[match[0]:match[1]])
}

func parseVersionTriplet(raw string) ([3]int, bool) {
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
