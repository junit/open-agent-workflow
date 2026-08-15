package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/management"
)

type checkCommand struct {
	request management.CheckRequest
	help    bool
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseCheck(args)
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
	result, err := management.Check(management.Environment{
		Home: home, ConfigHome: configHome, StateHome: stateHome,
		Path: os.Getenv("PATH"),
	}, parsed.request)
	if writeErr := management.WriteResult(result, stdout); writeErr != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n", writeErr)
		return 1
	}
	if err != nil {
		var checkError *management.Error
		if errors.As(err, &checkError) {
			fmt.Fprintf(stderr, "oaw: error: %s\n", checkError.Message)
			return checkError.Status
		}
		fmt.Fprintf(stderr, "oaw: error: %s\n", err)
		return 1
	}
	return 0
}

func parseCheck(args []string) (checkCommand, error) {
	var result checkCommand
	targetSeen, projectSeen := false, false
	dryRunSeen, forceSeen, helpSeen := false, false, false
	for index := 0; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--target":
			if targetSeen {
				return checkCommand{}, fmt.Errorf("--target may be specified only once")
			}
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
				return checkCommand{}, fmt.Errorf("--target requires a value")
			}
			result.request.Targets = args[index+1]
			targetSeen = true
			index += 2
		case strings.HasPrefix(argument, "--target="):
			if targetSeen {
				return checkCommand{}, fmt.Errorf("--target may be specified only once")
			}
			result.request.Targets = strings.TrimPrefix(argument, "--target=")
			if result.request.Targets == "" {
				return checkCommand{}, fmt.Errorf("--target requires a value")
			}
			targetSeen = true
			index++
		case argument == "--project":
			if projectSeen {
				return checkCommand{}, fmt.Errorf("--project may be specified only once")
			}
			if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
				return checkCommand{}, fmt.Errorf("--project requires a value")
			}
			result.request.Project = args[index+1]
			projectSeen = true
			index += 2
		case strings.HasPrefix(argument, "--project="):
			if projectSeen {
				return checkCommand{}, fmt.Errorf("--project may be specified only once")
			}
			result.request.Project = strings.TrimPrefix(argument, "--project=")
			if result.request.Project == "" {
				return checkCommand{}, fmt.Errorf("--project requires a value")
			}
			projectSeen = true
			index++
		case argument == "--dry-run":
			if dryRunSeen {
				return checkCommand{}, fmt.Errorf("--dry-run may be specified only once")
			}
			dryRunSeen = true
			index++
		case argument == "--force":
			if forceSeen {
				return checkCommand{}, fmt.Errorf("--force may be specified only once")
			}
			forceSeen = true
			index++
		case argument == "-h" || argument == "--help":
			if helpSeen {
				return checkCommand{}, fmt.Errorf("--help may be specified only once")
			}
			result.help = true
			helpSeen = true
			index++
		case strings.HasPrefix(argument, "-"):
			return checkCommand{}, fmt.Errorf("unknown option '%s'", argument)
		default:
			return checkCommand{}, fmt.Errorf("unexpected argument '%s'", argument)
		}
	}
	if dryRunSeen {
		return checkCommand{}, fmt.Errorf("--dry-run is not valid for check")
	}
	if forceSeen {
		return checkCommand{}, fmt.Errorf("--force is not valid for check")
	}
	return result, nil
}

func installerUsage() string {
	return "Usage: ./install.sh <command> [options]\n\n" +
		"Commands:\n" +
		"  check       Report installation and target readiness\n" +
		"  install     Install the Policy Set and selected Host targets\n" +
		"  update      Update the Policy Set and selected Host targets\n" +
		"  uninstall   Remove the Policy Set from selected Host targets\n\n" +
		"Options:\n" +
		"  --target <ids>   Select comma-separated targets\n" +
		"  --project <path> Use project scope at an existing path\n" +
		"  --dry-run        Preview a mutating command\n" +
		"  --force          Override recoverable drift checks\n" +
		"  -h, --help       Show this help\n"
}
