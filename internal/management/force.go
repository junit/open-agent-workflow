package management

import (
	"bytes"
	"fmt"
)

type forcePreparation struct {
	backupRequired bool
	policyBaseline string
	repaired       map[string]installPathSnapshot
	manual         *manualRecovery
}

type manualRecovery struct {
	record  targetRecord
	current installPathSnapshot
}

func prepareMutationForce(
	state installationState,
	coords coordinates,
	resolved resolvedRequest,
	policy installPathSnapshot,
	force bool,
) (forcePreparation, error) {
	if !force {
		if err := validateCleanMutationState(state, coords, resolved, policy); err != nil {
			return forcePreparation{}, err
		}
		return forcePreparation{repaired: make(map[string]installPathSnapshot)}, nil
	}
	if state.scope != resolved.scope {
		return forcePreparation{}, compatibilityError("installed scope does not match")
	}
	if resolved.scope == "user" {
		if state.project != "" {
			return forcePreparation{}, compatibilityError("installed project root does not match")
		}
	} else if state.project != resolved.projectRoot {
		return forcePreparation{}, compatibilityError("installed project root does not match")
	}
	if state.policyPath != coords.policyPath {
		return forcePreparation{}, compatibilityError("installed policy path does not match")
	}
	if policy.kind != installPathRegular {
		return forcePreparation{}, compatibilityError("managed policy is missing")
	}
	result := forcePreparation{repaired: make(map[string]installPathSnapshot)}
	if checksumBytes(policy.data) != state.policyChecksum {
		result.backupRequired = true
		result.policyBaseline = state.policyChecksum
	}
	selectedPaths := make(map[string]struct{})
	for _, id := range resolved.targets {
		if record, found := findTargetRecord(state.targets, id); found {
			selectedPaths[record.path] = struct{}{}
		}
	}
	for _, record := range state.targets {
		if _, selected := selectedPaths[record.path]; !selected {
			if err := validateLiveTargetRecord(record, coords, state.scope, state.project); err != nil {
				return forcePreparation{}, err
			}
			continue
		}
		current, repaired, manual, changed, err := verifyForcedTargetRecord(record, coords, state)
		if err != nil {
			return forcePreparation{}, err
		}
		if manual {
			result.manual = &manualRecovery{record: record, current: current}
			return result, nil
		}
		if repaired.kind != 0 {
			result.repaired[record.path] = cloneInstallPathSnapshot(repaired)
		}
		if changed {
			result.backupRequired = true
		}
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return forcePreparation{}, compatibilityError(err.Error())
	}
	return result, nil
}

func verifyForcedTargetRecord(record targetRecord, coords coordinates, state installationState) (
	current installPathSnapshot,
	repaired installPathSnapshot,
	manual bool,
	changed bool,
	err error,
) {
	expected, err := targetDestination(coords, state.scope, state.project, record.id)
	if err != nil {
		return current, repaired, false, false, err
	}
	if record.path != expected {
		return current, repaired, false, false, compatibilityError(fmt.Sprintf("installed target path does not match: %s at %s", record.id, record.path))
	}
	candidate, _ := findTarget(record.id)
	if record.mode != candidate.Ownership {
		return current, repaired, false, false, compatibilityError(fmt.Sprintf("installed target ownership does not match: %s at %s", record.id, record.path))
	}
	current, err = inspectInstallPath(record.path)
	if err != nil {
		return current, repaired, false, false, err
	}
	if current.kind != installPathRegular {
		return current, repaired, false, false, compatibilityError(fmt.Sprintf("forced target has no recoverable file: %s at %s", record.id, record.path))
	}
	switch record.mode {
	case "managed-block":
		status, actual := managedInstallStatus(current)
		if status == "present" {
			return current, repaired, false, actual != record.checksum, nil
		}
		expectedBlock, renderErr := renderManagedBlock(targetID(record.id), scope(state.scope), state.policyPath)
		if renderErr != nil {
			return current, repaired, false, false, renderErr
		}
		if checksumBytes(expectedBlock) == record.checksum {
			if repairedBytes, ok := repairManagedMarkerStructure(current.data, expectedBlock); ok {
				repaired = cloneInstallPathSnapshot(current)
				repaired.data = repairedBytes
				return current, repaired, false, true, nil
			}
		}
		return current, repaired, true, false, nil
	case "owned-file":
		return current, repaired, false, checksumBytes(current.data) != record.checksum, nil
	default:
		return current, repaired, false, false, compatibilityError("unknown target ownership mode: " + record.mode)
	}
}

