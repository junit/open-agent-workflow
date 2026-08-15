package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/hook"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
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
	providerCatalog, userHome, skillPath := assuranceBridgeCatalog(t)
	projectRoot := t.TempDir()
	configHome := t.TempDir()
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
		Catalog: &providerCatalog, ProfileConfigHome: configHome, UserHome: userHome,
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
		WorkingDir: projectRoot, Home: userHome, ConfigHome: configHome,
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

func assuranceBridgeCatalog(t *testing.T) (catalog.Catalog, string, string) {
	t.Helper()
	userHome := t.TempDir()
	providerRoot := filepath.Join(userHome, ".codex", "plugins", "acme")
	skillRoot := filepath.Join(providerRoot, "skills", "review")
	skillPath := filepath.Join(skillRoot, "SKILL.md")
	writeAssuranceBridgeFixture(t, skillPath, "---\nname: acme:review\n---\n")
	writeAssuranceBridgeFixture(t, filepath.Join(providerRoot, "marker.txt"), "acme\n")
	bindingTree, err := integrity.DigestTree(skillRoot)
	if err != nil {
		t.Fatal(err)
	}
	distributionTree, err := integrity.DigestTree(providerRoot)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV5, DescriptorVersion: "5.0.0",
		ID: "acme/suite", DisplayName: "Acme Suite",
		Distributions: []catalog.DistributionRecord{{
			ID: "acme", SourceURI: "https://example.test/acme/suite",
			Revision: strings.Repeat("a", 40), TreeDigest: distributionTree.RootDigest,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "acme",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/plugins/acme", EvidencePath: "marker.txt",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-review", DistributionID: "acme", ContentRoot: "skills/review", InstallRoot: "skills/review",
			TreeDigest: bindingTree.RootDigest, Host: "codex", Surface: "codex-plugin",
			Kind: catalog.BindingSkill, Reference: "acme:review", Invocation: catalog.InvocationModel,
		}},
	}
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor})
	if err != nil {
		t.Fatal(err)
	}
	return value, userHome, skillPath
}
