package codex

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestRunnerUsesExactBindingAndDeduplicatesInvocation(t *testing.T) {
	calls := 0
	runner := &Runner{
		command:        "codex",
		maxOutputBytes: 1 << 20,
		maxEvents:      16,
		prepared:       make(map[string]host.DispatchRequest),
		results:        make(map[string]host.DispatchResult),
		active:         make(map[string]context.CancelFunc),
		run: func(_ context.Context, _ string, args []string, _ int64) ([]byte, []byte, error) {
			calls++
			if len(args) < 7 || args[0] != "exec" || args[1] != "--json" || args[2] != "--ephemeral" || args[3] != "--sandbox" || args[4] != "workspace-write" || !strings.Contains(args[len(args)-1], "to-spec") {
				t.Fatalf("Codex args = %#v", args)
			}
			return []byte(`{"type":"turn.completed","id":"turn-1"}` + "\n"), nil, nil
		},
	}
	request := testDispatchRequest()
	if err := runner.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	first, err := runner.Invoke(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.Invoke(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || first.ExecutionID != second.ExecutionID || first.Outcome != host.DispatchSucceeded {
		t.Fatalf("calls=%d first=%#v second=%#v", calls, first, second)
	}
}

func TestRunnerUsesReadOnlySandboxForReadOnlyGrant(t *testing.T) {
	inventoryCalls := 0
	isolationCalls := 0
	runner := &Runner{
		command:        "codex",
		maxOutputBytes: 1 << 20,
		maxEvents:      16,
		prepared:       make(map[string]host.DispatchRequest),
		results:        make(map[string]host.DispatchResult),
		active:         make(map[string]context.CancelFunc),
		mcpInventory: func(_ context.Context, command string, _ int64) ([]string, error) {
			inventoryCalls++
			if command != "codex" {
				t.Fatalf("inventory command = %q", command)
			}
			return []string{"filesystem", "serena"}, nil
		},
		mcpIsolation: func(_ context.Context, command string, servers []string, _ int64) error {
			isolationCalls++
			if command != "codex" || strings.Join(servers, ",") != "filesystem,serena" {
				t.Fatalf("isolation probe = command %q servers %#v", command, servers)
			}
			return nil
		},
		run: func(_ context.Context, _ string, args []string, _ int64) ([]byte, []byte, error) {
			wantPrefix := []string{"exec", "--json", "--ephemeral", "-c", "mcp_servers.filesystem.enabled=false", "-c", "mcp_servers.serena.enabled=false", "--sandbox", "read-only", "--"}
			if len(args) < len(wantPrefix)+1 {
				t.Fatalf("Codex args = %#v", args)
			}
			for index, want := range wantPrefix {
				if args[index] != want {
					t.Fatalf("Codex args[%d] = %q, want %q; all args = %#v", index, args[index], want, args)
				}
			}
			return []byte(`{"type":"turn.completed","id":"turn-1"}` + "\n"), nil, nil
		},
	}
	request := host.DispatchRequest{
		GrantID: "grant-read", InvocationID: "invocation-read", ExecutorID: "executor-read",
		BundleDigest: strings.Repeat("a", 64),
		Binding:      catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "review"},
		Effects:      []string{"read-project"}, Resources: []string{"project"},
	}
	if err := runner.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Invoke(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if inventoryCalls != 1 {
		t.Fatalf("inventory calls = %d, want 1", inventoryCalls)
	}
	if isolationCalls != 2 {
		t.Fatalf("isolation calls = %d, want Prepare and pre-exec verification", isolationCalls)
	}
}

func TestRunnerFailsClosedWhenReadOnlyMCPInventoryIsUnavailable(t *testing.T) {
	runner := &Runner{
		command: "codex", maxOutputBytes: 1 << 20,
		prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc),
		mcpInventory: func(context.Context, string, int64) ([]string, error) {
			return nil, errors.New("inventory failed")
		},
		run: func(context.Context, string, []string, int64) ([]byte, []byte, error) {
			t.Fatal("Codex execution started without a trustworthy MCP inventory")
			return nil, nil, nil
		},
	}
	request := host.DispatchRequest{
		GrantID: "grant-read", InvocationID: "invocation-read", ExecutorID: "executor-read",
		BundleDigest: strings.Repeat("a", 64),
		Binding:      catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "review"},
		Effects:      []string{"read-project"}, Resources: []string{"project"},
	}
	if err := runner.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "CODEX_MCP_INVENTORY_FAILED") {
		t.Fatalf("Prepare error = %v, want CODEX_MCP_INVENTORY_FAILED", err)
	}
}

func TestRunnerFailsClosedWhenMCPServerNameCannotBeOverridden(t *testing.T) {
	runner := &Runner{
		command: "codex", maxOutputBytes: 1 << 20,
		prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc),
		mcpInventory: func(context.Context, string, int64) ([]string, error) {
			return []string{"unsafe.server"}, nil
		},
	}
	request := host.DispatchRequest{
		GrantID: "grant-read", InvocationID: "invocation-read", ExecutorID: "executor-read",
		BundleDigest: strings.Repeat("a", 64),
		Binding:      catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "review"},
		Effects:      []string{"read-project"}, Resources: []string{"project"},
	}
	if err := runner.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "CODEX_MCP_INVENTORY_FAILED") {
		t.Fatalf("Prepare error = %v, want fail-closed invalid MCP name", err)
	}
}

