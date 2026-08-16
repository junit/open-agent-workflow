package management

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSerializeInstallStateUsesCanonicalOrderAndRoundTrips(t *testing.T) {
	state := installationState{
		version:        "0.1.0",
		scope:          "project",
		project:        "/project path",
		policyPath:     "/config/open-agent-workflow/POLICY.md",
		policyChecksum: "101:202",
		policyFiles:    []policyFileRecord{{path: "/config/open-agent-workflow/POLICY.md", checksum: "101:202"}},
		backupPath:     "/state/open-agent-workflow/backups/prior",
		directories:    []string{"/project path/.codex", "/state/open-agent-workflow"},
		targets: []targetRecord{
			{id: "codex", artifact: routerArtifactID, path: "/project path/AGENTS.md", mode: "managed-block", checksum: "303:404", origin: "existing-file"},
			{id: "codex", artifact: nativeEntrypointArtifactID, path: "/project path/.agents/skills/oaw/SKILL.md", mode: "owned-file", checksum: "304:405", origin: "created-file"},
			{id: "codex", artifact: nativePolicyArtifactID, path: "/project path/.agents/skills/oaw/agents/openai.yaml", mode: "owned-file", checksum: "305:406", origin: "created-file"},
			{id: "opencode", artifact: routerArtifactID, path: "/project path/AGENTS.md", mode: "managed-block", checksum: "303:404", origin: "existing-file"},
			{id: "opencode", artifact: nativeEntrypointArtifactID, path: "/project path/.opencode/commands/oaw.md", mode: "owned-file", checksum: "505:606", origin: "created-file"},
			{id: "cursor", artifact: routerArtifactID, path: "/project path/.cursor/rules/open-agent-workflow.mdc", mode: "owned-file", checksum: "507:608", origin: "created-file"},
			{id: "cursor", artifact: nativeEntrypointArtifactID, path: "/project path/.cursor/skills/oaw/SKILL.md", mode: "owned-file", checksum: "508:609", origin: "created-file"},
		},
	}
	want := strings.Join([]string{
		"format\t2",
		"version\t0.1.0",
		"scope\tproject",
		"project\t/project path",
		"policy\t/config/open-agent-workflow/POLICY.md\t101:202",
		"policy-file\t/config/open-agent-workflow/POLICY.md\t101:202",
		"backup\t/state/open-agent-workflow/backups/prior",
		"directory\t/project path/.codex",
		"directory\t/state/open-agent-workflow",
		"target\tcodex\trouter\t/project path/AGENTS.md\tmanaged-block\t303:404\texisting-file",
		"target\tcodex\tnative-entrypoint\t/project path/.agents/skills/oaw/SKILL.md\towned-file\t304:405\tcreated-file",
		"target\tcodex\tnative-policy\t/project path/.agents/skills/oaw/agents/openai.yaml\towned-file\t305:406\tcreated-file",
		"target\topencode\trouter\t/project path/AGENTS.md\tmanaged-block\t303:404\texisting-file",
		"target\topencode\tnative-entrypoint\t/project path/.opencode/commands/oaw.md\towned-file\t505:606\tcreated-file",
		"target\tcursor\trouter\t/project path/.cursor/rules/open-agent-workflow.mdc\towned-file\t507:608\tcreated-file",
		"target\tcursor\tnative-entrypoint\t/project path/.cursor/skills/oaw/SKILL.md\towned-file\t508:609\tcreated-file",
	}, "\n") + "\n"

	got, err := serializeInstallState(state)
	if err != nil {
		t.Fatalf("serializeInstallState() error = %v", err)
	}
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("serializeInstallState() = %q, want %q", got, want)
	}
	parsed, err := parseInstallationState(got)
	if err != nil {
		t.Fatalf("parseInstallationState(serialized): %v", err)
	}
	if !reflect.DeepEqual(parsed, state) {
		t.Fatalf("round trip = %#v, want %#v", parsed, state)
	}
}

