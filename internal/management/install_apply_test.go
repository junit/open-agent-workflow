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
		"oaw: create: " + prepared.policyAction.destination,
	}
	for _, action := range prepared.policySetActions {
		wantLines = append(wantLines, "oaw: create: "+action.destination)
	}
	for index, action := range prepared.targetActions {
		verb := "create"
		if index == 0 {
			verb = "update"
		}
		wantLines = append(wantLines, "oaw: "+verb+": "+action.destination)
	}
	wantLines = append(wantLines, "oaw: create: "+prepared.stateActions[0].destination)
	if !reflect.DeepEqual(result.Lines, wantLines) {
		t.Fatalf("result lines = %#v, want %#v", result.Lines, wantLines)
	}
	for _, action := range prepared.targetActions {
		assertAppliedAction(t, action, 0o644)
	}
	assertAppliedAction(t, prepared.policyAction, 0o644)
	assertAppliedAction(t, prepared.stateActions[0], 0o600)
	if !bytes.HasPrefix(prepared.targetActions[0].data, []byte("user content\n")) {
		t.Fatalf("target did not preserve existing bytes: %q", prepared.targetActions[0].data)
	}
	state := parsePreparedState(t, prepared.stateActions[0])
	if got := targetArtifactIDs(state.targets); !reflect.DeepEqual(got, []string{"claude/router", "claude/native-entrypoint"}) {
		t.Fatalf("state targets = %#v", got)
	}
	if state.targets[0].mode != "managed-block" || state.targets[0].origin != "existing-file" ||
		state.targets[1].mode != "owned-file" || state.targets[1].origin != "created-file" {
		t.Fatalf("target ownership = %#v", state.targets)
	}
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
	if err != nil || len(first.Lines) != len(fixture.source.policySet)+3 {
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
	want := make([]string, 0, len(fixture.source.policySet)+3)
	want = append(want, "oaw: unchanged: policy")
	for _, action := range prepared.policySetActions {
		want = append(want, "oaw: unchanged: "+action.label)
	}
	for _, action := range prepared.targetActions {
		want = append(want, "oaw: unchanged: "+action.label)
	}
	want = append(want, "oaw: unchanged: state")
	if got := result.Lines; !reflect.DeepEqual(got, want) {
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
	prepared, err := PrepareInstall(fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, len(fixture.source.policySet)+3)
	want = append(want, "oaw: unchanged: policy")
	for _, action := range prepared.policySetActions {
		want = append(want, "oaw: unchanged: "+action.label)
	}
	for _, action := range prepared.targetActions {
		want = append(want, "oaw: unchanged: "+action.label)
	}
	want = append(want, "oaw: unchanged: state")
	if got := result.Lines; !reflect.DeepEqual(got, want) {
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
	if line, err := applyPreparedInstallAction(action, nil, make(createdDirectorySet)); err == nil || line != "" || !strings.Contains(err.Error(), "changed after preparation") {
		t.Fatalf("applyPreparedInstallAction() line=%q error=%v", line, err)
	}
}
