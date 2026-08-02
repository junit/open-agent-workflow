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

type shadowManagementCommand struct {
	operation string
	targets   string
	project   string
	dryRun    bool
	force     bool
	help      bool
}

func RunShadowManagement(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseShadowManagement(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n", err)
		return 64
	}
	if parsed.help {
		fmt.Fprint(stdout, installerUsage())
		return 0
	}
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = home + "/.config"
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = home + "/.local/state"
	}
	environment := management.Environment{
		Home: home, ConfigHome: configHome, StateHome: stateHome, Path: os.Getenv("PATH"),
	}
	var result management.Result
	var managementErr error
	switch parsed.operation {
	case "install", "update":
		source, err := management.NewSource(oaw.Version(), oaw.CanonicalPolicy())
		if err != nil {
			return writeShadowManagementError(err, stderr)
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
	if writeErr := management.WriteResult(result, stdout); writeErr != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n", writeErr)
		return 1
	}
	if managementErr != nil {
		return writeShadowManagementError(managementErr, stderr)
	}
	return 0
}

func parseShadowManagement(args []string) (shadowManagementCommand, error) {
	if len(args) == 0 {
		return shadowManagementCommand{}, fmt.Errorf("unknown command ''")
	}
	if args[0] != "install" && args[0] != "update" && args[0] != "uninstall" {
		return shadowManagementCommand{}, fmt.Errorf("unknown command '%s'", args[0])
	}
	result := shadowManagementCommand{operation: args[0]}
	targetSeen, projectSeen := false, false
	dryRunSeen, forceSeen, helpSeen := false, false, false
	for index := 1; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--target":
			if targetSeen {
				return shadowManagementCommand{}, fmt.Errorf("--target may be specified only once")
			}
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
				return shadowManagementCommand{}, fmt.Errorf("--target requires a value")
			}
			result.targets = args[index+1]
			targetSeen = true
			index += 2
		case strings.HasPrefix(argument, "--target="):
			if targetSeen {
				return shadowManagementCommand{}, fmt.Errorf("--target may be specified only once")
			}
			result.targets = strings.TrimPrefix(argument, "--target=")
			if result.targets == "" {
				return shadowManagementCommand{}, fmt.Errorf("--target requires a value")
			}
			targetSeen = true
			index++
		case argument == "--project":
			if projectSeen {
				return shadowManagementCommand{}, fmt.Errorf("--project may be specified only once")
			}
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
				return shadowManagementCommand{}, fmt.Errorf("--project requires a value")
			}
			result.project = args[index+1]
			projectSeen = true
			index += 2
		case strings.HasPrefix(argument, "--project="):
			if projectSeen {
				return shadowManagementCommand{}, fmt.Errorf("--project may be specified only once")
			}
			result.project = strings.TrimPrefix(argument, "--project=")
			if result.project == "" {
				return shadowManagementCommand{}, fmt.Errorf("--project requires a value")
			}
			projectSeen = true
			index++
		case argument == "--dry-run":
			if dryRunSeen {
				return shadowManagementCommand{}, fmt.Errorf("--dry-run may be specified only once")
			}
			result.dryRun = true
			dryRunSeen = true
			index++
		case argument == "--force":
			if forceSeen {
				return shadowManagementCommand{}, fmt.Errorf("--force may be specified only once")
			}
			result.force = true
			forceSeen = true
			index++
		case argument == "-h" || argument == "--help":
			if helpSeen {
				return shadowManagementCommand{}, fmt.Errorf("--help may be specified only once")
			}
			result.help = true
			helpSeen = true
			index++
		case strings.HasPrefix(argument, "-"):
			return shadowManagementCommand{}, fmt.Errorf("unknown option '%s'", argument)
		default:
			return shadowManagementCommand{}, fmt.Errorf("unexpected argument '%s'", argument)
		}
	}
	return result, nil
}

func writeShadowManagementError(err error, stderr io.Writer) int {
	var managementError *management.Error
	if errors.As(err, &managementError) {
		fmt.Fprintf(stderr, "oaw: error: %s\n", managementError.Message)
		return managementError.Status
	}
	fmt.Fprintf(stderr, "oaw: error: %s\n", err)
	return 1
}
