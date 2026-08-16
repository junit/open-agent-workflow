package management

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseInstallationStateAcceptsCanonicalUserAndSharedProjectRecords(t *testing.T) {
	user := canonicalUserState()
	state, err := parseInstallationState([]byte(user))
	if err != nil {
		t.Fatalf("parse user state: %v", err)
	}
	if state.scope != "user" || len(state.targets) != 2 || state.targets[0].id != "claude" || state.targets[0].artifact != routerArtifactID {
		t.Fatalf("user state = %#v", state)
	}
	project := strings.Join([]string{
		"format\t2",
		"version\t0.1.0",
		"scope\tproject",
		"project\t/project",
		"policy\t/config/open-agent-workflow/POLICY.md\t1:1",
		"policy-file\t/config/open-agent-workflow/POLICY.md\t1:1",
		"target\tcodex\trouter\t/project/AGENTS.md\tmanaged-block\t2:2\texisting-file",
		"target\tcodex\tnative-entrypoint\t/project/.agents/skills/oaw/SKILL.md\towned-file\t3:3\tcreated-file",
		"target\tcodex\tnative-policy\t/project/.agents/skills/oaw/agents/openai.yaml\towned-file\t4:4\tcreated-file",
		"target\topencode\trouter\t/project/AGENTS.md\tmanaged-block\t2:2\texisting-file",
		"target\topencode\tnative-entrypoint\t/project/.opencode/commands/oaw.md\towned-file\t5:5\tcreated-file",
	}, "\n") + "\n"
	state, err = parseInstallationState([]byte(project))
	if err != nil {
		t.Fatalf("parse shared project state: %v", err)
	}
	if state.scope != "project" || state.project != "/project" || len(state.targets) != 5 {
		t.Fatalf("project state = %#v", state)
	}
}

func TestParseInstallationStateAcceptsFormatOneAsLegacyRouters(t *testing.T) {
	legacy := strings.Join([]string{
		"format\t1",
		"version\t0.1.0",
		"scope\tuser",
		"policy\t/config/open-agent-workflow/POLICY.md\t1:1",
		"policy-file\t/config/open-agent-workflow/POLICY.md\t1:1",
		"target\tclaude\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file",
	}, "\n") + "\n"
	state, err := parseInstallationState([]byte(legacy))
	if err != nil {
		t.Fatalf("parse format 1 state: %v", err)
	}
	if len(state.targets) != 1 || state.targets[0].artifact != routerArtifactID {
		t.Fatalf("legacy targets = %#v", state.targets)
	}
	rendered, err := serializeInstallState(state)
	if err != nil {
		t.Fatalf("serialize format 1 state: %v", err)
	}
	if !strings.HasPrefix(string(rendered), "format\t1\n") || strings.Contains(string(rendered), "target\tclaude\trouter\t") {
		t.Fatalf("serialized legacy state = %q", rendered)
	}
}

func TestParseInstallationStateRejectsMalformedFormatOneRecords(t *testing.T) {
	base := strings.Join([]string{
		"format\t1",
		"version\t0.1.1",
		"scope\tuser",
		"policy\t/config/open-agent-workflow/POLICY.md\t1:1",
		"policy-file\t/config/open-agent-workflow/POLICY.md\t1:1",
		"target\tclaude\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file",
	}, "\n") + "\n"
	tests := map[string]string{
		"format two target fields": strings.Replace(
			base,
			"target\tclaude\t/home/.claude/CLAUDE.md",
			"target\tclaude\trouter\t/home/.claude/CLAUDE.md",
			1,
		),
		"duplicate target":        base + "target\tclaude\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n",
		"unsupported user target": strings.Replace(base, "target\tclaude\t/home/.claude/CLAUDE.md", "target\tcursor\t/project/.cursor/rules/open-agent-workflow.mdc", 1),
		"wrong router ownership":  strings.Replace(base, "managed-block", "owned-file", 1),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if state, err := parseInstallationState([]byte(raw)); err == nil {
				t.Fatalf("parseInstallationState() = %#v, want error", state)
			}
		})
	}
}

