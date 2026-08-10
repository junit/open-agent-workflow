package integration_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/builtin"
	"github.com/wifibaby4u/open-agent-workflow/internal/canonicaljson"
	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/discovery"
	"github.com/wifibaby4u/open-agent-workflow/internal/execution"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/provideraudit"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

type profileFixtureOptions struct {
	hostID              string
	topology            execution.Topology
	complete            bool
	unavailableFeatures map[host.FeatureID]bool
}

type builtInProfileFixture struct {
	catalog  catalog.Catalog
	audit    provideraudit.Manifest
	matrix   builtin.ProfileMatrixRecord
	registry *integrationRegistry
	evidence profile.HostEvidence
	hostID   string
	topology execution.Topology
}

type integrationRegistry struct {
	hostID       string
	providers    []registry.ProviderInstance
	providerByID map[string]registry.ProviderInstance
	bindings     map[string]registry.VerifiedBinding
	capabilities map[string]registry.VerifiedCapability
	digest       string
}

func (value *integrationRegistry) HostID() string { return value.hostID }

func (value *integrationRegistry) Providers() []registry.ProviderInstance {
	return append([]registry.ProviderInstance{}, value.providers...)
}

func (value *integrationRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	provider, found := value.providerByID[id]
	return provider, found
}

func (value *integrationRegistry) Binding(providerID, bindingID string) (registry.VerifiedBinding, bool) {
	binding, found := value.bindings[providerID+"\x00"+bindingID]
	binding.SupportedTopologies = append([]execution.Topology{}, binding.SupportedTopologies...)
	return binding, found
}

func (value *integrationRegistry) Bindings(providerID string) []registry.VerifiedBinding {
	result := []registry.VerifiedBinding{}
	for _, provider := range value.providers {
		if provider.ProviderID == providerID {
			result = append(result, provider.Bindings...)
			break
		}
	}
	return result
}

func (value *integrationRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	capability, found := value.capabilities[providerID+"\x00"+capabilityID]
	capability.BindingIDs = append([]string{}, capability.BindingIDs...)
	return capability, found
}

func (value *integrationRegistry) Digest() string { return value.digest }

func completeCodexEvidence(t testing.TB) builtInProfileFixture {
	t.Helper()
	return newBuiltInProfileFixture(t, profileFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent, complete: true})
}

func completeClaudeEvidence(t testing.TB) builtInProfileFixture {
	t.Helper()
	return newBuiltInProfileFixture(t, profileFixtureOptions{hostID: "claude", topology: execution.TopologyCurrent, complete: true})
}

func currentCodexV1Evidence(t testing.TB) builtInProfileFixture {
	t.Helper()
	return newBuiltInProfileFixture(t, profileFixtureOptions{hostID: "codex", topology: execution.TopologyCurrent})
}

func completeEvidenceWithOptions(t testing.TB, options profileFixtureOptions) builtInProfileFixture {
	t.Helper()
	options.complete = true
	return newBuiltInProfileFixture(t, options)
}

func newBuiltInProfileFixture(t testing.TB, options profileFixtureOptions) builtInProfileFixture {
	t.Helper()
	available, err := builtin.Load()
	if err != nil {
		t.Fatal(err)
	}
	audit, err := builtin.LoadSourceAudit()
	if err != nil {
		t.Fatal(err)
	}
	matrix, err := builtin.LoadProfileMatrix()
	if err != nil {
		t.Fatal(err)
	}
	if options.topology == "" {
		options.topology = execution.TopologyCurrent
	}

	observations := profileBindingObservations(t, available, options)
	inventory, err := host.BuildBindingInventoryV3(options.hostID, observations)
	if err != nil {
		t.Fatal(err)
	}
	manifest := profileHostManifest(t, observations, options)
	environment := profileEnvironmentReport(t, options)
	session := profileHostSession(t, manifest, inventory, environment, options)
	evidence, err := profile.NewHostEvidence(manifest, session, inventory, environment)
	if err != nil {
		t.Fatal(err)
	}
	return builtInProfileFixture{
		catalog: available, audit: audit, matrix: matrix,
		registry: profileEffectiveRegistry(t, available, inventory, observations, options.hostID),
		evidence: evidence, hostID: options.hostID, topology: options.topology,
	}
}

