package management

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareUpdateForcePlansRecoveryBackup(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	target := installed.targetActions[0].destination
	drift := []byte(beginMarker + "\ndrifted body\n" + endMarker + "\n")
	if err := os.WriteFile(target, drift, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"}); err == nil {
		t.Fatal("drifted update without force succeeded")
	}
	prepared, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.plan.backup.required || prepared.plan.backup.operation != "update" {
		t.Fatalf("backup = %#v", prepared.plan.backup)
	}
	wantOriginals := []string{target, installed.stateActions[0].destination}
	if got := backupCandidateOriginals(prepared.plan.backup.candidates); !reflect.DeepEqual(got, wantOriginals) {
		t.Fatalf("backup candidates = %v, want %v", got, wantOriginals)
	}
	state, err := parseInstallationState(prepared.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if state.backupPath != prepared.plan.backup.path {
		t.Fatalf("state backup = %q, plan = %q", state.backupPath, prepared.plan.backup.path)
	}
	if len(prepared.plan.predicted.Lines) == 0 || prepared.plan.predicted.Lines[0] != "oaw: would-backup: "+prepared.plan.backup.path {
		t.Fatalf("predicted = %v", prepared.plan.predicted.Lines)
	}
}

func TestPrepareUpdateForceRepairsSingleMissingMarker(t *testing.T) {
	for _, missing := range []string{"begin", "end"} {
		t.Run(missing, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
			target := installed.targetActions[0].destination
			current, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			marker := beginMarker + "\n"
			if missing == "end" {
				marker = endMarker + "\n"
			}
			corrupt := bytes.Replace(current, []byte(marker), nil, 1)
			if err := os.WriteFile(target, corrupt, 0o644); err != nil {
				t.Fatal(err)
			}
			prepared, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
			if err != nil {
				t.Fatal(err)
			}
			if !prepared.plan.backup.required || prepared.plan.terminal.status != 0 {
				t.Fatalf("plan = %#v", prepared.plan)
			}
			rendered := prepared.plan.targetActions[0].data
			if bytes.Count(rendered, []byte(beginMarker)) != 1 || bytes.Count(rendered, []byte(endMarker)) != 1 {
				t.Fatalf("repaired target = %q", rendered)
			}
			if !bytes.Contains(rendered, []byte("Open Agent Workflow is opt-in.")) {
				t.Fatalf("repaired target does not contain the activation router: %q", rendered)
			}
			if bytes.Contains(rendered, []byte("\n@"+installed.policyAction.destination+"\n")) {
				t.Fatalf("repaired target retains eager policy import: %q", rendered)
			}
			if !bytes.Equal(rendered, current) {
				t.Fatalf("repaired bytes = %q, want original %q", rendered, current)
			}
		})
	}
}

func TestPrepareUpdateForceAmbiguousMarkerCreatesTerminalRecoveryPlan(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	target := installed.targetActions[0].destination
	corrupt := []byte("personal\n" + endMarker + "\nambiguous\n" + endMarker + "\n")
	if err := os.WriteFile(target, corrupt, 0o640); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.plan.terminal.status != 65 || !strings.Contains(prepared.plan.terminal.message, "manual recovery required; backup: ") {
		t.Fatalf("terminal = %#v", prepared.plan.terminal)
	}
	want := []string{target, installed.stateActions[0].destination, installed.policyAction.destination}
	if got := backupCandidateOriginals(prepared.plan.backup.candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("manual candidates = %v, want %v", got, want)
	}
	if len(prepared.plan.targetActions) != 0 || len(prepared.plan.stateActions) != 0 {
		t.Fatalf("manual recovery includes mutations: %#v", prepared.plan)
	}
}

func TestPrepareUpdateForcePolicyDriftAndCleanForce(t *testing.T) {
	t.Run("policy drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.WriteFile(installed.policyAction.destination, []byte("policy drift\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		prepared, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{installed.policyAction.destination, installed.stateActions[0].destination}
		if got := backupCandidateOriginals(prepared.plan.backup.candidates); !reflect.DeepEqual(got, want) {
			t.Fatalf("policy candidates = %v, want %v", got, want)
		}
	})

	t.Run("clean force", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		prepared, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		if err != nil {
			t.Fatal(err)
		}
		if prepared.plan.backup.required || len(prepared.plan.backup.candidates) != 0 {
			t.Fatalf("clean force backup = %#v", prepared.plan.backup)
		}
	})
}

