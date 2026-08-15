package management

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareInstallFreshUserIsImmutableAndDeterministic(t *testing.T) {
	fixture := newPrepareFixture(t)
	source := policySetSource(t, "0.1.0", "")

	prepared, err := prepareWithoutWrites(t, fixture.root, source, fixture.environment, InstallRequest{
		Targets: "codex,claude,codex",
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("PrepareInstall() error = %v", err)
	}
	if got := prepared.resolved.targets; !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Fatalf("resolved targets = %#v", got)
	}
	if got := actionLabels(prepared.targetActions); !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Fatalf("target action labels = %#v", got)
	}
	if len(prepared.policyAction.data) == 0 {
		t.Fatal("policy action is empty")
	}

	state := parsePreparedState(t, prepared.stateActions[0])
	if state.scope != "user" || state.project != "" || state.backupPath != "" {
		t.Fatalf("prepared state coordinates = %#v", state)
	}
	if got := targetRecordIDs(state.targets); !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Fatalf("state targets = %#v", got)
	}
	wantDirectories := []string{
		filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow"),
		filepath.Join(fixture.environment.StateHome, "open-agent-workflow"),
		filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "installations"),
		filepath.Join(fixture.environment.Home, ".claude"),
		filepath.Join(fixture.environment.Home, ".codex"),
		filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "adapters"),
		filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "profiles"),
		filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "profiles", "builtin"),
	}
	if !reflect.DeepEqual(state.directories, wantDirectories) {
		t.Fatalf("owned directories = %#v, want %#v", state.directories, wantDirectories)
	}
	if !reflect.DeepEqual(prepared.plannedDirectories, wantDirectories) {
		t.Fatalf("planned directories = %#v, want %#v", prepared.plannedDirectories, wantDirectories)
	}
	wantLines := []string{
		"oaw: would-create: " + filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "POLICY.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "adapters", "codex-policy.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "cooperative-protocol.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "profiles", "builtin", "ECC-FULL.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "profiles", "builtin", "MATT-FULL.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "profiles", "builtin", "MATT-SP-HYBRID.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.ConfigHome, "open-agent-workflow", "profiles", "builtin", "SP-FULL.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.Home, ".codex", "AGENTS.md"),
		"oaw: would-create: " + filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "installations", "user.state"),
	}
	if !reflect.DeepEqual(prepared.predicted.Lines, wantLines) {
		t.Fatalf("dry-run lines = %#v, want %#v", prepared.predicted.Lines, wantLines)
	}
}

func TestPrepareInstallFreshUserDefaultsCoverEveryUserTarget(t *testing.T) {
	fixture := newPrepareFixture(t)
	prepared, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := actionLabels(prepared.targetActions); !reflect.DeepEqual(got, []string{"claude", "codex", "gemini", "opencode"}) {
		t.Fatalf("default user actions = %#v", got)
	}
	opencode := prepared.targetActions[3]
	if opencode.allowedRoot != fixture.environment.ConfigHome || opencode.relativeSuffix != "opencode/AGENTS.md" {
		t.Fatalf("OpenCode coordinates = %#v", opencode)
	}
}

func TestPrepareInstallPreservesExistingManagedContentPlacement(t *testing.T) {
	tests := []struct {
		name    string
		current string
		prefix  bool
	}{
		{name: "trailing newline", current: "user content\n"},
		{name: "without trailing newline", current: "user content", prefix: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			target := filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md")
			writePrepareFile(t, target, []byte(tt.current), 0o644)
			prepared, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
			if err != nil {
				t.Fatal(err)
			}
			block, err := renderManagedBlock("claude", "user", prepared.coordinates.policyPath)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]byte(tt.current), block...)
			if tt.prefix {
				want = append(append([]byte(nil), block...), []byte(tt.current)...)
			}
			if !bytes.Equal(prepared.targetActions[0].data, want) {
				t.Fatalf("target bytes = %q, want %q", prepared.targetActions[0].data, want)
			}
			state := parsePreparedState(t, prepared.stateActions[0])
			if state.targets[0].origin != "existing-file" {
				t.Fatalf("origin = %q", state.targets[0].origin)
			}
		})
	}
}

