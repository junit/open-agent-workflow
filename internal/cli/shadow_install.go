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

type shadowInstallCommand struct {
	targets string
	project string
	dryRun  bool
	force   bool
	help    bool
}

func RunShadowInstall(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseShadowInstall(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n", err)
		return 64
	}
	if parsed.help {
		fmt.Fprint(stdout, installerUsage())
		return 0
	}
	source, err := management.NewSource(oaw.Version(), oaw.CanonicalPolicy())
	if err != nil {
		return writeShadowInstallError(err, stderr)
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
	result, installErr := management.Install(source, management.Environment{
		Home: home, ConfigHome: configHome, StateHome: stateHome, Path: os.Getenv("PATH"),
	}, management.InstallRequest{
		Project: parsed.project, Targets: parsed.targets, DryRun: parsed.dryRun, Force: parsed.force,
	})
	if writeErr := management.WriteResult(result, stdout); writeErr != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n", writeErr)
		return 1
	}
	if installErr != nil {
		return writeShadowInstallError(installErr, stderr)
	}
	return 0
}

func parseShadowInstall(args []string) (shadowInstallCommand, error) {
	if len(args) == 0 {
		return shadowInstallCommand{}, fmt.Errorf("unknown command ''")
	}
	if args[0] != "install" {
		return shadowInstallCommand{}, fmt.Errorf("unknown command '%s'", args[0])
	}
	var result shadowInstallCommand
	targetSeen, projectSeen := false, false
	dryRunSeen, forceSeen, helpSeen := false, false, false
	for index := 1; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--target":
			if targetSeen {
				return shadowInstallCommand{}, fmt.Errorf("--target may be specified only once")
			}
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
				return shadowInstallCommand{}, fmt.Errorf("--target requires a value")
			}
			result.targets = args[index+1]
			targetSeen = true
			index += 2
		case strings.HasPrefix(argument, "--target="):
			if targetSeen {
				return shadowInstallCommand{}, fmt.Errorf("--target may be specified only once")
			}
			result.targets = strings.TrimPrefix(argument, "--target=")
			if result.targets == "" {
				return shadowInstallCommand{}, fmt.Errorf("--target requires a value")
			}
			targetSeen = true
			index++
		case argument == "--project":
			if projectSeen {
				return shadowInstallCommand{}, fmt.Errorf("--project may be specified only once")
			}
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
				return shadowInstallCommand{}, fmt.Errorf("--project requires a value")
			}
			result.project = args[index+1]
			projectSeen = true
			index += 2
		case strings.HasPrefix(argument, "--project="):
			if projectSeen {
				return shadowInstallCommand{}, fmt.Errorf("--project may be specified only once")
			}
			result.project = strings.TrimPrefix(argument, "--project=")
			if result.project == "" {
				return shadowInstallCommand{}, fmt.Errorf("--project requires a value")
			}
			projectSeen = true
			index++
		case argument == "--dry-run":
			if dryRunSeen {
				return shadowInstallCommand{}, fmt.Errorf("--dry-run may be specified only once")
			}
			result.dryRun = true
			dryRunSeen = true
			index++
		case argument == "--force":
			if forceSeen {
				return shadowInstallCommand{}, fmt.Errorf("--force may be specified only once")
			}
			result.force = true
			forceSeen = true
			index++
		case argument == "-h" || argument == "--help":
			if helpSeen {
				return shadowInstallCommand{}, fmt.Errorf("--help may be specified only once")
			}
			result.help = true
			helpSeen = true
			index++
		case strings.HasPrefix(argument, "-"):
			return shadowInstallCommand{}, fmt.Errorf("unknown option '%s'", argument)
		default:
			return shadowInstallCommand{}, fmt.Errorf("unexpected argument '%s'", argument)
		}
	}
	return result, nil
}

func writeShadowInstallError(err error, stderr io.Writer) int {
	var managementError *management.Error
	if errors.As(err, &managementError) {
		fmt.Fprintf(stderr, "oaw: error: %s\n", managementError.Message)
		return managementError.Status
	}
	fmt.Fprintf(stderr, "oaw: error: %s\n", err)
	return 1
}
