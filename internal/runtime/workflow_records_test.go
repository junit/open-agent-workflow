package runtime

import (
	"strings"
	"testing"

	"github.com/wifibaby4u/open-agent-workflow/internal/catalog"
	"github.com/wifibaby4u/open-agent-workflow/internal/host"
	"github.com/wifibaby4u/open-agent-workflow/internal/hosttest"
	"github.com/wifibaby4u/open-agent-workflow/internal/profile"
	"github.com/wifibaby4u/open-agent-workflow/internal/registry"
)

func TestLifecycleBundlePinsWorkflowSelectionAndTrustedInputs(t *testing.T) {
	configuration, integration := hosttest.LoadManagedSnapshot(t, "")
	graph := workflowRecordGraph(t)
	hostAdmission, err := host.AdmitWorkflow(configuration.HostIntegrations(), host.RuntimeFrame{HostID: "codex", IntegrationID: integration.ID}, []catalog.HostBinding{{Host: "codex", Kind: "skill", Reference: "acme:complete"}})
	if err != nil {
		t.Fatalf("host.AdmitWorkflow() error = %v", err)
	}
	bundle, err := newLifecycleBundle(bundleRequest{
		RunID: "run-0123456789abcdef0123456789abcdef", DeliverableID: "delivery-1",
		InputDigest: strings.Repeat("1", 64), Generation: 1, CreatedRevision: 2,
		Selection:     ProfileSelection{Profile: "acme/delivery", Bindings: []profile.ProfileBinding{}},
		Configuration: configuration.Record(), RegistryDigest: strings.Repeat("2", 64), Graph: graph.Record(), Host: hostAdmission,
	})
	if err != nil {
		t.Fatalf("newLifecycleBundle() error = %v", err)
	}
	if bundle.Generation != 1 || bundle.Selection.Profile != "acme/delivery" || bundle.RecipeID != "acme/delivery" || bundle.RecipeDigest != graph.RecipeDigest() || bundle.RegistryDigest != strings.Repeat("2", 64) || bundle.Configuration.Digest != configuration.Digest() || bundle.GraphDigest != graph.Digest() || len(bundle.ProviderInstances) != 1 || bundle.ID == "" || bundle.Digest == "" {
		t.Fatalf("bundle = %#v", bundle)
	}
	if err := validateLifecycleBundle(bundle); err != nil {
		t.Fatalf("validateLifecycleBundle() error = %v", err)
	}
	tamperedAdmission := host.CloneWorkflowAdmission(hostAdmission)
	tamperedAdmission.IntegrationDigest = strings.Repeat("0", 64)
	if _, err := newLifecycleBundle(bundleRequest{
		RunID: "run-0123456789abcdef0123456789abcdef", DeliverableID: "delivery-1",
		InputDigest: strings.Repeat("1", 64), Generation: 1, CreatedRevision: 2,
		Selection: ProfileSelection{Profile: "acme/delivery"}, Configuration: configuration.Record(),
		RegistryDigest: strings.Repeat("2", 64), Graph: graph.Record(), Host: tamperedAdmission,
	}); ErrorCode(err) != "WORKFLOW_BUNDLE_INVALID" {
		t.Fatalf("tampered Bundle Host admission error = %v", err)
	}

	copyValue := cloneLifecycleBundle(bundle)
	copyValue.Graph.Digest = strings.Repeat("0", 64)
	if err := validateLifecycleBundle(copyValue); err == nil {
		t.Fatal("validateLifecycleBundle() accepted tampered graph")
	}
	copyValue = cloneLifecycleBundle(bundle)
	copyValue.ProviderInstances[0].InstanceDigest = "mutated"
	if bundle.ProviderInstances[0].InstanceDigest == "mutated" {
		t.Fatal("cloneLifecycleBundle() exposed mutable provider storage")
	}
}

type workflowRecordRegistry struct {
	provider   registry.ProviderInstance
	capability registry.VerifiedCapability
}

func (value workflowRecordRegistry) Provider(id string) (registry.ProviderInstance, bool) {
	return value.provider, id == value.provider.ProviderID
}

func (value workflowRecordRegistry) Capability(providerID, capabilityID string) (registry.VerifiedCapability, bool) {
	return value.capability, providerID == value.provider.ProviderID && capabilityID == value.capability.ID
}

func workflowRecordGraph(t *testing.T) profile.ExecutionGraph {
	t.Helper()
	binding := catalog.HostBinding{Host: "codex", Kind: "skill", Reference: "acme:complete"}
	provider := catalog.ProviderDescriptorRecord{
		SchemaVersion: catalog.ProviderDescriptorSchemaV2, DescriptorVersion: "2.0.0",
		ID: "acme/suite", DisplayName: "Acme", Discovery: []catalog.DiscoveryProbe{{ID: "codex", Hosts: []string{"codex"}, Surface: "codex-skills", Distribution: "acme", Kind: "path-exists", Root: "user-home", CandidatePath: ".agents/skills/acme", EvidencePath: "SKILL.md"}},
		Capabilities: []catalog.CapabilityRecord{{
			ID: "completion", InputSchema: "acme.input/v1", OutcomeSchema: "acme.output/v1",
			MaximumEffects: []string{"read-project"}, Resources: []string{"project"},
			RequestModes: []catalog.RequestMode{catalog.RequestModeWorkflow}, Responsibilities: []string{"completion"},
			ExecutorTopology: catalog.IsolatedRequired, DelegationAllowList: []string{}, HostBindings: []catalog.HostBinding{binding},
		}},
	}
	recipe := catalog.ProfileRecipeRecord{
		SchemaVersion: catalog.ProfileRecipeSchemaV1, RecipeVersion: "1.0.0", ID: "acme/delivery", DisplayName: "Acme Delivery",
		RequiredResponsibilities: []string{"completion"},
		Nodes:                    []catalog.RecipeNode{{ID: "completion", Kind: catalog.GateNode, Responsibility: "completion", Selector: catalog.CapabilitySelector{ProviderID: "acme/suite", CapabilityID: "completion"}, Transitions: []catalog.RecipeTransition{}}},
		IncidentRoutes:           []catalog.IncidentRoute{}, Entry: "completion", TerminalGates: []string{"completion"}, StableBoundaries: []string{"complete"},
	}
	available, err := catalog.New([]catalog.ProviderDescriptorRecord{provider}, []catalog.ProfileRecipeRecord{recipe}, nil)
	if err != nil {
		t.Fatalf("catalog.New() error = %v", err)
	}
	verified := workflowRecordRegistry{
		provider:   registry.ProviderInstance{ProviderID: "acme/suite", Digest: strings.Repeat("3", 64), Capabilities: []registry.VerifiedCapability{{ID: "completion", Binding: binding}}},
		capability: registry.VerifiedCapability{ID: "completion", Binding: binding},
	}
	graph, err := profile.CompileRecipe(available, verified, recipe, nil)
	if err != nil {
		t.Fatalf("profile.CompileRecipe() error = %v", err)
	}
	return graph
}
