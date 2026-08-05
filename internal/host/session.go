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
	manifest, err := NewManifest(manifest)
	if err != nil {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "invalid Host Manifest", err)
	}
	if manifest.ControlSurface != SurfaceHostNative {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "Host Manifest is not host-native", nil)
	}
	providedDigest := input.Digest
	input.Digest = ""
	topologies, err := execution.NormalizeTopologies(input.SupportedTopologies)
	if err != nil {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "invalid supported topologies", err)
	}
	manifestTopologies, err := execution.NormalizeTopologies(manifest.SupportedTopologies)
	if err != nil {
		return SessionSnapshot{}, hostError("HOST_SESSION_INVALID", "Manifest has invalid supported topologies", err)
	}
	if err := validateSessionSnapshot(manifest, input, topologies, manifestTopologies); err != nil {
		return SessionSnapshot{}, err
	}
	input.SupportedTopologies = topologies
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

func validateSessionSnapshot(manifest Manifest, input SessionSnapshot, topologies, manifestTopologies []execution.Topology) error {
	if input.SchemaVersion != HostSessionSchemaV2 || input.HostID != manifest.HostID || input.IntegrationVersion == "" || !versionPattern.MatchString(input.IntegrationVersion) {
		return hostError("HOST_SESSION_INVALID", "invalid session identity", nil)
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
		if !slices.Contains(manifestTopologies, topology) {
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

func validSessionID(value string) bool {
	return validHostText(value, 512)
}

func validHostText(value string, maximumRunes int) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= maximumRunes &&
		strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}
