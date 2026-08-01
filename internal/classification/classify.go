package classification

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

const decisionSchemaV1 = "oaw.classification-decision/v1"

func Classify(proposal *ClassificationProposal, rules ClassificationRules) (ClassificationDecision, error) {
	if err := validateRules(rules); err != nil {
		return ClassificationDecision{}, err
	}
	if proposal == nil {
		return fallbackDecision([]string{"CLASSIFICATION_UNAVAILABLE"})
	}
	value := cloneProposal(*proposal)
	if err := normalizeProposal(&value); err != nil {
		return fallbackDecision([]string{classificationReason(err)})
	}
	traits, missing, uncertain := traitValues(value.Traits)
	if missing {
		return fallbackDecision([]string{"CLASSIFICATION_TRAIT_MISSING"})
	}
	if uncertain {
		return fallbackDecision([]string{"CLASSIFICATION_TRAIT_UNCERTAIN"})
	}

	reasons := []string{}
	requirements := []EvidenceRequirement{}
	mode := RequestModeDirect
	risk := RiskNormal
	workflow := false
	for _, trait := range allCriticalTraits() {
		if traits[trait] != TraitTrue {
			continue
		}
		if reason := workflowReasonForTrait(trait); reason != "" {
			workflow = true
			reasons = append(reasons, reason)
		}
		if riskRank(traitRisk(trait)) > riskRank(risk) {
			risk = traitRisk(trait)
		}
		if kind, reason := evidenceForTrait(trait); kind != "" {
			requirements = appendRequirement(requirements, EvidenceRequirement{Kind: kind, Reason: reason})
		}
	}
	for _, resource := range value.Resources {
		if reason := workflowReasonForResource(resource); reason != "" {
			workflow = true
			reasons = append(reasons, reason)
		}
		if resource == ResourceDestructive || resource == ResourceCredentials {
			risk = maxRisk(risk, RiskCritical)
		} else if resource != ResourceProject && resource != ResourceWorktree && resource != ResourceGitRepository {
			risk = maxRisk(risk, RiskElevated)
		}
	}

	bounded := traits[TraitBoundedCapabilityRequest] == TraitTrue
	if !bounded {
		if traits[TraitScopeClear] != TraitTrue || traits[TraitChangePointKnown] != TraitTrue || traits[TraitRecoverable] != TraitTrue {
			workflow = true
			reasons = append(reasons, "DIRECT_SCOPE_UNCLEAR")
		}
		if traits[TraitFocusedVerificationKnown] != TraitTrue {
			workflow = true
			reasons = append(reasons, "DIRECT_VERIFICATION_REQUIRED")
		}
	}
	if bounded {
		mode = RequestModeBounded
		if value.CapabilitySelector == nil {
			reasons = append(reasons, "CAPABILITY_SELECTION_REQUIRED")
			requirements = appendRequirement(requirements, EvidenceRequirement{Kind: EvidenceCapabilitySelector, Reason: "bounded capability selection"})
		}
	} else if !workflow {
		mode = RequestModeDirect
	}
	if workflow {
		mode = RequestModeWorkflow
	}
	if mode != RequestModeWorkflow {
		baseRequirements := []EvidenceRequirement{}
		addBaseEvidenceRequirements(&baseRequirements)
		for _, requirement := range baseRequirements {
			requirements = appendRequirement(requirements, requirement)
		}
		if missingEvidence(value.Evidence, baseRequirements) {
			workflow = true
			mode = RequestModeWorkflow
			reasons = append(reasons, "DIRECT_VERIFICATION_REQUIRED")
		}
	}
	if mode == RequestModeWorkflow {
		complexity := ComplexityComplex
		return applyRules(ClassificationDecision{RequestMode: mode, WorkflowComplexity: &complexity, RiskClass: risk, EvidenceRequirements: requirements, EscalationReasons: reasons}, value, rules)
	}
	decision := ClassificationDecision{RequestMode: mode, RiskClass: risk, EvidenceRequirements: requirements, EscalationReasons: reasons}
	if bounded && value.CapabilitySelector != nil {
		selector := *value.CapabilitySelector
		decision.CapabilitySelector = &selector
	}
	return applyRules(decision, value, rules)
}

