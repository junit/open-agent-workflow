package management

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func prepareUpdateWithoutWrites(t *testing.T, root string, source Source, environment Environment, request UpdateRequest) (PreparedUpdate, error) {
	t.Helper()
	before := snapshotPrepareTree(t, root)
	prepared, err := PrepareUpdate(source, environment, request)
	after := snapshotPrepareTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("PrepareUpdate() changed fixture:\nbefore=%#v\nafter=%#v", before, after)
	}
	return prepared, err
}

func materializeInstallRequest(t *testing.T, fixture prepareFixture, request InstallRequest) PreparedInstall {
	t.Helper()
	prepared, err := PrepareInstall(fixture.source, fixture.environment, request)
	if err != nil {
		t.Fatal(err)
	}
	materializePreparedFixture(t, prepared)
	return prepared
}

func TestPrepareUpdateUsesCurrentCheckoutWithoutWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude,codex"})
	updated, err := NewSource("0.2.0", []byte("updated canonical policy\n"))
	if err != nil {
		t.Fatal(err)
	}

	prepared, err := prepareUpdateWithoutWrites(t, fixture.root, updated, fixture.environment, UpdateRequest{Targets: "claude,codex", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.plan.operation != mutationUpdate {
		t.Fatalf("operation = %v", prepared.plan.operation)
	}
	if got := mutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Fatalf("target labels = %v", got)
	}
	if prepared.plan.policyAction.effect != mutationReplace || !bytes.Equal(prepared.plan.policyAction.data, []byte("updated canonical policy\n")) {
		t.Fatalf("policy action = %#v", prepared.plan.policyAction)
	}
	if got := mutationActionLabels(prepared.plan.stateActions); !reflect.DeepEqual(got, []string{"state"}) {
		t.Fatalf("state labels = %v", got)
	}
	state, err := parseInstallationState(prepared.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if state.version != "0.2.0" || state.policyChecksum != checksumBytes([]byte("updated canonical policy\n")) {
		t.Fatalf("updated state = %#v", state)
	}
	if !reflect.DeepEqual(state.directories, parsePreparedState(t, installed.stateActions[0]).directories) {
		t.Fatalf("directories changed: %v", state.directories)
	}
	wantLines := []string{
		"oaw: unchanged: claude",
		"oaw: unchanged: codex",
		"oaw: would-update: " + prepared.plan.coordinates.policyPath,
		"oaw: would-update: " + prepared.plan.coordinates.stateFile,
	}
	if !reflect.DeepEqual(prepared.plan.predicted.Lines, wantLines) {
		t.Fatalf("predicted = %#v, want %#v", prepared.plan.predicted.Lines, wantLines)
	}
}

func TestUpdateMigratesLegacyManagedBlockToActivationRouter(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	legacyBlock := []byte(beginMarker + "\n" +
		"Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:\n" +
		"@" + installed.policyAction.destination + "\n" + endMarker + "\n")
	targetBytes := append([]byte("before\n"), legacyBlock...)
	targetBytes = append(targetBytes, []byte("after\n")...)
	writePrepareFile(t, installed.targetActions[0].destination, targetBytes, 0o644)

	state, err := parseInstallationState(installed.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	for index := range state.targets {
		if state.targets[index].id == "claude" {
			state.targets[index].checksum = checksumBytes(legacyBlock)
		}
	}
	renderedState, err := serializeInstallState(state)
	if err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, installed.stateActions[0].destination, renderedState, 0o600)

	updated, err := NewSource("0.2.0", []byte("updated canonical policy\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Update(updated, fixture.environment, UpdateRequest{Targets: "claude"}); err != nil {
		t.Fatal(err)
	}

	updatedTarget, err := os.ReadFile(installed.targetActions[0].destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(updatedTarget, []byte("before\n")) || !bytes.HasSuffix(updatedTarget, []byte("after\n")) {
		t.Fatalf("update did not preserve sentinels: %q", updatedTarget)
	}
	if bytes.Count(updatedTarget, []byte(beginMarker)) != 1 || bytes.Count(updatedTarget, []byte(endMarker)) != 1 {
		t.Fatalf("update did not retain exactly one marker pair: %q", updatedTarget)
	}
	if bytes.Contains(updatedTarget, []byte("Before any new top-level engineering task that may use workflow skills")) ||
		bytes.Contains(updatedTarget, []byte("\n@"+installed.policyAction.destination+"\n")) ||
		!bytes.Contains(updatedTarget, []byte("Open Agent Workflow is opt-in.")) {
		t.Fatalf("update did not migrate the legacy block: %q", updatedTarget)
	}

	expectedBlock, err := renderManagedBlock("claude", "user", installed.policyAction.destination)
	if err != nil {
		t.Fatal(err)
	}
	updatedState, exists, err := readInstallationState(installed.stateActions[0].destination)
	if err != nil || !exists {
		t.Fatalf("updated state: exists=%v err=%v", exists, err)
	}
	if got := findPreparedRecord(t, updatedState.targets, "claude").checksum; got != checksumBytes(expectedBlock) {
		t.Fatalf("updated checksum = %q, want %q", got, checksumBytes(expectedBlock))
	}

	targetSnapshot := append([]byte(nil), updatedTarget...)
	policySnapshot, err := os.ReadFile(installed.policyAction.destination)
	if err != nil {
		t.Fatal(err)
	}
	stateSnapshot, err := os.ReadFile(installed.stateActions[0].destination)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Update(updated, fixture.environment, UpdateRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"oaw: unchanged: claude", "oaw: unchanged: policy", "oaw: unchanged: state"}; !reflect.DeepEqual(result.Lines, want) {
		t.Fatalf("repeat update result = %v, want %v", result.Lines, want)
	}
	for _, snapshot := range []struct {
		name string
		path string
		data []byte
	}{
		{"target", installed.targetActions[0].destination, targetSnapshot},
		{"policy", installed.policyAction.destination, policySnapshot},
		{"state", installed.stateActions[0].destination, stateSnapshot},
	} {
		current, err := os.ReadFile(snapshot.path)
		if err != nil || !bytes.Equal(current, snapshot.data) {
			t.Fatalf("repeat update changed %s: %q, %v", snapshot.name, current, err)
		}
	}
}

func TestPrepareUpdateNormalizesSharedDestinationAndCoordinatesStates(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeInstallRequest(t, fixture, InstallRequest{Targets: "codex"})
	project := filepath.Join(fixture.root, "project with shared state")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	projectInstall := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "codex,opencode"})
	userTarget := filepath.Join(fixture.environment.Home, ".codex", "AGENTS.md")
	userTargetBefore, err := os.ReadFile(userTarget)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := NewSource("0.2.0", []byte("coordinated policy\n"))
	if err != nil {
		t.Fatal(err)
	}

	prepared, err := prepareUpdateWithoutWrites(t, fixture.root, updated, fixture.environment, UpdateRequest{Project: project, Targets: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if got := mutationActionLabels(prepared.plan.stateActions); !reflect.DeepEqual(got, []string{"state", "state-reference-1"}) {
		t.Fatalf("state labels = %v", got)
	}
	projectState, err := parseInstallationState(prepared.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	codex := findPreparedRecord(t, projectState.targets, "codex")
	opencode := findPreparedRecord(t, projectState.targets, "opencode")
	if codex.path != opencode.path || codex.checksum != opencode.checksum {
		t.Fatalf("shared records not normalized: %#v %#v", codex, opencode)
	}
	userState, err := parseInstallationState(prepared.plan.stateActions[1].data)
	if err != nil {
		t.Fatal(err)
	}
	if userState.version != "0.2.0" || userState.policyChecksum != checksumBytes([]byte("coordinated policy\n")) {
		t.Fatalf("coordinated state = %#v", userState)
	}
	if got, err := os.ReadFile(userTarget); err != nil || !bytes.Equal(got, userTargetBefore) {
		t.Fatalf("other-scope adapter changed: %q, %v", got, err)
	}
	if len(projectInstall.stateActions) != 2 {
		t.Fatalf("project install state actions = %d", len(projectInstall.stateActions))
	}
}

func TestPrepareUpdateRejectsMissingUninstalledAndDriftedStateWithoutWrites(t *testing.T) {
	t.Run("missing state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 66, "no installation state; run install first")
	})

	t.Run("untracked marker without state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		path := filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md")
		writePrepareFile(t, path, []byte(beginMarker+"\nforeign\n"+endMarker+"\n"), 0o644)
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "untracked OAW markers already exist: claude at "+path)
	})

	t.Run("selected target not installed", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "codex"})
		assertManagementError(t, err, 65, "selected target is not installed: codex")
	})

	t.Run("policy drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		prepared := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.WriteFile(prepared.policyAction.destination, []byte("policy drift\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "managed policy has drifted")
	})

	t.Run("target drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		prepared := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.WriteFile(prepared.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "managed target block has drifted: claude at ") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestPrepareUpdatePreservesExistingBackupReference(t *testing.T) {
	fixture := newPrepareFixture(t)
	prepared := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	state, err := parseInstallationState(prepared.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	state.backupPath = filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "backups", "existing")
	rendered, err := serializeInstallState(state)
	if err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, prepared.stateActions[0].destination, rendered, 0o600)
	updated, err := NewSource("0.2.0", []byte("updated policy\n"))
	if err != nil {
		t.Fatal(err)
	}
	update, err := prepareUpdateWithoutWrites(t, fixture.root, updated, fixture.environment, UpdateRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseInstallationState(update.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if got.backupPath != state.backupPath {
		t.Fatalf("backup = %q, want %q", got.backupPath, state.backupPath)
	}
}

func TestPrepareUpdateEveryInstalledTarget(t *testing.T) {
	for _, candidate := range targetRegistry {
		candidate := candidate
		scopes := []string{"project"}
		if candidate.User {
			scopes = append([]string{"user"}, scopes...)
		}
		for _, selectedScope := range scopes {
			selectedScope := selectedScope
			t.Run(selectedScope+"/"+candidate.ID, func(t *testing.T) {
				fixture := newPrepareFixture(t)
				project := ""
				if selectedScope == "project" {
					project = filepath.Join(fixture.root, "project "+candidate.ID)
					if err := os.Mkdir(project, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: candidate.ID})
				updated, err := NewSource("0.2.0", []byte("updated "+candidate.ID+" policy\n"))
				if err != nil {
					t.Fatal(err)
				}
				prepared, err := prepareUpdateWithoutWrites(t, fixture.root, updated, fixture.environment, UpdateRequest{Project: project, Targets: candidate.ID})
				if err != nil {
					t.Fatal(err)
				}
				if got := mutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, []string{candidate.ID}) {
					t.Fatalf("target actions = %v", got)
				}
				state, err := parseInstallationState(prepared.plan.stateActions[0].data)
				if err != nil {
					t.Fatal(err)
				}
				if state.version != "0.2.0" || len(state.targets) != 1 || state.targets[0].id != candidate.ID {
					t.Fatalf("state = %#v", state)
				}
			})
		}
	}
}

