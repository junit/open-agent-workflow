package management

func PrepareUninstall(environment Environment, request UninstallRequest) (PreparedUninstall, error) {
	resolved, err := resolve(CheckRequest{Project: request.Project, Targets: request.Targets})
	if err != nil {
		return PreparedUninstall{}, err
	}
	coords, err := initializeCoordinates(environment, resolved)
	if err != nil {
		return PreparedUninstall{}, err
	}
	backupPath := ""
	if request.Force {
		backupPath, err = reserveMutationBackupPath(coords)
		if err != nil {
			return PreparedUninstall{}, err
		}
	}
	policyView, err := inspectInstallPath(coords.policyPath)
	if err != nil {
		return PreparedUninstall{}, err
	}
	state, exists, err := readInstallationState(coords.stateFile)
	if err != nil {
		return PreparedUninstall{}, installError(err)
	}
	if !exists {
		if err := verifyUntrackedMutationMarkers(coords, resolved); err != nil {
			return PreparedUninstall{}, err
		}
		plan := mutationPlan{
			operation: mutationUninstall, environment: environment,
			request:  mutationRequest{project: request.Project, targets: request.Targets, dryRun: request.DryRun, force: request.Force},
			resolved: cloneResolvedRequest(resolved), coordinates: coords,
		}
		for _, id := range resolved.targets {
			plan.leadingLines = append(plan.leadingLines, "oaw: unchanged: "+id)
		}
		plan.predicted = predictMutationResult(plan)
		return PreparedUninstall{plan: cloneMutationPlan(plan)}, nil
	}
	force, err := prepareMutationForce(state, coords, resolved, policyView, request.Force)
	if err != nil {
		return PreparedUninstall{}, err
	}
	if force.manual != nil {
		plan, err := prepareManualRecoveryPlan(
			mutationUninstall, Source{}, environment,
			mutationRequest{project: request.Project, targets: request.Targets, dryRun: request.DryRun, force: request.Force},
			resolved, coords, policyView, *force.manual, backupPath,
		)
		if err != nil {
			return PreparedUninstall{}, err
		}
		return PreparedUninstall{plan: plan}, nil
	}

	remaining, removed := filterUninstallRecords(state.targets, resolved.targets)
	leading := make([]string, 0)
	for _, id := range resolved.targets {
		if _, found := findTargetRecord(state.targets, id); !found {
			leading = append(leading, "oaw: unchanged: "+id)
		}
	}
	targetActions := make([]mutationAction, 0, len(removed))
	for _, record := range removed {
		if destinationReferenced(remaining, record.path) {
			continue
		}
		current, err := inspectInstallPath(record.path)
		if err != nil {
			return PreparedUninstall{}, err
		}
		renderCurrent := current
		if repaired, found := force.repaired[record.path]; found {
			renderCurrent = cloneInstallPathSnapshot(repaired)
		}
		root, suffix, err := targetInstallCoordinates(coords, resolved, record.id)
		if err != nil {
			return PreparedUninstall{}, err
		}
		var action mutationAction
		switch record.mode {
		case "managed-block":
			rendered, renderErr := renderManagedFileWithoutBlock(renderCurrent.data)
			if renderErr != nil {
				return PreparedUninstall{}, compatibilityError("managed markers are invalid: " + record.path)
			}
			if record.origin == "created-file" && len(rendered) == 0 {
				action, err = newMutationAction(mutationRemove, record.id, nil, record.path, 0, root, suffix, current)
			} else {
				action, err = newMutationAction(mutationReplace, record.id, rendered, record.path, 0o644, root, suffix, current)
			}
		case "owned-file":
			if record.origin != "created-file" {
				return PreparedUninstall{}, compatibilityError("invalid owned target origin")
			}
			action, err = newMutationAction(mutationRemove, record.id, nil, record.path, 0, root, suffix, current)
		default:
			return PreparedUninstall{}, compatibilityError("unknown target ownership mode: " + record.mode)
		}
		if err != nil {
			return PreparedUninstall{}, err
		}
		targetActions, err = addMutationAction(targetActions, action)
		if err != nil {
			return PreparedUninstall{}, err
		}
	}

	remainingDirectories, directoryActions, err := prepareUninstallDirectories(coords, state, remaining)
	if err != nil {
		return PreparedUninstall{}, err
	}
	stateActions := make([]mutationAction, 0, 1)
	if len(remaining) > 0 {
		updated := cloneInstallationStateValue(state)
		updated.targets = remaining
		updated.directories = remainingDirectories
		if force.backupRequired {
			updated.backupPath = backupPath
		}
		rendered, err := serializeInstallState(updated)
		if err != nil {
			return PreparedUninstall{}, err
		}
		action, err := newStateMutationAction("state", rendered, coords.stateFile, environment.StateHome)
		if err != nil {
			return PreparedUninstall{}, err
		}
		stateActions = append(stateActions, action)
	} else {
		action, err := newRemoveMutationAction("state", coords.stateFile, environment.StateHome)
		if err != nil {
			return PreparedUninstall{}, err
		}
		stateActions = append(stateActions, action)
	}

	policyEffect := mutationRetain
	if len(remaining) == 0 {
		references, err := collectPolicyStateReferencesWithBaseline(coords, coords.stateFile, policyView, force.policyBaseline)
		if err != nil {
			return PreparedUninstall{}, err
		}
		if len(references) == 0 {
			policyEffect = mutationRemove
		}
	}
	policyAction, err := newMutationAction(
		policyEffect, "policy", nil, coords.policyPath, 0,
		environment.ConfigHome, "open-agent-workflow/ENGINEERING.md", policyView,
	)
	if err != nil {
		return PreparedUninstall{}, err
	}
	backup, err := buildMutationBackupPlan(
		force.backupRequired, "uninstall", resolved.scope, backupPath,
		policyAction, targetActions, stateActions,
	)
	if err != nil {
		return PreparedUninstall{}, err
	}
	plan := mutationPlan{
		operation: mutationUninstall, environment: environment,
		request:  mutationRequest{project: request.Project, targets: request.Targets, dryRun: request.DryRun, force: request.Force},
		resolved: cloneResolvedRequest(resolved), coordinates: coords,
		targetActions: cloneMutationActions(targetActions), policyAction: cloneMutationAction(policyAction),
		stateActions: cloneMutationActions(stateActions), directoryActions: cloneDirectoryActions(directoryActions),
		leadingLines: append([]string(nil), leading...), backup: cloneBackupPlan(backup),
	}
	plan.predicted = predictMutationResult(plan)
	return PreparedUninstall{plan: cloneMutationPlan(plan)}, nil
}

