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
	return classifyNormalized(value, traits, rules)
}

type baseAssessment struct {
	reasons      []string
	requirements []EvidenceRequirement
	risk         RiskClass
	workflow     bool
}

func assessBase(traits map[Trait]TraitValue, resources []Resource) baseAssessment {
	result := baseAssessment{reasons: []string{}, requirements: []EvidenceRequirement{}, risk: RiskNormal}
	for _, trait := range allCriticalTraits() {
		if traits[trait] != TraitTrue {
			continue
		}
		if reason := workflowReasonForTrait(trait); reason != "" {
			result.workflow = true
			result.reasons = append(result.reasons, reason)
		}
		result.risk = maxRisk(result.risk, traitRisk(trait))
		if kind, reason := evidenceForTrait(trait); kind != "" {
			result.requirements = appendRequirement(result.requirements, EvidenceRequirement{Kind: kind, Reason: reason})
		}
	}
	for _, resource := range resources {
		if reason := workflowReasonForResource(resource); reason != "" {
			result.workflow = true
			result.reasons = append(result.reasons, reason)
		}
		if resource == ResourceDestructive || resource == ResourceCredentials {
			result.risk = maxRisk(result.risk, RiskCritical)
		} else if resource != ResourceProject && resource != ResourceWorktree && resource != ResourceGitRepository {
			result.risk = maxRisk(result.risk, RiskElevated)
		}
	}
	return result
}

func (value *baseAssessment) requireDirectTraits(traits map[Trait]TraitValue) {
	if traits[TraitScopeClear] != TraitTrue || traits[TraitChangePointKnown] != TraitTrue || traits[TraitRecoverable] != TraitTrue {
		value.workflow = true
		value.reasons = append(value.reasons, "DIRECT_SCOPE_UNCLEAR")
	}
	if traits[TraitFocusedVerificationKnown] != TraitTrue {
		value.workflow = true
		value.reasons = append(value.reasons, "DIRECT_VERIFICATION_REQUIRED")
	}
}

func (value *baseAssessment) requireBaseEvidence(evidence []ProposalEvidence) {
	baseRequirements := []EvidenceRequirement{}
	addBaseEvidenceRequirements(&baseRequirements)
	for _, requirement := range baseRequirements {
		value.requirements = appendRequirement(value.requirements, requirement)
	}
	if missingEvidence(evidence, baseRequirements) {
		value.workflow = true
		value.reasons = append(value.reasons, "DIRECT_VERIFICATION_REQUIRED")
	}
}

func classifyNormalized(value ClassificationProposal, traits map[Trait]TraitValue, rules ClassificationRules) (ClassificationDecision, error) {
	assessment := assessBase(traits, value.Resources)
	bounded := traits[TraitBoundedCapabilityRequest] == TraitTrue
	if !bounded {
		assessment.requireDirectTraits(traits)
	}
	mode := RequestModeDirect
	if bounded {
		mode = RequestModeBounded
	}
	if assessment.workflow {
		mode = RequestModeWorkflow
	} else {
		assessment.requireBaseEvidence(value.Evidence)
		if assessment.workflow {
			mode = RequestModeWorkflow
		}
	}
	decision := assessment.decision(mode)
	if mode == RequestModeBounded {
		setBoundedSelector(&decision, value.CapabilitySelector)
	}
	return applyRules(decision, value, rules)
}

func (value baseAssessment) decision(mode RequestMode) ClassificationDecision {
	decision := ClassificationDecision{RequestMode: mode, RiskClass: value.risk, EvidenceRequirements: value.requirements, EscalationReasons: value.reasons}
	if mode == RequestModeWorkflow {
		complexity := ComplexityComplex
		decision.WorkflowComplexity = &complexity
	}
	return decision
}

func setBoundedSelector(decision *ClassificationDecision, value *CapabilitySelector) {
	if value == nil {
		decision.EscalationReasons = append(decision.EscalationReasons, "CAPABILITY_SELECTION_REQUIRED")
		decision.EvidenceRequirements = appendRequirement(decision.EvidenceRequirements, EvidenceRequirement{
			Kind: EvidenceCapabilitySelector, Reason: "bounded capability selection",
		})
	} else {
		selector := *value
		decision.CapabilitySelector = &selector
	}
}

