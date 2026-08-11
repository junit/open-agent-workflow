package codexbridge

import (
	"errors"
	"testing"
)

func TestProtocolRejectsUnknownOperation(t *testing.T) {
	if _, err := ParseOperation("plugin/list"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("ParseOperation() error = %v", err)
	}
}

func TestBridgeV2ProtocolConstantsAreHardCut(t *testing.T) {
	if BridgeProtocolVersion != "oaw.codex-bridge/v2" ||
		HookContextSchemaV2 != "oaw.codex-hook-context/v2" ||
		EvidenceHandleVersion != "oaw.host-evidence-handle/v2" ||
		BridgeIntegrationVersion != "2.0.0" {
		t.Fatalf("bridge tuple = %q, %q, %q, %q", BridgeProtocolVersion, HookContextSchemaV2, EvidenceHandleVersion, BridgeIntegrationVersion)
	}
}

func TestProtocolAcceptsOnlyBridgeOperations(t *testing.T) {
	allowed := []Operation{
		OperationObserveCurrent,
		OperationCoreInspect,
		OperationCoreCompile,
		OperationWorkflowExchange,
	}
	for _, operation := range allowed {
		got, err := ParseOperation(string(operation))
		if err != nil || got != operation {
			t.Fatalf("ParseOperation(%q) = %q, %v", operation, got, err)
		}
	}
}

func TestBridgeV2ProtocolRejectsV1OperationSurface(t *testing.T) {
	for _, operation := range []string{"provider.inspect", "provider.compile", "workflow.start", "workflow.receipt"} {
		if _, err := ParseOperation(operation); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
			t.Fatalf("ParseOperation(%q) error = %v", operation, err)
		}
	}
}

func TestNewErrorCarriesLayerAndCause(t *testing.T) {
	cause := errFixture{}
	err := NewError("HOST_EVIDENCE_HANDLE_INVALID", "invalid handle", cause)
	value, ok := err.(*Error)
	if !ok {
		t.Fatalf("NewError() type = %T", err)
	}
	if value.Layer != "evidence" || value.Cause != cause || Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("error = %#v", value)
	}
}

type errFixture struct{}

func (errFixture) Error() string { return "fixture" }

func TestErrorFormattingUnwrapAndExternalCode(t *testing.T) {
	cause := errFixture{}
	withDetail := NewError("HOST_OBSERVATION_FAILED", "observation failed", cause)
	if withDetail.Error() != "HOST_OBSERVATION_FAILED: observation failed" || !errors.Is(withDetail, cause) {
		t.Fatalf("error=%v", withDetail)
	}
	withoutDetail := NewError("HOST_OBSERVATION_FAILED", "", nil)
	if withoutDetail.Error() != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("error=%v", withoutDetail)
	}
	if Code(externalCodeError{}) != "EXTERNAL_CODE" || Code(errors.New("plain")) != "" || Code(nil) != "" {
		t.Fatal("Code() did not preserve the external error boundary")
	}
}

type externalCodeError struct{}

func (externalCodeError) Error() string     { return "external" }
func (externalCodeError) ErrorCode() string { return "EXTERNAL_CODE" }
