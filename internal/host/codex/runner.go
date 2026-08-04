package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	defaultCommand     = "codex"
	defaultSandbox     = "workspace-write"
	defaultOutputBytes = 4 << 20
	defaultEventLimit  = 256
)

type Runner struct {
	command        string
	maxOutputBytes int64
	maxEvents      int
	diagnostics    io.Writer
	run            func(context.Context, string, []string, int64) ([]byte, []byte, error)

	mu           sync.Mutex
	prepared     map[string]host.DispatchRequest
	results      map[string]host.DispatchResult
	attempts     map[string]*invocationAttempt
	active       map[string]context.CancelFunc
	mcpServers   map[string][]string
	mcpInventory func(context.Context, string, int64) ([]string, error)
	mcpIsolation func(context.Context, string, []string, int64) error
}

type invocationAttempt struct {
	done   chan struct{}
	result host.DispatchResult
	err    error
}

func New(options Options) *Runner {
	command := options.Command
	if command == "" {
		command = defaultCommand
	}
	maxOutputBytes := options.MaxOutputBytes
	if maxOutputBytes <= 0 {
		maxOutputBytes = defaultOutputBytes
	}
	maxEvents := options.MaxEvents
	if maxEvents <= 0 {
		maxEvents = defaultEventLimit
	}
	return &Runner{
		command: command, maxOutputBytes: maxOutputBytes, maxEvents: maxEvents,
		diagnostics: options.Diagnostics, run: runCommand,
		prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), attempts: make(map[string]*invocationAttempt), active: make(map[string]context.CancelFunc),
		mcpServers: make(map[string][]string), mcpInventory: listMCPServers, mcpIsolation: verifyMCPIsolation,
	}
}

type Options struct {
	Command        string
	MaxOutputBytes int64
	MaxEvents      int
	Diagnostics    io.Writer
}

