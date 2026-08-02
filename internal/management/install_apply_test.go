package management

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApplyInstallCreatesTargetsPolicyStateModesAndOwnership(t *testing.T) {
	fixture := newPrepareFixture(t)
	existing := filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md")
	writePrepareFile(t, existing, []byte("user content\n"), 0o640)
	prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyInstall(prepared)
	if err != nil {
		t.Fatalf("ApplyInstall() result=%#v error=%v", result, err)
	}
	wantLines := []string{
		"oaw: update: " + prepared.targetActions[0].destination,
		"oaw: create: " + prepared.policyAction.destination,
		"oaw: create: " + prepared.stateActions[0].destination,
	}
	if !reflect.DeepEqual(result.Lines, wantLines) {
		t.Fatalf("result lines = %#v, want %#v", result.Lines, wantLines)
	}
	assertAppliedAction(t, prepared.targetActions[0], 0o644)
	assertAppliedAction(t, prepared.policyAction, 0o600)
	assertAppliedAction(t, prepared.stateActions[0], 0o600)
	if !bytes.HasPrefix(prepared.targetActions[0].data, []byte("user content\n")) {
		t.Fatalf("target did not preserve existing bytes: %q", prepared.targetActions[0].data)
	}
	state := parsePreparedState(t, prepared.stateActions[0])
	if containsString(state.directories, filepath.Join(fixture.environment.Home, ".claude")) {
		t.Fatalf("state claimed pre-existing directory: %#v", state.directories)
	}
	if _, err := os.Lstat(prepared.coordinates.backupRoot); !os.IsNotExist(err) {
		t.Fatalf("install created a backup root: %v", err)
	}
}

func TestApplyInstallIsIdempotentWithoutMtimeChanges(t *testing.T) {
	fixture := newPrepareFixture(t)
	first, err := Install(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil || len(first.Lines) != 3 {
		t.Fatalf("Install() result=%#v error=%v", first, err)
	}
	prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	before := actionMetadata(t, append(append([]installAction(nil), prepared.targetActions...), prepared.policyAction, prepared.stateActions[0])...)
	result, err := ApplyInstall(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Lines; !reflect.DeepEqual(got, []string{
		"oaw: unchanged: claude", "oaw: unchanged: policy", "oaw: unchanged: state",
	}) {
		t.Fatalf("idempotent lines = %#v", got)
	}
	after := actionMetadata(t, append(append([]installAction(nil), prepared.targetActions...), prepared.policyAction, prepared.stateActions[0])...)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("idempotent install changed metadata:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestApplyInstallIsIdempotentWithRedundantStateRootSeparator(t *testing.T) {
	fixture := newPrepareFixture(t)
	fixture.environment.StateHome = filepath.Dir(fixture.environment.StateHome) +
		string(filepath.Separator) + string(filepath.Separator) + filepath.Base(fixture.environment.StateHome)

	if _, err := Install(fixture.source, fixture.environment, InstallRequest{Targets: "claude"}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	result, err := Install(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatalf("repeated Install() error = %v", err)
	}
	if got := result.Lines; !reflect.DeepEqual(got, []string{
		"oaw: unchanged: claude", "oaw: unchanged: policy", "oaw: unchanged: state",
	}) {
		t.Fatalf("repeated Install() lines = %#v", got)
	}
}

func TestApplyInstallDryRunReturnsPredictionWithoutWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude,codex", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotPrepareTree(t, fixture.root)
	result, err := ApplyInstall(prepared)
	if err != nil {
		t.Fatal(err)
	}
	after := snapshotPrepareTree(t, fixture.root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dry-run changed tree:\nbefore=%#v\nafter=%#v", before, after)
	}
	if !reflect.DeepEqual(result, prepared.predicted) {
		t.Fatalf("dry-run result = %#v, want %#v", result, prepared.predicted)
	}
}

func TestApplyInstallRevalidatesCompletePlanBeforeFirstWrite(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, prepareFixture, PreparedInstall)
	}{
		{
			name: "destination appeared",
			mutate: func(t *testing.T, _ prepareFixture, prepared PreparedInstall) {
				writePrepareFile(t, prepared.targetActions[1].destination, []byte("foreign\n"), 0o644)
			},
		},
		{
			name: "planned directory became symlink",
			mutate: func(t *testing.T, fixture prepareFixture, _ PreparedInstall) {
				if err := os.Symlink(t.TempDir(), filepath.Join(fixture.environment.Home, ".codex")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude,codex"})
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(t, fixture, prepared)
			before := snapshotPrepareTree(t, fixture.root)
			result, err := ApplyInstall(prepared)
			if err == nil {
				t.Fatalf("ApplyInstall() result=%#v", result)
			}
			after := snapshotPrepareTree(t, fixture.root)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("failed preflight changed tree:\nbefore=%#v\nafter=%#v", before, after)
			}
			if _, err := os.Lstat(prepared.targetActions[0].destination); !os.IsNotExist(err) {
				t.Fatalf("first target was written: %v", err)
			}
			if _, err := os.Lstat(prepared.policyAction.destination); !os.IsNotExist(err) {
				t.Fatalf("policy was written: %v", err)
			}
		})
	}
}

func TestApplyInstallRejectsStateChangeAfterPreparation(t *testing.T) {
	fixture := newPrepareFixture(t)
	if _, err := Install(fixture.source, fixture.environment, InstallRequest{Targets: "claude"}); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, prepared.stateActions[0].destination, []byte("format\t2\n"), 0o600)
	before := snapshotPrepareTree(t, fixture.root)
	result, err := ApplyInstall(prepared)
	if err == nil {
		t.Fatalf("ApplyInstall() result=%#v", result)
	}
	if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatal("state-change rejection mutated the tree")
	}
}

