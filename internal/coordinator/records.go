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
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

const (
	WorkflowCommandSchemaV2  = "oaw.workflow-command/v2"
	WorkflowResultSchemaV2   = "oaw.workflow-result/v2"
	WorkflowSnapshotSchemaV2 = "oaw.workflow-snapshot/v2"
	WorkflowRevisionSchemaV2 = "oaw.workflow-revision/v2"
	WorkflowHeadSchemaV1     = "oaw.workflow-head/v1"
	DispatchPacketSchemaV2   = "oaw.dispatch-packet/v2"
	GateAttestationSchemaV1  = "oaw.gate-attestation/v1"
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
	RequestedEffects      []string                                 `json:"requested_effects"`
	RequestedResources    []string                                 `json:"requested_resources"`
	TerminationCondition  string                                   `json:"termination_condition"`
	InputReferences       []ArtifactReference                      `json:"input_references"`
	EvidenceRequirements  []EvidenceRequirement                    `json:"evidence_requirements"`
	Authorization         *admission.UserAuthorization             `json:"authorization,omitempty"`
	InvocationAttestation *admission.ExplicitInvocationAttestation `json:"invocation_attestation,omitempty"`
	GateAttestation       *GateAttestation                         `json:"gate_attestation,omitempty"`
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
	SchemaVersion    string                `json:"schema_version"`
	ID               string                `json:"id"`
	WorkflowID       string                `json:"workflow_id"`
	GrantID          string                `json:"grant_id"`
	BundleID         string                `json:"bundle_id"`
	BundleGeneration uint64                `json:"bundle_generation"`
	Cursor           execution.GraphCursor `json:"cursor"`
	Resource         string                `json:"resource"`
	PhysicalRoot     string                `json:"physical_root"`
	AcquiredRevision uint64                `json:"acquired_revision"`
	ReleasedRevision uint64                `json:"released_revision,omitempty"`
	Digest           string                `json:"digest"`
}

type GateDecision string

const (
	GateSatisfied GateDecision = "satisfied"
	GateRejected  GateDecision = "rejected"
)

type GateAttestation struct {
	SchemaVersion    string                   `json:"schema_version"`
	WorkflowID       string                   `json:"workflow_id"`
	BundleID         string                   `json:"bundle_id"`
	BundleGeneration uint64                   `json:"bundle_generation"`
	BundleDigest     string                   `json:"bundle_digest"`
	Cursor           execution.GraphCursor    `json:"cursor"`
	GateID           string                   `json:"gate_id"`
	Authority        catalog.GateAuthority    `json:"authority"`
	Decision         GateDecision             `json:"decision"`
	Evidence         []host.EvidenceReference `json:"evidence"`
	Digest           string                   `json:"digest"`
}

type ProjectionLag struct {
	Revision uint64 `json:"revision"`
	Digest   string `json:"digest"`
	Reason   string `json:"reason"`
}

type Snapshot struct {
	SchemaVersion          string                                    `json:"schema_version"`
	WorkflowID             string                                    `json:"workflow_id"`
	RequestID              string                                    `json:"request_id"`
	DeliverableID          string                                    `json:"deliverable_id"`
	Revision               uint64                                    `json:"revision"`
	Status                 Status                                    `json:"status"`
	Classification         classification.ClassificationDecision     `json:"classification"`
	Bundles                []core.LifecycleBundle                    `json:"bundles"`
	ActiveGeneration       uint64                                    `json:"active_generation"`
	Cursor                 execution.GraphCursor                     `json:"cursor"`
	ActiveTicket           string                                    `json:"active_ticket"`
	ActiveGrant            *admission.CapabilityGrant                `json:"active_grant,omitempty"`
	GrantHistory           []admission.CapabilityGrant               `json:"grant_history"`
	UserAuthorizations     []admission.UserAuthorization             `json:"user_authorizations"`
	InvocationAttestations []admission.ExplicitInvocationAttestation `json:"invocation_attestations"`
	GateAttestations       []GateAttestation                         `json:"gate_attestations"`
	Receipts               []host.InvocationReceipt                  `json:"receipts"`
	ResourceLeases         []ResourceLease                           `json:"resource_leases"`
	LastStableBoundary     string                                    `json:"last_stable_boundary"`
	ProcessedMessages      []ProcessedMessage                        `json:"processed_messages"`
	ProjectionLag          []ProjectionLag                           `json:"projection_lag"`
}

