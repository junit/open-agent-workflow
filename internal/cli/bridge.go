package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/hook"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/install"
)

const maximumBridgeHookInputBytes = 1 << 20

type bridgeCommand struct {
	Operation string
	Host      string
	Format    string
	DryRun    bool
}

func runBridge(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, bridgeUsage())
		return 0
	}
	parsed, err := parseBridge(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: INVALID_ARGUMENT: %v\n%s", err, bridgeUsage())
		return 64
	}
	if parsed.Operation == "hook" {
		return runBridgeHook(stdin, stdout, stderr)
	}
	environment, err := bridgeEnvironment()
	if err != nil {
		return writeBridgeError(err, stderr)
	}
	if parsed.Operation == "serve" {
		if err := runCodexBridgeServe(ctx, environment); err != nil {
			return writeBridgeError(err, stderr)
		}
		return 0
	}

	binary := ""
	if parsed.Operation == "install" || parsed.Operation == "update" {
		binary, err = bridgeExecutable()
		if err != nil {
			return writeBridgeError(err, stderr)
		}
	}
	runner := install.ExecRunner{Binary: environment.CodexBinary, Dir: environment.ProjectRoot}
	result, check, err := executeBridgeManagement(ctx, parsed, environment, runner, binary, oaw.Version())
	if err != nil {
		return writeBridgeError(err, stderr)
	}
	if check != nil {
		if err := writeBridgeCheck(*check, parsed.Format, stdout); err != nil {
			return writeBridgeError(err, stderr)
		}
		return 0
	}
	if err := writeBridgeResult(result, parsed.Format, stdout); err != nil {
		return writeBridgeError(err, stderr)
	}
	return 0
}

func parseBridge(args []string) (bridgeCommand, error) {
	if len(args) < 2 {
		return bridgeCommand{}, errors.New("expected bridge operation and host codex")
	}
	result := bridgeCommand{Operation: args[0], Host: args[1], Format: "text"}
	switch result.Operation {
	case "serve", "hook", "install", "update", "check", "uninstall":
	default:
		return bridgeCommand{}, fmt.Errorf("unknown bridge operation %q", result.Operation)
	}
	if result.Host != "codex" {
		return bridgeCommand{}, fmt.Errorf("unsupported bridge host %q", result.Host)
	}
	formatSeen := false
	dryRunSeen := false
	for index := 2; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--dry-run":
			if dryRunSeen || result.Operation != "install" && result.Operation != "update" {
				return bridgeCommand{}, fmt.Errorf("--dry-run is not valid for bridge %s", result.Operation)
			}
			dryRunSeen = true
			result.DryRun = true
			index++
		case argument == "--format" || strings.HasPrefix(argument, "--format="):
			if formatSeen || result.Operation == "serve" || result.Operation == "hook" {
				return bridgeCommand{}, fmt.Errorf("--format is not valid or was specified more than once")
			}
			formatSeen = true
			if argument == "--format" {
				if index+1 >= len(args) || args[index+1] == "" {
					return bridgeCommand{}, errors.New("--format requires one value")
				}
				result.Format = args[index+1]
				index += 2
			} else {
				result.Format = strings.TrimPrefix(argument, "--format=")
				index++
			}
		default:
			return bridgeCommand{}, fmt.Errorf("unknown bridge argument %q", argument)
		}
	}
	if result.Format != "text" && result.Format != "json" {
		return bridgeCommand{}, fmt.Errorf("unknown bridge format %q", result.Format)
	}
	return result, nil
}

func executeBridgeManagement(
	ctx context.Context,
	command bridgeCommand,
	environment install.Environment,
	runner install.CodexRunner,
	binary string,
	version string,
) (install.Result, *install.CheckResult, error) {
	switch command.Operation {
	case "install":
		result, err := install.Install(ctx, environment, runner, install.InstallRequest{Binary: binary, Version: version, DryRun: command.DryRun})
		return result, nil, err
	case "update":
		result, err := install.Update(ctx, environment, runner, install.InstallRequest{Binary: binary, Version: version, DryRun: command.DryRun})
		return result, nil, err
	case "check":
		result, err := install.Check(ctx, environment, runner)
		return install.Result{}, &result, err
	case "uninstall":
		result, err := install.Uninstall(ctx, environment, runner, install.UninstallRequest{})
		return result, nil, err
	default:
		return install.Result{}, nil, newBridgeCLIError("BRIDGE_INSTALL_INPUT_INVALID", "unsupported Bridge management operation")
	}
}

func runBridgeHook(stdin io.Reader, stdout, stderr io.Writer) int {
	raw, err := io.ReadAll(io.LimitReader(stdin, maximumBridgeHookInputBytes+1))
	if err != nil {
		fmt.Fprintf(stderr, "oaw: HOST_BRIDGE_CONTEXT_REQUIRED: read Hook input\n")
		return 65
	}
	output, err := hook.ProcessPreToolUse(raw)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: HOST_BRIDGE_CONTEXT_REQUIRED: process Hook input\n")
		return 65
	}
	if output.HookSpecificOutput == nil {
		return 0
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(stderr, "oaw: HOST_BRIDGE_OPERATION_FAILED: write Hook output\n")
		return 74
	}
	return 0
}

