package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
)

type profileCommand struct {
	action string
	value  string
	help   bool
}

func runProfile(args []string, stdout, stderr io.Writer) int {
	command, err := parseProfileCommand(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: error: %s\n%s", err, profileUsage())
		return 64
	}
	if command.help {
		fmt.Fprint(stdout, profileUsage())
		return 0
	}
	if command.action == "check" && existingProfilePathSelector(command.value) {
		return checkProfilePath(command.value, stdout, stderr)
	}
	inventory, err := profileinspect.Discover(profileEnvironment())
	if err != nil {
		fmt.Fprintf(stderr, "oaw: error: PROFILE_INSPECTION_FAILED: %s\n", err)
		return 65
	}
	switch command.action {
	case "list":
		writeProfileList(inventory, stdout)
		return 0
	case "show":
		profile, err := profileinspect.Resolve(inventory, command.value)
		if err != nil {
			fmt.Fprintf(stderr, "oaw: error: PROFILE_SELECTION_INVALID: %s\n", err)
			return 65
		}
		writeProfileShow(profile, stdout)
		return 0
	case "check":
		return runProfileCheck(inventory, command.value, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "oaw: error: unknown profile action %q\n", command.action)
		return 64
	}
}

func parseProfileCommand(args []string) (profileCommand, error) {
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		return profileCommand{help: true}, nil
	}
	if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
		switch args[0] {
		case "list", "show", "check":
			return profileCommand{help: true}, nil
		}
	}
	if len(args) == 0 {
		return profileCommand{}, fmt.Errorf("expected list, show, or check")
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return profileCommand{}, fmt.Errorf("profile list takes no arguments")
		}
		return profileCommand{action: "list"}, nil
	case "show", "check":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" || strings.HasPrefix(args[1], "-") {
			return profileCommand{}, fmt.Errorf("profile %s requires exactly one Profile ID or path", args[0])
		}
		return profileCommand{action: args[0], value: args[1]}, nil
	default:
		return profileCommand{}, fmt.Errorf("unknown profile action %q", args[0])
	}
}

func profileEnvironment() profileinspect.Environment {
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" && home != "" {
		configHome = filepath.Join(home, ".config")
	}
	workingDirectory, _ := os.Getwd()
	return profileinspect.Environment{
		WorkingDir: workingDirectory,
		Home:       home,
		ConfigHome: configHome,
	}
}

func writeProfileList(inventory profileinspect.Inventory, output io.Writer) {
	fmt.Fprintln(output, "Profile inspection is advisory; it does not decide Profile or Skill usability.")
	fmt.Fprintln(output, "QUALIFIED ID\tNAME\tPATH")
	for _, profile := range inventory.Profiles {
		fmt.Fprintf(output, "%s:%s\t%s\t%s\n",
			profile.Source, profile.Metadata.ID, profile.Metadata.Name, profile.Path)
	}
	writeProfileDiagnostics(inventory.Diagnostics, output)
}

func writeProfileShow(profile profileinspect.Profile, output io.Writer) {
	fmt.Fprintln(output, "Profile inspection is advisory; the Markdown body remains normative.")
	fmt.Fprintf(output, "source: %s\n", profile.Source)
	fmt.Fprintf(output, "id: %s\n", profile.Metadata.ID)
	fmt.Fprintf(output, "name: %s\n", profile.Metadata.Name)
	fmt.Fprintf(output, "path: %s\n\n", profile.Path)
	_, _ = output.Write(profile.Content)
	if len(profile.Content) == 0 || profile.Content[len(profile.Content)-1] != '\n' {
		fmt.Fprintln(output)
	}
}

func runProfileCheck(
	inventory profileinspect.Inventory,
	selector string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	pathLike := profilePathSelector(selector)
	if pathLike && existingProfilePathSelector(selector) {
		return checkProfilePath(selector, stdout, stderr)
	}
	profile, err := profileinspect.Resolve(inventory, selector)
	if err != nil {
		if pathLike && !sourceQualifiedProfileSelector(selector) && !inventoryContainsProfileID(inventory, selector) {
			return checkProfilePath(selector, stdout, stderr)
		}
		fmt.Fprintf(stderr, "oaw: error: PROFILE_SELECTION_INVALID: %s\n", err)
		return 65
	}
	diagnostics := diagnosticsForProfile(inventory.Diagnostics, profile)
	writeProfileCheck(profile, diagnostics, stdout)
	if profileDiagnosticsHaveErrors(diagnostics) {
		return 65
	}
	return 0
}

