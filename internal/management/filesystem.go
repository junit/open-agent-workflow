package management

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
)

type createdDirectorySet map[string]fs.FileInfo

func scopedAtomicReplaceMutation(action mutationAction) error {
	return scopedAtomicReplaceMutationWithDirectories(action, nil, nil)
}

func scopedAtomicReplaceMutationWithDirectories(
	action mutationAction,
	planned map[string]struct{},
	created createdDirectorySet,
) error {
	root, err := openExistingInstallRoot(action.allowedRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	install := installAction{
		label: action.label, data: action.data, destination: action.destination,
		mode: action.mode, allowedRoot: action.allowedRoot,
		relativeSuffix: action.relativeSuffix, before: action.before,
	}
	if len(planned) != 0 {
		if err := requirePlannedMutationDirectories(root, install, planned); err != nil {
			return err
		}
		if err := ensureScopedInstallDirectory(root, install, planned, created); err != nil {
			return err
		}
		refreshed, captureErr := captureMutationPathIdentity(action.allowedRoot, action.destination)
		if captureErr != nil {
			return captureErr
		}
		if !sameMutationFileIdentity(action.identity.root, refreshed.root) {
			return integrityError("destination identity changed after preparation: " + action.destination)
		}
		action.identity.parent = refreshed.parent
		action.identity.destination = refreshed.destination
	}
	if err := revalidateScopedAction(root, install); err != nil {
		return err
	}
	directoryRoot, err := openScopedActionDirectory(root, install)
	if err != nil {
		return err
	}
	defer directoryRoot.Close()
	temporaryName, temporary, err := createScopedTemporary(directoryRoot, ".")
	if err != nil {
		return err
	}
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = directoryRoot.Remove(temporaryName)
		}
	}()
	if err := writeScopedTemporaryFile(temporary, action.data, action.mode, action.destination); err != nil {
		return err
	}
	if err := publishScopedMutationReplacement(root, directoryRoot, install, action, temporaryName); err != nil {
		return err
	}
	keepTemporary = false
	syncScopedDirectory(directoryRoot, ".")
	return nil
}

func requirePlannedMutationDirectories(
	root *os.Root,
	action installAction,
	planned map[string]struct{},
) error {
	directorySuffix := path.Dir(action.relativeSuffix)
	if directorySuffix == "." {
		return nil
	}
	consumed := ""
	for _, component := range strings.Split(directorySuffix, "/") {
		if consumed == "" {
			consumed = component
		} else {
			consumed += "/" + component
		}
		expected, err := validatedDestinationPath(action.allowedRoot, consumed)
		if err != nil {
			return err
		}
		info, err := root.Lstat(consumed)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return integrityError("destination path contains a symlink: " + expected)
			}
			if !info.IsDir() {
				return integrityError("destination path component is not a directory: " + expected)
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return integrityError("destination path could not be inspected: " + expected)
		}
		if _, found := planned[expected]; !found {
			return integrityError("unplanned destination directory is missing: " + expected)
		}
	}
	return nil
}

func publishScopedMutationReplacement(
	root *os.Root,
	directoryRoot *os.Root,
	install installAction,
	action mutationAction,
	temporaryName string,
) error {
	if err := revalidateScopedAction(root, install); err != nil {
		return err
	}
	if err := verifyScopedActionDirectory(directoryRoot, install); err != nil {
		return err
	}
	if err := revalidateMutationActionSnapshot(action); err != nil {
		return err
	}
	destinationName := path.Base(action.relativeSuffix)
	if info, err := directoryRoot.Lstat(destinationName); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return integrityError("destination path contains a symlink: " + action.destination)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return integrityError("destination path could not be inspected: " + action.destination)
	}
	if err := directoryRoot.Rename(temporaryName, destinationName); err != nil {
		return installIOError("cannot replace destination: " + action.destination)
	}
	return nil
}

