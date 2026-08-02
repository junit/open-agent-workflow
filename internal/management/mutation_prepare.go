package management

import "bytes"

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
	operation     mutationOperation
	source        Source
	environment   Environment
	request       mutationRequest
	resolved      resolvedRequest
	coordinates   coordinates
	targetActions []mutationAction
	policyAction  mutationAction
	stateActions  []mutationAction
	predicted     Result
}

type PreparedUpdate struct {
	plan mutationPlan
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
	actions := make([]mutationAction, 0, len(plan.targetActions)+1+len(plan.stateActions))
	actions = append(actions, plan.targetActions...)
	actions = append(actions, plan.policyAction)
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
	return Result{Lines: lines}
}

func cloneMutationPlan(plan mutationPlan) mutationPlan {
	plan.source = Source{version: plan.source.version, policy: bytes.Clone(plan.source.policy)}
	plan.resolved = cloneResolvedRequest(plan.resolved)
	plan.targetActions = cloneMutationActions(plan.targetActions)
	plan.policyAction = cloneMutationAction(plan.policyAction)
	plan.stateActions = cloneMutationActions(plan.stateActions)
	plan.predicted = cloneManagementResult(plan.predicted)
	return plan
}
