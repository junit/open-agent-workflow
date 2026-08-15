package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckNeverClaimsCurrentSessionLoaded(t *testing.T) {
	environment := testEnvironment(t)
	runner := &recordingRunner{}
	result, err := Check(context.Background(), environment, runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentSessionLoaded {
		t.Fatal("installation check inferred an active Codex session")
	}
	if !hasDiagnostic(result.Diagnostics, "BRIDGE_INSTALL_NOT_INSTALLED") {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

func TestCheckFailsClosedOnInvalidEnvironmentAndCodexInventoryErrors(t *testing.T) {
	environment := testEnvironment(t)
	invalid := environment
	invalid.StateFile = filepath.Join(filepath.Dir(environment.StateRoot), "install.json")
	if _, err := Check(context.Background(), invalid, &recordingRunner{}); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
		t.Fatalf("invalid environment error = %v", err)
	}
	for _, command := range []string{"plugin marketplace", "plugin list"} {
		t.Run(command, func(t *testing.T) {
			runner := &recordingRunner{Failures: map[string]error{command: errors.New("inventory unavailable")}}
			if _, err := Check(context.Background(), environment, runner); err == nil {
				t.Fatal("inventory failure was accepted")
			}
		})
	}
}

func TestCodexStatusProjectionRejectsDuplicateAuthority(t *testing.T) {
	marketplace := MarketplaceRecord{Name: MarketplaceName, Root: "/marketplace", SourceType: "local"}
	marketplaceStatus := projectMarketplaceStatus([]MarketplaceRecord{marketplace, marketplace})
	if marketplaceStatus.Registered || marketplaceStatus.Root != "" || marketplaceStatus.SourceType != "" {
		t.Fatalf("duplicate marketplace status = %#v", marketplaceStatus)
	}
	plugin := PluginRecord{
		PluginID: PluginName + "@" + MarketplaceName, Name: PluginName,
		MarketplaceName: MarketplaceName, Version: "1.0.0", Installed: true, Enabled: true,
	}
	pluginStatus := projectPluginStatus([]PluginRecord{plugin, plugin})
	if pluginStatus.Installed || pluginStatus.Enabled || pluginStatus.Version != "" {
		t.Fatalf("duplicate Plugin status = %#v", pluginStatus)
	}
}

func TestCheckReportsFilesAndExactCodexIdentity(t *testing.T) {
	environment, runner, state := installedFixture(t)
	runner.Results = map[string]CLIResult{
		"plugin list":        {Stdout: []byte(`{"installed":[{"pluginId":"oaw-codex-assurance@oaw-local","name":"oaw-codex-assurance","marketplaceName":"oaw-local","version":"1.0.0","installed":true,"enabled":true}]}`)},
		"plugin marketplace": {Stdout: []byte(`{"marketplaces":[{"name":"oaw-local","root":"` + state.MarketplacePath + `","marketplaceSource":{"sourceType":"local","source":"` + state.MarketplacePath + `"}}]}`)},
	}
	result, err := Check(context.Background(), environment, runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.CurrentSessionLoaded || !result.RequiresNewSession || len(result.Files) != len(state.Files) {
		t.Fatalf("result = %#v", result)
	}
	if !result.CodexMarketplace.Registered || result.CodexMarketplace.Name != MarketplaceName || !result.CodexPlugin.Installed || result.CodexPlugin.PluginID != PluginName+"@"+MarketplaceName {
		t.Fatalf("Codex status = marketplace:%#v plugin:%#v", result.CodexMarketplace, result.CodexPlugin)
	}
	for _, file := range result.Files {
		if file.Status != "clean" || file.Path == "" || file.PathDigest == "" {
			t.Fatalf("file status = %#v", file)
		}
	}
}

func TestCheckReportsDriftWithoutExposingPathInDiagnostic(t *testing.T) {
	environment, runner, state := installedFixture(t)
	drifted := filepath.Join(environment.DataRoot, filepath.FromSlash(state.Files[0].Path))
	if err := os.WriteFile(drifted, []byte("user edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Check(context.Background(), environment, runner)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(result.Diagnostics, "BRIDGE_INSTALL_DRIFT") {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.PathDigest == "" && diagnostic.Code == "BRIDGE_INSTALL_DRIFT" {
			t.Fatalf("drift diagnostic lacks path digest: %#v", diagnostic)
		}
		if diagnostic.Message == drifted {
			t.Fatalf("diagnostic exposes absolute path: %#v", diagnostic)
		}
	}
}

func TestCheckReportsEveryOwnedFileMissingWhenDataRootDisappears(t *testing.T) {
	environment, runner, state := installedFixture(t)
	if err := os.RemoveAll(environment.DataRoot); err != nil {
		t.Fatal(err)
	}
	result, err := Check(context.Background(), environment, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != len(state.Files) {
		t.Fatalf("files = %#v", result.Files)
	}
	for _, file := range result.Files {
		if file.Status != "missing" || file.PathDigest == "" {
			t.Fatalf("file = %#v", file)
		}
	}
	if !hasDiagnostic(result.Diagnostics, "BRIDGE_INSTALL_MISSING") {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}

func TestCheckReportsEveryOwnedFileUnsafeWhenDataRootIsReplacedBySymlink(t *testing.T) {
	environment, runner, state := installedFixture(t)
	if err := os.RemoveAll(environment.DataRoot); err != nil {
		t.Fatal(err)
	}
	redirect := filepath.Join(filepath.Dir(filepath.Dir(environment.DataRoot)), "redirect")
	if err := os.MkdirAll(redirect, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(redirect, environment.DataRoot); err != nil {
		t.Fatal(err)
	}
	result, err := Check(context.Background(), environment, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != len(state.Files) {
		t.Fatalf("files = %#v", result.Files)
	}
	for _, file := range result.Files {
		if file.Status != "unsafe" || file.PathDigest == "" {
			t.Fatalf("file = %#v", file)
		}
	}
	if !hasDiagnostic(result.Diagnostics, "BRIDGE_INSTALL_DRIFT") {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
}
