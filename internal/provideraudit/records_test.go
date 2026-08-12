package provideraudit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

func TestDecodeManifestRejectsUnknownFields(t *testing.T) {
	manifest := buildTestManifest(t)
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	value["unknown"] = true
	raw, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(raw); err == nil || !strings.Contains(err.Error(), "PROVIDER_AUDIT_INVALID") {
		t.Fatalf("Decode() error = %v", err)
	}
}

func TestDecodeManifestRoundTripsAndRejectsTrailingValue(t *testing.T) {
	manifest := buildTestManifest(t)
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Digest != manifest.Digest || decoded.Providers[0].ProviderID != "oaw/matt" {
		t.Fatalf("Decode() = %#v", decoded)
	}
	if _, err := Decode(append(raw, []byte(" {}")...)); err == nil {
		t.Fatal("Decode accepted a trailing JSON value")
	}
}

func TestDecodeManifestRejectsRetiredVersion(t *testing.T) {
	manifest := buildTestManifest(t)
	manifest.SchemaVersion = "oaw.provider-source-audit/v0"
	if err := Validate(manifest); err == nil {
		t.Fatal("Validate accepted retired schema")
	}
}

func TestManifestRequiresExactProviderPins(t *testing.T) {
	manifest := buildTestManifest(t)
	manifest.Providers[0].Revision = "main"
	if err := Validate(manifest); err == nil {
		t.Fatal("Validate accepted floating revision")
	}
}

func TestManifestRequiresUniqueBindingRoots(t *testing.T) {
	manifest := buildTestManifest(t)
	manifest.Providers[0].Bindings[1].ID = manifest.Providers[0].Bindings[0].ID
	if err := Validate(manifest); err == nil {
		t.Fatal("Validate accepted duplicate Binding ID")
	}
}

func TestManifestRequiresExactInstallRootMappings(t *testing.T) {
	manifest := buildTestManifest(t)
	manifest.Providers[0].Bindings[0].InstallRoot = "skills/engineering/grill-with-docs"
	if err := Validate(manifest); err == nil {
		t.Fatal("Validate accepted source-style Matt install root")
	}
	manifest = buildTestManifest(t)
	manifest.Providers[0].Bindings[0].ContentRoot = "../escape"
	if err := Validate(manifest); err == nil {
		t.Fatal("Validate accepted escaping content root")
	}
}

func TestManifestRequiresPrefixedTreeDigests(t *testing.T) {
	manifest := buildTestManifest(t)
	manifest.Providers[0].Bindings[0].TreeDigest = strings.Repeat("a", 64)
	if err := Validate(manifest); err == nil {
		t.Fatal("Validate accepted bare tree digest")
	}
}

func TestManifestRequiresCanonicalMatrixDigest(t *testing.T) {
	manifest := buildTestManifest(t)
	manifest.CanonicalMatrixDigest = strings.Repeat("a", 64)
	if err := Validate(manifest); err == nil {
		t.Fatal("Validate accepted incorrect matrix digest")
	}
}

