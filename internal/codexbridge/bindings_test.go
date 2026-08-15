package codexbridge

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
)

func TestBuildBindingInventoryMatchesPathAndName(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	metadata := bindingMetadata(fixture.Home, "acme:review", filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/SKILL.md"))
	inventory, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil || len(diagnostics) != 0 || len(inventory.Observations) != 1 {
		t.Fatalf("inventory=%#v diagnostics=%#v err=%v", inventory, diagnostics, err)
	}
	observation := inventory.Observations[0]
	if observation.InstallationKey != fixture.Candidate.InstallationKey || observation.ProviderID != "acme/suite" ||
		observation.BindingID != "codex-review" || observation.Reference != "acme:review" ||
		observation.Source != "native-api" || len(observation.Topologies) != 1 || observation.EvidenceReference == "" ||
		observation.BindingTreeDigest != fixture.Candidate.BindingRoots[0].Tree.RootDigest {
		t.Fatalf("observation=%#v", observation)
	}
}

func TestBindingTreeRejectsSiblingDriftAfterDiscovery(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	path := filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/SKILL.md")
	writeSkillFixture(t, filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/reference.md"), "drift")
	inventory, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, bindingMetadata(fixture.Home, "acme:review", path), fixture.Home)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Observations) != 0 || !hasDiagnostic(diagnostics, "PROVIDER_BINDING_CONTENT_MISMATCH") {
		t.Fatalf("inventory=%#v diagnostics=%#v", inventory, diagnostics)
	}
}

func TestLiveBindingRootUsesRootedDigestForRegularFile(t *testing.T) {
	installation := t.TempDir()
	writeSkillFixture(t, filepath.Join(installation, "agents/reviewer.md"), "reviewer")
	candidate := discovery.Candidate{DiagnosticLocation: installation}
	binding := catalog.BindingRecord{InstallRoot: "agents/reviewer.md", ContentRoot: "agents/reviewer.md"}

	got, err := digestLiveBindingRoot(candidate, binding)
	if err != nil {
		t.Fatalf("digestLiveBindingRoot() error = %v", err)
	}
	want, err := integrity.DigestBindingRoot(installation, binding.InstallRoot, binding.ContentRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got.RootDigest != want.RootDigest || len(got.Entries) != 1 || got.Entries[0].Path != binding.ContentRoot {
		t.Fatalf("live evidence = %#v, want %#v", got, want)
	}
}

func TestBindingTreeRejectsSameNameOutsideExactInstallRoot(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	for _, relative := range []string{"skills/foreign/SKILL.md", "skills/SKILL.md"} {
		path := filepath.Join(fixture.Candidate.DiagnosticLocation, filepath.FromSlash(relative))
		writeSkillFixture(t, path, "acme:review")
		inventory, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, bindingMetadata(fixture.Home, "acme:review", path), fixture.Home)
		if err != nil {
			t.Fatal(err)
		}
		if len(inventory.Observations) != 0 || !hasDiagnostic(diagnostics, "HOST_BINDING_INSTALL_ROOT_MISMATCH") {
			t.Fatalf("path=%s inventory=%#v diagnostics=%#v", relative, inventory, diagnostics)
		}
	}
}

func TestBindingTreeRejectsSymlinkedSkillPath(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	aliasRoot := filepath.Join(fixture.Candidate.DiagnosticLocation, "alias")
	if err := os.MkdirAll(aliasRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(aliasRoot, "SKILL.md")
	if err := os.Symlink(filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/SKILL.md"), alias); err != nil {
		t.Fatal(err)
	}
	inventory, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, bindingMetadata(fixture.Home, "acme:review", alias), fixture.Home)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Observations) != 0 || !hasDiagnostic(diagnostics, "PROVIDER_BINDING_CONTENT_MISMATCH") {
		t.Fatalf("inventory=%#v diagnostics=%#v", inventory, diagnostics)
	}
}

