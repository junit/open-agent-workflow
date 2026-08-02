package management

func PrepareUninstall(environment Environment, request UninstallRequest) (PreparedUninstall, error) {
	mutationRequest := mutationRequest{
		project: request.Project, targets: request.Targets,
		dryRun: request.DryRun, force: request.Force,
	}
	preparation, err := prepareMutationInputs(environment, mutationRequest)
	if err != nil {
		return PreparedUninstall{}, err
	}
	if !preparation.stateExists {
		return prepareMissingStateUninstall(environment, mutationRequest, preparation)
	}
	force, err := prepareMutationForce(
		preparation.state, preparation.coordinates, preparation.resolved,
		preparation.policy, request.Force,
	)
	if err != nil {
		return PreparedUninstall{}, err
	}
	if force.manual != nil {
		plan, err := prepareManualRecoveryPlan(
			mutationUninstall, Source{}, environment, mutationRequest,
			preparation.resolved, preparation.coordinates, preparation.policy,
			*force.manual, preparation.backupPath,
		)
		if err != nil {
			return PreparedUninstall{}, err
		}
		return PreparedUninstall{plan: plan}, nil
	}
	plan, err := prepareUninstallPlan(environment, mutationRequest, preparation, force)
	if err != nil {
		return PreparedUninstall{}, err
	}
	return PreparedUninstall{plan: plan}, nil
}

func prepareMissingStateUninstall(
	environment Environment,
	request mutationRequest,
	preparation mutationPreparation,
) (PreparedUninstall, error) {
	if err := verifyUntrackedMutationMarkers(preparation.coordinates, preparation.resolved); err != nil {
		return PreparedUninstall{}, err
	}
	plan := mutationPlan{
		operation: mutationUninstall, environment: environment, request: request,
		resolved: cloneResolvedRequest(preparation.resolved), coordinates: preparation.coordinates,
	}
	for _, id := range preparation.resolved.targets {
		plan.leadingLines = append(plan.leadingLines, "oaw: unchanged: "+id)
	}
	plan.predicted = predictMutationResult(plan)
	return PreparedUninstall{plan: cloneMutationPlan(plan)}, nil
}

func prepareUninstallPlan(
	environment Environment,
	request mutationRequest,
	preparation mutationPreparation,
	force forcePreparation,
) (mutationPlan, error) {
	remaining, removed := filterUninstallRecords(preparation.state.targets, preparation.resolved.targets)
	targetActions, err := prepareUninstallTargets(preparation, force, remaining, removed)
	if err != nil {
		return mutationPlan{}, err
	}
	remainingDirectories, directoryActions, err := prepareUninstallDirectories(
		preparation.coordinates, preparation.state, remaining,
	)
	if err != nil {
		return mutationPlan{}, err
	}
	stateActions, err := prepareUninstallStateActions(
		environment, preparation, force, remaining, remainingDirectories,
	)
	if err != nil {
		return mutationPlan{}, err
	}
	policyAction, err := prepareUninstallPolicyAction(preparation, force, remaining)
	if err != nil {
		return mutationPlan{}, err
	}
	backup, err := buildMutationBackupPlan(
		force.backupRequired, "uninstall", preparation.resolved.scope, preparation.backupPath,
		policyAction, targetActions, stateActions,
	)
	if err != nil {
		return mutationPlan{}, err
	}
	plan := mutationPlan{
		operation: mutationUninstall, environment: environment, request: request,
		resolved: cloneResolvedRequest(preparation.resolved), coordinates: preparation.coordinates,
		targetActions: cloneMutationActions(targetActions), policyAction: cloneMutationAction(policyAction),
		stateActions: cloneMutationActions(stateActions), directoryActions: cloneDirectoryActions(directoryActions),
		leadingLines: uninstallLeadingLines(preparation.state.targets, preparation.resolved.targets),
		backup:       cloneBackupPlan(backup),
	}
	plan.predicted = predictMutationResult(plan)
	return cloneMutationPlan(plan), nil
}

