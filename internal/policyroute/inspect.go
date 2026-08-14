// Package policyroute observes cooperative Host routes for the Policy
// Workflow Module. It reports local callability only, without installation
// source or content-identity claims.
package policyroute

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

const maximumCodexConfigBytes = 4 << 20

type Options struct {
	HostID string
	Home   string
}

// Inspect returns the current Host's routes needed by the built-in Policy
// Profiles. The result is sorted by route name and contains no duplicates.
func Inspect(options Options) (policyflow.RouteInventory, error) {
	if options.HostID != "codex" {
		return nil, fmt.Errorf("POLICY_ROUTE_HOST_UNSUPPORTED: policy route inspection currently supports codex")
	}
	if err := validateHome(options.Home); err != nil {
		return nil, err
	}

	requirements, err := requiredRoutes()
	if err != nil {
		return nil, fmt.Errorf("POLICY_ROUTE_SEMANTICS_INVALID: %w", err)
	}
	plugins, err := readCodexPlugins(options.Home)
	if err != nil {
		return nil, err
	}

	roots := routeRoots{
		matt: filepath.Join(options.Home, ".agents", "skills"),
	}
	if plugins.superpowers {
		roots.superpowers, err = superpowersRoots(options.Home)
		if err != nil {
			return nil, err
		}
	}
	if plugins.ecc {
		roots.ecc, err = eccRoots(options.Home)
		if err != nil {
			return nil, err
		}
	}

	observed := make(map[string]policyflow.Route, len(requirements))
	for _, requirement := range requirements {
		route, found := observeRoute(requirement, roots)
		if found {
			observed[route.Name] = route
		}
	}

	names := make([]string, 0, len(observed))
	for name := range observed {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make(policyflow.RouteInventory, 0, len(names))
	for _, name := range names {
		result = append(result, observed[name])
	}
	return result, nil
}

type routeRequirement struct {
	name string
	kind policyflow.RouteKind
}

func requiredRoutes() ([]routeRequirement, error) {
	offer, err := policyflow.New().Offer(nil)
	if err != nil {
		return nil, err
	}
	byName := map[string]routeRequirement{}
	for _, profile := range offer.Profiles {
		for _, route := range profile.Routes {
			if route.Kind == policyflow.RouteUserGate || route.Kind == policyflow.RouteHostGate {
				continue
			}
			existing, found := byName[route.Name]
			if found && existing.kind != route.Kind {
				return nil, fmt.Errorf("route %q has conflicting kinds", route.Name)
			}
			byName[route.Name] = routeRequirement{name: route.Name, kind: route.Kind}
		}
		for _, incident := range profile.IncidentRoutes {
			if incident.Skill == "" {
				continue
			}
			existing, found := byName[incident.Skill]
			if found && existing.kind != policyflow.RouteSkill {
				return nil, fmt.Errorf("route %q has conflicting kinds", incident.Skill)
			}
			byName[incident.Skill] = routeRequirement{name: incident.Skill, kind: policyflow.RouteSkill}
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]routeRequirement, 0, len(names))
	for _, name := range names {
		result = append(result, byName[name])
	}
	return result, nil
}

type routeRoots struct {
	matt        string
	superpowers string
	ecc         string
}

func observeRoute(requirement routeRequirement, roots routeRoots) (policyflow.Route, bool) {
	if requirement.kind == policyflow.RouteHostAction {
		return policyflow.Route{Name: requirement.name, Mode: policyflow.HostControlled}, true
	}

	switch {
	case strings.HasPrefix(requirement.name, "superpowers:"):
		name := strings.TrimPrefix(requirement.name, "superpowers:")
		if skillVisible(roots.superpowers, name, false) {
			// Static Policy inspection can prove an enabled installation route,
			// not that this already-running Host session loaded it.
			return policyflow.Route{Name: requirement.name, Mode: policyflow.UserExplicit}, true
		}
	case strings.HasPrefix(requirement.name, "ecc:"):
		name := strings.TrimPrefix(requirement.name, "ecc:")
		if skillVisible(roots.ecc, name, true) {
			return policyflow.Route{Name: requirement.name, Mode: policyflow.UserExplicit}, true
		}
	default:
		if regularFile(filepath.Join(roots.matt, requirement.name, "SKILL.md")) {
			// A regular Skill file is an installed, user-invocable route. It is
			// never current-session visibility evidence.
			mode := policyflow.UserExplicit
			return policyflow.Route{Name: requirement.name, Mode: mode}, true
		}
	}
	return policyflow.Route{}, false
}

func skillVisible(root, name string, includeMigratedCommands bool) bool {
	if root == "" {
		return false
	}
	if regularFile(filepath.Join(root, "skills", name, "SKILL.md")) {
		return true
	}
	return includeMigratedCommands && regularFile(filepath.Join(root, ".codex-plugin", "migrated-command-skills", name, "SKILL.md"))
}

type codexPlugins struct {
	superpowers bool
	ecc         bool
}

func readCodexPlugins(home string) (codexPlugins, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	raw, err := readOptionalFile(path, maximumCodexConfigBytes)
	if err != nil {
		return codexPlugins{}, fmt.Errorf("POLICY_ROUTE_CONFIG_READ_FAILED: %w", err)
	}
	if raw == nil {
		return codexPlugins{}, nil
	}
	var document struct {
		Plugins map[string]struct {
			Enabled bool `toml:"enabled"`
		} `toml:"plugins"`
	}
	if _, err := toml.Decode(string(raw), &document); err != nil {
		return codexPlugins{}, fmt.Errorf("POLICY_ROUTE_CONFIG_INVALID: %w", err)
	}
	return codexPlugins{
		superpowers: document.Plugins["superpowers@openai-api-curated"].Enabled,
		ecc:         document.Plugins["ecc@ecc"].Enabled,
	}, nil
}

func superpowersRoots(home string) (string, error) {
	plugins := filepath.Join(home, ".codex", "plugins")
	return selectRoot(
		[]string{filepath.Join(plugins, "superpowers")},
		[]string{
			filepath.Join(plugins, "cache", "openai-api-curated", "superpowers"),
			filepath.Join(plugins, "cache", "openai-curated-remote", "superpowers"),
		},
	)
}

func eccRoots(home string) (string, error) {
	plugins := filepath.Join(home, ".codex", "plugins")
	return selectRoot(
		[]string{
			filepath.Join(plugins, "ecc"),
			filepath.Join(plugins, "everything-claude-code", "ecc"),
		},
		[]string{
			filepath.Join(plugins, "cache", "ecc", "ecc"),
			filepath.Join(plugins, "cache", "everything-claude-code", "ecc"),
		},
	)
}

// selectRoot returns one physical installation atomically. Direct roots and
// cache namespaces are ordered by the caller; only versions inside the first
// available cache namespace compete. Skills from different installations must
// never be merged into a synthetic Profile.
func selectRoot(direct, versionParents []string) (string, error) {
	for _, root := range direct {
		if directory(root) {
			return root, nil
		}
	}
	for _, parent := range versionParents {
		entries, err := os.ReadDir(parent)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("POLICY_ROUTE_PLUGIN_READ_FAILED: %s: %w", parent, err)
		}
		candidates := []string{}
		for _, entry := range entries {
			root := filepath.Join(parent, entry.Name())
			if !entry.IsDir() || !validPluginVersion(entry.Name()) {
				continue
			}
			candidates = append(candidates, root)
		}
		if len(candidates) == 0 {
			continue
		}
		for _, candidate := range candidates {
			if filepath.Base(candidate) == "local" {
				return candidate, nil
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			left := filepath.Base(candidates[i])
			right := filepath.Base(candidates[j])
			if compared := comparePluginVersions(left, right); compared != 0 {
				return compared < 0
			}
			return candidates[i] < candidates[j]
		})
		return candidates[len(candidates)-1], nil
	}
	return "", nil
}

