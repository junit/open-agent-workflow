package host

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
)

func NewSessionSnapshot(manifest Manifest, input SessionSnapshot) (SessionSnapshot, error) {
	if input.SchemaVersion != HostSessionSchemaV3 {
		return SessionSnapshot{}, hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Session schema", nil)
	}
	normalizedManifest, err := NewManifest(manifest)
	if err != nil {
		if ErrorCode(err) == "HOST_SCHEMA_UNSUPPORTED" {
			return SessionSnapshot{}, err
		}
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "invalid Host Manifest", err)
	}
	if normalizedManifest.ControlSurface != SurfaceHostNative {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "Host Manifest is not host-native", nil)
	}
	providedDigest := input.Digest
	providedFeatureDigest := input.FeatureDigest
	providedActionDigest := input.HostActionDigest
	input = CloneSessionSnapshot(input)
	input.Digest = ""
	input.FeatureDigest = ""
	input.HostActionDigest = ""
	topologies, err := execution.NormalizeTopologies(input.SupportedTopologies)
	if err != nil {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "invalid supported topologies", err)
	}
	features, featureDigest, err := normalizeFeatureObservations(normalizedManifest, input.FeatureObservations)
	if err != nil {
		return SessionSnapshot{}, err
	}
	actions, actionDigest, err := normalizeHostActionObservations(normalizedManifest, input.HostActionObservations)
	if err != nil {
		return SessionSnapshot{}, err
	}
	if providedFeatureDigest != "" && providedFeatureDigest != featureDigest {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "feature observation digest mismatch", nil)
	}
	if providedActionDigest != "" && providedActionDigest != actionDigest {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "Host action observation digest mismatch", nil)
	}
	if err := validateSessionSnapshot(normalizedManifest, input, topologies); err != nil {
		return SessionSnapshot{}, err
	}
	input.SupportedTopologies = topologies
	input.FeatureObservations = features
	input.FeatureDigest = featureDigest
	input.HostActionObservations = actions
	input.HostActionDigest = actionDigest
	digest, _, err := canonicaljson.Digest(input)
	if err != nil {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "session snapshot cannot be canonicalized", err)
	}
	if providedDigest != "" && providedDigest != digest {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "session snapshot digest mismatch", nil)
	}
	input.Digest = digest
	return input, nil
}

func validateSessionSnapshot(manifest Manifest, input SessionSnapshot, topologies []execution.Topology) error {
	if input.HostID != manifest.HostID || !versionPattern.MatchString(input.IntegrationVersion) || input.ManifestDigest != manifest.Digest {
		return hostError("HOST_SESSION_INVALID", "invalid session or Manifest identity", nil)
	}
	if _, err := catalog.ParseQualifiedID(input.IntegrationID); err != nil {
		return hostError("HOST_SESSION_INVALID", "invalid integration identity", err)
	}
	if !validSessionID(input.SessionID) {
		return hostError("HOST_SESSION_INVALID", "invalid session ID", nil)
	}
	if !slices.Contains(topologies, execution.TopologyCurrent) {
		return hostError("HOST_SESSION_INVALID", "CURRENT topology is required", nil)
	}
	for _, topology := range topologies {
		if !slices.Contains(manifest.SupportedTopologies, topology) {
			return hostError("HOST_SESSION_INVALID", fmt.Sprintf("Manifest does not support %s", topology), nil)
		}
	}
	if !digestPattern.MatchString(input.ProviderInventoryDigest) || !digestPattern.MatchString(input.EnvironmentReportDigest) {
		return hostError("HOST_SESSION_INVALID", "missing Host inventory or environment report digest", nil)
	}
	if input.SandboxPolicyDigest != "" && !digestPattern.MatchString(input.SandboxPolicyDigest) {
		return hostError("HOST_SESSION_INVALID", "invalid sandbox policy digest", nil)
	}
	if input.ApprovalPolicyDigest != "" && !digestPattern.MatchString(input.ApprovalPolicyDigest) {
		return hostError("HOST_SESSION_INVALID", "invalid approval policy digest", nil)
	}
	return nil
}

func validateStoredSessionSnapshot(value SessionSnapshot) error {
	if value.SchemaVersion != HostSessionSchemaV3 {
		return hostError("HOST_SCHEMA_UNSUPPORTED", "unsupported Host Session schema", nil)
	}
	if !validSessionID(value.SessionID) || !digestPattern.MatchString(value.Digest) || !digestPattern.MatchString(value.ManifestDigest) {
		return hostError("HOST_SESSION_CHANGED", "Host session identity is invalid", nil)
	}
	if _, err := catalog.ParseLocalID(value.HostID); err != nil {
		return hostError("HOST_SESSION_CHANGED", "Host session has an invalid Host", err)
	}
	if _, err := catalog.ParseQualifiedID(value.IntegrationID); err != nil || !versionPattern.MatchString(value.IntegrationVersion) {
		return hostError("HOST_SESSION_CHANGED", "Host session has an invalid integration", err)
	}
	if !digestPattern.MatchString(value.ProviderInventoryDigest) || !digestPattern.MatchString(value.EnvironmentReportDigest) ||
		value.SandboxPolicyDigest != "" && !digestPattern.MatchString(value.SandboxPolicyDigest) ||
		value.ApprovalPolicyDigest != "" && !digestPattern.MatchString(value.ApprovalPolicyDigest) {
		return hostError("HOST_SESSION_CHANGED", "Host session has invalid fact digests", nil)
	}
	topologies, err := execution.NormalizeTopologies(value.SupportedTopologies)
	if err != nil || !slices.Equal(topologies, value.SupportedTopologies) || !slices.Contains(topologies, execution.TopologyCurrent) {
		return hostError("HOST_SESSION_CHANGED", "Host session topologies changed", err)
	}
	featureManifest := Manifest{DelegationFeatures: append([]FeatureID{}, knownDelegationFeatures...), HostActions: cloneHostActionContracts(canonicalHostActions)}
	features, featureDigest, err := normalizeFeatureObservations(featureManifest, value.FeatureObservations)
	if err != nil || !slices.Equal(features, value.FeatureObservations) || featureDigest != value.FeatureDigest {
		return hostError("HOST_SESSION_CHANGED", "Host feature observations changed", err)
	}
	actions, actionDigest, err := normalizeHostActionObservations(featureManifest, value.HostActionObservations)
	if err != nil || !hostActionObservationsEqual(actions, value.HostActionObservations) || actionDigest != value.HostActionDigest {
		return hostError("HOST_SESSION_CHANGED", "Host action observations changed", err)
	}
	content := CloneSessionSnapshot(value)
	content.Digest = ""
	digest, _, err := canonicaljson.Digest(content)
	if err != nil || digest != value.Digest {
		return hostError("HOST_SESSION_CHANGED", "Host session digest changed", err)
	}
	return nil
}

func hostActionObservationsEqual(left, right []HostActionObservation) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Action.ID != right[index].Action.ID || left[index].State != right[index].State || left[index].Source != right[index].Source ||
			left[index].EvidenceReference != right[index].EvidenceReference || left[index].Digest != right[index].Digest ||
			!slices.Equal(left[index].Action.MaximumEffects, right[index].Action.MaximumEffects) || !slices.Equal(left[index].Action.Resources, right[index].Action.Resources) ||
			left[index].Action.InputSchema != right[index].Action.InputSchema || left[index].Action.OutcomeSchema != right[index].Action.OutcomeSchema {
			return false
		}
	}
	return true
}

func validSessionID(value string) bool {
	return validHostText(value, 512)
}

func validHostText(value string, maximumRunes int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximumRunes &&
		strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}