func prepareUninstallTargets(
	preparation mutationPreparation,
	force forcePreparation,
	remaining []targetRecord,
	removed []targetRecord,
) ([]mutationAction, error) {
	actions := make([]mutationAction, 0, len(removed))
	for _, record := range removed {
		if destinationReferenced(remaining, record.path) {
			continue
		}
		action, err := prepareUninstallTarget(preparation, force, record)
		if err != nil {
			return nil, err
		}
		actions, err = addMutationAction(actions, action)
		if err != nil {
			return nil, err
		}
	}
	return actions, nil
}

func prepareUninstallTarget(
	preparation mutationPreparation,
	force forcePreparation,
	record targetRecord,
) (mutationAction, error) {
	current, err := inspectInstallPath(record.path)
	if err != nil {
		return mutationAction{}, err
	}
	renderCurrent := current
	if repaired, found := force.repaired[record.path]; found {
		renderCurrent = cloneInstallPathSnapshot(repaired)
	}
	root, suffix, err := targetInstallCoordinates(preparation.coordinates, preparation.resolved, record.id)
	if err != nil {
		return mutationAction{}, err
	}
	switch record.mode {
	case "managed-block":
		rendered, err := renderManagedFileWithoutBlock(renderCurrent.data)
		if err != nil {
			return mutationAction{}, compatibilityError("managed markers are invalid: " + record.path)
		}
		if record.origin == "created-file" && len(rendered) == 0 {
			return newMutationAction(mutationRemove, record.id, nil, record.path, 0, root, suffix, current)
		}
		return newMutationAction(mutationReplace, record.id, rendered, record.path, 0o644, root, suffix, current)
	case "owned-file":
		if record.origin != "created-file" {
			return mutationAction{}, compatibilityError("invalid owned target origin")
		}
		return newMutationAction(mutationRemove, record.id, nil, record.path, 0, root, suffix, current)
	default:
		return mutationAction{}, compatibilityError("unknown target ownership mode: " + record.mode)
	}
}

func prepareUninstallStateActions(
	environment Environment,
	preparation mutationPreparation,
	force forcePreparation,
	remaining []targetRecord,
	remainingDirectories []string,
) ([]mutationAction, error) {
	if len(remaining) == 0 {
		action, err := newRemoveMutationAction("state", preparation.coordinates.stateFile, environment.StateHome)
		if err != nil {
			return nil, err
		}
		return []mutationAction{action}, nil
	}
	updated := cloneInstallationStateValue(preparation.state)
	updated.targets = cloneTargetRecords(remaining)
	updated.directories = append([]string(nil), remainingDirectories...)
	if force.backupRequired {
		updated.backupPath = preparation.backupPath
	}
	rendered, err := serializeInstallState(updated)
	if err != nil {
		return nil, err
	}
	action, err := newStateMutationAction(
		"state", rendered, preparation.coordinates.stateFile, environment.StateHome,
	)
	if err != nil {
		return nil, err
	}
	return []mutationAction{action}, nil
}

func prepareUninstallPolicyAction(
	preparation mutationPreparation,
	force forcePreparation,
	remaining []targetRecord,
) (mutationAction, error) {
	effect := mutationRetain
	if len(remaining) == 0 {
		references, err := collectPolicyStateReferencesWithBaseline(
			preparation.coordinates, preparation.coordinates.stateFile,
			preparation.policy, force.policyBaseline,
		)
		if err != nil {
			return mutationAction{}, err
		}
		if len(references) == 0 {
			effect = mutationRemove
		}
	}
	return newMutationAction(
		effect, "policy", nil, preparation.coordinates.policyPath, 0,
		preparation.coordinates.environment.ConfigHome,
		"open-agent-workflow/ENGINEERING.md", preparation.policy,
	)
}

func uninstallLeadingLines(records []targetRecord, selected []string) []string {
	lines := make([]string, 0)
	for _, id := range selected {
		if _, found := findTargetRecord(records, id); !found {
			lines = append(lines, "oaw: unchanged: "+id)
		}
	}
	return lines
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
