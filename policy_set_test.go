package oaw

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCanonicalPolicySetMatchesRepositoryAndReturnsCopies(t *testing.T) {
	wantPaths := []string{
		"POLICY.md",
		"adapters/codex-policy.md",
		"cooperative-protocol.md",
		"profiles/ECC-FULL.md",
		"profiles/MATT-FULL.md",
		"profiles/MATT-SP-HYBRID.md",
		"profiles/SP-FULL.md",
	}

	first := CanonicalPolicySet()
	if err := ValidatePolicySet(first); err != nil {
		t.Fatalf("ValidatePolicySet(CanonicalPolicySet()) error = %v", err)
	}
	gotPaths := make([]string, 0, len(first))
	for _, file := range first {
		gotPaths = append(gotPaths, file.Path)
		want, err := os.ReadFile(filepath.Join("policy", filepath.FromSlash(file.Path)))
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", file.Path, err)
		}
		if !bytes.Equal(file.Content, want) {
			t.Errorf("CanonicalPolicySet file %q differs from repository content", file.Path)
		}
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("CanonicalPolicySet paths = %q, want %q", gotPaths, wantPaths)
	}

	first[0].Content[0] ^= 0xff
	second := CanonicalPolicySet()
	if bytes.Equal(first[0].Content, second[0].Content) {
		t.Fatal("CanonicalPolicySet returned shared mutable content")
	}
}

func TestBuiltInProfilesUseTheCustomProfileContract(t *testing.T) {
	wantIDs := map[string]bool{
		"SP-FULL":        true,
		"MATT-FULL":      true,
		"ECC-FULL":       true,
		"MATT-SP-HYBRID": true,
	}
	for _, file := range CanonicalPolicySet() {
		if !strings.HasPrefix(file.Path, "profiles/") {
			continue
		}
		profile, err := ParsePolicyProfile(file.Path, file.Content)
		if err != nil {
			t.Errorf("ParsePolicyProfile(%q) error = %v", file.Path, err)
			continue
		}
		if !wantIDs[profile.ID] {
			t.Errorf("unexpected built-in Profile ID %q", profile.ID)
		}
		delete(wantIDs, profile.ID)
		for _, responsibility := range PolicyResponsibilities() {
			if strings.TrimSpace(profile.Responsibilities[responsibility]) == "" {
				t.Errorf("Profile %q does not define %q", profile.ID, responsibility)
			}
		}
	}
	if len(wantIDs) != 0 {
		t.Fatalf("missing built-in Profile IDs: %v", wantIDs)
	}

	partial := []byte("---\nid: team-delivery\nname: Team Delivery\n---\n\n# Team Delivery\n\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Implementation and TDD | `tdd` |\n")
	profile, err := ParsePolicyProfile("profiles/team-delivery.md", partial)
	if err != nil {
		t.Fatalf("ParsePolicyProfile(partial Custom Profile) error = %v", err)
	}
	if profile.ID != "team-delivery" || profile.Name != "Team Delivery" {
		t.Fatalf("partial Custom Profile metadata = %#v", profile)
	}
	if len(profile.Responsibilities) != 1 {
		t.Fatalf("partial Custom Profile Responsibilities = %#v", profile.Responsibilities)
	}
}

func TestInspectPolicyProfileKeepsResponsibilityDiagnosticsAdvisory(t *testing.T) {
	content := []byte("---\nid: team-delivery\nname: Team Delivery\n---\n\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Implementation and TDD | `tdd` |\n| Unknown responsibility | `other` |\n| Implementation and TDD | `duplicate` |\n| Fresh verification | |\n")
	profile, warnings, err := InspectPolicyProfile("team-delivery.md", content)
	if err != nil {
		t.Fatalf("InspectPolicyProfile() error = %v", err)
	}
	if profile.ID != "team-delivery" || profile.Name != "Team Delivery" {
		t.Fatalf("profile = %#v", profile)
	}
	if got := profile.Responsibilities[ImplementationAndTDD]; got != "`tdd`" {
		t.Fatalf("Implementation and TDD = %q", got)
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings = %q", warnings)
	}
	if _, err := ParsePolicyProfile("team-delivery.md", content); err == nil {
		t.Fatal("strict ParsePolicyProfile accepted advisory table diagnostics")
	}
}

