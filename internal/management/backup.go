package management

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type backupCandidate struct {
	original       string
	allowedRoot    string
	relativeSuffix string
	backup         string
	checksum       string
	before         installPathSnapshot
}

type backupPlan struct {
	required   bool
	operation  string
	scope      string
	path       string
	candidates []backupCandidate
}

type terminalMutation struct {
	status  int
	message string
}

func reserveMutationBackupPath(coords coordinates) (string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	name := fmt.Sprintf("%s-%d", timestamp, os.Getpid())
	return validatedDestinationPath(coords.backupRoot, name)
}

func buildMutationBackupPlan(
	required bool,
	operation string,
	scope string,
	path string,
	policy mutationAction,
	targets []mutationAction,
	states []mutationAction,
) (backupPlan, error) {
	if !required {
		return backupPlan{}, nil
	}
	plan := backupPlan{required: true, operation: operation, scope: scope, path: path}
	actions := make([]mutationAction, 0, len(targets)+1+len(states))
	actions = append(actions, policy)
	actions = append(actions, targets...)
	actions = append(actions, states...)
	for _, action := range actions {
		if !mutationActionNeedsBackup(action) {
			continue
		}
		var err error
		plan.candidates, err = addBackupCandidate(plan.candidates, action.destination, action.allowedRoot, action.relativeSuffix, action.before, path)
		if err != nil {
			return backupPlan{}, err
		}
	}
	if len(plan.candidates) == 0 {
		return backupPlan{}, compatibilityError("forced mutation has no recoverable artifacts")
	}
	if _, err := renderBackupManifest(plan); err != nil {
		return backupPlan{}, err
	}
	return cloneBackupPlan(plan), nil
}

func mutationActionNeedsBackup(action mutationAction) bool {
	if action.before.kind != installPathRegular {
		return false
	}
	switch action.effect {
	case mutationRemove:
		return true
	case mutationReplace:
		return !bytes.Equal(action.before.data, action.data)
	default:
		return false
	}
}

func addBackupCandidate(
	candidates []backupCandidate,
	original string,
	root string,
	suffix string,
	before installPathSnapshot,
	backupPath string,
) ([]backupCandidate, error) {
	for _, candidate := range candidates {
		if candidate.original == original {
			return candidates, nil
		}
	}
	if before.kind != installPathRegular {
		return nil, compatibilityError("backup source is not a file: " + original)
	}
	if !safeStateField(original) || !safeStateField(root) || !safeStateField(suffix) {
		return nil, compatibilityError("backup candidate cannot be serialized")
	}
	rebuilt, err := validatedDestinationPath(root, suffix)
	if err != nil {
		return nil, err
	}
	if rebuilt != original {
		return nil, compatibilityError("backup candidate destination does not match: " + original)
	}
	index := len(candidates) + 1
	artifact := backupPath + string(filepath.Separator) + fmt.Sprintf("%03d-%s", index, filepath.Base(original))
	candidate := backupCandidate{
		original: original, allowedRoot: root, relativeSuffix: suffix,
		backup: artifact, checksum: checksumBytes(before.data), before: cloneInstallPathSnapshot(before),
	}
	return append(candidates, candidate), nil
}

func renderBackupManifest(plan backupPlan) ([]byte, error) {
	if !plan.required {
		return nil, compatibilityError("backup plan is not required")
	}
	if plan.operation != "update" && plan.operation != "uninstall" {
		return nil, compatibilityError("invalid backup operation")
	}
	if plan.scope != "user" && plan.scope != "project" {
		return nil, compatibilityError("invalid backup scope")
	}
	if !filepath.IsAbs(plan.path) || !safeStateField(plan.path) {
		return nil, compatibilityError("invalid backup path")
	}
	if len(plan.candidates) == 0 {
		return nil, compatibilityError("forced mutation has no recoverable artifacts")
	}
	seenOriginals := make(map[string]struct{}, len(plan.candidates))
	seenBackups := make(map[string]struct{}, len(plan.candidates))
	var result bytes.Buffer
	result.WriteString("format\t1\n")
	fmt.Fprintf(&result, "operation\t%s\n", plan.operation)
	fmt.Fprintf(&result, "scope\t%s\n", plan.scope)
	for _, candidate := range plan.candidates {
		if !filepath.IsAbs(candidate.original) || !safeStateField(candidate.original) ||
			!filepath.IsAbs(candidate.backup) || !safeStateField(candidate.backup) ||
			!validChecksum(candidate.checksum) {
			return nil, compatibilityError("backup candidate cannot be serialized")
		}
		artifactName := filepath.Base(candidate.backup)
		expectedArtifact := plan.path + string(filepath.Separator) + artifactName
		if candidate.backup != expectedArtifact || filepath.Dir(filepath.Clean(candidate.backup)) != filepath.Clean(plan.path) {
			return nil, compatibilityError("backup artifact escapes operation directory")
		}
		if _, exists := seenOriginals[candidate.original]; exists {
			return nil, compatibilityError("duplicate backup original")
		}
		if _, exists := seenBackups[candidate.backup]; exists {
			return nil, compatibilityError("duplicate backup artifact")
		}
		seenOriginals[candidate.original] = struct{}{}
		seenBackups[candidate.backup] = struct{}{}
		fmt.Fprintf(&result, "artifact\t%s\t%s\t%s\n", candidate.original, candidate.backup, candidate.checksum)
	}
	return result.Bytes(), nil
}

