package install

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPublishesOwnedFilesAndState(t *testing.T) {
	environment := testEnvironment(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source", "binary-v1")
	runner := &recordingRunner{}

	result, err := Install(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || !result.RequiresNewSession || result.Operation != "install" {
		t.Fatalf("result = %#v", result)
	}
	state, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	if state.BridgeVersion != "1.0.0" || len(state.Files) != 6 {
		t.Fatalf("state = %#v", state)
	}
	content, err := os.ReadFile(state.BinaryPath)
	if err != nil || string(content) != "binary-v1" {
		t.Fatalf("managed binary = %q, %v", content, err)
	}
	if !runner.Saw("plugin marketplace add ") || !runner.Saw("plugin add "+PluginName+"@"+MarketplaceName) {
		t.Fatalf("commands = %#v", runner.Commands)
	}
}

func TestInstallDryRunDoesNotWriteOrInvokeCodex(t *testing.T) {
	environment := testEnvironment(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source", "binary-v1")
	runner := &recordingRunner{}
	result, err := Install(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "1.0.0", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || result.Operation != "install" || !result.RequiresNewSession {
		t.Fatalf("result = %#v", result)
	}
	if len(runner.Commands) != 0 {
		t.Fatalf("dry run invoked Codex: %#v", runner.Commands)
	}
	if _, err := os.Stat(environment.StateFile); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dry run wrote state: %v", err)
	}
}

func TestInstallDoesNotReplayOverExistingState(t *testing.T) {
	environment, runner, _ := installedFixture(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-replay", "binary-replay")
	before := len(runner.Commands)
	if _, err := Install(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "1.0.1"}); Code(err) != "BRIDGE_INSTALL_ALREADY_INSTALLED" {
		t.Fatalf("error = %v", err)
	}
	if len(runner.Commands) != before {
		t.Fatalf("replay invoked Codex: %#v", runner.Commands[before:])
	}
}

func TestInstallRollsBackMarketplaceWhenPluginAddFails(t *testing.T) {
	environment := testEnvironment(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source", "binary-v1")
	runner := &recordingRunner{Failures: map[string]error{"plugin add": errors.New("denied")}}
	_, err := Install(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "1.0.0"})
	if Code(err) != "BRIDGE_INSTALL_ROLLBACK" || !runner.Saw("plugin remove") || !runner.Saw("plugin marketplace remove") {
		t.Fatalf("error = %v, calls = %#v", err, runner.Commands)
	}
	if _, statErr := os.Stat(environment.StateFile); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("state remains: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(environment.DataRoot, "bin", "oaw")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("managed binary remains: %v", statErr)
	}
}

func TestInstallRollsBackPartialMarketplaceAdd(t *testing.T) {
	environment := testEnvironment(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source", "binary-v1")
	runner := &recordingRunner{FailureSequences: map[string][]error{
		"plugin marketplace": {errors.New("partial add"), nil},
	}}
	if _, err := Install(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "1.0.0"}); Code(err) != "BRIDGE_INSTALL_ROLLBACK" {
		t.Fatalf("error = %v, commands = %#v", err, runner.Commands)
	}
	if runner.Saw("plugin remove") || !runner.Saw("plugin marketplace remove") {
		t.Fatalf("rollback commands = %#v", runner.Commands)
	}
}

func TestInstallRejectsUnsafeSourceAndExistingDestination(t *testing.T) {
	environment := testEnvironment(t)
	nonExecutable := filepath.Join(environment.ProjectRoot, "not-executable")
	if err := os.WriteFile(nonExecutable, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), environment, &recordingRunner{}, InstallRequest{Binary: nonExecutable, Version: "1.0.0"}); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
		t.Fatalf("non-executable error = %v", err)
	}
	link := filepath.Join(environment.ProjectRoot, "binary-link")
	if err := os.Symlink(nonExecutable, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), environment, &recordingRunner{}, InstallRequest{Binary: link, Version: "1.0.0"}); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
		t.Fatalf("symlink error = %v", err)
	}
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source", "binary-v1")
	if err := os.MkdirAll(filepath.Join(environment.DataRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), environment, &recordingRunner{}, InstallRequest{Binary: binary, Version: "1.0.0"}); Code(err) != "BRIDGE_INSTALL_ALREADY_INSTALLED" {
		t.Fatalf("existing destination error = %v", err)
	}
}

