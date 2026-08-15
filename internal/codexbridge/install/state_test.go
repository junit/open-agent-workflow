package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallStateIsClosedAndDigestPinned(t *testing.T) {
	state := validInstallState(t)
	encoded, err := EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeState(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Digest) != 64 || decoded.Digest == strings.Repeat("0", 64) {
		t.Fatalf("state digest = %q", decoded.Digest)
	}

	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	object["unknown"] = true
	mutated, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeState(mutated); Code(err) != "BRIDGE_INSTALL_STATE_INVALID" {
		t.Fatalf("unknown state field error = %v", err)
	}

	delete(object, "unknown")
	object["bridge_version"] = "2.0.0"
	mutated, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeState(mutated); Code(err) != "BRIDGE_INSTALL_STATE_INVALID" {
		t.Fatalf("digest mismatch error = %v", err)
	}
}

func TestEncodeStateIsDeterministicAndDoesNotMutateInput(t *testing.T) {
	state := validInstallState(t)
	before := state
	before.Files = append([]OwnedFile(nil), state.Files...)
	first, err := EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("state encoding is not deterministic:\n%s\n%s", first, second)
	}
	if state.Digest != before.Digest || state.Files[0] != before.Files[0] {
		t.Fatalf("EncodeState mutated input: before=%#v after=%#v", before, state)
	}
}

func TestInstallStateRejectsUnsafeOwnedFiles(t *testing.T) {
	for _, path := range []string{"", "/absolute", "../escape", "plugins/../escape", `plugins\escape`} {
		t.Run(strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			state := validInstallState(t)
			state.Files[0].Path = path
			if _, err := EncodeState(state); Code(err) != "BRIDGE_INSTALL_STATE_INVALID" {
				t.Fatalf("path %q error = %v", path, err)
			}
		})
	}
}