func checkProfilePath(selector string, stdout, stderr io.Writer) int {
	profile, diagnostics, err := profileinspect.InspectPath(selector)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: error: PROFILE_INSPECTION_FAILED: %s\n", err)
		return 65
	}
	writeProfileCheck(profile, diagnostics, stdout)
	if profileDiagnosticsHaveErrors(diagnostics) {
		return 65
	}
	return 0
}

func writeProfileCheck(profile profileinspect.Profile, diagnostics []profileinspect.Diagnostic, output io.Writer) {
	fmt.Fprintln(output, "Profile inspection is advisory; it does not decide Profile or Skill usability.")
	if profile.Source != "" {
		fmt.Fprintf(output, "source: %s\n", profile.Source)
	}
	if profile.Metadata.ID != "" {
		fmt.Fprintf(output, "id: %s\n", profile.Metadata.ID)
		fmt.Fprintf(output, "name: %s\n", profile.Metadata.Name)
	}
	if profile.Path != "" {
		fmt.Fprintf(output, "path: %s\n", profile.Path)
	} else if len(diagnostics) != 0 && diagnostics[0].Path != "" {
		fmt.Fprintf(output, "path: %s\n", diagnostics[0].Path)
	}
	if profile.Metadata.ID != "" {
		fmt.Fprintf(output, "responsibilities: %d/%d (Policy defaults cover omitted Responsibilities)\n",
			len(profile.Metadata.Responsibilities), len(oaw.PolicyResponsibilities()))
		fmt.Fprintln(output, "Skill availability: not evaluated")
	}
	result := "metadata-valid"
	if profileDiagnosticsHaveErrors(diagnostics) {
		result = "metadata-invalid"
	}
	fmt.Fprintf(output, "result: %s\n", result)
	writeProfileDiagnostics(diagnostics, output)
}

func diagnosticsForProfile(
	diagnostics []profileinspect.Diagnostic,
	profile profileinspect.Profile,
) []profileinspect.Diagnostic {
	var result []profileinspect.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.ID != profile.Metadata.ID {
			continue
		}
		if diagnostic.Source != "" && diagnostic.Source != profile.Source && diagnostic.Code != "PROFILE_ID_CROSS_SCOPE" {
			continue
		}
		result = append(result, diagnostic)
	}
	return result
}

func writeProfileDiagnostics(diagnostics []profileinspect.Diagnostic, output io.Writer) {
	for _, diagnostic := range diagnostics {
		location := ""
		if diagnostic.Source != "" && diagnostic.ID != "" {
			location = fmt.Sprintf(" [%s:%s]", diagnostic.Source, diagnostic.ID)
		} else if diagnostic.Path != "" {
			location = fmt.Sprintf(" [%s]", diagnostic.Path)
		}
		fmt.Fprintf(output, "%s %s%s: %s\n",
			diagnostic.Severity, diagnostic.Code, location, diagnostic.Message)
	}
}

func profileDiagnosticsHaveErrors(diagnostics []profileinspect.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == profileinspect.SeverityError {
			return true
		}
	}
	return false
}

func profilePathSelector(value string) bool {
	return filepath.IsAbs(value) || filepath.Ext(value) == ".md" || strings.ContainsAny(value, `/\`)
}

func existingProfilePathSelector(value string) bool {
	if !profilePathSelector(value) {
		return false
	}
	_, err := os.Lstat(value)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

func sourceQualifiedProfileSelector(value string) bool {
	for _, prefix := range []string{"built-in:", "project:", "user:"} {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func inventoryContainsProfileID(inventory profileinspect.Inventory, id string) bool {
	for _, profile := range inventory.Profiles {
		if profile.Metadata.ID == id {
			return true
		}
	}
	return false
}

func profileUsage() string {
	return "Usage: oaw profile <command> [argument]\n\n" +
		"Commands:\n" +
		"  profile list             List built-in, project, and user Profiles\n" +
		"  profile show <id>        Show one Profile; use source:id when required\n" +
		"  profile check <id|path>  Check Profile metadata and advisory body diagnostics\n"
}
