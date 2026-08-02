package runtime

import (
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func admitWorkflowHost(options WorkflowOptions, graph profile.ExecutionGraphRecord) (host.WorkflowAdmission, error) {
	bindings := make([]catalog.HostBinding, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		bindings = append(bindings, node.Binding)
	}
	admitted, err := host.AdmitWorkflow(options.Configuration.HostIntegrations(), options.Host, bindings)
	if err != nil {
		if host.ErrorCode(err) == "HOST_RUNTIME_REQUIREMENTS_UNMET" {
			compatibility := runtimeError("HOST_ISOLATION_UNAVAILABLE", "Host cannot currently provide the isolated Workflow guarantees", err)
			return host.WorkflowAdmission{}, runtimeError(host.ErrorCode(err), "Workflow Host admission failed", compatibility)
		}
		return host.WorkflowAdmission{}, runtimeError(host.ErrorCode(err), "Workflow Host admission failed", err)
	}
	return admitted, nil
}

func validateActiveWorkflowHost(options WorkflowOptions, bundle LifecycleBundle) error {
	admitted, err := admitWorkflowHost(options, bundle.Graph)
	if err != nil {
		return err
	}
	if admitted.IntegrationID != bundle.HostIntegrationID || admitted.IntegrationDigest != bundle.HostIntegrationDigest || admitted.ManifestDigest != bundle.HostManifestDigest || admitted.AuditDigest != bundle.HostAuditDigest || admitted.ConformanceDigest != bundle.HostConformanceDigest {
		return runtimeError("HOST_INTEGRATION_CHANGED", "active Bundle Host Integration does not match current trusted inputs", nil)
	}
	return nil
}
