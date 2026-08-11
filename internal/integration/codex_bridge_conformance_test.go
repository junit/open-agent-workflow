package integration_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge"
	"github.com/wifibaby4u/open-agent-workflow/internal/codexbridge/appserver"
	"github.com/wifibaby4u/open-agent-workflow/internal/config"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/integrity"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
)

func TestCodexBridgeUsesOneBindingAlgorithmForEveryProvider(t *testing.T) {
	cases := []struct {
		provider string
		skill    string
	}{
		{provider: "oaw/superpowers", skill: "superpowers:writing-plans"},
		{provider: "oaw/matt", skill: "tdd"},
		{provider: "oaw/ecc", skill: "tdd-workflow"},
		{provider: "acme/custom", skill: "acme:delivery"},
	}
	for _, test := range cases {
		t.Run(test.provider, func(t *testing.T) {
			assertExactSkillBinding(t, test.provider, test.skill)
		})
	}
}

func TestCodexBridgeECCFullFixtureRequiresEverySkill(t *testing.T) {
	assertECCFullSkillCoverage(t)
}

func TestCodexPolicyInstallationNeverClaimsHostNative(t *testing.T) {
	snapshot := loadPolicyAndNativeCodexSnapshot(t)
	if integrationCanSupplyInventory(snapshot, "oaw/codex-policy") {
		t.Fatal("policy Integration supplied Host authority")
	}
	if !integrationCanSupplyInventory(snapshot, "oaw/codex-host") {
		t.Fatal("audited Host-native Integration did not supply inventory authority")
	}
}

func assertExactSkillBinding(t *testing.T, providerID, skill string) {
	t.Helper()
	home := t.TempDir()
	providerRoot := filepath.Join(home, ".codex", "plugins", "provider")
	skillRoot := filepath.Join(providerRoot, "skills", "observed")
	skillPath := filepath.Join(skillRoot, "SKILL.md")
	writeBridgeFixtureFile(t, filepath.Join(providerRoot, "provider.marker"), providerID)
	writeBridgeFixtureFile(t, skillPath, "---\nname: "+skill+"\n---\n")
	bindingTree := digestBridgeFixtureTree(t, skillRoot)
	distributionTree := digestBridgeFixtureTree(t, providerRoot)
	descriptor := singleSkillProvider(providerID, skill, bindingTree, distributionTree)
	available, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(available, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	inventory, diagnostics, err := codexbridge.BuildBindingInventory(available, report, appserver.MetadataObservation{
		Skills: appserver.SkillsEntry{CWD: home, Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{{
			Name: skill, Enabled: true, Path: skillPath, Scope: "user",
		}}},
	}, home)
	if err != nil || len(diagnostics) != 0 || len(inventory.Observations) != 1 {
		t.Fatalf("inventory = %#v, diagnostics = %#v, error = %v", inventory, diagnostics, err)
	}
	observation := inventory.Observations[0]
	candidates := report.Candidates(providerID)
	if len(candidates) != 1 || observation.InstallationKey != candidates[0].InstallationKey ||
		observation.ProviderID != providerID || observation.DistributionID != "fixture" || observation.BindingID != "codex-delivery" ||
		observation.Reference != skill || observation.Kind != catalog.BindingSkill || observation.Invocation != catalog.InvocationModel ||
		observation.BindingTreeDigest != bindingTree || observation.Source != host.SourceNativeAPI {
		t.Fatalf("candidates = %#v, observation = %#v", candidates, observation)
	}
}

func singleSkillProvider(providerID, skill, bindingTree, distributionTree string) catalog.ProviderDescriptorRecord {
	return catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV4, DescriptorVersion: "4.0.0",
		ID: providerID, DisplayName: providerID,
		Distributions: []catalog.DistributionRecord{{
			ID: "fixture", SourceURI: "https://example.test/provider", Revision: strings.Repeat("a", 40), TreeDigest: distributionTree,
		}},
		Discovery: []catalog.DiscoveryProbe{{
			ID: "codex-skill", Hosts: []string{"codex"}, Surface: "codex-plugin", DistributionID: "fixture",
			Kind: "path-exists", Root: "user-home", CandidatePath: ".codex/plugins/provider", EvidencePath: "provider.marker",
		}},
		Bindings: []catalog.BindingRecord{{
			ID: "codex-delivery", DistributionID: "fixture", ContentRoot: "skills/observed", InstallRoot: "skills/observed", TreeDigest: bindingTree,
			Host: "codex", Surface: "codex-plugin", Kind: catalog.BindingSkill, Reference: skill, Invocation: catalog.InvocationModel,
			Responsibilities: []catalog.ResponsibilityClaim{{
				Namespace: catalog.OwnershipStage, Name: string(catalog.SlotImplementation), SlotID: catalog.SlotImplementation, OutcomeOwner: true,
			}},
			InputArtifact: "artifact", OutputArtifact: "artifact", MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			SupportedTopologies: []execution.Topology{execution.TopologyCurrent}, Delegation: catalog.DelegationRequirements{},
			StageSpan: []catalog.SlotID{catalog.SlotImplementation}, InternalCalls: []catalog.InternalCall{}, Alternatives: []string{}, Conflicts: []string{},
		}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "delivery", InputSchema: "artifact", OutcomeSchema: "artifact",
			RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, BindingRefs: []string{"codex-delivery"},
		}},
	}
}

