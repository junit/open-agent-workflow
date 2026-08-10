package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
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
	digest := strings.Repeat("a", 64)
	manifest := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-manifest/v3","manifest_version":"3.0.0","host_id":"codex","control_surface":"policy","protocols":[],"binding_kinds":[],"supported_topologies":["CURRENT"],"features":[],"delegation_features":[],"host_actions":[],"digest":"%s"}`, digest))
	if err := registry.Validate(HostManifestV3, manifest); err != nil {
		t.Fatalf("Validate(Manifest) error = %v", err)
	}
	integration := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-integration/v3","integration_version":"3.0.0","id":"oaw/codex-policy","manifest":%s,"manifest_digest":"%s","audit":{"status":"pending","references":[],"digest":"%s"},"digest":"%s"}`, manifest, digest, digest, digest))
	if err := registry.Validate(HostIntegrationV3, integration); err != nil {
		t.Fatalf("Validate(Integration) error = %v", err)
	}
	invalid := []byte(strings.Replace(string(integration), `"features":[]`, `"features":["native-invocation"]`, 1))
	if err := registry.Validate(HostIntegrationV3, invalid); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(policy feature) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/host-manifest.schema.json", manifest); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Host Manifest v2) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/host-integration.schema.json", integration); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Host Integration v2) error = %v", err)
	}
}

func TestRegistryValidatesKnownSchemaAndRejectsUnknown(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	valid := validProviderV4JSON(t)
	if err := registry.Validate(ProviderDescriptorV4, valid); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}
	if err := registry.Validate("oaw.capability-input/v1", valid); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(unknown) error = %v", err)
	}
}

func TestRegistryRejectsRetiredGrantAuthorityBeforeCoordinatorCutover(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/capability-grant.schema.json", []byte(`{}`)); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Grant v2) error = %v", err)
	}
}

