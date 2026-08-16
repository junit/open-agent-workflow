package management

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

func TestRenderTargetMatchesAdapterContract(t *testing.T) {
	policyPath := "/config path/`policy`/POLICY.md"
	policyReference := markdownCodeSpan(policyPath)
	prefix := "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. "
	suffix := " Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit ends OAW governance for that deliverable.\n"
	userRouter := prefix + "On explicit activation, if the current project contains `.oaw/policy/POLICY.md`, read that Project Policy Set and do not read or merge the User Policy Set; otherwise read " + policyReference + " as the User Policy Set." + suffix
	projectRouter := prefix + "On explicit activation, read " + policyReference + " as the Project Policy Set and do not read or merge the User Policy Set." + suffix
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
		{name: "project cursor", scope: "project", id: "cursor", want: "---\ndescription: Open Agent Workflow activation router\nglobs: \"**/*\"\nalwaysApply: true\n---\n\n" + projectRouter},
		{name: "project windsurf", scope: "project", id: "windsurf", want: "---\ntrigger: always_on\n---\n\n" + projectRouter},
		{name: "project cline", scope: "project", id: "cline", want: projectRouter},
		{name: "project roo", scope: "project", id: "roo", want: projectRouter},
		{name: "project copilot", scope: "project", id: "copilot", want: "---\napplyTo: \"**\"\n---\n\n" + projectRouter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderArtifact(tt.id, routerArtifactID, tt.scope, policyPath)
			if err != nil {
				t.Fatalf("renderArtifact() error = %v", err)
			}
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Fatalf("renderArtifact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTargetEnforcesActivationRouterContract(t *testing.T) {
	policyPath := "/config/POLICY.md"
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
			rendered, err := renderArtifact(target.id, routerArtifactID, target.scope, policyPath)
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
				"explicit exit ends OAW governance for that deliverable",
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

func TestRenderNativeArtifactsAreThinExplicitDispatchers(t *testing.T) {
	tests := []struct {
		name       string
		scope      scope
		id         targetID
		artifact   string
		source     string
		required   []string
		forbidden  []string
		policyPath string
	}{
		{name: "user Claude Skill", scope: "user", id: "claude", artifact: nativeEntrypointArtifactID, policyPath: "/config/open-agent-workflow/POLICY.md", source: "Claude manual-only Skill selection", required: []string{"name: oaw", "user-invocable: true", "disable-model-invocation: true", "$ARGUMENTS"}},
		{name: "user Codex Skill", scope: "user", id: "codex", artifact: nativeEntrypointArtifactID, policyPath: "/config/open-agent-workflow/POLICY.md", source: "Codex explicit Skill selection", required: []string{"name: oaw", "the remainder of this user request"}},
		{name: "Codex native policy", scope: "project", id: "codex", artifact: nativePolicyArtifactID, policyPath: ".oaw/policy/POLICY.md", required: []string{"display_name: \"Open Agent Workflow\"", "allow_implicit_invocation: false"}, forbidden: []string{"MATT-SP-HYBRID", "Spec", "TDD"}},
		{name: "user Gemini command", scope: "user", id: "gemini", artifact: nativeEntrypointArtifactID, policyPath: "/config/open-agent-workflow/POLICY.md", source: "Gemini user-command event", required: []string{"prompt = \"", "{{args}}"}, forbidden: []string{"prompt = \"\"\""}},
		{name: "user OpenCode command", scope: "user", id: "opencode", artifact: nativeEntrypointArtifactID, policyPath: "/config/open-agent-workflow/POLICY.md", source: "OpenCode user-command event", required: []string{"$ARGUMENTS"}, forbidden: []string{"argument-hint:"}},
		{name: "project Cursor Skill", scope: "project", id: "cursor", artifact: nativeEntrypointArtifactID, policyPath: ".oaw/policy/POLICY.md", source: "Cursor manual-only Skill selection", required: []string{"name: oaw", "disable-model-invocation: true", "the remainder of this user request"}},
		{name: "project Windsurf workflow", scope: "project", id: "windsurf", artifact: nativeEntrypointArtifactID, policyPath: ".oaw/policy/POLICY.md", source: "Windsurf user-Workflow event", required: []string{"the remainder of this user request"}},
		{name: "project Cline Skill", scope: "project", id: "cline", artifact: nativeEntrypointArtifactID, policyPath: ".oaw/policy/POLICY.md", source: "original pre-expansion Cline user input", required: []string{"name: oaw", "the remainder of this user request"}},
		{name: "project Roo command", scope: "project", id: "roo", artifact: nativeEntrypointArtifactID, policyPath: ".oaw/policy/POLICY.md", source: "original pre-expansion Roo user input", required: []string{"argument-hint:", "the remainder of this user request"}},
		{name: "project Copilot Skill", scope: "project", id: "copilot", artifact: nativeEntrypointArtifactID, policyPath: ".oaw/policy/POLICY.md", source: "Copilot manual-only Agent Skill selection", required: []string{"name: oaw", "argument-hint:", "disable-model-invocation: true", "the remainder of this user request"}, forbidden: []string{"user-invocable:"}},
	}
	commonRequired := []string{
		"evidence must come from outside this dispatcher's bytes and any Host-expanded template text",
		"name, description, body, argument hint, and expanded text are never activation evidence",
		"quoted or discussed invocation text",
		"automatic discovery or matching",
		"model-led invocation or loading",
		"user provenance is unavailable or ambiguous",
		"do not activate OAW and continue as the native Host",
		"Follow the current OAW Activation Router to select and read one Policy Set",
		"Do not embed or infer a Policy path here",
		"Pass the optional Profile and task",
	}
	commonForbidden := []string{
		"MATT-SP-HYBRID", "SP-FULL", "MATT-FULL", "ECC-FULL",
		"/oaw", "$oaw", "literal native form", "natural-language activation request",
		"Explicitly activate", "explicitly activate",
		"Spec -> Plan", "Spec → Plan", "TDD ->", "must wait for approval",
		"without an explicit `/oaw`, `$oaw`, or natural-language request",
		"An explicit native invocation or user selection",
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := renderArtifact(tt.id, tt.artifact, tt.scope, tt.policyPath)
			if err != nil {
				t.Fatal(err)
			}
			text := string(rendered)
			required := tt.required
			forbidden := tt.forbidden
			if tt.artifact != nativePolicyArtifactID {
				required = append(append([]string(nil), commonRequired...), required...)
				required = append(required, tt.source)
				forbidden = append(append([]string(nil), commonForbidden...), forbidden...)
				forbidden = append(forbidden, tt.policyPath)
			}
			for _, fragment := range required {
				if !strings.Contains(text, fragment) {
					t.Fatalf("rendered %s/%s omits %q: %q", tt.id, tt.artifact, fragment, text)
				}
			}
			for _, fragment := range forbidden {
				if strings.Contains(text, fragment) {
					t.Fatalf("rendered %s/%s contains forbidden %q: %q", tt.id, tt.artifact, fragment, text)
				}
			}
		})
	}
}

func TestRenderRouterQuotesSpecialPolicyPaths(t *testing.T) {
	policyPath := "/config/with`ticks/and\"\"\"quotes/POLICY.md"
	wantReference := markdownCodeSpan(policyPath)

	markdown, err := renderArtifact("claude", routerArtifactID, "user", policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdown), wantReference) {
		t.Fatalf("Activation Router does not preserve quoted policy path: %q", markdown)
	}
}

