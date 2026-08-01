package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

func TestRevisionValidationFailsClosedForEveryPinnedIdentity(t *testing.T) {
	_, _, _, committed := internalCommittedRevision(t)
	if err := validateRevision(committed, committed.RunID, committed.Revision); err != nil {
		t.Fatalf("valid committed revision rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*revisionRecord)
		code   string
	}{
		{"revision schema", func(value *revisionRecord) { value.SchemaVersion = "wrong" }, "RUN_STATE_REVISION_INVALID"},
		{"revision run", func(value *revisionRecord) { value.RunID = "run-00000000000000000000000000000000" }, "RUN_STATE_REVISION_INVALID"},
		{"revision number", func(value *revisionRecord) { value.Revision = 2 }, "RUN_STATE_REVISION_INVALID"},
		{"message ID", func(value *revisionRecord) { value.MessageID = "" }, "RUN_STATE_REVISION_INVALID"},
		{"idempotency key", func(value *revisionRecord) { value.IdempotencyKey = "" }, "RUN_STATE_REVISION_INVALID"},
		{"message digest", func(value *revisionRecord) { value.MessageDigest = "bad" }, "RUN_STATE_REVISION_INVALID"},
		{"event", func(value *revisionRecord) { value.Event = "" }, "RUN_STATE_REVISION_INVALID"},
		{"revision one predecessor", func(value *revisionRecord) { value.PredecessorDigest = strings.Repeat("0", 64) }, "RUN_STATE_REVISION_INVALID"},
		{"snapshot schema", func(value *revisionRecord) { value.Snapshot.SchemaVersion = "wrong" }, "RUN_STATE_REVISION_INVALID"},
		{"snapshot run", func(value *revisionRecord) { value.Snapshot.RunID = "run-00000000000000000000000000000000" }, "RUN_STATE_REVISION_INVALID"},
		{"snapshot revision", func(value *revisionRecord) { value.Snapshot.Revision = 2 }, "RUN_STATE_REVISION_INVALID"},
		{"record configuration", func(value *revisionRecord) { value.ConfigurationDigest = strings.Repeat("0", 64) }, "RUN_STATE_REVISION_INVALID"},
		{"project configuration", func(value *revisionRecord) { value.Snapshot.Project.ConfigurationDigest = strings.Repeat("0", 64) }, "RUN_STATE_REVISION_INVALID"},
		{"state digest", func(value *revisionRecord) { value.StateDigest = strings.Repeat("0", 64) }, "RUN_STATE_DIGEST_MISMATCH"},
		{"reply schema", func(value *revisionRecord) { value.Reply.SchemaVersion = "wrong" }, "RUN_STATE_REVISION_INVALID"},
		{"reply run", func(value *revisionRecord) { value.Reply.RunID = "run-00000000000000000000000000000000" }, "RUN_STATE_REVISION_INVALID"},
		{"reply revision", func(value *revisionRecord) { value.Reply.Revision = 2 }, "RUN_STATE_REVISION_INVALID"},
		{"reply revision digest", func(value *revisionRecord) { value.Reply.RevisionDigest = strings.Repeat("0", 64) }, "RUN_STATE_REVISION_INVALID"},
		{"reply state", func(value *revisionRecord) { value.Reply.Snapshot.Status = "TAMPERED" }, "RUN_STATE_DIGEST_MISMATCH"},
		{"revision digest shape", func(value *revisionRecord) { value.Digest = "bad"; value.Reply.RevisionDigest = "bad" }, "RUN_STATE_REVISION_INVALID"},
		{"revision digest content", func(value *revisionRecord) { value.Reply.Diagnostics[0].Message = "tampered" }, "RUN_STATE_DIGEST_MISMATCH"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := committed
			candidate.Snapshot = cloneSnapshot(committed.Snapshot)
			candidate.Reply = cloneReply(committed.Reply)
			test.mutate(&candidate)
			err := validateRevision(candidate, committed.RunID, committed.Revision)
			assertInternalErrorCode(t, err, test.code)
		})
	}

	revisionTwo := committed
	revisionTwo.Revision = 2
	revisionTwo.Snapshot.Revision = 2
	revisionTwo.Reply.Revision = 2
	revisionTwo.PredecessorDigest = "bad"
	assertInternalErrorCode(t, validateRevision(revisionTwo, committed.RunID, 2), "RUN_STATE_REVISION_INVALID")
}

