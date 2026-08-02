package check

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

func installationLines(environment Environment, resolved resolvedRequest) ([]string, error) {
	coords, err := initializeCoordinates(environment, resolved)
	if err != nil {
		return nil, err
	}
	health := installationHealth{stateStatus: "not-installed", coords: coords}
	state, exists, stateErr := loadInstallationState(coords.stateFile, coords)
	if exists {
		health.stateStatus = "invalid-state"
		if stateErr == nil && state.policyPath == coords.policyPath && state.scope == resolved.scope && ((resolved.scope == "user" && state.project == "") || (resolved.scope == "project" && state.project == resolved.projectRoot)) {
			health.stateStatus = "valid"
			health.state = state
			if info, err := os.Stat(state.policyPath); err == nil && info.Mode().IsRegular() {
				actual, checksumErr := checksumFile(state.policyPath)
				health.policyClean = checksumErr == nil && actual == state.policyChecksum
			}
		}
	}
	lines := make([]string, 0, len(resolved.targets))
	for _, id := range resolved.targets {
		status, err := health.targetStatus(id)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fmt.Sprintf("installed %s: %s", id, status))
	}
	return lines, nil
}

func (health installationHealth) targetStatus(id string) (string, error) {
	if health.stateStatus == "invalid-state" {
		return "invalid-state", nil
	}
	if health.stateStatus == "not-installed" {
		return health.untrackedStatus(id)
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
	if !health.policyClean {
		return "drift", nil
	}
	switch record.mode {
	case "managed-block":
		status, block := managedBlock(record.path)
		if status != "present" || checksumBytes(block) != record.checksum {
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