func TestRenderNativeArtifactsDoNotEmbedHostPreprocessedPolicyPaths(t *testing.T) {
	tests := []struct {
		name      string
		id        targetID
		path      string
		forbidden []string
	}{
		{name: "Claude", id: "claude", path: "/config/$1/${CLAUDE_SESSION_ID}/!`policy-command`/POLICY.md", forbidden: []string{"$1", "${CLAUDE_SESSION_ID}", "!`policy-command`"}},
		{name: "OpenCode", id: "opencode", path: "/config/$1/@policy-file/!`policy-command`/POLICY.md", forbidden: []string{"$1", "@policy-file", "!`policy-command`"}},
		{name: "Gemini", id: "gemini", path: "/config/@{policy-file}/!{policy-command}/{{args}}/POLICY.md", forbidden: []string{"@{policy-file}", "!{policy-command}"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := renderArtifact(tt.id, nativeEntrypointArtifactID, "user", tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(rendered), tt.path) {
				t.Fatalf("native dispatcher embeds Policy path: %q", rendered)
			}
			for _, fragment := range tt.forbidden {
				if strings.Contains(string(rendered), fragment) {
					t.Fatalf("native dispatcher contains Host-preprocessed path fragment %q: %q", fragment, rendered)
				}
			}
		})
	}

	geminiPath := "/config/@{policy-file}/!{policy-command}/{{args}}/POLICY.md"
	gemini, err := renderArtifact("gemini", nativeEntrypointArtifactID, "user", geminiPath)
	if err != nil {
		t.Fatal(err)
	}
	_, encodedPrompt, found := strings.Cut(string(gemini), "prompt = ")
	if !found {
		t.Fatalf("Gemini command has no prompt: %q", gemini)
	}
	decodedPrompt, err := strconv.Unquote(strings.TrimSpace(encodedPrompt))
	if err != nil {
		t.Fatalf("Gemini prompt is not a quoted TOML-compatible basic string: %v: %q", err, gemini)
	}
	if strings.Contains(decodedPrompt, geminiPath) || !strings.Contains(decodedPrompt, "{{args}}") {
		t.Fatalf("Gemini prompt lost dispatcher content: %q", decodedPrompt)
	}
}

