package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/hook"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
)

type assuranceBridgeObserver struct {
	metadata appserver.MetadataObservation
	err      error
}

func (observer assuranceBridgeObserver) Observe(_ context.Context, cwd string) (appserver.MetadataObservation, error) {
	value := observer.metadata
	value.Skills.CWD = cwd
	value.Skills.Errors = append([]appserver.MetadataError(nil), observer.metadata.Skills.Errors...)
	value.Skills.Skills = append([]appserver.SkillMetadata(nil), observer.metadata.Skills.Skills...)
	return value, observer.err
}

func TestStandaloneCodexBridgeIssuesProfileBoundOverlayThroughHookAndMCP(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	projectRoot := t.TempDir()
	configHome := t.TempDir()
	configRoot := filepath.Join(configHome, "open-agent-workflow")
	profilePath := filepath.Join(projectRoot, ".oaw", "profiles", "team-review.md")
	writeAssuranceBridgeFixture(t, profilePath,
		"---\nid: team-review\nname: Team Review\n---\n\n"+
			"## Responsibilities\n\n"+
			"| Responsibility | Skill or action |\n| --- | --- |\n"+
			"| Review and remediation | `acme:review` |\n\n"+
			"## Rules\n\nThe Markdown Profile remains normative.\n")
	before, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}

	descriptor := fixture.Catalog.Providers()[0]
	descriptorRaw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	writeAssuranceBridgeFixture(t, filepath.Join(configRoot, "providers", "acme.json"), string(descriptorRaw))
	writeAssuranceBridgeFixture(t, filepath.Join(configRoot, "config.toml"),
		"schema_version = \"oaw.user-config/v3\"\n[[provider_descriptors]]\nid = \"acme/suite\"\npath = \"providers/acme.json\"\n")

	skillPath := filepath.Join(fixture.Candidate.DiagnosticLocation, "skills", "review", "SKILL.md")
	service, err := codexbridge.NewService(codexbridge.ServiceOptions{
		Observer: assuranceBridgeObserver{metadata: appserver.MetadataObservation{
			Skills: appserver.SkillsEntry{
				Errors: []appserver.MetadataError{},
				Skills: []appserver.SkillMetadata{{
					Name: "acme:review", Enabled: true, Path: skillPath, Scope: "user",
				}},
			},
			CodexVersion: "codex-cli/test",
		}},
		UserConfigRoot: configRoot, ProfileConfigHome: configHome, UserHome: fixture.Home,
	})
	if err != nil {
		t.Fatal(err)
	}

	arguments := bridgeArgumentsThroughHook(t, projectRoot, "project:team-review")
	client := connectAssuranceBridgeMCP(t, service)
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "observe_profile", Arguments: arguments})
	if err != nil || result.IsError {
		t.Fatalf("observe_profile result=%#v error=%v", result, err)
	}
	resultRaw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output codexbridge.ObserveProfileOutput
	if err := json.Unmarshal(resultRaw, &output); err != nil {
		t.Fatal(err)
	}
	if output.Overlay.Profile.ID != "team-review" || output.Overlay.Issuer != codexbridge.BridgeIntegrationID || len(output.Overlay.Claims) != 1 {
		t.Fatalf("Overlay = %#v", output.Overlay)
	}

	profiles, err := profileinspect.Discover(profileinspect.Environment{
		WorkingDir: projectRoot, Home: fixture.Home, ConfigHome: configHome,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := profileinspect.Resolve(profiles, "project:team-review")
	if err != nil {
		t.Fatal(err)
	}
	if err := assurance.Verify(profile, output.Overlay); err != nil {
		t.Fatalf("issued Overlay does not verify: %v", err)
	}
	after, err := os.ReadFile(profilePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("Bridge modified the Markdown Profile: %v", err)
	}
}

func bridgeArgumentsThroughHook(t *testing.T, projectRoot, selector string) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"session_id": "bridge-blackbox-session", "transcript_path": nil,
		"turn_id": "turn-1", "tool_use_id": "tool-1", "cwd": projectRoot,
		"hook_event_name": "PreToolUse", "model": "gpt-test", "permission_mode": "workspace-write",
		"tool_name": "mcp__oaw_codex_bridge__observe_profile", "tool_input": map[string]any{"profile": selector},
	})
	if err != nil {
		t.Fatal(err)
	}
	output, err := hook.ProcessPreToolUse(raw)
	if err != nil || output.HookSpecificOutput == nil || output.HookSpecificOutput.PermissionDecision != "allow" {
		t.Fatalf("Hook output=%#v error=%v", output, err)
	}
	projected, err := json.Marshal(output.HookSpecificOutput.UpdatedInput)
	if err != nil {
		t.Fatal(err)
	}
	var arguments map[string]any
	if err := json.Unmarshal(projected, &arguments); err != nil {
		t.Fatal(err)
	}
	return arguments
}

func connectAssuranceBridgeMCP(t *testing.T, service *codexbridge.Service) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- codexbridge.NewMCPServer(service, "test").Run(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "bridge-blackbox-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = session.Close()
		cancel()
		<-done
	})
	return session
}

func writeAssuranceBridgeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