func TestJournalRejectsInvalidHeadAndSignedPredecessorFork(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*headRecord)
		code   string
	}{
		{"schema", func(value *headRecord) { value.SchemaVersion = "wrong" }, "RUN_STATE_HEAD_INVALID"},
		{"run", func(value *headRecord) { value.RunID = "run-00000000000000000000000000000000" }, "RUN_STATE_HEAD_INVALID"},
		{"revision", func(value *headRecord) { value.Revision = 0 }, "RUN_STATE_HEAD_INVALID"},
		{"digest shape", func(value *headRecord) { value.RevisionDigest = "bad" }, "RUN_STATE_HEAD_INVALID"},
		{"digest content", func(value *headRecord) { value.RevisionDigest = strings.Repeat("0", 64) }, "RUN_STATE_DIGEST_MISMATCH"},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, stateRoot, reply, _ := internalCommittedRevision(t)
			head := headRecord{SchemaVersion: headSchemaV1, RunID: reply.RunID, Revision: reply.Revision, RevisionDigest: reply.RevisionDigest}
			test.mutate(&head)
			writeInternalCanonical(t, filepath.Join(stateRoot, "runs", reply.RunID, "HEAD"), head)
			_, err := engine.journal.loadCommitted(reply.RunID)
			assertInternalErrorCode(t, err, test.code)
		})
	}

	t.Run("strict trailing JSON", func(t *testing.T) {
		engine, stateRoot, reply, _ := internalCommittedRevision(t)
		headPath := filepath.Join(stateRoot, "runs", reply.RunID, "HEAD")
		raw, err := os.ReadFile(headPath)
		if err != nil {
			t.Fatalf("ReadFile(HEAD) error = %v", err)
		}
		if err := os.WriteFile(headPath, append(raw, []byte(` {}`)...), 0o600); err != nil {
			t.Fatalf("WriteFile(HEAD) error = %v", err)
		}
		_, err = engine.journal.loadCommitted(reply.RunID)
		assertInternalErrorCode(t, err, "RUN_STATE_HEAD_INVALID")
	})

	t.Run("signed predecessor fork", func(t *testing.T) {
		engine, stateRoot, started, _ := internalCommittedRevision(t)
		continued, err := engine.Exchange(RunFrame{
			SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: "continue", IdempotencyKey: "continue",
			RunID: started.RunID, ExpectedRevision: 1, Continue: &ContinueInput{Signal: SignalScopeExpanded},
		})
		if err != nil {
			t.Fatalf("CONTINUE error = %v", err)
		}
		revision, err := engine.journal.loadRevision(started.RunID, 2)
		if err != nil {
			t.Fatalf("loadRevision(2) error = %v", err)
		}
		revision.PredecessorDigest = strings.Repeat("0", 64)
		resignRevision(t, &revision)
		writeInternalCanonical(t, filepath.Join(stateRoot, "runs", started.RunID, "revisions", revisionFileName(2)), revision)
		writeInternalCanonical(t, filepath.Join(stateRoot, "runs", started.RunID, "HEAD"), headRecord{
			SchemaVersion: headSchemaV1, RunID: started.RunID, Revision: 2, RevisionDigest: revision.Digest,
		})
		if continued.Revision != 2 {
			t.Fatalf("continued revision = %d", continued.Revision)
		}
		_, err = engine.journal.loadCommitted(started.RunID)
		assertInternalErrorCode(t, err, "RUN_STATE_REVISION_INVALID")
	})
}

