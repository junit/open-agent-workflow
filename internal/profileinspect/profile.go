// Package profileinspect provides read-only, advisory Profile discovery.
package profileinspect

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	oaw "github.com/wifibaby4u/open-agent-workflow"
)

const maximumProfileBytes = 1 << 20

var reservedBuiltInIDs = loadBuiltInIDs()

type Source string

const (
	SourceBuiltIn Source = "built-in"
	SourceProject Source = "project"
	SourceUser    Source = "user"
	SourcePath    Source = "path"
)

type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Environment struct {
	WorkingDir string
	Home       string
	ConfigHome string
}

type Profile struct {
	Source   Source
	Path     string
	Content  []byte
	Metadata oaw.PolicyProfile
	Warnings []string
}

type Diagnostic struct {
	Severity Severity
	Code     string
	Source   Source
	Path     string
	ID       string
	Message  string
}

type Inventory struct {
	ProjectRoot string
	Profiles    []Profile
	Diagnostics []Diagnostic
}

type SelectionError struct {
	Code    string
	Message string
}

func (err *SelectionError) Error() string { return err.Message }

func SelectionErrorCode(err error) string {
	var value *SelectionError
	if errors.As(err, &value) {
		return value.Code
	}
	return ""
}

func selectionError(code, format string, arguments ...any) error {
	return &SelectionError{Code: code, Message: fmt.Sprintf(format, arguments...)}
}

func (inventory Inventory) HasErrors() bool {
	for _, diagnostic := range inventory.Diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

func Discover(environment Environment) (Inventory, error) {
	projectRoot, err := findProjectRoot(environment.WorkingDir)
	if err != nil {
		return Inventory{}, err
	}
	inventory := Inventory{ProjectRoot: projectRoot}
	for _, file := range oaw.CanonicalPolicySet() {
		if !strings.HasPrefix(file.Path, "profiles/") || filepath.Ext(file.Path) != ".md" {
			continue
		}
		profile, warnings, err := oaw.InspectPolicyProfile(file.Path, file.Content)
		if err != nil {
			return Inventory{}, fmt.Errorf("embedded built-in Profile is invalid: %w", err)
		}
		inventory.Profiles = append(inventory.Profiles, Profile{
			Source: SourceBuiltIn, Path: file.Path, Content: bytes.Clone(file.Content),
			Metadata: profile, Warnings: append([]string(nil), warnings...),
		})
	}

	projectProfiles := filepath.Join(projectRoot, ".oaw", "profiles")
	profiles, diagnostics, err := discoverDirectory(SourceProject, projectRoot, projectProfiles)
	if err != nil {
		return Inventory{}, err
	}
	inventory.Profiles = append(inventory.Profiles, profiles...)
	inventory.Diagnostics = append(inventory.Diagnostics, diagnostics...)

	configHome := environment.ConfigHome
	if configHome == "" && environment.Home != "" {
		configHome = filepath.Join(environment.Home, ".config")
	}
	if configHome != "" {
		if !filepath.IsAbs(configHome) {
			return Inventory{}, fmt.Errorf("Profile config root must be an absolute path: %s", configHome)
		}
		userProfiles := filepath.Join(configHome, "open-agent-workflow", "profiles")
		profiles, diagnostics, err = discoverDirectory(SourceUser, configHome, userProfiles)
		if err != nil {
			return Inventory{}, err
		}
		inventory.Profiles = append(inventory.Profiles, profiles...)
		inventory.Diagnostics = append(inventory.Diagnostics, diagnostics...)
	}

	inventory.addIdentityDiagnostics()
	sortInventory(&inventory)
	return inventory, nil
}

func InspectPath(profilePath string) (Profile, []Diagnostic, error) {
	absolute, err := filepath.Abs(profilePath)
	if err != nil {
		return Profile{}, nil, fmt.Errorf("resolve Profile path: %w", err)
	}
	content, err := readProfileFile(absolute)
	if err != nil {
		return Profile{}, nil, err
	}
	parsed, warnings, err := oaw.InspectPolicyProfile(absolute, content)
	if err != nil {
		return Profile{}, []Diagnostic{{
			Severity: SeverityError, Code: "PROFILE_METADATA_INVALID", Source: SourcePath,
			Path: absolute, Message: err.Error(),
		}}, nil
	}
	profile := Profile{
		Source: SourcePath, Path: absolute, Content: content,
		Metadata: parsed, Warnings: append([]string(nil), warnings...),
	}
	return profile, warningDiagnostics(profile), nil
}

func Resolve(inventory Inventory, selector string) (Profile, error) {
	source, id, qualified, err := parseSelector(selector)
	if err != nil {
		return Profile{}, err
	}
	if !qualified {
		for _, profile := range inventory.Profiles {
			if profile.Source == SourceBuiltIn && profile.Metadata.ID == id {
				return cloneProfile(profile), nil
			}
		}
	}
	var matches []Profile
	for _, profile := range inventory.Profiles {
		if profile.Metadata.ID != id || qualified && profile.Source != source {
			continue
		}
		matches = append(matches, profile)
	}
	if len(matches) == 0 {
		return Profile{}, selectionError("PROFILE_NOT_FOUND", "Profile %q was not found", selector)
	}
	if len(matches) != 1 {
		if qualified {
			return Profile{}, selectionError("PROFILE_AMBIGUOUS", "Profile %q is ambiguous within its source", selector)
		}
		return Profile{}, selectionError("PROFILE_AMBIGUOUS", "Profile %q is ambiguous; use a source qualifier", id)
	}
	if matches[0].Source != SourceBuiltIn && builtInID(id) {
		return Profile{}, selectionError("PROFILE_SELECTION_INVALID", "Custom Profile ID %q is reserved by a built-in Profile", id)
	}
	return cloneProfile(matches[0]), nil
}

func ResolveQualified(inventory Inventory, selector string) (Profile, error) {
	_, _, qualified, err := parseSelector(selector)
	if err != nil {
		return Profile{}, err
	}
	if !qualified {
		return Profile{}, selectionError("PROFILE_SELECTION_INVALID", "Profile selector %q requires a source qualifier", strings.TrimSpace(selector))
	}
	return Resolve(inventory, selector)
}

func discoverDirectory(source Source, root, directory string) ([]Profile, []Diagnostic, error) {
	exists, err := validateProfileDirectory(root, directory)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s Profile directory %q: %w", source, directory, err)
	}
	if !exists {
		return nil, nil, nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s Profile directory %q: %w", source, directory, err)
	}
	var profiles []Profile
	var diagnostics []Diagnostic
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		profilePath := filepath.Join(directory, entry.Name())
		content, err := readProfileFile(profilePath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityError, Code: "PROFILE_FILE_INVALID", Source: source,
				Path: profilePath, Message: err.Error(),
			})
			continue
		}
		parsed, warnings, err := oaw.InspectPolicyProfile(profilePath, content)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityError, Code: "PROFILE_METADATA_INVALID", Source: source,
				Path: profilePath, Message: err.Error(),
			})
			continue
		}
		profile := Profile{
			Source: source, Path: profilePath, Content: content,
			Metadata: parsed, Warnings: append([]string(nil), warnings...),
		}
		profiles = append(profiles, profile)
		diagnostics = append(diagnostics, warningDiagnostics(profile)...)
	}
	return profiles, diagnostics, nil
}

