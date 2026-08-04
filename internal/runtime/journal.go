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
	goruntime "runtime"

	"github.com/gofrs/flock"
	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

var runIDPattern = regexp.MustCompile(`^run-[0-9a-f]{32}$`)

const (
	maximumHeadBytes     = 4 << 10
	maximumRevisionBytes = 4 << 20
	maximumRunRevisions  = 1 << 20
)

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
	resourceLeasesRoot := filepath.Join(stateRoot, "resource-leases")
	if err := ensurePrivateDir(resourceLeasesRoot); err != nil {
		return nil, runtimeError("RUN_STATE_WRITE_FAILED", "create Resource Lease root", err)
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

// withResourceLeaseLock is the outer lock for Workflow mutations that may own
// a physical Worktree. Callers must acquire this lock before the per-Run lock.
func (value *journal) withResourceLeaseLock(action func() error) error {
	resourceRoot := filepath.Join(value.stateRoot, "resource-leases")
	if err := ensurePrivateDir(resourceRoot); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "create Resource Lease root", err)
	}
	lockPath := filepath.Join(resourceRoot, "LOCK")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "create Resource Lease lock", err)
	}
	if err := lockFile.Chmod(0o600); err != nil {
		_ = lockFile.Close()
		return runtimeError("RUN_STATE_WRITE_FAILED", "protect Resource Lease lock", err)
	}
	if err := lockFile.Close(); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "close Resource Lease lock", err)
	}
	guard := flock.New(lockPath)
	if err := guard.Lock(); err != nil {
		return runtimeError("RUN_STATE_WRITE_FAILED", "lock Resource Leases", err)
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
	rawHead, err := readLimitedFile(headPath, maximumHeadBytes)
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
	if head.SchemaVersion != headSchemaV1 || head.RunID != runID || head.Revision == 0 || head.Revision > maximumRunRevisions || !validDigest(head.RevisionDigest) {
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
		if revision > 1 {
			if transitionErr := validateRevisionTransition(current, loaded); transitionErr != nil {
				return revisionRecord{}, transitionErr
			}
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
	raw, err := readLimitedFile(path, maximumRevisionBytes)
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
	if record.SchemaVersion != revisionSchemaV1 || record.RunID != runID || record.Revision != revision || validateIdentifier(record.MessageID) != nil || validateIdentifier(record.IdempotencyKey) != nil || !validDigest(record.MessageDigest) || record.Event == "" {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid revision identity", nil)
	}
	if revision == 1 {
		if record.PredecessorDigest != "" {
			return runtimeError("RUN_STATE_REVISION_INVALID", "revision 1 has a predecessor", nil)
		}
	} else if !validDigest(record.PredecessorDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "missing predecessor digest", nil)
	}
	if record.Snapshot.SchemaVersion != snapshotSchemaV2 || record.Snapshot.RunID != runID || record.Snapshot.Revision != revision || record.ConfigurationDigest != record.Snapshot.ConfigurationDigest || record.Snapshot.Project.ConfigurationDigest != record.ConfigurationDigest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "snapshot identity mismatch", nil)
	}
	switch record.Snapshot.RequestMode {
	case classification.RequestModeDirect:
		if err := validateDirectState(record); err != nil {
			return err
		}
	case classification.RequestModeBounded:
		if err := validateBoundedState(record); err != nil {
			return err
		}
	case classification.RequestModeWorkflow:
		if err := validateWorkflowState(record); err != nil {
			return err
		}
	default:
		return runtimeError("RUN_STATE_REVISION_INVALID", "unsupported persisted Request Mode", nil)
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
	reusedDigest, err := value.writeImmutableRevision(record.RunID, record.Revision, rawRevision)
	if err != nil {
		return revisionRecord{}, err
	}
	committedDigest := digest
	if reusedDigest != "" {
		committedDigest = reusedDigest
	}
	head := headRecord{SchemaVersion: headSchemaV1, RunID: record.RunID, Revision: record.Revision, RevisionDigest: committedDigest}
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

func (value *journal) writeImmutableRevision(runID string, revision uint64, raw []byte) (string, error) {
	revisionsRoot := value.revisionsRoot(runID)
	if err := ensurePrivateDir(revisionsRoot); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "create revisions directory", err)
	}
	path := filepath.Join(revisionsRoot, revisionFileName(revision))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return reuseMatchingOrphan(path, raw, revisionsRoot)
		}
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "create immutable revision", err)
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "protect immutable revision", err)
	}
	if _, err := file.Write(raw); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "write immutable revision", err)
	}
	if err := file.Sync(); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "sync immutable revision", err)
	}
	if err := file.Close(); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "close immutable revision", err)
	}
	if err := syncDirectory(revisionsRoot); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "sync revisions directory", err)
	}
	remove = false
	return "", nil
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
	if err := replaceFile(temporaryPath, path); err != nil {
		return err
	}
	remove = false
	return syncDirectory(directory)
}