func applyRules(decision ClassificationDecision, proposal ClassificationProposal, rules ClassificationRules) (ClassificationDecision, error) {
	layers := []PolicyLayer{rules.User, rules.Project}
	for _, layer := range layers {
		for _, resource := range layer.ProtectedResources {
			if containsResource(proposal.Resources, resource) {
				if decision.RequestMode != RequestModeWorkflow {
					decision.RequestMode = RequestModeWorkflow
					complexity := ComplexityComplex
					decision.WorkflowComplexity = &complexity
					decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_PROTECTED_RESOURCE")
				}
				decision.RiskClass = maxRisk(decision.RiskClass, RiskElevated)
			}
		}
		if modeRank(layer.MinimumMode) > modeRank(decision.RequestMode) {
			decision.RequestMode = layer.MinimumMode
			complexity := ComplexityComplex
			decision.WorkflowComplexity = &complexity
			decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_MINIMUM_MODE")
		}
		if riskRank(layer.MinimumRisk) > riskRank(decision.RiskClass) {
			decision.RiskClass = layer.MinimumRisk
			decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_MINIMUM_RISK")
		}
		for _, kind := range layer.RequiredEvidence {
			decision.EvidenceRequirements = appendRequirement(decision.EvidenceRequirements, EvidenceRequirement{Kind: kind, Reason: "policy-required evidence"})
			if !hasEvidenceKind(proposal.Evidence, kind) && decision.RequestMode != RequestModeWorkflow {
				decision.RequestMode = RequestModeWorkflow
				complexity := ComplexityComplex
				decision.WorkflowComplexity = &complexity
				decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_EVIDENCE_REQUIRED")
			}
		}
	}
	if decision.RequestMode == RequestModeWorkflow && decision.WorkflowComplexity == nil {
		complexity := ComplexityComplex
		decision.WorkflowComplexity = &complexity
	}
	decision.EvidenceRequirements = sortRequirements(decision.EvidenceRequirements)
	decision.EscalationReasons = uniqueSortedStrings(decision.EscalationReasons)
	return withDecisionDigest(decision)
}

func fallbackDecision(reasons []string) (ClassificationDecision, error) {
	complexity := ComplexityComplex
	decision := ClassificationDecision{
		RequestMode: RequestModeWorkflow, WorkflowComplexity: &complexity, RiskClass: RiskCritical,
		EvidenceRequirements: []EvidenceRequirement{{Kind: EvidenceScope, Reason: "classification input"}, {Kind: EvidenceVerification, Reason: "classification input"}},
		EscalationReasons:    reasons,
	}
	return withDecisionDigest(decision)
}

func withDecisionDigest(decision ClassificationDecision) (ClassificationDecision, error) {
	decision.EvidenceRequirements = sortRequirements(decision.EvidenceRequirements)
	decision.EscalationReasons = uniqueSortedStrings(decision.EscalationReasons)
	record := struct {
		SchemaVersion        string                `json:"schema_version"`
		RequestMode          RequestMode           `json:"request_mode"`
		WorkflowComplexity   *WorkflowComplexity   `json:"workflow_complexity"`
		RiskClass            RiskClass             `json:"risk_class"`
		EvidenceRequirements []EvidenceRequirement `json:"evidence_requirements"`
		EscalationReasons    []string              `json:"escalation_reasons"`
		CapabilitySelector   *CapabilitySelector   `json:"capability_selector,omitempty"`
	}{decisionSchemaV1, decision.RequestMode, decision.WorkflowComplexity, decision.RiskClass, decision.EvidenceRequirements, decision.EscalationReasons, decision.CapabilitySelector}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return ClassificationDecision{}, err
	}
	decision.digest = digest
	return decision, nil
}

