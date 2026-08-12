package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
)

func TestRegistryCompilesCoordinatorV2Schemas(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	for _, schemaID := range []string{
		DispatchPacketV2,
		WorkflowCommandV2,
		WorkflowResultV2,
		WorkflowSnapshotV2,
		WorkflowRevisionV2,
		GateAttestationV1,
	} {
		if _, found := registry.schemas[schemaID]; !found {
			t.Fatalf("active schema %q was not compiled", schemaID)
		}
	}
}

func TestRegistryRejectsCoordinatorV1Authority(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	for _, schemaID := range []string{
		"https://open-agent-workflow.dev/schemas/v1/dispatch-packet.schema.json",
		"https://open-agent-workflow.dev/schemas/v1/workflow-command.schema.json",
		"https://open-agent-workflow.dev/schemas/v1/workflow-result.schema.json",
		"https://open-agent-workflow.dev/schemas/v1/workflow-snapshot.schema.json",
		"https://open-agent-workflow.dev/schemas/v1/workflow-revision.schema.json",
	} {
		if err := registry.Validate(schemaID, []byte(`{}`)); err == nil || !strings.Contains(err.Error(), "UNKNOWN_SCHEMA") {
			t.Fatalf("retired schema %q error = %v", schemaID, err)
		}
	}
}

func TestRegistryWorkflowCommandV2AcceptsStartAlternativeAndGateOnlyPrepare(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Run("START with AlternativeChoice", func(t *testing.T) {
		if err := registry.Validate(WorkflowCommandV2, validWorkflowStartV2(t)); err != nil {
			t.Fatalf("Validate(START v2 with AlternativeChoice) error = %v", err)
		}
	})
	t.Run("gate-only PREPARE", func(t *testing.T) {
		if err := registry.Validate(WorkflowCommandV2, validGateOnlyPrepareV2(t, false)); err != nil {
			t.Fatalf("Validate(gate-only PREPARE v2) error = %v", err)
		}
	})
}

func TestRegistryWorkflowCommandV2RejectsMixedPayloadAndGateAuthorityMix(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	var mixed map[string]any
	if err := json.Unmarshal(validWorkflowStartV2(t), &mixed); err != nil {
		t.Fatal(err)
	}
	mixed["prepare"] = map[string]any{
		"requested_effects": []any{"read-project"}, "requested_resources": []any{"project-worktree"},
		"termination_condition": "complete", "input_references": []any{}, "evidence_requirements": []any{},
	}
	if err := registry.Validate(WorkflowCommandV2, mustJSON(t, mixed)); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(START with PREPARE payload) error = %v", err)
	}
	if err := registry.Validate(WorkflowCommandV2, validGateOnlyPrepareV2(t, true)); err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED") {
		t.Fatalf("Validate(execution PREPARE with Gate Attestation) error = %v", err)
	}
}

func TestRegistryWorkflowSnapshotV2AcceptsAlternativeChoice(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Validate(WorkflowSnapshotV2, validWorkflowSnapshotV2(t)); err != nil {
		t.Fatalf("Validate(Snapshot v2 with AlternativeChoice) error = %v", err)
	}
}

