package management

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

func ApplyInstall(prepared PreparedInstall) (Result, error) {
	return applyInstall(prepared, nil)
}

type installFaultMoment uint8

const (
	installFaultBefore installFaultMoment = iota + 1
	installFaultAfter
)

type installFaultPoint struct {
	moment installFaultMoment
	index  int
}

func (point installFaultPoint) String() string {
	moment := "before"
	if point.moment == installFaultAfter {
		moment = "after"
	}
	return fmt.Sprintf("%s-%d", moment, point.index)
}

type installFaultInjector func(installFaultPoint) error

func applyInstall(prepared PreparedInstall, injector installFaultInjector) (Result, error) {
	revalidated, err := PrepareInstall(prepared.source, prepared.environment, prepared.request)
	if err != nil {
		return Result{}, err
	}
	if !reflect.DeepEqual(prepared, revalidated) {
		return Result{}, integrityError("installation changed after preparation")
	}
	if prepared.request.DryRun {
		return cloneManagementResult(revalidated.predicted), nil
	}
	planned, err := installPathSet(revalidated.plannedDirectories)
	if err != nil {
		return Result{}, err
	}
	created := make(createdDirectorySet, len(planned))
	actions := make([]installAction, 0, len(revalidated.targetActions)+1+len(revalidated.policySetActions)+len(revalidated.stateActions))
	actions = append(actions, revalidated.policyAction)
	actions = append(actions, revalidated.policySetActions...)
	actions = append(actions, revalidated.targetActions...)
	actions = append(actions, revalidated.stateActions...)
	result := Result{Lines: make([]string, 0, len(actions))}
	journal := make([]mutationAction, 0, len(actions))
	createdDirectories := make(map[string]directoryAction)
	for index, action := range actions {
		if err := injectInstallFault(injector, installFaultPoint{moment: installFaultBefore, index: index}); err != nil {
			return failInstallApplication(result, journal, createdDirectories, err)
		}
		line, err := applyPreparedInstallAction(action, planned, created)
		if err == nil && installActionChangesDestination(action) {
			journal = append(journal, mutationActionFromInstallForRollback(action))
		}
		if directoryErr := recordCreatedInstallDirectories(created, createdDirectories, action); directoryErr != nil {
			if err == nil {
				err = directoryErr
			}
		}
		if err != nil {
			return failInstallApplication(result, journal, createdDirectories, err)
		}
		result.Lines = append(result.Lines, line)
		if err := injectInstallFault(injector, installFaultPoint{moment: installFaultAfter, index: index}); err != nil {
			return failInstallApplication(result, journal, createdDirectories, err)
		}
	}
	if err := validateCreatedInstallDirectories(planned, created); err != nil {
		return failInstallApplication(result, journal, createdDirectories, err)
	}
	return result, nil
}

func injectInstallFault(injector installFaultInjector, point installFaultPoint) error {
	if injector == nil {
		return nil
	}
	return injector(point)
}

func installActionChangesDestination(action installAction) bool {
	return !(action.before.kind == installPathRegular && bytes.Equal(action.before.data, action.data))
}

func mutationActionFromInstallForRollback(action installAction) mutationAction {
	return mutationAction{
		effect: mutationReplace, label: action.label, data: bytes.Clone(action.data),
		destination: action.destination, mode: action.mode,
		allowedRoot: action.allowedRoot, relativeSuffix: action.relativeSuffix,
		before: cloneInstallPathSnapshot(action.before),
	}
}

func recordCreatedInstallDirectories(
	created createdDirectorySet,
	recorded map[string]directoryAction,
	action installAction,
) error {
	for directory := range created {
		if _, alreadyRecorded := recorded[directory]; alreadyRecorded {
			continue
		}
		relative, err := relativeInstallDirectorySuffix(action.allowedRoot, directory)
		if err != nil {
			return err
		}
		owned, err := newDirectoryAction(directory, action.allowedRoot, relative, false)
		if err != nil {
			return err
		}
		if !owned.identity.captured || !sameMutationFileIdentity(created[directory], owned.identity.destination) {
			return integrityError("created owned directory identity changed: " + directory)
		}
		recorded[directory] = owned
	}
	return nil
}

func relativeInstallDirectorySuffix(root, directory string) (string, error) {
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", integrityError("owned directory escapes its allowed root: " + directory)
	}
	return filepath.ToSlash(relative), nil
}

func failInstallApplication(
	result Result,
	journal []mutationAction,
	directories map[string]directoryAction,
	err error,
) (Result, error) {
	if rollbackErr := rollbackInstallApplication(journal, directories); rollbackErr != nil {
		return result, &Error{Status: 74, Message: err.Error() + "; rollback failed: " + rollbackErr.Error()}
	}
	return result, err
}

func rollbackInstallApplication(journal []mutationAction, directories map[string]directoryAction) error {
	var first error
	for index := len(journal) - 1; index >= 0; index-- {
		if err := restoreMutationAction(journal[index]); err != nil && first == nil {
			first = err
		}
	}
	ordered := make([]directoryAction, 0, len(directories))
	for _, action := range directories {
		ordered = append(ordered, action)
	}
	sortDirectoryActions(ordered)
	for _, action := range ordered {
		if _, err := scopedRemoveMutationDirectory(action); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func Install(source Source, environment Environment, request InstallRequest) (Result, error) {
	prepared, err := PrepareInstall(source, environment, request)
	if err != nil {
		return Result{}, err
	}
	return ApplyInstall(prepared)
}

func applyPreparedInstallAction(action installAction, planned map[string]struct{}, created createdDirectorySet) (string, error) {
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
		return installPathSnapshot{}, integrityError("destination changed after preparation: " + action.destination)
	}
	return current, nil
}

func cloneManagementResult(result Result) Result {
	result.Lines = append([]string(nil), result.Lines...)
	return result
}