func TestInspectPolicyProfileDerivesOrderedReferenceOccurrences(t *testing.T) {
	content := []byte("---\nid: ordered\nname: Ordered\n---\n\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Problem framing | `skill-a`, then `skill-b` |\n| Implementation and TDD | `skill-a` |\n\n## Rules\n\n- `not-a-responsibility-reference` remains normative prose.\n")
	profile, warnings, err := InspectPolicyProfile("ordered.md", content)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("InspectPolicyProfile() = %#v, %q, %v", profile, warnings, err)
	}
	wantReferences := []string{"skill-a", "skill-b", "skill-a"}
	gotReferences := make([]string, 0, len(profile.Occurrences))
	seenRefs := make(map[string]bool)
	for _, occurrence := range profile.Occurrences {
		gotReferences = append(gotReferences, occurrence.Reference)
		if !strings.HasPrefix(occurrence.Ref, "profile-occurrence:sha256:") {
			t.Errorf("occurrence Ref = %q", occurrence.Ref)
		}
		if seenRefs[occurrence.Ref] {
			t.Errorf("duplicate occurrence Ref %q", occurrence.Ref)
		}
		seenRefs[occurrence.Ref] = true
	}
	if !slices.Equal(gotReferences, wantReferences) {
		t.Fatalf("occurrence references = %q, want %q", gotReferences, wantReferences)
	}
	if profile.Occurrences[0].Responsibility != ProblemFraming ||
		profile.Occurrences[2].Responsibility != ImplementationAndTDD {
		t.Fatalf("occurrence Responsibilities = %#v", profile.Occurrences)
	}
	again, _, err := InspectPolicyProfile("ordered.md", bytes.Clone(content))
	if err != nil || !slices.Equal(profile.Occurrences, again.Occurrences) {
		t.Fatalf("occurrences are not deterministic: %#v, %#v, %v", profile.Occurrences, again.Occurrences, err)
	}
}

func TestInspectPolicyProfileRejectsMalformedRequiredMetadata(t *testing.T) {
	for _, content := range [][]byte{
		[]byte("# no frontmatter\n"),
		[]byte("---\nname: Missing ID\n---\n"),
		[]byte("---\nid: missing-name\n---\n"),
		[]byte("---\nid: bad-id\nname: \"Bad\\nName\"\n---\n"),
	} {
		if _, _, err := InspectPolicyProfile("invalid.md", content); err == nil {
			t.Fatalf("InspectPolicyProfile accepted %q", content)
		}
	}
}

func TestBuiltInProfilesPreserveTheirDeclaredMethodOwnership(t *testing.T) {
	wantAssignments := map[string]map[Responsibility][]string{
		"SP-FULL": {
			ProblemFraming:       {"superpowers:brainstorming"},
			ImplementationAndTDD: {"superpowers:executing-plans", "superpowers:test-driven-development"},
		},
		"MATT-FULL": {
			ProblemFraming:       {"grill-with-docs"},
			DeliveryPlanning:     {"to-tickets"},
			ImplementationAndTDD: {"implement", "tdd"},
		},
		"ECC-FULL": {
			ProblemFraming:       {"ecc:intent-driven-development"},
			ImplementationAndTDD: {"ecc:tdd-workflow"},
		},
		"MATT-SP-HYBRID": {
			DeliveryPlanning:     {"to-tickets", "superpowers:writing-plans"},
			ImplementationAndTDD: {"superpowers:executing-plans", "tdd"},
		},
	}

	for _, file := range CanonicalPolicySet() {
		if !strings.HasPrefix(file.Path, "profiles/") {
			continue
		}
		profile, err := ParsePolicyProfile(file.Path, file.Content)
		if err != nil {
			t.Fatalf("ParsePolicyProfile(%q) error = %v", file.Path, err)
		}
		for responsibility, literals := range wantAssignments[profile.ID] {
			assignment := profile.Responsibilities[responsibility]
			for _, literal := range literals {
				if !strings.Contains(assignment, literal) {
					t.Errorf("Profile %q assignment for %q = %q, want it to contain %q",
						profile.ID, responsibility, assignment, literal)
				}
			}
		}
	}
}

