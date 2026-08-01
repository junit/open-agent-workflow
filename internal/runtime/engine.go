package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

const maximumIdentifierLength = 256

type Engine struct {
	rules   classification.ClassificationRules
	journal *journal
}

func NewEngine(options Options) (*Engine, error) {
	journal, err := newJournal(options.StateRoot)
	if err != nil {
		return nil, err
	}
	return &Engine{rules: cloneRules(options.Rules), journal: journal}, nil
}

func (engine *Engine) Exchange(frame RunFrame) (RunReply, error) {
	if err := validateCommonFrame(frame); err != nil {
		return RunReply{}, err
	}
	switch frame.Kind {
	case FrameStart:
		return engine.start(frame)
	case FrameInspect:
		return engine.inspect(frame)
	case FrameContinue:
		return engine.continueRun(frame)
	default:
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "unknown frame kind", nil)
	}
}

func (engine *Engine) start(frame RunFrame) (RunReply, error) {
	if frame.Start == nil || frame.Continue != nil || frame.RunID != "" || frame.ExpectedRevision != 0 {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "START payload shape is invalid", nil)
	}
	if frame.Start.Proposal == nil {
		return RunReply{}, runtimeError("CLASSIFICATION_REQUIRED", "START requires a classification proposal", nil)
	}
	if err := validateIdentifier(frame.Start.RequestID); err != nil {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "invalid request ID", err)
	}
	project, err := normalizeProject(frame.Start.Project)
	if err != nil {
		return RunReply{}, err
	}
	proposal, err := normalizeProposal(frame.Start.Proposal)
	if err != nil {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "invalid classification proposal", err)
	}
	decision, err := classification.Classify(proposal, engine.rules)
	if err != nil {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "invalid classification rules", err)
	}
	if decision.RequestMode != classification.RequestModeDirect {
		return RunReply{}, runtimeError("REQUEST_MODE_NOT_IMPLEMENTED", fmt.Sprintf("request mode %s is not implemented", decision.RequestMode), nil)
	}
	normalizedStart := StartInput{RequestID: frame.Start.RequestID, Project: project, Proposal: cloneProposal(proposal)}
	messageDigest, err := frameContentDigest(RunFrame{
		SchemaVersion: RuntimeSchemaV1,
		Kind:          FrameStart,
		Start:         &normalizedStart,
	})
	if err != nil {
		return RunReply{}, err
	}
	runID := deriveRunID(frame.IdempotencyKey)
	var reply RunReply
	err = engine.journal.withRunLock(runID, func() error {
		current, loadErr := engine.journal.loadCommitted(runID)
		if loadErr == nil {
			replayed, found, replayErr := engine.replay(current.Snapshot, frame.IdempotencyKey, messageDigest)
			if replayErr != nil {
				return replayErr
			}
			if found {
				reply = replayed
				return nil
			}
			return runtimeError("IDEMPOTENCY_KEY_REUSED", "derived Run already exists for another message", nil)
		}
		if ErrorCode(loadErr) != "RUN_NOT_FOUND" {
			return loadErr
		}

		snapshot := RunSnapshot{
			SchemaVersion:        snapshotSchemaV1,
			RunID:                runID,
			RequestID:            frame.Start.RequestID,
			Project:              project,
			Revision:             1,
			RequestMode:          classification.RequestModeDirect,
			Status:               RunReleased,
			Classification:       cloneDecision(decision),
			ClassificationDigest: decision.Digest(),
			ConfigurationDigest:  project.ConfigurationDigest,
			ProcessedMessages: []ProcessedMessage{{
				IdempotencyKey: frame.IdempotencyKey,
				ContentDigest:  messageDigest,
				Revision:       1,
			}},
			LifecycleBundles: []string{},
			GrantIDs:         []string{},
			ResourceLeaseIDs: []string{},
		}
		candidateReply := directReleaseReply(snapshot)
		committed, commitErr := engine.journal.commit(revisionRecord{
			SchemaVersion:  revisionSchemaV1,
			RunID:          runID,
			Revision:       1,
			MessageID:      frame.MessageID,
			IdempotencyKey: frame.IdempotencyKey,
			MessageDigest:  messageDigest,
			Event:          "DIRECT_RELEASED",
			Snapshot:       snapshot,
			Reply:          candidateReply,
		})
		if commitErr != nil {
			return commitErr
		}
		reply = cloneReply(committed.Reply)
		return nil
	})
	if err != nil {
		return RunReply{}, err
	}
	return cloneReply(reply), nil
}

