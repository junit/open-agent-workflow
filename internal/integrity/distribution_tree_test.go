package integrity

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDigestDistributionTreeRecordsContainedSymlinkWithoutFollowingIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	evidence, err := DigestDistributionTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.Entries) != 2 || evidence.Entries[0].Path != "link.txt" || evidence.Entries[0].Kind != "symlink" ||
		evidence.Entries[0].Size != int64(len("target.txt")) || evidence.Entries[0].Digest != SHA256Digest([]byte("target.txt")) {
		t.Fatalf("contained symlink evidence = %#v", evidence.Entries)
	}
	if err := os.WriteFile(target, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := DigestDistributionTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Entries[0] != evidence.Entries[0] {
		t.Fatalf("symlink entry followed its target: before=%#v after=%#v", evidence.Entries[0], changed.Entries[0])
	}
}

func TestDigestDistributionTreeRejectsEscapingSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside.txt", filepath.Join(root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := DigestDistributionTree(root); err == nil || !strings.Contains(err.Error(), "DISTRIBUTION_TREE_INVALID") {
		t.Fatalf("escaping symlink error=%v", err)
	}
}

func TestVerifyRootedDistributionDirectoryRejectsSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	rootPath := t.TempDir()
	childPath := filepath.Join(rootPath, "child")
	if err := os.Mkdir(childPath, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	childInfo, err := root.Lstat("child")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(childPath, filepath.Join(rootPath, "original-child")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("original-child", childPath); err != nil {
		t.Fatal(err)
	}
	if err := verifyRootedDistributionDirectory(root, "child", childInfo); err == nil || !strings.Contains(err.Error(), "DISTRIBUTION_TREE_INVALID") {
		t.Fatalf("swapped directory error=%v", err)
	}
}

func TestVerifyRootedDistributionDirectorySnapshotRejectsAddedEntry(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootPath, "first.txt"), []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	snapshot, err := rootedDirectorySnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "second.txt"), []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyRootedDistributionDirectorySnapshot(root, snapshot); err == nil || !strings.Contains(err.Error(), "DISTRIBUTION_TREE_INVALID") {
		t.Fatalf("verifyRootedDistributionDirectorySnapshot() error = %v", err)
	}
}
