package core

import "github.com/wifibaby4u/open-agent-workflow/internal/classification"

func Classify(proposal *classification.ClassificationProposal, rules classification.ClassificationRules) (classification.ClassificationDecision, error) {
	return classification.Classify(proposal, rules)
}