func TestPrepareUpdateForceHandlesProjectPolicySetOwnership(t *testing.T) {
	t.Run("backs up tracked drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		source := projectPolicySetSource(t, "0.1.0", "")
		installed, err := PrepareInstall(source, fixture.environment, InstallRequest{Project: project, Targets: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ApplyInstall(installed); err != nil {
			t.Fatal(err)
		}
		physicalProject, err := filepath.EvalSymlinks(project)
		if err != nil {
			t.Fatal(err)
		}
		drifted := filepath.Join(physicalProject, ".oaw", "policy", "cooperative-protocol.md")
		writePrepareFile(t, drifted, []byte("tracked drift\n"), 0o644)

		prepared, err := prepareUpdateWithoutWrites(
			t, fixture.root, source, fixture.environment,
			UpdateRequest{Project: project, Targets: "codex", Force: true},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !prepared.plan.backup.required {
			t.Fatal("forced project Policy Set update did not require a backup")
		}
		want := []string{drifted, installed.stateActions[0].destination}
		if got := backupCandidateOriginals(prepared.plan.backup.candidates); !reflect.DeepEqual(got, want) {
			t.Fatalf("project Policy Set backup candidates = %v, want %v", got, want)
		}
	})

	t.Run("rejects untracked injection", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		source := projectPolicySetSource(t, "0.1.0", "")
		if _, err := Install(source, fixture.environment, InstallRequest{Project: project, Targets: "codex"}); err != nil {
			t.Fatal(err)
		}
		injected := filepath.Join(project, ".oaw", "policy", "injected.md")
		writePrepareFile(t, injected, []byte("untracked\n"), 0o644)

		_, err := prepareUpdateWithoutWrites(
			t, fixture.root, source, fixture.environment,
			UpdateRequest{Project: project, Targets: "codex", Force: true},
		)
		if err == nil || !strings.Contains(err.Error(), "unexpected managed Policy Set entry") {
			t.Fatalf("forced update error = %v", err)
		}
	})
}

func TestPrepareUninstallForceBacksUpFinalArtifacts(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	target := installed.targetActions[0].destination
	if err := os.WriteFile(target, []byte("drifted owned file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "cursor", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{installed.policyAction.destination, target, installed.stateActions[0].destination}
	if got := backupCandidateOriginals(prepared.plan.backup.candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("uninstall candidates = %v, want %v", got, want)
	}
	if prepared.plan.backup.operation != "uninstall" || prepared.plan.targetActions[0].effect != mutationRemove {
		t.Fatalf("plan = %#v", prepared.plan)
	}
}

func TestPrepareUpdateForcePolicyDriftCoordinatesAllLiveStates(t *testing.T) {
	fixture := newPrepareFixture(t)
	userInstall := materializeInstallRequest(t, fixture, InstallRequest{Targets: "codex"})
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	projectInstall := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	if err := os.WriteFile(userInstall.policyAction.destination, []byte("shared policy drift\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "codex", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{userInstall.policyAction.destination, userInstall.stateActions[0].destination, projectInstall.stateActions[0].destination}
	if got := backupCandidateOriginals(prepared.plan.backup.candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("coordinated candidates = %v, want %v", got, want)
	}
	if len(prepared.plan.stateActions) != 2 {
		t.Fatalf("state actions = %#v", prepared.plan.stateActions)
	}
	for _, action := range prepared.plan.stateActions {
		state, err := parseInstallationState(action.data)
		if err != nil {
			t.Fatal(err)
		}
		if state.backupPath != prepared.plan.backup.path {
			t.Fatalf("state %s backup = %q", action.label, state.backupPath)
		}
	}
}

func backupCandidateOriginals(candidates []backupCandidate) []string {
	result := make([]string, len(candidates))
	for index, candidate := range candidates {
		result[index] = candidate.original
	}
	return result
}

func TestPrepareMutationForceRejectsUnsafeAndUnselectedDrift(t *testing.T) {
	t.Run("unselected drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude,codex"})
		if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "codex", Force: true})
		if err == nil || !strings.Contains(err.Error(), "managed target block has drifted: claude") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("selected file missing", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.Remove(installed.targetActions[0].destination); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		if err == nil || !strings.Contains(err.Error(), "forced target has no recoverable file") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("scope mismatch", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		state := parsePreparedState(t, installed.stateActions[0])
		state.scope = "project"
		state.project = filepath.Join(fixture.root, "other")
		rendered, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, installed.stateActions[0].destination, rendered, 0o600)
		_, err = prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		assertManagementError(t, err, 65, "installed scope does not match")
	})

	t.Run("policy missing", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.Remove(installed.policyAction.destination); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		assertManagementError(t, err, 65, "managed policy is missing")
	})
}

