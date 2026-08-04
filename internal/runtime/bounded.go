package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

func cloneBoundedOptions(value BoundedOptions) BoundedOptions {
	value.Authority = admission.CloneAuthority(value.Authority)
	value.Executors = admission.CloneExecutors(value.Executors)
	return value
}

func proposalRequestsBounded(value classification.ClassificationProposal) bool {
	for _, observation := range value.Traits {
		if observation.Trait == classification.TraitBoundedCapabilityRequest && observation.Value == classification.TraitTrue {
			return true
		}
	}
	return false
}

func cloneBoundedInput(value *BoundedInput) *BoundedInput {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.RequestedEffects = append([]string{}, value.RequestedEffects...)
	cloned.RequestedResources = append([]string{}, value.RequestedResources...)
	return &cloned
}

func normalizeBoundedInput(value *BoundedInput) (BoundedInput, error) {
	if value == nil {
		return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "Bounded input is required", nil)
	}
	if err := validateIdentifier(value.DeliverableID); err != nil {
		return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "invalid Bounded identity", err)
	}
	if !validDigest(value.InputDigest) {
		return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "invalid Bounded input digest", nil)
	}
	if err := validateIdentifier(strings.TrimSpace(value.TerminationCondition)); err != nil {
		return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "invalid termination condition", err)
	}
	if err := validateIdentifier(value.ExecutorID); err != nil {
		return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "invalid Executor ID", err)
	}
	effects, err := normalizeBoundedSet(value.RequestedEffects)
	if err != nil || len(effects) == 0 {
		return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "requested effects must be a unique non-empty set", err)
	}
	resources, err := normalizeBoundedSet(value.RequestedResources)
	if err != nil || len(resources) == 0 {
		return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "requested resources must be a unique non-empty set", err)
	}
	trustedRuleID := strings.TrimSpace(value.TrustedRuleID)
	if trustedRuleID != "" {
		if _, err := catalog.ParseLocalID(trustedRuleID); err != nil {
			return BoundedInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "invalid trusted rule ID", err)
		}
	}
	return BoundedInput{
		DeliverableID:        value.DeliverableID,
		InputDigest:          value.InputDigest,
		RequestedEffects:     effects,
		RequestedResources:   resources,
		TerminationCondition: strings.TrimSpace(value.TerminationCondition),
		ExecutorID:           value.ExecutorID,
		TrustedRuleID:        trustedRuleID,
	}, nil
}

func normalizeBoundedSelector(value *classification.CapabilitySelector) (*classification.CapabilitySelector, error) {
	if value == nil {
		return nil, nil
	}
	if _, err := catalog.ParseQualifiedID(value.ProviderID); err != nil {
		return nil, runtimeError("BOUNDED_REQUEST_INVALID", "invalid selector Provider ID", err)
	}
	if _, err := catalog.ParseLocalID(value.CapabilityID); err != nil {
		return nil, runtimeError("BOUNDED_REQUEST_INVALID", "invalid selector Capability ID", err)
	}
	if value.Source != classification.SelectorUserIntent && value.Source != classification.SelectorTrustedRule {
		return nil, runtimeError("BOUNDED_REQUEST_INVALID", "invalid selector source", nil)
	}
	selector := *value
	return &selector, nil
}

type boundedSelectionDiagnostic struct {
	Code    string
	Message string
}

func newBoundedSelectionDiagnostic(code string) boundedSelectionDiagnostic {
	return boundedSelectionDiagnostic{Code: code, Message: fmt.Sprintf("Bounded Capability selection is not admissible: %s.", code)}
}

