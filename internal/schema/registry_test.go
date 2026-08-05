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
	manifest := []byte(`{"schema_version":"oaw.host-manifest/v2","manifest_version":"2.0.0","host_id":"codex","control_surface":"policy","protocols":[],"binding_kinds":[],"supported_topologies":["CURRENT"],"features":[]}`)
	if err := registry.Validate(HostManifestV2, manifest); err != nil {
		t.Fatalf("Validate(Manifest) error = %v", err)
	}
	digest := strings.Repeat("a", 64)
	integration := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-integration/v2","integration_version":"2.0.0","id":"oaw/codex-policy","manifest":%s,"manifest_digest":"%s","audit":{"status":"pending","references":[],"digest":"%s"},"digest":"%s"}`, manifest, digest, digest, digest))
	if err := registry.Validate(HostIntegrationV2, integration); err != nil {
		t.Fatalf("Validate(Integration) error = %v", err)
	}
	invalid := []byte(strings.Replace(string(integration), `"features":[]`, `"features":["native-invocation"]`, 1))
	if err := registry.Validate(HostIntegrationV2, invalid); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(policy feature) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v1/host-manifest.schema.json", manifest); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Host Manifest v1) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v1/host-integration.schema.json", integration); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Host Integration v1) error = %v", err)
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

func TestRegistryValidatesHostNeutralGrantAndDispatchSchemas(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	grant := []byte(fmt.Sprintf(`{"schema_version":"oaw.capability-grant/v2","id":"grant-0123456789abcdef0123456789abcdef","workflow_id":"workflow-0123456789abcdef0123456789abcdef","request_id":"request-1","bundle_id":"bundle-0123456789abcdef0123456789abcdef","bundle_generation":1,"bundle_digest":"%s","node_id":"implementation","topology":"CURRENT","host_session_digest":"%s","provider_id":"oaw/superpowers","provider_instance_digest":"%s","capability_id":"implementation","binding":{"host":"codex","kind":"skill","reference":"superpowers:executing-plans","topologies":["CURRENT"]},"effects":["read-project"],"resources":["project"],"termination_condition":"complete","digest":"%s"}`, digest, digest, digest, digest))
	if err := registry.Validate(CapabilityGrantV2, grant); err != nil {
		t.Fatalf("Validate(CapabilityGrantV2) error = %v", err)
	}
	packet := []byte(fmt.Sprintf(`{"schema_version":"oaw.dispatch-packet/v1","id":"dispatch-0123456789abcdef0123456789abcdef","workflow_id":"workflow-0123456789abcdef0123456789abcdef","request_id":"request-1","bundle_id":"bundle-0123456789abcdef0123456789abcdef","bundle_generation":1,"bundle_digest":"%s","node_id":"implementation","ticket":"","topology":"CURRENT","host_session_digest":"%s","environment_report_digest":"%s","grant":%s,"input_references":[],"evidence_requirements":[],"environment_requirements":[],"digest":"%s"}`, digest, digest, digest, grant, digest))
	if err := registry.Validate(DispatchPacketV1, packet); err != nil {
		t.Fatalf("Validate(DispatchPacketV1) error = %v", err)
	}
	withExecutor := []byte(strings.Replace(string(grant), `"effects":`, `"executor":{"id":"agent-1","kind":"isolated"},"effects":`, 1))
	if err := registry.Validate(CapabilityGrantV2, withExecutor); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(Grant with executor) error = %v", err)
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

func TestRegistryValidatesHostSessionAndEnvironmentV2(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	session := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-session/v2","host_id":"codex","integration_id":"acme/codex-native","integration_version":"1.0.0","session_id":"session-current-1","supported_topologies":["CURRENT","SUBAGENT"],"provider_inventory_digest":"%s","environment_report_digest":"%s","sandbox_policy_digest":"","approval_policy_digest":"%s","digest":"%s"}`, digest, digest, digest, digest))
	if err := registry.Validate(HostSessionV2, session); err != nil {
		t.Fatalf("Validate(HostSessionV2) error = %v", err)
	}
	report := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-environment-report/v2","session_id":"session-current-1","parent_session_id":"","topology":"CURRENT","observations":[{"surface":"skills","disposition":"inherited","source":"codex-session","digest":"%s"}],"digest":"%s"}`, digest, digest))
	if err := registry.Validate(HostEnvironmentReportV2, report); err != nil {
		t.Fatalf("Validate(HostEnvironmentReportV2) error = %v", err)
	}
	invalid := []byte(strings.Replace(string(report), `"disposition":"inherited"`, `"disposition":"copied"`, 1))
	if err := registry.Validate(HostEnvironmentReportV2, invalid); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(invalid disposition) error = %v", err)
	}
}