func validPluginVersion(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if !unicode.IsDigit(character) && !unicode.IsLetter(character) &&
			character != '-' && character != '_' && character != '.' && character != '+' {
			return false
		}
		if character > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// Codex treats strict semantic versions semantically and all other accepted
// cache version identifiers as opaque strings.
func comparePluginVersions(left, right string) int {
	leftVersion, leftOK := parseSemanticVersion(left)
	rightVersion, rightOK := parseSemanticVersion(right)
	if !leftOK || !rightOK {
		return strings.Compare(left, right)
	}
	if leftVersion.major != rightVersion.major {
		return compareUint64(leftVersion.major, rightVersion.major)
	}
	if leftVersion.minor != rightVersion.minor {
		return compareUint64(leftVersion.minor, rightVersion.minor)
	}
	if leftVersion.patch != rightVersion.patch {
		return compareUint64(leftVersion.patch, rightVersion.patch)
	}
	if compared := comparePrerelease(leftVersion.prerelease, rightVersion.prerelease); compared != 0 {
		return compared
	}
	return compareBuildMetadata(leftVersion.build, rightVersion.build)
}

type semanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []string
	build      []string
}

func parseSemanticVersion(value string) (semanticVersion, bool) {
	withoutBuild, build, hasBuild := strings.Cut(value, "+")
	if hasBuild && !validSemanticIdentifiers(build, false) {
		return semanticVersion{}, false
	}
	core, prerelease, hasPrerelease := strings.Cut(withoutBuild, "-")
	if hasPrerelease && !validSemanticIdentifiers(prerelease, true) {
		return semanticVersion{}, false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semanticVersion{}, false
	}
	numbers := [3]uint64{}
	for index, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return semanticVersion{}, false
		}
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return semanticVersion{}, false
		}
		numbers[index] = parsed
	}
	version := semanticVersion{major: numbers[0], minor: numbers[1], patch: numbers[2]}
	if hasPrerelease {
		version.prerelease = strings.Split(prerelease, ".")
	}
	if hasBuild {
		version.build = strings.Split(build, ".")
	}
	return version, true
}