func (runner *Runner) Prepare(ctx context.Context, request host.DispatchRequest) error {
	if ctx == nil {
		return errors.New("CODEX_CONTEXT_REQUIRED: Prepare context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("CODEX_PREPARE_CANCELLED: %w", err)
	}
	if err := host.ValidateDispatchRequest(request); err != nil {
		return err
	}
	request = host.CloneDispatchRequest(request)
	if request.Binding.Host != "codex" {
		return errors.New("CODEX_BINDING_UNSUPPORTED: Binding Host is not codex")
	}
	var mcpServers []string
	if isReadOnly(request) {
		if runner.mcpInventory == nil {
			return errors.New("CODEX_MCP_INVENTORY_FAILED: read-only dispatch requires MCP inventory")
		}
		inventory, err := runner.mcpInventory(ctx, runner.command, runner.maxOutputBytes)
		if err != nil {
			return fmt.Errorf("CODEX_MCP_INVENTORY_FAILED: %w", err)
		}
		mcpServers, err = normalizeMCPServers(inventory)
		if err != nil {
			return fmt.Errorf("CODEX_MCP_INVENTORY_FAILED: %w", err)
		}
		if runner.mcpIsolation == nil {
			return errors.New("CODEX_MCP_ISOLATION_FAILED: read-only dispatch requires MCP isolation verification")
		}
		if err := runner.mcpIsolation(ctx, runner.command, mcpServers, runner.maxOutputBytes); err != nil {
			return fmt.Errorf("CODEX_MCP_ISOLATION_FAILED: %w", err)
		}
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.prepared == nil {
		runner.prepared = make(map[string]host.DispatchRequest)
	}
	if runner.mcpServers == nil {
		runner.mcpServers = make(map[string][]string)
	}
	if existing, found := runner.prepared[request.InvocationID]; found && !host.EqualDispatchRequest(existing, request) {
		return errors.New("CODEX_INVOCATION_CONFLICT: Invocation ID was reused")
	}
	runner.prepared[request.InvocationID] = host.CloneDispatchRequest(request)
	if isReadOnly(request) {
		runner.mcpServers[request.InvocationID] = append([]string{}, mcpServers...)
	}
	return nil
}

func (runner *Runner) Invoke(ctx context.Context, request host.DispatchRequest) (host.DispatchResult, error) {
	if ctx == nil {
		return host.DispatchResult{}, errors.New("CODEX_CONTEXT_REQUIRED: Invoke context is required")
	}
	if err := host.ValidateDispatchRequest(request); err != nil {
		return host.DispatchResult{}, err
	}
	request = host.CloneDispatchRequest(request)
	runner.mu.Lock()
	if existing, found := runner.results[request.InvocationID]; found {
		runner.mu.Unlock()
		return cloneResult(existing), nil
	}
	if attempt, found := runner.attempts[request.InvocationID]; found {
		done := attempt.done
		runner.mu.Unlock()
		select {
		case <-done:
			return cloneResult(attempt.result), attempt.err
		case <-ctx.Done():
			return host.DispatchResult{}, fmt.Errorf("CODEX_INVOCATION_CANCELLED: %w", ctx.Err())
		}
	}
	prepared, found := runner.prepared[request.InvocationID]
	if !found || !host.EqualDispatchRequest(prepared, request) {
		runner.mu.Unlock()
		return host.DispatchResult{}, errors.New("CODEX_INVOCATION_NOT_PREPARED: Prepare is required before Invoke")
	}
	mcpServers := append([]string{}, runner.mcpServers[request.InvocationID]...)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if runner.attempts == nil {
		runner.attempts = make(map[string]*invocationAttempt)
	}
	if runner.active == nil {
		runner.active = make(map[string]context.CancelFunc)
	}
	runner.attempts[request.InvocationID] = &invocationAttempt{done: make(chan struct{})}
	runner.active[request.InvocationID] = cancel
	runner.mu.Unlock()
	sandbox := sandboxFor(request)
	maximum := runner.maxOutputBytes
	if maximum <= 0 {
		maximum = defaultOutputBytes
	}
	if isReadOnly(request) {
		if runner.mcpIsolation == nil {
			return runner.finishInvocation(request.InvocationID, host.DispatchResult{}, errors.New("CODEX_MCP_ISOLATION_FAILED: pre-exec verification is unavailable"))
		}
		if err := runner.mcpIsolation(ctx, runner.command, mcpServers, maximum); err != nil {
			return runner.finishInvocation(request.InvocationID, host.DispatchResult{}, fmt.Errorf("CODEX_MCP_ISOLATION_FAILED: pre-exec verification: %w", err))
		}
	}
	args := BuildArgs(sandbox, request, mcpServers)
	run := runner.run
	if run == nil {
		run = runCommand
	}
	stdout, stderr, err := run(ctx, runner.command, args, maximum)
	if runner.diagnostics != nil && len(stderr) != 0 {
		_, _ = runner.diagnostics.Write(stderr)
	}
	if err != nil {
		return runner.finishInvocation(request.InvocationID, host.DispatchResult{}, fmt.Errorf("CODEX_PROCESS_FAILED: %w", err))
	}
	maxEvents := runner.maxEvents
	if maxEvents <= 0 {
		maxEvents = defaultEventLimit
	}
	result, err := normalizeJSONL(request, stdout, maxEvents)
	if err != nil {
		return runner.finishInvocation(request.InvocationID, host.DispatchResult{}, err)
	}
	return runner.finishInvocation(request.InvocationID, result, nil)
}

func (runner *Runner) finishInvocation(invocationID string, result host.DispatchResult, err error) (host.DispatchResult, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	attempt := runner.attempts[invocationID]
	if err == nil {
		runner.results[invocationID] = cloneResult(result)
	}
	if attempt != nil {
		attempt.result = cloneResult(result)
		attempt.err = err
		close(attempt.done)
	}
	delete(runner.active, invocationID)
	delete(runner.mcpServers, invocationID)
	return cloneResult(result), err
}

func (runner *Runner) Cancel(ctx context.Context, invocationID string) error {
	if ctx == nil {
		return errors.New("CODEX_CONTEXT_REQUIRED: Cancel context is required")
	}
	runner.mu.Lock()
	cancel := runner.active[invocationID]
	runner.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func sandboxFor(request host.DispatchRequest) string {
	if isReadOnly(request) {
		return "read-only"
	}
	return defaultSandbox
}

func isReadOnly(request host.DispatchRequest) bool {
	return !slices.Contains(request.Effects, "write-project") && !slices.Contains(request.Effects, "git-local")
}

func BuildArgs(sandbox string, request host.DispatchRequest, disabledMCP []string) []string {
	args := []string{"exec", "--json", "--ephemeral"}
	args = append(args, mcpOverrideArgs(disabledMCP)...)
	args = append(args, "--sandbox", sandbox, "--",
		"OAW Runtime invocation "+request.InvocationID+"\n"+
			"Binding: "+request.Binding.Host+"/"+request.Binding.Kind+"/"+request.Binding.Reference+"\n"+
			"Grant: "+request.GrantID+"\n"+
			"Bundle: "+request.BundleDigest+"\n"+
			"Effects: "+strings.Join(request.Effects, ",")+"\n"+
			"Resources: "+strings.Join(request.Resources, ","))
	return args
}

type mcpServerRecord struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func listMCPServers(ctx context.Context, command string, maximum int64) ([]string, error) {
	records, err := readMCPRecords(ctx, command, []string{"mcp", "list", "--json"}, maximum)
	if err != nil {
		return nil, err
	}
	active := make([]string, 0, len(records))
	for _, record := range records {
		if record.Enabled {
			active = append(active, record.Name)
		}
	}
	return normalizeMCPServers(active)
}

func verifyMCPIsolation(ctx context.Context, command string, servers []string, maximum int64) error {
	records, err := readMCPRecords(ctx, command, append(mcpOverrideArgs(servers), "mcp", "list", "--json"), maximum)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Enabled {
			return fmt.Errorf("MCP server %q remains enabled after read-only overrides", record.Name)
		}
	}
	return nil
}

func readMCPRecords(ctx context.Context, command string, args []string, maximum int64) ([]mcpServerRecord, error) {
	stdout, _, err := runCommand(ctx, command, args, maximum)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout))
	var records []mcpServerRecord
	if err := decoder.Decode(&records); err != nil {
		return nil, fmt.Errorf("decode MCP inventory: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("MCP inventory contains multiple JSON values")
		}
		return nil, fmt.Errorf("decode MCP inventory trailer: %w", err)
	}
	return records, nil
}

