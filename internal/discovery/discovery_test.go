package discovery_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
)

func TestDiscoverBuiltInsProducesSortedEvidence(t *testing.T) {
	home := t.TempDir()
	writeEvidence(t, home, ".codex/plugins/superpowers/skills/using-superpowers/SKILL.md", "superpowers")
	writeEvidence(t, home, ".agents/skills/to-spec/SKILL.md", "matt")
	writeEvidence(t, home, ".agents/skills/everything-claude-code/SKILL.md", "ecc")
	value, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	first, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	second, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover(second) error = %v", err)
	}
	if first.Digest() == "" || first.Digest() != second.Digest() {
		t.Fatalf("report digests = %q / %q", first.Digest(), second.Digest())
	}

	for providerID, content := range map[string]string{
		"oaw/ecc":         "ecc",
		"oaw/matt":        "matt",
		"oaw/superpowers": "superpowers",
	} {
		candidates := first.Candidates(providerID)
		if len(candidates) != 1 || len(candidates[0].Evidence) != 1 {
			t.Fatalf("Candidates(%q) = %#v", providerID, candidates)
		}
		if candidates[0].Evidence[0].ContentDigest != contentDigest(content) {
			t.Fatalf("Candidates(%q) content digest = %q", providerID, candidates[0].Evidence[0].ContentDigest)
		}
	}
}

