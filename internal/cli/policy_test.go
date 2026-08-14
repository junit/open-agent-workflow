package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/policyengagement"
	"github.com/wifibaby4u/open-agent-workflow/internal/policyflow"
)

func TestPolicyProfilesExposeAllBridgeFreeBuiltIns(t *testing.T) {
	root := policyCLIHome(t)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profiles"}, &stdout, &stderr); status != 0 {
		t.Fatalf("profiles status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("profiles stderr=%q", stderr.String())
	}
	var result policyengagement.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode profiles: %v; output=%q", err, stdout.String())
	}
	want := []string{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"}
	if len(result.Profiles) != len(want) {
		t.Fatalf("profiles=%#v", result.Profiles)
	}
	for index, profile := range result.Profiles {
		if profile.Name != want[index] || !profile.PolicySelectable || !profile.HostRoutable {
			t.Fatalf("profile[%d]=%#v", index, profile)
		}
	}
	ecc := result.Profiles[2]
	if len(ecc.IncidentRoutes) == 0 {
		t.Fatal("ECC profile omitted conditional incident diagnostics")
	}
	if strings.Contains(stdout.String(), `"available"`) {
		t.Fatalf("profiles retained ambiguous availability field: %s", stdout.String())
	}
}

func TestPolicyUseAndBusinessEventsTraverseEveryProfile(t *testing.T) {
	for _, profile := range []policyflow.ProfileID{
		policyflow.ProfileSPFull,
		policyflow.ProfileMattFull,
		policyflow.ProfileECCFull,
		policyflow.ProfileMattSPHybrid,
	} {
		t.Run(string(profile), func(t *testing.T) {
			root := policyCLIHome(t)
			project := filepath.Join(root, "project")
			if err := os.MkdirAll(project, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(project)
			var stdout, stderr bytes.Buffer
			args := []string{"use", "--profile", string(profile), "--complexity", "complex", "--risk", "normal", "--", "editor"}
			if status := Run(args, &stdout, &stderr); status != 0 {
				t.Fatalf("use status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
			status := decodePublicStatus(t, stdout.Bytes())
			if status.Profile != string(profile) || status.State != "active" {
				t.Fatalf("initial status=%#v", status)
			}

			for steps := 0; steps < 96 && status.State == "active"; steps++ {
				stdout.Reset()
				stderr.Reset()
				var action string
				switch status.Next {
				case "invoke-skill", "invoke-user-skill", "host-action":
					action = "complete"
				case "review-skill", "review-user-skill", "review-host-action":
					action = "review"
				case "user-approval":
					action = "approve"
				case "host-confirmation":
					action = "satisfy"
				default:
					t.Fatalf("unknown active next=%#v", status)
				}
				command := []string{action}
				if action == "review" {
					command = append(command, "clean")
				}
				if code := Run(command, &stdout, &stderr); code != 0 {
					t.Fatalf("%s status=%d stdout=%q stderr=%q", action, code, stdout.String(), stderr.String())
				}
				status = decodePublicStatus(t, stdout.Bytes())
			}
			if status.State != "completed" {
				t.Fatalf("workflow did not complete: %#v", status)
			}
		})
	}
}

func TestPolicyStatusDoesNotExposeInternalState(t *testing.T) {
	root := policyCLIHome(t)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)

	var stdout, stderr bytes.Buffer
	if status := Run([]string{"use", "--profile=SP-FULL", "--complexity=complex", "--risk=normal", "--", "editor"}, &stdout, &stderr); status != 0 {
		t.Fatalf("use status=%d stderr=%q", status, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if status := Run([]string{"status"}, &stdout, &stderr); status != 0 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	lower := strings.ToLower(stdout.String())
	for _, forbidden := range []string{"route_inventory", "schema_version", "flow_id", "intent-", "policy-flow-", "project-"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("status leaked %q: %s", forbidden, stdout.String())
		}
	}
}

func TestPolicyReviewFindingsRequireRemediationAndFreshReview(t *testing.T) {
	root := policyCLIHome(t)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"use", "--profile=SP-FULL", "--complexity=complex", "--risk=normal", "--", "editor"}, &stdout, &stderr); code != 0 {
		t.Fatalf("use status=%d stderr=%q", code, stderr.String())
	}
	status := decodePublicStatus(t, stdout.Bytes())
	foundReview := false
	for steps := 0; steps < 64; steps++ {
		if (status.Next == "review-skill" || status.Next == "review-user-skill") &&
			status.Name == "superpowers:requesting-code-review" {
			foundReview = true
			break
		}
		if status.State != "active" {
			break
		}
		stdout.Reset()
		stderr.Reset()
		command := []string{"complete"}
		switch status.Next {
		case "review-skill", "review-user-skill":
			command = []string{"review", "clean"}
		case "user-approval":
			command = []string{"approve"}
		case "host-confirmation":
			command = []string{"satisfy"}
		}
		if code := Run(command, &stdout, &stderr); code != 0 {
			t.Fatalf("advance next=%q name=%q status=%d stderr=%q", status.Next, status.Name, code, stderr.String())
		}
		status = decodePublicStatus(t, stdout.Bytes())
	}
	if !foundReview {
		t.Fatalf("review status=%#v", status)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"complete"}, &stdout, &stderr); code != 65 || !strings.Contains(stderr.String(), "POLICY_ACTION_NOT_APPLICABLE") {
		t.Fatalf("generic completion status=%d stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"review", "findings"}, &stdout, &stderr); code != 0 {
		t.Fatalf("findings status=%d stderr=%q", code, stderr.String())
	}
	status = decodePublicStatus(t, stdout.Bytes())
	if status.Next != "invoke-skill" && status.Next != "invoke-user-skill" ||
		status.Name != "superpowers:executing-plans" {
		t.Fatalf("remediation status=%#v", status)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"complete"}, &stdout, &stderr); code != 0 {
		t.Fatalf("remediation status=%d stderr=%q", code, stderr.String())
	}
	status = decodePublicStatus(t, stdout.Bytes())
	if status.Next != "review-skill" && status.Next != "review-user-skill" ||
		status.Name != "superpowers:requesting-code-review" {
		t.Fatalf("re-review status=%#v", status)
	}
}