func TestRegistryUsesHostBindingInventoryV2Only(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	inventory := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-binding-inventory/v2","host_id":"codex","observations":[{"host_id":"codex","installation_key":"installation-acme","binding":{"host":"codex","kind":"skill","reference":"acme:review","topologies":["CURRENT","SUBAGENT"]},"topologies":["CURRENT"],"source":"native-probe","evidence_reference":"evidence://binding/acme-review","digest":"%s"}],"digest":"%s"}`, digest, digest))
	if err := registry.Validate(HostBindingInventoryV2, inventory); err != nil {
		t.Fatalf("Validate(HostBindingInventoryV2) error = %v", err)
	}
	invalid := []byte(strings.Replace(string(inventory), `"topologies":["CURRENT"]`, `"topologies":[]`, 1))
	if err := registry.Validate(HostBindingInventoryV2, invalid); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(empty observed topologies) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v1/host-binding-inventory.schema.json", inventory); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Host Binding Inventory v1) error = %v", err)
	}
}

func TestRegistryValidatesReceiptTranscriptAndReportV2(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	receipt := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-invocation-receipt/v2","kind":"COMPLETED","workflow_id":"workflow-1","bundle_generation":1,"bundle_digest":"%s","node_id":"implementation","topology":"CURRENT","host_session_digest":"%s","dispatch_digest":"%s","invocation_handle":"","context_freshness":"shared","environment_report_digest":"%s","outcome":"succeeded","failure_code":"","evidence":[{"kind":"report","reference":"evidence://report","digest":"%s"}],"digest":"%s"}`, digest, digest, digest, digest, digest, digest))
	if err := registry.Validate(HostInvocationReceiptV2, receipt); err != nil {
		t.Fatalf("Validate(HostInvocationReceiptV2) error = %v", err)
	}
	transcript := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-conformance-transcript/v2","session":{"schema_version":"oaw.host-session/v2","host_id":"codex","integration_id":"acme/codex-host","integration_version":"2.0.0","session_id":"session-current","supported_topologies":["CURRENT"],"provider_inventory_digest":"%s","environment_report_digest":"%s","sandbox_policy_digest":"","approval_policy_digest":"","digest":"%s"},"inventory":{"schema_version":"oaw.host-binding-inventory/v2","host_id":"codex","observations":[],"digest":"%s"},"environment_reports":[{"schema_version":"oaw.host-environment-report/v2","session_id":"session-current","parent_session_id":"","topology":"CURRENT","observations":[],"digest":"%s"}],"receipts":[],"invocations":[],"digest":"%s"}`, digest, digest, digest, digest, digest, digest))
	if err := registry.Validate(HostConformanceTranscriptV2, transcript); err != nil {
		t.Fatalf("Validate(HostConformanceTranscriptV2) error = %v", err)
	}
	report := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-conformance-report/v2","manifest_digest":"%s","transcript_digest":"%s","verified_features":["normalized-receipts"],"diagnostics":[],"digest":"%s"}`, digest, digest, digest))
	if err := registry.Validate(HostConformanceReportV2, report); err != nil {
		t.Fatalf("Validate(HostConformanceReportV2) error = %v", err)
	}
	legacyReport := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-conformance-report/v1","suite_version":"oaw.host-conformance/v1","integration_id":"acme/codex-host","manifest_digest":"%s","checks":[],"transcript_digest":"%s","passed":true,"digest":"%s"}`, digest, digest, digest))
	if err := registry.Validate(HostConformanceReportV2, legacyReport); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(legacy report) error = %v", err)
	}
}

