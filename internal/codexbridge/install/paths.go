package install

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
	"unicode/utf8"
)

type Environment struct {
	StateRoot   string
	DataRoot    string
	StateFile   string
	CodexBinary string
	ProjectRoot string
}

func NewEnvironment(stateHome, dataHome, codexBinary, projectRoot string) (Environment, error) {
	if !isAbsoluteCleanPath(stateHome) || !isAbsoluteCleanPath(dataHome) || !isAbsoluteCleanPath(projectRoot) {
		return Environment{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge environment roots must be absolute and clean", nil)
	}
	if !validExecutableCoordinate(codexBinary) {
		return Environment{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex executable coordinate is invalid", nil)
	}
	stateRoot := filepath.Join(stateHome, "open-agent-workflow", "codex-bridge")
	environment := Environment{
		StateRoot:   stateRoot,
		DataRoot:    filepath.Join(dataHome, "open-agent-workflow", "codex-bridge"),
		StateFile:   filepath.Join(stateRoot, "install.json"),
		CodexBinary: codexBinary,
		ProjectRoot: projectRoot,
	}
	if err := ValidateEnvironment(environment); err != nil {
		return Environment{}, err
	}
	return environment, nil
}

func ValidateEnvironment(environment Environment) error {
	if !isAbsoluteCleanPath(environment.StateRoot) || !isAbsoluteCleanPath(environment.DataRoot) || !isAbsoluteCleanPath(environment.ProjectRoot) {
		return installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge environment roots must be absolute and clean", nil)
	}
	if environment.StateFile != filepath.Join(environment.StateRoot, "install.json") {
		return installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge state file is outside its owned root", nil)
	}
	if !validExecutableCoordinate(environment.CodexBinary) {
		return installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex executable coordinate is invalid", nil)
	}
	return nil
}

func validExecutableCoordinate(value string) bool {
	if value == "" || !utf8.ValidString(value) || hasControl(value) || strings.ContainsAny(value, " \t") {
		return false
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value) == value
	}
	return filepath.Base(value) == value && value != "." && value != ".."
}

func managedDataPath(environment Environment, relative string) (string, error) {
	if err := ValidateEnvironment(environment); err != nil {
		return "", err
	}
	if !validOwnedFilePath(relative) {
		return "", installError("BRIDGE_INSTALL_INPUT_INVALID", "managed Bridge path is invalid", nil)
	}
	resolved := filepath.Join(environment.DataRoot, filepath.FromSlash(relative))
	contained, err := filepath.Rel(environment.DataRoot, resolved)
	if err != nil || contained == ".." || strings.HasPrefix(contained, ".."+string(filepath.Separator)) {
		return "", installError("BRIDGE_INSTALL_INPUT_INVALID", "managed Bridge path escapes its owned root", err)
	}
	return resolved, nil
}

func WriteState(environment Environment, state InstallState) error {
	return writeState(environment, state, "")
}

func ReplaceState(environment Environment, state InstallState, expectedDigest string) error {
	if !validSHA256(expectedDigest) {
		return invalidState("expected install state digest is invalid", nil)
	}
	return writeState(environment, state, expectedDigest)
}

func writeState(environment Environment, state InstallState, expectedDigest string) error {
	if err := validateStateBinding(environment, state); err != nil {
		return err
	}
	encoded, err := EncodeState(state)
	if err != nil {
		return err
	}
	root, err := openOrCreateManagedRoot(environment.StateRoot)
	if err != nil {
		return err
	}
	defer root.Close()

	var previous fs.FileInfo
	if info, inspectErr := root.Lstat("install.json"); inspectErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return unsafePath("Bridge state destination is not a regular owned file", nil)
		}
		if expectedDigest == "" {
			return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state already exists", nil)
		}
		digest, err := stateDigestFromRoot(root, info)
		if err != nil {
			return err
		}
		if digest != expectedDigest {
			return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state changed before replacement", nil)
		}
		previous = info
	} else if expectedDigest != "" && errors.Is(inspectErr, fs.ErrNotExist) {
		return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state disappeared before replacement", nil)
	} else if !errors.Is(inspectErr, fs.ErrNotExist) {
		return unsafePath("inspect Bridge state destination", inspectErr)
	}

	temporaryName, temporary, err := createRootTemporary(root)
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
	if _, err := temporary.Write(encoded); err != nil {
		return installError("BRIDGE_INSTALL_IO", "write temporary Bridge state", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		return installError("BRIDGE_INSTALL_IO", "set Bridge state permissions", err)
	}
	if err := temporary.Sync(); err != nil {
		return installError("BRIDGE_INSTALL_IO", "sync temporary Bridge state", err)
	}
	if err := temporary.Close(); err != nil {
		return installError("BRIDGE_INSTALL_IO", "close temporary Bridge state", err)
	}
	current, inspectErr := root.Lstat("install.json")
	if expectedDigest == "" {
		if inspectErr == nil || !errors.Is(inspectErr, fs.ErrNotExist) {
			return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state appeared before creation", inspectErr)
		}
	} else if inspectErr != nil || !os.SameFile(previous, current) {
		return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state changed before replacement", inspectErr)
	} else if digest, err := stateDigestFromRoot(root, current); err != nil || digest != expectedDigest {
		return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state content changed before replacement", err)
	}
	if err := root.Rename(temporaryName, "install.json"); err != nil {
		return installError("BRIDGE_INSTALL_IO", "publish Bridge state", err)
	}
	keepTemporary = false
	syncRoot(root)
	return nil
}