func TestApplyInstallCoordinatesSharedProjectAndCrossScopeState(t *testing.T) {
	fixture := newPrepareFixture(t)
	fixture.environment.StateHome = filepath.Dir(fixture.environment.StateHome) +
		string(filepath.Separator) + string(filepath.Separator) + filepath.Base(fixture.environment.StateHome)
	if _, err := Install(fixture.source, fixture.environment, InstallRequest{Targets: "claude"}); err != nil {
		t.Fatal(err)
	}
	userPrepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	userState := parsePreparedState(t, userPrepared.stateActions[0])
	userState.backupPath = filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "backups", "existing")
	userStateBytes, err := serializeInstallState(userState)
	if err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, userPrepared.stateActions[0].destination, userStateBytes, 0o600)

	project := filepath.Join(fixture.root, "shared project")
	if err := os.MkdirAll(filepath.Join(project, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(project, ".cursor", "sibling.txt")
	writePrepareFile(t, sibling, []byte("sentinel\n"), 0o644)
	prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Project: project, Targets: "codex,opencode,cursor"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyInstall(prepared)
	if err != nil {
		t.Fatalf("ApplyInstall() result=%#v error=%v", result, err)
	}
	want := []string{
		"oaw: create: " + prepared.targetActions[0].destination,
		"oaw: create: " + prepared.targetActions[1].destination,
		"oaw: unchanged: policy",
		"oaw: create: " + prepared.stateActions[0].destination,
		"oaw: unchanged: state-reference-1",
	}
	if !reflect.DeepEqual(result.Lines, want) {
		t.Fatalf("cross-scope lines = %#v, want %#v", result.Lines, want)
	}
	if data, err := os.ReadFile(sibling); err != nil || string(data) != "sentinel\n" {
		t.Fatalf("sibling data=%q error=%v", data, err)
	}
	projectState := parsePreparedState(t, prepared.stateActions[0])
	if got := targetRecordIDs(projectState.targets); !reflect.DeepEqual(got, []string{"codex", "opencode", "cursor"}) {
		t.Fatalf("project state targets = %#v", got)
	}
	if containsString(projectState.directories, filepath.Join(prepared.resolved.projectRoot, ".cursor")) {
		t.Fatalf("project state claimed pre-existing .cursor: %#v", projectState.directories)
	}
	updatedUserBytes, err := os.ReadFile(userPrepared.stateActions[0].destination)
	if err != nil {
		t.Fatal(err)
	}
	updatedUser, err := parseInstallationState(updatedUserBytes)
	if err != nil {
		t.Fatal(err)
	}
	if updatedUser.backupPath != userState.backupPath {
		t.Fatalf("backup reference = %q, want %q", updatedUser.backupPath, userState.backupPath)
	}
	if _, err := os.Lstat(prepared.coordinates.backupRoot); !os.IsNotExist(err) {
		t.Fatalf("cross-scope install created backup root: %v", err)
	}
}

func TestApplyInstallReportsCompletedBashOrderActionsBeforeWriteFailure(t *testing.T) {
	fixture := newPrepareFixture(t)
	policyDestination := filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "ENGINEERING.md")
	if err := os.MkdirAll(policyDestination, 0o755); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyInstall(prepared)
	if err == nil {
		t.Fatalf("ApplyInstall() result=%#v", result)
	}
	if got := result.Lines; !reflect.DeepEqual(got, []string{"oaw: create: " + prepared.targetActions[0].destination}) {
		t.Fatalf("partial result = %#v", got)
	}
	if _, statErr := os.Stat(prepared.targetActions[0].destination); statErr != nil {
		t.Fatalf("completed target is missing: %v", statErr)
	}
	if info, statErr := os.Lstat(prepared.policyAction.destination); statErr != nil || !info.IsDir() {
		t.Fatalf("failed policy action changed destination: info=%v error=%v", info, statErr)
	}
	if _, statErr := os.Lstat(prepared.stateActions[0].destination); !os.IsNotExist(statErr) {
		t.Fatalf("state was written after failure: %v", statErr)
	}
}

