package coordinator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	oawcore "github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestCoordinatorValidationHelpersCoverStableIdentityAndKeys(t *testing.T) {
	validID := "dispatch-" + strings.Repeat("a", 32)
	if !validStableID("dispatch-", validID) || validStableID("dispatch-", "dispatch-"+strings.Repeat("A", 32)) || validStableID("dispatch-", "dispatch-short") {
		t.Fatal("validStableID accepted an invalid identity or rejected a valid identity")
	}
	if !validHex("0123abcdef") || validHex("") || validHex("g") {
		t.Fatal("validHex validation is incorrect")
	}
	workflowID := deriveWorkflowID("validation")
	if !validWorkflowID(workflowID) || validWorkflowID("run-invalid") {
		t.Fatal("validWorkflowID validation is incorrect")
	}
	if validUniqueTextSet([]string{"a", "a"}, 10) || !validUniqueTextSet([]string{"a", "b"}, 10) {
		t.Fatal("validUniqueTextSet validation is incorrect")
	}

	for _, status := range []Status{StatusReady, StatusPrepared, StatusInFlight, StatusPaused, StatusFinished, StatusCancelled} {
		if !validSnapshotStatus(status) {
			t.Fatalf("validSnapshotStatus(%q) = false", status)
		}
	}
	if validSnapshotStatus(Status("UNKNOWN")) {
		t.Fatal("validSnapshotStatus accepted an unknown status")
	}
	for _, risk := range []classification.RiskClass{classification.RiskNormal, classification.RiskElevated, classification.RiskCritical} {
		if !validRiskClass(risk) {
			t.Fatalf("validRiskClass(%q) = false", risk)
		}
	}
	if validRiskClass(classification.RiskClass("UNKNOWN")) {
		t.Fatal("validRiskClass accepted an unknown risk")
	}

	if artifactReferenceKey(ArtifactReference{Kind: "input", Reference: "ref", Digest: strings.Repeat("a", 64)}) == "" ||
		evidenceRequirementKey(EvidenceRequirement{Kind: "report", Minimum: 1, Description: "report"}) == "" ||
		diagnosticKey(Diagnostic{Code: "CODE", Detail: "detail"}) == "" {
		t.Fatal("record key helpers returned an empty key")
	}
}

func TestCoordinatorErrorFormatsEmptyAndDetailedMessages(t *testing.T) {
	if (&Error{Code: "CODE"}).Error() != "CODE" || (&Error{Code: "CODE", Detail: "detail"}).Error() != "CODE: detail" {
		t.Fatal("Coordinator Error formatting is incorrect")
	}
}

func TestPrepareInputValidationFailsClosedOnMalformedAuthorityAndEvidence(t *testing.T) {
	valid := PrepareInput{
		RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"},
		TerminationCondition: "complete", InputReferences: []ArtifactReference{}, EvidenceRequirements: []EvidenceRequirement{},
	}
	if err := validatePrepareInput(valid); err != nil {
		t.Fatalf("valid PREPARE input error = %v", err)
	}
	tests := []PrepareInput{
		{},
		{
			RequestedEffects: []string{"read-project", "read-project"}, RequestedResources: []string{"project"},
			TerminationCondition: "complete", InputReferences: []ArtifactReference{}, EvidenceRequirements: []EvidenceRequirement{},
		},
		{
			RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"}, TerminationCondition: "complete",
			InputReferences:      []ArtifactReference{{Kind: "", Reference: "artifact://input", Digest: strings.Repeat("a", 64)}},
			EvidenceRequirements: []EvidenceRequirement{},
		},
		{
			RequestedEffects: []string{"read-project"}, RequestedResources: []string{"project"}, TerminationCondition: "complete",
			InputReferences: []ArtifactReference{}, EvidenceRequirements: []EvidenceRequirement{{Kind: "report", Description: "report"}},
		},
	}
	for index, input := range tests {
		if err := validatePrepareInput(input); ErrorCode(err) != "WORKFLOW_COMMAND_INVALID" {
			t.Fatalf("invalid PREPARE input %d error = %v", index, err)
		}
	}
}

