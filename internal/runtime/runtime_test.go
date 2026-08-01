package runtime_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
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

func TestInspectReturnsCommittedSnapshotWithoutRevision(t *testing.T) {
	stateRoot, engine, started := startDirectRun(t)
	runRoot := filepath.Join(stateRoot, "runs", started.RunID)
	headPath := filepath.Join(runRoot, "HEAD")
	revisionsRoot := filepath.Join(runRoot, "revisions")
	headBefore, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("ReadFile(HEAD) error = %v", err)
	}
	revisionsBefore, err := os.ReadDir(revisionsRoot)
	if err != nil {
		t.Fatalf("ReadDir(revisions) error = %v", err)
	}

	inspected, err := engine.Exchange(inspectFrame(started.RunID, "inspect-001"))
	if err != nil {
		t.Fatalf("Exchange(INSPECT) error = %v", err)
	}
	if inspected.Kind != runtime.ReplyStateSnapshot || inspected.Revision != 1 || inspected.RevisionDigest != started.RevisionDigest {
		t.Fatalf("inspect reply = %#v", inspected)
	}
	if !reflect.DeepEqual(inspected.Snapshot, started.Snapshot) {
		t.Fatalf("inspect snapshot differs from committed START\n got: %#v\nwant: %#v", inspected.Snapshot, started.Snapshot)
	}
	headAfter, err := os.ReadFile(headPath)
	if err != nil {
		t.Fatalf("ReadFile(HEAD after inspect) error = %v", err)
	}
	revisionsAfter, err := os.ReadDir(revisionsRoot)
	if err != nil {
		t.Fatalf("ReadDir(revisions after inspect) error = %v", err)
	}
	if !bytes.Equal(headBefore, headAfter) || len(revisionsAfter) != len(revisionsBefore) {
		t.Fatalf("INSPECT mutated journal: HEAD changed=%v revisions=%d->%d", !bytes.Equal(headBefore, headAfter), len(revisionsBefore), len(revisionsAfter))
	}
}

func TestNewEngineReadsCommittedDirectRunAfterRestart(t *testing.T) {
	stateRoot, _, started := startDirectRun(t)
	restarted, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine(restart) error = %v", err)
	}
	inspected, err := restarted.Exchange(inspectFrame(started.RunID, "inspect-after-restart"))
	if err != nil {
		t.Fatalf("Exchange(INSPECT after restart) error = %v", err)
	}
	if !reflect.DeepEqual(inspected.Snapshot, started.Snapshot) || inspected.RevisionDigest != started.RevisionDigest {
		t.Fatalf("restarted snapshot differs from commit\n got: %#v\nwant: %#v", inspected.Snapshot, started.Snapshot)
	}
}

func TestInspectReturnsDefensiveSnapshotCopies(t *testing.T) {
	_, engine, started := startDirectRun(t)
	evidenceRequirementCount := len(started.Snapshot.Classification.EvidenceRequirements)
	started.Snapshot.ProcessedMessages[0].ContentDigest = strings.Repeat("0", 64)
	started.Snapshot.LifecycleBundles = append(started.Snapshot.LifecycleBundles, "mutated")
	started.Snapshot.GrantIDs = append(started.Snapshot.GrantIDs, "mutated")
	started.Snapshot.ResourceLeaseIDs = append(started.Snapshot.ResourceLeaseIDs, "mutated")
	started.Snapshot.Classification.EscalationReasons = append(started.Snapshot.Classification.EscalationReasons, "mutated")

	first, err := engine.Exchange(inspectFrame(started.RunID, "inspect-copy-001"))
	if err != nil {
		t.Fatalf("first INSPECT error = %v", err)
	}
	first.Snapshot.ProcessedMessages[0].ContentDigest = strings.Repeat("1", 64)
	first.Snapshot.LifecycleBundles = append(first.Snapshot.LifecycleBundles, "mutated-again")
	first.Snapshot.Classification.EvidenceRequirements = append(first.Snapshot.Classification.EvidenceRequirements, classification.EvidenceRequirement{Kind: classification.EvidenceScope})

	second, err := engine.Exchange(inspectFrame(started.RunID, "inspect-copy-002"))
	if err != nil {
		t.Fatalf("second INSPECT error = %v", err)
	}
	if second.Snapshot.ProcessedMessages[0].ContentDigest == strings.Repeat("0", 64) || second.Snapshot.ProcessedMessages[0].ContentDigest == strings.Repeat("1", 64) {
		t.Fatal("returned mutation reached committed processed messages")
	}
	if len(second.Snapshot.LifecycleBundles) != 0 || len(second.Snapshot.GrantIDs) != 0 || len(second.Snapshot.ResourceLeaseIDs) != 0 || len(second.Snapshot.Classification.EscalationReasons) != 0 || len(second.Snapshot.Classification.EvidenceRequirements) != evidenceRequirementCount {
		t.Fatalf("returned mutation reached committed state: %#v", second.Snapshot)
	}
}

