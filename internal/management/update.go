package management

import (
	"path/filepath"
)

func PrepareUpdate(source Source, environment Environment, request UpdateRequest) (PreparedUpdate, error) {
	source, err := validateSource(source)
	if err != nil {
		return PreparedUpdate{}, err
	}
	mutationRequest := mutationRequest{
		project: request.Project, targets: request.Targets,
		dryRun: request.DryRun, force: request.Force,
	}
	preparation, err := prepareMutationInputs(environment, mutationRequest)
	if err != nil {
		return PreparedUpdate{}, err
	}
	if !preparation.stateExists {
		if err := verifyUntrackedMutationMarkers(preparation.coordinates, preparation.resolved); err != nil {
			return PreparedUpdate{}, err
		}
		return PreparedUpdate{}, &Error{Status: 66, Message: "no installation state; run install first"}
	}
	force, err := prepareMutationForce(
		preparation.state, preparation.coordinates, preparation.resolved,
		preparation.policy, request.Force,
	)
	if err != nil {
		return PreparedUpdate{}, err
	}
	if force.manual != nil {
		plan, err := prepareManualRecoveryPlan(
			mutationUpdate, source, environment, mutationRequest,
			preparation.resolved, preparation.coordinates, preparation.policy,
			*force.manual, preparation.backupPath,
		)
		if err != nil {
			return PreparedUpdate{}, err
		}
		return PreparedUpdate{plan: plan}, nil
	}
	plan, err := prepareUpdatePlan(source, environment, mutationRequest, preparation, force)
	if err != nil {
		return PreparedUpdate{}, err
	}
	return PreparedUpdate{plan: plan}, nil
}

func prepareUpdatePlan(
	source Source,
	environment Environment,
	request mutationRequest,
	preparation mutationPreparation,
	force forcePreparation,
) (mutationPlan, error) {
	targetActions, records, err := prepareUpdateTargets(preparation, force)
	if err != nil {
		return mutationPlan{}, err
	}
	policyAction, policySetActions, policyFiles, policyChecksum, err := prepareUpdatePolicyActions(source, preparation)
	if err != nil {
		return mutationPlan{}, err
	}
	stateActions, err := prepareUpdateStateActions(
		source, environment, preparation, force, policyChecksum, policyFiles, records,
	)
	if err != nil {
		return mutationPlan{}, err
	}
	backupTargets := append(cloneMutationActions(targetActions), cloneMutationActions(policySetActions)...)
	backup, err := buildMutationBackupPlan(
		force.backupRequired, "update", preparation.resolved.scope, preparation.backupPath,
		policyAction, backupTargets, stateActions,
	)
	if err != nil {
		return mutationPlan{}, err
	}
	plan := mutationPlan{
		operation: mutationUpdate, source: source, environment: environment, request: request,
		resolved: cloneResolvedRequest(preparation.resolved), coordinates: preparation.coordinates,
		targetActions: cloneMutationActions(targetActions), policyAction: cloneMutationAction(policyAction),
		policySetActions: cloneMutationActions(policySetActions),
		stateActions:     cloneMutationActions(stateActions), backup: cloneBackupPlan(backup),
	}
	plan.predicted = predictMutationResult(plan)
	return cloneMutationPlan(plan), nil
}

func prepareUpdatePolicyActions(
	source Source,
	preparation mutationPreparation,
) (mutationAction, []mutationAction, []policyFileRecord, string, error) {
	currentByPath := make(map[string]policyFileRecord, len(preparation.state.policyFiles))
	for _, record := range preparation.state.policyFiles {
		currentByPath[filepath.Clean(record.path)] = record
	}
	var primary mutationAction
	extras := make([]mutationAction, 0, len(source.policySet)-1)
	records := make([]policyFileRecord, 0, len(source.policySet))
	wanted := make(map[string]struct{}, len(source.policySet))
	for _, file := range source.policySet {
		destination, root, suffix, err := policySetDestination(preparation.coordinates, preparation.resolved, file.Path)
		if err != nil {
			return mutationAction{}, nil, nil, "", err
		}
		current, err := inspectInstallPath(destination)
		if err != nil {
			return mutationAction{}, nil, nil, "", err
		}
		label := "policy/" + file.Path
		if file.Path == "POLICY.md" {
			label = "policy"
		}
		action, err := newMutationAction(
			mutationReplace, label, policySetFileContent(preparation.coordinates, file), destination, 0o644,
			root, suffix, current,
		)
		if err != nil {
			return mutationAction{}, nil, nil, "", err
		}
		wanted[filepath.Clean(destination)] = struct{}{}
		records = append(records, policyFileRecord{path: destination, checksum: checksumBytes(policySetFileContent(preparation.coordinates, file))})
		if file.Path == "POLICY.md" {
			primary = action
		} else {
			extras = append(extras, action)
		}
	}
	for path := range currentByPath {
		if _, keep := wanted[path]; keep {
			continue
		}
		root := preparation.resolved.projectRoot
		if preparation.resolved.scope == "user" {
			root = preparation.coordinates.environment.ConfigHome
		}
		relative, err := stateActionRelativeSuffix(root, path)
		if err != nil {
			return mutationAction{}, nil, nil, "", err
		}
		current, err := inspectInstallPath(path)
		if err != nil {
			return mutationAction{}, nil, nil, "", err
		}
		action, err := newMutationAction(
			mutationRemove, "policy/obsolete", nil, path, 0,
			root, relative, current,
		)
		if err != nil {
			return mutationAction{}, nil, nil, "", err
		}
		extras = append(extras, action)
	}
	if primary.destination == "" {
		return mutationAction{}, nil, nil, "", compatibilityError("managed Policy Set is missing POLICY.md")
	}
	return primary, extras, records, checksumBytes(primary.data), nil
}

