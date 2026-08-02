package host

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var workflowFeatures = []Feature{
	FeatureBundleInheritance,
	FeatureCancellation,
	FeatureEvidenceReturn,
	FeatureExactBindingInvocation,
	FeatureInvocationDedup,
	FeatureIsolatedExecutor,
	FeatureNormalizedObservation,
	FeaturePause,
}

var knownFeatures = append(append([]Feature{}, workflowFeatures...), FeatureNativeInvocation)

var knownChecks = []CheckID{
	CheckBundleInheritance,
	CheckCancellation,
	CheckEvidenceReturn,
	CheckExactBindingInvocation,
	CheckInvocationDedup,
	CheckIsolatedExecutor,
	CheckNativeInvocation,
	CheckNormalizedObservation,
	CheckPause,
}

func NewManifest(value Manifest) (Manifest, error) {
	value = CloneManifest(value)
	sort.Strings(value.Protocols)
	sort.Strings(value.BindingKinds)
	sort.Slice(value.Features, func(left, right int) bool { return value.Features[left] < value.Features[right] })
	if err := validateManifest(value); err != nil {
		return Manifest{}, err
	}
	return value, nil
}

func validateManifest(value Manifest) error {
	if value.SchemaVersion != HostManifestSchemaV1 {
		return hostError("HOST_MANIFEST_INVALID", "unsupported schema version", nil)
	}
	if !versionPattern.MatchString(value.ManifestVersion) {
		return hostError("HOST_MANIFEST_INVALID", "invalid manifest version", nil)
	}
	if _, err := catalog.ParseLocalID(value.HostID); err != nil {
		return hostError("HOST_MANIFEST_INVALID", "invalid Host ID", err)
	}
	if err := uniqueStrings(value.Protocols, "protocol"); err != nil {
		return err
	}
	if err := uniqueStrings(value.BindingKinds, "binding kind"); err != nil {
		return err
	}
	if err := uniqueFeatures(value.Features); err != nil {
		return err
	}
	switch value.IntegrationLevel {
	case InstructionOnly:
		if len(value.Protocols) != 0 || len(value.BindingKinds) != 0 || len(value.Features) != 0 {
			return hostError("HOST_MANIFEST_INVALID", "instruction-only integration declares Runtime capabilities", nil)
		}
	case RunnerManaged, NativeManaged:
		if !slices.Equal(value.Protocols, []string{RuntimeProtocolV1}) {
			return hostError("HOST_MANIFEST_INVALID", "managed integration must support Runtime Protocol v1", nil)
		}
		for _, kind := range value.BindingKinds {
			if kind != "agent" && kind != "skill" && kind != "tool" {
				return hostError("HOST_MANIFEST_INVALID", fmt.Sprintf("unsupported binding kind %q", kind), nil)
			}
		}
		if len(value.BindingKinds) == 0 {
			return hostError("HOST_MANIFEST_INVALID", "managed integration has no binding kinds", nil)
		}
		for _, feature := range workflowFeatures {
			if !slices.Contains(value.Features, feature) {
				return hostError("HOST_MANIFEST_INVALID", fmt.Sprintf("missing required feature %q", feature), nil)
			}
		}
		if value.IntegrationLevel == NativeManaged && !slices.Contains(value.Features, FeatureNativeInvocation) {
			return hostError("HOST_MANIFEST_INVALID", "native integration lacks native invocation", nil)
		}
		if value.IntegrationLevel == RunnerManaged && slices.Contains(value.Features, FeatureNativeInvocation) {
			return hostError("HOST_MANIFEST_INVALID", "runner integration claims native invocation", nil)
		}
	default:
		return hostError("HOST_MANIFEST_INVALID", "unknown integration level", nil)
	}
	return nil
}

func uniqueStrings(values []string, kind string) error {
	for index, value := range values {
		if value == "" || index > 0 && values[index-1] == value {
			return hostError("HOST_MANIFEST_INVALID", fmt.Sprintf("invalid or duplicate %s", kind), nil)
		}
	}
	return nil
}

func uniqueFeatures(values []Feature) error {
	for index, value := range values {
		if !slices.Contains(knownFeatures, value) || index > 0 && values[index-1] == value {
			return hostError("HOST_MANIFEST_INVALID", "unknown or duplicate feature", nil)
		}
	}
	return nil
}