func TestInspectFailsClosedForMissingAndCorruptState(t *testing.T) {
	t.Run("missing run", func(t *testing.T) {
		engine, err := runtime.NewEngine(runtime.Options{StateRoot: filepath.Join(t.TempDir(), "state")})
		if err != nil {
			t.Fatalf("NewEngine() error = %v", err)
		}
		_, err = engine.Exchange(inspectFrame("run-0123456789abcdef0123456789abcdef", "inspect-missing"))
		assertErrorCode(t, err, "RUN_NOT_FOUND")
	})

	for _, test := range []struct {
		name string
		edit func(t *testing.T, runRoot string)
		code string
	}{
		{
			name: "malformed HEAD",
			edit: func(t *testing.T, runRoot string) {
				t.Helper()
				writeTestFile(t, filepath.Join(runRoot, "HEAD"), []byte(`{"broken":`))
			},
			code: "RUN_STATE_HEAD_INVALID",
		},
		{
			name: "missing revision",
			edit: func(t *testing.T, runRoot string) {
				t.Helper()
				if err := os.Remove(filepath.Join(runRoot, "revisions", "00000000000000000001.json")); err != nil {
					t.Fatalf("Remove(revision) error = %v", err)
				}
			},
			code: "RUN_STATE_REVISION_INVALID",
		},
		{
			name: "revision digest mismatch",
			edit: func(t *testing.T, runRoot string) {
				t.Helper()
				path := filepath.Join(runRoot, "revisions", "00000000000000000001.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile(revision) error = %v", err)
				}
				writeTestFile(t, path, bytes.Replace(raw, []byte("DIRECT_RELEASED"), []byte("DIRECT_TAMPERED"), 1))
			},
			code: "RUN_STATE_DIGEST_MISMATCH",
		},
		{
			name: "state digest mismatch",
			edit: func(t *testing.T, runRoot string) {
				t.Helper()
				path := filepath.Join(runRoot, "revisions", "00000000000000000001.json")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("ReadFile(revision) error = %v", err)
				}
				writeTestFile(t, path, bytes.Replace(raw, []byte(`"status":"RELEASED"`), []byte(`"status":"TAMPERED"`), 1))
			},
			code: "RUN_STATE_DIGEST_MISMATCH",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, engine, started := startDirectRun(t)
			test.edit(t, filepath.Join(stateRoot, "runs", started.RunID))
			_, err := engine.Exchange(inspectFrame(started.RunID, "inspect-corrupt"))
			assertErrorCode(t, err, test.code)
		})
	}
}