func scopedAtomicRemoveMutation(action mutationAction) error {
	root, err := openExistingInstallRoot(action.allowedRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	install := installAction{
		label: action.label, destination: action.destination, allowedRoot: action.allowedRoot,
		relativeSuffix: action.relativeSuffix, before: action.before,
	}
	if err := revalidateScopedAction(root, install); err != nil {
		return err
	}
	if err := revalidateMutationActionSnapshot(action); err != nil {
		return err
	}
	directoryRoot, err := openScopedActionDirectory(root, install)
	if err != nil {
		return err
	}
	defer directoryRoot.Close()
	name := path.Base(action.relativeSuffix)
	info, err := directoryRoot.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return integrityError("destination path could not be inspected: " + action.destination)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return integrityError("destination path contains a symlink: " + action.destination)
	}
	if !info.Mode().IsRegular() {
		return integrityError("mutation destination is not a regular file: " + action.destination)
	}
	if err := revalidateMutationActionSnapshot(action); err != nil {
		return err
	}
	if err := directoryRoot.Remove(name); err != nil {
		return installIOError("cannot remove destination: " + action.destination)
	}
	syncScopedDirectory(directoryRoot, ".")
	return nil
}

func scopedRemoveMutationDirectory(action directoryAction) (bool, error) {
	current, err := inspectInstallPath(action.destination)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(current, action.before) {
		return false, integrityError("owned directory changed after preparation: " + action.destination)
	}
	if err := revalidateMutationPathIdentity(action.identity, action.allowedRoot, action.destination); err != nil {
		return false, err
	}
	if current.kind == installPathMissing {
		return false, nil
	}
	root, err := openExistingInstallRoot(action.allowedRoot)
	if err != nil {
		return false, err
	}
	defer root.Close()
	return removeScopedMutationDirectory(root, action)
}

func removeScopedMutationDirectory(root *os.Root, action directoryAction) (bool, error) {
	rebuilt, err := validatedDestinationPath(action.allowedRoot, action.relativeSuffix)
	if err != nil {
		return false, err
	}
	if rebuilt != action.destination {
		return false, integrityError("directory action destination does not match registry: " + action.destination)
	}
	install := installAction{
		destination: action.destination, allowedRoot: action.allowedRoot,
		relativeSuffix: action.relativeSuffix,
	}
	if err := revalidateScopedAction(root, install); err != nil {
		return false, err
	}
	directoryRoot, err := openScopedActionDirectory(root, install)
	if err != nil {
		return false, err
	}
	defer directoryRoot.Close()
	name := path.Base(action.relativeSuffix)
	info, err := directoryRoot.Lstat(name)
	if err != nil {
		return false, integrityError("owned directory changed before removal: " + action.destination)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, integrityError("owned directory changed before removal: " + action.destination)
	}
	if !sameMutationFileIdentity(action.identity.destination, info) {
		return false, integrityError("destination identity changed after preparation: " + action.destination)
	}
	if err := revalidateMutationPathIdentity(action.identity, action.allowedRoot, action.destination); err != nil {
		return false, err
	}
	if err := verifyScopedActionDirectory(directoryRoot, install); err != nil {
		return false, err
	}
	if err := directoryRoot.Remove(name); err != nil {
		if errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
			return false, nil
		}
		return false, installIOError("cannot remove owned directory: " + action.destination)
	}
	syncScopedDirectory(directoryRoot, ".")
	return true, nil
}

func scopedAtomicReplace(action installAction, planned map[string]struct{}, created createdDirectorySet) error {
	root, err := openInstallRoot(action.allowedRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := ensureScopedInstallDirectory(root, action, planned, created); err != nil {
		return err
	}
	if _, err := revalidateInstallActionSnapshot(action); err != nil {
		return err
	}
	if err := revalidateScopedAction(root, action); err != nil {
		return err
	}
	directoryRoot, err := openScopedActionDirectory(root, action)
	if err != nil {
		return err
	}
	defer directoryRoot.Close()

	temporaryName, temporary, err := createScopedTemporary(directoryRoot, ".")
	if err != nil {
		return err
	}
	keepTemporary := true
	defer func() {
		_ = temporary.Close()
		if keepTemporary {
			_ = directoryRoot.Remove(temporaryName)
		}
	}()
	if err := writeScopedTemporaryFile(temporary, action.data, action.mode, action.destination); err != nil {
		return err
	}
	if err := publishScopedInstallReplacement(root, directoryRoot, action, temporaryName); err != nil {
		return err
	}
	keepTemporary = false
	syncScopedDirectory(directoryRoot, ".")
	return nil
}

func publishScopedInstallReplacement(
	root *os.Root,
	directoryRoot *os.Root,
	action installAction,
	temporaryName string,
) error {
	if err := revalidateScopedAction(root, action); err != nil {
		return err
	}
	if _, err := revalidateInstallActionSnapshot(action); err != nil {
		return err
	}
	if err := verifyScopedActionDirectory(directoryRoot, action); err != nil {
		return err
	}
	destinationName := path.Base(action.relativeSuffix)
	if info, err := directoryRoot.Lstat(destinationName); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return integrityError("destination path contains a symlink: " + action.destination)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return integrityError("destination path could not be inspected: " + action.destination)
	}
	if err := directoryRoot.Rename(temporaryName, destinationName); err != nil {
		return installIOError("cannot replace destination: " + action.destination)
	}
	return nil
}