func syncDirectory(path string) error {
	if goruntime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func validateDirectState(record revisionRecord) error {
	snapshot := record.Snapshot
	if snapshot.RequestMode != classification.RequestModeDirect || snapshot.Status != RunReleased || snapshot.Classification.RequestMode != classification.RequestModeDirect || snapshot.ClassificationDigest == "" || !validDigest(snapshot.ClassificationDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Direct classification state", nil)
	}
	if err := validateIdentifier(snapshot.RequestID); err != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted request ID", err)
	}
	if snapshot.Project.Root == "" || !filepath.IsAbs(snapshot.Project.Root) || filepath.Clean(snapshot.Project.Root) != snapshot.Project.Root || !validDigest(snapshot.ConfigurationDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted project identity", nil)
	}
	if snapshot.Bounded != nil || snapshot.Workflow != nil || snapshot.Grants != nil || snapshot.Observations != nil || snapshot.ProcessedMessages == nil || uint64(len(snapshot.ProcessedMessages)) != record.Revision || snapshot.LifecycleBundles == nil || len(snapshot.LifecycleBundles) != 0 || snapshot.GrantIDs == nil || len(snapshot.GrantIDs) != 0 || snapshot.ResourceLeaseIDs == nil || len(snapshot.ResourceLeaseIDs) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Direct authority or message collections", nil)
	}
	if snapshot.Classification.EvidenceRequirements == nil || snapshot.Classification.EscalationReasons == nil || snapshot.Classification.WorkflowComplexity != nil || snapshot.Classification.CapabilitySelector != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Direct classification details", nil)
	}
	for index, message := range snapshot.ProcessedMessages {
		if err := validateIdentifier(message.IdempotencyKey); err != nil || !validDigest(message.ContentDigest) || message.Revision == 0 || message.Revision > record.Revision {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid processed message", err)
		}
		if index > 0 && snapshot.ProcessedMessages[index-1].IdempotencyKey >= message.IdempotencyKey {
			return runtimeError("RUN_STATE_REVISION_INVALID", "processed messages are not uniquely sorted", nil)
		}
	}
	currentMessage := snapshot.ProcessedMessages[len(snapshot.ProcessedMessages)-1]
	for _, message := range snapshot.ProcessedMessages {
		if message.Revision == record.Revision {
			currentMessage = message
			break
		}
	}
	if currentMessage.Revision != record.Revision || currentMessage.IdempotencyKey != record.IdempotencyKey || currentMessage.ContentDigest != record.MessageDigest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "current processed message does not match revision", nil)
	}
	if err := validateDirectReply(record); err != nil {
		return err
	}
	return nil
}

