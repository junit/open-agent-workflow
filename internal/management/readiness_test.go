package management

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableOnPathTreatsEmptyEntryAsWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	path := filepath.Join(root, "oaw-tool")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !executableOnPath(string(os.PathListSeparator), "oaw-tool") {
		t.Fatal("empty PATH entry did not resolve against the working directory")
	}
}
