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
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

var hostNativeFeatures = []Feature{
	FeatureCancellation,
	FeatureEnvironmentReporting,
	FeatureInvocationDedup,
	FeatureNormalizedReceipts,
	FeaturePause,
	FeatureProviderBindingInventory,
}

var knownFeatures = append([]Feature{}, hostNativeFeatures...)

var knownChecks = []CheckID{
	CheckCancellation,
	CheckID(FeatureEnvironmentReporting),
	CheckInvocationDedup,
	CheckID(FeatureNormalizedReceipts),
	CheckPause,
	CheckProviderBindingInventory,
}

func NewManifest(value Manifest) (Manifest, error) {
	value = CloneManifest(value)
	if value.SchemaVersion != HostManifestSchemaV2 {
		return Manifest{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Manifest schema", nil)
	}
	if value.IntegrationLevel != "" {
		return Manifest{}, hostError("HOST_SCHEMA_UNSUPPORTED", "Integration Level is retired", nil)
	}
	if isRetiredControlSurface(value.ControlSurface) {
		return Manifest{}, hostError("HOST_SCHEMA_UNSUPPORTED", "retired Host control surface", nil)
	}
	sort.Strings(value.Protocols)
	sort.Strings(value.BindingKinds)
	topologies, err := execution.NormalizeTopologies(value.SupportedTopologies)
	if err != nil {
		return Manifest{}, hostError("HOST_MANIFEST_INVALID", "invalid supported topologies", err)
	}
	value.SupportedTopologies = topologies
	sort.Slice(value.Features, func(left, right int) bool { return value.Features[left] < value.Features[right] })
	if err := validateManifest(value); err != nil {
		return Manifest{}, err
	}
	return value, nil
}

func validateManifest(value Manifest) error {
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
	switch value.ControlSurface {
	case SurfacePolicy:
		if len(value.Protocols) != 0 || len(value.BindingKinds) != 0 || len(value.Features) != 0 ||
			!slices.Equal(value.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
			return hostError("HOST_MANIFEST_INVALID", "policy integration declares Host-native capabilities", nil)
		}
	case SurfaceHostNative:
		if !slices.Equal(value.Protocols, []string{WorkflowProtocolV1}) {
			return hostError("HOST_MANIFEST_INVALID", "host-native integration must support Workflow Protocol v1", nil)
		}
		for _, kind := range value.BindingKinds {
			if kind != "agent" && kind != "skill" && kind != "tool" {
				return hostError("HOST_MANIFEST_INVALID", fmt.Sprintf("unsupported binding kind %q", kind), nil)
			}
		}
		if len(value.BindingKinds) == 0 {
			return hostError("HOST_MANIFEST_INVALID", "host-native integration has no binding kinds", nil)
		}
		if !slices.Contains(value.SupportedTopologies, execution.TopologyCurrent) {
			return hostError("HOST_MANIFEST_INVALID", "host-native integration must support CURRENT", nil)
		}
		for _, feature := range []Feature{FeatureProviderBindingInventory, FeatureNormalizedReceipts} {
			if !slices.Contains(value.Features, feature) {
				return hostError("HOST_MANIFEST_INVALID", fmt.Sprintf("missing required feature %q", feature), nil)
			}
		}
		if slices.Contains(value.SupportedTopologies, execution.TopologySubagent) && !slices.Contains(value.Features, FeatureEnvironmentReporting) {
			return hostError("HOST_MANIFEST_INVALID", "SUBAGENT support requires environment reporting", nil)
		}
	default:
		return hostError("HOST_MANIFEST_INVALID", "unknown control surface", nil)
	}
	return nil
}

func isRetiredControlSurface(value ControlSurface) bool {
	return value == ControlSurface(InstructionOnly) || value == ControlSurface(RunnerManaged) || value == ControlSurface(NativeManaged)
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
	if value.SchemaVersion != HostIntegrationSchemaV2 {
		return hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Integration schema", nil)
	}
	if !versionPattern.MatchString(value.IntegrationVersion) {
		return hostError("HOST_INTEGRATION_INVALID", "invalid Integration version", nil)
	}
	if _, err := catalog.ParseQualifiedID(value.ID); err != nil {
		return hostError("HOST_INTEGRATION_INVALID", "invalid Integration ID", err)
	}
	switch value.Manifest.ControlSurface {
	case SurfacePolicy:
		if value.Audit.Status != AuditPending || value.Conformance != nil {
			return hostError("HOST_INTEGRATION_INVALID", "policy Integration claims Host-native proof", nil)
		}
	case SurfaceHostNative:
		if value.Audit.Status != AuditPassed || value.Conformance == nil || !value.Conformance.Passed {
			return hostError("HOST_INTEGRATION_INVALID", "host-native Integration lacks passed audit or Conformance", nil)
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
		return hostError("HOST_INTEGRATION_INVALID", "unknown control surface", nil)
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