func TestPrepareInstallProjectUsesPhysicalRootAndDeduplicatesSharedDestination(t *testing.T) {
	fixture := newPrepareFixture(t)
	realProject := filepath.Join(fixture.root, "real project")
	projectLink := filepath.Join(fixture.root, "project link")
	if err := os.MkdirAll(realProject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realProject, projectLink); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(realProject)
	if err != nil {
		t.Fatal(err)
	}

	prepared, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Project: projectLink})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.resolved.projectRoot != physical {
		t.Fatalf("project root = %q, want %q", prepared.resolved.projectRoot, physical)
	}
	if len(prepared.targetActions) != 8 {
		t.Fatalf("target action count = %d, want 8", len(prepared.targetActions))
	}
	state := parsePreparedState(t, prepared.stateActions[0])
	if len(state.targets) != 9 {
		t.Fatalf("state target count = %d, want 9", len(state.targets))
	}
	codex := findPreparedRecord(t, state.targets, "codex")
	opencode := findPreparedRecord(t, state.targets, "opencode")
	if codex.path != filepath.Join(physical, "AGENTS.md") || codex.path != opencode.path || codex.mode != opencode.mode || codex.checksum != opencode.checksum || codex.origin != opencode.origin {
		t.Fatalf("shared records: codex=%#v opencode=%#v", codex, opencode)
	}
	for _, record := range state.targets {
		if !strings.HasPrefix(record.path, physical+string(filepath.Separator)) {
			t.Fatalf("target path does not use physical project: %#v", record)
		}
	}
}

func TestProjectInstallRejectsUntrackedPolicySetContentWithoutWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	foreign := filepath.Join(project, ".oaw", "policy", "POLICY.md")
	writePrepareFile(t, foreign, []byte("foreign policy\n"), 0o644)
	before := snapshotPrepareTree(t, fixture.root)

	_, err := Install(
		policySetSource(t, "0.1.0", ""), fixture.environment,
		InstallRequest{Project: project, Targets: "codex"},
	)
	if err == nil {
		t.Fatal("project install replaced untracked Policy Set content")
	}
	if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatal("rejected project install changed the filesystem")
	}
}

func TestPrepareInstallRepeatedAdditiveAndForcePreserveState(t *testing.T) {
	fixture := newPrepareFixture(t)
	fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	materializePreparedFixture(t, fresh)
	state := parsePreparedState(t, fresh.stateActions[0])
	state.backupPath = filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "backups", "prior")
	stateBytes, err := serializeInstallState(state)
	if err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, fresh.stateActions[0].destination, stateBytes, 0o600)

	repeated, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	wantRepeated := []string{"oaw: unchanged: policy"}
	for _, action := range repeated.policySetActions {
		wantRepeated = append(wantRepeated, "oaw: unchanged: "+action.label)
	}
	wantRepeated = append(wantRepeated, "oaw: unchanged: claude", "oaw: unchanged: state")
	if got := repeated.predicted.Lines; !reflect.DeepEqual(got, wantRepeated) {
		t.Fatalf("repeated predictions = %#v", got)
	}
	if got := parsePreparedState(t, repeated.stateActions[0]).backupPath; got != state.backupPath {
		t.Fatalf("backup = %q, want %q", got, state.backupPath)
	}

	additive, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	additiveState := parsePreparedState(t, additive.stateActions[0])
	if got := targetRecordIDs(additiveState.targets); !reflect.DeepEqual(got, []string{"claude", "codex"}) {
		t.Fatalf("additive targets = %#v", got)
	}
	if additiveState.backupPath != state.backupPath {
		t.Fatalf("additive backup = %q", additiveState.backupPath)
	}

	withoutForce, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	withForce, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "codex", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(semanticPreparedSnapshot(withoutForce), semanticPreparedSnapshot(withForce)) {
		t.Fatal("--force changed a clean install plan")
	}
}

