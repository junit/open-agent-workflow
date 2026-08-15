package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunValidatesManifestFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.manifest")
	if err := os.WriteFile(path, []byte("version=1.2.3\ncommit=abc123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{path}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if stdout.String() != "valid release manifest: 1.2.3\n" || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunReportsMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run([]string{"missing.manifest"}, &stdout, &stderr); status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunRequiresOnePath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := run(nil, &stdout, &stderr); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if stdout.Len() != 0 || stderr.String() != "usage: manifestcheck [--require key]... <path>\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunAcceptsAdditionalRequiredField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.manifest")
	if err := os.WriteFile(path, []byte("version=1.2.3\ncommit=abc123\nowner=platform\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--require", "owner", path}, &stdout, &stderr); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if stdout.String() != "valid release manifest: 1.2.3\n" || stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}
