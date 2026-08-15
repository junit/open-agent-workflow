package codexbridge

import (
	"errors"
	"testing"
)

func TestBridgeV3AcceptsOnlyObserveProfile(t *testing.T) {
	got, err := ParseOperation("observe_profile")
	if err != nil || got != OperationObserveProfile {
		t.Fatalf("ParseOperation(observe_profile) = %q, %v", got, err)
	}
	for _, retired := range []string{"observe_current", "core.inspect", "core.compile", "workflow_exchange"} {
		if _, err := ParseOperation(retired); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
			t.Fatalf("ParseOperation(%q) error = %v", retired, err)
		}
	}
	if BridgeProtocolVersion != "oaw.codex-bridge/v3" || HookContextSchemaV3 != "oaw.codex-hook-context/v3" ||
		BridgeIntegrationVersion != "3.0.0" {
		t.Fatalf("Bridge tuple = %q, %q, %q", BridgeProtocolVersion, HookContextSchemaV3, BridgeIntegrationVersion)
	}
}

func TestHookContextIsSessionAndProjectBound(t *testing.T) {
	context := bridgeTestContext("/repo")
	firstSession, firstCWD, err := ContextDigestHeaders(context)
	if err != nil {
		t.Fatal(err)
	}
	context.SessionID = "session-other"
	secondSession, secondCWD, err := ContextDigestHeaders(context)
	if err != nil {
		t.Fatal(err)
	}
	if firstSession == secondSession || firstCWD != secondCWD {
		t.Fatalf("context digests = %q/%q and %q/%q", firstSession, firstCWD, secondSession, secondCWD)
	}
	context.SchemaVersion = "oaw.codex-hook-context/v2"
	if Code(ValidateHookContext(context)) != "HOST_BRIDGE_CONTEXT_REQUIRED" {
		t.Fatal("ValidateHookContext accepted the retired v2 context")
	}
}

func TestNewErrorCarriesLayerAndCause(t *testing.T) {
	cause := errors.New("fixture")
	err := NewError("HOST_OBSERVATION_FAILED", "observation failed", cause)
	value, ok := err.(*Error)
	if !ok || value.Cause != cause || Code(err) != "HOST_OBSERVATION_FAILED" || !errors.Is(err, cause) {
		t.Fatalf("error = %#v", err)
	}
	if err.Error() != "HOST_OBSERVATION_FAILED: observation failed" || Code(errors.New("plain")) != "" || Code(nil) != "" {
		t.Fatal("error formatting or code projection changed")
	}
}
