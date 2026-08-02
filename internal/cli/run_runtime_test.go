package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	oawruntime "github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestRunWithInputRuntimeExchangeEmitsCanonicalJSONOnly(t *testing.T) {
	projectRoot := t.TempDir()
	frame := oawruntime.RunFrame{
		SchemaVersion:  oawruntime.RuntimeSchemaV1,
		Kind:           oawruntime.FrameStart,
		MessageID:      "cli-runtime-start",
		IdempotencyKey: "cli-runtime-start",
		Start: &oawruntime.StartInput{
			RequestID: "cli-runtime-request",
			Project:   oawruntime.ProjectIdentity{Root: projectRoot, ConfigurationDigest: strings.Repeat("a", 64)},
			Proposal:  cliDirectProposal(),
		},
	}
	input, err := canonicaljson.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := RunWithInput(
		[]string{"runtime", "exchange", "--state-root", filepath.Join(t.TempDir(), "runtime")},
		bytes.NewReader(input), &stdout, &stderr,
	)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("runtime exchange status=%d stderr=%q", status, stderr.String())
	}
	var reply oawruntime.RunReply
	if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
		t.Fatalf("stdout = %q: %v", stdout.String(), err)
	}
	canonical, err := oawruntime.EncodeReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), canonical) || reply.Kind != oawruntime.ReplyModeDecided {
		t.Fatalf("runtime exchange stdout = %q", stdout.String())
	}
}

func TestRunWithInputRuntimeExchangeReturnsMachineDenialAndStderrDiagnostic(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := RunWithInput(
		[]string{"runtime", "exchange", "--state-root", filepath.Join(t.TempDir(), "runtime")},
		strings.NewReader(`{"schema_version":"oaw.runtime/v1","kind":"INSPECT","unknown":true}`),
		&stdout, &stderr,
	)
	if status == 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("invalid frame status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	var denial struct {
		SchemaVersion string `json:"schema_version"`
		Kind          string `json:"kind"`
		Reason        string `json:"reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &denial); err != nil {
		t.Fatal(err)
	}
	if denial.SchemaVersion != oawruntime.RuntimeSchemaV1 || denial.Kind != "DENIED" || denial.Reason != "RUNTIME_FRAME_DECODE_INVALID" {
		t.Fatalf("denial = %#v", denial)
	}
	if !strings.Contains(stderr.String(), "RUNTIME_FRAME_DECODE_INVALID") || strings.Contains(stdout.String(), "unknown field") {
		t.Fatalf("stream separation stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestParseRunCommandAcceptsBoundedProjectRootOption(t *testing.T) {
	projectRoot := filepath.Join(string(filepath.Separator), "workspace", "project")
	parsed, err := parseRunCommand([]string{"--host", "codex", "--state-root", filepath.Join(string(filepath.Separator), "state"), "--project-root=" + projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.projectRoot != projectRoot {
		t.Fatalf("project root = %q, want %q", parsed.projectRoot, projectRoot)
	}
}

func TestParseRunCommandRejectsUnsafeProjectRootOption(t *testing.T) {
	if _, err := parseRunCommand([]string{"--host", "codex", "--project-root", "relative/project"}); err == nil {
		t.Fatal("relative project root was accepted")
	}
}

func cliDirectProposal() *classification.ClassificationProposal {
	trueTraits := map[classification.Trait]bool{
		classification.TraitScopeClear:               true,
		classification.TraitChangePointKnown:         true,
		classification.TraitRecoverable:              true,
		classification.TraitFocusedVerificationKnown: true,
	}
	traits := make([]classification.TraitObservation, 0, 19)
	for _, trait := range []classification.Trait{
		classification.TraitScopeClear,
		classification.TraitChangePointKnown,
		classification.TraitRecoverable,
		classification.TraitFocusedVerificationKnown,
		classification.TraitBoundedCapabilityRequest,
		classification.TraitArchitectureDecision,
		classification.TraitPublicContractChange,
		classification.TraitSchemaChange,
		classification.TraitDependencyChange,
		classification.TraitSecuritySensitive,
		classification.TraitDataSensitive,
		classification.TraitDeploymentChange,
		classification.TraitDomainUncertainty,
		classification.TraitRootCauseUncertain,
		classification.TraitMultipleResponsibilities,
		classification.TraitMultipleTickets,
		classification.TraitLongLivedDelegation,
		classification.TraitDestructiveMutation,
		classification.TraitCriticalRelease,
	} {
		value := classification.TraitFalse
		if trueTraits[trait] {
			value = classification.TraitTrue
		}
		traits = append(traits, classification.TraitObservation{Trait: trait, Value: value})
	}
	return &classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1,
		Traits:        traits,
		Resources:     []classification.Resource{classification.ResourceProject},
		Evidence: []classification.ProposalEvidence{
			{Kind: classification.EvidenceScope, Reference: "test:scope", Digest: strings.Repeat("b", 64)},
			{Kind: classification.EvidenceChangePoint, Reference: "test:change", Digest: strings.Repeat("c", 64)},
			{Kind: classification.EvidenceVerification, Reference: "test:verification", Digest: strings.Repeat("d", 64)},
		},
	}
}
