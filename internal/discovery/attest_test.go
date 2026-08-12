package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadImmutableSourceManifestRejectsNonPhysicalPaths(t *testing.T) {
	if err := os.Mkdir(filepath.Join(t.TempDir(), "probe"), 0o700); err != nil {
		t.Fatal(err)
	}
	physical := t.TempDir()
	manifest := []byte(`{"distribution_id":"distribution","revision":"` + strings.Repeat("a", 40) + `","tree_digest":"sha256:` + strings.Repeat("b", 64) + `"}`)
	if err := os.WriteFile(filepath.Join(physical, immutableSourceManifest), manifest, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("symlinked root", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "provider")
		if err := os.Symlink(physical, root); err != nil {
			t.Fatal(err)
		}
		if _, found, err := readImmutableSourceManifest(root); !found || err == nil {
			t.Fatalf("readImmutableSourceManifest(symlinked root) = found %v, error %v", found, err)
		}
	})

	t.Run("symlinked manifest", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Symlink(filepath.Join(physical, immutableSourceManifest), filepath.Join(root, immutableSourceManifest)); err != nil {
			t.Fatal(err)
		}
		if _, found, err := readImmutableSourceManifest(root); !found || err == nil {
			t.Fatalf("readImmutableSourceManifest(symlinked manifest) = found %v, error %v", found, err)
		}
	})
}

func TestReadRootedImmutableSourceManifestRejectsReplacementAfterInspection(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, immutableSourceManifest)
	manifest := []byte(`{"distribution_id":"distribution","revision":"` + strings.Repeat("a", 40) + `","tree_digest":"sha256:` + strings.Repeat("b", 64) + `"}`)
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	rooted, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer rooted.Close()
	inspected, err := rooted.Lstat(immutableSourceManifest)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "replacement.json")
	if err := os.WriteFile(replacement, manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, manifestPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readRootedImmutableSourceManifest(rooted, inspected); err == nil {
		t.Fatal("readRootedImmutableSourceManifest accepted a replacement file")
	}
}