func TestInstallStateRejectsInvalidRecords(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*InstallState)
	}{
		{name: "schema", mutate: func(state *InstallState) { state.SchemaVersion = "old" }},
		{name: "version", mutate: func(state *InstallState) { state.BridgeVersion = "latest" }},
		{name: "binary digest", mutate: func(state *InstallState) { state.BinaryDigest = "bad" }},
		{name: "identity", mutate: func(state *InstallState) { state.PluginName = "other" }},
		{name: "duplicate", mutate: func(state *InstallState) { state.Files = append(state.Files, state.Files[0]) }},
		{name: "mode", mutate: func(state *InstallState) { state.Files[0].Mode = 0o1000 }},
		{name: "timestamp", mutate: func(state *InstallState) { state.InstalledAt = "not-a-time" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := validInstallState(t)
			test.mutate(&state)
			if _, err := EncodeState(state); Code(err) != "BRIDGE_INSTALL_STATE_INVALID" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestNewEnvironmentUsesOnlyOAWOwnedXDGCoordinates(t *testing.T) {
	root := t.TempDir()
	stateHome := filepath.Join(root, "state")
	dataHome := filepath.Join(root, "data")
	projectRoot := filepath.Join(root, "project")
	environment, err := NewEnvironment(stateHome, dataHome, "codex", projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	wantStateRoot := filepath.Join(stateHome, "open-agent-workflow", "codex-bridge")
	wantDataRoot := filepath.Join(dataHome, "open-agent-workflow", "codex-bridge")
	if environment.StateRoot != wantStateRoot || environment.StateFile != filepath.Join(wantStateRoot, "install.json") {
		t.Fatalf("state coordinates = %#v", environment)
	}
	if environment.DataRoot != wantDataRoot || strings.Contains(environment.DataRoot, ".codex") {
		t.Fatalf("data coordinates = %#v", environment)
	}
	if environment.CodexBinary != "codex" || environment.ProjectRoot != projectRoot {
		t.Fatalf("execution coordinates = %#v", environment)
	}
}

func TestNewEnvironmentRejectsUnsafeCoordinates(t *testing.T) {
	root := t.TempDir()
	for name, arguments := range map[string][]string{
		"relative state": {"relative", filepath.Join(root, "data"), "codex", root},
		"relative data":  {filepath.Join(root, "state"), "relative", "codex", root},
		"relative root":  {filepath.Join(root, "state"), filepath.Join(root, "data"), "codex", "relative"},
		"shell codex":    {filepath.Join(root, "state"), filepath.Join(root, "data"), "codex --help", root},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewEnvironment(arguments[0], arguments[1], arguments[2], arguments[3]); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestValidateEnvironmentRejectsMalformedProjection(t *testing.T) {
	environment := testEnvironment(t)
	tests := []struct {
		name   string
		mutate func(*Environment)
	}{
		{name: "state root", mutate: func(value *Environment) { value.StateRoot = "relative" }},
		{name: "data root", mutate: func(value *Environment) { value.DataRoot = "relative" }},
		{name: "project root", mutate: func(value *Environment) { value.ProjectRoot = "relative" }},
		{name: "state file", mutate: func(value *Environment) {
			value.StateFile = filepath.Join(filepath.Dir(value.StateRoot), "install.json")
		}},
		{name: "codex binary", mutate: func(value *Environment) { value.CodexBinary = "codex --help" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := environment
			test.mutate(&candidate)
			if err := ValidateEnvironment(candidate); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestWriteAndReadStateUsesAtomicPrivateFile(t *testing.T) {
	environment := testEnvironment(t)
	state := stateForEnvironment(t, environment)
	if err := WriteState(environment, state); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(environment.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %s", info.Mode())
	}
	decoded, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.BinaryPath != state.BinaryPath || decoded.MarketplacePath != state.MarketplacePath || decoded.Digest == "" {
		t.Fatalf("decoded state = %#v", decoded)
	}
	entries, err := os.ReadDir(environment.StateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "install.json" {
		t.Fatalf("state root entries = %#v", entries)
	}
}

func TestWriteStateRejectsSymlinkedManagedDirectory(t *testing.T) {
	root := t.TempDir()
	stateHome := filepath.Join(root, "state")
	dataHome := filepath.Join(root, "data")
	projectRoot := filepath.Join(root, "project")
	for _, directory := range []string{stateHome, dataHome, projectRoot, filepath.Join(root, "redirect")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "redirect"), filepath.Join(stateHome, "open-agent-workflow")); err != nil {
		t.Fatal(err)
	}
	environment, err := NewEnvironment(stateHome, dataHome, "codex", projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	state := stateForEnvironment(t, environment)
	if err := WriteState(environment, state); Code(err) != "BRIDGE_INSTALL_PATH_UNSAFE" {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "redirect", "codex-bridge", "install.json")); !os.IsNotExist(err) {
		t.Fatalf("write followed symlink: %v", err)
	}
}

func TestWriteStateRejectsCoordinatesOutsideDataRoot(t *testing.T) {
	environment := testEnvironment(t)
	state := stateForEnvironment(t, environment)
	state.BinaryPath = filepath.Join(filepath.Dir(environment.DataRoot), "outside", "oaw")
	if err := WriteState(environment, state); Code(err) != "BRIDGE_INSTALL_STATE_INVALID" {
		t.Fatalf("error = %v", err)
	}
}

func TestRemoveStateRejectsConcurrentStateReplacement(t *testing.T) {
	environment := testEnvironment(t)
	first := stateForEnvironment(t, environment)
	if err := WriteState(environment, first); err != nil {
		t.Fatal(err)
	}
	installed, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	second := stateForEnvironment(t, environment)
	second.BridgeVersion = "2.0.0"
	if err := ReplaceState(environment, second, installed.Digest); err != nil {
		t.Fatal(err)
	}
	if err := RemoveState(environment, installed.Digest); Code(err) != "BRIDGE_INSTALL_STATE_CONFLICT" {
		t.Fatalf("error = %v", err)
	}
	current, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	if current.BridgeVersion != "2.0.0" {
		t.Fatalf("concurrent state was removed or replaced: %#v", current)
	}
}

func TestRemoveStateIsIdempotentAndRejectsUnsafeInputs(t *testing.T) {
	environment := testEnvironment(t)
	digest := strings.Repeat("a", 64)
	if err := RemoveState(environment, "invalid"); Code(err) != "BRIDGE_INSTALL_STATE_INVALID" {
		t.Fatalf("invalid digest error = %v", err)
	}
	if err := RemoveState(environment, digest); err != nil {
		t.Fatalf("missing root error = %v", err)
	}
	if err := os.MkdirAll(environment.StateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RemoveState(environment, digest); err != nil {
		t.Fatalf("missing state error = %v", err)
	}
	target := filepath.Join(filepath.Dir(environment.StateRoot), "foreign-state")
	if err := os.WriteFile(target, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, environment.StateFile); err != nil {
		t.Fatal(err)
	}
	if err := RemoveState(environment, digest); Code(err) != "BRIDGE_INSTALL_PATH_UNSAFE" {
		t.Fatalf("symlink error = %v", err)
	}
	if content, err := os.ReadFile(target); err != nil || string(content) != "foreign" {
		t.Fatalf("foreign state changed: %q, %v", content, err)
	}
}

func TestStatePublicationRequiresCreateOrMatchingReplace(t *testing.T) {
	environment := testEnvironment(t)
	state := stateForEnvironment(t, environment)
	if err := WriteState(environment, state); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(environment, state); Code(err) != "BRIDGE_INSTALL_STATE_CONFLICT" {
		t.Fatalf("second create error = %v", err)
	}
	state.BridgeVersion = "2.0.0"
	if err := ReplaceState(environment, state, strings.Repeat("f", 64)); Code(err) != "BRIDGE_INSTALL_STATE_CONFLICT" {
		t.Fatalf("mismatched replace error = %v", err)
	}
}

func TestReadStateRejectsUnsafePermissions(t *testing.T) {
	environment := testEnvironment(t)
	if err := WriteState(environment, stateForEnvironment(t, environment)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(environment.StateFile, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadState(environment); Code(err) != "BRIDGE_INSTALL_PATH_UNSAFE" {
		t.Fatalf("error = %v", err)
	}
}

func testEnvironment(t *testing.T) Environment {
	t.Helper()
	root := t.TempDir()
	stateHome := filepath.Join(root, "state")
	dataHome := filepath.Join(root, "data")
	projectRoot := filepath.Join(root, "project")
	for _, directory := range []string{stateHome, dataHome, projectRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	environment, err := NewEnvironment(stateHome, dataHome, "codex", projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	return environment
}

func stateForEnvironment(t *testing.T, environment Environment) InstallState {
	t.Helper()
	state := validInstallState(t)
	state.BinaryPath = filepath.Join(environment.DataRoot, "bin", "oaw-bridge")
	state.MarketplacePath = filepath.Join(environment.DataRoot, "marketplace")
	return state
}

func validInstallState(t *testing.T) InstallState {
	t.Helper()
	root := t.TempDir()
	return InstallState{
		SchemaVersion:   InstallStateSchemaV1,
		BridgeVersion:   testVersion,
		BinaryPath:      filepath.Join(root, "data", "bin", "oaw-bridge"),
		BinaryDigest:    strings.Repeat("a", 64),
		MarketplacePath: filepath.Join(root, "data", "marketplace"),
		MarketplaceName: MarketplaceName,
		PluginName:      PluginName,
		Files: []OwnedFile{
			{Path: "plugins/oaw-codex-assurance/.mcp.json", Digest: strings.Repeat("b", 64), Mode: 0o600},
		},
		CodexPluginID: PluginName + "@" + MarketplaceName,
		InstalledAt:   "2026-08-07T12:00:00Z",
	}
}
