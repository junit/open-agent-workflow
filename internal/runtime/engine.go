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
	bounded BoundedOptions
	journal *journal
}

func NewEngine(options Options) (*Engine, error) {
	journal, err := newJournal(options.StateRoot)
	if err != nil {
		return nil, err
	}
	return &Engine{rules: cloneRules(options.Rules), bounded: cloneBoundedOptions(options.Bounded), journal: journal}, nil
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
	var boundedInput *BoundedInput
	if proposalRequestsBounded(*proposal) {
		normalized, normalizeErr := normalizeBoundedInput(frame.Start.Bounded)
		if normalizeErr != nil {
			return RunReply{}, normalizeErr
		}
		boundedInput = &normalized
	} else if frame.Start.Bounded != nil {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "non-Bounded START carries Bounded input", nil)
	}
	normalizedStart := StartInput{RequestID: frame.Start.RequestID, Project: project, Proposal: cloneProposal(proposal), Bounded: cloneBoundedInput(boundedInput)}
	messageDigest, err := frameContentDigest(RunFrame{
		SchemaVersion: RuntimeSchemaV1,
		Kind:          FrameStart,
		Start:         &normalizedStart,
	})
	if err != nil {
		return RunReply{}, err
	}
	runID := deriveRunID(frame.IdempotencyKey)
	if current, loadErr := engine.journal.loadCommitted(runID); loadErr == nil {
		replayed, found, replayErr := engine.replay(current.Snapshot, frame.IdempotencyKey, messageDigest)
		if replayErr != nil {
			return RunReply{}, replayErr
		}
		if found {
			return cloneReply(replayed), nil
		}
		return RunReply{}, runtimeError("IDEMPOTENCY_KEY_REUSED", "derived Run already exists for another message", nil)
	} else if ErrorCode(loadErr) != "RUN_NOT_FOUND" {
		return RunReply{}, loadErr
	}
	classificationProposal := cloneProposal(proposal)
	if boundedInput != nil && classificationProposal.CapabilitySelector == nil && boundedInput.TrustedRuleID != "" && boundedConfigurationReady(project, engine.bounded) {
		resolved, _, resolveErr := resolveBoundedSelector(nil, boundedInput.TrustedRuleID, engine.bounded)
		if resolveErr != nil {
			return RunReply{}, resolveErr
		}
		classificationProposal.CapabilitySelector = resolved
	}
	decision, err := classification.Classify(classificationProposal, engine.rules)
	if err != nil {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "invalid classification rules", err)
	}
	status := RunReleased
	event := "DIRECT_RELEASED"
	var bounded *BoundedState
	selectionDiagnostic := ""
	switch decision.RequestMode {
	case classification.RequestModeDirect:
		if boundedInput != nil {
			return RunReply{}, runtimeError("BOUNDED_REQUEST_INVALID", "Bounded request was not classified as BOUNDED", nil)
		}
	case classification.RequestModeBounded:
		if boundedInput == nil {
			return RunReply{}, runtimeError("BOUNDED_REQUEST_INVALID", "Bounded input is required", nil)
		}
		if !boundedConfigurationReady(project, engine.bounded) {
			return RunReply{}, runtimeError("BOUNDED_CONFIGURATION_REQUIRED", "pinned Configuration and Registry are required", nil)
		}
		selector, diagnostic, resolveErr := resolveBoundedSelector(classificationProposal.CapabilitySelector, boundedInput.TrustedRuleID, engine.bounded)
		if resolveErr != nil {
			return RunReply{}, resolveErr
		}
		status = RunReady
		event = "BOUNDED_READY"
		if selector == nil {
			status = RunAwaitingCapability
			event = "BOUNDED_AWAITING_CAPABILITY"
			selectionDiagnostic = diagnostic
		}
		bounded = boundedState(*boundedInput, selector, engine.bounded)
	default:
		return RunReply{}, runtimeError("REQUEST_MODE_NOT_IMPLEMENTED", fmt.Sprintf("request mode %s is not implemented", decision.RequestMode), nil)
	}
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
			RequestMode:          decision.RequestMode,
			Status:               status,
			Classification:       cloneDecision(decision),
			ClassificationDigest: decision.Digest(),
			ConfigurationDigest:  project.ConfigurationDigest,
			Bounded:              bounded,
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
		if snapshot.RequestMode == classification.RequestModeBounded {
			candidateReply = boundedReply(snapshot, selectionDiagnostic)
		}
		committed, commitErr := engine.journal.commit(revisionRecord{
			SchemaVersion:  revisionSchemaV1,
			RunID:          runID,
			Revision:       1,
			MessageID:      frame.MessageID,
			IdempotencyKey: frame.IdempotencyKey,
			MessageDigest:  messageDigest,
			Event:          event,
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
	if frame.Start != nil || frame.Continue == nil || !runIDPattern.MatchString(frame.RunID) || frame.ExpectedRevision == 0 {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "CONTINUE payload shape is invalid", nil)
	}
	var normalizedContinue ContinueInput
	switch frame.Continue.Signal {
	case SignalScopeExpanded:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "SCOPE_EXPANDED carries Capability selection", nil)
		}
		normalizedContinue = ContinueInput{Signal: SignalScopeExpanded}
	case SignalCapabilitySelected:
		var err error
		normalizedContinue, err = normalizeCapabilitySelection(*frame.Continue)
		if err != nil {
			return RunReply{}, err
		}
	default:
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "unknown CONTINUE signal", nil)
	}
	messageDigest, err := frameContentDigest(RunFrame{
		SchemaVersion:    RuntimeSchemaV1,
		Kind:             FrameContinue,
		RunID:            frame.RunID,
		ExpectedRevision: frame.ExpectedRevision,
		Continue:         &normalizedContinue,
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
		if normalizedContinue.Signal == SignalCapabilitySelected {
			if current.Snapshot.RequestMode != classification.RequestModeBounded || current.Snapshot.Status != RunAwaitingCapability || current.Snapshot.Bounded == nil || current.Snapshot.Bounded.Selector != nil {
				return runtimeError("RUN_TRANSITION_INVALID", "Capability selection requires an awaiting Bounded run", nil)
			}
			if !boundedConfigurationMatches(current.Snapshot.Bounded, engine.bounded) {
				return runtimeError("BOUNDED_CONFIGURATION_REQUIRED", "active Run trusted inputs do not match Engine options", nil)
			}
			selector, diagnostic, resolveErr := resolveBoundedSelector(normalizedContinue.CapabilitySelector, normalizedContinue.TrustedRuleID, engine.bounded)
			if resolveErr != nil {
				return resolveErr
			}
			if selector == nil {
				return runtimeError(diagnostic, "Capability selection is not admissible", nil)
			}
			nextRevision := current.Revision + 1
			snapshot := cloneSnapshot(current.Snapshot)
			snapshot.Revision = nextRevision
			snapshot.Status = RunReady
			snapshot.Bounded.Selector = selector
			snapshot.Bounded.Input.TrustedRuleID = normalizedContinue.TrustedRuleID
			snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{
				IdempotencyKey: frame.IdempotencyKey,
				ContentDigest:  messageDigest,
				Revision:       nextRevision,
			})
			sort.Slice(snapshot.ProcessedMessages, func(i, j int) bool {
				return snapshot.ProcessedMessages[i].IdempotencyKey < snapshot.ProcessedMessages[j].IdempotencyKey
			})
			committed, commitErr := engine.journal.commit(revisionRecord{
				SchemaVersion:     revisionSchemaV1,
				RunID:             frame.RunID,
				Revision:          nextRevision,
				PredecessorDigest: current.Digest,
				MessageID:         frame.MessageID,
				IdempotencyKey:    frame.IdempotencyKey,
				MessageDigest:     messageDigest,
				Event:             "BOUNDED_CAPABILITY_SELECTED",
				Snapshot:          snapshot,
				Reply:             boundedReply(snapshot, ""),
			})
			if commitErr != nil {
				return commitErr
			}
			reply = cloneReply(committed.Reply)
			return nil
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