func mcpOverrideArgs(servers []string) []string {
	args := make([]string, 0, len(servers)*2)
	for _, name := range servers {
		args = append(args, "-c", "mcp_servers."+name+".enabled=false")
	}
	return args
}

func normalizeMCPServers(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !isTOMLBareKey(value) {
			return nil, errors.New("MCP inventory contains an invalid server name")
		}
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func isTOMLBareKey(value string) bool {
	if value == "" || len(value) > 256 || strings.TrimSpace(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '-' || character == '_') {
			return false
		}
	}
	return true
}

func runCommand(ctx context.Context, command string, args []string, maximum int64) ([]byte, []byte, error) {
	if command == "" || filepath.IsAbs(command) && filepath.Clean(command) != command {
		return nil, nil, errors.New("invalid Codex executable path")
	}
	stdout := &limitedBuffer{maximum: maximum}
	stderr := &limitedBuffer{maximum: maximum}
	process := exec.CommandContext(ctx, command, args...)
	process.Stdout = stdout
	process.Stderr = stderr
	err := process.Run()
	if stdout.exceeded || stderr.exceeded {
		return stdout.Bytes(), stderr.Bytes(), errors.New("Codex process output exceeded limit")
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

type limitedBuffer struct {
	data     []byte
	maximum  int64
	exceeded bool
}

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	if int64(len(buffer.data)+len(value)) > buffer.maximum {
		remaining := buffer.maximum - int64(len(buffer.data))
		if remaining > 0 {
			buffer.data = append(buffer.data, value[:remaining]...)
		}
		buffer.exceeded = true
		return len(value), errors.New("output limit exceeded")
	}
	buffer.data = append(buffer.data, value...)
	return len(value), nil
}

func (buffer *limitedBuffer) Bytes() []byte { return append([]byte{}, buffer.data...) }

func cloneResult(value host.DispatchResult) host.DispatchResult {
	value.Evidence = append([]host.DispatchEvidence{}, value.Evidence...)
	return value
}
