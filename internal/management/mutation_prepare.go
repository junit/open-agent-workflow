package management

import (
	"bytes"
	"os"
	"path/filepath"
)

type mutationOperation uint8

const (
	mutationUpdate mutationOperation = iota + 1
	mutationUninstall
)

type mutationRequest struct {
	project string
	targets string
	dryRun  bool
	force   bool
}

type mutationPlan struct {
	operation        mutationOperation
	source           Source
	environment      Environment
	request          mutationRequest
	resolved         resolvedRequest
	coordinates      coordinates
	targetActions    []mutationAction
	policyAction     mutationAction
	policySetActions []mutationAction
	stateActions     []mutationAction
	directoryActions []directoryAction
	leadingLines     []string
	backup           backupPlan
	terminal         terminalMutation
	predicted        Result
}

type PreparedUpdate struct {
	plan mutationPlan
}

type PreparedUninstall struct {
	plan mutationPlan
}

type mutationPreparation struct {
	resolved    resolvedRequest
	coordinates coordinates
	backupPath  string
	policy      installPathSnapshot
	state       installationState
	stateExists bool
}

type directoryAction struct {
	destination    string
	allowedRoot    string
	relativeSuffix string
	before         installPathSnapshot
	namespace      bool
	identity       mutationPathIdentity
}

func prepareMutationInputs(environment Environment, request mutationRequest) (mutationPreparation, error) {
	resolved, err := resolve(CheckRequest{Project: request.project, Targets: request.targets})
	if err != nil {
		return mutationPreparation{}, err
	}
	coords, err := initializeCoordinates(environment, resolved)
	if err != nil {
		return mutationPreparation{}, err
	}
	backupPath := ""
	if request.force {
		backupPath, err = reserveMutationBackupPath(coords)
		if err != nil {
			return mutationPreparation{}, err
		}
	}
	state, exists, err := readInstallationState(coords.stateFile)
	if err != nil {
		return mutationPreparation{}, installError(err)
	}
	policySetCoords, policySetErr := initializePolicySetCoordinates(environment, resolved)
	if policySetErr != nil {
		return mutationPreparation{}, policySetErr
	}
	if !exists || state.policyPath == policySetCoords.policyPath {
		coords = policySetCoords
	}
	policy, err := inspectInstallPath(coords.policyPath)
	if err != nil {
		return mutationPreparation{}, err
	}
	return mutationPreparation{
		resolved: cloneResolvedRequest(resolved), coordinates: coords,
		backupPath: backupPath, policy: cloneInstallPathSnapshot(policy),
		state: cloneInstallationStateValue(state), stateExists: exists,
	}, nil
}

func verifyUntrackedMutationMarkers(coords coordinates, resolved resolvedRequest) error {
	for _, id := range resolved.targets {
		candidate, found := findTarget(id)
		if !found {
			return compatibilityError("unknown target '" + id + "'")
		}
		if candidate.Ownership != "managed-block" {
			continue
		}
		destination, err := targetDestination(coords, resolved.scope, resolved.projectRoot, id)
		if err != nil {
			return err
		}
		current, err := inspectInstallPath(destination)
		if err != nil {
			return err
		}
		status, _ := managedInstallStatus(current)
		if status != "absent" {
			return compatibilityError("untracked OAW markers already exist: " + id + " at " + destination)
		}
	}
	return nil
}

func validateCleanMutationState(state installationState, coords coordinates, resolved resolvedRequest, policy installPathSnapshot) error {
	if state.scope != resolved.scope {
		return compatibilityError("installed scope does not match")
	}
	if resolved.scope == "user" {
		if state.project != "" {
			return compatibilityError("installed project root does not match")
		}
	} else if state.project != resolved.projectRoot {
		return compatibilityError("installed project root does not match")
	}
	if state.policyPath != coords.policyPath {
		return compatibilityError("installed policy path does not match")
	}
	if policy.kind != installPathRegular {
		return compatibilityError("managed policy is missing")
	}
	if checksumBytes(policy.data) != state.policyChecksum {
		return compatibilityError("managed policy has drifted")
	}
	if err := validateManagedPolicySetFiles(state, coords); err != nil {
		return err
	}
	if err := validateLiveTargetRecords(state.targets, coords, state.scope, state.project); err != nil {
		return err
	}
	if err := validateOwnedDirectories(state, coords); err != nil {
		return compatibilityError(err.Error())
	}
	return nil
}

func selectedInstalledRecords(records []targetRecord, selected []string) ([]targetRecord, error) {
	result := make([]targetRecord, 0, len(selected))
	for _, id := range selected {
		record, found := findTargetRecord(records, id)
		if !found {
			return nil, compatibilityError("selected target is not installed: " + id)
		}
		result = append(result, record)
	}
	return result, nil
}

func newStateMutationAction(label string, data []byte, destination, root string) (mutationAction, error) {
	relative, err := stateActionRelativeSuffix(root, destination)
	if err != nil {
		return mutationAction{}, err
	}
	before, err := inspectInstallPath(destination)
	if err != nil {
		return mutationAction{}, err
	}
	return newMutationAction(mutationReplace, label, data, destination, 0o600, root, relative, before)
}