func TestPrepareUpdateRejectsInvalidMutationBindings(t *testing.T) {
	tests := []struct {
		name    string
		project bool
		mutate  func(t *testing.T, fixture prepareFixture, installed PreparedInstall, state *installationState)
		status  int
		message string
	}{
		{name: "scope mismatch", mutate: func(t *testing.T, fixture prepareFixture, installed PreparedInstall, state *installationState) {
			state.scope = "project"
			state.project = filepath.Join(fixture.root, "other project")
		}, status: 65, message: "installed scope does not match"},
		{name: "policy path mismatch", mutate: func(t *testing.T, fixture prepareFixture, installed PreparedInstall, state *installationState) {
			state.policyPath = filepath.Join(fixture.environment.ConfigHome, "other", "ENGINEERING.md")
		}, status: 65, message: "installed policy path does not match"},
		{name: "unowned directory", mutate: func(t *testing.T, fixture prepareFixture, installed PreparedInstall, state *installationState) {
			state.directories = append(state.directories, filepath.Join(fixture.root, "unowned"))
		}, status: 65, message: "owned directory does not match an installed target"},
		{name: "project root mismatch", project: true, mutate: func(t *testing.T, fixture prepareFixture, installed PreparedInstall, state *installationState) {
			state.project = filepath.Join(fixture.root, "different project")
		}, status: 65, message: "installed project root does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			project := ""
			if tt.project {
				project = filepath.Join(fixture.root, "project")
				if err := os.Mkdir(project, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "claude"})
			state, err := parseInstallationState(installed.stateActions[0].data)
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(t, fixture, installed, &state)
			rendered, err := serializeInstallState(state)
			if err != nil {
				t.Fatal(err)
			}
			writePrepareFile(t, installed.stateActions[0].destination, rendered, 0o600)
			_, err = prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Project: project, Targets: "claude"})
			assertManagementError(t, err, tt.status, tt.message)
		})
	}

	t.Run("missing policy", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.Remove(installed.policyAction.destination); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "managed policy is missing")
	})

	t.Run("invalid state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		coords, err := initializeCoordinates(fixture.environment, resolvedRequest{scope: "user", targets: []string{"claude"}})
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, coords.stateFile, []byte("format\t2\n"), 0o600)
		_, err = prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "invalid state format")
	})

	t.Run("invalid request", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "unknown"})
		assertManagementError(t, err, 64, "unknown target 'unknown'")
	})

	t.Run("invalid environment", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		environment := fixture.environment
		environment.StateHome = "relative-state"
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 64, "root must be an absolute path: relative-state")
	})

	t.Run("owned target without state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Project: project, Targets: "cursor"})
		assertManagementError(t, err, 66, "no installation state; run install first")
	})

	t.Run("invalid source", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		_, err := prepareUpdateWithoutWrites(t, fixture.root, Source{}, fixture.environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 70, "VERSION is invalid")
		_, err = prepareUpdateWithoutWrites(t, fixture.root, Source{version: "0.2.0"}, fixture.environment, UpdateRequest{Targets: "claude"})
		assertManagementError(t, err, 70, "canonical policy source is invalid")
	})

	t.Run("untracked destination through symlink", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		outside := filepath.Join(fixture.root, "outside")
		if err := os.Mkdir(outside, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(fixture.environment.Home, ".claude")); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUpdateWithoutWrites(t, fixture.root, fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "destination path contains a symlink") {
			t.Fatalf("error = %v", err)
		}
	})
}

func mutationActionLabels(actions []mutationAction) []string {
	result := make([]string, len(actions))
	for index, action := range actions {
		result[index] = action.label
	}
	return result
}

func assertManagementError(t *testing.T, err error, status int, message string) {
	t.Helper()
	managementError, ok := err.(*Error)
	if !ok || managementError.Status != status || managementError.Message != message {
		t.Fatalf("error = %#v, want status=%d message=%q", err, status, message)
	}
}
