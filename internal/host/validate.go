package host

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var treeDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

var hostNativeFeatures = []Feature{
	FeatureCancellation,
	FeatureEnvironmentReporting,
	FeatureInvocationDedup,
	FeatureNormalizedReceipts,
	FeaturePause,
	FeatureProviderBindingInventory,
}

var knownFeatures = append([]Feature{}, hostNativeFeatures...)

func NewManifest(value Manifest) (Manifest, error) {
	if value.SchemaVersion != HostManifestSchemaV3 {
		return Manifest{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Manifest schema", nil)
	}
	providedDigest := value.Digest
	value = CloneManifest(value)
	value.Digest = ""
	sort.Strings(value.Protocols)
	sort.Slice(value.BindingKinds, func(left, right int) bool { return value.BindingKinds[left] < value.BindingKinds[right] })
	topologies, err := execution.NormalizeTopologies(value.SupportedTopologies)
	if err != nil {
		return Manifest{}, hostError("HOST_MANIFEST_INVALID", "invalid supported topologies", err)
	}
	value.SupportedTopologies = topologies
	sort.Slice(value.Features, func(left, right int) bool { return value.Features[left] < value.Features[right] })
	sort.Slice(value.DelegationFeatures, func(left, right int) bool { return value.DelegationFeatures[left] < value.DelegationFeatures[right] })
	for index, action := range value.HostActions {
		normalized, ok := normalizeHostActionContract(action)
		if !ok {
			return Manifest{}, hostError("HOST_MANIFEST_INVALID", "unknown or drifted Host action contract", nil)
		}
		value.HostActions[index] = normalized
	}
	sort.Slice(value.HostActions, func(left, right int) bool { return value.HostActions[left].ID < value.HostActions[right].ID })
	if err := validateManifest(value); err != nil {
		return Manifest{}, err
	}
	digest, _, err := canonicaljson.Digest(value)
	if err != nil {
		return Manifest{}, hostError("HOST_MANIFEST_INVALID", "Host Manifest cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return Manifest{}, hostError("HOST_MANIFEST_INVALID", "Host Manifest digest mismatch", nil)
	}
	value.Digest = digest
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
	if err := uniqueBindingKinds(value.BindingKinds); err != nil {
		return err
	}
	if err := uniqueFeatures(value.Features); err != nil {
		return err
	}
	if err := uniqueDelegationFeatures(value.DelegationFeatures); err != nil {
		return err
	}
	if err := uniqueHostActions(value.HostActions); err != nil {
		return err
	}
	switch value.ControlSurface {
	case SurfacePolicy:
		if len(value.Protocols) != 0 || len(value.BindingKinds) != 0 || len(value.Features) != 0 || len(value.DelegationFeatures) != 0 || len(value.HostActions) != 0 ||
			!slices.Equal(value.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
			return hostError("HOST_MANIFEST_INVALID", "policy integration declares Host-native capabilities", nil)
		}
	case SurfaceHostNative:
		if !slices.Equal(value.Protocols, []string{WorkflowProtocolV1}) {
			return hostError("HOST_MANIFEST_INVALID", "host-native integration must support Workflow Protocol v1", nil)
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

func uniqueStrings(values []string, kind string) error {
	for index, value := range values {
		if !validHostText(value, 512) || index > 0 && values[index-1] == value {
			return hostError("HOST_MANIFEST_INVALID", fmt.Sprintf("invalid or duplicate %s", kind), nil)
		}
	}
	return nil
}

func uniqueBindingKinds(values []catalog.BindingKind) error {
	for index, value := range values {
		if !validBindingKind(value) || index > 0 && values[index-1] == value {
			return hostError("HOST_MANIFEST_INVALID", "unknown or duplicate binding kind", nil)
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

func uniqueDelegationFeatures(values []FeatureID) error {
	for index, value := range values {
		if !slices.Contains(knownDelegationFeatures, value) || index > 0 && values[index-1] == value {
			return hostError("HOST_MANIFEST_INVALID", "unknown or duplicate delegation feature", nil)
		}
	}
	return nil
}

func uniqueHostActions(values []HostActionContract) error {
	for index, value := range values {
		if _, ok := normalizeHostActionContract(value); !ok || index > 0 && values[index-1].ID == value.ID {
			return hostError("HOST_MANIFEST_INVALID", "unknown, duplicate, or drifted Host action", nil)
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
	if len(value.References) > 128 {
		return hostError("HOST_AUDIT_INVALID", "audit contains too many evidence references", nil)
	}
	for index, reference := range value.References {
		if strings.TrimSpace(reference.Reference) != reference.Reference || reference.Reference == "" || len(reference.Reference) > 2048 ||
			strings.IndexFunc(reference.Reference, unicode.IsControl) >= 0 || filepath.IsAbs(reference.Reference) || !digestPattern.MatchString(reference.Digest) {
			return hostError("HOST_AUDIT_INVALID", "invalid audit evidence reference", nil)
		}
		if index > 0 && value.References[index-1].Reference == reference.Reference {
			return hostError("HOST_AUDIT_INVALID", "duplicate audit evidence reference", nil)
		}
	}
	return nil
}

func validateConformanceReport(value ConformanceReport) error {
	if !digestPattern.MatchString(value.ManifestDigest) || !digestPattern.MatchString(value.TranscriptDigest) {
		return hostError("HOST_CONFORMANCE_REPORT_INVALID", "missing Conformance Report identity", nil)
	}
	if len(value.Diagnostics) > 32 {
		return hostError("HOST_CONFORMANCE_REPORT_INVALID", "Conformance Report has too many diagnostics", nil)
	}
	for index, feature := range value.VerifiedFeatures {
		if !slices.Contains(knownFeatures, feature) || index > 0 && value.VerifiedFeatures[index-1] == feature {
			return hostError("HOST_CONFORMANCE_REPORT_INVALID", "unknown or duplicate verified control feature", nil)
		}
	}
	for index, feature := range value.VerifiedDelegationFeatures {
		if !slices.Contains(knownDelegationFeatures, feature) || index > 0 && value.VerifiedDelegationFeatures[index-1] == feature {
			return hostError("HOST_CONFORMANCE_REPORT_INVALID", "unknown or duplicate verified delegation feature", nil)
		}
	}
	for index, id := range value.VerifiedHostActionIDs {
		if _, ok := hostActionByID(canonicalHostActions, id); !ok || index > 0 && value.VerifiedHostActionIDs[index-1] == id {
			return hostError("HOST_CONFORMANCE_REPORT_INVALID", "unknown or duplicate verified Host action", nil)
		}
	}
	for index, diagnostic := range value.Diagnostics {
		if !validHostText(diagnostic, 2048) || index > 0 && value.Diagnostics[index-1] == diagnostic {
			return hostError("HOST_CONFORMANCE_REPORT_INVALID", "invalid or duplicate diagnostic", nil)
		}
	}
	return nil
}

func validateIntegration(value IntegrationRecord) error {
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
		if value.Audit.Status != AuditPassed || value.Conformance == nil || len(value.Conformance.Diagnostics) != 0 {
			return hostError("HOST_INTEGRATION_INVALID", "host-native Integration lacks passed audit or Conformance", nil)
		}
		if value.Conformance.ManifestDigest != value.ManifestDigest {
			return hostError("HOST_INTEGRATION_INVALID", "Conformance identity mismatch", nil)
		}
		if !slices.Equal(value.Manifest.Features, value.Conformance.VerifiedFeatures) ||
			!slices.Equal(value.Manifest.DelegationFeatures, value.Conformance.VerifiedDelegationFeatures) ||
			!slices.Equal(hostActionIDs(value.Manifest.HostActions), value.Conformance.VerifiedHostActionIDs) {
			return hostError("HOST_INTEGRATION_INVALID", "Conformance surfaces do not match Host Manifest", nil)
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

func hostActionIDs(values []HostActionContract) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.ID
	}
	return result
}
