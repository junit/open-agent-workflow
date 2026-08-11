package coordinator

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const testWorkflowID = "workflow-0123456789abcdef0123456789abcdef"

func TestJournalCommitsAndReplaysWorkflowRevisions(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	j, err := newJournal(stateRoot)
	if err != nil {
		t.Fatalf("newJournal() error = %v", err)
	}
	first := testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1")
	committedFirst := commitRevision(t, j, first)
	if committedFirst.Digest == "" || committedFirst.Result.Digest == "" || committedFirst.Result.RevisionDigest != committedFirst.Digest {
		t.Fatalf("first revision was not pinned: %#v", committedFirst)
	}
	if committedFirst.Snapshot.ProcessedMessages[0].ResultDigest != committedFirst.Result.Digest {
		t.Fatalf("processed message does not pin Result: %#v", committedFirst.Snapshot.ProcessedMessages)
	}

	second := nextTestRevision(t, committedFirst, "message-2", "message-content-2", "key-2")
	committedSecond := commitRevision(t, j, second)
	if committedSecond.Revision != 2 || committedSecond.PredecessorDigest != committedFirst.Digest {
		t.Fatalf("second revision identity = %#v", committedSecond)
	}
	inspected, err := j.inspect(testWorkflowID)
	if err != nil {
		t.Fatalf("inspect() error = %v", err)
	}
	if !reflect.DeepEqual(inspected, committedSecond) {
		t.Fatalf("inspect() differs from committed revision\n got: %#v\nwant: %#v", inspected, committedSecond)
	}

	replayed, found, err := j.replay(testWorkflowID, "key-1", committedFirst.MessageDigest)
	if err != nil || !found {
		t.Fatalf("replay() = %#v, %v, found=%v", replayed, err, found)
	}
	if !replayed.Replayed || replayed.WorkflowID != testWorkflowID || replayed.Revision != 1 || replayed.Digest == "" {
		t.Fatalf("replayed Result = %#v", replayed)
	}
	if _, _, err := j.replay(testWorkflowID, "key-1", strings.Repeat("f", 64)); ErrorCode(err) != "WORKFLOW_IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("conflicting replay error = %v", err)
	}
	if _, found, err := j.replay(testWorkflowID, "missing-key", strings.Repeat("a", 64)); err != nil || found {
		t.Fatalf("missing replay = found %v, error %v", found, err)
	}
}

