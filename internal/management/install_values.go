package management

import "bytes"

func cloneTargetRecords(records []targetRecord) []targetRecord {
	return append([]targetRecord(nil), records...)
}

func cloneInstallationStateValue(state installationState) installationState {
	state.directories = append([]string(nil), state.directories...)
	state.policyFiles = append([]policyFileRecord(nil), state.policyFiles...)
	state.targets = cloneTargetRecords(state.targets)
	return state
}

func cloneInstallPathSnapshot(snapshot installPathSnapshot) installPathSnapshot {
	snapshot.data = bytes.Clone(snapshot.data)
	return snapshot
}

func cloneInstallAction(action installAction) installAction {
	action.data = bytes.Clone(action.data)
	action.before = cloneInstallPathSnapshot(action.before)
	return action
}

func cloneInstallActions(actions []installAction) []installAction {
	result := make([]installAction, len(actions))
	for index, action := range actions {
		result[index] = cloneInstallAction(action)
	}
	return result
}

func cloneResolvedRequest(resolved resolvedRequest) resolvedRequest {
	resolved.targets = append([]string(nil), resolved.targets...)
	return resolved
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func installError(err error) error {
	if _, ok := err.(*Error); ok {
		return err
	}
	return compatibilityError(err.Error())
}