func TestPrepareInstallAddsASecondSharedProjectTarget(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "shared project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	codex, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Project: project, Targets: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	materializePreparedFixture(t, codex)

	opencode, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Project: project, Targets: "opencode"})
	if err != nil {
		t.Fatal(err)
	}
	state := parsePreparedState(t, opencode.stateActions[0])
	if got := targetRecordIDs(state.targets); !reflect.DeepEqual(got, []string{"codex", "opencode"}) {
		t.Fatalf("shared additive targets = %#v", got)
	}
	first := findPreparedRecord(t, state.targets, "codex")
	second := findPreparedRecord(t, state.targets, "opencode")
	if first.path != second.path || first.checksum != second.checksum || first.origin != second.origin {
		t.Fatalf("shared records differ: %#v %#v", first, second)
	}
}

func TestPrepareInstallRejectsConflictsAndInvalidLaterTargetsWithoutWrites(t *testing.T) {
	tests := []struct {
		name    string
		request InstallRequest
		setup   func(*testing.T, prepareFixture)
		message string
	}{
		{
			name:    "owned file collision",
			request: InstallRequest{Targets: "cursor"},
			setup: func(t *testing.T, fixture prepareFixture) {
				project := filepath.Join(fixture.root, "project")
				if err := os.Mkdir(project, 0o755); err != nil {
					t.Fatal(err)
				}
				writePrepareFile(t, filepath.Join(project, ".cursor", "rules", "open-agent-workflow.mdc"), []byte("foreign\n"), 0o644)
			},
			message: "owned target already exists",
		},
		{
			name:    "partial markers",
			request: InstallRequest{Targets: "claude"},
			setup: func(t *testing.T, fixture prepareFixture) {
				writePrepareFile(t, filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md"), []byte(beginMarker+"\npartial\n"), 0o644)
			},
			message: "untracked OAW markers already exist",
		},
		{
			name:    "later invalid target parent",
			request: InstallRequest{Targets: "claude,cursor"},
			setup: func(t *testing.T, fixture prepareFixture) {
				project := filepath.Join(fixture.root, "project")
				if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
					t.Fatal(err)
				}
				writePrepareFile(t, filepath.Join(project, ".claude", "CLAUDE.md"), []byte("foreign\n"), 0o644)
				writePrepareFile(t, filepath.Join(project, ".cursor"), []byte("not a directory\n"), 0o644)
			},
			message: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			tt.setup(t, fixture)
			request := tt.request
			if strings.Contains(tt.name, "owned") || strings.Contains(tt.name, "later") {
				request.Project = filepath.Join(fixture.root, "project")
			}
			_, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, request)
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("PrepareInstall() error = %v, want %q", err, tt.message)
			}
			request.Force = true
			_, forceErr := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, request)
			if forceErr == nil || forceErr.Error() != err.Error() {
				t.Fatalf("forced error = %v, want %v", forceErr, err)
			}
		})
	}
}