func TestVerifyForcedTargetRecordRejectsInvalidCoordinates(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	state := parsePreparedState(t, installed.stateActions[0])
	record := state.targets[0]

	wrongPath := record
	wrongPath.path += "-other"
	if _, _, _, _, err := verifyForcedTargetRecord(wrongPath, installed.coordinates, state); err == nil || !strings.Contains(err.Error(), "path does not match") {
		t.Fatalf("path error = %v", err)
	}
	wrongMode := record
	wrongMode.mode = "managed-block"
	if _, _, _, _, err := verifyForcedTargetRecord(wrongMode, installed.coordinates, state); err == nil || !strings.Contains(err.Error(), "ownership does not match") {
		t.Fatalf("mode error = %v", err)
	}
	unknown := record
	unknown.id = "unknown"
	if _, _, _, _, err := verifyForcedTargetRecord(unknown, installed.coordinates, state); err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("unknown error = %v", err)
	}
	current, repaired, manual, changed, err := verifyForcedTargetRecord(record, installed.coordinates, state)
	if err != nil || current.kind != installPathRegular || repaired.kind != 0 || manual || changed {
		t.Fatalf("clean owned result = %#v %#v %v %v %v", current, repaired, manual, changed, err)
	}
}

func TestRepairManagedMarkerStructureRejectsAmbiguousFragments(t *testing.T) {
	expected := []byte(beginMarker + "\nbody\n" + endMarker + "\n")
	tests := []struct {
		name     string
		current  []byte
		expected []byte
	}{
		{name: "invalid expected", current: []byte("body\n" + endMarker + "\n"), expected: []byte("one line")},
		{name: "begin fragment too short", current: []byte(endMarker + "\n"), expected: expected},
		{name: "begin fragment mismatch", current: []byte("other\n" + endMarker + "\n"), expected: expected},
		{name: "end fragment too short", current: []byte(beginMarker + "\n"), expected: expected},
		{name: "end fragment mismatch", current: []byte(beginMarker + "\nother\n"), expected: expected},
		{name: "duplicate markers", current: []byte(beginMarker + "\n" + beginMarker + "\nbody\n" + endMarker + "\n"), expected: expected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if repaired, ok := repairManagedMarkerStructure(tt.current, tt.expected); ok || repaired != nil {
				t.Fatalf("repair = %q, %v", repaired, ok)
			}
		})
	}
}

func TestPrepareManualRecoveryPlanRejectsInvalidRecoveryCoordinates(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	state := parsePreparedState(t, installed.stateActions[0])
	policy, err := inspectInstallPath(installed.policyAction.destination)
	if err != nil {
		t.Fatal(err)
	}
	current, err := inspectInstallPath(installed.targetActions[0].destination)
	if err != nil {
		t.Fatal(err)
	}
	recovery := manualRecovery{record: state.targets[0], current: current}
	recovery.record.id = "unknown"
	if _, err := prepareManualRecoveryPlan(
		mutationUpdate, fixture.source, fixture.environment, mutationRequest{targets: "claude", force: true},
		installed.resolved, installed.coordinates, policy, recovery,
		filepath.Join(installed.coordinates.backupRoot, "operation"),
	); err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("error = %v", err)
	}
}

