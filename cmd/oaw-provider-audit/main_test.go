package main

import (
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
	if err := run([]string{"--check", "--manifest", t.TempDir() + "/manifest.json", "--matt-root", t.TempDir()}); err == nil || !strings.Contains(err.Error(), "all four") {
		t.Fatalf("check with partial roots error = %v", err)
	}
}

func TestAuditCLIRejectsExportedRootsMode(t *testing.T) {
	matt, superpowers, openaiPlugins, ecc := exportedProviderRoots(t)
	err := run([]string{
		"--write", "--output", filepath.Join(t.TempDir(), "manifest.json"), "--exported",
		"--matt-root", matt, "--superpowers-root", superpowers, "--openai-plugins-root", openaiPlugins, "--ecc-root", ecc,
	})
	if err == nil || !strings.Contains(err.Error(), "CLI_ARGUMENT_INVALID") {
		t.Fatalf("exported roots error = %v", err)
	}
}

func TestAuditCLIRoundTripsPinnedGitRoots(t *testing.T) {
	matt, superpowers, openaiPlugins, ecc := pinnedGitProviderRoots(t)
	manifest := filepath.Join(t.TempDir(), "provider-sources-v5.json")
	providerArgs := []string{"--matt-root", matt, "--superpowers-root", superpowers, "--openai-plugins-root", openaiPlugins, "--ecc-root", ecc}
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

func TestAuditCLIProcessRoundTripsPinnedGitRoots(t *testing.T) {
	matt, superpowers, openaiPlugins, ecc := pinnedGitProviderRoots(t)
	manifest := filepath.Join(t.TempDir(), "provider-sources-v5.json")
	providerArgs := []string{"--matt-root", matt, "--superpowers-root", superpowers, "--openai-plugins-root", openaiPlugins, "--ecc-root", ecc}
	runAuditCLIProcess(t, append([]string{"--write", "--output", manifest}, providerArgs...)...)
	output := runAuditCLIProcess(t, "--validate", "--manifest", manifest)
	if !strings.Contains(output, provideraudit.ProviderSourceAuditSchemaV2) {
		t.Fatalf("validate output = %q", output)
	}
	runAuditCLIProcess(t, append([]string{"--check", "--manifest", manifest}, providerArgs...)...)
}

func TestAuditCLIProcessHelper(t *testing.T) {
	if os.Getenv("OAW_PROVIDER_AUDIT_PROCESS_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator == -1 {
		t.Fatal("missing helper argument separator")
	}
	os.Args = append([]string{"oaw-provider-audit"}, os.Args[separator+1:]...)
	main()
}

func runAuditCLIProcess(t *testing.T, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-test.run=^TestAuditCLIProcessHelper$", "--"}, args...)
	command := exec.Command(os.Args[0], commandArgs...)
	command.Env = append(os.Environ(), "OAW_PROVIDER_AUDIT_PROCESS_HELPER=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("oaw-provider-audit %q failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func TestAuditCLIRejectsConflictingModesAndPaths(t *testing.T) {
	tests := [][]string{
		{},
		{"--validate", "--write", "--manifest", "manifest", "--output", "output"},
		{"--validate", "--manifest", "manifest", "operand"},
		{"--validate", "--manifest", "manifest", "--output", "output"},
		{"--validate", "--manifest", "manifest", "--openai-plugins-root", "c"},
		{"--write", "--output", "output", "--manifest", "manifest", "--matt-root", "a", "--superpowers-root", "b", "--openai-plugins-root", "c", "--ecc-root", "d"},
		{"--check", "--manifest", "manifest", "--output", "output", "--matt-root", "a", "--superpowers-root", "b", "--openai-plugins-root", "c", "--ecc-root", "d"},
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

func TestGitRevisionAndObjectExport(t *testing.T) {
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
	if err := exportGitTree(repository, revision, t.TempDir()); err == nil {
		t.Fatal("exportGitTree accepted an existing destination directory")
	}
}

func TestGitObjectExportPreservesExecutableModeWithRestrictiveUmask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the restrictive umask fixture requires a POSIX shell")
	}
	command := exec.Command("sh", "-c", `umask 077; exec "$1" -test.run='^TestGitRevisionAndObjectExport$' -test.count=1`, "sh", os.Args[0])
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("object export under umask 077 failed: %v\n%s", err, output)
	}
}

func TestExportGitTreeIgnoresReplaceObjectsAndExternalAttributes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the local git command fixture uses POSIX-style repository metadata")
	}
	repository := t.TempDir()
	runGit(t, repository, "init", "-q")
	tracked := filepath.Join(repository, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("locked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "tracked.txt")
	runGit(t, repository, "-c", "user.name=OAW Test", "-c", "user.email=oaw@example.invalid", "commit", "-q", "-m", "fixture")
	revision := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	originalBlob := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD:tracked.txt"))
	replacement := filepath.Join(repository, "replacement.txt")
	if err := os.WriteFile(replacement, []byte("replaced\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replacementBlob := strings.TrimSpace(runGit(t, repository, "hash-object", "-w", replacement))
	runGit(t, repository, "replace", originalBlob, replacementBlob)
	attributes := filepath.Join(repository, ".git", "info", "attributes")
	if err := os.WriteFile(attributes, []byte("tracked.txt export-ignore\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "export")
	if err := exportGitTree(repository, revision, destination); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "locked\n" {
		t.Fatalf("exported tracked content = %q, want locked blob", content)
	}
}

func TestExportGitTreeRejectsCaseAliasedSymlinkAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the symlink fixture requires POSIX symlink behavior")
	}
	if !caseInsensitiveFilesystem(t, t.TempDir()) {
		t.Skip("case-alias behavior requires a case-insensitive filesystem")
	}
	repository := t.TempDir()
	runGit(t, repository, "init", "-q")
	fileBlob := strings.TrimSpace(runGitInput(t, repository, "locked\n", "hash-object", "-w", "--stdin"))
	symlinkBlob := strings.TrimSpace(runGitInput(t, repository, ".", "hash-object", "-w", "--stdin"))
	nestedTree := strings.TrimSpace(runGitInput(t, repository, "100644 blob "+fileBlob+"\tfile\n", "mktree"))
	rootTree := strings.TrimSpace(runGitInput(t, repository,
		"120000 blob "+symlinkBlob+"\tA\n040000 tree "+nestedTree+"\ta\n", "mktree"))
	revision := strings.TrimSpace(runGitInput(t, repository, "", "-c", "user.name=OAW Test", "-c", "user.email=oaw@example.invalid", "commit-tree", rootTree, "-m", "fixture"))

	if err := exportGitTree(repository, revision, filepath.Join(t.TempDir(), "export")); err == nil {
		t.Fatal("exportGitTree followed a case-aliased symlink ancestor")
	}
}

func TestEnsurePhysicalExportParentRejectsSymlink(t *testing.T) {
	destination := t.TempDir()
	if err := os.Symlink(".", filepath.Join(destination, "link")); err != nil {
		t.Fatal(err)
	}
	if err := ensurePhysicalExportParent(destination, "link/file"); err == nil {
		t.Fatal("ensurePhysicalExportParent accepted a symlink ancestor")
	}
}

func TestGitCommandsIgnoreInheritedEnvironment(t *testing.T) {
	repository := t.TempDir()
	runGit(t, repository, "init", "-q")
	tracked := filepath.Join(repository, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("locked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "tracked.txt")
	runGit(t, repository, "-c", "user.name=OAW Test", "-c", "user.email=oaw@example.invalid", "commit", "-q", "-m", "fixture")
	revision := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	invalidConfig := filepath.Join(t.TempDir(), "invalid.gitconfig")
	if err := os.WriteFile(invalidConfig, []byte("[invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "foreign.git"))
	t.Setenv("GIT_CONFIG_GLOBAL", invalidConfig)

	if err := verifyGitRevision(repository, revision); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "export")
	if err := exportGitTree(repository, revision, destination); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(destination, "tracked.txt")); err != nil || string(content) != "locked\n" {
		t.Fatalf("exported content = %q, %v", content, err)
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
	matt, superpowers, openaiPlugins, ecc := pinnedGitProviderRoots(t)
	mattRevision, _ := provideraudit.LockedRevision("oaw/matt", "matt-skills")
	spRevision, _ := provideraudit.LockedRevision("oaw/superpowers", "superpowers")
	openaiPluginsRevision, _ := provideraudit.LockedRevision("oaw/superpowers", "superpowers-codex")
	eccRevision, _ := provideraudit.LockedRevision("oaw/ecc", "ecc")
	manifest, err := buildFromPinnedGitCheckouts(matt, superpowers, openaiPlugins, ecc)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Providers) != 4 || manifest.Providers[0].Revision != mattRevision || manifest.Providers[2].Revision != openaiPluginsRevision || manifest.Providers[3].Revision != eccRevision {
		t.Fatalf("manifest = %#v", manifest)
	}
	gitLog, err := os.ReadFile(os.Getenv("OAW_TEST_GIT_LOG"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		matt + "\tls-tree\t-rz\t--full-tree\t" + mattRevision,
		superpowers + "\tls-tree\t-rz\t--full-tree\t" + spRevision,
		openaiPlugins + "\tls-tree\t-rz\t--full-tree\t" + openaiPluginsRevision,
		ecc + "\tls-tree\t-rz\t--full-tree\t" + eccRevision,
	} {
		if !strings.Contains(string(gitLog), expected+"\n") {
			t.Fatalf("git invocation log does not contain %q:\n%s", expected, gitLog)
		}
	}
	if strings.Contains(string(gitLog), "\tarchive\t") {
		t.Fatalf("git invocation log contains archive:\n%s", gitLog)
	}
}

const pinnedGitFakeScript = `#!/bin/sh
if [ "$1" != "--no-replace-objects" ] || [ "$2" != "-C" ]; then
  exit 65
fi
if [ "$GIT_NO_LAZY_FETCH" != "1" ]; then
  exit 67
fi
root=$3
shift 3
printf '%s' "$root" >> "$OAW_TEST_GIT_LOG"
for argument in "$@"; do
  printf '\t%s' "$argument" >> "$OAW_TEST_GIT_LOG"
done
printf '\n' >> "$OAW_TEST_GIT_LOG"
case "$root" in
  "$OAW_TEST_MATT_ROOT") revision=$OAW_TEST_MATT_REVISION ;;
  "$OAW_TEST_SP_ROOT") revision=$OAW_TEST_SP_REVISION ;;
  "$OAW_TEST_OPENAI_PLUGINS_ROOT") revision=$OAW_TEST_OPENAI_PLUGINS_REVISION ;;
  "$OAW_TEST_ECC_ROOT") revision=$OAW_TEST_ECC_REVISION ;;
  *) exit 64 ;;
esac
case "$1" in
  rev-parse)
    [ "$2" = "HEAD" ] || exit 66
    printf '%s\n' "$revision"
    ;;
  ls-tree)
    [ "$2" = "-rz" ] && [ "$3" = "--full-tree" ] && [ "$4" = "$revision" ] || exit 66
    "$OAW_TEST_REAL_GIT" --no-replace-objects -C "$root" ls-tree -rz --full-tree HEAD
    ;;
  cat-file)
    "$OAW_TEST_REAL_GIT" --no-replace-objects -C "$root" "$@"
    ;;
  *) exit 64 ;;
esac
`

func pinnedGitProviderRoots(t *testing.T) (string, string, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the local git command fixture uses a POSIX shell")
	}
	matt, superpowers, openaiPlugins, ecc := exportedProviderRoots(t)
	roots := []string{matt, superpowers, openaiPlugins, ecc}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		createGitRepository(t, root)
	}
	helperRoot := t.TempDir()
	helper := filepath.Join(helperRoot, "git")
	if err := os.WriteFile(helper, []byte(pinnedGitFakeScript), 0o700); err != nil {
		t.Fatal(err)
	}
	mattRevision, _ := provideraudit.LockedRevision("oaw/matt", "matt-skills")
	spRevision, _ := provideraudit.LockedRevision("oaw/superpowers", "superpowers")
	openaiPluginsRevision, _ := provideraudit.LockedRevision("oaw/superpowers", "superpowers-codex")
	eccRevision, _ := provideraudit.LockedRevision("oaw/ecc", "ecc")
	t.Setenv("OAW_TEST_GIT_LOG", filepath.Join(helperRoot, "invocations.log"))
	t.Setenv("OAW_TEST_REAL_GIT", realGit)
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("OAW_TEST_MATT_ROOT", matt)
	t.Setenv("OAW_TEST_MATT_REVISION", mattRevision)
	t.Setenv("OAW_TEST_SP_ROOT", superpowers)
	t.Setenv("OAW_TEST_SP_REVISION", spRevision)
	t.Setenv("OAW_TEST_OPENAI_PLUGINS_ROOT", openaiPlugins)
	t.Setenv("OAW_TEST_OPENAI_PLUGINS_REVISION", openaiPluginsRevision)
	t.Setenv("OAW_TEST_ECC_ROOT", ecc)
	t.Setenv("OAW_TEST_ECC_REVISION", eccRevision)
	t.Setenv("PATH", helperRoot+string(os.PathListSeparator)+os.Getenv("PATH"))
	return matt, superpowers, openaiPlugins, ecc
}

func TestWriteAtomicRejectsMissingParent(t *testing.T) {
	if err := writeAtomic(filepath.Join(t.TempDir(), "missing", "manifest.json"), []byte("{}\n")); err == nil {
		t.Fatal("writeAtomic accepted a missing parent directory")
	}
	if err := writeAtomic(t.TempDir(), []byte("{}\n")); err == nil {
		t.Fatal("writeAtomic replaced a directory")
	}
}

func TestParseGitTreeRejectsUnsafeEntries(t *testing.T) {
	oid := strings.Repeat("a", 40)
	tests := map[string][]byte{
		"malformed":        []byte("not-a-tree-entry\x00"),
		"absolute path":    []byte("100644 blob " + oid + "\t/escape\x00"),
		"parent path":      []byte("100644 blob " + oid + "\t../escape\x00"),
		"backslash":        []byte("100644 blob " + oid + "\tnested\\escape\x00"),
		"unsupported mode": []byte("160000 commit " + oid + "\tsubmodule\x00"),
		"duplicate":        []byte("100644 blob " + oid + "\tfile\x00100644 blob " + oid + "\tfile\x00"),
		"ancestor conflict": []byte("120000 blob " + oid + "\tlink\x00" +
			"100644 blob " + oid + "\tlink/file\x00"),
		"invalid utf8": append([]byte("100644 blob "+oid+"\t"),
			[]byte{0xff, 0x00}...),
	}
	for name, tree := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseGitTree(tree); err == nil {
				t.Fatalf("parseGitTree accepted %q", tree)
			}
		})
	}
}

func TestContainedGitSymlinkRejectsNonCanonicalTargets(t *testing.T) {
	destination := t.TempDir()
	link := filepath.Join(destination, "nested", "link")
	for _, target := range []string{"", "/absolute", "../../outside", "./target", "nested\\target"} {
		if containedGitSymlink(destination, link, target) {
			t.Fatalf("containedGitSymlink accepted %q", target)
		}
	}
}

func exportedProviderRoots(t *testing.T) (string, string, string, string) {
	t.Helper()
	roots := []string{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	checkouts := provideraudit.LockedCheckouts(roots[0], roots[1], roots[2], roots[3])
	for _, checkout := range checkouts {
		for _, binding := range checkout.BindingRoots {
			path := filepath.Join(checkout.Root, filepath.FromSlash(checkout.DistributionRoot), filepath.FromSlash(binding.ContentRoot))
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
	for _, evidence := range []struct {
		root string
		path string
	}{
		{root: roots[0], path: ".agents/invocation.md"},
		{root: roots[0], path: ".claude-plugin/plugin.json"},
		{root: roots[0], path: "README.md"},
		{root: roots[1], path: "README.md"},
		{root: roots[1], path: "RELEASE-NOTES.md"},
		{root: roots[1], path: "skills/using-superpowers/SKILL.md"},
		{root: filepath.Join(roots[2], "plugins", "superpowers"), path: ".codex-plugin/plugin.json"},
		{root: filepath.Join(roots[2], "plugins", "superpowers"), path: "README.md"},
		{root: filepath.Join(roots[2], "plugins", "superpowers"), path: "skills/using-superpowers/SKILL.md"},
		{root: roots[3], path: ".codex-plugin/plugin.json"},
		{root: roots[3], path: "AGENTS.md"},
		{root: roots[3], path: "hooks/evidence.txt"},
	} {
		path := filepath.Join(evidence.root, filepath.FromSlash(evidence.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("evidence\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return roots[0], roots[1], roots[2], roots[3]
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

func runGitInput(t *testing.T, repository, input string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, args...)...)
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

func caseInsensitiveFilesystem(t *testing.T, root string) bool {
	t.Helper()
	probe := filepath.Join(root, "OAW-Case-Probe")
	if err := os.WriteFile(probe, []byte("probe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Stat(filepath.Join(root, "oaw-case-probe"))
	return err == nil
}

func createGitRepository(t *testing.T, repository string) {
	t.Helper()
	runGit(t, repository, "init", "-q")
	runGit(t, repository, "add", ".")
	runGit(t, repository, "-c", "user.name=OAW Test", "-c", "user.email=oaw@example.invalid", "commit", "-q", "-m", "fixture")
}