func TestRegistryValidatesGrantV3AuthorizationReceiptV3ConformanceV4AndRejectsOldAuthority(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	tree := "sha256:" + digest
	cursor := `{"slot_id":"implementation","kind":"binding","unit_id":"implementation-main","ordinal":1}`
	target := fmt.Sprintf(`{"target_kind":"provider-binding","provider_binding":{"provider_id":"acme/provider","provider_instance_digest":"%s","distribution_id":"distribution","distribution_revision":"0123456789abcdef0123456789abcdef01234567","distribution_tree_digest":"%s","binding_id":"binding","surface":"codex-skills","kind":"skill","reference":"acme:implementation","invocation":"model","binding_tree_digest":"%s","binding_evidence_digest":"%s","input_artifact":"workflow-input","output_artifact":"workflow-output","input_schema":"oaw.workflow-input/v1","outcome_schema":"oaw.workflow-output/v1","requires_explicit_invocation":false}}`, digest, tree, tree, digest)
	explicitTarget := strings.Replace(strings.TrimSuffix(strings.TrimPrefix(target, `{"target_kind":"provider-binding","provider_binding":`), `}`), `"invocation":"model"`, `"invocation":"human-explicit"`, 1)
	explicitTarget = strings.Replace(explicitTarget, `"requires_explicit_invocation":false`, `"requires_explicit_invocation":true`, 1)
	authorization := fmt.Sprintf(`{"schema_version":"oaw.user-authorization/v1","id":"authorization-0123456789abcdef0123456789abcdef","issuer_host_id":"codex","host_session_digest":"%s","evidence_handle_digest":"%s","authorization_nonce":"nonce-1","workflow_id":"workflow-0123456789abcdef0123456789abcdef","bundle_id":"bundle-0123456789abcdef0123456789abcdef","bundle_generation":1,"bundle_digest":"%s","cursor":%s,"target":%s,"decision":"allowed","effects":["network-write"],"resources":["project-worktree"],"evidence":[{"kind":"user-approval","reference":"evidence://host/approval","digest":"%s"}],"digest":"%s"}`, digest, digest, digest, cursor, target, digest, digest)
	attestation := fmt.Sprintf(`{"schema_version":"oaw.explicit-invocation-attestation/v1","id":"invocation-attestation-0123456789abcdef0123456789abcdef","issuer_host_id":"codex","host_session_digest":"%s","evidence_handle_digest":"%s","invocation_nonce":"nonce-1","workflow_id":"workflow-0123456789abcdef0123456789abcdef","bundle_id":"bundle-0123456789abcdef0123456789abcdef","bundle_generation":1,"bundle_digest":"%s","cursor":%s,"provider_binding":%s,"evidence":[{"kind":"explicit-invocation","reference":"evidence://host/invocation","digest":"%s"}],"digest":"%s"}`, digest, digest, digest, cursor, explicitTarget, digest, digest)
	grant := fmt.Sprintf(`{"schema_version":"oaw.capability-grant/v3","id":"grant-0123456789abcdef0123456789abcdef","workflow_id":"workflow-0123456789abcdef0123456789abcdef","request_id":"request-1","bundle_id":"bundle-0123456789abcdef0123456789abcdef","bundle_generation":1,"bundle_digest":"%s","cursor":%s,"target":%s,"topology":"CURRENT","host_session_digest":"%s","effects":["network-write"],"resources":["project-worktree"],"termination_condition":"complete","authorization_digest":"%s","invocation_attestation_digest":"","digest":"%s"}`, digest, cursor, target, digest, digest, digest)
	receipt := fmt.Sprintf(`{"schema_version":"oaw.host-invocation-receipt/v3","kind":"COMPLETED","workflow_id":"workflow-0123456789abcdef0123456789abcdef","bundle_id":"bundle-0123456789abcdef0123456789abcdef","bundle_generation":1,"bundle_digest":"%s","cursor":%s,"topology":"CURRENT","host_session_digest":"%s","dispatch_digest":"%s","invocation_handle":"","context_freshness":"shared","environment_report_digest":"%s","outcome":"succeeded","failure_code":"","outputs":[{"artifact_id":"workflow-output","schema":"oaw.workflow-output/v1","reference":"artifact://workflow/output/1","digest":"%s"}],"evidence":[{"kind":"report","reference":"evidence://report","digest":"%s"}],"digest":"%s"}`, digest, cursor, digest, digest, digest, digest, digest, digest)
	for schemaID, raw := range map[string][]byte{
		UserAuthorizationV1: []byte(authorization), ExplicitInvocationAttestationV1: []byte(attestation), CapabilityGrantV3: []byte(grant), HostInvocationReceiptV3: []byte(receipt),
	} {
		if err := registry.Validate(schemaID, raw); err != nil {
			t.Fatalf("Validate(%s) error = %v", schemaID, err)
		}
	}
	for _, retired := range []string{
		"https://open-agent-workflow.dev/schemas/v2/capability-grant.schema.json",
		"https://open-agent-workflow.dev/schemas/v2/host-invocation-receipt.schema.json",
		"https://open-agent-workflow.dev/schemas/v3/host-conformance-transcript.schema.json",
		"https://open-agent-workflow.dev/schemas/v3/host-conformance-report.schema.json",
	} {
		if err := registry.Validate(retired, []byte(`{}`)); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
			t.Fatalf("retired authority schema %s error = %v", retired, err)
		}
	}
}

func TestRegistryUsesProviderV4AndRecipeV3Only(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	provider := validProviderV4JSON(t)
	if err := registry.Validate(ProviderDescriptorV4, provider); err != nil {
		t.Fatalf("Validate(provider v4) error = %v", err)
	}
	recipe := validRecipeV3JSON(t)
	if err := registry.Validate(ProfileRecipeV3, recipe); err != nil {
		t.Fatalf("Validate(recipe v3) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v3/provider-descriptor.schema.json", provider); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Provider v3) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/profile-recipe.schema.json", recipe); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Recipe v2) error = %v", err)
	}
}

func TestRegistryValidatesHostScopedProviderDescriptorV4(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	raw := validProviderV4JSON(t)
	if err := registry.Validate(ProviderDescriptorV4, raw); err != nil {
		t.Fatalf("Validate(v4 descriptor) error = %v", err)
	}
}