func resolveBoundedSelector(value *classification.CapabilitySelector, trustedRuleID string, options BoundedOptions) (*classification.CapabilitySelector, boundedSelectionDiagnostic, error) {
	selector, err := normalizeBoundedSelector(value)
	if err != nil {
		return nil, boundedSelectionDiagnostic{}, err
	}
	trustedRuleID = strings.TrimSpace(trustedRuleID)
	if selector == nil && trustedRuleID == "" {
		return nil, newBoundedSelectionDiagnostic("CAPABILITY_SELECTION_REQUIRED"), nil
	}
	if selector != nil && selector.Source == classification.SelectorUserIntent && trustedRuleID != "" {
		return nil, boundedSelectionDiagnostic{}, runtimeError("BOUNDED_REQUEST_INVALID", "user-intent selector cannot carry a trusted rule ID", nil)
	}
	if selector != nil && selector.Source == classification.SelectorTrustedRule && trustedRuleID == "" {
		return nil, boundedSelectionDiagnostic{}, runtimeError("BOUNDED_REQUEST_INVALID", "trusted-rule selector requires a trusted rule ID", nil)
	}
	if trustedRuleID != "" {
		trusted := false
		for _, rule := range options.Configuration.BoundedCapabilityDefaults() {
			if rule.ID == trustedRuleID {
				trusted = true
				if selector == nil {
					selector = &classification.CapabilitySelector{ProviderID: rule.ProviderID, CapabilityID: rule.CapabilityID, Source: classification.SelectorTrustedRule}
				} else if selector.ProviderID != rule.ProviderID || selector.CapabilityID != rule.CapabilityID || selector.Source != classification.SelectorTrustedRule {
					return nil, newBoundedSelectionDiagnostic("CAPABILITY_NOT_VERIFIED"), nil
				}
				break
			}
		}
		if !trusted {
			return nil, newBoundedSelectionDiagnostic("CAPABILITY_NOT_VERIFIED"), nil
		}
	}
	if selector == nil {
		return nil, newBoundedSelectionDiagnostic("CAPABILITY_SELECTION_REQUIRED"), nil
	}
	if err := admission.VerifyBoundedCapability(*selector, options.Configuration.Catalog(), options.Registry); err != nil {
		code := admission.ErrorCode(err)
		if code == "" {
			code = "CAPABILITY_NOT_VERIFIED"
		}
		if code == "CAPABILITY_NOT_VERIFIED" {
			if diagnostic, found := providerResolutionDiagnostic(options.Resolutions, selector.ProviderID); found {
				return nil, boundedSelectionDiagnostic(diagnostic), nil
			}
		}
		return nil, newBoundedSelectionDiagnostic(code), nil
	}
	return selector, boundedSelectionDiagnostic{}, nil
}

func boundedConfigurationReady(project ProjectIdentity, options BoundedOptions) bool {
	return boundedConfigurationError(project, options) == nil
}

func boundedConfigurationError(project ProjectIdentity, options BoundedOptions) error {
	configurationDigest := options.Configuration.Digest()
	catalogDigest := options.Configuration.Catalog().Digest()
	registryDigest := options.Registry.Digest()
	if !validDigest(configurationDigest) || project.ConfigurationDigest != configurationDigest || !validDigest(catalogDigest) || !validDigest(registryDigest) || !validDigest(options.Resolutions.Digest()) {
		return runtimeError("BOUNDED_CONFIGURATION_REQUIRED", "pinned Configuration and Registry are required", nil)
	}
	registryHostID := options.Registry.HostID()
	resolutionHostID := options.Resolutions.HostID()
	for _, hostID := range []string{options.HostID, registryHostID, resolutionHostID} {
		if _, err := catalog.ParseLocalID(hostID); err != nil {
			return runtimeError("BOUNDED_CONFIGURATION_REQUIRED", "configured Host, Registry, and Resolution Report Host identities are required", err)
		}
	}
	if registryHostID != options.HostID || resolutionHostID != options.HostID {
		return runtimeError("HOST_PROVIDER_SCOPE_MISMATCH", "configured Host, Registry, and Resolution Report do not agree", nil)
	}
	return nil
}

func boundedConfigurationMatchError(state *BoundedState, options BoundedOptions) error {
	if state == nil {
		return runtimeError("BOUNDED_CONFIGURATION_REQUIRED", "active Run has no Bounded trusted inputs", nil)
	}
	if err := boundedConfigurationError(ProjectIdentity{ConfigurationDigest: options.Configuration.Digest()}, options); err != nil {
		return err
	}
	if state.HostID != options.HostID {
		return runtimeError("HOST_PROVIDER_SCOPE_MISMATCH", "active Bounded Run belongs to another Host", nil)
	}
	if state.ConfigurationDigest != options.Configuration.Digest() || state.CatalogDigest != options.Configuration.Catalog().Digest() || state.RegistryDigest != options.Registry.Digest() || !validDigest(state.ConfigurationDigest) || !validDigest(state.CatalogDigest) || !validDigest(state.RegistryDigest) {
		return runtimeError("BOUNDED_CONFIGURATION_REQUIRED", "active Run trusted inputs do not match Engine options", nil)
	}
	return nil
}

func normalizeCapabilitySelection(value ContinueInput) (ContinueInput, error) {
	selector, err := normalizeBoundedSelector(value.CapabilitySelector)
	if err != nil {
		return ContinueInput{}, err
	}
	trustedRuleID := strings.TrimSpace(value.TrustedRuleID)
	if trustedRuleID != "" {
		if _, err := catalog.ParseLocalID(trustedRuleID); err != nil {
			return ContinueInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "invalid trusted rule ID", err)
		}
	}
	if selector == nil && trustedRuleID == "" {
		return ContinueInput{}, runtimeError("BOUNDED_REQUEST_INVALID", "CAPABILITY_SELECTED requires a selector or trusted rule", nil)
	}
	return ContinueInput{Signal: SignalCapabilitySelected, CapabilitySelector: selector, TrustedRuleID: trustedRuleID}, nil
}