func (engine *Engine) inspect(frame RunFrame) (RunReply, error) {
	if frame.Start != nil || frame.Continue != nil || frame.ExpectedRevision != 0 || !runIDPattern.MatchString(frame.RunID) {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "INSPECT payload shape is invalid", nil)
	}
	committed, err := engine.journal.inspect(frame.RunID)
	if err != nil {
		return RunReply{}, err
	}
	reply := RunReply{
		SchemaVersion:   RuntimeSchemaV1,
		Kind:            ReplyStateSnapshot,
		RunID:           committed.RunID,
		Revision:        committed.Revision,
		RevisionDigest:  committed.Digest,
		Snapshot:        cloneSnapshot(committed.Snapshot),
		Diagnostics:     []Diagnostic{},
		RecoveryActions: []string{},
	}
	return cloneReply(reply), nil
}

func (engine *Engine) continueRun(frame RunFrame) (RunReply, error) {
	if frame.Start != nil || frame.Continue == nil || !runIDPattern.MatchString(frame.RunID) || frame.ExpectedRevision == 0 || frame.Continue.Signal != SignalScopeExpanded {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "CONTINUE payload shape is invalid", nil)
	}
	messageDigest, err := frameContentDigest(RunFrame{
		SchemaVersion:    RuntimeSchemaV1,
		Kind:             FrameContinue,
		RunID:            frame.RunID,
		ExpectedRevision: frame.ExpectedRevision,
		Continue:         &ContinueInput{Signal: frame.Continue.Signal},
	})
	if err != nil {
		return RunReply{}, err
	}
	var reply RunReply
	err = engine.journal.withRunLock(frame.RunID, func() error {
		current, loadErr := engine.journal.loadCommitted(frame.RunID)
		if loadErr != nil {
			return loadErr
		}
		replayed, found, replayErr := engine.replay(current.Snapshot, frame.IdempotencyKey, messageDigest)
		if replayErr != nil {
			return replayErr
		}
		if found {
			reply = replayed
			return nil
		}
		if frame.ExpectedRevision != current.Revision {
			return runtimeError("RUN_REVISION_CONFLICT", fmt.Sprintf("expected revision %d, current revision %d", frame.ExpectedRevision, current.Revision), nil)
		}
		if current.Snapshot.RequestMode != classification.RequestModeDirect || current.Snapshot.Status != RunReleased {
			return runtimeError("RUN_STATE_REVISION_INVALID", "scope expansion requires a released Direct run", nil)
		}
		nextRevision := current.Revision + 1
		snapshot := cloneSnapshot(current.Snapshot)
		snapshot.Revision = nextRevision
		snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{
			IdempotencyKey: frame.IdempotencyKey,
			ContentDigest:  messageDigest,
			Revision:       nextRevision,
		})
		sort.Slice(snapshot.ProcessedMessages, func(i, j int) bool {
			return snapshot.ProcessedMessages[i].IdempotencyKey < snapshot.ProcessedMessages[j].IdempotencyKey
		})
		candidateReply := RunReply{
			SchemaVersion:   RuntimeSchemaV1,
			Kind:            ReplyPaused,
			RunID:           frame.RunID,
			Revision:        nextRevision,
			Snapshot:        cloneSnapshot(snapshot),
			Diagnostics:     []Diagnostic{},
			Reason:          ReasonModeEscalationRequired,
			RecoveryActions: []string{RecoveryStartSuccessorRun},
		}
		committed, commitErr := engine.journal.commit(revisionRecord{
			SchemaVersion:     revisionSchemaV1,
			RunID:             frame.RunID,
			Revision:          nextRevision,
			PredecessorDigest: current.Digest,
			MessageID:         frame.MessageID,
			IdempotencyKey:    frame.IdempotencyKey,
			MessageDigest:     messageDigest,
			Event:             "DIRECT_SCOPE_EXPANDED",
			Snapshot:          snapshot,
			Reply:             candidateReply,
		})
		if commitErr != nil {
			return commitErr
		}
		reply = cloneReply(committed.Reply)
		return nil
	})
	if err != nil {
		return RunReply{}, err
	}
	return cloneReply(reply), nil
}

func (engine *Engine) replay(snapshot RunSnapshot, key, digest string) (RunReply, bool, error) {
	for _, message := range snapshot.ProcessedMessages {
		if message.IdempotencyKey != key {
			continue
		}
		if message.ContentDigest != digest {
			return RunReply{}, false, runtimeError("IDEMPOTENCY_KEY_REUSED", "idempotency key content changed", nil)
		}
		revision, err := engine.journal.loadRevision(snapshot.RunID, message.Revision)
		if err != nil {
			return RunReply{}, false, err
		}
		return cloneReply(revision.Reply), true, nil
	}
	return RunReply{}, false, nil
}

