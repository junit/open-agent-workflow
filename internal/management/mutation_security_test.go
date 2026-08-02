package management

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMutationSecurityRejectsDuplicateDirectoryActionsBeforeWrites(t *testing.T) {
	fixture := newPrepareFixture(t)
	project := filepath.Join(fixture.root, "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
	prepared, err := PrepareUninstall(fixture.environment, UninstallRequest{Project: project, Targets: "cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.plan.directoryActions) == 0 {
		t.Fatal("uninstall plan has no directory action")
	}

	plan := cloneMutationPlan(prepared.plan)
	plan.directoryActions = append(plan.directoryActions, plan.directoryActions[0])
	before := snapshotPrepareTree(t, fixture.root)
	_, err = applyMutationPlan(plan, nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate prepared directory destination") {
		t.Fatalf("error = %v", err)
	}
	if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
		t.Fatalf("duplicate directory action changed tree:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestMutationSecurityRejectsLexicalPathAliases(t *testing.T) {
	t.Run("mutation actions", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		updated, err := NewSource("0.2.0", []byte("updated policy\n"))
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := PrepareUpdate(updated, fixture.environment, UpdateRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		plan := cloneMutationPlan(prepared.plan)
		alias := cloneMutationAction(plan.policyAction)
		alias.allowedRoot += string(filepath.Separator)
		alias.destination = alias.allowedRoot + string(filepath.Separator) + filepath.FromSlash(alias.relativeSuffix)
		plan.stateActions = append(plan.stateActions, alias)
		before := snapshotPrepareTree(t, fixture.root)
		_, err = applyMutationPlan(plan, nil)
		if err == nil || !strings.Contains(err.Error(), "duplicate prepared mutation destination") {
			t.Fatalf("error = %v", err)
		}
		if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
			t.Fatalf("aliased mutation action changed tree:\nbefore=%#v\nafter=%#v", before, after)
		}
	})

	t.Run("backup candidates", func(t *testing.T) {
		root := t.TempDir()
		operation := filepath.Join(root, "backups", "operation")
		first := filepath.Join(root, "target")
		second := root + string(filepath.Separator) + string(filepath.Separator) + "target"
		plan := backupPlan{required: true, operation: "update", scope: "user", path: operation, candidates: []backupCandidate{
			{original: first, backup: filepath.Join(operation, "001-target"), checksum: "1:1"},
			{original: second, backup: filepath.Join(operation, "002-target"), checksum: "1:1"},
		}}
		if _, err := renderBackupManifest(plan); err == nil || !strings.Contains(err.Error(), "duplicate backup original") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestMutationSecurityRejectsRedirectedActiveBackup(t *testing.T) {
	tests := []struct {
		name   string
		change func(*testing.T, prepareFixture, mutationPlan, mutationAction)
	}{
		{
			name: "manifest symlink",
			change: func(t *testing.T, fixture prepareFixture, plan mutationPlan, _ mutationAction) {
				manifestPath := filepath.Join(plan.backup.path, "manifest.tsv")
				manifest, err := os.ReadFile(manifestPath)
				if err != nil {
					t.Fatal(err)
				}
				twin := filepath.Join(fixture.root, "manifest-twin")
				writePrepareFile(t, twin, manifest, 0o600)
				if err := os.Remove(manifestPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(twin, manifestPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "artifact symlink",
			change: func(t *testing.T, fixture prepareFixture, plan mutationPlan, action mutationAction) {
				candidate := backupCandidateForDestination(t, plan.backup, action.destination)
				twin := filepath.Join(fixture.root, "artifact-twin")
				writePrepareFile(t, twin, candidate.before.data, 0o600)
				if err := os.Remove(candidate.backup); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(twin, candidate.backup); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "operation directory symlink",
			change: func(t *testing.T, _ prepareFixture, plan mutationPlan, _ mutationAction) {
				moved := plan.backup.path + "-moved"
				if err := os.Rename(plan.backup.path, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, plan.backup.path); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "public operation directory",
			change: func(t *testing.T, _ prepareFixture, plan mutationPlan, _ mutationAction) {
				if err := os.Chmod(plan.backup.path, 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "public artifact",
			change: func(t *testing.T, _ prepareFixture, plan mutationPlan, action mutationAction) {
				candidate := backupCandidateForDestination(t, plan.backup, action.destination)
				if err := os.Chmod(candidate.backup, 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "non regular artifact",
			change: func(t *testing.T, _ prepareFixture, plan mutationPlan, action mutationAction) {
				candidate := backupCandidateForDestination(t, plan.backup, action.destination)
				if err := os.Remove(candidate.backup); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(candidate.backup, 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "oversized artifact",
			change: func(t *testing.T, _ prepareFixture, plan mutationPlan, action mutationAction) {
				candidate := backupCandidateForDestination(t, plan.backup, action.destination)
				writePrepareFile(t, candidate.backup, make([]byte, maximumInstallArtifactBytes+1), 0o600)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
			if err := os.WriteFile(installed.targetActions[0].destination, []byte(beginMarker+"\ndrift\n"+endMarker+"\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := applyMutationBackup(prepared.plan.backup, fixture.environment); err != nil {
				t.Fatal(err)
			}
			action := prepared.plan.targetActions[0]
			test.change(t, fixture, prepared.plan, action)
			if err := verifyActiveMutationBackup(prepared.plan, action); err == nil {
				t.Fatal("redirected active backup was accepted")
			}
		})
	}
}

func TestMutationCredentialContentDoesNotLeakThroughBackupFailure(t *testing.T) {
	fixture := newPrepareFixture(t)
	installed := materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
	sentinel := "AWS_SECRET_ACCESS_KEY=credential-shaped-sentinel"
	drift := []byte(beginMarker + "\n" + sentinel + "\n" + endMarker + "\n")
	if err := os.WriteFile(installed.targetActions[0].destination, drift, 0o640); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareUpdate(fixture.source, fixture.environment, UpdateRequest{Targets: "claude", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	action := prepared.plan.targetActions[0]
	result, err := applyMutationPlan(prepared.plan, func(point mutationFaultPoint) error {
		if point.phase == mutationPhaseBackup && point.moment == mutationAfter {
			candidate := backupCandidateForDestination(t, prepared.plan.backup, action.destination)
			if chmodErr := os.Chmod(candidate.backup, 0o644); chmodErr != nil {
				t.Fatal(chmodErr)
			}
		}
		return nil
	})
	if err == nil {
		t.Fatal("unsafe active backup was accepted")
	}
	manifest, readErr := os.ReadFile(filepath.Join(prepared.plan.backup.path, "manifest.tsv"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	visible := strings.Join(result.Lines, "\n") + result.Trailing + err.Error() + string(manifest)
	if strings.Contains(visible, sentinel) {
		t.Fatalf("credential-shaped file content leaked: %q", visible)
	}
}

func TestMutationSecurityIdentityAndBackupHelpersFailClosed(t *testing.T) {
	t.Run("identity capture", func(t *testing.T) {
		root := t.TempDir()
		destination := filepath.Join(root, "artifact")
		identity, err := captureMutationPathIdentity(root, destination)
		if err != nil || !identity.captured || identity.root == nil || identity.parent == nil || identity.destination != nil {
			t.Fatalf("identity = %#v, %v", identity, err)
		}
		if _, err := captureMutationPathIdentity(filepath.Join(root, "missing"), destination); err == nil {
			t.Fatal("missing mutation root was accepted")
		}
		parentFile := filepath.Join(root, "parent-file")
		writePrepareFile(t, parentFile, []byte("file\n"), 0o600)
		if _, err := captureMutationPathIdentity(root, filepath.Join(parentFile, "child")); err == nil {
			t.Fatal("non-directory mutation parent was accepted")
		}
		outside := filepath.Join(root, "outside")
		writePrepareFile(t, outside, []byte("outside\n"), 0o600)
		if err := os.Symlink(outside, destination); err != nil {
			t.Fatal(err)
		}
		if _, err := captureMutationPathIdentity(root, destination); err == nil {
			t.Fatal("symlink mutation destination was accepted")
		}
	})

	t.Run("root identity swap", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "root")
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(root, "missing")
		identity, err := captureMutationPathIdentity(root, destination)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(root, root+"-prepared"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := revalidateMutationPathIdentity(identity, root, destination); err == nil || !strings.Contains(err.Error(), "identity changed") {
			t.Fatalf("error = %v", err)
		}
		if err := compareMutationPathIdentity(mutationPathIdentity{}, identity, destination); err == nil || !strings.Contains(err.Error(), "identity is missing") {
			t.Fatalf("missing identity error = %v", err)
		}

		gone := filepath.Join(parent, "gone")
		if err := os.Mkdir(gone, 0o755); err != nil {
			t.Fatal(err)
		}
		goneDestination := filepath.Join(gone, "missing")
		goneIdentity, err := captureMutationPathIdentity(gone, goneDestination)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(gone); err != nil {
			t.Fatal(err)
		}
		if err := revalidateMutationPathIdentity(goneIdentity, gone, goneDestination); err == nil || !strings.Contains(err.Error(), "root identity could not be captured") {
			t.Fatalf("removed root error = %v", err)
		}
	})

	t.Run("private backup reads", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		operation := filepath.Join(fixture.environment.StateHome, "open-agent-workflow", "backups", "operation")
		if err := verifyPrivateBackupDirectory(fixture.environment, operation); err == nil {
			t.Fatalf("missing directory error = %v", err)
		}
		if _, err := os.Lstat(operation); !os.IsNotExist(err) {
			t.Fatalf("missing directory was created: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(operation), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := verifyPrivateBackupDirectory(fixture.environment, operation); err == nil || !strings.Contains(err.Error(), "directory is missing") {
			t.Fatalf("missing operation error = %v", err)
		}
		if err := os.MkdirAll(operation, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(operation, 0o700); err != nil {
			t.Fatal(err)
		}
		missing := filepath.Join(operation, "missing")
		if _, err := readVerifiedBackupFile(fixture.environment, missing, "backup artifact"); err == nil || !strings.Contains(err.Error(), "is missing") {
			t.Fatalf("missing artifact error = %v", err)
		}
		directory := filepath.Join(operation, "directory")
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := readVerifiedBackupFile(fixture.environment, directory, "backup artifact"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("directory artifact error = %v", err)
		}
		artifact := filepath.Join(operation, "artifact")
		writePrepareFile(t, artifact, []byte("private\n"), 0o600)
		data, err := readVerifiedBackupFile(fixture.environment, artifact, "backup artifact")
		if err != nil || string(data) != "private\n" {
			t.Fatalf("artifact = %q, %v", data, err)
		}
	})

	t.Run("rollback root disappeared", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "missing-root")
		destination := filepath.Join(root, "artifact")
		applied := mutationAction{
			effect: mutationRemove, label: "artifact", destination: destination,
			allowedRoot: root, relativeSuffix: "artifact",
			before: installPathSnapshot{kind: installPathRegular, data: []byte("before\n"), mode: 0o600},
		}
		if err := restoreMutationAction(applied); err == nil || !strings.Contains(err.Error(), "root identity could not be captured") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestMutationSecurityReadAndRollbackDoNotCreateMissingRoots(t *testing.T) {
	t.Run("mutation primitives", func(t *testing.T) {
		for name, effect := range map[string]mutationEffect{
			"replace": mutationReplace,
			"remove":  mutationRemove,
		} {
			t.Run(name, func(t *testing.T) {
				parent := t.TempDir()
				root := filepath.Join(parent, "missing-root")
				destination := filepath.Join(root, "artifact")
				action := mutationAction{
					effect: effect, label: "artifact", destination: destination,
					allowedRoot: root, relativeSuffix: "artifact",
					before: installPathSnapshot{kind: installPathMissing},
				}
				var err error
				if effect == mutationReplace {
					action.data = []byte("replacement\n")
					action.mode = 0o600
					err = scopedAtomicReplaceMutation(action)
				} else {
					err = scopedAtomicRemoveMutation(action)
				}
				if err == nil {
					t.Fatal("mutation primitive accepted a missing root")
				}
				if _, statErr := os.Lstat(root); !os.IsNotExist(statErr) {
					t.Fatalf("mutation primitive created missing root: %v", statErr)
				}
			})
		}
	})

	t.Run("backup verification", func(t *testing.T) {
		parent := t.TempDir()
		stateHome := filepath.Join(parent, "missing-state")
		environment := Environment{StateHome: stateHome}
		operation := filepath.Join(stateHome, "open-agent-workflow", "backups", "operation")
		if err := verifyPrivateBackupDirectory(environment, operation); err == nil {
			t.Fatal("missing backup root was accepted")
		}
		if _, err := os.Lstat(stateHome); !os.IsNotExist(err) {
			t.Fatalf("backup verification created missing state root: %v", err)
		}
	})

	t.Run("directory rollback", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "missing-root")
		destination := filepath.Join(root, "owned")
		action := directoryAction{
			destination: destination, allowedRoot: root, relativeSuffix: "owned",
			before: installPathSnapshot{kind: installPathDirectory, mode: 0o750},
		}
		if err := restoreMutationDirectory(action); err == nil {
			t.Fatal("directory rollback accepted a missing root")
		}
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatalf("directory rollback created missing root: %v", err)
		}
	})
}

func TestMutationSecurityRejectsIdenticalPathSwapsBeforeWrites(t *testing.T) {
	tests := []struct {
		name   string
		change func(*testing.T, mutationPlan)
	}{
		{
			name: "final file",
			change: func(t *testing.T, plan mutationPlan) {
				destination := plan.policyAction.destination
				moved := destination + ".prepared"
				if err := os.Rename(destination, moved); err != nil {
					t.Fatal(err)
				}
				writePrepareFile(t, destination, plan.policyAction.before.data, plan.policyAction.before.mode.Perm())
			},
		},
		{
			name: "parent directory",
			change: func(t *testing.T, plan mutationPlan) {
				destination := plan.policyAction.destination
				parent := filepath.Dir(destination)
				moved := parent + ".prepared"
				if err := os.Rename(parent, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(parent, 0o755); err != nil {
					t.Fatal(err)
				}
				writePrepareFile(t, destination, plan.policyAction.before.data, plan.policyAction.before.mode.Perm())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newPrepareFixture(t)
			materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
			updated, err := NewSource("0.2.0", []byte("updated policy\n"))
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := PrepareUpdate(updated, fixture.environment, UpdateRequest{Targets: "claude"})
			if err != nil {
				t.Fatal(err)
			}
			test.change(t, prepared.plan)
			before := snapshotPrepareTree(t, fixture.root)
			if _, err := ApplyUpdate(prepared); err == nil || !strings.Contains(err.Error(), "identity changed after preparation") {
				t.Fatalf("error = %v", err)
			}
			if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
				t.Fatalf("identical path swap changed tree:\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

func TestMutationSecurityRechecksIdentityAtEachEffect(t *testing.T) {
	t.Run("file effect", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		materializeInstallRequest(t, fixture, InstallRequest{Targets: "claude"})
		updated, err := NewSource("0.2.0", []byte("updated policy\n"))
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := PrepareUpdate(updated, fixture.environment, UpdateRequest{Targets: "claude"})
		if err != nil {
			t.Fatal(err)
		}
		if err := validateMutationPlan(prepared.plan); err != nil {
			t.Fatal(err)
		}
		action := prepared.plan.policyAction
		moved := action.destination + ".prepared"
		if err := os.Rename(action.destination, moved); err != nil {
			t.Fatal(err)
		}
		writePrepareFile(t, action.destination, action.before.data, action.before.mode.Perm())
		if _, _, err := applyMutationAction(prepared.plan, action); err == nil || !strings.Contains(err.Error(), "identity changed") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("directory effect", func(t *testing.T) {
		fixture := newPrepareFixture(t)
		project := filepath.Join(fixture.root, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatal(err)
		}
		materializeInstallRequest(t, fixture, InstallRequest{Project: project, Targets: "cursor"})
		prepared, err := PrepareUninstall(fixture.environment, UninstallRequest{Project: project, Targets: "cursor"})
		if err != nil {
			t.Fatal(err)
		}
		if err := validateMutationPlan(prepared.plan); err != nil {
			t.Fatal(err)
		}
		action := prepared.plan.directoryActions[0]
		moved := action.destination + ".prepared"
		if err := os.Rename(action.destination, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(action.destination, action.before.mode.Perm()); err != nil {
			t.Fatal(err)
		}
		if removed, err := scopedRemoveMutationDirectory(action); err == nil || removed || !strings.Contains(err.Error(), "identity changed") {
			t.Fatalf("removed=%t error=%v", removed, err)
		}
	})

	t.Run("uninspectable file effect", func(t *testing.T) {
		root := t.TempDir()
		destination := filepath.Join(root, strings.Repeat("x", 1024))
		action := mutationAction{effect: mutationRemove, destination: destination}
		if _, _, err := applyMutationAction(mutationPlan{}, action); err == nil || !strings.Contains(err.Error(), "could not be inspected") {
			t.Fatalf("error = %v", err)
		}
	})
}