func validateBoundedState(record revisionRecord) error {
	snapshot := record.Snapshot
	if snapshot.RequestMode != classification.RequestModeBounded || snapshot.Classification.RequestMode != classification.RequestModeBounded || snapshot.ClassificationDigest == "" || !validDigest(snapshot.ClassificationDigest) || snapshot.Bounded == nil || snapshot.Workflow != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded classification state", nil)
	}
	if snapshot.Status != RunAwaitingCapability && snapshot.Status != RunReady && snapshot.Status != RunGranted && snapshot.Status != RunInFlight && snapshot.Status != RunFinished && snapshot.Status != RunPaused {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded status", nil)
	}
	if err := validateIdentifier(snapshot.RequestID); err != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted request ID", err)
	}
	if snapshot.Project.Root == "" || !filepath.IsAbs(snapshot.Project.Root) || filepath.Clean(snapshot.Project.Root) != snapshot.Project.Root || !validDigest(snapshot.ConfigurationDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted project identity", nil)
	}
	if _, err := catalog.ParseLocalID(snapshot.Bounded.HostID); err != nil || snapshot.ConfigurationDigest != snapshot.Bounded.ConfigurationDigest || !validDigest(snapshot.Bounded.CatalogDigest) || !validDigest(snapshot.Bounded.RegistryDigest) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded trusted input digests", nil)
	}
	if snapshot.ProcessedMessages == nil || uint64(len(snapshot.ProcessedMessages)) != record.Revision || snapshot.LifecycleBundles == nil || len(snapshot.LifecycleBundles) != 0 || snapshot.ResourceLeaseIDs == nil || len(snapshot.ResourceLeaseIDs) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded authority or message collections", nil)
	}
	if snapshot.Status == RunGranted || snapshot.Status == RunInFlight || snapshot.Status == RunFinished || snapshot.Status == RunPaused {
		if len(snapshot.Grants) != 1 || len(snapshot.GrantIDs) != 1 || snapshot.GrantIDs[0] != snapshot.Grants[0].ID {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Bounded Grant collection", nil)
		}
		if err := validatePersistedGrant(record, snapshot.Grants[0]); err != nil {
			return err
		}
	} else if len(snapshot.Grants) != 0 || len(snapshot.GrantIDs) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "unexpected Bounded Grant before issuance", nil)
	}
	if snapshot.Status == RunAwaitingCapability || snapshot.Status == RunReady || snapshot.Status == RunGranted || snapshot.Status == RunInFlight || snapshot.Status == RunPaused && len(snapshot.Observations) == 0 {
		if snapshot.Observations != nil && len(snapshot.Observations) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "unexpected persisted Bounded observation", nil)
		}
	}
	if snapshot.Status == RunFinished || snapshot.Status == RunPaused && len(snapshot.Observations) == 1 {
		if len(snapshot.Observations) != 1 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Bounded observation collection", nil)
		}
		if err := validatePersistedObservation(record, snapshot.Observations[0]); err != nil {
			return err
		}
		if snapshot.Status == RunFinished && snapshot.Observations[0].Outcome != ObservationSucceeded || snapshot.Status == RunPaused && snapshot.Observations[0].Outcome != ObservationFailed {
			return runtimeError("RUN_STATE_REVISION_INVALID", "Bounded status and observation outcome disagree", nil)
		}
	} else if snapshot.Status != RunPaused && len(snapshot.Observations) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "unexpected persisted Bounded observations", nil)
	} else if snapshot.Status == RunPaused && len(snapshot.Observations) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid paused Bounded observations", nil)
	}
	if snapshot.Classification.EvidenceRequirements == nil || snapshot.Classification.EscalationReasons == nil || snapshot.Classification.WorkflowComplexity != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded classification details", nil)
	}
	if snapshot.Classification.CapabilitySelector != nil {
		if err := validatePersistedSelector(*snapshot.Classification.CapabilitySelector, snapshot.Bounded.Input.TrustedRuleID); err != nil {
			return err
		}
	}
	if err := validatePersistedBoundedInput(snapshot.Bounded.Input); err != nil {
		return err
	}
	if snapshot.Status == RunAwaitingCapability && snapshot.Bounded.Selector != nil || snapshot.Status != RunAwaitingCapability && snapshot.Bounded.Selector == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "Bounded status and selector disagree", nil)
	}
	if snapshot.Bounded.Selector != nil {
		if err := validatePersistedSelector(*snapshot.Bounded.Selector, snapshot.Bounded.Input.TrustedRuleID); err != nil {
			return err
		}
	}
	for index, message := range snapshot.ProcessedMessages {
		if err := validateIdentifier(message.IdempotencyKey); err != nil || !validDigest(message.ContentDigest) || message.Revision == 0 || message.Revision > record.Revision {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid processed message", err)
		}
		if index > 0 && snapshot.ProcessedMessages[index-1].IdempotencyKey >= message.IdempotencyKey {
			return runtimeError("RUN_STATE_REVISION_INVALID", "processed messages are not uniquely sorted", nil)
		}
	}
	currentMessage := snapshot.ProcessedMessages[len(snapshot.ProcessedMessages)-1]
	for _, message := range snapshot.ProcessedMessages {
		if message.Revision == record.Revision {
			currentMessage = message
			break
		}
	}
	if currentMessage.Revision != record.Revision || currentMessage.IdempotencyKey != record.IdempotencyKey || currentMessage.ContentDigest != record.MessageDigest {
		return runtimeError("RUN_STATE_REVISION_INVALID", "current processed message does not match revision", nil)
	}
	return validateBoundedReply(record)
}