func applyRules(decision ClassificationDecision, proposal ClassificationProposal, rules ClassificationRules) (ClassificationDecision, error) {
	policy := composeRules(rules)
	baseMode, baseRisk := decision.RequestMode, decision.RiskClass
	protected := containsAnyResource(proposal.Resources, policy.protectedResources)
	if protected && modeRank(RequestModeWorkflow) > modeRank(baseMode) {
		decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_PROTECTED_RESOURCE")
	}
	if modeRank(policy.minimumMode) > modeRank(baseMode) {
		decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_MINIMUM_MODE")
	}
	if riskRank(policy.minimumRisk) > riskRank(baseRisk) {
		decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_MINIMUM_RISK")
	}
	decision.RequestMode = maxMode(decision.RequestMode, policy.minimumMode)
	decision.RiskClass = maxRisk(decision.RiskClass, policy.minimumRisk)
	if protected {
		decision.RequestMode = RequestModeWorkflow
		decision.RiskClass = maxRisk(decision.RiskClass, RiskElevated)
	}
	missingPolicyEvidence := false
	for _, kind := range policy.requiredEvidence {
		decision.EvidenceRequirements = appendRequirement(decision.EvidenceRequirements, EvidenceRequirement{Kind: kind, Reason: "policy-required evidence"})
		missingPolicyEvidence = missingPolicyEvidence || !hasEvidenceKind(proposal.Evidence, kind)
	}
	if missingPolicyEvidence && modeRank(RequestModeWorkflow) > modeRank(baseMode) {
		decision.RequestMode = RequestModeWorkflow
		decision.EscalationReasons = append(decision.EscalationReasons, "POLICY_EVIDENCE_REQUIRED")
	}
	if decision.RequestMode == RequestModeWorkflow {
		complexity := ComplexityComplex
		decision.WorkflowComplexity = &complexity
	} else {
		decision.WorkflowComplexity = nil
	}
	if decision.RequestMode != RequestModeBounded {
		decision.CapabilitySelector = nil
	} else if decision.CapabilitySelector == nil {
		setBoundedSelector(&decision, nil)
	}
	decision.EvidenceRequirements = sortRequirements(decision.EvidenceRequirements)
	decision.EscalationReasons = uniqueSortedStrings(decision.EscalationReasons)
	return withDecisionDigest(decision)
}

type effectivePolicy struct {
	minimumMode        RequestMode
	minimumRisk        RiskClass
	protectedResources []Resource
	requiredEvidence   []EvidenceKind
}

func composeRules(rules ClassificationRules) effectivePolicy {
	policy := effectivePolicy{minimumMode: RequestModeDirect, minimumRisk: RiskNormal}
	for _, layer := range []PolicyLayer{rules.User, rules.Project} {
		policy.minimumMode = maxMode(policy.minimumMode, layer.MinimumMode)
		policy.minimumRisk = maxRisk(policy.minimumRisk, layer.MinimumRisk)
		for _, resource := range layer.ProtectedResources {
			if !containsResource(policy.protectedResources, resource) {
				policy.protectedResources = append(policy.protectedResources, resource)
			}
		}
		for _, kind := range layer.RequiredEvidence {
			if !containsEvidenceKind(policy.requiredEvidence, kind) {
				policy.requiredEvidence = append(policy.requiredEvidence, kind)
			}
		}
	}
	sort.Slice(policy.protectedResources, func(i, j int) bool { return policy.protectedResources[i] < policy.protectedResources[j] })
	sort.Slice(policy.requiredEvidence, func(i, j int) bool { return policy.requiredEvidence[i] < policy.requiredEvidence[j] })
	return policy
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

func containsAnyResource(values, wanted []Resource) bool {
	for _, resource := range wanted {
		if containsResource(values, resource) {
			return true
		}
	}
	return false
}

func containsEvidenceKind(values []EvidenceKind, wanted EvidenceKind) bool {
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

func maxMode(left, right RequestMode) RequestMode {
	if modeRank(right) > modeRank(left) {
		return right
	}
	return left
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
