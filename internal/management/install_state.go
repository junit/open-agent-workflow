package management

import (
	"bytes"
	"fmt"
	"path/filepath"
)

func serializeInstallState(state installationState) ([]byte, error) {
	if state.version == "" || !safeStateField(state.version) {
		return nil, compatibilityError("version cannot be serialized")
	}
	switch state.scope {
	case "user":
		if state.project != "" {
			return nil, compatibilityError("user state cannot contain a project root")
		}
	case "project":
		if state.project == "" || !safeStateField(state.project) {
			return nil, compatibilityError("project root cannot be serialized")
		}
		if !filepath.IsAbs(state.project) {
			return nil, compatibilityError("invalid project root")
		}
	default:
		return nil, compatibilityError("invalid state scope")
	}
	if state.policyPath == "" || !safeStateField(state.policyPath) {
		return nil, compatibilityError("policy path cannot be serialized")
	}
	if !filepath.IsAbs(state.policyPath) {
		return nil, compatibilityError("invalid policy path")
	}
	if !validChecksum(state.policyChecksum) {
		return nil, compatibilityError("invalid policy checksum")
	}
	if state.backupPath != "" {
		if !safeStateField(state.backupPath) {
			return nil, compatibilityError("backup path cannot be serialized")
		}
		if !filepath.IsAbs(state.backupPath) {
			return nil, compatibilityError("invalid backup path")
		}
	}

	seenDirectories := make(map[string]struct{}, len(state.directories))
	for _, directory := range state.directories {
		if directory == "" || !safeStateField(directory) {
			return nil, compatibilityError("directory record cannot be serialized")
		}
		if !filepath.IsAbs(directory) {
			return nil, compatibilityError("invalid owned directory")
		}
		if _, exists := seenDirectories[directory]; exists {
			return nil, compatibilityError("duplicate owned directory")
		}
		seenDirectories[directory] = struct{}{}
	}
	if len(state.targets) == 0 {
		return nil, compatibilityError("state has no target records")
	}
	if err := validateTargetRecords(state); err != nil {
		return nil, compatibilityError(err.Error())
	}

	var result bytes.Buffer
	result.WriteString("format\t1\n")
	fmt.Fprintf(&result, "version\t%s\n", state.version)
	fmt.Fprintf(&result, "scope\t%s\n", state.scope)
	if state.scope == "project" {
		fmt.Fprintf(&result, "project\t%s\n", state.project)
	}
	fmt.Fprintf(&result, "policy\t%s\t%s\n", state.policyPath, state.policyChecksum)
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
