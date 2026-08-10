package main

import (
	"archive/tar"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
)

func TestAuditCLIUsesExplicitRoots(t *testing.T) {
	if err := run([]string{"--write", "--output", t.TempDir() + "/manifest.json"}); err == nil || !strings.Contains(err.Error(), "explicit") {
		t.Fatalf("write without roots error = %v", err)
	}
	if err := run([]string{"--check", "--manifest", t.TempDir() + "/manifest.json", "--matt-root", t.TempDir()}); err == nil || !strings.Contains(err.Error(), "all three") {
		t.Fatalf("check with partial roots error = %v", err)
	}
}

func TestAuditCLIRoundTripsExportedRoots(t *testing.T) {
	matt, superpowers, ecc := exportedProviderRoots(t)
	manifest := filepath.Join(t.TempDir(), "provider-sources-v4.json")
	providerArgs := []string{"--exported", "--matt-root", matt, "--superpowers-root", superpowers, "--ecc-root", ecc}
	if err := run(append([]string{"--write", "--output", manifest}, providerArgs...)); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--validate", "--manifest", manifest}); err != nil {
		t.Fatal(err)
	}
	if err := run(append([]string{"--check", "--manifest", manifest}, providerArgs...)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(append([]string{"--check", "--manifest", manifest}, providerArgs...)); err == nil || !strings.Contains(err.Error(), "PROVIDER_AUDIT_DRIFT") {
		t.Fatalf("drift check error = %v", err)
	}
}

func TestAuditCLIRejectsConflictingModesAndPaths(t *testing.T) {
	tests := [][]string{
		{},
		{"--validate", "--write", "--manifest", "manifest", "--output", "output"},
		{"--validate", "--manifest", "manifest", "operand"},
		{"--validate", "--manifest", "manifest", "--output", "output"},
		{"--write", "--output", "output", "--manifest", "manifest", "--matt-root", "a", "--superpowers-root", "b", "--ecc-root", "c"},
		{"--check", "--manifest", "manifest", "--output", "output", "--matt-root", "a", "--superpowers-root", "b", "--ecc-root", "c"},
	}
	for _, args := range tests {
		if err := run(args); err == nil || !strings.Contains(err.Error(), "CLI_ARGUMENT_INVALID") {
			t.Fatalf("run(%q) error = %v", args, err)
		}
	}
}

func TestAuditCLIRejectsMissingAndMalformedManifest(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	if err := run([]string{"--validate", "--manifest", missing}); err == nil {
		t.Fatal("validate accepted a missing manifest")
	}
	malformed := filepath.Join(t.TempDir(), "malformed.json")
	if err := os.WriteFile(malformed, []byte("{not-json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--validate", "--manifest", malformed}); err == nil || !strings.Contains(err.Error(), "PROVIDER_AUDIT_INVALID") {
		t.Fatalf("malformed manifest error = %v", err)
	}
}

