package codexbridge

import (
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	BridgeIntegrationID      = "oaw/codex-host"
	BridgeIntegrationVersion = "1.0.0"
)

func CodexHostManifest() (host.Manifest, error) {
	return host.NewManifest(host.Manifest{
		SchemaVersion:       host.HostManifestSchemaV2,
		ManifestVersion:     BridgeIntegrationVersion,
		HostID:              "codex",
		ControlSurface:      host.SurfaceHostNative,
		Protocols:           []string{host.WorkflowProtocolV1},
		BindingKinds:        []string{"skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{
			host.FeatureEnvironmentReporting,
			host.FeatureNormalizedReceipts,
			host.FeatureProviderBindingInventory,
		},
	})
}
