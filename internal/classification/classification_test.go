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

func TestDecodeProposalRejectsInvalidUTF8AndOversizedInput(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		code string
	}{
		{"invalid UTF-8", []byte("{\"schema_version\":\"\xff\"}"), "CLASSIFICATION_JSON_INVALID"},
		{"oversized input", make([]byte, (1<<20)+1), "CLASSIFICATION_TOO_LARGE"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := classification.DecodeProposal(tt.raw); err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("DecodeProposal() error = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestTypedProposalLimitsFailUpward(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*classification.ClassificationProposal)
		reason string
	}{
		{"too many evidence records", func(value *classification.ClassificationProposal) {
			value.Evidence = make([]classification.ProposalEvidence, 129)
		}, "CLASSIFICATION_EVIDENCE_LIMIT_EXCEEDED"},
		{"evidence reference too long", func(value *classification.ClassificationProposal) {
			value.Evidence[0].Reference = strings.Repeat("x", 513)
		}, "CLASSIFICATION_EVIDENCE_INVALID"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			proposal := clearDirectProposal()
			tt.mutate(&proposal)
			decision, err := classification.Classify(&proposal, classification.ClassificationRules{})
			if err != nil {
				t.Fatal(err)
			}
			if decision.RequestMode != classification.RequestModeWorkflow || !hasReason(decision, tt.reason) {
				t.Fatalf("typed proposal fallback = %#v", decision)
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

func TestClassifyMalformedProposalFallsBackConservatively(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*classification.ClassificationProposal)
		reason string
	}{
		{"unsupported schema", func(value *classification.ClassificationProposal) {
			value.SchemaVersion = "oaw.classification-proposal/v2"
		}, "UNSUPPORTED_CLASSIFICATION_SCHEMA"},
		{"unknown trait", func(value *classification.ClassificationProposal) {
			value.Traits[0].Trait = "future-trait"
		}, "CLASSIFICATION_TRAIT_UNKNOWN"},
		{"duplicate trait", func(value *classification.ClassificationProposal) {
			value.Traits[1] = value.Traits[0]
		}, "CLASSIFICATION_DUPLICATE_TRAIT"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			proposal := clearDirectProposal()
			tt.mutate(&proposal)
			decision, err := classification.Classify(&proposal, classification.ClassificationRules{})
			if err != nil {
				t.Fatal(err)
			}
			if decision.RequestMode != classification.RequestModeWorkflow || decision.RiskClass != classification.RiskCritical || !hasReason(decision, tt.reason) {
				t.Fatalf("fallback decision = %#v", decision)
			}
		})
	}
}

func TestClassifyResourceSafetyFloors(t *testing.T) {
	cases := []struct {
		resource classification.Resource
		reason   string
		risk     classification.RiskClass
	}{
		{classification.ResourcePublicAPI, "WORKFLOW_REQUIRED_PUBLIC_CONTRACT", classification.RiskElevated},
		{classification.ResourceSchema, "WORKFLOW_REQUIRED_SCHEMA", classification.RiskElevated},
		{classification.ResourceDependency, "WORKFLOW_REQUIRED_DEPENDENCY", classification.RiskElevated},
		{classification.ResourceSecurity, "WORKFLOW_REQUIRED_SECURITY", classification.RiskElevated},
		{classification.ResourceCredentials, "WORKFLOW_REQUIRED_SECURITY", classification.RiskCritical},
		{classification.ResourceData, "WORKFLOW_REQUIRED_DATA", classification.RiskElevated},
		{classification.ResourceDeployment, "WORKFLOW_REQUIRED_DEPLOYMENT", classification.RiskElevated},
		{classification.ResourceDestructive, "WORKFLOW_REQUIRED_DESTRUCTIVE", classification.RiskCritical},
	}
	for _, tt := range cases {
		t.Run(string(tt.resource), func(t *testing.T) {
			proposal := clearDirectProposal()
			proposal.Resources = append(proposal.Resources, tt.resource)
			decision, err := classification.Classify(&proposal, classification.ClassificationRules{})
			if err != nil {
				t.Fatal(err)
			}
			if decision.RequestMode != classification.RequestModeWorkflow || decision.RiskClass != tt.risk || !hasReason(decision, tt.reason) {
				t.Fatalf("resource decision = %#v", decision)
			}
		})
	}
}

func TestUserAndProjectRulesOnlyRaiseModeRiskAndEvidence(t *testing.T) {
	proposal := clearDirectProposal()
	rules := classification.ClassificationRules{
		User: classification.PolicyLayer{
			MinimumMode:      classification.RequestModeBounded,
			MinimumRisk:      classification.RiskElevated,
			RequiredEvidence: []classification.EvidenceKind{classification.EvidenceNegativeTest, classification.EvidenceArchitecture},
		},
		Project: classification.PolicyLayer{
			MinimumMode:      classification.RequestModeWorkflow,
			MinimumRisk:      classification.RiskNormal,
			RequiredEvidence: []classification.EvidenceKind{classification.EvidenceArchitecture, classification.EvidenceNegativeTest},
		},
	}
	decision, err := classification.Classify(&proposal, rules)
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeWorkflow || decision.WorkflowComplexity == nil || *decision.WorkflowComplexity != classification.ComplexityComplex {
		t.Fatalf("decision = %#v", decision)
	}
	if decision.RiskClass != classification.RiskElevated || decision.CapabilitySelector != nil {
		t.Fatalf("policy result = %#v", decision)
	}
	assertEvidenceKinds(t, decision, []classification.EvidenceKind{
		classification.EvidenceArchitecture,
		classification.EvidenceChangePoint,
		classification.EvidenceNegativeTest,
		classification.EvidenceScope,
		classification.EvidenceVerification,
	})

	critical := clearDirectProposal()
	setTrait(&critical, classification.TraitCriticalRelease, classification.TraitTrue)
	lowered, err := classification.Classify(&critical, classification.ClassificationRules{
		User:    classification.PolicyLayer{MinimumMode: classification.RequestModeDirect, MinimumRisk: classification.RiskNormal},
		Project: classification.PolicyLayer{MinimumMode: classification.RequestModeBounded, MinimumRisk: classification.RiskElevated},
	})
	if err != nil {
		t.Fatal(err)
	}
	if lowered.RequestMode != classification.RequestModeWorkflow || lowered.RiskClass != classification.RiskCritical {
		t.Fatalf("lowering rules changed built-in floor: %#v", lowered)
	}
}

func TestProtectedResourcesRaiseWorkflowWithoutSelectingProvider(t *testing.T) {
	proposal := clearDirectProposal()
	proposal.Resources = append(proposal.Resources, classification.ResourceNetwork)
	decision, err := classification.Classify(&proposal, classification.ClassificationRules{
		Project: classification.PolicyLayer{ProtectedResources: []classification.Resource{classification.ResourceNetwork}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if decision.RequestMode != classification.RequestModeWorkflow || decision.CapabilitySelector != nil || !hasReason(decision, "POLICY_PROTECTED_RESOURCE") {
		t.Fatalf("protected resource decision = %#v", decision)
	}
}

func TestPolicyRulesAreOrderIndependentAndDigestStable(t *testing.T) {
	proposal := clearDirectProposal()
	proposal.Resources = append(proposal.Resources, classification.ResourceNetwork)
	proposal.Evidence = append(proposal.Evidence, classification.ProposalEvidence{
		Kind: classification.EvidenceAuthorization, Reference: "test:authorization", Digest: strings.Repeat("d", 64),
	})
	minimum := classification.PolicyLayer{
		MinimumMode:      classification.RequestModeWorkflow,
		RequiredEvidence: []classification.EvidenceKind{classification.EvidenceNegativeTest, classification.EvidenceAuthorization},
	}
	protected := classification.PolicyLayer{
		ProtectedResources: []classification.Resource{classification.ResourceNetwork},
		RequiredEvidence:   []classification.EvidenceKind{classification.EvidenceAuthorization, classification.EvidenceNegativeTest},
	}
	first, err := classification.Classify(&proposal, classification.ClassificationRules{User: minimum, Project: protected})
	if err != nil {
		t.Fatal(err)
	}
	second, err := classification.Classify(&proposal, classification.ClassificationRules{User: protected, Project: minimum})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != second.Digest() || !decisionsEqual(first, second) {
		t.Fatalf("policy order changed decision:\nfirst:  %#v\nsecond: %#v", first, second)
	}
}

func TestPolicyValidationRejectsOpenOrDuplicateValues(t *testing.T) {
	cases := []struct {
		name  string
		layer classification.PolicyLayer
		code  string
	}{
		{"mode", classification.PolicyLayer{MinimumMode: "AUTO"}, "CLASSIFICATION_POLICY_MODE_INVALID"},
		{"risk", classification.PolicyLayer{MinimumRisk: "severe"}, "CLASSIFICATION_POLICY_RISK_INVALID"},
		{"resource", classification.PolicyLayer{ProtectedResources: []classification.Resource{"filesystem"}}, "CLASSIFICATION_POLICY_RESOURCE_INVALID"},
		{"duplicate resource", classification.PolicyLayer{ProtectedResources: []classification.Resource{classification.ResourceData, classification.ResourceData}}, "CLASSIFICATION_POLICY_RESOURCE_DUPLICATE"},
		{"evidence", classification.PolicyLayer{RequiredEvidence: []classification.EvidenceKind{"approval"}}, "CLASSIFICATION_POLICY_EVIDENCE_INVALID"},
		{"duplicate evidence", classification.PolicyLayer{RequiredEvidence: []classification.EvidenceKind{classification.EvidenceScope, classification.EvidenceScope}}, "CLASSIFICATION_POLICY_EVIDENCE_DUPLICATE"},
	}
	proposal := clearDirectProposal()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := classification.Classify(&proposal, classification.ClassificationRules{User: tt.layer})
			if err == nil || !strings.Contains(err.Error(), tt.code) {
				t.Fatalf("Classify() error = %v, want %s", err, tt.code)
			}
		})
	}
}

func TestPolicyModeProjectionPreservesModeSpecificFields(t *testing.T) {
	direct := clearDirectProposal()
	bounded, err := classification.Classify(&direct, classification.ClassificationRules{
		Project: classification.PolicyLayer{MinimumMode: classification.RequestModeBounded},
	})
	if err != nil {
		t.Fatal(err)
	}
	if bounded.RequestMode != classification.RequestModeBounded || bounded.WorkflowComplexity != nil || bounded.CapabilitySelector != nil ||
		!hasReason(bounded, "CAPABILITY_SELECTION_REQUIRED") || !hasEvidence(bounded, classification.EvidenceCapabilitySelector) {
		t.Fatalf("bounded policy projection = %#v", bounded)
	}

	proposal := clearDirectProposal()
	setTrait(&proposal, classification.TraitBoundedCapabilityRequest, classification.TraitTrue)
	proposal.CapabilitySelector = &classification.CapabilitySelector{
		ProviderID: "acme/suite", CapabilityID: "review", Source: classification.SelectorUserIntent,
	}
	workflow, err := classification.Classify(&proposal, classification.ClassificationRules{
		Project: classification.PolicyLayer{MinimumMode: classification.RequestModeWorkflow},
	})
	if err != nil {
		t.Fatal(err)
	}
	if workflow.RequestMode != classification.RequestModeWorkflow || workflow.WorkflowComplexity == nil || workflow.CapabilitySelector != nil {
		t.Fatalf("workflow policy projection = %#v", workflow)
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

func assertEvidenceKinds(t *testing.T, decision classification.ClassificationDecision, want []classification.EvidenceKind) {
	t.Helper()
	if len(decision.EvidenceRequirements) != len(want) {
		t.Fatalf("evidence requirements = %#v, want %#v", decision.EvidenceRequirements, want)
	}
	for i, requirement := range decision.EvidenceRequirements {
		if requirement.Kind != want[i] {
			t.Fatalf("evidence requirements = %#v, want %#v", decision.EvidenceRequirements, want)
		}
	}
}

func decisionsEqual(left, right classification.ClassificationDecision) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func clearDirectProposal() classification.ClassificationProposal {
	proposal := completeProposal()
	setTrait(&proposal, classification.TraitScopeClear, classification.TraitTrue)
	setTrait(&proposal, classification.TraitChangePointKnown, classification.TraitTrue)
	setTrait(&proposal, classification.TraitRecoverable, classification.TraitTrue)
	setTrait(&proposal, classification.TraitFocusedVerificationKnown, classification.TraitTrue)
	return proposal
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
