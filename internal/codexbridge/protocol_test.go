package codexbridge

import "testing"

func TestProtocolRejectsUnknownOperation(t *testing.T) {
	if _, err := ParseOperation("plugin/list"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("ParseOperation() error = %v", err)
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
