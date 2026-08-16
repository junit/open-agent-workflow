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

func TestProjectUninstallRemovesOnlyManagedPolicyContent(t *testing.T) {
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
	source := policySetSource(t, "0.1.0", "")
	if _, err := Install(source, fixture.environment, InstallRequest{Project: project, Targets: "codex"}); err != nil {
		t.Fatal(err)
	}

	if _, err := Uninstall(fixture.environment, UninstallRequest{Project: project, Targets: "codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(project, ".oaw", "policy")); !os.IsNotExist(err) {
		t.Fatalf("uninstall retained managed Policy Set directory: %v", err)
	}
	gotCustom, err := os.ReadFile(customProfile)
	if err != nil || string(gotCustom) != customContent {
		t.Fatalf("uninstall changed Custom Profile: content=%q error=%v", gotCustom, err)
	}
	gotAgents, err := os.ReadFile(agents)
	if err != nil || string(gotAgents) != "user instructions\n" {
		t.Fatalf("uninstall changed surrounding Host instructions: content=%q error=%v", gotAgents, err)
	}
	for _, native := range []string{
		filepath.Join(project, ".agents", "skills", "oaw", "SKILL.md"),
		filepath.Join(project, ".agents", "skills", "oaw", "agents", "openai.yaml"),
	} {
		if _, err := os.Lstat(native); !os.IsNotExist(err) {
			t.Fatalf("uninstall retained Codex native artifact %q: %v", native, err)
		}
	}
}

func TestUserUninstallRemovesOnlyManagedPolicyContent(t *testing.T) {
	fixture := newPrepareFixture(t)
	policyRoot := filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow")
	customProfile := filepath.Join(policyRoot, "profiles", "team-delivery.md")
	agents := filepath.Join(fixture.environment.Home, ".codex", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(customProfile), 0o755); err != nil {
		t.Fatal(err)
	}
	const customContent = "---\nid: team-delivery\nname: Team Delivery\n---\n"
	writePrepareFile(t, customProfile, []byte(customContent), 0o644)
	writePrepareFile(t, agents, []byte("user instructions\n"), 0o644)
	source := policySetSource(t, "0.1.0", "")
	if _, err := Install(source, fixture.environment, InstallRequest{Targets: "codex"}); err != nil {
		t.Fatal(err)
	}

	if _, err := Uninstall(fixture.environment, UninstallRequest{Targets: "codex"}); err != nil {
		t.Fatal(err)
	}
	for _, managed := range []string{
		filepath.Join(policyRoot, "POLICY.md"),
		filepath.Join(policyRoot, "cooperative-protocol.md"),
		filepath.Join(policyRoot, "adapters"),
		filepath.Join(policyRoot, "profiles", "builtin"),
	} {
		if _, err := os.Stat(managed); !os.IsNotExist(err) {
			t.Fatalf("uninstall retained managed user Policy Set content %q: %v", managed, err)
		}
	}
	gotCustom, err := os.ReadFile(customProfile)
	if err != nil || string(gotCustom) != customContent {
		t.Fatalf("uninstall changed user Custom Profile: content=%q error=%v", gotCustom, err)
	}
	gotAgents, err := os.ReadFile(agents)
	if err != nil || string(gotAgents) != "user instructions\n" {
		t.Fatalf("uninstall changed surrounding user Host instructions: content=%q error=%v", gotAgents, err)
	}
	for _, native := range []string{
		filepath.Join(fixture.environment.Home, ".agents", "skills", "oaw", "SKILL.md"),
		filepath.Join(fixture.environment.Home, ".agents", "skills", "oaw", "agents", "openai.yaml"),
	} {
		if _, err := os.Lstat(native); !os.IsNotExist(err) {
			t.Fatalf("uninstall retained user Codex native artifact %q: %v", native, err)
		}
	}
}

func TestUserUninstallRejectsStateClaimsOnCustomProfileContent(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(installationState, string) installationState
	}{
		{
			name: "profile file",
			mutate: func(state installationState, customProfile string) installationState {
				content, err := os.ReadFile(customProfile)
				if err != nil {
					t.Fatal(err)
				}
				state.policyFiles = append(state.policyFiles, policyFileRecord{
					path: customProfile, checksum: checksumBytes(content),
				})
				return state
			},
		},
		{
			name: "profile directory",
			mutate: func(state installationState, customProfile string) installationState {
				state.directories = append(state.directories, filepath.Dir(customProfile))
				return state
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			source := policySetSource(t, "0.1.0", "")
			installed, err := PrepareInstall(source, fixture.environment, InstallRequest{Targets: "codex"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ApplyInstall(installed); err != nil {
				t.Fatal(err)
			}
			customProfile := filepath.Join(
				fixture.environment.ConfigHome, "open-agent-workflow", "profiles", "team", "delivery.md",
			)
			writePrepareFile(t, customProfile, []byte("user-owned\n"), 0o644)
			state := parsePreparedState(t, installed.stateActions[0])
			state = tt.mutate(state, customProfile)
			rendered, err := serializeInstallState(state)
			if err != nil {
				t.Fatal(err)
			}
			writePrepareFile(t, installed.coordinates.stateFile, rendered, 0o600)

			if _, err := Uninstall(fixture.environment, UninstallRequest{Targets: "codex"}); err == nil {
				t.Fatal("uninstall accepted a state claim on user Custom Profile content")
			}
			content, err := os.ReadFile(customProfile)
			if err != nil || string(content) != "user-owned\n" {
				t.Fatalf("uninstall changed Custom Profile content: content=%q error=%v", content, err)
			}
		})
	}
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
	if got := uninstallActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, []string{"claude/router", "claude/native-entrypoint"}) {
		t.Fatalf("target actions = %v", got)
	}
	for _, action := range prepared.plan.targetActions {
		if action.effect != mutationRemove {
			t.Fatalf("target action = %#v", action)
		}
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
	if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, []string{"codex/router", "codex/native-entrypoint", "codex/native-policy"}) {
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
			if got := uninstallActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, []string{"claude/router", "claude/native-entrypoint"}) {
				t.Fatalf("target action = %#v", prepared.plan.targetActions)
			}
			if prepared.plan.targetActions[0].effect != mutationReplace || prepared.plan.targetActions[1].effect != mutationRemove {
				t.Fatalf("target effects = %#v", prepared.plan.targetActions)
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
	if got := uninstallActionLabels(partial.plan.targetActions); !reflect.DeepEqual(got, []string{"codex/native-entrypoint", "codex/native-policy"}) {
		t.Fatalf("partial shared actions = %#v", partial.plan.targetActions)
	}
	for _, action := range partial.plan.targetActions {
		if action.effect != mutationRemove {
			t.Fatalf("partial target was not removed: %#v", action)
		}
	}
	state, err := parseInstallationState(partial.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, []string{"opencode/router", "opencode/native-entrypoint"}) {
		t.Fatalf("remaining targets = %v", got)
	}

	if _, err := ApplyUninstall(partial); err != nil {
		t.Fatal(err)
	}
	final, err := prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "opencode"})
	if err != nil {
		t.Fatal(err)
	}
	if got := uninstallActionLabels(final.plan.targetActions); !reflect.DeepEqual(got, []string{"opencode/router", "opencode/native-entrypoint"}) {
		t.Fatalf("final shared action = %#v", final.plan.targetActions)
	}
	for _, action := range final.plan.targetActions {
		if action.effect != mutationRemove {
			t.Fatalf("final target was not removed: %#v", action)
		}
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
	if len(prepared.plan.targetActions) != len(installed.targetActions) || prepared.plan.policyAction.effect != mutationRemove || prepared.plan.stateActions[0].effect != mutationRemove {
		t.Fatalf("final effects target=%#v policy=%#v state=%#v", prepared.plan.targetActions, prepared.plan.policyAction, prepared.plan.stateActions)
	}
	for _, action := range prepared.plan.targetActions {
		if action.effect != mutationRemove {
			t.Fatalf("target action was not removed: %#v", action)
		}
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
	if got := uninstallActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, []string{"claude/router", "claude/native-entrypoint"}) {
		t.Fatalf("target actions = %#v", prepared.plan.targetActions)
	}
	for _, action := range prepared.plan.targetActions {
		if action.effect != mutationRemove {
			t.Fatalf("target action was not removed: %#v", action)
		}
	}
	targetDirectory := filepath.Dir(installed.targetActions[0].destination)
	entries, err := os.ReadDir(targetDirectory)
	if err != nil {
		t.Fatal(err)
	}
	entryNames := make([]string, len(entries))
	for index, entry := range entries {
		entryNames[index] = entry.Name()
	}
	if want := []string{filepath.Base(installed.targetActions[0].destination), "skills"}; !reflect.DeepEqual(entryNames, want) {
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
	assertManagementError(t, err, 65, "managed target block has drifted: claude/router at "+installed.targetActions[0].destination)
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
				wantLabels := make([]string, 0, len(candidate.Artifacts))
				for _, artifact := range candidate.Artifacts {
					wantLabels = append(wantLabels, targetArtifactLabel(candidate.ID, artifact.ID))
				}
				if got := uninstallActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, wantLabels) {
					t.Fatalf("target actions = %v", got)
				}
				if len(prepared.plan.targetActions) != len(candidate.Artifacts) || prepared.plan.policyAction.effect != mutationRemove || prepared.plan.stateActions[0].effect != mutationRemove {
					t.Fatalf("effects target=%#v policy=%#v state=%#v", prepared.plan.targetActions, prepared.plan.policyAction, prepared.plan.stateActions)
				}
				for _, action := range prepared.plan.targetActions {
					if action.effect != mutationRemove {
						t.Fatalf("target action was not removed: %#v", action)
					}
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
		assertManagementError(t, err, 65, "untracked OAW markers already exist: claude/router at "+path)
	})

	t.Run("invalid state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		coords, err := initializeCoordinates(fixture.environment, resolvedRequest{scope: "user", targets: []string{"claude"}})
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, coords.stateFile, []byte("format\t2\n"), 0o600)
		_, err = prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Targets: "claude"})
		assertManagementError(t, err, 65, "state is incomplete or duplicated")
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
		rendered, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		rendered = bytes.Replace(rendered, []byte("\tcreated-file\n"), []byte("\texisting-file\n"), 1)
		writePrepareFile(t, installed.stateActions[0].destination, rendered, 0o600)
		_, err = prepareUninstallWithoutWrites(t, fixture.root, fixture.environment, UninstallRequest{Project: project, Targets: "cursor"})
		assertManagementError(t, err, 65, "invalid target ownership")
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
	state := installationState{scope: "user", targets: []targetRecord{
		{id: "claude", artifact: routerArtifactID, path: filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md"), mode: "managed-block", origin: "existing-file"},
		{id: "claude", artifact: nativeEntrypointArtifactID, path: filepath.Join(fixture.environment.Home, ".claude", "skills", "oaw", "SKILL.md"), mode: "owned-file", origin: "created-file"},
	}}
	if directoryMatchesTargetRecords(unbound, state.targets, state, coords) {
		t.Fatal("unbound directory matched target records")
	}
	if _, err := ownedDirectoryMutationRoot(unbound, state, coords); err == nil {
		t.Fatal("unbound directory acquired a mutation root")
	}
}

func uninstallActionLabels(actions []mutationAction) []string {
	result := make([]string, len(actions))
	for index, action := range actions {
		result[index] = action.label
	}
	return result
}
