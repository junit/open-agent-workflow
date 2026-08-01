package classification_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

func TestDecodeProposalAcceptsClosedEvidenceBackedRecord(t *testing.T) {
	raw, err := json.Marshal(completeProposal())
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := classification.DecodeProposal(raw)
	if err != nil {
		t.Fatalf("DecodeProposal() error = %v", err)
	}
	if proposal.SchemaVersion != classification.ProposalSchemaV1 || len(proposal.Traits) != len(allTraits()) || len(proposal.Evidence) != 3 {
		t.Fatalf("proposal = %#v", proposal)
	}
	if proposal.Traits[0].Trait != classification.TraitArchitectureDecision || proposal.Evidence[0].Kind != classification.EvidenceChangePoint {
		t.Fatalf("proposal was not normalized: %#v", proposal)
	}
	if proposal.Resources == nil || proposal.Evidence == nil || proposal.Traits == nil {
		t.Fatal("normalized collections must be non-nil")
	}
}

func TestDecodeProposalRejectsUnknownFieldsTrailingJSONAndUnknownTraits(t *testing.T) {
	raw, err := json.Marshal(completeProposal())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		data []byte
		code string
	}{
		{"unknown field", append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"extra":true}`)...), "CLASSIFICATION_UNKNOWN_FIELD"},
		{"trailing JSON", append(append([]byte{}, raw...), []byte(`{}`)...), "CLASSIFICATION_TRAILING_JSON"},
		{"unknown trait", bytes.Replace(append([]byte{}, raw...), []byte(`architecture-decision`), []byte(`future-trait`), 1), "CLASSIFICATION_TRAIT_UNKNOWN"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := classification.DecodeProposal(tt.data); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("DecodeProposal() error = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestDecodeProposalRejectsDuplicateTraitsInvalidDigestsAndBadSelectors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*classification.ClassificationProposal)
		code   string
	}{
		{"duplicate traits", func(value *classification.ClassificationProposal) {
			value.Traits = append(value.Traits, value.Traits[0])
			value.Evidence[0].Digest = strings.Repeat("a", 64)
			value.CapabilitySelector = nil
		}, "CLASSIFICATION_DUPLICATE_TRAIT"},
		{"invalid digest", func(value *classification.ClassificationProposal) {
			value.Evidence[0].Digest = strings.Repeat("Z", 64)
		}, "CLASSIFICATION_EVIDENCE_DIGEST_INVALID"},
		{"bad selector", func(value *classification.ClassificationProposal) {
			value.CapabilitySelector = &classification.CapabilitySelector{ProviderID: "bad", CapabilityID: "review", Source: classification.SelectorUserIntent}
		}, "CLASSIFICATION_SELECTOR_INVALID"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			value := completeProposal()
			tt.mutate(&value)
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := classification.DecodeProposal(raw); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("DecodeProposal() error = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestProposalNormalizationIsDeterministicAndDefensive(t *testing.T) {
	first, err := classification.DecodeProposal(mustJSON(t, completeProposal()))
	if err != nil {
		t.Fatal(err)
	}
	second, err := classification.DecodeProposal(mustJSON(t, completeProposal()))
	if err != nil {
		t.Fatal(err)
	}
	if first.Traits[0].Trait != second.Traits[0].Trait || first.Evidence[0].Digest != second.Evidence[0].Digest {
		t.Fatalf("normalization differs: %#v / %#v", first, second)
	}
	first.Traits[0].Trait = "changed"
	if second.Traits[0].Trait == "changed" {
		t.Fatal("decoded proposals share trait storage")
	}
}

func TestClassifyClearDirectRequest(t *testing.T) {
	proposal := completeProposal()
	setTrait(&proposal, classification.TraitScopeClear, classification.TraitTrue)
	setTrait(&proposal, classification.TraitChangePointKnown, classification.TraitTrue)
	setTrait(&proposal, classification.TraitRecoverable, classification.TraitTrue)
	setTrait(&proposal, classification.TraitFocusedVerificationKnown, classification.TraitTrue)
	decision, err := classification.Classify(&proposal, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeDirect || decision.WorkflowComplexity != nil || decision.RiskClass != classification.RiskNormal {
		t.Fatalf("decision = %#v", decision)
	}
	if len(decision.EscalationReasons) != 0 || decision.CapabilitySelector != nil || decision.Digest() == "" {
		t.Fatalf("direct decision = %#v", decision)
	}
}

func TestClassifyBoundedRequestRequiresExactSelector(t *testing.T) {
	withScope := completeProposal()
	setTrait(&withScope, classification.TraitScopeClear, classification.TraitTrue)
	setTrait(&withScope, classification.TraitChangePointKnown, classification.TraitTrue)
	setTrait(&withScope, classification.TraitRecoverable, classification.TraitTrue)
	setTrait(&withScope, classification.TraitFocusedVerificationKnown, classification.TraitTrue)
	setTrait(&withScope, classification.TraitBoundedCapabilityRequest, classification.TraitTrue)
	withScope.CapabilitySelector = &classification.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "review", Source: classification.SelectorUserIntent}
	decision, err := classification.Classify(&withScope, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeBounded || decision.CapabilitySelector == nil || decision.CapabilitySelector.ProviderID != "acme/suite" {
		t.Fatalf("bounded decision = %#v", decision)
	}
	withoutSelector := withScope
	withoutSelector.CapabilitySelector = nil
	decision, err = classification.Classify(&withoutSelector, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeBounded || !hasReason(decision, "CAPABILITY_SELECTION_REQUIRED") || !hasEvidence(decision, classification.EvidenceCapabilitySelector) {
		t.Fatalf("missing selector decision = %#v", decision)
	}
}

func TestClassifyWorkflowTriggersFromSemanticTraits(t *testing.T) {
	for _, trait := range []classification.Trait{
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
		t.Run(string(trait), func(t *testing.T) {
			proposal := completeProposal()
			setTrait(&proposal, trait, classification.TraitTrue)
			decision, err := classification.Classify(&proposal, classification.ClassificationRules{})
			if err != nil {
				t.Fatal(err)
			}
			if decision.RequestMode != classification.RequestModeWorkflow || decision.WorkflowComplexity == nil || *decision.WorkflowComplexity != classification.ComplexityComplex {
				t.Fatalf("decision = %#v", decision)
			}
		})
	}
}

func TestClassifyMissingOrUncertainCriticalTraitsFailsUpward(t *testing.T) {
	missing := completeProposal()
	missing.Traits = missing.Traits[:len(missing.Traits)-1]
	decision, err := classification.Classify(&missing, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeWorkflow || !hasReason(decision, "CLASSIFICATION_TRAIT_MISSING") {
		t.Fatalf("missing trait decision = %#v", decision)
	}
	uncertain := completeProposal()
	setTrait(&uncertain, classification.TraitScopeClear, classification.TraitUnknown)
	decision, err = classification.Classify(&uncertain, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeWorkflow || !hasReason(decision, "CLASSIFICATION_TRAIT_UNCERTAIN") {
		t.Fatalf("uncertain trait decision = %#v", decision)
	}
	decision, err = classification.Classify(nil, classification.ClassificationRules{})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeWorkflow || !hasReason(decision, "CLASSIFICATION_UNAVAILABLE") {
		t.Fatalf("nil proposal decision = %#v", decision)
	}
}

func setTrait(proposal *classification.ClassificationProposal, wanted classification.Trait, value classification.TraitValue) {
	for i := range proposal.Traits {
		if proposal.Traits[i].Trait == wanted {
			proposal.Traits[i].Value = value
			return
		}
	}
	proposal.Traits = append(proposal.Traits, classification.TraitObservation{Trait: wanted, Value: value})
}

func hasReason(decision classification.ClassificationDecision, wanted string) bool {
	for _, reason := range decision.EscalationReasons {
		if reason == wanted {
			return true
		}
	}
	return false
}

func hasEvidence(decision classification.ClassificationDecision, wanted classification.EvidenceKind) bool {
	for _, requirement := range decision.EvidenceRequirements {
		if requirement.Kind == wanted {
			return true
		}
	}
	return false
}

func completeProposal() classification.ClassificationProposal {
	traits := make([]classification.TraitObservation, 0, len(allTraits()))
	for _, trait := range allTraits() {
		traits = append(traits, classification.TraitObservation{Trait: trait, Value: classification.TraitFalse})
	}
	return classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1,
		Traits:        traits,
		Resources:     []classification.Resource{},
		Evidence: []classification.ProposalEvidence{
			{Kind: classification.EvidenceVerification, Reference: "test:verification", Digest: strings.Repeat("b", 64)},
			{Kind: classification.EvidenceScope, Reference: "test:scope", Digest: strings.Repeat("a", 64)},
			{Kind: classification.EvidenceChangePoint, Reference: "test:change-point", Digest: strings.Repeat("c", 64)},
		},
	}
}

func allTraits() []classification.Trait {
	return []classification.Trait{
		classification.TraitArchitectureDecision,
		classification.TraitBoundedCapabilityRequest,
		classification.TraitChangePointKnown,
		classification.TraitCriticalRelease,
		classification.TraitDataSensitive,
		classification.TraitDependencyChange,
		classification.TraitDeploymentChange,
		classification.TraitDestructiveMutation,
		classification.TraitDomainUncertainty,
		classification.TraitFocusedVerificationKnown,
		classification.TraitLongLivedDelegation,
		classification.TraitMultipleResponsibilities,
		classification.TraitMultipleTickets,
		classification.TraitPublicContractChange,
		classification.TraitRecoverable,
		classification.TraitRootCauseUncertain,
		classification.TraitSchemaChange,
		classification.TraitScopeClear,
		classification.TraitSecuritySensitive,
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
