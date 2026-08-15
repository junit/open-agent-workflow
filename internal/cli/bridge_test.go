package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDefaultOAWDoesNotExposeOrManageBridge(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := RunWithInput([]string{"bridge", "check", "codex"}, strings.NewReader(""), &stdout, &stderr)
	if status != 64 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "use oaw-bridge") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	status = RunWithInput([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if status != 0 || stderr.Len() != 0 || strings.Contains(stdout.String(), "bridge install") || strings.Contains(stdout.String(), "bridge serve") {
		t.Fatalf("default help status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}