func TestSerializeInstallStateRejectsInvalidValues(t *testing.T) {
	valid := installationState{
		version:        "0.1.0",
		scope:          "user",
		policyPath:     "/config/open-agent-workflow/POLICY.md",
		policyChecksum: "1:1",
		policyFiles:    []policyFileRecord{{path: "/config/open-agent-workflow/POLICY.md", checksum: "1:1"}},
		directories:    []string{"/home/.claude"},
		targets: []targetRecord{
			{id: "claude", artifact: routerArtifactID, path: "/home/.claude/CLAUDE.md", mode: "managed-block", checksum: "2:2", origin: "existing-file"},
			{id: "claude", artifact: nativeEntrypointArtifactID, path: "/home/.claude/skills/oaw/SKILL.md", mode: "owned-file", checksum: "3:3", origin: "created-file"},
		},
	}
	tests := []struct {
		name   string
		mutate func(*installationState)
	}{
		{name: "invalid scope", mutate: func(value *installationState) { value.scope = "workspace" }},
		{name: "project missing root", mutate: func(value *installationState) { value.scope = "project" }},
		{name: "unsafe version", mutate: func(value *installationState) { value.version = "bad\nversion" }},
		{name: "user project", mutate: func(value *installationState) { value.project = "/project" }},
		{name: "relative project", mutate: func(value *installationState) { value.scope, value.project = "project", "project" }},
		{name: "unsafe policy", mutate: func(value *installationState) { value.policyPath = "/config/bad\tpolicy" }},
		{name: "relative policy", mutate: func(value *installationState) { value.policyPath = "config/policy" }},
		{name: "invalid policy checksum", mutate: func(value *installationState) { value.policyChecksum = "sha256:bad" }},
		{name: "unsafe backup", mutate: func(value *installationState) { value.backupPath = "/backup/bad\npath" }},
		{name: "relative backup", mutate: func(value *installationState) { value.backupPath = "backups/prior" }},
		{name: "duplicate directory", mutate: func(value *installationState) { value.directories = append(value.directories, value.directories[0]) }},
		{name: "empty directory", mutate: func(value *installationState) { value.directories[0] = "" }},
		{name: "unsafe directory", mutate: func(value *installationState) { value.directories[0] = "/home/bad\tdir" }},
		{name: "no targets", mutate: func(value *installationState) { value.targets = nil }},
		{name: "relative target", mutate: func(value *installationState) { value.targets[0].path = ".claude/CLAUDE.md" }},
		{name: "unsafe artifact", mutate: func(value *installationState) { value.targets[0].artifact = "bad\tartifact" }},
		{name: "unknown artifact", mutate: func(value *installationState) { value.targets[0].artifact = "unknown" }},
		{name: "invalid origin", mutate: func(value *installationState) { value.targets[0].origin = "borrowed-file" }},
		{name: "registry order", mutate: func(value *installationState) {
			value.targets = []targetRecord{
				{id: "codex", artifact: routerArtifactID, path: "/home/.codex/AGENTS.md", mode: "managed-block", checksum: "3:3", origin: "existing-file"},
				{id: "codex", artifact: nativeEntrypointArtifactID, path: "/home/.agents/skills/oaw/SKILL.md", mode: "owned-file", checksum: "4:4", origin: "created-file"},
				{id: "codex", artifact: nativePolicyArtifactID, path: "/home/.agents/skills/oaw/agents/openai.yaml", mode: "owned-file", checksum: "5:5", origin: "created-file"},
				value.targets[0],
			}
		}},
		{name: "duplicate target artifact", mutate: func(value *installationState) {
			value.targets = append(value.targets, value.targets[0])
		}},
		{name: "incomplete target artifacts", mutate: func(value *installationState) {
			value.targets = value.targets[:1]
		}},
		{name: "conflicting shared destination", mutate: func(value *installationState) {
			value.scope, value.project = "project", "/project"
			value.targets = []targetRecord{
				{id: "codex", artifact: routerArtifactID, path: "/project/AGENTS.md", mode: "managed-block", checksum: "2:2", origin: "existing-file"},
				{id: "codex", artifact: nativeEntrypointArtifactID, path: "/project/.agents/skills/oaw/SKILL.md", mode: "owned-file", checksum: "4:4", origin: "created-file"},
				{id: "codex", artifact: nativePolicyArtifactID, path: "/project/.agents/skills/oaw/agents/openai.yaml", mode: "owned-file", checksum: "5:5", origin: "created-file"},
				{id: "opencode", artifact: routerArtifactID, path: "/project/AGENTS.md", mode: "managed-block", checksum: "3:3", origin: "existing-file"},
				{id: "opencode", artifact: nativeEntrypointArtifactID, path: "/project/.opencode/commands/oaw.md", mode: "owned-file", checksum: "6:6", origin: "created-file"},
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := cloneInstallationState(valid)
			tt.mutate(&candidate)
			if got, err := serializeInstallState(candidate); err == nil {
				t.Fatalf("serializeInstallState() = %q, want error", got)
			}
		})
	}
}

func cloneInstallationState(value installationState) installationState {
	value.directories = append([]string(nil), value.directories...)
	value.policyFiles = append([]policyFileRecord(nil), value.policyFiles...)
	value.targets = append([]targetRecord(nil), value.targets...)
	return value
}