func TestPrimitiveValidationRejectsBoundaryViolations(t *testing.T) {
	if !validDigest(strings.Repeat("a", 64)) || validDigest(strings.Repeat("g", 64)) || validDigest("short") {
		t.Fatal("validDigest validation is incorrect")
	}
	if !validText("value", 5) || validText(" value", 6) || validText("value\n", 6) || validText("value", 4) {
		t.Fatal("validText validation is incorrect")
	}
	if expectedPayloadCount(CommandKind("UNKNOWN")) != -1 {
		t.Fatal("unknown command kind unexpectedly accepts a payload")
	}
	if commandPayloadMatchesKind(Command{Kind: CommandKind("UNKNOWN")}) {
		t.Fatal("unknown command kind unexpectedly matched a payload")
	}
}

func TestPhysicalRootAndProjectionEvidenceValidation(t *testing.T) {
	if _, err := activeBundle(Snapshot{}); ErrorCode(err) != "WORKFLOW_PREPARE_INVALID" {
		t.Fatalf("missing active Bundle error = %v", err)
	}
	if _, err := newProjectionRecord(Result{Revision: 1, RevisionDigest: strings.Repeat("a", 64), Snapshot: &Snapshot{}}); ErrorCode(err) != "PROJECTION_INVALID" {
		t.Fatalf("projection without active Bundle error = %v", err)
	}
	if _, err := canonicalPhysicalRoot(""); ErrorCode(err) != "RESOURCE_LEASE_INVALID" {
		t.Fatalf("empty physical root error = %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := canonicalPhysicalRoot(missing); ErrorCode(err) != "RESOURCE_LEASE_INVALID" {
		t.Fatalf("missing physical root error = %v", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalPhysicalRoot(file); ErrorCode(err) != "RESOURCE_LEASE_INVALID" {
		t.Fatalf("file physical root error = %v", err)
	}
	root := t.TempDir()
	physical, err := canonicalPhysicalRoot(root)
	if err != nil || physical == "" {
		t.Fatalf("valid physical root = %q, %v", physical, err)
	}

	var absent *journal
	if err := absent.withResourceLeaseLock(func() error { return nil }); ErrorCode(err) != "RESOURCE_LEASE_INVALID" {
		t.Fatalf("nil Resource Lease journal error = %v", err)
	}
	reference := host.EvidenceReference{Kind: "report", Reference: "evidence://report", Digest: strings.Repeat("a", 64)}
	evidence := projectionEvidence([]host.InvocationReceipt{{Evidence: []host.EvidenceReference{reference, reference}}})
	if len(evidence) != 1 || evidence[0] != reference {
		t.Fatalf("projection evidence = %#v", evidence)
	}
	var engine *Engine
	engine.projectResult(Result{})
}

func TestDefaultCoreDelegatesClassificationAndCompilation(t *testing.T) {
	compiler := defaultCore{}
	decision, err := compiler.Classify(nil, classification.ClassificationRules{})
	if err != nil || decision.RequestMode != classification.RequestModeWorkflow {
		t.Fatalf("default Core Classify() = %#v, %v", decision, err)
	}
	if _, err := compiler.Compile(oawcore.CompilationRequest{}); err == nil {
		t.Fatal("default Core Compile() accepted an empty request")
	}
	if _, err := NewEngine(Options{StateRoot: "relative"}); ErrorCode(err) != "WORKFLOW_STATE_ROOT_INVALID" {
		t.Fatalf("invalid NewEngine() error = %v", err)
	}
	var absent *Engine
	if _, err := absent.Exchange(Command{}); ErrorCode(err) != "WORKFLOW_ENGINE_UNAVAILABLE" {
		t.Fatalf("nil Engine Exchange() error = %v", err)
	}
	if ErrorCode(errors.New("not a Coordinator error")) != "" {
		t.Fatal("ErrorCode() classified a non-Coordinator error")
	}
	engine, err := NewEngine(Options{StateRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Exchange(Command{SchemaVersion: "oaw.runtime/v1"}); ErrorCode(err) != "SCHEMA_UNSUPPORTED" {
		t.Fatalf("unsupported Exchange schema error = %v", err)
	}
}
