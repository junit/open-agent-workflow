// Package bridgecli implements the standalone oaw-bridge command.
package bridgecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/hook"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/install"
)

const maximumBridgeHookInputBytes = 1 << 20

type command struct {
	operation string
	host      string
	format    string
	dryRun    bool
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return RunWithContext(ctx, args, stdin, stdout, stderr)
}

func RunWithContext(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, usage())
		return 0
	}
	parsed, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw-bridge: INVALID_ARGUMENT: %v\n%s", err, usage())
		return 64
	}
	if parsed.operation == "hook" {
		return runHook(stdin, stdout, stderr)
	}
	environment, err := bridgeEnvironment()
	if err != nil {
		return writeError(err, stderr)
	}
	if parsed.operation == "serve" {
		if err := serve(ctx, environment); err != nil {
			return writeError(err, stderr)
		}
		return 0
	}

	binary := ""
	if parsed.operation == "install" || parsed.operation == "update" {
		binary, err = bridgeExecutable()
		if err != nil {
			return writeError(err, stderr)
		}
	}
	runner := install.ExecRunner{Binary: environment.CodexBinary, Dir: environment.ProjectRoot}
	result, check, err := executeManagement(ctx, parsed, environment, runner, binary, oaw.Version())
	if err != nil {
		return writeError(err, stderr)
	}
	if check != nil {
		if err := writeCheck(*check, parsed.format, stdout); err != nil {
			return writeError(err, stderr)
		}
		return 0
	}
	if err := writeResult(result, parsed.format, stdout); err != nil {
		return writeError(err, stderr)
	}
	return 0
}

func wantsHelp(args []string) bool {
	return len(args) == 0 || len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help")
}

func parse(args []string) (command, error) {
	if len(args) < 2 {
		return command{}, errors.New("expected operation and host codex")
	}
	result := command{operation: args[0], host: args[1], format: "text"}
	switch result.operation {
	case "serve", "hook", "install", "update", "check", "uninstall":
	default:
		return command{}, fmt.Errorf("unknown operation %q", result.operation)
	}
	if result.host != "codex" {
		return command{}, fmt.Errorf("unsupported host %q", result.host)
	}
	formatSeen, dryRunSeen := false, false
	for index := 2; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--dry-run":
			if dryRunSeen || result.operation != "install" && result.operation != "update" {
				return command{}, fmt.Errorf("--dry-run is not valid for %s", result.operation)
			}
			dryRunSeen, result.dryRun = true, true
			index++
		case argument == "--format" || strings.HasPrefix(argument, "--format="):
			if formatSeen || result.operation == "serve" || result.operation == "hook" {
				return command{}, errors.New("--format is not valid or was specified more than once")
			}
			formatSeen = true
			if argument == "--format" {
				if index+1 >= len(args) || args[index+1] == "" {
					return command{}, errors.New("--format requires one value")
				}
				result.format, index = args[index+1], index+2
			} else {
				result.format, index = strings.TrimPrefix(argument, "--format="), index+1
			}
		default:
			return command{}, fmt.Errorf("unknown argument %q", argument)
		}
	}
	if result.format != "text" && result.format != "json" {
		return command{}, fmt.Errorf("unknown format %q", result.format)
	}
	return result, nil
}

func executeManagement(
	ctx context.Context,
	command command,
	environment install.Environment,
	runner install.CodexRunner,
	binary string,
	version string,
) (install.Result, *install.CheckResult, error) {
	switch command.operation {
	case "install":
		result, err := install.Install(ctx, environment, runner, install.InstallRequest{Binary: binary, Version: version, DryRun: command.dryRun})
		return result, nil, err
	case "update":
		result, err := install.Update(ctx, environment, runner, install.InstallRequest{Binary: binary, Version: version, DryRun: command.dryRun})
		return result, nil, err
	case "check":
		result, err := install.Check(ctx, environment, runner)
		return install.Result{}, &result, err
	case "uninstall":
		result, err := install.Uninstall(ctx, environment, runner, install.UninstallRequest{})
		return result, nil, err
	default:
		return install.Result{}, nil, newCLIError("BRIDGE_INSTALL_INPUT_INVALID", "unsupported Bridge management operation")
	}
}