func stateDigestFromRoot(root *os.Root, info fs.FileInfo) (string, error) {
	if info.Mode().Perm() != 0o600 || info.Size() < 1 || info.Size() > maximumInstallStateBytes {
		return "", unsafePath("Bridge state file is unsafe", nil)
	}
	file, err := root.Open("install.json")
	if err != nil {
		return "", installError("BRIDGE_INSTALL_IO", "open Bridge state", err)
	}
	defer file.Close()
	opened, statErr := file.Stat()
	content, readErr := io.ReadAll(io.LimitReader(file, maximumInstallStateBytes+1))
	if statErr != nil || readErr != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return "", unsafePath("Bridge state changed while opening", errors.Join(statErr, readErr))
	}
	state, err := DecodeState(content)
	if err != nil {
		return "", err
	}
	return state.Digest, nil
}

func ReadState(environment Environment) (InstallState, error) {
	if err := ValidateEnvironment(environment); err != nil {
		return InstallState{}, err
	}
	root, err := openExistingManagedRoot(environment.StateRoot)
	if err != nil {
		return InstallState{}, err
	}
	defer root.Close()
	info, err := root.Lstat("install.json")
	if errors.Is(err, fs.ErrNotExist) {
		return InstallState{}, installError("BRIDGE_INSTALL_NOT_INSTALLED", "Codex Bridge is not installed", nil)
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maximumInstallStateBytes {
		return InstallState{}, unsafePath("Bridge state file is unsafe", err)
	}
	if info.Mode().Perm() != 0o600 {
		return InstallState{}, unsafePath("Bridge state file permissions are unsafe", nil)
	}
	file, err := root.Open("install.json")
	if err != nil {
		return InstallState{}, installError("BRIDGE_INSTALL_IO", "open Bridge state", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return InstallState{}, unsafePath("Bridge state changed while opening", err)
	}
	content, err := io.ReadAll(io.LimitReader(file, maximumInstallStateBytes+1))
	if err != nil {
		return InstallState{}, installError("BRIDGE_INSTALL_IO", "read Bridge state", err)
	}
	state, err := DecodeState(content)
	if err != nil {
		return InstallState{}, err
	}
	if err := validateStateBinding(environment, state); err != nil {
		return InstallState{}, err
	}
	return state, nil
}

func RemoveState(environment Environment, expectedDigest string) error {
	if err := ValidateEnvironment(environment); err != nil {
		return err
	}
	if !validSHA256(expectedDigest) {
		return invalidState("expected install state digest is invalid", nil)
	}
	root, err := openExistingManagedRoot(environment.StateRoot)
	if Code(err) == "BRIDGE_INSTALL_NOT_INSTALLED" {
		return nil
	}
	if err != nil {
		return err
	}
	info, err := root.Lstat("install.json")
	if errors.Is(err, fs.ErrNotExist) {
		root.Close()
		return nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		root.Close()
		return unsafePath("Bridge state file is unsafe", err)
	}
	file, err := root.Open("install.json")
	if err != nil {
		root.Close()
		return installError("BRIDGE_INSTALL_IO", "open Bridge state before removal", err)
	}
	opened, statErr := file.Stat()
	content, readErr := io.ReadAll(io.LimitReader(file, maximumInstallStateBytes+1))
	file.Close()
	if statErr != nil || readErr != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		root.Close()
		return unsafePath("Bridge state changed before removal", errors.Join(statErr, readErr))
	}
	state, decodeErr := DecodeState(content)
	if decodeErr != nil || state.Digest != expectedDigest {
		root.Close()
		return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state changed before removal", decodeErr)
	}
	current, inspectErr := root.Lstat("install.json")
	if inspectErr != nil || !os.SameFile(info, current) {
		root.Close()
		return installError("BRIDGE_INSTALL_STATE_CONFLICT", "Bridge state changed before removal", inspectErr)
	}
	if err := root.Remove("install.json"); err != nil {
		root.Close()
		return installError("BRIDGE_INSTALL_IO", "remove Bridge state", err)
	}
	syncRoot(root)
	root.Close()
	removeEmptyManagedRoot(environment.StateRoot)
	return nil
}

func removeEmptyManagedRoot(rootPath string) {
	base, relative, err := managedRootCoordinates(rootPath)
	if err != nil {
		return
	}
	baseRoot, err := openInspectedRoot(base)
	if err != nil {
		return
	}
	defer baseRoot.Close()
	_ = baseRoot.Remove(relative)
	_ = baseRoot.Remove("open-agent-workflow")
}

func validateStateBinding(environment Environment, state InstallState) error {
	if err := ValidateEnvironment(environment); err != nil {
		return err
	}
	if state.BinaryPath != filepath.Join(environment.DataRoot, "bin", "oaw") ||
		state.MarketplacePath != filepath.Join(environment.DataRoot, "marketplace") {
		return invalidState("install state coordinates are outside the OAW data root", nil)
	}
	return nil
}

func openOrCreateManagedRoot(rootPath string) (*os.Root, error) {
	base, relative, err := managedRootCoordinates(rootPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, installError("BRIDGE_INSTALL_IO", "create Bridge XDG root", err)
	}
	baseRoot, err := openInspectedRoot(base)
	if err != nil {
		return nil, err
	}
	defer baseRoot.Close()
	consumed := ""
	for _, component := range strings.Split(relative, "/") {
		consumed = path.Join(consumed, component)
		info, inspectErr := baseRoot.Lstat(consumed)
		if errors.Is(inspectErr, fs.ErrNotExist) {
			if err := baseRoot.Mkdir(consumed, 0o700); err != nil {
				return nil, installError("BRIDGE_INSTALL_IO", "create managed Bridge directory", err)
			}
			info, inspectErr = baseRoot.Lstat(consumed)
		}
		if inspectErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, unsafePath("managed Bridge directory is unsafe", inspectErr)
		}
	}
	return openRelativeRoot(baseRoot, relative)
}

