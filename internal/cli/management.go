package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/management"
)

type managementCommand struct {
	operation string
	targets   string
	project   string
	dryRun    bool
	force     bool
	help      bool
}

func runManagement(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help")) {
		fmt.Fprint(stdout, installerUsage())
		return 0
	}
	parsed, err := parseManagement(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n", err)
		return 64
	}
	if parsed.help {
		fmt.Fprint(stdout, installerUsage())
		return 0
	}
	result, managementErr := executeManagement(parsed, managementEnvironment())
	if writeErr := management.WriteResult(result, stdout); writeErr != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n", writeErr)
		return 1
	}
	if managementErr != nil {
		return writeManagementError(managementErr, stderr)
	}
	return 0
}

func managementEnvironment() management.Environment {
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = home + "/.config"
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = home + "/.local/state"
	}
	return management.Environment{
		Home: home, ConfigHome: configHome, StateHome: stateHome, Path: os.Getenv("PATH"),
	}
}

func executeManagement(
	parsed managementCommand,
	environment management.Environment,
) (management.Result, error) {
	var result management.Result
	var managementErr error
	switch parsed.operation {
	case "install", "update":
		source, err := management.NewSource(oaw.Version(), oaw.CanonicalPolicy())
		if err != nil {
			return management.Result{}, err
		}
		if parsed.operation == "install" {
			result, managementErr = management.Install(source, environment, management.InstallRequest{
				Project: parsed.project, Targets: parsed.targets, DryRun: parsed.dryRun, Force: parsed.force,
			})
		} else {
			result, managementErr = management.Update(source, environment, management.UpdateRequest{
				Project: parsed.project, Targets: parsed.targets, DryRun: parsed.dryRun, Force: parsed.force,
			})
		}
	case "uninstall":
		result, managementErr = management.Uninstall(environment, management.UninstallRequest{
			Project: parsed.project, Targets: parsed.targets, DryRun: parsed.dryRun, Force: parsed.force,
		})
	}
	return result, managementErr
}

func parseManagement(args []string) (managementCommand, error) {
	if len(args) == 0 {
		return managementCommand{}, fmt.Errorf("unknown command ''")
	}
	if args[0] != "install" && args[0] != "update" && args[0] != "uninstall" {
		return managementCommand{}, fmt.Errorf("unknown command '%s'", args[0])
	}
	result := managementCommand{operation: args[0]}
	targetSeen, projectSeen := false, false
	dryRunSeen, forceSeen, helpSeen := false, false, false
	for index := 1; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--target" || strings.HasPrefix(argument, "--target="):
			value, next, err := parseManagementValueOption(args, index, "--target", &targetSeen)
			if err != nil {
				return managementCommand{}, err
			}
			result.targets, index = value, next
		case argument == "--project" || strings.HasPrefix(argument, "--project="):
			value, next, err := parseManagementValueOption(args, index, "--project", &projectSeen)
			if err != nil {
				return managementCommand{}, err
			}
			result.project, index = value, next
		case argument == "--dry-run":
			if err := setManagementFlag("--dry-run", &dryRunSeen, &result.dryRun); err != nil {
				return managementCommand{}, err
			}
			index++
		case argument == "--force":
			if err := setManagementFlag("--force", &forceSeen, &result.force); err != nil {
				return managementCommand{}, err
			}
			index++
		case argument == "-h" || argument == "--help":
			if err := setManagementFlag("--help", &helpSeen, &result.help); err != nil {
				return managementCommand{}, err
			}
			index++
		case strings.HasPrefix(argument, "-"):
			return managementCommand{}, fmt.Errorf("unknown option '%s'", argument)
		default:
			return managementCommand{}, fmt.Errorf("unexpected argument '%s'", argument)
		}
	}
	return result, nil
}

func parseManagementValueOption(
	args []string,
	index int,
	name string,
	seen *bool,
) (string, int, error) {
	if *seen {
		return "", index, fmt.Errorf("%s may be specified only once", name)
	}
	argument := args[index]
	if argument == name {
		if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
			return "", index, fmt.Errorf("%s requires a value", name)
		}
		*seen = true
		return args[index+1], index + 2, nil
	}
	value := strings.TrimPrefix(argument, name+"=")
	if value == "" {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	*seen = true
	return value, index + 1, nil
}

func setManagementFlag(name string, seen *bool, value *bool) error {
	if *seen {
		return fmt.Errorf("%s may be specified only once", name)
	}
	*seen = true
	*value = true
	return nil
}

func writeManagementError(err error, stderr io.Writer) int {
	var managementError *management.Error
	if errors.As(err, &managementError) {
		fmt.Fprintf(stderr, "oaw: error: %s\n", managementError.Message)
		return managementError.Status
	}
	fmt.Fprintf(stderr, "oaw: error: %s\n", err)
	return 1
}
