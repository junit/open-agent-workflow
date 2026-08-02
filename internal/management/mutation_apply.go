package management

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

type mutationPhase uint8

const (
	mutationPhaseBackup mutationPhase = iota + 1
	mutationPhaseTarget
	mutationPhaseTargetDirectory
	mutationPhasePolicy
	mutationPhaseState
	mutationPhaseNamespaceDirectory
)

type mutationFaultMoment uint8

const (
	mutationBefore mutationFaultMoment = iota + 1
	mutationAfter
)

type mutationFaultPoint struct {
	phase  mutationPhase
	moment mutationFaultMoment
	index  int
}

func (point mutationFaultPoint) String() string {
	phases := map[mutationPhase]string{
		mutationPhaseBackup: "backup", mutationPhaseTarget: "target",
		mutationPhaseTargetDirectory: "target-directory", mutationPhasePolicy: "policy",
		mutationPhaseState: "state", mutationPhaseNamespaceDirectory: "namespace-directory",
	}
	moment := "before"
	if point.moment == mutationAfter {
		moment = "after"
	}
	return fmt.Sprintf("%s-%s-%d", phases[point.phase], moment, point.index)
}

type mutationFaultInjector func(mutationFaultPoint) error

type mutationInverse struct {
	action    *mutationAction
	directory *directoryAction
}

func ApplyUpdate(prepared PreparedUpdate) (Result, error) {
	return applyMutationPlan(cloneMutationPlan(prepared.plan), nil)
}

func Update(source Source, environment Environment, request UpdateRequest) (Result, error) {
	prepared, err := PrepareUpdate(source, environment, request)
	if err != nil {
		return Result{}, err
	}
	return ApplyUpdate(prepared)
}

func ApplyUninstall(prepared PreparedUninstall) (Result, error) {
	return applyMutationPlan(cloneMutationPlan(prepared.plan), nil)
}

func Uninstall(environment Environment, request UninstallRequest) (Result, error) {
	prepared, err := PrepareUninstall(environment, request)
	if err != nil {
		return Result{}, err
	}
	return ApplyUninstall(prepared)
}

func applyMutationPlan(plan mutationPlan, injector mutationFaultInjector) (Result, error) {
	if err := validateMutationPlan(plan); err != nil {
		return Result{}, err
	}
	if plan.request.dryRun {
		result := cloneManagementResult(plan.predicted)
		if plan.terminal.status != 0 {
			return result, &Error{Status: plan.terminal.status, Message: plan.terminal.message}
		}
		return result, nil
	}
	result := Result{}
	if plan.operation == mutationUninstall {
		result.Lines = append(result.Lines, plan.leadingLines...)
	}
	journal := make([]mutationInverse, 0)
	fail := func(err error) (Result, error) {
		if err == nil {
			return result, nil
		}
		if rollbackErr := rollbackMutationJournal(journal); rollbackErr != nil {
			return result, &Error{Status: 74, Message: err.Error() + "; rollback failed: " + rollbackErr.Error()}
		}
		return result, err
	}
	inject := func(point mutationFaultPoint) error {
		if injector == nil {
			return nil
		}
		return injector(point)
	}

	if plan.backup.required {
		if err := inject(mutationFaultPoint{phase: mutationPhaseBackup, moment: mutationBefore}); err != nil {
			return fail(err)
		}
		line, err := applyMutationBackup(plan.backup, plan.environment)
		if err != nil {
			return fail(err)
		}
		result.Lines = append(result.Lines, line)
		if err := inject(mutationFaultPoint{phase: mutationPhaseBackup, moment: mutationAfter}); err != nil {
			return fail(err)
		}
	}
	if plan.terminal.status != 0 {
		return result, &Error{Status: plan.terminal.status, Message: plan.terminal.message}
	}

	applyActions := func(actions []mutationAction, phase mutationPhase) error {
		for index, action := range actions {
			point := mutationFaultPoint{phase: phase, moment: mutationBefore, index: index}
			if err := inject(point); err != nil {
				return err
			}
			line, changed, err := applyMutationAction(plan, action)
			if err != nil {
				return err
			}
			if changed {
				copy := cloneMutationAction(action)
				journal = append(journal, mutationInverse{action: &copy})
			}
			if line != "" {
				result.Lines = append(result.Lines, line)
			}
			point.moment = mutationAfter
			if err := inject(point); err != nil {
				return err
			}
		}
		return nil
	}
	applyDirectories := func(namespace bool, phase mutationPhase) error {
		for index, action := range plan.directoryActions {
			if action.namespace != namespace {
				continue
			}
			point := mutationFaultPoint{phase: phase, moment: mutationBefore, index: index}
			if err := inject(point); err != nil {
				return err
			}
			removed, err := scopedRemoveMutationDirectory(action)
			if err != nil {
				return err
			}
			if removed {
				copy := action
				copy.before = cloneInstallPathSnapshot(action.before)
				journal = append(journal, mutationInverse{directory: &copy})
				result.Lines = append(result.Lines, "oaw: remove-directory: "+action.destination)
			} else if action.before.kind != installPathMissing {
				result.Lines = append(result.Lines, "oaw: unchanged-directory: "+action.destination)
			}
			point.moment = mutationAfter
			if err := inject(point); err != nil {
				return err
			}
		}
		return nil
	}

	if err := applyActions(plan.targetActions, mutationPhaseTarget); err != nil {
		return fail(err)
	}
	if err := applyDirectories(false, mutationPhaseTargetDirectory); err != nil {
		return fail(err)
	}
	if plan.policyAction.effect != 0 {
		if err := applyActions([]mutationAction{plan.policyAction}, mutationPhasePolicy); err != nil {
			return fail(err)
		}
	}
	if err := applyActions(plan.stateActions, mutationPhaseState); err != nil {
		return fail(err)
	}
	if err := applyDirectories(true, mutationPhaseNamespaceDirectory); err != nil {
		return fail(err)
	}
	return result, nil
}

