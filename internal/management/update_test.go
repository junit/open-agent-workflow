package management

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectUpdateChangesOnlyManagedPolicySetContent(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	customProfile := filepath.Join(project, ".oaw", "profiles", "team-delivery.md")
	agents := filepath.Join(project, "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(customProfile), 0o755); err != nil {
		t.Fatal(err)
	}
	const customContent = "---\nid: team-delivery\nname: Team Delivery\n---\n"
	writePrepareFile(t, customProfile, []byte(customContent), 0o644)
	writePrepareFile(t, agents, []byte("user instructions\n"), 0o644)

	installed := policySetSource(t, "0.1.0", "\nold revision\n")
	request := InstallRequest{Project: project, Targets: "codex"}
	if _, err := Install(installed, fixture.environment, request); err != nil {
		t.Fatal(err)
	}
	updated := policySetSource(t, "0.2.0", "\nnew revision\n")
	if _, err := Update(updated, fixture.environment, UpdateRequest{Project: project, Targets: "codex"}); err != nil {
		t.Fatal(err)
	}

	for _, file := range updated.policySet {
		path := filepath.Join(project, ".oaw", "policy", filepath.FromSlash(file.Path))
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read updated Policy Set file %q: %v", file.Path, err)
		}
		if !bytes.Equal(got, file.Content) {
			t.Errorf("updated Policy Set file %q differs from source", file.Path)
		}
	}
	gotCustom, err := os.ReadFile(customProfile)
	if err != nil || string(gotCustom) != customContent {
		t.Fatalf("update changed Custom Profile: content=%q error=%v", gotCustom, err)
	}
	gotAgents, err := os.ReadFile(agents)
	if err != nil || !bytes.HasPrefix(gotAgents, []byte("user instructions\n")) {
		t.Fatalf("update changed surrounding Host instructions: content=%q error=%v", gotAgents, err)
	}
}

func TestUserUpdateChangesOnlyManagedPolicySetContent(t *testing.T) {
	fixture := newPrepareFixture(t)
	customProfile := filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "profiles", "team-delivery.md")
	agents := filepath.Join(fixture.environment.Home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(customProfile), 0o755); err != nil {
		t.Fatal(err)
	}
	const customContent = "---\nid: team-delivery\nname: Team Delivery\n---\n"
	writePrepareFile(t, customProfile, []byte(customContent), 0o644)
	writePrepareFile(t, agents, []byte("user instructions\n"), 0o644)

	installed := policySetSource(t, "0.1.0", "\nold revision\n")
	request := InstallRequest{Targets: "codex"}
	if _, err := Install(installed, fixture.environment, request); err != nil {
		t.Fatal(err)
	}
	updated := policySetSource(t, "0.2.0", "\nnew revision\n")
	if _, err := Update(updated, fixture.environment, UpdateRequest{Targets: "codex"}); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolve(CheckRequest{Targets: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	coords, err := initializeCoordinates(fixture.environment, resolved)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range updated.policySet {
		path, _, _, err := policySetDestination(coords, resolved, file.Path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read updated user Policy Set file %q: %v", file.Path, err)
		}
		if want := policySetFileContent(coords, file); !bytes.Equal(got, want) {
			t.Errorf("updated user Policy Set file %q differs from mapped source", file.Path)
		}
	}
	gotCustom, err := os.ReadFile(customProfile)
	if err != nil || string(gotCustom) != customContent {
		t.Fatalf("update changed user Custom Profile: content=%q error=%v", gotCustom, err)
	}
	gotAgents, err := os.ReadFile(agents)
	if err != nil || !bytes.HasPrefix(gotAgents, []byte("user instructions\n")) {
		t.Fatalf("update changed surrounding user Host instructions: content=%q error=%v", gotAgents, err)
	}
}

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
	updated := policySetSource(t, "0.2.0", "\nupdated canonical policy\n")

	prepared, err := prepareUpdateWithoutWrites(t, fixture.root, updated, fixture.environment, UpdateRequest{Targets: "claude,codex", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.plan.operation != mutationUpdate {
		t.Fatalf("operation = %v", prepared.plan.operation)
	}
	wantTargetLabels := []string{
		"claude/router", "claude/native-entrypoint",
		"codex/router", "codex/native-entrypoint", "codex/native-policy",
	}
	if got := artifactMutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, wantTargetLabels) {
		t.Fatalf("target labels = %v", got)
	}
	if prepared.plan.policyAction.effect != mutationReplace || !bytes.Contains(prepared.plan.policyAction.data, []byte("updated canonical policy")) {
		t.Fatalf("policy action = %#v", prepared.plan.policyAction)
	}
	if got := mutationActionLabels(prepared.plan.stateActions); !reflect.DeepEqual(got, []string{"state"}) {
		t.Fatalf("state labels = %v", got)
	}
	state, err := parseInstallationState(prepared.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if state.version != "0.2.0" || state.policyChecksum != checksumBytes(prepared.plan.policyAction.data) {
		t.Fatalf("updated state = %#v", state)
	}
	if !reflect.DeepEqual(state.directories, parsePreparedState(t, installed.stateActions[0]).directories) {
		t.Fatalf("directories changed: %v", state.directories)
	}
	wantLines := make([]string, 0, len(prepared.plan.predicted.Lines))
	for _, action := range prepared.plan.targetActions {
		wantLines = append(wantLines, predictMutationAction(action)...)
	}
	wantLines = append(wantLines, predictMutationAction(prepared.plan.policyAction)...)
	for _, action := range prepared.plan.policySetActions {
		wantLines = append(wantLines, predictMutationAction(action)...)
	}
	wantLines = append(wantLines, predictMutationAction(prepared.plan.stateActions[0])...)
	if !reflect.DeepEqual(prepared.plan.predicted.Lines, wantLines) {
		t.Fatalf("predicted = %#v, want %#v", prepared.plan.predicted.Lines, wantLines)
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
		assertManagementError(t, err, 65, "untracked OAW markers already exist: claude/router at "+path)
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
		if err == nil || !strings.Contains(err.Error(), "managed target block has drifted: claude/router at ") {
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
	updated := policySetSource(t, "0.2.0", "\nupdated policy\n")
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
				updated := policySetSource(t, "0.2.0", "\nupdated "+candidate.ID+" policy\n")
				prepared, err := prepareUpdateWithoutWrites(t, fixture.root, updated, fixture.environment, UpdateRequest{Project: project, Targets: candidate.ID})
				if err != nil {
					t.Fatal(err)
				}
				wantLabels := make([]string, 0, len(candidate.Artifacts))
				for _, artifact := range candidate.Artifacts {
					wantLabels = append(wantLabels, targetArtifactLabel(candidate.ID, artifact.ID))
				}
				if got := artifactMutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, wantLabels) {
					t.Fatalf("target actions = %v", got)
				}
				state, err := parseInstallationState(prepared.plan.stateActions[0].data)
				if err != nil {
					t.Fatal(err)
				}
				if state.version != "0.2.0" || len(state.targets) != len(candidate.Artifacts) {
					t.Fatalf("state = %#v", state)
				}
				if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, wantLabels) {
					t.Fatalf("state target artifacts = %v, want %v", got, wantLabels)
				}
			})
		}
	}
}