func openExistingManagedRoot(rootPath string) (*os.Root, error) {
	base, relative, err := managedRootCoordinates(rootPath)
	if err != nil {
		return nil, err
	}
	baseRoot, err := openInspectedRoot(base)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, installError("BRIDGE_INSTALL_NOT_INSTALLED", "Codex Bridge is not installed", nil)
	}
	if err != nil {
		return nil, err
	}
	defer baseRoot.Close()
	consumed := ""
	for _, component := range strings.Split(relative, "/") {
		consumed = path.Join(consumed, component)
		info, inspectErr := baseRoot.Lstat(consumed)
		if errors.Is(inspectErr, fs.ErrNotExist) {
			return nil, installError("BRIDGE_INSTALL_NOT_INSTALLED", "Codex Bridge is not installed", nil)
		}
		if inspectErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, unsafePath("managed Bridge directory is unsafe", inspectErr)
		}
	}
	return openRelativeRoot(baseRoot, relative)
}

func managedRootCoordinates(rootPath string) (string, string, error) {
	if !isAbsoluteCleanPath(rootPath) {
		return "", "", installError("BRIDGE_INSTALL_INPUT_INVALID", "managed Bridge root must be absolute and clean", nil)
	}
	base := filepath.Dir(filepath.Dir(rootPath))
	relative, err := filepath.Rel(base, rootPath)
	if err != nil || filepath.ToSlash(relative) != "open-agent-workflow/codex-bridge" {
		return "", "", installError("BRIDGE_INSTALL_INPUT_INVALID", "managed Bridge root has an invalid layout", err)
	}
	return base, filepath.ToSlash(relative), nil
}

func openInspectedRoot(rootPath string) (*os.Root, error) {
	info, err := os.Lstat(rootPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, unsafePath("Bridge XDG root is unsafe", nil)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, unsafePath("open Bridge XDG root", err)
	}
	opened, err := root.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
		root.Close()
		return nil, unsafePath("Bridge XDG root changed while opening", err)
	}
	return root, nil
}

func openRelativeRoot(base *os.Root, relative string) (*os.Root, error) {
	info, err := base.Lstat(relative)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, unsafePath("managed Bridge root is unsafe", err)
	}
	root, err := base.OpenRoot(relative)
	if err != nil {
		return nil, unsafePath("open managed Bridge root", err)
	}
	opened, err := root.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
		root.Close()
		return nil, unsafePath("managed Bridge root changed while opening", err)
	}
	return root, nil
}

func createRootTemporary(root *os.Root) (string, *os.File, error) {
	for range 100 {
		var random [8]byte
		if _, err := io.ReadFull(rand.Reader, random[:]); err != nil {
			return "", nil, installError("BRIDGE_INSTALL_IO", "generate Bridge state temporary name", err)
		}
		name := ".oaw-" + hex.EncodeToString(random[:])
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", nil, installError("BRIDGE_INSTALL_IO", "create Bridge state temporary file", err)
		}
	}
	return "", nil, installError("BRIDGE_INSTALL_IO", "reserve Bridge state temporary file", nil)
}

func syncRoot(root *os.Root) {
	directory, err := root.Open(".")
	if err != nil {
		return
	}
	defer directory.Close()
	_ = directory.Sync()
}

func unsafePath(message string, cause error) error {
	return installError("BRIDGE_INSTALL_PATH_UNSAFE", message, cause)
}