func assertECCFullSkillCoverage(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	providerRoot := filepath.Join(home, ".codex", "plugins", "provider")
	skillRoot := filepath.Join(providerRoot, "skills", "observed")
	skillPath := filepath.Join(skillRoot, "SKILL.md")
	writeBridgeFixtureFile(t, filepath.Join(providerRoot, "provider.marker"), "fixture/ecc")
	writeBridgeFixtureFile(t, skillPath, "---\nname: tdd-workflow\n---\n")
	descriptor := singleSkillProvider(
		"fixture/ecc", "tdd-workflow", digestBridgeFixtureTree(t, skillRoot), digestBridgeFixtureTree(t, providerRoot),
	)
	claims := make([]catalog.ResponsibilityClaim, 0, len(catalog.CanonicalSlots()))
	stageSpan := make([]catalog.SlotID, 0, len(catalog.CanonicalSlots()))
	for _, definition := range catalog.CanonicalSlots() {
		claims = append(claims, catalog.ResponsibilityClaim{
			Namespace: catalog.OwnershipStage, Name: string(definition.ID), SlotID: definition.ID, OutcomeOwner: true,
		})
		stageSpan = append(stageSpan, definition.ID)
	}
	descriptor.Bindings[0].Responsibilities = claims
	descriptor.Bindings[0].StageSpan = stageSpan
	recipe := completeSkillLifecycleRecipe(t, descriptor.ID, descriptor.Bindings[0].ID)
	fixtureCatalog, err := catalog.New([]catalog.ProviderDescriptorRecord{descriptor}, []catalog.ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatal(err)
	}
	report, err := discovery.Discover(fixtureCatalog, discovery.Options{HostID: "codex", UserHome: home})
	if err != nil {
		t.Fatal(err)
	}
	metadata := appserver.MetadataObservation{Skills: appserver.SkillsEntry{
		CWD: home, Errors: []appserver.MetadataError{}, Skills: []appserver.SkillMetadata{{Name: "tdd-workflow", Enabled: true, Path: skillPath, Scope: "user"}},
	}}
	fullInventory, diagnostics, err := codexbridge.BuildBindingInventory(fixtureCatalog, report, metadata, home)
	if err != nil || len(diagnostics) != 0 || len(fullInventory.Observations) != 1 {
		t.Fatalf("inventory = %#v, diagnostics = %#v, error = %v", fullInventory, diagnostics, err)
	}
	fullEvidence := bridgeProfileEvidence(t, fullInventory)
	fullRegistry := profileEffectiveRegistry(t, fixtureCatalog, fullInventory, fullInventory.Observations, "codex")
	request := profile.CompileRequest{
		Profile: recipe.ID, Topology: execution.TopologyCurrent, AddOns: []string{}, Alternatives: []profile.AlternativeChoice{}, Overlays: []string{}, Host: fullEvidence,
	}
	compiled, err := profile.CompileProfile(fixtureCatalog, fullRegistry, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := compiled.Graph(); !found {
		t.Fatalf("complete ECC Skill fixture diagnostics = %#v", compiled.Diagnostics())
	}

	missing := metadata
	missing.Skills.Skills = []appserver.SkillMetadata{}
	partialInventory, diagnostics, err := codexbridge.BuildBindingInventory(fixtureCatalog, report, missing, home)
	if err != nil || len(diagnostics) != 0 || len(partialInventory.Observations) != 0 {
		t.Fatalf("partial inventory = %#v, diagnostics = %#v, error = %v", partialInventory, diagnostics, err)
	}
	partialEvidence := bridgeProfileEvidence(t, partialInventory)
	partialRegistry := profileEffectiveRegistry(t, fixtureCatalog, partialInventory, partialInventory.Observations, "codex")
	request.Host = partialEvidence
	partial, err := profile.CompileProfile(fixtureCatalog, partialRegistry, request)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnostic(t, partial, "PROFILE_BINDING_UNAVAILABLE", descriptor.Bindings[0].ID)
}

func completeSkillLifecycleRecipe(t *testing.T, providerID, bindingID string) catalog.ProfileRecipeRecord {
	t.Helper()
	available, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	var recipe catalog.ProfileRecipeRecord
	for _, candidate := range available.Recipes() {
		if candidate.ID == "oaw/delivery" {
			recipe = candidate
			break
		}
	}
	if recipe.ID == "" {
		t.Fatal("built-in delivery Recipe missing")
	}
	recipe.ID = "fixture/ecc-full"
	recipe.DisplayName = "ECC Full Skill Fixture"
	recipe.Family = "user-defined"
	recipe.Template = ""
	recipe.AddOns = []catalog.AddOnRecord{}
	recipe.IncidentRoutes = []catalog.IncidentRoute{}
	recipe.Overlays = []catalog.OverlayRecord{}
	for index := range recipe.Slots {
		slotID := recipe.Slots[index].SlotID
		stepID := "fixture-" + string(slotID)
		recipe.Slots[index].Pipeline = []catalog.PipelineStep{{
			ID: stepID, Selector: catalog.BindingSelector{ProviderID: providerID, BindingID: bindingID}, StageSpan: []catalog.SlotID{slotID},
			RequiredInputArtifact: "artifact", ProducedOutputArtifact: "artifact",
		}}
		recipe.Slots[index].OutcomeOwner = catalog.OutcomeOwner{Kind: catalog.OwnerProviderBinding, StepID: stepID}
		recipe.Slots[index].HostAction = nil
	}
	return recipe
}

func bridgeProfileEvidence(t *testing.T, inventory host.BindingInventory) profile.HostEvidence {
	t.Helper()
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: "codex",
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: []catalog.BindingKind{catalog.BindingSkill},
		SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		Features:            []host.Feature{host.FeatureEnvironmentReporting, host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory},
		DelegationFeatures:  []host.FeatureID{}, HostActions: []host.HostActionContract{},
	})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := host.NewEnvironmentReport(host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-codex",
		Topology: execution.TopologyCurrent, Observations: []execution.EnvironmentObservation{},
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: "codex", IntegrationID: "test/codex-host", IntegrationVersion: "3.0.0",
		SessionID: "session-codex", ManifestDigest: manifest.Digest, SupportedTopologies: []execution.Topology{execution.TopologyCurrent},
		ProviderInventoryDigest: inventory.Digest, FeatureObservations: []host.FeatureObservation{}, HostActionObservations: []host.HostActionObservation{},
		EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func digestBridgeFixtureTree(t *testing.T, path string) string {
	t.Helper()
	tree, err := integrity.DigestTree(path)
	if err != nil {
		t.Fatal(err)
	}
	return tree.RootDigest
}

func loadPolicyAndNativeCodexSnapshot(t *testing.T) config.Snapshot {
	t.Helper()
	snapshot, err := config.Load(config.LoadOptions{ProjectRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func integrationCanSupplyInventory(snapshot config.Snapshot, id string) bool {
	for _, integration := range snapshot.HostIntegrations() {
		if integration.ID != id {
			continue
		}
		return integration.Manifest.ControlSurface == host.SurfaceHostNative &&
			slices.Contains(integration.Manifest.BindingKinds, catalog.BindingSkill) &&
			slices.Contains(integration.Manifest.Features, host.FeatureProviderBindingInventory) &&
			integration.Audit.Status == host.AuditPassed && integration.Conformance != nil
	}
	return false
}

func writeBridgeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
