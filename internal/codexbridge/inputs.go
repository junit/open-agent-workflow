package codexbridge

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type ObserveCurrentInput struct{}

type CoreInspectInput struct {
	HostEvidenceHandle HostEvidenceHandle                    `json:"host_evidence_handle"`
	DeliverableID      string                                `json:"deliverable_id"`
	InputDigest        string                                `json:"input_digest"`
	Proposal           classification.ClassificationProposal `json:"proposal"`
}

type CoreCompileInput struct {
	HostEvidenceHandle HostEvidenceHandle                    `json:"host_evidence_handle"`
	DeliverableID      string                                `json:"deliverable_id"`
	InputDigest        string                                `json:"input_digest"`
	Proposal           classification.ClassificationProposal `json:"proposal"`
	Selection          core.Selection                        `json:"selection"`
}

type WorkflowExchangeInput struct {
	HostEvidenceHandle HostEvidenceHandle   `json:"host_evidence_handle"`
	Command            WorkflowCommandInput `json:"command"`
}

// WorkflowCommandInput is the public Bridge projection of a Coordinator
// command. HostSession and Environment are deliberately absent: those facts
// are Host-owned and are hydrated from the verified evidence handle.
type WorkflowCommandInput struct {
	SchemaVersion    string                    `json:"schema_version"`
	Kind             coordinator.CommandKind   `json:"kind"`
	MessageID        string                    `json:"message_id"`
	IdempotencyKey   string                    `json:"idempotency_key"`
	WorkflowID       string                    `json:"workflow_id"`
	ExpectedRevision uint64                    `json:"expected_revision"`
	Start            *WorkflowStartInput       `json:"start,omitempty"`
	Prepare          *coordinator.PrepareInput `json:"prepare,omitempty"`
	Receipt          *coordinator.ReceiptInput `json:"receipt,omitempty"`
	Switch           *WorkflowSwitchInput      `json:"switch,omitempty"`
	Cancel           *coordinator.CancelInput  `json:"cancel,omitempty"`
}

type WorkflowStartInput struct {
	RequestID     string                                `json:"request_id"`
	DeliverableID string                                `json:"deliverable_id"`
	InputDigest   string                                `json:"input_digest"`
	ActiveTicket  string                                `json:"active_ticket"`
	Proposal      classification.ClassificationProposal `json:"proposal"`
	Selection     core.Selection                        `json:"selection"`
}

type WorkflowSwitchInput struct {
	Boundary  string         `json:"boundary"`
	Selection core.Selection `json:"selection"`
}

func (input WorkflowCommandInput) coordinatorCommand(facts Facts) (coordinator.Command, error) {
	command := coordinator.Command{
		SchemaVersion: input.SchemaVersion, Kind: input.Kind, MessageID: input.MessageID,
		IdempotencyKey: input.IdempotencyKey, WorkflowID: input.WorkflowID,
		ExpectedRevision: input.ExpectedRevision, Prepare: input.Prepare,
		Cancel: input.Cancel,
	}
	if input.Receipt != nil {
		receipt := *input.Receipt
		normalized, err := host.NewInvocationReceipt(receipt.Receipt)
		if err != nil {
			return coordinator.Command{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Receipt is not a valid Host record", err)
		}
		receipt.Receipt = normalized
		command.Receipt = &receipt
	}
	if input.Start != nil {
		command.Start = &coordinator.StartInput{
			RequestID: input.Start.RequestID, DeliverableID: input.Start.DeliverableID,
			InputDigest: input.Start.InputDigest, ActiveTicket: input.Start.ActiveTicket,
			Proposal: input.Start.Proposal, Selection: input.Start.Selection,
			HostSession: facts.Session, Environment: facts.Environment,
		}
	}
	if input.Switch != nil {
		command.Switch = &coordinator.SwitchInput{
			Boundary: input.Switch.Boundary, Selection: input.Switch.Selection,
			HostSession: facts.Session, Environment: facts.Environment,
		}
	}
	return command, nil
}

type ProviderStateSummary struct {
	ProviderID string                 `json:"provider_id"`
	State      registry.ProviderState `json:"state"`
}

type HostSummary struct {
	SessionDigest     string                 `json:"session_digest"`
	InventoryDigest   string                 `json:"inventory_digest"`
	EnvironmentDigest string                 `json:"environment_digest"`
	Providers         []ProviderStateSummary `json:"providers"`
	Diagnostics       []Diagnostic           `json:"diagnostics"`
	DirectAvailable   bool                   `json:"direct_available"`
}

type ObserveCurrentOutput struct {
	HostEvidenceHandle HostEvidenceHandle `json:"host_evidence_handle"`
	HostSummary        HostSummary        `json:"host_summary"`
}

type CoreInspectOutput struct {
	Classification classification.ClassificationDecision `json:"classification"`
	HostSummary    HostSummary                           `json:"host_summary"`
	Compilation    *core.CompilationResult               `json:"compilation,omitempty"`
}

func DecodeObserveCurrentInput(raw []byte) (ObserveCurrentInput, error) {
	return decodePublicInput[ObserveCurrentInput](raw)
}

func DecodeCoreInspectInput(raw []byte) (CoreInspectInput, error) {
	return decodePublicInput[CoreInspectInput](raw)
}

func DecodeCoreCompileInput(raw []byte) (CoreCompileInput, error) {
	return decodePublicInput[CoreCompileInput](raw)
}

func DecodeWorkflowExchangeInput(raw []byte) (WorkflowExchangeInput, error) {
	return decodePublicInput[WorkflowExchangeInput](raw)
}

func decodePublicInput[T any](raw []byte) (T, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var input T
	if err := decoder.Decode(&input); err != nil {
		return input, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "unknown or malformed public field", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return input, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "trailing JSON value", err)
	}
	return input, nil
}
