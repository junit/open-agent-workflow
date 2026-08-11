package codexbridge

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
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

// WorkflowCommandInput is a closed Bridge projection. Host authorization,
// invocation, gate, session, environment, Bundle, Cursor, and Dispatch facts
// are deliberately absent and are hydrated from the current trusted state.
type WorkflowCommandInput struct {
	SchemaVersion    string                  `json:"schema_version"`
	Kind             coordinator.CommandKind `json:"kind"`
	MessageID        string                  `json:"message_id"`
	IdempotencyKey   string                  `json:"idempotency_key"`
	WorkflowID       string                  `json:"workflow_id"`
	ExpectedRevision uint64                  `json:"expected_revision"`
	Start            *WorkflowStartInput     `json:"start,omitempty"`
	Prepare          *WorkflowPrepareInput   `json:"prepare,omitempty"`
	Receipt          *WorkflowReceiptInput   `json:"receipt,omitempty"`
	Switch           *WorkflowSwitchInput    `json:"switch,omitempty"`
	Cancel           *WorkflowCancelInput    `json:"cancel,omitempty"`
}

type WorkflowStartInput struct {
	RequestID     string                                `json:"request_id"`
	DeliverableID string                                `json:"deliverable_id"`
	InputDigest   string                                `json:"input_digest"`
	ActiveTicket  string                                `json:"active_ticket"`
	Proposal      classification.ClassificationProposal `json:"proposal"`
	Selection     core.Selection                        `json:"selection"`
}

type WorkflowArtifactReference struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

type WorkflowEvidenceRequirement struct {
	Kind        string `json:"kind"`
	Minimum     uint64 `json:"minimum"`
	Description string `json:"description"`
}

type WorkflowPrepareInput struct {
	RequestedEffects     []string                      `json:"requested_effects"`
	RequestedResources   []string                      `json:"requested_resources"`
	TerminationCondition string                        `json:"termination_condition"`
	InputReferences      []WorkflowArtifactReference   `json:"input_references"`
	EvidenceRequirements []WorkflowEvidenceRequirement `json:"evidence_requirements"`
}

type WorkflowReceiptInput struct {
	Kind           host.ReceiptKind         `json:"kind"`
	Outcome        string                   `json:"outcome"`
	FailureCode    string                   `json:"failure_code"`
	Outputs        []host.OutputReference   `json:"outputs"`
	Evidence       []host.EvidenceReference `json:"evidence"`
	Signal         string                   `json:"signal"`
	StableBoundary string                   `json:"stable_boundary"`
}

type WorkflowSwitchInput struct {
	Boundary  string         `json:"boundary"`
	Selection core.Selection `json:"selection"`
}

type WorkflowCancelInput struct {
	Reason             string `json:"reason"`
	InvocationTerminal bool   `json:"invocation_terminal"`
}

func (input WorkflowCommandInput) coordinatorCommand(facts Facts, current coordinator.Result) (coordinator.Command, error) {
	command := coordinator.Command{
		SchemaVersion: input.SchemaVersion, Kind: input.Kind, MessageID: input.MessageID,
		IdempotencyKey: input.IdempotencyKey, WorkflowID: input.WorkflowID, ExpectedRevision: input.ExpectedRevision,
	}
	if input.Start != nil {
		command.Start = &coordinator.StartInput{
			RequestID: input.Start.RequestID, DeliverableID: input.Start.DeliverableID,
			InputDigest: input.Start.InputDigest, ActiveTicket: input.Start.ActiveTicket,
			Proposal: input.Start.Proposal, Selection: input.Start.Selection,
			HostSession: facts.Session, Environment: facts.Environment,
		}
	}
	if input.Prepare != nil {
		prepare := &coordinator.PrepareInput{
			RequestedEffects: append([]string{}, input.Prepare.RequestedEffects...), RequestedResources: append([]string{}, input.Prepare.RequestedResources...),
			TerminationCondition: input.Prepare.TerminationCondition,
			InputReferences:      make([]coordinator.ArtifactReference, len(input.Prepare.InputReferences)),
			EvidenceRequirements: make([]coordinator.EvidenceRequirement, len(input.Prepare.EvidenceRequirements)),
		}
		for index, value := range input.Prepare.InputReferences {
			prepare.InputReferences[index] = coordinator.ArtifactReference{Kind: value.Kind, Reference: value.Reference, Digest: value.Digest}
		}
		for index, value := range input.Prepare.EvidenceRequirements {
			prepare.EvidenceRequirements[index] = coordinator.EvidenceRequirement{Kind: value.Kind, Minimum: value.Minimum, Description: value.Description}
		}
		command.Prepare = prepare
	}
	if input.Receipt != nil {
		receipt, err := hydrateReceipt(input.WorkflowID, *input.Receipt, facts, current)
		if err != nil {
			return coordinator.Command{}, err
		}
		command.Receipt = &coordinator.ReceiptInput{Receipt: receipt, Signal: input.Receipt.Signal, StableBoundary: input.Receipt.StableBoundary}
	}
	if input.Switch != nil {
		command.Switch = &coordinator.SwitchInput{
			Boundary: input.Switch.Boundary, Selection: input.Switch.Selection,
			HostSession: facts.Session, Environment: facts.Environment,
		}
	}
	if input.Cancel != nil {
		command.Cancel = &coordinator.CancelInput{Reason: input.Cancel.Reason, InvocationTerminal: input.Cancel.InvocationTerminal}
	}
	return command, nil
}