func normalizeDispatchPreparation(value *DispatchPreparation) (*DispatchPreparation, error) {
	if value == nil || validateIdentifier(value.GrantID) != nil || validateIdentifier(value.InvocationID) != nil || validateIdentifier(value.ExecutorID) != nil {
		return nil, runtimeError("DISPATCH_PREPARATION_INVALID", "dispatch preparation identities are invalid", nil)
	}
	return &DispatchPreparation{GrantID: value.GrantID, InvocationID: value.InvocationID, ExecutorID: value.ExecutorID}, nil
}

func normalizeCapabilityObservation(value *CapabilityObservation) (*CapabilityObservation, error) {
	if value == nil || validateIdentifier(value.GrantID) != nil || validateIdentifier(value.InvocationID) != nil || validateIdentifier(value.ExecutorID) != nil {
		return nil, runtimeError("OBSERVATION_INVALID", "observation identities are invalid", nil)
	}
	if value.Outcome != ObservationSucceeded && value.Outcome != ObservationFailed {
		return nil, runtimeError("OBSERVATION_INVALID", "observation outcome is not normalized", nil)
	}
	if value.RawOutput != "" || len(value.EvidenceReferences) == 0 {
		return nil, runtimeError("OBSERVATION_INVALID", "observation must contain no raw output and pinned evidence", nil)
	}
	evidence := append([]EvidenceReference{}, value.EvidenceReferences...)
	for index := range evidence {
		evidence[index].Reference = strings.TrimSpace(evidence[index].Reference)
		if validateIdentifier(evidence[index].Reference) != nil || !validDigest(evidence[index].Digest) {
			return nil, runtimeError("OBSERVATION_INVALID", "observation evidence is not digest-pinned", nil)
		}
	}
	sort.Slice(evidence, func(i, j int) bool {
		if evidence[i].Reference == evidence[j].Reference {
			return evidence[i].Digest < evidence[j].Digest
		}
		return evidence[i].Reference < evidence[j].Reference
	})
	for index := 1; index < len(evidence); index++ {
		if evidence[index-1] == evidence[index] {
			return nil, runtimeError("OBSERVATION_INVALID", "observation evidence is duplicated", nil)
		}
	}
	return &CapabilityObservation{
		GrantID:            value.GrantID,
		InvocationID:       value.InvocationID,
		ExecutorID:         value.ExecutorID,
		Outcome:            value.Outcome,
		EvidenceReferences: evidence,
	}, nil
}

func boundedEscalationEvent(signal ContinueSignal) string {
	switch signal {
	case SignalScopeExpanded:
		return "BOUNDED_SCOPE_EXPANDED"
	case SignalAdditionalCapabilityRequired:
		return "BOUNDED_ADDITIONAL_CAPABILITY_REQUIRED"
	case SignalRemediationRequired:
		return "BOUNDED_REMEDIATION_REQUIRED"
	case SignalArchitectureRequired:
		return "BOUNDED_ARCHITECTURE_REQUIRED"
	default:
		return ""
	}
}

func issueBoundedGrant(snapshot RunSnapshot, options BoundedOptions, issuedRevision uint64) (admission.CapabilityGrant, error) {
	if snapshot.Bounded == nil || snapshot.Bounded.Selector == nil {
		return admission.CapabilityGrant{}, runtimeError("RUN_TRANSITION_INVALID", "Bounded Grant requires an admitted selector", nil)
	}
	// Ticket 06 has no durable Resource Lease primitive. Keep that capability
	// disabled at the Runtime boundary even when the trusted adapter advertises
	// a broader future authority ceiling.
	authority := admission.CloneAuthority(options.Authority)
	authority.ResourceLeases = false
	executor := admission.ExecutorRegistration{ID: snapshot.Bounded.Input.ExecutorID}
	for _, registered := range options.Executors {
		if registered.ID == executor.ID {
			executor = registered
			break
		}
	}
	grant, err := admission.IssueBoundedGrant(admission.GrantRequest{
		RunID:                snapshot.RunID,
		RequestID:            snapshot.RequestID,
		DeliverableID:        snapshot.Bounded.Input.DeliverableID,
		InputDigest:          snapshot.Bounded.Input.InputDigest,
		IssuedRevision:       issuedRevision,
		HostID:               options.HostID,
		Selector:             *snapshot.Bounded.Selector,
		Effects:              append([]string{}, snapshot.Bounded.Input.RequestedEffects...),
		Resources:            append([]string{}, snapshot.Bounded.Input.RequestedResources...),
		TerminationCondition: snapshot.Bounded.Input.TerminationCondition,
		Executor:             executor,
		Catalog:              options.Configuration.Catalog(),
		Registry:             options.Registry,
		Authority:            authority,
		Executors:            options.Executors,
		DelegationAllowList:  []string{},
	})
	if err != nil {
		code := admission.ErrorCode(err)
		if code == "" {
			code = "BOUNDED_REQUEST_INVALID"
		}
		return admission.CapabilityGrant{}, runtimeError(code, err.Error(), err)
	}
	return grant, nil
}

