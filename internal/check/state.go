package check

import (
	"fmt"
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

type installationState struct {
	version        string
	scope          string
	project        string
	policyPath     string
	policyChecksum string
	backupPath     string
	directories    []string
	targets        []targetRecord
}

func loadInstallationState(path string, coords coordinates) (installationState, bool, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return installationState{}, false, nil
	}
	if info.Size() > maximumStateBytes {
		return installationState{}, true, fmt.Errorf("state exceeds read limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return installationState{}, true, err
	}
	state, err := parseInstallationState(data)
	if err != nil {
		return installationState{}, true, err
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return installationState{}, true, err
	}
	return state, true, nil
}

func parseInstallationState(data []byte) (installationState, error) {
	var state installationState
	counts := make(map[string]int)
	lines := strings.Split(string(data), "\n")
	if len(lines) != 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		fields := strings.Split(line, "\t")
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
	seenDirectories := make(map[string]bool)
	for _, directory := range state.directories {
		if !filepath.IsAbs(directory) || seenDirectories[directory] {
			return installationState{}, fmt.Errorf("invalid owned directory")
		}
		seenDirectories[directory] = true
	}
	return state, nil
}

func validateTargetRecords(state installationState) error {
	seen := make(map[string]bool)
	position := 0
	shared := make(map[string]targetRecord)
	for _, record := range state.targets {
		candidate, found := findTarget(record.id)
		if !found || (state.scope == "user" && !candidate.User) || seen[record.id] {
			return fmt.Errorf("invalid target state")
		}
		currentPosition := targetPosition(record.id)
		if currentPosition <= position || !filepath.IsAbs(record.path) || !safeStateField(record.path) || record.mode != candidate.Ownership || !validChecksum(record.checksum) || (record.origin != "created-file" && record.origin != "existing-file") {
			return fmt.Errorf("invalid target record")
		}
		if previous, exists := shared[record.path]; exists && (previous.mode != record.mode || previous.checksum != record.checksum || previous.origin != record.origin) {
			return fmt.Errorf("invalid shared destination state")
		}
		shared[record.path] = record
		seen[record.id] = true
		position = currentPosition
	}
	return nil
}

func validateOwnedDirectories(state installationState, coords coordinates) error {
	for _, directory := range state.directories {
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
	if !strings.HasPrefix(candidate, prefix) || filepath.Clean(candidate) != candidate {
		return false
	}
	rebuilt, err := validatedDestinationPath(root, strings.TrimPrefix(candidate, prefix))
	return err == nil && rebuilt == candidate
}

func containedStrictly(root, candidate string) bool {
	relation, err := filepath.Rel(root, candidate)
	return err == nil && relation != "." && relation != ".." && !strings.HasPrefix(relation, ".."+string(filepath.Separator)) && !filepath.IsAbs(relation)
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
	return !strings.ContainsAny(value, "\t\r\n")
}

func validChecksum(value string) bool {
	return checksumPattern.MatchString(value)
}
