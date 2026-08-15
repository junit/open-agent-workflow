package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const privateHookContextField = "_oaw_host_context"

func NewMCPServer(service *Service, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "oaw-codex-bridge", Version: version}, nil)
	server.AddTool(
		closedTool[ObserveProfileInput, ObserveProfileOutput](
			"observe_profile", "Issue an Assurance Overlay from current Codex Binding observations",
		),
		service.observeProfileTool,
	)
	return server
}

func ServeStdio(ctx context.Context, service *Service, version string) error {
	return NewMCPServer(service, version).Run(ctx, &mcp.StdioTransport{})
}

func closedTool[Input, Output any](name, description string) *mcp.Tool {
	closedWorld := false
	return &mcp.Tool{
		Name: name, Description: description,
		InputSchema: closedSchema[Input](), OutputSchema: closedSchema[Output](),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld},
	}
}

func closedSchema[T any]() map[string]any {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("infer MCP schema: %v", err))
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("marshal MCP schema: %v", err))
	}
	var projected map[string]any
	if err := json.Unmarshal(raw, &projected); err != nil {
		panic(fmt.Sprintf("project MCP schema: %v", err))
	}
	closeObjectSchemas(projected)
	return projected
}

func closeObjectSchemas(value any) {
	switch typed := value.(type) {
	case map[string]any:
		if typed["type"] == "object" {
			typed["additionalProperties"] = false
		}
		for _, nested := range typed {
			closeObjectSchemas(nested)
		}
	case []any:
		for _, nested := range typed {
			closeObjectSchemas(nested)
		}
	}
}

func (service *Service) observeProfileTool(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	public, hostContext, err := observeArguments(request)
	if err != nil {
		return toolError(err), nil
	}
	input, err := DecodeObserveProfileInput(public)
	if err != nil {
		return toolError(err), nil
	}
	output, err := service.ObserveProfile(ctx, input, hostContext)
	if err != nil {
		return toolError(err), nil
	}
	return successfulToolResult(output, "Issued an Assurance Overlay from current Codex Binding observations."), nil
}

func observeArguments(request *mcp.CallToolRequest) ([]byte, HookContext, error) {
	raw := rawToolArguments(request)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return nil, HookContext{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "observe_profile arguments are malformed", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, HookContext{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "observe_profile has trailing arguments", err)
	}
	contextRaw, found := fields[privateHookContextField]
	if !found {
		return nil, HookContext{}, NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "trusted Hook context is required", nil)
	}
	delete(fields, privateHookContextField)
	hostContext, err := decodePublicInput[HookContext](contextRaw)
	if err != nil {
		return nil, HookContext{}, NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "trusted Hook context is malformed", err)
	}
	if err := ValidateHookContext(hostContext); err != nil {
		return nil, HookContext{}, NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "trusted Hook context is malformed", err)
	}
	public, err := json.Marshal(fields)
	if err != nil {
		return nil, HookContext{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "observe_profile arguments cannot be projected", err)
	}
	return public, hostContext, nil
}

func rawToolArguments(request *mcp.CallToolRequest) []byte {
	if request == nil || request.Params == nil || len(request.Params.Arguments) == 0 {
		return []byte("{}")
	}
	return request.Params.Arguments
}

func successfulToolResult(output any, summary string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: summary}}, StructuredContent: output,
	}
}

func toolError(err error) *mcp.CallToolResult {
	diagnostic := ProjectDiagnostic(normalizeToolError(err))
	text := diagnostic.Code + ": " + diagnostic.Detail + ". Recovery: " + diagnostic.RecoveryAction
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}}, StructuredContent: diagnostic, IsError: true,
	}
}

func normalizeToolError(err error) error {
	if err == nil || Code(err) != "" {
		return err
	}
	return NewError("HOST_BRIDGE_OPERATION_FAILED", "Bridge observation failed", err)
}
