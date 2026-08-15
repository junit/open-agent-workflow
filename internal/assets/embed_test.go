package assets

import (
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

func TestEmbeddedAssetsContainOnlyProviderBindingEvidence(t *testing.T) {
	var paths []string
	if err := fs.WalkDir(FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"audits/provider-sources-v5.json",
		"providers/oaw-ecc.json",
		"providers/oaw-matt.json",
		"providers/oaw-superpowers.json",
	}
	if !slices.Equal(paths, want) {
		t.Fatalf("embedded assets = %q, want %q", paths, want)
	}
	for _, path := range paths[1:] {
		raw, err := fs.ReadFile(FS(), path)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(raw, &document); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		for _, forbidden := range []string{"capabilities", "recipes", "aliases", "responsibilities", "request_modes"} {
			if _, found := document[forbidden]; found || strings.Contains(string(raw), `"`+forbidden+`"`) {
				t.Fatalf("%s contains duplicate workflow field %q", path, forbidden)
			}
		}
	}
}
