package install

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestExecRunnerPassesArgumentsWithoutShellEvaluation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bridge v1 installation is unsupported on Windows")
	}
	runner := ExecRunner{Binary: "/usr/bin/printf", Environment: []string{}}
	result, err := runner.Run(context.Background(), "%s", "$(printf injected); echo changed")
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Stdout) != "$(printf injected); echo changed" || len(result.Stderr) != 0 || result.ExitCode != 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecRunnerCopiesExplicitEnvironmentWithoutMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bridge v1 installation is unsupported on Windows")
	}
	environment := []string{"OAW_RUNNER_TEST=present"}
	runner := ExecRunner{Binary: "/usr/bin/env", Environment: environment}
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Stdout) != "OAW_RUNNER_TEST=present\n" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	if !slices.Equal(environment, []string{"OAW_RUNNER_TEST=present"}) {
		t.Fatalf("environment mutated: %#v", environment)
	}
}

func TestExecRunnerProjectsExitAndLaunchFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bridge v1 installation is unsupported on Windows")
	}
	result, err := (ExecRunner{Binary: "/usr/bin/false", Environment: []string{}}).Run(context.Background(), "failure")
	if err == nil || result.ExitCode == 0 {
		t.Fatalf("false result = %#v, %v", result, err)
	}
	result, err = (ExecRunner{Binary: "/definitely/missing/codex", Environment: []string{}}).Run(context.Background(), "plugin")
	if err == nil || result.ExitCode != -1 {
		t.Fatalf("missing result = %#v, %v", result, err)
	}
	if _, err := (ExecRunner{Binary: "/usr/bin/false"}).Run(context.Background(), "bad\nargument"); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
		t.Fatalf("argument error = %v", err)
	}
}

func TestOfficialCodexCommandsUseExactArgumentVectors(t *testing.T) {
	runner := &recordingRunner{Results: map[string]CLIResult{
		"plugin list":        {Stdout: []byte(`{"installed":[]}`)},
		"plugin marketplace": {Stdout: []byte(`{"marketplaces":[]}`)},
	}}
	ctx := context.Background()
	marketplacePath := "/state/marketplace"

	if _, err := AddMarketplace(ctx, runner, marketplacePath); err != nil {
		t.Fatal(err)
	}
	if _, err := AddPlugin(ctx, runner); err != nil {
		t.Fatal(err)
	}
	if _, err := ListPlugins(ctx, runner); err != nil {
		t.Fatal(err)
	}
	if _, err := ListMarketplaces(ctx, runner); err != nil {
		t.Fatal(err)
	}
	if _, err := RemovePlugin(ctx, runner); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveMarketplace(ctx, runner); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"plugin", "marketplace", "add", marketplacePath, "--json"},
		{"plugin", "add", PluginName + "@" + MarketplaceName, "--json"},
		{"plugin", "list", "--json"},
		{"plugin", "marketplace", "list", "--json"},
		{"plugin", "remove", PluginName + "@" + MarketplaceName, "--json"},
		{"plugin", "marketplace", "remove", MarketplaceName, "--json"},
	}
	if len(runner.Commands) != len(want) {
		t.Fatalf("commands = %#v, want %#v", runner.Commands, want)
	}
	for index := range want {
		if !slices.Equal(runner.Commands[index], want[index]) {
			t.Fatalf("command %d = %#v, want %#v", index, runner.Commands[index], want[index])
		}
	}
	for _, command := range runner.Commands {
		for _, argument := range command {
			if strings.ContainsAny(argument, ";\n") {
				t.Fatalf("shell-like argument in command %#v", command)
			}
		}
	}
}

func TestOfficialCodexCommandsRejectUnsafeMarketplacePath(t *testing.T) {
	runner := &recordingRunner{}
	if _, err := AddMarketplace(context.Background(), runner, "relative/marketplace"); Code(err) != "BRIDGE_INSTALL_INPUT_INVALID" {
		t.Fatalf("error = %v", err)
	}
	if len(runner.Commands) != 0 {
		t.Fatalf("runner called with invalid path: %#v", runner.Commands)
	}
}

func TestListPluginsParsesOnlyIdentityAndStatus(t *testing.T) {
	runner := &recordingRunner{Results: map[string]CLIResult{
		"plugin list": {Stdout: []byte(`{
			"installed":[{
				"pluginId":"oaw-codex-host@oaw-local",
				"name":"oaw-codex-host",
				"marketplaceName":"oaw-local",
				"version":"1.0.0",
				"installed":true,
				"enabled":true,
				"source":{"source":"local","path":"/secret/path"},
				"futureField":"ignored"
			}],
			"available":[],
			"futureTopLevel":"ignored"
		}`)},
	}}
	records, err := ListPlugins(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("plugins = %#v", records)
	}
	record := records[0]
	if record.PluginID != PluginName+"@"+MarketplaceName || record.Name != PluginName || record.MarketplaceName != MarketplaceName || record.Version != testVersion || !record.Installed || !record.Enabled {
		t.Fatalf("plugin = %#v", record)
	}
}

