package management

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyStateCheckReportsUpgradeRequiredAndConflicts(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})

	result, err := Check(fixture.environment, CheckRequest{Targets: "claude"})
	if err != nil || !resultContainsLine(result, "installed claude: upgrade-required") {
		t.Fatalf("legacy Check() result=%#v error=%v", result, err)
	}

	native := findInstallActionByLabel(t, installed.targetActions, "claude/native-entrypoint")
	writePrepareFile(t, native.destination, []byte("foreign dispatcher\n"), 0o644)
	result, err = Check(fixture.environment, CheckRequest{Targets: "claude"})
	if err != nil || !resultContainsLine(result, "installed claude: drift") {
		t.Fatalf("legacy conflict Check() result=%#v error=%v", result, err)
	}
}

func TestInstallMigratesEveryLegacyTargetToFormatTwo(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude,codex"})

	prepared, err := prepareWithoutWrites(
		t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"claude/router", "claude/native-entrypoint",
		"codex/router", "codex/native-entrypoint", "codex/native-policy",
	}
	if got := artifactActionLabels(prepared.targetActions); !reflect.DeepEqual(got, want) {
		t.Fatalf("migration actions = %v, want %v", got, want)
	}
	if !bytes.HasPrefix(prepared.stateActions[0].data, []byte("format\t2\n")) {
		t.Fatalf("migrated state is not format 2: %q", prepared.stateActions[0].data)
	}
	state := parsePreparedState(t, prepared.stateActions[0])
	if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, want) {
		t.Fatalf("migrated artifacts = %v, want %v", got, want)
	}
	if _, err := ApplyInstall(prepared); err != nil {
		t.Fatal(err)
	}
	result, err := Check(fixture.environment, CheckRequest{Targets: "claude,codex"})
	if err != nil || !resultContainsLine(result, "installed claude: clean") || !resultContainsLine(result, "installed codex: clean") {
		t.Fatalf("migrated Check() result=%#v error=%v", result, err)
	}
}

func TestLegacyInstallRefusesForeignNativeEntrypointWithoutWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
	native := findInstallActionByLabel(t, installed.targetActions, "claude/native-entrypoint")
	writePrepareFile(t, native.destination, []byte("foreign dispatcher\n"), 0o644)

	_, err := prepareWithoutWrites(
		t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"},
	)
	assertManagementError(t, err, 65, "owned target artifact already exists: "+native.destination)
}

func TestUpdateMigratesEveryLegacyTargetAndCreatesOwnedDirectories(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude,codex"})
	updated := policySetSource(t, "0.2.0", "\nupdated during migration\n")

	prepared, err := prepareUpdateWithoutWrites(
		t, fixture.root, updated, fixture.environment, UpdateRequest{Targets: "claude"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"claude/router", "claude/native-entrypoint",
		"codex/router", "codex/native-entrypoint", "codex/native-policy",
	}
	if got := artifactMutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, want) {
		t.Fatalf("migration actions = %v, want %v", got, want)
	}
	if !bytes.HasPrefix(prepared.plan.stateActions[0].data, []byte("format\t2\n")) {
		t.Fatalf("migrated state is not format 2: %q", prepared.plan.stateActions[0].data)
	}
	if _, err := ApplyUpdate(prepared); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"claude/native-entrypoint", "codex/native-entrypoint", "codex/native-policy"} {
		action := findMutationActionByLabel(t, prepared.plan.targetActions, label)
		if current, err := inspectInstallPath(action.destination); err != nil || current.kind != installPathRegular {
			t.Fatalf("created %s = %#v, error=%v", label, current, err)
		}
	}
	result, err := Check(fixture.environment, CheckRequest{Targets: "claude,codex"})
	if err != nil || !resultContainsLine(result, "installed claude: clean") || !resultContainsLine(result, "installed codex: clean") {
		t.Fatalf("migrated Check() result=%#v error=%v", result, err)
	}
}

