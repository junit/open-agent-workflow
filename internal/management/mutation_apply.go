package management

import (
	"bytes"
	"fmt"
	"os"
	"path"
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

type mutationApplicationStage struct {
	phase       mutationPhase
	actions     []mutationAction
	directories bool
	namespace   bool
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
	if plan.backup.required {
		line, err := applyMutationBackupStage(plan, injector)
		if line != "" {
			result.Lines = append(result.Lines, line)
		}
		if err != nil {
			return failMutationApplication(result, nil, err)
		}
	}
	if plan.terminal.status != 0 {
		return result, &Error{Status: plan.terminal.status, Message: plan.terminal.message}
	}
	journal := make([]mutationInverse, 0)
	for _, stage := range mutationApplicationStages(plan) {
		lines, inverses, err := applyMutationStage(plan, stage, injector)
		result.Lines = append(result.Lines, lines...)
		journal = append(journal, inverses...)
		if err != nil {
			return failMutationApplication(result, journal, err)
		}
	}
	return result, nil
}

func mutationApplicationStages(plan mutationPlan) []mutationApplicationStage {
	stages := []mutationApplicationStage{
		{phase: mutationPhaseTarget, actions: plan.targetActions},
		{phase: mutationPhaseTargetDirectory, directories: true},
	}
	if plan.policyAction.effect != 0 {
		stages = append(stages, mutationApplicationStage{
			phase: mutationPhasePolicy, actions: []mutationAction{plan.policyAction},
		})
	}
	return append(stages,
		mutationApplicationStage{phase: mutationPhaseState, actions: plan.stateActions},
		mutationApplicationStage{phase: mutationPhaseNamespaceDirectory, directories: true, namespace: true},
	)
}

func applyMutationBackupStage(plan mutationPlan, injector mutationFaultInjector) (string, error) {
	point := mutationFaultPoint{phase: mutationPhaseBackup, moment: mutationBefore}
	if err := injectMutationFault(injector, point); err != nil {
		return "", err
	}
	line, err := applyMutationBackup(plan.backup, plan.environment)
	if err != nil {
		return "", err
	}
	point.moment = mutationAfter
	return line, injectMutationFault(injector, point)
}

func applyMutationStage(
	plan mutationPlan,
	stage mutationApplicationStage,
	injector mutationFaultInjector,
) ([]string, []mutationInverse, error) {
	if stage.directories {
		return applyMutationDirectories(plan.directoryActions, stage, injector)
	}
	return applyMutationActions(plan, stage.actions, stage.phase, injector)
}

func applyMutationActions(
	plan mutationPlan,
	actions []mutationAction,
	phase mutationPhase,
	injector mutationFaultInjector,
) ([]string, []mutationInverse, error) {
	lines := make([]string, 0, len(actions))
	inverses := make([]mutationInverse, 0, len(actions))
	for index, action := range actions {
		point := mutationFaultPoint{phase: phase, moment: mutationBefore, index: index}
		if err := injectMutationFault(injector, point); err != nil {
			return lines, inverses, err
		}
		line, changed, err := applyMutationAction(plan, action)
		if err != nil {
			return lines, inverses, err
		}
		if changed {
			copy := cloneMutationAction(action)
			inverses = append(inverses, mutationInverse{action: &copy})
		}
		if line != "" {
			lines = append(lines, line)
		}
		point.moment = mutationAfter
		if err := injectMutationFault(injector, point); err != nil {
			return lines, inverses, err
		}
	}
	return lines, inverses, nil
}

func applyMutationDirectories(
	actions []directoryAction,
	stage mutationApplicationStage,
	injector mutationFaultInjector,
) ([]string, []mutationInverse, error) {
	lines := make([]string, 0, len(actions))
	inverses := make([]mutationInverse, 0, len(actions))
	for index, action := range actions {
		if action.namespace != stage.namespace {
			continue
		}
		point := mutationFaultPoint{phase: stage.phase, moment: mutationBefore, index: index}
		if err := injectMutationFault(injector, point); err != nil {
			return lines, inverses, err
		}
		removed, err := scopedRemoveMutationDirectory(action)
		if err != nil {
			return lines, inverses, err
		}
		if removed {
			copy := action
			copy.before = cloneInstallPathSnapshot(action.before)
			inverses = append(inverses, mutationInverse{directory: &copy})
			lines = append(lines, "oaw: remove-directory: "+action.destination)
		} else if action.before.kind != installPathMissing {
			lines = append(lines, "oaw: unchanged-directory: "+action.destination)
		}
		point.moment = mutationAfter
		if err := injectMutationFault(injector, point); err != nil {
			return lines, inverses, err
		}
	}
	return lines, inverses, nil
}

func injectMutationFault(injector mutationFaultInjector, point mutationFaultPoint) error {
	if injector == nil {
		return nil
	}
	return injector(point)
}

func failMutationApplication(result Result, journal []mutationInverse, err error) (Result, error) {
	if rollbackErr := rollbackMutationJournal(journal); rollbackErr != nil {
		return result, &Error{Status: 74, Message: err.Error() + "; rollback failed: " + rollbackErr.Error()}
	}
	return result, err
}

func validateMutationPlan(plan mutationPlan) error {
	if plan.operation != mutationUpdate && plan.operation != mutationUninstall {
		return compatibilityError("invalid prepared mutation operation")
	}
	if err := validatePreparedMutationActions(plan); err != nil {
		return err
	}
	if err := validatePreparedDirectoryActions(plan.directoryActions); err != nil {
		return err
	}
	if plan.backup.required {
		if _, err := renderBackupManifest(plan.backup); err != nil {
			return err
		}
		return revalidateBackupSources(plan.backup)
	}
	return nil
}

func validatePreparedMutationActions(plan mutationPlan) error {
	actions := make([]mutationAction, 0, len(plan.targetActions)+1+len(plan.stateActions))
	actions = append(actions, plan.targetActions...)
	if plan.policyAction.effect != 0 {
		actions = append(actions, plan.policyAction)
	}
	actions = append(actions, plan.stateActions...)
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		rebuilt, err := newMutationAction(action.effect, action.label, action.data, action.destination, action.mode, action.allowedRoot, action.relativeSuffix, action.before)
		if err != nil {
			return err
		}
		if err := compareMutationPathIdentity(action.identity, rebuilt.identity, action.destination); err != nil {
			return err
		}
		destinationKey := filepath.Clean(action.destination)
		if _, exists := seen[destinationKey]; exists {
			return compatibilityError("duplicate prepared mutation destination: " + action.destination)
		}
		seen[destinationKey] = struct{}{}
		if err := revalidateMutationActionSnapshot(action); err != nil {
			return err
		}
	}
	return nil
}