func TestListMarketplacesParsesOnlyExactIdentity(t *testing.T) {
	runner := &recordingRunner{Results: map[string]CLIResult{
		"plugin marketplace": {Stdout: []byte(`{
			"marketplaces":[{
				"name":"oaw-local",
				"root":"/state/marketplace",
				"marketplaceSource":{"sourceType":"local","source":"/state/marketplace"},
				"futureField":"ignored"
			}],
			"futureTopLevel":"ignored"
		}`)},
	}}
	records, err := ListMarketplaces(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Name != MarketplaceName || records[0].Root != "/state/marketplace" || records[0].SourceType != "local" || records[0].Source != "/state/marketplace" {
		t.Fatalf("marketplaces = %#v", records)
	}
}

func TestCodexCommandFailureIsTypedAndDoesNotExposeOutput(t *testing.T) {
	runner := &recordingRunner{
		Results:  map[string]CLIResult{"plugin add": {Stderr: []byte("token=secret"), ExitCode: 9}},
		Failures: map[string]error{"plugin add": errors.New("exit status 9: token=secret")},
	}
	_, err := AddPlugin(context.Background(), runner)
	if Code(err) != "BRIDGE_INSTALL_CODEX_FAILED" {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error exposes subprocess output: %v", err)
	}
}

func TestCodexNonzeroResultWithoutErrorIsStillRejected(t *testing.T) {
	runner := &recordingRunner{Results: map[string]CLIResult{
		"plugin add": {ExitCode: 7},
	}}
	if _, err := AddPlugin(context.Background(), runner); Code(err) != "BRIDGE_INSTALL_CODEX_FAILED" {
		t.Fatalf("error = %v", err)
	}
}

func TestCodexInventoryRejectsMalformedAndInvalidRecords(t *testing.T) {
	runner := &recordingRunner{Results: map[string]CLIResult{
		"plugin list": {Stdout: []byte(`{"installed":`)},
	}}
	if _, err := ListPlugins(context.Background(), runner); Code(err) != "BRIDGE_INSTALL_CODEX_INVALID" {
		t.Fatalf("malformed Plugin inventory error = %v", err)
	}
	runner.Results["plugin list"] = CLIResult{Stdout: []byte(`{"installed":[{"pluginId":"bad\n","name":"p","marketplaceName":"m","version":"1","installed":true}]}`)}
	if _, err := ListPlugins(context.Background(), runner); Code(err) != "BRIDGE_INSTALL_CODEX_INVALID" {
		t.Fatalf("invalid Plugin inventory error = %v", err)
	}
	runner.Results["plugin marketplace"] = CLIResult{Stdout: []byte(`{"marketplaces":[{"name":"bad\n"}]}`)}
	if _, err := ListMarketplaces(context.Background(), runner); Code(err) != "BRIDGE_INSTALL_CODEX_INVALID" {
		t.Fatalf("invalid marketplace inventory error = %v", err)
	}
}

type recordingRunner struct {
	Commands         [][]string
	Results          map[string]CLIResult
	Failures         map[string]error
	FailureSequences map[string][]error
}

func (r *recordingRunner) Run(_ context.Context, args ...string) (CLIResult, error) {
	r.Commands = append(r.Commands, slices.Clone(args))
	key := commandKey(args)
	result, ok := r.Results[key]
	if !ok {
		switch strings.Join(args, " ") {
		case "plugin list --json":
			result.Stdout = []byte(`{"installed":[]}`)
		case "plugin marketplace list --json":
			result.Stdout = []byte(`{"marketplaces":[]}`)
		}
	}
	if sequence := r.FailureSequences[key]; len(sequence) != 0 {
		r.FailureSequences[key] = sequence[1:]
		return result, sequence[0]
	}
	return result, r.Failures[key]
}

func (r *recordingRunner) Saw(prefix string) bool {
	for _, command := range r.Commands {
		if strings.HasPrefix(strings.Join(command, " "), prefix) {
			return true
		}
	}
	return false
}

func commandKey(arguments []string) string {
	limit := min(2, len(arguments))
	return strings.Join(arguments[:limit], " ")
}
