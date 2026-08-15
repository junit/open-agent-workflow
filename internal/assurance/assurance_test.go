package assurance

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
)

func TestIssueAndVerifyOverlayPreserveProfileSemantics(t *testing.T) {
	profile := assuranceTestProfile(t)
	before := cloneAssuranceTestProfile(profile)
	occurrences := assuranceTestOccurrences(t, profile)
	first, second := occurrences[0], occurrences[1]
	request := IssueRequest{
		SchemaVersion: IssueRequestSchemaV1,
		Issuer:        "test-ci",
		Claims: []BindingClaim{
			assuranceTestClaim(second.OccurrenceRef, second.BindingReference, "binding-b", 'b'),
			assuranceTestClaim(first.OccurrenceRef, first.BindingReference, "binding-a", 'a'),
		},
	}

	overlay, err := Issue(profile, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(profile, before) {
		t.Fatalf("Issue mutated the Profile\nbefore: %#v\nafter:  %#v", before, profile)
	}
	if overlay.SchemaVersion != OverlaySchemaV1 || overlay.Profile.Source != string(profile.Source) ||
		overlay.Profile.ID != profile.Metadata.ID || overlay.Profile.ContentDigest != digestBytes(profile.Content) {
		t.Fatalf("Overlay Profile reference = %#v", overlay.Profile)
	}
	if len(overlay.Claims) != 2 || overlay.Claims[0].OccurrenceRef != first.OccurrenceRef ||
		overlay.Claims[1].OccurrenceRef != second.OccurrenceRef {
		t.Fatalf("Overlay claims do not follow Profile order: %#v", overlay.Claims)
	}
	if overlay.Digest == "" {
		t.Fatal("Overlay has no artifact digest")
	}
	if err := Verify(profile, overlay); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestOverlayCannotRedefineResponsibilitiesSkillsOrderOrRules(t *testing.T) {
	profile := assuranceTestProfile(t)
	occurrences := assuranceTestOccurrences(t, profile)
	first, second := occurrences[0], occurrences[1]
	valid := IssueRequest{
		SchemaVersion: IssueRequestSchemaV1,
		Issuer:        "test-ci",
		Claims: []BindingClaim{
			assuranceTestClaim(first.OccurrenceRef, first.BindingReference, "binding-a", 'a'),
			assuranceTestClaim(second.OccurrenceRef, second.BindingReference, "binding-b", 'b'),
		},
	}

	t.Run("Skill substitution", func(t *testing.T) {
		request := valid
		request.Claims = cloneClaims(valid.Claims)
		request.Claims[0].BindingReference = "different-skill"
		if _, err := Issue(profile, request); err == nil || !strings.Contains(err.Error(), "Binding reference") {
			t.Fatalf("Issue() substitution error = %v", err)
		}
	})

	t.Run("Responsibility ownership", func(t *testing.T) {
		request := valid
		request.Claims = cloneClaims(valid.Claims)
		request.Claims[0].OccurrenceRef = second.OccurrenceRef
		request.Claims[0].BindingReference = second.BindingReference
		if _, err := Issue(profile, request); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("Issue() duplicate occurrence error = %v", err)
		}
	})

	t.Run("Order", func(t *testing.T) {
		overlay, err := Issue(profile, valid)
		if err != nil {
			t.Fatal(err)
		}
		overlay.Claims[0], overlay.Claims[1] = overlay.Claims[1], overlay.Claims[0]
		overlay.Digest, err = overlayDigest(overlay)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(profile, overlay); err == nil || !strings.Contains(err.Error(), "Profile order") {
			t.Fatalf("Verify() reordered claims error = %v", err)
		}
	})

	t.Run("Rules", func(t *testing.T) {
		overlay, err := Issue(profile, valid)
		if err != nil {
			t.Fatal(err)
		}
		changed := cloneAssuranceTestProfile(profile)
		changed.Content = bytes.Replace(changed.Content, []byte("Original rule."), []byte("Changed rule."), 1)
		parsed, _, err := oaw.InspectPolicyProfile(changed.Path, changed.Content)
		if err != nil {
			t.Fatal(err)
		}
		changed.Metadata = parsed
		if err := Verify(changed, overlay); err == nil || !strings.Contains(err.Error(), "Profile reference") {
			t.Fatalf("Verify() changed rules error = %v", err)
		}
	})
}

func TestStrictDecodersRejectSemanticAndUnknownFields(t *testing.T) {
	profile := assuranceTestProfile(t)
	occurrence := assuranceTestOccurrences(t, profile)[0]
	request := IssueRequest{
		SchemaVersion: IssueRequestSchemaV1,
		Issuer:        "test-ci",
		Claims: []BindingClaim{assuranceTestClaim(
			occurrence.OccurrenceRef, occurrence.BindingReference, "binding-a", 'a',
		)},
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	requestJSON = bytes.Replace(requestJSON, []byte(`"issuer":"test-ci"`),
		[]byte(`"issuer":"test-ci","responsibilities":["replacement"]`), 1)
	if _, err := DecodeIssueRequest(requestJSON); err == nil {
		t.Fatal("DecodeIssueRequest accepted Responsibilities")
	}

	overlay, err := Issue(profile, request)
	if err != nil {
		t.Fatal(err)
	}
	overlayJSON, err := json.Marshal(overlay)
	if err != nil {
		t.Fatal(err)
	}
	overlayJSON = bytes.Replace(overlayJSON, []byte(`"issuer":"test-ci"`),
		[]byte(`"issuer":"test-ci","rules":"replacement"`), 1)
	if _, err := DecodeOverlay(overlayJSON); err == nil {
		t.Fatal("DecodeOverlay accepted Rules")
	}

	encoded, err := json.Marshal(overlay)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"responsibilities", "skill_composition", "order", "rules", "add_ons", "risk", "request_mode", "topology", "progress", "completion"} {
		if bytes.Contains(encoded, []byte(`"`+forbidden+`"`)) {
			t.Errorf("Overlay contains forbidden semantic field %q: %s", forbidden, encoded)
		}
	}
}

func TestInspectConsumesAllBuiltInMarkdownProfiles(t *testing.T) {
	inventory, err := profileinspect.Discover(profileinspect.Environment{
		WorkingDir: t.TempDir(), ConfigHome: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"SP-FULL", "MATT-FULL", "ECC-FULL", "MATT-SP-HYBRID"} {
		profile, err := profileinspect.Resolve(inventory, "built-in:"+id)
		if err != nil {
			t.Fatalf("Resolve(%s): %v", id, err)
		}
		index, err := Inspect(profile)
		if err != nil {
			t.Fatalf("Inspect(%s): %v", id, err)
		}
		if index.Profile.ID != id || index.Profile.Source != "built-in" ||
			!strings.HasPrefix(index.Profile.ContentDigest, "sha256:") || len(index.Occurrences) == 0 {
			t.Errorf("Reference index for %s = %#v", id, index)
		}
	}
}

func TestOccurrenceReferencesAreScopedToTheSourceQualifiedProfile(t *testing.T) {
	projectProfile := assuranceTestProfile(t)
	userProfile := cloneAssuranceTestProfile(projectProfile)
	userProfile.Source = profileinspect.SourceUser

	projectIndex, err := Inspect(projectProfile)
	if err != nil {
		t.Fatal(err)
	}
	userIndex, err := Inspect(userProfile)
	if err != nil {
		t.Fatal(err)
	}
	if projectIndex.Occurrences[0].OccurrenceRef == userIndex.Occurrences[0].OccurrenceRef {
		t.Fatal("identical Markdown in different Profile sources shared an occurrence reference")
	}

	claim := assuranceTestClaim(
		projectIndex.Occurrences[0].OccurrenceRef,
		projectIndex.Occurrences[0].BindingReference,
		"binding-a",
		'a',
	)
	if _, err := Issue(userProfile, IssueRequest{
		SchemaVersion: IssueRequestSchemaV1,
		Issuer:        "test-ci",
		Claims:        []BindingClaim{claim},
	}); err == nil || !strings.Contains(err.Error(), "not in the selected Profile") {
		t.Fatalf("Issue() cross-Profile occurrence error = %v", err)
	}
}

func assuranceTestProfile(t *testing.T) profileinspect.Profile {
	t.Helper()
	content := []byte("---\nid: team-delivery\nname: Team Delivery\n---\n\n## Responsibilities\n\n| Responsibility | Skill or action |\n| --- | --- |\n| Problem framing | `skill-a`, then `skill-b` |\n| Implementation and TDD | `skill-c` |\n\n## Rules\n\nOriginal rule.\n")
	metadata, warnings, err := oaw.InspectPolicyProfile("team-delivery.md", content)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("InspectPolicyProfile() = %#v, %q, %v", metadata, warnings, err)
	}
	return profileinspect.Profile{
		Source:   profileinspect.SourceProject,
		Path:     "/project/.oaw/profiles/team-delivery.md",
		Content:  bytes.Clone(content),
		Metadata: metadata,
	}
}

func assuranceTestOccurrences(t *testing.T, profile profileinspect.Profile) []ReferenceOccurrence {
	t.Helper()
	index, err := Inspect(profile)
	if err != nil {
		t.Fatal(err)
	}
	return index.Occurrences
}

func assuranceTestClaim(occurrenceRef, bindingReference, bindingID string, digestByte byte) BindingClaim {
	digest := "sha256:" + strings.Repeat(string(digestByte), 64)
	return BindingClaim{
		OccurrenceRef:          occurrenceRef,
		ProviderID:             "oaw/provider",
		DistributionID:         "provider",
		DistributionRevision:   strings.Repeat(string(digestByte), 40),
		DistributionTreeDigest: digest,
		HostID:                 "codex",
		Surface:                "codex-plugin",
		BindingID:              bindingID,
		BindingKind:            "skill",
		BindingReference:       bindingReference,
		Invocation:             "model",
		BindingContentDigest:   digest,
		Evidence: []EvidenceReference{{
			Kind: "host-observation", Reference: "evidence://test/" + bindingID, Digest: digest,
		}},
	}
}

func cloneAssuranceTestProfile(profile profileinspect.Profile) profileinspect.Profile {
	profile.Content = bytes.Clone(profile.Content)
	profile.Warnings = append([]string(nil), profile.Warnings...)
	profile.Metadata.Occurrences = append([]oaw.ProfileOccurrence(nil), profile.Metadata.Occurrences...)
	responsibilities := profile.Metadata.Responsibilities
	profile.Metadata.Responsibilities = make(map[oaw.Responsibility]string, len(profile.Metadata.Responsibilities))
	for responsibility, action := range responsibilities {
		profile.Metadata.Responsibilities[responsibility] = action
	}
	return profile
}
