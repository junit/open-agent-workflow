package management

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderBackupManifestExactBytes(t *testing.T) {
	root := t.TempDir()
	backupPath := filepath.Join(root, "backups", "20260802T010203Z-42")
	first := backupCandidate{
		original: filepath.Join(root, "target"), backup: filepath.Join(backupPath, "001-target"),
		checksum: "123:4",
	}
	second := backupCandidate{
		original: filepath.Join(root, "state.tsv"), backup: filepath.Join(backupPath, "002-state.tsv"),
		checksum: "456:5",
	}
	plan := backupPlan{required: true, operation: "update", scope: "user", path: backupPath, candidates: []backupCandidate{first, second}}
	got, err := renderBackupManifest(plan)
	if err != nil {
		t.Fatal(err)
	}
	want := "format\t1\noperation\tupdate\nscope\tuser\n" +
		"artifact\t" + first.original + "\t" + first.backup + "\t123:4\n" +
		"artifact\t" + second.original + "\t" + second.backup + "\t456:5\n"
	if string(got) != want {
		t.Fatalf("manifest = %q, want %q", got, want)
	}
	got[0] = 'X'
	again, err := renderBackupManifest(plan)
	if err != nil || strings.HasPrefix(string(again), "X") {
		t.Fatalf("manifest aliases output: %q, %v", again, err)
	}
}

func TestApplyMutationBackupWritesPrivateVerifiedArtifacts(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	target := installed.targetActions[0].destination
	drift := []byte(beginMarker + "\ndrift\n" + endMarker + "\n")
	if err := os.WriteFile(target, drift, 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	line, err := applyMutationBackup(prepared.plan.backup, fixture.environment)
	if err != nil {
		t.Fatal(err)
	}
	if line != "oaw: backup: "+prepared.plan.backup.path {
		t.Fatalf("line = %q", line)
	}
	info, err := os.Stat(prepared.plan.backup.path)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("backup directory mode = %v, %v", info, err)
	}
	manifest, err := os.ReadFile(filepath.Join(prepared.plan.backup.path, "manifest.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	wantManifest, err := renderBackupManifest(prepared.plan.backup)
	if err != nil || string(manifest) != string(wantManifest) {
		t.Fatalf("manifest = %q, want %q, %v", manifest, wantManifest, err)
	}
	for _, candidate := range prepared.plan.backup.candidates {
		copied, err := os.ReadFile(candidate.backup)
		if err != nil || string(copied) != string(candidate.before.data) {
			t.Fatalf("artifact %s = %q, %v", candidate.backup, copied, err)
		}
		info, err := os.Stat(candidate.backup)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("artifact mode %s = %v, %v", candidate.backup, info, err)
		}
	}
	manifestInfo, err := os.Stat(filepath.Join(prepared.plan.backup.path, "manifest.tsv"))
	if err != nil || manifestInfo.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode = %v, %v", manifestInfo, err)
	}
}