func cloneBackupPlan(plan backupPlan) backupPlan {
	source := plan.candidates
	plan.candidates = make([]backupCandidate, len(source))
	for index, candidate := range source {
		candidate.before = cloneInstallPathSnapshot(candidate.before)
		plan.candidates[index] = candidate
	}
	return plan
}

func applyMutationBackup(plan backupPlan, environment Environment) (string, error) {
	manifest, err := renderBackupManifest(plan)
	if err != nil {
		return "", err
	}
	if err := revalidateBackupSources(plan); err != nil {
		return "", err
	}
	relative, err := stateActionRelativeSuffix(environment.StateHome, plan.path)
	if err != nil {
		return "", err
	}
	root, err := openInstallRoot(environment.StateHome)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := createPrivateBackupPath(root, relative, plan.path); err != nil {
		return "", err
	}
	operationRoot, err := root.OpenRoot(filepath.ToSlash(relative))
	if err != nil {
		return "", installIOError("cannot enter backup directory: " + plan.path)
	}
	defer operationRoot.Close()
	for _, candidate := range plan.candidates {
		if err := writePrivateBackupFile(operationRoot, filepath.Base(candidate.backup), candidate.before.data); err != nil {
			return "", err
		}
		copied, err := operationRoot.ReadFile(filepath.Base(candidate.backup))
		if err != nil || checksumBytes(copied) != candidate.checksum {
			return "", &Error{Status: 74, Message: "backup verification failed: " + candidate.original}
		}
	}
	if err := writePrivateBackupFile(operationRoot, "manifest.tsv", manifest); err != nil {
		return "", err
	}
	if err := revalidateBackupSources(plan); err != nil {
		return "", err
	}
	syncScopedDirectory(operationRoot, ".")
	return "oaw: backup: " + plan.path, nil
}

func revalidateBackupSources(plan backupPlan) error {
	for _, candidate := range plan.candidates {
		rebuilt, err := validatedDestinationPath(candidate.allowedRoot, candidate.relativeSuffix)
		if err != nil {
			return err
		}
		if rebuilt != candidate.original {
			return compatibilityError("backup candidate destination does not match: " + candidate.original)
		}
		current, err := inspectInstallPath(candidate.original)
		if err != nil {
			return err
		}
		if current.kind != installPathRegular || checksumBytes(current.data) != candidate.checksum {
			return compatibilityError("backup source changed before mutation: " + candidate.original)
		}
	}
	return nil
}

func createPrivateBackupPath(root *os.Root, relative, expected string) error {
	components := strings.Split(filepath.ToSlash(relative), "/")
	consumed := ""
	for index, component := range components {
		if consumed == "" {
			consumed = component
		} else {
			consumed = path.Join(consumed, component)
		}
		info, err := root.Lstat(consumed)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return compatibilityError("destination path contains a symlink: " + filepath.Join(root.Name(), filepath.FromSlash(consumed)))
			}
			if !info.IsDir() {
				return compatibilityError("destination path component is not a directory: " + filepath.Join(root.Name(), filepath.FromSlash(consumed)))
			}
			if index == len(components)-1 {
				return compatibilityError("backup directory already exists: " + expected)
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return compatibilityError("destination path could not be inspected: " + expected)
		}
		if err := root.Mkdir(consumed, 0o700); err != nil {
			return &Error{Status: 73, Message: "cannot create backup directory: " + expected}
		}
		if err := root.Chmod(consumed, 0o700); err != nil {
			return &Error{Status: 73, Message: "cannot protect backup directory: " + expected}
		}
	}
	return nil
}

func writePrivateBackupFile(root *os.Root, name string, data []byte) error {
	if path.Base(name) != name || name == "." || name == ".." {
		return compatibilityError("invalid backup artifact name")
	}
	temporaryName, temporary, err := createScopedTemporary(root, ".")
	if err != nil {
		return err
	}
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = root.Remove(temporaryName)
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		return installIOError("cannot write backup artifact")
	}
	if err := temporary.Chmod(0o600); err != nil {
		return installIOError("cannot protect backup artifact")
	}
	if err := temporary.Sync(); err != nil {
		return installIOError("cannot sync backup artifact")
	}
	if err := temporary.Close(); err != nil {
		return installIOError("cannot close backup artifact")
	}
	if _, err := root.Lstat(name); err == nil {
		return compatibilityError("backup artifact already exists: " + name)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return compatibilityError("backup artifact could not be inspected: " + name)
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return installIOError("cannot publish backup artifact")
	}
	keepTemporary = false
	return nil
}
