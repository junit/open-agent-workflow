package provideraudit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	if len(manifest.Providers) != 3 || !strings.HasPrefix(manifest.Providers[0].DistributionTreeDigest, "sha256:") {
		t.Fatalf("Build() manifest = %#v", manifest)
	}
	binding, found := manifest.Binding("oaw/matt", "codex-grill-with-docs")
	if !found || !strings.HasPrefix(binding.TreeDigest, "sha256:") || binding.InstallRoot != "grill-with-docs" {
		t.Fatalf("Binding() = %#v, %v", binding, found)
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

func TestBuildManifestAllowsContainedDistributionSymlink(t *testing.T) {
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
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.HasPrefix(manifest.Providers[0].DistributionTreeDigest, "sha256:") {
		t.Fatalf("DistributionTreeDigest = %q", manifest.Providers[0].DistributionTreeDigest)
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

func TestLockedCheckoutAndRevisionRecordsAreDefensive(t *testing.T) {
	values := LockedCheckouts("matt", "superpowers", "ecc")
	if len(values) != 3 || values[0].Root != "matt" || values[1].Root != "superpowers" || values[2].Root != "ecc" {
		t.Fatalf("LockedCheckouts() = %#v", values)
	}
	values[0].BindingRoots[0].ID = "changed"
	again := LockedCheckouts("matt", "superpowers", "ecc")
	if again[0].BindingRoots[0].ID == "changed" {
		t.Fatal("LockedCheckouts returned shared Binding state")
	}
	if revision, found := LockedRevision("oaw/matt"); !found || revision != "84fdeffd12f2ee307994d1eb6feb48173b6e0502" {
		t.Fatalf("LockedRevision(oaw/matt) = %q, %v", revision, found)
	}
	if _, found := LockedRevision("oaw/unknown"); found {
		t.Fatal("LockedRevision accepted an unknown Provider")
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
		for _, binding := range provider.Bindings {
			path := filepath.Join(root, filepath.FromSlash(binding.ContentRoot), "SKILL.md")
			if binding.Kind == "agent" || binding.Kind == "role" || binding.Kind == "instruction" {
				path = filepath.Join(root, filepath.FromSlash(binding.ContentRoot))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(binding.ID+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, evidenceRoot := range provider.EvidenceRoots {
			path := filepath.Join(root, filepath.FromSlash(evidenceRoot))
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
		checkouts = append(checkouts, Checkout{ProviderID: provider.ID, SourceURI: provider.SourceURI, Revision: provider.Revision, Root: root, DistributionRoot: ".", BindingRoots: bindingRoots})
	}
	return checkouts
}