func validateRules(rules ClassificationRules) error {
	for _, layer := range []PolicyLayer{rules.User, rules.Project} {
		if layer.MinimumMode != "" && modeRank(layer.MinimumMode) == 0 {
			return fmt.Errorf("CLASSIFICATION_POLICY_MODE_INVALID: %q", layer.MinimumMode)
		}
		if layer.MinimumRisk != "" && riskRank(layer.MinimumRisk) == 0 {
			return fmt.Errorf("CLASSIFICATION_POLICY_RISK_INVALID: %q", layer.MinimumRisk)
		}
		seenResources := make(map[Resource]struct{}, len(layer.ProtectedResources))
		for _, resource := range layer.ProtectedResources {
			if !knownResource(resource) {
				return fmt.Errorf("CLASSIFICATION_POLICY_RESOURCE_INVALID: %q", resource)
			}
			if _, found := seenResources[resource]; found {
				return fmt.Errorf("CLASSIFICATION_POLICY_RESOURCE_DUPLICATE: %s", resource)
			}
			seenResources[resource] = struct{}{}
		}
		seenEvidence := make(map[EvidenceKind]struct{}, len(layer.RequiredEvidence))
		for _, kind := range layer.RequiredEvidence {
			if !knownEvidence(kind) {
				return fmt.Errorf("CLASSIFICATION_POLICY_EVIDENCE_INVALID: %q", kind)
			}
			if _, found := seenEvidence[kind]; found {
				return fmt.Errorf("CLASSIFICATION_POLICY_EVIDENCE_DUPLICATE: %s", kind)
			}
			seenEvidence[kind] = struct{}{}
		}
	}
	return nil
}

func traitValues(values []TraitObservation) (map[Trait]TraitValue, bool, bool) {
	result := make(map[Trait]TraitValue, len(values))
	for _, value := range values {
		result[value.Trait] = value.Value
	}
	missing, uncertain := false, false
	for _, trait := range allCriticalTraits() {
		value, found := result[trait]
		if !found {
			missing = true
		} else if value == TraitUnknown {
			uncertain = true
		}
	}
	return result, missing, uncertain
}

func classificationReason(err error) string {
	message := err.Error()
	if index := strings.IndexByte(message, ':'); index > 0 {
		return message[:index]
	}
	return "CLASSIFICATION_UNAVAILABLE"
}

func workflowReasonForTrait(trait Trait) string {
	switch trait {
	case TraitArchitectureDecision:
		return "WORKFLOW_REQUIRED_ARCHITECTURE"
	case TraitPublicContractChange:
		return "WORKFLOW_REQUIRED_PUBLIC_CONTRACT"
	case TraitSchemaChange:
		return "WORKFLOW_REQUIRED_SCHEMA"
	case TraitDependencyChange:
		return "WORKFLOW_REQUIRED_DEPENDENCY"
	case TraitSecuritySensitive:
		return "WORKFLOW_REQUIRED_SECURITY"
	case TraitDataSensitive:
		return "WORKFLOW_REQUIRED_DATA"
	case TraitDeploymentChange:
		return "WORKFLOW_REQUIRED_DEPLOYMENT"
	case TraitDomainUncertainty, TraitRootCauseUncertain:
		return "WORKFLOW_REQUIRED_UNRESOLVED"
	case TraitMultipleResponsibilities:
		return "WORKFLOW_REQUIRED_MULTIPLE_RESPONSIBILITIES"
	case TraitMultipleTickets:
		return "WORKFLOW_REQUIRED_MULTIPLE_TICKETS"
	case TraitLongLivedDelegation:
		return "WORKFLOW_REQUIRED_DELEGATION"
	case TraitDestructiveMutation:
		return "WORKFLOW_REQUIRED_DESTRUCTIVE"
	case TraitCriticalRelease:
		return "WORKFLOW_REQUIRED_CRITICAL_RELEASE"
	default:
		return ""
	}
}