func validateAuditEvidence(value AuditEvidence) error {
	switch value.Status {
	case AuditPending:
		if len(value.References) != 0 {
			return hostError("HOST_AUDIT_INVALID", "pending audit contains evidence", nil)
		}
	case AuditPassed:
		if len(value.References) == 0 {
			return hostError("HOST_AUDIT_INVALID", "passed audit has no evidence", nil)
		}
	default:
		return hostError("HOST_AUDIT_INVALID", "unknown audit status", nil)
	}
	for index, reference := range value.References {
		if strings.TrimSpace(reference.Reference) != reference.Reference || reference.Reference == "" || len(reference.Reference) > 2048 || strings.IndexFunc(reference.Reference, unicode.IsControl) >= 0 || !digestPattern.MatchString(reference.Digest) {
			return hostError("HOST_AUDIT_INVALID", "invalid audit evidence reference", nil)
		}
		if index > 0 && value.References[index-1].Reference == reference.Reference {
			return hostError("HOST_AUDIT_INVALID", "duplicate audit evidence reference", nil)
		}
	}
	return nil
}

func validateConformanceReport(value ConformanceReport) error {
	if value.SchemaVersion != ConformanceReportSchemaV1 || value.SuiteVersion != ConformanceSuiteV1 {
		return hostError("HOST_CONFORMANCE_INVALID", "unsupported Conformance Report schema or suite", nil)
	}
	if _, err := catalog.ParseQualifiedID(value.IntegrationID); err != nil {
		return hostError("HOST_CONFORMANCE_INVALID", "invalid Integration ID", err)
	}
	if !digestPattern.MatchString(value.ManifestDigest) || !digestPattern.MatchString(value.TranscriptDigest) || len(value.Checks) == 0 {
		return hostError("HOST_CONFORMANCE_INVALID", "missing Conformance identity", nil)
	}
	allPassed := true
	for index, check := range value.Checks {
		if !slices.Contains(knownChecks, check.ID) || !digestPattern.MatchString(check.Evidence) || index > 0 && value.Checks[index-1].ID == check.ID {
			return hostError("HOST_CONFORMANCE_INVALID", "unknown, duplicate, or unpinned check", nil)
		}
		allPassed = allPassed && check.Passed
	}
	if value.Passed != allPassed {
		return hostError("HOST_CONFORMANCE_INVALID", "Report result does not match checks", nil)
	}
	return nil
}

func validateIntegration(value IntegrationRecord) error {
	if value.SchemaVersion != HostIntegrationSchemaV1 || !versionPattern.MatchString(value.IntegrationVersion) {
		return hostError("HOST_INTEGRATION_INVALID", "unsupported Integration schema or version", nil)
	}
	if _, err := catalog.ParseQualifiedID(value.ID); err != nil {
		return hostError("HOST_INTEGRATION_INVALID", "invalid Integration ID", err)
	}
	switch value.Manifest.IntegrationLevel {
	case InstructionOnly:
		if value.Audit.Status != AuditPending || value.Conformance != nil {
			return hostError("HOST_INTEGRATION_INVALID", "instruction-only Integration claims Runtime proof", nil)
		}
	case RunnerManaged, NativeManaged:
		if value.Audit.Status != AuditPassed || value.Conformance == nil || !value.Conformance.Passed {
			return hostError("HOST_INTEGRATION_INVALID", "managed Integration lacks passed audit or Conformance", nil)
		}
		if value.Conformance.IntegrationID != value.ID || value.Conformance.ManifestDigest != value.ManifestDigest {
			return hostError("HOST_INTEGRATION_INVALID", "Conformance identity mismatch", nil)
		}
		expected := make([]CheckID, 0, len(value.Manifest.Features))
		for _, feature := range value.Manifest.Features {
			expected = append(expected, CheckID(feature))
		}
		sort.Slice(expected, func(left, right int) bool { return expected[left] < expected[right] })
		actual := make([]CheckID, len(value.Conformance.Checks))
		for index, check := range value.Conformance.Checks {
			actual[index] = check.ID
		}
		if !slices.Equal(expected, actual) {
			return hostError("HOST_INTEGRATION_INVALID", "Conformance checks do not match Manifest Features", nil)
		}
	default:
		return hostError("HOST_INTEGRATION_INVALID", "unknown Integration level", nil)
	}
	return nil
}

func ValidateIntegrationRecord(value IntegrationRecord) error {
	normalized, err := NewIntegration(value)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(value, normalized) {
		return hostError("HOST_INTEGRATION_INVALID", "Integration record is not canonical", nil)
	}
	return nil
}