func TestEngineAndJournalRejectInvalidBoundaries(t *testing.T) {
	fileRoot := filepath.Join(t.TempDir(), "state-file")
	if err := os.WriteFile(fileRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, options := range []Options{{StateRoot: "relative"}, {StateRoot: fileRoot}} {
		if _, err := NewEngine(options); err == nil {
			t.Fatalf("NewEngine(%q) error = nil", options.StateRoot)
		}
	}

	stateRoot := filepath.Join(t.TempDir(), "state")
	engine, err := NewEngine(Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatal(err)
	}
	invalidFrames := []struct {
		frame RunFrame
		code  string
	}{
		{RunFrame{SchemaVersion: "wrong", MessageID: "message", IdempotencyKey: "key"}, "RUNTIME_SCHEMA_UNSUPPORTED"},
		{RunFrame{SchemaVersion: RuntimeSchemaV1, Kind: "UNKNOWN", MessageID: "message", IdempotencyKey: "key"}, "RUNTIME_FRAME_INVALID"},
		{RunFrame{SchemaVersion: RuntimeSchemaV1, Kind: FrameInspect, MessageID: "", IdempotencyKey: "key"}, "RUNTIME_FRAME_INVALID"},
		{RunFrame{SchemaVersion: RuntimeSchemaV1, Kind: FrameInspect, MessageID: "bad\nmessage", IdempotencyKey: "key"}, "RUNTIME_FRAME_INVALID"},
		{RunFrame{SchemaVersion: RuntimeSchemaV1, Kind: FrameInspect, MessageID: strings.Repeat("x", maximumIdentifierLength+1), IdempotencyKey: "key"}, "RUNTIME_FRAME_INVALID"},
		{RunFrame{SchemaVersion: RuntimeSchemaV1, Kind: FrameInspect, MessageID: "message", IdempotencyKey: ""}, "RUNTIME_FRAME_INVALID"},
		{RunFrame{SchemaVersion: RuntimeSchemaV1, Kind: FrameInspect, MessageID: "message", IdempotencyKey: "key", RunID: "bad"}, "RUNTIME_FRAME_INVALID"},
		{RunFrame{SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: "message", IdempotencyKey: "key", RunID: "run-0123456789abcdef0123456789abcdef", ExpectedRevision: 1, Continue: &ContinueInput{Signal: SignalScopeExpanded}}, "RUN_NOT_FOUND"},
	}
	for _, test := range invalidFrames {
		_, err := engine.Exchange(test.frame)
		assertInternalErrorCode(t, err, test.code)
	}

	runsRoot := filepath.Join(stateRoot, "runs")
	if err := os.RemoveAll(runsRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runsRoot, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	frame := internalStartFrame(t.TempDir(), "blocked-run-root")
	_, err = engine.Exchange(frame)
	assertInternalErrorCode(t, err, "RUN_STATE_HEAD_INVALID")
}

func TestStartRejectsInvalidInputsAndUnimplementedModesWithoutState(t *testing.T) {
	projectDirectory := t.TempDir()
	projectFile := filepath.Join(t.TempDir(), "project-file")
	if err := os.WriteFile(projectFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		options Options
		mutate  func(*RunFrame)
		code    string
	}{
		{"invalid request", Options{}, func(frame *RunFrame) { frame.Start.RequestID = "bad\nrequest" }, "RUNTIME_FRAME_INVALID"},
		{"relative project", Options{}, func(frame *RunFrame) { frame.Start.Project.Root = "relative" }, "PROJECT_IDENTITY_INVALID"},
		{"invalid configuration digest", Options{}, func(frame *RunFrame) { frame.Start.Project.ConfigurationDigest = strings.Repeat("A", 64) }, "PROJECT_IDENTITY_INVALID"},
		{"missing project", Options{}, func(frame *RunFrame) { frame.Start.Project.Root = filepath.Join(t.TempDir(), "missing") }, "PROJECT_IDENTITY_INVALID"},
		{"project is file", Options{}, func(frame *RunFrame) { frame.Start.Project.Root = projectFile }, "PROJECT_IDENTITY_INVALID"},
		{"invalid proposal", Options{}, func(frame *RunFrame) { frame.Start.Proposal.SchemaVersion = "wrong" }, "RUNTIME_FRAME_INVALID"},
		{"invalid rules", Options{Rules: classification.ClassificationRules{User: classification.PolicyLayer{MinimumMode: "INVALID"}}}, func(*RunFrame) {}, "RUNTIME_FRAME_INVALID"},
		{"workflow mode", Options{}, func(frame *RunFrame) {
			setInternalTrait(frame.Start.Proposal, classification.TraitSchemaChange, classification.TraitTrue)
		}, "REQUEST_MODE_NOT_IMPLEMENTED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "state")
			options := test.options
			options.StateRoot = stateRoot
			engine, err := NewEngine(options)
			if err != nil {
				t.Fatalf("NewEngine() error = %v", err)
			}
			frame := internalStartFrame(projectDirectory, "invalid-start")
			test.mutate(&frame)
			_, err = engine.Exchange(frame)
			assertInternalErrorCode(t, err, test.code)
			entries, readErr := os.ReadDir(filepath.Join(stateRoot, "runs"))
			if readErr != nil {
				t.Fatalf("ReadDir(runs) error = %v", readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("rejected START created %d Run directories", len(entries))
			}
		})
	}
}

func TestMutationsPropagateCorruptionAndWriteFailures(t *testing.T) {
	t.Run("START reports corrupt existing Run", func(t *testing.T) {
		engine, stateRoot, started, _ := internalCommittedRevision(t)
		headPath := filepath.Join(stateRoot, "runs", started.RunID, "HEAD")
		if err := os.WriteFile(headPath, []byte(`{"broken":`), 0o600); err != nil {
			t.Fatal(err)
		}
		frame := internalStartFrame(started.Snapshot.Project.Root, "internal-start")
		_, err := engine.Exchange(frame)
		assertInternalErrorCode(t, err, "RUN_STATE_HEAD_INVALID")
	})

	t.Run("orphan revision blocks START commit", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		engine, err := NewEngine(Options{StateRoot: stateRoot})
		if err != nil {
			t.Fatal(err)
		}
		frame := internalStartFrame(t.TempDir(), "orphan-start")
		runID := deriveRunID(frame.IdempotencyKey)
		revisionsRoot := filepath.Join(stateRoot, "runs", runID, "revisions")
		if err := os.MkdirAll(revisionsRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(revisionsRoot, revisionFileName(1)), []byte("orphan"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = engine.Exchange(frame)
		assertInternalErrorCode(t, err, "RUN_STATE_WRITE_FAILED")
	})

	t.Run("orphan revision blocks CONTINUE commit", func(t *testing.T) {
		engine, stateRoot, started, _ := internalCommittedRevision(t)
		path := filepath.Join(stateRoot, "runs", started.RunID, "revisions", revisionFileName(2))
		if err := os.WriteFile(path, []byte("orphan"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := engine.Exchange(RunFrame{
			SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: "continue", IdempotencyKey: "continue",
			RunID: started.RunID, ExpectedRevision: 1, Continue: &ContinueInput{Signal: SignalScopeExpanded},
		})
		assertInternalErrorCode(t, err, "RUN_STATE_WRITE_FAILED")
	})

	t.Run("CONTINUE rejects signed non-Direct state", func(t *testing.T) {
		engine, stateRoot, started, committed := internalCommittedRevision(t)
		committed.Snapshot.Status = "TAMPERED"
		committed.Reply.Snapshot = cloneSnapshot(committed.Snapshot)
		stateDigest, _, err := canonicaljson.Digest(committed.Snapshot)
		if err != nil {
			t.Fatal(err)
		}
		committed.StateDigest = stateDigest
		resignRevision(t, &committed)
		writeInternalCanonical(t, filepath.Join(stateRoot, "runs", started.RunID, "revisions", revisionFileName(1)), committed)
		writeInternalCanonical(t, filepath.Join(stateRoot, "runs", started.RunID, "HEAD"), headRecord{
			SchemaVersion: headSchemaV1, RunID: started.RunID, Revision: 1, RevisionDigest: committed.Digest,
		})
		_, err = engine.Exchange(RunFrame{
			SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: "continue", IdempotencyKey: "continue",
			RunID: started.RunID, ExpectedRevision: 1, Continue: &ContinueInput{Signal: SignalScopeExpanded},
		})
		assertInternalErrorCode(t, err, "RUN_STATE_REVISION_INVALID")
	})
}

func TestSignedSemanticStateTamperingFailsClosedOnLoad(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*revisionRecord)
	}{
		{"status", func(value *revisionRecord) { value.Snapshot.Status = "TAMPERED" }},
		{"request mode", func(value *revisionRecord) { value.Snapshot.RequestMode = classification.RequestModeWorkflow }},
		{"classification mode", func(value *revisionRecord) {
			value.Snapshot.Classification.RequestMode = classification.RequestModeWorkflow
		}},
		{"authority", func(value *revisionRecord) { value.Snapshot.GrantIDs = []string{"grant"} }},
		{"processed message", func(value *revisionRecord) {
			value.Snapshot.ProcessedMessages[0].ContentDigest = strings.Repeat("0", 64)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, stateRoot, started, committed := internalCommittedRevision(t)
			test.mutate(&committed)
			committed.Reply.Snapshot = cloneSnapshot(committed.Snapshot)
			stateDigest, _, err := canonicaljson.Digest(committed.Snapshot)
			if err != nil {
				t.Fatal(err)
			}
			committed.StateDigest = stateDigest
			resignRevision(t, &committed)
			writeInternalCanonical(t, filepath.Join(stateRoot, "runs", started.RunID, "revisions", revisionFileName(1)), committed)
			writeInternalCanonical(t, filepath.Join(stateRoot, "runs", started.RunID, "HEAD"), headRecord{
				SchemaVersion: headSchemaV1, RunID: started.RunID, Revision: 1, RevisionDigest: committed.Digest,
			})
			_, err = engine.journal.loadCommitted(started.RunID)
			assertInternalErrorCode(t, err, "RUN_STATE_REVISION_INVALID")
		})
	}
}

func TestDirectStateSemanticValidationRejectsMalformedCollectionsAndReplies(t *testing.T) {
	_, _, _, revisionOne := internalCommittedRevision(t)
	engine, _, started, _ := internalCommittedRevision(t)
	if _, err := engine.Exchange(RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameContinue, MessageID: "continue", IdempotencyKey: "continue",
		RunID: started.RunID, ExpectedRevision: 1, Continue: &ContinueInput{Signal: SignalScopeExpanded},
	}); err != nil {
		t.Fatalf("CONTINUE error = %v", err)
	}
	revisionTwo, err := engine.journal.loadCommitted(started.RunID)
	if err != nil {
		t.Fatalf("load revision 2 error = %v", err)
	}

	complexity := classification.ComplexityComplex
	selector := &classification.CapabilitySelector{ProviderID: "oaw.test/provider", CapabilityID: "test", Source: classification.SelectorUserIntent}
	tests := []struct {
		name   string
		base   revisionRecord
		mutate func(*revisionRecord)
	}{
		{"request ID", revisionOne, func(value *revisionRecord) { value.Snapshot.RequestID = "bad\nrequest" }},
		{"project root", revisionOne, func(value *revisionRecord) { value.Snapshot.Project.Root = "relative" }},
		{"configuration digest", revisionOne, func(value *revisionRecord) { value.Snapshot.ConfigurationDigest = "bad" }},
		{"nil messages", revisionOne, func(value *revisionRecord) { value.Snapshot.ProcessedMessages = nil }},
		{"message count", revisionOne, func(value *revisionRecord) {
			value.Snapshot.ProcessedMessages = append(value.Snapshot.ProcessedMessages, ProcessedMessage{})
		}},
		{"nil evidence requirements", revisionOne, func(value *revisionRecord) { value.Snapshot.Classification.EvidenceRequirements = nil }},
		{"nil escalation reasons", revisionOne, func(value *revisionRecord) { value.Snapshot.Classification.EscalationReasons = nil }},
		{"workflow complexity", revisionOne, func(value *revisionRecord) { value.Snapshot.Classification.WorkflowComplexity = &complexity }},
		{"capability selector", revisionOne, func(value *revisionRecord) { value.Snapshot.Classification.CapabilitySelector = selector }},
		{"message key", revisionOne, func(value *revisionRecord) { value.Snapshot.ProcessedMessages[0].IdempotencyKey = "bad\nkey" }},
		{"message digest", revisionOne, func(value *revisionRecord) { value.Snapshot.ProcessedMessages[0].ContentDigest = "bad" }},
		{"message revision zero", revisionOne, func(value *revisionRecord) { value.Snapshot.ProcessedMessages[0].Revision = 0 }},
		{"message revision high", revisionOne, func(value *revisionRecord) { value.Snapshot.ProcessedMessages[0].Revision = 2 }},
		{"messages not sorted", revisionTwo, func(value *revisionRecord) {
			value.Snapshot.ProcessedMessages[1].IdempotencyKey = value.Snapshot.ProcessedMessages[0].IdempotencyKey
		}},
		{"current message missing", revisionOne, func(value *revisionRecord) { value.IdempotencyKey = "different" }},
		{"nil diagnostics", revisionOne, func(value *revisionRecord) { value.Reply.Diagnostics = nil }},
		{"nil recovery", revisionOne, func(value *revisionRecord) { value.Reply.RecoveryActions = nil }},
		{"release reply", revisionOne, func(value *revisionRecord) { value.Reply.Kind = ReplyPaused }},
		{"escalation reply", revisionTwo, func(value *revisionRecord) { value.Reply.Reason = "wrong" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := test.base
			candidate.Snapshot = cloneSnapshot(test.base.Snapshot)
			candidate.Reply = cloneReply(test.base.Reply)
			test.mutate(&candidate)
			assertInternalErrorCode(t, validateDirectState(candidate), "RUN_STATE_REVISION_INVALID")
		})
	}
}

func TestBoundedStateSemanticValidationRejectsSignedTampering(t *testing.T) {
	ready := internalBoundedRevision(t, RunReady)
	awaiting := internalBoundedRevision(t, RunAwaitingCapability)
	if err := validateRevision(ready, ready.RunID, ready.Revision); err != nil {
		t.Fatalf("valid READY Bounded revision rejected: %v", err)
	}
	if err := validateRevision(awaiting, awaiting.RunID, awaiting.Revision); err != nil {
		t.Fatalf("valid AWAITING_CAPABILITY revision rejected: %v", err)
	}

	complexity := classification.ComplexityComplex
	for _, test := range []struct {
		name   string
		base   revisionRecord
		mutate func(*revisionRecord)
	}{
		{"status", ready, func(value *revisionRecord) { value.Snapshot.Status = "TAMPERED" }},
		{"missing Bounded state", ready, func(value *revisionRecord) { value.Snapshot.Bounded = nil }},
		{"configuration digest", ready, func(value *revisionRecord) { value.Snapshot.Bounded.ConfigurationDigest = strings.Repeat("0", 64) }},
		{"catalog digest", ready, func(value *revisionRecord) { value.Snapshot.Bounded.CatalogDigest = "bad" }},
		{"registry digest", ready, func(value *revisionRecord) { value.Snapshot.Bounded.RegistryDigest = "bad" }},
		{"workflow complexity", ready, func(value *revisionRecord) { value.Snapshot.Classification.WorkflowComplexity = &complexity }},
		{"classification selector provenance", ready, func(value *revisionRecord) {
			value.Snapshot.Classification.CapabilitySelector.Source = classification.SelectorTrustedRule
		}},
		{"status selector mismatch", ready, func(value *revisionRecord) { value.Snapshot.Bounded.Selector = nil }},
		{"selector provider", ready, func(value *revisionRecord) { value.Snapshot.Bounded.Selector.ProviderID = "bad" }},
		{"selector provenance", ready, func(value *revisionRecord) {
			value.Snapshot.Bounded.Selector.Source = classification.SelectorTrustedRule
		}},
		{"unsorted effects", ready, func(value *revisionRecord) {
			value.Snapshot.Bounded.Input.RequestedEffects = []string{"run-process", "read-project"}
		}},
		{"duplicate resources", ready, func(value *revisionRecord) {
			value.Snapshot.Bounded.Input.RequestedResources = []string{"project", "project"}
		}},
		{"authority leaked", ready, func(value *revisionRecord) { value.Snapshot.GrantIDs = []string{"grant"} }},
		{"ready reply kind", ready, func(value *revisionRecord) { value.Reply.Kind = ReplyCapabilitySelectionRequired }},
		{"ready event", ready, func(value *revisionRecord) { value.Event = "BOUNDED_AWAITING_CAPABILITY" }},
		{"awaiting selector", awaiting, func(value *revisionRecord) {
			value.Snapshot.Bounded.Selector = &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}
		}},
		{"awaiting diagnostic", awaiting, func(value *revisionRecord) { value.Reply.Diagnostics[0].Code = "UNTRUSTED" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := test.base
			candidate.Snapshot = cloneSnapshot(test.base.Snapshot)
			candidate.Reply = cloneReply(test.base.Reply)
			test.mutate(&candidate)
			resignStateRevision(t, &candidate)
			assertInternalErrorCode(t, validateRevision(candidate, candidate.RunID, candidate.Revision), "RUN_STATE_REVISION_INVALID")
		})
	}
}

func TestJournalRejectsOversizedStateAndConflictingValidOrphan(t *testing.T) {
	t.Run("oversized HEAD", func(t *testing.T) {
		engine, stateRoot, started, _ := internalCommittedRevision(t)
		headPath := filepath.Join(stateRoot, "runs", started.RunID, "HEAD")
		if err := os.WriteFile(headPath, []byte(strings.Repeat("x", maximumHeadBytes+1)), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := engine.journal.loadCommitted(started.RunID)
		assertInternalErrorCode(t, err, "RUN_STATE_HEAD_INVALID")
	})

	t.Run("oversized revision", func(t *testing.T) {
		engine, stateRoot, started, _ := internalCommittedRevision(t)
		revisionPath := filepath.Join(stateRoot, "runs", started.RunID, "revisions", revisionFileName(1))
		if err := os.WriteFile(revisionPath, []byte(strings.Repeat("x", maximumRevisionBytes+1)), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := engine.journal.loadCommitted(started.RunID)
		assertInternalErrorCode(t, err, "RUN_STATE_REVISION_INVALID")
	})

	t.Run("revision limit", func(t *testing.T) {
		engine, stateRoot, started, _ := internalCommittedRevision(t)
		writeInternalCanonical(t, filepath.Join(stateRoot, "runs", started.RunID, "HEAD"), headRecord{
			SchemaVersion: headSchemaV1, RunID: started.RunID, Revision: maximumRunRevisions + 1, RevisionDigest: started.RevisionDigest,
		})
		_, err := engine.journal.loadCommitted(started.RunID)
		assertInternalErrorCode(t, err, "RUN_STATE_HEAD_INVALID")
	})

	t.Run("valid conflicting orphan", func(t *testing.T) {
		engine, stateRoot, started, _ := internalCommittedRevision(t)
		headPath := filepath.Join(stateRoot, "runs", started.RunID, "HEAD")
		if err := os.Remove(headPath); err != nil {
			t.Fatal(err)
		}
		frame := internalStartFrame(started.Snapshot.Project.Root, "internal-start")
		frame.MessageID = "changed-message"
		frame.Start.Proposal.Evidence[0].Reference = "internal:changed-scope"
		_, err := engine.Exchange(frame)
		assertInternalErrorCode(t, err, "RUN_STATE_WRITE_FAILED")
	})
}

func TestJournalHelpersFailClosedOnInvalidFilesystemTargets(t *testing.T) {
	blockedStateRoot := filepath.Join(t.TempDir(), "blocked-state")
	if err := os.MkdirAll(blockedStateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockedStateRoot, "runs"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newJournal(blockedStateRoot); ErrorCode(err) != "RUN_STATE_WRITE_FAILED" {
		t.Fatalf("blocked runs root error = %v", err)
	}

	stateRoot := filepath.Join(t.TempDir(), "state")
	journal, err := newJournal(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.withRunLock("bad", func() error { return nil }); ErrorCode(err) != "RUNTIME_FRAME_INVALID" {
		t.Fatalf("withRunLock invalid ID error = %v", err)
	}
	if _, err := journal.inspect("bad"); ErrorCode(err) != "RUNTIME_FRAME_INVALID" {
		t.Fatalf("inspect invalid ID error = %v", err)
	}

	runID := "run-0123456789abcdef0123456789abcdef"
	runRoot := journal.runRoot(runID)
	if err := os.MkdirAll(runRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(runRoot, "LOCK"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := journal.withRunLock(runID, func() error { return nil }); ErrorCode(err) != "RUN_STATE_WRITE_FAILED" {
		t.Fatalf("directory lock error = %v", err)
	}

	blockedRunID := "run-fedcba9876543210fedcba9876543210"
	blockedRunRoot := journal.runRoot(blockedRunID)
	if err := os.MkdirAll(blockedRunRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockedRunRoot, "revisions"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.writeImmutableRevision(blockedRunID, 1, []byte("revision")); ErrorCode(err) != "RUN_STATE_WRITE_FAILED" {
		t.Fatalf("blocked revisions root error = %v", err)
	}

	if err := atomicWriteFile(filepath.Join(t.TempDir(), "missing", "HEAD"), []byte("head"), 0o600); err == nil {
		t.Fatal("atomicWriteFile missing directory error = nil")
	}
	targetDirectory := filepath.Join(t.TempDir(), "HEAD")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(targetDirectory, []byte("head"), 0o600); err == nil {
		t.Fatal("atomicWriteFile directory target error = nil")
	}
	if err := syncDirectory(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("syncDirectory missing path error = nil")
	}
	if _, err := readLimitedFile(t.TempDir(), maximumHeadBytes); err == nil {
		t.Fatal("readLimitedFile accepted a directory")
	}
	orphanDirectory := t.TempDir()
	if _, err := reuseMatchingOrphan(filepath.Join(orphanDirectory, "missing.json"), []byte("candidate"), orphanDirectory); ErrorCode(err) != "RUN_STATE_WRITE_FAILED" {
		t.Fatalf("missing orphan error = %v", err)
	}
	_, committedStateRoot, committedReply, _ := internalCommittedRevision(t)
	committedRevisionPath := filepath.Join(committedStateRoot, "runs", committedReply.RunID, "revisions", revisionFileName(1))
	if _, err := reuseMatchingOrphan(committedRevisionPath, []byte("invalid candidate"), filepath.Dir(committedRevisionPath)); ErrorCode(err) != "RUN_STATE_WRITE_FAILED" {
		t.Fatalf("invalid orphan candidate error = %v", err)
	}
	var decoded headRecord
	if err := decodeStrict([]byte(`{} trailing`), &decoded); err == nil {
		t.Fatal("decodeStrict invalid trailing bytes error = nil")
	}
	if validDigest(strings.Repeat("g", 64)) {
		t.Fatal("validDigest accepted non-hex input")
	}
}

func TestRecordCopiesAndRuntimeErrorsAreDefensive(t *testing.T) {
	if cloneProposal(nil) != nil {
		t.Fatal("cloneProposal(nil) returned non-nil")
	}
	selector := &classification.CapabilitySelector{ProviderID: "oaw.example/provider", CapabilityID: "capability", Source: classification.SelectorUserIntent}
	proposal := &classification.ClassificationProposal{
		Traits:             []classification.TraitObservation{{Trait: classification.TraitScopeClear, Value: classification.TraitTrue}},
		Resources:          []classification.Resource{classification.ResourceProject},
		Evidence:           []classification.ProposalEvidence{{Kind: classification.EvidenceScope}},
		CapabilitySelector: selector,
	}
	clonedProposal := cloneProposal(proposal)
	clonedProposal.Traits[0].Value = classification.TraitFalse
	clonedProposal.CapabilitySelector.CapabilityID = "changed"
	if proposal.Traits[0].Value != classification.TraitTrue || proposal.CapabilitySelector.CapabilityID != "capability" {
		t.Fatal("cloneProposal aliased source")
	}

	complexity := classification.ComplexityComplex
	decision := classification.ClassificationDecision{
		WorkflowComplexity:   &complexity,
		CapabilitySelector:   selector,
		EvidenceRequirements: []classification.EvidenceRequirement{{Kind: classification.EvidenceScope}},
		EscalationReasons:    []string{"reason"},
	}
	clonedDecision := cloneDecision(decision)
	*clonedDecision.WorkflowComplexity = classification.ComplexityOrdinary
	clonedDecision.CapabilitySelector.CapabilityID = "changed"
	if *decision.WorkflowComplexity != classification.ComplexityComplex || decision.CapabilitySelector.CapabilityID != "capability" {
		t.Fatal("cloneDecision aliased source")
	}

	cause := errors.New("cause")
	err := runtimeError("CODE", "message", cause)
	if err.Error() != "CODE: message" || !errors.Is(err, cause) || ErrorCode(errors.New("other")) != "" {
		t.Fatalf("runtime error contract failed: %v", err)
	}
	if (&Error{Code: "CODE"}).Error() != "CODE" {
		t.Fatal("code-only runtime error string is unstable")
	}
}

func internalCommittedRevision(t *testing.T) (*Engine, string, RunReply, revisionRecord) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "state")
	engine, err := NewEngine(Options{StateRoot: stateRoot})
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}
	reply, err := engine.Exchange(internalStartFrame(t.TempDir(), "internal-start"))
	if err != nil {
		t.Fatalf("START error = %v", err)
	}
	committed, err := engine.journal.loadCommitted(reply.RunID)
	if err != nil {
		t.Fatalf("loadCommitted() error = %v", err)
	}
	return engine, stateRoot, reply, committed
}

func internalBoundedRevision(t *testing.T, status RunStatus) revisionRecord {
	t.Helper()
	_, _, _, record := internalCommittedRevision(t)
	selector := &classification.CapabilitySelector{ProviderID: "oaw/ecc", CapabilityID: "review", Source: classification.SelectorUserIntent}
	record.Snapshot.RequestMode = classification.RequestModeBounded
	record.Snapshot.Status = status
	record.Snapshot.Classification.RequestMode = classification.RequestModeBounded
	record.Snapshot.Classification.CapabilitySelector = selector
	record.Snapshot.Bounded = &BoundedState{
		Input: BoundedInput{
			DeliverableID: "deliverable", InputDigest: strings.Repeat("1", 64), RequestedEffects: []string{"read-project"},
			RequestedResources: []string{"project"}, TerminationCondition: "one report", ExecutorID: "executor",
		},
		Selector: selector, ConfigurationDigest: record.Snapshot.ConfigurationDigest,
		CatalogDigest: strings.Repeat("b", 64), RegistryDigest: strings.Repeat("c", 64),
	}
	record.Event = "BOUNDED_READY"
	record.Reply.Kind = ReplyModeDecided
	record.Reply.Diagnostics = []Diagnostic{}
	record.Reply.Reason = ""
	record.Reply.RecoveryActions = []string{}
	if status == RunAwaitingCapability {
		record.Snapshot.Bounded.Selector = nil
		record.Snapshot.Classification.CapabilitySelector = nil
		record.Event = "BOUNDED_AWAITING_CAPABILITY"
		record.Reply.Kind = ReplyCapabilitySelectionRequired
		record.Reply.Diagnostics = []Diagnostic{{Code: "CAPABILITY_SELECTION_REQUIRED", Message: "selection required"}}
	}
	resignStateRevision(t, &record)
	return record
}

func resignStateRevision(t *testing.T, record *revisionRecord) {
	t.Helper()
	record.Reply.Snapshot = cloneSnapshot(record.Snapshot)
	stateDigest, _, err := canonicaljson.Digest(record.Snapshot)
	if err != nil {
		t.Fatalf("Digest(snapshot) error = %v", err)
	}
	record.StateDigest = stateDigest
	resignRevision(t, record)
}

func internalStartFrame(projectRoot, key string) RunFrame {
	proposal := internalDirectProposal()
	return RunFrame{
		SchemaVersion: RuntimeSchemaV1, Kind: FrameStart, MessageID: key + "-message", IdempotencyKey: key,
		Start: &StartInput{
			RequestID: "internal-request",
			Project:   ProjectIdentity{Root: projectRoot, ConfigurationDigest: strings.Repeat("a", 64)},
			Proposal:  &proposal,
		},
	}
}

func internalDirectProposal() classification.ClassificationProposal {
	positive := map[classification.Trait]bool{
		classification.TraitScopeClear: true, classification.TraitChangePointKnown: true,
		classification.TraitRecoverable: true, classification.TraitFocusedVerificationKnown: true,
	}
	traits := make([]classification.TraitObservation, 0, 19)
	for _, trait := range []classification.Trait{
		classification.TraitScopeClear, classification.TraitChangePointKnown, classification.TraitRecoverable,
		classification.TraitFocusedVerificationKnown, classification.TraitBoundedCapabilityRequest,
		classification.TraitArchitectureDecision, classification.TraitPublicContractChange,
		classification.TraitSchemaChange, classification.TraitDependencyChange,
		classification.TraitSecuritySensitive, classification.TraitDataSensitive,
		classification.TraitDeploymentChange, classification.TraitDomainUncertainty,
		classification.TraitRootCauseUncertain, classification.TraitMultipleResponsibilities,
		classification.TraitMultipleTickets, classification.TraitLongLivedDelegation,
		classification.TraitDestructiveMutation, classification.TraitCriticalRelease,
	} {
		value := classification.TraitFalse
		if positive[trait] {
			value = classification.TraitTrue
		}
		traits = append(traits, classification.TraitObservation{Trait: trait, Value: value})
	}
	return classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1,
		Traits:        traits,
		Resources:     []classification.Resource{classification.ResourceProject},
		Evidence: []classification.ProposalEvidence{
			{Kind: classification.EvidenceScope, Reference: "internal:scope", Digest: strings.Repeat("b", 64)},
			{Kind: classification.EvidenceChangePoint, Reference: "internal:change", Digest: strings.Repeat("c", 64)},
			{Kind: classification.EvidenceVerification, Reference: "internal:verification", Digest: strings.Repeat("d", 64)},
		},
	}
}

func setInternalTrait(proposal *classification.ClassificationProposal, trait classification.Trait, value classification.TraitValue) {
	for index := range proposal.Traits {
		if proposal.Traits[index].Trait == trait {
			proposal.Traits[index].Value = value
			return
		}
	}
}

func resignRevision(t *testing.T, record *revisionRecord) {
	t.Helper()
	record.Digest = ""
	record.Reply.RevisionDigest = ""
	digest, _, err := canonicaljson.Digest(*record)
	if err != nil {
		t.Fatalf("Digest(revision) error = %v", err)
	}
	record.Digest = digest
	record.Reply.RevisionDigest = digest
}

func writeInternalCanonical(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func assertInternalErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || ErrorCode(err) != code {
		t.Fatalf("error = %v (code %q), want code %q", err, ErrorCode(err), code)
	}
}
