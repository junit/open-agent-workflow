package codexbridge

import (
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

const (
	BridgeIntegrationID      = "oaw/codex-host"
	BridgeIntegrationVersion = "2.0.0"
)

func CodexHostManifest() (host.Manifest, error) {
	return host.NewManifest(host.Manifest{
		SchemaVersion:       host.HostManifestSchemaV3,
		ManifestVersion:     BridgeIntegrationVersion,
		HostID:              "codex",
		ControlSurface:      host.SurfaceHostNative,
		Protocols:           []string{host.WorkflowProtocolV1},
		BindingKinds:        []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features: []host.Feature{
			host.FeatureEnvironmentReporting,
			host.FeatureNormalizedReceipts,
			host.FeatureProviderBindingInventory,
		},
		DelegationFeatures: []host.FeatureID{host.FeatureChildDelegation},
		HostActions:        []host.HostActionContract{},
	})
}
