package codexbridge

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPListsOneClosedReadOnlyAssuranceTool(t *testing.T) {
	service, _ := newBridgeTestService(t, true)
	client := connectInMemoryMCP(t, service)
	result, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "observe_profile" {
		t.Fatalf("tools = %#v", result.Tools)
	}
	tool := result.Tools[0]
	if !closedObjectSchemas(tool.InputSchema) || tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
		t.Fatalf("tool contract = %#v", tool)
	}
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"_oaw_host_context", "workflow", "classification", "receipt", "lease", "core"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("input schema exposes %q: %s", forbidden, raw)
		}
	}
}

func TestMCPObserveProfileRequiresHookContextAndReturnsOverlay(t *testing.T) {
	service, profilePath := newBridgeTestService(t, true)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(profilePath)))
	client := connectInMemoryMCP(t, service)
	missing, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "observe_profile", Arguments: map[string]any{"profile": "project:team-delivery"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !missing.IsError || !structuredHasCode(missing.StructuredContent, "HOST_BRIDGE_CONTEXT_REQUIRED") {
		t.Fatalf("missing context result = %#v", missing)
	}

	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "observe_profile",
		Arguments: map[string]any{
			"profile": "project:team-delivery", "_oaw_host_context": bridgeTestContext(projectRoot),
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("observe result = %#v, error = %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output ObserveProfileOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	if output.Overlay.Profile.ID != "team-delivery" || len(output.Overlay.Claims) != 1 {
		t.Fatalf("structured output = %#v", output)
	}
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
