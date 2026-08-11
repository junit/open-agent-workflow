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
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
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
	distributionRoot := filepath.Dir(skillDirectory)
	distributionTree, err := integrity.DigestTree(distributionRoot)
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("digest pilot Distribution: %w", err)
	}
	bindingTree, err := integrity.DigestTree(skillDirectory)
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("digest pilot Binding: %w", err)
	}
	provider := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0",
		ID: providerID, DisplayName: "OpenCodeReview Read-only Skill",
		Distributions: []catalog.DistributionRecord{{
			ID: providerDistributionID, SourceURI: "https://example.invalid/open-code-review",
			Revision: fingerprint.Commit, TreeDigest: distributionTree.RootDigest,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: providerProbeID, Hosts: []string{hostID}, Surface: providerSurface,
			DistributionID: providerDistributionID, Kind: "path-exists", Root: "user-home",
			CandidatePath: "skills", EvidencePath: filepath.ToSlash(filepath.Join("open-code-review", "SKILL.md")),
		}},
		Bindings: pilotBindings(bindingTree.RootDigest),
		Capabilities: []catalog.CapabilityRecord{{
			ID: capabilityID, InputSchema: workflowArtifactSchema, OutcomeSchema: workflowArtifactSchema,
			RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow},
			BindingRefs:  []string{scopeBindingID, reviewBindingID, verificationBindingID},
		}},
	}
	recipe := pilotRecipe()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: hostID,
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Features: []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		DelegationFeatures: []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host manifest: %w", err)
	}
	audit, err := host.NewAuditEvidence(host.AuditEvidence{Status: host.AuditPassed, References: []host.AuditEvidenceReference{{Reference: "evidence://dogfood/skill-tree-audit", Digest: canonicaljson.DigestBytes([]byte(fingerprint.SkillTreeDigest))}}})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host audit: %w", err)
	}
	conformance, err := host.NewConformanceReport(host.ConformanceReport{
		SchemaVersion: host.HostConformanceReportSchemaV4, ManifestDigest: manifest.ContentDigest(),
		HostSessionDigest:          canonicaljson.DigestBytes([]byte("oaw current dogfood conformance session\n")),
		BindingInventoryDigest:     canonicaljson.DigestBytes([]byte("oaw current dogfood conformance inventory\n")),
		TranscriptDigest:           canonicaljson.DigestBytes([]byte("oaw current dogfood transcript\n")),
		VerifiedFeatures:           []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		VerifiedDelegationFeatures: []host.FeatureID{}, VerifiedHostActionIDs: []string{}, Diagnostics: []string{},
	})
	if err != nil {
		return config.Snapshot{}, host.IntegrationRecord{}, fmt.Errorf("pilot Host conformance: %w", err)
	}
	integration, err := host.NewIntegration(host.IntegrationRecord{
		SchemaVersion: host.HostIntegrationSchemaV3, IntegrationVersion: "3.0.0", ID: integrationID,
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
	installationPath := strconv.Quote(distributionRoot)
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

func pilotBindings(treeDigest string) []catalog.BindingRecord {
	allSlots := pilotSlotIDs()
	scopeSlots := append([]catalog.SlotID{}, allSlots[:4]...)
	reviewSlots := append([]catalog.SlotID{}, allSlots[4:8]...)
	verificationSlots := append([]catalog.SlotID{}, allSlots[8:]...)
	return []catalog.BindingRecord{
		pilotBinding(scopeBindingID, treeDigest, scopeSlots, []catalog.ResponsibilityClaim{
			pilotClaim(catalog.OwnershipStage, catalog.SlotProblemFraming),
			pilotClaim(catalog.OwnershipStage, catalog.SlotSolutionSpecification),
			pilotClaim(catalog.OwnershipStage, catalog.SlotDeliveryPlanning),
			pilotClaim(catalog.OwnershipStage, catalog.SlotWorkspacePreparation),
		}),
		pilotBinding(reviewBindingID, treeDigest, reviewSlots, []catalog.ResponsibilityClaim{
			pilotClaim(catalog.OwnershipStage, catalog.SlotImplementation),
			pilotClaim(catalog.OwnershipProcedure, catalog.SlotImplementationTDD),
			pilotClaim(catalog.OwnershipAssurance, catalog.SlotReviewRemediation),
		}),
		pilotBinding(verificationBindingID, treeDigest, verificationSlots, []catalog.ResponsibilityClaim{
			pilotClaim(catalog.OwnershipProcedure, catalog.SlotFreshVerification),
			pilotClaim(catalog.OwnershipStage, catalog.SlotCloseout),
		}),
	}
}

func pilotBinding(id, treeDigest string, span []catalog.SlotID, claims []catalog.ResponsibilityClaim) catalog.BindingRecord {
	return catalog.BindingRecord{
		ID: id, DistributionID: providerDistributionID, ContentRoot: "skills/open-code-review", InstallRoot: "open-code-review", TreeDigest: treeDigest,
		Host: hostID, Surface: providerSurface, Kind: catalog.BindingSkill, Reference: bindingReference, Invocation: catalog.InvocationModel,
		Responsibilities: claims, InputArtifact: workflowArtifactSchema, OutputArtifact: workflowArtifactSchema,
		MaximumEffects: []string{"read-project"}, Resources: []string{"project"}, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Delegation: catalog.DelegationRequirements{}, StageSpan: span, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
	}
}

func pilotClaim(namespace catalog.OwnershipNamespace, slot catalog.SlotID) catalog.ResponsibilityClaim {
	return catalog.ResponsibilityClaim{Namespace: namespace, Name: string(slot), SlotID: slot, OutcomeOwner: true}
}

func pilotSlotIDs() []catalog.SlotID {
	definitions := catalog.CanonicalSlots()
	result := make([]catalog.SlotID, len(definitions))
	for index, definition := range definitions {
		result[index] = definition.ID
	}
	return result
}

func pilotRecipe() catalog.ProfileRecipeRecord {
	definitions := catalog.CanonicalSlots()
	slots := make([]catalog.SlotRecipe, len(definitions))
	for index, definition := range definitions {
		slot := catalog.SlotRecipe{
			SlotID: definition.ID, Applicability: catalog.SlotMandatory,
			Pipeline: []catalog.PipelineStep{}, Gates: []catalog.GateRecord{}, Transitions: []catalog.RecipeTransition{},
		}
		if definition.ID == catalog.SlotIncidentRecovery {
			slot.Applicability = catalog.SlotConditional
			slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerNone}
			slots[index] = slot
			continue
		}
		bindingID, span := pilotBindingForSlot(definition.ID)
		step := catalog.PipelineStep{
			ID: "run", Selector: catalog.BindingSelector{ProviderID: providerID, BindingID: bindingID}, StageSpan: span,
			RequiredInputArtifact: workflowArtifactSchema, ProducedOutputArtifact: workflowArtifactSchema,
		}
		slot.Pipeline = []catalog.PipelineStep{step}
		slot.OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: step.ID}
		if target, found := pilotNextSlot(definition.ID); found {
			slot.Transitions = []catalog.RecipeTransition{{Signal: "succeeded", Target: target}}
		}
		if definition.ID == catalog.SlotCloseout {
			slot.Gates = []catalog.GateRecord{{
				ID: "read-only-closeout", Authority: catalog.GateUser, Predicate: "approved-read-only-dogfood-closeout",
				EvidenceRequirements: []catalog.EvidenceRequirementRecord{{Kind: "user-decision", Minimum: 1, Description: "digest-pinned approved repository selection"}},
			}}
		}
		slots[index] = slot
	}
	return catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV3, TaxonomyVersion: catalog.TaxonomyVersionV1, RecipeVersion: "3.0.0", ID: profileID,
		DisplayName: "OpenCodeReview Current Read-only Workflow", Family: "dogfood", Slots: slots,
		AddOns: []catalog.AddOnRecord{}, IncidentRoutes: []catalog.IncidentRoute{}, Overlays: []catalog.OverlayRecord{},
		StableBoundaries: []string{"review-complete", "scope-complete", "verification-complete"}, EnvironmentRequirements: []execution.EnvironmentRequirement{},
	}
}

