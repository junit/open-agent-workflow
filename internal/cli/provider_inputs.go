package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wifibaby4u/open-agent-workflow/internal/assets"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type providerInputOptions struct {
	HostID         string
	ProjectRoot    string
	UserConfigRoot string
	UserHome       string
}

type providerInputs struct {
	Configuration    config.Snapshot
	Discovery        discovery.Report
	Resolutions      registry.ResolutionReport
	Registry         registry.Registry
	UserConfigPath   string
	UserConfigExists bool
}

func loadProviderInputs(options providerInputOptions) (providerInputs, error) {
	if options.HostID == "" {
		return providerInputs{}, fmt.Errorf("PROVIDER_HOST_UNSUPPORTED: host is required")
	}
	integrations, err := host.LoadBuiltinIntegrations(assets.FS())
	if err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_HOST_UNSUPPORTED: %w", err)
	}
	if err := host.RuntimeEntrypointAllowed(integrations, options.HostID); err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_HOST_UNSUPPORTED: %w", err)
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
	userHome := options.UserHome
	if userHome == "" {
		userHome, err = os.UserHomeDir()
		if err != nil {
			return providerInputs{}, fmt.Errorf("PROVIDER_DISCOVERY_REQUIRED: %w", err)
		}
	}
	evidence, err := discovery.Discover(snapshot.Catalog(), discovery.Options{UserHome: userHome})
	if err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_DISCOVERY_REQUIRED: %w", err)
	}
	resolutions, effective, err := registry.Resolve(snapshot, evidence, &registry.BindingInventory{
		Host: options.HostID, Bindings: catalogHostBindings(snapshot.Catalog(), options.HostID),
	})
	if err != nil {
		return providerInputs{}, fmt.Errorf("PROVIDER_REGISTRY_REQUIRED: %w", err)
	}
	configPath := filepath.Join(userConfigRoot, "config.toml")
	exists := false
	if _, statErr := os.Stat(configPath); statErr == nil {
		exists = true
	} else if !os.IsNotExist(statErr) {
		return providerInputs{}, fmt.Errorf("PROVIDER_CONFIGURATION_REQUIRED: %w", statErr)
	}
	return providerInputs{
		Configuration: snapshot, Discovery: evidence, Resolutions: resolutions, Registry: effective,
		UserConfigPath: configPath, UserConfigExists: exists,
	}, nil
}