func TestLoadInstallationStateEnforcesReadLimit(t *testing.T) {
	root := t.TempDir()
	environment := Environment{Home: filepath.Join(root, "home"), ConfigHome: filepath.Join(root, "config"), StateHome: filepath.Join(root, "state")}
	coords, err := initializeCoordinates(environment, resolvedRequest{scope: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(coords.stateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coords.stateFile, make([]byte, maximumStateBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := loadInstallationState(coords.stateFile, coords); !exists || err == nil {
		t.Fatalf("loadInstallationState() exists=%t error=%v", exists, err)
	}
}

func TestLoadInstallationStateIgnoresNonRegularPath(t *testing.T) {
	root := t.TempDir()
	environment := Environment{Home: filepath.Join(root, "home"), ConfigHome: filepath.Join(root, "config"), StateHome: filepath.Join(root, "state")}
	coords, err := initializeCoordinates(environment, resolvedRequest{scope: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(coords.stateFile, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := loadInstallationState(coords.stateFile, coords); exists || err != nil {
		t.Fatalf("loadInstallationState() exists=%t error=%v", exists, err)
	}
}

func TestParseInstallationStateRejectsInvalidSeparatorsAndEOF(t *testing.T) {
	repeatedTabs := strings.ReplaceAll(canonicalUserState(), "\t", "\t\t")
	if _, err := parseInstallationState([]byte(repeatedTabs)); err != nil {
		t.Fatalf("repeated tab separators: %v", err)
	}
	trailingTabs := strings.ReplaceAll(canonicalUserState(), "\n", "\t\n")
	if _, err := parseInstallationState([]byte(trailingTabs)); err != nil {
		t.Fatalf("trailing tab separators: %v", err)
	}
	unterminated := strings.TrimSuffix(canonicalUserState(), "\n")
	if _, err := parseInstallationState([]byte(unterminated)); err == nil {
		t.Fatal("unterminated final target record was accepted")
	}
}

func TestParseInstallationStateRejectsMalformedRecords(t *testing.T) {
	base := canonicalUserState()
	tests := []struct {
		name string
		raw  string
	}{
		{name: "duplicate format", raw: "format\t2\n" + base},
		{name: "legacy format with format two target", raw: strings.Replace(base, "format\t2", "format\t1", 1)},
		{name: "project in user", raw: strings.Replace(base, "scope\tuser\n", "scope\tuser\nproject\t/project\n", 1)},
		{name: "project missing root", raw: strings.Replace(base, "scope\tuser", "scope\tproject", 1)},
		{name: "unknown record", raw: base + "mystery\tvalue\n"},
		{name: "too many fields", raw: strings.Replace(base, "version\t0.1.0", "version\t0.1.0\textra", 1)},
		{name: "relative policy", raw: strings.Replace(base, "/config/open-agent-workflow/POLICY.md", "config/POLICY.md", 1)},
		{name: "invalid checksum", raw: strings.Replace(base, "1:1", "sha256:bad", 1)},
		{name: "unknown target", raw: strings.Replace(base, "target\tclaude\t", "target\tvscode\t", 1)},
		{name: "unknown artifact", raw: strings.Replace(base, "target\tclaude\trouter\t", "target\tclaude\tunknown\t", 1)},
		{name: "extension user target", raw: strings.Replace(base, "target\tclaude\t", "target\tcursor\t", 1)},
		{name: "relative target path", raw: strings.Replace(base, "/home/.claude/CLAUDE.md", ".claude/CLAUDE.md", 1)},
		{name: "wrong ownership", raw: strings.Replace(base, "managed-block", "owned-file", 1)},
		{name: "wrong origin", raw: strings.Replace(base, "existing-file", "borrowed-file", 1)},
		{name: "duplicate target artifact", raw: base + "target\tclaude\trouter\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n"},
		{name: "incomplete target artifacts", raw: strings.Replace(base, "target\tclaude\tnative-entrypoint\t/home/.claude/skills/oaw/SKILL.md\towned-file\t3:3\tcreated-file\n", "", 1)},
		{name: "artifact registry order", raw: strings.Replace(base,
			"target\tclaude\trouter\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n"+
				"target\tclaude\tnative-entrypoint\t/home/.claude/skills/oaw/SKILL.md\towned-file\t3:3\tcreated-file\n",
			"target\tclaude\tnative-entrypoint\t/home/.claude/skills/oaw/SKILL.md\towned-file\t3:3\tcreated-file\n"+
				"target\tclaude\trouter\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n", 1)},
		{name: "registry order", raw: strings.Replace(base, "target\tclaude\trouter\t", "target\tcodex\trouter\t", 1) + "target\tclaude\trouter\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n"},
		{name: "conflicting shared destination", raw: strings.Join([]string{
			"format\t2", "version\t0.1.0", "scope\tproject", "project\t/project",
			"policy\t/config/open-agent-workflow/POLICY.md\t1:1",
			"policy-file\t/config/open-agent-workflow/POLICY.md\t1:1",
			"target\tcodex\trouter\t/project/AGENTS.md\tmanaged-block\t2:2\texisting-file",
			"target\tcodex\tnative-entrypoint\t/project/.agents/skills/oaw/SKILL.md\towned-file\t3:3\tcreated-file",
			"target\tcodex\tnative-policy\t/project/.agents/skills/oaw/agents/openai.yaml\towned-file\t4:4\tcreated-file",
			"target\topencode\trouter\t/project/AGENTS.md\tmanaged-block\t3:3\texisting-file",
			"target\topencode\tnative-entrypoint\t/project/.opencode/commands/oaw.md\towned-file\t5:5\tcreated-file",
		}, "\n") + "\n"},
		{name: "duplicate directory", raw: base + "directory\t/home/.claude\ndirectory\t/home/.claude\n"},
		{name: "relative directory", raw: base + "directory\t.home\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if state, err := parseInstallationState([]byte(tt.raw)); err == nil {
				t.Fatalf("parseInstallationState() = %#v, want error", state)
			}
		})
	}
}

func TestParseInstallationStateRejectsMalformedTargetRecords(t *testing.T) {
	base := canonicalUserState()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "relative path", raw: strings.Replace(base, "/home/.claude/CLAUDE.md", ".claude/CLAUDE.md", 1), want: "invalid target path"},
		{name: "ownership", raw: strings.Replace(base, "managed-block", "owned-file", 1), want: "invalid target ownership"},
		{name: "checksum", raw: strings.Replace(base, "2:2", "invalid", 1), want: "invalid target checksum"},
		{name: "origin", raw: strings.Replace(base, "existing-file", "borrowed-file", 1), want: "invalid target ownership"},
		{name: "duplicate", raw: base + "target\tclaude\trouter\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n", want: "duplicate target artifact state: claude/router"},
		{name: "incomplete", raw: strings.Replace(base, "target\tclaude\tnative-entrypoint\t/home/.claude/skills/oaw/SKILL.md\towned-file\t3:3\tcreated-file\n", "", 1), want: "incomplete target artifact state: claude/native-entrypoint"},
		{name: "registry order", raw: strings.Replace(base, "target\tclaude\trouter\t", "target\tcodex\trouter\t", 1) + "target\tclaude\trouter\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n", want: "target state is not in registry order"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseInstallationState([]byte(test.raw))
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func canonicalUserState() string {
	return strings.Join([]string{
		"format\t2",
		"version\t0.1.0",
		"scope\tuser",
		"policy\t/config/open-agent-workflow/POLICY.md\t1:1",
		"policy-file\t/config/open-agent-workflow/POLICY.md\t1:1",
		"target\tclaude\trouter\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file",
		"target\tclaude\tnative-entrypoint\t/home/.claude/skills/oaw/SKILL.md\towned-file\t3:3\tcreated-file",
	}, "\n") + "\n"
}