func filterUninstallRecords(records []targetRecord, selected []string) ([]targetRecord, []targetRecord) {
	selectedSet := make(map[string]struct{}, len(selected))
	for _, id := range selected {
		selectedSet[id] = struct{}{}
	}
	remaining := make([]targetRecord, 0, len(records))
	removed := make([]targetRecord, 0, len(records))
	for _, record := range records {
		if _, exists := selectedSet[record.id]; exists {
			removed = append(removed, record)
		} else {
			remaining = append(remaining, record)
		}
	}
	return remaining, removed
}

func newRemoveMutationAction(label, destination, root string) (mutationAction, error) {
	suffix, err := stateActionRelativeSuffix(root, destination)
	if err != nil {
		return mutationAction{}, err
	}
	before, err := inspectInstallPath(destination)
	if err != nil {
		return mutationAction{}, err
	}
	return newMutationAction(mutationRemove, label, nil, destination, 0, root, suffix, before)
}

func prepareUninstallDirectories(coords coordinates, state installationState, remaining []targetRecord) ([]string, []directoryAction, error) {
	retained := make([]string, 0, len(state.directories))
	actions := make([]directoryAction, 0, len(state.directories))
	for _, directory := range state.directories {
		namespace := installNamespaceDirectory(coords, directory)
		if (namespace && len(remaining) > 0) || (!namespace && directoryMatchesTargetRecords(directory, remaining, state, coords)) {
			retained = append(retained, directory)
			continue
		}
		root, err := ownedDirectoryMutationRoot(directory, state, coords)
		if err != nil {
			return nil, nil, err
		}
		suffix, err := stateActionRelativeSuffix(root, directory)
		if err != nil {
			return nil, nil, err
		}
		action, err := newDirectoryAction(directory, root, suffix, namespace)
		if err != nil {
			return nil, nil, err
		}
		actions = append(actions, action)
	}
	sortDirectoryActions(actions)
	return retained, actions, nil
}

func directoryMatchesTargetRecords(directory string, records []targetRecord, state installationState, coords coordinates) bool {
	for _, record := range records {
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
			return true
		}
	}
	return false
}

func ownedDirectoryMutationRoot(directory string, state installationState, coords coordinates) (string, error) {
	if directory == coords.configDir {
		return coords.environment.ConfigHome, nil
	}
	if directory == coords.stateDir || directory == coords.installations || directory == coords.projects {
		return coords.environment.StateHome, nil
	}
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
			return root, nil
		}
	}
	return "", compatibilityError("cannot bind owned directory removal: " + directory)
}
