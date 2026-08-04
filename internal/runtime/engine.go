package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

const maximumIdentifierLength = 256

type Engine struct {
	rules      classification.ClassificationRules
	bounded    BoundedOptions
	workflow   WorkflowOptions
	projection ProjectionSink
	journal    *journal
}

func NewEngine(options Options) (*Engine, error) {
	journal, err := newJournal(options.StateRoot)
	if err != nil {
		return nil, err
	}
	workflow := cloneWorkflowOptions(options.Workflow)
	projection, err := projectionSinkFromOptions(workflow.Projection)
	if err != nil {
		return nil, err
	}
	return &Engine{rules: cloneRules(options.Rules), bounded: cloneBoundedOptions(options.Bounded), workflow: workflow, projection: projection, journal: journal}, nil
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
	var workflowInput *WorkflowInput
	if proposalRequestsBounded(*proposal) {
		normalized, normalizeErr := normalizeBoundedInput(frame.Start.Bounded)
		if normalizeErr != nil {
			return RunReply{}, normalizeErr
		}
		boundedInput = &normalized
	} else if frame.Start.Bounded != nil {
		return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "non-Bounded START carries Bounded input", nil)
	}
	if frame.Start.Workflow != nil {
		normalized, normalizeErr := normalizeWorkflowInput(frame.Start.Workflow)
		if normalizeErr != nil {
			return RunReply{}, normalizeErr
		}
		workflowInput = &normalized
	}
	normalizedStart := StartInput{RequestID: frame.Start.RequestID, Project: project, Proposal: cloneProposal(proposal), Bounded: cloneBoundedInput(boundedInput), Workflow: cloneWorkflowInput(workflowInput)}
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
	var workflow *WorkflowState
	selectionDiagnostic := boundedSelectionDiagnostic{}
	switch decision.RequestMode {
	case classification.RequestModeDirect:
		if boundedInput != nil || workflowInput != nil {
			return RunReply{}, runtimeError("BOUNDED_REQUEST_INVALID", "Bounded request was not classified as BOUNDED", nil)
		}
	case classification.RequestModeBounded:
		if boundedInput == nil {
			return RunReply{}, runtimeError("BOUNDED_REQUEST_INVALID", "Bounded input is required", nil)
		}
		if err := boundedConfigurationError(project, engine.bounded); err != nil {
			return RunReply{}, err
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
		if workflowInput != nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "Bounded START carries Workflow input", nil)
		}
	case classification.RequestModeWorkflow:
		if workflowInput == nil || boundedInput != nil {
			return RunReply{}, runtimeError("WORKFLOW_REQUEST_INVALID", "Workflow input is required", nil)
		}
		if err := workflowConfigurationError(project, engine.workflow); err != nil {
			return RunReply{}, err
		}
		status = RunAwaitingSelection
		event = "WORKFLOW_SELECTION_REQUIRED"
		workflow = workflowAwaitingState(*workflowInput, engine.workflow)
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
			SchemaVersion:        snapshotSchemaV2,
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
			Workflow:             workflow,
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
		} else if snapshot.RequestMode == classification.RequestModeWorkflow {
			candidateReply = workflowSelectionReply(snapshot)
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
		if snapshot.RequestMode == classification.RequestModeWorkflow {
			engine.projectCommittedWorkflow(committed)
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
	if committed.Snapshot.RequestMode == classification.RequestModeWorkflow && committed.Snapshot.Workflow != nil && committed.Snapshot.Workflow.ActiveGeneration > 0 {
		bundle, bundleErr := workflowActiveBundle(committed.Snapshot)
		if bundleErr != nil {
			return RunReply{}, bundleErr
		}
		if hostErr := validateActiveWorkflowHost(engine.workflow, bundle); hostErr != nil {
			return RunReply{}, hostErr
		}
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
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.DispatchPreparation != nil || frame.Continue.Observation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageGrant != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch != nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "SCOPE_EXPANDED carries an unexpected payload", nil)
		}
		normalizedContinue = ContinueInput{Signal: SignalScopeExpanded}
	case SignalCapabilitySelected:
		if frame.Continue.DispatchPreparation != nil || frame.Continue.Observation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageGrant != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch != nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "CAPABILITY_SELECTED carries a dispatch payload", nil)
		}
		var err error
		normalizedContinue, err = normalizeCapabilitySelection(*frame.Continue)
		if err != nil {
			return RunReply{}, err
		}
	case SignalProfileSelected:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.DispatchPreparation != nil || frame.Continue.Observation != nil || frame.Continue.StageGrant != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch != nil || frame.Continue.ProfileSelection == nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "PROFILE_SELECTED carries an invalid payload", nil)
		}
		selection, normalizeErr := normalizeProfileSelection(*frame.Continue.ProfileSelection)
		if normalizeErr != nil {
			return RunReply{}, normalizeErr
		}
		normalizedContinue = ContinueInput{Signal: SignalProfileSelected, ProfileSelection: &selection}
	case SignalRequestStageGrant:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.DispatchPreparation != nil || frame.Continue.Observation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch != nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "REQUEST_STAGE_GRANT carries an invalid payload", nil)
		}
		stage, normalizeErr := normalizeStageGrantRequest(frame.Continue.StageGrant)
		if normalizeErr != nil {
			return RunReply{}, normalizeErr
		}
		normalizedContinue = ContinueInput{Signal: SignalRequestStageGrant, StageGrant: stage}
	case SignalRequestDispatch:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.DispatchPreparation != nil || frame.Continue.Observation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageGrant != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch != nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "REQUEST_DISPATCH carries an unexpected payload", nil)
		}
		normalizedContinue = ContinueInput{Signal: SignalRequestDispatch}
	case SignalDispatchPrepared:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.Observation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageGrant != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch != nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "DISPATCH_PREPARED carries an unexpected payload", nil)
		}
		preparation, normalizeErr := normalizeDispatchPreparation(frame.Continue.DispatchPreparation)
		if normalizeErr != nil {
			return RunReply{}, normalizeErr
		}
		normalizedContinue = ContinueInput{Signal: SignalDispatchPrepared, DispatchPreparation: preparation}
	case SignalCapabilityObserved:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.DispatchPreparation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageGrant != nil || frame.Continue.StableBoundarySwitch != nil || (frame.Continue.Observation == nil && frame.Continue.StageObservation == nil) || (frame.Continue.Observation != nil && frame.Continue.StageObservation != nil) {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "CAPABILITY_OBSERVED carries an unexpected payload", nil)
		}
		if frame.Continue.StageObservation != nil {
			observation, normalizeErr := normalizeStageObservation(frame.Continue.StageObservation)
			if normalizeErr != nil {
				return RunReply{}, normalizeErr
			}
			normalizedContinue = ContinueInput{Signal: SignalCapabilityObserved, StageObservation: observation}
		} else {
			observation, normalizeErr := normalizeCapabilityObservation(frame.Continue.Observation)
			if normalizeErr != nil {
				return RunReply{}, normalizeErr
			}
			normalizedContinue = ContinueInput{Signal: SignalCapabilityObserved, Observation: observation}
		}
	case SignalSwitchProfile:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.DispatchPreparation != nil || frame.Continue.Observation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageGrant != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch == nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "SWITCH_PROFILE carries an invalid payload", nil)
		}
		switchRequest, normalizeErr := normalizeStableBoundarySwitch(frame.Continue.StableBoundarySwitch)
		if normalizeErr != nil {
			return RunReply{}, normalizeErr
		}
		normalizedContinue = ContinueInput{Signal: SignalSwitchProfile, StableBoundarySwitch: switchRequest}
	case SignalExecutionUncertain, SignalAdditionalCapabilityRequired, SignalRemediationRequired, SignalArchitectureRequired:
		if frame.Continue.CapabilitySelector != nil || frame.Continue.TrustedRuleID != "" || frame.Continue.DispatchPreparation != nil || frame.Continue.Observation != nil || frame.Continue.ProfileSelection != nil || frame.Continue.StageGrant != nil || frame.Continue.StageObservation != nil || frame.Continue.StableBoundarySwitch != nil {
			return RunReply{}, runtimeError("RUNTIME_FRAME_INVALID", "control signal carries an unexpected payload", nil)
		}
		normalizedContinue = ContinueInput{Signal: frame.Continue.Signal}
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
	runAction := func() error {
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
		if normalizedContinue.Signal == SignalProfileSelected {
			selected, selectErr := engine.selectWorkflowProfile(current, frame, normalizedContinue, messageDigest)
			if selectErr != nil {
				return selectErr
			}
			reply = selected
			return nil
		}
		if normalizedContinue.Signal == SignalRequestStageGrant {
			granted, grantErr := engine.issueWorkflowStage(current, frame, normalizedContinue.StageGrant, messageDigest)
			if grantErr != nil {
				return grantErr
			}
			reply = granted
			return nil
		}
		if current.Snapshot.RequestMode == classification.RequestModeWorkflow {
			workflowReply, workflowErr := engine.continueWorkflow(current, frame, normalizedContinue, messageDigest)
			if workflowErr != nil {
				return workflowErr
			}
			reply = workflowReply
			return nil
		}
		if normalizedContinue.Signal == SignalCapabilitySelected {
			if current.Snapshot.RequestMode != classification.RequestModeBounded || current.Snapshot.Status != RunAwaitingCapability || current.Snapshot.Bounded == nil || current.Snapshot.Bounded.Selector != nil {
				return runtimeError("RUN_TRANSITION_INVALID", "Capability selection requires an awaiting Bounded run", nil)
			}
			if err := boundedConfigurationMatchError(current.Snapshot.Bounded, engine.bounded); err != nil {
				return err
			}
			selector, diagnostic, resolveErr := resolveBoundedSelector(normalizedContinue.CapabilitySelector, normalizedContinue.TrustedRuleID, engine.bounded)
			if resolveErr != nil {
				return resolveErr
			}
			if selector == nil {
				return runtimeError(diagnostic.Code, diagnostic.Message, nil)
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
				Reply:             boundedReply(snapshot, boundedSelectionDiagnostic{}),
			})
			if commitErr != nil {
				return commitErr
			}
			reply = cloneReply(committed.Reply)
			return nil
		}
		if normalizedContinue.Signal == SignalRequestDispatch {
			if current.Snapshot.RequestMode != classification.RequestModeBounded || current.Snapshot.Status != RunReady || current.Snapshot.Bounded == nil || current.Snapshot.Bounded.Selector == nil || len(current.Snapshot.Grants) != 0 || len(current.Snapshot.GrantIDs) != 0 {
				return runtimeError("RUN_TRANSITION_INVALID", "REQUEST_DISPATCH requires a ready Bounded run without a Grant", nil)
			}
			if err := boundedConfigurationMatchError(current.Snapshot.Bounded, engine.bounded); err != nil {
				return err
			}
			nextRevision := current.Revision + 1
			grant, grantErr := issueBoundedGrant(current.Snapshot, engine.bounded, nextRevision)
			if grantErr != nil {
				return grantErr
			}
			snapshot := cloneSnapshot(current.Snapshot)
			snapshot.Revision = nextRevision
			snapshot.Status = RunGranted
			snapshot.Grants = []admission.CapabilityGrant{admission.CloneGrant(grant)}
			snapshot.GrantIDs = []string{grant.ID}
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
				Event:             "BOUNDED_GRANT_ISSUED",
				Snapshot:          snapshot,
				Reply:             boundedGrantReply(snapshot),
			})
			if commitErr != nil {
				return commitErr
			}
			reply = cloneReply(committed.Reply)
			return nil
		}
		if current.Snapshot.RequestMode == classification.RequestModeBounded {
			boundedReply, boundedErr := engine.continueBoundedHandshake(current, frame, normalizedContinue, messageDigest)
			if boundedErr != nil {
				return boundedErr
			}
			reply = boundedReply
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
	}
	if normalizedContinue.Signal == SignalRequestStageGrant || normalizedContinue.Signal == SignalCapabilityObserved || normalizedContinue.Signal == SignalSwitchProfile {
		err = engine.journal.withResourceLeaseLock(func() error {
			return engine.journal.withRunLock(frame.RunID, runAction)
		})
	} else {
		err = engine.journal.withRunLock(frame.RunID, runAction)
	}
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
	switch frame.Kind {
	case FrameStart, FrameContinue, FrameInspect:
	default:
		return runtimeError("RUNTIME_FRAME_INVALID", fmt.Sprintf("unsupported frame kind %q", frame.Kind), nil)
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
	physical, err := canonicalPhysicalRoot(value.Root)
	if err != nil {
		return ProjectIdentity{}, runtimeError("PROJECT_IDENTITY_INVALID", "resolve physical project root", err)
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
