package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

const ManagementResultSchemaV1 = "oaw.codex-bridge-management/v1"

type Diagnostic struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	PathDigest string `json:"path_digest,omitempty"`
}

type Result struct {
	SchemaVersion      string       `json:"schema_version"`
	Operation          string       `json:"operation"`
	Changed            bool         `json:"changed"`
	RequiresNewSession bool         `json:"requires_new_session"`
	Diagnostics        []Diagnostic `json:"diagnostics"`
}

type FileStatus struct {
	Path           string `json:"path"`
	PathDigest     string `json:"path_digest"`
	ExpectedDigest string `json:"expected_digest"`
	CurrentDigest  string `json:"current_digest,omitempty"`
	Mode           uint32 `json:"mode"`
	Status         string `json:"status"`
}

type MarketplaceStatus struct {
	Name       string `json:"name"`
	Registered bool   `json:"registered"`
	Root       string `json:"root,omitempty"`
	SourceType string `json:"source_type,omitempty"`
}

type PluginStatus struct {
	PluginID  string `json:"plugin_id"`
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	Version   string `json:"version,omitempty"`
}

type CheckResult struct {
	SchemaVersion        string            `json:"schema_version"`
	Files                []FileStatus      `json:"files"`
	CodexMarketplace     MarketplaceStatus `json:"codex_marketplace"`
	CodexPlugin          PluginStatus      `json:"codex_plugin"`
	CurrentSessionLoaded bool              `json:"current_session_loaded"`
	RequiresNewSession   bool              `json:"requires_new_session"`
	Diagnostics          []Diagnostic      `json:"diagnostics"`
}

func Check(ctx context.Context, environment Environment, runner CodexRunner) (CheckResult, error) {
	if err := ValidateEnvironment(environment); err != nil {
		return CheckResult{}, err
	}
	marketplaces, err := ListMarketplaces(ctx, runner)
	if err != nil {
		return CheckResult{}, err
	}
	plugins, err := ListPlugins(ctx, runner)
	if err != nil {
		return CheckResult{}, err
	}
	result := CheckResult{
		SchemaVersion:        ManagementResultSchemaV1,
		Files:                []FileStatus{},
		CodexMarketplace:     projectMarketplaceStatus(marketplaces),
		CodexPlugin:          projectPluginStatus(plugins),
		CurrentSessionLoaded: false,
		Diagnostics:          []Diagnostic{},
	}
	state, err := ReadState(environment)
	if Code(err) == "BRIDGE_INSTALL_NOT_INSTALLED" {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code: "BRIDGE_INSTALL_NOT_INSTALLED", Message: "OAW has no Codex Bridge installation state",
		})
		return result, nil
	}
	if err != nil {
		return CheckResult{}, err
	}
	result.Files, result.Diagnostics = inspectOwnedFiles(environment, state.Files)
	result.RequiresNewSession = result.CodexPlugin.Installed || result.CodexMarketplace.Registered
	if !result.CodexMarketplace.Registered || result.CodexMarketplace.SourceType != "local" ||
		filepath.Clean(result.CodexMarketplace.Root) != state.MarketplacePath ||
		!result.CodexPlugin.Installed || result.CodexPlugin.Version != state.BridgeVersion {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code: "BRIDGE_INSTALL_AUTHORITY_MISMATCH", Message: "Codex registration does not match OAW installation state",
		})
	}
	return result, nil
}

func projectMarketplaceStatus(records []MarketplaceRecord) MarketplaceStatus {
	status := MarketplaceStatus{Name: MarketplaceName}
	for _, record := range records {
		if record.Name != MarketplaceName {
			continue
		}
		if status.Registered {
			return MarketplaceStatus{Name: MarketplaceName}
		}
		status.Registered = true
		status.Root = record.Root
		status.SourceType = record.SourceType
	}
	return status
}

func projectPluginStatus(records []PluginRecord) PluginStatus {
	pluginID := PluginName + "@" + MarketplaceName
	status := PluginStatus{PluginID: pluginID}
	for _, record := range records {
		if record.PluginID != pluginID || record.Name != PluginName || record.MarketplaceName != MarketplaceName {
			continue
		}
		if status.Installed {
			return PluginStatus{PluginID: pluginID}
		}
		status.Installed = record.Installed
		status.Enabled = record.Enabled
		status.Version = record.Version
	}
	return status
}

