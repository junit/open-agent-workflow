package codex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"

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
	sandbox        string
	maxOutputBytes int64
	maxEvents      int
	diagnostics    io.Writer
	run            func(context.Context, string, []string, int64) ([]byte, []byte, error)

	mu       sync.Mutex
	prepared map[string]host.DispatchRequest
	results  map[string]host.DispatchResult
	active   map[string]context.CancelFunc
}

func New(options Options) *Runner {
	command := options.Command
	if command == "" {
		command = defaultCommand
	}
	sandbox := options.Sandbox
	if sandbox == "" {
		sandbox = defaultSandbox
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
		command: command, sandbox: sandbox, maxOutputBytes: maxOutputBytes, maxEvents: maxEvents,
		diagnostics: options.Diagnostics, run: runCommand,
		prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc),
	}
}

type Options struct {
	Command        string
	Sandbox        string
	MaxOutputBytes int64
	MaxEvents      int
	Diagnostics    io.Writer
}

func (runner *Runner) Prepare(request host.DispatchRequest) error {
	if err := host.ValidateDispatchRequest(request); err != nil {
		return err
	}
	if request.Binding.Host != "codex" {
		return errors.New("CODEX_BINDING_UNSUPPORTED: Binding Host is not codex")
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if existing, found := runner.prepared[request.InvocationID]; found && existing != request {
		return errors.New("CODEX_INVOCATION_CONFLICT: Invocation ID was reused")
	}
	runner.prepared[request.InvocationID] = request
	return nil
}

func (runner *Runner) Invoke(request host.DispatchRequest) (host.DispatchResult, error) {
	if err := host.ValidateDispatchRequest(request); err != nil {
		return host.DispatchResult{}, err
	}
	runner.mu.Lock()
	if existing, found := runner.results[request.InvocationID]; found {
		runner.mu.Unlock()
		return cloneResult(existing), nil
	}
	prepared, found := runner.prepared[request.InvocationID]
	if !found || prepared != request {
		runner.mu.Unlock()
		return host.DispatchResult{}, errors.New("CODEX_INVOCATION_NOT_PREPARED: Prepare is required before Invoke")
	}
	ctx, cancel := context.WithCancel(context.Background())
	runner.active[request.InvocationID] = cancel
	runner.mu.Unlock()
	defer func() {
		runner.mu.Lock()
		delete(runner.active, request.InvocationID)
		runner.mu.Unlock()
	}()
	sandbox := runner.sandbox
	if sandbox == "" {
		sandbox = defaultSandbox
	}
	maximum := runner.maxOutputBytes
	if maximum <= 0 {
		maximum = defaultOutputBytes
	}
	args := BuildArgs(sandbox, request)
	run := runner.run
	if run == nil {
		run = runCommand
	}
	stdout, stderr, err := run(ctx, runner.command, args, maximum)
	if runner.diagnostics != nil && len(stderr) != 0 {
		_, _ = runner.diagnostics.Write(stderr)
	}
	if err != nil {
		return host.DispatchResult{}, fmt.Errorf("CODEX_PROCESS_FAILED: %w", err)
	}
	maxEvents := runner.maxEvents
	if maxEvents <= 0 {
		maxEvents = defaultEventLimit
	}
	result, err := normalizeJSONL(request, stdout, maxEvents)
	if err != nil {
		return host.DispatchResult{}, err
	}
	runner.mu.Lock()
	runner.results[request.InvocationID] = cloneResult(result)
	runner.mu.Unlock()
	return result, nil
}

func (runner *Runner) Cancel(invocationID string) error {
	runner.mu.Lock()
	cancel := runner.active[invocationID]
	runner.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func BuildArgs(sandbox string, request host.DispatchRequest) []string {
	return []string{
		"exec", "--json", "--ephemeral", "--sandbox", sandbox, "--",
		"OAW Runtime invocation " + request.InvocationID + "\n" +
			"Binding: " + request.Binding.Host + "/" + request.Binding.Kind + "/" + request.Binding.Reference + "\n" +
			"Grant: " + request.GrantID + "\n" +
			"Bundle: " + request.BundleDigest,
	}
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