func TestBuildManifestUsesTrackedBindingRoots(t *testing.T) {
	manifest := buildTestManifest(t)
	if len(manifest.Providers) != 4 || !strings.HasPrefix(manifest.Providers[0].DistributionTreeDigest, "sha256:") {
		t.Fatalf("Build() manifest = %#v", manifest)
	}
	providerIDs := make(map[string]struct{})
	for _, provider := range manifest.Providers {
		providerIDs[provider.ProviderID] = struct{}{}
	}
	if len(providerIDs) != 3 {
		t.Fatalf("Build() Provider IDs = %#v", providerIDs)
	}
	bindingsByDistribution := make(map[string]map[string]BindingSource)
	for _, provider := range manifest.Providers {
		bindings := make(map[string]BindingSource, len(provider.Bindings))
		for _, binding := range provider.Bindings {
			bindings[binding.ID] = binding
		}
		bindingsByDistribution[provider.DistributionID] = bindings
	}
	for _, skill := range superpowersSkillRoots() {
		reference := "superpowers:" + skill.Name
		for _, expected := range []struct {
			distributionID string
			bindingID      string
		}{
			{distributionID: "superpowers", bindingID: "codex-upstream-" + skill.Name},
			{distributionID: "superpowers", bindingID: "claude-" + skill.Name},
			{distributionID: "superpowers-codex", bindingID: "codex-" + skill.Name},
		} {
			binding, found := bindingsByDistribution[expected.distributionID][expected.bindingID]
			if !found || len(binding.References) != 1 || binding.References[0] != reference {
				t.Fatalf("Binding %s/%s = %#v, %v", expected.distributionID, expected.bindingID, binding, found)
			}
		}
	}
	binding, found := manifest.Binding("oaw/matt", "codex-grill-with-docs")
	if !found || !strings.HasPrefix(binding.TreeDigest, "sha256:") || binding.InstallRoot != "grill-with-docs" {
		t.Fatalf("Binding() = %#v, %v", binding, found)
	}
	upstream, found := manifest.Binding("oaw/superpowers", "codex-upstream-brainstorming")
	if !found || upstream.References[0] != "superpowers:brainstorming" {
		t.Fatalf("upstream Superpowers Binding = %#v, %v", upstream, found)
	}
	packaged, found := manifest.Binding("oaw/superpowers", "codex-brainstorming")
	if !found || packaged.References[0] != "superpowers:brainstorming" || packaged.TreeDigest == upstream.TreeDigest {
		t.Fatalf("packaged Superpowers Binding = %#v, %v", packaged, found)
	}
}

func TestBuildManifestDistributionDigestsMatchRuntimeAttestation(t *testing.T) {
	checkouts := testCheckouts(t)
	manifest, err := Build(checkouts)
	if err != nil {
		t.Fatal(err)
	}
	for _, checkout := range checkouts {
		root := filepath.Join(checkout.Root, filepath.FromSlash(checkout.DistributionRoot))
		evidence, err := integrity.DigestDistributionTree(root)
		if err != nil {
			t.Fatal(err)
		}
		providerIndex := slices.IndexFunc(manifest.Providers, func(provider ProviderSource) bool {
			return provider.ProviderID == checkout.ProviderID && provider.DistributionID == checkout.DistributionID
		})
		if providerIndex < 0 {
			t.Fatalf("missing manifest Distribution %s/%s", checkout.ProviderID, checkout.DistributionID)
		}
		if got := manifest.Providers[providerIndex].DistributionTreeDigest; got != evidence.RootDigest {
			t.Fatalf("Distribution %s digest = %q, runtime = %q", checkout.DistributionID, got, evidence.RootDigest)
		}
	}
}