func predictMutationResult(plan mutationPlan) Result {
	if plan.operation == mutationUninstall {
		return predictUninstallResult(plan)
	}
	actions := make([]mutationAction, 0, len(plan.targetActions)+1+len(plan.policySetActions)+len(plan.stateActions))
	actions = append(actions, plan.targetActions...)
	actions = append(actions, plan.policyAction)
	actions = append(actions, plan.policySetActions...)
	actions = append(actions, plan.stateActions...)
	lines := make([]string, 0, len(actions))
	for _, action := range actions {
		switch action.effect {
		case mutationRetain:
			continue
		case mutationRemove:
			if action.before.kind != installPathMissing {
				lines = append(lines, "oaw: would-remove: "+action.destination)
			}
		case mutationReplace:
			if action.before.kind == installPathRegular && bytes.Equal(action.before.data, action.data) {
				lines = append(lines, "oaw: unchanged: "+action.label)
				continue
			}
			verb := "would-update"
			if action.before.kind == installPathMissing {
				verb = "would-create"
			}
			lines = append(lines, "oaw: "+verb+": "+action.destination)
		}
	}
	return prependBackupPrediction(plan, Result{Lines: lines})
}

func prependBackupPrediction(plan mutationPlan, result Result) Result {
	if !plan.backup.required {
		return result
	}
	result.Lines = append([]string{"oaw: would-backup: " + plan.backup.path}, result.Lines...)
	return result
}

func predictUninstallResult(plan mutationPlan) Result {
	lines := append([]string(nil), plan.leadingLines...)
	if plan.backup.required {
		lines = append(lines, "oaw: would-backup: "+plan.backup.path)
	}
	for _, action := range plan.targetActions {
		lines = append(lines, predictMutationAction(action)...)
	}
	lines = append(lines, predictDirectoryRemovalClass(plan, false)...)
	lines = append(lines, predictMutationAction(plan.policyAction)...)
	for _, action := range plan.policySetActions {
		lines = append(lines, predictMutationAction(action)...)
	}
	for _, action := range plan.stateActions {
		lines = append(lines, predictMutationAction(action)...)
	}
	lines = append(lines, predictDirectoryRemovalClass(plan, true)...)
	return Result{Lines: lines}
}

func predictMutationAction(action mutationAction) []string {
	switch action.effect {
	case mutationRetain, 0:
		return nil
	case mutationRemove:
		if action.before.kind == installPathMissing {
			return nil
		}
		return []string{"oaw: would-remove: " + action.destination}
	case mutationReplace:
		if action.before.kind == installPathRegular && bytes.Equal(action.before.data, action.data) {
			return []string{"oaw: unchanged: " + action.label}
		}
		verb := "would-update"
		if action.before.kind == installPathMissing {
			verb = "would-create"
		}
		return []string{"oaw: " + verb + ": " + action.destination}
	default:
		return nil
	}
}

func predictDirectoryRemovalClass(plan mutationPlan, namespace bool) []string {
	planned := make(map[string]struct{})
	actions := make([]mutationAction, 0, len(plan.targetActions)+1+len(plan.stateActions))
	actions = append(actions, plan.targetActions...)
	actions = append(actions, plan.policyAction)
	actions = append(actions, plan.policySetActions...)
	actions = append(actions, plan.stateActions...)
	for _, action := range actions {
		if action.effect == mutationRemove {
			planned[filepath.Clean(action.destination)] = struct{}{}
		}
	}
	lines := make([]string, 0)
	for _, action := range plan.directoryActions {
		if action.namespace != namespace || action.before.kind == installPathMissing {
			continue
		}
		entries, err := os.ReadDir(action.destination)
		if err != nil {
			continue
		}
		empty := true
		for _, entry := range entries {
			if _, removed := planned[filepath.Clean(filepath.Join(action.destination, entry.Name()))]; !removed {
				empty = false
				break
			}
		}
		if empty {
			lines = append(lines, "oaw: would-remove-directory: "+action.destination)
			planned[filepath.Clean(action.destination)] = struct{}{}
		} else {
			lines = append(lines, "oaw: unchanged-directory: "+action.destination)
		}
	}
	return lines
}

func cloneMutationPlan(plan mutationPlan) mutationPlan {
	plan.source = cloneSource(plan.source)
	plan.resolved = cloneResolvedRequest(plan.resolved)
	plan.targetActions = cloneMutationActions(plan.targetActions)
	plan.policyAction = cloneMutationAction(plan.policyAction)
	plan.policySetActions = cloneMutationActions(plan.policySetActions)
	plan.stateActions = cloneMutationActions(plan.stateActions)
	plan.directoryActions = cloneDirectoryActions(plan.directoryActions)
	plan.leadingLines = append([]string(nil), plan.leadingLines...)
	plan.backup = cloneBackupPlan(plan.backup)
	plan.predicted = cloneManagementResult(plan.predicted)
	return plan
}
