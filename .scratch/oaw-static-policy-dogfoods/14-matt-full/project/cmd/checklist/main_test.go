package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSummarizesMarkdownFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.md")
	if err := os.WriteFile(path, []byte("- [x] Policy\n- [ ] Tests\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{path}, &stdout, &stderr); status != 0 {
		t.Fatalf("run status = %d, stderr = %q", status, stderr.String())
	}
	if stdout.String() != "1/2 complete\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "1/2 complete\n")
	}
}

func TestRunReportsUsageAndReadErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run(nil, &stdout, &stderr); status != 2 {
		t.Fatalf("run(nil) status = %d, want 2", status)
	}
	if !strings.Contains(stderr.String(), "usage: checklist <markdown-path>") {
		t.Fatalf("usage stderr = %q", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if status := run([]string{"missing.md"}, &stdout, &stderr); status != 1 {
		t.Fatalf("run(missing) status = %d, want 1", status)
	}
	if !strings.Contains(stderr.String(), "read checklist") {
		t.Fatalf("read error stderr = %q", stderr.String())
	}
}
