package hosttest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/host/codex"
)

type ProviderFixture struct {
	Home         string
	Catalog      catalog.Catalog
	Discovery    discovery.Report
	Inventory    host.BindingInventory
	Candidate    discovery.Candidate
	Installation string
}

// BuildProviderFixture creates Host evidence through the Codex observer. It
// deliberately does not derive inventory observations from Descriptor fields.
func BuildProviderFixture(t testing.TB) ProviderFixture {
	t.Helper()
	home := t.TempDir()
	writeProviderFixtureFile(t, home, ".codex/plugins/acme/marker.txt", "acme")
	writeProviderFixtureFile(t, home, ".codex/plugins/acme/skills/review/SKILL.md", "---\nname: review\n---\n")
	descriptor := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV2, DescriptorVersion: "2.0.0", ID: "acme/suite", DisplayName: "Acme Suite",
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex", Hosts: []string{"codex"}, Surface: "codex-plugin", Distribution: "acme", Kind: "path-exists", Root: "user-home",
			CandidatePath: ".codex/plugins/acme", EvidencePath: "marker.txt",
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "review", InputSchema: "in", OutcomeSchema: "out", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			RequestModes: []catalog.RequestMode{catalog.RequestModeBounded, catalog.RequestModeWorkflow}, Responsibilities: []string{"review"},
			ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "acme:review"}},
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
	inventory, err := codex.ObserveBindings(value, report, codex.InventoryOptions{UserHome: home, CodexConfigRoot: filepath.Join(home, ".codex")})
	if err != nil {
		t.Fatal(err)
	}
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