func TestRegistryProviderV4AcceptsExplicitNetworkWriteEffect(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	provider := catalog.ProviderDescriptorRecord{}
	if err := json.Unmarshal(validProviderV4JSON(t), &provider); err != nil {
		t.Fatal(err)
	}
	provider.Bindings[0].MaximumEffects = append(provider.Bindings[0].MaximumEffects, "network-write")
	raw, err := json.Marshal(provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Validate(ProviderDescriptorV4, raw); err != nil {
		t.Fatalf("Validate(network-write provider) error = %v", err)
	}
}

func TestRegistryRejectsRetiredProviderAndRecipeSchemas(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	provider := validProviderV4JSON(t)
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v3/provider-descriptor.schema.json", provider); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("retired provider schema error = %v", err)
	}
	recipe := validRecipeV3JSON(t)
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/profile-recipe.schema.json", recipe); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("retired recipe schema error = %v", err)
	}
	providerV3 := []byte(`{"schema_version":"oaw.provider-descriptor/v3"}`)
	if err := registry.Validate(ProviderDescriptorV4, providerV3); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("provider v3 wire error = %v", err)
	}
	recipeV2 := []byte(`{"schema_version":"oaw.profile-recipe/v2"}`)
	if err := registry.Validate(ProfileRecipeV3, recipeV2); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("recipe v2 wire error = %v", err)
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

func TestRegistryValidatesHostSessionV3AndEnvironmentV2Bridge(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	session := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-session/v3","host_id":"codex","integration_id":"acme/codex-native","integration_version":"3.0.0","session_id":"session-current-1","manifest_digest":"%s","supported_topologies":["CURRENT","SUBAGENT"],"provider_inventory_digest":"%s","feature_observations":[],"feature_digest":"%s","host_action_observations":[],"host_action_digest":"%s","environment_report_digest":"%s","sandbox_policy_digest":"","approval_policy_digest":"%s","digest":"%s"}`, digest, digest, digest, digest, digest, digest, digest))
	if err := registry.Validate(HostSessionV3, session); err != nil {
		t.Fatalf("Validate(HostSessionV3) error = %v", err)
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

func TestRegistryUsesHostBindingInventoryV3Only(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	inventory := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-binding-inventory/v3","host_id":"codex","observations":[{"host_id":"codex","provider_id":"oaw/provider","installation_key":"installation-acme","distribution_id":"distribution","binding_id":"binding-skill","surface":"codex","kind":"skill","reference":"provider:review","invocation":"model","binding_tree_digest":"sha256:%s","topologies":["CURRENT"],"source":"native-api","evidence_reference":"evidence://binding/acme-review","digest":"%s"}],"digest":"%s"}`, digest, digest, digest))
	if err := registry.Validate(HostBindingInventoryV3, inventory); err != nil {
		t.Fatalf("Validate(HostBindingInventoryV3) error = %v", err)
	}
	invalid := []byte(strings.Replace(string(inventory), `"topologies":["CURRENT"]`, `"topologies":[]`, 1))
	if err := registry.Validate(HostBindingInventoryV3, invalid); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(empty observed topologies) error = %v", err)
	}
	if err := registry.Validate("https://open-agent-workflow.dev/schemas/v2/host-binding-inventory.schema.json", inventory); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
		t.Fatalf("Validate(retired Host Binding Inventory v2) error = %v", err)
	}
}