func validatePersistedBoundedInput(value BoundedInput) error {
	normalized, err := normalizeBoundedInput(&value)
	if err != nil || !equalStrings(normalized.RequestedEffects, value.RequestedEffects) || !equalStrings(normalized.RequestedResources, value.RequestedResources) || normalized.TerminationCondition != value.TerminationCondition || normalized.TrustedRuleID != value.TrustedRuleID {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Bounded input", err)
	}
	return nil
}

func validatePersistedSelector(value classification.CapabilitySelector, trustedRuleID string) error {
	if _, err := catalog.ParseQualifiedID(value.ProviderID); err != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted selector Provider", err)
	}
	if _, err := catalog.ParseLocalID(value.CapabilityID); err != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted selector Capability", err)
	}
	if value.Source == classification.SelectorUserIntent && trustedRuleID != "" || value.Source == classification.SelectorTrustedRule && trustedRuleID == "" {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted selector provenance", nil)
	}
	if value.Source != classification.SelectorUserIntent && value.Source != classification.SelectorTrustedRule {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted selector source", nil)
	}
	return nil
}

func validatePersistedGrant(record revisionRecord, grant admission.CapabilityGrant) error {
	if err := admission.ValidateGrant(grant); err != nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Capability Grant", err)
	}
	snapshot := record.Snapshot
	if snapshot.Bounded == nil || snapshot.Bounded.Selector == nil || grant.RunID != snapshot.RunID || grant.RequestID != snapshot.RequestID || grant.IssuedRevision > record.Revision || snapshot.Status == RunGranted && grant.IssuedRevision != record.Revision || grant.DeliverableID != snapshot.Bounded.Input.DeliverableID || grant.InputDigest != snapshot.Bounded.Input.InputDigest || grant.ProviderID != snapshot.Bounded.Selector.ProviderID || grant.CapabilityID != snapshot.Bounded.Selector.CapabilityID || grant.Binding.Host != snapshot.Bounded.HostID || grant.Executor.ID != snapshot.Bounded.Input.ExecutorID || grant.RegistryDigest != snapshot.Bounded.RegistryDigest || grant.CatalogDigest != snapshot.Bounded.CatalogDigest || grant.ParentGrantID != "" || !equalStrings(grant.Effects, snapshot.Bounded.Input.RequestedEffects) || !equalStrings(grant.Resources, snapshot.Bounded.Input.RequestedResources) || grant.TerminationCondition != snapshot.Bounded.Input.TerminationCondition {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted Grant exceeds Bounded request", nil)
	}
	return nil
}