func repairManagedMarkerStructure(current, expected []byte) ([]byte, bool) {
	currentLines := managedLineSpans(current)
	expectedLines := managedLineSpans(expected)
	if len(expectedLines) < 2 {
		return nil, false
	}
	beginIndexes, endIndexes := markerIndexes(current, currentLines)
	bodyLines := len(expectedLines) - 2
	switch {
	case len(beginIndexes) == 0 && len(endIndexes) == 1:
		endIndex := endIndexes[0]
		startIndex := endIndex - bodyLines
		if startIndex < 0 {
			return nil, false
		}
		fragment := current[currentLines[startIndex].start:currentLines[endIndex].end]
		expectedFragment := expected[expectedLines[1].start:]
		if !bytes.Equal(fragment, expectedFragment) {
			return nil, false
		}
		result := make([]byte, 0, len(current)+len(beginMarker)+1)
		result = append(result, current[:currentLines[startIndex].start]...)
		result = append(result, beginMarker...)
		result = append(result, '\n')
		result = append(result, current[currentLines[startIndex].start:]...)
		return result, true
	case len(beginIndexes) == 1 && len(endIndexes) == 0:
		beginIndex := beginIndexes[0]
		fragmentEnd := beginIndex + bodyLines
		if fragmentEnd >= len(currentLines) {
			return nil, false
		}
		fragment := current[currentLines[beginIndex].start:currentLines[fragmentEnd].end]
		expectedFragment := expected[:expectedLines[len(expectedLines)-1].start]
		if !bytes.Equal(fragment, expectedFragment) {
			return nil, false
		}
		result := make([]byte, 0, len(current)+len(endMarker)+1)
		result = append(result, current[:currentLines[fragmentEnd].end]...)
		result = append(result, endMarker...)
		result = append(result, '\n')
		result = append(result, current[currentLines[fragmentEnd].end:]...)
		return result, true
	default:
		return nil, false
	}
}

func markerIndexes(data []byte, lines []managedLineSpan) ([]int, []int) {
	begins := make([]int, 0, 1)
	ends := make([]int, 0, 1)
	for index, line := range lines {
		switch string(data[line.start:line.contentEnd]) {
		case beginMarker:
			begins = append(begins, index)
		case endMarker:
			ends = append(ends, index)
		}
	}
	return begins, ends
}

func prepareManualRecoveryPlan(
	operation mutationOperation,
	source Source,
	environment Environment,
	request mutationRequest,
	resolved resolvedRequest,
	coords coordinates,
	policy installPathSnapshot,
	recovery manualRecovery,
	backupPath string,
) (mutationPlan, error) {
	operationName := "update"
	if operation == mutationUninstall {
		operationName = "uninstall"
	}
	plan := mutationPlan{
		operation: operation, source: source, environment: environment, request: request,
		resolved: cloneResolvedRequest(resolved), coordinates: coords,
		terminal: terminalMutation{status: 65, message: "manual recovery required; backup: " + backupPath},
		backup:   backupPlan{required: true, operation: operationName, scope: resolved.scope, path: backupPath},
	}
	targetRoot, targetSuffix, err := targetInstallCoordinates(coords, resolved, recovery.record.id)
	if err != nil {
		return mutationPlan{}, err
	}
	plan.backup.candidates, err = addBackupCandidate(
		plan.backup.candidates, recovery.record.path, targetRoot, targetSuffix,
		recovery.current, backupPath,
	)
	if err != nil {
		return mutationPlan{}, err
	}
	stateView, err := inspectInstallPath(coords.stateFile)
	if err != nil {
		return mutationPlan{}, err
	}
	stateSuffix, err := stateActionRelativeSuffix(environment.StateHome, coords.stateFile)
	if err != nil {
		return mutationPlan{}, err
	}
	plan.backup.candidates, err = addBackupCandidate(
		plan.backup.candidates, coords.stateFile, environment.StateHome, stateSuffix,
		stateView, backupPath,
	)
	if err != nil {
		return mutationPlan{}, err
	}
	if policy.kind == installPathRegular {
		plan.backup.candidates, err = addBackupCandidate(
			plan.backup.candidates, coords.policyPath, environment.ConfigHome,
			"open-agent-workflow/ENGINEERING.md", policy, backupPath,
		)
		if err != nil {
			return mutationPlan{}, err
		}
	}
	if _, err := renderBackupManifest(plan.backup); err != nil {
		return mutationPlan{}, err
	}
	plan.predicted = predictMutationResult(plan)
	return cloneMutationPlan(plan), nil
}
