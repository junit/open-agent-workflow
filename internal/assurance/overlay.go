package assurance

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	oaw "github.com/wifibaby4u/open-agent-workflow"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/profileinspect"
)

var (
	digestPattern     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	identityPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+\-]{0,255}$`)
	occurrencePattern = regexp.MustCompile(`^profile-occurrence:sha256:[0-9a-f]{64}$`)
)

type inspectedProfile struct {
	reference   ProfileReference
	occurrences []oaw.ProfileOccurrence
	byRef       map[string]profileOccurrence
}

type profileOccurrence struct {
	index     int
	reference string
}

func Issue(profile profileinspect.Profile, request IssueRequest) (Overlay, error) {
	if err := validateIssueRequestShape(request); err != nil {
		return Overlay{}, err
	}
	inspected, err := inspectProfile(profile)
	if err != nil {
		return Overlay{}, err
	}
	claims, err := normalizeClaims(inspected, request.Claims)
	if err != nil {
		return Overlay{}, err
	}
	overlay := Overlay{
		SchemaVersion: OverlaySchemaV1,
		Profile:       inspected.reference,
		Issuer:        request.Issuer,
		Claims:        claims,
	}
	overlay.Digest, err = overlayDigest(overlay)
	if err != nil {
		return Overlay{}, assuranceError("OVERLAY_INVALID", "digest Assurance Overlay", err)
	}
	return overlay, nil
}

func Inspect(profile profileinspect.Profile) (ReferenceIndex, error) {
	inspected, err := inspectProfile(profile)
	if err != nil {
		return ReferenceIndex{}, err
	}
	result := ReferenceIndex{
		SchemaVersion: ReferenceIndexSchemaV1,
		Profile:       inspected.reference,
		Occurrences:   make([]ReferenceOccurrence, len(inspected.occurrences)),
	}
	for index, occurrence := range inspected.occurrences {
		result.Occurrences[index] = ReferenceOccurrence{
			OccurrenceRef: occurrence.Ref, BindingReference: occurrence.Reference,
		}
	}
	return result, nil
}

func Verify(profile profileinspect.Profile, overlay Overlay) error {
	inspected, err := inspectProfile(profile)
	if err != nil {
		return err
	}
	if overlay.SchemaVersion != OverlaySchemaV1 {
		return assuranceError("OVERLAY_INVALID", "unsupported Assurance Overlay schema", nil)
	}
	if !reflect.DeepEqual(overlay.Profile, inspected.reference) {
		return assuranceError("PROFILE_REFERENCE_MISMATCH", "Profile reference does not match the selected Markdown Profile", nil)
	}
	if !validIdentity(overlay.Issuer) {
		return assuranceError("OVERLAY_INVALID", "invalid issuer identity", nil)
	}
	if !digestPattern.MatchString(overlay.Digest) {
		return assuranceError("OVERLAY_INVALID", "invalid Overlay digest", nil)
	}
	if err := validateClaimsInProfileOrder(inspected, overlay.Claims); err != nil {
		return err
	}
	digest, err := overlayDigest(overlay)
	if err != nil {
		return assuranceError("OVERLAY_INVALID", "digest Assurance Overlay", err)
	}
	if digest != overlay.Digest {
		return assuranceError("OVERLAY_INVALID", "Assurance Overlay digest mismatch", nil)
	}
	return nil
}

func inspectProfile(profile profileinspect.Profile) (inspectedProfile, error) {
	switch profile.Source {
	case profileinspect.SourceBuiltIn, profileinspect.SourceProject, profileinspect.SourceUser:
	default:
		return inspectedProfile{}, assuranceError("PROFILE_NOT_ASSURABLE", "Profile requires a built-in, project, or user source", nil)
	}
	parsed, warnings, err := oaw.InspectPolicyProfile(profile.Path, profile.Content)
	if err != nil {
		return inspectedProfile{}, assuranceError("PROFILE_NOT_ASSURABLE", "read selected Markdown Profile", err)
	}
	if len(warnings) != 0 {
		return inspectedProfile{}, assuranceError("PROFILE_NOT_ASSURABLE", "Profile body is ambiguous: "+warnings[0], nil)
	}
	if len(parsed.Occurrences) == 0 {
		return inspectedProfile{}, assuranceError("PROFILE_NOT_ASSURABLE", "Profile has no machine-addressable references", nil)
	}
	result := inspectedProfile{
		reference: ProfileReference{
			Source: string(profile.Source), ID: parsed.ID, ContentDigest: digestBytes(profile.Content),
		},
		occurrences: append([]oaw.ProfileOccurrence(nil), parsed.Occurrences...),
		byRef:       make(map[string]profileOccurrence, len(parsed.Occurrences)),
	}
	for index, occurrence := range parsed.Occurrences {
		occurrence.Ref = scopedOccurrenceRef(result.reference, occurrence.Ref)
		result.occurrences[index] = occurrence
		if _, duplicate := result.byRef[occurrence.Ref]; duplicate {
			return inspectedProfile{}, assuranceError("PROFILE_NOT_ASSURABLE", "Profile has a duplicate occurrence reference", nil)
		}
		result.byRef[occurrence.Ref] = profileOccurrence{index: index, reference: occurrence.Reference}
	}
	return result, nil
}

func validateIssueRequestShape(request IssueRequest) error {
	if request.SchemaVersion != IssueRequestSchemaV1 {
		return assuranceError("ASSURANCE_INPUT_INVALID", "unsupported Issue Request schema", nil)
	}
	if !validIdentity(request.Issuer) {
		return assuranceError("ASSURANCE_INPUT_INVALID", "invalid issuer identity", nil)
	}
	if len(request.Claims) == 0 {
		return assuranceError("ASSURANCE_INPUT_INVALID", "at least one Binding claim is required", nil)
	}
	return nil
}

func normalizeClaims(profile inspectedProfile, claims []BindingClaim) ([]BindingClaim, error) {
	result := cloneClaims(claims)
	seen := make(map[string]bool, len(result))
	for index := range result {
		if _, err := validateClaim(profile, result[index]); err != nil {
			return nil, err
		}
		if seen[result[index].OccurrenceRef] {
			return nil, assuranceError("BINDING_CLAIM_INVALID", "duplicate occurrence reference "+result[index].OccurrenceRef, nil)
		}
		seen[result[index].OccurrenceRef] = true
		result[index].Evidence = normalizeEvidence(result[index].Evidence)
	}
	sort.Slice(result, func(left, right int) bool {
		return profile.byRef[result[left].OccurrenceRef].index < profile.byRef[result[right].OccurrenceRef].index
	})
	return result, nil
}

func validateClaimsInProfileOrder(profile inspectedProfile, claims []BindingClaim) error {
	if len(claims) == 0 {
		return assuranceError("OVERLAY_INVALID", "Assurance Overlay has no Binding claims", nil)
	}
	seen := make(map[string]bool, len(claims))
	previous := -1
	for _, claim := range claims {
		occurrence, err := validateClaim(profile, claim)
		if err != nil {
			return err
		}
		if seen[claim.OccurrenceRef] {
			return assuranceError("BINDING_CLAIM_INVALID", "duplicate occurrence reference "+claim.OccurrenceRef, nil)
		}
		seen[claim.OccurrenceRef] = true
		if occurrence.index <= previous {
			return assuranceError("OVERLAY_INVALID", "Binding claims do not follow Profile order", nil)
		}
		previous = occurrence.index
		if !reflect.DeepEqual(claim.Evidence, normalizeEvidence(claim.Evidence)) {
			return assuranceError("OVERLAY_INVALID", "Binding claim evidence is not in canonical order", nil)
		}
	}
	return nil
}

func validateClaim(profile inspectedProfile, claim BindingClaim) (profileOccurrence, error) {
	if !occurrencePattern.MatchString(claim.OccurrenceRef) {
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "invalid occurrence reference", nil)
	}
	occurrence, found := profile.byRef[claim.OccurrenceRef]
	if !found {
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "occurrence reference is not in the selected Profile", nil)
	}
	if claim.BindingReference != occurrence.reference {
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "Binding reference does not match the Profile occurrence", nil)
	}
	identities := []struct {
		label string
		value string
	}{
		{label: "Provider ID", value: claim.ProviderID},
		{label: "distribution ID", value: claim.DistributionID},
		{label: "distribution revision", value: claim.DistributionRevision},
		{label: "Host ID", value: claim.HostID},
		{label: "surface", value: claim.Surface},
		{label: "Binding ID", value: claim.BindingID},
	}
	for _, identity := range identities {
		if !validIdentity(identity.value) {
			return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "invalid "+identity.label, nil)
		}
	}
	if !validReference(claim.BindingReference) {
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "invalid Binding reference", nil)
	}
	switch claim.BindingKind {
	case "skill", "agent", "role", "instruction", "tool", "host-action":
	default:
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "invalid Binding kind", nil)
	}
	switch claim.Invocation {
	case "human-explicit", "model", "host", "internal":
	default:
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "invalid invocation", nil)
	}
	if !digestPattern.MatchString(claim.DistributionTreeDigest) || !digestPattern.MatchString(claim.BindingContentDigest) {
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "invalid content digest", nil)
	}
	if len(claim.Evidence) == 0 {
		return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "at least one evidence reference is required", nil)
	}
	seenEvidence := make(map[string]bool, len(claim.Evidence))
	for _, evidence := range claim.Evidence {
		if !validIdentity(evidence.Kind) || !validReference(evidence.Reference) || !digestPattern.MatchString(evidence.Digest) {
			return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "invalid evidence reference", nil)
		}
		key := evidence.Kind + "\x00" + evidence.Reference + "\x00" + evidence.Digest
		if seenEvidence[key] {
			return profileOccurrence{}, assuranceError("BINDING_CLAIM_INVALID", "duplicate evidence reference", nil)
		}
		seenEvidence[key] = true
	}
	return occurrence, nil
}

func overlayDigest(overlay Overlay) (string, error) {
	value := overlay
	value.Claims = cloneClaims(overlay.Claims)
	value.Digest = ""
	encoded, err := canonicaljson.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestBytes(encoded), nil
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func scopedOccurrenceRef(profile ProfileReference, locator string) string {
	payload := strings.Join([]string{
		"oaw.assurance-profile-occurrence/v1",
		profile.Source,
		profile.ID,
		profile.ContentDigest,
		locator,
	}, "\x00")
	return "profile-occurrence:" + digestBytes([]byte(payload))
}

func normalizeEvidence(values []EvidenceReference) []EvidenceReference {
	result := append([]EvidenceReference(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		first := result[left].Kind + "\x00" + result[left].Reference + "\x00" + result[left].Digest
		second := result[right].Kind + "\x00" + result[right].Reference + "\x00" + result[right].Digest
		return first < second
	})
	return result
}

func cloneClaims(values []BindingClaim) []BindingClaim {
	result := make([]BindingClaim, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Evidence = append([]EvidenceReference(nil), value.Evidence...)
	}
	return result
}

func validIdentity(value string) bool {
	return identityPattern.MatchString(value)
}

func validReference(value string) bool {
	return value != "" && len(value) <= 2048 && value == strings.TrimSpace(value) &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}
