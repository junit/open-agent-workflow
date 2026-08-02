package management

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type prepareFixture struct {
	root        string
	environment Environment
	source      Source
}

func newPrepareFixture(t *testing.T) prepareFixture {
	t.Helper()
	root := t.TempDir()
	environment := Environment{
		Home:       filepath.Join(root, "home"),
		ConfigHome: filepath.Join(root, "config"),
		StateHome:  filepath.Join(root, "state"),
		Path:       filepath.Join(root, "bin"),
	}
	for _, directory := range []string{environment.Home, environment.ConfigHome, environment.StateHome, environment.Path} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewSource("0.1.0", []byte("canonical policy\n"))
	if err != nil {
		t.Fatal(err)
	}
	return prepareFixture{root: root, environment: environment, source: source}
}

func prepareWithoutWrites(t *testing.T, root string, source Source, environment Environment, request InstallRequest) (PreparedInstall, error) {
	t.Helper()
	before := snapshotPrepareTree(t, root)
	prepared, err := PrepareInstall(source, environment, request)
	after := snapshotPrepareTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("PrepareInstall() changed fixture:\nbefore=%#v\nafter=%#v", before, after)
	}
	return prepared, err
}

type prepareTreeEntry struct {
	mode fs.FileMode
	data string
}

func snapshotPrepareTree(t *testing.T, root string) map[string]prepareTreeEntry {
	t.Helper()
	result := make(map[string]prepareTreeEntry)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		value := prepareTreeEntry{mode: info.Mode()}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			value.data, err = os.Readlink(path)
		case info.Mode().IsRegular():
			var data []byte
			data, err = os.ReadFile(path)
			value.data = string(data)
		}
		if err != nil {
			return err
		}
		result[relative] = value
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot tree: %v", err)
	}
	return result
}

func materializePreparedFixture(t *testing.T, prepared PreparedInstall) {
	t.Helper()
	for _, directory := range prepared.plannedDirectories {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, action := range append(append([]installAction(nil), prepared.targetActions...), prepared.policyAction) {
		writePrepareFile(t, action.destination, action.data, action.mode)
	}
	for _, action := range prepared.stateActions {
		writePrepareFile(t, action.destination, action.data, action.mode)
	}
}

func writePrepareFile(t *testing.T, path string, data []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func parsePreparedState(t *testing.T, action installAction) installationState {
	t.Helper()
	state, err := parseInstallationState(action.data)
	if err != nil {
		t.Fatalf("parse prepared state %s: %v\n%s", action.label, err, action.data)
	}
	return state
}

func actionLabels(actions []installAction) []string {
	result := make([]string, len(actions))
	for index, action := range actions {
		result[index] = action.label
	}
	return result
}

func targetRecordIDs(records []targetRecord) []string {
	result := make([]string, len(records))
	for index, record := range records {
		result[index] = record.id
	}
	return result
}

func findPreparedRecord(t *testing.T, records []targetRecord, id string) targetRecord {
	t.Helper()
	for _, record := range records {
		if record.id == id {
			return record
		}
	}
	t.Fatalf("record %q not found", id)
	return targetRecord{}
}

func semanticPreparedSnapshot(prepared PreparedInstall) string {
	var result strings.Builder
	for _, action := range append(append([]installAction(nil), prepared.targetActions...), prepared.policyAction) {
		fmt.Fprintf(&result, "%s\x00%s\x00%d\x00%s\n", action.label, action.destination, action.mode, action.data)
	}
	for _, action := range prepared.stateActions {
		fmt.Fprintf(&result, "%s\x00%s\x00%d\x00%s\n", action.label, action.destination, action.mode, action.data)
	}
	fmt.Fprintf(&result, "%q\n%q", prepared.plannedDirectories, prepared.predicted.Lines)
	return result.String()
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
