package check

import (
	"fmt"
	"io"
	"strings"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

type Environment struct {
	Home       string
	ConfigHome string
	StateHome  string
	Path       string
	TempDir    string
}

type Request struct {
	Project string
	Targets string
}

type Result struct {
	Lines []string
}

type Error struct {
	Status  int
	Message string
}

func (err *Error) Error() string { return err.Message }

func Execute(value catalog.Catalog, environment Environment, request Request) (Result, error) {
	resolved, err := resolve(environment, request)
	if err != nil {
		return Result{}, err
	}
	lines := []string{"version: " + oaw.Version()}
	if resolved.scope == "project" {
		lines = append(lines, fmt.Sprintf("scope: project (%s)", resolved.projectRoot))
	} else {
		lines = append(lines, "scope: user")
	}
	lines = append(lines, "targets: "+strings.Join(resolved.targets, ","))
	providers, err := providerLines(value, environment.Home)
	if err != nil {
		return Result{}, err
	}
	lines = append(lines, providers...)
	lines = append(lines, readinessLines(environment, resolved.targets)...)
	installed, err := installationLines(environment, resolved)
	if err != nil {
		return Result{}, err
	}
	lines = append(lines, installed...)
	return Result{Lines: lines}, nil
}

func Write(result Result, output io.Writer) error {
	for _, line := range result.Lines {
		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
	}
	return nil
}

func usageError(message string) error {
	return &Error{Status: 64, Message: message}
}