func pilotBindingForSlot(slot catalog.SlotID) (string, []catalog.SlotID) {
	allSlots := pilotSlotIDs()
	switch slot {
	case catalog.SlotProblemFraming, catalog.SlotSolutionSpecification, catalog.SlotDeliveryPlanning, catalog.SlotWorkspacePreparation:
		return scopeBindingID, append([]catalog.SlotID{}, allSlots[:4]...)
	case catalog.SlotImplementation, catalog.SlotImplementationTDD, catalog.SlotReviewRemediation:
		return reviewBindingID, append([]catalog.SlotID{}, allSlots[4:8]...)
	default:
		return verificationBindingID, append([]catalog.SlotID{}, allSlots[8:]...)
	}
}

func pilotNextSlot(slot catalog.SlotID) (catalog.SlotID, bool) {
	switch slot {
	case catalog.SlotProblemFraming:
		return catalog.SlotSolutionSpecification, true
	case catalog.SlotSolutionSpecification:
		return catalog.SlotDeliveryPlanning, true
	case catalog.SlotDeliveryPlanning:
		return catalog.SlotWorkspacePreparation, true
	case catalog.SlotWorkspacePreparation:
		return catalog.SlotImplementation, true
	case catalog.SlotImplementation:
		return catalog.SlotImplementationTDD, true
	case catalog.SlotImplementationTDD:
		return catalog.SlotReviewRemediation, true
	case catalog.SlotReviewRemediation:
		return catalog.SlotFreshVerification, true
	case catalog.SlotFreshVerification:
		return catalog.SlotCloseout, true
	default:
		return "", false
	}
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
	distributionRoot := filepath.Dir(filepath.Dir(fingerprint.SkillPath))
	if len(candidates) != 1 || candidates[0].DiagnosticLocation != distributionRoot {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot Provider must resolve to the exact Distribution directory")
	}
	provider, found := catalogProvider(snapshot.Catalog(), providerID)
	if !found {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot Provider descriptor is missing")
	}
	capability, found := catalogCapability(provider, capabilityID)
	if !found || len(capability.BindingRefs) != 3 || len(provider.Bindings) != 3 {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot review Capability is missing or ambiguous")
	}
	roots := make(map[string]discovery.BindingRootEvidence, len(candidates[0].BindingRoots))
	for _, root := range candidates[0].BindingRoots {
		roots[root.BindingID] = root
	}
	observations := make([]host.BindingObservation, 0, len(provider.Bindings))
	for _, binding := range provider.Bindings {
		root, exists := roots[binding.ID]
		if !exists || root.Tree.RootDigest != binding.TreeDigest || binding.Host != hostID || binding.Surface != providerSurface ||
			binding.DistributionID != providerDistributionID || binding.Reference != bindingReference || binding.Kind != catalog.BindingSkill {
			return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("pilot review Binding %q is unavailable or drifted", binding.ID)
		}
		observations = append(observations, host.BindingObservation{
			HostID: hostID, ProviderID: providerID, InstallationKey: candidates[0].InstallationKey,
			DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface,
			Kind: binding.Kind, Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: root.Tree.RootDigest,
			Topologies: []execution.Topology{execution.TopologyCurrent}, Source: host.SourceLiveFilesystem,
			EvidenceReference: "evidence://dogfood/binding/" + binding.ID,
		})
	}
	inventory, err := host.BuildBindingInventoryV3(hostID, observations)
	if err != nil {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("build pilot Host inventory: %w", err)
	}
	resolved, err := core.Resolve(core.ResolutionRequest{Configuration: snapshot, HostID: hostID, Discovery: discovered, Inventory: &inventory})
	if err != nil {
		return core.ResolutionResult{}, host.BindingInventory{}, fmt.Errorf("resolve pilot Provider: %w", err)
	}
	resolution, found := resolved.Report.Resolution(providerID)
	if !found || resolution.State != registry.ProviderVerified || resolution.Instance == nil {
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
