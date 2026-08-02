package management

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTargetMatchesBashBytes(t *testing.T) {
	policyPath := "/config path/`policy`/ENGINEERING.md"
	bootstrap := "Before engineering lifecycle work, read `" + policyPath + "`, follow its blocking selection gate, and preserve the selected lifecycle bundle for the task.\n"
	tests := []struct {
		name  string
		scope scope
		id    targetID
		want  string
	}{
		{name: "user claude", scope: "user", id: "claude", want: "Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:\n@" + policyPath + "\n"},
		{name: "user codex", scope: "user", id: "codex", want: "For every new top-level engineering task that may use workflow skills, first read `" + policyPath + "`, run its blocking selection gate, and preserve the selected lifecycle bundle for the task.\n"},
		{name: "user gemini", scope: "user", id: "gemini", want: "Follow the Open Agent Workflow policy before engineering lifecycle work:\n@" + policyPath + "\n"},
		{name: "user opencode", scope: "user", id: "opencode", want: "Before engineering lifecycle work, use the Read tool to read `" + policyPath + "`, then follow its blocking selection gate and lifecycle lock.\n"},
		{name: "project claude", scope: "project", id: "claude", want: "Before any new top-level engineering task that may use workflow skills, read and follow the Open Agent Workflow policy:\n@" + policyPath + "\n"},
		{name: "project codex", scope: "project", id: "codex", want: bootstrap},
		{name: "project gemini", scope: "project", id: "gemini", want: "Follow the Open Agent Workflow policy before engineering lifecycle work:\n@" + policyPath + "\n"},
		{name: "project opencode", scope: "project", id: "opencode", want: bootstrap},
		{name: "project cursor", scope: "project", id: "cursor", want: "---\ndescription: Open Agent Workflow lifecycle policy\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + bootstrap},
		{name: "project windsurf", scope: "project", id: "windsurf", want: "---\ntrigger: always_on\n---\n\n" + bootstrap},
		{name: "project cline", scope: "project", id: "cline", want: bootstrap},
		{name: "project roo", scope: "project", id: "roo", want: bootstrap},
		{name: "project copilot", scope: "project", id: "copilot", want: "---\napplyTo: \"**\"\n---\n\n" + bootstrap},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderTarget(tt.id, tt.scope, policyPath)
			if err != nil {
				t.Fatalf("renderTarget() error = %v", err)
			}
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Fatalf("renderTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderManagedBlockWrapsExactRendererBytes(t *testing.T) {
	got, err := renderManagedBlock("codex", "user", "/config/ENGINEERING.md")
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- BEGIN OPEN AGENT WORKFLOW -->\n" +
		"For every new top-level engineering task that may use workflow skills, first read `/config/ENGINEERING.md`, run its blocking selection gate, and preserve the selected lifecycle bundle for the task.\n" +
		"<!-- END OPEN AGENT WORKFLOW -->\n"
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("renderManagedBlock() = %q, want %q", got, want)
	}
}

func TestRenderManagedFileMatchesBashPlacementRules(t *testing.T) {
	block := []byte("<!-- BEGIN OPEN AGENT WORKFLOW -->\nnew body\n<!-- END OPEN AGENT WORKFLOW -->\n")
	old := "<!-- BEGIN OPEN AGENT WORKFLOW -->\nold body\n<!-- END OPEN AGENT WORKFLOW -->"
	tests := []struct {
		name    string
		current string
		want    string
	}{
		{name: "missing or empty", want: string(block)},
		{name: "existing final newline", current: "user content\n", want: "user content\n" + string(block)},
		{name: "existing without final newline", current: "user content", want: string(block) + "user content"},
		{name: "replace only block", current: "before\n" + old + "\nafter\n", want: "before\n" + string(block) + "after\n"},
		{name: "replace block and preserve unterminated suffix", current: old + "\nafter", want: string(block) + "after"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderManagedFile([]byte(tt.current), block)
			if err != nil {
				t.Fatalf("renderManagedFile() error = %v", err)
			}
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Fatalf("renderManagedFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderManagedFileRejectsInvalidMarkers(t *testing.T) {
	block := []byte(beginMarker + "\nbody\n" + endMarker + "\n")
	for name, current := range map[string]string{
		"duplicate": beginMarker + "\n" + beginMarker + "\n" + endMarker + "\n",
		"reversed":  endMarker + "\n" + beginMarker + "\n",
		"partial":   beginMarker + "\nbody\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := renderManagedFile([]byte(current), block); err == nil || !strings.Contains(err.Error(), "managed markers are invalid") {
				t.Fatalf("renderManagedFile() error = %v", err)
			}
		})
	}
}

func TestRenderTargetRejectsUnsupportedPair(t *testing.T) {
	if _, err := renderTarget("cursor", "user", "/config/ENGINEERING.md"); err == nil {
		t.Fatal("renderTarget() accepted user cursor")
	}
}
