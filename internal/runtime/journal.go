package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gofrs/flock"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

var runIDPattern = regexp.MustCompile(`^run-[0-9a-f]{32}$`)

type headRecord struct {
	SchemaVersion  string `json:"schema_version"`
	RunID          string `json:"run_id"`
	Revision       uint64 `json:"revision"`
	RevisionDigest string `json:"revision_digest"`
}

type revisionRecord struct {
	SchemaVersion       string      `json:"schema_version"`
	RunID               string      `json:"run_id"`
	Revision            uint64      `json:"revision"`
	PredecessorDigest   string      `json:"predecessor_digest"`
	MessageID           string      `json:"message_id"`
	IdempotencyKey      string      `json:"idempotency_key"`
	MessageDigest       string      `json:"message_digest"`
	Event               string      `json:"event"`
	Snapshot            RunSnapshot `json:"snapshot"`
	StateDigest         string      `json:"state_digest"`
	ConfigurationDigest string      `json:"configuration_digest"`
	Reply               RunReply    `json:"reply"`
	Digest              string      `json:"digest"`
}

type journal struct {
	stateRoot string
}

func newJournal(stateRoot string) (*journal, error) {
	if stateRoot == "" || !filepath.IsAbs(stateRoot) || filepath.Clean(stateRoot) != stateRoot {
		return nil, runtimeError("RUNTIME_STATE_ROOT_INVALID", "state root must be a clean absolute path", nil)
	}
	if err := ensurePrivateDir(stateRoot); err != nil {
		return nil, runtimeError("RUN_STATE_WRITE_FAILED", "create state root", err)
	}
	runsRoot := filepath.Join(stateRoot, "runs")
	if err := ensurePrivateDir(runsRoot); err != nil {
		return nil, runtimeError("RUN_STATE_WRITE_FAILED", "create runs root", err)
	}
	return &journal{stateRoot: stateRoot}, nil
}

func (value *journal) withRunLock(runID string, action func() error) error {
	if !runIDPattern.MatchString(runID) {
		return runtimeError("RUNTIME_FRAME_INVALID", "invalid run ID", nil)
	}
	runRoot := value.runRoot(runID)
	if err := ensurePrivateDir(runRoot); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "create run directory", err)
	}
	lockPath := filepath.Join(runRoot, "LOCK")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "create run lock", err)
	}
	if err := lockFile.Chmod(0o600); err != nil {
		_ = lockFile.Close()
		return runtimeError("RUN_STATE_WRITE_FAILED", "protect run lock", err)
	}
	if err := lockFile.Close(); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "close run lock", err)
	}
	guard := flock.New(lockPath)
	if err := guard.Lock(); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "lock run", err)
	}
	defer func() {
		_ = guard.Unlock()
		_ = guard.Close()
	}()
	return action()
}

func (value *journal) inspect(runID string) (revisionRecord, error) {
	if !runIDPattern.MatchString(runID) {
		return revisionRecord{}, runtimeError("RUNTIME_FRAME_INVALID", "invalid run ID", nil)
	}
	return value.loadCommitted(runID)
}

func (value *journal) loadCommitted(runID string) (revisionRecord, error) {
	headPath := filepath.Join(value.runRoot(runID), "HEAD")
	rawHead, err := os.ReadFile(headPath)
	if errors.Is(err, os.ErrNotExist) {
		return revisionRecord{}, runtimeError("RUN_NOT_FOUND", runID, err)
	}
	if err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_HEAD_INVALID", "read HEAD", err)
	}
	var head headRecord
	if err := decodeStrict(rawHead, &head); err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_HEAD_INVALID", "decode HEAD", err)
	}
	if head.SchemaVersion != headSchemaV1 || head.RunID != runID || head.Revision == 0 || !validDigest(head.RevisionDigest) {
		return revisionRecord{}, runtimeError("RUN_STATE_HEAD_INVALID", "invalid HEAD identity", nil)
	}

	var current revisionRecord
	predecessorDigest := ""
	for revision := uint64(1); revision <= head.Revision; revision++ {
		loaded, loadErr := value.loadRevision(runID, revision)
		if loadErr != nil {
			return revisionRecord{}, loadErr
		}
		if loaded.PredecessorDigest != predecessorDigest {
			return revisionRecord{}, runtimeError("RUN_STATE_REVISION_INVALID", "invalid predecessor chain", nil)
		}
		predecessorDigest = loaded.Digest
		current = loaded
	}
	if current.Digest != head.RevisionDigest {
		return revisionRecord{}, runtimeError("RUN_STATE_DIGEST_MISMATCH", "HEAD revision digest mismatch", nil)
	}
	return current, nil
}

func (value *journal) loadRevision(runID string, revision uint64) (revisionRecord, error) {
	path := filepath.Join(value.revisionsRoot(runID), revisionFileName(revision))
	raw, err := os.ReadFile(path)
	if err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_REVISION_INVALID", "read immutable revision", err)
	}
	var record revisionRecord
	if err := decodeStrict(raw, &record); err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_REVISION_INVALID", "decode immutable revision", err)
	}
	if err := validateRevision(record, runID, revision); err != nil {
		return revisionRecord{}, err
	}
	return record, nil
}