func TestBuildManifestDigestsBindingsRelativeToDistributionRoot(t *testing.T) {
	checkouts := testCheckouts(t)
	var packaged Checkout
	for _, checkout := range checkouts {
		if checkout.DistributionID == "superpowers-codex" {
			packaged = checkout
			break
		}
	}
	if packaged.Root == "" {
		t.Fatal("missing packaged Superpowers checkout")
	}
	decoy := filepath.Join(packaged.Root, "skills", "brainstorming", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(decoy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decoy, []byte("checkout-root decoy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := Build(checkouts)
	if err != nil {
		t.Fatal(err)
	}
	binding, found := manifest.Binding("oaw/superpowers", "codex-brainstorming")
	if !found {
		t.Fatal("missing packaged Superpowers Binding")
	}
	want, err := digestBindingRoot(filepath.Join(packaged.Root, filepath.FromSlash(packaged.DistributionRoot)), "skills/brainstorming")
	if err != nil {
		t.Fatal(err)
	}
	if binding.TreeDigest != want {
		t.Fatalf("packaged Binding digest = %q, want %q", binding.TreeDigest, want)
	}
}

func TestBuildManifestRejectsRevisionDrift(t *testing.T) {
	checkouts := testCheckouts(t)
	checkouts[0].Revision = "main"
	if _, err := Build(checkouts); err == nil {
		t.Fatal("Build accepted revision drift")
	}
}

func TestBuildManifestRejectsMissingRoot(t *testing.T) {
	checkouts := testCheckouts(t)
	if err := os.RemoveAll(filepath.Join(checkouts[0].Root, "skills/engineering/grill-with-docs")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(checkouts); err == nil {
		t.Fatal("Build accepted missing Binding root")
	}
}

func TestBuildManifestRejectsMissingEvidenceRoot(t *testing.T) {
	checkouts := testCheckouts(t)
	if err := os.Remove(filepath.Join(checkouts[0].Root, "README.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(checkouts); err == nil {
		t.Fatal("Build accepted a missing evidence root")
	}
}

func TestBuildManifestRejectsEvidenceRootSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}
	checkouts := testCheckouts(t)
	root := checkouts[0].Root
	evidenceRoot := filepath.Join(root, "README.md")
	realEvidenceRoot := filepath.Join(root, "README.real.md")
	if err := os.Rename(evidenceRoot, realEvidenceRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(realEvidenceRoot), evidenceRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(checkouts); err == nil {
		t.Fatal("Build accepted an evidence root symlink")
	}
}

func TestBuildManifestRecordsContainedDistributionSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}
	checkouts := testCheckouts(t)
	root := checkouts[0].Root
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("instructions\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("CLAUDE.md", filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	manifest, err := Build(checkouts)
	if err != nil {
		t.Fatal(err)
	}
	root = checkouts[0].Root
	evidence, err := integrity.DigestDistributionTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Providers[0].DistributionTreeDigest != evidence.RootDigest {
		t.Fatalf("contained Distribution symlink digest = %q, want %q", manifest.Providers[0].DistributionTreeDigest, evidence.RootDigest)
	}
}

func TestBuildManifestRejectsEscapingDistributionSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}
	checkouts := testCheckouts(t)
	if err := os.Symlink("../outside", filepath.Join(checkouts[0].Root, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(checkouts); err == nil {
		t.Fatal("Build accepted escaping Distribution symlink")
	}
}

func TestBuildManifestRejectsDistributionRootSymlinkAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}
	checkouts := testCheckouts(t)
	packagedIndex := -1
	for index, checkout := range checkouts {
		if checkout.DistributionID == "superpowers-codex" {
			packagedIndex = index
			break
		}
	}
	if packagedIndex == -1 {
		t.Fatal("missing packaged Superpowers checkout")
	}
	checkout := checkouts[packagedIndex]
	pluginsRoot := filepath.Join(checkout.Root, "plugins")
	outsidePlugins := filepath.Join(t.TempDir(), "plugins")
	if err := os.Rename(pluginsRoot, outsidePlugins); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePlugins, pluginsRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(checkouts); err == nil {
		t.Fatal("Build accepted a Distribution root through a symlink ancestor")
	}
}

func TestBuildManifestRejectsBindingRootSymlinkAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges on Windows")
	}
	for _, test := range []struct {
		name           string
		distributionID string
		path           string
		replacement    string
	}{
		{name: "first directory", distributionID: "matt-skills", path: "skills", replacement: "real-skills"},
		{name: "intermediate directory", distributionID: "matt-skills", path: "skills/engineering", replacement: "real-engineering"},
		{name: "final directory", distributionID: "matt-skills", path: "skills/engineering/grill-with-docs", replacement: "real-grill-with-docs"},
		{name: "final file", distributionID: "ecc", path: "agents/architect.md", replacement: "architect.real.md"},
	} {
		t.Run(test.name, func(t *testing.T) {
			checkouts := testCheckouts(t)
			var distributionRoot string
			for _, checkout := range checkouts {
				if checkout.DistributionID == test.distributionID {
					distributionRoot = filepath.Join(checkout.Root, filepath.FromSlash(checkout.DistributionRoot))
					break
				}
			}
			if distributionRoot == "" {
				t.Fatalf("Distribution %s not found", test.distributionID)
			}
			root := filepath.Join(distributionRoot, filepath.FromSlash(test.path))
			replacement := filepath.Join(filepath.Dir(root), test.replacement)
			if err := os.Rename(root, replacement); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(replacement), root); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(checkouts); err == nil {
				t.Fatal("Build accepted a Binding root through a symlink ancestor")
			}
		})
	}
}

