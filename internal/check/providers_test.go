package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

func TestCompatibilityProbeRejectsUnsupportedAndNonDirectoryRoots(t *testing.T) {
	root := t.TempDir()
	if compatibilityProbeExists(root, catalog.DiscoveryProbe{Root: "project-root", Kind: "path-exists", Path: "indicator"}) {
		t.Fatal("project-root compatibility probe was accepted")
	}
	if compatibilityProbeExists(root, catalog.DiscoveryProbe{Root: "user-home", Kind: "unsupported"}) {
		t.Fatal("unsupported compatibility probe was accepted")
	}
	prefix := filepath.Join(root, "versions")
	if err := os.WriteFile(prefix, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if compatibilityProbeExists(root, catalog.DiscoveryProbe{
		Root: "user-home", Kind: "one-level-version-path-exists",
		Prefix: "versions", Suffix: "skills/using-superpowers/SKILL.md",
	}) {
		t.Fatal("non-directory version root was accepted")
	}
}
