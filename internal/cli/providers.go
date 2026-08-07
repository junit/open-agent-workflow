package cli

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const providerInspectionSchemaV3 = "oaw.provider-inspection/v3"

type providerCommand struct {
	hostID      string
	projectRoot string
	format      string
	strict      bool
}

type providerInspectionOutput struct {
	SchemaVersion       string                          `json:"schema_version"`
	UserConfigPath      string                          `json:"user_config_path"`
	UserConfigExists    bool                            `json:"user_config_exists"`
	ConfigurationDigest string                          `json:"configuration_digest"`
	CatalogDigest       string                          `json:"catalog_digest"`
	CurrentHost         providerInspectionCurrentHost   `json:"current_host"`
	ForeignHosts        []providerInspectionForeignHost `json:"foreign_hosts"`
}

type providerInspectionCurrentHost struct {
	HostID           string                       `json:"host_id"`
	DiscoveryDigest  string                       `json:"discovery_digest"`
	ResolutionDigest string                       `json:"resolution_digest"`
	RegistryDigest   string                       `json:"registry_digest"`
	Providers        []providerInspectionProvider `json:"providers"`
}

type providerInspectionForeignHost struct {
	HostID          string                              `json:"host_id"`
	DiscoveryDigest string                              `json:"discovery_digest"`
	Providers       []providerInspectionForeignProvider `json:"providers"`
}

type providerInspectionForeignProvider struct {
	ProviderID       string                        `json:"provider_id"`
	DiagnosticReason string                        `json:"diagnostic_reason"`
	Candidates       []providerInspectionCandidate `json:"candidates"`
}

type providerInspectionProvider struct {
	ProviderID string                        `json:"provider_id"`
	State      registry.ProviderState        `json:"state"`
	Reason     string                        `json:"reason"`
	Instance   *providerInspectionInstance   `json:"instance,omitempty"`
	Candidates []providerInspectionCandidate `json:"candidates"`
}

type providerInspectionInstance struct {
	HostID                 string `json:"host_id"`
	DistributionKey        string `json:"distribution_key"`
	InstallationKey        string `json:"installation_key"`
	Location               string `json:"location"`
	Version                string `json:"version"`
	DescriptorDigest       string `json:"descriptor_digest"`
	ConfigurationDigest    string `json:"configuration_digest"`
	BindingInventoryDigest string `json:"binding_inventory_digest"`
	EvidenceDigest         string `json:"evidence_digest"`
	Digest                 string `json:"digest"`
}

type providerInspectionCandidate struct {
	HostID          string              `json:"host_id"`
	SurfaceID       string              `json:"surface_id"`
	DistributionKey string              `json:"distribution_key"`
	InstallationKey string              `json:"installation_key"`
	Version         string              `json:"version"`
	Location        string              `json:"location"`
	EvidenceDigest  string              `json:"evidence_digest"`
	ProviderPin     *config.ProviderPin `json:"provider_pin,omitempty"`
}

type providerPinDocument struct {
	SchemaVersion string               `toml:"schema_version,omitempty"`
	ProviderPins  []config.ProviderPin `toml:"provider_pins"`
}

func runProviders(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseProvidersCommand(args)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: INVALID_ARGUMENT: %s\n", err)
		return 64
	}
	inputs, err := loadProviderInputs(providerInputOptions{
		HostID: parsed.hostID, ProjectRoot: parsed.projectRoot, UserConfigRoot: defaultConfigRoot(), IncludeForeignDiagnostics: true,
	})
	if err != nil {
		reason := providerInputReason(err)
		status := 65
		if reason == "PROVIDER_HOST_UNSUPPORTED" {
			status = 69
		}
		fmt.Fprintf(stderr, "oaw: %s: %v\n", reason, err)
		return status
	}
	if parsed.strict && inputs.Inventory == nil {
		fmt.Fprintln(stderr, "oaw: HOST_BRIDGE_UNAVAILABLE: strict inspection requires current-session Bridge evidence; invoke core.inspect in the active Codex session")
		return 69
	}
	output := providerInspectionProjection(inputs)
	if parsed.format == "json" {
		return writeProviderInspectionJSON(output, stdout, stderr)
	}
	return writeProviderInspectionText(inputs, output, stdout, stderr)
}

