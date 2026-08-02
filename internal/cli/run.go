package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

type catalogLoader func() (catalog.Catalog, error)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunWithInput(args, os.Stdin, stdout, stderr)
}

func RunWithInput(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 0 {
		switch args[0] {
		case "runtime":
			return runRuntimeExchange(args[1:], stdin, stdout, stderr)
		case "run":
			return runCodex(args[1:], stdin, stdout, stderr)
		case "check":
			return runCheck(args[1:], stdout, stderr)
		case "catalog":
			return run(args, stdout, stderr, builtin.Load)
		}
	}
	return runManagement(args, stdout, stderr)
}

type command struct {
	mode   string
	kind   string
	format string
}

func run(args []string, stdout io.Writer, stderr io.Writer, load catalogLoader) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, usage())
		return 0
	}
	parsed, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: INVALID_ARGUMENT: %s\n%s", err, usage())
		return 64
	}
	catalog, err := load()
	if err != nil {
		fmt.Fprintf(stderr, "oaw: CATALOG_INVALID: %v\n", err)
		return 65
	}
	if parsed.mode == "validate" {
		return renderValidation(catalog, parsed.format, stdout, stderr)
	}
	return renderList(catalog, parsed.kind, parsed.format, stdout, stderr)
}

func wantsHelp(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, arg := range args {
		if arg == "help" || arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func parse(args []string) (command, error) {
	if len(args) < 2 || args[0] != "catalog" {
		return command{}, fmt.Errorf("expected catalog command")
	}
	result := command{format: "text"}
	index := 1
	switch args[index] {
	case "list":
		result.mode = "list"
		index++
		if index >= len(args) || (args[index] != "providers" && args[index] != "recipes" && args[index] != "aliases") {
			return command{}, fmt.Errorf("expected providers, recipes, or aliases")
		}
		result.kind = args[index]
		index++
	case "validate":
		result.mode = "validate"
		index++
	default:
		return command{}, fmt.Errorf("unknown catalog action %q", args[index])
	}
	formatSeen := false
	for index < len(args) {
		arg := args[index]
		if strings.HasPrefix(arg, "--format=") {
			if formatSeen {
				return command{}, fmt.Errorf("format specified more than once")
			}
			formatSeen = true
			result.format = strings.TrimPrefix(arg, "--format=")
			index++
			continue
		}
		if arg == "--format" {
			if formatSeen || index+1 >= len(args) {
				return command{}, fmt.Errorf("format requires one value")
			}
			formatSeen = true
			result.format = args[index+1]
			index += 2
			continue
		}
		return command{}, fmt.Errorf("unknown argument %q", arg)
	}
	if result.format != "text" && result.format != "json" {
		return command{}, fmt.Errorf("unknown format %q", result.format)
	}
	return result, nil
}

func usage() string {
	return "usage: oaw catalog list providers|recipes|aliases [--format text|json]\n       oaw catalog validate [--format text|json]\n       oaw runtime exchange [--state-root path]\n       oaw run --host codex [--state-root path] [--project-root path]\n"
}
