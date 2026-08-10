package host_test

import (
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
)

func TestOldAuthorityBuiltinHostIsRejectedUntilBridgeCutover(t *testing.T) {
	if _, err := host.LoadBuiltinIntegrations(assets.FS()); host.ErrorCode(err) != "HOST_INTEGRATION_SET_INVALID" {
		t.Fatalf("LoadBuiltinIntegrations(old authority) error = %v", err)
	}
}
