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
	"strings"
)

func scopedAtomicReplace(action installAction, planned, created map[string]struct{}) error {
	root, err := openInstallRoot(action.allowedRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := ensureScopedInstallDirectory(root, action, planned, created); err != nil {
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
	if _, err := temporary.Write(action.data); err != nil {
		return installIOError("cannot write temporary file for " + action.destination)
	}
	if err := temporary.Chmod(action.mode); err != nil {
		return installIOError("cannot set destination mode for " + action.destination)
	}
	if err := temporary.Sync(); err != nil {
		return installIOError("cannot sync temporary file for " + action.destination)
	}
	if err := temporary.Close(); err != nil {
		return installIOError("cannot close temporary file for " + action.destination)
	}
	if err := revalidateScopedAction(root, action); err != nil {
		return err
	}
	if err := verifyScopedActionDirectory(directoryRoot, action); err != nil {
		return err
	}
	destinationName := path.Base(action.relativeSuffix)
	if info, err := directoryRoot.Lstat(destinationName); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return compatibilityError("destination path contains a symlink: " + action.destination)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return compatibilityError("destination path could not be inspected: " + action.destination)
	}
	if err := directoryRoot.Rename(temporaryName, destinationName); err != nil {
		return installIOError("cannot replace destination: " + action.destination)
	}
	keepTemporary = false
	syncScopedDirectory(directoryRoot, ".")
	return nil
}

func openScopedActionDirectory(root *os.Root, action installAction) (*os.Root, error) {
	directory := path.Dir(action.relativeSuffix)
	directoryRoot, err := root.OpenRoot(directory)
	if err != nil {
		return nil, compatibilityError("cannot enter destination directory: " + filepath.Dir(action.destination))
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
		return compatibilityError("destination directory changed during creation: " + filepath.Dir(action.destination))
	}
	opened, err := directoryRoot.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(expected, opened) {
		return compatibilityError("destination directory changed during creation: " + filepath.Dir(action.destination))
	}
	return nil
}

func openInstallRoot(name string) (*os.Root, error) {
	info, err := os.Lstat(name)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, compatibilityError("allowed root could not be inspected: " + name)
		}
		if err := os.MkdirAll(name, 0o755); err != nil {
			return nil, &Error{Status: 73, Message: "cannot create allowed root: " + name}
		}
		info, err = os.Lstat(name)
	}
	if err != nil {
		return nil, compatibilityError("allowed root could not be inspected: " + name)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, compatibilityError("allowed root is a symlink: " + name)
	}
	if !info.IsDir() {
		return nil, compatibilityError("allowed root is not a directory: " + name)
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, compatibilityError("cannot enter allowed root: " + name)
	}
	return root, nil
}

func ensureScopedInstallDirectory(root *os.Root, action installAction, planned, created map[string]struct{}) error {
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
		_, isPlanned := planned[expected]
		_, wasCreated := created[expected]
		info, err := root.Lstat(consumed)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return compatibilityError("destination path contains a symlink: " + expected)
			}
			if isPlanned && !wasCreated {
				return compatibilityError("owned directory appeared before creation: " + expected)
			}
			if !info.IsDir() {
				return compatibilityError("destination path component is not a directory: " + expected)
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return compatibilityError("destination path could not be inspected: " + expected)
		}
		if err := root.Mkdir(consumed, 0o755); err != nil {
			if isPlanned {
				return compatibilityError("owned directory appeared before creation: " + expected)
			}
			return &Error{Status: 73, Message: "cannot create destination directory: " + expected}
		}
		info, err = root.Lstat(consumed)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return compatibilityError("destination directory changed during creation: " + expected)
		}
		if isPlanned {
			created[expected] = struct{}{}
		}
	}
	return nil
}

func revalidateScopedAction(root *os.Root, action installAction) error {
	rebuilt, err := validatedDestinationPath(action.allowedRoot, action.relativeSuffix)
	if err != nil {
		return err
	}
	if rebuilt != action.destination {
		return compatibilityError("destination changed after preparation: " + action.destination)
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
			return compatibilityError("destination path could not be inspected: " + action.destination)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return compatibilityError("destination path contains a symlink: " + filepath.Join(action.allowedRoot, filepath.FromSlash(consumed)))
		}
		if index != len(components)-1 && !info.IsDir() {
			return compatibilityError("destination path component is not a directory: " + filepath.Join(action.allowedRoot, filepath.FromSlash(consumed)))
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

func validateCreatedInstallDirectories(planned, created map[string]struct{}) error {
	for directory := range planned {
		if _, exists := created[directory]; !exists {
			return compatibilityError("planned owned directory was not created: " + directory)
		}
	}
	for directory := range created {
		if _, exists := planned[directory]; !exists {
			return compatibilityError("unplanned owned directory was created: " + directory)
		}
	}
	return nil
}

func installPathSet(values []string) (map[string]struct{}, error) {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || !filepath.IsAbs(value) || !safeStateField(value) {
			return nil, compatibilityError("invalid planned owned directory: " + value)
		}
		if _, exists := result[value]; exists {
			return nil, compatibilityError("duplicate planned owned directory: " + value)
		}
		result[value] = struct{}{}
	}
	return result, nil
}
