package coordinator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestRecordV2SchemaVersionsAreTheOnlyActiveCoordinatorAuthority(t *testing.T) {
	want := []string{WorkflowCommandSchemaV2, WorkflowResultSchemaV2, WorkflowSnapshotSchemaV2, WorkflowRevisionSchemaV2, WorkflowHeadSchemaV1, DispatchPacketSchemaV2, GateAttestationSchemaV1}
	for _, value := range want {
		if value == "" || strings.Contains(value, "/v0") {
			t.Fatalf("invalid active schema %q", value)
		}
	}
	if WorkflowCommandSchemaV2 == "oaw.workflow-command/v1" || WorkflowResultSchemaV2 == "oaw.workflow-result/v1" || WorkflowSnapshotSchemaV2 == "oaw.workflow-snapshot/v1" || WorkflowRevisionSchemaV2 == "oaw.workflow-revision/v1" || DispatchPacketSchemaV2 == "oaw.dispatch-packet/v1" {
		t.Fatal("Coordinator v1 schema became active")
	}
}

func TestGateAttestationIsClosedAndNeverCarriesExecutionAuthority(t *testing.T) {
	cursor, err := execution.NewGraphCursor("solution-specification", execution.CursorGate, "gate-approval", 1)
	if err != nil {
		t.Fatal(err)
	}
	value, err := normalizeGateAttestation(GateAttestation{
		SchemaVersion: GateAttestationSchemaV1, WorkflowID: "workflow-0123456789abcdef0123456789abcdef", BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleGeneration: 1, BundleDigest: strings.Repeat("a", 64), Cursor: cursor, GateID: "gate-approval", Authority: catalog.GateUser, Decision: GateSatisfied,
		Evidence: []host.EvidenceReference{{Kind: "approval", Reference: "evidence://approval/1", Digest: strings.Repeat("b", 64)}},
	})
	if err != nil || value.Digest == "" {
		t.Fatalf("normalizeGateAttestation() = %#v, %v", value, err)
	}
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"grant", "provider_binding", "host_action", "authorization", "invocation_attestation"} {
		if _, found := fields[forbidden]; found {
			t.Fatalf("Gate Attestation contains execution authority field %q", forbidden)
		}
	}
}

func TestDispatchV2RejectsOldPacketBeforeAuthorityDecode(t *testing.T) {
	value := map[string]any{"schema_version": "oaw.dispatch-packet/v1", "grant": map[string]any{"schema_version": "oaw.capability-grant/v2", "secret": "must-not-be-read"}}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateSchemaVersion(raw); err != "oaw.dispatch-packet/v1" {
		t.Fatalf("schema discriminator = %q", err)
	}
	if _, err := DecodeCommand([]byte(`{"schema_version":"oaw.workflow-command/v1","kind":"INSPECT","workflow_id":"workflow-0123456789abcdef0123456789abcdef"}`)); ErrorCode(err) != "SCHEMA_UNSUPPORTED" {
		t.Fatalf("old command error = %v", err)
	}
}

func TestCursorValidationRejectsTerminalAndGateExecutionGrantKinds(t *testing.T) {
	gate, err := execution.NewGraphCursor("closeout", execution.CursorGate, "gate", 1)
	if err != nil {
		t.Fatal(err)
	}
	if gate.Kind != execution.CursorGate {
		t.Fatal("gate cursor kind changed")
	}
	if _, err := execution.NewGraphCursor("closeout", execution.CursorTerminal, "terminal", 1); err != nil {
		t.Fatal(err)
	}
}

