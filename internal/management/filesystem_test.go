package management

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicReplaceCreatesScopedDirectoriesAndPrivateFile(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "one", "two", "artifact")
	action := installAction{
		label: "artifact", data: []byte("content\n"), destination: destination, mode: 0o600,
		allowedRoot: root, relativeSuffix: "one/two/artifact", before: installPathSnapshot{kind: installPathMissing},
	}
	planned := map[string]struct{}{
		filepath.Join(root, "one"):        {},
		filepath.Join(root, "one", "two"): {},
	}
	created := make(map[string]struct{})
	if err := scopedAtomicReplace(action, planned, created); err != nil {
		t.Fatalf("scopedAtomicReplace() error = %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, action.data) {
		t.Fatalf("destination bytes = %q", data)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("destination mode = %o", info.Mode().Perm())
	}
	if len(created) != 2 {
		t.Fatalf("created directories = %#v", created)
	}
	entries, err := os.ReadDir(filepath.Dir(destination))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".oaw-") {
			t.Fatalf("temporary file leaked: %s", entry.Name())
		}
	}
}

func TestAtomicReplaceRejectsSymlinkAndNonDirectoryComponents(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
		action := installAction{
			label: "artifact", data: []byte("content\n"), destination: filepath.Join(root, "linked", "artifact"), mode: 0o644,
			allowedRoot: root, relativeSuffix: "linked/artifact",
		}
		if err := scopedAtomicReplace(action, nil, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("scopedAtomicReplace() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(outside, "artifact")); !os.IsNotExist(err) {
			t.Fatalf("outside artifact exists: %v", err)
		}
	})

	t.Run("non-directory", func(t *testing.T) {
		root := t.TempDir()
		writePrepareFile(t, filepath.Join(root, "file"), []byte("foreign\n"), 0o644)
		action := installAction{
			label: "artifact", data: []byte("content\n"), destination: filepath.Join(root, "file", "artifact"), mode: 0o644,
			allowedRoot: root, relativeSuffix: "file/artifact",
		}
		if err := scopedAtomicReplace(action, nil, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("scopedAtomicReplace() error = %v", err)
		}
	})
}

func TestAtomicReplaceRefusesPlannedDirectoryThatAppeared(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "planned")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	action := installAction{
		label: "artifact", data: []byte("content\n"), destination: filepath.Join(directory, "artifact"), mode: 0o644,
		allowedRoot: root, relativeSuffix: "planned/artifact",
	}
	if err := scopedAtomicReplace(action, map[string]struct{}{directory: {}}, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "appeared before creation") {
		t.Fatalf("scopedAtomicReplace() error = %v", err)
	}
}

func TestAtomicReplaceCreatesMissingAllowedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-root")
	destination := filepath.Join(root, "artifact")
	action := installAction{
		label: "artifact", data: []byte("content\n"), destination: destination, mode: 0o644,
		allowedRoot: root, relativeSuffix: "artifact", before: installPathSnapshot{kind: installPathMissing},
	}
	if err := scopedAtomicReplace(action, nil, make(map[string]struct{})); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(destination); err != nil || string(data) != "content\n" {
		t.Fatalf("destination data=%q error=%v", data, err)
	}
}

func TestAtomicReplaceRejectsUnsafeAllowedRoots(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "root")
		outside := t.TempDir()
		if err := os.Symlink(outside, root); err != nil {
			t.Fatal(err)
		}
		action := installAction{label: "artifact", data: []byte("x"), destination: filepath.Join(root, "artifact"), mode: 0o644, allowedRoot: root, relativeSuffix: "artifact"}
		if err := scopedAtomicReplace(action, nil, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "root is a symlink") {
			t.Fatalf("scopedAtomicReplace() error = %v", err)
		}
	})

	t.Run("file", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "root")
		writePrepareFile(t, root, []byte("file\n"), 0o644)
		action := installAction{label: "artifact", data: []byte("x"), destination: filepath.Join(root, "artifact"), mode: 0o644, allowedRoot: root, relativeSuffix: "artifact"}
		if err := scopedAtomicReplace(action, nil, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "root is not a directory") {
			t.Fatalf("scopedAtomicReplace() error = %v", err)
		}
	})
}

func TestInstallDirectorySetsFailClosed(t *testing.T) {
	root := t.TempDir()
	if _, err := installPathSet([]string{"relative"}); err == nil {
		t.Fatal("relative planned directory was accepted")
	}
	if _, err := installPathSet([]string{root, root}); err == nil {
		t.Fatal("duplicate planned directory was accepted")
	}
	if err := validateCreatedInstallDirectories(map[string]struct{}{root: {}}, nil); err == nil {
		t.Fatal("missing created directory was accepted")
	}
	if err := validateCreatedInstallDirectories(nil, map[string]struct{}{root: {}}); err == nil {
		t.Fatal("unplanned created directory was accepted")
	}
}

