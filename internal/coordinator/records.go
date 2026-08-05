package coordinator

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/admission"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	WorkflowCommandSchemaV1  = "oaw.workflow-command/v1"
	WorkflowResultSchemaV1   = "oaw.workflow-result/v1"
	WorkflowSnapshotSchemaV1 = "oaw.workflow-snapshot/v1"
	WorkflowRevisionSchemaV1 = "oaw.workflow-revision/v1"
	WorkflowHeadSchemaV1     = "oaw.workflow-head/v1"
	DispatchPacketSchemaV1   = "oaw.dispatch-packet/v1"
)

type CommandKind string

const (
	CommandStart   CommandKind = "START"
	CommandInspect CommandKind = "INSPECT"
	CommandPrepare CommandKind = "PREPARE"
	CommandReceipt CommandKind = "RECEIPT"
	CommandSwitch  CommandKind = "SWITCH"
	CommandCancel  CommandKind = "CANCEL"
)

type Command struct {
	SchemaVersion    string        `json:"schema_version"`
	Kind             CommandKind   `json:"kind"`
	MessageID        string        `json:"message_id"`
	IdempotencyKey   string        `json:"idempotency_key"`
	WorkflowID       string        `json:"workflow_id"`
	ExpectedRevision uint64        `json:"expected_revision"`
	Start            *StartInput   `json:"start,omitempty"`
	Prepare          *PrepareInput `json:"prepare,omitempty"`
	Receipt          *ReceiptInput `json:"receipt,omitempty"`
	Switch           *SwitchInput  `json:"switch,omitempty"`
	Cancel           *CancelInput  `json:"cancel,omitempty"`
}

type StartInput struct {
	RequestID     string                                `json:"request_id"`
	DeliverableID string                                `json:"deliverable_id"`
	InputDigest   string                                `json:"input_digest"`
	ActiveTicket  string                                `json:"active_ticket"`
	Proposal      classification.ClassificationProposal `json:"proposal"`
	Selection     core.Selection                        `json:"selection"`
	HostSession   host.SessionSnapshot                  `json:"host_session"`
	Environment   host.EnvironmentReport                `json:"environment"`
}

type ArtifactReference struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

type EvidenceRequirement struct {
	Kind        string `json:"kind"`
	Minimum     uint64 `json:"minimum"`
	Description string `json:"description"`
}

type PrepareInput struct {
	RequestedEffects     []string              `json:"requested_effects"`
	RequestedResources   []string              `json:"requested_resources"`
	TerminationCondition string                `json:"termination_condition"`
	InputReferences      []ArtifactReference   `json:"input_references"`
	EvidenceRequirements []EvidenceRequirement `json:"evidence_requirements"`
}

type ReceiptInput struct {
	Receipt        host.InvocationReceipt `json:"receipt"`
	Signal         string                 `json:"signal"`
	StableBoundary string                 `json:"stable_boundary"`
}

type SwitchInput struct {
	Boundary    string                 `json:"boundary"`
	Selection   core.Selection         `json:"selection"`
	HostSession host.SessionSnapshot   `json:"host_session"`
	Environment host.EnvironmentReport `json:"environment"`
}

type CancelInput struct {
	Reason             string `json:"reason"`
	InvocationTerminal bool   `json:"invocation_terminal"`
}

type Status string

const (
	StatusReady     Status = "READY"
	StatusPrepared  Status = "PREPARED"
	StatusInFlight  Status = "IN_FLIGHT"
	StatusPaused    Status = "PAUSED"
	StatusFinished  Status = "FINISHED"
	StatusCancelled Status = "CANCELLED"
)

type ProcessedMessage struct {
	IdempotencyKey string `json:"idempotency_key"`
	ContentDigest  string `json:"content_digest"`
	Revision       uint64 `json:"revision"`
	ResultDigest   string `json:"result_digest"`
}

