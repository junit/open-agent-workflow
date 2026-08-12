package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "oaw-provider-audit: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("oaw-provider-audit", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	validateMode := flags.Bool("validate", false, "validate a committed manifest")
	writeMode := flags.Bool("write", false, "write a manifest from pinned checkouts")
	checkMode := flags.Bool("check", false, "compare pinned checkouts with a committed manifest")
	manifestPath := flags.String("manifest", "", "committed manifest path")
	outputPath := flags.String("output", "", "output manifest path")
	mattRoot := flags.String("matt-root", "", "Matt source checkout")
	superpowersRoot := flags.String("superpowers-root", "", "Superpowers source checkout")
	openaiPluginsRoot := flags.String("openai-plugins-root", "", "OpenAI plugins source checkout")
	eccRoot := flags.String("ecc-root", "", "ECC source checkout")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: positional arguments are not supported")
	}
	modes := 0
	for _, selected := range []bool{*validateMode, *writeMode, *checkMode} {
		if selected {
			modes++
		}
	}
	if modes != 1 {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: select exactly one of --validate, --write, or --check")
	}
	if *validateMode {
		if *manifestPath == "" || *outputPath != "" || *mattRoot != "" || *superpowersRoot != "" || *openaiPluginsRoot != "" || *eccRoot != "" {
			return fmt.Errorf("CLI_ARGUMENT_INVALID: --validate requires only an explicit --manifest")
		}
		raw, err := os.ReadFile(*manifestPath)
		if err != nil {
			return err
		}
		manifest, err := provideraudit.Decode(raw)
		if err != nil {
			return err
		}
		fmt.Printf("%s %s\n", manifest.SchemaVersion, manifest.Digest)
		return nil
	}
	if *writeMode && (*outputPath == "" || *manifestPath != "") {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: --write requires only an explicit --output")
	}
	if *checkMode && (*manifestPath == "" || *outputPath != "") {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: --check requires only an explicit --manifest")
	}
	if *mattRoot == "" || *superpowersRoot == "" || *openaiPluginsRoot == "" || *eccRoot == "" {
		return fmt.Errorf("CLI_ARGUMENT_INVALID: all four explicit Distribution roots are required")
	}
	manifest, err := buildFromPinnedGitCheckouts(*mattRoot, *superpowersRoot, *openaiPluginsRoot, *eccRoot)
	if err != nil {
		return err
	}
	encoded, err := canonicalManifest(manifest)
	if err != nil {
		return err
	}
	if *writeMode {
		return writeAtomic(*outputPath, encoded)
	}
	committed, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	if _, err := provideraudit.Decode(committed); err != nil {
		return err
	}
	if !bytes.Equal(committed, encoded) {
		return fmt.Errorf("PROVIDER_AUDIT_DRIFT: committed manifest differs from pinned source trees")
	}
	return nil
}

func buildFromPinnedGitCheckouts(mattRoot, superpowersRoot, openaiPluginsRoot, eccRoot string) (provideraudit.Manifest, error) {
	checkouts := provideraudit.LockedCheckouts(mattRoot, superpowersRoot, openaiPluginsRoot, eccRoot)
	exportRoot, err := os.MkdirTemp("", "oaw-provider-audit-")
	if err != nil {
		return provideraudit.Manifest{}, err
	}
	defer os.RemoveAll(exportRoot)
	for index := range checkouts {
		checkout := &checkouts[index]
		if err := verifyGitRevision(checkout.Root, checkout.Revision); err != nil {
			return provideraudit.Manifest{}, err
		}
		destinationName := strings.ReplaceAll(checkout.ProviderID+"-"+checkout.DistributionID, "/", "-")
		destination := filepath.Join(exportRoot, destinationName)
		if err := exportGitTree(checkout.Root, checkout.Revision, destination); err != nil {
			return provideraudit.Manifest{}, err
		}
		checkout.Root = destination
	}
	return provideraudit.Build(checkouts)
}

func verifyGitRevision(root, expected string) error {
	output, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: verify %s: %w", root, err)
	}
	if strings.TrimSpace(string(output)) != expected {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: checkout %s is not pinned to %s", root, expected)
	}
	return nil
}

func exportGitTree(root, revision, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: create private export destination: %w", err)
	}
	removeDestination := true
	defer func() {
		if removeDestination {
			_ = os.RemoveAll(destination)
		}
	}()

	encodedTree, err := gitOutput(root, "ls-tree", "-rz", "--full-tree", revision)
	if err != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: read locked tree: %w", err)
	}
	entries, err := parseGitTree(encodedTree)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := exportGitTreeEntry(root, destination, entry); err != nil {
			return err
		}
	}
	removeDestination = false
	return nil
}

type gitTreeEntry struct {
	mode     string
	objectID string
	path     string
}

