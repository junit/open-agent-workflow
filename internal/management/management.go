package management

import (
	"bytes"
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
}

type CheckRequest struct {
	Project string
	Targets string
}

type Source struct {
	version string
	policy  []byte
}

func NewSource(version string, policy []byte) (Source, error) {
	if version == "" || !safeStateField(version) {
		return Source{}, &Error{Status: 70, Message: "VERSION is invalid"}
	}
	if len(policy) == 0 || len(policy) > maximumInstallArtifactBytes {
		return Source{}, &Error{Status: 70, Message: "canonical policy source is invalid"}
	}
	return Source{version: version, policy: bytes.Clone(policy)}, nil
}

type InstallRequest struct {
	Project string
	Targets string
	DryRun  bool
	Force   bool
}

type Result struct {
	Lines    []string
	Trailing string
}

type Error struct {
	Status  int
	Message string
}

func (err *Error) Error() string { return err.Message }

func Check(value catalog.Catalog, environment Environment, request CheckRequest) (Result, error) {
	resolved, err := resolve(request)
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
	lines = append(lines, installed.lines...)
	result := Result{Lines: lines, Trailing: installed.trailing}
	if err != nil {
		return result, err
	}
	return result, nil
}

func WriteResult(result Result, output io.Writer) error {
	for _, line := range result.Lines {
		if _, err := fmt.Fprintln(output, line); err != nil {
			return err
		}
	}
	if result.Trailing != "" {
		if _, err := fmt.Fprint(output, result.Trailing); err != nil {
			return err
		}
	}
	return nil
}

func usageError(message string) error {
	return &Error{Status: 64, Message: message}
}