type ResourceLease struct {
	SchemaVersion    string `json:"schema_version"`
	ID               string `json:"id"`
	WorkflowID       string `json:"workflow_id"`
	GrantID          string `json:"grant_id"`
	BundleID         string `json:"bundle_id"`
	BundleGeneration uint64 `json:"bundle_generation"`
	Resource         string `json:"resource"`
	PhysicalRoot     string `json:"physical_root"`
	AcquiredRevision uint64 `json:"acquired_revision"`
	ReleasedRevision uint64 `json:"released_revision,omitempty"`
	Digest           string `json:"digest"`
}

type ProjectionLag struct {
	Revision uint64 `json:"revision"`
	Digest   string `json:"digest"`
	Reason   string `json:"reason"`
}

type Snapshot struct {
	SchemaVersion      string                                `json:"schema_version"`
	WorkflowID         string                                `json:"workflow_id"`
	RequestID          string                                `json:"request_id"`
	DeliverableID      string                                `json:"deliverable_id"`
	Revision           uint64                                `json:"revision"`
	Status             Status                                `json:"status"`
	Classification     classification.ClassificationDecision `json:"classification"`
	Bundles            []core.LifecycleBundle                `json:"bundles"`
	ActiveGeneration   uint64                                `json:"active_generation"`
	ActiveNodeID       string                                `json:"active_node_id"`
	ActiveTicket       string                                `json:"active_ticket"`
	ActiveGrant        *admission.CapabilityGrant            `json:"active_grant,omitempty"`
	GrantHistory       []admission.CapabilityGrant           `json:"grant_history"`
	Receipts           []host.InvocationReceipt              `json:"receipts"`
	ResourceLeases     []ResourceLease                       `json:"resource_leases"`
	LastStableBoundary string                                `json:"last_stable_boundary"`
	ProcessedMessages  []ProcessedMessage                    `json:"processed_messages"`
	ProjectionLag      []ProjectionLag                       `json:"projection_lag"`
}

type DispatchPacket struct {
	SchemaVersion           string                             `json:"schema_version"`
	ID                      string                             `json:"id"`
	WorkflowID              string                             `json:"workflow_id"`
	RequestID               string                             `json:"request_id"`
	BundleID                string                             `json:"bundle_id"`
	BundleGeneration        uint64                             `json:"bundle_generation"`
	BundleDigest            string                             `json:"bundle_digest"`
	NodeID                  string                             `json:"node_id"`
	Ticket                  string                             `json:"ticket,omitempty"`
	Topology                execution.Topology                 `json:"topology"`
	HostSessionDigest       string                             `json:"host_session_digest"`
	Grant                   admission.CapabilityGrant          `json:"grant"`
	InputReferences         []ArtifactReference                `json:"input_references"`
	EvidenceRequirements    []EvidenceRequirement              `json:"evidence_requirements"`
	EnvironmentRequirements []execution.EnvironmentRequirement `json:"environment_requirements"`
	Digest                  string                             `json:"digest"`
}

type ResultKind string

const (
	ResultState    ResultKind = "STATE"
	ResultDispatch ResultKind = "DISPATCH"
	ResultRejected ResultKind = "REJECTED"
)

type Diagnostic struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type Result struct {
	SchemaVersion  string          `json:"schema_version"`
	Kind           ResultKind      `json:"kind"`
	WorkflowID     string          `json:"workflow_id"`
	Revision       uint64          `json:"revision"`
	RevisionDigest string          `json:"revision_digest"`
	Snapshot       *Snapshot       `json:"snapshot,omitempty"`
	Dispatch       *DispatchPacket `json:"dispatch,omitempty"`
	Diagnostics    []Diagnostic    `json:"diagnostics"`
	Replayed       bool            `json:"replayed"`
	Digest         string          `json:"digest"`
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
	var coordinatorErr *Error
	if errors.As(err, &coordinatorErr) {
		return coordinatorErr.Code
	}
	return ""
}

func coordinatorError(code, detail string, cause error) error {
	return &Error{Code: code, Detail: detail, Cause: cause}
}