func inspectOwnedFiles(environment Environment, files []OwnedFile) ([]FileStatus, []Diagnostic) {
	statuses := make([]FileStatus, 0, len(files))
	diagnostics := []Diagnostic{}
	root, err := openExistingManagedRoot(environment.DataRoot)
	if err != nil {
		for _, owned := range files {
			status := missingFileStatus(environment, owned)
			code := "BRIDGE_INSTALL_MISSING"
			message := "An OAW-owned Bridge file is missing"
			if Code(err) != "BRIDGE_INSTALL_NOT_INSTALLED" {
				status.Status = "unsafe"
				code = "BRIDGE_INSTALL_DRIFT"
				message = "The OAW Bridge data root cannot be verified"
			}
			statuses = append(statuses, status)
			diagnostics = append(diagnostics, fileDiagnostic(code, message, status.Path))
		}
		return statuses, diagnostics
	}
	defer root.Close()
	for _, owned := range files {
		absolute, pathErr := managedDataPath(environment, owned.Path)
		if pathErr != nil {
			status := FileStatus{Path: owned.Path, PathDigest: shortPathDigest(owned.Path), ExpectedDigest: owned.Digest, Mode: owned.Mode, Status: "unsafe"}
			statuses = append(statuses, status)
			diagnostics = append(diagnostics, fileDiagnostic("BRIDGE_INSTALL_DRIFT", "An OAW-owned Bridge file has an unsafe path", owned.Path))
			continue
		}
		status := FileStatus{Path: absolute, PathDigest: shortPathDigest(absolute), ExpectedDigest: owned.Digest, Mode: owned.Mode}
		currentDigest, currentMode, inspectErr := digestRootFile(root, owned.Path)
		switch {
		case errors.Is(inspectErr, fs.ErrNotExist):
			status.Status = "missing"
			diagnostics = append(diagnostics, fileDiagnostic("BRIDGE_INSTALL_MISSING", "An OAW-owned Bridge file is missing", absolute))
		case inspectErr != nil:
			status.Status = "unsafe"
			diagnostics = append(diagnostics, fileDiagnostic("BRIDGE_INSTALL_DRIFT", "An OAW-owned Bridge file cannot be verified", absolute))
		default:
			status.CurrentDigest = currentDigest
			if currentDigest != owned.Digest || currentMode != owned.Mode {
				status.Status = "drifted"
				diagnostics = append(diagnostics, fileDiagnostic("BRIDGE_INSTALL_DRIFT", "An OAW-owned Bridge file has changed", absolute))
			} else {
				status.Status = "clean"
			}
		}
		statuses = append(statuses, status)
	}
	slices.SortFunc(statuses, func(left, right FileStatus) int { return compareStrings(left.Path, right.Path) })
	return statuses, diagnostics
}

func missingFileStatus(environment Environment, owned OwnedFile) FileStatus {
	absolute, err := managedDataPath(environment, owned.Path)
	if err != nil {
		absolute = owned.Path
	}
	return FileStatus{
		Path: absolute, PathDigest: shortPathDigest(absolute), ExpectedDigest: owned.Digest,
		Mode: owned.Mode, Status: "missing",
	}
}

func digestRootFile(root *os.Root, relative string) (string, uint32, error) {
	info, err := root.Lstat(relative)
	if err != nil {
		return "", 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", 0, unsafePath("managed Bridge file is unsafe", nil)
	}
	if info.Size() < 0 || info.Size() > maximumBridgeBinaryBytes {
		return "", 0, unsafePath("managed Bridge file exceeds the verification limit", nil)
	}
	file, err := root.Open(relative)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return "", 0, unsafePath("managed Bridge file changed while opening", err)
	}
	digest := sha256.New()
	written, err := io.Copy(digest, io.LimitReader(file, maximumBridgeBinaryBytes+1))
	if err != nil {
		return "", 0, err
	}
	if written != info.Size() || written > maximumBridgeBinaryBytes {
		return "", 0, unsafePath("managed Bridge file changed while hashing", nil)
	}
	return hex.EncodeToString(digest.Sum(nil)), uint32(opened.Mode().Perm()), nil
}

func shortPathDigest(value string) string {
	digest := sha256.Sum256([]byte(filepath.Clean(value)))
	return hex.EncodeToString(digest[:6])
}

func fileDiagnostic(code, message, path string) Diagnostic {
	return Diagnostic{Code: code, Message: message, PathDigest: shortPathDigest(path)}
}

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
