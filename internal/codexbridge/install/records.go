package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const InstallStateSchemaV1 = "oaw.codex-bridge-install/v1"

const maximumInstallStateBytes = 1 << 20

type OwnedFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode"`
}

type InstallState struct {
	SchemaVersion   string      `json:"schema_version"`
	BridgeVersion   string      `json:"bridge_version"`
	BinaryPath      string      `json:"binary_path"`
	BinaryDigest    string      `json:"binary_digest"`
	MarketplacePath string      `json:"marketplace_path"`
	MarketplaceName string      `json:"marketplace_name"`
	PluginName      string      `json:"plugin_name"`
	Files           []OwnedFile `json:"files"`
	CodexPluginID   string      `json:"codex_plugin_id"`
	InstalledAt     string      `json:"installed_at"`
	Digest          string      `json:"digest"`
}

type InstallRequest struct {
	Binary  string
	Version string
	DryRun  bool
}

type UninstallRequest struct{}

func EncodeState(state InstallState) ([]byte, error) {
	candidate := cloneInstallState(state)
	providedDigest := candidate.Digest
	candidate.Digest = ""
	if err := validateInstallState(candidate, false); err != nil {
		return nil, err
	}
	digest, err := stateDigest(candidate)
	if err != nil {
		return nil, err
	}
	if providedDigest != "" && providedDigest != digest {
		return nil, invalidState("install state digest does not match its content", nil)
	}
	candidate.Digest = digest
	encoded, err := json.Marshal(candidate)
	if err != nil {
		return nil, invalidState("encode install state", err)
	}
	return append(encoded, '\n'), nil
}

func DecodeState(content []byte) (InstallState, error) {
	if len(content) == 0 || len(content) > maximumInstallStateBytes {
		return InstallState{}, invalidState("install state size is invalid", nil)
	}
	var state InstallState
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return InstallState{}, invalidState("decode install state", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return InstallState{}, invalidState("install state must contain one JSON value", err)
	}
	if err := validateInstallState(state, true); err != nil {
		return InstallState{}, err
	}
	providedDigest := state.Digest
	state.Digest = ""
	digest, err := stateDigest(state)
	if err != nil {
		return InstallState{}, err
	}
	if providedDigest != digest {
		return InstallState{}, invalidState("install state digest does not match its content", nil)
	}
	state.Digest = providedDigest
	return cloneInstallState(state), nil
}

func cloneInstallState(state InstallState) InstallState {
	cloned := state
	cloned.Files = append([]OwnedFile(nil), state.Files...)
	return cloned
}

func stateDigest(state InstallState) (string, error) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", invalidState("encode install state digest input", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validateInstallState(state InstallState, requireDigest bool) error {
	if state.SchemaVersion != InstallStateSchemaV1 {
		return invalidState("install state schema is unsupported", nil)
	}
	if !semverPattern.MatchString(state.BridgeVersion) {
		return invalidState("install state Bridge version is invalid", nil)
	}
	if !isAbsoluteCleanPath(state.BinaryPath) || !isAbsoluteCleanPath(state.MarketplacePath) {
		return invalidState("install state contains an unsafe absolute path", nil)
	}
	if !validSHA256(state.BinaryDigest) {
		return invalidState("install state binary digest is invalid", nil)
	}
	if state.MarketplaceName != MarketplaceName || state.PluginName != PluginName || state.CodexPluginID != PluginName+"@"+MarketplaceName {
		return invalidState("install state Bridge identity is invalid", nil)
	}
	if len(state.Files) == 0 {
		return invalidState("install state has no owned files", nil)
	}
	seen := make(map[string]struct{}, len(state.Files))
	for _, file := range state.Files {
		if !validOwnedFilePath(file.Path) || !validSHA256(file.Digest) || file.Mode == 0 || file.Mode&^0o777 != 0 {
			return invalidState("install state contains an invalid owned file", nil)
		}
		if _, duplicate := seen[file.Path]; duplicate {
			return invalidState("install state contains duplicate owned files", nil)
		}
		seen[file.Path] = struct{}{}
	}
	installedAt, err := time.Parse(time.RFC3339, state.InstalledAt)
	if err != nil || installedAt.Format(time.RFC3339) != state.InstalledAt {
		return invalidState("install state timestamp is invalid", nil)
	}
	if requireDigest && !validSHA256(state.Digest) {
		return invalidState("install state digest is invalid", nil)
	}
	if !requireDigest && state.Digest != "" {
		return invalidState("install state digest input must be empty", nil)
	}
	return nil
}

func validOwnedFilePath(value string) bool {
	return value != "" && utf8.ValidString(value) && !hasControl(value) &&
		!strings.Contains(value, `\`) && !path.IsAbs(value) && path.Clean(value) == value && value != "." &&
		value != ".." && !strings.HasPrefix(value, "../")
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func isAbsoluteCleanPath(value string) bool {
	return value != "" && utf8.ValidString(value) && !hasControl(value) && filepath.IsAbs(value) && filepath.Clean(value) == value
}

func hasControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}

func invalidState(message string, cause error) error {
	return installError("BRIDGE_INSTALL_STATE_INVALID", message, cause)
}
