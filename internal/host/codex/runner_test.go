package codex

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	request := host.DispatchRequest{GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", BundleDigest: strings.Repeat("a", 64), Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "to-spec"}}
	if err := runner.Prepare(request); err != nil {
		t.Fatal(err)
	}
	first, err := runner.Invoke(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.Invoke(request)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || first.ExecutionID != second.ExecutionID || first.Outcome != host.DispatchSucceeded {
		t.Fatalf("calls=%d first=%#v second=%#v", calls, first, second)
	}
}

func TestRunnerRejectsInvocationWithoutPreparation(t *testing.T) {
	runner := &Runner{prepared: make(map[string]host.DispatchRequest), results: make(map[string]host.DispatchResult), active: make(map[string]context.CancelFunc)}
	request := host.DispatchRequest{GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", BundleDigest: strings.Repeat("a", 64), Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "to-spec"}}
	if _, err := runner.Invoke(request); err == nil {
		t.Fatal("Invoke accepted an unprepared request")
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
	request := host.DispatchRequest{GrantID: "grant", InvocationID: "invocation", ExecutorID: "executor", BundleDigest: strings.Repeat("a", 64), Binding: catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "to-spec"}}
	if err := runner.Prepare(request); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Invoke(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != host.DispatchSucceeded || strings.Contains(result.ExecutionID, "secret") {
		t.Fatalf("fixture result = %#v", result)
	}
	var diagnostics bytes.Buffer
	runner = New(Options{Command: path, MaxOutputBytes: 1024, MaxEvents: 4, Diagnostics: &diagnostics})
	if err := runner.Prepare(request); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Invoke(request); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.String(), "fixture diagnostic secret") {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
}