func profileBindingObservations(t testing.TB, available catalog.Catalog, options profileFixtureOptions) []host.BindingObservation {
	t.Helper()
	result := []host.BindingObservation{}
	for _, provider := range available.Providers() {
		for _, binding := range provider.Bindings {
			if binding.Host != options.hostID || !options.complete && binding.Kind != catalog.BindingSkill {
				continue
			}
			observation, err := host.NewBindingObservation(host.BindingObservation{
				HostID: options.hostID, ProviderID: provider.ID, InstallationKey: profileInstallationKey(provider.ID),
				DistributionID: binding.DistributionID, BindingID: binding.ID, Surface: binding.Surface, Kind: binding.Kind,
				Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
				Topologies: append([]execution.Topology{}, binding.SupportedTopologies...), Source: host.SourceNativeAPI,
				EvidenceReference: "evidence://integration/bindings/" + options.hostID + "/" + strings.ReplaceAll(provider.ID, "/", "-") + "/" + binding.ID,
			})
			if err != nil {
				t.Fatalf("Binding observation %s/%s: %v", provider.ID, binding.ID, err)
			}
			result = append(result, observation)
		}
	}
	return result
}

func profileHostManifest(t testing.TB, observations []host.BindingObservation, options profileFixtureOptions) host.Manifest {
	t.Helper()
	kindSet := map[catalog.BindingKind]struct{}{}
	for _, observation := range observations {
		kindSet[observation.Kind] = struct{}{}
	}
	kinds := make([]catalog.BindingKind, 0, len(kindSet))
	for kind := range kindSet {
		kinds = append(kinds, kind)
	}
	topologies := []execution.Topology{execution.TopologyCurrent}
	features := []host.Feature{host.FeatureNormalizedReceipts, host.FeatureProviderBindingInventory}
	delegation := []host.FeatureID{}
	actions := []host.HostActionContract{}
	if options.complete {
		topologies = []execution.Topology{execution.TopologyCurrent, execution.TopologySubagent}
		features = append(features, host.FeatureEnvironmentReporting)
		delegation = []host.FeatureID{
			host.FeatureChildDelegation, host.FeatureNestedChildDelegation,
			host.FeatureNestedParallelDelegation, host.FeatureParallelChildDelegation,
		}
		actions = profileHostActions()
	}
	manifest, err := host.NewManifest(host.Manifest{
		SchemaVersion: host.HostManifestSchemaV3, ManifestVersion: "3.0.0", HostID: options.hostID,
		ControlSurface: host.SurfaceHostNative, Protocols: []string{host.WorkflowProtocolV1}, BindingKinds: kinds,
		SupportedTopologies: topologies, Features: features, DelegationFeatures: delegation, HostActions: actions,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func profileEnvironmentReport(t testing.TB, options profileFixtureOptions) host.EnvironmentReport {
	t.Helper()
	report := host.EnvironmentReport{
		SchemaVersion: host.HostEnvironmentReportSchemaV2, SessionID: "session-" + options.hostID,
		Topology: options.topology, Observations: []execution.EnvironmentObservation{},
	}
	if options.topology == execution.TopologySubagent {
		report.SessionID = "session-" + options.hostID + "-child"
		report.ParentSessionID = "session-" + options.hostID
	}
	value, err := host.NewEnvironmentReport(report)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func profileHostSession(t testing.TB, manifest host.Manifest, inventory host.BindingInventory, environment host.EnvironmentReport, options profileFixtureOptions) host.SessionSnapshot {
	t.Helper()
	features := make([]host.FeatureObservation, len(manifest.DelegationFeatures))
	for index, feature := range manifest.DelegationFeatures {
		state := host.AvailabilityAvailable
		if options.unavailableFeatures[feature] {
			state = host.AvailabilityUnavailable
		}
		observation, err := host.NewFeatureObservation(host.FeatureObservation{
			Feature: feature, State: state, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://integration/features/" + options.hostID + "/" + string(feature),
		})
		if err != nil {
			t.Fatal(err)
		}
		features[index] = observation
	}
	actions := make([]host.HostActionObservation, len(manifest.HostActions))
	for index, action := range manifest.HostActions {
		observation, err := host.NewHostActionObservation(host.HostActionObservation{
			Action: action, State: host.AvailabilityAvailable, Source: host.SourceNativeAPI,
			EvidenceReference: "evidence://integration/actions/" + options.hostID + "/" + action.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		actions[index] = observation
	}
	session, err := host.NewSessionSnapshot(manifest, host.SessionSnapshot{
		SchemaVersion: host.HostSessionSchemaV3, HostID: options.hostID, IntegrationID: "test/" + options.hostID + "-host",
		IntegrationVersion: "3.0.0", SessionID: "session-" + options.hostID, ManifestDigest: manifest.Digest,
		SupportedTopologies: append([]execution.Topology{}, manifest.SupportedTopologies...), ProviderInventoryDigest: inventory.Digest,
		FeatureObservations: features, HostActionObservations: actions, EnvironmentReportDigest: environment.Digest,
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func profileEffectiveRegistry(t testing.TB, available catalog.Catalog, inventory host.BindingInventory, observations []host.BindingObservation, hostID string) *integrationRegistry {
	t.Helper()
	observed := map[string]host.BindingObservation{}
	for _, observation := range observations {
		observed[observation.ProviderID+"\x00"+observation.BindingID] = observation
	}
	result := &integrationRegistry{
		hostID: hostID, providerByID: map[string]registry.ProviderInstance{}, bindings: map[string]registry.VerifiedBinding{},
		capabilities: map[string]registry.VerifiedCapability{},
	}
	for _, provider := range available.Providers() {
		verifiedBindings := []registry.VerifiedBinding{}
		for _, binding := range provider.Bindings {
			observation, found := observed[provider.ID+"\x00"+binding.ID]
			if !found {
				continue
			}
			verified := registry.VerifiedBinding{
				BindingID: binding.ID, DistributionID: binding.DistributionID, DistributionRevision: provider.Distributions[0].Revision,
				DistributionTreeDigest: provider.Distributions[0].TreeDigest, Surface: binding.Surface, Kind: binding.Kind,
				Reference: binding.Reference, Invocation: binding.Invocation, BindingTreeDigest: binding.TreeDigest,
				SupportedTopologies: append([]execution.Topology{}, observation.Topologies...), Delegation: binding.Delegation,
				Provenance: discovery.ProvenanceDistributionAttested, BindingEvidenceDigest: observation.Digest,
			}
			verifiedBindings = append(verifiedBindings, verified)
			result.bindings[provider.ID+"\x00"+binding.ID] = verified
		}
		if len(verifiedBindings) == 0 {
			continue
		}
		verifiedCapabilities := []registry.VerifiedCapability{}
		for _, capability := range provider.Capabilities {
			bindingIDs := []string{}
			for _, bindingID := range capability.BindingRefs {
				if _, found := result.bindings[provider.ID+"\x00"+bindingID]; found {
					bindingIDs = append(bindingIDs, bindingID)
				}
			}
			if len(bindingIDs) == 0 {
				continue
			}
			value := registry.VerifiedCapability{ID: capability.ID, BindingIDs: bindingIDs}
			verifiedCapabilities = append(verifiedCapabilities, value)
			result.capabilities[provider.ID+"\x00"+capability.ID] = value
		}
		descriptorDigest, _, err := canonicaljson.Digest(provider)
		if err != nil {
			t.Fatal(err)
		}
		instance := registry.ProviderInstance{
			ProviderID: provider.ID, HostID: hostID, DescriptorDigest: descriptorDigest,
			DistributionID: provider.Distributions[0].ID, DistributionRevision: provider.Distributions[0].Revision,
			DistributionTreeDigest: provider.Distributions[0].TreeDigest, InstallationKey: profileInstallationKey(provider.ID),
			ConfigurationDigest: canonicaljson.DigestBytes([]byte(provider.ID + "/configuration")), BindingInventoryDigest: inventory.Digest,
			EvidenceDigest: canonicaljson.DigestBytes([]byte(provider.ID + "/evidence/" + inventory.Digest)),
			Bindings:       verifiedBindings, Capabilities: verifiedCapabilities,
		}
		instance.Digest = profileRecordDigest(instance)
		result.providers = append(result.providers, instance)
		result.providerByID[provider.ID] = instance
	}
	sort.Slice(result.providers, func(left, right int) bool {
		return result.providers[left].ProviderID < result.providers[right].ProviderID
	})
	result.digest, _, _ = canonicaljson.Digest(struct {
		HostID    string                      `json:"host_id"`
		Inventory string                      `json:"inventory"`
		Providers []registry.ProviderInstance `json:"providers"`
	}{hostID, inventory.Digest, result.providers})
	return result
}

func profileRecordDigest(value registry.ProviderInstance) string {
	value.Digest = ""
	digest, _, _ := canonicaljson.Digest(value)
	return digest
}

func profileInstallationKey(providerID string) string {
	return "installation-" + strings.ReplaceAll(providerID, "/", "-")
}

func profileHostActions() []host.HostActionContract {
	return []host.HostActionContract{
		{ID: "workspace.prepare-or-confirm", InputSchema: "oaw.host-action.workspace-input/v1", OutcomeSchema: "oaw.host-action.workspace-outcome/v1", MaximumEffects: []string{"git-local", "read-project", "run-process", "write-project"}, Resources: []string{"git-repository", "project-worktree"}},
		{ID: "verification.execute", InputSchema: "oaw.host-action.verification-input/v1", OutcomeSchema: "oaw.host-action.verification-outcome/v1", MaximumEffects: []string{"read-project", "run-process"}, Resources: []string{"project"}},
		{ID: "closeout.execute", InputSchema: "oaw.host-action.closeout-input/v1", OutcomeSchema: "oaw.host-action.closeout-outcome/v1", MaximumEffects: []string{"git-local", "network-mutation", "read-project", "run-process"}, Resources: []string{"git-repository", "network", "project-worktree"}},
	}
}

func profileRecipeFor(t testing.TB, available catalog.Catalog, identity string) catalog.ProfileRecipeRecord {
	t.Helper()
	recipeID := identity
	for _, alias := range available.Aliases() {
		if alias.Alias == identity {
			recipeID = alias.RecipeID
			break
		}
	}
	for _, recipe := range available.Recipes() {
		if recipe.ID == recipeID {
			return recipe
		}
	}
	t.Fatalf("Recipe %s not found", identity)
	return catalog.ProfileRecipeRecord{}
}

func profileCompileRequest(t testing.TB, fixture builtInProfileFixture, identity string, recipe catalog.ProfileRecipeRecord) profile.CompileRequest {
	t.Helper()
	overlays := []string{}
	if recipe.Template == "default" {
		for _, overlay := range recipe.Overlays {
			if overlay.ID == "default-inline" {
				overlays = append(overlays, overlay.ID)
			}
		}
	}
	return profile.CompileRequest{
		Profile: identity, Topology: fixture.topology, AddOns: []string{}, Overlays: overlays,
		Alternatives: profileAlternativesForHost(t, fixture.catalog, recipe, fixture.hostID), Host: fixture.evidence,
	}
}

func profileAlternativesForHost(t testing.TB, available catalog.Catalog, recipe catalog.ProfileRecipeRecord, hostID string) []profile.AlternativeChoice {
	t.Helper()
	providers := map[string]catalog.ProviderDescriptorRecord{}
	for _, provider := range available.Providers() {
		providers[provider.ID] = provider
	}
	choices := []profile.AlternativeChoice{}
	for _, slot := range recipe.Slots {
		for _, step := range slot.Pipeline {
			provider := providers[step.Selector.ProviderID]
			binding := profileDescriptorBinding(t, provider, step.Selector.BindingID)
			if binding.Host == hostID {
				continue
			}
			alternativeID := ""
			for _, candidateID := range binding.Alternatives {
				candidate := profileDescriptorBinding(t, provider, candidateID)
				if candidate.Host == hostID {
					alternativeID = candidateID
					break
				}
			}
			if alternativeID == "" {
				t.Fatalf("%s/%s has no %s alternative", provider.ID, binding.ID, hostID)
			}
			choices = append(choices, profile.AlternativeChoice{
				SlotID: slot.SlotID, StepID: step.ID, AlternativeID: alternativeID,
				Selector: catalog.BindingSelector{ProviderID: provider.ID, BindingID: alternativeID},
			})
		}
	}
	return choices
}

func profileDescriptorBinding(t testing.TB, provider catalog.ProviderDescriptorRecord, bindingID string) catalog.BindingRecord {
	t.Helper()
	for _, binding := range provider.Bindings {
		if binding.ID == bindingID {
			return binding
		}
	}
	t.Fatalf("Binding %s/%s not found", provider.ID, bindingID)
	return catalog.BindingRecord{}
}

func profileCompiledSlot(t testing.TB, graph profile.ExecutionGraphRecord, slotID catalog.SlotID) profile.CompiledSlot {
	t.Helper()
	for _, slot := range graph.Slots {
		if slot.SlotID == slotID {
			return slot
		}
	}
	t.Fatalf("compiled slot %s not found", slotID)
	return profile.CompiledSlot{}
}

func profileCompile(t testing.TB, fixture builtInProfileFixture, identity string) (profile.ExecutionGraphRecord, profile.CompileResult) {
	t.Helper()
	recipe := profileRecipeFor(t, fixture.catalog, identity)
	request := profileCompileRequest(t, fixture, identity, recipe)
	result, err := profile.CompileProfile(fixture.catalog, fixture.registry, request)
	if err != nil {
		t.Fatal(err)
	}
	graph, found := result.Graph()
	if !found {
		t.Fatalf("CompileProfile(%s) diagnostics = %#v", identity, result.Diagnostics())
	}
	return graph, result
}