func TestPrepareInstallRejectsInvalidDriftedAndCheckoutMismatchedState(t *testing.T) {
	t.Run("invalid state", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		statePath := filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "installations", "user.state")
		writePrepareFile(t, statePath, []byte("format\t2\n"), 0o600)
		_, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "state") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("managed policy drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		writePrepareFile(t, fresh.policyAction.destination, []byte("drift\n"), 0o600)
		_, err = prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "managed policy has drifted") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("checkout version mismatch", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		newSource := policySetSource(t, "0.2.0", "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = prepareWithoutWrites(t, fixture.root, newSource, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "installed content differs from this checkout; run update") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("scope mismatch", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		state := parsePreparedState(t, fresh.stateActions[0])
		state.scope = "project"
		state.project = fixture.root
		data, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, fresh.stateActions[0].destination, data, 0o600)
		_, err = prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "installed scope does not match") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("target path mismatch", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		state := parsePreparedState(t, fresh.stateActions[0])
		state.targets[0].path = filepath.Join(fixture.environment.Home, ".claude", "wrong.md")
		data, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, fresh.stateActions[0].destination, data, 0o600)
		_, err = prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "installed target path does not match") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("managed target drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		writePrepareFile(t, fresh.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644)
		_, err = prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "managed target block has drifted") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("missing managed policy", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		if err := os.Remove(fresh.policyAction.destination); err != nil {
			t.Fatal(err)
		}
		_, err = prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "managed policy is missing") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("installed policy path mismatch", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		state := parsePreparedState(t, fresh.stateActions[0])
		state.policyPath = filepath.Join(fixture.environment.ConfigHome, "other-policy.md")
		data, err := serializeInstallState(state)
		if err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, fresh.stateActions[0].destination, data, 0o600)
		_, err = prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "installed policy path does not match") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("owned target drift", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "owned project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		fresh, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Project: project, Targets: "cursor"})
		if err != nil {
			t.Fatal(err)
		}
		materializePreparedFixture(t, fresh)
		writePrepareFile(t, fresh.targetActions[0].destination, []byte("drift\n"), 0o644)
		_, err = prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Project: project, Targets: "cursor"})
		if err == nil || !strings.Contains(err.Error(), "owned target file has drifted") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})
}

func TestNewSourceAndPrepareInstallRejectUnsafeInputs(t *testing.T) {
	if _, err := NewSource("", nil); err == nil {
		t.Fatal("NewSource() accepted an empty version")
	}
	if _, err := NewSource("bad\nversion", nil); err == nil {
		t.Fatal("NewSource() accepted an unsafe version")
	}
	if _, err := NewSource("0.1.0", nil); err == nil {
		t.Fatal("NewSource() accepted an empty policy")
	}

	t.Run("symlinked target component", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		if err := os.Symlink(t.TempDir(), filepath.Join(fixture.environment.Home, ".claude")); err != nil {
			t.Fatal(err)
		}
		_, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "destination path contains a symlink") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("oversized managed file", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		path := filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md")
		writePrepareFile(t, path, bytes.Repeat([]byte{'x'}, maximumInstallArtifactBytes+1), 0o644)
		_, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "could not be read") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})

	t.Run("managed destination directory", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		if err := os.MkdirAll(filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := prepareWithoutWrites(t, fixture.root, fixture.source, fixture.environment, InstallRequest{Targets: "claude"})
		if err == nil || !strings.Contains(err.Error(), "untracked OAW markers already exist") {
			t.Fatalf("PrepareInstall() error = %v", err)
		}
	})
}