func TestDiscoverScopesCandidatesToSelectedHost(t *testing.T) {
	home := t.TempDir()
	writeEvidence(t, home, ".codex/skills/acme/review/SKILL.md", "codex")
	writeEvidence(t, home, ".claude/skills/acme/review/SKILL.md", "claude")
	value := testCatalog(t,
		catalog.DiscoveryProbe{ID: "codex", Hosts: []string{"codex"}, Surface: "codex-skills", Distribution: "acme", Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/skills/acme", EvidencePath: "review/SKILL.md"},
		catalog.DiscoveryProbe{ID: "claude", Hosts: []string{"claude"}, Surface: "claude-skills", Distribution: "acme", Kind: "path-exists", Root: "user-home", CandidatePath: ".claude/skills/acme", EvidencePath: "review/SKILL.md"},
	)
	codex, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	got := codex.Candidates("acme/suite")
	if len(got) != 1 || got[0].HostID != "codex" || strings.Contains(got[0].Location, ".claude") {
		t.Fatalf("Codex candidates = %#v", got)
	}
}

func TestDiscoverSeparatesSharedInstallationByHost(t *testing.T) {
	home := t.TempDir()
	writeEvidence(t, home, ".agents/skills/acme/review/SKILL.md", "shared")
	value := testCatalog(t, catalog.DiscoveryProbe{
		ID: "shared", Hosts: []string{"claude", "codex"}, Surface: "shared-skills", Distribution: "acme", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/skills/acme", EvidencePath: "review/SKILL.md",
	})
	codex, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	claude, err := discovery.Discover(value, discovery.Options{HostID: "claude", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	codexCandidate := codex.Candidates("acme/suite")[0]
	claudeCandidate := claude.Candidates("acme/suite")[0]
	if codexCandidate.Location != claudeCandidate.Location || codexCandidate.InstallationKey == claudeCandidate.InstallationKey {
		t.Fatalf("shared candidates = %#v / %#v", codexCandidate, claudeCandidate)
	}
}

func TestDiscoverGroupsDirectEvidenceIntoOneCandidate(t *testing.T) {
	home := t.TempDir()
	writeEvidence(t, home, ".agents/acme/skill/SKILL.md", "skill")
	target := writeEvidence(t, home, ".agents/acme/assets/plugin.json", "plugin")
	symlink := filepath.Join(home, ".agents", "acme", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(symlink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	value := testCatalog(t,
		catalog.DiscoveryProbe{ID: "skill", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/acme", EvidencePath: "skill/SKILL.md"},
		catalog.DiscoveryProbe{ID: "plugin", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/acme", EvidencePath: "plugin.json"},
	)
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidates := report.Candidates("acme/suite")
	if len(candidates) != 1 {
		t.Fatalf("Candidates() = %#v", candidates)
	}
	candidate := candidates[0]
	physicalCandidate := physicalPath(t, filepath.Join(home, ".agents", "acme"))
	if candidate.Location != physicalCandidate || candidate.InstallationKey == "" || !strings.HasPrefix(candidate.Version, "content-") {
		t.Fatalf("direct candidate = %#v", candidate)
	}
	if len(candidate.Evidence) != 2 || candidate.Evidence[0].ProbeID != "plugin" || candidate.Evidence[1].ProbeID != "skill" {
		t.Fatalf("evidence order = %#v", candidate.Evidence)
	}
	physicalTarget := physicalPath(t, target)
	if candidate.Evidence[0].Path != physicalTarget {
		t.Fatalf("symlink physical path = %q, want %q", candidate.Evidence[0].Path, physicalTarget)
	}
	candidate.Evidence[0].Path = "changed"
	if report.Candidates("acme/suite")[0].Evidence[0].Path == "changed" {
		t.Fatal("Candidates exposed mutable evidence storage")
	}
}

func TestDiscoverCreatesOneCandidatePerImmediateVersion(t *testing.T) {
	home := t.TempDir()
	writeEvidence(t, home, ".cache/acme/2.0.0/SKILL.md", "two")
	writeEvidence(t, home, ".cache/acme/1.0.0/SKILL.md", "one")
	value := testCatalog(t, catalog.DiscoveryProbe{
		ID: "versions", Kind: "one-level-version-path-exists", Root: "user-home", Prefix: ".cache/acme", EvidencePath: "SKILL.md",
	})
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	candidates := report.Candidates("acme/suite")
	if len(candidates) != 2 || candidates[0].Version != "1.0.0" || candidates[1].Version != "2.0.0" {
		t.Fatalf("Candidates() = %#v", candidates)
	}
	physicalHome := physicalPath(t, home)
	for _, candidate := range candidates {
		if candidate.Location != filepath.Join(physicalHome, ".cache", "acme", candidate.Version) || len(candidate.Evidence) != 1 {
			t.Fatalf("version candidate = %#v", candidate)
		}
		if candidate.Evidence[0].Version != candidate.Version || candidate.EvidenceDigest == "" {
			t.Fatalf("version evidence = %#v", candidate.Evidence)
		}
	}
}

func TestDiscoverReportsNoCandidateForMissingFiles(t *testing.T) {
	value := testCatalog(t, catalog.DiscoveryProbe{ID: "missing", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/missing", EvidencePath: "SKILL.md"})
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: t.TempDir()})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if candidates := report.Candidates("acme/suite"); candidates == nil || len(candidates) != 0 {
		t.Fatalf("Candidates() = %#v", candidates)
	}
	if report.Digest() == "" {
		t.Fatal("empty report digest")
	}
}

func TestDiscoverRejectsEscapingSymlinkAndOversizedEvidence(t *testing.T) {
	t.Run("escaping symlink", func(t *testing.T) {
		home := t.TempDir()
		outside := writeEvidence(t, t.TempDir(), "outside.txt", "outside")
		if err := os.MkdirAll(filepath.Join(home, "candidate"), 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(home, "candidate", "evidence.txt")
		if err := os.Symlink(outside, link); err != nil {
			t.Fatal(err)
		}
		value := testCatalog(t, catalog.DiscoveryProbe{ID: "direct", Kind: "path-exists", Root: "user-home", CandidatePath: "candidate", EvidencePath: "evidence.txt"})
		if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_PATH_ESCAPE") {
			t.Fatalf("Discover() error = %v", err)
		}
	})

	t.Run("oversized evidence", func(t *testing.T) {
		home := t.TempDir()
		writeEvidence(t, home, "candidate/evidence.txt", "12345")
		value := testCatalog(t, catalog.DiscoveryProbe{ID: "direct", Kind: "path-exists", Root: "user-home", CandidatePath: "candidate", EvidencePath: "evidence.txt"})
		if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home, MaximumEvidenceBytes: 4}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_EVIDENCE_TOO_LARGE") {
			t.Fatalf("Discover() error = %v", err)
		}
	})

	t.Run("hard limit cannot be raised", func(t *testing.T) {
		home := t.TempDir()
		writeEvidence(t, home, "candidate/evidence.txt", strings.Repeat("x", (4<<20)+1))
		value := testCatalog(t, catalog.DiscoveryProbe{ID: "direct", Kind: "path-exists", Root: "user-home", CandidatePath: "candidate", EvidencePath: "evidence.txt"})
		if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home, MaximumEvidenceBytes: 8 << 20}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_EVIDENCE_TOO_LARGE") {
			t.Fatalf("Discover() error = %v", err)
		}
	})
}

func TestDiscoverIgnoresNestedVersionDirectories(t *testing.T) {
	home := t.TempDir()
	writeEvidence(t, home, ".cache/acme/channel/1.0.0/SKILL.md", "nested")
	value := testCatalog(t, catalog.DiscoveryProbe{
		ID: "versions", Kind: "one-level-version-path-exists", Root: "user-home", Prefix: ".cache/acme", EvidencePath: "SKILL.md",
	})
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if candidates := report.Candidates("acme/suite"); len(candidates) != 0 {
		t.Fatalf("Candidates() = %#v", candidates)
	}
}

func TestDiscoveryDigestIsIndependentOfDirectoryEnumerationOrder(t *testing.T) {
	home := t.TempDir()
	prefix := filepath.Join(home, ".cache", "acme")
	createVersions := func(versions []string) {
		t.Helper()
		for _, version := range versions {
			writeEvidence(t, home, filepath.ToSlash(filepath.Join(".cache", "acme", version, "SKILL.md")), version)
		}
	}
	value := testCatalog(t, catalog.DiscoveryProbe{
		ID: "versions", Kind: "one-level-version-path-exists", Root: "user-home", Prefix: ".cache/acme", EvidencePath: "SKILL.md",
	})
	createVersions([]string{"3", "1", "2"})
	first, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(prefix); err != nil {
		t.Fatal(err)
	}
	createVersions([]string{"2", "3", "1"})
	second, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("report digests = %q / %q", first.Digest(), second.Digest())
	}
}

func TestDiscoverRejectsUnsupportedProbeSurface(t *testing.T) {
	tests := []struct {
		name  string
		probe catalog.DiscoveryProbe
	}{
		{"root", catalog.DiscoveryProbe{ID: "xdg", Kind: "path-exists", Root: "xdg-config-home", CandidatePath: "acme", EvidencePath: "SKILL.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := discovery.Discover(testCatalog(t, tt.probe), discovery.Options{HostID: "codex", UserHome: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_PROBE_UNSUPPORTED") {
				t.Fatalf("Discover() error = %v", err)
			}
		})
	}
}

func TestDiscoverRejectsInvalidRootAndNonRegularEvidence(t *testing.T) {
	value := testCatalog(t, catalog.DiscoveryProbe{ID: "direct", Kind: "path-exists", Root: "user-home", CandidatePath: "candidate", EvidencePath: "evidence"})
	for _, root := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: root}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_ROOT_INVALID") {
			t.Fatalf("Discover(root=%q) error = %v", root, err)
		}
	}
	fileRoot := writeEvidence(t, t.TempDir(), "root.txt", "file")
	if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: fileRoot}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_ROOT_INVALID") {
		t.Fatalf("Discover(file root) error = %v", err)
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "candidate", "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_EVIDENCE_NOT_REGULAR") {
		t.Fatalf("Discover(directory evidence) error = %v", err)
	}
}

