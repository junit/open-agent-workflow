package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
)

const privateHookContextField = "_oaw_host_context"

func NewMCPServer(service *Service, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "oaw-codex-bridge", Version: version}, nil)
	server.AddTool(closedTool[ObserveCurrentInput, ObserveCurrentOutput]("observe_current", "Observe the current Codex Host facts", true), service.observeTool)
	server.AddTool(closedTool[CoreInspectInput, CoreInspectOutput]("core_inspect", "Inspect verified Providers and Profile eligibility", true), service.inspectTool)
	server.AddTool(closedTool[CoreCompileInput, core.CompilationResult]("core_compile", "Compile one explicit Lifecycle Bundle", true), service.compileTool)
	server.AddTool(closedTool[WorkflowExchangeInput, coordinator.Result]("workflow_exchange", "Exchange one Coordinator command", false), service.workflowTool)
	return server
}

func ServeStdio(ctx context.Context, service *Service, version string) error {
	return NewMCPServer(service, version).Run(ctx, &mcp.StdioTransport{})
}

func closedTool[Input, Output any](name, description string, readOnly bool) *mcp.Tool {
	closedWorld := false
	return &mcp.Tool{
		Name: name, Description: description,
		InputSchema: closedSchema[Input](), OutputSchema: closedSchema[Output](),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, OpenWorldHint: &closedWorld},
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

func (service *Service) observeTool(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	public, hostContext, err := observeArguments(request)
	if err != nil {
		return service.toolError(err, HostEvidenceHandle{}), nil
	}
	input, err := DecodeObserveCurrentInput(public)
	if err != nil {
		return service.toolError(err, HostEvidenceHandle{}), nil
	}
	output, err := service.ObserveCurrent(ctx, input, hostContext)
	if err != nil {
		return service.toolError(err, HostEvidenceHandle{}), nil
	}
	return successfulToolResult(output, "Observed current Codex Host facts."), nil
}

func (service *Service) inspectTool(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := DecodeCoreInspectInput(rawToolArguments(request))
	if err != nil {
		return service.toolError(err, HostEvidenceHandle{}), nil
	}
	output, err := service.CoreInspect(ctx, input)
	if err != nil {
		return service.toolError(err, input.HostEvidenceHandle), nil
	}
	return successfulToolResult(output, "Inspected current Provider and Profile eligibility."), nil
}

func (service *Service) compileTool(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := DecodeCoreCompileInput(rawToolArguments(request))
	if err != nil {
		return service.toolError(err, HostEvidenceHandle{}), nil
	}
	output, err := service.CoreCompile(ctx, input)
	if err != nil {
		return service.toolError(err, input.HostEvidenceHandle), nil
	}
	return successfulToolResult(output, "Compiled the selected Lifecycle Bundle."), nil
}

func (service *Service) workflowTool(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	input, err := DecodeWorkflowExchangeInput(rawToolArguments(request))
	if err != nil {
		return service.toolError(err, HostEvidenceHandle{}), nil
	}
	output, err := service.WorkflowExchange(ctx, input)
	if err != nil {
		return service.toolError(err, input.HostEvidenceHandle), nil
	}
	return successfulToolResult(output, "Processed the Workflow Coordinator command."), nil
}

func observeArguments(request *mcp.CallToolRequest) ([]byte, HookContext, error) {
	raw := rawToolArguments(request)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return nil, HookContext{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "observe_current arguments are malformed", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, HookContext{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "observe_current has trailing arguments", err)
	}
	contextRaw, found := fields[privateHookContextField]
	if !found {
		return nil, HookContext{}, NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "trusted Hook context is required", nil)
	}
	delete(fields, privateHookContextField)
	hostContext, err := decodePublicInput[HookContext](contextRaw)
	if err != nil || !validInjectedHookContext(hostContext) {
		return nil, HookContext{}, NewError("HOST_BRIDGE_CONTEXT_REQUIRED", "trusted Hook context is malformed", err)
	}
	public, err := json.Marshal(fields)
	if err != nil {
		return nil, HookContext{}, NewError("HOST_BRIDGE_PROTOCOL_MISMATCH", "observe_current arguments cannot be projected", err)
	}
	return public, hostContext, nil
}

func validInjectedHookContext(value HookContext) bool {
	if value.SchemaVersion != HookContextSchemaV1 || value.BridgeProtocolVersion != BridgeProtocolVersion {
		return false
	}
	if _, _, err := ContextDigestHeaders(value); err != nil {
		return false
	}
	for _, field := range []string{value.TurnID, value.ToolUseID, value.Model, value.PermissionMode} {
		if !utf8.ValidString(field) || field == "" || len(field) > 4096 || strings.IndexFunc(field, unicode.IsControl) >= 0 {
			return false
		}
	}
	return true
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

func (service *Service) toolError(err error, handle HostEvidenceHandle) *mcp.CallToolResult {
	evidenceDigest := ""
	if handle.Token != "" {
		if facts, getErr := service.getFacts(handle); getErr == nil {
			evidenceDigest = facts.FactDigests.Session
		}
	}
	diagnostic := ProjectDiagnostic(normalizeToolError(err), evidenceDigest, true)
	text := diagnostic.Code + ": " + diagnostic.Detail + ". Recovery: " + diagnostic.RecoveryAction
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}}, StructuredContent: diagnostic, IsError: true,
	}
}

func normalizeToolError(err error) error {
	if err == nil || Code(err) != "" {
		return err
	}
	if code := coordinator.ErrorCode(err); code != "" {
		return NewError(code, "Workflow Coordinator rejected the operation", err)
	}
	var coreErr *core.Error
	if errors.As(err, &coreErr) && coreErr.Code != "" {
		return NewError(coreErr.Code, "OAW Core rejected the operation", err)
	}
	return NewError("HOST_BRIDGE_OPERATION_FAILED", "Bridge operation failed", err)
}
