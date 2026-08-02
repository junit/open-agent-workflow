package management

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func prepareUninstallWithoutWrites(t *testing.T, root string, environment Environment, request UninstallRequest) (PreparedUninstall, error) {
	t.Helper()
	before := snapshotPrepareTree(t, root)
	prepared, err := PrepareUninstall(environment, request)
	after := snapshotPrepareTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("PrepareUninstall() changed fixture:\nbefore=%#v\nafter=%#v", before, after)
	}
	return prepared, err
}

func TestPrepareUninstallMissingStateIsIdempotent(t *testing.T) {
	fixture := newPrepareFixture(t)
	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude,codex", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.plan.operation != mutationUninstall || len(prepared.plan.targetActions) != 0 || len(prepared.plan.stateActions) != 0 {
		t.Fatalf("plan = %#v", prepared.plan)
	}
	if got, want := prepared.plan.predicted.Lines, []string{"oaw: unchanged: claude", "oaw: unchanged: codex"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("predicted = %v, want %v", got, want)
	}
}

func TestPrepareUninstallPartialManagedTarget(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude,codex"})
	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := mutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, []string{"claude"}) {
		t.Fatalf("target actions = %v", got)
	}
	if prepared.plan.targetActions[0].effect != mutationRemove {
		t.Fatalf("target action = %#v", prepared.plan.targetActions[0])
	}
	if prepared.plan.policyAction.effect != mutationRetain {
		t.Fatalf("policy action = %#v", prepared.plan.policyAction)
	}
	if len(prepared.plan.stateActions) != 1 || prepared.plan.stateActions[0].effect != mutationReplace {
		t.Fatalf("state actions = %#v", prepared.plan.stateActions)
	}
	state, err := parseInstallationState(prepared.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if got := targetRecordIDs(state.targets); !reflect.DeepEqual(got, []string{"codex"}) {
		t.Fatalf("remaining targets = %v", got)
	}
	if state.version != parsePreparedState(t, installed.stateActions[0]).version {
		t.Fatalf("version changed: %q", state.version)
	}
}

func TestPrepareUninstallPreservesExistingManagedContent(t *testing.T) {
	for _, fixtureCase := range []struct {
		name     string
		personal []byte
	}{
		{name: "newline", personal: []byte("personal instruction\n")},
		{name: "no newline", personal: []byte("personal instruction")},
	} {
		t.Run(fixtureCase.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			target := filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md")
			writePrepareFile(t, target, fixtureCase.personal, 0o640)
			materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
			prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude"})
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.plan.targetActions) != 1 || prepared.plan.targetActions[0].effect != mutationReplace {
				t.Fatalf("target action = %#v", prepared.plan.targetActions)
			}
			if !bytes.Equal(prepared.plan.targetActions[0].data, fixtureCase.personal) {
				t.Fatalf("rendered = %q, want %q", prepared.plan.targetActions[0].data, fixtureCase.personal)
			}
			if prepared.plan.policyAction.effect != mutationRemove || len(prepared.plan.stateActions) != 1 || prepared.plan.stateActions[0].effect != mutationRemove {
				t.Fatalf("final actions policy=%#v state=%#v", prepared.plan.policyAction, prepared.plan.stateActions)
			}
		})
	}
}

func TestPrepareUninstallSharedDestinationUsesLastReference(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "shared project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "codex,opencode"})

	partial, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(partial.plan.targetActions) != 0 {
		t.Fatalf("partial shared actions = %#v", partial.plan.targetActions)
	}
	state, err := parseInstallationState(partial.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if got := targetRecordIDs(state.targets); !reflect.DeepEqual(got, []string{"opencode"}) {
		t.Fatalf("remaining targets = %v", got)
	}

	materializeMutationStateForTest(t, partial.plan.stateActions)
	final, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "opencode"})
	if err != nil {
		t.Fatal(err)
	}
	if len(final.plan.targetActions) != 1 || final.plan.targetActions[0].effect != mutationRemove {
		t.Fatalf("final shared action = %#v", final.plan.targetActions)
	}
}