func TestRegistryValidatesConformanceV4WithReceiptV3(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	receipt := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-invocation-receipt/v3","kind":"COMPLETED","workflow_id":"workflow-0123456789abcdef0123456789abcdef","bundle_id":"bundle-0123456789abcdef0123456789abcdef","bundle_generation":1,"bundle_digest":"%s","cursor":{"slot_id":"implementation","kind":"binding","unit_id":"implementation-main","ordinal":1},"topology":"CURRENT","host_session_digest":"%s","dispatch_digest":"%s","invocation_handle":"","context_freshness":"shared","environment_report_digest":"%s","outcome":"succeeded","failure_code":"","outputs":[{"artifact_id":"workflow-output","schema":"oaw.workflow-output/v1","reference":"artifact://output/1","digest":"%s"}],"evidence":[{"kind":"report","reference":"evidence://report","digest":"%s"}],"digest":"%s"}`, digest, digest, digest, digest, digest, digest, digest))
	if err := registry.Validate(HostInvocationReceiptV3, receipt); err != nil {
		t.Fatalf("Validate(HostInvocationReceiptV3) error = %v", err)
	}
	transcript := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-conformance-transcript/v4","session":{"schema_version":"oaw.host-session/v3","host_id":"codex","integration_id":"acme/codex-host","integration_version":"3.0.0","session_id":"session-current","manifest_digest":"%s","supported_topologies":["CURRENT"],"provider_inventory_digest":"%s","feature_observations":[],"feature_digest":"%s","host_action_observations":[],"host_action_digest":"%s","environment_report_digest":"%s","sandbox_policy_digest":"","approval_policy_digest":"","digest":"%s"},"inventory":{"schema_version":"oaw.host-binding-inventory/v3","host_id":"codex","observations":[],"digest":"%s"},"environment_reports":[{"schema_version":"oaw.host-environment-report/v2","session_id":"session-current","parent_session_id":"","topology":"CURRENT","observations":[],"digest":"%s"}],"receipts":[],"invocations":[],"digest":"%s"}`, digest, digest, digest, digest, digest, digest, digest, digest, digest))
	if err := registry.Validate(HostConformanceTranscriptV4, transcript); err != nil {
		t.Fatalf("Validate(HostConformanceTranscriptV4) error = %v", err)
	}
	report := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-conformance-report/v4","manifest_digest":"%s","host_session_digest":"%s","binding_inventory_digest":"%s","transcript_digest":"%s","verified_features":["normalized-receipts"],"verified_delegation_features":[],"verified_host_action_ids":[],"diagnostics":[],"digest":"%s"}`, digest, digest, digest, digest, digest))
	if err := registry.Validate(HostConformanceReportV4, report); err != nil {
		t.Fatalf("Validate(HostConformanceReportV4) error = %v", err)
	}
	legacyReport := []byte(fmt.Sprintf(`{"schema_version":"oaw.host-conformance-report/v2","manifest_digest":"%s","transcript_digest":"%s","verified_features":[],"diagnostics":[],"digest":"%s"}`, digest, digest, digest))
	if err := registry.Validate(HostConformanceReportV4, legacyReport); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
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
	start := []byte(fmt.Sprintf(`{"schema_version":"oaw.workflow-command/v1","kind":"START","message_id":"message-1","idempotency_key":"start-1","workflow_id":"","expected_revision":0,"start":{"request_id":"request-1","deliverable_id":"deliverable-1","input_digest":"%s","active_ticket":"","proposal":{"schema_version":"oaw.classification-proposal/v1","traits":[],"resources":[],"evidence":[]},"selection":{"profile":"SP-FULL","profile_source":"user-selection","topology":"CURRENT","topology_source":"host-only-option","add_ons":[],"bindings":[]},"host_session":{"schema_version":"oaw.host-session/v3","host_id":"codex","integration_id":"acme/codex","integration_version":"3.0.0","session_id":"session-1","manifest_digest":"%s","supported_topologies":["CURRENT"],"provider_inventory_digest":"%s","feature_observations":[],"feature_digest":"%s","host_action_observations":[],"host_action_digest":"%s","environment_report_digest":"%s","sandbox_policy_digest":"","approval_policy_digest":"","digest":"%s"},"environment":{"schema_version":"oaw.host-environment-report/v2","session_id":"session-1","parent_session_id":"","topology":"CURRENT","observations":[],"digest":"%s"}}}`, digest, digest, digest, digest, digest, digest, digest, digest))
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
	raw := append(validProviderV4JSON(t), []byte(` {}`)...)
	if err := registry.Validate(ProviderDescriptorV4, raw); err == nil || !strings.Contains(err.Error(), "SCHEMA_INPUT_INVALID") {
		t.Fatalf("Validate(trailing) error = %v", err)
	}
}

