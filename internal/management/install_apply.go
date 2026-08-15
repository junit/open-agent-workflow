package management

import (
	"bytes"
	"reflect"
)

func ApplyInstall(prepared PreparedInstall) (Result, error) {
	revalidated, err := PrepareInstall(prepared.source, prepared.environment, prepared.request)
	if err != nil {
		return Result{}, err
	}
	if !reflect.DeepEqual(prepared, revalidated) {
		return Result{}, compatibilityError("installation changed after preparation")
	}
	if prepared.request.DryRun {
		return cloneManagementResult(revalidated.predicted), nil
	}
	planned, err := installPathSet(revalidated.plannedDirectories)
	if err != nil {
		return Result{}, err
	}
	created := make(map[string]struct{}, len(planned))
	actions := make([]installAction, 0, len(revalidated.targetActions)+1+len(revalidated.policySetActions)+len(revalidated.stateActions))
	actions = append(actions, revalidated.policyAction)
	actions = append(actions, revalidated.policySetActions...)
	actions = append(actions, revalidated.targetActions...)
	actions = append(actions, revalidated.stateActions...)
	result := Result{Lines: make([]string, 0, len(actions))}
	for _, action := range actions {
		line, err := applyPreparedInstallAction(action, planned, created)
		if err != nil {
			return result, err
		}
		result.Lines = append(result.Lines, line)
	}
	if err := validateCreatedInstallDirectories(planned, created); err != nil {
		return result, err
	}
	return result, nil
}

func Install(source Source, environment Environment, request InstallRequest) (Result, error) {
	prepared, err := PrepareInstall(source, environment, request)
	if err != nil {
		return Result{}, err
	}
	return ApplyInstall(prepared)
}

func applyPreparedInstallAction(action installAction, planned, created map[string]struct{}) (string, error) {
	current, err := revalidateInstallActionSnapshot(action)
	if err != nil {
		return "", err
	}
	if current.kind == installPathRegular && bytes.Equal(current.data, action.data) {
		return "oaw: unchanged: " + action.label, nil
	}
	verb := "update"
	if current.kind == installPathMissing {
		verb = "create"
	}
	if err := scopedAtomicReplace(action, planned, created); err != nil {
		return "", err
	}
	return "oaw: " + verb + ": " + action.destination, nil
}

func revalidateInstallActionSnapshot(action installAction) (installPathSnapshot, error) {
	current, err := inspectInstallPath(action.destination)
	if err != nil {
		return installPathSnapshot{}, err
	}
	if !reflect.DeepEqual(current, action.before) {
		return installPathSnapshot{}, compatibilityError("destination changed after preparation: " + action.destination)
	}
	return current, nil
}

func cloneManagementResult(result Result) Result {
	result.Lines = append([]string(nil), result.Lines...)
	return result
}
