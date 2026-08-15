package management

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSerializeInstallStateMatchesBashOrderAndRoundTrips(t *testing.T) {
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
			{id: "codex", path: "/project path/AGENTS.md", mode: "managed-block", checksum: "303:404", origin: "existing-file"},
			{id: "opencode", path: "/project path/AGENTS.md", mode: "managed-block", checksum: "303:404", origin: "existing-file"},
			{id: "cursor", path: "/project path/.cursor/rules/open-agent-workflow.mdc", mode: "owned-file", checksum: "505:606", origin: "created-file"},
		},
	}
	want := strings.Join([]string{
		"format\t1",
		"version\t0.1.0",
		"scope\tproject",
		"project\t/project path",
		"policy\t/config/open-agent-workflow/POLICY.md\t101:202",
		"policy-file\t/config/open-agent-workflow/POLICY.md\t101:202",
		"backup\t/state/open-agent-workflow/backups/prior",
		"directory\t/project path/.codex",
		"directory\t/state/open-agent-workflow",
		"target\tcodex\t/project path/AGENTS.md\tmanaged-block\t303:404\texisting-file",
		"target\topencode\t/project path/AGENTS.md\tmanaged-block\t303:404\texisting-file",
		"target\tcursor\t/project path/.cursor/rules/open-agent-workflow.mdc\towned-file\t505:606\tcreated-file",
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
			{id: "claude", path: "/home/.claude/CLAUDE.md", mode: "managed-block", checksum: "2:2", origin: "existing-file"},
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
		{name: "invalid origin", mutate: func(value *installationState) { value.targets[0].origin = "borrowed-file" }},
		{name: "registry order", mutate: func(value *installationState) {
			value.targets = []targetRecord{
				{id: "codex", path: "/home/.codex/AGENTS.md", mode: "managed-block", checksum: "3:3", origin: "existing-file"},
				value.targets[0],
			}
		}},
		{name: "conflicting shared destination", mutate: func(value *installationState) {
			value.scope, value.project = "project", "/project"
			value.targets = []targetRecord{
				{id: "codex", path: "/project/AGENTS.md", mode: "managed-block", checksum: "2:2", origin: "existing-file"},
				{id: "opencode", path: "/project/AGENTS.md", mode: "managed-block", checksum: "3:3", origin: "existing-file"},
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