func directReleaseReply(snapshot RunSnapshot) RunReply {
	return RunReply{
		SchemaVersion: RuntimeSchemaV1,
		Kind:          ReplyModeDecided,
		RunID:         snapshot.RunID,
		Revision:      snapshot.Revision,
		Snapshot:      cloneSnapshot(snapshot),
		Diagnostics: []Diagnostic{
			{Code: DiagnosticDirectOutsideCapabilityAdmission, Message: "Direct execution is outside Capability admission."},
			{Code: DiagnosticHostToolCallsUncontrolled, Message: "OAW does not control subsequent Host tool calls."},
			{Code: DiagnosticResourceLeaseNotApplicable, Message: "OAW Resource Lease guarantees do not apply to Direct execution."},
		},
		RecoveryActions: []string{},
	}
}

func validateCommonFrame(frame RunFrame) error {
	if frame.SchemaVersion != RuntimeSchemaV1 {
		return runtimeError("RUNTIME_SCHEMA_UNSUPPORTED", fmt.Sprintf("unsupported schema %q", frame.SchemaVersion), nil)
	}
	if err := validateIdentifier(frame.MessageID); err != nil {
		return runtimeError("RUNTIME_FRAME_INVALID", "invalid message ID", err)
	}
	if err := validateIdentifier(frame.IdempotencyKey); err != nil {
		return runtimeError("RUNTIME_FRAME_INVALID", "invalid idempotency key", err)
	}
	return nil
}

func validateIdentifier(value string) error {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximumIdentifierLength {
		return errors.New("identifier is empty, invalid UTF-8, or too long")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("identifier contains a control character")
		}
	}
	return nil
}

func normalizeProject(value ProjectIdentity) (ProjectIdentity, error) {
	if value.Root == "" || !filepath.IsAbs(value.Root) || filepath.Clean(value.Root) != value.Root || !validDigest(value.ConfigurationDigest) {
		return ProjectIdentity{}, runtimeError("PROJECT_IDENTITY_INVALID", "project root or configuration digest is invalid", nil)
	}
	physical, err := filepath.EvalSymlinks(value.Root)
	if err != nil {
		return ProjectIdentity{}, runtimeError("PROJECT_IDENTITY_INVALID", "resolve physical project root", err)
	}
	physical = filepath.Clean(physical)
	info, err := os.Stat(physical)
	if err != nil || !info.IsDir() {
		return ProjectIdentity{}, runtimeError("PROJECT_IDENTITY_INVALID", "project root is not an existing directory", err)
	}
	return ProjectIdentity{Root: physical, ConfigurationDigest: value.ConfigurationDigest}, nil
}

func normalizeProposal(value *classification.ClassificationProposal) (*classification.ClassificationProposal, error) {
	raw, err := canonicaljson.Marshal(value)
	if err != nil {
		return nil, err
	}
	normalized, err := classification.DecodeProposal(raw)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func frameContentDigest(frame RunFrame) (string, error) {
	record := struct {
		SchemaVersion    string         `json:"schema_version"`
		Kind             FrameKind      `json:"kind"`
		RunID            string         `json:"run_id,omitempty"`
		ExpectedRevision uint64         `json:"expected_revision,omitempty"`
		Start            *StartInput    `json:"start,omitempty"`
		Continue         *ContinueInput `json:"continue,omitempty"`
	}{frame.SchemaVersion, frame.Kind, frame.RunID, frame.ExpectedRevision, frame.Start, frame.Continue}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return "", runtimeError("RUNTIME_FRAME_INVALID", "digest frame content", err)
	}
	return digest, nil
}

func deriveRunID(idempotencyKey string) string {
	digest := sha256.Sum256([]byte(idempotencyKey))
	return "run-" + hex.EncodeToString(digest[:16])
}

func cloneRules(value classification.ClassificationRules) classification.ClassificationRules {
	value.User.ProtectedResources = append([]classification.Resource{}, value.User.ProtectedResources...)
	value.User.RequiredEvidence = append([]classification.EvidenceKind{}, value.User.RequiredEvidence...)
	value.Project.ProtectedResources = append([]classification.Resource{}, value.Project.ProtectedResources...)
	value.Project.RequiredEvidence = append([]classification.EvidenceKind{}, value.Project.RequiredEvidence...)
	return value
}
