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
	records := make([]targetRecord, 0, 2)
	for _, candidate := range health.state.targets {
		if candidate.id == id {
			records = append(records, candidate)
		}
	}
	if len(records) == 0 {
		return health.untrackedStatus(id)
	}
	artifacts, err := targetArtifactsForScope(id, health.state.scope)
	if err != nil {
		return "", err
	}
	if health.state.legacy {
		if len(records) != 1 || records[0].artifact != routerArtifactID {
			return "invalid-state", nil
		}
		status, err := health.trackedArtifactStatus(records[0], artifacts[0])
		if err != nil || status != "clean" {
			return status, err
		}
		for _, artifact := range artifacts[1:] {
			status, err := health.untrackedArtifactStatus(id, artifact)
			if err != nil || status != "not-installed" {
				return status, err
			}
		}
		return "upgrade-required", nil
	}
	if len(records) != len(artifacts) {
		return "invalid-state", nil
	}
	for _, artifact := range artifacts {
		record, found := findTargetArtifactRecord(records, id, artifact.ID)
		if !found {
			return "invalid-state", nil
		}
		status, err := health.trackedArtifactStatus(record, artifact)
		if err != nil || status != "clean" {
			return status, err
		}
	}
	return "clean", nil
}

func (health installationHealth) trackedArtifactStatus(record targetRecord, artifact targetArtifact) (string, error) {
	expected, err := artifactDestination(
		health.coords, health.state.scope, health.state.project, record.id, artifact.ID,
	)
	if err != nil {
		return "", err
	}
	if record.path != expected || record.mode != artifact.Ownership {
		return "invalid-state", nil
	}
	switch record.mode {
	case "managed-block":
		status, actual := managedBlock(record.path)
		if status != "present" || actual != record.checksum {
			return "drift", nil
		}
	case "owned-file":
		current, err := inspectInstallPath(record.path)
		if err != nil || current.kind != installPathRegular || checksumBytes(current.data) != record.checksum {
			return "drift", nil
		}
	default:
		return "invalid-state", nil
	}
	return "clean", nil
}

func (health installationHealth) untrackedStatus(id string) (string, error) {
	artifacts, err := targetArtifactsForScope(id, health.coords.currentScope)
	if err != nil {
		return "", err
	}
	for _, artifact := range artifacts {
		status, err := health.untrackedArtifactStatus(id, artifact)
		if err != nil || status != "not-installed" {
			return status, err
		}
	}
	return "not-installed", nil
}

func (health installationHealth) untrackedArtifactStatus(id string, artifact targetArtifact) (string, error) {
	path, err := artifactDestination(
		health.coords, health.coords.currentScope, health.coords.currentProject, id, artifact.ID,
	)
	if err != nil {
		return "", err
	}
	switch artifact.Ownership {
	case "managed-block":
		status, _ := managedBlock(path)
		if status != "absent" {
			return "drift", nil
		}
	case "owned-file":
		current, inspectErr := inspectInstallPath(path)
		if inspectErr != nil {
			return "", inspectErr
		}
		if current.kind != installPathMissing {
			return "drift", nil
		}
	default:
		return "invalid-state", nil
	}
	return "not-installed", nil
}
