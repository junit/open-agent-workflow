package management

import (
	"fmt"
	"os"
)

type installationHealth struct {
	stateStatus string
	policyClean bool
	state       installationState
	coords      coordinates
}

type installationResult struct {
	lines    []string
	trailing string
}

func installationLines(environment Environment, resolved resolvedRequest) (installationResult, error) {
	coords, err := initializeCoordinates(environment, resolved)
	if err != nil {
		return installationResult{}, err
	}
	health := installationHealth{stateStatus: "not-installed", coords: coords}
	state, exists, stateErr := readInstallationState(coords.stateFile)
	if !exists {
		if _, err := os.Stat(coords.policyPath); err == nil {
			health.stateStatus = "valid"
			health.policyClean = false
		}
	}
	if exists {
		health.stateStatus = "invalid-state"
		if stateErr == nil && validateOwnedDirectories(state, coords) == nil && state.policyPath == coords.policyPath && state.scope == resolved.scope && ((resolved.scope == "user" && state.project == "") || (resolved.scope == "project" && state.project == resolved.projectRoot)) {
			health.stateStatus = "valid"
			health.state = state
			health.policyClean = validatePolicySetFiles(state, coords) == nil
		}
	}
	lines := make([]string, 0, len(resolved.targets))
	for _, id := range resolved.targets {
		status, err := health.targetStatus(id)
		if err != nil {
			return installationResult{lines: lines, trailing: fmt.Sprintf("installed %s: ", id)}, err
		}
		lines = append(lines, fmt.Sprintf("installed %s: %s", id, status))
	}
	return installationResult{lines: lines}, nil
}

func (health installationHealth) targetStatus(id string) (string, error) {
	if health.stateStatus == "invalid-state" {
		return "invalid-state", nil
	}
	if health.stateStatus == "not-installed" {
		return health.untrackedStatus(id)
	}
	if !health.policyClean {
		return "drift", nil
	}
	var record targetRecord
	found := false
	for _, candidate := range health.state.targets {
		if candidate.id == id {
			record = candidate
			found = true
			break
		}
	}
	if !found {
		return health.untrackedStatus(id)
	}
	expected, err := targetDestination(health.coords, health.state.scope, health.state.project, id)
	if err != nil {
		return "", err
	}
	if record.path != expected {
		return "invalid-state", nil
	}
	switch record.mode {
	case "managed-block":
		status, actual := managedBlock(record.path)
		if status != "present" || actual != record.checksum {
			return "drift", nil
		}
	case "owned-file":
		info, err := os.Stat(record.path)
		if err != nil || !info.Mode().IsRegular() {
			return "drift", nil
		}
		actual, err := checksumFile(record.path)
		if err != nil || actual != record.checksum {
			return "drift", nil
		}
	default:
		return "invalid-state", nil
	}
	return "clean", nil
}

func (health installationHealth) untrackedStatus(id string) (string, error) {
	path, err := targetDestination(health.coords, health.coords.currentScope, health.coords.currentProject, id)
	if err != nil {
		return "", err
	}
	candidate, _ := findTarget(id)
	switch candidate.Ownership {
	case "managed-block":
		status, _ := managedBlock(path)
		if status == "absent" {
			return "not-installed", nil
		}
		return "drift", nil
	case "owned-file":
		if _, err := os.Stat(path); err == nil {
			return "drift", nil
		}
		return "not-installed", nil
	default:
		return "invalid-state", nil
	}
}