func TestLegacyUpdateRollsBackCreatedArtifactsAndDirectories(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
	before := snapshotPrepareTree(t, fixture.root)
	prepared, err := PrepareUpdate(
		fixture.source, fixture.environment, UpdateRequest{Targets: "claude"},
	)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected migration failure")
	_, err = applyMutationPlan(prepared.plan, func(point mutationFaultPoint) error {
		if point.phase == mutationPhasePolicy && point.moment == mutationBefore {
			return injected
		}
		return nil
	})
	if !errors.Is(err, injected) {
		t.Fatalf("ApplyUpdate() error = %v, want %v", err, injected)
	}
	after := snapshotPrepareTree(t, fixture.root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy migration rollback differs:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestLegacyUpdateRollbackPreservesReplacedCreatedDirectory(t *testing.T) {
	fixture := newPrepareFixture(t)
	materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
	prepared, err := PrepareUpdate(
		fixture.source, fixture.environment, UpdateRequest{Targets: "claude"},
	)
	if err != nil {
		t.Fatal(err)
	}

	nativeIndex := -1
	var native mutationAction
	for index, action := range prepared.plan.targetActions {
		if action.label == "claude/native-entrypoint" {
			nativeIndex = index
			native = action
			break
		}
	}
	if nativeIndex < 0 {
		t.Fatal("legacy update has no Claude native entrypoint action")
	}
	replacedDirectory := filepath.Dir(native.destination)
	injected := errors.New("injected directory replacement")
	var replacementIdentity os.FileInfo

	_, err = applyMutationPlan(prepared.plan, func(point mutationFaultPoint) error {
		if point.phase != mutationPhaseTarget || point.moment != mutationAfter || point.index != nativeIndex {
			return nil
		}
		if removeErr := os.RemoveAll(replacedDirectory); removeErr != nil {
			t.Fatal(removeErr)
		}
		if mkdirErr := os.Mkdir(replacedDirectory, 0o700); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		replacementIdentity, err = os.Lstat(replacedDirectory)
		if err != nil {
			t.Fatal(err)
		}
		return injected
	})
	if err == nil || !strings.Contains(err.Error(), injected.Error()) ||
		!strings.Contains(err.Error(), "created owned directory identity changed during rollback") {
		t.Fatalf("ApplyUpdate() error = %v, want injected rollback identity failure", err)
	}
	current, statErr := os.Lstat(replacedDirectory)
	if statErr != nil {
		t.Fatalf("foreign replacement directory was removed: %v", statErr)
	}
	if !os.SameFile(replacementIdentity, current) {
		t.Fatal("foreign replacement directory identity changed during rollback")
	}
}

func TestLegacyUpdateRejectsTamperedOrRacedDirectoryPlanBeforeWrites(t *testing.T) {
	t.Run("tampered identity", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
		prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		if len(prepared.plan.plannedDirectories) == 0 {
			t.Fatal("legacy update has no planned directories")
		}
		plan := cloneMutationPlan(prepared.plan)
		plan.plannedDirectories[0].identity = mutationPathIdentity{}
		before := snapshotPrepareTree(t, fixture.root)
		if _, err := applyMutationPlan(plan, nil); err == nil || !strings.Contains(err.Error(), "identity changed after preparation") {
			t.Fatalf("tampered directory plan error = %v", err)
		}
		if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(after, before) {
			t.Fatalf("tampered directory plan changed tree:\nbefore=%#v\nafter=%#v", before, after)
		}
	})

	t.Run("directory appeared", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
		prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		appeared := prepared.plan.plannedDirectories[len(prepared.plan.plannedDirectories)-1].destination
		if err := os.MkdirAll(appeared, 0o755); err != nil {
			t.Fatal(err)
		}
		before := snapshotPrepareTree(t, fixture.root)
		if _, err := ApplyUpdate(prepared); err == nil || !strings.Contains(err.Error(), "changed after preparation") {
			t.Fatalf("raced directory plan error = %v", err)
		}
		if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(after, before) {
			t.Fatalf("raced directory plan changed tree:\nbefore=%#v\nafter=%#v", before, after)
		}
	})
}

func TestLegacyUpdateForceRefusesForeignNativeEntrypoint(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
	native := findInstallActionByLabel(t, installed.targetActions, "claude/native-entrypoint")
	writePrepareFile(t, native.destination, []byte("foreign dispatcher\n"), 0o644)

	_, err := prepareUpdateWithoutWrites(
		t, fixture.root, fixture.source, fixture.environment,
		UpdateRequest{Targets: "claude", Force: true},
	)
	assertManagementError(t, err, 65, "owned target artifact already exists: "+native.destination)
	content, readErr := os.ReadFile(native.destination)
	if readErr != nil || string(content) != "foreign dispatcher\n" {
		t.Fatalf("foreign native entrypoint changed: content=%q error=%v", content, readErr)
	}
}

