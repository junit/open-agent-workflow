package profileinspect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverIncludesBuiltInProjectAndUserProfiles(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	writeProfile(t, filepath.Join(project, ".oaw", "profiles", "project.md"), "shared", "Project Shared", "")
	writeProfile(t, filepath.Join(config, "open-agent-workflow", "profiles", "user.md"), "shared", "User Shared", "")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	workingDirectory := filepath.Join(project, "docs", "design")
	if err := os.MkdirAll(workingDirectory, 0o755); err != nil {
		t.Fatal(err)
	}

	inventory, err := Discover(Environment{WorkingDir: workingDirectory, Home: home, ConfigHome: config})
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Profiles) != 6 {
		t.Fatalf("Profiles = %#v", inventory.Profiles)
	}
	if !hasDiagnostic(inventory.Diagnostics, "PROFILE_ID_CROSS_SCOPE") {
		t.Fatalf("Diagnostics = %#v", inventory.Diagnostics)
	}
	projectProfile, err := Resolve(inventory, "project:shared")
	if err != nil || projectProfile.Source != SourceProject {
		t.Fatalf("Resolve(project:shared) = %#v, %v", projectProfile, err)
	}
	userProfile, err := Resolve(inventory, "user:shared")
	if err != nil || userProfile.Source != SourceUser {
		t.Fatalf("Resolve(user:shared) = %#v, %v", userProfile, err)
	}
	if _, err := Resolve(inventory, "shared"); err == nil {
		t.Fatal("Resolve(shared) accepted a cross-scope conflict")
	}
}

func TestInspectPathAllowsBuiltInIdentityWithoutGuessingOwnership(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "SP-FULL.md")
	writeProfile(t, profilePath, "SP-FULL", "Superpowers Full", "")
	profile, diagnostics, err := InspectPath(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Metadata.ID != "SP-FULL" || len(diagnostics) != 0 {
		t.Fatalf("InspectPath() = %#v, %#v", profile, diagnostics)
	}
}

func TestDiscoverReportsDuplicateMalformedAndReservedCustomProfiles(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	config := filepath.Join(root, "config")
	writeProfile(t, filepath.Join(project, ".oaw", "profiles", "one.md"), "duplicate", "One", "")
	writeProfile(t, filepath.Join(project, ".oaw", "profiles", "two.md"), "duplicate", "Two", "")
	writeProfile(t, filepath.Join(project, ".oaw", "profiles", "reserved.md"), "SP-FULL", "Not Built In", "")
	writeRaw(t, filepath.Join(config, "open-agent-workflow", "profiles", "malformed.md"), []byte("---\nname: Missing ID\n---\n"))

	inventory, err := Discover(Environment{WorkingDir: project, ConfigHome: config})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"PROFILE_ID_DUPLICATE", "PROFILE_METADATA_INVALID", "PROFILE_ID_RESERVED"} {
		if !hasDiagnostic(inventory.Diagnostics, code) {
			t.Errorf("Diagnostics omit %s: %#v", code, inventory.Diagnostics)
		}
	}
	if !inventory.HasErrors() {
		t.Fatal("inventory with structural diagnostics reports no errors")
	}
	builtIn, err := Resolve(inventory, "SP-FULL")
	if err != nil || builtIn.Source != SourceBuiltIn {
		t.Fatalf("reserved built-in resolution = %#v, %v", builtIn, err)
	}
	if _, err := Resolve(inventory, "project:SP-FULL"); err == nil {
		t.Fatal("reserved project Profile resolved")
	}
}

func TestDiscoverAcceptsPartialProfilesAndKeepsBodyWarningsAdvisory(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	body := "\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Implementation and TDD | `tdd` |\n| Unknown | `other` |\n"
	writeProfile(t, filepath.Join(project, ".oaw", "profiles", "partial.md"), "partial", "Partial", body)

	inventory, err := Discover(Environment{WorkingDir: project})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := Resolve(inventory, "partial")
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.Warnings) != 1 || profile.Metadata.ID != "partial" {
		t.Fatalf("partial Profile = %#v", profile)
	}
	if inventory.HasErrors() {
		t.Fatalf("advisory warning became an error: %#v", inventory.Diagnostics)
	}
}

func TestDiscoverReportsSymlinkedProfileWithoutReadingIt(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	outside := filepath.Join(root, "outside.md")
	writeProfile(t, outside, "outside", "Outside", "")
	profilePath := filepath.Join(project, ".oaw", "profiles", "linked.md")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, profilePath); err != nil {
		t.Fatal(err)
	}

	inventory, err := Discover(Environment{WorkingDir: project})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDiagnostic(inventory.Diagnostics, "PROFILE_FILE_INVALID") {
		t.Fatalf("Diagnostics = %#v", inventory.Diagnostics)
	}
	for _, profile := range inventory.Profiles {
		if profile.Metadata.ID == "outside" {
			t.Fatal("symlinked Profile was read")
		}
	}
}

func TestDiscoverRejectsSymlinkedProfileDirectoryComponents(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(filepath.Join(outside, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(project, ".oaw")); err != nil {
		t.Fatal(err)
	}

	if _, err := Discover(Environment{WorkingDir: project}); err == nil || !strings.Contains(err.Error(), "contains a symlink") {
		t.Fatalf("Discover() error = %v", err)
	}
}

func writeProfile(t *testing.T, path, id, name, body string) {
	t.Helper()
	writeRaw(t, path, []byte("---\nid: "+id+"\nname: "+name+"\n---\n"+body))
}

func writeRaw(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
