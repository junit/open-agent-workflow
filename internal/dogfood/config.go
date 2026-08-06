package dogfood

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"

	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/core"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

const (
	providerProbeID = "current-skill"
	providerSurface = "codex-current-skill"
)

func writePilotConfiguration(root string, fingerprint repositoryFingerprint, sessionID string) (config.Snapshot, host.IntegrationRecord, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("configuration root must be a clean absolute path")
	}
	skillDirectory := filepath.Dir(fingerprint.SkillPath)
	provider := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV3, DescriptorVersion: "3.0.0",
		ID: providerID, DisplayName: "OpenCodeReview Read-only Skill",
		Discovery: []catalog.DiscoveryProbe{{
			ID: providerProbeID, Hosts: []string{hostID}, Surface: providerSurface,
			Distribution: "open-code-review", Kind: "path-exists", Root: "user-home",
			CandidatePath: filepath.Join("skills", "open-code-review"), EvidencePath: "SKILL.md",
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: capabilityID, InputSchema: "oaw.capability-input/v1", OutcomeSchema: "oaw.capability-outcome/v1",
			MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			RequestModes:        []catalog.RequestMode{catalog.RequestModeWorkflow},
			Responsibilities:    []string{"review-scope", "code-review", "verification"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, DelegationAllowList: []string{},
			HostBindings: []catalog.HostBinding{{Host: hostID, Kind: "skill", Reference: bindingReference, Topologies: []execution.Topology{execution.TopologyCurrent}}},
		}},
	}
	recipe := catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV2, RecipeVersion: "2.0.0", ID: profileID,
		DisplayName:              "OpenCodeReview Current Read-only Workflow",
		RequiredResponsibilities: []string{"review-scope", "code-review", "verification"},
		Nodes: []catalog.RecipeNode{
			{ID: "review-scope", Kind: catalog.PhaseNode, Responsibility: "review-scope", Selector: catalog.CapabilitySelector{ProviderID: providerID, CapabilityID: capabilityID}, Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "code-review"}}},
			{ID: "code-review", Kind: catalog.PhaseNode, Responsibility: "code-review", Selector: catalog.CapabilitySelector{ProviderID: providerID, CapabilityID: capabilityID}, Transitions: []catalog.RecipeTransition{{Signal: "succeeded", Target: "verification"}}},
			{ID: "verification", Kind: catalog.GateNode, Responsibility: "verification", Selector: catalog.CapabilitySelector{ProviderID: providerID, CapabilityID: capabilityID}, Transitions: []catalog.RecipeTransition{}},
		},
		IncidentRoutes: []catalog.IncidentRoute{}, Entry: "review-scope", TerminalGates: []string{"verification"},
		StableBoundaries: []string{"scope-complete", "review-complete", "verification-complete"}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV2, ManifestVersion: "2.0.0", HostID: hostID,
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []string{"skill"},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
	})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host manifest: %w", err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPassed, References: []host.AuditEvidenceReference{{Reference: "file://" + fingerprint.SkillPath, Digest: fingerprint.SkillDigest}}})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host audit: %w", err)
	}
	conformance, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion: host.HostConformanceReportSchemaV2, ManifestDigest: manifest.ContentDigest(),
		TranscriptDigest: canonicaljson.DigestBytes([]byte("oaw current dogfood transcript\n")),
		VerifiedFeatures: []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}, Diagnostics: []string{},
	})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host conformance: %w", err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV2, IntegrationVersion: "2.0.0", ID: integrationID,
		Manifest: manifest, ManifestDigest: manifest.ContentDigest(), Audit: audit, Conformance: &conformance,
	})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host integration: %w", err)
	}
	if sessionID == "" {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host session is required")
	}
	if err := writeCanonical(filepath.Join(root, providerFile), provider); err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, err
	}
	if err := writeCanonical(filepath.Join(root, profileFile), recipe); err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, err
	}
	var integrationTOML bytes.Buffer
	if err := toml.NewEncoder(&integrationTOML).Encode(integration); err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("encode pilot Host integration: %w", err)
	}
	if err := writeText(filepath.Join(root, integrationFile), integrationTOML.Bytes(), 0o600); err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, err
	}
	installationPath := strconv.Quote(skillDirectory)
	userConfig := "schema_version = \"" + config.UserConfigSchemaV3 + "\"\n\n" +
		"[[provider_descriptors]]\nid = \"" + providerID + "\"\npath = \"" + providerFile + "\"\n\n" +
		"[[profile_recipes]]\nid = \"" + profileID + "\"\npath = \"" + profileFile + "\"\n\n" +
		"[[host_integrations]]\nid = \"" + integrationID + "\"\npath = \"" + integrationFile + "\"\n\n" +
		"[[provider_installations]]\nprovider_id = \"" + providerID + "\"\nhost_id = \"" + hostID + "\"\nsurface_id = \"" + providerSurface + "\"\nlocation = " + installationPath + "\ndiscovery_probe_id = \"" + providerProbeID + "\"\n"
	if err := writeText(filepath.Join(root, userConfigFile), []byte(userConfig), 0o600); err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, err
	}
	snapshot, err := config.Load(config.LoadOptions{UserConfigRoot: root})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("load pilot configuration: %w", err)
	}
	return snapshot, integration, nil
}