func TestPrepareUninstallRetainsCrossScopePolicy(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeInstallRequest(t, fixture, InstallRequest{Targets: "codex"})
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})

	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.plan.policyAction.effect != mutationRetain || prepared.plan.stateActions[0].effect != mutationRemove {
		t.Fatalf("actions policy=%#v state=%#v", prepared.plan.policyAction, prepared.plan.stateActions)
	}
}

func TestPrepareUninstallRetainsPolicyForOlderValidCrossScopeReference(t *testing.T) {
	fixture := newPrepareFixture(t)
	user := materializeInstallRequest(t, fixture, InstallRequest{Targets: "codex"})
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})

	older := parsePreparedState(t, user.stateActions[0])
	older.version = "0.0.9"
	older.policyChecksum = checksumBytes([]byte("older canonical policy\n"))
	olderBytes, err := serializeInstallState(older)
	if err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, user.stateActions[0].destination, olderBytes, 0o600)

	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.plan.policyAction.effect != mutationRetain || prepared.plan.stateActions[0].effect != mutationRemove {
		t.Fatalf("actions policy=%#v state=%#v", prepared.plan.policyAction, prepared.plan.stateActions)
	}
}

func TestPrepareUninstallPlansOwnedDirectoriesDeepestFirst(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "owned project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	state := parsePreparedState(t, installed.stateActions[0])
	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "cursor", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.plan.directoryActions) != len(state.directories) {
		t.Fatalf("directory actions=%d, state directories=%d: %#v", len(prepared.plan.directoryActions), len(state.directories), prepared.plan.directoryActions)
	}
	for index := 1; index < len(prepared.plan.directoryActions); index++ {
		previous := prepared.plan.directoryActions[index-1].destination
		current := prepared.plan.directoryActions[index].destination
		if len(previous) < len(current) {
			t.Fatalf("directories are not deepest first: %q before %q", previous, current)
		}
	}
	if prepared.plan.targetActions[0].effect != mutationRemove || prepared.plan.policyAction.effect != mutationRemove || prepared.plan.stateActions[0].effect != mutationRemove {
		t.Fatalf("final effects target=%#v policy=%#v state=%#v", prepared.plan.targetActions, prepared.plan.policyAction, prepared.plan.stateActions)
	}
}

func TestPrepareUninstallDryRunPredictsRemovalWithLexicalRootAlias(t *testing.T) {
	fixture := newPrepareFixture(t)
	fixture.environment.Home = filepath.Dir(fixture.environment.Home) + string(filepath.Separator) + string(filepath.Separator) + filepath.Base(fixture.environment.Home)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.plan.targetActions) != 1 || prepared.plan.targetActions[0].effect != mutationRemove {
		t.Fatalf("target actions = %#v", prepared.plan.targetActions)
	}
	targetDirectory := filepath.Dir(installed.targetActions[0].destination)
	entries, err := os.ReadDir(targetDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(installed.targetActions[0].destination) {
		t.Fatalf("target directory entries = %#v", entries)
	}
	predictedDirectory := ""
	for _, action := range prepared.plan.directoryActions {
		if filepath.Clean(action.destination) == filepath.Clean(targetDirectory) {
			predictedDirectory = action.destination
			break
		}
	}
	if predictedDirectory == "" {
		t.Fatalf("directory actions = %#v, missing %q", prepared.plan.directoryActions, targetDirectory)
	}
	want := "oaw: would-remove-directory: " + predictedDirectory
	for _, line := range prepared.plan.predicted.Lines {
		if line == want {
			return
		}
	}
	t.Fatalf("predicted = %v, missing %q", prepared.plan.predicted.Lines, want)
}

func TestPrepareUninstallRejectsDriftWithoutWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude"})
	if err == nil {
		t.Fatal("drifted uninstall succeeded")
	}
	assertManagementError(t, err, 65, "managed target block has drifted: claude at "+installed.targetActions[0].destination)
}

