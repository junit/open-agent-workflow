package integrity

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"testing"
)

var prefixedDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func TestSHA256DigestUsesPrefixedLowercaseForm(t *testing.T) {
	const want = "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := SHA256Digest([]byte("abc")); got != want {
		t.Fatalf("SHA256Digest() = %q, want %q", got, want)
	}
}

func TestDigestTreeEnumeratesCanonicalCompleteTree(t *testing.T) {
	root := t.TempDir()
	writeTreeFile(t, root, "z.txt", []byte("z"), 0o600)
	writeTreeFile(t, root, "nested/run.sh", []byte("#!/bin/sh\n"), 0o700)

	first, err := DigestTree(root)
	if err != nil {
		t.Fatalf("DigestTree() error = %v", err)
	}
	if !prefixedDigestPattern.MatchString(first.RootDigest) {
		t.Fatalf("RootDigest = %q", first.RootDigest)
	}
	if len(first.Entries) != 2 {
		t.Fatalf("Entries = %#v", first.Entries)
	}
	if first.Entries[0].Path != "nested/run.sh" || first.Entries[1].Path != "z.txt" {
		t.Fatalf("entry order = %#v", first.Entries)
	}
	if first.Entries[0].Mode != 0o100 || first.Entries[0].Size != int64(len("#!/bin/sh\n")) || !prefixedDigestPattern.MatchString(first.Entries[0].Digest) {
		t.Fatalf("executable entry = %#v", first.Entries[0])
	}
	if first.Entries[1].Mode != 0 {
		t.Fatalf("plain entry mode = %#o", first.Entries[1].Mode)
	}

	first.Entries[0].Path = "changed"
	second, err := DigestTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if second.Entries[0].Path != "nested/run.sh" || second.RootDigest != first.RootDigest {
		t.Fatalf("DigestTree is not deterministic/defensive: %#v", second)
	}
}

func TestDigestTreeChangesForContentModeAndPath(t *testing.T) {
	root := t.TempDir()
	path := writeTreeFile(t, root, "file.txt", []byte("one"), 0o600)
	original := mustDigestTree(t, root)

	if err := os.WriteFile(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	contentChanged := mustDigestTree(t, root)
	if contentChanged.RootDigest == original.RootDigest {
		t.Fatal("content change did not change root digest")
	}

	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	modeChanged := mustDigestTree(t, root)
	if modeChanged.RootDigest == contentChanged.RootDigest {
		t.Fatal("executable-mode change did not change root digest")
	}

	renamed := filepath.Join(root, "renamed.txt")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	pathChanged := mustDigestTree(t, root)
	if pathChanged.RootDigest == modeChanged.RootDigest {
		t.Fatal("path change did not change root digest")
	}
}

func TestDigestTreeRejectsInvalidTrees(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assertBindingTreeInvalid(t, t.TempDir())
	})

	t.Run("root is file", func(t *testing.T) {
		root := t.TempDir()
		path := writeTreeFile(t, root, "root-file", []byte("x"), 0o600)
		assertBindingTreeInvalid(t, path)
	})

	t.Run("symlink in tree", func(t *testing.T) {
		root := t.TempDir()
		target := writeTreeFile(t, root, "target", []byte("x"), 0o600)
		if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		assertBindingTreeInvalid(t, root)
	})

	t.Run("symlink outside tree", func(t *testing.T) {
		root := t.TempDir()
		outside := writeTreeFile(t, t.TempDir(), "outside", []byte("x"), 0o600)
		if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		assertBindingTreeInvalid(t, root)
	})

	t.Run("root symlink", func(t *testing.T) {
		root := t.TempDir()
		writeTreeFile(t, root, "file", []byte("x"), 0o600)
		link := filepath.Join(t.TempDir(), "root-link")
		if err := os.Symlink(root, link); err != nil {
			t.Fatal(err)
		}
		assertBindingTreeInvalid(t, link)
	})

	t.Run("fifo", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("FIFO is not available on Windows")
		}
		root := t.TempDir()
		writeTreeFile(t, root, "file", []byte("x"), 0o600)
		if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertBindingTreeInvalid(t, root)
	})
}

func writeTreeFile(t *testing.T, root, relative string, content []byte, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustDigestTree(t *testing.T, root string) TreeEvidence {
	t.Helper()
	value, err := DigestTree(root)
	if err != nil {
		t.Fatalf("DigestTree() error = %v", err)
	}
	return value
}

func assertBindingTreeInvalid(t *testing.T, root string) {
	t.Helper()
	if _, err := DigestTree(root); err == nil || !regexp.MustCompile(`BINDING_TREE_INVALID`).MatchString(err.Error()) {
		t.Fatalf("DigestTree(%q) error = %v", root, err)
	}
}