func validSemanticIdentifiers(value string, prerelease bool) bool {
	if value == "" {
		return false
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" || prerelease && numericIdentifier(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
		for _, character := range identifier {
			if character > unicode.MaxASCII || !unicode.IsLetter(character) && !unicode.IsDigit(character) && character != '-' {
				return false
			}
		}
	}
	return true
}

func comparePrerelease(left, right []string) int {
	if len(left) == 0 && len(right) == 0 {
		return 0
	}
	if len(left) == 0 {
		return 1
	}
	if len(right) == 0 {
		return -1
	}
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] == right[index] {
			continue
		}
		leftNumeric := numericIdentifier(left[index])
		rightNumeric := numericIdentifier(right[index])
		switch {
		case leftNumeric && !rightNumeric:
			return -1
		case !leftNumeric && rightNumeric:
			return 1
		case leftNumeric:
			if len(left[index]) < len(right[index]) {
				return -1
			}
			if len(left[index]) > len(right[index]) {
				return 1
			}
		}
		return strings.Compare(left[index], right[index])
	}
	return compareInt(len(left), len(right))
}

func compareBuildMetadata(left, right []string) int {
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] == right[index] {
			continue
		}
		leftNumeric := numericIdentifier(left[index])
		rightNumeric := numericIdentifier(right[index])
		switch {
		case leftNumeric && !rightNumeric:
			return -1
		case !leftNumeric && rightNumeric:
			return 1
		case leftNumeric:
			leftValue := strings.TrimLeft(left[index], "0")
			rightValue := strings.TrimLeft(right[index], "0")
			if len(leftValue) != len(rightValue) {
				return compareInt(len(leftValue), len(rightValue))
			}
			if leftValue != rightValue {
				return strings.Compare(leftValue, rightValue)
			}
			return compareInt(len(left[index]), len(right[index]))
		default:
			return strings.Compare(left[index], right[index])
		}
	}
	return compareInt(len(left), len(right))
}

func numericIdentifier(value string) bool {
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return value != ""
}

func compareUint64(left, right uint64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareInt(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func validateHome(home string) error {
	if home == "" || !filepath.IsAbs(home) || filepath.Clean(home) != home || strings.IndexFunc(home, unicode.IsControl) >= 0 {
		return fmt.Errorf("POLICY_ROUTE_HOME_INVALID: home must be a clean absolute path")
	}
	info, err := os.Stat(home)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("POLICY_ROUTE_HOME_INVALID: home must be a directory")
	}
	return nil
}

func readOptionalFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return os.ReadFile(path)
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func directory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