func parseGitTree(encoded []byte) ([]gitTreeEntry, error) {
	entries := make([]gitTreeEntry, 0)
	seen := make(map[string]struct{})
	for len(encoded) > 0 {
		terminator := bytes.IndexByte(encoded, 0)
		if terminator < 0 {
			return nil, fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: unterminated tree entry")
		}
		record := encoded[:terminator]
		encoded = encoded[terminator+1:]
		if !utf8.Valid(record) {
			return nil, fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: tree entry is not UTF-8")
		}
		metadata, name, found := bytes.Cut(record, []byte{'\t'})
		fields := strings.Split(string(metadata), " ")
		if !found || len(fields) != 3 || !validGitObjectID(fields[2]) {
			return nil, fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: malformed tree entry %q", record)
		}
		mode, objectType := fields[0], fields[1]
		if objectType != "blob" || (mode != "100644" && mode != "100755" && mode != "120000") {
			return nil, fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: unsupported tree entry mode %q and type %q", mode, objectType)
		}
		entryPath := string(name)
		if !safeGitTreePath(entryPath) {
			return nil, fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: unsafe tree path %q", entryPath)
		}
		if _, duplicate := seen[entryPath]; duplicate {
			return nil, fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: duplicate tree path %q", entryPath)
		}
		seen[entryPath] = struct{}{}
		entries = append(entries, gitTreeEntry{mode: mode, objectID: fields[2], path: entryPath})
	}
	for _, entry := range entries {
		for ancestor := path.Dir(entry.path); ancestor != "."; ancestor = path.Dir(ancestor) {
			if _, conflict := seen[ancestor]; conflict {
				return nil, fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: tree path %q has non-directory ancestor %q", entry.path, ancestor)
			}
		}
	}
	return entries, nil
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func safeGitTreePath(value string) bool {
	cleaned := path.Clean(value)
	if value == "" || !utf8.ValidString(value) || path.IsAbs(value) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != value || strings.Contains(value, "\\") {
		return false
	}
	local := filepath.FromSlash(value)
	return filepath.VolumeName(local) == "" && !filepath.IsAbs(local)
}

func exportGitTreeEntry(root, destination string, entry gitTreeEntry) error {
	if err := ensurePhysicalExportParent(destination, entry.path); err != nil {
		return err
	}
	content, err := gitOutput(root, "cat-file", "blob", entry.objectID)
	if err != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: read blob %s: %w", entry.objectID, err)
	}
	target := filepath.Join(destination, filepath.FromSlash(entry.path))
	if entry.mode == "120000" {
		linkTarget := string(content)
		if !containedGitSymlink(destination, target, linkTarget) {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: unsafe symlink target %q", linkTarget)
		}
		if err := os.Symlink(linkTarget, target); err != nil {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: create symlink %q: %w", entry.path, err)
		}
		return nil
	}
	mode := os.FileMode(0o600)
	if entry.mode == "100755" {
		mode |= 0o111
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: create file %q: %w", entry.path, err)
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: set file mode %q: %w", entry.path, err)
	}
	written, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: write file %q: %w", entry.path, writeErr)
	}
	if written != len(content) {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: write file %q: %w", entry.path, io.ErrShortWrite)
	}
	if closeErr != nil {
		return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: close file %q: %w", entry.path, closeErr)
	}
	return nil
}

func ensurePhysicalExportParent(destination, entryPath string) error {
	parent := path.Dir(entryPath)
	if parent == "." {
		return nil
	}
	current := destination
	for _, component := range strings.Split(parent, "/") {
		physicalParent := current
		current = filepath.Join(current, filepath.FromSlash(component))
		if err := os.Mkdir(current, 0o700); err == nil {
			if err := os.Chmod(current, 0o700); err != nil {
				return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: set export directory mode %q: %w", parent, err)
			}
			continue
		} else if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: create export directory %q: %w", parent, err)
		}
		children, err := os.ReadDir(physicalParent)
		if err != nil {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: inspect export directory %q: %w", parent, err)
		}
		exact := false
		for _, child := range children {
			if child.Name() == component {
				exact = true
				break
			}
		}
		if !exact {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: tree path %q aliases an existing filesystem path", entryPath)
		}
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: inspect export ancestor %q: %w", component, err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("PROVIDER_AUDIT_GIT_INVALID: tree path %q has non-directory filesystem ancestor %q", entryPath, component)
		}
	}
	return nil
}

func containedGitSymlink(destination, linkPath, target string) bool {
	if target == "" || !utf8.ValidString(target) || strings.ContainsRune(target, 0) || path.IsAbs(target) || path.Clean(target) != target || strings.Contains(target, "\\") {
		return false
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), filepath.FromSlash(target)))
	relative, err := filepath.Rel(destination, resolved)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func gitOutput(root string, args ...string) ([]byte, error) {
	command := gitCommand(root, args...)
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr)
		}
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

func gitCommand(root string, args ...string) *exec.Cmd {
	gitArgs := make([]string, 0, len(args)+3)
	gitArgs = append(gitArgs, "--no-replace-objects", "-C", root)
	gitArgs = append(gitArgs, args...)
	command := exec.Command("git", gitArgs...)
	command.Env = isolatedGitEnvironment(os.Environ())
	return command
}

func isolatedGitEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment)+5)
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if len(key) >= 4 && strings.EqualFold(key[:4], "GIT_") {
			continue
		}
		result = append(result, entry)
	}
	return append(result,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_NO_LAZY_FETCH=1",
		"GIT_TERMINAL_PROMPT=0",
	)
}

func canonicalManifest(manifest provideraudit.Manifest) ([]byte, error) {
	encoded, err := canonicaljson.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func writeAtomic(target string, content []byte) error {
	directory := filepath.Dir(target)
	temporary, err := os.CreateTemp(directory, ".provider-sources-v4-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return err
	}
	remove = false
	return nil
}