func TestOldStateRevisionIsRejectedWithoutWriting(t *testing.T) {
	root := t.TempDir()
	engine, err := NewEngine(Options{StateRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	workflowID := "workflow-0123456789abcdef0123456789abcdef"
	revisionDir := filepath.Join(root, workflowRecordsDirectory, workflowID, "revisions")
	if err := os.MkdirAll(revisionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(revisionDir, revisionFileName(1))
	old := []byte(`{"schema_version":"oaw.workflow-revision/v1","workflow_id":"workflow-0123456789abcdef0123456789abcdef","revision":1}`)
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.journal.loadRevision(workflowID, 1); ErrorCode(err) != "WORKFLOW_STATE_UNSUPPORTED" {
		t.Fatalf("old revision error = %v", err)
	}
	after, err := os.Stat(path)
	if err != nil || before.Size() != after.Size() || before.Mode() != after.Mode() {
		t.Fatalf("old state metadata changed: before=%v after=%v err=%v", before, after, err)
	}
}

func TestStartPinsCommandHostFactsToTrustedHostEvidence(t *testing.T) {
	engine := &Engine{}
	_, err := engine.start(Command{Start: &StartInput{
		HostSession: host.SessionSnapshot{Digest: strings.Repeat("a", 64)},
		Environment: host.EnvironmentReport{Digest: strings.Repeat("b", 64)},
	}})
	if ErrorCode(err) != "WORKFLOW_HOST_EVIDENCE_MISMATCH" {
		t.Fatalf("start() error = %v", err)
	}
}

func TestActiveBundleUsesGenerationIdentityInsteadOfSlicePosition(t *testing.T) {
	want := core.LifecycleBundle{ID: "bundle-0123456789abcdef0123456789abcdef", Generation: 7}
	bundle, err := activeBundle(Snapshot{
		Bundles: []core.LifecycleBundle{{Generation: 2}, want}, ActiveGeneration: want.Generation,
	})
	if err != nil || bundle.ID != want.ID {
		t.Fatalf("activeBundle() = %#v, %v", bundle, err)
	}
	if _, err := activeBundle(Snapshot{
		Bundles:          []core.LifecycleBundle{want, {ID: "bundle-00000000000000000000000000000000", Generation: want.Generation}},
		ActiveGeneration: want.Generation,
	}); ErrorCode(err) != "WORKFLOW_PREPARE_INVALID" {
		t.Fatalf("activeBundle(duplicate generation) error = %v", err)
	}
}

func TestGateOnlyPrepareRejectsExecutionPayload(t *testing.T) {
	cursor, err := execution.NewGraphCursor("closeout", execution.CursorGate, "approval", 1)
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := normalizeGateAttestation(GateAttestation{
		SchemaVersion:    GateAttestationSchemaV1,
		WorkflowID:       "workflow-0123456789abcdef0123456789abcdef",
		BundleID:         "bundle-0123456789abcdef0123456789abcdef",
		BundleGeneration: 1,
		BundleDigest:     strings.Repeat("a", 64),
		Cursor:           cursor,
		GateID:           "approval",
		Authority:        catalog.GateUser,
		Decision:         GateSatisfied,
		Evidence:         []host.EvidenceReference{{Kind: "approval", Reference: "evidence://approval/1", Digest: strings.Repeat("b", 64)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := PrepareInput{GateAttestation: &attestation, InputReferences: []ArtifactReference{{
		Kind: "artifact", Reference: "artifact://input/1", Digest: strings.Repeat("c", 64),
	}}}
	if ErrorCode(validatePrepareInput(input)) != "WORKFLOW_COMMAND_INVALID" {
		t.Fatal("gate-only PREPARE accepted execution input references")
	}
}

func TestGateAttestationRequiresDeclaredEvidence(t *testing.T) {
	requirements := []catalog.EvidenceRequirementRecord{{Kind: "user-decision", Minimum: 1, Description: "user approves"}}
	wrong := []host.EvidenceReference{{Kind: "report", Reference: "evidence://report/1", Digest: strings.Repeat("a", 64)}}
	if ErrorCode(validateGateEvidenceClosure(requirements, wrong)) != "GATE_EVIDENCE_INCOMPLETE" {
		t.Fatal("gate evidence closure accepted the wrong evidence kind")
	}
	matching := []host.EvidenceReference{{Kind: "user-decision", Reference: "evidence://approval/1", Digest: strings.Repeat("b", 64)}}
	if err := validateGateEvidenceClosure(requirements, matching); err != nil {
		t.Fatalf("validateGateEvidenceClosure() error = %v", err)
	}
}

func TestGateAttestationRejectsRewrittenEvidenceDigest(t *testing.T) {
	cursor, err := execution.NewGraphCursor("closeout", execution.CursorGate, "approval", 1)
	if err != nil {
		t.Fatal(err)
	}
	reference := host.EvidenceReference{Kind: "approval", Reference: "evidence://approval/1", Digest: strings.Repeat("a", 64)}
	rewritten := reference
	rewritten.Digest = strings.Repeat("b", 64)
	_, err = normalizeGateAttestation(GateAttestation{
		SchemaVersion: GateAttestationSchemaV1, WorkflowID: "workflow-0123456789abcdef0123456789abcdef",
		BundleID: "bundle-0123456789abcdef0123456789abcdef", BundleGeneration: 1, BundleDigest: strings.Repeat("c", 64),
		Cursor: cursor, GateID: "approval", Authority: catalog.GateUser, Decision: GateSatisfied,
		Evidence: []host.EvidenceReference{reference, rewritten},
	})
	if ErrorCode(err) != "GATE_ATTESTATION_INVALID" {
		t.Fatalf("normalizeGateAttestation(rewritten evidence digest) error = %v", err)
	}
}

func TestOutputEvidenceMustMatchDispatchTarget(t *testing.T) {
	packet := DispatchPacket{Grant: admission.CapabilityGrant{Target: admission.AuthorizationTarget{
		TargetKind: admission.GrantProviderBinding,
		ProviderBinding: &admission.ProviderBindingAuthority{
			OutputArtifact: "implementation-result", OutcomeSchema: "oaw.implementation-result/v1",
		},
	}}}
	matching := []host.OutputReference{{
		ArtifactID: "implementation-result", Schema: "oaw.implementation-result/v1",
		Reference: "artifact://implementation/result/1", Digest: strings.Repeat("a", 64),
	}}
	if err := validateOutputClosure(packet, matching); err != nil {
		t.Fatalf("validateOutputClosure() error = %v", err)
	}
	mismatched := append([]host.OutputReference{}, matching...)
	mismatched[0].Schema = "oaw.wrong/v1"
	if ErrorCode(validateOutputClosure(packet, mismatched)) != "WORKFLOW_OUTPUT_INVALID" {
		t.Fatal("output closure accepted the wrong outcome schema")
	}
}

func TestResourceLeasePinsGrantCursor(t *testing.T) {
	projectRoot := t.TempDir()
	engine, err := NewEngine(Options{StateRoot: t.TempDir(), PhysicalProjectRoot: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := execution.NewGraphCursor("implementation", execution.CursorBinding, "implementation-main", 1)
	if err != nil {
		t.Fatal(err)
	}
	grant := admission.CapabilityGrant{
		ID: "grant-0123456789abcdef0123456789abcdef", BundleID: "bundle-0123456789abcdef0123456789abcdef",
		BundleGeneration: 4, Cursor: cursor, Effects: []string{"write-project"},
	}
	leases, err := engine.prepareProjectLease(Snapshot{WorkflowID: "workflow-0123456789abcdef0123456789abcdef"}, grant, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(leases) != 1 || leases[0].Cursor != cursor {
		t.Fatalf("Resource Lease cursor = %#v, want %#v", leases, cursor)
	}
}