func normalizeCommand(value Command) (Command, error) {
	value = cloneCommand(value)
	if value.Start != nil {
		normalizeSelection(&value.Start.Selection)
	}
	if value.Prepare != nil {
		sort.Strings(value.Prepare.RequestedEffects)
		sort.Strings(value.Prepare.RequestedResources)
		sort.Slice(value.Prepare.InputReferences, func(left, right int) bool {
			return artifactReferenceKey(value.Prepare.InputReferences[left]) < artifactReferenceKey(value.Prepare.InputReferences[right])
		})
		sort.Slice(value.Prepare.EvidenceRequirements, func(left, right int) bool {
			return evidenceRequirementKey(value.Prepare.EvidenceRequirements[left]) < evidenceRequirementKey(value.Prepare.EvidenceRequirements[right])
		})
	}
	if value.Receipt != nil {
		receipt, err := host.NewInvocationReceipt(value.Receipt.Receipt)
		if err != nil || !reflect.DeepEqual(receipt, value.Receipt.Receipt) {
			return Command{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "Receipt is not a canonical Host record", err)
		}
		value.Receipt.Receipt = receipt
	}
	if value.Switch != nil {
		normalizeSelection(&value.Switch.Selection)
	}
	return value, nil
}

func validateCommand(value Command) error {
	if value.SchemaVersion != WorkflowCommandSchemaV1 {
		return coordinatorError("SCHEMA_UNSUPPORTED", "unsupported Workflow Command schema", nil)
	}
	if !commandPayloadMatchesKind(value) {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "Command payload does not match kind", nil)
	}
	switch value.Kind {
	case CommandStart:
		if !validText(value.MessageID, 512) || !validText(value.IdempotencyKey, 512) || value.WorkflowID != "" || value.ExpectedRevision != 0 {
			return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid START command identity", nil)
		}
		return validateStartInput(*value.Start)
	case CommandInspect:
		if value.MessageID != "" || value.IdempotencyKey != "" || !validText(value.WorkflowID, 512) || value.ExpectedRevision != 0 {
			return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid INSPECT command identity", nil)
		}
	case CommandPrepare:
		if err := validateMutationIdentity(value); err != nil {
			return err
		}
		return validatePrepareInput(*value.Prepare)
	case CommandReceipt:
		if err := validateMutationIdentity(value); err != nil {
			return err
		}
		return validateReceiptInput(*value.Receipt)
	case CommandSwitch:
		if err := validateMutationIdentity(value); err != nil {
			return err
		}
		return validateSwitchInput(*value.Switch)
	case CommandCancel:
		if err := validateMutationIdentity(value); err != nil {
			return err
		}
		if !validText(value.Cancel.Reason, 2048) {
			return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid CANCEL reason", nil)
		}
	default:
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "unknown Workflow Command kind", nil)
	}
	return nil
}

func validateMutationIdentity(value Command) error {
	if !validText(value.MessageID, 512) || !validText(value.IdempotencyKey, 512) || !validText(value.WorkflowID, 512) || value.ExpectedRevision == 0 {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid mutating Command identity", nil)
	}
	return nil
}

func validateStartInput(value StartInput) error {
	if !validText(value.RequestID, 512) || !validText(value.DeliverableID, 512) || !validDigest(value.InputDigest) ||
		value.ActiveTicket != "" && !validText(value.ActiveTicket, 512) || value.Proposal.SchemaVersion != classification.ProposalSchemaV1 {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid START input identity", nil)
	}
	if err := validateSelection(value.Selection); err != nil {
		return err
	}
	if value.Environment.Topology != value.Selection.Topology {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "START environment topology does not match Profile selection", nil)
	}
	return validateHostFacts(value.HostSession, value.Environment)
}

