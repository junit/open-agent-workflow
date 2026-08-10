package discovery_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

const distributionRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestDiscoveryDistributionAttestedRequiresExactManifestAndTrees(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "provider")
	writeFile(t, root, "probe.txt", "probe")
	bindingTree := writeFile(t, root, "to-spec/SKILL.md", "exact")
	bindingDigest := treeDigest(t, filepath.Dir(bindingTree))
	distributionDigest := "sha256:" + strings.Repeat("b", 64)
	writeJSON(t, root, ".oaw-distribution.json", map[string]string{"distribution_id": "distribution", "revision": distributionRevision, "tree_digest": distributionDigest})

	value := testCatalog(t, "oaw/matt", "provider", "to-spec", bindingDigest, distributionDigest, distributionRevision)
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidates := report.Candidates("oaw/matt")
	if len(candidates) != 1 {
		t.Fatalf("Candidates() = %#v", candidates)
	}
	candidate := candidates[0]
	if candidate.Provenance != discovery.ProvenanceDistributionAttested || candidate.ObservedRevision != distributionRevision || candidate.DistributionTreeDigest != distributionDigest {
		t.Fatalf("attested candidate = %#v", candidate)
	}
	if len(candidate.BindingRoots) != 1 || candidate.BindingRoots[0].InstallRoot != "to-spec" || candidate.BindingRoots[0].ContentRoot != "skills/engineering/to-spec" {
		t.Fatalf("binding roots = %#v", candidate.BindingRoots)
	}
	if !strings.HasPrefix(candidate.Evidence[0].EvidenceReference, "evidence://") || strings.Contains(candidate.Evidence[0].EvidenceReference, root) {
		t.Fatalf("evidence reference leaked a path: %q", candidate.Evidence[0].EvidenceReference)
	}
}

func TestDiscoveryContentEquivalentFlattenedBindingHasNoFabricatedRevision(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "flat")
	writeFile(t, root, "probe.txt", "probe")
	bindingTree := writeFile(t, root, "to-spec/SKILL.md", "flattened")
	bindingDigest := treeDigest(t, filepath.Dir(bindingTree))
	distributionDigest := "sha256:" + strings.Repeat("c", 64)
	value := testCatalog(t, "oaw/matt", "flat", "to-spec", bindingDigest, distributionDigest, distributionRevision)

	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidate := report.Candidates("oaw/matt")[0]
	if candidate.Provenance != discovery.ProvenanceContentEquivalent || candidate.ObservedRevision != "" || candidate.DistributionTreeDigest != "" {
		t.Fatalf("content-equivalent candidate fabricated provenance: %#v", candidate)
	}
}

func TestDiscoverySharedAncestorNeverEstablishesProviderProvenance(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "shared")
	writeFile(t, root, "probe.txt", "probe")
	value := testCatalog(t, "oaw/matt", "shared", "to-spec", "sha256:"+strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64), distributionRevision)
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidates := report.Candidates("oaw/matt")
	if len(candidates) != 1 || candidates[0].Provenance != "" || len(candidates[0].BindingRoots) != 0 {
		t.Fatalf("shared ancestor candidate = %#v", candidates)
	}
}

func TestDiscoveryContentMismatchRetainsDiagnosticCandidateWithoutDisposition(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "provider")
	writeFile(t, root, "probe.txt", "probe")
	bindingTree := writeFile(t, root, "to-spec/SKILL.md", "original")
	expectedDigest := treeDigest(t, filepath.Dir(bindingTree))
	if err := os.WriteFile(bindingTree, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	value := testCatalog(t, "oaw/matt", "provider", "to-spec", expectedDigest, "sha256:"+strings.Repeat("b", 64), distributionRevision)
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidates := report.Candidates("oaw/matt")
	if len(candidates) != 1 || candidates[0].Provenance != "" || candidates[0].ObservedRevision != "" {
		t.Fatalf("mismatched candidate = %#v", candidates)
	}
}