func TestInstallActionValuesAreDefensiveAndFailClosed(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "managed", "file")
	data := []byte("artifact\n")
	action, err := newInstallAction("claude", data, destination, 0o644, root, "managed/file", installPathSnapshot{kind: installPathMissing})
	if err != nil {
		t.Fatal(err)
	}
	data[0] = 'X'
	if string(action.data) != "artifact\n" {
		t.Fatalf("action data = %q", action.data)
	}
	if _, err := newInstallAction("bad\nlabel", []byte("x"), destination, 0o644, root, "managed/file", installPathSnapshot{}); err == nil {
		t.Fatal("unsafe action label was accepted")
	}
	if _, err := newInstallAction("claude", []byte("x"), destination, 0o755, root, "managed/file", installPathSnapshot{}); err == nil {
		t.Fatal("unsafe action mode was accepted")
	}
	if _, err := newInstallAction("claude", []byte("x"), filepath.Join(root, "other"), 0o644, root, "managed/file", installPathSnapshot{}); err == nil {
		t.Fatal("mismatched action destination was accepted")
	}
	if _, err := newStateInstallAction("state", []byte("state\n"), filepath.Join(t.TempDir(), "outside.state"), root); err == nil {
		t.Fatal("state action outside its root was accepted")
	}
	stateAction, err := newStateInstallAction("state", []byte("state\n"), filepath.Join(root, "states", "user.state"), root)
	if err != nil || stateAction.relativeSuffix != "states/user.state" || stateAction.before.kind != installPathMissing {
		t.Fatalf("valid state action = %#v, error=%v", stateAction, err)
	}

	actions, err := addInstallAction(nil, action)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := cloneInstallAction(action)
	duplicate.label = "codex"
	actions, err = addInstallAction(actions, duplicate)
	if err != nil || len(actions) != 1 {
		t.Fatalf("identical shared action = %#v, error=%v", actions, err)
	}
	conflict := cloneInstallAction(action)
	conflict.data = []byte("different\n")
	if _, err := addInstallAction(actions, conflict); err == nil {
		t.Fatal("conflicting shared action was accepted")
	}
	coords := coordinates{environment: Environment{Home: root, ConfigHome: root}}
	if _, _, err := targetInstallCoordinates(coords, resolvedRequest{scope: "user"}, "cursor"); err == nil {
		t.Fatal("unsupported user target coordinates were accepted")
	}
	if _, _, err := targetInstallCoordinates(coords, resolvedRequest{scope: "user"}, "unknown"); err == nil {
		t.Fatal("unknown target coordinates were accepted")
	}
	typed := compatibilityError("typed")
	if installError(typed) != typed {
		t.Fatal("installError() replaced a typed error")
	}
}

func TestInstallPathSnapshotsClassifyFilesystemObjectsWithoutFollowingLinks(t *testing.T) {
	root := t.TempDir()
	missing, err := inspectInstallPath(filepath.Join(root, "missing"))
	if err != nil || missing.kind != installPathMissing {
		t.Fatalf("missing snapshot = %#v, error=%v", missing, err)
	}
	regularPath := filepath.Join(root, "regular")
	writePrepareFile(t, regularPath, []byte("bytes\n"), 0o640)
	regular, err := inspectInstallPath(regularPath)
	if err != nil || regular.kind != installPathRegular || string(regular.data) != "bytes\n" {
		t.Fatalf("regular snapshot = %#v, error=%v", regular, err)
	}
	directoryPath := filepath.Join(root, "directory")
	if err := os.Mkdir(directoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	directory, err := inspectInstallPath(directoryPath)
	if err != nil || directory.kind != installPathDirectory {
		t.Fatalf("directory snapshot = %#v, error=%v", directory, err)
	}
	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(regularPath, linkPath); err != nil {
		t.Fatal(err)
	}
	link, err := inspectInstallPath(linkPath)
	if err != nil || link.kind != installPathSymlink || link.link != regularPath || len(link.data) != 0 {
		t.Fatalf("link snapshot = %#v, error=%v", link, err)
	}
}

func TestSharedDestinationValuesRejectMissingAndConflictingRecords(t *testing.T) {
	destination := "/project/AGENTS.md"
	conflicting := []targetRecord{
		{id: "codex", path: destination, checksum: "1:1", origin: "created-file"},
		{id: "opencode", path: destination, checksum: "2:2", origin: "existing-file"},
	}
	if _, err := sharedDestinationOrigin(conflicting, destination); err == nil {
		t.Fatal("conflicting shared origins were accepted")
	}
	if _, err := sharedDestinationChecksum(conflicting, destination); err == nil {
		t.Fatal("conflicting shared checksums were accepted")
	}
	if _, err := sharedDestinationOrigin(nil, destination); err == nil {
		t.Fatal("missing shared origin was accepted")
	}
	if _, err := sharedDestinationChecksum(nil, destination); err == nil {
		t.Fatal("missing shared checksum was accepted")
	}
}
