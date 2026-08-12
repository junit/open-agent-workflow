package integrity

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

func TestDigestBindingRootDispatchesDirectoriesAndRegularFiles(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		installation := t.TempDir()
		root := filepath.Join(installation, "skills/review")
		writeTreeFile(t, root, "SKILL.md", []byte("skill"), 0o600)
		writeTreeFile(t, root, ".hidden", []byte("complete"), 0o600)
		want, err := DigestTree(root)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DigestBindingRoot(installation, "skills/review", "skills/review")
		if err != nil {
			t.Fatalf("DigestBindingRoot() error = %v", err)
		}
		if got.RootDigest != want.RootDigest || len(got.Entries) != 2 {
			t.Fatalf("directory evidence = %#v, want %#v", got, want)
		}
	})

	t.Run("regular file", func(t *testing.T) {
		installation := t.TempDir()
		writeTreeFile(t, installation, "reviewer.md", []byte("agent"), 0o600)
		got, err := DigestBindingRoot(installation, "reviewer.md", "agents/reviewer.md")
		if err != nil {
			t.Fatalf("DigestBindingRoot() error = %v", err)
		}
		want := bindingFileRootDigest(t, "agents/reviewer.md", []byte("agent"), false)
		if got.RootDigest != want || len(got.Entries) != 1 || got.Entries[0].Path != "agents/reviewer.md" || got.Entries[0].Digest != SHA256Digest([]byte("agent")) {
			t.Fatalf("file evidence = %#v, want digest %q", got, want)
		}
	})

	t.Run("executable regular file", func(t *testing.T) {
		installation := t.TempDir()
		writeTreeFile(t, installation, "reviewer.md", []byte("agent"), 0o700)
		got, err := DigestBindingRoot(installation, "reviewer.md", "agents/reviewer.md")
		if err != nil {
			t.Fatalf("DigestBindingRoot() error = %v", err)
		}
		want := bindingFileRootDigest(t, "agents/reviewer.md", []byte("agent"), true)
		if got.RootDigest != want || len(got.Entries) != 1 || got.Entries[0].Mode != 0o100 {
			t.Fatalf("executable file evidence = %#v, want digest %q", got, want)
		}
	})
}

func TestDigestBindingRootRejectsSymlinksAndUnsupportedRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink and FIFO creation require platform-specific privileges")
	}
	t.Run("symlink", func(t *testing.T) {
		installation := t.TempDir()
		target := writeTreeFile(t, installation, "target.md", []byte("agent"), 0o600)
		link := filepath.Join(installation, "reviewer.md")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		assertBindingRootInvalid(t, installation, "reviewer.md")
	})

	t.Run("fifo", func(t *testing.T) {
		installation := t.TempDir()
		path := filepath.Join(installation, "reviewer")
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
		assertBindingRootInvalid(t, installation, "reviewer")
	})

	t.Run("non-canonical logical root", func(t *testing.T) {
		installation := t.TempDir()
		writeTreeFile(t, installation, "reviewer.md", []byte("agent"), 0o600)
		for _, logicalRoot := range []string{".", "..", "../reviewer.md", "agents//reviewer.md", " agents/reviewer.md", "C:/agents/reviewer.md"} {
			if _, err := DigestBindingRoot(installation, "reviewer.md", logicalRoot); err == nil || !strings.Contains(err.Error(), "BINDING_ROOT_INVALID") {
				t.Errorf("DigestBindingRoot(%q) error = %v", logicalRoot, err)
			}
		}
	})
}

func TestVerifyPhysicalBindingRootRejectsAncestorSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	installation := t.TempDir()
	writeTreeFile(t, installation, "agents/reviewer.md", []byte("agent"), 0o600)
	root, err := os.OpenRoot(installation)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	snapshot, err := physicalBindingRoot(root, "agents/reviewer.md")
	if err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(installation, "agents")
	moved := filepath.Join(installation, "original-agents")
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("original-agents", original); err != nil {
		t.Fatal(err)
	}
	if err := verifyPhysicalBindingRoot(root, snapshot); err == nil || !strings.Contains(err.Error(), "BINDING_ROOT_INVALID") {
		t.Fatalf("verifyPhysicalBindingRoot() error = %v", err)
	}
}

func TestVerifyRootedBindingDirectorySnapshotRejectsAddedEntry(t *testing.T) {
	rootPath := t.TempDir()
	writeTreeFile(t, rootPath, "first.txt", []byte("first"), 0o600)
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	snapshot, err := rootedDirectorySnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	writeTreeFile(t, rootPath, "second.txt", []byte("second"), 0o600)
	if err := verifyRootedBindingDirectorySnapshot(root, snapshot); err == nil || !strings.Contains(err.Error(), "BINDING_ROOT_INVALID") {
		t.Fatalf("verifyRootedBindingDirectorySnapshot() error = %v", err)
	}
}

func bindingFileRootDigest(t *testing.T, path string, content []byte, executable bool) string {
	t.Helper()
	payload := struct {
		SchemaVersion string `json:"schema_version"`
		Path          string `json:"path"`
		Executable    bool   `json:"executable"`
		Size          int64  `json:"size"`
		Digest        string `json:"digest"`
	}{"oaw.binding-file/v1", path, executable, int64(len(content)), SHA256Digest(content)}
	encoded, err := canonicaljson.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return SHA256Digest(encoded)
}

func assertBindingRootInvalid(t *testing.T, installationRoot, installRoot string) {
	t.Helper()
	if _, err := DigestBindingRoot(installationRoot, installRoot, "agents/reviewer.md"); err == nil || !strings.Contains(err.Error(), "BINDING_ROOT_INVALID") {
		t.Fatalf("DigestBindingRoot(%q) error = %v", installRoot, err)
	}
}