func boundedState(input BoundedInput, selector *classification.CapabilitySelector, options BoundedOptions) *BoundedState {
	state := &BoundedState{
		Input:               input,
		HostID:              options.HostID,
		ConfigurationDigest: options.Configuration.Digest(),
		CatalogDigest:       options.Configuration.Catalog().Digest(),
		RegistryDigest:      options.Registry.Digest(),
	}
	state.Input.RequestedEffects = append([]string{}, input.RequestedEffects...)
	state.Input.RequestedResources = append([]string{}, input.RequestedResources...)
	if selector != nil {
		copySelector := *selector
		state.Selector = &copySelector
	}
	return state
}

func boundedReply(snapshot RunSnapshot, diagnostic boundedSelectionDiagnostic) RunReply {
	diagnostics := []Diagnostic{}
	kind := ReplyModeDecided
	if snapshot.Status == RunAwaitingCapability {
		kind = ReplyCapabilitySelectionRequired
		diagnostics = append(diagnostics, Diagnostic{Code: diagnostic.Code, Message: diagnostic.Message})
	}
	return RunReply{
		SchemaVersion:   RuntimeSchemaV1,
		Kind:            kind,
		RunID:           snapshot.RunID,
		Revision:        snapshot.Revision,
		Snapshot:        cloneSnapshot(snapshot),
		Diagnostics:     diagnostics,
		RecoveryActions: []string{},
	}
}

func boundedGrantReply(snapshot RunSnapshot) RunReply {
	return RunReply{
		SchemaVersion:   RuntimeSchemaV1,
		Kind:            ReplyGrantIssued,
		RunID:           snapshot.RunID,
		Revision:        snapshot.Revision,
		Snapshot:        cloneSnapshot(snapshot),
		Diagnostics:     []Diagnostic{},
		RecoveryActions: []string{},
	}
}

func boundedTransitionReply(snapshot RunSnapshot, kind ReplyKind, reason string, recoveryActions []string) RunReply {
	return RunReply{
		SchemaVersion:   RuntimeSchemaV1,
		Kind:            kind,
		RunID:           snapshot.RunID,
		Revision:        snapshot.Revision,
		Snapshot:        cloneSnapshot(snapshot),
		Diagnostics:     []Diagnostic{},
		Reason:          reason,
		RecoveryActions: append([]string{}, recoveryActions...),
	}
}

func boundedTransitionSnapshot(current RunSnapshot, frame RunFrame, messageDigest string, nextRevision uint64) RunSnapshot {
	snapshot := cloneSnapshot(current)
	snapshot.Revision = nextRevision
	snapshot.ProcessedMessages = append(snapshot.ProcessedMessages, ProcessedMessage{
		IdempotencyKey: frame.IdempotencyKey,
		ContentDigest:  messageDigest,
		Revision:       nextRevision,
	})
	sort.Slice(snapshot.ProcessedMessages, func(i, j int) bool {
		return snapshot.ProcessedMessages[i].IdempotencyKey < snapshot.ProcessedMessages[j].IdempotencyKey
	})
	return snapshot
}

