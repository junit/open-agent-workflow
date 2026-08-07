package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type providerInputOptions struct {
	HostID                    string
	ProjectRoot               string
	UserConfigRoot            string
	UserHome                  string
	Inventory                 *host.BindingInventory
	IncludeForeignDiagnostics bool
}

type foreignProviderDiscovery struct {
	HostID    string
	Discovery discovery.Report
}

type providerInputs struct {
	HostID           string
	Configuration    config.Snapshot
	Discovery        discovery.Report
	Inventory        *host.BindingInventory
	Resolutions      registry.ResolutionReport
	Registry         registry.Registry
	Foreign          []foreignProviderDiscovery
	UserConfigPath   string
	UserConfigExists bool
}

func loadProviderInputs(options providerInputOptions) (providerInputs, error) {
	if options.HostID == "" {
		return providerInputs{}, fmt.Errorf("PROVIDER_HOST_UNSUPPORTED: host is required")
	}
	userConfigRoot := options.UserConfigRoot
	if userConfigRoot == "" {
		userConfigRoot = defaultConfigRoot()
	}
	loadConfigRoot := userConfigRoot
	if _, statErr := os.Stat(userConfigRoot); os.IsNotExist(statErr) {
		// An absent user-config directory is equivalent to an absent config file;
		// config.Load's empty-root path preserves that state without creating it.
		loadConfigRoot = ""
	} else if statErr != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_CONFIGURATION_REQUIRED: %w", statErr)
	}
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: loadConfigRoot, ProjectRoot: options.ProjectRoot})
	if err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_CONFIGURATION_REQUIRED: %w", err)
	}
	hostIDs := providerHostIDs(snapshot)
	if !containsProviderHost(hostIDs, options.HostID) {
		return providerInputs{}, fmt.Errorf("PROVIDER_HOST_UNSUPPORTED: Host %q is not declared", options.HostID)
	}
	userHome := options.UserHome
	if userHome == "" {
		userHome, err = os.UserHomeDir()
		if err != nil {
			return providerInputs{}, fmt.Errorf("PROVIDER_DISCOVERY_REQUIRED: %w", err)
		}
	}
	evidence, err := discoverProviderHost(snapshot, options.HostID, userHome)
	if err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_DISCOVERY_REQUIRED: %w", err)
	}
	inventory, err := validatedProviderInventory(options.Inventory, options.HostID)
	if err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_REGISTRY_REQUIRED: %w", err)
	}
	resolved, err := core.Resolve(core.ResolutionRequest{
		Configuration: snapshot,
		HostID:        options.HostID,
		Discovery:     evidence,
		Inventory:     inventory,
	})
	if err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_REGISTRY_REQUIRED: %w", err)
	}
	foreign := make([]foreignProviderDiscovery, 0)
	if options.IncludeForeignDiagnostics {
		for _, hostID := range hostIDs {
			if hostID == options.HostID {
				continue
			}
			report, discoverErr := discoverProviderHost(snapshot, hostID, userHome)
			if discoverErr != nil {
				return providerInputs{}, fmt.Errorf("PROVIDER_DISCOVERY_REQUIRED: %w", discoverErr)
			}
			if providerReportHasCandidates(snapshot, report) {
				foreign = append(foreign, foreignProviderDiscovery{HostID: hostID, Discovery: report})
			}
		}
	}
	configPath := filepath.Join(userConfigRoot, "config.toml")
	exists := false
	if _, statErr := os.Stat(configPath); statErr == nil {
		exists = true
	} else if !os.IsNotExist(statErr) {
		return providerInputs{}, fmt.Errorf("PROVIDER_CONFIGURATION_REQUIRED: %w", statErr)
	}
	return providerInputs{
		HostID:        options.HostID,
		Configuration: snapshot, Discovery: evidence, Inventory: inventory, Resolutions: resolved.Report, Registry: resolved.Registry, Foreign: foreign,
		UserConfigPath: configPath, UserConfigExists: exists,
	}, nil
}

func validatedProviderInventory(value *host.BindingInventory, hostID string) (*host.BindingInventory, error) {
	if value == nil {
		return nil, nil
	}
	cloned := host.CloneBindingInventory(*value)
	rebuilt, err := host.NewBindingInventory(cloned.HostID, cloned.Observations)
	if err != nil || cloned.SchemaVersion != host.BindingInventorySchemaV2 || cloned.HostID != hostID || rebuilt.Digest != cloned.Digest {
		return nil, fmt.Errorf("HOST_BINDING_INVENTORY_INVALID: inventory does not match Host %q", hostID)
	}
	return &cloned, nil
}

func providerHostIDs(snapshot config.Snapshot) []string {
	result := make([]string, 0)
	for _, integration := range snapshot.HostIntegrations() {
		if !containsProviderHost(result, integration.Manifest.HostID) {
			result = append(result, integration.Manifest.HostID)
		}
	}
	sort.Strings(result)
	return result
}

func containsProviderHost(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func discoverProviderHost(snapshot config.Snapshot, hostID, userHome string) (discovery.Report, error) {
	hints := make([]discovery.InstallationHint, 0)
	for _, installation := range snapshot.ProviderInstallations() {
		if installation.HostID != hostID {
			continue
		}
		hints = append(hints, discovery.InstallationHint{
			ProviderID: installation.ProviderID, HostID: installation.HostID, SurfaceID: installation.SurfaceID,
			Location: installation.Location, DiscoveryProbeID: installation.DiscoveryProbeID,
		})
	}
	return discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: hostID, UserHome: userHome, Installations: hints})
}

func providerReportHasCandidates(snapshot config.Snapshot, report discovery.Report) bool {
	for _, provider := range snapshot.Catalog().Providers() {
		if len(report.Candidates(provider.ID)) != 0 {
			return true
		}
	}
	return false
}