func TestRunnerFailsClosedWhenMCPIsolationVerificationIsUnavailable(t *testing.T) {
	runner := &Runner{
		command: "codex", maxOutputBytes: 1 << 20,
		prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc),
		mcpInventory: func(context.Context, string, int64) ([]string, error) {
			return []string{"serena"}, nil
		},
	}
	request := host.DispatchRequest{
		GrantID: "grant-read", InvocationID: "invocation-read", ExecutorID: "executor-read",
		BundleDigest: strings.Repeat("a", 64),
		Binding:      catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "review"},
		Effects:      []string{"read-project"}, Resources: []string{"project"},
	}
	if err := runner.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "CODEX_MCP_ISOLATION_FAILED") {
		t.Fatalf("Prepare error = %v, want CODEX_MCP_ISOLATION_FAILED", err)
	}
}

func TestRunnerFailsClosedWhenCodexLeavesMCPEnabled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture executable")
	}
	path := filepath.Join(t.TempDir(), "codex-fixture")
	script := "#!/bin/sh\nprintf '%s\\n' '[{\"name\":\"serena\",\"enabled\":true}]'\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := New(Options{Command: path, MaxOutputBytes: 1024})
	request := host.DispatchRequest{
		GrantID: "grant-read", InvocationID: "invocation-read", ExecutorID: "executor-read",
		BundleDigest: strings.Repeat("a", 64),
		Binding:      catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "review"},
		Effects:      []string{"read-project"}, Resources: []string{"project"},
	}
	if err := runner.Prepare(context.Background(), request); err == nil || !strings.Contains(err.Error(), "CODEX_MCP_ISOLATION_FAILED") {
		t.Fatalf("Prepare error = %v, want residual MCP failure", err)
	}
}

func TestRunnerRejectsInvocationWithoutPreparation(t *testing.T) {
	runner := &Runner{prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc)}
	request := testDispatchRequest()
	if _, err := runner.Invoke(context.Background(), request); err == nil {
		t.Fatal("Invoke accepted an unprepared request")
	}
}

func TestRunnerDeduplicatesConcurrentInvocation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	runner := &Runner{
		command: "codex", maxOutputBytes: 1024, maxEvents: 4,
		prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc),
		run: func(_ context.Context, _ string, _ []string, _ int64) ([]byte, []byte, error) {
			calls.Add(1)
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			return []byte(`{"type":"turn.completed","id":"turn-1"}` + "\n"), nil, nil
		},
	}
	request := testDispatchRequest()
	if err := runner.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	results := make(chan host.DispatchResult, 2)
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			result, err := runner.Invoke(context.Background(), request)
			results <- result
			errors <- err
		}()
	}
	<-started
	close(release)
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
		if result := <-results; result.Outcome != host.DispatchSucceeded {
			t.Fatalf("result = %#v", result)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("Codex process calls = %d, want 1", calls.Load())
	}
}

func TestRunnerUsesFixtureExecutableAndSeparatesDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture executable")
	}
	path := filepath.Join(t.TempDir(), "codex-fixture")
	script := "#!/bin/sh\nprintf '%s\\n' '{\"type\":\"turn.completed\",\"id\":\"fixture-turn\"}'\nprintf '%s\\n' 'fixture diagnostic secret' >&2\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := New(Options{Command: path, MaxOutputBytes: 1024, MaxEvents: 4})
	request := testDispatchRequest()
	if err := runner.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Invoke(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != host.DispatchSucceeded || strings.Contains(result.ExecutionID, "secret") {
		t.Fatalf("fixture result = %#v", result)
	}
	var diagnostics bytes.Buffer
	runner = New(Options{Command: path, MaxOutputBytes: 1024, MaxEvents: 4, Diagnostics: &diagnostics})
	if err := runner.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Invoke(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.String(), "fixture diagnostic secret") {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
}

func TestRunnerCancelStopsActiveInvocation(t *testing.T) {
	started := make(chan struct{})
	runner := &Runner{
		command: "codex", maxOutputBytes: 1024, maxEvents: 4,
		prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc),
		run: func(ctx context.Context, _ string, _ []string, _ int64) ([]byte, []byte, error) {
			close(started)
			<-ctx.Done()
			return nil, nil, ctx.Err()
		},
	}
	request := testDispatchRequest()
	if err := runner.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := runner.Invoke(context.Background(), request); done <- err }()
	<-started
	if err := runner.Cancel(context.Background(), request.InvocationID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("cancelled invocation unexpectedly succeeded")
	}
}

func testDispatchRequest() host.DispatchRequest {
	return host.DispatchRequest{
		GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor",
		BundleDigest: strings.Repeat("a", 64),
		Binding:      catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "to-spec"},
		Effects:      []string{"write-project"}, Resources: []string{"project-worktree"},
	}
}
