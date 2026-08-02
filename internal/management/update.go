package management

import "fmt"

func PrepareUpdate(source Source, environment Environment, request UpdateRequest) (PreparedUpdate, error) {
	source, err := NewSource(source.version, source.policy)
	if err != nil {
		return PreparedUpdate{}, err
	}
	resolved, err := resolve(CheckRequest{Project: request.Project, Targets: request.Targets})
	if err != nil {
		return PreparedUpdate{}, err
	}
	coords, err := initializeCoordinates(environment, resolved)
	if err != nil {
		return PreparedUpdate{}, err
	}
	backupPath := ""
	if request.Force {
		backupPath, err = reserveMutationBackupPath(coords)
		if err != nil {
			return PreparedUpdate{}, err
		}
	}
	policyView, err := inspectInstallPath(coords.policyPath)
	if err != nil {
		return PreparedUpdate{}, err
	}
	state, exists, err := readInstallationState(coords.stateFile)
	if err != nil {
		return PreparedUpdate{}, installError(err)
	}
	if !exists {
		if err := verifyUntrackedMutationMarkers(coords, resolved); err != nil {
			return PreparedUpdate{}, err
		}
		return PreparedUpdate{}, &Error{Status: 66, Message: "no installation state; run install first"}
	}
	force, err := prepareMutationForce(state, coords, resolved, policyView, request.Force)
	if err != nil {
		return PreparedUpdate{}, err
	}
	if force.manual != nil {
		plan, err := prepareManualRecoveryPlan(
			mutationUpdate, source, environment,
			mutationRequest{project: request.Project, targets: request.Targets, dryRun: request.DryRun, force: request.Force},
			resolved, coords, policyView, *force.manual, backupPath,
		)
		if err != nil {
			return PreparedUpdate{}, err
		}
		return PreparedUpdate{plan: plan}, nil
	}
	selected, err := selectedInstalledRecords(state.targets, resolved.targets)
	if err != nil {
		return PreparedUpdate{}, err
	}

	targetActions := make([]mutationAction, 0, len(selected))
	selectedRendered := make([]targetRecord, 0, len(selected))
	for _, record := range selected {
		candidate, _ := findTarget(record.id)
		current, err := inspectInstallPath(record.path)
		if err != nil {
			return PreparedUpdate{}, err
		}
		renderCurrent := current
		if repaired, found := force.repaired[record.path]; found {
			renderCurrent = cloneInstallPathSnapshot(repaired)
		}
		allowedRoot, relativeSuffix, err := targetInstallCoordinates(coords, resolved, record.id)
		if err != nil {
			return PreparedUpdate{}, err
		}
		var rendered []byte
		var renderedChecksum string
		switch candidate.Ownership {
		case "managed-block":
			block, err := renderManagedBlock(targetID(record.id), scope(resolved.scope), coords.policyPath)
			if err != nil {
				return PreparedUpdate{}, err
			}
			rendered, err = renderManagedFile(renderCurrent.data, block)
			if err != nil {
				return PreparedUpdate{}, err
			}
			renderedChecksum = checksumBytes(block)
		case "owned-file":
			rendered, err = renderTarget(targetID(record.id), scope(resolved.scope), coords.policyPath)
			if err != nil {
				return PreparedUpdate{}, err
			}
			renderedChecksum = checksumBytes(rendered)
		default:
			return PreparedUpdate{}, compatibilityError("unknown target ownership mode: " + candidate.Ownership)
		}
		action, err := newMutationAction(
			mutationReplace, record.id, rendered, record.path, 0o644,
			allowedRoot, relativeSuffix, current,
		)
		if err != nil {
			return PreparedUpdate{}, err
		}
		targetActions, err = addMutationAction(targetActions, action)
		if err != nil {
			return PreparedUpdate{}, err
		}
		record.checksum = renderedChecksum
		selectedRendered = append(selectedRendered, record)
	}

	finalRecords, err := mergeInstallRecords(state.targets, selectedRendered, resolved.scope)
	if err != nil {
		return PreparedUpdate{}, err
	}
	references, err := collectPolicyStateReferencesWithBaseline(coords, coords.stateFile, policyView, force.policyBaseline)
	if err != nil {
		return PreparedUpdate{}, err
	}
	newPolicyChecksum := checksumBytes(source.policy)
	updatedState := cloneInstallationStateValue(state)
	updatedState.version = source.version
	updatedState.policyChecksum = newPolicyChecksum
	updatedState.targets = finalRecords
	if force.backupRequired {
		updatedState.backupPath = backupPath
	}
	stateBytes, err := serializeInstallState(updatedState)
	if err != nil {
		return PreparedUpdate{}, err
	}
	currentStateAction, err := newStateMutationAction("state", stateBytes, coords.stateFile, environment.StateHome)
	if err != nil {
		return PreparedUpdate{}, err
	}
	stateActions := []mutationAction{currentStateAction}
	for _, reference := range references {
		updatedReference := cloneInstallationStateValue(reference.state)
		updatedReference.version = source.version
		updatedReference.policyChecksum = newPolicyChecksum
		if force.backupRequired {
			updatedReference.backupPath = backupPath
		}
		rendered, err := serializeInstallState(updatedReference)
		if err != nil {
			return PreparedUpdate{}, err
		}
		action, err := newStateMutationAction(fmt.Sprintf("state-reference-%d", reference.index), rendered, reference.path, environment.StateHome)
		if err != nil {
			return PreparedUpdate{}, err
		}
		stateActions, err = addMutationAction(stateActions, action)
		if err != nil {
			return PreparedUpdate{}, err
		}
	}
	policyAction, err := newMutationAction(
		mutationReplace, "policy", source.policy, coords.policyPath, 0o600,
		environment.ConfigHome, "open-agent-workflow/ENGINEERING.md", policyView,
	)
	if err != nil {
		return PreparedUpdate{}, err
	}
	backup, err := buildMutationBackupPlan(
		force.backupRequired, "update", resolved.scope, backupPath,
		policyAction, targetActions, stateActions,
	)
	if err != nil {
		return PreparedUpdate{}, err
	}
	plan := mutationPlan{
		operation: mutationUpdate, source: source, environment: environment,
		request:  mutationRequest{project: request.Project, targets: request.Targets, dryRun: request.DryRun, force: request.Force},
		resolved: cloneResolvedRequest(resolved), coordinates: coords,
		targetActions: cloneMutationActions(targetActions), policyAction: cloneMutationAction(policyAction),
		stateActions: cloneMutationActions(stateActions), backup: cloneBackupPlan(backup),
	}
	plan.predicted = predictMutationResult(plan)
	return PreparedUpdate{plan: cloneMutationPlan(plan)}, nil
}