func validatePrepareInput(value PrepareInput) error {
	if len(value.RequestedEffects) == 0 || len(value.RequestedEffects) > 128 || len(value.RequestedResources) == 0 || len(value.RequestedResources) > 128 ||
		!validText(value.TerminationCondition, 2048) || len(value.InputReferences) > 128 || len(value.EvidenceRequirements) > 128 {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid PREPARE input", nil)
	}
	if !validUniqueTextSet(value.RequestedEffects, 128) || !validUniqueTextSet(value.RequestedResources, 512) {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "PREPARE effects or resources are not canonical", nil)
	}
	for index, reference := range value.InputReferences {
		if !validText(reference.Kind, 128) || !validText(reference.Reference, 2048) || !validDigest(reference.Digest) ||
			index > 0 && artifactReferenceKey(value.InputReferences[index-1]) == artifactReferenceKey(reference) {
			return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid PREPARE input reference", nil)
		}
	}
	for index, requirement := range value.EvidenceRequirements {
		if !validText(requirement.Kind, 128) || requirement.Minimum == 0 || !validText(requirement.Description, 2048) ||
			index > 0 && evidenceRequirementKey(value.EvidenceRequirements[index-1]) == evidenceRequirementKey(requirement) {
			return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid PREPARE evidence requirement", nil)
		}
	}
	return nil
}

func validateReceiptInput(value ReceiptInput) error {
	if value.Receipt.SchemaVersion != host.HostInvocationReceiptSchemaV2 || !validDigest(value.Receipt.Digest) ||
		value.Signal != "" && !validText(value.Signal, 512) || value.StableBoundary != "" && !validText(value.StableBoundary, 512) {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid RECEIPT input", nil)
	}
	return nil
}

func validateSwitchInput(value SwitchInput) error {
	if !validText(value.Boundary, 512) {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid SWITCH boundary", nil)
	}
	if err := validateSelection(value.Selection); err != nil {
		return err
	}
	return validateHostFacts(value.HostSession, value.Environment)
}

func validateSelection(value core.Selection) error {
	if !validText(value.Profile, 512) || value.ProfileSource != core.SelectionUser ||
		value.Topology != execution.TopologyCurrent && value.Topology != execution.TopologySubagent ||
		value.TopologySource != core.SelectionUser && value.TopologySource != core.SelectionHostOnlyOption ||
		len(value.AddOns) > 128 || !validUniqueTextSet(value.AddOns, 512) {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid Profile selection", nil)
	}
	return nil
}

func validateHostFacts(session host.SessionSnapshot, environment host.EnvironmentReport) error {
	if err := host.ValidateEnvironmentReport(session, environment); err != nil {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "invalid Host session environment", err)
	}
	return nil
}

func expectedPayloadCount(kind CommandKind) int {
	switch kind {
	case CommandStart, CommandPrepare, CommandReceipt, CommandSwitch, CommandCancel:
		return 1
	case CommandInspect:
		return 0
	default:
		return -1
	}
}

func commandPayloadMatchesKind(value Command) bool {
	if commandPayloadCount(value) != expectedPayloadCount(value.Kind) {
		return false
	}
	switch value.Kind {
	case CommandStart:
		return value.Start != nil
	case CommandInspect:
		return true
	case CommandPrepare:
		return value.Prepare != nil
	case CommandReceipt:
		return value.Receipt != nil
	case CommandSwitch:
		return value.Switch != nil
	case CommandCancel:
		return value.Cancel != nil
	default:
		return false
	}
}

func commandPayloadCount(value Command) int {
	count := 0
	for _, present := range []bool{value.Start != nil, value.Prepare != nil, value.Receipt != nil, value.Switch != nil, value.Cancel != nil} {
		if present {
			count++
		}
	}
	return count
}

func normalizeSelection(value *core.Selection) {
	value.AddOns = append([]string{}, value.AddOns...)
	sort.Strings(value.AddOns)
	value.Bindings = append(value.Bindings[:0:0], value.Bindings...)
	sort.Slice(value.Bindings, func(left, right int) bool {
		leftValue, rightValue := value.Bindings[left], value.Bindings[right]
		return leftValue.Selector.ProviderID+"\x00"+leftValue.Selector.CapabilityID+"\x00"+leftValue.PreferredProviderID <
			rightValue.Selector.ProviderID+"\x00"+rightValue.Selector.CapabilityID+"\x00"+rightValue.PreferredProviderID
	})
}

