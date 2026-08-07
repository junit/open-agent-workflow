package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/classification"
	"github.com/wifibaby4u/open-agent-workflow/internal/cli"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/coordinator"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func TestCodexBridgeCurrentWorkflowTranscript(t *testing.T) {
	fixture := newCodexBridgeFixture(t)
	observed := callObserveThroughMCP(t, fixture)
	inspection := callInspectThroughMCP(t, fixture, observed.HostEvidenceHandle)
	if inspection.Compilation == nil || len(inspection.Compilation.EligibleProfiles) == 0 || inspection.HostSummary.SessionDigest == "" {
		t.Fatalf("inspection=%#v", inspection)
	}
	compiled := callCompileThroughMCP(t, fixture, observed.HostEvidenceHandle, explicitCurrentSelection(t, inspection))
	started := callWorkflowStartThroughMCP(t, fixture, observed.HostEvidenceHandle, compiled)
	if started.Snapshot == nil || len(started.Snapshot.Bundles) != 1 ||
		started.Snapshot.Bundles[0].HostSessionDigest[:16] != inspection.HostSummary.SessionDigest {
		t.Fatalf("started=%#v", started)
	}
}

func TestDirectPathDoesNotRequireBridge(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := cli.RunWithInput([]string{"catalog", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), "HOST_BRIDGE") {
		t.Fatalf("direct path unexpectedly consulted Bridge: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

type codexBridgeFixture struct {
	client        *mcp.ClientSession
	hostContext   codexbridge.HookContext
	proposal      classification.ClassificationProposal
	deliverableID string
	inputDigest   string
}

type codexBridgeObserver struct {
	metadata appserver.MetadataObservation
}

func (observer codexBridgeObserver) Observe(_ context.Context, cwd string) (appserver.MetadataObservation, error) {
	value := observer.metadata
	value.Skills.CWD = cwd
	value.Skills.Errors = append([]appserver.MetadataError{}, value.Skills.Errors...)
	value.Skills.Skills = append([]appserver.SkillMetadata{}, value.Skills.Skills...)
	value.Hooks.CWD = cwd
	value.Hooks.Errors = append([]appserver.MetadataError{}, value.Hooks.Errors...)
	value.Hooks.Warnings = append([]string{}, value.Hooks.Warnings...)
	value.Hooks.Hooks = append([]appserver.HookMetadata{}, value.Hooks.Hooks...)
	value.Methods = append([]string{}, value.Methods...)
	value.Diagnostics = append([]appserver.ObservationDiagnostic{}, value.Diagnostics...)
	return value, nil
}

func newCodexBridgeFixture(t *testing.T) codexBridgeFixture {
	t.Helper()
	projectRoot := t.TempDir()
	userHome := t.TempDir()
	userConfigRoot := t.TempDir()
	metadata := appserver.MetadataObservation{
		Skills: appserver.SkillsEntry{Errors: []appserver.MetadataError{}, Skills: installSuperpowersSkillFixture(t, userHome)},
		Hooks:  appserver.HooksEntry{Errors: []appserver.MetadataError{}, Warnings: []string{}, Hooks: []appserver.HookMetadata{}},
		Config: appserver.ConfigProjection{
			CWDObserved: true, SandboxDisposition: "host-configured", MCPDisposition: "host-configured",
			HookDisposition: "host-configured", ApprovalDisposition: "host-configured",
		},
		Methods: []string{"config/read", "hooks/list", "skills/list"}, CodexVersion: "codex-cli/0.146.1",
	}
	hostContext := codexbridge.HookContext{
		SchemaVersion: codexbridge.HookContextSchemaV1, BridgeProtocolVersion: codexbridge.BridgeProtocolVersion,
		SessionID: "bridge-blackbox-session", TurnID: "turn-1", ToolUseID: "tool-1", CWD: projectRoot,
		Model: "gpt-test", PermissionMode: "workspace-write",
	}
	observer := codexBridgeObserver{metadata: metadata}
	service, err := codexbridge.NewService(codexbridge.ServiceOptions{
		Observer: observer, Store: codexbridge.NewEvidenceStore(codexbridge.CacheOptions{MaximumEntries: 8}),
		StateRoot: t.TempDir(), ProjectRoot: projectRoot, UserConfigRoot: userConfigRoot,
		UserHome: userHome, BridgeVersion: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return codexBridgeFixture{
		client: connectCodexBridgeMCP(t, service), hostContext: hostContext,
		proposal:      classification.ClassificationProposal{SchemaVersion: classification.ProposalSchemaV1},
		deliverableID: "codex-bridge-blackbox", inputDigest: canonicaljson.DigestBytes([]byte("codex-bridge-blackbox-input")),
	}
}

func installSuperpowersSkillFixture(t *testing.T, userHome string) []appserver.SkillMetadata {
	t.Helper()
	root := filepath.Join(userHome, ".codex", "plugins", "superpowers")
	writeCodexBridgeFixture(t, filepath.Join(root, "skills", "using-superpowers", "SKILL.md"), "provider evidence")
	names := []string{
		"superpowers:brainstorming",
		"superpowers:writing-plans",
		"superpowers:using-git-worktrees",
		"superpowers:subagent-driven-development",
		"superpowers:test-driven-development",
		"superpowers:systematic-debugging",
		"superpowers:requesting-code-review",
		"superpowers:verification-before-completion",
		"superpowers:finishing-a-development-branch",
	}
	result := make([]appserver.SkillMetadata, 0, len(names))
	for _, name := range names {
		path := filepath.Join(root, "observed-skills", strings.ReplaceAll(name, ":", "_"), "SKILL.md")
		writeCodexBridgeFixture(t, path, "---\nname: "+name+"\n---\n")
		result = append(result, appserver.SkillMetadata{Name: name, Enabled: true, Path: path, Scope: "user"})
	}
	return result
}

func writeCodexBridgeFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func connectCodexBridgeMCP(t *testing.T, service *codexbridge.Service) *mcp.ClientSession {
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

func callObserveThroughMCP(t *testing.T, fixture codexBridgeFixture) codexbridge.ObserveCurrentOutput {
	t.Helper()
	return callCodexBridgeTool[codexbridge.ObserveCurrentOutput](t, fixture.client, "observe_current", map[string]any{
		"_oaw_host_context": fixture.hostContext,
	})
}

func callInspectThroughMCP(t *testing.T, fixture codexBridgeFixture, handle codexbridge.HostEvidenceHandle) codexbridge.CoreInspectOutput {
	t.Helper()
	return callCodexBridgeTool[codexbridge.CoreInspectOutput](t, fixture.client, "core_inspect", codexbridge.CoreInspectInput{
		HostEvidenceHandle: handle, DeliverableID: fixture.deliverableID,
		InputDigest: fixture.inputDigest, Proposal: fixture.proposal,
	})
}

func callCompileThroughMCP(
	t *testing.T,
	fixture codexBridgeFixture,
	handle codexbridge.HostEvidenceHandle,
	selection core.Selection,
) core.CompilationResult {
	t.Helper()
	result := callCodexBridgeTool[core.CompilationResult](t, fixture.client, "core_compile", codexbridge.CoreCompileInput{
		HostEvidenceHandle: handle, DeliverableID: fixture.deliverableID,
		InputDigest: fixture.inputDigest, Proposal: fixture.proposal, Selection: selection,
	})
	if result.Bundle == nil {
		t.Fatalf("compiled=%#v", result)
	}
	return result
}

func explicitCurrentSelection(t *testing.T, inspection codexbridge.CoreInspectOutput) core.Selection {
	t.Helper()
	for _, candidate := range inspection.Compilation.EligibleProfiles {
		if candidate.Profile == "SP-FULL" && candidate.Eligible && slices.Contains(candidate.EligibleTopologies, execution.TopologyCurrent) {
			return core.Selection{
				Profile: "SP-FULL", ProfileSource: core.SelectionUser, Topology: execution.TopologyCurrent,
				TopologySource: core.SelectionHostOnlyOption, AddOns: []string{},
			}
		}
	}
	t.Fatalf("SP-FULL CURRENT unavailable: %#v", inspection.Compilation.EligibleProfiles)
	return core.Selection{}
}

func callWorkflowStartThroughMCP(
	t *testing.T,
	fixture codexBridgeFixture,
	handle codexbridge.HostEvidenceHandle,
	compiled core.CompilationResult,
) coordinator.Result {
	t.Helper()
	command := codexbridge.WorkflowCommandInput{
		SchemaVersion: coordinator.WorkflowCommandSchemaV1, Kind: coordinator.CommandStart,
		MessageID: "message-start", IdempotencyKey: "codex-bridge-blackbox-start",
		Start: &codexbridge.WorkflowStartInput{
			RequestID: "request-codex-bridge-blackbox", DeliverableID: fixture.deliverableID,
			InputDigest: fixture.inputDigest, Proposal: fixture.proposal, Selection: compiled.Bundle.Selection,
		},
	}
	result := callCodexBridgeTool[coordinator.Result](t, fixture.client, "workflow_exchange", codexbridge.WorkflowExchangeInput{
		HostEvidenceHandle: handle, Command: command,
	})
	if result.Snapshot == nil || len(result.Snapshot.Bundles) != 1 || result.Snapshot.Bundles[0].Digest != compiled.Bundle.Digest {
		t.Fatalf("compiled=%#v started=%#v", compiled.Bundle, result)
	}
	return result
}

func callCodexBridgeTool[T any](t *testing.T, client *mcp.ClientSession, name string, arguments any) T {
	t.Helper()
	result, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil || result.IsError {
		t.Fatalf("%s result=%#v err=%v", name, result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output T
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	return output
}