func TestECCFullUsesBlueprintOnlyWithinItsDeclaredTrigger(t *testing.T) {
	for _, file := range CanonicalPolicySet() {
		if file.Path != "profiles/ECC-FULL.md" {
			continue
		}
		profile, err := ParsePolicyProfile(file.Path, file.Content)
		if err != nil {
			t.Fatal(err)
		}
		assignment := profile.Responsibilities[DeliveryPlanning]
		for _, fragment := range []string{"Policy Default", "ordinary or single-delivery", "ecc:blueprint", "complex multi-session"} {
			if !strings.Contains(assignment, fragment) {
				t.Errorf("ECC-FULL Delivery planning = %q, want it to contain %q", assignment, fragment)
			}
		}
		return
	}
	t.Fatal("ECC-FULL Profile is missing")
}

func TestValidatePolicySetRejectsDuplicateProfileIdentity(t *testing.T) {
	files := CanonicalPolicySet()
	for _, file := range files {
		if file.Path == "profiles/SP-FULL.md" {
			files = append(files, PolicyFile{Path: "profiles/duplicate.md", Content: bytes.Clone(file.Content)})
			break
		}
	}

	err := ValidatePolicySet(files)
	if err == nil || !strings.Contains(err.Error(), "duplicate Profile ID") {
		t.Fatalf("ValidatePolicySet duplicate error = %v", err)
	}
}

func TestValidatePolicySetRejectsMissingBuiltInResponsibility(t *testing.T) {
	files := CanonicalPolicySet()
	for index := range files {
		if files[index].Path == "profiles/SP-FULL.md" {
			files[index].Content = bytes.Replace(files[index].Content,
				[]byte("| Closeout | `superpowers:finishing-a-development-branch` with user authority |\n"), nil, 1)
		}
	}

	err := ValidatePolicySet(files)
	if err == nil || !strings.Contains(err.Error(), "missing Responsibility") {
		t.Fatalf("ValidatePolicySet missing Responsibility error = %v", err)
	}
}

func TestValidatePolicySetRejectsBrokenLocalReference(t *testing.T) {
	files := CanonicalPolicySet()
	for index := range files {
		if files[index].Path == "POLICY.md" {
			files[index].Content = bytes.Replace(files[index].Content, []byte("cooperative-protocol.md"), []byte("missing-protocol.md"), 1)
		}
	}

	err := ValidatePolicySet(files)
	if err == nil || !strings.Contains(err.Error(), "missing local reference") {
		t.Fatalf("ValidatePolicySet broken reference error = %v", err)
	}
}

func TestHostSpecificDetailsLiveOnlyInTheCodexAdapter(t *testing.T) {
	forbidden := []string{
		".agents/skills",
		".codex/skills",
		".codex/plugins/cache",
		"openai-api-curated",
		"Codex /plan",
		"review-pr",
	}
	var adapter []byte
	for _, file := range CanonicalPolicySet() {
		if file.Path == "adapters/codex-policy.md" {
			adapter = file.Content
			continue
		}
		for _, literal := range forbidden {
			if bytes.Contains(file.Content, []byte(literal)) {
				t.Errorf("portable file %q contains Host-specific detail %q", file.Path, literal)
			}
		}
	}
	for _, literal := range forbidden {
		if !bytes.Contains(adapter, []byte(literal)) {
			t.Errorf("Codex Adapter is missing expected Host-specific detail %q", literal)
		}
	}
}
