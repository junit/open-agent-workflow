package management

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maximumStateBytes = 4 << 20

var checksumPattern = regexp.MustCompile(`^[0-9]+:[0-9]+$`)

type targetRecord struct {
	id       string
	path     string
	mode     string
	checksum string
	origin   string
}

type policyFileRecord struct {
	path     string
	checksum string
}

type installationState struct {
	version        string
	scope          string
	project        string
	policyPath     string
	policyChecksum string
	policyFiles    []policyFileRecord
	backupPath     string
	directories    []string
	targets        []targetRecord
}

func loadInstallationState(path string, coords coordinates) (installationState, bool, error) {
	state, exists, err := readInstallationState(path)
	if err != nil || !exists {
		return state, exists, err
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return installationState{}, true, err
	}
	return state, true, nil
}

func readInstallationState(path string) (installationState, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return installationState{}, false, nil
	}
	if !info.Mode().IsRegular() {
		return installationState{}, false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return installationState{}, true, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return installationState{}, false, nil
	}
	if info.Size() > maximumStateBytes {
		return installationState{}, true, fmt.Errorf("state exceeds read limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, maximumStateBytes+1))
	if err != nil {
		return installationState{}, true, err
	}
	if len(data) > maximumStateBytes {
		return installationState{}, true, fmt.Errorf("state exceeds read limit")
	}
	state, err := parseInstallationState(data)
	if err != nil {
		return installationState{}, true, err
	}
	return state, true, nil
}

func parseInstallationState(data []byte) (installationState, error) {
	var state installationState
	counts := make(map[string]int)
	lines := strings.Split(string(data), "\n")
	if len(lines) != 0 {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		fields := strings.FieldsFunc(line, func(character rune) bool {
			return character == '\t'
		})
		if len(fields) == 0 {
			return installationState{}, fmt.Errorf("invalid state record")
		}
		kind := fields[0]
		counts[kind]++
		switch kind {
		case "format":
			if len(fields) != 2 || fields[1] != "1" {
				return installationState{}, fmt.Errorf("invalid state format")
			}
		case "version":
			if len(fields) != 2 || !safeStateField(fields[1]) || fields[1] == "" {
				return installationState{}, fmt.Errorf("invalid state version")
			}
			state.version = fields[1]
		case "scope":
			if len(fields) != 2 || (fields[1] != "user" && fields[1] != "project") {
				return installationState{}, fmt.Errorf("invalid state scope")
			}
			state.scope = fields[1]
		case "project":
			if len(fields) != 2 || fields[1] == "" || !safeStateField(fields[1]) {
				return installationState{}, fmt.Errorf("invalid project state")
			}
			state.project = fields[1]
		case "policy":
			if len(fields) != 3 || fields[1] == "" || !safeStateField(fields[1]) || !validChecksum(fields[2]) {
				return installationState{}, fmt.Errorf("invalid policy state")
			}
			state.policyPath, state.policyChecksum = fields[1], fields[2]
		case "policy-file":
			if len(fields) != 3 || fields[1] == "" || !safeStateField(fields[1]) || !validChecksum(fields[2]) {
				return installationState{}, fmt.Errorf("invalid Policy Set file state")
			}
			state.policyFiles = append(state.policyFiles, policyFileRecord{path: fields[1], checksum: fields[2]})
		case "backup":
			if len(fields) != 2 || fields[1] == "" || !safeStateField(fields[1]) {
				return installationState{}, fmt.Errorf("invalid backup state")
			}
			state.backupPath = fields[1]
		case "directory":
			if len(fields) != 2 || fields[1] == "" || !safeStateField(fields[1]) {
				return installationState{}, fmt.Errorf("invalid directory state")
			}
			state.directories = append(state.directories, fields[1])
		case "target":
			if len(fields) != 6 {
				return installationState{}, fmt.Errorf("invalid target state")
			}
			record := targetRecord{id: fields[1], path: fields[2], mode: fields[3], checksum: fields[4], origin: fields[5]}
			state.targets = append(state.targets, record)
		default:
			return installationState{}, fmt.Errorf("invalid state record type: %s", kind)
		}
	}
	if counts["format"] != 1 || counts["version"] != 1 || counts["scope"] != 1 || counts["policy"] != 1 || len(state.targets) < 1 {
		return installationState{}, fmt.Errorf("state is incomplete or duplicated")
	}
	if state.scope == "user" {
		if counts["project"] != 0 {
			return installationState{}, fmt.Errorf("user state contains project data")
		}
	} else if counts["project"] != 1 {
		return installationState{}, fmt.Errorf("project state is incomplete or duplicated")
	}
	if counts["backup"] > 1 || !filepath.IsAbs(state.policyPath) || (state.backupPath != "" && !filepath.IsAbs(state.backupPath)) || (state.project != "" && !filepath.IsAbs(state.project)) {
		return installationState{}, fmt.Errorf("invalid absolute state path")
	}
	if err := validateTargetRecords(state); err != nil {
		return installationState{}, err
	}
	if err := validatePolicyFileRecords(state.policyFiles); err != nil {
		return installationState{}, err
	}
	seenDirectories := make(map[string]bool)
	for _, directory := range state.directories {
		if !filepath.IsAbs(directory) || seenDirectories[directory] {
			return installationState{}, fmt.Errorf("invalid owned directory")
		}
		seenDirectories[directory] = true
	}
	return state, nil
}

func validatePolicyFileRecords(records []policyFileRecord) error {
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record.path == "" || !safeStateField(record.path) || !filepath.IsAbs(record.path) || !validChecksum(record.checksum) {
			return fmt.Errorf("invalid Policy Set file state")
		}
		key := filepath.Clean(record.path)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate Policy Set file state")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateManagedPolicySetFiles(state installationState, coords coordinates) error {
	if !coords.managedPolicySet {
		return nil
	}
	if len(state.policyFiles) == 0 {
		return compatibilityError("managed Policy Set state is missing")
	}
	if err := validateManagedPolicySetTree(state, coords); err != nil {
		return err
	}
	mainFound := false
	for _, record := range state.policyFiles {
		relative, err := filepath.Rel(coords.policyDir, record.path)
		if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) ||
			strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return compatibilityError("managed Policy Set file escapes its directory")
		}
		rebuilt, err := validatedDestinationPath(coords.policyDir, filepath.ToSlash(relative))
		if err != nil || rebuilt != record.path {
			return compatibilityError("managed Policy Set file path is invalid")
		}
		if coords.currentScope == "user" && userCustomProfilePath(coords, record.path) {
			return compatibilityError("managed Policy Set file claims user Custom Profile: " + record.path)
		}
		current, err := inspectInstallPath(record.path)
		if err != nil || current.kind != installPathRegular || checksumBytes(current.data) != record.checksum {
			return compatibilityError("managed Policy Set file has drifted: " + record.path)
		}
		if record.path == state.policyPath {
			mainFound = record.checksum == state.policyChecksum
		}
	}
	if !mainFound {
		return compatibilityError("managed Policy Set main file state is invalid")
	}
	return nil
}