func validatePreparedDirectoryActions(actions []directoryAction) error {
	seenDirectories := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		destinationKey := filepath.Clean(action.destination)
		if _, exists := seenDirectories[destinationKey]; exists {
			return compatibilityError("duplicate prepared directory destination: " + action.destination)
		}
		seenDirectories[destinationKey] = struct{}{}
		rebuilt, err := newDirectoryAction(action.destination, action.allowedRoot, action.relativeSuffix, action.namespace)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(rebuilt.before, action.before) {
			return compatibilityError("owned directory changed after preparation: " + action.destination)
		}
		if err := compareMutationPathIdentity(action.identity, rebuilt.identity, action.destination); err != nil {
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
	return revalidateMutationPathIdentity(action.identity, action.allowedRoot, action.destination)
}

func compareMutationPathIdentity(expected, current mutationPathIdentity, destination string) error {
	if !expected.captured || !current.captured {
		return compatibilityError("prepared mutation identity is missing: " + destination)
	}
	if !sameMutationFileIdentity(expected.root, current.root) ||
		!sameMutationFileIdentity(expected.parent, current.parent) ||
		!sameMutationFileIdentity(expected.destination, current.destination) {
		return compatibilityError("destination identity changed after preparation: " + destination)
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
	if err := revalidateMutationPathIdentity(action.identity, action.allowedRoot, action.destination); err != nil {
		return "", false, err
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
	manifest, err := readVerifiedBackupFile(plan.environment, filepath.Join(plan.backup.path, "manifest.tsv"), "active backup manifest")
	if err != nil {
		return err
	}
	expected, err := renderBackupManifest(plan.backup)
	if err != nil || !bytes.Equal(manifest, expected) {
		return compatibilityError("active backup manifest has changed")
	}
	for _, candidate := range plan.backup.candidates {
		if candidate.original != action.destination {
			continue
		}
		backupBytes, err := readVerifiedBackupFile(plan.environment, candidate.backup, "backup artifact")
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
	currentIdentity, err := captureMutationPathIdentity(applied.allowedRoot, applied.destination)
	if err != nil {
		return err
	}
	switch applied.before.kind {
	case installPathMissing:
		remove := cloneMutationAction(applied)
		remove.effect = mutationRemove
		remove.data = nil
		remove.mode = 0
		remove.before = current
		remove.identity = currentIdentity
		return scopedAtomicRemoveMutation(remove)
	case installPathRegular:
		restore := cloneMutationAction(applied)
		restore.effect = mutationReplace
		restore.data = bytes.Clone(applied.before.data)
		restore.mode = applied.before.mode.Perm()
		restore.before = current
		restore.identity = currentIdentity
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
	if removed.before.kind != installPathDirectory {
		return compatibilityError("cannot restore owned directory: " + removed.destination)
	}
	current, err := inspectInstallPath(removed.destination)
	if err != nil {
		return err
	}
	if current.kind == installPathDirectory {
		if current.mode.Perm() != removed.before.mode.Perm() {
			return compatibilityError("owned directory changed before rollback: " + removed.destination)
		}
		return nil
	}
	if current.kind != installPathMissing {
		return compatibilityError("cannot restore owned directory: " + removed.destination)
	}
	return restoreMissingMutationDirectory(removed)
}

func restoreMissingMutationDirectory(removed directoryAction) error {
	root, err := openExistingInstallRoot(removed.allowedRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	if removed.identity.captured {
		opened, openedErr := root.Stat(".")
		inspected, inspectedErr := os.Lstat(removed.allowedRoot)
		if openedErr != nil || inspectedErr != nil ||
			!sameMutationFileIdentity(removed.identity.root, opened) ||
			!sameMutationFileIdentity(opened, inspected) {
			return compatibilityError("destination identity changed after preparation: " + removed.destination)
		}
	}
	install := installAction{
		destination: removed.destination, allowedRoot: removed.allowedRoot,
		relativeSuffix: removed.relativeSuffix,
	}
	if err := revalidateScopedAction(root, install); err != nil {
		return err
	}
	directoryRoot, err := openScopedActionDirectory(root, install)
	if err != nil {
		return err
	}
	defer directoryRoot.Close()
	name := path.Base(removed.relativeSuffix)
	if err := directoryRoot.Mkdir(name, removed.before.mode.Perm()); err != nil {
		return installIOError("cannot restore owned directory: " + removed.destination)
	}
	created, err := directoryRoot.Lstat(name)
	if err != nil || !created.IsDir() || created.Mode()&os.ModeSymlink != 0 {
		return compatibilityError("owned directory changed during rollback: " + removed.destination)
	}
	if err := directoryRoot.Chmod(name, removed.before.mode.Perm()); err != nil {
		if current, currentErr := directoryRoot.Lstat(name); currentErr == nil && sameMutationFileIdentity(created, current) {
			_ = directoryRoot.Remove(name)
		}
		return installIOError("cannot restore owned directory mode: " + removed.destination)
	}
	info, err := directoryRoot.Lstat(name)
	if err != nil || !sameMutationFileIdentity(created, info) || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != removed.before.mode.Perm() {
		return compatibilityError("owned directory changed during rollback: " + removed.destination)
	}
	syncScopedDirectory(directoryRoot, ".")
	return nil
}
