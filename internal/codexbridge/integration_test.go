package codexbridge

import (
	"slices"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestCodexHostManifestDeclaresOnlyCurrentSkillEvidence(t *testing.T) {
	manifest, err := CodexHostManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ControlSurface != host.SurfaceHostNative ||
		!slices.Equal(manifest.BindingKinds, []string{"skill"}) ||
		!slices.Equal(manifest.SupportedTopologies, []execution.Topology{execution.TopologyCurrent}) {
		t.Fatalf("manifest = %#v", manifest)
	}
}
