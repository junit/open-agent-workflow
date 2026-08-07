package install

import (
	"context"
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

func TestCheckReportsFilesAndExactCodexIdentity(t *testing.T) {
	environment, runner, state := installedFixture(t)
	runner.Results = map[string]CLIResult{
		"plugin list":        {Stdout: []byte(`{"installed":[{"pluginId":"oaw-codex-host@oaw-local","name":"oaw-codex-host","marketplaceName":"oaw-local","version":"1.0.0","installed":true,"enabled":true}]}`)},
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