func validateMutationPlan(plan mutationPlan) error {
	if plan.operation != mutationUpdate && plan.operation != mutationUninstall {
		return compatibilityError("invalid prepared mutation operation")
	}
	actions := make([]mutationAction, 0, len(plan.targetActions)+1+len(plan.stateActions))
	actions = append(actions, plan.targetActions...)
	if plan.policyAction.effect != 0 {
		actions = append(actions, plan.policyAction)
	}
	actions = append(actions, plan.stateActions...)
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		if _, err := newMutationAction(action.effect, action.label, action.data, action.destination, action.mode, action.allowedRoot, action.relativeSuffix, action.before); err != nil {
			return err
		}
		if _, exists := seen[action.destination]; exists {
			return compatibilityError("duplicate prepared mutation destination: " + action.destination)
		}
		seen[action.destination] = struct{}{}
		if err := revalidateMutationActionSnapshot(action); err != nil {
			return err
		}
	}
	for _, action := range plan.directoryActions {
		rebuilt, err := newDirectoryAction(action.destination, action.allowedRoot, action.relativeSuffix, action.namespace)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(rebuilt.before, action.before) {
			return compatibilityError("owned directory changed after preparation: " + action.destination)
		}
	}
	if plan.backup.required {
		if _, err := renderBackupManifest(plan.backup); err != nil {
			return err
		}
		if err := revalidateBackupSources(plan.backup); err != nil {
			return err
		}
	}
	return nil
}

func revalidateMutationActionSnapshot(action mutationAction) error {
	current, err := inspectInstallPath(action.destination)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, action.before) {
		return compatibilityError("destination changed after preparation: " + action.destination)
	}
	return nil
}

func applyMutationAction(plan mutationPlan, action mutationAction) (string, bool, error) {
	if action.effect == mutationRetain {
		return "", false, nil
	}
	current, err := inspectInstallPath(action.destination)
	if err != nil {
		return "", false, err
	}
	if !reflect.DeepEqual(current, action.before) {
		return "", false, compatibilityError("destination changed after preparation: " + action.destination)
	}
	switch action.effect {
	case mutationReplace:
		if current.kind == installPathRegular && bytes.Equal(current.data, action.data) {
			return "oaw: unchanged: " + action.label, false, nil
		}
		if err := verifyActiveMutationBackup(plan, action); err != nil {
			return "", false, err
		}
		if err := scopedAtomicReplaceMutation(action); err != nil {
			return "", false, err
		}
		verb := "update"
		if current.kind == installPathMissing {
			verb = "create"
		}
		return "oaw: " + verb + ": " + action.destination, true, nil
	case mutationRemove:
		if current.kind == installPathMissing {
			return "", false, nil
		}
		if err := verifyActiveMutationBackup(plan, action); err != nil {
			return "", false, err
		}
		if err := scopedAtomicRemoveMutation(action); err != nil {
			return "", false, err
		}
		return "oaw: remove: " + action.destination, true, nil
	default:
		return "", false, compatibilityError("invalid mutation effect")
	}
}