func TestBuildBindingInventoryRejectsDisabledOrphanAndUnboundSkills(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	orphan := filepath.Join(fixture.Home, "unowned", "SKILL.md")
	unbound := filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/unbound/SKILL.md")
	writeSkillFixture(t, orphan, "orphan")
	writeSkillFixture(t, unbound, "unbound")
	if err := os.Truncate(orphan, maximumSkillEvidenceBytes+1); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(unbound, maximumSkillEvidenceBytes+1); err != nil {
		t.Fatal(err)
	}
	metadata := appserver.MetadataObservation{Skills: appserver.SkillsEntry{CWD: fixture.Home, Skills: []appserver.SkillMetadata{
		{Name: "acme:review", Enabled: false, Path: filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/SKILL.md"), Scope: "user"},
		{Name: "orphan", Enabled: true, Path: orphan, Scope: "user"},
		{Name: "unbound", Enabled: true, Path: unbound, Scope: "user"},
	}}}
	inventory, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Observations) != 0 {
		t.Fatalf("rejected Skills produced observations: %#v", inventory.Observations)
	}
	if !hasDiagnostic(diagnostics, "HOST_SKILL_ORPHAN") || !hasDiagnostic(diagnostics, "HOST_BINDING_EVIDENCE_REQUIRED") {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestBuildBindingInventoryAggregatesRepeatedSkillDiagnosticsWithOwnership(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	unbound := filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/unbound/SKILL.md")
	writeSkillFixture(t, unbound, "unbound")
	metadata := appserver.MetadataObservation{Skills: appserver.SkillsEntry{CWD: fixture.Home, Skills: []appserver.SkillMetadata{
		{Name: "unbound", Enabled: true, Path: unbound, Scope: "user"},
		{Name: "unbound", Enabled: true, Path: unbound, Scope: "user"},
		{Name: "unbound", Enabled: true, Path: unbound, Scope: "user"},
	}}}

	_, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "HOST_BINDING_EVIDENCE_REQUIRED" {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	if !slices.Equal(diagnostics[0].AffectedProviders, []string{"acme/suite"}) {
		t.Fatalf("affected providers=%q", diagnostics[0].AffectedProviders)
	}
}

func TestBuildBindingInventoryRejectsAmbiguousInstallation(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	providers := fixture.Catalog.Providers()
	shadow := providers[0].Discovery[0]
	shadow.ID = "codex-shadow"
	shadow.Surface = "codex-plugin-shadow"
	providers[0].Discovery = append(providers[0].Discovery, shadow)
	value, err := catalog.New(providers, fixture.Catalog.Recipes(), fixture.Catalog.Aliases())
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(value, discovery.Options{HostID: "codex", UserHome: fixture.Home})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates("acme/suite")) != 2 {
		t.Fatalf("candidates=%#v", report.Candidates("acme/suite"))
	}
	metadata := bindingMetadata(fixture.Home, "acme:review", filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/SKILL.md"))
	inventory, diagnostics, err := BuildBindingInventory(value, report, metadata, fixture.Home)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Observations) != 0 || !hasDiagnostic(diagnostics, "HOST_SKILL_INSTALLATION_AMBIGUOUS") {
		t.Fatalf("inventory=%#v diagnostics=%#v", inventory, diagnostics)
	}
}

func TestBindingTreeDriftFailsClosedInsteadOfRewritingEvidence(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	path := filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/SKILL.md")
	metadata := bindingMetadata(fixture.Home, "acme:review", path)
	first, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil || len(diagnostics) != 0 || len(first.Observations) != 1 {
		t.Fatalf("first=%#v diagnostics=%#v err=%v", first, diagnostics, err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\nchanged\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	second, diagnostics, err := BuildBindingInventory(fixture.Catalog, fixture.Discovery, metadata, fixture.Home)
	if err != nil || len(second.Observations) != 0 || !hasDiagnostic(diagnostics, "PROVIDER_BINDING_CONTENT_MISMATCH") {
		t.Fatalf("second=%#v diagnostics=%#v err=%v", second, diagnostics, err)
	}
	if first.Digest == second.Digest {
		t.Fatal("drifted inventory retained the trusted inventory digest")
	}
}

func TestValidateSkillIdentityRejectsMalformedNameAndUnknownScope(t *testing.T) {
	for _, value := range []appserver.SkillMetadata{
		{Name: "", Scope: "user"},
		{Name: " leading", Scope: "user"},
		{Name: "bad\nname", Scope: "repo"},
		{Name: "acme:review", Scope: "workspace"},
	} {
		if err := validateSkillIdentity(value.Name, value.Scope); err == nil {
			t.Fatalf("accepted %#v", value)
		}
	}
	for _, scope := range []string{"user", "repo", "system", "admin"} {
		if err := validateSkillIdentity("acme:review", scope); err != nil {
			t.Fatalf("scope %q: %v", scope, err)
		}
	}
}

func TestCandidatePathRejectsForeignHost(t *testing.T) {
	fixture := hosttest.BuildProviderFixture(t)
	candidate := fixture.Candidate
	candidate.HostID = "claude"
	path := filepath.Join(fixture.Candidate.DiagnosticLocation, "skills/review/SKILL.md")
	if candidateContainsPath(candidate, "codex", path) {
		t.Fatal("foreign Host Candidate matched Codex Skill")
	}
}

func bindingMetadata(cwd, name, path string) appserver.MetadataObservation {
	return appserver.MetadataObservation{Skills: appserver.SkillsEntry{CWD: cwd, Skills: []appserver.SkillMetadata{{Name: name, Enabled: true, Path: path, Scope: "user"}}}}
}

func writeSkillFixture(t *testing.T, path, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: "+name+"\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func hasDiagnostic(values []Diagnostic, code string) bool {
	return slices.ContainsFunc(values, func(value Diagnostic) bool { return value.Code == code })
}