func TestGitRevisionAndArchiveExport(t *testing.T) {
	repository := t.TempDir()
	runGit(t, repository, "init", "-q")
	if err := os.WriteFile(filepath.Join(repository, "CLAUDE.md"), []byte("instructions\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("CLAUDE.md", filepath.Join(repository, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "CLAUDE.md", "AGENTS.md", "run.sh")
	runGit(t, repository, "-c", "user.name=OAW Test", "-c", "user.email=oaw@example.invalid", "commit", "-q", "-m", "fixture")
	revision := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	if err := verifyGitRevision(repository, revision); err != nil {
		t.Fatal(err)
	}
	if err := verifyGitRevision(repository, strings.Repeat("0", 40)); err == nil {
		t.Fatal("verifyGitRevision accepted the wrong revision")
	}
	destination := filepath.Join(t.TempDir(), "export")
	if err := exportGitTree(repository, revision, destination); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(destination, "AGENTS.md")); err != nil || target != "CLAUDE.md" {
		t.Fatalf("exported symlink target = %q, %v", target, err)
	}
	info, err := os.Stat(filepath.Join(destination, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if executable := info.Mode().Perm() & 0o111; executable != 0o111 {
		t.Fatalf("exported executable bits = %#o, want %#o", executable, os.FileMode(0o111))
	}
}

func TestExportGitTreeRejectsMissingRepository(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if err := verifyGitRevision(missing, strings.Repeat("0", 40)); err == nil {
		t.Fatal("verifyGitRevision accepted a missing repository")
	}
	if err := exportGitTree(missing, strings.Repeat("0", 40), filepath.Join(t.TempDir(), "export")); err == nil {
		t.Fatal("exportGitTree accepted a missing repository")
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := exportGitTree(t.TempDir(), strings.Repeat("0", 40), blocked); err == nil {
		t.Fatal("exportGitTree accepted a file as destination")
	}
}

func TestBuildFromPinnedGitCheckouts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the local git command fixture uses a POSIX shell")
	}
	matt, superpowers, ecc := exportedProviderRoots(t)
	roots := []string{matt, superpowers, ecc}
	archives := make([]string, len(roots))
	for index, root := range roots {
		archives[index] = createGitArchive(t, root)
	}
	helperRoot := t.TempDir()
	helper := filepath.Join(helperRoot, "git")
	script := `#!/bin/sh
case "$2" in
  "$OAW_TEST_MATT_ROOT") revision=$OAW_TEST_MATT_REVISION; archive=$OAW_TEST_MATT_ARCHIVE ;;
  "$OAW_TEST_SP_ROOT") revision=$OAW_TEST_SP_REVISION; archive=$OAW_TEST_SP_ARCHIVE ;;
  "$OAW_TEST_ECC_ROOT") revision=$OAW_TEST_ECC_REVISION; archive=$OAW_TEST_ECC_ARCHIVE ;;
  *) exit 64 ;;
esac
case "$3" in
  rev-parse) printf '%s\n' "$revision" ;;
  archive) cat "$archive" ;;
  *) exit 64 ;;
esac
`
	if err := os.WriteFile(helper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	mattRevision, _ := provideraudit.LockedRevision("oaw/matt")
	spRevision, _ := provideraudit.LockedRevision("oaw/superpowers")
	eccRevision, _ := provideraudit.LockedRevision("oaw/ecc")
	t.Setenv("OAW_TEST_MATT_ROOT", matt)
	t.Setenv("OAW_TEST_MATT_REVISION", mattRevision)
	t.Setenv("OAW_TEST_MATT_ARCHIVE", archives[0])
	t.Setenv("OAW_TEST_SP_ROOT", superpowers)
	t.Setenv("OAW_TEST_SP_REVISION", spRevision)
	t.Setenv("OAW_TEST_SP_ARCHIVE", archives[1])
	t.Setenv("OAW_TEST_ECC_ROOT", ecc)
	t.Setenv("OAW_TEST_ECC_REVISION", eccRevision)
	t.Setenv("OAW_TEST_ECC_ARCHIVE", archives[2])
	t.Setenv("PATH", helperRoot+string(os.PathListSeparator)+os.Getenv("PATH"))
	manifest, err := buildFromPinnedGitCheckouts(matt, superpowers, ecc)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Providers) != 3 || manifest.Providers[0].Revision != mattRevision || manifest.Providers[2].Revision != eccRevision {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestWriteAtomicRejectsMissingParent(t *testing.T) {
	if err := writeAtomic(filepath.Join(t.TempDir(), "missing", "manifest.json"), []byte("{}\n")); err == nil {
		t.Fatal("writeAtomic accepted a missing parent directory")
	}
	if err := writeAtomic(t.TempDir(), []byte("{}\n")); err == nil {
		t.Fatal("writeAtomic replaced a directory")
	}
}

func TestExtractTrackedArchiveAcceptsCanonicalDirectoryEntries(t *testing.T) {
	var encoded bytes.Buffer
	writer := tar.NewWriter(&encoded)
	if err := writer.WriteHeader(&tar.Header{Name: "skills/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	content := []byte("skill\n")
	if err := writer.WriteHeader(&tar.Header{Name: "skills/SKILL.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := extractTrackedArchive(tar.NewReader(bytes.NewReader(encoded.Bytes())), destination); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(filepath.Join(destination, "skills", "SKILL.md")); err != nil || !bytes.Equal(raw, content) {
		t.Fatalf("extracted content = %q, %v", raw, err)
	}
}

func TestExtractTrackedArchiveAcceptsContainedSymlink(t *testing.T) {
	var encoded bytes.Buffer
	writer := tar.NewWriter(&encoded)
	content := []byte("instructions\n")
	if err := writer.WriteHeader(&tar.Header{Name: "CLAUDE.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: "AGENTS.md", Typeflag: tar.TypeSymlink, Mode: 0o777, Linkname: "CLAUDE.md"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := extractTrackedArchive(tar.NewReader(bytes.NewReader(encoded.Bytes())), destination); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(destination, "AGENTS.md")); err != nil || target != "CLAUDE.md" {
		t.Fatalf("symlink target = %q, %v", target, err)
	}
}

func TestExtractTrackedArchiveRejectsEscapingSymlink(t *testing.T) {
	var encoded bytes.Buffer
	writer := tar.NewWriter(&encoded)
	if err := writer.WriteHeader(&tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Mode: 0o777, Linkname: "../outside"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractTrackedArchive(tar.NewReader(bytes.NewReader(encoded.Bytes())), t.TempDir()); err == nil {
		t.Fatal("extractTrackedArchive accepted escaping symlink")
	}
}

func TestExtractTrackedArchiveRejectsUnsafePathsAndEntryKinds(t *testing.T) {
	tests := []tar.Header{
		{Name: "../escape", Typeflag: tar.TypeReg, Mode: 0o600},
		{Name: "/absolute", Typeflag: tar.TypeReg, Mode: 0o600},
		{Name: "hardlink", Typeflag: tar.TypeLink, Linkname: "target"},
		{Name: "fifo", Typeflag: tar.TypeFifo, Mode: 0o600},
	}
	for _, header := range tests {
		var encoded bytes.Buffer
		writer := tar.NewWriter(&encoded)
		if err := writer.WriteHeader(&header); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := extractTrackedArchive(tar.NewReader(bytes.NewReader(encoded.Bytes())), t.TempDir()); err == nil {
			t.Fatalf("extractTrackedArchive accepted %#v", header)
		}
	}
}

func TestExtractTrackedArchiveRejectsCorruptStream(t *testing.T) {
	if err := extractTrackedArchive(tar.NewReader(strings.NewReader("not-a-tar")), t.TempDir()); err == nil {
		t.Fatal("extractTrackedArchive accepted a corrupt stream")
	}
}

func TestContainedArchiveSymlinkRejectsNonCanonicalTargets(t *testing.T) {
	destination := t.TempDir()
	link := filepath.Join(destination, "nested", "link")
	for _, target := range []string{"", "/absolute", "../../outside", "./target", "nested\\target"} {
		if containedArchiveSymlink(destination, link, target) {
			t.Fatalf("containedArchiveSymlink accepted %q", target)
		}
	}
}

func exportedProviderRoots(t *testing.T) (string, string, string) {
	t.Helper()
	roots := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	checkouts := provideraudit.LockedCheckouts(roots[0], roots[1], roots[2])
	for _, checkout := range checkouts {
		for _, binding := range checkout.BindingRoots {
			path := filepath.Join(checkout.Root, filepath.FromSlash(binding.ContentRoot))
			if extension := filepath.Ext(path); extension == ".md" || extension == ".toml" {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(binding.ID+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				continue
			}
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(binding.ID+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return roots[0], roots[1], roots[2]
}

func runGit(t *testing.T, repository string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

func createGitArchive(t *testing.T, repository string) string {
	t.Helper()
	runGit(t, repository, "init", "-q")
	runGit(t, repository, "add", ".")
	runGit(t, repository, "-c", "user.name=OAW Test", "-c", "user.email=oaw@example.invalid", "commit", "-q", "-m", "fixture")
	archive := filepath.Join(t.TempDir(), "repository.tar")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "-C", repository, "archive", "--format=tar", "HEAD")
	command.Stdout = file
	command.Stderr = os.Stderr
	runErr := command.Run()
	closeErr := file.Close()
	if runErr != nil {
		t.Fatal(runErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return archive
}