func TestRegistryRejectsSchemaViolations(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	unknownField := append(validProviderV4JSON(t)[:len(validProviderV4JSON(t))-1], []byte(`,"extra":true}`)...)
	if err := registry.Validate(ProviderDescriptorV4, unknownField); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(unknown field) error = %v", err)
	}
	unsafePath := append(validProviderV4JSON(t)[:len(validProviderV4JSON(t))-1], []byte(`,"discovery":[{"id":"probe","hosts":["codex"],"surface":"codex-skills","distribution_id":"distribution","kind":"path-exists","root":"user-home","candidate_path":".agents/../secret","evidence_path":"skill/SKILL.md"}]}`)...)
	if err := registry.Validate(ProviderDescriptorV4, unsafePath); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(unsafe path) error = %v", err)
	}
}

func validProviderV4JSON(t *testing.T) []byte {
	t.Helper()
	record := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "test/provider", DisplayName: "Test Provider",
		Distributions: []catalog.DistributionRecord{{ID: "distribution", SourceURI: "https://example.test/provider", Revision: strings.Repeat("a", 40), TreeDigest: "sha256:" + strings.Repeat("a", 64)}},
		Discovery:     []catalog.DiscoveryProbe{{ID: "probe", Hosts: []string{"codex"}, Surface: "codex-skills", DistributionID: "distribution", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/skills", EvidencePath: "skill/SKILL.md"}},
		Bindings: []catalog.BindingRecord{{
			ID: "binding", DistributionID: "distribution", ContentRoot: "skills/skill", InstallRoot: "skill", TreeDigest: "sha256:" + strings.Repeat("a", 64), Host: "codex", Surface: "codex-skills", Kind: catalog.BindingSkill, Reference: "skill", Invocation: catalog.InvocationModel,
			Responsibilities: []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: "implementation", SlotID: catalog.SlotImplementation, OutcomeOwner: true}}, InputArtifact: "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, StageSpan: []catalog.SlotID{catalog.SlotImplementation}, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{ID: "workflow", InputSchema: "artifact", OutcomeSchema: "artifact", RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"binding"}}},
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func validRecipeV3JSON(t *testing.T) []byte {
	t.Helper()
	slots := make([]catalog.SlotRecipe, len(catalog.CanonicalSlots()))
	for index, definition := range catalog.CanonicalSlots() {
		slot := catalog.SlotRecipe{SlotID: definition.ID, Applicability: catalog.SlotMandatory, Pipeline: []catalog.PipelineStep{}, Gates: []catalog.GateRecord{}, Transitions: []catalog.RecipeTransition{}}
		if definition.ID == catalog.SlotIncidentRecovery {
			slot.Applicability = catalog.SlotConditional
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerNone}
		} else {
			step := catalog.PipelineStep{ID: "main", Selector: catalog.BindingSelector{ProviderID: "test/provider", BindingID: "binding"}, StageSpan: []catalog.SlotID{definition.ID}, RequiredInputArtifact: "artifact", ProducedOutputArtifact: "artifact"}
			slot.Pipeline = []catalog.PipelineStep{step}
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: step.ID}
		}
		if index+1 < len(slots) {
			slot.Transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: catalog.CanonicalSlots()[index+1].ID}}
		}
		slots[index] = slot
	}
	record := catalog.ProfileRecipeRecord{SchemaVersion: catalog.ProfileRecipeSchemaV3, TaxonomyVersion: catalog.TaxonomyVersionV1, RecipeVersion: "3.0.0", ID: "test/recipe", DisplayName: "Test Recipe", Family: "test", Slots: slots, AddOns: []catalog.AddOnRecord{}, IncidentRoutes: []catalog.IncidentRoute{}, Overlays: []catalog.OverlayRecord{}, StableBoundaries: []string{"between-slots"}, EnvironmentRequirements: []execution.EnvironmentRequirement{}}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return raw
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
