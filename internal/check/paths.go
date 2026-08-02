package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type coordinates struct {
	configDir      string
	policyPath     string
	stateDir       string
	installations  string
	projects       string
	backupRoot     string
	stateFile      string
	currentScope   string
	currentProject string
	environment    Environment
}

func initializeCoordinates(environment Environment, resolved resolvedRequest) (coordinates, error) {
	for _, root := range []struct {
		label string
		value string
	}{
		{label: "HOME", value: environment.Home},
		{label: "XDG_CONFIG_HOME", value: environment.ConfigHome},
		{label: "XDG_STATE_HOME", value: environment.StateHome},
	} {
		if !filepath.IsAbs(root.value) {
			return coordinates{}, usageError(fmt.Sprintf("root must be an absolute path: %s", root.value))
		}
		if hasControl(root.value) {
			return coordinates{}, usageError(root.label + " contains control characters")
		}
	}
	configDir, err := validatedDestinationPath(environment.ConfigHome, "open-agent-workflow")
	if err != nil {
		return coordinates{}, err
	}
	policyPath, err := validatedDestinationPath(environment.ConfigHome, "open-agent-workflow/ENGINEERING.md")
	if err != nil {
		return coordinates{}, err
	}
	stateDir, err := validatedDestinationPath(environment.StateHome, "open-agent-workflow")
	if err != nil {
		return coordinates{}, err
	}
	installations, err := validatedDestinationPath(environment.StateHome, "open-agent-workflow/installations")
	if err != nil {
		return coordinates{}, err
	}
	backupRoot, err := validatedDestinationPath(environment.StateHome, "open-agent-workflow/backups")
	if err != nil {
		return coordinates{}, err
	}
	projects := installations + string(filepath.Separator) + "projects"
	stateSuffix := "open-agent-workflow/installations/user.state"
	if resolved.scope == "project" {
		stateSuffix = "open-agent-workflow/installations/projects/" + checksumBytes([]byte(resolved.projectRoot)) + ".state"
		stateSuffix = strings.Replace(stateSuffix, ":", "-", 1)
	}
	stateFile, err := validatedDestinationPath(environment.StateHome, stateSuffix)
	if err != nil {
		return coordinates{}, err
	}
	return coordinates{
		configDir: configDir, policyPath: policyPath, stateDir: stateDir,
		installations: installations, projects: projects, backupRoot: backupRoot, stateFile: stateFile,
		currentScope: resolved.scope, currentProject: resolved.projectRoot,
		environment: environment,
	}, nil
}

func validatedDestinationPath(root, suffix string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", usageError("root must be an absolute path: " + root)
	}
	if hasControl(root) {
		return "", usageError("root contains control characters")
	}
	if suffix == "" || filepath.IsAbs(suffix) {
		return "", compatibilityError("destination suffix must be relative: " + suffix)
	}
	components := strings.Split(filepath.ToSlash(suffix), "/")
	candidate := root
	for index, component := range components {
		if component == "" || component == "." || component == ".." {
			return "", compatibilityError("destination suffix contains an unsafe component: " + suffix)
		}
		if hasControl(component) {
			return "", compatibilityError("destination suffix contains control characters")
		}
		candidate = candidate + string(filepath.Separator) + filepath.FromSlash(component)
		info, err := os.Lstat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", compatibilityError("destination path could not be inspected: " + candidate)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", compatibilityError("destination path contains a symlink: " + candidate)
		}
		if index != len(components)-1 && !info.IsDir() {
			return "", compatibilityError("destination path component is not a directory: " + candidate)
		}
	}
	return candidate, nil
}

func targetDestination(coords coordinates, scope, project, id string) (string, error) {
	candidate, found := findTarget(id)
	if !found {
		return "", compatibilityError("unknown target '" + id + "'")
	}
	var root, suffix string
	switch scope {
	case "user":
		if !candidate.User {
			return "", compatibilityError(fmt.Sprintf("target '%s' is not implemented for user scope", id))
		}
		root = coords.environment.Home
		suffix = candidate.UserSuffix
		if id == "opencode" {
			root = coords.environment.ConfigHome
		}
	case "project":
		root = project
		suffix = candidate.ProjectSuffix
	default:
		return "", compatibilityError("unknown operation scope: " + scope)
	}
	return validatedDestinationPath(root, suffix)
}
