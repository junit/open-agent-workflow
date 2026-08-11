package codexbridge

import (
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestAssembleFactsBuildsPinnedCurrentHostRecords(t *testing.T) {
	snapshot, report, inventory, resolution := emptyFactInputs(t)
	context := HookContext{SessionID: "session-codex-1", CWD: "/repo"}
	metadata := completeFactMetadata()
	facts, err := AssembleFacts(context, metadata, snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if facts.Session.IntegrationID != BridgeIntegrationID || facts.Session.IntegrationVersion != BridgeIntegrationVersion ||
		!slices.Equal(facts.Session.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
		facts.Session.SchemaVersion != host.HostSessionSchemaV3 || facts.Session.SandboxPolicyDigest != "" || facts.Session.ApprovalPolicyDigest != "" ||
		len(facts.Session.FeatureObservations) != 0 || len(facts.Session.HostActionObservations) != 0 ||
		facts.Session.FeatureDigest == "" || facts.Session.HostActionDigest == "" {
		t.Fatalf("session=%#v", facts.Session)
	}
	want := map[string]execution.EnvironmentDisposition{
		"approvals": execution.DispositionHostConfigured,
		"hooks":     execution.DispositionHostConfigured,
		"mcp":       execution.DispositionHostConfigured,
		"sandbox":   execution.DispositionHostConfigured,
		"skills":    execution.DispositionInherited,
	}
	if len(facts.Environment.Observations) != len(want) {
		t.Fatalf("environment=%#v", facts.Environment)
	}
	for _, observation := range facts.Environment.Observations {
		if want[observation.Surface] != observation.Disposition || observation.Source != "codex-app-server" || len(observation.Digest) != 64 {
			t.Fatalf("observation=%#v", observation)
		}
	}
	if err := validateFacts(facts); err != nil {
		t.Fatalf("assembled facts are invalid: %v", err)
	}
	if facts.FactDigests.Session != facts.Session.Digest || facts.FactDigests.Inventory != facts.Inventory.Digest ||
		facts.FactDigests.Environment != facts.Environment.Digest || facts.FactDigests.Configuration != snapshot.Digest() ||
		facts.FactDigests.Discovery != report.Digest() || facts.FactDigests.Resolution != resolution.Report.Digest() ||
		facts.FactDigests.Registry != resolution.Registry.Digest() || facts.FactDigests.Features != facts.Session.FeatureDigest ||
		facts.FactDigests.Actions != facts.Session.HostActionDigest || facts.FactDigests.Version != facts.VersionEvidence.Digest {
		t.Fatalf("fact digests=%#v", facts.FactDigests)
	}
	if facts.VersionEvidence.PluginVersion != "1.2.3" || facts.VersionEvidence.HostConformanceTranscriptSchema != host.HostConformanceTranscriptSchemaV4 || facts.VersionEvidence.HostConformanceReportSchema != host.HostConformanceReportSchemaV4 {
		t.Fatalf("VersionEvidence=%#v", facts.VersionEvidence)
	}
}

func TestAssembleFactsKeepsMissingOptionalSurfacesUnknown(t *testing.T) {
	snapshot, report, inventory, resolution := emptyFactInputs(t)
	metadata := completeFactMetadata()
	metadata.Methods = []string{"skills/list"}
	metadata.Config.CWDObserved = false
	facts, err := AssembleFacts(HookContext{SessionID: "session-codex-1", CWD: "/repo"}, metadata, snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range facts.Environment.Observations {
		if observation.Surface == "skills" {
			if observation.Disposition != execution.DispositionInherited {
				t.Fatalf("skills=%#v", observation)
			}
			continue
		}
		if observation.Disposition != execution.DispositionUnknown {
			t.Fatalf("optional observation guessed availability: %#v", observation)
		}
	}
}

func TestAssembleFactsChangesEnvironmentWhenHookEvidenceChanges(t *testing.T) {
	snapshot, report, inventory, resolution := emptyFactInputs(t)
	context := HookContext{SessionID: "session-codex-1", CWD: "/repo"}
	metadata := completeFactMetadata()
	metadata.Hooks.Hooks = []appserver.HookMetadata{{CurrentHash: "first", Enabled: true, EventName: "preToolUse", PluginID: "oaw-codex-host", Source: "plugin", TrustStatus: "trusted"}}
	first, err := AssembleFacts(context, metadata, snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	metadata.Hooks.Hooks[0].CurrentHash = "second"
	second, err := AssembleFacts(context, metadata, snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if first.Environment.Digest == second.Environment.Digest {
		t.Fatal("Hook evidence change did not change the Environment Report digest")
	}
}

func TestEnvironmentEvidenceDigestsAreSurfaceSpecific(t *testing.T) {
	snapshot, report, inventory, resolution := emptyFactInputs(t)
	context := HookContext{SessionID: "session-codex-1", CWD: "/repo"}
	metadata := completeFactMetadata()
	first, err := AssembleFacts(context, metadata, snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	metadata.Config.SandboxDisposition = "unknown"
	second, err := AssembleFacts(context, metadata, snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	firstDigests := environmentDigests(first.Environment)
	secondDigests := environmentDigests(second.Environment)
	for surface := range firstDigests {
		changed := firstDigests[surface] != secondDigests[surface]
		if changed != (surface == "sandbox") {
			t.Fatalf("surface %q changed=%t", surface, changed)
		}
	}
}

func TestAssembleFactsRejectsNonCanonicalInventoryAndMetadata(t *testing.T) {
	snapshot, report, inventory, resolution := emptyFactInputs(t)
	invalid := inventory
	invalid.Digest = strings.Repeat("f", 64)
	if _, err := AssembleFacts(HookContext{SessionID: "session-codex-1", CWD: "/repo"}, completeFactMetadata(), snapshot, report, invalid, resolution, "1.2.3"); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("inventory error=%v", err)
	}
	metadata := completeFactMetadata()
	metadata.Methods = []string{"config/read", "skills/list", "skills/list"}
	if _, err := AssembleFacts(HookContext{SessionID: "session-codex-1", CWD: "/repo"}, metadata, snapshot, report, inventory, resolution, "1.2.3"); Code(err) != "HOST_BRIDGE_PROTOCOL_MISMATCH" {
		t.Fatalf("metadata error=%v", err)
	}
	resolution.Digest = strings.Repeat("e", 64)
	if _, err := AssembleFacts(HookContext{SessionID: "session-codex-1", CWD: "/repo"}, completeFactMetadata(), snapshot, report, inventory, resolution, "1.2.3"); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("resolution error=%v", err)
	}
}

func TestAssembleFactsDoesNotShareInventoryStorage(t *testing.T) {
	snapshot, report, _, _ := emptyFactInputs(t)
	inventory, err := host.BuildBindingInventoryV3("codex", []host.BindingObservation{{
		HostID: "codex", ProviderID: "acme/suite", InstallationKey: "installation-test", DistributionID: "acme", BindingID: "codex-review",
		Surface: "codex-plugin", Kind: catalog.BindingSkill, Reference: "acme:review", Invocation: catalog.InvocationModel,
		BindingTreeDigest: "sha256:" + strings.Repeat("a", 64), Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceNativeAPI,
		EvidenceReference: "evidence://codex/skills-list/fixture",
	}})
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: "codex", Discovery: report, Inventory: &inventory})
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := inventory.Observations[0].Digest
	facts, err := AssembleFacts(HookContext{SessionID: "session-codex-1", CWD: "/repo"}, completeFactMetadata(), snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	inventory.Observations[0].Digest = strings.Repeat("b", 64)
	if facts.Inventory.Observations[0].Digest != originalDigest {
		t.Fatal("assembled Facts share Inventory storage with the caller")
	}
}

func TestCodexHostManifestDeclaresOnlyCurrentSkillSurface(t *testing.T) {
	manifest, err := CodexHostManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.HostID != "codex" || manifest.ControlSurface != host.SurfaceHostNative ||
		!slices.Equal(manifest.BindingKinds, []catalog.BindingKind{catalog.BindingSkill}) || !slices.Equal(manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("manifest=%#v", manifest)
	}
}

func TestValidateFactsRejectsAlternateCodexIntegration(t *testing.T) {
	snapshot, report, inventory, resolution := emptyFactInputs(t)
	facts, err := AssembleFacts(HookContext{SessionID: "session-codex-1", CWD: "/repo"}, completeFactMetadata(), snapshot, report, inventory, resolution, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := CodexHostManifest()
	if err != nil {
		t.Fatal(err)
	}
	facts.Session, err = host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "alternate/codex-host",
		IntegrationVersion: BridgeIntegrationVersion, SessionID: facts.Session.SessionID,
		ManifestDigest:          manifest.Digest,
		SupportedTopologies:     []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: facts.Inventory.Digest, EnvironmentReportDigest: facts.Environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	facts.FactDigests.Session = facts.Session.Digest
	if err := validateFacts(facts); Code(err) != "HOST_EVIDENCE_HANDLE_INVALID" {
		t.Fatalf("error=%v", err)
	}
}

func completeFactMetadata() appserver.MetadataObservation {
	return appserver.MetadataObservation{
		Skills:  appserver.SkillsEntry{CWD: "/repo", Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{}},
		Hooks:   appserver.HooksEntry{CWD: "/repo", Errors: []appserver.MetadataError{}, Warnings: []string{}, Hooks: []appserver.HookMetadata{}},
		Config:  appserver.ConfigProjection{CWDObserved: true, SandboxDisposition: "host-configured", MCPDisposition: "host-configured", HookDisposition: "host-configured", ApprovalDisposition: "host-configured"},
		Methods: []string{"config/read", "hooks/list", "skills/list"}, CodexVersion: "codex-cli/0.146.1",
	}
}

func environmentDigests(report host.EnvironmentReport) map[string]string {
	result := make(map[string]string, len(report.Observations))
	for _, observation := range report.Observations {
		result[observation.Surface] = observation.Digest
	}
	return result
}

func emptyFactInputs(t *testing.T) (config.Snapshot, discovery.Report, host.BindingInventory, core.ResolutionResult) {
	t.Helper()
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: "codex", UserHome: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := host.BuildBindingInventoryV3("codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: "codex", Discovery: report, Inventory: &inventory})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, report, inventory, resolution
}