func prepareUpdateTargets(preparation mutationPreparation, force forcePreparation) ([]mutationAction, []targetRecord, error) {
	selected, err := selectedInstalledRecords(preparation.state.targets, preparation.resolved.targets)
	if err != nil {
		return nil, nil, err
	}
	actions := make([]mutationAction, 0, len(selected))
	records := make([]targetRecord, 0, len(selected))
	for _, record := range selected {
		action, updated, err := prepareUpdateTarget(preparation, force, record)
		if err != nil {
			return nil, nil, err
		}
		actions, err = addMutationAction(actions, action)
		if err != nil {
			return nil, nil, err
		}
		records = append(records, updated)
	}
	merged, err := mergeInstallRecords(preparation.state.targets, records, preparation.resolved.scope)
	if err != nil {
		return nil, nil, err
	}
	return actions, merged, nil
}

func prepareUpdateTarget(
	preparation mutationPreparation,
	force forcePreparation,
	record targetRecord,
) (mutationAction, targetRecord, error) {
	candidate, _ := findTarget(record.id)
	current, err := inspectInstallPath(record.path)
	if err != nil {
		return mutationAction{}, targetRecord{}, err
	}
	renderCurrent := current
	if repaired, found := force.repaired[record.path]; found {
		renderCurrent = cloneInstallPathSnapshot(repaired)
	}
	root, suffix, err := targetInstallCoordinates(preparation.coordinates, preparation.resolved, record.id)
	if err != nil {
		return mutationAction{}, targetRecord{}, err
	}
	rendered, renderedChecksum, err := renderUpdatedTarget(candidate, record, preparation, renderCurrent)
	if err != nil {
		return mutationAction{}, targetRecord{}, err
	}
	action, err := newMutationAction(
		mutationReplace, record.id, rendered, record.path, 0o644,
		root, suffix, current,
	)
	if err != nil {
		return mutationAction{}, targetRecord{}, err
	}
	record.checksum = renderedChecksum
	return action, record, nil
}

func renderUpdatedTarget(
	candidate target,
	record targetRecord,
	preparation mutationPreparation,
	current installPathSnapshot,
) ([]byte, string, error) {
	switch candidate.Ownership {
	case "managed-block":
		block, err := renderManagedBlock(targetID(record.id), scope(preparation.resolved.scope), policyRouterReference(preparation.coordinates))
		if err != nil {
			return nil, "", err
		}
		rendered, err := renderManagedFile(current.data, block)
		return rendered, checksumBytes(block), err
	case "owned-file":
		rendered, err := renderTarget(targetID(record.id), scope(preparation.resolved.scope), policyRouterReference(preparation.coordinates))
		return rendered, checksumBytes(rendered), err
	default:
		return nil, "", compatibilityError("unknown target ownership mode: " + candidate.Ownership)
	}
}

func prepareUpdateStateActions(
	source Source,
	environment Environment,
	preparation mutationPreparation,
	force forcePreparation,
	policyChecksum string,
	policyFiles []policyFileRecord,
	records []targetRecord,
) ([]mutationAction, error) {
	current := cloneInstallationStateValue(preparation.state)
	current.targets = cloneTargetRecords(records)
	action, err := newUpdatedStateMutationAction(
		"state", current, source.version, policyChecksum, policyFiles, preparation.backupPath,
		force.backupRequired, preparation.coordinates.stateFile, environment.StateHome,
	)
	if err != nil {
		return nil, err
	}
	return []mutationAction{action}, nil
}

func newUpdatedStateMutationAction(
	label string,
	state installationState,
	version string,
	policyChecksum string,
	policyFiles []policyFileRecord,
	backupPath string,
	backupRequired bool,
	destination string,
	root string,
) (mutationAction, error) {
	updated := cloneInstallationStateValue(state)
	updated.version = version
	updated.policyChecksum = policyChecksum
	updated.policyFiles = append([]policyFileRecord(nil), policyFiles...)
	if backupRequired {
		updated.backupPath = backupPath
	}
	rendered, err := serializeInstallState(updated)
	if err != nil {
		return mutationAction{}, err
	}
	return newStateMutationAction(label, rendered, destination, root)
}