func readProfileFile(profilePath string) ([]byte, error) {
	before, err := os.Lstat(profilePath)
	if err != nil {
		return nil, fmt.Errorf("inspect Profile %q: %w", profilePath, err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("Profile %q is not a regular file", profilePath)
	}
	if before.Size() > maximumProfileBytes {
		return nil, fmt.Errorf("Profile %q exceeds the read limit", profilePath)
	}
	file, err := os.Open(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read Profile %q: %w", profilePath, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect Profile %q: %w", profilePath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("Profile %q is not a regular file", profilePath)
	}
	if !os.SameFile(before, info) {
		return nil, fmt.Errorf("Profile %q changed while it was inspected", profilePath)
	}
	if info.Size() > maximumProfileBytes {
		return nil, fmt.Errorf("Profile %q exceeds the read limit", profilePath)
	}
	content, err := io.ReadAll(io.LimitReader(file, maximumProfileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Profile %q: %w", profilePath, err)
	}
	if len(content) > maximumProfileBytes {
		return nil, fmt.Errorf("Profile %q exceeds the read limit", profilePath)
	}
	return content, nil
}

func (inventory *Inventory) addIdentityDiagnostics() {
	byScope := make(map[string][]Profile)
	byID := make(map[string]map[Source]bool)
	for _, profile := range inventory.Profiles {
		key := string(profile.Source) + "\x00" + profile.Metadata.ID
		byScope[key] = append(byScope[key], profile)
		if byID[profile.Metadata.ID] == nil {
			byID[profile.Metadata.ID] = make(map[Source]bool)
		}
		byID[profile.Metadata.ID][profile.Source] = true
		if profile.Source != SourceBuiltIn && builtInID(profile.Metadata.ID) {
			inventory.Diagnostics = append(inventory.Diagnostics, Diagnostic{
				Severity: SeverityError, Code: "PROFILE_ID_RESERVED", Source: profile.Source,
				Path: profile.Path, ID: profile.Metadata.ID,
				Message: fmt.Sprintf("Custom Profile ID %q is reserved by a built-in Profile", profile.Metadata.ID),
			})
		}
	}
	for _, profiles := range byScope {
		if len(profiles) < 2 {
			continue
		}
		inventory.Diagnostics = append(inventory.Diagnostics, Diagnostic{
			Severity: SeverityError, Code: "PROFILE_ID_DUPLICATE", Source: profiles[0].Source,
			ID: profiles[0].Metadata.ID,
			Message: fmt.Sprintf("Profile ID %q appears more than once in the %s scope: %s and %s",
				profiles[0].Metadata.ID, profiles[0].Source, profiles[0].Path, profiles[1].Path),
		})
	}
	for id, sources := range byID {
		if !sources[SourceProject] || !sources[SourceUser] || sources[SourceBuiltIn] {
			continue
		}
		inventory.Diagnostics = append(inventory.Diagnostics, Diagnostic{
			Severity: SeverityWarning, Code: "PROFILE_ID_CROSS_SCOPE", ID: id,
			Message: fmt.Sprintf("Profile ID %q exists in project and user scopes; use project:%s or user:%s", id, id, id),
		})
	}
}

func warningDiagnostics(profile Profile) []Diagnostic {
	result := make([]Diagnostic, 0, len(profile.Warnings))
	for _, warning := range profile.Warnings {
		result = append(result, Diagnostic{
			Severity: SeverityWarning, Code: "PROFILE_BODY_WARNING", Source: profile.Source,
			Path: profile.Path, ID: profile.Metadata.ID, Message: warning,
		})
	}
	return result
}

func parseSelector(selector string) (Source, string, bool, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", "", false, selectionError("PROFILE_SELECTION_INVALID", "Profile selector is empty")
	}
	for _, source := range []Source{SourceBuiltIn, SourceProject, SourceUser} {
		prefix := string(source) + ":"
		if strings.HasPrefix(selector, prefix) {
			id := strings.TrimSpace(strings.TrimPrefix(selector, prefix))
			if id == "" {
				return "", "", false, selectionError("PROFILE_SELECTION_INVALID", "Profile selector %q has no ID", selector)
			}
			return source, id, true, nil
		}
	}
	return "", selector, false, nil
}

func builtInID(id string) bool {
	return reservedBuiltInIDs[id]
}

func loadBuiltInIDs() map[string]bool {
	result := make(map[string]bool)
	for _, file := range oaw.CanonicalPolicySet() {
		if !strings.HasPrefix(file.Path, "profiles/") || filepath.Ext(file.Path) != ".md" {
			continue
		}
		profile, err := oaw.ParsePolicyProfile(file.Path, file.Content)
		if err != nil {
			panic(fmt.Sprintf("load built-in Profile identity: %v", err))
		}
		result[profile.ID] = true
	}
	return result
}

func findProjectRoot(workingDirectory string) (string, error) {
	root := workingDirectory
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current project: %w", err)
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve current project: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve current project: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("current project directory is unavailable: %s", root)
	}
	for candidate := root; ; candidate = filepath.Dir(candidate) {
		for _, marker := range []string{".oaw", ".git"} {
			if _, err := os.Lstat(filepath.Join(candidate, marker)); err == nil {
				return candidate, nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("inspect current project root: %w", err)
			}
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return root, nil
		}
	}
}