func workflowReasonForResource(resource Resource) string {
	switch resource {
	case ResourcePublicAPI:
		return "WORKFLOW_REQUIRED_PUBLIC_CONTRACT"
	case ResourceSchema:
		return "WORKFLOW_REQUIRED_SCHEMA"
	case ResourceDependency:
		return "WORKFLOW_REQUIRED_DEPENDENCY"
	case ResourceSecurity, ResourceCredentials:
		return "WORKFLOW_REQUIRED_SECURITY"
	case ResourceData:
		return "WORKFLOW_REQUIRED_DATA"
	case ResourceDeployment:
		return "WORKFLOW_REQUIRED_DEPLOYMENT"
	case ResourceDestructive:
		return "WORKFLOW_REQUIRED_DESTRUCTIVE"
	default:
		return ""
	}
}

func traitRisk(trait Trait) RiskClass {
	switch trait {
	case TraitCriticalRelease, TraitDestructiveMutation:
		return RiskCritical
	case TraitSecuritySensitive, TraitDataSensitive, TraitDeploymentChange, TraitDependencyChange, TraitSchemaChange, TraitPublicContractChange, TraitArchitectureDecision:
		return RiskElevated
	default:
		return RiskNormal
	}
}

func evidenceForTrait(trait Trait) (EvidenceKind, string) {
	switch trait {
	case TraitArchitectureDecision:
		return EvidenceArchitecture, "architecture decision"
	case TraitSecuritySensitive, TraitDataSensitive:
		return EvidenceSecurityAcceptance, "sensitive change"
	case TraitDestructiveMutation:
		return EvidenceRecovery, "destructive change"
	case TraitCriticalRelease:
		return EvidenceNegativeTest, "critical release"
	default:
		return "", ""
	}
}

func addBaseEvidenceRequirements(values *[]EvidenceRequirement) {
	*values = appendRequirement(*values, EvidenceRequirement{Kind: EvidenceScope, Reason: "bounded scope"})
	*values = appendRequirement(*values, EvidenceRequirement{Kind: EvidenceChangePoint, Reason: "known change point"})
	*values = appendRequirement(*values, EvidenceRequirement{Kind: EvidenceVerification, Reason: "focused verification"})
}

func appendRequirement(values []EvidenceRequirement, value EvidenceRequirement) []EvidenceRequirement {
	for _, existing := range values {
		if existing.Kind == value.Kind {
			return values
		}
	}
	return append(values, value)
}

func sortRequirements(values []EvidenceRequirement) []EvidenceRequirement {
	result := append([]EvidenceRequirement{}, values...)
	sort.Slice(result, func(i, j int) bool { return result[i].Kind < result[j].Kind })
	return result
}

func uniqueSortedStrings(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	unique := result[:0]
	for _, value := range result {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique
}

func missingEvidence(values []ProposalEvidence, requirements []EvidenceRequirement) bool {
	for _, requirement := range requirements {
		if !hasEvidenceKind(values, requirement.Kind) {
			return true
		}
	}
	return false
}

func hasEvidenceKind(values []ProposalEvidence, wanted EvidenceKind) bool {
	for _, value := range values {
		if value.Kind == wanted {
			return true
		}
	}
	return false
}

func containsResource(values []Resource, wanted Resource) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func modeRank(value RequestMode) int {
	switch value {
	case RequestModeDirect:
		return 1
	case RequestModeBounded:
		return 2
	case RequestModeWorkflow:
		return 3
	default:
		return 0
	}
}

func riskRank(value RiskClass) int {
	switch value {
	case RiskNormal:
		return 1
	case RiskElevated:
		return 2
	case RiskCritical:
		return 3
	default:
		return 0
	}
}

func maxRisk(left, right RiskClass) RiskClass {
	if riskRank(right) > riskRank(left) {
		return right
	}
	return left
}
