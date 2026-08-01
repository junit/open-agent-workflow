package classification_test

import (
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