func TestApplyMutationBackupRejectsChangedSourceBeforeCreatingBackup(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	target := installed.targetActions[0].destination
	if err := os.WriteFile(target, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("changed after prepare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := applyMutationBackup(prepared.plan.backup, fixture.environment); err == nil || !strings.Contains(err.Error(), "backup source changed before mutation") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Lstat(prepared.plan.backup.path); !os.IsNotExist(err) {
		t.Fatalf("rejected backup path exists: %v", err)
	}
}

func TestApplyMutationBackupRejectsSameContentSourceReplacement(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	target := installed.targetActions[0].destination
	if err := os.WriteFile(target, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	candidate := backupCandidateForDestination(t, prepared.plan.backup, target)
	if err := os.Rename(target, target+".prepared"); err != nil {
		t.Fatal(err)
	}
	writePrepareFile(t, target, candidate.before.data, candidate.before.mode.Perm())
	if _, err := applyMutationBackup(prepared.plan.backup, fixture.environment); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Lstat(prepared.plan.backup.path); !os.IsNotExist(err) {
		t.Fatalf("rejected backup path exists: %v", err)
	}
}

func TestBackupPlanRejectsUnsafeAndDuplicateCandidates(t *testing.T) {
	root := t.TempDir()
	backupPath := filepath.Join(root, "backups", "operation")
	if _, err := renderBackupManifest(backupPlan{required: true, operation: "install", scope: "user", path: backupPath}); err == nil {
		t.Fatal("invalid operation succeeded")
	}
	if _, err := renderBackupManifest(backupPlan{required: true, operation: "update", scope: "bad", path: backupPath}); err == nil {
		t.Fatal("invalid scope succeeded")
	}
	unsafe := backupCandidate{original: "bad\npath", backup: filepath.Join(backupPath, "001-target"), checksum: "1:1"}
	if _, err := renderBackupManifest(backupPlan{required: true, operation: "update", scope: "user", path: backupPath, candidates: []backupCandidate{unsafe}}); err == nil {
		t.Fatal("unsafe candidate succeeded")
	}
	duplicate := backupCandidate{original: filepath.Join(root, "target"), backup: filepath.Join(backupPath, "001-target"), checksum: "1:1"}
	if _, err := renderBackupManifest(backupPlan{required: true, operation: "update", scope: "user", path: backupPath, candidates: []backupCandidate{duplicate, duplicate}}); err == nil {
		t.Fatal("duplicate candidate succeeded")
	}
}

func TestBackupPureBranchContracts(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "target")
	writePrepareFile(t, original, []byte("before"), 0o644)
	before, err := inspectInstallPath(original)
	if err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(root, "backups", "operation")
	replaceSame, err := newMutationAction(mutationReplace, "same", []byte("before"), original, 0o644, root, "target", before)
	if err != nil {
		t.Fatal(err)
	}
	replaceChanged, err := newMutationAction(mutationReplace, "changed", []byte("after"), original, 0o644, root, "target", before)
	if err != nil {
		t.Fatal(err)
	}
	remove, err := newMutationAction(mutationRemove, "remove", nil, original, 0, root, "target", before)
	if err != nil {
		t.Fatal(err)
	}
	retain, err := newMutationAction(mutationRetain, "retain", nil, original, 0, root, "target", before)
	if err != nil {
		t.Fatal(err)
	}
	if mutationActionNeedsBackup(replaceSame) || !mutationActionNeedsBackup(replaceChanged) || !mutationActionNeedsBackup(remove) || mutationActionNeedsBackup(retain) {
		t.Fatal("mutation backup classification differs")
	}
	missingRemove := remove
	missingRemove.before = installPathSnapshot{kind: installPathMissing}
	if mutationActionNeedsBackup(missingRemove) {
		t.Fatal("missing destination requires backup")
	}
	if plan, err := buildMutationBackupPlan(false, "update", "user", "", mutationAction{}, nil, nil); err != nil || plan.required {
		t.Fatalf("optional backup = %#v, %v", plan, err)
	}
	if _, err := buildMutationBackupPlan(true, "update", "user", backupPath, retain, nil, nil); err == nil || !strings.Contains(err.Error(), "no recoverable artifacts") {
		t.Fatalf("empty required backup error = %v", err)
	}

	candidates, err := addBackupCandidate(nil, original, root, "target", before, backupPath)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err = addBackupCandidate(candidates, original, root, "target", before, backupPath)
	if err != nil || len(candidates) != 1 {
		t.Fatalf("deduplicated candidates = %#v, %v", candidates, err)
	}
	if _, err := addBackupCandidate(nil, original, root, "target", installPathSnapshot{kind: installPathMissing}, backupPath); err == nil || !strings.Contains(err.Error(), "not a file") {
		t.Fatalf("missing source error = %v", err)
	}
	if _, err := addBackupCandidate(nil, "bad\npath", root, "target", before, backupPath); err == nil || !strings.Contains(err.Error(), "cannot be serialized") {
		t.Fatalf("unsafe source error = %v", err)
	}
	if _, err := addBackupCandidate(nil, original+"-other", root, "target", before, backupPath); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatch source error = %v", err)
	}
	if _, err := addBackupCandidate(nil, original, root, "../target", before, backupPath); err == nil || !strings.Contains(err.Error(), "unsafe component") {
		t.Fatalf("unsafe suffix error = %v", err)
	}
	if _, err := buildMutationBackupPlan(true, "update", "user", "relative-backup", replaceChanged, nil, nil); err == nil || !strings.Contains(err.Error(), "invalid backup path") {
		t.Fatalf("invalid built plan error = %v", err)
	}

	valid := backupPlan{required: true, operation: "update", scope: "user", path: backupPath, candidates: candidates}
	invalidPath := valid
	invalidPath.path = "relative"
	if _, err := renderBackupManifest(invalidPath); err == nil || !strings.Contains(err.Error(), "invalid backup path") {
		t.Fatalf("invalid path error = %v", err)
	}
	noCandidates := valid
	noCandidates.candidates = nil
	if _, err := renderBackupManifest(noCandidates); err == nil || !strings.Contains(err.Error(), "no recoverable artifacts") {
		t.Fatalf("no candidates error = %v", err)
	}
	escape := valid
	escape.candidates = cloneBackupPlan(valid).candidates
	escape.candidates[0].backup = filepath.Join(root, "outside")
	if _, err := renderBackupManifest(escape); err == nil || !strings.Contains(err.Error(), "escapes operation") {
		t.Fatalf("escape error = %v", err)
	}
	badChecksum := valid
	badChecksum.candidates = cloneBackupPlan(valid).candidates
	badChecksum.candidates[0].checksum = "bad"
	if _, err := renderBackupManifest(badChecksum); err == nil || !strings.Contains(err.Error(), "cannot be serialized") {
		t.Fatalf("checksum error = %v", err)
	}
	duplicateArtifact := valid
	duplicateArtifact.candidates = append(cloneBackupPlan(valid).candidates, cloneBackupPlan(valid).candidates[0])
	duplicateArtifact.candidates[1].original = filepath.Join(root, "second")
	if _, err := renderBackupManifest(duplicateArtifact); err == nil || !strings.Contains(err.Error(), "duplicate backup artifact") {
		t.Fatalf("duplicate artifact error = %v", err)
	}
}