func TestPrepareUpdateSelectedHostIncludesEveryArtifactAndRetainsOthers(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude,codex"})
	updated := policySetSource(t, "0.2.0", "\nupdated policy\n")

	prepared, err := prepareUpdateWithoutWrites(
		t, fixture.root, updated, fixture.environment, UpdateRequest{Targets: "claude"},
	)
	if err != nil {
		t.Fatal(err)
	}
	wantSelected := []string{"claude/router", "claude/native-entrypoint"}
	if got := artifactMutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, wantSelected) {
		t.Fatalf("selected target actions = %v, want %v", got, wantSelected)
	}

	state, err := parseInstallationState(prepared.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	wantState := []string{
		"claude/router", "claude/native-entrypoint",
		"codex/router", "codex/native-entrypoint", "codex/native-policy",
	}
	if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, wantState) {
		t.Fatalf("updated state artifacts = %v, want %v", got, wantState)
	}
	if !reflect.DeepEqual(state.directories, parsePreparedState(t, installed.stateActions[0]).directories) {
		t.Fatalf("selective update changed owned directories: %v", state.directories)
	}
}

func TestMergeInstallRecordsPropagatesSharedDestinationChecksum(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := materializeInstallRequest(
		t, fixture, InstallRequest{Project: project, Targets: "codex,opencode"},
	)
	state := parsePreparedState(t, installed.stateActions[0])
	codexRouter, found := findTargetArtifactRecord(state.targets, "codex", routerArtifactID)
	if !found {
		t.Fatal("installed state has no Codex Router")
	}
	codexRouter.checksum = "9:9"

	merged, err := mergeInstallRecords(state.targets, []targetRecord{codexRouter}, "project")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"codex", "opencode"} {
		router, found := findTargetArtifactRecord(merged, id, routerArtifactID)
		if !found || router.path != codexRouter.path || router.checksum != codexRouter.checksum {
			t.Fatalf("shared %s Router = %#v", id, router)
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
			state.policyPath = filepath.Join(fixture.environment.ConfigHome, "other", "POLICY.md")
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
		assertManagementError(t, err, 65, "state is incomplete or duplicated")
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
		assertManagementError(t, err, 70, "canonical Policy Set source is invalid: Policy Set is missing required file \"POLICY.md\"")
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
	result := make([]string, 0, len(actions))
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		label := strings.SplitN(action.label, "/", 2)[0]
		if _, exists := seen[label]; exists {
			continue
		}
		seen[label] = struct{}{}
		result = append(result, label)
	}
	return result
}

func artifactMutationActionLabels(actions []mutationAction) []string {
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