func resolveProvider(snapshot config.Snapshot, fingerprint repositoryFingerprint) (core.ResolutionResult, host.BindingInventory, error) {
	if snapshot.Digest() == "" || fingerprint.Root == "" || fingerprint.SkillPath == "" {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot Provider inputs are incomplete")
	}
	if filepath.Dir(fingerprint.SkillPath) != filepath.Join(fingerprint.Root, "skills", "open-code-review") {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("OpenCodeReview Skill is outside its required installation directory")
	}
	hints := make([]discovery.InstallationHint, 0)
	for _, installation := range snapshot.ProviderInstallations() {
		if installation.ProviderID == providerID && installation.HostID == hostID {
			hints = append(hints, discovery.InstallationHint{ProviderID: installation.ProviderID, HostID: installation.HostID, SurfaceID: installation.SurfaceID, Location: installation.Location, DiscoveryProbeID: installation.DiscoveryProbeID})
		}
	}
	discovered, err := discovery.Discover(snapshot.Catalog(), discovery.Options{HostID: hostID, UserHome: fingerprint.Root, Installations: hints})
	if err != nil {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("discover pilot Provider: %w", err)
	}
	candidates := discovered.Candidates(providerID)
	if len(candidates) != 1 || candidates[0].Location != filepath.Dir(fingerprint.SkillPath) {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot Provider must resolve to the exact Skill directory")
	}
	provider, found := catalogProvider(snapshot.Catalog(), providerID)
	if !found {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot Provider descriptor is missing")
	}
	capability, found := catalogCapability(provider, capabilityID)
	if !found || len(capability.HostBindings) != 1 {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot review Capability is missing or ambiguous")
	}
	binding := capability.HostBindings[0]
	if binding.Host != hostID || binding.Reference != bindingReference || binding.Kind != "skill" {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot review Binding is not the required Skill")
	}
	inventory, err := host.NewBindingInventory(hostID, []host.BindingObservation{{
		HostID: hostID, InstallationKey: candidates[0].InstallationKey, Binding: binding,
		Topologies: []execution.Topology{execution.TopologyCurrent}, Source: "host-filesystem",
		EvidenceReference: fingerprint.SkillPath, Digest: fingerprint.SkillDigest,
	}})
	if err != nil {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("build pilot Host inventory: %w", err)
	}
	resolved, err := core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: hostID, Discovery: discovered, Inventory: &inventory})
	if err != nil {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("resolve pilot Provider: %w", err)
	}
	resolution, found := resolved.Report.Resolution(providerID)
	if !found || resolution.State != registry.Verified || resolution.Instance == nil {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot Provider is not verified: %v", resolution.State)
	}
	return resolved, inventory, nil
}

func catalogProvider(value catalog.Catalog, id string) (catalog.ProviderDescriptorRecord, bool) {
	for _, provider := range value.Providers() {
		if provider.ID == id {
			return provider, true
		}
	}
	return catalog.ProviderDescriptorRecord{}, false
}

func catalogCapability(provider catalog.ProviderDescriptorRecord, id string) (catalog.CapabilityRecord, bool) {
	for _, capability := range provider.Capabilities {
		if capability.ID == id {
			return capability, true
		}
	}
	return catalog.CapabilityRecord{}, false
}
