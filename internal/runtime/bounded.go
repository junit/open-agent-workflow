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

func resolveBoundedSelector(value *classification.CapabilitySelector, trustedRuleID string, options BoundedOptions) (*classification.CapabilitySelector, string, error) {
	selector, err := normalizeBoundedSelector(value)
	if err != nil {
		return nil, "", err
	}
	trustedRuleID = strings.TrimSpace(trustedRuleID)
	if selector == nil && trustedRuleID == "" {
		return nil, "CAPABILITY_SELECTION_REQUIRED", nil
	}
	if selector != nil && selector.Source == classification.SelectorUserIntent && trustedRuleID != "" {
		return nil, "", runtimeError("BOUNDED_REQUEST_INVALID", "user-intent selector cannot carry a trusted rule ID", nil)
	}
	if selector != nil && selector.Source == classification.SelectorTrustedRule && trustedRuleID == "" {
		return nil, "", runtimeError("BOUNDED_REQUEST_INVALID", "trusted-rule selector requires a trusted rule ID", nil)
	}
	if trustedRuleID != "" {
		trusted := false
		for _, rule := range options.Configuration.BoundedCapabilityDefaults() {
			if rule.ID == trustedRuleID {
				trusted = true
				if selector == nil {
					selector = &classification.CapabilitySelector{ProviderID: rule.ProviderID, CapabilityID: rule.CapabilityID, Source: classification.SelectorTrustedRule}
				} else if selector.ProviderID != rule.ProviderID || selector.CapabilityID != rule.CapabilityID || selector.Source != classification.SelectorTrustedRule {
					return nil, "CAPABILITY_NOT_VERIFIED", nil
				}
				break
			}
		}
		if !trusted {
			return nil, "CAPABILITY_NOT_VERIFIED", nil
		}
	}
	if selector == nil {
		return nil, "CAPABILITY_SELECTION_REQUIRED", nil
	}
	if err := admission.VerifyBoundedCapability(*selector, options.Configuration.Catalog(), options.Registry); err != nil {
		code := admission.ErrorCode(err)
		if code == "" {
			code = "CAPABILITY_NOT_VERIFIED"
		}
		return nil, code, nil
	}
	return selector, "", nil
}

func boundedConfigurationReady(project ProjectIdentity, options BoundedOptions) bool {
	configurationDigest := options.Configuration.Digest()
	catalogDigest := options.Configuration.Catalog().Digest()
	registryDigest := options.Registry.Digest()
	return validDigest(configurationDigest) && project.ConfigurationDigest == configurationDigest && validDigest(catalogDigest) && validDigest(registryDigest)
}

func boundedConfigurationMatches(state *BoundedState, options BoundedOptions) bool {
	if state == nil {
		return false
	}
	return state.ConfigurationDigest == options.Configuration.Digest() &&
		state.CatalogDigest == options.Configuration.Catalog().Digest() &&
		state.RegistryDigest == options.Registry.Digest() &&
		validDigest(state.ConfigurationDigest) && validDigest(state.CatalogDigest) && validDigest(state.RegistryDigest)
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

func boundedReply(snapshot RunSnapshot, diagnosticCode string) RunReply {
	diagnostics := []Diagnostic{}
	kind := ReplyModeDecided
	if snapshot.Status == RunAwaitingCapability {
		kind = ReplyCapabilitySelectionRequired
		diagnostics = append(diagnostics, Diagnostic{Code: diagnosticCode, Message: fmt.Sprintf("Bounded Capability selection is not admissible: %s.", diagnosticCode)})
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
