package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type target struct {
	ID            string
	UserSuffix    string
	ProjectSuffix string
	Ownership     string
	User          bool
}

var targetRegistry = [...]target{
	{ID: "claude", UserSuffix: ".claude/CLAUDE.md", ProjectSuffix: ".claude/CLAUDE.md", Ownership: "managed-block", User: true},
	{ID: "codex", UserSuffix: ".codex/AGENTS.md", ProjectSuffix: "AGENTS.md", Ownership: "managed-block", User: true},
	{ID: "gemini", UserSuffix: ".gemini/GEMINI.md", ProjectSuffix: "GEMINI.md", Ownership: "managed-block", User: true},
	{ID: "opencode", UserSuffix: "opencode/AGENTS.md", ProjectSuffix: "AGENTS.md", Ownership: "managed-block", User: true},
	{ID: "cursor", ProjectSuffix: ".cursor/rules/open-agent-workflow.mdc", Ownership: "owned-file"},
	{ID: "windsurf", ProjectSuffix: ".devin/rules/open-agent-workflow.md", Ownership: "owned-file"},
	{ID: "cline", ProjectSuffix: ".clinerules/open-agent-workflow.md", Ownership: "owned-file"},
	{ID: "roo", ProjectSuffix: ".roo/rules/open-agent-workflow.md", Ownership: "owned-file"},
	{ID: "copilot", ProjectSuffix: ".github/instructions/open-agent-workflow.instructions.md", Ownership: "owned-file"},
}

type resolvedRequest struct {
	scope       string
	projectRoot string
	targets     []string
}

func resolve(request Request) (resolvedRequest, error) {
	result := resolvedRequest{scope: "user"}
	if request.Project != "" {
		if hasControl(request.Project) {
			return resolvedRequest{}, usageError("project path contains control characters")
		}
		info, err := os.Stat(request.Project)
		if err != nil || !info.IsDir() {
			return resolvedRequest{}, usageError("project directory does not exist: " + request.Project)
		}
		absolute, err := filepath.Abs(request.Project)
		if err != nil {
			return resolvedRequest{}, usageError("project directory could not be resolved: " + request.Project)
		}
		physical, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return resolvedRequest{}, usageError("project directory could not be resolved: " + request.Project)
		}
		if hasControl(physical) {
			return resolvedRequest{}, usageError("resolved project path contains control characters")
		}
		if !filepath.IsAbs(physical) {
			return resolvedRequest{}, usageError("root must be an absolute path: " + physical)
		}
		result.scope = "project"
		result.projectRoot = filepath.Clean(physical)
	}
	targets, err := normalizeTargets(request.Targets, result.scope)
	if err != nil {
		return resolvedRequest{}, err
	}
	result.targets = targets
	return result, nil
}

func normalizeTargets(selection, scope string) ([]string, error) {
	if selection == "" {
		result := make([]string, 0, len(targetRegistry))
		for _, candidate := range targetRegistry {
			if scope == "project" || candidate.User {
				result = append(result, candidate.ID)
			}
		}
		return result, nil
	}
	if strings.IndexFunc(selection, unicode.IsSpace) >= 0 {
		return nil, usageError("target selection must not contain whitespace")
	}
	if strings.HasPrefix(selection, ",") || strings.HasSuffix(selection, ",") || strings.Contains(selection, ",,") {
		return nil, usageError("target selection contains an empty member")
	}
	selected := make(map[string]bool)
	for _, member := range strings.Split(selection, ",") {
		candidate, found := findTarget(member)
		if !found {
			return nil, usageError(fmt.Sprintf("unknown target '%s'", member))
		}
		if scope == "user" && !candidate.User {
			return nil, usageError(fmt.Sprintf("target '%s' does not support user scope", member))
		}
		selected[member] = true
	}
	result := make([]string, 0, len(selected))
	for _, candidate := range targetRegistry {
		if selected[candidate.ID] {
			result = append(result, candidate.ID)
		}
	}
	return result, nil
}

func findTarget(id string) (target, bool) {
	for _, candidate := range targetRegistry {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return target{}, false
}

func hasControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}
