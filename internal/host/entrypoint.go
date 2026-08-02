package host

import "fmt"

const SelectedRuntimeIntegrationID = "oaw/codex-runner"

func RuntimeEntrypointAllowed(integrations []IntegrationRecord, hostID string) error {
	for _, integration := range integrations {
		if integration.ID != SelectedRuntimeIntegrationID || integration.Manifest.HostID != hostID {
			continue
		}
		if err := ValidateIntegrationRecord(integration); err != nil {
			return hostError("HOST_RUNTIME_UNSUPPORTED", "selected Runtime Integration is invalid", err)
		}
		if integration.Manifest.IntegrationLevel != RunnerManaged || integration.Manifest.HostID != "codex" || integration.Manifest.Protocols == nil || len(integration.Manifest.Protocols) != 1 || integration.Manifest.Protocols[0] != RuntimeProtocolV1 || integration.Conformance == nil || !integration.Conformance.Passed || integration.Audit.Status != AuditPassed {
			return hostError("HOST_RUNTIME_UNSUPPORTED", "selected Host does not provide exact Runtime capability", nil)
		}
		return nil
	}
	return hostError("HOST_RUNTIME_UNSUPPORTED", fmt.Sprintf("Host %q has no selected Runtime Integration", hostID), nil)
}
