package codexbridge

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type Facts struct {
	Session       host.SessionSnapshot
	Inventory     host.BindingInventory
	Environment   host.EnvironmentReport
	Configuration config.Snapshot
	Discovery     discovery.Report
	Resolutions   registry.ResolutionReport
	Registry      registry.Registry
	FactDigests   FactDigests
}

func cloneFacts(value Facts) Facts {
	value.Session = host.CloneSessionSnapshot(value.Session)
	value.Inventory = host.CloneBindingInventory(value.Inventory)
	value.Environment = host.CloneEnvironmentReport(value.Environment)
	return value
}

func validateFacts(value Facts) error {
	manifest, err := bridgeManifest(value.Session)
	if err != nil {
		return err
	}
	normalizedSession, err := host.NewSessionSnapshot(manifest, value.Session)
	if err != nil || normalizedSession.Digest != value.Session.Digest {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host session facts are not canonical", err)
	}
	normalizedInventory, err := host.NewBindingInventory(value.Inventory.HostID, value.Inventory.Observations)
	if err != nil || normalizedInventory.Digest != value.Inventory.Digest {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host binding facts are not canonical", err)
	}
	normalizedEnvironment, err := host.NewEnvironmentReport(value.Environment)
	if err != nil || normalizedEnvironment.Digest != value.Environment.Digest {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host environment facts are not canonical", err)
	}
	if value.Session.ProviderInventoryDigest != value.Inventory.Digest ||
		value.Session.EnvironmentReportDigest != value.Environment.Digest ||
		value.Environment.SessionID != value.Session.SessionID ||
		value.Environment.Topology != execution.TopologyCurrent {
		return NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host facts are not pinned to the same session", nil)
	}
	if err := validateFactDigests(value.FactDigests); err != nil {
		return err
	}
	return nil
}

func bridgeManifest(session host.SessionSnapshot) (host.Manifest, error) {
	if session.HostID == "" || session.IntegrationVersion == "" {
		return host.Manifest{}, NewError("HOST_EVIDENCE_HANDLE_INVALID", "Host session identity is incomplete", nil)
	}
	return host.NewManifest(host.Manifest{
		SchemaVersion:       host.HostManifestSchemaV2,
		ManifestVersion:     "2.0.0",
		HostID:              session.HostID,
		ControlSurface:      host.SurfaceHostNative,
		Protocols:           []string{host.WorkflowProtocolV1},
		BindingKinds:        []string{"agent", "skill", "tool"},
		SupportedTopologies: append([]execution.Topology{}, session.SupportedTopologies...),
		Features: []host.Feature{
			host.FeatureCancellation,
			host.FeatureEnvironmentReporting,
			host.FeatureInvocationDedup,
			host.FeatureNormalizedReceipts,
			host.FeaturePause,
			host.FeatureProviderBindingInventory,
		},
	})
}

func validateFactDigests(value FactDigests) error {
	values := []struct {
		name   string
		digest string
	}{
		{"session", value.Session}, {"inventory", value.Inventory}, {"environment", value.Environment},
		{"configuration", value.Configuration}, {"discovery", value.Discovery}, {"resolution", value.Resolution}, {"registry", value.Registry},
	}
	for _, item := range values {
		if item.digest != "" && (len(item.digest) != 64 || strings.Trim(item.digest, "0123456789abcdef") != "") {
			return NewError("HOST_EVIDENCE_HANDLE_INVALID", fmt.Sprintf("invalid %s fact digest", item.name), nil)
		}
	}
	return nil
}

func factDigest(value string) string {
	if len(value) != 64 {
		return ""
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return ""
	}
	return value
}
