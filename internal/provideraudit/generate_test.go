package provideraudit

import (
	"testing"
)

func TestManifestDigestIsStableAndDefensive(t *testing.T) {
	manifest := buildTestManifest(t)
	original := manifest.Digest
	manifest.Providers[0].Bindings[0].References[0] = "changed"
	if manifest.ContentDigest() == original {
		t.Fatal("manifest content digest did not change after mutation")
	}
	if len(manifest.Digest) != 64 {
		t.Fatalf("manifest digest = %q", manifest.Digest)
	}
}

func TestManifestBindingAllowsSameContentRootAcrossHostSurfaces(t *testing.T) {
	manifest := buildTestManifest(t)
	first, found := manifest.Binding("oaw/matt", "codex-grill-with-docs")
	if !found {
		t.Fatal("missing Codex Matt Binding")
	}
	second, found := manifest.Binding("oaw/matt", "claude-grill-with-docs")
	if !found || first.ContentRoot != second.ContentRoot || first.ID == second.ID {
		t.Fatalf("host-qualified roots = %#v %#v", first, second)
	}
}
