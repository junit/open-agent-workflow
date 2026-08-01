package classification_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

func TestCriticalReleaseEvaluationCorpusRequiresWorkflow(t *testing.T) {
	cases := []struct {
		name      string
		traits    []classification.Trait
		resources []classification.Resource
	}{
		{name: "public-contract-release", traits: []classification.Trait{classification.TraitPublicContractChange}},
		{name: "schema-migration", traits: []classification.Trait{classification.TraitSchemaChange}},
		{name: "dependency-upgrade", traits: []classification.Trait{classification.TraitDependencyChange}},
		{name: "security-sensitive-mutation", traits: []classification.Trait{classification.TraitSecuritySensitive}},
		{name: "credential-data-operation", traits: []classification.Trait{classification.TraitDataSensitive}, resources: []classification.Resource{classification.ResourceCredentials}},
		{name: "deployment-change", traits: []classification.Trait{classification.TraitDeploymentChange}},
		{name: "destructive-migration", traits: []classification.Trait{classification.TraitDestructiveMutation}},
		{name: "unresolved-architecture", traits: []classification.Trait{classification.TraitArchitectureDecision, classification.TraitDomainUncertainty}},
		{name: "multi-ticket-delegation", traits: []classification.Trait{classification.TraitMultipleTickets, classification.TraitLongLivedDelegation}},
		{name: "critical-release", traits: []classification.Trait{classification.TraitCriticalRelease}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			proposal := clearDirectProposal()
			for _, trait := range tt.traits {
				setTrait(&proposal, trait, classification.TraitTrue)
			}
			proposal.Resources = append(proposal.Resources, tt.resources...)
			decision, err := classification.Classify(&proposal, classification.ClassificationRules{})
			if err != nil {
				t.Fatal(err)
			}
			if decision.RequestMode == classification.RequestModeDirect || decision.RequestMode == classification.RequestModeBounded {
				t.Fatalf("critical corpus admitted unsafe mode: %#v", decision)
			}
			if decision.RequestMode != classification.RequestModeWorkflow || decision.WorkflowComplexity == nil ||
				*decision.WorkflowComplexity != classification.ComplexityComplex {
				t.Fatalf("critical corpus decision = %#v", decision)
			}
		})
	}
}

func TestClassificationPolicyMonotonicityInvariant(t *testing.T) {
	proposals := map[string]classification.ClassificationProposal{
		"direct":   clearDirectProposal(),
		"bounded":  boundedEvalProposal(),
		"workflow": workflowEvalProposal(),
	}
	modes := []classification.RequestMode{classification.RequestModeDirect, classification.RequestModeBounded, classification.RequestModeWorkflow}
	risks := []classification.RiskClass{classification.RiskNormal, classification.RiskElevated, classification.RiskCritical}
	for name, proposal := range proposals {
		base, err := classification.Classify(&proposal, classification.ClassificationRules{})
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range modes {
			for _, risk := range risks {
				layer := classification.PolicyLayer{MinimumMode: mode, MinimumRisk: risk}
				userFirst, err := classification.Classify(&proposal, classification.ClassificationRules{User: layer})
				if err != nil {
					t.Fatal(err)
				}
				projectFirst, err := classification.Classify(&proposal, classification.ClassificationRules{Project: layer})
				if err != nil {
					t.Fatal(err)
				}
				if requestModeRank(userFirst.RequestMode) < requestModeRank(base.RequestMode) || riskClassRank(userFirst.RiskClass) < riskClassRank(base.RiskClass) {
					t.Fatalf("%s policy lowered base %#v to %#v", name, base, userFirst)
				}
				if userFirst.Digest() != projectFirst.Digest() {
					t.Fatalf("%s equivalent layer placement changed digest: %s != %s", name, userFirst.Digest(), projectFirst.Digest())
				}
			}
		}
	}
}

func FuzzDecodeProposalFailsClosed(f *testing.F) {
	valid, err := json.Marshal(clearDirectProposal())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(valid)
	f.Add([]byte(`{}`))
	f.Add([]byte("{\"schema_version\":\"\xff\"}"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		proposal, err := classification.DecodeProposal(raw)
		if err != nil {
			return
		}
		before, err := json.Marshal(proposal)
		if err != nil {
			t.Fatal(err)
		}
		first, err := classification.Classify(&proposal, classification.ClassificationRules{})
		if err != nil {
			t.Fatal(err)
		}
		second, err := classification.Classify(&proposal, classification.ClassificationRules{})
		if err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal(proposal)
		if err != nil {
			t.Fatal(err)
		}
		if first.Digest() == "" || first.Digest() != second.Digest() || string(before) != string(after) {
			t.Fatalf("classification is not deterministic or mutated input: %s / %s", first.Digest(), second.Digest())
		}
	})
}

func boundedEvalProposal() classification.ClassificationProposal {
	proposal := clearDirectProposal()
	setTrait(&proposal, classification.TraitBoundedCapabilityRequest, classification.TraitTrue)
	proposal.CapabilitySelector = &classification.CapabilitySelector{
		ProviderID: "acme/suite", CapabilityID: "review", Source: classification.SelectorUserIntent,
	}
	proposal.Evidence = append(proposal.Evidence, classification.ProposalEvidence{
		Kind: classification.EvidenceCapabilitySelector, Reference: "eval:selector", Digest: strings.Repeat("d", 64),
	})
	return proposal
}

func workflowEvalProposal() classification.ClassificationProposal {
	proposal := clearDirectProposal()
	setTrait(&proposal, classification.TraitCriticalRelease, classification.TraitTrue)
	return proposal
}

func requestModeRank(value classification.RequestMode) int {
	switch value {
	case classification.RequestModeDirect:
		return 1
	case classification.RequestModeBounded:
		return 2
	case classification.RequestModeWorkflow:
		return 3
	default:
		return 0
	}
}

func riskClassRank(value classification.RiskClass) int {
	switch value {
	case classification.RiskNormal:
		return 1
	case classification.RiskElevated:
		return 2
	case classification.RiskCritical:
		return 3
	default:
		return 0
	}
}
