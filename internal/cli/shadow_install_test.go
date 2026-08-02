package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseShadowManagementAcceptsBashOptions(t *testing.T) {
	project := t.TempDir()
	for _, operation := range []string{"install", "update", "uninstall"} {
		t.Run(operation, func(t *testing.T) {
			tests := []struct {
				name string
				args []string
				want shadowManagementCommand
			}{
				{name: "empty", args: []string{operation}, want: shadowManagementCommand{operation: operation}},
				{name: "separate values", args: []string{operation, "--target", "codex,claude", "--project", project, "--dry-run", "--force"}, want: shadowManagementCommand{operation: operation, targets: "codex,claude", project: project, dryRun: true, force: true}},
				{name: "equals values", args: []string{operation, "--target=claude", "--project=" + project}, want: shadowManagementCommand{operation: operation, targets: "claude", project: project}},
				{name: "short help", args: []string{operation, "-h"}, want: shadowManagementCommand{operation: operation, help: true}},
				{name: "long help", args: []string{operation, "--help"}, want: shadowManagementCommand{operation: operation, help: true}},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := parseShadowManagement(tt.args)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, tt.want) {
						t.Fatalf("parseShadowManagement() = %#v, want %#v", got, tt.want)
					}
				})
			}
		})
	}
}

func TestRunShadowManagementMatchesBashArgumentErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "missing command", message: "unknown command ''"},
		{name: "wrong command", args: []string{"check"}, message: "unknown command 'check'"},
		{name: "missing target", args: []string{"install", "--target"}, message: "--target requires a value"},
		{name: "empty target", args: []string{"install", "--target="}, message: "--target requires a value"},
		{name: "dash target", args: []string{"install", "--target", "--force"}, message: "--target requires a value"},
		{name: "duplicate target", args: []string{"install", "--target", "claude", "--target=codex"}, message: "--target may be specified only once"},
		{name: "missing project", args: []string{"install", "--project"}, message: "--project requires a value"},
		{name: "empty project", args: []string{"install", "--project="}, message: "--project requires a value"},
		{name: "duplicate project", args: []string{"install", "--project", "/one", "--project=/two"}, message: "--project may be specified only once"},
		{name: "duplicate dry run", args: []string{"install", "--dry-run", "--dry-run"}, message: "--dry-run may be specified only once"},
		{name: "duplicate force", args: []string{"install", "--force", "--force"}, message: "--force may be specified only once"},
		{name: "duplicate help", args: []string{"install", "--help", "-h"}, message: "--help may be specified only once"},
		{name: "unknown option", args: []string{"install", "--bogus"}, message: "unknown option '--bogus'"},
		{name: "operand", args: []string{"install", "operand"}, message: "unexpected argument 'operand'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if status := RunShadowManagement(tt.args, &stdout, &stderr); status != 64 {
				t.Fatalf("RunShadowManagement() = %d, stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || stderr.String() != "oaw: error: "+tt.message+"\n" {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunShadowManagementHelpAndLifecycle(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, operation := range []string{"install", "update", "uninstall"} {
		stdout.Reset()
		stderr.Reset()
		if status := RunShadowManagement([]string{operation, "--help"}, &stdout, &stderr); status != 0 {
			t.Fatalf("%s help status=%d stderr=%q", operation, status, stderr.String())
		}
		if stderr.Len() != 0 || stdout.String() != installerUsage() {
			t.Fatalf("%s help stdout=%q stderr=%q", operation, stdout.String(), stderr.String())
		}
	}

	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	for _, directory := range []string{home, config, state} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)
	stdout.Reset()
	stderr.Reset()
	if status := RunShadowManagement([]string{"install", "--target", "claude", "--dry-run"}, &stdout, &stderr); status != 0 {
		t.Fatalf("dry-run status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 || !strings.Contains(stdout.String(), "oaw: would-create: "+filepath.Join(home, ".claude", "CLAUDE.md")+"\n") {
		t.Fatalf("dry-run stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, directory := range []string{home, config, state} {
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 0 {
			t.Fatalf("dry-run changed %s: entries=%v error=%v", directory, entries, err)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if status := RunShadowManagement([]string{"update", "--target", "claude"}, &stdout, &stderr); status != 66 {
		t.Fatalf("missing-state update status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || stderr.String() != "oaw: error: no installation state; run install first\n" {
		t.Fatalf("missing-state update stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if status := RunShadowManagement([]string{"install", "--target", "claude"}, &stdout, &stderr); status != 0 {
		t.Fatalf("install status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if status := RunShadowManagement([]string{"update", "--target", "claude"}, &stdout, &stderr); status != 0 {
		t.Fatalf("update status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "oaw: unchanged: claude\n") || stderr.Len() != 0 {
		t.Fatalf("update stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if status := RunShadowManagement([]string{"uninstall", "--target", "claude"}, &stdout, &stderr); status != 0 {
		t.Fatalf("uninstall status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "oaw: remove: ") || stderr.Len() != 0 {
		t.Fatalf("uninstall stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestPublicRunDoesNotRouteManagement(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	for _, directory := range []string{home, config, state} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)
	for _, operation := range []string{"install", "update", "uninstall"} {
		var stdout, stderr bytes.Buffer
		if status := Run([]string{operation}, &stdout, &stderr); status != 64 {
			t.Fatalf("Run(%s)=%d stdout=%q stderr=%q", operation, status, stdout.String(), stderr.String())
		}
		if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "oaw: INVALID_ARGUMENT: expected catalog command\n") {
			t.Fatalf("%s stdout=%q stderr=%q", operation, stdout.String(), stderr.String())
		}
	}
	for _, directory := range []string{home, config, state} {
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 0 {
			t.Fatalf("public Run changed %s: entries=%v error=%v", directory, entries, err)
		}
	}
}