func TestDiscoveryRevisionMismatchRetainsCandidateWithoutDistributionAttestation(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "provider")
	writeFile(t, root, "probe.txt", "probe")
	writeFile(t, root, "to-spec/SKILL.md", "exact")
	bindingDigest := treeDigest(t, filepath.Join(root, "to-spec"))
	distributionDigest := "sha256:" + strings.Repeat("b", 64)
	writeJSON(t, root, ".oaw-distribution.json", map[string]string{"distribution_id": "distribution", "revision": strings.Repeat("c", 40), "tree_digest": distributionDigest})
	value := testCatalog(t, "oaw/matt", "provider", "to-spec", bindingDigest, distributionDigest, distributionRevision)
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidate := report.Candidates("oaw/matt")[0]
	if candidate.Provenance != "" || candidate.ObservedRevision != "" || candidate.DistributionTreeDigest != "" {
		t.Fatalf("revision-mismatched candidate = %#v", candidate)
	}
}

func TestDiscoveryPreservesBindingRootOrderAndDefensiveCopies(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "provider")
	writeFile(t, root, "probe.txt", "probe")
	bindingADigest := treeDigest(t, filepath.Dir(writeFile(t, root, "a/SKILL.md", "a")))
	bindingZDigest := treeDigest(t, filepath.Dir(writeFile(t, root, "z/SKILL.md", "z")))
	distributionDigest := "sha256:" + strings.Repeat("d", 64)
	writeJSON(t, root, ".oaw-distribution.json", map[string]string{"distribution_id": "distribution", "revision": distributionRevision, "tree_digest": distributionDigest})
	value := testCatalogWithBindings(t, "oaw/matt", "provider", []catalog.BindingRecord{
		bindingRecord("z-binding", "z", bindingZDigest),
		bindingRecord("a-binding", "a", bindingADigest),
	}, distributionDigest, distributionRevision)
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidate := report.Candidates("oaw/matt")[0]
	if len(candidate.BindingRoots) != 2 || candidate.BindingRoots[0].BindingID != "a-binding" || candidate.BindingRoots[1].BindingID != "z-binding" {
		t.Fatalf("binding root order = %#v", candidate.BindingRoots)
	}
	digest := report.Digest()
	candidate.BindingRoots[0].Tree.Entries[0].Path = "changed"
	if report.Candidates("oaw/matt")[0].BindingRoots[0].Tree.Entries[0].Path == "changed" || report.Digest() != digest {
		t.Fatal("Candidates exposed mutable binding evidence")
	}
}

func TestDiscoverySeparatesProvidersAndSurfacesWithSameBindingIDs(t *testing.T) {
	home := t.TempDir()
	for _, provider := range []string{"matt", "other"} {
		root := filepath.Join(home, provider)
		writeFile(t, root, "probe.txt", provider)
		writeFile(t, root, "to-spec/SKILL.md", provider)
	}
	mattDigest := treeDigest(t, filepath.Join(home, "matt", "to-spec"))
	otherDigest := treeDigest(t, filepath.Join(home, "other", "to-spec"))
	value := testCatalogWithProviders(t, []providerInput{
		{ID: "oaw/matt", CandidatePath: "matt", Surface: "codex-skills", BindingDigest: mattDigest},
		{ID: "oaw/other", CandidatePath: "other", Surface: "other-skills", BindingDigest: otherDigest},
	})
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(report.Candidates("oaw/matt")) != 1 || len(report.Candidates("oaw/other")) != 1 || report.Candidates("oaw/matt")[0].Surface != "codex-skills" || report.Candidates("oaw/other")[0].Surface != "other-skills" {
		t.Fatalf("provider/surface candidates = %#v", report)
	}
}

type providerInput struct {
	ID            string
	CandidatePath string
	Surface       string
	BindingDigest string
}

func testCatalog(t *testing.T, providerID, candidatePath, installRoot, bindingDigest, distributionDigest, revision string) catalog.Catalog {
	t.Helper()
	return testCatalogWithBindings(t, providerID, candidatePath, []catalog.BindingRecord{bindingRecord("binding", installRoot, bindingDigest)}, distributionDigest, revision)
}

