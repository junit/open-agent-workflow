package dogfood

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
)

const maximumRecordBytes = 4 << 20

func createEvidenceRoot(value string) (string, error) {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", errors.New("evidence root must be a clean absolute path")
	}
	if _, err := os.Lstat(value); err == nil {
		return "", errors.New("evidence root must initially be absent")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect evidence root: %w", err)
	}
	parent := filepath.Dir(value)
	if err := ensureDirectoryTree(parent); err != nil {
		return "", fmt.Errorf("evidence parent: %w", err)
	}
	if err := os.Mkdir(value, 0o700); err != nil {
		return "", fmt.Errorf("create evidence root: %w", err)
	}
	created := true
	defer func() {
		if created {
			_ = os.Remove(value)
		}
	}()
	info, err := os.Lstat(value)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("created evidence root is not a regular directory")
	}
	physical, err := canonicalExistingDirectory(value)
	if err != nil {
		return "", fmt.Errorf("canonicalize evidence root: %w", err)
	}
	created = false
	return physical, nil
}

func ensureDirectoryTree(value string) error {
	ancestor := value
	for {
		info, err := os.Lstat(ancestor)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return errors.New("existing evidence parent is not a regular directory")
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return errors.New("evidence parent has no existing ancestor")
		}
		ancestor = next
	}
	if err := os.MkdirAll(value, 0o700); err != nil {
		return err
	}
	current := value
	for current != ancestor {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("evidence parent contains a non-directory or symlink")
		}
		current = filepath.Dir(current)
	}
	return nil
}

func canonicalExistingDirectory(value string) (string, error) {
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", errors.New("path must be a clean absolute directory")
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("path is not a directory")
	}
	physical, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", err
	}
	physical = filepath.Clean(physical)
	info, err = os.Stat(physical)
	if err != nil || !info.IsDir() {
		return "", errors.New("canonical path is not a directory")
	}
	return physical, nil
}

func containedRegularFile(value, root string) (string, error) {
	canonicalRoot, err := canonicalExistingDirectory(root)
	if err != nil {
		return "", fmt.Errorf("root: %w", err)
	}
	if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", errors.New("file path must be a clean absolute path")
	}
	if !containedPath(canonicalRoot, value) {
		return "", errors.New("file path escapes evidence root")
	}
	current := canonicalRoot
	relative, err := filepath.Rel(canonicalRoot, value)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("file path escapes evidence root")
	}
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, segment)
		entry, statErr := os.Lstat(current)
		if statErr != nil {
			return "", statErr
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symlinked evidence files are not accepted")
		}
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("evidence path is not a regular file")
	}
	return filepath.Clean(value), nil
}

func containedPath(root, value string) bool {
	relative, err := filepath.Rel(root, value)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func readLimited(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("record is not a regular file")
	}
	if info.Size() > maximumRecordBytes {
		return nil, errors.New("record is too large")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximumRecordBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maximumRecordBytes {
		return nil, errors.New("record is too large")
	}
	return raw, nil
}

func readCanonical(path string, destination any) ([]byte, error) {
	raw, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(raw) {
		return nil, errors.New("record is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return nil, fmt.Errorf("record JSON is invalid: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return nil, fmt.Errorf("record JSON has trailing content: %w", err)
	}
	normalized, err := canonicaljson.Marshal(destination)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, normalized) {
		return nil, errors.New("record JSON is not canonical")
	}
	return raw, nil
}

func writeCanonical(path string, value any) error {
	data, err := canonicaljson.Marshal(value)
	if err != nil {
		return err
	}
	return writeText(path, data, 0o600)
}

func writeText(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".oaw-write-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
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
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func printCanonical(value any) error {
	data, err := canonicaljson.Marshal(value)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func validText(value string, maximumRunes int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximumRunes && strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func sealPilot(value *pilotRecord) error {
	if value == nil {
		return errors.New("pilot record is required")
	}
	provided := value.Digest
	value.Digest = ""
	digest, _, err := canonicaljson.Digest(*value)
	if err != nil {
		return err
	}
	if provided != "" && provided != digest {
		return errors.New("pilot digest mismatch")
	}
	value.Digest = digest
	return nil
}

func loadPilot(evidencePath string) (pilotRecord, error) {
	entry, err := os.Lstat(evidencePath)
	if err != nil {
		return pilotRecord{}, fmt.Errorf("evidence root: %w", err)
	}
	if entry.Mode()&os.ModeSymlink != 0 || !entry.IsDir() {
		return pilotRecord{}, errors.New("evidence root must be a regular directory")
	}
	evidence, err := canonicalExistingDirectory(evidencePath)
	if err != nil {
		return pilotRecord{}, fmt.Errorf("evidence root: %w", err)
	}
	var pilot pilotRecord
	if _, err := readCanonical(filepath.Join(evidence, "pilot.json"), &pilot); err != nil {
		return pilotRecord{}, fmt.Errorf("pilot record: %w", err)
	}
	if pilot.EvidenceRoot != evidence || pilot.ConfigRoot != filepath.Join(evidence, "runtime", "config") || pilot.StateRoot != filepath.Join(evidence, "runtime", "state", "workflows") {
		return pilotRecord{}, errors.New("pilot paths do not match the evidence root")
	}
	if pilot.SchemaVersion != pilotSchema || !validText(pilot.WorkflowID, 512) || !validText(pilot.Profile, 512) || !validDigest(pilot.Digest) || !validDigest(pilot.HostSessionDigest) || !validDigest(pilot.EnvironmentReportDigest) || !validDigest(pilot.InventoryDigest) || !validDigest(pilot.ConfigurationDigest) || !validDigest(pilot.ResolutionDigest) || !validDigest(pilot.RegistryDigest) || !validDigest(pilot.BundleDigest) {
		return pilotRecord{}, errors.New("pilot record identity is invalid")
	}
	unsigned := pilot
	unsigned.Digest = ""
	digest, _, err := canonicaljson.Digest(unsigned)
	if err != nil || digest != pilot.Digest {
		return pilotRecord{}, errors.New("pilot record digest is invalid")
	}
	configInfo, err := os.Lstat(pilot.ConfigRoot)
	if err != nil || configInfo.Mode()&os.ModeSymlink != 0 || !configInfo.IsDir() {
		return pilotRecord{}, errors.New("pilot configuration root must be a regular directory")
	}
	canonicalConfig, err := canonicalExistingDirectory(pilot.ConfigRoot)
	if err != nil {
		return pilotRecord{}, fmt.Errorf("pilot configuration root: %w", err)
	}
	if canonicalConfig != pilot.ConfigRoot {
		return pilotRecord{}, errors.New("pilot configuration root contains a symlink")
	}
	stateInfo, err := os.Lstat(pilot.StateRoot)
	if err != nil || stateInfo.Mode()&os.ModeSymlink != 0 || !stateInfo.IsDir() {
		return pilotRecord{}, errors.New("pilot state root must be a regular directory")
	}
	canonicalState, err := canonicalExistingDirectory(pilot.StateRoot)
	if err != nil {
		return pilotRecord{}, fmt.Errorf("pilot state root: %w", err)
	}
	if canonicalState != pilot.StateRoot {
		return pilotRecord{}, errors.New("pilot state root contains a symlink")
	}
	if _, err := canonicalExistingDirectory(pilot.Repository.Root); err != nil {
		return pilotRecord{}, fmt.Errorf("pilot repository root: %w", err)
	}
	return pilot, nil
}