type DispatchPacket struct {
	SchemaVersion           string                                   `json:"schema_version"`
	ID                      string                                   `json:"id"`
	WorkflowID              string                                   `json:"workflow_id"`
	RequestID               string                                   `json:"request_id"`
	BundleID                string                                   `json:"bundle_id"`
	BundleGeneration        uint64                                   `json:"bundle_generation"`
	BundleDigest            string                                   `json:"bundle_digest"`
	Cursor                  execution.GraphCursor                    `json:"cursor"`
	TargetKind              admission.GrantTargetKind                `json:"target_kind"`
	Ticket                  string                                   `json:"ticket,omitempty"`
	Topology                execution.Topology                       `json:"topology"`
	HostSessionDigest       string                                   `json:"host_session_digest"`
	EnvironmentReportDigest string                                   `json:"environment_report_digest"`
	Grant                   admission.CapabilityGrant                `json:"grant"`
	Authorization           *admission.UserAuthorization             `json:"authorization,omitempty"`
	InvocationAttestation   *admission.ExplicitInvocationAttestation `json:"invocation_attestation,omitempty"`
	InputReferences         []ArtifactReference                      `json:"input_references"`
	EvidenceRequirements    []EvidenceRequirement                    `json:"evidence_requirements"`
	EnvironmentRequirements []execution.EnvironmentRequirement       `json:"environment_requirements"`
	Digest                  string                                   `json:"digest"`
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
		if value.Prepare.Authorization != nil {
			authorization, err := admission.NewUserAuthorization(*value.Prepare.Authorization)
			if err != nil || !reflect.DeepEqual(authorization, *value.Prepare.Authorization) {
				return Command{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "User Authorization is not a canonical Host record", err)
			}
			value.Prepare.Authorization = &authorization
		}
		if value.Prepare.InvocationAttestation != nil {
			attestation, err := admission.NewExplicitInvocationAttestation(*value.Prepare.InvocationAttestation)
			if err != nil || !reflect.DeepEqual(attestation, *value.Prepare.InvocationAttestation) {
				return Command{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "Explicit Invocation Attestation is not a canonical Host record", err)
			}
			value.Prepare.InvocationAttestation = &attestation
		}
		if value.Prepare.GateAttestation != nil {
			attestation, err := normalizeGateAttestation(*value.Prepare.GateAttestation)
			if err != nil || !reflect.DeepEqual(attestation, *value.Prepare.GateAttestation) {
				return Command{}, coordinatorError("WORKFLOW_COMMAND_INVALID", "Gate Attestation is not canonical", err)
			}
			value.Prepare.GateAttestation = &attestation
		}
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
	if value.SchemaVersion != WorkflowCommandSchemaV2 {
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
	gateOnly := value.GateAttestation != nil
	if len(value.RequestedEffects) > 128 || len(value.RequestedResources) > 128 ||
		!gateOnly && (len(value.RequestedEffects) == 0 || len(value.RequestedResources) == 0 || !validText(value.TerminationCondition, 2048)) ||
		gateOnly && (len(value.RequestedEffects) != 0 || len(value.RequestedResources) != 0 || value.TerminationCondition != "" || len(value.InputReferences) != 0 || len(value.EvidenceRequirements) != 0 || value.Authorization != nil || value.InvocationAttestation != nil) ||
		len(value.InputReferences) > 128 || len(value.EvidenceRequirements) > 128 {
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
	if value.Receipt.SchemaVersion != host.HostInvocationReceiptSchemaV3 || !validDigest(value.Receipt.Digest) ||
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
	if value.Environment.Topology != value.Selection.Topology {
		return coordinatorError("WORKFLOW_COMMAND_INVALID", "SWITCH environment topology does not match Profile selection", nil)
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
	value.Alternatives = append([]profile.AlternativeChoice{}, value.Alternatives...)
	value.Overlays = append([]string{}, value.Overlays...)
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
	if value.SchemaVersion != WorkflowResultSchemaV2 {
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
		if err := validateSnapshot(*value.Snapshot, value.WorkflowID, value.Revision, false); err != nil {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid STATE Result snapshot", err)
		}
	case ResultDispatch:
		if value.Snapshot == nil || value.Dispatch == nil || !validResultIdentity(value) || value.Snapshot.WorkflowID != value.WorkflowID ||
			value.Snapshot.Revision != value.Revision || value.Dispatch.WorkflowID != value.WorkflowID {
			return coordinatorError("WORKFLOW_RESULT_INVALID", "invalid DISPATCH Result", nil)
		}
		if err := validateDispatchPacket(*value.Dispatch); err != nil {
			return err
		}
		if value.Snapshot.Status != StatusPrepared || value.Snapshot.ActiveGrant == nil ||
			!sameCanonicalValue(*value.Snapshot.ActiveGrant, value.Dispatch.Grant) {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "DISPATCH Result active Grant does not match Packet Grant", nil)
		}
		if value.Dispatch.RequestID != value.Snapshot.RequestID || value.Dispatch.Cursor != value.Snapshot.Cursor ||
			value.Dispatch.BundleGeneration != value.Snapshot.ActiveGeneration || value.Dispatch.Ticket != value.Snapshot.ActiveTicket {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet does not match active Workflow state", nil)
		}
		bundle, err := activeBundle(*value.Snapshot)
		if err != nil {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet does not match active Workflow Bundle", err)
		}
		if err := validateDispatchBundleClosure(*value.Dispatch, bundle); err != nil {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet does not close over the active Workflow Bundle", err)
		}
		if err := validateSnapshot(*value.Snapshot, value.WorkflowID, value.Revision, false); err != nil {
			return coordinatorError("WORKFLOW_STATE_REVISION_INVALID", "invalid DISPATCH Result snapshot", err)
		}
	default:
		return coordinatorError("WORKFLOW_RESULT_INVALID", "unknown Workflow Result kind", nil)
	}
	return nil
}

func validResultIdentity(value Result) bool {
	return validText(value.WorkflowID, 512) && value.Revision > 0 && validDigest(value.RevisionDigest)
}

func validateDispatchPacket(value DispatchPacket) error {
	if value.SchemaVersion != DispatchPacketSchemaV2 || !validStableID("dispatch-", value.ID) || !validWorkflowID(value.WorkflowID) ||
		!validText(value.RequestID, 512) || !validStableID("bundle-", value.BundleID) || value.BundleGeneration == 0 ||
		!validDigest(value.BundleDigest) || execution.ValidateGraphCursor(value.Cursor) != nil ||
		(value.TargetKind != admission.GrantProviderBinding && value.TargetKind != admission.GrantHostAction) ||
		(value.Topology != execution.TopologyCurrent && value.Topology != execution.TopologySubagent) || !validDigest(value.HostSessionDigest) || !validDigest(value.EnvironmentReportDigest) ||
		!validDigest(value.Digest) {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "invalid Dispatch Packet identity", nil)
	}
	if err := admission.ValidateGrant(value.Grant); err != nil {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet Grant is invalid", err)
	}
	if value.Grant.WorkflowID != value.WorkflowID || value.Grant.RequestID != value.RequestID || value.Grant.BundleID != value.BundleID || value.Grant.BundleGeneration != value.BundleGeneration ||
		value.Grant.BundleDigest != value.BundleDigest || value.Grant.Cursor != value.Cursor || value.Grant.Target.TargetKind != value.TargetKind || value.Grant.Topology != value.Topology ||
		value.Grant.HostSessionDigest != value.HostSessionDigest {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet identity does not match Grant", nil)
	}
	if value.Authorization != nil {
		normalized, err := admission.NewUserAuthorization(*value.Authorization)
		if err != nil || !reflect.DeepEqual(normalized, *value.Authorization) || normalized.Digest != value.Grant.AuthorizationDigest {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet Authorization does not match Grant", err)
		}
	}
	if value.InvocationAttestation != nil {
		normalized, err := admission.NewExplicitInvocationAttestation(*value.InvocationAttestation)
		if err != nil || !reflect.DeepEqual(normalized, *value.InvocationAttestation) || normalized.Digest != value.Grant.InvocationAttestationDigest {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet Invocation Attestation does not match Grant", err)
		}
	}
	if value.Grant.AuthorizationDigest != "" && value.Authorization == nil || value.Grant.InvocationAttestationDigest != "" && value.InvocationAttestation == nil {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet omits required Host authority", nil)
	}
	if value.Ticket != "" && !validText(value.Ticket, 512) {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "invalid Dispatch Packet ticket", nil)
	}
	for index, reference := range value.InputReferences {
		if !validText(reference.Kind, 128) || !validText(reference.Reference, 2048) || !validDigest(reference.Digest) ||
			index > 0 && artifactReferenceKey(value.InputReferences[index-1]) >= artifactReferenceKey(reference) {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet input references are not canonical", nil)
		}
	}
	for index, requirement := range value.EvidenceRequirements {
		if !validText(requirement.Kind, 128) || requirement.Minimum == 0 || !validText(requirement.Description, 2048) ||
			index > 0 && evidenceRequirementKey(value.EvidenceRequirements[index-1]) >= evidenceRequirementKey(requirement) {
			return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet evidence requirements are not canonical", nil)
		}
	}
	normalizedRequirements, err := execution.NormalizeRequirements(value.EnvironmentRequirements)
	if err != nil || !sameCanonicalValue(normalizedRequirements, value.EnvironmentRequirements) {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet environment requirements are not canonical", err)
	}
	seed := value
	seed.ID, seed.Digest = "", ""
	digest, _, err := canonicaljson.Digest(seed)
	if err != nil || value.ID != "dispatch-"+digest[:32] {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet ID does not match content", err)
	}
	unsigned := value
	unsigned.Digest = ""
	digest, _, err = canonicaljson.Digest(unsigned)
	if err != nil || digest != value.Digest {
		return coordinatorError("WORKFLOW_DISPATCH_INVALID", "Dispatch Packet digest does not match content", err)
	}
	return nil
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
		start.Selection.Alternatives = append([]profile.AlternativeChoice{}, start.Selection.Alternatives...)
		start.Selection.Overlays = append([]string{}, start.Selection.Overlays...)
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
		if prepare.Authorization != nil {
			authorization := admission.CloneUserAuthorization(*prepare.Authorization)
			prepare.Authorization = &authorization
		}
		if prepare.InvocationAttestation != nil {
			attestation := admission.CloneExplicitInvocationAttestation(*prepare.InvocationAttestation)
			prepare.InvocationAttestation = &attestation
		}
		if prepare.GateAttestation != nil {
			attestation := cloneGateAttestation(*prepare.GateAttestation)
			prepare.GateAttestation = &attestation
		}
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
		switchInput.Selection.Alternatives = append([]profile.AlternativeChoice{}, switchInput.Selection.Alternatives...)
		switchInput.Selection.Overlays = append([]string{}, switchInput.Selection.Overlays...)
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
