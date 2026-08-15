package install

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const maximumBridgeBinaryBytes = 256 << 20

type payload struct {
	state InstallState
	files map[string]payloadFile
}

type payloadFile struct {
	content []byte
	mode    fs.FileMode
}

type stagedPayload struct {
	payload
	name string
}

func Install(ctx context.Context, environment Environment, runner CodexRunner, request InstallRequest) (Result, error) {
	if runner == nil {
		return Result{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex runner is required", nil)
	}
	if err := ensureNotInstalled(environment); err != nil {
		return Result{}, err
	}
	prepared, err := preparePayload(environment, request)
	if err != nil {
		return Result{}, err
	}
	result := operationResult("install", false, true)
	if request.DryRun {
		return result, nil
	}
	staged, err := stagePayload(environment, prepared, true)
	if err != nil {
		return Result{}, err
	}
	if err := publishFreshPayload(environment, staged); err != nil {
		cleanupStaging(environment, staged.name)
		return Result{}, err
	}
	if _, err := AddMarketplace(ctx, runner, prepared.state.MarketplacePath); err != nil {
		return rollbackInstall(ctx, environment, runner, prepared.state, false)
	}
	if _, err := AddPlugin(ctx, runner); err != nil {
		return rollbackInstall(ctx, environment, runner, prepared.state, true)
	}
	if err := WriteState(environment, prepared.state); err != nil {
		return rollbackInstall(ctx, environment, runner, prepared.state, true)
	}
	result.Changed = true
	return result, nil
}

func Update(ctx context.Context, environment Environment, runner CodexRunner, request InstallRequest) (Result, error) {
	if runner == nil {
		return Result{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex runner is required", nil)
	}
	previous, err := ReadState(environment)
	if err != nil {
		return Result{}, err
	}
	if err := requireCleanExactPayload(environment, previous); err != nil {
		return Result{}, err
	}
	if err := requireExactCodexRegistration(ctx, runner, previous); err != nil {
		return Result{}, err
	}
	prepared, err := preparePayload(environment, request)
	if err != nil {
		return Result{}, err
	}
	result := operationResult("update", false, true)
	if request.DryRun {
		return result, nil
	}
	staged, err := stagePayload(environment, prepared, false)
	if err != nil {
		return Result{}, err
	}
	backupName, err := swapPayloadWithBackup(environment, staged)
	if err != nil {
		cleanupStaging(environment, staged.name)
		return Result{}, err
	}
	if _, err := RemovePlugin(ctx, runner); err != nil {
		return rollbackUpdate(ctx, environment, runner, previous, prepared.state, backupName, false)
	}
	if _, err := AddPlugin(ctx, runner); err != nil {
		return rollbackUpdate(ctx, environment, runner, previous, prepared.state, backupName, true)
	}
	if err := ReplaceState(environment, prepared.state, previous.Digest); err != nil {
		return rollbackUpdate(ctx, environment, runner, previous, prepared.state, backupName, true)
	}
	if diagnostics := removeBackupPayload(environment, backupName, previous.Files); len(diagnostics) != 0 {
		return Result{}, installError("BRIDGE_INSTALL_CLEANUP_INCOMPLETE", "updated Bridge but could not remove the clean backup", nil)
	}
	result.Changed = true
	return result, nil
}

func Uninstall(ctx context.Context, environment Environment, runner CodexRunner, _ UninstallRequest) (Result, error) {
	if runner == nil {
		return Result{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Codex runner is required", nil)
	}
	state, err := ReadState(environment)
	if err != nil {
		return Result{}, err
	}
	plugins, err := ListPlugins(ctx, runner)
	if err != nil {
		return Result{}, err
	}
	marketplaces, err := ListMarketplaces(ctx, runner)
	if err != nil {
		return Result{}, err
	}
	cleanupFailed := false
	if projectPluginStatus(plugins).Installed {
		if _, err := RemovePlugin(ctx, runner); err != nil {
			cleanupFailed = true
		}
	}
	if projectMarketplaceStatus(marketplaces).Registered {
		if _, err := RemoveMarketplace(ctx, runner); err != nil {
			cleanupFailed = true
		}
	}
	if cleanupFailed {
		return Result{}, installError("BRIDGE_INSTALL_UNINSTALL_INCOMPLETE", "Codex registration cleanup is incomplete", nil)
	}

	remaining, diagnostics := removeOwnedPayload(environment, state.Files)
	result := operationResult("uninstall", true, true)
	result.Diagnostics = diagnostics
	if len(remaining) != 0 {
		previousDigest := state.Digest
		state.Files = remaining
		state.Digest = ""
		if err := ReplaceState(environment, state, previousDigest); err != nil {
			return Result{}, err
		}
		return result, nil
	}
	if err := RemoveState(environment, state.Digest); err != nil {
		return Result{}, err
	}
	return result, nil
}

func operationResult(operation string, changed, requiresNewSession bool) Result {
	return Result{
		SchemaVersion: ManagementResultSchemaV1, Operation: operation, Changed: changed,
		RequiresNewSession: requiresNewSession, Diagnostics: []Diagnostic{},
	}
}

func ensureNotInstalled(environment Environment) error {
	_, err := ReadState(environment)
	switch Code(err) {
	case "BRIDGE_INSTALL_NOT_INSTALLED":
		return nil
	case "":
		if err == nil {
			return installError("BRIDGE_INSTALL_ALREADY_INSTALLED", "Codex Bridge is already installed", nil)
		}
	}
	return err
}

func preparePayload(environment Environment, request InstallRequest) (payload, error) {
	if err := ValidateEnvironment(environment); err != nil {
		return payload{}, err
	}
	if !isAbsoluteCleanPath(request.Binary) || !semverPattern.MatchString(request.Version) {
		return payload{}, installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge binary or version is invalid", nil)
	}
	binary, binaryMode, binaryDigest, err := readSourceBinary(request.Binary)
	if err != nil {
		return payload{}, err
	}
	destinationBinary := filepath.Join(environment.DataRoot, "bin", "oaw-bridge")
	rendered, err := Render(RenderOptions{
		Binary: destinationBinary, Version: request.Version, Marketplace: MarketplaceName, Plugin: PluginName,
	})
	if err != nil {
		return payload{}, err
	}
	files := make(map[string]payloadFile, len(rendered)+1)
	files["bin/oaw-bridge"] = payloadFile{content: binary, mode: binaryMode}
	owned := []OwnedFile{{Path: "bin/oaw-bridge", Digest: binaryDigest, Mode: uint32(binaryMode)}}
	for relative, content := range rendered {
		target := path.Join("marketplace", relative)
		digest := sha256.Sum256(content)
		files[target] = payloadFile{content: slices.Clone(content), mode: 0o600}
		owned = append(owned, OwnedFile{Path: target, Digest: hex.EncodeToString(digest[:]), Mode: 0o600})
	}
	slices.SortFunc(owned, func(left, right OwnedFile) int { return compareStrings(left.Path, right.Path) })
	return payload{
		state: InstallState{
			SchemaVersion: InstallStateSchemaV1, BridgeVersion: request.Version,
			BinaryPath: destinationBinary, BinaryDigest: binaryDigest,
			MarketplacePath: filepath.Join(environment.DataRoot, "marketplace"),
			MarketplaceName: MarketplaceName, PluginName: PluginName, Files: owned,
			CodexPluginID: PluginName + "@" + MarketplaceName,
			InstalledAt:   time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		},
		files: files,
	}, nil
}

func readSourceBinary(binaryPath string) ([]byte, fs.FileMode, string, error) {
	info, err := os.Lstat(binaryPath)
	if err != nil {
		return nil, 0, "", installError("BRIDGE_INSTALL_INPUT_INVALID", "inspect Bridge binary", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || info.Size() > maximumBridgeBinaryBytes {
		return nil, 0, "", installError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge binary must be a bounded executable regular file", nil)
	}
	file, err := os.Open(binaryPath)
	if err != nil {
		return nil, 0, "", installError("BRIDGE_INSTALL_IO", "open Bridge binary", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, 0, "", installError("BRIDGE_INSTALL_PATH_UNSAFE", "Bridge binary changed while opening", err)
	}
	content, err := io.ReadAll(io.LimitReader(file, maximumBridgeBinaryBytes+1))
	if err != nil || int64(len(content)) != info.Size() {
		return nil, 0, "", installError("BRIDGE_INSTALL_IO", "read Bridge binary", err)
	}
	digest := sha256.Sum256(content)
	return content, 0o700, hex.EncodeToString(digest[:]), nil
}

func stagePayload(environment Environment, prepared payload, requireFresh bool) (stagedPayload, error) {
	root, err := openOrCreateManagedRoot(environment.DataRoot)
	if err != nil {
		return stagedPayload{}, err
	}
	defer root.Close()
	if requireFresh {
		if err := requireFreshDestinations(root); err != nil {
			return stagedPayload{}, err
		}
	}
	name, err := createRootDirectory(root, ".stage-")
	if err != nil {
		return stagedPayload{}, err
	}
	stageRoot, err := openRelativeRoot(root, name)
	if err != nil {
		_ = root.Remove(name)
		return stagedPayload{}, err
	}
	defer stageRoot.Close()
	for relative, file := range prepared.files {
		if err := writeNewRootFile(stageRoot, relative, file.content, file.mode); err != nil {
			stageRoot.Close()
			_ = root.RemoveAll(name)
			return stagedPayload{}, err
		}
	}
	return stagedPayload{payload: prepared, name: name}, nil
}

func requireFreshDestinations(root *os.Root) error {
	for _, name := range []string{"bin", "marketplace"} {
		_, err := root.Lstat(name)
		if err == nil {
			return installError("BRIDGE_INSTALL_ALREADY_INSTALLED", "managed Bridge destination already exists", nil)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return unsafePath("inspect managed Bridge destination", err)
		}
	}
	return nil
}

func publishFreshPayload(environment Environment, staged stagedPayload) error {
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := requireFreshDestinations(root); err != nil {
		return err
	}
	if err := root.Rename(path.Join(staged.name, "bin"), "bin"); err != nil {
		return installError("BRIDGE_INSTALL_IO", "publish managed Bridge binary", err)
	}
	if err := root.Rename(path.Join(staged.name, "marketplace"), "marketplace"); err != nil {
		_ = root.Rename("bin", path.Join(staged.name, "bin"))
		return installError("BRIDGE_INSTALL_IO", "publish managed Bridge marketplace", err)
	}
	_ = root.Remove(staged.name)
	syncRoot(root)
	return nil
}

func rollbackInstall(ctx context.Context, environment Environment, runner CodexRunner, state InstallState, pluginAttempted bool) (Result, error) {
	failed := false
	if pluginAttempted {
		if _, err := RemovePlugin(ctx, runner); err != nil {
			failed = true
		}
	}
	if _, err := RemoveMarketplace(ctx, runner); err != nil {
		failed = true
	}
	if failed {
		state.Digest = ""
		_ = WriteState(environment, state)
		return Result{}, installError("BRIDGE_INSTALL_ROLLBACK_INCOMPLETE", "Codex Bridge install rollback is incomplete", nil)
	}
	remaining, diagnostics := removeOwnedPayload(environment, state.Files)
	if len(remaining) != 0 || len(diagnostics) != 0 {
		state.Files = remaining
		state.Digest = ""
		_ = WriteState(environment, state)
		return Result{}, installError("BRIDGE_INSTALL_ROLLBACK_INCOMPLETE", "Codex Bridge file rollback is incomplete", nil)
	}
	return Result{}, installError("BRIDGE_INSTALL_ROLLBACK", "Codex Bridge install failed and was rolled back", nil)
}

func requireCleanExactPayload(environment Environment, state InstallState) error {
	statuses, _ := inspectOwnedFiles(environment, state.Files)
	for _, status := range statuses {
		if status.Status != "clean" {
			return installError("BRIDGE_INSTALL_DRIFT", "OAW-owned Bridge files have changed", nil)
		}
	}
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	actual, err := listPayloadFiles(root)
	if err != nil {
		return err
	}
	expected := make([]string, 0, len(state.Files))
	for _, file := range state.Files {
		expected = append(expected, file.Path)
	}
	slices.Sort(expected)
	if !slices.Equal(actual, expected) {
		return installError("BRIDGE_INSTALL_DRIFT", "managed Bridge payload contains unrecorded files", nil)
	}
	return nil
}

func requireExactCodexRegistration(ctx context.Context, runner CodexRunner, state InstallState) error {
	plugins, err := ListPlugins(ctx, runner)
	if err != nil {
		return err
	}
	marketplaces, err := ListMarketplaces(ctx, runner)
	if err != nil {
		return err
	}
	plugin := projectPluginStatus(plugins)
	marketplace := projectMarketplaceStatus(marketplaces)
	if !plugin.Installed || plugin.PluginID != state.CodexPluginID || !marketplace.Registered ||
		plugin.Version != state.BridgeVersion || marketplace.Name != state.MarketplaceName ||
		marketplace.SourceType != "local" || filepath.Clean(marketplace.Root) != state.MarketplacePath {
		return installError("BRIDGE_INSTALL_AUTHORITY_MISMATCH", "Codex registration does not match OAW installation state", nil)
	}
	return nil
}

func listPayloadFiles(root *os.Root) ([]string, error) {
	files := []string{}
	for _, top := range []string{"bin", "marketplace"} {
		if err := walkRootFiles(root, top, &files); err != nil {
			return nil, err
		}
	}
	slices.Sort(files)
	return files, nil
}

func walkRootFiles(root *os.Root, relative string, files *[]string) error {
	info, err := root.Lstat(relative)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return unsafePath("managed Bridge payload contains a symlink", nil)
	}
	if info.Mode().IsRegular() {
		*files = append(*files, relative)
		return nil
	}
	if !info.IsDir() {
		return unsafePath("managed Bridge payload contains a special file", nil)
	}
	directory, err := root.Open(relative)
	if err != nil {
		return err
	}
	defer directory.Close()
	opened, err := directory.Stat()
	if err != nil || !opened.IsDir() || !os.SameFile(info, opened) {
		return unsafePath("managed Bridge directory changed while opening", err)
	}
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := walkRootFiles(root, path.Join(relative, entry.Name()), files); err != nil {
			return err
		}
	}
	return nil
}

func swapPayloadWithBackup(environment Environment, staged stagedPayload) (string, error) {
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err != nil {
		return "", err
	}
	defer root.Close()
	backupName, err := createRootDirectory(root, ".backup-")
	if err != nil {
		return "", err
	}
	operations := [][2]string{
		{"bin", path.Join(backupName, "bin")},
		{"marketplace", path.Join(backupName, "marketplace")},
		{path.Join(staged.name, "bin"), "bin"},
		{path.Join(staged.name, "marketplace"), "marketplace"},
	}
	completed := 0
	for _, operation := range operations {
		if err := root.Rename(operation[0], operation[1]); err != nil {
			for index := completed - 1; index >= 0; index-- {
				_ = root.Rename(operations[index][1], operations[index][0])
			}
			_ = root.Remove(backupName)
			return "", installError("BRIDGE_INSTALL_IO", "swap managed Bridge payload", err)
		}
		completed++
	}
	_ = root.Remove(staged.name)
	syncRoot(root)
	return backupName, nil
}

func rollbackUpdate(ctx context.Context, environment Environment, runner CodexRunner, previous, current InstallState, backupName string, pluginRemoved bool) (Result, error) {
	_ = previous
	failed := false
	if pluginRemoved {
		if _, err := RemovePlugin(ctx, runner); err != nil {
			failed = true
		}
	}
	restored := restoreBackupPayload(environment, backupName, current.Files) == nil
	if !restored {
		failed = true
	}
	if restored {
		if _, err := AddPlugin(ctx, runner); err != nil {
			failed = true
		}
	}
	if failed {
		return Result{}, installError("BRIDGE_INSTALL_ROLLBACK_INCOMPLETE", "Codex Bridge update rollback is incomplete", nil)
	}
	return Result{}, installError("BRIDGE_INSTALL_ROLLBACK", "Codex Bridge update failed and was rolled back", nil)
}

func restoreBackupPayload(environment Environment, backupName string, currentFiles []OwnedFile) error {
	remaining, diagnostics := removeOwnedPayload(environment, currentFiles)
	if len(remaining) != 0 || len(diagnostics) != 0 {
		return installError("BRIDGE_INSTALL_DRIFT", "new Bridge payload changed during rollback", nil)
	}
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Rename(path.Join(backupName, "bin"), "bin"); err != nil {
		return err
	}
	if err := root.Rename(path.Join(backupName, "marketplace"), "marketplace"); err != nil {
		_ = root.Rename("bin", path.Join(backupName, "bin"))
		return err
	}
	_ = root.Remove(backupName)
	return nil
}

func removeBackupPayload(environment Environment, backupName string, files []OwnedFile) []Diagnostic {
	mapped := make([]OwnedFile, 0, len(files))
	for _, file := range files {
		copy := file
		copy.Path = path.Join(backupName, file.Path)
		mapped = append(mapped, copy)
	}
	_, diagnostics := removeOwnedPayload(environment, mapped)
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err == nil {
		defer root.Close()
		if _, inspectErr := root.Lstat(backupName); inspectErr == nil {
			diagnostics = append(diagnostics, fileDiagnostic("BRIDGE_INSTALL_DRIFT", "The previous Bridge backup contains unrecorded entries", backupName))
		}
	}
	return diagnostics
}

func removeOwnedPayload(environment Environment, files []OwnedFile) ([]OwnedFile, []Diagnostic) {
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err != nil {
		if Code(err) == "BRIDGE_INSTALL_NOT_INSTALLED" {
			return nil, nil
		}
		return slices.Clone(files), []Diagnostic{{Code: "BRIDGE_INSTALL_DRIFT", Message: "OAW data root cannot be verified"}}
	}
	remaining := []OwnedFile{}
	diagnostics := []Diagnostic{}
	for _, owned := range files {
		currentDigest, currentMode, inspectErr := digestRootFile(root, owned.Path)
		if errors.Is(inspectErr, fs.ErrNotExist) {
			continue
		}
		if inspectErr != nil || currentDigest != owned.Digest || currentMode != owned.Mode {
			remaining = append(remaining, owned)
			diagnostics = append(diagnostics, fileDiagnostic("BRIDGE_INSTALL_DRIFT", "An OAW-owned Bridge file has changed and was preserved", owned.Path))
			continue
		}
		if err := root.Remove(owned.Path); err != nil {
			remaining = append(remaining, owned)
			diagnostics = append(diagnostics, fileDiagnostic("BRIDGE_INSTALL_DRIFT", "An OAW-owned Bridge file could not be removed", owned.Path))
		}
	}
	removeEmptyOwnedDirectories(root, files)
	syncRoot(root)
	root.Close()
	removeEmptyManagedRoot(environment.DataRoot)
	return remaining, diagnostics
}

func removeEmptyOwnedDirectories(root *os.Root, files []OwnedFile) {
	directories := map[string]struct{}{}
	for _, file := range files {
		for directory := path.Dir(file.Path); directory != "."; directory = path.Dir(directory) {
			directories[directory] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(directories))
	for directory := range directories {
		ordered = append(ordered, directory)
	}
	slices.SortFunc(ordered, func(left, right string) int {
		leftDepth := strings.Count(left, "/")
		rightDepth := strings.Count(right, "/")
		if leftDepth != rightDepth {
			return rightDepth - leftDepth
		}
		return compareStrings(right, left)
	})
	for _, directory := range ordered {
		info, err := root.Lstat(directory)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		_ = root.Remove(directory)
	}
}

func createRootDirectory(root *os.Root, prefix string) (string, error) {
	for range 100 {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", installError("BRIDGE_INSTALL_IO", "generate managed directory name", err)
		}
		name := prefix + hex.EncodeToString(random[:])
		if err := root.Mkdir(name, 0o700); err == nil {
			return name, nil
		} else if !errors.Is(err, fs.ErrExist) {
			return "", installError("BRIDGE_INSTALL_IO", "create managed Bridge directory", err)
		}
	}
	return "", installError("BRIDGE_INSTALL_IO", "reserve managed Bridge directory", nil)
}

func writeNewRootFile(root *os.Root, relative string, content []byte, mode fs.FileMode) error {
	if !validOwnedFilePath(relative) {
		return installError("BRIDGE_INSTALL_INPUT_INVALID", "managed payload path is invalid", nil)
	}
	if err := ensureRootDirectories(root, path.Dir(relative)); err != nil {
		return err
	}
	file, err := root.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return installError("BRIDGE_INSTALL_IO", "create managed Bridge file", err)
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = root.Remove(relative)
		}
	}()
	if _, err := file.Write(content); err != nil {
		return installError("BRIDGE_INSTALL_IO", "write managed Bridge file", err)
	}
	if err := file.Chmod(mode); err != nil {
		return installError("BRIDGE_INSTALL_IO", "set managed Bridge file mode", err)
	}
	if err := file.Sync(); err != nil {
		return installError("BRIDGE_INSTALL_IO", "sync managed Bridge file", err)
	}
	if err := file.Close(); err != nil {
		return installError("BRIDGE_INSTALL_IO", "close managed Bridge file", err)
	}
	keep = true
	return nil
}

func ensureRootDirectories(root *os.Root, relative string) error {
	if relative == "." {
		return nil
	}
	consumed := ""
	for _, component := range strings.Split(relative, "/") {
		consumed = path.Join(consumed, component)
		info, err := root.Lstat(consumed)
		if errors.Is(err, fs.ErrNotExist) {
			if err := root.Mkdir(consumed, 0o700); err != nil {
				return installError("BRIDGE_INSTALL_IO", "create managed payload directory", err)
			}
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return unsafePath("managed payload directory is unsafe", err)
		}
	}
	return nil
}

func cleanupStaging(environment Environment, name string) {
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err != nil {
		return
	}
	defer root.Close()
	if strings.HasPrefix(name, ".stage-") {
		_ = root.RemoveAll(name)
	}
}