func validateManagedPolicySetTree(state installationState, coords coordinates) error {
	expected := map[string]bool{filepath.Clean(coords.policyDir): true}
	for _, record := range state.policyFiles {
		current := filepath.Clean(record.path)
		expected[current] = false
		for parent := filepath.Dir(current); containedStrictly(coords.policyDir, parent); parent = filepath.Dir(parent) {
			expected[parent] = true
		}
	}
	err := filepath.WalkDir(coords.policyDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		cleanPath := filepath.Clean(path)
		if coords.currentScope == "user" && containedStrictly(coords.customProfilesDir, cleanPath) {
			builtin := filepath.Join(coords.customProfilesDir, "builtin")
			if cleanPath != builtin && !containedStrictly(builtin, cleanPath) {
				if entry.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		wantDirectory, exists := expected[cleanPath]
		if !exists || wantDirectory != entry.IsDir() {
			return fmt.Errorf("unexpected managed Policy Set entry: %s", path)
		}
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("invalid managed Policy Set entry: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		return compatibilityError("managed Policy Set tree has drifted: " + err.Error())
	}
	return nil
}

func validateTargetRecords(state installationState) error {
	seen := make(map[string]bool)
	position := 0
	for _, record := range state.targets {
		if !safeStateField(record.id) || !safeStateField(record.path) ||
			!safeStateField(record.mode) || !safeStateField(record.checksum) ||
			!safeStateField(record.origin) {
			return fmt.Errorf("target record cannot be serialized")
		}
		candidate, found := findTarget(record.id)
		if !found || (state.scope == "user" && !candidate.User) {
			return fmt.Errorf("invalid target state")
		}
		if record.path == "" || !filepath.IsAbs(record.path) {
			return fmt.Errorf("invalid target path")
		}
		if record.mode != candidate.Ownership {
			return fmt.Errorf("invalid target ownership")
		}
		if !validChecksum(record.checksum) {
			return fmt.Errorf("invalid target checksum")
		}
		if record.origin != "created-file" && record.origin != "existing-file" {
			return fmt.Errorf("invalid target ownership")
		}
		if seen[record.id] {
			return fmt.Errorf("duplicate target state: %s", record.id)
		}
		currentPosition := targetPosition(record.id)
		if currentPosition <= position {
			return fmt.Errorf("target state is not in registry order")
		}
		seen[record.id] = true
		position = currentPosition
	}
	shared := make(map[string]targetRecord)
	for _, record := range state.targets {
		if previous, exists := shared[record.path]; exists && (previous.mode != record.mode || previous.checksum != record.checksum || previous.origin != record.origin) {
			return fmt.Errorf("invalid shared destination state")
		}
		shared[record.path] = record
	}
	return nil
}

func validateOwnedDirectories(state installationState, coords coordinates) error {
	for _, directory := range state.directories {
		if coords.managedPolicySet && isManagedPolicySetDirectory(coords, state.project, directory) {
			continue
		}
		if directory == coords.configDir || directory == coords.stateDir || directory == coords.installations || directory == coords.projects {
			root := coords.environment.StateHome
			if directory == coords.configDir {
				root = coords.environment.ConfigHome
			}
			if !recordedCoordinateMatches(root, directory) {
				return fmt.Errorf("invalid owned directory")
			}
			continue
		}
		matched := false
		for _, record := range state.targets {
			if record.origin != "created-file" {
				continue
			}
			root := coords.environment.Home
			if state.scope == "project" {
				root = state.project
			} else if record.id == "opencode" {
				root = coords.environment.ConfigHome
			}
			if recordedCoordinateMatches(root, directory) && containedStrictly(directory, record.path) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("owned directory does not match an installed target")
		}
	}
	return nil
}

func recordedCoordinateMatches(root, candidate string) bool {
	prefix := root + string(filepath.Separator)
	if !strings.HasPrefix(candidate, prefix) {
		return false
	}
	rebuilt, err := validatedDestinationPath(root, strings.TrimPrefix(candidate, prefix))
	return err == nil && rebuilt == candidate
}

func containedStrictly(root, candidate string) bool {
	relation, err := filepath.Rel(root, candidate)
	return err == nil && relation != "." && relation != ".." && !strings.HasPrefix(relation, ".."+string(filepath.Separator)) && !filepath.IsAbs(relation)
}

func userCustomProfilePath(coords coordinates, candidate string) bool {
	candidate = filepath.Clean(candidate)
	customProfilesDir := filepath.Clean(coords.customProfilesDir)
	if candidate != customProfilesDir && !containedStrictly(customProfilesDir, candidate) {
		return false
	}
	builtin := filepath.Join(customProfilesDir, "builtin")
	return candidate != builtin && !containedStrictly(builtin, candidate)
}

func targetPosition(id string) int {
	for index, candidate := range targetRegistry {
		if candidate.ID == id {
			return index + 1
		}
	}
	return 0
}

func safeStateField(value string) bool {
	return !hasControl(value)
}

func validChecksum(value string) bool {
	return checksumPattern.MatchString(value)
}