func validatePersistedObservation(record revisionRecord, observation CapabilityObservation) error {
	normalized, err := normalizeCapabilityObservation(&observation)
	if err != nil || normalized.RawOutput != "" || normalized.GrantID != observation.GrantID || normalized.InvocationID != observation.InvocationID || normalized.ExecutorID != observation.ExecutorID || normalized.Outcome != observation.Outcome || !equalEvidenceReferences(normalized.EvidenceReferences, observation.EvidenceReferences) {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid persisted Capability observation", err)
	}
	if len(record.Snapshot.Grants) != 1 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted observation has no Grant", nil)
	}
	grant := record.Snapshot.Grants[0]
	if observation.GrantID != grant.ID || observation.InvocationID != grant.InvocationID || observation.ExecutorID != grant.Executor.ID {
		return runtimeError("RUN_STATE_REVISION_INVALID", "persisted observation exceeds authorized invocation", nil)
	}
	return nil
}

func equalEvidenceReferences(left, right []EvidenceReference) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateBoundedReply(record revisionRecord) error {
	reply := record.Reply
	if reply.Diagnostics == nil || reply.RecoveryActions == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded reply collections", nil)
	}
	snapshot := record.Snapshot
	switch snapshot.Status {
	case RunAwaitingCapability:
		if reply.Reason != "" || len(reply.RecoveryActions) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded selection reply recovery", nil)
		}
		if record.Event != "BOUNDED_AWAITING_CAPABILITY" || reply.Kind != ReplyCapabilitySelectionRequired || len(reply.Diagnostics) != 1 || !validSelectionDiagnostic(reply.Diagnostics[0].Code) {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded selection reply", nil)
		}
		return nil
	case RunGranted:
		if reply.Reason != "" || len(reply.RecoveryActions) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded Grant reply recovery", nil)
		}
		if record.Event != "BOUNDED_GRANT_ISSUED" || reply.Kind != ReplyGrantIssued || len(reply.Diagnostics) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded Grant reply", nil)
		}
		return nil
	case RunReady:
		if reply.Reason != "" || len(reply.RecoveryActions) != 0 || reply.Kind != ReplyModeDecided || len(reply.Diagnostics) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded ready reply", nil)
		}
		if record.Revision == 1 && record.Event != "BOUNDED_READY" || record.Revision > 1 && record.Event != "BOUNDED_CAPABILITY_SELECTED" {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded ready event", nil)
		}
		return nil
	case RunInFlight:
		if reply.Reason != "" || len(reply.RecoveryActions) != 0 || record.Event != "BOUNDED_DISPATCH_AUTHORIZED" || reply.Kind != ReplyDispatchAuthorized || len(reply.Diagnostics) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded dispatch authorization reply", nil)
		}
		return nil
	case RunFinished:
		if reply.Reason != "" || len(reply.RecoveryActions) != 0 || record.Event != "BOUNDED_CAPABILITY_FINISHED" || reply.Kind != ReplyFinished || len(reply.Diagnostics) != 0 || len(snapshot.Observations) != 1 || snapshot.Observations[0].Outcome != ObservationSucceeded {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded finished reply", nil)
		}
		return nil
	case RunPaused:
		if reply.Kind != ReplyPaused || len(reply.Diagnostics) != 0 {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded pause reply", nil)
		}
		if len(snapshot.Observations) == 1 {
			if record.Event != "BOUNDED_CAPABILITY_FAILED" || reply.Reason != ReasonModeEscalationRequired || len(reply.RecoveryActions) != 1 || reply.RecoveryActions[0] != RecoveryStartSuccessorRun || snapshot.Observations[0].Outcome != ObservationFailed {
				return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded failed observation pause", nil)
			}
			return nil
		}
		if reply.Reason == ReasonExecutionUncertain {
			if record.Event != "BOUNDED_EXECUTION_UNCERTAIN" || len(reply.RecoveryActions) != 1 || reply.RecoveryActions[0] != RecoveryReconcileInvocation {
				return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded uncertainty pause", nil)
			}
			return nil
		}
		if reply.Reason == ReasonModeEscalationRequired && len(reply.RecoveryActions) == 1 && reply.RecoveryActions[0] == RecoveryStartSuccessorRun && validBoundedEscalationEvent(record.Event) {
			return nil
		}
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Bounded escalation pause", nil)
	default:
		return runtimeError("RUN_STATE_REVISION_INVALID", "unsupported Bounded reply status", nil)
	}
}