func TestPolicySubdirectoryUsesSameProjectEngagement(t *testing.T) {
	root := policyCLIHome(t)
	project := filepath.Join(root, "project")
	subdir := filepath.Join(project, "docs", "design")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	var stdout, stderr bytes.Buffer
	if status := Run([]string{"use", "--profile=SP-FULL", "--complexity=complex", "--risk=normal", "--", "editor"}, &stdout, &stderr); status != 0 {
		t.Fatalf("use status=%d stderr=%q", status, stderr.String())
	}
	t.Chdir(subdir)
	stdout.Reset()
	stderr.Reset()
	if status := Run([]string{"status"}, &stdout, &stderr); status != 0 {
		t.Fatalf("subdirectory status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if got := decodePublicStatus(t, stdout.Bytes()); got.Deliverable != "editor" {
		t.Fatalf("status=%#v", got)
	}
}

func TestPolicyProfilesDoesNotCreateEngagementState(t *testing.T) {
	root := policyCLIHome(t)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	var stdout, stderr bytes.Buffer
	if status := Run([]string{"profiles"}, &stdout, &stderr); status != 0 {
		t.Fatalf("profiles status=%d stderr=%q", status, stderr.String())
	}
	stateRoot := filepath.Join(root, "state", "open-agent-workflow", "policy-engagements")
	if _, err := os.Stat(stateRoot); !os.IsNotExist(err) {
		t.Fatalf("profiles created state root: %v", err)
	}
}

func TestPolicyOldInterfaceIsRemovedWithoutCompatibilitySimulation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := Run([]string{"policy", "start"}, &stdout, &stderr); status != 64 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "POLICY_INTERFACE_REMOVED") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestPolicyUseRequiresExplicitProfile(t *testing.T) {
	root := policyCLIHome(t)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	var stdout, stderr bytes.Buffer
	if status := Run([]string{"use", "--", "editor"}, &stdout, &stderr); status != 65 {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "PROFILE_SELECTION_REQUIRED") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	var result policyengagement.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode candidates: %v; stdout=%q", err, stdout.String())
	}
	if len(result.Profiles) != 4 {
		t.Fatalf("profiles=%#v", result.Profiles)
	}
}

func TestPolicyUseWithoutAssessmentReturnsTypedPolicyError(t *testing.T) {
	root := policyCLIHome(t)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	var stdout, stderr bytes.Buffer
	status := Run([]string{"use", "--profile", "SP-FULL", "--", "editor"}, &stdout, &stderr)
	if status != 65 || !strings.Contains(stderr.String(), "POLICY_ASSESSMENT_REQUIRED") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func decodePublicStatus(t *testing.T, raw []byte) policyengagement.Status {
	t.Helper()
	var result policyengagement.Result
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode status: %v; output=%q", err, raw)
	}
	if result.Status == nil {
		t.Fatalf("missing status: %s", raw)
	}
	return *result.Status
}

func policyCLIHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	config := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(config, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)
	writePolicyFixture(t, filepath.Join(home, ".codex", "config.toml"), "[plugins.\"superpowers@openai-api-curated\"]\nenabled = true\n[plugins.\"ecc@ecc\"]\nenabled = true\n")
	for _, skill := range []string{"grill-with-docs", "grilling", "domain-modeling", "to-spec", "to-tickets", "implement", "tdd", "diagnosing-bugs", "code-review"} {
		writePolicyFixture(t, filepath.Join(home, ".agents", "skills", skill, "SKILL.md"), "fixture")
	}
	for _, skill := range []string{"brainstorming", "writing-plans", "using-git-worktrees", "executing-plans", "test-driven-development", "systematic-debugging", "requesting-code-review", "receiving-code-review", "verification-before-completion", "finishing-a-development-branch"} {
		writePolicyFixture(t, filepath.Join(home, ".codex", "plugins", "cache", "openai-api-curated", "superpowers", "v1", "skills", skill, "SKILL.md"), "fixture")
	}
	for _, skill := range []string{"intent-driven-development", "product-capability", "blueprint", "git-workflow", "tdd-workflow", "verification-loop"} {
		writePolicyFixture(t, filepath.Join(home, ".codex", "plugins", "cache", "ecc", "ecc", "v1", "skills", skill, "SKILL.md"), "fixture")
	}
	return root
}

func writePolicyFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
