package management

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInstallFaultRollbackRestoresTreeBeforeAndAfterEveryAction(t *testing.T) {
	prototypeFixture := newPrepareFixture(t)
	prototype, err := PrepareInstall(
		prototypeFixture.source,
		prototypeFixture.environment,
		InstallRequest{Targets: "claude,codex"},
	)
	if err != nil {
		t.Fatal(err)
	}
	actionCount := installFaultActionCount(prototype)
	points := make([]installFaultPoint, 0, actionCount*2)
	for index := range actionCount {
		points = append(points,
			installFaultPoint{moment: installFaultBefore, index: index},
			installFaultPoint{moment: installFaultAfter, index: index},
		)
	}

	for _, point := range points {
		point := point
		t.Run(point.String(), func(t *testing.T) {
			fixture := newPrepareFixture(t)
			existing := filepath.Join(fixture.environment.Home, ".claude", "CLAUDE.md")
			writePrepareFile(t, existing, []byte("user content\n"), 0o640)
			prepared, err := PrepareInstall(
				fixture.source,
				fixture.environment,
				InstallRequest{Targets: "claude,codex"},
			)
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotPrepareTree(t, fixture.root)
			result, err := applyInstall(prepared, func(actual installFaultPoint) error {
				if actual == point {
					return fmt.Errorf("injected install fault at %s", point.String())
				}
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), point.String()) {
				t.Fatalf("applyInstall() result=%#v error=%v", result, err)
			}
			wantLines := point.index
			if point.moment == installFaultAfter {
				wantLines++
			}
			if len(result.Lines) != wantLines {
				t.Fatalf("result lines at %s = %d, want %d", point.String(), len(result.Lines), wantLines)
			}
			if after := snapshotPrepareTree(t, fixture.root); !reflect.DeepEqual(before, after) {
				t.Fatalf("rollback differs at %s:\nbefore=%#v\nafter=%#v", point.String(), before, after)
			}
		})
	}
}

func TestInstallRollbackPreservesConcurrentForeignDirectoryContent(t *testing.T) {
	fixture := newPrepareFixture(t)
	prepared, err := PrepareInstall(
		fixture.source,
		fixture.environment,
		InstallRequest{Targets: "claude"},
	)
	if err != nil {
		t.Fatal(err)
	}
	native := prepared.targetActions[1]
	nativeIndex := 1 + len(prepared.policySetActions) + 1
	point := installFaultPoint{moment: installFaultAfter, index: nativeIndex}
	foreign := filepath.Join(filepath.Dir(native.destination), "foreign.txt")

	result, err := applyInstall(prepared, func(actual installFaultPoint) error {
		if actual != point {
			return nil
		}
		if err := os.WriteFile(foreign, []byte("foreign\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return fmt.Errorf("injected concurrent content at %s", point.String())
	})
	if err == nil || !strings.Contains(err.Error(), point.String()) {
		t.Fatalf("applyInstall() result=%#v error=%v", result, err)
	}
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "foreign\n" {
		t.Fatalf("foreign content = %q, %v", data, err)
	}

	for _, action := range installFaultActions(prepared) {
		if _, err := os.Lstat(action.destination); !os.IsNotExist(err) {
			t.Fatalf("rolled-back artifact remains at %s: %v", action.destination, err)
		}
	}
	foreignDirectory := filepath.Dir(foreign)
	for _, directory := range prepared.plannedDirectories {
		containsForeign := directory == foreignDirectory ||
			strings.HasPrefix(foreignDirectory, directory+string(filepath.Separator))
		_, err := os.Lstat(directory)
		if containsForeign {
			if err != nil {
				t.Fatalf("foreign content parent was removed: %s: %v", directory, err)
			}
			continue
		}
		if !os.IsNotExist(err) {
			t.Fatalf("empty created directory remains at %s: %v", directory, err)
		}
	}
}

func TestInstallRollbackPreservesReplacedCreatedDirectory(t *testing.T) {
	fixture := newPrepareFixture(t)
	prepared, err := PrepareInstall(
		fixture.source,
		fixture.environment,
		InstallRequest{Targets: "claude"},
	)
	if err != nil {
		t.Fatal(err)
	}
	native := prepared.targetActions[1]
	nativeIndex := 1 + len(prepared.policySetActions) + 1
	point := installFaultPoint{moment: installFaultAfter, index: nativeIndex}
	replacedDirectory := filepath.Dir(native.destination)
	var replacementIdentity os.FileInfo

	result, err := applyInstall(prepared, func(actual installFaultPoint) error {
		if actual != point {
			return nil
		}
		if removeErr := os.RemoveAll(replacedDirectory); removeErr != nil {
			t.Fatal(removeErr)
		}
		if mkdirErr := os.Mkdir(replacedDirectory, 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		replacementIdentity, err = os.Lstat(replacedDirectory)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Errorf("injected directory replacement at %s", point.String())
	})
	if err == nil || !strings.Contains(err.Error(), point.String()) ||
		!strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("applyInstall() result=%#v error=%v", result, err)
	}
	current, statErr := os.Lstat(replacedDirectory)
	if statErr != nil {
		t.Fatalf("foreign replacement directory was removed: %v", statErr)
	}
	if !os.SameFile(replacementIdentity, current) {
		t.Fatal("foreign replacement directory identity changed during rollback")
	}
}

func installFaultActionCount(prepared PreparedInstall) int {
	return len(installFaultActions(prepared))
}

func installFaultActions(prepared PreparedInstall) []installAction {
	actions := make([]installAction, 0, len(prepared.targetActions)+1+len(prepared.policySetActions)+len(prepared.stateActions))
	actions = append(actions, prepared.policyAction)
	actions = append(actions, prepared.policySetActions...)
	actions = append(actions, prepared.targetActions...)
	actions = append(actions, prepared.stateActions...)
	return actions
}
