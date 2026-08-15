package management

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	oaw "github.com/wifibaby4u/open-agent-workflow"
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
	version   string
	policySet []oaw.PolicyFile
}

func NewSource(version string, files []oaw.PolicyFile) (Source, error) {
	if version == "" || !safeStateField(version) {
		return Source{}, &Error{Status: 70, Message: "VERSION is invalid"}
	}
	if err := oaw.ValidatePolicySet(files); err != nil {
		return Source{}, &Error{Status: 70, Message: "canonical Policy Set source is invalid: " + err.Error()}
	}
	for _, file := range files {
		if len(file.Content) > maximumInstallArtifactBytes {
			return Source{}, &Error{Status: 70, Message: "canonical Policy Set file is too large: " + file.Path}
		}
	}
	return Source{version: version, policySet: clonePolicySet(files)}, nil
}

func clonePolicySet(files []oaw.PolicyFile) []oaw.PolicyFile {
	result := make([]oaw.PolicyFile, len(files))
	for index, file := range files {
		result[index] = oaw.PolicyFile{Path: file.Path, Content: bytes.Clone(file.Content)}
	}
	return result
}

func cloneSource(source Source) Source {
	return Source{
		version: source.version, policySet: clonePolicySet(source.policySet),
	}
}

func validateSource(source Source) (Source, error) {
	return NewSource(source.version, source.policySet)
}

type InstallRequest struct {
	Project string
	Targets string
	DryRun  bool
	Force   bool
}

type UpdateRequest struct {
	Project string
	Targets string
	DryRun  bool
	Force   bool
}

type UninstallRequest struct {
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

func Check(environment Environment, request CheckRequest) (Result, error) {
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

func integrityError(message string) error {
	return &Error{Status: 65, Message: message}
}