func TestLegacyPartialUninstallPreservesFormatOneAndUnownedNativeFile(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude,codex"})
	native := findInstallActionByLabel(t, installed.targetActions, "claude/native-entrypoint")
	writePrepareFile(t, native.destination, []byte("foreign dispatcher\n"), 0o644)

	if _, err := Uninstall(fixture.environment, UninstallRequest{Targets: "claude"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(installed.coordinates.stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte("format\t1\n")) {
		t.Fatalf("partial uninstall promoted legacy state: %q", raw)
	}
	state, err := parseInstallationState(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, []string{"codex/router"}) {
		t.Fatalf("remaining legacy records = %v", got)
	}
	content, err := os.ReadFile(native.destination)
	if err != nil || string(content) != "foreign dispatcher\n" {
		t.Fatalf("unowned native entrypoint changed: content=%q error=%v", content, err)
	}
}

func TestProjectLegacyUpdateMigratesSharedRouterTargets(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "legacy project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	materializeLegacyInstall(
		t, fixture, InstallRequest{Project: project, Targets: "codex,opencode"},
	)

	prepared, err := PrepareUpdate(
		fixture.source, fixture.environment, UpdateRequest{Project: project, Targets: "opencode"},
	)
	if err != nil {
		t.Fatal(err)
	}
	wantActions := []string{
		"codex/router", "codex/native-entrypoint", "codex/native-policy",
		"opencode/native-entrypoint",
	}
	if got := artifactMutationActionLabels(prepared.plan.targetActions); !reflect.DeepEqual(got, wantActions) {
		t.Fatalf("shared migration actions = %v, want %v", got, wantActions)
	}
	wantState := []string{
		"codex/router", "codex/native-entrypoint", "codex/native-policy",
		"opencode/router", "opencode/native-entrypoint",
	}
	state, err := parseInstallationState(prepared.plan.stateActions[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, wantState) {
		t.Fatalf("shared migration state = %v, want %v", got, wantState)
	}
	if _, err := ApplyUpdate(prepared); err != nil {
		t.Fatal(err)
	}
	result, err := Check(
		fixture.environment, CheckRequest{Project: project, Targets: "codex,opencode"},
	)
	if err != nil || !resultContainsLine(result, "installed codex: clean") || !resultContainsLine(result, "installed opencode: clean") {
		t.Fatalf("shared migration Check() result=%#v error=%v", result, err)
	}
}

func TestLegacyUpdateForceRepairsRouterAndCompletesMigration(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
	router := findInstallActionByLabel(t, installed.targetActions, "claude/router")
	writePrepareFile(
		t, router.destination, []byte(beginMarker+"\ndrifted legacy router\n"+endMarker+"\n"), 0o644,
	)

	prepared, err := PrepareUpdate(
		fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.plan.backup.required {
		t.Fatalf("forced legacy migration has no backup: %#v", prepared.plan.backup)
	}
	if _, err := ApplyUpdate(prepared); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(installed.coordinates.stateFile)
	if err != nil || !bytes.HasPrefix(raw, []byte("format\t2\n")) {
		t.Fatalf("forced migration state=%q error=%v", raw, err)
	}
	result, err := Check(fixture.environment, CheckRequest{Targets: "claude"})
	if err != nil || !resultContainsLine(result, "installed claude: clean") {
		t.Fatalf("forced migration Check() result=%#v error=%v", result, err)
	}
}

func TestLegacyFullUninstallLeavesUnownedNativeFile(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
	native := findInstallActionByLabel(t, installed.targetActions, "claude/native-entrypoint")
	writePrepareFile(t, native.destination, []byte("foreign dispatcher\n"), 0o644)

	if _, err := Uninstall(fixture.environment, UninstallRequest{Targets: "claude"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installed.coordinates.stateFile); !os.IsNotExist(err) {
		t.Fatalf("legacy state remains after full uninstall: %v", err)
	}
	content, err := os.ReadFile(native.destination)
	if err != nil || string(content) != "foreign dispatcher\n" {
		t.Fatalf("full uninstall changed unowned native entrypoint: content=%q error=%v", content, err)
	}
}

func materializeLegacyInstall(t *testing.T, fixture prepareFixture, request InstallRequest) PreparedInstall {
	t.Helper()
	prepared, err := PrepareInstall(fixture.source, fixture.environment, request)
	if err != nil {
		t.Fatal(err)
	}
	state := parsePreparedState(t, prepared.stateActions[0])
	routers := make([]targetRecord, 0, len(state.targets))
	keptActions := append([]installAction{prepared.policyAction}, prepared.policySetActions...)
	for _, action := range prepared.targetActions {
		if strings.HasSuffix(action.label, "/router") {
			keptActions = append(keptActions, action)
		}
	}
	for _, record := range state.targets {
		if record.artifact == routerArtifactID {
			routers = append(routers, record)
		}
	}
	state.targets = routers
	destinations := make([]string, 0, len(keptActions)+1)
	for _, action := range keptActions {
		destinations = append(destinations, action.destination)
	}
	destinations = append(destinations, prepared.stateActions[0].destination)
	state.directories = filterLegacyDirectories(state.directories, destinations)
	for _, action := range keptActions {
		writePrepareFile(t, action.destination, action.data, action.mode)
	}
	writePrepareFile(t, prepared.stateActions[0].destination, renderLegacyStateFixture(t, state), 0o600)
	return prepared
}

func filterLegacyDirectories(directories, destinations []string) []string {
	result := make([]string, 0, len(directories))
	for _, directory := range directories {
		for _, destination := range destinations {
			if containedStrictly(directory, destination) {
				result = append(result, directory)
				break
			}
		}
	}
	return result
}

func renderLegacyStateFixture(t *testing.T, state installationState) []byte {
	t.Helper()
	var result bytes.Buffer
	result.WriteString("format\t1\n")
	fmt.Fprintf(&result, "version\t%s\n", state.version)
	fmt.Fprintf(&result, "scope\t%s\n", state.scope)
	if state.scope == "project" {
		fmt.Fprintf(&result, "project\t%s\n", state.project)
	}
	fmt.Fprintf(&result, "policy\t%s\t%s\n", state.policyPath, state.policyChecksum)
	for _, record := range state.policyFiles {
		fmt.Fprintf(&result, "policy-file\t%s\t%s\n", record.path, record.checksum)
	}
	if state.backupPath != "" {
		fmt.Fprintf(&result, "backup\t%s\n", state.backupPath)
	}
	for _, directory := range state.directories {
		fmt.Fprintf(&result, "directory\t%s\n", directory)
	}
	for _, record := range state.targets {
		if record.artifact != routerArtifactID {
			t.Fatalf("legacy fixture contains non-router record: %#v", record)
		}
		fmt.Fprintf(
			&result, "target\t%s\t%s\t%s\t%s\t%s\n",
			record.id, record.path, record.mode, record.checksum, record.origin,
		)
	}
	return result.Bytes()
}

func findInstallActionByLabel(t *testing.T, actions []installAction, label string) installAction {
	t.Helper()
	for _, action := range actions {
		if action.label == label {
			return action
		}
	}
	t.Fatalf("install action %q not found", label)
	return installAction{}
}

func findMutationActionByLabel(t *testing.T, actions []mutationAction, label string) mutationAction {
	t.Helper()
	for _, action := range actions {
		if action.label == label {
			return action
		}
	}
	t.Fatalf("mutation action %q not found", label)
	return mutationAction{}
}

func TestLegacyFixtureUsesOnlyOwnedHistoricalPaths(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeLegacyInstall(t, fixture, InstallRequest{Targets: "claude"})
	native := findInstallActionByLabel(t, installed.targetActions, "claude/native-entrypoint")
	if _, err := os.Stat(native.destination); !os.IsNotExist(err) {
		t.Fatalf("legacy fixture unexpectedly created native entrypoint: %v", err)
	}
	if !filepath.IsAbs(installed.coordinates.stateFile) {
		t.Fatalf("legacy fixture state path is not absolute: %q", installed.coordinates.stateFile)
	}
}
