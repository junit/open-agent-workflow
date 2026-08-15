package management

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApplyUpdateAndUninstall(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	updated := policySetSource(t, "0.2.0", "\nupdated policy\n")
	preparedUpdate, err := PrepareUpdate(updated, fixture.environment, UpdateRequest{Project: project, Targets: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyUpdate(preparedUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != len(updated.policySet)+2 || !strings.HasPrefix(result.Lines[0], "oaw: unchanged: cursor") || !strings.HasPrefix(result.Lines[1], "oaw: update: ") {
		t.Fatalf("update result = %v", result.Lines)
	}
	policy, err := os.ReadFile(installed.policyAction.destination)
	if err != nil || !bytes.Equal(policy, policySetFileContent(preparedUpdate.plan.coordinates, updated.policySet[0])) {
		t.Fatalf("policy = %q, %v", policy, err)
	}
	state, exists, err := readInstallationState(installed.stateActions[0].destination)
	if err != nil || !exists || state.version != "0.2.0" {
		t.Fatalf("state = %#v, %v, %v", state, exists, err)
	}

	preparedUninstall, err := PrepareUninstall(fixture.environment, UninstallRequest{Project: project, Targets: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	result, err = ApplyUninstall(preparedUninstall)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) == 0 || !strings.HasPrefix(result.Lines[0], "oaw: remove: ") {
		t.Fatalf("uninstall result = %v", result.Lines)
	}
	paths := []string{installed.targetActions[0].destination, installed.policyAction.destination, installed.stateActions[0].destination}
	for _, action := range installed.policySetActions {
		paths = append(paths, action.destination)
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("removed path %s exists: %v", path, err)
		}
	}
}

func TestApplyMutationDryRunIsWriteFree(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	updated := policySetSource(t, "0.2.0", "\nupdated policy\n")
	prepared, err := PrepareUpdate(updated, fixture.environment, UpdateRequest{Targets: "claude", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotPrepareTree(t, fixture.root)
	result, err := ApplyUpdate(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, prepared.plan.predicted) {
		t.Fatalf("result = %#v, predicted %#v", result, prepared.plan.predicted)
	}
	after := snapshotPrepareTree(t, fixture.root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dry run changed tree:\n%#v\n%#v", before, after)
	}
}

func TestPrepareForcedMutationAcceptsLexicalStateRootAlias(t *testing.T) {
	fixture := newPrepareFixture(t)
	fixture.environment.StateHome = filepath.Dir(fixture.environment.StateHome) + string(filepath.Separator) + string(filepath.Separator) + filepath.Base(fixture.environment.StateHome)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.plan.backup.required || len(prepared.plan.backup.candidates) == 0 {
		t.Fatalf("backup = %#v", prepared.plan.backup)
	}
	for _, candidate := range prepared.plan.backup.candidates {
		want := prepared.plan.backup.path + string(filepath.Separator) + filepath.Base(candidate.backup)
		if candidate.backup != want {
			t.Fatalf("backup artifact = %q, want %q", candidate.backup, want)
		}
	}
	if _, err := renderBackupManifest(prepared.plan.backup); err != nil {
		t.Fatal(err)
	}
}

func TestApplyManualRecoveryOnlyCreatesBackup(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	target := installed.targetActions[0].destination
	corrupt := []byte("personal\n" + endMarker + "\nambiguous\n" + endMarker + "\n")
	if err := os.WriteFile(target, corrupt, 0o640); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	nonBackupBefore := snapshotWithoutBackup(t, fixture.root)
	result, err := ApplyUpdate(prepared)
	assertManagementError(t, err, 65, prepared.plan.terminal.message)
	if len(result.Lines) != 1 || result.Lines[0] != "oaw: backup: "+prepared.plan.backup.path {
		t.Fatalf("result = %v", result.Lines)
	}
	if _, err := os.Stat(filepath.Join(prepared.plan.backup.path, "manifest.tsv")); err != nil {
		t.Fatal(err)
	}
	if after := snapshotWithoutBackup(t, fixture.root); !reflect.DeepEqual(nonBackupBefore, after) {
		t.Fatalf("manual recovery changed non-backup tree:\n%#v\n%#v", nonBackupBefore, after)
	}
}

func TestApplyMutationRejectsStalePlanBeforeWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	updated := policySetSource(t, "0.2.0", "\nupdated policy\n")
	prepared, err := PrepareUpdate(updated, fixture.environment, UpdateRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed.policyAction.destination, []byte("changed after prepare\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotPrepareTree(t, fixture.root)
	if _, err := ApplyUpdate(prepared); err == nil || !strings.Contains(err.Error(), "destination changed after preparation") {
		t.Fatalf("error = %v", err)
	}
	if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatalf("stale plan changed tree:\n%#v\n%#v", before, after)
	}
}

func TestUpdateAndUninstallComposePrepareAndApply(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})

	result, err := Update(fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) == 0 || result.Lines[0] != "oaw: unchanged: claude" {
		t.Fatalf("update result = %#v", result)
	}

	result, err = Uninstall(fixture.environment, UninstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) == 0 || !strings.HasPrefix(result.Lines[0], "oaw: remove: ") {
		t.Fatalf("uninstall result = %#v", result)
	}

	empty := newPrepareFixture(t)
	_, err = Update(empty.source, empty.environment, UpdateRequest{Targets: "claude"})
	assertManagementError(t, err, 66, "no installation state; run install first")
	result, err = Uninstall(empty.environment, UninstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 1 || result.Lines[0] != "oaw: unchanged: claude" {
		t.Fatalf("idempotent uninstall result = %#v", result)
	}
	if _, err := Uninstall(empty.environment, UninstallRequest{Targets: "unknown"}); err == nil {
		t.Fatal("invalid uninstall request succeeded")
	}
}

func TestApplyDryRunManualRecoveryReturnsPredictionWithoutWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	target := installed.targetActions[0].destination
	corrupt := []byte("personal\n" + endMarker + "\nambiguous\n" + endMarker + "\n")
	if err := os.WriteFile(target, corrupt, 0o640); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotPrepareTree(t, fixture.root)
	result, err := ApplyUpdate(prepared)
	assertManagementError(t, err, 65, prepared.plan.terminal.message)
	if !reflect.DeepEqual(result, prepared.plan.predicted) {
		t.Fatalf("result = %#v, predicted = %#v", result, prepared.plan.predicted)
	}
	if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatalf("dry-run manual recovery changed tree:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestApplyMutationRejectsTamperedPlansBeforeWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	prototype, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		change func(*mutationPlan)
		want   string
	}{
		{name: "operation", change: func(plan *mutationPlan) { plan.operation = 0 }, want: "invalid prepared mutation operation"},
		{name: "action", change: func(plan *mutationPlan) { plan.targetActions[0].label = "" }, want: "mutation action cannot be serialized"},
		{name: "duplicate destination", change: func(plan *mutationPlan) {
			plan.stateActions = append(plan.stateActions, cloneMutationAction(plan.targetActions[0]))
		}, want: "duplicate prepared mutation destination"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := cloneMutationPlan(prototype.plan)
			test.change(&plan)
			before := snapshotPrepareTree(t, fixture.root)
			_, err := applyMutationPlan(plan, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
				t.Fatalf("tampered plan changed tree:\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}

	projectFixture := newPrepareFixture(t)
	project := filepath.Join(projectFixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	materializeInstallRequest(t, projectFixture, InstallRequest{Project: project, Targets: "cursor"})
	uninstall, err := PrepareUninstall(projectFixture.environment, UninstallRequest{Project: project, Targets: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func(*mutationPlan)
		want   string
	}{
		{name: "directory coordinates", change: func(plan *mutationPlan) {
			plan.directoryActions[0].relativeSuffix = "../escape"
		}, want: "unsafe component"},
		{name: "directory snapshot", change: func(plan *mutationPlan) {
			plan.directoryActions[0].before = installPathSnapshot{kind: installPathMissing}
		}, want: "owned directory changed after preparation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := cloneMutationPlan(uninstall.plan)
			test.change(&plan)
			before := snapshotPrepareTree(t, projectFixture.root)
			_, err := applyMutationPlan(plan, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if after := snapshotPrepareTree(t, projectFixture.root); !reflect.DeepEqual(before, after) {
				t.Fatalf("tampered directory plan changed tree:\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}

	forceFixture := newPrepareFixture(t)
	forceInstalled := materializeInstallRequest(t, forceFixture, InstallRequest{Targets: "claude"})
	if err := os.WriteFile(forceInstalled.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	forcePrototype, err := PrepareUpdate(forceFixture.source, forceFixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func(*mutationPlan)
		want   string
	}{
		{name: "backup manifest", change: func(plan *mutationPlan) {
			plan.backup.operation = "install"
		}, want: "invalid backup operation"},
		{name: "backup source coordinates", change: func(plan *mutationPlan) {
			plan.backup.candidates[0].relativeSuffix = "different"
		}, want: "backup candidate destination does not match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := cloneMutationPlan(forcePrototype.plan)
			test.change(&plan)
			before := snapshotPrepareTree(t, forceFixture.root)
			_, err := applyMutationPlan(plan, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if after := snapshotPrepareTree(t, forceFixture.root); !reflect.DeepEqual(before, after) {
				t.Fatalf("tampered backup plan changed tree:\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

func TestApplyMutationActionClosedOutcomes(t *testing.T) {
	root := t.TempDir()
	plan := mutationPlan{}

	retainDestination := filepath.Join(root, "retain")
	retain := mutationAction{
		effect: mutationRetain, label: "retain", destination: retainDestination,
		allowedRoot: root, relativeSuffix: "retain", before: installPathSnapshot{kind: installPathMissing},
	}
	if line, changed, err := applyMutationAction(plan, retain); err != nil || changed || line != "" {
		t.Fatalf("retain line=%q changed=%t error=%v", line, changed, err)
	}

	createDestination := filepath.Join(root, "create")
	create, err := newMutationAction(
		mutationReplace, "create", []byte("created\n"), createDestination, 0o600,
		root, "create", installPathSnapshot{kind: installPathMissing},
	)
	if err != nil {
		t.Fatal(err)
	}
	if line, changed, err := applyMutationAction(plan, create); err != nil || !changed || line != "oaw: create: "+createDestination {
		t.Fatalf("create line=%q changed=%t error=%v", line, changed, err)
	}

	missingDestination := filepath.Join(root, "missing")
	removeMissing, err := newMutationAction(
		mutationRemove, "missing", nil, missingDestination, 0,
		root, "missing", installPathSnapshot{kind: installPathMissing},
	)
	if err != nil {
		t.Fatal(err)
	}
	if line, changed, err := applyMutationAction(plan, removeMissing); err != nil || changed || line != "" {
		t.Fatalf("remove missing line=%q changed=%t error=%v", line, changed, err)
	}

	invalid := removeMissing
	invalid.effect = 99
	if _, _, err := applyMutationAction(plan, invalid); err == nil || !strings.Contains(err.Error(), "invalid mutation effect") {
		t.Fatalf("invalid effect error = %v", err)
	}

	stale := removeMissing
	writePrepareFile(t, stale.destination, []byte("appeared\n"), 0o600)
	if _, _, err := applyMutationAction(plan, stale); err == nil || !strings.Contains(err.Error(), "changed after preparation") {
		t.Fatalf("stale action error = %v", err)
	}

	nestedDestination := filepath.Join(root, "absent", "artifact")
	nested, err := newMutationAction(
		mutationReplace, "nested", []byte("nested\n"), nestedDestination, 0o600,
		root, "absent/artifact", installPathSnapshot{kind: installPathMissing},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := applyMutationAction(plan, nested); err == nil || !strings.Contains(err.Error(), "destination path could not be inspected") {
		t.Fatalf("nested replace error = %v", err)
	}

	outPath := filepath.Join(root, "outside")
	writePrepareFile(t, outPath, []byte("outside\n"), 0o600)
	linkPath := filepath.Join(root, "remove-link")
	if err := os.Symlink(outPath, linkPath); err != nil {
		t.Fatal(err)
	}
	linkBefore, err := inspectInstallPath(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	removeLink := mutationAction{
		effect: mutationRemove, label: "remove-link", destination: linkPath,
		allowedRoot: root, relativeSuffix: "remove-link", before: linkBefore,
	}
	if _, _, err := applyMutationAction(plan, removeLink); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink remove error = %v", err)
	}

	blocker := filepath.Join(root, "blocker")
	writePrepareFile(t, blocker, []byte("blocker\n"), 0o600)
	uninspectable := mutationAction{destination: filepath.Join(blocker, "child")}
	if err := revalidateMutationActionSnapshot(uninspectable); err == nil {
		t.Fatal("uninspectable mutation destination was accepted")
	}
}

func TestActiveMutationBackupMustRemainVerified(t *testing.T) {
	tests := []struct {
		name   string
		change func(*testing.T, mutationPlan, mutationAction)
		want   string
	}{
		{name: "missing manifest", change: func(t *testing.T, plan mutationPlan, _ mutationAction) {
			if err := os.Remove(filepath.Join(plan.backup.path, "manifest.tsv")); err != nil {
				t.Fatal(err)
			}
		}, want: "active backup manifest is missing"},
		{name: "changed manifest", change: func(t *testing.T, plan mutationPlan, _ mutationAction) {
			if err := os.WriteFile(filepath.Join(plan.backup.path, "manifest.tsv"), []byte("changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: "active backup manifest has changed"},
		{name: "missing artifact", change: func(t *testing.T, plan mutationPlan, action mutationAction) {
			candidate := backupCandidateForDestination(t, plan.backup, action.destination)
			if err := os.Remove(candidate.backup); err != nil {
				t.Fatal(err)
			}
		}, want: "backup verification failed"},
		{name: "changed artifact", change: func(t *testing.T, plan mutationPlan, action mutationAction) {
			candidate := backupCandidateForDestination(t, plan.backup, action.destination)
			if err := os.WriteFile(candidate.backup, []byte("changed backup\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: "backup verification failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
			if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
			if err != nil {
				t.Fatal(err)
			}
			action := prepared.plan.targetActions[0]
			_, err = applyMutationPlan(prepared.plan, func(point mutationFaultPoint) error {
				if point.phase == mutationPhaseBackup && point.moment == mutationAfter {
					test.change(t, prepared.plan, action)
				}
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	t.Run("destination omitted", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		if err != nil {
			t.Fatal(err)
		}
		target := prepared.plan.targetActions[0].destination
		filtered := make([]backupCandidate, 0, len(prepared.plan.backup.candidates)-1)
		for _, candidate := range prepared.plan.backup.candidates {
			if candidate.original != target {
				filtered = append(filtered, candidate)
			}
		}
		prepared.plan.backup.candidates = filtered
		_, err = applyMutationPlan(prepared.plan, nil)
		if err == nil || !strings.Contains(err.Error(), "mutation destination is missing from backup") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("backup directory already exists", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
			t.Fatal(err)
		}
		prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(prepared.plan.backup.path, 0o700); err != nil {
			t.Fatal(err)
		}
		_, err = applyMutationPlan(prepared.plan, nil)
		if err == nil || !strings.Contains(err.Error(), "backup directory already exists") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestActiveMutationBackupRejectsChangedSource(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyMutationBackup(prepared.plan.backup, prepared.plan.environment); err != nil {
		t.Fatal(err)
	}
	action := prepared.plan.targetActions[0]
	if err := os.WriteFile(action.destination, []byte("changed after backup\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := verifyActiveMutationBackup(prepared.plan, action); err == nil || !strings.Contains(err.Error(), "backup source changed before mutation") {
		t.Fatalf("error = %v", err)
	}
}

func TestRollbackMutationJournalRestoresFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original")
	writePrepareFile(t, original, []byte("before\n"), 0o640)
	originalBefore, err := inspectInstallPath(original)
	if err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, original, []byte("after\n"), 0o600)
	originalAction := mutationAction{
		effect: mutationReplace, label: "original", data: []byte("after\n"), destination: original, mode: 0o600,
		allowedRoot: root, relativeSuffix: "original", before: originalBefore,
	}

	created := filepath.Join(root, "created")
	writePrepareFile(t, created, []byte("created\n"), 0o600)
	createdAction := mutationAction{
		effect: mutationReplace, label: "created", data: []byte("created\n"), destination: created, mode: 0o600,
		allowedRoot: root, relativeSuffix: "created", before: installPathSnapshot{kind: installPathMissing},
	}

	directory := filepath.Join(root, "owned")
	if err := os.Mkdir(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	directoryBefore, err := inspectInstallPath(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(directory); err != nil {
		t.Fatal(err)
	}
	directoryAction := directoryAction{
		destination: directory, allowedRoot: root, relativeSuffix: "owned", before: directoryBefore,
	}

	journal := []mutationInverse{
		{action: &originalAction},
		{action: &createdAction},
		{directory: &directoryAction},
		{},
	}
	if err := rollbackMutationJournal(journal); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(original); err != nil || string(data) != "before\n" {
		t.Fatalf("restored original = %q, %v", data, err)
	}
	if info, err := os.Stat(original); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("restored original mode = %v, %v", info, err)
	}
	if _, err := os.Lstat(created); !os.IsNotExist(err) {
		t.Fatalf("created destination still exists: %v", err)
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() || info.Mode().Perm() != 0o750 {
		t.Fatalf("restored directory = %v, %v", info, err)
	}
	if err := restoreMutationAction(createdAction); err != nil {
		t.Fatalf("already absent created action: %v", err)
	}
	if err := restoreMutationDirectory(directoryAction); err != nil {
		t.Fatalf("already restored directory: %v", err)
	}

	nonrestorableAction := createdAction
	nonrestorableAction.before = installPathSnapshot{kind: installPathDirectory}
	if err := restoreMutationAction(nonrestorableAction); err == nil || !strings.Contains(err.Error(), "cannot restore mutation destination") {
		t.Fatalf("nonrestorable action error = %v", err)
	}
	nonrestorableDirectory := directoryAction
	nonrestorableDirectory.destination = original
	nonrestorableDirectory.relativeSuffix = "original"
	if err := restoreMutationDirectory(nonrestorableDirectory); err == nil || !strings.Contains(err.Error(), "cannot restore owned directory") {
		t.Fatalf("nonrestorable directory error = %v", err)
	}
}

func TestRestoreMutationDirectoryPreservesOriginalMode(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "owned")
	action := directoryAction{
		destination: destination, allowedRoot: root, relativeSuffix: "owned",
		before: installPathSnapshot{kind: installPathDirectory, mode: 0o777},
	}
	if err := restoreMutationDirectory(action); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0o777 {
		t.Fatalf("restored directory mode = %v, %v", info, err)
	}
	wrongMode := action
	wrongMode.before.mode = 0o700
	if err := restoreMutationDirectory(wrongMode); err == nil || !strings.Contains(err.Error(), "changed before rollback") {
		t.Fatalf("wrong mode error = %v", err)
	}
	invalidSnapshot := action
	invalidSnapshot.before.kind = installPathMissing
	if err := restoreMutationDirectory(invalidSnapshot); err == nil || !strings.Contains(err.Error(), "cannot restore") {
		t.Fatalf("invalid snapshot error = %v", err)
	}
}

func TestRestoreMutationDirectoryRejectsUnsafeRollbackCoordinates(t *testing.T) {
	t.Run("changed root identity", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "root")
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(root, "owned")
		identity, err := captureMutationPathIdentity(root, destination)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(root, root+".prepared"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		action := directoryAction{
			destination: destination, allowedRoot: root, relativeSuffix: "owned",
			before:   installPathSnapshot{kind: installPathDirectory, mode: 0o755},
			identity: identity,
		}
		if err := restoreMutationDirectory(action); err == nil || !strings.Contains(err.Error(), "identity changed") {
			t.Fatalf("changed identity error = %v", err)
		}
	})

	t.Run("unsafe suffix", func(t *testing.T) {
		root := t.TempDir()
		action := directoryAction{
			destination: filepath.Join(root, "owned"), allowedRoot: root, relativeSuffix: "../owned",
			before: installPathSnapshot{kind: installPathDirectory, mode: 0o755},
		}
		if err := restoreMutationDirectory(action); err == nil || !strings.Contains(err.Error(), "unsafe component") {
			t.Fatalf("unsafe suffix error = %v", err)
		}
	})
}

func TestRollbackFailureReturnsCombinedRedactedDiagnostic(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	sentinel := "password=rollback-secret"
	if err := os.WriteFile(installed.targetActions[0].destination, []byte(sentinel+"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUninstall(fixture.environment, UninstallRequest{Project: project, Targets: "cursor", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	target := prepared.plan.targetActions[0].destination
	_, err = applyMutationPlan(prepared.plan, func(point mutationFaultPoint) error {
		if point.phase == mutationPhaseTarget && point.moment == mutationAfter && point.index == 0 {
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Fatal(err)
			}
			writePrepareFile(t, filepath.Join(target, "blocker"), []byte("block\n"), 0o600)
			return fmt.Errorf("injected target failure")
		}
		return nil
	})
	assertManagementError(t, err, 74, err.Error())
	if !strings.Contains(err.Error(), "injected target failure; rollback failed:") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("rollback diagnostic leaked file bytes: %v", err)
	}
}

func TestRollbackRefusesToOverwriteConcurrentFileChanges(t *testing.T) {
	if err := revalidateAppliedMutationForRollback(mutationAction{}, installPathSnapshot{}); err == nil || !strings.Contains(err.Error(), "cannot restore") {
		t.Fatalf("invalid rollback effect error = %v", err)
	}
	tests := []struct {
		name   string
		before installPathSnapshot
		effect mutationEffect
		data   []byte
		mode   os.FileMode
	}{
		{
			name: "replace existing", before: installPathSnapshot{kind: installPathRegular, data: []byte("original\n"), mode: 0o640},
			effect: mutationReplace, data: []byte("operation\n"), mode: 0o600,
		},
		{
			name: "create missing", before: installPathSnapshot{kind: installPathMissing},
			effect: mutationReplace, data: []byte("operation\n"), mode: 0o600,
		},
		{
			name: "remove existing", before: installPathSnapshot{kind: installPathRegular, data: []byte("original\n"), mode: 0o640},
			effect: mutationRemove,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			destination := filepath.Join(root, "artifact")
			concurrent := []byte("concurrent user bytes\n")
			writePrepareFile(t, destination, concurrent, 0o644)
			applied := mutationAction{
				effect: test.effect, label: "artifact", data: bytes.Clone(test.data), destination: destination,
				mode: test.mode, allowedRoot: root, relativeSuffix: "artifact", before: cloneInstallPathSnapshot(test.before),
			}
			if err := restoreMutationAction(applied); err == nil || !strings.Contains(err.Error(), "changed before rollback") {
				t.Fatalf("error = %v", err)
			}
			if data, err := os.ReadFile(destination); err != nil || !bytes.Equal(data, concurrent) {
				t.Fatalf("concurrent bytes = %q, %v", data, err)
			}
		})
	}
}

func backupCandidateForDestination(t *testing.T, plan backupPlan, destination string) backupCandidate {
	t.Helper()
	for _, candidate := range plan.candidates {
		if candidate.original == destination {
			return candidate
		}
	}
	t.Fatalf("no backup candidate for %s", destination)
	return backupCandidate{}
}

func snapshotWithoutBackup(t *testing.T, root string) map[string]prepareTreeEntry {
	t.Helper()
	snapshot := snapshotPrepareTree(t, root)
	for path := range snapshot {
		if strings.Contains(filepath.ToSlash(path), "/open-agent-workflow/backups/") || strings.HasSuffix(filepath.ToSlash(path), "/open-agent-workflow/backups") {
			delete(snapshot, path)
		}
	}
	return snapshot
}
