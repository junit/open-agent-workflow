package admission

import (
	"errors"
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

const (
	CapabilityGrantSchemaV3               = "oaw.capability-grant/v3"
	UserAuthorizationSchemaV1             = "oaw.user-authorization/v1"
	ExplicitInvocationAttestationSchemaV1 = "oaw.explicit-invocation-attestation/v1"
)

type GrantTargetKind string

const (
	GrantProviderBinding GrantTargetKind = "provider-binding"
	GrantHostAction      GrantTargetKind = "host-action"
)

type AuthorizationDecision string

const (
	AuthorizationAllowed AuthorizationDecision = "allowed"
	AuthorizationDenied  AuthorizationDecision = "denied"
)

type ProviderBindingAuthority struct {
	ProviderID                 string                        `json:"provider_id"`
	ProviderInstanceDigest     string                        `json:"provider_instance_digest"`
	DistributionID             string                        `json:"distribution_id"`
	DistributionRevision       string                        `json:"distribution_revision"`
	DistributionTreeDigest     string                        `json:"distribution_tree_digest"`
	BindingID                  string                        `json:"binding_id"`
	Surface                    string                        `json:"surface"`
	Kind                       catalog.BindingKind           `json:"kind"`
	Reference                  string                        `json:"reference"`
	Invocation                 catalog.InvocationDisposition `json:"invocation"`
	BindingTreeDigest          string                        `json:"binding_tree_digest"`
	BindingEvidenceDigest      string                        `json:"binding_evidence_digest"`
	InputArtifact              string                        `json:"input_artifact"`
	OutputArtifact             string                        `json:"output_artifact"`
	InputSchema                string                        `json:"input_schema"`
	OutcomeSchema              string                        `json:"outcome_schema"`
	RequiresExplicitInvocation bool                          `json:"requires_explicit_invocation"`
}

type HostActionAuthority struct {
	ID                string   `json:"id"`
	InputArtifact     string   `json:"input_artifact"`
	OutputArtifact    string   `json:"output_artifact"`
	InputSchema       string   `json:"input_schema"`
	OutcomeSchema     string   `json:"outcome_schema"`
	MaximumEffects    []string `json:"maximum_effects"`
	Resources         []string `json:"resources"`
	ObservationDigest string   `json:"observation_digest"`
}

type AuthorizationTarget struct {
	TargetKind      GrantTargetKind           `json:"target_kind"`
	ProviderBinding *ProviderBindingAuthority `json:"provider_binding,omitempty"`
	HostAction      *HostActionAuthority      `json:"host_action,omitempty"`
}

type UserAuthorization struct {
	SchemaVersion        string                   `json:"schema_version"`
	ID                   string                   `json:"id"`
	IssuerHostID         string                   `json:"issuer_host_id"`
	HostSessionDigest    string                   `json:"host_session_digest"`
	EvidenceHandleDigest string                   `json:"evidence_handle_digest"`
	AuthorizationNonce   string                   `json:"authorization_nonce"`
	WorkflowID           string                   `json:"workflow_id"`
	BundleID             string                   `json:"bundle_id"`
	BundleGeneration     uint64                   `json:"bundle_generation"`
	BundleDigest         string                   `json:"bundle_digest"`
	Cursor               execution.GraphCursor    `json:"cursor"`
	Target               AuthorizationTarget      `json:"target"`
	Decision             AuthorizationDecision    `json:"decision"`
	Effects              []string                 `json:"effects"`
	Resources            []string                 `json:"resources"`
	Evidence             []host.EvidenceReference `json:"evidence"`
	Digest               string                   `json:"digest"`
}

type ExplicitInvocationAttestation struct {
	SchemaVersion        string                   `json:"schema_version"`
	ID                   string                   `json:"id"`
	IssuerHostID         string                   `json:"issuer_host_id"`
	HostSessionDigest    string                   `json:"host_session_digest"`
	EvidenceHandleDigest string                   `json:"evidence_handle_digest"`
	InvocationNonce      string                   `json:"invocation_nonce"`
	WorkflowID           string                   `json:"workflow_id"`
	BundleID             string                   `json:"bundle_id"`
	BundleGeneration     uint64                   `json:"bundle_generation"`
	BundleDigest         string                   `json:"bundle_digest"`
	Cursor               execution.GraphCursor    `json:"cursor"`
	ProviderBinding      ProviderBindingAuthority `json:"provider_binding"`
	Evidence             []host.EvidenceReference `json:"evidence"`
	Digest               string                   `json:"digest"`
}

type AuthorityCeiling struct {
	Effects         []string `json:"effects"`
	Resources       []string `json:"resources"`
	ResourceLeases  bool     `json:"resource_leases"`
	AllowDelegation bool     `json:"allow_delegation"`
}

type WorkflowGrantRequest struct {
	WorkflowID            string
	RequestID             string
	BundleID              string
	BundleGeneration      uint64
	BundleDigest          string
	Cursor                execution.GraphCursor
	ProviderBinding       *profile.ResolvedBinding
	Capability            *catalog.CapabilityRecord
	HostAction            *profile.CompiledHostAction
	Topology              execution.Topology
	HostID                string
	HostSessionDigest     string
	Effects               []string
	Resources             []string
	TerminationCondition  string
	Authorization         *UserAuthorization
	InvocationAttestation *ExplicitInvocationAttestation
	Authority             AuthorityCeiling
}

type CapabilityGrant struct {
	SchemaVersion               string                `json:"schema_version"`
	ID                          string                `json:"id"`
	WorkflowID                  string                `json:"workflow_id"`
	RequestID                   string                `json:"request_id"`
	BundleID                    string                `json:"bundle_id"`
	BundleGeneration            uint64                `json:"bundle_generation"`
	BundleDigest                string                `json:"bundle_digest"`
	Cursor                      execution.GraphCursor `json:"cursor"`
	Target                      AuthorizationTarget   `json:"target"`
	Topology                    execution.Topology    `json:"topology"`
	HostSessionDigest           string                `json:"host_session_digest"`
	Effects                     []string              `json:"effects"`
	Resources                   []string              `json:"resources"`
	TerminationCondition        string                `json:"termination_condition"`
	AuthorizationDigest         string                `json:"authorization_digest"`
	InvocationAttestationDigest string                `json:"invocation_attestation_digest"`
	Digest                      string                `json:"digest"`
}

type Error struct {
	Code   string
	Detail string
	Cause  error
}

func (value *Error) Error() string {
	if value.Detail == "" {
		return value.Code
	}
	return fmt.Sprintf("%s: %s", value.Code, value.Detail)
}

func (value *Error) Unwrap() error { return value.Cause }

func ErrorCode(err error) string {
	var admissionErr *Error
	if errors.As(err, &admissionErr) {
		return admissionErr.Code
	}
	return ""
}

func admissionError(code, detail string, cause error) error {
	return &Error{Code: code, Detail: detail, Cause: cause}
}

func CloneGrant(value CapabilityGrant) CapabilityGrant {
	value.Target = CloneAuthorizationTarget(value.Target)
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}

func CloneAuthorizationTarget(value AuthorizationTarget) AuthorizationTarget {
	if value.ProviderBinding != nil {
		binding := *value.ProviderBinding
		value.ProviderBinding = &binding
	}
	if value.HostAction != nil {
		action := cloneHostActionAuthority(*value.HostAction)
		value.HostAction = &action
	}
	return value
}

func CloneUserAuthorization(value UserAuthorization) UserAuthorization {
	value.Target = CloneAuthorizationTarget(value.Target)
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	value.Evidence = append([]host.EvidenceReference{}, value.Evidence...)
	return value
}

func CloneExplicitInvocationAttestation(value ExplicitInvocationAttestation) ExplicitInvocationAttestation {
	value.Evidence = append([]host.EvidenceReference{}, value.Evidence...)
	return value
}

func CloneAuthority(value AuthorityCeiling) AuthorityCeiling {
	value.Effects = append([]string{}, value.Effects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}

func cloneHostActionAuthority(value HostActionAuthority) HostActionAuthority {
	value.MaximumEffects = append([]string{}, value.MaximumEffects...)
	value.Resources = append([]string{}, value.Resources...)
	return value
}
