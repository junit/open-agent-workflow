package management

import (
	"bytes"
	"fmt"
	"path/filepath"
)

func serializeInstallState(state installationState) ([]byte, error) {
	if state.version == "" || !safeStateField(state.version) {
		return nil, integrityError("version cannot be serialized")
	}
	switch state.scope {
	case "user":
		if state.project != "" {
			return nil, integrityError("user state cannot contain a project root")
		}
	case "project":
		if state.project == "" || !safeStateField(state.project) {
			return nil, integrityError("project root cannot be serialized")
		}
		if !filepath.IsAbs(state.project) {
			return nil, integrityError("invalid project root")
		}
	default:
		return nil, integrityError("invalid state scope")
	}
	if state.policyPath == "" || !safeStateField(state.policyPath) {
		return nil, integrityError("policy path cannot be serialized")
	}
	if !filepath.IsAbs(state.policyPath) {
		return nil, integrityError("invalid policy path")
	}
	if !validChecksum(state.policyChecksum) {
		return nil, integrityError("invalid policy checksum")
	}
	if err := validatePolicyFileRecords(state.policyFiles); err != nil {
		return nil, integrityError(err.Error())
	}
	if state.backupPath != "" {
		if !safeStateField(state.backupPath) {
			return nil, integrityError("backup path cannot be serialized")
		}
		if !filepath.IsAbs(state.backupPath) {
			return nil, integrityError("invalid backup path")
		}
	}

	seenDirectories := make(map[string]struct{}, len(state.directories))
	for _, directory := range state.directories {
		if directory == "" || !safeStateField(directory) {
			return nil, integrityError("directory record cannot be serialized")
		}
		if !filepath.IsAbs(directory) {
			return nil, integrityError("invalid owned directory")
		}
		if _, exists := seenDirectories[directory]; exists {
			return nil, integrityError("duplicate owned directory")
		}
		seenDirectories[directory] = struct{}{}
	}
	if len(state.targets) == 0 {
		return nil, integrityError("state has no target records")
	}
	if err := validateTargetRecords(state); err != nil {
		return nil, integrityError(err.Error())
	}

	var result bytes.Buffer
	result.WriteString("format\t1\n")
	fmt.Fprintf(&result, "version\t%s\n", state.version)
	fmt.Fprintf(&result, "scope\t%s\n", state.scope)
	if state.scope == "project" {
		fmt.Fprintf(&result, "project\t%s\n", state.project)
	}
	fmt.Fprintf(&result, "policy\t%s\t%s\n", state.policyPath, state.policyChecksum)
	for _, record := range state.policyFiles {
		fmt.Fprintf(&result, "policy-file\t%s\t%s\n", record.path, record.checksum)
	}
	if state.backupPath != "" {
		fmt.Fprintf(&result, "backup\t%s\n", state.backupPath)
	}
	for _, directory := range state.directories {
		fmt.Fprintf(&result, "directory\t%s\n", directory)
	}
	for _, record := range state.targets {
		fmt.Fprintf(&result, "target\t%s\t%s\t%s\t%s\t%s\n", record.id, record.path, record.mode, record.checksum, record.origin)
	}
	return result.Bytes(), nil
}
