package hosttest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
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
		for _, capability := range provider.Capabilities {
			for _, binding := range capability.HostBindings {
				if binding.Host != report.HostID() || !slices.Contains(binding.Topologies, execution.TopologyCurrent) {
					continue
				}
				observation, found := observeFixtureBinding(t, report.HostID(), candidate, binding)
				if found {
					observations = append(observations, observation)
				}
			}
		}
	}
	inventory, err := host.NewBindingInventory(report.HostID(), observations)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func observeFixtureBinding(t testing.TB, hostID string, candidate discovery.Candidate, binding catalog.HostBinding) (host.BindingObservation, bool) {
	t.Helper()
	name, relative, source := binding.Reference, "", "host-filesystem"
	switch binding.Kind {
	case "skill":
		if _, suffix, found := strings.Cut(binding.Reference, ":"); found {
			name = suffix
			relative = filepath.Join("skills", suffix, "SKILL.md")
		} else {
			relative = filepath.Join(binding.Reference, "SKILL.md")
		}
	case "agent":
		relative = filepath.Join("agents", binding.Reference+".toml")
		source = "host-index"
	default:
		return host.BindingObservation{}, false
	}
	path := filepath.Join(candidate.Location, relative)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		content := "name = \"" + name + "\"\n"
		if binding.Kind == "skill" {
			content = "---\nname: " + name + "\n---\n"
		}
		writeProviderFixtureFile(t, candidate.Location, relative, content)
	} else if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return host.BindingObservation{
		HostID: hostID, InstallationKey: candidate.InstallationKey, Binding: binding,
		Topologies: []execution.Topology{execution.TopologyCurrent}, Source: source,
		EvidenceReference: physical, Digest: canonicaljson.DigestBytes(data),
	}, true
}

// BuildProviderFixture creates generic discovery and Host binding evidence.
func BuildProviderFixture(t testing.TB) ProviderFixture {
	t.Helper()
	home := t.TempDir()
	writeProviderFixtureFile(t, home, ".codex/plugins/acme/marker.txt", "acme")
	writeProviderFixtureFile(t, home, ".codex/plugins/acme/skills/review/SKILL.md", "---\nname: review\n---\n")
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV3, DescriptorVersion: "3.0.0", ID: "acme/suite", DisplayName: "Acme Suite",
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", Distribution: "acme", Kind: "path-exists", Root: "user-home",
			CandidatePath: ".codex/plugins/acme", EvidencePath: "marker.txt",
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "review", InputSchema: "in", OutcomeSchema: "out", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			RequestModes: []catalog.RequestMode{catalog.RequestModeBounded, catalog.RequestModeWorkflow}, Responsibilities: []string{"review"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}, DelegationAllowList: []string{},
			HostBindings: []catalog.HostBinding{{
				Host: "codex", Kind: "skill", Reference: "acme:review",
				Topologies: []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent},
			}},
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
