package runtime

import (
	"fmt"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
)

const RuntimeSchemaV1 = "oaw.runtime/v1"

const (
	headSchemaV1     = "oaw.runtime-head/v1"
	revisionSchemaV1 = "oaw.runtime-revision/v1"
	snapshotSchemaV1 = "oaw.runtime-snapshot/v1"
)

type FrameKind string

const (
	FrameStart    FrameKind = "START"
	FrameContinue FrameKind = "CONTINUE"
	FrameInspect  FrameKind = "INSPECT"
)

type ContinueSignal string

const (
	SignalScopeExpanded ContinueSignal = "SCOPE_EXPANDED"
)

type ReplyKind string

const (
	ReplyModeDecided   ReplyKind = "MODE_DECIDED"
	ReplyPaused        ReplyKind = "PAUSED"
	ReplyStateSnapshot ReplyKind = "STATE_SNAPSHOT"
)

type RunStatus string

const (
	RunReleased RunStatus = "RELEASED"
)

const (
	DiagnosticDirectOutsideCapabilityAdmission = "DIRECT_OUTSIDE_CAPABILITY_ADMISSION"
	DiagnosticHostToolCallsUncontrolled        = "HOST_TOOL_CALLS_UNCONTROLLED"
	DiagnosticResourceLeaseNotApplicable       = "RESOURCE_LEASE_NOT_APPLICABLE"
	ReasonModeEscalationRequired               = "MODE_ESCALATION_REQUIRED"
	RecoveryStartSuccessorRun                  = "START_SUCCESSOR_RUN"
)

type Options struct {
	StateRoot string
	Rules     classification.ClassificationRules
}

type RunFrame struct {
	SchemaVersion    string         `json:"schema_version"`
	Kind             FrameKind      `json:"kind"`
	MessageID        string         `json:"message_id"`
	IdempotencyKey   string         `json:"idempotency_key"`
	RunID            string         `json:"run_id,omitempty"`
	ExpectedRevision uint64         `json:"expected_revision,omitempty"`
	Start            *StartInput    `json:"start,omitempty"`
	Continue         *ContinueInput `json:"continue,omitempty"`
}

type StartInput struct {
	RequestID string                                 `json:"request_id"`
	Project   ProjectIdentity                        `json:"project"`
	Proposal  *classification.ClassificationProposal `json:"proposal,omitempty"`
}

type ContinueInput struct {
	Signal ContinueSignal `json:"signal"`
}

type ProjectIdentity struct {
	Root                string `json:"root"`
	ConfigurationDigest string `json:"configuration_digest"`
}

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ProcessedMessage struct {
	IdempotencyKey string `json:"idempotency_key"`
	ContentDigest  string `json:"content_digest"`
	Revision       uint64 `json:"revision"`
}

type RunSnapshot struct {
	SchemaVersion        string                                `json:"schema_version"`
	RunID                string                                `json:"run_id"`
	RequestID            string                                `json:"request_id"`
	Project              ProjectIdentity                       `json:"project"`
	Revision             uint64                                `json:"revision"`
	RequestMode          classification.RequestMode            `json:"request_mode"`
	Status               RunStatus                             `json:"status"`
	Classification       classification.ClassificationDecision `json:"classification"`
	ClassificationDigest string                                `json:"classification_digest"`
	ConfigurationDigest  string                                `json:"configuration_digest"`
	ProcessedMessages    []ProcessedMessage                    `json:"processed_messages"`
	LifecycleBundles     []string                              `json:"lifecycle_bundles"`
	GrantIDs             []string                              `json:"grant_ids"`
	ResourceLeaseIDs     []string                              `json:"resource_lease_ids"`
}

type RunReply struct {
	SchemaVersion   string       `json:"schema_version"`
	Kind            ReplyKind    `json:"kind"`
	RunID           string       `json:"run_id"`
	Revision        uint64       `json:"revision"`
	RevisionDigest  string       `json:"revision_digest"`
	Snapshot        RunSnapshot  `json:"snapshot"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
	Reason          string       `json:"reason,omitempty"`
	RecoveryActions []string     `json:"recovery_actions"`
}

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (value *Error) Error() string {
	if value.Message == "" {
		return value.Code
	}
	return fmt.Sprintf("%s: %s", value.Code, value.Message)
}

func (value *Error) Unwrap() error { return value.Cause }

func ErrorCode(err error) string {
	if runtimeErr, ok := err.(*Error); ok {
		return runtimeErr.Code
	}
	return ""
}

func runtimeError(code, message string, cause error) error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func cloneProposal(value *classification.ClassificationProposal) *classification.ClassificationProposal {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Traits = append([]classification.TraitObservation{}, value.Traits...)
	cloned.Resources = append([]classification.Resource{}, value.Resources...)
	cloned.Evidence = append([]classification.ProposalEvidence{}, value.Evidence...)
	if value.CapabilitySelector != nil {
		selector := *value.CapabilitySelector
		cloned.CapabilitySelector = &selector
	}
	return &cloned
}

func cloneDecision(value classification.ClassificationDecision) classification.ClassificationDecision {
	value.EvidenceRequirements = append([]classification.EvidenceRequirement{}, value.EvidenceRequirements...)
	value.EscalationReasons = append([]string{}, value.EscalationReasons...)
	if value.WorkflowComplexity != nil {
		complexity := *value.WorkflowComplexity
		value.WorkflowComplexity = &complexity
	}
	if value.CapabilitySelector != nil {
		selector := *value.CapabilitySelector
		value.CapabilitySelector = &selector
	}
	return value
}

func cloneSnapshot(value RunSnapshot) RunSnapshot {
	value.Classification = cloneDecision(value.Classification)
	value.ProcessedMessages = append([]ProcessedMessage{}, value.ProcessedMessages...)
	value.LifecycleBundles = append([]string{}, value.LifecycleBundles...)
	value.GrantIDs = append([]string{}, value.GrantIDs...)
	value.ResourceLeaseIDs = append([]string{}, value.ResourceLeaseIDs...)
	return value
}

func cloneReply(value RunReply) RunReply {
	value.Snapshot = cloneSnapshot(value.Snapshot)
	value.Diagnostics = append([]Diagnostic{}, value.Diagnostics...)
	value.RecoveryActions = append([]string{}, value.RecoveryActions...)
	return value
}
