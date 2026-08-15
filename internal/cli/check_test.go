package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheckMatchesBashArgumentErrors(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("PATH", filepath.Join(root, "bin"))
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "unknown option", args: []string{"check", "--bogus"}, message: "unknown option '--bogus'"},
		{name: "missing target", args: []string{"check", "--target"}, message: "--target requires a value"},
		{name: "empty target", args: []string{"check", "--target="}, message: "--target requires a value"},
		{name: "duplicate target", args: []string{"check", "--target", "claude", "--target=codex"}, message: "--target may be specified only once"},
		{name: "missing project", args: []string{"check", "--project"}, message: "--project requires a value"},
		{name: "duplicate project", args: []string{"check", "--project", root, "--project=" + root}, message: "--project may be specified only once"},
		{name: "dry run", args: []string{"check", "--dry-run"}, message: "--dry-run is not valid for check"},
		{name: "force", args: []string{"check", "--force"}, message: "--force is not valid for check"},
		{name: "unexpected", args: []string{"check", "operand"}, message: "unexpected argument 'operand'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tt.args, &stdout, &stderr); code != 64 {
				t.Fatalf("Run(%v) = %d, stdout=%q stderr=%q", tt.args, code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || stderr.String() != "oaw: error: "+tt.message+"\n" {
				t.Fatalf("Run(%v) stdout=%q stderr=%q", tt.args, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunCheckHelpMatchesInstallerUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"check", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run() = %d, stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 || !strings.HasPrefix(stdout.String(), "Usage: ./install.sh <command> [options]\n") || !strings.Contains(stdout.String(), "  check       Report installation and target readiness\n") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
