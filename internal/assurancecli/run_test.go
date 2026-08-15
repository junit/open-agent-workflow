package assurancecli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assurance"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
)

func TestIssueAndVerifyOverlayThroughStandaloneCLI(t *testing.T) {
	project, config, profilePath := writeAssuranceCLIProfile(t)
	environment := Environment{WorkingDir: project, ConfigHome: config}
	profile := resolveAssuranceCLIProfile(t, environment, "project:team-delivery")
	index, err := assurance.Inspect(profile)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	request := assurance.IssueRequest{
		SchemaVersion: assurance.IssueRequestSchemaV1,
		Issuer:        "test-ci",
		Claims: []assurance.BindingClaim{
			assuranceCLIClaim(index.Occurrences[0].OccurrenceRef, index.Occurrences[0].BindingReference),
		},
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	var issued, issueErrors bytes.Buffer
	status := RunWithEnvironment(
		[]string{"overlay", "issue", "--profile", "project:team-delivery"},
		bytes.NewReader(requestJSON), &issued, &issueErrors, environment,
	)
	if status != 0 || issueErrors.Len() != 0 {
		t.Fatalf("issue status=%d stderr=%q", status, issueErrors.String())
	}
	overlay, err := assurance.DecodeOverlay(issued.Bytes())
	if err != nil {
		t.Fatalf("DecodeOverlay(%s): %v", issued.Bytes(), err)
	}
	if overlay.Profile.ID != "team-delivery" || overlay.Profile.Source != "project" {
		t.Fatalf("Overlay Profile = %#v", overlay.Profile)
	}

	var verified, verifyErrors bytes.Buffer
	status = RunWithEnvironment(
		[]string{"overlay", "verify", "--profile", "project:team-delivery"},
		bytes.NewReader(issued.Bytes()), &verified, &verifyErrors, environment,
	)
	if status != 0 || verifyErrors.Len() != 0 {
		t.Fatalf("verify status=%d stderr=%q", status, verifyErrors.String())
	}
	var result VerificationResult
	if err := json.Unmarshal(verified.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Valid || result.OverlayDigest != overlay.Digest || result.Profile != overlay.Profile {
		t.Fatalf("Verification result = %#v", result)
	}
	after, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Assurance CLI modified the Policy Profile")
	}
	if _, err := os.Stat(filepath.Join(project, ".oaw", "state")); !os.IsNotExist(err) {
		t.Fatalf("Assurance CLI created workflow state: %v", err)
	}
}

func TestAssuranceFailureDoesNotMakeProfileUnavailable(t *testing.T) {
	project, config, profilePath := writeAssuranceCLIProfile(t)
	environment := Environment{WorkingDir: project, ConfigHome: config}
	before, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := RunWithEnvironment(
		[]string{"overlay", "issue", "--profile", "project:team-delivery"},
		strings.NewReader(`{"schema_version":"wrong","issuer":"test-ci","claims":[]}`),
		&stdout, &stderr, environment,
	)
	if status != 65 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "ASSURANCE_INPUT_INVALID") {
		t.Fatalf("failure status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	profile := resolveAssuranceCLIProfile(t, environment, "project:team-delivery")
	if profile.Metadata.ID != "team-delivery" {
		t.Fatalf("Profile after Assurance failure = %#v", profile)
	}
	after, err := os.ReadFile(profilePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("Profile changed after Assurance failure: %v", err)
	}
}

func TestStandaloneCLIHelpHasOnlyOverlayCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := RunWithEnvironment([]string{"--help"}, strings.NewReader(""), &stdout, &stderr, Environment{})
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("help status=%d stderr=%q", status, stderr.String())
	}
	for _, expected := range []string{"oaw-assurance overlay inspect", "oaw-assurance overlay issue", "oaw-assurance overlay verify", "does not select or run a Policy Profile"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("help omits %q: %s", expected, stdout.String())
		}
	}
	for _, forbidden := range []string{"workflow exchange", "Lifecycle Bundle", "Resource Lease", "Receipt"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Errorf("help contains retired machine workflow term %q", forbidden)
		}
	}
}

func TestInspectListsOpaqueReferencesWithoutResponsibilitySemantics(t *testing.T) {
	project, config, _ := writeAssuranceCLIProfile(t)
	var stdout, stderr bytes.Buffer
	status := RunWithEnvironment(
		[]string{"overlay", "inspect", "--profile", "project:team-delivery"},
		strings.NewReader(""), &stdout, &stderr,
		Environment{WorkingDir: project, ConfigHome: config},
	)
	if status != 0 || stderr.Len() != 0 {
		t.Fatalf("inspect status=%d stderr=%q", status, stderr.String())
	}
	var index assurance.ReferenceIndex
	if err := json.Unmarshal(stdout.Bytes(), &index); err != nil {
		t.Fatal(err)
	}
	if index.SchemaVersion != assurance.ReferenceIndexSchemaV1 || len(index.Occurrences) != 1 ||
		index.Occurrences[0].BindingReference != "test-skill" ||
		!strings.HasPrefix(index.Occurrences[0].OccurrenceRef, "profile-occurrence:sha256:") {
		t.Fatalf("Reference index = %#v", index)
	}
	for _, forbidden := range []string{"responsibility", "rules", "order"} {
		if bytes.Contains(stdout.Bytes(), []byte(`"`+forbidden+`"`)) {
			t.Errorf("Reference index contains semantic field %q: %s", forbidden, stdout.Bytes())
		}
	}
}

func writeAssuranceCLIProfile(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	config := filepath.Join(root, "config")
	profilePath := filepath.Join(project, ".oaw", "profiles", "team-delivery.md")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("---\nid: team-delivery\nname: Team Delivery\n---\n\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Implementation and TDD | `test-skill` |\n\n## Rules\n\nKeep the original rules.\n")
	if err := os.WriteFile(profilePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return project, config, profilePath
}

func resolveAssuranceCLIProfile(t *testing.T, environment Environment, selector string) profileinspect.Profile {
	t.Helper()
	inventory, err := profileinspect.Discover(profileinspect.Environment{
		WorkingDir: environment.WorkingDir, Home: environment.Home, ConfigHome: environment.ConfigHome,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := profileinspect.Resolve(inventory, selector)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func assuranceCLIClaim(occurrenceRef, reference string) assurance.BindingClaim {
	digest := "sha256:" + strings.Repeat("a", 64)
	return assurance.BindingClaim{
		OccurrenceRef: occurrenceRef, ProviderID: "oaw/provider", DistributionID: "provider",
		DistributionRevision: strings.Repeat("a", 40), DistributionTreeDigest: digest,
		HostID: "codex", Surface: "codex-plugin", BindingID: "test-skill", BindingKind: "skill",
		BindingReference: reference, Invocation: "model", BindingContentDigest: digest,
		Evidence: []assurance.EvidenceReference{{
			Kind: "host-observation", Reference: "evidence://test/test-skill", Digest: digest,
		}},
	}
}
