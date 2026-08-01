package integration

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
)

func TestTicket03EmptyConfigProjectionRaisesClassificationMonotonically(t *testing.T) {
	snapshot, err := config.Load(config.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Digest() == "" {
		t.Fatal("empty configuration snapshot has no digest")
	}

	proposal := integrationDirectProposal()
	projectRaised, err := classification.Classify(&proposal, classification.ClassificationRules{
		Project: classification.PolicyLayer{MinimumMode: classification.RequestModeWorkflow},
	})
	if err != nil {
		t.Fatal(err)
	}
	laterLowered, err := classification.Classify(&proposal, classification.ClassificationRules{
		User:    classification.PolicyLayer{MinimumMode: classification.RequestModeWorkflow},
		Project: classification.PolicyLayer{MinimumMode: classification.RequestModeDirect},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range []classification.ClassificationDecision{projectRaised, laterLowered} {
		if decision.RequestMode != classification.RequestModeWorkflow || decision.WorkflowComplexity == nil ||
			*decision.WorkflowComplexity != classification.ComplexityComplex || decision.CapabilitySelector != nil {
			t.Fatalf("policy projection decision = %#v", decision)
		}
	}
	if projectRaised.Digest() != laterLowered.Digest() {
		t.Fatalf("equivalent policy projections changed digest: %s != %s", projectRaised.Digest(), laterLowered.Digest())
	}
}

func integrationDirectProposal() classification.ClassificationProposal {
	trueTraits := map[classification.Trait]bool{
		classification.TraitScopeClear:               true,
		classification.TraitChangePointKnown:         true,
		classification.TraitRecoverable:              true,
		classification.TraitFocusedVerificationKnown: true,
	}
	traits := make([]classification.TraitObservation, 0, len(integrationCriticalTraits()))
	for _, trait := range integrationCriticalTraits() {
		value := classification.TraitFalse
		if trueTraits[trait] {
			value = classification.TraitTrue
		}
		traits = append(traits, classification.TraitObservation{Trait: trait, Value: value})
	}
	return classification.ClassificationProposal{
		SchemaVersion: classification.ProposalSchemaV1,
		Traits:        traits,
		Resources:     []classification.Resource{},
		Evidence: []classification.ProposalEvidence{
			{Kind: classification.EvidenceScope, Reference: "integration:scope", Digest: strings.Repeat("a", 64)},
			{Kind: classification.EvidenceChangePoint, Reference: "integration:change-point", Digest: strings.Repeat("b", 64)},
			{Kind: classification.EvidenceVerification, Reference: "integration:verification", Digest: strings.Repeat("c", 64)},
		},
	}
}

func integrationCriticalTraits() []classification.Trait {
	return []classification.Trait{
		classification.TraitScopeClear,
		classification.TraitChangePointKnown,
		classification.TraitRecoverable,
		classification.TraitFocusedVerificationKnown,
		classification.TraitBoundedCapabilityRequest,
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
	}
}
