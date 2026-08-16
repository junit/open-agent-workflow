package management

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type coordinates struct {
	configDir         string
	policyPath        string
	policyDir         string
	policyReference   string
	customProfilesDir string
	stateDir          string
	installations     string
	projects          string
	backupRoot        string
	stateFile         string
	currentScope      string
	currentProject    string
	environment       Environment
}

func initializeBaseCoordinates(environment Environment, resolved resolvedRequest) (coordinates, error) {
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
		configDir: configDir, stateDir: stateDir,
		installations: installations, projects: projects, backupRoot: backupRoot, stateFile: stateFile,
		currentScope: resolved.scope, currentProject: resolved.projectRoot,
		environment: environment,
	}, nil
}

func initializeCoordinates(environment Environment, resolved resolvedRequest) (coordinates, error) {
	coords, err := initializeBaseCoordinates(environment, resolved)
	if err != nil {
		return coordinates{}, err
	}
	switch resolved.scope {
	case "project":
		policyDir, err := validatedDestinationPath(resolved.projectRoot, ".oaw/policy")
		if err != nil {
			return coordinates{}, err
		}
		policyPath, err := validatedDestinationPath(resolved.projectRoot, ".oaw/policy/POLICY.md")
		if err != nil {
			return coordinates{}, err
		}
		customProfilesDir, err := validatedDestinationPath(resolved.projectRoot, ".oaw/profiles")
		if err != nil {
			return coordinates{}, err
		}
		coords.policyDir = policyDir
		coords.policyPath = policyPath
		coords.policyReference = ".oaw/policy/POLICY.md"
		coords.customProfilesDir = customProfilesDir
	case "user":
		policyPath, err := validatedDestinationPath(environment.ConfigHome, "open-agent-workflow/POLICY.md")
		if err != nil {
			return coordinates{}, err
		}
		customProfilesDir, err := validatedDestinationPath(environment.ConfigHome, "open-agent-workflow/profiles")
		if err != nil {
			return coordinates{}, err
		}
		coords.policyDir = coords.configDir
		coords.policyPath = policyPath
		coords.policyReference = policyPath
		coords.customProfilesDir = customProfilesDir
	default:
		return coordinates{}, integrityError("unknown Policy Set scope: " + resolved.scope)
	}
	return coords, nil
}

func policySetRelativePath(coords coordinates, sourcePath string) string {
	if coords.currentScope == "user" && strings.HasPrefix(sourcePath, "profiles/") {
		return filepath.ToSlash(filepath.Join("profiles", "builtin", strings.TrimPrefix(sourcePath, "profiles/")))
	}
	return sourcePath
}

func policySetDestination(coords coordinates, resolved resolvedRequest, sourcePath string) (string, string, string, error) {
	relative := policySetRelativePath(coords, sourcePath)
	root := resolved.projectRoot
	suffix := filepath.ToSlash(filepath.Join(".oaw", "policy", filepath.FromSlash(relative)))
	if resolved.scope == "user" {
		root = coords.environment.ConfigHome
		suffix = filepath.ToSlash(filepath.Join("open-agent-workflow", filepath.FromSlash(relative)))
	}
	destination, err := validatedDestinationPath(root, suffix)
	return destination, root, suffix, err
}

func policyRouterReference(coords coordinates) string {
	return coords.policyReference
}

func validatedDestinationPath(root, suffix string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", usageError("root must be an absolute path: " + root)
	}
	if hasControl(root) {
		return "", usageError("root contains control characters")
	}
	if suffix == "" || filepath.IsAbs(suffix) {
		return "", integrityError("destination suffix must be relative: " + suffix)
	}
	components := strings.Split(filepath.ToSlash(suffix), "/")
	candidate := root
	for index, component := range components {
		if component == "" || component == "." || component == ".." {
			return "", integrityError("destination suffix contains an unsafe component: " + suffix)
		}
		if hasControl(component) {
			return "", integrityError("destination suffix contains control characters")
		}
		candidate = candidate + string(filepath.Separator) + filepath.FromSlash(component)
		info, err := os.Lstat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", integrityError("destination path could not be inspected: " + candidate)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", integrityError("destination path contains a symlink: " + candidate)
		}
		if index != len(components)-1 && !info.IsDir() {
			return "", integrityError("destination path component is not a directory: " + candidate)
		}
	}
	return candidate, nil
}

func matchesValidatedDestination(rebuilt, destination string) bool {
	return rebuilt == destination || filepath.Clean(rebuilt) == destination
}

func artifactDestination(coords coordinates, operationScope, project, targetID, artifactID string) (string, error) {
	root, suffix, err := artifactInstallCoordinates(coords, operationScope, project, targetID, artifactID)
	if err != nil {
		return "", err
	}
	return validatedDestinationPath(root, suffix)
}

func artifactInstallCoordinates(coords coordinates, operationScope, project, targetID, artifactID string) (string, string, error) {
	candidate, found := findTarget(targetID)
	if !found {
		return "", "", integrityError("unknown target '" + targetID + "'")
	}
	artifact, found := findTargetArtifact(targetID, artifactID)
	if !found {
		return "", "", integrityError(fmt.Sprintf("unknown artifact '%s' for target '%s'", artifactID, targetID))
	}
	if !targetSupportsScope(candidate, operationScope) {
		return "", "", integrityError(fmt.Sprintf("target '%s' is not implemented for %s scope", targetID, operationScope))
	}

	switch operationScope {
	case "user":
		root := ""
		switch artifact.UserRoot {
		case userRootHome:
			root = coords.environment.Home
		case userRootConfig:
			root = coords.environment.ConfigHome
		default:
			return "", "", integrityError(fmt.Sprintf("target '%s' artifact '%s' has an invalid user root", targetID, artifactID))
		}
		return root, artifact.UserSuffix, nil
	case "project":
		return project, artifact.ProjectSuffix, nil
	default:
		return "", "", integrityError("unknown operation scope: " + operationScope)
	}
}