func TestPrepareManualRecoveryPlanRejectsInvalidBackupInputs(t *testing.T) {
	setup := func(t *testing.T) (prepareFixture, PreparedInstall, installationState, installPathSnapshot, manualRecovery) {
		t.Helper()
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		state := parsePreparedState(t, installed.stateActions[0])
		policy, err := inspectInstallPath(installed.policyAction.destination)
		if err != nil {
			t.Fatal(err)
		}
		current, err := inspectInstallPath(installed.targetActions[0].destination)
		if err != nil {
			t.Fatal(err)
		}
		return fixture, installed, state, policy, manualRecovery{record: state.targets[0], current: current}
	}

	t.Run("nonregular recovery source", func(t *testing.T) {
		fixture, installed, _, policy, recovery := setup(t)
		recovery.current = installPathSnapshot{kind: installPathMissing}
		_, err := prepareManualRecoveryPlan(
			mutationUpdate, fixture.source, fixture.environment, mutationRequest{targets: "claude", force: true},
			installed.resolved, installed.coordinates, policy, recovery,
			filepath.Join(installed.coordinates.backupRoot, "operation"),
		)
		if err == nil || !strings.Contains(err.Error(), "backup source is not a file") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("missing state source", func(t *testing.T) {
		fixture, installed, _, policy, recovery := setup(t)
		if err := os.Remove(installed.stateActions[0].destination); err != nil {
			t.Fatal(err)
		}
		_, err := prepareManualRecoveryPlan(
			mutationUpdate, fixture.source, fixture.environment, mutationRequest{targets: "claude", force: true},
			installed.resolved, installed.coordinates, policy, recovery,
			filepath.Join(installed.coordinates.backupRoot, "operation"),
		)
		if err == nil || !strings.Contains(err.Error(), "backup source is not a file") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("state root mismatch", func(t *testing.T) {
		fixture, installed, _, policy, recovery := setup(t)
		environment := fixture.environment
		environment.StateHome = filepath.Join(fixture.root, "other-state")
		_, err := prepareManualRecoveryPlan(
			mutationUpdate, fixture.source, environment, mutationRequest{targets: "claude", force: true},
			installed.resolved, installed.coordinates, policy, recovery,
			filepath.Join(installed.coordinates.backupRoot, "operation"),
		)
		if err == nil || !strings.Contains(err.Error(), "escapes its allowed root") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid backup path", func(t *testing.T) {
		fixture, installed, _, policy, recovery := setup(t)
		_, err := prepareManualRecoveryPlan(
			mutationUpdate, fixture.source, fixture.environment, mutationRequest{targets: "claude", force: true},
			installed.resolved, installed.coordinates, policy, recovery, "relative-backup",
		)
		if err == nil || !strings.Contains(err.Error(), "invalid backup path") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("policy root mismatch", func(t *testing.T) {
		fixture, installed, _, policy, recovery := setup(t)
		environment := fixture.environment
		environment.ConfigHome = filepath.Join(fixture.root, "other-config")
		_, err := prepareManualRecoveryPlan(
			mutationUpdate, fixture.source, environment, mutationRequest{targets: "claude", force: true},
			installed.resolved, installed.coordinates, policy, recovery,
			filepath.Join(installed.coordinates.backupRoot, "operation"),
		)
		if err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestPrepareMutationForceRejectsProjectPolicyAndDirectoryBindings(t *testing.T) {
	t.Run("policy path mismatch", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		state := parsePreparedState(t, installed.stateActions[0])
		state.policyPath = filepath.Join(fixture.environment.ConfigHome, "other", "ENGINEERING.md")
		rendered, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, installed.stateActions[0].destination, rendered, 0o600)
		_, err = prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		assertManagementError(t, err, 65, "installed policy path does not match")
	})

	t.Run("project mismatch", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "claude"})
		state := parsePreparedState(t, installed.stateActions[0])
		state.project = filepath.Join(fixture.root, "other-project")
		rendered, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, installed.stateActions[0].destination, rendered, 0o600)
		_, err = prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Project: project, Targets: "claude", Force: true})
		assertManagementError(t, err, 65, "installed project root does not match")
	})

	t.Run("unowned directory", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		state := parsePreparedState(t, installed.stateActions[0])
		state.directories = append(state.directories, filepath.Join(fixture.root, "unowned"))
		rendered, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, installed.stateActions[0].destination, rendered, 0o600)
		_, err = prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		assertManagementError(t, err, 65, "owned directory does not match an installed target")
	})
}