func TestPrepareUninstallEveryInstalledTarget(t *testing.T) {
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
				prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: candidate.ID})
				if err != nil {
					t.Fatal(err)
				}
				if got := mutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, []string{candidate.ID}) {
					t.Fatalf("target actions = %v", got)
				}
				if prepared.plan.targetActions[0].effect != mutationRemove || prepared.plan.policyAction.effect != mutationRemove || prepared.plan.stateActions[0].effect != mutationRemove {
					t.Fatalf("effects target=%#v policy=%#v state=%#v", prepared.plan.targetActions, prepared.plan.policyAction, prepared.plan.stateActions)
				}
			})
		}
	}
}

func TestPrepareUninstallMissingSelectedTargetKeepsLiveState(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	prepared, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "codex", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.plan.targetActions) != 0 || prepared.plan.policyAction.effect != mutationRetain || prepared.plan.stateActions[0].effect != mutationReplace {
		t.Fatalf("plan = %#v", prepared.plan)
	}
	want := []string{"oaw: unchanged: codex", "oaw: unchanged: state"}
	if !reflect.DeepEqual(prepared.plan.predicted.Lines, want) {
		t.Fatalf("predicted = %v, want %v", prepared.plan.predicted.Lines, want)
	}
}

func TestPrepareUninstallRejectsInvalidInputsAndOwnership(t *testing.T) {
	t.Run("invalid target", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		_, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "unknown"})
		assertManagementError(t, err, 64, "unknown target 'unknown'")
	})

	t.Run("invalid environment", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		environment := fixture.environment
		environment.ConfigHome = "relative-config"
		_, err := prepareUninstallWithoutWrites(t, fixture.root, environment, UninstallRequest{Targets: "claude"})
		assertManagementError(t, err, 64, "root must be an absolute path: relative-config")
	})

	t.Run("untracked marker without state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		path := filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md")
		writePrepareFile(t, path, []byte(beginMarker+"\nforeign\n"+endMarker+"\n"), 0o644)
		_, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "untracked OAW markers already exist: claude at "+path)
	})

	t.Run("invalid state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		coords, err := initializeCoordinates(fixture.environment, resolvedRequest{scope: "user", targets: []string{"claude"}})
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, coords.stateFile, []byte("format\t2\n"), 0o600)
		_, err = prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "invalid state format")
	})

	t.Run("missing policy", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.Remove(installed.policyAction.destination); err != nil {
			t.Fatal(err)
		}
		_, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "managed policy is missing")
	})

	t.Run("invalid owned origin", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		installed := materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
		state, err := parseInstallationState(installed.stateActions[0].data)
		if err != nil {
			t.Fatal(err)
		}
		state.targets[0].origin = "existing-file"
		namespaces := make([]string, 0)
		for _, directory := range state.directories {
			if installNamespaceDirectory(installed.coordinates, directory) {
				namespaces = append(namespaces, directory)
			}
		}
		state.directories = namespaces
		rendered, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, installed.stateActions[0].destination, rendered, 0o600)
		_, err = prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "cursor"})
		assertManagementError(t, err, 65, "invalid owned target origin")
	})
}

func TestNewRemoveMutationActionRejectsDestinationOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.state")
	if _, err := newRemoveMutationAction("state", outside, root); err == nil {
		t.Fatal("outside remove action succeeded")
	}
}

func TestUninstallDirectoryOwnershipHelpersRejectUnboundPaths(t *testing.T) {
	fixture := newPrepareFixture(t)
	resolved := resolvedRequest{scope: "user", targets: []string{"claude"}}
	coords, err := initializeCoordinates(fixture.environment, resolved)
	if err != nil {
		t.Fatal(err)
	}
	unbound := filepath.Join(fixture.root, "unbound")
	state := installationState{scope: "user", targets: []targetRecord{{id: "claude", path: filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md"), mode: "managed-block", origin: "existing-file"}}}
	if directoryMatchesTargetRecords(unbound, state.targets, state, coords) {
		t.Fatal("unbound directory matched target records")
	}
	if _, err := ownedDirectoryMutationRoot(unbound, state, coords); err == nil {
		t.Fatal("unbound directory acquired a mutation root")
	}
}

func materializeMutationStateForTest(t *testing.T, actions []mutationAction) {
	t.Helper()
	for _, action := range actions {
		if action.effect == mutationReplace {
			writePrepareFile(t, action.destination, action.data, action.mode)
		}
	}
}
