package host_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestRuntimeEntrypointAllowedOnlyForSelectedCodex(t *testing.T) {
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	if err := host.RuntimeEntrypointAllowed(integrations, "codex"); err != nil {
		t.Fatalf("Codex Runtime entrypoint denied: %v", err)
	}
	for _, hostID := range []string{"claude", "gemini", "opencode", "cursor"} {
		if err := host.RuntimeEntrypointAllowed(integrations, hostID); host.ErrorCode(err) != "HOST_RUNTIME_UNSUPPORTED" {
			t.Fatalf("Runtime entrypoint for %s error = %v", hostID, err)
		}
	}
}