func TestLockedCheckoutAndRevisionRecordsAreDefensive(t *testing.T) {
	values := LockedCheckouts("matt", "superpowers", "openai-plugins", "ecc")
	if len(values) != 4 || values[0].Root != "matt" || values[1].Root != "superpowers" || values[2].Root != "openai-plugins" || values[3].Root != "ecc" {
		t.Fatalf("LockedCheckouts() = %#v", values)
	}
	if values[1].DistributionID != "superpowers" || values[1].DistributionRoot != "." || values[2].DistributionID != "superpowers-codex" || values[2].DistributionRoot != "plugins/superpowers" {
		t.Fatalf("Superpowers checkouts = %#v %#v", values[1], values[2])
	}
	values[0].BindingRoots[0].ID = "changed"
	again := LockedCheckouts("matt", "superpowers", "openai-plugins", "ecc")
	if again[0].BindingRoots[0].ID == "changed" {
		t.Fatal("LockedCheckouts returned shared Binding state")
	}
	if revision, found := LockedRevision("oaw/matt", "matt-skills"); !found || revision != "84fdeffd12f2ee307994d1eb6feb48173b6e0502" {
		t.Fatalf("LockedRevision(oaw/matt) = %q, %v", revision, found)
	}
	if revision, found := LockedRevision("oaw/superpowers", "superpowers-codex"); !found || revision != "11c74d6ba24d3a6d48f54a194cd00ef3beea18f9" {
		t.Fatalf("LockedRevision(oaw/superpowers, superpowers-codex) = %q, %v", revision, found)
	}
	if _, found := LockedRevision("oaw/superpowers", "unknown"); found {
		t.Fatal("LockedRevision accepted an unknown Distribution")
	}
}

func buildTestManifest(t *testing.T) Manifest {
	t.Helper()
	manifest, err := Build(testCheckouts(t))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func testCheckouts(t *testing.T) []Checkout {
	t.Helper()
	checkouts := make([]Checkout, 0, len(lockedProviderSpecs))
	for _, provider := range lockedProviderSpecs {
		root := t.TempDir()
		distributionRoot := "."
		if provider.DistributionID == "superpowers-codex" {
			distributionRoot = "plugins/superpowers"
		}
		sourceRoot := filepath.Join(root, filepath.FromSlash(distributionRoot))
		for _, binding := range provider.Bindings {
			path := filepath.Join(sourceRoot, filepath.FromSlash(binding.ContentRoot), "SKILL.md")
			if binding.Kind == "agent" || binding.Kind == "role" || binding.Kind == "instruction" {
				path = filepath.Join(sourceRoot, filepath.FromSlash(binding.ContentRoot))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(binding.ID+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, evidenceRoot := range provider.EvidenceRoots {
			path := filepath.Join(sourceRoot, filepath.FromSlash(evidenceRoot))
			if filepath.Ext(path) == "" {
				path = filepath.Join(path, "evidence.txt")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("evidence\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		bindingRoots := make([]BindingCheckout, len(provider.Bindings))
		for index, binding := range provider.Bindings {
			bindingRoots[index] = BindingCheckout{ID: binding.ID, ContentRoot: binding.ContentRoot, InstallRoot: binding.InstallRoot, Root: binding.ContentRoot}
		}
		checkouts = append(checkouts, Checkout{ProviderID: provider.ID, DistributionID: provider.DistributionID, SourceURI: provider.SourceURI, Revision: provider.Revision, Root: root, DistributionRoot: distributionRoot, BindingRoots: bindingRoots})
	}
	return checkouts
}
