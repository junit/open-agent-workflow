package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/runtime"
)

func TestStartDirectRunCommitsReleasedSnapshotBeforeReply(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	projectRoot := t.TempDir()
	physicalProjectRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(project root) error = %v", err)
	}
	engine, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	reply, err := engine.Exchange(runtime.RunFrame{
		SchemaVersion:  runtime.RuntimeSchemaV1,
		Kind:           runtime.FrameStart,
		MessageID:      "message-001",
		IdempotencyKey: "start-direct-001",
		Start: &runtime.StartInput{
			RequestID: "request-001",
			Project: runtime.ProjectIdentity{
				Root:                projectRoot,
				ConfigurationDigest: strings.Repeat("a", 64),
			},
			Proposal: directProposal(),
		},
	})
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}

	if reply.Kind != runtime.ReplyModeDecided {
		t.Fatalf("reply kind = %q, want %q", reply.Kind, runtime.ReplyModeDecided)
	}
	if reply.RunID == "" || reply.Revision != 1 || reply.RevisionDigest == "" {
		t.Fatalf("reply identity = %#v", reply)
	}
	if reply.Snapshot.RequestMode != classification.RequestModeDirect || reply.Snapshot.Status != runtime.RunReleased {
		t.Fatalf("snapshot mode/status = %q/%q", reply.Snapshot.RequestMode, reply.Snapshot.Status)
	}
	if reply.Snapshot.ClassificationDigest == "" || reply.Snapshot.Classification.RequestMode != classification.RequestModeDirect {
		t.Fatalf("classification = %#v", reply.Snapshot.Classification)
	}
	if reply.Snapshot.Project.Root != physicalProjectRoot {
		t.Fatalf("physical project root = %q, want %q", reply.Snapshot.Project.Root, physicalProjectRoot)
	}
	if reply.Snapshot.LifecycleBundles == nil || len(reply.Snapshot.LifecycleBundles) != 0 {
		t.Fatalf("lifecycle bundles = %#v, want non-nil empty", reply.Snapshot.LifecycleBundles)
	}
	if reply.Snapshot.GrantIDs == nil || len(reply.Snapshot.GrantIDs) != 0 {
		t.Fatalf("grant IDs = %#v, want non-nil empty", reply.Snapshot.GrantIDs)
	}
	if reply.Snapshot.ResourceLeaseIDs == nil || len(reply.Snapshot.ResourceLeaseIDs) != 0 {
		t.Fatalf("resource lease IDs = %#v, want non-nil empty", reply.Snapshot.ResourceLeaseIDs)
	}
	assertDiagnosticCodes(t, reply.Diagnostics,
		runtime.DiagnosticDirectOutsideCapabilityAdmission,
		runtime.DiagnosticHostToolCallsUncontrolled,
		runtime.DiagnosticResourceLeaseNotApplicable,
	)

	runRoot := filepath.Join(stateRoot, "runs", reply.RunID)
	for _, path := range []string{
		filepath.Join(runRoot, "HEAD"),
		filepath.Join(runRoot, "revisions", "00000000000000000001.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("committed file %q unavailable after Exchange returned: %v", path, err)
		}
	}
}

func directProposal() *classification.ClassificationProposal {
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

func assertDiagnosticCodes(t *testing.T, diagnostics []runtime.Diagnostic, expected ...string) {
	t.Helper()
	seen := make(map[string]bool, len(diagnostics))
	for _, diagnostic := range diagnostics {
		seen[diagnostic.Code] = true
	}
	for _, code := range expected {
		if !seen[code] {
			t.Errorf("diagnostic %q missing from %#v", code, diagnostics)
		}
	}
}
