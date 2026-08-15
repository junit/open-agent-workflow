package management

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

type forcePreparation struct {
	backupRequired bool
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
	if err := validateForcedMutationState(state, coords, resolved, policy); err != nil {
		return forcePreparation{}, err
	}
	result := forcePreparation{repaired: make(map[string]installPathSnapshot)}
	changed, err := policySetNeedsBackup(state, coords)
	if err != nil {
		return forcePreparation{}, err
	}
	result.backupRequired = changed
	if checksumBytes(policy.data) != state.policyChecksum {
		result.backupRequired = true
	}
	result, err = prepareForcedTargetRecords(result, state, coords, resolved)
	if err != nil {
		return forcePreparation{}, err
	}
	if result.manual != nil {
		return result, nil
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return forcePreparation{}, integrityError(err.Error())
	}
	return result, nil
}

func policySetNeedsBackup(state installationState, coords coordinates) (bool, error) {
	if err := validatePolicyFileRecords(state.policyFiles); err != nil {
		return false, integrityError(err.Error())
	}
	if len(state.policyFiles) == 0 {
		return false, integrityError("managed Policy Set state is missing")
	}
	if err := validatePolicySetTree(state, coords); err != nil {
		return false, err
	}
	changed := false
	for _, record := range state.policyFiles {
		relative, err := filepath.Rel(coords.policyDir, record.path)
		if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return false, integrityError("managed Policy Set file escapes its directory")
		}
		current, err := inspectInstallPath(record.path)
		if err != nil {
			return false, err
		}
		if current.kind != installPathRegular {
			return false, integrityError("managed Policy Set file has no recoverable content: " + record.path)
		}
		if checksumBytes(current.data) != record.checksum {
			changed = true
		}
	}
	return changed, nil
}

func validateForcedMutationState(
	state installationState,
	coords coordinates,
	resolved resolvedRequest,
	policy installPathSnapshot,
) error {
	if state.scope != resolved.scope {
		return integrityError("installed scope does not match")
	}
	if resolved.scope == "user" {
		if state.project != "" {
			return integrityError("installed project root does not match")
		}
	} else if state.project != resolved.projectRoot {
		return integrityError("installed project root does not match")
	}
	if state.policyPath != coords.policyPath {
		return integrityError("installed policy path does not match")
	}
	if policy.kind != installPathRegular {
		return integrityError("managed policy is missing")
	}
	return nil
}

func prepareForcedTargetRecords(
	result forcePreparation,
	state installationState,
	coords coordinates,
	resolved resolvedRequest,
) (forcePreparation, error) {
	selectedPaths := make(map[string]struct{})
	for _, id := range resolved.targets {
		if record, found := findTargetRecord(state.targets, id); found {
			selectedPaths[record.path] = struct{}{}
		}
	}
	for _, record := range state.targets {
		if _, selected := selectedPaths[record.path]; !selected {
			if err := validateLiveTargetRecord(record, coords, state.scope, state.project); err != nil {
				return result, err
			}
			continue
		}
		current, repaired, manual, changed, err := verifyForcedTargetRecord(record, coords, state)
		if err != nil {
			return result, err
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
		return current, repaired, false, false, integrityError(fmt.Sprintf("installed target path does not match: %s at %s", record.id, record.path))
	}
	candidate, _ := findTarget(record.id)
	if record.mode != candidate.Ownership {
		return current, repaired, false, false, integrityError(fmt.Sprintf("installed target ownership does not match: %s at %s", record.id, record.path))
	}
	current, err = inspectInstallPath(record.path)
	if err != nil {
		return current, repaired, false, false, err
	}
	if current.kind != installPathRegular {
		return current, repaired, false, false, integrityError(fmt.Sprintf("forced target has no recoverable file: %s at %s", record.id, record.path))
	}
	switch record.mode {
	case "managed-block":
		status, actual := managedInstallStatus(current)
		if status == "present" {
			return current, repaired, false, actual != record.checksum, nil
		}
		expectedBlock, renderErr := renderManagedBlock(targetID(record.id), scope(state.scope), policyRouterReference(coords))
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
		return current, repaired, false, false, integrityError("unknown target ownership mode: " + record.mode)
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
	backup, err := prepareManualRecoveryBackup(plan.backup, environment, resolved, coords, policy, recovery)
	if err != nil {
		return mutationPlan{}, err
	}
	plan.backup = backup
	plan.predicted = predictMutationResult(plan)
	return cloneMutationPlan(plan), nil
}

func prepareManualRecoveryBackup(
	backup backupPlan,
	environment Environment,
	resolved resolvedRequest,
	coords coordinates,
	policy installPathSnapshot,
	recovery manualRecovery,
) (backupPlan, error) {
	targetRoot, targetSuffix, err := targetInstallCoordinates(coords, resolved, recovery.record.id)
	if err != nil {
		return backupPlan{}, err
	}
	backup.candidates, err = addBackupCandidate(
		backup.candidates, recovery.record.path, targetRoot, targetSuffix,
		recovery.current, backup.path,
	)
	if err != nil {
		return backupPlan{}, err
	}
	stateView, err := inspectInstallPath(coords.stateFile)
	if err != nil {
		return backupPlan{}, err
	}
	stateSuffix, err := stateActionRelativeSuffix(environment.StateHome, coords.stateFile)
	if err != nil {
		return backupPlan{}, err
	}
	backup.candidates, err = addBackupCandidate(
		backup.candidates, coords.stateFile, environment.StateHome, stateSuffix,
		stateView, backup.path,
	)
	if err != nil {
		return backupPlan{}, err
	}
	if policy.kind == installPathRegular {
		policyRoot := environment.ConfigHome
		if resolved.scope == "project" {
			policyRoot = resolved.projectRoot
		}
		policySuffix, suffixErr := stateActionRelativeSuffix(policyRoot, coords.policyPath)
		if suffixErr != nil {
			return backupPlan{}, suffixErr
		}
		backup.candidates, err = addBackupCandidate(
			backup.candidates, coords.policyPath, policyRoot,
			policySuffix, policy, backup.path,
		)
		if err != nil {
			return backupPlan{}, err
		}
	}
	if _, err := renderBackupManifest(backup); err != nil {
		return backupPlan{}, err
	}
	return cloneBackupPlan(backup), nil
}