func normalizeResult(value Result) (Result, error) {
	providedDigest := value.Digest
	value.Digest = ""
	value.Diagnostics = append([]Diagnostic{}, value.Diagnostics...)
	sort.Slice(value.Diagnostics, func(left, right int) bool {
		return diagnosticKey(value.Diagnostics[left]) < diagnosticKey(value.Diagnostics[right])
	})
	if err := validateResult(value); err != nil {
		return Result{}, err
	}
	if err := validateResultProcessedMessagePin(value, providedDigest); err != nil {
		return Result{}, err
	}
	digest, _, err := canonicaljson.Digest(resultDigestProjection(value))
	if err != nil {
		return Result{}, coordinatorError("WORKFLOW_RESULT_ENCODE_FAILED", "Workflow Result cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return Result{}, coordinatorError("WORKFLOW_RESULT_INVALID", "Workflow Result digest mismatch", nil)
	}
	value.Digest = digest
	return value, nil
}

func resultDigestProjection(value Result) Result {
	value.Digest = ""
	if value.Snapshot != nil {
		snapshot := *value.Snapshot
		snapshot.ProcessedMessages = clearResultPinForRevision(snapshot.ProcessedMessages, value.Revision)
		value.Snapshot = &snapshot
	}
	return value
}

func validateResultProcessedMessagePin(value Result, providedDigest string) error {
	if value.Kind != ResultState && value.Kind != ResultDispatch {
		return nil
	}
	message, found := processedMessageForRevision(value.Snapshot.ProcessedMessages, value.Revision)
	if !found {
		return coordinatorError("WORKFLOW_RESULT_INVALID", "Result snapshot is missing the current processed message", nil)
	}
	if providedDigest == "" && message.ResultDigest != "" || providedDigest != "" && message.ResultDigest != providedDigest {
		return coordinatorError("WORKFLOW_RESULT_INVALID", "Result processed message digest does not pin the Result", nil)
	}
	return nil
}

func clearResultPinForRevision(values []ProcessedMessage, revision uint64) []ProcessedMessage {
	result := append([]ProcessedMessage{}, values...)
	for index := range result {
		if result[index].Revision == revision {
			result[index].ResultDigest = ""
		}
	}
	return result
}

func setResultProcessedMessagePin(value *Result, digest string) bool {
	if value == nil || value.Snapshot == nil {
		return false
	}
	updated := false
	value.Snapshot.ProcessedMessages = append([]ProcessedMessage{}, value.Snapshot.ProcessedMessages...)
	for index := range value.Snapshot.ProcessedMessages {
		if value.Snapshot.ProcessedMessages[index].Revision == value.Revision {
			value.Snapshot.ProcessedMessages[index].ResultDigest = digest
			updated = true
		}
	}
	return updated
}

func validateResult(value Result) error {
	if value.SchemaVersion != WorkflowResultSchemaV1 {
		return coordinatorError("SCHEMA_UNSUPPORTED", "unsupported Workflow Result schema", nil)
	}
	if len(value.Diagnostics) > 32 {
		return coordinatorError("WORKFLOW_RESULT_INVALID", "too many Workflow diagnostics", nil)
	}
	for index, diagnostic := range value.Diagnostics {
		if !validText(diagnostic.Code, 128) || !validText(diagnostic.Detail, 2048) ||
			index > 0 && diagnosticKey(value.Diagnostics[index-1]) == diagnosticKey(diagnostic) {
			return coordinatorError("WORKFLOW_RESULT_INVALID", "invalid or duplicate Workflow diagnostic", nil)
		}
	}
	switch value.Kind {
	case ResultRejected:
		if value.Snapshot != nil || value.Dispatch != nil || len(value.Diagnostics) == 0 {
			return coordinatorError("WORKFLOW_RESULT_INVALID", "invalid REJECTED Result", nil)
		}
		if value.WorkflowID != "" || value.Revision != 0 || value.RevisionDigest != "" {
			if !validText(value.WorkflowID, 512) || value.Revision == 0 || !validDigest(value.RevisionDigest) {
				return coordinatorError("WORKFLOW_RESULT_INVALID", "invalid persisted REJECTED Result identity", nil)
			}
		}
	case ResultState:
		if value.Snapshot == nil || value.Dispatch != nil || !validResultIdentity(value) || value.Snapshot.WorkflowID != value.WorkflowID || value.Snapshot.Revision != value.Revision {
			return coordinatorError("WORKFLOW_RESULT_INVALID", "invalid STATE Result", nil)
		}
	case ResultDispatch:
		if value.Snapshot == nil || value.Dispatch == nil || !validResultIdentity(value) || value.Snapshot.WorkflowID != value.WorkflowID ||
			value.Snapshot.Revision != value.Revision || value.Dispatch.WorkflowID != value.WorkflowID {
			return coordinatorError("WORKFLOW_RESULT_INVALID", "invalid DISPATCH Result", nil)
		}
	default:
		return coordinatorError("WORKFLOW_RESULT_INVALID", "unknown Workflow Result kind", nil)
	}
	return nil
}

func validResultIdentity(value Result) bool {
	return validText(value.WorkflowID, 512) && value.Revision > 0 && validDigest(value.RevisionDigest)
}

func cloneCommand(value Command) Command {
	if value.Start != nil {
		start := *value.Start
		start.Proposal.Traits = append([]classification.TraitObservation{}, start.Proposal.Traits...)
		start.Proposal.Resources = append([]classification.Resource{}, start.Proposal.Resources...)
		start.Proposal.Evidence = append([]classification.ProposalEvidence{}, start.Proposal.Evidence...)
		if start.Proposal.CapabilitySelector != nil {
			selector := *start.Proposal.CapabilitySelector
			start.Proposal.CapabilitySelector = &selector
		}
		start.Selection.AddOns = append([]string{}, start.Selection.AddOns...)
		start.Selection.Bindings = append(start.Selection.Bindings[:0:0], start.Selection.Bindings...)
		start.HostSession = host.CloneSessionSnapshot(start.HostSession)
		start.Environment = host.CloneEnvironmentReport(start.Environment)
		value.Start = &start
	}
	if value.Prepare != nil {
		prepare := *value.Prepare
		prepare.RequestedEffects = append([]string{}, prepare.RequestedEffects...)
		prepare.RequestedResources = append([]string{}, prepare.RequestedResources...)
		prepare.InputReferences = append([]ArtifactReference{}, prepare.InputReferences...)
		prepare.EvidenceRequirements = append([]EvidenceRequirement{}, prepare.EvidenceRequirements...)
		value.Prepare = &prepare
	}
	if value.Receipt != nil {
		receipt := *value.Receipt
		receipt.Receipt = host.CloneInvocationReceipt(receipt.Receipt)
		value.Receipt = &receipt
	}
	if value.Switch != nil {
		switchInput := *value.Switch
		switchInput.Selection.AddOns = append([]string{}, switchInput.Selection.AddOns...)
		switchInput.Selection.Bindings = append(switchInput.Selection.Bindings[:0:0], switchInput.Selection.Bindings...)
		switchInput.HostSession = host.CloneSessionSnapshot(switchInput.HostSession)
		switchInput.Environment = host.CloneEnvironmentReport(switchInput.Environment)
		value.Switch = &switchInput
	}
	if value.Cancel != nil {
		cancel := *value.Cancel
		value.Cancel = &cancel
	}
	return value
}

func validText(value string, maximumRunes int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximumRunes &&
		strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validUniqueTextSet(values []string, maximumRunes int) bool {
	for index, value := range values {
		if !validText(value, maximumRunes) || index > 0 && values[index-1] == value {
			return false
		}
	}
	return true
}

func artifactReferenceKey(value ArtifactReference) string {
	return value.Kind + "\x00" + value.Reference + "\x00" + value.Digest
}

func evidenceRequirementKey(value EvidenceRequirement) string {
	return fmt.Sprintf("%s\x00%020d\x00%s", value.Kind, value.Minimum, value.Description)
}

func diagnosticKey(value Diagnostic) string {
	return value.Code + "\x00" + value.Detail
}