func (engine *Engine) continueBoundedHandshake(current revisionRecord, frame RunFrame, normalized ContinueInput, messageDigest string) (RunReply, error) {
	if normalized.Signal == SignalDispatchPrepared {
		if current.Snapshot.Status != RunGranted || len(current.Snapshot.Grants) != 1 || normalized.DispatchPreparation == nil {
			return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "DISPATCH_PREPARED requires a granted Bounded run", nil)
		}
		grant := current.Snapshot.Grants[0]
		preparation := normalized.DispatchPreparation
		if preparation.GrantID != grant.ID || preparation.InvocationID != grant.InvocationID || preparation.ExecutorID != grant.Executor.ID {
			return RunReply{}, runtimeError("DISPATCH_PREPARATION_INVALID", "preparation does not identify the committed Grant", nil)
		}
		nextRevision := current.Revision + 1
		snapshot := boundedTransitionSnapshot(current.Snapshot, frame, messageDigest, nextRevision)
		snapshot.Status = RunInFlight
		return engine.commitBoundedHandshake(current, frame, messageDigest, snapshot, "BOUNDED_DISPATCH_AUTHORIZED", boundedTransitionReply(snapshot, ReplyDispatchAuthorized, "", nil))
	}

	if normalized.Signal == SignalCapabilityObserved {
		if current.Snapshot.Status != RunInFlight || len(current.Snapshot.Grants) != 1 || normalized.Observation == nil {
			return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "CAPABILITY_OBSERVED requires an authorized Bounded invocation", nil)
		}
		grant := current.Snapshot.Grants[0]
		observation := normalized.Observation
		if observation.GrantID != grant.ID || observation.InvocationID != grant.InvocationID || observation.ExecutorID != grant.Executor.ID {
			return RunReply{}, runtimeError("OBSERVATION_INVALID", "observation does not identify the authorized invocation", nil)
		}
		nextRevision := current.Revision + 1
		snapshot := boundedTransitionSnapshot(current.Snapshot, frame, messageDigest, nextRevision)
		snapshot.Observations = append(snapshot.Observations, *observation)
		if observation.Outcome == ObservationSucceeded {
			snapshot.Status = RunFinished
			return engine.commitBoundedHandshake(current, frame, messageDigest, snapshot, "BOUNDED_CAPABILITY_FINISHED", boundedTransitionReply(snapshot, ReplyFinished, "", nil))
		}
		snapshot.Status = RunPaused
		return engine.commitBoundedHandshake(current, frame, messageDigest, snapshot, "BOUNDED_CAPABILITY_FAILED", boundedTransitionReply(snapshot, ReplyPaused, ReasonModeEscalationRequired, []string{RecoveryStartSuccessorRun}))
	}

	if normalized.Signal == SignalExecutionUncertain {
		if current.Snapshot.Status != RunInFlight || len(current.Snapshot.Grants) != 1 {
			return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "EXECUTION_UNCERTAIN requires an authorized Bounded invocation", nil)
		}
		nextRevision := current.Revision + 1
		snapshot := boundedTransitionSnapshot(current.Snapshot, frame, messageDigest, nextRevision)
		snapshot.Status = RunPaused
		return engine.commitBoundedHandshake(current, frame, messageDigest, snapshot, "BOUNDED_EXECUTION_UNCERTAIN", boundedTransitionReply(snapshot, ReplyPaused, ReasonExecutionUncertain, []string{RecoveryReconcileInvocation}))
	}

	if event := boundedEscalationEvent(normalized.Signal); event != "" {
		if current.Snapshot.Status != RunInFlight || len(current.Snapshot.Grants) != 1 {
			return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "Bounded escalation requires an authorized invocation", nil)
		}
		nextRevision := current.Revision + 1
		snapshot := boundedTransitionSnapshot(current.Snapshot, frame, messageDigest, nextRevision)
		snapshot.Status = RunPaused
		return engine.commitBoundedHandshake(current, frame, messageDigest, snapshot, event, boundedTransitionReply(snapshot, ReplyPaused, ReasonModeEscalationRequired, []string{RecoveryStartSuccessorRun}))
	}

	return RunReply{}, runtimeError("RUN_TRANSITION_INVALID", "unsupported Bounded transition", nil)
}

func (engine *Engine) commitBoundedHandshake(current revisionRecord, frame RunFrame, messageDigest string, snapshot RunSnapshot, event string, candidateReply RunReply) (RunReply, error) {
	committed, err := engine.journal.commit(revisionRecord{
		SchemaVersion:     revisionSchemaV1,
		RunID:             frame.RunID,
		Revision:          snapshot.Revision,
		PredecessorDigest: current.Digest,
		MessageID:         frame.MessageID,
		IdempotencyKey:    frame.IdempotencyKey,
		MessageDigest:     messageDigest,
		Event:             event,
		Snapshot:          snapshot,
		Reply:             candidateReply,
	})
	if err != nil {
		return RunReply{}, err
	}
	return cloneReply(committed.Reply), nil
}

func normalizeBoundedSet(values []string) ([]string, error) {
	result := append([]string{}, values...)
	sort.Strings(result)
	for index, value := range result {
		if value == "" {
			return nil, fmt.Errorf("empty set member")
		}
		if index > 0 && result[index-1] == value {
			return nil, fmt.Errorf("duplicate set member %q", value)
		}
	}
	return result, nil
}