func TestRegistryValidatesWorkflowCoordinatorSchemaFamily(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	command := []byte(`{"schema_version":"oaw.workflow-command/v1","kind":"INSPECT","message_id":"","idempotency_key":"","workflow_id":"workflow-1","expected_revision":0}`)
	if err := registry.Validate(WorkflowCommandV1, command); err != nil {
		t.Fatalf("Validate(WorkflowCommandV1) error = %v", err)
	}
	start := []byte(fmt.Sprintf(`{"schema_version":"oaw.workflow-command/v1","kind":"START","message_id":"message-1","idempotency_key":"start-1","workflow_id":"","expected_revision":0,"start":{"request_id":"request-1","deliverable_id":"deliverable-1","input_digest":"%s","active_ticket":"","proposal":{"schema_version":"oaw.classification-proposal/v1","traits":[],"resources":[],"evidence":[]},"selection":{"profile":"SP-FULL","profile_source":"user-selection","topology":"CURRENT","topology_source":"host-only-option","add_ons":[],"bindings":[]},"host_session":{"schema_version":"oaw.host-session/v2","host_id":"codex","integration_id":"acme/codex","integration_version":"2.0.0","session_id":"session-1","supported_topologies":["CURRENT"],"provider_inventory_digest":"%s","environment_report_digest":"%s","sandbox_policy_digest":"","approval_policy_digest":"","digest":"%s"},"environment":{"schema_version":"oaw.host-environment-report/v2","session_id":"session-1","parent_session_id":"","topology":"CURRENT","observations":[],"digest":"%s"}}}`, digest, digest, digest, digest, digest))
	if err := registry.Validate(WorkflowCommandV1, start); err != nil {
		t.Fatalf("Validate(START) error = %v", err)
	}
	invalidStart := []byte(strings.Replace(string(start), `"message_id":"message-1"`, `"message_id":""`, 1))
	if err := registry.Validate(WorkflowCommandV1, invalidStart); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(empty START message_id) error = %v", err)
	}
	invalidInspect := []byte(strings.Replace(string(command), `"workflow_id":"workflow-1"`, `"workflow_id":""`, 1))
	if err := registry.Validate(WorkflowCommandV1, invalidInspect); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(empty INSPECT workflow_id) error = %v", err)
	}
	mixed := []byte(`{"schema_version":"oaw.workflow-command/v1","kind":"INSPECT","message_id":"","idempotency_key":"","workflow_id":"workflow-1","expected_revision":0,"cancel":{"reason":"stop","invocation_terminal":true}}`)
	if err := registry.Validate(WorkflowCommandV1, mixed); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(mixed Workflow Command) error = %v", err)
	}
	legacy := []byte(`{"schema_version":"oaw.runtime/v1","kind":"INSPECT","message_id":"","idempotency_key":"","workflow_id":"workflow-1","expected_revision":0}`)
	if err := registry.Validate(WorkflowCommandV1, legacy); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(legacy Runtime Command) error = %v", err)
	}
	snapshot := `{"schema_version":"oaw.workflow-snapshot/v1","workflow_id":"workflow-1","request_id":"request-1","deliverable_id":"deliverable-1","revision":1,"status":"READY","classification":{"request_mode":"WORKFLOW","workflow_complexity":"complex","risk_class":"normal","evidence_requirements":[],"escalation_reasons":[]},"bundles":[],"active_generation":0,"active_node_id":"","active_ticket":"","grant_history":[],"receipts":[],"resource_leases":[],"last_stable_boundary":"","processed_messages":[],"projection_lag":[]}`
	if err := registry.Validate(WorkflowSnapshotV1, []byte(snapshot)); err != nil {
		t.Fatalf("Validate(WorkflowSnapshotV1) error = %v", err)
	}
	result := fmt.Sprintf(`{"schema_version":"oaw.workflow-result/v1","kind":"STATE","workflow_id":"workflow-1","revision":1,"revision_digest":"%s","snapshot":%s,"diagnostics":[],"replayed":false,"digest":"%s"}`, digest, snapshot, digest)
	if err := registry.Validate(WorkflowResultV1, []byte(result)); err != nil {
		t.Fatalf("Validate(WorkflowResultV1) error = %v", err)
	}
	persistedRejected := fmt.Sprintf(`{"schema_version":"oaw.workflow-result/v1","kind":"REJECTED","workflow_id":"workflow-1","revision":2,"revision_digest":"%s","diagnostics":[{"code":"WORKFLOW_DENIED","detail":"selection is not eligible"}],"replayed":false,"digest":"%s"}`, digest, digest)
	if err := registry.Validate(WorkflowResultV1, []byte(persistedRejected)); err != nil {
		t.Fatalf("Validate(persisted REJECTED) error = %v", err)
	}
	revision := fmt.Sprintf(`{"schema_version":"oaw.workflow-revision/v1","workflow_id":"workflow-1","revision":1,"predecessor_digest":"","message_id":"message-1","idempotency_key":"start-1","message_digest":"%s","event":"WORKFLOW_STARTED","snapshot":%s,"result":%s,"digest":"%s"}`, digest, snapshot, result, digest)
	if err := registry.Validate(WorkflowRevisionV1, []byte(revision)); err != nil {
		t.Fatalf("Validate(WorkflowRevisionV1) error = %v", err)
	}
	head := []byte(fmt.Sprintf(`{"schema_version":"oaw.workflow-head/v1","workflow_id":"workflow-1","revision":1,"revision_digest":"%s","digest":"%s"}`, digest, digest))
	if err := registry.Validate(WorkflowHeadV1, head); err != nil {
		t.Fatalf("Validate(WorkflowHeadV1) error = %v", err)
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
