package core

import (
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const coreResolutionSchemaV1 = "oaw.core-resolution/v1"

func Resolve(request ResolutionRequest) (ResolutionResult, error) {
	if err := validateConfiguration(request.Configuration); err != nil {
		return ResolutionResult{}, err
	}
	if _, err := catalog.ParseLocalID(request.HostID); err != nil {
		return ResolutionResult{}, coreError("HOST_PROVIDER_SCOPE_MISMATCH", "invalid Host %q", request.HostID)
	}
	if request.Discovery.HostID() != request.HostID || !validDigest(request.Discovery.Digest()) {
		return ResolutionResult{}, coreError("HOST_PROVIDER_SCOPE_MISMATCH", "Discovery does not match Host %q", request.HostID)
	}
	inventoryDigest := ""
	if request.Inventory != nil {
		if request.Inventory.HostID != request.HostID {
			return ResolutionResult{}, coreError("HOST_PROVIDER_SCOPE_MISMATCH", "Binding Inventory does not match Host %q", request.HostID)
		}
		rebuilt, err := host.NewBindingInventory(request.Inventory.HostID, request.Inventory.Observations)
		if err != nil || rebuilt.Digest != request.Inventory.Digest {
			return ResolutionResult{}, coreError("HOST_BINDING_INVENTORY_INVALID", "Binding Inventory digest is invalid")
		}
		inventoryDigest = request.Inventory.Digest
	}
	report, effective, err := registry.Resolve(request.Configuration, request.HostID, request.Discovery, request.Inventory)
	if err != nil {
		return ResolutionResult{}, err
	}
	record := struct {
		SchemaVersion       string `json:"schema_version"`
		HostID              string `json:"host_id"`
		ConfigurationDigest string `json:"configuration_digest"`
		DiscoveryDigest     string `json:"discovery_digest"`
		InventoryDigest     string `json:"inventory_digest"`
		ReportDigest        string `json:"report_digest"`
		RegistryDigest      string `json:"registry_digest"`
	}{coreResolutionSchemaV1, request.HostID, request.Configuration.Digest(), request.Discovery.Digest(), inventoryDigest, report.Digest(), effective.Digest()}
	digest, _, err := canonicaljson.Digest(record)
	if err != nil {
		return ResolutionResult{}, err
	}
	return ResolutionResult{Report: report, Registry: effective, Digest: digest}, nil
}
