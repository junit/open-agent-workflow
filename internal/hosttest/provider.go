package hosttest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

type ProviderFixture struct {
	Home         string
	Catalog      catalog.Catalog
	Discovery    discovery.Report
	Inventory    host.BindingInventory
	Candidate    discovery.Candidate
	Installation string
}

// ObserveProviderBindings creates test-only Host observations for the named
// discovered Providers. Production binding inventories are supplied by Hosts.
func ObserveProviderBindings(t testing.TB, value catalog.Catalog, report discovery.Report, home string, providerIDs ...string) host.BindingInventory {
	t.Helper()
	_ = home
	selected := make(map[string]struct{}, len(providerIDs))
	for _, providerID := range providerIDs {
		selected[providerID] = struct{}{}
	}
	observations := make([]host.BindingObservation, 0)
	for _, provider := range value.Providers() {
		if _, found := selected[provider.ID]; !found {
			continue
		}
		candidates := report.Candidates(provider.ID)
		if len(candidates) != 1 {
			t.Fatalf("Provider %s candidates = %d, want one", provider.ID, len(candidates))
		}
		candidate := candidates[0]
		for _, binding := range provider.Bindings {
			if binding.Host != report.HostID() || binding.DistributionID != candidate.DistributionID ||
				binding.Surface != candidate.Surface || !slices.Contains(binding.SupportedTopologies, execution.TopologyCurrent) {
				continue
			}
			observation, found := observeFixtureBinding(t, report.HostID(), provider.ID, candidate, binding)
			if found {
				observations = append(observations, observation)
			}
		}
	}
	inventory, err := host.BuildBindingInventoryV3(report.HostID(), observations)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func observeFixtureBinding(t testing.TB, hostID, providerID string, candidate discovery.Candidate, binding catalog.BindingRecord) (host.BindingObservation, bool) {
	t.Helper()
	name, relative := binding.Reference, ""
	switch binding.Kind {
	case catalog.BindingSkill:
		if _, suffix, found := strings.Cut(binding.Reference, ":"); found {
			name = suffix
			relative = filepath.Join("skills", suffix, "SKILL.md")
		} else {
			relative = filepath.Join(binding.Reference, "SKILL.md")
		}
	default:
		return host.BindingObservation{}, false
	}
	root := candidate.DiagnosticLocation
	path := filepath.Join(root, relative)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		content := "name = \"" + name + "\"\n"
		if binding.Kind == catalog.BindingSkill {
			content = "---\nname: " + name + "\n---\n"
		}
		writeProviderFixtureFile(t, root, relative, content)
	} else if err != nil {
		t.Fatal(err)
	}
	var treeDigest string
	for _, evidence := range candidate.BindingRoots {
		if evidence.BindingID == binding.ID {
			treeDigest = evidence.Tree.RootDigest
			break
		}
	}
	if treeDigest == "" {
		return host.BindingObservation{}, false
	}
	return host.BindingObservation{
		HostID: hostID, ProviderID: providerID, InstallationKey: candidate.InstallationKey,
		DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface,
		Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation,
		BindingTreeDigest: treeDigest, Topologies: []execution.Topology{execution.TopologyCurrent},
		Source: host.SourceLiveFilesystem, EvidenceReference: "evidence://hosttest/" + binding.ID,
	}, true
}

// BuildProviderFixture creates generic discovery and Host binding evidence.
func BuildProviderFixture(t testing.TB) ProviderFixture {
	t.Helper()
	home := t.TempDir()
	writeProviderFixtureFile(t, home, ".codex/plugins/acme/marker.txt", "acme")
	writeProviderFixtureFile(t, home, ".codex/plugins/acme/skills/review/SKILL.md", "---\nname: review\n---\n")
	installRoot := filepath.Join(home, ".codex", "plugins", "acme")
	bindingTree, err := integrity.DigestTree(filepath.Join(installRoot, "skills", "review"))
	if err != nil {
		t.Fatal(err)
	}
	distributionTree, err := integrity.DigestTree(installRoot)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: "acme/suite", DisplayName: "Acme Suite",
		Distributions: []catalog.DistributionRecord{{
			ID: "acme", SourceURI: "https://example.test/acme/suite", Revision: strings.Repeat("a", 40), TreeDigest: distributionTree.RootDigest,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "acme", Kind: "path-exists", Root: "user-home",
			CandidatePath: ".codex/plugins/acme", EvidencePath: "marker.txt",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-review", DistributionID: "acme", ContentRoot: "skills/review", InstallRoot: "skills/review", TreeDigest: bindingTree.RootDigest,
			Host: "codex", Surface: "codex-plugin", Kind: catalog.BindingSkill, Reference: "acme:review", Invocation: catalog.InvocationModel,
			Responsibilities: []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: string(catalog.SlotImplementation), SlotID: catalog.SlotImplementation, OutcomeOwner: true}},
			InputArtifact:    "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: catalog.DelegationRequirements{},
			StageSpan: []catalog.SlotID{catalog.SlotImplementation}, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "review", InputSchema: "artifact", OutcomeSchema: "artifact",
			RequestModes: []catalog.RequestMode{catalog.RequestModeBounded, catalog.RequestModeWorkflow}, BindingRefs: []string{"codex-review"},
		}},
	}
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor}, []catalog.ProfileRecipeRecord{}, []catalog.ProfileAliasRecord{})
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory := ObserveProviderBindings(t, value, report, home, "acme/suite")
	candidates := report.Candidates("acme/suite")
	if len(candidates) != 1 {
		t.Fatalf("ProviderFixture candidates = %#v", candidates)
	}
	return ProviderFixture{Home: home, Catalog: value, Discovery: report, Inventory: inventory, Candidate: candidates[0], Installation: candidates[0].InstallationKey}
}

func writeProviderFixtureFile(t testing.TB, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