func TestJournalRejectsStaleRevisionWithoutMutation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	j, err := newJournal(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	committed := commitRevision(t, j, testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1"))
	headBefore, err := os.ReadFile(filepath.Join(j.workflowRoot(testWorkflowID), "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	stale := testRevision(t, testWorkflowID, 1, "message-stale", "message-content-stale", "key-stale")
	if err := j.withWorkflowLock(testWorkflowID, func() error { _, err := j.commit(stale); return err }); ErrorCode(err) != "WORKFLOW_REVISION_CONFLICT" {
		t.Fatalf("stale commit error = %v", err)
	}
	headAfter, err := os.ReadFile(filepath.Join(j.workflowRoot(testWorkflowID), "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(headBefore, headAfter) {
		t.Fatal("stale commit changed HEAD")
	}
	if committed.Revision != 1 {
		t.Fatalf("committed revision = %d", committed.Revision)
	}
}

func TestResultDigestPinsPreviousReplayLinks(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	j, err := newJournal(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	first := commitRevision(t, j, testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1"))
	second := commitRevision(t, j, nextTestRevision(t, first, "message-2", "message-content-2", "key-2"))
	forged := second.Result
	snapshot := *forged.Snapshot
	snapshot.ProcessedMessages = append([]ProcessedMessage{}, snapshot.ProcessedMessages...)
	snapshot.ProcessedMessages[0].ResultDigest = strings.Repeat("0", 64)
	forged.Snapshot = &snapshot
	if _, err := normalizeResult(forged); err == nil {
		t.Fatal("normalizeResult() accepted a forged historical Result pin")
	}
}

func TestJournalCommitsPersistedRejectedResult(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	j, err := newJournal(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	first := commitRevision(t, j, testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1"))
	rejected := nextTestRevision(t, first, "message-2", "message-content-2", "key-2")
	rejected.Result.Kind = ResultRejected
	rejected.Result.Diagnostics = []Diagnostic{{Code: "WORKFLOW_DENIED", Detail: "selection is not eligible"}}
	committed := commitRevision(t, j, rejected)
	if committed.Result.Kind != ResultRejected || committed.Result.Snapshot != nil || committed.Result.Dispatch != nil || committed.Result.Digest == "" {
		t.Fatalf("persisted REJECTED Result = %#v", committed.Result)
	}
	replayed, found, err := j.replay(testWorkflowID, "key-2", committed.MessageDigest)
	if err != nil || !found || !replayed.Replayed || replayed.Kind != ResultRejected || replayed.Snapshot != nil {
		t.Fatalf("replay(REJECTED) = %#v, found=%v, error=%v", replayed, found, err)
	}
}

func TestJournalIgnoresTornTemporaryFiles(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	j, err := newJournal(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	committed := commitRevision(t, j, testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1"))
	if err := os.WriteFile(filepath.Join(j.workflowRoot(testWorkflowID), ".HEAD-torn-1"), []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	inspected, err := j.inspect(testWorkflowID)
	if err != nil {
		t.Fatalf("inspect() with torn temporary file error = %v", err)
	}
	if inspected.Digest != committed.Digest {
		t.Fatalf("inspect() digest = %q, want %q", inspected.Digest, committed.Digest)
	}
}

func TestJournalRecoversMatchingOrphanAndRejectsConflict(t *testing.T) {
	t.Run("matching orphan", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		j, err := newJournal(stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		candidate := testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1")
		committed := commitRevision(t, j, candidate)
		headPath := filepath.Join(j.workflowRoot(testWorkflowID), "HEAD")
		if err := os.Remove(headPath); err != nil {
			t.Fatal(err)
		}
		candidate.MessageID = "message-retry"
		recovered := commitRevision(t, j, candidate)
		if !reflect.DeepEqual(recovered, committed) {
			t.Fatalf("recovered orphan differs\n got: %#v\nwant: %#v", recovered, committed)
		}
		if _, err := os.Stat(headPath); err != nil {
			t.Fatalf("matching orphan did not restore HEAD: %v", err)
		}
	})

	t.Run("conflicting orphan", func(t *testing.T) {
		stateRoot := filepath.Join(t.TempDir(), "state")
		j, err := newJournal(stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		committed := commitRevision(t, j, testRevision(t, testWorkflowID, 1, "message-1", "message-content-1", "key-1"))
		revisionPath := filepath.Join(j.revisionsRoot(testWorkflowID), revisionFileName(1))
		before, err := os.ReadFile(revisionPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(j.workflowRoot(testWorkflowID), "HEAD")); err != nil {
			t.Fatal(err)
		}
		conflict := testRevision(t, testWorkflowID, 1, "message-conflict", "different-content", "key-1")
		if err := j.withWorkflowLock(testWorkflowID, func() error { _, err := j.commit(conflict); return err }); ErrorCode(err) != "WORKFLOW_STATE_WRITE_FAILED" {
			t.Fatalf("conflicting orphan error = %v", err)
		}
		after, err := os.ReadFile(revisionPath)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(after, before) || committed.Digest == "" {
			t.Fatal("conflicting orphan changed immutable revision")
		}
	})
}

func TestOldStateJournalRejectsOldRuntimeStateWithoutTouchingIt(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	runsRoot := filepath.Join(stateRoot, "runs")
	sentinel := filepath.Join(runsRoot, "run-0123456789abcdef0123456789abcdef", "HEAD")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newJournal(stateRoot); ErrorCode(err) != "WORKFLOW_STATE_UNSUPPORTED" {
		t.Fatalf("newJournal() error = %v", err)
	}
	content, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "legacy" {
		t.Fatalf("legacy sentinel changed to %q", content)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "records")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new state records directory exists after legacy rejection: %v", err)
	}
}

func TestJournalRejectsUnsafeStateRoots(t *testing.T) {
	if _, err := newJournal("relative-state"); ErrorCode(err) != "WORKFLOW_STATE_ROOT_INVALID" {
		t.Fatalf("relative state root error = %v", err)
	}
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "state-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := newJournal(link); ErrorCode(err) != "WORKFLOW_STATE_ROOT_INVALID" {
		t.Fatalf("symlinked state root error = %v", err)
	}
}

func TestJournalCrossProcessWorkflowLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process lock helper uses POSIX polling semantics")
	}
	stateRoot := filepath.Join(t.TempDir(), "state")
	j, err := newJournal(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	markerRoot := t.TempDir()
	startedPath := filepath.Join(markerRoot, "started")
	acquiredPath := filepath.Join(markerRoot, "acquired")
	var cmd *exec.Cmd
	if err := j.withWorkflowLock(testWorkflowID, func() error {
		cmd = exec.Command(os.Args[0], "-test.run=^TestJournalLockHelper$", "--")
		cmd.Env = append(os.Environ(),
			"OAW_JOURNAL_LOCK_HELPER=1",
			"OAW_JOURNAL_STATE_ROOT="+stateRoot,
			"OAW_JOURNAL_WORKFLOW_ID="+testWorkflowID,
			"OAW_JOURNAL_STARTED="+startedPath,
			"OAW_JOURNAL_ACQUIRED="+acquiredPath,
		)
		if err := cmd.Start(); err != nil {
			return err
		}
		if err := waitForTestFile(startedPath, time.Second); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(acquiredPath); !errors.Is(err, os.ErrNotExist) {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return errors.New("child acquired Workflow lock while parent held it")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := assertLockHelperProcess(cmd, acquiredPath); err != nil {
		t.Fatal(err)
	}
}

func TestJournalLockHelper(t *testing.T) {
	if os.Getenv("OAW_JOURNAL_LOCK_HELPER") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("OAW_JOURNAL_STARTED"), []byte("started"), 0o600); err != nil {
		t.Fatal(err)
	}
	j, err := newJournal(os.Getenv("OAW_JOURNAL_STATE_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	if err := j.withWorkflowLock(os.Getenv("OAW_JOURNAL_WORKFLOW_ID"), func() error {
		return os.WriteFile(os.Getenv("OAW_JOURNAL_ACQUIRED"), []byte("acquired"), 0o600)
	}); err != nil {
		t.Fatal(err)
	}
}

func assertLockHelperProcess(cmd *exec.Cmd, acquiredPath string) error {
	if err := cmd.Wait(); err != nil {
		return err
	}
	content, err := os.ReadFile(acquiredPath)
	if err != nil {
		return err
	}
	if string(content) != "acquired" {
		return errors.New("child did not acquire Workflow lock after parent release")
	}
	return nil
}

func commitRevision(t *testing.T, j *journal, record revisionRecord) revisionRecord {
	t.Helper()
	var committed revisionRecord
	if err := j.withWorkflowLock(record.WorkflowID, func() error {
		var err error
		committed, err = j.commit(record)
		return err
	}); err != nil {
		t.Fatalf("commit() error = %v", err)
	}
	return committed
}

func testRevision(t *testing.T, workflowID string, revision uint64, messageID, content, key string) revisionRecord {
	t.Helper()
	digest := contentDigest(content)
	decision, err := classification.Classify(nil, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	start := startTestCommand(t, "journal-fixture")
	options := startTestOptions(t, filepath.Join(t.TempDir(), "state"), nil)
	bundle := compiledStartTestBundle(compilationRequestFromStart(options, decision, *start.Start))
	snapshot := Snapshot{
		SchemaVersion: WorkflowSnapshotSchemaV2, WorkflowID: workflowID, RequestID: "request-1", DeliverableID: "deliverable-1", Revision: revision,
		Status: StatusReady, Classification: decision, Bundles: []core.LifecycleBundle{bundle}, ActiveGeneration: bundle.Generation,
		Cursor: firstStartTestCursor(t, bundle.Graph), ActiveTicket: "",
		GrantHistory: []admission.CapabilityGrant{}, UserAuthorizations: []admission.UserAuthorization{}, InvocationAttestations: []admission.ExplicitInvocationAttestation{},
		GateAttestations: []GateAttestation{}, Receipts: []host.InvocationReceipt{}, ResourceLeases: []ResourceLease{}, LastStableBoundary: "",
		ProcessedMessages: []ProcessedMessage{{IdempotencyKey: key, ContentDigest: digest, Revision: revision}}, ProjectionLag: []ProjectionLag{},
	}
	return revisionRecord{
		SchemaVersion: WorkflowRevisionSchemaV2, WorkflowID: workflowID, Revision: revision, MessageID: messageID, IdempotencyKey: key,
		MessageDigest: digest, Event: "WORKFLOW_TEST", Snapshot: snapshot,
		Result: Result{SchemaVersion: WorkflowResultSchemaV2, Kind: ResultState, WorkflowID: workflowID, Revision: revision, Diagnostics: []Diagnostic{}, Replayed: false},
	}
}

func nextTestRevision(t *testing.T, previous revisionRecord, messageID, content, key string) revisionRecord {
	t.Helper()
	next := testRevision(t, previous.WorkflowID, previous.Revision+1, messageID, content, key)
	next.PredecessorDigest = previous.Digest
	next.Snapshot.ProcessedMessages = append([]ProcessedMessage{}, previous.Snapshot.ProcessedMessages...)
	next.Snapshot.ProcessedMessages = append(next.Snapshot.ProcessedMessages, ProcessedMessage{IdempotencyKey: key, ContentDigest: contentDigest(content), Revision: next.Revision})
	sort.Slice(next.Snapshot.ProcessedMessages, func(left, right int) bool {
		return next.Snapshot.ProcessedMessages[left].IdempotencyKey < next.Snapshot.ProcessedMessages[right].IdempotencyKey
	})
	return next
}

func contentDigest(value string) string {
	digest, _, _ := canonicaljson.Digest(struct {
		Content string `json:"content"`
	}{Content: value})
	return digest
}

func waitForTestFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return errors.New("timed out waiting for test marker")
}