func hydrateReceipt(workflowID string, input WorkflowReceiptInput, facts Facts, current coordinator.Result) (host.InvocationReceipt, error) {
	if current.Snapshot == nil || current.Snapshot.WorkflowID != workflowID {
		return host.InvocationReceipt{}, NewError("HOST_SESSION_CHANGED", "Workflow state is unavailable for Receipt hydration", nil)
	}
	bundle, found := activeBundle(current.Snapshot)
	if !found {
		return host.InvocationReceipt{}, NewError("HOST_SESSION_CHANGED", "active Bundle is unavailable for Receipt hydration", nil)
	}
	dispatchDigest := ""
	if current.Dispatch != nil && current.Dispatch.WorkflowID == workflowID && current.Dispatch.BundleDigest == bundle.Digest && current.Dispatch.Cursor == current.Snapshot.Cursor {
		dispatchDigest = current.Dispatch.Digest
	}
	if dispatchDigest == "" {
		for index := len(current.Snapshot.Receipts) - 1; index >= 0; index-- {
			candidate := current.Snapshot.Receipts[index]
			if candidate.WorkflowID == workflowID && candidate.BundleDigest == bundle.Digest && candidate.Cursor == current.Snapshot.Cursor {
				dispatchDigest = candidate.DispatchDigest
				break
			}
		}
	}
	if dispatchDigest == "" {
		return host.InvocationReceipt{}, NewError("HOST_SESSION_CHANGED", "active Dispatch is unavailable for Receipt hydration", nil)
	}
	receipt, err := host.NewInvocationReceipt(host.InvocationReceipt{
		SchemaVersion: host.HostInvocationReceiptSchemaV3, Kind: input.Kind, WorkflowID: workflowID,
		BundleID: bundle.ID, BundleGeneration: bundle.Generation, BundleDigest: bundle.Digest,
		Cursor: current.Snapshot.Cursor, Topology: bundle.Topology, HostSessionDigest: facts.Session.Digest,
		DispatchDigest: dispatchDigest, ContextFreshness: host.ContextShared, EnvironmentReportDigest: facts.Environment.Digest,
		Outcome: input.Outcome, FailureCode: input.FailureCode,
		Outputs: append([]host.OutputReference{}, input.Outputs...), Evidence: append([]host.EvidenceReference{}, input.Evidence...),
	})
	if err != nil {
		return host.InvocationReceipt{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "Receipt outcome is invalid", err)
	}
	return receipt, nil
}

func activeBundle(snapshot *coordinator.Snapshot) (core.LifecycleBundle, bool) {
	if snapshot == nil {
		return core.LifecycleBundle{}, false
	}
	for _, bundle := range snapshot.Bundles {
		if bundle.Generation == snapshot.ActiveGeneration {
			return bundle, true
		}
	}
	return core.LifecycleBundle{}, false
}

type ProviderStateSummary struct {
	ProviderID string                 `json:"provider_id"`
	State      registry.ProviderState `json:"state"`
}

type HostSummary struct {
	SessionDigest         string                 `json:"session_digest"`
	InventoryDigest       string                 `json:"inventory_digest"`
	EnvironmentDigest     string                 `json:"environment_digest"`
	VersionEvidenceDigest string                 `json:"version_evidence_digest"`
	Providers             []ProviderStateSummary `json:"providers"`
	Diagnostics           []Diagnostic           `json:"diagnostics"`
	DirectAvailable       bool                   `json:"direct_available"`
}

type ObserveCurrentOutput struct {
	HostEvidenceHandle HostEvidenceHandle `json:"host_evidence_handle"`
	HostSummary        HostSummary        `json:"host_summary"`
}

type CoreInspectOutput struct {
	Classification classification.ClassificationDecision `json:"classification"`
	HostSummary    HostSummary                           `json:"host_summary"`
	Compilation    *core.CompilationResult               `json:"compilation,omitempty"`
	Builder        *profile.BuilderProjection            `json:"builder,omitempty"`
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