func TestDiscoverVersionProbeHandlesOnlyContainedDirectories(t *testing.T) {
	probe := catalog.DiscoveryProbe{
		ID: "versions", Kind: "one-level-version-path-exists", Root: "user-home", Prefix: ".cache/acme", EvidencePath: "SKILL.md",
	}
	value := testCatalog(t, probe)

	t.Run("prefix must be directory", func(t *testing.T) {
		home := t.TempDir()
		writeEvidence(t, home, ".cache/acme", "file")
		if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_CANDIDATE_NOT_DIRECTORY") {
			t.Fatalf("Discover() error = %v", err)
		}
	})

	t.Run("ordinary files are ignored and contained symlinks are followed", func(t *testing.T) {
		home := t.TempDir()
		writeEvidence(t, home, ".cache/acme/not-a-version", "file")
		writeEvidence(t, home, ".local/acme-version/SKILL.md", "inside")
		link := filepath.Join(home, ".cache", "acme", "1.0.0")
		if err := os.Symlink(filepath.Join(home, ".local", "acme-version"), link); err != nil {
			t.Fatal(err)
		}
		report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home})
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		candidates := report.Candidates("acme/suite")
		if len(candidates) != 1 || candidates[0].Version != "1.0.0" || candidates[0].Location != physicalPath(t, filepath.Join(home, ".local", "acme-version")) {
			t.Fatalf("Candidates() = %#v", candidates)
		}
	})

	t.Run("escaping symlink is rejected", func(t *testing.T) {
		home := t.TempDir()
		prefix := filepath.Join(home, ".cache", "acme")
		if err := os.MkdirAll(prefix, 0o700); err != nil {
			t.Fatal(err)
		}
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(prefix, "1.0.0")); err != nil {
			t.Fatal(err)
		}
		if _, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: home}); err == nil || !strings.Contains(err.Error(), "DISCOVERY_PATH_ESCAPE") {
			t.Fatalf("Discover() error = %v", err)
		}
	})
}

func testCatalog(t *testing.T, probes ...catalog.DiscoveryProbe) catalog.Catalog {
	t.Helper()
	for i := range probes {
		if probes[i].Hosts == nil {
			probes[i].Hosts = []string{"codex"}
		}
		if probes[i].Surface == "" {
			probes[i].Surface = "codex-skills"
		}
		if probes[i].Distribution == "" {
			probes[i].Distribution = "acme"
		}
	}
	value, err := catalog.New([]catalog.ProviderDescriptorRecord{{
		SchemaVersion: catalog.ProviderDescriptorSchemaV3, DescriptorVersion: "3.0.0", ID: "acme/suite", DisplayName: "Acme Suite", Discovery: probes, Capabilities: []catalog.CapabilityRecord{},
	}}, []catalog.ProfileRecipeRecord{}, []catalog.ProfileAliasRecord{})
	if err != nil {
		t.Fatalf("catalog.New() error = %v", err)
	}
	return value
}

func writeEvidence(t *testing.T, root, relative, content string) string {
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

func contentDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func physicalPath(t *testing.T, value string) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(value)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(physical)
}