func openScopedActionDirectory(root *os.Root, action installAction) (*os.Root, error) {
	directory := path.Dir(action.relativeSuffix)
	directoryRoot, err := root.OpenRoot(directory)
	if err != nil {
		return nil, integrityError("cannot enter destination directory: " + filepath.Dir(action.destination))
	}
	if err := verifyScopedActionDirectory(directoryRoot, action); err != nil {
		directoryRoot.Close()
		return nil, err
	}
	return directoryRoot, nil
}

func verifyScopedActionDirectory(directoryRoot *os.Root, action installAction) error {
	expected, err := os.Stat(filepath.Dir(action.destination))
	if err != nil || !expected.IsDir() {
		return integrityError("destination directory changed during creation: " + filepath.Dir(action.destination))
	}
	opened, err := directoryRoot.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(expected, opened) {
		return integrityError("destination directory changed during creation: " + filepath.Dir(action.destination))
	}
	return nil
}

func openInstallRoot(name string) (*os.Root, error) {
	info, err := os.Lstat(name)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, integrityError("allowed root could not be inspected: " + name)
		}
		if err := os.MkdirAll(name, 0o755); err != nil {
			return nil, &Error{Status: 73, Message: "cannot create allowed root: " + name}
		}
		info, err = os.Lstat(name)
	}
	if err != nil {
		return nil, integrityError("allowed root could not be inspected: " + name)
	}
	return openInspectedInstallRoot(name, info)
}

func openExistingInstallRoot(name string) (*os.Root, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, integrityError("allowed root could not be inspected: " + name)
	}
	return openInspectedInstallRoot(name, info)
}

func openInspectedInstallRoot(name string, info fs.FileInfo) (*os.Root, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, integrityError("allowed root is a symlink: " + name)
	}
	if !info.IsDir() {
		return nil, integrityError("allowed root is not a directory: " + name)
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, integrityError("cannot enter allowed root: " + name)
	}
	if err := verifyOpenedInstallRoot(name, info, root); err != nil {
		root.Close()
		return nil, err
	}
	return root, nil
}

func verifyOpenedInstallRoot(name string, inspected fs.FileInfo, root *os.Root) error {
	opened, err := root.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(inspected, opened) {
		return integrityError("allowed root changed while opening: " + name)
	}
	return nil
}

func ensureScopedInstallDirectory(root *os.Root, action installAction, planned map[string]struct{}, created createdDirectorySet) error {
	directorySuffix := path.Dir(action.relativeSuffix)
	if directorySuffix == "." {
		return nil
	}
	consumed := ""
	for _, component := range strings.Split(directorySuffix, "/") {
		if consumed == "" {
			consumed = component
		} else {
			consumed += "/" + component
		}
		expected, err := validatedDestinationPath(action.allowedRoot, consumed)
		if err != nil {
			return err
		}
		if err := ensureScopedInstallDirectoryComponent(root, consumed, expected, planned, created); err != nil {
			return err
		}
	}
	return nil
}