func TestScopedFilesystemHelpersRejectInvalidCoordinatesAndMissingTempDirectory(t *testing.T) {
	rootPath := t.TempDir()
	action := installAction{
		label: "artifact", data: []byte("x"), destination: filepath.Join(rootPath, "other"), mode: 0o644,
		allowedRoot: rootPath, relativeSuffix: "artifact",
	}
	if err := scopedAtomicReplace(action, nil, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "changed after preparation") {
		t.Fatalf("mismatched scoped action error = %v", err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, _, err := createScopedTemporary(root, "missing"); err == nil {
		t.Fatal("temporary file was created in a missing directory")
	}
	syncScopedDirectory(root, "missing")
}

func TestScopedFilesystemCreatesUnplannedCompatibilityDirectory(t *testing.T) {
	root := t.TempDir()
	action := installAction{
		label: "artifact", data: []byte("x"), destination: filepath.Join(root, "compat", "artifact"), mode: 0o644,
		allowedRoot: root, relativeSuffix: "compat/artifact",
	}
	created := make(map[string]struct{})
	if err := scopedAtomicReplace(action, nil, created); err != nil {
		t.Fatal(err)
	}
	if len(created) != 0 {
		t.Fatalf("unplanned compatibility directory was claimed: %#v", created)
	}
}

func TestScopedFilesystemDetectsChangedDirectoryHandleAndFinalSymlink(t *testing.T) {
	rootPath := t.TempDir()
	first := filepath.Join(rootPath, "first")
	second := filepath.Join(rootPath, "second")
	if err := os.Mkdir(first, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(second, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	firstRoot, err := root.OpenRoot("first")
	if err != nil {
		t.Fatal(err)
	}
	defer firstRoot.Close()
	action := installAction{destination: filepath.Join(second, "artifact"), allowedRoot: rootPath, relativeSuffix: "second/artifact"}
	if err := verifyScopedActionDirectory(firstRoot, action); err == nil || !strings.Contains(err.Error(), "changed during creation") {
		t.Fatalf("directory handle mismatch error = %v", err)
	}

	outside := filepath.Join(rootPath, "outside")
	writePrepareFile(t, outside, []byte("outside\n"), 0o644)
	link := filepath.Join(rootPath, "artifact-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	linkAction := installAction{
		label: "artifact", data: []byte("replacement\n"), destination: link, mode: 0o644,
		allowedRoot: rootPath, relativeSuffix: "artifact-link",
	}
	if err := scopedAtomicReplace(linkAction, nil, make(map[string]struct{})); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("final symlink error = %v", err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "outside\n" {
		t.Fatalf("outside data=%q error=%v", data, err)
	}
}

func TestScopedFilesystemHelpersRejectMissingDirectoryAndInvalidRoot(t *testing.T) {
	if root, err := openInstallRoot(""); err == nil {
		_ = root.Close()
		t.Fatal("empty allowed root was opened")
	}
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	action := installAction{
		destination: filepath.Join(rootPath, "missing", "artifact"),
		allowedRoot: rootPath, relativeSuffix: "missing/artifact",
	}
	if directoryRoot, err := openScopedActionDirectory(root, action); err == nil {
		_ = directoryRoot.Close()
		t.Fatal("missing action directory was opened")
	}
	tooLong := strings.Repeat("x", 1024)
	action.destination = filepath.Join(rootPath, tooLong)
	action.relativeSuffix = tooLong
	if err := revalidateScopedAction(root, action); err == nil || !strings.Contains(err.Error(), "could not be inspected") {
		t.Fatalf("overlong destination error = %v", err)
	}
}

func TestScopedFilesystemAndBoundedReadsDetectCoordinateRaces(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifact")
	writePrepareFile(t, path, []byte("small\n"), 0o644)
	staleInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := readBoundedInstallFile(path, staleInfo); err == nil {
		t.Fatal("bounded read accepted a removed file")
	}
	writePrepareFile(t, path, bytes.Repeat([]byte{'x'}, maximumInstallArtifactBytes+1), 0o644)
	if _, err := readBoundedInstallFile(path, staleInfo); err == nil {
		t.Fatal("bounded read accepted a file that grew after inspection")
	}
	if missing, err := installPathIsMissing(filepath.Join(path, "child")); err == nil || missing {
		t.Fatalf("non-directory coordinate missing=%t error=%v", missing, err)
	}
	if scopedRoot, err := openInstallRoot(filepath.Join(path, "child")); err == nil {
		_ = scopedRoot.Close()
		t.Fatal("root below a file was opened")
	}
}
