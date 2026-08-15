package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileListIncludesQualifiedSourcesAndAdvisoryConflicts(t *testing.T) {
	root, project, config := profileCLIFixture(t)
	writeCLIProfile(t, filepath.Join(project, ".oaw", "profiles", "project.md"), "shared", "Project Shared", "")
	writeCLIProfile(t, filepath.Join(config, "open-agent-workflow", "profiles", "user.md"), "shared", "User Shared", "")

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profile", "list"}, &stdout, &stderr); status != 0 {
		t.Fatalf("profile list status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	for _, literal := range []string{
		"Profile inspection is advisory",
		"built-in:SP-FULL",
		"project:shared",
		"user:shared",
		"PROFILE_ID_CROSS_SCOPE",
	} {
		if !strings.Contains(stdout.String(), literal) {
			t.Errorf("profile list omits %q:\n%s", literal, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("profile list stderr=%q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "state")); !os.IsNotExist(err) {
		t.Fatalf("profile list created state: %v", err)
	}
}

func TestProfileShowRequiresQualifierForCrossScopeIdentity(t *testing.T) {
	_, project, config := profileCLIFixture(t)
	writeCLIProfile(t, filepath.Join(project, ".oaw", "profiles", "project.md"), "shared", "Project Shared", "\nPROJECT BODY\n")
	writeCLIProfile(t, filepath.Join(config, "open-agent-workflow", "profiles", "user.md"), "shared", "User Shared", "\nUSER BODY\n")

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profile", "show", "shared"}, &stdout, &stderr); status != 65 {
		t.Fatalf("ambiguous show status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "source qualifier") {
		t.Fatalf("ambiguous show stderr=%q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if status := Run([]string{"profile", "show", "project:shared"}, &stdout, &stderr); status != 0 {
		t.Fatalf("qualified show status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	for _, literal := range []string{"source: project", "id: shared", "PROJECT BODY"} {
		if !strings.Contains(stdout.String(), literal) {
			t.Errorf("qualified show omits %q:\n%s", literal, stdout.String())
		}
	}
}

func TestProfileCheckAcceptsPartialProfileAndAdvisoryBodyWarnings(t *testing.T) {
	_, project, _ := profileCLIFixture(t)
	body := "\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Implementation and TDD | `tdd` |\n| Unknown | `other` |\n"
	writeCLIProfile(t, filepath.Join(project, ".oaw", "profiles", "partial.md"), "partial", "Partial", body)

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profile", "check", "partial"}, &stdout, &stderr); status != 0 {
		t.Fatalf("partial check status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	for _, literal := range []string{
		"Profile inspection is advisory",
		"result: metadata-valid",
		"responsibilities: 1/8",
		"Policy defaults cover omitted Responsibilities",
		"Skill availability: not evaluated",
		"PROFILE_BODY_WARNING",
	} {
		if !strings.Contains(stdout.String(), literal) {
			t.Errorf("partial check omits %q:\n%s", literal, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("partial check stderr=%q", stderr.String())
	}
}

func TestProfileCheckDoesNotMistakeMarkdownLikeIDForAPath(t *testing.T) {
	_, project, _ := profileCLIFixture(t)
	writeCLIProfile(t, filepath.Join(project, ".oaw", "profiles", "delivery.md"), "delivery.md", "Delivery", "")

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profile", "check", "delivery.md"}, &stdout, &stderr); status != 0 {
		t.Fatalf("Profile ID check status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "id: delivery.md") {
		t.Fatalf("Profile ID check output=%q", stdout.String())
	}
}

func TestProfileCheckReportsMalformedMetadataAndReservedIDs(t *testing.T) {
	_, project, _ := profileCLIFixture(t)
	malformed := filepath.Join(project, "malformed.md")
	if err := os.WriteFile(malformed, []byte("---\nname: Missing ID\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCLIProfile(t, filepath.Join(project, ".oaw", "profiles", "reserved.md"), "SP-FULL", "Reserved", "")

	for _, args := range [][]string{
		{"profile", "check", malformed},
		{"profile", "check", "project:SP-FULL"},
	} {
		var stdout, stderr bytes.Buffer
		if status := Run(args, &stdout, &stderr); status != 65 {
			t.Fatalf("Run(%v) status=%d stdout=%q stderr=%q", args, status, stdout.String(), stderr.String())
		}
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, "PROFILE_") {
			t.Fatalf("Run(%v) omits Profile diagnostic: %s", args, combined)
		}
	}
}

func TestProfileCheckExistingPathDoesNotDependOnInventoryRoots(t *testing.T) {
	_, project, _ := profileCLIFixture(t)
	profilePath := filepath.Join(project, "standalone.md")
	writeCLIProfile(t, profilePath, "standalone", "Standalone", "")
	t.Setenv("XDG_CONFIG_HOME", "relative-config")

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profile", "check", profilePath}, &stdout, &stderr); status != 0 {
		t.Fatalf("standalone path check status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "id: standalone") {
		t.Fatalf("standalone path check output=%q", stdout.String())
	}
}

func TestProfileCommandRejectsInvalidArgumentsWithoutWrites(t *testing.T) {
	root, _, _ := profileCLIFixture(t)
	for _, args := range [][]string{
		{"profile"},
		{"profile", "unknown"},
		{"profile", "list", "extra"},
		{"profile", "show"},
		{"profile", "check", "one", "two"},
	} {
		var stdout, stderr bytes.Buffer
		if status := Run(args, &stdout, &stderr); status != 64 {
			t.Fatalf("Run(%v) status=%d stdout=%q stderr=%q", args, status, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "oaw: error:") {
			t.Fatalf("Run(%v) stderr=%q", args, stderr.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, "state")); !os.IsNotExist(err) {
		t.Fatalf("invalid profile commands created state: %v", err)
	}
}

func TestProfileHelpIsVisibleAndReadOnly(t *testing.T) {
	root, _, _ := profileCLIFixture(t)
	for _, args := range [][]string{{"profile", "--help"}, {"profile", "list", "--help"}} {
		var stdout, stderr bytes.Buffer
		if status := Run(args, &stdout, &stderr); status != 0 {
			t.Fatalf("Run(%v) status=%d stdout=%q stderr=%q", args, status, stdout.String(), stderr.String())
		}
		for _, command := range []string{"profile list", "profile show", "profile check"} {
			if !strings.Contains(stdout.String(), command) {
				t.Errorf("Run(%v) help omits %q: %s", args, command, stdout.String())
			}
		}
	}
	var stdout, stderr bytes.Buffer
	if status := Run(nil, &stdout, &stderr); status != 0 || !strings.Contains(stdout.String(), "profile list") {
		t.Fatalf("root help status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "state")); !os.IsNotExist(err) {
		t.Fatalf("Profile help created state: %v", err)
	}
}

func profileCLIFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	for _, directory := range []string{project, home, config} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(project)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	return root, project, config
}

func writeCLIProfile(t *testing.T, path, id, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: " + id + "\nname: " + name + "\n---\n" + body
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