func TestRenderArtifactRejectsUnknownAndUnsupportedPairs(t *testing.T) {
	for name, test := range map[string]struct {
		id       targetID
		artifact string
		scope    scope
		policy   string
	}{
		"unknown target":   {id: "missing", artifact: routerArtifactID, scope: "project", policy: ".oaw/policy/POLICY.md"},
		"unknown artifact": {id: "claude", artifact: "missing", scope: "user", policy: "/config/POLICY.md"},
		"unsupported user": {id: "cursor", artifact: nativeEntrypointArtifactID, scope: "user", policy: "/config/POLICY.md"},
		"empty policy":     {id: "claude", artifact: nativeEntrypointArtifactID, scope: "user"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := renderArtifact(test.id, test.artifact, test.scope, test.policy); err == nil {
				t.Fatalf("renderArtifact(%q, %q, %q) succeeded", test.id, test.artifact, test.scope)
			}
		})
	}
}

func TestRenderManagedBlockWrapsExactRendererBytes(t *testing.T) {
	got, err := renderManagedBlock("codex", "user", "/config/POLICY.md")
	if err != nil {
		t.Fatal(err)
	}
	router := "Open Agent Workflow is opt-in. Unless the current top-level user request explicitly asks to use OAW, or clearly continues an active OAW task, behave as the native Host: do not read the OAW Policy, classify the request, inspect OAW Providers, mention OAW, create OAW state, or change normal Skill, Agent, role, instruction, or tool selection. Installing OAW, discussing or quoting OAW, task complexity, and ordinary Skill invocation do not activate OAW. On explicit activation, if the current project contains `.oaw/policy/POLICY.md`, read that Project Policy Set and do not read or merge the User Policy Set; otherwise read `/config/POLICY.md` as the User Policy Set. Apply the selected Policy Set only to that deliverable. Related follow-ups inherit activation; unrelated requests remain native. Completion, cancellation, or explicit exit ends OAW governance for that deliverable.\n"
	want := "<!-- BEGIN OPEN AGENT WORKFLOW -->\n" +
		router +
		"<!-- END OPEN AGENT WORKFLOW -->\n"
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("renderManagedBlock() = %q, want %q", got, want)
	}
}

func TestRenderManagedFilePreservesPlacementRules(t *testing.T) {
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

func TestRenderRouterRejectsUnsupportedPair(t *testing.T) {
	if _, err := renderArtifact("cursor", routerArtifactID, "user", "/config/POLICY.md"); err == nil {
		t.Fatal("renderArtifact() accepted user cursor Router")
	}
}