func runHook(stdin io.Reader, stdout, stderr io.Writer) int {
	raw, err := io.ReadAll(io.LimitReader(stdin, maximumBridgeHookInputBytes+1))
	if err != nil || len(raw) > maximumBridgeHookInputBytes {
		fmt.Fprintln(stderr, "oaw-bridge: HOST_BRIDGE_CONTEXT_REQUIRED: read Hook input")
		return 65
	}
	output, err := hook.ProcessPreToolUse(raw)
	if err != nil {
		fmt.Fprintln(stderr, "oaw-bridge: HOST_BRIDGE_CONTEXT_REQUIRED: process Hook input")
		return 65
	}
	if output.HookSpecificOutput == nil {
		return 0
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(stderr, "oaw-bridge: HOST_BRIDGE_OPERATION_FAILED: write Hook output")
		return 74
	}
	return 0
}

func serve(ctx context.Context, environment install.Environment) error {
	client := appserver.NewClient(appserver.ClientOptions{
		Launcher: appserver.CodexLauncher{Binary: environment.CodexBinary}, MaximumMessageBytes: 8 << 20,
		RequestTimeout: 10 * time.Second,
	})
	home, configHome, err := hostRoots()
	if err != nil {
		return err
	}
	service, err := codexbridge.NewService(codexbridge.ServiceOptions{
		Observer: client, UserConfigRoot: filepath.Join(configHome, "open-agent-workflow"),
		ProfileConfigHome: configHome, UserHome: home,
	})
	if err != nil {
		return err
	}
	serveErr := codexbridge.ServeStdio(ctx, service, oaw.Version())
	return errors.Join(serveErr, client.Close())
}

func hostRoots() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", newCLIError("HOST_BRIDGE_UNAVAILABLE", "resolve current user home")
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	if !filepath.IsAbs(configHome) {
		return "", "", newCLIError("BRIDGE_INSTALL_INPUT_INVALID", "XDG_CONFIG_HOME must be absolute")
	}
	return filepath.Clean(home), filepath.Clean(configHome), nil
}

func bridgeEnvironment() (install.Environment, error) {
	home, _, err := hostRoots()
	if err != nil {
		return install.Environment{}, err
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	if !filepath.IsAbs(stateHome) || !filepath.IsAbs(dataHome) {
		return install.Environment{}, newCLIError("BRIDGE_INSTALL_INPUT_INVALID", "Bridge state and data roots must be absolute")
	}
	projectRoot, err := os.Getwd()
	if err != nil {
		return install.Environment{}, newCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve current project root")
	}
	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return install.Environment{}, newCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve physical project root")
	}
	return install.NewEnvironment(filepath.Clean(stateHome), filepath.Clean(dataHome), "codex", filepath.Clean(projectRoot))
}

func bridgeExecutable() (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", newCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve current oaw-bridge executable")
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return "", newCLIError("BRIDGE_INSTALL_INPUT_INVALID", "resolve physical oaw-bridge executable")
	}
	return filepath.Clean(binary), nil
}

func writeResult(result install.Result, format string, output io.Writer) error {
	if format == "json" {
		return writeJSON(result, output)
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

func writeCheck(result install.CheckResult, format string, output io.Writer) error {
	if format == "json" {
		return writeJSON(result, output)
	}
	if _, err := fmt.Fprintf(output,
		"proof_scope: installation-integrity\nlive_protocol_proof: false\ncodex_marketplace: %t\ncodex_plugin: %t\ncurrent_session_loaded: %t\nrequires_new_session: %t\n",
		result.CodexMarketplace.Registered, result.CodexPlugin.Installed, result.CurrentSessionLoaded, result.RequiresNewSession,
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

func writeJSON(value any, output io.Writer) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeError(err error, stderr io.Writer) int {
	code := install.Code(err)
	if code == "" {
		code = codexbridge.Code(err)
	}
	var cliErr *cliError
	if code == "" && errors.As(err, &cliErr) {
		code = cliErr.code
	}
	if code == "" {
		code = "HOST_BRIDGE_OPERATION_FAILED"
	}
	fmt.Fprintf(stderr, "oaw-bridge: %s: %v\n", code, err)
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

type cliError struct {
	code    string
	message string
}

func (err *cliError) Error() string { return err.message }

func newCLIError(code, message string) error { return &cliError{code: code, message: message} }

func usage() string {
	return "Usage: oaw-bridge serve codex\n" +
		"       oaw-bridge hook codex\n" +
		"       oaw-bridge install codex [--dry-run] [--format text|json]\n" +
		"       oaw-bridge update codex [--dry-run] [--format text|json]\n" +
		"       oaw-bridge check codex [--format text|json]\n" +
		"       oaw-bridge uninstall codex [--format text|json]\n\n" +
		"Observes current Codex Bindings for optional Machine Assurance. It does not select or run a Policy Profile.\n"
}