func validBoundedEscalationEvent(value string) bool {
	switch value {
	case "BOUNDED_SCOPE_EXPANDED", "BOUNDED_ADDITIONAL_CAPABILITY_REQUIRED", "BOUNDED_REMEDIATION_REQUIRED", "BOUNDED_ARCHITECTURE_REQUIRED":
		return true
	default:
		return false
	}
}

func validSelectionDiagnostic(value string) bool {
	return value == "CAPABILITY_SELECTION_REQUIRED" || value == "CAPABILITY_NOT_VERIFIED" || value == "CAPABILITY_MODE_NOT_ALLOWED" || validProviderResolutionReason(value)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateDirectReply(record revisionRecord) error {
	reply := record.Reply
	if reply.Diagnostics == nil || reply.RecoveryActions == nil {
		return runtimeError("RUN_STATE_REVISION_INVALID", "nil Direct reply collections", nil)
	}
	if record.Revision == 1 {
		if record.Event != "DIRECT_RELEASED" || reply.Kind != ReplyModeDecided || reply.Reason != "" || len(reply.RecoveryActions) != 0 || len(reply.Diagnostics) != 3 || reply.Diagnostics[0].Code != DiagnosticDirectOutsideCapabilityAdmission || reply.Diagnostics[1].Code != DiagnosticHostToolCallsUncontrolled || reply.Diagnostics[2].Code != DiagnosticResourceLeaseNotApplicable {
			return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Direct release reply", nil)
		}
		return nil
	}
	if record.Event != "DIRECT_SCOPE_EXPANDED" || reply.Kind != ReplyPaused || reply.Reason != ReasonModeEscalationRequired || len(reply.RecoveryActions) != 1 || reply.RecoveryActions[0] != RecoveryStartSuccessorRun || len(reply.Diagnostics) != 0 {
		return runtimeError("RUN_STATE_REVISION_INVALID", "invalid Direct escalation reply", nil)
	}
	return nil
}

func reuseMatchingOrphan(path string, expected []byte, directory string) (string, error) {
	existing, err := readLimitedFile(path, maximumRevisionBytes)
	if err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "read orphan revision", err)
	}
	var existingRecord revisionRecord
	var expectedRecord revisionRecord
	if err := decodeStrict(existing, &existingRecord); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "decode orphan revision", err)
	}
	if err := decodeStrict(expected, &expectedRecord); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "decode candidate revision", err)
	}
	if err := validateRevision(existingRecord, expectedRecord.RunID, expectedRecord.Revision); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "validate orphan revision", err)
	}
	if !sameLogicalRevision(existingRecord, expectedRecord) {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "conflicting orphan revision", nil)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "open matching orphan revision", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "protect matching orphan revision", err)
	}
	if err := file.Sync(); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "sync matching orphan revision", err)
	}
	if err := syncDirectory(directory); err != nil {
		return "", runtimeError("RUN_STATE_WRITE_FAILED", "sync matching orphan directory", err)
	}
	return existingRecord.Digest, nil
}

func sameLogicalRevision(left, right revisionRecord) bool {
	if left.RunID != right.RunID || left.Revision != right.Revision || left.PredecessorDigest != right.PredecessorDigest || left.IdempotencyKey != right.IdempotencyKey || left.MessageDigest != right.MessageDigest || left.Event != right.Event || left.StateDigest != right.StateDigest || left.ConfigurationDigest != right.ConfigurationDigest {
		return false
	}
	leftReply := cloneReply(left.Reply)
	rightReply := cloneReply(right.Reply)
	leftReply.RevisionDigest = ""
	rightReply.RevisionDigest = ""
	leftDigest, _, leftErr := canonicaljson.Digest(leftReply)
	rightDigest, _, rightErr := canonicaljson.Digest(rightReply)
	return leftErr == nil && rightErr == nil && leftDigest == rightDigest
}

func readLimitedFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, fmt.Errorf("state file is not regular or exceeds %d bytes", maximum)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("state file exceeds %d bytes", maximum)
	}
	return raw, nil
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