func TestDirectScopeExpansionRequiresSuccessorRun(t *testing.T) {
	stateRoot, engine, started := startDirectRun(t)
	reply, err := engine.Exchange(continueFrame(started.RunID, started.Revision, "continue-scope", "scope-expanded"))
	if err != nil {
		t.Fatalf("Exchange(CONTINUE scope expansion) error = %v", err)
	}
	if reply.Kind != runtime.ReplyPaused || reply.Reason != runtime.ReasonModeEscalationRequired {
		t.Fatalf("scope expansion reply kind/reason = %q/%q", reply.Kind, reply.Reason)
	}
	if !reflect.DeepEqual(reply.RecoveryActions, []string{runtime.RecoveryStartSuccessorRun}) {
		t.Fatalf("recovery actions = %#v", reply.RecoveryActions)
	}
	if reply.Revision != 2 || reply.RevisionDigest == "" || reply.Snapshot.Revision != 2 {
		t.Fatalf("scope expansion revision = %#v", reply)
	}
	if reply.Snapshot.RequestMode != classification.RequestModeDirect || reply.Snapshot.Status != runtime.RunReleased {
		t.Fatalf("scope expansion changed mode/status = %q/%q", reply.Snapshot.RequestMode, reply.Snapshot.Status)
	}
	if len(reply.Snapshot.LifecycleBundles) != 0 || len(reply.Snapshot.GrantIDs) != 0 || len(reply.Snapshot.ResourceLeaseIDs) != 0 {
		t.Fatalf("scope expansion created authority: %#v", reply.Snapshot)
	}
	if len(reply.Diagnostics) != 0 {
		t.Fatalf("scope expansion diagnostics = %#v, want empty", reply.Diagnostics)
	}
	if len(reply.Snapshot.ProcessedMessages) != 2 {
		t.Fatalf("processed messages = %#v", reply.Snapshot.ProcessedMessages)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "runs", started.RunID, "revisions", "00000000000000000002.json")); err != nil {
		t.Fatalf("revision 2 unavailable after reply: %v", err)
	}

	inspected, err := engine.Exchange(inspectFrame(started.RunID, "inspect-expanded"))
	if err != nil {
		t.Fatalf("INSPECT expanded run error = %v", err)
	}
	if !reflect.DeepEqual(inspected.Snapshot, reply.Snapshot) || inspected.RevisionDigest != reply.RevisionDigest {
		t.Fatalf("committed expansion differs from reply\n got: %#v\nwant: %#v", inspected, reply)
	}
}

func TestContinueRejectsRevisionConflictWithoutWriting(t *testing.T) {
	stateRoot, engine, started := startDirectRun(t)
	runRoot := filepath.Join(stateRoot, "runs", started.RunID)
	headBefore, err := os.ReadFile(filepath.Join(runRoot, "HEAD"))
	if err != nil {
		t.Fatalf("ReadFile(HEAD) error = %v", err)
	}

	_, err = engine.Exchange(continueFrame(started.RunID, started.Revision+1, "continue-conflict", "scope-conflict"))
	assertErrorCode(t, err, "RUN_REVISION_CONFLICT")
	headAfter, err := os.ReadFile(filepath.Join(runRoot, "HEAD"))
	if err != nil {
		t.Fatalf("ReadFile(HEAD after conflict) error = %v", err)
	}
	if !bytes.Equal(headBefore, headAfter) {
		t.Fatal("revision conflict changed HEAD")
	}
	if _, err := os.Stat(filepath.Join(runRoot, "revisions", "00000000000000000002.json")); !os.IsNotExist(err) {
		t.Fatalf("revision conflict created revision 2: %v", err)
	}
}