func ensureScopedInstallDirectoryComponent(
	root *os.Root,
	relative string,
	expected string,
	planned map[string]struct{},
	created createdDirectorySet,
) error {
	_, isPlanned := planned[expected]
	createdIdentity, wasCreated := created[expected]
	info, err := root.Lstat(relative)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return integrityError("destination path contains a symlink: " + expected)
		}
		if isPlanned && !wasCreated {
			return integrityError("owned directory appeared before creation: " + expected)
		}
		if wasCreated && !sameMutationFileIdentity(createdIdentity, info) {
			return integrityError("created owned directory identity changed: " + expected)
		}
		if !info.IsDir() {
			return integrityError("destination path component is not a directory: " + expected)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return integrityError("destination path could not be inspected: " + expected)
	}
	if err := root.Mkdir(relative, 0o755); err != nil {
		if isPlanned {
			return integrityError("owned directory appeared before creation: " + expected)
		}
		return &Error{Status: 73, Message: "cannot create destination directory: " + expected}
	}
	info, err = root.Lstat(relative)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return integrityError("destination directory changed during creation: " + expected)
	}
	if isPlanned {
		created[expected] = info
	}
	return nil
}

func revalidateScopedAction(root *os.Root, action installAction) error {
	rebuilt, err := validatedDestinationPath(action.allowedRoot, action.relativeSuffix)
	if err != nil {
		return err
	}
	if !matchesValidatedDestination(rebuilt, action.destination) {
		return integrityError("destination changed after preparation: " + action.destination)
	}
	components := strings.Split(action.relativeSuffix, "/")
	consumed := ""
	for index, component := range components {
		if consumed == "" {
			consumed = component
		} else {
			consumed += "/" + component
		}
		info, err := root.Lstat(consumed)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && index == len(components)-1 {
				return nil
			}
			return integrityError("destination path could not be inspected: " + action.destination)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return integrityError("destination path contains a symlink: " + filepath.Join(action.allowedRoot, filepath.FromSlash(consumed)))
		}
		if index != len(components)-1 && !info.IsDir() {
			return integrityError("destination path component is not a directory: " + filepath.Join(action.allowedRoot, filepath.FromSlash(consumed)))
		}
	}
	return nil
}

func createScopedTemporary(root *os.Root, directory string) (string, *os.File, error) {
	for range 100 {
		var random [8]byte
		if _, err := io.ReadFull(rand.Reader, random[:]); err != nil {
			return "", nil, installIOError("cannot generate temporary file name")
		}
		name := ".oaw-" + hex.EncodeToString(random[:])
		if directory != "." {
			name = path.Join(directory, name)
		}
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", nil, &Error{Status: 73, Message: "cannot create temporary file"}
		}
	}
	return "", nil, &Error{Status: 73, Message: "cannot reserve temporary file name"}
}

func writeScopedTemporaryFile(file *os.File, data []byte, mode fs.FileMode, destination string) error {
	if _, err := file.Write(data); err != nil {
		return installIOError("cannot write temporary file for " + destination)
	}
	if err := file.Chmod(mode); err != nil {
		return installIOError("cannot set destination mode for " + destination)
	}
	if err := file.Sync(); err != nil {
		return installIOError("cannot sync temporary file for " + destination)
	}
	if err := file.Close(); err != nil {
		return installIOError("cannot close temporary file for " + destination)
	}
	return nil
}

func syncScopedDirectory(root *os.Root, directory string) {
	file, err := root.Open(directory)
	if err != nil {
		return
	}
	defer file.Close()
	_ = file.Sync()
}

func installIOError(message string) error {
	return &Error{Status: 74, Message: message}
}

func validateCreatedInstallDirectories(planned map[string]struct{}, created createdDirectorySet) error {
	for directory := range planned {
		if _, exists := created[directory]; !exists {
			return integrityError("planned owned directory was not created: " + directory)
		}
	}
	for directory := range created {
		if _, exists := planned[directory]; !exists {
			return integrityError("unplanned owned directory was created: " + directory)
		}
		if created[directory] == nil {
			return integrityError("created owned directory identity is missing: " + directory)
		}
	}
	return nil
}

func installPathSet(values []string) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || !filepath.IsAbs(value) || !safeStateField(value) {
			return nil, integrityError("invalid planned owned directory: " + value)
		}
		if _, exists := result[value]; exists {
			return nil, integrityError("duplicate planned owned directory: " + value)
		}
		result[value] = struct{}{}
	}
	return result, nil
}
