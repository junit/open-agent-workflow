package codexbridge

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestCodexHostManifestDeclaresOnlyCurrentSkillEvidence(t *testing.T) {
	manifest, err := CodexHostManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ControlSurface != host.SurfaceHostNative ||
		manifest.SchemaVersion != host.HostManifestSchemaV3 || manifest.ManifestVersion != "2.0.0" ||
		!slices.Equal(manifest.BindingKinds, []catalog.BindingKind{catalog.BindingSkill}) ||
		!slices.Equal(manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) ||
		!slices.Equal(manifest.DelegationFeatures, []host.FeatureID{host.FeatureChildDelegation}) || len(manifest.HostActions) != 0 {
		t.Fatalf("manifest = %#v", manifest)
	}
}