func sortInventory(inventory *Inventory) {
	order := map[Source]int{SourceBuiltIn: 0, SourceProject: 1, SourceUser: 2}
	sort.Slice(inventory.Profiles, func(left, right int) bool {
		first, second := inventory.Profiles[left], inventory.Profiles[right]
		if order[first.Source] != order[second.Source] {
			return order[first.Source] < order[second.Source]
		}
		if first.Metadata.ID != second.Metadata.ID {
			return first.Metadata.ID < second.Metadata.ID
		}
		return first.Path < second.Path
	})
	sort.Slice(inventory.Diagnostics, func(left, right int) bool {
		first, second := inventory.Diagnostics[left], inventory.Diagnostics[right]
		if first.Severity != second.Severity {
			return first.Severity < second.Severity
		}
		if first.Code != second.Code {
			return first.Code < second.Code
		}
		if first.ID != second.ID {
			return first.ID < second.ID
		}
		return first.Path < second.Path
	})
}

func cloneProfile(profile Profile) Profile {
	profile.Content = bytes.Clone(profile.Content)
	profile.Warnings = append([]string(nil), profile.Warnings...)
	responsibilities := make(map[oaw.Responsibility]string, len(profile.Metadata.Responsibilities))
	for responsibility, action := range profile.Metadata.Responsibilities {
		responsibilities[responsibility] = action
	}
	profile.Metadata.Responsibilities = responsibilities
	profile.Metadata.Occurrences = append([]oaw.ProfileOccurrence(nil), profile.Metadata.Occurrences...)
	return profile
}

func validateProfileDirectory(root, directory string) (bool, error) {
	root = filepath.Clean(root)
	directory = filepath.Clean(directory)
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, errors.New("Profile directory escapes its source root")
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("Profile directory contains a symlink: %s", current)
		}
		if !info.IsDir() {
			return false, fmt.Errorf("Profile directory component is not a directory: %s", current)
		}
	}
	return true, nil
}