func validateRevision(record revisionRecord, runID string, revision uint64) error {
	if record.SchemaVersion != revisionSchemaV1 || record.RunID != runID || record.Revision != revision || record.MessageID == "" || record.IdempotencyKey == "" || !validDigest(record.MessageDigest) || record.Event == "" {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid revision identity", nil)
	}
	if revision == 1 {
		if record.PredecessorDigest != "" {
			return runtimeError("RUN_STATE_REVISION_INVALID", "revision 1 has a predecessor", nil)
		}
	} else if !validDigest(record.PredecessorDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "missing predecessor digest", nil)
	}
	if record.Snapshot.SchemaVersion != snapshotSchemaV1 || record.Snapshot.RunID != runID || record.Snapshot.Revision != revision || record.ConfigurationDigest != record.Snapshot.ConfigurationDigest || record.Snapshot.Project.ConfigurationDigest != record.ConfigurationDigest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "snapshot identity mismatch", nil)
	}
	stateDigest, _, err := canonicaljson.Digest(record.Snapshot)
	if err != nil || stateDigest != record.StateDigest {
		return runtimeError("RUN_STATE_DIGEST_MISMATCH", "state digest mismatch", err)
	}
	if record.Reply.SchemaVersion != RuntimeSchemaV1 || record.Reply.RunID != runID || record.Reply.Revision != revision || record.Reply.RevisionDigest != record.Digest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "reply identity mismatch", nil)
	}
	replyStateDigest, _, err := canonicaljson.Digest(record.Reply.Snapshot)
	if err != nil || replyStateDigest != record.StateDigest {
		return runtimeError("RUN_STATE_DIGEST_MISMATCH", "reply state mismatch", err)
	}
	storedDigest := record.Digest
	if !validDigest(storedDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid revision digest", nil)
	}
	record.Digest = ""
	record.Reply.RevisionDigest = ""
	digest, _, err := canonicaljson.Digest(record)
	if err != nil || digest != storedDigest {
		return runtimeError("RUN_STATE_DIGEST_MISMATCH", "revision digest mismatch", err)
	}
	return nil
}

func (value *journal) commit(record revisionRecord) (revisionRecord, error) {
	stateDigest, _, err := canonicaljson.Digest(record.Snapshot)
	if err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_WRITE_FAILED", "digest state", err)
	}
	record.StateDigest = stateDigest
	record.ConfigurationDigest = record.Snapshot.ConfigurationDigest
	record.Reply.Snapshot = cloneSnapshot(record.Snapshot)
	record.Digest = ""
	record.Reply.RevisionDigest = ""
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_WRITE_FAILED", "digest revision", err)
	}
	record.Digest = digest
	record.Reply.RevisionDigest = digest
	rawRevision, err := canonicaljson.Marshal(record)
	if err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_WRITE_FAILED", "encode revision", err)
	}
	if err := value.writeImmutableRevision(record.RunID, record.Revision, rawRevision); err != nil {
		return revisionRecord{}, err
	}
	head := headRecord{SchemaVersion: headSchemaV1, RunID: record.RunID, Revision: record.Revision, RevisionDigest: digest}
	rawHead, err := canonicaljson.Marshal(head)
	if err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_WRITE_FAILED", "encode HEAD", err)
	}
	if err := atomicWriteFile(filepath.Join(value.runRoot(record.RunID), "HEAD"), rawHead, 0o600); err != nil {
		return revisionRecord{}, runtimeError("RUN_STATE_WRITE_FAILED", "replace HEAD", err)
	}
	committed, err := value.loadRevision(record.RunID, record.Revision)
	if err != nil {
		return revisionRecord{}, err
	}
	return committed, nil
}

func (value *journal) writeImmutableRevision(runID string, revision uint64, raw []byte) error {
	revisionsRoot := value.revisionsRoot(runID)
	if err := ensurePrivateDir(revisionsRoot); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "create revisions directory", err)
	}
	path := filepath.Join(revisionsRoot, revisionFileName(revision))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "create immutable revision", err)
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "protect immutable revision", err)
	}
	if _, err := file.Write(raw); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "write immutable revision", err)
	}
	if err := file.Sync(); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "sync immutable revision", err)
	}
	if err := file.Close(); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "close immutable revision", err)
	}
	if err := syncDirectory(revisionsRoot); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "sync revisions directory", err)
	}
	remove = false
	return nil
}

func (value *journal) runRoot(runID string) string {
	return filepath.Join(value.stateRoot, "runs", runID)
}

func (value *journal) revisionsRoot(runID string) string {
	return filepath.Join(value.runRoot(runID), "revisions")
}

func revisionFileName(revision uint64) string {
	return fmt.Sprintf("%020d.json", revision)
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700)
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".HEAD-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	remove := true
	defer func() {
		_ = temporary.Close()
		if remove {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	remove = false
	return syncDirectory(directory)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || (character > '9' && character < 'a') || character > 'f' {
			return false
		}
	}
	return true
}
