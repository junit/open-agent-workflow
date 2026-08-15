package management

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTargetMatchesBashBytes(t *testing.T) {
	policyPath := "/config path/`policy`/ENGINEERING.md"
	prefix := "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. "
	suffix := " Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.\n"
	userRouter := prefix + "On explicit activation, if the current project contains `.oaw/policy/POLICY.md`, read that Project Policy Set and do not read or merge the User Policy Set; otherwise read `" + policyPath + "` as the User Policy Set." + suffix
	projectRouter := prefix + "On explicit activation, read `" + policyPath + "` as the Project Policy Set and do not read or merge the User Policy Set." + suffix
	tests := []struct {
		name  string
		scope scope
		id    targetID
		want  string
	}{
		{name: "user claude", scope: "user", id: "claude", want: userRouter},
		{name: "user codex", scope: "user", id: "codex", want: userRouter},
		{name: "user gemini", scope: "user", id: "gemini", want: userRouter},
		{name: "user opencode", scope: "user", id: "opencode", want: userRouter},
		{name: "project claude", scope: "project", id: "claude", want: projectRouter},
		{name: "project codex", scope: "project", id: "codex", want: projectRouter},
		{name: "project gemini", scope: "project", id: "gemini", want: projectRouter},
		{name: "project opencode", scope: "project", id: "opencode", want: projectRouter},
		{name: "project cursor", scope: "project", id: "cursor", want: "---\ndescription: Open Agent Workflow lifecycle policy\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + projectRouter},
		{name: "project windsurf", scope: "project", id: "windsurf", want: "---\ntrigger: always_on\n---\n\n" + projectRouter},
		{name: "project cline", scope: "project", id: "cline", want: projectRouter},
		{name: "project roo", scope: "project", id: "roo", want: projectRouter},
		{name: "project copilot", scope: "project", id: "copilot", want: "---\napplyTo: \"**\"\n---\n\n" + projectRouter},
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

func TestRenderTargetEnforcesActivationRouterContract(t *testing.T) {
	policyPath := "/config/ENGINEERING.md"
	targets := []struct {
		scope scope
		id    targetID
	}{
		{"user", "claude"}, {"user", "codex"}, {"user", "gemini"}, {"user", "opencode"},
		{"project", "claude"}, {"project", "codex"}, {"project", "gemini"}, {"project", "opencode"},
		{"project", "cursor"}, {"project", "windsurf"}, {"project", "cline"}, {"project", "roo"}, {"project", "copilot"},
	}
	for _, target := range targets {
		t.Run(string(target.scope)+"/"+string(target.id), func(t *testing.T) {
			rendered, err := renderTarget(target.id, target.scope, policyPath)
			if err != nil {
				t.Fatal(err)
			}
			text := string(rendered)
			for _, required := range []string{
				"Open Agent Workflow is opt-in.",
				"explicitly asks to use OAW",
				"behave as the native Host",
				"do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state",
				"ordinary Skill invocation do not activate OAW",
				"Apply the selected Policy Set only to that deliverable",
				"Related follow-ups inherit activation; unrelated requests remain native",
				"explicit exit closes the OAW Engagement",
			} {
				if !strings.Contains(text, required) {
					t.Fatalf("%s/%s omits %q: %q", target.scope, target.id, required, text)
				}
			}
			if target.scope == "user" {
				for _, required := range []string{"current project contains `.oaw/policy/POLICY.md`", "otherwise read `" + policyPath + "` as the User Policy Set"} {
					if !strings.Contains(text, required) {
						t.Fatalf("%s/%s omits %q: %q", target.scope, target.id, required, text)
					}
				}
			} else {
				for _, required := range []string{"read `" + policyPath + "` as the Project Policy Set", "do not read or merge the User Policy Set"} {
					if !strings.Contains(text, required) {
						t.Fatalf("%s/%s omits %q: %q", target.scope, target.id, required, text)
					}
				}
			}
			for _, forbidden := range []string{
				"\n@" + policyPath + "\n",
				"For every new top-level engineering request, first read",
				"Before engineering lifecycle work, read",
				"classify it as DIRECT, BOUNDED, or WORKFLOW",
				"follow its blocking selection gate",
				"preserve the selected lifecycle bundle",
			} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("%s/%s retains forbidden %q: %q", target.scope, target.id, forbidden, text)
				}
			}
		})
	}
}

func TestRenderManagedBlockWrapsExactRendererBytes(t *testing.T) {
	got, err := renderManagedBlock("codex", "user", "/config/ENGINEERING.md")
	if err != nil {
		t.Fatal(err)
	}
	router := "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, if the current project contains `.oaw/policy/POLICY.md`, read that Project Policy Set and do not read or merge the User Policy Set; otherwise read `/config/ENGINEERING.md` as the User Policy Set. Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit closes the OAW Engagement.\n"
	want := "<!-- BEGIN OPEN AGENT WORKFLOW -->\n" +
		router +
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