func TestRegistryWorkflowSnapshotV2EnforcesActiveDispatchState(t *testing.T) {
	registry, err := New(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	grant := validSnapshotGrantV3(digest)
	for _, test := range []struct {
		name           string
		status         string
		activeGrant    any
		dispatchDigest string
		wantValid      bool
	}{
		{name: "prepared pair", status: "PREPARED", activeGrant: grant, dispatchDigest: digest, wantValid: true},
		{name: "prepared missing pair", status: "PREPARED"},
		{name: "in flight missing dispatch", status: "IN_FLIGHT", activeGrant: grant},
		{name: "ready active pair", status: "READY", activeGrant: grant, dispatchDigest: digest},
		{name: "paused pair", status: "PAUSED", activeGrant: grant, dispatchDigest: digest, wantValid: true},
		{name: "paused inactive", status: "PAUSED", wantValid: true},
		{name: "paused digest only", status: "PAUSED", dispatchDigest: digest},
		{name: "paused grant only", status: "PAUSED", activeGrant: grant},
	} {
		t.Run(test.name, func(t *testing.T) {
			var snapshot map[string]any
			if err := json.Unmarshal(validWorkflowSnapshotV2(t), &snapshot); err != nil {
				t.Fatal(err)
			}
			snapshot["status"] = test.status
			if test.activeGrant != nil {
				snapshot["active_grant"] = test.activeGrant
			}
			if test.dispatchDigest != "" {
				snapshot["active_dispatch_digest"] = test.dispatchDigest
			}
			err := registry.Validate(WorkflowSnapshotV2, mustJSON(t, snapshot))
			if test.wantValid && err != nil {
				t.Fatalf("Validate(active Snapshot) error = %v", err)
			}
			if !test.wantValid && (err == nil || !strings.Contains(err.Error(), "SCHEMA_VALIDATION_FAILED")) {
				t.Fatalf("Validate(invalid active Snapshot) error = %v", err)
			}
		})
	}
}

func validSnapshotGrantV3(digest string) map[string]any {
	return map[string]any{
		"schema_version": "oaw.capability-grant/v3", "id": "grant-0123456789abcdef0123456789abcdef",
		"workflow_id": "workflow-0123456789abcdef0123456789abcdef", "request_id": "request-1",
		"bundle_id": "bundle-0123456789abcdef0123456789abcdef", "bundle_generation": 1, "bundle_digest": digest,
		"cursor": map[string]any{"slot_id": "problem-framing", "kind": "binding", "unit_id": "unit-1", "ordinal": 1},
		"target": map[string]any{
			"target_kind": "host-action",
			"host_action": map[string]any{
				"id": "verification.execute", "input_artifact": "workflow-input", "output_artifact": "workflow-output",
				"input_schema": "oaw.workflow-input/v1", "outcome_schema": "oaw.workflow-output/v1",
				"maximum_effects": []any{"read-project"}, "resources": []any{"project-worktree"}, "observation_digest": digest,
			},
		},
		"topology": "CURRENT", "host_session_digest": digest, "effects": []any{"read-project"},
		"resources": []any{"project-worktree"}, "termination_condition": "complete",
		"authorization_digest": "", "invocation_attestation_digest": "", "digest": digest,
	}
}

func validWorkflowStartV2(t testing.TB) []byte {
	t.Helper()
	digest := strings.Repeat("a", 64)
	selection := workflowSelectionV2(digest)
	return mustJSON(t, map[string]any{
		"schema_version": "oaw.workflow-command/v2", "kind": "START", "message_id": "message-1", "idempotency_key": "start-1", "workflow_id": "", "expected_revision": 0,
		"start": map[string]any{
			"request_id": "request-1", "deliverable_id": "deliverable-1", "input_digest": digest, "active_ticket": "",
			"proposal":  map[string]any{"schema_version": "oaw.classification-proposal/v1", "traits": []any{}, "resources": []any{}, "evidence": []any{}},
			"selection": selection,
			"host_session": map[string]any{
				"schema_version": "oaw.host-session/v3", "host_id": "codex", "integration_id": "acme/codex", "integration_version": "3.0.0", "session_id": "session-1",
				"manifest_digest": digest, "supported_topologies": []any{"CURRENT"}, "provider_inventory_digest": digest,
				"feature_observations": []any{}, "feature_digest": digest, "host_action_observations": []any{}, "host_action_digest": digest,
				"environment_report_digest": digest, "sandbox_policy_digest": "", "approval_policy_digest": "", "digest": digest,
			},
			"environment": map[string]any{"schema_version": "oaw.host-environment-report/v2", "session_id": "session-1", "parent_session_id": "", "topology": "CURRENT", "observations": []any{}, "digest": digest},
		},
	})
}

func validGateOnlyPrepareV2(t testing.TB, mixExecutionAuthority bool) []byte {
	t.Helper()
	digest := strings.Repeat("a", 64)
	effects, resources, termination := []any{}, []any{}, ""
	if mixExecutionAuthority {
		effects, resources, termination = []any{"read-project"}, []any{"project-worktree"}, "complete"
	}
	return mustJSON(t, map[string]any{
		"schema_version": "oaw.workflow-command/v2", "kind": "PREPARE", "message_id": "message-2", "idempotency_key": "prepare-2",
		"workflow_id": "workflow-0123456789abcdef0123456789abcdef", "expected_revision": 1,
		"prepare": map[string]any{
			"requested_effects": effects, "requested_resources": resources, "termination_condition": termination,
			"input_references": []any{}, "evidence_requirements": []any{},
			"gate_attestation": map[string]any{
				"schema_version": "oaw.gate-attestation/v1", "workflow_id": "workflow-0123456789abcdef0123456789abcdef",
				"bundle_id": "bundle-0123456789abcdef0123456789abcdef", "bundle_generation": 1, "bundle_digest": digest,
				"cursor":  map[string]any{"slot_id": "closeout", "kind": "gate", "unit_id": "approval", "ordinal": 1},
				"gate_id": "approval", "authority": "user", "decision": "satisfied",
				"evidence": []any{map[string]any{"kind": "approval", "reference": "evidence://approval/1", "digest": digest}}, "digest": digest,
			},
		},
	})
}

func validWorkflowSnapshotV2(t testing.TB) []byte {
	t.Helper()
	digest := strings.Repeat("a", 64)
	slotIDs := []string{"problem-framing", "solution-specification", "delivery-planning", "workspace-preparation", "implementation", "implementation-tdd", "incident-recovery", "review-remediation", "fresh-verification", "closeout"}
	recipeSlots := make([]any, 0, len(slotIDs))
	graphSlots := make([]any, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		recipeSlots = append(recipeSlots, map[string]any{
			"slot_id": slotID, "applicability": "mandatory", "outcome_owner": map[string]any{"kind": "none"},
			"pipeline": []any{}, "gates": []any{}, "transitions": []any{},
		})
		graphSlots = append(graphSlots, map[string]any{
			"slot_id": slotID, "applicability": "mandatory", "active": false, "entry_artifact": "", "outcome_artifact": "",
			"outcome_owner": map[string]any{"kind": "none", "unit_id": ""}, "pipeline": []any{}, "gates": []any{},
			"transitions": []any{}, "terminal": false, "traversal": []any{},
		})
	}
	workflowSelection := workflowSelectionV2(digest)
	graphSelection := map[string]any{
		"profile": "SP-FULL", "recipe_id": "test/delivery", "recipe_digest": digest, "topology": "CURRENT",
		"add_ons": []any{}, "alternatives": alternativeChoices(), "overlays": []any{}, "digest": digest,
	}
	recipe := map[string]any{
		"schema_version": "oaw.profile-recipe/v3", "taxonomy_version": "oaw.lifecycle-taxonomy/v1", "recipe_version": "3.0.0",
		"id": "test/delivery", "display_name": "Test delivery", "family": "test", "slots": recipeSlots,
		"add_ons": []any{}, "incident_routes": []any{}, "overlays": []any{}, "stable_boundaries": []any{}, "environment_requirements": []any{},
	}
	graph := map[string]any{
		"schema_version": "oaw.execution-graph/v4", "host_id": "codex", "host_evidence_digest": digest, "registry_digest": digest,
		"taxonomy_version": "oaw.lifecycle-taxonomy/v1", "recipe_id": "test/delivery", "recipe_version": "3.0.0", "recipe_digest": digest,
		"selection": graphSelection, "provider_instances": []any{}, "entry_slot_id": "problem-framing", "slots": graphSlots,
		"incident_routes": []any{}, "stable_boundaries": []any{}, "topology": "CURRENT", "environment_requirements": []any{}, "decisions": []any{}, "digest": digest,
	}
	classificationRecord := map[string]any{
		"request_mode": "WORKFLOW", "workflow_complexity": "complex", "risk_class": "normal",
		"evidence_requirements": []any{}, "escalation_reasons": []any{},
	}
	configurationRecord := map[string]any{
		"schema_version": "oaw.configuration-snapshot/v2", "catalog_digest": digest, "user_config_digest": digest,
		"project_root": "", "project_config_digest": "", "project_status": "absent", "project_reason": "",
		"settings": []any{}, "provider_installations": []any{}, "bounded_capability_defaults": []any{},
		"required_providers": []any{}, "recommended_providers": []any{}, "untrusted_provider_ids": []any{},
		"host_integrations": []any{}, "digest": digest,
	}
	bundle := map[string]any{
		"schema_version": "oaw.lifecycle-bundle/v4", "id": "bundle-0123456789abcdef0123456789abcdef", "deliverable_id": "deliverable-1",
		"input_digest": digest, "generation": 1, "classification": classificationRecord, "classification_digest": digest,
		"selection": workflowSelection, "recipe": recipe, "recipe_digest": digest, "host_id": "codex", "host_session_digest": digest,
		"reporter_identity_digest": digest,
		"host_manifest_digest":     digest, "environment_report_digest": digest, "provider_inventory_digest": digest, "host_feature_digest": digest,
		"host_action_digest": digest, "host_evidence_digest": digest, "configuration": configurationRecord, "resolution_digest": digest,
		"registry_digest": digest, "provider_instances": []any{}, "execution_graph": graph, "topology": "CURRENT",
		"environment_requirements": []any{}, "add_ons": []any{}, "digest": digest,
	}
	return mustJSON(t, map[string]any{
		"schema_version": "oaw.workflow-snapshot/v2", "workflow_id": "workflow-0123456789abcdef0123456789abcdef",
		"request_id": "request-1", "deliverable_id": "deliverable-1", "revision": 1, "status": "READY", "classification": classificationRecord,
		"bundles": []any{bundle}, "active_generation": 1,
		"cursor":        map[string]any{"slot_id": "problem-framing", "kind": "binding", "unit_id": "unit-1", "ordinal": 1},
		"active_ticket": "", "grant_history": []any{}, "user_authorizations": []any{}, "invocation_attestations": []any{},
		"gate_attestations": []any{}, "receipts": []any{}, "resource_leases": []any{}, "last_stable_boundary": "",
		"processed_messages": []any{map[string]any{"idempotency_key": "start-1", "content_digest": digest, "revision": 1, "result_digest": digest}},
		"projection_lag":     []any{},
	})
}

func workflowSelectionV2(digest string) map[string]any {
	return map[string]any{
		"profile": "SP-FULL", "recipe_id": "test/delivery", "recipe_digest": digest, "profile_source": "user-selection",
		"topology": "CURRENT", "topology_source": "host-only-option", "add_ons": []any{}, "alternatives": alternativeChoices(),
		"overlays": []any{}, "graph_selection_digest": digest,
	}
}

func alternativeChoices() []any {
	return []any{map[string]any{
		"slot_id": "delivery-planning", "step_id": "plan", "alternative_id": "writing-plans",
		"selector": map[string]any{"provider_id": "oaw/superpowers", "binding_id": "codex-writing-plans"},
	}}
}

func mustJSON(t testing.TB, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