func TestApplyMutationBackupRejectsExistingAndSymlinkedDestinations(t *testing.T) {
	t.Run("existing operation directory", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := applyMutationBackup(prepared.plan.backup, fixture.environment); err != nil {
			t.Fatal(err)
		}
		if _, err := applyMutationBackup(prepared.plan.backup, fixture.environment); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("repeat error = %v", err)
		}
	})

	t.Run("symlinked backup root", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
		if err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(fixture.root, "outside-backups")
		if err := os.Mkdir(outside, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, prepared.plan.coordinates.backupRoot); err != nil {
			t.Fatal(err)
		}
		if _, err := applyMutationBackup(prepared.plan.backup, fixture.environment); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink error = %v", err)
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatalf("outside entries = %v, %v", entries, err)
		}
	})
}

func TestWritePrivateBackupFileRejectsInvalidAndExistingNames(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := writePrivateBackupFile(root, "../escape", []byte("secret")); err == nil || !strings.Contains(err.Error(), "invalid backup artifact name") {
		t.Fatalf("invalid name error = %v", err)
	}
	if err := root.WriteFile("existing", []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateBackupFile(root, "existing", []byte("new")); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing error = %v", err)
	}
	entries, err := os.ReadDir(rootPath)
	if err != nil || len(entries) != 1 || entries[0].Name() != "existing" {
		t.Fatalf("temporary cleanup entries = %v, %v", entries, err)
	}
}

func TestApplyMutationBackupRejectsInvalidPlanAndStateRoot(t *testing.T) {
	fixture := newPrepareFixture(t)
	if _, err := applyMutationBackup(backupPlan{}, fixture.environment); err == nil || !strings.Contains(err.Error(), "not required") {
		t.Fatalf("invalid plan error = %v", err)
	}
	original := filepath.Join(fixture.root, "source")
	writePrepareFile(t, original, []byte("source"), 0o644)
	before, err := inspectInstallPath(original)
	if err != nil {
		t.Fatal(err)
	}
	stateRootFile := filepath.Join(fixture.root, "state-file")
	writePrepareFile(t, stateRootFile, []byte("not a directory"), 0o600)
	backupPath := filepath.Join(stateRootFile, "operation")
	candidates, err := addBackupCandidate(nil, original, fixture.root, "source", before, backupPath)
	if err != nil {
		t.Fatal(err)
	}
	plan := backupPlan{required: true, operation: "update", scope: "user", path: backupPath, candidates: candidates}
	environment := fixture.environment
	environment.StateHome = stateRootFile
	if _, err := applyMutationBackup(plan, environment); err == nil || !strings.Contains(err.Error(), "could not be inspected") {
		t.Fatalf("state root error = %v", err)
	}
}

func TestCreatePrivateBackupPathRejectsSymlinkAndRegularComponents(t *testing.T) {
	rootPath := t.TempDir()
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(rootPath, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := createPrivateBackupPath(root, "linked/operation", filepath.Join(rootPath, "linked", "operation")); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, "regular"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := createPrivateBackupPath(root, "regular/operation", filepath.Join(rootPath, "regular", "operation")); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("regular error = %v", err)
	}
}

func TestRevalidateBackupSourcesRejectsCoordinateMismatch(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "source")
	writePrepareFile(t, original, []byte("source"), 0o644)
	before, err := inspectInstallPath(original)
	if err != nil {
		t.Fatal(err)
	}
	plan := backupPlan{required: true, operation: "update", scope: "user", path: filepath.Join(root, "backup"), candidates: []backupCandidate{{
		original: original + "-other", allowedRoot: root, relativeSuffix: "source",
		checksum: checksumBytes(before.data), before: before,
	}}}
	if err := revalidateBackupSources(plan); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v", err)
	}
}