func TestPublishConflictCleanupRemovesOnlyStagingDirectory(t *testing.T) {
	environment := testEnvironment(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source", "binary-v1")
	prepared, err := preparePayload(environment, InstallRequest{Binary: binary, Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := stagePayload(environment, prepared, true)
	if err != nil {
		t.Fatal(err)
	}
	stagePath := filepath.Join(environment.DataRoot, staged.name)
	if err := os.Mkdir(filepath.Join(environment.DataRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := publishFreshPayload(environment, staged); Code(err) != "BRIDGE_INSTALL_ALREADY_INSTALLED" {
		t.Fatalf("publish error = %v", err)
	}
	cleanupStaging(environment, "bin")
	if _, err := os.Stat(stagePath); err != nil {
		t.Fatalf("non-staging cleanup changed staged payload: %v", err)
	}
	cleanupStaging(environment, staged.name)
	if _, err := os.Stat(stagePath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("staging directory remains: %v", err)
	}
	if info, err := os.Stat(filepath.Join(environment.DataRoot, "bin")); err != nil || !info.IsDir() {
		t.Fatalf("conflicting destination changed: %v", err)
	}
}

func TestManagedPayloadInspectionRejectsUnsafeFiles(t *testing.T) {
	environment := testEnvironment(t)
	root, err := openOrCreateManagedRoot(environment.DataRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := writeNewRootFile(root, "bin/oaw", []byte("binary"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ensureRootDirectories(root, "marketplace"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../bin/oaw", filepath.Join(environment.DataRoot, "marketplace", "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := listPayloadFiles(root); Code(err) != "BRIDGE_INSTALL_PATH_UNSAFE" {
		t.Fatalf("payload symlink error = %v", err)
	}
	if _, _, err := digestRootFile(root, "marketplace/linked"); Code(err) != "BRIDGE_INSTALL_PATH_UNSAFE" {
		t.Fatalf("digest symlink error = %v", err)
	}
	oversized := filepath.Join(environment.DataRoot, "oversized")
	file, err := os.OpenFile(oversized, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maximumBridgeBinaryBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := digestRootFile(root, "oversized"); Code(err) != "BRIDGE_INSTALL_PATH_UNSAFE" {
		t.Fatalf("oversized digest error = %v", err)
	}
}

func TestInstallPreservesRecoveryStateWhenRollbackCommandFails(t *testing.T) {
	environment := testEnvironment(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source", "binary-v1")
	runner := &recordingRunner{Failures: map[string]error{
		"plugin add":    errors.New("denied"),
		"plugin remove": errors.New("still registered"),
	}}
	_, err := Install(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "1.0.0"})
	if Code(err) != "BRIDGE_INSTALL_ROLLBACK_INCOMPLETE" {
		t.Fatalf("error = %v", err)
	}
	if _, stateErr := ReadState(environment); stateErr != nil {
		t.Fatalf("recovery state missing: %v", stateErr)
	}
	if _, statErr := os.Stat(filepath.Join(environment.DataRoot, "bin", "oaw")); statErr != nil {
		t.Fatalf("recovery files missing: %v", statErr)
	}
}

func TestUpdateReplacesCleanInstallationWithoutMarketplaceUpgrade(t *testing.T) {
	environment, runner, _ := installedFixture(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v2", "binary-v2")
	before := len(runner.Commands)
	result, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation != "update" || !result.Changed || !result.RequiresNewSession {
		t.Fatalf("result = %#v", result)
	}
	state, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	if state.BridgeVersion != "2.0.0" {
		t.Fatalf("state version = %q", state.BridgeVersion)
	}
	content, err := os.ReadFile(state.BinaryPath)
	if err != nil || string(content) != "binary-v2" {
		t.Fatalf("managed binary = %q, %v", content, err)
	}
	commands := runner.Commands[before:]
	if !commandsContain(commands, "plugin remove "+PluginName+"@"+MarketplaceName) || !commandsContain(commands, "plugin add "+PluginName+"@"+MarketplaceName) {
		t.Fatalf("update commands = %#v", commands)
	}
	if commandsContain(commands, "marketplace upgrade") {
		t.Fatalf("local marketplace upgrade invoked: %#v", commands)
	}
}

func TestUpdateDryRunAndUnrecordedFileAreNonMutating(t *testing.T) {
	environment, runner, state := installedFixture(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v2", "binary-v2")
	before := len(runner.Commands)
	result, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || result.Operation != "update" {
		t.Fatalf("result = %#v", result)
	}
	if commandsContain(runner.Commands[before:], "plugin remove") || commandsContain(runner.Commands[before:], "plugin add") {
		t.Fatalf("dry-run mutation commands = %#v", runner.Commands[before:])
	}
	extra := filepath.Join(state.MarketplacePath, "unrecorded.txt")
	if err := os.WriteFile(extra, []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}
	before = len(runner.Commands)
	if _, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0"}); Code(err) != "BRIDGE_INSTALL_DRIFT" {
		t.Fatalf("unrecorded file error = %v", err)
	}
	if len(runner.Commands) != before {
		t.Fatalf("unrecorded file invoked Codex: %#v", runner.Commands[before:])
	}
}

func TestUpdateRejectsDriftBeforeInvokingCodex(t *testing.T) {
	environment, runner, state := installedFixture(t)
	drifted := filepath.Join(environment.DataRoot, filepath.FromSlash(state.Files[0].Path))
	if err := os.WriteFile(drifted, []byte("user edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v2", "binary-v2")
	before := len(runner.Commands)
	if _, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0"}); Code(err) != "BRIDGE_INSTALL_DRIFT" {
		t.Fatalf("error = %v", err)
	}
	if len(runner.Commands) != before {
		t.Fatalf("Codex invoked after drift: %#v", runner.Commands[before:])
	}
}

func TestUpdateRestoresPreviousPayloadWhenPluginAddFails(t *testing.T) {
	environment, installedRunner, previous := installedFixture(t)
	runner := &recordingRunner{
		Results: installedRunner.Results,
		FailureSequences: map[string][]error{
			"plugin add": {errors.New("new Plugin rejected"), nil},
		},
	}
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v2", "binary-v2")
	if _, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0"}); Code(err) != "BRIDGE_INSTALL_ROLLBACK" {
		t.Fatalf("error = %v, commands = %#v", err, runner.Commands)
	}
	restored, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	if restored.BridgeVersion != previous.BridgeVersion || restored.BinaryDigest != previous.BinaryDigest {
		t.Fatalf("restored state = %#v, previous = %#v", restored, previous)
	}
	content, err := os.ReadFile(restored.BinaryPath)
	if err != nil || string(content) != "binary-v1" {
		t.Fatalf("restored binary = %q, %v", content, err)
	}
	entries, err := os.ReadDir(environment.DataRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".backup-") || strings.HasPrefix(entry.Name(), ".stage-") {
			t.Fatalf("transaction directory remains: %q", entry.Name())
		}
	}
}

func TestUpdateRestoresPreviousPayloadWhenPluginRemoveFails(t *testing.T) {
	environment, installedRunner, previous := installedFixture(t)
	runner := &recordingRunner{
		Results: installedRunner.Results,
		FailureSequences: map[string][]error{
			"plugin remove": {errors.New("old Plugin removal denied")},
		},
	}
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v2", "binary-v2")
	if _, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0"}); Code(err) != "BRIDGE_INSTALL_ROLLBACK" {
		t.Fatalf("error = %v, commands = %#v", err, runner.Commands)
	}
	restored, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Digest != previous.Digest || restored.BinaryDigest != previous.BinaryDigest {
		t.Fatalf("restored state = %#v, previous = %#v", restored, previous)
	}
	content, err := os.ReadFile(restored.BinaryPath)
	if err != nil || string(content) != "binary-v1" {
		t.Fatalf("restored binary = %q, %v", content, err)
	}
	if !runner.Saw("plugin add") {
		t.Fatalf("previous Plugin was not re-added: %#v", runner.Commands)
	}
}

func TestUpdatePreservesConcurrentPayloadDriftWhenRollbackCannotRestore(t *testing.T) {
	environment, installedRunner, previous := installedFixture(t)
	delegate := &recordingRunner{
		Results:  installedRunner.Results,
		Failures: map[string]error{"plugin add": errors.New("new Plugin rejected")},
	}
	var mutationErr error
	mutated := false
	runner := &callbackRunner{
		delegate: delegate,
		before: func(arguments []string) {
			if !mutated && commandKey(arguments) == "plugin add" {
				mutated = true
				mutationErr = os.WriteFile(previous.BinaryPath, []byte("concurrent edit"), 0o700)
			}
		},
	}
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v2", "binary-v2")
	if _, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0"}); Code(err) != "BRIDGE_INSTALL_ROLLBACK_INCOMPLETE" {
		t.Fatalf("error = %v, commands = %#v", err, delegate.Commands)
	}
	if mutationErr != nil {
		t.Fatal(mutationErr)
	}
	content, err := os.ReadFile(previous.BinaryPath)
	if err != nil || string(content) != "concurrent edit" {
		t.Fatalf("concurrent edit was not preserved: %q, %v", content, err)
	}
	state, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	if state.Digest != previous.Digest {
		t.Fatalf("previous authority state changed: %#v", state)
	}
	foundBackup := false
	entries, err := os.ReadDir(environment.DataRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		foundBackup = foundBackup || strings.HasPrefix(entry.Name(), ".backup-")
	}
	if !foundBackup {
		t.Fatal("recovery backup was removed after incomplete rollback")
	}
}

func TestUpdateRejectsCodexAuthorityMismatchBeforeSwap(t *testing.T) {
	environment, runner, state := installedFixture(t)
	state.BridgeVersion = "9.9.9"
	runner.Results = inventoryResults(t, state)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v2", "binary-v2")
	before := len(runner.Commands)
	if _, err := Update(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "2.0.0"}); Code(err) != "BRIDGE_INSTALL_AUTHORITY_MISMATCH" {
		t.Fatalf("error = %v", err)
	}
	for _, command := range runner.Commands[before:] {
		joined := strings.Join(command, " ")
		if strings.Contains(joined, "plugin remove") || strings.Contains(joined, "plugin add") {
			t.Fatalf("authority mismatch mutated Codex: %#v", runner.Commands[before:])
		}
	}
}

func TestUninstallRemovesOnlyRecordedCleanFiles(t *testing.T) {
	environment, runner, state := installedFixture(t)
	unrelated := filepath.Join(environment.DataRoot, "unrelated.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := len(runner.Commands)
	result, err := Uninstall(context.Background(), environment, runner, UninstallRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || !result.RequiresNewSession || len(result.Diagnostics) != 0 {
		t.Fatalf("result = %#v", result)
	}
	for _, owned := range state.Files {
		if _, err := os.Stat(filepath.Join(environment.DataRoot, filepath.FromSlash(owned.Path))); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("owned file %q remains: %v", owned.Path, err)
		}
	}
	if content, err := os.ReadFile(unrelated); err != nil || string(content) != "keep" {
		t.Fatalf("unrelated file changed: %q, %v", content, err)
	}
	if _, err := os.Stat(environment.StateFile); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("state remains: %v", err)
	}
	commands := runner.Commands[before:]
	if !commandsContain(commands, "plugin remove") || !commandsContain(commands, "plugin marketplace remove") {
		t.Fatalf("cleanup commands = %#v", commands)
	}
}

func TestUninstallRemovesEmptyOAWRoots(t *testing.T) {
	environment, runner, _ := installedFixture(t)
	features := filepath.Join(environment.StateRoot, "features")
	if err := os.MkdirAll(features, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(features, "child-stale.json"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(context.Background(), environment, runner, UninstallRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(environment.DataRoot); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty OAW data root remains: %v", err)
	}
	if _, err := os.Stat(environment.StateRoot); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("empty OAW state root remains: %v", err)
	}
}

func TestUninstallPreservesDriftedUserFile(t *testing.T) {
	environment, runner, state := installedFixture(t)
	owned := filepath.Join(environment.DataRoot, filepath.FromSlash(state.Files[0].Path))
	if err := os.WriteFile(owned, []byte("user edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Uninstall(context.Background(), environment, runner, UninstallRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(result.Diagnostics, "BRIDGE_INSTALL_DRIFT") {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	if content, readErr := os.ReadFile(owned); readErr != nil || string(content) != "user edit" {
		t.Fatalf("drifted file changed: %q, %v", content, readErr)
	}
	if _, stateErr := ReadState(environment); stateErr != nil {
		t.Fatalf("ownership state removed: %v", stateErr)
	}
	if !runner.Saw("plugin remove") || !runner.Saw("plugin marketplace remove") {
		t.Fatalf("official cleanup missing: %#v", runner.Commands)
	}
}

func TestUninstallPreservesFilesWhenCodexCleanupFails(t *testing.T) {
	environment, runner, state := installedFixture(t)
	runner.Failures = map[string]error{"plugin remove": errors.New("denied")}
	if _, err := Uninstall(context.Background(), environment, runner, UninstallRequest{}); Code(err) != "BRIDGE_INSTALL_UNINSTALL_INCOMPLETE" {
		t.Fatalf("error = %v", err)
	}
	if _, err := ReadState(environment); err != nil {
		t.Fatalf("state removed: %v", err)
	}
	for _, owned := range state.Files {
		if _, err := os.Stat(filepath.Join(environment.DataRoot, filepath.FromSlash(owned.Path))); err != nil {
			t.Fatalf("owned file %q removed: %v", owned.Path, err)
		}
	}
}

func installedFixture(t *testing.T) (Environment, *recordingRunner, InstallState) {
	t.Helper()
	environment := testEnvironment(t)
	binary := writeTestBinary(t, environment.ProjectRoot, "oaw-source-v1", "binary-v1")
	runner := &recordingRunner{}
	if _, err := Install(context.Background(), environment, runner, InstallRequest{Binary: binary, Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	state, err := ReadState(environment)
	if err != nil {
		t.Fatal(err)
	}
	runner.Results = inventoryResults(t, state)
	return environment, runner, state
}

func inventoryResults(t *testing.T, state InstallState) map[string]CLIResult {
	t.Helper()
	plugins, err := json.Marshal(map[string]any{
		"installed": []map[string]any{{
			"pluginId": state.CodexPluginID, "name": state.PluginName,
			"marketplaceName": state.MarketplaceName, "version": state.BridgeVersion,
			"installed": true, "enabled": true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	marketplaces, err := json.Marshal(map[string]any{
		"marketplaces": []map[string]any{{
			"name": state.MarketplaceName, "root": state.MarketplacePath,
			"marketplaceSource": map[string]any{"sourceType": "local", "source": state.MarketplacePath},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]CLIResult{
		"plugin list":        {Stdout: plugins},
		"plugin marketplace": {Stdout: marketplaces},
	}
}

func writeTestBinary(t *testing.T, directory, name, content string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func commandsContain(commands [][]string, fragment string) bool {
	for _, command := range commands {
		if strings.Contains(strings.Join(command, " "), fragment) {
			return true
		}
	}
	return false
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

type callbackRunner struct {
	delegate *recordingRunner
	before   func([]string)
}

func (r *callbackRunner) Run(ctx context.Context, arguments ...string) (CLIResult, error) {
	if r.before != nil {
		r.before(arguments)
	}
	return r.delegate.Run(ctx, arguments...)
}