func parseProvidersCommand(args []string) (providerCommand, error) {
	result := providerCommand{format: "text"}
	if len(args) == 0 || args[0] != "inspect" {
		return providerCommand{}, fmt.Errorf("expected providers inspect command")
	}
	hostSeen, projectSeen, formatSeen, strictSeen := false, false, false, false
	for index := 1; index < len(args); {
		argument := args[index]
		switch {
		case argument == "--host":
			if hostSeen || index+1 >= len(args) || args[index+1] == "" {
				return providerCommand{}, fmt.Errorf("--host requires one value")
			}
			hostSeen = true
			result.hostID = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--host="):
			if hostSeen || strings.TrimPrefix(argument, "--host=") == "" {
				return providerCommand{}, fmt.Errorf("--host may be specified only once")
			}
			hostSeen = true
			result.hostID = strings.TrimPrefix(argument, "--host=")
			index++
		case argument == "--project-root":
			if projectSeen || index+1 >= len(args) || args[index+1] == "" {
				return providerCommand{}, fmt.Errorf("--project-root requires one value")
			}
			projectSeen = true
			result.projectRoot = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--project-root="):
			if projectSeen || strings.TrimPrefix(argument, "--project-root=") == "" {
				return providerCommand{}, fmt.Errorf("--project-root may be specified only once")
			}
			projectSeen = true
			result.projectRoot = strings.TrimPrefix(argument, "--project-root=")
			index++
		case argument == "--format":
			if formatSeen || index+1 >= len(args) || args[index+1] == "" {
				return providerCommand{}, fmt.Errorf("--format requires one value")
			}
			formatSeen = true
			result.format = args[index+1]
			index += 2
		case strings.HasPrefix(argument, "--format="):
			if formatSeen || strings.TrimPrefix(argument, "--format=") == "" {
				return providerCommand{}, fmt.Errorf("--format may be specified only once")
			}
			formatSeen = true
			result.format = strings.TrimPrefix(argument, "--format=")
			index++
		case argument == "--strict":
			if strictSeen {
				return providerCommand{}, fmt.Errorf("--strict may be specified only once")
			}
			strictSeen = true
			result.strict = true
			index++
		default:
			return providerCommand{}, fmt.Errorf("unexpected providers argument %q", argument)
		}
	}
	if result.hostID == "" || strings.IndexFunc(result.hostID, unicode.IsControl) >= 0 {
		return providerCommand{}, fmt.Errorf("--host is required")
	}
	if result.projectRoot != "" && (strings.IndexFunc(result.projectRoot, unicode.IsControl) >= 0 || !filepath.IsAbs(result.projectRoot) || filepath.Clean(result.projectRoot) != result.projectRoot) {
		return providerCommand{}, fmt.Errorf("project root must be a clean absolute path")
	}
	if result.format != "text" && result.format != "json" {
		return providerCommand{}, fmt.Errorf("unknown format %q", result.format)
	}
	return result, nil
}

func providerInputReason(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if index := strings.IndexByte(message, ':'); index > 0 {
		return message[:index]
	}
	return "PROVIDER_INPUTS_REQUIRED"
}

func providerInspectionProjection(inputs providerInputs) providerInspectionOutput {
	resolutions := inputs.Resolutions.Resolutions()
	providers := make([]providerInspectionProvider, 0, len(resolutions))
	for _, resolution := range resolutions {
		provider := providerInspectionProvider{ProviderID: resolution.ProviderID, State: resolution.State, Reason: resolution.Reason, Candidates: make([]providerInspectionCandidate, 0, len(resolution.Candidates))}
		if resolution.Instance != nil {
			instance := *resolution.Instance
			provider.Instance = &providerInspectionInstance{
				HostID: instance.HostID, DistributionKey: instance.DistributionKey, InstallationKey: instance.InstallationKey,
				Location: instance.Location, Version: instance.Version, DescriptorDigest: instance.DescriptorDigest,
				ConfigurationDigest: instance.ConfigurationDigest, BindingInventoryDigest: instance.BindingInventoryDigest,
				EvidenceDigest: instance.EvidenceDigest, Digest: instance.Digest,
			}
		}
		for _, candidate := range resolution.Candidates {
			var pin *config.ProviderPin
			if resolution.State == registry.Ambiguous {
				value := config.ProviderPin{
					ProviderID: resolution.ProviderID, HostID: inputs.HostID,
					InstallationKey: candidate.InstallationKey, EvidenceDigest: candidate.EvidenceDigest,
					Location: candidate.Location, Version: candidate.Version,
				}
				pin = &value
			}
			provider.Candidates = append(provider.Candidates, inspectionCandidate(candidate, pin))
		}
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(left, right int) bool { return providers[left].ProviderID < providers[right].ProviderID })
	foreignHosts := make([]providerInspectionForeignHost, 0, len(inputs.Foreign))
	for _, foreign := range inputs.Foreign {
		foreignProviders := make([]providerInspectionForeignProvider, 0)
		for _, descriptor := range inputs.Configuration.Catalog().Providers() {
			candidates := foreign.Discovery.Candidates(descriptor.ID)
			if len(candidates) == 0 {
				continue
			}
			projected := make([]providerInspectionCandidate, 0, len(candidates))
			for _, candidate := range candidates {
				projected = append(projected, inspectionCandidate(candidate, nil))
			}
			foreignProviders = append(foreignProviders, providerInspectionForeignProvider{
				ProviderID: descriptor.ID, DiagnosticReason: "PROVIDER_FOREIGN_HOST_ONLY", Candidates: projected,
			})
		}
		foreignHosts = append(foreignHosts, providerInspectionForeignHost{
			HostID: foreign.HostID, DiscoveryDigest: foreign.Discovery.Digest(), Providers: foreignProviders,
		})
	}
	return providerInspectionOutput{
		SchemaVersion: providerInspectionSchemaV3, UserConfigPath: inputs.UserConfigPath, UserConfigExists: inputs.UserConfigExists,
		ConfigurationDigest: inputs.Configuration.Digest(), CatalogDigest: inputs.Configuration.Catalog().Digest(),
		CurrentHost: providerInspectionCurrentHost{
			HostID: inputs.HostID, DiscoveryDigest: inputs.Discovery.Digest(),
			ResolutionDigest: inputs.Resolutions.Digest(), RegistryDigest: inputs.Registry.Digest(), Providers: providers,
		},
		ForeignHosts: foreignHosts,
	}
}