func verifyActiveMutationBackup(plan mutationPlan, action mutationAction) error {
	if !plan.backup.required || !mutationActionNeedsBackup(action) {
		return nil
	}
	manifest, err := os.ReadFile(filepath.Join(plan.backup.path, "manifest.tsv"))
	if err != nil {
		return compatibilityError("active backup manifest is missing")
	}
	expected, err := renderBackupManifest(plan.backup)
	if err != nil || !bytes.Equal(manifest, expected) {
		return compatibilityError("active backup manifest has changed")
	}
	for _, candidate := range plan.backup.candidates {
		if candidate.original != action.destination {
			continue
		}
		backupBytes, err := os.ReadFile(candidate.backup)
		if err != nil || checksumBytes(backupBytes) != candidate.checksum {
			return compatibilityError("backup verification failed: " + action.destination)
		}
		current, err := inspectInstallPath(action.destination)
		if err != nil || current.kind != installPathRegular || checksumBytes(current.data) != candidate.checksum {
			return compatibilityError("backup source changed before mutation: " + action.destination)
		}
		return nil
	}
	return compatibilityError("mutation destination is missing from backup: " + action.destination)
}

func rollbackMutationJournal(journal []mutationInverse) error {
	var first error
	for index := len(journal) - 1; index >= 0; index-- {
		inverse := journal[index]
		var err error
		if inverse.directory != nil {
			err = restoreMutationDirectory(*inverse.directory)
		} else if inverse.action != nil {
			err = restoreMutationAction(*inverse.action)
		}
		if err != nil && first == nil {
			first = err
		}
	}
	return first
}

func restoreMutationAction(applied mutationAction) error {
	current, err := inspectInstallPath(applied.destination)
	if err != nil {
		return err
	}
	if applied.before.kind != installPathMissing && applied.before.kind != installPathRegular {
		return compatibilityError("cannot restore mutation destination: " + applied.destination)
	}
	if reflect.DeepEqual(current, applied.before) {
		return nil
	}
	if err := revalidateAppliedMutationForRollback(applied, current); err != nil {
		return err
	}
	switch applied.before.kind {
	case installPathMissing:
		remove := cloneMutationAction(applied)
		remove.effect = mutationRemove
		remove.data = nil
		remove.mode = 0
		remove.before = current
		return scopedAtomicRemoveMutation(remove)
	case installPathRegular:
		restore := cloneMutationAction(applied)
		restore.effect = mutationReplace
		restore.data = bytes.Clone(applied.before.data)
		restore.mode = applied.before.mode.Perm()
		restore.before = current
		return scopedAtomicReplaceMutation(restore)
	}
	return nil
}

func revalidateAppliedMutationForRollback(applied mutationAction, current installPathSnapshot) error {
	switch applied.effect {
	case mutationReplace:
		if current.kind != installPathRegular ||
			!bytes.Equal(current.data, applied.data) ||
			current.mode.Perm() != applied.mode.Perm() {
			return compatibilityError("mutation destination changed before rollback: " + applied.destination)
		}
	case mutationRemove:
		if current.kind != installPathMissing {
			return compatibilityError("mutation destination changed before rollback: " + applied.destination)
		}
	default:
		return compatibilityError("cannot restore mutation destination: " + applied.destination)
	}
	return nil
}

func restoreMutationDirectory(removed directoryAction) error {
	current, err := inspectInstallPath(removed.destination)
	if err != nil {
		return err
	}
	if current.kind == installPathDirectory {
		return nil
	}
	if current.kind != installPathMissing {
		return compatibilityError("cannot restore owned directory: " + removed.destination)
	}
	root, err := openInstallRoot(removed.allowedRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Mkdir(filepath.ToSlash(removed.relativeSuffix), removed.before.mode.Perm()); err != nil {
		return installIOError("cannot restore owned directory: " + removed.destination)
	}
	return nil
}
