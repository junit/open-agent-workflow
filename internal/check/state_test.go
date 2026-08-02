package check

import (
	"strings"
	"testing"
)

func TestParseInstallationStateAcceptsCanonicalUserAndSharedProjectRecords(t *testing.T) {
	user := canonicalUserState()
	state, err := parseInstallationState([]byte(user))
	if err != nil {
		t.Fatalf("parse user state: %v", err)
	}
	if state.scope != "user" || len(state.targets) != 1 || state.targets[0].id != "claude" {
		t.Fatalf("user state = %#v", state)
	}
	project := strings.Join([]string{
		"format\t1",
		"version\t0.1.0",
		"scope\tproject",
		"project\t/project",
		"policy\t/config/open-agent-workflow/ENGINEERING.md\t1:1",
		"target\tcodex\t/project/AGENTS.md\tmanaged-block\t2:2\texisting-file",
		"target\topencode\t/project/AGENTS.md\tmanaged-block\t2:2\texisting-file",
	}, "\n") + "\n"
	state, err = parseInstallationState([]byte(project))
	if err != nil {
		t.Fatalf("parse shared project state: %v", err)
	}
	if state.scope != "project" || state.project != "/project" || len(state.targets) != 2 {
		t.Fatalf("project state = %#v", state)
	}
}

func TestParseInstallationStateRejectsMalformedRecords(t *testing.T) {
	base := canonicalUserState()
	tests := []struct {
		name string
		raw  string
	}{
		{name: "duplicate format", raw: "format\t1\n" + base},
		{name: "project in user", raw: strings.Replace(base, "scope\tuser\n", "scope\tuser\nproject\t/project\n", 1)},
		{name: "project missing root", raw: strings.Replace(base, "scope\tuser", "scope\tproject", 1)},
		{name: "unknown record", raw: base + "mystery\tvalue\n"},
		{name: "too many fields", raw: strings.Replace(base, "version\t0.1.0", "version\t0.1.0\textra", 1)},
		{name: "relative policy", raw: strings.Replace(base, "/config/open-agent-workflow/ENGINEERING.md", "config/ENGINEERING.md", 1)},
		{name: "invalid checksum", raw: strings.Replace(base, "1:1", "sha256:bad", 1)},
		{name: "unknown target", raw: strings.Replace(base, "target\tclaude", "target\tvscode", 1)},
		{name: "extension user target", raw: strings.Replace(base, "target\tclaude", "target\tcursor", 1)},
		{name: "relative target path", raw: strings.Replace(base, "/home/.claude/CLAUDE.md", ".claude/CLAUDE.md", 1)},
		{name: "wrong ownership", raw: strings.Replace(base, "managed-block", "owned-file", 1)},
		{name: "wrong origin", raw: strings.Replace(base, "existing-file", "borrowed-file", 1)},
		{name: "duplicate target", raw: base + "target\tclaude\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n"},
		{name: "registry order", raw: strings.Replace(base, "target\tclaude", "target\tcodex", 1) + "target\tclaude\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file\n"},
		{name: "conflicting shared destination", raw: strings.Join([]string{
			"format\t1", "version\t0.1.0", "scope\tproject", "project\t/project",
			"policy\t/config/open-agent-workflow/ENGINEERING.md\t1:1",
			"target\tcodex\t/project/AGENTS.md\tmanaged-block\t2:2\texisting-file",
			"target\topencode\t/project/AGENTS.md\tmanaged-block\t3:3\texisting-file",
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

func canonicalUserState() string {
	return strings.Join([]string{
		"format\t1",
		"version\t0.1.0",
		"scope\tuser",
		"policy\t/config/open-agent-workflow/ENGINEERING.md\t1:1",
		"target\tclaude\t/home/.claude/CLAUDE.md\tmanaged-block\t2:2\texisting-file",
	}, "\n") + "\n"
}