func inspectionCandidate(candidate discovery.Candidate, pin *config.ProviderPin) providerInspectionCandidate {
	return providerInspectionCandidate{
		HostID: candidate.HostID, SurfaceID: candidate.SurfaceID, DistributionKey: candidate.DistributionKey,
		InstallationKey: candidate.InstallationKey, Version: candidate.Version, Location: candidate.Location,
		EvidenceDigest: candidate.EvidenceDigest, ProviderPin: pin,
	}
}

func writeProviderInspectionText(inputs providerInputs, output providerInspectionOutput, stdout, stderr io.Writer) int {
	sections := make([]string, 0, len(output.CurrentHost.Providers)+len(output.ForeignHosts))
	includeSchema := !inputs.UserConfigExists
	for _, provider := range output.CurrentHost.Providers {
		lines := []string{fmt.Sprintf("provider %s state=%s reason=%s", provider.ProviderID, provider.State, provider.Reason)}
		for _, candidate := range provider.Candidates {
			lines = append(lines, fmt.Sprintf("candidate host_id=%s surface_id=%s distribution_key=%s installation_key=%s version=%s location=%s evidence_digest=%s", candidate.HostID, candidate.SurfaceID, candidate.DistributionKey, candidate.InstallationKey, candidate.Version, candidate.Location, candidate.EvidenceDigest))
			if candidate.ProviderPin != nil {
				fragment, err := encodeProviderPin(*candidate.ProviderPin, includeSchema)
				if err != nil {
					fmt.Fprintf(stderr, "oaw: OUTPUT_ENCODE_FAILED: %v\n", err)
					return 70
				}
				lines = append(lines, fragment)
				includeSchema = false
			}
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	for _, foreign := range output.ForeignHosts {
		for _, provider := range foreign.Providers {
			lines := []string{fmt.Sprintf("foreign_host %s provider=%s reason=%s", foreign.HostID, provider.ProviderID, provider.DiagnosticReason)}
			for _, candidate := range provider.Candidates {
				lines = append(lines, fmt.Sprintf("candidate host_id=%s surface_id=%s distribution_key=%s installation_key=%s version=%s location=%s evidence_digest=%s", candidate.HostID, candidate.SurfaceID, candidate.DistributionKey, candidate.InstallationKey, candidate.Version, candidate.Location, candidate.EvidenceDigest))
			}
			sections = append(sections, strings.Join(lines, "\n"))
		}
	}
	value := fmt.Sprintf("configuration path=%s exists=%t\ncurrent_host %s\n%s\n", output.UserConfigPath, output.UserConfigExists, output.CurrentHost.HostID, strings.Join(sections, "\n\n"))
	if _, err := io.WriteString(stdout, value); err != nil {
		fmt.Fprintf(stderr, "oaw: OUTPUT_WRITE_FAILED: %v\n", err)
		return 74
	}
	return 0
}

func encodeProviderPin(pin config.ProviderPin, includeSchema bool) (string, error) {
	document := providerPinDocument{ProviderPins: []config.ProviderPin{pin}}
	if includeSchema {
		document.SchemaVersion = config.UserConfigSchemaV3
	}
	var buffer bytes.Buffer
	if err := toml.NewEncoder(&buffer).Encode(document); err != nil {
		return "", err
	}
	return strings.TrimRight(buffer.String(), "\n"), nil
}

func writeProviderInspectionJSON(output providerInspectionOutput, stdout, stderr io.Writer) int {
	encoded, err := canonicaljson.Marshal(output)
	if err != nil {
		fmt.Fprintf(stderr, "oaw: OUTPUT_ENCODE_FAILED: %v\n", err)
		return 70
	}
	encoded = append(encoded, '\n')
	if _, err := stdout.Write(encoded); err != nil {
		fmt.Fprintf(stderr, "oaw: OUTPUT_WRITE_FAILED: %v\n", err)
		return 74
	}
	return 0
}
