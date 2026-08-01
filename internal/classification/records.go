package classification

import "sort"

const ProposalSchemaV1 = "oaw.classification-proposal/v1"

type RequestMode string

const (
	RequestModeDirect   RequestMode = "DIRECT"
	RequestModeBounded  RequestMode = "BOUNDED"
	RequestModeWorkflow RequestMode = "WORKFLOW"
)

type WorkflowComplexity string

const (
	ComplexityOrdinary WorkflowComplexity = "ordinary"
	ComplexityComplex  WorkflowComplexity = "complex"
)

type RiskClass string

const (
	RiskNormal   RiskClass = "normal"
	RiskElevated RiskClass = "elevated"
	RiskCritical RiskClass = "critical"
)

type Trait string

const (
	TraitScopeClear               Trait = "scope-clear"
	TraitChangePointKnown         Trait = "change-point-known"
	TraitRecoverable              Trait = "recoverable"
	TraitFocusedVerificationKnown Trait = "focused-verification-known"
	TraitBoundedCapabilityRequest Trait = "bounded-capability-request"
	TraitArchitectureDecision     Trait = "architecture-decision"
	TraitPublicContractChange     Trait = "public-contract-change"
	TraitSchemaChange             Trait = "schema-change"
	TraitDependencyChange         Trait = "dependency-change"
	TraitSecuritySensitive        Trait = "security-sensitive"
	TraitDataSensitive            Trait = "data-sensitive"
	TraitDeploymentChange         Trait = "deployment-change"
	TraitDomainUncertainty        Trait = "domain-uncertainty"
	TraitRootCauseUncertain       Trait = "root-cause-uncertain"
	TraitMultipleResponsibilities Trait = "multiple-responsibilities"
	TraitMultipleTickets          Trait = "multiple-tickets"
	TraitLongLivedDelegation      Trait = "long-lived-delegation"
	TraitDestructiveMutation      Trait = "destructive-mutation"
	TraitCriticalRelease          Trait = "critical-release"
)

type TraitValue string

const (
	TraitTrue    TraitValue = "true"
	TraitFalse   TraitValue = "false"
	TraitUnknown TraitValue = "unknown"
)

type TraitObservation struct {
	Trait Trait      `json:"trait"`
	Value TraitValue `json:"value"`
}

type Resource string

const (
	ResourceProject       Resource = "project"
	ResourceWorktree      Resource = "project-worktree"
	ResourceGitRepository Resource = "git-repository"
	ResourcePublicAPI     Resource = "public-api"
	ResourceSchema        Resource = "schema"
	ResourceDependency    Resource = "dependency"
	ResourceSecurity      Resource = "security"
	ResourceData          Resource = "data"
	ResourceDeployment    Resource = "deployment"
	ResourceCredentials   Resource = "credentials"
	ResourceNetwork       Resource = "network"
	ResourceDestructive   Resource = "destructive"
)

type EvidenceKind string

const (
	EvidenceScope              EvidenceKind = "scope"
	EvidenceChangePoint        EvidenceKind = "change-point"
	EvidenceVerification       EvidenceKind = "verification"
	EvidenceCapabilitySelector EvidenceKind = "capability-selector"
	EvidenceSecurityAcceptance EvidenceKind = "security-acceptance"
	EvidenceNegativeTest       EvidenceKind = "negative-test"
	EvidenceArchitecture       EvidenceKind = "architecture"
	EvidenceAuthorization      EvidenceKind = "authorization"
	EvidenceRecovery           EvidenceKind = "recovery"
)

type SelectorSource string

const (
	SelectorUserIntent  SelectorSource = "user-intent"
	SelectorTrustedRule SelectorSource = "trusted-rule"
)

type CapabilitySelector struct {
	ProviderID   string         `json:"provider_id"`
	CapabilityID string         `json:"capability_id"`
	Source       SelectorSource `json:"source"`
}

type ProposalEvidence struct {
	Kind      EvidenceKind `json:"kind"`
	Reference string       `json:"reference"`
	Digest    string       `json:"digest"`
}

type ClassificationProposal struct {
	SchemaVersion      string              `json:"schema_version"`
	Traits             []TraitObservation  `json:"traits"`
	Resources          []Resource          `json:"resources"`
	Evidence           []ProposalEvidence  `json:"evidence"`
	CapabilitySelector *CapabilitySelector `json:"capability_selector,omitempty"`
}

type EvidenceRequirement struct {
	Kind   EvidenceKind `json:"kind"`
	Reason string       `json:"reason"`
}

type ClassificationDecision struct {
	RequestMode          RequestMode           `json:"request_mode"`
	WorkflowComplexity   *WorkflowComplexity   `json:"workflow_complexity"`
	RiskClass            RiskClass             `json:"risk_class"`
	EvidenceRequirements []EvidenceRequirement `json:"evidence_requirements"`
	EscalationReasons    []string              `json:"escalation_reasons"`
	CapabilitySelector   *CapabilitySelector   `json:"capability_selector,omitempty"`
	digest               string
}

type PolicyLayer struct {
	MinimumMode        RequestMode    `json:"minimum_mode"`
	MinimumRisk        RiskClass      `json:"minimum_risk"`
	ProtectedResources []Resource     `json:"protected_resources"`
	RequiredEvidence   []EvidenceKind `json:"required_evidence"`
}

type ClassificationRules struct {
	User    PolicyLayer `json:"user"`
	Project PolicyLayer `json:"project"`
}

func (value ClassificationDecision) Digest() string { return value.digest }

func cloneProposal(value ClassificationProposal) ClassificationProposal {
	value.Traits = append([]TraitObservation{}, value.Traits...)
	value.Resources = append([]Resource{}, value.Resources...)
	value.Evidence = append([]ProposalEvidence{}, value.Evidence...)
	if value.CapabilitySelector != nil {
		selector := *value.CapabilitySelector
		value.CapabilitySelector = &selector
	}
	return value
}

func sortProposalCollections(value *ClassificationProposal) {
	sort.Slice(value.Traits, func(i, j int) bool { return value.Traits[i].Trait < value.Traits[j].Trait })
	sort.Slice(value.Resources, func(i, j int) bool { return value.Resources[i] < value.Resources[j] })
	sort.Slice(value.Evidence, func(i, j int) bool {
		left := string(value.Evidence[i].Kind) + "\x00" + value.Evidence[i].Reference + "\x00" + value.Evidence[i].Digest
		right := string(value.Evidence[j].Kind) + "\x00" + value.Evidence[j].Reference + "\x00" + value.Evidence[j].Digest
		return left < right
	})
}

func allCriticalTraits() []Trait {
	return []Trait{
		TraitScopeClear,
		TraitChangePointKnown,
		TraitRecoverable,
		TraitFocusedVerificationKnown,
		TraitBoundedCapabilityRequest,
		TraitArchitectureDecision,
		TraitPublicContractChange,
		TraitSchemaChange,
		TraitDependencyChange,
		TraitSecuritySensitive,
		TraitDataSensitive,
		TraitDeploymentChange,
		TraitDomainUncertainty,
		TraitRootCauseUncertain,
		TraitMultipleResponsibilities,
		TraitMultipleTickets,
		TraitLongLivedDelegation,
		TraitDestructiveMutation,
		TraitCriticalRelease,
	}
}
