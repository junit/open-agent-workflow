package codexbridge

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
)

func TestMCPListsExactlyFourClosedTools(t *testing.T) {
	client := connectInMemoryMCP(t, newTestService(t))
	tools := collectTools(t, client)
	if got := toolNames(tools); !slices.Equal(got, []string{"core_compile", "core_inspect", "observe_current", "workflow_exchange"}) {
		t.Fatalf("tools = %v", got)
	}
	for _, tool := range tools {
		if !closedObjectSchemas(tool.InputSchema) {
			t.Fatalf("open input schema for %s: %#v", tool.Name, tool.InputSchema)
		}
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != (tool.Name != "workflow_exchange") {
			t.Fatalf("annotations for %s = %#v", tool.Name, tool.Annotations)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "_oaw_host_context") {
			t.Fatalf("private Hook context advertised by %s", tool.Name)
		}
		if tool.Name == "workflow_exchange" && (strings.Contains(string(raw), `"host_session"`) || strings.Contains(string(raw), `"environment"`)) {
			t.Fatalf("Host-owned facts advertised by %s: %s", tool.Name, raw)
		}
	}
}

func TestMCPObserveRequiresPrivateHookContextAndReturnsStructuredResult(t *testing.T) {
	service := newTestService(t)
	client := connectInMemoryMCP(t, service)
	missing, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "observe_current", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !missing.IsError || !structuredHasCode(missing.StructuredContent, "HOST_BRIDGE_CONTEXT_REQUIRED") {
		t.Fatalf("missing context result = %#v", missing)
	}

	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "observe_current",
		Arguments: map[string]any{"_oaw_host_context": testHookContext("session-1", service.projectRoot)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("observe result = %#v", result)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var observed ObserveCurrentOutput
	if err := json.Unmarshal(raw, &observed); err != nil {
		t.Fatal(err)
	}
	if observed.HostEvidenceHandle.Token == "" {
		t.Fatalf("structured result = %#v", observed)
	}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok && strings.Contains(text.Text, observed.HostEvidenceHandle.Token) {
			t.Fatalf("text summary leaked handle token: %q", text.Text)
		}
	}
}

func TestMCPErrorUsesStableDiagnosticAndPinnedEvidence(t *testing.T) {
	service := newTestService(t)
	client := connectInMemoryMCP(t, service)
	observed := callMCPObserve(t, client, service)
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "core_compile",
		Arguments: CoreCompileInput{
			HostEvidenceHandle: observed.HostEvidenceHandle, DeliverableID: "bridge-mcp-test",
			InputDigest: testDigest("mcp-input"), Proposal: workflowProposal(), Selection: core.Selection{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || !structuredHasCode(result.StructuredContent, "PROFILE_SELECTION_INVALID") {
		t.Fatalf("error result = %#v", result)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostic Diagnostic
	if err := json.Unmarshal(raw, &diagnostic); err != nil {
		t.Fatal(err)
	}
	if len(diagnostic.EvidenceDigest) != 64 || diagnostic.RecoveryAction == "" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok && (strings.Contains(text.Text, observed.HostEvidenceHandle.Token) || strings.Contains(text.Text, service.projectRoot)) {
			t.Fatalf("error text leaked private state: %q", text.Text)
		}
	}
}

func callMCPObserve(t *testing.T, client *mcp.ClientSession, service *Service) ObserveCurrentOutput {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "observe_current", Arguments: map[string]any{"_oaw_host_context": testHookContext("session-1", service.projectRoot)},
	})
	if err != nil || result.IsError {
		t.Fatalf("observe result = %#v, error = %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var observed ObserveCurrentOutput
	if err := json.Unmarshal(raw, &observed); err != nil {
		t.Fatal(err)
	}
	return observed
}

func connectInMemoryMCP(t *testing.T, service *Service) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	server := NewMCPServer(service, "test")
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "oaw-test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		<-done
	})
	return session
}

func collectTools(t *testing.T, client *mcp.ClientSession) []*mcp.Tool {
	t.Helper()
	result, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.NextCursor != "" {
		t.Fatalf("unexpected tools cursor %q", result.NextCursor)
	}
	return result.Tools
}

func toolNames(values []*mcp.Tool) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Name)
	}
	sort.Strings(result)
	return result
}

func closedObjectSchemas(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if typed["type"] == "object" && typed["additionalProperties"] != false {
			return false
		}
		for _, nested := range typed {
			if !closedObjectSchemas(nested) {
				return false
			}
		}
	case []any:
		for _, nested := range typed {
			if !closedObjectSchemas(nested) {
				return false
			}
		}
	}
	return true
}

func structuredHasCode(value any, code string) bool {
	raw, err := json.Marshal(value)
	return err == nil && strings.Contains(string(raw), `"code":"`+code+`"`)
}