func runCodexBridgeServe(ctx context.Context, environment install.Environment) error {
	client := appserver.NewClient(appserver.ClientOptions{
		Launcher:            appserver.CodexLauncher{Binary: environment.CodexBinary},
		MaximumMessageBytes: 8 << 20,
		RequestTimeout:      10 * time.Second,
	})
	store := codexbridge.NewEvidenceStore(codexbridge.CacheOptions{TTL: 10 * time.Minute, MaximumEntries: 64})
	home, err := os.UserHomeDir()
	if err != nil {
		return newBridgeCLIError("HOST_BRIDGE_UNAVAILABLE", "resolve current user home")
	}
	service, err := codexbridge.NewService(codexbridge.ServiceOptions{
		Observer: client, Store: store, StateRoot: defaultWorkflowStateRoot(),
		ProjectRoot: environment.ProjectRoot, UserConfigRoot: defaultConfigRoot(), UserHome: home,
		BridgeVersion: oaw.Version(), Rules: classification.ClassificationRules{}, Authority: workflowAuthorityCeiling(),
	})
	if err != nil {
		return err
	}
	serveErr := codexbridge.ServeStdio(ctx, service, oaw.Version())
	closeErr := client.Close()
	return errors.Join(serveErr, closeErr)
}

func bridgeEnvironment() (install.Environment, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return install.Environment{}, newBridgeCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve current user home")
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	stateHome = filepath.Clean(stateHome)
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	dataHome = filepath.Clean(dataHome)
	projectRoot, err := os.Getwd()
	if err != nil {
		return install.Environment{}, newBridgeCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve current project root")
	}
	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return install.Environment{}, newBridgeCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve physical project root")
	}
	return install.NewEnvironment(stateHome, dataHome, "codex", filepath.Clean(projectRoot))
}

func bridgeExecutable() (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", newBridgeCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve current OAW executable")
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return "", newBridgeCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve physical OAW executable")
	}
	return filepath.Clean(binary), nil
}

func writeBridgeResult(result install.Result, format string, output io.Writer) error {
	if format == "json" {
		return writeBridgeJSON(result, output)
	}
	if _, err := fmt.Fprintf(output, "operation: %s\nchanged: %t\nrequires_new_session: %t\n", result.Operation, result.Changed, result.RequiresNewSession); err != nil {
		return err
	}
	for _, diagnostic := range result.Diagnostics {
		if _, err := fmt.Fprintf(output, "diagnostic: %s path=%s %s\n", diagnostic.Code, diagnostic.PathDigest, diagnostic.Message); err != nil {
			return err
		}
	}
	return nil
}

func writeBridgeCheck(result install.CheckResult, format string, output io.Writer) error {
	if format == "json" {
		return writeBridgeJSON(result, output)
	}
	if _, err := fmt.Fprintf(output,
		"proof_scope: installation-integrity\nlive_protocol_proof: false\ncodex_marketplace: %t\ncodex_plugin: %t\ncurrent_session_loaded: %t\nrequires_new_session: %t\n",
		result.CodexMarketplace.Registered, result.CodexPlugin.Installed,
		result.CurrentSessionLoaded, result.RequiresNewSession,
	); err != nil {
		return err
	}
	for _, file := range result.Files {
		if _, err := fmt.Fprintf(output, "file: %s %s\n", file.PathDigest, file.Status); err != nil {
			return err
		}
	}
	for _, diagnostic := range result.Diagnostics {
		if _, err := fmt.Fprintf(output, "diagnostic: %s path=%s %s\n", diagnostic.Code, diagnostic.PathDigest, diagnostic.Message); err != nil {
			return err
		}
	}
	return nil
}

func writeBridgeJSON(value any, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeBridgeError(err error, stderr io.Writer) int {
	code := install.Code(err)
	if code == "" {
		code = codexbridge.Code(err)
	}
	var cliErr *bridgeCLIError
	if code == "" && errors.As(err, &cliErr) {
		code = cliErr.code
	}
	if code == "" {
		code = "HOST_BRIDGE_OPERATION_FAILED"
	}
	fmt.Fprintf(stderr, "oaw: %s: %v\n", code, err)
	switch code {
	case "BRIDGE_INSTALL_INPUT_INVALID":
		return 64
	case "BRIDGE_INSTALL_CODEX_FAILED", "BRIDGE_INSTALL_UNINSTALL_INCOMPLETE", "BRIDGE_INSTALL_ROLLBACK_INCOMPLETE", "HOST_BRIDGE_UNAVAILABLE":
		return 69
	case "BRIDGE_INSTALL_IO":
		return 74
	default:
		return 65
	}
}

type bridgeCLIError struct {
	code    string
	message string
}

func (err *bridgeCLIError) Error() string {
	return err.message
}

func newBridgeCLIError(code, message string) error {
	return &bridgeCLIError{code: code, message: message}
}

func bridgeUsage() string {
	return "usage: oaw bridge serve codex\n" +
		"       oaw bridge hook codex\n" +
		"       oaw bridge install codex [--dry-run] [--format text|json]\n" +
		"       oaw bridge update codex [--dry-run] [--format text|json]\n" +
		"       oaw bridge check codex [--format text|json]\n" +
		"       oaw bridge uninstall codex [--format text|json]\n"
}