func TestExchangeRejectsInvalidFrameShapesWithoutMutation(t *testing.T) {
	t.Run("missing classification", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
		if err != nil {
			t.Fatalf("NewEngine() error = %v", err)
		}
		frame := startFrame(t.TempDir(), "start-missing-proposal", "missing-proposal")
		frame.Start.Proposal = nil
		_, err = engine.Exchange(frame)
		assertErrorCode(t, err, "CLASSIFICATION_REQUIRED")
		assertRunsRootEmpty(t, stateRoot)
	})

	t.Run("mixed START payload", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
		if err != nil {
			t.Fatalf("NewEngine() error = %v", err)
		}
		frame := startFrame(t.TempDir(), "start-mixed", "start-mixed")
		frame.Continue = &runtime.ContinueInput{Signal: runtime.SignalScopeExpanded}
		_, err = engine.Exchange(frame)
		assertErrorCode(t, err, "RUNTIME_FRAME_INVALID")
		assertRunsRootEmpty(t, stateRoot)
	})

	for _, test := range []struct {
		name   string
		mutate func(*runtime.RunFrame)
	}{
		{"unknown CONTINUE signal", func(frame *runtime.RunFrame) { frame.Continue.Signal = "UNKNOWN" }},
		{"CONTINUE carrying START", func(frame *runtime.RunFrame) { frame.Start = &runtime.StartInput{} }},
		{"CONTINUE without expected revision", func(frame *runtime.RunFrame) { frame.ExpectedRevision = 0 }},
		{"INSPECT carrying CONTINUE", func(frame *runtime.RunFrame) { frame.Kind = runtime.FrameInspect }},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, engine, started := startDirectRun(t)
			runRoot := filepath.Join(stateRoot, "runs", started.RunID)
			headBefore, err := os.ReadFile(filepath.Join(runRoot, "HEAD"))
			if err != nil {
				t.Fatalf("ReadFile(HEAD) error = %v", err)
			}
			frame := continueFrame(started.RunID, started.Revision, "continue-invalid", "continue-invalid")
			test.mutate(&frame)
			_, err = engine.Exchange(frame)
			assertErrorCode(t, err, "RUNTIME_FRAME_INVALID")
			headAfter, readErr := os.ReadFile(filepath.Join(runRoot, "HEAD"))
			if readErr != nil {
				t.Fatalf("ReadFile(HEAD after invalid frame) error = %v", readErr)
			}
			if !bytes.Equal(headBefore, headAfter) {
				t.Fatal("invalid frame changed HEAD")
			}
		})
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

func startDirectRun(t *testing.T) (string, *runtime.Engine, runtime.RunReply) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine, err := runtime.NewEngine(runtime.Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	reply, err := engine.Exchange(startFrame(t.TempDir(), "message-start", "start-direct"))
	if err != nil {
		t.Fatalf("Exchange(START) error = %v", err)
	}
	return stateRoot, engine, reply
}

func startFrame(projectRoot, messageID, idempotencyKey string) runtime.RunFrame {
	return runtime.RunFrame{
		SchemaVersion:  runtime.RuntimeSchemaV1,
		Kind:           runtime.FrameStart,
		MessageID:      messageID,
		IdempotencyKey: idempotencyKey,
		Start: &runtime.StartInput{
			RequestID: "request-001",
			Project: runtime.ProjectIdentity{
				Root:                projectRoot,
				ConfigurationDigest: strings.Repeat("a", 64),
			},
			Proposal: directProposal(),
		},
	}
}

func inspectFrame(runID, messageID string) runtime.RunFrame {
	return runtime.RunFrame{
		SchemaVersion:  runtime.RuntimeSchemaV1,
		Kind:           runtime.FrameInspect,
		MessageID:      messageID,
		IdempotencyKey: messageID,
		RunID:          runID,
	}
}

func continueFrame(runID string, expectedRevision uint64, messageID, idempotencyKey string) runtime.RunFrame {
	return runtime.RunFrame{
		SchemaVersion:    runtime.RuntimeSchemaV1,
		Kind:             runtime.FrameContinue,
		MessageID:        messageID,
		IdempotencyKey:   idempotencyKey,
		RunID:            runID,
		ExpectedRevision: expectedRevision,
		Continue:         &runtime.ContinueInput{Signal: runtime.SignalScopeExpanded},
	}
}

func assertErrorCode(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %s", expected)
	}
	if code := runtime.ErrorCode(err); code != expected {
		t.Fatalf("error code = %q, want %q (error: %v)", code, expected, err)
	}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func assertRunsRootEmpty(t *testing.T, stateRoot string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(stateRoot, "runs"))
	if err != nil {
		t.Fatalf("ReadDir(runs) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid frame created Run State: %#v", entries)
	}
}