func assertAppliedAction(t *testing.T, action installAction, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(action.destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, action.data) {
		t.Fatalf("%s bytes = %q, want %q", action.label, data, action.data)
	}
	info, err := os.Stat(action.destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode.Perm() {
		t.Fatalf("%s mode = %o, want %o", action.label, info.Mode().Perm(), mode.Perm())
	}
}

type appliedMetadata struct {
	mode    os.FileMode
	size    int64
	modTime int64
	data    string
}

func actionMetadata(t *testing.T, actions ...installAction) map[string]appliedMetadata {
	t.Helper()
	result := make(map[string]appliedMetadata, len(actions))
	for _, action := range actions {
		info, err := os.Stat(action.destination)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(action.destination)
		if err != nil {
			t.Fatal(err)
		}
		result[action.destination] = appliedMetadata{
			mode: info.Mode(), size: info.Size(), modTime: info.ModTime().UnixNano(), data: string(data),
		}
	}
	return result
}

func TestApplyInstallRejectsForgedPreparedValue(t *testing.T) {
	result, err := ApplyInstall(PreparedInstall{})
	if err == nil || len(result.Lines) != 0 || !strings.Contains(err.Error(), "VERSION") {
		t.Fatalf("ApplyInstall() result=%#v error=%v", result, err)
	}
}

func TestInstallAndActionApplicationRejectChangedInputs(t *testing.T) {
	fixture := newPrepareFixture(t)
	if result, err := Install(Source{}, fixture.environment, InstallRequest{}); err == nil || len(result.Lines) != 0 {
		t.Fatalf("Install(zero source) result=%#v error=%v", result, err)
	}
	destination := filepath.Join(fixture.environment.Home, "artifact")
	writePrepareFile(t, destination, []byte("appeared\n"), 0o644)
	action := installAction{
		label: "artifact", data: []byte("replacement\n"), destination: destination, mode: 0o644,
		allowedRoot: fixture.environment.Home, relativeSuffix: "artifact", before: installPathSnapshot{kind: installPathMissing},
	}
	if line, err := applyPreparedInstallAction(action, nil, make(map[string]struct{})); err == nil || line != "" || !strings.Contains(err.Error(), "changed after preparation") {
		t.Fatalf("applyPreparedInstallAction() line=%q error=%v", line, err)
	}
}