func testCatalogWithBindings(t *testing.T, providerID, candidatePath string, bindings []catalog.BindingRecord, distributionDigest, revision string) catalog.Catalog {
	t.Helper()
	for index := range bindings {
		bindings[index].DistributionID = "distribution"
		bindings[index].Host = "codex"
		bindings[index].Surface = "codex-skills"
	}
	provider := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: providerID, DisplayName: providerID,
		Distributions: []catalog.DistributionRecord{{ID: "distribution", SourceURI: "https://example.test/" + strings.TrimPrefix(providerID, "oaw/"), Revision: revision, TreeDigest: distributionDigest}},
		Discovery:     []catalog.DiscoveryProbe{{ID: "probe", Hosts: []string{"codex"}, Surface: bindings[0].Surface, DistributionID: "distribution", Kind: "path-exists", Root: "user-home", CandidatePath: candidatePath, EvidencePath: "probe.txt"}},
		Bindings:      bindings,
		Capabilities:  []catalog.CapabilityRecord{{ID: "capability", InputSchema: "artifact", OutcomeSchema: "artifact", RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: bindingIDs(bindings)}},
	}
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, nil, nil)
	if err != nil {
		t.Fatalf("catalog.New() error = %v", err)
	}
	return value
}

func testCatalogWithProviders(t *testing.T, inputs []providerInput) catalog.Catalog {
	t.Helper()
	providers := make([]catalog.ProviderDescriptorRecord, 0, len(inputs))
	for _, input := range inputs {
		binding := bindingRecord("binding", "to-spec", input.BindingDigest)
		binding.Surface = input.Surface
		binding.Host = "codex"
		providers = append(providers, catalog.ProviderDescriptorRecord{
			SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0", ID: input.ID, DisplayName: input.ID,
			Distributions: []catalog.DistributionRecord{{ID: "distribution", SourceURI: "https://example.test/provider", Revision: distributionRevision, TreeDigest: "sha256:" + strings.Repeat("e", 64)}},
			Discovery:     []catalog.DiscoveryProbe{{ID: "probe", Hosts: []string{"codex"}, Surface: input.Surface, DistributionID: "distribution", Kind: "path-exists", Root: "user-home", CandidatePath: input.CandidatePath, EvidencePath: "probe.txt"}},
			Bindings:      []catalog.BindingRecord{binding}, Capabilities: []catalog.CapabilityRecord{{ID: "capability", InputSchema: "artifact", OutcomeSchema: "artifact", RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"binding"}}},
		})
	}
	value, err := catalog.New(providers, nil, nil)
	if err != nil {
		t.Fatalf("catalog.New() error = %v", err)
	}
	return value
}

func bindingRecord(id, installRoot, digest string) catalog.BindingRecord {
	return catalog.BindingRecord{
		ID: id, DistributionID: "distribution", ContentRoot: "skills/engineering/to-spec", InstallRoot: installRoot, TreeDigest: digest,
		Host: "codex", Surface: "codex-skills", Kind: catalog.BindingSkill, Reference: "to-spec", Invocation: catalog.InvocationModel,
		Responsibilities: []catalog.ResponsibilityClaim{{Namespace: catalog.OwnershipStage, Name: "implementation", SlotID: catalog.SlotImplementation, OutcomeOwner: true}}, InputArtifact: "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: catalog.DelegationRequirements{}, StageSpan: []catalog.SlotID{catalog.SlotImplementation}, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
	}
}

func bindingIDs(values []catalog.BindingRecord) []string {
	result := make([]string, len(values))
	for index := range values {
		result[index] = values[index].ID
	}
	return result
}

func writeFile(t *testing.T, root, relative, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeJSON(t *testing.T, root, relative string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, relative, string(raw))
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	evidence, err := integrity.DigestTree(root)
	if err != nil {
		t.Fatalf("DigestTree() error = %v", err)
	}
	return evidence.RootDigest
}
