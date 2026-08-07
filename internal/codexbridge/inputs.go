package codexbridge

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
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
	HostEvidenceHandle HostEvidenceHandle  `json:"host_evidence_handle"`
	Command            coordinator.Command `json:"command"`
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
