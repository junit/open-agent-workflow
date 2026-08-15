package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

func TestCodexBridgeUsesOneBindingAlgorithmForEveryProvider(t *testing.T) {
	cases := []struct {
		provider string
		skill    string
	}{
		{provider: "oaw/superpowers", skill: "superpowers:writing-plans"},
		{provider: "oaw/matt", skill: "tdd"},
		{provider: "oaw/ecc", skill: "tdd-workflow"},
		{provider: "acme/custom", skill: "acme:delivery"},
	}
	for _, test := range cases {
		t.Run(test.provider, func(t *testing.T) {
			assertExactSkillBinding(t, test.provider, test.skill)
		})
	}
}

func TestDefaultCodexPolicyDoesNotDependOnBridgeAuthority(t *testing.T) {
	snapshot, err := config.Load(config.LoadOptions{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := snapshot.HostIntegration(codexbridge.BridgeIntegrationID); found {
		t.Fatal("standalone Assurance Bridge appeared in the default oaw configuration")
	}
	for _, integration := range snapshot.HostIntegrations() {
		if integration.Manifest.ControlSurface != host.SurfacePolicy {
			t.Fatalf("default Integration %q claims machine authority", integration.ID)
		}
	}
}

func assertExactSkillBinding(t *testing.T, providerID, skill string) {
	t.Helper()
	home := t.TempDir()
	providerRoot := filepath.Join(home, ".codex", "plugins", "provider")
	skillRoot := filepath.Join(providerRoot, "skills", "observed")
	skillPath := filepath.Join(skillRoot, "SKILL.md")
	writeBridgeFixtureFile(t, filepath.Join(providerRoot, "provider.marker"), providerID)
	writeBridgeFixtureFile(t, skillPath, "---\nname: "+skill+"\n---\n")
	bindingTree := digestBridgeFixtureTree(t, skillRoot)
	distributionTree := digestBridgeFixtureTree(t, providerRoot)
	descriptor := singleSkillProvider(providerID, skill, bindingTree, distributionTree)
	available, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(available, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory, diagnostics, err := codexbridge.BuildBindingInventory(available, report, appserver.MetadataObservation{
		Skills: appserver.SkillsEntry{CWD: home, Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{{
			Name: skill, Enabled: true, Path: skillPath, Scope: "user",
		}}},
	}, home)
	if err != nil || len(diagnostics) != 0 || len(inventory.Observations) != 1 {
		t.Fatalf("inventory = %#v, diagnostics = %#v, error = %v", inventory, diagnostics, err)
	}
	observation := inventory.Observations[0]
	candidates := report.Candidates(providerID)
	if len(candidates) != 1 || observation.InstallationKey != candidates[0].InstallationKey ||
		observation.ProviderID != providerID || observation.DistributionID != "fixture" || observation.BindingID != "codex-delivery" ||
		observation.Reference != skill || observation.Kind != catalog.BindingSkill || observation.Invocation != catalog.InvocationModel ||
		observation.BindingTreeDigest != bindingTree || observation.Source != host.SourceNativeAPI {
		t.Fatalf("candidates = %#v, observation = %#v", candidates, observation)
	}
}

func singleSkillProvider(providerID, skill, bindingTree, distributionTree string) catalog.ProviderDescriptorRecord {
	return catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0",
		ID: providerID, DisplayName: providerID,
		Distributions: []catalog.DistributionRecord{{
			ID: "fixture", SourceURI: "https://example.test/provider", Revision: strings.Repeat("a", 40), TreeDigest: distributionTree,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex-skill", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "fixture",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/plugins/provider", EvidencePath: "provider.marker",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-delivery", DistributionID: "fixture", ContentRoot: "skills/observed", InstallRoot: "skills/observed", TreeDigest: bindingTree,
			Host: "codex", Surface: "codex-plugin", Kind: catalog.BindingSkill, Reference: skill, Invocation: catalog.InvocationModel,
			Responsibilities: []catalog.ResponsibilityClaim{{
				Namespace: catalog.OwnershipStage, Name: string(catalog.SlotImplementation), SlotID: catalog.SlotImplementation, OutcomeOwner: true,
			}},
			InputArtifact: "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: catalog.DelegationRequirements{},
			StageSpan: []catalog.SlotID{catalog.SlotImplementation}, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "delivery", InputSchema: "artifact", OutcomeSchema: "artifact",
			RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"codex-delivery"},
		}},
	}
}

func digestBridgeFixtureTree(t *testing.T, path string) string {
	t.Helper()
	tree, err := integrity.DigestTree(path)
	if err != nil {
		t.Fatal(err)
	}
	return tree.RootDigest
}

func writeBridgeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
