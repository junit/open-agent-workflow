package management

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type target struct {
	ID        string
	User      bool
	Artifacts []targetArtifact
}

type targetArtifact struct {
	ID            string
	Kind          string
	UserRoot      string
	UserSuffix    string
	ProjectSuffix string
	Ownership     string
}

const (
	routerArtifactID           = "router"
	nativeEntrypointArtifactID = "native-entrypoint"
	nativePolicyArtifactID     = "native-policy"

	activationRouterArtifactKind = "activation-router"
	skillArtifactKind            = "skill"
	commandArtifactKind          = "command"
	geminiCommandArtifactKind    = "gemini-command"
	workflowArtifactKind         = "workflow"
	codexPolicyArtifactKind      = "codex-policy"

	userRootHome   = "home"
	userRootConfig = "config"
)

var targetRegistry = [...]target{
	newTarget("claude", true,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, UserRoot: userRootHome, UserSuffix: ".claude/CLAUDE.md", ProjectSuffix: ".claude/CLAUDE.md", Ownership: "managed-block"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: skillArtifactKind, UserRoot: userRootHome, UserSuffix: ".claude/skills/oaw/SKILL.md", ProjectSuffix: ".claude/skills/oaw/SKILL.md", Ownership: "owned-file"},
	),
	newTarget("codex", true,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, UserRoot: userRootHome, UserSuffix: ".codex/AGENTS.md", ProjectSuffix: "AGENTS.md", Ownership: "managed-block"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: skillArtifactKind, UserRoot: userRootHome, UserSuffix: ".agents/skills/oaw/SKILL.md", ProjectSuffix: ".agents/skills/oaw/SKILL.md", Ownership: "owned-file"},
		targetArtifact{ID: nativePolicyArtifactID, Kind: codexPolicyArtifactKind, UserRoot: userRootHome, UserSuffix: ".agents/skills/oaw/agents/openai.yaml", ProjectSuffix: ".agents/skills/oaw/agents/openai.yaml", Ownership: "owned-file"},
	),
	newTarget("gemini", true,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, UserRoot: userRootHome, UserSuffix: ".gemini/GEMINI.md", ProjectSuffix: "GEMINI.md", Ownership: "managed-block"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: geminiCommandArtifactKind, UserRoot: userRootHome, UserSuffix: ".gemini/commands/oaw.toml", ProjectSuffix: ".gemini/commands/oaw.toml", Ownership: "owned-file"},
	),
	newTarget("opencode", true,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, UserRoot: userRootConfig, UserSuffix: "opencode/AGENTS.md", ProjectSuffix: "AGENTS.md", Ownership: "managed-block"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: commandArtifactKind, UserRoot: userRootConfig, UserSuffix: "opencode/commands/oaw.md", ProjectSuffix: ".opencode/commands/oaw.md", Ownership: "owned-file"},
	),
	newTarget("cursor", false,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, ProjectSuffix: ".cursor/rules/open-agent-workflow.mdc", Ownership: "owned-file"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: skillArtifactKind, ProjectSuffix: ".cursor/skills/oaw/SKILL.md", Ownership: "owned-file"},
	),
	newTarget("windsurf", false,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, ProjectSuffix: ".devin/rules/open-agent-workflow.md", Ownership: "owned-file"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: workflowArtifactKind, ProjectSuffix: ".windsurf/workflows/oaw.md", Ownership: "owned-file"},
	),
	newTarget("cline", false,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, ProjectSuffix: ".clinerules/open-agent-workflow.md", Ownership: "owned-file"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: skillArtifactKind, ProjectSuffix: ".cline/skills/oaw/SKILL.md", Ownership: "owned-file"},
	),
	newTarget("roo", false,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, ProjectSuffix: ".roo/rules/open-agent-workflow.md", Ownership: "owned-file"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: commandArtifactKind, ProjectSuffix: ".roo/commands/oaw.md", Ownership: "owned-file"},
	),
	newTarget("copilot", false,
		targetArtifact{ID: routerArtifactID, Kind: activationRouterArtifactKind, ProjectSuffix: ".github/instructions/open-agent-workflow.instructions.md", Ownership: "owned-file"},
		targetArtifact{ID: nativeEntrypointArtifactID, Kind: skillArtifactKind, ProjectSuffix: ".github/skills/oaw/SKILL.md", Ownership: "owned-file"},
	),
}

func newTarget(id string, user bool, artifacts ...targetArtifact) target {
	return target{ID: id, User: user, Artifacts: append([]targetArtifact(nil), artifacts...)}
}

type resolvedRequest struct {
	scope       string
	projectRoot string
	targets     []string
}

func resolve(request CheckRequest) (resolvedRequest, error) {
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
			if targetSupportsScope(candidate, scope) {
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
		if !targetSupportsScope(candidate, scope) {
			return nil, usageError(fmt.Sprintf("target '%s' does not support %s scope", member, scope))
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

func targetSupportsScope(candidate target, operationScope string) bool {
	switch operationScope {
	case "user":
		if !candidate.User {
			return false
		}
	case "project":
		// Every registered target currently supports project scope, but the
		// artifact-level check below keeps that invariant explicit.
	default:
		return false
	}
	if len(candidate.Artifacts) == 0 {
		return false
	}
	for _, artifact := range candidate.Artifacts {
		if artifact.ID == "" || artifact.ProjectSuffix == "" {
			return false
		}
		if operationScope == "user" && (artifact.UserSuffix == "" || artifact.UserRoot == "") {
			return false
		}
	}
	return true
}

func targetArtifactsForScope(id, operationScope string) ([]targetArtifact, error) {
	candidate, found := findTarget(id)
	if !found {
		return nil, integrityError("unknown target '" + id + "'")
	}
	if !targetSupportsScope(candidate, operationScope) {
		return nil, integrityError(fmt.Sprintf("target '%s' does not support %s scope", id, operationScope))
	}
	return append([]targetArtifact(nil), candidate.Artifacts...), nil
}

func findTargetArtifact(targetID, artifactID string) (targetArtifact, bool) {
	candidate, found := findTarget(targetID)
	if !found {
		return targetArtifact{}, false
	}
	for _, artifact := range candidate.Artifacts {
		if artifact.ID == artifactID {
			return artifact, true
		}
	}
	return targetArtifact{}, false
}

// targetArtifactPosition is a stable global registry position used to enforce
// deterministic state ordering across logical targets and their artifacts.
func targetArtifactPosition(targetID, artifactID string) int {
	position := 0
	for _, candidate := range targetRegistry {
		for _, artifact := range candidate.Artifacts {
			position++
			if candidate.ID == targetID && artifact.ID == artifactID {
				return position
			}
		}
	}
	return 0
}

func hasControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}
